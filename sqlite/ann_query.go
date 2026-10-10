package sqlite

import (
	"bytes"
	"container/heap"
	"context"
	"fmt"
	"sort"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/extension/vector/ann"
	"github.com/ms-xiaosnow/gosqlite/internal/btree"
)

// VectorSearchMode controls recall. An empty mode is exact for compatibility.
type VectorSearchMode string

const (
	VectorModeExact       VectorSearchMode = "exact"
	VectorModeApproximate VectorSearchMode = "approximate"
)

// ANNWorkLimits are cumulative operation limits, including repeated widening
// searches. Node reads include graph and identity-validation node fetches.
// IVF separately bounds centroid records and selected covering member keys;
// each member additionally receives a table/reverse-key identity recheck.
// These limits count logical records/keys, not total shadow or page I/O. Distance
// evaluations share one allowance across graph distances and centroid scores.
// MaxBytes bounds the current algorithm cache; source/result bookkeeping also
// shares DB.MaxSortBytes. Zero fields use bounded defaults, never infinity.
// Existing vector.SearchLimits separately bound cumulative source fetches and
// all query, graph-distance, centroid-distance, and base-row-rescoring components.
type ANNWorkLimits struct {
	MaxNodeReads           int
	MaxCentroidReads       int
	MaxMemberKeysRead      int
	MaxDistanceEvaluations int
	MaxIterations          int
	MaxBytes               int64
}

// DefaultANNWorkLimits returns the bounded cumulative query defaults.
func DefaultANNWorkLimits() ANNWorkLimits {
	return ANNWorkLimits{MaxNodeReads: 100000, MaxCentroidReads: 32768, MaxMemberKeysRead: 1 << 20, MaxDistanceEvaluations: 1000000, MaxIterations: 16, MaxBytes: 32 << 20}
}
func normalizeVectorANNOptions(o VectorSearchOptions) (VectorSearchOptions, error) {
	if o.Mode == "" {
		o.Mode = VectorModeExact
	}
	if o.Mode != VectorModeExact && o.Mode != VectorModeApproximate {
		return o, vector.ErrOptions
	}
	if o.Probes < 0 || o.Probes > 32768 || o.MaxProbes < 0 || o.MaxProbes > 32768 || (o.MaxProbes != 0 && o.MaxProbes < o.Probes) {
		return o, vector.ErrOptions
	}
	if o.EFSearch == 0 {
		o.EFSearch = 40
	}
	if o.MaxEFSearch == 0 {
		o.MaxEFSearch = 1000
	}
	if o.EFSearch < 1 || o.EFSearch > 1000 || o.MaxEFSearch < o.EFSearch || o.MaxEFSearch > 1000 {
		return o, vector.ErrOptions
	}
	defaults := DefaultANNWorkLimits()
	if o.ANNLimits.MaxNodeReads == 0 {
		o.ANNLimits.MaxNodeReads = defaults.MaxNodeReads
	}
	if o.ANNLimits.MaxCentroidReads == 0 {
		o.ANNLimits.MaxCentroidReads = defaults.MaxCentroidReads
	}
	if o.ANNLimits.MaxMemberKeysRead == 0 {
		o.ANNLimits.MaxMemberKeysRead = defaults.MaxMemberKeysRead
	}
	if o.ANNLimits.MaxDistanceEvaluations == 0 {
		o.ANNLimits.MaxDistanceEvaluations = defaults.MaxDistanceEvaluations
	}
	if o.ANNLimits.MaxIterations == 0 {
		o.ANNLimits.MaxIterations = defaults.MaxIterations
	}
	if o.ANNLimits.MaxBytes == 0 {
		o.ANNLimits.MaxBytes = defaults.MaxBytes
	}
	w := o.ANNLimits
	if w.MaxNodeReads < 1 || w.MaxNodeReads > 1<<24 || w.MaxCentroidReads < 1 || w.MaxCentroidReads > 1<<24 || w.MaxMemberKeysRead < 1 || w.MaxMemberKeysRead > 1<<24 || w.MaxDistanceEvaluations < 1 || w.MaxDistanceEvaluations > 1<<28 || w.MaxIterations < 1 || w.MaxIterations > 64 || w.MaxBytes < 1 || w.MaxBytes > 1<<34 {
		return o, vector.ErrOptions
	}
	return o, nil
}
func (p QueryPlan) annDetail() string {
	if p.ANNOpclass == "" && p.ANNFallback == "" {
		return ""
	}
	return fmt.Sprintf("; ann_opclass=%s; ann_generation=%d; ef_search=%d; max_ef_search=%d; effective_ef=%d; lists=%d; probes=%d; max_probes=%d; effective_probes=%d; training_state=%s; ann_stop=%s; ann_fallback=%s", p.ANNOpclass, p.ANNGeneration, p.ANNEFSearch, p.ANNMaxEFSearch, p.ANNEffectiveEF, p.ANNLists, p.ANNProbes, p.ANNMaxProbes, p.ANNEffectiveProbes, p.ANNTrainingState, p.ANNStopReason, p.ANNFallback)
}

