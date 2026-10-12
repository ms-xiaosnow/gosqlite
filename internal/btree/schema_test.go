package btree

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/pager"
)

// These schema-record fixtures exist only in memory. Even rejected values are
// encoded through the regular record/tree API; no file or VFS is involved.
func TestSchemaSQLNullDiscriminatorMemory(t *testing.T) {
	ctx := context.Background()
	m := newMemoryPages(512)
	for i, value := range []any{nil, "", "CREATE INDEX ix ON t(a)"} {
		root, err := CreateTree(ctx, m, IndexTree)
		if err != nil {
			t.Fatal(err)
		}
		if err = InsertTable(ctx, m, 1, int64(i+1), []any{"index", string(rune('a' + i)), "t", int64(root), value}); err != nil {
			t.Fatal(err)
		}
	}
	writes := m.writes
	entries, err := ReadSchema(ctx, m)
	if err != nil || len(entries) != 3 {
		t.Fatal(entries, err)
	}
	if !entries[0].SQLNull || entries[0].SQL != "" || entries[1].SQLNull || entries[1].SQL != "" || entries[2].SQLNull || entries[2].SQL != "CREATE INDEX ix ON t(a)" {
		t.Fatal("NULL and empty TEXT were conflated", entries)
	}
	var raw []any
	if err = Scan(ctx, m, 1, func(_ int64, values []any) error {
		raw = append(raw, values[4])
		return nil
	}); err != nil || !reflect.DeepEqual(raw, []any{nil, "", "CREATE INDEX ix ON t(a)"}) || m.writes != writes {
		t.Fatal("schema read changed raw SQL storage classes", raw, err)
	}
}

func TestSchemaSQLInvalidTypeMemory(t *testing.T) {
	for _, value := range []any{int64(0), []byte{}, float64(1.25)} {
		ctx := context.Background()
		m := newMemoryPages(512)
		root, err := CreateTree(ctx, m, IndexTree)
		if err != nil {
			t.Fatal(err)
		}
		if err = InsertTable(ctx, m, 1, 1, []any{"index", "ix", "t", int64(root), value}); err != nil {
			t.Fatal(err)
		}
		if _, err = ReadSchema(ctx, m); !errors.Is(err, pager.ErrCorrupt) {
			t.Fatal("non-text/non-NULL schema SQL accepted", value, err)
		}
	}
}
