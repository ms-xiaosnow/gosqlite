package sqlite

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// Independent finite IEEE-754 binary16 widening for reference arithmetic. The
// reference never uses HalfVector.Values, any production metric, or its heap.
func halfReviewWiden(bits []uint16) []float32 {
	values := make([]float32, len(bits))
	for i, b := range bits {
		e, m := int(b>>10&31), int(b&1023)
		x := math.Ldexp(float64(m), -24)
		if e != 0 {
			x = math.Ldexp(float64(1024+m), e-25)
		}
		if b&0x8000 != 0 {
			x = -x
		}
		values[i] = float32(x)
	}
	return values
}

func TestHalfReviewIndependentNativeHeapAndFullSort(t *testing.T) {
	db, _ := reviewVectorDB(t, Options{})
	reviewVectorExec(t, db, "CREATE TABLE halves(id INTEGER PRIMARY KEY,h HALFVEC(3),active INTEGER)")
	type item struct {
		id     int64
		bits   []uint16
		active int
	}
	qbits := []uint16{0x3c01, 0xbc02, 1}
	items := []item{{math.MinInt64, qbits, 1}, {math.MaxInt64, qbits, 1}, {-100, nil, 1}, {100, nil, 0}}
	for i := 0; i < 32; i++ {
		items = append(items, item{int64(i), []uint16{0x3b00 + uint16((i*23)%1536), 0xb800 + uint16((i*37)%1024), uint16(i % 8)}, i % 3})
	}
	tx, e := db.Begin(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	for _, it := range items {
		var value any
		if it.bits != nil {
			h, e := vector.NewHalfBits(it.bits)
			if e != nil {
				t.Fatal(e)
			}
			value = h
		}
		if _, e = tx.Exec(context.Background(), "INSERT INTO halves VALUES(?,?,?)", it.id, value, it.active); e != nil {
			t.Fatal(e)
		}
	}
	if e = tx.Commit(context.Background()); e != nil {
		t.Fatal(e)
	}
	type reference struct {
		id       int64
		distance float64
		null     bool
	}
	for qi, qb := range [][]uint16{qbits, {0x4000, 0x3400, 0xbc00}} {
		q, e := vector.NewHalfBits(qb)
		if e != nil {
			t.Fatal(e)
		}
		for _, metric := range []vector.MetricKind{vector.L2, vector.L1, vector.NegativeIP, vector.Cosine} {
			for _, filter := range []int{-1, 1} {
				where := "1"
				if filter >= 0 {
					where = fmt.Sprintf("active=%d", filter)
				}
				var want []reference
				var nativeWant []vector.Hit
				var filtered, nulls int64
				for _, it := range items {
					if filter >= 0 && it.active != filter {
						filtered++
						continue
					}
					r := reference{id: it.id, null: it.bits == nil}
					if r.null {
						nulls++
					} else {
						r.distance = reviewVectorDistance(halfReviewWiden(it.bits), halfReviewWiden(qb), metric)
						nativeWant = append(nativeWant, vector.Hit{RowID: r.id, Distance: r.distance})
					}
					want = append(want, r)
				}
				sort.Slice(want, func(i, j int) bool {
					a, b := want[i], want[j]
					if a.null != b.null {
						return a.null
					}
					return a.distance < b.distance || a.distance == b.distance && a.id < b.id
				})
				sort.Slice(nativeWant, func(i, j int) bool {
					a, b := nativeWant[i], nativeWant[j]
					return a.Distance < b.Distance || a.Distance == b.Distance && a.RowID < b.RowID
				})
				for _, window := range [][2]int{{1, 0}, {5, 2}, {100, 0}, {3, 100}} {
					limit, offset := window[0], window[1]
					op := map[vector.MetricKind]string{vector.L2: "<->", vector.L1: "<+>", vector.NegativeIP: "<#>", vector.Cosine: "<=>"}[metric]
					expr := "h " + op + " ?"
					if qi == 1 {
						fn := map[vector.MetricKind]string{vector.L2: "l2_distance", vector.L1: "l1_distance", vector.NegativeIP: "negative_inner_product", vector.Cosine: "cosine_distance"}[metric]
						expr = fn + "(?,h)"
					}
					prefix := "SELECT id," + expr + " AS distance FROM halves WHERE " + where + " ORDER BY distance"
					suffix := fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)
					got, p, s := reviewVectorQuery(t, db, prefix+suffix, q)
					full, fp, _ := reviewVectorQuery(t, db, prefix+",id"+suffix, q)
					if !reflect.DeepEqual(got, full) || p.VectorRepresentation != "halfvec" || p.VectorRanking != "bounded_heap" || !p.VectorExact || p.VectorMetric != metric || p.VectorTopK != int64(limit+offset) || fp.VectorRanking != "" {
						t.Fatal(metric, window, got, full, p, fp)
					}
					if p.ANNFallback != "HALFVEC(n) ANN is unsupported; exact binary16 ranking" || s.VectorRankPackedBytes != 0 || s.ANNExactRescores != 0 || s.VectorRankedRows != int64(len(nativeWant)) || s.VectorRankComponents != 3*(1+int64(len(nativeWant))) || s.BaseRowFetches != int64(len(items)) {
						t.Fatal(p, s)
					}
					start := min(offset, len(want))
					selected := want[start:min(start+limit, len(want))]
					if len(got) != len(selected) {
						t.Fatal(got, selected)
					}
					for i, h := range selected {
						if got[i][0] != h.id {
							t.Fatal(got, selected)
						}
						if h.null {
							if got[i][1] != nil {
								t.Fatal(got)
							}
						} else if math.Abs(got[i][1].(float64)-h.distance) > 1e-12 {
							t.Fatal(got[i], h)
						}
					}
					native, e := db.HalfSearch(context.Background(), "halves", "h", q, HalfSearchOptions{Metric: metric, Limit: limit, Where: where})
					if e != nil || len(native.Hits) != min(limit, len(nativeWant)) || native.Stats.FilteredRows != filtered || native.Stats.NullRows != nulls || native.Plan.Method != "exact_halfvec" || native.Plan.VectorRepresentation != "" || !native.Plan.Exact {
						t.Fatal(native, e)
					}
					for i, h := range native.Hits {
						if h.RowID != nativeWant[i].RowID || math.Abs(h.Distance-nativeWant[i].Distance) > 1e-12 {
							t.Fatal(native.Hits, nativeWant)
						}
					}
				}
			}
		}
	}
}

