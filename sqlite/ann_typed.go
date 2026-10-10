package sqlite

import (
	"bytes"
	"context"
	"fmt"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/extension/vector/ann"
	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// Representation is an index identity, never inferred from a matching width.
func annColumnType(declaration string) (ann.ValueKind, int, bool) {
	if n, ok := sqlast.VectorDimensions(declaration); ok {
		return ann.DenseKind, n, true
	}
	if n, ok := sqlast.HalfDimensions(declaration); ok {
		return ann.HalfKind, n, true
	}
	if n, ok := sqlast.BitDimensions(declaration); ok {
		return ann.BitKind, n, true
	}
	if n, ok := sqlast.SparseDimensions(declaration); ok {
		return ann.SparseKind, n, true
	}
	return ann.DenseKind, 0, false
}

func annCreationMetric(kind ann.ValueKind, metric vector.MetricKind, opclass string) (vector.MetricKind, error) {
	if kind == ann.DenseKind {
		return vectorMetric(metric)
	}
	if metric == "" && opclass != "" {
		for _, candidate := range []vector.MetricKind{vector.L2, vector.L1, vector.NegativeIP, vector.Cosine, vector.MetricKind(vector.Hamming), vector.MetricKind(vector.Jaccard)} {
			want, err := ann.OpclassFor(kind, candidate)
			if err == nil && opclass == want {
				metric = candidate
				break
			}
		}
	}
	if metric == "" {
		metric = vector.L2
		if kind == ann.BitKind {
			metric = vector.MetricKind(vector.Hamming)
		}
	}
	if _, err := ann.OpclassFor(kind, metric); err != nil {
		return "", fmt.Errorf("%w: ANN representation/metric", ErrMismatch)
	}
	return metric, nil
}

func decodeANNColumn(value any, cfg ann.TypedConfig) (ann.Value, bool, error) {
	switch cfg.Kind {
	case ann.DenseKind:
		v, null, err := decodeVectorColumn(value, cfg.Dimensions)
		return ann.DenseValue(v), null, err
	case ann.HalfKind:
		v, null, err := decodeHalfColumn(value, cfg.Dimensions)
		return ann.HalfValue(v), null, err
	case ann.BitKind:
		v, null, err := decodeBitColumn(value, cfg.Dimensions)
		return ann.BitValue(v), null, err
	case ann.SparseKind:
		v, null, err := decodeSparseColumn(value, cfg.Dimensions)
		return ann.SparseValue(v), null, err
	}
	return ann.Value{}, false, ann.ErrConfig
}

func decodeTypedANNCandidate(value any, cfg ann.TypedConfig, expected *ann.Value) (ann.Value, error) {
	v, null, err := decodeANNColumn(value, cfg)
	if err != nil {
		return ann.Value{}, fmt.Errorf("%w: ANN source value: %w", ErrCorrupt, err)
	}
	if null {
		return ann.Value{}, extensionCorrupt("ANN candidate refers to NULL source value")
	}
	if err = ann.ValidateValue(v, cfg); err != nil {
		return ann.Value{}, fmt.Errorf("%w: ANN source value: %w", ErrCorrupt, err)
	}
	if expected != nil {
		got, err := v.MarshalBinary()
		if err != nil {
			return ann.Value{}, err
		}
		want, err := expected.MarshalBinary()
		if err != nil {
			return ann.Value{}, err
		}
		if !bytes.Equal(got, want) {
			return ann.Value{}, extensionCorrupt("ANN source/node representation or value differs")
		}
	}
	return v, nil
}

func annPairWork(a, b ann.Value) uint64 {
	if a.Kind() == ann.SparseKind {
		return uint64(a.WorkUnits()) + uint64(b.WorkUnits())
	}
	return uint64(a.WorkUnits())
}

// typedSearchANN shares the genuine persistent graph/list candidate path with
// dense ANN. The caller retains its exact typed scan for an explained fallback.
func (d *DB) typedSearchANN(ctx context.Context, reader btree.PageReader, tab *table, ordinal int, q ann.Value, metric vector.MetricKind, options VectorSearchOptions, limits vector.SearchLimits, filter func(int64, []any) (bool, error), plan *QueryPlan, stats *QueryStats, annWork *ANNWorkStats, scoredWork, scoredRows, filteredRows *int64) ([]vector.Hit, bool, error) {
	x, why, err := d.selectANNIndex(ctx, reader, tab, ordinal, metric, q.Dims(), options.Index)
	if err != nil {
		return nil, false, err
	}
	if x == nil {
		exactANNFallback(plan, why)
		return nil, false, nil
	}
	if err = ann.ValidateValue(q, *x.ann); err != nil {
		return nil, false, err
	}
	options, err = annIndexQueryOptions(x, options)
	if err != nil {
		return nil, false, err
	}
	applyANNPlan(plan, x, options)
	queryInputWork := uint64(*scoredWork)
	helper := &writeTable{table: tab}
	run := annCandidateQuery{db: d, reader: reader, table: tab, index: x, query: q, retain: options.Limit, options: options, limits: limits, plan: plan, stats: stats, annWork: annWork, components: func() int64 { return *scoredWork }, retainedBytes: func() int64 { return 0 }}
	hits, why, err := run.run(ctx, func(id int64, raw []any, expected *ann.Value) (float64, bool, error) {
		stats.BaseRowFetches++
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
				*filteredRows++
				return 0, false, nil
			}
		}
		scratch, scratchErr := annSourceScratch(q, values[ordinal], *x.ann)
		if scratchErr != nil {
			return 0, false, scratchErr
		}
		if scratch > d.opts.MaxSortBytes-run.activeBytes {
			return 0, false, ErrMemoryLimit
		}
		stats.ANNPeakBytes = max(stats.ANNPeakBytes, run.activeBytes+scratch)
		needed, err := annRescoreWork(q, values[ordinal], queryInputWork)
		if err != nil {
			return 0, false, err
		}
		used := uint64(*scoredWork) + uint64(annWork.ComponentWork)
		if used > limits.MaxComponents || needed > limits.MaxComponents-used {
			return 0, false, fmt.Errorf("%w: cumulative typed ANN work", vector.ErrLimit)
		}
		v, err := decodeTypedANNCandidate(values[ordinal], *x.ann, expected)
		if err != nil {
			return 0, false, err
		}
		if q.MemoryBytes()+v.MemoryBytes()*3 > d.opts.MaxSortBytes {
			return 0, false, ErrMemoryLimit
		}
		distance, err := ann.DistanceContext(ctx, *x.ann, q, v)
		if err != nil {
			return 0, false, err
		}
		*scoredRows++
		*scoredWork += int64(needed)
		stats.ANNExactRescores++
		return distance, true, nil
	})
	if err != nil {
		return nil, false, err
	}
	if why != "" {
		exactANNFallback(plan, why)
		return nil, false, nil
	}
	stats.ReturnedRows = int64(len(hits))
	return hits, true, nil
}

