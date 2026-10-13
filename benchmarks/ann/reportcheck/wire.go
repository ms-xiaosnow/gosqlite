package reportcheck

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
	"unicode/utf8"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/extension/vector/ann"
	"github.com/ms-xiaosnow/gosqlite/internal/pager"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// SchemaVersion is the only accepted report schema.
const SchemaVersion = 3
const schemaVersion = SchemaVersion

// MaxReportBytes bounds the JSON bytes accepted by DecodeReport.
const MaxReportBytes = 256 << 20
const maxErrorNodes = 128
const maxReportBytes = MaxReportBytes
const maxReportNodes = 4_000_000
const maxReportDepth = 64

// Only losslessly typed, closed objects enter material wire identity. In
// particular no map[string]any or direct ann.Config JSON is part of a digest.
type DBLimits struct {
	Journal                 string `json:"journal"`
	PageSize                uint32 `json:"page_size"`
	SectorSize              uint32 `json:"sector_size"`
	MaxSortBytes            int64  `json:"max_sort_bytes"`
	MaxTransactionPages     uint32 `json:"max_transaction_pages"`
	MaxTransactionUndoBytes int64  `json:"max_transaction_undo_bytes"`
	MaxFreelistPages        uint32 `json:"max_freelist_pages"`
	EffectiveUndoBytes      uint64 `json:"effective_undo_bytes"`
	ReopenNoCreate          bool   `json:"reopen_no_create"`
}
type LimitsWire struct {
	DB              DBLimits             `json:"native_db"`
	Vector          vector.SearchLimits  `json:"native_vector_query"`
	ANN             sqlite.ANNWorkLimits `json:"native_ann_query"`
	ErrorGraphNodes int                  `json:"error_graph_nodes"`
	ReportNodes     int                  `json:"max_report_wire_nodes"`
	ReportDepth     int                  `json:"max_report_wire_depth"`
	ReportBytes     int                  `json:"max_report_wire_bytes"`
	Duration        string               `json:"duration"`
	Memory          string               `json:"memory"`
	Disk            string               `json:"disk"`
}

func nativeLimits() LimitsWire {
	o := nativeOptions()
	return LimitsWire{DB: DBLimits{"DELETE", o.PageSize, o.SectorSize, o.MaxSortBytes, o.MaxTransactionPages, o.MaxTransactionUndoBytes, 1 << 20, pager.UndoLimit(pager.Options{MaxDirtyPages: o.MaxTransactionPages, MaxUndoBytes: o.MaxTransactionUndoBytes}, o.PageSize), true}, Vector: vector.DefaultSearchLimits(), ANN: sqlite.DefaultANNWorkLimits(), ErrorGraphNodes: maxErrorNodes, ReportBytes: maxReportBytes, ReportNodes: maxReportNodes, ReportDepth: maxReportDepth, Duration: "boundary-only; native calls and retirement are not preempted", Memory: "Cartesian estimate plus sampled process Go heap; not a hard RSS limit", Disk: "Cartesian estimate plus boundary directory-byte observations; not an OS quota"}
}

type BudgetWire struct {
	DurationNS    int64  `json:"duration_ns"`
	MemoryBytes   uint64 `json:"memory_bytes"`
	DataBytes     uint64 `json:"data_bytes"`
	AdmissionWork uint64 `json:"admission_work"`
}

func budgets(c Config) BudgetWire {
	return BudgetWire{int64(c.MaxDuration), c.MaxMemoryBytes, c.MaxDataBytes, c.MaxWork}
}

type QueryPolicy struct {
	Version        string `json:"version"`
	Table          string `json:"table"`
	Field          string `json:"field"`
	Representation string `json:"representation"`
	Metric         string `json:"metric"`
	Method         string `json:"method"`
	MethodVersion  int    `json:"method_version"`
	Index          string `json:"index"`
	Generation     int64  `json:"generation"`
	K              [3]int `json:"k"`
	EFSearch       int    `json:"ef_search"`
	MaxEFSearch    int    `json:"max_ef_search"`
	Probes         int    `json:"probes"`
	MaxProbes      int    `json:"max_probes"`
	FallbackPolicy string `json:"fallback_policy"`
}

