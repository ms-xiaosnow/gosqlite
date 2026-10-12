package sqlite_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func checkConstraintFailure(t *testing.T, db interface {
	Exec(context.Context, string, ...any) (sqlite.Result, error)
}, query string, args ...any) {
	t.Helper()
	r, err := db.Exec(context.Background(), query, args...)
	var detail *sqlite.Error
	if !errors.Is(err, sqlite.ErrConstraint) || !errors.As(err, &detail) || detail.Code != sqlite.CodeConstraint || !strings.Contains(err.Error(), "CHECK constraint failed:") {
		t.Fatal("want CHECK primary constraint error", query, err)
	}
	if n, err := r.RowsAffected(); err != nil || n != 0 {
		t.Fatal("failed statement affected rows", n, err)
	}
	if n, err := r.LastInsertId(); err != nil || n != 0 {
		t.Fatal("failed statement result id", n, err)
	}
}

// Only healthy, SQL-created ordinary rowid files and logical constraint/quota
// failures. No ignored CHECKs, physical corruption, fault injection or recovery.
func TestCheckConstraintsAtomicRowsAndReopen(t *testing.T) {
	ctx := context.Background()
	db, path := writable(t, sqlite.Options{PageSize: 512})
	definition := "CREATE TABLE t(id INTEGER PRIMARY KEY CHECK(id>=-9),a NUMERIC NOT NULL DEFAULT '02' CONSTRAINT positive CHECK(a>0),b NUMERIC DEFAULT 10,note TEXT UNIQUE,CHECK(a<=b))"
	execute(t, db, definition)
	execute(t, db, "CREATE INDEX by_a ON t(a)")
	beforeSchema, err := db.Schema(ctx)
	if err != nil || len(beforeSchema) != 3 {
		t.Fatal("CHECK added a schema root", beforeSchema, err)
	}
	if got := query(t, db, "SELECT sql FROM sqlite_schema WHERE name='t'"); !reflect.DeepEqual(got, [][]any{{definition}}) {
		t.Fatal("original CHECK SQL changed", got)
	}
	cookie := db.Info().SchemaCookie
	execute(t, db, "INSERT INTO t VALUES(-9,'01',2,'base')")
	execute(t, db, "INSERT INTO t DEFAULT VALUES")
	execute(t, db, "BEGIN")
	execute(t, db, "INSERT INTO t VALUES(5,5,9,'kept')")
	execute(t, db, "SAVEPOINT s")
	checkConstraintFailure(t, db, "INSERT INTO t VALUES(6,6,9,'partial'),(7,0,9,'bad')")
	checkCounters(t, db, 0, 3, 6)
	checkConstraintFailure(t, db, "UPDATE t SET a=CASE WHEN id=-9 THEN 2 ELSE 0 END WHERE id<0")
	checkCounters(t, db, 0, 3, 6)
	want := [][]any{{int64(-9), int64(1), int64(2), "base"}, {int64(-8), int64(2), int64(10), nil}, {int64(5), int64(5), int64(9), "kept"}}
	uniqueRows(t, db, "SELECT id,a,b,note FROM t ORDER BY id", want)
	if r := checkPass(t, db); r.TableRows != 3 || r.Indexes != 2 || r.IndexEntries != 6 {
		t.Fatal("statement undo lost table/index entries", r)
	}
	checkConstraintFailure(t, db, "UPDATE t SET id=-10 WHERE id=5")
	execute(t, db, "UPDATE t SET a=a+1,b=b+1")
	execute(t, db, "ROLLBACK TO s")
	execute(t, db, "RELEASE s")
	uniqueRows(t, db, "SELECT id,a,b,note FROM t ORDER BY id", want)
	execute(t, db, "UPDATE t SET a=a,b=b")
	execute(t, db, "COMMIT")
	if _, err = db.Schema(ctx); err != nil || db.Info().SchemaCookie != cookie {
		t.Fatal("CHECK failure changed schema cookie", err)
	}
	checkPass(t, db)
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	checkConstraintFailure(t, db, "INSERT INTO t VALUES(10,-1,10,'bad reopen')")
	execute(t, db, "INSERT INTO t VALUES(10,2,10,'new')")
	execute(t, db, "DELETE FROM t WHERE id=5")
	uniqueConstraint(t, db, "INSERT INTO t VALUES(11,1,10,'base')")
	if schema, err := db.Schema(ctx); err != nil || !reflect.DeepEqual(schema, beforeSchema) {
		t.Fatal("reopen changed CHECK/UNIQUE catalog", schema, err)
	}
	checkPass(t, db)
}

