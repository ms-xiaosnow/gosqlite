// Command encrypted-wal demonstrates ordinary, correct-key encrypted WAL use.
// Its random key and database are disposable; this is not durable key management.
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
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
		log.Fatal(err) // run's deferred cleanup has already completed.
	}
}

func run(out io.Writer) (err error) {
	ctx := context.Background()
	dir, err := os.MkdirTemp("", "gosqlite-encrypted-wal-demo-")
	if err != nil {
		return err
	}
	// MkdirTemp creates a private directory. Remove only this disposable family,
	// after every owned handle has closed. Removal is not secure erasure.
	defer func() { err = errors.Join(err, os.RemoveAll(dir)) }()
	provider, err := newProvider()
	if err != nil {
		return err
	}
	defer provider.clear()
	cfg := sqlite.EncryptionOptions{
		KeyProvider:    provider,
		CredentialKind: encryption.RawKey256,
		Mode:           sqlite.EncryptionWAL,
	}
	path := filepath.Join(dir, "demo.gse")
	db, err := sqlite.OpenContext(ctx, path, sqlite.Options{
		Encryption: &cfg, KeepWALOnClose: true,
	})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	initial, err := db.VerifyEncrypted(ctx)
	if err != nil {
		return err
	}
	if initial.Mode != sqlite.EncryptionWAL || initial.StorageLayout != 2 || initial.WALSegments != 0 || initial.CommitSequence != 1 {
		return errors.New("unexpected initial encrypted WAL metadata")
	}
	if _, err = db.Exec(ctx, "CREATE TABLE notes(id INTEGER PRIMARY KEY, body TEXT)"); err != nil {
		return err
	}
	if _, err = db.Exec(ctx, "INSERT INTO notes VALUES(?, ?)", 1, "first committed value"); err != nil {
		return err
	}
	if _, err = db.Exec(ctx, "UPDATE notes SET body=? WHERE id=1", "latest committed WAL value"); err != nil {
		return err
	}
	if err = checkNote(ctx, db); err != nil {
		return err
	}
	head, err := db.VerifyEncrypted(ctx)
	if err != nil {
		return err
	}
	if head.WALSegments != 3 || head.WALFrames == 0 || head.WALBytes == 0 || head.CommitSequence != initial.CommitSequence+3 || head.DatabaseID != initial.DatabaseID || head.HeadCommitID == initial.HeadCommitID {
		return errors.New("unexpected committed encrypted WAL head")
	}
	if _, err = fmt.Fprintln(out, "Created, changed, queried, and authenticated an encrypted WAL head (3 segments)."); err != nil {
		return err
	}
	// Release the lifetime EXCLUSIVE lease before any reopen. KeepWALOnClose
	// retains committed segments; this is an ordinary close, not a crash test.
	if err = db.Close(); err != nil {
		return err
	}
	reopened, err := sqlite.OpenContext(ctx, path, sqlite.Options{
		Encryption: &cfg, KeepWALOnClose: true, NoCreate: true,
	})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, reopened.Close()) }()
	if err = checkNote(ctx, reopened); err != nil {
		return err
	}
	retained, err := reopened.VerifyEncrypted(ctx)
	if err != nil {
		return err
	}
	if retained != head {
		return errors.New("normal reopen changed the selected WAL head")
	}
	if _, err = fmt.Fprintln(out, "Normally closed and reopened with the same key; the committed segments remain selected."); err != nil {
		return err
	}
	if err = reopened.Checkpoint(ctx); err != nil {
		return err
	}
	checkpoint, err := reopened.VerifyEncrypted(ctx)
	if err != nil {
		return err
	}
	if checkpoint.Mode != sqlite.EncryptionWAL || checkpoint.StorageLayout != 2 || checkpoint.WALSegments != 0 || checkpoint.WALFrames != 0 || checkpoint.WALBytes != 0 || checkpoint.DatabaseID != head.DatabaseID || checkpoint.PageCount != head.PageCount || checkpoint.CommitSequence != head.CommitSequence+1 || checkpoint.HeadCommitID == head.HeadCommitID || !checkpoint.RetainsHistory {
		return errors.New("unexpected checkpoint metadata")
	}
	if err = checkNote(ctx, reopened); err != nil {
		return err
	}
	if err = reopened.Close(); err != nil {
		return err
	}
	reader, err := sqlite.OpenContext(ctx, path, sqlite.Options{
		Encryption: &cfg, ReadOnly: true, NoCreate: true,
	})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, reader.Close()) }()
	if err = checkNote(ctx, reader); err != nil {
		return err
	}
	verified, err := reader.VerifyEncrypted(ctx)
	if err != nil {
		return err
	}
	if verified != checkpoint {
		return errors.New("read-only reopen changed the checkpointed head")
	}
	if _, err = fmt.Fprintln(out, "Checkpointed and reopened read-only: zero selected segments, same data, retained ciphertext history."); err != nil {
		return err
	}
	_, err = fmt.Fprintln(out, "The key stayed in memory; disposable files will now be removed (not securely erased).")
	return err
}

// Rows must close before VerifyEncrypted, Checkpoint or other maintenance.
func checkNote(ctx context.Context, db *sqlite.DB) (err error) {
	rows, err := db.Query(ctx, "SELECT id, body FROM notes")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, rows.Close()) }()
	if !rows.Next() {
		return errors.Join(errors.New("expected one demo row"), rows.Err())
	}
	values := rows.Values()
	if len(values) != 2 || values[0] != int64(1) || values[1] != "latest committed WAL value" {
		return errors.New("demo row mismatch")
	}
	if rows.Next() {
		return errors.New("unexpected extra demo row")
	}
	return rows.Err()
}
