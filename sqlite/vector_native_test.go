package sqlite_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"sort"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/similarity"
	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

type nativeVectorSearcher interface {
	VectorSearch(context.Context, string, string, any, sqlite.VectorSearchOptions) (sqlite.VectorResult, error)
}

func searchNativeVector(t *testing.T, db nativeVectorSearcher, query any, options sqlite.VectorSearchOptions) sqlite.VectorResult {
	t.Helper()
	result, err := db.VectorSearch(context.Background(), "items", "embedding", query, options)
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func mustNativeVector(t *testing.T, values ...float32) vector.Vector {
	t.Helper()
	v, err := vector.New(values)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

// Independent arithmetic and full sorting below deliberately do not call the
// production metrics, scan, or heap. Quarter-integer inputs keep the sums exact
// in float64; only sqrt and the final cosine division need a rounding tolerance.
func referenceNativeVectorDistance(a, b []float32, metric vector.MetricKind) float64 {
	var squared, manhattan, dot, aa, bb float64
	for i, x := range a {
		f, g := float64(x), float64(b[i])
		d := f - g
		squared += d * d
		manhattan += math.Abs(d)
		dot += f * g
		aa += f * f
		bb += g * g
	}
	switch metric {
	case vector.L1:
		return manhattan
	case vector.NegativeIP:
		return -dot
	case vector.Cosine:
		return 1 - math.Max(-1, math.Min(1, dot/math.Sqrt(aa*bb)))
	default:
		return math.Sqrt(squared)
	}
}

func assertNativeVectorHits(t *testing.T, got, want []vector.Hit) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("hits: got %+v want %+v", got, want)
	}
	for i := range want {
		if got[i].RowID != want[i].RowID || math.IsNaN(got[i].Distance) || math.IsInf(got[i].Distance, 0) || math.Abs(got[i].Distance-want[i].Distance) > 1e-13*math.Max(1, math.Abs(want[i].Distance)) {
			t.Fatalf("hit %d: got %+v want %+v", i, got[i], want[i])
		}
	}
}

