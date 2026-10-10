package sqlite

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/ms-xiaosnow/gosqlite/extension/grams13"
	"github.com/ms-xiaosnow/gosqlite/extension/registry"
	"github.com/ms-xiaosnow/gosqlite/extension/vector/ann"
	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// ErrReindexRequired is returned before native writes when an external writer,
// changed schema, or missing guard makes extension maintenance unsafe.
var ErrReindexRequired = errors.New("extension index requires REINDEX")

const extensionPrefix = "__gosqlite_ext_"
const extensionStorageVersion = 1
const extensionCatalogName = extensionPrefix + "catalog"
const extensionFieldsName = extensionPrefix + "fields"
const extensionPostingsName = extensionPrefix + "postings"
const extensionFrequencyName = extensionPrefix + "frequency"
const extensionFieldsLookup = extensionPrefix + "fields_lookup"
const extensionPostingsLookup = extensionPrefix + "postings_lookup"
const extensionFrequencyLookup = extensionPrefix + "frequency_lookup"

// These are legal ordinary SQLite schemas. Exact bytes and all object names
// are part of the owned storage format, not an authorization by name prefix.
type extensionSchemaSpec struct{ typ, name, table, sql string }

func extensionSchemas() []extensionSchemaSpec {
	return []extensionSchemaSpec{
		{"table", extensionCatalogName, extensionCatalogName, "CREATE TABLE " + extensionCatalogName + " (id INTEGER PRIMARY KEY, name TEXT NOT NULL, source_table TEXT NOT NULL, source_hash TEXT NOT NULL, method TEXT NOT NULL, method_version INTEGER NOT NULL, storage_version INTEGER NOT NULL, config TEXT NOT NULL, config_hash TEXT NOT NULL, generation INTEGER NOT NULL, needs_reindex INTEGER NOT NULL, source_rows INTEGER NOT NULL, schema_cookie INTEGER NOT NULL)"},
		{"table", extensionFieldsName, extensionFieldsName, "CREATE TABLE " + extensionFieldsName + " (id INTEGER PRIMARY KEY, index_id INTEGER NOT NULL, field_id INTEGER NOT NULL, column_name TEXT NOT NULL, column_ordinal INTEGER NOT NULL)"},
		{"table", extensionPostingsName, extensionPostingsName, "CREATE TABLE " + extensionPostingsName + " (id INTEGER PRIMARY KEY, index_id INTEGER NOT NULL, field_id INTEGER NOT NULL, namespace TEXT NOT NULL, gram_length INTEGER NOT NULL, gram TEXT NOT NULL, content_rowid INTEGER NOT NULL)"},
		{"table", extensionFrequencyName, extensionFrequencyName, "CREATE TABLE " + extensionFrequencyName + " (id INTEGER PRIMARY KEY, index_id INTEGER NOT NULL, field_id INTEGER NOT NULL, namespace TEXT NOT NULL, gram_length INTEGER NOT NULL, gram TEXT NOT NULL, documents INTEGER NOT NULL)"},
		{"index", extensionFieldsLookup, extensionFieldsName, "CREATE INDEX " + extensionFieldsLookup + " ON " + extensionFieldsName + " (index_id, field_id)"},
		{"index", extensionPostingsLookup, extensionPostingsName, "CREATE INDEX " + extensionPostingsLookup + " ON " + extensionPostingsName + " (index_id, field_id, namespace, gram_length, gram, content_rowid)"},
		{"index", extensionFrequencyLookup, extensionFrequencyName, "CREATE INDEX " + extensionFrequencyLookup + " ON " + extensionFrequencyName + " (index_id, field_id, namespace, gram_length, gram)"},
	}
}

type extensionField struct {
	id      int64
	name    string
	ordinal int
}
type extensionIndex struct {
	schemaCookie                          uint32
	id                                    int64
	name, sourceTable, sourceHash, method string
	methodVersion                         int
	generation, documentCount             int64
	canonical                             []byte
	instance                              registry.Index
	ann                                   *ann.Config
	fields                                []extensionField
	likeConfig, matchConfig               grams13.Config
	usable                                bool
	fallback                              string
	catalog                               *extensionCatalog
}
type extensionCatalog struct {
	indexes []*extensionIndex
	roots   map[string]uint32
	guards  map[string]btree.SchemaEntry
	entries map[string]btree.SchemaEntry
}

