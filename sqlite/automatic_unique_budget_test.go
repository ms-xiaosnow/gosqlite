package sqlite_test

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// A tiny healthy ordinary file reaches an existing allocation quota. There is
// no fault hook, malformed page, I/O injection, large-name disk fixture, or
// recovery operation. All prospective schema records are far below the cap.
func TestAutomaticUniqueAllocationQuotaStatementUndo(t *testing.T) {
	ctx := context.Background()
	db, _ := writable(t, sqlite.Options{PageSize: 4096, MaxTransactionPages: 4})
	execute(t, db, "CREATE TABLE base(id INTEGER PRIMARY KEY,k TEXT)")
	execute(t, db, "INSERT INTO base VALUES(1,'committed')")
	execute(t, db, "BEGIN")
	execute(t, db, "INSERT INTO base VALUES(2,'prior transaction work')")
	execute(t, db, "SAVEPOINT keep")
	before, err := db.Schema(ctx)
	if err != nil {
		t.Fatal(err)
	}
	beforeInfo, beforeCheck := db.Info(), checkPass(t, db)
	checkCounters(t, db, 1, 2, 2)
	// The prior INSERT dirties pages 1/2. The new table root and its schema
	// row, then the first implicit root/row fit pages 1..4. The second index
	// root would require page 5 and fails applyPages' four-dirty-page quota.
	result, err := db.Exec(ctx, "CREATE TABLE attempt(a UNIQUE,b UNIQUE)")
	if !errors.Is(err, sqlite.ErrMemoryLimit) {
		t.Fatal("expected ordinary allocation quota during compound CREATE", err)
	}
	if n, err := result.RowsAffected(); err != nil || n != 0 {
		t.Fatal(n, err)
	}
	if n, err := result.LastInsertId(); err != nil || n != 0 {
		t.Fatal(n, err)
	}
	after, err := db.Schema(ctx)
	if err != nil || !reflect.DeepEqual(before, after) || db.Info() != beforeInfo {
		t.Fatal("failed CREATE leaked schema, roots or cookie", before, after, db.Info(), err)
	}
	if got := checkPass(t, db); got.Pages != beforeCheck.Pages || got.FreePages != beforeCheck.FreePages || got.Tables != beforeCheck.Tables || got.Indexes != beforeCheck.Indexes || got.TableRows != beforeCheck.TableRows || got.IndexEntries != beforeCheck.IndexEntries {
		t.Fatal("failed CREATE leaked allocations or changed prior rows", beforeCheck, got)
	}
	uniqueRows(t, db, "SELECT id,k FROM base ORDER BY id", [][]any{{int64(1), "committed"}, {int64(2), "prior transaction work"}})
	checkCounters(t, db, 1, 2, 2)
	// Retry a smaller schema using the failed name, roll that successful DDL
	// back to the existing savepoint, then retry again with fresh admission.
	execute(t, db, "CREATE TABLE attempt(a UNIQUE)")
	execute(t, db, "ROLLBACK TO keep")
	after, err = db.Schema(ctx)
	if err != nil || !reflect.DeepEqual(before, after) || db.Info() != beforeInfo {
		t.Fatal("rollback-to retained schema admission state", after, err)
	}
	execute(t, db, "CREATE TABLE attempt(a UNIQUE)")
	execute(t, db, "INSERT INTO attempt VALUES('kept')")
	execute(t, db, "RELEASE keep")
	execute(t, db, "COMMIT")
	uniqueConstraint(t, db, "INSERT INTO attempt VALUES('kept')")
	uniqueRows(t, db, "SELECT id,k FROM base ORDER BY id", [][]any{{int64(1), "committed"}, {int64(2), "prior transaction work"}})
	if got := checkPass(t, db); got.Tables != 2 || got.Indexes != 1 || got.TableRows != 3 || got.IndexEntries != 1 {
		t.Fatal("successful retry did not publish complete owned index", got)
	}
}
