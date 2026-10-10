package btree

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/pager"
	"github.com/ms-xiaosnow/gosqlite/internal/record"
)

func collectIndexRange(t *testing.T, c *IndexRangeCursor) [][]any {
	t.Helper()
	defer c.Close()
	var keys [][]any
	for {
		key, err := c.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		keys = append(keys, key)
	}
	if key, err := c.Next(); key != nil || err != io.EOF {
		t.Fatal("non-sticky range EOF", key, err)
	}
	if c.cursor.stack != nil || c.cursor.visited != nil || c.cursor.overflow != nil || c.cursor.last != nil || c.prefix != nil {
		t.Fatal("range retains traversal state after EOF")
	}
	return keys
}

func readIndexRange(t *testing.T, r PageReader, root uint32, info KeyInfo, lower, prefix []any) [][]any {
	t.Helper()
	c, err := NewIndexRangeCursor(context.Background(), r, root, info, lower, prefix)
	if err != nil {
		t.Fatal(err)
	}
	return collectIndexRange(t, c)
}

// makeIndexRangeTree creates a balanced in-memory tree. Interior records are
// removed from both children, matching SQLite index (not table) B-trees.
func makeIndexRangeTree(t *testing.T, keys [][]any) (*indexTestReader, uint32) {
	t.Helper()
	r := newIndexTestReader()
	r.next = 100000 // Overflow allocation cannot collide with tree pages.
	next := uint32(2)
	var add func([][]any) uint32
	add = func(part [][]any) uint32 {
		n := next
		next++
		if len(part) <= 7 {
			addIndexTestPage(t, r, n, true, part, nil)
			return n
		}
		mid := len(part) / 2
		left, right := add(part[:mid]), add(part[mid+1:])
		addIndexTestPage(t, r, n, false, part[mid:mid+1], []uint32{left, right})
		return n
	}
	return r, add(keys)
}

func TestIndexRangeBoundariesAndStorageClasses(t *testing.T) {
	keys := [][]any{
		{nil, int64(math.MinInt64)}, {nil, int64(-1)}, {nil, int64(0)},
		{math.Inf(-1), int64(1)}, {int64(math.MinInt64), int64(2)}, {-1.5, int64(3)},
		{int64(0), int64(-10)}, {float64(0), int64(-3)},
		{int64(1), int64(-4)}, {float64(1), int64(8)},
		{float64(9007199254740992), int64(9)}, {int64(9007199254740993), int64(10)},
		{int64(math.MaxInt64), int64(11)}, {math.Inf(1), int64(12)},
		{"", int64(13)}, {"a", int64(math.MinInt64)}, {"a", int64(-1)},
		{"a", int64(0)}, {"a", int64(math.MaxInt64)}, {"a\x00", int64(15)},
		{"b", int64(16)}, {[]byte{}, int64(17)}, {[]byte{0}, int64(18)}, {[]byte{255}, int64(19)},
	}
	r, root := makeIndexRangeTree(t, keys)
	for _, tc := range []struct {
		name          string
		lower, prefix []any
		first, end    int
	}{
		{"unbounded", nil, nil, 0, len(keys)},
		{"null_prefix", nil, []any{nil}, 0, 3},
		{"nan_as_null", nil, []any{math.NaN()}, 0, 3},
		{"numeric_equality", nil, []any{float64(1)}, 8, 10},
		{"exact_integer_precision", []any{int64(9007199254740993)}, nil, 11, len(keys)},
		{"int64_end", []any{int64(math.MaxInt64)}, nil, 12, len(keys)},
		{"inclusive_partial", []any{"a"}, nil, 15, len(keys)},
		{"inclusive_rowid", []any{"a", int64(-1)}, []any{"a"}, 16, 19},
		{"minimum_rowid", []any{"a", int64(math.MinInt64)}, []any{"a"}, 15, 19},
		{"maximum_rowid", []any{"a", int64(math.MaxInt64)}, []any{"a"}, 18, 19},
		{"absent_rowid", []any{"a", int64(2)}, []any{"a"}, 18, 19},
		{"complete_prefix", nil, []any{"a", int64(0)}, 17, 18},
		{"later_prefix", []any{"a"}, []any{"b"}, 20, 21},
		{"lower_after_prefix", []any{"b"}, []any{"a"}, 0, 0},
		{"longer_prefix", []any{"a"}, []any{"a", int64(-1)}, 16, 17},
		{"lower_after_complete_prefix", []any{"a", int64(0)}, []any{"a", int64(-1)}, 0, 0},
		{"field_not_string_prefix", nil, []any{"a"}, 15, 19},
		{"empty_blob", nil, []any{[]byte{}}, 21, 22},
		{"binary_blob", nil, []any{[]byte{0}}, 22, 23},
		{"absent_prefix", nil, []any{"missing"}, 0, 0},
		{"above_maximum", []any{[]byte{255, 0}}, nil, 0, 0},
		{"absent_between", []any{"a\x00\x00"}, nil, 20, len(keys)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := readIndexRange(t, r, root, KeyInfo{1}, tc.lower, tc.prefix)
			want := keys[tc.first:tc.end]
			if len(got) != len(want) || len(got) > 0 && !reflect.DeepEqual(got, want) {
				t.Fatalf("got %#v, want %#v", got, want)
			}
		})
	}
}