func TestVectorNativeRandomizedIndependentRanking(t *testing.T) {
	for _, dimensions := range []int{1, 3, 17} {
		t.Run(fmt.Sprint(dimensions), func(t *testing.T) {
			db, _ := writable(t, sqlite.Options{})
			execute(t, db, fmt.Sprintf("CREATE TABLE items(id INTEGER PRIMARY KEY,embedding VECTOR(%d),active INTEGER)", dimensions))
			rng := rand.New(rand.NewSource(61000 + int64(dimensions)))
			type candidate struct {
				id     int64
				values []float32
				active int
			}
			var candidates []candidate
			tx, err := db.Begin(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 60; i++ {
				values := make([]float32, dimensions)
				for j := range values {
					values[j] = float32(rng.Intn(65)-32) / 4
				}
				// A nonzero first component guarantees every cosine is defined.
				if values[0] == 0 {
					values[0] = .25
				}
				if i > 0 && i%10 == 0 {
					values = append([]float32(nil), candidates[i-1].values...)
				}
				row := candidate{int64(i) - 30, values, i % 3}
				candidates = append(candidates, row)
				txExecute(t, tx, "INSERT INTO items VALUES(?,?,?)", row.id, mustNativeVector(t, values...), row.active)
			}
			txExecute(t, tx, "INSERT INTO items VALUES(100,NULL,1)")
			if err := tx.Commit(context.Background()); err != nil {
				t.Fatal(err)
			}
			for qi := 0; qi < 8; qi++ {
				q := candidates[qi*7].values
				for _, metric := range []vector.MetricKind{vector.L2, vector.L1, vector.Cosine, vector.NegativeIP} {
					for _, filtered := range []bool{false, true} {
						for _, limit := range []int{0, 1, 7, 100} {
							o := sqlite.VectorSearchOptions{Metric: metric, Limit: limit}
							if filtered {
								o.Where, o.Args = "items.active>=:minimum", []any{sql.Named("minimum", 1)}
							}
							var want []vector.Hit
							for _, row := range candidates {
								if !filtered || row.active >= 1 {
									want = append(want, vector.Hit{RowID: row.id, Distance: referenceNativeVectorDistance(q, row.values, metric)})
								}
							}
							sort.Slice(want, func(i, j int) bool {
								return want[i].Distance < want[j].Distance || want[i].Distance == want[j].Distance && want[i].RowID < want[j].RowID
							})
							want = want[:min(limit, len(want))]
							got := searchNativeVector(t, db, mustNativeVector(t, q...), o)
							assertNativeVectorHits(t, got.Hits, want)
							access := "SCAN"
							if limit == 0 {
								access = "EMPTY"
							}
							if got.Plan.Access != access || !got.Plan.Exact || got.Plan.Table != "items" || got.Plan.Field != "embedding" || got.Plan.Dimensions != dimensions || got.Plan.Metric != metric || got.Plan.Limit != limit || len(got.Plan.Indexes) != 0 {
								t.Fatalf("untruthful exact plan: %+v", got.Plan)
							}
							if got.Stats.ReturnedRows != int64(len(want)) || got.Stats.PostingKeysRead != 0 {
								t.Fatalf("untruthful work: %+v", got.Stats)
							}
							if limit == 0 {
								if got.Stats.BaseRowFetches != 0 || got.Stats.ScoredRows != 0 {
									t.Fatalf("zero limit scanned: %+v", got)
								}
							} else {
								scored, excluded := int64(60), int64(0)
								if filtered {
									scored, excluded = 40, 20
								}
								if got.Stats.BaseRowFetches != 61 || got.Stats.ScoredRows != scored || got.Stats.FilteredRows != excluded || got.Stats.NullRows != 1 || got.Stats.ScoredComponents != (scored+1)*int64(dimensions) {
									t.Fatalf("actual scan work: %+v", got.Stats)
								}
							}
						}
					}
				}
			}
		})
	}
}

func TestVectorNativeSignedTiesNullAndQueryForms(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,embedding VECTOR(3))")
	for _, id := range []int64{math.MaxInt64, 7, -1, math.MinInt64} {
		execute(t, db, "INSERT INTO items VALUES(?,'[1,2,3]')", id)
	}
	execute(t, db, "INSERT INTO items VALUES(0,NULL)")
	v := mustNativeVector(t, 1, 2, 3)
	blob, err := v.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []any{" [1, 2, 3] ", v, blob, vector.NullVector{Vector: v, Valid: true}} {
		for _, metric := range []vector.MetricKind{"", vector.L2, vector.L1, vector.Cosine, vector.NegativeIP} {
			got := searchNativeVector(t, db, q, sqlite.VectorSearchOptions{Metric: metric, Limit: 3})
			distance := float64(0)
			if metric == vector.NegativeIP {
				distance = -14
			}
			assertNativeVectorHits(t, got.Hits, []vector.Hit{{RowID: math.MinInt64, Distance: distance}, {RowID: -1, Distance: distance}, {RowID: 7, Distance: distance}})
			if metric == "" && got.Plan.Metric != vector.L2 {
				t.Fatalf("default metric: %+v", got.Plan)
			}
		}
	}
	for _, q := range []any{nil, []byte(nil), vector.NullVector{}} {
		got := searchNativeVector(t, db, q, sqlite.VectorSearchOptions{Limit: 3})
		if len(got.Hits) != 0 || got.Plan.Access != "EMPTY" || got.Stats.BaseRowFetches != 0 || got.Stats.ScoredRows != 0 {
			t.Fatalf("NULL query must read no source rows: %+v", got)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if got, err := db.VectorSearch(context.Background(), "items", "embedding", v, sqlite.VectorSearchOptions{Limit: 1}); !errors.Is(err, sqlite.ErrClosed) || !reflect.DeepEqual(got, sqlite.VectorResult{}) {
		t.Fatalf("closed database: %+v %v", got, err)
	}
}

func TestVectorNativeWhereMatchesSQL(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,embedding VECTOR(3),active INTEGER,flag TEXT)")
	execute(t, db, "INSERT INTO items VALUES(-3,'[1,2,3]',1,'1'),(1,'[1,2,3]',0,'0'),(2,'[1,2,3]',NULL,NULL),(3,'[1,2,3]',2,'2')")
	for _, tc := range []struct {
		where string
		args  []any
	}{
		{"active", nil}, {"active=TRUE", nil}, {"active IS FALSE", nil}, {"active IS NULL", nil}, {"flag=1", nil},
		{"items.active=?2 AND id>=?1", []any{-5, 1}},
		{"items.active=:value OR items.id=@value", []any{sql.Named("value", 1)}},
		{"_rowid_>=? AND oid<?", []any{-3, 2}},
		{"active=1 /* trailing comment */", nil},
		{"active=1 -- trailing comment", nil},
		{"active=? -- trailing parameter comment", []any{1}},
	} {
		t.Run(tc.where, func(t *testing.T) {
			rows := query(t, db, "SELECT id FROM items WHERE "+tc.where+"\n ORDER BY id", tc.args...)
			want := make([]vector.Hit, len(rows))
			for i, row := range rows {
				want[i].RowID = row[0].(int64)
			}
			got := searchNativeVector(t, db, "[1,2,3]", sqlite.VectorSearchOptions{Limit: 10, Where: tc.where, Args: tc.args})
			assertNativeVectorHits(t, got.Hits, want)
		})
	}
}

func TestVectorNativeCosineFilterBeforeScoring(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,embedding VECTOR(3),active INTEGER)")
	execute(t, db, "INSERT INTO items VALUES(1,'[1,0,0]',1),(2,'[0,0,0]',0),(3,NULL,1)")
	got := searchNativeVector(t, db, "[1,0,0]", sqlite.VectorSearchOptions{Metric: vector.Cosine, Limit: 10, Where: "active=1"})
	assertNativeVectorHits(t, got.Hits, []vector.Hit{{RowID: 1}})
	if got.Stats.FilteredRows != 1 || got.Stats.NullRows != 1 || got.Stats.ScoredRows != 1 {
		t.Fatalf("cosine filter counters: %+v", got)
	}
	got, err := db.VectorSearch(context.Background(), "items", "embedding", "[1,0,0]", sqlite.VectorSearchOptions{Metric: vector.Cosine, Limit: 1})
	if !errors.Is(err, vector.ErrZeroNorm) || !reflect.DeepEqual(got, sqlite.VectorResult{}) {
		t.Fatalf("zero-norm candidate must discard earlier hits: %+v %v", got, err)
	}
	for _, limit := range []int{0, 1} {
		got, err := db.VectorSearch(context.Background(), "items", "embedding", "[0,0,0]", sqlite.VectorSearchOptions{Metric: vector.Cosine, Limit: limit, Where: "active=99"})
		if !errors.Is(err, vector.ErrZeroNorm) || !reflect.DeepEqual(got, sqlite.VectorResult{}) {
			t.Fatalf("zero-norm query must be validated even without matches: %+v %v", got, err)
		}
	}
	// Zero vectors remain valid for all three metrics whose domain includes them.
	for _, metric := range []vector.MetricKind{vector.L2, vector.L1, vector.NegativeIP} {
		got := searchNativeVector(t, db, "[0,0,0]", sqlite.VectorSearchOptions{Metric: metric, Limit: 10})
		if len(got.Hits) != 2 {
			t.Fatal(got)
		}
	}
}

