package vector

import (
	"container/heap"
	"context"
	"fmt"
	"math"
	"reflect"
)

// Row is one visible database row or in-memory candidate. Null explicitly marks
// SQL NULL and causes Vector to be ignored. A non-NULL zero Vector is an error.
type Row struct {
	RowID  int64
	Vector Vector
	Null   bool
}

// Hit contains only the row identity and exact distance, never candidate data.
type Hit struct {
	RowID    int64
	Distance float64
}

// SearchLimits bound retained identities, hits, and arithmetic work. Every row
// counts against MaxCandidates, including NULL and filtered rows; query Dims
// plus every scored row's Dims count against MaxComponents. These are logical
// budgets, not an exact byte or wall-clock bound. Callback and caller-owned
// memory are outside the budgets. A completely zero limits value uses defaults;
// partially specified limits are rejected, and zero never means unlimited.
type SearchLimits struct {
	MaxCandidates int
	MaxResults    int
	MaxComponents uint64
}

func DefaultSearchLimits() SearchLimits {
	return SearchLimits{MaxCandidates: 100000, MaxResults: 10000, MaxComponents: 1 << 27}
}

func (l SearchLimits) normalized() (SearchLimits, error) {
	if l == (SearchLimits{}) {
		l = DefaultSearchLimits()
	}
	if l.MaxCandidates < 1 || l.MaxCandidates > 1<<20 ||
		l.MaxResults < 1 || l.MaxResults > 1<<20 ||
		l.MaxComponents < 1 || l.MaxComponents > 1<<34 {
		return SearchLimits{}, ErrOptions
	}
	return l, nil
}

// SearchOptions specifies exact Top-K with optional business filtering. Filter
// runs on non-NULL rows before vector validation and scoring, so excluded rows
// may have a different dimension. Filter and the source must honor ctx inside
// their own work. Callback errors abort the whole search without partial hits.
type SearchOptions struct {
	Metric Metric
	Limit  int
	Limits SearchLimits
	Filter func(context.Context, Row) (bool, error)
}

// ScanStats describe actual full-scan work. No ANN or indexed plan is claimed.
type ScanStats struct {
	Visited  int
	Scored   int
	Filtered int
	Nulls    int
}

type SearchResult struct {
	Hits  []Hit
	Stats ScanStats
}

// TopK searches a caller-owned slice using Scan. The slice must not be mutated
// concurrently with the call; individual Vectors remain immutable and shareable.
func TopK(ctx context.Context, query Vector, rows []Row, options SearchOptions) (SearchResult, error) {
	i := 0
	return Scan(ctx, query, func(context.Context) (Row, bool, error) {
		if i == len(rows) {
			return Row{}, false, nil
		}
		r := rows[i]
		i++
		return r, true, nil
	}, options)
}

