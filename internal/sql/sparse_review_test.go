package sql

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"math"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

// These expected bytes are assembled independently of the production codec.
func sparseReviewBlob(dim uint32, indices []uint32, bits []uint32) []byte {
	b := make([]byte, 16+8*len(indices))
	copy(b, []byte{'G', 'S', 'S', 0, 1, 4, 1, 0})
	binary.BigEndian.PutUint32(b[8:], dim)
	binary.BigEndian.PutUint32(b[12:], uint32(len(indices)))
	for i, k := range indices {
		binary.BigEndian.PutUint32(b[16+8*i:], k)
		binary.BigEndian.PutUint32(b[20+8*i:], bits[i])
	}
	return b
}

func TestSparseReviewCanonicalCodecAndPreflight(t *testing.T) {
	const dims = 1000000000
	text := "{1000000000:-2,1:3,17:-0,1:0,17:0}/1000000000"
	want := sparseReviewBlob(dims, []uint32{0, dims - 1}, []uint32{0x40400000, 0xc0000000})
	got, e := BuiltinFunctions["sparsevec"]([]any{text})
	if e != nil || !bytes.Equal(got.([]byte), want) {
		t.Fatal(got, e, want)
	}
	v, e := SparseValueDims(want, dims)
	if e != nil || v.String() != "{1:3,1000000000:-2}/1000000000" || v.NNZ() != 2 {
		t.Fatal(v, e)
	}
	if n, e := SparseValueEntries(text); e != nil || n != 5 {
		t.Fatal(n, e)
	}
	if n, e := SparseValueEntries(want); e != nil || n != 2 {
		t.Fatal(n, e)
	}
	empty := sparseReviewBlob(dims, nil, nil)
	z, e := SparseValueDims(empty, dims)
	if e != nil || !z.Valid() || z.NNZ() != 0 {
		t.Fatal(z, e)
	}
	if _, e := z.Dense(); !errors.Is(e, vector.ErrLimit) {
		t.Fatal("large dense conversion must fail before allocation", e)
	}

	// Noncanonical and invalid values are memory-only; no disk bytes are altered.
	bad := [][]byte{
		sparseReviewBlob(dims, []uint32{0}, []uint32{0}),
		sparseReviewBlob(dims, []uint32{0}, []uint32{0x80000000}),
		sparseReviewBlob(dims, []uint32{0}, []uint32{0x7f800000}),
		sparseReviewBlob(dims, []uint32{0}, []uint32{0x7fc00000}),
		sparseReviewBlob(dims, []uint32{1, 0}, []uint32{0x3f800000, 0x40000000}),
		sparseReviewBlob(dims, []uint32{1, 1}, []uint32{0x3f800000, 0x40000000}),
		sparseReviewBlob(dims, []uint32{dims}, []uint32{0x3f800000}),
		sparseReviewBlob(0, nil, nil), sparseReviewBlob(dims+1, nil, nil),
		append(append([]byte(nil), want...), 0), want[:len(want)-1], []byte(text),
	}
	for i, b := range bad {
		if _, e := SparseValue(b); e == nil {
			t.Fatalf("accepted noncanonical %d", i)
		}
	}
	for _, s := range []string{"{0:0}/1000000000", "{1000000001:0}/1000000000", "{2:0}/1", "{1:2,1:3}/1000000000", "{1:1e-100}/1000000000"} {
		if _, e := SparseValue(s); e == nil {
			t.Fatal(s)
		}
	}
	subnormal := sparseReviewBlob(dims, []uint32{dims - 1}, []uint32{1})
	vv, e := SparseValue(subnormal)
	if e != nil || math.Float32bits(vv.Entries()[0].Value) != 1 {
		t.Fatal(vv, e)
	}
	// Scan failure retains both fields; successful binary scan owns its backing.
	n := vector.NullSparseVector{SparseVector: v, Valid: true}
	for _, b := range bad {
		if e := n.Scan(b); e == nil || !n.Valid || n.SparseVector.String() != v.String() {
			t.Fatal(e, n)
		}
	}
	owned := append([]byte(nil), want...)
	if e := n.Scan(owned); e != nil {
		t.Fatal(e)
	}
	owned[20] = 0
	entries := n.SparseVector.Entries()
	entries[0].Value = 99
	if n.SparseVector.String() != v.String() {
		t.Fatal("borrowed codec or accessor memory")
	}
	if e := n.Scan(nil); e != nil || n.Valid || n.SparseVector.Valid() {
		t.Fatal(n, e)
	}
	if _, e := SparseValueEntries(strings.Repeat(":", vector.MaxSparseNonzeros+1)); !errors.Is(e, vector.ErrLimit) {
		t.Fatal(e)
	}
	if _, e := SparseValueEntries(strings.Repeat(" ", vector.MaxTextBytes+1)); !errors.Is(e, vector.ErrLimit) {
		t.Fatal(e)
	}
	huge := append([]byte(nil), empty...)
	binary.BigEndian.PutUint32(huge[12:], math.MaxUint32)
	if _, e := SparseValueEntries(huge); !errors.Is(e, vector.ErrEncoding) {
		t.Fatal(e)
	}
}

