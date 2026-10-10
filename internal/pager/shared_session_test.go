package pager

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/wal"
	"github.com/ms-xiaosnow/gosqlite/internal/walindex"
)

// All file fixtures in this file arise from ordinary successful commits.
// No damaged files, injected failures, forced exits, or recovery replay run.
func sharedOpen(t *testing.T, path string, o Options) *SharedSession {
	t.Helper()
	s, e := OpenSharedSession(context.Background(), path, o)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() {
		if e := s.Close(); e != nil {
			t.Errorf("close: %v", e)
		}
	})
	return s
}
func sharedBump(t *testing.T, s *SharedSession) {
	t.Helper()
	tx, e := s.Begin(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	if e = tx.BumpSchemaCookie(context.Background()); e != nil {
		_ = tx.Rollback()
		t.Fatal(e)
	}
	if e = tx.Commit(context.Background()); e != nil {
		_ = tx.Rollback()
		t.Fatal(e)
	}
}
func sharedCookie(t *testing.T, r *Reader, want uint32) {
	t.Helper()
	p, e := r.ReadPage(context.Background(), 1)
	if e != nil {
		t.Fatal(e)
	}
	if got := binary.BigEndian.Uint32(p[40:44]); got != want {
		t.Fatalf("cookie=%d, want %d", got, want)
	}
}
func TestSharedWALSnapshotsPromotionAndCheckpoint(t *testing.T) {
	txPlatform(t)
	ctx := context.Background()
	for _, ps := range []uint32{512, 4096, 65536} {
		t.Run(fmt.Sprint(ps), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "shared.db")
			a := sharedOpen(t, path, Options{Create: true, PageSize: ps, KeepWALOnClose: true})
			b := sharedOpen(t, path, Options{KeepWALOnClose: true})
			r, e := a.Reader(ctx)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = a.Begin(ctx); !errors.Is(e, ErrBusy) {
				t.Fatalf("local lease: %v", e)
			}
			sharedCookie(t, r, 0)
			sharedBump(t, b)
			sharedCookie(t, r, 0)
			if e = b.Checkpoint(ctx); !errors.Is(e, ErrBusy) {
				t.Fatalf("read0 checkpoint: %v", e)
			}
			if e = r.Close(); e != nil {
				t.Fatal(e)
			}
			stale, e := a.Begin(ctx)
			if e != nil {
				t.Fatal(e)
			}
			sharedBump(t, b)
			if e = stale.PromoteWrite(ctx); !errors.Is(e, ErrBusySnapshot) || !errors.Is(e, ErrBusy) {
				t.Fatalf("snapshot promotion: %v", e)
			}
			if e = stale.Rollback(); e != nil {
				t.Fatal(e)
			}
			old, e := a.Reader(ctx)
			if e != nil {
				t.Fatal(e)
			}
			sharedCookie(t, old, 2)
			sharedBump(t, b)
			if e = b.Checkpoint(ctx); !errors.Is(e, ErrBusy) {
				t.Fatalf("old WAL reader checkpoint: %v", e)
			}
			sharedCookie(t, old, 2)
			if e = old.Close(); e != nil {
				t.Fatal(e)
			}
			if e = b.Checkpoint(ctx); e != nil {
				t.Fatal(e)
			}
			before, e := b.index.ReadHeader()
			if e != nil {
				t.Fatal(e)
			}
			dbOnly, e := a.Reader(ctx)
			if e != nil {
				t.Fatal(e)
			}
			if dbOnly.lease.sharedRead.slot != 0 {
				t.Fatal("expected main-only reader")
			}
			sharedBump(t, b)
			after, e := b.index.ReadHeader()
			if e != nil {
				t.Fatal(e)
			}
			if before.Salt == after.Salt {
				t.Fatal("fully checkpointed WAL did not reset salts")
			}
			sharedCookie(t, dbOnly, 3)
			if e = dbOnly.Close(); e != nil {
				t.Fatal(e)
			}
			fresh, e := a.Reader(ctx)
			if e != nil {
				t.Fatal(e)
			}
			sharedCookie(t, fresh, 4)
			if e = fresh.Close(); e != nil {
				t.Fatal(e)
			}
			write, e := a.Begin(ctx)
			if e != nil {
				t.Fatal(e)
			}
			if e = write.PromoteWrite(ctx); e != nil {
				t.Fatal(e)
			}
			other, e := b.Begin(ctx)
			if e != nil {
				t.Fatal(e)
			}
			if e = other.PromoteWrite(ctx); !errors.Is(e, ErrBusy) {
				t.Fatalf("two writers: %v", e)
			}
			if e = write.Rollback(); e != nil {
				t.Fatal(e)
			}
			if e = other.PromoteWrite(ctx); e != nil {
				t.Fatal(e)
			}
			if e = other.Rollback(); e != nil {
				t.Fatal(e)
			}
			if e = a.Close(); e != nil {
				t.Fatal(e)
			}
			if e = b.Close(); e != nil {
				t.Fatal(e)
			}
			// Last-close DMS causes a normal transient-index rebuild from the
			// retained, fully committed WAL.
			reopened := sharedOpen(t, path, Options{KeepWALOnClose: true})
			rr, e := reopened.Reader(ctx)
			if e != nil {
				t.Fatal(e)
			}
			sharedCookie(t, rr, 4)
			if e = rr.Close(); e != nil {
				t.Fatal(e)
			}
			if e = reopened.Checkpoint(ctx); e != nil {
				t.Fatal(e)
			}
		})
	}
}
func TestSharedWALConcurrentReadersAndWriter(t *testing.T) {
	txPlatform(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "concurrent.db")
	writer := sharedOpen(t, path, Options{Create: true, KeepWALOnClose: true})
	sharedBump(t, writer)
	var readers []*Reader
	for i := 0; i < 4; i++ {
		s := sharedOpen(t, path, Options{KeepWALOnClose: true})
		r, e := s.Reader(ctx)
		if e != nil {
			t.Fatal(e)
		}
		readers = append(readers, r)
	}
	var wg sync.WaitGroup
	for _, r := range readers {
		wg.Add(1)
		go func(r *Reader) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				p, e := r.ReadPage(ctx, 1)
				if e != nil || binary.BigEndian.Uint32(p[40:44]) != 1 {
					t.Errorf("snapshot changed: %v", e)
					return
				}
			}
		}(r)
	}
	for i := 0; i < 10; i++ {
		sharedBump(t, writer)
	}
	wg.Wait()
	for _, r := range readers {
		if e := r.Close(); e != nil {
			t.Fatal(e)
		}
	}
	if e := writer.Checkpoint(ctx); e != nil {
		t.Fatal(e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e := writer.Begin(canceled); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	if e := writer.Checkpoint(canceled); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
}
func TestSharedWALCrossesNativeIndexBlock(t *testing.T) {
	txPlatform(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "blocks.db")
	a := sharedOpen(t, path, Options{Create: true, PageSize: 512, MaxDirtyPages: 6000, KeepWALOnClose: true})
	tx, e := a.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 4200; i++ {
		if _, e = tx.AllocatePage(ctx); e != nil {
			t.Fatal(e)
		}
	}
	for n := uint32(2); n <= 4201; n++ {
		if e = tx.FreePage(ctx, n); e != nil {
			t.Fatal(e)
		}
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	h, e := a.index.ReadHeader()
	if e != nil {
		t.Fatal(e)
	}
	if h.MaxFrame <= walindex.FirstBlockFrames {
		t.Fatal("did not cross hash blocks")
	}
	b := sharedOpen(t, path, Options{KeepWALOnClose: true})
	r, e := b.Reader(ctx)
	if e != nil {
		t.Fatal(e)
	}
	for _, n := range []uint32{1, 2, 4062, 4063, 4201} {
		if _, e = r.ReadPage(ctx, n); e != nil {
			t.Fatalf("read page %d: %v", n, e)
		}
	}
	if e = r.Close(); e != nil {
		t.Fatal(e)
	}
	if e = a.Checkpoint(ctx); e != nil {
		t.Fatal(e)
	}
}

func TestSharedWALImportedSectorBoundary(t *testing.T) {
	txPlatform(t)
	ctx := context.Background()
	for _, psow := range []bool{false, true} {
		t.Run(fmt.Sprint(psow), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "import.db")
			source, e := OpenSession(ctx, path, Options{Create: true, PageSize: 512, SectorSize: 512, KeepWALOnClose: true})
			if e != nil {
				t.Fatal(e)
			}
			tx, e := source.Begin(ctx)
			if e != nil {
				t.Fatal(e)
			}
			if e = tx.BumpSchemaCookie(ctx); e != nil {
				t.Fatal(e)
			}
			if e = tx.Commit(ctx); e != nil {
				t.Fatal(e)
			}
			if e = source.Close(); e != nil {
				t.Fatal(e)
			}
			s := sharedOpen(t, path, Options{SectorSize: 4096, KeepWALOnClose: true, WALPowersafeOverwrite: psow})
			tx, e = s.Begin(ctx)
			if e != nil {
				t.Fatal(e)
			}
			if e = tx.BumpSchemaCookie(ctx); e != nil {
				t.Fatal(e)
			}
			e = tx.Commit(ctx)
			if psow {
				if e != nil {
					t.Fatal(e)
				}
			} else {
				if !errors.Is(e, ErrUnsupported) {
					t.Fatalf("unprotected imported append: %v", e)
				}
				if e = tx.Rollback(); e != nil {
					t.Fatal(e)
				}
				if e = s.Checkpoint(ctx); e != nil {
					t.Fatal(e)
				}
				sharedBump(t, s)
			}
			r, e := s.Reader(ctx)
			if e != nil {
				t.Fatal(e)
			}
			sharedCookie(t, r, 2)
			if e = r.Close(); e != nil {
				t.Fatal(e)
			}
		})
	}
}