// Scan is an exact FULL SCAN reference, retaining the best Limit rows by
// distance ASC then signed rowid ASC. It uses O(MaxCandidates+Limit) bounded
// auxiliary storage for duplicate detection and a worst-first heap. Every rowid
// must be unique, including NULL and excluded rows; duplicates are errors.
// A NULL row is excluded rather than treated as a zero vector. Cosine on an
// eligible zero-norm candidate returns ErrZeroNorm instead of silently skipping.
//
// The caller owns source closure, snapshot/transaction visibility, and callback
// cancellation. Scan owns no persistence, index, or database transaction. Context
// is checked around each callback and metric call and during output extraction;
// a built-in metric does at most MaxDimensions components between checks.
// Errors return the zero result, never a falsely exact partial Top-K. Limit=0
// validates options and the query without reading the source; nil next is still
// invalid. A nil context fails rather than panicking.
func Scan(ctx context.Context, query Vector, next func(context.Context) (Row, bool, error), options SearchOptions) (SearchResult, error) {
	if ctx == nil {
		return SearchResult{}, ErrContext
	}
	if err := ctx.Err(); err != nil {
		return SearchResult{}, err
	}
	if !query.Valid() {
		return SearchResult{}, ErrInvalid
	}
	if next == nil || options.Limit < 0 {
		return SearchResult{}, ErrOptions
	}
	limits, err := options.Limits.normalized()
	if err != nil {
		return SearchResult{}, err
	}
	if options.Limit > limits.MaxResults {
		return SearchResult{}, fmt.Errorf("%w: results", ErrLimit)
	}
	if nilMetric(options.Metric) {
		return SearchResult{}, ErrMetric
	}
	// MetricKind has value methods, so its pointer also satisfies Metric. Copy
	// it here to apply identical query validation and freeze its selected kind.
	if m, ok := options.Metric.(*MetricKind); ok {
		options.Metric = *m
	}
	if m, ok := options.Metric.(MetricKind); ok {
		switch m {
		case L2, L1, NegativeIP:
		case Cosine:
			norm, _ := Norm(query)
			if norm == 0 {
				return SearchResult{}, ErrZeroNorm
			}
		default:
			return SearchResult{}, ErrMetric
		}
	}
	used := uint64(query.Dims())
	if used > limits.MaxComponents {
		return SearchResult{}, fmt.Errorf("%w: components", ErrLimit)
	}
	if err := ctx.Err(); err != nil {
		return SearchResult{}, err
	}
	if options.Limit == 0 {
		return SearchResult{}, nil
	}
	result := SearchResult{}
	seen := make(map[int64]struct{})
	retained := hitHeap{}
	for {
		if err := ctx.Err(); err != nil {
			return SearchResult{}, err
		}
		row, ok, err := next(ctx)
		if err != nil {
			return SearchResult{}, err
		}
		if err := ctx.Err(); err != nil {
			return SearchResult{}, err
		}
		if !ok {
			break
		}
		if result.Stats.Visited == limits.MaxCandidates {
			return SearchResult{}, fmt.Errorf("%w: candidates", ErrLimit)
		}
		result.Stats.Visited++
		if _, exists := seen[row.RowID]; exists {
			return SearchResult{}, ErrDuplicateRowID
		}
		seen[row.RowID] = struct{}{}
		if row.Null {
			result.Stats.Nulls++
			continue
		}
		if options.Filter != nil {
			include, err := options.Filter(ctx, row)
			if err != nil {
				return SearchResult{}, err
			}
			if err := ctx.Err(); err != nil {
				return SearchResult{}, err
			}
			if !include {
				result.Stats.Filtered++
				continue
			}
		}
		if err := checkPair(query, row.Vector); err != nil {
			return SearchResult{}, fmt.Errorf("row %d: %w", row.RowID, err)
		}
		if uint64(row.Vector.Dims()) > limits.MaxComponents-used {
			return SearchResult{}, fmt.Errorf("%w: components", ErrLimit)
		}
		used += uint64(row.Vector.Dims())
		distance, err := options.Metric.Distance(query, row.Vector)
		if err != nil {
			return SearchResult{}, fmt.Errorf("row %d: %w", row.RowID, err)
		}
		if err := ctx.Err(); err != nil {
			return SearchResult{}, err
		}
		if math.IsNaN(distance) || math.IsInf(distance, 0) {
			return SearchResult{}, ErrNonFinite
		}
		result.Stats.Scored++
		hit := Hit{RowID: row.RowID, Distance: distance}
		if len(retained) < options.Limit {
			heap.Push(&retained, hit)
		} else if better(hit, retained[0]) {
			retained[0] = hit
			heap.Fix(&retained, 0)
		}
	}
	result.Hits = make([]Hit, len(retained))
	for i := len(result.Hits) - 1; i >= 0; i-- {
		if i&255 == 0 {
			if err := ctx.Err(); err != nil {
				return SearchResult{}, err
			}
		}
		result.Hits[i] = heap.Pop(&retained).(Hit)
	}
	if err := ctx.Err(); err != nil {
		return SearchResult{}, err
	}
	return result, nil
}

// A typed-nil metric must fail like a nil interface instead of reaching a
// compiler-generated value-method dereference. Callbacks are otherwise trusted
// caller code: their panics and internal resource consumption are not contained.
func nilMetric(m Metric) bool {
	if m == nil {
		return true
	}
	v := reflect.ValueOf(m)
	switch v.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return v.IsNil()
	default:
		return false
	}
}

func better(a, b Hit) bool {
	return a.Distance < b.Distance || a.Distance == b.Distance && a.RowID < b.RowID
}

type hitHeap []Hit

func (h hitHeap) Len() int           { return len(h) }
func (h hitHeap) Less(i, j int) bool { return better(h[j], h[i]) }
func (h hitHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *hitHeap) Push(x any)        { *h = append(*h, x.(Hit)) }
func (h *hitHeap) Pop() any          { a := *h; x := a[len(a)-1]; *h = a[:len(a)-1]; return x }
