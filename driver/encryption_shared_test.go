package driver_test

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/driver"
	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestEncryptedSharedConnectorIndependentPoolSnapshots(t *testing.T) {
	ctx := context.Background()
	key := make([]byte, 32)
	if _, e := rand.Read(key); e != nil {
		t.Fatal(e)
	}
	defer clear(key)
	cfg := sqlite.EncryptionOptions{KeyProvider: encryptedProvider(key), CredentialKind: encryption.RawKey256, Mode: sqlite.EncryptionWALShared}
	connector, e := driver.NewConnector(filepath.Join(t.TempDir(), "pool.gse"), sqlite.Options{Encryption: &cfg, KeepWALOnClose: true})
	if e != nil {
		t.Fatal(e)
	}
	cfg.Mode = 99
	cfg.KeyProvider = nil
	cfg.WALLimits.MaxSegments = 1
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(2)
	defer db.Close()
	a, e := db.Conn(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	for _, q := range []string{"CREATE TABLE t(id INTEGER PRIMARY KEY,v TEXT)", "INSERT INTO t VALUES(1,'old')"} {
		if _, e = a.ExecContext(ctx, q); e != nil {
			t.Fatal(e)
		}
	}
	b, e := db.Conn(ctx)
	if e != nil {
		t.Fatal("independent pooled connection", e)
	}
	defer b.Close()
	tx, e := a.BeginTx(ctx, nil)
	if e != nil {
		t.Fatal(e)
	}
	var got string
	if e = tx.QueryRowContext(ctx, "SELECT v FROM t").Scan(&got); e != nil || got != "old" {
		t.Fatal(got, e)
	}
	if _, e = b.ExecContext(ctx, "UPDATE t SET v='new'"); e != nil {
		t.Fatal(e)
	}
	if e = tx.QueryRowContext(ctx, "SELECT v FROM t").Scan(&got); e != nil || got != "old" {
		t.Fatal("old snapshot", got, e)
	}
	if _, e = tx.ExecContext(ctx, "UPDATE t SET v='stale'"); !errors.Is(e, sqlite.ErrBusySnapshot) {
		t.Fatal(e)
	}
	if e = tx.Rollback(); e != nil {
		t.Fatal(e)
	}
	if e = a.QueryRowContext(ctx, "SELECT v FROM t").Scan(&got); e != nil || got != "new" {
		t.Fatal("fresh snapshot", got, e)
	}
	// Return an explicit SQL transaction to the pool. ResetSession must roll it
	// back and release this connection's writer before reusing pooled handles.
	if _, e = a.ExecContext(ctx, "BEGIN IMMEDIATE"); e != nil {
		t.Fatal(e)
	}
	if e = a.Close(); e != nil {
		t.Fatal(e)
	}
	a, e = db.Conn(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	if _, e = b.ExecContext(ctx, "UPDATE t SET v='reset-released'"); e != nil {
		t.Fatal("pool reset writer ownership", e)
	}
}