func TestCheckConstraintsTruthAffinityAndDiagnostics(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE truth(id INTEGER PRIMARY KEY,v CONSTRAINT truth_value CHECK(v))")
	for i, value := range []any{nil, int64(1), float64(-0.25), "1tail", "-0.1tail", []byte{'1'}} {
		execute(t, db, "INSERT INTO truth VALUES(?,?)", i, value)
	}
	for _, value := range []any{int64(0), float64(0), "", "word", "0x1", "0_1", []byte{0}} {
		checkConstraintFailure(t, db, "INSERT INTO truth VALUES(99,?)", value)
	}
	_, err := db.Exec(context.Background(), "INSERT INTO truth VALUES(99,0)")
	if err == nil || !strings.Contains(err.Error(), "truth_value") {
		t.Fatal("named CHECK diagnostic", err)
	}
	execute(t, db, "CREATE TABLE affine(n NUMERIC CHECK(n=1),t TEXT CHECK(t='1'),b BLOB CHECK(b=X'31'))")
	execute(t, db, "INSERT INTO affine VALUES('01',1,X'31')")
	execute(t, db, "CREATE TABLE branches(a CHECK(NULL OR ('x' LIKE 'x' ESCAPE 'xx')))")
	execute(t, db, "INSERT INTO branches VALUES(1)")
	execute(t, db, "CREATE TABLE empty(a CHECK(0))")
	checkConstraintFailure(t, db, "INSERT INTO empty VALUES(1)")
	execute(t, db, "UPDATE empty SET a=1")
	execute(t, db, "DELETE FROM empty")
	execute(t, db, `CREATE TABLE shadow(rowid TEXT,"true","false",CHECK(rowid='r'),CHECK(true>false),CHECK(oid>0))`)
	execute(t, db, "INSERT INTO shadow VALUES('r',2,1)")
	checkConstraintFailure(t, db, "UPDATE shadow SET _rowid_=-1")
	checkConstraintFailure(t, db, "UPDATE shadow SET true=false")
	checkPass(t, db)
}

func TestCheckConstraintsQuotaStatementUndo(t *testing.T) {
	db, _ := writable(t, sqlite.Options{PageSize: 512, MaxSortBytes: 16384})
	execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,a,note UNIQUE,CHECK(CASE WHEN id=1 THEN 1 ELSE a END))")
	execute(t, db, "CREATE INDEX by_a ON t(a)")
	execute(t, db, "BEGIN")
	execute(t, db, "INSERT INTO t VALUES(0,1,'kept')")
	execute(t, db, "SAVEPOINT s")
	r, err := db.Exec(context.Background(), "INSERT INTO t VALUES(1,1,'partial'),(2,?,'quota')", strings.Repeat("1", 4096))
	if !errors.Is(err, sqlite.ErrMemoryLimit) || !strings.Contains(err.Error(), "CHECK") {
		t.Fatal("want bounded scalar quota failure", err)
	}
	if n, e := r.RowsAffected(); e != nil || n != 0 {
		t.Fatal(n, e)
	}
	checkCounters(t, db, 0, 1, 1)
	uniqueRows(t, db, "SELECT id,a,note FROM t", [][]any{{int64(0), int64(1), "kept"}})
	if checked := checkPass(t, db); checked.TableRows != 1 || checked.IndexEntries != 2 {
		t.Fatal("quota rollback lost prior work or indexes", checked)
	}
	execute(t, db, "ROLLBACK TO s")
	execute(t, db, "INSERT INTO t VALUES(2,1,'retry')")
	execute(t, db, "RELEASE s")
	execute(t, db, "COMMIT")
	checkPass(t, db)
}
