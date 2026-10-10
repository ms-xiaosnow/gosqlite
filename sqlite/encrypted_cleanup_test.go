package sqlite_test

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Only intact correct-key files and ordinary close/rollback/reopen are used.
func TestEncryptedCleanupOrdinaryAllModesCloseAndProviders(t *testing.T) {
	for _, mode := range []sqlite.EncryptionMode{sqlite.EncryptionSnapshot, sqlite.EncryptionWAL, sqlite.EncryptionWALShared} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			cfg := encryptionOptions(t, encryption.RawKey256)
			cfg.Mode = mode
			p := cfg.KeyProvider.(*encryptionTestProvider)
			path := filepath.Join(t.TempDir(), "normal.gse")
			db, e := sqlite.Open(path, sqlite.Options{Encryption: &cfg, PageSize: 512})
			if e != nil {
				t.Fatal(e)
			}
			encryptedExec(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY)")
			encryptedExec(t, db, "INSERT INTO t VALUES(1)")
			tx, e := db.Begin(context.Background())
			if e != nil {
				t.Fatal(e)
			}
			if _, e = tx.Exec(context.Background(), "INSERT INTO t VALUES(2)"); e != nil {
				t.Fatal(e)
			}
			rows, e := tx.Query(context.Background(), "SELECT id FROM t ORDER BY id")
			if e != nil {
				t.Fatal(e)
			}
			if e = db.Close(); e != nil {
				t.Fatal(e)
			}
			if e = db.Close(); e != nil {
				t.Fatal(e)
			}
			if !errors.Is(rows.Err(), sqlite.ErrClosed) {
				t.Fatal(rows.Err())
			}
			if p.acquires.Load() != 1 || p.releases.Load() != 1 {
				t.Fatal("close reacquired provider", p.acquires.Load(), p.releases.Load())
			}
			db, e = sqlite.Open(path, sqlite.Options{Encryption: &cfg, ReadOnly: true})
			if e != nil {
				t.Fatal(e)
			}
			if got := query(t, db, "SELECT id FROM t"); !reflect.DeepEqual(got, [][]any{{int64(1)}}) {
				t.Fatal(got)
			}
			if e = db.Close(); e != nil {
				t.Fatal(e)
			}
			if p.acquires.Load() != 2 || p.releases.Load() != 2 {
				t.Fatal("reopen provider count")
			}
			if len(sqlite.PendingEncryptedCleanups()) != 0 {
				t.Fatal("normal path retained pending ownership")
			}
		})
	}
}
