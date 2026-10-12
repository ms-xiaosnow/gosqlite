package driver

import (
	"context"
	"database/sql"
	"reflect"
	"testing"
)

func TestDriverAutomaticUniquePreparedTransactionAndReopen(t *testing.T) {
	db, path := writableDatabase(t)
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	mustExec(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY UNIQUE,k NUMERIC CONSTRAINT uq_key UNIQUE,note TEXT,UNIQUE(k,note))")
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
	bound := tx.StmtContext(ctx, stmt)
	defer bound.Close()
	_, err = bound.ExecContext(ctx, sql.Named("id", 3), sql.Named("key", "02"), sql.Named("note", "duplicate"))
	driverUniqueConstraint(t, err)
	_, err = tx.ExecContext(ctx, "INSERT INTO t VALUES(4,4,'partial'),(5,1,'conflict')")
	driverUniqueConstraint(t, err)
	_, err = tx.ExecContext(ctx, "UPDATE t SET k=1,note='aborted' WHERE id=-7")
	driverUniqueConstraint(t, err)
	var count, key int64
	var note, storage string
	if err = tx.QueryRowContext(ctx, "SELECT count(*) FROM t").Scan(&count); err != nil || count != 2 {
		t.Fatal(count, err)
	}
	if err = tx.QueryRowContext(ctx, "SELECT k,typeof(k),note FROM t WHERE id=-7").Scan(&key, &storage, &note); err != nil || key != 2 || storage != "integer" || note != "kept" {
		t.Fatal(key, storage, note, err)
	}
	mustExec(t, tx, "SAVEPOINT rows")
	mustExec(t, tx, "UPDATE t SET k=3 WHERE id=-7")
	mustExec(t, tx, "ROLLBACK TO rows")
	mustExec(t, tx, "RELEASE rows")
	_, err = tx.ExecContext(ctx, "INSERT INTO t VALUES(6,2,'restored constraint')")
	driverUniqueConstraint(t, err)
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
	mustExec(t, reopened, "DELETE FROM t WHERE id=-8")
	mustExec(t, reopened, "INSERT INTO t VALUES(-7,2,'reused after reopen')")
	if err = reopened.QueryRowContext(ctx, "SELECT count(*) FROM t").Scan(&count); err != nil || count != 4 {
		t.Fatal(count, err)
	}
}

func TestDriverAutomaticUniqueSQLNullMetadata(t *testing.T) {
	db, _ := writableDatabase(t)
	mustExec(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY UNIQUE,k UNIQUE,UNIQUE(k),UNIQUE(id,k))")
	mustExec(t, db, "CREATE UNIQUE INDEX explicit_k ON t(k)")
	rows, err := db.QueryContext(context.Background(), "SELECT name,sql,sql IS NULL,typeof(sql),rootpage FROM sqlite_schema WHERE type='index' ORDER BY name")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var names []string
	roots := map[int64]bool{}
	for rows.Next() {
		var name, storage string
		var text sql.NullString
		var isNull, root int64
		if err := rows.Scan(&name, &text, &isNull, &storage, &root); err != nil {
			t.Fatal(err)
		}
		if root < 2 || roots[root] {
			t.Fatalf("nonphysical/shared root %d for %s", root, name)
		}
		roots[root] = true
		names = append(names, name)
		if name == "explicit_k" {
			if !text.Valid || text.String == "" || isNull != 0 || storage != "text" {
				t.Fatal(name, text, isNull, storage)
			}
		} else if text.Valid || isNull != 1 || storage != "null" {
			t.Fatal(name, text, isNull, storage)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if want := []string{"explicit_k", "sqlite_autoindex_t_1", "sqlite_autoindex_t_2", "sqlite_autoindex_t_3"}; !reflect.DeepEqual(names, want) {
		t.Fatalf("index names=%v want=%v", names, want)
	}
}

func TestDriverAutomaticUniquePreparedDDLRollback(t *testing.T) {
	db, _ := writableDatabase(t)
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	c, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	mustExec(t, c, "BEGIN")
	mustExec(t, c, "CREATE TABLE t(id INTEGER PRIMARY KEY,k UNIQUE)")
	stmt, err := c.PrepareContext(ctx, "INSERT INTO t VALUES(?,?)")
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	if _, err = stmt.ExecContext(ctx, 1, "base"); err != nil {
		t.Fatal(err)
	}
	_, err = stmt.ExecContext(ctx, 2, "base")
	driverUniqueConstraint(t, err)
	mustExec(t, c, "ROLLBACK")
	mustExec(t, c, "CREATE TABLE t(id INTEGER PRIMARY KEY,k)")
	for _, id := range []int64{1, 2} {
		if _, err = stmt.ExecContext(ctx, id, "base"); err != nil {
			t.Fatal("rolled-back automatic index leaked into prepared write", err)
		}
	}
	var count int64
	if err = c.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_schema WHERE type='index'").Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
	mustExec(t, c, "SAVEPOINT ddl")
	mustExec(t, c, "CREATE TABLE undone(a UNIQUE,b,UNIQUE(a,b))")
	mustExec(t, c, "ROLLBACK TO ddl")
	mustExec(t, c, "RELEASE ddl")
	if err = c.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_schema WHERE tbl_name='undone'").Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
}

func TestDriverAutomaticUniqueCountersAndPoolReuse(t *testing.T) {
	db, _ := writableDatabase(t)
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	c, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	mustExec(t, c, "CREATE TABLE t(id INTEGER PRIMARY KEY,k UNIQUE)")
	mustExec(t, c, "INSERT INTO t VALUES(7,'base')")
	_, err = c.ExecContext(ctx, "INSERT INTO t VALUES(99,'base')")
	driverUniqueConstraint(t, err)
	var changes, total, last int64
	check := func(conn *sql.Conn, wantChanges, wantTotal, wantLast int64) {
		t.Helper()
		if err := conn.QueryRowContext(ctx, "SELECT changes(),total_changes(),last_insert_rowid()").Scan(&changes, &total, &last); err != nil || changes != wantChanges || total != wantTotal || last != wantLast {
			t.Fatal(changes, total, last, err)
		}
	}
	check(c, 0, 1, 7)
	_, err = c.ExecContext(ctx, "INSERT INTO t VALUES(10,'partial'),(11,'base')")
	driverUniqueConstraint(t, err)
	check(c, 0, 1, 10)
	mustExec(t, c, "SAVEPOINT abandoned")
	mustExec(t, c, "CREATE TABLE abandoned(k UNIQUE)")
	mustExec(t, c, "INSERT INTO t VALUES(20,'abandoned')")
	_, err = c.ExecContext(ctx, "INSERT INTO t VALUES(21,'base')")
	driverUniqueConstraint(t, err)
	if err = c.Close(); err != nil {
		t.Fatal(err)
	}
	reused, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer reused.Close()
	var count int64
	if err = reused.QueryRowContext(ctx, "SELECT count(*) FROM t").Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
	if err = reused.QueryRowContext(ctx, "SELECT count(*) FROM sqlite_schema WHERE tbl_name='abandoned'").Scan(&count); err != nil || count != 0 {
		t.Fatal(count, err)
	}
	check(reused, 0, 2, 20)
	_, err = reused.ExecContext(ctx, "INSERT INTO t VALUES(30,'base')")
	driverUniqueConstraint(t, err)
	mustExec(t, reused, "INSERT INTO t VALUES(31,'abandoned')")
}
