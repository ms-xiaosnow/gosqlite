package sqlite

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// Catalog anomalies and inconsistent logical keys in this file are detached
// descriptor-free memory models only. They never go through native files/VFS.
func automaticTableDefinition(t *testing.T, query string) *sqlast.CreateTable {
	t.Helper()
	stmt, err := sqlast.Parse(query)
	if err != nil {
		t.Fatal(err)
	}
	return stmt.(*sqlast.CreateTable)
}

func TestAutomaticUniqueDerivationMemory(t *testing.T) {
	def := automaticTableDefinition(t, `CREATE TABLE "T界"(id INTEGER PRIMARY KEY UNIQUE UNIQUE,"A" TEXT CONSTRAINT label UNIQUE COLLATE BINARY,b,UNIQUE(a),UNIQUE(a,id),UNIQUE(id,a),UNIQUE(a,a),CONSTRAINT other UNIQUE("A",id))`)
	tab, err := writableTable(def, 2)
	if err != nil {
		t.Fatal(err)
	}
	want := [][]int{{0}, {1}, {1, 0}, {0, 1}, {1, 1}}
	if tab.ipk != 0 || len(tab.indexes) != len(want) {
		t.Fatal("duplicate constraints consumed suffixes or IPK UNIQUE disappeared", tab)
	}
	for i, idx := range tab.indexes {
		name := "sqlite_autoindex_T界_" + string(rune('1'+i))
		if idx.name != name || idx.root != 0 || !idx.unique || !idx.constraintOwned || !reflect.DeepEqual(idx.columns, want[i]) {
			t.Fatal("wrong canonical descriptor", i, idx)
		}
	}
	plain, err := writableTable(automaticTableDefinition(t, "CREATE TABLE plain(id INTEGER PRIMARY KEY,a)"), 2)
	if err != nil || len(plain.indexes) != 0 {
		t.Fatal("INTEGER PRIMARY KEY alone created an autoindex", plain, err)
	}
}

func TestAutomaticUniqueSemanticAdmissionMemory(t *testing.T) {
	for _, typ := range []string{"VECTOR(2)", "HALFVEC(2)", "BIT(2)", "SPARSEVEC(2)"} {
		for _, query := range []string{"CREATE TABLE t(v " + typ + " UNIQUE)", "CREATE TABLE t(v " + typ + ",a,UNIQUE(a,v))"} {
			if _, err := writableTable(automaticTableDefinition(t, query), 0); !errors.Is(err, ErrUnsupported) {
				t.Fatal("typed-vector constraint key admitted", query, err)
			}
		}
		tab, err := writableTable(automaticTableDefinition(t, "CREATE TABLE t(id INTEGER PRIMARY KEY UNIQUE,a TEXT UNIQUE,v "+typ+")"), 0)
		if err != nil || len(tab.indexes) != 2 {
			t.Fatal("unrelated typed-vector column blocked scalar constraints", typ, err)
		}
	}
	for _, query := range []string{"CREATE TABLE t(a,UNIQUE(missing))", "CREATE TABLE t(a,UNIQUE(rowid))", "CREATE TABLE t(a,UNIQUE(_rowid_))", "CREATE TABLE t(a,UNIQUE(oid))", "CREATE TABLE t(a INT PRIMARY KEY UNIQUE)"} {
		if _, err := writableTable(automaticTableDefinition(t, query), 0); err == nil {
			t.Fatal("unknown column/hidden alias/unsupported PK admitted", query)
		}
	}
	if tab, err := writableTable(automaticTableDefinition(t, "CREATE TABLE t(rowid TEXT UNIQUE)"), 0); err != nil || !reflect.DeepEqual(tab.indexes[0].columns, []int{0}) {
		t.Fatal("declared rowid column confused with hidden alias", tab, err)
	}
	for _, constraints := range [][]sqlast.UniqueConstraint{
		{{}}, {{Columns: make([]string, sqlast.MaxColumns+1)}}, make([]sqlast.UniqueConstraint, sqlast.MaxUniqueConstraints+1),
	} {
		def := &sqlast.CreateTable{Name: "t", Columns: []sqlast.ColumnDef{{Name: "a"}}, UniqueConstraints: constraints}
		if _, err := writableTable(def, 0); !errors.Is(err, ErrUnsupported) {
			t.Fatal("manual AST bypassed UNIQUE budgets", err)
		}
	}
	// A very long table name cannot multiply into an unbounded descriptor set.
	def := &sqlast.CreateTable{Name: strings.Repeat("x", 1<<20)}
	for i := 0; i < 20; i++ {
		name := string(rune('a' + i))
		def.Columns = append(def.Columns, sqlast.ColumnDef{Name: name})
		def.UniqueConstraints = append(def.UniqueConstraints, sqlast.UniqueConstraint{Columns: []string{name}})
	}
	if _, err := writableTable(def, 0); !errors.Is(err, ErrMemoryLimit) {
		t.Fatal("derived name retention was not bounded", err)
	}
}