func TestSparseReviewClosedTextAndBlobSignatures(t *testing.T) {
	text := map[string]any{"s": "{1:3,3:-4}/3", "v": "[3,0,-4]", "h": "[3,0,-4]", "b": "101"}
	types := map[string]string{"s": "SPARSEVEC(3)", "v": "VECTOR(3)", "h": "HALFVEC(3)", "b": "BIT(3)"}
	ec := &EvalContext{Row: text, DeclaredType: func(_, n string) string { return types[n] }}
	for _, expr := range []string{"s", "+s", "coalesce(NULL,s)", "ifnull(s,NULL)", "nullif(s,NULL)"} {
		ast, e := ParseExpr(expr)
		if e != nil || VectorExpressionRepresentation(ast, ec.DeclaredType) != "sparsevec" {
			t.Fatal(expr, e)
		}
		for _, fn := range []string{"l2_distance", "l1_distance", "negative_inner_product", "inner_product", "cosine_distance"} {
			q := fn + "(" + expr + ",'{1:3,3:-4}/3')"
			want := float64(0)
			if fn == "negative_inner_product" {
				want = -25
			}
			if fn == "inner_product" {
				want = 25
			}
			if got := evalText(t, q, ec); got != want {
				t.Fatal(q, got, want)
			}
		}
	}
	for _, other := range []string{"v", "h", "b"} {
		for _, fn := range []string{"l2_distance", "l1_distance", "negative_inner_product", "inner_product", "cosine_distance"} {
			for _, args := range []string{"s," + other, other + ",s", "coalesce(s," + other + "),s", "ifnull(" + other + ",s),s"} {
				q := fn + "(" + args + ")"
				ast, e := ParseExpr(q)
				if e != nil {
					t.Fatal(e)
				}
				if _, e = Eval(ast, ec); e == nil {
					t.Fatal(q)
				}
			}
		}
	}
	for _, q := range []string{"subvector(s,1,2)", "binary_quantize(s)"} {
		ast, _ := ParseExpr(q)
		_, e := Eval(ast, ec)
		var unsupported *ErrUnsupported
		if !errors.As(e, &unsupported) {
			t.Fatal(q, e)
		}
	}
	for _, q := range []string{"vector(s)", "halfvec(s)", "bit_vector(s)", "sparsevec(v)", "sparsevec(h)", "sparsevec(b)", "sparsevec_from_vector(s)", "vector_from_sparsevec(v)"} {
		ast, _ := ParseExpr(q)
		if _, e := Eval(ast, ec); !errors.Is(e, vector.ErrInvalid) {
			t.Fatal(q, e)
		}
	}
	if got := evalText(t, "l2_distance('[3,0,-4]','[3,0,-4]')", ec); got != float64(0) {
		t.Fatal(got)
	}
	blob, e := BuiltinFunctions["sparsevec"]([]any{text["s"]})
	if e != nil {
		t.Fatal(e)
	}
	for _, other := range []string{"vector", "halfvec", "bit_vector"} {
		arg := "[3,0,-4]"
		if other == "bit_vector" {
			arg = "101"
		}
		ob, e := BuiltinFunctions[other]([]any{arg})
		if e != nil {
			t.Fatal(e)
		}
		for _, pair := range [][]any{{blob, ob}, {ob, blob}} {
			if _, e := BuiltinFunctions["l2_distance"](pair); !errors.Is(e, vector.ErrInvalid) {
				t.Fatal(other, e)
			}
		}
	}
	// SQL NULL propagation happens before untyped parameter decoding.
	for _, fn := range []string{"l2_distance", "l1_distance", "negative_inner_product", "inner_product", "cosine_distance"} {
		for _, pair := range [][]any{{nil, "malformed"}, {"malformed", nil}} {
			if got, e := BuiltinFunctions[fn](pair); e != nil || got != nil {
				t.Fatal(fn, got, e)
			}
		}
	}
	dense := evalText(t, "vector_from_sparsevec(s)", ec)
	if got := evalText(t, "sparsevec_from_vector(vector_from_sparsevec(s))", ec); !reflect.DeepEqual(got, blob) {
		t.Fatal(got, blob)
	}
	if got, e := BuiltinFunctions["vector_to_text"]([]any{dense}); e != nil || got != "[3,0,-4]" {
		t.Fatal(got, e)
	}
}

