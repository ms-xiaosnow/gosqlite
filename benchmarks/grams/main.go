// Command grams measures the native persisted grams13 engine against genuine
// NOT INDEXED scans. It does not invoke another database, flush OS caches, or
// exercise faults, crashes, recovery, or interrupted publication.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
	"golang.org/x/text/unicode/norm"
)

const corpusVersion = "grams-nfc-selectivity-v1"

type config struct {
	Rows        int    `json:"rows"`
	Queries     int    `json:"queries_per_gram_length"`
	Repeats     int    `json:"independent_database_builds"`
	Seed        int64  `json:"dataset_seed"`
	PageSize    uint32 `json:"page_size"`
	MaxPages    uint32 `json:"max_transaction_pages"`
	SourceID    string `json:"source_id"`
	Environment string `json:"hardware_id"`
	CorpusOnly  bool   `json:"-"`
}

func (c config) validate() error {
	if c.Rows < 64 || c.Rows > 20000 || c.Queries < 1 || c.Queries > 128 || c.Repeats < 2 || c.Repeats > 10 || c.MaxPages < 4096 || c.MaxPages > 65536 {
		return fmt.Errorf("bounded options: rows 64..20000, queries 1..128 per length, repeats 2..10, max-pages 4096..65536")
	}
	if c.PageSize != 512 && c.PageSize != 4096 && c.PageSize != 65536 {
		return fmt.Errorf("page-size must be 512, 4096 or 65536")
	}
	if !c.CorpusOnly && (strings.TrimSpace(c.SourceID) == "" || strings.TrimSpace(c.Environment) == "") {
		return fmt.Errorf("source-id and environment are required; describe the actual source and hardware/measurement conditions")
	}
	return nil
}

type query struct {
	Length     int    `json:"gram_length"`
	Pattern    string `json:"pattern"`
	WantSearch bool   `json:"require_real_search"`
}
type corpus struct {
	Version string   `json:"version"`
	Seed    int64    `json:"dataset_seed"`
	Texts   []string `json:"texts"`
	Queries []query  `json:"queries"`
}

func marker(n, group int) string {
	var s strings.Builder
	for j := 0; j < n; j++ {
		s.WriteRune(rune(0x7800 + n*512 + j*128 + group))
	}
	return s.String()
}
func makeCorpus(c config) (corpus, string, int64) {
	rng := rand.New(rand.NewSource(c.Seed))
	out := corpus{Version: corpusVersion, Seed: c.Seed, Texts: make([]string, c.Rows)}
	tokens := []string{"a", "B", "z", "数", "据", "库", "か", "な", "한", "글", "🙂", "e\u0301", "A\u030a", "か\u3099", "한", "_", "%", "\\", " "}
	var bytes int64
	for i := range out.Texts {
		var b strings.Builder
		b.WriteString("common ")
		for k := 8 + rng.Intn(17); k > 0; k-- {
			b.WriteString(tokens[rng.Intn(len(tokens))])
		}
		// Every planted marker has approximately 1/32 selectivity. These controlled
		// queries isolate 1/2/3-gram paths, not a claim about natural-language traffic.
		for n := 1; n <= 3; n++ {
			b.WriteString(" ")
			b.WriteString(marker(n, i%32))
		}
		out.Texts[i] = norm.NFC.String(b.String())
		bytes += int64(len(out.Texts[i]))
	}
	for n := 1; n <= 3; n++ {
		for i := 0; i < c.Queries; i++ {
			out.Queries = append(out.Queries, query{n, "%" + marker(n, rng.Intn(32)) + "%", true})
		}
	}
	out.Queries = append(out.Queries, query{0, "%", false}, query{0, "%common%", false})
	raw, _ := json.Marshal(out)
	sum := sha256.Sum256(raw)
	return out, hex.EncodeToString(sum[:]), bytes
}

type work struct {
	FrequencyLookups int64 `json:"frequency_lookups"`
	PostingKeysRead  int64 `json:"posting_keys_read"`
	Candidates       int64 `json:"candidates"`
	BaseRowFetches   int64 `json:"base_row_fetches"`
	ReturnedRows     int64 `json:"returned_rows"`
}

