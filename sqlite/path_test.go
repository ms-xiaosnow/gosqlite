package sqlite

import (
	"context"
	"crypto/rand"
	"path/filepath"
	"sync"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/encryption"
)

type pathTestProvider struct {
	mu       sync.Mutex
	key      [32]byte
	paths    []string
	releases int
}
type pathTestMaterial struct{ provider *pathTestProvider }

func (p *pathTestProvider) Acquire(_ context.Context, request encryption.KeyRequest) (encryption.KeyMaterial, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.paths = append(p.paths, request.Path)
	return &pathTestMaterial{provider: p}, nil
}
func (*pathTestMaterial) Kind() encryption.CredentialKind { return encryption.RawKey256 }
func (m *pathTestMaterial) Bytes() []byte                 { return m.provider.key[:] }
func (m *pathTestMaterial) Release() {
	m.provider.mu.Lock()
	defer m.provider.mu.Unlock()
	m.provider.releases++
}

// Ordinary successful creation/reopen only. No symlink/namespace manipulation,
// selected-file removal, disk mutation, failure injection or wrong key is used.
func TestCanonicalOpenPathOrdinaryConnectionsAndProvider(t *testing.T) {
	for _, mode := range []string{"plain", "exclusive", "shared", "encrypted"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			path := filepath.Join(t.TempDir(), "ordinary.db")
			opts := Options{}
			provider := &pathTestProvider{}
			if _, err := rand.Read(provider.key[:]); err != nil {
				t.Fatal(err)
			}
			defer clear(provider.key[:])
			switch mode {
			case "exclusive":
				opts.WALExclusive = true
			case "shared":
				opts.WALShared = true
			case "encrypted":
				opts.Encryption = &EncryptionOptions{KeyProvider: provider, CredentialKind: encryption.RawKey256}
			}
			canonical, err := canonicalOpenPath(path, opts.Encryption != nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(provider.paths) != 0 {
				t.Fatal("path preflight acquired key material")
			}
			db, err := OpenContext(ctx, path, opts)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if db.path != canonical {
				t.Fatal("connection path differs from backend canonical path")
			}
			for _, sql := range []string{"CREATE TABLE path_values(id INTEGER PRIMARY KEY)", "INSERT INTO path_values VALUES(7)"} {
				if _, err = db.Exec(ctx, sql); err != nil {
					t.Fatal(err)
				}
			}
			if err = db.Close(); err != nil {
				t.Fatal(err)
			}
			opts.NoCreate = true
			db, err = OpenContext(ctx, path, opts)
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			if db.path != canonical {
				t.Fatal("reopen changed the canonical family")
			}
			rows, err := db.Query(ctx, "SELECT id FROM path_values")
			if err != nil {
				t.Fatal(err)
			}
			if !rows.Next() || rows.Values()[0] != int64(7) || rows.Next() || rows.Err() != nil {
				t.Fatal("ordinary reopen changed committed values", rows.Err())
			}
			if err = rows.Close(); err != nil {
				t.Fatal(err)
			}
			if err = db.Close(); err != nil {
				t.Fatal(err)
			}
			provider.mu.Lock()
			defer provider.mu.Unlock()
			for _, requested := range provider.paths {
				if requested != canonical {
					t.Fatal("provider path differs from DB.path")
				}
			}
			if mode == "encrypted" && (len(provider.paths) != 2 || provider.releases != 2) {
				t.Fatal("provider acquisition/release lifecycle changed", len(provider.paths), provider.releases)
			}
		})
	}
}
