package journal

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

// All fixtures are byte slices only. These tests never create a database,
// sidecar, crash, physical error, process-exit, or recovery-on-disk workflow.
func memoryJournal(h Header, numbers ...uint32) []byte {
	data := make([]byte, int(h.SectorSize)+len(numbers)*(int(h.PageSize)+8))
	copy(data, []byte{0xd9, 0xd5, 0x05, 0xf9, 0x20, 0xa1, 0x63, 0xd7})
	for off, value := range map[int]uint32{8: h.Records, 12: h.Nonce, 16: h.OriginalPages, 20: h.SectorSize, 24: h.PageSize} {
		binary.BigEndian.PutUint32(data[off:], value)
	}
	for i, number := range numbers {
		off := int(h.SectorSize) + i*(int(h.PageSize)+8)
		binary.BigEndian.PutUint32(data[off:], number)
		page := data[off+4 : off+4+int(h.PageSize)]
		for j := range page {
			page[j] = byte(int(number) + j)
		}
		// Independent direct implementation of the documented sparse sum.
		sum := h.Nonce
		for j := int(h.PageSize) - 200; j > 0; j -= 200 {
			sum += uint32(page[j])
		}
		binary.BigEndian.PutUint32(data[off+4+int(h.PageSize):], sum)
	}
	return data
}

func TestReplayPlanGeometryInMemory(t *testing.T) {
	for pageSize := uint32(512); pageSize <= 65536; pageSize *= 2 {
		for sectorSize := uint32(32); sectorSize <= 65536; sectorSize *= 2 {
			h := Header{Records: 2, Nonce: 0xfffffff0, OriginalPages: 7, SectorSize: sectorSize, PageSize: pageSize}
			data := memoryJournal(h, 3, 1)
			// Header-sector padding is unspecified, not an extra record.
			for i := 28; i < int(sectorSize); i++ {
				data[i] = 0xa5
			}
			plan, err := ParseReplay(data, ReplayLimits{})
			if err != nil {
				t.Fatalf("page=%d sector=%d: %v", pageSize, sectorSize, err)
			}
			if plan.Header != h || plan.DatabaseBytes != int64(7*pageSize) || len(plan.Pages) != 2 || plan.Pages[0].Number != 3 || plan.Pages[1].Number != 1 {
				t.Fatalf("wrong plan: %+v", plan)
			}
			want := append([]byte(nil), data[int(sectorSize)+4:int(sectorSize)+4+int(pageSize)]...)
			if !bytes.Equal(plan.Pages[0].Data, want) {
				t.Fatal("original image mismatch")
			}
			data[int(sectorSize)+4] ^= 1
			if !bytes.Equal(plan.Pages[0].Data, want) {
				t.Fatal("plan aliases input")
			}
			plan.Pages[0].Data[0] ^= 2
			if plan.Pages[1].Data[0] != 1 {
				t.Fatal("page images alias one another")
			}
		}
	}
}

func TestReplayRejectsEveryTruncationInMemory(t *testing.T) {
	h := Header{Records: 2, Nonce: 19, OriginalPages: 3, SectorSize: 512, PageSize: 512}
	data := memoryJournal(h, 1, 3)
	for length := range len(data) {
		plan, err := ParseReplay(data[:length], ReplayLimits{})
		if plan != nil || !errors.Is(err, ErrFormat) {
			t.Fatalf("length %d returned plan=%v err=%v", length, plan, err)
		}
	}
}

