package sqlite_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// These independent review workflows use intact normally-created files and
// correct credentials only. Negative cases are ordinary API/quota preflight,
// with no changed files, wrong keys, faults, crashes or recovery injection.
func walReviewFileHashes(t *testing.T, dir string) map[string][32]byte {
	t.Helper()
	list, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][32]byte{}
	for _, f := range list {
		if f.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, f.Name()))
		if err != nil {
			t.Fatal(err)
		}
		out[f.Name()] = sha256.Sum256(b)
	}
	return out
}
func walReviewCheckData(t *testing.T, db *sqlite.DB, blob []byte) {
	t.Helper()
	ctx := context.Background()
	if got := query(t, db, "SELECT id,body,payload,vector_to_text(v) FROM entries WHERE id=4"); !reflect.DeepEqual(got, [][]any{{int64(4), "wal-review-数据库-最新", blob, "[99,4]"}}) {
		t.Fatalf("head data mismatch: %v", got)
	}
	if got := query(t, db, "SELECT id FROM entries WHERE body LIKE '%最新%'"); !reflect.DeepEqual(got, [][]any{{int64(4)}}) {
		t.Fatalf("grams head mismatch: %v", got)
	}
	for _, name := range []string{"body_grams", "near_hnsw", "near_ivf"} {
		if err := db.CheckExtensionIndex(ctx, name); err != nil {
			t.Fatalf("head index %s: %v", name, err)
		}
	}
	if got := query(t, db, "SELECT id FROM entries WHERE body='wal-review-数据库-最新'"); !reflect.DeepEqual(got, [][]any{{int64(4)}}) {
		t.Fatalf("ordinary index head mismatch: %v", got)
	}
}
func TestEncryptionWALReviewHeadCopiesAndSparsePublication(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	path := filepath.Join(dir, "main.gse")
	cfg := encryptionOptions(t, encryption.RawKey256)
	cfg.Mode = sqlite.EncryptionWAL
	db, err := sqlite.Open(path, sqlite.Options{Encryption: &cfg, PageSize: 512, KeepWALOnClose: true})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if db != nil {
			_ = db.Close()
		}
	}()
	encryptedExec(t, db, "CREATE TABLE entries(id INTEGER PRIMARY KEY,body TEXT,payload BLOB,v VECTOR(2))")
	encryptedExec(t, db, "CREATE TABLE filler(id INTEGER PRIMARY KEY,payload BLOB)")
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 12; i++ {
		if _, err = tx.Exec(ctx, "INSERT INTO entries VALUES(?,?,?,?)", i, fmt.Sprintf("数据库-%d", i), []byte{byte(i)}, fmt.Sprintf("[%d,%d]", i, i%3)); err != nil {
			t.Fatal(err)
		}
	}
	for i := 1; i <= 60; i++ {
		if _, err = tx.Exec(ctx, "INSERT INTO filler VALUES(?,?)", i, bytes.Repeat([]byte{byte(i)}, 1000)); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	encryptedExec(t, db, "CREATE INDEX body_plain ON entries(body)")
	if err = db.CreateGramsIndex(ctx, "body_grams", "entries", []string{"body"}); err != nil {
		t.Fatal(err)
	}
	if err = db.CreateHNSWIndex(ctx, "near_hnsw", "entries", "v"); err != nil {
		t.Fatal(err)
	}
	if err = db.CreateIVFFlatIndex(ctx, "near_ivf", "entries", "v", sqlite.IVFFlatIndexOptions{Lists: 2, TrainingSamples: 12}); err != nil {
		t.Fatal(err)
	}
	if err = db.Checkpoint(ctx); err != nil {
		t.Fatal(err)
	}
	before, err := db.VerifyEncrypted(ctx)
	if err != nil {
		t.Fatal(err)
	}
	old := walReviewFileHashes(t, dir)
	blob := bytes.Repeat([]byte{199, 42, 0, 71}, 300)
	tx, err = db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, body := range []string{"intermediate-one", "intermediate-two", "wal-review-数据库-最新"} {
		if _, err = tx.Exec(ctx, "UPDATE entries SET body=?,payload=?,v='[99,4]' WHERE id=4", body, blob); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	after, err := db.VerifyEncrypted(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if after.WALSegments != 1 || after.WALFrames == 0 || after.WALFrames >= uint64(after.PageCount)/2 || after.CommitSequence != before.CommitSequence+1 || after.HeadCommitID == before.HeadCommitID {
		t.Fatalf("expected sparse final-page publication: before=%+v after=%+v", before, after)
	}
	if after.WALBytes != 928+after.WALFrames*(uint64(after.PageSize)+44) {
		t.Fatal("segment byte extent does not reflect dirty frame count")
	}
	newHashes := walReviewFileHashes(t, dir)
	for name, digest := range old {
		if strings.HasSuffix(name, ".base") && newHashes[name] != digest {
			t.Fatal("existing base changed", name)
		}
	}
	oldBases, newBases := 0, 0
	for name := range old {
		if strings.HasSuffix(name, ".base") {
			oldBases++
		}
	}
	for name := range newHashes {
		if strings.HasSuffix(name, ".base") {
			newBases++
		}
	}
	if oldBases != newBases {
		t.Fatal("dirty WAL commit emitted a full base")
	}
	walReviewCheckData(t, db, blob)
	// Portable archive and explicit export both consume the uncheckpointed head.
	archiveKey := encryptionOptions(t, encryption.RawKey256)
	archive := filepath.Join(t.TempDir(), "archive.gse")
	if _, err = db.BackupEncrypted(ctx, archive, archiveKey); err != nil {
		t.Fatal(err)
	}
	restoredKey := encryptionOptions(t, encryption.RawKey256)
	restoredKey.Mode = sqlite.EncryptionWAL
	restored := filepath.Join(t.TempDir(), "restored.gse")
	restoredInfo, err := sqlite.RestoreEncrypted(ctx, archive, restored, archiveKey, restoredKey)
	if err != nil {
		t.Fatal(err)
	}
	if restoredInfo.WALSegments != 0 || restoredInfo.StorageLayout != 2 || restoredInfo.DatabaseID == after.DatabaseID {
		t.Fatal("restored target is not fresh zero-segment WAL", restoredInfo)
	}
	copyDB, err := sqlite.Open(restored, sqlite.Options{Encryption: &restoredKey, ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	walReviewCheckData(t, copyDB, blob)
	if err = copyDB.Close(); err != nil {
		t.Fatal(err)
	}
	plain := filepath.Join(t.TempDir(), "plain.db")
	if _, err = db.ExportPlaintext(ctx, plain); err != nil {
		t.Fatal(err)
	}
	plainDB, err := sqlite.Open(plain, sqlite.Options{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	walReviewCheckData(t, plainDB, blob)
	if err = plainDB.Close(); err != nil {
		t.Fatal(err)
	}
	importedKey := encryptionOptions(t, encryption.RawKey256)
	importedKey.Mode = sqlite.EncryptionWAL
	imported := filepath.Join(t.TempDir(), "imported.gse")
	if _, err = sqlite.ImportEncrypted(ctx, plain, imported, sqlite.Options{}, importedKey); err != nil {
		t.Fatal(err)
	}
	copyDB, err = sqlite.Open(imported, sqlite.Options{Encryption: &importedKey, ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	walReviewCheckData(t, copyDB, blob)
	if err = copyDB.Close(); err != nil {
		t.Fatal(err)
	}
	// No-op and normal rollback leave the entire family byte-for-byte unchanged.
	unchanged := walReviewFileHashes(t, dir)
	tx, err = db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, "DELETE FROM entries WHERE id=4"); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	tx, err = db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(unchanged, walReviewFileHashes(t, dir)) {
		t.Fatal("rollback/no-op emitted physical changes")
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db = nil
	db, err = sqlite.Open(path, sqlite.Options{Encryption: &cfg, KeepWALOnClose: true, NoCreate: true})
	if err != nil {
		t.Fatal(err)
	}
	walReviewCheckData(t, db, blob)
	rekey := encryptionOptions(t, encryption.RawKey256)
	rekey.Mode = sqlite.EncryptionWAL
	if err = db.Rekey(ctx, rekey); err != nil {
		t.Fatal(err)
	}
	info, err := db.VerifyEncrypted(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if info.WALSegments != 0 || info.DatabaseID != after.DatabaseID || info.CommitSequence != after.CommitSequence+1 {
		t.Fatal("rekey did not materialize complete head", info)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db = nil
	db, err = sqlite.Open(path, sqlite.Options{Encryption: &rekey, ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	walReviewCheckData(t, db, blob)
	t.Logf("Sparse evidence: %d-page head, %d final dirty frames, %d WAL bytes, unchanged %d bases; full index/copy/rekey/head round trips passed", after.PageCount, after.WALFrames, after.WALBytes, oldBases)
}
func TestEncryptionWALReviewExplicitUpgradeAndCloseQuota(t *testing.T) {
	ctx := context.Background()
	cfg := encryptionOptions(t, encryption.RawKey256)
	dir := t.TempDir()
	path := filepath.Join(dir, "upgrade.gse")
	db, err := sqlite.Open(path, sqlite.Options{Encryption: &cfg, PageSize: 512})
	if err != nil {
		t.Fatal(err)
	}
	encryptedExec(t, db, "CREATE TABLE entries(id INTEGER PRIMARY KEY,body TEXT)")
	encryptedExec(t, db, "INSERT INTO entries VALUES(1,'snapshot stays readable')")
	info, err := db.VerifyEncrypted(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if info.StorageLayout != 1 || info.Mode != sqlite.EncryptionSnapshot {
		t.Fatal("zero mode is not preserved snapshot")
	}
	if err = db.UpgradeEncryptedWAL(ctx); err != nil {
		t.Fatal(err)
	}
	upgraded, err := db.VerifyEncrypted(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if upgraded.StorageLayout != 2 || upgraded.WALSegments != 0 || upgraded.DatabaseID != info.DatabaseID || upgraded.CommitSequence != info.CommitSequence+1 {
		t.Fatal("explicit upgrade identity", upgraded)
	}
	if err = db.Checkpoint(ctx); err != nil {
		t.Fatal(err)
	}
	same, err := db.VerifyEncrypted(ctx)
	if err != nil || same.CommitSequence != upgraded.CommitSequence {
		t.Fatal("empty checkpoint consumed generation", err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	untouched := walReviewFileHashes(t, dir)
	if mismatch, err := sqlite.Open(path, sqlite.Options{Encryption: &cfg, ReadOnly: true}); err == nil {
		mismatch.Close()
		t.Fatal("snapshot mode silently opened WAL")
	}
	if !reflect.DeepEqual(untouched, walReviewFileHashes(t, dir)) {
		t.Fatal("mode mismatch changed family")
	}
	cfg.Mode = sqlite.EncryptionWAL
	db, err = sqlite.Open(path, sqlite.Options{Encryption: &cfg, ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	if got := query(t, db, "SELECT body FROM entries"); !reflect.DeepEqual(got, [][]any{{"snapshot stays readable"}}) {
		t.Fatal(got)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	limited := encryptionOptions(t, encryption.RawKey256)
	limited.Mode = sqlite.EncryptionWAL
	limited.MaxGenerations = 3
	p := filepath.Join(t.TempDir(), "close.gse")
	db, err = sqlite.Open(p, sqlite.Options{Encryption: &limited})
	if err != nil {
		t.Fatal(err)
	}
	encryptedExec(t, db, "CREATE TABLE entries(id INTEGER PRIMARY KEY)")
	encryptedExec(t, db, "INSERT INTO entries VALUES(9)")
	if err = db.Close(); !errors.Is(err, sqlite.ErrMemoryLimit) {
		t.Fatalf("close did not return checkpoint quota: %v", err)
	}
	db, err = sqlite.Open(p, sqlite.Options{Encryption: &limited, ReadOnly: true})
	if err != nil {
		t.Fatalf("failed close did not release lease: %v", err)
	}
	defer db.Close()
	if got := query(t, db, "SELECT id FROM entries"); !reflect.DeepEqual(got, [][]any{{int64(9)}}) {
		t.Fatal("committed head lost after close quota", got)
	}
}
func TestEncryptionWALReviewPreflightBeforeFiles(t *testing.T) {
	cfg := encryptionOptions(t, encryption.RawKey256)
	cfg.Mode = sqlite.EncryptionWAL
	for name, limits := range map[string]sqlite.EncryptionWALLimits{"root": {MaxRootBytes: 1063}, "memory": {MaxMemoryBytes: 1}, "descriptors": {MaxOpenFiles: 8}, "work": {MaxOperationBytes: 1}, "scan": {MaxScanBytes: 1}} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			opt := cfg
			opt.WALLimits = limits
			if db, err := sqlite.Open(filepath.Join(dir, "limited.gse"), sqlite.Options{Encryption: &opt}); err == nil {
				db.Close()
				t.Fatal("impossible initial resource budget accepted")
			}
			entries, err := os.ReadDir(dir)
			if err != nil || len(entries) != 0 {
				t.Fatal("preflight left family files", len(entries), err)
			}
		})
	}
}
