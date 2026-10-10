package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// SQLTarget distinguishes parser-checked local SQL from review-only PostgreSQL
// text. The converter has no database connection or statement execution path.
type SQLTarget string

const (
	TargetLocal    SQLTarget = "local"
	TargetPostgres SQLTarget = "postgres"
)

// TableMapping explicitly chooses a destination for a catalog relation. Local
// TargetSchema must be empty; PostgreSQL requires an explicit target schema.
type TableMapping struct {
	RelationOID  uint32 `json:"relation_oid"`
	TargetName   string `json:"target_name"`
	TargetSchema string `json:"target_schema,omitempty"`
}
type ConvertOptions struct {
	Target                       SQLTarget
	Tables                       []TableMapping
	Selected                     []Selection
	SourceProfile, TargetProfile string
	// VectorSchema is the reviewed intended pgvector namespace. Supplying it
	// does not prove the extension is installed or authorize remote execution.
	VectorSchema string
	// Only ordinary strings with standard_conforming_strings=on are supported.
	StandardConformingStrings bool
	Limits                    SQLLimits
}
type SQLEdit struct {
	Span        SQLSpan `json:"span"`
	Replacement string  `json:"replacement"`
}
type SQLFinding struct {
	Code    string  `json:"code"`
	Span    SQLSpan `json:"span"`
	Message string  `json:"message"`
}
type BindTransform struct {
	Parameter   int       `json:"parameter"`
	SourceKind  string    `json:"source_kind"`
	Target      SQLTarget `json:"target"`
	Dimensions  int       `json:"dimensions,omitempty"`
	Action      string    `json:"action"`
	ScalarOID   uint32    `json:"scalar_oid,omitempty"`
	RejectNull  bool      `json:"reject_null,omitempty"`
	IntegerBits int       `json:"integer_bits,omitempty"`
	NonNegative bool      `json:"nonnegative,omitempty"`
}

// ConvertValue applies a reported real[] bind conversion. Accepted inputs are
// NULL, PostgreSQL array text, or []float32. Local output is canonical GSV v1;
// PostgreSQL output is vector text. Identity binds validate source scalar type,
// width, nullability and nonnegative LIMIT constraints before returning the value.
// It does not normalize, pad, truncate, or accept float64 arrays as real[] values.
func (b BindTransform) ConvertValue(value any) (any, error) {
	if b.RejectNull && value == nil {
		return nil, fmt.Errorf("convert bind: non-NULL value required")
	}
	if b.Action == "identity" {
		if value != nil {
			switch b.SourceKind {
			case "integer":
				if _, ok := bindInteger(value); !ok {
					return nil, fmt.Errorf("convert bind: integral scalar required")
				}
			case "bool":
				if _, ok := value.(bool); !ok {
					return nil, fmt.Errorf("convert bind: boolean scalar required")
				}
			case "text":
				v, ok := value.(string)
				if !ok || !utf8.ValidString(v) || strings.IndexByte(v, 0) >= 0 {
					return nil, fmt.Errorf("convert bind: valid PostgreSQL text scalar required")
				}
			case "real":
				valid := false
				switch v := value.(type) {
				case float32:
					valid = !math.IsNaN(float64(v)) && !math.IsInf(float64(v), 0)
				case float64:
					valid = b.ScalarOID == 701 && !math.IsNaN(v) && !math.IsInf(v, 0)
				}
				if !valid {
					return nil, fmt.Errorf("convert bind: finite source-width real scalar required")
				}
			}
		}
		if value != nil && (b.IntegerBits != 0 || b.NonNegative) {
			n, ok := bindInteger(value)
			if !ok {
				return nil, fmt.Errorf("convert bind: integral scalar required")
			}
			if b.IntegerBits != 0 && b.IntegerBits != 16 && b.IntegerBits != 32 && b.IntegerBits != 64 {
				return nil, fmt.Errorf("convert bind: invalid integer width")
			}
			if b.IntegerBits == 16 && (n < -32768 || n > 32767) || b.IntegerBits == 32 && (n < -2147483648 || n > 2147483647) || b.NonNegative && n < 0 {
				return nil, fmt.Errorf("convert bind: integer constraint violation")
			}
		}
		return value, nil
	}
	if b.Action != "real_array_to_vector" || (b.Target != TargetLocal && b.Target != TargetPostgres) || b.Dimensions < 1 || b.Dimensions > vector.MaxDimensions {
		return nil, fmt.Errorf("convert bind: invalid transformation")
	}
	if value == nil {
		return nil, nil
	}
	var v vector.Vector
	var err error
	switch x := value.(type) {
	case string:
		v, err = parseConverterRealArray(x, b.Dimensions)
	case []float32:
		v, err = vector.New(x)
	default:
		return nil, fmt.Errorf("convert bind: expected real array text or []float32")
	}
	if err != nil {
		return nil, fmt.Errorf("convert bind: invalid real array")
	}
	if err = v.RequireDims(b.Dimensions); err != nil {
		return nil, fmt.Errorf("convert bind: dimension mismatch")
	}
	if b.Target == TargetLocal {
		return v.Value()
	}
	return v.String(), nil
}

