package driver

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestDriverSavepointsPoolResetAndOuterRelease(t *testing.T) {
	ctx := context.Background()
	db, _ := writableDatabase(t)
	db.SetMaxOpenConns(1)
	mustExec(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,v TEXT)")
	for _, finish := range []string{"abandon", "RELEASE root", "ROLLBACK TO root"} {
		conn, err := db.Conn(ctx)
		if err != nil {
			t.Fatal(err)
		}
		mustExec(t, conn, "SAVEPOINT root")
		mustExec(t, conn, "INSERT INTO t VALUES(1,?)", finish)
		mustExec(t, conn, "SAVEPOINT inner")
		mustExec(t, conn, "RELEASE inner")
		if finish != "abandon" {
			mustExec(t, conn, finish)
		}
		if err = conn.Close(); err != nil {
			t.Fatal(err)
		}
		var value string
		err = db.QueryRow("SELECT v FROM t WHERE id=1").Scan(&value)
		if finish == "RELEASE root" {
			if err != nil || value != finish {
				t.Fatal(value, err)
			}
			mustExec(t, db, "DELETE FROM t")
		} else if !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("pooled savepoint leaked: %s %v", value, err)
		}
	}
	// A native sql.Tx is authoritative even when controls are sent through SQL.
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, tx, "SAVEPOINT native")
	mustExec(t, tx, "INSERT INTO t VALUES(2,'native')")
	mustExec(t, tx, "RELEASE native")
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	var id int
	if err = db.QueryRow("SELECT id FROM t").Scan(&id); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal(id, err)
	}
}

func TestDriverFailedOuterReleaseRemainsActive(t *testing.T) {
	ctx := context.Background()
	db, path := writableDatabase(t)
	db.SetMaxOpenConns(1)
	mustExec(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY)")
	reader, err := sqlite.Open(path, sqlite.Options{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	read, err := reader.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer read.Rollback()
	rows, err := read.Query(ctx, "SELECT id FROM t")
	if err != nil {
		t.Fatal(err)
	}
	rows.Close()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, conn, "SAVEPOINT root")
	mustExec(t, conn, "INSERT INTO t VALUES(1)")
	if _, err = conn.ExecContext(ctx, "RELEASE root"); !errors.Is(err, sqlite.ErrBusy) {
		t.Fatal(err)
	}
	if err = conn.Close(); err != nil {
		t.Fatal(err)
	}
	if err = read.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var id int
	if err = db.QueryRow("SELECT id FROM t").Scan(&id); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("failed release transaction leaked to pool: %d %v", id, err)
	}
	mustExec(t, db, "INSERT INTO t VALUES(2)")
}

func TestDriverDeferredBeginSeesFirstAccessState(t *testing.T) {
	ctx := context.Background()
	db, path := writableDatabase(t)
	mustExec(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY)")
	other, err := sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	var one int
	if err = tx.QueryRow("SELECT 1").Scan(&one); err != nil || one != 1 {
		t.Fatal(one, err)
	}
	if _, err = other.Exec(ctx, "INSERT INTO t VALUES(42)"); err != nil {
		t.Fatal("BEGIN/constant SELECT pinned snapshot", err)
	}
	var id int
	if err = tx.QueryRow("SELECT id FROM t").Scan(&id); err != nil || id != 42 {
		t.Fatal(id, err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
}
