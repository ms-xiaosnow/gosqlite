package reportcheck

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"math"
	"sort"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

type Observation struct {
	Completed  bool                `json:"completed"`
	Stage      string              `json:"stage"`
	Query      int                 `json:"query_ordinal"`
	ExactFirst bool                `json:"exact_first"`
	ExactMS    float64             `json:"exact_ms"`
	ANNMS      float64             `json:"ann_ms"`
	Hits       int                 `json:"intersection_count"`
	Exact      sqlite.VectorResult `json:"exact"`
	ANN        sqlite.VectorResult `json:"ann"`
}
type Sample struct {
	K                            int           `json:"k"`
	Queries                      int           `json:"queries"`
	Recall                       float64       `json:"recall"`
	ExactP50MS                   float64       `json:"exact_p50_ms"`
	ExactP95MS                   float64       `json:"exact_p95_ms"`
	ANNP50MS                     float64       `json:"ann_p50_ms"`
	ANNP95MS                     float64       `json:"ann_p95_ms"`
	MeanExactBaseRows            float64       `json:"mean_exact_base_rows"`
	MeanANNBaseRows              float64       `json:"mean_ann_base_rows"`
	MeanANNNodeReads             float64       `json:"mean_ann_node_reads"`
	MeanANNDistanceEvaluations   float64       `json:"mean_ann_distance_evaluations"`
	MeanCentroidScores           float64       `json:"mean_centroid_scores"`
	MeanListsOpened              float64       `json:"mean_lists_opened"`
	MeanMemberKeysRead           float64       `json:"mean_member_keys_read"`
	MeanTotalDistanceEvaluations float64       `json:"mean_total_distance_evaluations"`
	MinEffectiveProbes           int           `json:"min_effective_probes"`
	MaxEffectiveProbes           int           `json:"max_effective_probes"`
	FallbackQueries              int           `json:"fallback_queries"`
	NativeANNQueries             int           `json:"native_ann_queries"`
	Observations                 []Observation `json:"observations"`
}
type Hardware struct {
	CPUModel         string   `json:"cpu_model"`
	CPUAffinity      string   `json:"cpu_affinity"`
	MemTotal         string   `json:"mem_total"`
	CgroupMemoryMax  string   `json:"cgroup_memory_max"`
	CgroupCPUMax     string   `json:"cgroup_cpu_max"`
	ObservationNotes []string `json:"observation_notes"`
}
type Report struct {
	PendingObservation      *Observation         `json:"incomplete_observation,omitempty"`
	SchemaVersion           int                  `json:"schema_version"`
	HarnessRevision         string               `json:"harness_revision"`
	Benchmark               string               `json:"benchmark"`
	WorkloadVersion         string               `json:"workload_version"`
	MeasurementVersion      string               `json:"measurement_version"`
	WorkloadSHA256          string               `json:"workload_sha256"`
	DatasetSHA256           string               `json:"dataset_sha256"`
	QuerySHA256             string               `json:"query_sha256"`
	DatasetAndQueriesSHA256 string               `json:"dataset_and_queries_gsv_sha256"`
	QueryPolicy             QueryPolicy          `json:"query_policy"`
	BuildSHA256             string               `json:"build_sha256"`
	Config                  Config               `json:"config"`
	Algorithm               json.RawMessage      `json:"algorithm"`
	Admission               Admission            `json:"admission"`
	Status                  string               `json:"status"`
	Errors                  []string             `json:"errors"`
	StartedUTC              string               `json:"started_utc"`
	FinishedUTC             string               `json:"finished_utc"`
	ElapsedSeconds          float64              `json:"elapsed_seconds"`
	GoVersion               string               `json:"go_version"`
	GOOS                    string               `json:"goos"`
	GOARCH                  string               `json:"goarch"`
	CPUs                    int                  `json:"logical_cpus"`
	GOMAXPROCS              int                  `json:"gomaxprocs"`
	BuildSettings           map[string]string    `json:"build_settings"`
	Hardware                Hardware             `json:"hardware"`
	HardwareSHA256          string               `json:"hardware_sha256"`
	PageSize                uint32               `json:"page_size"`
	CacheCondition          string               `json:"cache_condition"`
	Limits                  LimitsWire           `json:"limits"`
	TimingSemantics         TimingWire           `json:"timing_semantics"`
	DataDirectory           string               `json:"retained_data_directory"`
	ReportPath              string               `json:"retained_report_path"`
	InsertMS                float64              `json:"insert_ms"`
	BuildMS                 float64              `json:"index_build_ms"`
	DatabaseBytes           int64                `json:"database_bytes"`
	BeforeIndexBytes        int64                `json:"before_index_bytes"`
	IndexBytesDelta         int64                `json:"index_bytes_delta"`
	MaxObservedDataBytes    uint64               `json:"max_observed_data_bytes"`
	MaxObservedHeapBytes    uint64               `json:"max_observed_heap_bytes"`
	ReopenVerified          bool                 `json:"reopen_verified"`
	ReopenedDatasetSHA256   string               `json:"reopened_dataset_sha256"`
	FinalModelVerified      bool                 `json:"final_model_verified"`
	Samples                 []Sample             `json:"samples"`
	Ownership               map[string]Ownership `json:"ownership"`
	CloseFailures           int                  `json:"close_failures"`
	UnverifiedCloseHandles  int                  `json:"unverified_close_handles"`
	UnverifiedRetirement    bool                 `json:"unverified_retirement"`
	Shutdown                string               `json:"shutdown"`
	Notes                   []string             `json:"notes"`
}

