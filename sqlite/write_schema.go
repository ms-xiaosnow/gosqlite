package sqlite

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

type writeIndex struct {
	name    string
	root    uint32
	columns []int
}
type writeTable struct {
	*table
	indexes    []writeIndex
	extensions []*extensionIndex
}
type writeCatalog struct {
	tables     map[string]*writeTable
	names      map[string]string
	extensions *extensionCatalog
}

// Every stored definition is parsed before mutation. Unknown constraints,
// triggers, virtual tables, generated columns, collations, autoindexes and
// other unsupported objects fail closed rather than silently losing data.
func (t *Tx) validateCatalog(ctx context.Context) (*writeCatalog, error) {
	entries, err := btree.ReadSchema(ctx, t.reader)
	if err != nil {
		return nil, err
	}
	extensions, err := t.db.loadExtensionsEntries(ctx, t.reader, entries)
	if err != nil {
		return nil, err
	}
	return buildWriteCatalog(ctx, entries, extensions)
}

// buildWriteCatalog validates declarations only. It never reads row pages.
func buildWriteCatalog(ctx context.Context, entries []btree.SchemaEntry, extensions *extensionCatalog) (*writeCatalog, error) {
	c := &writeCatalog{tables: map[string]*writeTable{}, names: map[string]string{}, extensions: extensions}
	roots := map[uint32]bool{1: true}
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if e.Type == "trigger" {
			if _, owned := extensions.guards[fold(e.Name)]; owned {
				continue
			}
			for _, x := range extensions.indexes {
				for _, op := range []string{"INSERT", "UPDATE", "DELETE"} {
					if fold(e.Name) == extensionGuardName(x.id, op) {
						return nil, fmt.Errorf("%w: changed guard %q must be removed before REINDEX", ErrReindexRequired, e.Name)
					}
				}
			}
		}
		if e.Type != "table" && e.Type != "index" {
			return nil, fmt.Errorf("%w: writing a database containing %s %q", ErrUnsupported, e.Type, e.Name)
		}
		if strings.HasPrefix(fold(e.Name), "sqlite_") {
			return nil, fmt.Errorf("%w: internal schema object %q", ErrUnsupported, e.Name)
		}
		if _, exists := c.names[fold(e.Name)]; exists {
			return nil, fmt.Errorf("%w: duplicate schema name", ErrCorrupt)
		}
		c.names[fold(e.Name)] = e.Type
		if e.RootPage < 2 || roots[e.RootPage] {
			return nil, fmt.Errorf("%w: invalid or shared schema root", ErrCorrupt)
		}
		roots[e.RootPage] = true
		if e.Type != "table" {
			continue
		}
		parsed, err := sqlast.Parse(e.SQL)
		if err != nil {
			return nil, fmt.Errorf("%w: table %q: %w", ErrUnsupported, e.Name, err)
		}
		def, ok := parsed.(*sqlast.CreateTable)
		if !ok || fold(def.Name) != fold(e.Name) || fold(e.TableName) != fold(e.Name) {
			return nil, fmt.Errorf("%w: table schema mismatch", ErrCorrupt)
		}
		tab, err := writableTable(def, e.RootPage)
		if err != nil {
			return nil, err
		}
		c.tables[fold(e.Name)] = tab
	}
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if e.Type != "index" {
			continue
		}
		parsed, err := sqlast.Parse(e.SQL)
		if err != nil {
			return nil, fmt.Errorf("%w: index %q: %w", ErrUnsupported, e.Name, err)
		}
		def, ok := parsed.(*sqlast.CreateIndex)
		if !ok || fold(def.Name) != fold(e.Name) || fold(def.Table) != fold(e.TableName) {
			return nil, fmt.Errorf("%w: index schema mismatch", ErrCorrupt)
		}
		tab := c.tables[fold(e.TableName)]
		if tab == nil {
			return nil, fmt.Errorf("%w: index references missing table", ErrCorrupt)
		}
		index, err := writableIndex(def, tab, e.RootPage)
		if err != nil {
			return nil, err
		}
		tab.indexes = append(tab.indexes, index)
	}
	for _, x := range extensions.indexes {
		c.names[fold(x.name)] = "extension index"
		if tab := c.tables[fold(x.sourceTable)]; tab != nil {
			tab.extensions = append(tab.extensions, x)
		}
	}
	return c, nil
}
func writableTable(def *sqlast.CreateTable, root uint32) (*writeTable, error) {
	tab := &table{name: def.Name, root: root, columns: def.Columns, index: map[string]int{}, ipk: -1}
	if len(def.Columns) == 0 {
		return nil, fmt.Errorf("empty table definition")
	}
	for i, col := range def.Columns {
		if _, exists := tab.index[fold(col.Name)]; exists {
			return nil, fmt.Errorf("duplicate column name")
		}
		tab.index[fold(col.Name)] = i
		if col.PrimaryKey {
			if col.Type != "INTEGER" || tab.ipk >= 0 {
				return nil, fmt.Errorf("%w: only INTEGER PRIMARY KEY uniqueness is implemented", ErrUnsupported)
			}
			tab.ipk = i
		}
		if col.Default != nil {
			if err := validate(col.Default, nil, nil); err != nil {
				return nil, err
			}
			// Parenthesized constant arithmetic is accepted at CREATE time; columns
			// missing after an external ALTER require literal defaults on read.
			value, err := sqlast.Eval(col.Default, &sqlast.EvalContext{})
			if err != nil {
				return nil, err
			}
			// A nonzero root reconstructs an existing catalog. Official SQLite
			// may legally persist a half/sparse default outside our typed contract.
			// Defer its interpretation until read/insert so an explicit UPDATE
			// can replace that value. Root zero is native CREATE and must reject
			// invalid half/sparse defaults before publishing the schema. Existing
			// dense/default handling remains unchanged.
			_, half := sqlast.HalfDimensions(col.Type)
			_, sparse := sqlast.SparseDimensions(col.Type)
			if (!half && !sparse) || root == 0 {
				if _, err := storedColumnValue(value, col.Type); err != nil {
					return nil, err
				}
			}
		}
	}
	return &writeTable{table: tab}, nil
}
func writableIndex(def *sqlast.CreateIndex, tab *writeTable, root uint32) (writeIndex, error) {
	idx := writeIndex{name: def.Name, root: root}
	if def.Method != "" {
		return idx, fmt.Errorf("%w: extension syntax in sqlite_schema", ErrUnsupported)
	}
	if len(def.Columns) == 0 {
		return idx, fmt.Errorf("index requires columns")
	}
	for i, name := range def.Columns {
		if i < len(def.Desc) && def.Desc[i] {
			return idx, fmt.Errorf("%w: descending indexes", ErrUnsupported)
		}
		column, ok := tab.index[fold(name)]
		if !ok {
			return idx, fmt.Errorf("no such column: %s", name)
		}
		idx.columns = append(idx.columns, column)
	}
	return idx, nil
}
func newSchemaName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: empty schema object name", ErrUnsupported)
	}
	if strings.HasPrefix(fold(name), "sqlite_") || extensionReserved(name) {
		return fmt.Errorf("%w: reserved schema object name", ErrUnsupported)
	}
	return nil
}
func (t *Tx) createTable(ctx context.Context, c *writeCatalog, def *sqlast.CreateTable) error {
	if err := newSchemaName(def.Name); err != nil {
		return err
	}
	if typ, exists := c.names[fold(def.Name)]; exists {
		if def.IfNotExists && typ == "table" {
			return nil
		}
		return fmt.Errorf("object already exists: %s", def.Name)
	}
	tab, err := writableTable(def, 0)
	if err != nil {
		return err
	}
	root, err := btree.CreateTree(ctx, t.storage, btree.TableTree)
	if err != nil {
		return err
	}
	if err = t.addSchema(ctx, "table", def.Name, def.Name, root, def.SQL); err != nil {
		return err
	}
	tab.root = root
	c.tables[fold(def.Name)] = tab
	c.names[fold(def.Name)] = "table"
	return nil
}
func (t *Tx) createIndex(ctx context.Context, c *writeCatalog, def *sqlast.CreateIndex) error {
	if err := newSchemaName(def.Name); err != nil {
		return err
	}
	if typ, exists := c.names[fold(def.Name)]; exists {
		if def.IfNotExists && typ == "index" {
			return nil
		}
		return fmt.Errorf("object already exists: %s", def.Name)
	}
	if extensionReserved(def.Table) {
		return fmt.Errorf("%w: owned extension table", ErrUnsupported)
	}
	tab := c.tables[fold(def.Table)]
	if tab == nil {
		return fmt.Errorf("no such table: %s", def.Table)
	}
	idx, err := writableIndex(def, tab, 0)
	if err != nil {
		return err
	}
	root, err := btree.CreateTree(ctx, t.storage, btree.IndexTree)
	if err != nil {
		return err
	}
	idx.root = root
	cursor, err := btree.NewCursor(ctx, t.reader, tab.root)
	if err != nil {
		return err
	}
	defer cursor.Close()
	for {
		rowid, raw, err := cursor.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		values, err := tab.values(rowid, raw)
		if err != nil {
			return err
		}
		if err = btree.InsertIndex(ctx, t.storage, root, btree.KeyInfo{Columns: len(idx.columns)}, idx.key(rowid, values)); err != nil {
			return err
		}
	}
	return t.addSchema(ctx, "index", def.Name, tab.name, root, def.SQL)
}
func (t *Tx) addSchema(ctx context.Context, typ, name, table string, root uint32, sql string) error {
	id, err := t.nextRowID(ctx, 1)
	if err != nil {
		return err
	}
	if err = btree.InsertTable(ctx, t.storage, 1, id, []any{typ, name, table, int64(root), sql}); err != nil {
		return err
	}
	return t.storage.BumpSchemaCookie(ctx)
}
func (idx writeIndex) key(rowid int64, values []any) []any {
	key := make([]any, len(idx.columns)+1)
	for i, col := range idx.columns {
		key[i] = values[col]
	}
	key[len(idx.columns)] = rowid
	return key
}

// values converts the physical record to the SQL row including rowid alias,
// affinity of missing-column defaults, and REAL readback representation.
func (t *writeTable) values(rowid int64, raw []any) ([]any, error) {
	if len(raw) > len(t.columns) {
		return nil, fmt.Errorf("%w: record has too many columns", ErrCorrupt)
	}
	result := make([]any, len(t.columns))
	copy(result, raw)
	for i := len(raw); i < len(result); i++ {
		if t.columns[i].Default != nil {
			if !isConstantDefault(t.columns[i].Default) {
				return nil, fmt.Errorf("%w: missing column nonliteral default", ErrUnsupported)
			}
			value, err := sqlast.Eval(t.columns[i].Default, &sqlast.EvalContext{})
			if err != nil {
				return nil, err
			}
			value, err = missingColumnValue(value, t.columns[i].Type)
			if err != nil {
				return nil, err
			}
			result[i] = value
		}
	}
	if t.ipk >= 0 {
		result[t.ipk] = rowid
	}
	for i, col := range t.columns {
		if affinity(col.Type) == "REAL" {
			if n, ok := result[i].(int64); ok {
				result[i] = float64(n)
			}
		}
	}
	return result, nil
}
