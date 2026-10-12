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

// ReplayLimits bounds the entire input, cumulative records across all segments,
// and the resulting database extent. Zero values select 64 MiB, 65536 records,
// and 1 TiB, respectively. Owned page-image bytes never exceed MaxJournalBytes
// (each image occupies a disjoint input record); page and duplicate-detection
// metadata are O(MaxRecords). Allow roughly twice the input size plus that
// metadata when loading a journal. These are not a bound on total process RSS.
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

// ReplayCountEvidence records how the supplied bytes describe record counts.
// It is not authentication, proof of durability, or native replay authority.
type ReplayCountEvidence uint8

const (
	// ReplayCountUnknown is deliberately the zero value and is not admitted
	// by native recovery. Future evidence values must also fail closed.
	ReplayCountUnknown ReplayCountEvidence = iota
	// ReplayCountFiniteHeaders means every published segment supplies a count.
	ReplayCountFiniteHeaders
	// ReplayCountEOFUnproven means the first raw count is 0xffffffff and the
	// effective count comes from supplied EOF. Whole-record truncation cannot
	// be detected from the remaining bytes, including a header-only prefix.
	ReplayCountEOFUnproven
)

// ReplayPlan is a structurally validated journal from the supported subset.
// Header is the first segment's unmodified header, including a raw 0xffffffff
// count; len(Pages) is the effective cumulative count. Pages contains unique,
// owned images in journal order. DatabaseBytes is the original extent.
// CountEvidence distinguishes finite published counts from an EOF-derived,
// completeness-unproven count. Neither value authenticates the journal.
// Parsing and extent validation perform no I/O and establish neither that an
// on-disk journal is hot nor that it is eligible for native recovery. Native
// recovery must reject EOF-derived plans before any replay/publication work.
type ReplayPlan struct {
	Header        Header
	Pages         []PageImage
	DatabaseBytes int64
	CountEvidence ReplayCountEvidence
}

