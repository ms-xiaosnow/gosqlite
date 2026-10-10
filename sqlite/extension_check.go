package sqlite

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"

	"github.com/ms-xiaosnow/gosqlite/extension/registry"
	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// CheckExtensionIndex reconciles source rows, every expected posting, and exact
// document frequencies. It is a bounded read-only integrity check, not a repair.
// Dirty or schema-mismatched indexes must be rebuilt before they can pass.
func (d *DB) CheckExtensionIndex(ctx context.Context, name string) error {
	if err := checkContext(ctx); err != nil {
		return wrap("check extension index", err)
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return wrap("check extension index", ErrClosed)
	}
	var reader btree.PageReader
	if d.tx != nil {
		if err := d.tx.check(); err != nil {
			return wrap("check extension index", err)
		}
		if err := d.tx.ensureSnapshotLocked(ctx); err != nil {
			return wrap("check extension index", err)
		}
		reader = d.tx.reader
	} else {
		r, err := d.openReader(ctx)
		if err != nil {
			return wrap("check extension index", err)
		}
		err = d.checkExtensionIndex(ctx, r, name)
		return wrap("check extension index", errors.Join(err, r.Close()))
	}
	return wrap("check extension index", d.checkExtensionIndex(ctx, reader, name))
}
func (t *Tx) CheckExtensionIndex(ctx context.Context, name string) error {
	if err := checkContext(ctx); err != nil {
		return wrap("check extension index", err)
	}
	t.db.mu.Lock()
	defer t.db.mu.Unlock()
	if err := t.check(); err != nil {
		return wrap("check extension index", err)
	}
	if err := t.ensureSnapshotLocked(ctx); err != nil {
		return wrap("check extension index", err)
	}
	return wrap("check extension index", t.db.checkExtensionIndex(ctx, t.reader, name))
}
func (d *DB) checkExtensionIndex(ctx context.Context, reader btree.PageReader, name string) error {
	c, err := d.loadExtensions(ctx, reader)
	if err != nil {
		return err
	}
	var x *extensionIndex
	for _, candidate := range c.indexes {
		if fold(candidate.name) == fold(name) {
			x = candidate
			break
		}
	}
	if x == nil {
		return fmt.Errorf("no such extension index: %s", name)
	}
	if !x.usable {
		return fmt.Errorf("%w: %s", ErrReindexRequired, x.fallback)
	}
	if x.ann != nil {
		return d.checkANNIndex(ctx, reader, x)
	}
	source := c.entries[fold(x.sourceTable)]
	parsed, err := sqlast.Parse(source.SQL)
	if err != nil {
		return err
	}
	def, ok := parsed.(*sqlast.CreateTable)
	if !ok {
		return ErrUnsupported
	}
	tab, err := writableTable(def, source.RootPage)
	if err != nil {
		return err
	}
	type frequencyKey struct {
		field int64
		key   registry.Key
	}
	counts := map[frequencyKey]int64{}
	var bytes, expected, rows int64
	cur, err := btree.NewCursor(ctx, reader, tab.root)
	if err != nil {
		return err
	}
	defer cur.Close()
	helper := &Tx{db: d}
	for {
		id, raw, err := cur.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		values, err := tab.values(id, raw)
		if err != nil {
			return err
		}
		rows++
		for _, f := range x.fields {
			keys, size, err := helper.extensionKeys(ctx, x, values[f.ordinal])
			if err != nil {
				return err
			}
			if size > d.opts.MaxSortBytes-bytes {
				return ErrMemoryLimit
			}
			for k := range keys {
				fk := frequencyKey{f.id, k}
				if _, exists := counts[fk]; !exists {
					bytes += int64(len(k.Text)+len(k.Namespace)) + 160
					if bytes > d.opts.MaxSortBytes {
						return ErrMemoryLimit
					}
				}
				counts[fk]++
				if expected == math.MaxInt64 {
					return ErrFull
				}
				expected++
				prefix := append(extensionKeyPrefix(x, f, k), id)
				entry, found, err := extensionRangeOne(ctx, reader, c.roots[extensionPostingsLookup], 6, prefix)
				if err != nil {
					return err
				}
				if !found {
					return extensionCorrupt("missing expected source posting")
				}
				postingID, ok := entry[6].(int64)
				if !ok {
					return extensionCorrupt("posting id")
				}
				posting, found, err := btree.SeekTable(ctx, reader, c.roots[extensionPostingsName], postingID)
				if err != nil {
					return err
				}
				if !found || len(posting) != 7 {
					return extensionCorrupt("posting index has no table row")
				}
				for i, v := range prefix {
					if posting[i+1] != v {
						return extensionCorrupt("posting table/index mismatch")
					}
				}
			}
		}
	}
	if rows != x.documentCount {
		return extensionCorrupt("source row count differs")
	}
	actual, err := countExtensionRange(ctx, reader, c.roots[extensionPostingsLookup], 6, x.id)
	if err != nil {
		return err
	}
	if actual != expected {
		return extensionCorrupt("unexpected posting keys")
	}
	var tablePostings int64
	err = btree.Scan(ctx, reader, c.roots[extensionPostingsName], func(_ int64, row []any) error {
		if len(row) != 7 {
			return extensionCorrupt("posting row shape")
		}
		if row[1] == x.id {
			tablePostings++
		}
		return nil
	})
	if err != nil {
		return err
	}
	if tablePostings != expected {
		return extensionCorrupt("orphan posting table rows")
	}
	for fk, count := range counts {
		prefix := []any{x.id, fk.field, fk.key.Namespace, int64(fk.key.Length), fk.key.Text}
		key, found, err := extensionRangeOne(ctx, reader, c.roots[extensionFrequencyLookup], 5, prefix)
		if err != nil {
			return err
		}
		if !found {
			return extensionCorrupt("missing expected frequency")
		}
		id, ok := key[5].(int64)
		if !ok {
			return extensionCorrupt("frequency id")
		}
		row, found, err := btree.SeekTable(ctx, reader, c.roots[extensionFrequencyName], id)
		if err != nil {
			return err
		}
		if !found || len(row) != 7 || row[6] != count {
			return extensionCorrupt("frequency differs from source")
		}
		for i, v := range prefix {
			if row[i+1] != v {
				return extensionCorrupt("frequency table/index mismatch")
			}
		}
	}
	actual, err = countExtensionRange(ctx, reader, c.roots[extensionFrequencyLookup], 5, x.id)
	if err != nil {
		return err
	}
	if actual != int64(len(counts)) {
		return extensionCorrupt("unexpected frequency keys")
	}
	var tableFrequencies int64
	err = btree.Scan(ctx, reader, c.roots[extensionFrequencyName], func(_ int64, row []any) error {
		if len(row) != 7 {
			return extensionCorrupt("frequency row shape")
		}
		if row[1] == x.id {
			tableFrequencies++
		}
		return nil
	})
	if err != nil {
		return err
	}
	if tableFrequencies != int64(len(counts)) {
		return extensionCorrupt("orphan frequency table rows")
	}
	return nil
}
func countExtensionRange(ctx context.Context, reader btree.PageReader, root uint32, columns int, id int64) (int64, error) {
	prefix := []any{id}
	cur, err := btree.NewIndexRangeCursor(ctx, reader, root, btree.KeyInfo{Columns: columns}, prefix, prefix)
	if err != nil {
		return 0, err
	}
	defer cur.Close()
	var count int64
	for {
		_, err := cur.Next()
		if err == io.EOF {
			return count, nil
		}
		if err != nil {
			return 0, err
		}
		if count == math.MaxInt64 {
			return 0, ErrFull
		}
		count++
	}
}
