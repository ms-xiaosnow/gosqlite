package wal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"testing"
)

// The destination is a byte slice. None of these tests acquires a database,
// touches a native WAL, injects native I/O, or executes recovery/publication.
type prefixMemoryWriter struct {
	bytes []byte
	calls int
}

func (w *prefixMemoryWriter) WriteAt(b []byte, off int64) (int, error) {
	w.calls++
	if end := int(off) + len(b); end > len(w.bytes) {
		w.bytes = append(w.bytes, make([]byte, end-len(w.bytes))...)
	}
	copy(w.bytes[int(off):], b)
	return len(b), nil
}

func TestPrefixReplacementMemoryByteEquivalence(t *testing.T) {
	for _, magic := range []uint32{MagicLittleEndian, MagicBigEndian} {
		for page := uint32(512); page <= 65536; page *= 2 {
			for sector := uint32(512); sector <= 65536; sector *= 2 {
				for _, commits := range []int{0, 1, 2, 20} {
					t.Run(fmt.Sprintf("%x/%d/%d/%d", magic, page, sector, commits), func(t *testing.T) {
						l := newMemoryLog(t, magic, page)
						for i := 0; i < commits; i++ {
							l.append(t, uint32(i%2+1), 2, byte(i+1))
						}
						commitEnd := int64(len(l.bytes))
						l.append(t, 1, 0, 201)
						l.append(t, 3, 0, 202)
						original := append([]byte(nil), l.bytes...)
						p, scanErr := ScanPrefix(context.Background(), bytes.NewReader(l.bytes), int64(len(l.bytes)), Limits{})
						plan, err := PlanPrefixReplacement(p, scanErr, sector)
						if err != nil {
							t.Fatal(err)
						}
						var dst prefixMemoryWriter
						built, err := plan.WriteCandidate(context.Background(), bytes.NewReader(l.bytes), &dst)
						if err != nil || int64(len(dst.bytes)) != plan.Size() {
							t.Fatal("candidate", err, len(dst.bytes), plan.Size())
						}
						scanned, err := Scan(context.Background(), bytes.NewReader(dst.bytes), int64(len(dst.bytes)), Limits{})
						if err != nil || !reflect.DeepEqual(built, scanned) {
							t.Fatalf("candidate disagrees with independent scan: %v\n%+v\n%+v", err, built, scanned)
						}
						before := p.Snapshot()
						if scanned.DBSize != before.DBSize || scanned.Header != before.Header || len(scanned.Pages) != len(before.Pages) {
							t.Fatal("logical state changed")
						}
						for n, old := range before.Pages {
							new := scanned.Pages[n]
							if !bytes.Equal(l.bytes[old.Offset:old.Offset+int64(page)], dst.bytes[new.Offset:new.Offset+int64(page)]) {
								t.Fatal("committed page changed", n)
							}
						}
						if !bytes.Equal(original, l.bytes) || !bytes.Equal(original[:commitEnd], dst.bytes[:commitEnd]) {
							t.Fatal("old bytes/prefix changed")
						}
						if commits == 0 && len(dst.bytes) != HeaderSize {
							t.Fatal("pending-only tail survived")
						}
						if commits != 0 && int64(len(dst.bytes)) < (commitEnd+int64(sector)-1)/int64(sector)*int64(sector) {
							t.Fatal("unprotected committed sector")
						}
						if _, err := PlanPrefixReplacement(p, errors.Join(scanErr, io.ErrClosedPipe), sector); err == nil {
							t.Fatal("joined I/O was admitted")
						}
					})
				}
			}
		}
	}
}

func TestPrefixReplacementMemoryShrinkingCommit(t *testing.T) {
	l := newMemoryLog(t, MagicLittleEndian, 512)
	l.append(t, 1, 3, 1)
	l.append(t, 3, 3, 3)
	l.append(t, 4, 1, 4)
	l.append(t, 1, 0, 99)
	p, e := ScanPrefix(context.Background(), bytes.NewReader(l.bytes), int64(len(l.bytes)), Limits{})
	plan, e := PlanPrefixReplacement(p, e, 4096)
	if e != nil {
		t.Fatal(e)
	}
	var dst prefixMemoryWriter
	s, e := plan.WriteCandidate(context.Background(), bytes.NewReader(l.bytes), &dst)
	if e != nil || s.DBSize != 1 || len(s.Pages) != 1 || s.Pages[1].Frame != 1 {
		t.Fatal("padding resurrected shrink", s, e)
	}
	parsed, e := Scan(context.Background(), bytes.NewReader(dst.bytes), int64(len(dst.bytes)), Limits{})
	if e != nil || !reflect.DeepEqual(parsed, s) {
		t.Fatal("shrink parse mismatch", e)
	}
}

type prefixWriteFailure struct {
	cause error
	full  bool
}

func (w prefixWriteFailure) WriteAt(b []byte, _ int64) (int, error) {
	if w.full {
		return len(b), w.cause
	}
	return 0, w.cause
}

func TestPrefixReplacementMemoryFailuresAndBudgets(t *testing.T) {
	l := newMemoryLog(t, MagicLittleEndian, 512)
	l.append(t, 1, 1, 1)
	l.append(t, 1, 0, 2)
	p, e := ScanPrefix(context.Background(), bytes.NewReader(l.bytes), int64(len(l.bytes)), Limits{})
	for _, sector := range []uint32{0, 1, 511, 513, 131072} {
		if _, err := PlanPrefixReplacement(p, e, sector); !errors.Is(err, ErrUnsupported) {
			t.Fatal("sector", sector, err)
		}
	}
	plan, err := PlanPrefixReplacement(p, e, 4096)
	if err != nil {
		t.Fatal(err)
	}
	cause := errors.New("synthetic candidate failure")
	for _, full := range []bool{false, true} {
		_, err := plan.WriteCandidate(context.Background(), bytes.NewReader(l.bytes), prefixWriteFailure{cause, full})
		if !errors.Is(err, cause) || (!full && !errors.Is(err, io.ErrShortWrite)) {
			t.Fatal("write cause erased", err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var dst prefixMemoryWriter
	if _, err := plan.WriteCandidate(ctx, bytes.NewReader(l.bytes), &dst); !errors.Is(err, context.Canceled) || dst.calls != 0 {
		t.Fatal("cancellation", err)
	}
	if _, err := (PrefixReplacement{}).WriteCandidate(context.Background(), bytes.NewReader(nil), &dst); !errors.Is(err, ErrUnsupported) || dst.calls != 0 {
		t.Fatal("zero recipe wrote", err)
	}
	// Private metadata-only budget models allocate no giant log or map.
	for _, edit := range []func(*Snapshot){func(s *Snapshot) { s.FrameCount = 1 << 20 }, func(s *Snapshot) { s.EndOffset = (1 << 30) + 1 }, func(s *Snapshot) { s.IndexWork = 16 << 20 }} {
		s := p.proof.snapshot
		edit(&s)
		modeled := CommittedPrefix{&prefixProof{snapshot: s, scanError: e}}
		if _, err := PlanPrefixReplacement(modeled, e, 4096); !errors.Is(err, ErrLimit) {
			t.Fatal("budget admitted", err)
		}
	}
}
