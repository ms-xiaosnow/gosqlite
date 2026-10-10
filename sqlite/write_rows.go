package sqlite

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

func (c *writeCatalog) find(name string) (*writeTable, error) {
	if extensionReserved(name) {
		return nil, fmt.Errorf("%w: owned extension table", ErrUnsupported)
	}
	tab := c.tables[fold(name)]
	if tab == nil {
		return nil, fmt.Errorf("no such table: %s", name)
	}
	for _, x := range tab.extensions {
		if !x.usable {
			return nil, fmt.Errorf("%w: %s: %s", ErrReindexRequired, x.name, x.fallback)
		}
	}
	return tab, nil
}
func (t *Tx) nextRowID(ctx context.Context, root uint32) (int64, error) {
	largest, found, err := btree.LastTableRowID(ctx, t.reader, root)
	if err != nil {
		return 0, err
	}
	if !found {
		return 1, nil
	}
	if largest < math.MaxInt64 {
		return largest + 1, nil
	}
	// SQLite uses at most 100 random positive candidates when the maximum key
	// is already INT64_MAX. Do not overflow to a negative generated key.
	for range 100 {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		var bytes [8]byte
		if _, err := rand.Read(bytes[:]); err != nil {
			return 0, err
		}
		id := int64(binary.BigEndian.Uint64(bytes[:]) & math.MaxInt64)
		if id == 0 {
			continue
		}
		_, exists, err := btree.SeekTable(ctx, t.reader, root, id)
		if err != nil {
			return 0, err
		}
		if !exists {
			return id, nil
		}
	}
	return 0, fmt.Errorf("%w: cannot allocate a positive rowid", ErrFull)
}
func integerRowID(value any) (int64, error) {
	v, err := sqlast.ApplyAffinity(value, "INTEGER")
	if err != nil {
		return 0, err
	}
	if n, ok := v.(int64); ok {
		return n, nil
	}
	if f, ok := v.(float64); ok && f >= -9223372036854775808.0 && f < 9223372036854775808.0 && math.Trunc(f) == f {
		return int64(f), nil
	}
	return 0, fmt.Errorf("%w: rowid must be an integer", ErrMismatch)
}
func (tab *writeTable) target(name string) (int, error) {
	i, err := tab.column("", name)
	if err == nil && i == tab.ipk {
		i = -1
	}
	return i, err
}
func (tab *writeTable) constrain(rowid int64, values []any) ([]any, error) {
	for i, col := range tab.columns {
		if i == tab.ipk {
			values[i] = rowid
			continue
		}
		var err error
		values[i], err = storedColumnValue(values[i], col.Type)
		if err != nil {
			return nil, err
		}
		if values[i] == nil && col.NotNull {
			return nil, fmt.Errorf("%w: NOT NULL %s.%s", ErrConstraint, tab.name, col.Name)
		}
	}
	return values, nil
}
func (tab *writeTable) physical(values []any) []any {
	record := append([]any(nil), values...)
	if tab.ipk >= 0 {
		record[tab.ipk] = nil
	}
	return record
}
func (t *Tx) addIndexes(ctx context.Context, tab *writeTable, rowid int64, values []any) error {
	for _, idx := range tab.indexes {
		if err := btree.InsertIndex(ctx, t.storage, idx.root, btree.KeyInfo{Columns: len(idx.columns)}, idx.key(rowid, values)); err != nil {
			return err
		}
	}
	return nil
}
func (t *Tx) removeIndexes(ctx context.Context, tab *writeTable, rowid int64, values []any) error {
	for _, idx := range tab.indexes {
		found, err := btree.DeleteIndex(ctx, t.storage, idx.root, btree.KeyInfo{Columns: len(idx.columns)}, idx.key(rowid, values))
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("%w: index %q is missing a table row", ErrCorrupt, idx.name)
		}
	}
	return nil
}
func (t *Tx) insert(ctx context.Context, c *writeCatalog, stmt *sqlast.Insert, params []any) (Result, error) {
	var result Result
	tab, err := c.find(stmt.Table)
	if err != nil {
		return result, err
	}
	var targets []int
	seen := map[int]bool{}
	if len(stmt.Columns) == 0 && !stmt.DefaultValues {
		for i := range tab.columns {
			if i == tab.ipk {
				targets = append(targets, -1)
			} else {
				targets = append(targets, i)
			}
		}
	} else {
		for _, name := range stmt.Columns {
			i, err := tab.target(name)
			if err != nil {
				return result, err
			}
			if seen[i] {
				return result, fmt.Errorf("%w: duplicate rowid target", ErrUnsupported)
			}
			seen[i] = true
			targets = append(targets, i)
		}
	}
	rows := stmt.Rows
	if stmt.DefaultValues {
		rows = [][]sqlast.Expr{nil}
	}
	for _, exprs := range rows {
		if !stmt.DefaultValues && len(exprs) != len(targets) {
			return result, fmt.Errorf("VALUES column count mismatch")
		}
		for _, expr := range exprs {
			if err := validate(expr, nil, nil); err != nil {
				return result, err
			}
		}
	}
	for _, exprs := range rows {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		values := make([]any, len(tab.columns))
		for i, col := range tab.columns {
			if col.Default != nil {
				values[i], err = sqlast.Eval(col.Default, &sqlast.EvalContext{Context: ctx})
				if err != nil {
					return result, err
				}
			}
		}
		var rowValue any
		if tab.ipk >= 0 {
			rowValue = values[tab.ipk]
		}
		for i, expr := range exprs {
			v, err := sqlast.Eval(expr, &sqlast.EvalContext{Context: ctx, Params: params})
			if err != nil {
				return result, err
			}
			if targets[i] < 0 {
				rowValue = v
			} else {
				values[targets[i]] = v
			}
		}
		var rowid int64
		if rowValue == nil {
			rowid, err = t.nextRowID(ctx, tab.root)
		} else {
			rowid, err = integerRowID(rowValue)
		}
		if err != nil {
			return result, err
		}
		values, err = tab.constrain(rowid, values)
		if err != nil {
			return result, err
		}
		if err = btree.InsertTable(ctx, t.storage, tab.root, rowid, tab.physical(values)); err != nil {
			return result, err
		}
		if err = t.addIndexes(ctx, tab, rowid, values); err != nil {
			return result, err
		}
		if err = t.applyExtensions(ctx, c, tab, 0, nil, rowid, values); err != nil {
			return result, err
		}
		result.affected++
		result.lastID = rowid
	}
	return result, nil
}
func (tab *writeTable) evalContext(ctx context.Context, params []any, rowid int64, values []any) *sqlast.EvalContext {
	return &sqlast.EvalContext{Context: ctx, Params: params, Resolver: func(q, name string) (any, error) {
		i, err := tab.column(q, name)
		if err != nil {
			return nil, err
		}
		if i < 0 {
			return rowid, nil
		}
		return columnValue(values[i], tab.columns[i].Type)
	}, Affinity: func(q, name string) string {
		i, err := tab.column(q, name)
		if err != nil {
			return ""
		}
		if i < 0 {
			return "INTEGER"
		}
		return affinity(tab.columns[i].Type)
	}}
}

