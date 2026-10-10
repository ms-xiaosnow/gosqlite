package btree

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/pager"
	"github.com/ms-xiaosnow/gosqlite/internal/record"
)

// Review-only regression coverage uses in-memory valid pages exclusively.
func reviewTreeLevels(t *testing.T, p *memoryPages, root uint32, kind TreeKind, info KeyInfo) int {
	t.Helper()
	m, err := newMutation(context.Background(), p, kind, info)
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[uint32]bool)
	var walk func(uint32, bool) int
	walk = func(number uint32, isRoot bool) int {
		if seen[number] {
			t.Fatalf("repeated page %d", number)
		}
		seen[number] = true
		n, err := m.load(number)
		if err != nil {
			t.Fatal(err)
		}
		if !isRoot && len(n.entries) == 0 {
			t.Fatalf("empty non-root page %d", number)
		}
		if n.leaf {
			return 1
		}
		level := walk(n.children[0], false)
		for _, child := range n.children[1:] {
			if got := walk(child, false); got != level {
				t.Fatalf("unequal subtree heights %d and %d at page %d", level, got, number)
			}
		}
		return level + 1
	}
	return walk(root, true)
}

func reviewIndexModel(t *testing.T, p *memoryPages, root uint32, info KeyInfo, model map[int64][]any) {
	t.Helper()
	want := make([][]any, 0, len(model))
	for _, key := range model {
		want = append(want, key)
	}
	sort.Slice(want, func(i, j int) bool { return compareIndexKeys(want[i], want[j]) < 0 })
	c, err := NewIndexCursor(context.Background(), p, root, info)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	for i, key := range want {
		got, err := c.Next()
		if err != nil || !reflect.DeepEqual(got, key) {
			t.Fatalf("key %d mismatch: %v", i, err)
		}
	}
	if _, err := c.Next(); err != io.EOF {
		t.Fatal("index EOF", err)
	}
	if err := CheckOwnership(context.Background(), p, []RootSpec{{Page: root, Kind: IndexTree, KeyInfo: info}}, p.free); err != nil {
		t.Fatal(err)
	}
	reviewTreeLevels(t, p, root, IndexTree, info)
}

func TestReviewMutableInteriorIndexDrain(t *testing.T) {
	ctx := context.Background()
	for _, usable := range []uint32{480, 512, 1024, 2048, 8192} {
		t.Run(fmt.Sprint(usable), func(t *testing.T) {
			size := usable
			if size == 480 {
				size = 512
			}
			p := newMemoryPages(size)
			p.h.UsableSize = usable
			binary.BigEndian.PutUint16(p.pages[1][105:], uint16(usable))
			info := KeyInfo{Columns: 2}
			root, err := CreateTree(ctx, p, IndexTree)
			if err != nil {
				t.Fatal(err)
			}
			model := make(map[int64][]any)
			rng := rand.New(rand.NewPCG(739, uint64(usable)))
			for _, id := range rng.Perm(1800) {
				key := []any{int64(id), strings.Repeat("v", rng.IntN(int(usable)*3)), int64(id)}
				if err := InsertIndex(ctx, p, root, info, key); err != nil {
					t.Fatalf("insert %d: %v", id, err)
				}
				model[int64(id)] = key
			}
			reviewIndexModel(t, p, root, info, model)
			if depth := reviewTreeLevels(t, p, root, IndexTree, info); depth < 4 {
				t.Fatalf("fixture too shallow: %d", depth)
			}
			for step := 0; len(model) > 0; step++ {
				m, err := newMutation(ctx, p, IndexTree, info)
				if err != nil {
					t.Fatal(err)
				}
				n, err := m.load(root)
				if err != nil {
					t.Fatal(err)
				}
				// Alternate between root records and records in the deepest
				// interior level, to exercise predecessor propagation.
				if step%2 != 0 && !n.leaf {
					for {
						child, err := m.load(n.children[rng.IntN(len(n.children))])
						if err != nil {
							t.Fatal(err)
						}
						if child.leaf {
							break
						}
						n = child
					}
				}
				key, err := m.key(n.entries[rng.IntN(len(n.entries))])
				if err != nil {
					t.Fatal(err)
				}
				if ok, err := DeleteIndex(ctx, p, root, info, key); err != nil || !ok {
					t.Fatalf("delete step %d: %v %v", step, ok, err)
				}
				delete(model, key[0].(int64))
				if step%47 == 0 {
					reviewIndexModel(t, p, root, info, model)
				}
			}
			reviewIndexModel(t, p, root, info, model)
			if len(p.pages) != 2 {
				t.Fatalf("leaked %d pages", len(p.pages)-2)
			}
		})
	}
}