func TestSharedWALImportedCheckpointSectorBoundary(t *testing.T) {
	txPlatform(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "neighbor-import.db")
	tx, e := OpenTx(ctx, path, Options{Create: true, PageSize: 512, SectorSize: 512})
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 15; i++ {
		if _, e = tx.AllocatePage(ctx); e != nil {
			t.Fatal(e)
		}
	}
	for n := uint32(2); n <= 16; n++ {
		if e = tx.FreePage(ctx, n); e != nil {
			t.Fatal(e)
		}
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	source, e := OpenSession(ctx, path, Options{SectorSize: 512, KeepWALOnClose: true})
	if e != nil {
		t.Fatal(e)
	}
	tx, e = source.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if e = tx.BumpSchemaCookie(ctx); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	if e = source.Close(); e != nil {
		t.Fatal(e)
	}
	conservative := sharedOpen(t, path, Options{SectorSize: 4096, KeepWALOnClose: true})
	if e = conservative.Checkpoint(ctx); !errors.Is(e, ErrUnsupported) {
		t.Fatalf("unlogged sector neighbors: %v", e)
	}
	if e = conservative.Close(); e != nil {
		t.Fatal(e)
	}
	asserted := sharedOpen(t, path, Options{SectorSize: 4096, KeepWALOnClose: true, WALPowersafeOverwrite: true})
	if e = asserted.Checkpoint(ctx); e != nil {
		t.Fatal(e)
	}
	r, e := asserted.Reader(ctx)
	if e != nil {
		t.Fatal(e)
	}
	sharedCookie(t, r, 1)
	if e = r.Close(); e != nil {
		t.Fatal(e)
	}
}

func TestSharedWALAllReadmarksPinned(t *testing.T) {
	txPlatform(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "all-marks.db")
	writer := sharedOpen(t, path, Options{Create: true, KeepWALOnClose: true})
	var readers []*Reader
	slots := map[int]bool{}
	for i := 1; i <= 5; i++ {
		sharedBump(t, writer)
		s := sharedOpen(t, path, Options{KeepWALOnClose: true})
		r, e := s.Reader(ctx)
		if e != nil {
			t.Fatal(e)
		}
		readers = append(readers, r)
		sharedCookie(t, r, uint32(i))
		slot := r.lease.sharedRead.slot
		if slot == 0 {
			t.Fatal("expected WAL readmark")
		}
		if i <= 4 {
			if slots[slot] {
				t.Fatal("staggered snapshot did not select a free mark")
			}
			slots[slot] = true
		}
	}
	if readers[4].lease.sharedRead.slot != readers[3].lease.sharedRead.slot {
		t.Fatal("fifth snapshot should share newest older readmark")
	}
	sharedBump(t, writer)
	for released := 0; released < 5; released++ {
		if e := writer.Checkpoint(ctx); !errors.Is(e, ErrBusy) {
			t.Fatalf("protected checkpoint %d: %v", released, e)
		}
		for i := released; i < 5; i++ {
			sharedCookie(t, readers[i], uint32(i+1))
		}
		if e := readers[released].Close(); e != nil {
			t.Fatal(e)
		}
	}
	if e := writer.Checkpoint(ctx); e != nil {
		t.Fatal(e)
	}
}

func TestSharedWALCommittedPrefixTailPolicyInMemory(t *testing.T) {
	h := wal.Header{Magic: wal.MagicLittleEndian, Version: wal.Version, PageSize: 512, Salt: [2]uint32{123, 456}}
	raw, e := wal.EncodeHeader(h)
	if e != nil {
		t.Fatal(e)
	}
	h, e = wal.ParseHeader(raw)
	if e != nil {
		t.Fatal(e)
	}
	page := emptyDatabasePage(512)
	page[18], page[19] = 2, 2
	committed, sum, e := wal.EncodeFrame(h, 1, 1, page, h.Checksum)
	if e != nil {
		t.Fatal(e)
	}
	pendingPage := append([]byte(nil), page...)
	binary.BigEndian.PutUint32(pendingPage[40:], 999)
	pending, _, e := wal.EncodeFrame(h, 1, 0, pendingPage, sum)
	if e != nil {
		t.Fatal(e)
	}
	full := append(append(append([]byte(nil), raw...), committed...), pending...)
	snap, e := wal.Scan(context.Background(), bytes.NewReader(full), int64(len(full)), wal.Limits{})
	if !sharedWALPrefixAllowed(snap, e) || snap.FrameCount != 1 || snap.ValidFrameCount != 2 || snap.DBSize != 1 || snap.Checksum != sum || snap.Pages[1].Frame != 1 {
		t.Fatalf("committed prefix: %+v %v", snap, e)
	}
	incomplete := full[:len(full)-1]
	snap, e = wal.Scan(context.Background(), bytes.NewReader(incomplete), int64(len(incomplete)), wal.Limits{})
	if sharedWALPrefixAllowed(snap, e) {
		t.Fatal("accepted incomplete same-salt suffix")
	}
	damaged := append([]byte(nil), full...)
	damaged[len(damaged)-1] ^= 1
	snap, e = wal.Scan(context.Background(), bytes.NewReader(damaged), int64(len(damaged)), wal.Limits{})
	if sharedWALPrefixAllowed(snap, e) {
		t.Fatal("accepted invalid same-salt suffix")
	}
	old := h
	old.Salt[0]--
	suffix, _, e := wal.EncodeFrame(old, 1, 1, page, sum)
	if e != nil {
		t.Fatal(e)
	}
	reused := append(append([]byte(nil), full...), suffix...)
	snap, e = wal.Scan(context.Background(), bytes.NewReader(reused), int64(len(reused)), wal.Limits{})
	if !sharedWALPrefixAllowed(snap, e) || snap.FrameCount != 1 || snap.ValidFrameCount != 2 || snap.Pages[1].Frame != 1 {
		t.Fatalf("ordinary rollback then old-generation suffix: %+v %v", snap, e)
	}
}
