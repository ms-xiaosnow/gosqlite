//go:build linux

package sqlite_test

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

type readOnlyWALFamilyMember struct {
	Size int64
	Hash [32]byte
}
func readOnlyWALFamily(t *testing.T, dir string) map[string]readOnlyWALFamilyMember {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil { t.Fatal(err) }
	result := make(map[string]readOnlyWALFamilyMember)
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || !info.Mode().IsRegular() { t.Fatal("unexpected family member", entry.Name(), err) }
		b, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil { t.Fatal(err) }
		result[entry.Name()] = readOnlyWALFamilyMember{Size: info.Size(), Hash: sha256.Sum256(b)}
	}
	return result
}
func readOnlyWALKeeper(t *testing.T, path string, pageSize uint32) *sqlite.DB {
	t.Helper()
	db, err := sqlite.Open(path, sqlite.Options{WALShared: true, PageSize: pageSize, KeepWALOnClose: true})
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { if err := db.Close(); err != nil { t.Error(err) } })
	for _, q := range []string{"CREATE TABLE t(id INTEGER PRIMARY KEY,text TEXT)", "CREATE INDEX ix ON t(text)", "INSERT INTO t VALUES(1,'current'),(2,'stable')"} {
		if _, err := db.Exec(context.Background(), q); err != nil { t.Fatal(err) }
	}
	return db
}
func readOnlyWALValue(t *testing.T, q interface{ Query(context.Context, string, ...any) (*sqlite.Rows, error) }) string {
	t.Helper()
	r, err := q.Query(context.Background(), "SELECT text FROM t WHERE id=1")
	if err != nil { t.Fatal(err) }
	defer r.Close()
	if !r.Next() { t.Fatal("missing row", r.Err()) }
	value, ok := r.Values()[0].(string)
	if !ok || r.Next() || r.Err() != nil { t.Fatal("unexpected row", r.Err()) }
	if err := r.Close(); err != nil { t.Fatal(err) }
	return value
}
func TestReadOnlySharedWALNormalLiveSnapshot(t *testing.T) {
	ctx := context.Background()
	for _, pageSize := range []uint32{512, 4096, 65536} {
		for _, keep := range []bool{false, true} {
			t.Run(fmt.Sprintf("%d-keep-%v", pageSize, keep), func(t *testing.T) {
				dir := t.TempDir()
				path := filepath.Join(dir, "readonly.db")
				_ = readOnlyWALKeeper(t, path, pageSize)
				before := readOnlyWALFamily(t, dir)
				if len(before) != 3 { t.Fatal("expected main/WAL/SHM", before) }
				db, err := sqlite.Open(path, sqlite.Options{ReadOnly: true, WALShared: true, KeepWALOnClose: keep})
				if err != nil { t.Fatal(err) }
				defer db.Close()
				if got := readOnlyWALValue(t, db); got != "current" { t.Fatal(got) }
				if schema, err := db.Schema(ctx); err != nil || len(schema) != 2 { t.Fatal(schema, err) }
				if inspection, err := db.Inspect(ctx, sqlite.InspectOptions{IncludeExtensions: true}); err != nil || inspection.Header.PageSize != pageSize { t.Fatal(inspection, err) }
				if check, err := db.Check(ctx); err != nil || check.Scope != sqlite.CheckScope || check.TableRows != 2 { t.Fatal(check, err) }
				if err := db.Close(); err != nil { t.Fatal(err) }
				if !reflect.DeepEqual(before, readOnlyWALFamily(t, dir)) { t.Fatal("quiescent readonly open/read/close changed names, sizes, or bytes") }
			})
		}
	}
}
func TestReadOnlySharedWALPublicWriteAndCheckpointRefusal(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "refusal.db")
	keeper := readOnlyWALKeeper(t, path, 4096)
	db, err := sqlite.Open(path, sqlite.Options{ReadOnly: true, WALShared: true})
	if err != nil { t.Fatal(err) }
	defer db.Close()
	before := readOnlyWALFamily(t, dir)
	assertReadOnly := func(err error) {
		t.Helper()
		var coded *sqlite.Error
		if !errors.Is(err, sqlite.ErrReadOnly) || !errors.As(err, &coded) || coded.Code != sqlite.CodeReadOnly { t.Fatal(err) }
	}
	for _, q := range []string{"UPDATE t SET text='forbidden' WHERE id=1", "INSERT INTO t VALUES(3,'forbidden')", "CREATE TABLE forbidden(id INTEGER PRIMARY KEY)"} {
		_, err := db.Exec(ctx, q)
		assertReadOnly(err)
	}
	assertReadOnly(db.Checkpoint(ctx))
	for _, opts := range []sqlite.TxOptions{{Immediate: true}, {Exclusive: true}} {
		_, err := db.Begin(ctx, opts)
		assertReadOnly(err)
	}
	for _, commit := range []bool{false, true} {
		tx, err := db.Begin(ctx, sqlite.TxOptions{})
		if err != nil { t.Fatal(err) }
		if got := readOnlyWALValue(t, tx); got != "current" { t.Fatal(got) }
		_, err = tx.Exec(ctx, "DELETE FROM t")
		assertReadOnly(err)
		if commit { err = tx.Commit(ctx) } else { err = tx.Rollback() }
		if err != nil { t.Fatal(err) }
	}
	if err := db.Close(); err != nil { t.Fatal(err) }
	if !reflect.DeepEqual(before, readOnlyWALFamily(t, dir)) { t.Fatal("refusal or readonly transaction modified family") }
	// A separate concurrent interval tests row/transaction pinning. Its writer
	// changes are intentionally excluded from the quiescent byte assertion.
	db, err = sqlite.Open(path, sqlite.Options{ReadOnly: true, WALShared: true})
	if err != nil { t.Fatal(err) }
	defer db.Close()
	tx, err := db.Begin(ctx, sqlite.TxOptions{ReadOnly: true})
	if err != nil { t.Fatal(err) }
	defer tx.Rollback()
	if got := readOnlyWALValue(t, tx); got != "current" { t.Fatal(got) }
	if _, err := keeper.Exec(ctx, "UPDATE t SET text='newer' WHERE id=1"); err != nil { t.Fatal(err) }
	if got := readOnlyWALValue(t, tx); got != "current" { t.Fatal(got) }
	if err := tx.Commit(ctx); err != nil { t.Fatal(err) }
	if got := readOnlyWALValue(t, db); got != "newer" { t.Fatal(got) }
	rows, err := db.Query(ctx, "SELECT id,text FROM t ORDER BY id")
	if err != nil { t.Fatal(err) }
	defer rows.Close()
	if _, err := keeper.Exec(ctx, "UPDATE t SET text='after-rows' WHERE id=1"); err != nil { t.Fatal(err) }
	if !rows.Next() || rows.Values()[1] != "newer" { t.Fatal("Rows lost pinned snapshot", rows.Err()) }
	if !rows.Next() || rows.Values()[1] != "stable" || rows.Next() || rows.Err() != nil { t.Fatal("Rows changed", rows.Err()) }
	if err := rows.Close(); err != nil { t.Fatal(err) }
	if got := readOnlyWALValue(t, db); got != "after-rows" { t.Fatal(got) }
	if err := db.Close(); err != nil { t.Fatal(err) }
}
