// Run with a new disposable path. No PostgreSQL process, network or ANN is used.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

type output struct {
	Dimensions        int          `json:"dimensions"`
	Nonzeros          int          `json:"query_nonzeros"`
	EncodedBytes      int          `json:"query_gss_bytes"`
	DenseConversion   string       `json:"explicit_dense_conversion"`
	Hits              []vector.Hit `json:"exact_l2_hits"`
	NativePlan        string       `json:"native_plan"`
	SourceRows        int64        `json:"native_source_rows"`
	ScoredNonzeros    int64        `json:"native_support_work"`
	SQLHits           []vector.Hit `json:"sql_exact_l2_hits"`
	SQLRepresentation string       `json:"sql_representation"`
	SQLRanking        string       `json:"sql_ranking"`
	SQLNonzeros       int64        `json:"sql_support_work"`
}

func example(ctx context.Context, path string, out io.Writer) (err error) {
	if _, err = os.Stat(path); err == nil {
		return fmt.Errorf("example requires a new database path")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	db, err := sqlite.OpenContext(ctx, path, sqlite.Options{})
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, db.Close()) }()
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec(ctx, "CREATE TABLE items(id INTEGER PRIMARY KEY,s SPARSEVEC(1000000000),active INTEGER)"); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, "INSERT INTO items VALUES(10,'{1:1,999999999:2}/1000000000',1),(20,'{1:2,999999999:2}/1000000000',1),(30,'{2:3}/1000000000',1),(5,'{1:1,999999999:2}/1000000000',0),(40,NULL,1)"); err != nil {
		return err
	}
	q, err := vector.ParseSparse("{1:1,999999999:2}/1000000000")
	if err != nil {
		return err
	}
	encoded, err := q.MarshalBinary()
	if err != nil {
		return err
	}
	if _, err = q.Dense(); !errors.Is(err, vector.ErrLimit) {
		return fmt.Errorf("expected bounded dense conversion refusal: %v", err)
	}
	// Both queries see the same transaction's uncommitted writes.
	native, err := tx.SparseSearch(ctx, "items", "s", q, sqlite.SparseSearchOptions{Metric: vector.L2, Limit: 3, Where: "active=1"})
	if err != nil {
		return err
	}
	result := output{Dimensions: q.Dims(), Nonzeros: q.NNZ(), EncodedBytes: len(encoded), DenseConversion: "rejected before dimension-sized allocation: dense output cap is 16000", Hits: native.Hits, NativePlan: native.Plan.Detail(), SourceRows: native.Stats.BaseRowFetches, ScoredNonzeros: native.Stats.ScoredNonzeros}
	rows, err := tx.Query(ctx, "SELECT id,s <-> ? AS distance FROM items WHERE active=1 AND s IS NOT NULL ORDER BY distance LIMIT 3", q)
	if err != nil {
		return err
	}
	for rows.Next() {
		v := rows.Values()
		result.SQLHits = append(result.SQLHits, vector.Hit{RowID: v[0].(int64), Distance: v[1].(float64)})
	}
	result.SQLRepresentation = rows.Plan().VectorRepresentation
	result.SQLRanking = rows.Plan().VectorRanking
	result.SQLNonzeros = rows.Stats().VectorRankNonzeros
	if err = errors.Join(rows.Err(), rows.Close()); err != nil {
		return err
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	encoder := json.NewEncoder(out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}
func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: sparse-exact NEW_DATABASE_PATH")
		return
	}
	if err := example(context.Background(), os.Args[1], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
