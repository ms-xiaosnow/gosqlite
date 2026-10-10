package ann

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/rand/v2"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

func ivfConfig(t *testing.T, dims, lists int, metric vector.MetricKind) Config {
	t.Helper()
	c, err := DefaultConfig(IVFFlat, dims, metric)
	if err != nil {
		t.Fatal(err)
	}
	c.Lists = lists
	return c
}
func trainIVF(t *testing.T, c Config, s []Sample) []Centroid {
	t.Helper()
	centers, stats, ready, err := Train(context.Background(), s, c)
	if err != nil || !ready || len(centers) != c.Lists {
		t.Fatalf("train ready=%v stats=%+v err=%v", ready, stats, err)
	}
	return centers
}

func TestIVFConfigAndLegacyHNSWCanonical(t *testing.T) {
	c := config(t, 3, vector.L2)
	const old = `{"method":"hnsw","version":1,"dimensions":3,"metric":"l2","opclass":"vector_l2_ops","seed":2611923443488327891,"m":16,"ef_construction":64,"max_nodes":100000,"max_tombstones":25000,"max_visited":4096,"max_distance_evaluations":1000000,"max_memory_bytes":33554432,"build_policy":"splitmix64-geometric-v1","zero_policy":"reject-cosine-zero","max_level":32}`
	got, err := c.Canonical()
	if err != nil || string(got) != old {
		t.Fatalf("legacy canonical changed: %s %v", got, err)
	}
	parsed, err := ParseConfig([]byte(old))
	if err != nil || parsed != c {
		t.Fatalf("old HNSW unreadable %v", err)
	}
	for _, metric := range []vector.MetricKind{vector.L2, vector.Cosine, vector.NegativeIP} {
		c := ivfConfig(t, 2000, 100, metric)
		data, err := c.Canonical()
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := ParseConfig(data)
		if err != nil || parsed != c {
			t.Fatalf("IVF roundtrip %v", err)
		}
		for _, bad := range [][]byte{append(append([]byte(nil), data...), ' '), bytes.Replace(data, []byte(`"lists":100`), []byte(`"lists":100,"lists":100`), 1), bytes.Replace(data, []byte(`"lists":100`), []byte(`"lists":100,"m":16`), 1), bytes.Replace(data, []byte(`"version":1`), []byte(`"version":2`), 1), bytes.Replace(data, []byte("retain-prior-empty-or-zero-sum"), []byte("random"), 1), bytes.Replace(data, []byte("splitmix64-bottom-hash-rowid-v1"), []byte("first-rows"), 1)} {
			if _, err := ParseConfig(bad); !errors.Is(err, ErrConfig) {
				t.Fatalf("noncanonical accepted: %s", bad)
			}
		}
	}
	if _, err := DefaultConfig(IVFFlat, 2, vector.L1); !errors.Is(err, ErrConfig) {
		t.Fatal("accepted IVF L1", err)
	}
	c = ivfConfig(t, 3, 2, vector.L2)
	mutations := []func(*Config){func(c *Config) { c.Lists = 0 }, func(c *Config) { c.Lists = MaxLists + 1 }, func(c *Config) { c.TrainingSamples = c.Lists - 1 }, func(c *Config) { c.TrainingSamples = 1<<20 + 1 }, func(c *Config) { c.TrainingIterations = 0 }, func(c *Config) { c.TrainingIterations = 1001 }, func(c *Config) { c.MaxTrainingWork = 0 }, func(c *Config) { c.MaxTrainingWork = 1<<40 + 1 }, func(c *Config) { c.M = 16 }, func(c *Config) { c.EFConstruction = 64 }, func(c *Config) { c.MaxTombstones = 1 }, func(c *Config) { c.Method = HNSW }}
	for i, f := range mutations {
		bad := c
		f(&bad)
		if !errors.Is(bad.Validate(), ErrConfig) {
			t.Fatalf("accepted bad config %d", i)
		}
	}
	normalized, err := (Config{Method: IVFFlat, Dimensions: 2000, Metric: vector.L2, Lists: MaxLists}).Normalize()
	if err != nil || normalized.TrainingSamples < MaxLists {
		t.Fatalf("maximum lists %v", err)
	}
	state := NewState()
	state.TrainingState = "untrained"
	state.Live = 7
	if err := state.Validate(c); err != nil {
		t.Fatal(err)
	}
	state.TrainingState = "ready"
	if err := state.Validate(c); err != nil {
		t.Fatal(err)
	}
	for _, f := range []func(*State){func(s *State) { s.Entry = 1 }, func(s *State) { s.MaxLevel = 0 }, func(s *State) { s.NextID = 2 }, func(s *State) { s.Tombstones = 1 }, func(s *State) { s.TrainingState = "" }} {
		bad := state
		f(&bad)
		if !errors.Is(bad.Validate(c), ErrCorrupt) {
			t.Fatal("accepted corrupt IVF state")
		}
	}
	if _, _, err := Search(context.Background(), newMemoryStore(), state, vec(t, 1, 2, 3), c, 40, Limits{}); !errors.Is(err, ErrConfig) {
		t.Fatal("IVF accepted by HNSW", err)
	}
}

