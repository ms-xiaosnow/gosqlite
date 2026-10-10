package sqlite

import (
	"io"
	"math"

	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// SQLite's finite-limit result sorter checks the sort keys against its worst
// retained candidate before evaluating the remaining scalar projection. Keep
// that observable error/laziness boundary within the grouped path only. The
// sorted array has a bounded insertion cost, charged before every move.
func (r *Rows) materializeGroupedOrdered() error {
	b := r.relational
	k := r.limit
	if r.offset > math.MaxInt64-k {
		k = math.MaxInt64
	} else {
		k += r.offset
	}
	var costs []int64
	var retained int64
	for {
		if err := b.step(1); err != nil {
			return err
		}
		id, raw, err := r.source.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		row, cost, pos, keep, err := r.groupedOrderedCandidate(id, raw, k)
		if err != nil {
			return err
		}
		if !keep {
			continue
		}
		// Admission precedes retention/capacity allocation, with the old worst
		// candidate still charged until the replacement owns all its buffers.
		if cost > r.db.opts.MaxSortBytes-retained {
			return ErrMemoryLimit
		}
		full := int64(len(r.sorted)) == k
		if !full && len(r.sorted) == cap(r.sorted) {
			capacity := max(4, 2*cap(r.sorted))
			if int64(capacity) > k {
				capacity = int(k)
			}
			if err := b.step(int64(len(r.sorted))); err != nil {
				return err
			}
			if err := b.reserve(int64(capacity) * 80); err != nil {
				return err
			}
			rows := make([]resultRow, len(r.sorted), capacity)
			nextCosts := make([]int64, len(costs), capacity)
			copy(rows, r.sorted)
			copy(nextCosts, costs)
			r.sorted, costs = rows, nextCosts
		}
		if err := b.step(int64(len(r.sorted)-pos) + 1); err != nil {
			return err
		}
		if full {
			last := len(costs) - 1
			b.release(costs[last])
			retained -= costs[last]
		} else {
			r.sorted = append(r.sorted, resultRow{})
			costs = append(costs, 0)
		}
		copy(r.sorted[pos+1:], r.sorted[pos:len(r.sorted)-1])
		copy(costs[pos+1:], costs[pos:len(costs)-1])
		r.sorted[pos], costs[pos] = row, cost
		retained += cost
	}
	if err := r.source.Close(); err != nil {
		return err
	}
	r.source = nil
	return nil
}

func (r *Rows) groupedOrderedCandidate(id int64, raw []any, k int64) (resultRow, int64, int, bool, error) {
	b := r.relational
	defer b.clearScalar()
	if err := r.startPredicates(); err != nil {
		if err == io.EOF {
			return resultRow{}, 0, 0, false, nil
		}
		return resultRow{}, 0, 0, false, err
	}
	if err := b.Allocate(int64(len(r.exprs))*17 + int64(len(r.order))*16 + 256); err != nil {
		return resultRow{}, 0, 0, false, err
	}
	ec := &sqlast.EvalContext{Context: r.ctx, Params: r.params, Functions: r.functions, DeclaredType: declaredTypeFor(r.table)}
	ec.Resolver = func(q, n string) (any, error) {
		i, err := r.table.column(q, n)
		if err != nil {
			return nil, err
		}
		if i < 0 {
			return id, nil
		}
		return r.relationalColumnValue(raw[i], r.table.columns[i].Type)
	}
	ec.Affinity = func(q, n string) string {
		i, err := r.table.column(q, n)
		if err != nil {
			return ""
		}
		if i < 0 {
			return "INTEGER"
		}
		return affinity(r.table.columns[i].Type)
	}
	if r.where != nil {
		keep, err := r.where.evaluate(ec)
		if err != nil || !keep {
			return resultRow{}, 0, 0, false, err
		}
	}
	row := resultRow{rowid: id, values: make([]any, len(r.exprs)), keys: make([]any, len(r.order))}
	computed := make([]bool, len(r.exprs))
	for i, o := range r.order {
		var err error
		if o.column >= 0 {
			if !computed[o.column] {
				row.values[o.column], err = sqlast.Eval(r.exprs[o.column], ec)
				computed[o.column] = true
			}
			row.keys[i] = row.values[o.column]
		} else {
			row.keys[i], err = sqlast.Eval(o.expr, ec)
		}
		if err != nil {
			return resultRow{}, 0, 0, false, err
		}
	}
	lo, hi := 0, len(r.sorted)
	for lo < hi {
		mid := lo + (hi-lo)/2
		cmp, err := r.compareRelationalRows(row, r.sorted[mid])
		if err != nil {
			return resultRow{}, 0, 0, false, err
		}
		// Equal keys follow already-seen groups, matching the sorter's sequence
		// tiebreaker. A tied later candidate cannot replace an earlier one.
		if cmp < 0 {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	if int64(lo) >= k {
		return resultRow{}, 0, 0, false, nil
	}
	for i, e := range r.exprs {
		if computed[i] {
			continue
		}
		var err error
		row.values[i], err = sqlast.Eval(e, ec)
		if err != nil {
			return resultRow{}, 0, 0, false, err
		}
	}
	cost := int64(192) + 2*(valuesSize(raw)+valuesSize(row.values)+valuesSize(row.keys))
	// Transfer the still-live scalar buffers into retained accounting before
	// clearScalar runs, so admission includes the peak of both lifetimes.
	if err := b.reserve(cost); err != nil {
		return resultRow{}, 0, 0, false, err
	}
	return row, cost, lo, true, nil
}
