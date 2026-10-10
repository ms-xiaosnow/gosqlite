package sqlite_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func annQueryFixture(t *testing.T, n int, nullable bool) (*sqlite.DB, string) {
	t.Helper()
	db, path := writable(t, sqlite.Options{})
	nullability := " NOT NULL"
	if nullable {
		nullability = ""
	}
	execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,embedding VECTOR(3)"+nullability+",active INTEGER,title TEXT)")
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= n; i++ {
		txExecute(t, tx, "INSERT INTO items VALUES(?,?,?,?)", i, fmt.Sprintf("[%d,%d,1]", i, i%7), i%19 == 0, fmt.Sprintf("item %d", i))
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = db.CreateHNSWIndex(context.Background(), "items_hnsw", "items", "embedding"); err != nil {
		t.Fatal(err)
	}
	return db, path
}
func annNative(t *testing.T, db *sqlite.DB, q any, o sqlite.VectorSearchOptions) sqlite.VectorResult {
	t.Helper()
	got, err := db.VectorSearch(context.Background(), "items", "embedding", q, o)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestANNQueryNativeExactDefaultAndRealSearch(t *testing.T) {
	db, _ := annQueryFixture(t, 180, false)
	exact := annNative(t, db, "[20,6,1]", sqlite.VectorSearchOptions{Limit: 5})
	if !exact.Plan.Exact || exact.Plan.Access != "SCAN" || exact.Stats.BaseRowFetches != 180 || exact.Stats.ANNNodeReads != 0 {
		t.Fatal(exact)
	}
	approx := annNative(t, db, "[20,6,1]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Limit: 5})
	if approx.Plan.Exact || approx.Plan.Access != "SEARCH" || approx.Plan.Method != "hnsw" || approx.Plan.ANNOpclass != "vector_l2_ops" || approx.Plan.ANNGeneration != 1 || approx.Stats.ANNNodeReads == 0 || approx.Stats.ANNEdgeReads == 0 || approx.Stats.ANNDistanceEvaluations == 0 || approx.Stats.ANNExactRescores == 0 || approx.Stats.BaseRowFetches >= 180 {
		t.Fatal(approx)
	}
	assertNativeVectorHits(t, approx.Hits, exact.Hits)
	if len(approx.Hits) != 5 || approx.Stats.ReturnedRows != 5 || approx.Stats.ANNIdentityLookups != approx.Stats.BaseRowFetches {
		t.Fatal(approx)
	}
	if !strings.Contains(approx.Plan.Detail(), "ann_opclass=vector_l2_ops") {
		t.Fatal(approx.Plan.Detail())
	}
}

func TestANNQueryNativeWideningFallbackAndCumulativeBudgets(t *testing.T) {
	db, _ := annQueryFixture(t, 180, false)
	want := annNative(t, db, "[1,1,1]", sqlite.VectorSearchOptions{Limit: 3, Where: "active=1"})
	got := annNative(t, db, "[1,1,1]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Limit: 3, Where: "active=1", EFSearch: 2, MaxEFSearch: 128})
	assertNativeVectorHits(t, got.Hits, want.Hits)
	if got.Stats.ANNIterations < 2 || got.Stats.FilteredRows == 0 || got.Stats.ANNDistanceEvaluations <= got.Stats.ScoredRows {
		t.Fatal(got)
	}
	fallback := annNative(t, db, "[1,1,1]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Limit: 3, Where: "active=1", EFSearch: 2, MaxEFSearch: 2})
	assertNativeVectorHits(t, fallback.Hits, want.Hits)
	if fallback.Plan.Access != "SCAN" || !fallback.Plan.Exact || fallback.Plan.ANNFallback == "" || fallback.Stats.ANNNodeReads == 0 || fallback.Stats.BaseRowFetches <= 180 {
		t.Fatal(fallback)
	}
	limits := vector.DefaultSearchLimits()
	limits.MaxCandidates = 180
	fail, err := db.VectorSearch(context.Background(), "items", "embedding", "[1,1,1]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Limit: 3, Where: "active=1", EFSearch: 2, MaxEFSearch: 2, Limits: limits})
	if !errors.Is(err, vector.ErrLimit) || !reflect.DeepEqual(fail, sqlite.VectorResult{}) {
		t.Fatal(fail, err)
	}
	for _, work := range []sqlite.ANNWorkLimits{{MaxNodeReads: 1}, {MaxDistanceEvaluations: 1}, {MaxIterations: 1}, {MaxBytes: 1}} {
		fail, err = db.VectorSearch(context.Background(), "items", "embedding", "[1,1,1]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Limit: 3, Where: "active=1", EFSearch: 2, ANNLimits: work})
		if err == nil || !reflect.DeepEqual(fail, sqlite.VectorResult{}) {
			t.Fatal(work, fail, err)
		}
	}
}

func TestANNQueryIndexIdentityAndOptions(t *testing.T) {
	db, _ := annQueryFixture(t, 48, false)
	for _, o := range []sqlite.VectorSearchOptions{
		{Mode: sqlite.VectorModeApproximate, Limit: 3, Metric: vector.L1},
		{Mode: sqlite.VectorModeApproximate, Limit: 3, Index: "unknown"},
	} {
		got := annNative(t, db, "[2,2,1]", o)
		if got.Plan.Access != "SCAN" || !got.Plan.Exact || got.Plan.ANNFallback == "" || got.Stats.ANNNodeReads != 0 {
			t.Fatal(got)
		}
	}
	for _, o := range []sqlite.VectorSearchOptions{
		{Mode: "arbitrary", Limit: 1}, {Mode: sqlite.VectorModeApproximate, Limit: 1, EFSearch: -1}, {Mode: sqlite.VectorModeApproximate, Limit: 1, EFSearch: 50, MaxEFSearch: 20}, {Mode: sqlite.VectorModeApproximate, Limit: 1, Probes: 1}, {Mode: sqlite.VectorModeApproximate, Limit: 1, ANNLimits: sqlite.ANNWorkLimits{MaxNodeReads: -1}},
	} {
		got, err := db.VectorSearch(context.Background(), "items", "embedding", "[2,2,1]", o)
		if err == nil || !reflect.DeepEqual(got, sqlite.VectorResult{}) {
			t.Fatal(o, got, err)
		}
	}
	for _, o := range []sqlite.VectorSearchOptions{{Mode: sqlite.VectorModeApproximate, Limit: 0}, {Mode: sqlite.VectorModeApproximate, Limit: 1}} {
		var q any = "[2,2,1]"
		if o.Limit == 1 {
			q = nil
		}
		got := annNative(t, db, q, o)
		if got.Plan.Access != "EMPTY" || got.Stats.ANNNodeReads != 0 || got.Stats.BaseRowFetches != 0 {
			t.Fatal(got)
		}
	}
}

func TestANNQuerySQLAliasesOffsetsAndExplain(t *testing.T) {
	db, _ := annQueryFixture(t, 180, false)
	for _, sql := range []string{
		"SELECT id FROM items ORDER BY embedding <-> '[20,6,1]' LIMIT 5 OFFSET 2",
		"SELECT id FROM items ORDER BY l2_distance('[20,6,1]',embedding) LIMIT 5 OFFSET 2",
		"SELECT id,embedding <-> '[20,6,1]' AS distance FROM items ORDER BY distance LIMIT 5 OFFSET 2",
		"SELECT id,embedding <-> '[20,6,1]' FROM items ORDER BY 2 LIMIT 5 OFFSET 2",
	} {
		got, plan, stats := gramResults(t, db, sql)
		exact, ep, _ := gramResults(t, db, strings.Replace(sql, "FROM items", "FROM items NOT INDEXED", 1))
		if !reflect.DeepEqual(got, exact) || plan.Method != "hnsw" || plan.VectorExact || plan.Access != "SEARCH" || plan.VectorTopK != 7 || stats.ANNExactRescores == 0 || stats.BaseRowFetches >= 180 || ep.Access != "SCAN" || !ep.VectorExact {
			t.Fatal(sql, got, exact, plan, stats)
		}
		explained, p, s := gramResults(t, db, "EXPLAIN QUERY PLAN "+sql)
		if len(explained) != 1 || p.Method != "hnsw" || !strings.Contains(explained[0][3].(string), "approximate_recall=true") || strings.Contains(explained[0][3].(string), "covering_posting_ranges=true") || s.ANNStateReads != 0 || s.ANNNodeReads != 0 || s.ANNDistanceEvaluations != 0 || s.BaseRowFetches != 0 {
			t.Fatal(explained, p, s)
		}
	}
	got, plan, stats := gramResults(t, db, "SELECT id FROM items WHERE active=1 ORDER BY embedding <-> '[1,1,1]' LIMIT 5")
	exact, _, _ := gramResults(t, db, "SELECT id FROM items NOT INDEXED WHERE active=1 ORDER BY embedding <-> '[1,1,1]' LIMIT 5")
	if !reflect.DeepEqual(got, exact) || plan.VectorExact || stats.ANNIterations < 2 || stats.VectorFilteredRows == 0 {
		t.Fatal(got, exact, plan, stats)
	}
}

func TestANNQuerySQLNullsFallbackShapesAndLazyErrors(t *testing.T) {
	db, _ := annQueryFixture(t, 48, true)
	execute(t, db, "INSERT INTO items VALUES(-99,NULL,1,'null')")
	got, plan, stats := gramResults(t, db, "SELECT id FROM items ORDER BY embedding <-> '[1,1,1]' LIMIT 2")
	if got[0][0] != int64(-99) || !plan.VectorExact || plan.Method == "hnsw" || stats.ANNNodeReads != 0 || !strings.Contains(plan.ANNFallback, "NULL-first") {
		t.Fatal(got, plan, stats)
	}
	native := annNative(t, db, "[1,1,1]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Limit: 2})
	if native.Plan.Exact || native.Hits[0].RowID == -99 {
		t.Fatal(native)
	}
	other, _ := annQueryFixture(t, 48, false)
	if err := other.CreateHNSWIndex(context.Background(), "items_cosine", "items", "embedding", sqlite.HNSWIndexOptions{Metric: vector.Cosine}); err != nil {
		t.Fatal(err)
	}
	for _, sql := range []string{
		"SELECT id FROM items WHERE 0 ORDER BY embedding <-> 'invalid' LIMIT 1",
		"SELECT id FROM items WHERE 0 ORDER BY embedding <=> '[0,0,0]' LIMIT 1",
		"SELECT id FROM items ORDER BY embedding <-> 'invalid' LIMIT 0",
	} {
		got, _, _ := gramResults(t, other, sql)
		if len(got) != 0 {
			t.Fatal(sql, got)
		}
	}
	for _, sql := range []string{
		"SELECT id FROM items ORDER BY embedding <-> '[1,1,1]' DESC LIMIT 2",
		"SELECT id FROM items ORDER BY embedding <-> '[1,1,1]',id LIMIT 2",
		"SELECT id FROM items ORDER BY embedding <-> vector('[1,1,1]') LIMIT 2",
		"SELECT id FROM items ORDER BY embedding <-> '[1,1,1]'",
	} {
		_, p, s := gramResults(t, other, sql)
		if p.Method == "hnsw" || s.ANNNodeReads != 0 {
			t.Fatal(sql, p, s)
		}
	}
}

func TestANNQueryAllMetricsAndSignedTies(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,embedding VECTOR(3) NOT NULL,active INTEGER)")
	execute(t, db, "INSERT INTO items VALUES(-9,'[1,2,3]',1),(1,'[2,3,4]',0),(5,'[1,2,3]',1),(8,'[-1,-2,-3]',1),(12,'[1,0,0]',1),(15,'[0,1,0]',1)")
	for _, metric := range []vector.MetricKind{vector.L2, vector.L1, vector.Cosine, vector.NegativeIP} {
		name := "idx_" + string(metric)
		if err := db.CreateHNSWIndex(context.Background(), name, "items", "embedding", sqlite.HNSWIndexOptions{Metric: metric}); err != nil {
			t.Fatal(err)
		}
		exact := annNative(t, db, "[1,2,3]", sqlite.VectorSearchOptions{Metric: metric, Limit: 4, Where: "active=1"})
		got := annNative(t, db, "[1,2,3]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Index: name, Metric: metric, Limit: 4, Where: "active=1"})
		assertNativeVectorHits(t, got.Hits, exact.Hits)
		if got.Plan.Exact || got.Plan.Method != "hnsw" || got.Plan.Metric != metric || got.Stats.FilteredRows != 1 {
			t.Fatal(got)
		}
		op := map[vector.MetricKind]string{vector.L2: "<->", vector.L1: "<+>", vector.Cosine: "<=>", vector.NegativeIP: "<#>"}[metric]
		sql := "SELECT id FROM items WHERE active=1 ORDER BY embedding " + op + " '[1,2,3]' LIMIT 4"
		rows, p, _ := gramResults(t, db, sql)
		want, _, _ := gramResults(t, db, strings.Replace(sql, "FROM items", "FROM items NOT INDEXED", 1))
		if !reflect.DeepEqual(rows, want) || p.Method != "hnsw" || p.VectorMetric != metric {
			t.Fatal(metric, rows, want, p)
		}
	}
}

func TestANNQueryFallbackComponentsAndRuntimeFailure(t *testing.T) {
	db, _ := annQueryFixture(t, 72, false)
	options := sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Limit: 3, Where: "active=1", EFSearch: 2, MaxEFSearch: 2}
	got := annNative(t, db, "[1,1,1]", options)
	if !got.Plan.Exact || got.Stats.ANNDistanceEvaluations == 0 {
		t.Fatal(got)
	}
	limits := vector.DefaultSearchLimits()
	limits.MaxComponents = uint64(got.Stats.ScoredComponents + 3*got.Stats.ANNDistanceEvaluations - 1)
	options.Limits = limits
	failed, err := db.VectorSearch(context.Background(), "items", "embedding", "[1,1,1]", options)
	if !errors.Is(err, vector.ErrLimit) || !reflect.DeepEqual(failed, sqlite.VectorResult{}) {
		t.Fatal(failed, err)
	}
	// A later candidate fails after earlier valid distances. Neither API returns
	// successful partial hits or a Rows object containing the preceding rows.
	options = sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Limit: 5, Where: "ngram_similarity('abc','abc',id%4,'dice')>=0"}
	failed, err = db.VectorSearch(context.Background(), "items", "embedding", "[1,1,1]", options)
	if err == nil || !reflect.DeepEqual(failed, sqlite.VectorResult{}) {
		t.Fatal(failed, err)
	}
	rows, err := db.Query(context.Background(), "SELECT id FROM items WHERE ngram_similarity('abc','abc',id%4,'dice')>=0 ORDER BY embedding <-> '[1,1,1]' LIMIT 5")
	if err == nil || rows != nil {
		if rows != nil {
			rows.Close()
		}
		t.Fatal(rows, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	canceled := &nativeVectorCancelContext{Context: ctx, cancel: cancel}
	failed, err = db.VectorSearch(canceled, "items", "embedding", "[1,1,1]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Limit: 5})
	cancel()
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(failed, sqlite.VectorResult{}) {
		t.Fatal(failed, err)
	}
	// The failed query released its owned snapshot, so ordinary writes and the
	// next successful query remain usable.
	execute(t, db, "UPDATE items SET active=1 WHERE id=1")
	got = annNative(t, db, "[1,1,1]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Limit: 1})
	if got.Hits[0].RowID != 1 {
		t.Fatal(got)
	}
}

func TestANNQuerySmallSourceRetentionLimitAndOffset(t *testing.T) {
	db, _ := writable(t, sqlite.Options{MaxSortBytes: 16 << 10})
	execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,embedding VECTOR(3) NOT NULL,active INTEGER)")
	execute(t, db, "INSERT INTO items VALUES(1,'[1,1,1]',1),(2,'[2,1,1]',0)")
	if err := db.CreateHNSWIndex(context.Background(), "items_hnsw", "items", "embedding"); err != nil {
		t.Fatal(err)
	}
	for _, tail := range []string{"LIMIT 10000", "LIMIT 9999 OFFSET 1", "LIMIT 9998 OFFSET 2"} {
		sql := "SELECT id FROM items ORDER BY embedding <-> '[0,0,0]' " + tail
		got, plan, _ := gramResults(t, db, sql)
		want, _, _ := gramResults(t, db, strings.Replace(sql, "FROM items", "FROM items NOT INDEXED", 1))
		if !reflect.DeepEqual(got, want) || plan.Method != "hnsw" || plan.VectorTopK != 10000 {
			t.Fatal(tail, got, want, plan)
		}
	}
	result := annNative(t, db, "[0,0,0]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Limit: 10000, Where: "active=1"})
	if result.Plan.Limit != 10000 || result.Plan.Exact || len(result.Hits) != 1 || result.Hits[0].RowID != 1 {
		t.Fatal(result)
	}
}
