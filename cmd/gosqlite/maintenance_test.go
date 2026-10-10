package main

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestCLIMaintenanceMemoryArguments(t *testing.T) {
	const secret = "must-not-print-private-argument"
	for _, tc := range []struct {
		command string
		args    []string
	}{
		{"backup", nil}, {"backup", []string{"db", "--out"}},
		{"backup", []string{"db", "--out", "new", "--out", "other"}},
		{"backup", []string{"db", "--out", "new", "--key-provider", secret}},
		{"backup", []string{"db", "--out", "new", "--key-provider=" + secret}},
		{"backup", []string{"db", "--out", "new", "--key-provider", "env-hex:KEY", "--mode", secret}},
		{"backup", []string{"db", "--out", "new", "--key-provider", "stdin-hex", "--out-key-provider", "stdin-hex"}},
		{"rekey", []string{"db", "--key-provider", "env-hex:KEY"}},
		{"rekey", []string{"db", "--key-provider", "env-hex:NEW", "--current-key-provider", secret}},
		{"rekey", []string{"db", "--key-provider", "stdin-password", "--current-key-provider", "stdin-password"}},
	} {
		var output bytes.Buffer
		input := credentialInput{
			lookupEnv: func(string) (string, bool) { t.Fatal("parser touched credential input"); return "", false },
			stdin:     func() (io.Reader, error) { t.Fatal("parser touched stdin"); return nil, nil },
		}
		err := runMaintenanceWithInput(context.Background(), tc.command, tc.args, &output, input)
		if err == nil || output.Len() != 0 || strings.Contains(err.Error(), secret) {
			t.Fatal("invalid arguments accepted, disclosed, or emitted success")
		}
	}
	var out bytes.Buffer
	err := run([]string{"backup", "untouched.db", "--out", "untouched.backup"}, &out)
	if !errors.Is(err, sqlite.ErrUnsupported) || !strings.Contains(err.Error(), "plaintext backup is not implemented") || out.Len() != 0 {
		t.Fatal("plaintext boundary is not explicit")
	}
	if err := run([]string{"help"}, &out); err != nil || !strings.Contains(out.String(), "gosqlite backup") || !strings.Contains(out.String(), "gosqlite rekey") {
		t.Fatal("maintenance commands missing from help")
	}
}

func TestCLIMaintenanceMemoryErrorRedaction(t *testing.T) {
	const marker = "secret-from-untrusted-error"
	for _, cause := range []error{errors.New(marker), errors.Join(sqlite.ErrUnsupported, errors.New(marker)), errors.Join(sqlite.ErrBusy, errors.New(marker)), errors.Join(sqlite.ErrBusy, sqlite.ErrCommitUncertain, errors.New(marker))} {
		err := maintenanceError("backup", cause)
		if strings.Contains(err.Error(), marker) {
			t.Fatal("untrusted error text escaped")
		}
		if errors.Is(cause, sqlite.ErrCommitUncertain) && !errors.Is(err, sqlite.ErrCommitUncertain) {
			t.Fatal("uncertainty lost to BUSY")
		}
	}
}

