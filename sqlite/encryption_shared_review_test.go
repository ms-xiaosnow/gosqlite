package sqlite_test

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/internal/encstore"
	"github.com/ms-xiaosnow/gosqlite/internal/pager"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Review physical fixtures remain intact and use correct credentials. The
// callback barriers below interleave only ordinary cooperating API operations.
func TestReviewEncryptionSharedValidationAndCopyPinOneHead(t *testing.T) {
	ctx := context.Background()
	for _, archive := range []bool{false, true} {
		name := "plaintext"
		if archive {
			name = "archive"
		}
		t.Run(name, func(t *testing.T) {
			cfg := encryptionOptions(t, encryption.RawKey256)
			cfg.Mode = sqlite.EncryptionWALShared
			dir := t.TempDir()
			path := filepath.Join(dir, "source.gse")
			db, err := sqlite.Open(path, sqlite.Options{Encryption: &cfg, PageSize: 512, KeepWALOnClose: true})
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			encryptedExec(t, db, "CREATE TABLE t(id INTEGER PRIMARY KEY,v TEXT)")
			encryptedExec(t, db, "CREATE INDEX tv ON t(v)")
			encryptedExec(t, db, "INSERT INTO t VALUES(1,'before')")
			stcfg := encstore.Options{Mode: encstore.WALShared, KeyProvider: cfg.KeyProvider, CredentialKind: cfg.CredentialKind}
			source, err := pager.OpenEncryptedSession(ctx, path, pager.Options{PageSize: 512, KeepWALOnClose: true}, stcfg, false)
			if err != nil {
				t.Fatal(err)
			}
			defer source.Close()
			var selected pager.Header
			calls := 0
			validator := func(ctx context.Context, r pager.SnapshotReader) error {
				calls++
				selected = r.Header()
				p, err := r.ReadPage(ctx, 1)
				if err != nil {
					return err
				}
				// Grow and publish B while A retains the validation/copy selection.
				encryptedExec(t, db, "UPDATE t SET v='after' WHERE id=1")
				encryptedExec(t, db, "INSERT INTO t VALUES(2,?)", string(bytes.Repeat([]byte{'z'}, 4096)))
				if err = db.Checkpoint(ctx); err != nil {
					return err
				}
				again, err := r.ReadPage(ctx, 1)
				if err != nil {
					return err
				}
				if !bytes.Equal(p, again) || r.Header() != selected {
					t.Fatal("validation view changed during external commit/checkpoint")
				}
				return nil
			}
			output := filepath.Join(dir, "copy.sqlite")
			if archive {
				key := encryptionOptions(t, encryption.RawKey256)
				target := encstore.Options{KeyProvider: key.KeyProvider, CredentialKind: key.CredentialKind}
				prepared, err := encstore.Prepare(ctx, output, target, encryption.OperationBackupTarget, [16]byte{})
				if err != nil {
					t.Fatal(err)
				}
				defer prepared.Close()
				info, err := source.BackupPrepared(ctx, output, prepared, validator)
				if err != nil {
					t.Fatal(err)
				}
				if info.PageCount != selected.PageCount || info.PageSize != selected.PageSize {
					t.Fatal("archive metadata differs from validated head")
				}
				restored := filepath.Join(dir, "restore.gse")
				restoredKey := encryptionOptions(t, encryption.RawKey256)
				restoredKey.Mode = sqlite.EncryptionWALShared
				if _, err = sqlite.RestoreEncrypted(ctx, output, restored, key, restoredKey); err != nil {
					t.Fatal(err)
				}
				copyDB, err := sqlite.Open(restored, sqlite.Options{Encryption: &restoredKey, KeepWALOnClose: true, ReadOnly: true})
				if err != nil {
					t.Fatal(err)
				}
				defer copyDB.Close()
				if got := query(t, copyDB, "SELECT id,v FROM t ORDER BY id"); !reflect.DeepEqual(got, [][]any{{int64(1), "before"}}) {
					t.Fatal("archive copied a different head", got)
				}
			} else {
				if err = source.ExportPlaintext(ctx, output, validator); err != nil {
					t.Fatal(err)
				}
				copyDB, err := sqlite.Open(output, sqlite.Options{ReadOnly: true, NoCreate: true})
				if err != nil {
					t.Fatal(err)
				}
				defer copyDB.Close()
				if got := query(t, copyDB, "SELECT id,v FROM t ORDER BY id"); !reflect.DeepEqual(got, [][]any{{int64(1), "before"}}) {
					t.Fatal("export copied a different head", got)
				}
				if source.Header() != selected {
					t.Fatal("copy header refreshed after validation")
				}
			}
			if calls != 1 {
				t.Fatal("validator was not single-shot", calls)
			}
			if got := query(t, db, "SELECT v FROM t WHERE id=1"); !reflect.DeepEqual(got, [][]any{{"after"}}) {
				t.Fatal("concurrent writer did not publish", got)
			}
			if err = source.Verify(ctx); err != nil {
				t.Fatal("copy retained local lease", err)
			}
			if source.Header().PageCount <= selected.PageCount {
				t.Fatal("next operation did not refresh growth")
			}
		})
	}
}

