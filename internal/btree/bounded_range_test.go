package btree

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/record"
)

// All synthetic page structures in this file are descriptor-free. Existing
// oracle fixtures are opened intact and read only; no fault injection occurs.
func collectBoundedIndex(t *testing.T, c *BoundedIndexCursor) [][]any {
	t.Helper()
	defer c.Close()
	var got [][]any
	for {
		key, err := c.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, key)
	}
	for range 2 {
		if _, err := c.Next(); err != io.EOF {
			t.Fatal("sticky EOF", err)
		}
	}
	if c.stack != nil || c.lower != nil || c.upper != nil || c.base != nil && (c.base.visited != nil || c.base.overflow != nil || c.base.last != nil) {
		t.Fatal("terminal retained state")
	}
	return got
}
func boundedWant(keys [][]any, lo, hi *IndexBound, rev bool) [][]any {
	var out [][]any
	for _, k := range keys {
		if lo != nil {
			cmp := compareIndexPrefix(k, lo.Key)
			if cmp < 0 || cmp == 0 && !lo.Inclusive {
				continue
			}
		}
		if hi != nil {
			cmp := compareIndexPrefix(k, hi.Key)
			if cmp > 0 || cmp == 0 && !hi.Inclusive {
				continue
			}
		}
		out = append(out, k)
	}
	if rev {
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
	}
	return out
}
func TestBoundedIndexCompositeProperty(t *testing.T) {
	values := []any{nil, math.Inf(-1), int64(math.MinInt64), int64(-9007199254740993), float64(-9007199254740992), -0.5, int64(0), float64(0), int64(1), float64(9007199254740992), int64(9007199254740993), int64(math.MaxInt64), math.Inf(1), "", "a", "a\x00", "数据库", []byte{}, []byte{0}, []byte{255}}
	var keys [][]any
	// Equal 0 and 0.0 retain unique rowid order across every suffix.
	for i, a := range values {
		for j, b := range []any{nil, int64(-1), "", []byte{0}} {
			keys = append(keys, []any{a, b, int64(i*4 + j - 40)})
		}
	}
	// Use the production comparator only for the property reference; the CLI
	// suite independently exercises comparator and endpoint SQL semantics.
	sortKeys(keys)
	r, root := makeIndexRangeTree(t, keys)
	bounds := []*IndexBound{nil}
	for _, k := range keys {
		for n := 1; n <= len(k); n++ {
			for _, inc := range []bool{false, true} {
				bounds = append(bounds, &IndexBound{k[:n], inc})
			}
		}
	}
	bounds = append(bounds, &IndexBound{[]any{math.NaN()}, true}, &IndexBound{[]any{"a\x00x"}, false})
	random := rand.New(rand.NewSource(20261010))
	for n := 0; n < 1500; n++ {
		lo, hi := bounds[random.Intn(len(bounds))], bounds[random.Intn(len(bounds))]
		for _, rev := range []bool{false, true} {
			c, err := NewBoundedIndexCursor(context.Background(), r, root, KeyInfo{2}, lo, hi, rev, RangeLimits{})
			if err != nil {
				t.Fatal(err)
			}
			got, want := collectBoundedIndex(t, c), boundedWant(keys, lo, hi, rev)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("case %d reverse %v lower %#v upper %#v got %#v want %#v", n, rev, lo, hi, got, want)
			}
		}
	}
}
func sortKeys(keys [][]any) {
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && compareIndexKeys(keys[j-1], keys[j]) > 0; j-- {
			keys[j-1], keys[j] = keys[j], keys[j-1]
		}
	}
}
func TestBoundedIndexExactSelectiveTraversal(t *testing.T) {
	r := newIndexTestReader()
	addIndexTestPage(t, r, 2, false, [][]any{{"m", int64(0)}}, []uint32{3, 4})
	addIndexTestPage(t, r, 3, false, [][]any{{"c", int64(-3)}}, []uint32{7, 8})
	addIndexTestPage(t, r, 4, false, [][]any{{"t", int64(3)}}, []uint32{5, 6})
	addIndexTestPage(t, r, 5, true, [][]any{{"q", int64(1)}}, nil)
	addIndexTestPage(t, r, 6, true, [][]any{{"z", int64(5)}}, nil)
	addIndexTestPage(t, r, 7, true, [][]any{{"a", int64(-5)}}, nil)
	addIndexTestPage(t, r, 8, true, [][]any{{"f", int64(-1)}}, nil)
	for _, tc := range []struct {
		lo, hi *IndexBound
		rev    bool
		reads  []uint32
		want   [][]any
	}{
		{&IndexBound{[]any{"t", int64(3)}, true}, &IndexBound{[]any{"t", int64(3)}, true}, false, []uint32{2, 4}, [][]any{{"t", int64(3)}}},
		{&IndexBound{[]any{"t", int64(3)}, true}, &IndexBound{[]any{"t", int64(3)}, true}, true, []uint32{2, 4}, [][]any{{"t", int64(3)}}},
		{&IndexBound{[]any{"q"}, true}, &IndexBound{[]any{"t", int64(3)}, true}, false, []uint32{2, 4, 5}, [][]any{{"q", int64(1)}, {"t", int64(3)}}},
		{&IndexBound{[]any{"t", int64(3)}, true}, &IndexBound{[]any{"z"}, true}, true, []uint32{2, 4, 6}, [][]any{{"z", int64(5)}, {"t", int64(3)}}},
		{&IndexBound{[]any{"z"}, true}, nil, false, []uint32{2, 4, 6}, [][]any{{"z", int64(5)}}},
		{nil, &IndexBound{[]any{"a"}, true}, true, []uint32{2, 3, 7}, [][]any{{"a", int64(-5)}}},
	} {
		r.reads = nil
		c, err := NewBoundedIndexCursor(context.Background(), r, 2, KeyInfo{1}, tc.lo, tc.hi, tc.rev, RangeLimits{})
		if err != nil {
			t.Fatal(err)
		}
		got := collectBoundedIndex(t, c)
		if !reflect.DeepEqual(got, tc.want) || !reflect.DeepEqual(r.reads, tc.reads) {
			t.Fatalf("reverse=%v keys=%v reads=%v want %v %v", tc.rev, got, r.reads, tc.want, tc.reads)
		}
		if c.Stats().PageReads != int64(len(r.reads)) {
			t.Fatal(c.Stats(), r.reads)
		}
	}
}
func TestBoundedIndexOverflowDuplicatesOwnership(t *testing.T) {
	r := newIndexTestReader()
	label := strings.Repeat("a\x00long", 700)
	keys := [][]any{{label, int64(math.MinInt64)}, {label, int64(0)}, {label, int64(math.MaxInt64)}}
	addIndexTestPage(t, r, 2, false, keys[1:2], []uint32{3, 4})
	addIndexTestPage(t, r, 3, true, keys[:1], nil)
	addIndexTestPage(t, r, 4, true, keys[2:], nil)
	for _, rev := range []bool{false, true} {
		for _, inc := range []bool{false, true} {
			lo, hi := &IndexBound{[]any{label}, inc}, &IndexBound{[]any{label}, true}
			c, err := NewBoundedIndexCursor(context.Background(), r, 2, KeyInfo{1}, lo, hi, rev, RangeLimits{})
			if err != nil {
				t.Fatal(err)
			}
			if got, want := collectBoundedIndex(t, c), boundedWant(keys, lo, hi, rev); !reflect.DeepEqual(got, want) {
				t.Fatal(got, want)
			}
		}
	}
	blob := []byte{0}
	r = newIndexTestReader()
	addIndexTestPage(t, r, 2, true, [][]any{{blob, int64(-1)}, {blob, int64(0)}, {blob, int64(1)}}, nil)
	lo, hi := &IndexBound{[]any{blob}, true}, &IndexBound{[]any{[]byte{0}}, true}
	c, err := NewBoundedIndexCursor(context.Background(), r, 2, KeyInfo{1}, lo, hi, true, RangeLimits{})
	if err != nil {
		t.Fatal(err)
	}
	blob[0] = 255
	hi.Key[0].([]byte)[0] = 255
	for _, id := range []int64{1, 0, -1} {
		key, err := c.Next()
		if err != nil || key[1] != id {
			t.Fatal(key, err)
		}
		key[0].([]byte)[0] = 255
		key[1] = int64(99)
	}
	if _, err = c.Next(); err != io.EOF {
		t.Fatal(err)
	}
}
func TestBoundedRowIDOracleFixtureProperty(t *testing.T) {
	for _, size := range []int{512, 4096, 65536} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			r := openFixture(t, size)
			root := schemaRoot(t, r, "samples")
			var ids []int64
			var rows [][]any
			if err := Scan(context.Background(), r, root, func(id int64, row []any) error { ids = append(ids, id); rows = append(rows, row); return nil }); err != nil {
				t.Fatal(err)
			}
			bounds := []*RowIDBound{nil}
			for _, v := range []int64{math.MinInt64, -1000, -1, 0, 1, 500, 1024, 1025, math.MaxInt64} {
				bounds = append(bounds, &RowIDBound{v, false}, &RowIDBound{v, true})
			}
			for _, lo := range bounds {
				for _, hi := range bounds {
					for _, rev := range []bool{false, true} {
						c, err := NewRowIDRangeCursor(context.Background(), r, root, lo, hi, rev, RangeLimits{})
						if err != nil {
							t.Fatal(err)
						}
						var want []int
						for i, id := range ids {
							if lo != nil && (id < lo.Value || id == lo.Value && !lo.Inclusive) || hi != nil && (id > hi.Value || id == hi.Value && !hi.Inclusive) {
								continue
							}
							want = append(want, i)
						}
						if rev {
							for i, j := 0, len(want)-1; i < j; i, j = i+1, j-1 {
								want[i], want[j] = want[j], want[i]
							}
						}
						for _, i := range want {
							id, row, err := c.Next()
							if err != nil || id != ids[i] || !reflect.DeepEqual(row, rows[i]) {
								t.Fatalf("reverse=%v lo=%v hi=%v id=%d want=%d err=%v", rev, lo, hi, id, ids[i], err)
							}
						}
						for range 2 {
							if _, _, err := c.Next(); err != io.EOF {
								t.Fatal(err)
							}
						}
						c.Close()
					}
				}
			}
		})
	}
}