func cliFixture(t *testing.T, path string, cfg sqlite.EncryptionOptions) sqlite.EncryptionInfo {
	t.Helper()
	db, err := sqlite.Open(path, sqlite.Options{Encryption: &cfg, KeepWALOnClose: cfg.Mode != sqlite.EncryptionSnapshot})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	ctx := context.Background()
	for _, sql := range []string{
		"CREATE TABLE notes(id INTEGER PRIMARY KEY, body TEXT, payload BLOB)",
		"CREATE INDEX notes_body ON notes(body)",
		"INSERT INTO notes VALUES(1,'ordinary 数据库',X'0001ff')",
	} {
		if _, err := db.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	info, err := db.VerifyEncrypted(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return info
}

func checkCLIRestoredRows(t *testing.T, path string, cfg sqlite.EncryptionOptions) sqlite.EncryptionInfo {
	t.Helper()
	db, err := sqlite.Open(path, sqlite.Options{Encryption: &cfg, ReadOnly: true, NoCreate: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	rows, err := db.Query(context.Background(), "SELECT id,body,payload FROM notes WHERE body='ordinary 数据库'")
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() || len(rows.Values()) != 3 || rows.Values()[0] != int64(1) || rows.Values()[1] != "ordinary 数据库" || !bytes.Equal(rows.Values()[2].([]byte), []byte{0, 1, 255}) {
		t.Fatal("restored row mismatch")
	}
	if rows.Next() || rows.Err() != nil {
		t.Fatal("unexpected extra row or query error")
	}
	if err := rows.Close(); err != nil {
		t.Fatal(err)
	}
	info, err := db.VerifyEncrypted(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	return info
}

func testCLIProvider(t *testing.T, descriptor string, input credentialInput) *cliKeyProvider {
	t.Helper()
	p, err := newCLIKeyProvider(descriptor, input)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.close)
	return p
}

func TestCLIMaintenanceCorrectKeyRoundTrips(t *testing.T) {
	// Intact disposable databases, generated correct keys and normal close only.
	for _, tc := range []struct {
		name string
		mode sqlite.EncryptionMode
	}{{"snapshot", sqlite.EncryptionSnapshot}, {"wal", sqlite.EncryptionWAL}, {"shared", sqlite.EncryptionWALShared}} {
		t.Run(tc.name, func(t *testing.T) {
			oldKey, newKey, archiveKey := disposableCLIKey(t), disposableCLIKey(t), disposableCLIKey(t)
			values := map[string]string{"OLD": hex.EncodeToString(oldKey), "NEW": hex.EncodeToString(newKey), "ARCHIVE": hex.EncodeToString(archiveKey)}
			input := credentialInput{lookupEnv: func(name string) (string, bool) { value, ok := values[name]; return value, ok }}
			oldProvider := testCLIProvider(t, "env-hex:OLD", input)
			newProvider := testCLIProvider(t, "env-hex:NEW", input)
			archiveProvider := testCLIProvider(t, "env-hex:ARCHIVE", input)
			oldCfg := sqlite.EncryptionOptions{KeyProvider: oldProvider, CredentialKind: encryption.RawKey256, Mode: tc.mode}
			newCfg := sqlite.EncryptionOptions{KeyProvider: newProvider, CredentialKind: encryption.RawKey256, Mode: tc.mode}
			archiveCfg := sqlite.EncryptionOptions{KeyProvider: archiveProvider, CredentialKind: encryption.RawKey256}
			dir := t.TempDir()
			path, archive, restored := filepath.Join(dir, "source.gse"), filepath.Join(dir, "snapshot.backup"), filepath.Join(dir, "restored.gse")
			before := cliFixture(t, path, oldCfg)
			if tc.mode != sqlite.EncryptionSnapshot && before.WALSegments == 0 {
				t.Fatal("test did not select committed WAL segments")
			}
			var output bytes.Buffer
			if err := runMaintenanceWithInput(context.Background(), "backup", []string{path, "--out", archive, "--key-provider", "env-hex:OLD", "--out-key-provider", "env-hex:ARCHIVE", "--mode", tc.name}, &output, input); err != nil {
				t.Fatal(err)
			}
			var result maintenanceResult
			if err := json.Unmarshal(output.Bytes(), &result); err != nil || result.Status != "ok" || result.Format != "gosqlite-encrypted-archive" || result.RetainsHistory || result.PageCount != before.PageCount {
				t.Fatal("unexpected backup result")
			}
			if _, err := sqlite.RestoreEncrypted(context.Background(), archive, restored, archiveCfg, newCfg); err != nil {
				t.Fatal(err)
			}
			checkCLIRestoredRows(t, restored, newCfg)
			output.Reset()
			if err := runMaintenanceWithInput(context.Background(), "rekey", []string{path, "--key-provider", "env-hex:NEW", "--current-key-provider", "env-hex:OLD", "--mode", tc.name}, &output, input); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(output.Bytes(), &result); err != nil || !result.RetainsHistory || result.Mode != tc.name || result.Operation != "rekey" {
				t.Fatal("unexpected rekey result")
			}
			after := checkCLIRestoredRows(t, path, newCfg)
			if after.DatabaseID != before.DatabaseID || after.CommitSequence != before.CommitSequence+1 || after.WALSegments != 0 {
				t.Fatal("rekey changed identity, skipped sequence or retained selected WAL segments")
			}
			// A stdin source is reused for the default archive credential, once.
			stdinCalls := 0
			input.stdin = func() (io.Reader, error) { stdinCalls++; return strings.NewReader(hex.EncodeToString(newKey)), nil }
			archive2 := filepath.Join(dir, "same-key.backup")
			output.Reset()
			if err := runMaintenanceWithInput(context.Background(), "backup", []string{path, "--out", archive2, "--key-provider", "stdin-hex", "--mode", tc.name}, &output, input); err != nil || stdinCalls != 1 {
				t.Fatal("single stdin credential backup failed", err)
			}
			sourceArchiveCfg := newCfg
			sourceArchiveCfg.Mode = sqlite.EncryptionSnapshot
			restored2 := filepath.Join(dir, "restored2.gse")
			if _, err := sqlite.RestoreEncrypted(context.Background(), archive2, restored2, sourceArchiveCfg, newCfg); err != nil {
				t.Fatal(err)
			}
			checkCLIRestoredRows(t, restored2, newCfg)
		})
	}
}

func TestCLIMaintenancePasswordAndDispatch(t *testing.T) {
	oldKey := disposableCLIKey(t)
	// Generated disposable password; it never appears in command arguments.
	password := hex.EncodeToString(disposableCLIKey(t))
	t.Setenv("GOSQLITE_CLI_TEST_OLD", hex.EncodeToString(oldKey))
	t.Setenv("GOSQLITE_CLI_TEST_NEW", password)
	input := credentialInput{lookupEnv: func(name string) (string, bool) {
		switch name {
		case "OLD":
			return hex.EncodeToString(oldKey), true
		case "NEW":
			return password, true
		}
		return "", false
	}}
	oldProvider := testCLIProvider(t, "env-hex:OLD", input)
	newProvider := testCLIProvider(t, "env-password:NEW", input)
	oldCfg := sqlite.EncryptionOptions{KeyProvider: oldProvider, CredentialKind: encryption.RawKey256}
	newCfg := sqlite.EncryptionOptions{KeyProvider: newProvider, CredentialKind: encryption.PasswordArgon2id}
	dir := t.TempDir()
	path, archive := filepath.Join(dir, "source.gse"), filepath.Join(dir, "snapshot.backup")
	cliFixture(t, path, oldCfg)
	var output bytes.Buffer
	if err := run([]string{"rekey", path, "--key-provider", "env-password:GOSQLITE_CLI_TEST_NEW", "--current-key-provider", "env-hex:GOSQLITE_CLI_TEST_OLD"}, &output); err != nil {
		t.Fatal(err)
	}
	checkCLIRestoredRows(t, path, newCfg)
	if err := run([]string{"backup", path, "--out", archive, "--key-provider", "env-password:GOSQLITE_CLI_TEST_NEW", "--out-key-provider", "env-hex:GOSQLITE_CLI_TEST_OLD"}, &output); err != nil {
		t.Fatal(err)
	}
	restored := filepath.Join(dir, "restored.gse")
	if _, err := sqlite.RestoreEncrypted(context.Background(), archive, restored, oldCfg, oldCfg); err != nil {
		t.Fatal(err)
	}
	checkCLIRestoredRows(t, restored, oldCfg)
	if strings.Contains(output.String(), password) || strings.Contains(output.String(), hex.EncodeToString(oldKey)) {
		t.Fatal("credential appeared in output")
	}
}
