package postgres

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

// All native fixtures use intact freshly imported files and ordinary committed
// stop/close/resume/read operations. Error controls below are options, source
// parsing, reference arithmetic or fabricated in-memory results, never native
// file faults, physical mutation, missing selected paths or recovery probes.
func TestImportDatasetDistancesMappedFormatsAndResume(t *testing.T) {
	for _, format := range []string{"csv", "ndjson"} {
		t.Run(format, func(t *testing.T) {
			inputs := []struct {
				pk string
				v  string
			}{
				{"9223372036854775807", "[3.4028234663852886e38,-3.4028234663852886e38]"},
				{"-7", "[3,4]"}, {"-9223372036854775808", "[-0,1.401298464324817e-45]"},
				{"9", "[1,1]"}, {"0", "[1.00000011920928955078125,1]"}, {"-2", "[3,4]"}, {"11", "null"},
				{"-7", "[3,4]"}, {"20", "[1e-100,1]"},
			}
			var data strings.Builder
			if format == "csv" {
				w := csv.NewWriter(&data)
				_ = w.Write([]string{"id", "embedding"})
				for _, row := range inputs {
					value := strings.NewReplacer("[", "{", "]", "}").Replace(row.v)
					if row.v == "null" {
						value = `\N`
					}
					_ = w.Write([]string{row.pk, value})
				}
				w.Flush()
				if err := w.Error(); err != nil {
					t.Fatal(err)
				}
			} else {
				for _, row := range inputs {
					fmt.Fprintf(&data, "{\"id\":%s,\"embedding\":%s}\n", row.pk, row.v)
				}
			}
			m, in, out := importFixture(t, format, data.String())
			m.Table = "mapped_vectors"
			m.Fields[0].Target = "signed_key"
			m.Fields[1].Target = "dense"
			ctx := context.Background()
			paused, err := Import(ctx, in, out, m, ImportOptions{Apply: true, OnError: "quarantine", BatchRows: 3, StopAfterBatches: 1})
			if err != nil || paused.Status != "paused_committed" {
				t.Fatal(paused, err)
			}
			imported, err := Import(ctx, in, out, m, ImportOptions{Apply: true, Resume: true, OnError: "quarantine", BatchRows: 2})
			if err != nil {
				t.Fatal(err)
			}
			options := ImportDistanceOptions{Column: "dense", QueryCount: 4, TopK: 2}
			report, err := VerifyImportDistances(ctx, in, out, m, options)
			if err != nil {
				t.Fatal(err)
			}
			r := report.Distances
			if report.Import.Status != "complete_with_rejections" || report.Import.Accepted != 7 || report.Import.Rejected != 2 || !reflect.DeepEqual(report.Import.MissingPKs, []int64{20}) || report.Import.CanonicalSHA256 != imported.CanonicalSHA256 || report.Import.CanonicalSHA256 != report.Import.DatabaseCanonicalSHA256 {
				t.Fatal(report.Import)
			}
			if r.Status != "passed" || r.CandidateRows != 6 || r.NullRows != 1 || r.NativeRowsChecked != 7 || r.NormsChecked != 6 || r.SourceField != "embedding" || r.PrimaryKeyField != "signed_key" || r.Dimensions != 2 || r.DistancesChecked != 96 || r.TopKChecked != 32 || r.ZeroNormRows != 0 || r.QueryCoverage != "sampled_queries_all_candidates" {
				t.Fatalf("report: %+v", r)
			}
			if !reflect.DeepEqual(r.QueryPKs, []int64{math.MinInt64, -7, 0, math.MaxInt64}) {
				t.Fatal(r.QueryPKs)
			}
			for _, q := range r.Queries {
				if q.Status != "passed" || !q.Exact || len(q.All) != 6 || len(q.TopK) != 2 || q.TopKBoundary == "" {
					t.Fatal(q)
				}
			}
			if !strings.HasPrefix(report.Import.ANNRecall, "not_run") || !strings.HasPrefix(r.ANNRecall, "not_run") {
				t.Fatal("exact-only verification claimed recall")
			}
			again, err := VerifyImportDistances(ctx, in, out, m, options)
			if err != nil || !reflect.DeepEqual(report, again) {
				t.Fatal("not deterministic", err)
			}
			legacy, err := VerifyImport(ctx, in, out, m)
			if err != nil || !strings.HasPrefix(legacy.ExactDistances, "not_run") {
				t.Fatal("legacy API changed", legacy, err)
			}
			options.QueryCount = 0
			options.QueryPKs = []int64{9, -7}
			supplied, err := VerifyImportDistances(ctx, in, out, m, options)
			if err != nil || !reflect.DeepEqual(supplied.Distances.QueryPKs, options.QueryPKs) || supplied.Distances.QuerySelection != "supplied_source_pks" {
				t.Fatal(supplied, err)
			}
		})
	}
}

