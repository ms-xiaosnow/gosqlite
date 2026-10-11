// Command encrypted demonstrates only ordinary, correct-key snapshot workflows.
// Every key and database is disposable. Do not use this provider for durable data.
package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/examples/internal/fixturecleanup"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// ephemeralProvider retains one random key in memory for this process only.
// Acquire returns an independent borrow; Release clears that borrow exactly once.
// A real application needs its own trusted, durable credential-management code.
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
	// Do not use unauthenticated request IDs to choose an external account or key.
	return &borrowedKey{key: append([]byte(nil), p.key...)}, nil
}

func (p *ephemeralProvider) clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	clear(p.key)
	runtime.KeepAlive(p.key)
	p.key = nil
	// Best effort only: Go and crypto implementations may retain other copies.
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err) // run's deferred cleanup has already completed.
	}
}

func run() (err error) {
	ctx := context.Background()
	dir, err := os.MkdirTemp("", "gosqlite-encrypted-demo-")
	if err != nil {
		return err
	}
	// Only ordinary completed success may remove the private demo directory.
	completed := false
	// Evaluate the final result after every later resource defer has run.
	defer func() { err = fixturecleanup.Finalize(dir, completed, err, os.RemoveAll) }()
	for _, name := range []string{"source", "archive", "restored"} {
		if err = os.Mkdir(filepath.Join(dir, name), 0700); err != nil {
			return err
		}
	}

	var options [4]sqlite.EncryptionOptions
	for i := range options {
		provider, e := newProvider()
		if e != nil {
			return e
		}
		defer provider.clear()
		options[i] = sqlite.EncryptionOptions{
			KeyProvider:    provider,
			CredentialKind: encryption.RawKey256,
			Mode:           sqlite.EncryptionSnapshot,
		}
	}
	initial, rotated, archiveKey, restoredKey := options[0], options[1], options[2], options[3]
	path := filepath.Join(dir, "source", "demo.gse")
	db, err := sqlite.OpenContext(ctx, path, sqlite.Options{Encryption: &initial})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	if _, err = db.Exec(ctx, "CREATE TABLE notes(id INTEGER PRIMARY KEY, body TEXT)"); err != nil {
		return err
	}
	if _, err = db.Exec(ctx, "INSERT INTO notes VALUES(?, ?)", 1, "disposable encrypted example"); err != nil {
		return err
	}
	if err = checkNote(ctx, db); err != nil {
		return err
	}
	before, err := db.VerifyEncrypted(ctx)
	if err != nil {
		return err
	}
	if _, err = fmt.Println("Created, wrote, queried, and authenticated an encrypted snapshot."); err != nil {
		return err
	}

	if err = db.Rekey(ctx, rotated); err != nil {
		return err
	}
	after, err := db.VerifyEncrypted(ctx)
	if err != nil {
		return err
	}
	if after.DatabaseID != before.DatabaseID || after.CommitSequence != before.CommitSequence+1 || !after.RetainsHistory {
		return errors.New("unexpected rekey metadata")
	}
	if err = db.Close(); err != nil {
		return err
	}

	// Release the lifetime EXCLUSIVE lease before a read-only reopen.
	reopened, err := sqlite.OpenContext(ctx, path, sqlite.Options{
		Encryption: &rotated, ReadOnly: true, NoCreate: true,
	})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, reopened.Close()) }()
	if err = checkNote(ctx, reopened); err != nil {
		return err
	}
	if _, err = fmt.Println("Rekeyed and reopened with the new key; old ciphertext is retained."); err != nil {
		return err
	}

	archive := filepath.Join(dir, "archive", "demo.backup.gse")
	backupInfo, err := reopened.BackupEncrypted(ctx, archive, archiveKey)
	if err != nil {
		return err
	}
	if backupInfo.DatabaseID == after.DatabaseID || backupInfo.CommitSequence != 1 || backupInfo.RetainsHistory {
		return errors.New("unexpected portable backup metadata")
	}
	restoredPath := filepath.Join(dir, "restored", "demo.gse")
	restoredInfo, err := sqlite.RestoreEncrypted(ctx, archive, restoredPath, archiveKey, restoredKey)
	if err != nil {
		return err
	}
	if restoredInfo.DatabaseID == backupInfo.DatabaseID || restoredInfo.DatabaseID == after.DatabaseID || restoredInfo.CommitSequence != 1 {
		return errors.New("unexpected restored snapshot metadata")
	}
	restored, err := sqlite.OpenContext(ctx, restoredPath, sqlite.Options{
		Encryption: &restoredKey, ReadOnly: true, NoCreate: true,
	})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, restored.Close()) }()
	if err = checkNote(ctx, restored); err != nil {
		return err
	}
	if _, err = restored.VerifyEncrypted(ctx); err != nil {
		return err
	}
	if _, err = fmt.Println("Created a portable encrypted backup and verified a fresh restored family."); err != nil {
		return err
	}
	if _, err = fmt.Println("Keys stayed in memory; disposable files will now be removed (not securely erased)."); err != nil {
		return err
	}
	completed = true
	return nil
}

// Always release Rows before verification, rekey, backup, or closing a connection.
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
	if len(values) != 2 || values[0] != int64(1) || values[1] != "disposable encrypted example" {
		return errors.New("demo row mismatch")
	}
	if rows.Next() {
		return errors.New("unexpected extra demo row")
	}
	return rows.Err()
}
