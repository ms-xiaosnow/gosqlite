package sqlite_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestCloseCleanupOrdinarySharedRowsTxAndReopen(t *testing.T) {
	ctx := context.Background()
	for _, keep := range []bool{false, true} {
		for _, active := range []string{"rows", "tx"} {
			t.Run(active+map[bool]string{false: "checkpoint", true: "keep"}[keep], func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "ordinary.db")
				db, e := sqlite.Open(path, sqlite.Options{WALShared: true, KeepWALOnClose: keep})
				if e != nil {
					t.Fatal(e)
				}
				defer db.Close()
				for _, q := range []string{"CREATE TABLE t(id INTEGER PRIMARY KEY, value TEXT)", "INSERT INTO t VALUES(1,'kept')"} {
					if _, e = db.Exec(ctx, q); e != nil {
						t.Fatal(e)
					}
				}
				var rows *sqlite.Rows
				if active == "tx" {
					tx, e := db.Begin(ctx, sqlite.TxOptions{})
					if e != nil {
						t.Fatal(e)
					}
					if _, e = tx.Exec(ctx, "INSERT INTO t VALUES(2,'rolled back')"); e != nil {
						t.Fatal(e)
					}
					rows, e = tx.Query(ctx, "SELECT id FROM t ORDER BY id")
					if e != nil {
						t.Fatal(e)
					}
				} else {
					rows, e = db.Query(ctx, "SELECT id FROM t")
					if e != nil {
						t.Fatal(e)
					}
				}
				if !rows.Next() {
					t.Fatal(rows.Err())
				}
				if e = db.Close(); e != nil {
					t.Fatal(e)
				}
				if rows.Next() || !errors.Is(rows.Err(), sqlite.ErrClosed) {
					t.Fatal("rows were not invalidated", rows.Err())
				}
				if e = db.Close(); e != nil {
					t.Fatal(e)
				}
				if _, e = db.Exec(ctx, "INSERT INTO t VALUES(3,'closed')"); !errors.Is(e, sqlite.ErrClosed) {
					t.Fatal(e)
				}
				if len(sqlite.PendingCloseCleanups()) != 0 {
					t.Fatal("ordinary close retained pending owners")
				}
				reopened, e := sqlite.Open(path, sqlite.Options{WALShared: true})
				if e != nil {
					t.Fatal(e)
				}
				defer reopened.Close()
				r, e := reopened.Query(ctx, "SELECT id, value FROM t ORDER BY id")
				if e != nil {
					t.Fatal(e)
				}
				if !r.Next() || r.Values()[0] != int64(1) || r.Values()[1] != "kept" || r.Next() {
					t.Fatal("ordinary close changed committed result", r.Err())
				}
				if e = r.Close(); e != nil {
					t.Fatal(e)
				}
				if e = reopened.Close(); e != nil {
					t.Fatal(e)
				}
			})
		}
	}
}

func TestCloseCleanupPublicNoTokenAndTraversalRefusal(t *testing.T) {
	if e := sqlite.RetryCloseCleanup(nil); e != nil {
		t.Fatal(e)
	}
	if e := sqlite.RetryCloseCleanup(errors.New("ordinary")); e != nil {
		t.Fatal(e)
	}
	err := sqlite.RetryCloseCleanup(cyclicCleanupInput{})
	var detail *sqlite.Error
	if !errors.Is(err, sqlite.ErrMemoryLimit) || !errors.As(err, &detail) || detail.Code != sqlite.CodeNoMem || errors.Is(err, sqlite.ErrCloseCleanupPending) {
		t.Fatal(err)
	}
}
