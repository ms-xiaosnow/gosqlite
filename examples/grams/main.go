// Run against a new disposable path: go run ./examples/grams example.db
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func example(ctx context.Context, path string) (err error) {
	db, err := sqlite.OpenContext(ctx, path, sqlite.Options{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	for _, q := range []string{
		"CREATE TABLE docs(id INTEGER PRIMARY KEY,title TEXT NOT NULL)",
		"INSERT INTO docs VALUES(1,'数据库查询'),(2,'普通文本'),(3,'全文检索'),(4,'emoji🙂'),(5,'其他内容')",
		"CREATE INDEX docs_grams ON docs(title) USING grams13",
	} {
		if _, err = db.Exec(ctx, q); err != nil {
			return err
		}
	}
	for _, needle := range []string{"库", "数据", "数据库"} {
		rows, e := db.Query(ctx, "SELECT id,title FROM docs WHERE title LIKE ? ORDER BY id", "%"+needle+"%")
		if e != nil {
			return e
		}
		fmt.Println(rows.Plan().Detail())
		for rows.Next() {
			if e = json.NewEncoder(os.Stdout).Encode(rows.Values()); e != nil {
				rows.Close()
				return e
			}
		}
		e = errors.Join(rows.Err(), rows.Close())
		if e != nil {
			return e
		}
		fmt.Printf("actual work: %+v\n", rows.Stats())
	}
	return db.CheckExtensionIndex(ctx, "docs_grams")
}
func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: grams <new-disposable-db>")
		os.Exit(2)
	}
	if err := example(context.Background(), os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
