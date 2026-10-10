package ann

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

// Version-one level policy: derive a private SplitMix64 stream from the seed
// and immutable positive node identity. Each promotion has probability 1/M
// using rejection-sampled integers, so P(level>=k)=M^-k (capped at MaxLevel).
// No floating-point logarithm or mutable PRNG state enters persisted topology.
func nodeLevel(seed uint64, id int64, m int) int {
	x := seed ^ (uint64(id) * 0xd6e8feb86659fd93)
	next := func() uint64 {
		x += 0x9e3779b97f4a7c15
		z := x
		z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
		z = (z ^ (z >> 27)) * 0x94d049bb133111eb
		return z ^ (z >> 31)
	}
	bound := uint64(m)
	threshold := -bound % bound
	for level := 0; level < MaxLevel; level++ {
		r := next()
		for r < threshold {
			r = next()
		}
		if r%bound != 0 {
			return level
		}
	}
	return MaxLevel
}

func candidateLess(a, b Candidate) bool {
	if a.Distance != b.Distance {
		return a.Distance < b.Distance
	}
	if a.RowID != b.RowID {
		return a.RowID < b.RowID
	}
	return a.NodeID < b.NodeID
}

type candidateHeap struct {
	values     []Candidate
	worstFirst bool
}

func (h candidateHeap) Len() int { return len(h.values) }
func (h candidateHeap) Less(i, j int) bool {
	if h.worstFirst {
		return candidateLess(h.values[j], h.values[i])
	}
	return candidateLess(h.values[i], h.values[j])
}
func (h candidateHeap) Swap(i, j int) { h.values[i], h.values[j] = h.values[j], h.values[i] }
func (h *candidateHeap) Push(v any)   { h.values = append(h.values, v.(Candidate)) }
func (h *candidateHeap) Pop() any {
	n := len(h.values) - 1
	v := h.values[n]
	h.values = h.values[:n]
	return v
}

type operation struct {
	ctx    context.Context
	r      Reader
	state  State
	cfg    Config
	limits Limits
	stats  Stats
	cache  map[int64]Node
	bytes  int64
}

func nilPort(v any) bool {
	if v == nil {
		return true
	}
	r := reflect.ValueOf(v)
	switch r.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map, reflect.Func, reflect.Slice:
		return r.IsNil()
	}
	return false
}

