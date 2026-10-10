package vector

import "fmt"

// MaxBitDimensions is the local packed-bit storage and exact-distance limit.
// Native values accept zero bits; SQL BIT(n) declarations require n >= 1.
const MaxBitDimensions = 64000

// BitVector is an immutable, most-significant-bit-first sequence of 0..64000
// bits. Unused low bits in its last byte are always zero. Constructors and
// accessors copy storage, making successfully constructed values safe to share.
// The zero Go value is invalid, distinct from a constructed empty sequence.
// This type does not implement PostgreSQL varbit casts or width coercion.
type BitVector struct {
	packed      []byte
	dims        int
	initialized bool
}

// NewBit validates and copies an exactly sized, canonically padded byte slice.
// Wrong lengths and nonzero unused low bits return ErrEncoding; dimensions
// outside 0..MaxBitDimensions return ErrDimensions. It never truncates or pads.
func NewBit(dims int, packed []byte) (BitVector, error) {
	if err := checkBitDimensions(dims); err != nil {
		return BitVector{}, err
	}
	if len(packed) != (dims+7)/8 || !canonicalBitPadding(dims, packed) {
		return BitVector{}, ErrEncoding
	}
	return BitVector{packed: append([]byte(nil), packed...), dims: dims, initialized: true}, nil
}

func checkBitDimensions(n int) error {
	if n < 0 || n > MaxBitDimensions {
		return fmt.Errorf("%w: got %d, want 0..%d", ErrDimensions, n, MaxBitDimensions)
	}
	return nil
}

// canonicalBitPadding requires an already checked dimension and byte length.
func canonicalBitPadding(dims int, packed []byte) bool {
	if remainder := dims % 8; remainder != 0 {
		mask := byte(1<<(8-remainder)) - 1
		return packed[len(packed)-1]&mask == 0
	}
	return true
}

// Valid reports whether v was successfully constructed, including an empty v.
func (v BitVector) Valid() bool { return v.initialized }

// Dims returns the bit length, or zero for an invalid value. Use Valid to
// distinguish a constructed empty sequence from an invalid value.
func (v BitVector) Dims() int { return v.dims }

// Packed returns an independent copy of the MSB-first bytes. An empty sequence
// and an invalid value both return nil; neither exposes the private storage.
func (v BitVector) Packed() []byte { return append([]byte(nil), v.packed...) }

// RequireDims checks the exact native bit length, accepting n == 0. It does not
// implement the separate, positive dimension rule for SQL BIT(n) declarations.
func (v BitVector) RequireDims(n int) error {
	if err := checkBitDimensions(n); err != nil {
		return err
	}
	if !v.Valid() {
		return ErrInvalid
	}
	if v.dims != n {
		return fmt.Errorf("%w: got %d, want %d", ErrDimensions, v.dims, n)
	}
	return nil
}

// String returns exactly Dims ASCII 0/1 bytes, including an empty string for a
// constructed empty value. An invalid value returns "<invalid bit vector>".
func (v BitVector) String() string {
	if !v.Valid() {
		return "<invalid bit vector>"
	}
	b := make([]byte, v.dims)
	for i := range b {
		b[i] = '0' + (v.packed[i/8]>>uint(7-i%8))&1
	}
	return string(b)
}

func (v BitVector) MarshalText() ([]byte, error) {
	if !v.Valid() {
		return nil, ErrInvalid
	}
	return []byte(v.String()), nil
}

// ParseBit accepts only ASCII 0/1, with no prefixes, quotes, or whitespace.
// The empty string is valid. Input length is bounded by MaxBitDimensions, and
// errors do not echo input contents or return partially constructed values.
func ParseBit(text string) (BitVector, error) {
	if err := checkBitDimensions(len(text)); err != nil {
		return BitVector{}, err
	}
	packed := make([]byte, (len(text)+7)/8)
	for i := 0; i < len(text); i++ {
		switch text[i] {
		case '0':
		case '1':
			packed[i/8] |= 1 << uint(7-i%8)
		default:
			return BitVector{}, ErrSyntax
		}
	}
	return BitVector{packed: packed, dims: len(text), initialized: true}, nil
}

// ParseBitDims parses text and requires its exact bit length to equal n.
// Invalid n fails before parsing. Native n == 0 is accepted without coercion.
func ParseBitDims(text string, n int) (BitVector, error) {
	if err := checkBitDimensions(n); err != nil {
		return BitVector{}, err
	}
	v, err := ParseBit(text)
	if err != nil {
		return BitVector{}, err
	}
	if err := v.RequireDims(n); err != nil {
		return BitVector{}, err
	}
	return v, nil
}