func TestImportDatasetDistancesEmptyNullZeroAndTies(t *testing.T) {
	for _, tc := range []struct {
		name, data, status string
		norms, zeros       int
	}{
		{"empty", "", "skipped_no_nonnull_vectors", 0, 0},
		{"null", "{\"id\":1,\"embedding\":null}\n", "skipped_no_nonnull_vectors", 0, 0},
		{"zero", "{\"id\":0,\"embedding\":[0,-0]}\n{\"id\":1,\"embedding\":[1,1]}\n{\"id\":2,\"embedding\":[1,-1]}\n", "partial_skipped_metrics", 3, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, in, out := importFixture(t, "ndjson", tc.data)
			ctx := context.Background()
			if _, err := Import(ctx, in, out, m, ImportOptions{Apply: true}); err != nil {
				t.Fatal(err)
			}
			report, err := VerifyImportDistances(ctx, in, out, m, ImportDistanceOptions{Column: "embedding", TopK: 2})
			if err != nil {
				t.Fatal(err)
			}
			r := report.Distances
			if r.Status != tc.status || r.NormsChecked != tc.norms || r.ZeroNormRows != tc.zeros {
				t.Fatal(r)
			}
			if tc.name == "zero" {
				ambiguous := false
				for _, q := range r.Queries {
					if q.Metric == vector.Cosine {
						if q.Status != "skipped_zero_norm" || q.Exact || len(q.All) != 0 {
							t.Fatal(q)
						}
					} else if q.QueryPK == 0 && q.Metric == vector.L2 {
						ambiguous = strings.HasPrefix(q.TopKBoundary, "ambiguous")
					}
				}
				if !ambiguous {
					t.Fatal("mathematical top-K tie not disclosed")
				}
				skipped, err := VerifyImportDistances(ctx, in, out, m, ImportDistanceOptions{Column: "embedding", Metrics: []vector.MetricKind{vector.Cosine}})
				if err != nil || skipped.Distances.Status != "skipped_no_defined_metrics" {
					t.Fatal(skipped, err)
				}
			}
		})
	}
}

