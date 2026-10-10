// A real persisted typed HNSW candidate stage followed by original dense
// reranking, all within the same ordinary read snapshot.
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

type run struct {
	Representation  string            `json:"candidate_representation"`
	CandidateLimit  int               `json:"candidate_limit"`
	CandidateMetric string            `json:"candidate_metric"`
	CandidatePlan   sqlite.QueryPlan  `json:"candidate_plan"`
	CandidateStats  sqlite.QueryStats `json:"candidate_stats"`
	CandidateHits   []vector.Hit      `json:"candidate_hits"`
	CandidateExact  bool              `json:"candidate_recall_exact"`
	DenseReranked   []vector.Hit      `json:"dense_l2_reranked_top1"`
	ExactDense      []vector.Hit      `json:"exact_dense_top1"`
	RecallAt1       float64           `json:"observed_dense_recall_at_1"`
}
type report struct {
	Runs   []run  `json:"runs"`
	Caveat string `json:"caveat"`
}

func example(ctx context.Context, path string, out io.Writer) (err error) {
	if _, e := os.Stat(path); e == nil {
		return fmt.Errorf("example requires a new database path")
	} else if !errors.Is(e, os.ErrNotExist) {
		return e
	}
	db, err := sqlite.OpenContext(ctx, path, sqlite.Options{})
	if err != nil {
		return err
	}
	defer func() {
		if db != nil {
			err = errors.Join(err, db.Close())
		}
	}()
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, kind := range []string{"halfvec", "bit"} {
		declaration, transform := "HALFVEC(4)", "halfvec_from_vector"
		if kind == "bit" {
			declaration, transform = "BIT(4)", "binary_quantize"
		}
		table := kind + "_items"
		if _, err = tx.Exec(ctx, "CREATE TABLE "+table+"(id INTEGER PRIMARY KEY,original VECTOR(4) NOT NULL,candidate "+declaration+" NOT NULL,active INTEGER)"); err != nil {
			return err
		}
		texts := []string{"[1.0004,1,-1e-8,0]", "[1.00021,1,-1e-8,0]", "[1.0002,1,-1e-8,0]"}
		if kind == "bit" {
			texts = []string{"[-100,100,0,0]", "[0.001,1,0,0]", "[-0.001,1,0,0]"}
		}
		for i, id := range []int{10, 20, 5} {
			active := 1
			if id == 5 {
				active = 0
			}
			if _, err = tx.Exec(ctx, "INSERT INTO "+table+" VALUES(?,?,"+transform+"(?),?)", id, texts[i], texts[i], active); err != nil {
				return err
			}
		}
		for i := 0; i < 48; i++ {
			text := fmt.Sprintf("[-%d,-10,1,1]", 10+i)
			if kind == "bit" {
				patterns := []int{0, 1, 2, 3, 5, 6, 7, 8, 9, 10, 11, 13, 14, 15}
				p := patterns[i%len(patterns)]
				v := make([]int, 4)
				for j := range v {
					v[j] = -(10 + i)
					if p&(1<<(3-j)) != 0 {
						v[j] = -v[j]
					}
				}
				text = fmt.Sprintf("[%d,%d,%d,%d]", v[0], v[1], v[2], v[3])
			}
			if _, err = tx.Exec(ctx, "INSERT INTO "+table+" VALUES(?,?,"+transform+"(?),1)", 100+i, text, text); err != nil {
				return err
			}
		}
		if err = tx.CreateHNSWIndex(ctx, kind+"_hnsw", table, "candidate"); err != nil {
			return err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	if err = db.Close(); err != nil {
		return err
	}
	db, err = sqlite.OpenContext(ctx, path, sqlite.Options{})
	if err != nil {
		return err
	}
	tx, err = db.Begin(ctx, sqlite.TxOptions{ReadOnly: true})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result := report{Caveat: "Actual typed HNSW graph candidates are approximate. Exact original dense reranking cannot recover excluded candidates. Both stages use the same read transaction and business filter. Materialized half/bit columns are maintained explicitly during writes; generated columns/expression ANN indexes are not implemented. This small fixture is illustrative, not a speed or universal recall claim."}
	for _, kind := range []string{"halfvec", "bit"} {
		text := "[1.0002,1,-1e-8,0]"
		if kind == "bit" {
			text = "[-0.001,1,0,0]"
		}
		query, e := vector.Parse(text)
		if e != nil {
			return e
		}
		table := kind + "_items"
		exact, e := tx.VectorSearch(ctx, table, "original", query, sqlite.VectorSearchOptions{Metric: vector.L2, Limit: 1, Where: "active=1"})
		if e != nil {
			return e
		}
		for _, limit := range []int{1, 2} {
			item := run{Representation: kind, CandidateLimit: limit, ExactDense: exact.Hits}
			if kind == "halfvec" {
				q, _, e := vector.HalfFromVector(query)
				if e != nil {
					return e
				}
				r, e := tx.HalfSearch(ctx, table, "candidate", q, sqlite.HalfSearchOptions{Mode: sqlite.VectorModeApproximate, Index: kind + "_hnsw", Limit: limit, EFSearch: 16, Where: "active=1"})
				if e != nil {
					return e
				}
				item.CandidateHits, item.CandidatePlan, item.CandidateStats, item.CandidateExact, item.CandidateMetric = r.Hits, r.Plan.QueryPlan, r.Stats.QueryStats, r.Plan.Exact, string(vector.L2)
			} else {
				q, e := vector.BinaryQuantize(query)
				if e != nil {
					return e
				}
				r, e := tx.BitSearch(ctx, table, "candidate", q, sqlite.BitSearchOptions{Mode: sqlite.VectorModeApproximate, Index: kind + "_hnsw", Limit: limit, EFSearch: 16, Where: "active=1"})
				if e != nil {
					return e
				}
				item.CandidateHits, item.CandidatePlan, item.CandidateStats, item.CandidateExact, item.CandidateMetric = r.Hits, r.Plan.QueryPlan, r.Stats.QueryStats, r.Plan.Exact, string(vector.Hamming)
			}
			for _, hit := range item.CandidateHits {
				rows, e := tx.Query(ctx, "SELECT id,original <-> ? FROM "+table+" WHERE id=? AND active=1", query, hit.RowID)
				if e != nil {
					return e
				}
				if !rows.Next() {
					return errors.Join(fmt.Errorf("candidate absent in snapshot"), rows.Err(), rows.Close())
				}
				values := rows.Values()
				item.DenseReranked = append(item.DenseReranked, vector.Hit{RowID: values[0].(int64), Distance: values[1].(float64)})
				if e = rows.Close(); e != nil {
					return e
				}
			}
			sort.Slice(item.DenseReranked, func(i, j int) bool {
				a, b := item.DenseReranked[i], item.DenseReranked[j]
				return a.Distance < b.Distance || a.Distance == b.Distance && a.RowID < b.RowID
			})
			item.DenseReranked = item.DenseReranked[:min(1, len(item.DenseReranked))]
			if len(item.DenseReranked) == 1 && item.DenseReranked[0].RowID == exact.Hits[0].RowID {
				item.RecallAt1 = 1
			}
			result.Runs = append(result.Runs, item)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	return json.NewEncoder(out).Encode(result)
}
func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: typed-ann-rerank <new-disposable-db>")
		return
	}
	if err := example(context.Background(), os.Args[1], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
