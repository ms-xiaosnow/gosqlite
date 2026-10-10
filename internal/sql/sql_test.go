package sql

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

func evalText(t *testing.T, s string, ctx *EvalContext) any {
	t.Helper()
	e, err := ParseExpr(s)
	if err != nil {
		t.Fatalf("parse %s: %v", s, err)
	}
	v, err := Eval(e, ctx)
	if err != nil {
		t.Fatalf("eval %s: %v", s, err)
	}
	return v
}
func TestLexer(t *testing.T) {
	in := "\ufeff/* comment */SELECT \"a\"\"b\", [中 文], `tick``name`, 'it''s', X'00ff', 1_024, .5e+2, ?12, :name, @abc, $arg -- fin"
	ts, err := Tokenize(in)
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, tok := range ts {
		if tok.Kind != EOF {
			got = append(got, tok.Text)
			if tok.Start < 0 || tok.End > len(in) || tok.End <= tok.Start {
				t.Fatal("invalid span")
			}
		}
	}
	want := []string{"SELECT", `a"b`, ",", "中 文", ",", "tick`name", ",", "it's", ",", "00ff", ",", "1_024", ",", ".5e+2", ",", "?12", ",", ":name", ",", "@abc", ",", "$arg"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("%q != %q", got, want)
	}
	for _, bad := range []string{"SELECT '\x00'", "/* unfinished", "'unfinished", "X'1'", "1e+", "12abc", "1__2", "1_.2", "?0!", string([]byte{0xff})} {
		if _, err := Tokenize(bad); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}
func TestParseSelect(t *testing.T) {
	s, err := Parse("-- hello\n SELECT t.*, (1 + 2) AS value, 'FROM' literal FROM \"items\" AS t WHERE t.a NOT LIKE 'x!_%' ESCAPE '!' AND t.id >= ?2 ORDER BY 2 DESC, t.a ASC LIMIT 10 OFFSET 3;")
	if err != nil {
		t.Fatal(err)
	}
	q := s.(*Select)
	if q.From != "items" || q.Alias != "t" || q.ParameterCount() != 2 || len(q.Columns) != 3 || q.Columns[1].Text != "(1 + 2)" || q.Columns[2].Text != "'FROM'" {
		t.Fatalf("%#v", q)
	}
	if _, ok := q.Columns[0].Expr.(*Star); !ok {
		t.Fatal("missing qualified star")
	}
	if len(q.OrderBy) != 2 || !q.OrderBy[0].Desc {
		t.Fatal("order terms")
	}
	s, err = Parse("SELECT ?2, ?, :name, :name, @name, ?1, $n")
	if err != nil {
		t.Fatal(err)
	}
	if s.ParameterCount() != 6 {
		t.Fatal(s.ParameterCount())
	}
	params := s.Parameters()
	if len(params) != 6 || params[3].Name != ":name" || params[4].Name != "@name" {
		t.Fatalf("%+v", params)
	}
}
func TestParseCreateAndDML(t *testing.T) {
	s, err := Parse("CREATE TABLE IF NOT EXISTS docs(id INTEGER PRIMARY KEY, title TEXT NOT NULL DEFAULT 'untitled', weight REAL DEFAULT (-2.5), body BLOB, empty)")
	if err != nil {
		t.Fatal(err)
	}
	c := s.(*CreateTable)
	if !c.IfNotExists || !c.Columns[0].PrimaryKey || !c.Columns[1].NotNull || c.Columns[4].Type != "" {
		t.Fatalf("%+v", c)
	}
	if got := evalText(t, "-9223372036854775808", nil); got != int64(math.MinInt64) {
		t.Fatalf("min integer = %T %v", got, got)
	}
	for _, s := range []string{"CREATE INDEX ix ON docs(title,id DESC)", "INSERT INTO docs(id,title) VALUES (1,'a'),(2,?)", "INSERT INTO docs DEFAULT VALUES", "UPDATE docs SET title = 'new', weight = weight + 1 WHERE id = 1", "DELETE FROM docs WHERE id IS NULL", "BEGIN IMMEDIATE TRANSACTION", "COMMIT", "END TRANSACTION", "ROLLBACK TRANSACTION"} {
		if _, err := Parse(s); err != nil {
			t.Errorf("%s: %v", s, err)
		}
	}
}
func TestRejectUnsupported(t *testing.T) {
	for _, s := range []string{
		"SELECT 1; SELECT 2", "SELECT DISTINCT a FROM t", "SELECT a FROM t JOIN u ON t.x=u.x", "SELECT count(*) FROM t", "SELECT sum(x) FROM t", "SELECT (SELECT 1)", "SELECT x COLLATE NOCASE FROM t", "SELECT x FROM t GROUP BY x", "SELECT x FROM t UNION SELECT x FROM u", "SELECT x IN (1,2) FROM t", "SELECT x BETWEEN 1 AND 2 FROM t", "SELECT CASE WHEN 1 THEN 1 END", "SELECT 1 'AND' 2", "SELECT a NOT \"LIKE\" 'x'", "SELECT 1 FROM main.t", "CREATE TABLE t(x INTEGER PRIMARY KEY DESC)", "CREATE TABLE t(x INTEGER PRIMARY KEY AUTOINCREMENT)", "CREATE TABLE t(x TEXT UNIQUE)", "CREATE TABLE t(x TEXT COLLATE NOCASE)", "CREATE TABLE t(x INTEGER, PRIMARY KEY(x))", "CREATE TABLE t(x BOGUS)", "CREATE TABLE t(x TEXT) STRICT", "CREATE TABLE t(x TEXT DEFAULT ?)", "CREATE TABLE t(x INTEGER DEFAULT abs(2))", "CREATE UNIQUE INDEX i ON t(x)", "CREATE INDEX i ON t(lower(x))", "CREATE INDEX i ON t(x) WHERE x>0", "INSERT INTO t VALUES(1),(1,2)", "INSERT OR REPLACE INTO t VALUES(1)", "UPDATE t SET a=1 RETURNING a", "DELETE FROM t RETURNING *", "SAVEPOINT a", "ROLLBACK TO a", "SELECT ?0", "SELECT ?32767", "SELECT -0x8000000000000000", "SELECT -(+0x8000000000000000)",
	} {
		if _, err := Parse(s); err == nil {
			t.Errorf("accepted unsupported: %s", s)
		}
	}
	_, err := Parse("SELECT sum(x) FROM t")
	var unsupported *ErrUnsupported
	if !errors.As(err, &unsupported) {
		t.Fatalf("expected unsupported, got %v", err)
	}
}
func TestResourceBounds(t *testing.T) {
	for _, s := range []string{"SELECT " + strings.Repeat("(", 129) + "1" + strings.Repeat(")", 129), "SELECT 1" + strings.Repeat("+1", 128), strings.Repeat(" ", MaxSQLBytes+1), "SELECT " + strings.Repeat("NOT ", 129) + "1", "SELECT coalesce(" + strings.Repeat("1,", 1000) + "1)", "SELECT " + strings.Repeat("1,", 2000) + "1", strings.Repeat(";", MaxTokens+1)} {
		if _, err := Parse(s); err == nil {
			t.Fatal("accepted out-of-bounds SQL")
		}
	}
}
func TestValuesAndThreeValuedLogic(t *testing.T) {
	cases := []struct {
		s    string
		want any
	}{
		{"NULL AND 0", int64(0)}, {"NULL AND 1", nil}, {"NULL OR 1", int64(1)}, {"NULL OR 0", nil}, {"NOT NULL", nil}, {"NULL = NULL", nil}, {"NULL IS NULL", int64(1)}, {"NULL IS FALSE", int64(0)}, {"2 IS TRUE", int64(1)}, {"2 IS NOT TRUE", int64(0)}, {"2 IS +TRUE", int64(0)},
		{"NOT 1=2", int64(1)}, {"1=2<3", int64(1)}, {"1+2*3", int64(7)}, {"5/2", int64(2)}, {"5.0/2", 2.5}, {"1/0", nil}, {"5.0%2", float64(1)}, {"9223372036854775807+1", float64(9223372036854775808.0)}, {"-9223372036854775808 / -1", float64(9223372036854775808.0)},
		{"'123junk'+1", int64(124)}, {"'junk'+1", int64(1)}, {"'1.0'+0", float64(1)}, {"'1e0'+0", float64(1)}, {"1='1'", int64(0)}, {"0xFFFFFFFFFFFFFFFF", int64(-1)}, {"-8 >> 2", int64(-2)}, {"-8 >> 200", int64(-1)}, {"1 << -1", int64(0)}, {"1e999 - 1e999", nil},
		{"'ab'||'中'", "ab中"}, {"X'0061'", []byte{0, 97}}, {"'A' LIKE 'a'", int64(1)}, {"'Æ' LIKE 'æ'", int64(0)}, {"'数据库' LIKE '数_库'", int64(1)}, {"'a_b' LIKE 'a!_b' ESCAPE '!'", int64(1)}, {"'a%b' LIKE 'a中%b' ESCAPE '中'", int64(1)},
		{"coalesce(NULL,3,4)", int64(3)}, {"ifnull(NULL,'yes')", "yes"}, {"typeof(1.0)", "real"}, {"length('数据库')", int64(3)}, {"length(X'00ff')", int64(2)}, {"lower('AÉ')", "aÉ"}, {"upper('aé')", "Aé"}, {"nullif(1,1)", nil}, {"hex(X'0061')", "0061"}, {"abs(-4)", int64(4)}, {"abs('12')", float64(12)}, {"abs('abc')", float64(0)}, {"'1e2'%7", float64(1)}, {"'1e2'|0", int64(1)}, {"~'1e2'", int64(-2)},
	}
	for _, c := range cases {
		t.Run(c.s, func(t *testing.T) {
			v := evalText(t, c.s, nil)
			if !reflect.DeepEqual(v, c.want) {
				t.Fatalf("got %T(%v), want %T(%v)", v, v, c.want, c.want)
			}
		})
	}
}
func TestAffinityAndIntegerComparison(t *testing.T) {
	ctx := &EvalContext{Row: map[string]any{"i": int64(12), "t": "12", "r": float64(12)}, Affinity: func(_, n string) string {
		switch n {
		case "i":
			return "INTEGER"
		case "t":
			return "TEXT"
		case "r":
			return "REAL"
		}
		return "BLOB"
	}}
	for _, s := range []string{"i='12'", "t=12", "r='12'", "+i != '12'", "i=t"} {
		if got := evalText(t, s, ctx); got != int64(1) {
			t.Errorf("%s = %v", s, got)
		}
	}
	for _, pair := range []struct {
		a, b any
		want int
	}{{int64(math.MaxInt64), float64(9223372036854775808.0), -1}, {int64(9007199254740993), float64(9007199254740992), 1}, {int64(math.MinInt64), float64(math.MinInt64), 0}, {nil, int64(1), -1}, {"a", []byte("a"), -1}} {
		got, err := Compare(pair.a, pair.b)
		if err != nil || got != pair.want {
			t.Fatalf("Compare(%v,%v)=%d,%v", pair.a, pair.b, got, err)
		}
	}
	if EqualName("É", "é") || !EqualName("AÉ", "aÉ") {
		t.Fatal("incorrect identifier folding")
	}
}
func TestFunctionsAndBindings(t *testing.T) {
	ctx := &EvalContext{Params: []any{int64(5)}, Functions: map[string]Function{"twice": func(a []any) (any, error) { return a[0].(int64) * 2, nil }}}
	if got := evalText(t, "twice(?1)", ctx); got != int64(10) {
		t.Fatal(got)
	}
	for _, s := range []string{"?2", "missing(1)", "length()", "abs(-9223372036854775808)", "'a' LIKE 'a' ESCAPE 'xx'"} {
		e, err := ParseExpr(s)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = Eval(e, ctx); err == nil {
			t.Errorf("expected eval error for %s", s)
		}
	}
	for _, s := range []string{"coalesce(NULL,1)", "ifnull(2,3)"} {
		e, _ := ParseExpr(s)
		if err := Walk(e, func(e Expr) error {
			if c, ok := e.(*Call); ok {
				return ValidateFunction(c.Name, len(c.Args))
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
}
func FuzzParseNoPanic(f *testing.F) {
	for _, s := range []string{"SELECT 1", "SELECT ? FROM t WHERE a LIKE '中%'", "CREATE TABLE t(id INTEGER PRIMARY KEY)", "SELECT (((1)))"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 8192 {
			return
		}
		_, _ = Parse(s)
	})
}

func TestInlineNonIntegerPrimaryKeySchema(t *testing.T) {
	s, err := Parse("CREATE TABLE noninteger(id INT PRIMARY KEY, value TEXT)")
	if err != nil {
		t.Fatal(err)
	}
	c := s.(*CreateTable).Columns[0]
	if !c.PrimaryKey || c.Type != "INT" {
		t.Fatalf("%+v", c)
	}
}
func TestInvalidUTF8TextOperations(t *testing.T) {
	ctx := &EvalContext{Params: []any{string([]byte{0xff})}}
	for _, s := range []string{"length(?)", "? LIKE '_'"} {
		e, err := ParseExpr(s)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = Eval(e, ctx); err == nil {
			t.Errorf("accepted invalid UTF-8 in %s", s)
		}
	}
	if got := evalText(t, "?=?", &EvalContext{Params: []any{string([]byte{0xff}), string([]byte{0xff})}}); got != int64(1) {
		t.Fatal("raw byte equality changed")
	}
}

func TestRealTextBaseline(t *testing.T) {
	cases := []struct {
		value float64
		want  string
	}{
		{0.1, "0.1"}, {0.3, "0.3"}, {math.SmallestNonzeroFloat64, "4.9406564584124654e-324"}, {1e-320, "9.9998886718268301e-321"}, {math.MaxFloat64, "1.7976931348623157e+308"}, {1e16, "10000000000000000.0"}, {1e17, "1.0e+17"}, {111111111111111.125, "111111111111111.13"}, {-6.464007607046158e-91, "-6.464007607046158e-91"}, {-9.074019899851194e83, "-9.0740198998511939e+83"},
	}
	for _, c := range cases {
		if got := realText(c.value); got != c.want {
			t.Errorf("%g: %s != %s", c.value, got, c.want)
		}
	}
}

func TestHexadecimalNumericSeparators(t *testing.T) {
	if got := evalText(t, "0xFE_EF", nil); got != int64(0xfeef) {
		t.Fatal(got)
	}
}
func TestLikeEscapeNULAndLimit(t *testing.T) {
	for _, ctx := range []*EvalContext{{Params: []any{"\x00"}}, {Params: []any{strings.Repeat("a", MaxLikePatternBytes+1)}}} {
		s := "'a' LIKE 'a' ESCAPE ?"
		if len(ctx.Params[0].(string)) > 1 {
			s = "'a' LIKE ?"
		}
		e, err := ParseExpr(s)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = Eval(e, ctx); err == nil {
			t.Fatal("expected LIKE error")
		}
	}
}
