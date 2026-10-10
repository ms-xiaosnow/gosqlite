package sqlite_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync/atomic"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

type encryptionTestProvider struct {
	kind               encryption.CredentialKind
	key                []byte
	acquires, releases atomic.Int64
}
type encryptionTestMaterial struct {
	provider *encryptionTestProvider
	key      []byte
}

func (p *encryptionTestProvider) Acquire(context.Context, encryption.KeyRequest) (encryption.KeyMaterial, error) {
	p.acquires.Add(1)
	return &encryptionTestMaterial{provider: p, key: bytes.Clone(p.key)}, nil
}
func (m *encryptionTestMaterial) Kind() encryption.CredentialKind { return m.provider.kind }
func (m *encryptionTestMaterial) Bytes() []byte                   { return m.key }
func (m *encryptionTestMaterial) Release()                        { clear(m.key); m.provider.releases.Add(1) }
func encryptionOptions(t *testing.T, kind encryption.CredentialKind) sqlite.EncryptionOptions {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	p := &encryptionTestProvider{kind: kind, key: key}
	t.Cleanup(func() {
		clear(key)
		if p.acquires.Load() != p.releases.Load() {
			t.Error("provider releases disagree")
		}
	})
	return sqlite.EncryptionOptions{KeyProvider: p, CredentialKind: kind}
}
func encryptedExec(t *testing.T, db *sqlite.DB, q string, args ...any) {
	t.Helper()
	if _, err := db.Exec(context.Background(), q, args...); err != nil {
		t.Fatal(err)
	}
}