type sparseReviewCancelContext struct {
	context.Context
	checks atomic.Int64
	after  int64
}

func (c *sparseReviewCancelContext) Err() error {
	if c.checks.Add(1) >= c.after {
		return context.Canceled
	}
	return c.Context.Err()
}

func TestSparseReviewSupportBoundedArithmeticCancellation(t *testing.T) {
	entries := make([]vector.SparseEntry, vector.MaxSparseNonzeros)
	for i := range entries {
		entries[i] = vector.SparseEntry{Index: uint32(i * 60000), Value: float32(i%7 + 1)}
	}
	a, e := vector.NewSparse(1000000000, entries)
	if e != nil {
		t.Fatal(e)
	}
	b, e := vector.NewSparse(1000000000, []vector.SparseEntry{{Index: 999999999, Value: 3}})
	if e != nil {
		t.Fatal(e)
	}
	for _, m := range []vector.MetricKind{vector.L2, vector.L1, vector.NegativeIP, vector.Cosine} {
		ctx := &sparseReviewCancelContext{Context: context.Background(), after: 4}
		if got, e := m.DistanceSparseContext(ctx, a, b); !errors.Is(e, context.Canceled) || got != 0 || ctx.checks.Load() != 4 {
			t.Fatal(m, got, e, ctx.checks.Load())
		}
		complete := &sparseReviewCancelContext{Context: context.Background(), after: 100000}
		if _, e := m.DistanceSparseContext(complete, a, b); e != nil || complete.checks.Load() > 66 {
			t.Fatal(m, e, complete.checks.Load())
		}
	}
	ctx := &sparseReviewCancelContext{Context: context.Background(), after: 4}
	if got, e := vector.SparseNormContext(ctx, a); !errors.Is(e, context.Canceled) || got != 0 || ctx.checks.Load() != 4 {
		t.Fatal(got, e, ctx.checks.Load())
	}
	if _, e := vector.MetricKind("hamming").DistanceSparse(a, b); !errors.Is(e, vector.ErrMetric) {
		t.Fatal(e)
	}
	if _, e := vector.L2.DistanceSparseContext(nil, a, b); !errors.Is(e, vector.ErrContext) {
		t.Fatal(e)
	}
	// Empty billion-dimensional arithmetic uses no dimension-sized scratch.
	empty, e := vector.NewSparse(1000000000, nil)
	if e != nil {
		t.Fatal(e)
	}
	for _, m := range []vector.MetricKind{vector.L2, vector.L1, vector.NegativeIP} {
		if got, e := m.DistanceSparse(empty, empty); e != nil || got != 0 {
			t.Fatal(m, got, e)
		}
	}
	if _, e := vector.Cosine.DistanceSparse(empty, b); !errors.Is(e, vector.ErrZeroNorm) {
		t.Fatal(e)
	}
	if _, e := vector.NewSparse(1000000000, append(entries, vector.SparseEntry{})); !errors.Is(e, vector.ErrLimit) {
		t.Fatal(e)
	}
}
