package pager

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

func txPlatform(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("native SQLite-compatible write locks currently Linux-only")
	}
}
func createEmpty(t *testing.T, size uint32) string {
	t.Helper()
	txPlatform(t)
	path := filepath.Join(t.TempDir(), "ordinary.db")
	tx, err := OpenTx(context.Background(), path, Options{Create: true, PageSize: size})
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = tx.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}
func openTxTest(t *testing.T, path string, opts Options) *Tx {
	t.Helper()
	txPlatform(t)
	tx, err := OpenTx(context.Background(), path, opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := tx.Close(); err != nil {
			t.Error(err)
		}
	})
	return tx
}
func fileBytes(t *testing.T, path string) []byte {
	t.Helper()
	b, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func noJournal(t *testing.T, path string) {
	t.Helper()
	if _, e := os.Stat(path + "-journal"); !os.IsNotExist(e) {
		t.Fatalf("unexpected journal: %v", e)
	}
}

func TestTxCreatePageSizesAndReadSnapshot(t *testing.T) {
	for _, size := range []uint32{512, 4096, 65536} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			path := createEmpty(t, size)
			before := fileBytes(t, path)
			r, err := Open(path)
			if err != nil {
				t.Fatal(err)
			}
			if r.Header().PageSize != size || r.Header().PageCount != 1 || r.Header().ChangeCounter != 1 || r.Header().VersionValidFor != 1 {
				t.Fatal(r.Header())
			}
			p, err := r.ReadPage(context.Background(), 1)
			if err != nil {
				t.Fatal(err)
			}
			if p[100] != 13 || binary.BigEndian.Uint16(p[103:]) != 0 || len(p) != int(size) {
				t.Fatal("invalid empty schema root")
			}
			if err = r.Close(); err != nil {
				t.Fatal(err)
			}
			tx := openTxTest(t, path, Options{})
			p, err = tx.ReadPage(context.Background(), 1)
			if err != nil {
				t.Fatal(err)
			}
			p[200] = 12
			again, err := tx.ReadPage(context.Background(), 1)
			if err != nil || again[200] != 0 {
				t.Fatal("read page aliases storage", err)
			}
			if err = tx.Commit(context.Background()); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, fileBytes(t, path)) {
				t.Fatal("read-only transaction changed bytes")
			}
			noJournal(t, path)
		})
	}
}

func TestTxCommitPreservesUnrelatedHeaderAndPages(t *testing.T) {
	txPlatform(t)
	ctx := context.Background()
	for _, size := range []int{512, 4096, 65536} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			_, original := fixture(t, size)
			path := filepath.Join(t.TempDir(), "copy.db")
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			tx := openTxTest(t, path, Options{})
			old := tx.Header()
			if err := tx.BumpSchemaCookie(ctx); err != nil {
				t.Fatal(err)
			}
			if err := tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			got := fileBytes(t, path)
			want := append([]byte(nil), original...)
			binary.BigEndian.PutUint32(want[24:], old.ChangeCounter+1)
			binary.BigEndian.PutUint32(want[40:], old.SchemaCookie+1)
			binary.BigEndian.PutUint32(want[92:], old.ChangeCounter+1)
			if !bytes.Equal(got, want) {
				t.Fatal("commit changed unrelated database bytes")
			}
			if tx.Header().ChangeCounter != old.ChangeCounter+1 || tx.Header().SchemaCookie != old.SchemaCookie+1 {
				t.Fatal(tx.Header())
			}
			noJournal(t, path)
		})
	}
}

