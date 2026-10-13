package reportcheck

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path"
	"reflect"
	"strings"
	"time"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func validStats(s sqlite.VectorStats) bool {
	var valid func(reflect.Value) bool
	valid = func(v reflect.Value) bool {
		if v.Kind() == reflect.Struct {
			for i := 0; i < v.NumField(); i++ {
				if !valid(v.Field(i)) {
					return false
				}
			}
			return true
		}
		return v.Kind() == reflect.Int64 && v.Int() >= 0
	}
	return valid(reflect.ValueOf(s))
}
func exactPlan(c Config, k int) sqlite.VectorPlan {
	return sqlite.VectorPlan{QueryPlan: sqlite.QueryPlan{Access: "SCAN", Table: "items", Method: "exact_vector", MethodVersion: 1, Fields: []string{"embedding"}, EstimatedCandidates: -1, Reason: "exact full scan; WHERE before distance"}, Field: "embedding", Metric: "l2", Dimensions: c.Dimensions, Limit: k, Exact: true}
}
func nativePlan(c Config, k, ef, probes int) sqlite.VectorPlan {
	p := exactPlan(c, k)
	p.Exact = false
	p.Access = "SEARCH"
	p.Method = c.Method
	p.Recheck = true
	p.VectorRanking = "ann_exact_rescore"
	p.ANNOpclass = "vector_l2_ops"
	p.ANNGeneration = 1
	p.ANNStopReason = "eligible_limit_satisfied"
	q := queryPolicy(c)
	p.Indexes = []string{q.Index}
	p.Reason = "lazy HNSW traversal with original WHERE and exact base-row rescoring"
	p.ANNEFSearch = q.EFSearch
	p.ANNMaxEFSearch = q.MaxEFSearch
	p.ANNEffectiveEF = ef
	if c.Method == "ivfflat" {
		p.Reason = "lazy IVFFlat selected-list ranges with original WHERE and exact base-row rescoring"
		p.ANNLists = c.Lists
		p.ANNTrainingState = "ready"
		p.ANNProbes = q.Probes
		p.ANNMaxProbes = q.MaxProbes
		p.ANNEffectiveProbes = probes
	}
	return p
}
func validateObservation(o Observation, k int, c Config) error {
	if !o.Completed || o.Stage != "complete" {
		return errors.New("incomplete observation")
	}
	hit, e := intersection(o.Exact, o.ANN, k, c.Rows)
	if e != nil {
		return e
	}
	if hit != o.Hits {
		return errors.New("incorrect intersection count")
	}
	// Paired calls use the same query, snapshot, original dense source vector,
	// and vector.L2.Distance implementation. ANN approximation selects rows;
	// it does not approximate these final exact-rescore distances. intersection
	// already checked finite values and unique RowIDs. Numeric equality also
	// treats +0 and -0 as equal; there is no independent rounding tolerance.
	exactDistances := make(map[int64]float64, len(o.Exact.Hits))
	for _, h := range o.Exact.Hits {
		exactDistances[h.RowID] = h.Distance
	}
	for _, h := range o.ANN.Hits {
		if d, ok := exactDistances[h.RowID]; ok && h.Distance != d {
			return errors.New("same-row exact/rescore distance mismatch")
		}
	}
	if !validStats(o.Exact.Stats) || !validStats(o.ANN.Stats) {
		return errors.New("negative or unrecognized counter")
	}
	if !reflect.DeepEqual(o.Exact.Plan, exactPlan(c, k)) {
		return errors.New("exact plan identity mismatch")
	}
	exact := sqlite.VectorStats{QueryStats: sqlite.QueryStats{BaseRowFetches: int64(c.Rows), ReturnedRows: int64(k)}, ScoredRows: int64(c.Rows), ScoredComponents: int64(c.Rows+1) * int64(c.Dimensions)}
	if o.Exact.Stats != exact {
		return errors.New("exact source/component/counter conservation mismatch")
	}
	s, p := o.ANN.Stats, o.ANN.Plan
	lim := nativeLimits()
	q := queryPolicy(c)
	alg, e := algorithm(c)
	if e != nil {
		return e
	}
	if p.Exact || p.ANNFallback != "" {
		return errors.New("exact fallback is retained but cannot pass schema3 policy")
	}
	if !reflect.DeepEqual(p, nativePlan(c, k, p.ANNEffectiveEF, p.ANNEffectiveProbes)) {
		return errors.New("native plan material identity mismatch")
	}
	if s.ANNIterations < 1 || s.ANNIterations > int64(lim.ANN.MaxIterations) || s.ANNStateReads != 1 || s.BaseRowFetches < int64(k) || s.BaseRowFetches > int64(c.Rows) || s.BaseRowFetches > int64(lim.Vector.MaxCandidates) || s.Candidates != s.BaseRowFetches || s.ANNIdentityLookups != s.BaseRowFetches || s.ANNExactRescores != s.BaseRowFetches || s.ScoredRows != s.BaseRowFetches || s.ScoredComponents != (s.BaseRowFetches+1)*int64(c.Dimensions) || s.ReturnedRows != int64(k) || s.FilteredRows != 0 || s.NullRows != 0 || s.ANNPeakBytes < 1 || s.ANNPeakBytes > lim.DB.MaxSortBytes {
		return errors.New("native base/component/iteration/memory conservation mismatch")
	}
	// Bounds precede all additions/products used to validate untrusted counters.
	if s.ANNNodeReads > int64(lim.ANN.MaxNodeReads) || s.ANNCentroidReads > int64(lim.ANN.MaxCentroidReads) || s.ANNMemberKeysRead > int64(lim.ANN.MaxMemberKeysRead) || s.ANNDistanceEvaluations > int64(lim.ANN.MaxDistanceEvaluations) || s.ANNCentroidScores > int64(lim.ANN.MaxDistanceEvaluations) {
		return errors.New("native query work quota exceeded")
	}
	distance, e := checkedSum(uint64(s.ANNDistanceEvaluations), uint64(s.ANNCentroidScores))
	if e != nil || distance > uint64(lim.ANN.MaxDistanceEvaluations) {
		return errors.New("distance quota exceeded")
	}
	component, e := checkedProduct(distance, uint64(c.Dimensions))
	if e != nil {
		return e
	}
	component, e = checkedSum(component, uint64(s.ScoredComponents))
	if e != nil || component > lim.Vector.MaxComponents {
		return errors.New("cumulative component quota exceeded")
	}
	// Every remaining native-API counter has a fixed zero meaning in this
	// unfiltered dense workload. Future fields default to rejection, not admission.
	rest := s.QueryStats
	rest.ANNStateReads = 0
	rest.ANNNodeReads = 0
	rest.ANNEdgeReads = 0
	rest.ANNVisited = 0
	rest.ANNDistanceEvaluations = 0
	rest.ANNIdentityLookups = 0
	rest.ANNCentroidScores = 0
	rest.ANNCentroidReads = 0
	rest.ANNListsOpened = 0
	rest.ANNMemberKeysRead = 0
	rest.ANNIterations = 0
	rest.ANNExactRescores = 0
	rest.ANNPeakBytes = 0
	rest.Candidates = 0
	rest.BaseRowFetches = 0
	rest.ReturnedRows = 0
	if rest != (sqlite.QueryStats{}) {
		return errors.New("unexpected native API counter")
	}
	if c.Method == "hnsw" {
		ef := max(q.EFSearch, min(k, q.MaxEFSearch))
		for i := int64(1); i < s.ANNIterations; i++ {
			if ef == q.MaxEFSearch {
				return errors.New("iterations after maximum EF")
			}
			ef = min(q.MaxEFSearch, max(ef+1, ef*2))
		}
		// Each successful traversal caches one read per visited node. Each
		// deduplicated source recheck adds one further node read. Across widening
		// iterations visits are cumulative; source rechecks remain deduplicated.
		// Every rechecked candidate was visited and every visit was scored.
		// Nonnegativity and quotas were checked above. Guarded subtraction
		// avoids adding untrusted signed counters (and thus avoids overflow).
		if s.ANNNodeReads < s.BaseRowFetches || s.ANNNodeReads-s.BaseRowFetches != s.ANNVisited || s.ANNVisited < s.ANNExactRescores || s.ANNDistanceEvaluations < s.ANNVisited {
			return errors.New("HNSW traversal/read/rescore conservation mismatch")
		}
		if p.ANNEffectiveEF != ef || p.ANNEffectiveProbes != 0 || s.ANNVisited < 1 || s.ANNVisited > int64(alg.MaxVisited)*s.ANNIterations || s.ANNDistanceEvaluations < 1 || s.ANNDistanceEvaluations > int64(alg.MaxDistanceEvaluations)*s.ANNIterations || s.ANNEdgeReads > s.ANNNodeReads*int64(2*alg.M*32) || s.ANNCentroidScores != 0 || s.ANNCentroidReads != 0 || s.ANNListsOpened != 0 || s.ANNMemberKeysRead != 0 {
			return errors.New("HNSW policy/counter mismatch")
		}
	} else {
		probes := q.Probes
		for i := int64(1); i < s.ANNIterations; i++ {
			if probes == q.MaxProbes {
				return errors.New("iterations after maximum probes")
			}
			probes = min(q.MaxProbes, max(probes+1, probes*2))
		}
		if p.ANNEffectiveEF != 0 || p.ANNEffectiveProbes != probes || s.ANNListsOpened != int64(probes) || s.ANNCentroidReads != int64(c.Lists) || s.ANNCentroidScores != int64(c.Lists) || s.ANNVisited != int64(c.Lists) || s.ANNMemberKeysRead != s.BaseRowFetches || s.ANNNodeReads != 0 || s.ANNEdgeReads != 0 || s.ANNDistanceEvaluations != 0 {
			return errors.New("IVF training/probe/counter mismatch")
		}
		// The immutable, unfiltered NOT NULL fixture has one membership per
		// source row. Widening opens each list once and consumes its full range.
		// Opening every list must therefore cover exactly Config.Rows members;
		// the checks above already equate members, fetches, and exact rescores.
		if s.ANNListsOpened == int64(c.Lists) && s.ANNMemberKeysRead != int64(c.Rows) {
			return errors.New("IVF all-lists/source coverage mismatch")
		}
	}
	return nil
}
func validateProvenance(r Report) error {
	if r.GoVersion != "go1.27.2" || r.CPUs < 1 || r.GOMAXPROCS < 1 || r.CPUs > 1<<20 || r.GOMAXPROCS > 1<<20 {
		return errors.New("unsupported runtime identity")
	}
	if (r.GOOS != "linux" && r.GOOS != "darwin" && r.GOOS != "windows") || (r.GOARCH != "amd64" && r.GOARCH != "arm64") {
		return errors.New("unsupported platform identity")
	}
	expected := map[string]string{"-buildmode": "exe", "-compiler": "gc", "-trimpath": "true", "CGO_ENABLED": "0", "GOOS": r.GOOS, "GOARCH": r.GOARCH, "main_path": "github.com/ms-xiaosnow/gosqlite", "main_version": "(devel)", "main_sum": ""}
	for key := range r.BuildSettings {
		if _, ok := expected[key]; !ok && key != "GOAMD64" && key != "GOARM64" {
			return fmt.Errorf("unsupported build setting %s", key)
		}
	}
	for k, v := range expected {
		got, ok := r.BuildSettings[k]
		if !ok || got != v {
			return fmt.Errorf("missing/unsupported build setting %s", k)
		}
	}
	for _, k := range []string{"-race", "-msan", "-asan", "-cover", "-gcflags", "-ldflags"} {
		if r.BuildSettings[k] != "" {
			return fmt.Errorf("instrumented/nondefault build setting %s", k)
		}
	}
	if r.GOARCH == "amd64" && r.BuildSettings["GOAMD64"] != "v1" {
		return errors.New("unsupported GOAMD64 identity")
	}
	if r.GOARCH == "arm64" && r.BuildSettings["GOARM64"] != "v8.0" {
		return errors.New("unsupported GOARM64 identity")
	}
	h := r.Hardware
	for field, value := range map[string]string{"cpu_model": h.CPUModel, "cpu_affinity": h.CPUAffinity, "mem_total": h.MemTotal, "cgroup_memory_max": h.CgroupMemoryMax, "cgroup_cpu_max": h.CgroupCPUMax} {
		if strings.TrimSpace(value) == "" || len(value) > 4096 {
			return errors.New("missing/bounded hardware observation")
		}
		if value == "unavailable" {
			found := false
			for _, note := range h.ObservationNotes {
				if note == field+": unavailable" {
					found = true
				}
			}
			if !found {
				return errors.New("unexplained unavailable hardware observation")
			}
		}
	}
	if r.Errors == nil || len(r.Notes) == 0 || len(r.BuildSettings) > 128 || len(h.ObservationNotes) > 32 {
		return errors.New("missing/excessive report metadata")
	}
	return nil
}

