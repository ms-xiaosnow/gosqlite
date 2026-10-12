package sqlite_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"github.com/ms-xiaosnow/gosqlite/internal/testkit/fileview"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// This independent review test uses only correct credentials, normal SQL,
// rollback, successful rekey and ordinary reopen of intact generated files.
// Ciphertext is observed read-only; no failure or corruption is injected.
func TestEncryptionReviewFreshIdentityAndRetainedBytes(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "review.gse")
	cfg := encryptionOptions(t, encryption.RawKey256)
	db, err := sqlite.Open(path, sqlite.Options{Encryption: &cfg, PageSize: 512})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if db != nil {
			_ = db.Close()
		}
	}()
	encryptedExec(t, db, "CREATE TABLE retained(id INTEGER PRIMARY KEY,payload BLOB)")
	payload := bytes.Repeat([]byte("ordinary-overflow-page-你好"), 100)
	encryptedExec(t, db, "INSERT INTO retained VALUES(1,?)", payload)
	encryptedExec(t, db, "DELETE FROM retained WHERE id=1")
	encryptedExec(t, db, "INSERT INTO retained VALUES(2,?)", payload)
	before, err := fileview.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	head, err := encryption.DecodeHeader(before[:encryption.HeaderSize], encryption.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	old := make(map[string][32]byte)
	for _, f := range files {
		if f.Name() == filepath.Base(path) {
			continue
		}
		b, e := fileview.ReadFile(filepath.Join(dir, f.Name()))
		if e != nil {
			t.Fatal(e)
		}
		old[f.Name()] = sha256.Sum256(b)
	}
	noncesBefore := captureEncryptedNonces(t, path)
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, "UPDATE retained SET payload=? WHERE id=2", []byte("rolled-back")); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	afterRollback, err := fileview.ReadFile(path)
	if err != nil || !bytes.Equal(before, afterRollback) {
		t.Fatalf("ordinary rollback changed committed root: %v", err)
	}
	if !reflect.DeepEqual(noncesBefore, captureEncryptedNonces(t, path)) {
		t.Fatal("ordinary rollback changed nonce-bearing inventory")
	}
	// Rekeying with the same application credential still needs fresh epoch,
	// salt and file identities. We never open with a wrong credential.
	if err = db.Rekey(ctx, cfg); err != nil {
		t.Fatal(err)
	}
	after, err := fileview.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	next, err := encryption.DecodeHeader(after[:encryption.HeaderSize], encryption.Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if next.Epoch.DatabaseID != head.Epoch.DatabaseID || next.Epoch.EpochID == head.Epoch.EpochID || next.Epoch.RootSalt == head.Epoch.RootSalt || next.FileID == head.FileID {
		t.Fatal("same-credential rekey did not preserve DBID and replace epoch/root identities")
	}
	if next.Descriptor.ParentCommitID != sha256.Sum256(before) || next.Descriptor.CommitSequence != head.Descriptor.CommitSequence+1 {
		t.Fatal("new root does not identify its immediate predecessor")
	}
	noncesAfterRekey := captureEncryptedNonces(t, path)
	assertEncryptedNonceAdvance(t, noncesBefore, noncesAfterRekey, encryption.FileBase)
	for name, hash := range old {
		b, e := fileview.ReadFile(filepath.Join(dir, name))
		if e != nil || sha256.Sum256(b) != hash {
			t.Fatalf("retained artifact changed during rekey: %s: %v", name, e)
		}
	}
	if _, err = db.VerifyEncrypted(ctx); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = sqlite.Open(path, sqlite.Options{Encryption: &cfg, ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := query(t, db, "SELECT id,payload FROM retained"); !reflect.DeepEqual(got, [][]any{{int64(2), payload}}) {
		t.Fatal("freelist reuse/rekey/reopen changed ordinary data")
	}
	if !reflect.DeepEqual(noncesAfterRekey, captureEncryptedNonces(t, path)) {
		t.Fatal("ordinary post-rekey reopen changed nonce-bearing inventory")
	}
}
