package similarity

import (
	"context"
	"fmt"

	"github.com/ms-xiaosnow/gosqlite/extension/grams13"
)

// Set is a prepared immutable token set. Its zero value is invalid; nil is not
// SQL NULL. Tokens returns a copy, and no caller-owned slice is retained.
type Set struct {
	tokens   []grams13.Token
	identity [32]byte
	valid    bool
}

func (s *Set) Len() int {
	if s == nil {
		return 0
	}
	return len(s.tokens)
}

// Tokens returns a copy sorted by Length and then UTF-8 Text bytes. These are
// the complete selected query grams, suitable for a proven postings union.
func (s *Set) Tokens() []grams13.Token {
	if s == nil {
		return nil
	}
	return append([]grams13.Token(nil), s.tokens...)
}

func (s *Set) ConfigHash() [32]byte {
	if s == nil {
		return [32]byte{}
	}
	return s.identity
}

func (e *Engine) Prepare(ctx context.Context, input string) (*Set, error) {
	if err := e.check(ctx); err != nil {
		return nil, err
	}
	tokens, err := e.tokenizer.Tokenize(ctx, input)
	if err != nil {
		return nil, err
	}
	start, end := 0, len(tokens)
	for start < end && int(tokens[start].Length) < e.config.MinGram {
		if start&1023 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		start++
	}
	for end > start && int(tokens[end-1].Length) > e.config.MaxGram {
		if end&1023 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		end--
	}
	// Retain at most the existing bounded tokenizer allocation; no second
	// potentially large copy is necessary because tokens are private.
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &Set{tokens[start:end:end], e.identity, true}, nil
}

// Score records exact integer cardinalities and their correctly rounded
// float64 ratio. Threshold and ranking APIs compare Value, including ties at
// float64 rounding boundaries. The counts allow a caller to recover a rational
// representation if a different comparison contract is needed.
type Score struct {
	Value        float64
	LeftTokens   int
	RightTokens  int
	Intersection int
}

func scoreValue(a, b, common int, metric Metric) float64 {
	if a == 0 && b == 0 {
		return 1
	}
	if metric == Dice {
		return float64(2*common) / float64(a+b)
	}
	return float64(common) / float64(a+b-common)
}

func (e *Engine) Compare(ctx context.Context, left, right string, metric Metric) (Score, error) {
	if err := e.check(ctx); err != nil {
		return Score{}, err
	}
	if err := validateMetric(metric); err != nil {
		return Score{}, err
	}
	a, err := e.Prepare(ctx, left)
	if err != nil {
		return Score{}, err
	}
	b, err := e.Prepare(ctx, right)
	if err != nil {
		return Score{}, err
	}
	return e.ComparePrepared(ctx, a, b, metric)
}

func (e *Engine) ComparePrepared(ctx context.Context, left, right *Set, metric Metric) (Score, error) {
	if err := e.check(ctx); err != nil {
		return Score{}, err
	}
	remaining := e.config.Limits.MaxSetComparisons
	return e.comparePrepared(ctx, left, right, metric, &remaining)
}

func (e *Engine) comparePrepared(ctx context.Context, left, right *Set, metric Metric, remaining *uint64) (Score, error) {
	if err := e.check(ctx); err != nil {
		return Score{}, err
	}
	if err := validateMetric(metric); err != nil {
		return Score{}, err
	}
	if left == nil || right == nil || !left.valid || !right.valid || left.identity != e.identity || right.identity != e.identity {
		return Score{}, ErrIdentity
	}
	i, j, common, steps := 0, 0, 0, 0
	for i < len(left.tokens) && j < len(right.tokens) {
		if *remaining == 0 {
			return Score{}, limitError("set comparisons")
		}
		*remaining -= 1
		if steps&1023 == 0 {
			if err := ctx.Err(); err != nil {
				return Score{}, err
			}
		}
		steps++
		a, b := left.tokens[i], right.tokens[j]
		if a == b {
			common++
			i++
			j++
		} else if a.Length < b.Length || a.Length == b.Length && a.Text < b.Text {
			i++
		} else {
			j++
		}
	}
	if err := ctx.Err(); err != nil {
		return Score{}, err
	}
	return Score{scoreValue(len(left.tokens), len(right.tokens), common, metric), len(left.tokens), len(right.tokens), common}, nil
}

const maxCardinality = 1 << 20 // grams13 hard ceiling on distinct tokens

func validateCardinality(n int) error {
	if n < 0 || n > maxCardinality {
		return fmt.Errorf("%w: token cardinality", ErrValue)
	}
	return nil
}

// MinimumIntersection gives the least intersection size that can pass the
// inclusive float64 threshold for the given cardinalities, and possible=false
// if even maximal overlap cannot pass. Because the same monotone scoreValue is
// used as final scoring, this avoids an unsafe ceil(t*(a+b)/2) rounding shortcut.
// Dice's real-number bound is ceil(t*(a+b)/2); Jaccard's is
// ceil(t*(a+b)/(1+t)). Binary search implements the declared rounded-score
// contract at exact threshold boundaries. Counts are bounded before arithmetic.
func MinimumIntersection(a, b int, threshold float64, metric Metric) (minimum int, possible bool, err error) {
	if err = validateCardinality(a); err != nil {
		return
	}
	if err = validateCardinality(b); err != nil {
		return
	}
	if err = validateThreshold(threshold); err != nil {
		return
	}
	if err = validateMetric(metric); err != nil {
		return
	}
	hi := min(a, b)
	if scoreValue(a, b, hi, metric) < threshold {
		return 0, false, nil
	}
	lo := 0
	for lo < hi {
		mid := lo + (hi-lo)/2
		if scoreValue(a, b, mid, metric) >= threshold {
			hi = mid
		} else {
			lo = mid + 1
		}
	}
	return lo, true, nil
}

type CandidateMode string

const (
	FullScan      CandidateMode = "full-scan"
	UnionPostings CandidateMode = "union-all-query-gram-postings"
)

type CandidatePlan struct {
	Mode   CandidateMode
	Reason string
}

// PlanCandidates is a correctness rule, not an index implementation. For a
// nonempty query set and positive threshold, any qualifying Dice/Jaccard pair
// has positive intersection. The UNION of EVERY selected query gram posting
// therefore contains every qualifying row. Apply exact scoring and all business
// predicates before selecting Top-K. Never cap postings/candidates or substitute
// an AND. It requires matching tokenizer/config identity, complete transaction-
// consistent postings, and no stale rows; otherwise the caller MUST full-scan.
//
// Empty-query sets need empty-document sets (score 1), which have no posting;
// threshold zero admits disjoint sets. Both require a full scan here. Unrestricted
// Top-K uses threshold zero and must retain zero-score rowid ties, so also scans.
// Top-K with an explicit positive threshold can use the union, even if fewer
// than K rows qualify. Any resource-limit failure must return an error, never a
// silently truncated supposedly exact answer.
func PlanCandidates(queryTokens int, threshold float64) (CandidatePlan, error) {
	if err := validateCardinality(queryTokens); err != nil {
		return CandidatePlan{}, err
	}
	if err := validateThreshold(threshold); err != nil {
		return CandidatePlan{}, err
	}
	if queryTokens == 0 {
		return CandidatePlan{FullScan, "empty query token set requires rows without postings"}, nil
	}
	if threshold == 0 {
		return CandidatePlan{FullScan, "zero threshold includes disjoint and zero-score ties"}, nil
	}
	return CandidatePlan{UnionPostings, "positive score implies a shared query gram; exact recheck required"}, nil
}
