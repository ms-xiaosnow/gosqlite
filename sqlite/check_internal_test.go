package sqlite

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	"github.com/ms-xiaosnow/gosqlite/internal/pager"
	"github.com/ms-xiaosnow/gosqlite/internal/record"
)

// Every malformed fixture in this file is exclusively an in-memory PageReader.
// No fixture bytes are written to a native file or routed through a VFS.
type checkMemory struct {
	h        pager.Header
	pages    map[uint32][]byte
	reads    int
	cancel   context.CancelFunc
	cancelAt int
}

func newCheckMemory() *checkMemory {
	p := make([]byte, 512)
	copy(p, "SQLite format 3\x00")
	binary.BigEndian.PutUint16(p[16:], 512)
	p[18], p[19] = 1, 1
	p[21], p[22], p[23] = 64, 32, 32
	binary.BigEndian.PutUint32(p[28:], 1)
	binary.BigEndian.PutUint32(p[44:], 4)
	binary.BigEndian.PutUint32(p[56:], 1)
	p[100] = 13
	binary.BigEndian.PutUint16(p[105:], 512)
	h, _ := pager.ParseHeader(p, 512)
	return &checkMemory{h: h, pages: map[uint32][]byte{1: p}}
}
func (m *checkMemory) Header() pager.Header { return m.h }
func (m *checkMemory) ReadPage(ctx context.Context, n uint32) ([]byte, error) {
	m.reads++
	if m.cancel != nil && m.reads == m.cancelAt {
		m.cancel()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p, ok := m.pages[n]
	if !ok {
		return nil, pager.ErrCorrupt
	}
	return append([]byte(nil), p...), nil
}
func (m *checkMemory) WritePage(ctx context.Context, n uint32, p []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.pages[n] = append([]byte(nil), p...)
	return nil
}
func (m *checkMemory) AllocatePage(ctx context.Context) (uint32, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	m.h.PageCount++
	m.pages[m.h.PageCount] = make([]byte, m.h.PageSize)
	binary.BigEndian.PutUint32(m.pages[1][28:], m.h.PageCount)
	return m.h.PageCount, nil
}
func (m *checkMemory) FreePage(context.Context, uint32) error {
	return fmt.Errorf("fixture never frees pages")
}
func checkFixture(t *testing.T) (*DB, *checkMemory) {
	t.Helper()
	d, err := copyValidator()
	if err != nil {
		t.Fatal(err)
	}
	m := newCheckMemory()
	ctx := context.Background()
	table, err := btree.CreateTree(ctx, m, btree.TableTree)
	if err != nil {
		t.Fatal(err)
	}
	index, err := btree.CreateTree(ctx, m, btree.IndexTree)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range []struct {
		id int64
		v  []any
	}{{1, []any{"table", "t", "t", int64(table), "CREATE TABLE t(id INTEGER PRIMARY KEY,k TEXT NOT NULL,n NUMERIC)"}}, {2, []any{"index", "idx", "t", int64(index), "CREATE INDEX idx ON t(k,n)"}}} {
		if err = btree.InsertTable(ctx, m, 1, e.id, e.v); err != nil {
			t.Fatal(err)
		}
	}
	for _, e := range []struct {
		id int64
		k  string
		n  any
	}{{-1, "", nil}, {1, "a", int64(9007199254740993)}, {2, "数据库", []byte{0, 1, 2}}, {3, "z", float64(1.25)}} {
		if err = btree.InsertTable(ctx, m, table, e.id, []any{nil, e.k, e.n}); err != nil {
			t.Fatal(err)
		}
		if err = btree.InsertIndex(ctx, m, index, btree.KeyInfo{Columns: 2}, []any{e.k, e.n, e.id}); err != nil {
			t.Fatal(err)
		}
	}
	m.reads = 0
	return d, m
}
func checkMemoryRun(d *DB, m *checkMemory, options CheckOptions) (CheckResult, error) {
	o, err := checkOptions([]CheckOptions{options})
	if err != nil {
		return CheckResult{}, err
	}
	r, err := d.checkSnapshot(context.Background(), m, o)
	return r, wrap("check", err)
}
func TestNativeCheckMemoryIntactAndBudgets(t *testing.T) {
	d, m := checkFixture(t)
	before := map[uint32][]byte{}
	for n, p := range m.pages {
		before[n] = append([]byte(nil), p...)
	}
	got, err := checkMemoryRun(d, m, CheckOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Scope != CheckScope || got.Tables != 1 || got.Indexes != 1 || got.TableRows != 4 || got.IndexEntries != 4 || got.PageReads != uint64(m.reads) {
		t.Fatal(got, m.reads)
	}
	if !reflect.DeepEqual(before, m.pages) {
		t.Fatal("read-only check changed memory source")
	}
	exact, err := checkMemoryRun(d, m, CheckOptions{MaxPages: got.Pages, MaxPageReads: got.PageReads})
	if err != nil || exact != got {
		t.Fatal(exact, err)
	}
	for _, o := range []CheckOptions{{MaxPages: got.Pages - 1}, {MaxPageReads: got.PageReads - 1}} {
		r, err := checkMemoryRun(d, m, o)
		if !errors.Is(err, ErrMemoryLimit) || r != (CheckResult{}) {
			t.Fatal(r, err)
		}
	}
	if _, err = checkOptions([]CheckOptions{{}, {}}); err == nil {
		t.Fatal("multiple options")
	}
	if _, err = checkOptions([]CheckOptions{{MaxPages: btree.MaxVisitedPages + 1}}); !errors.Is(err, ErrUnsupported) {
		t.Fatal(err)
	}
	if _, err = checkOptions([]CheckOptions{{MaxPageReads: 1<<32 + 1}}); !errors.Is(err, ErrUnsupported) {
		t.Fatal(err)
	}
}
func TestNativeCheckMemoryRejectsInconsistency(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*testing.T, *checkMemory)
		kind   error
	}{
		{"missing expected key", func(t *testing.T, m *checkMemory) {
			if _, err := btree.DeleteIndex(context.Background(), m, 3, btree.KeyInfo{Columns: 2}, []any{"a", int64(9007199254740993), int64(1)}); err != nil {
				t.Fatal(err)
			}
		}, ErrCorrupt},
		{"extra wrong value same rowid", func(t *testing.T, m *checkMemory) {
			if err := btree.InsertIndex(context.Background(), m, 3, btree.KeyInfo{Columns: 2}, []any{"bad", nil, int64(1)}); err != nil {
				t.Fatal(err)
			}
		}, ErrCorrupt},
		{"orphan rowid", func(t *testing.T, m *checkMemory) {
			if err := btree.InsertIndex(context.Background(), m, 3, btree.KeyInfo{Columns: 2}, []any{"bad", nil, int64(44)}); err != nil {
				t.Fatal(err)
			}
		}, ErrCorrupt},
		{"null constraint", func(t *testing.T, m *checkMemory) {
			if err := btree.InsertTable(context.Background(), m, 2, 8, []any{nil, nil, nil}); err != nil {
				t.Fatal(err)
			}
		}, ErrCorrupt},
		{"ipk slot", func(t *testing.T, m *checkMemory) {
			if err := btree.InsertTable(context.Background(), m, 2, 8, []any{int64(8), "new", nil}); err != nil {
				t.Fatal(err)
			}
		}, ErrCorrupt},
		{"extra record field", func(t *testing.T, m *checkMemory) {
			if err := btree.InsertTable(context.Background(), m, 2, 8, []any{nil, "new", nil, int64(1)}); err != nil {
				t.Fatal(err)
			}
		}, ErrCorrupt},
		{"unsupported encoding", func(t *testing.T, m *checkMemory) {
			m.h.TextEncoding = 2
			binary.BigEndian.PutUint32(m.pages[1][56:], 2)
		}, ErrUnsupported},
		{"unsupported invalid text", func(t *testing.T, m *checkMemory) {
			if err := btree.InsertTable(context.Background(), m, 2, 8, []any{nil, string([]byte{255}), nil}); err != nil {
				t.Fatal(err)
			}
		}, ErrUnsupported},
		{"unknown schema", func(t *testing.T, m *checkMemory) {
			if err := btree.InsertTable(context.Background(), m, 1, 3, []any{"view", "v", "v", int64(0), "CREATE VIEW v AS SELECT 1"}); err != nil {
				t.Fatal(err)
			}
		}, ErrUnsupported},
		{"shared root", func(t *testing.T, m *checkMemory) {
			if err := btree.InsertTable(context.Background(), m, 1, 3, []any{"table", "u", "u", int64(2), "CREATE TABLE u(k TEXT)"}); err != nil {
				t.Fatal(err)
			}
		}, ErrCorrupt},
		{"unaccounted page", func(t *testing.T, m *checkMemory) {
			if _, err := m.AllocatePage(context.Background()); err != nil {
				t.Fatal(err)
			}
		}, ErrCorrupt},
		{"free live page", func(t *testing.T, m *checkMemory) {
			binary.BigEndian.PutUint32(m.pages[1][32:], 2)
			binary.BigEndian.PutUint32(m.pages[1][36:], 1)
		}, ErrCorrupt},
		{"short page", func(t *testing.T, m *checkMemory) { m.pages[2] = m.pages[2][:10] }, ErrCorrupt},
		{"reserved serial", func(t *testing.T, m *checkMemory) {
			p := m.pages[2]
			cell := int(binary.BigEndian.Uint16(p[8:]))
			_, payloadSize, err := record.DecodeVarint(p[cell:])
			if err != nil {
				t.Fatal(err)
			}
			_, rowidSize, err := record.DecodeVarint(p[cell+payloadSize:])
			if err != nil {
				t.Fatal(err)
			}
			payload := cell + payloadSize + rowidSize
			_, headerSize, err := record.DecodeVarint(p[payload:])
			if err != nil {
				t.Fatal(err)
			}
			p[payload+headerSize] = 10
		}, ErrCorrupt},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d, m := checkFixture(t)
			tc.change(t, m)
			r, err := checkMemoryRun(d, m, CheckOptions{})
			if r != (CheckResult{}) || !errors.Is(err, tc.kind) {
				t.Fatal(r, err)
			}
		})
	}
}
func TestNativeCheckMemoryCancellationAndTypes(t *testing.T) {
	d, m := checkFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	m.cancel = cancel
	m.cancelAt = 6
	o, _ := checkOptions(nil)
	r, err := d.checkSnapshot(ctx, m, o)
	if !errors.Is(err, context.Canceled) || r != (CheckResult{}) {
		t.Fatal(r, err)
	}
	for _, typ := range []string{"VECTOR(2)", "HALFVEC(2)", "BIT(2)", "SPARSEVEC(2)"} {
		t.Run(typ, func(t *testing.T) {
			d, m := checkFixture(t)
			ctx := context.Background()
			if _, err := btree.DeleteTable(ctx, m, 1, 2); err != nil {
				t.Fatal(err)
			}
			if _, err := btree.DeleteTable(ctx, m, 1, 1); err != nil {
				t.Fatal(err)
			}
			if err := btree.InsertTable(ctx, m, 1, 1, []any{"table", "t", "t", int64(2), "CREATE TABLE t(id INTEGER PRIMARY KEY,k TEXT NOT NULL,n " + typ + ")"}); err != nil {
				t.Fatal(err)
			}
			if err := btree.InsertTable(ctx, m, 1, 2, []any{"index", "idx", "t", int64(3), "CREATE INDEX idx ON t(k,n)"}); err != nil {
				t.Fatal(err)
			}
			if r, err := checkMemoryRun(d, m, CheckOptions{}); err == nil || r != (CheckResult{}) {
				t.Fatal(r, err)
			}
		})
	}
}
func TestNativeCheckMemoryShadowOwners(t *testing.T) {
	byID := map[int64]*extensionIndex{1: {id: 1, method: "grams13"}, 2: {id: 2, method: "hnsw"}, 3: {id: 3, method: "ivfflat"}}
	for _, name := range []string{extensionFieldsName, extensionPostingsName, extensionFrequencyName, annStateName, annNodesName, annLiveName, annCentroidsName, annMembersName} {
		if err := checkShadowOwner(name, []any{nil, int64(9)}, byID); !errors.Is(err, ErrCorrupt) {
			t.Fatal(name, err)
		}
	}
	if err := checkShadowOwner(annMembersName, []any{nil, int64(2)}, byID); !errors.Is(err, ErrCorrupt) {
		t.Fatal(err)
	}
}

