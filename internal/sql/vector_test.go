package sql

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

func TestVectorDeclarations(t *testing.T) {
	for _, tt := range []struct {
		declared string
		n        int
	}{
		{"VECTOR(1)", 1}, {"VECTOR(3)", 3}, {"vector(16000)", 16000},
		{"vEcToR ( 0003 )", 3}, {"VECTOR\n(\t12\r)", 12},
	} {
		s, err := Parse("CREATE TABLE items(id INTEGER PRIMARY KEY, embedding " + tt.declared + " NOT NULL DEFAULT '[1,2,3]')")
		if err != nil {
			t.Fatalf("%q: %v", tt.declared, err)
		}
		column := s.(*CreateTable).Columns[1]
		want := "VECTOR(" + strconv.Itoa(tt.n) + ")"
		if column.Type != want || !column.NotNull || column.Default == nil {
			t.Fatalf("%q: %#v, want type %s", tt.declared, column, want)
		}
		if n, ok := VectorDimensions(column.Type); !ok || n != tt.n {
			t.Fatalf("canonical dimensions %q: %d, %v", column.Type, n, ok)
		}
		if n, ok := VectorDimensions(" \t" + tt.declared + "\r\n"); !ok || n != tt.n {
			t.Fatalf("declared dimensions %q: %d, %v", tt.declared, n, ok)
		}
	}
	for _, bad := range []string{
		"VECTOR", "VECTOR()", "VECTOR(0)", "VECTOR(16001)", "VECTOR(-1)", "VECTOR(+1)",
		"VECTOR(1.0)", "VECTOR(.1)", "VECTOR(1e0)", "VECTOR(0x1)", "VECTOR(1_0)",
		"VECTOR('3')", "VECTOR(?)", "VECTOR(3,4)", "VECTOR(3+1)", "VECTOR(3) TEXT",
		"VECTOR(99999999999999999999999999999999999999)",
	} {
		if _, err := Parse("CREATE TABLE items(embedding " + bad + ")"); err == nil {
			t.Errorf("accepted invalid declaration %q", bad)
		}
	}
	for _, ordinary := range []string{"", "INTEGER", "TEXT", "BLOB", "NUMERIC", "VECTOR", "VECTOR(0)", "VECTOR(16001)", "VECTOR(3)junk", "VECTORISH(3)", "VECTOR(1e0)", "VECTOR(3,4)", "VECTOR(+3)", "VECTOR(3\x00)", strings.Repeat(" ", MaxSQLBytes+1) + "VECTOR(3)"} {
		if n, ok := VectorDimensions(ordinary); ok || n != 0 {
			t.Fatalf("recognized invalid/ordinary declaration %.100q: %d, %v", ordinary, n, ok)
		}
	}
}

func TestVectorSchemaDefaultsStaySQLiteSyntax(t *testing.T) {
	for _, defaultSQL := range []string{"NULL", "'[1,2]'", "X'4753560001010100000000023f80000040000000'"} {
		if _, err := Parse("CREATE TABLE t(v VECTOR(2) DEFAULT " + defaultSQL + ")"); err != nil {
			t.Fatalf("legal SQLite default %s: %v", defaultSQL, err)
		}
	}
	for _, op := range []string{"<->", "<#>", "<=>", "<+>"} {
		for _, expression := range []string{"'[1]' " + op + " '[2]'", "1 + ('[1]' " + op + " '[2]')"} {
			_, err := Parse("CREATE TABLE t(x REAL DEFAULT (" + expression + "))")
			var unsupported *ErrUnsupported
			if !errors.As(err, &unsupported) {
				t.Fatalf("non-SQLite schema expression accepted: %s; %v", expression, err)
			}
		}
	}
	if _, err := Parse("CREATE TABLE t(v VECTOR(2) DEFAULT (vector('[1,2]')))"); err == nil {
		t.Fatal("nonconstant function DEFAULT accepted")
	}
}