func TestIVFSamplerBoundedDeterministicAndValidation(t *testing.T) {
	c := ivfConfig(t, 2, 4, vector.L2)
	c.TrainingSamples = 7
	sampler, err := NewIVFSampler(c)
	if err != nil {
		t.Fatal(err)
	}
	var all []Sample
	for id := int64(-50); id < 150; id++ {
		s := Sample{id, vec(t, float32(id), float32(id%7))}
		all = append(all, s)
		if err := sampler.Add(context.Background(), id, s.Vector); err != nil {
			t.Fatal(err)
		}
	}
	selected, err := sampler.Samples(context.Background())
	if err != nil || len(selected) != 7 {
		t.Fatal(err, len(selected))
	}
	sort.Slice(all, func(i, j int) bool {
		a, b := SamplePriority(c.Seed, all[i].RowID), SamplePriority(c.Seed, all[j].RowID)
		if a != b {
			return a < b
		}
		return all[i].RowID < all[j].RowID
	})
	want := all[:7]
	sort.Slice(want, func(i, j int) bool { return want[i].RowID < want[j].RowID })
	if !reflect.DeepEqual(selected, want) {
		t.Fatal("sample is not seeded bottom hash")
	}
	if len(sampler.entries) != 0 {
		t.Fatal("sampler retained references after drain")
	}
	if err := sampler.Add(context.Background(), 200, vec(t, 1, 2)); !errors.Is(err, ErrConfig) {
		t.Fatal("reuse", err)
	}
	if _, err := sampler.Samples(context.Background()); !errors.Is(err, ErrConfig) {
		t.Fatal("double drain", err)
	}
	sampler, _ = NewIVFSampler(c)
	if err := sampler.Add(context.Background(), math.MinInt64, vec(t, 1, 2)); err != nil {
		t.Fatal(err)
	}
	if err := sampler.Add(context.Background(), math.MinInt64, vec(t, 1, 2)); !errors.Is(err, vector.ErrDuplicateRowID) {
		t.Fatal("duplicate", err)
	}
	if err := sampler.Add(context.Background(), math.MaxInt64, vec(t, 1, 2)); err != nil {
		t.Fatal(err)
	}
	c.MaxMemoryBytes = 1024
	sampler, _ = NewIVFSampler(c)
	if err := sampler.Add(context.Background(), 1, vec(t, 1, 2)); !errors.Is(err, ErrLimit) {
		t.Fatal("sample memory", err)
	}
	empty, err := sampler.Samples(context.Background())
	if err != nil || len(empty) != 0 {
		t.Fatal("failed Add mutated sampler")
	}
	c = ivfConfig(t, 2, 1, vector.NegativeIP)
	sampler, _ = NewIVFSampler(c)
	if err := sampler.Add(context.Background(), 1, vec(t, 0, 0)); err != nil {
		t.Fatal(err)
	}
	if err := sampler.Add(context.Background(), 2, vec(t, 3, 4)); err != nil {
		t.Fatal(err)
	}
	selected, err = sampler.Samples(context.Background())
	if err != nil || len(selected) != 1 || selected[0].Vector.String() != "[3,4]" {
		t.Fatal("IP sample normalization/exclusion", selected, err)
	}
	c = ivfConfig(t, 2, 1, vector.Cosine)
	sampler, _ = NewIVFSampler(c)
	if err := sampler.Add(context.Background(), 1, vec(t, 0, 0)); !errors.Is(err, vector.ErrZeroNorm) {
		t.Fatal("cosine zero", err)
	}
}

