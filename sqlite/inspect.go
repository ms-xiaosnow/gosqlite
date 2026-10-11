package sqlite

import (
	"context"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/ms-xiaosnow/gosqlite/internal/btree"
)

// InspectOptions selects optional stored metadata. Its zero value reads only
// the header and sqlite_schema; it does not read extension catalog records.
type InspectOptions struct{ IncludeExtensions bool }

// ExtensionIndexMetadata describes persisted identity and configuration, not
// index integrity or usability. Config is an opaque string, not necessarily
// JSON. Inspection never resolves methods or invokes application callbacks.
type ExtensionIndexMetadata struct {
	ID             int64  `json:"id"`
	Name           string `json:"name"`
	SourceTable    string `json:"source_table"`
	Method         string `json:"method"`
	MethodVersion  int    `json:"method_version"`
	StorageVersion int    `json:"storage_version"`
	Config         string `json:"config"`
	Generation     int64  `json:"generation"`
}

// InspectResult contains metadata from one snapshot, including uncommitted DDL
// in the connection's active transaction. ExtensionIndexes is nil when omitted
// and a nonnil, possibly empty slice when IncludeExtensions is true.
type InspectResult struct {
	Header           FileInfo
	Schema           []SchemaEntry
	ExtensionIndexes []ExtensionIndexMetadata
}

// Inspect reads stored metadata, not an integrity or index-usability verdict.
// It borrows an active transaction's reader without ending that transaction;
// otherwise it opens and closes exactly one reader. No partial result is
// returned on an error, including cancellation or owned-reader cleanup failure.
//
// Optional extension inspection reads only sqlite_schema and the extension
// catalog, never source rows, fields, postings, frequency or ANN data trees.
// At most 1024 catalog records are retained. Options.MaxSortBytes bounds its
// conservatively charged metadata maps, strings and catalog record decoding.
// The existing independent schema/payload and B-tree traversal limits still
// apply: this is not a bound on all Go heap, page-cache or schema allocations.
func (d *DB) Inspect(ctx context.Context, options InspectOptions) (result InspectResult, retErr error) {
	if err := checkContext(ctx); err != nil {
		return InspectResult{}, wrap("inspect", err)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return InspectResult{}, wrap("inspect", ErrClosed)
	}
	var reader btree.PageReader
	var closeReader func() error
	if d.tx != nil {
		if err := d.tx.check(); err != nil {
			return InspectResult{}, wrap("inspect", err)
		}
		if err := d.tx.ensureSnapshotLocked(ctx); err != nil {
			return InspectResult{}, wrap("inspect", err)
		}
		reader = d.tx.reader
	} else {
		r, err := d.openReader(ctx)
		if err != nil {
			return InspectResult{}, wrap("inspect", err)
		}
		reader, closeReader = r, r.Close
	}
	defer func() {
		if closeReader != nil {
			retErr = wrapCleanup("inspect", retErr, closeReader())
		}
		if err := checkContext(ctx); err != nil {
			retErr = wrapCleanup("inspect", retErr, err)
		}
		if d.tx != nil {
			if err := d.tx.check(); err != nil {
				retErr = wrapCleanup("inspect", retErr, err)
			}
		}
		if retErr != nil {
			result = InspectResult{}
		} else {
			d.info = result.Header
		}
	}()
	result, err := d.inspectSnapshot(ctx, reader, options)
	return result, wrap("inspect", err)
}

func (d *DB) inspectSnapshot(ctx context.Context, reader btree.PageReader, options InspectOptions) (InspectResult, error) {
	entries, err := btree.ReadSchema(ctx, reader)
	if err != nil {
		return InspectResult{}, err
	}
	result := InspectResult{Header: fileInfo(reader.Header()), Schema: make([]SchemaEntry, len(entries))}
	for i, e := range entries {
		result.Schema[i] = SchemaEntry{e.Type, e.Name, e.TableName, e.SQL, e.RootPage}
	}
	if options.IncludeExtensions {
		result.ExtensionIndexes, err = d.inspectExtensionCatalog(ctx, reader, entries)
		if err != nil {
			return InspectResult{}, err
		}
	}
	if err := ctx.Err(); err != nil {
		return InspectResult{}, err
	}
	return result, nil
}

// Charges include slice/map growth and temporary folded names. Catalog record
// reservations remain charged for the entire inspection, conservatively covering
// retained result strings/maps as well as each temporary payload and Decode's
// variable-column allocations. Reserve before overflow assembly and decoding.
type inspectMetadataBudget struct{ used, max int64 }

func (b *inspectMetadataBudget) reserve(n int64) error {
	if n < 0 || n > b.max-b.used {
		return ErrMemoryLimit
	}
	b.used += n
	return nil
}

