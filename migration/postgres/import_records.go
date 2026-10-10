package postgres

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

// ImportRowError is a deterministic value error in one completely framed
// source record. Field is one-based in manifest order, or zero for the whole
// record. Its text never contains source values. Only errors of this type are
// candidates for explicitly requested quarantine. Schema, framing-resource,
// source I/O, and context errors are not row errors.
type ImportRowError struct {
	Code  string
	Field int
}

// Keep source errors outside the quarantine type hierarchy even if a custom
// Reader happens to return an ImportRowError itself. Is retains normal I/O and
// cancellation sentinel matching; deliberately do not expose Unwrap/As.
type importRecordSourceError struct{ cause error }

func (e *importRecordSourceError) Error() string { return "import records: source read failed" }
func (e *importRecordSourceError) Is(target error) bool {
	// Only an exact io.EOF from the Reader establishes a clean source end.
	return target != io.EOF && errors.Is(e.cause, target)
}

func (e *ImportRowError) Error() string {
	return fmt.Sprintf("import row: %s (field=%d)", e.Code, e.Field)
}

func importRowError(code string, field int) error {
	return &ImportRowError{Code: code, Field: field}
}

// importRecord contains values in manifest order. Its digest covers every raw
// source byte in the record, including original CRLFs and a final newline when
// present. Offsets are absolute source byte offsets, never buffered-read counts.
// Even when Next returns a row error, offsets and digest describe that record.
type importRecord struct {
	Values                 []any
	PK                     int64
	HasPK                  bool
	StartOffset, EndOffset int64
	Digest                 [32]byte
	DecodedBytes           int64
}

// importRecordReader owns one bounded input buffer and at most one bounded raw
// record. CSV quoting is framed before encoding/csv sees the record, preventing
// that package's otherwise unbounded multiline-record allocation. A record
// exceeding the configured byte limit is terminal: do not guess a resume point
// or drain an arbitrarily long malformed quoted field to quarantine it.
type importRecordReader struct {
	input      *bufio.Reader
	manifest   ImportManifest
	maxBytes   int
	offset     int64
	header     []string
	csvColumns []int
	fieldByKey map[string]int
	pkField    int
	terminal   error
}

func newImportRecordReader(ctx context.Context, input io.Reader, manifest ImportManifest, maxRecordBytes int, startOffset int64, header []string) (*importRecordReader, error) {
	if ctx == nil {
		return nil, fmt.Errorf("import records: nil context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if input == nil || maxRecordBytes < 1 || maxRecordBytes > MaxImportRecordBytes || startOffset < 0 {
		return nil, fmt.Errorf("import records: invalid input or resource limit")
	}
	if manifest.Format != "csv" && manifest.Format != "ndjson" {
		return nil, fmt.Errorf("import records: unsupported format")
	}
	if len(manifest.Fields) == 0 || len(manifest.Fields) > 32 {
		return nil, fmt.Errorf("import records: invalid field count")
	}
	r := &importRecordReader{
		input:    bufio.NewReaderSize(input, min(maxRecordBytes, 4096)),
		manifest: manifest, maxBytes: maxRecordBytes, offset: startOffset,
		fieldByKey: make(map[string]int, len(manifest.Fields)), pkField: -1,
	}
	// Copy schema slices so a caller cannot change decoding after construction.
	r.manifest.Fields = append([]ImportField(nil), manifest.Fields...)
	for i, field := range manifest.Fields {
		if field.Source == "" || len(field.Source) > 128 || !validImportText(field.Source) || strings.ContainsAny(field.Source, "\r\n") {
			return nil, fmt.Errorf("import schema: invalid source field (field=%d)", i+1)
		}
		if _, ok := r.fieldByKey[field.Source]; ok {
			return nil, fmt.Errorf("import schema: duplicate source field (field=%d)", i+1)
		}
		r.fieldByKey[field.Source] = i
		valid := false
		switch field.Type {
		case "int64":
			valid = field.Encoding == "integer" && field.Dimensions == 0
		case "float64":
			valid = field.Encoding == "decimal" && field.Dimensions == 0
		case "text":
			valid = field.Encoding == "utf8" && field.Dimensions == 0
		case "blob":
			valid = field.Encoding == "base64" && field.Dimensions == 0
		case "vector":
			valid = field.Dimensions >= 1 && field.Dimensions <= vector.MaxDimensions &&
				(field.Encoding == "pg_array" || (manifest.Format == "ndjson" && field.Encoding == "json_array"))
		}
		if !valid {
			return nil, fmt.Errorf("import schema: unsupported field mapping (field=%d)", i+1)
		}
		if field.PrimaryKey {
			if r.pkField != -1 || field.Type != "int64" || field.Nullable {
				return nil, fmt.Errorf("import schema: exactly one nonnullable int64 primary key required")
			}
			r.pkField = i
		}
	}
	if r.pkField < 0 {
		return nil, fmt.Errorf("import schema: exactly one nonnullable int64 primary key required")
	}
	if manifest.Format == "ndjson" {
		if len(header) != 0 {
			return nil, fmt.Errorf("import schema: NDJSON cannot have a CSV header")
		}
		return r, nil
	}
	// The chosen CSV null token applies to all mapped fields, including text.
	// A literal text value equal to \\N cannot be represented in this profile.
	if manifest.NullToken != `\N` {
		return nil, fmt.Errorf("import schema: CSV null token must be backslash N")
	}
	if startOffset == 0 {
		if len(header) != 0 {
			return nil, fmt.Errorf("import schema: initial CSV header must come from source")
		}
		raw, err := r.readFrame(ctx)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil, fmt.Errorf("import schema: CSV header required")
			}
			return nil, err
		}
		header, err = readImportCSV(raw, len(manifest.Fields))
		if err != nil {
			return nil, fmt.Errorf("import schema: invalid CSV header")
		}
	}
	if len(header) != len(manifest.Fields) {
		return nil, fmt.Errorf("import schema: CSV header field count differs")
	}
	r.header = append([]string(nil), header...)
	r.csvColumns = make([]int, len(header))
	seen := make([]bool, len(header))
	for column, key := range header {
		field, ok := r.fieldByKey[key]
		if !ok || seen[field] || field != column {
			return nil, fmt.Errorf("import schema: CSV header must exactly match manifest field order")
		}
		seen[field] = true
		r.csvColumns[column] = field
	}
	return r, nil
}

