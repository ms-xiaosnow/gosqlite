package vector

import (
	"database/sql/driver"
	"encoding/binary"
	"fmt"
)

const (
	HalfBinaryVersion    = 1
	HalfBinaryHeaderSize = 12
	MaxHalfBinaryBytes   = HalfBinaryHeaderSize + 2*MaxHalfDimensions
)

// MarshalBinary encodes GSH\x00, version=1, format=2 (binary16), endian=1
// (big-endian), flags=0, uint32 dimensions, then exactly dimension uint16 bits.
// This project-specific canonical format is not PostgreSQL wire format.
func (v HalfVector) MarshalBinary() ([]byte, error) {
	if !v.Valid() {
		return nil, ErrInvalid
	}
	b := make([]byte, HalfBinaryHeaderSize+2*v.Dims())
	copy(b, "GSH\x00")
	b[4], b[5], b[6] = HalfBinaryVersion, 2, 1
	binary.BigEndian.PutUint32(b[8:12], uint32(v.Dims()))
	for i, h := range v.bits {
		binary.BigEndian.PutUint16(b[12+2*i:], h)
	}
	return b, nil
}

func DecodeHalf(b []byte) (HalfVector, error) {
	if len(b) < HalfBinaryHeaderSize || len(b) > MaxHalfBinaryBytes {
		return HalfVector{}, ErrEncoding
	}
	if string(b[:4]) != "GSH\x00" || b[4] != HalfBinaryVersion || b[5] != 2 || b[6] != 1 || b[7] != 0 {
		return HalfVector{}, ErrEncoding
	}
	n := binary.BigEndian.Uint32(b[8:12])
	if n < 1 || n > MaxHalfDimensions {
		return HalfVector{}, fmt.Errorf("%w: %w", ErrEncoding, ErrDimensions)
	}
	if len(b) != HalfBinaryHeaderSize+2*int(n) {
		return HalfVector{}, ErrEncoding
	}
	x := make([]uint16, int(n))
	for i := range x {
		h := binary.BigEndian.Uint16(b[12+2*i:])
		if h&0x7c00 == 0x7c00 {
			return HalfVector{}, fmt.Errorf("%w: %w", ErrEncoding, ErrNonFinite)
		}
		x[i] = h
	}
	return HalfVector{bits: x}, nil
}

func (v HalfVector) Value() (driver.Value, error) { return v.MarshalBinary() }

// NullHalfVector is the nullable, mutable Scanner/Valuer counterpart. Failed
// scans preserve both fields; binary []byte is never guessed to be text.
type NullHalfVector struct {
	HalfVector HalfVector
	Valid      bool
}

func (n *NullHalfVector) Scan(src any) error {
	if n == nil {
		return fmt.Errorf("%w: nil scan destination", ErrInvalid)
	}
	if src == nil {
		*n = NullHalfVector{}
		return nil
	}
	var v HalfVector
	var err error
	switch x := src.(type) {
	case string:
		v, err = ParseHalf(x)
	case []byte:
		v, err = DecodeHalf(x)
	default:
		return fmt.Errorf("vector: cannot scan %T into half vector", src)
	}
	if err != nil {
		return err
	}
	*n = NullHalfVector{HalfVector: v, Valid: true}
	return nil
}

func (n NullHalfVector) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.HalfVector.Value()
}

var _ driver.Valuer = HalfVector{}
var _ driver.Valuer = NullHalfVector{}
var _ interface{ Scan(any) error } = (*NullHalfVector)(nil)
