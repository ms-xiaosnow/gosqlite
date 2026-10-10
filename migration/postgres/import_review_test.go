package postgres

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/internal/pager"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Review fixtures are intact synthetic exports; no selected file is modified
// after creation. All reopened databases were closed normally by Import.
func reviewImportFixture(t *testing.T, data string) (ImportManifest, string, string) {
	t.Helper()
	dir := t.TempDir()
	input := filepath.Join(dir, "source.ndjson")
	if err := os.WriteFile(input, []byte(data), 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(data))
	m := ImportManifest{FormatVersion: 1, SourceProfile: "pg_embedding-real-float4-v1", SourceID: "independent-review", SourceSHA256: hex.EncodeToString(sum[:]), SourceSize: int64(len(data)), Format: "ndjson", RowOrder: "source", DecimalFidelity: "roundtrip_binary32", Table: "items", Fields: []ImportField{{Source: "id", Target: "id", Type: "int64", Encoding: "integer", PrimaryKey: true}, {Source: "embedding", Target: "embedding", Type: "vector", Encoding: "json_array", Dimensions: 2, Nullable: true}}, Indexes: []ImportIndex{}}
	return m, input, filepath.Join(dir, "imported.db")
}

func reviewImportOpen(t *testing.T, output string) *sqlite.DB {
	t.Helper()
	db, err := sqlite.Open(output, sqlite.Options{ReadOnly: true, NoCreate: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	return db
}

func reviewImportJob(t *testing.T, db *sqlite.DB) importJob {
	t.Helper()
	var j importJob
	ok, err := readImportJSON(context.Background(), db, importJobs, 1, &j)
	if err != nil || !ok {
		t.Fatalf("job present=%t: %v", ok, err)
	}
	return j
}

func reviewImportCount(t *testing.T, db *sqlite.DB, table string, want int64) {
	t.Helper()
	rows, err := db.Query(context.Background(), "SELECT id FROM "+importQuote(table))
	if err != nil {
		t.Fatal(err)
	}
	var got int64
	for rows.Next() {
		got++
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s count=%d, want %d", table, got, want)
	}
}

type reviewCanonicalRow struct {
	pk   int64
	bits []uint32 // nil represents SQL NULL, never an empty non-NULL vector.
}

// This encoder does not call the production canonical/hash helpers or parser.
// Expected values are independently specified exact component bits and PKs.
func reviewCanonicalSHA(rows []reviewCanonicalRow) string {
	h := sha256.New()
	for _, row := range rows {
		var b bytes.Buffer
		b.WriteString("GMI1")
		binary.Write(&b, binary.BigEndian, uint32(2))
		b.Write([]byte{5, 'i', 'n', 't', '6', '4', 1})
		binary.Write(&b, binary.BigEndian, uint64(row.pk))
		b.Write([]byte{6, 'v', 'e', 'c', 't', 'o', 'r'})
		if row.bits == nil {
			b.WriteByte(0)
		} else {
			b.WriteByte(1)
			binary.Write(&b, binary.BigEndian, uint32(len(row.bits)))
			for _, bits := range row.bits {
				binary.Write(&b, binary.BigEndian, bits)
			}
		}
		binary.Write(h, binary.BigEndian, uint64(b.Len()))
		h.Write(b.Bytes())
	}
	return hex.EncodeToString(h.Sum(nil))
}

func TestImportReviewIndependentPKBitsAndCanonicalHash(t *testing.T) {
	ctx := context.Background()
	data := "{\"id\":-9223372036854775808,\"embedding\":[-0,1.401298464324817e-45,1.000000059604644775390626,3.4028234663852886e38]}\n" +
		"{\"id\":0,\"embedding\":null}\n" +
		"{\"id\":9223372036854775807,\"embedding\":[-1.401298464324817e-45,0,-1.000000059604644775390626,-3.4028234663852886e38]}\n"
	m, input, output := reviewImportFixture(t, data)
	m.Fields[1].Dimensions = 4
	m.RowOrder = "ascending"
	want := []reviewCanonicalRow{{math.MinInt64, []uint32{0x80000000, 1, 0x3f800001, 0x7f7fffff}}, {0, nil}, {math.MaxInt64, []uint32{0x80000001, 0, 0xbf800001, 0xff7fffff}}}
	result, err := Import(ctx, input, output, m, ImportOptions{Apply: true, BatchRows: 1})
	if err != nil {
		t.Fatal(err)
	}
	canonical := reviewCanonicalSHA(want)
	if result.Status != "complete_verified" || result.Accepted != 3 || result.Rejected != 0 || result.SourceRecords != 3 || result.Vectors[0].Nulls != 1 || result.CanonicalSHA256 != canonical || result.DatabaseCanonicalSHA256 != canonical {
		t.Fatalf("result=%+v, independent canonical=%s", result, canonical)
	}
	db := reviewImportOpen(t, output)
	rows, err := db.Query(ctx, "SELECT id, embedding FROM items ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	var n int
	for rows.Next() {
		if n >= len(want) {
			t.Fatal("extra destination row")
		}
		values := rows.Values()
		if values[0] != want[n].pk {
			t.Fatalf("PK %v, want %d", values[0], want[n].pk)
		}
		if want[n].bits == nil {
			if values[1] != nil {
				t.Fatal("SQL NULL lost")
			}
		} else {
			v, err := vector.Decode(values[1].([]byte))
			if err != nil {
				t.Fatal(err)
			}
			got := make([]uint32, v.Dims())
			for i, x := range v.Values() {
				got[i] = math.Float32bits(x)
			}
			if !reflect.DeepEqual(got, want[n].bits) {
				t.Fatalf("component bits=%x, want %x", got, want[n].bits)
			}
		}
		n++
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil || n != len(want) {
		t.Fatalf("count=%d, error=%v", n, err)
	}
	if j := reviewImportJob(t, db); j.Status != "complete_verified" || j.Cursor != 3 || j.Offset != int64(len(data)) {
		t.Fatal(j)
	}
}

func TestImportReviewDuplicateBatchIsAtomic(t *testing.T) {
	ctx := context.Background()
	m, input, output := reviewImportFixture(t, "{\"id\":1,\"embedding\":[1,2]}\n{\"id\":2,\"embedding\":[3,4]}\n{\"id\":3,\"embedding\":[5,6]}\n{\"id\":3,\"embedding\":[7,8]}\n")
	for _, resume := range []bool{false, true} {
		if _, err := Import(ctx, input, output, m, ImportOptions{Apply: true, Resume: resume, BatchRows: 2}); err == nil {
			t.Fatal("duplicate accepted")
		}
		db, err := sqlite.Open(output, sqlite.Options{ReadOnly: true, NoCreate: true})
		if err != nil {
			t.Fatal(err)
		}
		j := reviewImportJob(t, db)
		if j.Cursor != 2 || j.Accepted != 2 || j.Rejected != 0 || j.Batch != 1 || j.Status != "importing" {
			t.Fatal(j)
		}
		for table, count := range map[string]int64{"items": 2, importAccepted: 2, importBatches: 1, importRejections: 0} {
			reviewImportCount(t, db, table, count)
		}
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := VerifyImport(ctx, input, output, m); err == nil {
		t.Fatal("incomplete data verified")
	}
}

func TestImportReviewQuarantineCommittedResume(t *testing.T) {
	ctx := context.Background()
	m, input, output := reviewImportFixture(t, "{\"id\":5,\"embedding\":[1,2]}\n{\"id\":5,\"embedding\":[1,2]}\n{\"id\":1e0,\"embedding\":[1,2]}\n{\"id\":6,\"embedding\":[1e-100,2]}\n{\"id\":7,\"embedding\":null}\n{\"id\":5,\"embedding\":[3,4]}\n{\"id\":8,\"embedding\":[3,4]}\n")
	r, err := Import(ctx, input, output, m, ImportOptions{Apply: true, OnError: "quarantine", BatchRows: 2, StopAfterBatches: 1})
	if err != nil || r.Status != "paused_committed" || r.Accepted != 1 || r.Rejected != 1 || r.SourceRecords != 2 {
		t.Fatal(r, err)
	}
	r, err = Import(ctx, input, output, m, ImportOptions{Apply: true, Resume: true, OnError: "quarantine", BatchRows: 2})
	if err != nil || r.Status != "complete_with_rejections" || r.Accepted != 3 || r.Rejected != 4 || r.SourceRecords != 7 || r.RejectionsWithoutPK != 1 || !reflect.DeepEqual(r.RejectedPKs, []int64{5, 6, 5}) || !reflect.DeepEqual(r.MissingPKs, []int64{6}) {
		t.Fatal(r, err)
	}
	want := reviewCanonicalSHA([]reviewCanonicalRow{{5, []uint32{0x3f800000, 0x40000000}}, {7, nil}, {8, []uint32{0x40400000, 0x40800000}}})
	if r.CanonicalSHA256 != want || r.DatabaseCanonicalSHA256 != want {
		t.Fatalf("canonical got %s/%s, want %s", r.CanonicalSHA256, r.DatabaseCanonicalSHA256, want)
	}
	db := reviewImportOpen(t, output)
	j := reviewImportJob(t, db)
	if j.Status != "complete_with_rejections" || j.Accepted+j.Rejected != j.Cursor || j.Cursor != 7 {
		t.Fatal(j)
	}
	for table, count := range map[string]int64{"items": 3, importAccepted: 3, importBatches: 4, importRejections: 4} {
		reviewImportCount(t, db, table, count)
	}
}

func TestImportReviewPreflightAndCancellationBeforeCreation(t *testing.T) {
	ctx := context.Background()
	m, input, output := reviewImportFixture(t, "{\"id\":1,\"embedding\":[1,2]}\n")
	changed := m
	changed.SourceSHA256 = strings.Repeat("0", 64)
	if _, err := Import(ctx, input, output, changed, ImportOptions{Apply: true}); err == nil {
		t.Fatal("false manifest input hash accepted")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := Import(canceled, input, output, m, ImportOptions{Apply: true}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(filepath.Dir(input))
	if err != nil || len(entries) != 1 || entries[0].Name() != "source.ndjson" {
		t.Fatalf("preflight/cancellation created output: %v, %v", entries, err)
	}
	if _, err := Import(ctx, input, output, m, ImportOptions{Apply: true}); err != nil {
		t.Fatal(err)
	}
	changed = m
	changed.SourceID = "different-immutable-export"
	if _, err := Import(ctx, input, output, changed, ImportOptions{Apply: true, Resume: true}); err == nil {
		t.Fatal("different manifest identity resumed")
	}
	if _, err := Import(ctx, input, output, m, ImportOptions{Apply: true}); err == nil {
		t.Fatal("existing destination adopted without resume")
	}
}

func TestImportReviewNativeIndexesAndReadyState(t *testing.T) {
	ctx := context.Background()
	var data strings.Builder
	for i := 1; i <= 12; i++ {
		fmt.Fprintf(&data, "{\"id\":%d,\"embedding\":[%d,%d]}\n", i, i, 2*i)
	}
	m, input, output := reviewImportFixture(t, data.String())
	m.Indexes = []ImportIndex{{Name: "items_hnsw", Column: "embedding", Method: "hnsw", Metric: "l2", M: 2, EFConstruction: 4}, {Name: "items_ivf", Column: "embedding", Method: "ivfflat", Metric: "l2", Lists: 2, TrainingSamples: 12, TrainingIterations: 3}}
	r, err := Import(ctx, input, output, m, ImportOptions{Apply: true, BatchRows: 4})
	if err != nil || r.Status != "complete_verified" || r.Accepted != 12 {
		t.Fatal(r, err)
	}
	db := reviewImportOpen(t, output)
	if j := reviewImportJob(t, db); j.IndexesDone != 2 || j.Status != "complete_verified" {
		t.Fatal(j)
	}
	for _, index := range m.Indexes {
		if err := db.CheckExtensionIndex(ctx, index.Name); err != nil {
			t.Fatal(err)
		}
		o := sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Index: index.Name, Limit: 1, Metric: vector.L2}
		if index.Method == "ivfflat" {
			o.Probes, o.MaxProbes = 1, 1
		}
		got, err := db.VectorSearch(ctx, "items", "embedding", "[2,4]", o)
		if err != nil || len(got.Hits) != 1 || got.Plan.Method != index.Method || got.Plan.Exact {
			t.Fatalf("%s search: %+v, %v", index.Method, got, err)
		}
		if index.Method == "ivfflat" && got.Plan.ANNTrainingState != "ready" {
			t.Fatal(got.Plan)
		}
	}
}

func TestImportReviewIndexFailureRetainsDataPending(t *testing.T) {
	ctx := context.Background()
	for _, index := range []ImportIndex{{Name: "items_cos", Column: "embedding", Method: "hnsw", Metric: "cosine", M: 2, EFConstruction: 4}, {Name: "items_ivf", Column: "embedding", Method: "ivfflat", Metric: "l2", Lists: 3}} {
		t.Run(index.Method, func(t *testing.T) {
			m, input, output := reviewImportFixture(t, "{\"id\":1,\"embedding\":[0,0]}\n{\"id\":2,\"embedding\":[1,2]}\n")
			m.Indexes = []ImportIndex{index}
			for _, resume := range []bool{false, true} {
				r, err := Import(ctx, input, output, m, ImportOptions{Apply: true, Resume: resume, BatchRows: 1})
				if err == nil || r.Status != "data_complete_index_pending" || r.Accepted != 2 || r.Rejected != 0 {
					t.Fatal(r, err)
				}
				db, err := sqlite.Open(output, sqlite.Options{ReadOnly: true, NoCreate: true})
				if err != nil {
					t.Fatal(err)
				}
				j := reviewImportJob(t, db)
				if j.Status != "data_complete_index_pending" || j.IndexesDone != 0 || j.Accepted != 2 || j.Cursor != 2 {
					t.Fatal(j)
				}
				reviewImportCount(t, db, "items", 2)
				if err := db.Close(); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := VerifyImport(ctx, input, output, m); err == nil {
				t.Fatal("pending index accepted as verified")
			}
		})
	}
}

func TestImportReviewMetadataModelsAndExplicitManifest(t *testing.T) {
	// Invalid models remain entirely in memory, never persisted to selected files.
	m := ImportManifest{FormatVersion: 1, SourceProfile: "pg_embedding-real-float4-v1", SourceID: "model", SourceSHA256: strings.Repeat("0", 64), SourceSize: 42, Format: "ndjson", RowOrder: "ascending", DecimalFidelity: "roundtrip_binary32", Table: "items", Fields: []ImportField{{Source: "id", Target: "id", Type: "int64", Encoding: "integer", PrimaryKey: true}, {Source: "embedding", Target: "embedding", Type: "vector", Encoding: "json_array", Dimensions: 2, Nullable: true}}, Indexes: []ImportIndex{{Name: "idx", Column: "embedding", Method: "hnsw", Metric: "l2", M: 2, EFConstruction: 4}}}
	if err := m.Validate(); err != nil {
		t.Fatal(err)
	}
	emptyMethod := m
	emptyMethod.Indexes = append([]ImportIndex(nil), m.Indexes...)
	emptyMethod.Indexes[0].Method = ""
	if err := emptyMethod.Validate(); err == nil {
		t.Error("missing explicit index method accepted")
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatal(err)
	}
	for name, bad := range map[string]string{
		"missing_nullable":   strings.Replace(string(raw), `"nullable":true,`, "", 1),
		"null_indexes":       strings.Replace(string(raw), `"indexes":[`, `"indexes":null,"discarded":[`, 1),
		"duplicate_identity": strings.Replace(string(raw), `"source_id":"model"`, `"source_id":"model","source_id":"different"`, 1),
		"unsupported_table":  strings.Replace(string(raw), `"table":"items"`, `"table":"items; SELECT 1"`, 1),
	} {
		if _, err := DecodeImportManifest(strings.NewReader(bad)); err == nil {
			t.Errorf("accepted %s", name)
		}
	}
	pk := int64(1)
	j := importJob{Version: 1, Converter: ImportConverter, ManifestHash: m.hash(), Policy: "stop", Cursor: 1, Offset: 42, Accepted: 1, Batch: 1, LastPK: &pk, CanonicalHash: strings.Repeat("0", 64), RecordsHash: strings.Repeat("0", 64), RejectionsHash: strings.Repeat("0", 64), SchemaHash: strings.Repeat("0", 64), Status: "data_complete_index_pending"}
	if err := j.validate(m, "stop"); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(*importJob){
		"negative_offset":            func(x *importJob) { x.Offset = -1 },
		"cursor_accounting":          func(x *importJob) { x.Accepted = 0 },
		"future_version":             func(x *importJob) { x.Version++ },
		"converter_drift":            func(x *importJob) { x.Converter += "-other" },
		"manifest_drift":             func(x *importJob) { x.ManifestHash = strings.Repeat("1", 64) },
		"bad_hash":                   func(x *importJob) { x.SchemaHash = "not-a-hash" },
		"unknown_phase":              func(x *importJob) { x.Status = "done" },
		"verified_pending_index":     func(x *importJob) { x.Status = "complete_verified" },
		"verify_phase_pending_index": func(x *importJob) { x.Status = "data_complete_verify_pending" },
		"false_rejections_phase":     func(x *importJob) { x.Status = "complete_with_rejections"; x.IndexesDone = 1 },
	} {
		x := j
		change(&x)
		if err := x.validate(m, "stop"); err == nil {
			t.Errorf("accepted %s", name)
		}
	}
}

func TestImportReviewNeverAdoptsUnrelatedNativeDestination(t *testing.T) {
	ctx := context.Background()
	m, input, output := reviewImportFixture(t, "{\"id\":1,\"embedding\":[1,2]}\n")
	// Construct a legitimate unrelated database normally, then keep it intact.
	tx, err := pager.OpenTx(ctx, output, pager.Options{Create: true})
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	db, err := sqlite.Open(output, sqlite.Options{NoCreate: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, "CREATE TABLE existing (id INTEGER PRIMARY KEY, note TEXT NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, "INSERT INTO existing (id, note) VALUES (?, ?)", int64(17), "preserve me"); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	for _, resume := range []bool{false, true} {
		if _, err := Import(ctx, input, output, m, ImportOptions{Apply: true, Resume: resume}); err == nil {
			t.Fatalf("unrelated destination adopted, resume=%t", resume)
		}
	}
	after, err := os.ReadFile(output)
	if err != nil || !bytes.Equal(before, after) {
		t.Fatalf("unrelated destination changed: %v", err)
	}
	db = reviewImportOpen(t, output)
	reviewImportCount(t, db, "existing", 1)
}

func TestImportReviewCSVCursorAndBoundValues(t *testing.T) {
	ctx := context.Background()
	first := "id,embedding,note\r\n-3,\"{-0,1.401298464324817e-45}\",\"first\nsecond\"\r\n"
	data := first + "4,\\N,\"'); DROP TABLE items; --\"\r\n9,\"{3,4}\",last"
	m, input, output := reviewImportFixture(t, data)
	m.Format, m.NullToken, m.Fields[1].Encoding = "csv", `\N`, "pg_array"
	m.Fields = append(m.Fields, ImportField{Source: "note", Target: "note", Type: "text", Encoding: "utf8"})
	r, err := Import(ctx, input, output, m, ImportOptions{Apply: true, BatchRows: 1, StopAfterBatches: 1})
	if err != nil || r.Status != "paused_committed" || r.CursorOffset != int64(len(first)) {
		t.Fatal(r, err)
	}
	r, err = Import(ctx, input, output, m, ImportOptions{Apply: true, Resume: true, BatchRows: 1})
	if err != nil || r.Status != "complete_verified" || r.SourceRecords != 3 || r.Accepted != 3 || r.CursorOffset != int64(len(data)) || r.Vectors[0].Nulls != 1 {
		t.Fatal(r, err)
	}
	db := reviewImportOpen(t, output)
	rows, err := db.Query(ctx, "SELECT id, note FROM items ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"first\nsecond", "'); DROP TABLE items; --", "last"}
	var i int
	for rows.Next() {
		if i >= len(want) || rows.Values()[1] != want[i] {
			t.Fatalf("bound text changed: %v", rows.Values())
		}
		i++
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil || i != len(want) {
		t.Fatal(i, err)
	}
}

func TestImportReviewSchemaErrorsNeverQuarantined(t *testing.T) {
	ctx := context.Background()
	for name, bad := range map[string]string{
		"missing":   `{ "id": 2 }`,
		"unknown":   `{ "id": 2, "embedding": [1,2], "extra": 1 }`,
		"duplicate": `{ "id": 2, "embedding": [1,2], "id": 2 }`,
	} {
		t.Run(name, func(t *testing.T) {
			m, input, output := reviewImportFixture(t, "{\"id\":1,\"embedding\":[1,2]}\n"+bad+"\n")
			if _, err := Import(ctx, input, output, m, ImportOptions{Apply: true, OnError: "quarantine", BatchRows: 2}); err == nil {
				t.Fatal("schema error quarantined")
			}
			db := reviewImportOpen(t, output)
			j := reviewImportJob(t, db)
			if j.Cursor != 0 || j.Accepted != 0 || j.Rejected != 0 || j.Batch != 0 || j.Status != "importing" {
				t.Fatal(j)
			}
			reviewImportCount(t, db, "items", 0)
			reviewImportCount(t, db, importRejections, 0)
		})
	}
}

func TestImportReviewExpectedRowsNeverFalseComplete(t *testing.T) {
	ctx := context.Background()
	m, input, output := reviewImportFixture(t, "{\"id\":1,\"embedding\":[1,2]}\n")
	expected := int64(2)
	m.ExpectedRows = &expected
	for _, resume := range []bool{false, true} {
		if r, err := Import(ctx, input, output, m, ImportOptions{Apply: true, Resume: resume}); err == nil || strings.HasPrefix(r.Status, "complete_") {
			t.Fatal(r, err)
		}
	}
	if _, err := VerifyImport(ctx, input, output, m); err == nil {
		t.Fatal("missing expected row verified")
	}
	db := reviewImportOpen(t, output)
	j := reviewImportJob(t, db)
	if j.Cursor != 1 || j.Accepted != 1 || j.Status != "importing" {
		t.Fatal(j)
	}
	reviewImportCount(t, db, "items", 1)
}

func TestImportReviewDryRunKeyBudgetAndAscendingStream(t *testing.T) {
	ctx := context.Background()
	var data strings.Builder
	for i := 0; i <= MaxImportDryRunKeys; i++ {
		fmt.Fprintf(&data, "{\"id\":%d,\"embedding\":null}\n", i)
	}
	m, input, output := reviewImportFixture(t, data.String())
	r, err := Import(ctx, input, output, m, ImportOptions{OnError: "quarantine"})
	if err == nil || !strings.Contains(err.Error(), "unique-key limit") || r.Rejected != 0 || r.Accepted != MaxImportDryRunKeys || r.Status != "" {
		t.Fatal(r, err)
	}
	m.RowOrder = "ascending"
	r, err = Import(ctx, input, output, m, ImportOptions{})
	if err != nil || r.Status != "dry_run_validated" || r.Accepted != MaxImportDryRunKeys+1 {
		t.Fatal(r, err)
	}
	entries, err := os.ReadDir(filepath.Dir(input))
	if err != nil || len(entries) != 1 {
		t.Fatalf("dry-run created destination: %v, %v", entries, err)
	}
}

func TestImportReviewEmptyAndAllRejectedCompletion(t *testing.T) {
	ctx := context.Background()
	for _, csv := range []bool{false, true} {
		t.Run(fmt.Sprintf("empty_csv_%t", csv), func(t *testing.T) {
			data := ""
			if csv {
				data = "id,embedding\r\n"
			}
			m, input, output := reviewImportFixture(t, data)
			zero := int64(0)
			m.ExpectedRows = &zero
			if csv {
				m.Format, m.NullToken, m.Fields[1].Encoding = "csv", `\N`, "pg_array"
			}
			for _, resume := range []bool{false, true} {
				r, err := Import(ctx, input, output, m, ImportOptions{Apply: true, Resume: resume})
				if err != nil || r.Status != "complete_verified" || r.Accepted != 0 || r.SourceRecords != 0 || r.CommittedBatches != 0 || r.CursorOffset != int64(len(data)) || r.CanonicalSHA256 != reviewCanonicalSHA(nil) {
					t.Fatal(r, err)
				}
			}
		})
	}
	t.Run("all_rejected", func(t *testing.T) {
		m, input, output := reviewImportFixture(t, "{\"id\":null,\"embedding\":[1,2]}\n{\"id\":1e0,\"embedding\":[1,2]}\n")
		m.RowOrder = "ascending"
		for _, resume := range []bool{false, true} {
			r, err := Import(ctx, input, output, m, ImportOptions{Apply: true, Resume: resume, OnError: "quarantine", BatchRows: 1})
			if err != nil || r.Status != "complete_with_rejections" || r.Accepted != 0 || r.Rejected != 2 || r.SourceRecords != 2 || r.RejectionsWithoutPK != 2 || r.CanonicalSHA256 != reviewCanonicalSHA(nil) {
				t.Fatal(r, err)
			}
		}
		db := reviewImportOpen(t, output)
		j := reviewImportJob(t, db)
		if j.LastPK != nil || j.Status != "complete_with_rejections" {
			t.Fatal(j)
		}
		reviewImportCount(t, db, "items", 0)
		reviewImportCount(t, db, importRejections, 2)
	})
}

func TestImportReviewDecodedByteBoundAndPendingCursor(t *testing.T) {
	ctx := context.Background()
	array := "[" + strings.Repeat("0,", vector.MaxDimensions-1) + "0]"
	t.Run("oversized_decoded_record", func(t *testing.T) {
		var data strings.Builder
		data.WriteString(`{"id":1`)
		fields := []ImportField{{Source: "id", Target: "id", Type: "int64", Encoding: "integer", PrimaryKey: true}}
		for i := 0; i < 17; i++ {
			name := fmt.Sprintf("v%d", i)
			fmt.Fprintf(&data, ",%q:%s", name, array)
			fields = append(fields, ImportField{Source: name, Target: name, Type: "vector", Encoding: "json_array", Dimensions: vector.MaxDimensions})
		}
		data.WriteString("}\n")
		if data.Len() >= MaxImportRecordBytes {
			t.Fatal("fixture must fit raw record bound")
		}
		m, input, output := reviewImportFixture(t, data.String())
		m.Fields = fields
		r, err := Import(ctx, input, output, m, ImportOptions{Apply: true, OnError: "quarantine", BatchBytes: MaxImportRecordBytes})
		if err == nil || !strings.Contains(err.Error(), "decoded-byte bound") || r.Rejected != 0 {
			t.Fatal(r, err)
		}
		db := reviewImportOpen(t, output)
		j := reviewImportJob(t, db)
		if j.Cursor != 0 || j.Batch != 0 || j.Accepted != 0 || j.Rejected != 0 {
			t.Fatal(j)
		}
		reviewImportCount(t, db, "items", 0)
	})
	t.Run("carry_at_complete_boundary", func(t *testing.T) {
		var data strings.Builder
		var offset int64
		for i := 0; i < 17; i++ {
			fmt.Fprintf(&data, "{\"id\":%d,\"embedding\":%s}\n", i, array)
			if i == 15 {
				offset = int64(data.Len())
			}
		}
		m, input, output := reviewImportFixture(t, data.String())
		m.Fields[1].Dimensions = vector.MaxDimensions
		r, err := Import(ctx, input, output, m, ImportOptions{Apply: true, BatchBytes: MaxImportRecordBytes, StopAfterBatches: 1})
		if err != nil || r.Accepted != 16 || r.CursorOffset != offset || r.Status != "paused_committed" {
			t.Fatal(r, err)
		}
		r, err = Import(ctx, input, output, m, ImportOptions{Apply: true, Resume: true, BatchBytes: MaxImportRecordBytes})
		if err != nil || r.Status != "complete_verified" || r.Accepted != 17 || r.CommittedBatches != 2 || r.CursorOffset != int64(data.Len()) {
			t.Fatal(r, err)
		}
	})
}

func TestImportReviewNativeIndexIdentityMemoryModels(t *testing.T) {
	// Only in-memory representations are changed. No selected database's
	// extension tables, index bytes, schema, or persisted state are altered.
	m := ImportManifest{Table: "items", Fields: []ImportField{{Target: "id", Type: "int64"}, {Target: "embedding", Type: "vector", Dimensions: 2}}}
	for _, method := range []string{"hnsw", "ivfflat"} {
		t.Run(method, func(t *testing.T) {
			x := ImportIndex{Name: "idx", Column: "embedding", Method: method, Metric: "l2"}
			if method == "ivfflat" {
				x.Lists = 2
			}
			cfg, err := x.config(2)
			if err != nil {
				t.Fatal(err)
			}
			canonical, err := cfg.Canonical()
			if err != nil {
				t.Fatal(err)
			}
			values := []any{int64(1), "items", method, int64(1), int64(1), string(canonical)}
			fields := []any{"embedding", int64(1)}
			var state []any
			if method == "ivfflat" {
				state = []any{"ready"}
			}
			if err := validateImportIndexIdentity(m, x, values, fields, state); err != nil {
				t.Fatal(err)
			}
			for ordinal, replacements := range map[int][]any{
				0: {int64(0), "1"},
				1: {"other_source"},
				2: {"different_method"},
				3: {int64(2), "1"},
				4: {int64(2), "1"},
				5: {"{}", append([]byte(nil), canonical...), string(canonical) + " "},
			} {
				for _, replacement := range replacements {
					bad := append([]any(nil), values...)
					bad[ordinal] = replacement
					if err := validateImportIndexIdentity(m, x, bad, fields, state); err == nil {
						t.Fatalf("accepted altered native identity field %d", ordinal)
					}
				}
			}
			for _, bad := range [][]any{nil, {"different_column", int64(1)}, {"embedding", int64(0)}, {"embedding", "1"}, {"embedding", int64(1), "extra"}} {
				if err := validateImportIndexIdentity(m, x, values, bad, state); err == nil {
					t.Fatalf("accepted altered index field model: %v", bad)
				}
			}
			if method == "ivfflat" {
				for _, bad := range [][]any{nil, {"untrained"}, {"ready", "extra"}} {
					if err := validateImportIndexIdentity(m, x, values, fields, bad); err == nil {
						t.Fatalf("accepted nonready IVF model: %v", bad)
					}
				}
			}
		})
	}
}

func TestImportReviewMissingPKSetExcludesLaterAcceptedRow(t *testing.T) {
	ctx := context.Background()
	var data strings.Builder
	for _, pk := range []int64{-4, 9, -4, -2} {
		fmt.Fprintf(&data, "{\"id\":%d,\"embedding\":[1e-100,2]}\n", pk)
	}
	data.WriteString("{\"id\":9,\"embedding\":[1,2]}\n")
	m, input, output := reviewImportFixture(t, data.String())
	r, err := Import(ctx, input, output, m, ImportOptions{Apply: true, OnError: "quarantine", BatchRows: 2, StopAfterBatches: 1})
	if err != nil || r.Status != "paused_committed" || r.Accepted != 0 || r.Rejected != 2 || r.MissingPKs != nil {
		t.Fatal(r, err)
	}
	r, err = Import(ctx, input, output, m, ImportOptions{Apply: true, Resume: true, OnError: "quarantine", BatchRows: 2})
	if err != nil || r.Status != "complete_with_rejections" || r.Accepted != 1 || r.Rejected != 4 || !reflect.DeepEqual(r.MissingPKs, []int64{-4, -2}) || !reflect.DeepEqual(r.RejectedPKs, []int64{-4, 9, -4, -2}) {
		t.Fatal(r, err)
	}
}

func TestImportReviewPartialIndexProgressIsResumable(t *testing.T) {
	ctx := context.Background()
	m, input, output := reviewImportFixture(t, "{\"id\":1,\"embedding\":[0,0]}\n{\"id\":2,\"embedding\":[1,2]}\n")
	m.Indexes = []ImportIndex{{Name: "items_l2", Column: "embedding", Method: "hnsw", Metric: "l2", M: 2, EFConstruction: 4}, {Name: "items_cos", Column: "embedding", Method: "hnsw", Metric: "cosine", M: 2, EFConstruction: 4}}
	for _, resume := range []bool{false, true} {
		r, err := Import(ctx, input, output, m, ImportOptions{Apply: true, Resume: resume})
		if err == nil || r.Status != "data_complete_index_pending" || r.Accepted != 2 {
			t.Fatal(r, err)
		}
		db, err := sqlite.Open(output, sqlite.Options{ReadOnly: true, NoCreate: true})
		if err != nil {
			t.Fatal(err)
		}
		j := reviewImportJob(t, db)
		if j.IndexesDone != 1 || j.Status != "data_complete_index_pending" || j.Cursor != 2 {
			t.Fatal(j)
		}
		if err = db.CheckExtensionIndex(ctx, "items_l2"); err != nil {
			t.Fatal(err)
		}
		reviewImportCount(t, db, "items", 2)
		if err := db.Close(); err != nil {
			t.Fatal(err)
		}
	}
}
