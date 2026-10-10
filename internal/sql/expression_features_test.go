package sql

import (
	"context"
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
)

// Ordinary memory-only scalar/parser tests: no database or native I/O faults.
func TestExpressionFeaturesValues(t *testing.T) {
	cases := []struct {
		sql  string
		want any
	}{
		{"CASE WHEN NULL THEN 1 WHEN 0 THEN 2 WHEN '1x' THEN 3 ELSE 4 END", int64(3)},
		{"CASE 2 WHEN '2' THEN 1 WHEN 2 THEN 'yes' ELSE 3 END", "yes"},
		{"CASE NULL WHEN NULL THEN 1 END", nil},
		{"CASE WHEN 0 THEN abs(-9223372036854775808) ELSE 7 END", int64(7)},
		{"CASE 1 WHEN 1 THEN 7 WHEN abs(-9223372036854775808) THEN 0 END", int64(7)},
		{"CAST('123e+5' AS INTEGER)", int64(123)}, {"CAST('123e+5' AS NUMERIC)", int64(12300000)},
		{"CAST('123e+5' AS REAL)", float64(12300000)}, {"CAST('9223372036854775808' AS INTEGER)", int64(math.MaxInt64)},
		{"CAST('-9223372036854775809' AS INTEGER)", int64(math.MinInt64)}, {"CAST('abc' AS NUMERIC)", int64(0)},
		{"CAST(1.0 AS NUMERIC)", float64(1)}, {`CAST('1.5' AS "")`, float64(1.5)}, {"CAST('1.5' AS)", float64(1.5)}, {`CAST('1.5' AS "x" INT)`, float64(1.5)}, {"CAST('1.5' AS weird /*INT*/ BLOB)", int64(1)}, {"CAST('2251799813685248.0' AS NUMERIC)", float64(2251799813685248)},
		{"CAST('-2251799813685248.0' AS NUMERIC)", int64(-2251799813685248)},
		{"CAST(X'610062FF' AS TEXT)", string([]byte{'a', 0, 'b', 255})}, {"CAST('abc' AS BLOB)", []byte("abc")},
		{"CAST('[1,2]' AS VECTOR(2))", int64(0)}, {"CAST('1.5' AS FLOATING POINT)", int64(1)},
		{"2 BETWEEN 1 AND 3", int64(1)}, {"2 NOT BETWEEN 1 AND 3", int64(0)},
		{"2 BETWEEN NULL AND 1", int64(0)}, {"2 NOT BETWEEN NULL AND 1", int64(1)},
		{"2 BETWEEN NULL AND 3", nil}, {"NULL BETWEEN 0 AND 1", nil},
		{"1 BETWEEN 0=0 AND 2", int64(1)}, {"1 BETWEEN 0 AND 0=0", int64(1)},
		{"NOT 1 BETWEEN 0 AND 2", int64(0)}, {"1 BETWEEN 0 AND 2 AND 0", int64(0)},
		{"absent IN ()", int64(0)}, {"NULL IN ()", int64(0)}, {"NULL NOT IN ()", int64(1)}, {"abs(-9223372036854775808) IN ()", int64(0)},
		{"1 IN (2,NULL)", nil}, {"1 NOT IN (2,NULL)", nil}, {"1 IN (NULL,1)", int64(1)},
		{"1 IN ('1',2)", int64(0)}, {"NULL IN (1,2)", nil}, {"2 IN (1,2) BETWEEN 0 AND 2", int64(1)},
		{"CAST(12 AS NUMERIC) = '12'", int64(1)}, {"12 IN (CAST('12' AS TEXT))", int64(0)},
	}
	for _, c := range cases {
		t.Run(c.sql, func(t *testing.T) {
			got := evalText(t, c.sql, nil)
			if !reflect.DeepEqual(got, c.want) {
				t.Fatalf("got %T(%v), want %T(%v)", got, got, c.want, c.want)
			}
		})
	}
}

