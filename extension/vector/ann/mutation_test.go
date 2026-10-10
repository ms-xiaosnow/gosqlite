package ann

import (
	"context"
	"errors"
	"math/rand/v2"
	"reflect"
	"sort"
	"sync"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

// immutableStore deliberately has no shared counters or mutable cache. It
// models the Reader snapshot contract for concurrent independent searches.
type immutableStore map[int64]Node

func (s immutableStore) GetNode(ctx context.Context, id int64) (Node, error) {
	if err := ctx.Err(); err != nil {
		return Node{}, err
	}
	n, ok := s[id]
	if !ok {
		return Node{}, ErrCorrupt
	}
	return n, nil
}

func TestConcurrentSnapshotSearch(t *testing.T) {
	c := config(t, 2, vector.L2)
	values := make([]vector.Vector, 200)
	for i := range values {
		values[i] = vec(t, float32(i%31), float32(i/31))
	}
	s, state := build(t, c, values)
	snapshot := immutableStore(s.nodes)
	query := vec(t, 4.3, 2.1)
	want, wantStats, err := Search(context.Background(), snapshot, state, query, c, 40, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 8 {
				got, stats, err := Search(context.Background(), snapshot, state, query, c, 40, Limits{})
				if err != nil || !reflect.DeepEqual(got, want) || stats != wantStats {
					t.Errorf("snapshot search differs: %v", err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestRejectedConstructionPreservesStateAndStore(t *testing.T) {
	c := config(t, 1, vector.L2)
	values := []vector.Vector{vec(t, 0), vec(t, 1), vec(t, 2), vec(t, 3)}
	s, state := build(t, c, values)
	for _, modify := range []func(*Config){func(c *Config) { c.MaxVisited = 1 }, func(c *Config) { c.MaxDistanceEvaluations = 1 }, func(c *Config) { c.MaxMemoryBytes = 1024 }} {
		limited := c
		modify(&limited)
		before := state
		nodes := make(map[int64]Node)
		for id, n := range s.nodes {
			nodes[id] = cloneNode(n)
		}
		writes := s.writes
		if err := Insert(context.Background(), s, &state, 30, vec(t, 1.5), limited); !errors.Is(err, ErrLimit) {
			t.Fatalf("limit %+v: %v", limited, err)
		}
		if state != before || s.writes != writes || !reflect.DeepEqual(nodes, s.nodes) {
			t.Fatal("construction wrote before completing bounded staging")
		}
	}
	state = State{}
	empty := newMemoryStore()
	if err := Insert(context.Background(), empty, &state, -5, vec(t, 1), c); err != nil || state.NextID != 2 {
		t.Fatal("zero initial state", err)
	}
}

func TestUpdateDeleteAndRebuildRecall(t *testing.T) {
	const count, dims = 768, 8
	rng := rand.New(rand.NewPCG(88, 93))
	values := make([]vector.Vector, count)
	for i := range values {
		x := make([]float32, dims)
		for j := range x {
			x[j] = float32(rng.NormFloat64())
		}
		values[i] = vec(t, x...)
	}
	c := config(t, dims, vector.L2)
	s, state := build(t, c, values)
	live := make(map[int64]vector.Vector)
	for _, n := range s.nodes {
		live[n.RowID] = n.Vector
	}
	// Model one transaction per delete or update; row IDs are deliberately signed.
	for id := int64(1); id <= count; id++ {
		n := s.nodes[id]
		if id%7 == 0 {
			if err := Tombstone(context.Background(), s, &state, id, c); err != nil {
				t.Fatal(err)
			}
			delete(live, n.RowID)
		} else if id%11 == 0 {
			if err := Tombstone(context.Background(), s, &state, id, c); err != nil {
				t.Fatal(err)
			}
			v := n.Vector.Values()
			v[0] += .5
			updated := vec(t, v...)
			if err := Insert(context.Background(), s, &state, n.RowID, updated, c); err != nil {
				t.Fatal(err)
			}
			live[n.RowID] = updated
		}
	}
	checkGraph(t, s, state, c)
	if state.Tombstones == 0 {
		t.Fatal("mutation test lacks tombstones")
	}
	// Rebuild streams current live rows in signed-rowid order with fresh node IDs.
	rebuilt := newMemoryStore()
	rebuiltState := NewState()
	ids := make([]int64, 0, len(live))
	for id := range live {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		if err := Insert(context.Background(), rebuilt, &rebuiltState, id, live[id], c); err != nil {
			t.Fatal(err)
		}
	}
	checkGraph(t, rebuilt, rebuiltState, c)
	if rebuiltState.Tombstones != 0 || rebuiltState.Live != state.Live {
		t.Fatal("rebuild counts")
	}
	queries := make([]vector.Vector, 32)
	for i := range queries {
		x := make([]float32, dims)
		for j := range x {
			x[j] = float32(rng.NormFloat64())
		}
		queries[i] = vec(t, x...)
	}
	for _, graph := range []struct {
		name  string
		store *memoryStore
		state State
	}{{"after_mutations", s, state}, {"after_rebuild", rebuilt, rebuiltState}} {
		var matched, total, distanceWork int
		for _, q := range queries {
			hits, stats, err := Search(context.Background(), graph.store, graph.state, q, c, 40, Limits{})
			if err != nil || len(hits) < 10 {
				t.Fatal(err)
			}
			distanceWork += stats.DistanceEvaluations
			exact := make([]Candidate, 0, len(live))
			for rowid, v := range live {
				distance, _ := c.Metric.Distance(q, v)
				exact = append(exact, Candidate{RowID: rowid, Distance: distance})
			}
			sort.Slice(exact, func(i, j int) bool { return candidateLess(exact[i], exact[j]) })
			want := make(map[int64]bool)
			for _, hit := range exact[:10] {
				want[hit.RowID] = true
			}
			seen := make(map[int64]bool)
			for _, hit := range hits {
				if _, ok := live[hit.RowID]; !ok || seen[hit.RowID] {
					t.Fatal("dead or duplicate result")
				}
				seen[hit.RowID] = true
			}
			for _, hit := range hits[:10] {
				if want[hit.RowID] {
					matched++
				}
			}
			total += 10
		}
		recall := float64(matched) / float64(total)
		t.Logf("%s: live=%d tombstones=%d recall@10=%.6f mean graph distances=%.2f", graph.name, graph.state.Live, graph.state.Tombstones, recall, float64(distanceWork)/float64(len(queries)))
		if recall < .95 {
			t.Fatalf("%s recall %.6f", graph.name, recall)
		}
	}
}

func FuzzConfigCanonical(f *testing.F) {
	c, _ := DefaultConfig(HNSW, 2, vector.L2)
	b, _ := c.Canonical()
	f.Add(b)
	f.Fuzz(func(t *testing.T, data []byte) {
		cfg, err := ParseConfig(data)
		if err != nil {
			return
		}
		encoded, err := cfg.Canonical()
		if err != nil || string(data) != string(encoded) {
			t.Fatalf("noncanonical config accepted: %v", err)
		}
	})
}

func FuzzBoundedGraphOperations(f *testing.F) {
	f.Add([]byte{3, 4, 12, 2, 8, 99, 13, 5, 74, 10, 15, 56})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 96 {
			return
		}
		c := config(t, 2, vector.L2)
		c.M = 4
		c.EFConstruction = 16
		s := newMemoryStore()
		state := NewState()
		for i, b := range data {
			v := vec(t, float32(int8(b)), float32(i%7))
			if err := Insert(context.Background(), s, &state, int64(i)-48, v, c); err != nil {
				t.Fatal(err)
			}
			if b%5 == 0 && state.Live > 1 {
				id := int64(1) + int64(b)%int64(i+1)
				if err := Tombstone(context.Background(), s, &state, id, c); err != nil {
					t.Fatal(err)
				}
			}
		}
		checkGraph(t, s, state, c)
		hits, _, err := Search(context.Background(), s, state, vec(t, 3, 2), c, 40, Limits{})
		if err != nil {
			t.Fatal(err)
		}
		seen := make(map[int64]bool)
		for i, hit := range hits {
			if seen[hit.NodeID] || !s.nodes[hit.NodeID].Live {
				t.Fatal("dead or duplicate")
			}
			seen[hit.NodeID] = true
			if i > 0 && candidateLess(hit, hits[i-1]) {
				t.Fatal("out of order")
			}
		}
	})
}
