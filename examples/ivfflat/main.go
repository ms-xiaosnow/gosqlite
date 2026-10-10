// Run with: go run ./examples/ivfflat
// The example creates and removes only its own temporary database directory.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

type searchOutput struct {
	Stage            string    `json:"stage"`
	Access           string    `json:"access"`
	Method           string    `json:"method"`
	Exact            bool      `json:"exact"`
	TrainingState    string    `json:"training_state,omitempty"`
	Generation       int64     `json:"generation,omitempty"`
	RequestedProbes  int       `json:"requested_probes,omitempty"`
	EffectiveProbes  int       `json:"effective_probes,omitempty"`
	StopReason       string    `json:"stop_reason,omitempty"`
	Fallback         string    `json:"fallback,omitempty"`
	RowIDs           []int64   `json:"rowids"`
	Distances        []float64 `json:"distances"`
	StateReads       int64     `json:"state_reads"`
	CentroidReads    int64     `json:"centroid_reads"`
	CentroidScores   int64     `json:"centroid_scores"`
	ListsOpened      int64     `json:"lists_opened"`
	MemberKeysRead   int64     `json:"member_keys_read"`
	IdentityLookups  int64     `json:"identity_lookups"`
	BaseRowsFetched  int64     `json:"base_rows_fetched"`
	FilteredRows     int64     `json:"filtered_rows"`
	ANNExactRescores int64     `json:"ann_exact_rescores"`
	GraphNodeReads   int64     `json:"graph_node_reads"`
	GraphDistances   int64     `json:"graph_distance_evaluations"`
	Iterations       int64     `json:"iterations"`
}

type sqlOutput struct {
	SQL   string            `json:"sql"`
	Plan  sqlite.QueryPlan  `json:"plan"`
	Stats sqlite.QueryStats `json:"stats"`
	Rows  [][]any           `json:"rows"`
}

type output struct {
	Note     string         `json:"note"`
	Searches []searchOutput `json:"searches"`
	SQL      []sqlOutput    `json:"sql"`
}

func summarize(stage string, r sqlite.VectorResult) searchOutput {
	p, s := r.Plan, r.Stats
	o := searchOutput{
		Stage: stage, Access: p.Access, Method: p.Method, Exact: p.Exact,
		TrainingState: p.ANNTrainingState, Generation: p.ANNGeneration,
		RequestedProbes: p.ANNProbes, EffectiveProbes: p.ANNEffectiveProbes,
		StopReason: p.ANNStopReason, Fallback: p.ANNFallback,
		StateReads: s.ANNStateReads, CentroidReads: s.ANNCentroidReads,
		CentroidScores: s.ANNCentroidScores, ListsOpened: s.ANNListsOpened,
		MemberKeysRead: s.ANNMemberKeysRead, IdentityLookups: s.ANNIdentityLookups,
		BaseRowsFetched: s.BaseRowFetches, FilteredRows: s.FilteredRows,
		ANNExactRescores: s.ANNExactRescores, GraphNodeReads: s.ANNNodeReads,
		GraphDistances: s.ANNDistanceEvaluations, Iterations: s.ANNIterations,
	}
	for _, hit := range r.Hits {
		o.RowIDs = append(o.RowIDs, hit.RowID)
		o.Distances = append(o.Distances, hit.Distance)
	}
	return o
}

