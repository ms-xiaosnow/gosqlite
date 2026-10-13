package reportcheck

import (
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/extension/vector/ann"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

const harnessRevision = "ordinary-ann-v3"
const workloadVersion = "gaussian-heldout-gsv-v1"
const measurementVersion = "paired-exact-ann-reopened-v3"
const cacheCondition = "warm OS cache; unmeasured warmup; exact/ANN timing order alternates per query; no cache flush"

// Admission estimates deliberately over-count materialized vectors and logical
// distance components. They are not a worst-case native RSS, disk or time bound.
type Config struct {
	Rows           int           `json:"rows"`
	Dimensions     int           `json:"dimensions"`
	Queries        int           `json:"queries"`
	Seed           int64         `json:"dataset_seed"`
	Method         string        `json:"method"`
	Lists          int           `json:"lists"`
	Probes         int           `json:"probes"`
	MaxProbes      int           `json:"max_probes"`
	SourceID       string        `json:"source_id"`
	HardwareID     string        `json:"hardware_id"`
	MaxDuration    time.Duration `json:"max_duration_ns"`
	MaxMemoryBytes uint64        `json:"max_memory_bytes"`
	MaxDataBytes   uint64        `json:"max_data_bytes"`
	MaxWork        uint64        `json:"max_admission_component_work"`
}

func defaults() Config {
	return Config{Rows: 2048, Dimensions: 12, Queries: 48, Seed: 20261010, Method: "hnsw", Lists: 100, Probes: 1, MaxDuration: 10 * time.Minute, MaxMemoryBytes: 256 << 20, MaxDataBytes: 128 << 20, MaxWork: 1_000_000_000}
}

type Admission struct {
	EstimatedReportBytes uint64 `json:"estimated_report_bytes"`
	EstimatedReportNodes uint64 `json:"estimated_report_nodes"`
	VectorComponents     uint64 `json:"vector_components"`
	EstimatedMemoryBytes uint64 `json:"estimated_memory_bytes"`
	EstimatedDataBytes   uint64 `json:"estimated_data_bytes"`
	BuildComponentWork   uint64 `json:"build_component_work"`
	QueryComponentWork   uint64 `json:"query_component_work"`
	TotalComponentWork   uint64 `json:"total_component_work"`
}

func checkedProduct(v ...uint64) (uint64, error) {
	n := uint64(1)
	for _, x := range v {
		if x != 0 && n > ^uint64(0)/x {
			return 0, errors.New("admission product overflow")
		}
		n *= x
	}
	return n, nil
}
func checkedSum(v ...uint64) (uint64, error) {
	var n uint64
	for _, x := range v {
		if n > ^uint64(0)-x {
			return 0, errors.New("admission sum overflow")
		}
		n += x
	}
	return n, nil
}
func sourceIdentity(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	_, e := hex.DecodeString(s)
	return e == nil && s == strings.ToLower(s)
}
func (c Config) admission() (Admission, error) {
	var a Admission
	if c.Method != "hnsw" && c.Method != "ivfflat" {
		return a, errors.New("method must be hnsw or ivfflat")
	}
	if c.Rows < 100 || c.Rows > 100000 || c.Dimensions < 1 || c.Dimensions > 2000 || c.Queries < 1 || c.Queries > 10000 {
		return a, errors.New("bounds: rows 100..100000, dimensions 1..2000, queries 1..10000")
	}
	if c.Lists < 1 || c.Lists > 10000 || c.Lists > c.Rows || c.Probes < 1 || c.Probes > c.Lists || c.MaxProbes < 0 || c.MaxProbes > c.Lists || (c.MaxProbes != 0 && c.MaxProbes < c.Probes) {
		return a, errors.New("bounds: lists 1..min(10000,rows); 1 <= probes <= effective max-probes <= lists")
	}
	if c.Method == "ivfflat" && c.Probes == c.Lists {
		return a, errors.New("probes equals lists always requests exact fallback; schema3 pass requires native ANN")
	}
	if c.Method == "hnsw" && (c.Lists != 100 || c.Probes != 1 || c.MaxProbes != 0) {
		return a, errors.New("IVF flags cannot change an HNSW workload")
	}
	if !sourceIdentity(c.SourceID) || strings.TrimSpace(c.HardwareID) == "" || len(c.HardwareID) > 256 || strings.ContainsAny(c.HardwareID, "\r\n\x00") {
		return a, errors.New("source-id must be an exact lowercase 40/64-hex tree/manifest digest; hardware-id must be a nonblank controlled host label, <=256 bytes")
	}
	if c.MaxDuration < time.Second || c.MaxDuration > 24*time.Hour || c.MaxMemoryBytes < 64<<20 || c.MaxMemoryBytes > 1<<30 || c.MaxDataBytes < 1<<20 || c.MaxDataBytes > 1<<30 || c.MaxWork < 1 || c.MaxWork > 1_000_000_000_000 {
		return a, errors.New("budgets: duration 1s..24h, memory 64MiB..1GiB, data 1MiB..1GiB, admission work 1..1e12")
	}
	// Scalar bounds above make every product below representable in uint64.
	n, d, q := uint64(c.Rows), uint64(c.Dimensions), uint64(c.Queries)
	a.VectorComponents = (n + q) * d
	a.EstimatedReportBytes = 16384 + q*65536
	a.EstimatedReportNodes = 2048 + q*1500
	if a.EstimatedReportBytes > maxReportBytes || a.EstimatedReportNodes > maxReportNodes {
		return a, errors.New("report wire shape admission exceeded")
	}
	a.EstimatedMemoryBytes = 48<<20 + a.VectorComponents*16 + (n+q)*512 + q*3*65536
	a.EstimatedDataBytes = 1<<20 + n*(d*16+2048) + q*65536
	if c.Method == "hnsw" {
		a.BuildComponentWork = n * n * d
	} else {
		a.BuildComponentWork = uint64(min(c.Rows, 10000))*uint64(c.Lists)*20*d + n*uint64(c.Lists)*d
	}
	// Six searches per query (3 K, exact+ANN), plus six warmups. Multiplication
	// by 16 admits cumulative widening; this is a planning proxy, not measured work.
	a.QueryComponentWork = 6 * (q + 1) * n * d * 16
	a.TotalComponentWork = a.BuildComponentWork + a.QueryComponentWork
	if a.EstimatedMemoryBytes > c.MaxMemoryBytes || a.EstimatedDataBytes > c.MaxDataBytes || a.TotalComponentWork > c.MaxWork {
		return a, fmt.Errorf("Cartesian admission exceeds configured budget: memory=%d/%d data=%d/%d component_work=%d/%d", a.EstimatedMemoryBytes, c.MaxMemoryBytes, a.EstimatedDataBytes, c.MaxDataBytes, a.TotalComponentWork, c.MaxWork)
	}
	// Do not knowingly start a workload exceeding unchanged production quotas.
	if n*(d*4+512) > 16<<20 || a.VectorComponents > 1<<27 || (c.Method == "ivfflat" && a.BuildComponentWork > 1<<28) {
		return a, errors.New("workload exceeds conservative unchanged pager/vector quota admission")
	}
	return a, nil
}
func (c Config) effectiveMaxProbes() int {
	if c.MaxProbes == 0 {
		return c.Lists
	}
	return c.MaxProbes
}
func nativeOptions() sqlite.Options {
	return sqlite.Options{PageSize: 4096, SectorSize: 4096, MaxSortBytes: 64 << 20, MaxTransactionPages: 4096, MaxTransactionUndoBytes: 0}
}
func algorithm(c Config) (ann.Config, error) {
	v, e := ann.DefaultConfig(c.Method, c.Dimensions, vector.L2)
	if e != nil {
		return v, e
	}
	if c.Method == "ivfflat" {
		v.Lists = c.Lists
	}
	return v.Normalize()
}
