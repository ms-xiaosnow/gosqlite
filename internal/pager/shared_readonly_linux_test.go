//go:build linux

package pager

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// All fixtures use ordinary successful creation, commits, locks and closes.
// No on-disk mutation, native failure injection, or oracle is used by these roots.
func openReadOnlySharedNormal(t *testing.T, path string) *SharedSession {
	t.Helper()
	admission, err := ReserveSharedClose()
	if err != nil { t.Fatal(err) }
	defer admission.Cancel()
	s, err := OpenReadOnlySharedSessionAdmitted(context.Background(), path, Options{}, admission)
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { if err := s.Close(); err != nil { t.Error(err) } })
	return s
}
func TestReadOnlySharedWALOlderReadmarkSnapshot(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "older.db")
	writer := sharedOpen(t, path, Options{Create: true, KeepWALOnClose: true})
	sharedBump(t, writer)
	keeper := sharedOpen(t, path, Options{KeepWALOnClose: true})
	old, err := keeper.Reader(ctx)
	if err != nil { t.Fatal(err) }
	defer old.Close()
	sharedBump(t, writer)
	live, err := writer.index.ReadHeader()
	if err != nil { t.Fatal(err) }
	reader := openReadOnlySharedNormal(t, path)
	snapshot, err := reader.Reader(ctx)
	if err != nil { t.Fatal(err) }
	defer snapshot.Close()
	selected := snapshot.lease.sharedRead
	mark, err := reader.index.ReadMark(selected.slot)
	if err != nil || selected.slot == 0 || mark >= live.MaxFrame || selected.hdr != live {
		t.Fatalf("older protective mark changed logical snapshot: selected=%+v mark=%d live=%+v err=%v", selected, mark, live, err)
	}
	sharedCookie(t, snapshot, 2)
	if err := old.Close(); err != nil { t.Fatal(err) }
	sharedBump(t, writer)
	if err := writer.Checkpoint(ctx); !errors.Is(err, ErrBusy) { t.Fatal("readonly snapshot did not bound checkpoint", err) }
	sharedCookie(t, snapshot, 2)
	if err := snapshot.Close(); err != nil { t.Fatal(err) }
	fresh, err := reader.Reader(ctx)
	if err != nil { t.Fatal(err) }
	sharedCookie(t, fresh, 3)
	if err := fresh.Close(); err != nil { t.Fatal(err) }
	if err := writer.Checkpoint(ctx); err != nil { t.Fatal(err) }
}

func TestReadOnlySharedWALFullyBackfilledSlotZero(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "read-zero.db")
	writer := sharedOpen(t, path, Options{Create: true, KeepWALOnClose: true})
	sharedBump(t, writer)
	if err := writer.Checkpoint(ctx); err != nil { t.Fatal(err) }
	reader := openReadOnlySharedNormal(t, path)
	snapshot, err := reader.Reader(ctx)
	if err != nil { t.Fatal(err) }
	defer snapshot.Close()
	if snapshot.lease.sharedRead.slot != 0 { t.Fatal("fully backfilled snapshot did not select slot zero") }
	sharedCookie(t, snapshot, 1)
	sharedBump(t, writer)
	if err := writer.Checkpoint(ctx); !errors.Is(err, ErrBusy) { t.Fatal(err) }
	sharedCookie(t, snapshot, 1)
	if err := snapshot.Close(); err != nil { t.Fatal(err) }
	fresh, err := reader.Reader(ctx)
	if err != nil { t.Fatal(err) }
	sharedCookie(t, fresh, 2)
	if err := fresh.Close(); err != nil { t.Fatal(err) }
	if err := writer.Checkpoint(ctx); err != nil { t.Fatal(err) }
}

func TestReadOnlySharedWALNormalConcurrentReaders(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "concurrent-readonly.db")
	writer := sharedOpen(t, path, Options{Create: true, KeepWALOnClose: true})
	sharedBump(t, writer)
	// A normal writable read seeds a reusable mark after the first commit.
	seed, err := writer.Reader(ctx)
	if err != nil { t.Fatal(err) }
	if err := seed.Close(); err != nil { t.Fatal(err) }
	var readers []*Reader
	for i := 0; i < 3; i++ {
		s := openReadOnlySharedNormal(t, path)
		r, err := s.Reader(ctx)
		if err != nil { t.Fatal(err) }
		readers = append(readers, r)
		defer r.Close()
	}
	var wg sync.WaitGroup
	for _, r := range readers {
		wg.Add(1)
		go func(r *Reader) {
			defer wg.Done()
			for i := 0; i < 20; i++ {
				page, err := r.ReadPage(ctx, 1)
				if err != nil { t.Error(err); return }
				if binary.BigEndian.Uint32(page[40:44]) != 1 { t.Error("snapshot changed"); return }
			}
		}(r)
	}
	for i := 0; i < 4; i++ { sharedBump(t, writer) }
	wg.Wait()
	for _, r := range readers { if err := r.Close(); err != nil { t.Fatal(err) } }
	if err := writer.Checkpoint(ctx); err != nil { t.Fatal(err) }
}

func TestReadOnlySharedWALNormalRefusals(t *testing.T) {
	ctx := context.Background()
	open := func(ctx context.Context, path string) (*SharedSession, error) {
		a, err := ReserveSharedClose()
		if err != nil { return nil, err }
		defer a.Cancel()
		return OpenReadOnlySharedSessionAdmitted(ctx, path, Options{}, a)
	}
	path := filepath.Join(t.TempDir(), "absent.db")
	if s, err := open(ctx, path); err == nil { s.Close(); t.Fatal("created missing family") }
	if entries, err := os.ReadDir(filepath.Dir(path)); err != nil || len(entries) != 0 { t.Fatal("no-create refusal changed directory", entries, err) }
	writer := sharedOpen(t, path, Options{Create: true, KeepWALOnClose: true})
	sharedBump(t, writer)
	// The first normal commit follows a slot-zero snapshot and has no reusable
	// WAL mark yet; readonly must refuse without synthesizing one.
	if s, err := open(ctx, path); !errors.Is(err, ErrUnsupported) { if s != nil { s.Close() }; t.Fatal("no-mark live generation accepted", err) }
	if err := writer.Close(); err != nil { t.Fatal(err) }
	// Retained bytes are intact, but the last normal owner has released DMS.
	if s, err := open(ctx, path); !errors.Is(err, ErrUnsupported) { if s != nil { s.Close() }; t.Fatal("cold DMS accepted", err) }
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if s, err := open(canceled, path); !errors.Is(err, context.Canceled) { if s != nil { s.Close() }; t.Fatal(err) }
	rollback := filepath.Join(t.TempDir(), "rollback.db")
	tx, err := OpenTx(ctx, rollback, Options{Create: true})
	if err != nil { t.Fatal(err) }
	if err := tx.Commit(ctx); err != nil { t.Fatal(err) }
	if s, err := open(ctx, rollback); !errors.Is(err, ErrUnsupported) { if s != nil { s.Close() }; t.Fatal("mode conversion accepted", err) }
	// The no-mark guard above uses an intact live generation; malformed-index
	// source/model cases never write invalid SHM fixture bytes.
}
