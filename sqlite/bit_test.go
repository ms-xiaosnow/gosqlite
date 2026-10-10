package sqlite

import (
	"bytes"
	"context"
	"database/sql/driver"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

func mustBit(t *testing.T, s string) vector.BitVector {
	t.Helper()
	v, e := vector.ParseBit(s)
	if e != nil {
		t.Fatal(e)
	}
	return v
}

func TestBitStorageOrdinaryLifecycle(t *testing.T) {
	ctx := context.Background()
	for _, wal := range []bool{false, true} {
		for _, page := range []uint32{512, 4096, 65536} {
			t.Run(fmt.Sprintf("wal%t/page%d", wal, page), func(t *testing.T) {
				opts := Options{WALShared: wal, PageSize: page}
				db, path := reviewVectorDB(t, opts)
				reviewVectorExec(t, db, "CREATE TABLE bits(id INTEGER PRIMARY KEY,b BIT(9) DEFAULT '001010001',required BIT(9) NOT NULL DEFAULT '000000000')")
				reviewVectorExec(t, db, "INSERT INTO bits DEFAULT VALUES")
				reviewVectorExec(t, db, "INSERT INTO bits(id,b) VALUES(2,?),(3,?),(4,?),(5,?)", mustBit(t, "001010001"), vector.NullBitVector{}, []byte(nil), "001010001")
				want := []byte{'G', 'S', 'B', 0, 1, 3, 1, 0, 0, 0, 0, 9, 0x28, 0x80}
				verify := func(db *DB) {
					rows, _, _ := reviewVectorQuery(t, db, "SELECT id,b,typeof(b),bit_to_text(b) FROM bits ORDER BY id")
					if len(rows) != 5 {
						t.Fatal(rows)
					}
					for i, row := range rows {
						if i == 2 || i == 3 {
							if row[1] != nil || row[2] != "null" || row[3] != nil {
								t.Fatal(row)
							}
							continue
						}
						b, ok := row[1].([]byte)
						if !ok || !bytes.Equal(b, want) || row[2] != "blob" || row[3] != "001010001" {
							t.Fatal(row)
						}
					}
				}
				verify(db)
				tx, err := db.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = tx.Exec(ctx, "INSERT INTO bits(id,b) VALUES(6,'111111111')"); err != nil {
					t.Fatal(err)
				}
				if _, err = tx.Exec(ctx, "INSERT INTO bits(id,b) VALUES(7,'111111111'),(8,'1')"); !errors.Is(err, vector.ErrDimensions) {
					t.Fatal(err)
				}
				got, err := tx.BitSearch(ctx, "bits", "b", "111111111", BitSearchOptions{Limit: 10})
				if err != nil || len(got.Hits) != 4 || got.Hits[0].RowID != 6 {
					t.Fatal(got, err)
				}
				if _, err = tx.Exec(ctx, "UPDATE bits SET b='000000000' WHERE id=1"); err != nil {
					t.Fatal(err)
				}
				got, err = tx.BitSearch(ctx, "bits", "b", "000000000", BitSearchOptions{Limit: 1})
				if err != nil || got.Hits[0].RowID != 1 || got.Hits[0].Distance != 0 {
					t.Fatal(got, err)
				}
				if err = tx.Rollback(); err != nil {
					t.Fatal(err)
				}
				verify(db)
				for _, q := range []string{"UPDATE bits SET required=NULL WHERE id=1", "INSERT INTO bits(id,b) VALUES(8,'001010001'),(9,'001')", "UPDATE bits SET b=101 WHERE id=1"} {
					if _, err = db.Exec(ctx, q); err == nil {
						t.Fatal("accepted invalid typed mutation", q)
					}
				}
				if _, err = db.Exec(ctx, "CREATE TABLE bad(v BIT(3) DEFAULT '0011')"); !errors.Is(err, vector.ErrDimensions) {
					t.Fatal(err)
				}
				verify(db)
				// Ordinary DELETE rollback follows the same row/representation lifecycle.
				tx, err = db.Begin(ctx)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = tx.Exec(ctx, "DELETE FROM bits WHERE id=2"); err != nil {
					t.Fatal(err)
				}
				if err = tx.Rollback(); err != nil {
					t.Fatal(err)
				}
				if wal {
					if err = db.Checkpoint(ctx); err != nil {
						t.Fatal(err)
					}
				}
				if err = db.Close(); err != nil {
					t.Fatal(err)
				}
				reopened, err := Open(path, opts)
				if err != nil {
					t.Fatal(err)
				}
				defer reopened.Close()
				verify(reopened)
				if _, err = reopened.Exec(ctx, "UPDATE bits SET b='01' WHERE id=2"); !errors.Is(err, vector.ErrDimensions) {
					t.Fatal(err)
				}
				verify(reopened)
			})
		}
	}
}

func bitReference(a, b string, metric vector.BitMetricKind) float64 {
	diff, intersection, union := 0, 0, 0
	for i := range a {
		if a[i] != b[i] {
			diff++
		}
		if a[i] == '1' && b[i] == '1' {
			intersection++
		}
		if a[i] == '1' || b[i] == '1' {
			union++
		}
	}
	if metric == vector.Hamming {
		return float64(diff)
	}
	if intersection == 0 {
		return 1
	}
	return 1 - float64(intersection)/float64(union)
}

func TestBitSQLAndNativeIndependentRanking(t *testing.T) {
	db, _ := reviewVectorDB(t, Options{})
	reviewVectorExec(t, db, "CREATE TABLE bits(id INTEGER PRIMARY KEY,b BIT(17),active INTEGER)")
	type item struct {
		id     int64
		bit    *string
		active int
	}
	equal := "10101010101010101"
	items := []item{{math.MinInt64, &equal, 1}, {math.MaxInt64, &equal, 1}, {-1000, nil, 1}, {1000, nil, 0}}
	rng := rand.New(rand.NewSource(10102026))
	for i := 0; i < 64; i++ {
		var s strings.Builder
		for j := 0; j < 17; j++ {
			s.WriteByte(byte('0' + rng.Intn(2)))
		}
		str := s.String()
		items = append(items, item{int64(i) - 32, &str, i % 3})
	}
	for _, i := range items {
		var v any
		if i.bit != nil {
			v = *i.bit
		}
		reviewVectorExec(t, db, "INSERT INTO bits VALUES(?,?,?)", i.id, v, i.active)
	}
	for _, metric := range []vector.BitMetricKind{vector.Hamming, vector.Jaccard} {
		for _, q := range []string{equal, "00000000000000000", "11111111111111111"} {
			for _, filter := range []int{-1, 0, 1} {
				where := "1"
				if filter >= 0 {
					where = fmt.Sprintf("active=%d", filter)
				}
				type expected struct {
					vector.Hit
					null bool
				}
				var want []expected
				var native []vector.Hit
				var nulls, filtered int64
				for _, i := range items {
					if filter >= 0 && i.active != filter {
						filtered++
						continue
					}
					e := expected{Hit: vector.Hit{RowID: i.id}, null: i.bit == nil}
					if e.null {
						nulls++
					} else {
						e.Distance = bitReference(*i.bit, q, metric)
						native = append(native, e.Hit)
					}
					want = append(want, e)
				}
				sort.Slice(want, func(i, j int) bool {
					if want[i].null != want[j].null {
						return want[i].null
					}
					return bitHitBetter(want[i].Hit, want[j].Hit)
				})
				sort.Slice(native, func(i, j int) bool { return bitHitBetter(native[i], native[j]) })
				op := "<~>"
				fn := "hamming_distance"
				if metric == vector.Jaccard {
					op = "<%>"
					fn = "jaccard_distance"
				}
				for _, window := range [][2]int{{1, 0}, {7, 0}, {3, 2}, {13, 5}, {200, 0}, {3, 200}} {
					limit, offset := window[0], window[1]
					for _, expr := range []string{"b " + op + " ?", "? " + op + " b", fn + "(b,?)"} {
						rows, plan, stats := reviewVectorQuery(t, db, "SELECT id,"+expr+" AS distance FROM bits WHERE "+where+" ORDER BY distance LIMIT ? OFFSET ?", q, limit, offset)
						begin := min(offset, len(want))
						end := min(begin+limit, len(want))
						if len(rows) != end-begin {
							t.Fatal(len(rows), end-begin)
						}
						for i, row := range rows {
							w := want[begin+i]
							if row[0] != w.RowID || w.null && row[1] != nil || !w.null && row[1] != w.Distance {
								t.Fatal(rows, want)
							}
						}
						if plan.VectorRanking != "bounded_heap" || plan.VectorRepresentation != "bit" || plan.VectorBitMetric != metric || plan.VectorMetric != "" || !plan.VectorExact || plan.VectorDimensions != 17 || plan.ANNFallback != "BIT(n) ANN is unsupported; exact packed-bit ranking" {
							t.Fatal(plan)
						}
						if stats.VectorRankComponents != 0 || stats.VectorRankPackedBytes != 3*(1+int64(len(native))) || stats.VectorRankedRows != int64(len(native)) || stats.VectorNullRows != nulls || stats.VectorFilteredRows != filtered {
							t.Fatal(stats)
						}
					}
				}
				got, err := db.BitSearch(context.Background(), "bits", "b", q, BitSearchOptions{Metric: metric, Limit: 7, Where: where})
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got.Hits, native[:min(7, len(native))]) || got.Plan.Metric != metric || !got.Plan.Exact || got.Plan.Method != "exact_bit" || got.Stats.ScoredPackedBytes != 3*(1+int64(len(native))) || got.Stats.BaseRowFetches != int64(len(items)) || got.Stats.FilteredRows != filtered || got.Stats.NullRows != nulls {
					t.Fatal(got, native)
				}
			}
		}
	}
	// Unsupported SQL shapes retain the general sorter and explicit secondary keys.
	rows, plan, _ := reviewVectorQuery(t, db, "SELECT id FROM bits WHERE id<0 ORDER BY b <~> ? DESC,id DESC LIMIT 3", equal)
	if len(rows) != 3 || plan.VectorRanking != "" {
		t.Fatal(rows, plan)
	}
	// Type identity is not inferred from matching dimension or a raw BLOB column.
	reviewVectorExec(t, db, "CREATE TABLE other(id INTEGER PRIMARY KEY,b BLOB,v VECTOR(1))")
	for _, field := range []string{"b", "v", "id"} {
		if _, err := db.BitSearch(context.Background(), "other", field, "1", BitSearchOptions{Limit: 1}); !errors.Is(err, ErrMismatch) {
			t.Fatal(field, err)
		}
	}
	if _, err := db.VectorSearch(context.Background(), "bits", "b", equal, VectorSearchOptions{Limit: 1}); !errors.Is(err, ErrMismatch) {
		t.Fatal(err)
	}
	if err := db.CreateHNSWIndex(context.Background(), "bits_hnsw", "bits", "b"); !errors.Is(err, ErrMismatch) {
		t.Fatal(err)
	}
	if err := db.CreateIVFFlatIndex(context.Background(), "bits_ivf", "bits", "b"); !errors.Is(err, ErrMismatch) {
		t.Fatal(err)
	}
}

