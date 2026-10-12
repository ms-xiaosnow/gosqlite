// Package journal implements the public SQLite rollback-journal wire format.
// It is independent of I/O and does not imply verified crash recovery.
package journal

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

var ErrFormat = errors.New("invalid SQLite rollback journal")
var magic = [8]byte{0xd9, 0xd5, 0x05, 0xf9, 0x20, 0xa1, 0x63, 0xd7}

// Header is one rollback-journal segment header. This encoding/record API
// accepts only finite counts; super-journals are unsupported. ParseReplay
// separately decodes a bounded first-only EOF count without native authority.
type Header struct {
	Records       uint32
	Nonce         uint32
	OriginalPages uint32
	SectorSize    uint32
	PageSize      uint32
}

func (h Header) Validate() error {
	if h.PageSize < 512 || h.PageSize > 65536 || h.PageSize&(h.PageSize-1) != 0 || h.SectorSize < 512 || h.SectorSize > 65536 || h.SectorSize&(h.SectorSize-1) != 0 || h.OriginalPages > 0xfffffffe || h.Records == 0xffffffff {
		return ErrFormat
	}
	return nil
}

// EncodeHeader returns a complete, sector-padded header. Before the journal
// body is synced, callers must use valid=false (zero magic and record count).
func EncodeHeader(h Header, valid bool) ([]byte, error) {
	if err := h.Validate(); err != nil {
		return nil, err
	}
	b := make([]byte, h.SectorSize)
	if valid {
		copy(b, magic[:])
		binary.BigEndian.PutUint32(b[8:], h.Records)
	}
	binary.BigEndian.PutUint32(b[12:], h.Nonce)
	binary.BigEndian.PutUint32(b[16:], h.OriginalPages)
	binary.BigEndian.PutUint32(b[20:], h.SectorSize)
	binary.BigEndian.PutUint32(b[24:], h.PageSize)
	return b, nil
}

func ParseHeader(b []byte) (Header, error) {
	var h Header
	if len(b) < 28 || !bytes.Equal(b[:8], magic[:]) {
		return h, ErrFormat
	}
	h = Header{binary.BigEndian.Uint32(b[8:]), binary.BigEndian.Uint32(b[12:]), binary.BigEndian.Uint32(b[16:]), binary.BigEndian.Uint32(b[20:]), binary.BigEndian.Uint32(b[24:])}
	if err := h.Validate(); err != nil {
		return Header{}, err
	}
	if uint64(len(b)) < uint64(h.SectorSize) {
		return Header{}, ErrFormat
	}
	return h, nil
}

// Checksum is the format-defined sparse checksum, not a cryptographic hash.
// See SQLite 3.54.0 src/pager.c pager_cksum().
func Checksum(page []byte, nonce uint32) uint32 {
	sum := nonce
	for i := len(page) - 200; i > 0; i -= 200 {
		sum += uint32(page[i])
	}
	return sum
}

func EncodeRecord(h Header, pageNo uint32, page []byte) ([]byte, error) {
	if err := h.Validate(); err != nil {
		return nil, err
	}
	if len(page) != int(h.PageSize) || pageNo == 0 || pageNo > h.OriginalPages || pageNo == 0x40000000/h.PageSize+1 {
		return nil, ErrFormat
	}
	b := make([]byte, len(page)+8)
	binary.BigEndian.PutUint32(b, pageNo)
	copy(b[4:], page)
	binary.BigEndian.PutUint32(b[len(b)-4:], Checksum(page, h.Nonce))
	return b, nil
}

// ParseRecord validates one complete record and returns an independent page.
func ParseRecord(h Header, b []byte) (uint32, []byte, error) {
	if err := h.Validate(); err != nil {
		return 0, nil, err
	}
	if uint64(len(b)) != uint64(h.PageSize)+8 {
		return 0, nil, ErrFormat
	}
	n := binary.BigEndian.Uint32(b)
	if n == 0 || n > h.OriginalPages || n == 0x40000000/h.PageSize+1 {
		return 0, nil, ErrFormat
	}
	p := b[4 : len(b)-4]
	if binary.BigEndian.Uint32(b[len(b)-4:]) != Checksum(p, h.Nonce) {
		return 0, nil, fmt.Errorf("%w: page checksum", ErrFormat)
	}
	return n, append([]byte(nil), p...), nil
}
