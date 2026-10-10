// Package vector implements immutable, validated dense float32 vectors and
// bounded exact search. It performs no database I/O and supplies no ANN index.
//
// The semantic reference is pgvector v0.8.7 (commit
// f37c13f68b57d2c3472b2214fbcff699d6d34876). Dimensions are 1..16000; the upstream
// ANN dense-index limit of 2000 is separate and does not restrict exact search.
// NULL is represented by NullVector or a Row's Null flag, never an empty vector.
// The zero Go Vector is invalid. An all-zero vector of valid dimension is valid,
// but cosine distance involving it returns ErrZeroNorm (upstream returns NaN).
//
// Metrics widen components to float64 before arithmetic and use compensated
// sums. All supported finite float32 inputs therefore produce finite results,
// except undefined cosine on zero norms, which fails explicitly. This differs
// from upstream's float32 accumulations and possible architecture-dependent FMA,
// overflow, and underflow. Semantic compatibility is not bit-exact arithmetic
// compatibility. Cosine is clamped to [0,2].
//
// Parse accepts bracketed comma-separated signed decimal numbers, optional
// decimal points/exponents, and ASCII whitespace. It rounds directly to
// float32; decimal underflow to signed zero is accepted. Hexadecimal, Go digit
// separators, quoted numbers, NaN, and infinities are rejected. Input bytes and
// individual number lengths are bounded, unlike upstream's unbounded text
// input. String uses Go's shortest float32-round-tripping decimal formatting;
// PostgreSQL's formatting thresholds may differ. Binary storage has its own
// versioned big-endian format, not PostgreSQL's wire or internal vector format.
// See codec.go and testdata/pgvector-v0.8.7 for the compatibility boundaries.
package vector

import (
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	// MaxDimensions is the dense type limit in pgvector v0.8.7.
	MaxDimensions = 16000
	// ANNMaxDimensions records the upstream dense ANN limit only. This package
	// implements no ANN index, and exact operations accept MaxDimensions.
	ANNMaxDimensions    = 2000
	MaxTextBytes        = 1 << 20
	MaxElementTextBytes = 128
)

var (
	ErrInvalid        = errors.New("vector: invalid zero value")
	ErrDimensions     = errors.New("vector: invalid or mismatched dimensions")
	ErrNonFinite      = errors.New("vector: non-finite component or result")
	ErrSyntax         = errors.New("vector: invalid text syntax")
	ErrEncoding       = errors.New("vector: invalid binary encoding")
	ErrLimit          = errors.New("vector: resource limit exceeded")
	ErrZeroNorm       = errors.New("vector: cosine distance requires nonzero norms")
	ErrMetric         = errors.New("vector: invalid metric")
	ErrContext        = errors.New("vector: nil context")
	ErrOptions        = errors.New("vector: invalid search options")
	ErrDuplicateRowID = errors.New("vector: duplicate rowid")
)

// Vector contains 1..MaxDimensions finite float32 components. Construction and
// access copy slices, so a constructed value is immutable and safe to share
// concurrently. Its zero value is invalid, not NULL and not a zero-norm vector.
type Vector struct{ values []float32 }

// New validates and copies values, preserving their exact float32 bits.
// A nil or empty slice is invalid rather than SQL NULL.
func New(values []float32) (Vector, error) {
	if err := checkDimensions(len(values)); err != nil {
		return Vector{}, err
	}
	for i, v := range values {
		if !finite32(v) {
			return Vector{}, fmt.Errorf("%w at component %d", ErrNonFinite, i)
		}
	}
	return Vector{values: append([]float32(nil), values...)}, nil
}

func checkDimensions(n int) error {
	if n < 1 || n > MaxDimensions {
		return fmt.Errorf("%w: got %d, want 1..%d", ErrDimensions, n, MaxDimensions)
	}
	return nil
}

func finite32(v float32) bool { return math.Float32bits(v)&0x7f800000 != 0x7f800000 }

// Valid reports whether v was successfully constructed. The private storage is
// never exposed or mutated, so validation need not re-scan components.
func (v Vector) Valid() bool { return len(v.values) >= 1 && len(v.values) <= MaxDimensions }

// Dims returns the number of components, or zero for an invalid Vector.
func (v Vector) Dims() int { return len(v.values) }

