package wal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"reflect"
	"sync"
	"testing"
)

// All logs in these tests are byte slices. No database/WAL files are opened,
// no process exits are induced, and no failed-write recovery is exercised.
type memoryLog struct {
	h     Header
	bytes []byte
	sum   Checksum
	count uint32
}

func newMemoryLog(t testing.TB, magic, pageSize uint32) *memoryLog {
	t.Helper()
	h, b := testHeader(t, magic, pageSize)
	return &memoryLog{h: h, bytes: b, sum: h.Checksum}
}

func (l *memoryLog) append(t testing.TB, pageNo, dbSize uint32, tag byte) PageRef {
	t.Helper()
	page := bytes.Repeat([]byte{tag}, int(l.h.PageSize))
	frame, sum, err := EncodeFrame(l.h, pageNo, dbSize, page, l.sum)
	if err != nil {
		t.Fatal(err)
	}
	l.count++
	ref := PageRef{Frame: l.count, Offset: int64(len(l.bytes)) + FrameHeaderSize}
	l.bytes = append(l.bytes, frame...)
	l.sum = sum
	return ref
}

func scanMemory(b []byte, limits Limits) (Snapshot, error) {
	return Scan(context.Background(), bytes.NewReader(b), int64(len(b)), limits)
}

func TestScanHeaderOnlyAndAllPageSizes(t *testing.T) {
	for _, magic := range []uint32{MagicLittleEndian, MagicBigEndian} {
		for ps := uint32(512); ps <= 65536; ps *= 2 {
			l := newMemoryLog(t, magic, ps)
			s, err := scanMemory(l.bytes, Limits{})
			if err != nil || !s.Complete || s.Header != l.h || s.DBSize != 0 || len(s.Pages) != 0 || s.FrameCount != 0 || s.EndOffset != HeaderSize || s.Checksum != l.h.Checksum || s.ValidEndOffset != HeaderSize || s.Tail != TailClean {
				t.Fatalf("header only %d: %+v %v", ps, s, err)
			}
			ref := l.append(t, 1, 1, 0x57)
			s, err = scanMemory(l.bytes, Limits{})
			if err != nil || !s.Complete || s.Pages[1] != ref || s.DBSize != 1 || s.FrameCount != 1 || s.ValidFrameCount != 1 || s.EndOffset != int64(len(l.bytes)) || s.Checksum != l.sum {
				t.Fatalf("single frame %d: %+v %v", ps, s, err)
			}
		}
	}
}

func TestScanLatestCommitDuplicatesPaddingAndTruncation(t *testing.T) {
	l := newMemoryLog(t, MagicLittleEndian, 512)
	l.append(t, 1, 0, 1)
	l.append(t, 1, 0, 2)
	p2 := l.append(t, 2, 0, 3)
	l.append(t, 4, 4, 4)
	p1 := l.append(t, 1, 0, 5)
	// A truncating transaction may contain a frame above its final DBSize.
	l.append(t, 9, 0, 6)
	l.append(t, 3, 3, 7)
	p3 := l.append(t, 3, 3, 7) // Recomputed-checksum duplicate commit padding.
	s, err := scanMemory(l.bytes, Limits{})
	want := map[uint32]PageRef{1: p1, 2: p2, 3: p3}
	if err != nil || !s.Complete || !reflect.DeepEqual(s.Pages, want) || s.DBSize != 3 || s.FrameCount != 8 || s.Checksum != l.sum || s.EndOffset != int64(len(l.bytes)) {
		t.Fatalf("snapshot = %+v, %v; want %+v", s, err, want)
	}
	for pg, tag := range map[uint32]byte{1: 5, 2: 3, 3: 7} {
		ref := s.Pages[pg]
		if !bytes.Equal(l.bytes[ref.Offset:ref.Offset+int64(l.h.PageSize)], bytes.Repeat([]byte{tag}, int(l.h.PageSize))) {
			t.Fatalf("page %d offset %d wrong", pg, ref.Offset)
		}
	}
	// Growing does not resurrect a page removed by an earlier shrink.
	p8 := l.append(t, 8, 8, 8)
	s, err = scanMemory(l.bytes, Limits{})
	if err != nil || s.DBSize != 8 || s.Pages[8] != p8 {
		t.Fatalf("grow: %+v %v", s, err)
	}
	if _, exists := s.Pages[4]; exists {
		t.Fatal("truncated page was resurrected")
	}
}

