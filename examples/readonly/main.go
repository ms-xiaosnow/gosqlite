// Read an existing supported SQLite database without CGO.
// Usage: go run ./examples/readonly ./testdata/readonly/page4096.db
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/ms-xiaosnow/gosqlite/driver"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: readonly <existing.db>")
	}
	db, err := sql.Open("gosqlite", os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	rows, err := db.QueryContext(context.Background(), "SELECT id, label FROM samples WHERE id >= ? ORDER BY id LIMIT ?", int64(1), 3)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var id int64
		var label string
		if err := rows.Scan(&id, &label); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%d\t%s\n", id, label)
	}
	if err := rows.Err(); err != nil {
		log.Fatal(err)
	}
}