func halfReviewMemoryRows(t *testing.T, candidate, query any, heap bool) *Rows {
	t.Helper()
	tab := &table{name: "items", root: 2, ipk: 0, index: map[string]int{"id": 0, "h": 1, "active": 2}, columns: []sqlast.ColumnDef{{Name: "id", Type: "INTEGER", PrimaryKey: true}, {Name: "h", Type: "HALFVEC(2)"}, {Name: "active", Type: "INTEGER"}}}
	expr := &sqlast.Binary{Op: "<->", Left: &sqlast.Column{Name: "h"}, Right: &sqlast.Literal{Value: query}}
	r := &Rows{db: &DB{opts: Options{MaxSortBytes: 1 << 20}}, ctx: context.Background(), table: tab, sel: &sqlast.Select{}, exprs: []sqlast.Expr{&sqlast.Column{Name: "id"}}, aliases: map[string]int{}, order: []orderExpr{{expr: expr, column: -1}}, source: &reviewVectorSource{rows: [][]any{{nil, candidate, int64(1)}}}, limit: 1}
	if heap {
		if e := r.prepareVectorRank(); e != nil {
			t.Fatal(e)
		}
	}
	return r
}

func TestHalfReviewMemoryTypedNullAndLogicalBudgets(t *testing.T) {
	// Malformed representation inputs exist only in this in-memory source.
	for _, bad := range []any{"[1]", int64(1), "bad", []byte("[1,2]")} {
		for _, heap := range []bool{false, true} {
			r := halfReviewMemoryRows(t, bad, nil, heap)
			if e := r.materialize(); !errors.Is(e, ErrMismatch) {
				t.Fatal(heap, bad, e)
			}
			if r.source != nil {
				r.source.Close()
			}
		}
	}
	for _, pair := range [][2]any{{nil, "bad"}, {"[1.0001,2]", nil}, {nil, nil}} {
		for _, heap := range []bool{false, true} {
			r := halfReviewMemoryRows(t, pair[0], pair[1], heap)
			if e := r.materialize(); e != nil || len(r.sorted) != 1 || r.sorted[0].keys[0] != nil {
				t.Fatal(heap, pair, r.sorted, e)
			}
			if r.source != nil {
				r.source.Close()
			}
		}
	}
	for _, filtered := range []bool{false, true} {
		r := halfReviewMemoryRows(t, nil, nil, true)
		r.vectorRank.limits.MaxCandidates = 3
		r.source = &reviewVectorSource{rows: [][]any{{nil, nil, int64(0)}, {nil, nil, int64(0)}, {nil, nil, int64(0)}, {nil, nil, int64(0)}}}
		if filtered {
			r.sel.Where = &sqlast.Column{Name: "active"}
		}
		if e := r.materialize(); !errors.Is(e, vector.ErrLimit) || len(r.sorted) != 0 || r.queryStats.BaseRowFetches != 4 {
			t.Fatal(filtered, r.sorted, r.queryStats, e)
		}
		if r.source != nil {
			r.source.Close()
		}
	}
	for _, components := range []uint64{1, 2, 3, 5} {
		r := halfReviewMemoryRows(t, "[1,2]", "[1,2]", true)
		r.vectorRank.limits.MaxComponents = components
		r.source = &reviewVectorSource{rows: [][]any{{nil, "[1,2]", int64(1)}, {nil, "[1,2]", int64(1)}}}
		if e := r.materialize(); !errors.Is(e, vector.ErrLimit) || len(r.sorted) != 0 {
			t.Fatal(components, r.sorted, e)
		}
		if r.source != nil {
			r.source.Close()
		}
	}
	for _, heap := range []bool{false, true} {
		r := halfReviewMemoryRows(t, "[1,2]", "[1,2]", heap)
		r.db.opts.MaxSortBytes = 16
		if e := r.materialize(); !errors.Is(e, ErrMemoryLimit) || len(r.sorted) != 0 {
			t.Fatal(heap, r.sorted, e)
		}
		if r.source != nil {
			r.source.Close()
		}
	}
}