func TestIVFLloydMeansConvergenceAndOrderInvariance(t *testing.T) {
	c := ivfConfig(t, 1, 2, vector.L2)
	samples := []Sample{{-7, vec(t, 0)}, {9, vec(t, 2)}, {4, vec(t, 10)}, {-2, vec(t, 14)}}
	original := append([]Sample(nil), samples...)
	centers, stats, ready, err := Train(context.Background(), samples, c)
	if err != nil || !ready || !stats.Converged || stats.Iterations < 2 || stats.DistanceEvaluations == 0 {
		t.Fatalf("%+v %v %v", stats, ready, err)
	}
	means := []float32{centers[0].Vector.Values()[0], centers[1].Vector.Values()[0]}
	sort.Slice(means, func(i, j int) bool { return means[i] < means[j] })
	if !reflect.DeepEqual(means, []float32{1, 12}) {
		t.Fatalf("not learned arithmetic means: %v", means)
	}
	if !reflect.DeepEqual(samples, original) {
		t.Fatal("training mutated original sample")
	}
	for i, j := 0, len(samples)-1; i < j; i, j = i+1, j-1 {
		samples[i], samples[j] = samples[j], samples[i]
	}
	again, againStats, againReady, err := Train(context.Background(), samples, c)
	if err != nil || !againReady || !reflect.DeepEqual(again, centers) || againStats != stats {
		t.Fatal("input order changes training", err)
	}
	var before, after float64
	for _, s := range samples {
		x := float64(s.Vector.Values()[0])
		a, b := x-0, x-14
		before += math.Min(a*a, b*b)
		a, b = x-1, x-12
		after += math.Min(a*a, b*b)
	}
	if after >= before {
		t.Fatal("learned means did not improve seed objective")
	}
	// A one-iteration budget is an explicit successful approximate training cap.
	c.TrainingIterations = 1
	_, shortStats, shortReady, err := Train(context.Background(), samples, c)
	if err != nil || !shortReady || shortStats.Converged || shortStats.Iterations != 1 || shortStats.StopReason != "iteration_limit" {
		t.Fatal("iteration cap", shortStats, err)
	}
}

