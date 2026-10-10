package postgres

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func parsePGTest(t *testing.T, source string) *pgStatement {
	t.Helper()
	tokens, parts, err := pgScan(source, SQLLimits{})
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(parts) != 1 {
		t.Fatalf("statements=%d", len(parts))
	}
	stmt, err := pgParseStatement(source, tokens, parts[0], SQLLimits{})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return stmt
}
func significantPG(tokens []pgToken) []pgToken {
	out := []pgToken{}
	for _, t := range tokens {
		if !pgTrivia(t) && t.Kind != pgEOF {
			out = append(out, t)
		}
	}
	return out
}
func TestPGScannerLosslessBoundaries(t *testing.T) {
	source := " \t-- <~> ;\nSELECT \"a;\"\"<~>\", 'x''; ARRAY[1]' FROM t /* outer /* nested ; */ tail */;\nCREATE FUNCTION f() RETURNS text AS $body$begin; '<~>'; $q$x$q$; end$body$ LANGUAGE sql;\nSELECT E'a\\'; <~>', B'101', U&'d\\0061t', $1::real[] FROM t; -- end\n"
	tokens, parts, err := pgScan(source, SQLLimits{})
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 3 {
		t.Fatalf("parts=%d", len(parts))
	}
	end := 0
	for _, token := range tokens {
		if token.Start != end || token.End < token.Start || token.End > len(source) {
			t.Fatalf("noncontiguous token: %+v after %d", token, end)
		}
		end = token.End
	}
	if end != len(source) {
		t.Fatal("source truncated")
	}
	sig := significantPG(tokens)
	if sig[1].Text != "a;\"<~>" || !sig[1].Quoted || sig[3].Text != "x'; ARRAY[1]" {
		t.Fatalf("decoded tokens: %+v", sig[:5])
	}
	if _, err := pgParseStatement(source, tokens, parts[0], SQLLimits{}); err != nil {
		t.Fatal(err)
	}
	if stmt, err := pgParseStatement(source, tokens, parts[1], SQLLimits{}); err == nil || stmt != nil {
		t.Fatal("procedural statement was accepted")
	}
	s, err := pgParseStatement(source, tokens, parts[2], SQLLimits{})
	if err != nil {
		t.Fatal(err)
	}
	if s.Select.Columns[0].Expr.Kind != "opaque" || s.Select.Columns[3].Expr.Kind != "cast" {
		t.Fatal("opaque/cast distinction")
	}
}
func TestPGScannerOperatorMunch(t *testing.T) {
	source := "SELECT A*-2, A@-2, A+/*keep*/2, A<~>B, A!=B, A--keep\n+2, A&-2, A||B FROM T"
	tokens, _, err := pgScan(source, SQLLimits{})
	if err != nil {
		t.Fatal(err)
	}
	var ops []string
	for _, token := range tokens {
		if token.Kind == pgSymbol && token.Text != "," {
			ops = append(ops, token.Text)
		}
	}
	want := []string{"*", "-", "@-", "+", "<~>", "!=", "+", "&-", "||"}
	if !reflect.DeepEqual(ops, want) {
		t.Fatalf("ops=%v want=%v", ops, want)
	}
	if significantPG(tokens)[1].Text != "a" {
		t.Fatal("unquoted ASCII folding")
	}
}
func TestPGScannerRejectsInvalidStreams(t *testing.T) {
	cases := []struct{ sql, code string }{
		{"SELECT '\x00' FROM t", "invalid_encoding"}, {"SELECT '\xff' FROM t", "invalid_encoding"},
		{"SELECT 'unterminated", "unterminated_quote"}, {"SELECT E'abc\\'", "unterminated_quote"}, {"/* x /* y */", "unterminated_comment"},
		{"SELECT $a$body$b$", "unterminated_dollar_quote"}, {"SELECT $bad-tag$body$bad-tag$", "invalid_dollar_quote"},
		{"SELECT 1e+ FROM t", "invalid_number"}, {"SELECT 1_000 FROM t", "invalid_number"}, {"SELECT 0x10 FROM t", "invalid_number"},
		{"SELECT $1x FROM t", "invalid_parameter"}, {"\\copy t FROM x", "invalid_character"}, {"COPY t FROM STDIN;\n1\n\\.", "copy_unsupported"},
		{"SELECT (1] FROM t", "unmatched_delimiter"}, {"SELECT (1; SELECT 2)", "unmatched_delimiter"}, {"SELECT (1", "unmatched_delimiter"},
		{"SELECT \"\" FROM t", "identifier_length"}, {"SELECT " + strings.Repeat("a", 64) + " FROM t", "identifier_length"},
	}
	for _, tt := range cases {
		t.Run(tt.code, func(t *testing.T) {
			tokens, parts, err := pgScan(tt.sql, SQLLimits{})
			var pe *SQLParseError
			if !errors.As(err, &pe) || pe.Code != tt.code || tokens != nil || parts != nil {
				t.Fatalf("result=%v %v %v", tokens, parts, err)
			}
			if strings.Contains(err.Error(), "STDIN") || strings.Contains(err.Error(), "unterminated_quote at bytes") && pe.Code != "unterminated_quote" {
				t.Fatal("unsafe error")
			}
		})
	}
}
func TestPGScannerAndParserLimits(t *testing.T) {
	checks := []struct {
		source string
		set    func(*SQLLimits)
		code   string
	}{
		{"SELECT a FROM t", func(l *SQLLimits) { l.MaxBytes = 5 }, "input_limit"},
		{"SELECT a FROM t", func(l *SQLLimits) { l.MaxTokens = 3 }, "token_limit"},
		{"SELECT a FROM t;SELECT b FROM t", func(l *SQLLimits) { l.MaxStatements = 1 }, "statement_limit"},
		{"SELECT '1234' FROM t", func(l *SQLLimits) { l.MaxLiteralBytes = 4 }, "literal_limit"},
		{"SELECT (((1))) FROM t", func(l *SQLLimits) { l.MaxDepth = 2 }, "depth_limit"},
		{"/* /* /* x */ */ */", func(l *SQLLimits) { l.MaxDepth = 2 }, "depth_limit"},
	}
	for _, tt := range checks {
		l := DefaultSQLLimits()
		tt.set(&l)
		_, _, err := pgScan(tt.source, l)
		var pe *SQLParseError
		if !errors.As(err, &pe) || pe.Code != tt.code {
			t.Fatalf("expected %s got %v", tt.code, err)
		}
	}
	for _, source := range []string{"SELECT " + strings.Repeat("1+", 64) + "1 FROM t", "SELECT " + strings.Repeat("not ", 65) + "true FROM t"} {
		tokens, parts, err := pgScan(source, SQLLimits{})
		if err != nil {
			t.Fatal(err)
		}
		stmt, err := pgParseStatement(source, tokens, parts[0], SQLLimits{})
		var pe *SQLParseError
		if !errors.As(err, &pe) || pe.Code != "depth_limit" || stmt != nil {
			t.Fatalf("depth: %v", err)
		}
	}
	if _, _, err := pgScan("", SQLLimits{MaxBytes: 1}); err == nil {
		t.Fatal("partial custom limits accepted")
	}
}
func exprPGShape(e *pgExpr) string {
	if e == nil {
		return "nil"
	}
	switch e.Kind {
	case "binary":
		return "(" + exprPGShape(e.Left) + e.Op.Text + exprPGShape(e.Right) + ")"
	case "unary":
		return "(" + e.Op.Text + exprPGShape(e.Left) + ")"
	case "paren":
		return "[" + exprPGShape(e.Left) + "]"
	case "cast":
		return "cast(" + exprPGShape(e.Left) + ")"
	case "column":
		return e.Name[len(e.Name)-1].Text
	default:
		return e.Token.Text
	}
}
func TestPGParserPrecedenceAndSpan(t *testing.T) {
	cases := []struct{ expr, want string }{
		{"flags & 3 = 1", "((flags&3)=1)"},
		{"a + b <~> c * d", "((a+b)<~>(c*d))"},
		{"a | b & c << d", "(((a|b)&c)<<d)"},
		{"~a + b", "(~(a+b))"},
		{"-a ^ b ^ c", "(((-a)^b)^c)"},
		{"not a = b and c or d", "(((not(a=b))andc)ord)"},
		{"a::real[] <-> $1::real[]", "(cast(a)<->cast($1))"},
		{"a is not null", "(ais notnull)"},
		{"(a = b) = true", "([(a=b)]=true)"},
	}
	for _, tt := range cases {
		source := "-- prefix\nSELECT " + tt.expr + " AS result FROM public.docs AS d;"
		s := parsePGTest(t, source)
		e := s.Select.Columns[0].Expr
		if got := exprPGShape(e); got != tt.want {
			t.Errorf("%s => %s want %s", tt.expr, got, tt.want)
		}
		if source[e.Span.Start:e.Span.End] != tt.expr {
			t.Fatalf("expression span %+v", e.Span)
		}
	}
}
func TestPGParserSupportedStatements(t *testing.T) {
	s := parsePGTest(t, `SELECT d.id, 'text <~>' AS note FROM "public"."Docs" d WHERE d.flag & 3 = 1 ORDER BY d.embedding <~> ARRAY[1,-2.5,3e-4] DESC NULLS /*stay*/ FIRST, d.id ASC LIMIT $2;`)
	q := s.Select
	if q.Alias.Text != "d" || q.From[1].Text != "Docs" || !q.From[1].Quoted || q.OrderBy[0].Nulls != "first" || !q.OrderBy[0].Desc || !q.OrderBy[0].DirectionExplicit {
		t.Fatal("SELECT fields")
	}
	if q.OrderBy[0].NullsSpan.Start == q.OrderBy[0].NullsSpan.End || q.Limit.Token.Text != "$2" {
		t.Fatal("SELECT spans/limit")
	}
	ins := parsePGTest(t, `INSERT INTO public.docs (id, embedding) VALUES (1, ARRAY[1,2,3]), (2, '{4,5,6}'::real[]);`).Insert
	if len(ins.Rows) != 2 || ins.Rows[1][1].Kind != "cast" || ins.Rows[1][1].Type.ArrayDims != 1 {
		t.Fatal("INSERT fields")
	}
	upd := parsePGTest(t, `UPDATE public.docs AS d SET embedding = CAST($1 AS real[]), flag = 2 WHERE d.id = $2;`).Update
	if upd.Alias.Text != "d" || len(upd.Assignments) != 2 || upd.Assignments[0].Expr.Kind != "cast" {
		t.Fatal("UPDATE fields")
	}
	ct := parsePGTest(t, `CREATE TABLE public.docs (id bigint PRIMARY KEY, embedding REAL[] NOT NULL DEFAULT ARRAY[1,2,3], weight double precision, note text DEFAULT 'untouched');`).CreateTable
	if ct.Columns[1].Type.Name[0].Text != "real" || !ct.Columns[1].NotNull || ct.Columns[2].Type.Name[1].Text != "precision" {
		t.Fatal("CREATE fields")
	}
	ci := parsePGTest(t, `CREATE INDEX docs_idx ON public.docs USING hnsw (embedding public.ann_manhattan_ops) WITH (dims = 3, m = 8, efconstruction = 16, efsearch = 20);`).CreateIndex
	if len(ci.OpClass) != 2 || ci.Method.Text != "hnsw" || ci.Options[3].Value.Text != "20" || ci.BodySpan != ci.Span {
		t.Fatal("INDEX fields")
	}
}
func TestPGParserRejectsWholeStatement(t *testing.T) {
	cases := []string{
		"SELECT id FROM a JOIN b ON a.id=b.id", "WITH q AS (SELECT id FROM a) SELECT id FROM q", "SELECT id FROM t WHERE id IN (1,2)",
		"SELECT id FROM t OFFSET 1", "SELECT id FROM t ORDER BY id USING >", "SELECT id FROM t LIMIT 1; SELECT x FROM t",
		"SELECT a = b = c FROM t", "SELECT a is null is null FROM t", "SELECT a is distinct from b FROM t",
		"SELECT ARRAY[[1,2],[3,4]] FROM t", "SELECT ARRAY[ARRAY[1]] FROM t", "SELECT a[1] FROM t", "SELECT '{1}'::real[3] FROM t",
		"SELECT $0 FROM t", "SELECT $999999999999999999999 FROM t", "SELECT U&\"name\" FROM t AS U&\"alias\"", "SELECT 'a'\n'b' FROM t",
		"INSERT INTO t VALUES (1)", "INSERT INTO t(a,b) VALUES(1)", "INSERT INTO t(a) VALUES(1) RETURNING a", "UPDATE t SET a=1 FROM x",
		"CREATE TABLE t(a real[] UNIQUE)", "CREATE TABLE t(a integer, PRIMARY KEY(a))", "CREATE TABLE t(a integer DEFAULT 1 DEFAULT 2)",
		"CREATE UNIQUE INDEX x ON t USING hnsw(v)", "CREATE INDEX x ON t USING hnsw((v))", "CREATE INDEX x ON t USING hnsw(v) WHERE id=1", "CREATE INDEX x ON t USING hnsw(v) WITH(m=8.5)", "CREATE INDEX x ON t USING hnsw(v) WITH(m=8,m=9)",
	}
	for _, source := range cases {
		tokens, parts, err := pgScan(source, SQLLimits{})
		if err != nil {
			continue
		}
		if len(parts) != 1 {
			continue
		}
		stmt, err := pgParseStatement(source, tokens, parts[0], SQLLimits{})
		var pe *SQLParseError
		if !errors.As(err, &pe) || stmt != nil {
			t.Errorf("accepted %q: %+v err=%v", source, stmt, err)
		}
	}
}
func FuzzPGScannerParser(f *testing.F) {
	for _, source := range []string{"SELECT v <~> ARRAY[1,2] FROM t;", "SELECT $$;<~>$$ FROM t;", "/* /* x */ */ SELECT E'a\\'' FROM t", "SELECT flags & 3 = 1 FROM t", "CREATE INDEX idx ON t USING hnsw(v ann_l2_ops) WITH(m=8)"} {
		f.Add(source)
	}
	f.Fuzz(func(t *testing.T, source string) {
		if len(source) > MaxConvertSQLBytes+1 {
			t.Skip()
		}
		tokens, parts, err := pgScan(source, SQLLimits{})
		if err != nil {
			if tokens != nil || parts != nil {
				t.Fatal("partial failed scan")
			}
			return
		}
		end := 0
		for _, token := range tokens {
			if token.Start != end || token.End < token.Start || token.End > len(source) {
				t.Fatal("non-lossless spans")
			}
			end = token.End
		}
		if end != len(source) {
			t.Fatal("truncated source")
		}
		for _, part := range parts {
			stmt, err := pgParseStatement(source, tokens, part, SQLLimits{})
			if err != nil && stmt != nil {
				t.Fatal("partial failed AST")
			}
			if err == nil {
				if stmt == nil || stmt.Span != part.Span {
					t.Fatal("statement span")
				}
			}
		}
	})
}
