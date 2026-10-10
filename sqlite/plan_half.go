package sqlite

import (
	"fmt"
	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

func (v *vectorRank) scoreHalf(r *Rows, value any) (any, error) {
	if value == nil {
		r.queryStats.VectorNullRows++
		return nil, nil
	}
	// Check component and scratch budgets before decoding either borrowed value.
	// A NULL query still validates the selected typed candidate, like scalar SQL.
	if int64(v.dimensions)*16 >= r.db.opts.MaxSortBytes {
		return nil, ErrMemoryLimit
	}
	components := uint64(v.dimensions)
	if v.query != nil {
		needed := components
		if !v.prepared {
			needed += components
		}
		used := uint64(r.queryStats.VectorRankComponents)
		if used > v.limits.MaxComponents || needed > v.limits.MaxComponents-used {
			return nil, fmt.Errorf("%w: half components", vector.ErrLimit)
		}
	}
	candidate, _, err := decodeHalfColumn(value, v.dimensions)
	if err != nil {
		return nil, err
	}
	if v.query == nil {
		r.queryStats.VectorNullRows++
		return nil, nil
	}
	if !v.prepared {
		q, _, e := decodeHalfColumn(v.query, v.dimensions)
		if e != nil {
			return nil, e
		}
		if e = r.ctx.Err(); e != nil {
			return nil, e
		}
		v.parsedHalf, v.prepared = q, true
		r.queryStats.VectorRankComponents = int64(components)
	}
	distance, err := v.metric.DistanceHalfContext(r.ctx, candidate, v.parsedHalf)
	if err != nil {
		return nil, err
	}
	r.queryStats.VectorRankedRows++
	r.queryStats.VectorRankComponents += int64(components)
	return distance, nil
}