func TestVectorOperatorTokens(t *testing.T) {
	in := "a<->b, a<#>b, a<=>b, a<+>b, 1<=2, 1<>2, 1<<2, x->'x', x->>'x', '[1]<#>[2]', \"<=>\" /* <-> */ -- <#>\n"
	tokens, err := Tokenize(in)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, token := range tokens {
		if token.Kind == Symbol && token.Text != "," {
			got = append(got, token.Text)
			if in[token.Start:token.End] != token.Text {
				t.Fatalf("incorrect token span: %+v", token)
			}
		}
	}
	want := []string{"<->", "<#>", "<=>", "<+>", "<=", "<>", "<<", "->", "->>"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tokens %q, want %q", got, want)
	}
	for _, bad := range []string{"'[1]' < - > '[2]'", "'[1]' < # > '[2]'"} {
		if _, err := ParseExpr(bad); err == nil {
			t.Fatalf("accepted unsupported operator %s", bad)
		}
	}
}

func vectorExprShape(e Expr) string {
	switch x := e.(type) {
	case *Binary:
		return "(" + vectorExprShape(x.Left) + " " + x.Op + " " + vectorExprShape(x.Right) + ")"
	case *Unary:
		return "(" + x.Op + " " + vectorExprShape(x.Expr) + ")"
	default:
		return ExprName(e)
	}
}

func TestVectorOperatorPrecedence(t *testing.T) {
	for _, tt := range []struct{ sql, shape string }{
		{"a <-> b + c", "(a <-> (b + c))"},
		{"a + b <-> c", "((a + b) <-> c)"},
		{"a <-> b < c", "((a <-> b) < c)"},
		{"a < b <-> c", "(a < (b <-> c))"},
		{"a <-> b & c", "((a <-> b) & c)"},
		{"a & b <-> c", "((a & b) <-> c)"},
		{"a <-> b <+> c", "((a <-> b) <+> c)"},
		{"a <#> b || c", "(a <#> (b || c))"},
		{"NOT a <=> b = 0", "(NOT ((a <=> b) = 0))"},
		{"a <-> b <= c AND d", "(((a <-> b) <= c) AND d)"},
	} {
		e, err := ParseExpr(tt.sql)
		if err != nil {
			t.Fatal(err)
		}
		if got := vectorExprShape(e); got != tt.shape {
			t.Fatalf("%s: %s, want %s", tt.sql, got, tt.shape)
		}
	}
	for _, tt := range []struct {
		sql  string
		want any
	}{
		{"'[1]' <-> '[3]' < 3 AND 1", int64(1)},
		{"NOT '[1]' <-> '[3]' = 2", int64(0)},
		{"('[1]' <-> '[3]') + 1", float64(3)},
		{"'[1]' <-> '[' || '3]'", float64(2)},
		{"1 < -2", int64(0)}, {"1 <= 2", int64(1)}, {"1 <> 2", int64(1)},
		{"1 + 2 << 1", int64(6)}, {"1 = 2 < 3", int64(1)},
	} {
		if got := evalText(t, tt.sql, nil); got != tt.want {
			t.Fatalf("%s: %v, want %v", tt.sql, got, tt.want)
		}
	}
}

func TestVectorScalarGoldens(t *testing.T) {
	for _, tt := range []struct {
		expr string
		want any
	}{
		{"vector_dims('[1,2,3]')", int64(3)},
		{"VECTOR_DIMS(vector('[1,2,3]'))", int64(3)},
		{"vector_norm('[3,4]')", float64(5)},
		{"vector_norm('[0,0]')", float64(0)},
		{"vector_to_text(' [ +1.0, 2e0 ] ')", "[1,2]"},
		{"vector_to_text(vector('[1,2]'))", "[1,2]"},
		{"typeof(vector('[1,2]'))", "blob"},
		{"length(vector('[1,2]'))", int64(20)},
		{"l2_distance('[1,2]','[4,6]')", float64(5)},
		{"l1_distance('[1,2]','[4,6]')", float64(7)},
		{"cosine_distance('[1,0]','[0,2]')", float64(1)},
		{"cosine_distance('[1,0]','[-2,0]')", float64(2)},
		{"inner_product('[1,2]','[4,6]')", float64(16)},
		{"negative_inner_product('[1,2]','[4,6]')", float64(-16)},
		{"'[1,2]' <-> '[4,6]'", float64(5)},
		{"vector('[1,2]') <+> '[4,6]'", float64(7)},
		{"'[1,0]' <=> vector('[0,2]')", float64(1)},
		{"vector('[1,2]') <#> vector('[4,6]')", float64(-16)},
	} {
		t.Run(tt.expr, func(t *testing.T) {
			if got := evalText(t, tt.expr, nil); got != tt.want {
				t.Fatalf("got %T %v, want %T %v", got, got, tt.want, tt.want)
			}
		})
	}
}