// selectANNIndex resolves only immutable metadata in the caller's snapshot.
// It never loads a graph node, including when called for EXPLAIN.
func (d *DB) selectANNIndex(ctx context.Context, reader btree.PageReader, tab *table, ordinal int, metric vector.MetricKind, dims int, name string) (*extensionIndex, string, error) {
	catalog, err := d.loadExtensions(ctx, reader)
	if err != nil {
		return nil, "", err
	}
	kind, _, ok := annColumnType(tab.columns[ordinal].Type)
	if !ok {
		return nil, "source is not a declared vector", nil
	}
	opclass, err := ann.OpclassFor(kind, metric)
	if err != nil {
		return nil, "", err
	}
	reason := "no matching ANN index"
	for _, x := range catalog.indexes {
		if name != "" && fold(x.name) != fold(name) {
			continue
		}
		if x.ann == nil {
			if name != "" {
				reason = "requested index is not an ANN index"
			}
			continue
		}
		if fold(x.sourceTable) != fold(tab.name) || len(x.fields) != 1 || x.fields[0].ordinal != ordinal || fold(x.fields[0].name) != fold(tab.columns[ordinal].Name) {
			if name != "" {
				reason = "requested ANN index source does not match"
			}
			continue
		}
		if x.ann.Kind != kind || x.ann.Metric != metric || x.ann.Opclass != opclass || x.ann.Dimensions != dims {
			reason = "ANN metric, opclass, or dimensions do not match"
			continue
		}
		if !x.usable {
			reason = x.fallback
			continue
		}
		if x.method != ann.HNSW && x.method != ann.IVFFlat {
			reason = "ANN method is not supported for this query"
			continue
		}
		return x, "", nil
	}
	return nil, reason, nil
}
func applyANNPlan(p *QueryPlan, x *extensionIndex, o VectorSearchOptions) {
	p.Access = "SEARCH"
	p.Indexes = []string{x.name}
	p.Method = x.method
	p.MethodVersion = x.methodVersion
	p.Fields = []string{x.fields[0].name}
	p.Namespace = ""
	p.GramLengths = nil
	p.EstimatedCandidates = -1
	p.Recheck = true
	p.Reason = "lazy HNSW traversal with original WHERE and exact base-row rescoring"
	p.VectorExact = false
	p.VectorRanking = "ann_exact_rescore"
	p.ANNOpclass = x.ann.Opclass
	p.ANNGeneration = x.generation
	p.ANNEFSearch = o.EFSearch
	p.ANNMaxEFSearch = o.MaxEFSearch
	if x.method == ann.IVFFlat {
		p.Reason = "lazy IVFFlat selected-list ranges with original WHERE and exact base-row rescoring"
		p.ANNEFSearch, p.ANNMaxEFSearch = 0, 0
		p.ANNLists, p.ANNProbes, p.ANNMaxProbes = x.ann.Lists, o.Probes, o.MaxProbes
	}
}
func exactANNFallback(p *QueryPlan, reason string) {
	p.Access = "SCAN"
	p.Indexes = nil
	p.Method = "exact_vector"
	switch p.VectorRepresentation {
	case "halfvec":
		p.Method = "exact_halfvec"
	case "bit":
		p.Method = "exact_bit"
	case "sparsevec":
		p.Method = "exact_sparsevec"
	}
	p.MethodVersion = 1
	p.Namespace = ""
	p.GramLengths = nil
	p.EstimatedCandidates = -1
	p.Recheck = false
	p.VectorExact = true
	p.VectorRanking = "bounded_heap"
	p.Reason = "ANN exact fallback: " + reason
	p.ANNFallback = reason
}

// ANN candidates must refer to the same original source-vector bits. This runs
// after WHERE, so excluded rows are not decoded or scored merely for a recheck.
func decodeANNVector(value any, dims int, expected vector.Vector) (vector.Vector, error) {
	v, null, err := decodeVectorColumn(value, dims)
	if err != nil {
		return vector.Vector{}, fmt.Errorf("%w: invalid ANN source vector: %w", ErrCorrupt, err)
	}
	if null {
		return vector.Vector{}, extensionCorrupt("live ANN node refers to NULL source vector")
	}
	got, err := v.MarshalBinary()
	if err != nil {
		return vector.Vector{}, err
	}
	want, err := expected.MarshalBinary()
	if err != nil {
		return vector.Vector{}, err
	}
	if !bytes.Equal(got, want) {
		return vector.Vector{}, extensionCorrupt("live ANN node/source vector mismatch")
	}
	return v, nil
}

