package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/migration/postgres"
)

func TestMigrateImportDryRunApplyResumeVerifyCLI(t *testing.T) {
	dir := t.TempDir()
	data := []byte("{\"id\":-9223372036854775808,\"embedding\":[-0,1e-45]}\n{\"id\":9,\"embedding\":null}\n")
	sum := sha256.Sum256(data)
	manifest := postgres.ImportManifest{FormatVersion: 1, SourceProfile: "pg_embedding-real-float4-v1", SourceID: "synthetic-cli", SourceSHA256: hex.EncodeToString(sum[:]), SourceSize: int64(len(data)), Format: "ndjson", RowOrder: "ascending", DecimalFidelity: "roundtrip_binary32", Table: "items", Fields: []postgres.ImportField{{Source: "id", Target: "id", Type: "int64", Encoding: "integer", PrimaryKey: true}, {Source: "embedding", Target: "embedding", Type: "vector", Encoding: "json_array", Dimensions: 2, Nullable: true}}, Indexes: []postgres.ImportIndex{}}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(dir, "input.ndjson")
	meta := filepath.Join(dir, "manifest.json")
	output := filepath.Join(dir, "result.db")
	if err = os.WriteFile(input, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(meta, raw, 0600); err != nil {
		t.Fatal(err)
	}
	runArgs := func(args ...string) postgres.ImportResult {
		t.Helper()
		var out bytes.Buffer
		if e := run(args, &out); e != nil {
			t.Fatal(e, out.String())
		}
		var result postgres.ImportResult
		if e := json.Unmarshal(out.Bytes(), &result); e != nil {
			t.Fatal(e, out.String())
		}
		return result
	}
	result := runArgs("migrate", "pgembedding", "--input", input, "--manifest", meta, "--output", output)
	if result.Status != "dry_run_validated" || !result.DryRun {
		t.Fatal(result)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 2 {
		t.Fatal(entries, err)
	}
	result = runArgs("migrate", "pgembedding", "import", "--input", input, "--manifest", meta, "--output", output, "--apply", "--batch-rows", "1", "--stop-after-batches", "1")
	if result.Status != "paused_committed" {
		t.Fatal(result)
	}
	result = runArgs("migrate", "pgembedding", "--input", input, "--manifest", meta, "--output", output, "--apply", "--resume")
	if result.Status != "complete_verified" || result.Accepted != 2 {
		t.Fatal(result)
	}
	result = runArgs("migrate", "pgembedding", "verify", "--input", input, "--manifest", meta, "--output", output)
	if result.Status != "complete_verified" {
		t.Fatal(result)
	}
	for _, extra := range [][]string{{"--apply", "--dry-run"}, {"--dry-run=false"}, {"--resume"}, {"--input", input}, {"--apply"}} {
		args := []string{"migrate", "pgembedding", "--input", input, "--manifest", meta, "--output", output}
		args = append(args, extra...)
		if err = run(args, &bytes.Buffer{}); err == nil {
			t.Fatal("accepted bad options/existing target", extra)
		}
	}
}
