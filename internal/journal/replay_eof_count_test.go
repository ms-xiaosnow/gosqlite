package journal

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

// These deterministic fixtures and models use byte arrays only. They do not
// open a database, use a pager, create files, invoke recovery, or claim native
// safety. In particular, an exact whole-record prefix can still decode: EOF
// does not establish that every original image of a transaction is present.
const eofCountMemorySentinel uint32 = 0xffffffff

var eofCountMemoryMagic = [...]byte{0xd9, 0xd5, 0x05, 0xf9, 0x20, 0xa1, 0x63, 0xd7}

type eofCountMemoryFixture struct {
	header Header
	data   []byte
	pages  []PageImage
}

// makeEOFCountMemoryFixture independently writes the five raw header words,
// unspecified padding, record numbers, original images, and sparse sums. It
// deliberately calls none of the production format encoders or Checksum.
func makeEOFCountMemoryFixture(h Header, numbers ...uint32) eofCountMemoryFixture {
	f := eofCountMemoryFixture{header: h}
	f.data = bytes.Repeat([]byte{0x6d}, int(h.SectorSize))
	copy(f.data[:8], eofCountMemoryMagic[:])
	for i, word := range []uint32{h.Records, h.Nonce, h.OriginalPages, h.SectorSize, h.PageSize} {
		binary.BigEndian.PutUint32(f.data[8+4*i:], word)
	}
	for index, number := range numbers {
		page := make([]byte, int(h.PageSize))
		for i := range page {
			// Reducing the page number before conversion keeps this fixture
			// arithmetic bounded on both 32-bit and 64-bit int platforms.
			page[i] = byte(i*7 + int(number%251)*13 + index*23)
		}
		record := make([]byte, len(page)+8)
		binary.BigEndian.PutUint32(record, number)
		copy(record[4:], page)
		binary.BigEndian.PutUint32(record[len(record)-4:], eofCountMemorySum(page, h.Nonce))
		f.data = append(f.data, record...)
		f.pages = append(f.pages, PageImage{Number: number, Data: page})
	}
	return f
}

func eofCountMemorySum(page []byte, nonce uint32) uint32 {
	// Independent positive strides from the end, accumulated widely before
	// the specified uint32 reduction. The sum is sparse, not authentication.
	sum := uint64(nonce)
	for stride := 200; stride < len(page); stride += 200 {
		sum += uint64(page[len(page)-stride])
	}
	return uint32(sum)
}

func checkEOFCountMemoryPlan(t *testing.T, f eofCountMemoryFixture, limits ReplayLimits) *ReplayPlan {
	t.Helper()
	inputBefore := bytes.Clone(f.data)
	plan, err := ParseReplay(f.data, limits)
	if err != nil || plan == nil {
		t.Fatalf("decode: plan=%v err=%v", plan, err)
	}
	if !bytes.Equal(f.data, inputBefore) {
		t.Fatal("parser changed input")
	}
	if plan.Header != f.header || plan.DatabaseBytes != int64(f.header.OriginalPages)*int64(f.header.PageSize) || len(plan.Pages) != len(f.pages) {
		t.Fatalf("raw header, effective count, or extent differs: header=%+v count=%d extent=%d", plan.Header, len(plan.Pages), plan.DatabaseBytes)
	}
	if f.header.Records == eofCountMemorySentinel {
		if plan.CountEvidence != ReplayCountEOFUnproven {
			t.Fatalf("EOF-derived count evidence=%v", plan.CountEvidence)
		}
		bodyBytes := uint64(len(f.data)) - uint64(f.header.SectorSize)
		recordBytes := uint64(f.header.PageSize) + 8
		if bodyBytes%recordBytes != 0 || bodyBytes/recordBytes != uint64(len(plan.Pages)) {
			t.Fatal("EOF count does not match exact sector-plus-record extent")
		}
	} else if plan.CountEvidence != ReplayCountFiniteHeaders {
		t.Fatalf("finite count evidence=%v", plan.CountEvidence)
	}
	for i, want := range f.pages {
		if plan.Pages[i].Number != want.Number || len(plan.Pages[i].Data) != int(f.header.PageSize) || !bytes.Equal(plan.Pages[i].Data, want.Data) {
			t.Fatalf("image %d differs from independent number/order/data model", i)
		}
	}
	return plan
}

