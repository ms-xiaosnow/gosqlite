package postgres

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

func importRecordsManifest(format, encoding string, dimensions int) ImportManifest {
	nullToken := ""
	if format == "csv" {
		nullToken = `\N`
	}
	return ImportManifest{
		Format: format, NullToken: nullToken, RowOrder: "ascending",
		Fields: []ImportField{
			{Source: "id", Target: "id", Type: "int64", Encoding: "integer", PrimaryKey: true},
			{Source: "embedding", Target: "embedding", Type: "vector", Encoding: encoding, Dimensions: dimensions},
		},
	}
}

func newImportRecordsTestReader(t *testing.T, input string, m ImportManifest) *importRecordReader {
	t.Helper()
	r, err := newImportRecordReader(context.Background(), strings.NewReader(input), m, MaxImportRecordBytes, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func requireImportRecordBits(t *testing.T, value any, bits []uint32) {
	t.Helper()
	v, ok := value.(vector.Vector)
	if !ok {
		t.Fatalf("expected vector, got %T", value)
	}
	values := v.Values()
	if len(values) != len(bits) {
		t.Fatal("dimension changed")
	}
	for i, x := range values {
		if math.Float32bits(x) != bits[i] {
			t.Fatalf("component %d bits=%08x want %08x", i, math.Float32bits(x), bits[i])
		}
	}
}

func TestImportRecordsNDJSONExactTypes(t *testing.T) {
	m := importRecordsManifest("ndjson", "json_array", 4)
	m.Fields = append(m.Fields,
		ImportField{Source: "score", Type: "float64", Encoding: "decimal"},
		ImportField{Source: "label", Type: "text", Encoding: "utf8"},
		ImportField{Source: "payload", Type: "blob", Encoding: "base64"},
	)
	// The fourth coordinate is just above a binary32 midpoint but rounds to
	// that midpoint as binary64: a float64 intermediate would change its bits.
	input := "{\"payload\":\"AP8=\",\"label\":\"snowman ☃\\nnext\",\"score\":-0,\"embedding\":[-0,1.401298464324817e-45,-1.401298464324817e-45,1.0000000596046447753906250000000001],\"id\":9223372036854775807}\r\n"
	r := newImportRecordsTestReader(t, input, m)
	record, err := r.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if record.PK != math.MaxInt64 || !record.HasPK || record.StartOffset != 0 || record.EndOffset != int64(len(input)) || record.Digest != sha256.Sum256([]byte(input)) {
		t.Fatal("raw source identity or exact primary key changed")
	}
	requireImportRecordBits(t, record.Values[1], []uint32{0x80000000, 1, 0x80000001, 0x3f800001})
	if math.Float64bits(record.Values[2].(float64)) != 0x8000000000000000 || record.Values[3] != "snowman ☃\nnext" || !bytes.Equal(record.Values[4].([]byte), []byte{0, 255}) {
		t.Fatal("primitive mapping changed")
	}
	if record.DecodedBytes != int64(8+16+8+len("snowman ☃\nnext")+2) {
		t.Fatal("decoded byte accounting changed")
	}
	if _, err = r.Next(context.Background()); err != io.EOF {
		t.Fatal("missing EOF", err)
	}
}

func TestImportRecordsCSVExactOffsetsAndResume(t *testing.T) {
	m := importRecordsManifest("csv", "pg_array", 3)
	m.Fields = append(m.Fields, ImportField{Source: "label", Type: "text", Encoding: "utf8"})
	header := "id,embedding,label\r\n"
	first := "-9223372036854775808,\"{-0,1.40129846e-45,2}\",\"line one\r\nline \"\"two\"\"\"\r\n"
	last := "9223372036854775807,\"[1:3]={3,4,5}\",tail"
	r := newImportRecordsTestReader(t, header+first+last, m)
	if r.Offset() != int64(len(header)) || !reflect.DeepEqual(r.Header(), []string{"id", "embedding", "label"}) {
		t.Fatal("wrong initial header boundary")
	}
	savedHeader := r.Header()
	savedHeader[0] = "untrusted mutation"
	one, err := r.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if one.StartOffset != int64(len(header)) || one.EndOffset != int64(len(header+first)) || one.Digest != sha256.Sum256([]byte(first)) || one.PK != math.MinInt64 {
		t.Fatal("CSV offset, digest or exact int64 differs")
	}
	requireImportRecordBits(t, one.Values[1], []uint32{0x80000000, 1, 0x40000000})
	if one.Values[2] != "line one\nline \"two\"" {
		t.Fatal("encoding/csv multiline normalization changed")
	}
	resumed, err := newImportRecordReader(context.Background(), strings.NewReader(last), m, MaxImportRecordBytes, one.EndOffset, r.Header())
	if err != nil {
		t.Fatal(err)
	}
	two, err := resumed.Next(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if two.PK != math.MaxInt64 || two.StartOffset != one.EndOffset || two.EndOffset != int64(len(header+first+last)) || two.Digest != sha256.Sum256([]byte(last)) {
		t.Fatal("resumed CSV cursor changed")
	}
	originalTwo, err := r.Next(context.Background())
	if err != nil || !reflect.DeepEqual(two, originalTwo) {
		t.Fatal("resumed record differs from uninterrupted parse", err)
	}
	if _, err := resumed.Next(context.Background()); err != io.EOF {
		t.Fatal("missing resumed EOF", err)
	}
}

func TestImportRecordsExplicitNullPolicy(t *testing.T) {
	for _, format := range []string{"csv", "ndjson"} {
		t.Run(format, func(t *testing.T) {
			m := importRecordsManifest(format, "pg_array", 1)
			m.Fields[1].Nullable = true
			m.Fields = append(m.Fields,
				ImportField{Source: "text", Type: "text", Encoding: "utf8", Nullable: true},
				ImportField{Source: "blob", Type: "blob", Encoding: "base64", Nullable: true},
			)
			input := "id,embedding,text,blob\n1,\\N,\\N,\\N\n2,{0},,\n"
			if format == "ndjson" {
				input = "{\"id\":1,\"embedding\":null,\"text\":null,\"blob\":null}\n{\"id\":2,\"embedding\":\"{0}\",\"text\":\"\\\\N\",\"blob\":\"\"}\n"
			}
			r := newImportRecordsTestReader(t, input, m)
			record, err := r.Next(context.Background())
			if err != nil || !reflect.DeepEqual(record.Values, []any{int64(1), nil, nil, nil}) {
				t.Fatal("explicit null changed", err)
			}
			record, err = r.Next(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if record.Values[1] == nil || record.Values[2] == nil || record.Values[3] == nil || len(record.Values[3].([]byte)) != 0 {
				t.Fatal("empty text/blob became NULL")
			}
			if format == "ndjson" && record.Values[2] != `\N` {
				t.Fatal("JSON text literal was confused with CSV null token")
			}
		})
	}
}

func TestImportRecordsNDJSONSchemaIsTerminal(t *testing.T) {
	m := importRecordsManifest("ndjson", "json_array", 1)
	for _, input := range []string{
		`{"id":1,"embedding":[0],"extra":"private-marker"}`,
		`{"id":1,"id":2,"embedding":[0]}`,
		`{"id":1,"\u0069d":2,"embedding":[0]}`,
		`{"id":1,"id":2,"embedding":"\ud800"}`,
		`{"id":1}`,
		`{"embedding":[0]}`,
	} {
		r := newImportRecordsTestReader(t, input+"\n"+`{"id":3,"embedding":[1]}`+"\n", m)
		_, err := r.Next(context.Background())
		var rowError *ImportRowError
		if err == nil || errors.As(err, &rowError) || strings.Contains(err.Error(), "private-marker") {
			t.Fatal("schema mismatch was quarantinable or leaked content", err)
		}
		if _, nextErr := r.Next(context.Background()); nextErr != err {
			t.Fatal("schema mismatch did not stop subsequent records", nextErr)
		}
	}
}

func TestImportRecordsNDJSONBadRowsKeepSafeBoundaries(t *testing.T) {
	m := importRecordsManifest("ndjson", "json_array", 1)
	cases := []string{
		"", `[]`, `null`, `{"id":1,"embedding":[0]`,
		`{"id":1,"embedding":[0]} {"id":2,"embedding":[0]}`,
		`{"id":1,"embedding":[0],}`, `{"id":1,"embedding":[true]}`,
		`{"id":1,"embedding":[null]}`, `{"id":1,"embedding":["0"]}`,
		`{"id":1,"embedding":[[0]]}`, `{"id":1,"embedding":[]}`,
		`{"id":1,"embedding":[0,1]}`, `{"id":1,"embedding":[1e-99]}`,
		`{"id":1,"embedding":[1e99]}`, `{"id":1,"embedding":[NaN]}`,
		`{"id":1,"embedding":[Infinity]}`, `{"id":1,"embedding":"{0}"}`,
		`{"id":9223372036854775808,"embedding":[0]}`,
		`{"id":-9223372036854775809,"embedding":[0]}`,
		`{"id":1.0,"embedding":[0]}`, `{"id":1e0,"embedding":[0]}`,
		`{"id":"1","embedding":[0]}`, `{"id":null,"embedding":[0]}`,
		`{"id":true,"embedding":[0]}`, `{"id":1,"embedding":null}`,
		`{"id":1,"embedding":{"private-marker":0}}`,
		"{\"id\":1,\"embedding\":[\xff]}",
	}
	for _, bad := range cases {
		t.Run(bad, func(t *testing.T) {
			good := `{"id":2,"embedding":[-0]}` + "\n"
			r := newImportRecordsTestReader(t, bad+"\n"+good, m)
			record, err := r.Next(context.Background())
			var rowError *ImportRowError
			if !errors.As(err, &rowError) || record.Values != nil || record.StartOffset != 0 || record.EndOffset != int64(len(bad)+1) || record.Digest != sha256.Sum256([]byte(bad+"\n")) || strings.Contains(err.Error(), "private-marker") {
				t.Fatal("bad row escaped strict redacted framing", err)
			}
			record, err = r.Next(context.Background())
			if err != nil || record.PK != 2 || record.StartOffset != int64(len(bad)+1) {
				t.Fatal("did not resume at complete physical line", err)
			}
			requireImportRecordBits(t, record.Values[1], []uint32{0x80000000})
		})
	}
}

func TestImportRecordsPostgresArrayAndPrimitiveStrictness(t *testing.T) {
	m := importRecordsManifest("ndjson", "pg_array", 1)
	for _, bad := range []string{`{}`, `{NULL}`, `{"NULL"}`, `{{1}}`, `[0:0]={1}`, `[1:2]={1}`, `{1,}`, `{NaN}`, `{Infinity}`, `{1e99}`, `{1e-99}`, `{0x1p0}`, `{1_0}`, `{1} trailing`, `{private-marker}`} {
		// Marshal array text rather than confuse PostgreSQL and JSON escapes.
		var encoded strings.Builder
		encoded.WriteString(`{"id":1,"embedding":"`)
		for _, b := range []byte(bad) {
			if b == '\\' || b == '"' {
				encoded.WriteByte('\\')
			}
			encoded.WriteByte(b)
		}
		encoded.WriteString(`"}`)
		r := newImportRecordsTestReader(t, encoded.String(), m)
		record, err := r.Next(context.Background())
		var rowError *ImportRowError
		if !errors.As(err, &rowError) || rowError.Field != 2 || !record.HasPK || record.PK != 1 || strings.Contains(err.Error(), "private-marker") {
			t.Fatal("invalid PostgreSQL array accepted or PK/redaction lost", err)
		}
	}
	for _, test := range []struct{ typ, encoding, value string }{
		{"float64", "decimal", `"1.0"`}, {"float64", "decimal", `true`},
		{"float64", "decimal", `1e400`}, {"float64", "decimal", `1e-4000`},
		{"blob", "base64", `"AP9="`}, {"blob", "base64", `"AA"`},
		{"blob", "base64", `"AA==\n"`}, {"blob", "base64", `"_-=="`},
		{"text", "utf8", `"private-marker\u0000"`},
		{"text", "utf8", `"\ud800"`}, {"text", "utf8", `"\udc00"`},
		{"text", "utf8", `1`},
	} {
		m := importRecordsManifest("ndjson", "json_array", 1)
		m.Fields = append(m.Fields, ImportField{Source: "value", Type: test.typ, Encoding: test.encoding})
		r := newImportRecordsTestReader(t, `{"id":1,"embedding":[0],"value":`+test.value+`}`, m)
		_, err := r.Next(context.Background())
		var rowError *ImportRowError
		if !errors.As(err, &rowError) || strings.Contains(err.Error(), "private-marker") {
			t.Fatal("invalid primitive accepted or disclosed", test.typ, err)
		}
	}
}

func TestImportRecordsCSVSchemaAndQuotesStop(t *testing.T) {
	m := importRecordsManifest("csv", "pg_array", 1)
	for _, header := range []string{"", "embedding,id\n", "id,id\n", "id,extra\n", "id\n", "id,embedding,extra\n", "\"id,embedding\n"} {
		if _, err := newImportRecordReader(context.Background(), strings.NewReader(header), m, MaxImportRecordBytes, 0, nil); err == nil {
			t.Fatal("invalid CSV header accepted")
		}
	}
	for _, bad := range []string{"1,\"{0}\n", "1,{\"0\"}\n", "1,\"{0}\"junk\n", "1\n", "1,{0},extra\n", "\n", "1,\"\xff\n", "1,\xff,extra\n"} {
		r := newImportRecordsTestReader(t, "id,embedding\n"+bad, m)
		_, err := r.Next(context.Background())
		var rowError *ImportRowError
		if err == nil || errors.As(err, &rowError) {
			t.Fatal("malformed CSV schema or quotes was quarantinable", err)
		}
		if _, nextErr := r.Next(context.Background()); nextErr != err {
			t.Fatal("CSV malformed framing did not stop reader")
		}
	}
	for _, bad := range []string{"1,\n", "1,\\N\n", "+1,{0}\n", "01,{0}\n", "1,{}\n"} {
		r := newImportRecordsTestReader(t, "id,embedding\n"+bad+"2,{1}\n", m)
		_, err := r.Next(context.Background())
		var rowError *ImportRowError
		if !errors.As(err, &rowError) {
			t.Fatal("deterministic CSV value error not classified", err)
		}
		if record, err := r.Next(context.Background()); err != nil || record.PK != 2 {
			t.Fatal("valid CSV record following rejected value lost", err)
		}
	}
}

type importCountReader struct {
	reader io.Reader
	count  int
}

func (r *importCountReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.count += n
	return n, err
}

type importErrorReader struct{ err error }

func (r importErrorReader) Read([]byte) (int, error) { return 0, r.err }

type importCancelReader struct {
	cancel context.CancelFunc
}

func (r importCancelReader) Read(p []byte) (int, error) {
	p[0] = '{'
	r.cancel()
	return 1, nil
}

func TestImportRecordsBoundedFramingAndSourceErrors(t *testing.T) {
	for _, format := range []string{"csv", "ndjson"} {
		m := importRecordsManifest(format, "pg_array", 1)
		prefix := ""
		if format == "csv" {
			prefix = "id,embedding\n1,\""
		}
		input := &importCountReader{reader: strings.NewReader(prefix + strings.Repeat("x", 1<<20))}
		r, err := newImportRecordReader(context.Background(), input, m, 64, 0, nil)
		if err != nil {
			t.Fatal(err)
		}
		_, err = r.Next(context.Background())
		var rowError *ImportRowError
		if err == nil || errors.As(err, &rowError) || input.count > 128 {
			t.Fatal("oversize record was unbounded or skippable", format, input.count, err)
		}
		if _, nextErr := r.Next(context.Background()); nextErr != err {
			t.Fatal("record limit did not remain terminal")
		}
	}
	m := importRecordsManifest("ndjson", "json_array", 1)
	sourceErr := errors.New("memory source unavailable")
	r, err := newImportRecordReader(context.Background(), io.MultiReader(strings.NewReader(`{"id":1,"embedding":[0]}`), importErrorReader{sourceErr}), m, MaxImportRecordBytes, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if record, err := r.Next(context.Background()); !errors.Is(err, sourceErr) || record.Values != nil {
		t.Fatal("source read error became a complete record", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := newImportRecordReader(ctx, strings.NewReader(""), m, MaxImportRecordBytes, 0, nil); !errors.Is(err, context.Canceled) {
		t.Fatal("constructor ignored canceled context")
	}
	r = newImportRecordsTestReader(t, `{"id":1,"embedding":[0]}`, m)
	if _, err := r.Next(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal("Next ignored canceled context")
	}
	if _, err := r.Next(context.Background()); err != nil {
		t.Fatal("pre-read cancellation consumed source", err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	r, err = newImportRecordReader(context.Background(), importCancelReader{cancel}, m, MaxImportRecordBytes, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Next(ctx); !errors.Is(err, context.Canceled) {
		t.Fatal("mid-read cancellation ignored", err)
	}
	if _, err := r.Next(context.Background()); !errors.Is(err, context.Canceled) {
		t.Fatal("partially consumed canceled record was resumed unsafely", err)
	}
}

func TestImportRecordsConfigurationAndExactRecordLimit(t *testing.T) {
	m := importRecordsManifest("ndjson", "json_array", 1)
	input := `{"id":1,"embedding":[0]}` + "\n"
	for _, limit := range []int{len(input), len(input) - 1} {
		r, err := newImportRecordReader(context.Background(), strings.NewReader(input), m, limit, 0, nil)
		if err != nil {
			t.Fatal(err)
		}
		_, err = r.Next(context.Background())
		if (err == nil) != (limit == len(input)) {
			t.Fatal("record limit does not include exact newline bytes", limit, err)
		}
	}
	for _, limit := range []int{-1, 0, MaxImportRecordBytes + 1} {
		if _, err := newImportRecordReader(context.Background(), strings.NewReader(""), m, limit, 0, nil); err == nil {
			t.Fatal("invalid resource limit accepted")
		}
	}
	for _, mutate := range []func(*ImportManifest){
		func(m *ImportManifest) { m.Format = "sql" },
		func(m *ImportManifest) { m.Fields[0].PrimaryKey = false },
		func(m *ImportManifest) { m.Fields[0].Nullable = true },
		func(m *ImportManifest) { m.Fields[1].PrimaryKey = true },
		func(m *ImportManifest) { m.Fields[1].Dimensions = 0 },
		func(m *ImportManifest) { m.Fields[1].Dimensions = vector.MaxDimensions + 1 },
		func(m *ImportManifest) { m.Fields[1].Encoding = "infer" },
		func(m *ImportManifest) { m.Fields[1].Source = "id" },
	} {
		bad := importRecordsManifest("ndjson", "json_array", 1)
		mutate(&bad)
		if _, err := newImportRecordReader(context.Background(), strings.NewReader(""), bad, MaxImportRecordBytes, 0, nil); err == nil {
			t.Fatal("invalid parsing schema accepted")
		}
	}
	if _, err := newImportRecordReader(nil, strings.NewReader(""), m, MaxImportRecordBytes, 0, nil); err == nil {
		t.Fatal("nil context accepted")
	}
	if _, err := newImportRecordReader(context.Background(), nil, m, MaxImportRecordBytes, 0, nil); err == nil {
		t.Fatal("nil reader accepted")
	}
	if _, err := newImportRecordReader(context.Background(), strings.NewReader(""), m, MaxImportRecordBytes, -1, nil); err == nil {
		t.Fatal("negative offset accepted")
	}
	if _, err := newImportRecordReader(context.Background(), strings.NewReader(""), m, MaxImportRecordBytes, 0, []string{"id", "embedding"}); err == nil {
		t.Fatal("NDJSON header accepted")
	}
	csvM := importRecordsManifest("csv", "pg_array", 1)
	if _, err := newImportRecordReader(context.Background(), strings.NewReader(""), csvM, MaxImportRecordBytes, 20, nil); err == nil {
		t.Fatal("resumed CSV without header accepted")
	}
	csvM.NullToken = ""
	if _, err := newImportRecordReader(context.Background(), strings.NewReader("id,embedding\n"), csvM, MaxImportRecordBytes, 0, nil); err == nil {
		t.Fatal("implicit CSV null policy accepted")
	}
}

func TestImportRecordsCSVFramingMatchesStandard(t *testing.T) {
	m := importRecordsManifest("csv", "pg_array", 1)
	m.Fields = append(m.Fields, ImportField{Source: "text", Type: "text", Encoding: "utf8"})
	texts := []string{"", "a", "a,b", "a\"b", "\"", "\"\"", "\n", "\r", "\r\n", "a\nb\r\nc", " leading", "☃", "\n\"a,b\"\n"}
	var input strings.Builder
	header := "id,embedding,text\n"
	input.WriteString(header)
	var rawRecords []string
	var expected [][]string
	for i := 0; i < 128; i++ {
		var encoded strings.Builder
		writer := csv.NewWriter(&encoded)
		writer.UseCRLF = i%2 == 0
		fields := []string{strconv.Itoa(i), "{0}", texts[i%len(texts)] + texts[(i/len(texts))%len(texts)]}
		if err := writer.Write(fields); err != nil {
			t.Fatal(err)
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			t.Fatal(err)
		}
		raw := encoded.String()
		rawRecords = append(rawRecords, raw)
		input.WriteString(raw)
		// encoding/csv deliberately normalizes CRLF and strips certain CR
		// bytes on write; use its independent decode as the text contract.
		decoded, err := csv.NewReader(strings.NewReader(raw)).Read()
		if err != nil {
			t.Fatal(err)
		}
		expected = append(expected, decoded)
	}
	r := newImportRecordsTestReader(t, input.String(), m)
	offset := int64(len(header))
	for i, raw := range rawRecords {
		record, err := r.Next(context.Background())
		if err != nil {
			t.Fatalf("record %d: %v", i, err)
		}
		if record.StartOffset != offset || record.EndOffset != offset+int64(len(raw)) || record.Digest != sha256.Sum256([]byte(raw)) || record.Values[2] != expected[i][2] {
			t.Fatalf("record %d differs from independent CSV framing", i)
		}
		offset = record.EndOffset
	}
	if _, err := r.Next(context.Background()); err != io.EOF {
		t.Fatal("unexpected trailing CSV record", err)
	}
}

func TestImportRecordsSourceErrorsCannotMasqueradeAsRowsOrEOF(t *testing.T) {
	// These are memory-only Reader contract checks. No files, database, disk
	// fault, interruption, corruption, or recovery workflow is involved.
	m := importRecordsManifest("ndjson", "json_array", 1)
	for _, sourceErr := range []error{&ImportRowError{Code: "invalid_component", Field: 2}, fmt.Errorf("source read: %w", io.EOF)} {
		r, err := newImportRecordReader(context.Background(), importErrorReader{sourceErr}, m, MaxImportRecordBytes, 0, nil)
		if err != nil {
			t.Fatal(err)
		}
		_, err = r.Next(context.Background())
		var rowError *ImportRowError
		if err == nil || errors.As(err, &rowError) || errors.Is(err, io.EOF) {
			t.Fatal("source error became a row rejection or clean EOF", err)
		}
	}
}

// encoding/csv slices all fields from one record string. Keeping a tiny text
// field must not keep the large original numeric spellings after conversion.
// This is a pure-memory retained-heap check with a 4-record allowance versus
// 32 independent large records; it does not inspect or alter disk files.
func TestImportRecordsCSVTextDoesNotRetainNumericRecord(t *testing.T) {
	const dimensions, records = 1024, 32
	m := importRecordsManifest("csv", "pg_array", dimensions)
	m.Fields = append(m.Fields, ImportField{Source: "label", Target: "label", Type: "text", Encoding: "utf8"})
	token := "1." + strings.Repeat("0", 100)
	large := "{" + strings.TrimSuffix(strings.Repeat(token+",", dimensions), ",") + "}"
	var source strings.Builder
	source.WriteString("id,embedding,label\n")
	for i := 0; i < records; i++ {
		fmt.Fprintf(&source, "%d,\"%s\",ok\n", i, large)
	}
	input := source.String()
	reader := newImportRecordsTestReader(t, input, m)
	retained := make([]string, 0, records)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for i := 0; i < records; i++ {
		record, err := reader.Next(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if record.DecodedBytes != dimensions*4+8+2 {
			t.Fatal("decoded-byte charge", record.DecodedBytes)
		}
		label, ok := record.Values[2].(string)
		if !ok || label != "ok" {
			t.Fatal("text conversion")
		}
		retained = append(retained, label)
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	runtime.KeepAlive(retained)
	runtime.KeepAlive(reader)
	runtime.KeepAlive(input)
	runtime.KeepAlive(large)
	growth := int64(after.HeapAlloc) - int64(before.HeapAlloc)
	if growth > int64(4*len(large)) {
		t.Fatalf("tiny selected text retained large CSV numeric record backing: heap growth %d, bound %d", growth, 4*len(large))
	}
}