func TestTxRollbackAndStatementUndo(t *testing.T) {
	ctx := context.Background()
	path := createEmpty(t, 512)
	before := fileBytes(t, path)
	tx := openTxTest(t, path, Options{})
	if err := tx.BumpSchemaCookie(ctx); err != nil {
		t.Fatal(err)
	}
	prior, err := tx.ReadPage(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("ordinary statement constraint failure")
	err = tx.Statement(ctx, func() error {
		for i := 0; i < 9; i++ {
			if _, e := tx.AllocatePage(ctx); e != nil {
				return e
			}
		}
		if e := tx.FreePage(ctx, 3); e != nil {
			return e
		}
		if e := tx.BumpSchemaCookie(ctx); e != nil {
			return e
		}
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	after, err := tx.ReadPage(ctx, 1)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(prior, after) || tx.Header().PageCount != 1 || tx.Header().SchemaCookie != 1 || len(tx.free) != 0 {
		t.Fatal("statement undo failed")
	}
	if !bytes.Equal(before, fileBytes(t, path)) {
		t.Fatal("uncommitted writes reached disk")
	}
	// A succeeding statement after a failed one remains usable.
	if err = tx.Statement(ctx, func() error { return tx.BumpSchemaCookie(ctx) }); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, fileBytes(t, path)) {
		t.Fatal("rollback changed database bytes")
	}
	noJournal(t, path)
}

func TestTxFreelistAllocateFreeReuse(t *testing.T) {
	ctx := context.Background()
	path := createEmpty(t, 512)
	tx := openTxTest(t, path, Options{})
	const n = 150 // More than one SQLite 512-byte freelist trunk.
	for i := 0; i < n; i++ {
		p, e := tx.AllocatePage(ctx)
		if e != nil || p != uint32(i+2) {
			t.Fatalf("allocate %d: %d %v", i, p, e)
		}
	}
	for i := 0; i < n; i++ {
		if e := tx.FreePage(ctx, uint32(i+2)); e != nil {
			t.Fatal(e)
		}
	}
	if err := tx.FreePage(ctx, 1); !errors.Is(err, ErrCorrupt) {
		t.Fatal(err)
	}
	if err := tx.FreePage(ctx, 2); !errors.Is(err, ErrMisuse) {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	tx = openTxTest(t, path, Options{})
	seen := map[uint32]bool{}
	for i := 0; i < n; i++ {
		p, e := tx.AllocatePage(ctx)
		if e != nil {
			t.Fatal(e)
		}
		if p < 2 || p > n+1 || seen[p] {
			t.Fatalf("bad reused page %d", p)
		}
		seen[p] = true
	}
	if tx.Header().PageCount != n+1 {
		t.Fatal("freelist ignored")
	}
	for p := range seen {
		if e := tx.FreePage(ctx, p); e != nil {
			t.Fatal(e)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	tx = openTxTest(t, path, Options{})
	if len(tx.free) != n {
		t.Fatal("lost freelist pages", len(tx.free))
	}
}

func TestTxBusyCommitIsRetryableWithoutSideEffects(t *testing.T) {
	ctx := context.Background()
	path := createEmpty(t, 4096)
	before := fileBytes(t, path)
	reader, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	writer := openTxTest(t, path, Options{})
	if err = writer.BumpSchemaCookie(ctx); err != nil {
		t.Fatal(err)
	}
	competing := openTxTest(t, path, Options{})
	if err = competing.PromoteWrite(ctx); !errors.Is(err, ErrBusy) {
		t.Fatal(err)
	}
	if err = competing.Close(); err != nil {
		t.Fatal(err)
	}
	if err = writer.Commit(ctx); !errors.Is(err, ErrBusy) {
		t.Fatal(err)
	}
	if !bytes.Equal(before, fileBytes(t, path)) {
		t.Fatal("BUSY changed database")
	}
	noJournal(t, path)
	// Failed promotion released PENDING, so fresh readers remain possible.
	another, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	another.Close()
	if _, err = reader.ReadPage(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if err = reader.Close(); err != nil {
		t.Fatal(err)
	}
	if err = writer.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if writer.Header().ChangeCounter != 2 {
		t.Fatal("BUSY changed commit counter", writer.Header())
	}
	noJournal(t, path)
}

func TestTxLimitsAndCancellationRemainAtomic(t *testing.T) {
	ctx := context.Background()
	path := createEmpty(t, 4096)
	before := fileBytes(t, path)
	tx := openTxTest(t, path, Options{MaxDirtyPages: 2})
	if _, err := tx.AllocatePage(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.AllocatePage(ctx); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	if tx.Header().PageCount != 2 {
		t.Fatal("failed allocation changed page count")
	}
	if err := tx.FreePage(ctx, 2); err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := tx.Commit(canceled); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if !bytes.Equal(before, fileBytes(t, path)) {
		t.Fatal("canceled commit changed bytes")
	}
	noJournal(t, path)
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestTxStatementPanicAndCancellationUndo(t *testing.T) {
	ctx := context.Background()
	path := createEmpty(t, 4096)
	tx := openTxTest(t, path, Options{})
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("expected callback panic")
			}
		}()
		_ = tx.Statement(ctx, func() error {
			if err := tx.BumpSchemaCookie(ctx); err != nil {
				t.Fatal(err)
			}
			panic("caller panic")
		})
	}()
	if tx.Header().SchemaCookie != 0 {
		t.Fatal("panic did not undo")
	}
	cctx, cancel := context.WithCancel(ctx)
	if err := tx.Statement(cctx, func() error {
		if err := tx.BumpSchemaCookie(ctx); err != nil {
			return err
		}
		cancel()
		return nil
	}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if tx.Header().SchemaCookie != 0 {
		t.Fatal("cancellation did not undo")
	}
}

func TestTxConcurrentReadsAndWritesRaceSafe(t *testing.T) {
	ctx := context.Background()
	path := createEmpty(t, 4096)
	tx := openTxTest(t, path, Options{})
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Go(func() {
			for j := 0; j < 30; j++ {
				p, e := tx.ReadPage(ctx, 1)
				if e != nil {
					t.Error(e)
					return
				}
				if len(p) != 4096 {
					t.Error("page size")
				}
				_ = tx.Header()
				if e = tx.BumpSchemaCookie(ctx); e != nil {
					t.Error(e)
					return
				}
			}
		})
	}
	wg.Wait()
	if tx.Header().SchemaCookie != 240 {
		t.Fatal(tx.Header())
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestJournalSectorNeighborsAndNewPageNeighbors(t *testing.T) {
	tx := &Tx{header: Header{PageSize: 512}, originalPages: 10, opts: Options{SectorSize: 4096}}
	for _, tt := range []struct{ dirty, want []uint32 }{
		{[]uint32{2}, []uint32{1, 2, 3, 4, 5, 6, 7, 8}},
		{[]uint32{11}, []uint32{9, 10}},
		{[]uint32{17}, nil},
	} {
		got := tx.journalPages(tt.dirty)
		if len(got) != len(tt.want) {
			t.Fatalf("%v => %v want %v", tt.dirty, got, tt.want)
		}
		for i := range got {
			if got[i] != tt.want[i] {
				t.Fatalf("%v => %v want %v", tt.dirty, got, tt.want)
			}
		}
	}
}

func TestJournalSectorArithmeticAtMaximumPageInMemory(t *testing.T) {
	tx := &Tx{header: Header{PageSize: 512}, originalPages: 0xfffffffe, opts: Options{SectorSize: 65536}}
	got := tx.journalPages([]uint32{0xfffffffe})
	if len(got) != 126 || got[0] != 0xffffff81 || got[len(got)-1] != 0xfffffffe {
		t.Fatalf("sector boundary overflow: %v", got)
	}
	lock := lockPage(512)
	tx.originalPages = lock + 130
	got = tx.journalPages([]uint32{lock + 1})
	for _, n := range got {
		if n == lock {
			t.Fatal("lock-byte page journaled")
		}
	}
}

func TestTxConcurrentEmptyCreationAndCanceledOpen(t *testing.T) {
	txPlatform(t)
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "new.db")
	first := openTxTest(t, path, Options{Create: true})
	if second, err := OpenTx(ctx, path, Options{Create: true}); !errors.Is(err, ErrBusy) {
		if second != nil {
			second.Close()
		}
		t.Fatal(err)
	}
	if st, err := os.Stat(path); err != nil || st.Size() != 0 {
		t.Fatal("new database written before commit", err)
	}
	noJournal(t, path)
	if err := first.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	absent := filepath.Join(t.TempDir(), "canceled.db")
	if tx, err := OpenTx(canceled, absent, Options{Create: true}); !errors.Is(err, context.Canceled) {
		if tx != nil {
			tx.Close()
		}
		t.Fatal(err)
	}
	if _, err := os.Stat(absent); !os.IsNotExist(err) {
		t.Fatal("canceled open created a file", err)
	}
}

func TestValidateTxOptionsWithoutIO(t *testing.T) {
	for _, opts := range []Options{{}, {PageSize: 512, SectorSize: 65536}, {PageSize: 65536, SectorSize: 512}} {
		if err := ValidateOptions(opts); err != nil {
			t.Fatal(err)
		}
	}
	for _, opts := range []Options{{PageSize: 256}, {PageSize: 513}, {PageSize: 131072}, {SectorSize: 256}, {SectorSize: 513}, {SectorSize: 131072}} {
		if err := ValidateOptions(opts); !errors.Is(err, ErrUnsupported) {
			t.Fatalf("%+v: %v", opts, err)
		}
	}
}