func rejectEOFCountMemoryInput(t *testing.T, data []byte, limits ReplayLimits, want error) {
	t.Helper()
	before := bytes.Clone(data)
	plan, err := ParseReplay(data, limits)
	if plan != nil || err == nil || (want != nil && !errors.Is(err, want)) || !bytes.Equal(before, data) {
		t.Fatalf("rejection returned partial plan, wrong error, or changed input: plan=%v err=%v want=%v", plan, err, want)
	}
}

func TestReplayEOFCountGeometryAndOwnershipInMemory(t *testing.T) {
	// All eight supported page sizes, all twelve supported sector sizes,
	// and zero, one, and multiple records. Nonce addition also wraps uint32.
	for pageSize := uint32(512); pageSize <= 65536; pageSize *= 2 {
		for sectorSize := uint32(32); sectorSize <= 65536; sectorSize *= 2 {
			for _, numbers := range [][]uint32{nil, {3}, {7, 2, 1}} {
				t.Run(fmt.Sprintf("page-%d/sector-%d/records-%d", pageSize, sectorSize, len(numbers)), func(t *testing.T) {
					h := Header{Records: eofCountMemorySentinel, Nonce: 0xfffffff0, OriginalPages: 7, SectorSize: sectorSize, PageSize: pageSize}
					f := makeEOFCountMemoryFixture(h, numbers...)
					plan := checkEOFCountMemoryPlan(t, f, ReplayLimits{})
					other := checkEOFCountMemoryPlan(t, f, ReplayLimits{})
					if !reflect.DeepEqual(plan, other) {
						t.Fatal("repeated decoding is not deterministic")
					}
					for i := range f.data {
						f.data[i] ^= 0xff
					}
					if !reflect.DeepEqual(plan, other) || plan.Header != h {
						t.Fatal("result or raw header aliases input")
					}
					inputAfter := bytes.Clone(f.data)
					for i := range plan.Pages {
						if !bytes.Equal(plan.Pages[i].Data, f.pages[i].Data) {
							t.Fatal("owned image changed with input")
						}
						plan.Pages[i].Data[0] ^= 0x3c
						if !bytes.Equal(inputAfter, f.data) {
							t.Fatal("image edit reached input")
						}
						for j := range other.Pages {
							if !bytes.Equal(other.Pages[j].Data, f.pages[j].Data) || (j != i && !bytes.Equal(plan.Pages[j].Data, f.pages[j].Data)) {
								t.Fatal("images alias another page or another returned plan")
							}
						}
						plan.Pages[i].Data[0] ^= 0x3c
					}
				})
			}
		}
	}
}

