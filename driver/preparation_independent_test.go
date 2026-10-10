package driver

import (
	"context"
	"errors"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
	"testing"
)

func TestReviewPreparationDriverPreparedLockAndReset(t *testing.T) {
	db, _ := writableDatabase(t)
	db.SetMaxOpenConns(2)
	ctx := context.Background()
	a, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	mustExec(t, a, "CREATE TABLE t(id INTEGER PRIMARY KEY,v)")
	mustExec(t, a, "INSERT INTO t VALUES(1,10),(2,20)")
	valid, err := a.PrepareContext(ctx, "UPDATE t SET v=? WHERE id=?")
	if err != nil {
		t.Fatal(err)
	}
	defer valid.Close()
	invalid, err := a.PrepareContext(ctx, "UPDATE t SET missing=?")
	if err != nil {
		t.Fatal(err)
	}
	defer invalid.Close()
	mustExec(t, b, "BEGIN EXCLUSIVE")
	if result, err := invalid.ExecContext(ctx, 5); err == nil || errors.Is(err, sqlite.ErrBusy) || result != nil {
		t.Fatal(result, err)
	}
	// Pinned deferred transaction can read connection counters without a data snapshot.
	mustExec(t, a, "BEGIN")
	var changes, total, last int64
	if err = a.QueryRowContext(ctx, "SELECT changes(),total_changes(),last_insert_rowid()").Scan(&changes, &total, &last); err != nil || changes != 2 || total != 2 || last != 2 {
		t.Fatal(changes, total, last, err)
	}
	if result, err := valid.ExecContext(ctx, 30, 1); !errors.Is(err, sqlite.ErrBusy) || result != nil {
		t.Fatal(result, err)
	}
	if err = a.QueryRowContext(ctx, "SELECT changes(),total_changes(),last_insert_rowid()").Scan(&changes, &total, &last); err != nil || changes != 0 || total != 2 || last != 2 {
		t.Fatal(changes, total, last, err)
	}
	mustExec(t, b, "ROLLBACK")
	mustExec(t, a, "ROLLBACK")
	result, err := valid.ExecContext(ctx, 30, 1)
	if err != nil {
		t.Fatal(err)
	}
	checkAffected(t, result, 1)
	if id, err := result.LastInsertId(); err != nil || id != 0 {
		t.Fatal(id, err)
	}
	mustExec(t, a, "BEGIN")
	mustExec(t, a, "CREATE TABLE abandoned(x)")
	if err = a.Close(); err != nil {
		t.Fatal(err)
	}
	reused, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer reused.Close()
	mustExec(t, b, "BEGIN EXCLUSIVE")
	if _, err = reused.ExecContext(ctx, "DELETE FROM abandoned"); !errors.Is(err, sqlite.ErrBusy) {
		t.Fatal("pool rollback should clear uncommitted DDL cache", err)
	}
	mustExec(t, b, "ROLLBACK")
	if err = reused.QueryRowContext(ctx, "SELECT changes(),total_changes(),last_insert_rowid()").Scan(&changes, &total, &last); err != nil || changes != 1 || total != 3 || last != 2 {
		t.Fatal(changes, total, last, err)
	}
	var sum int64
	if err = reused.QueryRowContext(ctx, "SELECT sum(v) FROM t").Scan(&sum); err != nil || sum != 50 {
		t.Fatal(sum, err)
	}
}
