//go:build manual_namespace

package sqlite_test

// Restricted namespace-mutation regressions. These were accidentally included
// in historical default suites before the 2026-10-10 scope correction. Their
// previous PASS results are factual, not evidence of clean ordinary-test scope.
// They are now excluded from ordinary/CI/oracle builds and guarded separately.
// Do not execute here or relocate them to another environment. Compilation only:
//   CGO_ENABLED=0 go test -c -tags manual_namespace -o /tmp/namespace.test ./sqlite
// Any later independent authorized manual execution requires the explicit
// GOSQLITE_MANUAL_NAMESPACE=independently_authorized gate. No such execution is
// authorized or performed by this correction.

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestNativeConnectionKeepsResolvedSymlink(t *testing.T) {
	if os.Getenv("GOSQLITE_MANUAL_NAMESPACE") != "independently_authorized" {
		t.Skip("restricted namespace-mutation test: source only in ordinary workflows")
	}
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

func TestNativeTransactionDetectsReplacedValidFile(t *testing.T) {
	if os.Getenv("GOSQLITE_MANUAL_NAMESPACE") != "independently_authorized" {
		t.Skip("restricted namespace-mutation test: source only in ordinary workflows")
	}
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
