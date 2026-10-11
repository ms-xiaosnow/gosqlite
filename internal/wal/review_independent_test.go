package wal_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"reflect"
	"sync"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/wal"
)

// All fixtures, readers, writers, error graphs and randomized models below
// exist only in memory. No file, database, native VFS or process is acquired.
func reviewSum(magic uint32, b []byte, seed [2]uint32) [2]uint32 {
	x, y := uint64(seed[0]), uint64(seed[1])
	for i := 0; i < len(b); i += 8 {
		word := func(off int) uint64 {
			var v uint64
			for j := 0; j < 4; j++ {
				shift := j * 8
				if magic == wal.MagicBigEndian {
					shift = (3 - j) * 8
				}
				v |= uint64(b[off+j]) << shift
			}
			return v
		}
		x = (x + word(i) + y) & 0xffffffff
		y = (y + word(i+4) + x) & 0xffffffff
	}
	return [2]uint32{uint32(x), uint32(y)}
}

type reviewLog struct {
	b           []byte
	magic, page uint32
	sum         [2]uint32
}

func reviewNew(magic, page uint32) *reviewLog {
	b := make([]byte, 32)
	put := binary.BigEndian.PutUint32
	for i, v := range []uint32{magic, 3007000, page, 0x12345678, 0xaabbccdd, 0x99887766} {
		put(b[i*4:], v)
	}
	sum := reviewSum(magic, b[:24], [2]uint32{})
	put(b[24:], sum[0])
	put(b[28:], sum[1])
	return &reviewLog{b, magic, page, sum}
}
func (l *reviewLog) append(pg, db uint32, p []byte) {
	b := make([]byte, 24+len(p))
	put := binary.BigEndian.PutUint32
	put(b, pg)
	put(b[4:], db)
	copy(b[8:16], l.b[16:24])
	copy(b[24:], p)
	l.sum = reviewSum(l.magic, b[24:], reviewSum(l.magic, b[:8], l.sum))
	put(b[16:], l.sum[0])
	put(b[20:], l.sum[1])
	l.b = append(l.b, b...)
}

type reviewWriter struct {
	b      []byte
	calls  int
	failAt int
	n      int
	err    error
	after  func()
}

func (w *reviewWriter) WriteAt(b []byte, off int64) (int, error) {
	w.calls++
	if w.calls == w.failAt {
		return w.n, w.err
	}
	if end := int(off) + len(b); end > len(w.b) {
		w.b = append(w.b, make([]byte, end-len(w.b))...)
	}
	copy(w.b[int(off):], b)
	if w.after != nil {
		w.after()
	}
	return len(b), nil
}
func reviewScan(t *testing.T, l *reviewLog) (wal.CommittedPrefix, error) {
	t.Helper()
	p, e := wal.ScanPrefix(context.Background(), bytes.NewReader(l.b), int64(len(l.b)), wal.Limits{})
	if !p.Allows(e, wal.TailUncommitted) {
		t.Fatalf("fixture failed: %v", e)
	}
	return p, e
}

