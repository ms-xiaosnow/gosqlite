package sqlite

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"
	"unicode/utf8"

	"github.com/ms-xiaosnow/gosqlite/extension/grams13"
	"github.com/ms-xiaosnow/gosqlite/extension/vector/ann"
	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	"github.com/ms-xiaosnow/gosqlite/internal/pager"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// CheckScope identifies the versioned, fail-closed integrity profile. It is not
// SQLite's PRAGMA integrity_check or a crash/durability/cryptographic audit.
const CheckScope = "gosqlite-supported-profile-v1"

// CheckOptions bounds whole-check page ownership metadata and cumulative logical
// page reads after snapshot acquisition, including catalog reloads, overflow,
// seeks and extension work. Reader/Tx acquisition retains existing pager caps
// and is outside these traversal quotas.
// Zero uses 1,048,576 pages and 16,777,216 reads. MaxPages cannot exceed the
// B-tree ownership limit; MaxPageReads cannot exceed 2^32. Existing record (64 MiB), schema (16 MiB), cursor,
// connection MaxSortBytes and extension-specific limits still apply separately;
// this is not an aggregate heap-byte limit. A quota failure never passes.
type CheckOptions struct {
	MaxPages     uint32
	MaxPageReads uint64
}

// CheckResult is returned only after all checks succeed. Counts include owned
// extension shadow tables and their ordinary B-tree indexes; sqlite_schema is
// checked but excluded from Tables/TableRows. IndexEntries counts each physical
// ordinary index entry once. PageReads includes repeated logical reads, not OS
// I/O. FreePages includes freelist trunks. ReservedPages is the lock-byte page.
type CheckResult struct {
	Scope            string `json:"scope"`
	Pages            uint32 `json:"pages"`
	FreePages        uint32 `json:"free_pages"`
	ReservedPages    uint32 `json:"reserved_pages"`
	Tables           int    `json:"tables"`
	Indexes          int    `json:"indexes"`
	ExtensionIndexes int    `json:"extension_indexes"`
	TableRows        uint64 `json:"table_rows"`
	IndexEntries     uint64 `json:"index_entries"`
	PageReads        uint64 `json:"page_reads"`
}

func checkOptions(options []CheckOptions) (CheckOptions, error) {
	if len(options) > 1 {
		return CheckOptions{}, fmt.Errorf("at most one CheckOptions is accepted")
	}
	var o CheckOptions
	if len(options) == 1 {
		o = options[0]
	}
	if o.MaxPages == 0 {
		o.MaxPages = btree.MaxVisitedPages
	}
	if o.MaxPages > btree.MaxVisitedPages {
		return CheckOptions{}, fmt.Errorf("%w: check page budget exceeds supported maximum", ErrUnsupported)
	}
	if o.MaxPageReads == 0 {
		o.MaxPageReads = 1 << 24
	}
	if o.MaxPageReads > 1<<32 {
		return CheckOptions{}, fmt.Errorf("%w: check read budget exceeds supported maximum", ErrUnsupported)
	}
	return o, nil
}

// Check validates one pinned logical snapshot without promoting a write lease,
// repairing data, creating a journal or changing SQL connection counters. With
// an active transaction it checks that transaction, including its own writes.
// Otherwise it owns and closes a read snapshot before reporting success.
// Unsupported catalog features fail closed, even when normal SELECT can read
// some objects in that database. See docs/native-check.md for the exact profile.
func (d *DB) Check(ctx context.Context, options ...CheckOptions) (CheckResult, error) {
	if err := checkContext(ctx); err != nil {
		return CheckResult{}, wrap("check", err)
	}
	o, err := checkOptions(options)
	if err != nil {
		return CheckResult{}, wrap("check", err)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return CheckResult{}, wrap("check", ErrClosed)
	}
	if d.tx != nil {
		return d.tx.checkLocked(ctx, o)
	}
	r, err := d.openReader(ctx)
	if err != nil {
		return CheckResult{}, wrap("check", err)
	}
	result, err := d.checkSnapshot(ctx, r, o)
	err = errors.Join(err, r.Close())
	if err != nil {
		return CheckResult{}, wrap("check", err)
	}
	return result, nil
}

