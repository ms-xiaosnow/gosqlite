package sqlite

import (
	"context"
	"errors"
	"io"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	"github.com/ms-xiaosnow/gosqlite/internal/record"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// All fixtures in this file are descriptor-free, detached in-memory trees.
// Only ordinary B-tree APIs create their records and pages. No Open, native
// file, VFS, malformed page bytes, subprocess, or physical failure is involved.
func uniqueProbeFixture(t *testing.T, columns int, keys ...[]any) (*checkMemory, writeIndex) {
	t.Helper()
	m := newCheckMemory()
	root, err := btree.CreateTree(context.Background(), m, btree.IndexTree)
	if err != nil {
		t.Fatal(err)
	}
	idx := writeIndex{name: "uq", root: root, columns: make([]int, columns), unique: true}
	for i := range idx.columns {
		idx.columns[i] = i
	}
	for _, key := range keys {
		if err := btree.InsertIndex(context.Background(), m, root, btree.KeyInfo{Columns: columns}, key); err != nil {
			t.Fatal(err)
		}
	}
	m.reads = 0
	return m, idx
}

func uniqueProbeLimits() btree.RangeLimits {
	return btree.RangeLimits{PageReads: 4096, Work: 1 << 20, Records: 4096, PayloadBytes: 1 << 20}
}

func uniqueMemoryPages(m *checkMemory) map[uint32][]byte {
	copy := make(map[uint32][]byte, len(m.pages))
	for n, page := range m.pages {
		copy[n] = append([]byte(nil), page...)
	}
	return copy
}

func TestUniqueIndexProbeLogicalKeys(t *testing.T) {
	for _, tc := range []struct {
		name               string
		existing, proposed []any
		conflict           bool
	}{
		{"same integer", []any{int64(1)}, []any{int64(1)}, true},
		{"different integer", []any{int64(1)}, []any{int64(2)}, false},
		{"integer real equality", []any{int64(1)}, []any{float64(1)}, true},
		{"real integer equality", []any{float64(1)}, []any{int64(1)}, true},
		{"fractional real", []any{float64(1.5)}, []any{int64(1)}, false},
		{"signed zero", []any{math.Copysign(0, -1)}, []any{int64(0)}, true},
		{"integer text distinct", []any{int64(1)}, []any{"1"}, false},
		{"text blob distinct", []any{"same"}, []any{[]byte("same")}, false},
		{"binary text case", []any{"A"}, []any{"a"}, false},
		{"binary text spaces", []any{"a"}, []any{"a "}, false},
		{"nul unicode text", []any{"a\x00数据库"}, []any{"a\x00数据库"}, true},
		{"nul unicode distinction", []any{"a\x00数据库"}, []any{"a\x00数据库x"}, false},
		{"binary bytes", []any{[]byte{0, 255, 1}}, []any{[]byte{0, 255, 1}}, true},
		{"binary bytes distinction", []any{[]byte{0, 255, 1}}, []any{[]byte{0, 255, 2}}, false},
		{"empty text blob distinction", []any{""}, []any{[]byte{}}, false},
		{"nil blob is not null", []any{[]byte(nil)}, []any{[]byte{}}, true},
		{"two to fifty three equal", []any{int64(9007199254740992)}, []any{float64(9007199254740992)}, true},
		{"two to fifty three adjacent integers", []any{int64(9007199254740992)}, []any{int64(9007199254740993)}, false},
		{"two to fifty three integer real distinction", []any{int64(9007199254740993)}, []any{float64(9007199254740992)}, false},
		{"negative exact boundary", []any{int64(-9007199254740993)}, []any{float64(-9007199254740992)}, false},
		{"minimum integer real equality", []any{int64(math.MinInt64)}, []any{float64(-9223372036854775808.0)}, true},
		{"minimum adjacent integer", []any{int64(math.MinInt64 + 1)}, []any{float64(-9223372036854775808.0)}, false},
		{"maximum integer rounded real distinction", []any{int64(math.MaxInt64)}, []any{float64(9223372036854775808.0)}, false},
		{"maximum adjacent integer", []any{int64(math.MaxInt64)}, []any{int64(math.MaxInt64 - 1)}, false},
		{"composite equal", []any{"prefix", int64(4)}, []any{"prefix", float64(4)}, true},
		{"composite trailing differs", []any{"prefix", int64(4)}, []any{"prefix", int64(5)}, false},
		{"composite leading differs", []any{"prefix", int64(4)}, []any{"other", int64(4)}, false},
		{"null scalar", []any{nil}, []any{nil}, false},
		{"existing null proposed nonnull", []any{nil}, []any{int64(1)}, false},
		{"existing nonnull proposed null", []any{int64(1)}, []any{nil}, false},
		{"null first field", []any{nil, int64(4)}, []any{nil, int64(4)}, false},
		{"null final field", []any{int64(4), nil}, []any{int64(4), nil}, false},
		{"all null", []any{nil, nil}, []any{nil, nil}, false},
		{"three columns null first", []any{nil, "middle", int64(4)}, []any{nil, "middle", int64(4)}, false},
		{"three columns null middle", []any{"first", nil, int64(4)}, []any{"first", nil, int64(4)}, false},
		{"three columns null last", []any{"first", "middle", nil}, []any{"first", "middle", nil}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			existing := append(append([]any(nil), tc.existing...), int64(-10))
			proposed := append(append([]any(nil), tc.proposed...), int64(20))
			m, idx := uniqueProbeFixture(t, len(tc.existing), existing)
			before := uniqueMemoryPages(m)
			err := probeUniqueIndex(context.Background(), m, idx, proposed, 0, false, uniqueProbeLimits())
			if tc.conflict {
				if !errors.Is(err, ErrConstraint) {
					t.Fatal("expected unique-prefix conflict", err)
				}
				var sqlErr *Error
				if wrapped := wrap("insert", err); !errors.As(wrapped, &sqlErr) || sqlErr.Code != CodeConstraint {
					t.Fatal("wrong public constraint class", wrapped)
				}
			} else if err != nil {
				t.Fatal("distinct or NULL key rejected", err)
			}
			if !reflect.DeepEqual(before, m.pages) {
				t.Fatal("read-only probe changed memory pages")
			}
			for _, value := range tc.proposed {
				if value == nil && m.reads != 0 {
					t.Fatal("NULL exemption read index pages", m.reads)
				}
			}
		})
	}
	// An empty physical tree and an ordinary nonunique descriptor both pass.
	m, idx := uniqueProbeFixture(t, 1)
	if err := probeUniqueIndex(context.Background(), m, idx, []any{int64(1), int64(1)}, 0, false, uniqueProbeLimits()); err != nil {
		t.Fatal(err)
	}
	m, idx = uniqueProbeFixture(t, 1, []any{"same", int64(1)})
	idx.unique = false
	if err := probeUniqueIndex(context.Background(), m, idx, []any{"same", int64(2)}, 0, false, uniqueProbeLimits()); err != nil || m.reads != 0 {
		t.Fatal("nonunique index was probed", err, m.reads)
	}
}

