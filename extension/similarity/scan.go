package similarity

import (
	"container/heap"
	"context"
	"fmt"
)

// Document is a scan row. Null models SQL NULL and is excluded from all
// similarity matches rather than being coerced to empty text.
type Document struct {
	RowID int64
	Text  string
	Null  bool
}

type SearchOptions struct {
	Metric    Metric
	Threshold float64 // inclusive, finite [0,1], compared with rounded Score.Value
	Limit     int     // exact Top-K, 0 returns empty; no unbounded result mode
}

type Hit struct {
	RowID int64
	Score Score
}

type ScanStats struct {
	Mode    CandidateMode
	Visited int
	Scored  int
	Matched int
}

// ScanResult is returned only after complete success. A budget/callback/context
// failure returns its zero value, never a falsely exact partial Top-K.
type ScanResult struct {
	Hits  []Hit
	Stats ScanStats
}

type hitHeap []Hit

func (h hitHeap) Len() int { return len(h) }

// The worst retained hit is the heap root; lower rowids win score ties.
func (h hitHeap) Less(i, j int) bool { return betterHit(h[j], h[i]) }
func (h hitHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *hitHeap) Push(x any)        { *h = append(*h, x.(Hit)) }
func (h *hitHeap) Pop() any          { a := *h; x := a[len(a)-1]; *h = a[:len(a)-1]; return x }
func betterHit(a, b Hit) bool {
	return a.Score.Value > b.Score.Value || a.Score.Value == b.Score.Value && a.RowID < b.RowID
}

// Scan performs a declared FULL SCAN, retaining at most Limit hits. Next must
// yield every visible row at most once; ok=false ends input. Duplicate rowids
// are errors, including duplicates involving NULLs. The caller owns snapshot
// consistency, source closure, and cancellation inside Next. This helper does
// not advertise an index or truncate candidate recall. Filter business predicates
// in the source before ranking. Input order does not affect output: score DESC,
// signed rowid ASC. Only score metadata is retained, not document text.
//
// A zero limit still validates/normalizes the query but does not call Next.
// A nil Next is invalid, even at limit zero. Query NULL must be handled in SQL.
func (e *Engine) Scan(ctx context.Context, query string, next func(context.Context) (Document, bool, error), options SearchOptions) (ScanResult, error) {
	if err := e.check(ctx); err != nil {
		return ScanResult{}, err
	}
	if err := validateMetric(options.Metric); err != nil {
		return ScanResult{}, err
	}
	if err := validateThreshold(options.Threshold); err != nil {
		return ScanResult{}, err
	}
	if next == nil || options.Limit < 0 {
		return ScanResult{}, fmt.Errorf("%w: scan source or limit", ErrValue)
	}
	if options.Limit > e.config.Limits.MaxResults {
		return ScanResult{}, limitError("results")
	}
	if int64(len(query)) > e.config.Limits.MaxTotalInputBytes {
		return ScanResult{}, limitError("total scan input bytes")
	}
	q, err := e.Prepare(ctx, query)
	if err != nil {
		return ScanResult{}, err
	}
	result := ScanResult{Stats: ScanStats{Mode: FullScan}}
	if options.Limit == 0 {
		return result, nil
	}
	seen := make(map[int64]struct{})
	retained := hitHeap{}
	usedBytes := int64(len(query))
	remaining := e.config.Limits.MaxSetComparisons
	for {
		if err := ctx.Err(); err != nil {
			return ScanResult{}, err
		}
		row, ok, err := next(ctx)
		if err != nil {
			return ScanResult{}, err
		}
		if err := ctx.Err(); err != nil {
			return ScanResult{}, err
		}
		if !ok {
			break
		}
		if result.Stats.Visited == e.config.Limits.MaxCandidates {
			return ScanResult{}, limitError("scan candidates")
		}
		result.Stats.Visited++
		if _, exists := seen[row.RowID]; exists {
			return ScanResult{}, fmt.Errorf("%w: duplicate scan rowid", ErrValue)
		}
		seen[row.RowID] = struct{}{}
		if row.Null {
			continue
		}
		if int64(len(row.Text)) > e.config.Limits.MaxTotalInputBytes-usedBytes {
			return ScanResult{}, limitError("total scan input bytes")
		}
		usedBytes += int64(len(row.Text))
		s, err := e.Prepare(ctx, row.Text)
		if err != nil {
			return ScanResult{}, err
		}
		score, err := e.comparePrepared(ctx, q, s, options.Metric, &remaining)
		if err != nil {
			return ScanResult{}, err
		}
		result.Stats.Scored++
		if score.Value < options.Threshold {
			continue
		}
		result.Stats.Matched++
		hit := Hit{row.RowID, score}
		if retained.Len() < options.Limit {
			heap.Push(&retained, hit)
		} else if betterHit(hit, retained[0]) {
			retained[0] = hit
			heap.Fix(&retained, 0)
		}
	}
	result.Hits = make([]Hit, len(retained))
	for i := len(result.Hits) - 1; i >= 0; i-- {
		if i&1023 == 0 {
			if err := ctx.Err(); err != nil {
				return ScanResult{}, err
			}
		}
		result.Hits[i] = heap.Pop(&retained).(Hit)
	}
	if err := ctx.Err(); err != nil {
		return ScanResult{}, err
	}
	return result, nil
}