func TestReviewIndependentPrefixProperty(t *testing.T) {
	rng := rand.New(rand.NewSource(0x134fba))
	cases := 0
	for _, magic := range []uint32{wal.MagicLittleEndian, wal.MagicBigEndian} {
		for page := uint32(512); page <= 65536; page *= 2 {
			for sector := uint32(512); sector <= 65536; sector *= 2 {
				for trial := 0; trial < 6; trial++ {
					l := reviewNew(magic, page)
					model := map[uint32][]byte{}
					var db uint32
					ntx := trial * 2
					for tx := 0; tx < ntx; tx++ {
						pending := map[uint32][]byte{}
						db = uint32(1 + rng.Intn(11))
						nf := 1 + rng.Intn(6)
						for f := 0; f < nf; f++ {
							pg := uint32(1 + rng.Intn(15))
							payload := bytes.Repeat([]byte{byte(1 + rng.Intn(250))}, int(page))
							commit := uint32(0)
							if f == nf-1 {
								commit = db
							}
							l.append(pg, commit, payload)
							pending[pg] = payload
						}
						for pg, payload := range pending {
							model[pg] = payload
						}
						for pg := range model {
							if pg > db {
								delete(model, pg)
							}
						}
					}
					commitEnd := len(l.b)
					original := bytes.Clone(l.b)
					commitSum := l.sum
					l.append(1, 0, bytes.Repeat([]byte{254}, int(page)))
					l.append(20, 0, bytes.Repeat([]byte{253}, int(page)))
					sourceBefore := bytes.Clone(l.b)
					p, e := reviewScan(t, l)
					s := p.Snapshot()
					if s.DBSize != db || int(s.EndOffset) != commitEnd || s.Checksum != commitSum {
						t.Fatal("commit metadata")
					}
					plan, err := wal.PlanPrefixReplacement(p, e, sector)
					if err != nil {
						t.Fatal(err)
					}
					want := reviewNew(magic, page)
					want.b = original
					want.sum = commitSum
					boundary := commitEnd
					if ntx > 0 {
						boundary = ((commitEnd + int(sector) - 1) / int(sector)) * int(sector)
					}
					if commitEnd > 32 {
						last := sourceBefore[commitEnd-int(page)-24 : commitEnd]
						for len(want.b) < boundary {
							want.append(binary.BigEndian.Uint32(last), db, last[24:])
						}
					}
					var dst reviewWriter
					got, err := plan.WriteCandidate(context.Background(), bytes.NewReader(l.b), &dst)
					if err != nil || !bytes.Equal(dst.b, want.b) || plan.Size() != int64(len(want.b)) {
						t.Fatalf("exact bytes case %d: err=%v got=%d want=%d planned=%d", cases, err, len(dst.b), len(want.b), plan.Size())
					}
					if !bytes.Equal(l.b, sourceBefore) {
						t.Fatal("source mutated")
					}
					parsed, err := wal.Scan(context.Background(), bytes.NewReader(dst.b), int64(len(dst.b)), wal.Limits{})
					if err != nil || !reflect.DeepEqual(got, parsed) || len(got.Pages) != len(model) || got.DBSize != db {
						t.Fatalf("result/model mismatch case %d: %v", cases, err)
					}
					for pg, payload := range model {
						ref, ok := got.Pages[pg]
						if !ok || !bytes.Equal(dst.b[ref.Offset:ref.Offset+int64(page)], payload) {
							t.Fatalf("page %d mismatch case %d", pg, cases)
						}
					}
					if ntx == 0 && (len(dst.b) != 32 || got.FrameCount != 0) {
						t.Fatal("zero-commit mismatch")
					}
					cases++
				}
			}
		}
	}
	t.Logf("%d exact-byte recipes: independent checksum/transaction model, all 8 page sizes, all 8 sector sizes, both orders", cases)
}

type reviewUncomparable []int

func (reviewUncomparable) Error() string { return "uncomparable synthetic error" }

type reviewPoison struct{}

func (*reviewPoison) Error() string   { return "typed-nil or hostile error" }
func (*reviewPoison) Is(error) bool   { panic("must not traverse Is") }
func (*reviewPoison) As(any) bool     { panic("must not traverse As") }
func (p *reviewPoison) Unwrap() error { panic("must not traverse Unwrap") }
func TestReviewIndependentExactIdentityAndCopies(t *testing.T) {
	l := reviewNew(wal.MagicLittleEndian, 512)
	l.append(1, 1, bytes.Repeat([]byte{1}, 512))
	l.append(1, 0, bytes.Repeat([]byte{2}, 512))
	p, e := reviewScan(t, l)
	var typedNil *reviewPoison
	for _, bad := range []error{nil, wal.ErrTail, fmt.Errorf("wrapped: %w", e), errors.Join(e), errors.Join(e, io.ErrClosedPipe), typedNil, &reviewPoison{}, reviewUncomparable{1, 2}} {
		if p.Allows(bad, wal.TailUncommitted, wal.TailSaltMismatch, wal.TailInvalid, wal.TailIncomplete) {
			t.Fatal("nonexact error admitted")
		}
		if _, err := wal.PlanPrefixReplacement(p, bad, 4096); err == nil {
			t.Fatal("nonexact recipe admitted")
		}
	}
	q := p
	s := p.Snapshot()
	clear(s.Pages)
	s.Header = wal.Header{}
	s.EndOffset = 0
	s.FrameCount = 99
	s.Tail = wal.TailClean
	s.Complete = true
	s.TailCause = io.ErrClosedPipe
	if !q.Allows(e, wal.TailUncommitted) || q.Snapshot().Pages[1].Frame != 1 || p.Snapshot().EndOffset != 568 {
		t.Fatal("copy/seal mutable")
	}
	clean, ce := wal.ScanPrefix(context.Background(), bytes.NewReader(l.b[:32]), 32, wal.Limits{})
	if ce != nil || !clean.Allows(nil) || clean.Allows(typedNil) || clean.Allows(reviewUncomparable{1}) || (wal.CommittedPrefix{}).Allows(nil) {
		t.Fatal("zero/clean identity")
	}
	p2, e2 := reviewScan(t, l)
	if p2.Allows(e, wal.TailUncommitted) || p.Allows(e2, wal.TailUncommitted) {
		t.Fatal("cross-scan identity")
	}
}

type reviewReader struct {
	b                   []byte
	call, target, delta int
	err                 error
	after               func()
}