func TestReplayRejectsAmbiguousInputInMemory(t *testing.T) {
	h := Header{Records: 2, Nonce: 19, OriginalPages: 3, SectorSize: 512, PageSize: 512}
	base := memoryJournal(h, 1, 3)
	cases := []struct {
		name string
		edit func([]byte) []byte
		want error
	}{
		{"magic", func(b []byte) []byte { b[0] = 0; return b }, ErrFormat},
		{"zero-page", func(b []byte) []byte { binary.BigEndian.PutUint32(b[512:], 0); return b }, ErrFormat},
		{"beyond-original", func(b []byte) []byte { binary.BigEndian.PutUint32(b[512:], 4); return b }, ErrFormat},
		{"checksum", func(b []byte) []byte { b[512+4+312] ^= 1; return b }, ErrFormat},
		{"duplicate", func(b []byte) []byte { binary.BigEndian.PutUint32(b[512+520:], 1); return b }, ErrReplayUnsupported},
		{"no-sync", func(b []byte) []byte { binary.BigEndian.PutUint32(b[8:], 0xffffffff); return b }, ErrReplayUnsupported},
		{"legacy-page-size", func(b []byte) []byte { binary.BigEndian.PutUint32(b[24:], 0); return b }, ErrReplayUnsupported},
		{"odd-page-size", func(b []byte) []byte { binary.BigEndian.PutUint32(b[24:], 513); return b }, ErrFormat},
		{"small-page-size", func(b []byte) []byte { binary.BigEndian.PutUint32(b[24:], 256); return b }, ErrFormat},
		{"large-page-size", func(b []byte) []byte { binary.BigEndian.PutUint32(b[24:], 131072); return b }, ErrFormat},
		{"small-sector", func(b []byte) []byte { binary.BigEndian.PutUint32(b[20:], 16); return b }, ErrFormat},
		{"odd-sector", func(b []byte) []byte { binary.BigEndian.PutUint32(b[20:], 513); return b }, ErrFormat},
		{"large-sector", func(b []byte) []byte { binary.BigEndian.PutUint32(b[20:], 131072); return b }, ErrFormat},
		{"invalid-db-pages", func(b []byte) []byte { binary.BigEndian.PutUint32(b[16:], 0xffffffff); return b }, ErrFormat},
		{"too-many-records", func(b []byte) []byte { binary.BigEndian.PutUint32(b[16:], 1); return b }, ErrFormat},
		{"trailing-byte", func(b []byte) []byte { return append(b, 0) }, ErrReplayUnsupported},
		{"unpublished-extra-record", func(b []byte) []byte { binary.BigEndian.PutUint32(b[8:], 1); return b }, ErrReplayUnsupported},
		{"zero-count-with-body", func(b []byte) []byte { binary.BigEndian.PutUint32(b[8:], 0); return b }, ErrReplayUnsupported},
		{"second-aligned-segment", func(b []byte) []byte {
			b = append(b, make([]byte, (512-len(b)%512)%512)...)
			return append(b, memoryJournal(h, 1, 3)...)
		}, ErrReplayUnsupported},
		{"super-journal", func(b []byte) []byte {
			name := []byte("/example/database-mjABCDEF9AB")
			footer := make([]byte, 4+len(name)+16)
			binary.BigEndian.PutUint32(footer, 0x40000000/h.PageSize+1)
			copy(footer[4:], name)
			binary.BigEndian.PutUint32(footer[len(footer)-16:], uint32(len(name)))
			var sum uint32
			for _, v := range name {
				sum += uint32(v)
			}
			binary.BigEndian.PutUint32(footer[len(footer)-12:], sum)
			copy(footer[len(footer)-8:], magic[:])
			return append(b, footer...)
		}, ErrReplayUnsupported},
		{"malformed-super-trailer", func(b []byte) []byte { return append(b, magic[:]...) }, ErrReplayUnsupported},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := tc.edit(append([]byte(nil), base...))
			plan, err := ParseReplay(data, ReplayLimits{})
			if plan != nil || !errors.Is(err, tc.want) {
				t.Fatalf("plan=%v err=%v want=%v", plan, err, tc.want)
			}
		})
	}
	lock := h
	lock.OriginalPages = 0x40000000/h.PageSize + 1
	lock.Records = 1
	if plan, err := ParseReplay(memoryJournal(lock, lock.OriginalPages), ReplayLimits{}); plan != nil || !errors.Is(err, ErrFormat) {
		t.Fatalf("locking page plan=%v err=%v", plan, err)
	}
}

