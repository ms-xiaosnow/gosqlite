package sql

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

// Pure-memory scalar inputs. BLOB is a declared affinity even when the stored
// value is numeric; an affinity-less expression/alias uses the empty marker.
func TestAffinityDeclaredVersusAbsent(t *testing.T) {
	ctx := &EvalContext{Row: map[string]any{"t": "1.0", "b": float64(1), "u": float64(1), "n": int64(1), "r": float64(1), "physical_alias": float64(1), "computed_alias": float64(1)}, Params: []any{float64(1)}}
	ctx.Affinity = func(_, n string) string {
		switch n {
		case "t":
			return "TEXT"
		case "b", "u", "physical_alias":
			return "BLOB"
		case "n":
			return "NUMERIC"
		case "r":
			return "REAL"
		}
		return ""
	}
	for _, tc := range []struct {
		q    string
		want any
	}{
		{"t=b", int64(0)}, {"b=t", int64(0)}, {"t=u", int64(0)}, {"u=t", int64(0)},
		{"t IS b", int64(0)}, {"b IS NOT t", int64(1)}, {"t!=b", int64(1)},
		{"t>b", int64(1)}, {"b<t", int64(1)}, {"t<=b", int64(0)}, {"b>=t", int64(0)},
		{"t=+b", int64(1)}, {"+b=t", int64(1)}, {"t=+u", int64(1)}, {"+t=b", int64(0)},
		{"t=n", int64(1)}, {"n=t", int64(1)}, {"t=r", int64(1)}, {"r=t", int64(1)},
		{"t=physical_alias", int64(0)}, {"physical_alias=t", int64(0)},
		{"t=computed_alias", int64(1)}, {"computed_alias=t", int64(1)},
		{"t=?1", int64(1)}, {"?1=t", int64(1)}, {"t=1.0", int64(1)}, {"t=1", int64(0)},
		{"t=coalesce(b,NULL)", int64(1)}, {"t=CASE WHEN 1 THEN b END", int64(1)},
		{"CAST(1.0 AS TEXT)=b", int64(0)}, {"b=CAST(1.0 AS TEXT)", int64(0)},
		{"t=CAST(b AS TEXT)", int64(1)}, {"CAST(b AS TEXT)", "1.0"},
		{"CASE t WHEN b THEN 1 ELSE 0 END", int64(0)}, {"CASE t WHEN +b THEN 1 ELSE 0 END", int64(1)},
		{"t IN (b)", int64(1)}, {"t NOT IN (b)", int64(0)},
		{"t BETWEEN b AND b", int64(0)}, {"t NOT BETWEEN b AND b", int64(1)},
		{"t BETWEEN +b AND +b", int64(1)}, {"t=NULL", nil},
	} {
		e, err := ParseExpr(tc.q)
		if err != nil {
			t.Fatal(err)
		}
		got, err := Eval(e, ctx)
		if err != nil || !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: got %T(%v), %v; want %T(%v)", tc.q, got, got, err, tc.want, tc.want)
		}
	}
	if AffinityType("") != "BLOB" {
		t.Fatal("typeless physical column lost BLOB affinity")
	}
	for _, tc := range []struct{ q, want string }{{"u", "BLOB"}, {"physical_alias", "BLOB"}, {"computed_alias", ""}, {"+u", ""}, {"?1", ""}, {"coalesce(u,NULL)", ""}, {"CASE WHEN 1 THEN u END", ""}} {
		e, err := ParseExpr(tc.q)
		if err != nil {
			t.Fatal(err)
		}
		if got := ExpressionAffinity(e, ctx.Affinity); got != tc.want {
			t.Errorf("%s affinity=%q want %q", tc.q, got, tc.want)
		}
	}
}

func TestAffinityComparisonContextAndBudget(t *testing.T) {
	for _, q := range []string{"t=b", "t=+b", "t=?1", "t BETWEEN b AND b", "t IN (b)"} {
		e, err := ParseExpr(q)
		if err != nil {
			t.Fatal(err)
		}
		ec := &EvalContext{Row: map[string]any{"t": "1.0", "b": float64(1)}, Params: []any{float64(1)}, Affinity: func(_, n string) string {
			if n == "t" {
				return "TEXT"
			}
			return "BLOB"
		}}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		ec.Context = ctx
		if _, err := Eval(e, ec); !errors.Is(err, context.Canceled) {
			t.Errorf("%s cancellation: %v", q, err)
		}
		ec.Context = WithEvalBudget(context.Background(), &scalarTestBudget{work: 0, bytes: 1 << 20})
		if _, err := Eval(e, ec); !errors.Is(err, errScalarTestWork) {
			t.Errorf("%s work: %v", q, err)
		}
		ec.Context = WithEvalBudget(context.Background(), &scalarTestBudget{work: 1 << 20, bytes: 0})
		if _, err := Eval(e, ec); !errors.Is(err, errScalarTestBytes) {
			t.Errorf("%s bytes: %v", q, err)
		}
	}
}