func TestExpressionFeaturesEvaluationAndCancellation(t *testing.T) {
	calls := 0
	ec := &EvalContext{Functions: map[string]Function{"tick": func([]any) (any, error) { calls++; return int64(2), nil }}}
	for _, q := range []string{"CASE tick() WHEN 1 THEN 0 WHEN 2 THEN 1 ELSE 0 END", "tick() BETWEEN 1 AND 3", "tick() IN (0,1,2,3)"} {
		calls = 0
		if got := evalText(t, q, ec); got != int64(1) || calls != 1 {
			t.Fatalf("%s: %v; calls %d", q, got, calls)
		}
	}
	e, err := ParseExpr("CASE WHEN 1 THEN 7 ELSE tick() END")
	if err != nil {
		t.Fatal(err)
	}
	calls = 0
	v, err := Eval(e, ec)
	if err != nil || v != int64(7) || calls != 0 {
		t.Fatal(v, err, calls)
	}
	ctx, cancel := context.WithCancel(context.Background())
	ec.Context = ctx
	ec.Functions["tick"] = func([]any) (any, error) { cancel(); return int64(0), nil }
	e, err = ParseExpr("1 IN (tick(),2,3)")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Eval(e, ec); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	for _, q := range []string{"CAST(1 AS TEXT)", "CASE WHEN 1 THEN 2 END", "1 IN ()", "1 BETWEEN 0 AND 2"} {
		e, err = ParseExpr(q)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = Eval(e, ec); !errors.Is(err, context.Canceled) {
			t.Fatal(q, err)
		}
	}
}

