package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

// These independent regressions only create ordinary source exports, import,
// committed pause/resume, and read intact native databases. Error controls are
// in-memory format/math/options. No physical faults or recovery are exercised.

// Rational sums differ from the production reference's floating accumulation.
// Only irrational sqrt/division is rounded, using independent 1024-bit floats.
func independentReviewDistance(a, b []float32, metric vector.MetricKind) float64 {
	rat := func(x float32) *big.Rat { return new(big.Rat).SetFloat64(float64(x)) }
	dot, aa, bb, l1, l2 := new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat)
	for i, x := range a {
		av, bv := rat(x), rat(b[i])
		d := new(big.Rat).Sub(av, bv)
		dot.Add(dot, new(big.Rat).Mul(av, bv))
		aa.Add(aa, new(big.Rat).Mul(av, av))
		bb.Add(bb, new(big.Rat).Mul(bv, bv))
		l1.Add(l1, new(big.Rat).Abs(d))
		l2.Add(l2, new(big.Rat).Mul(d, d))
	}
	f := func(r *big.Rat) *big.Float { return new(big.Float).SetPrec(1024).SetRat(r) }
	var v *big.Float
	switch metric {
	case vector.L1:
		v = f(l1)
	case vector.L2:
		v = f(l2)
		v.Sqrt(v)
	case vector.NegativeIP:
		v = f(dot)
		v.Neg(v)
	case vector.Cosine:
		den := f(new(big.Rat).Mul(aa, bb))
		den.Sqrt(den)
		v = f(dot)
		v.Quo(v, den)
		v.Sub(new(big.Float).SetPrec(1024).SetInt64(1), v)
	default:
		panic("unsupported review metric")
	}
	n, _ := v.Float64()
	if metric == vector.Cosine {
		n = max(0, min(2, n))
	}
	return n
}

func TestIndependentImportDistanceRationalOracle(t *testing.T) {
	// Full exponent span, both product signs, subnormals, cancellation, tiny norms.
	for _, dims := range []int{1, 3, 17, 129, 16000} {
		a, b := make([]float32, dims), make([]float32, dims)
		for i := range a {
			bits := uint32((i*37)%255)<<23 | uint32((i*104729+1)&0x7fffff)
			if bits == 0 {
				bits = 1
			}
			if i%2 == 0 {
				bits |= 1 << 31
			}
			a[i] = math.Float32frombits(bits)
			bits = uint32((i*67+13)%255)<<23 | uint32((i*65537+3)&0x7fffff)
			if i%3 == 0 {
				bits |= 1 << 31
			}
			b[i] = math.Float32frombits(bits)
		}
		for _, metric := range []vector.MetricKind{vector.L1, vector.L2, vector.NegativeIP, vector.Cosine} {
			want := independentReviewDistance(a, b, metric)
			got := importDistanceReference(a, b, metric)
			if got != want {
				t.Fatalf("dims=%d metric=%s production reference=%.17g rational=%.17g", dims, metric, got, want)
			}
			av, _ := vector.New(a)
			bv, _ := vector.New(b)
			native, err := metric.Distance(av, bv)
			bound := importDistanceErrorBound(a, b, metric, want)
			if err != nil || !importDistanceClose(native, want, bound) {
				t.Fatalf("native bound dims=%d metric=%s native=%.17g rational=%.17g bound=%g err=%v", dims, metric, native, want, bound, err)
			}
		}
	}
	maxf, tiny := float32(math.MaxFloat32), float32(math.SmallestNonzeroFloat32)
	a, b := []float32{maxf, tiny, maxf}, []float32{maxf, tiny, -maxf}
	if want := independentReviewDistance(a, b, vector.NegativeIP); want != -float64(tiny)*float64(tiny) || importDistanceReference(a, b, vector.NegativeIP) != want {
		t.Fatal("lost low exponent cancellation residual", want)
	}
}

