package sqlite_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

const counterSelect = "SELECT changes(),total_changes(),last_insert_rowid(),typeof(changes()),typeof(total_changes()),typeof(last_insert_rowid())"

func checkCounters(t *testing.T, db *sqlite.DB, changes, total, last int64) {
	t.Helper()
	got := query(t, db, counterSelect)
	want := [][]any{{changes, total, last, "integer", "integer", "integer"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("counters got %#v want %#v", got, want)
	}
}

// Only intact SQL-created databases, normal transactions and logical errors.
func TestConnectionCountersStatementsAndUndo(t *testing.T) {
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprint(shared), func(t *testing.T) {
			db, _ := writable(t, sqlite.Options{WALShared: shared})
			checkCounters(t, db, 0, 0, 0)
			execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,v NOT NULL,c,tc,li)")
			execute(t, db, "CREATE INDEX ix ON t(v)")
			checkCounters(t, db, 0, 0, 0)
			res := execute(t, db, "INSERT INTO t VALUES(4,'a',changes(),total_changes(),last_insert_rowid()),(7,'b',changes(),total_changes(),last_insert_rowid())")
			if n, _ := res.RowsAffected(); n != 2 {
				t.Fatal(n)
			}
			if n, _ := res.LastInsertID(); n != 7 {
				t.Fatal(n)
			}
			if got := query(t, db, "SELECT id,c,tc,li FROM t ORDER BY id"); !reflect.DeepEqual(got, [][]any{{int64(4), int64(0), int64(0), int64(0)}, {int64(7), int64(0), int64(0), int64(4)}}) {
				t.Fatal(got)
			}
			checkCounters(t, db, 2, 2, 7)
			for _, q := range []string{"CREATE TABLE extra(x)", "CREATE TABLE IF NOT EXISTS extra(x)", "CREATE INDEX extra_x ON extra(x)", "BEGIN", "SAVEPOINT s", "ROLLBACK TO s", "RELEASE s", "COMMIT"} {
				execute(t, db, q)
				checkCounters(t, db, 2, 2, 7)
			}
			res = execute(t, db, "UPDATE t SET v=v,c=changes(),tc=total_changes(),li=last_insert_rowid()")
			if n, _ := res.RowsAffected(); n != 2 {
				t.Fatal(n)
			}
			if n, _ := res.LastInsertId(); n != 0 {
				t.Fatal(n)
			}
			checkCounters(t, db, 2, 4, 7)
			execute(t, db, "UPDATE t SET id=id+20 WHERE id=4")
			checkCounters(t, db, 1, 5, 7)
			execute(t, db, "DELETE FROM t WHERE id=-1")
			checkCounters(t, db, 0, 5, 7)
			execute(t, db, "BEGIN")
			execute(t, db, "INSERT INTO t VALUES(30,'undo',0,0,0)")
			checkCounters(t, db, 1, 6, 30)
			execute(t, db, "SAVEPOINT s")
			execute(t, db, "INSERT INTO t VALUES(40,'undo',0,0,0),(50,'undo',0,0,0)")
			execute(t, db, "ROLLBACK TO s")
			checkCounters(t, db, 2, 8, 50)
			execute(t, db, "ROLLBACK")
			checkCounters(t, db, 2, 8, 50)
			for _, q := range []string{"INSERT INTO t VALUES(60,'first',0,0,0),(70,NULL,0,0,0)", "BEGIN", "INSERT INTO t VALUES(60,'first',0,0,0),(70,NULL,0,0,0)"} {
				if q == "BEGIN" {
					execute(t, db, q)
					continue
				}
				r, err := db.Exec(context.Background(), q)
				if !errors.Is(err, sqlite.ErrConstraint) {
					t.Fatal(q, err)
				}
				if n, _ := r.RowsAffected(); n != 0 {
					t.Fatal(n)
				}
				checkCounters(t, db, 0, 8, 60)
			}
			execute(t, db, "ROLLBACK")
			execute(t, db, "UPDATE t SET v=v")
			checkCounters(t, db, 2, 10, 60)
			for _, q := range []string{"UPDATE t SET missing=1", "DELETE FROM absent", "INSERT INTO t(id) VALUES(1,2)", "UPDATE t SET v=changes(1)", "INSERT OR IGNORE INTO t VALUES(99,'x',0,0,0)"} {
				if _, err := db.Exec(context.Background(), q); err == nil {
					t.Fatal(q)
				}
				checkCounters(t, db, 2, 10, 60)
			}
			if _, err := db.Exec(context.Background(), "UPDATE t SET v=abs(-9223372036854775808)"); err == nil {
				t.Fatal("missing execution error")
			}
			checkCounters(t, db, 0, 10, 60)
			execute(t, db, "DELETE FROM t")
			checkCounters(t, db, 2, 12, 60)
			execute(t, db, "UPDATE t SET v=1")
			checkCounters(t, db, 0, 12, 60)
		})
	}
}