func TestIVFSphericalTrainingOriginalIPAndCancellationPolicy(t *testing.T) {
	for _, metric := range []vector.MetricKind{vector.Cosine, vector.NegativeIP} {
		c := ivfConfig(t, 2, 1, metric)
		samples := []Sample{{1, vec(t, 100, 0)}, {2, vec(t, 0, 1)}}
		centers := trainIVF(t, c, samples)
		got := centers[0].Vector.Values()
		want := math.Sqrt(.5)
		if math.Abs(float64(got[0])-want) > 1e-6 || math.Abs(float64(got[1])-want) > 1e-6 {
			t.Fatalf("not spherical mean for %s: %v", metric, got)
		}
		if samples[0].Vector.String() != "[100,0]" {
			t.Fatal("IP magnitude destroyed")
		}
		// Antipodal unit directions cancel exactly; retain the seeded direction.
		opposing := []Sample{{-1, vec(t, -1, 0)}, {1, vec(t, 1, 0)}}
		cancelCenters, stats, ready, err := Train(context.Background(), opposing, c)
		if err != nil || !ready || stats.ZeroSumClusters != 1 {
			t.Fatal("zero-sum policy", stats, err)
		}
		seed := opposing[0]
		if SamplePriority(c.Seed, opposing[1].RowID) < SamplePriority(c.Seed, opposing[0].RowID) {
			seed = opposing[1]
		}
		if !reflect.DeepEqual(cancelCenters[0].Vector, seed.Vector) {
			t.Fatal("invented cancellation centroid")
		}
	}
	c := ivfConfig(t, 2, 2, vector.NegativeIP)
	centers := trainIVF(t, c, []Sample{{1, vec(t, 2, 0)}, {2, vec(t, 0, 8)}, {3, vec(t, 0, 0)}})
	id, stats, err := Assign(context.Background(), vec(t, 0, 0), centers, c, Limits{})
	if err != nil || id != 1 || stats.DistanceEvaluations != 2 {
		t.Fatal("IP zero assignment", id, stats, err)
	}
	ordered, _, err := Probe(context.Background(), vec(t, 3, 4), centers, c, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range ordered {
		d, _ := vector.NegativeIP.Distance(vec(t, 3, 4), centers[p.ListID-1].Vector)
		if p.Distance != d {
			t.Fatal("probe did not use original IP query")
		}
	}
	// Candidate final ranking must retain magnitude: this is the vector layer's
	// exact metric, intentionally not the spherical sample representation.
	a, _ := vector.NegativeIP.Distance(vec(t, 1, 1), vec(t, 100, 0))
	b, _ := vector.NegativeIP.Distance(vec(t, 1, 1), vec(t, 0, 1))
	if a != -100 || b != -1 {
		t.Fatal("original IP rank changed")
	}
}

func TestIVFUntrainedDuplicateEmptyAndExtremeValues(t *testing.T) {
	c := ivfConfig(t, 2000, 100, vector.L2)
	c.MaxMemoryBytes = 1024
	centers, stats, ready, err := Train(context.Background(), nil, c)
	if err != nil || ready || centers != nil || stats.StopReason != "insufficient_trainable_samples" {
		t.Fatal("empty high-dimensional index", stats, err)
	}
	c = ivfConfig(t, 2, 2, vector.NegativeIP)
	centers, stats, ready, err = Train(context.Background(), []Sample{{1, vec(t, 0, 0)}, {2, vec(t, 1, 0)}}, c)
	if err != nil || ready || centers != nil || stats.TrainableSamples != 1 || stats.SkippedZeros != 1 {
		t.Fatal("insufficient nonzero training", stats, err)
	}
	c = ivfConfig(t, 2, 3, vector.L2)
	same := []Sample{{1, vec(t, 2, 4)}, {2, vec(t, 2, 4)}, {3, vec(t, 2, 4)}}
	centers, stats, ready, err = Train(context.Background(), same, c)
	if err != nil || !ready || stats.EmptyClusters != 2 {
		t.Fatal("empty-cluster policy", stats, err)
	}
	for _, center := range centers {
		if center.Vector.String() != "[2,4]" {
			t.Fatal("random empty center")
		}
	}
	if _, _, ready, err := Train(context.Background(), []Sample{{1, vec(t, 2, 4)}, {1, vec(t, 2, 4)}}, c); !errors.Is(err, vector.ErrDuplicateRowID) || ready {
		t.Fatal("duplicate input identity", err)
	}
	for _, metric := range []vector.MetricKind{vector.L2, vector.Cosine, vector.NegativeIP} {
		c = ivfConfig(t, 2, 1, metric)
		for _, values := range [][]float32{{math.MaxFloat32, math.MaxFloat32}, {math.SmallestNonzeroFloat32, 0}} {
			centers = trainIVF(t, c, []Sample{{1, vec(t, values...)}, {2, vec(t, values...)}})
			if err := ValidateCentroid(centers[0], c); err != nil {
				t.Fatalf("extreme %s %v", metric, err)
			}
		}
	}
}

// errAfterContext deterministically exercises mid-operation cancellation.
type errAfterContext struct{ calls, after int }

func (c *errAfterContext) Deadline() (time.Time, bool) { return time.Time{}, false }
func (c *errAfterContext) Done() <-chan struct{}       { return nil }
func (c *errAfterContext) Value(any) any               { return nil }
func (c *errAfterContext) Err() error {
	c.calls++
	if c.calls >= c.after {
		return context.Canceled
	}
	return nil
}

func TestIVFResourceAndCancellationNoPartialResults(t *testing.T) {
	c := ivfConfig(t, 2, 2, vector.L2)
	s := []Sample{{1, vec(t, 1, 2)}, {2, vec(t, 3, 4)}, {3, vec(t, 5, 8)}}
	centers := trainIVF(t, c, s)
	for _, mutate := range []func(*Config){func(c *Config) { c.MaxTrainingWork = 1 }, func(c *Config) { c.MaxMemoryBytes = 1024 }, func(c *Config) { c.TrainingSamples = 2 }} {
		limited := c
		mutate(&limited)
		got, stats, ready, err := Train(context.Background(), s, limited)
		if !errors.Is(err, ErrLimit) || ready || got != nil || stats.StopReason != "resource_limit" {
			t.Fatalf("partial training on limit %+v %v", stats, err)
		}
	}
	if got, _, ready, err := Train(nil, s, c); !errors.Is(err, vector.ErrContext) || got != nil || ready {
		t.Fatal("nil context", err)
	}
	probeCtx := &errAfterContext{after: math.MaxInt}
	if _, _, ready, err := Train(probeCtx, s, c); err != nil || !ready {
		t.Fatal("cancellation baseline", err)
	}
	for _, after := range []int{1, 3, 12, probeCtx.calls - 1, probeCtx.calls} {
		got, stats, ready, err := Train(&errAfterContext{after: after}, s, c)
		if !errors.Is(err, context.Canceled) || ready || got != nil || stats.StopReason != "canceled" {
			t.Fatal("cancellation partial", after, stats, err)
		}
	}
	for _, limits := range []Limits{{MaxDistanceEvaluations: 1}, {MaxVisited: 1}, {MaxBytes: 1}} {
		got, stats, err := Probe(context.Background(), vec(t, 1, 2), centers, c, limits)
		if !errors.Is(err, ErrLimit) || got != nil || stats.StopReason != "resource_limit" {
			t.Fatal("probe limit", stats, err)
		}
		id, _, err := Assign(context.Background(), vec(t, 1, 2), centers, c, limits)
		if !errors.Is(err, ErrLimit) || id != 0 {
			t.Fatal("assignment limit", id, err)
		}
	}
	got, stats, err := Probe(&errAfterContext{after: 3}, vec(t, 1, 2), centers, c, Limits{})
	if !errors.Is(err, context.Canceled) || got != nil || stats.StopReason != "canceled" {
		t.Fatal("probe cancellation", err)
	}
}

func TestIVFProbeValidationExactMetricAndStableTies(t *testing.T) {
	for _, metric := range []vector.MetricKind{vector.L2, vector.Cosine, vector.NegativeIP} {
		c := ivfConfig(t, 2, 3, metric)
		centers := []Centroid{{1, vec(t, 1, 0)}, {2, vec(t, 1, 0)}, {3, vec(t, 0, 1)}}
		results, stats, err := Probe(context.Background(), vec(t, 2, 1), centers, c, Limits{})
		if err != nil || len(results) != 3 || stats.DistanceEvaluations != 3 || stats.NodeReads != 0 || stats.Visited != 3 {
			t.Fatal(metric, stats, err)
		}
		if results[0].ListID != 1 || results[1].ListID != 2 || results[2].ListID != 3 {
			t.Fatal("unstable equal-distance ties", results)
		}
		for _, result := range results {
			expected, _ := metric.Distance(vec(t, 2, 1), centers[result.ListID-1].Vector)
			if result.Distance != expected {
				t.Fatal("wrong exact centroid score")
			}
		}
		bad := append([]Centroid(nil), centers...)
		bad[1].ID = 1
		if result, _, err := Probe(context.Background(), vec(t, 2, 1), bad, c, Limits{}); !errors.Is(err, ErrCorrupt) || result != nil {
			t.Fatal("duplicate centroid ID", err)
		}
		if result, _, err := Probe(context.Background(), vec(t, 2, 1), centers[:2], c, Limits{}); !errors.Is(err, ErrCorrupt) || result != nil {
			t.Fatal("missing centroid", err)
		}
		if metric != vector.L2 {
			bad = append([]Centroid(nil), centers...)
			bad[0].Vector = vec(t, 2, 0)
			if err := ValidateCentroid(bad[0], c); !errors.Is(err, ErrCorrupt) {
				t.Fatal("nonunit centroid", err)
			}
		}
	}
}

func TestIVFConcurrentImmutableTrainingAndProbing(t *testing.T) {
	c := ivfConfig(t, 4, 5, vector.Cosine)
	rng := rand.New(rand.NewPCG(501, 19))
	samples := make([]Sample, 128)
	for i := range samples {
		v := make([]float32, 4)
		for j := range v {
			v[j] = float32(rng.NormFloat64())
		}
		samples[i] = Sample{int64(i) - 80, vec(t, v...)}
	}
	want, stats, _, err := Train(context.Background(), samples, c)
	if err != nil {
		t.Fatal(err)
	}
	query := vec(t, 1, 2, 3, 4)
	probes, pstats, err := Probe(context.Background(), query, want, c, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got, gs, ready, e := Train(context.Background(), samples, c)
			if e != nil || !ready || !reflect.DeepEqual(got, want) || gs != stats {
				t.Errorf("concurrent train differs %v", e)
				return
			}
			gp, gps, e := Probe(context.Background(), query, want, c, Limits{})
			if e != nil || !reflect.DeepEqual(gp, probes) || gps != pstats {
				t.Errorf("concurrent probe differs %v", e)
			}
		}()
	}
	wg.Wait()
}