func digest(v any) (string, error) {
	b, e := json.Marshal(v)
	if e != nil {
		return "", e
	}
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:]), nil
}
func hardwareDigest(r Report) (string, error) {
	return digest(struct {
		ID          string
		Hardware    Hardware
		OS, Arch    string
		CPUs, Procs int
	}{r.Config.HardwareID, r.Hardware, r.GOOS, r.GOARCH, r.CPUs, r.GOMAXPROCS})
}
func workloadDigest(r Report) (string, error) {
	canonical, e := canonicalBytes(r.Algorithm)
	if e != nil {
		return "", e
	}
	c := r.Config
	return digest(struct {
		Schema                                                                         int
		Harness, DatasetVersion, MeasurementVersion, Dataset, Queries, Combined, Cache string
		Rows, Dimensions, QueryCount                                                   int
		Seed                                                                           int64
		Algorithm                                                                      json.RawMessage
		QueryPolicy                                                                    QueryPolicy
		PageSize                                                                       uint32
		Limits                                                                         LimitsWire
		Timing                                                                         TimingWire
		Budgets                                                                        BudgetWire
	}{r.SchemaVersion, r.HarnessRevision, r.WorkloadVersion, r.MeasurementVersion, r.DatasetSHA256, r.QuerySHA256, r.DatasetAndQueriesSHA256, r.CacheCondition, c.Rows, c.Dimensions, c.Queries, c.Seed, canonical, r.QueryPolicy, r.PageSize, r.Limits, r.TimingSemantics, budgets(c)})
}
func buildDigest(r Report) (string, error) {
	return digest(struct {
		Source, Go, OS, Arch string
		Settings             map[string]string
	}{r.Config.SourceID, r.GoVersion, r.GOOS, r.GOARCH, r.BuildSettings})
}
func percentile(v []float64, p float64) (float64, error) {
	if len(v) == 0 || math.IsNaN(p) || p < 0 || p > 1 {
		return 0, errors.New("invalid percentile input")
	}
	w := append([]float64(nil), v...)
	for _, x := range w {
		if math.IsNaN(x) || math.IsInf(x, 0) || x < 0 {
			return 0, errors.New("invalid latency")
		}
	}
	sort.Float64s(w)
	return w[int(float64(len(w)-1)*p)], nil
}
func intersection(exact, approx sqlite.VectorResult, k, n int) (int, error) {
	if !exact.Plan.Exact || len(exact.Hits) != k || len(approx.Hits) != k {
		return 0, errors.New("top-k cardinality/exact-plan invariant failed")
	}
	sets := []map[int64]bool{make(map[int64]bool, k), make(map[int64]bool, k)}
	for j, result := range []sqlite.VectorResult{exact, approx} {
		for i, h := range result.Hits {
			if h.RowID < 1 || h.RowID > int64(n) || sets[j][h.RowID] || math.IsNaN(h.Distance) || math.IsInf(h.Distance, 0) || h.Distance < 0 {
				return 0, errors.New("invalid or duplicate result hit")
			}
			if i > 0 {
				p := result.Hits[i-1]
				if p.Distance > h.Distance || (p.Distance == h.Distance && p.RowID >= h.RowID) {
					return 0, errors.New("result ordering invariant failed")
				}
			}
			sets[j][h.RowID] = true
		}
	}
	count := 0
	for id := range sets[1] {
		if sets[0][id] {
			count++
		}
	}
	return count, nil
}
func summarizeSample(s *Sample) error {
	s.Queries = len(s.Observations)
	if s.Queries == 0 {
		return errors.New("no observations")
	}
	var ex, ap []float64
	hits := 0
	for i, o := range s.Observations {
		if s.K < 1 || !o.Completed || o.Hits < 0 || o.Hits > s.K || o.Query != i || o.ExactFirst != (i%2 == 0) {
			return errors.New("invalid observation identity or hit count")
		}
		ex = append(ex, o.ExactMS)
		ap = append(ap, o.ANNMS)
		hits += o.Hits
		s.MeanExactBaseRows += float64(o.Exact.Stats.BaseRowFetches)
		s.MeanANNBaseRows += float64(o.ANN.Stats.BaseRowFetches)
		s.MeanANNNodeReads += float64(o.ANN.Stats.ANNNodeReads)
		s.MeanANNDistanceEvaluations += float64(o.ANN.Stats.ANNDistanceEvaluations)
		s.MeanCentroidScores += float64(o.ANN.Stats.ANNCentroidScores)
		s.MeanListsOpened += float64(o.ANN.Stats.ANNListsOpened)
		s.MeanMemberKeysRead += float64(o.ANN.Stats.ANNMemberKeysRead)
		if !validStats(o.Exact.Stats) || !validStats(o.ANN.Stats) {
			return errors.New("negative raw counters")
		}
		total, e := checkedSum(uint64(o.ANN.Stats.ANNDistanceEvaluations), uint64(o.ANN.Stats.ANNCentroidScores), uint64(o.ANN.Stats.ScoredRows))
		if e != nil || total > math.MaxInt64 {
			return errors.New("distance counter sum overflow")
		}
		s.MeanTotalDistanceEvaluations += float64(total)
		if o.ANN.Plan.Exact {
			s.FallbackQueries++
		} else {
			s.NativeANNQueries++
		}
		if i == 0 || o.ANN.Plan.ANNEffectiveProbes < s.MinEffectiveProbes {
			s.MinEffectiveProbes = o.ANN.Plan.ANNEffectiveProbes
		}
		s.MaxEffectiveProbes = max(s.MaxEffectiveProbes, o.ANN.Plan.ANNEffectiveProbes)
	}
	var e error
	if s.ExactP50MS, e = percentile(ex, .5); e != nil {
		return e
	}
	if s.ExactP95MS, e = percentile(ex, .95); e != nil {
		return e
	}
	if s.ANNP50MS, e = percentile(ap, .5); e != nil {
		return e
	}
	if s.ANNP95MS, e = percentile(ap, .95); e != nil {
		return e
	}
	q := float64(s.Queries)
	s.Recall = float64(hits) / (float64(s.K) * q)
	for _, v := range []*float64{&s.MeanExactBaseRows, &s.MeanANNBaseRows, &s.MeanANNNodeReads, &s.MeanANNDistanceEvaluations, &s.MeanCentroidScores, &s.MeanListsOpened, &s.MeanMemberKeysRead, &s.MeanTotalDistanceEvaluations} {
		*v /= q
	}
	return nil
}

// Ownership contains reported counts only; this package never owns database handles.
type Ownership struct {
	Acquired int `json:"acquired"`
	Retired  int `json:"retired"`
	Live     int `json:"unverified"`
}