func TestReviewEncryptionSharedRowsPinCatalogAcrossCheckpoint(t *testing.T) {
	ctx := context.Background()
	cfg := encryptionOptions(t, encryption.RawKey256)
	cfg.Mode = sqlite.EncryptionWALShared
	path := filepath.Join(t.TempDir(), "rows.gse")
	opts := sqlite.Options{Encryption: &cfg, KeepWALOnClose: true, PageSize: 512}
	a, err := sqlite.Open(path, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	encryptedExec(t, a, "CREATE TABLE t(id INTEGER PRIMARY KEY,v TEXT)")
	encryptedExec(t, a, "CREATE INDEX tv ON t(v)")
	encryptedExec(t, a, "INSERT INTO t VALUES(1,'alpha'),(2,'beta'),(3,'gamma')")
	b, err := sqlite.Open(path, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	rows, err := a.Query(ctx, "SELECT id,v FROM t ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatal(rows.Err())
	}
	if got := rows.Values(); !reflect.DeepEqual(got, []any{int64(1), "alpha"}) {
		t.Fatal(got)
	}
	encryptedExec(t, b, "UPDATE t SET v='changed' WHERE id=2")
	encryptedExec(t, b, "DELETE FROM t WHERE id=3")
	encryptedExec(t, b, "CREATE TABLE later(id INTEGER PRIMARY KEY)")
	if err = b.Checkpoint(ctx); err != nil {
		t.Fatal(err)
	}
	var remaining [][]any
	for rows.Next() {
		remaining = append(remaining, rows.Values())
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(remaining, [][]any{{int64(2), "beta"}, {int64(3), "gamma"}}) {
		t.Fatal("old Rows changed", remaining)
	}
	if err = rows.Close(); err != nil {
		t.Fatal(err)
	}
	if got := query(t, a, "SELECT id,v FROM t ORDER BY id"); !reflect.DeepEqual(got, [][]any{{int64(1), "alpha"}, {int64(2), "changed"}}) {
		t.Fatal("fresh Rows stale", got)
	}
	if _, err = a.Exec(ctx, "INSERT INTO later VALUES(7)"); err != nil {
		t.Fatal("fresh catalog stale", err)
	}
}

func TestReviewEncryptionSharedCancelledAdmissionReleasesWriter(t *testing.T) {
	ctx := context.Background()
	cfg := encryptionOptions(t, encryption.RawKey256)
	cfg.Mode = sqlite.EncryptionWALShared
	path := filepath.Join(t.TempDir(), "cancel.gse")
	opts := sqlite.Options{Encryption: &cfg, KeepWALOnClose: true, PageSize: 512}
	a, err := sqlite.Open(path, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	encryptedExec(t, a, "CREATE TABLE t(id INTEGER PRIMARY KEY,v TEXT)")
	b, err := sqlite.Open(path, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = a.Begin(cancelled, sqlite.TxOptions{Immediate: true}); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled admission", err)
	}
	encryptedExec(t, b, "BEGIN IMMEDIATE")
	encryptedExec(t, b, "ROLLBACK")
	encryptedExec(t, a, "BEGIN IMMEDIATE")
	if _, err = a.Exec(cancelled, "INSERT INTO t VALUES(1,'cancelled')"); !errors.Is(err, context.Canceled) {
		t.Fatal("cancelled statement", err)
	}
	if _, err = b.Exec(ctx, "BEGIN IMMEDIATE"); !errors.Is(err, sqlite.ErrBusy) {
		t.Fatal("cancelled statement released outer writer", err)
	}
	encryptedExec(t, a, "ROLLBACK")
	encryptedExec(t, b, "BEGIN IMMEDIATE")
	encryptedExec(t, b, "INSERT INTO t VALUES(1,'committed')")
	encryptedExec(t, b, "COMMIT")
	if got := query(t, a, "SELECT v FROM t"); !reflect.DeepEqual(got, [][]any{{"committed"}}) {
		t.Fatal(got)
	}
}

func TestReviewEncryptionSharedLatestCheckpointMetadata(t *testing.T) {
	ctx := context.Background()
	cfg := encryptionOptions(t, encryption.RawKey256)
	cfg.Mode = sqlite.EncryptionWALShared
	path := filepath.Join(t.TempDir(), "checkpoint-meta.gse")
	opts := sqlite.Options{Encryption: &cfg, PageSize: 512, KeepWALOnClose: true}
	a, err := sqlite.Open(path, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	encryptedExec(t, a, "CREATE TABLE t(id INTEGER PRIMARY KEY,v TEXT)")
	if err = a.Checkpoint(ctx); err != nil {
		t.Fatal(err)
	}
	old := a.Info()
	b, err := sqlite.Open(path, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	encryptedExec(t, b, "INSERT INTO t VALUES(1,?)", string(bytes.Repeat([]byte{'x'}, 4096)))
	latest := b.Info()
	if latest.PageCount <= old.PageCount {
		t.Fatal("fixture did not grow", old, latest)
	}
	// A previously saw a zero-segment small head; checkpoint must refresh first.
	if err = a.Checkpoint(ctx); err != nil {
		t.Fatal(err)
	}
	if got := a.Info(); got != latest {
		t.Fatal("checkpoint returned stale observed geometry", got, latest)
	}
	info, err := a.VerifyEncrypted(ctx)
	if err != nil || info.PageCount != latest.PageCount || info.WALSegments != 0 {
		t.Fatal("checkpoint missed latest head", info, err)
	}
}