func TestNativeCheckTransactionOwnershipAndCounters(t *testing.T) {
	ctx := context.Background()
	db, err := Open(t.TempDir()+"/counters.db", Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{"CREATE TABLE t(id INTEGER PRIMARY KEY,k TEXT)", "CREATE INDEX idx ON t(k)", "INSERT INTO t VALUES(1,'one'),(2,'two')"} {
		if _, err = db.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	before := db.counters
	if _, err = db.Check(ctx); err != nil {
		t.Fatal(err)
	}
	if db.counters != before || db.tx != nil || len(db.rows) != 0 {
		t.Fatal("standalone check changed connection state")
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if tx.reader != nil {
		t.Fatal("BEGIN was not lazy")
	}
	if _, err = tx.Check(ctx); err != nil {
		t.Fatal(err)
	}
	reader := tx.reader
	if reader == nil || tx.storage == nil {
		t.Fatal("check did not retain deferred transaction snapshot")
	}
	rows, err := tx.Query(ctx, "SELECT id FROM t")
	if err != nil {
		t.Fatal(err)
	}
	if !rows.Next() || rows.Values()[0] != int64(1) {
		t.Fatal(rows.Err())
	}
	if _, err = tx.Check(ctx); err != nil {
		t.Fatal(err)
	}
	if !rows.Next() || rows.Values()[0] != int64(2) {
		t.Fatal("check advanced caller-owned cursor", rows.Err())
	}
	if err = rows.Close(); err != nil {
		t.Fatal(err)
	}
	if tx.reader != reader || db.counters != before || tx.activeRows != 0 {
		t.Fatal("check lost caller resources or counters")
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if r, err := tx.Check(ctx); !errors.Is(err, ErrTxDone) || r != (CheckResult{}) {
		t.Fatal(r, err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	if r, err := db.Check(ctx); !errors.Is(err, ErrClosed) || r != (CheckResult{}) {
		t.Fatal(r, err)
	}
}

func TestNativeCheckMemoryExtensionOrphanBeyondNamedScope(t *testing.T) {
	// Build only an intact database on disk, read it normally, then perform every
	// negative change exclusively in detached in-memory pages.
	ctx := context.Background()
	db, err := Open(t.TempDir()+"/intact.db", Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, q := range []string{"CREATE TABLE docs(id INTEGER PRIMARY KEY,body TEXT)", "INSERT INTO docs VALUES(1,'数据库')", "CREATE INDEX grams ON docs(body) USING grams13"} {
		if _, err = db.Exec(ctx, q); err != nil {
			t.Fatal(err)
		}
	}
	r, err := db.openReader(ctx)
	if err != nil {
		t.Fatal(err)
	}
	m := &checkMemory{h: r.Header(), pages: map[uint32][]byte{}}
	for n := uint32(1); n <= m.h.PageCount; n++ {
		p, err := r.ReadPage(ctx, n)
		if err != nil {
			t.Fatal(err)
		}
		m.pages[n] = p
	}
	if err = r.Close(); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	d, err := copyValidator()
	if err != nil {
		t.Fatal(err)
	}
	helper := &Tx{db: d, reader: m}
	catalog, err := helper.validateCatalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	tab := catalog.tables[extensionPostingsName]
	raw := []any{nil, int64(9999), int64(1), "match", int64(1), "orphan", int64(1234)}
	const rowID int64 = 100000
	if err = btree.InsertTable(ctx, m, tab.root, rowID, raw); err != nil {
		t.Fatal(err)
	}
	values, err := tab.values(rowID, raw)
	if err != nil {
		t.Fatal(err)
	}
	for _, idx := range tab.indexes {
		if err = btree.InsertIndex(ctx, m, idx.root, btree.KeyInfo{Columns: len(idx.columns)}, idx.key(rowID, values)); err != nil {
			t.Fatal(err)
		}
	}
	if err = d.checkExtensionIndex(ctx, m, "grams"); err != nil {
		t.Fatal("named check should remain scoped to its own valid logical index", err)
	}
	if got, err := checkMemoryRun(d, m, CheckOptions{}); !errors.Is(err, ErrCorrupt) || got != (CheckResult{}) {
		t.Fatal(got, err)
	}
}
