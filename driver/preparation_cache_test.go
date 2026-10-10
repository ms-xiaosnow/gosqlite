package driver

import (
	"context"
	"errors"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestDriverCachedPreparationBusyReuse(t *testing.T) {
	db, _ := writableDatabase(t)
	db.SetMaxOpenConns(2)
	ctx := context.Background()
	a, e := db.Conn(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	b, e := db.Conn(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer b.Close()
	mustExec(t, a, "CREATE TABLE t(id INTEGER PRIMARY KEY,v)")
	mustExec(t, a, "INSERT INTO t VALUES(1,1),(2,2)")
	valid, e := a.PrepareContext(ctx, "UPDATE t SET v=? WHERE id=?")
	if e != nil {
		t.Fatal(e)
	}
	defer valid.Close()
	invalid, e := a.PrepareContext(ctx, "UPDATE t SET missing=?")
	if e != nil {
		t.Fatal(e)
	}
	defer invalid.Close()
	check := func(changes, total int64) {
		t.Helper()
		var c, tc, id int64
		e := a.QueryRowContext(ctx, "SELECT changes(),total_changes(),last_insert_rowid()").Scan(&c, &tc, &id)
		if e != nil || c != changes || tc != total || id != 2 {
			t.Fatal(c, tc, id, e)
		}
	}
	mustExec(t, b, "BEGIN EXCLUSIVE")
	// Driver preparation remains reusable SQL syntax. Native execution performs
	// cached name validation; parameters are bound normally before admission.
	if _, e = invalid.ExecContext(ctx, 3); e == nil || errors.Is(e, sqlite.ErrBusy) {
		t.Fatal(e)
	}
	mustExec(t, b, "ROLLBACK")
	check(2, 2)
	mustExec(t, b, "BEGIN EXCLUSIVE")
	if _, e = valid.ExecContext(ctx, 3, 1); !errors.Is(e, sqlite.ErrBusy) {
		t.Fatal(e)
	}
	mustExec(t, b, "ROLLBACK")
	check(0, 2)
	r, e := valid.ExecContext(ctx, 3, 1)
	if e != nil {
		t.Fatal(e)
	}
	checkAffected(t, r, 1)
	if id, e := r.LastInsertId(); e != nil || id != 0 {
		t.Fatal(id, e)
	}
	check(1, 3)
	var v int64
	if e = a.QueryRowContext(ctx, "SELECT v FROM t WHERE id=1").Scan(&v); e != nil || v != 3 {
		t.Fatal(v, e)
	}
}
