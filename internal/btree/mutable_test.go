package btree

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/pager"
)

// memoryPages exercises valid SQLite pages without file I/O or failure injection.
type memoryPages struct {
	h      pager.Header
	pages  map[uint32][]byte
	free   []uint32
	writes int
}

func newMemoryPages(size uint32) *memoryPages {
	p := make([]byte, size)
	p[100] = 13
	binary.BigEndian.PutUint16(p[105:], uint16(size))
	return &memoryPages{h: pager.Header{PageSize: size, UsableSize: size, PageCount: 1, TextEncoding: 1}, pages: map[uint32][]byte{1: p}}
}
func (p *memoryPages) Header() pager.Header { return p.h }
func (p *memoryPages) ReadPage(ctx context.Context, n uint32) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	v, ok := p.pages[n]
	if !ok {
		return nil, fmt.Errorf("missing page %d", n)
	}
	return append([]byte(nil), v...), nil
}
func (p *memoryPages) WritePage(ctx context.Context, n uint32, b []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if _, ok := p.pages[n]; !ok {
		return fmt.Errorf("missing write %d", n)
	}
	if len(b) != int(p.h.PageSize) {
		return fmt.Errorf("bad page size")
	}
	p.pages[n] = append([]byte(nil), b...)
	p.writes++
	return nil
}
func (p *memoryPages) AllocatePage(ctx context.Context) (uint32, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	n := p.h.PageCount + 1
	if len(p.free) > 0 {
		n = p.free[len(p.free)-1]
		p.free = p.free[:len(p.free)-1]
	} else {
		p.h.PageCount = n
	}
	p.pages[n] = make([]byte, p.h.PageSize)
	return n, nil
}
func (p *memoryPages) FreePage(ctx context.Context, n uint32) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if n <= 1 {
		return fmt.Errorf("free schema root")
	}
	if _, ok := p.pages[n]; !ok {
		return fmt.Errorf("double free %d", n)
	}
	delete(p.pages, n)
	p.free = append(p.free, n)
	return nil
}

