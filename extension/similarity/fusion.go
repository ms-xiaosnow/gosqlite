package similarity

import (
	"container/heap"
	"context"
	"fmt"
	"math"
)

// RankedList is an ordered best-first list with one occurrence of each rowid.
// A row may occur in multiple lists. Weight must be finite and strictly positive;
// Name is optional metadata (at most 1024 bytes), not a unique list identifier.
type RankedList struct {
	Name   string
	Weight float64
	RowIDs []int64
}

// Contribution preserves each source's one-based rank, original weight, and
// rounded weighted reciprocal term. List is the zero-based input list index.
type Contribution struct {
	List   int
	Name   string
	Rank   int
	Weight float64
	Value  float64
}

type FusedHit struct {
	RowID         int64
	Score         float64
	Contributions []Contribution // input-list order
}

// Fuse computes weighted reciprocal rank fusion: sum(weight/(offset+rank)),
// with rank starting at 1. Offset must be finite and strictly positive. Inputs
// and per-row terms are processed in input-list order, making rounding and ties
// deterministic; output is score DESC, signed rowid ASC. A missing row contributes
// zero; nil/empty lists produce an empty result. An empty list still requires a
// valid weight. A zero result limit validates all input without returning hits.
//
// This is exact for these supplied ranked lists, not evidence that an upstream
// approximate list had complete recall. Overflow/nonfinite sums fail rather than
// rank unreliable values. Positive tiny terms may round to zero under IEEE-754.
// All rank entries (including repeated rows across lists) share MaxCandidates.
// The intermediate maps/details therefore use O(MaxCandidates) bounded storage.
func (e *Engine) Fuse(ctx context.Context, lists []RankedList, offset float64, limit int) ([]FusedHit, error) {
	if err := e.check(ctx); err != nil {
		return nil, err
	}
	if math.IsNaN(offset) || math.IsInf(offset, 0) || offset <= 0 || limit < 0 {
		return nil, fmt.Errorf("%w: fusion offset or limit", ErrValue)
	}
	if limit > e.config.Limits.MaxResults {
		return nil, limitError("fusion results")
	}
	if len(lists) > e.config.Limits.MaxFusionLists {
		return nil, limitError("fusion lists")
	}
	total := 0
	for _, list := range lists {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if math.IsNaN(list.Weight) || math.IsInf(list.Weight, 0) || list.Weight <= 0 || len(list.Name) > 1024 {
			return nil, fmt.Errorf("%w: fusion weight or name", ErrValue)
		}
		if len(list.RowIDs) > e.config.Limits.MaxCandidates-total {
			return nil, limitError("fusion ranks")
		}
		total += len(list.RowIDs)
	}
	rows := make(map[int64]*FusedHit)
	for listIndex, list := range lists {
		seen := make(map[int64]struct{})
		for j, rowid := range list.RowIDs {
			if j&1023 == 0 {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
			}
			if _, exists := seen[rowid]; exists {
				return nil, fmt.Errorf("%w: duplicate fusion rowid in list %d", ErrValue, listIndex)
			}
			seen[rowid] = struct{}{}
			term := list.Weight / (offset + float64(j+1))
			row := rows[rowid]
			if row == nil {
				row = &FusedHit{RowID: rowid}
				rows[rowid] = row
			}
			sum := row.Score + term
			if math.IsInf(sum, 0) || math.IsNaN(sum) {
				return nil, fmt.Errorf("%w: nonfinite fusion sum", ErrValue)
			}
			row.Score = sum
			row.Contributions = append(row.Contributions, Contribution{listIndex, list.Name, j + 1, list.Weight, term})
		}
	}
	retained := hitHeap{}
	steps := 0
	if limit > 0 {
		for _, row := range rows {
			if steps&1023 == 0 {
				if err := ctx.Err(); err != nil {
					return nil, err
				}
			}
			steps++
			hit := Hit{RowID: row.RowID, Score: Score{Value: row.Score}}
			if retained.Len() < limit {
				heap.Push(&retained, hit)
			} else if betterHit(hit, retained[0]) {
				retained[0] = hit
				heap.Fix(&retained, 0)
			}
		}
	}
	result := make([]FusedHit, len(retained))
	for i := len(result) - 1; i >= 0; i-- {
		if i&1023 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		rowid := heap.Pop(&retained).(Hit).RowID
		result[i] = *rows[rowid]
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