func TestScanUncommittedDoesNotLeakIntoSnapshot(t *testing.T) {
	for _, withCommit := range []bool{false, true} {
		l := newMemoryLog(t, MagicBigEndian, 512)
		want := make(map[uint32]PageRef)
		var count, dbSize uint32
		if withCommit {
			want[1] = l.append(t, 1, 1, 1)
			count, dbSize = 1, 1
		}
		end, sum := int64(len(l.bytes)), l.sum
		l.append(t, 1, 0, 2)
		l.append(t, 2, 0, 3)
		s, err := scanMemory(l.bytes, Limits{})
		if !errors.Is(err, ErrTail) || s.Complete || s.Tail != TailUncommitted || s.TailOffset != end || s.EndOffset != end || s.Checksum != sum || s.FrameCount != count || s.DBSize != dbSize || !reflect.DeepEqual(s.Pages, want) || s.ValidFrameCount != count+2 || s.ValidEndOffset != int64(len(l.bytes)) {
			t.Fatalf("withCommit=%t: %+v %v", withCommit, s, err)
		}
	}
}

func TestScanEveryTruncationBoundary(t *testing.T) {
	l := newMemoryLog(t, MagicLittleEndian, 512)
	l.append(t, 1, 1, 1)
	l.append(t, 2, 0, 2)
	l.append(t, 2, 2, 3)
	l.append(t, 3, 0, 4)
	frameSize := FrameHeaderSize + int(l.h.PageSize)
	for n := 0; n <= len(l.bytes); n++ {
		s, err := scanMemory(l.bytes[:n], Limits{})
		if n < HeaderSize {
			if !errors.Is(err, ErrFormat) || s.Complete {
				t.Fatalf("prefix %d: %+v %v", n, s, err)
			}
			continue
		}
		valid := uint32((n - HeaderSize) / frameSize)
		commit, dbSize := uint32(0), uint32(0)
		if valid >= 1 {
			commit, dbSize = 1, 1
		}
		if valid >= 3 {
			commit, dbSize = 3, 2
		}
		wantTail := TailClean
		if (n-HeaderSize)%frameSize != 0 {
			wantTail = TailIncomplete
		} else if valid != commit {
			wantTail = TailUncommitted
		}
		if s.Tail != wantTail || s.FrameCount != commit || s.DBSize != dbSize || s.ValidFrameCount != valid || s.EndOffset != HeaderSize+int64(commit)*int64(frameSize) || s.Complete != (wantTail == TailClean) {
			t.Fatalf("prefix %d: %+v err=%v wantTail=%s commit=%d", n, s, err, wantTail, commit)
		}
		if wantTail == TailClean && err != nil || wantTail != TailClean && !errors.Is(err, ErrTail) {
			t.Fatalf("prefix %d error=%v wantTail=%s", n, err, wantTail)
		}
		if len(s.Pages) != int(dbSize) {
			t.Fatalf("prefix %d leaked pages: %+v", n, s.Pages)
		}
	}
}

func TestScanInvalidTailStopsAtFirstFailure(t *testing.T) {
	for _, pos := range []int{0, 8, 12, 16, 20, 24, 255, 535} {
		l := newMemoryLog(t, MagicLittleEndian, 512)
		ref := l.append(t, 1, 1, 1)
		end, sum := int64(len(l.bytes)), l.sum
		l.append(t, 1, 0, 2)
		l.append(t, 1, 1, 3)
		l.bytes[int(end)+pos] ^= 1
		s, err := scanMemory(l.bytes, Limits{})
		wantTail := TailInvalid
		if pos == 8 || pos == 12 {
			wantTail = TailSaltMismatch
		}
		if !errors.Is(err, ErrTail) || s.Complete || s.Tail != wantTail || s.TailOffset != end || s.FrameCount != 1 || s.ValidFrameCount != 1 || s.EndOffset != end || s.Checksum != sum || s.Pages[1] != ref || len(s.Pages) != 1 {
			t.Fatalf("mutation %d: %+v %v", pos, s, err)
		}
	}
	// A copied old frame has the right salt but the wrong rolling checksum.
	l := newMemoryLog(t, MagicLittleEndian, 512)
	l.append(t, 1, 1, 1)
	l.bytes = append(l.bytes, l.bytes[HeaderSize:]...)
	s, err := scanMemory(l.bytes, Limits{})
	if !errors.Is(err, ErrChecksum) || !errors.Is(err, ErrTail) || s.FrameCount != 1 {
		t.Fatalf("raw duplicated frame accepted: %+v %v", s, err)
	}
}