type annCandidateQuery struct {
	db      *DB
	reader  btree.PageReader
	table   *table
	index   *extensionIndex
	query   ann.Value
	retain  int
	options VectorSearchOptions
	limits  vector.SearchLimits
	plan    *QueryPlan
	stats   *QueryStats
	state   *ann.State // optional state metadata already read by the SQL planner
	// Components already consumed by query validation and exact rescoring.
	components func() int64
	// Caller-owned result memory retained in addition to query/hit bookkeeping.
	retainedBytes func() int64
	annWork       *ANNWorkStats
	activeBytes   int64 // current candidate bookkeeping; shared with typed scratch admission
}

type annCountedReader struct {
	store        *annStore
	reads, limit int
}

func (r *annCountedReader) GetNode(ctx context.Context, id int64) (ann.TypedNode, error) {
	if r.reads >= r.limit {
		return ann.TypedNode{}, fmt.Errorf("%w: cumulative ANN node reads", vector.ErrLimit)
	}
	r.reads++
	return r.store.GetNode(ctx, id)
}

type annHitHeap []vector.Hit

func (h annHitHeap) Len() int           { return len(h) }
func (h annHitHeap) Less(i, j int) bool { return annHitBetter(h[j], h[i]) }
func (h annHitHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *annHitHeap) Push(v any)        { *h = append(*h, v.(vector.Hit)) }
func (h *annHitHeap) Pop() any          { a := *h; v := a[len(a)-1]; *h = a[:len(a)-1]; return v }
func annHitBetter(a, b vector.Hit) bool {
	return a.Distance < b.Distance || a.Distance == b.Distance && a.RowID < b.RowID
}

