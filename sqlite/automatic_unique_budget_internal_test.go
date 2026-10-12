package sqlite

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	"github.com/ms-xiaosnow/gosqlite/internal/record"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// All oversized/edge cases here are inert strings or detached memory models.
// In particular, the reviewed long-name examples are never native file fixtures.
func automaticBudgetDefinition(t *testing.T, first byte, nameBytes, columns int) *sqlast.CreateTable {
	t.Helper()
	name := string(first) + strings.Repeat("x", nameBytes-1)
	var fields []string
	for i := 0; i < columns; i++ {
		fields = append(fields, fmt.Sprintf("c%02d UNIQUE", i))
	}
	return automaticTableDefinition(t, `CREATE TABLE "`+name+`"(`+strings.Join(fields, ",")+`)`)
}

func automaticBudgetRows(t *testing.T, def *sqlast.CreateTable) []btree.SchemaEntry {
	t.Helper()
	tab, err := writableTable(def, 0)
	if err != nil {
		t.Fatal(err)
	}
	return automaticTableSchemaRows(def, tab)
}

func TestAutomaticUniqueOwnerAmplificationPreflightMemory(t *testing.T) {
	ctx := context.Background()
	def := automaticBudgetDefinition(t, 'x', 600000, 20)
	rows := automaticBudgetRows(t, def)
	if len(def.SQL) != 600236 || len(rows) != 21 {
		t.Fatal("reviewed source-only model changed", len(def.SQL), len(rows))
	}
	if err := admitSchemaEntries(ctx, nil, rows); !errors.Is(err, record.ErrLimit) {
		t.Fatal("owner amplification escaped exact schema budget", err)
	}
	// No pager is supplied: touching storage before rejection would panic.
	// The complete table+implicit delta must fail before any root/row allocation.
	m := newCheckMemory()
	before := uniqueMemoryPages(m)
	catalog := &writeCatalog{tables: map[string]*writeTable{}, names: map[string]string{}}
	tx := &Tx{reader: m}
	if err := tx.createTable(ctx, catalog, def); !errors.Is(err, record.ErrLimit) {
		t.Fatal("compound CREATE did not reject in preflight", err)
	}
	if !reflect.DeepEqual(before, m.pages) || len(catalog.tables) != 0 || len(catalog.names) != 0 {
		t.Fatal("failed preflight changed source or local catalog")
	}
}

func TestAutomaticUniqueExistingOccupancyBudgetMemory(t *testing.T) {
	ctx := context.Background()
	var existing []btree.SchemaEntry
	for i, first := range []byte{'a', 'b', 'c'} {
		def := automaticBudgetDefinition(t, first, 300000, 10)
		rows := automaticBudgetRows(t, def)
		usage := btree.SchemaUsage{}
		for _, row := range rows {
			if err := usage.Add(row); err != nil {
				t.Fatal(err)
			}
		}
		if usage.Bytes != 6903188 || usage.Entries != 11 {
			t.Fatal("reviewed per-table source model changed", usage)
		}
		before := append([]btree.SchemaEntry(nil), existing...)
		err := admitSchemaEntries(ctx, existing, rows)
		if i < 2 {
			if err != nil {
				t.Fatal("first two tables fit the full catalog", i, err)
			}
			existing = append(existing, rows...)
		} else if !errors.Is(err, record.ErrLimit) || !reflect.DeepEqual(existing, before) {
			t.Fatal("third table exceeded aggregate budget or changed input", err)
		}
	}
}

func TestAutomaticUniqueCountDedupAndGuardBudgetMemory(t *testing.T) {
	ctx := context.Background()
	// Names need not be valid/unique in this pure accounting model. Every raw
	// row counts, including an extension-owned trigger omitted from c.names.
	existing := make([]btree.SchemaEntry, btree.MaxSchemaEntries)
	for i := range existing {
		existing[i] = btree.SchemaEntry{Type: "trigger", Name: "owned_guard", TableName: "t", SQL: "guard SQL"}
	}
	def := automaticTableDefinition(t, "CREATE TABLE t(a UNIQUE,b UNIQUE,c UNIQUE,d UNIQUE,e UNIQUE,f UNIQUE,g UNIQUE,h UNIQUE)")
	rows := automaticBudgetRows(t, def)
	if len(rows) != 9 {
		t.Fatal(rows)
	}
	if err := admitSchemaEntries(ctx, existing[:32758], rows); err != nil {
		t.Fatal("exact 32767 count rejected", err)
	}
	for _, old := range [][]btree.SchemaEntry{existing[:32759], existing[:32760]} {
		if err := admitSchemaEntries(ctx, old, rows); !errors.Is(err, record.ErrLimit) {
			t.Fatal("raw owned guards were dropped from occupancy", err)
		}
	}
	duplicate := automaticTableDefinition(t, "CREATE TABLE t(a UNIQUE UNIQUE,b UNIQUE,UNIQUE(a),UNIQUE(b))")
	rows = automaticBudgetRows(t, duplicate)
	if len(duplicate.UniqueConstraints) != 5 || len(rows) != 3 {
		t.Fatal("declaration budget and persisted index budget were conflated", duplicate, len(rows))
	}
	if err := admitSchemaEntries(ctx, existing[:32764], rows); err != nil {
		t.Fatal("deduplicated physical rows should fit", err)
	}
	if err := admitSchemaEntries(ctx, existing[:32765], rows); !errors.Is(err, record.ErrLimit) {
		t.Fatal("one deduplicated row over accepted", err)
	}
}