func (r *importRecordReader) Header() []string { return append([]string(nil), r.header...) }
func (r *importRecordReader) Offset() int64    { return r.offset }

func (r *importRecordReader) Next(ctx context.Context) (importRecord, error) {
	record := importRecord{StartOffset: r.offset, EndOffset: r.offset}
	if ctx == nil {
		return record, fmt.Errorf("import records: nil context")
	}
	if err := ctx.Err(); err != nil {
		return record, err
	}
	if r.terminal != nil {
		return record, r.terminal
	}
	raw, err := r.readFrame(ctx)
	record.EndOffset = r.offset
	if err != nil {
		if !errors.Is(err, io.EOF) {
			r.terminal = err
		}
		return record, err
	}
	record.Digest = sha256.Sum256(raw)
	var values []any
	if r.manifest.Format == "csv" {
		columns, csvErr := readImportCSV(raw, len(r.manifest.Fields))
		if csvErr != nil {
			// A quote error has no trustworthy logical-record boundary. Field
			// count changes violate the declared schema. Neither is skippable.
			err = fmt.Errorf("import schema: malformed CSV record or field count")
		} else if !utf8.Valid(raw) || bytes.IndexByte(raw, 0) >= 0 {
			err = importRowError("invalid_encoding", 0)
		} else {
			ordered := make([]string, len(columns))
			for column, field := range r.csvColumns {
				ordered[field] = columns[column]
			}
			values, err = r.decodeValues(ctx, ordered, nil, &record)
		}
	} else {
		var fields []json.RawMessage
		fields, err = r.readJSONObject(raw)
		if err == nil {
			if !utf8.Valid(raw) || bytes.IndexByte(raw, 0) >= 0 {
				err = importRowError("invalid_encoding", 0)
			} else {
				values, err = r.decodeValues(ctx, nil, fields, &record)
			}
		}
	}
	if err != nil {
		var rowErr *ImportRowError
		if !errors.As(err, &rowErr) {
			r.terminal = err
		}
		return record, err
	}
	record.Values = values
	return record, nil
}

func (r *importRecordReader) readFrame(ctx context.Context) ([]byte, error) {
	// States: beginning of CSV field, unquoted field, quoted field, just
	// after a quote. Only a quote at the start of a field opens quoting;
	// doubled quotes inside a quoted field do not close that field.
	const (
		fieldStart = iota
		unquoted
		quoted
		afterQuote
	)
	state := fieldStart
	raw := make([]byte, 0, min(r.maxBytes, 4096))
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		b, err := r.input.ReadByte()
		if err != nil {
			if err == io.EOF && len(raw) > 0 {
				return raw, nil
			}
			if err == io.EOF {
				return nil, io.EOF
			}
			return nil, &importRecordSourceError{cause: err}
		}
		if r.offset == math.MaxInt64 {
			return nil, fmt.Errorf("import records: source offset overflow")
		}
		r.offset++
		if len(raw) == r.maxBytes {
			return nil, fmt.Errorf("import records: record byte limit exceeded")
		}
		raw = append(raw, b)
		if r.manifest.Format == "ndjson" {
			if b == '\n' {
				return raw, nil
			}
			continue
		}
		switch state {
		case fieldStart:
			switch b {
			case '"':
				state = quoted
			case ',':
			case '\n':
				return raw, nil
			default:
				state = unquoted
			}
		case unquoted:
			switch b {
			case ',':
				state = fieldStart
			case '\n':
				return raw, nil
			}
		case quoted:
			if b == '"' {
				state = afterQuote
			}
		case afterQuote:
			switch b {
			case '"':
				state = quoted
			case ',':
				state = fieldStart
			case '\n':
				return raw, nil
			default:
				state = unquoted
			}
		}
	}
}