func workOf(s sqlite.QueryStats) work {
	return work{s.FrequencyLookups, s.PostingKeysRead, s.Candidates, s.BaseRowFetches, s.ReturnedRows}
}

type observation struct {
	Query         query            `json:"query"`
	AutomaticMS   float64          `json:"automatic_ms"`
	ScanMS        float64          `json:"real_scan_ms"`
	AutomaticPlan sqlite.QueryPlan `json:"automatic_plan"`
	ScanPlan      sqlite.QueryPlan `json:"scan_plan"`
	AutomaticWork work             `json:"automatic_work"`
	ScanWork      work             `json:"scan_work"`
	Equal         bool             `json:"full_ordered_results_equal"`
	ResultSHA256  string           `json:"result_sha256"`
}
type distribution struct {
	Mean             float64  `json:"mean_ms"`
	SampleStddev     float64  `json:"sample_stddev_ms"`
	ConfidenceMethod string   `json:"confidence_method"`
	CILow            *float64 `json:"ci_low_ms"`
	CIHigh           *float64 `json:"ci_high_ms"`
	N                int      `json:"sample_count"`
	P50              float64  `json:"p50_ms"`
	P95              float64  `json:"p95_ms"`
	Min              float64  `json:"min_ms"`
	Max              float64  `json:"max_ms"`
}

func summarize(v []float64) distribution {
	if len(v) == 0 {
		return distribution{}
	}
	v = append([]float64(nil), v...)
	sort.Float64s(v)
	p := func(p float64) float64 { return v[max(0, int(math.Ceil(p*float64(len(v))))-1)] }
	var mean, squared float64
	for _, x := range v {
		mean += x
	}
	mean /= float64(len(v))
	for _, x := range v {
		squared += (x - mean) * (x - mean)
	}
	sd := 0.0
	if len(v) > 1 {
		sd = math.Sqrt(squared / float64(len(v)-1))
	}
	return distribution{N: len(v), P50: p(.5), P95: p(.95), Min: v[0], Max: v[len(v)-1], Mean: mean, SampleStddev: sd, ConfidenceMethod: "descriptive only; correlated shared-host observations; no inferential CI"}
}

type sample struct {
	Cache                 string        `json:"cache_condition"`
	Length                int           `json:"gram_length_0_is_fallback_control"`
	Automatic             distribution  `json:"automatic_query_latency"`
	Scan                  distribution  `json:"real_scan_query_latency"`
	SearchCount           int           `json:"actual_grams_search_queries"`
	FallbackCount         int           `json:"actual_scan_fallback_queries"`
	MembershipComparisons int64         `json:"query_text_membership_comparisons"`
	Observations          []observation `json:"observations"`
}
type trial struct {
	RetainedTemporaryDirectory string   `json:"retained_temporary_directory_on_error,omitempty"`
	Number                     int      `json:"repeat"`
	InsertMS                   float64  `json:"source_insert_ms"`
	BuildMS                    float64  `json:"index_build_ms"`
	BeforeBytes                int64    `json:"database_bytes_before_index"`
	AfterBytes                 int64    `json:"database_bytes_with_index"`
	IndexBytes                 int64    `json:"incremental_database_bytes"`
	DBAmplification            float64  `json:"database_with_index_over_before"`
	IndexOverText              float64  `json:"incremental_bytes_over_utf8_text_bytes"`
	DatabaseSHA256             string   `json:"database_sha256_after_build_close"`
	IntegrityBeforeClose       bool     `json:"extension_consistency_before_close"`
	IntegrityAfterReopen       bool     `json:"extension_consistency_after_reopen"`
	Samples                    []sample `json:"samples"`
}
type report struct {
	SchemaVersion     int                  `json:"schema_version"`
	Benchmark         string               `json:"benchmark"`
	WorkloadVersion   string               `json:"workload_version"`
	Finished          string               `json:"finished_utc"`
	ElapsedSeconds    float64              `json:"elapsed_seconds"`
	SourceID          string               `json:"source_id"`
	HardwareID        string               `json:"hardware_id"`
	DatasetSeed       int64                `json:"dataset_seed"`
	CacheCondition    string               `json:"cache_condition"`
	PageSize          uint32               `json:"page_size"`
	Errors            []string             `json:"errors"`
	Format            string               `json:"format"`
	Status            string               `json:"status"`
	Error             string               `json:"error,omitempty"`
	Started           string               `json:"started_utc"`
	Config            config               `json:"config"`
	GoVersion         string               `json:"go_version"`
	GOOS              string               `json:"goos"`
	GOARCH            string               `json:"goarch"`
	CPUs              int                  `json:"cpus"`
	GOMAXPROCS        int                  `json:"gomaxprocs"`
	BuildSettings     []debug.BuildSetting `json:"go_build_settings"`
	Modules           []*debug.Module      `json:"modules"`
	DatasetSHA256     string               `json:"workload_sha256"`
	SourceTextBytes   int64                `json:"source_utf8_text_bytes"`
	Trials            []trial              `json:"trials"`
	BuildDistribution distribution         `json:"index_build_latency_across_repeats"`
	Notes             []string             `json:"notes"`
}

