package vector

import (
	"database/sql/driver"
	"encoding/binary"
	"fmt"
)

const (
	BitBinaryVersion    = 1
	BitBinaryHeaderSize = 12
	MaxBitBinaryBytes   = BitBinaryHeaderSize + (MaxBitDimensions+7)/8
)

// MarshalBinary returns independent bytes in the local packed-bit v1 format:
//
//	bytes 0..3:  "GSB\x00" magic
//	byte 4:      version 1
//	byte 5:      component format 3 = packed bits
//	byte 6:      byte order 1 = big-endian, MSB-first bits
//	byte 7:      reserved flags, zero
//	bytes 8..11: unsigned big-endian bit length (0..64000)
//	remaining:  exactly ceil(bit length / 8) bytes, zero low padding bits
//
// Empty values encode to the header alone. This is not PostgreSQL's wire or
// internal bit format and supplies no checksum or authentication.
func (v BitVector) MarshalBinary() ([]byte, error) {
	if !v.Valid() {
		return nil, ErrInvalid
	}
	b := make([]byte, BitBinaryHeaderSize+len(v.packed))
	copy(b, "GSB\x00")
	b[4], b[5], b[6] = BitBinaryVersion, 3, 1
	binary.BigEndian.PutUint32(b[8:12], uint32(v.dims))
	copy(b[BitBinaryHeaderSize:], v.packed)
	return b, nil
}

// DecodeBit checks the complete header, dimension, exact length, and padding
// before copying the bounded payload. Unknown formats, flags, versions, and
// noncanonical encodings fail with ErrEncoding and an invalid result.
func DecodeBit(b []byte) (BitVector, error) {
	if len(b) < BitBinaryHeaderSize || len(b) > MaxBitBinaryBytes {
		return BitVector{}, ErrEncoding
	}
	if string(b[:4]) != "GSB\x00" || b[4] != BitBinaryVersion || b[5] != 3 || b[6] != 1 || b[7] != 0 {
		return BitVector{}, ErrEncoding
	}
	n := binary.BigEndian.Uint32(b[8:12])
	if n > MaxBitDimensions {
		return BitVector{}, fmt.Errorf("%w: %w", ErrEncoding, ErrDimensions)
	}
	dims := int(n)
	if len(b) != BitBinaryHeaderSize+(dims+7)/8 {
		return BitVector{}, ErrEncoding
	}
	if !canonicalBitPadding(dims, b[BitBinaryHeaderSize:]) {
		return BitVector{}, ErrEncoding
	}
	return BitVector{packed: append([]byte(nil), b[BitBinaryHeaderSize:]...), dims: dims, initialized: true}, nil
}

// Value binds the versioned BLOB, including the non-NULL empty sequence. Use
// nil or a NullBitVector with Valid false for SQL NULL.
func (v BitVector) Value() (driver.Value, error) { return v.MarshalBinary() }

// NullBitVector is a mutable database/sql scan destination and nullable binding
// adapter. Like sql.Scanner destinations generally, it requires caller
// synchronization when mutated concurrently. Its BitVector remains immutable.
type NullBitVector struct {
	BitVector BitVector
	Valid     bool
}

// Scan accepts nil, ASCII bit text as string, or a versioned binary []byte.
// Byte slices are never guessed to be text or retained. Errors leave both
// fields unchanged; a nil source resets both fields.
func (n *NullBitVector) Scan(src any) error {
	if n == nil {
		return fmt.Errorf("%w: nil scan destination", ErrInvalid)
	}
	if src == nil {
		*n = NullBitVector{}
		return nil
	}
	var v BitVector
	var err error
	switch x := src.(type) {
	case string:
		v, err = ParseBit(x)
	case []byte:
		v, err = DecodeBit(x)
	default:
		return fmt.Errorf("vector: cannot scan %T into bit vector", src)
	}
	if err != nil {
		return err
	}
	*n = NullBitVector{BitVector: v, Valid: true}
	return nil
}

func (n NullBitVector) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.BitVector.Value()
}

var _ driver.Valuer = BitVector{}
var _ driver.Valuer = NullBitVector{}
var _ interface{ Scan(any) error } = (*NullBitVector)(nil)
