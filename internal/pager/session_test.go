package pager

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/wal"
)

// Every on-disk fixture here is a normal successfully committed database.
// There are no I/O faults, damaged files, replay fixtures, or forced exits.
func TestSessionDescriptorLeaseAndOrdinaryRetainedClose(t *testing.T) {
	ctx := context.Background()
	for _, size := range []uint32{512, 4096, 65536} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "session.db")
			s, err := OpenSession(ctx, path, Options{Create: true, PageSize: size, KeepWALOnClose: true})
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			fd := s.file
			for i := 0; i < 3; i++ {
				tx, err := s.Begin(ctx)
				if err != nil || tx.file != fd {
					t.Fatalf("same descriptor: %v", err)
				}
				if err := s.Close(); !errors.Is(err, ErrBusy) {
					t.Fatalf("close with transaction: %v", err)
				}
				if err := tx.BumpSchemaCookie(ctx); err != nil {
					t.Fatal(err)
				}
				if err := tx.Commit(ctx); err != nil {
					t.Fatal(err)
				}
				if s.file != fd || tx.file != nil || s.active != nil {
					t.Fatal("commit changed connection ownership")
				}
				if other, err := Open(path); !errors.Is(err, ErrBusy) {
					if other != nil {
						_ = other.Close()
					}
					t.Fatalf("idle exclusive ownership: %v", err)
				}
			}
			r, err := s.Reader(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := s.Reader(ctx); !errors.Is(err, ErrBusy) {
				t.Fatalf("second lease: %v", err)
			}
			if err := s.Checkpoint(ctx); !errors.Is(err, ErrBusy) {
				t.Fatalf("checkpoint lease: %v", err)
			}
			var wg sync.WaitGroup
			for i := 0; i < 8; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for j := 0; j < 10; j++ {
						p, e := r.ReadPage(ctx, 1)
						if e != nil || binary.BigEndian.Uint32(p[40:]) != 3 {
							t.Errorf("snapshot read: %v", e)
							return
						}
					}
				}()
			}
			wg.Wait()
			if err := r.Close(); err != nil {
				t.Fatal(err)
			}
			tx, err := s.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if err = tx.BumpSchemaCookie(ctx); err != nil {
				t.Fatal(err)
			}
			if err = tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			if s.Header().SchemaCookie != 3 || s.file != fd {
				t.Fatal("rollback modified durable snapshot or descriptor")
			}
			if err = s.Close(); err != nil {
				t.Fatal(err)
			}
			s, err = OpenSession(ctx, path, Options{})
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			if s.Header().SchemaCookie != 3 {
				t.Fatal(s.Header())
			}
			if err = s.Checkpoint(ctx); err != nil {
				t.Fatal(err)
			}
			if _, err = os.Stat(path + "-wal"); !os.IsNotExist(err) {
				t.Fatal(err)
			}
			if err = s.Close(); err != nil {
				t.Fatal(err)
			}
			s, err = OpenSession(ctx, path, Options{}) // Valid mode 2 with no WAL.
			if err != nil {
				t.Fatal(err)
			}
			if err = s.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSessionCommitProtectsSectorNeighbors(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "sectors.db")
	// A schema-only database with fifteen valid freelist pages gives the
	// first modified sector unchanged neighbors that must be WAL-protected.
	tx, err := OpenTx(ctx, path, Options{Create: true, PageSize: 512})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 15; i++ {
		if _, err = tx.AllocatePage(ctx); err != nil {
			t.Fatal(err)
		}
	}
	for n := uint32(2); n <= 16; n++ {
		if err = tx.FreePage(ctx, n); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	s, err := OpenSession(ctx, path, Options{SectorSize: 4096, KeepWALOnClose: true})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tx, err = s.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.BumpSchemaCookie(ctx); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	for n := uint32(1); n <= 8; n++ {
		if _, ok := s.snapshot.Pages[n]; !ok {
			t.Fatalf("missing neighbor %d", n)
		}
	}
	if _, ok := s.snapshot.Pages[9]; ok {
		t.Fatal("unrelated sector was logged")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("main changed before checkpoint: %v", err)
	}
	st, err := s.walFile.Stat()
	if err != nil {
		t.Fatal(err)
	}
	scan, err := wal.Scan(ctx, s.walFile, st.Size(), wal.Limits{})
	if err != nil || !scan.Complete || scan.EndOffset != st.Size() {
		t.Fatalf("complete frame padding: %+v %v", scan, err)
	}
	if scan.FrameCount <= 8 {
		t.Fatal("expected complete commit-frame sector padding")
	}
	if scan.IndexWork != s.snapshot.IndexWork {
		t.Fatalf("writer and scanner work budget differ: %d != %d", scan.IndexWork, s.snapshot.IndexWork)
	}
	if err = s.Checkpoint(ctx); err != nil {
		t.Fatal(err)
	}
	if s.Header().PageCount != 16 || s.Header().SchemaCookie != 1 {
		t.Fatal(s.Header())
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestWALLogicalHeaderExtentInMemory(t *testing.T) {
	p := emptyDatabasePage(512)
	p[18], p[19] = 2, 2
	binary.BigEndian.PutUint32(p[28:], 20)
	if _, err := ParseHeader(p, 512); !errors.Is(err, ErrUnsupported) {
		t.Fatal(err)
	}
	h, err := parseHeader(p, 512, true, 20)
	if err != nil || h.PageCount != 20 {
		t.Fatalf("logical WAL extent: %+v %v", h, err)
	}
	if _, err = parseHeader(p, 512, true, 0); !errors.Is(err, ErrCorrupt) {
		t.Fatal(err)
	}
}

func TestWALSectorsBoundedBeforeReadInMemory(t *testing.T) {
	reads := 0
	_, err := walSectorImages(context.Background(), map[uint32][]byte{1: make([]byte, 512)},
		Header{PageSize: 512, PageCount: 16}, Options{SectorSize: 4096, MaxDirtyPages: 4},
		func(context.Context, uint32) ([]byte, error) { reads++; return make([]byte, 512), nil })
	if !errors.Is(err, ErrLimit) || reads != 0 {
		t.Fatalf("bounded before page I/O: %v reads=%d", err, reads)
	}
}

func TestSessionReopenedWALNormalizationPreservesSnapshot(t *testing.T) {
	ctx := context.Background()
	for _, size := range []uint32{512, 4096, 65536} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "normalization.db")
			s, err := OpenSession(ctx, path, Options{Create: true, PageSize: size, KeepWALOnClose: true})
			if err != nil {
				t.Fatal(err)
			}
			tx, err := s.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if err = tx.BumpSchemaCookie(ctx); err != nil {
				t.Fatal(err)
			}
			if err = tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			if err = s.Close(); err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(path + "-wal")
			if err != nil {
				t.Fatal(err)
			}
			s, err = OpenSession(ctx, path, Options{KeepWALOnClose: true})
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			mainFD := s.file
			oldPage, err := s.readPage(ctx, 1)
			if err != nil {
				t.Fatal(err)
			}
			s.mu.Lock()
			err = s.normalizeAppendWALLocked()
			s.mu.Unlock()
			if err != nil {
				t.Fatal(err)
			}
			after, err := os.Stat(path + "-wal")
			if err != nil {
				t.Fatal(err)
			}
			if os.SameFile(before, after) || s.file != mainFD {
				t.Fatal("normalization must replace only the WAL inode")
			}
			newPage, err := s.readPage(ctx, 1)
			if err != nil || !bytes.Equal(oldPage, newPage) {
				t.Fatalf("snapshot changed: %v", err)
			}
			scan, err := wal.Scan(ctx, s.walFile, after.Size(), wal.Limits{})
			if err != nil || !reflect.DeepEqual(scan.Pages, s.snapshot.Pages) || scan.Checksum != s.snapshot.Checksum || scan.IndexWork != s.snapshot.IndexWork {
				t.Fatalf("normalized index differs from disk scan: %v", err)
			}
			left, err := filepath.Glob(path + "-wal-pad-*")
			if err != nil || len(left) != 0 {
				t.Fatalf("normalization temporary files: %v %v", left, err)
			}
			tx, err = s.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if err = tx.BumpSchemaCookie(ctx); err != nil {
				t.Fatal(err)
			}
			if err = tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			if s.Header().SchemaCookie != 2 {
				t.Fatal(s.Header())
			}
			if err = s.Checkpoint(ctx); err != nil {
				t.Fatal(err)
			}
			if err = s.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