type boundedHookReader struct {
	PageReader
	pages, records       int
	denyPage, denyRecord bool
}

var boundedQuota = errors.New("descriptor-free quota refusal")

func (r *boundedHookReader) BeforePage(n int64) error {
	r.pages++
	if r.denyPage {
		return boundedQuota
	}
	return nil
}
func (r *boundedHookReader) BeforeRecord(n int) error {
	r.records++
	if r.denyRecord {
		return boundedQuota
	}
	return nil
}
func TestBoundedBudgetsCancellationAndClose(t *testing.T) {
	r := newIndexTestReader()
	addIndexTestPage(t, r, 2, true, [][]any{{strings.Repeat("x", 5000), int64(1)}}, nil)
	key := []any{strings.Repeat("x", 5000), int64(1)}
	for _, limits := range []RangeLimits{{PageReads: 1}, {Work: 1}, {Records: 1}, {PayloadBytes: 1}} {
		tracked := &boundedHookReader{PageReader: r}
		r.reads = nil
		c, err := NewBoundedIndexCursor(context.Background(), tracked, 2, KeyInfo{1}, &IndexBound{key, true}, nil, false, limits)
		if err == nil {
			_, err = c.Next()
			if _, again := c.Next(); again != err {
				t.Fatal("sticky quota", again, err)
			}
			c.Close()
			if _, e := c.Next(); e != io.EOF {
				t.Fatal(e)
			}
		}
		if !errors.Is(err, record.ErrLimit) {
			t.Fatal(limits, err)
		}
		if limits.PageReads > 0 && len(r.reads) > int(limits.PageReads) {
			t.Fatal("page quota exceeded")
		}
		if limits.PayloadBytes > 0 && tracked.records != 0 {
			t.Fatal("record admitted before quota")
		}
	}
	for _, recordHook := range []bool{false, true} {
		tracked := &boundedHookReader{PageReader: r, denyPage: !recordHook, denyRecord: recordHook}
		_, err := NewBoundedIndexCursor(context.Background(), tracked, 2, KeyInfo{1}, &IndexBound{key, true}, nil, false, RangeLimits{})
		if !errors.Is(err, boundedQuota) {
			t.Fatal("lost upstream hook", err)
		}
		if recordHook && len(r.reads) == 0 {
			t.Fatal("record hook wasn't reached")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	tracked := indexRangeCancelReader{PageReader: r, cancel: cancel}
	if _, err := NewBoundedIndexCursor(ctx, tracked, 2, KeyInfo{1}, nil, nil, true, RangeLimits{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	c, err := NewBoundedIndexCursor(ctx, r, 2, KeyInfo{1}, nil, nil, true, RangeLimits{})
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	for range 2 {
		if _, err = c.Next(); !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	}
	if c.stack != nil || c.base.visited != nil {
		t.Fatal("cancellation retained state")
	}
	c.Close()
	for _, bad := range []*IndexBound{{nil, true}, {[]any{true}, true}, {[]any{"x", float64(1)}, true}} {
		r.reads = nil
		if _, err := NewBoundedIndexCursor(context.Background(), r, 2, KeyInfo{1}, bad, nil, false, RangeLimits{}); !errors.Is(err, record.ErrUnsupported) || len(r.reads) != 0 {
			t.Fatal(bad, err, r.reads)
		}
	}
}

func TestBoundedRowIDBudgetsAndExactStop(t *testing.T) {
	ctx := context.Background()
	p := newMemoryPages(512)
	root, err := CreateTree(ctx, p, TableTree)
	if err != nil {
		t.Fatal(err)
	}
	for id := int64(-100); id <= 100; id++ {
		if err := InsertTable(ctx, p, root, id, []any{strings.Repeat("row", 20)}); err != nil {
			t.Fatal(err)
		}
	}
	for _, rev := range []bool{false, true} {
		tracked := &boundedCountReader{PageReader: p}
		bound := &RowIDBound{73, true}
		c, err := NewRowIDRangeCursor(ctx, tracked, root, bound, bound, rev, RangeLimits{})
		if err != nil {
			t.Fatal(err)
		}
		// Input reuse cannot change endpoints.
		bound.Value = -99
		id, _, err := c.Next()
		if err != nil || id != 73 {
			t.Fatal(id, err)
		}
		before := len(tracked.reads)
		for range 2 {
			if _, _, err = c.Next(); err != io.EOF {
				t.Fatal(err)
			}
		}
		if len(tracked.reads) != before || c.Stats().PageReads != int64(before) {
			t.Fatal("terminal overread")
		}
		equality := &boundedCountReader{PageReader: p}
		if _, ok, err := SeekTable(ctx, equality, root, 73); err != nil || !ok {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(tracked.reads, equality.reads) {
			t.Fatal("singleton did more than native equality", tracked.reads, equality.reads)
		}
		c.Close()
	}
	for _, limits := range []RangeLimits{{PageReads: 1}, {Work: 1}, {Records: 1}, {PayloadBytes: 1}} {
		tracked := &boundedCountReader{PageReader: p}
		c, err := NewRowIDRangeCursor(ctx, tracked, root, nil, nil, true, limits)
		if err == nil {
			for err == nil {
				_, _, err = c.Next()
			}
			for range 2 {
				if _, _, again := c.Next(); again != err {
					t.Fatal("sticky quota", err, again)
				}
			}
			c.Close()
			if _, _, e := c.Next(); e != io.EOF {
				t.Fatal(e)
			}
		}
		if !errors.Is(err, record.ErrLimit) {
			t.Fatal(limits, err)
		}
		if limits.PageReads > 0 && int64(len(tracked.reads)) > limits.PageReads {
			t.Fatal("page quota exceeded")
		}
	}
	for _, recordHook := range []bool{false, true} {
		hooked := &boundedHookReader{PageReader: p, denyPage: !recordHook, denyRecord: recordHook}
		c, err := NewRowIDRangeCursor(ctx, hooked, root, nil, nil, false, RangeLimits{})
		if err == nil {
			_, _, err = c.Next()
			c.Close()
		}
		if !errors.Is(err, boundedQuota) {
			t.Fatal("lost upstream hook", err)
		}
	}
	cancelCtx, cancel := context.WithCancel(ctx)
	c, err := NewRowIDRangeCursor(cancelCtx, p, root, nil, nil, true, RangeLimits{})
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	for range 2 {
		if _, _, err = c.Next(); !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	}
	if c.cursor.stack != nil || c.cursor.visited != nil {
		t.Fatal("cancel retained state")
	}
	c.Close()
	cancelCtx, cancel = context.WithCancel(ctx)
	cancelReader := indexRangeCancelReader{PageReader: p, cancel: cancel}
	if _, err := NewRowIDRangeCursor(cancelCtx, cancelReader, root, nil, nil, true, RangeLimits{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	tracked := &boundedCountReader{PageReader: p}
	empty, err := NewRowIDRangeCursor(ctx, tracked, root, &RowIDBound{5, false}, &RowIDBound{5, true}, false, RangeLimits{})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = empty.Next(); err != io.EOF || len(tracked.reads) != 0 || empty.lower != nil || empty.upper != nil {
		t.Fatal("empty interval read", err, tracked.reads)
	}
}

func TestBoundedOverflowCancellationOnReread(t *testing.T) {
	r := newIndexTestReader()
	key := []any{strings.Repeat("x", 5000), int64(1)}
	addIndexTestPage(t, r, 2, true, [][]any{key}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c, err := NewBoundedIndexCursor(ctx, r, 2, KeyInfo{1}, &IndexBound{key, true}, nil, false, RangeLimits{})
	if err != nil {
		t.Fatal(err)
	}
	// The initial seek has decoded/claimed the overflow chain. Cancel during its
	// ordinary reread, keeping the quota wrapper in place so every call counts.
	c.budget.reader = indexRangeCancelReader{PageReader: r, cancel: cancel}
	before := len(r.reads)
	for range 2 {
		if _, err = c.Next(); !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	}
	if len(r.reads) != before+1 || c.Stats().PageReads != int64(len(r.reads)) {
		t.Fatal("reread count", c.Stats(), r.reads)
	}
	if c.stack != nil || c.base.overflow != nil || c.base.visited != nil || c.base.last != nil {
		t.Fatal("canceled reread retained state")
	}
	c.Close()
}

type boundedCountReader struct {
	PageReader
	reads []uint32
}

func (r *boundedCountReader) ReadPage(ctx context.Context, n uint32) ([]byte, error) {
	r.reads = append(r.reads, n)
	return r.PageReader.ReadPage(ctx, n)
}

type boundedCancelAdmissionReader struct {
	PageReader
	cancel context.CancelFunc
}

func (r boundedCancelAdmissionReader) BeforePage(int64) error { r.cancel(); return nil }
func TestBoundedCancellationInsidePageAdmission(t *testing.T) {
	r := newIndexTestReader()
	addIndexTestPage(t, r, 2, true, [][]any{{"a", int64(1)}}, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	budget, err := newRangeBudget(ctx, boundedCancelAdmissionReader{r, cancel}, RangeLimits{})
	if err != nil {
		t.Fatal(err)
	}
	if err = budget.BeforePage(512); !errors.Is(err, context.Canceled) {
		t.Fatal("canceled admission permitted following allocation", err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	if _, err = NewBoundedIndexCursor(ctx, boundedCancelAdmissionReader{r, cancel}, 2, KeyInfo{1}, nil, nil, false, RangeLimits{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	// The original rowid parser also observes the forwarded cancellation before
	// allocating its cell/span buffers.
	p := newMemoryPages(512)
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	if _, err = NewRowIDRangeCursor(ctx, boundedCancelAdmissionReader{p, cancel}, 1, nil, nil, false, RangeLimits{}); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}
func TestBoundedEndpointCopyAdmission(t *testing.T) {
	r := newIndexTestReader()
	addIndexTestPage(t, r, 2, true, [][]any{{[]byte{0}, int64(1)}}, nil)
	lower := &IndexBound{[]any{make([]byte, 1024*1024)}, true}
	upper := &IndexBound{[]any{[]byte{255}}, true}
	for _, rev := range []bool{false, true} {
		r.reads = nil
		if _, err := NewBoundedIndexCursor(context.Background(), r, 2, KeyInfo{1}, lower, upper, rev, RangeLimits{Work: 1024}); !errors.Is(err, record.ErrLimit) || len(r.reads) != 0 {
			t.Fatal("endpoint allocation not preflighted", err, r.reads)
		}
	}
	// A valid empty interval requires neither endpoint copies nor page reads.
	c, err := NewBoundedIndexCursor(context.Background(), r, 2, KeyInfo{1}, upper, lower, false, RangeLimits{Work: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = c.Next(); err != io.EOF || c.Stats().Work != 0 || c.lower != nil || c.upper != nil {
		t.Fatal("empty interval allocated bounds", err, c.Stats())
	}
}