func newOperation(ctx context.Context, r Reader, state State, cfg Config, limits Limits) (*operation, error) {
	if cfg.Method != HNSW {
		return nil, ErrConfig
	}
	if ctx == nil {
		return nil, vector.ErrContext
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if nilPort(r) {
		return nil, ErrConfig
	}
	if err := state.Validate(cfg); err != nil {
		return nil, err
	}
	l, err := limits.normalized(cfg)
	if err != nil {
		return nil, err
	}
	return &operation{ctx: ctx, r: r, state: state, cfg: cfg, limits: l, cache: make(map[int64]Node)}, nil
}

func (o *operation) retain(n Node) error {
	old, exists := o.cache[n.ID]
	if !exists && o.stats.Visited >= o.limits.MaxVisited {
		return fmt.Errorf("%w: visited nodes", ErrLimit)
	}
	bytes := o.bytes + NodeMemoryBytes(n)
	if exists {
		bytes -= NodeMemoryBytes(old)
	}
	if bytes > o.limits.MaxBytes {
		return fmt.Errorf("%w: retained graph bytes", ErrLimit)
	}
	o.cache[n.ID] = n
	o.bytes = bytes
	if !exists {
		o.stats.Visited++
	}
	if bytes > o.stats.PeakBytes {
		o.stats.PeakBytes = bytes
	}
	return nil
}

func (o *operation) get(id int64) (Node, error) {
	if err := o.ctx.Err(); err != nil {
		return Node{}, err
	}
	if n, ok := o.cache[id]; ok {
		return n, nil
	}
	if id < 1 || id >= o.state.NextID {
		return Node{}, ErrCorrupt
	}
	if o.stats.Visited >= o.limits.MaxVisited {
		return Node{}, fmt.Errorf("%w: visited nodes", ErrLimit)
	}
	o.stats.NodeReads++
	n, err := o.r.GetNode(o.ctx, id)
	if err != nil {
		return Node{}, err
	}
	if err := o.ctx.Err(); err != nil {
		return Node{}, err
	}
	if n.ID != id {
		return Node{}, ErrCorrupt
	}
	if err := ValidateNode(n, o.cfg); err != nil {
		return Node{}, err
	}
	for _, ids := range n.Neighbors {
		for _, neighbor := range ids {
			if neighbor >= o.state.NextID {
				return Node{}, ErrCorrupt
			}
		}
	}
	n = cloneNode(n)
	if err := o.retain(n); err != nil {
		return Node{}, err
	}
	return n, nil
}

func (o *operation) distance(a, b vector.Vector) (float64, error) {
	if err := o.ctx.Err(); err != nil {
		return 0, err
	}
	if o.stats.DistanceEvaluations >= o.limits.MaxDistanceEvaluations {
		return 0, fmt.Errorf("%w: distance evaluations", ErrLimit)
	}
	o.stats.DistanceEvaluations++
	d, err := o.cfg.Metric.Distance(a, b)
	if err != nil {
		return 0, err
	}
	if err := o.ctx.Err(); err != nil {
		return 0, err
	}
	return d, nil
}

func (o *operation) score(query vector.Vector, id int64, layer int) (Candidate, error) {
	n, err := o.get(id)
	if err != nil {
		return Candidate{}, err
	}
	if n.Level < layer {
		return Candidate{}, ErrCorrupt
	}
	d, err := o.distance(query, n.Vector)
	if err != nil {
		return Candidate{}, err
	}
	return Candidate{NodeID: n.ID, RowID: n.RowID, Distance: d}, nil
}

func (o *operation) greedy(query vector.Vector, entry Candidate, layer int) (Candidate, error) {
	for {
		if err := o.ctx.Err(); err != nil {
			return Candidate{}, err
		}
		n, err := o.get(entry.NodeID)
		if err != nil {
			return Candidate{}, err
		}
		if n.Level < layer {
			return Candidate{}, ErrCorrupt
		}
		best := entry
		for _, id := range n.Neighbors[layer] {
			o.stats.EdgeReads++
			c, err := o.score(query, id, layer)
			if err != nil {
				return Candidate{}, err
			}
			if candidateLess(c, best) {
				best = c
			}
		}
		if best.NodeID == entry.NodeID {
			return entry, nil
		}
		entry = best
	}
}

// searchLayer is bounded best-first graph exploration. For query searches the
// retained result heap contains live nodes only, so tombstones never consume
// ef. Dead nodes can still enter and expand the frontier by the same distance
// rule. Construction includes dead nodes to retain paths through deleted data.
func (o *operation) searchLayer(query vector.Vector, entries []Candidate, layer, ef int, liveOnly bool) ([]Candidate, bool, error) {
	frontier := &candidateHeap{}
	best := &candidateHeap{worstFirst: true}
	seen := make(map[int64]bool)
	for _, c := range entries {
		if seen[c.NodeID] {
			continue
		}
		seen[c.NodeID] = true
		n, err := o.get(c.NodeID)
		if err != nil {
			return nil, false, err
		}
		if n.Level < layer {
			return nil, false, ErrCorrupt
		}
		heap.Push(frontier, c)
		if !liveOnly || n.Live {
			heap.Push(best, c)
			if best.Len() > ef {
				heap.Pop(best)
			}
		}
	}
	exhausted := true
	for frontier.Len() > 0 {
		if err := o.ctx.Err(); err != nil {
			return nil, false, err
		}
		current := heap.Pop(frontier).(Candidate)
		if best.Len() >= ef && candidateLess(best.values[0], current) {
			exhausted = false
			break
		}
		n, err := o.get(current.NodeID)
		if err != nil {
			return nil, false, err
		}
		for _, id := range n.Neighbors[layer] {
			o.stats.EdgeReads++
			if err := o.ctx.Err(); err != nil {
				return nil, false, err
			}
			if seen[id] {
				continue
			}
			seen[id] = true
			c, err := o.score(query, id, layer)
			if err != nil {
				return nil, false, err
			}
			if best.Len() < ef || candidateLess(c, best.values[0]) {
				heap.Push(frontier, c)
				target, _ := o.cache[id]
				if !liveOnly || target.Live {
					heap.Push(best, c)
					if best.Len() > ef {
						heap.Pop(best)
					}
				}
			}
		}
	}
	result := append([]Candidate(nil), best.values...)
	sort.Slice(result, func(i, j int) bool { return candidateLess(result[i], result[j]) })
	return result, exhausted, nil
}

// Search performs upper-layer greedy descent followed by ef-wide best-first
// layer-zero exploration. It returns approximate live candidates, ordered by
// exact metric distance, signed rowid, then immutable node ID. No filter runs
// here. The caller must validate live mappings and source values in its own
// snapshot and apply business filters before reporting results.
//
// Errors return no candidates and preserve counters for work already done.
func Search(ctx context.Context, r Reader, state State, query vector.Vector, cfg Config, ef int, limits Limits) (candidates []Candidate, stats Stats, err error) {
	if ef < 1 || ef > MaxEFSearch {
		return nil, Stats{}, ErrConfig
	}
	if err = ValidateVector(query, cfg); err != nil {
		return nil, Stats{}, err
	}
	o, err := newOperation(ctx, r, state, cfg, limits)
	if err != nil {
		return nil, Stats{}, err
	}
	defer func() {
		stats = o.stats
		if err != nil {
			candidates = nil
			stats.Exhausted = false
			switch {
			case errors.Is(err, ErrLimit):
				stats.StopReason = "resource_limit"
			case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
				stats.StopReason = "canceled"
			default:
				stats.StopReason = "error"
			}
		}
	}()
	if state.Live == 0 {
		o.stats.Exhausted = true
		o.stats.StopReason = "empty"
		return nil, Stats{}, nil
	}
	entry, err := o.score(query, state.Entry, state.MaxLevel)
	if err != nil {
		return nil, Stats{}, err
	}
	if o.cache[state.Entry].Level != state.MaxLevel {
		return nil, Stats{}, ErrCorrupt
	}
	for layer := state.MaxLevel; layer > 0; layer-- {
		entry, err = o.greedy(query, entry, layer)
		if err != nil {
			return nil, Stats{}, err
		}
	}
	candidates, o.stats.Exhausted, err = o.searchLayer(query, []Candidate{entry}, 0, ef, true)
	if err != nil {
		return nil, Stats{}, err
	}
	if o.stats.Exhausted {
		o.stats.StopReason = "frontier_exhausted"
	} else {
		o.stats.StopReason = "ef_satisfied"
	}
	return candidates, Stats{}, nil
}

// diverse selects nearest candidates whose distance from every previously
// selected neighbor is at least their distance from the center. Rejected
// candidates fill remaining degree slots afterward (keep-pruned-connections),
// preserving local density while preferring diverse routes.
func (o *operation) diverse(center Node, candidates []Candidate, capacity int) ([]int64, error) {
	ordered := append([]Candidate(nil), candidates...)
	sort.Slice(ordered, func(i, j int) bool { return candidateLess(ordered[i], ordered[j]) })
	selected := make([]int64, 0, capacity)
	rejected := make([]int64, 0, len(ordered))
	seen := make(map[int64]bool)
	for _, candidate := range ordered {
		if err := o.ctx.Err(); err != nil {
			return nil, err
		}
		if candidate.NodeID == center.ID || seen[candidate.NodeID] {
			continue
		}
		seen[candidate.NodeID] = true
		n, err := o.get(candidate.NodeID)
		if err != nil {
			return nil, err
		}
		accept := true
		for _, id := range selected {
			other, err := o.get(id)
			if err != nil {
				return nil, err
			}
			d, err := o.distance(n.Vector, other.Vector)
			if err != nil {
				return nil, err
			}
			if d < candidate.Distance {
				accept = false
				break
			}
		}
		if accept {
			selected = append(selected, n.ID)
			if len(selected) == capacity {
				break
			}
		} else {
			rejected = append(rejected, n.ID)
		}
	}
	for _, id := range rejected {
		if len(selected) == capacity {
			break
		}
		selected = append(selected, id)
	}
	sort.Slice(selected, func(i, j int) bool { return selected[i] < selected[j] })
	return selected, nil
}

// Insert connects a new immutable node identity using genuine multilayer
// HNSW and reciprocal diversity pruning. All changed nodes are staged in a
// bounded per-operation cache, then written in node-ID order. Source rowid
// uniqueness is a storage-layer responsibility. State changes only on success.
// A storage error still requires caller-owned statement rollback of all writes.
func Insert(ctx context.Context, w Writer, state *State, rowid int64, value vector.Vector, cfg Config) error {
	if state == nil || nilPort(w) {
		return ErrConfig
	}
	if err := ValidateVector(value, cfg); err != nil {
		return err
	}
	next := *state
	if next == (State{}) {
		next = NewState()
	}
	o, err := newOperation(ctx, w, next, cfg, Limits{})
	if err != nil {
		return err
	}
	if next.NextID > cfg.MaxNodes {
		return fmt.Errorf("%w: total nodes", ErrLimit)
	}
	n := Node{ID: next.NextID, RowID: rowid, Live: true, Level: nodeLevel(cfg.Seed, next.NextID, cfg.M), Vector: value}
	n.Neighbors = make([][]int64, n.Level+1)
	if err := o.retain(n); err != nil {
		return err
	}
	dirty := map[int64]bool{n.ID: true}
	if next.Entry != 0 {
		entry, err := o.score(value, next.Entry, next.MaxLevel)
		if err != nil {
			return err
		}
		if o.cache[next.Entry].Level != next.MaxLevel {
			return ErrCorrupt
		}
		for layer := next.MaxLevel; layer > n.Level; layer-- {
			entry, err = o.greedy(value, entry, layer)
			if err != nil {
				return err
			}
		}
		entries := []Candidate{entry}
		start := n.Level
		if next.MaxLevel < start {
			start = next.MaxLevel
		}
		for layer := start; layer >= 0; layer-- {
			found, _, err := o.searchLayer(value, entries, layer, cfg.EFConstruction, false)
			if err != nil {
				return err
			}
			links, err := o.diverse(n, found, cfg.M)
			if err != nil {
				return err
			}
			n = cloneNode(n)
			n.Neighbors[layer] = links
			if err := o.retain(n); err != nil {
				return err
			}
			for _, id := range links {
				neighbor, err := o.get(id)
				if err != nil {
					return err
				}
				if neighbor.Level < layer {
					return ErrCorrupt
				}
				neighbor = cloneNode(neighbor)
				ids := append(append([]int64(nil), neighbor.Neighbors[layer]...), n.ID)
				if len(ids) > layerCapacity(cfg, layer) {
					choices := make([]Candidate, 0, len(ids))
					for _, other := range ids {
						c, err := o.score(neighbor.Vector, other, layer)
						if err != nil {
							return err
						}
						choices = append(choices, c)
					}
					ids, err = o.diverse(neighbor, choices, layerCapacity(cfg, layer))
					if err != nil {
						return err
					}
				} else {
					sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
				}
				neighbor.Neighbors[layer] = ids
				if err := o.retain(neighbor); err != nil {
					return err
				}
				dirty[id] = true
			}
			entries = found
		}
	}
	ids := make([]int64, 0, len(dirty))
	for id := range dirty {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		if err := ctx.Err(); err != nil {
			return err
		}
		node := o.cache[id]
		if err := ValidateNode(node, cfg); err != nil {
			return err
		}
		if err := w.PutNode(ctx, cloneNode(node)); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if next.Entry == 0 || n.Level > next.MaxLevel {
		next.Entry = n.ID
		next.MaxLevel = n.Level
	}
	next.NextID++
	next.Live++
	*state = next
	return nil
}

// Tombstone preserves vector and links and never reuses a node identity. It is
// idempotent for an already-dead node; live-row mapping removal belongs to the
// same caller-owned statement. A configured tombstone limit requires REINDEX.
func Tombstone(ctx context.Context, w Writer, state *State, nodeID int64, cfg Config) error {
	if state == nil || nilPort(w) {
		return ErrConfig
	}
	o, err := newOperation(ctx, w, *state, cfg, Limits{})
	if err != nil {
		return err
	}
	n, err := o.get(nodeID)
	if err != nil {
		return err
	}
	if !n.Live {
		return nil
	}
	if state.Tombstones >= cfg.MaxTombstones {
		return fmt.Errorf("%w: tombstones", ErrLimit)
	}
	if state.Live < 1 {
		return ErrCorrupt
	}
	n.Live = false
	if err := w.PutNode(ctx, n); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	state.Live--
	state.Tombstones++
	return nil
}