func TestReplayEOFCountBoundariesInMemory(t *testing.T) {
	for _, sector := range []uint32{32, 512} {
		h := Header{Records: eofCountMemorySentinel, Nonce: 19, OriginalPages: 5, SectorSize: sector, PageSize: 512}
		f := makeEOFCountMemoryFixture(h, 5, 1, 3)
		for length := 0; length <= len(f.data); length++ {
			body := length - int(sector)
			if body >= 0 && body%520 == 0 {
				count := body / 520
				prefix := eofCountMemoryFixture{header: h, data: f.data[:length], pages: f.pages[:count]}
				plan := checkEOFCountMemoryPlan(t, prefix, ReplayLimits{})
				// Even zero records or an omitted whole final record can be
				// structurally valid. Full main extent does not prove that
				// the journal has all originals of the transaction.
				if err := plan.ValidateExtent(plan.DatabaseBytes); err != nil || plan.CountEvidence != ReplayCountEOFUnproven {
					t.Fatalf("whole-record prefix %d: evidence=%v err=%v", length, plan.CountEvidence, err)
				}
			} else {
				// Every byte cut inside the header sector, page number,
				// image, and checksum must reject without a partial plan.
				rejectEOFCountMemoryInput(t, f.data[:length], ReplayLimits{}, nil)
			}
		}
		for _, extra := range []int{1, 4, 8, 31, 32, 511, 512, 519, 520, 1040} {
			// Even an exact record's worth of zero padding is not a page.
			padded := append(bytes.Clone(f.data), make([]byte, extra)...)
			rejectEOFCountMemoryInput(t, padded, ReplayLimits{}, nil)
		}
		aligned := bytes.Clone(f.data)
		for len(aligned)%int(sector) != 0 {
			aligned = append(aligned, 0x6d)
		}
		if len(aligned) == len(f.data) {
			t.Fatal("fixture did not exercise a padding-only tail")
		}
		rejectEOFCountMemoryInput(t, aligned, ReplayLimits{}, nil)
	}
	// A 28-byte fixed header never substitutes for its full declared sector,
	// including sectors substantially larger than one database page.
	for sector := uint32(32); sector <= 65536; sector *= 2 {
		h := Header{Records: eofCountMemorySentinel, Nonce: 1, OriginalPages: 0, SectorSize: sector, PageSize: 512}
		f := makeEOFCountMemoryFixture(h)
		for _, length := range []int{0, 7, 8, 27, 28, int(sector) - 1} {
			rejectEOFCountMemoryInput(t, f.data[:length], ReplayLimits{}, ErrFormat)
		}
		checkEOFCountMemoryPlan(t, f, ReplayLimits{})
	}
}

