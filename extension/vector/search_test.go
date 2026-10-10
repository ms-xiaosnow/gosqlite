package vector_test

import (
	"context"
	"errors"
	"math"
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

func sortedReference(query vector.Vector, rows []vector.Row, metric vector.MetricKind, limit int) []vector.Hit {
	hits := make([]vector.Hit, 0, len(rows))
	for _, r := range rows {
		if !r.Null {
			hits = append(hits, vector.Hit{RowID: r.RowID, Distance: referenceMetrics(query, r.Vector)[metric.Name()]})
		}
	}
	slices.SortFunc(hits, func(a, b vector.Hit) int {
		if a.Distance < b.Distance {
			return -1
		}
		if a.Distance > b.Distance {
			return 1
		}
		if a.RowID < b.RowID {
			return -1
		}
		if a.RowID > b.RowID {
			return 1
		}
		return 0
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits
}

func TestVectorTopKIndependentAndOrder(t *testing.T) {
	rng := rand.New(rand.NewPCG(55, 91))
	q := must(t, "[7,13,-2,9,16]")
	rows := make([]vector.Row, 97)
	for i := range rows {
		v := make([]float32, 5)
		for j := range v {
			v[j] = float32(rng.IntN(2001) - 1000)
		}
		vec, _ := vector.New(v)
		rows[i] = vector.Row{RowID: int64(i) - 47, Vector: vec}
	}
	for _, m := range []vector.MetricKind{vector.L2, vector.L1, vector.Cosine, vector.NegativeIP} {
		for _, k := range []int{1, 3, 10, 97, 120} {
			want := sortedReference(q, rows, m, k)
			for repeat := 0; repeat < 4; repeat++ {
				rng.Shuffle(len(rows), func(i, j int) { rows[i], rows[j] = rows[j], rows[i] })
				got, err := vector.TopK(context.Background(), q, rows, vector.SearchOptions{Metric: m, Limit: k})
				if err != nil || len(got.Hits) != len(want) {
					t.Fatal(m, k, got, err)
				}
				for i, h := range got.Hits {
					if h.RowID != want[i].RowID || !closeFloat(h.Distance, want[i].Distance, 3e-15, 2e-15) {
						t.Fatalf("%s k=%d at=%d got=%v want=%v", m, k, i, h, want[i])
					}
				}
				if got.Stats.Visited != len(rows) || got.Stats.Scored != len(rows) {
					t.Fatal(got.Stats)
				}
			}
		}
	}
}

func TestVectorTopKTiesFilterNullAndNegativeIP(t *testing.T) {
	q := must(t, "[1,0]")
	rows := []vector.Row{{RowID: 9, Vector: q}, {RowID: 2, Vector: q}, {RowID: -7, Vector: q}, {RowID: 0, Vector: q}, {RowID: 99, Null: true}, {RowID: 80, Vector: must(t, "[1]")}}
	opt := vector.SearchOptions{Metric: vector.L2, Limit: 3, Filter: func(_ context.Context, r vector.Row) (bool, error) { return r.RowID != 80, nil }}
	got, err := vector.TopK(context.Background(), q, rows, opt)
	if err != nil {
		t.Fatal(err)
	}
	want := []vector.Hit{{RowID: -7, Distance: 0}, {RowID: 0, Distance: 0}, {RowID: 2, Distance: 0}}
	if !reflect.DeepEqual(got.Hits, want) || got.Stats != (vector.ScanStats{Visited: 6, Scored: 4, Filtered: 1, Nulls: 1}) {
		t.Fatal(got)
	}
	if result, err := vector.TopK(context.Background(), q, []vector.Row{{RowID: 1, Null: true}}, vector.SearchOptions{Metric: vector.L2, Limit: 2}); err != nil || len(result.Hits) != 0 || result.Stats.Nulls != 1 {
		t.Fatal(result, err)
	}
	got, err = vector.TopK(context.Background(), q, []vector.Row{{RowID: 1, Vector: must(t, "[-5,0]")}, {RowID: 2, Vector: must(t, "[3,0]")}, {RowID: 3, Vector: must(t, "[8,0]")}}, vector.SearchOptions{Metric: vector.NegativeIP, Limit: 2})
	if err != nil || !reflect.DeepEqual(got.Hits, []vector.Hit{{RowID: 3, Distance: -8}, {RowID: 2, Distance: -3}}) {
		t.Fatal(got, err)
	}
}

type testMetric struct {
	fn func(vector.Vector, vector.Vector) (float64, error)
}

func (testMetric) Name() string                                   { return "test" }
func (m testMetric) Distance(a, b vector.Vector) (float64, error) { return m.fn(a, b) }

func TestVectorSearchRejectsAndNoPartialResults(t *testing.T) {
	q := must(t, "[1,2]")
	background := context.Background()
	okRows := []vector.Row{{RowID: 1, Vector: q}, {RowID: 2, Vector: q}}
	base := vector.SearchOptions{Metric: vector.L2, Limit: 1}
	assert := func(result vector.SearchResult, err, target error) {
		t.Helper()
		if !errors.Is(err, target) || !reflect.DeepEqual(result, vector.SearchResult{}) {
			t.Fatalf("result=%+v err=%v want=%v", result, err, target)
		}
	}
	result, err := vector.TopK(nil, q, okRows, base)
	assert(result, err, vector.ErrContext)
	result, err = vector.TopK(background, vector.Vector{}, okRows, base)
	assert(result, err, vector.ErrInvalid)
	opt := base
	opt.Limit = -1
	result, err = vector.TopK(background, q, okRows, opt)
	assert(result, err, vector.ErrOptions)
	opt = base
	opt.Limit = vector.DefaultSearchLimits().MaxResults + 1
	result, err = vector.TopK(background, q, okRows, opt)
	assert(result, err, vector.ErrLimit)
	opt = base
	opt.Metric = nil
	result, err = vector.TopK(background, q, okRows, opt)
	assert(result, err, vector.ErrMetric)
	opt = base
	opt.Metric = vector.MetricKind("unknown")
	result, err = vector.TopK(background, q, okRows, opt)
	assert(result, err, vector.ErrMetric)
	opt = base
	opt.Limits = vector.SearchLimits{MaxCandidates: 1}
	result, err = vector.TopK(background, q, okRows, opt)
	assert(result, err, vector.ErrOptions)
	for _, l := range []vector.SearchLimits{{-1, 1, 1}, {1, 0, 1}, {1, 1, 0}, {1<<20 + 1, 1, 1}, {1, 1<<20 + 1, 1}, {1, 1, 1<<34 + 1}} {
		opt = base
		opt.Limits = l
		result, err = vector.TopK(background, q, okRows, opt)
		assert(result, err, vector.ErrOptions)
	}
	opt = base
	opt.Limits = vector.SearchLimits{MaxCandidates: 1, MaxResults: 1, MaxComponents: 100}
	result, err = vector.TopK(background, q, okRows, opt)
	assert(result, err, vector.ErrLimit)
	opt = base
	opt.Limits = vector.SearchLimits{MaxCandidates: 2, MaxResults: 1, MaxComponents: 5}
	result, err = vector.TopK(background, q, okRows, opt)
	assert(result, err, vector.ErrLimit)
	opt = base
	opt.Limits = vector.SearchLimits{MaxCandidates: 2, MaxResults: 1, MaxComponents: 1}
	result, err = vector.TopK(background, q, okRows, opt)
	assert(result, err, vector.ErrLimit)
	result, err = vector.TopK(background, q, []vector.Row{okRows[0], {RowID: 2}}, base)
	assert(result, err, vector.ErrInvalid)
	result, err = vector.TopK(background, q, []vector.Row{okRows[0], {RowID: 2, Vector: must(t, "[1]")}}, base)
	assert(result, err, vector.ErrDimensions)
	for _, duplicate := range []vector.Row{{RowID: 1, Vector: q}, {RowID: 1, Null: true}} {
		result, err = vector.TopK(background, q, []vector.Row{okRows[0], duplicate}, base)
		assert(result, err, vector.ErrDuplicateRowID)
	}
	opt = base
	opt.Filter = func(context.Context, vector.Row) (bool, error) { return false, nil }
	result, err = vector.TopK(background, q, []vector.Row{okRows[0], okRows[0]}, opt)
	assert(result, err, vector.ErrDuplicateRowID)
	opt = base
	opt.Metric = vector.Cosine
	result, err = vector.TopK(background, q, []vector.Row{okRows[0], {RowID: 2, Vector: must(t, "[0,0]")}}, opt)
	assert(result, err, vector.ErrZeroNorm)
	result, err = vector.TopK(background, must(t, "[0,0]"), nil, opt)
	assert(result, err, vector.ErrZeroNorm)
	for _, nonfinite := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		opt = base
		opt.Metric = testMetric{func(vector.Vector, vector.Vector) (float64, error) { return nonfinite, nil }}
		result, err = vector.TopK(background, q, okRows, opt)
		assert(result, err, vector.ErrNonFinite)
	}
	boom := errors.New("callback failure")
	opt = base
	opt.Filter = func(context.Context, vector.Row) (bool, error) { return false, boom }
	result, err = vector.TopK(background, q, okRows, opt)
	assert(result, err, boom)
	opt = base
	opt.Metric = testMetric{func(vector.Vector, vector.Vector) (float64, error) { return 0, boom }}
	result, err = vector.TopK(background, q, okRows, opt)
	assert(result, err, boom)
	result, err = vector.Scan(background, q, func(context.Context) (vector.Row, bool, error) { return vector.Row{}, false, boom }, base)
	assert(result, err, boom)
	result, err = vector.Scan(background, q, nil, base)
	assert(result, err, vector.ErrOptions)
	// Exhaustion at exactly the declared budgets is valid.
	opt = base
	opt.Limits = vector.SearchLimits{MaxCandidates: 2, MaxResults: 1, MaxComponents: 6}
	result, err = vector.TopK(background, q, okRows, opt)
	if err != nil || len(result.Hits) != 1 {
		t.Fatal(result, err)
	}
}

func TestVectorSearchCancellation(t *testing.T) {
	q := must(t, "[1]")
	base := vector.SearchOptions{Metric: vector.L2, Limit: 1}
	for _, stage := range []string{"before", "source", "filter", "metric", "exhaustion"} {
		t.Run(stage, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if stage == "before" {
				cancel()
			}
			i := 0
			next := func(context.Context) (vector.Row, bool, error) {
				i++
				if stage == "source" {
					cancel()
				}
				if i > 2 {
					if stage == "exhaustion" {
						cancel()
					}
					return vector.Row{}, false, nil
				}
				return vector.Row{RowID: int64(i), Vector: q}, true, nil
			}
			opt := base
			if stage == "filter" {
				opt.Filter = func(context.Context, vector.Row) (bool, error) { cancel(); return true, nil }
			}
			if stage == "metric" {
				opt.Metric = testMetric{func(a, b vector.Vector) (float64, error) { cancel(); return 0, nil }}
			}
			result, err := vector.Scan(ctx, q, next, opt)
			if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, vector.SearchResult{}) {
				t.Fatal(result, err)
			}
			if stage == "before" && i != 0 {
				t.Fatal("called source while canceled")
			}
		})
	}
}