func TestVectorValueCodecAndTypes(t *testing.T) {
	source := "[-0,1e-45,3.4028235e38,-3.4028235e38]"
	v, err := VectorValue(source)
	if err != nil {
		t.Fatal(err)
	}
	blob, err := v.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := VectorValue(blob)
	if err != nil {
		t.Fatal(err)
	}
	got := evalText(t, "vector(?)", &EvalContext{Params: []any{blob}}).([]byte)
	if !bytes.Equal(got, blob) {
		t.Fatal("binary scalar changed exact bits")
	}
	viaSQL := evalText(t, "vector(X'"+hex.EncodeToString(blob)+"')", nil).([]byte)
	if !bytes.Equal(viaSQL, blob) {
		t.Fatal("SQL BLOB literal changed exact bits")
	}
	before, err := decoded.MarshalBinary()
	if err != nil || !bytes.Equal(before, blob) || math.Float32bits(decoded.Values()[0]) != 0x80000000 {
		t.Fatal("codec lost float32 bits or signed zero")
	}
	blob[len(blob)-1] ^= 1
	after, _ := decoded.MarshalBinary()
	if !bytes.Equal(before, after) || !bytes.Equal(got, before) {
		t.Fatal("vector or returned BLOB retained caller-owned bytes")
	}
	for _, bad := range []any{nil, true, int64(1), float64(1), math.NaN(), math.Inf(1), []float32{1}, v} {
		value, err := VectorValue(bad)
		if err == nil || value.Valid() {
			t.Fatalf("accepted unsupported type %T", bad)
		}
	}
}

func TestVectorScalarValidationAndNulls(t *testing.T) {
	for name, argc := range map[string]int{"vector": 1, "vector_dims": 1, "vector_norm": 1, "vector_to_text": 1, "l2_distance": 2, "l1_distance": 2, "cosine_distance": 2, "inner_product": 2, "negative_inner_product": 2} {
		if err := ValidateFunction(strings.ToUpper(name), argc); err != nil {
			t.Fatal(err)
		}
		for _, badCount := range []int{0, argc + 1} {
			if _, err := BuiltinFunctions[name](make([]any, badCount)); err == nil {
				t.Fatalf("%s accepted %d arguments", name, badCount)
			}
		}
		for i := 0; i < argc; i++ {
			args := make([]any, argc)
			for j := range args {
				args[j] = "invalid vector"
			}
			args[i] = nil
			if got, err := BuiltinFunctions[name](args); err != nil || got != nil {
				t.Fatalf("%s NULL position %d: %v, %v", name, i, got, err)
			}
		}
	}
	for _, op := range []string{"<->", "<#>", "<=>", "<+>"} {
		for _, expr := range []string{"NULL " + op + " 123", "'bad' " + op + " NULL"} {
			if got := evalText(t, expr, nil); got != nil {
				t.Fatalf("%s = %v", expr, got)
			}
		}
	}
	for _, tt := range []struct {
		expr string
		want error
	}{
		{"vector(1)", vector.ErrInvalid}, {"vector(TRUE)", vector.ErrInvalid},
		{"vector('[]')", vector.ErrDimensions}, {"vector('[1,NaN]')", vector.ErrNonFinite},
		{"vector('[Infinity]')", vector.ErrNonFinite}, {"vector('[1e99]')", vector.ErrNonFinite},
		{"vector('[0x1]')", vector.ErrSyntax}, {"vector('[1_0]')", vector.ErrSyntax},
		{"vector('[1,]')", vector.ErrSyntax}, {"vector('[1]junk')", vector.ErrSyntax},
		{"vector(X'5b315d')", vector.ErrEncoding},
		{"l2_distance('[1]','[1,2]')", vector.ErrDimensions},
		{"'[1]' <#> '[1,2]'", vector.ErrDimensions},
		{"cosine_distance('[0,0]','[1,2]')", vector.ErrZeroNorm},
		{"'[1,2]' <=> '[0,0]'", vector.ErrZeroNorm},
	} {
		e, err := ParseExpr(tt.expr)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Eval(e, nil); !errors.Is(err, tt.want) {
			t.Fatalf("%s: %v, want %v", tt.expr, err, tt.want)
		}
	}
	if _, err := BuiltinFunctions["vector"]([]any{true}); !errors.Is(err, vector.ErrInvalid) {
		t.Fatalf("direct unsupported value: %v", err)
	}
}

