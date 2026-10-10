package sqlite

import (
	"fmt"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

func (v *vectorRank) scoreSparse(r *Rows, value any) (any, error) {
	if value == nil {
		r.queryStats.VectorNullRows++
		return nil, nil
	}
	candidateEntries, err := sparseValueEntries(value)
	if err != nil {
		return nil, err
	}
	queryEntries := v.sparseQueryEntries
	if v.query != nil && !v.prepared {
		queryEntries, err = sparseValueEntries(v.query)
		if err != nil {
			return nil, err
		}
	}
	scratch := sparseScratchBytes(queryEntries, candidateEntries)
	// Reserve support decoding alongside rows already retained by the heap,
	// before decoding either input. General filter/projection work is separate.
	if scratch >= r.db.opts.MaxSortBytes || v.sparseRetainedBytes > r.db.opts.MaxSortBytes || scratch > r.db.opts.MaxSortBytes-v.sparseRetainedBytes {
		return nil, ErrMemoryLimit
	}
	if scratch > v.sparseScratch {
		v.sparseScratch = scratch
	}
	needed := uint64(queryEntries) + uint64(candidateEntries)
	if !v.prepared {
		needed += uint64(queryEntries)
	}
	used := uint64(r.queryStats.VectorRankNonzeros)
	if v.query != nil && (used > v.limits.MaxComponents || needed > v.limits.MaxComponents-used) {
		return nil, fmt.Errorf("%w: sparse nonzeros", vector.ErrLimit)
	}
	candidate, _, err := decodeSparseColumn(value, v.dimensions)
	if err != nil {
		return nil, err
	}
	if v.query == nil {
		r.queryStats.VectorNullRows++
		return nil, nil
	}
	if !v.prepared {
		q, _, e := decodeSparseColumn(v.query, v.dimensions)
		if e != nil {
			return nil, e
		}
		if e = r.ctx.Err(); e != nil {
			return nil, e
		}
		v.parsedSparse, v.prepared = q, true
		v.sparseQueryEntries = queryEntries
		r.queryStats.VectorRankNonzeros = int64(queryEntries)
	}
	distance, err := v.metric.DistanceSparseContext(r.ctx, candidate, v.parsedSparse)
	if err != nil {
		return nil, err
	}
	r.queryStats.VectorRankedRows++
	r.queryStats.VectorRankNonzeros += int64(queryEntries + candidateEntries)
	return distance, nil
}