func TestScanRandomTransactionsAgainstMemoryModel(t *testing.T) {
	rng := rand.New(rand.NewSource(780023))
	for trial := 0; trial < 200; trial++ {
		magic := MagicLittleEndian
		if trial%2 != 0 {
			magic = MagicBigEndian
		}
		l := newMemoryLog(t, magic, 512<<uint(trial%4))
		model := make(map[uint32]PageRef)
		tags := make(map[uint32]byte)
		var dbSize uint32
		for tx := 0; tx < 12; tx++ {
			pending := make(map[uint32]PageRef)
			pendingTags := make(map[uint32]byte)
			dbSize = 1 + uint32(rng.Intn(20))
			nframes := 1 + rng.Intn(12)
			for i := 0; i < nframes; i++ {
				pg := 1 + uint32(rng.Intn(24))
				tag := byte(rng.Intn(256))
				commit := uint32(0)
				if i == nframes-1 {
					commit = dbSize
				}
				pending[pg] = l.append(t, pg, commit, tag)
				pendingTags[pg] = tag
			}
			for pg, ref := range pending {
				model[pg] = ref
				tags[pg] = pendingTags[pg]
			}
			for pg := range model {
				if pg > dbSize {
					delete(model, pg)
					delete(tags, pg)
				}
			}
			s, err := scanMemory(l.bytes, Limits{})
			if err != nil || !s.Complete || !reflect.DeepEqual(s.Pages, model) || s.DBSize != dbSize || s.FrameCount != l.count || s.Checksum != l.sum || s.EndOffset != int64(len(l.bytes)) {
				t.Fatalf("trial %d tx %d: %+v %v model%+v", trial, tx, s, err, model)
			}
			for pg, ref := range s.Pages {
				want := bytes.Repeat([]byte{tags[pg]}, int(l.h.PageSize))
				if !bytes.Equal(l.bytes[ref.Offset:ref.Offset+int64(l.h.PageSize)], want) {
					t.Fatalf("trial %d tx %d page %d points to wrong bytes", trial, tx, pg)
				}
			}
		}
	}
}

type observedReader struct {
	r        io.ReaderAt
	calls    int
	maxRead  int
	byteRead int
	after    func(int)
}

func (r *observedReader) ReadAt(p []byte, off int64) (int, error) {
	r.calls++
	r.maxRead = max(r.maxRead, len(p))
	r.byteRead += len(p)
	n, err := r.r.ReadAt(p, off)
	if r.after != nil {
		r.after(r.calls)
	}
	return n, err
}

func TestScanReaderAtBoundedAndCancellation(t *testing.T) {
	l := newMemoryLog(t, MagicBigEndian, 65536)
	for i := 0; i < 30; i++ {
		l.append(t, 1, 1, byte(i))
	}
	r := &observedReader{r: bytes.NewReader(l.bytes)}
	s, err := Scan(context.Background(), r, int64(len(l.bytes)), Limits{})
	if err != nil || !s.Complete || r.calls != 31 || r.maxRead != 65536+FrameHeaderSize || r.byteRead != len(l.bytes) {
		t.Fatalf("bounded reader: %+v calls%d max%d bytes%d err%v", s, r.calls, r.maxRead, r.byteRead, err)
	}
	for _, cancelOn := range []int{0, 1, 2, 5, 31} {
		ctx, cancel := context.WithCancel(context.Background())
		r := &observedReader{r: bytes.NewReader(l.bytes), after: func(call int) {
			if call == cancelOn {
				cancel()
			}
		}}
		if cancelOn == 0 {
			cancel()
		}
		s, err := Scan(ctx, r, int64(len(l.bytes)), Limits{})
		cancel()
		if !errors.Is(err, context.Canceled) || s.Complete || r.calls != cancelOn {
			t.Fatalf("cancel on %d: %+v calls%d %v", cancelOn, s, r.calls, err)
		}
	}
}