func (v *vectorRank) rankWork(s *QueryStats) int64 {
	if v.sparse {
		return s.VectorRankNonzeros
	}
	if v.bitMetric != "" {
		return s.VectorRankPackedBytes
	}
	return s.VectorRankComponents
}
func (v *vectorRank) scoreTypedANN(r *Rows, value any) (any, error) {
	a := v.ann
	scratch, scratchErr := annSourceScratch(a.query, value, *a.index.ann)
	if scratchErr != nil {
		return nil, scratchErr
	}
	if scratch > r.db.opts.MaxSortBytes-a.activeBytes {
		return nil, ErrMemoryLimit
	}
	r.queryStats.ANNPeakBytes = max(r.queryStats.ANNPeakBytes, a.activeBytes+scratch)
	needed, err := annRescoreWork(a.query, value, uint64(v.sparseQueryEntries))
	if err != nil {
		return nil, err
	}
	used := uint64(v.rankWork(&r.queryStats)) + uint64(r.annWork.ComponentWork)
	if used > v.limits.MaxComponents || needed > v.limits.MaxComponents-used {
		return nil, vector.ErrLimit
	}
	candidate, err := decodeTypedANNCandidate(value, *a.index.ann, v.expected)
	if err != nil {
		return nil, err
	}
	if candidate.MemoryBytes()*3+a.query.MemoryBytes()+a.retainedBytes() > r.db.opts.MaxSortBytes {
		return nil, ErrMemoryLimit
	}
	distance, err := ann.DistanceContext(r.ctx, *a.index.ann, a.query, candidate)
	if err != nil {
		return nil, err
	}
	r.queryStats.ANNExactRescores++
	r.queryStats.VectorRankedRows++
	if v.sparse {
		r.queryStats.VectorRankNonzeros += int64(needed)
	} else if v.bitMetric != "" {
		r.queryStats.VectorRankPackedBytes += int64(needed)
	} else {
		r.queryStats.VectorRankComponents += int64(needed)
	}
	return distance, nil
}

