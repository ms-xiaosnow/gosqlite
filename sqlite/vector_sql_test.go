package sqlite_test

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestVectorSQLRankShapesNullOffsetAndTies(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY, embedding VECTOR(2), active INTEGER, title TEXT)")
	execute(t, db, "INSERT INTO items VALUES(5,'[1,0]',1,'数据库'),(-9,'[1,0]',1,'数据库'),(2,NULL,1,'数据库'),(7,'[9,0]',1,'other'),(1,'[0,0]',0,'other')")
	for _, query := range []string{
		"SELECT id FROM items WHERE active=1 ORDER BY embedding <-> '[1,0]' LIMIT 3",
		"SELECT id FROM items WHERE active=1 ORDER BY l2_distance(embedding,'[1,0]') LIMIT 3",
		"SELECT id FROM items WHERE active=1 ORDER BY '[1,0]' <-> embedding LIMIT 3",
	} {
		got, plan, stats := gramResults(t, db, query)
		if !reflect.DeepEqual(got, [][]any{{int64(2)}, {int64(-9)}, {int64(5)}}) {
			t.Fatalf("%s: %v", query, got)
		}
		if plan.Access != "SCAN" || plan.VectorRanking != "bounded_heap" || !plan.VectorExact || plan.VectorMetric != vector.L2 || plan.VectorTopK != 3 {
			t.Fatal(plan)
		}
		if stats.BaseRowFetches != 5 || stats.VectorRankedRows != 3 || stats.VectorFilteredRows != 1 || stats.VectorNullRows != 1 || stats.VectorRankComponents != 8 || stats.ReturnedRows != 3 {
			t.Fatal(stats)
		}
	}
	got, plan, _ := gramResults(t, db, "SELECT id,l2_distance(embedding,'[1,0]') AS distance FROM items WHERE active=1 ORDER BY distance LIMIT 2 OFFSET 1")
	if !reflect.DeepEqual(got, [][]any{{int64(-9), float64(0)}, {int64(5), float64(0)}}) || plan.VectorTopK != 3 {
		t.Fatal(got, plan)
	}
	got, plan, _ = gramResults(t, db, "SELECT id,embedding <-> '[1,0]' FROM items WHERE active=1 ORDER BY 2 LIMIT 2 OFFSET 1")
	if !reflect.DeepEqual(got, [][]any{{int64(-9), float64(0)}, {int64(5), float64(0)}}) || plan.VectorRanking != "bounded_heap" {
		t.Fatal(got, plan)
	}
	// Secondary ordering, DESC and computed query operands retain general SQL.
	for _, query := range []string{
		"SELECT id FROM items WHERE active=1 ORDER BY embedding <-> '[1,0]',id DESC LIMIT 3",
		"SELECT id FROM items WHERE active=1 ORDER BY embedding <-> '[1,0]' DESC LIMIT 3",
		"SELECT id FROM items WHERE active=1 ORDER BY embedding <-> vector('[1,0]') LIMIT 3",
		"SELECT id FROM items WHERE active=1 ORDER BY embedding <-> '[1,0]'",
	} {
		_, plan, _ := gramResults(t, db, query)
		if plan.VectorRanking != "" {
			t.Fatalf("unsound shape optimized: %s %+v", query, plan)
		}
	}
	got, _, _ = gramResults(t, db, "SELECT id FROM items WHERE active=1 ORDER BY embedding <-> '[1,0]',id DESC LIMIT 3")
	if !reflect.DeepEqual(got, [][]any{{int64(2)}, {int64(5)}, {int64(-9)}}) {
		t.Fatal("secondary DESC lost", got)
	}
	// NULL scalar semantics remain lazy even if the other input is invalid text.
	got, plan, stats := gramResults(t, db, "SELECT id FROM items WHERE id=2 ORDER BY embedding <-> 'invalid' LIMIT 1")
	if !reflect.DeepEqual(got, [][]any{{int64(2)}}) || plan.VectorRanking == "" || stats.VectorRankedRows != 0 {
		t.Fatal(got, plan, stats)
	}
	for _, query := range []string{
		"SELECT id FROM items WHERE 0 ORDER BY embedding <-> 'invalid' LIMIT 1",
		"SELECT id FROM items ORDER BY embedding <-> 'invalid' LIMIT 0 OFFSET 9223372036854775807",
	} {
		got, _, stats = gramResults(t, db, query)
		if len(got) != 0 || stats.VectorRankedRows != 0 {
			t.Fatal(got, stats)
		}
	}
	// Business filtering excludes the valid zero-norm row before cosine scoring.
	got, _, _ = gramResults(t, db, "SELECT id FROM items WHERE active=1 AND embedding IS NOT NULL ORDER BY embedding <=> '[1,0]' LIMIT 2")
	if !reflect.DeepEqual(got, [][]any{{int64(-9)}, {int64(5)}}) {
		t.Fatal(got)
	}
	if rows, err := db.Query(context.Background(), "SELECT id FROM items ORDER BY embedding <=> '[1,0]' LIMIT 2"); !errors.Is(err, vector.ErrZeroNorm) {
		if rows != nil {
			rows.Close()
		}
		t.Fatal("zero norm hidden", err)
	}
}

