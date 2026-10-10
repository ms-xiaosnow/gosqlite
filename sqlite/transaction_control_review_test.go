package sqlite

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"

	"github.com/ms-xiaosnow/gosqlite/internal/pager"
)

// This uses a logical transaction only, with no filesystem or pager snapshot.
// Inspecting string pointers proves a short name does not retain its large
// source allocation. It does not fabricate or mutate string storage.
func TestReviewSavepointNamesOwnBoundedStorage(t *testing.T) {
	ctx := context.Background()
	db := &DB{opts: Options{MaxTransactionUndoBytes: 4096}, info: FileInfo{PageSize: 512}}
	tx := &Tx{db: db, ctx: ctx}
	db.tx = tx
	source := strings.Repeat("padding", 1<<16) + "short"
	name := source[len(source)-5:]
	if err := tx.savepointLocked(ctx, name); err != nil {
		t.Fatal(err)
	}
	got := tx.savepoints[0].name
	if got != name || unsafe.StringData(got) == unsafe.StringData(name) {
		t.Fatal("savepoint name retains the source string allocation")
	}
	if tx.markBytes != pager.SavepointMetadataBytes(name) || tx.reader != nil || tx.storage != nil {
		t.Fatal("logical mark acquired storage or was mischarged")
	}
	before := tx.markBytes
	if err := tx.savepointLocked(ctx, strings.Repeat("x", 4096)); !errors.Is(err, pager.ErrLimit) {
		t.Fatalf("oversized name accepted: %v", err)
	}
	if tx.markBytes != before || len(tx.savepoints) != 1 {
		t.Fatal("failed name quota changed existing marks")
	}
	for i := 0; i < 2; i++ {
		if err := tx.savepointLocked(ctx, ""); err != nil {
			t.Fatal(err)
		}
	}
	if err := tx.savepointLocked(ctx, ""); !errors.Is(err, pager.ErrLimit) {
		t.Fatalf("aggregate empty-name depth was unbounded: %v", err)
	}
	tx.discardSavepointsLocked(1)
	if tx.markBytes != before || len(tx.savepoints) != 1 || cap(tx.savepoints) != 1 {
		t.Fatal("released names retain backing stack/accounting")
	}
	tx.discardSavepointsLocked(0)
	if tx.markBytes != 0 || tx.savepoints != nil {
		t.Fatal("all released names remain retained")
	}
}

func reviewTransactionDB(t *testing.T) (*DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "transaction-review.db")
	db, err := Open(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return db, path
}

func reviewTransactionExec(t *testing.T, db *DB, query string) Result {
	t.Helper()
	result, err := db.Exec(context.Background(), query)
	if err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return result
}

func reviewTransactionScalar(t *testing.T, db *DB, query string) any {
	t.Helper()
	rows, err := db.Query(context.Background(), query)
	if err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatalf("%s returned no first row: %v", query, rows.Err())
	}
	value := rows.Values()[0]
	if rows.Next() || rows.Err() != nil {
		t.Fatalf("%s returned an extra row or failed: %v", query, rows.Err())
	}
	return value
}

