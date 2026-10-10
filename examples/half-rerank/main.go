// Run with a new disposable database path. This example executes ordinary
// SQLite-compatible storage, exact binary16 candidates, and dense reranking.
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
	CandidateMetric vector.MetricKind `json:"candidate_metric"`
	CandidateLimit  int               `json:"candidate_limit"`
	Candidates      sqlite.HalfResult `json:"exact_half_candidates"`
	FinalMetric     vector.MetricKind `json:"final_metric"`
	Reranked        []vector.Hit      `json:"dense_reranked_top1"`
	RecallAt1       float64           `json:"observed_dense_recall_at_1"`
}

type output struct {
	QueryConversion vector.HalfConversion `json:"query_binary32_to_binary16_loss"`
	QuantizedQuery  string                `json:"binary_quantized_half_query"`
	ExactDense      sqlite.VectorResult   `json:"exact_dense_reference"`
	Runs            []rerankRun           `json:"runs"`
	SQLSubvector    string                `json:"sql_subvector_of_query"`
	Caveat          string                `json:"caveat"`
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
	if _, err = tx.Exec(ctx, "CREATE TABLE items(id INTEGER PRIMARY KEY,embedding VECTOR(4),half HALFVEC(4),active INTEGER)"); err != nil {
		return err
	}
	// Static fixture: both representations are populated together. This is not
	// a generated column; applications must maintain future changes explicitly.
	for _, item := range []struct {
		id     int64
		text   string
		active int
	}{{10, "[1.0004,1,-1e-8,0]", 1}, {20, "[1.00021,1,-1e-8,0]", 1}, {5, "[1.0002,1,-1e-8,0]", 0}} {
		if _, err = tx.Exec(ctx, "INSERT INTO items VALUES(?,?,halfvec_from_vector(?),?)", item.id, item.text, item.text, item.active); err != nil {
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
	q, err := vector.Parse("[1.0002,1,-1e-8,0]")
	if err != nil {
		return err
	}
	half, report, err := vector.HalfFromVector(q)
	if err != nil {
		return err
	}
	exact, err := tx.VectorSearch(ctx, "items", "embedding", q, sqlite.VectorSearchOptions{Metric: vector.L2, Limit: 1, Where: "active=1"})
	if err != nil {
		return err
	}
	quantized, err := vector.HalfBinaryQuantize(half)
	if err != nil {
		return err
	}
	result := output{QueryConversion: report, QuantizedQuery: quantized.String(), ExactDense: exact, Caveat: "Exact binary16 L2 candidates are a full scan, not ANN. Rounding collapses distinct dense values into tied half values; candidate truncation loses the nearest dense neighbor in this fixture. Exact dense reranking cannot restore missing candidates. The materialized half column is a static fixture, not automatically maintained."}
	rows, err := tx.Query(ctx, "SELECT halfvec_to_text(subvector(?,1,2))", half)
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
		candidates, err := tx.HalfSearch(ctx, "items", "half", half, sqlite.HalfSearchOptions{Metric: vector.L2, Limit: limit, Where: "active=1"})
		if err != nil {
			return err
		}
		run := rerankRun{CandidateMetric: vector.L2, CandidateLimit: limit, Candidates: candidates, FinalMetric: vector.L2}
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
		fmt.Fprintln(os.Stderr, "usage: half-rerank <new-disposable-db>")
		os.Exit(2)
	}
	if err := example(context.Background(), os.Args[1], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
