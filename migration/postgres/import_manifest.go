package postgres

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/extension/vector/ann"
)

const ImportVersion = 1
const ImportConverter = "pgembedding-local-v1-strict-float4"
const MaxImportManifestBytes = 64 << 10
const MaxImportInputBytes int64 = 1 << 30
const MaxImportRecordBytes = 1 << 20
const MaxImportRows int64 = 1000000
const MaxImportRejections int64 = 10000
const MaxImportDryRunKeys = 65536

// ImportManifest is an explicit, bounded offline export contract, never SQL.
// DecimalFidelity describes the exporter's claim; an import cannot restore bits
// which a low-precision export discarded. All fields, including nullability,
// have an explicit mapping. No PostgreSQL connection or credential is accepted.
type ImportManifest struct {
	FormatVersion   int           `json:"format_version"`
	SourceProfile   string        `json:"source_profile"`
	SourceID        string        `json:"source_id"`
	SourceSHA256    string        `json:"source_sha256"`
	SourceSize      int64         `json:"source_size"`
	ExpectedRows    *int64        `json:"expected_rows,omitempty"`
	Format          string        `json:"format"`
	RowOrder        string        `json:"row_order"`
	DecimalFidelity string        `json:"decimal_fidelity"`
	NullToken       string        `json:"null_token"`
	Table           string        `json:"table"`
	Fields          []ImportField `json:"fields"`
	Indexes         []ImportIndex `json:"indexes"`
}
type ImportField struct {
	Source     string `json:"source"`
	Target     string `json:"target"`
	Type       string `json:"type"`
	Encoding   string `json:"encoding"`
	Nullable   bool   `json:"nullable"`
	Dimensions int    `json:"dimensions,omitempty"`
	PrimaryKey bool   `json:"primary_key"`
}
type ImportIndex struct {
	Name               string `json:"name"`
	Column             string `json:"column"`
	Method             string `json:"method"`
	Metric             string `json:"metric"`
	M                  int    `json:"m,omitempty"`
	EFConstruction     int    `json:"ef_construction,omitempty"`
	Lists              int    `json:"lists,omitempty"`
	TrainingSamples    int    `json:"training_samples,omitempty"`
	TrainingIterations int    `json:"training_iterations,omitempty"`
	Seed               uint64 `json:"seed,omitempty"`
}

