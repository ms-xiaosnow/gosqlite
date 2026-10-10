package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/migration/postgres"
)

func TestMigrateImportDatasetDistancesCLI(t *testing.T) {
	dir := t.TempDir()
	data := []byte("{\"key\":-4,\"v\":[3,4]}\n{\"key\":7,\"v\":[1,1]}\n{\"key\":9,\"v\":[-1,1]}\n")
	sum := sha256.Sum256(data)
	manifest := postgres.ImportManifest{FormatVersion: 1, SourceProfile: "pg_embedding-real-float4-v1", SourceID: "cli-distance-fixture", SourceSHA256: hex.EncodeToString(sum[:]), SourceSize: int64(len(data)), Format: "ndjson", RowOrder: "ascending", DecimalFidelity: "roundtrip_binary32", Table: "mapped", Fields: []postgres.ImportField{{Source: "key", Target: "pk", Type: "int64", Encoding: "integer", PrimaryKey: true}, {Source: "v", Target: "embedding", Type: "vector", Encoding: "json_array", Dimensions: 2}}, Indexes: []postgres.ImportIndex{}}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	input, meta, output, options := filepath.Join(dir, "source.ndjson"), filepath.Join(dir, "manifest.json"), filepath.Join(dir, "result.db"), filepath.Join(dir, "options.json")
	for path, content := range map[string][]byte{input: data, meta: raw, options: []byte(`{"column":"embedding","query_pks":[-4,9],"top_k":2}`)} {
		if err = os.WriteFile(path, content, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err = run([]string{"migrate", "pgembedding", "--input", input, "--manifest", meta, "--output", output, "--apply"}, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
	args := []string{"migrate", "pgembedding", "verify", "--input", input, "--manifest", meta, "--output", output, "--distances", options}
	var out bytes.Buffer
	if err = run(args, &out); err != nil {
		t.Fatal(err, out.String())
	}
	var result postgres.ImportDistanceVerification
	if err = json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Distances.Status != "passed" || result.Distances.DistancesChecked != 24 || result.Distances.TopKChecked != 16 || result.Import.Status != "complete_verified" || !strings.HasPrefix(result.Import.ANNRecall, "not_run") {
		t.Fatal(result)
	}
	if err = run(append(append([]string(nil), args...), "--distances", options), &bytes.Buffer{}); err == nil {
		t.Fatal("duplicate option accepted")
	}
	applyArgs := []string{"migrate", "pgembedding", "--input", input, "--manifest", meta, "--distances", options}
	if err = run(applyArgs, &bytes.Buffer{}); err == nil {
		t.Fatal("verify-only option accepted for import")
	}
}