func TestExpressionFeaturesParserAndWalkers(t *testing.T) {
	s, err := Parse("SELECT ALL CASE ?2 WHEN :x THEN CAST(? AS DECIMAL(10,2)) ELSE ? END AS chosen WHERE ? IN (?, :x) ORDER BY chosen DESC NULLS FIRST, 1 ASC NULLS LAST LIMIT ?")
	if err != nil {
		t.Fatal(err)
	}
	sel := s.(*Select)
	if s.ParameterCount() != 8 || len(s.Parameters()) != 7 || sel.OrderBy[0].Nulls != NullsFirst || sel.OrderBy[1].Nulls != NullsLast {
		t.Fatalf("%+v; %v", sel, s.Parameters())
	}
	names := []string{}
	e, err := ParseExpr("CASE a WHEN b THEN CAST(c AS TEXT) ELSE d BETWEEN e AND f IN(g,h) END")
	if err != nil {
		t.Fatal(err)
	}
	if err = Walk(e, func(e Expr) error {
		if c, ok := e.(*Column); ok {
			names = append(names, c.Name)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(names, []string{"a", "b", "c", "d", "e", "f", "g", "h"}) {
		t.Fatal(names)
	}
	rewritten := Rewrite(e, func(e Expr) Expr {
		if c, ok := e.(*Column); ok {
			return &Column{Name: "new_" + c.Name}
		}
		return e
	})
	if reflect.DeepEqual(e, rewritten) || e.(*Case).Base.(*Column).Name != "a" {
		t.Fatal("rewrite mutated or reused composite")
	}
	for _, q := range []string{"SELECT CASE END", "SELECT CASE WHEN 1 THEN END", "SELECT CASE WHEN 1 THEN 2", "SELECT CAST(1)", "SELECT CAST(1 AS TEXT(1,2,3))", "SELECT 1 BETWEEN 0", "SELECT 1 IN (1,)", "SELECT 1 IN (SELECT 1)", "SELECT 1 ORDER BY 1 NULLS", "SELECT 1 ORDER BY 1 NULLS MIDDLE", "SELECT ALL DISTINCT 1"} {
		if _, err := Parse(q); err == nil {
			t.Fatal("accepted", q)
		}
	}
}

func TestExpressionFeaturesResourceBounds(t *testing.T) {
	for _, q := range []string{
		strings.Repeat("CASE WHEN 1 THEN ", 129) + "0" + strings.Repeat(" END", 129),
		strings.Repeat("CAST(", 129) + "0" + strings.Repeat(" AS TEXT)", 129),
		"0" + strings.Repeat(" IN (0)", 129), "0" + strings.Repeat(" BETWEEN 0 AND 1", 129),
		"0 IN (" + strings.Repeat("0,", MaxTokens) + "0)",
	} {
		if _, err := ParseExpr(q); err == nil {
			t.Fatal("accepted unbounded expression")
		}
	}
	// The evaluator streams a wide list without materializing a set or RHS
	// value slice. Allocation count must not grow with list width.
	e, err := ParseExpr("-1 IN (" + strings.Repeat("0,", 8000) + "0)")
	if err != nil {
		t.Fatal(err)
	}
	ec := &EvalContext{}
	if n := testing.AllocsPerRun(10, func() {
		v, err := Eval(e, ec)
		if err != nil || v != int64(0) {
			panic("wide IN evaluation")
		}
	}); n > 5 {
		t.Fatalf("wide IN allocated %.0f objects", n)
	}
}

func TestExpressionFeaturesTypedExternalTextIdentity(t *testing.T) {
	values := map[string]any{"v": "[3,0,-4]", "h": "[3,0,-4]", "b": "101", "s": "{1:3,3:-4}/3"}
	types := map[string]string{"v": "VECTOR(3)", "h": "HALFVEC(3)", "b": "BIT(3)", "s": "SPARSEVEC(3)"}
	ec := &EvalContext{Row: values, DeclaredType: func(_, n string) string { return types[n] }}
	for _, c := range []struct{ column, kind, fn string }{{"v", "vector", "vector_dims"}, {"h", "halfvec", "halfvec_dims"}, {"b", "bit", "bit_dims"}, {"s", "sparsevec", "sparsevec_dims"}} {
		for _, form := range []string{"CASE WHEN 1 THEN " + c.column + " END", "CASE 1 WHEN 1 THEN CAST(" + c.column + " AS TEXT) END", "CAST(CASE WHEN 1 THEN " + c.column + " END AS TEXT)"} {
			e, err := ParseExpr(form)
			if err != nil {
				t.Fatal(err)
			}
			if kind := VectorExpressionRepresentation(e, ec.DeclaredType); kind != c.kind {
				t.Fatal(form, kind)
			}
			if got := evalText(t, c.fn+"("+form+")", ec); got != int64(3) {
				t.Fatal(got)
			}
			if c.kind != "vector" {
				e, err = ParseExpr("vector_dims(" + form + ")")
				if err != nil {
					t.Fatal(err)
				}
				if _, err = Eval(e, ec); err == nil {
					t.Fatal("identity erased", form)
				}
			}
		}
	}
	for _, form := range []string{"CASE WHEN 1 THEN v ELSE h END", "CASE 1 WHEN 1 THEN s ELSE v END", "CAST(CASE WHEN 1 THEN b ELSE v END AS TEXT)"} {
		e, err := ParseExpr(form)
		if err != nil {
			t.Fatal(err)
		}
		if VectorExpressionRepresentation(e, ec.DeclaredType) != "mixed" {
			t.Fatal("lost mixed identity", form)
		}
	}
}

func TestExpressionFeaturesConstantInEvaluation(t *testing.T) {
	for _, q := range []string{
		"1 IN (1,2,abs(-9223372036854775808))",
		"1 NOT IN (1,2,abs(-9223372036854775808))",
		"1 IN (1,?1,abs(-9223372036854775808))",
	} {
		e, err := ParseExpr(q)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = Eval(e, &EvalContext{Params: []any{int64(2)}}); err == nil || !strings.Contains(err.Error(), "integer overflow") {
			t.Fatal(q, err)
		}
	}
	for _, q := range []string{"1 IN(1,abs(-9223372036854775808))", "1 IN(1,n,abs(-9223372036854775808))", "1 IN(n,2,abs(-9223372036854775808))"} {
		if got := evalText(t, q, &EvalContext{Row: map[string]any{"n": int64(1)}}); got != int64(1) {
			t.Fatal(q, got)
		}
	}
	// A caller-supplied override has no deterministic guarantee, even when
	// its name matches a builtin. It must not become eager constant work.
	calls := 0
	ec := &EvalContext{Functions: map[string]Function{"abs": func([]any) (any, error) { calls++; return nil, errors.New("override") }}}
	if got := evalText(t, "1 IN(1,2,abs(3))", ec); got != int64(1) || calls != 0 {
		t.Fatal(got, calls)
	}
	e, err := ParseExpr("abs(-9223372036854775808) IN (1,2,'a' LIKE 'a' ESCAPE 'xx')")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Eval(e, nil); err == nil || !strings.Contains(err.Error(), "ESCAPE") {
		t.Fatal("constant RHS error precedence", err)
	}
}
