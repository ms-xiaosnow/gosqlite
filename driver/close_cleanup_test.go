package driver

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestSharedCloseCleanupConnectorCancellationAndOrdinaryDisposal(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "ordinary.db")
	connector, e := NewConnector(path, sqlite.Options{WALShared: true, KeepWALOnClose: true})
	if e != nil {
		t.Fatal(e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if conn, e := connector.Connect(cancelled); conn != nil || !errors.Is(e, context.Canceled) {
		t.Fatal("pre-open cancellation lost", conn, e)
	}
	pool := sql.OpenDB(connector)
	defer pool.Close()
	pool.SetMaxIdleConns(0)
	for _, q := range []string{"CREATE TABLE t(id INTEGER PRIMARY KEY)", "INSERT INTO t VALUES(7)"} {
		if _, e = pool.ExecContext(ctx, q); e != nil {
			t.Fatal(e)
		}
	}
	var id int
	if e = pool.QueryRowContext(ctx, "SELECT id FROM t").Scan(&id); e != nil || id != 7 {
		t.Fatal(id, e)
	}
	if e = pool.Close(); e != nil {
		t.Fatal(e)
	}
	if len(sqlite.PendingCloseCleanups()) != 0 {
		t.Fatal("ordinary pool disposal retained cleanup")
	}
	// Reopen after normal disposal, then abandon a memory-only transaction on a
	// pinned Conn. Pool reset must roll it back before the next borrower.
	pool = sql.OpenDB(connector)
	defer pool.Close()
	pool.SetMaxOpenConns(1)
	conn, e := pool.Conn(ctx)
	if e != nil {
		t.Fatal(e)
	}
	for _, q := range []string{"BEGIN", "INSERT INTO t VALUES(8)"} {
		if _, e = conn.ExecContext(ctx, q); e != nil {
			t.Fatal(e)
		}
	}
	if e = conn.Close(); e != nil {
		t.Fatal(e)
	}
	rows, e := pool.QueryContext(ctx, "SELECT id FROM t ORDER BY id")
	if e != nil {
		t.Fatal(e)
	}
	if !rows.Next() {
		t.Fatal("reset lost committed row", rows.Err())
	}
	if e = rows.Scan(&id); e != nil || id != 7 {
		t.Fatal(id, e)
	}
	if rows.Next() {
		t.Fatal("reset leaked uncommitted row")
	}
	if e = rows.Err(); e != nil {
		t.Fatal(e)
	}
	if e = rows.Close(); e != nil {
		t.Fatal(e)
	}
	if e = pool.Close(); e != nil {
		t.Fatal(e)
	}
	if len(sqlite.PendingCloseCleanups()) != 0 {
		t.Fatal("normal reset or close retained cleanup")
	}
}
