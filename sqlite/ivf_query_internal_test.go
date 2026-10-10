package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/extension/vector/ann"
)

// Pure in-memory typed boundary tests; no persisted state is modified.
func TestIVFQueryTypedMemberRecheck(t *testing.T) {
	for _, value := range []any{nil, "invalid", "[1]", int64(2), []byte{1, 2, 3}} {
		if _, err := decodeANNCandidate(value, 2, nil); !errors.Is(err, ErrCorrupt) {
			t.Fatal(value, err)
		}
	}
	v, err := decodeANNCandidate("[100,2]", 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	q, _ := vector.New([]float32{1, 0})
	distance, err := vector.NegativeIP.Distance(q, v)
	if err != nil || distance != -100 {
		t.Fatal(distance, err)
	}
}

// The storage reader is deliberately nil: a persisted memory cap must reject
// this bounded centroid allocation before any database read or allocation.
func TestIVFQueryPersistedMemoryAdmissionBeforeCentroidRead(t *testing.T) {
	cfg, err := (ann.Config{Method: ann.IVFFlat, Dimensions: 2000, Metric: vector.L2, Lists: 100, MaxMemoryBytes: 1024}).Normalize()
	if err != nil {
		t.Fatal(err)
	}
	q, err := vector.New(make([]float32, 2000))
	if err != nil {
		t.Fatal(err)
	}
	o, err := normalizeVectorANNOptions(VectorSearchOptions{Mode: VectorModeApproximate, Limit: 1, ANNLimits: ANNWorkLimits{MaxBytes: 64 << 20}})
	if err != nil {
		t.Fatal(err)
	}
	x := &extensionIndex{ann: &cfg, method: ann.IVFFlat}
	o, err = annIndexQueryOptions(x, o)
	if err != nil {
		t.Fatal(err)
	}
	a := annCandidateQuery{db: &DB{opts: Options{MaxSortBytes: 64 << 20}}, index: x, query: q, retain: 1, options: o, limits: vector.DefaultSearchLimits(), plan: &QueryPlan{}, stats: &QueryStats{}, components: func() int64 { return 2000 }, retainedBytes: func() int64 { return 0 }}
	_, _, err = a.runIVF(context.Background(), newANNStore(nil, x), ann.State{Live: 1}, nil)
	if !errors.Is(err, ErrMemoryLimit) || a.stats.ANNCentroidReads != 0 {
		t.Fatal(err, a.stats)
	}
}

func TestIVFQueryExactFallbackSourceAdmission(t *testing.T) {
	for _, tc := range []struct {
		count, bound       int64
		exhausted, corrupt bool
	}{
		{0, 0, true, false}, {2, 2, false, false}, {2, 2, true, false},
		{3, 2, false, true}, {1, 2, true, true}, {3, -1, true, false},
	} {
		err := checkANNExactSourceCount(tc.count, tc.bound, tc.exhausted)
		if errors.Is(err, ErrCorrupt) != tc.corrupt {
			t.Fatal(tc, err)
		}
	}
}
