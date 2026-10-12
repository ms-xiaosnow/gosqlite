package journal

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math/rand"
	"reflect"
	"testing"
)

// All helpers and tests in this file operate solely on byte arrays. They do
// not use the production journal encoders, any pager, database, filesystem,
// native fault hook, or external SQLite process.
type replayMemorySegment struct {
	nonce   uint32
	numbers []uint32
}

type replayMemoryFixture struct {
	data    []byte
	headers []int
	ends    []int
	first   Header
	pages   []PageImage
}

func makeReplayMemoryFixture(geometry Header, segments ...replayMemorySegment) replayMemoryFixture {
	var f replayMemoryFixture
	f.first = geometry
	f.first.Records = uint32(len(segments[0].numbers))
	f.first.Nonce = segments[0].nonce
	for si, segment := range segments {
		// Independent alignment model: append bytes until divisible, rather
		// than using the parser's rounding expression.
		for len(f.data)%int(geometry.SectorSize) != 0 {
			f.data = append(f.data, 0xa6)
		}
		off := len(f.data)
		f.headers = append(f.headers, off)
		header := bytes.Repeat([]byte{0x5b}, int(geometry.SectorSize))
		copy(header, []byte{0xd9, 0xd5, 0x05, 0xf9, 0x20, 0xa1, 0x63, 0xd7})
		for i, value := range []uint32{uint32(len(segment.numbers)), segment.nonce, geometry.OriginalPages, geometry.SectorSize, geometry.PageSize} {
			binary.BigEndian.PutUint32(header[8+4*i:], value)
		}
		f.data = append(f.data, header...)
		for ri, number := range segment.numbers {
			page := make([]byte, geometry.PageSize)
			for i := range page {
				page[i] = byte(i*7 + int(number)*13 + si*17 + ri*23)
			}
			record := make([]byte, len(page)+8)
			binary.BigEndian.PutUint32(record, number)
			copy(record[4:], page)
			// Equivalent mathematical checksum using positive strides from
			// the page end, without the production Checksum function.
			sum := uint64(segment.nonce)
			for stride := 200; stride < len(page); stride += 200 {
				sum += uint64(page[len(page)-stride])
			}
			binary.BigEndian.PutUint32(record[len(record)-4:], uint32(sum))
			f.data = append(f.data, record...)
			f.pages = append(f.pages, PageImage{number, page})
		}
		f.ends = append(f.ends, len(f.data))
	}
	return f
}

func checkReplayMemoryPlan(t *testing.T, f replayMemoryFixture, limits ReplayLimits) *ReplayPlan {
	t.Helper()
	plan, err := ParseReplay(f.data, limits)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Header != f.first || plan.DatabaseBytes != int64(f.first.OriginalPages)*int64(f.first.PageSize) || len(plan.Pages) != len(f.pages) {
		t.Fatalf("plan header/extent/count: %+v, first=%+v pages=%d", plan, f.first, len(f.pages))
	}
	for i, want := range f.pages {
		if plan.Pages[i].Number != want.Number || !bytes.Equal(plan.Pages[i].Data, want.Data) {
			t.Fatalf("page %d differs from independent image model", i)
		}
	}
	return plan
}

func TestReplaySegmentsGeometryAndOwnershipInMemory(t *testing.T) {
	for page := uint32(512); page <= 65536; page *= 2 {
		for sector := uint32(32); sector <= 65536; sector *= 2 {
			f := makeReplayMemoryFixture(Header{PageSize: page, SectorSize: sector, OriginalPages: 7},
				replayMemorySegment{0xfffffff0, []uint32{7, 2}},
				replayMemorySegment{71, nil},
				replayMemorySegment{0, []uint32{1}},
				replayMemorySegment{0xdeadbeef, []uint32{4, 6}})
			plan := checkReplayMemoryPlan(t, f, ReplayLimits{})
			// A completely unrelated edit to every input byte leaves each
			// result intact; result edits cannot reach input or other pages.
			for i := range f.data {
				f.data[i] ^= 0xff
			}
			for i := range plan.Pages {
				if !bytes.Equal(plan.Pages[i].Data, f.pages[i].Data) {
					t.Fatal("owned image aliases input")
				}
			}
			inputCopy := bytes.Clone(f.data)
			for i := range plan.Pages[0].Data {
				plan.Pages[0].Data[i] ^= 0x3c
			}
			if !bytes.Equal(inputCopy, f.data) || !bytes.Equal(plan.Pages[1].Data, f.pages[1].Data) {
				t.Fatal("owned image aliases input or another page")
			}
		}
	}
}