// Reserve typed source decoding and identity-comparison scratch together with
// graph/list result bookkeeping. Sparse metadata dimensions never enter this cost.
func annSourceScratch(query ann.Value, raw any, cfg ann.TypedConfig) (int64, error) {
	switch cfg.Kind {
	case ann.HalfKind:
		return 128 + 16*int64(cfg.Dimensions), nil
	case ann.BitKind:
		return 128 + 4*int64((cfg.Dimensions+7)/8), nil
	case ann.SparseKind:
		n, err := sparseValueEntries(raw)
		if err != nil {
			return 0, err
		}
		return sparseScratchBytes(query.Sparse().NNZ(), n), nil
	default:
		return 0, nil
	}
}

// ANNWorkStats supplements unchanged legacy QueryStats with actual typed metric
// support. Unit is "components", "packed_bytes", or "nonzeros". It excludes
// source rescoring, reported in the existing representation-specific counters.
type ANNWorkStats struct {
	ComponentWork int64
	Unit          string
}

// ANNWork reports actual typed graph/centroid support without changing the
// legacy QueryStats struct layout. Like Stats, it includes abandoned attempts.
func (r *Rows) ANNWork() ANNWorkStats { r.mu.Lock(); defer r.mu.Unlock(); return r.annWork }
func (a *annCandidateQuery) work() *ANNWorkStats {
	if a.annWork == nil {
		a.annWork = &ANNWorkStats{}
	}
	if a.annWork.Unit == "" {
		a.annWork.Unit = "components"
		switch a.query.Kind() {
		case ann.BitKind:
			a.annWork.Unit = "packed_bytes"
		case ann.SparseKind:
			a.annWork.Unit = "nonzeros"
		}
	}
	return a.annWork
}

// Preflight keeps bounded parsing independent from canonical sparse support.
// Scalar laziness is handled by callers falling back to the exact row path.
func annQueryPreflight(value any, cfg ann.TypedConfig) (uint64, int64, error) {
	switch cfg.Kind {
	case ann.HalfKind:
		return uint64(cfg.Dimensions), 128 + 16*int64(cfg.Dimensions), nil
	case ann.BitKind:
		return uint64((cfg.Dimensions + 7) / 8), 128 + 4*int64((cfg.Dimensions+7)/8), nil
	case ann.SparseKind:
		n, err := sparseValueEntries(value)
		if err != nil {
			return 0, 0, err
		}
		return uint64(n), sparseScratchBytes(n, 0), nil
	default:
		return uint64(cfg.Dimensions), 4 * int64(cfg.Dimensions), nil
	}
}
func annRescoreWork(q ann.Value, raw any, queryInputWork uint64) (uint64, error) {
	if q.Kind() == ann.SparseKind {
		n, err := sparseValueEntries(raw)
		if err != nil {
			return 0, err
		}
		return queryInputWork + uint64(n), nil
	}
	return q.WorkUnits(), nil
}