func TestReviewMutableTableVariableReplacements(t *testing.T) {
	ctx := context.Background()
	for _, size := range []uint32{512, 1024, 2048} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			p := newMemoryPages(size)
			model := make(map[int64][]any)
			rng := rand.New(rand.NewPCG(1843, uint64(size)))
			for step := 0; step < 5000; step++ {
				id := int64(rng.IntN(700))
				payload := rng.IntN(int(size) * 4)
				v := []any{strings.Repeat("x", payload)}
				_, exists := model[id]
				var err error
				if !exists {
					err = InsertTable(ctx, p, 1, id, v)
					model[id] = v
				} else if rng.IntN(4) == 0 {
					_, err = DeleteTable(ctx, p, 1, id)
					delete(model, id)
				} else {
					err = ReplaceTable(ctx, p, 1, id, v)
					model[id] = v
				}
				if err != nil {
					t.Fatalf("step %d: %v", step, err)
				}
				if step%151 == 0 {
					assertTableModel(t, p, 1, model)
					reviewTreeLevels(t, p, 1, TableTree, KeyInfo{})
					if err = CheckOwnership(ctx, p, nil, p.free); err != nil {
						t.Fatal(err)
					}
				}
			}
			assertTableModel(t, p, 1, model)
		})
	}
}

func TestReviewMutableIndexPayloadLimitRoundTrip(t *testing.T) {
	ctx := context.Background()
	p := newMemoryPages(4096)
	root, err := CreateTree(ctx, p, IndexTree)
	if err != nil {
		t.Fatal(err)
	}
	info := KeyInfo{Columns: 1}
	// This key needs six header bytes and one byte for its rowid. Keep the
	// encoded size assertions so a future record format change is explicit.
	key := []any{strings.Repeat("x", MaxMutableIndexPayload-7), int64(7)}
	encoded, err := record.Encode(key)
	if err != nil || len(encoded) != MaxMutableIndexPayload {
		t.Fatalf("boundary encoding: %d %v", len(encoded), err)
	}
	encoded = nil
	tooLarge := []any{key[0].(string) + "x", int64(7)}
	beforePages, beforeWrites := p.h.PageCount, p.writes
	if err = InsertIndex(ctx, p, root, info, tooLarge); !errors.Is(err, record.ErrLimit) {
		t.Fatalf("over-limit insert: %v", err)
	}
	if p.h.PageCount != beforePages || p.writes != beforeWrites || len(p.free) != 0 {
		t.Fatal("rejected oversized insert changed writer")
	}
	if err = InsertIndex(ctx, p, root, info, key); err != nil {
		t.Fatalf("boundary insert: %v", err)
	}
	if ok, err := DeleteIndex(ctx, p, root, info, key); err != nil || !ok {
		t.Fatalf("boundary delete: %v %v", ok, err)
	}
	if len(p.pages) != 2 {
		t.Fatalf("boundary key leaked %d pages", len(p.pages)-2)
	}
	if err := CheckOwnership(ctx, p, []RootSpec{{Page: root, Kind: IndexTree, KeyInfo: info}}, p.free); err != nil {
		t.Fatal(err)
	}
}