func TestVectorScalarBinaryRejectsAndLimits(t *testing.T) {
	valid := evalText(t, "vector('[1,2]')", nil).([]byte)
	badBlobs := [][]byte{nil, {}, []byte("[1,2]"), valid[:len(valid)-1], append(append([]byte(nil), valid...), 0)}
	for _, index := range []int{0, 4, 5, 6, 7} {
		b := append([]byte(nil), valid...)
		b[index]++
		badBlobs = append(badBlobs, b)
	}
	for _, blob := range badBlobs {
		if got, err := BuiltinFunctions["vector"]([]any{blob}); !errors.Is(err, vector.ErrEncoding) || got != nil {
			t.Fatalf("invalid binary accepted: %x, %v", blob, err)
		}
	}
	for _, bad := range []struct {
		value any
		want  error
	}{
		{strings.Repeat(" ", vector.MaxTextBytes+1), vector.ErrLimit},
		{"[" + strings.Repeat("0,", vector.MaxDimensions) + "0]", vector.ErrDimensions},
		{"[" + strings.Repeat("0", vector.MaxElementTextBytes+1) + "]", vector.ErrLimit},
		{make([]byte, vector.MaxBinaryBytes+1), vector.ErrEncoding},
	} {
		if _, err := BuiltinFunctions["vector"]([]any{bad.value}); !errors.Is(err, bad.want) {
			t.Fatalf("resource boundary: %v, want %v", err, bad.want)
		}
	}
	maxVector := "[" + strings.Repeat("0,", vector.MaxDimensions-1) + "0]"
	if got := evalText(t, "vector_dims(?)", &EvalContext{Params: []any{maxVector}}); got != int64(vector.MaxDimensions) {
		t.Fatalf("maximum dimensions: %v", got)
	}
}

func TestVectorScalarContextAndOverrides(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, expr := range []string{
		"vector('[1]')", "vector_dims('[1]')", "vector_norm('[1]')", "vector_to_text('[1]')",
		"l2_distance('[1]','[2]')", "l1_distance('[1]','[2]')", "cosine_distance('[1]','[2]')",
		"inner_product('[1]','[2]')", "negative_inner_product('[1]','[2]')", "'[1]' <-> '[2]'",
		"'[1]' <#> '[2]'", "'[1]' <=> '[2]'", "'[1]' <+> '[2]'", "NULL <-> NULL", "vector(NULL)",
	} {
		e, err := ParseExpr(expr)
		if err != nil {
			t.Fatal(err)
		}
		if got, err := Eval(e, &EvalContext{Context: ctx}); !errors.Is(err, context.Canceled) || got != nil {
			t.Fatalf("%s ignored cancellation: %v, %v", expr, got, err)
		}
	}
	for _, expr := range []string{"vector(cancel_input())", "cancel_input() <-> '[2]'"} {
		ctx, cancel := context.WithCancel(context.Background())
		e, _ := ParseExpr(expr)
		got, err := Eval(e, &EvalContext{Context: ctx, Functions: map[string]Function{
			"cancel_input": func([]any) (any, error) { cancel(); return "[1]", nil },
		}})
		cancel()
		if !errors.Is(err, context.Canceled) || got != nil {
			t.Fatalf("cancellation after argument evaluation: %v, %v", got, err)
		}
	}
	if got := evalText(t, "vector_dims('[1]')", &EvalContext{Functions: map[string]Function{
		"vector_dims": func([]any) (any, error) { return int64(7), nil },
	}}); got != int64(7) {
		t.Fatal("connection function override lost", got)
	}
}