func TestVectorNativeRejectsInvalidOptionsAndQueries(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,embedding VECTOR(3),active INTEGER)")
	execute(t, db, "INSERT INTO items VALUES(1,'[1,2,3]',1)")
	for _, options := range []sqlite.VectorSearchOptions{
		{Limit: -1}, {Limit: 10001}, {Limit: 1, Metric: "hnsw"},
		{Limit: 1, Limits: vector.SearchLimits{MaxCandidates: 1}},
		{Limit: 1, Where: "missing=1"}, {Limit: 1, Where: "active=?"},
		{Limit: 1, Where: "active) ORDER BY id --"}, {Limit: 1, Args: []any{1}},
		{Limit: 1, Where: "active=1; DELETE FROM items"}, {Limit: 1, Where: "active=1 ORDER BY id"},
	} {
		got, err := db.VectorSearch(context.Background(), "items", "embedding", "[1,2,3]", options)
		if err == nil || !reflect.DeepEqual(got, sqlite.VectorResult{}) {
			t.Fatalf("invalid options %+v: %+v %v", options, got, err)
		}
	}
	for _, q := range []any{"[1,2]", "[]", "[NaN,2,3]", "[1,Inf,3]", "[1,2,3] trailing", []byte("[1,2,3]"), []byte{}, vector.Vector{}, int64(3), struct{}{}} {
		for _, limit := range []int{0, 1} {
			got, err := db.VectorSearch(context.Background(), "items", "embedding", q, sqlite.VectorSearchOptions{Limit: limit})
			if err == nil || !reflect.DeepEqual(got, sqlite.VectorResult{}) {
				t.Fatalf("invalid query type=%T limit=%d: %+v %v", q, limit, got, err)
			}
		}
	}
	for _, target := range [][2]string{{"missing", "embedding"}, {"items", "missing"}, {"items", "active"}, {"items", "id"}} {
		if got, err := db.VectorSearch(context.Background(), target[0], target[1], "[1,2,3]", sqlite.VectorSearchOptions{Limit: 1}); err == nil || !reflect.DeepEqual(got, sqlite.VectorResult{}) {
			t.Fatalf("invalid target %v: %+v %v", target, got, err)
		}
	}
	if got, err := db.VectorSearch(nil, "items", "embedding", "[1,2,3]", sqlite.VectorSearchOptions{Limit: 1}); err == nil || !reflect.DeepEqual(got, sqlite.VectorResult{}) {
		t.Fatalf("nil context: %+v %v", got, err)
	}
	if got := query(t, db, "SELECT id FROM items"); !reflect.DeepEqual(got, [][]any{{int64(1)}}) {
		t.Fatalf("rejected filter syntax changed data: %+v", got)
	}
}