func TestReplayEOFCountRejectsAmbiguityInMemory(t *testing.T) {
	h := Header{Records: eofCountMemorySentinel, Nonce: 19, OriginalPages: 9, SectorSize: 512, PageSize: 512}
	f := makeEOFCountMemoryFixture(h, 1, 3)
	cases := []struct {
		name string
		edit func([]byte) []byte
		want error
	}{
		{"unpublished-magic", func(b []byte) []byte { b[0] = 0; return b }, ErrFormat},
		{"zero-page", func(b []byte) []byte { binary.BigEndian.PutUint32(b[512:], 0); return b }, ErrFormat},
		{"outside-original-extent", func(b []byte) []byte { binary.BigEndian.PutUint32(b[512:], 10); return b }, ErrFormat},
		{"duplicate-page", func(b []byte) []byte { binary.BigEndian.PutUint32(b[1032:], 1); return b }, ErrReplayUnsupported},
		{"sampled-byte-first-record", func(b []byte) []byte { b[512+4+312] ^= 1; return b }, ErrFormat},
		{"sampled-byte-last-record", func(b []byte) []byte { b[1032+4+112] ^= 1; return b }, ErrFormat},
		{"checksum-first-record", func(b []byte) []byte { b[1031] ^= 1; return b }, ErrFormat},
		{"checksum-last-record", func(b []byte) []byte { b[len(b)-1] ^= 1; return b }, ErrFormat},
		{"wrong-header-nonce", func(b []byte) []byte { binary.BigEndian.PutUint32(b[12:], h.Nonce+1); return b }, ErrFormat},
		{"second-record-other-nonce", func(b []byte) []byte {
			page := b[1032+4 : 1032+4+512]
			binary.BigEndian.PutUint32(b[len(b)-4:], eofCountMemorySum(page, h.Nonce+1))
			return b
		}, ErrFormat},
		{"legacy-zero-page-size", func(b []byte) []byte { binary.BigEndian.PutUint32(b[24:], 0); return b }, ErrReplayUnsupported},
		{"too-small-page-size", func(b []byte) []byte { binary.BigEndian.PutUint32(b[24:], 256); return b }, ErrFormat},
		{"non-power-page-size", func(b []byte) []byte { binary.BigEndian.PutUint32(b[24:], 513); return b }, ErrFormat},
		{"too-large-page-size", func(b []byte) []byte { binary.BigEndian.PutUint32(b[24:], 131072); return b }, ErrFormat},
		{"zero-sector", func(b []byte) []byte { binary.BigEndian.PutUint32(b[20:], 0); return b }, ErrFormat},
		{"too-small-sector", func(b []byte) []byte { binary.BigEndian.PutUint32(b[20:], 16); return b }, ErrFormat},
		{"non-power-sector", func(b []byte) []byte { binary.BigEndian.PutUint32(b[20:], 33); return b }, ErrFormat},
		{"too-large-sector", func(b []byte) []byte { binary.BigEndian.PutUint32(b[20:], 131072); return b }, ErrFormat},
		{"invalid-original-pages", func(b []byte) []byte { binary.BigEndian.PutUint32(b[16:], 0xffffffff); return b }, ErrFormat},
		{"zero-extent-with-records", func(b []byte) []byte { binary.BigEndian.PutUint32(b[16:], 0); return b }, ErrFormat},
		{"more-records-than-original-pages", func(b []byte) []byte { binary.BigEndian.PutUint32(b[16:], 1); return b }, ErrFormat},
		{"one-trailing-byte", func(b []byte) []byte { return append(b, 0) }, nil},
		{"apparent-super-trailer", func(b []byte) []byte { return append(b, eofCountMemoryMagic[:]...) }, ErrReplayUnsupported},
		{"super-journal-footer", func(b []byte) []byte {
			name := []byte("/memory-only/example-mj012345678")
			footer := make([]byte, 4+len(name)+16)
			binary.BigEndian.PutUint32(footer, 0x40000000/h.PageSize+1)
			copy(footer[4:], name)
			binary.BigEndian.PutUint32(footer[len(footer)-16:], uint32(len(name)))
			var sum uint32
			for _, c := range name {
				sum += uint32(c)
			}
			binary.BigEndian.PutUint32(footer[len(footer)-12:], sum)
			copy(footer[len(footer)-8:], eofCountMemoryMagic[:])
			return append(b, footer...)
		}, ErrReplayUnsupported},
		{"finite-header-after-sentinel", func(b []byte) []byte {
			for len(b)%int(h.SectorSize) != 0 {
				b = append(b, 0x6d)
			}
			next := h
			next.Records, next.Nonce = 1, 31
			return append(b, makeEOFCountMemoryFixture(next, 7).data...)
		}, nil},
		{"header-shaped-whole-record-tail", func(b []byte) []byte {
			next := h
			next.Records, next.Nonce = 0, 31
			b = append(b, makeEOFCountMemoryFixture(next).data...)
			for (len(b)-int(h.SectorSize))%520 != 0 {
				b = append(b, 0)
			}
			return b
		}, ErrFormat},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rejectEOFCountMemoryInput(t, tc.edit(bytes.Clone(f.data)), ReplayLimits{}, tc.want)
		})
	}
	locking := h
	locking.OriginalPages = 0x40000000/h.PageSize + 1
	rejectEOFCountMemoryInput(t, makeEOFCountMemoryFixture(locking, locking.OriginalPages).data, ReplayLimits{}, ErrFormat)

	// Only the first header may carry the sentinel. Both a nonempty and an
	// empty finite first segment still reject a later sentinel header.
	for _, numbers := range [][]uint32{nil, {1}} {
		first := h
		first.Records = uint32(len(numbers))
		joined := makeEOFCountMemoryFixture(first, numbers...).data
		for len(joined)%int(h.SectorSize) != 0 {
			joined = append(joined, 0x6d)
		}
		joined = append(joined, makeEOFCountMemoryFixture(h, 3).data...)
		rejectEOFCountMemoryInput(t, joined, ReplayLimits{}, ErrReplayUnsupported)
	}

	t.Run("magic-inside-page-is-data", func(t *testing.T) {
		inside := h
		inside.SectorSize = 32
		g := makeEOFCountMemoryFixture(inside, 1, 3)
		// This entire apparent fixed header begins on a sector boundary,
		// but it is inside a page image and must never start a segment.
		copy(g.pages[0].Data[28:56], g.data[:28])
		copy(g.data[32+4:32+4+512], g.pages[0].Data)
		binary.BigEndian.PutUint32(g.data[32+4+512:], eofCountMemorySum(g.pages[0].Data, inside.Nonce))
		checkEOFCountMemoryPlan(t, g, ReplayLimits{})
	})
	t.Run("magic-inside-header-padding-is-data", func(t *testing.T) {
		g := makeEOFCountMemoryFixture(h, 1, 3)
		copy(g.data[64:92], g.data[:28])
		checkEOFCountMemoryPlan(t, g, ReplayLimits{})
	})
	t.Run("unsampled-change-is-not-authenticated", func(t *testing.T) {
		g := makeEOFCountMemoryFixture(h, 1, 3)
		// Offset 1 is not sampled by the 512-byte sparse checksum. Its
		// undetected change is structural decodability, not authenticity,
		// completeness, identity, durability, or native replay authority.
		g.data[512+4+1] ^= 0x80
		g.pages[0].Data[1] ^= 0x80
		checkEOFCountMemoryPlan(t, g, ReplayLimits{})
	})
}

