package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestHNSWExampleActualGraph(t *testing.T) {
	var out bytes.Buffer
	if err := example(context.Background(), filepath.Join(t.TempDir(), "ann.db"), &out); err != nil {
		t.Fatal(err)
	}
	var got output
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if !got.Exact.Plan.Exact || got.Approximate.Plan.Exact || got.Approximate.Plan.Method != "hnsw" || got.Approximate.Stats.ANNNodeReads == 0 || len(got.Approximate.Hits) != 3 || got.Approximate.Hits[0].RowID != 21 || len(got.SQLRows) != 3 || got.SQLPlan.Method != "hnsw" || got.SQLPlan.VectorExact || got.SQLStats.ANNNodeReads == 0 {
		t.Fatal(out.String())
	}
}