// Values returns a defensive copy, or nil for an invalid Vector.
func (v Vector) Values() []float32 { return append([]float32(nil), v.values...) }

// RequireDims verifies a declared vector(n) dimension. It does not change v.
func (v Vector) RequireDims(n int) error {
	if err := checkDimensions(n); err != nil {
		return err
	}
	if !v.Valid() {
		return ErrInvalid
	}
	if v.Dims() != n {
		return fmt.Errorf("%w: got %d, want %d", ErrDimensions, v.Dims(), n)
	}
	return nil
}

// String returns a shortest-round-trip bracketed decimal representation. For
// an invalid zero Vector it returns "<invalid vector>", never a valid literal.
func (v Vector) String() string {
	if !v.Valid() {
		return "<invalid vector>"
	}
	b := make([]byte, 0, 2+len(v.values)*8)
	b = append(b, '[')
	for i, x := range v.values {
		if i != 0 {
			b = append(b, ',')
		}
		b = strconv.AppendFloat(b, float64(x), 'g', -1, 32)
	}
	b = append(b, ']')
	return string(b)
}

func (v Vector) MarshalText() ([]byte, error) {
	if !v.Valid() {
		return nil, ErrInvalid
	}
	return []byte(v.String()), nil
}

// ParseDims parses text and verifies the explicit dimension n. Invalid n is
// rejected before parsing; use Parse for an unspecified dimension.
func ParseDims(text string, n int) (Vector, error) {
	if err := checkDimensions(n); err != nil {
		return Vector{}, err
	}
	v, err := Parse(text)
	if err != nil {
		return Vector{}, err
	}
	if err := v.RequireDims(n); err != nil {
		return Vector{}, err
	}
	return v, nil
}

func asciiSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\v' || c == '\f'
}

// Parse parses at most MaxTextBytes and at most MaxElementTextBytes per number.
// It returns no partial value on error. Errors never echo input contents.
func Parse(text string) (Vector, error) {
	if len(text) > MaxTextBytes {
		return Vector{}, fmt.Errorf("%w: text bytes", ErrLimit)
	}
	i := 0
	skip := func() {
		for i < len(text) && asciiSpace(text[i]) {
			i++
		}
	}
	skip()
	if i == len(text) || text[i] != '[' {
		return Vector{}, ErrSyntax
	}
	i++
	skip()
	if i < len(text) && text[i] == ']' {
		return Vector{}, ErrDimensions
	}
	values := make([]float32, 0, 8)
	for {
		if len(values) == MaxDimensions {
			return Vector{}, ErrDimensions
		}
		skip()
		start := i
		for i < len(text) && text[i] != ',' && text[i] != ']' && !asciiSpace(text[i]) {
			i++
			if i-start > MaxElementTextBytes {
				return Vector{}, fmt.Errorf("%w: number bytes", ErrLimit)
			}
		}
		token := text[start:i]
		if !decimal(token) {
			switch strings.ToLower(token) {
			case "nan", "+nan", "-nan", "inf", "+inf", "-inf", "infinity", "+infinity", "-infinity":
				return Vector{}, ErrNonFinite
			}
			return Vector{}, ErrSyntax
		}
		// bitSize=32 rounds once to float32 instead of double-rounding via 64.
		x, err := strconv.ParseFloat(token, 32)
		if err != nil || math.IsInf(x, 0) || math.IsNaN(x) {
			return Vector{}, ErrNonFinite
		}
		values = append(values, float32(x))
		skip()
		if i == len(text) {
			return Vector{}, ErrSyntax
		}
		switch text[i] {
		case ',':
			i++
		case ']':
			i++
			skip()
			if i != len(text) {
				return Vector{}, ErrSyntax
			}
			return Vector{values: values}, nil
		default:
			return Vector{}, ErrSyntax
		}
	}
}

// decimal deliberately excludes ParseFloat's hexadecimal and underscore forms.
func decimal(s string) bool {
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	digits := 0
	for i < len(s) && s[i] >= '0' && s[i] <= '9' {
		i++
		digits++
	}
	if i < len(s) && s[i] == '.' {
		i++
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
			digits++
		}
	}
	if digits == 0 {
		return false
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		start := i
		for i < len(s) && s[i] >= '0' && s[i] <= '9' {
			i++
		}
		if i == start {
			return false
		}
	}
	return i == len(s)
}