func FuzzIVFTrainingAndProbe(f *testing.F) {
	f.Add([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12}, uint64(0), uint8(2), uint8(0))
	f.Add([]byte{0, 0, 0, 0, 255, 255, 255, 255}, DefaultSeed, uint8(1), uint8(2))
	f.Fuzz(func(t *testing.T, data []byte, seed uint64, listHint, metricHint uint8) {
		if len(data) > 512 {
			data = data[:512]
		}
		metrics := []vector.MetricKind{vector.L2, vector.Cosine, vector.NegativeIP}
		c := ivfConfig(t, 4, int(listHint%8)+1, metrics[metricHint%3])
		c.Seed = seed
		c.TrainingSamples = 128
		c.TrainingIterations = 8
		samples := make([]Sample, 0, len(data)/4)
		for i := 0; i+4 <= len(data); i += 4 {
			values := make([]float32, 4)
			for j := range values {
				values[j] = float32(int8(data[i+j]))
			}
			if c.Metric == vector.Cosine {
				n := float32(0)
				for _, v := range values {
					n += v * v
				}
				if n == 0 {
					values[0] = 1
				}
			}
			samples = append(samples, Sample{int64(i) - 256, vec(t, values...)})
		}
		centers, stats, ready, err := Train(context.Background(), samples, c)
		if err != nil {
			t.Fatal(err)
		}
		again, againStats, againReady, err := Train(context.Background(), samples, c)
		if err != nil || !reflect.DeepEqual(centers, again) || stats != againStats || ready != againReady {
			t.Fatal("nondeterministic train", err)
		}
		if !ready {
			if centers != nil || stats.TrainableSamples >= c.Lists {
				t.Fatal("invalid untrained result")
			}
			return
		}
		for _, s := range samples {
			id, _, err := Assign(context.Background(), s.Vector, centers, c, Limits{})
			if err != nil || id < 1 || id > c.Lists {
				t.Fatal("invalid assignment", err)
			}
			probes, _, err := Probe(context.Background(), s.Vector, centers, c, Limits{})
			if err != nil || len(probes) != c.Lists || probes[0].ListID != id {
				t.Fatal("probe/assign disagreement", err)
			}
			for i := 1; i < len(probes); i++ {
				a, b := probes[i-1], probes[i]
				if a.Distance > b.Distance || (a.Distance == b.Distance && a.ListID >= b.ListID) {
					t.Fatal("unstable probe ordering")
				}
			}
		}
	})
}
