package postgres

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"io"
	"math"
	"os"
	"sort"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

const importAccepted = "gosqlite_migration_accepted"

type importAcceptedRow struct {
	Record int64 `json:"record"`
}

type ImportVectorStats struct {
	Field     string  `json:"field"`
	Nulls     int64   `json:"nulls"`
	NonNulls  int64   `json:"non_nulls"`
	ZeroNorms int64   `json:"zero_norms"`
	MinNorm   float64 `json:"min_norm"`
	MaxNorm   float64 `json:"max_norm"`
}
type ImportResult struct {
	FormatVersion           int                 `json:"format_version"`
	Status                  string              `json:"status"`
	DryRun                  bool                `json:"dry_run"`
	SourceRecords           int64               `json:"source_records"`
	Accepted                int64               `json:"accepted"`
	Rejected                int64               `json:"rejected"`
	CursorOffset            int64               `json:"cursor_offset"`
	CommittedBatches        int64               `json:"committed_batches"`
	SourceSHA256            string              `json:"source_sha256"`
	CanonicalSHA256         string              `json:"canonical_sha256"`
	DatabaseCanonicalSHA256 string              `json:"database_canonical_sha256,omitempty"`
	DecimalFidelity         string              `json:"decimal_fidelity"`
	Vectors                 []ImportVectorStats `json:"vectors"`
	RejectedPKs             []int64             `json:"rejected_pks"`
	MissingPKs              []int64             `json:"missing_pks"`
	RejectionsWithoutPK     int64               `json:"rejections_without_pk"`
	ExactDistances          string              `json:"exact_distances"`
	ANNRecall               string              `json:"ann_recall"`
}
type importHashes struct{ canonical, records, rejections hash.Hash }

