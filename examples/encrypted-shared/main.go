// Command encrypted-shared demonstrates cooperating encrypted snapshots.
// Credentials exist only in memory; this disposable demo is not key management.
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
)

// ephemeralProvider owns one process-local key. Each Acquire returns a separate
// borrowed buffer; the library calls its Release after copying the credential.
type ephemeralProvider struct {
	mu  sync.Mutex
	key []byte
}

type borrowedKey struct{ key []byte }

func (m *borrowedKey) Kind() encryption.CredentialKind { return encryption.RawKey256 }
func (m *borrowedKey) Bytes() []byte                   { return m.key }
func (m *borrowedKey) Release()                        { clear(m.key); m.key = nil }

func newProvider() (*ephemeralProvider, error) {
	key := make([]byte, encryption.KeySize)
	if _, err := rand.Read(key); err != nil {
		clear(key)
		return nil, err
	}
	return &ephemeralProvider{key: key}, nil
}

func (p *ephemeralProvider) Acquire(ctx context.Context, req encryption.KeyRequest) (encryption.KeyMaterial, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if req.Kind != encryption.RawKey256 || len(p.key) != encryption.KeySize {
		return nil, errors.New("demo credential unavailable")
	}
	// Unauthenticated request IDs never select an external account or credential.
	return &borrowedKey{key: append([]byte(nil), p.key...)}, nil
}

func (p *ephemeralProvider) clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	clear(p.key)
	runtime.KeepAlive(p.key)
	p.key = nil
	// Best effort only: this does not clear all Go/runtime/crypto copies.
}

func main() {
	if err := run(os.Stdout); err != nil {
		log.Fatal(err)
	}
}
func run(out io.Writer) (err error) {
	ctx := context.Background()
	dir, err := os.MkdirTemp("", "gosqlite-encrypted-shared-")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(dir)) }()
	provider, err := newProvider()
	if err != nil {
		return err
	}
	defer provider.clear()
	cfg := sqlite.EncryptionOptions{KeyProvider: provider, CredentialKind: encryption.RawKey256, Mode: sqlite.EncryptionWALShared}
	opts := sqlite.Options{Encryption: &cfg, KeepWALOnClose: true}
	path := filepath.Join(dir, "demo.gse")
	a, err := sqlite.OpenContext(ctx, path, opts)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, a.Close()) }()
	exec := func(db *sqlite.DB, q string) error { _, e := db.Exec(ctx, q); return e }
	for _, q := range []string{"CREATE TABLE notes(id INTEGER PRIMARY KEY,body TEXT)", "INSERT INTO notes VALUES(1,'old')"} {
		if err = exec(a, q); err != nil {
			return err
		}
	}
	b, err := sqlite.OpenContext(ctx, path, opts)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, b.Close()) }()
	if err = exec(a, "BEGIN"); err != nil {
		return err
	}
	if err = checkValue(ctx, a, "old"); err != nil {
		return err
	}
	if err = exec(b, "BEGIN IMMEDIATE"); err != nil {
		return err
	}
	if err = exec(b, "UPDATE notes SET body='new'"); err != nil {
		return err
	}
	ro := opts
	ro.ReadOnly = true
	c, err := sqlite.OpenContext(ctx, path, ro)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, c.Close()) }()
	if err = checkValue(ctx, c, "old"); err != nil {
		return err
	}
	if err = c.Close(); err != nil {
		return err
	}
	fmt.Fprintln(out, "A new reader sees the committed head while another connection owns unpublished writes.")
	if err = exec(b, "COMMIT"); err != nil {
		return err
	}
	if err = checkValue(ctx, a, "old"); err != nil {
		return err
	}
	if err = exec(a, "UPDATE notes SET body='stale'"); !errors.Is(err, sqlite.ErrBusySnapshot) {
		return errors.Join(errors.New("expected stale-snapshot BUSY"), err)
	}
	if err = exec(a, "ROLLBACK"); err != nil {
		return err
	}
	if err = checkValue(ctx, a, "new"); err != nil {
		return err
	}
	fmt.Fprintln(out, "The old reader stays pinned; stale promotion fails, and a fresh transaction sees the new value.")
	if err = exec(a, "BEGIN"); err != nil {
		return err
	}
	if err = checkValue(ctx, a, "new"); err != nil {
		return err
	}
	if err = b.Checkpoint(ctx); err != nil {
		return err
	}
	if err = checkValue(ctx, a, "new"); err != nil {
		return err
	}
	if err = exec(a, "ROLLBACK"); err != nil {
		return err
	}
	info, err := b.VerifyEncrypted(ctx)
	if err != nil {
		return err
	}
	if info.StorageLayout != 3 || info.WALSegments != 0 {
		return errors.New("unexpected shared checkpoint metadata")
	}
	fmt.Fprintln(out, "Checkpoint retains old snapshots and selects a fresh base; all historical ciphertext remains.")
	_, err = fmt.Fprintln(out, "Credentials stayed in memory. Disposable files are removed after normal close, without secure-erasure claims.")
	return err
}
func checkValue(ctx context.Context, db *sqlite.DB, want string) (err error) {
	rows, err := db.Query(ctx, "SELECT body FROM notes")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, rows.Close()) }()
	if !rows.Next() {
		return errors.Join(errors.New("missing demo row"), rows.Err())
	}
	if values := rows.Values(); len(values) != 1 || values[0] != want {
		return errors.New("demo snapshot mismatch")
	}
	if rows.Next() {
		return errors.New("unexpected extra row")
	}
	return rows.Err()
}
