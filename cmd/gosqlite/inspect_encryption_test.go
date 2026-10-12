package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/extension/grams13"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// These are healthy, correct-key native fixture tests, not memory models.
// No wrong-key, tamper, failed native Close, recovery or namespace experiment.
// Every acquired DB has one checked Close attempt. Failed fixture directories
// and successful directories are retained; disposable key buffers are still cleared, so retained ciphertext
// is not promised to remain decryptable. No inherited initializer environment.
func inspectNativeDirectory(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "gosqlite-inspect-native-")
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("retained failed inspect fixture directory: %s; disposable credentials are not retained", dir)
			return
		}
		t.Logf("retained successful inspect fixture directory: %s; disposable credentials are not retained", dir)
	})
	return dir
}

func ownInspectNativeDB(t *testing.T, db *sqlite.DB) func() error {
	t.Helper()
	closeAttempted := false
	retire := func() error {
		if closeAttempted { t.Fatal("fixture DB Close was attempted more than once") }
		closeAttempted = true
		return db.Close()
	}
	t.Cleanup(func() {
		if !closeAttempted {
			if err := retire(); err != nil { t.Errorf("fixture DB cleanup failed: %v", err) }
		}
	})
	return retire
}

func inspectNativeFixture(t *testing.T, path string, cfg sqlite.EncryptionOptions) sqlite.InspectResult {
	t.Helper()
	ctx := context.Background()
	db, err := sqlite.OpenContext(ctx, path, sqlite.Options{Encryption: &cfg, KeepWALOnClose: cfg.Mode != sqlite.EncryptionSnapshot})
	if err != nil { t.Fatal(err) }
	retire := ownInspectNativeDB(t, db)
	for _, query := range []string{"CREATE TABLE docs(id INTEGER PRIMARY KEY, body TEXT)", "INSERT INTO docs VALUES(1,'ordinary 数据库')"} {
		if _, err := db.Exec(ctx, query); err != nil { t.Fatal(err) }
	}
	if err := db.CreateGramsIndex(ctx, "docs_grams", "docs", []string{"body"}); err != nil { t.Fatal(err) }
	schema, err := db.Schema(ctx)
	if err != nil { t.Fatal(err) }
	canonical, err := grams13.ConfigBytes(grams13.DefaultMatchConfig())
	if err != nil { t.Fatal(err) }
	want := sqlite.InspectResult{Header: db.Info(), Schema: schema, ExtensionIndexes: []sqlite.ExtensionIndexMetadata{{ID: 1, Name: "docs_grams", SourceTable: "docs", Method: "grams13", MethodVersion: 1, StorageVersion: 1, Config: string(canonical), Generation: 1}}}
	if err := retire(); err != nil { t.Fatal(err) }
	return want
}

func inspectNativeReopen(t *testing.T, path string, cfg sqlite.EncryptionOptions, want sqlite.InspectResult) {
	t.Helper()
	db, err := sqlite.OpenContext(context.Background(), path, sqlite.Options{ReadOnly: true, NoCreate: true, Encryption: &cfg, KeepWALOnClose: cfg.Mode != sqlite.EncryptionSnapshot})
	if err != nil { t.Fatal(err) }
	retire := ownInspectNativeDB(t, db)
	got, err := db.Inspect(context.Background(), sqlite.InspectOptions{IncludeExtensions: true})
	if err != nil { t.Fatal(err) }
	if !reflect.DeepEqual(got, want) { t.Fatal("healthy correct-key reopen metadata differs") }
	if err := retire(); err != nil { t.Fatal(err) }
}

func inspectNativeWantJSON(t *testing.T, mode string, extensions bool, want sqlite.InspectResult) []byte {
	t.Helper()
	var result any
	if extensions {
		result = struct {
			Format           string                          `json:"format"`
			Mode             string                          `json:"mode"`
			MetadataScope    string                          `json:"metadata_scope"`
			Header           sqlite.FileInfo                 `json:"header"`
			Schema           []sqlite.SchemaEntry            `json:"schema"`
			ExtensionIndexes []sqlite.ExtensionIndexMetadata `json:"extension_indexes"`
		}{"gosqlite-encrypted-family", mode, "logical SQLite metadata inspection only", want.Header, want.Schema, want.ExtensionIndexes}
	} else {
		result = struct {
			Format        string               `json:"format"`
			Mode          string               `json:"mode"`
			MetadataScope string               `json:"metadata_scope"`
			Header        sqlite.FileInfo      `json:"header"`
			Schema        []sqlite.SchemaEntry `json:"schema"`
		}{"gosqlite-encrypted-family", mode, "logical SQLite metadata inspection only", want.Header, want.Schema}
	}
	raw, err := json.Marshal(result)
	if err != nil { t.Fatal(err) }
	return append(raw, '\n')
}

