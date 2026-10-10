package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

const importJobs = "gosqlite_migration_jobs"
const importBatches = "gosqlite_migration_batches"
const importRejections = "gosqlite_migration_rejections"
const importMetaLimit = 16 << 10

// Metadata lives in ordinary tables and commits together with imported rows.
// A single job owns a new destination. No progress sidecar is authoritative.
type importJob struct {
	Version        int    `json:"version"`
	Converter      string `json:"converter"`
	ManifestHash   string `json:"manifest_hash"`
	Policy         string `json:"policy"`
	Cursor         int64  `json:"cursor"`
	Offset         int64  `json:"offset"`
	Accepted       int64  `json:"accepted"`
	Rejected       int64  `json:"rejected"`
	Batch          int64  `json:"batch"`
	LastPK         *int64 `json:"last_pk,omitempty"`
	CanonicalHash  string `json:"canonical_hash"`
	RecordsHash    string `json:"records_hash"`
	RejectionsHash string `json:"rejections_hash"`
	SchemaHash     string `json:"schema_hash"`
	Status         string `json:"status"`
	IndexesDone    int    `json:"indexes_done"`
}
type importBatch struct {
	Sequence       int64  `json:"sequence"`
	StartCursor    int64  `json:"start_cursor"`
	StartOffset    int64  `json:"start_offset"`
	Cursor         int64  `json:"cursor"`
	Offset         int64  `json:"offset"`
	Accepted       int64  `json:"accepted"`
	Rejected       int64  `json:"rejected"`
	CanonicalHash  string `json:"canonical_hash"`
	RecordsHash    string `json:"records_hash"`
	RejectionsHash string `json:"rejections_hash"`
}
type importRejection struct {
	Record int64  `json:"record"`
	PK     *int64 `json:"pk,omitempty"`
	Code   string `json:"code"`
	Field  int    `json:"field"`
	Digest string `json:"digest"`
}
type importQuerier interface {
	Query(context.Context, string, ...any) (*sqlite.Rows, error)
}

func readImportJSON(ctx context.Context, q importQuerier, table string, key int64, dst any) (bool, error) {
	rows, err := q.Query(ctx, "SELECT payload FROM "+importQuote(table)+" WHERE id = ?", key)
	if err != nil {
		return false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return false, rows.Err()
	}
	v := rows.Values()
	if len(v) != 1 {
		return false, fmt.Errorf("import metadata: row shape")
	}
	s, ok := v[0].(string)
	if !ok || len(s) > importMetaLimit {
		return false, fmt.Errorf("import metadata: payload type or bound")
	}
	limits := DefaultLimits()
	limits.MaxDepth = 4
	limits.MaxTokens = 256
	if err = strictJSON([]byte(s), dst, limits); err != nil {
		return false, fmt.Errorf("import metadata: malformed payload")
	}
	if rows.Next() {
		return false, fmt.Errorf("import metadata: duplicate identity")
	}
	return true, rows.Err()
}
func insertImportJSON(ctx context.Context, tx *sqlite.Tx, table string, key int64, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(raw) > importMetaLimit {
		return fmt.Errorf("import metadata: payload bound")
	}
	_, err = tx.Exec(ctx, "INSERT INTO "+importQuote(table)+" (id,payload) VALUES (?,?)", key, string(raw))
	return err
}
func saveImportJob(ctx context.Context, tx *sqlite.Tx, j importJob) error {
	raw, err := json.Marshal(j)
	if err != nil {
		return err
	}
	result, err := tx.Exec(ctx, "UPDATE "+importJobs+" SET payload = ? WHERE id = ?", string(raw), int64(1))
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return fmt.Errorf("import metadata: job disappeared")
	}
	return nil
}
func importSchemaHash(ctx context.Context, db *sqlite.DB) (string, error) {
	entries, err := db.Schema(ctx)
	if err != nil {
		return "", err
	}
	if len(entries) > 128 {
		return "", fmt.Errorf("import schema: object limit")
	}
	type schemaItem struct{ Type, Name, Table, SQL string }
	items := make([]schemaItem, len(entries))
	for i, e := range entries {
		items[i] = schemaItem{e.Type, e.Name, e.TableName, e.SQL}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	raw, err := json.Marshal(items)
	if err != nil {
		return "", err
	}
	return importHash(raw), nil
}
func countImportRows(ctx context.Context, q importQuerier, sql string, limit int64) (int64, error) {
	rows, err := q.Query(ctx, sql)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var n int64
	for rows.Next() {
		n++
		if n > limit {
			return n, fmt.Errorf("import verification: row count limit or mismatch")
		}
	}
	return n, rows.Err()
}
func (j importJob) validate(m ImportManifest, policy string) error {
	if j.Version != ImportVersion || j.Converter != ImportConverter || j.ManifestHash != m.hash() || j.Policy != policy || j.Cursor < 0 || j.Cursor > MaxImportRows || j.Offset < 0 || j.Offset > m.SourceSize || j.Accepted < 0 || j.Rejected < 0 || j.Rejected > MaxImportRejections || j.Accepted+j.Rejected != j.Cursor || j.Batch < 0 || j.Batch > j.Cursor || j.IndexesDone < 0 || j.IndexesDone > len(m.Indexes) || !validImportHash(j.CanonicalHash) || !validImportHash(j.RecordsHash) || !validImportHash(j.RejectionsHash) || !validImportHash(j.SchemaHash) {
		return fmt.Errorf("import resume: job identity or bounded state mismatch")
	}
	switch j.Status {
	case "importing":
		if j.IndexesDone != 0 {
			return fmt.Errorf("import resume: phase mismatch")
		}
	case "data_complete_index_pending", "data_complete_verify_pending", "complete_verified", "complete_with_rejections":
		if j.Offset != m.SourceSize {
			return fmt.Errorf("import resume: incomplete source cursor")
		}
	default:
		return fmt.Errorf("import resume: unknown job state")
	}
	if (j.Status == "data_complete_verify_pending" || j.Status == "complete_verified" || j.Status == "complete_with_rejections") && j.IndexesDone != len(m.Indexes) {
		return fmt.Errorf("import resume: requested indexes pending")
	}
	if j.Policy != "stop" && j.Policy != "quarantine" || j.Policy == "stop" && j.Rejected != 0 || (j.Cursor == 0) != (j.Batch == 0) || j.Cursor == 0 && j.LastPK != nil || j.Accepted > 0 && j.LastPK == nil {
		return fmt.Errorf("import resume: count, policy, or key state mismatch")
	}
	if j.Status != "importing" && m.ExpectedRows != nil && j.Cursor != *m.ExpectedRows {
		return fmt.Errorf("import resume: expected row count mismatch")
	}
	if j.Status == "complete_verified" && j.Rejected != 0 || j.Status == "complete_with_rejections" && j.Rejected == 0 {
		return fmt.Errorf("import resume: rejection status mismatch")
	}
	return nil
}
func rollbackImport(tx *sqlite.Tx, err error) error { return errors.Join(err, tx.Rollback()) }