func TestConnectionCountersAdmissionRowsAndReopen(t *testing.T) {
	db, path := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,v)")
	execute(t, db, "INSERT INTO t VALUES(9,1),(10,2)")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := db.Exec(ctx, "INSERT INTO t VALUES(11,3)"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	checkCounters(t, db, 2, 2, 10)
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	rows, err := tx.Query(context.Background(), counterSelect)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(context.Background(), "INSERT INTO t VALUES(11,3)"); !errors.Is(err, sqlite.ErrBusy) {
		t.Fatal(err)
	}
	if !rows.Next() || !reflect.DeepEqual(rows.Values(), []any{int64(2), int64(2), int64(10), "integer", "integer", "integer"}) {
		t.Fatal(rows.Values(), rows.Err())
	}
	if err = rows.Close(); err != nil {
		t.Fatal(err)
	}
	txExecute(t, tx, "INSERT INTO t VALUES(-7,3)")
	checkCounters(t, db, 1, 3, -7)
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	checkCounters(t, db, 1, 3, -7)
	peer, err := sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	checkCounters(t, peer, 0, 0, 0)
	execute(t, peer, "UPDATE t SET v=v")
	checkCounters(t, peer, 2, 2, 0)
	checkCounters(t, db, 1, 3, -7)
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	checkCounters(t, reopened, 0, 0, 0)
}

func TestConnectionCountersStreamingDynamicAndBusyCommit(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,v)")
	execute(t, db, "INSERT INTO t VALUES(1,1),(2,2)")
	rows, err := db.Query(context.Background(), "SELECT id,changes(),total_changes(),last_insert_rowid() FROM t")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal(rows.Err())
	}
	// The existing rollback-mode cursor lease prevents the automatic commit.
	// No change total is published; the successful insertion rowid survives.
	if _, err = db.Exec(context.Background(), "INSERT INTO t VALUES(50,50)"); !errors.Is(err, sqlite.ErrBusy) {
		t.Fatal(err)
	}
	if !rows.Next() || !reflect.DeepEqual(rows.Values(), []any{int64(2), int64(0), int64(2), int64(50)}) {
		t.Fatal(rows.Values(), rows.Err())
	}
	if err = rows.Close(); err != nil {
		t.Fatal(err)
	}
	checkCounters(t, db, 0, 2, 50)
	if got := query(t, db, "SELECT count(*) FROM t"); !reflect.DeepEqual(got, [][]any{{int64(2)}}) {
		t.Fatal(got)
	}
}

func TestConnectionCountersConcurrentNative(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY)")
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for k := 0; k < 4; k++ {
		wg.Add(1)
		go func(k int) {
			defer wg.Done()
			for j := 1; j <= 10; j++ {
				if _, err := tx.Exec(context.Background(), "INSERT INTO t VALUES(?)", k*10+j); err != nil {
					t.Error(err)
				}
			}
		}(k)
	}
	wg.Wait()
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := query(t, db, "SELECT changes(),total_changes()")
	if !reflect.DeepEqual(got, [][]any{{int64(1), int64(40)}}) {
		t.Fatal(got)
	}
}

func TestConnectionCountersExtensionMaintenanceExcluded(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY,body TEXT)")
	execute(t, db, "INSERT INTO docs VALUES(20,'words'),(30,'more words')")
	execute(t, db, "CREATE INDEX grams ON docs(body) USING grams13")
	checkCounters(t, db, 2, 2, 30)
	execute(t, db, "INSERT INTO docs VALUES(40,'new words')")
	checkCounters(t, db, 1, 3, 40)
	execute(t, db, "UPDATE docs SET body='changed words'")
	checkCounters(t, db, 3, 6, 40)
	execute(t, db, "REINDEX grams")
	checkCounters(t, db, 3, 6, 40)
	execute(t, db, "DELETE FROM docs")
	checkCounters(t, db, 3, 9, 40)
}
