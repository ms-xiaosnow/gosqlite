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
			if _, _, _, distance := vectorDistanceExpression(key); distance {
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
	x, why, err := r.db.selectANNIndex(r.ctx, reader, r.table, v.ordinal, v.metric, v.dimensions, "")
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
	applyANNPlan(&r.queryPlan, x, options)
	v.ann = &annCandidateQuery{db: r.db, reader: reader, table: r.table, index: x, retain: v.retain, options: options, limits: v.limits, plan: &r.queryPlan, stats: &r.queryStats, components: func() int64 { return r.queryStats.VectorRankComponents }}
	return nil
}

func (r *Rows) fallbackANNVector(reason string) error {
	a := r.vectorRank.ann
	reader := a.reader
	r.vectorRank.ann = nil
	r.vectorRank.expected = nil
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
	q, _, err := decodeVectorColumn(v.query, v.dimensions)
	if err != nil {
		return r.fallbackANNVector("query validation deferred to eligible source rows")
	}
	if err = ann.ValidateVector(q, *a.index.ann); err != nil {
		return r.fallbackANNVector("query validation deferred to eligible source rows")
	}
	if uint64(v.dimensions) > v.limits.MaxComponents {
		return fmt.Errorf("%w: vector components", vector.ErrLimit)
	}
	v.prepared = true
	v.parsed = q
	r.queryStats.VectorRankComponents = int64(q.Dims())
	a.query = q
	if r.source != nil {
		if err := r.source.Close(); err != nil {
			return err
		}
		r.source = nil
	}
	retained := vectorRowHeap{}
	var used int64
	a.retainedBytes = func() int64 { return used }
	_, why, err := a.run(r.ctx, func(id int64, raw []any, expected vector.Vector) (float64, bool, error) {
		v.expected = &expected
		row, cost, keep, err := r.evaluateSourceRow(id, raw)
		v.expected = nil
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