func (d *DB) inspectExtensionCatalog(ctx context.Context, reader btree.PageReader, entries []btree.SchemaEntry) ([]ExtensionIndexMetadata, error) {
	budget := inspectMetadataBudget{max: d.opts.MaxSortBytes}
	if err := budget.reserve(2048); err != nil {
		return nil, err
	}
	byName := make(map[string]btree.SchemaEntry)
	reserved := false
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := budget.reserve(256 + 3*int64(len(e.Name))); err != nil {
			return nil, err
		}
		key := fold(e.Name)
		if _, exists := byName[key]; exists {
			return nil, extensionCorrupt("duplicate schema name")
		}
		byName[key] = e
		reserved = reserved || extensionReserved(e.Name)
	}
	result := make([]ExtensionIndexMetadata, 0)
	if !reserved {
		return result, nil
	}
	seenRoots := make(map[uint32]bool)
	for _, e := range entries {
		if e.RootPage > 0 {
			if seenRoots[e.RootPage] {
				return nil, extensionCorrupt("shared schema root")
			}
			if err := budget.reserve(64); err != nil {
				return nil, err
			}
			seenRoots[e.RootPage] = true
		}
	}
	// The ANN envelope is optional as a whole. Checking its schema identities
	// does not read its roots or interpret method/configuration semantics.
	specs := extensionSchemas()
	for _, s := range annSchemas() {
		if _, exists := byName[s.name]; exists {
			specs = append(specs, annSchemas()...)
			break
		}
	}
	recognized := make(map[string]bool)
	for _, s := range specs {
		e, ok := byName[s.name]
		if !ok || e.Name != s.name || e.Type != s.typ || e.TableName != s.table || e.SQL != s.sql || e.RootPage < 2 {
			return nil, fmt.Errorf("%w: unrecognized or incomplete extension object %q", ErrUnsupported, s.name)
		}
		if err := budget.reserve(128); err != nil {
			return nil, err
		}
		recognized[s.name] = true
	}
	names := make(map[string]bool)
	cursor, err := btree.NewBudgetCursor(ctx, reader, byName[extensionCatalogName].RootPage, func(n int) error {
		if len(result) >= 1024 {
			return ErrMemoryLimit
		}
		return budget.reserve(128*int64(n) + 2048)
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close()
	for {
		id, row, err := cursor.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if id <= 0 || len(row) != 13 || row[0] != nil {
			return nil, extensionCorrupt("catalog row shape")
		}
		var text [6]string
		for i, column := range [...]int{1, 2, 3, 4, 7, 8} {
			value, ok := row[column].(string)
			if !ok {
				return nil, extensionCorrupt("catalog text field")
			}
			text[i] = value
		}
		mv, mok := row[5].(int64)
		sv, sok := row[6].(int64)
		generation, gok := row[9].(int64)
		dirty, dok := row[10].(int64)
		count, cok := row[11].(int64)
		cookie, cookieOK := row[12].(int64)
		if !mok || !sok || !gok || !dok || !cok || !cookieOK || cookie < 0 || cookie > 4294967295 || generation < 1 || count < 0 || (dirty != 0 && dirty != 1) {
			return nil, extensionCorrupt("catalog integer field")
		}
		if mv <= 0 || int64(int(mv)) != mv {
			return nil, fmt.Errorf("%w: extension method version %d", ErrUnsupported, mv)
		}
		if sv != extensionStorageVersion {
			return nil, fmt.Errorf("%w: extension storage version %d", ErrUnsupported, sv)
		}
		name := fold(text[0])
		if name == "" || extensionReserved(name) || strings.HasPrefix(name, "sqlite_") || names[name] {
			return nil, extensionCorrupt("logical index name")
		}
		if _, exists := byName[name]; exists {
			return nil, extensionCorrupt("logical index collides with schema")
		}
		// JSON encoders replace invalid UTF-8 silently. Reject it in every newly
		// exposed text field, preserving opaque configuration bytes on success.
		for _, value := range [...]string{text[0], text[1], text[3], text[4]} {
			if !utf8.ValidString(value) {
				return nil, fmt.Errorf("%w: extension metadata contains invalid UTF-8", ErrUnsupported)
			}
		}
		names[name] = true
		result = append(result, ExtensionIndexMetadata{id, text[0], text[1], text[3], int(mv), int(sv), text[4], generation})
		// The operational loader tolerates absent/changed guards as unusable.
		// Recognize those names without interpreting usability during inspection.
		for _, operation := range [...]string{"INSERT", "UPDATE", "DELETE"} {
			guard := extensionGuardName(id, operation)
			if _, exists := byName[guard]; exists {
				recognized[guard] = true
			}
		}
	}
	for name, e := range byName {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if extensionReserved(e.Name) && !recognized[name] {
			return nil, fmt.Errorf("%w: unknown owned-prefix object %q", ErrUnsupported, e.Name)
		}
	}
	return result, nil
}
