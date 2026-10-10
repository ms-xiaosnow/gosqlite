package pager

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/internal/encstore"
)

// Cooperative commits happen inside the pinned-copy validation callback. No
// selected file is removed, replaced manually, damaged or opened with bad keys.
func TestEncryptedSharedCopyPinsSelectionThroughValidationAndOutput(t *testing.T) {
	ctx := context.Background()
	cfg := encryptedTestOptions(t)
	cfg.Mode = encstore.WALShared
	dir := t.TempDir()
	path := filepath.Join(dir, "source.gse")
	opts := Options{Create: true, PageSize: 512, KeepWALOnClose: true}
	a, e := OpenEncryptedSession(ctx, path, opts, cfg, false)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	tx, e := a.BeginImmediate(ctx)
	if e != nil {
		t.Fatal(e)
	}
	n, e := tx.AllocatePage(ctx)
	if e != nil {
		t.Fatal(e)
	}
	before := bytes.Repeat([]byte{31}, 512)
	after := bytes.Repeat([]byte{32}, 512)
	if e = tx.WritePage(ctx, n, before); e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	b, e := OpenEncryptedSession(ctx, path, opts, cfg, false)
	if e != nil {
		t.Fatal(e)
	}
	defer b.Close()
	targetCfg := encryptedTestOptions(t)
	archive := filepath.Join(dir, "archive.gse")
	target, e := encstore.Prepare(ctx, archive, targetCfg, encryption.OperationBackupTarget, [16]byte{})
	if e != nil {
		t.Fatal(e)
	}
	defer target.Close()
	validate := func(c context.Context, r SnapshotReader) error {
		original, e := r.ReadPage(c, n)
		if e != nil {
			return e
		}
		defer clear(original)
		if !bytes.Equal(original, before) {
			t.Fatal("copy selected wrong initial head")
		}
		tx, e := b.BeginImmediate(c)
		if e != nil {
			return e
		}
		if e = tx.WritePage(c, n, after); e != nil {
			return errors.Join(e, tx.Rollback())
		}
		if e = tx.Commit(c); e != nil {
			return errors.Join(e, tx.Rollback())
		}
		old, e := r.ReadPage(c, n)
		if e != nil {
			return e
		}
		defer clear(old)
		if !bytes.Equal(old, before) {
			t.Fatal("validation changed its selection")
		}
		return nil
	}
	if _, e = a.BackupPrepared(ctx, archive, target, validate); e != nil {
		t.Fatal(e)
	}
	ar, e := OpenEncryptedArchive(ctx, archive, targetCfg)
	if e != nil {
		t.Fatal(e)
	}
	p, e := ar.ReadPage(ctx, n)
	if e != nil || !bytes.Equal(p, before) {
		t.Fatal("archive copied another head", e)
	}
	clear(p)
	if e = ar.Close(); e != nil {
		t.Fatal(e)
	}
	// Export captures the now-current head, then remains pinned through another commit.
	before, after = after, bytes.Repeat([]byte{33}, 512)
	plain := filepath.Join(dir, "explicit-export.db")
	if e = a.ExportPlaintext(ctx, plain, validate); e != nil {
		t.Fatal(e)
	}
	reader, e := Open(plain)
	if e != nil {
		t.Fatal(e)
	}
	p, e = reader.ReadPage(ctx, n)
	if e != nil || !bytes.Equal(p, before) {
		t.Fatal("plaintext copied another head", e)
	}
	clear(p)
	if e = reader.Close(); e != nil {
		t.Fatal(e)
	}
	fresh, e := a.Reader(ctx)
	if e != nil {
		t.Fatal(e)
	}
	p, e = fresh.ReadPage(ctx, n)
	if e != nil || !bytes.Equal(p, after) {
		t.Fatal("fresh snapshot after copy", e)
	}
	clear(p)
	if e = fresh.Close(); e != nil {
		t.Fatal(e)
	}
}

func TestEncryptedSharedPoisonPriorityMemory(t *testing.T) {
	err := encryptedError(errors.Join(encstore.ErrBusy, encstore.ErrUncertain))
	if !errors.Is(err, ErrPoisoned) || !errors.Is(err, ErrCommitUncertain) {
		t.Fatal("uncertainty hidden by busy", err)
	}
	err = encryptedError(errors.Join(encstore.ErrBusy, encstore.ErrPoisoned))
	if !errors.Is(err, ErrPoisoned) {
		t.Fatal("ownership poison hidden by busy", err)
	}
}
