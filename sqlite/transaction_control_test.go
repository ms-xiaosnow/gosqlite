package sqlite_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestTransactionDeferredSnapshotTiming(t *testing.T) {
	ctx := context.Background()
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprint(shared), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "timing.db")
			opts := sqlite.Options{WALShared: shared}
			a, err := sqlite.Open(path, opts)
			if err != nil {
				t.Fatal(err)
			}
			defer a.Close()
			execute(t, a, "CREATE TABLE t(id INTEGER PRIMARY KEY,v TEXT)")
			execute(t, a, "INSERT INTO t VALUES(1,'old')")
			b, err := sqlite.Open(path, opts)
			if err != nil {
				t.Fatal(err)
			}
			defer b.Close()
			for _, begin := range []string{"BEGIN DEFERRED", "SAVEPOINT root"} {
				execute(t, a, begin)
				if a.Autocommit() {
					t.Fatal("logical transaction absent")
				}
				if got := query(t, a, "SELECT 42"); !reflect.DeepEqual(got, [][]any{{int64(42)}}) {
					t.Fatal(got)
				}
				execute(t, b, "UPDATE t SET v='between' WHERE id=1")
				if got := query(t, a, "SELECT v FROM t"); !reflect.DeepEqual(got, [][]any{{"between"}}) {
					t.Fatalf("BEGIN pinned early: %v", got)
				}
				if shared {
					execute(t, b, "UPDATE t SET v='after-read' WHERE id=1")
					if got := query(t, a, "SELECT v FROM t"); !reflect.DeepEqual(got, [][]any{{"between"}}) {
						t.Fatalf("snapshot not pinned: %v", got)
					}
				} else {
					if _, err := b.Exec(ctx, "UPDATE t SET v='after-read' WHERE id=1"); !errors.Is(err, sqlite.ErrBusy) {
						t.Fatalf("reader did not block commit: %v", err)
					}
				}
				execute(t, a, "ROLLBACK")
				if !a.Autocommit() {
					t.Fatal("rollback retained transaction")
				}
			}
			tx, err := a.Begin(ctx, sqlite.TxOptions{ReadOnly: true})
			if err != nil {
				t.Fatal(err)
			}
			execute(t, b, "UPDATE t SET v='native-lazy' WHERE id=1")
			rows, err := tx.Query(ctx, "SELECT v FROM t")
			if err != nil {
				t.Fatal(err)
			}
			if !rows.Next() || rows.Values()[0] != "native-lazy" {
				t.Fatal(rows.Values(), rows.Err())
			}
			rows.Close()
			if err = tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestTransactionSavepointNamesDDLAndStatementFailure(t *testing.T) {
	ctx := context.Background()
	db, path := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,v TEXT)")
	execute(t, db, "INSERT INTO t VALUES(1,'base')")
	execute(t, db, "SAVEPOINT root")
	execute(t, db, "UPDATE t SET v='outer' WHERE id=1")
	execute(t, db, `SAVEPOINT "Repeated"`)
	execute(t, db, "CREATE TABLE transient(id INTEGER PRIMARY KEY)")
	execute(t, db, "CREATE INDEX transient_ix ON transient(id)")
	execute(t, db, "INSERT INTO transient VALUES(3)")
	execute(t, db, "SAVEPOINT repeated")
	execute(t, db, "UPDATE t SET v='inner' WHERE id=1")
	execute(t, db, "ROLLBACK TRANSACTION TO SAVEPOINT REPEATED")
	if got := query(t, db, "SELECT v FROM t"); !reflect.DeepEqual(got, [][]any{{"outer"}}) {
		t.Fatal(got)
	}
	// ROLLBACK TO keeps its target; RELEASE removes only the newest same-name mark.
	execute(t, db, "UPDATE t SET v='retry' WHERE id=1")
	execute(t, db, "ROLLBACK TO repeated")
	execute(t, db, "RELEASE repeated")
	execute(t, db, "ROLLBACK TO repeated")
	if _, err := db.Query(ctx, "SELECT * FROM transient"); err == nil {
		t.Fatal("transactional DDL survived rollback")
	}
	execute(t, db, "INSERT INTO t VALUES(2,'kept')")
	if _, err := db.Exec(ctx, "INSERT INTO t VALUES(3,'aborted'),(2,'duplicate')"); !errors.Is(err, sqlite.ErrConstraint) {
		t.Fatal(err)
	}
	if got := query(t, db, "SELECT id,v FROM t ORDER BY id"); !reflect.DeepEqual(got, [][]any{{int64(1), "outer"}, {int64(2), "kept"}}) {
		t.Fatal(got)
	}
	if _, err := db.Exec(ctx, "RELEASE missing"); err == nil || db.Autocommit() {
		t.Fatal("missing release changed state", err)
	}
	execute(t, db, "RELEASE SAVEPOINT root")
	if !db.Autocommit() {
		t.Fatal("outer release did not commit")
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if got := query(t, db, "SELECT id FROM t ORDER BY id"); !reflect.DeepEqual(got, [][]any{{int64(1)}, {int64(2)}}) {
		t.Fatal(got)
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	txExecute(t, tx, "SAVEPOINT 'native mark'")
	txExecute(t, tx, "INSERT INTO t VALUES(4,'native')")
	txExecute(t, tx, "RELEASE SAVEPOINT 'native mark'")
	if db.Autocommit() {
		t.Fatal("inner release committed explicit BEGIN")
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if got := query(t, db, "SELECT id FROM t ORDER BY id"); len(got) != 2 {
		t.Fatal(got)
	}
}

func TestTransactionImmediateExclusiveAndFailedOuterRelease(t *testing.T) {
	ctx := context.Background()
	for _, shared := range []bool{false, true} {
		t.Run(fmt.Sprint(shared), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "locks.db")
			opts := sqlite.Options{WALShared: shared}
			a, err := sqlite.Open(path, opts)
			if err != nil {
				t.Fatal(err)
			}
			defer a.Close()
			execute(t, a, "CREATE TABLE t(id INTEGER PRIMARY KEY)")
			execute(t, a, "INSERT INTO t VALUES(1)")
			b, err := sqlite.Open(path, opts)
			if err != nil {
				t.Fatal(err)
			}
			defer b.Close()
			for _, mode := range []string{"IMMEDIATE", "EXCLUSIVE"} {
				execute(t, a, "BEGIN "+mode)
				if _, err = b.Exec(ctx, "BEGIN IMMEDIATE"); !errors.Is(err, sqlite.ErrBusy) || !b.Autocommit() {
					t.Fatalf("writer reservation: %v", err)
				}
				rows, err := b.Query(ctx, "SELECT id FROM t")
				if mode == "EXCLUSIVE" && !shared {
					if !errors.Is(err, sqlite.ErrBusy) {
						if rows != nil {
							rows.Close()
						}
						t.Fatal("exclusive did not block reader", err)
					}
				} else {
					if err != nil {
						t.Fatal(err)
					}
					rows.Close()
				}
				execute(t, a, "ROLLBACK")
			}
			if !shared {
				read, err := b.Begin(ctx, sqlite.TxOptions{ReadOnly: true})
				if err != nil {
					t.Fatal(err)
				}
				rows, err := read.Query(ctx, "SELECT id FROM t")
				if err != nil {
					t.Fatal(err)
				}
				rows.Close()
				execute(t, a, "SAVEPOINT root")
				execute(t, a, "INSERT INTO t VALUES(2)")
				if _, err = a.Exec(ctx, "RELEASE root"); !errors.Is(err, sqlite.ErrBusy) || a.Autocommit() {
					t.Fatalf("failed release state: %v", err)
				}
				execute(t, a, "ROLLBACK TO root")
				execute(t, a, "INSERT INTO t VALUES(3)")
				if err = read.Commit(ctx); err != nil {
					t.Fatal(err)
				}
				execute(t, a, "RELEASE root")
				if got := query(t, a, "SELECT id FROM t ORDER BY id"); !reflect.DeepEqual(got, [][]any{{int64(1)}, {int64(3)}}) {
					t.Fatal(got)
				}
			}
		})
	}
}