func TestConcurrentScansOfImmutableReader(t *testing.T) {
	l := newMemoryLog(t, MagicLittleEndian, 1024)
	for i := uint32(1); i <= 60; i++ {
		l.append(t, i, i, byte(i))
	}
	original := bytes.Clone(l.bytes)
	r := bytes.NewReader(l.bytes)
	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				s, err := Scan(context.Background(), r, int64(len(l.bytes)), Limits{})
				if err != nil || !s.Complete || s.FrameCount != 60 || len(s.Pages) != 60 || s.Checksum != l.sum {
					t.Errorf("parallel scan: %+v, %v", s, err)
					return
				}
				// The result belongs to each scan, never shared global state.
				delete(s.Pages, 1)
			}
		}()
	}
	wg.Wait()
	if !bytes.Equal(l.bytes, original) {
		t.Fatal("scan mutated input")
	}
}

type errorReader struct {
	b     []byte
	err   error
	short bool
}

func (r errorReader) ReadAt(p []byte, off int64) (int, error) {
	n := copy(p, r.b[off:])
	if r.short && n > 0 {
		n--
	}
	return n, r.err
}

func TestScanReadErrorsNeverBecomeTails(t *testing.T) {
	l := newMemoryLog(t, MagicLittleEndian, 512)
	l.append(t, 1, 1, 1)
	boom := errors.New("reader failed")
	for _, tc := range []struct {
		err   error
		short bool
		want  error
	}{
		{nil, true, io.ErrUnexpectedEOF}, {io.EOF, true, io.ErrUnexpectedEOF},
		{boom, true, boom}, {boom, false, boom}, {errors.Join(io.EOF, boom), false, boom},
	} {
		s, err := Scan(context.Background(), errorReader{l.bytes, tc.err, tc.short}, int64(len(l.bytes)), Limits{})
		if !errors.Is(err, tc.want) || errors.Is(err, ErrTail) || s.Complete {
			t.Fatalf("read %+v: %+v %v", tc, s, err)
		}
	}
	// A legal full read accompanied by EOF must still succeed.
	s, err := Scan(context.Background(), errorReader{l.bytes, io.EOF, false}, int64(len(l.bytes)), Limits{})
	if err != nil || !s.Complete {
		t.Fatalf("full EOF: %+v %v", s, err)
	}
	// Caller-supplied extent disagrees with ReaderAt: this is an I/O error.
	s, err = Scan(context.Background(), bytes.NewReader(l.bytes[:len(l.bytes)-1]), int64(len(l.bytes)), Limits{})
	if !errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, ErrTail) || s.Complete {
		t.Fatalf("extent mismatch: %+v %v", s, err)
	}
}

func TestScanResourceLimits(t *testing.T) {
	l := newMemoryLog(t, MagicLittleEndian, 512)
	l.append(t, 1, 0, 1)
	l.append(t, 2, 2, 2)
	l.append(t, 1, 2, 3)
	for _, limits := range []Limits{
		{MaxWALBytes: -1}, {MaxWALBytes: int64(len(l.bytes) - 1)}, {MaxFrames: 2},
		{MaxIndexEntries: 1}, {MaxIndexEntries: 2}, {MaxDBPages: 1}, {MaxDBPages: 0xffffffff},
	} {
		s, err := scanMemory(l.bytes, limits)
		if !errors.Is(err, ErrLimit) || errors.Is(err, ErrTail) || s.Complete {
			t.Fatalf("limit %+v: %+v %v", limits, s, err)
		}
	}
	s, err := scanMemory(l.bytes, Limits{MaxWALBytes: int64(len(l.bytes)), MaxFrames: 3, MaxIndexEntries: 3, MaxDBPages: 2})
	if err != nil || !s.Complete {
		t.Fatalf("exact limits: %+v %v", s, err)
	}
	r := &observedReader{r: bytes.NewReader(l.bytes)}
	_, err = Scan(context.Background(), r, 1<<62, Limits{})
	if !errors.Is(err, ErrLimit) || r.calls != 0 {
		t.Fatalf("huge input read before limit: calls%d %v", r.calls, err)
	}
	_, err = Scan(context.Background(), r, int64(len(l.bytes)), Limits{MaxFrames: 1})
	if !errors.Is(err, ErrLimit) || r.calls != 1 {
		t.Fatalf("frame count limit read frames: calls%d %v", r.calls, err)
	}
	for _, size := range []int64{-1, -1 << 63, 0, HeaderSize - 1} {
		if _, err := Scan(context.Background(), r, size, Limits{}); !errors.Is(err, ErrFormat) {
			t.Fatalf("invalid size %d: %v", size, err)
		}
	}
}

