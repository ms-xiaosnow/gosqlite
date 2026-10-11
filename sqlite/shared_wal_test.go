package sqlite_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func sharedValue(t *testing.T, q interface {
	Query(context.Context, string, ...any) (*sqlite.Rows, error)
}, id int64) string {
	t.Helper()
	r, err := q.Query(context.Background(), "SELECT text FROM t WHERE id=?", id)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if !r.Next() {
		t.Fatalf("missing row: %v", r.Err())
	}
	v := r.Values()[0].(string)
	if r.Next() || r.Err() != nil {
		t.Fatalf("extra/error: %v", r.Err())
	}
	return v
}
func TestSharedWALSnapshotsAndWriterContention(t *testing.T) {
	ctx := context.Background()
	for _, size := range []uint32{512, 4096, 65536} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "shared.db")
			opts := sqlite.Options{WALShared: true, PageSize: size, KeepWALOnClose: true}
			a, err := sqlite.Open(path, opts)
			if err != nil {
				t.Fatal(err)
			}
			defer a.Close()
			execute(t, a, "CREATE TABLE t(id INTEGER PRIMARY KEY,text TEXT)")
			execute(t, a, "CREATE INDEX ix ON t(text)")
			execute(t, a, "INSERT INTO t VALUES(1,'old'),(2,'stable')")
			b, err := sqlite.Open(path, opts)
			if err != nil {
				t.Fatal(err)
			}
			defer b.Close()
			old, err := b.Begin(ctx, sqlite.TxOptions{ReadOnly: true})
			if err != nil {
				t.Fatal(err)
			}
			if got := sharedValue(t, old, 1); got != "old" {
				t.Fatal(got)
			}
			execute(t, a, "UPDATE t SET text='new' WHERE id=1")
			if got := sharedValue(t, old, 1); got != "old" {
				t.Fatalf("snapshot changed: %s", got)
			}
			c, err := sqlite.Open(path, opts)
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			if got := sharedValue(t, c, 1); got != "new" {
				t.Fatal(got)
			}
			if err = a.Checkpoint(ctx); !errors.Is(err, sqlite.ErrBusy) {
				t.Fatalf("checkpoint with old reader: %v", err)
			}
			if err = old.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			if err = a.Checkpoint(ctx); err != nil {
				t.Fatal(err)
			}
			writer, err := a.Begin(ctx, sqlite.TxOptions{Immediate: true})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = b.Begin(ctx, sqlite.TxOptions{Immediate: true}); !errors.Is(err, sqlite.ErrBusy) {
				t.Fatalf("second writer: %v", err)
			}
			txExecute(t, writer, "UPDATE t SET text='unpublished' WHERE id=1")
			if got := sharedValue(t, b, 1); got != "new" {
				t.Fatalf("uncommitted leaked: %s", got)
			}
			if err = writer.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			if got := sharedValue(t, b, 1); got != "unpublished" {
				t.Fatal(got)
			}
			stale, err := b.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if got := sharedValue(t, stale, 1); got != "unpublished" {
				t.Fatal(got)
			}
			execute(t, a, "UPDATE t SET text='winner' WHERE id=1")
			if _, err = stale.Exec(ctx, "UPDATE t SET text='lost update' WHERE id=1"); !errors.Is(err, sqlite.ErrBusySnapshot) || !errors.Is(err, sqlite.ErrBusy) {
				t.Fatalf("stale writer promotion: %v", err)
			}
			if err = stale.Rollback(); err != nil {
				t.Fatal(err)
			}
			if got := sharedValue(t, a, 1); got != "winner" {
				t.Fatal(got)
			}
			if err = c.Close(); err != nil {
				t.Fatal(err)
			}
			if err = b.Close(); err != nil {
				t.Fatal(err)
			}
			if err = a.Checkpoint(ctx); err != nil {
				t.Fatal(err)
			}
			if err = a.Close(); err != nil {
				t.Fatal(err)
			}
			a, err = sqlite.Open(path, opts)
			if err != nil {
				t.Fatal(err)
			}
			defer a.Close()
			if got := sharedValue(t, a, 1); got != "winner" {
				t.Fatal(got)
			}
		})
	}
}

// ReadOnly+WALShared absence is covered by the explicit readonly no-create tests.
func TestSharedWALOptionBoundaries(t *testing.T) {
	for _, opts := range []sqlite.Options{{WALShared: true, WALExclusive: true}, {WALPowersafeOverwrite: true}} {
		if _, err := sqlite.Open(filepath.Join(t.TempDir(), "none.db"), opts); err == nil {
			t.Fatalf("invalid options accepted: %+v", opts)
		}
	}
}