func example(ctx context.Context, path string, out io.Writer) (err error) {
	db, err := sqlite.OpenContext(ctx, path, sqlite.Options{})
	if err != nil {
		return err
	}
	defer func() {
		if db != nil {
			err = errors.Join(err, db.Close())
		}
	}()
	if _, err = db.Exec(ctx, "CREATE TABLE items(id INTEGER PRIMARY KEY,embedding VECTOR(3) NOT NULL,active INTEGER)"); err != nil {
		return err
	}
	// Four lists need at least four trainable samples. Creating on an empty
	// table is legal, but later INSERTs alone do not train the index.
	if err = db.CreateIVFFlatIndex(ctx, "items_ivf", "items", "embedding", sqlite.IVFFlatIndexOptions{Lists: 4}); err != nil {
		return err
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for cluster := range 4 {
		for offset := range 32 {
			id := cluster*32 + offset + 1
			v := fmt.Sprintf("[%f,%f,1]", float64(cluster*1000)+float64(offset)/100, float64(cluster*500)+float64(offset)/200)
			if _, err = tx.Exec(ctx, "INSERT INTO items VALUES(?,?,?)", id, v, cluster == 3); err != nil {
				return err
			}
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	result := output{Note: "Logical record/key counters, not page I/O or latency. Exact candidate distances do not guarantee approximate recall. This clustered example is not a recall benchmark."}
	search := func(stage string, options sqlite.VectorSearchOptions) error {
		r, err := db.VectorSearch(ctx, "items", "embedding", "[0,0,1]", options)
		if err == nil {
			result.Searches = append(result.Searches, summarize(stage, r))
		}
		return err
	}
	approx := sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Index: "items_ivf", Limit: 3, Probes: 1}
	if err = search("inserted_but_still_untrained", approx); err != nil {
		return err
	}
	if err = db.Reindex(ctx, "items_ivf"); err != nil {
		return err
	}
	if err = db.CheckExtensionIndex(ctx, "items_ivf"); err != nil {
		return err
	}
	if err = search("native_default_exact", sqlite.VectorSearchOptions{Limit: 3}); err != nil {
		return err
	}
	if err = search("trained_one_probe", approx); err != nil {
		return err
	}
	filtered := approx
	filtered.Where = "active=1"
	if err = search("filter_compensation", filtered); err != nil {
		return err
	}
	filtered.MaxProbes = 1
	if err = search("max_probes_exact_fallback", filtered); err != nil {
		return err
	}
	all := approx
	all.Probes = 4
	if err = search("all_probes_exact_scan", all); err != nil {
		return err
	}
	for _, query := range []string{
		"SELECT id,embedding <-> '[0,0,1]' AS distance FROM items ORDER BY distance LIMIT 3",
		"SELECT id,embedding <-> '[0,0,1]' AS distance FROM items NOT INDEXED ORDER BY distance LIMIT 3",
	} {
		rows, err := db.Query(ctx, query)
		if err != nil {
			return err
		}
		s := sqlOutput{SQL: query}
		for rows.Next() {
			s.Rows = append(s.Rows, append([]any(nil), rows.Values()...))
		}
		if err = errors.Join(rows.Err(), rows.Close()); err != nil {
			return err
		}
		s.Plan, s.Stats = rows.Plan(), rows.Stats()
		result.SQL = append(result.SQL, s)
	}
	// Trained DML adjusts membership/counts in the source transaction without
	// moving centroids. Explicit REINDEX below is the only retraining step.
	tx, err = db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, query := range []string{
		"INSERT INTO items VALUES(129,'[0,0,1]',1)",
		"UPDATE items SET embedding='[3000,1500,1]',active=1 WHERE id=1",
		"DELETE FROM items WHERE id=2",
	} {
		if _, err = tx.Exec(ctx, query); err != nil {
			return err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return err
	}
	if err = db.CheckExtensionIndex(ctx, "items_ivf"); err != nil {
		return err
	}
	if err = search("after_transactional_dml", approx); err != nil {
		return err
	}
	if _, err = db.Exec(ctx, "REINDEX items_ivf"); err != nil {
		return err
	}
	if err = search("after_explicit_reindex", approx); err != nil {
		return err
	}
	if err = db.Close(); err != nil {
		return err
	}
	db = nil
	db, err = sqlite.OpenContext(ctx, path, sqlite.Options{})
	if err != nil {
		return err
	}
	if err = db.CheckExtensionIndex(ctx, "items_ivf"); err != nil {
		return err
	}
	if err = search("ordinary_close_and_reopen", approx); err != nil {
		return err
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func run(ctx context.Context, out io.Writer) (err error) {
	dir, err := os.MkdirTemp("", "gosqlite-ivfflat-example-")
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, os.RemoveAll(dir)) }()
	return example(ctx, filepath.Join(dir, "ivfflat.db"), out)
}

func main() {
	if err := run(context.Background(), os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