func TestBitNativeQuotasNullsCancellationAndSQLLaziness(t *testing.T) {
	db, _ := reviewVectorDB(t, Options{})
	reviewVectorExec(t, db, "CREATE TABLE bits(id INTEGER PRIMARY KEY,b BIT(9),active INTEGER)")
	reviewVectorExec(t, db, "INSERT INTO bits VALUES(1,'000000000',1),(2,'111111111',1),(3,NULL,0)")
	ctx := context.Background()
	assertError := func(q any, o BitSearchOptions, want error) {
		t.Helper()
		got, err := db.BitSearch(ctx, "bits", "b", q, o)
		if !errors.Is(err, want) || !reflect.DeepEqual(got, BitResult{}) {
			t.Fatal(got, err, want)
		}
	}
	assertError("000000000", BitSearchOptions{Limit: -1}, vector.ErrOptions)
	assertError("000000000", BitSearchOptions{Limit: 1, Metric: "l2"}, vector.ErrMetric)
	assertError("000000000", BitSearchOptions{Limit: 1, Limits: BitSearchLimits{MaxCandidates: 1}}, vector.ErrOptions)
	assertError("000000000", BitSearchOptions{Limit: 1, Where: "active=1", Limits: BitSearchLimits{MaxCandidates: 2, MaxResults: 2, MaxPackedBytes: 8}}, vector.ErrLimit)
	assertError("000000000", BitSearchOptions{Limit: 1, Limits: BitSearchLimits{MaxCandidates: 3, MaxResults: 2, MaxPackedBytes: 5}}, vector.ErrLimit)
	got, err := db.BitSearch(ctx, "bits", "b", "000000000", BitSearchOptions{Limit: 2, Limits: BitSearchLimits{MaxCandidates: 3, MaxResults: 2, MaxPackedBytes: 6}})
	if err != nil || len(got.Hits) != 2 || got.Stats.ScoredPackedBytes != 6 {
		t.Fatal(got, err)
	}
	for _, q := range []any{nil, vector.NullBitVector{}, []byte(nil)} {
		got, err := db.BitSearch(ctx, "bits", "b", q, BitSearchOptions{Limit: 2})
		if err != nil || got.Plan.Access != "EMPTY" || len(got.Hits) != 0 || got.Stats.BaseRowFetches != 0 {
			t.Fatal(got, err)
		}
	}
	got, err = db.BitSearch(ctx, "bits", "b", "000000000", BitSearchOptions{Limit: 0})
	if err != nil || got.Plan.Access != "EMPTY" || got.Stats.BaseRowFetches != 0 || got.Stats.ScoredPackedBytes != 2 {
		t.Fatal(got, err)
	}
	assertError("", BitSearchOptions{Limit: 0}, vector.ErrDimensions)
	assertError("bad", BitSearchOptions{Limit: 0}, vector.ErrSyntax)
	assertError(vector.BitVector{}, BitSearchOptions{Limit: 0}, vector.ErrInvalid)
	assertError(strings.Repeat("1", 64001), BitSearchOptions{Limit: 0}, vector.ErrLimit)
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if got, err := db.BitSearch(canceled, "bits", "b", nil, BitSearchOptions{}); !errors.Is(err, context.Canceled) || !reflect.DeepEqual(got, BitResult{}) {
		t.Fatal(got, err)
	}
	if got, err := db.BitSearch(nil, "bits", "b", nil, BitSearchOptions{}); err == nil || !reflect.DeepEqual(got, BitResult{}) {
		t.Fatal(got, err)
	}
	rows, plan, stats := reviewVectorQuery(t, db, "SELECT id FROM bits WHERE active=0 ORDER BY b <~> 'bad' LIMIT 1")
	if len(rows) != 1 || rows[0][0] != int64(3) || plan.VectorBitMetric != vector.Hamming || stats.VectorRankPackedBytes != 0 {
		t.Fatal(rows, plan, stats)
	}
	rows, _, stats = reviewVectorQuery(t, db, "SELECT id FROM bits ORDER BY b <~> 'bad' LIMIT 0")
	if len(rows) != 0 || stats.BaseRowFetches != 0 {
		t.Fatal(rows, stats)
	}
	if r, err := db.Query(ctx, "SELECT id FROM bits ORDER BY b <~> 'bad' LIMIT 1"); err == nil {
		r.Close()
		t.Fatal("eligible invalid query accepted")
	}
	r, err := db.Query(ctx, "EXPLAIN QUERY PLAN SELECT id FROM bits ORDER BY b <%> 'bad' LIMIT 1")
	if err != nil {
		t.Fatal(err)
	}
	r.Close()
	// SQL quota checks on a memory-only ranking operation, with intact DB fixtures.
	r, err = db.Query(ctx, "SELECT b FROM bits")
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	rank := &vectorRank{dimensions: 9, query: "000000000", bitMetric: vector.Hamming, bitLimits: BitSearchLimits{MaxPackedBytes: 3}}
	if _, err = rank.scoreBit(r, "111111111"); !errors.Is(err, vector.ErrLimit) {
		t.Fatal(err)
	}
}

