package btree

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/pager"
	"github.com/ms-xiaosnow/gosqlite/internal/record"
)

// indexTestReader holds entirely in-memory pages; malformed-input tests never
// modify a database or introduce I/O or durability faults.
type indexTestReader struct {
	pages map[uint32][]byte
	next  uint32
	reads []uint32
	size  int
}

func newIndexTestReader() *indexTestReader {
	return &indexTestReader{pages: make(map[uint32][]byte), next: 10, size: 512}
}

func (r *indexTestReader) Header() pager.Header {
	var count uint32
	for n := range r.pages {
		if n > count {
			count = n
		}
	}
	return pager.Header{PageSize: uint32(r.size), UsableSize: uint32(r.size), PageCount: count}
}

func (r *indexTestReader) ReadPage(ctx context.Context, n uint32) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.reads = append(r.reads, n)
	p, ok := r.pages[n]
	if !ok {
		return nil, fmt.Errorf("test page %d missing", n)
	}
	return p, nil
}

func addIndexTestPage(t *testing.T, r *indexTestReader, n uint32, leaf bool, keys [][]any, children []uint32) {
	t.Helper()
	p := make([]byte, r.size)
	r.pages[n] = p
	off := 0
	if n == 1 {
		off = 100
	}
	hdr := 8
	p[off] = 10
	if !leaf {
		hdr = 12
		p[off] = 2
		if len(children) != len(keys)+1 {
			t.Fatal("test interior children")
		}
		binary.BigEndian.PutUint32(p[off+8:], children[len(keys)])
	}
	binary.BigEndian.PutUint16(p[off+3:], uint16(len(keys)))
	at := len(p)
	for i, key := range keys {
		payload, err := record.Encode(key)
		if err != nil {
			t.Fatal(err)
		}
		var cell []byte
		if !leaf {
			cell = binary.BigEndian.AppendUint32(cell, children[i])
		}
		cell = record.AppendVarint(cell, uint64(len(payload)))
		local := len(payload)
		maxLocal, minLocal := (len(p)-12)*64/255-23, (len(p)-12)*32/255-23
		if local > maxLocal {
			local = minLocal + (local-minLocal)%(len(p)-4)
			if local > maxLocal {
				local = minLocal
			}
		}
		cell = append(cell, payload[:local]...)
		if local < len(payload) {
			cell = binary.BigEndian.AppendUint32(cell, r.next)
			for remaining := payload[local:]; len(remaining) > 0; {
				overflow := make([]byte, len(p))
				num := r.next
				r.next++
				size := len(remaining)
				if size > len(p)-4 {
					size = len(p) - 4
				}
				copy(overflow[4:], remaining[:size])
				remaining = remaining[size:]
				if len(remaining) != 0 {
					binary.BigEndian.PutUint32(overflow, r.next)
				}
				r.pages[num] = overflow
			}
		}
		for len(cell) < 4 {
			cell = append(cell, 0)
		}
		at -= len(cell)
		if at < off+hdr+2*len(keys) {
			t.Fatal("test page cells do not fit")
		}
		copy(p[at:], cell)
		binary.BigEndian.PutUint16(p[off+hdr+2*i:], uint16(at))
	}
	binary.BigEndian.PutUint16(p[off+5:], uint16(at))
}

func TestIndexOracleScanAndSeekAllPageSizes(t *testing.T) {
	for _, size := range []int{512, 4096, 65536} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			r := openFixture(t, size)
			root := schemaRoot(t, r, "sample_label")
			var want [][]any
			err := Scan(context.Background(), r, schemaRoot(t, r, "samples"), func(id int64, row []any) error {
				want = append(want, []any{row[1], id})
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			// Independent expected ordering for this known TEXT-affinity fixture.
			sort.Slice(want, func(i, j int) bool {
				a, b := want[i], want[j]
				if a[0] == nil || b[0] == nil {
					if a[0] != b[0] {
						return a[0] == nil
					}
				} else if a[0].(string) != b[0].(string) {
					return a[0].(string) < b[0].(string)
				}
				return a[1].(int64) < b[1].(int64)
			})
			var got [][]any
			if err := ScanIndex(context.Background(), r, root, KeyInfo{Columns: 1}, func(key []any) error {
				got = append(got, key)
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("index scan differs: %d keys, want %d", len(got), len(want))
			}
			// Every record includes both interior-owned keys and the 200 KB
			// overflowing label, with signed rowids at both int64 extremes.
			for i, key := range want {
				found, ok, err := SeekIndex(context.Background(), r, root, KeyInfo{Columns: 1}, key)
				if err != nil || !ok || !reflect.DeepEqual(found, key) {
					t.Fatalf("seek key %d: found=%v, err=%v", i, ok, err)
				}
			}
			for _, key := range [][]any{{nil, int64(0)}, {"row-0001", int64(0)}, {"zzzz", int64(99)}} {
				if found, ok, err := SeekIndex(context.Background(), r, root, KeyInfo{Columns: 1}, key); err != nil || ok || found != nil {
					t.Fatalf("missing key: %v %v %v", found, ok, err)
				}
			}
		})
	}
}