func TestVectorNativeBudgetsCountEveryVisitedRow(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,embedding VECTOR(3),active INTEGER)")
	execute(t, db, "INSERT INTO items VALUES(1,'[1,2,3]',0),(2,'[2,3,4]',0),(3,NULL,0)")
	for _, where := range []string{"", "active=1"} {
		limits := vector.DefaultSearchLimits()
		limits.MaxCandidates = 2
		got, err := db.VectorSearch(context.Background(), "items", "embedding", "[1,2,3]", sqlite.VectorSearchOptions{Limit: 1, Where: where, Limits: limits})
		if !errors.Is(err, vector.ErrLimit) || !errors.Is(err, sqlite.ErrMemoryLimit) || !reflect.DeepEqual(got, sqlite.VectorResult{}) {
			t.Fatalf("filtered/NULL rows evaded candidate budget where=%q: %+v %v", where, got, err)
		}
	}
	for _, components := range []uint64{2, 3, 6, 8} {
		limits := vector.DefaultSearchLimits()
		limits.MaxComponents = components
		got, err := db.VectorSearch(context.Background(), "items", "embedding", "[1,2,3]", sqlite.VectorSearchOptions{Limit: 1, Limits: limits})
		if !errors.Is(err, vector.ErrLimit) || !reflect.DeepEqual(got, sqlite.VectorResult{}) {
			t.Fatalf("query and score component budget %d: %+v %v", components, got, err)
		}
	}
	limits := vector.DefaultSearchLimits()
	limits.MaxCandidates, limits.MaxComponents = 3, 9
	got := searchNativeVector(t, db, "[1,2,3]", sqlite.VectorSearchOptions{Limit: 1, Limits: limits})
	assertNativeVectorHits(t, got.Hits, []vector.Hit{{RowID: 1}})
	limits.MaxComponents = 3 // Filtered rows consume visits but no distance work.
	got = searchNativeVector(t, db, "[1,2,3]", sqlite.VectorSearchOptions{Limit: 1, Where: "active=1", Limits: limits})
	if len(got.Hits) != 0 || got.Stats.BaseRowFetches != 3 || got.Stats.ScoredComponents != 3 {
		t.Fatalf("filtered arithmetic budget: %+v", got)
	}
}

type nativeVectorCancelValuer struct {
	cancel context.CancelFunc
	value  driver.Value
}

func (v nativeVectorCancelValuer) Value() (driver.Value, error) {
	v.cancel()
	return v.value, nil
}

type nativeVectorCancelContext struct {
	context.Context
	cancel context.CancelFunc
	checks atomic.Int64
}

func (c *nativeVectorCancelContext) Err() error {
	if c.checks.Add(1) == 160 {
		c.cancel()
	}
	return c.Context.Err()
}