func TestBitMaximumWidthAndMemoryBound(t *testing.T) {
	db, path := reviewVectorDB(t, Options{PageSize: 512})
	s := strings.Repeat("10000001", 8000)
	reviewVectorExec(t, db, "CREATE TABLE bits(id INTEGER PRIMARY KEY,b BIT(64000))")
	reviewVectorExec(t, db, "INSERT INTO bits VALUES(1,?),(2,?)", s, mustBit(t, s))
	got, err := db.BitSearch(context.Background(), "bits", "b", s, BitSearchOptions{Limit: 2})
	if err != nil || len(got.Hits) != 2 || got.Stats.ScoredPackedBytes != 24000 {
		t.Fatal(got, err)
	}
	rows, _, _ := reviewVectorQuery(t, db, "SELECT b FROM bits ORDER BY id")
	for _, row := range rows {
		b := row[0].([]byte)
		if len(b) != 8012 || binary.BigEndian.Uint32(b[8:12]) != 64000 || !bytes.Equal(b[12:], bytes.Repeat([]byte{0x81}, 8000)) {
			t.Fatal("packed persistence changed")
		}
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path, Options{MaxSortBytes: 100})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if got, err = reopened.BitSearch(context.Background(), "bits", "b", s, BitSearchOptions{Limit: 1}); !errors.Is(err, ErrMemoryLimit) || !reflect.DeepEqual(got, BitResult{}) {
		t.Fatal(got, err)
	}
}