func (r *reviewReader) ReadAt(b []byte, off int64) (int, error) {
	r.call++
	n, e := bytes.NewReader(r.b).ReadAt(b, off)
	if r.after != nil {
		r.after()
	}
	if r.call == r.target {
		return n + r.delta, r.err
	}
	return n, e
}
func TestReviewIndependentReadGraphs(t *testing.T) {
	l := reviewNew(wal.MagicBigEndian, 512)
	l.append(1, 1, bytes.Repeat([]byte{1}, 512))
	l.append(1, 0, bytes.Repeat([]byte{2}, 512))
	_, tail := reviewScan(t, l)
	ioCause := errors.New("synthetic I/O owner")
	owner := errors.New("synthetic retained child owner")
	for call := 1; call <= 3; call++ {
		for _, delta := range []int{-1, 0, 1} {
			for _, cause := range []error{tail, errors.Join(io.EOF, ioCause, owner, tail), errors.Join(wal.ErrTail, wal.ErrUnsupported, owner)} {
				r := &reviewReader{b: l.b, target: call, delta: delta, err: cause}
				p, e := wal.ScanPrefix(context.Background(), r, int64(len(l.b)), wal.Limits{})
				if e == nil || p.Allows(e, wal.TailUncommitted, wal.TailInvalid, wal.TailIncomplete, wal.TailSaltMismatch) || p.Snapshot().EndOffset != 0 {
					t.Fatal("read graph minted proof")
				}
				if !errors.Is(e, cause) || (delta != 0 && !errors.Is(e, io.ErrUnexpectedEOF)) {
					t.Fatal("read graph lost cause", e)
				}
			}
		}
	}
	// Traversal traps are injected only into the I/O result; admission must not
	// invoke their Is, As, or Unwrap hooks, including a typed-nil implementation.
	var typedNil *reviewPoison
	for _, cause := range []error{typedNil, &reviewPoison{}, reviewUncomparable{1}} {
		r := &reviewReader{b: l.b, target: 2, err: cause}
		p, e := wal.ScanPrefix(context.Background(), r, int64(len(l.b)), wal.Limits{})
		if e == nil || p.Allows(e, wal.TailUncommitted) {
			t.Fatal("hostile cause admitted")
		}
	}
	// The ReaderAt contract permits exact EOF on a full read, not joined EOF.
	for call := 1; call <= 3; call++ {
		r := &reviewReader{b: l.b, target: call, err: io.EOF}
		p, e := wal.ScanPrefix(context.Background(), r, int64(len(l.b)), wal.Limits{})
		if !p.Allows(e, wal.TailUncommitted) {
			t.Fatal("legal full EOF rejected", e)
		}
	}
}

func TestReviewIndependentBuilderDiagnostics(t *testing.T) {
	l := reviewNew(wal.MagicLittleEndian, 512)
	l.append(1, 1, bytes.Repeat([]byte{1}, 512))
	l.append(1, 0, bytes.Repeat([]byte{2}, 512))
	p, e := reviewScan(t, l)
	plan, err := wal.PlanPrefixReplacement(p, e, 4096)
	if err != nil {
		t.Fatal(err)
	}
	owner := errors.New("synthetic owner retained")
	cause := errors.Join(io.EOF, io.ErrClosedPipe, owner)
	for call := 1; call <= 2; call++ {
		for _, delta := range []int{-1, 0} {
			r := &reviewReader{b: l.b, target: call, delta: delta, err: cause}
			var w reviewWriter
			s, err := plan.WriteCandidate(context.Background(), r, &w)
			if !errors.Is(err, owner) || !errors.Is(err, io.EOF) || !errors.Is(err, io.ErrClosedPipe) || s.Complete {
				t.Fatal("source causes lost", err)
			}
			if delta != 0 && !errors.Is(err, io.ErrUnexpectedEOF) {
				t.Fatal("source short read lost")
			}
		}
	}
	for call := 1; call <= 8; call++ {
		for _, delta := range []int{-1, 0} {
			n := 536
			if call == 1 {
				n = 568
			}
			w := reviewWriter{failAt: call, n: n + delta, err: cause}
			s, err := plan.WriteCandidate(context.Background(), bytes.NewReader(l.b), &w)
			if !errors.Is(err, owner) || !errors.Is(err, io.EOF) || !errors.Is(err, io.ErrClosedPipe) || s.Complete {
				t.Fatalf("write %d causes lost %v", call, err)
			}
			if delta != 0 && !errors.Is(err, io.ErrShortWrite) {
				t.Fatal("short write lost")
			}
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	w := reviewWriter{after: cancel}
	s, err := plan.WriteCandidate(ctx, bytes.NewReader(l.b), &w)
	if !errors.Is(err, context.Canceled) || s.Complete || w.calls != 1 {
		t.Fatal("inter-write cancellation", err, w.calls)
	}
	bad := bytes.Clone(l.b)
	bad[32+4] ^= 1
	var untouched reviewWriter
	_, err = plan.WriteCandidate(context.Background(), bytes.NewReader(bad), &untouched)
	if !errors.Is(err, wal.ErrFormat) || untouched.calls != 0 {
		t.Fatal("changed final marker not rejected before writes")
	}
}

func TestReviewIndependentConcurrentDetachedUse(t *testing.T) {
	l := reviewNew(wal.MagicBigEndian, 512)
	l.append(1, 1, bytes.Repeat([]byte{1}, 512))
	l.append(1, 0, bytes.Repeat([]byte{2}, 512))
	p, e := reviewScan(t, l)
	var wg sync.WaitGroup
	for i := 0; i < 12; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				s := p.Snapshot()
				clear(s.Pages)
				plan, err := wal.PlanPrefixReplacement(p, e, 4096)
				if err != nil {
					t.Error(err)
					return
				}
				var dst reviewWriter
				got, err := plan.WriteCandidate(context.Background(), bytes.NewReader(l.b), &dst)
				if err != nil || got.Pages[1].Frame != 8 {
					t.Error("concurrent detached result", err)
					return
				}
			}
		}()
	}
	wg.Wait()
}

