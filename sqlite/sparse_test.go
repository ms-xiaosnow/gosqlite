package sqlite

import (
	"bytes"
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"math"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

func mustSparseSQL(t *testing.T, s string) vector.SparseVector {
	t.Helper()
	v, e := vector.ParseSparse(s)
	if e != nil {
		t.Fatal(e)
	}
	return v
}

func TestSparseStorageOrdinaryLifecycle(t *testing.T) {
	ctx := context.Background()
	for _, wal := range []bool{false, true} {
		for _, page := range []uint32{512, 4096, 65536} {
			t.Run(fmt.Sprintf("wal%t/page%d", wal, page), func(t *testing.T) {
				opts := Options{WALShared: wal, PageSize: page}
				db, path := reviewVectorDB(t, opts)
				reviewVectorExec(t, db, "CREATE TABLE sparse_items(id INTEGER PRIMARY KEY,s SPARSEVEC(1000000000) DEFAULT '{1000000000:-2,1:3,9:-0}/1000000000',required SPARSEVEC(1000000000) NOT NULL DEFAULT '{} /1000000000')")
				v := mustSparseSQL(t, "{1:3,1000000000:-2}/1000000000")
				want, _ := v.MarshalBinary()
				reviewVectorExec(t, db, "INSERT INTO sparse_items DEFAULT VALUES")
				reviewVectorExec(t, db, "INSERT INTO sparse_items(id,s) VALUES(2,?),(3,?),(4,?)", v, vector.NullSparseVector{}, []byte(nil))
				verify := func(db *DB) {
					rows, _, _ := reviewVectorQuery(t, db, "SELECT id,s,typeof(s),sparsevec_to_text(s) FROM sparse_items ORDER BY id")
					if len(rows) != 4 {
						t.Fatal(rows)
					}
					for i, r := range rows {
						if i < 2 {
							if !bytes.Equal(r[1].([]byte), want) || r[2] != "blob" || r[3] != v.String() {
								t.Fatal(r)
							}
						} else if r[1] != nil || r[2] != "null" || r[3] != nil {
							t.Fatal(r)
						}
					}
				}
				verify(db)
				tx, e := db.Begin(ctx)
				if e != nil {
					t.Fatal(e)
				}
				for _, q := range []string{"SAVEPOINT s", "INSERT INTO sparse_items(id,s) VALUES(5,'{3:4}/1000000000')", "SAVEPOINT nested", "UPDATE sparse_items SET s='{9:8}/1000000000' WHERE id=1", "ROLLBACK TO nested", "RELEASE nested"} {
					if _, e = tx.Exec(ctx, q); e != nil {
						t.Fatal(q, e)
					}
				}
				if _, e = tx.Exec(ctx, "INSERT INTO sparse_items(id,s) VALUES(6,'{3:4}/1000000000'),(7,'{1:1}/2')"); !errors.Is(e, vector.ErrDimensions) {
					t.Fatal(e)
				}
				got, e := tx.SparseSearch(ctx, "sparse_items", "s", "{3:4}/1000000000", SparseSearchOptions{Limit: 3})
				if e != nil || len(got.Hits) != 3 || got.Hits[0].RowID != 5 {
					t.Fatal(got, e)
				}
				for _, q := range []string{"ROLLBACK TO s", "RELEASE s"} {
					if _, e = tx.Exec(ctx, q); e != nil {
						t.Fatal(e)
					}
				}
				if e = tx.Commit(ctx); e != nil {
					t.Fatal(e)
				}
				verify(db)
				for _, q := range []string{"UPDATE sparse_items SET required=NULL WHERE id=1", "UPDATE sparse_items SET s=1 WHERE id=1", "UPDATE sparse_items SET s='{1:NaN}/1000000000' WHERE id=1", "CREATE TABLE bad(s SPARSEVEC(2) DEFAULT '{1:1}/3')"} {
					if _, e = db.Exec(ctx, q); e == nil {
						t.Fatal(q)
					}
				}
				// Ordinary successful update, rowid change and delete preserve GSS encoding.
				reviewVectorExec(t, db, "UPDATE sparse_items SET id=8,s='{2:7}/1000000000' WHERE id=2")
				reviewVectorExec(t, db, "DELETE FROM sparse_items WHERE id=8")
				reviewVectorExec(t, db, "INSERT INTO sparse_items(id,s) VALUES(2,?)", v)
				if wal {
					if e = db.Checkpoint(ctx); e != nil {
						t.Fatal(e)
					}
				}
				if e = db.Close(); e != nil {
					t.Fatal(e)
				}
				next, e := Open(path, opts)
				if e != nil {
					t.Fatal(e)
				}
				defer next.Close()
				verify(next)
			})
		}
	}
}

// Independent dictionary-union reference. No production metric or dense
// dimension-sized slice participates, including the billion-dimensional case.
func sparseReference(a, b map[uint32]float64, m vector.MetricKind) float64 {
	keys := map[uint32]bool{}
	for k := range a {
		keys[k] = true
	}
	for k := range b {
		keys[k] = true
	}
	var sum, aa, bb float64
	for k := range keys {
		x, y := a[k], b[k]
		switch m {
		case vector.L2:
			sum += (x - y) * (x - y)
		case vector.L1:
			sum += math.Abs(x - y)
		case vector.NegativeIP:
			sum += x * y
		case vector.Cosine:
			sum += x * y
			aa += x * x
			bb += y * y
		}
	}
	switch m {
	case vector.L2:
		return math.Sqrt(sum)
	case vector.L1:
		return sum
	case vector.NegativeIP:
		return -sum
	default:
		return 1 - math.Max(-1, math.Min(1, sum/math.Sqrt(aa*bb)))
	}
}
func TestSparseIndependentNativeHeapAndFullSort(t *testing.T) {
	db, _ := reviewVectorDB(t, Options{})
	reviewVectorExec(t, db, "CREATE TABLE sparse_items(id INTEGER PRIMARY KEY,s SPARSEVEC(1000000000),active INTEGER)")
	type item struct {
		id     int64
		v      map[uint32]float64
		active int
	}
	q := map[uint32]float64{0: 3, 999999999: -2}
	query := mustSparseSQL(t, "{1:3,1000000000:-2}/1000000000")
	items := []item{{math.MinInt64, q, 1}, {math.MaxInt64, q, 1}, {-100, nil, 1}, {100, nil, 0}}
	for i := 0; i < 40; i++ {
		items = append(items, item{int64(i), map[uint32]float64{uint32(i % 5): float64(i%7 + 1), 999999999: float64(-1 - i%9)}, i % 3})
	}
	tx, e := db.Begin(context.Background())
	if e != nil {
		t.Fatal(e)
	}
	for _, it := range items {
		var val any
		if it.v != nil {
			var entries []vector.SparseEntry
			for k, v := range it.v {
				entries = append(entries, vector.SparseEntry{Index: k, Value: float32(v)})
			}
			val, e = vector.NewSparse(1000000000, entries)
			if e != nil {
				t.Fatal(e)
			}
		}
		if _, e = tx.Exec(context.Background(), "INSERT INTO sparse_items VALUES(?,?,?)", it.id, val, it.active); e != nil {
			t.Fatal(e)
		}
	}
	if e = tx.Commit(context.Background()); e != nil {
		t.Fatal(e)
	}
	for _, metric := range []vector.MetricKind{vector.L2, vector.L1, vector.NegativeIP, vector.Cosine} {
		for _, filter := range []bool{false, true} {
			where := "1"
			if filter {
				where = "active=1"
			}
			var want []vector.Hit
			var nulls []int64
			var filtered, nonzeros int64
			nonzeros = 2
			for _, it := range items {
				if filter && it.active != 1 {
					filtered++
					continue
				}
				if it.v == nil {
					nulls = append(nulls, it.id)
					continue
				}
				nonzeros += int64(2 + len(it.v))
				want = append(want, vector.Hit{RowID: it.id, Distance: sparseReference(it.v, q, metric)})
			}
			sort.Slice(want, func(i, j int) bool {
				return want[i].Distance < want[j].Distance || want[i].Distance == want[j].Distance && want[i].RowID < want[j].RowID
			})
			sort.Slice(nulls, func(i, j int) bool { return nulls[i] < nulls[j] })
			got, e := db.SparseSearch(context.Background(), "sparse_items", "s", query, SparseSearchOptions{Metric: metric, Limit: 7, Where: where})
			if e != nil || len(got.Hits) != 7 || got.Stats.ScoredNonzeros != nonzeros || got.Stats.FilteredRows != filtered || got.Stats.NullRows != int64(len(nulls)) || got.Plan.Method != "exact_sparsevec" || !got.Plan.Exact || got.Plan.VectorRepresentation != "" {
				t.Fatal(got, e)
			}
			for i, h := range got.Hits {
				if h.RowID != want[i].RowID || math.Abs(h.Distance-want[i].Distance) > 1e-12 {
					t.Fatal(got.Hits, want)
				}
			}
			op := map[vector.MetricKind]string{vector.L2: "<->", vector.L1: "<+>", vector.NegativeIP: "<#>", vector.Cosine: "<=>"}[metric]
			for _, expr := range []string{"s " + op + " ?", "? " + op + " s"} {
				for _, window := range [][2]int{{1, 0}, {7, 2}, {100, 0}, {3, 100}} {
					suffix := fmt.Sprintf(" LIMIT %d OFFSET %d", window[0], window[1])
					base := "SELECT id," + expr + " AS distance FROM sparse_items WHERE " + where + " ORDER BY distance"
					rows, p, stats := reviewVectorQuery(t, db, base+suffix, query)
					full, fp, _ := reviewVectorQuery(t, db, base+",id"+suffix, query)
					if !reflect.DeepEqual(rows, full) || p.VectorRepresentation != "sparsevec" || p.VectorMetric != metric || p.VectorRanking != "bounded_heap" || !p.VectorExact || p.ANNFallback != "SPARSEVEC(n) ANN is unsupported; exact sparse ranking" || fp.VectorRanking != "" || stats.VectorRankComponents != 0 || stats.VectorRankPackedBytes != 0 || stats.VectorRankNonzeros != nonzeros {
						t.Fatal(rows, full, p, stats)
					}
					var all [][]any
					for _, id := range nulls {
						all = append(all, []any{id, nil})
					}
					for _, h := range want {
						all = append(all, []any{h.RowID, h.Distance})
					}
					start := min(window[1], len(all))
					expected := all[start:min(start+window[0], len(all))]
					if len(rows) != len(expected) {
						t.Fatal(rows, expected)
					}
					for i, r := range rows {
						if r[0] != expected[i][0] {
							t.Fatal(rows, expected)
						}
						if expected[i][1] != nil && math.Abs(r[1].(float64)-expected[i][1].(float64)) > 1e-12 {
							t.Fatal(rows, expected)
						}
					}
				}
			}
		}
	}
	if _, e := db.VectorSearch(context.Background(), "sparse_items", "s", query, VectorSearchOptions{Limit: 1}); !errors.Is(e, ErrMismatch) {
		t.Fatal(e)
	}
	if e := db.CreateHNSWIndex(context.Background(), "sh", "sparse_items", "s"); !errors.Is(e, ErrMismatch) {
		t.Fatal(e)
	}
	if e := db.CreateIVFFlatIndex(context.Background(), "si", "sparse_items", "s"); !errors.Is(e, ErrMismatch) {
		t.Fatal(e)
	}
}

func TestSparseQuotasNullCancellationAndSQLLaziness(t *testing.T) {
	db, path := reviewVectorDB(t, Options{})
	ctx := context.Background()
	reviewVectorExec(t, db, "CREATE TABLE sparse_items(id INTEGER PRIMARY KEY,s SPARSEVEC(1000000000),active INTEGER)")
	reviewVectorExec(t, db, "INSERT INTO sparse_items VALUES(1,'{1:1,1000000000:2}/1000000000',1),(2,'{3:4}/1000000000',1),(3,NULL,0)")
	query := "{1:1}/1000000000"
	fail := func(q any, o SparseSearchOptions, want error) {
		t.Helper()
		got, e := db.SparseSearch(ctx, "sparse_items", "s", q, o)
		if !errors.Is(e, want) || !reflect.DeepEqual(got, SparseResult{}) {
			t.Fatal(got, e, want)
		}
	}
	fail(query, SparseSearchOptions{Limit: -1}, vector.ErrOptions)
	fail(query, SparseSearchOptions{Metric: "hamming"}, vector.ErrMetric)
	fail(query, SparseSearchOptions{Limits: SparseSearchLimits{MaxCandidates: 1}}, vector.ErrOptions)
	fail(query, SparseSearchOptions{Limit: 2, Limits: SparseSearchLimits{MaxCandidates: 3, MaxResults: 2, MaxNonzeros: 5}}, vector.ErrLimit)
	fail(query, SparseSearchOptions{Limit: 2, Limits: SparseSearchLimits{MaxCandidates: 2, MaxResults: 2, MaxNonzeros: 6}}, vector.ErrLimit)
	fail("{} /1000000000", SparseSearchOptions{Metric: vector.Cosine}, vector.ErrZeroNorm)
	fail("{1:1}/2", SparseSearchOptions{}, vector.ErrDimensions)
	fail(vector.SparseVector{}, SparseSearchOptions{}, vector.ErrInvalid)
	fail(strings.Repeat(" ", vector.MaxTextBytes+1), SparseSearchOptions{}, vector.ErrLimit)
	got, e := db.SparseSearch(ctx, "sparse_items", "s", query, SparseSearchOptions{Limit: 2, Limits: SparseSearchLimits{MaxCandidates: 3, MaxResults: 2, MaxNonzeros: 6}})
	if e != nil || len(got.Hits) != 2 || got.Stats.ScoredNonzeros != 6 {
		t.Fatal(got, e)
	}
	for _, q := range []any{nil, vector.NullSparseVector{}, []byte(nil)} {
		got, e := db.SparseSearch(ctx, "sparse_items", "s", q, SparseSearchOptions{Limit: 1})
		if e != nil || got.Plan.Access != "EMPTY" || got.Stats.BaseRowFetches != 0 {
			t.Fatal(got, e)
		}
	}
	got, e = db.SparseSearch(ctx, "sparse_items", "s", query, SparseSearchOptions{})
	if e != nil || got.Stats.BaseRowFetches != 0 || got.Stats.ScoredNonzeros != 1 {
		t.Fatal(got, e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if got, e := db.SparseSearch(canceled, "sparse_items", "s", nil, SparseSearchOptions{}); !errors.Is(e, context.Canceled) || !reflect.DeepEqual(got, SparseResult{}) {
		t.Fatal(got, e)
	}
	if _, e := db.SparseSearch(nil, "sparse_items", "s", nil, SparseSearchOptions{}); e == nil {
		t.Fatal("nil context")
	}
	rows, p, s := reviewVectorQuery(t, db, "SELECT id FROM sparse_items WHERE active=0 ORDER BY s <-> 'bad' LIMIT 1")
	if len(rows) != 1 || p.VectorRepresentation != "sparsevec" || s.VectorRankNonzeros != 0 {
		t.Fatal(rows, p, s)
	}
	rows, _, s = reviewVectorQuery(t, db, "SELECT id FROM sparse_items ORDER BY s <-> 'bad' LIMIT 0")
	if len(rows) != 0 || s.BaseRowFetches != 0 {
		t.Fatal(rows, s)
	}
	r, e := db.Query(ctx, "EXPLAIN QUERY PLAN SELECT id FROM sparse_items ORDER BY s <-> 'bad' LIMIT 1")
	if e != nil {
		t.Fatal(e)
	}
	r.Close()
	if r, e = db.Query(ctx, "SELECT id FROM sparse_items ORDER BY s <-> 'bad' LIMIT 1"); e == nil {
		r.Close()
		t.Fatal("bad query accepted")
	}
	if e = db.Close(); e != nil {
		t.Fatal(e)
	}
	small, e := Open(path, Options{MaxSortBytes: 300})
	if e != nil {
		t.Fatal(e)
	}
	defer small.Close()
	if got, e = small.SparseSearch(ctx, "sparse_items", "s", query, SparseSearchOptions{Limit: 2}); !errors.Is(e, ErrMemoryLimit) || !reflect.DeepEqual(got, SparseResult{}) {
		t.Fatal(got, e)
	}
	if r, e = small.Query(ctx, "SELECT id FROM sparse_items ORDER BY s <-> ? LIMIT 2", query); !errors.Is(e, ErrMemoryLimit) {
		if r != nil {
			r.Close()
		}
		t.Fatal(e)
	}
}

func TestSparseSharedSnapshotAndMaximumSupport(t *testing.T) {
	ctx := context.Background()
	db, path := reviewVectorDB(t, Options{WALShared: true, PageSize: 512})
	reviewVectorExec(t, db, "CREATE TABLE sparse_items(id INTEGER PRIMARY KEY,s SPARSEVEC(1000000000))")
	entries := make([]vector.SparseEntry, vector.MaxSparseNonzeros)
	for i := range entries {
		entries[i] = vector.SparseEntry{Index: uint32(i * 60000), Value: float32(i%7 + 1)}
	}
	v, e := vector.NewSparse(1000000000, entries)
	if e != nil {
		t.Fatal(e)
	}
	reviewVectorExec(t, db, "INSERT INTO sparse_items VALUES(1,?)", v)
	tx, e := db.Begin(ctx, TxOptions{ReadOnly: true})
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	first, e := tx.SparseSearch(ctx, "sparse_items", "s", v, SparseSearchOptions{Limit: 1})
	if e != nil || first.Hits[0].Distance != 0 || first.Stats.ScoredNonzeros != 48000 {
		t.Fatal(first, e)
	}
	writer, e := Open(path, Options{WALShared: true})
	if e != nil {
		t.Fatal(e)
	}
	defer writer.Close()
	reviewVectorExec(t, writer, "UPDATE sparse_items SET s='{1:1}/1000000000' WHERE id=1")
	for _, search := range []func(context.Context, string, string, any, SparseSearchOptions) (SparseResult, error){tx.SparseSearch, db.SparseSearch} {
		got, e := search(ctx, "sparse_items", "s", v, SparseSearchOptions{Limit: 1})
		if e != nil || !reflect.DeepEqual(first.Hits, got.Hits) {
			t.Fatal(got, e)
		}
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	fresh, e := db.SparseSearch(ctx, "sparse_items", "s", v, SparseSearchOptions{Limit: 1})
	if e != nil || fresh.Hits[0].Distance == 0 || fresh.Stats.ScoredNonzeros != 32001 {
		t.Fatal(fresh, e)
	}
	if e = writer.Checkpoint(ctx); e != nil {
		t.Fatal(e)
	}
}

type sparseCancelValuer struct{ cancel context.CancelFunc }

func (v sparseCancelValuer) Value() (driver.Value, error) { v.cancel(); return "{1:1}/1000000000", nil }
func TestSparseCallbackCancellationAndMixedKinds(t *testing.T) {
	db, _ := reviewVectorDB(t, Options{})
	reviewVectorExec(t, db, "CREATE TABLE sparse_items(id INTEGER PRIMARY KEY,s SPARSEVEC(3),v VECTOR(3),h HALFVEC(3),b BIT(3))")
	reviewVectorExec(t, db, "INSERT INTO sparse_items VALUES(1,'{1:1}/3','[1,0,0]','[1,0,0]','100')")
	for _, q := range []string{"UPDATE sparse_items SET s=v", "UPDATE sparse_items SET v=s", "UPDATE sparse_items SET h=s", "UPDATE sparse_items SET s=h", "UPDATE sparse_items SET s=b"} {
		if _, e := db.Exec(context.Background(), q); e == nil {
			t.Fatal(q)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	got, e := db.SparseSearch(ctx, "sparse_items", "s", sparseCancelValuer{cancel}, SparseSearchOptions{})
	if !errors.Is(e, context.Canceled) || !reflect.DeepEqual(got, SparseResult{}) {
		t.Fatal(got, e)
	}
	reviewVectorExec(t, db, "UPDATE sparse_items SET s=sparsevec_from_vector(v)")
	rows, _, _ := reviewVectorQuery(t, db, "SELECT sparsevec_to_text(s),vector_to_text(vector_from_sparsevec(s)) FROM sparse_items")
	if !reflect.DeepEqual(rows, [][]any{{"{1:1}/3", "[1,0,0]"}}) {
		t.Fatal(rows)
	}
}
