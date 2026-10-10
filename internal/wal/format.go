// Package wal implements SQLite's public write-ahead-log wire format and a
// bounded, read-only committed-page index. It does not implement locking,
// shared-memory coordination, recovery writes, checkpoints, or durability.
//
// The format follows SQLite 3.54.0 src/wal.c and
// https://www.sqlite.org/fileformat2.html#wal_file_format. All stored integers
// are big-endian; only the checksum input word order varies with the magic.
package wal

import (
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	HeaderSize        = 32
	FrameHeaderSize   = 24
	Version           = uint32(3007000)
	MagicLittleEndian = uint32(0x377f0682)
	MagicBigEndian    = uint32(0x377f0683)
	MaxPageNumber     = uint32(0xfffffffe)
)

var (
	ErrFormat      = errors.New("invalid SQLite WAL")
	ErrUnsupported = errors.New("unsupported SQLite WAL version")
	ErrChecksum    = errors.New("SQLite WAL checksum mismatch")
	ErrSalt        = errors.New("SQLite WAL salt mismatch")
	ErrLimit       = errors.New("SQLite WAL resource limit exceeded")
	ErrTail        = errors.New("SQLite WAL has a non-committed or invalid tail")
)

// Checksum contains the two rolling 32-bit sums. This is an accidental-damage
// check, not cryptographic authentication. Unsigned overflow is intentional.
type Checksum = [2]uint32

// Header is the 32-byte WAL header. PageSize is the actual byte count, including
// 65536; the database-header sentinel value 1 is NOT valid in a WAL header.
type Header struct {
	Magic              uint32
	Version            uint32
	PageSize           uint32
	CheckpointSequence uint32
	Salt               [2]uint32
	Checksum           Checksum
}

// Validate checks fields, not the checksum (which requires the encoded bytes).
func (h Header) Validate() error {
	if h.Magic != MagicLittleEndian && h.Magic != MagicBigEndian {
		return fmt.Errorf("%w: magic %#x", ErrFormat, h.Magic)
	}
	if h.Version != Version {
		return fmt.Errorf("%w: %d", ErrUnsupported, h.Version)
	}
	if h.PageSize < 512 || h.PageSize > 65536 || h.PageSize&(h.PageSize-1) != 0 {
		return fmt.Errorf("%w: page size %d", ErrFormat, h.PageSize)
	}
	return nil
}

// RollingChecksum extends previous with a whole number of 8-byte pairs. The
// magic chooses the input word order. The checksum is stored big-endian even
// when magic is MagicLittleEndian. On error, previous is returned unchanged.
func RollingChecksum(magic uint32, data []byte, previous Checksum) (Checksum, error) {
	if (magic != MagicLittleEndian && magic != MagicBigEndian) || len(data)%8 != 0 {
		return previous, fmt.Errorf("%w: checksum input", ErrFormat)
	}
	return checksum(magic, data, previous), nil
}

func checksum(magic uint32, data []byte, sum Checksum) Checksum {
	var order binary.ByteOrder = binary.LittleEndian
	if magic == MagicBigEndian {
		order = binary.BigEndian
	}
	for i := 0; i < len(data); i += 8 {
		sum[0] += order.Uint32(data[i:]) + sum[1]
		sum[1] += order.Uint32(data[i+4:]) + sum[0]
	}
	return sum
}

// EncodeHeader returns a canonical header, recomputing and ignoring h.Checksum.
// Parse the returned bytes to obtain the checksum used to seed the first frame.
// Magic and Version must be explicit; an all-zero Header is not valid.
func EncodeHeader(h Header) ([]byte, error) {
	if err := h.Validate(); err != nil {
		return nil, err
	}
	b := make([]byte, HeaderSize)
	put := binary.BigEndian.PutUint32
	put(b[0:], h.Magic)
	put(b[4:], h.Version)
	put(b[8:], h.PageSize)
	put(b[12:], h.CheckpointSequence)
	put(b[16:], h.Salt[0])
	put(b[20:], h.Salt[1])
	sum := checksum(h.Magic, b[:24], Checksum{})
	put(b[24:], sum[0])
	put(b[28:], sum[1])
	return b, nil
}

