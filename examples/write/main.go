// Command write demonstrates ordinary native Go SQLite CRUD and transactions.
package main

import (
	"context"
	"database/sql"
	"fmt"
	_ "github.com/ms-xiaosnow/gosqlite/driver"
	"log"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: go run ./examples/write <database-path>")
	}
	db, err := sql.Open("gosqlite", os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if _, err = db.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS docs(id INTEGER PRIMARY KEY,title TEXT NOT NULL)"); err != nil {
		log.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, "INSERT INTO docs(title) VALUES(?)", "数据库检索")
	if err != nil {
		log.Fatal(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		log.Fatal(err)
	}
	var title string
	if err = tx.QueryRowContext(ctx, "SELECT title FROM docs WHERE id=?", id).Scan(&title); err != nil {
		log.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("inserted %d: %s\n", id, title)
}