func DecodeImportManifest(r io.Reader) (ImportManifest, error) {
	var m ImportManifest
	if r == nil {
		return m, fmt.Errorf("import manifest: nil reader")
	}
	raw, err := io.ReadAll(io.LimitReader(r, MaxImportManifestBytes+1))
	if err != nil {
		return m, err
	}
	if len(raw) > MaxImportManifestBytes {
		return m, fmt.Errorf("import manifest: byte limit")
	}
	l := DefaultLimits()
	l.MaxBytes = MaxImportManifestBytes
	l.MaxTokens = 8192
	l.MaxDepth = 8
	if err = strictJSON(raw, &m, l); err != nil {
		return m, fmt.Errorf("import manifest: %w", err)
	}
	return m, m.Validate()
}
func importName(s string) bool {
	if len(s) < 1 || len(s) > 128 {
		return false
	}
	for i, c := range []byte(s) {
		if !(c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || i > 0 && c >= '0' && c <= '9') {
			return false
		}
	}
	lower := strings.ToLower(s)
	return !strings.HasPrefix(lower, "sqlite_") && !strings.HasPrefix(lower, "gosqlite_") && !strings.HasPrefix(lower, "__") && lower != "rowid" && lower != "_rowid_" && lower != "oid"
}
func validImportHash(s string) bool {
	if len(s) != 64 {
		return false
	}
	b, e := hex.DecodeString(s)
	return e == nil && len(b) == 32 && strings.ToLower(s) == s
}
func (m ImportManifest) Validate() error {
	bad := func() error { return fmt.Errorf("import manifest: invalid or unsupported explicit mapping") }
	if m.FormatVersion != ImportVersion || m.SourceProfile != "pg_embedding-real-float4-v1" || len(m.SourceID) < 1 || len(m.SourceID) > 128 || !utf8.ValidString(m.SourceID) || strings.ContainsAny(m.SourceID, "\x00\r\n") || !validImportHash(m.SourceSHA256) || m.SourceSize < 0 || m.SourceSize > MaxImportInputBytes || !importName(m.Table) || len(m.Fields) < 2 || len(m.Fields) > 32 || len(m.Indexes) > 8 {
		return bad()
	}
	if m.ExpectedRows != nil && (*m.ExpectedRows < 0 || *m.ExpectedRows > MaxImportRows) {
		return bad()
	}
	if m.Format != "csv" && m.Format != "ndjson" || m.RowOrder != "ascending" && m.RowOrder != "source" || m.DecimalFidelity != "roundtrip_binary32" && m.DecimalFidelity != "export_precision_only" {
		return bad()
	}
	if m.Format == "csv" && m.NullToken != "\\N" || m.Format == "ndjson" && m.NullToken != "" {
		return bad()
	}
	sources, targets := map[string]bool{}, map[string]bool{}
	pk, vectors := 0, 0
	for _, f := range m.Fields {
		if len(f.Source) == 0 || len(f.Source) > 128 || !utf8.ValidString(f.Source) || strings.ContainsAny(f.Source, "\x00\r\n") || sources[f.Source] || !importName(f.Target) || targets[strings.ToLower(f.Target)] {
			return bad()
		}
		sources[f.Source] = true
		targets[strings.ToLower(f.Target)] = true
		if f.PrimaryKey {
			pk++
			if f.Type != "int64" || f.Nullable {
				return bad()
			}
		}
		switch f.Type {
		case "int64":
			if f.Encoding != "integer" || f.Dimensions != 0 {
				return bad()
			}
		case "float64":
			if f.Encoding != "decimal" || f.Dimensions != 0 {
				return bad()
			}
		case "text":
			if f.Encoding != "utf8" || f.Dimensions != 0 {
				return bad()
			}
		case "blob":
			if f.Encoding != "base64" || f.Dimensions != 0 {
				return bad()
			}
		case "vector":
			vectors++
			if f.Dimensions < 1 || f.Dimensions > vector.MaxDimensions || f.Encoding != "pg_array" && f.Encoding != "json_array" || m.Format == "csv" && f.Encoding != "pg_array" {
				return bad()
			}
		default:
			return bad()
		}
	}
	if pk != 1 || vectors == 0 {
		return bad()
	}
	names := map[string]bool{strings.ToLower(m.Table): true}
	for _, x := range m.Indexes {
		if !importName(x.Name) || names[strings.ToLower(x.Name)] {
			return bad()
		}
		names[strings.ToLower(x.Name)] = true
		dims := 0
		for _, f := range m.Fields {
			if f.Target == x.Column && f.Type == "vector" {
				dims = f.Dimensions
			}
		}
		if dims == 0 {
			return bad()
		}
		if _, err := x.config(dims); err != nil {
			return bad()
		}
	}
	raw, err := json.Marshal(m)
	if err != nil || len(raw) > MaxImportManifestBytes {
		return bad()
	}
	return nil
}
func (m ImportManifest) hash() string { raw, _ := json.Marshal(m); return importHash(raw) }
func importHash(raw []byte) string    { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }
func (m ImportManifest) pkField() int {
	for i, f := range m.Fields {
		if f.PrimaryKey {
			return i
		}
	}
	return -1
}
func (x ImportIndex) config(dims int) (ann.Config, error) {
	if x.Method != ann.HNSW && x.Method != ann.IVFFlat {
		return ann.Config{}, fmt.Errorf("import index: explicit method required")
	}
	metric := vector.MetricKind(x.Metric)
	return (ann.Config{Method: x.Method, Dimensions: dims, Metric: metric, M: x.M, EFConstruction: x.EFConstruction, Lists: x.Lists, TrainingSamples: x.TrainingSamples, TrainingIterations: x.TrainingIterations, Seed: x.Seed}).Normalize()
}
func importQuote(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
func (m ImportManifest) createSQL() string {
	parts := make([]string, len(m.Fields))
	for i, f := range m.Fields {
		typ := map[string]string{"int64": "INTEGER", "float64": "REAL", "text": "TEXT", "blob": "BLOB"}[f.Type]
		if f.Type == "vector" {
			typ = fmt.Sprintf("VECTOR(%d)", f.Dimensions)
		}
		parts[i] = importQuote(f.Target) + " " + typ
		if f.PrimaryKey {
			parts[i] += " PRIMARY KEY"
		} else if !f.Nullable {
			parts[i] += " NOT NULL"
		}
	}
	return "CREATE TABLE " + importQuote(m.Table) + " (" + strings.Join(parts, ", ") + ")"
}
func (m ImportManifest) selectSQL() string {
	parts := make([]string, len(m.Fields))
	for i, f := range m.Fields {
		parts[i] = importQuote(f.Target)
	}
	return "SELECT " + strings.Join(parts, ", ") + " FROM " + importQuote(m.Table)
}
func (m ImportManifest) insertSQL() string {
	parts := make([]string, len(m.Fields))
	params := make([]string, len(m.Fields))
	for i, f := range m.Fields {
		parts[i] = importQuote(f.Target)
		params[i] = "?"
	}
	return "INSERT INTO " + importQuote(m.Table) + " (" + strings.Join(parts, ", ") + ") VALUES (" + strings.Join(params, ", ") + ")"
}
