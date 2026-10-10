package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

func cachedPeer(t *testing.T, path string) *DB {
	t.Helper()
	p, err := Open(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := p.Close(); err != nil {
			t.Error(err)
		}
	})
	return p
}
func cachedCode(t *testing.T, err error, code Code) {
	t.Helper()
	var detail *Error
	if !errors.As(err, &detail) || detail.Code != code {
		t.Fatalf("code %v: %v", code, err)
	}
}
func cachedCounters(t *testing.T, db *DB, changes, total, id int64) {
	t.Helper()
	if got := db.counters; got != (sqlast.ConnectionState{Changes: changes, TotalChanges: total, LastInsertRowID: id}) {
		t.Fatal(got)
	}
}

func TestCachedPreparationOutcomes(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		t.Run(fmt.Sprint(explicit), func(t *testing.T) {
			db, path := reviewTransactionDB(t)
			reviewTransactionExec(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,v,true)")
			reviewTransactionExec(t, db, "INSERT INTO t VALUES(1,10,1),(2,20,0)")
			peer := cachedPeer(t, path)
			reviewTransactionExec(t, peer, "BEGIN EXCLUSIVE")
			if explicit {
				reviewTransactionExec(t, db, "BEGIN")
			}
			invalid := []string{
				"DELETE FROM missing", "DELETE FROM t WHERE missing=1", "UPDATE t SET missing=1", "UPDATE t SET v=missing", "UPDATE t SET v=changes(1)",
				"INSERT INTO t(missing) VALUES(1)", "INSERT INTO t(id) VALUES(1,2)", "INSERT INTO t VALUES(3,changes(1),0)", "INSERT INTO t VALUES(3,missing,0)",
				"UPDATE t SET id=1,rowid=2", "DELETE FROM t WHERE CASE WHEN 0 THEN missing ELSE 1 END", "UPDATE t SET v=true.missing",
			}
			for _, q := range invalid {
				r, e := db.Exec(context.Background(), q)
				cachedCode(t, e, CodeError)
				if r != (Result{}) {
					t.Fatal(q, r)
				}
				cachedCounters(t, db, 2, 2, 2)
			}
			// Binding and pre-cancellation still precede any lock or cached preparation.
			for _, args := range [][]any{nil, {sql.Named("other", 3)}, {make(chan int)}} {
				_, e := db.Exec(context.Background(), "UPDATE t SET v=:v", args...)
				cachedCode(t, e, CodeError)
				cachedCounters(t, db, 2, 2, 2)
			}
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			_, e := db.Exec(ctx, "DELETE FROM t")
			cachedCode(t, e, CodeInterrupt)
			cachedCounters(t, db, 2, 2, 2)
			for _, q := range []string{"UPDATE t SET v=abs(-9223372036854775808)", "INSERT INTO t VALUES(3,4,1)", "DELETE FROM t WHERE true", "UPDATE t SET v=:v WHERE id=?2"} {
				var args []any
				if strings.Contains(q, ":v") {
					args = []any{3, 1}
				}
				_, e := db.Exec(context.Background(), q, args...)
				cachedCode(t, e, CodeBusy)
				cachedCounters(t, db, 0, 2, 2)
				if db.tx != nil && (db.tx.reader != nil || db.tx.storage != nil) {
					t.Fatal("cache acquired a reader or storage")
				}
			}
			reviewTransactionExec(t, peer, "ROLLBACK")
			if explicit {
				reviewTransactionExec(t, db, "ROLLBACK")
			}
			if got := reviewTransactionScalar(t, db, "SELECT count(*) FROM t"); got != int64(2) {
				t.Fatal(got)
			}
			r, e := db.Exec(context.Background(), "UPDATE t SET v=v")
			if e != nil {
				t.Fatal(e)
			}
			n, _ := r.RowsAffected()
			id, _ := r.LastInsertId()
			if n != 2 || id != 0 {
				t.Fatal(r)
			}
			cachedCounters(t, db, 2, 4, 2)
		})
	}
}

