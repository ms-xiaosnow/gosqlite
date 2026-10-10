package sqlite

import (
	"container/heap"
	"fmt"
	"io"
	"math"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// vectorRank is only the bounded ranking stage. Source access remains the
// existing SCAN or safe business-predicate index plan. It is not an ANN index.
type vectorRank struct {
	ordinal, dimensions int
	query               any
	metric              vector.MetricKind
	retain              int
	limits              vector.SearchLimits
	prepared            bool
	parsed              vector.Vector
}

func vectorDistanceExpression(e sqlast.Expr) (vector.MetricKind, sqlast.Expr, sqlast.Expr, bool) {
	switch x := e.(type) {
	case *sqlast.Binary:
		var metric vector.MetricKind
		switch x.Op {
		case "<->":
			metric = vector.L2
		case "<+>":
			metric = vector.L1
		case "<=>":
			metric = vector.Cosine
		case "<#>":
			metric = vector.NegativeIP
		default:
			return "", nil, nil, false
		}
		return metric, x.Left, x.Right, true
	case *sqlast.Call:
		if len(x.Args) != 2 {
			return "", nil, nil, false
		}
		var metric vector.MetricKind
		switch fold(x.Name) {
		case "l2_distance":
			metric = vector.L2
		case "l1_distance":
			metric = vector.L1
		case "cosine_distance":
			metric = vector.Cosine
		case "negative_inner_product":
			metric = vector.NegativeIP
		default:
			return "", nil, nil, false
		}
		return metric, x.Args[0], x.Args[1], true
	}
	return "", nil, nil, false
}
func vectorConstant(e sqlast.Expr) bool {
	switch e.(type) {
	case *sqlast.Literal, *sqlast.Parameter:
		return true
	}
	return false
}

// Recognize only a single ASC metric key over a typed column and a literal or
// bind. Aliases/ordinals have already been bound by plan. Secondary keys, DESC,
// arbitrary expression contexts and unlimited sorts keep their original path.
func (r *Rows) prepareVectorRank() error {
	if r.table == nil || len(r.order) != 1 || r.order[0].desc || r.limit < 0 {
		return nil
	}
	key := r.order[0].expr
	if r.order[0].column >= 0 {
		key = r.exprs[r.order[0].column]
	}
	metric, left, right, ok := vectorDistanceExpression(key)
	if !ok {
		return nil
	}
	col, ok := left.(*sqlast.Column)
	if !ok || !vectorConstant(right) {
		col, ok = right.(*sqlast.Column)
		if !ok || !vectorConstant(left) {
			return nil
		}
		right = left
	}
	ordinal, err := r.table.column(col.Table, col.Name)
	if err != nil || ordinal < 0 {
		return nil
	}
	dims, ok := sqlast.VectorDimensions(r.table.columns[ordinal].Type)
	if !ok {
		return nil
	}
	if r.limit != 0 && r.offset > math.MaxInt64-r.limit {
		return fmt.Errorf("%w: vector LIMIT plus OFFSET", vector.ErrLimit)
	}
	retain := r.limit + r.offset
	if r.limit == 0 {
		retain = 0
	}
	limits := vector.DefaultSearchLimits()
	if retain > int64(limits.MaxResults) {
		return fmt.Errorf("%w: vector LIMIT plus OFFSET", vector.ErrLimit)
	}
	query, err := sqlast.Eval(right, &sqlast.EvalContext{Context: r.ctx, Params: r.params})
	if err != nil {
		return err
	}
	// Preserve scalar laziness: do not parse input or reject zero cosine norms
	// until a non-NULL row actually requires a distance. EXPLAIN does no scoring.
	r.vectorRank = &vectorRank{ordinal: ordinal, dimensions: dims, query: query, metric: metric, retain: int(retain), limits: limits}
	r.queryPlan.VectorMetric = metric
	r.queryPlan.VectorColumn = r.table.columns[ordinal].Name
	r.queryPlan.VectorDimensions = dims
	r.queryPlan.VectorTopK = retain
	r.queryPlan.VectorExact = true
	r.queryPlan.VectorRanking = "bounded_heap"
	return nil
}

func (v *vectorRank) score(r *Rows, value any) (any, error) {
	if value == nil {
		r.queryStats.VectorNullRows++
		return nil, nil
	}
	candidate, _, err := decodeVectorColumn(value, v.dimensions)
	if err != nil {
		return nil, err
	}
	if v.query == nil {
		r.queryStats.VectorNullRows++
		return nil, nil
	}

	if !v.prepared {
		q, _, err := decodeVectorColumn(v.query, v.dimensions)
		if err != nil {
			return nil, err
		}
		if err = r.ctx.Err(); err != nil {
			return nil, err
		}
		if uint64(q.Dims()) > v.limits.MaxComponents {
			return nil, fmt.Errorf("%w: vector components", vector.ErrLimit)
		}
		v.parsed = q
		v.prepared = true
		r.queryStats.VectorRankComponents = int64(q.Dims())
	}
	if uint64(v.dimensions) > v.limits.MaxComponents-uint64(r.queryStats.VectorRankComponents) {
		return nil, fmt.Errorf("%w: vector components", vector.ErrLimit)
	}
	distance, err := v.metric.Distance(candidate, v.parsed)
	if err != nil {
		return nil, err
	}
	if err = r.ctx.Err(); err != nil {
		return nil, err
	}
	r.queryStats.VectorRankedRows++
	r.queryStats.VectorRankComponents += int64(v.dimensions)
	return distance, nil
}

type vectorRankedRow struct {
	row  resultRow
	cost int64
}
type vectorRowHeap []vectorRankedRow

func vectorRowBetter(a, b resultRow) bool {
	// Only validated finite float64 distances or SQL NULL reach this comparator.
	av, bv := a.keys[0], b.keys[0]
	if av == nil && bv != nil {
		return true
	}
	if av != nil && bv == nil {
		return false
	}
	if av != nil {
		af, bf := av.(float64), bv.(float64)
		if af != bf {
			return af < bf
		}
	}
	return a.rowid < b.rowid
}
func (h vectorRowHeap) Len() int           { return len(h) }
func (h vectorRowHeap) Less(i, j int) bool { return vectorRowBetter(h[j].row, h[i].row) }
func (h vectorRowHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *vectorRowHeap) Push(x any)        { *h = append(*h, x.(vectorRankedRow)) }
func (h *vectorRowHeap) Pop() any {
	old := *h
	x := old[len(old)-1]
	old[len(old)-1] = vectorRankedRow{}
	*h = old[:len(old)-1]
	return x
}

func (r *Rows) materializeVector() error {
	retained := vectorRowHeap{}
	var used int64
	for {
		row, cost, err := r.fetch()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if cost > r.db.opts.MaxSortBytes {
			return ErrMemoryLimit
		}
		item := vectorRankedRow{row: row, cost: cost}
		if len(retained) < r.vectorRank.retain {
			if cost > r.db.opts.MaxSortBytes-used {
				return ErrMemoryLimit
			}
			heap.Push(&retained, item)
			used += cost
		} else if len(retained) > 0 && vectorRowBetter(row, retained[0].row) {
			remaining := used - retained[0].cost
			if cost > r.db.opts.MaxSortBytes-remaining {
				return ErrMemoryLimit
			}
			used = remaining + cost
			retained[0] = item
			heap.Fix(&retained, 0)
		}
	}
	if err := r.source.Close(); err != nil {
		return err
	}
	r.source = nil
	r.sorted = make([]resultRow, len(retained))
	for i := len(r.sorted) - 1; i >= 0; i-- {
		if i&255 == 0 {
			if err := r.ctx.Err(); err != nil {
				return err
			}
		}
		r.sorted[i] = heap.Pop(&retained).(vectorRankedRow).row
	}
	return r.ctx.Err()
}
