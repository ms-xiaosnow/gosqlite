package sqlite_test

import (
	"bytes"
	"context"
	"errors"
	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/internal/testkit/fileview"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestEncryptionWALPasswordOrdinaryReopenAndCancellation(t *testing.T) {
	ctx := context.Background()
	cfg := encryptionOptions(t, encryption.PasswordArgon2id)
	cfg.Mode = sqlite.EncryptionWAL
	dir := t.TempDir()
	path := filepath.Join(dir, "password-wal.gse")
	db, e := sqlite.Open(path, sqlite.Options{Encryption: &cfg, KeepWALOnClose: true, PageSize: 512})
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	sentinel := "wal-private-最终图像-unique-password-178"
	encryptedExec(t, db, "CREATE TABLE records(id INTEGER PRIMARY KEY,body TEXT)")
	encryptedExec(t, db, "INSERT INTO records VALUES(1,?)", sentinel)
	before, e := fileview.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	names, e := os.ReadDir(dir)
	if e != nil {
		t.Fatal(e)
	}
	noncesBefore := captureEncryptedNonces(t, path)
	tx, e := db.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, "UPDATE records SET body='not-published' WHERE id=1"); e != nil {
		t.Fatal(e)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if e = tx.Commit(cancelled); !errors.Is(e, context.Canceled) {
		t.Fatal("cancelled commit", e)
	}
	if e = tx.Rollback(); e != nil {
		t.Fatal(e)
	}
	after, e := fileview.ReadFile(path)
	if e != nil || !bytes.Equal(before, after) {
		t.Fatal("cancelled commit changed root", e)
	}
	next, e := os.ReadDir(dir)
	if e != nil || len(names) != len(next) {
		t.Fatal("cancelled commit created files", e)
	}
	if !reflect.DeepEqual(noncesBefore, captureEncryptedNonces(t, path)) {
		t.Fatal("cancelled commit/rollback changed nonce-bearing inventory")
	}
	if e = db.Close(); e != nil {
		t.Fatal(e)
	}
	db, e = sqlite.Open(path, sqlite.Options{Encryption: &cfg, ReadOnly: true})
	if e != nil {
		t.Fatal(e)
	}
	r2, e := sqlite.Open(path, sqlite.Options{Encryption: &cfg, ReadOnly: true})
	if e != nil {
		t.Fatal("shared readonly", e)
	}
	if writer, e := sqlite.Open(path, sqlite.Options{Encryption: &cfg}); !errors.Is(e, sqlite.ErrBusy) {
		if writer != nil {
			writer.Close()
		}
		t.Fatal("exclusive writer accepted with readers", e)
	}
	if got := query(t, db, "SELECT body FROM records"); !reflect.DeepEqual(got, [][]any{{sentinel}}) {
		t.Fatal(got)
	}
	if e = r2.Close(); e != nil {
		t.Fatal(e)
	}
	if e = db.Close(); e != nil {
		t.Fatal(e)
	}
	snapshot := cfg
	snapshot.Mode = sqlite.EncryptionSnapshot
	if other, e := sqlite.Open(path, sqlite.Options{Encryption: &snapshot}); !errors.Is(e, sqlite.ErrUnsupported) {
		if other != nil {
			other.Close()
		}
		t.Fatal("silent mode transition", e)
	}
	db, e = sqlite.Open(path, sqlite.Options{Encryption: &cfg})
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if !reflect.DeepEqual(noncesBefore, captureEncryptedNonces(t, path)) {
		t.Fatal("ordinary reopen changed nonce-bearing inventory before a write")
	}
	encryptedExec(t, db, "INSERT INTO records VALUES(2,?)", sentinel)
	noncesAfterWrite := captureEncryptedNonces(t, path)
	assertEncryptedNonceAdvance(t, noncesBefore, noncesAfterWrite, encryption.FileWAL)
	if noncesAfterWrite.root.Epoch != noncesBefore.root.Epoch {
		t.Fatal("ordinary reopen/write unexpectedly changed key epoch")
	}
	if got := query(t, db, "SELECT id,body FROM records ORDER BY id"); !reflect.DeepEqual(got, [][]any{{int64(1), sentinel}, {int64(2), sentinel}}) {
		t.Fatal("ordinary reopen/write changed committed rows")
	}
	if e = db.Close(); e != nil {
		t.Fatal(e)
	}
	db, e = sqlite.Open(path, sqlite.Options{Encryption: &cfg, ReadOnly: true})
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	i, e := db.VerifyEncrypted(ctx)
	if e != nil || i.Mode != sqlite.EncryptionWAL || i.StorageLayout != 2 || i.WALSegments != 0 {
		t.Fatal(i, e)
	}
	noncesAfterClose := captureEncryptedNonces(t, path)
	assertEncryptedNonceAdvance(t, noncesAfterWrite, noncesAfterClose, encryption.FileBase)
	if noncesAfterClose.root.Epoch != noncesAfterWrite.root.Epoch {
		t.Fatal("ordinary close checkpoint unexpectedly changed key epoch")
	}
	names, e = os.ReadDir(dir)
	if e != nil {
		t.Fatal(e)
	}
	for _, name := range names {
		b, e := fileview.ReadFile(filepath.Join(dir, name.Name()))
		if e != nil {
			t.Fatal(e)
		}
		if bytes.Contains(b, []byte(sentinel)) || bytes.Contains(b, []byte("SQLite format 3\x00")) {
			t.Fatal("plaintext marker in encrypted artifact", name.Name())
		}
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		if _, e = os.Stat(path + suffix); !os.IsNotExist(e) {
			t.Fatal("native plaintext sidecar", suffix, e)
		}
	}
}

func TestEncryptionWALNativeOptionPreflight(t *testing.T) {
	cfg := encryptionOptions(t, encryption.RawKey256)
	cfg.Mode = sqlite.EncryptionWAL
	for _, options := range []sqlite.Options{{WALExclusive: true}, {WALShared: true}, {WALPowersafeOverwrite: true}} {
		dir := t.TempDir()
		options.Encryption = &cfg
		if db, e := sqlite.Open(filepath.Join(dir, "unsupported.gse"), options); !errors.Is(e, sqlite.ErrUnsupported) {
			if db != nil {
				db.Close()
			}
			t.Fatal(e)
		}
		entries, e := os.ReadDir(dir)
		if e != nil || len(entries) != 0 {
			t.Fatal("preflight created files", e)
		}
	}
}