func elapsed(start time.Time) float64 { return float64(time.Since(start)) / float64(time.Millisecond) }
func size(path string) (int64, error) {
	f, e := os.Stat(path)
	if e != nil {
		return 0, e
	}
	return f.Size(), nil
}
func fileHash(path string) (string, error) {
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	h := sha256.New()
	_, e = io.Copy(h, f)
	e = errors.Join(e, f.Close())
	return hex.EncodeToString(h.Sum(nil)), e
}
func execute(db *sqlite.DB, q string, args ...any) error {
	_, e := db.Exec(context.Background(), q, args...)
	return e
}

type queryResult struct {
	ids   []int64
	plan  sqlite.QueryPlan
	stats sqlite.QueryStats
	ms    float64
}

func runQuery(db *sqlite.DB, q query, scan bool) (r queryResult, err error) {
	sql := "SELECT id FROM docs"
	if scan {
		sql += " NOT INDEXED"
	}
	sql += " WHERE title LIKE ? ORDER BY id"
	start := time.Now()
	rows, e := db.Query(context.Background(), sql, q.Pattern)
	if e != nil {
		return r, e
	}
	for rows.Next() {
		v := rows.Values()
		id, ok := v[0].(int64)
		if !ok {
			err = fmt.Errorf("noninteger rowid %T", v[0])
			break
		}
		r.ids = append(r.ids, id)
	}
	r.plan = rows.Plan()
	r.stats = rows.Stats()
	err = errors.Join(err, rows.Err(), rows.Close())
	r.ms = elapsed(start)
	return r, err
}
func readFresh(path string, q query, scan bool) (r queryResult, err error) {
	db, e := sqlite.Open(path, sqlite.Options{ReadOnly: true, NoCreate: true})
	if e != nil {
		return r, e
	}
	r, err = runQuery(db, q, scan)
	err = errors.Join(err, db.Close())
	return r, err
}
func observe(db *sqlite.DB, path, cache string, q query, index int, rowCount int) (observation, error) {
	var a, b queryResult
	var err error
	run := func(scan bool) (queryResult, error) {
		if cache == "cold_connection" {
			return readFresh(path, q, scan)
		}
		return runQuery(db, q, scan)
	}
	if index%2 == 0 {
		a, err = run(false)
		if err == nil {
			b, err = run(true)
		}
	} else {
		b, err = run(true)
		if err == nil {
			a, err = run(false)
		}
	}
	o := observation{Query: q, AutomaticMS: a.ms, ScanMS: b.ms, AutomaticPlan: a.plan, ScanPlan: b.plan, AutomaticWork: workOf(a.stats), ScanWork: workOf(b.stats), Equal: reflect.DeepEqual(a.ids, b.ids)}
	raw, _ := json.Marshal(a.ids)
	sum := sha256.Sum256(raw)
	o.ResultSHA256 = hex.EncodeToString(sum[:])
	if err != nil {
		return o, err
	}
	if !o.Equal {
		return o, fmt.Errorf("full-result mismatch for %q", q.Pattern)
	}
	if b.plan.Access != "SCAN" || b.plan.Reason != "NOT INDEXED requested" || b.stats.BaseRowFetches != int64(rowCount) || b.stats.PostingKeysRead != 0 {
		return o, fmt.Errorf("reference is not a real full scan: %+v %+v", b.plan, b.stats)
	}
	if q.WantSearch {
		if a.plan.Access != "SEARCH" || a.plan.Method != "grams13" || a.plan.Namespace != "LIKE_RAW_V1" || !a.plan.Recheck || !reflect.DeepEqual(a.plan.GramLengths, []int{q.Length}) || a.stats.PostingKeysRead <= 0 || a.stats.Candidates <= 0 || a.stats.BaseRowFetches != a.stats.Candidates || a.stats.BaseRowFetches >= b.stats.BaseRowFetches || len(a.ids) == 0 {
			return o, fmt.Errorf("required actual %d-gram SEARCH not established: %+v %+v", q.Length, a.plan, a.stats)
		}
	} else if a.plan.Access != "SCAN" || a.plan.Reason == "" || a.stats.BaseRowFetches != b.stats.BaseRowFetches || (q.Pattern == "%common%" && !strings.Contains(a.plan.Reason, "frequency")) {
		return o, fmt.Errorf("fallback control is not an explained scan: %+v %+v", a.plan, a.stats)
	}
	return o, nil
}
func measure(path, cache string, data corpus, repeat int) (out []sample, err error) {
	var db *sqlite.DB
	if cache == "warm_process" {
		db, err = sqlite.Open(path, sqlite.Options{ReadOnly: true, NoCreate: true})
		if err != nil {
			return nil, err
		}
		defer func() { err = errors.Join(err, db.Close()) }()
		// Full untimed workload on the same connection; still no OS-cache claim.
		for _, q := range data.Queries {
			for _, scan := range []bool{false, true} {
				if _, err = runQuery(db, q, scan); err != nil {
					return nil, err
				}
			}
		}
	}
	for _, n := range []int{1, 2, 3, 0} {
		s := sample{Cache: cache, Length: n}
		var a, b []float64
		for i, q := range data.Queries {
			if q.Length != n {
				continue
			}
			o, e := observe(db, path, cache, q, i+repeat, len(data.Texts))
			s.Observations = append(s.Observations, o)
			if e != nil {
				return append(out, s), e
			}
			a = append(a, o.AutomaticMS)
			b = append(b, o.ScanMS)
			if o.AutomaticPlan.Access == "SEARCH" {
				s.SearchCount++
			} else {
				s.FallbackCount++
			}
			s.MembershipComparisons += int64(len(data.Texts))
		}
		s.Automatic = summarize(a)
		s.Scan = summarize(b)
		out = append(out, s)
	}
	return out, nil
}
func runTrial(c config, data corpus, textBytes int64, number int) (t trial, err error) {
	t.Number = number
	dir, e := os.MkdirTemp("", "gosqlite-grams-benchmark-")
	if e != nil {
		return t, e
	}
	// The directory contains only this harness's normally closed temporary DB.
	// No removal or mutation of a live/selected database namespace is performed.
	defer func() {
		// Preserve a failed run's files. In particular, never retire the namespace
		// when any native operation or resource close could still be incomplete.
		if err != nil {
			t.RetainedTemporaryDirectory = dir
			return
		}
		if e := os.RemoveAll(dir); e != nil {
			err = e
			t.RetainedTemporaryDirectory = dir
		}
	}()
	path := filepath.Join(dir, "benchmark.db")
	db, e := sqlite.Open(path, sqlite.Options{PageSize: c.PageSize, MaxTransactionPages: c.MaxPages})
	if e != nil {
		return t, e
	}
	defer func() {
		if db != nil {
			err = errors.Join(err, db.Close())
		}
	}()
	if info := db.Info(); info.PageSize != c.PageSize {
		return t, fmt.Errorf("actual page size %d differs from requested %d", info.PageSize, c.PageSize)
	}
	if e = execute(db, "CREATE TABLE docs(id INTEGER PRIMARY KEY,title TEXT NOT NULL)"); e != nil {
		return t, e
	}
	start := time.Now()
	tx, e := db.Begin(context.Background())
	if e != nil {
		return t, e
	}
	for i, s := range data.Texts {
		if _, e = tx.Exec(context.Background(), "INSERT INTO docs VALUES(?,?)", int64(i+1), s); e != nil {
			return t, errors.Join(e, tx.Rollback())
		}
	}
	if e = tx.Commit(context.Background()); e != nil {
		return t, e
	}
	t.InsertMS = elapsed(start)
	t.BeforeBytes, e = size(path)
	if e != nil {
		return t, e
	}
	start = time.Now()
	e = db.CreateGramsIndex(context.Background(), "docs_grams", "docs", []string{"title"})
	t.BuildMS = elapsed(start)
	if e != nil {
		return t, e
	}
	t.AfterBytes, e = size(path)
	if e != nil {
		return t, e
	}
	t.IndexBytes = t.AfterBytes - t.BeforeBytes
	t.DBAmplification = float64(t.AfterBytes) / float64(t.BeforeBytes)
	t.IndexOverText = float64(t.IndexBytes) / float64(textBytes)
	if e = db.CheckExtensionIndex(context.Background(), "docs_grams"); e != nil {
		return t, e
	}
	t.IntegrityBeforeClose = true
	if e = db.Close(); e != nil {
		return t, e
	}
	db = nil
	t.DatabaseSHA256, e = fileHash(path)
	if e != nil {
		return t, e
	}
	// Both profiles run against this normally closed persisted DB. The first
	// profile alternates across builds to expose, not conceal, order/cache effects.
	caches := []string{"cold_connection", "warm_process"}
	if number%2 == 0 {
		caches[0], caches[1] = caches[1], caches[0]
	}
	for _, cache := range caches {
		samples, e := measure(path, cache, data, number)
		t.Samples = append(t.Samples, samples...)
		if e != nil {
			return t, e
		}
	}
	db, e = sqlite.Open(path, sqlite.Options{ReadOnly: true, NoCreate: true})
	if e != nil {
		return t, e
	}
	if e = db.CheckExtensionIndex(context.Background(), "docs_grams"); e != nil {
		return t, e
	}
	t.IntegrityAfterReopen = true
	return t, nil
}
func run(c config) (r report, err error) {
	started := time.Now()
	defer func() {
		r.Finished = time.Now().UTC().Format(time.RFC3339Nano)
		r.ElapsedSeconds = time.Since(started).Seconds()
		if err != nil {
			r.Status = "failed"
			r.Error = err.Error()
			r.Errors = append(r.Errors, err.Error())
		}
	}()
	r = report{SchemaVersion: 1, Benchmark: "grams", WorkloadVersion: corpusVersion, SourceID: c.SourceID, HardwareID: c.Environment, DatasetSeed: c.Seed, PageSize: c.PageSize, CacheCondition: "cold_connection and warm_process; uncontrolled OS caches; no cache flush", Errors: []string{}, Format: "gosqlite-grams-benchmark-v1", Status: "incomplete", Started: time.Now().UTC().Format(time.RFC3339Nano), Config: c, GoVersion: runtime.Version(), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, CPUs: runtime.NumCPU(), GOMAXPROCS: runtime.GOMAXPROCS(0), Notes: []string{
		"Native persisted SQLite-format DB and grams13 shadow B-trees; no SQL oracle or external engine.",
		"Source identity and environment description are operator supplied, not automatically attested. Record the source manifest with this report.",
		"NFC-normalized synthetic input only; production LIKE_RAW_V1 is raw ASCII-folded LIKE and is not normalized.",
		"Joint grams13 index stores 1/2/3-grams in both LIKE_RAW_V1 and MATCH_CONFIG_V1; build time and incremental file bytes measure the whole index, not separate per-length builds.",
		"Incremental database bytes include catalog, postings, frequencies, both namespaces, B-tree structure and allocated slack; they are not isolated live payload bytes or peak disk/RSS.",
		"cold_connection: each query opens its own read-only connection without workload warmup. The process and OS caches may already be warm from build/prior queries; no cold-OS-cache claim or cache flush.",
		"warm_process: one reopened read-only connection runs the complete automatic/scan workload once unmeasured before timed queries. OS cache state is uncontrolled.",
		"Query timings include parse, planning, native execution, full result consumption and Rows.Close; exclude connection Open/Close, warmup, result comparison and integrity checks.",
		"Independent builds reuse exactly the same corpus and process; cache profile order alternates by build, automatic/scan query order alternates by query/build. Sequential single-threaded workload, rollback-journal mode, no concurrent writers.",
		"P50/P95 use nearest-rank empirical quantiles; raw timings are retained. Repeats expose descriptive variability only: correlated samples, shared host, no confidence interval or speedup claim. Small sample P95 can equal the maximum.",
		"Logical source/posting/frequency counters are not physical I/O counters. Fallback controls are reported separately and never called indexed latency.",
		"Normal close/reopen and extension consistency are checked, not timed as recovery. No crash, fault injection, OS-cache eviction, or recovery measurement.",
	}}
	if b, ok := debug.ReadBuildInfo(); ok {
		r.BuildSettings = b.Settings
		r.Modules = b.Deps
	}
	if err = c.validate(); err != nil {
		return r, err
	}
	data, hash, bytes := makeCorpus(c)
	r.DatasetSHA256 = hash
	r.SourceTextBytes = bytes
	var builds []float64
	for i := 1; i <= c.Repeats; i++ {
		t, e := runTrial(c, data, bytes, i)
		r.Trials = append(r.Trials, t)
		if e != nil {
			return r, fmt.Errorf("repeat %d: %w", i, e)
		}
		builds = append(builds, t.BuildMS)
	}
	r.BuildDistribution = summarize(builds)
	r.Status = "pass"
	return r, nil
}
func main() {
	c := config{}
	flag.IntVar(&c.Rows, "rows", 256, "source rows (64..20000)")
	flag.IntVar(&c.Queries, "queries", 12, "queries per gram length (1..128)")
	flag.IntVar(&c.Repeats, "repeats", 3, "separate database builds (2..10)")
	flag.Int64Var(&c.Seed, "seed", 2026101052, "fixed corpus seed")
	page := flag.Uint("page-size", 4096, "database page size (512, 4096, 65536)")
	pages := flag.Uint("max-pages", 32768, "index-build dirty-page budget (4096..65536)")
	flag.StringVar(&c.SourceID, "source-id", "", "exact source commit/tree or manifest identity, required")
	flag.StringVar(&c.Environment, "environment", "", "hardware and measurement conditions, required")
	flag.BoolVar(&c.CorpusOnly, "corpus-only", false, "emit deterministic corpus JSON only; no DB or timings")
	flag.Parse()
	c.PageSize = uint32(*page)
	c.MaxPages = uint32(*pages)
	// Reject out-of-range platform uint values before using the narrowed fields.
	if *page > 65536 || *pages > 65536 || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "invalid bounded options or positional arguments")
		returnWithFailure()
	}
	if e := c.validate(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		returnWithFailure()
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if c.CorpusOnly {
		data, _, _ := makeCorpus(c)
		if e := enc.Encode(data); e != nil {
			fmt.Fprintln(os.Stderr, e)
			returnWithFailure()
		}
		return
	}
	r, e := run(c)
	if e != nil {
		r.Status = "failed"
		r.Error = e.Error()
	}
	if writeErr := enc.Encode(r); writeErr != nil {
		fmt.Fprintln(os.Stderr, writeErr)
		returnWithFailure()
	}
	if e != nil {
		fmt.Fprintln(os.Stderr, e)
		returnWithFailure()
	}
}

// This is an ordinary CLI failure exit after run's deferred native cleanup, not
// a process-exit/crash test or an interrupted database operation.
func returnWithFailure() { os.Exit(1) }
