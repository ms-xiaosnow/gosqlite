package main

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"testing"
)

func TestIVFFlatExampleActualListsAndFallbacks(t *testing.T) {
	var out bytes.Buffer
	if err := run(context.Background(), &out); err != nil {
		t.Fatal(err)
	}
	var got output
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Searches) != 9 || len(got.SQL) != 2 {
		t.Fatal(out.String())
	}
	stages := make(map[string]searchOutput)
	for _, s := range got.Searches {
		stages[s.Stage] = s
		if s.GraphNodeReads != 0 || s.GraphDistances != 0 || len(s.RowIDs) != 3 {
			t.Fatalf("unexpected graph work or hit count: %+v", s)
		}
	}
	untrained := stages["inserted_but_still_untrained"]
	if !untrained.Exact || untrained.Access != "SCAN" || untrained.TrainingState != "untrained" || untrained.Fallback == "" || untrained.CentroidReads != 0 || untrained.BaseRowsFetched != 128 {
		t.Fatalf("untrained index did not scan: %+v", untrained)
	}
	exact := stages["native_default_exact"]
	if !exact.Exact || exact.Method != "exact_vector" || exact.BaseRowsFetched != 128 || exact.CentroidReads != 0 {
		t.Fatalf("native default changed: %+v", exact)
	}
	trained := stages["trained_one_probe"]
	if trained.Exact || trained.Access != "SEARCH" || trained.Method != "ivfflat" || trained.TrainingState != "ready" || trained.CentroidReads != 4 || trained.CentroidScores != 4 || trained.ListsOpened != 1 || trained.MemberKeysRead != 32 || trained.BaseRowsFetched != 32 || trained.ANNExactRescores != 32 || !reflect.DeepEqual(trained.RowIDs, exact.RowIDs) {
		t.Fatalf("trained search did not use selected list: %+v", trained)
	}
	compensated := stages["filter_compensation"]
	if compensated.Exact || compensated.EffectiveProbes != 4 || compensated.ListsOpened != 4 || compensated.MemberKeysRead != 128 || compensated.CentroidScores != 4 || compensated.Iterations < 2 || compensated.FilteredRows != 96 {
		t.Fatalf("filter compensation counters: %+v", compensated)
	}
	fallback := stages["max_probes_exact_fallback"]
	if !fallback.Exact || fallback.Access != "SCAN" || fallback.Fallback == "" || fallback.MemberKeysRead != 32 || fallback.BaseRowsFetched != 160 || fallback.CentroidScores != 4 || !reflect.DeepEqual(fallback.RowIDs, compensated.RowIDs) {
		t.Fatalf("fallback lost attempted work: %+v", fallback)
	}
	all := stages["all_probes_exact_scan"]
	if !all.Exact || all.Access != "SCAN" || all.Method != "exact_vector" || all.Fallback != "probes equals lists" || all.CentroidReads != 0 || all.CentroidScores != 0 || all.ListsOpened != 0 || all.MemberKeysRead != 0 || all.BaseRowsFetched != 128 || !reflect.DeepEqual(all.RowIDs, exact.RowIDs) {
		t.Fatalf("all-probe request did not scan: %+v", all)
	}
	dml := stages["after_transactional_dml"]
	rebuilt := stages["after_explicit_reindex"]
	reopened := stages["ordinary_close_and_reopen"]
	if dml.Generation != trained.Generation || rebuilt.Generation != trained.Generation+1 || dml.RowIDs[0] != 129 || rebuilt.RowIDs[0] != 129 || reopened.Generation != rebuilt.Generation || reopened.Exact || !reflect.DeepEqual(rebuilt.RowIDs, reopened.RowIDs) || !reflect.DeepEqual(rebuilt.Distances, reopened.Distances) {
		t.Fatalf("DML/reindex/reopen: %+v / %+v / %+v", dml, rebuilt, reopened)
	}
	if got.SQL[0].Plan.Method != "ivfflat" || got.SQL[0].Plan.VectorExact || got.SQL[0].Stats.ANNCentroidScores != 4 || got.SQL[0].Stats.ANNMemberKeysRead != 32 || !got.SQL[1].Plan.VectorExact || got.SQL[1].Stats.ANNCentroidScores != 0 || got.SQL[1].Stats.BaseRowFetches != 128 || !reflect.DeepEqual(got.SQL[0].Rows, got.SQL[1].Rows) {
		t.Fatalf("SQL automatic ANN/exact reference: %+v", got.SQL)
	}
}
