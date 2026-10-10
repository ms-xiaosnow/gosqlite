package vector

import (
	"database/sql/driver"
	"encoding/binary"
	"fmt"
	"math"
)

const (
	BinaryVersion    = 1
	BinaryHeaderSize = 12
	MaxBinaryBytes   = BinaryHeaderSize + 4*MaxDimensions
)

// MarshalBinary encodes v into a new independent BLOB. The stable v1 format is:
//
//	bytes 0..3:  "GSV\x00" magic
//	byte 4:      version 1
//	byte 5:      component format 1 = IEEE-754 binary32
//	byte 6:      byte order 1 = big-endian
//	byte 7:      reserved flags, zero
//	bytes 8..11: unsigned big-endian dimension (1..16000)
//	remaining:  exactly dimension big-endian uint32 float bit patterns
//
// Signed zero and subnormal bits are preserved. There is no alignment padding,
// checksum, or authentication; this is not PostgreSQL's binary wire format.
func (v Vector) MarshalBinary() ([]byte, error) {
	if !v.Valid() {
		return nil, ErrInvalid
	}
	b := make([]byte, BinaryHeaderSize+4*v.Dims())
	copy(b, "GSV\x00")
	b[4], b[5], b[6] = BinaryVersion, 1, 1
	binary.BigEndian.PutUint32(b[8:12], uint32(v.Dims()))
	for i, x := range v.values {
		binary.BigEndian.PutUint32(b[12+4*i:], math.Float32bits(x))
	}
	return b, nil
}

// Decode validates the entire header and exact length before allocating at most
// MaxDimensions components. Unknown versions, formats, byte order, flags,
// trailing bytes, and non-finite components fail closed; no partial v is returned.
func Decode(b []byte) (Vector, error) {
	if len(b) < BinaryHeaderSize || len(b) > MaxBinaryBytes {
		return Vector{}, ErrEncoding
	}
	if string(b[:4]) != "GSV\x00" || b[4] != BinaryVersion || b[5] != 1 || b[6] != 1 || b[7] != 0 {
		return Vector{}, ErrEncoding
	}
	n := binary.BigEndian.Uint32(b[8:12])
	if n < 1 || n > MaxDimensions {
		return Vector{}, fmt.Errorf("%w: %w", ErrEncoding, ErrDimensions)
	}
	if len(b) != BinaryHeaderSize+4*int(n) {
		return Vector{}, ErrEncoding
	}
	values := make([]float32, int(n))
	for i := range values {
		x := math.Float32frombits(binary.BigEndian.Uint32(b[12+4*i:]))
		if !finite32(x) {
			return Vector{}, fmt.Errorf("%w: %w at component %d", ErrEncoding, ErrNonFinite, i)
		}
		values[i] = x
	}
	return Vector{values: values}, nil
}

// Value implements driver.Valuer using the versioned BLOB representation. A
// zero Vector returns ErrInvalid. Bind SQL NULL with nil or a NullVector whose
// Valid is false; bind a []float32 through New rather than relying on driver
// reflection or a text/BLOB heuristic.
func (v Vector) Value() (driver.Value, error) { return v.MarshalBinary() }

// NullVector is the mutable database/sql scan destination and nullable binding
// adapter. Vector itself remains immutable. Like other sql.Scanner destinations,
// a NullVector must not be mutated concurrently without caller synchronization.
// When Valid is false, Vector is ignored by Value. A NULL scan resets both fields.
type NullVector struct {
	Vector Vector
	Valid  bool
}

// Scan accepts nil, a textual string, or a versioned binary []byte. []byte is
// always binary and never guessed to be text. On error the receiver is unchanged.
// Driver-owned byte buffers are copied by Decode and never retained.
func (n *NullVector) Scan(src any) error {
	if n == nil {
		return fmt.Errorf("%w: nil scan destination", ErrInvalid)
	}
	if src == nil {
		*n = NullVector{}
		return nil
	}
	var v Vector
	var err error
	switch x := src.(type) {
	case string:
		v, err = Parse(x)
	case []byte:
		v, err = Decode(x)
	default:
		return fmt.Errorf("vector: cannot scan %T", src)
	}
	if err != nil {
		return err
	}
	*n = NullVector{Vector: v, Valid: true}
	return nil
}

func (n NullVector) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.Vector.Value()
}

var _ driver.Valuer = Vector{}
var _ driver.Valuer = NullVector{}
var _ interface{ Scan(any) error } = (*NullVector)(nil)
