package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/migration/postgres"
)

// Native data is imported once and remains intact; negatives are only new
// options documents or argument lists. No failed I/O or native mutation probes.
func TestIndependentMigrateImportDistanceCLIReportsFailure(t *testing.T) {
	d := t.TempDir()
	data := []byte("{\"key\":-5,\"v\":[3,4]}\n{\"key\":12,\"v\":[4,3]}\n")
	sum := sha256.Sum256(data)
	m := postgres.ImportManifest{FormatVersion: 1, SourceProfile: "pg_embedding-real-float4-v1", SourceID: "independent-cli-distance", SourceSHA256: hex.EncodeToString(sum[:]), SourceSize: int64(len(data)), Format: "ndjson", RowOrder: "ascending", DecimalFidelity: "roundtrip_binary32", Table: "mapped", Fields: []postgres.ImportField{{Source: "key", Target: "signed_pk", Type: "int64", Encoding: "integer", PrimaryKey: true}, {Source: "v", Target: "embedding", Type: "vector", Encoding: "json_array", Dimensions: 2}}, Indexes: []postgres.ImportIndex{}}
	in, out, manifest := filepath.Join(d, "export.ndjson"), filepath.Join(d, "result.db"), filepath.Join(d, "manifest.json")
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	for p, b := range map[string][]byte{in: data, manifest: raw} {
		if err = os.WriteFile(p, b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = postgres.Import(context.Background(), in, out, m, postgres.ImportOptions{Apply: true}); err != nil {
		t.Fatal(err)
	}
	hash := func(p string) [32]byte {
		b, e := os.ReadFile(p)
		if e != nil {
			t.Fatal(e)
		}
		return sha256.Sum256(b)
	}
	before := hash(out)
	for _, tc := range []struct {
		name, options, status string
		wantErr               bool
	}{
		{"valid", `{"column":"embedding","query_pks":[12],"metrics":["l1"],"top_k":1}`, "passed", false},
		{"budget", `{"column":"embedding","max_rows":1}`, "failed", true},
		{"query", `{"column":"embedding","query_pks":[99]}`, "failed", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := filepath.Join(d, tc.name+"-options.json")
			if e := os.WriteFile(p, []byte(tc.options), 0600); e != nil {
				t.Fatal(e)
			}
			var buf bytes.Buffer
			e := run([]string{"migrate", "pgembedding", "verify", "--input", in, "--output", out, "--manifest", manifest, "--distances", p}, &buf)
			if (e != nil) != tc.wantErr {
				t.Fatal("unexpected command result", e, buf.String())
			}
			var got postgres.ImportDistanceVerification
			if e = json.Unmarshal(buf.Bytes(), &got); e != nil {
				t.Fatal(e, buf.String())
			}
			if got.Distances.Status != tc.status {
				t.Fatal(got)
			}
			if tc.wantErr && got.Import.Status != "verification_failed" {
				t.Fatal(got.Import)
			}
			if !strings.HasPrefix(got.Import.ANNRecall, "not_run") || !strings.HasPrefix(got.Distances.ANNRecall, "not_run") {
				t.Fatal("ANN changed", got)
			}
		})
	}
	var buf bytes.Buffer
	if err = run([]string{"migrate", "pgembedding", "verify", "--input", in, "--output", out, "--manifest", manifest}, &buf); err != nil {
		t.Fatal(err)
	}
	var plain postgres.ImportResult
	if err = json.Unmarshal(buf.Bytes(), &plain); err != nil || plain.Status != "complete_verified" || !strings.HasPrefix(plain.ExactDistances, "not_run") {
		t.Fatal(plain, err)
	}
	if hash(out) != before {
		t.Fatal("CLI verification changed database bytes")
	}
}
