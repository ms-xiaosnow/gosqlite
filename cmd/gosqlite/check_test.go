package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/internal/testkit/fileview"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func cliCheckFamily(t *testing.T, dir string) map[string][32]byte {
	t.Helper()
	m := map[string][32]byte{}
	if err := filepath.WalkDir(dir, func(p string, e os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			return nil
		}
		b, err := fileview.ReadFile(p)
		if err != nil {
			return err
		}
		m[p] = sha256.Sum256(b)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return m
}
func TestCLINativeCheckPlainAndNamed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "normal.db")
	for _, q := range []string{"CREATE TABLE t(id INTEGER PRIMARY KEY,k TEXT)", "CREATE INDEX idx ON t(k)", "INSERT INTO t VALUES(1,'数据库')", "CREATE INDEX grams ON t(k) USING grams13"} {
		if err := run([]string{"query", path, q}, io.Discard); err != nil {
			t.Fatal(err)
		}
	}
	before := cliCheckFamily(t, dir)
	var out bytes.Buffer
	if err := run([]string{"check", path}, &out); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Status string `json:"status"`
		sqlite.CheckResult
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil || result.Status != "ok" || result.Scope != sqlite.CheckScope || result.ExtensionIndexes != 1 || result.Indexes != 4 {
		t.Fatal(result, err)
	}
	out.Reset()
	if err := run([]string{"check", path, "grams"}, &out); err != nil || !strings.Contains(out.String(), "consistency only") {
		t.Fatal(out.String(), err)
	}
	out.Reset()
	if err := run([]string{"inspect", path}, &out); err != nil || !strings.Contains(out.String(), "metadata inspection only") {
		t.Fatal(out.String(), err)
	}
	if !reflect.DeepEqual(before, cliCheckFamily(t, dir)) {
		t.Fatal("check/inspect changed family")
	}
	for _, flag := range []string{"--wal-exclusive", "--wal-shared"} {
		out.Reset()
		if err := run([]string{"check", flag, path}, &out); !errors.Is(err, sqlite.ErrUnsupported) || out.Len() != 0 {
			t.Fatal(err, out.String())
		}
	}
	if !reflect.DeepEqual(before, cliCheckFamily(t, dir)) {
		t.Fatal("rejected full WAL check opened or converted database")
	}
}
func TestCLINativeCheckCorrectKeyProviders(t *testing.T) {
	for _, tc := range []struct {
		name string
		mode sqlite.EncryptionMode
	}{{"snapshot", sqlite.EncryptionSnapshot}, {"wal", sqlite.EncryptionWAL}, {"shared", sqlite.EncryptionWALShared}} {
		t.Run(tc.name, func(t *testing.T) {
			key := disposableCLIKey(t)
			input := credentialInput{lookupEnv: func(name string) (string, bool) { return hex.EncodeToString(key), name == "KEY" }, stdin: func() (io.Reader, error) { return strings.NewReader(hex.EncodeToString(key)), nil }}
			provider := testCLIProvider(t, "env-hex:KEY", input)
			cfg := sqlite.EncryptionOptions{KeyProvider: provider, CredentialKind: encryption.RawKey256, Mode: tc.mode}
			dir := t.TempDir()
			path := filepath.Join(dir, "normal.gse")
			cliFixture(t, path, cfg)
			before := cliCheckFamily(t, dir)
			for _, descriptor := range []string{"env-hex:KEY", "stdin-hex"} {
				var out bytes.Buffer
				if err := runCheckWithInput(context.Background(), []string{path, "--key-provider", descriptor, "--mode", tc.name}, &out, input); err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(out.String(), sqlite.CheckScope) || strings.Contains(out.String(), hex.EncodeToString(key)) {
					t.Fatal("missing scope or credential disclosure")
				}
			}
			if !reflect.DeepEqual(before, cliCheckFamily(t, dir)) {
				t.Fatal("encrypted check changed family")
			}
		})
	}
}
func TestCLINativeCheckMemoryArguments(t *testing.T) {
	const marker = "private-input-must-not-echo"
	input := credentialInput{lookupEnv: func(string) (string, bool) { t.Fatal("invalid args acquired key"); return "", false }, stdin: func() (io.Reader, error) { t.Fatal("invalid args read stdin"); return nil, nil }}
	for _, args := range [][]string{nil, {"db", "--key-provider"}, {"db", "--mode", marker}, {"db", "--key-provider", marker}, {"db", "--key-provider= " + marker}, {"db", "--key-provider", "env-hex:KEY", "--key-provider", marker}, {"--wal-shared", "db"}, {"--wal-exclusive", "db", "x", "--key-provider", "env-hex:KEY"}} {
		var out bytes.Buffer
		err := runCheckWithInput(context.Background(), args, &out, input)
		if err == nil || strings.Contains(err.Error(), marker) || out.Len() != 0 {
			t.Fatal(err, out.String())
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out bytes.Buffer
	if err := runCheckWithInput(ctx, []string{"unused"}, &out, input); !errors.Is(err, context.Canceled) || out.Len() != 0 {
		t.Fatal(err)
	}
}
