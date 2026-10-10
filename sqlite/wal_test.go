package sqlite_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestNativeWALConnectionLifecycle(t *testing.T) {
	ctx := context.Background()
	for _, size := range []uint32{512, 4096, 65536} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "wal.db")
			db, err := sqlite.Open(path, sqlite.Options{PageSize: size, WALExclusive: true, KeepWALOnClose: true})
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			for _, q := range []string{"CREATE TABLE t(id INTEGER PRIMARY KEY, text TEXT NOT NULL)", "CREATE INDEX ix ON t(text)", "INSERT INTO t VALUES(1,'数据库'),(2,'second')"} {
				if _, err := db.Exec(ctx, q); err != nil {
					t.Fatal(q, err)
				}
			}
			if _, err := sqlite.Open(path, sqlite.Options{ReadOnly: true}); !errors.Is(err, sqlite.ErrBusy) {
				t.Fatalf("idle exclusive lock: %v", err)
			}
			rows, err := db.Query(ctx, "SELECT id,text FROM t ORDER BY id")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.Query(ctx, "SELECT 1"); !errors.Is(err, sqlite.ErrBusy) {
				t.Fatalf("second reader: %v", err)
			}
			if _, err := db.Begin(ctx); !errors.Is(err, sqlite.ErrBusy) {
				t.Fatalf("begin with reader: %v", err)
			}
			if err := db.Checkpoint(ctx); !errors.Is(err, sqlite.ErrBusy) {
				t.Fatalf("checkpoint with reader: %v", err)
			}
			var count int
			for rows.Next() {
				count++
			}
			if err := rows.Err(); err != nil {
				t.Fatal(err)
			}
			if count != 2 {
				t.Fatal(count)
			}
			tx, err := db.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := tx.Exec(ctx, "INSERT INTO t VALUES(3,'rollback')"); err != nil {
				t.Fatal(err)
			}
			if err := tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			ro, err := db.Begin(ctx, sqlite.TxOptions{ReadOnly: true})
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ro.Exec(ctx, "DELETE FROM t"); !errors.Is(err, sqlite.ErrReadOnly) {
				t.Fatal(err)
			}
			if err := ro.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			canceled, cancel := context.WithCancel(ctx)
			cancel()
			if err := db.Checkpoint(canceled); !errors.Is(err, context.Canceled) {
				t.Fatal(err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			walInfo, err := os.Stat(path + "-wal")
			if err != nil || walInfo.Size() <= 32 {
				t.Fatalf("normal retained WAL: %v %v", walInfo, err)
			}
			db, err = sqlite.Open(path, sqlite.Options{WALExclusive: true, NoCreate: true})
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			rows, err = db.Query(ctx, "SELECT id,text FROM t ORDER BY id")
			if err != nil {
				t.Fatal(err)
			}
			count = 0
			for rows.Next() {
				count++
			}
			if rows.Err() != nil || count != 2 {
				t.Fatalf("reopen rows=%d err=%v", count, rows.Err())
			}
			if err := db.Checkpoint(ctx); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(path + "-wal"); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("checkpoint WAL removal: %v", err)
			}
			if _, err := db.Exec(ctx, "INSERT INTO t VALUES(4,'new generation')"); err != nil {
				t.Fatal(err)
			}
			rows, err = db.Query(ctx, "SELECT id FROM t")
			if err != nil {
				t.Fatal(err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			if rows.Next() || !errors.Is(rows.Err(), sqlite.ErrClosed) {
				t.Fatalf("close invalidates rows: %v", rows.Err())
			}
			if _, err := os.Stat(path + "-wal"); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("normal close WAL removal: %v", err)
			}
		})
	}
}

func TestWALOptionsAndCanceledOpen(t *testing.T) {
	for _, opts := range []sqlite.Options{{ReadOnly: true, WALExclusive: true}, {KeepWALOnClose: true}} {
		path := filepath.Join(t.TempDir(), "absent.db")
		if _, err := sqlite.Open(path, opts); err == nil {
			t.Fatal("invalid options accepted")
		}
		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	path := filepath.Join(t.TempDir(), "absent.db")
	if _, err := sqlite.OpenContext(ctx, path, sqlite.Options{WALExclusive: true}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
}
