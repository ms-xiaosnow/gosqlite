package driver_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/driver"
	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

type encryptedProvider []byte
type encryptedMaterial []byte

func (p encryptedProvider) Acquire(context.Context, encryption.KeyRequest) (encryption.KeyMaterial, error) {
	return encryptedMaterial(bytes.Clone(p)), nil
}
func (m encryptedMaterial) Kind() encryption.CredentialKind { return encryption.RawKey256 }
func (m encryptedMaterial) Bytes() []byte                   { return m }
func (m encryptedMaterial) Release()                        { clear(m) }
func TestEncryptedConnectorOrdinaryLifecycle(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	defer clear(key)
	cfg := sqlite.EncryptionOptions{KeyProvider: encryptedProvider(key), CredentialKind: encryption.RawKey256}
	path := filepath.Join(t.TempDir(), "driver.gse")
	connector, err := driver.NewConnector("file:"+path, sqlite.Options{Encryption: &cfg})
	if err != nil {
		t.Fatal(err)
	}
	// NewConnector must snapshot the option struct, never retain its pointer.
	cfg.KeyProvider = nil
	cfg.Mode = 99
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(1)
	defer db.Close()
	if _, err = db.Exec("CREATE TABLE records(id INTEGER PRIMARY KEY,body TEXT)"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec("INSERT INTO records VALUES(1,?)", "encrypted database/sql"); err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec("UPDATE records SET body='discarded' WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	var got string
	if err = db.QueryRow("SELECT body FROM records WHERE id=1").Scan(&got); err != nil || got != "encrypted database/sql" {
		t.Fatal(got, err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	cfg = sqlite.EncryptionOptions{KeyProvider: encryptedProvider(key), CredentialKind: encryption.RawKey256}
	connector, err = driver.NewConnector("file:"+path+"?mode=ro", sqlite.Options{Encryption: &cfg})
	if err != nil {
		t.Fatal(err)
	}
	db = sql.OpenDB(connector)
	db.SetMaxOpenConns(1)
	defer db.Close()
	if err = db.QueryRow("SELECT body FROM records WHERE id=1").Scan(&got); err != nil || got != "encrypted database/sql" {
		t.Fatal(got, err)
	}
}
func TestEncryptedConnectorRejectsSecretDSNWithoutFiles(t *testing.T) {
	dir := t.TempDir()
	for _, suffix := range []string{"?key=synthetic", "?password=synthetic", "?mode=rwc&kdf=synthetic"} {
		if _, err := driver.NewConnector("file:"+filepath.Join(dir, "reject.gse")+suffix, sqlite.Options{}); err == nil {
			t.Fatal("secret DSN parameter accepted")
		}
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 0 {
		t.Fatal(files, err)
	}
}
