package sql

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// Independent review: scalar ASTs only. No storage, native failures or process
// interruption. The large constant IN cases reproduce SQLite 3.54's observable
// whole-RHS evaluation before membership lookup.
func TestExpressionReviewConstantInErrors(t *testing.T) {
	for _, q := range []string{
		"1 IN (1,2,abs(-9223372036854775808))",
		"1 NOT IN (1,2,abs(-9223372036854775808))",
		"CASE WHEN 1 IN (1,2,abs(-9223372036854775808)) THEN 7 END",
	} {
		e, err := ParseExpr(q)
		if err != nil {
			t.Fatal(err)
		}
		if v, err := Eval(e, nil); err == nil {
			t.Errorf("%s: got %v, want SQLite integer-overflow error", q, v)
		}
	}
	for _, q := range []string{
		"1 IN (1,abs(-9223372036854775808))",
		"CASE WHEN 1 THEN 1 ELSE 1 IN (1,2,abs(-9223372036854775808)) END",
	} {
		if v := evalText(t, q, nil); v != int64(1) {
			t.Errorf("%s: %v", q, v)
		}
	}
}

func TestExpressionReviewCastContextualTypenames(t *testing.T) {
	for _, name := range []string{"STRICT", "WINDOW", "OVER", "FILTER"} {
		q := "CAST('1.5' AS " + name + ")"
		e, err := ParseExpr(q)
		if err != nil {
			t.Errorf("%s: %v", q, err)
			continue
		}
		if v, err := Eval(e, nil); err != nil || v != float64(1.5) {
			t.Errorf("%s: %v %v", q, v, err)
		}
	}
	for _, name := range []string{"ISNULL", "NOTNULL", "NATURAL", "ADD", "TO", "DEFERRABLE", "NOTHING"} {
		if _, err := ParseExpr("CAST('1.5' AS " + name + ")"); err == nil {
			t.Errorf("invalid unquoted typename %s accepted", name)
		}
		if v := evalText(t, "CAST('1.5' AS \""+name+"\")", nil); v != float64(1.5) {
			t.Errorf("quoted %s: %v", name, v)
		}
	}
}

func TestExpressionReviewCaseBetweenEvaluationOrder(t *testing.T) {
	var calls []string
	ec := &EvalContext{Functions: map[string]Function{}}
	for name, value := range map[string]int64{"base": 2, "first": 1, "second": 2, "result": 7, "lower": 1, "upper": 3} {
		ec.Functions[name] = func([]any) (any, error) { calls = append(calls, name); return value, nil }
	}
	for _, c := range []struct {
		q     string
		value any
		calls []string
	}{
		{"CASE base() WHEN first() THEN 0 WHEN second() THEN result() ELSE abs(-9223372036854775808) END", int64(7), []string{"base", "first", "second", "result"}},
		{"base() BETWEEN lower() AND upper()", int64(1), []string{"base", "lower", "upper"}},
		{"CASE WHEN first() THEN result() WHEN second() THEN 0 END", int64(7), []string{"first", "result"}},
	} {
		calls = nil
		if v := evalText(t, c.q, ec); v != c.value || !reflect.DeepEqual(calls, c.calls) {
			t.Errorf("%s: got %v %v", c.q, v, calls)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	ec.Context = ctx
	ec.Functions["first"] = func([]any) (any, error) { cancel(); return int64(0), nil }
	e, err := ParseExpr("CASE WHEN first() THEN 9 WHEN second() THEN 7 END")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = Eval(e, ec); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestExpressionReviewImmutableRewriteAllChildren(t *testing.T) {
	e, err := ParseExpr("CASE base WHEN first THEN CAST(value AS TEXT) ELSE lhs BETWEEN lo AND hi IN(a,b) END")
	if err != nil {
		t.Fatal(err)
	}
	var before []string
	Walk(e, func(n Expr) error {
		if c, ok := n.(*Column); ok {
			before = append(before, c.Name)
		}
		return nil
	})
	r := Rewrite(e, func(n Expr) Expr {
		if c, ok := n.(*Column); ok {
			return &Column{Name: "r_" + c.Name}
		}
		return n
	})
	var after, changed []string
	Walk(e, func(n Expr) error {
		if c, ok := n.(*Column); ok {
			after = append(after, c.Name)
		}
		return nil
	})
	Walk(r, func(n Expr) error {
		if c, ok := n.(*Column); ok {
			changed = append(changed, c.Name)
		}
		return nil
	})
	if !reflect.DeepEqual(before, after) || len(changed) != 8 {
		t.Fatal(before, after, changed)
	}
	for i, n := range changed {
		if n != "r_"+before[i] {
			t.Fatal(changed)
		}
	}
	want := errors.New("stop")
	n := 0
	if err := Walk(e, func(Expr) error {
		n++
		if n == 4 {
			return want
		}
		return nil
	}); !errors.Is(err, want) || n != 4 {
		t.Fatal(err, n)
	}
}
