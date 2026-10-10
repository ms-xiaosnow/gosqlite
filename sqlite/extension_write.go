package sqlite

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/ms-xiaosnow/gosqlite/extension/grams13"
	"github.com/ms-xiaosnow/gosqlite/extension/registry"
	"github.com/ms-xiaosnow/gosqlite/extension/vector/ann"
	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

func (t *Tx) initializeExtensionStorage(ctx context.Context, c *writeCatalog) error {
	if len(c.extensions.roots) > 0 {
		return nil
	}
	for _, s := range extensionSchemas() {
		kind := btree.TableTree
		if s.typ == "index" {
			kind = btree.IndexTree
		}
		root, err := btree.CreateTree(ctx, t.storage, kind)
		if err != nil {
			return err
		}
		if err = t.addSchema(ctx, s.typ, s.name, s.table, root, s.sql); err != nil {
			return err
		}
		c.extensions.roots[s.name] = root
		c.names[s.name] = s.typ
		parsed, err := sqlast.Parse(s.sql)
		if err != nil {
			return err
		}
		if s.typ == "table" {
			tab, err := writableTable(parsed.(*sqlast.CreateTable), root)
			if err != nil {
				return err
			}
			c.tables[s.name] = tab
		} else {
			tab := c.tables[s.table]
			idx, err := writableIndex(parsed.(*sqlast.CreateIndex), tab, root)
			if err != nil {
				return err
			}
			tab.indexes = append(tab.indexes, idx)
		}
	}
	return nil
}
func (t *Tx) insertExtensionRow(ctx context.Context, c *writeCatalog, table string, values []any) (int64, error) {
	tab := c.tables[table]
	id, err := t.nextRowID(ctx, tab.root)
	if err != nil {
		return 0, err
	}
	values[0] = id
	if err = btree.InsertTable(ctx, t.storage, tab.root, id, tab.physical(values)); err != nil {
		return 0, err
	}
	if err = t.addIndexes(ctx, tab, id, values); err != nil {
		return 0, err
	}
	return id, nil
}
func (t *Tx) replaceExtensionRow(ctx context.Context, c *writeCatalog, table string, id int64, values []any) error {
	tab := c.tables[table]
	raw, exists, err := btree.SeekTable(ctx, t.reader, tab.root, id)
	if err != nil {
		return err
	}
	if !exists {
		return extensionCorrupt("missing row during replacement")
	}
	old, err := tab.values(id, raw)
	if err != nil {
		return err
	}
	if err = t.removeIndexes(ctx, tab, id, old); err != nil {
		return err
	}
	values[0] = id
	if err = btree.ReplaceTable(ctx, t.storage, tab.root, id, tab.physical(values)); err != nil {
		return err
	}
	return t.addIndexes(ctx, tab, id, values)
}
func (t *Tx) deleteExtensionRow(ctx context.Context, c *writeCatalog, table string, id int64) error {
	tab := c.tables[table]
	raw, exists, err := btree.SeekTable(ctx, t.reader, tab.root, id)
	if err != nil {
		return err
	}
	if !exists {
		return extensionCorrupt("missing shadow row")
	}
	values, err := tab.values(id, raw)
	if err != nil {
		return err
	}
	if err = t.removeIndexes(ctx, tab, id, values); err != nil {
		return err
	}
	found, err := btree.DeleteTable(ctx, t.storage, tab.root, id)
	if err == nil && !found {
		return extensionCorrupt("missing row during deletion")
	}
	return err
}
func (x *extensionIndex) values(dirty int64) []any {
	return []any{x.id, x.name, x.sourceTable, x.sourceHash, x.method, int64(x.methodVersion), int64(extensionStorageVersion), string(x.canonical), extensionHash(string(x.canonical)), x.generation, dirty, x.documentCount, int64(x.schemaCookie)}
}
func (t *Tx) createMethodIndex(ctx context.Context, c *writeCatalog, name, tableName string, columns []string, methodName string, version int, canonical []byte, ifNotExists bool) error {
	if name == "" || len(name) > 1024 || !utf8.ValidString(name) || strings.ContainsRune(name, 0) {
		return fmt.Errorf("invalid extension index name")
	}
	if err := newSchemaName(name); err != nil {
		return err
	}
	if typ, exists := c.names[fold(name)]; exists {
		if ifNotExists && typ == "extension index" {
			return nil
		}
		return fmt.Errorf("object already exists: %s", name)
	}
	if extensionReserved(tableName) {
		return fmt.Errorf("%w: cannot index owned extension tables", ErrUnsupported)
	}
	tab := c.tables[fold(tableName)]
	if tab == nil {
		return fmt.Errorf("no such table: %s", tableName)
	}
	if len(columns) == 0 || len(columns) > 256 {
		return fmt.Errorf("%w: extension index requires 1..256 fields", ErrUnsupported)
	}
	if len(canonical) > 16384 {
		return ErrMemoryLimit
	}
	var instance registry.Index
	var annConfig *ann.Config
	var err error
	if methodName == ann.HNSW {
		if version != ann.MethodVersion {
			return fmt.Errorf("%w: ANN method version", ErrUnsupported)
		}
		cfg, parseErr := ann.ParseConfig(canonical)
		if parseErr != nil {
			return parseErr
		}
		if cfg.Method != methodName {
			return ann.ErrConfig
		}
		annConfig = &cfg
	} else {
		method, ok := t.db.methods.Lookup(methodName, version)
		if !ok {
			return fmt.Errorf("%w: unregistered index method %s version %d", ErrUnsupported, methodName, version)
		}
		instance, err = method.Configure(canonical)
		if err != nil {
			return err
		}
		if instance == nil || string(instance.Canonical()) != string(canonical) {
			return fmt.Errorf("%w: noncanonical method configuration", ErrUnsupported)
		}
	}
	var fields []extensionField
	seen := map[int]bool{}
	for _, name := range columns {
		if name == "" {
			return fmt.Errorf("%w: empty indexed field name", ErrUnsupported)
		}
		ordinal, exists := tab.index[fold(name)]
		if !exists {
			return fmt.Errorf("no such column: %s", name)
		}
		if seen[ordinal] {
			return fmt.Errorf("duplicate indexed field: %s", name)
		}
		seen[ordinal] = true
		fields = append(fields, extensionField{int64(len(fields) + 1), tab.columns[ordinal].Name, ordinal})
	}
	if annConfig != nil {
		if len(fields) != 1 {
			return fmt.Errorf("%w: ANN requires one vector field", ErrUnsupported)
		}
		dims, ok := sqlast.VectorDimensions(tab.columns[fields[0].ordinal].Type)
		if !ok || dims != annConfig.Dimensions {
			return fmt.Errorf("%w: ANN vector dimensions", ErrMismatch)
		}
	}
	if len(c.extensions.indexes) >= 1024 {
		return ErrMemoryLimit
	}
	if err = t.initializeExtensionStorage(ctx, c); err != nil {
		return err
	}
	if annConfig != nil {
		if err = t.initializeANNStorage(ctx, c); err != nil {
			return err
		}
	}
	source, ok := c.extensions.entries[fold(tab.name)]
	if !ok {
		return extensionCorrupt("source schema missing")
	}
	x := &extensionIndex{name: name, sourceTable: tab.name, sourceHash: extensionHash(source.SQL), method: methodName, methodVersion: version, generation: 1, schemaCookie: t.reader.Header().SchemaCookie, canonical: append([]byte(nil), canonical...), instance: instance, ann: annConfig, fields: fields, catalog: c.extensions}
	x.id, err = t.insertExtensionRow(ctx, c, extensionCatalogName, x.values(1))
	if err != nil {
		return err
	}
	for _, f := range fields {
		if _, err = t.insertExtensionRow(ctx, c, extensionFieldsName, []any{nil, x.id, f.id, f.name, int64(f.ordinal)}); err != nil {
			return err
		}
	}
	// Re-read staged metadata before building: creation must fit the same
	// connection budget used to reopen it, including all existing indexes.
	if _, err = t.db.loadExtensions(ctx, t.reader); err != nil {
		return err
	}
	if err = t.buildExtension(ctx, c, x, tab); err != nil {
		return err
	}
	if err = t.ensureExtensionGuards(ctx, x); err != nil {
		return err
	}
	x.usable = true
	x.schemaCookie = t.reader.Header().SchemaCookie
	c.extensions.indexes = append(c.extensions.indexes, x)
	return t.replaceExtensionRow(ctx, c, extensionCatalogName, x.id, x.values(0))
}
func (t *Tx) ensureExtensionGuards(ctx context.Context, x *extensionIndex) error {
	for _, op := range []string{"INSERT", "UPDATE", "DELETE"} {
		name := extensionGuardName(x.id, op)
		if e, exists := x.catalog.entries[name]; exists {
			if e.Type != "trigger" || e.Name != name || e.TableName != x.sourceTable || e.RootPage != 0 || e.SQL != extensionGuardSQL(x.id, x.sourceTable, op) {
				return fmt.Errorf("%w: changed guard %q must be removed before REINDEX", ErrReindexRequired, name)
			}
			continue
		}
		if err := t.addSchema(ctx, "trigger", name, x.sourceTable, 0, extensionGuardSQL(x.id, x.sourceTable, op)); err != nil {
			return err
		}
	}
	return nil
}
func (t *Tx) buildExtension(ctx context.Context, c *writeCatalog, x *extensionIndex, tab *writeTable) error {
	if x.ann != nil {
		if err := t.validateANNField(x, tab); err != nil {
			return err
		}
		if err := newANNWriteStore(t, c, x).putState(ctx, ann.NewState()); err != nil {
			return err
		}
	}
	cur, err := btree.NewCursor(ctx, t.reader, tab.root)
	if err != nil {
		return err
	}
	defer cur.Close()
	x.documentCount = 0
	for {
		id, raw, err := cur.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		values, err := tab.values(id, raw)
		if err != nil {
			return err
		}
		if err = t.applyExtensionDelta(ctx, c, x, 0, nil, id, values); err != nil {
			return err
		}
		if x.documentCount == math.MaxInt64 {
			return ErrFull
		}
		x.documentCount++
	}
}
func extensionTokenError(err error) error {
	if errors.Is(err, grams13.ErrLimit) {
		return fmt.Errorf("%w: %w", ErrMemoryLimit, err)
	}
	if errors.Is(err, grams13.ErrInvalidUTF8) || errors.Is(err, grams13.ErrStreamSafe) {
		return fmt.Errorf("%w: indexed field: %w", ErrConstraint, err)
	}
	return err
}
func (t *Tx) extensionKeys(ctx context.Context, x *extensionIndex, value any) (map[registry.Key]bool, int64, error) {
	result := map[registry.Key]bool{}
	if value == nil {
		return result, 0, nil
	}
	text, err := sqlast.LikeText(value)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: indexed field: %w", ErrConstraint, err)
	}
	keys, err := x.instance.Keys(ctx, text)
	if err != nil {
		return nil, 0, extensionTokenError(err)
	}
	var size int64
	for _, k := range keys {
		if err := ctx.Err(); err != nil {
			return nil, 0, err
		}
		if k.Namespace == "" || len(k.Namespace) > 128 || k.Length <= 0 || k.Length > 32767 || !utf8.ValidString(k.Text) {
			return nil, 0, fmt.Errorf("%w: invalid method key", ErrConstraint)
		}
		size += int64(len(k.Namespace)+len(k.Text)) + 128
		if size > t.db.opts.MaxSortBytes {
			return nil, 0, ErrMemoryLimit
		}
		if result[k] {
			return nil, 0, fmt.Errorf("%w: duplicate method key", ErrConstraint)
		}
		result[k] = true
	}
	return result, size, nil
}
func sortedExtensionKeys(keys map[registry.Key]bool) []registry.Key {
	result := make([]registry.Key, 0, len(keys))
	for k := range keys {
		result = append(result, k)
	}
	sort.Slice(result, func(i, j int) bool {
		a, b := result[i], result[j]
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		if a.Length != b.Length {
			return a.Length < b.Length
		}
		return a.Text < b.Text
	})
	return result
}
func extensionKeyPrefix(x *extensionIndex, f extensionField, k registry.Key) []any {
	return []any{x.id, f.id, k.Namespace, int64(k.Length), k.Text}
}
func (t *Tx) applyExtensionDelta(ctx context.Context, c *writeCatalog, x *extensionIndex, oldID int64, old []any, newID int64, next []any) error {
	if x.ann != nil {
		return t.applyANNDelta(ctx, c, x, oldID, old, newID, next)
	}
	for _, f := range x.fields {
		var oldValue, newValue any
		if old != nil {
			oldValue = old[f.ordinal]
		}
		if next != nil {
			newValue = next[f.ordinal]
		}
		before, oldSize, err := t.extensionKeys(ctx, x, oldValue)
		if err != nil {
			return err
		}
		after, newSize, err := t.extensionKeys(ctx, x, newValue)
		if err != nil {
			return err
		}
		if oldSize+newSize > t.db.opts.MaxSortBytes {
			return ErrMemoryLimit
		}
		for _, key := range sortedExtensionKeys(before) {
			if oldID == newID && after[key] {
				continue
			}
			if err = t.changeExtensionPosting(ctx, c, x, f, key, oldID, -1); err != nil {
				return err
			}
		}
		for _, key := range sortedExtensionKeys(after) {
			if oldID == newID && before[key] {
				continue
			}
			if err = t.changeExtensionPosting(ctx, c, x, f, key, newID, 1); err != nil {
				return err
			}
		}
	}
	return nil
}
func (t *Tx) applyExtensions(ctx context.Context, c *writeCatalog, tab *writeTable, oldID int64, old []any, newID int64, next []any) error {
	for _, x := range tab.extensions {
		if !x.usable {
			return fmt.Errorf("%w: %s", ErrReindexRequired, x.name)
		}
		if err := t.applyExtensionDelta(ctx, c, x, oldID, old, newID, next); err != nil {
			return err
		}
		if old == nil {
			if x.documentCount == math.MaxInt64 {
				return ErrFull
			}
			x.documentCount++
		} else if next == nil {
			if x.documentCount == 0 {
				return extensionCorrupt("source row count underflow")
			}
			x.documentCount--
		}
		if err := t.replaceExtensionRow(ctx, c, extensionCatalogName, x.id, x.values(0)); err != nil {
			return err
		}
	}
	return nil
}
func (t *Tx) changeExtensionPosting(ctx context.Context, c *writeCatalog, x *extensionIndex, f extensionField, k registry.Key, rowid, delta int64) error {
	prefix := extensionKeyPrefix(x, f, k)
	postingPrefix := append(append([]any(nil), prefix...), rowid)
	entry, exists, err := extensionRangeOne(ctx, t.reader, c.extensions.roots[extensionPostingsLookup], 6, postingPrefix)
	if err != nil {
		return err
	}
	if delta < 0 {
		if !exists {
			return extensionCorrupt("missing logical posting")
		}
		id, ok := entry[6].(int64)
		if !ok {
			return extensionCorrupt("posting id")
		}
		if err = t.deleteExtensionRow(ctx, c, extensionPostingsName, id); err != nil {
			return err
		}
	} else {
		if exists {
			return extensionCorrupt("duplicate logical posting")
		}
		values := append([]any{nil}, postingPrefix...)
		if _, err = t.insertExtensionRow(ctx, c, extensionPostingsName, values); err != nil {
			return err
		}
	}
	frequency, found, err := extensionRangeOne(ctx, t.reader, c.extensions.roots[extensionFrequencyLookup], 5, prefix)
	if err != nil {
		return err
	}
	if !found {
		if delta < 0 {
			return extensionCorrupt("missing frequency")
		}
		values := append([]any{nil}, prefix...)
		values = append(values, int64(1))
		_, err = t.insertExtensionRow(ctx, c, extensionFrequencyName, values)
		return err
	}
	id, ok := frequency[5].(int64)
	if !ok {
		return extensionCorrupt("frequency id")
	}
	raw, found, err := btree.SeekTable(ctx, t.reader, c.extensions.roots[extensionFrequencyName], id)
	if err != nil {
		return err
	}
	if !found || len(raw) != 7 {
		return extensionCorrupt("frequency row")
	}
	count, ok := raw[6].(int64)
	if !ok || count <= 0 {
		return extensionCorrupt("invalid frequency count")
	}
	if delta > 0 && count == math.MaxInt64 {
		return ErrFull
	}
	count += delta
	if count == 0 {
		return t.deleteExtensionRow(ctx, c, extensionFrequencyName, id)
	}
	raw[6] = count
	return t.replaceExtensionRow(ctx, c, extensionFrequencyName, id, raw)
}
func (t *Tx) clearExtensionRows(ctx context.Context, c *writeCatalog, x *extensionIndex, table, index string, columns int) error {
	prefix := []any{x.id}
	cur, err := btree.NewIndexRangeCursor(ctx, t.reader, c.extensions.roots[index], btree.KeyInfo{Columns: columns}, prefix, prefix)
	if err != nil {
		return err
	}
	var ids []int64
	for {
		key, err := cur.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			cur.Close()
			return err
		}
		if int64(len(ids)+1) > t.db.opts.MaxSortBytes/32 {
			cur.Close()
			return ErrMemoryLimit
		}
		id, ok := key[len(key)-1].(int64)
		if !ok {
			cur.Close()
			return extensionCorrupt("shadow key rowid")
		}
		ids = append(ids, id)
	}
	cur.Close()
	for _, id := range ids {
		if err := t.deleteExtensionRow(ctx, c, table, id); err != nil {
			return err
		}
	}
	return nil
}
func (t *Tx) reindexExtension(ctx context.Context, c *writeCatalog, name string) error {
	var x *extensionIndex
	for _, candidate := range c.extensions.indexes {
		if fold(candidate.name) == fold(name) {
			x = candidate
			break
		}
	}
	if x == nil {
		return fmt.Errorf("no such extension index: %s", name)
	}
	tab := c.tables[fold(x.sourceTable)]
	if tab == nil {
		return fmt.Errorf("%w: source table missing", ErrReindexRequired)
	}
	for i, f := range x.fields {
		ordinal, exists := tab.index[fold(f.name)]
		if !exists {
			return fmt.Errorf("%w: indexed field %q is missing", ErrReindexRequired, f.name)
		}
		x.fields[i].ordinal = ordinal
	}
	if x.generation == math.MaxInt64 {
		return ErrFull
	}
	if err := t.ensureExtensionGuards(ctx, x); err != nil {
		return err
	}
	if x.ann != nil {
		if err := t.validateANNField(x, tab); err != nil {
			return err
		}
		if err := t.clearANNRows(ctx, c, x); err != nil {
			return err
		}
	}
	if err := t.clearExtensionRows(ctx, c, x, extensionPostingsName, extensionPostingsLookup, 6); err != nil {
		return err
	}
	if err := t.clearExtensionRows(ctx, c, x, extensionFrequencyName, extensionFrequencyLookup, 5); err != nil {
		return err
	}
	if err := t.clearExtensionRows(ctx, c, x, extensionFieldsName, extensionFieldsLookup, 2); err != nil {
		return err
	}
	for _, f := range x.fields {
		if _, err := t.insertExtensionRow(ctx, c, extensionFieldsName, []any{nil, x.id, f.id, f.name, int64(f.ordinal)}); err != nil {
			return err
		}
	}
	x.sourceHash = extensionHash(c.extensions.entries[fold(x.sourceTable)].SQL)
	x.generation++
	if err := t.buildExtension(ctx, c, x, tab); err != nil {
		return err
	}
	x.usable = true
	x.schemaCookie = t.reader.Header().SchemaCookie
	if err := t.replaceExtensionRow(ctx, c, extensionCatalogName, x.id, x.values(0)); err != nil {
		return err
	}
	_, err := t.db.loadExtensions(ctx, t.reader)
	return err
}

// refreshExtensionSchemaCookies blesses only known-clean indexes after our own
// successful schema mutation. External epochs or already-dirty peers are never
// made usable merely by unrelated native DDL.
func (t *Tx) refreshExtensionSchemaCookies(ctx context.Context, c *writeCatalog, before uint32) error {
	current := t.reader.Header().SchemaCookie
	if current == before {
		return nil
	}
	for _, x := range c.extensions.indexes {
		if !x.usable {
			continue
		}
		x.schemaCookie = current
		if err := t.replaceExtensionRow(ctx, c, extensionCatalogName, x.id, x.values(0)); err != nil {
			return err
		}
	}
	return nil
}
