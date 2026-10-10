package sql

import (
	"context"
	"errors"
	"testing"
)

func TestConnectionFunctionContextAndVolatility(t *testing.T) {
	state := ConnectionState{Changes: 2, TotalChanges: 9, LastInsertRowID: -3}
	ctx := &EvalContext{Context: WithConnectionState(context.Background(), &state)}
	for _, tc := range []struct {
		name string
		want int64
	}{{"changes", 2}, {"total_changes", 9}, {"last_insert_rowid", -3}} {
		e := &Call{Name: tc.name}
		got, err := Eval(e, ctx)
		if err != nil || got != tc.want {
			t.Fatal(tc.name, got, err)
		}
		if ConstantExpression(e, ctx) {
			t.Fatal("connection counter classified deterministic", tc.name)
		}
		if _, err := Eval(e, nil); err == nil {
			t.Fatal("fabricated missing connection value")
		}
		if err := ValidateFunction(tc.name, 1); err == nil {
			t.Fatal("accepted wrong arity")
		}
	}
	state.LastInsertRowID = 42
	if got, err := Eval(&Call{Name: "LAST_INSERT_ROWID"}, ctx); err != nil || got != int64(42) {
		t.Fatal(got, err)
	}
	c, cancel := context.WithCancel(ctx.Context)
	cancel()
	if _, err := Eval(&Call{Name: "changes"}, &EvalContext{Context: c}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