// Check uses the transaction's pinned snapshot and leaves its lifetime, active
// Rows, savepoints and connection counters unchanged. First access pins a lazy
// transaction through its usual read path, without write promotion.
func (t *Tx) Check(ctx context.Context, options ...CheckOptions) (CheckResult, error) {
	if err := checkContext(ctx); err != nil {
		return CheckResult{}, wrap("check", err)
	}
	o, err := checkOptions(options)
	if err != nil {
		return CheckResult{}, wrap("check", err)
	}
	t.db.mu.Lock()
	defer t.db.mu.Unlock()
	return t.checkLocked(ctx, o)
}
func (t *Tx) checkLocked(ctx context.Context, o CheckOptions) (CheckResult, error) {
	if err := t.check(); err != nil {
		return CheckResult{}, wrap("check", err)
	}
	if err := t.ensureSnapshotLocked(ctx); err != nil {
		return CheckResult{}, wrap("check", err)
	}
	result, err := t.db.checkSnapshot(ctx, t.reader, o)
	if err != nil {
		return CheckResult{}, wrap("check", err)
	}
	return result, nil
}

type checkReader struct {
	btree.PageReader
	header          pager.Header
	maxReads, reads uint64
	txContext       context.Context
}

func (r *checkReader) Header() pager.Header { return r.header }
func (r *checkReader) ReadPage(ctx context.Context, n uint32) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if r.txContext != nil {
		if err := r.txContext.Err(); err != nil {
			return nil, err
		}
	}
	if n == 0 || n > r.header.PageCount {
		return nil, fmt.Errorf("%w: check page outside snapshot", ErrCorrupt)
	}
	if r.reads >= r.maxReads {
		return nil, fmt.Errorf("%w: check logical page-read budget", ErrMemoryLimit)
	}
	r.reads++
	p, err := r.PageReader.ReadPage(ctx, n)
	if err != nil {
		return nil, err
	}
	if len(p) != int(r.header.PageSize) {
		clear(p)
		return nil, fmt.Errorf("%w: check page byte count", ErrCorrupt)
	}
	return p, nil
}