type halfReviewValuer func() (driver.Value, error)

func (v halfReviewValuer) Value() (driver.Value, error) { return v() }

func TestHalfReviewCallbackCancellationAndReaderRelease(t *testing.T) {
	db, path := reviewVectorDB(t, Options{WALShared: true})
	reviewVectorExec(t, db, "CREATE TABLE halves(id INTEGER PRIMARY KEY,h HALFVEC(2))")
	tx, e := db.Begin(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 80; i++ {
		if _, e = tx.Exec(context.Background(), "INSERT INTO halves VALUES(?,?)", i, fmt.Sprintf("[%d,1]", i)); e != nil {
			t.Fatal(e)
		}
	}
	if e = tx.Commit(context.Background()); e != nil {
		t.Fatal(e)
	}
	for _, sqlRank := range []bool{false, true} {
		base, cancel := context.WithCancel(context.Background())
		ctx := &reviewVectorCancelContext{Context: base, cancel: cancel}
		if sqlRank {
			r, e := db.Query(ctx, "SELECT id FROM halves ORDER BY h <-> '[1,1]' LIMIT 5")
			if !errors.Is(e, context.Canceled) || r != nil {
				if r != nil {
					r.Close()
				}
				t.Fatal(r, e)
			}
		} else {
			r, e := db.HalfSearch(ctx, "halves", "h", "[1,1]", HalfSearchOptions{Limit: 5})
			if !errors.Is(e, context.Canceled) || !reflect.DeepEqual(r, HalfResult{}) {
				t.Fatal(r, e)
			}
		}
		cancel()
	}
	for _, value := range []driver.Value{nil, "[1,1]"} {
		ctx, cancel := context.WithCancel(context.Background())
		calls := 0
		q := halfReviewValuer(func() (driver.Value, error) { calls++; cancel(); return value, nil })
		r, e := db.HalfSearch(ctx, "halves", "h", q, HalfSearchOptions{Limit: 1})
		if !errors.Is(e, context.Canceled) || !reflect.DeepEqual(r, HalfResult{}) || calls != 1 {
			t.Fatal(r, e, calls)
		}
		cancel()
	}
	failure := errors.New("valuer rejected half query")
	r, e := db.HalfSearch(context.Background(), "halves", "h", halfReviewValuer(func() (driver.Value, error) { return nil, failure }), HalfSearchOptions{Limit: 1})
	if !errors.Is(e, failure) || !reflect.DeepEqual(r, HalfResult{}) {
		t.Fatal(r, e)
	}
	other, e := Open(path, Options{WALShared: true})
	if e != nil {
		t.Fatal(e)
	}
	defer other.Close()
	reviewVectorExec(t, other, "UPDATE halves SET h='[1,1]' WHERE id=79")
	if e = other.Checkpoint(context.Background()); e != nil {
		t.Fatal("failed searches retained reader", e)
	}
	got, _, _ := reviewVectorQuery(t, db, "SELECT id FROM halves ORDER BY h <-> '[1,1]' LIMIT 2")
	if !reflect.DeepEqual(got, [][]any{{int64(1)}, {int64(79)}}) {
		t.Fatal(got)
	}
	rows, e := db.Query(context.Background(), "SELECT id FROM halves ORDER BY h <-> '[1,1]' LIMIT 2")
	if e != nil {
		t.Fatal(e)
	}
	if rows.vectorRank == nil || !rows.vectorRank.parsedHalf.Valid() {
		t.Fatal("missing decoded half rank")
	}
	if e = rows.Close(); e != nil {
		t.Fatal(e)
	}
	if rows.vectorRank != nil || rows.params != nil || rows.sorted != nil || rows.Plan().VectorRepresentation != "halfvec" || rows.Stats().VectorRankedRows != 80 {
		t.Fatal("close ownership", rows.Plan(), rows.Stats())
	}
}

func TestHalfReviewLossyRerankSameSnapshotDiagnostics(t *testing.T) {
	ctx := context.Background()
	db, path := reviewVectorDB(t, Options{WALShared: true})
	reviewVectorExec(t, db, "CREATE TABLE halves(id INTEGER PRIMARY KEY,h HALFVEC(2),d VECTOR(2),active INTEGER)")
	for _, it := range []struct {
		id     int
		v      string
		active int
	}{{10, "[1.0004,1]", 1}, {20, "[1.00021,1]", 1}, {5, "[1.0002,1]", 0}} {
		reviewVectorExec(t, db, "INSERT INTO halves VALUES(?,halfvec_from_vector(?),?,?)", it.id, it.v, it.v, it.active)
	}
	q, e := vector.Parse("[1.0002,1]")
	if e != nil {
		t.Fatal(e)
	}
	h, loss, e := vector.HalfFromVector(q)
	if e != nil {
		t.Fatal(e)
	}
	wantAbs := float64(float32(1.0002)) - 1
	wantRel := wantAbs / float64(float32(1.0002))
	if loss.Changed != 1 || loss.MaxAbsoluteError != wantAbs || loss.MaxRelativeError != wantRel {
		t.Fatal(loss, wantAbs, wantRel)
	}
	diagnostics, _, _ := reviewVectorQuery(t, db, "SELECT halfvec_conversion_changed(?),halfvec_conversion_max_abs(?),halfvec_conversion_max_rel(?)", q, q, q)
	if !reflect.DeepEqual(diagnostics, [][]any{{int64(1), wantAbs, wantRel}}) {
		t.Fatal(diagnostics)
	}
	tx, e := db.Begin(ctx, TxOptions{ReadOnly: true})
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	exact, e := tx.VectorSearch(ctx, "halves", "d", q, VectorSearchOptions{Limit: 1, Where: "active=1"})
	if e != nil || exact.Hits[0].RowID != 20 {
		t.Fatal(exact, e)
	}
	short, e := tx.HalfSearch(ctx, "halves", "h", h, HalfSearchOptions{Limit: 1, Where: "active=1"})
	if e != nil || !reflect.DeepEqual(short.Hits, []vector.Hit{{RowID: 10}}) || short.Plan.Method != "exact_halfvec" || !short.Plan.Exact || short.Stats.ScoredComponents != 6 || short.Stats.FilteredRows != 1 {
		t.Fatal(short, e)
	}
	writer, e := Open(path, Options{WALShared: true})
	if e != nil {
		t.Fatal(e)
	}
	defer writer.Close()
	reviewVectorExec(t, writer, "UPDATE halves SET h=halfvec_from_vector('[8,8]'),d='[8,8]',active=0 WHERE id=20")
	for _, k := range []int{1, 2} {
		candidates, e := tx.HalfSearch(ctx, "halves", "h", h, HalfSearchOptions{Limit: k, Where: "active=1"})
		if e != nil {
			t.Fatal(e)
		}
		var reranked []vector.Hit
		for _, hit := range candidates.Hits {
			rows, e := tx.Query(ctx, "SELECT id,d <-> ? FROM halves WHERE active=1 AND id=?", q, hit.RowID)
			if e != nil {
				t.Fatal(e)
			}
			if !rows.Next() {
				t.Fatal("candidate disappeared from pinned snapshot", rows.Err())
			}
			v := rows.Values()
			reranked = append(reranked, vector.Hit{RowID: v[0].(int64), Distance: v[1].(float64)})
			if e = rows.Close(); e != nil {
				t.Fatal(e)
			}
		}
		sort.Slice(reranked, func(i, j int) bool {
			return reranked[i].Distance < reranked[j].Distance || reranked[i].Distance == reranked[j].Distance && reranked[i].RowID < reranked[j].RowID
		})
		wantID := int64(10)
		if k == 2 {
			wantID = 20
		}
		if len(reranked) != k || reranked[0].RowID != wantID {
			t.Fatal(k, reranked)
		}
		// The truncated half set has measured recall@1=0; expanded fixture=1.
		if (reranked[0].RowID == exact.Hits[0].RowID) != (k == 2) {
			t.Fatal("incorrect recall diagnostic", k, reranked, exact.Hits)
		}
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	fresh, e := db.VectorSearch(ctx, "halves", "d", q, VectorSearchOptions{Limit: 1, Where: "active=1"})
	if e != nil || fresh.Hits[0].RowID != 10 {
		t.Fatal(fresh, e)
	}
	if e = writer.Checkpoint(ctx); e != nil {
		t.Fatal(e)
	}
}