type QuerySetting struct {
	Index       string `json:"index"`
	Name        string `json:"name"`
	Value       int    `json:"value"`
	Application string `json:"application"`
	// PostgreSQL SQL is a separate review-only snippet. It assumes an existing
	// application transaction; the converter never adds BEGIN/COMMIT wrappers.
	ReviewSQL string `json:"review_sql,omitempty"`
}
type ConvertedStatement struct {
	Ordinal      int             `json:"ordinal"`
	Kind         string          `json:"kind"`
	Span         SQLSpan         `json:"span"`
	Status       string          `json:"status"`
	OriginalSQL  string          `json:"original_sql"`
	ConvertedSQL string          `json:"converted_sql,omitempty"`
	Edits        []SQLEdit       `json:"edits"`
	Bindings     []BindTransform `json:"bindings"`
	Settings     []QuerySetting  `json:"settings"`
	Findings     []SQLFinding    `json:"findings"`
}
type Conversion struct {
	// UnchangedSQL preserves an input containing only trivia/empty statements.
	UnchangedSQL  string    `json:"unchanged_sql,omitempty"`
	FormatVersion int       `json:"format_version"`
	Target        SQLTarget `json:"target"`
	SourceProfile string    `json:"source_profile"`
	TargetProfile string    `json:"target_profile"`
	SourceSHA256  string    `json:"source_sha256"`
	CatalogSHA256 string    `json:"catalog_sha256"`
	// TargetSHA256 is absent unless every statement converted successfully.
	TargetSHA256 string               `json:"target_sha256,omitempty"`
	Status       string               `json:"status"`
	Validation   string               `json:"validation"`
	Statements   []ConvertedStatement `json:"statements"`
}

// SQL returns a complete converted script only. Unsupported original SQL is
// retained in the report as evidence and can never leak into this output.
// PostgreSQL results are always review-only text, never execution approval.
func (r Conversion) SQL() (string, error) {
	if r.Status != "converted" {
		return "", fmt.Errorf("convert: script contains unsupported statements; inspect findings")
	}
	if len(r.Statements) == 0 {
		return r.UnchangedSQL, nil
	}
	var out strings.Builder
	for _, s := range r.Statements {
		if s.Status != "converted" || s.ConvertedSQL == "" {
			return "", fmt.Errorf("convert: incomplete statement")
		}
		out.WriteString(s.ConvertedSQL)
	}
	return out.String(), nil
}
func RenderConversionJSON(w io.Writer, r Conversion) error {
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	return e.Encode(r)
}
func sqlHash(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }

// Convert tokenizes PostgreSQL source independently, parses a bounded subset,
// binds it to the supplied catalog, and rewrites only identified source spans.
// Invalid lexical input returns an error; unsupported complete statements are
// returned unchanged as evidence with no converted SQL. The catalog is supplied
// evidence, not proof of server state, search_path, or extension installation.
func Convert(ctx context.Context, source string, catalog CatalogSnapshot, options ConvertOptions) (Conversion, error) {
	var result Conversion
	if ctx == nil {
		return result, fmt.Errorf("convert: nil context")
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	if options.Target != TargetLocal && options.Target != TargetPostgres {
		return result, fmt.Errorf("convert: explicit target required")
	}
	if !options.StandardConformingStrings {
		return result, fmt.Errorf("convert: requires standard_conforming_strings=on source contract")
	}
	if options.Target == TargetPostgres && !validName(options.VectorSchema) {
		return result, fmt.Errorf("convert: explicit pgvector schema required")
	}
	limits, err := options.Limits.normalized()
	if err != nil {
		return result, err
	}
	tokens, segments, err := pgScan(source, limits)
	if err != nil {
		return result, err
	}
	inspection, err := Inspect(ctx, catalog, InspectOptions{SourceProfile: options.SourceProfile, TargetProfile: options.TargetProfile, Selected: options.Selected})
	if err != nil {
		return result, err
	}
	cv := converter{source: source, tokens: tokens, opts: options, catalog: catalog, selected: map[string]ColumnReport{}, tables: map[uint32]TableMapping{}, types: map[uint32]Type{}}
	for _, t := range catalog.Types {
		cv.types[t.OID] = t
	}
	for _, c := range inspection.Columns {
		if c.Selected {
			cv.selected[conversionColumnKey(c.RelationOID, c.AttNum)] = c
		}
	}
	if len(options.Tables) > len(catalog.Relations) {
		return result, fmt.Errorf("convert: mapping limit exceeded")
	}
	destinations := map[string]bool{}
	for _, m := range options.Tables {
		if !validName(m.TargetName) || options.Target == TargetLocal && m.TargetSchema != "" || options.Target == TargetPostgres && !validName(m.TargetSchema) || cv.tables[m.RelationOID].RelationOID != 0 {
			return result, fmt.Errorf("convert: invalid or duplicate table mapping")
		}
		if options.Target == TargetLocal && (strings.HasPrefix(strings.ToLower(m.TargetName), "sqlite_") || strings.HasPrefix(strings.ToLower(m.TargetName), "__gosqlite_")) {
			return result, fmt.Errorf("convert: reserved local target name")
		}
		present := false
		for _, rel := range catalog.Relations {
			if rel.OID == m.RelationOID {
				present = true
			}
		}
		if !present {
			return result, fmt.Errorf("convert: mapping relation absent")
		}
		identity := objectKey(m.TargetSchema, m.TargetName)
		if options.Target == TargetLocal {
			identity = strings.ToLower(identity)
		}
		if destinations[identity] {
			return result, fmt.Errorf("convert: destination name collision")
		}
		destinations[identity] = true
		cv.tables[m.RelationOID] = m
	}
	result = Conversion{FormatVersion: 1, Target: options.Target, SourceProfile: SourceProfile, TargetProfile: TargetProfile, SourceSHA256: sqlHash(source), CatalogSHA256: inspection.CatalogSHA256, Status: "converted", Validation: "local_parser_checked", Statements: []ConvertedStatement{}}
	if len(segments) == 0 {
		result.UnchangedSQL = source
	}
	if options.Target == TargetPostgres {
		result.Validation = "postgresql_source_review_only_not_executed"
	}
	for i, segment := range segments {
		if err = ctx.Err(); err != nil {
			return Conversion{}, err
		}
		st := ConvertedStatement{Ordinal: i + 1, Span: segment.Span, Status: "converted", OriginalSQL: source[segment.Span.Start:segment.Span.End], Edits: []SQLEdit{}, Bindings: []BindTransform{}, Settings: []QuerySetting{}, Findings: []SQLFinding{}}
		ast, parseErr := pgParseStatement(source, tokens, segment, limits)
		if parseErr == nil && options.Target == TargetLocal {
			parseErr = localTriviaCompatibility(source, tokens[segment.First:segment.Last])
		}
		if parseErr == nil {
			st.Kind = ast.Kind
			bound := statementBinder{cv: &cv, result: &st, parameters: map[int]BindTransform{}, aliases: map[string]boundType{}}
			parseErr = bound.statement(ast)
		}
		if parseErr == nil {
			st.ConvertedSQL, parseErr = applySQLEdits(source, st.Span, st.Edits)
			if parseErr == nil && options.Target == TargetLocal {
				if _, e := sqlast.Parse(st.ConvertedSQL); e != nil {
					parseErr = conversionError("local_parser_rejected", st.Span, "Emitted syntax is outside the local parser subset.")
				}
			}
		}
		if parseErr != nil {
			st.Status = "unsupported"
			result.Status = "unsupported"
			st.ConvertedSQL = ""
			st.Edits = []SQLEdit{}
			st.Bindings = []BindTransform{}
			st.Settings = []QuerySetting{}
			span, code, message := st.Span, "unsupported_statement", "Statement is outside the supported conversion subset."
			if p, ok := parseErr.(*SQLParseError); ok {
				span, code = p.Span, p.Code
			}
			if p, ok := parseErr.(*conversionFailure); ok {
				span, code, message = p.span, p.code, p.message
			}
			st.Findings = append(st.Findings, SQLFinding{code, span, message})
		}
		sort.Slice(st.Edits, func(i, j int) bool { return st.Edits[i].Span.Start < st.Edits[j].Span.Start })
		sort.Slice(st.Findings, func(i, j int) bool {
			if st.Findings[i].Span.Start != st.Findings[j].Span.Start {
				return st.Findings[i].Span.Start < st.Findings[j].Span.Start
			}
			return st.Findings[i].Code < st.Findings[j].Code
		})
		result.Statements = append(result.Statements, st)
	}
	if result.Status == "converted" {
		sql, e := result.SQL()
		if e != nil {
			return Conversion{}, e
		}
		result.TargetSHA256 = sqlHash(sql)
	}
	return result, nil
}

type converter struct {
	source   string
	tokens   []pgToken
	opts     ConvertOptions
	catalog  CatalogSnapshot
	selected map[string]ColumnReport
	tables   map[uint32]TableMapping
	types    map[uint32]Type
}
type conversionFailure struct {
	code    string
	span    SQLSpan
	message string
}

func (e *conversionFailure) Error() string {
	return fmt.Sprintf("convert: %s at bytes %d..%d", e.code, e.span.Start, e.span.End)
}
func conversionError(code string, span SQLSpan, message string) error {
	return &conversionFailure{code, span, message}
}
func tokenSpan(t pgToken) SQLSpan { return SQLSpan{Start: t.Start, End: t.End} }

func applySQLEdits(source string, span SQLSpan, edits []SQLEdit) (string, error) {
	sorted := append([]SQLEdit(nil), edits...)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Span.Start != sorted[j].Span.Start {
			return sorted[i].Span.Start < sorted[j].Span.Start
		}
		return sorted[i].Span.End < sorted[j].Span.End
	})
	if span.Start < 0 || span.End < span.Start || span.End > len(source) {
		return "", fmt.Errorf("convert: invalid source span")
	}
	size := span.End - span.Start
	end := span.Start
	for _, edit := range sorted {
		if edit.Span.Start < end || edit.Span.End < edit.Span.Start || edit.Span.End > span.End {
			return "", fmt.Errorf("convert: overlapping source edits")
		}
		size += len(edit.Replacement) - (edit.Span.End - edit.Span.Start)
		if size > 4<<20 {
			return "", fmt.Errorf("convert: output byte limit")
		}
		end = edit.Span.End
	}
	var out strings.Builder
	out.Grow(size)
	end = span.Start
	for _, edit := range sorted {
		out.WriteString(source[end:edit.Span.Start])
		out.WriteString(edit.Replacement)
		end = edit.Span.End
	}
	out.WriteString(source[end:span.End])
	return out.String(), nil
}