func (d *DB) checkSnapshot(ctx context.Context, source btree.PageReader, o CheckOptions) (CheckResult, error) {
	h := source.Header()
	if h.PageSize < 512 || h.PageSize > 65536 || h.PageSize&(h.PageSize-1) != 0 || h.PageCount == 0 {
		return CheckResult{}, fmt.Errorf("%w: check snapshot geometry", ErrCorrupt)
	}
	if h.PageCount > o.MaxPages {
		return CheckResult{}, fmt.Errorf("%w: check page-ownership budget", ErrMemoryLimit)
	}
	r := &checkReader{PageReader: source, header: h, maxReads: o.MaxPageReads}
	if d.tx != nil {
		r.txContext = d.tx.ctx
	}
	if err := checkLogicalHeader(ctx, r); err != nil {
		return CheckResult{}, err
	}
	// Full integrity checking has a fixed persisted-semantics profile. Never
	// let an application registry redefine a built-in name/version and make its
	// own missing postings pass by returning a different key set.
	validator, err := copyValidator()
	if err != nil {
		return CheckResult{}, err
	}
	validator.opts = d.opts
	validator.opts.IndexMethods = validator.methods
	helper := &Tx{db: validator, reader: r}
	c, err := helper.validateCatalog(ctx)
	if err != nil {
		return CheckResult{}, err
	}
	if h.SchemaFormat == 0 && len(c.names) != 0 {
		return CheckResult{}, fmt.Errorf("%w: nonempty catalog has empty schema format", ErrCorrupt)
	}
	names := make([]string, 0, len(c.tables))
	for name := range c.tables {
		names = append(names, name)
	}
	sort.Strings(names)
	roots := make([]btree.RootSpec, 0, len(c.names))
	result := CheckResult{Scope: CheckScope, Pages: h.PageCount, Tables: len(c.tables), ExtensionIndexes: len(c.extensions.indexes)}
	for _, name := range names {
		tab := c.tables[name]
		roots = append(roots, btree.RootSpec{Page: tab.root, Kind: btree.TableTree})
		sort.Slice(tab.indexes, func(i, j int) bool { return fold(tab.indexes[i].name) < fold(tab.indexes[j].name) })
		for _, idx := range tab.indexes {
			roots = append(roots, btree.RootSpec{Page: idx.root, Kind: btree.IndexTree, KeyInfo: btree.KeyInfo{Columns: len(idx.columns)}})
			result.Indexes++
		}
	}
	free, err := pager.SnapshotFreePages(ctx, r, o.MaxPages)
	if err != nil {
		return CheckResult{}, err
	}
	if err = btree.CheckOwnership(ctx, r, roots, free); err != nil {
		return CheckResult{}, err
	}
	if err = btree.CheckTreeSpace(ctx, r, roots); err != nil {
		return CheckResult{}, err
	}
	result.FreePages = uint32(len(free))
	if uint32(0x40000000/h.PageSize+1) <= h.PageCount {
		result.ReservedPages = 1
	}
	byID := make(map[int64]*extensionIndex, len(c.extensions.indexes))
	for _, x := range c.extensions.indexes {
		if x.method != grams13.MethodName && x.method != ann.HNSW && x.method != ann.IVFFlat {
			return CheckResult{}, fmt.Errorf("%w: extension method outside check profile", ErrUnsupported)
		}
		if !x.usable {
			return CheckResult{}, fmt.Errorf("%w: %s", ErrReindexRequired, x.fallback)
		}
		byID[x.id] = x
	}
	for _, name := range names {
		tab := c.tables[name]
		err = btree.Scan(ctx, r, tab.root, func(id int64, raw []any) error {
			values, err := checkTableValues(tab, id, raw)
			if err != nil {
				return err
			}
			if err = checkShadowOwner(tab.name, raw, byID); err != nil {
				return err
			}
			result.TableRows++
			for _, idx := range tab.indexes {
				_, found, err := btree.SeekIndex(ctx, r, idx.root, btree.KeyInfo{Columns: len(idx.columns)}, idx.key(id, values))
				if err != nil {
					return err
				}
				if !found {
					return fmt.Errorf("%w: table row missing expected index key", ErrCorrupt)
				}
			}
			return nil
		})
		if err != nil {
			return CheckResult{}, err
		}
		for _, idx := range tab.indexes {
			err = btree.ScanIndex(ctx, r, idx.root, btree.KeyInfo{Columns: len(idx.columns)}, func(key []any) error {
				id := key[len(idx.columns)].(int64) // IndexCursor validates arity and rowid storage class.
				raw, found, err := btree.SeekTable(ctx, r, tab.root, id)
				if err != nil {
					return err
				}
				if !found {
					return fmt.Errorf("%w: index rowid has no table row", ErrCorrupt)
				}
				values, err := checkTableValues(tab, id, raw)
				if err != nil {
					return err
				}
				for i, col := range idx.columns {
					cmp, err := sqlast.Compare(key[i], values[col])
					if err != nil {
						return err
					}
					if cmp != 0 {
						return fmt.Errorf("%w: index value differs from table row", ErrCorrupt)
					}
				}
				result.IndexEntries++
				return nil
			})
			if err != nil {
				return CheckResult{}, err
			}
		}
	}
	extensions := append([]*extensionIndex(nil), c.extensions.indexes...)
	sort.Slice(extensions, func(i, j int) bool { return fold(extensions[i].name) < fold(extensions[j].name) })
	for _, x := range extensions {
		if err = validator.checkExtensionIndex(ctx, r, x.name); err != nil {
			return CheckResult{}, err
		}
	}
	if err = ctx.Err(); err != nil {
		return CheckResult{}, err
	}
	if r.txContext != nil {
		if err = r.txContext.Err(); err != nil {
			return CheckResult{}, err
		}
	}
	result.PageReads = r.reads
	return result, nil
}

