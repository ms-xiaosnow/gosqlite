package sqlite

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// VectorSearchOptions selects dense Top-K over one VECTOR(n) column.
// The default Mode is exact. Approximate mode uses a matching persistent ANN
// index and exact base-row rescoring; exact distances do not imply exact recall.
// Empty Metric means L2; only builtin MetricKind values are accepted. Where is
// one supported SQL expression evaluated before candidate vector decoding and
// scoring. Args follow Query binding rules. Limit is required; zero validates
// query/options without reading source rows. A zero Limits uses the bounded
// defaults; other limits must be complete. All fetched rows, including filtered
// and NULL rows, count against MaxCandidates, including an abandoned ANN
// attempt before an exact fallback. MaxComponents also charges graph distances.
type VectorSearchOptions struct {
	Mode                  VectorSearchMode
	Index                 string
	EFSearch, MaxEFSearch int
	// Probes are reserved for IVFFlat and currently return unsupported.
	Probes, MaxProbes int
	ANNLimits         ANNWorkLimits
	Metric            vector.MetricKind
	Limit             int
	Where             string
	Args              []any
	Limits            vector.SearchLimits
}

// VectorPlan identifies the actual completed access and recall mode.
// Native search excludes NULL vector candidates. SQL ORDER BY instead keeps
// SQLite's NULL-first ascending ordering.
type VectorPlan struct {
	QueryPlan
	Field      string
	Metric     vector.MetricKind
	Dimensions int
	Limit      int
	Exact      bool
}

func (p VectorPlan) Detail() string {
	return fmt.Sprintf("%s %s; vector_field=%s; vector_metric=%s; dimensions=%d; limit=%d; exact=%t; distance_order=ASC,rowid_ASC; null_candidates=excluded; reason=%s", p.Access, p.Table, p.Field, p.Metric, p.Dimensions, p.Limit, p.Exact, p.Reason) + fmt.Sprintf("; method=%s; indexes=%v", p.Method, p.Indexes) + p.annDetail()
}

// VectorStats reports actual source/ranking work, not B-tree page I/O.
// ScoredComponents charges the validated query once and every scored candidate.
// ANNDistanceEvaluations * Dimensions reports additional graph components.
// Both are charged against Limits.MaxComponents. Where evaluation and scalar
// projection work are outside this ranking counter.
type VectorStats struct {
	QueryStats
	FilteredRows     int64
	ScoredRows       int64
	NullRows         int64
	ScoredComponents int64
}
type VectorResult struct {
	Hits  []vector.Hit
	Plan  VectorPlan
	Stats VectorStats
}

func (d *DB) VectorSearch(ctx context.Context, table, field string, query any, options VectorSearchOptions) (VectorResult, error) {
	if err := checkContext(ctx); err != nil {
		return VectorResult{}, wrap("vector search", err)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return VectorResult{}, wrap("vector search", ErrClosed)
	}
	if d.tx != nil {
		if err := d.tx.check(); err != nil {
			return VectorResult{}, wrap("vector search", err)
		}
		result, err := d.vectorSearch(ctx, d.tx.reader, table, field, query, options)
		return result, wrap("vector search", err)
	}
	reader, err := d.openReader(ctx)
	if err != nil {
		return VectorResult{}, wrap("vector search", err)
	}
	d.info = fileInfo(reader.Header())
	result, err := d.vectorSearch(ctx, reader, table, field, query, options)
	err = errors.Join(err, reader.Close())
	if err != nil {
		return VectorResult{}, wrap("vector search", err)
	}
	return result, nil
}

func (t *Tx) VectorSearch(ctx context.Context, table, field string, query any, options VectorSearchOptions) (VectorResult, error) {
	if err := checkContext(ctx); err != nil {
		return VectorResult{}, wrap("vector search", err)
	}
	t.db.mu.Lock()
	defer t.db.mu.Unlock()
	if err := t.check(); err != nil {
		return VectorResult{}, wrap("vector search", err)
	}
	result, err := t.db.vectorSearch(ctx, t.reader, table, field, query, options)
	return result, wrap("vector search", err)
}