func conversionColumnKey(oid uint32, att int) string { return fmt.Sprintf("%d/%d", oid, att) }

// A parser-success check cannot detect a comment that the two dialects end at
// different positions. Reject these forms rather than changing source comments
// or letting text hidden in PostgreSQL become executable local syntax.
func localTriviaCompatibility(source string, tokens []pgToken) error {
	for _, t := range tokens {
		if t.Kind != pgComment {
			continue
		}
		raw := source[t.Start:t.End]
		if strings.HasPrefix(raw, "/*") && strings.Contains(raw[2:], "/*") {
			return conversionError("local_nested_comment_unsupported", tokenSpan(t), "Nested PostgreSQL comments cannot be retained safely by the local non-nested lexer.")
		}
		if strings.HasPrefix(raw, "--") && t.End < len(source) && source[t.End] == '\r' && (t.End+1 == len(source) || source[t.End+1] != '\n') {
			return conversionError("local_cr_comment_unsupported", tokenSpan(t), "A bare-CR PostgreSQL line-comment terminator differs from local LF semantics.")
		}
	}
	return nil
}

func bindInteger(value any) (int64, bool) {
	switch n := value.(type) {
	case int:
		return int64(n), true
	case int8:
		return int64(n), true
	case int16:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case uint:
		if uint64(n) <= math.MaxInt64 {
			return int64(n), true
		}
	case uint8:
		return int64(n), true
	case uint16:
		return int64(n), true
	case uint32:
		return int64(n), true
	case uint64:
		if n <= math.MaxInt64 {
			return int64(n), true
		}
	}
	return 0, false
}
