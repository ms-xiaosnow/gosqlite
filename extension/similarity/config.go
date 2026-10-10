// Package similarity implements bounded, exact Unicode set similarity, rune
// edit distance, full-scan ranking, and reciprocal rank fusion. It is a pure
// mathematical layer: it owns no database, persistence, or SQL execution.
//
// Gram sets contain distinct contiguous code-point grams with no padding or
// word boundaries. Repetition does not change their multiplicity. Two empty
// sets have similarity 1, even when both nonempty input strings are shorter
// than MinGram; exactly one empty set has similarity 0. NULL is not an empty
// string: SQL adapters must propagate NULL before calling string APIs. Scan
// excludes documents explicitly marked Null.
//
// Unicode conversion is delegated to grams13.Tokenizer without additional
// normalization. In particular NFC, folding, NUL handling, stream-safe rejection,
// invalid UTF-8, and acceptance budgets have exactly that tokenizer's semantics.
// These routines are not pg_bigm-compatible and make no such claim.
package similarity

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	"github.com/ms-xiaosnow/gosqlite/extension/grams13"
)

const Version = 1

type Metric string

const (
	Dice    Metric = "dice"
	Jaccard Metric = "jaccard"
)

var (
	ErrConfig   = errors.New("similarity: invalid configuration")
	ErrValue    = errors.New("similarity: invalid argument")
	ErrLimit    = errors.New("similarity: resource limit exceeded")
	ErrContext  = errors.New("similarity: nil context")
	ErrIdentity = errors.New("similarity: token-set configuration mismatch")
)

// Limits are strict per-operation budgets, independent of the tokenizer's
// strict per-text budgets. MaxDPCells counts visited recurrence cells (not
// initialization); exceeding it fails without an approximate result.
// MaxSetComparisons bounds merge comparisons per Compare/ComparePrepared and
// cumulatively across all rows in Scan. Scan counts query plus non-NULL
// document bytes against MaxTotalInputBytes and all
// documents against MaxCandidates. RRF counts every rank entry against
// MaxCandidates and list count against MaxFusionLists. Zero never means unlimited.
// Caller-owned input, context, and callback allocations are outside these bounds.
// No wall-clock or exact allocator-byte bound is claimed.
type Limits struct {
	MaxDistanceRunes   int    `json:"max_distance_runes"`
	MaxDPCells         uint64 `json:"max_dp_cells"`
	MaxSetComparisons  uint64 `json:"max_set_comparisons"`
	MaxCandidates      int    `json:"max_candidates"`
	MaxResults         int    `json:"max_results"`
	MaxTotalInputBytes int64  `json:"max_total_input_bytes"`
	MaxFusionLists     int    `json:"max_fusion_lists"`
}

func DefaultLimits() Limits {
	return Limits{4096, 1 << 24, 1 << 24, 100000, 10000, 64 << 20, 32}
}

// Config selects an inclusive range of gram lengths from 1 through 3. The
// tokenizer still generates and budgets all 1/2/3-grams before selection: this
// intentionally preserves the exact acceptance/config identity of grams13.
// Limits and all tokenizer policies participate in Hash; changed configurations
// cannot accidentally share a prepared Set. Index adapters must separately
// check Tokenizer.Hash and select the same gram lengths from postings.
type Config struct {
	Version   int            `json:"version"`
	Tokenizer grams13.Config `json:"tokenizer"`
	MinGram   int            `json:"min_gram"`
	MaxGram   int            `json:"max_gram"`
	Limits    Limits         `json:"limits"`
}

// DefaultConfig selects unpadded bigrams and case-sensitive NFC MATCH text.
func DefaultConfig() Config {
	return Config{Version, grams13.DefaultMatchConfig(), 2, 2, DefaultLimits()}
}

func (c Config) Validate() error {
	if c.Version != Version || c.MinGram < 1 || c.MaxGram > 3 || c.MinGram > c.MaxGram {
		return ErrConfig
	}
	if err := c.Tokenizer.Validate(); err != nil {
		return err
	}
	l := c.Limits
	if l.MaxDistanceRunes <= 0 || l.MaxDistanceRunes > 1<<20 ||
		l.MaxDPCells == 0 || l.MaxDPCells > 1<<30 ||
		l.MaxSetComparisons == 0 || l.MaxSetComparisons > 1<<30 ||
		l.MaxCandidates <= 0 || l.MaxCandidates > 1<<20 ||
		l.MaxResults <= 0 || l.MaxResults > 1<<20 ||
		l.MaxTotalInputBytes <= 0 || l.MaxTotalInputBytes > 1<<30 ||
		l.MaxFusionLists <= 0 || l.MaxFusionLists > 128 {
		return ErrConfig
	}
	return nil
}

func (c Config) Canonical() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(c)
}

func (c Config) Hash() ([32]byte, error) {
	b, err := c.Canonical()
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(b), nil
}

// Engine is immutable and safe for concurrent use. Construct with New; a nil
// receiver or zero value fails closed. Per-call data and limits are not shared.
type Engine struct {
	config    Config
	tokenizer *grams13.Tokenizer
	identity  [32]byte
}

func New(c Config) (*Engine, error) {
	id, err := c.Hash()
	if err != nil {
		return nil, err
	}
	t, err := grams13.NewTokenizer(c.Tokenizer)
	if err != nil {
		return nil, err
	}
	return &Engine{c, t, id}, nil
}

func (e *Engine) Config() Config {
	if e == nil {
		return Config{}
	}
	return e.config
}

func checkContext(ctx context.Context) error {
	if ctx == nil {
		return ErrContext
	}
	return ctx.Err()
}

func (e *Engine) check(ctx context.Context) error {
	if err := checkContext(ctx); err != nil {
		return err
	}
	if e == nil || e.tokenizer == nil {
		return ErrConfig
	}
	return nil
}

func limitError(name string) error { return fmt.Errorf("%w: %s", ErrLimit, name) }

func validateMetric(m Metric) error {
	if m != Dice && m != Jaccard {
		return fmt.Errorf("%w: metric", ErrValue)
	}
	return nil
}

func validateThreshold(t float64) error {
	if math.IsNaN(t) || math.IsInf(t, 0) || t < 0 || t > 1 {
		return fmt.Errorf("%w: threshold must be finite in [0,1]", ErrValue)
	}
	return nil
}