func TestScanAllocationsDoNotScaleWithDuplicateFrames(t *testing.T) {
	allocationCount := func(n int) float64 {
		l := newMemoryLog(t, MagicLittleEndian, 512)
		for i := 0; i < n; i++ {
			l.append(t, 1, 1, byte(i))
		}
		return testing.AllocsPerRun(5, func() {
			s, err := scanMemory(l.bytes, Limits{})
			if err != nil || !s.Complete || len(s.Pages) != 1 {
				t.Fatal(err)
			}
		})
	}
	small, large := allocationCount(10), allocationCount(1000)
	if large > small+8 {
		t.Fatalf("allocations scale with frames: 10=%g 1000=%g", small, large)
	}
}

func TestScanIndexWorkBoundAndNoopShrinks(t *testing.T) {
	l := newMemoryLog(t, MagicLittleEndian, 512)
	for pg := uint32(1); pg <= 64; pg++ {
		commit := uint32(0)
		if pg == 64 {
			commit = 128
		}
		l.append(t, pg, commit, byte(pg))
	}
	// All indexed pages are <=64. Alternating 127/128-page commit markers
	// must not sweep this map, even though half of the markers shrink DBSize.
	for i := 0; i < 200; i++ {
		l.append(t, 1, 127+uint32(i%2), byte(i))
	}
	s, err := scanMemory(l.bytes, Limits{MaxIndexWork: 128 + 400})
	if err != nil || !s.Complete || s.FrameCount != 264 || len(s.Pages) != 64 || s.IndexWork != 528 {
		t.Fatalf("no-op shrinks exceeded linear budget: %+v %v", s, err)
	}
	if _, err := scanMemory(l.bytes, Limits{MaxIndexWork: 128 + 399}); !errors.Is(err, ErrLimit) {
		t.Fatalf("index work budget not enforced: %v", err)
	}

	l = newMemoryLog(t, MagicLittleEndian, 512)
	for pg := uint32(1); pg <= 64; pg++ {
		commit := uint32(0)
		if pg == 64 {
			commit = 64
		}
		l.append(t, pg, commit, byte(pg))
	}
	for i := 0; i < 10; i++ {
		l.append(t, 65, 65, byte(i))
		l.append(t, 1, 64, byte(i))
	}
	// Real shrink/grow cycles must stop at the explicit total work budget,
	// rather than perform O(frames*indexEntries) work unchecked.
	s, err = scanMemory(l.bytes, Limits{MaxIndexWork: 200})
	if !errors.Is(err, ErrLimit) || s.Complete || s.FrameCount != 67 || s.DBSize != 65 || len(s.Pages) != 65 || s.IndexWork != 200 {
		t.Fatalf("real shrinks budget: %+v %v", s, err)
	}
	prefix, perr := scanMemory(l.bytes[:s.EndOffset], Limits{})
	if perr != nil || !reflect.DeepEqual(prefix.Pages, s.Pages) || prefix.Checksum != s.Checksum || prefix.IndexWork != s.IndexWork {
		t.Fatalf("budget failure mutated previous commit: %+v %v", prefix, perr)
	}
}

func TestTailString(t *testing.T) {
	for _, status := range []TailStatus{TailClean, TailUncommitted, TailIncomplete, TailInvalid, TailSaltMismatch, 255} {
		if fmt.Sprint(status) == "" {
			t.Fatal("empty tail status")
		}
	}
}
