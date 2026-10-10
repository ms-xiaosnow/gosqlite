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

func mustHalfSQL(t *testing.T, s string) vector.HalfVector {
	t.Helper()
	v, e := vector.ParseHalf(s)
	if e != nil {
		t.Fatal(e)
	}
	return v
}

func TestHalfStorageOrdinaryLifecycle(t *testing.T) {
	ctx := context.Background()
	for _, wal := range []bool{false, true} {
		for _, page := range []uint32{512, 4096, 65536} {
			t.Run(fmt.Sprintf("wal%t/page%d", wal, page), func(t *testing.T) {
				opts := Options{WALShared: wal, PageSize: page}
				db, path := reviewVectorDB(t, opts)
				reviewVectorExec(t, db, "CREATE TABLE halves(id INTEGER PRIMARY KEY,h HALFVEC(3) DEFAULT '[1.0001,-0,0.000000059604645]',required HALFVEC(3) NOT NULL DEFAULT '[0,0,0]')")
				reviewVectorExec(t, db, "INSERT INTO halves DEFAULT VALUES")
				reviewVectorExec(t, db, "INSERT INTO halves(id,h) VALUES(2,?),(3,?),(4,?)", mustHalfSQL(t, "[1.0001,-0,0.000000059604645]"), vector.NullHalfVector{}, []byte(nil))
				want := []byte{'G', 'S', 'H', 0, 1, 2, 1, 0, 0, 0, 0, 3, 0x3c, 0, 0x80, 0, 0, 1}
				verify := func(db *DB) {
					rows, _, _ := reviewVectorQuery(t, db, "SELECT id,h,typeof(h),halfvec_to_text(h) FROM halves ORDER BY id")
					if len(rows) != 4 {
						t.Fatal(rows)
					}
					for i, r := range rows {
						if i < 2 {
							if !bytes.Equal(r[1].([]byte), want) || r[2] != "blob" || r[3] != "[1,-0,5.9604645e-08]" {
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
				for _, q := range []string{"SAVEPOINT h", "INSERT INTO halves(id,h) VALUES(5,'[4,5,6]')", "SAVEPOINT nested", "UPDATE halves SET h='[7,8,9]' WHERE id=1", "ROLLBACK TO nested", "RELEASE nested"} {
					if _, e = tx.Exec(ctx, q); e != nil {
						t.Fatal(q, e)
					}
				}
				if _, e = tx.Exec(ctx, "INSERT INTO halves(id,h) VALUES(6,'[4,5,6]'),(7,'[1,2]')"); !errors.Is(e, vector.ErrDimensions) {
					t.Fatal(e)
				}
				got, e := tx.HalfSearch(ctx, "halves", "h", "[4,5,6]", HalfSearchOptions{Limit: 3})
				if e != nil || len(got.Hits) != 3 || got.Hits[0].RowID != 5 {
					t.Fatal(got, e)
				}
				if _, e = tx.Exec(ctx, "ROLLBACK TO h"); e != nil {
					t.Fatal(e)
				}
				if _, e = tx.Exec(ctx, "RELEASE h"); e != nil {
					t.Fatal(e)
				}
				if e = tx.Commit(ctx); e != nil {
					t.Fatal(e)
				}
				verify(db)
				for _, q := range []string{"UPDATE halves SET required=NULL WHERE id=1", "UPDATE halves SET h=1 WHERE id=1", "UPDATE halves SET h='[65520,1,2]' WHERE id=1", "CREATE TABLE bad(h HALFVEC(1) DEFAULT '[1,2]')"} {
					if _, e = db.Exec(ctx, q); e == nil {
						t.Fatal(q)
					}
				}
				if wal {
					if e = db.Checkpoint(ctx); e != nil {
						t.Fatal(e)
					}
				}
				if e = db.Close(); e != nil {
					t.Fatal(e)
				}
				db2, e := Open(path, opts)
				if e != nil {
					t.Fatal(e)
				}
				defer db2.Close()
				verify(db2)
			})
		}
	}
}

func TestHalfSQLNativeIndependentRanking(t *testing.T) {
	db, _ := reviewVectorDB(t, Options{})
	reviewVectorExec(t, db, "CREATE TABLE halves(id INTEGER PRIMARY KEY,h HALFVEC(3),active INTEGER)")
	type item struct {
		id     int64
		v      []float32
		active int
	}
	items := []item{{math.MinInt64, []float32{1.0001, 2.9999, -2.0003}, 1}, {math.MaxInt64, []float32{1.0001, 2.9999, -2.0003}, 1}, {-100, nil, 1}, {100, nil, 0}}
	for i := 0; i < 48; i++ {
		items = append(items, item{int64(i), []float32{float32(i%5) + 0.0001, float32(i%7) - 0.1234, float32(i%11) - 5.0001}, i % 3})
	}
	for _, i := range items {
		var v any
		if i.v != nil {
			h, e := vector.NewHalf(i.v)
			if e != nil {
				t.Fatal(e)
			}
			v = h
		}
		reviewVectorExec(t, db, "INSERT INTO halves VALUES(?,?,?)", i.id, v, i.active)
	}
	query := mustHalfSQL(t, "[1.0001,2.9999,-2.0003]")
	for _, metric := range []vector.MetricKind{vector.L2, vector.L1, vector.NegativeIP, vector.Cosine} {
		var want []vector.Hit
		for _, i := range items {
			if i.active != 1 || i.v == nil {
				continue
			}
			h, _ := vector.NewHalf(i.v)
			want = append(want, vector.Hit{RowID: i.id, Distance: reviewVectorDistance(h.Values(), query.Values(), metric)})
		}
		sort.Slice(want, func(i, j int) bool { return bitHitBetter(want[i], want[j]) })
		got, e := db.HalfSearch(context.Background(), "halves", "h", query, HalfSearchOptions{Limit: 7, Metric: metric, Where: "active=?", Args: []any{1}})
		if e != nil {
			t.Fatal(e)
		}
		if len(got.Hits) != 7 || got.Plan.Method != "exact_halfvec" || !got.Plan.Exact || got.Stats.ScoredComponents != 3*(1+int64(len(want))) {
			t.Fatal(got)
		}
		for i, h := range got.Hits {
			if h.RowID != want[i].RowID || math.Abs(h.Distance-want[i].Distance) > 1e-12 {
				t.Fatal(got.Hits, want)
			}
		}
		op := map[vector.MetricKind]string{vector.L2: "<->", vector.L1: "<+>", vector.NegativeIP: "<#>", vector.Cosine: "<=>"}[metric]
		for _, expr := range []string{"h " + op + " ?", "? " + op + " h"} {
			rows, p, s := reviewVectorQuery(t, db, "SELECT id,"+expr+" AS distance FROM halves WHERE active=1 ORDER BY distance LIMIT 8", query)
			if len(rows) != 8 || rows[0][0] != int64(-100) || rows[0][1] != nil || p.VectorRepresentation != "halfvec" || p.VectorMetric != metric || p.VectorRanking != "bounded_heap" || !p.VectorExact || p.ANNFallback != "SQL NULL-first ordering requires a declared NOT NULL vector column" || s.VectorRankComponents != got.Stats.ScoredComponents || s.VectorRankPackedBytes != 0 {
				t.Fatal(rows, p, s)
			}
			for i, r := range rows[1:] {
				if r[0] != want[i].RowID || math.Abs(r[1].(float64)-want[i].Distance) > 1e-12 {
					t.Fatal(rows, want)
				}
			}
		}
	}
	if _, e := db.VectorSearch(context.Background(), "halves", "h", query, VectorSearchOptions{Limit: 1}); !errors.Is(e, ErrMismatch) {
		t.Fatal(e)
	}
	if e := db.CreateHNSWIndex(context.Background(), "hh", "halves", "h"); e != nil {
		t.Fatal(e)
	}
	if e := db.CreateIVFFlatIndex(context.Background(), "hi", "halves", "h"); e != nil {
		t.Fatal(e)
	}
}

func TestHalfNativeQuotasNullCancellationAndSQLLaziness(t *testing.T) {
	db, _ := reviewVectorDB(t, Options{})
	ctx := context.Background()
	reviewVectorExec(t, db, "CREATE TABLE halves(id INTEGER PRIMARY KEY,h HALFVEC(2),active INTEGER)")
	reviewVectorExec(t, db, "INSERT INTO halves VALUES(1,'[1,2]',1),(2,'[3,4]',1),(3,NULL,0)")
	fail := func(q any, o HalfSearchOptions, want error) {
		t.Helper()
		got, e := db.HalfSearch(ctx, "halves", "h", q, o)
		if !errors.Is(e, want) || !reflect.DeepEqual(got, HalfResult{}) {
			t.Fatal(got, e, want)
		}
	}
	fail("[1,2]", HalfSearchOptions{Limit: -1}, vector.ErrOptions)
	fail("[1,2]", HalfSearchOptions{Limit: 1, Metric: "hamming"}, vector.ErrMetric)
	fail("[1,2]", HalfSearchOptions{Limit: 1, Limits: HalfSearchLimits{MaxCandidates: 1}}, vector.ErrOptions)
	fail("[1,2]", HalfSearchOptions{Limit: 1, Limits: HalfSearchLimits{MaxCandidates: 2, MaxResults: 1, MaxComponents: 6}}, vector.ErrLimit)
	fail("[1,2]", HalfSearchOptions{Limit: 1, Limits: HalfSearchLimits{MaxCandidates: 3, MaxResults: 1, MaxComponents: 5}}, vector.ErrLimit)
	fail("[1,2]", HalfSearchOptions{Limit: 0, Limits: HalfSearchLimits{MaxCandidates: 3, MaxResults: 1, MaxComponents: 1}}, vector.ErrLimit)
	fail("[0,0]", HalfSearchOptions{Metric: vector.Cosine}, vector.ErrZeroNorm)
	fail("[1]", HalfSearchOptions{}, vector.ErrDimensions)
	fail(vector.HalfVector{}, HalfSearchOptions{}, vector.ErrInvalid)
	fail(strings.Repeat(" ", vector.MaxTextBytes+1), HalfSearchOptions{}, vector.ErrLimit)
	for _, q := range []any{nil, vector.NullHalfVector{}, []byte(nil)} {
		got, e := db.HalfSearch(ctx, "halves", "h", q, HalfSearchOptions{Limit: 1})
		if e != nil || got.Plan.Access != "EMPTY" || len(got.Hits) != 0 || got.Stats.BaseRowFetches != 0 {
			t.Fatal(got, e)
		}
	}
	got, e := db.HalfSearch(ctx, "halves", "h", "[1,2]", HalfSearchOptions{Limit: 2, Limits: HalfSearchLimits{MaxCandidates: 3, MaxResults: 2, MaxComponents: 6}})
	if e != nil || len(got.Hits) != 2 || got.Stats.ScoredComponents != 6 {
		t.Fatal(got, e)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if got, e := db.HalfSearch(canceled, "halves", "h", nil, HalfSearchOptions{}); !errors.Is(e, context.Canceled) || !reflect.DeepEqual(got, HalfResult{}) {
		t.Fatal(got, e)
	}
	if _, e := db.HalfSearch(nil, "halves", "h", nil, HalfSearchOptions{}); e == nil {
		t.Fatal("nil context")
	}
	rows, p, s := reviewVectorQuery(t, db, "SELECT id FROM halves WHERE active=0 ORDER BY h <-> 'bad' LIMIT 1")
	if len(rows) != 1 || p.VectorRepresentation != "halfvec" || s.VectorRankComponents != 0 {
		t.Fatal(rows, p, s)
	}
	rows, _, s = reviewVectorQuery(t, db, "SELECT id FROM halves ORDER BY h <-> 'bad' LIMIT 0")
	if len(rows) != 0 || s.BaseRowFetches != 0 {
		t.Fatal(rows, s)
	}
	r, e := db.Query(ctx, "EXPLAIN QUERY PLAN SELECT id FROM halves ORDER BY h <-> 'bad' LIMIT 1")
	if e != nil {
		t.Fatal(e)
	}
	r.Close()
	if r, e = db.Query(ctx, "SELECT id FROM halves ORDER BY h <-> 'bad' LIMIT 1"); e == nil {
		r.Close()
		t.Fatal("invalid query accepted")
	}
	r, e = db.Query(ctx, "SELECT h FROM halves")
	if e != nil {
		t.Fatal(e)
	}
	defer r.Close()
	rank := &vectorRank{half: true, dimensions: 2, query: "[1,2]", metric: vector.L2, limits: vector.SearchLimits{MaxComponents: 3}}
	if _, e = rank.scoreHalf(r, "[3,4]"); !errors.Is(e, vector.ErrLimit) {
		t.Fatal(e)
	}
}

func TestHalfSharedSnapshotAndExplicitMaintenance(t *testing.T) {
	db, path := reviewVectorDB(t, Options{WALShared: true})
	ctx := context.Background()
	reviewVectorExec(t, db, "CREATE TABLE halves(id INTEGER PRIMARY KEY,h HALFVEC(2),v VECTOR(2),active INTEGER)")
	reviewVectorExec(t, db, "INSERT INTO halves VALUES(1,halfvec_from_vector('[1.0001,2]'),'[1.0001,2]',1)")
	tx, e := db.Begin(ctx, TxOptions{ReadOnly: true})
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback()
	first, e := tx.HalfSearch(ctx, "halves", "h", "[1,2]", HalfSearchOptions{Limit: 1})
	if e != nil || first.Hits[0].Distance != 0 {
		t.Fatal(first, e)
	}
	writer, e := Open(path, Options{WALShared: true})
	if e != nil {
		t.Fatal(e)
	}
	defer writer.Close()
	reviewVectorExec(t, writer, "UPDATE halves SET v='[4,6]',h=halfvec_from_vector('[4,6]') WHERE id=1")
	pinned, e := tx.HalfSearch(ctx, "halves", "h", "[1,2]", HalfSearchOptions{Limit: 1})
	if e != nil || !reflect.DeepEqual(first.Hits, pinned.Hits) {
		t.Fatal(pinned, e)
	}
	via, e := db.HalfSearch(ctx, "halves", "h", "[1,2]", HalfSearchOptions{Limit: 1})
	if e != nil || !reflect.DeepEqual(first.Hits, via.Hits) {
		t.Fatal(via, e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	current, e := db.HalfSearch(ctx, "halves", "h", "[1,2]", HalfSearchOptions{Limit: 1})
	if e != nil || current.Hits[0].Distance != 5 {
		t.Fatal(current, e)
	}
}

func TestHalfMaximumDimensionsAndMemoryBound(t *testing.T) {
	db, path := reviewVectorDB(t, Options{PageSize: 512})
	xs := make([]float32, vector.MaxHalfDimensions)
	for i := range xs {
		xs[i] = float32(i%7) + 0.0001
	}
	h, e := vector.NewHalf(xs)
	if e != nil {
		t.Fatal(e)
	}
	reviewVectorExec(t, db, "CREATE TABLE halves(id INTEGER PRIMARY KEY,h HALFVEC(16000))")
	reviewVectorExec(t, db, "INSERT INTO halves VALUES(1,?),(2,?)", h, h.String())
	got, e := db.HalfSearch(context.Background(), "halves", "h", h, HalfSearchOptions{Limit: 2})
	if e != nil || len(got.Hits) != 2 || got.Stats.ScoredComponents != 48000 || got.Hits[0].Distance != 0 || got.Hits[1].Distance != 0 {
		t.Fatal(got, e)
	}
	want, _ := h.MarshalBinary()
	rows, _, _ := reviewVectorQuery(t, db, "SELECT h FROM halves ORDER BY id")
	for _, r := range rows {
		if len(r[0].([]byte)) != 32012 || !bytes.Equal(r[0].([]byte), want) {
			t.Fatal("binary16 representation changed")
		}
	}
	if e = db.Close(); e != nil {
		t.Fatal(e)
	}
	reopened, e := Open(path, Options{MaxSortBytes: 100})
	if e != nil {
		t.Fatal(e)
	}
	defer reopened.Close()
	if got, e = reopened.HalfSearch(context.Background(), "halves", "h", h, HalfSearchOptions{Limit: 1}); !errors.Is(e, ErrMemoryLimit) || !reflect.DeepEqual(got, HalfResult{}) {
		t.Fatal(got, e)
	}
	if r, e := reopened.Query(context.Background(), "SELECT id FROM halves ORDER BY h <-> ? LIMIT 1", h); !errors.Is(e, ErrMemoryLimit) {
		if r != nil {
			r.Close()
		}
		t.Fatal(e)
	}
}

func TestHalfSQLShapesAndMixedKinds(t *testing.T) {
	db, _ := reviewVectorDB(t, Options{})
	ctx := context.Background()
	reviewVectorExec(t, db, "CREATE TABLE halves(id INTEGER PRIMARY KEY,h HALFVEC(2),d VECTOR(2),b BIT(2))")
	reviewVectorExec(t, db, "INSERT INTO halves VALUES(1,'[1.0001,2]','[1.0001,2]','11'),(2,'[3,4]','[3,4]','11'),(3,NULL,NULL,NULL)")
	for _, q := range []string{"SELECT id,h <-> '[1,2]' AS distance FROM halves ORDER BY 2 LIMIT 1 OFFSET 1", "SELECT id FROM halves ORDER BY h <-> '[1,2]' LIMIT 1 OFFSET 1"} {
		r, p, _ := reviewVectorQuery(t, db, q)
		if len(r) != 1 || r[0][0] != int64(1) || p.VectorRanking != "bounded_heap" {
			t.Fatal(r, p)
		}
	}
	for _, q := range []string{"SELECT id FROM halves ORDER BY h <-> halfvec('[1,2]') LIMIT 2", "SELECT id FROM halves ORDER BY h <-> '[1,2]' DESC,id DESC LIMIT 2"} {
		r, p, _ := reviewVectorQuery(t, db, q)
		if len(r) != 2 || p.VectorRanking != "" {
			t.Fatal(r, p)
		}
	}
	for _, q := range []string{"SELECT h <-> d FROM halves", "SELECT d <+> h FROM halves", "UPDATE halves SET h=d WHERE id=1", "UPDATE halves SET d=h WHERE id=1", "UPDATE halves SET h=b WHERE id=1"} {
		if strings.HasPrefix(q, "SELECT") {
			r, e := db.Query(ctx, q)
			if e == nil {
				for r.Next() {
				}
				e = r.Err()
				r.Close()
			}
			if e == nil {
				t.Fatal(q)
			}
		} else if _, e := db.Exec(ctx, q); e == nil {
			t.Fatal(q)
		}
	}
	for _, field := range []string{"d", "b", "id"} {
		if _, e := db.HalfSearch(ctx, "halves", field, "[1,2]", HalfSearchOptions{Limit: 1}); !errors.Is(e, ErrMismatch) {
			t.Fatal(field, e)
		}
	}
	for _, l := range []HalfSearchLimits{{MaxCandidates: 1<<20 + 1, MaxResults: 1, MaxComponents: 1}, {MaxCandidates: 1, MaxResults: 1<<20 + 1, MaxComponents: 1}, {MaxCandidates: 1, MaxResults: 1, MaxComponents: 1<<34 + 1}} {
		if _, e := db.HalfSearch(ctx, "halves", "h", nil, HalfSearchOptions{Limits: l}); !errors.Is(e, vector.ErrOptions) {
			t.Fatal(e)
		}
	}
	if _, e := db.HalfSearch(ctx, "halves", "h", nil, HalfSearchOptions{Limit: 10001}); !errors.Is(e, vector.ErrLimit) {
		t.Fatal(e)
	}
}

type halfCancelValuer struct{ cancel context.CancelFunc }

func (v halfCancelValuer) Value() (driver.Value, error) { v.cancel(); return "[1,2]", nil }
func TestHalfQueryCallbackCancellation(t *testing.T) {
	db, _ := reviewVectorDB(t, Options{})
	reviewVectorExec(t, db, "CREATE TABLE halves(id INTEGER PRIMARY KEY,h HALFVEC(2))")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	got, e := db.HalfSearch(ctx, "halves", "h", halfCancelValuer{cancel}, HalfSearchOptions{})
	if !errors.Is(e, context.Canceled) || !reflect.DeepEqual(got, HalfResult{}) {
		t.Fatal(got, e)
	}
}
