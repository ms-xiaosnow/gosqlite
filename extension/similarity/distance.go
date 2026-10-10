package similarity

import (
	"context"
	"fmt"
)

// Distance is exact when Exact is true. With a maximum, a result beyond the
// bound is always {maximum+1, false}; that sentinel is NOT the actual distance.
// Without a maximum every successful result is exact. Insert, delete, and
// substitute each cost one Unicode code point after the configured normalization.
// No transposition or grapheme-cluster operation is implied.
type Distance struct {
	Value int
	Exact bool
}

func boundedRunes(ctx context.Context, s string, limit int) ([]rune, error) {
	out := make([]rune, 0, min(len(s), limit))
	for _, r := range s {
		if len(out)&1023 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		if len(out) == limit {
			return nil, limitError("edit-distance runes")
		}
		out = append(out, r)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// Levenshtein uses two rows, O(min(m,n)) memory, and a |i-j| <= maximum band
// when supplied. If a row's reachable minimum exceeds the maximum no later
// path can recover (edit costs are nonnegative), so computation stops early.
// MaxDPCells is charged before each visited recurrence cell; no partial or
// approximate result escapes on cancellation or budget exhaustion. Maximum must
// be nonnegative and less than max int so its sentinel is representable.
func (e *Engine) Levenshtein(ctx context.Context, left, right string, maximum *int) (Distance, error) {
	if err := e.check(ctx); err != nil {
		return Distance{}, err
	}
	if maximum != nil && (*maximum < 0 || *maximum == int(^uint(0)>>1)) {
		return Distance{}, fmt.Errorf("%w: maximum distance", ErrValue)
	}
	left, err := e.tokenizer.Normalize(ctx, left)
	if err != nil {
		return Distance{}, err
	}
	right, err = e.tokenizer.Normalize(ctx, right)
	if err != nil {
		return Distance{}, err
	}
	a, err := boundedRunes(ctx, left, e.config.Limits.MaxDistanceRunes)
	if err != nil {
		return Distance{}, err
	}
	b, err := boundedRunes(ctx, right, e.config.Limits.MaxDistanceRunes)
	if err != nil {
		return Distance{}, err
	}
	if len(a) < len(b) {
		a, b = b, a
	}
	n, m := len(a), len(b)
	k := n
	if maximum != nil {
		k = min(k, *maximum)
	}
	exceeded := func() (Distance, error) {
		if err := ctx.Err(); err != nil {
			return Distance{}, err
		}
		return Distance{k + 1, false}, nil
	}
	if n-m > k {
		return exceeded()
	}
	if m == 0 {
		if err := ctx.Err(); err != nil {
			return Distance{}, err
		}
		return Distance{n, true}, nil
	}
	if left == right {
		if err := ctx.Err(); err != nil {
			return Distance{}, err
		}
		return Distance{0, true}, nil
	}
	inf := k + 1
	prev, cur := make([]int, m+1), make([]int, m+1)
	for j := 0; j <= m; j++ {
		if j&1023 == 0 {
			if err := ctx.Err(); err != nil {
				return Distance{}, err
			}
		}
		prev[j] = min(j, inf)
	}
	var cells uint64
	for i := 1; i <= n; i++ {
		if err := ctx.Err(); err != nil {
			return Distance{}, err
		}
		lo, hi := max(1, i-k), min(m, i+k)
		if lo == 1 {
			cur[0] = min(i, inf)
		} else {
			cur[lo-1] = inf
		}
		if hi < m {
			cur[hi+1] = inf
		}
		rowMin := cur[lo-1]
		for j := lo; j <= hi; j++ {
			if cells == e.config.Limits.MaxDPCells {
				return Distance{}, limitError("edit-distance cells")
			}
			if cells&1023 == 0 {
				if err := ctx.Err(); err != nil {
					return Distance{}, err
				}
			}
			cells++
			cost := 0
			if a[i-1] != b[j-1] {
				cost = 1
			}
			cur[j] = min(inf, prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
			rowMin = min(rowMin, cur[j])
		}
		if rowMin > k {
			return exceeded()
		}
		prev, cur = cur, prev
	}
	if err := ctx.Err(); err != nil {
		return Distance{}, err
	}
	if prev[m] > k {
		return exceeded()
	}
	return Distance{prev[m], true}, nil
}
