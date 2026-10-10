package sqlite

import (
	"context"
	"errors"
	"io"
	"math"
	"math/big"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	"github.com/ms-xiaosnow/gosqlite/internal/pager"
	"github.com/ms-xiaosnow/gosqlite/internal/record"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
	"github.com/ms-xiaosnow/gosqlite/internal/testkit/fileview"
)

// Compare integer/REAL cuts with exact arbitrary-precision rationals, not the
// production SQL or B-tree comparator. This entire law test is descriptor-free.
func TestRowIDRangeRationalBoundaryLaw(t *testing.T) {
	ids := []int64{math.MinInt64, math.MinInt64 + 1, -9007199254740993, -9007199254740992, -42, -2, -1, 0, 1, 2, 42, 9007199254740992, 9007199254740993, math.MaxInt64 - 1, math.MaxInt64}
	probes := []float64{math.Inf(-1), math.Nextafter(-9223372036854775808.0, math.Inf(-1)), -9223372036854775808.0, math.Nextafter(-9223372036854775808.0, 0), -9007199254740992, -42.25, -1.5, -0.5, math.Copysign(0, -1), 0, math.SmallestNonzeroFloat64, 0.5, 1.5, 42.25, 9007199254740992, math.Nextafter(9223372036854775808.0, 0), 9223372036854775808.0, math.Inf(1)}
	for _, id := range ids {
		for _, v := range probes {
			for _, op := range []string{"<", "<=", ">", ">="} {
				var interval rowIDInterval
				interval.add(op, v)
				got := !interval.empty && (interval.lower == nil || id > interval.lower.Value || id == interval.lower.Value && interval.lower.Inclusive) && (interval.upper == nil || id < interval.upper.Value || id == interval.upper.Value && interval.upper.Inclusive)
				cmp := 0
				if math.IsInf(v, -1) {
					cmp = 1
				} else if math.IsInf(v, 1) {
					cmp = -1
				} else {
					cmp = new(big.Rat).SetInt64(id).Cmp(new(big.Rat).SetFloat64(v))
				}
				want := op == "<" && cmp < 0 || op == "<=" && cmp <= 0 || op == ">" && cmp > 0 || op == ">=" && cmp >= 0
				if got != want {
					t.Fatalf("%d %s %g: got %v want %v bounds %+v", id, op, v, got, want, interval)
				}
			}
		}
	}
}

type rowIDMemoryReader struct {
	h     pager.Header
	image []byte
	reads int
}

func (r *rowIDMemoryReader) Header() pager.Header { return r.h }
func (r *rowIDMemoryReader) ReadPage(ctx context.Context, n uint32) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	r.reads++
	start := int(n-1) * int(r.h.PageSize)
	return append([]byte(nil), r.image[start:start+int(r.h.PageSize)]...), nil
}

// Copy one existing intact fixture into memory, then operate only on those
// unchanged bytes. Tiny cursor quotas never inject any physical I/O failure.
func TestRowIDRangeSourceMemoryQuotasWorkAndOwnership(t *testing.T) {
	path := filepath.Join("..", "testdata", "readonly", "page512.db")
	image, err := fileview.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	header, err := pager.ParseHeader(image[:100], int64(len(image)))
	if err != nil {
		t.Fatal(err)
	}
	mem := &rowIDMemoryReader{h: header, image: image}
	db := &DB{}
	tab, err := db.table(context.Background(), mem, &sqlast.Select{From: "samples"})
	if err != nil {
		t.Fatal(err)
	}
	full := &rowIDMemoryReader{h: header, image: image}
	scan, err := btree.NewCursor(context.Background(), full, tab.root)
	if err != nil {
		t.Fatal(err)
	}
	total := 0
	for {
		_, _, err := scan.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		total++
	}
	scan.Close()
	for _, reverse := range []bool{false, true} {
		mem := &rowIDMemoryReader{h: header, image: image}
		var stats QueryStats
		s := &rowIDRangeSource{ctx: context.Background(), reader: mem, root: tab.root, bounds: rowIDInterval{lower: &btree.RowIDBound{Value: 10, Inclusive: true}, upper: &btree.RowIDBound{Value: 12, Inclusive: true}, seen: true}, reverse: reverse, stats: &stats}
		var got []int64
		for {
			id, values, err := s.Next()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, id)
			if blob, ok := values[2].([]byte); ok && len(blob) > 0 {
				blob[0] ^= 255
			}
		}
		want := []int64{10, 11, 12}
		if reverse {
			want = []int64{12, 11, 10}
		}
		if !reflect.DeepEqual(got, want) || stats.Candidates != 3 || stats.RowIDSeeks != 1 || mem.reads >= full.reads {
			t.Fatal(got, stats, mem.reads, full.reads)
		}
		if s.cursor.Stats().PageReads != int64(mem.reads) {
			t.Fatal("page accounting", s.cursor.Stats(), mem.reads)
		}
		t.Logf("reverse=%v: %d native range page calls/3 rows vs %d full scan page calls/%d rows", reverse, mem.reads, full.reads, total)
		s.Close()
	}
	for _, limits := range []btree.RangeLimits{{Work: 1}, {PayloadBytes: 1}, {Records: 1}} {
		var stats QueryStats
		s := &rowIDRangeSource{ctx: context.Background(), reader: mem, root: tab.root, stats: &stats, limits: limits}
		var got int
		var failure error
		for {
			_, _, failure = s.Next()
			if failure != nil {
				break
			}
			got++
		}
		if !errors.Is(failure, record.ErrLimit) || got > 1 {
			t.Fatal(limits, got, failure)
		}
		before := stats
		if _, _, again := s.Next(); again != failure || stats != before {
			t.Fatal("nonsticky source quota", again, failure, stats, before)
		}
		s.Close()
		if _, _, again := s.Next(); again != io.EOF {
			t.Fatal("closed source reopened", again)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var stats QueryStats
	s := &rowIDRangeSource{ctx: ctx, reader: mem, root: tab.root, stats: &stats}
	if _, _, err = s.Next(); !errors.Is(err, context.Canceled) || stats.RowIDSeeks != 0 {
		t.Fatal(stats, err)
	}
}
