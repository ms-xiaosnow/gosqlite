// Run against a new disposable path: go run ./examples/vector example.db
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/ms-xiaosnow/gosqlite/extension/similarity"
	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// hybridHit retains the original text/vector measures as well as the exact
// reciprocal-rank contributions. A missing source measure is explicitly NULL.
type hybridHit struct {
	RowID          int64                     `json:"rowid"`
	TextScore      *similarity.Score         `json:"text_score"`
	VectorDistance *float64                  `json:"vector_distance"`
	FusedScore     float64                   `json:"fused_score"`
	Contributions  []similarity.Contribution `json:"contributions"`
}
type hybridOutput struct {
	Text     sqlite.SimilarityResult `json:"text_ranking"`
	Vector   sqlite.VectorResult     `json:"vector_ranking"`
	Hits     []hybridHit             `json:"fused_hits"`
	SQLPlan  sqlite.QueryPlan        `json:"sql_plan"`
	SQLStats sqlite.QueryStats       `json:"sql_stats"`
	SQLRows  [][]any                 `json:"sql_rows"`
}

func example(ctx context.Context, path string, out io.Writer) (err error) {
	db, err := sqlite.OpenContext(ctx, path, sqlite.Options{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	for _, q := range []string{
		"CREATE TABLE docs(id INTEGER PRIMARY KEY,title TEXT,embedding VECTOR(3),active INTEGER)",
		"INSERT INTO docs VALUES(1,'数据库查询','[1,0,1]',1),(2,'数据检索','[1,1,0]',1),(3,'向量搜索','[0,1,1]',1),(4,'数据库管理','[1,0,0]',0),(5,'ordinary text','[0,0,1]',1),(6,'extra text',NULL,1),(7,'unrelated','[3,2,1]',1),(8,'plain','[2,4,1]',1)",
		"CREATE INDEX docs_grams ON docs(title) USING grams13",
	} {
		if _, err = db.Exec(ctx, q); err != nil {
			return err
		}
	}
	// Both rank lists are complete for their requested Top-K and share one read
	// transaction, so concurrent commits cannot mix text and vector snapshots.
	tx, err := db.Begin(ctx, sqlite.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	text, err := tx.SimilaritySearch(ctx, "docs_grams", "title", "数据库", sqlite.SimilarityOptions{MinGram: 2, MaxGram: 2, Metric: similarity.Dice, Threshold: 0, Limit: 6, Where: "active=1"})
	if err != nil {
		return err
	}
	query, err := vector.New([]float32{1, 0, 1})
	if err != nil {
		return err
	}
	vectors, err := tx.VectorSearch(ctx, "docs", "embedding", query, sqlite.VectorSearchOptions{Metric: vector.Cosine, Limit: 6, Where: "active=1"})
	if err != nil {
		return err
	}
	textList := similarity.RankedList{Name: "title_dice", Weight: 1}
	vectorList := similarity.RankedList{Name: "embedding_cosine", Weight: 1}
	rawText, rawDistance := map[int64]similarity.Score{}, map[int64]float64{}
	for _, hit := range text.Hits {
		textList.RowIDs = append(textList.RowIDs, hit.RowID)
		rawText[hit.RowID] = hit.Score
	}
	for _, hit := range vectors.Hits {
		vectorList.RowIDs = append(vectorList.RowIDs, hit.RowID)
		rawDistance[hit.RowID] = hit.Distance
	}
	engine, err := similarity.New(similarity.DefaultConfig())
	if err != nil {
		return err
	}
	fused, err := engine.Fuse(ctx, []similarity.RankedList{textList, vectorList}, 60, 3)
	if err != nil {
		return err
	}
	output := hybridOutput{Text: text, Vector: vectors}
	for _, hit := range fused {
		item := hybridHit{RowID: hit.RowID, FusedScore: hit.Score, Contributions: hit.Contributions}
		if value, ok := rawText[hit.RowID]; ok {
			item.TextScore = &value
		}
		if value, ok := rawDistance[hit.RowID]; ok {
			item.VectorDistance = &value
		}
		output.Hits = append(output.Hits, item)
	}
	// SQL combines actual grams candidates, a business predicate, raw text score,
	// and exact dense ranking. It does not claim a vector ANN access path.
	rows, err := tx.Query(ctx, "SELECT id,title,bigram_similarity(title,'数据库') AS text_score,embedding <=> ? AS distance FROM docs WHERE title LIKE '%数据%' AND active=1 ORDER BY distance LIMIT 3", query)
	if err != nil {
		return err
	}
	output.SQLPlan = rows.Plan()
	for rows.Next() {
		output.SQLRows = append(output.SQLRows, rows.Values())
	}
	output.SQLStats = rows.Stats()
	if err = errors.Join(rows.Err(), rows.Close()); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(output)
}
func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: vector <new-disposable-db>")
		os.Exit(2)
	}
	if err := example(context.Background(), os.Args[1], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