func TestIndependentImportDistanceMaxDimensionsAndReadOnly(t *testing.T) {
	const dims = 16000
	var data strings.Builder
	source := map[int64][]float32{}
	for _, pk := range []int64{math.MaxInt64, 17, math.MinInt64} {
		v := make([]float32, dims)
		for j := range v {
			v[j] = float32(j%31-15) / 16
		}
		v[0] = float32(pk%7 + 1)
		source[pk] = v
		fmt.Fprintf(&data, "{\"id\":%d,\"embedding\":[", pk)
		for j, x := range v {
			if j > 0 {
				data.WriteByte(',')
			}
			fmt.Fprintf(&data, "%.9g", x)
		}
		data.WriteString("]}\n")
	}
	m, in, out := importFixture(t, "ndjson", data.String())
	m.Fields[0].Target = "mapped_pk"
	m.Fields[1].Target = "mapped_vector"
	m.Fields[1].Dimensions = dims
	ctx := context.Background()
	paused, err := Import(ctx, in, out, m, ImportOptions{Apply: true, BatchRows: 1, StopAfterBatches: 1})
	if err != nil || paused.Status != "paused_committed" {
		t.Fatal(paused, err)
	}
	if _, err = Import(ctx, in, out, m, ImportOptions{Apply: true, Resume: true, BatchRows: 1}); err != nil {
		t.Fatal(err)
	}
	hash := func(p string) [32]byte {
		b, e := os.ReadFile(p)
		if e != nil {
			t.Fatal(e)
		}
		return sha256.Sum256(b)
	}
	sourceBefore, dbBefore := hash(in), hash(out)
	opts := ImportDistanceOptions{Column: "mapped_vector", QueryCount: 3, TopK: 2}
	got, err := VerifyImportDistances(ctx, in, out, m, opts)
	if err != nil {
		t.Fatal(err)
	}
	r := got.Distances
	if r.Status != "passed" || r.QueryCoverage != "all_source_vectors_as_queries_all_candidates" || r.NormsChecked != 3 || r.NativeRowsChecked != 3 || r.DistancesChecked != 36 || r.TopKChecked != 24 || r.RetainedComponents != 48000 || r.PlannedComponentWork != 2784000 {
		t.Fatalf("report %+v", r)
	}
	if !reflect.DeepEqual(r.QueryPKs, []int64{math.MinInt64, 17, math.MaxInt64}) {
		t.Fatal(r.QueryPKs)
	}
	for _, q := range r.Queries {
		for _, h := range q.All {
			want := independentReviewDistance(source[h.PK], source[q.QueryPK], q.Metric)
			if h.Reference != want || math.Abs(h.Native-want) > h.Tolerance {
				t.Fatalf("query=%+v hit=%+v independent=%.17g", q, h, want)
			}
		}
		if !reflect.DeepEqual(q.TopK, q.All[:2]) {
			t.Fatal("top-k prefix differs")
		}
	}
	if hash(in) != sourceBefore || hash(out) != dbBefore {
		t.Fatal("verification changed ordinary source/database bytes")
	}
	plain, err := VerifyImport(ctx, in, out, m)
	if err != nil || !strings.HasPrefix(plain.ExactDistances, "not_run") || plain.CanonicalSHA256 != got.Import.CanonicalSHA256 {
		t.Fatal(plain, err)
	}
	if !strings.HasPrefix(got.Import.ANNRecall, "not_run") || !strings.HasPrefix(r.ANNRecall, "not_run") {
		t.Fatal("ANN promoted")
	}
	encoded, err := json.Marshal(got)
	if err != nil || !json.Valid(encoded) {
		t.Fatal(err)
	}
	// Exact pair/component limits pass; one fewer is an intact read-only budget error.
	opts.MaxPairs = r.PlannedComparisons
	opts.MaxComponentWork = r.PlannedComponentWork
	if _, err = VerifyImportDistances(ctx, in, out, m, opts); err != nil {
		t.Fatal("exact budget rejected", err)
	}
	opts.MaxPairs--
	bad, err := VerifyImportDistances(ctx, in, out, m, opts)
	if err == nil || bad.Distances.Status != "failed" {
		t.Fatal("under-budget accepted", err)
	}
}

func TestIndependentImportDistanceFormatsAndFalseControls(t *testing.T) {
	for _, raw := range []string{`{}`, `{"Column":"embedding"}`, `{"column":"embedding","Column":"other"}`, `{"column":"embedding","query_pks":null}`, `{"column":"embedding","metrics":[null]}`, `{"column":"embedding","query_count":1.5}`, `{"column":"embedding","query_pks":[9223372036854775808]}`, `{"column":"embedding","metrics":[["l2"]]}`, `{"column":"\ud800"}`} {
		if _, err := DecodeImportDistanceOptions(strings.NewReader(raw)); err == nil {
			t.Fatalf("invalid format accepted: %s", raw)
		}
	}
	tiny := float32(math.SmallestNonzeroFloat32)
	norm := independentReviewDistance([]float32{tiny}, []float32{0}, vector.L2)
	refs := map[int64]importDistanceReferenceValue{1: {norm, ImportDistanceTolerance * norm}}
	for _, distance := range []float64{0, math.Inf(1), math.Inf(-1), math.NaN(), norm * 2} {
		if _, err := checkImportDistanceHits([]vector.Hit{{RowID: 1, Distance: distance}}, refs); err == nil {
			t.Fatal("false distance accepted", distance)
		}
	}
	if _, err := checkImportDistanceHits([]vector.Hit{{RowID: 1, Distance: norm}}, refs); err != nil {
		t.Fatal(err)
	}
	near := []ImportDistanceHit{{PK: 1, Reference: 0, Tolerance: 2}, {PK: 2, Reference: 1, Tolerance: 0}, {PK: 3, Reference: 4, Tolerance: 0}}
	if !strings.HasPrefix(importDistanceBoundary(near, 1), "ambiguous") || !strings.HasPrefix(importDistanceBoundary(near, 2), "clear") {
		t.Fatal("boundary classification")
	}
	// A bad signed-PK tie fails even if both distances are individually accurate.
	tie := map[int64]importDistanceReferenceValue{-9: {1, 1e-14}, 7: {1, 1e-14}}
	if _, err := checkImportDistanceHits([]vector.Hit{{RowID: 7, Distance: 1}, {RowID: -9, Distance: 1}}, tie); err == nil {
		t.Fatal("bad signed-PK tie accepted")
	}
}
