package sql

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// Pure memory expressions and custom functions; no database fault injection.
func TestBooleanScalarVersusPredicate(t *testing.T) {
	const boom = "abs(-9223372036854775808)"
	for _, tc := range []struct {
		expression                         string
		scalar                             any
		scalarErr, predicate, predicateErr bool
	}{
		{"0 AND " + boom, int64(0), false, false, false}, {boom + " AND 0", int64(0), false, false, false},
		{"1 OR " + boom, int64(1), false, true, false}, {boom + " OR 1", int64(1), false, true, false},
		{"0.0 AND " + boom, nil, true, false, false}, {"?1 AND " + boom, nil, true, false, false},
		{"x AND " + boom, nil, true, false, false}, {"NULL AND " + boom, nil, true, false, false},
		{"NOT(NULL OR " + boom + ")", nil, true, false, false},
		{"(NULL AND " + boom + ") IS NOT TRUE", nil, true, true, false},
		{"(x AND " + boom + ")=0", nil, true, false, true},
		{"x BETWEEN 1 AND " + boom, nil, true, false, false},
		{"x NOT BETWEEN 1 AND " + boom, nil, true, true, false},
		{"NULL NOT BETWEEN 1 AND " + boom, nil, true, false, true},
		{"+1 OR " + boom, nil, true, true, false},
		{"0_0 AND " + boom, nil, true, false, false},
	} {
		e, err := ParseExpr(tc.expression)
		if err != nil {
			t.Fatal(err)
		}
		ec := &EvalContext{Params: []any{int64(0)}, Row: map[string]any{"x": int64(0)}}
		got, err := Eval(e, ec)
		if (err != nil) != tc.scalarErr || err == nil && !reflect.DeepEqual(got, tc.scalar) {
			t.Errorf("scalar %s: %#v %v", tc.expression, got, err)
		}
		match, err := EvalPredicate(e, ec)
		if (err != nil) != tc.predicateErr || err == nil && match != tc.predicate {
			t.Errorf("predicate %s: %v %v", tc.expression, match, err)
		}
	}
}

func TestBooleanThreeValuedTruthTables(t *testing.T) {
	values := []any{int64(0), int64(1), nil}
	for _, op := range []string{"AND", "OR"} {
		for _, a := range values {
			for _, b := range values {
				e, err := ParseExpr("?1 " + op + " ?2")
				if err != nil {
					t.Fatal(err)
				}
				ec := &EvalContext{Params: []any{a, b}}
				v, err := Eval(e, ec)
				if err != nil {
					t.Fatal(err)
				}
				truth, null := Truth(v)
				for _, wrapper := range []string{"%s", "NOT(%s)", "(%s) IS TRUE", "(%s) IS FALSE", "(%s) IS NOT TRUE", "(%s) IS NOT FALSE"} {
					text := strings.Replace(wrapper, "%s", "?1 "+op+" ?2", 1)
					expr, err := ParseExpr(text)
					if err != nil {
						t.Fatal(err)
					}
					expected := truth && !null
					switch wrapper {
					case "NOT(%s)":
						expected = !truth && !null
					case "(%s) IS FALSE":
						expected = !truth && !null
					case "(%s) IS NOT TRUE":
						expected = !truth || null
					case "(%s) IS NOT FALSE":
						expected = truth || null
					}
					got, err := EvalPredicate(expr, ec)
					if err != nil || got != expected {
						t.Errorf("%s %v/%v: %v %v expected %v", text, a, b, got, err, expected)
					}
				}
			}
		}
	}
}

func TestBooleanCustomFunctionsCancellationAndQuotas(t *testing.T) {
	sentinel := errors.New("ordinary scalar failure")
	calls := 0
	ec := &EvalContext{Functions: map[string]Function{
		"bad":  func([]any) (any, error) { calls++; return nil, sentinel },
		"zero": func([]any) (any, error) { calls++; return int64(0), nil },
	}}
	for _, q := range []string{"0 AND bad()", "1 OR bad()", "CASE WHEN zero() AND bad() THEN 1 ELSE 7 END", "CASE WHEN zero() BETWEEN 1 AND bad() THEN 1 ELSE 7 END"} {
		calls = 0
		e, err := ParseExpr(q)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = Eval(e, ec); err != nil {
			t.Fatal(q, err)
		}
		expected := 0
		if strings.Contains(q, "zero()") {
			expected = 1
		}
		if calls != expected {
			t.Fatal(q, calls)
		}
	}
	e, _ := ParseExpr("zero() AND bad()")
	if _, err := Eval(e, ec); !errors.Is(err, sentinel) {
		t.Fatal("scalar custom function must remain eager", err)
	}
	if ConstantExpression(e, ec) {
		t.Fatal("custom functions acquired a purity guarantee")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := EvalPredicate(e, &EvalContext{Context: ctx}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	e, _ = ParseExpr("?1 AND bad()")
	budget := &scalarTestBudget{work: 100, bytes: 1 << 20}
	ec.Context = WithEvalBudget(context.Background(), budget)
	ec.Params = []any{strings.Repeat("0", 10000)}
	if _, err := EvalPredicate(e, ec); !errors.Is(err, errScalarTestWork) {
		t.Fatal("predicate truth coercion escaped quota", err)
	}
	// LEFT ON cannot simplify a trailing literal around an error-producing lhs.
	e, _ = ParseExpr("bad() OR 1")
	ec.Context = nil
	ec.DisableBooleanSimplification = true
	if _, err := EvalPredicate(e, ec); !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
}
