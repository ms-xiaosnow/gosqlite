// Command transactions measures ordinary native transactions and real-duration
// concurrent soak. It never kills a process, mutates selected files, or injects faults.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"time"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

const harnessRevision = "ordinary-transactions-v4"

const workloadVersion = "ordinary-transfer-history-v1"
const epochAttempts = 64
const maxEpochs = 4096
const memoryLimit = 256 << 20

type Config struct {
	Mode         string        `json:"mode"`
	Profile      string        `json:"profile"`
	Duration     time.Duration `json:"requested_active_duration_ns"`
	Transactions int           `json:"requested_writer_attempts"`
	Rows         int           `json:"rows"`
	Readers      int           `json:"readers"`
	Batch        int           `json:"transfer_pairs_per_transaction"`
	Seed         uint64        `json:"dataset_seed"`
	WriterPause  time.Duration `json:"writer_pause_ns"`
	ReaderPause  time.Duration `json:"reader_pause_ns"`
	SnapshotHold time.Duration `json:"snapshot_hold_ns"`
	MaxDataBytes int64         `json:"max_data_bytes"`
	SourceID     string        `json:"source_id"`
	HardwareID   string        `json:"hardware_id"`
}

func (c Config) Validate() error {
	if c.Mode != "throughput" && c.Mode != "soak" {
		return errors.New("mode must be throughput or soak")
	}
	if c.Profile != "rollback" && c.Profile != "wal-shared" && c.Profile != "encrypted-shared" {
		return errors.New("profile must be rollback, wal-shared, or encrypted-shared; exclusive modes do not provide this concurrent workload")
	}
	if c.Duration < time.Second || c.Duration > 24*time.Hour || c.Transactions < 4 || c.Transactions > 200000 || c.Rows < 4 || c.Rows > 4096 || c.Readers < 1 || c.Readers > 8 || c.Batch < 1 || c.Batch > 16 || c.Batch*2 > c.Rows {
		return errors.New("bounds: duration 1s..24h, transactions 4..200000, rows 4..4096, readers 1..8, pairs 1..min(16,rows/2)")
	}
	if c.WriterPause < 0 || c.WriterPause > time.Second || c.ReaderPause < 0 || c.ReaderPause > time.Second || c.SnapshotHold < 0 || c.SnapshotHold > 100*time.Millisecond {
		return errors.New("pause/hold bounds exceeded")
	}
	if c.MaxDataBytes < 1<<20 || c.MaxDataBytes > 1<<30 {
		return errors.New("max-data-bytes must be 1MiB..1GiB")
	}
	if c.SourceID == "" || c.HardwareID == "" || len(c.SourceID) > 256 || len(c.HardwareID) > 256 {
		return errors.New("source-id and hardware-id are required, at most 256 bytes each")
	}
	return nil
}