func TestBitSharedSnapshotAndExplicitTransformMaintenance(t *testing.T) {
	ctx := context.Background()
	db, path := reviewVectorDB(t, Options{WALShared: true})
	reviewVectorExec(t, db, "CREATE TABLE bits(id INTEGER PRIMARY KEY,b BIT(2),v VECTOR(2),active INTEGER)")
	reviewVectorExec(t, db, "INSERT INTO bits VALUES(1,binary_quantize('[1,-1]'),'[1,-1]',1)")
	reader, err := db.Begin(ctx, TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Rollback()
	first, err := reader.BitSearch(ctx, "bits", "b", "10", BitSearchOptions{Limit: 1})
	if err != nil || first.Hits[0].Distance != 0 {
		t.Fatal(first, err)
	}
	writer, err := Open(path, Options{WALShared: true})
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	// Explicit same-statement maintenance, not an automatic generated column.
	reviewVectorExec(t, writer, "UPDATE bits SET v='[-1,1]',b=binary_quantize('[-1,1]') WHERE id=1")
	pinned, err := reader.BitSearch(ctx, "bits", "b", "10", BitSearchOptions{Limit: 1})
	if err != nil || !reflect.DeepEqual(first.Hits, pinned.Hits) {
		t.Fatal(pinned, err)
	}
	// The DB-level entry point in an active transaction uses that same snapshot.
	viaDB, err := db.BitSearch(ctx, "bits", "b", "10", BitSearchOptions{Limit: 1})
	if err != nil || !reflect.DeepEqual(first.Hits, viaDB.Hits) {
		t.Fatal(viaDB, err)
	}
	if err = reader.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	current, err := db.BitSearch(ctx, "bits", "b", "10", BitSearchOptions{Limit: 1})
	if err != nil || current.Hits[0].Distance != 2 {
		t.Fatal(current, err)
	}
	rows, _, _ := reviewVectorQuery(t, db, "SELECT bit_to_text(b),bit_to_text(binary_quantize(v)) FROM bits")
	if !reflect.DeepEqual(rows, [][]any{{"01", "01"}}) {
		t.Fatal(rows)
	}
}

type bitCancelValuer struct{ cancel context.CancelFunc }

func (v bitCancelValuer) Value() (driver.Value, error) { v.cancel(); return "000000000", nil }

func TestBitQueryCallbackCancellationAndSQLOrderShapes(t *testing.T) {
	db, _ := reviewVectorDB(t, Options{})
	reviewVectorExec(t, db, "CREATE TABLE bits(id INTEGER PRIMARY KEY,b BIT(9))")
	reviewVectorExec(t, db, "INSERT INTO bits VALUES(1,'000000000'),(2,'111111111'),(3,NULL)")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	got, err := db.BitSearch(ctx, "bits", "b", bitCancelValuer{cancel}, BitSearchOptions{Limit: 0})
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(got, BitResult{}) {
		t.Fatal(got, err)
	}
	for _, q := range []string{"SELECT id,b <~> '000000000' FROM bits ORDER BY 2 LIMIT 2", "SELECT id FROM bits ORDER BY b <~> '000000000' LIMIT 2"} {
		rows, plan, _ := reviewVectorQuery(t, db, q)
		if len(rows) != 2 || rows[0][0] != int64(3) || rows[1][0] != int64(1) || plan.VectorRanking != "bounded_heap" {
			t.Fatal(rows, plan)
		}
	}
	rows, plan, _ := reviewVectorQuery(t, db, "SELECT id FROM bits ORDER BY b <~> bit_vector('000000000') LIMIT 2")
	if len(rows) != 2 || rows[0][0] != int64(3) || rows[1][0] != int64(1) || plan.VectorRanking != "" {
		t.Fatal(rows, plan)
	}
	for _, limits := range []BitSearchLimits{{MaxCandidates: 1<<20 + 1, MaxResults: 1, MaxPackedBytes: 1}, {MaxCandidates: 1, MaxResults: 1<<20 + 1, MaxPackedBytes: 1}, {MaxCandidates: 1, MaxResults: 1, MaxPackedBytes: 1<<34 + 1}} {
		if got, err := db.BitSearch(context.Background(), "bits", "b", nil, BitSearchOptions{Limit: 1, Limits: limits}); !errors.Is(err, vector.ErrOptions) || !reflect.DeepEqual(got, BitResult{}) {
			t.Fatal(got, err)
		}
	}
	if got, err := db.BitSearch(context.Background(), "bits", "b", nil, BitSearchOptions{Limit: 10001}); !errors.Is(err, vector.ErrLimit) || !reflect.DeepEqual(got, BitResult{}) {
		t.Fatal(got, err)
	}
}
