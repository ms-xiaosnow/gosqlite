package sqlite

import (
	"context"
	"io"

	"github.com/ms-xiaosnow/gosqlite/extension/grams13"
	"github.com/ms-xiaosnow/gosqlite/internal/btree"
)

func (x *extensionIndex) frequency(ctx context.Context, reader btree.PageReader, fieldID int64, namespace grams13.Namespace, token grams13.Token) (int64, error) {
	prefix := []any{x.id, fieldID, string(namespace), int64(token.Length), token.Text}
	key, found, err := extensionRangeOne(ctx, reader, x.catalog.roots[extensionFrequencyLookup], 5, prefix)
	if err != nil || !found {
		return 0, err
	}
	id, ok := key[5].(int64)
	if !ok {
		return 0, extensionCorrupt("frequency key rowid")
	}
	row, found, err := btree.SeekTable(ctx, reader, x.catalog.roots[extensionFrequencyName], id)
	if err != nil {
		return 0, err
	}
	if !found || len(row) != 7 {
		return 0, extensionCorrupt("missing frequency row")
	}
	for i, v := range prefix {
		if row[i+1] != v {
			return 0, extensionCorrupt("frequency key/table mismatch")
		}
	}
	count, ok := row[6].(int64)
	if !ok || count <= 0 || count > x.documentCount {
		return 0, extensionCorrupt("frequency count outside source bounds")
	}
	return count, nil
}
func (x *extensionIndex) postings(ctx context.Context, reader btree.PageReader, fieldID int64, namespace grams13.Namespace, token grams13.Token, limit int) ([]int64, int64, error) {
	prefix := []any{x.id, fieldID, string(namespace), int64(token.Length), token.Text}
	cur, err := btree.NewIndexRangeCursor(ctx, reader, x.catalog.roots[extensionPostingsLookup], btree.KeyInfo{Columns: 6}, prefix, prefix)
	if err != nil {
		return nil, 0, err
	}
	defer cur.Close()
	var rows []int64
	var visited int64
	for {
		key, err := cur.Next()
		if err == io.EOF {
			return rows, visited, nil
		}
		if err != nil {
			return nil, visited, err
		}
		visited++
		id, ok := key[5].(int64)
		if !ok {
			return nil, visited, extensionCorrupt("posting content rowid")
		}
		if len(rows) > 0 && id <= rows[len(rows)-1] {
			return nil, visited, extensionCorrupt("duplicate or unordered posting")
		}
		if len(rows) >= limit {
			return nil, visited, ErrMemoryLimit
		}
		rows = append(rows, id)
	}
}