// ParseHeader validates exactly one complete header. No unverified fields are
// returned when validation fails.
func ParseHeader(b []byte) (Header, error) {
	if len(b) != HeaderSize {
		return Header{}, fmt.Errorf("%w: header length %d", ErrFormat, len(b))
	}
	get := binary.BigEndian.Uint32
	h := Header{
		Magic: get(b[0:]), Version: get(b[4:]), PageSize: get(b[8:]),
		CheckpointSequence: get(b[12:]),
		Salt:               [2]uint32{get(b[16:]), get(b[20:])},
		Checksum:           Checksum{get(b[24:]), get(b[28:])},
	}
	if err := h.Validate(); err != nil {
		return Header{}, err
	}
	if checksum(h.Magic, b[:24], Checksum{}) != h.Checksum {
		return Header{}, fmt.Errorf("%w: %w in header", ErrFormat, ErrChecksum)
	}
	return h, nil
}

// Frame is a validated frame header. DBSize is zero except on a commit marker.
// A frame can refer to a page above DBSize if the commit truncates the database;
// the scanner excludes such pages from its committed snapshot.
type Frame struct {
	PageNo   uint32
	DBSize   uint32
	Salt     [2]uint32
	Checksum Checksum
}

func validateFrame(h Header, pageNo, dbSize uint32, pageLen int) error {
	if err := h.Validate(); err != nil {
		return err
	}
	if pageLen != int(h.PageSize) {
		return fmt.Errorf("%w: frame page length %d", ErrFormat, pageLen)
	}
	if pageNo == 0 || pageNo > MaxPageNumber || pageNo == 0x40000000/h.PageSize+1 {
		return fmt.Errorf("%w: frame page number %d", ErrFormat, pageNo)
	}
	if dbSize > MaxPageNumber {
		return fmt.Errorf("%w: database size %d", ErrFormat, dbSize)
	}
	return nil
}

// EncodeFrame returns a complete frame and its rolling checksum. The caller
// supplies the preceding frame's checksum (the header's for the first frame).
// It does not mutate h or page. Salts are always taken from h.
func EncodeFrame(h Header, pageNo, dbSize uint32, page []byte, previous Checksum) ([]byte, Checksum, error) {
	if err := validateFrame(h, pageNo, dbSize, len(page)); err != nil {
		return nil, previous, err
	}
	b := make([]byte, FrameHeaderSize+len(page))
	put := binary.BigEndian.PutUint32
	put(b[0:], pageNo)
	put(b[4:], dbSize)
	put(b[8:], h.Salt[0])
	put(b[12:], h.Salt[1])
	copy(b[FrameHeaderSize:], page)
	sum := checksum(h.Magic, page, checksum(h.Magic, b[:8], previous))
	put(b[16:], sum[0])
	put(b[20:], sum[1])
	return b, sum, nil
}

// ParseFrame validates one header and its page using the previous rolling
// checksum. Page bytes are never retained or copied. FrameHeader must contain
// exactly FrameHeaderSize bytes, not the complete frame.
func ParseFrame(h Header, frameHeader, page []byte, previous Checksum) (Frame, error) {
	if len(frameHeader) != FrameHeaderSize {
		return Frame{}, fmt.Errorf("%w: frame header length %d", ErrFormat, len(frameHeader))
	}
	get := binary.BigEndian.Uint32
	f := Frame{
		PageNo: get(frameHeader[0:]), DBSize: get(frameHeader[4:]),
		Salt:     [2]uint32{get(frameHeader[8:]), get(frameHeader[12:])},
		Checksum: Checksum{get(frameHeader[16:]), get(frameHeader[20:])},
	}
	if err := validateFrame(h, f.PageNo, f.DBSize, len(page)); err != nil {
		return Frame{}, err
	}
	if f.Salt != h.Salt {
		return Frame{}, fmt.Errorf("%w: %w", ErrFormat, ErrSalt)
	}
	sum := checksum(h.Magic, page, checksum(h.Magic, frameHeader[:8], previous))
	if sum != f.Checksum {
		return Frame{}, fmt.Errorf("%w: %w in frame", ErrFormat, ErrChecksum)
	}
	return f, nil
}
