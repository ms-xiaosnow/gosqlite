package sqlite

import (
	"context"
	"errors"
	"testing"
)

// Cancellation is an ordinary context cancellation, triggered at a deterministic
// logical row boundary. No I/O, pager, lock or cleanup failure is injected.
type counterCancelContext struct {
	context.Context
	cancel context.CancelFunc
	db     *DB
}

func (c *counterCancelContext) Err() error {
	if c.db.counters.LastInsertRowID == 3 {
		c.cancel()
	}
	return c.Context.Err()
}
func TestConnectionCountersCancellationAfterInsertedRow(t *testing.T) {
	for _, explicit := range []bool{false, true} {
		db, _ := reviewTransactionDB(t)
		reviewTransactionExec(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY)")
		reviewTransactionExec(t, db, "INSERT INTO t VALUES(1),(2)")
		if explicit {
			reviewTransactionExec(t, db, "BEGIN")
		}
		ctx, cancel := context.WithCancel(context.Background())
		_, err := db.Exec(&counterCancelContext{Context: ctx, cancel: cancel, db: db}, "INSERT INTO t VALUES(3),(4)")
		cancel()
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
		if db.counters.Changes != 0 || db.counters.TotalChanges != 2 || db.counters.LastInsertRowID != 3 {
			t.Fatal(db.counters)
		}
		if got := reviewTransactionScalar(t, db, "SELECT count(*) FROM t"); got != int64(2) {
			t.Fatal(got)
		}
		if explicit {
			if db.Autocommit() {
				t.Fatal("native statement rollback lost transaction")
			}
			reviewTransactionExec(t, db, "ROLLBACK")
		}
	}
}

func TestConnectionCountersLazyTransactionReads(t *testing.T) {
	db, path := reviewTransactionDB(t)
	reviewTransactionExec(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY)")
	reviewTransactionExec(t, db, "INSERT INTO t VALUES(1),(2)")
	peer, err := Open(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	reviewTransactionExec(t, peer, "BEGIN EXCLUSIVE")
	reviewTransactionExec(t, db, "BEGIN")
	for _, q := range []string{"SELECT changes()", "SELECT total_changes()", "SELECT last_insert_rowid()"} {
		if got := reviewTransactionScalar(t, db, q); got != int64(2) {
			t.Fatal(q, got)
		}
		if db.tx.reader != nil || db.tx.storage != nil {
			t.Fatal("connection counters forced a snapshot")
		}
	}
	// The cached ordinary schema distinguishes preparation from execution BUSY.
	if _, err := db.Exec(context.Background(), "DELETE FROM missing"); err == nil || errors.Is(err, ErrBusy) {
		t.Fatal(err)
	}
	if got := reviewTransactionScalar(t, db, "SELECT changes()"); got != int64(2) {
		t.Fatal(got)
	}
	if _, err := db.Exec(context.Background(), "DELETE FROM t"); !errors.Is(err, ErrBusy) {
		t.Fatal(err)
	}
	if got := reviewTransactionScalar(t, db, "SELECT changes()"); got != int64(0) {
		t.Fatal(got)
	}
	reviewTransactionExec(t, peer, "ROLLBACK")
	reviewTransactionExec(t, db, "ROLLBACK")
}