func TestReplayLimitsAndZeroRecordsInMemory(t *testing.T) {
	h := Header{Records: 2, Nonce: 19, OriginalPages: 3, SectorSize: 512, PageSize: 512}
	data := memoryJournal(h, 1, 3)
	for _, limits := range []ReplayLimits{
		{MaxJournalBytes: uint64(len(data) - 1)}, {MaxRecords: 1}, {MaxDatabaseBytes: 1535},
	} {
		if plan, err := ParseReplay(data, limits); plan != nil || !errors.Is(err, ErrReplayLimit) {
			t.Fatalf("limits=%+v plan=%v err=%v", limits, plan, err)
		}
	}
	if _, err := ParseReplay(data, ReplayLimits{MaxJournalBytes: uint64(len(data)), MaxRecords: 2, MaxDatabaseBytes: 1536}); err != nil {
		t.Fatal(err)
	}
	for _, pages := range []uint32{0, 3, 0xfffffffe} {
		h.Records, h.OriginalPages = 0, pages
		plan, err := ParseReplay(memoryJournal(h), ReplayLimits{MaxDatabaseBytes: ^uint64(0)})
		if err != nil || len(plan.Pages) != 0 || plan.DatabaseBytes != int64(pages)*512 {
			t.Fatalf("zero records pages=%d plan=%v err=%v", pages, plan, err)
		}
	}
	// Huge advertised counts fail the bound before record allocation or scan.
	h.Records, h.OriginalPages = 0xfffffffe, 0xfffffffe
	if _, err := ParseReplay(memoryJournal(h), ReplayLimits{}); !errors.Is(err, ErrReplayLimit) {
		t.Fatal(err)
	}
}

func TestReplayExtentAndApplicationInMemory(t *testing.T) {
	h := Header{Records: 2, Nonce: 4, OriginalPages: 3, SectorSize: 512, PageSize: 512}
	plan, err := ParseReplay(memoryJournal(h, 1, 3), ReplayLimits{})
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []int64{1024, 1025, 1535, 1536, 2048} {
		if err := plan.ValidateExtent(n); err != nil {
			t.Fatalf("extent %d: %v", n, err)
		}
	}
	for _, n := range []int64{-1, 0, 512, 1023} {
		if err := plan.ValidateExtent(n); !errors.Is(err, ErrFormat) {
			t.Fatalf("extent %d: %v", n, err)
		}
	}
	// Model the replay solely in memory, preserving unjournaled page 2 and
	// removing a newly appended page 4. Reapplying the plan is idempotent.
	current := bytes.Repeat([]byte{0x72}, 2048)
	apply := func(in []byte) []byte {
		out := make([]byte, plan.DatabaseBytes)
		copy(out, in)
		for _, p := range plan.Pages {
			copy(out[int(p.Number-1)*512:], p.Data)
		}
		return out
	}
	restored := apply(current)
	if len(restored) != 1536 || !bytes.Equal(restored[512:1024], current[512:1024]) ||
		!bytes.Equal(restored[:512], plan.Pages[0].Data) || !bytes.Equal(restored[1024:], plan.Pages[1].Data) ||
		!bytes.Equal(restored, apply(restored)) {
		t.Fatal("original page, unaffected page, truncation, or idempotence mismatch")
	}
	h.Records, h.OriginalPages = 0, 0
	empty, _ := ParseReplay(memoryJournal(h), ReplayLimits{})
	if err := empty.ValidateExtent(4096); err != nil {
		t.Fatal(err)
	}
	// SQLite's unused locking page does not need an original image.
	h.OriginalPages = 0x40000000/h.PageSize + 1
	lockOnly, _ := ParseReplay(memoryJournal(h), ReplayLimits{})
	if err := lockOnly.ValidateExtent(0x40000000); err != nil {
		t.Fatal(err)
	}
}

func FuzzReplayPlanBytesOnly(f *testing.F) {
	h := Header{Records: 1, Nonce: 7, OriginalPages: 1, SectorSize: 512, PageSize: 512}
	f.Add(memoryJournal(h, 1))
	f.Add([]byte{})
	f.Add([]byte{0xd9, 0xd5})
	f.Fuzz(func(t *testing.T, data []byte) {
		plan, err := ParseReplay(data, ReplayLimits{MaxJournalBytes: 1 << 20, MaxRecords: 1024, MaxDatabaseBytes: 1 << 30})
		if err != nil {
			if plan != nil {
				t.Fatal("error returned a partial plan")
			}
			return
		}
		if int(plan.Header.Records) > len(plan.Pages) || len(plan.Pages) > 1024 || plan.DatabaseBytes != int64(plan.Header.OriginalPages)*int64(plan.Header.PageSize) {
			t.Fatal("invalid successful plan")
		}
		if err := plan.ValidateExtent(plan.DatabaseBytes); err != nil {
			t.Fatal(err)
		}
	})
}