type Report struct {
	HarnessRevision              string `json:"harness_revision"`
	UnverifiedRetirement         bool   `json:"unverified_retirement"`
	retainedDiagnostics          []error
	UnverifiedCloseHandles       int `json:"unverified_close_handles"`
	CloseFailures                int `json:"close_failures"`
	retainedOwners               *closeLedger
	FinalModelGeneration         int64             `json:"final_model_generation"`
	FinalModelSHA256             string            `json:"final_model_sha256"`
	MaxObservedDataFiles         int               `json:"max_observed_data_files"`
	FinalEncryptedCommitSequence uint64            `json:"final_encrypted_commit_sequence"`
	BuildSettings                map[string]string `json:"build_settings"`
	SchemaVersion                int               `json:"schema_version"`
	Benchmark                    string            `json:"benchmark"`
	WorkloadVersion              string            `json:"workload_version"`
	WorkloadSHA256               string            `json:"workload_sha256"`
	DatasetSHA256                string            `json:"dataset_sha256"`
	Config                       Config            `json:"config"`
	Status                       string            `json:"status"`
	Errors                       []string          `json:"errors"`
	StartedUTC                   string            `json:"started_utc"`
	FinishedUTC                  string            `json:"finished_utc"`
	ElapsedSeconds               float64           `json:"elapsed_seconds"`
	ActiveSeconds                float64           `json:"active_seconds"`
	GoVersion                    string            `json:"go_version"`
	GOOS                         string            `json:"goos"`
	GOARCH                       string            `json:"goarch"`
	CPUs                         int               `json:"logical_cpus"`
	GOMAXPROCS                   int               `json:"gomaxprocs"`
	PageSize                     int               `json:"page_size"`
	CacheCondition               string            `json:"cache_condition"`
	DataDirectory                string            `json:"retained_data_directory"`
	MaxObservedDataBytes         int64             `json:"max_observed_data_bytes"`
	MaxObservedHeapBytes         uint64            `json:"max_observed_heap_bytes"`
	Writer                       Counts            `json:"writer"`
	Readers                      Counts            `json:"readers"`
	WriteLatency                 LatencyReport     `json:"writer_attempt_latency"`
	ReadLatency                  LatencyReport     `json:"reader_attempt_latency"`
	CheckpointLatency            LatencyReport     `json:"ordinary_checkpoint_latency"`
	ReopenLatency                LatencyReport     `json:"ordinary_reopen_latency"`
	CheckpointStatus             string            `json:"checkpoint_status"`
	Checkpoints                  int               `json:"successful_checkpoints"`
	Reopens                      int               `json:"verified_reopens"`
	FinalModelVerified           bool              `json:"final_model_verified"`
	Shutdown                     string            `json:"shutdown"`
	CommitsPerSecond             float64           `json:"committed_transactions_per_active_second"`
	AttemptsPerSecond            float64           `json:"writer_attempts_per_active_second"`
	EndToEndCommitsPerSecond     float64           `json:"committed_transactions_per_elapsed_second"`
	WindowRates                  Summary           `json:"window_commit_rate_statistics"`
	Windows                      []Window          `json:"windows"`
	Limits                       map[string]int64  `json:"harness_limits"`
	Notes                        []string          `json:"notes"`
}

func main() {
	c := Config{}
	flag.StringVar(&c.Mode, "mode", "throughput", "throughput (fixed attempts) or soak (actual active duration)")
	flag.StringVar(&c.Profile, "profile", "rollback", "rollback, wal-shared, encrypted-shared")
	flag.DurationVar(&c.Duration, "duration", 30*time.Second, "soak active duration; maintenance excluded")
	flag.IntVar(&c.Transactions, "transactions", 256, "writer attempts for throughput mode")
	flag.IntVar(&c.Rows, "rows", 64, "fixed row count")
	flag.IntVar(&c.Readers, "readers", 2, "independent reader connections")
	flag.IntVar(&c.Batch, "batch", 2, "transfer pairs per writer attempt")
	flag.Uint64Var(&c.Seed, "seed", 20261010, "deterministic workload seed")
	flag.DurationVar(&c.WriterPause, "writer-pause", 0, "cooperative pause after writer attempts")
	flag.DurationVar(&c.ReaderPause, "reader-pause", time.Millisecond, "pause after reader attempts")
	flag.DurationVar(&c.SnapshotHold, "snapshot-hold", time.Millisecond, "hold snapshot between two complete model checks")
	flag.Int64Var(&c.MaxDataBytes, "max-data-bytes", 128<<20, "storage guard; encrypted engine also enforces retained quota")
	flag.StringVar(&c.SourceID, "source-id", "", "required exact source/tree or manifest identity")
	flag.StringVar(&c.HardwareID, "hardware-id", "", "required controlled host label; no inferred hardware identity")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	rep := run(ctx, c)
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	// run returns only after workers joined and normal DB closure was attempted.
	if err := enc.Encode(rep); err != nil {
		fmt.Fprintln(os.Stderr, err)
		runtime.KeepAlive(rep.retainedOwners)
		runtime.KeepAlive(rep.retainedDiagnostics)
		os.Exit(1)
	}
	// References and original managed diagnostics remain alive through emission.
	// Process completion does not establish retirement of failed native owners.
	runtime.KeepAlive(rep.retainedOwners)
	runtime.KeepAlive(rep.retainedDiagnostics)
	if rep.UnverifiedRetirement {
		fmt.Fprintln(os.Stderr, "close/retirement remains unverified; process completion is not proof of native owner retirement")
	}
	if rep.Status != "passed" {
		os.Exit(1)
	}
}

