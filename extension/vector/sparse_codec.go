package vector

import (
	"database/sql/driver"
	"encoding/binary"
	"fmt"
	"math"
)

const (
	SparseBinaryVersion    = 1
	SparseBinaryHeaderSize = 16
	MaxSparseBinaryBytes   = SparseBinaryHeaderSize + 8*MaxSparseNonzeros
)

// MarshalBinary encodes GSS\x00, version=1, format=4 (sparse binary32),
// endian=1, flags=0, uint32 dimension, uint32 nnz, then nnz interleaved
// (uint32 zero-based index, binary32 bits) pairs, all big-endian. There is no
// padding, checksum or authentication. This is not PostgreSQL wire format.
func (v SparseVector) MarshalBinary() ([]byte, error) {
	if !v.Valid() {
		return nil, ErrInvalid
	}
	b := make([]byte, SparseBinaryHeaderSize+8*v.NNZ())
	copy(b, "GSS\x00")
	b[4], b[5], b[6] = SparseBinaryVersion, 4, 1
	binary.BigEndian.PutUint32(b[8:12], uint32(v.dim))
	binary.BigEndian.PutUint32(b[12:16], uint32(v.NNZ()))
	for i, e := range v.entries {
		binary.BigEndian.PutUint32(b[16+8*i:], e.Index)
		binary.BigEndian.PutUint32(b[20+8*i:], math.Float32bits(e.Value))
	}
	return b, nil
}

// DecodeSparse requires canonical sorted, unique, nonzero, finite entries. It
// checks all lengths/counts before allocating and never sorts or drops data.
func DecodeSparse(b []byte) (SparseVector, error) {
	if len(b) < SparseBinaryHeaderSize || len(b) > MaxSparseBinaryBytes {
		return SparseVector{}, ErrEncoding
	}
	if string(b[:4]) != "GSS\x00" || b[4] != SparseBinaryVersion || b[5] != 4 || b[6] != 1 || b[7] != 0 {
		return SparseVector{}, ErrEncoding
	}
	n, nnz := binary.BigEndian.Uint32(b[8:12]), binary.BigEndian.Uint32(b[12:16])
	if n < 1 || n > MaxSparseDimensions {
		return SparseVector{}, fmt.Errorf("%w: %w", ErrEncoding, ErrDimensions)
	}
	if nnz > MaxSparseNonzeros || nnz > n || len(b) != SparseBinaryHeaderSize+8*int(nnz) {
		return SparseVector{}, ErrEncoding
	}
	var previous uint32
	for i := 0; i < int(nnz); i++ {
		index := binary.BigEndian.Uint32(b[16+8*i:])
		x := math.Float32frombits(binary.BigEndian.Uint32(b[20+8*i:]))
		if index >= n || (i > 0 && index <= previous) {
			return SparseVector{}, fmt.Errorf("%w: %w", ErrEncoding, ErrSparseIndex)
		}
		if !finite32(x) {
			return SparseVector{}, fmt.Errorf("%w: %w", ErrEncoding, ErrNonFinite)
		}
		if x == 0 {
			return SparseVector{}, ErrEncoding
		}
		previous = index
	}
	x := make([]SparseEntry, int(nnz))
	for i := range x {
		x[i] = SparseEntry{Index: binary.BigEndian.Uint32(b[16+8*i:]), Value: math.Float32frombits(binary.BigEndian.Uint32(b[20+8*i:]))}
	}
	return SparseVector{dim: int(n), entries: x}, nil
}

func (v SparseVector) Value() (driver.Value, error) { return v.MarshalBinary() }

// NullSparseVector is a mutable nullable scan/bind adapter. Failed scans leave
// the receiver unchanged. Scan(nil) resets it; []byte always means binary.
type NullSparseVector struct {
	SparseVector SparseVector
	Valid        bool
}

func (n *NullSparseVector) Scan(src any) error {
	if n == nil {
		return fmt.Errorf("%w: nil scan destination", ErrInvalid)
	}
	if src == nil {
		*n = NullSparseVector{}
		return nil
	}
	var v SparseVector
	var err error
	switch x := src.(type) {
	case string:
		v, err = ParseSparse(x)
	case []byte:
		v, err = DecodeSparse(x)
	default:
		return fmt.Errorf("vector: cannot scan %T into sparse vector", src)
	}
	if err != nil {
		return err
	}
	*n = NullSparseVector{SparseVector: v, Valid: true}
	return nil
}

func (n NullSparseVector) Value() (driver.Value, error) {
	if !n.Valid {
		return nil, nil
	}
	return n.SparseVector.Value()
}

var _ driver.Valuer = SparseVector{}
var _ driver.Valuer = NullSparseVector{}
var _ interface{ Scan(any) error } = (*NullSparseVector)(nil)
