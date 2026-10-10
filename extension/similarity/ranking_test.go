package similarity_test

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"reflect"
	"sort"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/grams13"
	sim "github.com/ms-xiaosnow/gosqlite/extension/similarity"
)

func source(rows []sim.Document) func(context.Context) (sim.Document, bool, error) {
	at := 0
	return func(context.Context) (sim.Document, bool, error) {
		if at == len(rows) {
			return sim.Document{}, false, nil
		}
		row := rows[at]
		at++
		return row, true, nil
	}
}

func TestScanDeterministicNullAndEmpty(t *testing.T) {
	e := defaultEngine(t)
	ctx := context.Background()
	rows := []sim.Document{{RowID: 9, Text: "bc"}, {RowID: 7, Text: "abc"}, {RowID: -5, Text: "abc"}, {RowID: 1, Text: "x"}, {RowID: 0, Null: true}, {RowID: math.MaxInt64, Text: "xy"}, {RowID: math.MinInt64, Text: "xz"}}
	result, err := e.Scan(ctx, "abc", source(rows), sim.SearchOptions{Metric: sim.Dice, Threshold: 0, Limit: 7})
	if err != nil {
		t.Fatal(err)
	}
	wantIDs := []int64{-5, 7, 9, math.MinInt64, 1, math.MaxInt64}
	ids := []int64{}
	for _, h := range result.Hits {
		ids = append(ids, h.RowID)
	}
	if !reflect.DeepEqual(ids, wantIDs) || result.Stats != (sim.ScanStats{Mode: sim.FullScan, Visited: 7, Scored: 6, Matched: 6}) {
		t.Fatal(ids, result.Stats)
	}
	result, err = e.Scan(ctx, "", source(rows), sim.SearchOptions{Metric: sim.Jaccard, Threshold: 1, Limit: 7})
	if err != nil || len(result.Hits) != 1 || result.Hits[0].RowID != 1 {
		t.Fatal(result, err)
	}
	called := false
	result, err = e.Scan(ctx, "", func(context.Context) (sim.Document, bool, error) { called = true; return sim.Document{}, false, nil }, sim.SearchOptions{Metric: sim.Dice, Limit: 0})
	if err != nil || called || len(result.Hits) != 0 {
		t.Fatal(result, err, called)
	}
	result, err = e.Scan(ctx, "abc", source(nil), sim.SearchOptions{Metric: sim.Dice, Limit: 10})
	if err != nil || len(result.Hits) != 0 || result.Stats.Visited != 0 {
		t.Fatal(result, err)
	}
}

func TestScanExactTopKReference(t *testing.T) {
	r := rand.New(rand.NewSource(812412))
	ctx := context.Background()
	for round := 0; round < 200; round++ {
		c := sim.DefaultConfig()
		c.MinGram = 1 + round%3
		c.MaxGram = 3
		e := engine(t, c)
		query := randomText(r, 10)
		// Use shared normalization only; token maps and final full sort below
		// are independent of engine's merge intersection and bounded heap.
		normalized := func(s string) string {
			// Prepare's tokens are deliberately not used by the reference.
			return normalizeReference(t, c.Tokenizer, s)
		}
		nq := normalized(query)
		rows := make([]sim.Document, 1+r.Intn(80))
		for i := range rows {
			rows[i] = sim.Document{RowID: int64(i) - 40, Text: randomText(r, 12), Null: r.Intn(11) == 0}
		}
		r.Shuffle(len(rows), func(i, j int) { rows[i], rows[j] = rows[j], rows[i] })
		m := sim.Dice
		if round%2 == 0 {
			m = sim.Jaccard
		}
		threshold := []float64{0, .1, .25, .5, 1}[round%5]
		k := 1 + r.Intn(20)
		expected := []sim.Hit{}
		scored := 0
		for _, row := range rows {
			if row.Null {
				continue
			}
			scored++
			s := referenceScore(nq, normalized(row.Text), c.MinGram, c.MaxGram, m)
			if s.Value >= threshold {
				expected = append(expected, sim.Hit{RowID: row.RowID, Score: s})
			}
		}
		matched := len(expected)
		sort.Slice(expected, func(i, j int) bool {
			a, b := expected[i], expected[j]
			return a.Score.Value > b.Score.Value || a.Score.Value == b.Score.Value && a.RowID < b.RowID
		})
		if len(expected) > k {
			expected = expected[:k]
		}
		got, err := e.Scan(ctx, query, source(rows), sim.SearchOptions{Metric: m, Threshold: threshold, Limit: k})
		if err != nil || !reflect.DeepEqual(got.Hits, expected) || got.Stats != (sim.ScanStats{Mode: sim.FullScan, Visited: len(rows), Scored: scored, Matched: matched}) {
			t.Fatalf("round %d: got %+v %v want %+v", round, got, err, expected)
		}
	}
}

