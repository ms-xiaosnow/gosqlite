package vector

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

const (
	MaxSparseDimensions = 1000000000
	MaxSparseNonzeros   = 16000
)

var (
	ErrSparseIndex     = errors.New("vector: sparse indices must be unique and in range")
	ErrSparseUnderflow = errors.New("vector: sparse decimal underflows binary32 to zero")
)

// SparseEntry uses a zero-based native index. Text uses one-based indices.
// NewSparse copies, sorts, and removes zeros; binary decoding never repairs data.
type SparseEntry struct {
	Index uint32
	Value float32
}

// SparseVector is immutable and contains sorted unique finite nonzero entries.
// A valid all-zero vector retains its dimension with no entries. The zero Go
// object is invalid. Work and storage scale with nonzeros, not dimension.
type SparseVector struct {
	dim     int
	entries []SparseEntry
}

func checkSparseDimensions(n int) error {
	if n < 1 || n > MaxSparseDimensions {
		return ErrDimensions
	}
	return nil
}

// NewSparse validates all indices, including explicit zero entries, then drops
// zeros, sorts a defensive copy, and rejects duplicate retained indices. At
// most MaxSparseNonzeros input entries are accepted, including explicit zeros.
// Checking zero indices is intentionally stricter than upstream sparse text.
func NewSparse(dimension int, entries []SparseEntry) (SparseVector, error) {
	if err := checkSparseDimensions(dimension); err != nil {
		return SparseVector{}, err
	}
	if len(entries) > MaxSparseNonzeros {
		return SparseVector{}, ErrLimit
	}
	nnz := 0
	for _, e := range entries {
		if e.Index >= uint32(dimension) {
			return SparseVector{}, ErrSparseIndex
		}
		if !finite32(e.Value) {
			return SparseVector{}, ErrNonFinite
		}
		if e.Value != 0 {
			nnz++
		}
	}
	x := make([]SparseEntry, 0, nnz)
	for _, e := range entries {
		if e.Value != 0 {
			x = append(x, e)
		}
	}
	sort.Slice(x, func(i, j int) bool { return x[i].Index < x[j].Index })
	for i := 1; i < len(x); i++ {
		if x[i-1].Index == x[i].Index {
			return SparseVector{}, ErrSparseIndex
		}
	}
	return SparseVector{dim: dimension, entries: x}, nil
}

func (v SparseVector) Valid() bool {
	return v.dim >= 1 && v.dim <= MaxSparseDimensions && len(v.entries) <= MaxSparseNonzeros
}
func (v SparseVector) Dims() int              { return v.dim }
func (v SparseVector) NNZ() int               { return len(v.entries) }
func (v SparseVector) Entries() []SparseEntry { return append([]SparseEntry(nil), v.entries...) }

func (v SparseVector) RequireDims(n int) error {
	if err := checkSparseDimensions(n); err != nil {
		return err
	}
	if !v.Valid() {
		return ErrInvalid
	}
	if n != v.dim {
		return ErrDimensions
	}
	return nil
}

func (v SparseVector) String() string {
	if !v.Valid() {
		return "<invalid sparse vector>"
	}
	b := make([]byte, 0, 16+len(v.entries)*20)
	b = append(b, '{')
	for i, e := range v.entries {
		if i != 0 {
			b = append(b, ',')
		}
		b = strconv.AppendUint(b, uint64(e.Index)+1, 10)
		b = append(b, ':')
		b = strconv.AppendFloat(b, float64(e.Value), 'g', -1, 32)
	}
	b = append(b, '}', '/')
	return string(strconv.AppendInt(b, int64(v.dim), 10))
}

func (v SparseVector) MarshalText() ([]byte, error) {
	if !v.Valid() {
		return nil, ErrInvalid
	}
	return []byte(v.String()), nil
}