func TestVectorSearchZeroLimitAndFiniteSourceBudget(t *testing.T) {
	q := must(t, "[1]")
	called := 0
	next := func(context.Context) (vector.Row, bool, error) {
		called++
		return vector.Row{RowID: int64(called), Vector: q}, true, nil
	}
	result, err := vector.Scan(context.Background(), q, next, vector.SearchOptions{Metric: vector.L2, Limit: 0})
	if err != nil || called != 0 || len(result.Hits) != 0 {
		t.Fatal(result, err, called)
	}
	result, err = vector.Scan(context.Background(), q, next, vector.SearchOptions{Metric: vector.L2, Limit: 1, Limits: vector.SearchLimits{MaxCandidates: 4, MaxResults: 1, MaxComponents: 100}})
	if !errors.Is(err, vector.ErrLimit) || !reflect.DeepEqual(result, vector.SearchResult{}) || called != 5 {
		t.Fatal(result, err, called)
	}
}

func TestVectorSearchMetricPointer(t *testing.T) {
	q := must(t, "[1]")
	for _, m := range []vector.Metric{(*vector.MetricKind)(nil), (*testMetric)(nil)} {
		if result, err := vector.TopK(context.Background(), q, nil, vector.SearchOptions{Metric: m, Limit: 0}); !errors.Is(err, vector.ErrMetric) || !reflect.DeepEqual(result, vector.SearchResult{}) {
			t.Fatal(result, err)
		}
	}
	kind := vector.L2
	if result, err := vector.TopK(context.Background(), q, []vector.Row{{RowID: 1, Vector: q}}, vector.SearchOptions{Metric: &kind, Limit: 1}); err != nil || len(result.Hits) != 1 || result.Hits[0].Distance != 0 {
		t.Fatal(result, err)
	}
	kind = vector.Cosine
	if _, err := vector.TopK(context.Background(), must(t, "[0]"), nil, vector.SearchOptions{Metric: &kind, Limit: 0}); !errors.Is(err, vector.ErrZeroNorm) {
		t.Fatal(err)
	}
}