func TestCachedPreparationFreshReadWarmAndQuota(t *testing.T) {
	db, path := reviewTransactionDB(t)
	reviewTransactionExec(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,v)")
	reviewTransactionExec(t, db, "INSERT INTO t VALUES(1,1),(2,2)")
	fresh := cachedPeer(t, path)
	peer := cachedPeer(t, path)
	if fresh.preparation != nil {
		t.Fatal("Open loaded schema")
	}
	reviewTransactionExec(t, peer, "BEGIN EXCLUSIVE")
	for _, q := range []string{"DELETE FROM t", "DELETE FROM missing", "UPDATE t SET missing=1"} {
		_, e := fresh.Exec(context.Background(), q)
		cachedCode(t, e, CodeBusy)
		cachedCounters(t, fresh, 0, 0, 0)
	}
	reviewTransactionExec(t, peer, "ROLLBACK")
	if got := reviewTransactionScalar(t, fresh, "SELECT count(*) FROM t"); got != int64(2) {
		t.Fatal(got)
	}
	if fresh.preparation == nil {
		t.Fatal("read did not warm ordinary schema")
	}
	reviewTransactionExec(t, peer, "BEGIN EXCLUSIVE")
	_, e := fresh.Exec(context.Background(), "DELETE FROM missing")
	cachedCode(t, e, CodeError)
	_, e = fresh.Exec(context.Background(), "DELETE FROM t")
	cachedCode(t, e, CodeBusy)
	reviewTransactionExec(t, peer, "ROLLBACK")
	limited, e := Open(filepath.Join(t.TempDir(), "small.db"), Options{MaxSortBytes: 512})
	if e != nil {
		t.Fatal(e)
	}
	defer limited.Close()
	reviewTransactionExec(t, limited, "CREATE TABLE t(id INTEGER PRIMARY KEY)")
	reviewTransactionExec(t, limited, "INSERT INTO t VALUES(1),(2)")
	if limited.preparation != nil {
		t.Fatal("cache exceeded configured memory allowance")
	}
	p := cachedPeer(t, limited.path)
	reviewTransactionExec(t, p, "BEGIN EXCLUSIVE")
	_, e = limited.Exec(context.Background(), "DELETE FROM t")
	cachedCode(t, e, CodeBusy)
	cachedCounters(t, limited, 2, 2, 2)
	reviewTransactionExec(t, p, "ROLLBACK")
}