func TestReplaySegmentsZeroCountsAndAlignedHeadersInMemory(t *testing.T) {
	// Empty first/final headers are explicit published segments. Equal
	// nonces are legal too: randomness is not a uniqueness/authentication law.
	f := makeReplayMemoryFixture(Header{PageSize: 512, SectorSize: 32, OriginalPages: 6},
		replayMemorySegment{9, nil}, replayMemorySegment{9, []uint32{1, 2, 3, 4}},
		replayMemorySegment{9, []uint32{5, 6}}, replayMemorySegment{9, nil})
	if f.headers[1] != f.ends[0] || f.headers[2] != f.ends[1] {
		t.Fatal("fixture does not exercise an already-aligned next header")
	}
	p := checkReplayMemoryPlan(t, f, ReplayLimits{})
	if p.Header.Records != 0 || len(p.Pages) != 6 || p.ValidateExtent(0) != nil {
		t.Fatal("first header/cumulative count or all-original coverage differs")
	}
	// Neither padding contents nor magic found away from the prescribed
	// header boundary changes segment recognition.
	f = makeReplayMemoryFixture(Header{PageSize: 512, SectorSize: 512, OriginalPages: 2},
		replayMemorySegment{1, []uint32{1}}, replayMemorySegment{2, []uint32{2}})
	copy(f.data[f.ends[0]:], []byte{0xd9, 0xd5, 0x05, 0xf9, 0x20, 0xa1, 0x63, 0xd7})
	checkReplayMemoryPlan(t, f, ReplayLimits{})
}

func TestReplaySegmentsExactBoundaryTruncationsInMemory(t *testing.T) {
	f := makeReplayMemoryFixture(Header{PageSize: 512, SectorSize: 512, OriginalPages: 4},
		replayMemorySegment{11, []uint32{4}}, replayMemorySegment{22, nil},
		replayMemorySegment{33, []uint32{1, 3}})
	ends := map[int]int{f.ends[0]: 1, f.ends[1]: 1, f.ends[2]: 3}
	for length := 0; length <= len(f.data); length++ {
		plan, err := ParseReplay(f.data[:length], ReplayLimits{})
		if count, boundary := ends[length]; boundary {
			// An exact earlier segment end is itself a complete supported
			// input; a parser cannot infer an omitted subsequent segment.
			if err != nil || len(plan.Pages) != count {
				t.Fatalf("complete prefix %d: plan=%v err=%v", length, plan, err)
			}
		} else if err == nil || plan != nil {
			t.Fatalf("partial prefix %d: plan=%v err=%v", length, plan, err)
		}
	}
}

