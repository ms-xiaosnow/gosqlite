package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestHybridExamplePreservesMeasures(t *testing.T) {
	var out bytes.Buffer
	if err := example(context.Background(), filepath.Join(t.TempDir(), "hybrid.db"), &out); err != nil {
		t.Fatal(err)
	}
	var result hybridOutput
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if len(result.Hits) != 3 || len(result.SQLRows) != 2 || result.Hits[0].RowID != 1 || result.SQLRows[0][0] != float64(1) {
		t.Fatal(out.String())
	}
	for _, hit := range result.Hits {
		if hit.TextScore == nil || hit.VectorDistance == nil || len(hit.Contributions) != 2 {
			t.Fatal(hit)
		}
	}
	if !result.Vector.Plan.Exact || result.Vector.Plan.Access != "SCAN" || result.SQLPlan.VectorRanking != "bounded_heap" || result.SQLStats.VectorRankedRows != 2 {
		t.Fatal(result)
	}
}
