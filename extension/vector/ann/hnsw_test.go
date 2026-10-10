package ann

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"math"
	"math/rand/v2"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

type memoryStore struct {
	nodes                    map[int64]Node
	reads, writes, failWrite int
	onRead                   func()
}

func newMemoryStore() *memoryStore { return &memoryStore{nodes: make(map[int64]Node)} }
func (s *memoryStore) GetNode(ctx context.Context, id int64) (Node, error) {
	s.reads++
	if s.onRead != nil {
		s.onRead()
	}
	if err := ctx.Err(); err != nil {
		return Node{}, err
	}
	n, ok := s.nodes[id]
	if !ok {
		return Node{}, ErrCorrupt
	}
	return n, nil
}
func (s *memoryStore) PutNode(ctx context.Context, n Node) error {
	s.writes++
	if s.failWrite > 0 && s.writes == s.failWrite {
		return errors.New("test store write refused")
	}
	s.nodes[n.ID] = cloneNode(n)
	return ctx.Err()
}
func config(t *testing.T, dims int, metric vector.MetricKind) Config {
	t.Helper()
	c, err := DefaultConfig(HNSW, dims, metric)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func vec(t *testing.T, v ...float32) vector.Vector {
	t.Helper()
	x, err := vector.New(v)
	if err != nil {
		t.Fatal(err)
	}
	return x
}
func build(t *testing.T, c Config, values []vector.Vector) (*memoryStore, State) {
	t.Helper()
	s := newMemoryStore()
	state := NewState()
	for i, v := range values {
		if err := Insert(context.Background(), s, &state, int64(i)-int64(len(values)/2), v, c); err != nil {
			t.Fatalf("insert %d: %v", i, err)
		}
	}
	return s, state
}
func checkGraph(t *testing.T, s *memoryStore, state State, c Config) {
	t.Helper()
	if err := state.Validate(c); err != nil {
		t.Fatal(err)
	}
	if int64(len(s.nodes)) != state.NextID-1 {
		t.Fatal("node count")
	}
	var live, dead int64
	maxLevel := -1
	for _, n := range s.nodes {
		if err := ValidateNode(n, c); err != nil {
			t.Fatalf("node %d: %v", n.ID, err)
		}
		if n.Live {
			live++
		} else {
			dead++
		}
		if n.Level > maxLevel {
			maxLevel = n.Level
		}
		for layer, ids := range n.Neighbors {
			for _, id := range ids {
				neighbor, ok := s.nodes[id]
				if !ok || neighbor.Level < layer {
					t.Fatalf("invalid edge %d -> %d layer %d", n.ID, id, layer)
				}
			}
		}
	}
	if live != state.Live || dead != state.Tombstones || maxLevel != state.MaxLevel || (state.Entry != 0 && s.nodes[state.Entry].Level != state.MaxLevel) {
		t.Fatal("state mismatch")
	}
}

func TestConfigCanonicalAndClosedIdentity(t *testing.T) {
	for _, metric := range []vector.MetricKind{vector.L2, vector.L1, vector.Cosine, vector.NegativeIP} {
		c := config(t, 2000, metric)
		b, err := c.Canonical()
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := ParseConfig(b)
		if err != nil || parsed != c {
			t.Fatalf("roundtrip %v %v", parsed, err)
		}
		for _, bad := range [][]byte{append(append([]byte(nil), b...), ' '), bytes.Replace(b, []byte(`"max_level":32`), []byte(`"max_level":33`), 1), bytes.Replace(b, []byte(`"m":16`), []byte(`"m":16,"m":16`), 1), bytes.Replace(b, []byte(`"version":1`), []byte(`"version":2`), 1), bytes.Replace(b, []byte(`"method":"hnsw"`), []byte(`"method":"hnsw","unknown":0`), 1), bytes.Replace(b, []byte(`"zero_policy":"reject-cosine-zero"`), []byte(`"zero_policy":"omit"`), 1)} {
			if _, err := ParseConfig(bad); !errors.Is(err, ErrConfig) {
				t.Fatalf("accepted noncanonical config: %s", bad)
			}
		}
	}
	c := config(t, 3, vector.L2)
	mutations := []func(*Config){func(c *Config) { c.Method = "ivfflat" }, func(c *Config) { c.Version = 2 }, func(c *Config) { c.Opclass = "custom_l2" }, func(c *Config) { c.Metric = "L2" }, func(c *Config) { c.Dimensions = 2001 }, func(c *Config) { c.M = 1 }, func(c *Config) { c.M = 101 }, func(c *Config) { c.EFConstruction = 3 }, func(c *Config) { c.EFConstruction = 1001 }, func(c *Config) { c.M = 33; c.EFConstruction = 64 }, func(c *Config) { c.MaxTombstones = c.MaxNodes + 1 }, func(c *Config) { c.MaxVisited = 0 }, func(c *Config) { c.MaxDistanceEvaluations = 0 }, func(c *Config) { c.MaxMemoryBytes = 0 }}
	for i, mutate := range mutations {
		bad := c
		mutate(&bad)
		if !errors.Is(bad.Validate(), ErrConfig) {
			t.Fatalf("accepted config mutation %d", i)
		}
	}
	if _, err := DefaultConfig(HNSW, 16000, vector.L2); !errors.Is(err, ErrConfig) {
		t.Fatal(err)
	}
	normal, err := (Config{Dimensions: 3, Metric: vector.L2}).Normalize()
	if err != nil || normal != c {
		t.Fatalf("normalize %v %v", normal, err)
	}
	zero := vec(t, 0, 0, 0)
	if err := ValidateVector(zero, c); err != nil {
		t.Fatal(err)
	}
	c.Metric = vector.Cosine
	c.Opclass, _ = Opclass(c.Metric)
	if !errors.Is(ValidateVector(zero, c), vector.ErrZeroNorm) {
		t.Fatal("cosine zero accepted")
	}
}

func TestAdjacencyValidationAndRoundtrip(t *testing.T) {
	c := config(t, 2, vector.L2)
	n := Node{ID: 1, Live: true, Level: 2, Vector: vec(t, 1, 2), Neighbors: [][]int64{{2, 3}, {2}, {}}}
	b, err := EncodeAdjacency(n, c)
	if err != nil {
		t.Fatal(err)
	}
	ids, err := DecodeAdjacency(b, n.ID, n.Level, c)
	if err != nil || !reflect.DeepEqual(ids, n.Neighbors) {
		t.Fatalf("roundtrip %v %v", ids, err)
	}
	for length := 0; length < len(b); length++ {
		if _, err := DecodeAdjacency(b[:length], 1, 2, c); !errors.Is(err, ErrCorrupt) {
			t.Fatalf("truncation %d: %v", length, err)
		}
	}
	mutations := []func([]byte){func(b []byte) { b[4] = 2 }, func(b []byte) { b[7] = 1 }, func(b []byte) { b[11] = 4 }, func(b []byte) { b[12] = 255 }, func(b []byte) { b[15] = 1 }, func(b []byte) { binary.BigEndian.PutUint64(b[16:24], 1) }, func(b []byte) { binary.BigEndian.PutUint64(b[24:32], 2) }, func(b []byte) { binary.BigEndian.PutUint64(b[16:24], math.MaxUint64) }}
	for i, mutate := range mutations {
		bad := append([]byte(nil), b...)
		mutate(bad)
		if _, err := DecodeAdjacency(bad, 1, 2, c); !errors.Is(err, ErrCorrupt) {
			t.Fatalf("mutation %d: %v", i, err)
		}
	}
	if _, err := DecodeAdjacency(append(b, 0), 1, 2, c); !errors.Is(err, ErrCorrupt) {
		t.Fatal(err)
	}
}

func TestDeterministicMultilayerAndSignedTies(t *testing.T) {
	c := config(t, 2, vector.L2)
	c.M = 4
	c.EFConstruction = 32
	values := make([]vector.Vector, 180)
	for i := range values {
		values[i] = vec(t, float32(i%23), float32(i/23))
	}
	s, state := build(t, c, values)
	again, state2 := build(t, c, values)
	checkGraph(t, s, state, c)
	if state.MaxLevel < 2 {
		t.Fatalf("not genuinely multilayer: %d", state.MaxLevel)
	}
	if state != state2 || !reflect.DeepEqual(s.nodes, again.nodes) {
		t.Fatal("nondeterministic build")
	}
	alternate := c
	alternate.Seed++
	other, _ := build(t, alternate, values)
	if reflect.DeepEqual(s.nodes, other.nodes) {
		t.Fatal("seed did not affect topology")
	}
	query := vec(t, 4.4, 3.2)
	hits, stats, err := Search(context.Background(), s, state, query, c, 40, Limits{})
	if err != nil || len(hits) != 40 || stats.NodeReads == 0 || stats.EdgeReads == 0 || stats.DistanceEvaluations == 0 {
		t.Fatalf("search %d %+v %v", len(hits), stats, err)
	}
	for i := 1; i < len(hits); i++ {
		if candidateLess(hits[i], hits[i-1]) {
			t.Fatal("unsorted candidates")
		}
	}
	ties := newMemoryStore()
	st := NewState()
	for _, id := range []int64{math.MaxInt64, 0, -1, math.MinInt64, 7} {
		if err := Insert(context.Background(), ties, &st, id, vec(t, 1, 0), c); err != nil {
			t.Fatal(err)
		}
	}
	hits, _, err = Search(context.Background(), ties, st, vec(t, 0, 0), c, 20, Limits{})
	if err != nil || len(hits) != 5 {
		t.Fatalf("ties %v %v", hits, err)
	}
	for i := 1; i < len(hits); i++ {
		if hits[i].RowID <= hits[i-1].RowID {
			t.Fatalf("not signed-rowid order: %v", hits)
		}
	}
}

func TestTombstonesTraverseAndRowidReuse(t *testing.T) {
	c := config(t, 1, vector.L2)
	s := newMemoryStore()
	s.nodes[1] = Node{ID: 1, RowID: 10, Live: true, Vector: vec(t, 10), Neighbors: [][]int64{{2}}}
	s.nodes[2] = Node{ID: 2, RowID: 20, Live: false, Vector: vec(t, 5), Neighbors: [][]int64{{1, 3}}}
	s.nodes[3] = Node{ID: 3, RowID: 30, Live: true, Vector: vec(t, 0), Neighbors: [][]int64{{2}}}
	state := State{Entry: 1, NextID: 4, Live: 2, Tombstones: 1, MaxLevel: 0}
	hits, stats, err := Search(context.Background(), s, state, vec(t, 0), c, 2, Limits{})
	if err != nil || len(hits) != 2 || hits[0].NodeID != 3 || stats.NodeReads != 3 {
		t.Fatalf("did not traverse dead bridge: %v %+v %v", hits, stats, err)
	}
	before := cloneNode(s.nodes[1])
	if err := Tombstone(context.Background(), s, &state, 1, c); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.nodes[1].Neighbors, before.Neighbors) || s.nodes[1].Vector.String() != before.Vector.String() {
		t.Fatal("destroyed tombstone contents")
	}
	if err := Insert(context.Background(), s, &state, 10, vec(t, -1), c); err != nil {
		t.Fatal(err)
	}
	if s.nodes[1].Live || s.nodes[4].RowID != 10 || !s.nodes[4].Live {
		t.Fatal("reused a dead node identity")
	}
	checkGraph(t, s, state, c)
	hits, _, err = Search(context.Background(), s, state, vec(t, -1), c, 10, Limits{})
	if err != nil || len(hits) != 2 || hits[0].NodeID != 4 {
		t.Fatalf("updated %v %v", hits, err)
	}
	if err := Tombstone(context.Background(), s, &state, 1, c); err != nil {
		t.Fatal("idempotent", err)
	}
	if err := Tombstone(context.Background(), s, &state, 3, c); err != nil {
		t.Fatal(err)
	}
	if err := Tombstone(context.Background(), s, &state, 4, c); err != nil {
		t.Fatal(err)
	}
	hits, stats, err = Search(context.Background(), s, state, vec(t, 0), c, 10, Limits{})
	if err != nil || len(hits) != 0 || stats.NodeReads != 0 || stats.StopReason != "empty" {
		t.Fatalf("all dead %v %+v %v", hits, stats, err)
	}
	if err := Insert(context.Background(), s, &state, 10, vec(t, 8), c); err != nil {
		t.Fatal(err)
	}
	hits, _, err = Search(context.Background(), s, state, vec(t, 8), c, 10, Limits{})
	if err != nil || len(hits) != 1 || hits[0].NodeID != 5 {
		t.Fatalf("insert after all dead %v %v", hits, err)
	}
}