func TestReplaySegmentsRejectAmbiguityInMemory(t *testing.T) {
	geometry := Header{PageSize: 512, SectorSize: 512, OriginalPages: 9}
	f := makeReplayMemoryFixture(geometry, replayMemorySegment{11, []uint32{1, 3}}, replayMemorySegment{22, []uint32{7, 9}})
	next, body := f.headers[1], f.headers[1]+512
	cases := []struct {
		name string
		edit func([]byte) []byte
		want error
	}{
		{"later-unpublished-magic", func(b []byte) []byte { b[next] = 0; return b }, ErrReplayUnsupported},
		{"later-unpublished-zero-body", func(b []byte) []byte { binary.BigEndian.PutUint32(b[next+8:], 0); return b }, ErrReplayUnsupported},
		{"later-no-sync", func(b []byte) []byte { binary.BigEndian.PutUint32(b[next+8:], 0xffffffff); return b }, ErrReplayUnsupported},
		{"later-page-size", func(b []byte) []byte { binary.BigEndian.PutUint32(b[next+24:], 1024); return b }, ErrReplayUnsupported},
		{"later-legacy-size", func(b []byte) []byte { binary.BigEndian.PutUint32(b[next+24:], 0); return b }, ErrReplayUnsupported},
		{"later-sector-size", func(b []byte) []byte { binary.BigEndian.PutUint32(b[next+20:], 32); return b }, ErrReplayUnsupported},
		{"later-original-extent", func(b []byte) []byte { binary.BigEndian.PutUint32(b[next+16:], 10); return b }, ErrReplayUnsupported},
		{"later-extent-invalid", func(b []byte) []byte { binary.BigEndian.PutUint32(b[next+16:], 0xffffffff); return b }, ErrReplayUnsupported},
		{"later-zero-page", func(b []byte) []byte { binary.BigEndian.PutUint32(b[body:], 0); return b }, ErrFormat},
		{"later-outside-first-extent", func(b []byte) []byte { binary.BigEndian.PutUint32(b[body:], 10); return b }, ErrFormat},
		{"duplicate-across-segments", func(b []byte) []byte { binary.BigEndian.PutUint32(b[body:], 1); return b }, ErrReplayUnsupported},
		{"duplicate-within-later-segment", func(b []byte) []byte { binary.BigEndian.PutUint32(b[body+520:], 7); return b }, ErrReplayUnsupported},
		{"wrong-later-nonce", func(b []byte) []byte { binary.BigEndian.PutUint32(b[next+12:], 11); return b }, ErrFormat},
		{"last-record-checksum", func(b []byte) []byte { b[len(b)-1] ^= 1; return b }, ErrFormat},
		{"last-record-sampled-byte", func(b []byte) []byte { b[body+520+4+312] ^= 1; return b }, ErrFormat},
		{"later-advertised-truncated-record", func(b []byte) []byte { binary.BigEndian.PutUint32(b[next+8:], 3); return b }, ErrFormat},
		{"trailing-one-byte", func(b []byte) []byte { return append(b, 0) }, ErrReplayUnsupported},
		{"trailing-padding-only", func(b []byte) []byte {
			for len(b)%512 != 0 {
				b = append(b, 0xa6)
			}
			return b
		}, ErrReplayUnsupported},
		{"trailing-unpublished-header", func(b []byte) []byte {
			for len(b)%512 != 0 {
				b = append(b, 0xa6)
			}
			return append(b, make([]byte, 512)...)
		}, ErrReplayUnsupported},
		{"unaligned-next-header", func(b []byte) []byte { return append(b[:f.ends[0]], b[next:]...) }, ErrReplayUnsupported},
		{"magic-away-from-aligned-header", func(b []byte) []byte {
			b[next] = 0
			copy(b[f.ends[0]:], []byte{0xd9, 0xd5, 0x05, 0xf9, 0x20, 0xa1, 0x63, 0xd7})
			return b
		}, ErrReplayUnsupported},
		{"apparent-super-journal-trailer", func(b []byte) []byte { return append(b, []byte{0xd9, 0xd5, 0x05, 0xf9, 0x20, 0xa1, 0x63, 0xd7}...) }, ErrReplayUnsupported},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := tc.edit(bytes.Clone(f.data))
			unchanged := bytes.Clone(data)
			plan, err := ParseReplay(data, ReplayLimits{})
			if plan != nil || !errors.Is(err, tc.want) || !bytes.Equal(data, unchanged) {
				t.Fatalf("partial plan, wrong error or changed input: plan=%v err=%v want=%v", plan, err, tc.want)
			}
		})
	}
	// Locking page is invalid even in a later segment.
	geometry.OriginalPages = 0x40000000/geometry.PageSize + 1
	locked := makeReplayMemoryFixture(geometry, replayMemorySegment{11, []uint32{1}}, replayMemorySegment{22, []uint32{geometry.OriginalPages}})
	if p, err := ParseReplay(locked.data, ReplayLimits{}); p != nil || !errors.Is(err, ErrFormat) {
		t.Fatalf("locking page: plan=%v err=%v", p, err)
	}
}