func TestCLIInspectNativeCorrectKeyProfiles(t *testing.T) {
	if runtime.GOOS != "linux" { t.Skip("healthy native inspect qualification is Linux only") }
	for _, tc := range []struct { name string; mode sqlite.EncryptionMode }{
		{"snapshot", sqlite.EncryptionSnapshot}, {"wal", sqlite.EncryptionWAL}, {"shared", sqlite.EncryptionWALShared},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := inspectNativeDirectory(t)
			key := disposableCLIKey(t)
			encoded := hex.EncodeToString(key)
			lookupReads, stdinReads := 0, 0
			input := credentialInput{
				lookupEnv: func(name string) (string, bool) { lookupReads++; return encoded, name == "INSPECT_KEY" },
				stdin: func() (io.Reader, error) { stdinReads++; return strings.NewReader(encoded+"\r\n"), nil },
			}
			provider, err := newCLIKeyProvider("env-hex:INSPECT_KEY", input)
			if err != nil { t.Fatal(err) }
			t.Cleanup(provider.close)
			cfg := sqlite.EncryptionOptions{KeyProvider: provider, CredentialKind: encryption.RawKey256, Mode: tc.mode}
			path := filepath.Join(dir, "healthy.gse")
			want := inspectNativeFixture(t, path, cfg)
			for _, descriptor := range []string{"env-hex:INSPECT_KEY", "stdin-hex"} {
				for _, extensions := range []bool{false, true} {
					lookupReads, stdinReads = 0, 0
					args := []string{path, "--key-provider", descriptor, "--mode", tc.name}
					if extensions { args = append([]string{"--extensions"}, args...) }
					var out bytes.Buffer
					if err := runInspectWithInput(context.Background(), args, &out, input); err != nil { t.Fatal(err) }
					if !bytes.Equal(out.Bytes(), inspectNativeWantJSON(t, tc.name, extensions, want)) { t.Fatal("complete native encrypted inspect JSON differs") }
					wantEnv, wantStdin := 1, 0
					if descriptor == "stdin-hex" { wantEnv, wantStdin = 0, 1 }
					if lookupReads != wantEnv || stdinReads != wantStdin { t.Fatal("provider acquired input more than once or from wrong source") }
					for _, private := range []string{encoded, descriptor, path, "INSPECT_KEY"} {
						if strings.Contains(out.String(), private) { t.Fatal("credential/provider/path leaked into result") }
					}
				}
			}
			if tc.mode == sqlite.EncryptionSnapshot {
				// Exercise real main dispatch and explicit env-name binding. This
				// process value is test-owned, never read from initializer state.
				t.Setenv("GOSQLITE_INSPECT_TEST_KEY", encoded)
				var out bytes.Buffer
				if err := runContext(context.Background(), []string{"inspect", path, "--key-provider", "env-hex:GOSQLITE_INSPECT_TEST_KEY"}, &out); err != nil { t.Fatal(err) }
				if !bytes.Equal(out.Bytes(), inspectNativeWantJSON(t, "snapshot", false, want)) { t.Fatal("main dispatch or default encrypted profile differs") }
			}
			inspectNativeReopen(t, path, cfg, want)
		})
	}
}

func TestCLIInspectNativeCorrectPasswordProviders(t *testing.T) {
	if runtime.GOOS != "linux" { t.Skip("healthy native inspect qualification is Linux only") }
	dir := inspectNativeDirectory(t)
	// Generated disposable password uses the unchanged production KDF profile.
	password := hex.EncodeToString(disposableCLIKey(t))+"\r\n"
	input := credentialInput{
		lookupEnv: func(name string) (string, bool) { return password, name == "INSPECT_PASSWORD" },
		stdin: func() (io.Reader, error) { return strings.NewReader(password), nil },
	}
	provider, err := newCLIKeyProvider("env-password:INSPECT_PASSWORD", input)
	if err != nil { t.Fatal(err) }
	t.Cleanup(provider.close)
	cfg := sqlite.EncryptionOptions{KeyProvider: provider, CredentialKind: encryption.PasswordArgon2id}
	path := filepath.Join(dir, "healthy-password.gse")
	want := inspectNativeFixture(t, path, cfg)
	for _, descriptor := range []string{"env-password:INSPECT_PASSWORD", "stdin-password"} {
		for _, extensions := range []bool{false, true} {
			args := []string{path, "--key-provider", descriptor}
			if extensions { args = append([]string{"--extensions"}, args...) }
			var out bytes.Buffer
			if err := runInspectWithInput(context.Background(), args, &out, input); err != nil { t.Fatal(err) }
			if !bytes.Equal(out.Bytes(), inspectNativeWantJSON(t, "snapshot", extensions, want)) { t.Fatal("correct-password default-profile JSON differs") }
			for _, private := range []string{strings.TrimSuffix(password, "\r\n"), descriptor, path, "INSPECT_PASSWORD"} {
				if strings.Contains(out.String(), private) { t.Fatal("password/provider/path leaked into result") }
			}
		}
	}
	inspectNativeReopen(t, path, cfg, want)
}