// These deliberately malformed fixtures exist only in memory. No disk file
// is created, read, changed, or recovered by this structural validation test.
func TestReviewCursorRejectsMalformedStructureMemory(t *testing.T) {
	ctx := context.Background()
	for _, kind := range []TreeKind{TableTree, IndexTree} {
		for _, defect := range []string{"unequal-depth", "empty-child", "empty-interior-root"} {
			t.Run(fmt.Sprintf("%d/%s", kind, defect), func(t *testing.T) {
				p := newMemoryPages(512)
				info := KeyInfo{Columns: 1}
				m, err := newMutation(ctx, p, kind, info)
				if err != nil {
					t.Fatal(err)
				}
				alloc := func(leaf bool) *mutableNode {
					n, err := m.allocate(leaf)
					if err != nil {
						t.Fatal(err)
					}
					return n
				}
				entry := func(id int64, interior bool) mutableEntry {
					if kind == TableTree && interior {
						return tableDivider(id)
					}
					payload, err := record.Encode([]any{id, id})
					if err != nil {
						t.Fatal(err)
					}
					e, err := m.makeEntry(id, payload)
					if err != nil {
						t.Fatal(err)
					}
					return e
				}
				root, left := alloc(false), alloc(true)
				left.entries = []mutableEntry{entry(1, false)}
				nodes := []*mutableNode{root, left}
				root.children = []uint32{left.number}
				if defect != "empty-interior-root" {
					right := alloc(defect != "unequal-depth")
					nodes = append(nodes, right)
					root.entries = []mutableEntry{entry(10, true)}
					root.children = append(root.children, right.number)
					if defect == "empty-child" {
						left.entries = nil
						right.entries = []mutableEntry{entry(20, false)}
					} else {
						a, b := alloc(true), alloc(true)
						a.entries, b.entries = []mutableEntry{entry(15, false)}, []mutableEntry{entry(25, false)}
						right.entries = []mutableEntry{entry(20, true)}
						right.children = []uint32{a.number, b.number}
						nodes = append(nodes, a, b)
					}
				}
				for _, n := range nodes {
					if err := m.write(n); err != nil {
						t.Fatal(err)
					}
				}
				var terminal error
				if kind == TableTree {
					c, err := NewCursor(ctx, p, root.number)
					terminal = err
					if err == nil {
						defer c.Close()
						for i := 0; i < 10 && terminal == nil; i++ {
							_, _, terminal = c.Next()
						}
					}
				} else {
					c, err := NewIndexCursor(ctx, p, root.number, info)
					terminal = err
					if err == nil {
						defer c.Close()
						for i := 0; i < 10 && terminal == nil; i++ {
							_, terminal = c.Next()
						}
					}
				}
				if !errors.Is(terminal, pager.ErrCorrupt) {
					t.Fatalf("malformed structure accepted: %v", terminal)
				}
			})
		}
	}
}

func TestReviewCursorAllowsSchemaUnaryRootMemory(t *testing.T) {
	ctx := context.Background()
	p := newMemoryPages(512)
	m, err := newMutation(ctx, p, TableTree, KeyInfo{})
	if err != nil {
		t.Fatal(err)
	}
	child, err := m.allocate(true)
	if err != nil {
		t.Fatal(err)
	}
	want := []any{strings.Repeat("x", 450)}
	payload, err := record.Encode(want)
	if err != nil {
		t.Fatal(err)
	}
	e, err := m.makeEntry(1, payload)
	if err != nil {
		t.Fatal(err)
	}
	child.entries = []mutableEntry{e}
	if err := m.write(child); err != nil {
		t.Fatal(err)
	}
	root := &mutableNode{number: 1, children: []uint32{child.number}}
	if err := m.write(root); err != nil {
		t.Fatal(err)
	}
	c, err := NewCursor(ctx, p, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	id, got, err := c.Next()
	if err != nil || id != 1 || !reflect.DeepEqual(got, want) {
		t.Fatalf("valid unary schema root: %d %v", id, err)
	}
	if _, _, err := c.Next(); err != io.EOF {
		t.Fatal("schema EOF", err)
	}
}
