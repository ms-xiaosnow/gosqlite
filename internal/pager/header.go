// Package pager provides bounded SQLite page reads, native rollback-mode write
// transactions, and opt-in connection-exclusive WAL sessions. Interrupted-I/O
// recovery and durability remain unverified.
package pager

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

var (
	ErrCorrupt     = errors.New("corrupt SQLite database")
	ErrUnsupported = errors.New("unsupported SQLite database feature")
	ErrBusy        = errors.New("SQLite database is busy")
	ErrChanged     = errors.New("SQLite database changed during read")
	ErrClosed      = errors.New("SQLite pager is closed")
)

// Header describes the validated 100-byte SQLite database header.
type Header struct {
	PageSize        uint32
	UsableSize      uint32
	PageCount       uint32
	ReservedBytes   byte
	WriteVersion    byte
	ReadVersion     byte
	ChangeCounter   uint32
	SchemaCookie    uint32
	SchemaFormat    uint32
	TextEncoding    uint32
	UserVersion     uint32
	ApplicationID   uint32
	VersionValidFor uint32
	SQLiteVersion   uint32
}

// ParseHeader validates SQLite's 100-byte header against the physical file
// length. Unsupported formats fail closed, before page data is interpreted.
func ParseHeader(p []byte, fileSize int64) (Header, error) {
	return parseHeader(p, fileSize, false, 0)
}

// parseHeader keeps the physical main-file extent distinct from a committed
// WAL snapshot's logical extent. Only a locked WAL session may enable WAL.
// A nonzero logicalPages comes from a validated WAL commit marker.
func parseHeader(p []byte, fileSize int64, allowWAL bool, logicalPages uint32) (Header, error) {
	var h Header
	if len(p) < 100 || !bytes.Equal(p[:16], []byte("SQLite format 3\x00")) {
		return h, fmt.Errorf("%w: invalid 100-byte header or magic", ErrCorrupt)
	}
	u32 := func(off int) uint32 { return binary.BigEndian.Uint32(p[off : off+4]) }
	h.PageSize = uint32(binary.BigEndian.Uint16(p[16:18]))
	if h.PageSize == 1 {
		h.PageSize = 65536
	}
	if h.PageSize < 512 || h.PageSize > 65536 || h.PageSize&(h.PageSize-1) != 0 {
		return h, fmt.Errorf("%w: invalid page size", ErrCorrupt)
	}
	h.WriteVersion, h.ReadVersion = p[18], p[19]
	if (p[18] != 1 || p[19] != 1) && !(allowWAL && p[18] == 2 && p[19] == 2) {
		return h, fmt.Errorf("%w: WAL or unknown read/write version", ErrUnsupported)
	}
	h.ReservedBytes = p[20]
	h.UsableSize = h.PageSize - uint32(p[20])
	if h.UsableSize < 480 {
		return h, fmt.Errorf("%w: usable page size below 480", ErrCorrupt)
	}
	if p[21] != 64 || p[22] != 32 || p[23] != 32 {
		return h, fmt.Errorf("%w: payload fractions", ErrCorrupt)
	}
	if fileSize < 100 || fileSize%int64(h.PageSize) != 0 {
		return h, fmt.Errorf("%w: truncated or unaligned database", ErrCorrupt)
	}
	physical := uint64(fileSize / int64(h.PageSize))
	if physical == 0 || physical > 0xfffffffe {
		return h, fmt.Errorf("%w: invalid page count", ErrCorrupt)
	}
	h.ChangeCounter = u32(24)
	h.VersionValidFor = u32(92)
	h.PageCount = u32(28)
	if logicalPages != 0 {
		if logicalPages > 0xfffffffe {
			return h, fmt.Errorf("%w: invalid WAL database extent", ErrCorrupt)
		}
		h.PageCount = logicalPages
	} else if h.PageCount == 0 || h.ChangeCounter != h.VersionValidFor {
		h.PageCount = uint32(physical)
	} else if uint64(h.PageCount) > physical {
		return h, fmt.Errorf("%w: header page count exceeds file", ErrCorrupt)
	}
	if u32(32) > h.PageCount || u32(36) >= h.PageCount || (u32(32) == 0) != (u32(36) == 0) {
		return h, fmt.Errorf("%w: freelist header", ErrCorrupt)
	}
	h.SchemaCookie = u32(40)
	h.SchemaFormat = u32(44)
	if h.SchemaFormat != 0 && (h.SchemaFormat < 1 || h.SchemaFormat > 4) {
		return h, fmt.Errorf("%w: schema format %d", ErrUnsupported, h.SchemaFormat)
	}
	if u32(52) != 0 || u32(64) != 0 {
		return h, fmt.Errorf("%w: auto-vacuum databases", ErrUnsupported)
	}
	h.TextEncoding = u32(56)
	if h.TextEncoding != 1 && !(h.TextEncoding == 0 && h.SchemaFormat == 0) {
		return h, fmt.Errorf("%w: text encoding %d", ErrUnsupported, h.TextEncoding)
	}
	h.UserVersion = u32(60)
	h.ApplicationID = u32(68)
	h.SQLiteVersion = u32(96)
	for _, b := range p[72:92] {
		if b != 0 {
			return h, fmt.Errorf("%w: nonzero reserved header bytes", ErrUnsupported)
		}
	}
	return h, nil
}