func TestCachedPreparationSchemaLifecycle(t *testing.T) {
	db, path := reviewTransactionDB(t)
	reviewTransactionExec(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,v)")
	reviewTransactionExec(t, db, "INSERT INTO t VALUES(1,1),(2,2)")
	peer := cachedPeer(t, path)
	// Own successful DDL is available immediately, without an intervening DML.
	reviewTransactionExec(t, db, "CREATE TABLE own(x)")
	reviewTransactionExec(t, peer, "BEGIN EXCLUSIVE")
	_, e := db.Exec(context.Background(), "DELETE FROM own WHERE missing=1")
	cachedCode(t, e, CodeError)
	cachedCounters(t, db, 2, 2, 2)
	_, e = db.Exec(context.Background(), "DELETE FROM own")
	cachedCode(t, e, CodeBusy)
	cachedCounters(t, db, 0, 2, 2)
	reviewTransactionExec(t, peer, "ROLLBACK")
	// An unseen peer schema remains stale until a normal snapshot is obtained.
	reviewTransactionExec(t, peer, "CREATE TABLE added(x)")
	reviewTransactionExec(t, peer, "BEGIN EXCLUSIVE")
	_, e = db.Exec(context.Background(), "DELETE FROM added")
	cachedCode(t, e, CodeError)
	reviewTransactionExec(t, peer, "ROLLBACK")
	reviewTransactionExec(t, db, "INSERT INTO added VALUES(1),(2)")
	cachedCounters(t, db, 2, 4, 2)
	reviewTransactionExec(t, peer, "BEGIN EXCLUSIVE")
	_, e = db.Exec(context.Background(), "DELETE FROM added")
	cachedCode(t, e, CodeBusy)
	cachedCounters(t, db, 0, 4, 2)
	reviewTransactionExec(t, peer, "ROLLBACK")
	for _, savepoint := range []bool{false, true} {
		reviewTransactionExec(t, db, "UPDATE t SET v=v")
		reviewTransactionExec(t, db, "BEGIN")
		if savepoint {
			reviewTransactionExec(t, db, "SAVEPOINT s")
		}
		reviewTransactionExec(t, db, "CREATE TABLE discarded(x)")
		reviewTransactionExec(t, db, "INSERT INTO discarded VALUES(10),(20)")
		before := db.counters
		if savepoint {
			reviewTransactionExec(t, db, "ROLLBACK TO s")
			reviewTransactionExec(t, db, "RELEASE s")
			reviewTransactionExec(t, db, "COMMIT")
		} else {
			reviewTransactionExec(t, db, "ROLLBACK")
		}
		if db.preparation != nil {
			t.Fatal("DDL rollback retained schema")
		}
		reviewTransactionExec(t, peer, "BEGIN EXCLUSIVE")
		for _, q := range []string{"DELETE FROM discarded", "DELETE FROM t"} {
			_, e = db.Exec(context.Background(), q)
			cachedCode(t, e, CodeBusy)
			if db.counters != before {
				t.Fatal(db.counters, before)
			}
		}
		reviewTransactionExec(t, peer, "ROLLBACK")
		_, e = db.Exec(context.Background(), "DELETE FROM discarded")
		cachedCode(t, e, CodeError)
	}
}

func TestCachedPreparationMetadataIsolation(t *testing.T) {
	db, _ := reviewTransactionDB(t)
	reviewTransactionExec(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,v DEFAULT 42)")
	old := db.preparation
	if old == nil || old.bytes > db.preparationLimit() {
		t.Fatal(old)
	}
	for _, tab := range old.tables {
		if tab.root != 0 || len(tab.indexes) != 0 || len(tab.extensions) != 0 {
			t.Fatal("cache contains storage metadata")
		}
		for _, col := range tab.columns {
			if col.Default != nil || col.Type != "" {
				t.Fatal("cache retained expression or irrelevant type")
			}
		}
	}
	before := fmt.Sprint(old.tables["t"].columns)
	reviewTransactionExec(t, db, "CREATE TABLE added(x)")
	if len(old.tables) != 1 || fmt.Sprint(old.tables["t"].columns) != before {
		t.Fatal("mutated published metadata")
	}
	if reflect.ValueOf(db.preparation).Pointer() == reflect.ValueOf(old).Pointer() {
		t.Fatal("schema not replaced")
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if db.preparation != nil {
		t.Fatal("Close retained schema")
	}
}

func TestCachedPreparationBusyClassification(t *testing.T) {
	// Pure error-tree classification; no native failure is injected or executed.
	for _, e := range []error{ErrBusy, fmt.Errorf("lock: %w", ErrBusy), errors.Join(ErrBusy, vfs.ErrBusy)} {
		if !onlyBusy(e) {
			t.Fatal(e)
		}
	}
	for _, e := range []error{nil, context.Canceled, errors.Join(ErrBusy, context.Canceled), errors.Join(ErrBusy, ErrPoisoned), errors.Join(ErrBusy, ErrChanged), errors.Join(ErrBusy, fmt.Errorf("other"))} {
		if onlyBusy(e) {
			t.Fatal(e)
		}
	}
}

func TestCachedPreparationEmptyQuota(t *testing.T) {
	for _, limit := range []int64{1, 64, 255} {
		db, err := Open(filepath.Join(t.TempDir(), "empty.db"), Options{MaxSortBytes: limit})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = db.Schema(context.Background()); err != nil {
			t.Fatal(err)
		}
		if db.preparation != nil {
			t.Fatalf("limit %d retained %d", limit, db.preparation.bytes)
		}
		if err = db.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
