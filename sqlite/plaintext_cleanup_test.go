package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	driver "github.com/ms-xiaosnow/gosqlite/driver"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// This is exclusively an intact ordinary lifecycle, with no failed native I/O,
// corrupt database, namespace mutation, missing-file probe or recovery action.
func TestPlaintextCleanupOrdinaryReaderRowsAndReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "ordinary.db")
	db, e := sqlite.Open(path, sqlite.Options{})
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	for _, q := range []string{"CREATE TABLE t(id INTEGER PRIMARY KEY,v TEXT)", "INSERT INTO t VALUES(1,'kept')"} {
		if _, e = db.Exec(ctx, q); e != nil {
			t.Fatal(e)
		}
	}
	if entries, e := db.Schema(ctx); e != nil || len(entries) != 1 {
		t.Fatal(entries, e)
	}
	if _, e = db.Query(ctx, "SELECT absent FROM t"); e == nil {
		t.Fatal("expected ordinary SQL planning error")
	}
	rows, e := db.Query(ctx, "SELECT id,v FROM t")
	if e != nil {
		t.Fatal(e)
	}
	if !rows.Next() {
		t.Fatal(rows.Err())
	}
	if e = db.Close(); e != nil {
		t.Fatal(e)
	}
	if rows.Next() || !errors.Is(rows.Err(), sqlite.ErrClosed) {
		t.Fatal("Rows not invalidated", rows.Err())
	}
	if e = rows.Close(); e != nil {
		t.Fatal(e)
	}
	if e = db.Close(); e != nil {
		t.Fatal(e)
	}
	if len(sqlite.PendingPlaintextCleanups()) != 0 {
		t.Fatal("ordinary Reader retained cleanup")
	}
	db, e = sqlite.Open(path, sqlite.Options{ReadOnly: true, NoCreate: true})
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, e = db.Query(canceled, "SELECT id FROM t"); !errors.Is(e, context.Canceled) {
		t.Fatal(e)
	}
	tx, e := db.Begin(ctx, sqlite.TxOptions{ReadOnly: true})
	if e != nil {
		t.Fatal(e)
	}
	rows, e = tx.Query(ctx, "SELECT id FROM t")
	if e != nil {
		t.Fatal(e)
	}
	if !rows.Next() {
		t.Fatal(rows.Err())
	}
	if e = tx.Rollback(); e != nil {
		t.Fatal(e)
	}
	if rows.Next() || !errors.Is(rows.Err(), sqlite.ErrTxDone) {
		t.Fatal("transaction rows not invalidated", rows.Err())
	}
	if e = db.Close(); e != nil {
		t.Fatal(e)
	}
	if len(sqlite.PendingPlaintextCleanups()) != 0 {
		t.Fatal("ordinary live reader cleanup pending")
	}
}
func TestPlaintextCleanupOrdinaryDatabaseSQLZeroIdle(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "pool.db")
	connector, e := driver.NewConnector(path, sqlite.Options{})
	if e != nil {
		t.Fatal(e)
	}
	pool := sql.OpenDB(connector)
	pool.SetMaxIdleConns(0)
	defer pool.Close()
	if _, e = pool.ExecContext(ctx, "CREATE TABLE t(id INTEGER PRIMARY KEY,v TEXT)"); e != nil {
		t.Fatal(e)
	}
	if _, e = pool.ExecContext(ctx, "INSERT INTO t VALUES(1,'kept')"); e != nil {
		t.Fatal(e)
	}
	rows, e := pool.QueryContext(ctx, "SELECT v FROM t")
	if e != nil {
		t.Fatal(e)
	}
	if !rows.Next() {
		t.Fatal(rows.Err())
	}
	var value string
	if e = rows.Scan(&value); e != nil || value != "kept" {
		t.Fatal(value, e)
	}
	if e = rows.Close(); e != nil {
		t.Fatal(e)
	}
	if e = pool.Close(); e != nil {
		t.Fatal(e)
	}
	if len(sqlite.PendingPlaintextCleanups()) != 0 {
		t.Fatal("ordinary pool disposal pending")
	}
}
