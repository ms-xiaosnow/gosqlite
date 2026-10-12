package btree

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/record"
)

// These are value/record models only; no database, file or VFS is opened.
func TestSchemaUsageExactByteBoundaryMemory(t *testing.T) {
	entry := SchemaEntry{Type: "index", Name: "i", TableName: "t", SQLNull: true}
	const charge = 256 + 5 + 1 + 1
	for _, excess := range []int{0, 1} {
		usage := SchemaUsage{Entries: 3, Bytes: MaxSchemaBytes - charge + excess}
		before := usage
		err := usage.Add(entry)
		if excess == 0 {
			if err != nil || usage.Bytes != MaxSchemaBytes || usage.Entries != 4 {
				t.Fatal("exact byte limit rejected", usage, err)
			}
			before = usage
			if err = usage.Add(entry); !errors.Is(err, record.ErrLimit) || usage != before {
				t.Fatal("full budget advanced on error", usage, err)
			}
		} else if !errors.Is(err, record.ErrLimit) || usage != before {
			t.Fatal("one byte over accepted or mutated usage", usage, err)
		}
	}
	if MaxSchemaBytes != 16777216 || MaxSchemaEntries != 32767 {
		t.Fatal("reader limits were changed", MaxSchemaBytes, MaxSchemaEntries)
	}
}

func TestSchemaUsageCountAndOverflowMemory(t *testing.T) {
	entry := SchemaEntry{Type: "index", Name: "i", TableName: "t", SQLNull: true}
	usage := SchemaUsage{Entries: MaxSchemaEntries - 1}
	if err := usage.Add(entry); err != nil || usage.Entries != MaxSchemaEntries {
		t.Fatal("exact count limit rejected", usage, err)
	}
	before := usage
	if err := usage.Add(entry); !errors.Is(err, record.ErrLimit) || usage != before {
		t.Fatal("one entry over accepted or mutated usage", usage, err)
	}
	maxInt := int(^uint(0) >> 1)
	for _, usage := range []SchemaUsage{
		{Entries: -1}, {Entries: maxInt}, {Entries: MaxSchemaEntries + 1},
		{Bytes: -1}, {Bytes: maxInt}, {Bytes: MaxSchemaBytes + 1},
	} {
		before := usage
		if err := usage.Add(entry); !errors.Is(err, record.ErrLimit) || usage != before {
			t.Fatal("invalid counter overflowed or was advanced", before, usage, err)
		}
	}
}

func TestSchemaUsageExactFieldChargeMemory(t *testing.T) {
	for _, sql := range []string{"", "CREATE INDEX i ON t(a)"} {
		entry := SchemaEntry{Type: "index", Name: "sqlite_autoindex_t_1", TableName: strings.Repeat("owner", 100), SQL: sql}
		usage := SchemaUsage{}
		if err := usage.Add(entry); err != nil {
			t.Fatal(err)
		}
		want := 256 + len(entry.Type) + len(entry.Name) + len(entry.TableName) + len(entry.SQL)
		if usage.Entries != 1 || usage.Bytes != want {
			t.Fatal("owner or SQL bytes omitted", usage, want)
		}
		if sql == "" {
			entry.SQLNull = true
			other := SchemaUsage{}
			if err := other.Add(entry); err != nil || other != usage {
				t.Fatal("NULL and empty TEXT need the same zero SQL-byte charge", other, err)
			}
		}
	}
}

func TestSchemaUsageReadSchemaAgreementMemory(t *testing.T) {
	ctx := context.Background()
	m := newMemoryPages(512)
	want := SchemaUsage{}
	for i, sql := range []any{nil, "", "CREATE INDEX i ON t(a)"} {
		root, err := CreateTree(ctx, m, IndexTree)
		if err != nil {
			t.Fatal(err)
		}
		name := string(rune('a' + i))
		if err = InsertTable(ctx, m, 1, int64(i+1), []any{"index", name, "owner", int64(root), sql}); err != nil {
			t.Fatal(err)
		}
		want.Entries++
		want.Bytes += 256 + 5 + len(name) + len("owner")
		if text, ok := sql.(string); ok {
			want.Bytes += len(text)
		}
	}
	entries, err := ReadSchema(ctx, m)
	if err != nil {
		t.Fatal(err)
	}
	got := SchemaUsage{}
	for _, entry := range entries {
		if err = got.Add(entry); err != nil {
			t.Fatal(err)
		}
	}
	if got != want {
		t.Fatal("shared charge differs from the original reader contract", got, want)
	}
}
