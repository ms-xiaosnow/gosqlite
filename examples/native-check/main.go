// The disposable proof uses native APIs to create intact fixtures and invokes
// the real CLI with explicit provider descriptors. No external SQL engine is used.
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

type keyProvider struct{ key []byte }
type keyMaterial struct{ key []byte }

func (p keyProvider) Acquire(context.Context, encryption.KeyRequest) (encryption.KeyMaterial, error) {
	return &keyMaterial{append([]byte(nil), p.key...)}, nil
}
func (m *keyMaterial) Kind() encryption.CredentialKind { return encryption.RawKey256 }
func (m *keyMaterial) Bytes() []byte                   { return m.key }
func (m *keyMaterial) Release()                        { clear(m.key) }
func snapshot(dir string) (map[string][32]byte, error) {
	out := map[string][32]byte{}
	err := filepath.WalkDir(dir, func(path string, e os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		out[path] = sha256.Sum256(b)
		return nil
	})
	return out, err
}
func create(path string, opts sqlite.Options) (result sqlite.CheckResult, retErr error) {
	db, err := sqlite.Open(path, opts)
	if err != nil {
		return result, err
	}
	defer func() { retErr = errors.Join(retErr, db.Close()) }()
	ctx := context.Background()
	for _, sql := range []string{"CREATE TABLE t(id INTEGER PRIMARY KEY,body TEXT)", "CREATE INDEX ordinary ON t(body)", "INSERT INTO t VALUES(1,'数据库'),(2,'bounded native check')", "CREATE INDEX grams ON t(body) USING grams13"} {
		if _, err = db.Exec(ctx, sql); err != nil {
			return result, err
		}
	}
	return db.Check(ctx)
}
func run(binary string) error {
	dir, err := os.MkdirTemp("", "gosqlite-check-proof-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir) // only normally completed disposable fixture directory
	key := make([]byte, 32)
	if _, err = rand.Read(key); err != nil {
		return err
	}
	defer clear(key)
	for _, mode := range []string{"plain", "snapshot", "wal", "shared"} {
		opts := sqlite.Options{}
		if mode != "plain" {
			cfg := sqlite.EncryptionOptions{KeyProvider: keyProvider{key}, CredentialKind: encryption.RawKey256}
			if mode == "wal" {
				cfg.Mode = sqlite.EncryptionWAL
			}
			if mode == "shared" {
				cfg.Mode = sqlite.EncryptionWALShared
			}
			opts.Encryption = &cfg
			opts.KeepWALOnClose = mode != "snapshot"
		}
		path := filepath.Join(dir, mode+".db")
		want, err := create(path, opts)
		if err != nil {
			return err
		}
		before, err := snapshot(dir)
		if err != nil {
			return err
		}
		args := []string{"check", path}
		if mode != "plain" {
			args = append(args, "--key-provider", "stdin-hex", "--mode", mode)
		}
		cmd := exec.Command(binary, args...)
		if mode != "plain" {
			cmd.Stdin = bytes.NewBufferString(hex.EncodeToString(key))
		}
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s CLI failed: %w", mode, err)
		}
		var got struct {
			Status string `json:"status"`
			sqlite.CheckResult
		}
		if err = json.Unmarshal(output, &got); err != nil {
			return err
		}
		if got.Status != "ok" || got.CheckResult != want {
			return fmt.Errorf("%s API/CLI result mismatch", mode)
		}
		after, err := snapshot(dir)
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(before, after) {
			return fmt.Errorf("%s read-only check changed family bytes", mode)
		}
		if err = json.NewEncoder(os.Stdout).Encode(struct {
			Mode string `json:"mode"`
			sqlite.CheckResult
		}{mode, got.CheckResult}); err != nil {
			return err
		}
	}
	return nil
}
func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: native-check <absolute gosqlite binary>")
		os.Exit(1)
	}
	if err := run(os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
