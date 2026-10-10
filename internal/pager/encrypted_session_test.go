package pager

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"github.com/ms-xiaosnow/gosqlite/internal/testkit/fileview"
	"os"
	"path/filepath"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/internal/encstore"
)

type encryptedTestMaterial []byte

func (m encryptedTestMaterial) Kind() encryption.CredentialKind { return encryption.RawKey256 }
func (m encryptedTestMaterial) Bytes() []byte                   { return m }
func (m encryptedTestMaterial) Release()                        { clear(m) }

type encryptedTestProvider []byte

func (p encryptedTestProvider) Acquire(context.Context, encryption.KeyRequest) (encryption.KeyMaterial, error) {
	return encryptedTestMaterial(bytes.Clone(p)), nil
}
func encryptedTestOptions(t *testing.T) encstore.Options {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { clear(key) })
	return encstore.Options{KeyProvider: encryptedTestProvider(key), CredentialKind: encryption.RawKey256}
}

func TestEncryptedPagerOrdinarySnapshotLifecycle(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "sealed.db")
	cfg := encryptedTestOptions(t)
	s, err := OpenEncryptedSession(ctx, path, Options{Create: true, PageSize: 512}, cfg, false)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path + ".gse-lock")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := s.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	n, err := tx.AllocatePage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	page := bytes.Repeat([]byte{71}, 512)
	if err = tx.WritePage(ctx, n, page); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	after, err := os.Stat(path + ".gse-lock")
	if err != nil || !os.SameFile(info, after) {
		t.Fatal("stable lock inode replaced", err)
	}
	tx, err = s.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	changed := bytes.Repeat([]byte{99}, 512)
	if err = tx.WritePage(ctx, n, changed); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	r, err := s.Reader(ctx)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := r.ReadPage(ctx, n)
	if err != nil || !bytes.Equal(actual, page) {
		t.Fatal("rollback lost committed image", err)
	}
	if err = r.Close(); err != nil {
		t.Fatal(err)
	}
	rootBefore, err := fileview.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	tx, err = s.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	rootAfter, err := fileview.ReadFile(path)
	if err != nil || !bytes.Equal(rootBefore, rootAfter) {
		t.Fatal("no-op changed root", err)
	}
	if err = s.Verify(ctx); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = OpenEncryptedSession(ctx, path, Options{}, cfg, true)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r, err = s.Reader(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	actual, err = r.ReadPage(ctx, n)
	if err != nil || !bytes.Equal(actual, page) {
		t.Fatal("normal reopen", err)
	}
	s2, err := OpenEncryptedSession(ctx, path, Options{}, cfg, true)
	if err != nil {
		t.Fatal("shared read-only lease", err)
	}
	if err = s2.Close(); err != nil {
		t.Fatal(err)
	}
	_, err = OpenEncryptedSession(ctx, path, Options{}, cfg, false)
	if !errors.Is(err, ErrBusy) {
		t.Fatalf("writer should be busy with readers: %v", err)
	}
}