// ParseSparse parses bounded {one-based-index:decimal,...}/dimension text.
// Decimal values round directly to binary32. As in the pinned reference, a
// mathematically nonzero decimal which rounds to binary32 zero is rejected;
// representable subnormals survive. Explicit +/-0 entries are removed before
// duplicate checks, but every index must be in range even when its value is 0.
func ParseSparse(text string) (SparseVector, error) {
	if len(text) > MaxTextBytes {
		return SparseVector{}, ErrLimit
	}
	i := 0
	skip := func() {
		for i < len(text) && asciiSpace(text[i]) {
			i++
		}
	}
	integer := func() (int64, error) {
		start := i
		if i < len(text) && (text[i] == '+' || text[i] == '-') {
			i++
		}
		digits := i
		for i < len(text) && text[i] >= '0' && text[i] <= '9' {
			i++
			if i-start > MaxElementTextBytes {
				return 0, ErrLimit
			}
		}
		if i == digits {
			return 0, ErrSyntax
		}
		n, err := strconv.ParseInt(text[start:i], 10, 64)
		if err != nil {
			return 0, ErrDimensions
		}
		return n, nil
	}
	skip()
	if i == len(text) || text[i] != '{' {
		return SparseVector{}, ErrSyntax
	}
	i++
	skip()
	entries := make([]SparseEntry, 0, 8)
	if i < len(text) && text[i] == '}' {
		i++
	} else {
		for {
			if len(entries) == MaxSparseNonzeros {
				return SparseVector{}, ErrLimit
			}
			skip()
			index, err := integer()
			if err != nil {
				return SparseVector{}, err
			}
			if index < 1 || index > MaxSparseDimensions {
				return SparseVector{}, ErrSparseIndex
			}
			skip()
			if i == len(text) || text[i] != ':' {
				return SparseVector{}, ErrSyntax
			}
			i++
			skip()
			start := i
			for i < len(text) && text[i] != ',' && text[i] != '}' && !asciiSpace(text[i]) {
				i++
				if i-start > MaxElementTextBytes {
					return SparseVector{}, ErrLimit
				}
			}
			token := text[start:i]
			if !decimal(token) {
				switch strings.ToLower(token) {
				case "nan", "+nan", "-nan", "inf", "+inf", "-inf", "infinity", "+infinity", "-infinity":
					return SparseVector{}, ErrNonFinite
				}
				return SparseVector{}, ErrSyntax
			}
			x, err := strconv.ParseFloat(token, 32)
			if err != nil || math.IsNaN(x) || math.IsInf(x, 0) {
				return SparseVector{}, ErrNonFinite
			}
			if x == 0 && sparseDecimalNonzero(token) {
				return SparseVector{}, ErrSparseUnderflow
			}
			entries = append(entries, SparseEntry{Index: uint32(index - 1), Value: float32(x)})
			skip()
			if i == len(text) {
				return SparseVector{}, ErrSyntax
			}
			if text[i] == '}' {
				i++
				break
			}
			if text[i] != ',' {
				return SparseVector{}, ErrSyntax
			}
			i++
		}
	}
	skip()
	if i == len(text) || text[i] != '/' {
		return SparseVector{}, ErrSyntax
	}
	i++
	skip()
	dim, err := integer()
	if err != nil {
		return SparseVector{}, err
	}
	if dim < 1 || dim > MaxSparseDimensions {
		return SparseVector{}, ErrDimensions
	}
	skip()
	if i != len(text) {
		return SparseVector{}, ErrSyntax
	}
	return NewSparse(int(dim), entries)
}

func sparseDecimalNonzero(token string) bool {
	for i := 0; i < len(token); i++ {
		if token[i] == 'e' || token[i] == 'E' {
			break
		}
		if token[i] >= '1' && token[i] <= '9' {
			return true
		}
	}
	return false
}

func ParseSparseDims(text string, n int) (SparseVector, error) {
	if err := checkSparseDimensions(n); err != nil {
		return SparseVector{}, err
	}
	v, err := ParseSparse(text)
	if err != nil {
		return SparseVector{}, err
	}
	if err := v.RequireDims(n); err != nil {
		return SparseVector{}, err
	}
	return v, nil
}

// SparseFromVector explicitly removes dense signed zeros while preserving all
// nonzero binary32 bits. Sparse representation cannot retain the sign of zero.
func SparseFromVector(v Vector) (SparseVector, error) {
	if !v.Valid() {
		return SparseVector{}, ErrInvalid
	}
	nnz := 0
	for _, x := range v.values {
		if x != 0 {
			nnz++
		}
	}
	x := make([]SparseEntry, 0, nnz)
	for i, value := range v.values {
		if value != 0 {
			x = append(x, SparseEntry{Index: uint32(i), Value: value})
		}
	}
	return SparseVector{dim: v.Dims(), entries: x}, nil
}

// Dense materializes an explicit dense conversion only if the output fits the
// existing dense dimension cap. The dimension is checked before allocation;
// a billion-dimensional sparse value never triggers a billion-element slice.
func (v SparseVector) Dense() (Vector, error) {
	if !v.Valid() {
		return Vector{}, ErrInvalid
	}
	if v.dim > MaxDimensions {
		return Vector{}, fmt.Errorf("%w: dense output dimensions", ErrLimit)
	}
	x := make([]float32, v.dim)
	for _, e := range v.entries {
		x[int(e.Index)] = e.Value
	}
	return Vector{values: x}, nil
}
