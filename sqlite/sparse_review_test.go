package sqlite

import (
	"bytes"
	"context"
	"database/sql/driver"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

func sparseReviewMemoryRows(t *testing.T, candidate, query any, heap bool) *Rows {
	t.Helper()
	tab := &table{name: "sparse_items", root: 2, ipk: 0, index: map[string]int{"id": 0, "s": 1, "active": 2}, columns: []sqlast.ColumnDef{{Name: "id", Type: "INTEGER", PrimaryKey: true}, {Name: "s", Type: "SPARSEVEC(1000000000)"}, {Name: "active", Type: "INTEGER"}}}
	expr := &sqlast.Binary{Op: "<->", Left: &sqlast.Column{Name: "s"}, Right: &sqlast.Literal{Value: query}}
	r := &Rows{db: &DB{opts: Options{MaxSortBytes: 1 << 20}}, ctx: context.Background(), table: tab, sel: &sqlast.Select{}, exprs: []sqlast.Expr{&sqlast.Column{Name: "id"}}, aliases: map[string]int{}, order: []orderExpr{{expr: expr, column: -1}}, source: &reviewVectorSource{rows: [][]any{{nil, candidate, int64(1)}}}, limit: 1}
	if heap {
		if e := r.prepareVectorRank(); e != nil {
			t.Fatal(e)
		}
	}
	t.Cleanup(func() {
		if r.source != nil {
			r.source.Close()
		}
	})
	return r
}

func TestSparseReviewMemoryLazyNullAndSupportBudgets(t *testing.T) {
	for _, bad := range []any{"{} /2", "bad", int64(1), []byte("{} /1000000000")} {
		for _, heap := range []bool{false, true} {
			r := sparseReviewMemoryRows(t, bad, nil, heap)
			if e := r.materialize(); !errors.Is(e, ErrMismatch) || len(r.sorted) != 0 {
				t.Fatal(bad, heap, e, r.sorted)
			}
		}
	}
	for _, pair := range [][2]any{{nil, "bad"}, {"{} /1000000000", nil}, {nil, nil}} {
		for _, heap := range []bool{false, true} {
			r := sparseReviewMemoryRows(t, pair[0], pair[1], heap)
			if e := r.materialize(); e != nil || len(r.sorted) != 1 || r.sorted[0].keys[0] != nil {
				t.Fatal(pair, heap, e, r.sorted)
			}
		}
	}
	for _, filtered := range []bool{false, true} {
		r := sparseReviewMemoryRows(t, nil, "bad", true)
		r.vectorRank.limits.MaxCandidates = 3
		r.source = &reviewVectorSource{rows: [][]any{{nil, nil, int64(0)}, {nil, nil, int64(0)}, {nil, nil, int64(0)}, {nil, nil, int64(0)}}}
		if filtered {
			r.sel.Where = &sqlast.Column{Name: "active"}
		}
		if e := r.materialize(); !errors.Is(e, vector.ErrLimit) || len(r.sorted) != 0 || r.queryStats.BaseRowFetches != 4 {
			t.Fatal(filtered, e, r.queryStats)
		}
	}
	// Explicit zeros count conservatively before canonicalization. Charges are
	// query=3, then (query=3 + candidate=2) per row, total 13, never 1e9.
	for _, budget := range []uint64{1, 3, 7, 8, 12, 13} {
		r := sparseReviewMemoryRows(t, "{1:1,8:0}/1000000000", "{1:1,9:0,10:-0}/1000000000", true)
		r.vectorRank.limits.MaxComponents = budget
		r.source = &reviewVectorSource{rows: [][]any{{nil, "{1:1,8:0}/1000000000", int64(1)}, {nil, "{1:1,8:0}/1000000000", int64(1)}}}
		e := r.materialize()
		if budget < 13 {
			if !errors.Is(e, vector.ErrLimit) || len(r.sorted) != 0 {
				t.Fatal(budget, e, r.sorted)
			}
		} else if e != nil || r.queryStats.VectorRankNonzeros != 13 || r.queryStats.VectorRankComponents != 0 || r.queryStats.VectorRankPackedBytes != 0 {
			t.Fatal(e, r.queryStats)
		}
	}
	// Entry count preflight must reject scratch before parsing malformed input.
	r := sparseReviewMemoryRows(t, strings.Repeat(":", 500), "{} /1000000000", true)
	r.db.opts.MaxSortBytes = 1000
	if e := r.materialize(); !errors.Is(e, ErrMemoryLimit) || len(r.sorted) != 0 {
		t.Fatal(e)
	}
	for _, heap := range []bool{false, true} {
		r := sparseReviewMemoryRows(t, "{} /1000000000", "{} /1000000000", heap)
		r.db.opts.MaxSortBytes = 16
		if e := r.materialize(); !errors.Is(e, ErrMemoryLimit) || len(r.sorted) != 0 {
			t.Fatal(heap, e)
		}
	}
	// Filtered malformed candidates are not decoded on either ranking path.
	for _, heap := range []bool{false, true} {
		r := sparseReviewMemoryRows(t, "bad", "bad", heap)
		r.sel.Where = &sqlast.Literal{Value: int64(0)}
		if e := r.materialize(); e != nil || len(r.sorted) != 0 {
			t.Fatal(heap, e, r.sorted)
		}
	}
}

func TestSparseReviewMemoryExternalDefaultsAndAliases(t *testing.T) {
	for _, heap := range []bool{false, true} {
		r := sparseReviewMemoryRows(t, nil, "{1000000000:5}/1000000000", heap)
		r.table.columns[1].Default = &sqlast.Literal{Value: "{1000000000:5}/1000000000"}
		r.source = &reviewVectorSource{rows: [][]any{{nil}}}
		r.exprs = []sqlast.Expr{&sqlast.Column{Name: "id"}, &sqlast.Column{Name: "s"}}
		if e := r.materialize(); e != nil || len(r.sorted) != 1 || r.sorted[0].values[1] != "{1000000000:5}/1000000000" || r.sorted[0].keys[0] != float64(0) {
			t.Fatal(heap, e, r.sorted)
		}
	}
	for _, filter := range []bool{false, true} {
		r := sparseReviewMemoryRows(t, nil, nil, true)
		r.table.columns[1].Default = &sqlast.Literal{Value: "1"}
		r.source = &reviewVectorSource{rows: [][]any{{nil}}}
		if filter {
			r.sel.Where = &sqlast.Literal{Value: int64(0)}
		}
		e := r.materialize()
		if filter {
			if e != nil || len(r.sorted) != 0 {
				t.Fatal(e)
			}
		} else if !errors.Is(e, ErrMismatch) {
			t.Fatal(e)
		}
	}
	for _, expr := range []string{"s", "+s", "coalesce(NULL,s)", "ifnull(s,NULL)", "nullif(s,NULL)"} {
		r := sparseReviewMemoryRows(t, "{1000000000:5}/1000000000", nil, false)
		ast, e := sqlast.ParseExpr(expr)
		if e != nil {
			t.Fatal(e)
		}
		r.exprs = []sqlast.Expr{ast}
		r.aliases = map[string]int{"chosen": 0}
		r.order = nil
		r.sel.Where, e = sqlast.ParseExpr("chosen <-> '{1000000000:5}/1000000000' = 0")
		if e != nil {
			t.Fatal(e)
		}
		if e := r.materialize(); e != nil || len(r.sorted) != 1 || r.sorted[0].values[0] != "{1000000000:5}/1000000000" {
			t.Fatal(expr, e, r.sorted)
		}
	}
}

// Independent reference embeds just three support coordinates at positions
// 0, 499999999, 999999999. It uses no production vector metric or heap.
func sparseReviewDistance(a, b [3]float64, m vector.MetricKind) float64 {
	var dot, aa, bb, l1, l2 float64
	for i, x := range a {
		y := b[i]
		dot += x * y
		aa += x * x
		bb += y * y
		l1 += math.Abs(x - y)
		l2 += (x - y) * (x - y)
	}
	switch m {
	case vector.L2:
		return math.Sqrt(l2)
	case vector.L1:
		return l1
	case vector.NegativeIP:
		return -dot
	default:
		return 1 - math.Max(-1, math.Min(1, dot/math.Sqrt(aa*bb)))
	}
}
func sparseReviewText(a [3]float64) string {
	return fmt.Sprintf("{1:%g,500000000:%g,1000000000:%g}/1000000000", a[0], a[1], a[2])
}

func TestSparseReviewIndependentBillionDimensionalRankings(t *testing.T) {
	db, _ := reviewVectorDB(t, Options{})
	reviewVectorExec(t, db, "CREATE TABLE sparse_review(id INTEGER PRIMARY KEY,s SPARSEVEC(1000000000),active INTEGER)")
	type item struct {
		id     int64
		a      [3]float64
		active int
		null   bool
	}
	queries := [][3]float64{{3, -2, 1}, {-1, 4, 2}}
	items := []item{{math.MinInt64, queries[0], 1, false}, {math.MaxInt64, queries[0], 1, false}, {-50, [3]float64{}, 1, true}, {-40, [3]float64{}, 0, true}}
	for i := 0; i < 45; i++ {
		items = append(items, item{int64(i), [3]float64{float64(i%9 - 4), float64(i%7 - 3), float64(i%5 + 1)}, i % 3, false})
	}
	tx, e := db.Begin(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	for _, it := range items {
		var value any
		if !it.null {
			value = sparseReviewText(it.a)
		}
		if _, e = tx.Exec(context.Background(), "INSERT INTO sparse_review VALUES(?,?,?)", it.id, value, it.active); e != nil {
			t.Fatal(e)
		}
	}
	if e = tx.Commit(context.Background()); e != nil {
		t.Fatal(e)
	}
	type rank struct {
		id       int64
		distance float64
		null     bool
	}
	for _, q := range queries {
		for _, m := range []vector.MetricKind{vector.L2, vector.L1, vector.NegativeIP, vector.Cosine} {
			for _, filtered := range []bool{false, true} {
				var want []rank
				var native []vector.Hit
				var filteredCount, nullCount int64
				where := "1"
				if filtered {
					where = "active=1"
				}
				for _, it := range items {
					if filtered && it.active != 1 {
						filteredCount++
						continue
					}
					r := rank{id: it.id, null: it.null}
					if it.null {
						nullCount++
					} else {
						r.distance = sparseReviewDistance(it.a, q, m)
						native = append(native, vector.Hit{RowID: r.id, Distance: r.distance})
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
				sort.Slice(native, func(i, j int) bool {
					return native[i].Distance < native[j].Distance || native[i].Distance == native[j].Distance && native[i].RowID < native[j].RowID
				})
				got, e := db.SparseSearch(context.Background(), "sparse_review", "s", sparseReviewText(q), SparseSearchOptions{Metric: m, Limit: 9, Where: where})
				if e != nil || len(got.Hits) != 9 || got.Stats.BaseRowFetches != int64(len(items)) || got.Stats.FilteredRows != filteredCount || got.Stats.NullRows != nullCount || got.Plan.Method != "exact_sparsevec" || got.Plan.VectorRepresentation != "" {
					t.Fatal(got, e)
				}
				for i, h := range got.Hits {
					if h.RowID != native[i].RowID || math.Abs(h.Distance-native[i].Distance) > 1e-12 {
						t.Fatal(m, h, native[i])
					}
				}
				fn := map[vector.MetricKind]string{vector.L2: "l2_distance", vector.L1: "l1_distance", vector.NegativeIP: "negative_inner_product", vector.Cosine: "cosine_distance"}[m]
				for _, expr := range []string{fn + "(s,?)", fn + "(?,s)"} {
					for _, window := range [][2]int{{1, 0}, {9, 2}, {100, 0}, {3, 100}} {
						prefix := "SELECT id," + expr + " AS distance FROM sparse_review WHERE " + where + " ORDER BY 2"
						suffix := fmt.Sprintf(" LIMIT %d OFFSET %d", window[0], window[1])
						rows, p, stats := reviewVectorQuery(t, db, prefix+suffix, sparseReviewText(q))
						full, fp, _ := reviewVectorQuery(t, db, prefix+",id"+suffix, sparseReviewText(q))
						if !reflect.DeepEqual(rows, full) || p.VectorRepresentation != "sparsevec" || p.VectorRanking != "bounded_heap" || p.VectorDimensions != 1000000000 || !p.VectorExact || p.VectorTopK != int64(window[0]+window[1]) || fp.VectorRanking != "" || stats.VectorRankComponents != 0 || stats.VectorRankPackedBytes != 0 || stats.ANNExactRescores != 0 {
							t.Fatal(rows, full, p, fp, stats)
						}
						start := min(window[1], len(want))
						selected := want[start:min(start+window[0], len(want))]
						if len(rows) != len(selected) {
							t.Fatal(rows, selected)
						}
						for i, r := range rows {
							w := selected[i]
							if r[0] != w.id {
								t.Fatal(rows, selected)
							}
							if w.null {
								if r[1] != nil {
									t.Fatal(r)
								}
							} else if math.Abs(r[1].(float64)-w.distance) > 1e-12 {
								t.Fatal(r, w)
							}
						}
					}
				}
			}
		}
	}
}

func TestSparseReviewExactBytesStatementsAndUnsupportedBoundaries(t *testing.T) {
	ctx := context.Background()
	db, _ := reviewVectorDB(t, Options{})
	reviewVectorExec(t, db, "CREATE TABLE sparse_review(id INTEGER PRIMARY KEY,s SPARSEVEC(1000000000) DEFAULT '{1000000000:-2,1:3,9:-0}/1000000000')")
	reviewVectorExec(t, db, "INSERT INTO sparse_review DEFAULT VALUES")
	want := make([]byte, 32)
	copy(want, []byte{'G', 'S', 'S', 0, 1, 4, 1, 0})
	binary.BigEndian.PutUint32(want[8:], 1000000000)
	binary.BigEndian.PutUint32(want[12:], 2)
	binary.BigEndian.PutUint32(want[20:], 0x40400000)
	binary.BigEndian.PutUint32(want[24:], 999999999)
	binary.BigEndian.PutUint32(want[28:], 0xc0000000)
	rows, _, _ := reviewVectorQuery(t, db, "SELECT s,typeof(s) FROM sparse_review")
	if len(rows) != 1 || !bytes.Equal(rows[0][0].([]byte), want) || rows[0][1] != "blob" {
		t.Fatal(rows, want)
	}
	tx, e := db.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, "INSERT INTO sparse_review VALUES(2,'{} /1000000000')"); e != nil {
		t.Fatal(e)
	}
	if _, e = tx.Exec(ctx, "UPDATE sparse_review SET s='{1:3}/2'"); !errors.Is(e, vector.ErrDimensions) {
		t.Fatal(e)
	}
	got, e := tx.SparseSearch(ctx, "sparse_review", "s", want, SparseSearchOptions{Limit: 2})
	if e != nil || len(got.Hits) != 2 || got.Hits[0].RowID != 1 || got.Hits[1].Distance != math.Sqrt(13) {
		t.Fatal(got, e)
	}
	if e = tx.Rollback(); e != nil {
		t.Fatal(e)
	}
	rows, _, _ = reviewVectorQuery(t, db, "SELECT id FROM sparse_review")
	if !reflect.DeepEqual(rows, [][]any{{int64(1)}}) {
		t.Fatal(rows)
	}
	for _, q := range []string{"SELECT subvector(s,1,1) FROM sparse_review", "SELECT binary_quantize(s) FROM sparse_review"} {
		r, e := db.Query(ctx, q)
		if e == nil {
			for r.Next() {
			}
			e = r.Err()
			r.Close()
		}
		if !errors.Is(e, ErrUnsupported) {
			t.Fatal(q, e)
		}
	}
	if r, e := db.Query(ctx, "SELECT vector_from_sparsevec(s) FROM sparse_review"); e == nil {
		for r.Next() {
		}
		e = r.Err()
		r.Close()
		if !errors.Is(e, vector.ErrLimit) {
			t.Fatal(e)
		}
	} else if !errors.Is(e, vector.ErrLimit) {
		t.Fatal(e)
	}
	if e := db.CreateHNSWIndex(ctx, "sparse_hnsw", "sparse_review", "s"); e != nil {
		t.Fatal(e)
	}
	if e := db.CreateIVFFlatIndex(ctx, "sparse_ivf", "sparse_review", "s"); !errors.Is(e, ErrUnsupported) {
		t.Fatal(e)
	}
	if got, e := db.VectorSearch(ctx, "sparse_review", "s", want, VectorSearchOptions{Limit: 1}); !errors.Is(e, ErrMismatch) || !reflect.DeepEqual(got, VectorResult{}) {
		t.Fatal(got, e)
	}
}

type sparseReviewValuer func() (driver.Value, error)

func (v sparseReviewValuer) Value() (driver.Value, error) { return v() }

func TestSparseReviewNativeBoundaryCancellationAndRelease(t *testing.T) {
	ctx := context.Background()
	db, path := reviewVectorDB(t, Options{WALShared: true})
	reviewVectorExec(t, db, "CREATE TABLE sparse_review(id INTEGER PRIMARY KEY,s SPARSEVEC(1000000000))")
	tx, e := db.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 80; i++ {
		if _, e = tx.Exec(ctx, "INSERT INTO sparse_review VALUES(?,?)", i, fmt.Sprintf("{1:%d,1000000000:1}/1000000000", i)); e != nil {
			t.Fatal(e)
		}
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	for _, sqlRank := range []bool{false, true} {
		base, cancel := context.WithCancel(ctx)
		c := &reviewVectorCancelContext{Context: base, cancel: cancel}
		if sqlRank {
			r, e := db.Query(c, "SELECT id FROM sparse_review ORDER BY s <-> '{1:1,1000000000:1}/1000000000' LIMIT 5")
			if r != nil {
				r.Close()
			}
			if !errors.Is(e, context.Canceled) || r != nil {
				t.Fatal(r, e)
			}
		} else {
			r, e := db.SparseSearch(c, "sparse_review", "s", "{1:1,1000000000:1}/1000000000", SparseSearchOptions{Limit: 5})
			if !errors.Is(e, context.Canceled) || !reflect.DeepEqual(r, SparseResult{}) {
				t.Fatal(r, e)
			}
		}
		cancel()
	}
	failure := errors.New("query value unavailable")
	for _, out := range []driver.Value{nil, "{} /1000000000"} {
		c, cancel := context.WithCancel(ctx)
		calls := 0
		r, e := db.SparseSearch(c, "sparse_review", "s", sparseReviewValuer(func() (driver.Value, error) { calls++; cancel(); return out, nil }), SparseSearchOptions{Limit: 1})
		if !errors.Is(e, context.Canceled) || !reflect.DeepEqual(r, SparseResult{}) || calls != 1 {
			t.Fatal(r, e, calls)
		}
	}
	r, e := db.SparseSearch(ctx, "sparse_review", "s", sparseReviewValuer(func() (driver.Value, error) { return nil, failure }), SparseSearchOptions{Limit: 1})
	if !errors.Is(e, failure) || !reflect.DeepEqual(r, SparseResult{}) {
		t.Fatal(r, e)
	}
	for _, metric := range []vector.MetricKind{"L2", "hamming", "jaccard", "euclidean"} {
		r, e := db.SparseSearch(ctx, "sparse_review", "s", nil, SparseSearchOptions{Metric: metric})
		if !errors.Is(e, vector.ErrMetric) || !reflect.DeepEqual(r, SparseResult{}) {
			t.Fatal(metric, r, e)
		}
	}
	other, e := Open(path, Options{WALShared: true})
	if e != nil {
		t.Fatal(e)
	}
	defer other.Close()
	reviewVectorExec(t, other, "UPDATE sparse_review SET s='{1:1,1000000000:1}/1000000000' WHERE id=79")
	if e = other.Checkpoint(ctx); e != nil {
		t.Fatal("canceled sparse search retained reader", e)
	}
	rows, e := db.Query(ctx, "SELECT id FROM sparse_review ORDER BY s <-> '{1:1,1000000000:1}/1000000000' LIMIT 2")
	if e != nil {
		t.Fatal(e)
	}
	if rows.vectorRank == nil || !rows.vectorRank.parsedSparse.Valid() {
		t.Fatal("missing sparse state")
	}
	if e = rows.Close(); e != nil {
		t.Fatal(e)
	}
	if rows.vectorRank != nil || rows.params != nil || rows.sorted != nil || rows.Plan().VectorRepresentation != "sparsevec" || rows.Stats().VectorRankedRows != 80 {
		t.Fatal("close ownership", rows.Plan(), rows.Stats())
	}
	got, _, _ := reviewVectorQuery(t, db, "SELECT id FROM sparse_review ORDER BY s <-> '{1:1,1000000000:1}/1000000000' LIMIT 2")
	if !reflect.DeepEqual(got, [][]any{{int64(1)}, {int64(79)}}) {
		t.Fatal(got)
	}
}

func TestSparseReviewNativePreflightEmptyAndClosedOptions(t *testing.T) {
	ctx := context.Background()
	db, path := reviewVectorDB(t, Options{})
	reviewVectorExec(t, db, "CREATE TABLE sparse_preflight(id INTEGER PRIMARY KEY,s SPARSEVEC(1000000000),active INTEGER,d VECTOR(1),h HALFVEC(1),b BIT(1))")
	reviewVectorExec(t, db, "INSERT INTO sparse_preflight VALUES(1,'{1000000000:1}/1000000000',1,'[1]','[1]','1'),(2,'{} /1000000000',0,NULL,NULL,NULL),(3,NULL,0,NULL,NULL,NULL)")
	failure := func(db *DB, query any, opt SparseSearchOptions, want error) {
		t.Helper()
		r, e := db.SparseSearch(ctx, "sparse_preflight", "s", query, opt)
		if !errors.Is(e, want) || !reflect.DeepEqual(r, SparseResult{}) {
			t.Fatal(query, opt, r, e, want)
		}
	}
	for _, limits := range []SparseSearchLimits{{MaxCandidates: 1}, {MaxCandidates: 1, MaxResults: 1}, {MaxCandidates: 1, MaxResults: 1, MaxNonzeros: 1<<34 + 1}, {MaxCandidates: 1<<20 + 1, MaxResults: 1, MaxNonzeros: 1}, {MaxCandidates: 1, MaxResults: 1<<20 + 1, MaxNonzeros: 1}, {MaxCandidates: -1, MaxResults: 1, MaxNonzeros: 1}} {
		failure(db, nil, SparseSearchOptions{Limits: limits}, vector.ErrOptions)
	}
	failure(db, nil, SparseSearchOptions{Limit: -1}, vector.ErrOptions)
	failure(db, nil, SparseSearchOptions{Limit: 2, Limits: SparseSearchLimits{MaxCandidates: 3, MaxResults: 1, MaxNonzeros: 1}}, vector.ErrLimit)
	// Entry-count work preflight happens before a malformed text parse.
	failure(db, strings.Repeat(":", 5), SparseSearchOptions{Limits: SparseSearchLimits{MaxCandidates: 3, MaxResults: 1, MaxNonzeros: 4}}, vector.ErrLimit)
	entries := make([]vector.SparseEntry, vector.MaxSparseNonzeros)
	for i := range entries {
		entries[i] = vector.SparseEntry{Index: uint32(i), Value: 1}
	}
	maxSupport, e := vector.NewSparse(1000000000, entries)
	if e != nil {
		t.Fatal(e)
	}
	for _, query := range []any{maxSupport, &maxSupport, vector.NullSparseVector{SparseVector: maxSupport, Valid: true}, &vector.NullSparseVector{SparseVector: maxSupport, Valid: true}} {
		failure(db, query, SparseSearchOptions{Limits: SparseSearchLimits{MaxCandidates: 3, MaxResults: 1, MaxNonzeros: 15999}}, vector.ErrLimit)
	}
	for _, query := range []any{(*vector.SparseVector)(nil), (*vector.NullSparseVector)(nil), vector.NullSparseVector{}, []byte(nil)} {
		got, e := db.SparseSearch(ctx, "sparse_preflight", "s", query, SparseSearchOptions{Limit: 1})
		if e != nil || got.Plan.Access != "EMPTY" || got.Stats.BaseRowFetches != 0 || len(got.Hits) != 0 {
			t.Fatal(got, e)
		}
	}
	for _, field := range []string{"d", "h", "b", "id", "rowid"} {
		r, e := db.SparseSearch(ctx, "sparse_preflight", field, nil, SparseSearchOptions{})
		if !errors.Is(e, ErrMismatch) || !reflect.DeepEqual(r, SparseResult{}) {
			t.Fatal(field, r, e)
		}
	}
	// Zero limit must still reject zero cosine query, unlike lazy SQL LIMIT 0.
	failure(db, "{} /1000000000", SparseSearchOptions{Metric: vector.Cosine}, vector.ErrZeroNorm)
	failure(db, "{1000000000:1}/1000000000", SparseSearchOptions{Metric: vector.Cosine, Limit: 1}, vector.ErrZeroNorm)
	got, e := db.SparseSearch(ctx, "sparse_preflight", "s", "{1000000000:1}/1000000000", SparseSearchOptions{Metric: vector.Cosine, Limit: 1, Where: "active=1"})
	if e != nil || len(got.Hits) != 1 || got.Hits[0].Distance != 0 || got.Stats.FilteredRows != 2 {
		t.Fatal(got, e)
	}
	for _, where := range []string{"0", "active=0"} {
		failure(db, "{} /1000000000", SparseSearchOptions{Limit: 1, Where: where, Limits: SparseSearchLimits{MaxCandidates: 2, MaxResults: 1, MaxNonzeros: 1}}, vector.ErrLimit)
	}
	if e = db.Close(); e != nil {
		t.Fatal(e)
	}
	small, e := Open(path, Options{MaxSortBytes: 512})
	if e != nil {
		t.Fatal(e)
	}
	defer small.Close()
	// A billion-dimensional empty value fits the same scratch as dimension 1.
	got, e = small.SparseSearch(ctx, "sparse_preflight", "s", "{} /1000000000", SparseSearchOptions{Limit: 1, Where: "active=0"})
	if e != nil || len(got.Hits) != 1 || got.Hits[0].RowID != 2 || got.Stats.ScoredNonzeros != 0 {
		t.Fatal(got, e)
	}
	for _, query := range []any{maxSupport, &maxSupport, vector.NullSparseVector{SparseVector: maxSupport, Valid: true}, &vector.NullSparseVector{SparseVector: maxSupport, Valid: true}} {
		failure(small, query, SparseSearchOptions{}, ErrMemoryLimit)
	}
	failure(small, strings.Repeat(":", 20), SparseSearchOptions{}, ErrMemoryLimit)
}

func TestSparseReviewCombinedScratchPreflightBeforeDecoding(t *testing.T) {
	// A maximum-support candidate fits the standalone scratch budget, but not
	// beside already retained rows. Reject before query/candidate decoding.
	entries := make([]vector.SparseEntry, vector.MaxSparseNonzeros)
	for i := range entries {
		entries[i] = vector.SparseEntry{Index: uint32(i * 60000), Value: 1}
	}
	candidate, e := vector.NewSparse(1000000000, entries)
	if e != nil {
		t.Fatal(e)
	}
	encoded, e := candidate.MarshalBinary()
	if e != nil {
		t.Fatal(e)
	}
	r := sparseReviewMemoryRows(t, encoded, "{} /1000000000", true)
	r.db.opts.MaxSortBytes = 600000
	r.vectorRank.sparseRetainedBytes = 88000
	var got any
	var scoreErr error
	var input any = encoded // Box the caller-owned slice before allocation measurement.
	allocations := testing.AllocsPerRun(20, func() { got, scoreErr = r.vectorRank.scoreSparse(r, input) })
	if !errors.Is(scoreErr, ErrMemoryLimit) || got != nil || allocations != 0 || r.vectorRank.prepared || r.queryStats.VectorRankNonzeros != 0 {
		t.Fatal(got, scoreErr, allocations, r.queryStats)
	}
	// Exact remaining-scratch boundary succeeds. Its storage is support-sized.
	r.vectorRank.sparseRetainedBytes = 600000 - sparseScratchBytes(0, len(entries))
	got, scoreErr = r.vectorRank.scoreSparse(r, encoded)
	if scoreErr != nil || got != math.Sqrt(float64(vector.MaxSparseNonzeros)) || r.queryStats.VectorRankNonzeros != 16000 {
		t.Fatal(got, scoreErr, r.queryStats)
	}
}