func TestReplaySegmentsCumulativeBudgetsInMemory(t *testing.T) {
	f := makeReplayMemoryFixture(Header{PageSize: 512, SectorSize: 512, OriginalPages: 7},
		replayMemorySegment{1, []uint32{1, 3}}, replayMemorySegment{2, []uint32{5, 7}})
	exact := ReplayLimits{MaxJournalBytes: uint64(len(f.data)), MaxRecords: 4, MaxDatabaseBytes: 7 * 512}
	plan := checkReplayMemoryPlan(t, f, exact)
	var imageBytes uint64
	for _, page := range plan.Pages {
		imageBytes += uint64(len(page.Data))
	}
	if imageBytes != 4*512 || imageBytes > exact.MaxJournalBytes || imageBytes > exact.MaxDatabaseBytes {
		t.Fatalf("retained image bound: %d", imageBytes)
	}
	for _, limits := range []ReplayLimits{
		{MaxJournalBytes: uint64(len(f.data) - 1)}, {MaxRecords: 3}, {MaxDatabaseBytes: 7*512 - 1},
	} {
		if p, err := ParseReplay(f.data, limits); p != nil || !errors.Is(err, ErrReplayLimit) {
			t.Fatalf("cumulative quota=%+v plan=%v err=%v", limits, p, err)
		}
	}
	// Both advertised counts individually fit MaxRecords; their sum must
	// not wrap a uint32 or reach any allocation based on that bogus count.
	huge := bytes.Clone(f.data)
	for _, header := range f.headers {
		binary.BigEndian.PutUint32(huge[header+16:], 0xfffffffe)
	}
	binary.BigEndian.PutUint32(huge[f.headers[1]+8:], 0xfffffffe)
	if p, err := ParseReplay(huge, ReplayLimits{MaxRecords: 0xffffffff, MaxDatabaseBytes: ^uint64(0)}); p != nil || !errors.Is(err, ErrReplayLimit) {
		t.Fatalf("cumulative uint32 overflow: plan=%v err=%v", p, err)
	}
	// Maximal geometry/finite count multiplication has to fail as short
	// input, rather than wrap into a small accepted extent or allocation.
	large := makeReplayMemoryFixture(Header{PageSize: 65536, SectorSize: 65536, OriginalPages: 0xfffffffe}, replayMemorySegment{1, nil})
	binary.BigEndian.PutUint32(large.data[8:], 0xfffffffe)
	if p, err := ParseReplay(large.data, ReplayLimits{MaxRecords: 0xffffffff, MaxDatabaseBytes: ^uint64(0)}); p != nil || !errors.Is(err, ErrFormat) {
		t.Fatalf("record byte overflow: plan=%v err=%v", p, err)
	}
	// Zero-record segments consume no record budget, but still consume
	// sector-sized bounded input and cannot create an unbounded header list.
	segments := make([]replayMemorySegment, 257)
	for i := range segments {
		segments[i].nonce = uint32(i)
	}
	empty := makeReplayMemoryFixture(Header{PageSize: 512, SectorSize: 32}, segments...)
	checkReplayMemoryPlan(t, empty, ReplayLimits{MaxJournalBytes: uint64(len(empty.data)), MaxRecords: 1})
	if p, err := ParseReplay(empty.data, ReplayLimits{MaxJournalBytes: uint64(len(empty.data) - 1)}); p != nil || !errors.Is(err, ErrReplayLimit) {
		t.Fatalf("zero-record segment input budget: plan=%v err=%v", p, err)
	}
}