func assertTableModel(t *testing.T, p *memoryPages, root uint32, model map[int64][]any) {
	t.Helper()
	keys := make([]int64, 0, len(model))
	for id := range model {
		keys = append(keys, id)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	c, err := NewCursor(context.Background(), p, root)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	for _, id := range keys {
		got, v, e := c.Next()
		if e != nil || got != id || !reflect.DeepEqual(v, model[id]) {
			t.Fatalf("row %d: got %d %#v, want %#v: %v", id, got, v, model[id], e)
		}
	}
	if _, _, e := c.Next(); e != io.EOF {
		t.Fatal("end", e)
	}
	last, ok, e := LastTableRowID(context.Background(), p, root)
	if e != nil || ok != (len(keys) > 0) || ok && last != keys[len(keys)-1] {
		t.Fatal("maximum", last, ok, e)
	}
}
func TestMutableTableModel(t *testing.T) {
	ctx := context.Background()
	for _, size := range []uint32{512, 4096, 65536} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			p := newMemoryPages(size)
			root, e := CreateTree(ctx, p, TableTree)
			if e != nil {
				t.Fatal(e)
			}
			model := make(map[int64][]any)
			rng := rand.New(rand.NewPCG(19, 23))
			for step := 0; step < 5000; step++ {
				id := int64(rng.IntN(800)) - 400
				op := rng.IntN(3)
				old, exists := model[id]
				if op == 0 || !exists {
					if exists {
						if e = InsertTable(ctx, p, root, id, []any{"duplicate"}); !errors.Is(e, ErrDuplicate) {
							t.Fatal(e)
						}
						continue
					}
					v := []any{nil, id, strings.Repeat("界", rng.IntN(500)), []byte{byte(step), 0}}
					if e = InsertTable(ctx, p, root, id, v); e != nil {
						t.Fatalf("insert step %d id %d: %v", step, id, e)
					}
					model[id] = v
				} else if op == 1 {
					v := []any{nil, id, strings.Repeat("x", rng.IntN(2500)), []byte{7, 8, 9}}
					if e = ReplaceTable(ctx, p, root, id, v); e != nil {
						t.Fatalf("replace step %d: %v", step, e)
					}
					model[id] = v
				} else {
					if ok, e := DeleteTable(ctx, p, root, id); e != nil || !ok {
						t.Fatalf("delete step %d: %v %v", step, ok, e)
					}
					delete(model, id)
				}
				_ = old
				if step%173 == 0 {
					assertTableModel(t, p, root, model)
				}
			}
			assertTableModel(t, p, root, model)
			for id, want := range model {
				v, ok, e := SeekTable(ctx, p, root, id)
				if e != nil || !ok || !reflect.DeepEqual(v, want) {
					t.Fatal("seek", id, ok, e)
				}
			}
			for id := range model {
				if ok, e := DeleteTable(ctx, p, root, id); e != nil || !ok {
					t.Fatalf("drain %d: %v", id, e)
				}
			}
			assertTableModel(t, p, root, nil)
			if len(p.pages) != 2 {
				t.Fatal("leaked table or overflow pages", len(p.pages))
			}
		})
	}
}
func TestMutableTableRootOneAndBoundaries(t *testing.T) {
	ctx := context.Background()
	p := newMemoryPages(512)
	header := append([]byte(nil), p.pages[1][:100]...)
	ids := []int64{math.MinInt64, math.MaxInt64, 0, -1, 1, -128, 128}
	model := make(map[int64][]any)
	for _, id := range ids {
		v := []any{strings.Repeat("z", 460)}
		if e := InsertTable(ctx, p, 1, id, v); e != nil {
			t.Fatal(e)
		}
		model[id] = v
	}
	assertTableModel(t, p, 1, model)
	if !reflect.DeepEqual(header, p.pages[1][:100]) {
		t.Fatal("schema header changed")
	}
	for _, id := range ids {
		if ok, e := DeleteTable(ctx, p, 1, id); !ok || e != nil {
			t.Fatal(ok, e)
		}
	}
	if len(p.pages) != 1 || p.pages[1][100] != 13 {
		t.Fatal("root collapse", len(p.pages), p.pages[1][100])
	}
	if e := ReplaceTable(ctx, p, 1, 42, nil); !errors.Is(e, ErrNotFound) {
		t.Fatal(e)
	}
	if ok, e := DeleteTable(ctx, p, 1, 42); ok || e != nil {
		t.Fatal(ok, e)
	}
}
func makeIndexKey(id int64, version int) []any {
	var v any
	switch (id + int64(version)) % 7 {
	case 0:
		v = nil
	case 1:
		v = int64(9007199254740992 + id)
	case 2:
		v = float64(id) / 7
	case 3:
		v = strings.Repeat("same", int(id%250))
	case 4:
		v = []byte(strings.Repeat("blob", int(id%270)))
	case 5:
		v = "common"
	case 6:
		v = math.Inf(1)
	}
	return []any{v, id}
}
func assertIndexModel(t *testing.T, p *memoryPages, root uint32, model map[int64][]any) {
	t.Helper()
	want := make([][]any, 0, len(model))
	for _, key := range model {
		want = append(want, key)
	}
	sort.Slice(want, func(i, j int) bool { return compareIndexKeys(want[i], want[j]) < 0 })
	c, e := NewIndexCursor(context.Background(), p, root, KeyInfo{1})
	if e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	for i, key := range want {
		got, e := c.Next()
		if e != nil || !reflect.DeepEqual(got, key) {
			t.Fatalf("index key %d got %#v, want %#v: %v", i, got, key, e)
		}
	}
	if _, e = c.Next(); e != io.EOF {
		t.Fatal("index end", e)
	}
}
func TestMutableIndexModel(t *testing.T) {
	ctx := context.Background()
	for _, size := range []uint32{512, 4096, 65536} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			p := newMemoryPages(size)
			root, e := CreateTree(ctx, p, IndexTree)
			if e != nil {
				t.Fatal(e)
			}
			model := make(map[int64][]any)
			rng := rand.New(rand.NewPCG(71, 13))
			info := KeyInfo{1}
			for step := 0; step < 4000; step++ {
				id := int64(rng.IntN(600))
				old, exists := model[id]
				if exists {
					if ok, e := DeleteIndex(ctx, p, root, info, old); e != nil || !ok {
						t.Fatalf("delete %d id %d: %v %v", step, id, ok, e)
					}
					delete(model, id)
				} else {
					key := makeIndexKey(id, step%7)
					if e := InsertIndex(ctx, p, root, info, key); e != nil {
						t.Fatalf("insert %d: %v", step, e)
					}
					model[id] = key
				}
				if step%139 == 0 {
					assertIndexModel(t, p, root, model)
				}
			}
			assertIndexModel(t, p, root, model)
			for _, key := range model {
				got, ok, e := SeekIndex(ctx, p, root, info, key)
				if e != nil || !ok || !reflect.DeepEqual(got, key) {
					t.Fatal("seek", key, ok, e)
				}
			}
			for _, key := range model {
				if ok, e := DeleteIndex(ctx, p, root, info, key); e != nil || !ok {
					t.Fatal("drain", key, ok, e)
				}
			}
			assertIndexModel(t, p, root, nil)
			if len(p.pages) != 2 {
				t.Fatal("leaked index or overflow pages", len(p.pages))
			}
		})
	}
}
func TestMutableLocalWorkAndReuse(t *testing.T) {
	ctx := context.Background()
	p := newMemoryPages(512)
	root, e := CreateTree(ctx, p, TableTree)
	if e != nil {
		t.Fatal(e)
	}
	for i := int64(0); i < 2000; i++ {
		if e = InsertTable(ctx, p, root, i, []any{strings.Repeat("r", 60)}); e != nil {
			t.Fatal(e)
		}
	}
	before := p.writes
	if e = ReplaceTable(ctx, p, root, 1000, []any{"small replacement"}); e != nil {
		t.Fatal(e)
	}
	if p.writes-before > 10 {
		t.Fatal("replacement rewrote too many pages", p.writes-before)
	}
	for i := int64(0); i < 2000; i++ {
		if _, e = DeleteTable(ctx, p, root, i); e != nil {
			t.Fatal(e)
		}
	}
	count := p.h.PageCount
	for i := int64(0); i < 2000; i++ {
		if e = InsertTable(ctx, p, root, i, []any{strings.Repeat("r", 60)}); e != nil {
			t.Fatal(e)
		}
	}
	if p.h.PageCount != count {
		t.Fatal("freed pages not reused", count, p.h.PageCount)
	}
}