func automaticCatalogEntries() []btree.SchemaEntry {
	return []btree.SchemaEntry{
		{Type: "table", Name: "T", TableName: "T", RootPage: 2, SQL: "CREATE TABLE T(id INTEGER PRIMARY KEY UNIQUE,a TEXT UNIQUE,b,UNIQUE(a),UNIQUE(a,b))"},
		{Type: "index", Name: "sqlite_autoindex_T_1", TableName: "T", RootPage: 3, SQLNull: true},
		{Type: "index", Name: "sqlite_autoindex_T_2", TableName: "T", RootPage: 4, SQLNull: true},
		{Type: "index", Name: "sqlite_autoindex_T_3", TableName: "T", RootPage: 5, SQLNull: true},
		{Type: "index", Name: "explicit_u", TableName: "T", RootPage: 6, SQL: "CREATE UNIQUE INDEX explicit_u ON T(a)"},
		{Type: "index", Name: "ordinary", TableName: "T", RootPage: 7, SQL: "CREATE INDEX ordinary ON T(b)"},
	}
}

func TestAutomaticUniqueCatalogBindingMemory(t *testing.T) {
	entries := automaticCatalogEntries()
	for _, order := range [][]int{{0, 1, 2, 3, 4, 5}, {5, 3, 1, 4, 2, 0}, {4, 2, 0, 1, 5, 3}} {
		shuffled := make([]btree.SchemaEntry, len(entries))
		for i, source := range order {
			shuffled[i] = entries[source]
		}
		before := append([]btree.SchemaEntry(nil), shuffled...)
		catalog, err := buildWriteCatalog(context.Background(), shuffled, &extensionCatalog{})
		if err != nil || len(catalog.tables) != 1 || len(catalog.names) != 6 {
			t.Fatal("row-order-dependent reconstruction", catalog, err)
		}
		tab := catalog.tables["t"]
		if len(tab.indexes) != 5 || len(catalog.extensions.indexes) != 0 {
			t.Fatal("lost implicit or explicit index", tab)
		}
		for i := 0; i < 3; i++ {
			idx := tab.indexes[i]
			if !idx.constraintOwned || !idx.unique || idx.root != uint32(i+3) || idx.name != entries[i+1].Name {
				t.Fatal("wrong ownership/root binding", idx)
			}
		}
		for _, idx := range tab.indexes[3:] {
			if idx.constraintOwned || idx.unique != (idx.name == "explicit_u") {
				t.Fatal("explicit index was coalesced or constraint-owned", idx)
			}
		}
		if !reflect.DeepEqual(before, shuffled) {
			t.Fatal("catalog validator mutated metadata input")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := buildWriteCatalog(ctx, entries, &extensionCatalog{}); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled catalog admitted", err)
	}
}

func TestAutomaticUniqueCatalogRejectsMemory(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func([]btree.SchemaEntry) []btree.SchemaEntry
		kind   error
	}{
		{"missing", func(e []btree.SchemaEntry) []btree.SchemaEntry { return append(e[:2], e[3:]...) }, ErrCorrupt},
		{"extra suffix", func(e []btree.SchemaEntry) []btree.SchemaEntry {
			return append(e, btree.SchemaEntry{Type: "index", Name: "sqlite_autoindex_T_4", TableName: "T", RootPage: 8, SQLNull: true})
		}, ErrUnsupported},
		{"duplicate name", func(e []btree.SchemaEntry) []btree.SchemaEntry {
			copy := e[1]
			copy.RootPage = 8
			return append(e, copy)
		}, ErrCorrupt},
		{"wrong owner", func(e []btree.SchemaEntry) []btree.SchemaEntry { e[1].TableName = "other"; return e }, ErrCorrupt},
		{"owner case", func(e []btree.SchemaEntry) []btree.SchemaEntry { e[1].TableName = "t"; return e }, ErrCorrupt},
		{"name case", func(e []btree.SchemaEntry) []btree.SchemaEntry { e[1].Name = "sqlite_autoindex_t_1"; return e }, ErrCorrupt},
		{"wrong type", func(e []btree.SchemaEntry) []btree.SchemaEntry { e[1].Type = "view"; return e }, ErrUnsupported},
		{"zero root", func(e []btree.SchemaEntry) []btree.SchemaEntry { e[1].RootPage = 0; return e }, ErrCorrupt},
		{"schema root", func(e []btree.SchemaEntry) []btree.SchemaEntry { e[1].RootPage = 1; return e }, ErrCorrupt},
		{"table root reused", func(e []btree.SchemaEntry) []btree.SchemaEntry { e[1].RootPage = 2; return e }, ErrCorrupt},
		{"index root reused", func(e []btree.SchemaEntry) []btree.SchemaEntry { e[1].RootPage = 4; return e }, ErrCorrupt},
		{"noncanonical empty TEXT", func(e []btree.SchemaEntry) []btree.SchemaEntry { e[1].SQLNull = false; return e }, ErrUnsupported},
		{"fabricated explicit SQL", func(e []btree.SchemaEntry) []btree.SchemaEntry {
			e[1].SQLNull = false
			e[1].SQL = "CREATE UNIQUE INDEX sqlite_autoindex_T_1 ON T(id)"
			return e
		}, ErrCorrupt},
		{"inconsistent typed NULL", func(e []btree.SchemaEntry) []btree.SchemaEntry { e[1].SQL = "not empty"; return e }, ErrCorrupt},
		{"orphan nonreserved NULL", func(e []btree.SchemaEntry) []btree.SchemaEntry { e[4].SQL = ""; e[4].SQLNull = true; return e }, ErrCorrupt},
		{"unknown internal name", func(e []btree.SchemaEntry) []btree.SchemaEntry {
			e[4].Name = "sqlite_unrecognized"
			e[4].SQL = "CREATE INDEX sqlite_unrecognized ON T(a)"
			return e
		}, ErrUnsupported},
		{"orphan autoindex", func(e []btree.SchemaEntry) []btree.SchemaEntry {
			e[0].SQL = "CREATE TABLE T(id INTEGER PRIMARY KEY,a TEXT,b)"
			return e
		}, ErrUnsupported},
		{"NULL table SQL", func(e []btree.SchemaEntry) []btree.SchemaEntry { e[0].SQL = ""; e[0].SQLNull = true; return e }, ErrCorrupt},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := buildWriteCatalog(context.Background(), tc.change(automaticCatalogEntries()), &extensionCatalog{})
			if !errors.Is(err, tc.kind) {
				t.Fatal("unexpected catalog admission/error", tc.kind, err)
			}
			if tc.name == "noncanonical empty TEXT" && errors.Is(err, ErrCorrupt) {
				t.Fatal("healthy SQLite empty TEXT was mislabeled corrupt", err)
			}
		})
	}
}