func TestVectorSQLBoundedHeapAgainstGeneralSort(t *testing.T) {
	db, path := writable(t, sqlite.Options{MaxSortBytes: 8192})
	execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,embedding VECTOR(3),active INTEGER)")
	rng := rand.New(rand.NewPCG(641, 191))
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for i := int64(-70); i < 70; i++ {
		values := []float32{float32(rng.IntN(31) - 15), float32(rng.IntN(31) - 15), float32(rng.IntN(31) - 15)}
		v, err := vector.New(values)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(context.Background(), "INSERT INTO items VALUES(?,?,?)", i, v, i%3); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	referenceDB, err := sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer referenceDB.Close()
	for _, op := range []string{"<->", "<+>", "<#>", "<=>"} {
		query := "SELECT id FROM items WHERE active!=0 ORDER BY embedding " + op + " '[2,3,4]' LIMIT 5 OFFSET 2"
		got, plan, stats := gramResults(t, db, query)
		if len(got) != 5 || plan.VectorTopK != 7 || stats.BaseRowFetches != 140 {
			t.Fatal(got, plan, stats)
		}
		// Native uses the independently tested mathematical reference path.
		metric := map[string]vector.MetricKind{"<->": vector.L2, "<+>": vector.L1, "<#>": vector.NegativeIP, "<=>": vector.Cosine}[op]
		result, err := referenceDB.VectorSearch(context.Background(), "items", "embedding", "[2,3,4]", sqlite.VectorSearchOptions{Metric: metric, Limit: 7, Where: "active!=0"})
		if err != nil {
			t.Fatal(err)
		}
		for i, row := range got {
			if row[0] != result.Hits[i+2].RowID {
				t.Fatal(op, got, result.Hits)
			}
		}

	}
	if rows, err := db.Query(context.Background(), "SELECT id FROM items ORDER BY embedding <-> '[2,3,4]',id LIMIT 5"); !errors.Is(err, sqlite.ErrMemoryLimit) {
		if rows != nil {
			rows.Close()
		}
		t.Fatal("reference full materialization should exceed tiny budget", err)
	}
}

func TestVectorSQLHybridActualAccessAndExplain(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY,body TEXT,embedding VECTOR(2),active INTEGER)")
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 60; i++ {
		text := "ordinary"
		if i <= 4 {
			text = "数据库查询"
		}
		if _, err = tx.Exec(context.Background(), "INSERT INTO docs VALUES(?,?,?,?)", i, text, fmt.Sprintf("[%d,1]", i), i%2); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	execute(t, db, "CREATE INDEX docs_grams ON docs(body) USING grams13")
	query := "SELECT id,bigram_similarity(body,'数据库') AS text_score,embedding <-> '[1,1]' AS distance FROM docs WHERE body LIKE '%数据库%' AND active=1 ORDER BY distance LIMIT 2"
	got, plan, stats := gramResults(t, db, query)
	if len(got) != 2 || got[0][0] != int64(1) || got[1][0] != int64(3) || plan.Access != "SEARCH" || plan.Method != "grams13" || plan.VectorMetric != vector.L2 || plan.VectorRanking != "bounded_heap" {
		t.Fatal(got, plan)
	}
	if stats.BaseRowFetches != 4 || stats.VectorFilteredRows != 2 || stats.VectorRankedRows != 2 || stats.PostingKeysRead == 0 {
		t.Fatal(stats)
	}
	explained, ep, es := gramResults(t, db, "EXPLAIN QUERY PLAN "+query)
	if len(explained) != 1 || ep.VectorRanking != "bounded_heap" || !strings.Contains(explained[0][3].(string), "vector_metric=l2") || es.BaseRowFetches != 0 || es.VectorRankedRows != 0 {
		t.Fatal(explained, ep, es)
	}
}