func newImportHashes() *importHashes { return &importHashes{sha256.New(), sha256.New(), sha256.New()} }
func importSum(h hash.Hash) string   { return hex.EncodeToString(h.Sum(nil)) }
func importWriteUint(h hash.Hash, v uint64) {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], v)
	h.Write(b[:])
}
func importWriteBytes(h hash.Hash, b []byte) { importWriteUint(h, uint64(len(b))); h.Write(b) }
func hashImportRecord(h hash.Hash, n int64, r importRecord) {
	h.Write([]byte("R1"))
	importWriteUint(h, uint64(n))
	importWriteUint(h, uint64(r.StartOffset))
	importWriteUint(h, uint64(r.EndOffset))
	h.Write(r.Digest[:])
}
func hashImportRejection(h hash.Hash, r importRejection) {
	b, _ := json.Marshal(r)
	importWriteBytes(h, b)
}
func importCanonicalValues(m ImportManifest, values []any) ([]byte, error) {
	if len(values) != len(m.Fields) {
		return nil, fmt.Errorf("import verification: field count")
	}
	var b bytes.Buffer
	b.WriteString("GMI1")
	binary.Write(&b, binary.BigEndian, uint32(len(m.Fields)))
	for i, f := range m.Fields {
		b.WriteByte(byte(len(f.Type)))
		b.WriteString(f.Type)
		v := values[i]
		if v == nil {
			if !f.Nullable {
				return nil, fmt.Errorf("import verification: forbidden NULL")
			}
			b.WriteByte(0)
			continue
		}
		b.WriteByte(1)
		switch f.Type {
		case "int64":
			n, ok := v.(int64)
			if !ok {
				return nil, fmt.Errorf("import verification: integer storage class")
			}
			binary.Write(&b, binary.BigEndian, uint64(n))
		case "float64":
			n, ok := v.(float64)
			if !ok {
				return nil, fmt.Errorf("import verification: real storage class")
			}
			if math.IsNaN(n) || math.IsInf(n, 0) {
				return nil, fmt.Errorf("import verification: nonfinite real")
			}
			binary.Write(&b, binary.BigEndian, math.Float64bits(n))
		case "text":
			s, ok := v.(string)
			if !ok {
				return nil, fmt.Errorf("import verification: text storage class")
			}
			binary.Write(&b, binary.BigEndian, uint64(len(s)))
			b.WriteString(s)
		case "blob":
			raw, ok := v.([]byte)
			if !ok {
				return nil, fmt.Errorf("import verification: blob storage class")
			}
			binary.Write(&b, binary.BigEndian, uint64(len(raw)))
			b.Write(raw)
		case "vector":
			var value vector.Vector
			var err error
			switch x := v.(type) {
			case vector.Vector:
				value = x
			case []byte:
				value, err = vector.Decode(x)
			default:
				err = fmt.Errorf("noncanonical vector storage class")
			}
			if err != nil || value.RequireDims(f.Dimensions) != nil {
				return nil, fmt.Errorf("import verification: vector encoding or dimension")
			}
			binary.Write(&b, binary.BigEndian, uint32(value.Dims()))
			for _, x := range value.Values() {
				binary.Write(&b, binary.BigEndian, math.Float32bits(x))
			}
		default:
			return nil, fmt.Errorf("import verification: field type")
		}
	}
	return b.Bytes(), nil
}
func newImportResult(m ImportManifest) ImportResult {
	r := ImportResult{FormatVersion: ImportVersion, SourceSHA256: m.SourceSHA256, DecimalFidelity: m.DecimalFidelity, RejectedPKs: []int64{}, Vectors: []ImportVectorStats{}, ExactDistances: "not_run; separate from data fidelity", ANNRecall: "not_run; separate from data fidelity"}
	for _, f := range m.Fields {
		if f.Type == "vector" {
			r.Vectors = append(r.Vectors, ImportVectorStats{Field: f.Target})
		}
	}
	return r
}
func importAddStats(result *ImportResult, m ImportManifest, values []any) {
	k := 0
	for i, f := range m.Fields {
		if f.Type != "vector" {
			continue
		}
		s := &result.Vectors[k]
		k++
		if values[i] == nil {
			s.Nulls++
			continue
		}
		v := values[i].(vector.Vector)
		sum := 0.0
		for _, x := range v.Values() {
			sum += float64(x) * float64(x)
		}
		norm := math.Sqrt(sum)
		if s.NonNulls == 0 || norm < s.MinNorm {
			s.MinNorm = norm
		}
		if norm > s.MaxNorm {
			s.MaxNorm = norm
		}
		s.NonNulls++
		if norm == 0 {
			s.ZeroNorms++
		}
	}
}
func importRowIssue(m ImportManifest, rec importRecord, parseErr error, last **int64) error {
	if parseErr != nil {
		return parseErr
	}
	if !rec.HasPK {
		return fmt.Errorf("import schema: missing primary key")
	}
	if m.RowOrder == "ascending" && *last != nil && rec.PK <= **last {
		return &ImportRowError{Code: "pk_order", Field: m.pkField() + 1}
	}
	value := rec.PK
	*last = &value
	return nil
}
func importRejectionFor(n int64, rec importRecord, issue *ImportRowError) importRejection {
	r := importRejection{Record: n, Code: issue.Code, Field: issue.Field, Digest: hex.EncodeToString(rec.Digest[:])}
	if rec.HasPK {
		pk := rec.PK
		r.PK = &pk
	}
	return r
}
func importPKEqual(a, b *int64) bool { return a == nil && b == nil || a != nil && b != nil && *a == *b }

