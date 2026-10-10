package sqlite

import (
	"fmt"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

func bitDistanceExpression(e sqlast.Expr) (vector.BitMetricKind, sqlast.Expr, sqlast.Expr, bool) {
	var name string
	var a, b sqlast.Expr
	switch x := e.(type) {
	case *sqlast.Binary:
		name, a, b = x.Op, x.Left, x.Right
	case *sqlast.Call:
		if len(x.Args) != 2 {
			return "", nil, nil, false
		}
		name, a, b = fold(x.Name), x.Args[0], x.Args[1]
	default:
		return "", nil, nil, false
	}
	switch name {
	case "<~>", "hamming_distance":
		return vector.Hamming, a, b, true
	case "<%>", "jaccard_distance":
		return vector.Jaccard, a, b, true
	}
	return "", nil, nil, false
}

func (v *vectorRank) scoreBit(r *Rows, value any) (any, error) {
	if value == nil {
		r.queryStats.VectorNullRows++
		return nil, nil
	}
	// Validate the selected typed column even for a NULL query, just as ordinary
	// column interpretation and dense ranking do. WHERE has already run.
	candidate, _, err := decodeBitColumn(value, v.dimensions)
	if err != nil {
		return nil, err
	}
	if v.query == nil {
		r.queryStats.VectorNullRows++
		return nil, nil
	}
	packedBytes := uint64((v.dimensions + 7) / 8)
	if !v.prepared {
		q, _, e := decodeBitColumn(v.query, v.dimensions)
		if e != nil {
			return nil, e
		}
		if e = r.ctx.Err(); e != nil {
			return nil, e
		}
		if packedBytes > v.bitLimits.MaxPackedBytes {
			return nil, fmt.Errorf("%w: bit packed bytes", vector.ErrLimit)
		}
		v.parsedBit, v.prepared = q, true
		r.queryStats.VectorRankPackedBytes = int64(packedBytes)
	}
	used := uint64(r.queryStats.VectorRankPackedBytes) + uint64(r.annWork.ComponentWork)
	if used > v.bitLimits.MaxPackedBytes || packedBytes > v.bitLimits.MaxPackedBytes-used {
		return nil, fmt.Errorf("%w: bit packed bytes", vector.ErrLimit)
	}
	distance, err := v.bitMetric.DistanceContext(r.ctx, candidate, v.parsedBit)
	if err != nil {
		return nil, err
	}
	r.queryStats.VectorRankedRows++
	r.queryStats.VectorRankPackedBytes += int64(packedBytes)
	return distance, nil
}
