package postgres

import (
	"context"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

// Valid finite binary32 input only. The main Neumaier sum remains 1 until
// the final -1; swallowed small products collect in the separately rounded
// correction. Greedy deterministic selection exposes its accumulated error.
func independentReviewCancellationInputs() ([]float32, []float32) {
	const dims = 16000
	a, b := make([]float32, dims), make([]float32, dims)
	a[0], b[0], a[dims-1], b[dims-1] = 1, 1, 1, -1
	state := uint32(100)
	correction := 0.0
	for i := 1; i < dims-1; i++ {
		a[i] = math.Float32frombits(0x327fffff)
		bestErr := math.Inf(-1)
		var best float32
		var bestSum float64
		for j := 0; j < 12; j++ {
			state = 1664525*state + 1013904223
			candidate := math.Float32frombits(0x31800000 | (state & 0x7fffff))
			product := float64(a[i]) * float64(candidate)
			next := correction + product
			rounding := (next - correction) - product
			if rounding > bestErr {
				bestErr, best, bestSum = rounding, candidate, next
			}
		}
		b[i] = best
		correction = bestSum
	}
	return a, b
}

func TestIndependentImportDistanceCancellationBound(t *testing.T) {
	a, b := independentReviewCancellationInputs()
	av, _ := vector.New(a)
	bv, _ := vector.New(b)
	native, err := vector.NegativeInnerProduct(av, bv)
	if err != nil {
		t.Fatal(err)
	}
	reference := independentReviewDistance(a, b, vector.NegativeIP)
	if got := importDistanceReference(a, b, vector.NegativeIP); got != reference {
		t.Fatal("production reference disagrees with rational sum", got, reference)
	}
	bound := importDistanceErrorBound(a, b, vector.NegativeIP, reference)
	actual := math.Abs(native - reference)
	t.Logf("dims=%d native=%.17g rational=%.17g absolute_error=%.17g reported_bound=%.17g ratio=%.9g", len(a), native, reference, actual, bound, actual/bound)
	if actual > bound {
		t.Fatalf("valid native compensated IP exceeds verifier's claimed bound by %.9gx", actual/bound)
	}
}

func TestIndependentImportDistanceCancellationDataset(t *testing.T) {
	a, b := independentReviewCancellationInputs()
	var data strings.Builder
	for row, v := range [][]float32{a, b} {
		fmt.Fprintf(&data, "{\"id\":%d,\"embedding\":[", row+1)
		for i, x := range v {
			if i > 0 {
				data.WriteByte(',')
			}
			fmt.Fprintf(&data, "%.9g", x)
		}
		data.WriteString("]}\n")
	}
	m, in, out := importFixture(t, "ndjson", data.String())
	m.Fields[1].Dimensions = len(a)
	ctx := context.Background()
	if _, err := Import(ctx, in, out, m, ImportOptions{Apply: true}); err != nil {
		t.Fatal(err)
	}
	report, err := VerifyImportDistances(ctx, in, out, m, ImportDistanceOptions{Column: "embedding", QueryPKs: []int64{1}, Metrics: []vector.MetricKind{vector.NegativeIP}, TopK: 2})
	if err != nil || report.Distances.Status != "passed" {
		t.Fatalf("ordinary intact finite dataset incorrectly rejected: status=%s error=%v", report.Distances.Status, err)
	}
}
