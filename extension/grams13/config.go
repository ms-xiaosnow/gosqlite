// Package grams13 provides versioned, bounded Unicode 1/2/3-gram tokenization.
// It does not implement SQL coercion, a posting store, or positional full-text
// search. Persist Config.Canonical alongside an index and include its hash (or
// an equivalent immutable index identity) in every posting namespace.
package grams13

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"unicode"

	"golang.org/x/text/cases"
	"golang.org/x/text/unicode/norm"
)

// Namespace identifies semantics, not merely a token format. Tokens from these
// namespaces MUST NOT be interchanged, even when their text happens to match.
type Namespace string

const (
	LikeRawV1                   Namespace = "LIKE_RAW_V1"
	MatchConfigV1               Namespace = "MATCH_CONFIG_V1"
	ConfigVersion                         = 1
	DependencyVersion                     = "golang.org/x/text@v0.43.0"
	NormalizationUnicodeVersion           = norm.Version
	CaseFoldUnicodeVersion                = cases.UnicodeVersion
	CategoryUnicodeVersion                = unicode.Version

	NormalizationNone   = "none"
	NormalizationNFC    = "NFC"
	CaseFoldASCII       = "ascii-A-Z"
	CaseFoldNone        = "none"
	CaseFoldUnicode     = "unicode-full-default"
	WhitespacePreserve  = "preserve"
	WhitespaceCollapse  = "unicode-space-collapse"
	PunctuationPreserve = "preserve"
	PunctuationRemove   = "unicode-punct-remove"
	NULTerminate        = "first-NUL"
	NULPreserve         = "preserve"
	StreamSafeNone      = "not-applicable"
	StreamSafeReject    = "reject-inserted-CGJ-v1"
)

var (
	ErrConfig      = errors.New("grams13: invalid or unsupported configuration")
	ErrInvalidUTF8 = errors.New("grams13: invalid UTF-8")
	ErrLimit       = errors.New("grams13: resource limit exceeded")
	ErrStreamSafe  = errors.New("grams13: normalization would insert U+034F")
	ErrContext     = errors.New("grams13: nil context")
)

// Limits are persisted acceptance limits. Byte and rune limits are checked on
// the entire original input (including text after NUL). Output limits apply to
// every materialized intermediate transform and to the final normalized text.
// MaxTokenBytes counts the sum of UTF-8 lengths of distinct token texts.
// No operation returns truncated text or a partial token set on failure.
type Limits struct {
	MaxInputBytes     int `json:"max_input_bytes"`
	MaxOutputBytes    int `json:"max_output_bytes"`
	MaxInputRunes     int `json:"max_input_runes"`
	MaxOutputRunes    int `json:"max_output_runes"`
	MaxDistinctTokens int `json:"max_distinct_tokens"`
	MaxTokenBytes     int `json:"max_token_bytes"`
}

// DefaultLimits permits 1 MiB inputs and normalized output, 262144 runes and
// distinct tokens, and 2 MiB aggregate distinct token bytes.
func DefaultLimits() Limits {
	return Limits{1 << 20, 1 << 20, 1 << 18, 1 << 18, 1 << 18, 2 << 20}
}

// Config is strict: zero values, unknown policies, mismatched data/dependency
// versions, and contradictory namespace settings are rejected. Config changes
// require index rebuild. Algorithms always produce contiguous rune grams of
// lengths 1, 2 and 3; runes are Unicode code points, not grapheme clusters.
//
// MATCH_CONFIG_V1 applies punctuation removal (if selected), then whitespace
// collapse (if selected), NFC, optional default full Unicode folding, and NFC
// again. Collapse replaces each run of unicode.IsSpace characters with one
// ASCII space, including leading/trailing runs. Removal uses unicode.IsPunct;
// symbols, emoji, marks, ZWJ, and CGJ are preserved. MATCH preserves NUL and
// defines matching as containment of the resulting normalized strings, including
// the usual empty-needle match. It has no FTS query operators or positions.
//
// LIKE_RAW_V1 preserves every rune and mark before the first NUL except ASCII
// A-Z folding. It MUST NOT use NFC or Unicode folding: a literal combining-mark
// or base-letter substring can match inside decomposed text under SQL LIKE.
// Normalizing that text would lose those literal grams and create false negatives.
// SQL wildcard/ESCAPE parsing and final original-LIKE verification belong to
// the engine; Contains is not an implementation of SQL LIKE.
type Config struct {
	Version                     int       `json:"version"`
	Namespace                   Namespace `json:"namespace"`
	Normalization               string    `json:"normalization"`
	CaseFold                    string    `json:"case_fold"`
	Whitespace                  string    `json:"whitespace"`
	Punctuation                 string    `json:"punctuation"`
	NUL                         string    `json:"nul"`
	StreamSafe                  string    `json:"stream_safe"`
	NormalizationUnicodeVersion string    `json:"normalization_unicode_version"`
	CaseFoldUnicodeVersion      string    `json:"case_fold_unicode_version"`
	CategoryUnicodeVersion      string    `json:"category_unicode_version"`
	DependencyVersion           string    `json:"dependency_version"`
	Limits                      Limits    `json:"limits"`
}

