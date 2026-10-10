package journal

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
)

var (
	ErrReplayUnsupported = errors.New("unsupported SQLite rollback journal recovery")
	ErrReplayLimit       = errors.New("SQLite rollback journal recovery resource limit exceeded")
)

// ReplayLimits bounds input, page-image allocation, and the resulting database
// extent. Zero values select 64 MiB, 65536 records, and 1 TiB, respectively.
// Replay keeps independent copies of page images, so allow roughly twice the
// input size plus per-record metadata when loading a journal into memory.
type ReplayLimits struct {
	MaxJournalBytes  uint64
	MaxRecords       uint32
	MaxDatabaseBytes uint64
}

func (l ReplayLimits) WithDefaults() ReplayLimits {
	if l.MaxJournalBytes == 0 {
		l.MaxJournalBytes = 64 << 20
	}
	if l.MaxRecords == 0 {
		l.MaxRecords = 1 << 16
	}
	if l.MaxDatabaseBytes == 0 {
		l.MaxDatabaseBytes = 1 << 40
	}
	return l
}

// PageImage is an original database page, owned by the replay plan.
type PageImage struct {
	Number uint32
	Data   []byte
}

// ReplayPlan is a fully validated, single-segment journal. Its pages must be
// restored, its database truncated to DatabaseBytes, and that database synced
// before the journal may be removed. Parsing and extent validation perform no
// I/O. Neither operation establishes that an on-disk journal is actually hot.
type ReplayPlan struct {
	Header        Header
	Pages         []PageImage
	DatabaseBytes int64
}

// ParseReplay validates the entire journal before producing a replay plan.
// It is deliberately stricter than SQLite's stop-at-invalid-record playback:
// partial headers, checksum failures, duplicate pages, and trailing bytes
// never yield a partially usable plan. Exactly one finite-count segment is
// supported. Super/master-journals, no-sync counts, legacy unspecified page
// sizes, and multiple segments require a different recovery implementation.
//
// Sector sizes 32..65536 follow SQLite 3.54 src/pager.c readJournalHdr(). The
// existing writer's Header.Validate intentionally has a narrower minimum of
// 512; replay does not use that writer validation. Padding is unspecified and
// need not contain zeros. SQLite's sparse checksum is not authentication.
func ParseReplay(data []byte, limits ReplayLimits) (*ReplayPlan, error) {
	limits = limits.WithDefaults()
	if uint64(len(data)) > limits.MaxJournalBytes {
		return nil, fmt.Errorf("%w: journal bytes", ErrReplayLimit)
	}
	if len(data) < 28 || !bytes.Equal(data[:8], magic[:]) {
		return nil, fmt.Errorf("%w: missing complete published header", ErrFormat)
	}
	// An apparent super-journal trailer is never treated as ordinary data,
	// even if its name/checksum is malformed. Do not inspect that other file
	// or guess whether a multi-database transaction committed.
	if len(data) >= 16 && bytes.Equal(data[len(data)-8:], magic[:]) {
		return nil, fmt.Errorf("%w: possible super-journal trailer", ErrReplayUnsupported)
	}
	h := Header{
		Records: binary.BigEndian.Uint32(data[8:]), Nonce: binary.BigEndian.Uint32(data[12:]),
		OriginalPages: binary.BigEndian.Uint32(data[16:]), SectorSize: binary.BigEndian.Uint32(data[20:]),
		PageSize: binary.BigEndian.Uint32(data[24:]),
	}
	if h.Records == 0xffffffff || h.PageSize == 0 {
		return nil, fmt.Errorf("%w: no-sync count or legacy page size", ErrReplayUnsupported)
	}
	if h.PageSize < 512 || h.PageSize > 65536 || h.PageSize&(h.PageSize-1) != 0 ||
		h.SectorSize < 32 || h.SectorSize > 65536 || h.SectorSize&(h.SectorSize-1) != 0 ||
		h.OriginalPages > 0xfffffffe {
		return nil, fmt.Errorf("%w: header geometry", ErrFormat)
	}
	if h.Records > limits.MaxRecords {
		return nil, fmt.Errorf("%w: record count", ErrReplayLimit)
	}
	extent := uint64(h.OriginalPages) * uint64(h.PageSize)
	if extent > limits.MaxDatabaseBytes {
		return nil, fmt.Errorf("%w: original database extent", ErrReplayLimit)
	}
	// These multiplications cannot overflow uint64: the operands are uint32
	// and the validated page and sector sizes are at most 65536.
	recordBytes := uint64(h.PageSize) + 8
	end := uint64(h.SectorSize) + uint64(h.Records)*recordBytes
	if uint64(len(data)) < end {
		return nil, fmt.Errorf("%w: truncated header sector or records", ErrFormat)
	}
	if uint64(len(data)) != end {
		return nil, fmt.Errorf("%w: trailing bytes or multiple segments", ErrReplayUnsupported)
	}
	if h.Records > h.OriginalPages {
		return nil, fmt.Errorf("%w: more records than original pages", ErrFormat)
	}
	plan := &ReplayPlan{Header: h, DatabaseBytes: int64(extent), Pages: make([]PageImage, 0, int(h.Records))}
	seen := make(map[uint32]struct{}, int(h.Records))
	for offset := uint64(h.SectorSize); offset < end; offset += recordBytes {
		record := data[offset : offset+recordBytes]
		number := binary.BigEndian.Uint32(record)
		if number == 0 || number > h.OriginalPages || number == 0x40000000/h.PageSize+1 {
			return nil, fmt.Errorf("%w: page number %d", ErrFormat, number)
		}
		if _, exists := seen[number]; exists {
			return nil, fmt.Errorf("%w: duplicate original page %d", ErrReplayUnsupported, number)
		}
		page := record[4 : len(record)-4]
		if binary.BigEndian.Uint32(record[len(record)-4:]) != Checksum(page, h.Nonce) {
			return nil, fmt.Errorf("%w: checksum of page %d", ErrFormat, number)
		}
		seen[number] = struct{}{}
		plan.Pages = append(plan.Pages, PageImage{number, append([]byte(nil), page...)})
	}
	return plan, nil
}

// ValidateExtent ensures that extending a shortened/partial database cannot
// silently invent zero-filled original pages. Every original page not fully
// present in the current extent needs a journal image, except SQLite's unused
// locking page. Extra current bytes will be removed by the planned truncation.
func (p *ReplayPlan) ValidateExtent(currentBytes int64) error {
	if p == nil || p.Header.PageSize == 0 || currentBytes < 0 {
		return fmt.Errorf("%w: invalid current extent", ErrFormat)
	}
	firstMissing := uint64(currentBytes)/uint64(p.Header.PageSize) + 1
	last := uint64(p.Header.OriginalPages)
	if firstMissing > last {
		return nil
	}
	missing := last - firstMissing + 1
	lockPage := uint64(0x40000000/p.Header.PageSize + 1)
	if lockPage >= firstMissing && lockPage <= last {
		missing--
	}
	for _, page := range p.Pages {
		if uint64(page.Number) >= firstMissing && uint64(page.Number) <= last {
			missing--
		}
	}
	if missing != 0 {
		return fmt.Errorf("%w: missing original page images for shortened database", ErrFormat)
	}
	return nil
}
