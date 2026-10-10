package sqlite

import (
	"context"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/grams13"
	"github.com/ms-xiaosnow/gosqlite/extension/registry"
	"github.com/ms-xiaosnow/gosqlite/internal/btree"
)

// Independent negative fixtures remain pure-memory maps; no altered bytes
// are passed to a file, VFS, pager session or driver.
func TestIndependentCheckFragmentAccounting(t *testing.T) {
	for _, page := range []uint32{1, 2, 3} {
		t.Run(string(rune('0'+page)), func(t *testing.T) {
			d, m := checkFixture(t)
			off := 0
			if page == 1 {
				off = 100
			}
			if m.pages[page][off+7] != 0 {
				t.Fatal("fixture unexpectedly fragmented")
			}
			m.pages[page][off+7] = 1
			got, err := checkMemoryRun(d, m, CheckOptions{})
			if err == nil || got != (CheckResult{}) {
				t.Fatalf("incorrect fragmented-byte count passed: result=%+v error=%v", got, err)
			}
		})
	}
}
func TestIndependentCheckUnrepresentedFreeBytes(t *testing.T) {
	d, m := checkFixture(t)
	p := m.pages[2]
	content := binary.BigEndian.Uint16(p[5:7])
	if content < 20 {
		t.Fatal("fixture has no header gap")
	}
	binary.BigEndian.PutUint16(p[5:7], content-8) // Eight bytes become unrepresented cell-content space.
	got, err := checkMemoryRun(d, m, CheckOptions{})
	if err == nil || got != (CheckResult{}) {
		t.Fatalf("unrepresented free bytes passed: result=%+v error=%v", got, err)
	}
}

type independentCustomGrams struct {
	calls   *int
	version int
	empty   bool
}

func (independentCustomGrams) Name() string { return grams13.MethodName }
func (m independentCustomGrams) Version() int {
	if m.version != 0 {
		return m.version
	}
	return 1
}
func (m independentCustomGrams) Configure(b []byte) (registry.Index, error) {
	if m.calls != nil {
		*m.calls++
	}
	if m.empty {
		return independentEmptyGrams{canonical: append([]byte(nil), b...), calls: m.calls}, nil
	}
	return grams13.Method().Configure(b)
}

type independentEmptyGrams struct {
	canonical []byte
	calls     *int
}

func (i independentEmptyGrams) Canonical() []byte { return append([]byte(nil), i.canonical...) }
func (i independentEmptyGrams) Keys(context.Context, string) ([]registry.Key, error) {
	if i.calls != nil {
		*i.calls++
	}
	return nil, nil
}