func TestScanFailureNeverPartial(t *testing.T) {
	ctx := context.Background()
	defaultC := sim.DefaultConfig()
	errSource := errors.New("source error")
	tests := []struct {
		name    string
		config  sim.Config
		next    func(context.Context) (sim.Document, bool, error)
		options sim.SearchOptions
		want    error
	}{
		{"nil source", defaultC, nil, sim.SearchOptions{Metric: sim.Dice, Limit: 1}, sim.ErrValue},
		{"negative limit", defaultC, source(nil), sim.SearchOptions{Metric: sim.Dice, Limit: -1}, sim.ErrValue},
		{"NaN", defaultC, source(nil), sim.SearchOptions{Metric: sim.Dice, Threshold: math.NaN(), Limit: 1}, sim.ErrValue},
		{"bad metric", defaultC, source(nil), sim.SearchOptions{Metric: "bad", Limit: 1}, sim.ErrValue},
		{"too many results", defaultC, source(nil), sim.SearchOptions{Metric: sim.Dice, Limit: defaultC.Limits.MaxResults + 1}, sim.ErrLimit},
		{"duplicate", defaultC, source([]sim.Document{{RowID: 1, Text: "abc"}, {RowID: 1, Text: "def"}}), sim.SearchOptions{Metric: sim.Dice, Limit: 1}, sim.ErrValue},
		{"null duplicate", defaultC, source([]sim.Document{{RowID: 1, Null: true}, {RowID: 1, Text: "def"}}), sim.SearchOptions{Metric: sim.Dice, Limit: 1}, sim.ErrValue},
		{"callback", defaultC, func(context.Context) (sim.Document, bool, error) { return sim.Document{}, false, errSource }, sim.SearchOptions{Metric: sim.Dice, Limit: 1}, errSource},
	}
	small := defaultC
	small.Limits.MaxCandidates = 1
	tests = append(tests, struct {
		name    string
		config  sim.Config
		next    func(context.Context) (sim.Document, bool, error)
		options sim.SearchOptions
		want    error
	}{"candidate budget", small, source([]sim.Document{{RowID: 1}, {RowID: 2}}), sim.SearchOptions{Metric: sim.Dice, Limit: 1}, sim.ErrLimit})
	small = defaultC
	small.Limits.MaxTotalInputBytes = 4
	tests = append(tests, struct {
		name    string
		config  sim.Config
		next    func(context.Context) (sim.Document, bool, error)
		options sim.SearchOptions
		want    error
	}{"byte budget", small, source([]sim.Document{{RowID: 1, Text: "abc"}}), sim.SearchOptions{Metric: sim.Dice, Limit: 1}, sim.ErrLimit})
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := engine(t, tt.config).Scan(ctx, "abc", tt.next, tt.options)
			if !errors.Is(err, tt.want) || !reflect.DeepEqual(got, sim.ScanResult{}) {
				t.Fatal(got, err)
			}
		})
	}
	cctx, cancel := context.WithCancel(ctx)
	got, err := defaultEngine(t).Scan(cctx, "abc", func(context.Context) (sim.Document, bool, error) {
		cancel()
		return sim.Document{RowID: 1, Text: "abc"}, true, nil
	}, sim.SearchOptions{Metric: sim.Dice, Limit: 1})
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(got, sim.ScanResult{}) {
		t.Fatal(got, err)
	}
}

