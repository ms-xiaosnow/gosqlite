// Read an existing supported SQLite database without CGO.
// Usage: go run ./examples/readonly ./testdata/readonly/page4096.db
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"

	"github.com/ms-xiaosnow/gosqlite/driver"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func example(ctx context.Context, input string, out io.Writer) (resultErr error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	// URI modes may further restrict these options, but cannot make this
	// example writable or allow it to create a database.
	connector, err := driver.NewConnector(input, sqlite.Options{ReadOnly: true, NoCreate: true})
	if err != nil {
		return err
	}
	db := sql.OpenDB(connector)
	defer func() { resultErr = errors.Join(resultErr, db.Close()) }()
	rows, err := db.QueryContext(ctx, "SELECT id, label FROM samples WHERE id >= ? ORDER BY id LIMIT ?", int64(1), 3)
	if err != nil {
		return err
	}
	// Registered after the DB cleanup, so rows are closed first on every
	// return path. Retain iteration errors even when output stops early.
	defer func() { resultErr = errors.Join(resultErr, rows.Close(), rows.Err()) }()
	for rows.Next() {
		var id int64
		var label string
		if err := rows.Scan(&id, &label); err != nil {
			return err
		}
		if err := writeRow(out, id, label); err != nil {
			return err
		}
	}
	return nil
}

func writeRow(out io.Writer, id int64, label string) error {
	_, err := fmt.Fprintf(out, "%d\t%s\n", id, label)
	return err
}

func run(ctx context.Context, args []string, out io.Writer) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: readonly <existing.db>")
	}
	return example(ctx, args[0], out)
}

// reportResult runs after example has released its resources. Keeping the
// diagnostic write error in the result also makes reporting independently usable.
func reportResult(err error, errOut io.Writer) (int, error) {
	if err == nil {
		return 0, nil
	}
	_, writeErr := fmt.Fprintln(errOut, "readonly:", err)
	return 1, errors.Join(err, writeErr)
}

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	err := run(ctx, os.Args[1:], os.Stdout)
	stop()
	if status, err := reportResult(err, os.Stderr); err != nil {
		os.Exit(status)
	}
}