// Revalidate logical page 1 without treating a WAL/transaction's selected extent
// as the main file's physical extent. Only the known WAL version and the pager's
// authoritative logical count are normalized in this private read buffer.
func checkLogicalHeader(ctx context.Context, r *checkReader) error {
	h := r.Header()
	if h.ReservedBytes != 0 || h.UsableSize != h.PageSize || h.SchemaFormat != 0 && h.SchemaFormat != 4 {
		return fmt.Errorf("%w: check requires unreserved schema format 4", ErrUnsupported)
	}
	if h.ReadVersion != h.WriteVersion || (h.ReadVersion != 1 && h.ReadVersion != 2) {
		return fmt.Errorf("%w: check read/write version", ErrUnsupported)
	}
	p, err := r.ReadPage(ctx, 1)
	if err != nil {
		return err
	}
	defer clear(p)
	if p[18] != h.WriteVersion || p[19] != h.ReadVersion {
		return fmt.Errorf("%w: logical header version mismatch", ErrCorrupt)
	}
	p[18], p[19] = 1, 1
	binary.BigEndian.PutUint32(p[28:], h.PageCount)
	parsed, err := pager.ParseHeader(p, int64(h.PageSize)*int64(h.PageCount))
	if err != nil {
		return err
	}
	parsed.ReadVersion, parsed.WriteVersion = h.ReadVersion, h.WriteVersion
	if parsed != h {
		return fmt.Errorf("%w: logical header metadata mismatch", ErrCorrupt)
	}
	return nil
}

func checkTableValues(tab *writeTable, id int64, raw []any) ([]any, error) {
	if tab.ipk >= 0 && tab.ipk < len(raw) && raw[tab.ipk] != nil {
		return nil, fmt.Errorf("%w: INTEGER PRIMARY KEY decoded record slot is not NULL", ErrCorrupt)
	}
	values, err := tab.values(id, raw)
	if err != nil {
		return nil, err
	}
	for i, value := range values {
		if tab.columns[i].NotNull && value == nil {
			return nil, fmt.Errorf("%w: NULL violates declared NOT NULL", ErrCorrupt)
		}
		if s, ok := value.(string); ok && !utf8.ValidString(s) {
			return nil, fmt.Errorf("%w: invalid UTF-8 TEXT outside check profile", ErrUnsupported)
		}
		if _, err = columnValue(value, tab.columns[i].Type); err != nil {
			return nil, err
		}
	}
	return values, nil
}

// Named per-index checks count only that index's rows. A full check additionally
// rejects shadow rows that belong to no logical index or the wrong method.
func checkShadowOwner(name string, raw []any, byID map[int64]*extensionIndex) error {
	if !extensionReserved(name) || name == extensionCatalogName {
		return nil
	}
	if len(raw) < 2 {
		return extensionCorrupt("shadow row shape")
	}
	id, ok := raw[1].(int64)
	x := byID[id]
	if !ok || x == nil {
		return extensionCorrupt("shadow row has no logical owner")
	}
	switch name {
	case extensionFieldsName:
	case extensionPostingsName, extensionFrequencyName:
		if x.ann != nil {
			return extensionCorrupt("ANN index owns text shadow rows")
		}
	case annStateName:
		if x.ann == nil {
			return extensionCorrupt("text index owns ANN state")
		}
	case annNodesName, annLiveName:
		if x.method != ann.HNSW {
			return extensionCorrupt("non-HNSW index owns graph rows")
		}
	case annCentroidsName, annMembersName:
		if x.method != ann.IVFFlat {
			return extensionCorrupt("non-IVF index owns IVF rows")
		}
	default:
		return fmt.Errorf("%w: shadow table outside check profile", ErrUnsupported)
	}
	return nil
}
