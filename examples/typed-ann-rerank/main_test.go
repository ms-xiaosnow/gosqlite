package main

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
)

func TestTypedANNRerankSameSnapshot(t *testing.T) {
	var out bytes.Buffer
	if err := example(context.Background(), filepath.Join(t.TempDir(), "example.db"), &out); err != nil {
		t.Fatal(err)
	}
	var got report
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Runs) != 4 {
		t.Fatal(got)
	}
	for _, r := range got.Runs {
		if r.CandidateExact || r.CandidatePlan.Method != "hnsw" || r.CandidatePlan.MethodVersion != 2 || r.CandidateStats.ANNNodeReads == 0 || r.CandidateStats.ANNExactRescores == 0 || r.CandidateStats.BaseRowFetches >= 51 {
			t.Fatal(r)
		}
		want := float64(r.CandidateLimit - 1)
		if r.RecallAt1 != want {
			t.Fatal(r)
		}
	}
}
