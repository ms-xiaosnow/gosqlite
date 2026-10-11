//go:build linux

package driver_test

import (
	"context"
	"database/sql"
	"errors"
	"net/url"
	"path/filepath"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/driver"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestDriverReadOnlySharedWAL(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "driver-readonly.db")
	keeper, err := sqlite.Open(path, sqlite.Options{WALShared: true, KeepWALOnClose: true})
	if err != nil { t.Fatal(err) }
	defer keeper.Close()
	for _, q := range []string{"CREATE TABLE t(id INTEGER PRIMARY KEY,text TEXT)", "INSERT INTO t VALUES(1,'current')"} {
		if _, err := keeper.Exec(ctx, q); err != nil { t.Fatal(err) }
	}
	for _, mode := range []string{"ro", "rw", "rwc"} {
		u := url.URL{Scheme: "file", Path: path, RawQuery: "mode="+mode}
		// rw/rwc must not upgrade an explicitly readonly configured connector.
		opts := sqlite.Options{WALShared: true, ReadOnly: mode != "ro"}
		connector, err := driver.NewConnector(u.String(), opts)
		if err != nil { t.Fatal(err) }
		db := sql.OpenDB(connector)
		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)
		defer db.Close()
		var value string
		if err := db.QueryRowContext(ctx, "SELECT text FROM t WHERE id=1").Scan(&value); err != nil || value != "current" { t.Fatal(value, err) }
		if _, err := db.ExecContext(ctx, "UPDATE t SET text='forbidden'"); !errors.Is(err, sqlite.ErrReadOnly) { t.Fatal(err) }
		tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
		if err != nil { t.Fatal(err) }
		if err := tx.QueryRowContext(ctx, "SELECT text FROM t WHERE id=1").Scan(&value); err != nil || value != "current" { t.Fatal(value, err) }
		if err := tx.Commit(); err != nil { t.Fatal(err) }
		if err := db.Close(); err != nil { t.Fatal(err) }
	}
	// mode=ro alone still does not infer or activate WALShared.
	u := url.URL{Scheme: "file", Path: path, RawQuery: "mode=ro"}
	connector, err := driver.NewConnector(u.String(), sqlite.Options{})
	if err != nil { t.Fatal(err) }
	if conn, err := connector.Connect(ctx); err == nil { conn.Close(); t.Fatal("unconfigured mode=ro silently enabled WAL") }
	if err := keeper.Close(); err != nil { t.Fatal(err) }
}
