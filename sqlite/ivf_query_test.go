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

func ivfQueryFixture(t *testing.T, nullable bool) *sqlite.DB {
	t.Helper()
	db, _ := writable(t, sqlite.Options{})
	nullability := " NOT NULL"
	if nullable {
		nullability = ""
	}
	execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,embedding VECTOR(3)"+nullability+",active INTEGER,title TEXT)")
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for cluster := range 4 {
		for offset := range 32 {
			id := cluster*32 + offset + 1
			value := fmt.Sprintf("[%f,%f,1]", float64(cluster*1000)+float64(offset)/100, float64(cluster*500)+float64(offset)/200)
			txExecute(t, tx, "INSERT INTO items VALUES(?,?,?,?)", id, value, cluster == 3, fmt.Sprintf("item %d", id))
		}
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err = db.CreateIVFFlatIndex(context.Background(), "items_ivf", "items", "embedding", sqlite.IVFFlatIndexOptions{Lists: 4}); err != nil {
		t.Fatal(err)
	}
	return db
}

func TestIVFQueryRealSelectedListAndExactDefault(t *testing.T) {
	db := ivfQueryFixture(t, false)
	exact := annNative(t, db, "[0,0,1]", sqlite.VectorSearchOptions{Limit: 5})
	if !exact.Plan.Exact || exact.Stats.BaseRowFetches != 128 || exact.Stats.ANNCentroidReads != 0 {
		t.Fatal(exact)
	}
	got := annNative(t, db, "[0,0,1]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Limit: 5})
	assertNativeVectorHits(t, got.Hits, exact.Hits)
	if got.Plan.Method != "ivfflat" || got.Plan.Access != "SEARCH" || got.Plan.Exact || got.Plan.ANNLists != 4 || got.Plan.ANNProbes != 1 || got.Plan.ANNMaxProbes != 4 || got.Plan.ANNEffectiveProbes != 1 || got.Plan.ANNTrainingState != "ready" {
		t.Fatal(got)
	}
	if got.Stats.ANNCentroidReads != 4 || got.Stats.ANNCentroidScores != 4 || got.Stats.ANNListsOpened != 1 || got.Stats.ANNMemberKeysRead != 32 || got.Stats.BaseRowFetches != 32 || got.Stats.ANNExactRescores != 32 || got.Stats.ANNIdentityLookups != 32 || got.Stats.ANNIterations != 1 || got.Stats.ANNNodeReads != 0 || got.Stats.ANNDistanceEvaluations != 0 {
		t.Fatal(got)
	}
	if !strings.Contains(got.Plan.Detail(), "lists=4; probes=1; max_probes=4; effective_probes=1; training_state=ready") {
		t.Fatal(got.Plan.Detail())
	}
	two := annNative(t, db, "[0,0,1]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Limit: 5, Probes: 2})
	assertNativeVectorHits(t, two.Hits, exact.Hits)
	if two.Plan.Exact || two.Plan.ANNProbes != 2 || two.Plan.ANNEffectiveProbes != 2 || two.Stats.ANNListsOpened != 2 || two.Stats.BaseRowFetches != 64 || two.Stats.ANNCentroidScores != 4 || two.Stats.ANNIterations != 1 {
		t.Fatal(two)
	}
	l1 := annNative(t, db, "[0,0,1]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Limit: 5, Metric: vector.L1})
	if !l1.Plan.Exact || l1.Plan.Method != "exact_vector" || l1.Stats.ANNCentroidReads != 0 {
		t.Fatal(l1)
	}
	all := annNative(t, db, "[0,0,1]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Limit: 5, Probes: 4})
	assertNativeVectorHits(t, all.Hits, exact.Hits)
	if !all.Plan.Exact || all.Plan.Access != "SCAN" || all.Plan.Method != "exact_vector" || all.Plan.ANNFallback != "probes equals lists" || all.Stats.ANNCentroidScores != 0 || all.Stats.ANNListsOpened != 0 || all.Stats.BaseRowFetches != 128 {
		t.Fatal(all)
	}
	for _, o := range []sqlite.VectorSearchOptions{{Probes: -1}, {Probes: 5}, {Probes: 2, MaxProbes: 1}, {MaxProbes: 5}, {ANNLimits: sqlite.ANNWorkLimits{MaxMemberKeysRead: -1}}, {ANNLimits: sqlite.ANNWorkLimits{MaxCentroidReads: -1}}} {
		o.Mode, o.Limit = sqlite.VectorModeApproximate, 3
		failed, err := db.VectorSearch(context.Background(), "items", "embedding", "[0,0,1]", o)
		if !errors.Is(err, vector.ErrOptions) || !reflect.DeepEqual(failed, sqlite.VectorResult{}) {
			t.Fatal(o, failed, err)
		}
	}
}

func TestIVFQueryFilterCompensationAndCumulativeFallback(t *testing.T) {
	db := ivfQueryFixture(t, false)
	want := annNative(t, db, "[0,0,1]", sqlite.VectorSearchOptions{Limit: 3, Where: "active=1"})
	o := sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Limit: 3, Where: "active=1"}
	got := annNative(t, db, "[0,0,1]", o)
	assertNativeVectorHits(t, got.Hits, want.Hits)
	if got.Stats.ANNIterations < 2 || got.Stats.FilteredRows == 0 || got.Stats.ANNCentroidReads != 4 || got.Stats.ANNCentroidScores != 4 || got.Stats.ANNListsOpened != 4 || got.Stats.ANNMemberKeysRead != 128 || got.Stats.BaseRowFetches != 128 || got.Plan.ANNEffectiveProbes != 4 {
		t.Fatal(got)
	}
	o.MaxProbes = 1
	fallback := annNative(t, db, "[0,0,1]", o)
	assertNativeVectorHits(t, fallback.Hits, want.Hits)
	if fallback.Plan.Access != "SCAN" || !fallback.Plan.Exact || fallback.Plan.Method != "exact_vector" || fallback.Plan.ANNFallback == "" || fallback.Stats.ANNMemberKeysRead != 32 || fallback.Stats.BaseRowFetches != 160 || fallback.Stats.ANNCentroidScores != 4 || fallback.Plan.ANNEffectiveProbes != 1 {
		t.Fatal(fallback)
	}
	// Exact fallback consumes the same source and component quotas as the
	// abandoned list attempt, including centroid scores and query validation.
	limits := vector.DefaultSearchLimits()
	limits.MaxCandidates = 159
	o.Limits = limits
	failed, err := db.VectorSearch(context.Background(), "items", "embedding", "[0,0,1]", o)
	if !errors.Is(err, vector.ErrLimit) || !reflect.DeepEqual(failed, sqlite.VectorResult{}) {
		t.Fatal(failed, err)
	}
	limits = vector.DefaultSearchLimits()
	limits.MaxComponents = uint64(fallback.Stats.ScoredComponents+3*fallback.Stats.ANNCentroidScores) - 1
	o.Limits = limits
	failed, err = db.VectorSearch(context.Background(), "items", "embedding", "[0,0,1]", o)
	if !errors.Is(err, vector.ErrLimit) || !reflect.DeepEqual(failed, sqlite.VectorResult{}) {
		t.Fatal(failed, err)
	}
	o.Limits = vector.SearchLimits{}
	o.MaxProbes = 0
	o.Where = "active=2"
	none := annNative(t, db, "[0,0,1]", o)
	if len(none.Hits) != 0 || !none.Plan.Exact || none.Stats.ANNListsOpened != 4 || none.Stats.BaseRowFetches != 256 || none.Stats.ANNCentroidScores != 4 || none.Plan.ANNStopReason != "all_lists_exhausted" {
		t.Fatal(none)
	}
	for _, work := range []sqlite.ANNWorkLimits{{MaxCentroidReads: 3}, {MaxDistanceEvaluations: 3}, {MaxMemberKeysRead: 31}, {MaxIterations: 1}, {MaxBytes: 1}} {
		o.Where, o.ANNLimits = "active=1", work
		failed, err = db.VectorSearch(context.Background(), "items", "embedding", "[0,0,1]", o)
		if err == nil || !reflect.DeepEqual(failed, sqlite.VectorResult{}) {
			t.Fatal(work, failed, err)
		}
	}
}

func TestIVFQuerySQLAliasesOffsetsExplainAndLaziness(t *testing.T) {
	db := ivfQueryFixture(t, false)
	for _, sql := range []string{
		"SELECT id FROM items ORDER BY embedding <-> '[0,0,1]' LIMIT 5 OFFSET 2",
		"SELECT id FROM items AS i ORDER BY l2_distance('[0,0,1]',i.embedding) LIMIT 5 OFFSET 2",
		"SELECT id,embedding <-> '[0,0,1]' AS distance FROM items ORDER BY distance LIMIT 5 OFFSET 2",
		"SELECT id,embedding <-> '[0,0,1]' FROM items ORDER BY 2 LIMIT 5 OFFSET 2",
	} {
		got, plan, stats := gramResults(t, db, sql)
		exactSQL := strings.Replace(sql, "FROM items AS i", "FROM items AS i NOT INDEXED", 1)
		if exactSQL == sql {
			exactSQL = strings.Replace(sql, "FROM items", "FROM items NOT INDEXED", 1)
		}
		want, exactPlan, _ := gramResults(t, db, exactSQL)
		if !reflect.DeepEqual(got, want) || plan.Method != "ivfflat" || plan.VectorExact || plan.VectorTopK != 7 || stats.ANNCentroidScores != 4 || stats.BaseRowFetches != 32 || stats.ANNExactRescores != 32 || exactPlan.Method == "ivfflat" {
			t.Fatal(sql, got, want, plan, stats)
		}
		explained, p, s := gramResults(t, db, "EXPLAIN QUERY PLAN "+sql)
		if len(explained) != 1 || p.Method != "ivfflat" || !strings.Contains(explained[0][3].(string), "approximate_recall=true") || s.ANNCentroidReads != 0 || s.ANNCentroidScores != 0 || s.ANNListsOpened != 0 || s.ANNMemberKeysRead != 0 || s.BaseRowFetches != 0 {
			t.Fatal(explained, p, s)
		}
	}
	for _, sql := range []string{
		"SELECT id FROM items WHERE 0 ORDER BY embedding <-> 'invalid' LIMIT 1",
		"SELECT id FROM items ORDER BY embedding <-> 'invalid' LIMIT 0",
	} {
		got, _, stats := gramResults(t, db, sql)
		if len(got) != 0 || stats.ANNCentroidReads != 0 || stats.ANNCentroidScores != 0 || stats.ANNMemberKeysRead != 0 {
			t.Fatal(sql, got, stats)
		}
	}
	for _, sql := range []string{
		"SELECT id FROM items ORDER BY embedding <-> '[0,0,1]' DESC LIMIT 2",
		"SELECT id FROM items ORDER BY embedding <-> '[0,0,1]',id LIMIT 2",
		"SELECT id FROM items ORDER BY embedding <-> vector('[0,0,1]') LIMIT 2",
		"SELECT id FROM items ORDER BY embedding <+> '[0,0,1]' LIMIT 2",
	} {
		_, p, s := gramResults(t, db, sql)
		if p.Method == "ivfflat" || s.ANNCentroidReads != 0 || s.ANNMemberKeysRead != 0 {
			t.Fatal(sql, p, s)
		}
	}
	got, p, s := gramResults(t, db, "SELECT id FROM items WHERE active=1 ORDER BY embedding <-> '[0,0,1]' LIMIT 3 OFFSET 2")
	want, _, _ := gramResults(t, db, "SELECT id FROM items NOT INDEXED WHERE active=1 ORDER BY embedding <-> '[0,0,1]' LIMIT 3 OFFSET 2")
	if !reflect.DeepEqual(got, want) || p.Method != "ivfflat" || s.ANNIterations < 2 || s.VectorFilteredRows == 0 || s.ANNCentroidScores != 4 {
		t.Fatal(got, want, p, s)
	}
}

func TestIVFQueryNullsAndUntrainedFallback(t *testing.T) {
	db := ivfQueryFixture(t, true)
	execute(t, db, "INSERT INTO items VALUES(-99,NULL,1,'null')")
	rows, plan, stats := gramResults(t, db, "SELECT id FROM items ORDER BY embedding <-> '[0,0,1]' LIMIT 2")
	if rows[0][0] != int64(-99) || !plan.VectorExact || plan.Method == "ivfflat" || stats.ANNCentroidReads != 0 || !strings.Contains(plan.ANNFallback, "NULL-first") {
		t.Fatal(rows, plan, stats)
	}
	got := annNative(t, db, "[0,0,1]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Limit: 2})
	if got.Plan.Exact || got.Hits[0].RowID == -99 {
		t.Fatal(got)
	}
	other, _ := writable(t, sqlite.Options{})
	execute(t, other, "CREATE TABLE items(id INTEGER PRIMARY KEY,embedding VECTOR(3) NOT NULL)")
	execute(t, other, "INSERT INTO items VALUES(1,'[1,1,1]'),(2,'[2,1,1]')")
	if err := other.CreateIVFFlatIndex(context.Background(), "untrained", "items", "embedding", sqlite.IVFFlatIndexOptions{Lists: 4}); err != nil {
		t.Fatal(err)
	}
	for _, prefix := range []string{"", "EXPLAIN QUERY PLAN "} {
		_, p, s := gramResults(t, other, prefix+"SELECT id FROM items ORDER BY embedding <-> '[0,0,0]' LIMIT 1")
		if p.Method != "exact_vector" || !p.VectorExact || p.ANNTrainingState != "untrained" || !strings.Contains(p.ANNFallback, "REINDEX") || s.ANNCentroidReads != 0 || s.ANNMemberKeysRead != 0 {
			t.Fatal(p, s)
		}
	}
	u := annNative(t, other, "[0,0,0]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Limit: 1})
	if !u.Plan.Exact || u.Plan.ANNTrainingState != "untrained" || u.Stats.ANNCentroidReads != 0 {
		t.Fatal(u)
	}
}

func TestIVFQueryMetricsOriginalIPAndSignedTies(t *testing.T) {
	for _, metric := range []vector.MetricKind{vector.L2, vector.Cosine, vector.NegativeIP} {
		t.Run(string(metric), func(t *testing.T) {
			db, _ := writable(t, sqlite.Options{})
			execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,embedding VECTOR(3) NOT NULL)")
			for i, v := range []string{"[1,0,1]", "[2,0,2]", "[3,0,3]", "[100,0,100]", "[0,1,1]", "[0,2,2]", "[0,3,3]", "[0,4,4]"} {
				execute(t, db, "INSERT INTO items VALUES(?,?)", i-4, v)
			}
			if err := db.CreateIVFFlatIndex(context.Background(), "metric", "items", "embedding", sqlite.IVFFlatIndexOptions{Metric: metric, Lists: 2}); err != nil {
				t.Fatal(err)
			}
			q := "[1,0,1]"
			got := annNative(t, db, q, sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Metric: metric, Limit: 2})
			want := annNative(t, db, q, sqlite.VectorSearchOptions{Metric: metric, Limit: 2})
			assertNativeVectorHits(t, got.Hits, want.Hits)
			if got.Plan.Exact || got.Stats.ANNCentroidScores != 2 || got.Stats.ANNExactRescores == 0 {
				t.Fatal(got)
			}
			if metric == vector.NegativeIP && (got.Hits[0].RowID != -1 || got.Hits[0].Distance != -200) {
				t.Fatal("IP must rescore original magnitudes", got)
			}
			if metric == vector.Cosine && (got.Hits[0].RowID != -4 || got.Hits[1].RowID != -3) {
				t.Fatal("cosine signed rowid ties", got)
			}
		})
	}
}

func TestIVFQueryFailureHasNoPartialResult(t *testing.T) {
	db := ivfQueryFixture(t, false)
	badWhere := "ngram_similarity('abc','abc',id%4,'dice')>=0"
	failed, err := db.VectorSearch(context.Background(), "items", "embedding", "[0,0,1]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Limit: 5, Where: badWhere})
	if err == nil || !reflect.DeepEqual(failed, sqlite.VectorResult{}) {
		t.Fatal(failed, err)
	}
	rows, err := db.Query(context.Background(), "SELECT id FROM items WHERE "+badWhere+" ORDER BY embedding <-> '[0,0,1]' LIMIT 5")
	if err == nil || rows != nil {
		if rows != nil {
			rows.Close()
		}
		t.Fatal(rows, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	canceled := &nativeVectorCancelContext{Context: ctx, cancel: cancel}
	failed, err = db.VectorSearch(canceled, "items", "embedding", "[0,0,1]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Limit: 5})
	cancel()
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(failed, sqlite.VectorResult{}) {
		t.Fatal(failed, err)
	}
	execute(t, db, "UPDATE items SET active=1 WHERE id=1")
	got := annNative(t, db, "[0,0,1]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Limit: 1})
	if got.Hits[0].RowID != 1 {
		t.Fatal(got)
	}
}

func TestIVFQuerySmallSourceRetentionAndSingleListExact(t *testing.T) {
	db, _ := writable(t, sqlite.Options{MaxSortBytes: 16 << 10})
	execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,embedding VECTOR(3) NOT NULL)")
	execute(t, db, "INSERT INTO items VALUES(1,'[1,1,1]'),(2,'[100,1,1]')")
	if err := db.CreateIVFFlatIndex(context.Background(), "small", "items", "embedding", sqlite.IVFFlatIndexOptions{Lists: 2}); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"LIMIT 10000", "LIMIT 9999 OFFSET 1", "LIMIT 9998 OFFSET 2"} {
		sql := "SELECT id FROM items ORDER BY embedding <-> '[0,0,0]' " + suffix
		got, p, _ := gramResults(t, db, sql)
		want, _, _ := gramResults(t, db, strings.Replace(sql, "FROM items", "FROM items NOT INDEXED", 1))
		if !reflect.DeepEqual(got, want) || p.Method != "ivfflat" || p.VectorTopK != 10000 {
			t.Fatal(got, want, p)
		}
	}
	got := annNative(t, db, "[0,0,0]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Limit: 10000})
	if len(got.Hits) != 2 || got.Plan.Exact || got.Stats.ANNListsOpened != 2 {
		t.Fatal(got)
	}
	filtered := annNative(t, db, "[0,0,0]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Limit: 10000, Where: "id=1"})
	if !filtered.Plan.Exact || len(filtered.Hits) != 1 || filtered.Hits[0].RowID != 1 || filtered.Stats.BaseRowFetches != 4 {
		t.Fatal(filtered)
	}
	other, _ := writable(t, sqlite.Options{})
	execute(t, other, "CREATE TABLE items(id INTEGER PRIMARY KEY,embedding VECTOR(3) NOT NULL)")
	execute(t, other, "INSERT INTO items VALUES(1,'[1,1,1]')")
	if err := other.CreateIVFFlatIndex(context.Background(), "one", "items", "embedding", sqlite.IVFFlatIndexOptions{Lists: 1}); err != nil {
		t.Fatal(err)
	}
	for _, prefix := range []string{"", "EXPLAIN QUERY PLAN "} {
		_, p, s := gramResults(t, other, prefix+"SELECT id FROM items ORDER BY embedding <-> '[0,0,0]' LIMIT 1")
		if p.Method != "exact_vector" || !p.VectorExact || p.ANNFallback != "probes equals lists" || s.ANNCentroidReads != 0 || s.ANNListsOpened != 0 {
			t.Fatal(p, s)
		}
	}
}

func TestIVFQueryZeroIPAndCosineQueries(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,embedding VECTOR(3) NOT NULL)")
	for i, v := range []string{"[1,0,1]", "[2,0,2]", "[3,0,3]", "[4,0,4]", "[0,1,1]", "[0,2,2]", "[0,3,3]", "[0,4,4]"} {
		execute(t, db, "INSERT INTO items VALUES(?,?)", i-4, v)
	}
	for _, metric := range []vector.MetricKind{vector.NegativeIP, vector.Cosine} {
		if err := db.CreateIVFFlatIndex(context.Background(), string(metric), "items", "embedding", sqlite.IVFFlatIndexOptions{Metric: metric, Lists: 2}); err != nil {
			t.Fatal(err)
		}
	}
	for _, mode := range []sqlite.VectorSearchMode{sqlite.VectorModeExact, sqlite.VectorModeApproximate} {
		got := annNative(t, db, "[0,0,0]", sqlite.VectorSearchOptions{Mode: mode, Metric: vector.NegativeIP, Limit: 3})
		if len(got.Hits) != 3 {
			t.Fatal(got)
		}
		for i, hit := range got.Hits {
			if hit.Distance != 0 || (i > 0 && got.Hits[i-1].RowID >= hit.RowID) {
				t.Fatal("zero IP query must preserve zero distances and signed identity ties", got)
			}
		}
		if mode == sqlite.VectorModeApproximate && (got.Plan.Exact || got.Stats.ANNCentroidScores != 2 || got.Stats.ANNListsOpened != 1 || got.Stats.BaseRowFetches != 4) {
			t.Fatal(got)
		}
		failed, err := db.VectorSearch(context.Background(), "items", "embedding", "[0,0,0]", sqlite.VectorSearchOptions{Mode: mode, Metric: vector.Cosine, Limit: 3})
		if !errors.Is(err, vector.ErrZeroNorm) || !reflect.DeepEqual(failed, sqlite.VectorResult{}) {
			t.Fatal(failed, err)
		}
	}
	rows, plan, stats := gramResults(t, db, "SELECT id,embedding <#> '[0,0,0]' AS distance FROM items ORDER BY distance LIMIT 3")
	if plan.Method != "ivfflat" || stats.ANNCentroidScores != 2 || len(rows) != 3 {
		t.Fatal(rows, plan, stats)
	}
	for i, row := range rows {
		if row[1] != float64(0) || (i > 0 && rows[i-1][0].(int64) >= row[0].(int64)) {
			t.Fatal(rows)
		}
	}
	rows, _, stats = gramResults(t, db, "SELECT id FROM items WHERE 0 ORDER BY embedding <=> '[0,0,0]' LIMIT 3")
	if len(rows) != 0 || stats.ANNCentroidScores != 0 || stats.ANNMemberKeysRead != 0 {
		t.Fatal(rows, stats)
	}
	failedRows, err := db.Query(context.Background(), "SELECT id FROM items ORDER BY embedding <=> '[0,0,0]' LIMIT 3")
	if !errors.Is(err, vector.ErrZeroNorm) || failedRows != nil {
		if failedRows != nil {
			failedRows.Close()
		}
		t.Fatal(failedRows, err)
	}
}
