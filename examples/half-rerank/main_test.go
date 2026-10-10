package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestHalfRerankExampleReportsCandidateRecallLoss(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rerank.db")
	var buf bytes.Buffer
	if err := example(context.Background(), path, &buf); err != nil {
		t.Fatal(err)
	}
	var got output
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.ExactDense.Hits) != 1 || got.ExactDense.Hits[0].RowID != 20 || !got.ExactDense.Plan.Exact || len(got.Runs) != 2 || got.SQLSubvector != "[1,1]" {
		t.Fatal(buf.String())
	}
	if got.QueryConversion.Changed != 2 || got.QueryConversion.MaxAbsoluteError <= 0 || got.QuantizedQuery != "1100" {
		t.Fatal(got)
	}
	for i, run := range got.Runs {
		wantID := int64(10)
		if i == 1 {
			wantID = 20
		}
		if run.CandidateMetric != vector.L2 || run.FinalMetric != vector.L2 || run.CandidateLimit != i+1 || len(run.Reranked) != 1 || run.Reranked[0].RowID != wantID || run.RecallAt1 != float64(i) || !run.Candidates.Plan.Exact || run.Candidates.Plan.Access != "SCAN" || run.Candidates.Stats.FilteredRows != 1 {
			t.Fatal(run)
		}
	}
	db, err := sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(context.Background(), "SELECT id,halfvec_to_text(half),halfvec_to_text(halfvec_from_vector(embedding)) FROM items ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for rows.Next() {
		v := rows.Values()
		if v[1] != v[2] {
			t.Fatal(v)
		}
		count++
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	if err = rows.Close(); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatal(count)
	}
	// Existing paths are refused, never cleared to manufacture a runnable demo.
	if err := example(context.Background(), path, &buf); err == nil {
		t.Fatal("existing database accepted")
	}
}