func TestUniqueIndexProbeExactSelfExclusion(t *testing.T) {
	for _, oldID := range []int64{math.MinInt64, -7, 0, 7, math.MaxInt64} {
		for _, changedID := range []bool{false, true} {
			newID := oldID
			if changedID {
				newID = 123
			}
			m, idx := uniqueProbeFixture(t, 2, []any{"same", int64(9), oldID})
			key := []any{"same", int64(9), newID}
			if err := probeUniqueIndex(context.Background(), m, idx, key, oldID, true, uniqueProbeLimits()); err != nil {
				t.Fatal("unchanged key must exclude exactly its old rowid", oldID, newID, err)
			}
			if err := probeUniqueIndex(context.Background(), m, idx, key, oldID, false, uniqueProbeLimits()); !errors.Is(err, ErrConstraint) {
				t.Fatal("disabled exclusion treated signed rowid as a sentinel", oldID, newID, err)
			}
			if err := probeUniqueIndex(context.Background(), m, idx, key, 124, true, uniqueProbeLimits()); !errors.Is(err, ErrConstraint) {
				t.Fatal("excluded an unrelated rowid", oldID, newID, err)
			}
			// Even a logically inconsistent prefix cannot pass after only the
			// excluded entry. Test both signed ordering positions of that entry.
			otherID := int64(-1)
			if err := btree.InsertIndex(context.Background(), m, idx.root, btree.KeyInfo{Columns: 2}, []any{"same", int64(9), otherID}); err != nil {
				t.Fatal(err)
			}
			if err := probeUniqueIndex(context.Background(), m, idx, key, oldID, true, uniqueProbeLimits()); !errors.Is(err, ErrConstraint) {
				t.Fatal("another row with the same prefix was ignored", oldID, newID, err)
			}
		}
	}
}

