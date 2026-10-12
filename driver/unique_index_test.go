package driver

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func driverUniqueConstraint(t *testing.T, err error) {
	t.Helper()
	var detail *sqlite.Error
	if !errors.Is(err, sqlite.ErrConstraint) || !errors.As(err, &detail) || detail.Code != sqlite.CodeConstraint {
		t.Fatalf("want ErrConstraint and primary code 19, got %v", err)
	}
}

func TestDriverUniqueIndexPreparedTransactionAndReopen(t *testing.T) {
	db, path := writableDatabase(t)
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	mustExec(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k NUMERIC,note TEXT)")
	mustExec(t, db, "CREATE UNIQUE INDEX unique_k ON t(k COLLATE BINARY ASC)")
	stmt, err := db.PrepareContext(ctx, "INSERT INTO t VALUES(:id,:key,:note)")
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	r, err := stmt.ExecContext(ctx, sql.Named("note", "base"), sql.Named("key", "01"), sql.Named("id", 0))
	if err != nil {
		t.Fatal(err)
	}
	checkAffected(t, r, 1)
	if id, err := r.LastInsertId(); err != nil || id != 0 {
		t.Fatal(id, err)
	}
	_, err = stmt.ExecContext(ctx, sql.Named("id", -7), sql.Named("key", float64(1)), sql.Named("note", "duplicate"))
	driverUniqueConstraint(t, err)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	mustExec(t, tx, "INSERT INTO t VALUES(-7,2,'kept')")
	bound, err := tx.PrepareContext(ctx, "INSERT INTO t VALUES(?,?,?)")
	if err != nil {
		t.Fatal(err)
	}
	defer bound.Close()
	_, err = bound.ExecContext(ctx, 3, "02", "duplicate")
	driverUniqueConstraint(t, err)
	_, err = tx.ExecContext(ctx, "INSERT INTO t VALUES(4,4,'partial'),(5,1,'conflict')")
	driverUniqueConstraint(t, err)
	_, err = tx.ExecContext(ctx, "UPDATE t SET k=1,note='aborted' WHERE id=-7")
	driverUniqueConstraint(t, err)
	var n int64
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM t").Scan(&n); err != nil || n != 2 {
		t.Fatal(n, err)
	}
	var note, storage string
	var k int64
	if err = tx.QueryRowContext(ctx, "SELECT k,typeof(k),note FROM t WHERE id=-7").Scan(&k, &storage, &note); err != nil || k != 2 || storage != "integer" || note != "kept" {
		t.Fatal(k, storage, note, err)
	}
	checkAffected(t, mustExec(t, tx, "UPDATE t SET id=-8,k=k WHERE id=-7"), 1)
	if err = bound.Close(); err != nil {
		t.Fatal(err)
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
	reopened, err := sql.Open("gosqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	_, err = reopened.ExecContext(ctx, "INSERT INTO t VALUES(9,2,'still duplicate')")
	driverUniqueConstraint(t, err)
	mustExec(t, reopened, "INSERT INTO t VALUES(10,NULL,'null'),(11,NULL,'null')")
	if err = reopened.QueryRowContext(ctx, "SELECT count(*) FROM t").Scan(&n); err != nil || n != 4 {
		t.Fatal(n, err)
	}
}

func TestDriverUniqueIndexPreparedSchemaRollback(t *testing.T) {
	db, _ := writableDatabase(t)
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	c, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	mustExec(t, c, "CREATE TABLE t(id INTEGER PRIMARY KEY,k)")
	mustExec(t, c, "INSERT INTO t VALUES(1,'base')")
	stmt, err := c.PrepareContext(ctx, "INSERT INTO t VALUES(?,?)")
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	mustExec(t, c, "BEGIN")
	mustExec(t, c, "CREATE UNIQUE INDEX unique_k ON t(k)")
	_, err = stmt.ExecContext(ctx, 2, "base")
	driverUniqueConstraint(t, err)
	mustExec(t, c, "ROLLBACK")
	if _, err = stmt.ExecContext(ctx, 2, "base"); err != nil {
		t.Fatal("rolled-back unique catalog leaked into a prepared write", err)
	}
	_, err = c.ExecContext(ctx, "CREATE UNIQUE INDEX unique_k ON t(k)")
	driverUniqueConstraint(t, err)
	mustExec(t, c, "DELETE FROM t WHERE id=2")
	mustExec(t, c, "CREATE UNIQUE INDEX unique_k ON t(k)")
	_, err = stmt.ExecContext(ctx, 2, "base")
	driverUniqueConstraint(t, err)
	if _, err = stmt.ExecContext(ctx, 3, "after build"); err != nil {
		t.Fatal(err)
	}
	var n int64
	if err = c.QueryRowContext(ctx, "SELECT count(*) FROM t").Scan(&n); err != nil || n != 2 {
		t.Fatal(n, err)
	}
}

func TestDriverUniqueIndexCountersAndPoolReuse(t *testing.T) {
	db, _ := writableDatabase(t)
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	c, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	mustExec(t, c, "CREATE TABLE t(id INTEGER PRIMARY KEY,k)")
	mustExec(t, c, "CREATE UNIQUE INDEX unique_k ON t(k)")
	mustExec(t, c, "INSERT INTO t VALUES(7,'base')")
	_, err = c.ExecContext(ctx, "INSERT INTO t VALUES(99,'base')")
	driverUniqueConstraint(t, err)
	var changes, total, last int64
	if err = c.QueryRowContext(ctx, "SELECT changes(),total_changes(),last_insert_rowid()").Scan(&changes, &total, &last); err != nil || changes != 0 || total != 1 || last != 7 {
		t.Fatal(changes, total, last, err)
	}
	_, err = c.ExecContext(ctx, "INSERT INTO t VALUES(10,'partial'),(11,'base')")
	driverUniqueConstraint(t, err)
	if err = c.QueryRowContext(ctx, "SELECT changes(),total_changes(),last_insert_rowid()").Scan(&changes, &total, &last); err != nil || changes != 0 || total != 1 || last != 10 {
		t.Fatal(changes, total, last, err)
	}
	mustExec(t, c, "SAVEPOINT abandoned")
	mustExec(t, c, "INSERT INTO t VALUES(20,'abandoned')")
	_, err = c.ExecContext(ctx, "INSERT INTO t VALUES(21,'base')")
	driverUniqueConstraint(t, err)
	if err = c.Close(); err != nil {
		t.Fatal(err)
	}
	// With one physical connection the next borrower must get clean transaction
	// state and the committed UNIQUE index, while retaining connection counters.
	reused, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer reused.Close()
	var n int64
	if err = reused.QueryRowContext(ctx, "SELECT count(*) FROM t").Scan(&n); err != nil || n != 1 {
		t.Fatal(n, err)
	}
	if err = reused.QueryRowContext(ctx, "SELECT changes(),total_changes(),last_insert_rowid()").Scan(&changes, &total, &last); err != nil || changes != 0 || total != 2 || last != 20 {
		t.Fatal(changes, total, last, err)
	}
	_, err = reused.ExecContext(ctx, "INSERT INTO t VALUES(30,'base')")
	driverUniqueConstraint(t, err)
	mustExec(t, reused, "INSERT INTO t VALUES(31,'abandoned')")
}
