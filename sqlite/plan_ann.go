package sqlite

import (
	"container/heap"
	"fmt"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/extension/vector/ann"
	"github.com/ms-xiaosnow/gosqlite/internal/btree"
)

func (r *Rows) prepareANNVector(reader btree.PageReader) error {
	v := r.vectorRank
	if v == nil {
		for _, order := range r.order {
			key := order.expr
			if order.column >= 0 {
				key = r.exprs[order.column]
			}
			_, _, _, denseDistance := vectorDistanceExpression(key)
			_, _, _, bitDistance := bitDistanceExpression(key)
			if denseDistance || bitDistance {
				r.queryPlan.ANNFallback = "unsupported ANN SQL shape; requires one ASC distance on a typed vector column, constant/bound query, and bounded LIMIT"
				break
			}
		}
		return nil
	}
	if r.sel.NotIndexed {
		r.queryPlan.ANNFallback = "NOT INDEXED requested"
		return nil
	}
	if !r.table.columns[v.ordinal].NotNull {
		r.queryPlan.ANNFallback = "SQL NULL-first ordering requires a declared NOT NULL vector column"
		return nil
	}
	if v.retain == 0 {
		return nil
	}
	if v.query == nil {
		r.queryPlan.ANNFallback = "NULL query retains SQL NULL-first ordering"
		return nil
	}
	metric := v.metric
	if v.bitMetric != "" {
		metric = vector.MetricKind(v.bitMetric)
	}
	x, why, err := r.db.selectANNIndex(r.ctx, reader, r.table, v.ordinal, metric, v.dimensions, "")
	if err != nil {
		return err
	}
	if x == nil {
		r.queryPlan.ANNFallback = why
		return nil
	}
	options, err := normalizeVectorANNOptions(VectorSearchOptions{Mode: VectorModeApproximate, Limit: v.retain})
	if err != nil {
		return err
	}
	options, err = annIndexQueryOptions(x, options)
	if err != nil {
		return err
	}
	applyANNPlan(&r.queryPlan, x, options)
	v.ann = &annCandidateQuery{db: r.db, reader: reader, table: r.table, index: x, retain: v.retain, options: options, limits: v.limits, plan: &r.queryPlan, stats: &r.queryStats, annWork: &r.annWork, components: func() int64 { return v.rankWork(&r.queryStats) }}
	if x.method == ann.IVFFlat {
		// State is eligibility metadata. EXPLAIN never reads centroids or lists.
		state, err := newANNStore(reader, x).GetState(r.ctx)
		r.queryStats.ANNStateReads++
		if err != nil {
			return err
		}
		v.ann.state = &state
		r.queryPlan.ANNTrainingState = state.TrainingState
		if state.TrainingState != "ready" {
			exactANNFallback(&r.queryPlan, "index is untrained; REINDEX required to train")
		} else if options.Probes == x.ann.Lists {
			exactANNFallback(&r.queryPlan, "probes equals lists")
		}
	}
	return nil
}

func (r *Rows) fallbackANNVector(reason string) error {
	a := r.vectorRank.ann
	reader := a.reader
	r.vectorRank.ann = nil
	r.vectorRank.expected = nil
	r.vectorRank.annCandidate = false
	exactANNFallback(&r.queryPlan, reason)
	if r.source != nil {
		if err := r.source.Close(); err != nil {
			return err
		}
		r.source = nil
	}
	var err error
	r.source, err = btree.NewCursor(r.ctx, reader, r.table.root)
	if err != nil {
		return err
	}
	return r.materializeVector()
}

func (r *Rows) materializeANNVector() error {
	v := r.vectorRank
	a := v.ann
	// SQL keeps its scalar laziness: invalid queries are evaluated only when an
	// eligible source row needs the distance. EXPLAIN never reaches this code.

	// Preserve lazy SQL errors while admitting the full input representation
	// before parsing. TEXT sparse zeros still consume parse/support work.
	queryWork, queryScratch, preflightErr := annQueryPreflight(v.query, *a.index.ann)
	if preflightErr != nil || queryWork > v.limits.MaxComponents || queryScratch >= r.db.opts.MaxSortBytes {
		return r.fallbackANNVector("query validation deferred to eligible source rows")
	}
	q, _, err := decodeANNColumn(v.query, *a.index.ann)
	if err != nil {
		return r.fallbackANNVector("query validation deferred to eligible source rows")
	}
	if err = ann.ValidateValue(q, *a.index.ann); err != nil {
		return r.fallbackANNVector("query validation deferred to eligible source rows")
	}
	if queryWork > v.limits.MaxComponents {
		return fmt.Errorf("%w: vector components", vector.ErrLimit)
	}
	v.prepared = true
	switch q.Kind() {
	case ann.DenseKind:
		v.parsed = q.Dense()
		r.queryStats.VectorRankComponents = int64(q.WorkUnits())
	case ann.HalfKind:
		v.parsedHalf = q.Half()
		r.queryStats.VectorRankComponents = int64(q.WorkUnits())
	case ann.BitKind:
		v.parsedBit = q.Bit()
		r.queryStats.VectorRankPackedBytes = int64(q.WorkUnits())
	case ann.SparseKind:
		v.parsedSparse = q.Sparse()
		v.sparseQueryEntries = int(queryWork)
		r.queryStats.VectorRankNonzeros = int64(queryWork)
	}
	a.query = q
	r.queryStats.ANNPeakBytes = max(r.queryStats.ANNPeakBytes, queryScratch)
	if r.source != nil {
		if err := r.source.Close(); err != nil {
			return err
		}
		r.source = nil
	}
	retained := vectorRowHeap{}
	var used int64
	a.retainedBytes = func() int64 { return used }
	_, why, err := a.run(r.ctx, func(id int64, raw []any, expected *ann.Value) (float64, bool, error) {
		v.expected = expected
		v.annCandidate = true
		row, cost, keep, err := r.evaluateSourceRow(id, raw)
		v.expected = nil
		v.annCandidate = false
		if err != nil || !keep {
			return 0, false, err
		}
		if cost > r.db.opts.MaxSortBytes {
			return 0, false, ErrMemoryLimit
		}
		item := vectorRankedRow{row: row, cost: cost}
		if len(retained) < v.retain {
			if cost > r.db.opts.MaxSortBytes-used {
				return 0, false, ErrMemoryLimit
			}
			heap.Push(&retained, item)
			used += cost
		} else if vectorRowBetter(row, retained[0].row) {
			remaining := used - retained[0].cost
			if cost > r.db.opts.MaxSortBytes-remaining {
				return 0, false, ErrMemoryLimit
			}
			used = remaining + cost
			retained[0] = item
			heap.Fix(&retained, 0)
		}
		return row.keys[0].(float64), true, nil
	})
	if err != nil {
		return err
	}
	if why != "" {
		// Do not merge a scan with approximate ranks: discard the heap, retain only
		// actual attempted-work counters, then rebuild ranking from the full source.
		retained = nil
		used = 0
		return r.fallbackANNVector(why)
	}
	r.sorted = make([]resultRow, len(retained))
	for i := len(r.sorted) - 1; i >= 0; i-- {
		if err := r.ctx.Err(); err != nil {
			return err
		}
		r.sorted[i] = heap.Pop(&retained).(vectorRankedRow).row
	}
	return r.ctx.Err()
}
