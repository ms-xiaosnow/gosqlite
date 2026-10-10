package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/ms-xiaosnow/gosqlite/internal/pager"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// ImportOptions defaults to read-only validation. Apply is the only authority
// for local writes. Resume requires an intact matching committed job. StopAfter
// returns at an ordinary committed boundary; it is not crash simulation.
type ImportOptions struct {
	Apply            bool
	Resume           bool
	OnError          string
	BatchRows        int
	BatchBytes       int64
	StopAfterBatches int64
}

func (o ImportOptions) normalized() (ImportOptions, error) {
	if o.OnError == "" {
		o.OnError = "stop"
	}
	if o.BatchRows == 0 {
		o.BatchRows = 500
	}
	if o.BatchBytes == 0 {
		o.BatchBytes = 8 << 20
	}
	if o.OnError != "stop" && o.OnError != "quarantine" || o.BatchRows < 1 || o.BatchRows > 5000 || o.BatchBytes < MaxImportRecordBytes || o.BatchBytes > 16<<20 || o.StopAfterBatches < 0 || o.Resume && !o.Apply {
		return o, fmt.Errorf("import: invalid options or resume without apply")
	}
	return o, nil
}

type importSource struct {
	file *os.File
	path string
	info os.FileInfo
}

func openImportSource(ctx context.Context, path string, m ImportManifest) (*importSource, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if path == "" || path == "-" {
		return nil, fmt.Errorf("import: a regular immutable source file is required; stdin unsupported")
	}
	// Reject named pipes/devices before opening: opening a FIFO can block
	// before a handle exists and cannot be canceled through this API. Namespace
	// immutability remains required between this check and the owned open.
	before, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !before.Mode().IsRegular() || before.Size() != m.SourceSize || before.Size() > MaxImportInputBytes {
		return nil, fmt.Errorf("import source: regular-file size required")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if !os.SameFile(before, info) {
		f.Close()
		return nil, fmt.Errorf("import source: identity changed before open")
	}
	s := &importSource{f, path, info}
	if err = s.recheck(ctx, m); err != nil {
		f.Close()
		return nil, err
	}
	return s, nil
}
func (s *importSource) recheck(ctx context.Context, m ImportManifest) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	before, err := s.file.Stat()
	if err != nil {
		return err
	}
	named, err := os.Stat(s.path)
	if err != nil {
		return err
	}
	if !before.Mode().IsRegular() || !os.SameFile(s.info, before) || !os.SameFile(before, named) || before.Size() != m.SourceSize || before.Size() > MaxImportInputBytes {
		return fmt.Errorf("import source: regular-file identity or size mismatch")
	}
	if _, err = s.file.Seek(0, io.SeekStart); err != nil {
		return err
	}
	h := sha256.New()
	buf := make([]byte, 64<<10)
	var total int64
	for {
		if err = ctx.Err(); err != nil {
			return err
		}
		n, e := s.file.Read(buf)
		total += int64(n)
		if total > m.SourceSize {
			return fmt.Errorf("import source: size changed")
		}
		if n > 0 {
			h.Write(buf[:n])
		}
		if e == io.EOF {
			break
		}
		if e != nil {
			return e
		}
	}
	after, err := s.file.Stat()
	if err != nil {
		return err
	}
	if !os.SameFile(before, after) || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) || total != m.SourceSize || hex.EncodeToString(h.Sum(nil)) != m.SourceSHA256 {
		return fmt.Errorf("import source: identity or SHA-256 mismatch")
	}
	_, err = s.file.Seek(0, io.SeekStart)
	return err
}

