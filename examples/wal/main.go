// The example intentionally requests exclusive WAL. It does not demonstrate
// concurrent SQLite/WAL clients or qualify crash durability.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"

	"github.com/ms-xiaosnow/gosqlite/driver"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func run(ctx context.Context, path string) (err error) {
	c, err := driver.NewConnector(path, sqlite.Options{WALExclusive: true})
	if err != nil {
		return err
	}
	db := sql.OpenDB(c)
	// The first connection retains EXCLUSIVE; another would correctly be BUSY.
	db.SetMaxOpenConns(1)
	defer func() {
		if e := db.Close(); err == nil {
			err = e
		}
	}()
	if _, err = db.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS notes(id INTEGER PRIMARY KEY, text TEXT)"); err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, "INSERT INTO notes(text) VALUES(?)", "正常提交 WAL"); err != nil {
		return err
	}
	return tx.Commit()
}
func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: go run ./examples/wal <disposable.db>")
		os.Exit(2)
	}
	if err := run(context.Background(), os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