func TestIndexInteriorRecordsAndOverflow(t *testing.T) {
	r := newIndexTestReader()
	middle := strings.Repeat("m", 5000)
	keys := [][]any{{"a", int64(-4)}, {middle, int64(5)}, {"z", int64(9)}}
	addIndexTestPage(t, r, 2, false, keys[1:2], []uint32{3, 4})
	addIndexTestPage(t, r, 3, true, keys[:1], nil)
	addIndexTestPage(t, r, 4, true, keys[2:], nil)
	var got [][]any
	if err := ScanIndex(context.Background(), r, 2, KeyInfo{1}, func(key []any) error {
		got = append(got, key)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, keys) {
		t.Fatalf("got %d records, want three child/key/child records", len(got))
	}
	r.reads = nil
	key, ok, err := SeekIndex(context.Background(), r, 2, KeyInfo{1}, keys[1])
	if err != nil || !ok || !reflect.DeepEqual(key, keys[1]) {
		t.Fatal(ok, err)
	}
	for _, n := range r.reads {
		if n == 3 || n == 4 {
			t.Fatal("exact interior match descended into a child")
		}
	}
}

func TestIndexStorageClassesAndExactNumericComparison(t *testing.T) {
	ordered := []any{nil, math.NaN(), math.Inf(-1), int64(math.MinInt64), float64(math.MinInt64),
		int64(-9007199254740993), float64(-9007199254740992), -1.5, int64(-1), -0.5,
		math.Copysign(0, -1), int64(0), 0.25, int64(1), 1.0, 1.5, float64(9007199254740992),
		int64(9007199254740993), int64(math.MaxInt64), float64(math.MaxInt64), math.Inf(1),
		"", "\x00", "a", "a\x00", "z", "数据库", "\xff", []byte{}, []byte{0}, []byte{1}, []byte{1, 0}, []byte{255}}
	for i, a := range ordered {
		for j, b := range ordered {
			ak, bk := []any{a, int64(7)}, []any{b, int64(7)}
			cmp := compareIndexKeys(ak, bk)
			if cmp != -compareIndexKeys(bk, ak) || (i == j && cmp != 0) || (i < j && cmp > 0) || (i > j && cmp < 0) {
				t.Fatalf("comparison at %d %d: %#v %#v -> %d", i, j, a, b, cmp)
			}
		}
	}
	// Independent exact rational arithmetic checks integer/float boundaries.
	ints := []int64{math.MinInt64, math.MinInt64 + 1, -9007199254740993, -9007199254740992, -1, 0, 1, 9007199254740992, 9007199254740993, math.MaxInt64 - 1, math.MaxInt64}
	reals := []float64{math.Nextafter(float64(math.MinInt64), math.Inf(-1)), float64(math.MinInt64), math.Nextafter(float64(math.MinInt64), 0), -9007199254740992, -1.5, -0.5, 0, 0.5, 1.5, 9007199254740992, math.Nextafter(float64(math.MaxInt64), 0), float64(math.MaxInt64)}
	for _, i := range ints {
		for _, f := range reals {
			want := new(big.Rat).SetInt64(i).Cmp(new(big.Rat).SetFloat64(f))
			if got := compareIndexKeys([]any{i}, []any{f}); got != want {
				t.Fatalf("compare %d and %.20g: got %d want %d", i, f, got, want)
			}
		}
	}
	if compareIndexKeys([]any{int64(1), int64(-1)}, []any{float64(1), int64(0)}) >= 0 {
		t.Fatal("equal numeric fields did not use signed rowid tie-break")
	}
}

func TestIndexCompositeKeysAndOwnedValues(t *testing.T) {
	r := newIndexTestReader()
	keys := [][]any{{nil, []byte{}, int64(-3)}, {int64(1), []byte{1}, int64(2)}, {float64(1), []byte{1}, int64(3)}, {"a", []byte{2}, int64(4)}}
	addIndexTestPage(t, r, 2, true, keys, nil)
	c, err := NewIndexCursor(context.Background(), r, 2, KeyInfo{2})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	for i, want := range keys {
		got, err := c.Next()
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("key %d: %v %v", i, got, err)
		}
		if b := got[1].([]byte); len(b) > 0 {
			b[0] = 255
		}
		got[0], got[2] = "zzz", int64(math.MaxInt64)
	}
	for range 2 {
		if _, err := c.Next(); err != io.EOF {
			t.Fatal("non-sticky EOF", err)
		}
	}
	for _, want := range keys {
		if got, ok, err := SeekIndex(context.Background(), r, 2, KeyInfo{2}, want); err != nil || !ok || !reflect.DeepEqual(got, want) {
			t.Fatal("composite seek", got, ok, err)
		}
	}
	if got, ok, err := SeekIndex(context.Background(), r, 2, KeyInfo{2}, []any{float64(1), []byte{1}, int64(2)}); err != nil || !ok || got[0] != int64(1) {
		t.Fatal("mixed numeric exact seek", got, ok, err)
	}
}

