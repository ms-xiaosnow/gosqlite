package sqlite

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"
)

// Ordinary intact empty databases; no injected storage behavior.
func TestReviewPreparationEmptyQuota(t *testing.T) {
	for _, limit := range []int64{1, 64, 255, 256, 512} {
		db, err := Open(filepath.Join(t.TempDir(), "empty.db"), Options{MaxSortBytes: limit})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Schema(context.Background()); err != nil {
			t.Fatal(err)
		}
		if db.preparation != nil && db.preparation.bytes > limit {
			t.Errorf("limit=%d cached=%d", limit, db.preparation.bytes)
		}
		if err = db.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

// Tests error precedence and state from ordinary SQL, without fabricating a pager failure.
func TestReviewPreparationErrorPrecedenceAndIsolation(t *testing.T) {
	ctx := context.Background()
	for _, warm := range []bool{false, true} {
		t.Run(fmt.Sprint(warm), func(t *testing.T) {
			setup, path := reviewTransactionDB(t)
			reviewTransactionExec(t, setup, "CREATE TABLE t(id INTEGER PRIMARY KEY,v,true,false)")
			reviewTransactionExec(t, setup, "INSERT INTO t VALUES(1,11,1,0),(2,12,0,1)")
			db, err := Open(path, Options{})
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if warm {
				reviewTransactionExec(t, db, "UPDATE t SET v=v")
			}
			before := db.counters
			lock, err := Open(path, Options{})
			if err != nil {
				t.Fatal(err)
			}
			defer lock.Close()
			reviewTransactionExec(t, lock, "BEGIN EXCLUSIVE")
			for _, tc := range []struct {
				q    string
				args []any
			}{
				{"UPDATE t SET v=?", nil},
				{"UPDATE t SET v=:v", []any{sql.Named("wrong", 1)}},
				{"UPDATE t SET v=?", []any{make(chan int)}},
				{"INSERT INTO t SELECT * FROM t", nil},
				{"UPDATE t SET", nil},
				{"DELETE FROM t RETURNING id", nil},
			} {
				_, err = db.Exec(ctx, tc.q, tc.args...)
				if err == nil || errors.Is(err, ErrBusy) {
					t.Fatalf("%q expected rejection before admission: %v", tc.q, err)
				}
				if db.counters != before {
					t.Fatalf("%q counters changed: %v -> %v", tc.q, before, db.counters)
				}
			}
			canceled, cancel := context.WithCancel(ctx)
			cancel()
			_, err = db.Exec(canceled, "DELETE FROM t")
			if !errors.Is(err, context.Canceled) || db.counters != before {
				t.Fatalf("pre-canceled: %v %v", err, db.counters)
			}
			bindctx, bindcancel := context.WithCancel(ctx)
			_, err = db.Exec(bindctx, "UPDATE t SET v=?", reviewCancelValue{cancel: bindcancel})
			if !errors.Is(err, context.Canceled) || db.counters != before {
				t.Fatalf("bind canceled: %v %v", err, db.counters)
			}
			reviewTransactionExec(t, lock, "ROLLBACK")
			if got := reviewTransactionScalar(t, db, "SELECT sum(v) FROM t"); got != int64(23) {
				t.Fatal(got)
			}
		})
	}
}

type reviewCancelValue struct{ cancel context.CancelFunc }

func (v reviewCancelValue) Value() (driver.Value, error) { v.cancel(); return int64(9), nil }

func TestReviewPreparationReadOnlyAndRows(t *testing.T) {
	ctx := context.Background()
	db, path := reviewTransactionDB(t)
	reviewTransactionExec(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,v)")
	reviewTransactionExec(t, db, "INSERT INTO t VALUES(1,11),(2,12)")
	ro, err := Open(path, Options{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	if _, err = ro.Schema(ctx); err != nil {
		t.Fatal(err)
	}
	lock, err := Open(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	reviewTransactionExec(t, lock, "BEGIN EXCLUSIVE")
	for _, q := range []string{"DELETE FROM absent", "DELETE FROM t"} {
		if _, err = ro.Exec(ctx, q); !errors.Is(err, ErrReadOnly) {
			t.Fatal(q, err)
		}
	}
	if ro.counters != (sqlast.ConnectionState{}) {
		t.Fatal(ro.counters)
	}
	reviewTransactionExec(t, lock, "ROLLBACK")
	tx, err := db.Begin(ctx, TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"DELETE FROM absent", "DELETE FROM t"} {
		if _, err = tx.Exec(ctx, q); !errors.Is(err, ErrReadOnly) {
			t.Fatal(q, err)
		}
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	tx, err = db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := tx.Query(ctx, "SELECT * FROM t")
	if err != nil {
		t.Fatal(err)
	}
	before := db.counters
	for _, q := range []string{"DELETE FROM absent", "DELETE FROM t"} {
		if _, err = tx.Exec(ctx, q); !errors.Is(err, ErrBusy) || db.counters != before {
			t.Fatal(q, err, db.counters)
		}
	}
	if err = rows.Close(); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	if db.preparation != nil {
		t.Fatal("closed DB retained preparation")
	}
	if _, err = db.Exec(ctx, "DELETE FROM t"); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
}

func TestReviewPreparationDeclaredNamesOwnStorage(t *testing.T) {
	ctx := context.Background()
	db, _ := reviewTransactionDB(t)
	query := "CREATE TABLE small(id INTEGER PRIMARY KEY,value DEFAULT 7) /*" + strings.Repeat("padding", 10000) + "*/"
	stmt, err := sqlast.Parse(query)
	if err != nil {
		t.Fatal(err)
	}
	tab, err := writableTable(stmt.(*sqlast.CreateTable), 2)
	if err != nil {
		t.Fatal(err)
	}
	c := &writeCatalog{tables: map[string]*writeTable{"small": tab}, extensions: &extensionCatalog{}}
	db.rememberPreparationCatalogLocked(ctx, 9, c)
	cache := db.preparation
	if cache == nil || cache.bytes > db.preparationLimit() {
		t.Fatal(cache)
	}
	copy := cache.tables["small"]
	if unsafe.StringData(copy.name) == unsafe.StringData(tab.name) {
		t.Fatal("table name aliases parser allocation")
	}
	for i, col := range tab.columns {
		if unsafe.StringData(copy.columns[i].Name) == unsafe.StringData(col.Name) {
			t.Fatal("column name aliases parser allocation")
		}
	}
	tab.columns[1].Name = "replaced"
	if copy.columns[1].Name != "value" {
		t.Fatal("published cache aliases mutable columns")
	}
	if copy.root != 0 || copy.columns[1].Default != nil || copy.columns[1].Type != "" || copy.indexes != nil || copy.extensions != nil {
		t.Fatal("cache retained operational schema state")
	}
}

func TestReviewPreparationDDLBusyRollback(t *testing.T) {
	ctx := context.Background()
	db, path := reviewTransactionDB(t)
	reviewTransactionExec(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,v)")
	reviewTransactionExec(t, db, "INSERT INTO t VALUES(1,11),(2,12)")
	peer, err := Open(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	reviewTransactionExec(t, peer, "BEGIN")
	rows, err := peer.Query(ctx, "SELECT * FROM t")
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() {
		t.Fatal(rows.Err())
	}
	before := db.counters
	if _, err = db.Exec(ctx, "CREATE TABLE discarded(x)"); !errors.Is(err, ErrBusy) {
		t.Fatal(err)
	}
	if db.preparation != nil || db.counters != before {
		t.Fatal("automatic DDL rollback retained metadata/state", db.preparation, db.counters)
	}
	if err = rows.Close(); err != nil {
		t.Fatal(err)
	}
	reviewTransactionExec(t, peer, "ROLLBACK")
	reviewTransactionExec(t, peer, "BEGIN EXCLUSIVE")
	if _, err = db.Exec(ctx, "DELETE FROM discarded"); !errors.Is(err, ErrBusy) || db.counters != before {
		t.Fatal(err, db.counters)
	}
	reviewTransactionExec(t, peer, "ROLLBACK")
	if _, err = db.Exec(ctx, "DELETE FROM discarded"); err == nil || errors.Is(err, ErrBusy) {
		t.Fatal(err)
	}
	reviewTransactionExec(t, db, "BEGIN")
	reviewTransactionExec(t, db, "CREATE TABLE kept(x)")
	reviewTransactionExec(t, db, "SAVEPOINT newer")
	reviewTransactionExec(t, db, "INSERT INTO kept VALUES(1),(2)")
	reviewTransactionExec(t, db, "ROLLBACK TO newer")
	if db.preparation == nil {
		t.Fatal("data-only rollback discarded current schema")
	}
	reviewTransactionExec(t, db, "RELEASE newer")
	reviewTransactionExec(t, db, "COMMIT")
	reviewTransactionExec(t, peer, "BEGIN EXCLUSIVE")
	if _, err = db.Exec(ctx, "DELETE FROM kept WHERE absent=1"); err == nil || errors.Is(err, ErrBusy) {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, "DELETE FROM kept"); !errors.Is(err, ErrBusy) {
		t.Fatal(err)
	}
	reviewTransactionExec(t, peer, "ROLLBACK")
}

func TestReviewPreparationExcludedExtensionAndSession(t *testing.T) {
	ctx := context.Background()
	db, path := reviewTransactionDB(t)
	reviewTransactionExec(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY,body TEXT)")
	reviewTransactionExec(t, db, "INSERT INTO docs VALUES(1,'alpha'),(2,'beta')")
	reviewTransactionExec(t, db, "CREATE INDEX grams ON docs(body) USING grams13")
	if db.preparation != nil {
		t.Fatal("extension schema retained preparation cache")
	}
	if _, err := db.Schema(ctx); err != nil {
		t.Fatal(err)
	}
	if db.preparation != nil {
		t.Fatal("Schema warmed excluded extension catalog")
	}
	lock, err := Open(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	reviewTransactionExec(t, lock, "BEGIN EXCLUSIVE")
	before := db.counters
	for _, q := range []string{"DELETE FROM missing", "UPDATE docs SET body='gamma'"} {
		if _, err = db.Exec(ctx, q); !errors.Is(err, ErrBusy) || db.counters != before {
			t.Fatal(q, err, db.counters)
		}
	}
	reviewTransactionExec(t, lock, "ROLLBACK")
	reviewTransactionExec(t, db, "UPDATE docs SET body='gamma'")
	if got := reviewTransactionScalar(t, db, "SELECT count(*) FROM docs WHERE body='gamma'"); got != int64(2) {
		t.Fatal(got)
	}
	shared, err := Open(filepath.Join(t.TempDir(), "shared.db"), Options{WALShared: true})
	if err != nil {
		t.Fatal(err)
	}
	defer shared.Close()
	reviewTransactionExec(t, shared, "CREATE TABLE t(x)")
	reviewTransactionExec(t, shared, "INSERT INTO t VALUES(1),(2)")
	if _, err = shared.Schema(ctx); err != nil {
		t.Fatal(err)
	}
	if shared.preparation != nil {
		t.Fatal("session mode retained excluded cache")
	}
}
