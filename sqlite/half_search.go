package sqlite

import (
	"container/heap"
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/extension/vector/ann"
	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// HalfSearchLimits bounds an exact scan. Every source row, including filtered or
// NULL rows, consumes one candidate. Components charge the query once and
// each scored candidate once (one component pair per metric iteration).
// These are logical work budgets, not total allocations, I/O or elapsed time.
// A zero structure selects defaults; partially specified limits are invalid.
type HalfSearchLimits = vector.SearchLimits

func DefaultHalfSearchLimits() HalfSearchLimits {
	return HalfSearchLimits{MaxCandidates: 100000, MaxResults: 10000, MaxComponents: 1 << 27}
}

func halfLimits(l HalfSearchLimits) (HalfSearchLimits, error) {
	if l == (HalfSearchLimits{}) {
		l = DefaultHalfSearchLimits()
	}
	if l.MaxCandidates < 1 || l.MaxCandidates > 1<<20 || l.MaxResults < 1 || l.MaxResults > 1<<20 || l.MaxComponents < 1 || l.MaxComponents > 1<<34 {
		return HalfSearchLimits{}, vector.ErrOptions
	}
	return l, nil
}

// HalfSearchOptions selects exact Top-K over one physical HALFVEC(n) column. Empty
// Metric selects L2. Only L2/L1/negative-inner-product/cosine identities are accepted. Mode defaults to exact; approximate mode opts into a matching typed ANN index.
// Where is one bound SQL expression,
// evaluated before half decoding/scoring. Limit must be nonnegative; zero
// validates query/options but visits no rows. NULL query yields EMPTY.
type HalfSearchOptions struct {
	Mode                  VectorSearchMode
	Index                 string
	EFSearch, MaxEFSearch int
	Probes, MaxProbes     int
	ANNLimits             ANNWorkLimits
	Metric                vector.MetricKind
	Limit                 int
	Where                 string
	Args                  []any
	Limits                HalfSearchLimits
}

// HalfPlan describes native exact half search. Embedded SQL ranking fields stay
// unset; Metric, Dimensions and Exact are the native operation identities.
type HalfPlan struct {
	QueryPlan
	Field      string
	Metric     vector.MetricKind
	Dimensions int
	Limit      int
	Exact      bool
}

func (p HalfPlan) Detail() string {
	return fmt.Sprintf("%s %s; representation=halfvec; half_field=%s; half_metric=%s; dimensions=%d; limit=%d; exact=%t; distance_order=ASC,rowid_ASC; null_candidates=excluded; reason=%s", p.Access, p.Table, p.Field, p.Metric, p.Dimensions, p.Limit, p.Exact, p.Reason) + p.annDetail()
}

// HalfStats separates fetched, excluded and scored source rows. Component work
// follows HalfSearchLimits; it does not measure arbitrary SQL expression work.
type HalfStats struct {
	ANNWork ANNWorkStats
	QueryStats
	FilteredRows     int64
	ScoredRows       int64
	NullRows         int64
	ScoredComponents int64
}

// HalfResult contains a complete requested ranking and explicit recall diagnostics.
type HalfResult struct {
	Hits  []vector.Hit
	Plan  HalfPlan
	Stats HalfStats
}

// HalfSearch owns one ordinary database snapshot and returns only complete
// requested results. A failure/cancellation returns the zero result, never hits.
func (d *DB) HalfSearch(ctx context.Context, table, field string, query any, options HalfSearchOptions) (HalfResult, error) {
	if err := checkContext(ctx); err != nil {
		return HalfResult{}, wrap("half search", err)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return HalfResult{}, wrap("half search", ErrClosed)
	}
	if d.tx != nil {
		if err := d.tx.check(); err != nil {
			return HalfResult{}, wrap("half search", err)
		}
		if err := d.tx.ensureSnapshotLocked(ctx); err != nil {
			return HalfResult{}, wrap("half search", err)
		}
		result, err := d.halfSearch(ctx, d.tx.reader, table, field, query, options)
		return result, wrap("half search", err)
	}
	reader, err := d.openReader(ctx)
	if err != nil {
		return HalfResult{}, wrap("half search", err)
	}
	d.info = fileInfo(reader.Header())
	result, err := d.halfSearch(ctx, reader, table, field, query, options)
	if err = errors.Join(err, reader.Close()); err != nil {
		return HalfResult{}, wrap("half search", err)
	}
	return result, nil
}

func (t *Tx) HalfSearch(ctx context.Context, table, field string, query any, options HalfSearchOptions) (HalfResult, error) {
	if err := checkContext(ctx); err != nil {
		return HalfResult{}, wrap("half search", err)
	}
	t.db.mu.Lock()
	defer t.db.mu.Unlock()
	if err := t.check(); err != nil {
		return HalfResult{}, wrap("half search", err)
	}
	if err := t.ensureSnapshotLocked(ctx); err != nil {
		return HalfResult{}, wrap("half search", err)
	}
	result, err := t.db.halfSearch(ctx, t.reader, table, field, query, options)
	return result, wrap("half search", err)
}

func halfQueryValue(ctx context.Context, query any) (any, error) {
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
			return nil, fmt.Errorf("%w: half query text bytes", vector.ErrLimit)
		}
	case []byte:
		if len(v) > vector.MaxHalfBinaryBytes {
			return nil, fmt.Errorf("%w: half query binary bytes", vector.ErrLimit)
		}
		if v == nil {
			return nil, nil
		}
	case nil:
	default:
		return nil, fmt.Errorf("%w: half query requires text or GSH BLOB", ErrMismatch)
	}
	return value, nil
}

