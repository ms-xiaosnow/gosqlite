package sql

import (
	"bytes"
	"context"
	"errors"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

func TestBitDeclarations(t *testing.T) {
	for _, n := range []int{1, 9, 64000} {
		declared := " bIt ( 000" + strconv.Itoa(n) + " ) "
		stmt, err := Parse("CREATE TABLE t(v " + declared + " DEFAULT NULL)")
		if err != nil {
			t.Fatal(err)
		}
		if got := stmt.(*CreateTable).Columns[0].Type; got != "BIT("+strconv.Itoa(n)+")" {
			t.Fatal(got)
		}
		if got, ok := BitDimensions(declared); !ok || got != n {
			t.Fatal(got, ok)
		}
		if !IsVectorDeclaration(declared) {
			t.Fatal("typed bit unrecognized")
		}
		if _, ok := VectorDimensions(declared); ok {
			t.Fatal("bit became dense")
		}
	}
	for _, bad := range []string{"BIT", "BIT(0)", "BIT(-1)", "BIT(+1)", "BIT(1.0)", "BIT(1e0)", "BIT(1_0)", "BIT(0x1)", "BIT(64001)", "BIT(999999999999999999999999999)", "BIT(1,2)", "BIT(?)", "BIT('1')", "BIT(1) TEXT", "BITS(1)", "BIT(1)junk"} {
		if _, err := Parse("CREATE TABLE t(v " + bad + ")"); err == nil {
			t.Fatalf("accepted %s", bad)
		}
		if _, ok := BitDimensions(bad); ok {
			t.Fatalf("recognized %s", bad)
		}
	}
	if _, ok := BitDimensions(strings.Repeat(" ", MaxSQLBytes+1) + "BIT(1)"); ok {
		t.Fatal("unbounded declaration")
	}
	for _, op := range []string{"<~>", "<%>"} {
		if _, err := Parse("CREATE TABLE t(v REAL DEFAULT ('1' " + op + " '0'))"); err == nil {
			t.Fatal("persisted non-SQLite default")
		}
	}
}

func TestBitScalarAndTransforms(t *testing.T) {
	cases := []struct {
		expr string
		want any
	}{
		{"bit_to_text(bit_vector('001010001'))", "001010001"},
		{"bit_dims(bit_vector(''))", int64(0)},
		{"typeof(bit_vector('001'))", "blob"},
		{"length(bit_vector('001'))", int64(13)},
		{"'101' <~> '011'", float64(2)},
		{"hamming_distance(bit_vector('101'),'011')", float64(2)},
		{"'100' <%> '110'", float64(.5)},
		{"jaccard_distance('000','000')", float64(1)},
		{"'' <%> ''", float64(1)},
		{"'' <~> ''", float64(0)},
		{"vector_to_text(subvector('[1,2,3,4,5]',-1,3))", "[1]"},
		{"vector_to_text(subvector('[1,2,3,4,5]',3,2147483647))", "[3,4,5]"},
		{"vector_to_text(subvector('[1,2,3,4,5]',-2147483644,2147483647))", "[1,2]"},
		{"bit_to_text(binary_quantize('[-1,-0,0,1e-45,3]'))", "00011"},
		{"'[1,2]' <+> '[4,6]'", float64(7)},
		{"~3 & 7 | 16", int64(20)},
		{"1 << 3 >> 1", int64(4)},
	}
	for _, c := range cases {
		if got := evalText(t, c.expr, nil); got != c.want {
			t.Fatalf("%s = %#v, want %#v", c.expr, got, c.want)
		}
	}
	// Dense subvector preserves signed zero and subnormal component bits.
	v, _ := vector.New([]float32{1, math.Float32frombits(1 << 31), math.SmallestNonzeroFloat32})
	input, _ := v.MarshalBinary()
	out := evalText(t, "subvector(?,2,2)", &EvalContext{Params: []any{input}}).([]byte)
	want, _ := vector.New([]float32{math.Float32frombits(1 << 31), math.SmallestNonzeroFloat32})
	binary, _ := want.MarshalBinary()
	if !bytes.Equal(out, binary) {
		t.Fatal("subvector changed bits")
	}
	for _, c := range []struct{ expr, shape string }{
		{"a <~> b + c", "(a <~> (b + c))"},
		{"a <~> b & c", "((a <~> b) & c)"},
		{"a & b <%> c", "((a & b) <%> c)"},
		{"a <~> b = c", "((a <~> b) = c)"},
		{"NOT a <%> b = 0", "(NOT ((a <%> b) = 0))"},
	} {
		e, err := ParseExpr(c.expr)
		if err != nil {
			t.Fatal(err)
		}
		if got := vectorExprShape(e); got != c.shape {
			t.Fatal(got, c.shape)
		}
	}
	tokens, err := Tokenize("a<~>b,a<%>b,~1,1<<2,1>>2,'<~>',\"<%>\" /* <%> */")
	if err != nil {
		t.Fatal(err)
	}
	var operators []string
	for _, tok := range tokens {
		if tok.Kind == Symbol && tok.Text != "," {
			operators = append(operators, tok.Text)
		}
	}
	if !reflect.DeepEqual(operators, []string{"<~>", "<%>", "~", "<<", ">>"}) {
		t.Fatal(operators)
	}
}

func TestBitScalarStrictBoundaries(t *testing.T) {
	for _, expr := range []string{
		"'[1]' <~> '[2]'", "'[1]' <%> '[2]'", "bit_vector(101)", "bit_vector(' 101')", "bit_vector('101 ')", "bit_vector('102')", "bit_vector(X'313031')",
		"bit_vector(vector('[1]'))", "vector(bit_vector('1'))", "'101' <~> '[1,0,1]'", "'1' <~> '01'",
		"bit_vector('1') <+> bit_vector('0')", "binary_quantize(bit_vector('1'))", "subvector(bit_vector('1'),1,1)",
		"subvector('[1,2]',1.0,1)", "subvector('[1,2]','1',1)", "subvector('[1,2]',1,1.5)",
		"subvector('[1,2]',2147483648,1)", "subvector('[1,2]',-2147483649,1)",
		"subvector('[1,2]',1,0)", "subvector('[1,2]',-1,2)", "subvector('[1,2]',3,1)",
	} {
		e, err := ParseExpr(expr)
		if err != nil {
			t.Fatal(expr, err)
		}
		if _, err = Eval(e, nil); err == nil {
			t.Fatalf("accepted %s", expr)
		}
	}
	for _, name := range []string{"HALFVEC(2)", "SPARSEVEC(2)"} {
		if _, err := Parse("CREATE TABLE t(v " + name + ")"); err == nil {
			t.Fatal("unimplemented type accepted", name)
		}
	}
	for name, arity := range map[string]int{"bit_vector": 1, "bit_dims": 1, "bit_to_text": 1, "hamming_distance": 2, "jaccard_distance": 2, "binary_quantize": 1, "subvector": 3} {
		for _, n := range []int{0, arity + 1} {
			if _, err := BuiltinFunctions[name](make([]any, n)); err == nil {
				t.Fatal(name, n)
			}
		}
		for pos := 0; pos < arity; pos++ {
			args := make([]any, arity)
			for i := range args {
				args[i] = "bad"
			}
			args[pos] = nil
			if got, err := BuiltinFunctions[name](args); err != nil || got != nil {
				t.Fatal(name, pos, got, err)
			}
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if got, err := vectorFunction(ctx, name, make([]any, arity)); !errors.Is(err, context.Canceled) || got != nil {
			t.Fatal(name, got, err)
		}
	}
	// Invalid in-memory codec data never reaches the filesystem.
	v, _ := vector.ParseBit("101")
	good, _ := v.MarshalBinary()
	for _, index := range []int{0, 4, 5, 6, 7, 12} {
		bad := append([]byte(nil), good...)
		bad[index] ^= 1
		if _, err := BitValue(bad); !errors.Is(err, vector.ErrEncoding) {
			t.Fatal(index, err)
		}
	}
	h, _ := vector.NewHalf([]float32{1})
	hb, _ := h.MarshalBinary()
	s, _ := vector.NewSparse(1, []vector.SparseEntry{{Index: 0, Value: 1}})
	sb, _ := s.MarshalBinary()
	for _, bad := range []any{hb, sb, vector.BitVector{}, int64(1)} {
		if _, err := BitValue(bad); err == nil {
			t.Fatal("cross type accepted")
		}
	}
	if got := evalText(t, "hamming_distance('1','0')", &EvalContext{Functions: map[string]Function{"hamming_distance": func([]any) (any, error) { return int64(42), nil }}}); got != int64(42) {
		t.Fatal("function override lost")
	}
}