// run does not filter traversal. Repeated graph searches charge every actual
// read/distance; source rechecks are deduplicated by immutable node identity.
// A short discovered frontier is not proof of exact source exhaustion.
func (a *annCandidateQuery) run(ctx context.Context, visit func(int64, []any, *ann.Value) (float64, bool, error)) ([]vector.Hit, string, error) {
	store := newANNStore(a.reader, a.index)
	var state ann.State
	var err error
	if a.state != nil {
		state = *a.state
	} else {
		state, err = store.GetState(ctx)
		a.stats.ANNStateReads++
		if err != nil {
			return nil, "", err
		}
	}
	a.plan.ANNTrainingState = state.TrainingState
	if state.Live > a.index.documentCount || (a.table.columns[a.index.fields[0].ordinal].NotNull && state.Live != a.index.documentCount) {
		return nil, "", extensionCorrupt("ANN live count does not match source cardinality")
	}
	if state.TrainingState != "" && state.TrainingState != "ready" {
		a.plan.ANNStopReason = "untrained"
		return nil, "index is untrained; REINDEX required to train", nil
	}
	if a.index.method == ann.IVFFlat {
		return a.runIVF(ctx, store, state, visit)
	}
	if state.Live == 0 {
		a.plan.ANNStopReason = "empty"
		return nil, "", nil
	}
	// A LIMIT is a ceiling, not a promise that this many source rows exist.
	// Bound the actual retained heap/reservation by snapshot cardinality while
	// leaving the caller's requested LIMIT/OFFSET and plan diagnostics intact.
	// The conversion is safe because this branch proves Live < retain (an int).
	retain := a.retain
	if state.Live < int64(retain) {
		retain = int(state.Live)
	}
	cfg := *a.index.ann
	seen := map[int64]int64{}
	rowNodes := map[int64]int64{}
	retained := annHitHeap{}
	ef := max(a.options.EFSearch, min(retain, a.options.MaxEFSearch))
	for {
		if err := ctx.Err(); err != nil {
			return nil, "", err
		}
		if a.stats.ANNIterations >= int64(a.options.ANNLimits.MaxIterations) {
			return nil, "", fmt.Errorf("%w: ANN iterations", vector.ErrLimit)
		}
		remainingReads := a.options.ANNLimits.MaxNodeReads - int(a.stats.ANNNodeReads)
		remainingDistances := a.options.ANNLimits.MaxDistanceEvaluations - int(a.stats.ANNDistanceEvaluations)
		used := uint64(a.components()) + uint64(a.work().ComponentWork)
		if used > a.limits.MaxComponents {
			return nil, "", fmt.Errorf("%w: cumulative ANN components", vector.ErrLimit)
		}

		reserved := a.query.MemoryBytes() + int64(retain)*128 + int64(len(seen))*128 + a.retainedBytes()
		memory := min(a.options.ANNLimits.MaxBytes, a.db.opts.MaxSortBytes-reserved)
		if remainingReads < 1 || remainingDistances < 1 {
			return nil, "", fmt.Errorf("%w: cumulative ANN work", vector.ErrLimit)
		}
		if memory < 1 {
			return nil, "", ErrMemoryLimit
		}
		counted := &annCountedReader{store: store, limit: remainingReads}
		candidates, work, err := ann.SearchValue(ctx, counted, state, a.query, cfg, ef, ann.TypedLimits{Limits: ann.Limits{MaxVisited: min(cfg.MaxVisited, remainingReads), MaxDistanceEvaluations: min(cfg.MaxDistanceEvaluations, remainingDistances), MaxBytes: min(cfg.MaxMemoryBytes, memory)}, ComponentWorkLimited: true, MaxComponentWork: a.limits.MaxComponents - used})
		a.stats.ANNIterations++
		a.stats.ANNNodeReads += int64(counted.reads)
		a.stats.ANNEdgeReads += int64(work.EdgeReads)
		a.stats.ANNVisited += int64(work.Visited)
		a.stats.ANNDistanceEvaluations += int64(work.DistanceEvaluations)
		a.work().ComponentWork += work.ComponentWork
		a.stats.ANNPeakBytes = max(a.stats.ANNPeakBytes, work.PeakBytes+reserved)
		a.plan.ANNEffectiveEF = ef
		a.plan.ANNStopReason = work.StopReason
		if err != nil {
			return nil, "", err
		}
		candidateBytes := int64(len(candidates)) * 32
		for _, candidate := range candidates {
			if row, ok := seen[candidate.NodeID]; ok {
				if row != candidate.RowID {
					return nil, "", extensionCorrupt("ANN node identity changed")
				}
				continue
			}
			if node, ok := rowNodes[candidate.RowID]; ok && node != candidate.NodeID {
				return nil, "", extensionCorrupt("duplicate live ANN source identity")
			}
			if candidateBytes+int64(len(seen)+1)*128+int64(retain)*128+a.query.MemoryBytes()+a.retainedBytes() > a.db.opts.MaxSortBytes {
				return nil, "", ErrMemoryLimit
			}
			seen[candidate.NodeID] = candidate.RowID
			rowNodes[candidate.RowID] = candidate.NodeID
			if int64(len(seen)) > state.Live {
				return nil, "", extensionCorrupt("ANN candidates exceed live count")
			}
			a.stats.Candidates++
			a.stats.ANNIdentityLookups++
			live, found, err := store.LiveNode(ctx, candidate.RowID)
			if err != nil {
				return nil, "", err
			}
			if !found || live != candidate.NodeID {
				return nil, "", extensionCorrupt("ANN live mapping mismatch")
			}
			if a.stats.ANNNodeReads >= int64(a.options.ANNLimits.MaxNodeReads) {
				return nil, "", fmt.Errorf("%w: cumulative ANN node reads", vector.ErrLimit)
			}
			a.stats.ANNNodeReads++
			node, err := store.GetNode(ctx, candidate.NodeID)
			if err != nil {
				return nil, "", err
			}
			if !node.Live || node.RowID != candidate.RowID || node.ID != candidate.NodeID {
				return nil, "", extensionCorrupt("ANN candidate node mismatch")
			}
			reserved := candidateBytes + int64(len(seen))*128 + int64(retain)*128 + a.query.MemoryBytes() + a.retainedBytes()
			if ann.TypedNodeMemoryBytes(node) > a.db.opts.MaxSortBytes-reserved {
				return nil, "", ErrMemoryLimit
			}
			if a.stats.BaseRowFetches >= int64(a.limits.MaxCandidates) {
				return nil, "", fmt.Errorf("%w: cumulative source candidates", vector.ErrLimit)
			}
			raw, found, err := btree.SeekTable(ctx, a.reader, a.table.root, candidate.RowID)
			if err != nil {
				return nil, "", err
			}
			if !found {
				return nil, "", extensionCorrupt("ANN candidate refers to missing source row")
			}

			a.activeBytes = reserved + ann.TypedNodeMemoryBytes(node)
			value := ann.NodeValue(node)
			distance, keep, err := visit(candidate.RowID, raw, &value)
			if err != nil {
				return nil, "", err
			}
			if err = ctx.Err(); err != nil {
				return nil, "", err
			}
			recheckBytes := candidateBytes + int64(len(seen))*128 + int64(retain)*128 + a.query.MemoryBytes() + a.retainedBytes() + ann.TypedNodeMemoryBytes(node)
			a.stats.ANNPeakBytes = max(a.stats.ANNPeakBytes, recheckBytes)
			if recheckBytes > a.db.opts.MaxSortBytes {
				return nil, "", ErrMemoryLimit
			}
			if !keep {
				continue
			}
			hit := vector.Hit{RowID: candidate.RowID, Distance: distance}
			if len(retained) < retain {
				heap.Push(&retained, hit)
			} else if annHitBetter(hit, retained[0]) {
				retained[0] = hit
				heap.Fix(&retained, 0)
			}
		}
		if len(retained) >= retain {
			a.plan.ANNStopReason = "eligible_limit_satisfied"
			if retain < a.retain {
				a.plan.ANNStopReason = "all_live_identities_rechecked"
			}
			break
		}
		if int64(len(seen)) == state.Live {
			a.plan.ANNStopReason = "all_live_identities_rechecked"
			break
		}
		if ef >= a.options.MaxEFSearch {
			return nil, "insufficient eligible ANN hits at max_ef_search", nil
		}
		// Widen even after frontier exhaustion: a larger ef may discover a wider
		// route. Only the exact fallback may claim complete source exhaustion.
		ef = min(a.options.MaxEFSearch, max(ef+1, ef*2))
	}
	hits := append([]vector.Hit(nil), retained...)
	sort.Slice(hits, func(i, j int) bool { return annHitBetter(hits[i], hits[j]) })
	return hits, "", ctx.Err()
}