func TestVectorNativeCancellationAndReaderRelease(t *testing.T) {
	ctx := context.Background()
	db, path := writable(t, sqlite.Options{WALShared: true})
	execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,embedding VECTOR(3),active INTEGER)")
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 80; i++ {
		txExecute(t, tx, "INSERT INTO items VALUES(?,'[1,2,3]',1)", i)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	peer, err := sqlite.Open(path, sqlite.Options{WALShared: true})
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	checkFailure := func(t *testing.T, searchCtx context.Context, q any, options sqlite.VectorSearchOptions) {
		t.Helper()
		got, err := db.VectorSearch(searchCtx, "items", "embedding", q, options)
		var coded *sqlite.Error
		if !errors.Is(err, context.Canceled) || !errors.As(err, &coded) || coded.Code != sqlite.CodeInterrupt || !reflect.DeepEqual(got, sqlite.VectorResult{}) {
			t.Fatalf("cancellation leaked result or lost code: %+v %v", got, err)
		}
		// A leaked shared-WAL reader would pin the older snapshot and make
		// this ordinary checkpoint busy after the peer's committed update.
		execute(t, peer, "UPDATE items SET active=active+1 WHERE id=1")
		if err := peer.Checkpoint(ctx); err != nil {
			t.Fatalf("failed search retained a reader: %v", err)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	checkFailure(t, canceled, "[1,2,3]", sqlite.VectorSearchOptions{Limit: 1})
	for _, value := range []driver.Value{nil, "[1,2,3]"} {
		qctx, qcancel := context.WithCancel(ctx)
		checkFailure(t, qctx, nativeVectorCancelValuer{cancel: qcancel, value: value}, sqlite.VectorSearchOptions{Limit: 1})
		qcancel()
	}
	argctx, argcancel := context.WithCancel(ctx)
	checkFailure(t, argctx, "[1,2,3]", sqlite.VectorSearchOptions{Limit: 1, Where: "active=?", Args: []any{nativeVectorCancelValuer{cancel: argcancel, value: int64(1)}}})
	argcancel()
	scanctx, scancel := context.WithCancel(ctx)
	defer scancel()
	checkFailure(t, &nativeVectorCancelContext{Context: scanctx, cancel: scancel}, "[1,2,3]", sqlite.VectorSearchOptions{Limit: 1})
	got := searchNativeVector(t, db, "[1,2,3]", sqlite.VectorSearchOptions{Limit: 1})
	assertNativeVectorHits(t, got.Hits, []vector.Hit{{RowID: 1}})
	execute(t, peer, "UPDATE items SET active=active+1 WHERE id=1")
	if err := peer.Checkpoint(ctx); err != nil {
		t.Fatalf("successful search retained a reader: %v", err)
	}
}

func TestVectorNativeRuntimeFilterFailureDiscardsHits(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,embedding VECTOR(3),n INTEGER)")
	execute(t, db, "INSERT INTO items VALUES(1,'[1,2,3]',2),(2,'[2,3,4]',4)")
	got, err := db.VectorSearch(context.Background(), "items", "embedding", "[1,2,3]", sqlite.VectorSearchOptions{Limit: 1, Where: "ngram_similarity('abc','abc',n,'dice')>=0"})
	if !errors.Is(err, similarity.ErrValue) || !reflect.DeepEqual(got, sqlite.VectorResult{}) {
		t.Fatalf("late WHERE error leaked retained hit: %+v %v", got, err)
	}
	got, err = db.VectorSearch(similaritySQLContext(t), "items", "embedding", "[1,2,3]", sqlite.VectorSearchOptions{Limit: 1, Where: "levenshtein(?,?)>0", Args: []any{strings.Repeat("a", 4096), strings.Repeat("b", 4096)}})
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(got, sqlite.VectorResult{}) {
		t.Fatalf("WHERE cancellation leaked retained hit: %+v %v", got, err)
	}
	execute(t, db, "UPDATE items SET n=2")
	assertNativeVectorHits(t, searchNativeVector(t, db, "[1,2,3]", sqlite.VectorSearchOptions{Limit: 1}).Hits, []vector.Hit{{RowID: 1}})
}

func TestVectorNativeTransactionsRollbackAndOrdinaryReopen(t *testing.T) {
	ctx := context.Background()
	for _, wal := range []bool{false, true} {
		t.Run(fmt.Sprintf("wal=%t", wal), func(t *testing.T) {
			opts := sqlite.Options{WALShared: wal}
			db, path := writable(t, opts)
			execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,embedding VECTOR(3))")
			execute(t, db, "INSERT INTO items VALUES(1,'[1,2,3]'),(2,'[4,5,6]'),(3,NULL)")
			o := sqlite.VectorSearchOptions{Limit: 10}
			before := searchNativeVector(t, db, "[1,2,3]", o)
			tx, err := db.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			txExecute(t, tx, "UPDATE items SET embedding='[1,2,3]' WHERE id=2")
			txExecute(t, tx, "DELETE FROM items WHERE id=1")
			txExecute(t, tx, "INSERT INTO items VALUES(-10,'[1,2,3]')")
			own := searchNativeVector(t, tx, "[1,2,3]", o)
			assertNativeVectorHits(t, own.Hits, []vector.Hit{{RowID: -10}, {RowID: 2}})
			assertNativeVectorHits(t, searchNativeVector(t, db, "[1,2,3]", o).Hits, own.Hits)
			if err := tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			assertNativeVectorHits(t, searchNativeVector(t, db, "[1,2,3]", o).Hits, before.Hits)
			if got, err := tx.VectorSearch(ctx, "items", "embedding", "[1,2,3]", o); !errors.Is(err, sqlite.ErrTxDone) || !reflect.DeepEqual(got, sqlite.VectorResult{}) {
				t.Fatalf("completed tx: %+v %v", got, err)
			}
			tx, err = db.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			txExecute(t, tx, "UPDATE items SET embedding='[-1,-2,-3]' WHERE id=2")
			txExecute(t, tx, "INSERT INTO items VALUES(-10,'[1,2,3]')")
			if err := tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			committed := searchNativeVector(t, db, "[1,2,3]", o)
			assertNativeVectorHits(t, committed.Hits, []vector.Hit{{RowID: -10}, {RowID: 1}, {RowID: 2, Distance: math.Sqrt(56)}})
			if wal {
				if err := db.Checkpoint(ctx); err != nil {
					t.Fatal(err)
				}
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			reopened, err := sqlite.Open(path, opts)
			if err != nil {
				t.Fatal(err)
			}
			defer reopened.Close()
			assertNativeVectorHits(t, searchNativeVector(t, reopened, "[1,2,3]", o).Hits, committed.Hits)
		})
	}
}

func TestVectorNativeSharedWALSnapshotVisibility(t *testing.T) {
	ctx := context.Background()
	opts := sqlite.Options{WALShared: true}
	a, path := writable(t, opts)
	execute(t, a, "CREATE TABLE items(id INTEGER PRIMARY KEY,embedding VECTOR(3))")
	execute(t, a, "INSERT INTO items VALUES(1,'[1,2,3]'),(2,'[4,5,6]')")
	b, err := sqlite.Open(path, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	o := sqlite.VectorSearchOptions{Limit: 1}
	reader, err := b.Begin(ctx, sqlite.TxOptions{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Rollback()
	assertNativeVectorHits(t, searchNativeVector(t, reader, "[1,2,3]", o).Hits, []vector.Hit{{RowID: 1}})
	writer, err := a.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Rollback()
	txExecute(t, writer, "DELETE FROM items WHERE id=1")
	txExecute(t, writer, "UPDATE items SET embedding='[1,2,3]' WHERE id=2")
	assertNativeVectorHits(t, searchNativeVector(t, writer, "[1,2,3]", o).Hits, []vector.Hit{{RowID: 2}})
	assertNativeVectorHits(t, searchNativeVector(t, reader, "[1,2,3]", o).Hits, []vector.Hit{{RowID: 1}})
	if err := writer.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	assertNativeVectorHits(t, searchNativeVector(t, reader, "[1,2,3]", o).Hits, []vector.Hit{{RowID: 1}})
	if err := a.Checkpoint(ctx); !errors.Is(err, sqlite.ErrBusy) {
		t.Fatalf("old snapshot did not hold checkpoint: %v", err)
	}
	if err := reader.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	assertNativeVectorHits(t, searchNativeVector(t, b, "[1,2,3]", o).Hits, []vector.Hit{{RowID: 2}})
	if err := a.Checkpoint(ctx); err != nil {
		t.Fatal(err)
	}
	assertNativeVectorHits(t, searchNativeVector(t, b, "[1,2,3]", o).Hits, []vector.Hit{{RowID: 2}})
}
