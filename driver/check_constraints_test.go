package driver

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func driverCheckConstraint(t *testing.T, err error) {
	t.Helper()
	var detail *sqlite.Error
	if !errors.Is(err, sqlite.ErrConstraint) || !errors.As(err, &detail) || detail.Code != sqlite.CodeConstraint || !strings.Contains(err.Error(), "CHECK constraint failed:") {
		t.Fatal("want CHECK constraint error", err)
	}
}

func TestDriverCheckPreparedTransactionsAndReopen(t *testing.T) {
	db, path := writableDatabase(t)
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	mustExec(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,a NUMERIC CHECK(a>0),b NUMERIC,tag TEXT UNIQUE,CHECK(a<=b))")
	mustExec(t, db, "CREATE INDEX by_a ON t(a)")
	stmt, err := db.PrepareContext(ctx, "INSERT INTO t VALUES(:id,:a,:b,:tag)")
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	_, err = stmt.ExecContext(ctx, sql.Named("id", 1), sql.Named("a", "01"), sql.Named("b", 3), sql.Named("tag", "base"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = stmt.ExecContext(ctx, sql.Named("id", 2), sql.Named("a", 0), sql.Named("b", 3), sql.Named("tag", "bad"))
	driverCheckConstraint(t, err)
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	mustExec(t, tx, "INSERT INTO t VALUES(2,2,3,'kept')")
	_, err = tx.ExecContext(ctx, "INSERT INTO t VALUES(3,3,3,'partial'),(4,4,3,'bad')")
	driverCheckConstraint(t, err)
	var changes, total, last, count int64
	if err = tx.QueryRowContext(ctx, "SELECT changes(),total_changes(),last_insert_rowid()").Scan(&changes, &total, &last); err != nil || changes != 0 || total != 2 || last != 3 {
		t.Fatal(changes, total, last, err)
	}
	mustExec(t, tx, "SAVEPOINT s")
	_, err = tx.ExecContext(ctx, "UPDATE t SET a=CASE WHEN id=1 THEN 2 ELSE 0 END")
	driverCheckConstraint(t, err)
	mustExec(t, tx, "ROLLBACK TO s")
	mustExec(t, tx, "RELEASE s")
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM t").Scan(&count); err != nil || count != 2 {
		t.Fatal(count, err)
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = stmt.Close(); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = sql.Open("gosqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.ExecContext(ctx, "UPDATE t SET a=4 WHERE id=1")
	driverCheckConstraint(t, err)
	mustExec(t, db, "UPDATE t SET a=a,b=b+1")
	mustExec(t, db, "INSERT INTO t VALUES(3,NULL,NULL,'null')")
	if err = db.QueryRowContext(ctx, "SELECT count(*) FROM t").Scan(&count); err != nil || count != 3 {
		t.Fatal(count, err)
	}
}
