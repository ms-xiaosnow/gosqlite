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

// BitSearchLimits bounds an exact scan. Every source row, including filtered or
// NULL rows, consumes one candidate. Packed bytes charge the query once and
// each scored candidate once (one paired byte position per metric iteration).
// These are logical work budgets, not total allocations, I/O or elapsed time.
// A zero structure selects defaults; partially specified limits are invalid.
type BitSearchLimits struct {
	MaxCandidates  int
	MaxResults     int
	MaxPackedBytes uint64
}

func DefaultBitSearchLimits() BitSearchLimits {
	return BitSearchLimits{MaxCandidates: 100000, MaxResults: 10000, MaxPackedBytes: 1 << 27}
}

func bitLimits(l BitSearchLimits) (BitSearchLimits, error) {
	if l == (BitSearchLimits{}) {
		l = DefaultBitSearchLimits()
	}
	if l.MaxCandidates < 1 || l.MaxCandidates > 1<<20 || l.MaxResults < 1 || l.MaxResults > 1<<20 || l.MaxPackedBytes < 1 || l.MaxPackedBytes > 1<<34 {
		return BitSearchLimits{}, vector.ErrOptions
	}
	return l, nil
}

// BitSearchOptions selects exact Top-K over one physical BIT(n) column. Empty
// Metric selects Hamming. Only Hamming/Jaccard identities are accepted. Mode defaults to exact; approximate mode opts into a matching typed ANN index.
// Where is one bound SQL expression,
// evaluated before bit decoding/scoring. Limit must be nonnegative; zero
// validates query/options but visits no rows. NULL query yields EMPTY.
type BitSearchOptions struct {
	Mode                  VectorSearchMode
	Index                 string
	EFSearch, MaxEFSearch int
	Probes, MaxProbes     int
	ANNLimits             ANNWorkLimits
	Metric                vector.BitMetricKind
	Limit                 int
	Where                 string
	Args                  []any
	Limits                BitSearchLimits
}

// BitPlan describes native exact bit search. Embedded SQL ranking fields stay
// unset; Metric, Dimensions and Exact are the native operation identities.
type BitPlan struct {
	QueryPlan
	Field      string
	Metric     vector.BitMetricKind
	Dimensions int
	Limit      int
	Exact      bool
}

func (p BitPlan) Detail() string {
	return fmt.Sprintf("%s %s; representation=bit; bit_field=%s; bit_metric=%s; dimensions=%d; limit=%d; exact=%t; distance_order=ASC,rowid_ASC; null_candidates=excluded; reason=%s", p.Access, p.Table, p.Field, p.Metric, p.Dimensions, p.Limit, p.Exact, p.Reason) + p.annDetail()
}

// BitStats separates fetched, excluded and scored source rows. Packed work
// follows BitSearchLimits; it does not measure arbitrary SQL expression work.
type BitStats struct {
	ANNWork ANNWorkStats
	QueryStats
	FilteredRows      int64
	ScoredRows        int64
	NullRows          int64
	ScoredPackedBytes int64
}

// BitResult contains a complete requested ranking and explicit recall diagnostics.
type BitResult struct {
	Hits  []vector.Hit
	Plan  BitPlan
	Stats BitStats
}

// BitSearch owns one ordinary database snapshot and returns only complete
// requested results. A failure/cancellation returns the zero result, never hits.
func (d *DB) BitSearch(ctx context.Context, table, field string, query any, options BitSearchOptions) (BitResult, error) {
	if err := checkContext(ctx); err != nil {
		return BitResult{}, wrap("bit search", err)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return BitResult{}, wrap("bit search", ErrClosed)
	}
	if d.tx != nil {
		if err := d.tx.check(); err != nil {
			return BitResult{}, wrap("bit search", err)
		}
		if err := d.tx.ensureSnapshotLocked(ctx); err != nil {
			return BitResult{}, wrap("bit search", err)
		}
		result, err := d.bitSearch(ctx, d.tx.reader, table, field, query, options)
		return result, wrap("bit search", err)
	}
	reader, err := d.openReader(ctx)
	if err != nil {
		return BitResult{}, wrap("bit search", err)
	}
	d.info = fileInfo(reader.Header())
	result, err := d.bitSearch(ctx, reader, table, field, query, options)
	if err = errors.Join(err, reader.Close()); err != nil {
		return BitResult{}, wrap("bit search", err)
	}
	return result, nil
}

func (t *Tx) BitSearch(ctx context.Context, table, field string, query any, options BitSearchOptions) (BitResult, error) {
	if err := checkContext(ctx); err != nil {
		return BitResult{}, wrap("bit search", err)
	}
	t.db.mu.Lock()
	defer t.db.mu.Unlock()
	if err := t.check(); err != nil {
		return BitResult{}, wrap("bit search", err)
	}
	if err := t.ensureSnapshotLocked(ctx); err != nil {
		return BitResult{}, wrap("bit search", err)
	}
	result, err := t.db.bitSearch(ctx, t.reader, table, field, query, options)
	return result, wrap("bit search", err)
}

