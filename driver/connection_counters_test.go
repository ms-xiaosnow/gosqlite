package driver

import (
	"context"
	"testing"
)

func TestDriverConnectionCountersPreparedPinnedAndPoolReset(t *testing.T) {
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
	stmt, err := a.PrepareContext(ctx, "INSERT INTO t VALUES(?,last_insert_rowid())")
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	for _, id := range []int64{10, 20, 30} {
		r, err := stmt.ExecContext(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		checkAffected(t, r, 1)
		if got, err := r.LastInsertId(); err != nil || got != id {
			t.Fatal(got, err)
		}
	}
	var changes, total, last int64
	if err = a.QueryRowContext(ctx, "SELECT changes(),total_changes(),last_insert_rowid()").Scan(&changes, &total, &last); err != nil || changes != 1 || total != 3 || last != 30 {
		t.Fatal(changes, total, last, err)
	}
	if err = b.QueryRowContext(ctx, "SELECT changes(),total_changes(),last_insert_rowid()").Scan(&changes, &total, &last); err != nil || changes != 0 || total != 0 || last != 0 {
		t.Fatal(changes, total, last, err)
	}
	mustExec(t, a, "BEGIN")
	mustExec(t, a, "INSERT INTO t VALUES(99,0)")
	mustExec(t, a, "ROLLBACK")
	if err = a.QueryRowContext(ctx, "SELECT changes(),total_changes(),last_insert_rowid()").Scan(&changes, &total, &last); err != nil || changes != 1 || total != 4 || last != 99 {
		t.Fatal(changes, total, last, err)
	}
	r := mustExec(t, a, "UPDATE t SET v=v")
	checkAffected(t, r, 3)
	if got, err := r.LastInsertId(); err != nil || got != 0 {
		t.Fatal("statement result insert-ID contract", got, err)
	}
	if err = stmt.Close(); err != nil {
		t.Fatal(err)
	}
	mustExec(t, a, "SAVEPOINT abandoned")
	mustExec(t, a, "INSERT INTO t VALUES(100,0)")
	if err = a.Close(); err != nil {
		t.Fatal(err)
	}
	// b remains pinned, forcing reuse of a's physical connection.
	reused, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer reused.Close()
	if err = reused.QueryRowContext(ctx, "SELECT changes(),total_changes(),last_insert_rowid()").Scan(&changes, &total, &last); err != nil || changes != 1 || total != 8 || last != 100 {
		t.Fatal(changes, total, last, err)
	}
	var n int64
	if err = reused.QueryRowContext(ctx, "SELECT count(*) FROM t").Scan(&n); err != nil || n != 3 {
		t.Fatal(n, err)
	}
}
