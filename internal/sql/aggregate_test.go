package sql

import (
	"math"
	"reflect"
	"strings"
	"testing"
)

// Descriptor-free arithmetic tests; no storage failure injection.
func TestGlobalAggregateArithmetic(t *testing.T) {
	cases := []struct {
		name  string
		input []any
		want  any
		err   string
	}{
		{"count", nil, int64(0), ""}, {"count", []any{nil, int64(1), "", []byte{}}, int64(3), ""},
		{"sum", nil, nil, ""}, {"total", nil, float64(0), ""}, {"avg", nil, nil, ""},
		{"sum", []any{int64(1), int64(2)}, int64(3), ""},
		{"sum", []any{"1", "2"}, int64(3), ""},
		{"sum", []any{"1\x00tail"}, int64(1), ""},
		{"sum", []any{"1.0\x00tail"}, float64(1), ""},
		{"sum", []any{"1tail\x00"}, float64(1), ""},
		{"sum", []any{"\x001"}, float64(0), ""},
		{"sum", []any{[]byte("1\x00tail")}, float64(1), ""},
		{"sum", []any{int64(math.MaxInt64), int64(1), "0\x00tail"}, nil, "integer overflow"}, {"sum", []any{"1.0", "2"}, float64(3), ""},
		{"sum", []any{"1suffix", "2"}, float64(3), ""}, {"sum", []any{[]byte("1"), int64(2)}, float64(3), ""},
		{"sum", []any{int64(math.MaxInt64), int64(1)}, nil, "integer overflow"},
		{"sum", []any{int64(math.MaxInt64), int64(1), float64(0)}, float64(9223372036854775808.0), ""},
		{"sum", []any{float64(1e16), float64(1), float64(-1e16)}, float64(1), ""},
		{"sum", []any{float64(1e308), float64(1e308)}, math.Inf(1), ""},
		{"sum", []any{math.Inf(1), math.Inf(-1)}, nil, ""},
		{"min", []any{nil, "abc", []byte{0}, int64(3), float64(2)}, float64(2), ""},
		{"max", []any{nil, "abc", []byte{0}, int64(3), float64(2)}, []byte{0}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := Aggregate{Name: c.name}
			for _, v := range c.input {
				if err := a.Step(v); err != nil {
					t.Fatal(err)
				}
			}
			got, err := a.Result()
			if c.err != "" {
				if err == nil || !strings.Contains(err.Error(), c.err) {
					t.Fatal(got, err)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(got, c.want) {
				t.Fatalf("%T %v, %v; want %T %v", got, got, err, c.want, c.want)
			}
		})
	}
	for _, name := range []string{"count", "sum", "total", "avg", "min", "max"} {
		a := Aggregate{Name: name}
		if err := a.Step([]float32{1}); err == nil {
			t.Fatal("accepted typed value", name)
		}
	}
}
func TestGlobalAggregateGrammar(t *testing.T) {
	for _, q := range []string{"SELECT count(*) FROM t", "SELECT sum(x) FROM t", "SELECT sum(x) FROM t GROUP BY x", "SELECT count(*) FROM t HAVING count(*)>0", "SELECT count()", "SELECT sum(x),total(x),avg(x),min(x),max(x) FROM t", "SELECT typeof(sum(x)),coalesce(avg(x),0) FROM t"} {
		if _, err := Parse(q); err != nil {
			t.Fatal(q, err)
		}
	}
	for _, q := range []string{"SELECT count(DISTINCT x) FROM t", "SELECT sum(x) OVER () FROM t", "SELECT count(*) FILTER (WHERE x) FROM t", "SELECT sum(x ORDER BY x) FROM t"} {
		if _, err := Parse(q); err == nil {
			t.Fatal("accepted", q)
		}
	}
}