func automaticCheckFixture(t *testing.T, duplicate bool) (*DB, *checkMemory) {
	t.Helper()
	d, err := copyValidator()
	if err != nil {
		t.Fatal(err)
	}
	m := newCheckMemory()
	ctx := context.Background()
	tableRoot, err := btree.CreateTree(ctx, m, btree.TableTree)
	if err != nil {
		t.Fatal(err)
	}
	indexRoot, err := btree.CreateTree(ctx, m, btree.IndexTree)
	if err != nil {
		t.Fatal(err)
	}
	for i, row := range [][]any{
		{"table", "t", "t", int64(tableRoot), "CREATE TABLE t(id INTEGER PRIMARY KEY,a,b,UNIQUE(a,b))"},
		{"index", "sqlite_autoindex_t_1", "t", int64(indexRoot), nil},
	} {
		if err = btree.InsertTable(ctx, m, 1, int64(i+1), row); err != nil {
			t.Fatal(err)
		}
	}
	rows := [][]any{{nil, "key", int64(3)}, {nil, nil, int64(3)}, {nil, nil, int64(3)}}
	if duplicate {
		rows = append(rows, []any{nil, "key", float64(3)})
	}
	for i, row := range rows {
		id := int64(i - 1)
		if err = btree.InsertTable(ctx, m, tableRoot, id, row); err != nil {
			t.Fatal(err)
		}
		if err = btree.InsertIndex(ctx, m, indexRoot, btree.KeyInfo{Columns: 2}, []any{row[1], row[2], id}); err != nil {
			t.Fatal(err)
		}
	}
	return d, m
}

func TestAutomaticUniqueCheckAndPlannerMemory(t *testing.T) {
	for _, duplicate := range []bool{false, true} {
		d, m := automaticCheckFixture(t, duplicate)
		before := uniqueMemoryPages(m)
		got, err := checkMemoryRun(d, m, CheckOptions{})
		if duplicate {
			if !errors.Is(err, ErrCorrupt) || got != (CheckResult{}) {
				t.Fatal("autoindex duplicate prefix passed Check", got, err)
			}
		} else if err != nil || got.Tables != 1 || got.Indexes != 1 || got.TableRows != 3 || got.IndexEntries != 3 {
			t.Fatal("Check omitted implicit root or NULL records", got, err)
		}
		catalog, err := (&Tx{db: d, reader: m}).validateCatalog(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		tab := catalog.tables["t"]
		if len(tab.indexes) != 1 || !tab.indexes[0].constraintOwned || tab.indexes[0].root != 3 {
			t.Fatal("writer ownership descriptor missing", tab)
		}
		idx, _, err := ordinaryReadIndex(context.Background(), m, tab.table, map[int]any{1: "key"})
		if err != nil || idx != nil {
			t.Fatal("autoindex was newly admitted to equality planner", idx, err)
		}
		bound := &secondaryInterval{}
		if err = bound.add(">=", "key"); err != nil {
			t.Fatal(err)
		}
		idx, interval, err := secondaryReadIndex(context.Background(), m, tab.table, map[int]*secondaryInterval{1: bound})
		if err != nil || idx != nil || interval != nil || !reflect.DeepEqual(before, m.pages) {
			t.Fatal("range planner admitted autoindex or read changed memory", idx, interval, err)
		}
	}
}