func TestAutomaticUniqueByteBoundaryAndFreshReaderMemory(t *testing.T) {
	ctx := context.Background()
	// Shared short strings keep the whole-catalog byte-edge model small.
	entry := btree.SchemaEntry{Type: "trigger", Name: "g", TableName: "t", SQL: strings.Repeat("x", 16384-256-7-1-1)}
	existing := make([]btree.SchemaEntry, 1023)
	for i := range existing {
		existing[i] = entry
	}
	if err := admitSchemaEntries(ctx, existing, []btree.SchemaEntry{entry}); err != nil {
		t.Fatal("exact byte budget rejected", err)
	}
	tooBig := entry
	tooBig.SQL += "x"
	if err := admitSchemaEntries(ctx, existing, []btree.SchemaEntry{tooBig}); !errors.Is(err, record.ErrLimit) {
		t.Fatal("one byte over admitted", err)
	}
	m := newCheckMemory()
	tx := &Tx{reader: m}
	if err := tx.admitSchemaRows(ctx, []btree.SchemaEntry{entry}); err != nil {
		t.Fatal(err)
	}
	root, err := btree.CreateTree(ctx, m, btree.TableTree)
	if err != nil {
		t.Fatal(err)
	}
	if err = btree.InsertTable(ctx, m, 1, 1, []any{"table", "t", "t", int64(root), "CREATE TABLE t(a)"}); err != nil {
		t.Fatal(err)
	}
	fresh, err := btree.ReadSchema(ctx, m)
	if err != nil || len(fresh) != 1 {
		t.Fatal(fresh, err)
	}
	// A delta that fills an empty catalog cannot fit after the real append.
	full := append(append([]btree.SchemaEntry(nil), existing...), entry)
	if err = tx.admitSchemaRows(ctx, full); !errors.Is(err, record.ErrLimit) {
		t.Fatal("admission retained stale pre-append usage", err)
	}
	if _, err = btree.DeleteTable(ctx, m, 1, 1); err != nil {
		t.Fatal(err)
	}
	if err = tx.admitSchemaRows(ctx, full); err != nil {
		t.Fatal("admission retained usage from removed memory record", err)
	}
}

func TestAutomaticUniqueNoopAndAdmissionErrorBeforeStorageMemory(t *testing.T) {
	ctx := context.Background()
	def := automaticTableDefinition(t, "CREATE TABLE IF NOT EXISTS t(a UNIQUE,b UNIQUE)")
	catalog := &writeCatalog{names: map[string]string{"t": "table"}}
	// Existing-table IF NOT EXISTS must not read/reserve budget or touch storage.
	if err := (&Tx{}).createTable(ctx, catalog, def); err != nil {
		t.Fatal("no-op attempted admission", err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	m := newCheckMemory()
	tab, err := writableTable(automaticTableDefinition(t, "CREATE TABLE t(a)"), 2)
	if err != nil {
		t.Fatal(err)
	}
	catalog = &writeCatalog{names: map[string]string{"t": "table"}, tables: map[string]*writeTable{"t": tab}}
	tx := &Tx{reader: m}
	index := &sqlast.CreateIndex{Base: sqlast.Base{SQL: "CREATE INDEX i ON t(a)"}, Name: "i", Table: "t", Columns: []string{"a"}}
	if err = tx.createIndex(canceled, catalog, index); !errors.Is(err, context.Canceled) {
		t.Fatal("explicit CREATE INDEX reached allocation before admission error", err)
	}
	if err = tx.addSchema(canceled, "trigger", "owned_guard", "t", 0, "guard SQL"); !errors.Is(err, context.Canceled) {
		t.Fatal("common append reached storage before admission error", err)
	}
}
