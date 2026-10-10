package sqlite

import (
	"container/heap"
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// SparseSearchLimits bounds an exact scan. Every source row, including filtered or
// NULL rows, consumes one candidate. Nonzeros charge the query once and
// both supports per scored merge. Text conservatively counts explicit zeros.
// These are logical work budgets, not total allocations, I/O or elapsed time.
// A zero structure selects defaults; partially specified limits are invalid.
type SparseSearchLimits struct {
	MaxCandidates int
	MaxResults    int
	MaxNonzeros   uint64
}

func DefaultSparseSearchLimits() SparseSearchLimits {
	return SparseSearchLimits{MaxCandidates: 100000, MaxResults: 10000, MaxNonzeros: 1 << 27}
}

func sparseLimits(l SparseSearchLimits) (SparseSearchLimits, error) {
	if l == (SparseSearchLimits{}) {
		l = DefaultSparseSearchLimits()
	}
	if l.MaxCandidates < 1 || l.MaxCandidates > 1<<20 || l.MaxResults < 1 || l.MaxResults > 1<<20 || l.MaxNonzeros < 1 || l.MaxNonzeros > 1<<34 {
		return SparseSearchLimits{}, vector.ErrOptions
	}
	return l, nil
}

// SparseSearchOptions selects exact Top-K over one physical SPARSEVEC(n) column. Empty
// Metric selects L2. Only L2/L1/negative-inner-product/cosine identities are accepted. There
// is no sparse ANN mode in this checkpoint. Where is one bound SQL expression,
// evaluated before sparse decoding/scoring. Limit must be nonnegative; zero
// validates query/options but visits no rows. NULL query yields EMPTY.
type SparseSearchOptions struct {
	Metric vector.MetricKind
	Limit  int
	Where  string
	Args   []any
	Limits SparseSearchLimits
}

// SparsePlan describes native exact sparse search. Embedded SQL ranking fields stay
// unset; Metric, Dimensions and Exact are the native operation identities.
type SparsePlan struct {
	QueryPlan
	Field      string
	Metric     vector.MetricKind
	Dimensions int
	Limit      int
	Exact      bool
}

func (p SparsePlan) Detail() string {
	return fmt.Sprintf("%s %s; representation=sparsevec; sparse_field=%s; sparse_metric=%s; dimensions=%d; limit=%d; exact=%t; distance_order=ASC,rowid_ASC; null_candidates=excluded; reason=%s", p.Access, p.Table, p.Field, p.Metric, p.Dimensions, p.Limit, p.Exact, p.Reason)
}

// SparseStats separates fetched, excluded and scored source rows. Support work
// follows SparseSearchLimits; it does not measure arbitrary SQL expression work.
type SparseStats struct {
	QueryStats
	FilteredRows   int64
	ScoredRows     int64
	NullRows       int64
	ScoredNonzeros int64
}

// SparseResult contains a complete exact ranking and value-only diagnostics.
type SparseResult struct {
	Hits  []vector.Hit
	Plan  SparsePlan
	Stats SparseStats
}

// SparseSearch owns one ordinary database snapshot and returns only complete
// exact results. A failure/cancellation returns the zero result, never hits.
func (d *DB) SparseSearch(ctx context.Context, table, field string, query any, options SparseSearchOptions) (SparseResult, error) {
	if err := checkContext(ctx); err != nil {
		return SparseResult{}, wrap("sparse search", err)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return SparseResult{}, wrap("sparse search", ErrClosed)
	}
	if d.tx != nil {
		if err := d.tx.check(); err != nil {
			return SparseResult{}, wrap("sparse search", err)
		}
		if err := d.tx.ensureSnapshotLocked(ctx); err != nil {
			return SparseResult{}, wrap("sparse search", err)
		}
		result, err := d.sparseSearch(ctx, d.tx.reader, table, field, query, options)
		return result, wrap("sparse search", err)
	}
	reader, err := d.openReader(ctx)
	if err != nil {
		return SparseResult{}, wrap("sparse search", err)
	}
	d.info = fileInfo(reader.Header())
	result, err := d.sparseSearch(ctx, reader, table, field, query, options)
	if err = errors.Join(err, reader.Close()); err != nil {
		return SparseResult{}, wrap("sparse search", err)
	}
	return result, nil
}

func (t *Tx) SparseSearch(ctx context.Context, table, field string, query any, options SparseSearchOptions) (SparseResult, error) {
	if err := checkContext(ctx); err != nil {
		return SparseResult{}, wrap("sparse search", err)
	}
	t.db.mu.Lock()
	defer t.db.mu.Unlock()
	if err := t.check(); err != nil {
		return SparseResult{}, wrap("sparse search", err)
	}
	if err := t.ensureSnapshotLocked(ctx); err != nil {
		return SparseResult{}, wrap("sparse search", err)
	}
	result, err := t.db.sparseSearch(ctx, t.reader, table, field, query, options)
	return result, wrap("sparse search", err)
}

func sparseQueryValue(ctx context.Context, query any) (any, error) {
	value, err := driver.DefaultParameterConverter.ConvertValue(query)
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	switch v := value.(type) {
	case string:
		if len(v) > vector.MaxTextBytes {
			return nil, fmt.Errorf("%w: sparse query text bytes", vector.ErrLimit)
		}
	case []byte:
		if len(v) > vector.MaxSparseBinaryBytes {
			return nil, fmt.Errorf("%w: sparse query binary bytes", vector.ErrLimit)
		}
		if v == nil {
			return nil, nil
		}
	case nil:
	default:
		return nil, fmt.Errorf("%w: sparse query requires text or GSS BLOB", ErrMismatch)
	}
	return value, nil
}

func (d *DB) sparseSearch(ctx context.Context, reader btree.PageReader, tableName, fieldName string, query any, options SparseSearchOptions) (SparseResult, error) {
	limits, err := sparseLimits(options.Limits)
	if err != nil {
		return SparseResult{}, err
	}
	metric := options.Metric
	if metric == "" {
		metric = vector.L2
	}
	if metric != vector.L2 && metric != vector.L1 && metric != vector.NegativeIP && metric != vector.Cosine {
		return SparseResult{}, vector.ErrMetric
	}
	if options.Limit < 0 {
		return SparseResult{}, vector.ErrOptions
	}
	if options.Limit > limits.MaxResults {
		return SparseResult{}, fmt.Errorf("%w: sparse results", vector.ErrLimit)
	}
	tab, err := d.table(ctx, reader, &sqlast.Select{From: tableName})
	if err != nil {
		return SparseResult{}, err
	}
	if tab == nil {
		return SparseResult{}, fmt.Errorf("sparse search requires a source table")
	}
	ordinal, err := tab.column("", fieldName)
	if err != nil {
		return SparseResult{}, err
	}
	if ordinal < 0 {
		return SparseResult{}, fmt.Errorf("%w: sparse search requires a SPARSEVEC(n) column", ErrMismatch)
	}
	dims, ok := sqlast.SparseDimensions(tab.columns[ordinal].Type)
	if !ok {
		return SparseResult{}, fmt.Errorf("%w: sparse search requires a SPARSEVEC(n) column", ErrMismatch)
	}
	filter, err := d.similarityFilter(ctx, reader, tab, SimilarityOptions{Where: options.Where, Args: options.Args})
	if err != nil {
		return SparseResult{}, err
	}
	// Preflight known typed Valuers before their binary encoding copy. Arbitrary
	// user callback allocations remain outside these logical search budgets.
	var known vector.SparseVector
	switch v := query.(type) {
	case vector.SparseVector:
		known = v
	case *vector.SparseVector:
		if v != nil {
			known = *v
		}
	case vector.NullSparseVector:
		if v.Valid {
			known = v.SparseVector
		}
	case *vector.NullSparseVector:
		if v != nil && v.Valid {
			known = v.SparseVector
		}
	}
	if uint64(known.NNZ()) > limits.MaxNonzeros {
		return SparseResult{}, vector.ErrLimit
	}
	if known.Valid() && sparseScratchBytes(known.NNZ(), 0) >= d.opts.MaxSortBytes {
		return SparseResult{}, ErrMemoryLimit
	}

	value, err := sparseQueryValue(ctx, query)
	if err != nil {
		return SparseResult{}, err
	}
	result := SparseResult{Plan: SparsePlan{QueryPlan: QueryPlan{Access: "SCAN", Table: tab.name, Method: "exact_sparsevec", MethodVersion: 1, Fields: []string{tab.columns[ordinal].Name}, EstimatedCandidates: -1, Reason: "exact full scan; WHERE before sparse binary32 distance"}, Field: tab.columns[ordinal].Name, Metric: metric, Dimensions: dims, Limit: options.Limit, Exact: true}}
	if value == nil {
		if err = ctx.Err(); err != nil {
			return SparseResult{}, err
		}
		result.Plan.Access, result.Plan.Reason = "EMPTY", "NULL query produces no sparse matches"
		return result, nil
	}
	queryEntries, err := sparseValueEntries(value)
	if err != nil {
		return SparseResult{}, err
	}
	if uint64(queryEntries) > limits.MaxNonzeros {
		return SparseResult{}, fmt.Errorf("%w: sparse query nonzeros", vector.ErrLimit)
	}
	queryScratch := sparseScratchBytes(queryEntries, 0)
	if queryScratch >= d.opts.MaxSortBytes {
		return SparseResult{}, fmt.Errorf("%w: sparse query memory", ErrMemoryLimit)
	}
	q, _, err := decodeSparseColumn(value, dims)
	if err != nil {
		return SparseResult{}, err
	}

	if err = ctx.Err(); err != nil {
		return SparseResult{}, err
	}
	if metric == vector.Cosine {
		if _, err = metric.DistanceSparseContext(ctx, q, q); err != nil {
			return SparseResult{}, err
		}
	}
	result.Stats.ScoredNonzeros = int64(queryEntries)
	if options.Limit == 0 {
		result.Plan.Access, result.Plan.Reason = "EMPTY", "zero limit; query validated without source reads"
		return result, nil
	}
	// Reserve query support and the result heap before retaining
	// row identities. Duplicate detection is bounded, never silently truncated.
	reserve := int64(options.Limit)*64 + queryScratch
	if reserve >= d.opts.MaxSortBytes {
		return SparseResult{}, fmt.Errorf("%w: sparse result memory", ErrMemoryLimit)
	}
	memoryCandidates := (d.opts.MaxSortBytes - reserve) / 64
	if memoryCandidates < 1 {
		return SparseResult{}, fmt.Errorf("%w: sparse identity memory", ErrMemoryLimit)
	}
	src, err := btree.NewCursor(ctx, reader, tab.root)
	if err != nil {
		return SparseResult{}, err
	}
	defer src.Close()
	valuesHelper := &writeTable{table: tab}
	seen := make(map[int64]struct{})
	retained := bitHitHeap{}
	for {
		if err = ctx.Err(); err != nil {
			return SparseResult{}, err
		}
		id, raw, e := src.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return SparseResult{}, e
		}
		result.Stats.BaseRowFetches++
		if result.Stats.BaseRowFetches > int64(limits.MaxCandidates) {
			return SparseResult{}, fmt.Errorf("%w: sparse source candidates", vector.ErrLimit)
		}
		if result.Stats.BaseRowFetches > memoryCandidates {
			return SparseResult{}, fmt.Errorf("%w: sparse identity memory", ErrMemoryLimit)
		}
		if _, duplicate := seen[id]; duplicate {
			return SparseResult{}, fmt.Errorf("%w: duplicate sparse source rowid", ErrCorrupt)
		}
		seen[id] = struct{}{}
		values, e := valuesHelper.values(id, raw)
		if e != nil {
			return SparseResult{}, e
		}
		if filter != nil {
			keep, e := filter(id, values)
			if e != nil {
				return SparseResult{}, e
			}
			if e = ctx.Err(); e != nil {
				return SparseResult{}, e
			}
			if !keep {
				result.Stats.FilteredRows++
				continue
			}
		}
		if values[ordinal] == nil {
			result.Stats.NullRows++
			continue
		}
		candidateEntries, e := sparseValueEntries(values[ordinal])
		if e != nil {
			return SparseResult{}, e
		}
		needed := uint64(queryEntries) + uint64(candidateEntries)
		used := uint64(result.Stats.ScoredNonzeros)
		if used > limits.MaxNonzeros || needed > limits.MaxNonzeros-used {
			return SparseResult{}, fmt.Errorf("%w: sparse nonzeros", vector.ErrLimit)
		}
		scratch := sparseScratchBytes(queryEntries, candidateEntries)
		// Combine transient support storage, retained heap and identities. Check
		// subtraction before adding so user-controlled options cannot overflow.
		retainedBytes := int64(options.Limit)*64 + result.Stats.BaseRowFetches*64
		if scratch > d.opts.MaxSortBytes-retainedBytes {
			return SparseResult{}, ErrMemoryLimit
		}
		candidate, _, e := decodeSparseColumn(values[ordinal], dims)
		if e != nil {
			return SparseResult{}, e
		}

		distance, e := metric.DistanceSparseContext(ctx, q, candidate)
		if e != nil {
			return SparseResult{}, e
		}
		result.Stats.ScoredRows++
		result.Stats.ScoredNonzeros += int64(needed)
		hit := vector.Hit{RowID: id, Distance: distance}
		if len(retained) < options.Limit {
			heap.Push(&retained, hit)
		} else if bitHitBetter(hit, retained[0]) {
			retained[0] = hit
			heap.Fix(&retained, 0)
		}
	}
	if err = src.Close(); err != nil {
		return SparseResult{}, err
	}
	result.Hits = make([]vector.Hit, len(retained))
	for i := len(result.Hits) - 1; i >= 0; i-- {
		if i&255 == 0 {
			if err = ctx.Err(); err != nil {
				return SparseResult{}, err
			}
		}
		result.Hits[i] = heap.Pop(&retained).(vector.Hit)
	}
	if err = ctx.Err(); err != nil {
		return SparseResult{}, err
	}
	result.Stats.ReturnedRows = int64(len(result.Hits))
	return result, nil
}
