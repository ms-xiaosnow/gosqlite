package sqlite_test

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Physical fixtures use correct credentials and intact immutable files only.
// Histories are deterministic cooperating normal transactions, never faults.
func TestEncryptionSharedReadersWriterAndStalePromotion(t *testing.T) {
	ctx := context.Background()
	cfg := encryptionOptions(t, encryption.RawKey256)
	cfg.Mode = sqlite.EncryptionWALShared
	path := filepath.Join(t.TempDir(), "shared.gse")
	opts := sqlite.Options{Encryption: &cfg, KeepWALOnClose: true, PageSize: 512}
	a, e := sqlite.Open(path, opts)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	encryptedExec(t, a, "CREATE TABLE t(id INTEGER PRIMARY KEY,v TEXT)")
	encryptedExec(t, a, "INSERT INTO t VALUES(1,'old')")
	b, e := sqlite.Open(path, opts)
	if e != nil {
		t.Fatal(e)
	}
	defer b.Close()
	expect := func(db *sqlite.DB, value string) {
		t.Helper()
		if got := query(t, db, "SELECT v FROM t"); !reflect.DeepEqual(got, [][]any{{value}}) {
			t.Fatalf("want %s got %v", value, got)
		}
	}
	encryptedExec(t, a, "BEGIN")
	expect(a, "old")
	encryptedExec(t, b, "BEGIN IMMEDIATE")
	encryptedExec(t, b, "UPDATE t SET v='new'")
	// C captures the committed root while B owns the writer and dirty overlay.
	ro := opts
	ro.ReadOnly = true
	c, e := sqlite.Open(path, ro)
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	expect(c, "old")
	if _, e = a.Exec(ctx, "UPDATE t SET v='blocked'"); !errors.Is(e, sqlite.ErrBusy) || errors.Is(e, sqlite.ErrBusySnapshot) {
		t.Fatal("writer contention", e)
	}
	encryptedExec(t, b, "COMMIT")
	expect(a, "old")
	expect(c, "new")
	if _, e = a.Exec(ctx, "UPDATE t SET v='stale'"); !errors.Is(e, sqlite.ErrBusySnapshot) || !errors.Is(e, sqlite.ErrBusy) {
		t.Fatal("stale promotion", e)
	}
	expect(a, "old")
	encryptedExec(t, a, "ROLLBACK")
	expect(a, "new")
	// Checkpoint on another connection cannot invalidate a held old selection.
	encryptedExec(t, a, "BEGIN")
	expect(a, "new")
	if e = b.Checkpoint(ctx); e != nil {
		t.Fatal(e)
	}
	expect(a, "new")
	if _, e = a.Exec(ctx, "UPDATE t SET v='precheckpoint'"); !errors.Is(e, sqlite.ErrBusySnapshot) {
		t.Fatal("checkpoint authority", e)
	}
	encryptedExec(t, a, "ROLLBACK")
	info, e := b.VerifyEncrypted(ctx)
	if e != nil || info.StorageLayout != 3 || info.WALSegments != 0 {
		t.Fatal(info, e)
	}
	if p := cfg.KeyProvider.(*encryptionTestProvider); p.acquires.Load() != 3 {
		t.Fatal("refresh reran provider", p.acquires.Load())
	}
}

