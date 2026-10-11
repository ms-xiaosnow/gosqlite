package sqlite_test

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Intact correct-key successful rekey only. All competing connections retire
// normally before rekey; no native lock/close failure or wrong-key open occurs.
// Mark each close attempt before calling Close, so cleanup cannot retry a failed
// native retirement. Each successful path checks its explicit Close result.
func TestEncryptionSharedRekeyLatestHeadOrdinary(t *testing.T) {
	ctx := context.Background()
	cfg := encryptionOptions(t, encryption.RawKey256)
	cfg.Mode = sqlite.EncryptionWALShared
	next := encryptionOptions(t, encryption.RawKey256)
	next.Mode = sqlite.EncryptionWALShared
	path := filepath.Join(t.TempDir(), "shared-rekey-latest.gse")
	options := sqlite.Options{Encryption: &cfg, KeepWALOnClose: true, PageSize: 512}
	a, err := sqlite.Open(path, options)
	if err != nil {
		t.Fatal(err)
	}
	aCloseAttempted := false
	t.Cleanup(func() {
		if !aCloseAttempted {
			aCloseAttempted = true
			if closeErr := a.Close(); closeErr != nil {
				t.Error("cleanup a:", closeErr)
			}
		}
	})
	encryptedExec(t, a, "CREATE TABLE t(id INTEGER PRIMARY KEY,v TEXT)")
	encryptedExec(t, a, "INSERT INTO t VALUES(1,'first')")
	before, err := a.VerifyEncrypted(ctx)
	if err != nil {
		t.Fatal(err)
	}
	b, err := sqlite.Open(path, options)
	if err != nil {
		t.Fatal(err)
	}
	bCloseAttempted := false
	t.Cleanup(func() {
		if !bCloseAttempted {
			bCloseAttempted = true
			if closeErr := b.Close(); closeErr != nil {
				t.Error("cleanup b:", closeErr)
			}
		}
	})
	encryptedExec(t, b, "UPDATE t SET v='latest'")
	encryptedExec(t, b, "INSERT INTO t VALUES(2,'also copied')")
	bCloseAttempted = true
	if err = b.Close(); err != nil {
		t.Fatal(err)
	}
	// A has not refreshed since B published; rekey itself must admit the latest.
	if err = a.Rekey(ctx, next); err != nil {
		t.Fatal(err)
	}
	want := [][]any{{int64(1), "latest"}, {int64(2), "also copied"}}
	if got := query(t, a, "SELECT id,v FROM t ORDER BY id"); !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
	after, err := a.VerifyEncrypted(ctx)
	if err != nil || after.DatabaseID != before.DatabaseID || after.StorageLayout != 3 || after.Mode != sqlite.EncryptionWALShared || after.WALSegments != 0 || after.CommitSequence <= before.CommitSequence || after.HeadCommitID == before.HeadCommitID {
		t.Fatal(after, err)
	}
	aCloseAttempted = true
	if err = a.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := sqlite.Open(path, sqlite.Options{Encryption: &next, NoCreate: true, ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	reopenedCloseAttempted := false
	t.Cleanup(func() {
		if !reopenedCloseAttempted {
			reopenedCloseAttempted = true
			if closeErr := reopened.Close(); closeErr != nil {
				t.Error("cleanup reopened:", closeErr)
			}
		}
	})
	if got := query(t, reopened, "SELECT id,v FROM t ORDER BY id"); !reflect.DeepEqual(got, want) {
		t.Fatal(got)
	}
	if _, err = reopened.VerifyEncrypted(ctx); err != nil {
		t.Fatal(err)
	}
	reopenedCloseAttempted = true
	if err = reopened.Close(); err != nil {
		t.Fatal(err)
	}
}
