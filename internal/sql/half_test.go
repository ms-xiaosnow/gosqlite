package sql

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

func TestHalfDeclarationsAndScalars(t *testing.T) {
	for _, n := range []int{1, 17, 16000} {
		decl := " hAlFvEc ( 000" + strconv.Itoa(n) + " ) "
		stmt, err := Parse("CREATE TABLE t(h " + decl + ")")
		if err != nil {
			t.Fatal(err)
		}
		if stmt.(*CreateTable).Columns[0].Type != "HALFVEC("+strconv.Itoa(n)+")" {
			t.Fatal(stmt)
		}
		if got, ok := HalfDimensions(decl); !ok || got != n || !IsVectorDeclaration(decl) {
			t.Fatal(got, ok)
		}
		if _, ok := VectorDimensions(decl); ok {
			t.Fatal("half is not dense")
		}
	}
	for _, bad := range []string{"HALFVEC", "HALFVEC(0)", "HALFVEC(-1)", "HALFVEC(+1)", "HALFVEC(1.0)", "HALFVEC(1e0)", "HALFVEC(16001)", "HALFVEC(99999999999999999999999999999)", "HALFVEC(1,2)", "HALFVEC(?)", "HALFVEC('1')", "HALFVEC(1) TEXT"} {
		if _, err := Parse("CREATE TABLE t(h " + bad + ")"); err == nil {
			t.Fatal(bad)
		}
		if _, ok := HalfDimensions(bad); ok {
			t.Fatal(bad)
		}
	}
	if _, ok := HalfDimensions(strings.Repeat(" ", MaxSQLBytes+1) + "HALFVEC(1)"); ok {
		t.Fatal("unbounded declaration")
	}
	for _, c := range []struct {
		expr string
		want any
	}{
		{"halfvec_to_text(halfvec('[1.0001,-0,0.000000059604645]'))", "[1,-0,5.9604645e-08]"},
		{"typeof(halfvec('[1]'))", "blob"}, {"length(halfvec('[1]'))", int64(14)},
		{"halfvec_dims(halfvec('[1,2,3]'))", int64(3)},
		{"halfvec_norm('[3,4]')", float64(5)},
		{"halfvec('[1,2]') <-> '[4,6]'", float64(5)},
		{"'[4,6]' <+> halfvec('[1,2]')", float64(7)},
		{"halfvec('[1,2]') <#> halfvec('[3,4]')", float64(-11)},
		{"inner_product(halfvec('[1,2]'),'[3,4]')", float64(11)},
		{"halfvec('[1,0]') <=> halfvec('[0,1]')", float64(1)},
		{"halfvec_to_text(subvector(halfvec('[1,2,3,4]'),-1,3))", "[1]"},
		{"halfvec_to_text(subvector(halfvec('[1,2,3,4]'),3,2147483647))", "[3,4]"},
		{"bit_to_text(binary_quantize(halfvec('[-1,-0,0,1e-45,0.000000059604645]')))", "00001"},
		{"vector_to_text(vector_from_halfvec(halfvec('[1.0001]')))", "[1]"},
		{"halfvec_to_text(halfvec_from_vector(vector('[1.0001]')))", "[1]"},
		{"halfvec_conversion_changed('[1,1.0001,-0]')", int64(1)},
		{"subvector(halfvec('[1]'),NULL,1)", nil},
		{"subvector(halfvec('[1]'),1,NULL)", nil},
		{"halfvec_conversion_max_abs('[1.000244140625]')", float64(0.000244140625)},
		{"halfvec_conversion_max_rel('[2.00048828125]')", float64(0.00048828125 / 2.00048828125)},
	} {
		if got := evalText(t, c.expr, nil); !reflect.DeepEqual(got, c.want) {
			t.Fatalf("%s=%#v want %#v", c.expr, got, c.want)
		}
	}
}

func TestHalfScalarIdentityNullAndBounds(t *testing.T) {
	for _, expr := range []string{"halfvec(vector('[1]'))", "vector(halfvec('[1]'))", "halfvec('[1]') <-> vector('[1]')", "vector('[1]') <+> halfvec('[1]')", "halfvec('[1]') <~> bit_vector('1')", "binary_quantize(halfvec('[65520]'))", "halfvec('[NaN]')", "halfvec('[]')", "halfvec(X'5b315d')", "halfvec('[0]') <=> '[1]'", "subvector(halfvec('[1,2]'),1.0,1)", "subvector(halfvec('[1,2]'),1,0)"} {
		e, err := ParseExpr(expr)
		if err != nil {
			t.Fatal(expr, err)
		}
		if _, err = Eval(e, nil); err == nil {
			t.Fatal("accepted", expr)
		}
	}
	for _, name := range []string{"halfvec", "halfvec_dims", "halfvec_norm", "halfvec_to_text", "halfvec_from_vector", "vector_from_halfvec", "halfvec_conversion_changed", "halfvec_conversion_max_abs", "halfvec_conversion_max_rel"} {
		if _, err := BuiltinFunctions[name](nil); err == nil {
			t.Fatal(name)
		}
		if v, err := BuiltinFunctions[name]([]any{nil}); err != nil || v != nil {
			t.Fatal(name, v, err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := vectorFunction(ctx, name, []any{nil}); !errors.Is(err, context.Canceled) {
			t.Fatal(name, err)
		}
	}
	// Type evidence is carried separately from affinity for valid external TEXT.
	ec := &EvalContext{Row: map[string]any{"h": "[1.0001]", "v": "[1.0001]"}, DeclaredType: func(_, n string) string {
		if n == "h" {
			return "HALFVEC(1)"
		}
		return "VECTOR(1)"
	}}
	if got := evalText(t, "h <-> '[1]'", ec); got != float64(0) {
		t.Fatal(got)
	}
	for _, expr := range []string{"+h <-> '[1]'", "coalesce(h,NULL) <-> '[1]'", "ifnull(h,'[8]') <-> '[1]'"} {
		if got := evalText(t, expr, ec); got != float64(0) {
			t.Fatal(expr, got)
		}
	}
	for _, expr := range []string{"halfvec(v)", "vector(h)", "halfvec_norm(v)", "vector_norm(h)", "halfvec_from_vector(h)", "vector_from_halfvec(v)", "coalesce(h,v) <-> '[1]'"} {
		e, _ := ParseExpr(expr)
		if _, err := Eval(e, ec); err == nil {
			t.Fatal("mixed external TEXT signature", expr)
		}
	}

	if got := evalText(t, "halfvec_to_text(subvector(h,1,1))", ec); got != "[1]" {
		t.Fatal(got)
	}
	e, _ := ParseExpr("h <-> v")
	if _, err := Eval(e, ec); err == nil {
		t.Fatal("mixed typed TEXT accepted")
	}
	// Malformed bytes stay strictly in-memory.
	h, _ := vector.ParseHalf("[1,2]")
	b, _ := h.MarshalBinary()
	for _, i := range []int{0, 4, 5, 6, 7} {
		bad := append([]byte(nil), b...)
		bad[i] ^= 1
		if _, err := HalfValueDims(bad, 2); err == nil {
			t.Fatal(i)
		}
	}
	if _, err := HalfValueDims(b, 1); !errors.Is(err, vector.ErrDimensions) {
		t.Fatal(err)
	}
	if _, err := HalfValueDims(strings.Repeat(" ", vector.MaxTextBytes+1), 1); !errors.Is(err, vector.ErrLimit) {
		t.Fatal(err)
	}
}