func bitQueryValue(ctx context.Context, query any) (any, error) {
	value, err := driver.DefaultParameterConverter.ConvertValue(query)
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	switch v := value.(type) {
	case string:
		if len(v) > vector.MaxBitDimensions {
			return nil, fmt.Errorf("%w: bit query text bytes", vector.ErrLimit)
		}
	case []byte:
		if len(v) > vector.MaxBitBinaryBytes {
			return nil, fmt.Errorf("%w: bit query binary bytes", vector.ErrLimit)
		}
		if v == nil {
			return nil, nil
		}
	case nil:
	default:
		return nil, fmt.Errorf("%w: bit query requires text or GSB BLOB", ErrMismatch)
	}
	return value, nil
}

func (d *DB) bitSearch(ctx context.Context, reader btree.PageReader, tableName, fieldName string, query any, options BitSearchOptions) (BitResult, error) {
	annOptions, err := normalizeVectorANNOptions(VectorSearchOptions{Mode: options.Mode, Index: options.Index, Limit: options.Limit, EFSearch: options.EFSearch, MaxEFSearch: options.MaxEFSearch, Probes: options.Probes, MaxProbes: options.MaxProbes, ANNLimits: options.ANNLimits})
	if err != nil {
		return BitResult{}, err
	}
	limits, err := bitLimits(options.Limits)
	if err != nil {
		return BitResult{}, err
	}
	metric := options.Metric
	if metric == "" {
		metric = vector.Hamming
	}
	if metric != vector.Hamming && metric != vector.Jaccard {
		return BitResult{}, vector.ErrMetric
	}
	if options.Limit < 0 {
		return BitResult{}, vector.ErrOptions
	}
	if options.Limit > limits.MaxResults {
		return BitResult{}, fmt.Errorf("%w: bit results", vector.ErrLimit)
	}
	tab, err := d.table(ctx, reader, &sqlast.Select{From: tableName})
	if err != nil {
		return BitResult{}, err
	}
	if tab == nil {
		return BitResult{}, fmt.Errorf("bit search requires a source table")
	}
	ordinal, err := tab.column("", fieldName)
	if err != nil {
		return BitResult{}, err
	}
	if ordinal < 0 {
		return BitResult{}, fmt.Errorf("%w: bit search requires a BIT(n) column", ErrMismatch)
	}
	dims, ok := sqlast.BitDimensions(tab.columns[ordinal].Type)
	if !ok {
		return BitResult{}, fmt.Errorf("%w: bit search requires a BIT(n) column", ErrMismatch)
	}
	filter, err := d.similarityFilter(ctx, reader, tab, SimilarityOptions{Where: options.Where, Args: options.Args})
	if err != nil {
		return BitResult{}, err
	}
	value, err := bitQueryValue(ctx, query)
	if err != nil {
		return BitResult{}, err
	}
	result := BitResult{Plan: BitPlan{QueryPlan: QueryPlan{Access: "SCAN", Table: tab.name, Method: "exact_bit", MethodVersion: 1, Fields: []string{tab.columns[ordinal].Name}, EstimatedCandidates: -1, Reason: "exact full scan; WHERE before packed-bit distance"}, Field: tab.columns[ordinal].Name, Metric: metric, Dimensions: dims, Limit: options.Limit, Exact: true}}
	if value == nil {
		if err = ctx.Err(); err != nil {
			return BitResult{}, err
		}
		result.Plan.Access, result.Plan.Reason = "EMPTY", "NULL query produces no bit matches"
		return result, nil
	}
	q, _, err := decodeBitColumn(value, dims)
	if err != nil {
		return BitResult{}, err
	}
	if err = ctx.Err(); err != nil {
		return BitResult{}, err
	}
	packedBytes := uint64((dims + 7) / 8)
	if packedBytes > limits.MaxPackedBytes {
		return BitResult{}, fmt.Errorf("%w: bit packed bytes", vector.ErrLimit)
	}
	result.Stats.ScoredPackedBytes = int64(packedBytes)
	if options.Limit == 0 {
		result.Plan.Access, result.Plan.Reason = "EMPTY", "zero limit; query validated without source reads"
		return result, nil
	}
	if annOptions.Mode == VectorModeApproximate {
		result.Plan.VectorRepresentation = "bit"
		hits, done, err := d.typedSearchANN(ctx, reader, tab, ordinal, ann.BitValue(q), vector.MetricKind(metric), annOptions, vector.SearchLimits{MaxCandidates: limits.MaxCandidates, MaxResults: limits.MaxResults, MaxComponents: limits.MaxPackedBytes}, filter, &result.Plan.QueryPlan, &result.Stats.QueryStats, &result.Stats.ANNWork, &result.Stats.ScoredPackedBytes, &result.Stats.ScoredRows, &result.Stats.FilteredRows)
		if err != nil {
			return BitResult{}, err
		}
		if done {
			result.Hits = hits
			result.Plan.Exact = false
			return result, nil
		}
	}
	// Reserve both query/candidate packed storage and result heap before retaining
	// row identities. Duplicate detection is bounded, never silently truncated.
	reserve := int64(options.Limit)*64 + int64(packedBytes)*2
	if reserve >= d.opts.MaxSortBytes {
		return BitResult{}, fmt.Errorf("%w: bit result memory", ErrMemoryLimit)
	}
	memoryCandidates := (d.opts.MaxSortBytes - reserve) / 64
	if memoryCandidates < 1 {
		return BitResult{}, fmt.Errorf("%w: bit identity memory", ErrMemoryLimit)
	}
	src, err := btree.NewCursor(ctx, reader, tab.root)
	if err != nil {
		return BitResult{}, err
	}
	defer src.Close()
	valuesHelper := &writeTable{table: tab}
	seen := make(map[int64]struct{})
	retained := bitHitHeap{}
	for {
		if err = ctx.Err(); err != nil {
			return BitResult{}, err
		}
		id, raw, e := src.Next()
		if e == io.EOF {
			break
		}
		if e != nil {
			return BitResult{}, e
		}
		result.Stats.BaseRowFetches++
		if result.Stats.BaseRowFetches > int64(limits.MaxCandidates) {
			return BitResult{}, fmt.Errorf("%w: bit source candidates", vector.ErrLimit)
		}
		if result.Stats.BaseRowFetches > memoryCandidates {
			return BitResult{}, fmt.Errorf("%w: bit identity memory", ErrMemoryLimit)
		}
		if _, duplicate := seen[id]; duplicate {
			return BitResult{}, fmt.Errorf("%w: duplicate bit source rowid", ErrCorrupt)
		}
		seen[id] = struct{}{}
		values, e := valuesHelper.values(id, raw)
		if e != nil {
			return BitResult{}, e
		}
		if filter != nil {
			keep, e := filter(id, values)
			if e != nil {
				return BitResult{}, e
			}
			if e = ctx.Err(); e != nil {
				return BitResult{}, e
			}
			if !keep {
				result.Stats.FilteredRows++
				continue
			}
		}
		candidate, null, e := decodeBitColumn(values[ordinal], dims)
		if e != nil {
			return BitResult{}, e
		}
		if null {
			result.Stats.NullRows++
			continue
		}
		used := uint64(result.Stats.ScoredPackedBytes) + uint64(result.Stats.ANNWork.ComponentWork)
		if used > limits.MaxPackedBytes || packedBytes > limits.MaxPackedBytes-used {
			return BitResult{}, fmt.Errorf("%w: bit packed bytes", vector.ErrLimit)
		}
		distance, e := metric.DistanceContext(ctx, q, candidate)
		if e != nil {
			return BitResult{}, e
		}
		result.Stats.ScoredRows++
		result.Stats.ScoredPackedBytes += int64(packedBytes)
		hit := vector.Hit{RowID: id, Distance: distance}
		if len(retained) < options.Limit {
			heap.Push(&retained, hit)
		} else if bitHitBetter(hit, retained[0]) {
			retained[0] = hit
			heap.Fix(&retained, 0)
		}
	}
	if err = src.Close(); err != nil {
		return BitResult{}, err
	}
	result.Hits = make([]vector.Hit, len(retained))
	for i := len(result.Hits) - 1; i >= 0; i-- {
		if i&255 == 0 {
			if err = ctx.Err(); err != nil {
				return BitResult{}, err
			}
		}
		result.Hits[i] = heap.Pop(&retained).(vector.Hit)
	}
	if err = ctx.Err(); err != nil {
		return BitResult{}, err
	}
	result.Stats.ReturnedRows = int64(len(result.Hits))
	return result, nil
}

type bitHitHeap []vector.Hit

func bitHitBetter(a, b vector.Hit) bool {
	return a.Distance < b.Distance || a.Distance == b.Distance && a.RowID < b.RowID
}
func (h bitHitHeap) Len() int           { return len(h) }
func (h bitHitHeap) Less(i, j int) bool { return bitHitBetter(h[j], h[i]) }
func (h bitHitHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *bitHitHeap) Push(x any)        { *h = append(*h, x.(vector.Hit)) }
func (h *bitHitHeap) Pop() any          { old := *h; x := old[len(old)-1]; *h = old[:len(old)-1]; return x }