func readImportCSV(raw []byte, fields int) ([]string, error) {
	reader := csv.NewReader(bytes.NewReader(raw))
	reader.FieldsPerRecord = fields
	values, err := reader.Read()
	if err != nil {
		return nil, err
	}
	// Use encoding/csv's actual consumed offset as an independent check of
	// the bounded framer, including multiline fields and CRLF translation.
	if reader.InputOffset() != int64(len(raw)) {
		return nil, fmt.Errorf("import CSV: record boundary mismatch")
	}
	return values, nil
}

func (r *importRecordReader) readJSONObject(raw []byte) ([]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, importRowError("json_object_required", 0)
	}
	fields := make([]json.RawMessage, len(r.manifest.Fields))
	for decoder.More() {
		token, err = decoder.Token()
		if err != nil {
			return nil, importRowError("json_syntax", 0)
		}
		key, ok := token.(string)
		if !ok {
			return nil, importRowError("json_syntax", 0)
		}
		field, ok := r.fieldByKey[key]
		if !ok {
			return nil, fmt.Errorf("import schema: unknown NDJSON field")
		}
		if fields[field] != nil {
			return nil, fmt.Errorf("import schema: duplicate NDJSON field (field=%d)", field+1)
		}
		if err = decoder.Decode(&fields[field]); err != nil {
			return nil, importRowError("json_syntax", field+1)
		}
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim('}') {
		return nil, importRowError("json_syntax", 0)
	}
	var trailing json.RawMessage
	if err = decoder.Decode(&trailing); err != io.EOF {
		return nil, importRowError("json_trailing_value", 0)
	}
	for field, value := range fields {
		if value == nil {
			return nil, fmt.Errorf("import schema: missing NDJSON field (field=%d)", field+1)
		}
	}
	if err := checkJSONUnicodeEscapes(raw); err != nil {
		return nil, importRowError("invalid_encoding", 0)
	}
	return fields, nil
}

