//go:build oracle

package testkit_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Ordinary source DML and schema changes through the pinned SQLite shell verify
// that native DDL/REINDEX cannot bless a different dirty index. The existing
// confCLI helper waits for normal EOF and process completion, without killing.
func TestOracleM4ReviewDirtyPeerSchemaEpochs(t *testing.T) {
	confOracle(t)
	ctx := context.Background()
	for _, shared := range []bool{false, true} {
		name := "rollback"
		if shared {
			name = "shared-WAL"
		}
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "peer.db")
			opts := sqlite.Options{WALShared: shared, KeepWALOnClose: shared}
			var db *sqlite.DB
			open := func() {
				var err error
				db, err = sqlite.Open(path, opts)
				if err != nil {
					t.Fatal(err)
				}
			}
			close := func() {
				if err := db.Close(); err != nil {
					t.Fatal(err)
				}
				db = nil
			}
			open()
			defer func() {
				if db != nil {
					if err := db.Close(); err != nil {
						t.Error(err)
					}
				}
			}()
			for _, table := range []string{"a", "b"} {
				writeExec(t, db, "CREATE TABLE "+table+"(id INTEGER PRIMARY KEY,title TEXT)")
				writeExec(t, db, "INSERT INTO "+table+" VALUES(1,'needle'),(2,'other'),(3,'third')")
				if err := db.CreateGramsIndex(ctx, table+"_grams", table, []string{"title"}); err != nil {
					t.Fatal(err)
				}
			}
			assert := func(index string, clean bool) {
				t.Helper()
				err := db.CheckExtensionIndex(ctx, index)
				if clean && err != nil {
					t.Fatalf("%s unexpectedly dirty: %v", index, err)
				}
				if !clean && !errors.Is(err, sqlite.ErrReindexRequired) {
					t.Fatalf("%s dirty peer was blessed: %v", index, err)
				}
			}
			// DML dirties only a; native unrelated DDL should advance only b.
			close()
			confCLI(t, path, "UPDATE a SET title='external needle' WHERE id=1;", false)
			open()
			assert("a_grams", false)
			assert("b_grams", true)
			writeExec(t, db, "CREATE TABLE unrelated(id INTEGER PRIMARY KEY,note TEXT)")
			assert("a_grams", false)
			assert("b_grams", true)
			if err := db.Reindex(ctx, "a_grams"); err != nil {
				t.Fatal(err)
			}
			assert("a_grams", true)
			assert("b_grams", true)
			// An external schema epoch invalidates both. Rebuilding a and then
			// running native DDL must leave unreconciled b unusable.
			close()
			confCLI(t, path, "CREATE INDEX unrelated_note ON unrelated(note);", false)
			open()
			assert("a_grams", false)
			assert("b_grams", false)
			if err := db.Reindex(ctx, "a_grams"); err != nil {
				t.Fatal(err)
			}
			writeExec(t, db, "CREATE TABLE second_native_table(v TEXT)")
			assert("a_grams", true)
			assert("b_grams", false)
			if err := db.Reindex(ctx, "b_grams"); err != nil {
				t.Fatal(err)
			}
			// Repairing a missing owned guard increments the schema epoch.
			// The repair itself must not clear b's external-DML invalidation.
			close()
			confCLI(t, path, "DROP TRIGGER __gosqlite_ext_guard_1_insert; UPDATE b SET title='other external needle' WHERE id=1;", false)
			open()
			assert("a_grams", false)
			assert("b_grams", false)
			if err := db.Reindex(ctx, "a_grams"); err != nil {
				t.Fatal(err)
			}
			assert("a_grams", true)
			assert("b_grams", false)
			if err := db.Reindex(ctx, "b_grams"); err != nil {
				t.Fatal(err)
			}
			if shared {
				if err := db.Checkpoint(ctx); err != nil {
					t.Fatal(err)
				}
			}
			close()
			writeIntegrity(t, path)
			open()
			assert("a_grams", true)
			assert("b_grams", true)
			for _, table := range []string{"a", "b"} {
				writeSame(t, db, path, "SELECT id,title FROM "+table+" WHERE title LIKE '%needle%' ORDER BY id")
			}
		})
	}
}