func TestReplayEOFCountBudgetsInMemory(t *testing.T) {
	h := Header{Records: eofCountMemorySentinel, Nonce: 0xfffffff0, OriginalPages: 7, SectorSize: 512, PageSize: 512}
	f := makeEOFCountMemoryFixture(h, 7, 1, 3)
	exact := ReplayLimits{MaxJournalBytes: uint64(len(f.data)), MaxRecords: 3, MaxDatabaseBytes: 7 * 512}
	plan := checkEOFCountMemoryPlan(t, f, exact)
	var imageBytes uint64
	for _, page := range plan.Pages {
		imageBytes += uint64(len(page.Data))
	}
	if imageBytes != 3*512 || imageBytes > exact.MaxJournalBytes || imageBytes > exact.MaxDatabaseBytes {
		t.Fatalf("retained image byte bound: %d", imageBytes)
	}
	for _, limits := range []ReplayLimits{
		{MaxJournalBytes: exact.MaxJournalBytes - 1, MaxRecords: exact.MaxRecords, MaxDatabaseBytes: exact.MaxDatabaseBytes},
		{MaxJournalBytes: exact.MaxJournalBytes, MaxRecords: exact.MaxRecords - 1, MaxDatabaseBytes: exact.MaxDatabaseBytes},
		{MaxJournalBytes: exact.MaxJournalBytes, MaxRecords: exact.MaxRecords, MaxDatabaseBytes: exact.MaxDatabaseBytes - 1},
	} {
		rejectEOFCountMemoryInput(t, f.data, limits, ErrReplayLimit)
	}
	defaults := ReplayLimits{MaxJournalBytes: 64 << 20, MaxRecords: 1 << 16, MaxDatabaseBytes: 1 << 40}
	if got := (ReplayLimits{}).WithDefaults(); got != defaults {
		t.Fatalf("zero limits changed defaults: %+v", got)
	}
	checkEOFCountMemoryPlan(t, f, ReplayLimits{})
	for _, limits := range []ReplayLimits{
		{MaxJournalBytes: exact.MaxJournalBytes}, {MaxRecords: exact.MaxRecords}, {MaxDatabaseBytes: exact.MaxDatabaseBytes},
	} {
		checkEOFCountMemoryPlan(t, f, limits)
	}
	// The derived count bound must be enforced before record validation.
	// This is error-precedence evidence, not an allocation/RSS measurement.
	badPage := bytes.Clone(f.data)
	binary.BigEndian.PutUint32(badPage[512:], 0)
	rejectEOFCountMemoryInput(t, badPage, ReplayLimits{MaxRecords: 2}, ErrReplayLimit)
	rejectEOFCountMemoryInput(t, badPage, ReplayLimits{MaxDatabaseBytes: 7*512 - 1}, ErrReplayLimit)
	rejectEOFCountMemoryInput(t, []byte{}, ReplayLimits{MaxJournalBytes: 1}, ErrFormat)
	rejectEOFCountMemoryInput(t, []byte{0, 0}, ReplayLimits{MaxJournalBytes: 1}, ErrReplayLimit)

	// The raw sentinel is never mistaken for an allocation-sized count.
	// A zero or one effective record fits a tiny explicit count budget.
	for _, numbers := range [][]uint32{nil, {1}} {
		g := makeEOFCountMemoryFixture(h, numbers...)
		checkEOFCountMemoryPlan(t, g, ReplayLimits{MaxRecords: 1})
	}
	// The largest legal extent is tested without materializing a database.
	// All fixture allocations stay at one header plus at most one page;
	// extent multiplication and comparisons do not pass through int.
	large := Header{Records: eofCountMemorySentinel, Nonce: 3, OriginalPages: 0xfffffffe, SectorSize: 65536, PageSize: 65536}
	wideExtent := uint64(large.OriginalPages) * uint64(large.PageSize)
	for _, numbers := range [][]uint32{nil, {0xfffffffe}} {
		g := makeEOFCountMemoryFixture(large, numbers...)
		limits := ReplayLimits{MaxJournalBytes: uint64(len(g.data)), MaxRecords: 1, MaxDatabaseBytes: wideExtent}
		p := checkEOFCountMemoryPlan(t, g, limits)
		if uint64(p.DatabaseBytes) != wideExtent || p.DatabaseBytes <= 1<<32 {
			t.Fatalf("large extent narrowed: %d", p.DatabaseBytes)
		}
		limits.MaxDatabaseBytes--
		rejectEOFCountMemoryInput(t, g.data, limits, ErrReplayLimit)
		rejectEOFCountMemoryInput(t, g.data, ReplayLimits{}, ErrReplayLimit)
		checkEOFCountMemoryPlan(t, g, ReplayLimits{MaxRecords: 0xffffffff, MaxDatabaseBytes: ^uint64(0)})
	}
	short := makeEOFCountMemoryFixture(large)
	rejectEOFCountMemoryInput(t, short.data[:28], ReplayLimits{MaxDatabaseBytes: ^uint64(0)}, ErrFormat)
	// Preserve finite huge-count refusal too. A short header must not wrap
	// record-size multiplication into a successful small input/extent.
	finite := large
	finite.Records = 0xfffffffe
	g := makeEOFCountMemoryFixture(finite)
	rejectEOFCountMemoryInput(t, g.data, ReplayLimits{}, ErrReplayLimit)
	rejectEOFCountMemoryInput(t, g.data, ReplayLimits{MaxRecords: 0xffffffff, MaxDatabaseBytes: ^uint64(0)}, ErrFormat)
}