func TestImportDatasetDistanceOptionsAndBudgets(t *testing.T) {
	m, in, out := importFixture(t, "ndjson", "{\"id\":1,\"embedding\":[3,4]}\n{\"id\":2,\"embedding\":[1,2]}\n")
	for _, o := range []ImportDistanceOptions{
		{}, {Column: "id"}, {Column: "embedding", Metrics: []vector.MetricKind{"unknown"}},
		{Column: "embedding", Metrics: []vector.MetricKind{vector.L2, vector.L2}},
		{Column: "embedding", QueryCount: 17}, {Column: "embedding", QueryCount: -1},
		{Column: "embedding", QueryCount: 1, QueryPKs: []int64{1}}, {Column: "embedding", QueryPKs: []int64{1, 1}},
		{Column: "embedding", MaxRows: 10001}, {Column: "embedding", TopK: -1},
		{Column: "embedding", MaxPairs: 100001}, {Column: "embedding", MaxComponentWork: 8000001},
	} {
		if _, _, err := o.normalized(m); err == nil {
			t.Fatal("invalid options accepted", o)
		}
	}
	for _, raw := range []string{`{"column":"embedding","unknown":1}`, `{"column":"embedding","column":"embedding"}`, `{"column":"embedding"} {}`, `null`, strings.Repeat(" ", MaxImportManifestBytes+1)} {
		if _, err := DecodeImportDistanceOptions(strings.NewReader(raw)); err == nil {
			t.Fatal("invalid options JSON accepted", raw[:min(100, len(raw))])
		}
	}
	o, err := DecodeImportDistanceOptions(strings.NewReader(`{"column":"embedding","query_pks":[-9223372036854775808,9223372036854775807],"metrics":["l2"]}`))
	if err != nil || !reflect.DeepEqual(o.QueryPKs, []int64{math.MinInt64, math.MaxInt64}) {
		t.Fatal(o, err)
	}
	ctx := context.Background()
	if _, err = Import(ctx, in, out, m, ImportOptions{Apply: true}); err != nil {
		t.Fatal(err)
	}
	for _, o := range []ImportDistanceOptions{{Column: "embedding", MaxRows: 1}, {Column: "embedding", MaxPairs: 1}, {Column: "embedding", MaxComponentWork: 1}, {Column: "embedding", QueryPKs: []int64{99}}} {
		r, e := VerifyImportDistances(ctx, in, out, m, o)
		if e == nil || r.Distances.Status != "failed" || r.Import.Status != "verification_failed" {
			t.Fatal("resource/query control", r, e)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = VerifyImportDistances(canceled, in, out, m, ImportDistanceOptions{Column: "embedding"}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, err = VerifyImportDistances(nil, in, out, m, ImportDistanceOptions{Column: "embedding"}); err == nil {
		t.Fatal("nil context accepted")
	}
}

func TestImportDatasetDistanceReferenceAndComparisonControls(t *testing.T) {
	a, b := []float32{3, 4}, []float32{0, 0}
	for _, tc := range []struct {
		metric vector.MetricKind
		want   float64
	}{{vector.L2, 5}, {vector.L1, 7}, {vector.NegativeIP, 0}} {
		if got := importDistanceReference(a, b, tc.metric); got != tc.want {
			t.Fatal(tc.metric, got)
		}
	}
	max := float32(math.MaxFloat32)
	if got := importDistanceReference([]float32{max, 1, max}, []float32{max, 1, -max}, vector.NegativeIP); got != -1 {
		t.Fatal("high precision cancellation", got)
	}
	tiny := float32(math.SmallestNonzeroFloat32)
	if got := importDistanceReferenceNorm([]float32{tiny, 0}); got != float64(tiny) {
		t.Fatal("subnormal norm", got)
	}
	if tol := importDistanceErrorBound([]float32{tiny, 0}, []float32{0, 0}, vector.L2, float64(tiny)); tol >= float64(tiny) {
		t.Fatal("bound hides zero/subnormal error")
	}
	refs := map[int64]importDistanceReferenceValue{-3: {1, 1e-14}, 7: {1, 1e-14}, 10: {2, 2e-14}}
	good := []vector.Hit{{RowID: -3, Distance: 1}, {RowID: 7, Distance: 1}, {RowID: 10, Distance: 2}}
	checked, err := checkImportDistanceHits(good, refs)
	if err != nil || !strings.HasPrefix(importDistanceBoundary(checked, 1), "ambiguous") || !strings.HasPrefix(importDistanceBoundary(checked, 2), "clear") {
		t.Fatal(checked, err)
	}
	for _, bad := range [][]vector.Hit{good[:2], {{RowID: -3, Distance: 1}, {RowID: -3, Distance: 1}, {RowID: 10, Distance: 2}}, {{RowID: -3, Distance: 1}, {RowID: 8, Distance: 1}, {RowID: 10, Distance: 2}}, {{RowID: 7, Distance: 1}, {RowID: -3, Distance: 1}, {RowID: 10, Distance: 2}}, {{RowID: -3, Distance: 1}, {RowID: 7, Distance: 1}, {RowID: 10, Distance: 2.1}}, {{RowID: -3, Distance: math.NaN()}, {RowID: 7, Distance: 1}, {RowID: 10, Distance: 2}}} {
		if _, e := checkImportDistanceHits(bad, refs); e == nil {
			t.Fatal("fabricated wrong result accepted", bad)
		}
	}
	if _, err = json.Marshal(checked); err != nil {
		t.Fatal(err)
	}
}

func TestImportDatasetDistancesNearTieAndMultipleColumns(t *testing.T) {
	data := "{\"id\":1,\"embedding\":[1,1],\"other\":[3,4]}\n{\"id\":2,\"embedding\":[1.0000001192092896,1],\"other\":[0,0]}\n{\"id\":3,\"embedding\":[1,1.0000001192092896],\"other\":[1,2]}\n"
	m, in, out := importFixture(t, "ndjson", data)
	m.Fields = append(m.Fields, ImportField{Source: "other", Target: "other_vector", Type: "vector", Encoding: "json_array", Dimensions: 2})
	ctx := context.Background()
	if _, err := Import(ctx, in, out, m, ImportOptions{Apply: true}); err != nil {
		t.Fatal(err)
	}
	r, err := VerifyImportDistances(ctx, in, out, m, ImportDistanceOptions{Column: "embedding", QueryPKs: []int64{1}, Metrics: []vector.MetricKind{vector.Cosine}, TopK: 2})
	if err != nil || r.Distances.Status != "passed" || !reflect.DeepEqual(r.Distances.UnselectedVectorColumns, []string{"other_vector"}) || !strings.HasPrefix(r.Distances.Queries[0].TopKBoundary, "ambiguous") {
		t.Fatal(r, err)
	}
	// The unselected zero vector must not cause selected-column cosine to skip.
	if r.Distances.ZeroNormRows != 0 {
		t.Fatal(r.Distances)
	}
	if r.Distances.Queries[0].All[1].Reference <= 0 || r.Distances.Queries[0].All[1].Reference >= r.Distances.Queries[0].All[1].Tolerance {
		t.Fatal("near-tie reference was not exercised", r.Distances.Queries[0])
	}
}

func TestImportDatasetDistancesWideFiniteVectors(t *testing.T) {
	const dims = 257
	var data strings.Builder
	for row := 0; row < 5; row++ {
		fmt.Fprintf(&data, "{\"id\":%d,\"embedding\":[", row)
		for i := 0; i < dims; i++ {
			if i > 0 {
				data.WriteByte(',')
			}
			x := float32((i%17)-8) * float32(row+1) / 8
			if i == 0 {
				x = math.MaxFloat32
			}
			if i == 1 {
				x = math.SmallestNonzeroFloat32
			}
			if i == 2 && row%2 == 0 {
				x = -math.MaxFloat32
			}
			fmt.Fprintf(&data, "%.9g", x)
		}
		data.WriteString("]}\n")
	}
	m, in, out := importFixture(t, "ndjson", data.String())
	m.Fields[1].Dimensions = dims
	ctx := context.Background()
	if _, err := Import(ctx, in, out, m, ImportOptions{Apply: true, BatchRows: 2}); err != nil {
		t.Fatal(err)
	}
	r, err := VerifyImportDistances(ctx, in, out, m, ImportDistanceOptions{Column: "embedding", QueryCount: 5, TopK: 3})
	if err != nil || r.Distances.Status != "passed" || r.Distances.DistancesChecked != 100 || r.Distances.NormsChecked != 5 || r.Distances.Dimensions != dims || r.Distances.QueryCoverage != "all_source_vectors_as_queries_all_candidates" {
		t.Fatal(r, err)
	}
}