func (d *DB) vectorSearchANN(ctx context.Context, reader btree.PageReader, tab *table, ordinal int, q vector.Vector, options VectorSearchOptions, limits vector.SearchLimits, filter func(int64, []any) (bool, error), result *VectorResult, exactSourceBound *int64) (bool, error) {
	x, why, err := d.selectANNIndex(ctx, reader, tab, ordinal, result.Plan.Metric, q.Dims(), options.Index)
	if err != nil {
		return false, err
	}
	if x == nil {
		exactANNFallback(&result.Plan.QueryPlan, why)
		return false, nil
	}
	options, err = annIndexQueryOptions(x, options)
	if err != nil {
		return false, err
	}
	applyANNPlan(&result.Plan.QueryPlan, x, options)
	// A clean, snapshot-bound catalog provides an upper bound on all eligible
	// exact-fallback hits. LIMIT is a ceiling, not a preallocated row count.
	*exactSourceBound = x.documentCount
	result.Plan.Exact = false
	helper := &writeTable{table: tab}
	run := annCandidateQuery{db: d, reader: reader, table: tab, index: x, query: ann.DenseValue(q), retain: options.Limit, options: options, limits: limits, plan: &result.Plan.QueryPlan, stats: &result.Stats.QueryStats, components: func() int64 { return result.Stats.ScoredComponents }, retainedBytes: func() int64 { return 0 }}
	hits, why, err := run.run(ctx, func(id int64, raw []any, expected *ann.Value) (float64, bool, error) {
		result.Stats.BaseRowFetches++
		values, err := helper.values(id, raw)
		if err != nil {
			return 0, false, err
		}
		if filter != nil {
			keep, err := filter(id, values)
			if err != nil {
				return 0, false, err
			}
			if err = ctx.Err(); err != nil {
				return 0, false, err
			}
			if !keep {
				result.Stats.FilteredRows++
				return 0, false, nil
			}
		}
		v, err := decodeANNCandidate(values[ordinal], q.Dims(), expected)
		if err != nil {
			return 0, false, err
		}
		used := uint64(result.Stats.ScoredComponents) + annDistanceComponents(&result.Stats.QueryStats, q.Dims())
		if used > limits.MaxComponents || uint64(q.Dims()) > limits.MaxComponents-used {
			return 0, false, fmt.Errorf("%w: cumulative vector components", vector.ErrLimit)
		}
		distance, err := result.Plan.Metric.Distance(q, v)
		if err != nil {
			return 0, false, err
		}
		result.Stats.ScoredRows++
		result.Stats.ScoredComponents += int64(q.Dims())
		result.Stats.ANNExactRescores++
		return distance, true, nil
	})
	if err != nil {
		return false, err
	}
	if why != "" {
		exactANNFallback(&result.Plan.QueryPlan, why)
		result.Plan.Exact = true
		return false, nil
	}
	result.Hits = hits
	result.Stats.ReturnedRows = int64(len(hits))
	return true, nil
}
