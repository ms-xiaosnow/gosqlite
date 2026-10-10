package sql

import (
	"context"
	"errors"
	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestSparseDeclarationsScalarsAndExplicitConversions(t *testing.T) {
	for _, n := range []int{1, 16000, 1000000000} {
		decl := " sPaRsEvEc ( 000" + strconv.Itoa(n) + " ) "
		stmt, e := Parse("CREATE TABLE t(s " + decl + ")")
		if e != nil {
			t.Fatal(e)
		}
		if stmt.(*CreateTable).Columns[0].Type != "SPARSEVEC("+strconv.Itoa(n)+")" {
			t.Fatal(stmt)
		}
		if got, ok := SparseDimensions(decl); !ok || got != n || !IsVectorDeclaration(decl) {
			t.Fatal(got, ok)
		}
	}
	for _, bad := range []string{"SPARSEVEC", "SPARSEVEC(0)", "SPARSEVEC(-1)", "SPARSEVEC(+1)", "SPARSEVEC(1.0)", "SPARSEVEC(1e0)", "SPARSEVEC(1000000001)", "SPARSEVEC(999999999999999999999)", "SPARSEVEC(4294967297)", "SPARSEVEC(1,2)", "SPARSEVEC(?)", "SPARSEVEC('1')", "SPARSEVEC(1) TEXT"} {
		if _, e := Parse("CREATE TABLE t(s " + bad + ")"); e == nil {
			t.Fatal(bad)
		}
		if _, ok := SparseDimensions(bad); ok {
			t.Fatal(bad)
		}
	}
	for _, c := range []struct {
		q    string
		want any
	}{
		{"sparsevec_to_text(sparsevec('{9:2,1:3,4:-0}/1000000000'))", "{1:3,9:2}/1000000000"},
		{"typeof(sparsevec('{} / 1000000000'))", "blob"}, {"length(sparsevec('{} / 1000000000'))", int64(16)},
		{"sparsevec_dims('{1:3}/1000000000')", int64(1000000000)}, {"sparsevec_nnz('{1:3,9:-4}/1000000000')", int64(2)},
		{"sparsevec_norm('{1:3,9:-4}/1000000000')", float64(5)},
		{"sparsevec('{1:1,9:2}/1000000000') <-> '{1:4,9:6}/1000000000'", float64(5)},
		{"'{1:4,9:6}/1000000000' <+> sparsevec('{1:1,9:2}/1000000000')", float64(7)},
		{"sparsevec('{1:1,9:2}/1000000000') <#> sparsevec('{1:3,9:4}/1000000000')", float64(-11)},
		{"inner_product(sparsevec('{1:1,9:2}/1000000000'),'{1:3,9:4}/1000000000')", float64(11)},
		{"sparsevec('{1:1}/1000000000') <=> '{999999999:1}/1000000000'", float64(1)},
		{"sparsevec_to_text(sparsevec_from_vector('[1,-0,2]'))", "{1:1,3:2}/3"},
		{"vector_to_text(vector_from_sparsevec('{1:1,3:2}/3'))", "[1,0,2]"},
		{"sparsevec('{} / 1000000000') <-> '{} / 1000000000'", float64(0)},
		{"sparsevec('{} / 1000000000') <#> '{} / 1000000000'", float64(0)},
	} {
		if got := evalText(t, c.q, nil); !reflect.DeepEqual(got, c.want) {
			t.Fatal(c.q, got, c.want)
		}
	}
	for _, q := range []string{"sparsevec('{0:1}/2')", "sparsevec('{3:1}/2')", "sparsevec('{1:1,1:2}/2')", "sparsevec('{1:NaN}/2')", "sparsevec('{1:1e-100}/2')", "sparsevec(X'7b7d2f32')", "vector_from_sparsevec('{} / 1000000000')", "binary_quantize(sparsevec('{} / 1000000000'))", "subvector(sparsevec('{} / 1000000000'),1,1)", "sparsevec('{} / 2') <=> '{1:1}/2'", "sparsevec('{} / 2') <-> vector('[0,0]')", "halfvec('[1]') <-> sparsevec('{1:1}/1')", "sparsevec('{} / 1') <~> bit_vector('1')"} {
		e, err := ParseExpr(q)
		if err != nil {
			t.Fatal(q, err)
		}
		if _, err = Eval(e, nil); err == nil {
			t.Fatal(q)
		}
	}
}

func TestSparseExternalTextIdentityNullAndPreflight(t *testing.T) {
	ec := &EvalContext{Row: map[string]any{"s": "{1:1}/3", "v": "[1,0,0]", "h": "[1,0,0]", "b": "100"}, DeclaredType: func(_, n string) string {
		return map[string]string{"s": "SPARSEVEC(3)", "v": "VECTOR(3)", "h": "HALFVEC(3)", "b": "BIT(3)"}[n]
	}}
	for _, q := range []string{"s <-> '{1:1}/3'", "+s <-> '{1:1}/3'", "coalesce(s,NULL) <-> '{1:1}/3'", "ifnull(s,'{1:8}/3') <-> '{1:1}/3'", "nullif(s,NULL) <-> '{1:1}/3'"} {
		if got := evalText(t, q, ec); got != float64(0) {
			t.Fatal(q, got)
		}
	}
	for _, q := range []string{"vector(s)", "halfvec(s)", "sparsevec(v)", "sparsevec(h)", "sparsevec(b)", "sparsevec_from_vector(s)", "vector_from_sparsevec(v)", "coalesce(s,v) <-> '{1:1}/3'", "s <-> v", "h <+> s", "binary_quantize(s)", "subvector(s,1,2)"} {
		e, _ := ParseExpr(q)
		if _, err := Eval(e, ec); err == nil {
			t.Fatal(q)
		}
	}
	for _, name := range []string{"sparsevec", "sparsevec_dims", "sparsevec_nnz", "sparsevec_norm", "sparsevec_to_text", "sparsevec_from_vector", "vector_from_sparsevec"} {
		if _, e := BuiltinFunctions[name](nil); e == nil {
			t.Fatal(name)
		}
		if got, e := BuiltinFunctions[name]([]any{nil}); e != nil || got != nil {
			t.Fatal(name, got, e)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, e := vectorFunction(ctx, name, []any{nil}); !errors.Is(e, context.Canceled) {
			t.Fatal(e)
		}
	}
	if _, e := SparseValueEntries(strings.Repeat(" ", vector.MaxTextBytes+1)); !errors.Is(e, vector.ErrLimit) {
		t.Fatal(e)
	}
	if _, e := SparseValueEntries(strings.Repeat(":", vector.MaxSparseNonzeros+1)); !errors.Is(e, vector.ErrLimit) {
		t.Fatal(e)
	}
	v, _ := vector.ParseSparse("{1:1}/1000000000")
	b, _ := v.MarshalBinary()
	for _, i := range []int{0, 4, 5, 6, 7, 12} {
		bad := append([]byte(nil), b...)
		bad[i] ^= 1
		if _, e := SparseValueDims(bad, 1000000000); e == nil {
			t.Fatal(i)
		}
	}
	if _, e := SparseValueDims(b, 1); !errors.Is(e, vector.ErrDimensions) {
		t.Fatal(e)
	}
}