func TestTransactionSavepointLimitsAndRowsRestrictions(t *testing.T) {
	ctx := context.Background()
	db, _ := writable(t, sqlite.Options{PageSize: 512, MaxTransactionUndoBytes: 6500})
	execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,v TEXT)")
	execute(t, db, "INSERT INTO t VALUES(1,'base')")
	execute(t, db, "SAVEPOINT root")
	execute(t, db, "UPDATE t SET v='before' WHERE id=1")
	execute(t, db, "SAVEPOINT middle")
	execute(t, db, "UPDATE t SET v='after' WHERE id=1")
	execute(t, db, "SAVEPOINT last")
	if _, err := db.Exec(ctx, "UPDATE t SET v='rejected' WHERE id=1"); !errors.Is(err, sqlite.ErrMemoryLimit) {
		t.Fatal(err)
	}
	execute(t, db, "RELEASE last")
	execute(t, db, "UPDATE t SET v='accepted' WHERE id=1")
	execute(t, db, "ROLLBACK TO middle")
	if got := query(t, db, "SELECT v FROM t"); !reflect.DeepEqual(got, [][]any{{"before"}}) {
		t.Fatal(got)
	}
	rows, err := db.Query(ctx, "SELECT v FROM t")
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"SAVEPOINT active", "RELEASE middle", "ROLLBACK TO root", "COMMIT"} {
		if _, err = db.Exec(ctx, q); !errors.Is(err, sqlite.ErrBusy) {
			t.Fatal(q, err)
		}
	}
	execute(t, db, "ROLLBACK")
	if rows.Next() || !errors.Is(rows.Err(), sqlite.ErrTxDone) {
		t.Fatal(rows.Err())
	}
	if !db.Autocommit() {
		t.Fatal("rollback left active transaction")
	}
	execute(t, db, "BEGIN")
	execute(t, db, "COMMIT")
	execute(t, db, "SAVEPOINT no_access")
	execute(t, db, "RELEASE no_access")
}

func TestTransactionLazySavepointQuotaFailureState(t *testing.T) {
	ctx := context.Background()
	db, path := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY)")
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := sqlite.Open(path, sqlite.Options{MaxTransactionUndoBytes: 1024})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(ctx, "SAVEPOINT named"); !errors.Is(err, sqlite.ErrMemoryLimit) || !db.Autocommit() {
		t.Fatalf("failed outer SAVEPOINT left transaction: %v", err)
	}
	execute(t, db, "SAVEPOINT ''")
	if _, err = db.Exec(ctx, "SAVEPOINT nested"); !errors.Is(err, sqlite.ErrMemoryLimit) || db.Autocommit() {
		t.Fatalf("failed nested SAVEPOINT ended transaction: %v", err)
	}
	execute(t, db, "RELEASE ''")
	if !db.Autocommit() {
		t.Fatal("release empty name did not commit")
	}
}