func (d *DB) halfSearch(ctx context.Context, reader btree.PageReader, tableName, fieldName string, query any, options HalfSearchOptions) (HalfResult, error) {
	annOptions, err := normalizeVectorANNOptions(VectorSearchOptions{Mode: options.Mode, Index: options.Index, Limit: options.Limit, EFSearch: options.EFSearch, MaxEFSearch: options.MaxEFSearch, Probes: options.Probes, MaxProbes: options.MaxProbes, ANNLimits: options.ANNLimits})
	if err != nil {
		return HalfResult{}, err
	}
	limits, err := halfLimits(options.Limits)
	if err != nil {
		return HalfResult{}, err
	}
	metric := options.Metric
	if metric == "" {
		metric = vector.L2
	}
	if metric != vector.L2 && metric != vector.L1 && metric != vector.NegativeIP && metric != vector.Cosine {
		return HalfResult{}, vector.ErrMetric
	}
	if options.Limit < 0 {
		return HalfResult{}, vector.ErrOptions
	}
	if options.Limit > limits.MaxResults {
		return HalfResult{}, fmt.Errorf("%w: half results", vector.ErrLimit)
	}
	tab, err := d.table(ctx, reader, &sqlast.Select{From: tableName})
	if err != nil {
		return HalfResult{}, err
	}
	if tab == nil {
		return HalfResult{}, fmt.Errorf("half search requires a source table")
	}
	ordinal, err := tab.column("", fieldName)
	if err != nil {
		return HalfResult{}, err
	}
	if ordinal < 0 {
		return HalfResult{}, fmt.Errorf("%w: half search requires a HALFVEC(n) column", ErrMismatch)
	}
	dims, ok := sqlast.HalfDimensions(tab.columns[ordinal].Type)
	if !ok {
		return HalfResult{}, fmt.Errorf("%w: half search requires a HALFVEC(n) column", ErrMismatch)
	}
	filter, err := d.similarityFilter(ctx, reader, tab, SimilarityOptions{Where: options.Where, Args: options.Args})
	if err != nil {
		return HalfResult{}, err
	}
	// Preflight known typed Valuers before their binary encoding copy. Arbitrary
	// user callback allocations remain outside these logical search budgets.
	var known vector.HalfVector
	switch v := query.(type) {
	case vector.HalfVector:
		known = v
	case *vector.HalfVector:
		if v != nil {
			known = *v
		}
	case vector.NullHalfVector:
		if v.Valid {
			known = v.HalfVector
		}
	case *vector.NullHalfVector:
		if v != nil && v.Valid {
			known = v.HalfVector
		}
	}
	if uint64(known.Dims()) > limits.MaxComponents {
		return HalfResult{}, vector.ErrLimit
	}
	if int64(known.Dims())*16 >= d.opts.MaxSortBytes {
		return HalfResult{}, ErrMemoryLimit
	}

	value, err := halfQueryValue(ctx, query)
	if err != nil {
		return HalfResult{}, err
	}
	result := HalfResult{Plan: HalfPlan{QueryPlan: QueryPlan{Access: "SCAN", Table: tab.name, Method: "exact_halfvec", MethodVersion: 1, Fields: []string{tab.columns[ordinal].Name}, EstimatedCandidates: -1, Reason: "exact full scan; WHERE before binary16 distance"}, Field: tab.columns[ordinal].Name, Metric: metric, Dimensions: dims, Limit: options.Limit, Exact: true}}
	if value == nil {
		if err = ctx.Err(); err != nil {
			return HalfResult{}, err
		}
		result.Plan.Access, result.Plan.Reason = "EMPTY", "NULL query produces no half matches"
		return result, nil
	}
	components := uint64(dims)
	if components > limits.MaxComponents {
		return HalfResult{}, fmt.Errorf("%w: half components", vector.ErrLimit)
	}
	if int64(dims)*16 >= d.opts.MaxSortBytes {
		return HalfResult{}, fmt.Errorf("%w: half query memory", ErrMemoryLimit)
	}
	q, _, err := decodeHalfColumn(value, dims)
	if err != nil {
		return HalfResult{}, err
	}
	if err = ctx.Err(); err != nil {
		return HalfResult{}, err
	}
	if metric == vector.Cosine {
		if _, err = metric.DistanceHalfContext(ctx, q, q); err != nil {
			return HalfResult{}, err
		}
	}
	result.Stats.ScoredComponents = int64(components)
	if options.Limit == 0 {
		result.Plan.Access, result.Plan.Reason = "EMPTY", "zero limit; query validated without source reads"
		return result, nil
	}
	if annOptions.Mode == VectorModeApproximate {
		result.Plan.VectorRepresentation = "halfvec"
		hits, done, err := d.typedSearchANN(ctx, reader, tab, ordinal, ann.HalfValue(q), vector.MetricKind(metric), annOptions, vector.SearchLimits{MaxCandidates: limits.MaxCandidates, MaxResults: limits.MaxResults, MaxComponents: limits.MaxComponents}, filter, &result.Plan.QueryPlan, &result.Stats.QueryStats, &result.Stats.ANNWork, &result.Stats.ScoredComponents, &result.Stats.ScoredRows, &result.Stats.FilteredRows)
		if err != nil {
			return HalfResult{}, err
		}
		if done {
			result.Hits = hits
			result.Plan.Exact = false
			return result, nil
		}
	}
	// Reserve both query/candidate binary16 storage and result heap before retaining
	// row identities. Duplicate detection is bounded, never silently truncated.
	reserve := int64(options.Limit)*64 + int64(components)*16
	if reserve >= d.opts.MaxSortBytes {
		return HalfResult{}, fmt.Errorf("%w: half result memory", ErrMemoryLimit)
	}
	memoryCandidates := (d.opts.MaxSortBytes - reserve) / 64
	if memoryCandidates < 1 {
		return HalfResult{}, fmt.Errorf("%w: half identity memory", ErrMemoryLimit)
	}
	src, err := btree.NewCursor(ctx, reader, tab.root)
	if err != nil {
		return HalfResult{}, err
	}
	defer src.Close()
	valuesHelper := &writeTable{table: tab}
	seen := make(map[int64]struct{})
	retained := bitHitHeap{}
	for {
		if err = ctx.Err(); err != nil {
			return HalfResult{}, err
		}
		id, raw, e := src.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return HalfResult{}, e
		}
		result.Stats.BaseRowFetches++
		if result.Stats.BaseRowFetches > int64(limits.MaxCandidates) {
			return HalfResult{}, fmt.Errorf("%w: half source candidates", vector.ErrLimit)
		}
		if result.Stats.BaseRowFetches > memoryCandidates {
			return HalfResult{}, fmt.Errorf("%w: half identity memory", ErrMemoryLimit)
		}
		if _, duplicate := seen[id]; duplicate {
			return HalfResult{}, fmt.Errorf("%w: duplicate half source rowid", ErrCorrupt)
		}
		seen[id] = struct{}{}
		values, e := valuesHelper.values(id, raw)
		if e != nil {
			return HalfResult{}, e
		}
		if filter != nil {
			keep, e := filter(id, values)
			if e != nil {
				return HalfResult{}, e
			}
			if e = ctx.Err(); e != nil {
				return HalfResult{}, e
			}
			if !keep {
				result.Stats.FilteredRows++
				continue
			}
		}
		used := uint64(result.Stats.ScoredComponents) + uint64(result.Stats.ANNWork.ComponentWork)
		if values[ordinal] != nil && (used > limits.MaxComponents || components > limits.MaxComponents-used) {
			return HalfResult{}, fmt.Errorf("%w: half components", vector.ErrLimit)
		}
		candidate, null, e := decodeHalfColumn(values[ordinal], dims)
		if e != nil {
			return HalfResult{}, e
		}
		if null {
			result.Stats.NullRows++
			continue
		}

		distance, e := metric.DistanceHalfContext(ctx, q, candidate)
		if e != nil {
			return HalfResult{}, e
		}
		result.Stats.ScoredRows++
		result.Stats.ScoredComponents += int64(components)
		hit := vector.Hit{RowID: id, Distance: distance}
		if len(retained) < options.Limit {
			heap.Push(&retained, hit)
		} else if bitHitBetter(hit, retained[0]) {
			retained[0] = hit
			heap.Fix(&retained, 0)
		}
	}
	if err = src.Close(); err != nil {
		return HalfResult{}, err
	}
	result.Hits = make([]vector.Hit, len(retained))
	for i := len(result.Hits) - 1; i >= 0; i-- {
		if i&255 == 0 {
			if err = ctx.Err(); err != nil {
				return HalfResult{}, err
			}
		}
		result.Hits[i] = heap.Pop(&retained).(vector.Hit)
	}
	if err = ctx.Err(); err != nil {
		return HalfResult{}, err
	}
	result.Stats.ReturnedRows = int64(len(result.Hits))
	return result, nil
}