func independentDetachedGrams(t *testing.T, body ...string) (*DB, *checkMemory) {
	t.Helper()
	ctx := context.Background()
	db, err := Open(t.TempDir()+"/intact.db", Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	initial := "database"
	if len(body) > 0 {
		initial = body[0]
	}
	for _, q := range []string{"CREATE TABLE docs(id INTEGER PRIMARY KEY,body TEXT)", "INSERT INTO docs VALUES(1,'" + initial + "')", "CREATE INDEX grams ON docs(body) USING grams13"} {
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
	return d, m
}
func TestIndependentCheckCustomBuiltinNamedRegistry(t *testing.T) {
	d, m := independentDetachedGrams(t)
	calls := 0
	methods, err := registry.New(independentCustomGrams{calls: &calls})
	if err != nil {
		t.Fatal(err)
	}
	d.methods = methods
	got, err := checkMemoryRun(d, m, CheckOptions{})
	if err != nil || got.Scope != CheckScope || calls != 0 {
		t.Fatalf("full check must independently use builtin semantics without invoking custom callbacks: result=%+v error=%v calls=%d", got, err, calls)
	}
}
func TestIndependentCheckCatalogUTF8(t *testing.T) {
	d, m := checkFixture(t)
	ctx := context.Background()
	_, err := btree.DeleteTable(ctx, m, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	invalid := string([]byte{'c', 255})
	q := "CREATE TABLE t(id INTEGER PRIMARY KEY,k TEXT NOT NULL,n NUMERIC,\"" + invalid + "\" TEXT)"
	if err = btree.InsertTable(ctx, m, 1, 1, []any{"table", "t", "t", int64(2), q}); err != nil {
		t.Fatal(err)
	}
	got, err := checkMemoryRun(d, m, CheckOptions{})
	if err == nil || got != (CheckResult{}) {
		t.Fatalf("invalid schema TEXT passed: result=%+v error=%v", got, err)
	}
}

type independentVersionTwo struct{ independentCustomGrams }

func (independentVersionTwo) Version() int { return 2 }
func TestIndependentCheckUnknownGramsVersion(t *testing.T) {
	d, m := independentDetachedGrams(t)
	ctx := context.Background()
	c, err := (&Tx{db: d, reader: m}).validateCatalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	root := c.tables[extensionCatalogName].root
	raw, found, err := btree.SeekTable(ctx, m, root, 1)
	if err != nil || !found {
		t.Fatal(found, err)
	}
	if _, err = btree.DeleteTable(ctx, m, root, 1); err != nil {
		t.Fatal(err)
	}
	raw[5] = int64(2)
	if err = btree.InsertTable(ctx, m, root, 1, raw); err != nil {
		t.Fatal(err)
	}
	d.methods, err = registry.New(independentVersionTwo{})
	if err != nil {
		t.Fatal(err)
	}
	got, err := checkMemoryRun(d, m, CheckOptions{})
	if !errors.Is(err, ErrUnsupported) || got != (CheckResult{}) {
		t.Fatalf("unknown grams method version accepted: result=%+v error=%v", got, err)
	}
}

func TestIndependentCheckUnknownBuiltinVersion(t *testing.T) {
	d, m := independentDetachedGrams(t)
	ctx := context.Background()
	c, err := (&Tx{db: d, reader: m}).validateCatalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	root := c.extensions.roots[extensionCatalogName]
	row, ok, err := btree.SeekTable(ctx, m, root, 1)
	if err != nil || !ok {
		t.Fatal(ok, err)
	}
	row[5] = int64(2)
	if _, err = btree.DeleteTable(ctx, m, root, 1); err != nil {
		t.Fatal(err)
	}
	if err = btree.InsertTable(ctx, m, root, 1, row); err != nil {
		t.Fatal(err)
	}
	calls := 0
	d.methods, err = registry.New(independentCustomGrams{calls: &calls, version: 2})
	if err != nil {
		t.Fatal(err)
	}
	got, err := checkMemoryRun(d, m, CheckOptions{})
	if !errors.Is(err, ErrUnsupported) || got != (CheckResult{}) || calls != 0 {
		t.Fatal(got, err, calls)
	}
}
func TestIndependentCheckCustomEmptyKeysCannotPass(t *testing.T) {
	d, m := independentDetachedGrams(t, "")
	ctx := context.Background()
	c, err := (&Tx{db: d, reader: m}).validateCatalog(ctx)
	if err != nil {
		t.Fatal(err)
	}
	root := c.tables["docs"].root
	if _, err = btree.DeleteTable(ctx, m, root, 1); err != nil {
		t.Fatal(err)
	}
	if err = btree.InsertTable(ctx, m, root, 1, []any{nil, "database"}); err != nil {
		t.Fatal(err)
	}
	calls := 0
	d.methods, err = registry.New(independentCustomGrams{calls: &calls, empty: true})
	if err != nil {
		t.Fatal(err)
	}
	// The application-directed named check can accept its own empty-key semantics.
	if err = d.checkExtensionIndex(ctx, m, "grams"); err != nil || calls == 0 {
		t.Fatal(err, calls)
	}
	calls = 0
	got, err := checkMemoryRun(d, m, CheckOptions{})
	if !errors.Is(err, ErrCorrupt) || got != (CheckResult{}) || calls != 0 {
		t.Fatal(got, err, calls)
	}
}
func TestIndependentCheckValidMemoryPageSpace(t *testing.T) {
	for _, page := range []uint32{1, 2, 3} {
		for gap := 1; gap <= 8; gap++ {
			d, m := checkFixture(t)
			p := m.pages[page]
			off := 0
			if page == 1 {
				off = 100
			}
			content := int(binary.BigEndian.Uint16(p[off+5:]))
			top := content - gap
			binary.BigEndian.PutUint16(p[off+5:], uint16(top))
			if gap <= 3 {
				p[off+7] = byte(gap)
			} else {
				binary.BigEndian.PutUint16(p[off+1:], uint16(top))
				binary.BigEndian.PutUint16(p[top:], 0)
				binary.BigEndian.PutUint16(p[top+2:], uint16(gap))
			}
			if r, err := checkMemoryRun(d, m, CheckOptions{}); err != nil || r.Scope != CheckScope {
				t.Fatal(page, gap, r, err)
			}
		}
	}
	d, m := checkFixture(t)
	_ = d
	// An empty schema leaf with no fragments is valid; declared fragments are not.
	empty := newCheckMemory()
	if err := btree.CheckTreeSpace(context.Background(), empty, nil); err != nil {
		t.Fatal(err)
	}
	empty.pages[1][107] = 1
	if err := btree.CheckTreeSpace(context.Background(), empty, nil); err == nil {
		t.Fatal("empty-page fragments accepted")
	}
	// Force actual interior table/index pages without native I/O.
	for id := int64(10); id < 400; id++ {
		if err := btree.InsertTable(context.Background(), m, 2, id, []any{nil, "b", id}); err != nil {
			t.Fatal(err)
		}
		if err := btree.InsertIndex(context.Background(), m, 3, btree.KeyInfo{Columns: 2}, []any{"b", id, id}); err != nil {
			t.Fatal(err)
		}
	}
	if m.pages[2][0] != 5 || m.pages[3][0] != 2 {
		t.Fatal("expected interior roots")
	}
	roots := []btree.RootSpec{{Page: 2, Kind: btree.TableTree}, {Page: 3, Kind: btree.IndexTree, KeyInfo: btree.KeyInfo{Columns: 2}}}
	if err := btree.CheckTreeSpace(context.Background(), m, roots); err != nil {
		t.Fatal(err)
	}
	for _, page := range []uint32{2, 3} {
		m.pages[page][7] = 1
		if err := btree.CheckTreeSpace(context.Background(), m, roots); err == nil {
			t.Fatal("interior fragments accepted", page)
		}
		m.pages[page][7] = 0
	}
}
