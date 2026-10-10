package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateTransitionReviewOnlyArtifacts(t *testing.T) {
	dir := t.TempDir()
	options := filepath.Join(dir, "options.json")
	raw := `{"relation_oid":30000,"attnum":3,"dimensions":3,"approach":"retain_real_array_cast","mode":"single_transaction","vector_schema":"public","target_version":"0.8.7","indexes":[{"source_index_oid":30100,"name":"new_l2","method":"hnsw","m":16,"ef_construction":64,"ef_search":64,"lists":0,"probes":0},{"source_index_oid":30101,"name":"new_cos","method":"hnsw","m":16,"ef_construction":64,"ef_search":64,"lists":0,"probes":0},{"source_index_oid":30102,"name":"new_l1","method":"hnsw","m":16,"ef_construction":64,"ef_search":64,"lists":0,"probes":0}],"shadow_column":"","state_schema":"","state_table":"","batch_size":0,"concurrent_indexes":false,"max_bytes":0}`
	if err := os.WriteFile(options, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	args := []string{"pgembedding", "transition", "--catalog", "../../testdata/pgembedding-0.3.6/catalog.json", "--options", options}
	var out bytes.Buffer
	if err := runMigrate(context.Background(), args, &out); err != nil {
		t.Fatal(err)
	}
	var bundle struct {
		Artifacts []struct{ Name, Kind, Content string }
	}
	if err := json.Unmarshal(out.Bytes(), &bundle); err != nil || len(bundle.Artifacts) != 13 {
		t.Fatal(len(bundle.Artifacts), err)
	}
	for _, a := range bundle.Artifacts {
		if a.Kind != "review_only_sql" {
			continue
		}
		for _, line := range strings.Split(a.Content, "\n") {
			if strings.TrimSpace(line) != "" && !strings.HasPrefix(strings.TrimSpace(line), "--") {
				t.Fatal("active mutation text", a.Name, line)
			}
		}
	}
	report := filepath.Join(dir, "bundle.json")
	if err := runMigrate(context.Background(), append(args, "--report", report), &out); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(report)
	if err != nil {
		t.Fatal(err)
	}
	if err := runMigrate(context.Background(), append(args, "--report", report), &out); err == nil {
		t.Fatal("existing report overwritten")
	}
	after, err := os.ReadFile(report)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatal("existing report changed", err)
	}
	for _, extra := range [][]string{{"--apply"}, {"--resume"}, {"--options", options}, {"extra"}} {
		if err := runMigrate(context.Background(), append(append([]string{}, args...), extra...), &out); err == nil {
			t.Fatal("accepted unsafe/invalid flags", extra)
		}
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := runMigrate(cancelled, args, &out); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 2 {
		t.Fatal("unexpected database/artifact", entries, err)
	}
}
