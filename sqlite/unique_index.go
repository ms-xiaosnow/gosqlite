package sqlite

import (
	"context"
	"fmt"
	"io"

	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// probeUniqueIndex uses the current write snapshot and the post-affinity key.
// Bounds contain every indexed field but no rowid, so INTEGER/REAL equality
// uses the B-tree comparator without rounding integer values through float64.
// The explicit exclusion flag is required: zero and negative rowids are valid.
func probeUniqueIndex(ctx context.Context, reader btree.PageReader, idx writeIndex, key []any, oldRowID int64, excludeOld bool, limits btree.RangeLimits) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !idx.unique {
		return nil
	}
	if len(idx.columns) == 0 || len(key) != len(idx.columns)+1 {
		return fmt.Errorf("%w: invalid UNIQUE index key arity", ErrCorrupt)
	}
	prefix := key[:len(idx.columns)]
	for _, value := range prefix {
		if value == nil {
			return nil // Any NULL field exempts the complete key from uniqueness.
		}
	}
	bound := &btree.IndexBound{Key: prefix, Inclusive: true}
	cursor, err := btree.NewBoundedIndexCursor(ctx, reader, idx.root, btree.KeyInfo{Columns: len(idx.columns)}, bound, bound, false, limits)
	if err != nil {
		return err
	}
	defer cursor.Close()
	for {
		found, err := cursor.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err // A quota/cancellation/corruption error is never absence.
		}
		rowid := found[len(idx.columns)].(int64) // Cursor validates full key shape.
		if !excludeOld || rowid != oldRowID {
			return fmt.Errorf("%w: UNIQUE index %q", ErrConstraint, idx.name)
		}
	}
}

// checkUniqueIndexes runs before source/index mutation and before updating the
// connection's last_insert_rowid. UPDATE's rowid check is performed by update;
// INSERT needs the same existing rowid-first ordering before these new checks.
func (t *Tx) checkUniqueIndexes(ctx context.Context, tab *writeTable, rowid int64, values []any, oldRowID int64, excludeOld bool) error {
	checkedRowID := excludeOld
	for _, idx := range tab.indexes {
		if !idx.unique {
			continue
		}
		if !checkedRowID {
			if _, exists, err := btree.SeekTable(ctx, t.reader, tab.root, rowid); err != nil {
				return err
			} else if exists {
				return fmt.Errorf("%w: duplicate rowid", ErrConstraint)
			}
			checkedRowID = true
		}
		if err := probeUniqueIndex(ctx, t.reader, idx, idx.key(rowid, values), oldRowID, excludeOld, btree.RangeLimits{}); err != nil {
			return err
		}
	}
	return nil
}

// equalUniquePrefixes compares adjacent full indexed prefixes for Check. NULL
// exempts a key here just as it does on writes; rowids never enter this test.
func equalUniquePrefixes(a, b []any) (bool, error) {
	if len(a) == 0 || len(a) != len(b) {
		return false, nil
	}
	for i := range a {
		if a[i] == nil || b[i] == nil {
			return false, nil
		}
		cmp, err := sqlast.Compare(a[i], b[i])
		if err != nil || cmp != 0 {
			return false, err
		}
	}
	return true, nil
}