func TestFusionGoldenAndDetails(t *testing.T) {
	e := defaultEngine(t)
	ctx := context.Background()
	lists := []sim.RankedList{{Name: "text", Weight: 1, RowIDs: []int64{2, 1}}, {Name: "vector", Weight: 2, RowIDs: []int64{1, 3}}}
	got, err := e.Fuse(ctx, lists, 1, 10)
	want := []sim.FusedHit{
		{RowID: 1, Score: 1.0/3 + 1, Contributions: []sim.Contribution{{List: 0, Name: "text", Rank: 2, Weight: 1, Value: 1.0 / 3}, {List: 1, Name: "vector", Rank: 1, Weight: 2, Value: 1}}},
		{RowID: 3, Score: 2.0 / 3, Contributions: []sim.Contribution{{List: 1, Name: "vector", Rank: 2, Weight: 2, Value: 2.0 / 3}}},
		{RowID: 2, Score: .5, Contributions: []sim.Contribution{{List: 0, Name: "text", Rank: 1, Weight: 1, Value: .5}}},
	}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal(got, err)
	}
	got[0].Contributions[0].Name = "mutated"
	if lists[0].Name != "text" {
		t.Fatal("mutated input")
	}
	tie := []sim.RankedList{{Weight: 1, RowIDs: []int64{math.MaxInt64, math.MinInt64}}, {Weight: 1, RowIDs: []int64{math.MinInt64, math.MaxInt64}}}
	got, err = e.Fuse(ctx, tie, 60, 1)
	if err != nil || len(got) != 1 || got[0].RowID != math.MinInt64 {
		t.Fatal("tie", got, err)
	}
	got, err = e.Fuse(ctx, nil, 60, 3)
	if err != nil || len(got) != 0 {
		t.Fatal(got, err)
	}
	got, err = e.Fuse(ctx, []sim.RankedList{{Weight: 1}}, 60, 3)
	if err != nil || len(got) != 0 {
		t.Fatal(got, err)
	}
	got, err = e.Fuse(ctx, lists, 60, 0)
	if err != nil || len(got) != 0 {
		t.Fatal(got, err)
	}
}