func baseConfig() Config {
	return Config{
		Version: ConfigVersion, Whitespace: WhitespacePreserve,
		Punctuation:                 PunctuationPreserve,
		NormalizationUnicodeVersion: NormalizationUnicodeVersion,
		CaseFoldUnicodeVersion:      CaseFoldUnicodeVersion,
		CategoryUnicodeVersion:      CategoryUnicodeVersion,
		DependencyVersion:           DependencyVersion, Limits: DefaultLimits(),
	}
}

func DefaultLikeConfig() Config {
	c := baseConfig()
	c.Namespace, c.Normalization, c.CaseFold = LikeRawV1, NormalizationNone, CaseFoldASCII
	c.NUL, c.StreamSafe = NULTerminate, StreamSafeNone
	return c
}

// DefaultMatchConfig uses NFC and preserves case, whitespace, and punctuation.
// Set CaseFold to CaseFoldUnicode to opt in to default full Unicode folding.
func DefaultMatchConfig() Config {
	c := baseConfig()
	c.Namespace, c.Normalization, c.CaseFold = MatchConfigV1, NormalizationNFC, CaseFoldNone
	c.NUL, c.StreamSafe = NULPreserve, StreamSafeReject
	return c
}

func (c Config) Validate() error {
	bad := func(field string) error { return fmt.Errorf("%w: %s", ErrConfig, field) }
	if c.Version != ConfigVersion {
		return bad("version")
	}
	if c.NormalizationUnicodeVersion != NormalizationUnicodeVersion {
		return bad("normalization Unicode version")
	}
	if c.CaseFoldUnicodeVersion != CaseFoldUnicodeVersion {
		return bad("casefold Unicode version")
	}
	if c.CategoryUnicodeVersion != CategoryUnicodeVersion {
		return bad("category Unicode version")
	}
	if c.DependencyVersion != DependencyVersion {
		return bad("dependency version")
	}
	switch c.Namespace {
	case LikeRawV1:
		if c.Normalization != NormalizationNone || c.CaseFold != CaseFoldASCII || c.Whitespace != WhitespacePreserve || c.Punctuation != PunctuationPreserve || c.NUL != NULTerminate || c.StreamSafe != StreamSafeNone {
			return bad("LIKE_RAW_V1 policies")
		}
	case MatchConfigV1:
		if c.Normalization != NormalizationNFC || (c.CaseFold != CaseFoldNone && c.CaseFold != CaseFoldUnicode) || (c.Whitespace != WhitespacePreserve && c.Whitespace != WhitespaceCollapse) || (c.Punctuation != PunctuationPreserve && c.Punctuation != PunctuationRemove) || c.NUL != NULPreserve || c.StreamSafe != StreamSafeReject {
			return bad("MATCH_CONFIG_V1 policies")
		}
	default:
		return bad("namespace")
	}
	// Hard ceilings make even caller-supplied configurations resource bounded.
	for _, v := range []struct {
		name       string
		value, max int
	}{
		{"max_input_bytes", c.Limits.MaxInputBytes, 16 << 20},
		{"max_output_bytes", c.Limits.MaxOutputBytes, 16 << 20},
		{"max_input_runes", c.Limits.MaxInputRunes, 1 << 20},
		{"max_output_runes", c.Limits.MaxOutputRunes, 1 << 20},
		{"max_distinct_tokens", c.Limits.MaxDistinctTokens, 1 << 20},
		{"max_token_bytes", c.Limits.MaxTokenBytes, 12 << 20},
	} {
		if v.value <= 0 || v.value > v.max {
			return bad(v.name)
		}
	}
	return nil
}

// Canonical returns the v1 canonical UTF-8 JSON encoding (fixed field order,
// no insignificant whitespace). Every field, including false/empty equivalents,
// is explicit. Persist these bytes unchanged; later versions must not reinterpret
// this encoding or silently replace stale Unicode tables.
func (c Config) Canonical() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(c)
}

// Hash is SHA-256 of Canonical. All policies, budgets, and Unicode versions are
// part of this identity; the hash is not a security authorization mechanism.
func (c Config) Hash() ([32]byte, error) {
	b, err := c.Canonical()
	if err != nil {
		return [32]byte{}, err
	}
	return sha256.Sum256(b), nil
}

// ParseConfig accepts only canonical encodings, rejecting unknown/duplicate
// fields, missing fields, alternate field order, whitespace, trailing data, and
// unsupported versions. This avoids ambiguous persisted metadata. The limit
// also bounds JSON work before decoding.
func ParseConfig(b []byte) (Config, error) {
	var c Config
	if len(b) > 4096 {
		return c, fmt.Errorf("%w: encoding too large", ErrConfig)
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&c); err != nil {
		return Config{}, fmt.Errorf("%w: JSON: %v", ErrConfig, err)
	}
	canonical, err := c.Canonical()
	if err != nil {
		return Config{}, err
	}
	if !bytes.Equal(b, canonical) {
		return Config{}, fmt.Errorf("%w: noncanonical encoding", ErrConfig)
	}
	return c, nil
}