func TestReplayEOFCountExtentAndApplicationInMemory(t *testing.T) {
	h := Header{Records: eofCountMemorySentinel, Nonce: 4, OriginalPages: 3, SectorSize: 512, PageSize: 512}
	f := makeEOFCountMemoryFixture(h, 3, 1)
	plan := checkEOFCountMemoryPlan(t, f, ReplayLimits{})
	for _, length := range []int64{-1, 0, 1, 511, 512, 513, 1023} {
		if err := plan.ValidateExtent(length); !errors.Is(err, ErrFormat) {
			t.Fatalf("missing original coverage at extent %d: %v", length, err)
		}
	}
	for _, length := range []int64{1024, 1025, 1535, 1536, 2048} {
		if err := plan.ValidateExtent(length); err != nil {
			t.Fatalf("covered extent %d: %v", length, err)
		}
	}
	apply := func(p *ReplayPlan, in []byte) []byte {
		if err := p.ValidateExtent(int64(len(in))); err != nil {
			t.Fatalf("invalid memory-model input: %v", err)
		}
		out := make([]byte, int(p.DatabaseBytes))
		copy(out, in)
		for _, page := range p.Pages {
			start := int(page.Number-1) * int(p.Header.PageSize)
			copy(out[start:start+int(p.Header.PageSize)], page.Data)
		}
		return out
	}
	for _, length := range []int{1024, 1025, 1535, 1536, 2048} {
		current := bytes.Repeat([]byte{0x72}, length)
		before := bytes.Clone(current)
		restored := apply(plan, current)
		if len(restored) != 1536 || !bytes.Equal(restored[512:1024], current[512:1024]) ||
			!bytes.Equal(restored[1024:1536], f.pages[0].Data) || !bytes.Equal(restored[:512], f.pages[1].Data) ||
			!bytes.Equal(restored, apply(plan, restored)) || !bytes.Equal(current, before) {
			t.Fatalf("extent %d: independent page/unaffected/truncation/idempotence model differs", length)
		}
		restored[0] ^= 0xff
		if !bytes.Equal(plan.Pages[1].Data, f.pages[1].Data) || !bytes.Equal(current, before) {
			t.Fatal("memory application output aliases plan or input")
		}
	}
	all := makeEOFCountMemoryFixture(h, 3, 1, 2)
	allPlan := checkEOFCountMemoryPlan(t, all, ReplayLimits{})
	fromEmpty := apply(allPlan, nil)
	for _, page := range all.pages {
		start := int(page.Number-1) * 512
		if !bytes.Equal(fromEmpty[start:start+512], page.Data) {
			t.Fatal("all-image coverage failed from an empty byte array")
		}
	}
	// Removing the final whole record also removes evidence of that
	// original page. A full-length current byte array still passes extent
	// validation, so neither that result nor parsing proves completeness.
	shorter := eofCountMemoryFixture{header: h, data: f.data[:512+520], pages: f.pages[:1]}
	shortPlan := checkEOFCountMemoryPlan(t, shorter, ReplayLimits{})
	if err := shortPlan.ValidateExtent(1536); err != nil || shortPlan.CountEvidence != ReplayCountEOFUnproven {
		t.Fatalf("whole-record shortening: evidence=%v err=%v", shortPlan.CountEvidence, err)
	}
	if err := shortPlan.ValidateExtent(0); !errors.Is(err, ErrFormat) {
		t.Fatalf("shortened journal unexpectedly covers an empty extent: %v", err)
	}
	headerOnly := checkEOFCountMemoryPlan(t, makeEOFCountMemoryFixture(h), ReplayLimits{})
	for _, length := range []int64{1536, 2048} {
		if err := headerOnly.ValidateExtent(length); err != nil {
			t.Fatal(err)
		}
	}
	for _, length := range []int64{0, 1535} {
		if err := headerOnly.ValidateExtent(length); !errors.Is(err, ErrFormat) {
			t.Fatalf("header-only missing coverage at %d: %v", length, err)
		}
	}
	emptyHeader := h
	emptyHeader.OriginalPages = 0
	empty := checkEOFCountMemoryPlan(t, makeEOFCountMemoryFixture(emptyHeader), ReplayLimits{})
	for _, current := range [][]byte{nil, {0x72}, bytes.Repeat([]byte{0x72}, 4096)} {
		if got := apply(empty, current); len(got) != 0 || empty.CountEvidence != ReplayCountEOFUnproven {
			t.Fatal("original empty extent did not truncate the memory model to zero")
		}
	}
	// The unused locking page is the sole absent original page here. No
	// billion-byte array is allocated and no physical extent is inspected.
	locking := h
	locking.OriginalPages = 0x40000000/h.PageSize + 1
	lockPlan := checkEOFCountMemoryPlan(t, makeEOFCountMemoryFixture(locking), ReplayLimits{})
	if err := lockPlan.ValidateExtent(0x40000000); err != nil {
		t.Fatalf("unused locking-page coverage: %v", err)
	}
	if err := lockPlan.ValidateExtent(0x40000000 - 1); !errors.Is(err, ErrFormat) {
		t.Fatalf("partially absent page before locking page: %v", err)
	}
}