func TestEncryptionSharedLazySavepointsAndNoopRelease(t *testing.T) {
	ctx := context.Background()
	cfg := encryptionOptions(t, encryption.RawKey256)
	cfg.Mode = sqlite.EncryptionWALShared
	path := filepath.Join(t.TempDir(), "savepoints.gse")
	opts := sqlite.Options{Encryption: &cfg, KeepWALOnClose: true, PageSize: 512}
	a, e := sqlite.Open(path, opts)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	encryptedExec(t, a, "CREATE TABLE t(id INTEGER PRIMARY KEY,v TEXT)")
	encryptedExec(t, a, "INSERT INTO t VALUES(1,'old')")
	b, e := sqlite.Open(path, opts)
	if e != nil {
		t.Fatal(e)
	}
	defer b.Close()
	encryptedExec(t, a, "SAVEPOINT root")
	query(t, a, "SELECT 1")
	encryptedExec(t, b, "UPDATE t SET v='between'")
	if got := query(t, a, "SELECT v FROM t"); !reflect.DeepEqual(got, [][]any{{"between"}}) {
		t.Fatal(got)
	}
	encryptedExec(t, a, "UPDATE t SET v='outer'")
	encryptedExec(t, a, "SAVEPOINT nested")
	encryptedExec(t, a, "UPDATE t SET v='inner'")
	encryptedExec(t, a, "ROLLBACK TO nested")
	if _, e = b.Exec(ctx, "BEGIN IMMEDIATE"); !errors.Is(e, sqlite.ErrBusy) {
		t.Fatal(e)
	}
	if got := query(t, a, "SELECT v FROM t"); !reflect.DeepEqual(got, [][]any{{"outer"}}) {
		t.Fatal(got)
	}
	if _, e = a.Exec(ctx, "INSERT INTO t VALUES(1,'duplicate')"); e == nil {
		t.Fatal("expected statement constraint")
	}
	encryptedExec(t, a, "RELEASE root")
	encryptedExec(t, b, "BEGIN EXCLUSIVE")
	encryptedExec(t, b, "COMMIT")
	encryptedExec(t, a, "BEGIN IMMEDIATE")
	encryptedExec(t, a, "ROLLBACK")
	if got := query(t, b, "SELECT v FROM t"); !reflect.DeepEqual(got, [][]any{{"outer"}}) {
		t.Fatal(got)
	}
}