// Synthetic public fixture key, never a user's key; each borrow has owned bytes.
type fixtureProvider struct{}
type fixtureMaterial struct{ key []byte }

func (fixtureProvider) Acquire(ctx context.Context, _ encryption.KeyRequest) (encryption.KeyMaterial, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	k := sha256.Sum256([]byte("gosqlite ordinary benchmark public fixture key v1"))
	return &fixtureMaterial{append([]byte(nil), k[:]...)}, nil
}
func (m *fixtureMaterial) Kind() encryption.CredentialKind { return encryption.RawKey256 }
func (m *fixtureMaterial) Bytes() []byte                   { return m.key }
func (m *fixtureMaterial) Release()                        { clear(m.key) }
func options(c Config) sqlite.Options {
	o := sqlite.Options{PageSize: 4096, MaxSortBytes: 1 << 20, MaxTransactionPages: 128, MaxTransactionUndoBytes: 8 << 20}
	if c.Profile == "wal-shared" {
		o.WALShared = true
		o.KeepWALOnClose = true
	}
	if c.Profile == "encrypted-shared" {
		o.KeepWALOnClose = true
		o.Encryption = &sqlite.EncryptionOptions{Mode: sqlite.EncryptionWALShared, CredentialKind: encryption.RawKey256, KeyProvider: fixtureProvider{}, MaxGenerations: 100000, MaxDirectoryEntries: 200000, MaxRetainedBytes: uint64(c.MaxDataBytes), Limits: encryption.Limits{MaxPages: 2048, MaxLogicalBytes: 8 << 20}, WALLimits: sqlite.EncryptionWALLimits{MaxSegments: 128, MaxMemoryBytes: 64 << 20}}
	}
	return o
}
func digest(v any) string {
	b, _ := json.Marshal(v)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
func dataSize(dir string) (int64, int, error) {
	var n int64
	files := 0
	err := filepath.WalkDir(dir, func(_ string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}
		files++
		i, e := d.Info()
		if e == nil {
			n += i.Size()
		}
		return e
	})
	return n, files, err
}
func run(ctx context.Context, c Config) (rep Report) {
	started := time.Now()
	rep = Report{HarnessRevision: harnessRevision, SchemaVersion: 1, Benchmark: "native-transactions-and-soak", WorkloadVersion: workloadVersion, Config: c, Status: "failed", StartedUTC: started.UTC().Format(time.RFC3339Nano), GoVersion: runtime.Version(), GOOS: runtime.GOOS, GOARCH: runtime.GOARCH, CPUs: runtime.NumCPU(), GOMAXPROCS: runtime.GOMAXPROCS(0), PageSize: 4096, CacheCondition: "warm OS cache, bootstrap excluded, no OS cache flush; new DB each invocation", CheckpointStatus: "not_run", Errors: []string{}, Windows: []Window{}, Shutdown: "not_started", Limits: map[string]int64{"rows": 4096, "reader_goroutines": 8, "writer_goroutines": 1, "writer_attempts_per_epoch": epochAttempts, "epochs": maxEpochs, "soft_go_memory_bytes": memoryLimit, "transaction_pages": 128, "transaction_undo_bytes": 8 << 20, "encrypted_generations": 100000, "encrypted_directory_entries": 200000, "encrypted_adapter_memory_bytes": 64 << 20, "encrypted_logical_bytes": 8 << 20}, Notes: []string{"All latency histograms include BUSY/canceled outcomes; percentiles are power-of-two microsecond bucket upper bounds, not exact order statistics.", "One writer and independent reader handles overlap. BUSY is reported, never silently retried without counting. Commit has at most 20 cooperative retries.", "Every successful reader checks all rows twice against an exact committed-generation model within one pinned snapshot. Histories are retained only until epoch readers join.", "Every epoch joins canceled workers, performs supported normal checkpoint, closes all connections, reopens the same intact database, and checks the exact model.", "No fault injection, abrupt process exit with an open database, crash recovery, physical tamper, namespace race, external oracle, or rekey. Ordinary reopen/checkpoint timings are not recovery time.", "Encrypted-shared uses a public synthetic fixture key and retains immutable ciphertext history; quota/epoch exhaustion fails the run rather than truncating or calling a shorter run passed.", "Windows within a run are correlated. CI bounds are null; independent repeated runs on identical source/workload/hardware conditions are needed before a confidence-qualified regression decision.", "All data directories and JSON stdout are retained for investigation. No results are deleted or rewritten by the harness.", "Any close/retirement diagnostic latches failure. Unverified handles and original diagnostics remain referenced through report emission; no automatic retry is added. Process completion is not evidence of native retirement, and terminal uncertainty is not recovered by this harness."}}
	rep.BuildSettings = map[string]string{}
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			switch setting.Key {
			case "CGO_ENABLED", "GOAMD64", "GOARM64", "-race", "-trimpath":
				rep.BuildSettings[setting.Key] = setting.Value
			}
		}
	}
	var writes, reads, checkpoints, reopens Latency
	var rates Moments
	defer func() {
		rep.FinishedUTC = time.Now().UTC().Format(time.RFC3339Nano)
		rep.ElapsedSeconds = time.Since(started).Seconds()
		rep.WriteLatency = writes.Report()
		rep.ReadLatency = reads.Report()
		rep.CheckpointLatency = checkpoints.Report()
		rep.ReopenLatency = reopens.Report()
		rep.WindowRates = summarize(rates)
		if rep.ActiveSeconds > 0 {
			rep.CommitsPerSecond = float64(rep.Writer.Committed) / rep.ActiveSeconds
			rep.AttemptsPerSecond = float64(rep.Writer.Attempts) / rep.ActiveSeconds
		}
		rep.EndToEndCommitsPerSecond = float64(rep.Writer.Committed) / rep.ElapsedSeconds
	}()
	fail := func(e error) { recordFailure(&rep, e) }
	if e := c.Validate(); e != nil {
		fail(e)
		setReportStatus(&rep, "rejected")
		return
	}
	oldLimit := debug.SetMemoryLimit(memoryLimit)
	defer debug.SetMemoryLimit(oldLimit)
	identity := c
	identity.SourceID, identity.HardwareID = "", ""
	rep.WorkloadSHA256 = digest(struct {
		Version string
		Config  Config
	}{workloadVersion, identity})
	m := newModel(c.Rows, c.Seed)
	defer func() {
		m.mu.RLock()
		defer m.mu.RUnlock()
		rep.FinalModelGeneration = m.generation
		rep.FinalModelSHA256 = digest(m.current)
	}()
	rep.DatasetSHA256 = digest(m.current)
	dir, e := os.MkdirTemp("", "gosqlite-ordinary-transactions-")
	if e != nil {
		fail(e)
		return
	}
	rep.DataDirectory = dir
	path := filepath.Join(dir, "benchmark.db")
	o := options(c)
	// Install ownership finalization before any potentially acquiring Open.
	owners := newCloseLedger()
	rep.retainedOwners = owners
	defer func() {
		if e := finalizeClosures(&rep, owners); e != nil {
			fail(e)
		}
	}()
	db, e := sqlite.Open(path, o)
	if e != nil {
		recordOpenFailure(&rep, e)
		return
	}
	writerOwner := owners.acquire(db, "writer")
	rep.Shutdown = "normal_close_pending"
	if e = bootstrap(db, m.current); e != nil {
		fail(e)
		return
	}
	rep.CheckpointStatus = "supported"
	if c.Profile == "rollback" {
		rep.CheckpointStatus = "skipped: rollback API has no WAL checkpoint"
	}
	var attempt uint64
	for epoch := 0; epoch < maxEpochs; epoch++ {
		if ctx.Err() != nil {
			fail(ctx.Err())
			return
		}
		if c.Mode == "throughput" && attempt >= uint64(c.Transactions) {
			break
		}
		if c.Mode == "soak" && rep.ActiveSeconds >= c.Duration.Seconds() {
			break
		}
		limit := epochAttempts
		if c.Mode == "throughput" && uint64(limit) > uint64(c.Transactions)-attempt {
			limit = int(uint64(c.Transactions) - attempt)
		}
		budget := 5 * time.Second
		if c.Mode == "soak" {
			left := c.Duration - time.Duration(rep.ActiveSeconds*float64(time.Second))
			if left < budget {
				budget = left
			}
		}
		rep.FinalModelVerified = false
		w, wl, rl, next, err := executeEpoch(ctx, db, path, o, c, m, owners, epoch, attempt, limit, budget)
		attempt = next
		rep.Windows = append(rep.Windows, w)
		rep.ActiveSeconds += w.ActiveSeconds
		rep.Writer.Merge(w.Writer)
		rep.Readers.Merge(w.Readers)
		writes.Merge(wl)
		reads.Merge(rl)
		rates.Add(w.CommittedPerSecond)
		if err != nil {
			fail(err)
			return
		}
		if c.Profile != "rollback" {
			t := time.Now()
			err = db.Checkpoint(context.Background())
			checkpoints.Add(time.Since(t))
			if err != nil {
				fail(fmt.Errorf("ordinary checkpoint: %w", err))
				return
			}
			rep.Checkpoints++
		}
		if err = owners.close(writerOwner); err != nil {
			fail(err)
			return
		}
		db = nil
		rep.Shutdown = owners.shutdown()
		t := time.Now()
		db, err = sqlite.Open(path, o)
		reopens.Add(time.Since(t))
		if err != nil {
			fail(fmt.Errorf("ordinary reopen: %w", err))
			return
		}
		writerOwner = owners.acquire(db, "writer")
		if err = verifyCurrent(db, m); err != nil {
			fail(fmt.Errorf("reopened model: %w", err))
			return
		}
		rep.Reopens++
		rep.FinalModelVerified = true
		m.prune()
		size, files, err := dataSize(dir)
		if err != nil {
			fail(err)
			return
		}
		if files > rep.MaxObservedDataFiles {
			rep.MaxObservedDataFiles = files
		}
		if c.Profile == "encrypted-shared" {
			info, e := db.VerifyEncrypted(context.Background())
			if e != nil {
				fail(e)
				return
			}
			rep.FinalEncryptedCommitSequence = info.CommitSequence
		}
		if size > rep.MaxObservedDataBytes {
			rep.MaxObservedDataBytes = size
		}
		if size > c.MaxDataBytes {
			fail(fmt.Errorf("data guard exceeded: observed %d > %d; incomplete", size, c.MaxDataBytes))
			return
		}
		var mem runtime.MemStats
		runtime.ReadMemStats(&mem)
		if mem.HeapAlloc > rep.MaxObservedHeapBytes {
			rep.MaxObservedHeapBytes = mem.HeapAlloc
		}
		if mem.HeapAlloc > memoryLimit {
			fail(fmt.Errorf("sampled heap guard exceeded: %d; incomplete", mem.HeapAlloc))
			return
		}
	}
	if c.Mode == "soak" && rep.ActiveSeconds < c.Duration.Seconds() {
		fail(errors.New("epoch guard reached before requested active duration"))
		return
	}
	if c.Mode == "throughput" && attempt != uint64(c.Transactions) {
		fail(errors.New("epoch guard reached before requested attempts"))
		return
	}
	if !rep.Writer.Valid() || !rep.Readers.Valid() || rep.Writer.Committed == 0 || rep.Readers.Success == 0 || rep.Writer.RolledBack == 0 || rep.Writer.SavepointRollback == 0 || rep.Writer.SavepointRelease == 0 || !rep.FinalModelVerified {
		fail(errors.New("required outcome coverage or accounting missing; incomplete"))
		return
	}
	setReportStatus(&rep, "passed")
	return
}