func TestIndexRangeCompositePrefixesAndOwnedValues(t *testing.T) {
	keys := [][]any{
		{nil, nil, int64(-9)},
		{int64(1), nil, int64(-3)},
		{int64(1), int64(-1), int64(4)},
		{float64(1), float64(0), int64(-5)},
		{int64(1), "", int64(-7)},
		{int64(1), []byte{}, int64(-8)},
		{int64(1), []byte{1}, int64(-2)},
		{float64(1), []byte{1}, int64(0)},
		{int64(1), []byte{1}, int64(math.MaxInt64)},
		{int64(1), []byte{1, 0}, int64(-10)},
		{int64(2), []byte{1}, int64(1)},
	}
	r, root := makeIndexRangeTree(t, keys)
	for _, tc := range []struct {
		lower, prefix []any
		first, end    int
	}{
		{nil, []any{float64(1)}, 1, 10},
		{nil, []any{int64(1), []byte{1}}, 6, 9},
		{[]any{float64(1), []byte{1}, int64(-1)}, []any{int64(1)}, 7, 10},
		{[]any{float64(1), []byte{1}, int64(0)}, []any{int64(1), []byte{1}}, 7, 9},
		{nil, []any{float64(1), float64(-1)}, 2, 3},
		{nil, []any{int64(1), nil}, 1, 2},
		{nil, []any{int64(1), []byte{1}, int64(math.MaxInt64)}, 8, 9},
	} {
		got := readIndexRange(t, r, root, KeyInfo{2}, tc.lower, tc.prefix)
		if !reflect.DeepEqual(got, keys[tc.first:tc.end]) {
			t.Fatalf("composite range %#v %#v: %#v", tc.lower, tc.prefix, got)
		}
	}
	lower, prefix := []any{int64(1), []byte{1}}, []any{float64(1), []byte{1}}
	c, err := NewIndexRangeCursor(context.Background(), r, root, KeyInfo{2}, lower, prefix)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	lower[0], prefix[0] = "changed", "changed"
	lower[1].([]byte)[0], prefix[1].([]byte)[0] = 255, 255
	for _, want := range keys[6:9] {
		got, err := c.Next()
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatal("mutating input/output changed cursor", got, err)
		}
		got[1].([]byte)[0], got[2] = 255, int64(math.MinInt64)
	}
	if _, err := c.Next(); err != io.EOF {
		t.Fatal(err)
	}
}

func TestIndexRangeDuplicatePrefixesAcrossPages(t *testing.T) {
	var keys [][]any
	for i := 0; i < 1023; i++ {
		keys = append(keys, []any{int64(i / 17), int64(i - 511)})
	}
	r, root := makeIndexRangeTree(t, keys)
	for group := 0; group <= 60; group++ {
		r.reads = nil
		got := readIndexRange(t, r, root, KeyInfo{1}, nil, []any{int64(group)})
		end := (group + 1) * 17
		if end > len(keys) {
			end = len(keys)
		}
		if !reflect.DeepEqual(got, keys[group*17:end]) {
			t.Fatalf("group %d got %d records, want %d", group, len(got), end-group*17)
		}
		if len(r.reads) > 20 {
			t.Fatalf("prefix %d read %d of %d tree pages", group, len(r.reads), len(r.pages))
		}
	}
	if got := readIndexRange(t, r, root, KeyInfo{1}, []any{int64(30), int64(0)}, []any{int64(30)}); !reflect.DeepEqual(got, keys[511:527]) {
		t.Fatal("rowid-inclusive duplicate range", got)
	}
}

