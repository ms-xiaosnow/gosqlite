package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/extension/vector/ann"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func buildImportIndexes(ctx context.Context, db *sqlite.DB, m ImportManifest, j *importJob) error {
	if j.Status == "complete_verified" || j.Status == "complete_with_rejections" {
		return nil
	}
	for j.IndexesDone < len(m.Indexes) {
		x := m.Indexes[j.IndexesDone]
		dims := 0
		for _, f := range m.Fields {
			if f.Target == x.Column {
				dims = f.Dimensions
			}
		}
		cfg, err := x.config(dims)
		if err != nil {
			return err
		}
		tx, err := db.Begin(ctx, sqlite.TxOptions{Immediate: true})
		if err != nil {
			return err
		}
		// Validate every non-NULL value before building. Zero-norm cosine values
		// remain in the imported data and explicitly block the requested index.
		rows, err := tx.Query(ctx, "SELECT "+importQuote(x.Column)+" FROM "+importQuote(m.Table))
		if err != nil {
			return rollbackImport(tx, err)
		}
		var eligible, trainable int64
		for rows.Next() {
			value := rows.Values()[0]
			if value == nil {
				continue
			}
			raw, ok := value.([]byte)
			if !ok {
				rows.Close()
				return rollbackImport(tx, fmt.Errorf("import index: noncanonical vector"))
			}
			v, e := vector.Decode(raw)
			if e != nil {
				rows.Close()
				return rollbackImport(tx, e)
			}
			if e = ann.ValidateVector(v, cfg); e != nil {
				rows.Close()
				return rollbackImport(tx, fmt.Errorf("import index: requested metric rejects a retained vector: %w", e))
			}
			eligible++
			if x.Method == ann.IVFFlat {
				ok, e := ann.Trainable(v, cfg)
				if e != nil {
					rows.Close()
					return rollbackImport(tx, e)
				}
				if ok {
					trainable++
				}
			}
		}
		err = errors.Join(rows.Err(), rows.Close())
		if err != nil {
			return rollbackImport(tx, err)
		}
		if eligible > cfg.MaxNodes {
			return rollbackImport(tx, fmt.Errorf("import index: native node quota"))
		}
		if x.Method == ann.IVFFlat && trainable < int64(cfg.Lists) {
			return rollbackImport(tx, fmt.Errorf("import index: insufficient trainable rows; requested IVF remains pending"))
		}
		canonical, err := cfg.Canonical()
		if err != nil {
			return rollbackImport(tx, err)
		}
		if err = tx.CreateMethodIndex(ctx, x.Name, m.Table, []string{x.Column}, x.Method, ann.MethodVersion, canonical); err != nil {
			return rollbackImport(tx, err)
		}
		if err = checkImportIndex(ctx, db, m, x); err != nil {
			return rollbackImport(tx, err)
		}
		next := *j
		next.IndexesDone++
		next.SchemaHash, err = importSchemaHash(ctx, db)
		if err != nil {
			return rollbackImport(tx, err)
		}
		if err = saveImportJob(ctx, tx, next); err != nil {
			return rollbackImport(tx, err)
		}
		if err = tx.Commit(ctx); err != nil {
			return rollbackImport(tx, err)
		}
		*j = next
	}
	j.Status = "data_complete_verify_pending"
	return updateImportStatus(ctx, db, j)
}

// Native extension storage version 1 uses ordinary SQLite metadata tables.
// Public consistency checking does not expose persisted configuration, so this
// bounded read-only adapter additionally checks exact requested identity. It
// never edits native extension tables or guesses a newer storage version.
func checkImportIndex(ctx context.Context, db *sqlite.DB, m ImportManifest, x ImportIndex) error {
	if err := db.CheckExtensionIndex(ctx, x.Name); err != nil {
		return err
	}
	values, err := oneImportNativeRow(ctx, db, "SELECT id, source_table, method, method_version, storage_version, config FROM __gosqlite_ext_catalog WHERE name = ?", x.Name)
	if err != nil {
		return err
	}
	if len(values) != 6 {
		return fmt.Errorf("import index: native catalog shape")
	}
	id, ok := values[0].(int64)
	if !ok || id < 1 {
		return fmt.Errorf("import index: native identity")
	}
	fields, err := oneImportNativeRow(ctx, db, "SELECT column_name, column_ordinal FROM __gosqlite_ext_fields WHERE index_id = ?", id)
	if err != nil {
		return err
	}
	var state []any
	if x.Method == ann.IVFFlat {
		state, err = oneImportNativeRow(ctx, db, "SELECT training_state FROM __gosqlite_ext_ann_state WHERE index_id = ?", id)
		if err != nil {
			return err
		}
	}
	return validateImportIndexIdentity(m, x, values, fields, state)
}

// Pure validation allows malformed native identity models to be reviewed
// without modifying any selected database or extension-owned file.
func validateImportIndexIdentity(m ImportManifest, x ImportIndex, values, fields, state []any) error {
	ordinal, dims := -1, 0
	for i, field := range m.Fields {
		if field.Target == x.Column {
			ordinal, dims = i, field.Dimensions
		}
	}
	cfg, err := x.config(dims)
	if err != nil {
		return err
	}
	canonical, err := cfg.Canonical()
	if err != nil {
		return err
	}
	if len(values) != 6 || values[1] != m.Table || values[2] != x.Method || values[3] != int64(ann.MethodVersion) || values[4] != int64(1) || values[5] != string(canonical) {
		return fmt.Errorf("import index: persisted method/config/source identity mismatch")
	}
	id, ok := values[0].(int64)
	if !ok || id < 1 {
		return fmt.Errorf("import index: native identity")
	}
	if len(fields) != 2 || fields[0] != x.Column || fields[1] != int64(ordinal) {
		return fmt.Errorf("import index: persisted field identity mismatch")
	}
	if x.Method == ann.IVFFlat && (len(state) != 1 || state[0] != "ready") {
		return fmt.Errorf("import index: IVF is not trained")
	}
	return nil
}

func oneImportNativeRow(ctx context.Context, db *sqlite.DB, sql string, args ...any) ([]any, error) {
	rows, err := db.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if !rows.Next() {
		return nil, errors.Join(rows.Err(), fmt.Errorf("import index: missing native metadata"))
	}
	values := rows.Values()
	for _, v := range values {
		if text, ok := v.(string); ok && len(text) > importMetaLimit {
			return nil, fmt.Errorf("import index: metadata bound")
		}
	}
	if rows.Next() {
		return nil, fmt.Errorf("import index: ambiguous native metadata")
	}
	return values, rows.Err()
}