func TestEncryptionSharedQuiescentRekeyAndLeaseRetention(t *testing.T) {
	ctx := context.Background()
	cfg := encryptionOptions(t, encryption.RawKey256)
	cfg.Mode = sqlite.EncryptionWALShared
	next := encryptionOptions(t, encryption.RawKey256)
	next.Mode = sqlite.EncryptionWALShared
	path := filepath.Join(t.TempDir(), "rekey.gse")
	opts := sqlite.Options{Encryption: &cfg, KeepWALOnClose: true, PageSize: 512}
	a, e := sqlite.Open(path, opts)
	if e != nil {
		t.Fatal(e)
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
	encryptedExec(t, a, "INSERT INTO t VALUES(1,'retained')")
	ro := opts
	ro.ReadOnly = true
	firstB, e := sqlite.Open(path, ro)
	if e != nil {
		t.Fatal(e)
	}
	firstBCloseAttempted := false
	t.Cleanup(func() {
		if !firstBCloseAttempted {
			firstBCloseAttempted = true
			if closeErr := firstB.Close(); closeErr != nil {
				t.Error("cleanup first b:", closeErr)
			}
		}
	})
	if e = a.Rekey(ctx, next); !errors.Is(e, sqlite.ErrBusy) {
		t.Fatal("idle readonly must block rekey", e)
	}
	firstBCloseAttempted = true
	if e = firstB.Close(); e != nil {
		t.Fatal(e)
	}
	// A failed lifecycle conversion must retain A's shared lifecycle lease.
	legacy := cfg
	legacy.Mode = sqlite.EncryptionWAL
	x, legacyErr := sqlite.Open(path, sqlite.Options{Encryption: &legacy})
	xCloseAttempted := false
	if x != nil {
		t.Cleanup(func() {
			if !xCloseAttempted {
				xCloseAttempted = true
				if closeErr := x.Close(); closeErr != nil {
					t.Error("cleanup unexpected legacy owner:", closeErr)
				}
			}
		})
	}
	if !errors.Is(legacyErr, sqlite.ErrBusy) {
		if x != nil {
			xCloseAttempted = true
			if closeErr := x.Close(); closeErr != nil {
				t.Error("close unexpected legacy owner:", closeErr)
			}
		}
		t.Fatal("failed rekey dropped lifecycle", legacyErr)
	}
	if got := query(t, a, "SELECT v FROM t"); !reflect.DeepEqual(got, [][]any{{"retained"}}) {
		t.Fatal(got)
	}
	if e = a.Rekey(ctx, next); e != nil {
		t.Fatal(e)
	}
	aCloseAttempted = true
	if e = a.Close(); e != nil {
		t.Fatal(e)
	}
	// Only the new correct provider is used for the ordinary reopen.
	reopenedB, e := sqlite.Open(path, sqlite.Options{Encryption: &next, ReadOnly: true})
	if e != nil {
		t.Fatal(e)
	}
	reopenedBCloseAttempted := false
	t.Cleanup(func() {
		if !reopenedBCloseAttempted {
			reopenedBCloseAttempted = true
			if closeErr := reopenedB.Close(); closeErr != nil {
				t.Error("cleanup reopened b:", closeErr)
			}
		}
	})
	if got := query(t, reopenedB, "SELECT v FROM t"); !reflect.DeepEqual(got, [][]any{{"retained"}}) {
		t.Fatal(got)
	}
	i, e := reopenedB.VerifyEncrypted(ctx)
	if e != nil || i.Mode != sqlite.EncryptionWALShared || i.WALSegments != 0 {
		t.Fatal(i, e)
	}
	reopenedBCloseAttempted = true
	if e = reopenedB.Close(); e != nil {
		t.Fatal(e)
	}
}

func TestEncryptionSharedExplicitUpgradeCopiesAndRestore(t *testing.T) {
	ctx := context.Background()
	for _, mode := range []sqlite.EncryptionMode{sqlite.EncryptionSnapshot, sqlite.EncryptionWAL} {
		t.Run(string(rune('0'+mode)), func(t *testing.T) {
			cfg := encryptionOptions(t, encryption.RawKey256)
			cfg.Mode = mode
			dir := t.TempDir()
			path := filepath.Join(dir, "upgrade.gse")
			a, e := sqlite.Open(path, sqlite.Options{Encryption: &cfg, PageSize: 512})
			if e != nil {
				t.Fatal(e)
			}
			defer a.Close()
			encryptedExec(t, a, "CREATE TABLE t(id INTEGER PRIMARY KEY,v TEXT)")
			encryptedExec(t, a, "INSERT INTO t VALUES(1,'before')")
			old, e := a.VerifyEncrypted(ctx)
			if e != nil {
				t.Fatal(e)
			}
			if e = a.UpgradeEncryptedWALShared(ctx); e != nil {
				t.Fatal(e)
			}
			cfg.Mode = sqlite.EncryptionWALShared
			b, e := sqlite.Open(path, sqlite.Options{Encryption: &cfg, KeepWALOnClose: true})
			if e != nil {
				t.Fatal(e)
			}
			defer b.Close()
			encryptedExec(t, b, "UPDATE t SET v='latest'")
			now, e := a.VerifyEncrypted(ctx)
			if e != nil || now.DatabaseID != old.DatabaseID || now.StorageLayout != 3 || now.CommitSequence <= old.CommitSequence {
				t.Fatal(now, e)
			}
			archiveCfg := encryptionOptions(t, encryption.RawKey256)
			archive := filepath.Join(dir, "archive.gse")
			if _, e = a.BackupEncrypted(ctx, archive, archiveCfg); e != nil {
				t.Fatal(e)
			}
			target := encryptionOptions(t, encryption.RawKey256)
			target.Mode = sqlite.EncryptionWALShared
			restored := filepath.Join(dir, "restored.gse")
			if _, e = sqlite.RestoreEncrypted(ctx, archive, restored, archiveCfg, target); e != nil {
				t.Fatal(e)
			}
			r, e := sqlite.Open(restored, sqlite.Options{Encryption: &target, ReadOnly: true})
			if e != nil {
				t.Fatal(e)
			}
			if got := query(t, r, "SELECT v FROM t"); !reflect.DeepEqual(got, [][]any{{"latest"}}) {
				t.Fatal(got)
			}
			if e = r.Close(); e != nil {
				t.Fatal(e)
			}
			plain := filepath.Join(dir, "explicit-export.db")
			if _, e = a.ExportPlaintext(ctx, plain); e != nil {
				t.Fatal(e)
			}
			imported := filepath.Join(dir, "import.gse")
			if _, e = sqlite.ImportEncrypted(ctx, plain, imported, sqlite.Options{}, target); e != nil {
				t.Fatal(e)
			}
			r, e = sqlite.Open(imported, sqlite.Options{Encryption: &target, ReadOnly: true})
			if e != nil {
				t.Fatal(e)
			}
			if got := query(t, r, "SELECT v FROM t"); !reflect.DeepEqual(got, [][]any{{"latest"}}) {
				t.Fatal(got)
			}
			if e = r.Close(); e != nil {
				t.Fatal(e)
			}
			if e = b.Close(); e != nil {
				t.Fatal(e)
			}
			if e = a.Close(); e != nil {
				t.Fatal(e)
			}
			mismatch := cfg
			mismatch.Mode = mode
			if r, e = sqlite.Open(path, sqlite.Options{Encryption: &mismatch, ReadOnly: true}); !errors.Is(e, sqlite.ErrUnsupported) {
				if r != nil {
					r.Close()
				}
				t.Fatal("mode mismatch", e)
			}
		})
	}
}

func TestEncryptionSharedCloseBusyAndPasswordRefresh(t *testing.T) {
	cfg := encryptionOptions(t, encryption.PasswordArgon2id)
	cfg.Mode = sqlite.EncryptionWALShared
	path := filepath.Join(t.TempDir(), "close.gse")
	a, e := sqlite.Open(path, sqlite.Options{Encryption: &cfg, PageSize: 512})
	if e != nil {
		t.Fatal(e)
	}
	encryptedExec(t, a, "CREATE TABLE t(id INTEGER PRIMARY KEY,v TEXT)")
	encryptedExec(t, a, "INSERT INTO t VALUES(1,'committed')")
	b, e := sqlite.Open(path, sqlite.Options{Encryption: &cfg, KeepWALOnClose: true})
	if e != nil {
		t.Fatal(e)
	}
	defer b.Close()
	encryptedExec(t, b, "BEGIN IMMEDIATE")
	if e = a.Close(); !errors.Is(e, sqlite.ErrBusy) {
		t.Fatal("busy optional checkpoint", e)
	}
	encryptedExec(t, b, "ROLLBACK")
	for i := 0; i < 3; i++ {
		if got := query(t, b, "SELECT v FROM t"); !reflect.DeepEqual(got, [][]any{{"committed"}}) {
			t.Fatal(got)
		}
	}
	if p := cfg.KeyProvider.(*encryptionTestProvider); p.acquires.Load() != 2 {
		t.Fatal("repeated KDF/provider", p.acquires.Load())
	}
	if e = b.Close(); e != nil {
		t.Fatal(e)
	}
	a, e = sqlite.Open(path, sqlite.Options{Encryption: &cfg, ReadOnly: true})
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	if got := query(t, a, "SELECT v FROM t"); !reflect.DeepEqual(got, [][]any{{"committed"}}) {
		t.Fatal(got)
	}
}

func TestEncryptionSharedPromotionQuotaPreservesReadLease(t *testing.T) {
	ctx := context.Background()
	cfg := encryptionOptions(t, encryption.RawKey256)
	cfg.Mode = sqlite.EncryptionWALShared
	path := filepath.Join(t.TempDir(), "quota.gse")
	opts := sqlite.Options{Encryption: &cfg, KeepWALOnClose: true, PageSize: 512}
	init, e := sqlite.Open(path, opts)
	if e != nil {
		t.Fatal(e)
	}
	encryptedExec(t, init, "CREATE TABLE t(id INTEGER PRIMARY KEY,v TEXT)")
	encryptedExec(t, init, "INSERT INTO t VALUES(1,'old')")
	if e = init.Checkpoint(ctx); e != nil {
		t.Fatal(e)
	}
	if e = init.Close(); e != nil {
		t.Fatal(e)
	}
	lower := cfg
	lower.WALLimits.MaxOpenFiles = 12
	lowopts := opts
	lowopts.Encryption = &lower
	a, e := sqlite.Open(path, lowopts)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	b, e := sqlite.Open(path, opts)
	if e != nil {
		t.Fatal(e)
	}
	defer b.Close()
	encryptedExec(t, a, "BEGIN")
	if got := query(t, a, "SELECT v FROM t"); !reflect.DeepEqual(got, [][]any{{"old"}}) {
		t.Fatal(got)
	}
	for _, value := range []string{"one", "two", "three"} {
		encryptedExec(t, b, "UPDATE t SET v=?", value)
	}
	// The new graph alone fits 12 descriptors; retaining both authorities needs
	// 13. Admission must fail before swapping out the exposed read selection.
	if _, e = a.Exec(ctx, "UPDATE t SET v='never-admitted'"); !errors.Is(e, sqlite.ErrMemoryLimit) {
		t.Fatal("combined admission", e)
	}
	if got := query(t, a, "SELECT v FROM t"); !reflect.DeepEqual(got, [][]any{{"old"}}) {
		t.Fatal("quota lost read lease", got)
	}
	encryptedExec(t, b, "BEGIN IMMEDIATE")
	encryptedExec(t, b, "ROLLBACK")
	encryptedExec(t, a, "ROLLBACK")
}

func TestEncryptionSharedGramsHNSWIVFSavepointAndCopy(t *testing.T) {
	ctx := context.Background()
	cfg := encryptionOptions(t, encryption.RawKey256)
	cfg.Mode = sqlite.EncryptionWALShared
	dir := t.TempDir()
	path := filepath.Join(dir, "extensions.gse")
	opts := sqlite.Options{Encryption: &cfg, KeepWALOnClose: true}
	a, e := sqlite.Open(path, opts)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	encryptedExec(t, a, "CREATE TABLE items(id INTEGER PRIMARY KEY,v VECTOR(2),note TEXT)")
	encryptedExec(t, a, "INSERT INTO items VALUES(1,'[1,1]','数据库一'),(2,'[2,2]','数据库二'),(3,'[3,3]','数据库三'),(4,'[4,4]','数据库四')")
	encryptedExec(t, a, "CREATE INDEX ordinary_note ON items(note)")
	if e = a.CreateGramsIndex(ctx, "grams", "items", []string{"note"}); e != nil {
		t.Fatal(e)
	}
	if e = a.CreateHNSWIndex(ctx, "hnsw", "items", "v"); e != nil {
		t.Fatal(e)
	}
	if e = a.CreateIVFFlatIndex(ctx, "ivf", "items", "v", sqlite.IVFFlatIndexOptions{Lists: 2}); e != nil {
		t.Fatal(e)
	}
	b, e := sqlite.Open(path, opts)
	if e != nil {
		t.Fatal(e)
	}
	defer b.Close()
	encryptedExec(t, a, "BEGIN")
	original := savepointTableImages(t, a)
	encryptedExec(t, b, "SAVEPOINT whole")
	encryptedExec(t, b, "SAVEPOINT changes")
	encryptedExec(t, b, "UPDATE items SET id=9,v='[9,9]',note='暂时数据库' WHERE id=1")
	encryptedExec(t, b, "DELETE FROM items WHERE id=2")
	for _, name := range []string{"grams", "hnsw", "ivf"} {
		if e = b.CheckExtensionIndex(ctx, name); e != nil {
			t.Fatal(name, e)
		}
	}
	encryptedExec(t, b, "ROLLBACK TO changes")
	if got := savepointTableImages(t, b); !reflect.DeepEqual(got, original) {
		t.Fatal("extension state did not roll back")
	}
	encryptedExec(t, b, "UPDATE items SET note='committed数据库',v='[8,8]' WHERE id=3")
	encryptedExec(t, b, "RELEASE whole")
	if e = b.Checkpoint(ctx); e != nil {
		t.Fatal(e)
	}
	if got := savepointTableImages(t, a); !reflect.DeepEqual(got, original) {
		t.Fatal("old indexed snapshot changed")
	}
	encryptedExec(t, a, "ROLLBACK")
	for _, name := range []string{"grams", "hnsw", "ivf"} {
		if e = a.CheckExtensionIndex(ctx, name); e != nil {
			t.Fatal(name, e)
		}
	}
	archiveKey := encryptionOptions(t, encryption.RawKey256)
	archive := filepath.Join(dir, "archive.gse")
	if _, e = a.BackupEncrypted(ctx, archive, archiveKey); e != nil {
		t.Fatal(e)
	}
	target := encryptionOptions(t, encryption.RawKey256)
	target.Mode = sqlite.EncryptionWALShared
	restored := filepath.Join(dir, "restored.gse")
	if _, e = sqlite.RestoreEncrypted(ctx, archive, restored, archiveKey, target); e != nil {
		t.Fatal(e)
	}
	copy, e := sqlite.Open(restored, sqlite.Options{Encryption: &target, ReadOnly: true})
	if e != nil {
		t.Fatal(e)
	}
	defer copy.Close()
	for _, name := range []string{"grams", "hnsw", "ivf"} {
		if e = copy.CheckExtensionIndex(ctx, name); e != nil {
			t.Fatal(name, e)
		}
	}
	if got, want := savepointTableImages(t, copy), savepointTableImages(t, a); !reflect.DeepEqual(got, want) {
		t.Fatal("full-head copy changed ordinary or extension tables")
	}
}