func queryPolicy(c Config) QueryPolicy {
	p := QueryPolicy{Version: "native-dense-paired-policy-v3", Table: "items", Field: "embedding", Representation: "dense-float32-gsv", Metric: "l2", Method: c.Method, MethodVersion: 1, Index: "items_hnsw", Generation: 1, K: [3]int{1, 10, 100}, EFSearch: 40, MaxEFSearch: 1000, FallbackPolicy: "retain-and-fail-any-exact-fallback"}
	if c.Method == "ivfflat" {
		p.Index = "items_ivf"
		p.EFSearch = 0
		p.MaxEFSearch = 0
		p.Probes = c.Probes
		p.MaxProbes = c.effectiveMaxProbes()
	}
	return p
}

type TimingWire struct {
	Percentiles string `json:"percentiles"`
	Insert      string `json:"insert_ms"`
	Build       string `json:"index_build_ms"`
	Query       string `json:"query_ms"`
	Order       string `json:"query_order"`
	Warmup      string `json:"warmup"`
	Elapsed     string `json:"elapsed_seconds"`
	Cache       string `json:"cache"`
}

func timingPolicy() TimingWire {
	return TimingWire{"sorted observed index floor((N-1)*p); no interpolation", "Begin, all inserts, boundary checks and successful Commit; table creation excluded", "Begin, native index construction and successful Commit; checks/reopen excluded", "single native Tx.VectorSearch call only; Begin, Rollback, correctness checks, timing storage and boundary guards excluded", "K ascending 1/10/100; query ordinal ascending; exact first on even zero-based ordinal; one shared read transaction per pair", "one exact and one ANN call per K in separate explicit read transactions; not timed", "entry through normal resource finalization; recorded UTC interval; includes generation, checks and reopen; serialization excluded", "OS cache neither flushed nor isolated; warmup performed after ordinary close/reopen"}
}

// The existing native canonical method envelope includes every IVF training and
// policy field and preserves uint64 seed bits. RawMessage never decodes numbers
// through float64. Pretty-print whitespace is removed before the native parser,
// which rejects missing/duplicate/unknown fields or changed ordering/policies.
func canonicalAlgorithm(c Config) (json.RawMessage, error) {
	v, e := algorithm(c)
	if e != nil {
		return nil, e
	}
	b, e := v.Canonical()
	return json.RawMessage(b), e
}
func parseAlgorithm(raw json.RawMessage) (ann.Config, error) {
	if len(raw) > 8192 {
		return ann.Config{}, errors.New("algorithm envelope too large")
	}
	var compact bytes.Buffer
	if e := json.Compact(&compact, raw); e != nil {
		return ann.Config{}, e
	}
	return ann.ParseConfig(compact.Bytes())
}
func canonicalBytes(raw json.RawMessage) (json.RawMessage, error) {
	v, e := parseAlgorithm(raw)
	if e != nil {
		return nil, e
	}
	b, e := v.Canonical()
	return json.RawMessage(b), e
}

