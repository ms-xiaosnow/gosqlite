package sql

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type scalarTestBudget struct{ work, bytes int64 }

var errScalarTestWork = errors.New("pure memory scalar work quota")
var errScalarTestBytes = errors.New("pure memory scalar bytes quota")

func (b *scalarTestBudget) Step(n int64) error {
	if n < 0 || n > b.work {
		return errScalarTestWork
	}
	b.work -= n
	return nil
}
func (b *scalarTestBudget) Allocate(n int64) error {
	if n < 0 || n > b.bytes {
		return errScalarTestBytes
	}
	b.bytes -= n
	return nil
}

func TestRelationalScalarTruthAndEscapeBudgets(t *testing.T) {
	for _, q := range []string{"CASE WHEN ? THEN 1 ELSE 0 END", "'x' LIKE 'x' ESCAPE ?"} {
		expr, err := ParseExpr(q)
		if err != nil {
			t.Fatal(err)
		}
		budget := &scalarTestBudget{work: 100, bytes: 1 << 20}
		ctx := WithEvalBudget(context.Background(), budget)
		if _, err = Eval(expr, &EvalContext{Context: ctx, Params: []any{strings.Repeat("0", 10000)}}); !errors.Is(err, errScalarTestWork) {
			t.Fatal(q, err)
		}
	}
	budget := &scalarTestBudget{work: 100, bytes: 1 << 20}
	if _, _, err := TruthContext(WithEvalBudget(context.Background(), budget), strings.Repeat("0", 10000)); !errors.Is(err, errScalarTestWork) {
		t.Fatal(err)
	}
	// Exact-match LIKE against a long ASCII literal needs rune/term scratch,
	// even when an empty input makes matching return immediately.
	expr, err := ParseExpr("'' LIKE ?")
	if err != nil {
		t.Fatal(err)
	}
	budget = &scalarTestBudget{work: 1 << 20, bytes: 500000}
	if _, err = Eval(expr, &EvalContext{Context: WithEvalBudget(context.Background(), budget), Params: []any{strings.Repeat("a", 10000)}}); !errors.Is(err, errScalarTestBytes) {
		t.Fatal(err)
	}
}

func TestRelationalCheckedRewriteStops(t *testing.T) {
	expr, err := ParseExpr("1+2+3+4")
	if err != nil {
		t.Fatal(err)
	}
	entered, transformed := 0, 0
	_, err = RewriteChecked(expr, func(Expr) error {
		entered++
		if entered == 2 {
			return errScalarTestWork
		}
		return nil
	}, func(e Expr) (Expr, error) { transformed++; return e, nil })
	if !errors.Is(err, errScalarTestWork) || entered != 2 || transformed != 0 {
		t.Fatal(err, entered, transformed)
	}
}