func TestReplayEOFCountFiniteEquivalenceInMemory(t *testing.T) {
	for pageSize := uint32(512); pageSize <= 65536; pageSize *= 2 {
		for sectorSize := uint32(32); sectorSize <= 65536; sectorSize *= 2 {
			for _, numbers := range [][]uint32{nil, {3}, {7, 2, 1}} {
				h := Header{Records: uint32(len(numbers)), Nonce: 0xfffffff0, OriginalPages: 7, SectorSize: sectorSize, PageSize: pageSize}
				finite := makeEOFCountMemoryFixture(h, numbers...)
				finitePlan := checkEOFCountMemoryPlan(t, finite, ReplayLimits{})
				eof := eofCountMemoryFixture{header: h, data: bytes.Clone(finite.data), pages: finite.pages}
				eof.header.Records = eofCountMemorySentinel
				binary.BigEndian.PutUint32(eof.data[8:], eofCountMemorySentinel)
				eofPlan := checkEOFCountMemoryPlan(t, eof, ReplayLimits{})
				if !bytes.Equal(finite.data[:8], eof.data[:8]) || !bytes.Equal(finite.data[12:], eof.data[12:]) ||
					finitePlan.Header.Records != uint32(len(numbers)) || eofPlan.Header.Records != eofCountMemorySentinel ||
					finitePlan.CountEvidence != ReplayCountFiniteHeaders || eofPlan.CountEvidence != ReplayCountEOFUnproven ||
					finitePlan.DatabaseBytes != eofPlan.DatabaseBytes || !reflect.DeepEqual(finitePlan.Pages, eofPlan.Pages) {
					t.Fatalf("finite/EOF raw-count, evidence, page, or extent distinction lost: page=%d sector=%d records=%d", pageSize, sectorSize, len(numbers))
				}
			}
		}
	}
	// Existing finite multi-segment decoding still preserves first-header
	// count/nonce and all image order. Later nonces are independently used;
	// no normalized EOF header or cumulative raw count is substituted.
	for _, sector := range []uint32{32, 512, 65536} {
		firstHeader := Header{Records: 2, Nonce: 11, OriginalPages: 7, SectorSize: sector, PageSize: 512}
		first := makeEOFCountMemoryFixture(firstHeader, 7, 2)
		secondHeader := firstHeader
		secondHeader.Records, secondHeader.Nonce = 0, 99
		second := makeEOFCountMemoryFixture(secondHeader)
		thirdHeader := firstHeader
		thirdHeader.Records, thirdHeader.Nonce = 1, 0xfffffff0
		third := makeEOFCountMemoryFixture(thirdHeader, 1)
		joined := eofCountMemoryFixture{header: firstHeader}
		for _, segment := range []eofCountMemoryFixture{first, second, third} {
			for len(joined.data)%int(sector) != 0 {
				joined.data = append(joined.data, 0xa6)
			}
			joined.data = append(joined.data, segment.data...)
			joined.pages = append(joined.pages, segment.pages...)
		}
		plan := checkEOFCountMemoryPlan(t, joined, ReplayLimits{})
		if plan.Header.Records != 2 || plan.Header.Nonce != 11 || len(plan.Pages) != 3 || plan.CountEvidence != ReplayCountFiniteHeaders {
			t.Fatalf("finite multi-segment representation changed: %+v", plan)
		}
	}
}
