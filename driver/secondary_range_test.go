package driver

import (
	"context"
	"database/sql"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// Intact ordinary SQL, prepared-statement reuse, rollback and cancellation only.
func TestDriverSecondaryRangePreparedAndRollback(t *testing.T) {
	ctx := context.Background()
	db, _ := writableDatabase(t)
	db.SetMaxOpenConns(1)
	mustExec(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,k BLOB,v BLOB)")
	mustExec(t, db, "CREATE INDEX by_k ON t(k)")
	mustExec(t, db, "INSERT INTO t VALUES(1,1,X'00FF'),(2,2,X''),(3,3,NULL),(4,'4',X'42'),(5,X'34',X'00')")
	stmt, err := db.PrepareContext(ctx, "SELECT id,v FROM t WHERE k>=:low AND k<:high ORDER BY k,id DESC")
	if err != nil {
		t.Fatal(err)
	}
	defer stmt.Close()
	for _, tc := range []struct {
		lo, hi any
		want   []int64
	}{{1, 4, []int64{1, 2, 3}}, {"1", "5", []int64{4}}, {[]byte{0}, []byte{255}, []int64{5}}, {nil, 5, nil}} {
		for range 2 {
			r, err := stmt.QueryContext(ctx, sql.Named("high", tc.hi), sql.Named("low", tc.lo))
			if err != nil {
				t.Fatal(err)
			}
			var ids []int64
			for r.Next() {
				var id int64
				var b []byte
				if err = r.Scan(&id, &b); err != nil {
					t.Fatal(err)
				}
				ids = append(ids, id)
				if id == 2 && (b == nil || len(b) != 0) || id == 3 && b != nil {
					t.Fatal(id, b)
				}
			}
			if r.Err() != nil || !reflect.DeepEqual(ids, tc.want) {
				t.Fatal(ids, tc.want, r.Err())
			}
			r.Close()
		}
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	mustExec(t, tx, "UPDATE t SET k=8 WHERE id=2")
	var id int64
	if err = tx.QueryRowContext(ctx, "SELECT id FROM t WHERE k>7 AND k<9").Scan(&id); err != nil || id != 2 {
		t.Fatal(id, err)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRowContext(ctx, "SELECT id FROM t WHERE k>7 AND k<9").Scan(&id); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal(err)
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	r, err := db.QueryContext(cancelCtx, "SELECT id FROM t WHERE k>0")
	if err != nil {
		t.Fatal(err)
	}
	if !r.Next() {
		t.Fatal(r.Err())
	}
	cancel()
	for r.Next() {
	}
	if !errors.Is(r.Err(), context.Canceled) {
		t.Fatal(r.Err())
	}
	r.Close()
	for _, q := range []string{"SELECT missing FROM t WHERE k>:x LIMIT 0", "SELECT id FROM t WHERE k>:x LIMIT 0"} {
		r, err := db.QueryContext(ctx, q)
		if err == nil {
			r.Close()
			t.Fatal("prebinding was skipped", q)
		}
	}
	var detail string
	var a, b, c int
	if err = db.QueryRowContext(ctx, "EXPLAIN QUERY PLAN SELECT id FROM t WHERE k>1").Scan(&a, &b, &c, &detail); err != nil || !strings.Contains(detail, "btree_range") {
		t.Fatal(detail, err)
	}
}