func TestEncryptionOrdinaryLifecycleAndCopies(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "main.gse")
	cfg := encryptionOptions(t, encryption.RawKey256)
	db, err := sqlite.Open(path, sqlite.Options{Encryption: &cfg, PageSize: 512})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	encryptedExec(t, db, "CREATE TABLE entries(id INTEGER PRIMARY KEY,body TEXT,payload BLOB)")
	sentinel := "unique-encrypted-test-你好-abcdefghijk"
	blob := bytes.Repeat([]byte{91, 92, 93, 94}, 800)
	encryptedExec(t, db, "INSERT INTO entries VALUES(?,?,?)", 1, sentinel, blob)
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, "UPDATE entries SET body='rollback-only' WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	want := [][]any{{int64(1), sentinel, blob}}
	if got := query(t, db, "SELECT id,body,payload FROM entries"); !reflect.DeepEqual(got, want) {
		t.Fatal("initial rows mismatch")
	}
	info, err := db.VerifyEncrypted(ctx)
	if err != nil || !info.RetainsHistory || info.PageCount < 2 {
		t.Fatal(info, err)
	}
	next := encryptionOptions(t, encryption.RawKey256)
	if err = db.Rekey(ctx, next); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = sqlite.Open(path, sqlite.Options{Encryption: &next, NoCreate: true})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if got := query(t, db, "SELECT id,body,payload FROM entries"); !reflect.DeepEqual(got, want) {
		t.Fatal("rekey rows mismatch")
	}
	backupKey := encryptionOptions(t, encryption.RawKey256)
	archive := filepath.Join(dir, "backup.gse")
	if _, err = db.BackupEncrypted(ctx, archive, backupKey); err != nil {
		t.Fatal(err)
	}
	restoredKey := encryptionOptions(t, encryption.RawKey256)
	restored := filepath.Join(dir, "restored.gse")
	if _, err = sqlite.RestoreEncrypted(ctx, archive, restored, backupKey, restoredKey); err != nil {
		t.Fatal(err)
	}
	copyDB, err := sqlite.Open(restored, sqlite.Options{Encryption: &restoredKey})
	if err != nil {
		t.Fatal(err)
	}
	if got := query(t, copyDB, "SELECT id,body,payload FROM entries"); !reflect.DeepEqual(got, want) {
		t.Fatal("restored rows mismatch")
	}
	if err = copyDB.Close(); err != nil {
		t.Fatal(err)
	}
	plain := filepath.Join(t.TempDir(), "export.db")
	if _, err = db.ExportPlaintext(ctx, plain); err != nil {
		t.Fatal(err)
	}
	plainDB, err := sqlite.Open(plain, sqlite.Options{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := query(t, plainDB, "SELECT id,body,payload FROM entries"); !reflect.DeepEqual(got, want) {
		t.Fatal("export rows mismatch")
	}
	if err = plainDB.Close(); err != nil {
		t.Fatal(err)
	}
	importedKey := encryptionOptions(t, encryption.RawKey256)
	imported := filepath.Join(dir, "imported.gse")
	if _, err = sqlite.ImportEncrypted(ctx, plain, imported, sqlite.Options{}, importedKey); err != nil {
		t.Fatal(err)
	}
	copyDB, err = sqlite.Open(imported, sqlite.Options{Encryption: &importedKey})
	if err != nil {
		t.Fatal(err)
	}
	if got := query(t, copyDB, "SELECT id,body,payload FROM entries"); !reflect.DeepEqual(got, want) {
		t.Fatal("import rows mismatch")
	}
	if err = copyDB.Close(); err != nil {
		t.Fatal(err)
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		data, err := os.ReadFile(filepath.Join(dir, file.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(data, []byte(sentinel)) || bytes.Contains(data, []byte("SQLite format 3\x00")) {
			t.Fatalf("plaintext marker found in valid encrypted artifact %s", file.Name())
		}
	}
}

func TestEncryptionPasswordNormalReopen(t *testing.T) {
	cfg := encryptionOptions(t, encryption.PasswordArgon2id)
	path := filepath.Join(t.TempDir(), "password.gse")
	db, err := sqlite.Open(path, sqlite.Options{Encryption: &cfg})
	if err != nil {
		t.Fatal(err)
	}
	encryptedExec(t, db, "CREATE TABLE entries(id INTEGER PRIMARY KEY,body TEXT)")
	encryptedExec(t, db, "INSERT INTO entries VALUES(1,'密码')")
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = sqlite.Open(path, sqlite.Options{Encryption: &cfg, ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if got := query(t, db, "SELECT body FROM entries"); !reflect.DeepEqual(got, [][]any{{"密码"}}) {
		t.Fatal(got)
	}
}

func TestEncryptionIncompatibleOptionsBeforeFiles(t *testing.T) {
	cfg := encryptionOptions(t, encryption.RawKey256)
	for _, o := range []sqlite.Options{{WALExclusive: true}, {WALShared: true}, {WALPowersafeOverwrite: true}, {KeepWALOnClose: true}} {
		dir := t.TempDir()
		o.Encryption = &cfg
		if db, err := sqlite.Open(filepath.Join(dir, "bad.gse"), o); err == nil {
			db.Close()
			t.Fatal("incompatible encrypted native WAL options accepted")
		}
		files, err := os.ReadDir(dir)
		if err != nil || len(files) != 0 {
			t.Fatal("validation created files", err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	dir := t.TempDir()
	_, err := sqlite.OpenContext(ctx, filepath.Join(dir, "cancelled.gse"), sqlite.Options{Encryption: &cfg})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 0 {
		t.Fatal("cancelled open created files", err)
	}
}

func TestEncryptionStatementQuotaNormalRollback(t *testing.T) {
	ctx := context.Background()
	cfg := encryptionOptions(t, encryption.RawKey256)
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "limited.gse"), sqlite.Options{Encryption: &cfg, PageSize: 512, MaxTransactionPages: 3})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	encryptedExec(t, db, "CREATE TABLE entries(id INTEGER PRIMARY KEY,body BLOB)")
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, "INSERT INTO entries VALUES(1,?)", []byte{1}); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, "INSERT INTO entries VALUES(2,?)", make([]byte, 20000)); !errors.Is(err, sqlite.ErrMemoryLimit) {
		t.Fatal("missing statement quota error", err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if got := query(t, db, "SELECT id FROM entries"); !reflect.DeepEqual(got, [][]any{{int64(1)}}) {
		t.Fatal(got)
	}
}

func TestEncryptionExtensionBackupRestore(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	cfg := encryptionOptions(t, encryption.RawKey256)
	db, err := sqlite.Open(filepath.Join(dir, "source.gse"), sqlite.Options{Encryption: &cfg})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	encryptedExec(t, db, "CREATE TABLE documents(id INTEGER PRIMARY KEY,body TEXT,embedding VECTOR(3))")
	encryptedExec(t, db, "INSERT INTO documents VALUES(1,'数据库查询','[1,2,3]'),(2,'数据库索引','[3,2,1]')")
	if err = db.CreateGramsIndex(ctx, "grams", "documents", []string{"body"}); err != nil {
		t.Fatal(err)
	}
	if err = db.CreateHNSWIndex(ctx, "hnsw", "documents", "embedding"); err != nil {
		t.Fatal(err)
	}
	backupKey := encryptionOptions(t, encryption.RawKey256)
	archive := filepath.Join(dir, "archive.gse")
	if _, err = db.BackupEncrypted(ctx, archive, backupKey); err != nil {
		t.Fatal(err)
	}
	restoredKey := encryptionOptions(t, encryption.RawKey256)
	path := filepath.Join(dir, "restored.gse")
	if _, err = sqlite.RestoreEncrypted(ctx, archive, path, backupKey, restoredKey); err != nil {
		t.Fatal(err)
	}
	restored, err := sqlite.Open(path, sqlite.Options{Encryption: &restoredKey})
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if got := query(t, restored, "SELECT id FROM documents WHERE body LIKE '%数据库%' ORDER BY id"); !reflect.DeepEqual(got, [][]any{{int64(1)}, {int64(2)}}) {
		t.Fatal(got)
	}
	if got := query(t, restored, "SELECT id FROM documents ORDER BY embedding <-> '[1,2,3]' LIMIT 1"); !reflect.DeepEqual(got, [][]any{{int64(1)}}) {
		t.Fatal(got)
	}
}

func TestEncryptionRetainedGenerationQuota(t *testing.T) {
	ctx := context.Background()
	cfg := encryptionOptions(t, encryption.RawKey256)
	cfg.MaxGenerations = 3
	dir := t.TempDir()
	path := filepath.Join(dir, "bounded.gse")
	db, err := sqlite.Open(path, sqlite.Options{Encryption: &cfg})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	encryptedExec(t, db, "CREATE TABLE entries(id INTEGER PRIMARY KEY)")
	encryptedExec(t, db, "INSERT INTO entries VALUES(1)")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	filesBefore, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, "INSERT INTO entries VALUES(2)"); !errors.Is(err, sqlite.ErrMemoryLimit) {
		t.Fatal("missing publication quota", err)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("quota changed committed root", err)
	}
	filesAfter, err := os.ReadDir(dir)
	if err != nil || len(filesAfter) != len(filesBefore) {
		t.Fatal("quota wrote candidates", err)
	}
	if got := query(t, db, "SELECT id FROM entries"); !reflect.DeepEqual(got, [][]any{{int64(1)}}) {
		t.Fatal(got)
	}
	// Ordinary backup/restore copies the latest valid state into a fresh family;
	// all old history remains in the source, and no automatic deletion occurs.
	backupKey := encryptionOptions(t, encryption.RawKey256)
	archive := filepath.Join(t.TempDir(), "snapshot.gse")
	if _, err = db.BackupEncrypted(ctx, archive, backupKey); err != nil {
		t.Fatal(err)
	}
	restoreKey := encryptionOptions(t, encryption.RawKey256)
	info, err := sqlite.RestoreEncrypted(ctx, archive, filepath.Join(t.TempDir(), "new.gse"), backupKey, restoreKey)
	if err != nil || info.CommitSequence != 1 {
		t.Fatal(info, err)
	}
}