func (r *importRecordReader) decodeValues(ctx context.Context, csvFields []string, jsonFields []json.RawMessage, record *importRecord) ([]any, error) {
	values := make([]any, len(r.manifest.Fields))
	// Decode the primary key first so a later rejected value can retain a
	// bounded exact int64 identifier without storing any other row contents.
	for n := -1; n < len(values); n++ {
		field := n
		if n == -1 {
			field = r.pkField
		} else if n == r.pkField {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var value any
		var size int64
		var err error
		if csvFields != nil {
			value, size, err = r.decodeCSVValue(csvFields[field], field)
		} else {
			value, size, err = r.decodeJSONValue(jsonFields[field], field)
		}
		if err != nil {
			return nil, err
		}
		values[field] = value
		record.DecodedBytes += size
		if field == r.pkField {
			record.PK, record.HasPK = value.(int64), true
		}
	}
	return values, nil
}

func (r *importRecordReader) decodeCSVValue(text string, field int) (any, int64, error) {
	mapping := r.manifest.Fields[field]
	if text == r.manifest.NullToken {
		if !mapping.Nullable {
			return nil, 0, importRowError("null_not_allowed", field+1)
		}
		return nil, 1, nil
	}
	return decodeImportTextValue(text, mapping, field)
}

func (r *importRecordReader) decodeJSONValue(raw json.RawMessage, field int) (any, int64, error) {
	mapping := r.manifest.Fields[field]
	if bytes.Equal(raw, []byte("null")) {
		if !mapping.Nullable {
			return nil, 0, importRowError("null_not_allowed", field+1)
		}
		return nil, 1, nil
	}
	if mapping.Type == "vector" && mapping.Encoding == "json_array" {
		return decodeImportJSONArray(raw, mapping.Dimensions, field)
	}
	if mapping.Type == "int64" || mapping.Type == "float64" {
		return decodeImportTextValue(string(raw), mapping, field)
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return nil, 0, importRowError("json_string_required", field+1)
	}
	return decodeImportTextValue(text, mapping, field)
}

func decodeImportTextValue(text string, mapping ImportField, field int) (any, int64, error) {
	bad := func(code string) (any, int64, error) { return nil, 0, importRowError(code, field+1) }
	switch mapping.Type {
	case "int64":
		if !importIntegerToken(text) {
			return bad("invalid_integer")
		}
		value, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			return bad("integer_range")
		}
		return value, 8, nil
	case "float64":
		if len(text) > vector.MaxElementTextBytes || !importJSONNumber(text) {
			return bad("invalid_decimal")
		}
		value, err := strconv.ParseFloat(text, 64)
		if err != nil || math.IsInf(value, 0) || math.IsNaN(value) {
			return bad("nonfinite_decimal")
		}
		if value == 0 && nonzeroDecimalSignificand(text) {
			return bad("decimal_underflow")
		}
		return value, 8, nil
	case "text":
		if !validImportText(text) {
			return bad("invalid_encoding")
		}
		// encoding/csv fields share one record string. A tiny selected text
		// field must not retain large discarded numeric spellings in the batch.
		return strings.Clone(text), int64(len(text)), nil
	case "blob":
		// Strict RFC 4648 standard base64 only. Reject whitespace explicitly;
		// encoding/base64 otherwise silently ignores CR and LF.
		if strings.ContainsAny(text, "\r\n\t ") {
			return bad("invalid_base64")
		}
		value, err := base64.StdEncoding.Strict().DecodeString(text)
		if err != nil {
			return bad("invalid_base64")
		}
		return value, int64(len(value)), nil
	case "vector":
		value, err := parseConverterRealArray(text, mapping.Dimensions)
		if err != nil {
			var typed *ValueError
			if errors.As(err, &typed) {
				return bad(typed.Code)
			}
			return bad("component_underflow")
		}
		return value, int64(value.Dims()) * 4, nil
	default:
		return nil, 0, fmt.Errorf("import schema: unsupported mapped value")
	}
}

func decodeImportJSONArray(raw []byte, dims, field int) (any, int64, error) {
	bad := func(code string) (any, int64, error) { return nil, 0, importRowError(code, field+1) }
	if len(raw) > vector.MaxTextBytes {
		return bad("resource_limit")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	token, err := decoder.Token()
	if err != nil || token != json.Delim('[') {
		return bad("json_array_required")
	}
	values := make([]float32, 0, min(dims, 64))
	for decoder.More() {
		if len(values) >= dims {
			return bad("dimension_mismatch")
		}
		token, err = decoder.Token()
		if err != nil {
			return bad("json_syntax")
		}
		number, ok := token.(json.Number)
		if !ok {
			if token == nil {
				return bad("null_element")
			}
			return bad("invalid_component")
		}
		text := string(number)
		if len(text) > vector.MaxElementTextBytes {
			return bad("resource_limit")
		}
		value, err := strconv.ParseFloat(text, 32)
		if err != nil || math.IsInf(value, 0) || math.IsNaN(value) {
			return bad("nonfinite_component")
		}
		if value == 0 && nonzeroDecimalSignificand(text) {
			return bad("component_underflow")
		}
		values = append(values, float32(value))
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim(']') {
		return bad("json_syntax")
	}
	var trailing json.RawMessage
	if err = decoder.Decode(&trailing); err != io.EOF {
		return bad("json_trailing_value")
	}
	if len(values) != dims {
		return bad("dimension_mismatch")
	}
	value, err := vector.New(values)
	if err != nil {
		return bad("invalid_component")
	}
	return value, int64(len(values)) * 4, nil
}

func validImportText(text string) bool {
	return utf8.ValidString(text) && strings.IndexByte(text, 0) < 0
}

func importIntegerToken(text string) bool {
	if len(text) == 0 || len(text) > 20 {
		return false
	}
	i := 0
	if text[0] == '-' {
		i++
	}
	if i == len(text) {
		return false
	}
	if text[i] == '0' && i+1 != len(text) {
		return false
	}
	for ; i < len(text); i++ {
		if text[i] < '0' || text[i] > '9' {
			return false
		}
	}
	return true
}

func importJSONNumber(text string) bool {
	if len(text) == 0 {
		return false
	}
	// A bounded JSON scalar validator excludes NaN, hexadecimal, underscores,
	// leading/trailing whitespace, quoted values and extra JSON tokens.
	if text[0] != '-' && (text[0] < '0' || text[0] > '9') {
		return false
	}
	return json.Valid([]byte(text)) && !strings.ContainsAny(text, " \t\r\n")
}