func TestAllMetricsAndCosineZeroPolicy(t *testing.T) {
	for _, metric := range []vector.MetricKind{vector.L2, vector.L1, vector.Cosine, vector.NegativeIP} {
		t.Run(string(metric), func(t *testing.T) {
			c := config(t, 2, metric)
			values := []vector.Vector{vec(t, 1, 0), vec(t, 0, 1), vec(t, -1, 0), vec(t, 2, 1), vec(t, 3, 2)}
			s, state := build(t, c, values)
			query := vec(t, 1, .3)
			hits, _, err := Search(context.Background(), s, state, query, c, 20, Limits{})
			if err != nil || len(hits) != len(values) {
				t.Fatalf("%v %v", hits, err)
			}
			for _, hit := range hits {
				want, _ := metric.Distance(query, s.nodes[hit.NodeID].Vector)
				if hit.Distance != want {
					t.Fatalf("distance %g want %g", hit.Distance, want)
				}
			}
			zero := vec(t, 0, 0)
			err = Insert(context.Background(), s, &state, 99, zero, c)
			if metric == vector.Cosine {
				if !errors.Is(err, vector.ErrZeroNorm) {
					t.Fatal(err)
				}
				if _, _, err := Search(context.Background(), s, state, zero, c, 40, Limits{}); !errors.Is(err, vector.ErrZeroNorm) {
					t.Fatal(err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestLimitsCancellationAndFailedWriteState(t *testing.T) {
	c := config(t, 2, vector.L2)
	values := make([]vector.Vector, 50)
	for i := range values {
		values[i] = vec(t, float32(i), 1)
	}
	s, state := build(t, c, values)
	query := vec(t, 23, 1)
	for _, limit := range []Limits{{MaxVisited: 1}, {MaxDistanceEvaluations: 1}, {MaxBytes: 1}} {
		hits, stats, err := Search(context.Background(), s, state, query, c, 40, limit)
		if !errors.Is(err, ErrLimit) || len(hits) != 0 || stats.StopReason != "resource_limit" {
			t.Fatalf("limits %+v: %v %+v %v", limit, hits, stats, err)
		}
	}
	if _, _, err := Search(context.Background(), s, state, query, c, 40, Limits{MaxVisited: c.MaxVisited + 1}); !errors.Is(err, ErrConfig) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	reads := s.reads
	if _, _, err := Search(ctx, s, state, query, c, 40, Limits{}); !errors.Is(err, context.Canceled) || s.reads != reads {
		t.Fatal("canceled read", err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	s.onRead = cancel
	if hits, _, err := Search(ctx, s, state, query, c, 40, Limits{}); !errors.Is(err, context.Canceled) || hits != nil {
		t.Fatal("canceled callback", err)
	}
	s.onRead = nil
	beforeState := state
	beforeNodes := make(map[int64]Node)
	for id, n := range s.nodes {
		beforeNodes[id] = cloneNode(n)
	}
	s.failWrite = s.writes + 1
	if err := Insert(context.Background(), s, &state, 999, vec(t, 24.5, 1), c); err == nil || !strings.Contains(err.Error(), "write refused") {
		t.Fatal(err)
	}
	if state != beforeState || !reflect.DeepEqual(beforeNodes, s.nodes) {
		t.Fatal("published state or mutated read alias before failing first write")
	}
	s.failWrite = 0
	small := c
	small.MaxNodes = state.NextID - 1
	small.MaxTombstones = 0
	if err := Insert(context.Background(), s, &state, 999, query, small); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	if err := Tombstone(context.Background(), s, &state, 1, small); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	if _, _, err := Search(nil, s, state, query, c, 40, Limits{}); !errors.Is(err, vector.ErrContext) {
		t.Fatal(err)
	}
	var nilStore *memoryStore
	if _, _, err := Search(context.Background(), nilStore, state, query, c, 40, Limits{}); !errors.Is(err, ErrConfig) {
		t.Fatal(err)
	}
}

func TestMalformedNodesFailClosed(t *testing.T) {
	c := config(t, 1, vector.L2)
	mutations := []func(*memoryStore, *State){func(s *memoryStore, st *State) { n := s.nodes[1]; n.ID = 2; s.nodes[1] = n }, func(s *memoryStore, st *State) { n := s.nodes[1]; n.Neighbors[0] = []int64{1}; s.nodes[1] = n }, func(s *memoryStore, st *State) { n := s.nodes[1]; n.Neighbors[0] = []int64{2, 2}; s.nodes[1] = n }, func(s *memoryStore, st *State) { n := s.nodes[1]; n.Neighbors[0] = []int64{3}; s.nodes[1] = n }, func(s *memoryStore, st *State) { delete(s.nodes, 2) }, func(s *memoryStore, st *State) { st.MaxLevel = 1 }}
	for i, mutate := range mutations {
		s := newMemoryStore()
		s.nodes[1] = Node{ID: 1, Live: true, Vector: vec(t, 0), Neighbors: [][]int64{{2}}}
		s.nodes[2] = Node{ID: 2, Live: true, Vector: vec(t, 1), Neighbors: [][]int64{{1}}}
		state := State{Entry: 1, NextID: 3, Live: 2, MaxLevel: 0}
		mutate(s, &state)
		hits, _, err := Search(context.Background(), s, state, vec(t, 0), c, 40, Limits{})
		if !errors.Is(err, ErrCorrupt) || hits != nil {
			t.Fatalf("mutation %d: %v %v", i, hits, err)
		}
	}
}

func TestDefaultRecallAndReducedDistancePath(t *testing.T) {
	const count, dims, queries = 2048, 12, 48
	rng := rand.New(rand.NewPCG(0x60e6317, 0x537cc291))
	values := make([]vector.Vector, count)
	for i := range values {
		v := make([]float32, dims)
		for j := range v {
			v[j] = float32(rng.NormFloat64())
		}
		values[i] = vec(t, v...)
	}
	c := config(t, dims, vector.L2)
	buildStart := time.Now()
	s, state := build(t, c, values)
	buildTime := time.Since(buildStart)
	checkGraph(t, s, state, c)
	var recalled, total, distanceWork, reads, recallOne, recallHundred, distanceHundred int
	var logicalBytes int
	for _, node := range s.nodes {
		adjacency, err := EncodeAdjacency(node, c)
		if err != nil {
			t.Fatal(err)
		}
		encoded, _ := node.Vector.MarshalBinary()
		logicalBytes += len(adjacency) + len(encoded)
	}
	rows := make([]vector.Row, len(values))
	for i, value := range values {
		rows[i] = vector.Row{RowID: int64(i) - count/2, Vector: value}
	}
	var annTimes, exactTimes []time.Duration
	for range queries {
		q := make([]float32, dims)
		for j := range q {
			q[j] = float32(rng.NormFloat64())
		}
		query := vec(t, q...)
		start := time.Now()
		hits, stats, err := Search(context.Background(), s, state, query, c, DefaultEFSearch, Limits{})
		annTimes = append(annTimes, time.Since(start))
		if err != nil || len(hits) < 10 {
			t.Fatalf("search %v", err)
		}
		exact := make([]Candidate, len(values))
		for i, v := range values {
			distance, _ := vector.L2.Distance(query, v)
			exact[i] = Candidate{NodeID: int64(i + 1), RowID: int64(i) - count/2, Distance: distance}
		}
		sort.Slice(exact, func(i, j int) bool { return candidateLess(exact[i], exact[j]) })
		if hits[0].NodeID == exact[0].NodeID {
			recallOne++
		}
		start = time.Now()
		exactTop, err := vector.TopK(context.Background(), query, rows, vector.SearchOptions{Metric: vector.L2, Limit: 10})
		exactTimes = append(exactTimes, time.Since(start))
		if err != nil || len(exactTop.Hits) != 10 {
			t.Fatalf("exact baseline: %v", err)
		}
		for i, hit := range exactTop.Hits {
			if hit.RowID != exact[i].RowID || hit.Distance != exact[i].Distance {
				t.Fatal("independent full-sort / M6 exact disagreement")
			}
		}
		large, largeStats, err := Search(context.Background(), s, state, query, c, 100, Limits{})
		if err != nil || len(large) != 100 {
			t.Fatalf("ef100: %v", err)
		}
		wantHundred := make(map[int64]bool)
		for _, candidate := range exact[:100] {
			wantHundred[candidate.NodeID] = true
		}
		for _, hit := range large {
			if wantHundred[hit.NodeID] {
				recallHundred++
			}
		}
		distanceHundred += largeStats.DistanceEvaluations
		want := make(map[int64]bool)
		for _, candidate := range exact[:10] {
			want[candidate.NodeID] = true
		}
		for _, hit := range hits[:10] {
			if want[hit.NodeID] {
				recalled++
			}
		}
		total += 10
		distanceWork += stats.DistanceEvaluations
		reads += stats.NodeReads
	}
	recall := float64(recalled) / float64(total)
	meanDistances := float64(distanceWork) / queries
	meanReads := float64(reads) / queries
	t.Logf("seeded independent Gaussian dataset: rows=%d dims=%d held_out=%d recall@10=%.6f mean graph distances=%.2f mean node reads=%.2f exact candidate distances=%d max_level=%d", count, dims, queries, recall, meanDistances, meanReads, count, state.MaxLevel)
	sort.Slice(annTimes, func(i, j int) bool { return annTimes[i] < annTimes[j] })
	sort.Slice(exactTimes, func(i, j int) bool { return exactTimes[i] < exactTimes[j] })
	t.Logf("memory-store algorithm only: recall@1(ef40)=%.6f recall@100(ef100)=%.6f mean ef100 distances=%.2f build=%s logical vector+adjacency bytes=%d; ANN p50=%s p95=%s; M6 exact k10 p50=%s p95=%s (wall times are observations, not asserted speedup)", float64(recallOne)/queries, float64(recallHundred)/(100*queries), float64(distanceHundred)/queries, buildTime, logicalBytes, annTimes[queries/2], annTimes[(queries*95-1)/100], exactTimes[queries/2], exactTimes[(queries*95-1)/100])
	if recall < .95 {
		t.Fatalf("default recall@10 %.6f below dataset target .95", recall)
	}
	if meanDistances >= count/2 {
		t.Fatalf("graph distance path not materially reduced: %.2f/%d", meanDistances, count)
	}
}

func FuzzAdjacencyDecode(f *testing.F) {
	c, _ := DefaultConfig(HNSW, 1, vector.L2)
	v, _ := vector.New([]float32{1})
	b, _ := EncodeAdjacency(Node{ID: 1, Live: true, Vector: v, Neighbors: [][]int64{{2, 3}}}, c)
	f.Add(b)
	f.Fuzz(func(t *testing.T, data []byte) {
		ids, err := DecodeAdjacency(data, 1, 0, c)
		if err != nil {
			return
		}
		encoded, err := EncodeAdjacency(Node{ID: 1, Live: true, Vector: v, Neighbors: ids}, c)
		if err != nil || !bytes.Equal(data, encoded) {
			t.Fatalf("decode not canonical %v", err)
		}
	})
}