func TestFusionFiniteAndBudgets(t *testing.T) {
	e := defaultEngine(t)
	ctx := context.Background()
	lists := []sim.RankedList{{Weight: 1, RowIDs: []int64{1, 2}}}
	for _, number := range []float64{0, -1, math.Inf(1), math.Inf(-1), math.NaN()} {
		if got, err := e.Fuse(ctx, lists, number, 1); !errors.Is(err, sim.ErrValue) || got != nil {
			t.Fatal(got, err)
		}
		if got, err := e.Fuse(ctx, []sim.RankedList{{Weight: number}}, 60, 1); !errors.Is(err, sim.ErrValue) || got != nil {
			t.Fatal(got, err)
		}
	}
	for _, ids := range [][]int64{{1, 1}, {math.MinInt64, 1, math.MinInt64}} {
		if got, err := e.Fuse(ctx, []sim.RankedList{{Weight: 1, RowIDs: ids}}, 60, 1); !errors.Is(err, sim.ErrValue) || got != nil {
			t.Fatal(got, err)
		}
	}
	over := []sim.RankedList{{Weight: math.MaxFloat64, RowIDs: []int64{1}}, {Weight: math.MaxFloat64, RowIDs: []int64{1}}}
	if got, err := e.Fuse(ctx, over, math.SmallestNonzeroFloat64, 1); !errors.Is(err, sim.ErrValue) || got != nil {
		t.Fatal(got, err)
	}
	c := sim.DefaultConfig()
	c.Limits.MaxCandidates = 1
	if got, err := engine(t, c).Fuse(ctx, lists, 60, 1); !errors.Is(err, sim.ErrLimit) || got != nil {
		t.Fatal(got, err)
	}
	c = sim.DefaultConfig()
	c.Limits.MaxFusionLists = 1
	if got, err := engine(t, c).Fuse(ctx, []sim.RankedList{{Weight: 1}, {Weight: 1}}, 60, 1); !errors.Is(err, sim.ErrLimit) || got != nil {
		t.Fatal(got, err)
	}
	if got, err := e.Fuse(nil, lists, 60, 1); !errors.Is(err, sim.ErrContext) || got != nil {
		t.Fatal(got, err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if got, err := e.Fuse(canceled, lists, 60, 1); !errors.Is(err, context.Canceled) || got != nil {
		t.Fatal(got, err)
	}
	if got, err := e.Fuse(ctx, lists, 60, -1); !errors.Is(err, sim.ErrValue) || got != nil {
		t.Fatal(got, err)
	}
	if got, err := e.Fuse(ctx, lists, 60, e.Config().Limits.MaxResults+1); !errors.Is(err, sim.ErrLimit) || got != nil {
		t.Fatal(got, err)
	}
}

func TestFusionReferenceProperty(t *testing.T) {
	r := rand.New(rand.NewSource(92187))
	e := defaultEngine(t)
	ctx := context.Background()
	for round := 0; round < 500; round++ {
		lists := make([]sim.RankedList, r.Intn(10))
		sum := map[int64]float64{}
		offset := float64(1 + r.Intn(100))
		for i := range lists {
			lists[i].Weight = float64(1+r.Intn(10)) / 3
			ids := r.Perm(60)
			n := r.Intn(50)
			for j := 0; j < n; j++ {
				id := int64(ids[j]) - 30
				lists[i].RowIDs = append(lists[i].RowIDs, id)
				sum[id] += lists[i].Weight / (offset + float64(j+1))
			}
		}
		expected := []sim.FusedHit{}
		for id, score := range sum {
			expected = append(expected, sim.FusedHit{RowID: id, Score: score})
		}
		sort.Slice(expected, func(i, j int) bool {
			a, b := expected[i], expected[j]
			return a.Score > b.Score || a.Score == b.Score && a.RowID < b.RowID
		})
		k := 1 + r.Intn(30)
		if len(expected) > k {
			expected = expected[:k]
		}
		got, err := e.Fuse(ctx, lists, offset, k)
		if err != nil || len(got) != len(expected) {
			t.Fatal(round, got, err, expected)
		}
		for i := range got {
			if got[i].RowID != expected[i].RowID || got[i].Score != expected[i].Score {
				t.Fatal(round, got, expected)
			}
			total := 0.0
			for _, c := range got[i].Contributions {
				total += c.Value
			}
			if total != got[i].Score {
				t.Fatal("details", got[i])
			}
		}
	}
}

func normalizeReference(t testing.TB, c grams13.Config, s string) string {
	t.Helper()
	tokenizer, err := grams13.NewTokenizer(c)
	if err != nil {
		t.Fatal(err)
	}
	out, err := tokenizer.Normalize(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestSetComparisonBudgetIsAggregate(t *testing.T) {
	c := sim.DefaultConfig()
	c.MinGram, c.MaxGram = 1, 1
	c.Limits.MaxSetComparisons = 1
	e := engine(t, c)
	ctx := context.Background()
	got, err := e.Compare(ctx, "ab", "ab", sim.Dice)
	if !errors.Is(err, sim.ErrLimit) || got != (sim.Score{}) {
		t.Fatal("single comparison", got, err)
	}
	result, err := e.Scan(ctx, "a", source([]sim.Document{{RowID: 1, Text: "a"}, {RowID: 2, Text: "a"}}), sim.SearchOptions{Metric: sim.Dice, Limit: 1})
	if !errors.Is(err, sim.ErrLimit) || !reflect.DeepEqual(result, sim.ScanResult{}) {
		t.Fatal("aggregate comparisons must not reset per row", result, err)
	}
	result, err = e.Scan(ctx, "a", source([]sim.Document{{RowID: 1, Text: "a"}}), sim.SearchOptions{Metric: sim.Dice, Limit: 1})
	if err != nil || len(result.Hits) != 1 {
		t.Fatal("exact budget", result, err)
	}
	c.Limits.MaxSetComparisons = 0
	if _, err := sim.New(c); !errors.Is(err, sim.ErrConfig) {
		t.Fatal(err)
	}
	c.Limits.MaxSetComparisons = 1<<30 + 1
	if _, err := sim.New(c); !errors.Is(err, sim.ErrConfig) {
		t.Fatal(err)
	}
}
