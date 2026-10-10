package postgres

import (
	"context"
	"fmt"
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// The reference widens exact float32 components into 256-bit arithmetic and
// does not call the production vector distance implementation.
func importReferenceDistance(a, b []float32, metric vector.MetricKind) float64 {
	fresh := func() *big.Float { return new(big.Float).SetPrec(256) }
	scalar := func(x float32) *big.Float { return fresh().SetFloat64(float64(x)) }
	dot, normA, normB, sum := fresh(), fresh(), fresh(), fresh()
	for i, x := range a {
		av, bv := scalar(x), scalar(b[i])
		dot.Add(dot, fresh().Mul(av, bv))
		normA.Add(normA, fresh().Mul(av, av))
		normB.Add(normB, fresh().Mul(bv, bv))
		d := fresh().Sub(av, bv)
		if metric == vector.L1 {
			sum.Add(sum, fresh().Abs(d))
		} else {
			sum.Add(sum, fresh().Mul(d, d))
		}
	}
	var result *big.Float
	switch metric {
	case vector.L2:
		result = fresh().Sqrt(sum)
	case vector.L1:
		result = sum
	case vector.NegativeIP:
		result = fresh().Neg(dot)
	case vector.Cosine:
		den := fresh().Sqrt(fresh().Mul(normA, normB))
		result = fresh().Sub(fresh().SetInt64(1), fresh().Quo(dot, den))
	}
	n, _ := result.Float64()
	if metric == vector.Cosine {
		n = max(0, min(2, n))
	}
	return n
}
func TestImportExactDistancesAgainstIndependentHighPrecision(t *testing.T) {
	ctx := context.Background()
	var source strings.Builder
	values := [][]float32{}
	for i := 0; i < 32; i++ {
		v := []float32{float32(i-13) * 0.125, float32((i*i)%19+1) * 0.0625}
		values = append(values, v)
		fmt.Fprintf(&source, "{\"id\":%d,\"embedding\":[%.9g,%.9g]}\n", i, v[0], v[1])
	}
	// These retained extremes document finite local float64 arithmetic where
	// legacy float32 products/sums overflow; they are not loose-tolerance passes.
	values = append(values, []float32{math.MaxFloat32, -math.MaxFloat32}, []float32{math.SmallestNonzeroFloat32, -math.SmallestNonzeroFloat32})
	for i := 32; i < len(values); i++ {
		fmt.Fprintf(&source, "{\"id\":%d,\"embedding\":[%.9g,%.9g]}\n", i, values[i][0], values[i][1])
	}
	m, in, out := importFixture(t, "ndjson", source.String())
	m.RowOrder = "ascending"
	if _, err := Import(ctx, in, out, m, ImportOptions{Apply: true, BatchRows: 7}); err != nil {
		t.Fatal(err)
	}
	db, err := sqlite.Open(out, sqlite.Options{ReadOnly: true, NoCreate: true})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	q := []float32{0.3125, -0.6875}
	query, _ := vector.New(q)
	for _, metric := range []vector.MetricKind{vector.L2, vector.L1, vector.NegativeIP, vector.Cosine} {
		want := make([]float64, len(values))
		for i, v := range values {
			want[i] = importReferenceDistance(v, q, metric)
		}
		got, e := db.VectorSearch(ctx, "items", "embedding", query, sqlite.VectorSearchOptions{Metric: metric, Limit: len(values)})
		if e != nil {
			t.Fatal(metric, e)
		}
		if !got.Plan.Exact || len(got.Hits) != len(want) {
			t.Fatal(metric, got)
		}
		seen := map[int64]bool{}
		for i, hit := range got.Hits {
			if hit.RowID < 0 || hit.RowID >= int64(len(want)) || seen[hit.RowID] {
				t.Fatalf("%s invalid or duplicate hit %d", metric, hit.RowID)
			}
			seen[hit.RowID] = true
			ref := want[hit.RowID]
			tol := 1e-12 * max(1, math.Abs(ref))
			if math.IsInf(hit.Distance, 0) || math.IsNaN(hit.Distance) || math.Abs(hit.Distance-ref) > tol {
				t.Fatalf("%s id=%d distance=%.17g independent=%.17g tolerance=%.3g", metric, hit.RowID, hit.Distance, ref, tol)
			}
			if i > 0 {
				prior := got.Hits[i-1]
				if prior.Distance > hit.Distance || prior.Distance == hit.Distance && prior.RowID >= hit.RowID {
					t.Fatalf("%s local exact distance/signed-PK order", metric)
				}
				// Collinear max/subnormal vectors have mathematically equal
				// cosine distance. Float64 normalization can separate them by
				// roundoff; require reference ordering only beyond both bounds.
				priorRef := want[prior.RowID]
				priorTol := 1e-12 * max(1, math.Abs(priorRef))
				if priorRef-ref > priorTol+tol {
					t.Fatalf("%s independent ordering beyond numeric tie", metric)
				}
			}
		}
	}
}