// reserveImportDestination only initializes a freshly O_EXCL-reserved file.
// The reserve descriptor is closed before native locking starts, important for
// process-scoped Darwin locks. Source/destination namespace immutability and
// exclusive ownership are required throughout; hostile path races are outside
// this importer. No existing database is truncated, renamed, or replaced.
func reserveImportDestination(ctx context.Context, path string) (*sqlite.DB, error) {
	if path == "" {
		return nil, fmt.Errorf("import apply: output required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return nil, err
	}
	owned, statErr := f.Stat()
	closeErr := f.Close()
	if err = errors.Join(statErr, closeErr); err != nil {
		return nil, err
	}
	current, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !current.Mode().IsRegular() || !os.SameFile(owned, current) || current.Size() != 0 {
		return nil, fmt.Errorf("import destination: exclusive reservation identity changed")
	}
	tx, err := pager.OpenTx(ctx, path, pager.Options{Create: true})
	if err != nil {
		return nil, err
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, errors.Join(err, tx.Rollback())
	}
	current, err = os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !os.SameFile(owned, current) {
		return nil, fmt.Errorf("import destination: initialized identity changed")
	}
	return sqlite.OpenContext(ctx, path, sqlite.Options{NoCreate: true})
}
func initializeImport(ctx context.Context, db *sqlite.DB, m ImportManifest, o ImportOptions, offset int64) (importJob, error) {
	empty := importHash(nil)
	j := importJob{Version: ImportVersion, Converter: ImportConverter, ManifestHash: m.hash(), Policy: o.OnError, Offset: offset, CanonicalHash: empty, RecordsHash: empty, RejectionsHash: empty, Status: "importing"}
	tx, err := db.Begin(ctx, sqlite.TxOptions{Immediate: true})
	if err != nil {
		return j, err
	}
	statements := []string{m.createSQL()}
	for _, table := range []string{importJobs, importBatches, importRejections, importAccepted} {
		statements = append(statements, "CREATE TABLE "+importQuote(table)+" (id INTEGER PRIMARY KEY, payload TEXT NOT NULL)")
	}
	for _, sql := range statements {
		if _, err = tx.Exec(ctx, sql); err != nil {
			return j, rollbackImport(tx, err)
		}
	}
	if j.SchemaHash, err = importSchemaHash(ctx, db); err != nil {
		return j, rollbackImport(tx, err)
	}
	if err = insertImportJSON(ctx, tx, importJobs, 1, j); err != nil {
		return j, rollbackImport(tx, err)
	}
	if err = tx.Commit(ctx); err != nil {
		return j, rollbackImport(tx, err)
	}
	return j, nil
}
func readImportJob(ctx context.Context, db *sqlite.DB, m ImportManifest, o ImportOptions) (importJob, error) {
	var j importJob
	ok, err := readImportJSON(ctx, db, importJobs, 1, &j)
	if err != nil || !ok {
		return j, errors.Join(err, fmt.Errorf("import resume: matching committed job required"))
	}
	if err = j.validate(m, o.OnError); err != nil {
		return j, err
	}
	fingerprint, err := importSchemaHash(ctx, db)
	if err != nil {
		return j, err
	}
	if fingerprint != j.SchemaHash {
		return j, fmt.Errorf("import resume: schema fingerprint mismatch")
	}
	return j, nil
}

type importPending struct {
	record importRecord
	err    error
}

func importDryRun(ctx context.Context, source *importSource, m ImportManifest, o ImportOptions) (ImportResult, error) {
	result := newImportResult(m)
	result.DryRun = true
	reader, err := newImportRecordReader(ctx, source.file, m, MaxImportRecordBytes, 0, nil)
	if err != nil {
		return result, err
	}
	hs := newImportHashes()
	seen := map[int64]bool{}
	var last *int64
	for {
		rec, e := reader.Next(ctx)
		if e == io.EOF {
			break
		}
		if result.SourceRecords >= MaxImportRows {
			return result, fmt.Errorf("import: source row limit")
		}
		n := result.SourceRecords + 1
		issue := importRowIssue(m, rec, e, &last)
		if issue == nil && m.RowOrder == "source" && seen[rec.PK] {
			issue = &ImportRowError{Code: "duplicate_pk", Field: m.pkField() + 1}
		}
		var rowIssue *ImportRowError
		if issue != nil {
			if !errors.As(issue, &rowIssue) || o.OnError != "quarantine" {
				return result, fmt.Errorf("import record %d: %w", n, issue)
			}
			if result.Rejected >= MaxImportRejections {
				return result, fmt.Errorf("import: rejection limit")
			}
			rejection := importRejectionFor(n, rec, rowIssue)
			hashImportRejection(hs.rejections, rejection)
			result.Rejected++
			if rejection.PK == nil {
				result.RejectionsWithoutPK++
			} else {
				result.RejectedPKs = append(result.RejectedPKs, *rejection.PK)
			}
		} else {
			if m.RowOrder == "source" {
				if len(seen) >= MaxImportDryRunKeys {
					return result, fmt.Errorf("import dry-run: unordered unique-key limit; use an ascending export for constant-memory validation")
				}
				seen[rec.PK] = true
			}
			raw, e := importCanonicalValues(m, rec.Values)
			if e != nil {
				return result, e
			}
			importWriteBytes(hs.canonical, raw)
			result.Accepted++
			importAddStats(&result, m, rec.Values)
		}
		hashImportRecord(hs.records, n, rec)
		result.SourceRecords = n
	}
	if m.ExpectedRows != nil && result.SourceRecords != *m.ExpectedRows {
		return result, fmt.Errorf("import: expected source count mismatch")
	}
	if err = source.recheck(ctx, m); err != nil {
		return result, err
	}
	result.Status = "dry_run_validated"
	if result.Rejected > 0 {
		result.Status = "dry_run_with_rejections"
	}
	result.CursorOffset = m.SourceSize
	result.CanonicalSHA256 = importSum(hs.canonical)
	return result, nil
}

// Import validates an immutable local CSV/NDJSON export and optionally creates
// a genuine native SQLite database. It never connects to PostgreSQL. A returned
// error must be inspected: committed earlier batches remain resumable, while an
// uncertain commit is not automatically retried or described as rolled back.
func Import(ctx context.Context, input, output string, m ImportManifest, options ImportOptions) (result ImportResult, resultErr error) {
	if ctx == nil {
		return result, fmt.Errorf("import: nil context")
	}
	if err := m.Validate(); err != nil {
		return result, err
	}
	o, err := options.normalized()
	if err != nil {
		return result, err
	}
	source, err := openImportSource(ctx, input, m)
	if err != nil {
		return result, err
	}
	defer func() { resultErr = errors.Join(resultErr, source.file.Close()) }()
	if !o.Apply {
		return importDryRun(ctx, source, m, o)
	}
	reader, err := newImportRecordReader(ctx, source.file, m, MaxImportRecordBytes, 0, nil)
	if err != nil {
		return result, err
	}
	header := reader.Header()
	offset := reader.Offset()
	var db *sqlite.DB
	var j importJob
	hs := newImportHashes()
	if o.Resume {
		db, err = sqlite.OpenContext(ctx, output, sqlite.Options{NoCreate: true})
	} else {
		db, err = reserveImportDestination(ctx, output)
	}
	if err != nil {
		return result, err
	}
	defer func() {
		if db != nil {
			resultErr = errors.Join(resultErr, db.Close())
		}
	}()
	if o.Resume {
		tx, e := db.Begin(ctx, sqlite.TxOptions{ReadOnly: true})
		if e != nil {
			return result, e
		}
		j, err = readImportJob(ctx, db, m, o)
		if err == nil {
			hs, result, err = verifyImportPrefix(ctx, source.file, m, db, j)
		}
		if err == nil {
			for _, index := range m.Indexes[:j.IndexesDone] {
				if err = checkImportIndex(ctx, db, m, index); err != nil {
					break
				}
			}
		}
		err = errors.Join(err, tx.Rollback())
		if err != nil {
			return result, err
		}
	} else {
		j, err = initializeImport(ctx, db, m, o, offset)
		if err != nil {
			return result, err
		}
	}
	if j.Status == "importing" {
		if _, err = source.file.Seek(j.Offset, io.SeekStart); err != nil {
			return result, err
		}
		reader, err = newImportRecordReader(ctx, source.file, m, MaxImportRecordBytes, j.Offset, header)
		if err != nil {
			return result, err
		}
		var pending *importPending
		finished := false
		startBatch := j.Batch
		for !finished {
			batch := []importPending{}
			var size int64
			for len(batch) < o.BatchRows {
				var p importPending
				if pending != nil {
					p = *pending
					pending = nil
				} else {
					p.record, p.err = reader.Next(ctx)
				}
				if p.err == io.EOF {
					finished = true
					break
				}
				var rowIssue *ImportRowError
				if p.err != nil && !errors.As(p.err, &rowIssue) {
					return result, p.err
				}
				if p.record.DecodedBytes > o.BatchBytes {
					return result, fmt.Errorf("import: record exceeds batch decoded-byte bound")
				}
				if size+p.record.DecodedBytes > o.BatchBytes && len(batch) > 0 {
					pending = &p
					break
				}
				size += p.record.DecodedBytes
				batch = append(batch, p)
			}
			if len(batch) == 0 {
				break
			}
			if j.Cursor+int64(len(batch)) > MaxImportRows {
				return result, fmt.Errorf("import: source row limit")
			}
			previous := j
			next := j
			tx, e := db.Begin(ctx, sqlite.TxOptions{Immediate: true})
			if e != nil {
				return result, e
			}
			actual, e := readImportJob(ctx, db, m, o)
			if e != nil {
				return result, rollbackImport(tx, e)
			}
			a, _ := json.Marshal(actual)
			b, _ := json.Marshal(previous)
			if string(a) != string(b) {
				return result, rollbackImport(tx, fmt.Errorf("import: committed job changed"))
			}
			issues := make([]*ImportRowError, len(batch))
			within := map[int64]bool{}
			for i, p := range batch {
				issue := importRowIssue(m, p.record, p.err, &next.LastPK)
				if issue == nil {
					if within[p.record.PK] {
						issue = &ImportRowError{Code: "duplicate_pk", Field: m.pkField() + 1}
					} else {
						var provenance importAcceptedRow
						exists, e := readImportJSON(ctx, tx, importAccepted, p.record.PK, &provenance)
						if e != nil {
							return result, rollbackImport(tx, e)
						}
						if exists {
							issue = &ImportRowError{Code: "duplicate_pk", Field: m.pkField() + 1}
						}
					}
				}
				if issue != nil {
					var rowIssue *ImportRowError
					if !errors.As(issue, &rowIssue) || o.OnError != "quarantine" {
						return result, rollbackImport(tx, fmt.Errorf("import record %d: %w", j.Cursor+int64(i)+1, issue))
					}
					issues[i] = rowIssue
					next.Rejected++
					if next.Rejected > MaxImportRejections {
						return result, rollbackImport(tx, fmt.Errorf("import: rejection limit"))
					}
				} else {
					within[p.record.PK] = true
					next.Accepted++
				}
			}
			for i, p := range batch {
				n := j.Cursor + int64(i) + 1
				hashImportRecord(hs.records, n, p.record)
				if issues[i] != nil {
					r := importRejectionFor(n, p.record, issues[i])
					hashImportRejection(hs.rejections, r)
					if e = insertImportJSON(ctx, tx, importRejections, n, r); e != nil {
						return result, rollbackImport(tx, e)
					}
				} else {
					if _, e = tx.Exec(ctx, m.insertSQL(), p.record.Values...); e != nil {
						return result, rollbackImport(tx, e)
					}
					if e = insertImportJSON(ctx, tx, importAccepted, p.record.PK, importAcceptedRow{Record: n}); e != nil {
						return result, rollbackImport(tx, e)
					}
					raw, e := importCanonicalValues(m, p.record.Values)
					if e != nil {
						return result, rollbackImport(tx, e)
					}
					importWriteBytes(hs.canonical, raw)
				}
			}
			next.Cursor += int64(len(batch))
			next.Offset = batch[len(batch)-1].record.EndOffset
			next.Batch++
			next.CanonicalHash = importSum(hs.canonical)
			next.RecordsHash = importSum(hs.records)
			next.RejectionsHash = importSum(hs.rejections)
			record := importBatch{Sequence: next.Batch, StartCursor: previous.Cursor, StartOffset: previous.Offset, Cursor: next.Cursor, Offset: next.Offset, Accepted: next.Accepted, Rejected: next.Rejected, CanonicalHash: next.CanonicalHash, RecordsHash: next.RecordsHash, RejectionsHash: next.RejectionsHash}
			if e = insertImportJSON(ctx, tx, importBatches, next.Batch, record); e != nil {
				return result, rollbackImport(tx, e)
			}
			if e = saveImportJob(ctx, tx, next); e != nil {
				return result, rollbackImport(tx, e)
			}
			if e = ctx.Err(); e != nil {
				return result, rollbackImport(tx, e)
			}
			if e = tx.Commit(ctx); e != nil {
				return result, rollbackImport(tx, e)
			}
			j = next
			if o.StopAfterBatches > 0 && j.Batch-startBatch >= o.StopAfterBatches {
				result = newImportResult(m)
				result.Status = "paused_committed"
				result.SourceRecords = j.Cursor
				result.Accepted = j.Accepted
				result.Rejected = j.Rejected
				result.CursorOffset = j.Offset
				result.CommittedBatches = j.Batch
				result.CanonicalSHA256 = j.CanonicalHash
				return result, nil
			}
		}
		if j.Offset != m.SourceSize {
			return result, fmt.Errorf("import: final record offset mismatch")
		}
		if m.ExpectedRows != nil && j.Cursor != *m.ExpectedRows {
			return result, fmt.Errorf("import: expected source count mismatch")
		}
		if err = source.recheck(ctx, m); err != nil {
			return result, err
		}
		j.Status = "data_complete_index_pending"
		if err = updateImportStatus(ctx, db, &j); err != nil {
			return result, err
		}
	}
	if err = buildImportIndexes(ctx, db, m, &j); err != nil {
		result = newImportResult(m)
		result.Status = j.Status
		result.Accepted = j.Accepted
		result.Rejected = j.Rejected
		result.SourceRecords = j.Cursor
		return result, err
	}
	// Verify using a genuinely closed/reopened read-only connection and a fresh
	// source scan. Keep completion pending until the exhaustive comparison passes.
	if err = db.Close(); err != nil {
		return result, err
	}
	db = nil
	result, err = VerifyImport(ctx, input, output, m)
	if err != nil {
		return result, err
	}
	verifiedStatus := result.Status
	result.Status = j.Status
	db, err = sqlite.OpenContext(ctx, output, sqlite.Options{NoCreate: true})
	if err != nil {
		return result, err
	}
	publication, err := db.Begin(ctx, sqlite.TxOptions{Immediate: true})
	if err != nil {
		return result, err
	}
	current, err := readImportJob(ctx, db, m, o)
	if err != nil {
		return result, rollbackImport(publication, err)
	}
	if current.Cursor != j.Cursor || current.CanonicalHash != j.CanonicalHash || current.IndexesDone != j.IndexesDone {
		return result, rollbackImport(publication, fmt.Errorf("import: job changed before verification publication"))
	}
	// Reconcile again under the writer-reserved snapshot used to publish status.
	// This prevents an intervening normal writer from receiving a stale success.
	_, checked, err := verifyImportPrefix(ctx, source.file, m, db, current)
	if err != nil {
		return result, rollbackImport(publication, err)
	}
	if err = source.recheck(ctx, m); err != nil {
		return result, rollbackImport(publication, err)
	}
	for _, index := range m.Indexes {
		if err = checkImportIndex(ctx, db, m, index); err != nil {
			return result, rollbackImport(publication, err)
		}
	}
	current.Status = verifiedStatus
	checked.Status = verifiedStatus
	if err = saveImportJob(ctx, publication, current); err != nil {
		return result, rollbackImport(publication, err)
	}
	if err = ctx.Err(); err != nil {
		return result, rollbackImport(publication, err)
	}
	if err = publication.Commit(ctx); err != nil {
		return result, rollbackImport(publication, err)
	}
	result = checked
	return result, nil
}
func updateImportStatus(ctx context.Context, db *sqlite.DB, j *importJob) error {
	tx, err := db.Begin(ctx, sqlite.TxOptions{Immediate: true})
	if err != nil {
		return err
	}
	if err = saveImportJob(ctx, tx, *j); err != nil {
		return rollbackImport(tx, err)
	}
	if err = tx.Commit(ctx); err != nil {
		return rollbackImport(tx, err)
	}
	return nil
}
