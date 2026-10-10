package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func importFixture(t *testing.T, format, data string) (ImportManifest, string, string) {
	t.Helper()
	dir := t.TempDir()
	input := filepath.Join(dir, "source."+format)
	if err := os.WriteFile(input, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	m := ImportManifest{FormatVersion: 1, SourceProfile: "pg_embedding-real-float4-v1", SourceID: "synthetic-test", SourceSHA256: importHash([]byte(data)), SourceSize: int64(len(data)), Format: format, RowOrder: "source", DecimalFidelity: "roundtrip_binary32", Table: "items", Fields: []ImportField{{Source: "id", Target: "id", Type: "int64", Encoding: "integer", PrimaryKey: true}, {Source: "embedding", Target: "embedding", Type: "vector", Encoding: "json_array", Dimensions: 2, Nullable: true}}, Indexes: []ImportIndex{}}
	if format == "csv" {
		m.NullToken = `\N`
		m.Fields[1].Encoding = "pg_array"
	}
	return m, input, filepath.Join(dir, "imported.db")
}
func TestImportNativeCommittedResumeAndVerify(t *testing.T) {
	ctx := context.Background()
	data := "{\"id\":-9223372036854775808,\"embedding\":[-0,1.401298464324817e-45]}\n{\"id\":-1,\"embedding\":null}\n{\"id\":0,\"embedding\":[3,4]}\n{\"id\":9223372036854775807,\"embedding\":[1,-2]}\n"
	m, in, out := importFixture(t, "ndjson", data)
	m.RowOrder = "ascending"
	r, err := Import(ctx, in, out, m, ImportOptions{})
	if err != nil || !r.DryRun || r.Accepted != 4 || r.Vectors[0].Nulls != 1 {
		t.Fatal(r, err)
	}
	r, err = Import(ctx, in, out, m, ImportOptions{Apply: true, BatchRows: 2, StopAfterBatches: 1})
	if err != nil || r.Status != "paused_committed" || r.Accepted != 2 || r.CommittedBatches != 1 {
		t.Fatal(r, err)
	}
	r, err = Import(ctx, in, out, m, ImportOptions{Apply: true, Resume: true, BatchRows: 1})
	if err != nil || r.Status != "complete_verified" || r.Accepted != 4 || r.CommittedBatches != 3 || r.CanonicalSHA256 != r.DatabaseCanonicalSHA256 {
		t.Fatal(r, err)
	}
	verified, err := VerifyImport(ctx, in, out, m)
	if err != nil || verified.CanonicalSHA256 != r.CanonicalSHA256 {
		t.Fatal(verified, err)
	}
	db, err := sqlite.Open(out, sqlite.Options{ReadOnly: true, NoCreate: true})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	rows, err := db.Query(ctx, "SELECT id, embedding FROM items WHERE id = ?", int64(math.MinInt64))
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() {
		t.Fatal(rows.Err())
	}
	values := rows.Values()
	rows.Close()
	if values[0] != int64(math.MinInt64) {
		t.Fatal(values)
	}
	v, err := vector.Decode(values[1].([]byte))
	if err != nil {
		t.Fatal(err)
	}
	bits := v.Values()
	if math.Float32bits(bits[0]) != 0x80000000 || math.Float32bits(bits[1]) != 1 {
		t.Fatal(bits)
	}
	// An ordinary existing, intact imported target is never overwritten.
	if _, err = Import(ctx, in, out, m, ImportOptions{Apply: true}); err == nil {
		t.Fatal("existing target accepted without resume")
	}
}
func TestImportCSVPrimitivesAndQuarantine(t *testing.T) {
	ctx := context.Background()
	data := "id,embedding,note,payload,weight,rank\r\n-3,\"{-0,1.401298464324817e-45}\",\"line1\nline2\",AAEC,-2.5,9223372036854775807\r\n2,\\N,,,-0,\\N\r\n2,\"{3,4}\",duplicate,,1,0\r\n4,\"{1e-100,2}\",bad,,1,0\r\n"
	m, in, out := importFixture(t, "csv", data)
	m.Fields = append(m.Fields, ImportField{Source: "note", Target: "note", Type: "text", Encoding: "utf8"}, ImportField{Source: "payload", Target: "payload", Type: "blob", Encoding: "base64"}, ImportField{Source: "weight", Target: "weight", Type: "float64", Encoding: "decimal"}, ImportField{Source: "rank", Target: "rank", Type: "int64", Encoding: "integer", Nullable: true})
	r, err := Import(ctx, in, out, m, ImportOptions{Apply: true, OnError: "quarantine", BatchRows: 2})
	if err != nil || r.Status != "complete_with_rejections" || r.Accepted != 2 || r.Rejected != 2 || len(r.RejectedPKs) != 2 {
		t.Fatal(r, err)
	}
	if len(r.MissingPKs) != 1 || r.MissingPKs[0] != 4 {
		t.Fatal("missing keys must distinguish rejected duplicate", r)
	}
	if r.CanonicalSHA256 != r.DatabaseCanonicalSHA256 || r.Vectors[0].Nulls != 1 {
		t.Fatal(r)
	}
	r, err = Import(ctx, in, out, m, ImportOptions{Apply: true, Resume: true, OnError: "quarantine"})
	if err != nil || r.Accepted != 2 || r.Rejected != 2 {
		t.Fatal(r, err)
	}
	if _, err = Import(ctx, in, out, m, ImportOptions{Apply: true, Resume: true, OnError: "stop"}); err == nil {
		t.Fatal("changed rejection policy accepted")
	}
}
func TestImportStopErrorAndOrdinaryCancellation(t *testing.T) {
	ctx := context.Background()
	m, in, out := importFixture(t, "ndjson", "{\"id\":1,\"embedding\":[1,2]}\n{\"id\":2,\"embedding\":[1e-100,2]}\n")
	r, err := Import(ctx, in, out, m, ImportOptions{Apply: true, BatchRows: 1})
	if err == nil {
		t.Fatal(r)
	}
	db, e := sqlite.Open(out, sqlite.Options{ReadOnly: true, NoCreate: true})
	if e != nil {
		t.Fatal(e)
	}
	var job importJob
	ok, e := readImportJSON(ctx, db, importJobs, 1, &job)
	db.Close()
	if e != nil || !ok || job.Cursor != 1 || job.Accepted != 1 {
		t.Fatal(job, e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = Import(canceled, in, out, m, ImportOptions{Apply: true, Resume: true})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestImportManifestStrictMemoryValidation(t *testing.T) {
	m, _, _ := importFixture(t, "ndjson", "")
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = DecodeImportManifest(strings.NewReader(string(raw))); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{strings.Replace(string(raw), `"format_version":1`, `"format_version":1,"format_version":1`, 1), strings.Replace(string(raw), `"format_version":1`, `"format_version":1,"surprise":0`, 1), strings.Replace(string(raw), `"nullable":true`, `"nullable":null`, 1), string(raw) + "{}", strings.Repeat(" ", MaxImportManifestBytes+1)} {
		if _, err = DecodeImportManifest(strings.NewReader(bad)); err == nil {
			t.Fatal("accepted bad manifest")
		}
	}
	// Invalid metadata is tested only in memory, not by altering selected files.
	j := importJob{Version: 1, Converter: ImportConverter, ManifestHash: m.hash(), Policy: "stop", CanonicalHash: importHash(nil), RecordsHash: importHash(nil), RejectionsHash: importHash(nil), SchemaHash: importHash(nil), Status: "importing"}
	if err = j.validate(m, "stop"); err != nil {
		t.Fatal(err)
	}
	j.Cursor = -1
	if err = j.validate(m, "stop"); err == nil {
		t.Fatal("accepted negative cursor")
	}
}

func TestImportBuildsRealHNSWAndIVF(t *testing.T) {
	ctx := context.Background()
	var source strings.Builder
	for id := 0; id < 64; id++ {
		cluster := float64(id/16) * 10
		d := float64(id%16) / 100
		fmt.Fprintf(&source, "{\"id\":%d,\"embedding\":[%.6f,%.6f]}\n", id, cluster+d, cluster+d*2)
	}
	m, in, out := importFixture(t, "ndjson", source.String())
	m.Indexes = []ImportIndex{{Name: "items_hnsw", Column: "embedding", Method: "hnsw", Metric: "l2", M: 4, EFConstruction: 32}, {Name: "items_ivf", Column: "embedding", Method: "ivfflat", Metric: "l2", Lists: 4, TrainingSamples: 64, TrainingIterations: 10}}
	r, err := Import(ctx, in, out, m, ImportOptions{Apply: true, BatchRows: 16})
	if err != nil || r.Status != "complete_verified" || r.Accepted != 64 {
		t.Fatal(r, err)
	}
	db, err := sqlite.Open(out, sqlite.Options{ReadOnly: true, NoCreate: true})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	query, _ := vector.New([]float32{20.055, 20.111})
	exact, err := db.VectorSearch(ctx, "items", "embedding", query, sqlite.VectorSearchOptions{Metric: vector.L2, Limit: 4})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"items_hnsw", "items_ivf"} {
		opts := sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Index: name, Metric: vector.L2, Limit: 4}
		wantMethod := "hnsw"
		if name == "items_hnsw" {
			opts.EFSearch = 64
		} else {
			opts.Probes = 1
			opts.MaxProbes = 1
			wantMethod = "ivfflat"
		}
		got, e := db.VectorSearch(ctx, "items", "embedding", query, opts)
		if e != nil {
			t.Fatal(e)
		}
		if got.Plan.Method != wantMethod || got.Plan.Exact {
			t.Fatal("requested ANN did not run", got.Plan)
		}
		if len(got.Hits) != len(exact.Hits) {
			t.Fatal(got, exact)
		}
		for i := range got.Hits {
			if got.Hits[i].RowID != exact.Hits[i].RowID || got.Hits[i].Distance != exact.Hits[i].Distance {
				t.Fatal("fixture recall@4 must equal 1", got.Hits, exact.Hits)
			}
		}
	}
}
func TestImportIndexSemanticBlockKeepsDataPending(t *testing.T) {
	ctx := context.Background()
	m, in, out := importFixture(t, "ndjson", "{\"id\":1,\"embedding\":[0,0]}\n")
	m.Indexes = []ImportIndex{{Name: "items_cosine", Column: "embedding", Method: "hnsw", Metric: "cosine"}}
	r, err := Import(ctx, in, out, m, ImportOptions{Apply: true})
	if err == nil || r.Status != "data_complete_index_pending" || r.Accepted != 1 {
		t.Fatal(r, err)
	}
	r, err = Import(ctx, in, out, m, ImportOptions{Apply: true, Resume: true})
	if err == nil || r.Status != "data_complete_index_pending" || r.Accepted != 1 {
		t.Fatal(r, err)
	}
	db, err := sqlite.Open(out, sqlite.Options{ReadOnly: true, NoCreate: true})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var job importJob
	ok, err := readImportJSON(ctx, db, importJobs, 1, &job)
	if err != nil || !ok || job.Status != "data_complete_index_pending" || job.Cursor != 1 || job.IndexesDone != 0 {
		t.Fatal(job, err)
	}
}