// Select only rowids before mutating, so changing keys never re-visits a row.
// This bounded work list contains no retained table values or whole DB image.
func (t *Tx) matchRows(ctx context.Context, tab *writeTable, where sqlast.Expr, params []any) ([]int64, error) {
	if err := validate(where, tab.table, nil); err != nil {
		return nil, err
	}
	cursor, err := btree.NewCursor(ctx, t.reader, tab.root)
	if err != nil {
		return nil, err
	}
	defer cursor.Close()
	var ids []int64
	for {
		id, raw, err := cursor.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		values, err := tab.values(id, raw)
		if err != nil {
			return nil, err
		}
		if where != nil {
			v, err := sqlast.Eval(where, tab.evalContext(ctx, params, id, values))
			if err != nil {
				return nil, err
			}
			truth, null := sqlast.Truth(v)
			if null || !truth {
				continue
			}
		}
		if int64(len(ids))+1 > t.db.opts.MaxSortBytes/32 {
			return nil, ErrMemoryLimit
		}
		ids = append(ids, id)
	}
	return ids, nil
}
func (t *Tx) update(ctx context.Context, c *writeCatalog, stmt *sqlast.Update, params []any) (Result, error) {
	var result Result
	tab, err := c.find(stmt.Table)
	if err != nil {
		return result, err
	}
	targets := make([]int, len(stmt.Assignments))
	seen := map[int]bool{}
	for i, assignment := range stmt.Assignments {
		targets[i], err = tab.target(assignment.Column)
		if err != nil {
			return result, err
		}
		if seen[targets[i]] {
			return result, fmt.Errorf("%w: duplicate rowid target", ErrUnsupported)
		}
		seen[targets[i]] = true
		stmt.Assignments[i].Expr = bindBooleans(assignment.Expr, tab.table)
		if err := validate(stmt.Assignments[i].Expr, tab.table, nil); err != nil {
			return result, err
		}
	}
	stmt.Where = bindBooleans(stmt.Where, tab.table)
	ids, err := t.matchRows(ctx, tab, stmt.Where, params)
	if err != nil {
		return result, err
	}
	for _, oldID := range ids {
		raw, exists, err := btree.SeekTable(ctx, t.reader, tab.root, oldID)
		if err != nil {
			return result, err
		}
		if !exists {
			return result, fmt.Errorf("%w: row disappeared during update", ErrCorrupt)
		}
		oldValues, err := tab.values(oldID, raw)
		if err != nil {
			return result, err
		}
		values := append([]any(nil), oldValues...)
		newID := oldID
		ec := tab.evalContext(ctx, params, oldID, oldValues)
		for i, assignment := range stmt.Assignments {
			v, err := sqlast.Eval(assignment.Expr, ec)
			if err != nil {
				return result, err
			}
			if targets[i] < 0 {
				newID, err = integerRowID(v)
				if err != nil {
					return result, err
				}
			} else {
				values[targets[i]] = v
			}
		}
		values, err = tab.constrain(newID, values)
		if err != nil {
			return result, err
		}
		if newID != oldID {
			if _, exists, err := btree.SeekTable(ctx, t.reader, tab.root, newID); err != nil {
				return result, err
			} else if exists {
				return result, fmt.Errorf("%w: duplicate rowid", ErrConstraint)
			}
		}
		if err = t.removeIndexes(ctx, tab, oldID, oldValues); err != nil {
			return result, err
		}
		if newID == oldID {
			err = btree.ReplaceTable(ctx, t.storage, tab.root, newID, tab.physical(values))
		} else {
			_, err = btree.DeleteTable(ctx, t.storage, tab.root, oldID)
			if err == nil {
				err = btree.InsertTable(ctx, t.storage, tab.root, newID, tab.physical(values))
			}
		}
		if err != nil {
			return result, err
		}
		if err = t.addIndexes(ctx, tab, newID, values); err != nil {
			return result, err
		}
		if err = t.applyExtensions(ctx, c, tab, oldID, oldValues, newID, values); err != nil {
			return result, err
		}
		result.affected++
	}
	return result, nil
}
func (t *Tx) delete(ctx context.Context, c *writeCatalog, stmt *sqlast.Delete, params []any) (Result, error) {
	var result Result
	tab, err := c.find(stmt.Table)
	if err != nil {
		return result, err
	}
	stmt.Where = bindBooleans(stmt.Where, tab.table)
	ids, err := t.matchRows(ctx, tab, stmt.Where, params)
	if err != nil {
		return result, err
	}
	for _, id := range ids {
		raw, exists, err := btree.SeekTable(ctx, t.reader, tab.root, id)
		if err != nil {
			return result, err
		}
		if !exists {
			return result, fmt.Errorf("%w: row disappeared during delete", ErrCorrupt)
		}
		values, err := tab.values(id, raw)
		if err != nil {
			return result, err
		}
		if err = t.removeIndexes(ctx, tab, id, values); err != nil {
			return result, err
		}
		if err = t.applyExtensions(ctx, c, tab, id, values, 0, nil); err != nil {
			return result, err
		}
		if _, err = btree.DeleteTable(ctx, t.storage, tab.root, id); err != nil {
			return result, err
		}
		result.affected++
	}
	return result, nil
}
