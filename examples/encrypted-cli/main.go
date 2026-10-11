// Command encrypted-cli exercises real backup/rekey subprocesses with disposable
// generated credentials supplied only through environment variables or stdin.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/examples/internal/fixturecleanup"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

type keyProvider struct{ key []byte }
type keyMaterial struct{ key []byte }

func (p *keyProvider) Acquire(ctx context.Context, r encryption.KeyRequest) (encryption.KeyMaterial, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.Kind != encryption.RawKey256 {
		return nil, encryption.ErrCredential
	}
	return &keyMaterial{append([]byte(nil), p.key...)}, nil
}
func (m *keyMaterial) Kind() encryption.CredentialKind { return encryption.RawKey256 }
func (m *keyMaterial) Bytes() []byte                   { return m.key }
func (m *keyMaterial) Release()                        { clear(m.key); m.key = nil }
func (*keyProvider) Format(s fmt.State, _ rune)        { _, _ = s.Write([]byte("demo-provider{redacted}")) }
func (*keyMaterial) Format(s fmt.State, _ rune)        { _, _ = s.Write([]byte("demo-material{redacted}")) }

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1) // All normal deferred resource cleanup has already completed.
	}
}

func run() (resultErr error) {
	if len(os.Args) != 2 {
		return errors.New("usage: encrypted-cli <path-to-gosqlite-binary>")
	}
	binary, err := filepath.Abs(os.Args[1])
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "gosqlite-cli-demo-")
	if err != nil {
		return err
	}
	completed := false
	// Evaluate the final result after every later resource defer has run.
	defer func() { resultErr = fixturecleanup.Finalize(dir, completed, resultErr, os.RemoveAll) }()
	// Removal of our disposable directory and clearing owned bytes are not
	// secure erase. The Go runtime/environment may retain credential copies.
	var cfg [3]sqlite.EncryptionOptions
	var providers [3]*keyProvider
	for i := range cfg {
		key := make([]byte, encryption.KeySize)
		if _, err := rand.Read(key); err != nil {
			return err
		}
		defer clear(key)
		providers[i] = &keyProvider{key}
		cfg[i] = sqlite.EncryptionOptions{KeyProvider: providers[i], CredentialKind: encryption.RawKey256}
	}
	ctx := context.Background()
	path := filepath.Join(dir, "source.gse")
	db, err := sqlite.Open(path, sqlite.Options{Encryption: &cfg[0]})
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, db.Close()) }()
	if _, err := db.Exec(ctx, "CREATE TABLE notes(id INTEGER PRIMARY KEY, body TEXT)"); err != nil {
		return err
	}
	if _, err := db.Exec(ctx, "INSERT INTO notes VALUES(1,'disposable CLI backup')"); err != nil {
		return err
	}
	if err := db.Close(); err != nil {
		return err
	}

	// exec.Command never receives credentials as arguments. We use ordinary
	// synchronous Run, with normal process completion and no signal/kill hooks.
	archive := filepath.Join(dir, "portable.backup")
	backup := exec.Command(binary, "backup", path, "--out", archive, "--key-provider", "env-hex:GOSQLITE_DEMO_OLD", "--out-key-provider", "stdin-hex")
	backup.Env = []string{"GOSQLITE_DEMO_OLD=" + hex.EncodeToString(providers[0].key)}
	backup.Stdin = bytes.NewBufferString(hex.EncodeToString(providers[2].key))
	var output bytes.Buffer
	backup.Stdout, backup.Stderr = &output, &output
	if err := backup.Run(); err != nil {
		return errors.New("disposable CLI backup failed")
	}
	if _, err = fmt.Print(output.String()); err != nil {
		return err
	}

	rekey := exec.Command(binary, "rekey", path, "--current-key-provider", "env-hex:GOSQLITE_DEMO_OLD", "--key-provider", "env-hex:GOSQLITE_DEMO_NEW")
	rekey.Env = []string{"GOSQLITE_DEMO_OLD=" + hex.EncodeToString(providers[0].key), "GOSQLITE_DEMO_NEW=" + hex.EncodeToString(providers[1].key)}
	output.Reset()
	rekey.Stdout, rekey.Stderr = &output, &output
	if err := rekey.Run(); err != nil {
		return errors.New("disposable CLI rekey failed")
	}
	if _, err = fmt.Print(output.String()); err != nil {
		return err
	}
	if err := verify(ctx, path, cfg[1]); err != nil {
		return err
	}

	restored := filepath.Join(dir, "restored.gse")
	if _, err := sqlite.RestoreEncrypted(ctx, archive, restored, cfg[2], cfg[1]); err != nil {
		return err
	}
	if err := verify(ctx, restored, cfg[1]); err != nil {
		return err
	}
	if _, err = fmt.Println("Correct-key CLI backup/rekey, native restore, and both data readbacks passed; disposable files will be removed."); err != nil {
		return err
	}
	completed = true
	return nil
}

func verify(ctx context.Context, path string, cfg sqlite.EncryptionOptions) (resultErr error) {
	db, err := sqlite.Open(path, sqlite.Options{Encryption: &cfg, ReadOnly: true, NoCreate: true})
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, db.Close()) }()
	rows, err := db.Query(ctx, "SELECT id,body FROM notes")
	if err != nil {
		return err
	}
	defer func() { resultErr = errors.Join(resultErr, rows.Close()) }()
	if !rows.Next() || len(rows.Values()) != 2 || rows.Values()[0] != int64(1) || rows.Values()[1] != "disposable CLI backup" {
		return errors.New("disposable CLI example row mismatch")
	}
	if rows.Next() {
		return errors.New("unexpected extra row")
	}
	return rows.Err()
}