// ParseReplay validates the entire supplied journal before producing an owned
// plan. It is deliberately stricter than SQLite's stop-at-invalid-record
// playback: partial headers/records, checksum failures, duplicate pages (within
// or across segments), and ambiguous tails never yield a partially usable plan.
// Finite-count segments repeat the first header's page size, sector size and
// original extent, and use their own nonce. Later headers start at the next
// sector boundary at or after the preceding segment's exact end; the last
// segment must end at input EOF. Padding alone is not a later segment.
//
// A first published raw count of 0xffffffff instead describes exactly one
// header followed solely by whole records to supplied EOF. Its effective count
// is bounded before record allocation, and CountEvidence is EOFUnproven. This
// cannot detect truncation at a whole-record boundary. The sentinel does not
// establish no-sync, memory-journal or safe-append writer provenance. No later
// sentinel, unpublished inferred count, super/master-journal, or legacy zero
// page size is supported. No header is found by scanning padding or page data.
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
	if h.PageSize == 0 {
		return nil, fmt.Errorf("%w: legacy page size", ErrReplayUnsupported)
	}
	if h.PageSize < 512 || h.PageSize > 65536 || h.PageSize&(h.PageSize-1) != 0 ||
		h.SectorSize < 32 || h.SectorSize > 65536 || h.SectorSize&(h.SectorSize-1) != 0 ||
		h.OriginalPages > 0xfffffffe {
		return nil, fmt.Errorf("%w: header geometry", ErrFormat)
	}
	if h.Records != 0xffffffff && h.Records > limits.MaxRecords {
		return nil, fmt.Errorf("%w: record count", ErrReplayLimit)
	}
	extent := uint64(h.OriginalPages) * uint64(h.PageSize)
	if extent > limits.MaxDatabaseBytes {
		return nil, fmt.Errorf("%w: original database extent", ErrReplayLimit)
	}
	// The first header alone determines alignment, record size and extent.
	// No segment/header list is retained: even zero-record segments consume a
	// full sector of bounded input and advance the cursor.
	size, sector := uint64(len(data)), uint64(h.SectorSize)
	recordBytes := uint64(h.PageSize) + 8
	countEvidence := ReplayCountFiniteHeaders
	firstCount := uint64(h.Records)
	if h.Records == 0xffffffff {
		// Prove the complete sector before subtraction. Unlike SQLite's
		// floor division, reject every partial record at supplied EOF.
		if sector > size {
			return nil, fmt.Errorf("%w: truncated header sector", ErrFormat)
		}
		remaining := size - sector
		if remaining%recordBytes != 0 {
			return nil, fmt.Errorf("%w: partial EOF-count record", ErrFormat)
		}
		firstCount = remaining / recordBytes
		if firstCount > uint64(limits.MaxRecords) {
			return nil, fmt.Errorf("%w: EOF-derived record count", ErrReplayLimit)
		}
		if firstCount > uint64(h.OriginalPages) {
			return nil, fmt.Errorf("%w: more records than original pages", ErrFormat)
		}
		countEvidence = ReplayCountEOFUnproven
	}
	plan := &ReplayPlan{Header: h, DatabaseBytes: int64(extent), Pages: make([]PageImage, 0), CountEvidence: countEvidence}
	seen := make(map[uint32]struct{})
	var total uint64
	for headerOffset := uint64(0); ; {
		segment := Header{
			Records: binary.BigEndian.Uint32(data[headerOffset+8:]), Nonce: binary.BigEndian.Uint32(data[headerOffset+12:]),
			OriginalPages: binary.BigEndian.Uint32(data[headerOffset+16:]), SectorSize: binary.BigEndian.Uint32(data[headerOffset+20:]),
			PageSize: binary.BigEndian.Uint32(data[headerOffset+24:]),
		}
		segmentRecords := uint64(segment.Records)
		if segment.Records == 0xffffffff {
			if headerOffset != 0 {
				return nil, fmt.Errorf("%w: later EOF-count header", ErrReplayUnsupported)
			}
			// The derived body ends exactly at EOF, so this path can never
			// enter the subsequent-header alignment logic.
			segmentRecords = firstCount
		}
		if segment.OriginalPages != h.OriginalPages || segment.SectorSize != h.SectorSize || segment.PageSize != h.PageSize {
			return nil, fmt.Errorf("%w: segment geometry differs from first header", ErrReplayUnsupported)
		}
		// Subtract before adding to keep cumulative counts bounded even when
		// individual advertised uint32 counts would wrap a uint32 sum.
		if segmentRecords > uint64(limits.MaxRecords)-total {
			return nil, fmt.Errorf("%w: cumulative record count", ErrReplayLimit)
		}
		total += segmentRecords
		// A uint32 count times (at most) 65544 fits uint64. Check remaining
		// input before adding offsets or allocating record metadata/images.
		bodyBytes := segmentRecords * recordBytes
		if sector > size-headerOffset || bodyBytes > size-headerOffset-sector {
			return nil, fmt.Errorf("%w: truncated header sector or records", ErrFormat)
		}
		end := headerOffset + sector + bodyBytes
		if segmentRecords > uint64(h.OriginalPages) {
			return nil, fmt.Errorf("%w: more records than original pages", ErrFormat)
		}
		for offset := headerOffset + sector; offset < end; offset += recordBytes {
			record := data[offset : offset+recordBytes]
			number := binary.BigEndian.Uint32(record)
			if number == 0 || number > h.OriginalPages || number == 0x40000000/h.PageSize+1 {
				return nil, fmt.Errorf("%w: page number %d", ErrFormat, number)
			}
			if _, exists := seen[number]; exists {
				return nil, fmt.Errorf("%w: duplicate original page %d", ErrReplayUnsupported, number)
			}
			page := record[4 : len(record)-4]
			if binary.BigEndian.Uint32(record[len(record)-4:]) != Checksum(page, segment.Nonce) {
				return nil, fmt.Errorf("%w: checksum of page %d", ErrFormat, number)
			}
			seen[number] = struct{}{}
			// These input views never escape. Copy only after every segment
			// has passed validation, so a late error owns no page-image bytes.
			plan.Pages = append(plan.Pages, PageImage{number, page})
		}
		if end == size {
			break
		}
		padding := (sector - end%sector) % sector
		if padding > size-end || size-end-padding < 28 {
			return nil, fmt.Errorf("%w: trailing bytes without a complete published header", ErrReplayUnsupported)
		}
		headerOffset = end + padding
		if !bytes.Equal(data[headerOffset:headerOffset+8], magic[:]) {
			return nil, fmt.Errorf("%w: unpublished or ambiguous next segment", ErrReplayUnsupported)
		}
	}
	for i := range plan.Pages {
		plan.Pages[i].Data = append([]byte(nil), plan.Pages[i].Data...)
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