func TestIndexRangeDoesNotReadUnrelatedSubtrees(t *testing.T) {
	r := newIndexTestReader()
	addIndexTestPage(t, r, 2, false, [][]any{{"m", int64(2)}}, []uint32{3, 4})
	addIndexTestPage(t, r, 3, false, [][]any{{"c", int64(3)}}, []uint32{7, 8})
	addIndexTestPage(t, r, 4, false, [][]any{{"t", int64(4)}}, []uint32{5, 6})
	addIndexTestPage(t, r, 5, true, [][]any{{"q", int64(5)}}, nil)
	addIndexTestPage(t, r, 6, true, [][]any{{"z", int64(6)}}, nil)
	addIndexTestPage(t, r, 7, true, [][]any{{"a", int64(7)}}, nil)
	addIndexTestPage(t, r, 8, true, [][]any{{"f", int64(8)}}, nil)
	for _, tc := range []struct {
		prefix []any
		reads  []uint32
		want   [][]any
	}{
		{[]any{"z"}, []uint32{2, 4, 6}, [][]any{{"z", int64(6)}}},
		{[]any{"q"}, []uint32{2, 4, 5}, [][]any{{"q", int64(5)}}},
		{[]any{"t", int64(4)}, []uint32{2, 4}, [][]any{{"t", int64(4)}}},
		{[]any{"x"}, []uint32{2, 4, 6}, nil},
	} {
		r.reads = nil
		got := readIndexRange(t, r, 2, KeyInfo{1}, nil, tc.prefix)
		if !reflect.DeepEqual(got, tc.want) || !reflect.DeepEqual(r.reads, tc.reads) {
			t.Fatalf("prefix %#v: keys %#v, pages %v; want %#v, %v", tc.prefix, got, r.reads, tc.want, tc.reads)
		}
	}
}

func TestIndexRangeInteriorOverflowProbeReread(t *testing.T) {
	r := newIndexTestReader()
	label := strings.Repeat("long-index-key", 400)
	keys := [][]any{{label, int64(-1)}, {label, int64(0)}, {label, int64(1)}}
	addIndexTestPage(t, r, 2, false, keys[1:2], []uint32{3, 4})
	addIndexTestPage(t, r, 3, true, keys[:1], nil)
	addIndexTestPage(t, r, 4, true, keys[2:], nil)
	if got := readIndexRange(t, r, 2, KeyInfo{1}, nil, []any{label}); !reflect.DeepEqual(got, keys) {
		t.Fatal("overflow prefix across interior record", len(got))
	}
	r.reads = nil
	if got := readIndexRange(t, r, 2, KeyInfo{1}, keys[1], keys[1]); !reflect.DeepEqual(got, keys[1:2]) {
		t.Fatal("exact overflowing interior range", len(got))
	}
	counts := make(map[uint32]int)
	for _, page := range r.reads {
		counts[page]++
		if page == 3 || page == 4 {
			t.Fatal("exact interior range read a child")
		}
	}
	if counts[2] != 1 || counts[10] != 2 {
		t.Fatal("test did not exercise same-cell overflow probe reread", counts)
	}
	// Lower-bound-only resumes at the interior hit and then its right child.
	if got := readIndexRange(t, r, 2, KeyInfo{1}, keys[1], nil); !reflect.DeepEqual(got, keys[1:]) {
		t.Fatal("interior lower bound continuation", len(got))
	}
}

func TestIndexRangeOracleFixturesAllPageSizes(t *testing.T) {
	for _, size := range []int{512, 4096, 65536} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			r := openFixture(t, size)
			root := schemaRoot(t, r, "sample_label")
			var keys [][]any
			if err := ScanIndex(context.Background(), r, root, KeyInfo{1}, func(key []any) error {
				keys = append(keys, key)
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			for _, i := range []int{0, 1, len(keys) / 4, len(keys) / 2, len(keys) - 2, len(keys) - 1} {
				if got := readIndexRange(t, r, root, KeyInfo{1}, keys[i], nil); !reflect.DeepEqual(got, keys[i:]) {
					t.Fatalf("official fixture lower bound %d: %d keys, want %d", i, len(got), len(keys)-i)
				}
			}
			// Include every full rowid prefix, all interior records, both signed
			// rowid extremes, and the official fixture's 200 KB overflow key.
			for i, key := range keys {
				if got := readIndexRange(t, r, root, KeyInfo{1}, nil, key); !reflect.DeepEqual(got, keys[i:i+1]) {
					t.Fatalf("official fixture full prefix %d", i)
				}
			}
		})
	}
}