// verifyImportPrefix independently re-reads source bytes and database values.
// It retains only one bounded source record, point lookup, and batch metadata.
// The provenance table binds each accepted PK to its exact source ordinal.
func verifyImportPrefix(ctx context.Context, src *os.File, m ImportManifest, db *sqlite.DB, j importJob) (*importHashes, ImportResult, error) {
	hs := newImportHashes()
	result := newImportResult(m)
	if _, err := src.Seek(0, io.SeekStart); err != nil {
		return nil, result, err
	}
	reader, err := newImportRecordReader(ctx, src, m, MaxImportRecordBytes, 0, nil)
	if err != nil {
		return nil, result, err
	}
	headerOffset := reader.Offset()
	var last *int64
	var batch importBatch
	var nextBatch int64 = 1
	startCursor, startOffset := int64(0), headerOffset
	if j.Batch > 0 {
		ok, e := readImportJSON(ctx, db, importBatches, nextBatch, &batch)
		if e != nil || !ok {
			return nil, result, errors.Join(e, fmt.Errorf("import verification: missing batch"))
		}
	}
	dbHash := sha256.New()
	for n := int64(1); n <= j.Cursor; n++ {
		rec, parseErr := reader.Next(ctx)
		if parseErr == io.EOF {
			return nil, result, fmt.Errorf("import verification: early source end")
		}
		issue := importRowIssue(m, rec, parseErr, &last)
		var rowIssue *ImportRowError
		if issue != nil && !errors.As(issue, &rowIssue) {
			return nil, result, issue
		}
		var rejection importRejection
		rejected, e := readImportJSON(ctx, db, importRejections, n, &rejection)
		if e != nil {
			return nil, result, e
		}
		var provenance importAcceptedRow
		hasProvenance := false
		if issue == nil {
			hasProvenance, e = readImportJSON(ctx, db, importAccepted, rec.PK, &provenance)
			if e != nil {
				return nil, result, e
			}
			if hasProvenance && provenance.Record < n {
				rowIssue = &ImportRowError{Code: "duplicate_pk", Field: m.pkField() + 1}
				issue = rowIssue
			}
		}
		hashImportRecord(hs.records, n, rec)
		if rejected {
			if rowIssue == nil {
				return nil, result, fmt.Errorf("import verification: unjustified rejection at record %d", n)
			}
			expected := importRejectionFor(n, rec, rowIssue)
			if rejection.Record != n || rejection.Code != expected.Code || rejection.Field != expected.Field || rejection.Digest != expected.Digest || !importPKEqual(rejection.PK, expected.PK) {
				return nil, result, fmt.Errorf("import verification: rejection identity mismatch at record %d", n)
			}
			hashImportRejection(hs.rejections, rejection)
			result.Rejected++
			if rejection.PK == nil {
				result.RejectionsWithoutPK++
			} else {
				result.RejectedPKs = append(result.RejectedPKs, *rejection.PK)
			}
		} else {
			if issue != nil || !hasProvenance || provenance.Record != n {
				return nil, result, fmt.Errorf("import verification: accepted source/provenance mismatch at record %d", n)
			}
			rows, e := db.Query(ctx, m.selectSQL()+" WHERE "+importQuote(m.Fields[m.pkField()].Target)+" = ?", rec.PK)
			if e != nil {
				return nil, result, e
			}
			if !rows.Next() {
				e = errors.Join(rows.Err(), rows.Close())
				return nil, result, errors.Join(e, fmt.Errorf("import verification: missing accepted PK at record %d", n))
			}
			actual := rows.Values()
			if rows.Next() {
				rows.Close()
				return nil, result, fmt.Errorf("import verification: duplicate destination PK")
			}
			e = errors.Join(rows.Err(), rows.Close())
			if e != nil {
				return nil, result, e
			}
			wanted, e := importCanonicalValues(m, rec.Values)
			if e != nil {
				return nil, result, e
			}
			got, e := importCanonicalValues(m, actual)
			if e != nil {
				return nil, result, e
			}
			if !bytes.Equal(wanted, got) {
				return nil, result, fmt.Errorf("import verification: exact typed value mismatch at record %d", n)
			}
			importWriteBytes(hs.canonical, wanted)
			importWriteBytes(dbHash, got)
			result.Accepted++
			importAddStats(&result, m, rec.Values)
		}
		if n == batch.Cursor && nextBatch <= j.Batch {
			if batch.Sequence != nextBatch || batch.StartCursor != startCursor || batch.StartOffset != startOffset || batch.Offset != rec.EndOffset || batch.Accepted != result.Accepted || batch.Rejected != result.Rejected || batch.CanonicalHash != importSum(hs.canonical) || batch.RecordsHash != importSum(hs.records) || batch.RejectionsHash != importSum(hs.rejections) {
				return nil, result, fmt.Errorf("import verification: committed batch identity mismatch")
			}
			startCursor, startOffset = n, rec.EndOffset
			nextBatch++
			if nextBatch <= j.Batch {
				ok, e := readImportJSON(ctx, db, importBatches, nextBatch, &batch)
				if e != nil || !ok {
					return nil, result, errors.Join(e, fmt.Errorf("import verification: missing batch"))
				}
			}
		}
		result.SourceRecords = n
	}
	if reader.Offset() != j.Offset || result.Accepted != j.Accepted || result.Rejected != j.Rejected || nextBatch != j.Batch+1 || !importPKEqual(last, j.LastPK) || importSum(hs.canonical) != j.CanonicalHash || importSum(hs.records) != j.RecordsHash || importSum(hs.rejections) != j.RejectionsHash {
		return nil, result, fmt.Errorf("import verification: committed cursor or cumulative identity mismatch")
	}
	for _, check := range []struct {
		table string
		want  int64
	}{{m.Table, j.Accepted}, {importAccepted, j.Accepted}, {importJobs, 1}, {importBatches, j.Batch}, {importRejections, j.Rejected}} {
		n, e := countImportRows(ctx, db, "SELECT "+map[bool]string{true: importQuote(m.Fields[m.pkField()].Target), false: "id"}[check.table == m.Table]+" FROM "+importQuote(check.table), check.want)
		if e != nil || n != check.want {
			return nil, result, errors.Join(e, fmt.Errorf("import verification: destination or metadata count mismatch"))
		}
	}
	// A rejected duplicate can still have an accepted destination row. Report
	// the exact set of known rejected PKs that is genuinely absent separately.
	result.MissingPKs = []int64{}
	checkedMissing := map[int64]bool{}
	for _, pk := range result.RejectedPKs {
		if checkedMissing[pk] {
			continue
		}
		checkedMissing[pk] = true
		var accepted importAcceptedRow
		exists, err := readImportJSON(ctx, db, importAccepted, pk, &accepted)
		if err != nil {
			return nil, result, err
		}
		if !exists {
			result.MissingPKs = append(result.MissingPKs, pk)
		}
	}
	sort.Slice(result.MissingPKs, func(i, j int) bool { return result.MissingPKs[i] < result.MissingPKs[j] })
	result.CursorOffset = j.Offset
	result.CommittedBatches = j.Batch
	result.CanonicalSHA256 = importSum(hs.canonical)
	result.DatabaseCanonicalSHA256 = importSum(dbHash)
	result.Status = j.Status
	return hs, result, nil
}