func TestReviewIndependentAlignedAndTailTaxonomy(t *testing.T) {
	for _, magic := range []uint32{wal.MagicLittleEndian, wal.MagicBigEndian} {
		l := reviewNew(magic, 512)
		for i := 0; i < 20; i++ {
			l.append(1, 1, bytes.Repeat([]byte{byte(i)}, 512))
		}
		end := len(l.b)
		l.append(1, 0, bytes.Repeat([]byte{99}, 512))
		p, e := reviewScan(t, l)
		plan, err := wal.PlanPrefixReplacement(p, e, 512)
		if err != nil || plan.Size() != int64(end) || end%512 != 0 {
			t.Fatal("aligned plan", err)
		}
		var w reviewWriter
		s, err := plan.WriteCandidate(context.Background(), bytes.NewReader(l.b), &w)
		if err != nil || !s.Complete || !bytes.Equal(w.b, l.b[:end]) || len(w.b) == len(l.b) {
			t.Fatal("aligned pending suffix survived", err)
		}
		invalid := bytes.Clone(l.b)
		invalid[len(invalid)-1] ^= 1
		salt := bytes.Clone(l.b)
		salt[end+8] ^= 1
		zeroPage := bytes.Clone(l.b)
		clear(zeroPage[end : end+4])
		version := bytes.Clone(l.b)
		version[7] ^= 1
		for _, tc := range []struct {
			name  string
			b     []byte
			tail  wal.TailStatus
			cause error
			proof bool
		}{
			{"clean", l.b[:end], wal.TailClean, nil, true}, {"incomplete", l.b[:len(l.b)-1], wal.TailIncomplete, io.ErrUnexpectedEOF, true}, {"checksum", invalid, wal.TailInvalid, wal.ErrChecksum, true}, {"old-salt", salt, wal.TailSaltMismatch, wal.ErrSalt, true}, {"zero-page", zeroPage, wal.TailInvalid, wal.ErrFormat, true}, {"version", version, wal.TailClean, wal.ErrUnsupported, false}, {"short-header", l.b[:31], wal.TailClean, wal.ErrFormat, false},
		} {
			t.Run(fmt.Sprintf("%x/%s", magic, tc.name), func(t *testing.T) {
				p, e := wal.ScanPrefix(context.Background(), bytes.NewReader(tc.b), int64(len(tc.b)), wal.Limits{})
				if !errors.Is(e, tc.cause) {
					t.Fatal("cause", e)
				}
				if p.Allows(e, tc.tail) != tc.proof {
					t.Fatal("taxonomy proof")
				}
				_, rejected := wal.PlanPrefixReplacement(p, e, 4096)
				if !errors.Is(rejected, wal.ErrUnsupported) {
					t.Fatal("builder admitted excluded taxonomy", err)
				}
				if tc.cause != nil && !errors.Is(rejected, tc.cause) {
					t.Fatal("planner erased rejected cause")
				}
			})
		}
	}
}

// Go 1.27.2 errors/errors.go requires every []error child to be non-nil.
func TestReviewIndependentUnwrapContract(t *testing.T) {
	l := reviewNew(wal.MagicLittleEndian, 512)
	l.append(1, 1, bytes.Repeat([]byte{1}, 512))
	l.append(1, 0, bytes.Repeat([]byte{2}, 512))
	_, e := reviewScan(t, l)
	children, ok := e.(interface{ Unwrap() []error })
	if !ok {
		t.Fatal("missing cause graph")
	}
	for i, child := range children.Unwrap() {
		if child == nil {
			t.Fatalf("Unwrap returned nil child %d for a valid-uncommitted tail; Go error-tree contract forbids nil entries", i)
		}
	}
}
