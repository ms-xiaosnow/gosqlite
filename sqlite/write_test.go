package sqlite_test

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func writable(t *testing.T, opts sqlite.Options) (*sqlite.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "native.db")
	db, err := sqlite.Open(path, opts)
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
func execute(t *testing.T, db *sqlite.DB, sql string, args ...any) sqlite.Result {
	t.Helper()
	r, err := db.Exec(context.Background(), sql, args...)
	if err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return r
}
func txExecute(t *testing.T, tx *sqlite.Tx, sql string, args ...any) sqlite.Result {
	t.Helper()
	r, err := tx.Exec(context.Background(), sql, args...)
	if err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return r
}
func TestNativeDefaultsAffinityAndRowIDs(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,title TEXT NOT NULL DEFAULT 'default',n NUMERIC DEFAULT (2+3),r REAL,b BLOB)")
	execute(t, db, "CREATE INDEX by_title ON t(title COLLATE BINARY ASC)")
	result := execute(t, db, "INSERT INTO t(title,n,r,b) VALUES(:t,?, ?,?),('two',2,3,X'')", sql.Named("t", "数据库🙂"), "42.0", "3", []byte{0, 255})
	if got, _ := result.RowsAffected(); got != 2 {
		t.Fatal(got)
	}
	if got, _ := result.LastInsertID(); got != 2 {
		t.Fatal(got)
	}
	execute(t, db, "INSERT INTO t DEFAULT VALUES")
	want := [][]any{{int64(1), "数据库🙂", int64(42), float64(3), []byte{0, 255}}, {int64(2), "two", int64(2), float64(3), []byte{}}, {int64(3), "default", int64(5), nil, nil}}
	if got := query(t, db, "SELECT * FROM t ORDER BY id"); !reflect.DeepEqual(got, want) {
		t.Fatalf("%#v", got)
	}
	execute(t, db, "UPDATE t SET n=n+1,title=n,r=n WHERE id=1")
	if got := query(t, db, "SELECT title,n,r FROM t WHERE id=1"); !reflect.DeepEqual(got, [][]any{{"42", int64(43), float64(42)}}) {
		t.Fatal(got)
	}
	execute(t, db, "CREATE TABLE ids(id INTEGER PRIMARY KEY)")
	execute(t, db, "INSERT INTO ids VALUES(-9)")
	r := execute(t, db, "INSERT INTO ids DEFAULT VALUES")
	if id, _ := r.LastInsertId(); id != -8 {
		t.Fatal(id)
	}
	execute(t, db, "INSERT INTO ids VALUES(?)", int64(math.MaxInt64))
	r = execute(t, db, "INSERT INTO ids DEFAULT VALUES")
	if id, _ := r.LastInsertId(); id <= 0 || id == math.MaxInt64 {
		t.Fatal(id)
	}
	for _, s := range []string{"INSERT INTO ids VALUES(1.5)", "UPDATE ids SET id=NULL", "INSERT INTO ids VALUES('bad')"} {
		if _, err := db.Exec(context.Background(), s); !errors.Is(err, sqlite.ErrMismatch) {
			t.Fatal(s, err)
		}
	}
	execute(t, db, "CREATE TABLE shadow(rowid TEXT,oid TEXT,_rowid_ TEXT,id INTEGER PRIMARY KEY)")
	execute(t, db, "INSERT INTO shadow VALUES('r','o','u',77)")
	if got := query(t, db, "SELECT * FROM shadow"); !reflect.DeepEqual(got, [][]any{{"r", "o", "u", int64(77)}}) {
		t.Fatal(got)
	}
}
func TestNativeStatementRollbackAndTransactionalDDL(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	ctx := context.Background()
	execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,v TEXT NOT NULL)")
	execute(t, db, "CREATE INDEX by_v ON t(v)")
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	txExecute(t, tx, "INSERT INTO t VALUES(1,'kept')")
	for _, s := range []string{"INSERT INTO t VALUES(2,'partial'),(1,'duplicate')", "INSERT INTO t VALUES(3,'partial'),(4,NULL)"} {
		if _, err := tx.Exec(ctx, s); !errors.Is(err, sqlite.ErrConstraint) {
			t.Fatal(err)
		}
	}
	rows, err := tx.Query(ctx, "SELECT * FROM t")
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() || rows.Values()[0] != int64(1) {
		t.Fatal(rows.Err())
	}
	if rows.Next() {
		t.Fatal("partial row remained")
	}
	rows.Close()
	txExecute(t, tx, "CREATE TABLE committed(x)")
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if got := query(t, db, "SELECT * FROM t"); !reflect.DeepEqual(got, [][]any{{int64(1), "kept"}}) {
		t.Fatal(got)
	}
	tx, err = db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	txExecute(t, tx, "CREATE TABLE rolledback(x)")
	txExecute(t, tx, "UPDATE t SET id=8,v='discarded'")
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Query(ctx, "SELECT * FROM rolledback"); err == nil {
		t.Fatal("DDL did not roll back")
	}
	if got := query(t, db, "SELECT * FROM t"); !reflect.DeepEqual(got, [][]any{{int64(1), "kept"}}) {
		t.Fatal(got)
	}
	execute(t, db, "BEGIN IMMEDIATE")
	execute(t, db, "DELETE FROM t")
	if len(query(t, db, "SELECT * FROM t")) != 0 {
		t.Fatal("missing own writes")
	}
	execute(t, db, "ROLLBACK")
	if len(query(t, db, "SELECT * FROM t")) != 1 {
		t.Fatal("explicit rollback")
	}
}
func TestNativeSnapshotsBusyAndCleanup(t *testing.T) {
	db, path := writable(t, sqlite.Options{})
	ctx := context.Background()
	execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,v TEXT)")
	execute(t, db, "INSERT INTO t VALUES(1,'old')")
	other, err := sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	rows, err := db.Query(ctx, "SELECT * FROM t")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := other.Begin(ctx, sqlite.TxOptions{Immediate: true})
	if err != nil {
		t.Fatal(err)
	}
	txExecute(t, tx, "INSERT INTO t VALUES(2,'new')")
	if err := tx.Commit(ctx); !errors.Is(err, sqlite.ErrBusy) {
		t.Fatal(err)
	}
	if !rows.Next() || rows.Values()[0] != int64(1) || rows.Next() {
		t.Fatal(rows.Err())
	}
	rows.Close()
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if len(query(t, db, "SELECT * FROM t")) != 2 {
		t.Fatal("new snapshot stale")
	}
	tx, err = db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	rows, err = tx.Query(ctx, "SELECT * FROM t")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "DELETE FROM t"); !errors.Is(err, sqlite.ErrBusy) {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if rows.Next() || !errors.Is(rows.Err(), sqlite.ErrTxDone) {
		t.Fatal(rows.Err())
	}
	if _, err := tx.Query(ctx, "SELECT 1"); !errors.Is(err, sqlite.ErrTxDone) {
		t.Fatal(err)
	}
}
func TestNativeBudgetsCancellationAndReadOnly(t *testing.T) {
	db, path := writable(t, sqlite.Options{MaxTransactionPages: 2, PageSize: 512, MaxSortBytes: 64})
	ctx := context.Background()
	execute(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,v TEXT)")
	execute(t, db, "INSERT INTO t VALUES(1,'keep')")
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "INSERT INTO t VALUES(2,?)", strings.Repeat("x", 20000)); !errors.Is(err, sqlite.ErrMemoryLimit) {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if got := query(t, db, "SELECT v FROM t"); !reflect.DeepEqual(got, [][]any{{"keep"}}) {
		t.Fatal(got)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := db.Exec(cancelled, "INSERT INTO t VALUES(2,'no')"); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	ro, err := sqlite.Open(path, sqlite.Options{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	if _, err := ro.Exec(ctx, "DELETE FROM t"); !errors.Is(err, sqlite.ErrReadOnly) {
		t.Fatal(err)
	}
	rtx, err := ro.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := rtx.Exec(ctx, "DELETE FROM t"); !errors.Is(err, sqlite.ErrReadOnly) {
		t.Fatal(err)
	}
	rtx.Rollback()
	execute(t, db, "INSERT INTO t VALUES(2,'two'),(3,'three')")
	if _, err := db.Exec(ctx, "DELETE FROM t"); !errors.Is(err, sqlite.ErrMemoryLimit) {
		t.Fatal(err)
	}
	if len(query(t, db, "SELECT id FROM t")) != 3 {
		t.Fatal("budget failure modified rows")
	}
}
func TestNativeUnsupportedWritesPreserveValidDatabase(t *testing.T) {
	source := filepath.Join("testdata", "schema.db")
	before, err := os.ReadFile(source)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "valid.db")
	if err := os.WriteFile(path, before, 0600); err != nil {
		t.Fatal(err)
	}
	db, err := sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(context.Background(), "UPDATE empty SET x=1"); !errors.Is(err, sqlite.ErrUnsupported) {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if sha256.Sum256(before) != sha256.Sum256(after) {
		t.Fatal("unsupported schema was changed")
	}
	fresh, _ := writable(t, sqlite.Options{})
	for _, s := range []string{"CREATE TABLE n(id INT PRIMARY KEY)", "CREATE TABLE n(id INTEGER PRIMARY KEY AUTOINCREMENT)", "CREATE TABLE n(x TEXT UNIQUE)", "CREATE TABLE n(x INT CHECK(x>0))", "CREATE TABLE n(x REFERENCES other(x))", "CREATE TABLE n(x) STRICT", "CREATE TABLE n(x) WITHOUT ROWID", "CREATE TABLE n(x GENERATED ALWAYS AS (2))"} {
		if _, err := fresh.Exec(context.Background(), s); !errors.Is(err, sqlite.ErrUnsupported) {
			t.Fatal(s, err)
		}
	}
	execute(t, fresh, "CREATE TABLE t(x)")
	for _, s := range []string{"CREATE UNIQUE INDEX i ON t(x)", "CREATE INDEX i ON t(x DESC)", "CREATE INDEX i ON t(x COLLATE NOCASE)", "CREATE INDEX i ON t(x) WHERE x>0", "CREATE INDEX i ON t(x+1)"} {
		if _, err := fresh.Exec(context.Background(), s); err == nil {
			t.Fatal(s)
		}
	}
}
func TestNativeConcurrentCloseAndRows(t *testing.T) {
	for range 8 {
		db, _ := writable(t, sqlite.Options{})
		execute(t, db, "CREATE TABLE t(x)")
		execute(t, db, "INSERT INTO t VALUES(1),(2),(3)")
		rows, err := db.Query(context.Background(), "SELECT * FROM t")
		if err != nil {
			t.Fatal(err)
		}
		var wg sync.WaitGroup
		wg.Add(3)
		go func() {
			defer wg.Done()
			for rows.Next() {
				_ = rows.Values()
			}
		}()
		go func() { defer wg.Done(); _ = rows.Close() }()
		go func() { defer wg.Done(); _ = db.Close() }()
		wg.Wait()
	}
}

func TestNativeConnectionKeepsCanonicalPath(t *testing.T) {
	dirA := t.TempDir()
	dirB := t.TempDir()
	t.Chdir(dirA)
	db, err := sqlite.Open("same.db", sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	execute(t, db, "CREATE TABLE t(x)")
	execute(t, db, "INSERT INTO t VALUES('original')")
	other, err := sqlite.Open(filepath.Join(dirB, "same.db"), sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	execute(t, other, "CREATE TABLE t(x)")
	execute(t, other, "INSERT INTO t VALUES('other')")
	t.Chdir(dirB)
	execute(t, db, "INSERT INTO t VALUES('after cwd change')")
	if got := query(t, db, "SELECT x FROM t"); !reflect.DeepEqual(got, [][]any{{"original"}, {"after cwd change"}}) {
		t.Fatal(got)
	}
	if got := query(t, other, "SELECT x FROM t"); !reflect.DeepEqual(got, [][]any{{"other"}}) {
		t.Fatal("wrong database was modified", got)
	}
	entries, err := db.Schema(context.Background())
	if err != nil || len(entries) != 1 || entries[0].Name != "t" {
		t.Fatal(entries, err)
	}
}
func TestNativeConnectionKeepsResolvedSymlink(t *testing.T) {
	dir := t.TempDir()
	firstPath := filepath.Join(dir, "first.db")
	secondPath := filepath.Join(dir, "second.db")
	link := filepath.Join(dir, "alias.db")
	first, err := sqlite.Open(firstPath, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	second, err := sqlite.Open(secondPath, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	execute(t, first, "CREATE TABLE t(x)")
	execute(t, first, "INSERT INTO t VALUES('first')")
	execute(t, second, "CREATE TABLE t(x)")
	execute(t, second, "INSERT INTO t VALUES('second')")
	if err := os.Symlink(firstPath, link); err != nil {
		t.Fatal(err)
	}
	throughLink, err := sqlite.Open(link, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer throughLink.Close()
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(secondPath, link); err != nil {
		t.Fatal(err)
	}
	execute(t, throughLink, "INSERT INTO t VALUES('still first')")
	if got := query(t, first, "SELECT x FROM t"); !reflect.DeepEqual(got, [][]any{{"first"}, {"still first"}}) {
		t.Fatal(got)
	}
	if got := query(t, throughLink, "SELECT x FROM t"); !reflect.DeepEqual(got, [][]any{{"first"}, {"still first"}}) {
		t.Fatal(got)
	}
	if got := query(t, second, "SELECT x FROM t"); !reflect.DeepEqual(got, [][]any{{"second"}}) {
		t.Fatal("symlink retarget redirected write", got)
	}
}

func TestNativeSectorOptions(t *testing.T) {
	db, path := writable(t, sqlite.Options{SectorSize: 8192, PageSize: 512})
	execute(t, db, "CREATE TABLE t(x)")
	execute(t, db, "INSERT INTO t VALUES(1)")
	for _, sector := range []uint32{1, 1000, 131072} {
		for _, name := range []string{path, filepath.Join(t.TempDir(), "absent.db")} {
			if opened, err := sqlite.Open(name, sqlite.Options{SectorSize: sector}); !errors.Is(err, sqlite.ErrUnsupported) {
				if opened != nil {
					opened.Close()
				}
				t.Fatalf("sector %d: %v", sector, err)
			}
		}
	}
}
func TestNativeTransactionDetectsReplacedValidFile(t *testing.T) {
	db, path := writable(t, sqlite.Options{})
	ctx := context.Background()
	execute(t, db, "CREATE TABLE t(x)")
	execute(t, db, "INSERT INTO t VALUES('original')")
	replacementPath := filepath.Join(filepath.Dir(path), "replacement.db")
	replacement, err := sqlite.Open(replacementPath, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	execute(t, replacement, "CREATE TABLE t(x)")
	execute(t, replacement, "INSERT INTO t VALUES('replacement')")
	replacement.Close()
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	txExecute(t, tx, "INSERT INTO t VALUES('uncommitted')")
	// Both files are valid SQLite databases. Replacing a pathname while an
	// existing transaction owns its descriptor must not redirect its commit.
	if err := os.Rename(replacementPath, path); err != nil {
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); !errors.Is(err, sqlite.ErrChanged) {
		t.Fatal(err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	current, err := sqlite.Open(path, sqlite.Options{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer current.Close()
	if got := query(t, current, "SELECT x FROM t"); !reflect.DeepEqual(got, [][]any{{"replacement"}}) {
		t.Fatal("replacement was modified", got)
	}
}
