package reportcheck

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Exact historical owner bytes are compile-time embedded. Runtime tests acquire
// no descriptor or native database. These are report controls, not new native
// measurements; original source/report/DB evidence is never rewritten.
//
//go:embed testdata/schema2-hnsw.json
var legacyHNSW []byte

//go:embed testdata/schema2-ivf.json
var legacyIVF []byte

func refreshModelHashes(t *testing.T, r *Report) {
	t.Helper()
	var e error
	r.WorkloadSHA256, e = workloadDigest(*r)
	if e != nil {
		t.Fatal(e)
	}
	r.HardwareSHA256, e = hardwareDigest(*r)
	if e != nil {
		t.Fatal(e)
	}
	r.BuildSHA256, e = buildDigest(*r)
	if e != nil {
		t.Fatal(e)
	}
}
func modelMethodReport(t *testing.T, method string) Report {
	t.Helper()
	raw := legacyHNSW
	if method == "ivfflat" {
		raw = legacyIVF
	}
	var r Report
	// Old extra generic limits keys are intentionally ignored only by this
	// descriptor-free fixture constructor. Production DecodeReport rejects v2.
	if e := json.Unmarshal(raw, &r); e != nil {
		t.Fatal(e)
	}
	r.SchemaVersion = schemaVersion
	r.HarnessRevision = harnessRevision
	r.MeasurementVersion = measurementVersion
	var admitErr error
	r.Admission, admitErr = r.Config.admission()
	if admitErr != nil {
		t.Fatal(admitErr)
	}
	r.Limits = nativeLimits()
	r.QueryPolicy = queryPolicy(r.Config)
	r.TimingSemantics = timingPolicy()
	var e error
	r.Algorithm, e = canonicalAlgorithm(r.Config)
	if e != nil {
		t.Fatal(e)
	}
	r.DataDirectory = "/descriptor-free-model/retained"
	r.ReportPath = r.DataDirectory + "/report.json"
	r.Hardware.ObservationNotes = append(r.Hardware.ObservationNotes, "cgroup_memory_max: unavailable", "cgroup_cpu_max: unavailable")
	r.StartedUTC = "2026-10-10T00:00:00Z"
	r.FinishedUTC = "2026-10-10T00:01:00Z"
	r.ElapsedSeconds = 60
	r.Notes = []string{"Descriptor-free producer-shaped model using unchanged historical raw observations; not new native evidence."}
	refreshModelHashes(t, &r)
	return r
}
func modelReport(t *testing.T) Report { return modelMethodReport(t, "hnsw") }
func TestANNHistoricalSchema2PreservedRejected(t *testing.T) {
	for method, v := range map[string]struct {
		raw  []byte
		hash string
	}{"hnsw": {legacyHNSW, "14f6fb98d01d05b650f08e54273278c9297c3db214c79f48a14875fd748009da"}, "ivfflat": {legacyIVF, "dbc2e1ab35341ca372c62df6f46d2b65adf2299fb955d8eaf961f99885c7f045"}} {
		sum := sha256.Sum256(v.raw)
		if hex.EncodeToString(sum[:]) != v.hash {
			t.Fatal(method, "legacy bytes changed")
		}
		if _, e := DecodeReport(v.raw); e == nil {
			t.Fatal("schema2 silently admitted")
		}
	}
}
func TestANNCompleteProducerWireRoundTrip(t *testing.T) {
	for _, method := range []string{"hnsw", "ivfflat"} {
		t.Run(method, func(t *testing.T) {
			r := modelMethodReport(t, method)
			if e := r.ValidatePass(); e != nil {
				t.Fatal(e)
			}
			raw, e := json.MarshalIndent(r, "", "  ")
			if e != nil {
				t.Fatal(e)
			}
			decoded, e := DecodeReport(raw)
			if e != nil {
				t.Fatal(e)
			}
			again, e := json.MarshalIndent(decoded, "", "  ")
			if e != nil {
				t.Fatal(e)
			}
			if !bytes.Equal(raw, again) {
				t.Fatal("producer-shaped report changes on wire round-trip")
			}
			if decoded.WorkloadSHA256 != r.WorkloadSHA256 || decoded.Limits != r.Limits || decoded.QueryPolicy != r.QueryPolicy {
				t.Fatal("material identity changed")
			}
			cfg, e := parseAlgorithm(decoded.Algorithm)
			if e != nil {
				t.Fatal(e)
			}
			expected, e := algorithm(r.Config)
			if e != nil || cfg != expected || cfg.Seed != 2611923443488327891 {
				t.Fatal("lossy algorithm/seed", cfg, e)
			}
			if method == "ivfflat" && (cfg.Lists != 8 || cfg.TrainingSamples != 10000 || cfg.TrainingIterations != 20 || cfg.MaxTrainingWork != 268435456) {
				t.Fatal("lost IVF config")
			}
		})
	}
}
func TestANNPercentilesAndRawMath(t *testing.T) {
	v := []float64{4, 1, 3, 2}
	before := append([]float64(nil), v...)
	p, e := percentile(v, .95)
	if e != nil || p != 3 || !reflect.DeepEqual(v, before) {
		t.Fatal(p, e, v)
	}
	for _, v := range [][]float64{nil, {math.NaN()}, {math.Inf(1)}, {-1}} {
		if _, e := percentile(v, .5); e == nil {
			t.Fatal("invalid math")
		}
	}
	for _, p := range []float64{-1, 2, math.NaN()} {
		if _, e := percentile([]float64{1}, p); e == nil {
			t.Fatal("invalid quantile")
		}
	}
	r := modelReport(t)
	s := r.Samples[0]
	s.Observations[0].Hits = 2
	if e := summarizeSample(&s); e == nil {
		t.Fatal("invalid hits")
	}
	s = r.Samples[1]
	s.Observations[0].ANN.Stats.ANNDistanceEvaluations = math.MaxInt64
	s.Observations[0].ANN.Stats.ANNCentroidScores = math.MaxInt64
	if e := summarizeSample(&s); e == nil {
		t.Fatal("counter sum overflow")
	}
}
func TestANNStrictReportControls(t *testing.T) {
	cases := map[string]func(*Report){"status": func(r *Report) { r.Status = "skipped_unsupported" }, "revision": func(r *Report) { r.HarnessRevision = "ordinary-ann-v2" }, "schema": func(r *Report) { r.SchemaVersion = 2 }, "errors": func(r *Report) { r.Errors = []string{"failure"} }, "cleanup latch": func(r *Report) { r.UnverifiedRetirement = true }, "open owner": func(r *Report) { r.UnverifiedCloseHandles = 1 }, "close failure": func(r *Report) { r.CloseFailures = 1 }, "missing DB": func(r *Report) { delete(r.Ownership, "DB") }, "unexpected owner": func(r *Report) { r.Ownership["untracked"] = Ownership{} }, "missing Tx": func(r *Report) { r.Ownership["Tx"] = Ownership{Acquired: 1, Retired: 1} }, "reopen": func(r *Report) { r.ReopenVerified = false }, "dataset": func(r *Report) { r.ReopenedDatasetSHA256 = strings.Repeat("c", 64) }, "hash": func(r *Report) { r.WorkloadSHA256 = "bogus" }, "config": func(r *Report) { r.Config.Queries++ }, "admission": func(r *Report) { r.Admission.TotalComponentWork++ }, "algorithm": func(r *Report) {
		r.Algorithm = bytes.Replace(r.Algorithm, []byte(`"seed":2611923443488327891`), []byte(`"seed":2611923443488327892`), 1)
	}, "timing": func(r *Report) { r.ElapsedSeconds = math.NaN() }, "elapsed reconciliation": func(r *Report) { r.ElapsedSeconds++ }, "date": func(r *Report) { r.FinishedUTC = "Sat, 10 Oct 2026 00:01:00 UTC" }, "size": func(r *Report) { r.IndexBytesDelta++ }, "space quota": func(r *Report) { r.MaxObservedDataBytes = r.Config.MaxDataBytes + 1 }, "heap quota": func(r *Report) { r.MaxObservedHeapBytes = r.Config.MaxMemoryBytes + 1 }, "samples": func(r *Report) { r.Samples = r.Samples[:2] }, "summary": func(r *Report) { r.Samples[0].Recall = .5 }, "negative stats": func(r *Report) { r.Samples[0].Observations[0].ANN.Stats.ANNNodeReads = -1 }, "native index": func(r *Report) { r.Samples[0].Observations[0].ANN.Plan.Indexes = nil }, "wrong raw hit": func(r *Report) { r.Samples[0].Observations[0].Hits = 0 }, "duplicate hit": func(r *Report) { r.Samples[1].Observations[0].ANN.Hits[1] = r.Samples[1].Observations[0].ANN.Hits[0] }, "incomplete": func(r *Report) { r.PendingObservation = &Observation{} }, "field": func(r *Report) { r.Samples[0].Observations[0].ANN.Plan.Field = "different_column" }, "EF policy": func(r *Report) {
		r.Samples[0].Observations[0].ANN.Plan.ANNEFSearch = 999
		r.Samples[0].Observations[0].ANN.Plan.ANNMaxEFSearch = 999
	}, "build settings": func(r *Report) { r.BuildSettings = nil }, "exact components": func(r *Report) { r.Samples[0].Observations[0].Exact.Stats.ScoredComponents = 99999 }, "limits": func(r *Report) { r.Limits.ANN.MaxNodeReads++ }, "timing policy": func(r *Report) { r.TimingSemantics.Query = "different" }, "query policy": func(r *Report) { r.QueryPolicy.EFSearch++ }, "runtime": func(r *Report) { r.GoVersion = "go1.28.0" }, "CGO": func(r *Report) { r.BuildSettings["CGO_ENABLED"] = "1" }, "race": func(r *Report) { r.BuildSettings["-race"] = "true" }, "source": func(r *Report) { r.Config.SourceID = strings.Repeat("c", 40) }, "hardware": func(r *Report) { r.Hardware.CPUModel = "changed" }, "hardware missing": func(r *Report) { r.Hardware.CPUAffinity = "" }, "hardware unavailable": func(r *Report) { r.Hardware.ObservationNotes = nil }, "paths": func(r *Report) { r.ReportPath = "/another/report.json" }, "data path": func(r *Report) { r.DataDirectory = "" }, "query counter quota": func(r *Report) { r.Samples[0].Observations[0].ANN.Stats.ANNNodeReads = math.MaxInt64 }, "method counter": func(r *Report) { r.Samples[0].Observations[0].ANN.Stats.ANNCentroidScores = 1 }, "query policy fallback": func(r *Report) { r.QueryPolicy.FallbackPolicy = "allow" }}
	for name, f := range cases {
		t.Run(name, func(t *testing.T) {
			r := modelReport(t)
			f(&r)
			if e := r.ValidatePass(); e == nil {
				t.Fatal("invalid report accepted")
			}
		})
	}
}
func TestANNResultHitControls(t *testing.T) {
	r := modelReport(t)
	o := r.Samples[1].Observations[0]
	for _, f := range []func(*sqlite.VectorResult){func(r *sqlite.VectorResult) { r.Hits = r.Hits[:9] }, func(r *sqlite.VectorResult) { r.Hits[0].RowID = 0 }, func(r *sqlite.VectorResult) { r.Hits[0].Distance = math.NaN() }, func(r *sqlite.VectorResult) { r.Hits[1].Distance = 0 }, func(r *sqlite.VectorResult) { r.Hits[1].RowID = r.Hits[0].RowID }} {
		v := o.ANN
		v.Hits = append([]vector.Hit(nil), v.Hits...)
		f(&v)
		if _, e := intersection(o.Exact, v, 10, r.Config.Rows); e == nil {
			t.Fatal("invalid result")
		}
	}
}
