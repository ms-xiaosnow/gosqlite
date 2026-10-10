package driver

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestDriverRowIDRangePreparedStreamingAndRollback(t *testing.T) {
	ctx := context.Background()
	db, _ := writableDatabase(t)
	db.SetMaxOpenConns(1)
	mustExec(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,v BLOB)")
	mustExec(t, db, "INSERT INTO t VALUES(1,X'00FF'),(2,X''),(3,NULL),(4,X'42')")
	stmt, err := db.PrepareContext(ctx, "SELECT id,v FROM t WHERE id>=:low AND id<:high ORDER BY id DESC LIMIT 2")
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	for range 3 {
		rows, err := stmt.QueryContext(ctx, sql.Named("high", 4), sql.Named("low", 1))
		if err != nil {
			t.Fatal(err)
		}
		var ids []int64
		for rows.Next() {
			var id int64
			var blob []byte
			if err = rows.Scan(&id, &blob); err != nil {
				t.Fatal(err)
			}
			ids = append(ids, id)
			if id == 3 && blob != nil || id == 2 && (blob == nil || len(blob) != 0) {
				t.Fatal(id, blob)
			}
		}
		if rows.Err() != nil || !reflect.DeepEqual(ids, []int64{3, 2}) {
			t.Fatal(ids, rows.Err())
		}
		rows.Close()
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, tx, "UPDATE t SET id=8 WHERE id=2")
	var id int64
	if err = tx.QueryRowContext(ctx, "SELECT id FROM t WHERE id>1 ORDER BY id DESC LIMIT 1").Scan(&id); err != nil || id != 8 {
		t.Fatal(id, err)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, "SELECT id FROM t WHERE id>1 ORDER BY id DESC LIMIT 1").Scan(&id); err != nil || id != 4 {
		t.Fatal(id, err)
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	rows, err := db.QueryContext(cancelCtx, "SELECT id FROM t WHERE id>0 ORDER BY id DESC")
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() {
		t.Fatal(rows.Err())
	}
	cancel()
	for rows.Next() {
	}
	if !errors.Is(rows.Err(), context.Canceled) {
		t.Fatal(rows.Err())
	}
	rows.Close()
	if err = db.QueryRowContext(ctx, "SELECT id FROM t WHERE id>0 ORDER BY id DESC LIMIT 1").Scan(&id); err != nil || id != 4 {
		t.Fatal(id, err)
	}
}

func TestDriverRowIDRangeAvoidsOnlyProvenSort(t *testing.T) {
	connector, err := NewConnector("file:"+filepath.Join("..", "testdata", "readonly", "page4096.db")+"?mode=ro", sqlite.Options{MaxSortBytes: 1})
	if err != nil {
		t.Fatal(err)
	}
	db := sql.OpenDB(connector)
	defer db.Close()
	for _, direction := range []string{"ASC", "DESC"} {
		rows, err := db.Query("SELECT id FROM samples WHERE id>=10 AND id<=12 ORDER BY id " + direction)
		if err != nil {
			t.Fatal(err)
		}
		var ids []int64
		for rows.Next() {
			var id int64
			if err = rows.Scan(&id); err != nil {
				t.Fatal(err)
			}
			ids = append(ids, id)
		}
		want := []int64{10, 11, 12}
		if direction == "DESC" {
			want = []int64{12, 11, 10}
		}
		if rows.Err() != nil || !reflect.DeepEqual(ids, want) {
			t.Fatal(ids, rows.Err())
		}
		rows.Close()
	}
	if _, err = db.Query("SELECT id FROM samples WHERE id>=10 AND id<=12 ORDER BY id+0"); !errors.Is(err, sqlite.ErrMemoryLimit) {
		t.Fatal("unproven sort escaped quota", err)
	}
}