func extensionHash(s string) string      { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }
func quoteExtensionName(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
func extensionGuardName(id int64, operation string) string {
	return fmt.Sprintf("%sguard_%d_%s", extensionPrefix, id, strings.ToLower(operation))
}
func extensionGuardSQL(id int64, table, operation string) string {
	return fmt.Sprintf("CREATE TRIGGER %s AFTER %s ON %s BEGIN UPDATE %s SET needs_reindex = 1 WHERE id = %d; END", quoteExtensionName(extensionGuardName(id, operation)), operation, quoteExtensionName(table), quoteExtensionName(extensionCatalogName), id)
}
func extensionReserved(name string) bool { return strings.HasPrefix(fold(name), extensionPrefix) }
func extensionCorrupt(detail string) error {
	return fmt.Errorf("%w: extension storage: %s", ErrCorrupt, detail)
}

func (d *DB) loadExtensions(ctx context.Context, reader btree.PageReader) (*extensionCatalog, error) {
	entries, err := btree.ReadSchema(ctx, reader)
	if err != nil {
		return nil, err
	}
	return d.loadExtensionsEntries(ctx, reader, entries)
}
func (d *DB) loadExtensionsEntries(ctx context.Context, reader btree.PageReader, entries []btree.SchemaEntry) (*extensionCatalog, error) {
	c := &extensionCatalog{roots: map[string]uint32{}, guards: map[string]btree.SchemaEntry{}, entries: map[string]btree.SchemaEntry{}}
	reserved := 0
	for _, e := range entries {
		key := fold(e.Name)
		if _, exists := c.entries[key]; exists {
			return nil, extensionCorrupt("duplicate schema name")
		}
		c.entries[key] = e
		if extensionReserved(e.Name) {
			reserved++
		}
	}
	if reserved == 0 {
		return c, nil
	}
	seenRoots := map[uint32]bool{}
	for _, e := range entries {
		if e.RootPage > 0 {
			if seenRoots[e.RootPage] {
				return nil, extensionCorrupt("shared schema root")
			}
			seenRoots[e.RootPage] = true
		}
	}
	specs := extensionSchemas()
	for _, s := range annSchemas() {
		if _, exists := c.entries[s.name]; exists {
			specs = append(specs, annSchemas()...)
			break
		}
	}
	recognized := map[string]bool{}
	for _, s := range specs {
		e, ok := c.entries[s.name]
		if !ok || e.Name != s.name || e.Type != s.typ || e.TableName != s.table || e.SQL != s.sql || e.RootPage < 2 {
			return nil, fmt.Errorf("%w: unrecognized or incomplete extension object %q", ErrUnsupported, s.name)
		}
		c.roots[s.name] = e.RootPage
		recognized[s.name] = true
	}
	names := map[string]bool{}
	byID := map[int64]*extensionIndex{}
	bytes := int64(0)
	err := btree.Scan(ctx, reader, c.roots[extensionCatalogName], func(id int64, v []any) error {
		if len(c.indexes) >= 1024 {
			return ErrMemoryLimit
		}
		if id <= 0 || len(v) != 13 || v[0] != nil {
			return extensionCorrupt("catalog row shape")
		}
		strs := make([]string, 0, 6)
		for _, p := range []int{1, 2, 3, 4, 7, 8} {
			s, ok := v[p].(string)
			if !ok {
				return extensionCorrupt("catalog text field")
			}
			strs = append(strs, s)
			bytes += int64(len(s)) + 64
		}
		if bytes > d.opts.MaxSortBytes {
			return ErrMemoryLimit
		}
		mv, mok := v[5].(int64)
		sv, sok := v[6].(int64)
		generation, gok := v[9].(int64)
		dirty, dok := v[10].(int64)
		count, cok := v[11].(int64)
		cookie, cookieOK := v[12].(int64)
		if !mok || !sok || !gok || !dok || !cok || !cookieOK || cookie < 0 || cookie > 4294967295 || generation < 1 || count < 0 || (dirty != 0 && dirty != 1) {
			return extensionCorrupt("catalog integer field")
		}
		if mv <= 0 || int64(int(mv)) != mv {
			return fmt.Errorf("%w: extension method version %d", ErrUnsupported, mv)
		}
		if sv != extensionStorageVersion {
			return fmt.Errorf("%w: extension storage version %d", ErrUnsupported, sv)
		}
		if strs[0] == "" || extensionReserved(strs[0]) || strings.HasPrefix(fold(strs[0]), "sqlite_") || names[fold(strs[0])] {
			return extensionCorrupt("logical index name")
		}
		if _, exists := c.entries[fold(strs[0])]; exists {
			return extensionCorrupt("logical index collides with schema")
		}
		names[fold(strs[0])] = true
		configHashMismatch := extensionHash(strs[4]) != strs[5]
		var instance registry.Index
		var annConfig *ann.Config
		if strs[3] == ann.HNSW {
			if mv != ann.MethodVersion || c.roots[annStateName] == 0 {
				return fmt.Errorf("%w: ANN method/storage version", ErrUnsupported)
			}
			cfg, err := ann.ParseConfig([]byte(strs[4]))
			if err != nil {
				return fmt.Errorf("%w: ANN configuration: %w", ErrUnsupported, err)
			}
			if cfg.Method != strs[3] {
				return extensionCorrupt("ANN configuration method differs")
			}
			annConfig = &cfg
		} else {
			method, ok := d.methods.Lookup(strs[3], int(mv))
			if !ok {
				return fmt.Errorf("%w: unregistered index method %s version %d", ErrUnsupported, strs[3], mv)
			}
			var err error
			instance, err = method.Configure([]byte(strs[4]))
			if err != nil {
				return fmt.Errorf("%w: index %q configuration: %w", ErrUnsupported, strs[0], err)
			}
			if instance == nil || string(instance.Canonical()) != strs[4] {
				return extensionCorrupt("method returned a different configuration")
			}
		}
		x := &extensionIndex{id: id, name: strs[0], sourceTable: strs[1], sourceHash: strs[2], method: strs[3], methodVersion: int(mv), canonical: []byte(strs[4]), instance: instance, ann: annConfig, generation: generation, documentCount: count, schemaCookie: uint32(cookie), usable: true, catalog: c}
		if x.method == grams13.MethodName {
			like, match, parseErr := grams13.ParseIndexConfig(x.canonical)
			if parseErr != nil {
				return parseErr
			}
			x.likeConfig, x.matchConfig = like, match
		}
		if configHashMismatch {
			x.usable = false
			x.fallback = "configuration hash changed; REINDEX required"
		}
		if uint32(cookie) != reader.Header().SchemaCookie {
			x.usable = false
			x.fallback = "schema epoch changed; REINDEX required"
		}
		if dirty == 1 {
			x.usable = false
			x.fallback = "external writer marked index dirty; REINDEX required"
		}
		source, exists := c.entries[fold(x.sourceTable)]
		if !exists || source.Type != "table" || extensionReserved(x.sourceTable) {
			x.usable = false
			x.fallback = "source table is missing or changed; REINDEX required"
		} else if extensionHash(source.SQL) != x.sourceHash {
			x.usable = false
			x.fallback = "source schema fingerprint changed; REINDEX required"
		}
		c.indexes = append(c.indexes, x)
		byID[id] = x
		return nil
	})
	if err != nil {
		return nil, err
	}
	err = btree.Scan(ctx, reader, c.roots[extensionFieldsName], func(_ int64, v []any) error {
		if len(v) != 5 || v[0] != nil {
			return extensionCorrupt("field row shape")
		}
		index, ok1 := v[1].(int64)
		field, ok2 := v[2].(int64)
		name, ok3 := v[3].(string)
		ordinal, ok4 := v[4].(int64)
		x := byID[index]
		if !ok1 || !ok2 || !ok3 || !ok4 || x == nil || field < 1 || field > 256 || ordinal < 0 || ordinal > 32767 || name == "" {
			return extensionCorrupt("field definition")
		}
		if len(x.fields) >= 256 {
			return ErrMemoryLimit
		}
		for _, f := range x.fields {
			if f.id == field || fold(f.name) == fold(name) {
				return extensionCorrupt("duplicate indexed field")
			}
		}
		bytes += int64(len(name)) + 128
		if bytes > d.opts.MaxSortBytes {
			return ErrMemoryLimit
		}
		x.fields = append(x.fields, extensionField{field, name, int(ordinal)})
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, x := range c.indexes {
		if x.ann != nil && len(x.fields) != 1 {
			return nil, extensionCorrupt("ANN requires one vector field")
		}
		if len(x.fields) == 0 {
			return nil, extensionCorrupt("index has no fields")
		}
		source := c.entries[fold(x.sourceTable)]
		if source.Type == "table" {
			parsed, parseErr := sqlast.Parse(source.SQL)
			def, ok := parsed.(*sqlast.CreateTable)
			if parseErr != nil || !ok {
				x.usable = false
				x.fallback = "source schema cannot be resolved; REINDEX required"
			} else {
				for _, f := range x.fields {
					if f.ordinal >= len(def.Columns) || fold(def.Columns[f.ordinal].Name) != fold(f.name) {
						x.usable = false
						x.fallback = "indexed field changed; REINDEX required"
					} else if x.ann != nil {
						dims, ok := sqlast.VectorDimensions(def.Columns[f.ordinal].Type)
						if !ok || dims != x.ann.Dimensions {
							x.usable = false
							x.fallback = "indexed vector type changed; REINDEX required"
						}
					}
				}
			}
		}
		for _, op := range []string{"INSERT", "UPDATE", "DELETE"} {
			name := extensionGuardName(x.id, op)
			e, exists := c.entries[name]
			if !exists {
				x.usable = false
				x.fallback = "external-writer guard missing; REINDEX required"
				continue
			}
			if e.Name != name || e.Type != "trigger" || e.TableName != x.sourceTable || e.RootPage != 0 || e.SQL != extensionGuardSQL(x.id, x.sourceTable, op) {
				x.usable = false
				x.fallback = "external-writer guard changed; REINDEX required"
				recognized[name] = true
				continue
			}
			recognized[name] = true
			c.guards[name] = e
		}
	}
	for name, e := range c.entries {
		if extensionReserved(e.Name) && !recognized[name] {
			return nil, fmt.Errorf("%w: unknown owned-prefix object %q", ErrUnsupported, e.Name)
		}
	}
	return c, nil
}

func (d *DB) gramIndexes(ctx context.Context, reader btree.PageReader, tab *table) ([]*extensionIndex, string, error) {
	c, err := d.loadExtensions(ctx, reader)
	if err != nil {
		return nil, "", err
	}
	var indexes []*extensionIndex
	fallback := ""
	if tab == nil {
		return indexes, fallback, nil
	}
	for _, x := range c.indexes {
		if fold(x.sourceTable) == fold(tab.name) && x.method == grams13.MethodName {
			if x.usable {
				indexes = append(indexes, x)
			} else {
				fallback = x.fallback
			}
		}
	}
	return indexes, fallback, nil
}

// rangeOne closes before returning. Callers may safely mutate the same tree.
func extensionRangeOne(ctx context.Context, reader btree.PageReader, root uint32, columns int, prefix []any) ([]any, bool, error) {
	cur, err := btree.NewIndexRangeCursor(ctx, reader, root, btree.KeyInfo{Columns: columns}, prefix, prefix)
	if err != nil {
		return nil, false, err
	}
	defer cur.Close()
	key, err := cur.Next()
	if err == io.EOF {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	_, err = cur.Next()
	if err != io.EOF {
		if err != nil {
			return nil, false, err
		}
		return nil, false, extensionCorrupt("duplicate logical key")
	}
	return key, true, nil
}