func TestReplaySegmentsExactApplicationModelInMemory(t *testing.T) {
	rng := rand.New(rand.NewSource(3540000))
	for iteration := 0; iteration < 128; iteration++ {
		pageSize := 512 << rng.Intn(4)
		pageCount := 1 + rng.Intn(20)
		order := rng.Perm(pageCount)
		chosen := rng.Intn(pageCount + 1)
		segments := make([]replayMemorySegment, 1+rng.Intn(8))
		for i := range segments {
			segments[i].nonce = rng.Uint32()
		}
		// Assign each selected page exactly once to a randomly selected
		// segment, without depending on parser traversal or checksum code.
		for _, number := range order[:chosen] {
			s := rng.Intn(len(segments))
			segments[s].numbers = append(segments[s].numbers, uint32(number+1))
		}
		f := makeReplayMemoryFixture(Header{PageSize: uint32(pageSize), SectorSize: uint32(32 << rng.Intn(7)), OriginalPages: uint32(pageCount)}, segments...)
		plan := checkReplayMemoryPlan(t, f, ReplayLimits{})
		current := bytes.Repeat([]byte{0x71}, (pageCount+2)*pageSize)
		want := bytes.Clone(current[:pageCount*pageSize])
		coverage := make(map[uint32]bool)
		for _, p := range f.pages {
			copy(want[int(p.Number-1)*pageSize:int(p.Number)*pageSize], p.Data)
			coverage[p.Number] = true
		}
		apply := func(in []byte) []byte {
			out := make([]byte, int(plan.DatabaseBytes))
			copy(out, in)
			for _, p := range plan.Pages {
				copy(out[int(p.Number-1)*pageSize:int(p.Number)*pageSize], p.Data)
			}
			return out
		}
		if got := apply(current); !bytes.Equal(got, want) || !bytes.Equal(apply(got), want) {
			t.Fatalf("iteration %d: exact image/unaffected/truncation/idempotence model differs", iteration)
		}
		for fullPages := 0; fullPages <= pageCount+1; fullPages++ {
			for _, extra := range []int{0, pageSize - 1} {
				length := fullPages*pageSize + extra
				complete := true
				for number := fullPages + 1; number <= pageCount; number++ {
					complete = complete && coverage[uint32(number)]
				}
				if err := plan.ValidateExtent(int64(length)); (err == nil) != complete {
					t.Fatalf("iteration %d extent %d: complete=%v err=%v", iteration, length, complete, err)
				}
			}
		}
	}
}

func FuzzReplaySegmentsBytesOnly(f *testing.F) {
	fixture := makeReplayMemoryFixture(Header{PageSize: 512, SectorSize: 32, OriginalPages: 4},
		replayMemorySegment{0xffffffff, []uint32{4}}, replayMemorySegment{0, nil}, replayMemorySegment{7, []uint32{1, 3}})
	f.Add(fixture.data)
	f.Add(fixture.data[:fixture.ends[0]])
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		const inputLimit = 1 << 16
		// Bound all copies made by the fuzz harness as well as the parser.
		if len(data) > inputLimit {
			return
		}
		input := bytes.Clone(data)
		limits := ReplayLimits{MaxJournalBytes: inputLimit, MaxRecords: 64, MaxDatabaseBytes: 1 << 20}
		plan, err := ParseReplay(input, limits)
		if !bytes.Equal(input, data) {
			t.Fatal("parser changed input")
		}
		if err != nil {
			if plan != nil {
				t.Fatal("error returned partial plan")
			}
			if !errors.Is(err, ErrFormat) && !errors.Is(err, ErrReplayUnsupported) && !errors.Is(err, ErrReplayLimit) {
				t.Fatal(err)
			}
			return
		}
		other, err := ParseReplay(input, limits)
		if err != nil || !reflect.DeepEqual(plan, other) {
			t.Fatal("non-deterministic plan")
		}
		checkReplayCountEvidence(t, data, plan)
		if len(plan.Pages) > 64 || plan.DatabaseBytes != int64(plan.Header.OriginalPages)*int64(plan.Header.PageSize) {
			t.Fatal("invalid first header, cumulative count or extent")
		}
		var retained uint64
		seen := map[uint32]bool{}
		for _, p := range plan.Pages {
			if seen[p.Number] || p.Number == 0 || p.Number > plan.Header.OriginalPages || p.Number == 0x40000000/plan.Header.PageSize+1 || len(p.Data) != int(plan.Header.PageSize) {
				t.Fatal("invalid successful page image")
			}
			seen[p.Number] = true
			retained += uint64(len(p.Data))
		}
		if retained > uint64(len(data)) || retained > limits.MaxJournalBytes || retained > limits.MaxDatabaseBytes {
			t.Fatal("unbounded retained image bytes")
		}
		if err := plan.ValidateExtent(plan.DatabaseBytes); err != nil {
			t.Fatal(err)
		}
		for i := range input {
			input[i] ^= 0xff
		}
		if !reflect.DeepEqual(plan, other) {
			t.Fatal("result aliases input")
		}
	})
}
