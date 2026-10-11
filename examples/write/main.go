// Command write demonstrates ordinary native Go SQLite CRUD and transactions.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"os"

	_ "github.com/ms-xiaosnow/gosqlite/driver"
)

// example returns its message only after the transaction and database retire.
// A named error retains cleanup failures alongside the operation's error.
func example(ctx context.Context, path string) (message string, err error) {
	db, err := sql.Open("gosqlite", path)
	if err != nil {
		return "", err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	if _, err = db.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS docs(id INTEGER PRIMARY KEY,title TEXT NOT NULL)"); err != nil {
		return "", err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer func() {
		// database/sql marks a committed transaction done. Its deferred
		// Rollback then returns sql.ErrTxDone without calling the driver.
		if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
			err = errors.Join(err, rollbackErr)
		}
	}()
	result, err := tx.ExecContext(ctx, "INSERT INTO docs(title) VALUES(?)", "数据库检索")
	if err != nil {
		return "", err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return "", err
	}
	var title string
	if err = tx.QueryRowContext(ctx, "SELECT title FROM docs WHERE id=?", id).Scan(&title); err != nil {
		return "", err
	}
	if err = tx.Commit(); err != nil {
		return "", err
	}
	return fmt.Sprintf("inserted %d: %s\n", id, title), nil
}

// run keeps argument, output and exit-status handling outside database ownership.
func run(args []string, out io.Writer, runExample func(context.Context, string) (string, error)) (int, error) {
	if len(args) != 1 {
		return 1, errors.New("usage: go run ./examples/write <database-path>")
	}
	message, err := runExample(context.Background(), args[0])
	if err != nil {
		return 1, err
	}
	if _, err := fmt.Fprint(out, message); err != nil {
		return 1, err
	}
	return 0, nil
}

func main() {
	status, err := run(os.Args[1:], os.Stdout, example)
	if err != nil {
		if _, reportErr := fmt.Fprintln(os.Stderr, err); reportErr != nil {
			status = 1
		}
	}
	os.Exit(status)
}