// These are published pgvector v0.8.7 examples, not a live PostgreSQL run. The
// pinned source/hash checks live in extension/vector; this test verifies that
// the SQL surface reaches the same semantics through text and binary operands.
func TestVectorSQLPublishedPGVectorGoldens(t *testing.T) {
	data, err := os.ReadFile("../../testdata/pgvector-v0.8.7/golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixtures struct {
		Parse []struct {
			Input           string
			Upstream, Error *string
		}
		Metrics []struct {
			Operation, A       string
			B, Upstream, Error *string
			LocalPolicy        string `json:"local_policy"`
		}
	}
	if err := json.Unmarshal(data, &fixtures); err != nil {
		t.Fatal(err)
	}
	if len(fixtures.Parse) != 30 || len(fixtures.Metrics) != 31 {
		t.Fatal("published fixture coverage changed without review")
	}
	for _, f := range fixtures.Parse {
		expr, _ := ParseExpr("vector(?)")
		got, err := Eval(expr, &EvalContext{Params: []any{f.Input}})
		if f.Error != nil {
			if err == nil || got != nil {
				t.Fatalf("accepted invalid published text %q", f.Input)
			}
			continue
		}
		if err != nil || f.Upstream == nil {
			t.Fatalf("published parse %q: %v", f.Input, err)
		}
		want, err := vector.Parse(*f.Upstream)
		if err != nil {
			t.Fatal(err)
		}
		encoded, _ := want.MarshalBinary()
		if !bytes.Equal(got.([]byte), encoded) {
			t.Fatalf("published float32 bits differ for %q", f.Input)
		}
	}
	operator := map[string]string{"l2_distance": "<->", "l1_distance": "<+>", "cosine_distance": "<=>", "negative_inner_product": "<#>"}
	for _, f := range fixtures.Metrics {
		query := f.Operation + "(?)"
		queries := []string{}
		params := []any{f.A}
		if f.B != nil {
			query = f.Operation + "(?,?)"
			params = append(params, *f.B)
			if op := operator[f.Operation]; op != "" {
				queries = append(queries, "? "+op+" ?")
			}
		}
		queries = append(queries, query)
		for _, query := range queries {
			for _, binary := range []bool{false, true} {
				args := append([]any(nil), params...)
				if binary {
					for i, value := range args {
						v, err := vector.Parse(value.(string))
						if err != nil {
							t.Fatal(err)
						}
						args[i], _ = v.MarshalBinary()
					}
				}
				expr, err := ParseExpr(query)
				if err != nil {
					t.Fatal(err)
				}
				result, err := Eval(expr, &EvalContext{Params: args})
				if f.Error != nil || f.LocalPolicy == "ErrZeroNorm" {
					want := vector.ErrDimensions
					if f.LocalPolicy == "ErrZeroNorm" {
						want = vector.ErrZeroNorm
					}
					if !errors.Is(err, want) || result != nil {
						t.Fatalf("%s: %v, %v; want %v", query, result, err, want)
					}
					continue
				}
				if err != nil {
					t.Fatalf("%s: %v", query, err)
				}
				got := result.(float64)
				var want float64
				if f.LocalPolicy == "finite_float64" {
					// Published overflow examples contain one component. Widen
					// that float32 before independently computing the reference.
					a, _ := vector.Parse(f.A)
					b, _ := vector.Parse(*f.B)
					if a.Dims() != 1 || b.Dims() != 1 {
						t.Fatal("finite reference fixture changed")
					}
					x, y := float64(a.Values()[0]), float64(b.Values()[0])
					switch f.Operation {
					case "l2_distance", "l1_distance":
						want = math.Abs(x - y)
					case "inner_product":
						want = x * y
					case "cosine_distance":
						want = 1 - (x*y)/math.Abs(x*y)
					default:
						t.Fatal("unknown finite reference operation")
					}
				} else {
					if f.Upstream == nil {
						t.Fatal("published metric missing expected value")
					}
					want, err = strconv.ParseFloat(*f.Upstream, 64)
					if err != nil {
						t.Fatal(err)
					}
				}
				if math.IsNaN(got) || math.IsInf(got, 0) || math.Abs(got-want) > 1e-7+1e-7*math.Abs(want) {
					t.Fatalf("%s binary=%v: %.17g, want %.17g", query, binary, got, want)
				}
			}
		}
	}
}