// VerifyImport performs an exhaustive source/destination comparison without
// changing the database or promoting its persisted completion status.
func VerifyImport(ctx context.Context, input, output string, m ImportManifest) (ImportResult, error) {
	return verifyImport(ctx, input, output, m, nil)
}

// additional runs after full fidelity/index checks and before the final source
// recheck, on the same native read snapshot. It never publishes job status.
func verifyImport(ctx context.Context, input, output string, m ImportManifest, additional func(*os.File, *sqlite.DB, ImportResult) error) (result ImportResult, resultErr error) {
	defer func() {
		if resultErr != nil && result.Status != "" {
			result.Status = "verification_failed"
		}
	}()
	if ctx == nil {
		return result, fmt.Errorf("import: nil context")
	}
	if err := m.Validate(); err != nil {
		return result, err
	}
	source, err := openImportSource(ctx, input, m)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, source.file.Close()) }()
	db, err := sqlite.OpenContext(ctx, output, sqlite.Options{ReadOnly: true, NoCreate: true})
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, db.Close()) }()
	// Keep all schema, metadata and row checks on one native read snapshot.
	snapshot, err := db.Begin(ctx, sqlite.TxOptions{ReadOnly: true})
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, snapshot.Rollback()) }()
	var job importJob
	ok, err := readImportJSON(ctx, db, importJobs, 1, &job)
	if err != nil || !ok {
		return result, errors.Join(err, fmt.Errorf("import verification: no committed job"))
	}
	if err = job.validate(m, job.Policy); err != nil {
		return result, err
	}
	if job.Policy != "stop" && job.Policy != "quarantine" {
		return result, fmt.Errorf("import verification: unknown rejection policy")
	}
	if job.Status == "importing" {
		return result, fmt.Errorf("import verification: data phase incomplete")
	}
	fingerprint, err := importSchemaHash(ctx, db)
	if err != nil {
		return result, err
	}
	if fingerprint != job.SchemaHash {
		return result, fmt.Errorf("import verification: schema identity mismatch")
	}
	_, result, err = verifyImportPrefix(ctx, source.file, m, db, job)
	if err != nil {
		return result, err
	}
	if err = source.recheck(ctx, m); err != nil {
		return result, err
	}
	if m.ExpectedRows != nil && result.SourceRecords != *m.ExpectedRows {
		return result, fmt.Errorf("import verification: expected source count mismatch")
	}
	if job.IndexesDone != len(m.Indexes) {
		return result, fmt.Errorf("import verification: requested indexes pending")
	}
	for _, x := range m.Indexes {
		if err = checkImportIndex(ctx, db, m, x); err != nil {
			return result, err
		}
	}
	if additional != nil {
		if err = additional(source.file, db, result); err != nil {
			return result, err
		}
		if err = source.recheck(ctx, m); err != nil {
			return result, err
		}
	}
	if result.Rejected == 0 {
		result.Status = "complete_verified"
	} else {
		result.Status = "complete_with_rejections"
	}
	return result, nil
}