// DecodeReport is the schema3 wire admission boundary. It rejects duplicate or
// unknown keys, missing fields (including false/zero safety fields), null typed
// objects, wrong numeric types, trailing documents and excessive nesting/size.
// It performs no file or native database operation and never reinterprets v2.
func DecodeReport(data []byte) (Report, error) {
	var r Report
	if len(data) == 0 || len(data) > maxReportBytes || !utf8.Valid(data) {
		return r, errors.New("report wire size exceeded")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	nodes := 0
	var value func(int) error
	value = func(depth int) error {
		nodes++
		if depth > maxReportDepth || nodes > maxReportNodes {
			return errors.New("report JSON shape budget exceeded")
		}
		tok, e := d.Token()
		if e != nil {
			return e
		}
		if delim, ok := tok.(json.Delim); ok {
			switch delim {
			case '{':
				keys := map[string]bool{}
				for d.More() {
					tok, e := d.Token()
					if e != nil {
						return e
					}
					key, ok := tok.(string)
					if !ok || keys[key] {
						return errors.New("duplicate/nonstring JSON key")
					}
					keys[key] = true
					if e = value(depth + 1); e != nil {
						return e
					}
				}
				end, e := d.Token()
				if e != nil || end != json.Delim('}') {
					return errors.New("invalid object closure")
				}
			case '[':
				for d.More() {
					if e = value(depth + 1); e != nil {
						return e
					}
				}
				end, e := d.Token()
				if e != nil || end != json.Delim(']') {
					return errors.New("invalid array closure")
				}
			default:
				return errors.New("unexpected JSON delimiter")
			}
		}
		return nil
	}
	if e := value(0); e != nil {
		return r, e
	}
	if _, e := d.Token(); e != io.EOF {
		return r, errors.New("trailing report JSON")
	}
	if e := requiredFields(data, reflect.TypeOf(r), 0); e != nil {
		return r, e
	}
	d = json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if e := d.Decode(&r); e != nil {
		return r, e
	}
	if e := r.ValidatePass(); e != nil {
		return r, e
	}
	return r, nil
}

var rawMessageType = reflect.TypeOf(json.RawMessage{})

type wireField struct {
	typ      reflect.Type
	optional bool
}

func wireFields(t reflect.Type) map[string]wireField {
	out := map[string]wireField{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}
		if f.Anonymous && tag == "" {
			for k, v := range wireFields(f.Type) {
				out[k] = v
			}
			continue
		}
		parts := strings.Split(tag, ",")
		name := f.Name
		if parts[0] != "" {
			name = parts[0]
		}
		optional := false
		for _, p := range parts[1:] {
			optional = optional || p == "omitempty"
		}
		out[name] = wireField{f.Type, optional}
	}
	return out
}
func requiredFields(data json.RawMessage, t reflect.Type, depth int) error {
	if depth > maxReportDepth {
		return errors.New("report object depth exceeded")
	}
	if t == rawMessageType {
		return nil
	}
	null := bytes.Equal(bytes.TrimSpace(data), []byte("null"))
	if t.Kind() == reflect.Pointer {
		if null {
			return nil
		}
		return requiredFields(data, t.Elem(), depth+1)
	}
	switch t.Kind() {
	case reflect.Struct:
		var obj map[string]json.RawMessage
		if e := json.Unmarshal(data, &obj); e != nil {
			return e
		}
		if obj == nil {
			return errors.New("null report object")
		}
		fields := wireFields(t)
		for name := range obj {
			if _, ok := fields[name]; !ok {
				return fmt.Errorf("unknown field %s.%s", t.Name(), name)
			}
		}
		for name, f := range fields {
			raw, ok := obj[name]
			if !ok {
				if f.optional {
					continue
				}
				return fmt.Errorf("missing required field %s.%s", t.Name(), name)
			}
			if e := requiredFields(raw, f.typ, depth+1); e != nil {
				return e
			}
		}
	case reflect.Slice, reflect.Array:
		var arr []json.RawMessage
		if e := json.Unmarshal(data, &arr); e != nil {
			return e
		}
		if t.Kind() == reflect.Array && len(arr) != t.Len() {
			return errors.New("array length mismatch")
		}
		for _, raw := range arr {
			if e := requiredFields(raw, t.Elem(), depth+1); e != nil {
				return e
			}
		}
	case reflect.Map:
		var obj map[string]json.RawMessage
		if e := json.Unmarshal(data, &obj); e != nil {
			return e
		}
		for _, raw := range obj {
			if e := requiredFields(raw, t.Elem(), depth+1); e != nil {
				return e
			}
		}
	default:
		if null {
			return errors.New("null scalar report field")
		}
	}
	return nil
}
