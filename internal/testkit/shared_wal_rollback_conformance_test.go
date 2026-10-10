//go:build oracle

package testkit_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/wal"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// This exercises an ordinary successful SQLite cache spill, explicit ROLLBACK,
// and normal .quit. Every disk byte comes from ordinary database operations;
// there are no interrupted processes, damaged fixtures, or injected failures.
func TestOracleSharedWALSpilledRollbackNormalReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "spilled-rollback.db")
	opts := sqlite.Options{WALShared: true, PageSize: 4096, KeepWALOnClose: true}
	a, err := sqlite.Open(path, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	writeExec(t, a, "CREATE TABLE t(id INTEGER PRIMARY KEY,text TEXT)")
	writeExec(t, a, "INSERT INTO t VALUES(1,'original')")
	p := startLiveOracle(t, path)
	p.run("WITH RECURSIVE n(id) AS (VALUES(2) UNION ALL SELECT id+1 FROM n WHERE id<200) INSERT INTO t SELECT id,replace(printf('%3500s',''),' ','x') FROM n;\nPRAGMA wal_checkpoint(TRUNCATE);\nPRAGMA cache_size=5;\nPRAGMA cache_spill=ON;")
	// Use a conservatively padded native prefix so the later append needs no
	// assertion about the official connection's storage PSOW assumptions.
	writeExec(t, a, "UPDATE t SET text='committed' WHERE id=1")
	// Retain the committed generation so the official writer appends rather
	// than resetting it before its rolled-back transaction.
	snapshot, err := a.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer snapshot.Rollback()
	p.run("BEGIN IMMEDIATE;\nUPDATE t SET text=CASE WHEN id=1 THEN 'discarded' ELSE replace(text,'x','y') END;\nROLLBACK;")
	liveWant(t, p, "SELECT text FROM t WHERE id=1;", "committed")
	p.close()
	if err = snapshot.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err = a.Close(); err != nil {
		t.Fatal(err)
	}

	// Prove this ordinary lifecycle produced the specific checksum-valid,
	// same-salt uncommitted suffix, not another cold-tail category.
	f, err := os.Open(path + "-wal")
	if err != nil {
		t.Fatal(err)
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		t.Fatal(err)
	}
	scan, scanErr := wal.Scan(ctx, f, st.Size(), wal.Limits{})
	if err = f.Close(); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(scanErr, wal.ErrTail) || scan.Tail != wal.TailUncommitted || scan.FrameCount == 0 || scan.ValidFrameCount <= scan.FrameCount {
		t.Fatalf("ordinary rollback fixture: committed=%d valid=%d tail=%s: %v", scan.FrameCount, scan.ValidFrameCount, scan.Tail, scanErr)
	}

	b, err := sqlite.Open(path, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	if got := sharedOracleValue(t, b); got != "committed" {
		t.Fatalf("rolled-back value became visible: %q", got)
	}
	writeExec(t, b, "UPDATE t SET text='native' WHERE id=1")
	if err = b.Close(); err != nil {
		t.Fatal(err)
	}
	c, err := sqlite.Open(path, opts)
	if err != nil {
		t.Fatalf("normal reopen after replacing the discarded suffix: %v", err)
	}
	defer c.Close()
	if got := sharedOracleValue(t, c); got != "native" {
		t.Fatalf("small native commit was lost: %q", got)
	}
	if err = c.Checkpoint(ctx); err != nil {
		t.Fatal(err)
	}
	if err = c.Close(); err != nil {
		t.Fatal(err)
	}
	writeIntegrity(t, path)
}