// ValidatePass establishes schema3 material report self-consistency. External
// source/binary/host/file attestation and statistical performance admission are
// still separately required; checksums cannot prove observations occurred.
func (r Report) ValidatePass() error {
	if r.SchemaVersion != schemaVersion || r.HarnessRevision != harnessRevision || r.Status != "passed" || len(r.Errors) != 0 || r.UnverifiedRetirement || r.UnverifiedCloseHandles != 0 || r.CloseFailures != 0 || r.Shutdown != "normal_retirement_verified" {
		return errors.New("report did not establish schema3 normal successful retirement")
	}
	a, e := r.Config.admission()
	if e != nil {
		return e
	}
	if a != r.Admission {
		return errors.New("admission identity mismatch")
	}
	expected, e := canonicalAlgorithm(r.Config)
	if e != nil {
		return e
	}
	got, e := canonicalBytes(r.Algorithm)
	if e != nil || !bytes.Equal(expected, got) {
		return errors.New("canonical algorithm identity mismatch")
	}
	if r.QueryPolicy != queryPolicy(r.Config) || r.Limits != nativeLimits() || r.TimingSemantics != timingPolicy() {
		return errors.New("query/limits/timing material identity mismatch")
	}
	if r.Benchmark != "native-ann" || r.WorkloadVersion != workloadVersion || r.MeasurementVersion != measurementVersion || r.CacheCondition != cacheCondition || r.PageSize != 4096 || r.PendingObservation != nil {
		return errors.New("workload identity mismatch")
	}
	if e = validateProvenance(r); e != nil {
		return e
	}
	for _, v := range []string{r.WorkloadSHA256, r.BuildSHA256, r.DatasetSHA256, r.QuerySHA256, r.DatasetAndQueriesSHA256, r.HardwareSHA256} {
		if len(v) != 64 || !sourceIdentity(v) {
			return errors.New("invalid hash identity")
		}
	}
	_, _, dataset, queries, combined, e := generateFixture(r.Config, false)
	if e != nil {
		return e
	}
	if dataset != r.DatasetSHA256 || queries != r.QuerySHA256 || combined != r.DatasetAndQueriesSHA256 {
		return errors.New("seeded dataset/query digest mismatch")
	}
	hash, e := workloadDigest(r)
	if e != nil || hash != r.WorkloadSHA256 {
		return errors.New("workload digest mismatch")
	}
	hash, e = hardwareDigest(r)
	if e != nil || hash != r.HardwareSHA256 {
		return errors.New("hardware digest mismatch")
	}
	hash, e = buildDigest(r)
	if e != nil || hash != r.BuildSHA256 {
		return errors.New("source/build digest mismatch")
	}
	started, e := time.ParseInLocation(time.RFC3339Nano, r.StartedUTC, time.UTC)
	if e != nil {
		return e
	}
	finished, e := time.ParseInLocation(time.RFC3339Nano, r.FinishedUTC, time.UTC)
	if e != nil {
		return e
	}
	if !strings.HasSuffix(r.StartedUTC, "Z") || !strings.HasSuffix(r.FinishedUTC, "Z") || !finished.After(started) || !finite(r.ElapsedSeconds) || r.ElapsedSeconds != finished.Sub(started).Seconds() {
		return errors.New("UTC/elapsed interval mismatch")
	}
	if !retainedPathsValid(r.GOOS, r.DataDirectory, r.ReportPath) {
		return errors.New("retained evidence path identity mismatch")
	}
	if r.DatabaseBytes <= 0 || r.BeforeIndexBytes <= 0 || r.DatabaseBytes%4096 != 0 || r.BeforeIndexBytes%4096 != 0 || r.BeforeIndexBytes > r.DatabaseBytes || r.IndexBytesDelta != r.DatabaseBytes-r.BeforeIndexBytes || uint64(r.DatabaseBytes) > r.MaxObservedDataBytes || r.MaxObservedDataBytes > r.Config.MaxDataBytes || r.MaxObservedHeapBytes == 0 || r.MaxObservedHeapBytes > r.Config.MaxMemoryBytes {
		return errors.New("space/observed-budget evidence mismatch")
	}
	if !finite(r.InsertMS) || !finite(r.BuildMS) || r.InsertMS < 0 || r.BuildMS < 0 {
		return errors.New("invalid build latency")
	}
	if !r.ReopenVerified || !r.FinalModelVerified || r.ReopenedDatasetSHA256 != r.DatasetSHA256 || len(r.Samples) != 3 {
		return errors.New("missing persisted model or sample evidence")
	}
	if len(r.Ownership) != 3 {
		return errors.New("unexpected owner kind")
	}
	for kind, count := range map[string]int{"DB": 2, "Tx": 12 + 3*r.Config.Queries, "Rows": 1} {
		v, ok := r.Ownership[kind]
		if !ok || v.Acquired != count || v.Retired != count || v.Live != 0 {
			return fmt.Errorf("incomplete %s ownership", kind)
		}
	}
	measuredMS := r.InsertMS + r.BuildMS
	for i, s := range r.Samples {
		if s.K != r.QueryPolicy.K[i] || s.Queries != r.Config.Queries || len(s.Observations) != s.Queries || s.NativeANNQueries != s.Queries || s.FallbackQueries != 0 {
			return errors.New("incomplete native ANN sample coverage")
		}
		for _, o := range s.Observations {
			if e = validateObservation(o, s.K, r.Config); e != nil {
				return e
			}
			if !finite(o.ExactMS) || !finite(o.ANNMS) || o.ExactMS < 0 || o.ANNMS < 0 {
				return errors.New("invalid raw latency")
			}
			measuredMS += o.ExactMS + o.ANNMS
		}
		fresh := Sample{K: s.K, Observations: s.Observations}
		if e = summarizeSample(&fresh); e != nil {
			return e
		}
		left, e := json.Marshal(s)
		if e != nil {
			return e
		}
		right, e := json.Marshal(fresh)
		if e != nil {
			return e
		}
		if !bytes.Equal(left, right) {
			return errors.New("sample summary/raw conservation mismatch")
		}
	}
	// Distinct measured calls cannot consume more wall-clock than the entire
	// single-threaded run. Tiny clock representation rounding gets 1 microsecond.
	if !finite(measuredMS) || measuredMS > r.ElapsedSeconds*1000+.001 {
		return errors.New("measured time exceeds elapsed interval")
	}
	return nil
}
func finite(v float64) bool { return !math.IsNaN(v) && !math.IsInf(v, 0) }

// Validate recorded path syntax using the report platform, without accessing
// any path and without depending on the consumer's host path conventions.
func retainedPathsValid(goos, dir, report string) bool {
	if strings.ContainsAny(dir+report, "\x00\r\n") || dir == "" || report == "" {
		return false
	}
	if goos == "windows" {
		dir = strings.ReplaceAll(dir, "\\", "/")
		report = strings.ReplaceAll(report, "\\", "/")
		if len(dir) >= 3 && ((dir[0] >= 'A' && dir[0] <= 'Z') || (dir[0] >= 'a' && dir[0] <= 'z')) && dir[1:3] == ":/" {
			return path.Clean(dir) == dir && report == path.Join(dir, "report.json")
		}
		if strings.HasPrefix(dir, "//") {
			rest := strings.TrimPrefix(dir, "//")
			parts := strings.Split(rest, "/")
			if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
				return false
			}
			for _, p := range parts {
				if p == "" || p == "." || p == ".." {
					return false
				}
			}
			return report == dir+"/report.json"
		}
		return false
	}
	return path.IsAbs(dir) && path.Clean(dir) == dir && report == path.Join(dir, "report.json")
}