func TestIndexCancellationCloseAndCallback(t *testing.T) {
	r := newIndexTestReader()
	addIndexTestPage(t, r, 2, true, [][]any{{"a", int64(1)}, {"b", int64(2)}}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	c, err := NewIndexCursor(ctx, r, 2, KeyInfo{1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Next(); err != nil {
		t.Fatal(err)
	}
	cancel()
	for range 2 {
		if _, err := c.Next(); !errors.Is(err, context.Canceled) {
			t.Fatal("expected sticky context error", err)
		}
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Next(); err != io.EOF {
		t.Fatal("closed cursor", err)
	}
	if _, err := NewIndexCursor(ctx, r, 2, KeyInfo{1}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if _, _, err := SeekIndex(ctx, r, 2, KeyInfo{1}, []any{"a", int64(1)}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	stop := errors.New("stop scan")
	calls := 0
	if err := ScanIndex(context.Background(), r, 2, KeyInfo{1}, func([]any) error { calls++; return stop }); err != stop || calls != 1 {
		t.Fatal("early stop", calls, err)
	}
}

func TestIndexKeyValidation(t *testing.T) {
	for _, tc := range []struct {
		info KeyInfo
		key  []any
	}{{KeyInfo{0}, []any{int64(1)}}, {KeyInfo{-1}, nil}, {KeyInfo{record.MaxColumns}, nil},
		{KeyInfo{1}, []any{"a"}}, {KeyInfo{1}, []any{"a", int(1)}},
		{KeyInfo{1}, []any{true, int64(1)}}, {KeyInfo{1}, []any{"a", float64(1)}}} {
		if err := validateIndexKey(tc.info, tc.key); err == nil {
			t.Fatalf("invalid index key accepted: %#v %#v", tc.info, tc.key)
		}
	}
	if err := validateIndexKey(KeyInfo{1}, []any{math.NaN(), int64(1)}); err != nil {
		t.Fatal(err)
	}
}

func TestIndexMalformedPageParserInMemory(t *testing.T) {
	r := newIndexTestReader()
	addIndexTestPage(t, r, 2, true, [][]any{{"abc", int64(1)}, {"def", int64(2)}}, nil)
	base := r.pages[2]
	cases := []struct {
		name   string
		change func([]byte)
	}{
		{"type", func(p []byte) { p[0] = 0 }},
		{"array", func(p []byte) { p[3], p[4] = 255, 255 }},
		{"content", func(p []byte) { p[5], p[6] = 0, 1 }},
		{"fragments", func(p []byte) { p[7] = 61 }},
		{"cell_offset", func(p []byte) { binary.BigEndian.PutUint16(p[8:], 512) }},
		{"cell_overlap", func(p []byte) { copy(p[8:10], p[10:12]) }},
		{"freeblock_offset", func(p []byte) { binary.BigEndian.PutUint16(p[1:], 511) }},
		{"freeblock_overlap", func(p []byte) {
			start := binary.BigEndian.Uint16(p[10:12])
			binary.BigEndian.PutUint16(p[1:], start)
			binary.BigEndian.PutUint16(p[start:], 0)
			binary.BigEndian.PutUint16(p[start+2:], 4)
		}},
		{"truncated_cell", func(p []byte) {
			binary.BigEndian.PutUint16(p[8:], 510)
			p[510], p[511] = 3, 3
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := append([]byte(nil), base...)
			tc.change(p)
			if _, err := parseIndexPage(p, 2, len(p)); err == nil {
				t.Fatal("malformed in-memory page accepted")
			}
		})
	}
	for _, usable := range []int{-1, 0, 479, 513, 65537} {
		if _, err := parseIndexPage(base, 2, usable); err == nil {
			t.Fatal("invalid usable size accepted", usable)
		}
	}
	if _, err := parseIndexPage(nil, 1, 512); err == nil {
		t.Fatal("truncated page accepted")
	}
	addIndexTestPage(t, r, 3, false, [][]any{{"b", int64(2)}}, []uint32{4, 5})
	for _, at := range []int{8, int(binary.BigEndian.Uint16(r.pages[3][12:14]))} {
		p := append([]byte(nil), r.pages[3]...)
		binary.BigEndian.PutUint32(p[at:], 0)
		if _, err := parseIndexPage(p, 3, len(p)); !errors.Is(err, pager.ErrCorrupt) {
			t.Fatal("zero child accepted", err)
		}
	}
}

func TestIndexEmptyAndPageOneLayouts(t *testing.T) {
	for _, size := range []int{512, 65536} {
		for _, n := range []uint32{1, 2} {
			r := newIndexTestReader()
			r.size = size
			addIndexTestPage(t, r, n, true, nil, nil)
			if err := ScanIndex(context.Background(), r, n, KeyInfo{1}, func([]any) error {
				t.Fatal("empty index returned a key")
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if _, ok, err := SeekIndex(context.Background(), r, n, KeyInfo{1}, []any{nil, int64(1)}); err != nil || ok {
				t.Fatal(ok, err)
			}
			addIndexTestPage(t, r, n, true, [][]any{{"x", int64(1)}}, nil)
			if _, ok, err := SeekIndex(context.Background(), r, n, KeyInfo{1}, []any{"x", int64(1)}); err != nil || !ok {
				t.Fatal(ok, err)
			}
		}
	}
}

func TestIndexInMemoryTraversalBounds(t *testing.T) {
	t.Run("depth", func(t *testing.T) {
		r := newIndexTestReader()
		for n := uint32(1); n <= MaxDepth+1; n++ {
			// Nonempty interior nodes ensure this isolates the depth limit, rather
			// than triggering the earlier empty-nonroot structural validation.
			addIndexTestPage(t, r, n, false, [][]any{{int64(MaxDepth + 2 - n), int64(1)}}, []uint32{n + 1, n + 1})
		}
		if err := ScanIndex(context.Background(), r, 1, KeyInfo{1}, func([]any) error { return nil }); !errors.Is(err, record.ErrLimit) {
			t.Fatal("depth limit", err)
		}
		if _, _, err := SeekIndex(context.Background(), r, 1, KeyInfo{1}, []any{nil, int64(1)}); !errors.Is(err, record.ErrLimit) {
			t.Fatal("seek depth limit", err)
		}
	})
	t.Run("cycle", func(t *testing.T) {
		r := newIndexTestReader()
		addIndexTestPage(t, r, 2, false, nil, []uint32{2})
		if err := ScanIndex(context.Background(), r, 2, KeyInfo{1}, func([]any) error { return nil }); !errors.Is(err, pager.ErrCorrupt) {
			t.Fatal("cycle", err)
		}
	})
	t.Run("outside_database", func(t *testing.T) {
		r := newIndexTestReader()
		addIndexTestPage(t, r, 2, false, nil, []uint32{3})
		if err := ScanIndex(context.Background(), r, 2, KeyInfo{1}, func([]any) error { return nil }); !errors.Is(err, pager.ErrCorrupt) {
			t.Fatal("page count bound", err)
		}
	})
	t.Run("key_order", func(t *testing.T) {
		r := newIndexTestReader()
		addIndexTestPage(t, r, 2, true, [][]any{{"z", int64(1)}, {"a", int64(2)}}, nil)
		if err := ScanIndex(context.Background(), r, 2, KeyInfo{1}, func([]any) error { return nil }); !errors.Is(err, pager.ErrCorrupt) {
			t.Fatal("key order", err)
		}
	})
}
