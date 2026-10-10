// Run with a new disposable database path. This example executes ordinary
// SQLite-compatible storage, exact packed-bit candidates, and dense reranking.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

type rerankRun struct {
	CandidateMetric vector.BitMetricKind `json:"candidate_metric"`
	CandidateLimit  int                  `json:"candidate_limit"`
	Candidates      sqlite.BitResult     `json:"exact_bit_candidates"`
	FinalMetric     vector.MetricKind    `json:"final_metric"`
	Reranked        []vector.Hit         `json:"dense_reranked_top1"`
	RecallAt1       float64              `json:"observed_dense_recall_at_1"`
}

type output struct {
	ExactDense   sqlite.VectorResult `json:"exact_dense_reference"`
	Runs         []rerankRun         `json:"runs"`
	SQLSubvector string              `json:"sql_subvector_of_query"`
	Caveat       string              `json:"caveat"`
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
	if _, err = tx.Exec(ctx, "CREATE TABLE items(id INTEGER PRIMARY KEY,embedding VECTOR(4),bits BIT(4),active INTEGER)"); err != nil {
		return err
	}
	// Static fixture: both representations are populated together. This is not
	// a generated column; applications must maintain future changes explicitly.
	for _, item := range []struct {
		id     int64
		text   string
		active int
	}{{10, "[-100,100,0,0]", 1}, {20, "[0.001,1,0,0]", 1}, {5, "[-0.001,1,0,0]", 0}} {
		if _, err = tx.Exec(ctx, "INSERT INTO items VALUES(?,?,binary_quantize(?),?)", item.id, item.text, item.text, item.active); err != nil {
			return err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	tx, err = db.Begin(ctx, sqlite.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	q, err := vector.Parse("[-0.001,1,0,0]")
	if err != nil {
		return err
	}
	bits, err := vector.BinaryQuantize(q)
	if err != nil {
		return err
	}
	exact, err := tx.VectorSearch(ctx, "items", "embedding", q, sqlite.VectorSearchOptions{Metric: vector.L2, Limit: 1, Where: "active=1"})
	if err != nil {
		return err
	}
	result := output{ExactDense: exact, Caveat: "Exact Hamming candidates are a full scan, not ANN. Candidate truncation can lose the nearest dense neighbor; exact dense reranking does not restore missing candidates. The materialized bit column is a static fixture, not automatically maintained."}
	rows, err := tx.Query(ctx, "SELECT vector_to_text(subvector(?,1,2))", q)
	if err != nil {
		return err
	}
	if !rows.Next() {
		return errors.Join(fmt.Errorf("missing subvector result"), rows.Err(), rows.Close())
	}
	result.SQLSubvector = rows.Values()[0].(string)
	if err = rows.Close(); err != nil {
		return err
	}
	for _, limit := range []int{1, 2} {
		candidates, err := tx.BitSearch(ctx, "items", "bits", bits, sqlite.BitSearchOptions{Metric: vector.Hamming, Limit: limit, Where: "active=1"})
		if err != nil {
			return err
		}
		run := rerankRun{CandidateMetric: vector.Hamming, CandidateLimit: limit, Candidates: candidates, FinalMetric: vector.L2}
		for _, hit := range candidates.Hits {
			// Same transaction, row identity, filter and original dense value. SQL
			// computes the final metric; unsupported nested-query syntax is not used.
			rows, err := tx.Query(ctx, "SELECT id,embedding <-> ? FROM items WHERE id=? AND active=1", q, hit.RowID)
			if err != nil {
				return err
			}
			if !rows.Next() {
				return errors.Join(fmt.Errorf("candidate is not visible in snapshot"), rows.Err(), rows.Close())
			}
			values := rows.Values()
			run.Reranked = append(run.Reranked, vector.Hit{RowID: values[0].(int64), Distance: values[1].(float64)})
			if err = rows.Close(); err != nil {
				return err
			}
		}
		sort.Slice(run.Reranked, func(i, j int) bool {
			a, b := run.Reranked[i], run.Reranked[j]
			return a.Distance < b.Distance || a.Distance == b.Distance && a.RowID < b.RowID
		})
		run.Reranked = run.Reranked[:min(1, len(run.Reranked))]
		if len(run.Reranked) == 1 && run.Reranked[0].RowID == exact.Hits[0].RowID {
			run.RecallAt1 = 1
		}
		result.Runs = append(result.Runs, run)
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(result)
}

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: bit-rerank <new-disposable-db>")
		os.Exit(2)
	}
	if err := example(context.Background(), os.Args[1], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