// Keep the accepted bounds equal to the immutable mathematical Scan contract.
func vectorLimits(l vector.SearchLimits) (vector.SearchLimits, error) {
	if l == (vector.SearchLimits{}) {
		l = vector.DefaultSearchLimits()
	}
	if l.MaxCandidates < 1 || l.MaxCandidates > 1<<20 || l.MaxResults < 1 || l.MaxResults > 1<<20 || l.MaxComponents < 1 || l.MaxComponents > 1<<34 {
		return vector.SearchLimits{}, vector.ErrOptions
	}
	return l, nil
}
func vectorMetric(m vector.MetricKind) (vector.MetricKind, error) {
	if m == "" {
		m = vector.L2
	}
	switch m {
	case vector.L2, vector.L1, vector.Cosine, vector.NegativeIP:
		return m, nil
	}
	return "", vector.ErrMetric
}

// Convert borrowed input before cloning. Valuer output is still untrusted data,
// and a callback that canceled its context cannot produce EMPTY success.
func vectorQueryValue(ctx context.Context, query any) (any, error) {
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
			return nil, fmt.Errorf("%w: vector query text bytes", vector.ErrLimit)
		}
	case []byte:
		if len(v) > vector.MaxBinaryBytes {
			return nil, fmt.Errorf("%w: vector query binary bytes", vector.ErrLimit)
		}
		if v == nil {
			return nil, nil
		}
	case nil:
	default:
		return nil, fmt.Errorf("%w: vector query requires text or binary vector", ErrMismatch)
	}
	return value, nil
}