func TestIndexRangeEmptyCancellationAndClose(t *testing.T) {
	r := newIndexTestReader()
	addIndexTestPage(t, r, 2, true, nil, nil)
	for _, lower := range [][]any{nil, {"missing"}} {
		if got := readIndexRange(t, r, 2, KeyInfo{1}, lower, nil); len(got) != 0 {
			t.Fatal("empty range returned a key")
		}
	}
	addIndexTestPage(t, r, 2, true, [][]any{{"a", int64(1)}, {"b", int64(2)}}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	c, err := NewIndexRangeCursor(ctx, r, 2, KeyInfo{1}, []any{"a"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	for range 2 {
		if _, err := c.Next(); !errors.Is(err, context.Canceled) {
			t.Fatal("expected sticky cancellation", err)
		}
	}
	if c.cursor.stack != nil || c.cursor.visited != nil || c.cursor.overflow != nil || c.prefix != nil {
		t.Fatal("canceled range retained state")
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Next(); err != io.EOF {
		t.Fatal("closed range", err)
	}
	if _, err := NewIndexRangeCursor(ctx, r, 2, KeyInfo{1}, nil, nil); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled constructor", err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	tracked := indexRangeCancelReader{PageReader: r, cancel: cancel}
	if _, err := NewIndexRangeCursor(ctx, tracked, 2, KeyInfo{1}, []any{"a"}, nil); !errors.Is(err, context.Canceled) {
		t.Fatal("cancellation inside page read", err)
	}
}

type indexRangeCancelReader struct {
	PageReader
	cancel context.CancelFunc
}

func (r indexRangeCancelReader) ReadPage(ctx context.Context, page uint32) ([]byte, error) {
	p, err := r.PageReader.ReadPage(ctx, page)
	r.cancel()
	return p, err
}

func TestIndexRangeCancellationDuringOverflowReread(t *testing.T) {
	r := newIndexTestReader()
	key := []any{strings.Repeat("x", 5000), int64(1)}
	addIndexTestPage(t, r, 2, true, [][]any{key}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, err := NewIndexRangeCursor(ctx, r, 2, KeyInfo{1}, key, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	// Constructor probes already claimed the overflow chain. Cancel during its
	// reread on Next, exercising the ownership-aware branch rather than initial
	// page validation. Both the cancellation and state cleanup must be sticky.
	c.cursor.reader = indexRangeCancelReader{PageReader: r, cancel: cancel}
	for range 2 {
		if _, err := c.Next(); !errors.Is(err, context.Canceled) {
			t.Fatal("overflow reread ignored cancellation", err)
		}
	}
	if c.cursor.stack != nil || c.cursor.visited != nil || c.cursor.overflow != nil || c.cursor.last != nil || c.prefix != nil {
		t.Fatal("canceled overflow reread retained state")
	}
}

func TestIndexRangeValidationAndLimitsInMemory(t *testing.T) {
	r := newIndexTestReader()
	addIndexTestPage(t, r, 2, true, nil, nil)
	for _, key := range [][]any{{true}, {int(1)}, {"a", nil}, {"a", float64(1)}, {"a", int64(1), int64(2)}} {
		for _, isPrefix := range []bool{false, true} {
			var lower, prefix []any
			if isPrefix {
				prefix = key
			} else {
				lower = key
			}
			r.reads = nil
			if _, err := NewIndexRangeCursor(context.Background(), r, 2, KeyInfo{1}, lower, prefix); !errors.Is(err, record.ErrUnsupported) {
				t.Fatalf("invalid bound %#v: %v", key, err)
			}
			if len(r.reads) != 0 {
				t.Fatal("invalid bound performed page I/O")
			}
		}
	}
	for _, columns := range []int{0, -1, record.MaxColumns} {
		if _, err := NewIndexRangeCursor(context.Background(), r, 2, KeyInfo{columns}, nil, nil); err == nil {
			t.Fatal("invalid key metadata accepted", columns)
		}
	}
	oversize := strings.Repeat("x", record.MaxPayload+1)
	if _, err := NewIndexRangeCursor(context.Background(), r, 2, KeyInfo{1}, []any{oversize}, nil); !errors.Is(err, record.ErrLimit) {
		t.Fatal("bound payload limit", err)
	}
	t.Run("depth", func(t *testing.T) {
		r := newIndexTestReader()
		for n := uint32(2); n <= MaxDepth+2; n++ {
			addIndexTestPage(t, r, n, false, [][]any{{int64(MaxDepth + 3 - n), int64(1)}}, []uint32{n + 1, n + 1})
		}
		if _, err := NewIndexRangeCursor(context.Background(), r, 2, KeyInfo{1}, []any{int64(-1)}, nil); !errors.Is(err, record.ErrLimit) {
			t.Fatal("range depth budget", err)
		}
	})
	t.Run("visited", func(t *testing.T) {
		r := newIndexTestReader()
		addIndexTestPage(t, r, 2, false, [][]any{{"m", int64(2)}}, []uint32{3, 4})
		addIndexTestPage(t, r, 3, true, [][]any{{"a", int64(3)}}, nil)
		addIndexTestPage(t, r, 4, true, [][]any{{"z", int64(4)}}, nil)
		c, err := NewIndexRangeCursor(context.Background(), r, 2, KeyInfo{1}, nil, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer c.Close()
		for n := uint32(10); len(c.cursor.visited) < MaxVisitedPages; n++ {
			c.cursor.visited[n] = struct{}{}
		}
		for range 2 {
			if _, err := c.Next(); !errors.Is(err, record.ErrLimit) {
				t.Fatal("visited-page budget", err)
			}
		}
	})
}

func TestIndexRangeMalformedOwnershipInMemory(t *testing.T) {
	for _, mode := range []string{"shared_chain", "overflow_cycle", "overflow_as_child", "repeated_child", "parent_bounds"} {
		t.Run(mode, func(t *testing.T) {
			r := newIndexTestReader()
			var lower []any
			switch mode {
			case "shared_chain", "overflow_cycle", "overflow_as_child":
				label := strings.Repeat("x", 5000)
				addIndexTestPage(t, r, 2, false, [][]any{{label, int64(0)}}, []uint32{3, 4})
				addIndexTestPage(t, r, 3, true, [][]any{{label, int64(-1)}}, nil)
				addIndexTestPage(t, r, 4, true, [][]any{{label, int64(1)}}, nil)
				root, err := parseIndexPage(r.pages[2], 2, r.size)
				if err != nil {
					t.Fatal(err)
				}
				item := root.cells[0]
				overflow := binary.BigEndian.Uint32(root.data[item.offset+item.local:])
				switch mode {
				case "shared_chain":
					leaf, err := parseIndexPage(r.pages[3], 3, r.size)
					if err != nil {
						t.Fatal(err)
					}
					at := leaf.cells[0].offset + leaf.cells[0].local
					binary.BigEndian.PutUint32(r.pages[3][at:], overflow)
				case "overflow_cycle":
					binary.BigEndian.PutUint32(r.pages[overflow], overflow)
				case "overflow_as_child":
					binary.BigEndian.PutUint32(r.pages[2][8:], overflow)
				}
				lower = []any{label}
			case "repeated_child":
				addIndexTestPage(t, r, 2, false, [][]any{{"m", int64(2)}}, []uint32{3, 3})
				addIndexTestPage(t, r, 3, true, [][]any{{"a", int64(3)}}, nil)
				lower = []any{"a"}
			case "parent_bounds":
				addIndexTestPage(t, r, 2, false, [][]any{{"m", int64(2)}}, []uint32{3, 4})
				addIndexTestPage(t, r, 3, true, [][]any{{"z", int64(3)}}, nil)
				addIndexTestPage(t, r, 4, true, [][]any{{"z", int64(4)}}, nil)
				lower = []any{"a"}
			}
			c, err := NewIndexRangeCursor(context.Background(), r, 2, KeyInfo{1}, lower, nil)
			if err == nil {
				defer c.Close()
				for err == nil {
					_, err = c.Next()
				}
				if _, again := c.Next(); again != err {
					t.Fatal("non-sticky ownership error", err, again)
				}
			}
			if !errors.Is(err, pager.ErrCorrupt) {
				t.Fatal("malformed in-memory ownership accepted", err)
			}
		})
	}
}
