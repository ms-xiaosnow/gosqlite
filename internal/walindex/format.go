// Package walindex implements SQLite's native-endian shared-memory WAL index.
// The layout and publication protocol follow SQLite 3.54.0 src/wal.c and
// src/os_unix.c. The index is transient: WAL bytes remain the durable record.
// Callers must retain SQLite's main-database SHARED lock for an Index's lifetime
// and obey WAL read, write, recovery, and checkpoint transaction protocols.
package walindex

import (
	"encoding/binary"
	"errors"
	"fmt"
)

const (
	Version          = uint32(3007000)
	HeaderSize       = 48
	FirstHeaderSize  = 136
	BlockSize        = 32768
	FirstBlockFrames = uint32(4062)
	BlockFrames      = uint32(4096)
	HashSlots        = 8192
	DefaultMaxFrames = uint32(1 << 20)
	WriteLock        = 0
	CheckpointLock   = 1
	RecoveryLock     = 2
	ReadLock0        = 3
	ReaderCount      = 5
	UnusedReadMark   = uint32(0xffffffff)
)

var (
	ErrPoisoned      = errors.New("SQLite WAL-index ownership cannot safely continue")
	ErrBusy          = errors.New("SQLite WAL-index lock busy")
	ErrHeader        = errors.New("invalid SQLite WAL-index")
	ErrUninitialized = errors.New("SQLite WAL-index is not initialized")
	ErrIdentity      = errors.New("SQLite WAL-index file identity changed")
	ErrLimit         = errors.New("SQLite WAL-index resource limit exceeded")
	ErrLock          = errors.New("required SQLite WAL-index lock not held")
	ErrClosed        = errors.New("SQLite WAL-index is closed")
	ErrUnsupported   = errors.New("SQLite WAL-index operation unsupported")
)

// Header is comparable and represents all 48 bytes of a validated index
// header. PageSize is the actual size, including 65536; zero is permitted only
// for an empty index. Salt is copied verbatim from WAL bytes 16..23, unlike
// integer fields, which use host byte order. Checksum is recomputed on encode.
// Initialized headers use isInit=1; BigEndianChecksum encodes a 0 or 1 byte.
type Header struct {
	Version           uint32
	Unused            uint32
	Change            uint32
	BigEndianChecksum bool
	PageSize          uint32
	MaxFrame          uint32
	DBSize            uint32
	FrameChecksum     [2]uint32
	Salt              [8]byte
	Checksum          [2]uint32
}

func (h Header) validate() error {
	if h.Version != Version {
		return fmt.Errorf("%w: %w: version %d", ErrHeader, ErrUnsupported, h.Version)
	}
	if h.PageSize == 0 {
		if h.MaxFrame != 0 || h.DBSize != 0 {
			return fmt.Errorf("%w: nonempty index with zero page size", ErrHeader)
		}
	} else if h.PageSize < 512 || h.PageSize > 65536 || h.PageSize&(h.PageSize-1) != 0 {
		return fmt.Errorf("%w: page size %d", ErrHeader, h.PageSize)
	}
	if h.DBSize > 0xfffffffe || (h.MaxFrame > 0 && h.DBSize == 0) {
		return fmt.Errorf("%w: database size %d", ErrHeader, h.DBSize)
	}
	return nil
}

func nativeChecksum(b []byte) (sum [2]uint32) {
	for off := 0; off < len(b); off += 8 {
		sum[0] += binary.NativeEndian.Uint32(b[off:]) + sum[1]
		sum[1] += binary.NativeEndian.Uint32(b[off+4:]) + sum[0]
	}
	return
}

// EncodeHeader returns one native-endian copy, recomputing h.Checksum.
// Version must be explicit. Publish supplies the current version itself.
func EncodeHeader(h Header) ([HeaderSize]byte, error) {
	var b [HeaderSize]byte
	if err := h.validate(); err != nil {
		return b, err
	}
	put := binary.NativeEndian.PutUint32
	put(b[0:], h.Version)
	put(b[4:], h.Unused)
	put(b[8:], h.Change)
	b[12] = 1
	if h.BigEndianChecksum {
		b[13] = 1
	}
	ps := h.PageSize
	if ps == 65536 {
		ps = 1
	}
	binary.NativeEndian.PutUint16(b[14:], uint16(ps))
	put(b[16:], h.MaxFrame)
	put(b[20:], h.DBSize)
	put(b[24:], h.FrameChecksum[0])
	put(b[28:], h.FrameChecksum[1])
	copy(b[32:40], h.Salt[:])
	sum := nativeChecksum(b[:40])
	put(b[40:], sum[0])
	put(b[44:], sum[1])
	return b, nil
}

// ParseHeader validates exactly one complete 48-byte copy. It does not inspect
// a second copy or accept partial, uninitialized, or checksum-invalid data.
func ParseHeader(b []byte) (Header, error) {
	if len(b) != HeaderSize {
		return Header{}, fmt.Errorf("%w: header length %d", ErrHeader, len(b))
	}
	if b[12] == 0 {
		return Header{}, ErrUninitialized
	}
	if b[12] != 1 || b[13] > 1 {
		return Header{}, fmt.Errorf("%w: flags", ErrHeader)
	}
	get := binary.NativeEndian.Uint32
	h := Header{Version: get(b), Unused: get(b[4:]), Change: get(b[8:]), BigEndianChecksum: b[13] == 1,
		PageSize: uint32(binary.NativeEndian.Uint16(b[14:])), MaxFrame: get(b[16:]), DBSize: get(b[20:]),
		FrameChecksum: [2]uint32{get(b[24:]), get(b[28:])}, Checksum: [2]uint32{get(b[40:]), get(b[44:])}}
	copy(h.Salt[:], b[32:40])
	if h.PageSize == 1 {
		h.PageSize = 65536
	}
	if err := h.validate(); err != nil {
		return Header{}, err
	}
	if nativeChecksum(b[:40]) != h.Checksum {
		return Header{}, fmt.Errorf("%w: header checksum", ErrHeader)
	}
	return h, nil
}

func frameBlock(frame uint32) uint32 {
	return uint32((uint64(frame) + uint64(BlockFrames-FirstBlockFrames) - 1) / uint64(BlockFrames))
}
func blockBase(block uint32) uint32 {
	if block == 0 {
		return 0
	}
	return FirstBlockFrames + (block-1)*BlockFrames
}
func blockCapacity(block uint32) uint32 {
	if block == 0 {
		return FirstBlockFrames
	}
	return BlockFrames
}
func pageOffset(block, relative uint32) int {
	start := 0
	if block == 0 {
		start = FirstHeaderSize
	}
	return start + int(relative-1)*4
}