func (d *DB) vectorSearch(ctx context.Context, reader btree.PageReader, tableName, fieldName string, query any, options VectorSearchOptions) (VectorResult, error) {
	options, err := normalizeVectorANNOptions(options)
	if err != nil {
		return VectorResult{}, err
	}
	limits, err := vectorLimits(options.Limits)
	if err != nil {
		return VectorResult{}, err
	}
	metric, err := vectorMetric(options.Metric)
	if err != nil {
		return VectorResult{}, err
	}
	if options.Limit < 0 {
		return VectorResult{}, vector.ErrOptions
	}
	if options.Limit > limits.MaxResults {
		return VectorResult{}, fmt.Errorf("%w: results", vector.ErrLimit)
	}
	tab, err := d.table(ctx, reader, &sqlast.Select{From: tableName})
	if err != nil {
		return VectorResult{}, err
	}
	if tab == nil {
		return VectorResult{}, fmt.Errorf("vector search requires a source table")
	}
	ordinal, err := tab.column("", fieldName)
	if err != nil {
		return VectorResult{}, err
	}
	if ordinal < 0 {
		return VectorResult{}, fmt.Errorf("%w: vector search requires a VECTOR(n) column", ErrMismatch)
	}
	dims, ok := sqlast.VectorDimensions(tab.columns[ordinal].Type)
	if !ok {
		return VectorResult{}, fmt.Errorf("%w: vector search requires a VECTOR(n) column", ErrMismatch)
	}
	// Reuse the strict single-expression binder, including named arguments and
	// snapshot-bound grams13_match functions; it does not execute a source scan.
	filter, err := d.similarityFilter(ctx, reader, tab, SimilarityOptions{Where: options.Where, Args: options.Args})
	if err != nil {
		return VectorResult{}, err
	}
	value, err := vectorQueryValue(ctx, query)
	if err != nil {
		return VectorResult{}, err
	}
	result := VectorResult{Plan: VectorPlan{QueryPlan: QueryPlan{Access: "SCAN", Table: tab.name, Method: "exact_vector", MethodVersion: 1, Fields: []string{tab.columns[ordinal].Name}, EstimatedCandidates: -1, Reason: "exact full scan; WHERE before distance"}, Field: tab.columns[ordinal].Name, Metric: metric, Dimensions: dims, Limit: options.Limit, Exact: true}}
	if value == nil {
		if err = ctx.Err(); err != nil {
			return VectorResult{}, err
		}
		result.Plan.Access, result.Plan.Reason = "EMPTY", "NULL query produces no vector matches"
		return result, nil
	}
	q, _, err := decodeVectorColumn(value, dims)
	if err != nil {
		return VectorResult{}, err
	}
	if err = ctx.Err(); err != nil {
		return VectorResult{}, err
	}
	// A no-source mathematical scan validates metric-specific query rules,
	// including cosine zero norms, and all arithmetic/result quotas.
	if _, err = vector.Scan(ctx, q, func(context.Context) (vector.Row, bool, error) { return vector.Row{}, false, nil }, vector.SearchOptions{Metric: metric, Limit: 0, Limits: limits}); err != nil {
		return VectorResult{}, err
	}
	result.Stats.ScoredComponents = int64(dims)
	if options.Limit == 0 {
		result.Plan.Access, result.Plan.Reason = "EMPTY", "zero limit; query validated without source reads"
		return result, nil
	}
	if options.Mode == VectorModeApproximate {
		done, err := d.vectorSearchANN(ctx, reader, tab, ordinal, q, options, limits, filter, &result)
		if err != nil {
			return VectorResult{}, err
		}
		if done {
			return result, nil
		}
	}
	// Reserve result/query memory, then bound duplicate-row identity storage.
	// This conservative retained-memory accounting never silently truncates rows.
	reserve := int64(options.Limit)*128 + int64(dims)*4
	if reserve >= d.opts.MaxSortBytes {
		return VectorResult{}, fmt.Errorf("%w: vector result memory", ErrMemoryLimit)
	}
	memoryCandidates := (d.opts.MaxSortBytes - reserve) / 64
	if memoryCandidates < 1 {
		return VectorResult{}, fmt.Errorf("%w: vector candidate memory", ErrMemoryLimit)
	}
	src, err := btree.NewCursor(ctx, reader, tab.root)
	if err != nil {
		return VectorResult{}, err
	}
	defer src.Close()
	valuesHelper := &writeTable{table: tab}
	remaining := limits
	remaining.MaxCandidates -= int(result.Stats.BaseRowFetches)
	usedComponents := uint64(result.Stats.ScoredComponents) + uint64(result.Stats.ANNDistanceEvaluations)*uint64(dims)
	if remaining.MaxCandidates < 1 || usedComponents > limits.MaxComponents {
		return VectorResult{}, fmt.Errorf("%w: cumulative vector work", vector.ErrLimit)
	}
	remaining.MaxComponents = limits.MaxComponents - usedComponents + uint64(dims)
	scanned, err := vector.Scan(ctx, q, func(ctx context.Context) (vector.Row, bool, error) {
		id, raw, err := src.Next()
		if err == io.EOF {
			return vector.Row{}, false, nil
		}
		if err != nil {
			return vector.Row{}, false, err
		}
		result.Stats.BaseRowFetches++
		if result.Stats.BaseRowFetches > int64(limits.MaxCandidates) {
			return vector.Row{}, false, fmt.Errorf("%w: source candidates", vector.ErrLimit)
		}
		if result.Stats.BaseRowFetches > memoryCandidates {
			return vector.Row{}, false, fmt.Errorf("%w: vector candidate memory", ErrMemoryLimit)
		}
		values, err := valuesHelper.values(id, raw)
		if err != nil {
			return vector.Row{}, false, err
		}
		if filter != nil {
			keep, err := filter(id, values)
			if err != nil {
				return vector.Row{}, false, err
			}
			if err = ctx.Err(); err != nil {
				return vector.Row{}, false, err
			}
			if !keep {
				result.Stats.FilteredRows++
				// Yield the identity so Scan's duplicate and candidate budgets include it.
				return vector.Row{RowID: id, Null: true}, true, nil
			}
		}
		candidate, null, err := decodeVectorColumn(values[ordinal], dims)
		if err != nil {
			return vector.Row{}, false, err
		}
		if null {
			result.Stats.NullRows++
		}
		return vector.Row{RowID: id, Vector: candidate, Null: null}, true, nil
	}, vector.SearchOptions{Metric: metric, Limit: options.Limit, Limits: remaining})
	closeErr := src.Close()
	if err = errors.Join(err, closeErr); err != nil {
		return VectorResult{}, err
	}
	result.Hits = scanned.Hits
	result.Stats.ScoredRows += int64(scanned.Stats.Scored)
	result.Stats.ScoredComponents += int64(scanned.Stats.Scored) * int64(dims)
	result.Stats.ReturnedRows = int64(len(result.Hits))
	return result, nil
}