// Both connections operate ordinary valid files. Contention ends by normal
// ROLLBACK; no interruption, invalid bytes, or failing physical I/O is induced.
func TestReviewLazySavepointFirstAccessBusyRetryAndConstantRows(t *testing.T) {
	ctx := context.Background()
	db, path := reviewTransactionDB(t)
	reviewTransactionExec(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,v TEXT)")
	reviewTransactionExec(t, db, "INSERT INTO t VALUES(1,'before')")
	peer, err := Open(path, Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	reviewTransactionExec(t, peer, "BEGIN EXCLUSIVE")
	reviewTransactionExec(t, db, "SAVEPOINT root")
	reviewTransactionExec(t, db, "SAVEPOINT leaf")
	tx := db.tx
	for _, query := range []string{"SELECT 7", "SELECT 7 WHERE 1=1 ORDER BY 1", "SELECT 7 LIMIT 1 OFFSET 0"} {
		if got := reviewTransactionScalar(t, db, query); got != int64(7) {
			t.Fatal(got)
		}
		if tx.reader != nil || tx.storage != nil {
			t.Fatalf("constant query acquired a snapshot: %s", query)
		}
	}
	rows, err := db.Query(ctx, "SELECT 8")
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"SAVEPOINT forbidden", "RELEASE root", "ROLLBACK TO leaf", "COMMIT"} {
		if _, err := db.Exec(ctx, query); !errors.Is(err, ErrBusy) {
			t.Fatalf("active constant Rows did not block %s: %v", query, err)
		}
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Query(ctx, "SELECT v FROM t"); !errors.Is(err, ErrBusy) {
		t.Fatalf("first access did not observe peer EXCLUSIVE: %v", err)
	}
	if db.tx != tx || db.Autocommit() || tx.reader != nil || tx.storage != nil || len(tx.savepoints) != 2 {
		t.Fatal("first-access BUSY damaged logical transaction or marks")
	}
	for _, mark := range tx.savepoints {
		if mark.id != 0 {
			t.Fatal("failed first access installed partial pager marks")
		}
	}
	reviewTransactionExec(t, peer, "ROLLBACK")
	reviewTransactionExec(t, peer, "UPDATE t SET v='after' WHERE id=1")
	if got := reviewTransactionScalar(t, db, "SELECT v FROM t"); got != "after" {
		t.Fatal(got)
	}
	if tx.reader == nil || tx.storage == nil || tx.savepoints[0].id == 0 || tx.savepoints[1].id == 0 {
		t.Fatal("successful retry did not install the snapshot and both marks")
	}
	reviewTransactionExec(t, db, "UPDATE t SET v='undone' WHERE id=1")
	reviewTransactionExec(t, db, "ROLLBACK TO root")
	if got := reviewTransactionScalar(t, db, "SELECT v FROM t"); got != "after" {
		t.Fatal(got)
	}
	if len(tx.savepoints) != 1 || tx.reader == nil || tx.storage == nil {
		t.Fatal("rollback lost retained snapshot or target")
	}
	reviewTransactionExec(t, db, "RELEASE root")
	if !db.Autocommit() {
		t.Fatal("outer release did not finish retried transaction")
	}
}

func TestReviewSavepointResultsAndOrdinaryIndexConstraintUndo(t *testing.T) {
	ctx := context.Background()
	db, _ := reviewTransactionDB(t)
	reviewTransactionExec(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,v TEXT)")
	reviewTransactionExec(t, db, "CREATE INDEX by_v ON t(v)")
	reviewTransactionExec(t, db, "BEGIN")
	result := reviewTransactionExec(t, db, "INSERT INTO t VALUES(41,'retained')")
	reviewTransactionExec(t, db, "SAVEPOINT root")
	if _, err := db.Exec(ctx, "INSERT INTO t VALUES(42,'transient'),(41,'duplicate-id')"); !errors.Is(err, ErrConstraint) {
		t.Fatalf("multi-row primary-key failure: %v", err)
	}
	if got := reviewTransactionScalar(t, db, "SELECT id FROM t WHERE v='retained'"); got != int64(41) {
		t.Fatal(got)
	}
	reviewTransactionExec(t, db, "INSERT INTO t VALUES(42,'transient')")
	reviewTransactionExec(t, db, "ROLLBACK TO root")
	reviewTransactionExec(t, db, "INSERT INTO t VALUES(42,'transient')")
	reviewTransactionExec(t, db, "ROLLBACK TO root")
	reviewTransactionExec(t, db, "RELEASE root")
	if db.Autocommit() {
		t.Fatal("inner release committed BEGIN")
	}
	reviewTransactionExec(t, db, "COMMIT")
	if id, err := result.LastInsertId(); err != nil || id != 41 {
		t.Fatal(id, err)
	}
	if changes, err := result.RowsAffected(); err != nil || changes != 1 {
		t.Fatal(changes, err)
	}
	if got := reviewTransactionScalar(t, db, "SELECT id FROM t"); got != int64(41) {
		t.Fatal(got)
	}
}

// A preexisting freelist contributes to every materialized pager mark, even
// though the logical SAVEPOINT statements themselves have not accessed it.
// Build that freelist by ordinary SQL and exercise quota failure before writes.
func TestReviewLazySavepointFreelistReplayQuotaReleasesPartialSnapshot(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "freelist-review.db")
	writer, err := Open(path, Options{PageSize: 512})
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	reviewTransactionExec(t, writer, "CREATE TABLE t(id INTEGER PRIMARY KEY,v TEXT)")
	reviewTransactionExec(t, writer, "BEGIN")
	for id := 1; id <= 20; id++ {
		if _, err := writer.Exec(ctx, "INSERT INTO t VALUES(?,?)", id, strings.Repeat("payload", 300)); err != nil {
			t.Fatal(err)
		}
	}
	reviewTransactionExec(t, writer, "COMMIT")
	reviewTransactionExec(t, writer, "DELETE FROM t")
	probe, err := pager.OpenTx(ctx, path, pager.Options{})
	if err != nil {
		t.Fatal(err)
	}
	freeCount := len(probe.FreePages())
	if err := probe.Rollback(); err != nil {
		t.Fatal(err)
	}
	if freeCount < 17 {
		t.Fatalf("ordinary SQL did not build the expected sized freelist: %d", freeCount)
	}
	// This is the documented per-entry allowance in pager/savepoint.go. The
	// limit permits two lazy names or one materialized mark, but not two.
	markCharge := pager.SavepointMetadataBytes("a") + uint64(freeCount)*32
	limit := markCharge + pager.SavepointMetadataBytes("b")/2
	db, err := Open(path, Options{MaxTransactionUndoBytes: int64(limit)})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	reviewTransactionExec(t, db, "SAVEPOINT a")
	reviewTransactionExec(t, db, "SAVEPOINT b")
	tx := db.tx
	if _, err := db.Query(ctx, "SELECT id FROM t"); !errors.Is(err, ErrMemoryLimit) {
		t.Fatalf("preexisting freelist escaped aggregate replay accounting: %v", err)
	}
	if tx != db.tx || tx.reader != nil || tx.storage != nil || len(tx.savepoints) != 2 || db.Autocommit() {
		t.Fatal("replay failure changed logical transaction state")
	}
	for _, mark := range tx.savepoints {
		if mark.id != 0 {
			t.Fatal("failed replay installed a partial mark id")
		}
	}
	// A successful exclusive reservation proves the abandoned partial replay
	// released its normal shared reader before returning the memory error.
	reviewTransactionExec(t, writer, "BEGIN EXCLUSIVE")
	reviewTransactionExec(t, writer, "ROLLBACK")
	reviewTransactionExec(t, db, "RELEASE b")
	rows, err := db.Query(ctx, "SELECT id FROM t")
	if err != nil {
		t.Fatalf("released logical mark did not return replay quota: %v", err)
	}
	if rows.Next() || rows.Err() != nil {
		t.Fatal("deleted rows reappeared", rows.Err())
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	if tx.reader == nil || tx.storage == nil || len(tx.savepoints) != 1 || tx.savepoints[0].id == 0 {
		t.Fatal("successful replay did not retain its snapshot/mark")
	}
	reviewTransactionExec(t, db, "RELEASE a")
	if !db.Autocommit() {
		t.Fatal("outer release retained the retried transaction")
	}
}
