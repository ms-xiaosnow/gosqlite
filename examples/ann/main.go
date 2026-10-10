// Run on a new disposable path: go run ./examples/ann example.db
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

type output struct {
	Exact       sqlite.VectorResult `json:"exact"`
	Approximate sqlite.VectorResult `json:"approximate"`
	SQLPlan     sqlite.QueryPlan    `json:"sql_plan"`
	SQLStats    sqlite.QueryStats   `json:"sql_stats"`
	SQLRows     [][]any             `json:"sql_rows"`
}

func example(ctx context.Context, path string, out io.Writer) (err error) {
	db, err := sqlite.OpenContext(ctx, path, sqlite.Options{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	if _, err = db.Exec(ctx, "CREATE TABLE items(id INTEGER PRIMARY KEY,embedding VECTOR(3) NOT NULL,active INTEGER)"); err != nil {
		return err
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for i := 1; i <= 128; i++ {
		if _, err = tx.Exec(ctx, "INSERT INTO items VALUES(?,?,?)", i, fmt.Sprintf("[%d,%d,1]", i, i%7), i%2); err != nil {
			return err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	if _, err = db.Exec(ctx, "CREATE INDEX items_hnsw ON items(embedding) USING hnsw WITH(opclass='vector_l2_ops',m=16,ef_construction=64)"); err != nil {
		return err
	}
	tx, err = db.Begin(ctx, sqlite.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var result output
	result.Exact, err = tx.VectorSearch(ctx, "items", "embedding", "[21,0,1]", sqlite.VectorSearchOptions{Limit: 3, Where: "active=1"})
	if err != nil {
		return err
	}
	result.Approximate, err = tx.VectorSearch(ctx, "items", "embedding", "[21,0,1]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Index: "items_hnsw", Limit: 3, Where: "active=1"})
	if err != nil {
		return err
	}
	rows, err := tx.Query(ctx, "SELECT id,embedding <-> '[21,0,1]' AS distance FROM items WHERE active=1 ORDER BY distance LIMIT 3")
	if err != nil {
		return err
	}
	for rows.Next() {
		result.SQLRows = append(result.SQLRows, append([]any(nil), rows.Values()...))
	}
	err = errors.Join(rows.Err(), rows.Close())
	if err != nil {
		return err
	}
	result.SQLPlan = rows.Plan()
	result.SQLStats = rows.Stats()
	if err = tx.Rollback(); err != nil {
		return err
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}
func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: ann NEW_DISPOSABLE_DB")
		return
	}
	if err := example(context.Background(), os.Args[1], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
