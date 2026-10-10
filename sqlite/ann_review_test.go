package sqlite_test

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Ordinary transactional operations only: no raw-file mutation, injected I/O,
// recovery, crash, or subprocess behavior is involved in these review tests.
func TestANNReviewReadOwnWritesAndRollback(t *testing.T) {
	ctx := context.Background()
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,v VECTOR(2) NOT NULL,body TEXT)")
	execute(t, db, "INSERT INTO items VALUES(1,'[1,1]','数据库'),(2,'[2,1]','数据库'),(3,'[3,1]','other')")
	if err := db.CreateGramsIndex(ctx, "words", "items", []string{"body"}); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateHNSWIndex(ctx, "knn", "items", "v"); err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	txExecute(t, tx, "UPDATE items SET v='[20,1]',body='other' WHERE id=1")
	txExecute(t, tx, "DELETE FROM items WHERE id=2")
	txExecute(t, tx, "INSERT INTO items VALUES(-7,'[0,1]','数据库')")
	txExecute(t, tx, "INSERT INTO items VALUES(2,'[0,1]','数据库')")
	check := func(stage string) {
		t.Helper()
		got, err := tx.VectorSearch(ctx, "items", "v", "[0,1]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Limit: 3, Where: "body LIKE '%数据库%'"})
		if err != nil {
			t.Fatalf("%s: %v", stage, err)
		}
		if got.Plan.Exact || !reflect.DeepEqual(got.Hits, []vector.Hit{{RowID: -7}, {RowID: 2}}) {
			t.Fatalf("%s: %+v", stage, got)
		}
		query := "SELECT id,body FROM items WHERE body LIKE '%数据库%' ORDER BY v <-> '[0,1]' LIMIT 3"
		rows, plan, _ := gramResults(t, tx, query)
		exact, _, _ := gramResults(t, tx, strings.Replace(query, "FROM items", "FROM items NOT INDEXED", 1))
		if plan.Method != "hnsw" || !reflect.DeepEqual(rows, exact) {
			t.Fatal(stage, rows, exact, plan)
		}
		for _, name := range []string{"words", "knn"} {
			if err := tx.CheckExtensionIndex(ctx, name); err != nil {
				t.Fatal(stage, err)
			}
		}
	}
	check("mutated transaction")
	if err = tx.Reindex(ctx, "knn"); err != nil {
		t.Fatal(err)
	}
	check("reindexed transaction")
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	got, err := db.VectorSearch(ctx, "items", "v", "[0,1]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Limit: 3})
	if err != nil || len(got.Hits) != 3 || got.Hits[0].RowID != 1 {
		t.Fatal(got, err)
	}
}

func TestANNReviewSignedExtremaTies(t *testing.T) {
	ctx := context.Background()
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,v VECTOR(2) NOT NULL)")
	for _, id := range []int64{math.MaxInt64, 0, -1, math.MinInt64, 7} {
		execute(t, db, "INSERT INTO items VALUES(?,'[1,1]')", id)
	}
	if err := db.CreateHNSWIndex(ctx, "knn", "items", "v"); err != nil {
		t.Fatal(err)
	}
	got, err := db.VectorSearch(ctx, "items", "v", "[0,0]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Limit: 5})
	if err != nil {
		t.Fatal(err)
	}
	want := []int64{math.MinInt64, -1, 0, 7, math.MaxInt64}
	for i, hit := range got.Hits {
		if hit.RowID != want[i] {
			t.Fatal(got.Hits)
		}
	}
	if len(got.Hits) != len(want) {
		t.Fatal(got.Hits)
	}
	rows, _, _ := gramResults(t, db, "SELECT id FROM items ORDER BY v <-> '[0,0]' LIMIT 3 OFFSET 1")
	if !reflect.DeepEqual(rows, [][]any{{int64(-1)}, {int64(0)}, {int64(7)}}) {
		t.Fatal(rows)
	}
}

func TestANNReviewSQLSmallTableLargeLimitMemory(t *testing.T) {
	ctx := context.Background()
	db, _ := writable(t, sqlite.Options{MaxSortBytes: 16 << 10})
	execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,v VECTOR(2) NOT NULL)")
	execute(t, db, "INSERT INTO items VALUES(1,'[1,1]'),(2,'[2,1]')")
	if err := db.CreateHNSWIndex(ctx, "knn", "items", "v"); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{"LIMIT 1", "LIMIT 10000", "LIMIT 1 OFFSET 9999", "LIMIT 0 OFFSET 9223372036854775807"} {
		q := fmt.Sprintf("SELECT id FROM items ORDER BY v <-> '[0,0]' %s", suffix)
		want, _, _ := gramResults(t, db, strings.Replace(q, "FROM items", "FROM items NOT INDEXED", 1))
		got, _, _ := gramResults(t, db, q)
		if !reflect.DeepEqual(got, want) {
			t.Fatal(got, want)
		}
	}
}

func TestANNReviewEmptyIndexAndAllFilteredSQL(t *testing.T) {
	ctx := context.Background()
	db, _ := writable(t, sqlite.Options{MaxSortBytes: 16 << 10})
	execute(t, db, "CREATE TABLE items(id INTEGER PRIMARY KEY,v VECTOR(2) NOT NULL)")
	if err := db.CreateHNSWIndex(ctx, "knn", "items", "v"); err != nil {
		t.Fatal(err)
	}
	for _, suffix := range []string{
		"ORDER BY v <-> '[0,0]' LIMIT 10000",
		"ORDER BY v <-> 'invalid' LIMIT 1",
		"ORDER BY v <-> NULL LIMIT 1",
		"ORDER BY v <-> '[0,0]' LIMIT 1 OFFSET 9999",
	} {
		rows, _, stats := gramResults(t, db, "SELECT id FROM items "+suffix)
		if len(rows) != 0 || stats.BaseRowFetches != 0 || stats.ANNNodeReads != 0 {
			t.Fatal(suffix, rows, stats)
		}
	}
	execute(t, db, "INSERT INTO items VALUES(1,'[1,1]'),(2,'[2,1]')")
	for _, q := range []string{"'[0,0]'", "'invalid'", "NULL"} {
		rows, _, _ := gramResults(t, db, "SELECT id FROM items WHERE 0 ORDER BY v <-> "+q+" LIMIT 1")
		if len(rows) != 0 {
			t.Fatal(q, rows)
		}
	}
	execute(t, db, "DELETE FROM items")
	got, err := db.VectorSearch(ctx, "items", "v", "[0,0]", sqlite.VectorSearchOptions{Mode: sqlite.VectorModeApproximate, Limit: 10000})
	if err != nil || len(got.Hits) != 0 || got.Stats.ANNNodeReads != 0 {
		t.Fatal(got, err)
	}
	rows, _, stats := gramResults(t, db, "SELECT id FROM items ORDER BY v <-> '[0,0]' LIMIT 1 OFFSET 9999")
	if len(rows) != 0 || stats.ANNNodeReads != 0 {
		t.Fatal(rows, stats)
	}
}
