// Command ann measures an ordinary native persistent HNSW or IVFFlat path against exact
// scan on the same fixed, synthetic dataset. It never invokes an SQL oracle or
// a fault/recovery workflow. Timings are observations, not acceptance promises.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

type sample struct {
	MeanCentroidScores           float64 `json:"mean_centroid_scores"`
	MeanListsOpened              float64 `json:"mean_lists_opened"`
	MeanMemberKeysRead           float64 `json:"mean_member_keys_read"`
	MeanTotalDistanceEvaluations float64 `json:"mean_total_distance_evaluations"`
	MinEffectiveProbes           int     `json:"min_effective_probes"`
	MaxEffectiveProbes           int     `json:"max_effective_probes"`
	K                            int     `json:"k"`
	Queries                      int     `json:"queries"`
	Recall                       float64 `json:"recall"`
	ExactP50MS                   float64 `json:"exact_p50_ms"`
	ExactP95MS                   float64 `json:"exact_p95_ms"`
	ANNP50MS                     float64 `json:"ann_p50_ms"`
	ANNP95MS                     float64 `json:"ann_p95_ms"`
	MeanExactBaseRows            float64 `json:"mean_exact_base_rows"`
	MeanANNBaseRows              float64 `json:"mean_ann_base_rows"`
	MeanANNNodeReads             float64 `json:"mean_ann_node_reads"`
	MeanANNDistanceEvaluations   float64 `json:"mean_ann_distance_evaluations"`
	FallbackQueries              int     `json:"fallback_queries"`
}
type report struct {
	Lists            int      `json:"lists,omitempty"`
	Probes           int      `json:"probes,omitempty"`
	MaxProbes        int      `json:"max_probes,omitempty"`
	StartedUTC       string   `json:"started_utc"`
	GoVersion        string   `json:"go_version"`
	GOOS             string   `json:"goos"`
	GOARCH           string   `json:"goarch"`
	CPUs             int      `json:"logical_cpus"`
	GOMAXPROCS       int      `json:"gomaxprocs"`
	Rows             int      `json:"rows"`
	Dimensions       int      `json:"dimensions"`
	Queries          int      `json:"queries"`
	Seed             int64    `json:"dataset_seed"`
	DatasetSHA256    string   `json:"dataset_and_queries_gsv_sha256"`
	PageSize         uint32   `json:"page_size"`
	Cache            string   `json:"cache_condition"`
	Method           string   `json:"method"`
	Metric           string   `json:"metric"`
	M                int      `json:"m"`
	EFConstruction   int      `json:"ef_construction"`
	EFSearch         int      `json:"ef_search"`
	MaxEFSearch      int      `json:"max_ef_search"`
	GraphSeed        uint64   `json:"graph_seed"`
	InsertMS         float64  `json:"insert_ms"`
	BuildMS          float64  `json:"index_build_ms"`
	DatabaseBytes    int64    `json:"database_bytes"`
	BeforeIndexBytes int64    `json:"before_index_bytes"`
	ReopenVerified   bool     `json:"reopen_verified"`
	Samples          []sample `json:"samples"`
	Notes            []string `json:"notes"`
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
func elapsed(t time.Time) float64 { return float64(time.Since(t)) / float64(time.Millisecond) }
func percentile(v []float64, p float64) float64 {
	sort.Float64s(v)
	i := int(float64(len(v)-1) * p)
	return v[i]
}
func fileSize(path string) int64 { s, e := os.Stat(path); must(e); return s.Size() }
func main() {
	n := flag.Int("rows", 2048, "synthetic source rows (100..100000)")
	dims := flag.Int("dims", 12, "finite dense dimensions (1..2000)")
	nq := flag.Int("queries", 48, "held-out queries (1..10000)")
	seed := flag.Int64("seed", 20261010, "math/rand source seed")
	method := flag.String("method", "hnsw", "hnsw or ivfflat")
	lists := flag.Int("lists", 100, "IVFFlat centroid/list count")
	probes := flag.Int("probes", 1, "IVFFlat initial probes")
	maxProbes := flag.Int("max-probes", 0, "IVFFlat maximum probes; zero means lists")
	flag.Parse()
	if *method != "hnsw" && *method != "ivfflat" {
		panic("unknown method")
	}
	if *n < 100 || *n > 100000 || *dims < 1 || *dims > 2000 || *nq < 1 || *nq > 10000 {
		panic("bounded flags invalid")
	}
	ctx := context.Background()
	rep := report{StartedUTC: time.Now().UTC().Format(time.RFC3339), GoVersion: runtime.Version(), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, CPUs: runtime.NumCPU(), GOMAXPROCS: runtime.GOMAXPROCS(0), Rows: *n, Dimensions: *dims, Queries: *nq, Seed: *seed, PageSize: 4096, Cache: "warm OS cache; unmeasured warmup; exact/ANN timing order alternates per query; no cache flush", Method: "hnsw", Metric: "l2", M: 16, EFConstruction: 64, EFSearch: 40, MaxEFSearch: 1000, GraphSeed: 0x243f6a8885a308d3, Notes: []string{"Synthetic independent standard-normal float32 rows and held-out queries; rowid ascending breaks ties.", "Go native persistent B-tree graph is reopened before measurement; no all-graph memory preload.", "Single process; ordinary DELETE-journal writes; no concurrent workload or physical fault/recovery test.", "These shared-cloud wall-clock samples are descriptive, not confidence intervals or speedup claims.", "Each query uses one read-only transaction for exact and ANN on the same snapshot; ANN is exact-rescored but recall remains approximate.", "Default ef_search=40; widening may increase it to1000 to obtain k candidates. Fallbacks are separately counted."}}
	indexName := "items_hnsw"
	if *method == "ivfflat" {
		indexName = "items_ivf"
		rep.Method = "ivfflat"
		rep.Notes[1] = "Go native persistent centroid/list B-trees are reopened before measurement; selected memberships are read lazily."
		rep.M = 0
		rep.EFConstruction = 0
		rep.EFSearch = 0
		rep.MaxEFSearch = 0
		rep.Lists = *lists
		rep.Probes = *probes
		rep.MaxProbes = *maxProbes
		if rep.MaxProbes == 0 {
			rep.MaxProbes = *lists
		}
		rep.Notes[len(rep.Notes)-1] = "IVFFlat uses 20 training iterations and 10000 sample cap; probes widen only to compensate insufficient eligible results. All-list request falls back to exact."
	}
	rng := rand.New(rand.NewSource(*seed))
	hash := sha256.New()
	makeVector := func() vector.Vector {
		values := make([]float32, *dims)
		for j := range values {
			values[j] = float32(rng.NormFloat64())
		}
		v, e := vector.New(values)
		must(e)
		b, e := v.MarshalBinary()
		must(e)
		_, e = hash.Write(b)
		must(e)
		return v
	}
	data := make([]vector.Vector, *n)
	for i := range data {
		data[i] = makeVector()
	}
	queries := make([]vector.Vector, *nq)
	for i := range queries {
		queries[i] = makeVector()
	}
	rep.DatasetSHA256 = hex.EncodeToString(hash.Sum(nil))
	dir, e := os.MkdirTemp("", "gosqlite-ann-benchmark-")
	must(e)
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "benchmark.db")
	db, e := sqlite.Open(path, sqlite.Options{PageSize: 4096})
	must(e)
	_, e = db.Exec(ctx, fmt.Sprintf("CREATE TABLE items(id INTEGER PRIMARY KEY,embedding VECTOR(%d) NOT NULL)", *dims))
	must(e)
	start := time.Now()
	tx, e := db.Begin(ctx)
	must(e)
	for i, v := range data {
		_, e = tx.Exec(ctx, "INSERT INTO items VALUES(?,?)", int64(i+1), v)
		must(e)
	}
	must(tx.Commit(ctx))
	rep.InsertMS = elapsed(start)
	rep.BeforeIndexBytes = fileSize(path)
	start = time.Now()
	if *method == "hnsw" {
		must(db.CreateHNSWIndex(ctx, indexName, "items", "embedding"))
	} else {
		must(db.CreateIVFFlatIndex(ctx, indexName, "items", "embedding", sqlite.IVFFlatIndexOptions{Lists: *lists}))
	}
	rep.BuildMS = elapsed(start)
	rep.DatabaseBytes = fileSize(path)
	must(db.CheckExtensionIndex(ctx, indexName))
	must(db.Close())
	db, e = sqlite.Open(path, sqlite.Options{})
	must(e)
	defer db.Close()
	must(db.CheckExtensionIndex(ctx, indexName))
	rep.ReopenVerified = true
	for _, k := range []int{1, 10, 100} {
		// Native defaults with explicit index select only the actual graph path.
		exactOptions := sqlite.VectorSearchOptions{Limit: k}
		annOptions := sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Index: indexName, Limit: k}
		if *method == "ivfflat" {
			annOptions.Probes = *probes
			annOptions.MaxProbes = *maxProbes
		}
		_, e = db.VectorSearch(ctx, "items", "embedding", queries[0], exactOptions)
		must(e)
		_, e = db.VectorSearch(ctx, "items", "embedding", queries[0], annOptions)
		must(e)
		s := sample{K: k, Queries: *nq}
		exactTimes := make([]float64, 0, *nq)
		annTimes := make([]float64, 0, *nq)
		hits := 0
		for i, q := range queries {
			tx, e = db.Begin(ctx, sqlite.TxOptions{ReadOnly: true})
			must(e)
			var exact, approx sqlite.VectorResult
			runExact := func() {
				start = time.Now()
				exact, e = tx.VectorSearch(ctx, "items", "embedding", q, exactOptions)
				must(e)
				exactTimes = append(exactTimes, elapsed(start))
			}
			runANN := func() {
				start = time.Now()
				approx, e = tx.VectorSearch(ctx, "items", "embedding", q, annOptions)
				must(e)
				annTimes = append(annTimes, elapsed(start))
			}
			if i%2 == 0 {
				runExact()
				runANN()
			} else {
				runANN()
				runExact()
			}
			must(tx.Rollback())
			if !exact.Plan.Exact || len(exact.Hits) != k || len(approx.Hits) != k {
				panic("benchmark result invariant")
			}
			if approx.Plan.Exact {
				s.FallbackQueries++
			} else if approx.Plan.Access != "SEARCH" || approx.Plan.Method != *method || (*method == "hnsw" && approx.Stats.ANNNodeReads == 0) || (*method == "ivfflat" && approx.Stats.ANNCentroidScores == 0) {
				panic("not an actual ANN path")
			}
			truth := make(map[int64]bool, k)
			for _, h := range exact.Hits {
				truth[h.RowID] = true
			}
			for _, h := range approx.Hits {
				if truth[h.RowID] {
					hits++
				}
			}
			s.MeanExactBaseRows += float64(exact.Stats.BaseRowFetches)
			s.MeanANNBaseRows += float64(approx.Stats.BaseRowFetches)
			s.MeanANNNodeReads += float64(approx.Stats.ANNNodeReads)
			s.MeanANNDistanceEvaluations += float64(approx.Stats.ANNDistanceEvaluations)
			s.MeanCentroidScores += float64(approx.Stats.ANNCentroidScores)
			s.MeanListsOpened += float64(approx.Stats.ANNListsOpened)
			s.MeanMemberKeysRead += float64(approx.Stats.ANNMemberKeysRead)
			s.MeanTotalDistanceEvaluations += float64(approx.Stats.ANNDistanceEvaluations + approx.Stats.ANNCentroidScores + approx.Stats.ScoredRows)
			if i == 0 || approx.Plan.ANNEffectiveProbes < s.MinEffectiveProbes {
				s.MinEffectiveProbes = approx.Plan.ANNEffectiveProbes
			}
			if approx.Plan.ANNEffectiveProbes > s.MaxEffectiveProbes {
				s.MaxEffectiveProbes = approx.Plan.ANNEffectiveProbes
			}
		}
		s.Recall = float64(hits) / float64(k**nq)
		s.ExactP50MS = percentile(exactTimes, .5)
		s.ExactP95MS = percentile(exactTimes, .95)
		s.ANNP50MS = percentile(annTimes, .5)
		s.ANNP95MS = percentile(annTimes, .95)
		s.MeanExactBaseRows /= float64(*nq)
		s.MeanANNBaseRows /= float64(*nq)
		s.MeanANNNodeReads /= float64(*nq)
		s.MeanANNDistanceEvaluations /= float64(*nq)
		s.MeanCentroidScores /= float64(*nq)
		s.MeanListsOpened /= float64(*nq)
		s.MeanMemberKeysRead /= float64(*nq)
		s.MeanTotalDistanceEvaluations /= float64(*nq)
		rep.Samples = append(rep.Samples, s)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	must(enc.Encode(rep))
}
