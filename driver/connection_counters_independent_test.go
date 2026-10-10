package driver

import (
	"context"
	"errors"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestReviewConnectionCountersPreparedReuse(t *testing.T) {
	db, _ := writableDatabase(t)
	db.SetMaxOpenConns(1)
	ctx := context.Background()
	c, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	mustExec(t, c, "CREATE TABLE t(id INTEGER PRIMARY KEY,v NOT NULL)")
	get, err := c.PrepareContext(ctx, "SELECT changes(),total_changes(),last_insert_rowid(),typeof(last_insert_rowid())")
	if err != nil {
		t.Fatal(err)
	}
	defer get.Close()
	insert, err := c.PrepareContext(ctx, "INSERT INTO t VALUES(?,?),(last_insert_rowid()+1,?)")
	if err != nil {
		t.Fatal(err)
	}
	defer insert.Close()
	check := func(wc, wt, wl int64) {
		t.Helper()
		var a, b, d int64
		var typ string
		if err := get.QueryRowContext(ctx).Scan(&a, &b, &d, &typ); err != nil || a != wc || b != wt || d != wl || typ != "integer" {
			t.Fatal(a, b, d, typ, err)
		}
	}
	check(0, 0, 0)
	for i := int64(0); i < 3; i++ {
		r, err := insert.ExecContext(ctx, i*10, "a", "b")
		if err != nil {
			t.Fatal(err)
		}
		checkAffected(t, r, 2)
		check(2, (i+1)*2, i*10+1)
	}
	if _, err := insert.ExecContext(ctx, 30, "first", nil); !errors.Is(err, sqlite.ErrConstraint) {
		t.Fatal(err)
	}
	check(0, 6, 30)
	mustExec(t, c, "BEGIN")
	mustExec(t, c, "INSERT INTO t VALUES(100,'rollback')")
	check(1, 7, 100)
	mustExec(t, c, "ROLLBACK")
	check(1, 7, 100)
	cancelctx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := insert.ExecContext(cancelctx, 200, "a", "b"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	check(1, 7, 100)
}
