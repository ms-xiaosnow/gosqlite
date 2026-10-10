package driver_test

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/driver"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestWALConnectorSingleConnection(t *testing.T) {
	for _, size := range []uint32{512, 4096, 65536} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "driver-wal.db")
			connector, err := driver.NewConnector(path, sqlite.Options{WALExclusive: true, PageSize: size})
			if err != nil {
				t.Fatal(err)
			}
			db := sql.OpenDB(connector)
			db.SetMaxOpenConns(1)
			defer db.Close()
			ctx := context.Background()
			if _, err = db.ExecContext(ctx, "CREATE TABLE t(id INTEGER PRIMARY KEY, value TEXT)"); err != nil {
				t.Fatal(err)
			}
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			st, err := tx.PrepareContext(ctx, "INSERT INTO t VALUES(?,?)")
			if err != nil {
				t.Fatal(err)
			}
			for i := 1; i <= 40; i++ {
				if _, err = st.ExecContext(ctx, i, fmt.Sprint("文", i)); err != nil {
					t.Fatal(err)
				}
			}
			if err = st.Close(); err != nil {
				t.Fatal(err)
			}
			if err = tx.Commit(); err != nil {
				t.Fatal(err)
			}
			var got string
			if err = db.QueryRowContext(ctx, "SELECT value FROM t WHERE id=?", 23).Scan(&got); err != nil || got != "文23" {
				t.Fatalf("%q %v", got, err)
			}
			ro, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
			if err != nil {
				t.Fatal(err)
			}
			if err = ro.QueryRowContext(ctx, "SELECT value FROM t WHERE id=1").Scan(&got); err != nil {
				t.Fatal(err)
			}
			if err = ro.Commit(); err != nil {
				t.Fatal(err)
			}
			if err = db.Close(); err != nil {
				t.Fatal(err)
			}
		})
	}
}
