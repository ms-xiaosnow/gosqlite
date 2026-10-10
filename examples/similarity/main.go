// Run against a new disposable path: go run ./examples/similarity example.db
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/ms-xiaosnow/gosqlite/extension/similarity"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func example(ctx context.Context, path string) (err error) {
	db, err := sqlite.OpenContext(ctx, path, sqlite.Options{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	for _, q := range []string{
		"CREATE TABLE docs(id INTEGER PRIMARY KEY,title TEXT,body TEXT)",
		"INSERT INTO docs VALUES(1,'数据库查询','数据索引'),(2,'数据检索','数据库查询'),(3,'普通文字','其他内容'),(4,'emoji🙂','plain'),(5,'none','none')",
		"CREATE INDEX docs_grams ON docs(title,body) USING grams13",
	} {
		if _, err = db.Exec(ctx, q); err != nil {
			return err
		}
	}
	options := sqlite.SimilarityOptions{MinGram: 2, MaxGram: 2, Metric: similarity.Dice, Threshold: 0.1, Limit: 3}
	var lists []similarity.RankedList
	for _, field := range []string{"title", "body"} {
		result, e := db.SimilaritySearch(ctx, "docs_grams", field, "数据库", options)
		if e != nil {
			return e
		}
		fmt.Println(result.Plan.Detail())
		if e = json.NewEncoder(os.Stdout).Encode(result); e != nil {
			return e
		}
		list := similarity.RankedList{Name: field, Weight: 1}
		for _, hit := range result.Hits {
			list.RowIDs = append(list.RowIDs, hit.RowID)
		}
		lists = append(lists, list)
	}
	engine, e := similarity.New(similarity.DefaultConfig())
	if e != nil {
		return e
	}
	fused, e := engine.Fuse(ctx, lists, 60, 3)
	if e != nil {
		return e
	}
	fmt.Println("Weighted reciprocal-rank fusion of the two supplied text rankings:")
	return json.NewEncoder(os.Stdout).Encode(fused)
}
func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: similarity <new-disposable-db>")
		os.Exit(2)
	}
	if err := example(context.Background(), os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