func TestUniqueIndexProbeBoundedSeek(t *testing.T) {
	const count = 1600
	keys := make([][]any, count)
	for i := range keys {
		keys[i] = []any{int64(i * 2), int64(i)}
	}
	m, idx := uniqueProbeFixture(t, 1, keys...)
	before := uniqueMemoryPages(m)
	for _, tc := range []struct {
		value    int64
		conflict bool
	}{{3198, true}, {3197, false}, {3200, false}} {
		m.reads = 0
		err := probeUniqueIndex(context.Background(), m, idx, []any{tc.value, int64(-20)}, 0, false, uniqueProbeLimits())
		if tc.conflict && !errors.Is(err, ErrConstraint) || !tc.conflict && err != nil {
			t.Fatal(tc, err)
		}
		if m.reads == 0 || m.reads > 8 {
			t.Fatal("prefix probe did not stay within a short B-tree descent", tc, m.reads)
		}
	}
	prefixReads := m.reads
	m.reads = 0
	cursor, err := btree.NewIndexCursor(context.Background(), m, idx.root, btree.KeyInfo{Columns: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer cursor.Close()
	seen := 0
	for {
		_, err := cursor.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		seen++
	}
	if seen != count || prefixReads >= m.reads/4 || !reflect.DeepEqual(before, m.pages) {
		t.Fatal("probe enumerated unrelated pages or changed source", prefixReads, m.reads, seen)
	}
}

type uniqueProbeAdmissionReader struct {
	btree.PageReader
	hook   string
	deny   error
	cancel context.CancelFunc
	calls  int
}

func (r *uniqueProbeAdmissionReader) admit(hook string) error {
	if r.hook != hook {
		return nil
	}
	r.calls++
	if r.cancel != nil {
		r.cancel()
	}
	return r.deny
}

func (r *uniqueProbeAdmissionReader) BeforePage(int64) error { return r.admit("page") }
func (r *uniqueProbeAdmissionReader) BeforeRecord(int) error { return r.admit("record") }

func TestUniqueIndexProbeBudgetsAndCancellation(t *testing.T) {
	key := []any{strings.Repeat("long\x00key", 700), int64(-1)}
	m, idx := uniqueProbeFixture(t, 1, key)
	before := uniqueMemoryPages(m)
	for _, limits := range []btree.RangeLimits{{PageReads: 1}, {Work: 1}, {Records: 1}, {PayloadBytes: 1}} {
		m.reads = 0
		err := probeUniqueIndex(context.Background(), m, idx, key, -1, true, limits)
		if !errors.Is(err, record.ErrLimit) || !errors.Is(wrap("update", err), ErrMemoryLimit) {
			t.Fatal("exhausted probe cannot establish uniqueness", limits, err)
		}
		if limits.PageReads > 0 && int64(m.reads) > limits.PageReads {
			t.Fatal("probe exceeded admitted page reads", limits, m.reads)
		}
	}
	quota := errors.New("upstream descriptor-free unique probe quota")
	for _, hook := range []string{"page", "record"} {
		r := &uniqueProbeAdmissionReader{PageReader: m, hook: hook, deny: quota}
		if err := probeUniqueIndex(context.Background(), r, idx, key, 0, false, uniqueProbeLimits()); !errors.Is(err, quota) || r.calls != 1 {
			t.Fatal("lost upstream admission refusal", hook, err, r.calls)
		}
		ctx, cancel := context.WithCancel(context.Background())
		r = &uniqueProbeAdmissionReader{PageReader: m, hook: hook, cancel: cancel}
		err := probeUniqueIndex(ctx, r, idx, key, 0, false, uniqueProbeLimits())
		cancel()
		if !errors.Is(err, context.Canceled) || r.calls != 1 {
			t.Fatal("lost admission cancellation", hook, err, r.calls)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m.reads = 0
	if err := probeUniqueIndex(ctx, m, idx, key, 0, false, uniqueProbeLimits()); !errors.Is(err, context.Canceled) || m.reads != 0 {
		t.Fatal("pre-canceled probe read pages or succeeded", err, m.reads)
	}
	if err := probeUniqueIndex(ctx, m, idx, []any{nil, int64(3)}, 0, false, uniqueProbeLimits()); !errors.Is(err, context.Canceled) || m.reads != 0 {
		t.Fatal("NULL exemption swallowed pre-existing cancellation", err, m.reads)
	}
	ctx, cancel = context.WithCancel(context.Background())
	m.reads, m.cancelAt, m.cancel = 0, 2, cancel
	err := probeUniqueIndex(ctx, m, idx, key, 0, false, uniqueProbeLimits())
	cancel()
	m.cancel = nil
	if !errors.Is(err, context.Canceled) || m.reads != 2 {
		t.Fatal("lost cancellation during overflow read", err, m.reads)
	}
	if !reflect.DeepEqual(before, m.pages) {
		t.Fatal("failed probes changed memory pages")
	}
}

type uniqueCheckRow struct {
	id   int64
	a, b any
}

func uniqueCheckFixture(t *testing.T, unique bool, rows []uniqueCheckRow) (*DB, *checkMemory) {
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
	indexSQL := "CREATE INDEX uq ON t(a,b)"
	if unique {
		indexSQL = "CREATE UNIQUE INDEX uq ON t(a,b)"
	}
	for _, e := range []struct {
		id     int64
		values []any
	}{
		{1, []any{"table", "t", "t", int64(tableRoot), "CREATE TABLE t(id INTEGER PRIMARY KEY,a,b)"}},
		{2, []any{"index", "uq", "t", int64(indexRoot), indexSQL}},
	} {
		if err := btree.InsertTable(ctx, m, 1, e.id, e.values); err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range rows {
		if err := btree.InsertTable(ctx, m, tableRoot, row.id, []any{nil, row.a, row.b}); err != nil {
			t.Fatal(err)
		}
		if err := btree.InsertIndex(ctx, m, indexRoot, btree.KeyInfo{Columns: 2}, []any{row.a, row.b, row.id}); err != nil {
			t.Fatal(err)
		}
	}
	m.reads = 0
	return d, m
}

func TestUniqueIndexCheckLogicalPrefixes(t *testing.T) {
	for _, tc := range []struct {
		name string
		rows []uniqueCheckRow
		bad  bool
	}{
		{"empty", nil, false},
		{"same complete prefix", []uniqueCheckRow{{-1, "same", int64(2)}, {0, "same", int64(2)}}, true},
		{"integer real equal", []uniqueCheckRow{{-1, int64(1), "same"}, {0, float64(1), "same"}}, true},
		{"signed zero equal", []uniqueCheckRow{{-1, math.Copysign(0, -1), "same"}, {0, int64(0), "same"}}, true},
		{"minimum signed equality", []uniqueCheckRow{{math.MinInt64, int64(math.MinInt64), "same"}, {math.MaxInt64, float64(-9223372036854775808.0), "same"}}, true},
		{"leading prefix only", []uniqueCheckRow{{-1, "same", int64(1)}, {0, "same", int64(2)}}, false},
		{"exact large integers", []uniqueCheckRow{{-1, int64(9007199254740992), "same"}, {0, int64(9007199254740993), "same"}}, false},
		{"rounded maximum real", []uniqueCheckRow{{-1, int64(math.MaxInt64), "same"}, {0, float64(9223372036854775808.0), "same"}}, false},
		{"adjacent minimum integer", []uniqueCheckRow{{-1, int64(math.MinInt64 + 1), "same"}, {0, float64(-9223372036854775808.0), "same"}}, false},
		{"text blob storage classes", []uniqueCheckRow{{-1, "same", "suffix"}, {0, []byte("same"), "suffix"}}, false},
		{"null first field", []uniqueCheckRow{{-1, nil, "same"}, {0, nil, "same"}}, false},
		{"null second field", []uniqueCheckRow{{-1, "same", nil}, {0, "same", nil}}, false},
		{"null both fields", []uniqueCheckRow{{-1, nil, nil}, {0, nil, nil}}, false},
		{"duplicates after nulls", []uniqueCheckRow{{-2, "same", nil}, {-1, "same", nil}, {0, "same", int64(2)}, {1, "same", float64(2)}}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, m := uniqueCheckFixture(t, true, tc.rows)
			before := uniqueMemoryPages(m)
			got, err := checkMemoryRun(d, m, CheckOptions{})
			if tc.bad {
				if !errors.Is(err, ErrCorrupt) || errors.Is(err, ErrConstraint) || got != (CheckResult{}) {
					t.Fatal("logical UNIQUE inconsistency must fail integrity checking", got, err)
				}
			} else if err != nil || got.Tables != 1 || got.Indexes != 1 || got.TableRows != uint64(len(tc.rows)) || got.IndexEntries != uint64(len(tc.rows)) {
				t.Fatal("valid NULL/distinct numeric prefix rejected", got, err)
			}
			if !reflect.DeepEqual(before, m.pages) {
				t.Fatal("native checker changed its memory source")
			}
			if tc.bad {
				d, m = uniqueCheckFixture(t, false, tc.rows)
				if _, err := checkMemoryRun(d, m, CheckOptions{}); err != nil {
					t.Fatal("ordinary nonunique index no longer permits duplicate prefixes", err)
				}
			}
		})
	}
}

func TestUniqueIndexCheckBudgetsAndCancellation(t *testing.T) {
	d, m := uniqueCheckFixture(t, true, []uniqueCheckRow{{-1, nil, "same"}, {0, nil, "same"}, {1, "one", int64(1)}, {2, "two", int64(2)}})
	before := uniqueMemoryPages(m)
	got, err := checkMemoryRun(d, m, CheckOptions{})
	if err != nil || got.PageReads == 0 {
		t.Fatal(got, err)
	}
	if exact, err := checkMemoryRun(d, m, CheckOptions{MaxPages: got.Pages, MaxPageReads: got.PageReads}); err != nil || exact != got {
		t.Fatal("exact checker budget changed", exact, got, err)
	}
	for _, limits := range []CheckOptions{{MaxPages: got.Pages - 1}, {MaxPageReads: got.PageReads - 1}} {
		if result, err := checkMemoryRun(d, m, limits); !errors.Is(err, ErrMemoryLimit) || result != (CheckResult{}) {
			t.Fatal("incomplete check must not report success", result, err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.reads, m.cancelAt, m.cancel = 0, 6, cancel
	options, err := checkOptions(nil)
	if err != nil {
		t.Fatal(err)
	}
	result, err := d.checkSnapshot(ctx, m, options)
	cancel()
	m.cancel = nil
	if !errors.Is(err, context.Canceled) || result != (CheckResult{}) {
		t.Fatal("canceled unique-index check reported success", result, err)
	}
	if !reflect.DeepEqual(before, m.pages) {
		t.Fatal("budgeted checker changed memory source")
	}
}

func TestUniqueIndexCheckSplitTreeDuplicate(t *testing.T) {
	// The unique claim is the only logical inconsistency. Every source row has
	// its matching complete key and all pages are created by ordinary inserts.
	const count = 320
	rows := make([]uniqueCheckRow, count+1)
	for i := range count {
		rows[i] = uniqueCheckRow{id: int64(i), a: int64(i), b: "suffix"}
	}
	rows[count] = uniqueCheckRow{id: -1, a: float64(count / 2), b: "suffix"}
	d, m := uniqueCheckFixture(t, true, rows)
	if m.h.PageCount <= 3 {
		t.Fatal("fixture did not split its table and index trees")
	}
	before := uniqueMemoryPages(m)
	if got, err := checkMemoryRun(d, m, CheckOptions{}); !errors.Is(err, ErrCorrupt) || got != (CheckResult{}) {
		t.Fatal("split-tree duplicate prefix was missed", got, err)
	}
	if !reflect.DeepEqual(before, m.pages) {
		t.Fatal("split-tree checker changed memory source")
	}
	d, m = uniqueCheckFixture(t, false, rows)
	if got, err := checkMemoryRun(d, m, CheckOptions{}); err != nil || got.TableRows != count+1 || got.IndexEntries != count+1 {
		t.Fatal("same split-tree fixture must be valid without a UNIQUE claim", got, err)
	}
}

func TestUniqueIndexWriteAdmission(t *testing.T) {
	for _, typ := range []string{"", "TEXT", "BLOB", "INTEGER", "REAL", "NUMERIC", "VECTOR(2)", "HALFVEC(2)", "BIT(2)", "SPARSEVEC(2)"} {
		t.Run(typ, func(t *testing.T) {
			tab, err := writableTable(&sqlast.CreateTable{Name: "t", Columns: []sqlast.ColumnDef{{Name: "id", Type: "INTEGER", PrimaryKey: true}, {Name: "a", Type: typ}, {Name: "b", Type: "TEXT"}}}, 2)
			if err != nil {
				t.Fatal(err)
			}
			def := &sqlast.CreateIndex{Name: "uq", Table: "t", Unique: true, Columns: []string{"a", "b"}, Desc: []bool{false, false}}
			idx, err := writableIndex(def, tab, 3)
			if sqlast.IsVectorDeclaration(typ) {
				if !errors.Is(err, ErrUnsupported) {
					t.Fatal("typed vector UNIQUE index admitted", typ, idx, err)
				}
				entries := []btree.SchemaEntry{
					{Type: "table", Name: "t", TableName: "t", RootPage: 2, SQL: "CREATE TABLE t(id INTEGER PRIMARY KEY,a " + typ + ",b TEXT)"},
					{Type: "index", Name: "uq", TableName: "t", RootPage: 3, SQL: "CREATE UNIQUE INDEX uq ON t(a,b)"},
				}
				if _, err := buildWriteCatalog(context.Background(), entries, &extensionCatalog{}); !errors.Is(err, ErrUnsupported) {
					t.Fatal("catalog reconstruction admitted typed vector UNIQUE index", typ, err)
				}
				entries[1].SQL = "CREATE INDEX uq ON t(a,b)"
				if catalog, err := buildWriteCatalog(context.Background(), entries, &extensionCatalog{}); err != nil || len(catalog.tables["t"].indexes) != 1 || catalog.tables["t"].indexes[0].unique {
					t.Fatal("catalog reconstruction changed nonunique typed index admission", typ, catalog, err)
				}
				// A typed declaration elsewhere on the table does not prevent
				// an ordinary unique index on independent scalar/IPK fields.
				def.Columns = []string{"id", "b"}
				idx, err = writableIndex(def, tab, 3)
			}
			if err != nil || !idx.unique || idx.root != 3 || idx.name != "uq" || len(idx.columns) != 2 {
				t.Fatal("ordinary UNIQUE metadata lost", typ, idx, err)
			}
			def.Unique = false
			def.Columns = []string{"a", "b"}
			if idx, err := writableIndex(def, tab, 3); err != nil || idx.unique {
				t.Fatal("nonunique descriptor admission changed", typ, idx, err)
			}
		})
	}
	tab, err := writableTable(&sqlast.CreateTable{Name: "t", Columns: []sqlast.ColumnDef{{Name: "a", Type: "TEXT"}, {Name: "b", Type: "TEXT"}}}, 2)
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"CREATE UNIQUE INDEX uq ON t(a DESC,b)", "CREATE UNIQUE INDEX uq ON t(a,b DESC)"} {
		stmt, err := sqlast.Parse(query)
		if err != nil {
			t.Fatal("DESC must retain existing parser representation", err)
		}
		if _, err := writableIndex(stmt.(*sqlast.CreateIndex), tab, 3); !errors.Is(err, ErrUnsupported) {
			t.Fatal("DESC UNIQUE write admission expanded", err)
		}
	}
	for _, method := range []string{"grams13", "hnsw", "ivfflat", "custom_method"} {
		def := &sqlast.CreateIndex{Name: "uq", Table: "t", Unique: true, Columns: []string{"a"}, Method: method}
		if _, err := writableIndex(def, tab, 3); !errors.Is(err, ErrUnsupported) {
			t.Fatal("extension UNIQUE descriptor admitted into ordinary catalog", method, err)
		}
		// A manually supplied AST must fail before dispatch touches a storage
		// transaction or extension catalog, even though the parser also rejects it.
		if err := (&Tx{}).createSQLMethodIndex(context.Background(), nil, def); !errors.Is(err, ErrUnsupported) {
			t.Fatal("extension dispatcher lost UNIQUE defense", method, err)
		}
	}
}

func TestUniqueIndexCatalogAndPlannerBoundary(t *testing.T) {
	for _, unique := range []bool{false, true} {
		d, m := uniqueCheckFixture(t, unique, []uniqueCheckRow{{0, int64(7), "suffix"}})
		ctx := context.Background()
		catalog, err := (&Tx{db: d, reader: m}).validateCatalog(ctx)
		if err != nil {
			t.Fatal(err)
		}
		tab := catalog.tables["t"]
		if tab == nil || len(tab.indexes) != 1 || tab.indexes[0].unique != unique || !reflect.DeepEqual(tab.indexes[0].columns, []int{1, 2}) {
			t.Fatal("catalog reload lost explicit UNIQUE declaration", catalog)
		}
		idx, _, err := ordinaryReadIndex(ctx, m, tab.table, map[int]any{1: int64(7)})
		if err != nil || unique && idx != nil || !unique && (idx == nil || idx.name != "uq") {
			t.Fatal("equality planner admission widened or ordinary index was lost", unique, idx, err)
		}
		bound := &secondaryInterval{}
		if err := bound.add(">=", int64(7)); err != nil {
			t.Fatal(err)
		}
		idx, selected, err := secondaryReadIndex(ctx, m, tab.table, map[int]*secondaryInterval{1: bound})
		if err != nil || unique && (idx != nil || selected != nil) || !unique && (idx == nil || idx.name != "uq" || selected != bound) {
			t.Fatal("range planner admission widened or ordinary index was lost", unique, idx, selected, err)
		}
	}
}
