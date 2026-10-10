package driver_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	_ "github.com/ms-xiaosnow/gosqlite/driver"
)

func TestDriverSimilarityScalarsAndRanking(t *testing.T) {
	db, err := sql.Open("gosqlite", filepath.Join(t.TempDir(), "scores.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	for _, q := range []string{"CREATE TABLE docs(id INTEGER PRIMARY KEY,title TEXT)", "INSERT INTO docs VALUES(1,'数据库'),(2,'数据'),(3,'other'),(4,NULL)", "CREATE INDEX docs_grams ON docs(title) USING grams13"} {
		if _, err = db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	var score, bigram float64
	var distance int64
	if err = db.QueryRow("SELECT ngram_similarity('é','é',1,'dice'),bigram_similarity('数据库','数据库'),levenshtein('kitten','sitting',1)").Scan(&score, &bigram, &distance); err != nil || score != 1 || bigram != 1 || distance != 2 {
		t.Fatalf("score=%g bigram=%g bounded-distance=%d error=%v", score, bigram, distance, err)
	}
	var null sql.NullFloat64
	if err = db.QueryRow("SELECT ngram_similarity(NULL,'value',2,'dice')").Scan(&null); err != nil || null.Valid {
		t.Fatalf("NULL result %+v %v", null, err)
	}
	rows, err := db.Query("SELECT id,ngram_similarity(title,?,2,'dice') AS score FROM docs WHERE score>=? ORDER BY score DESC,id LIMIT ?", "数据库", 0.1, 2)
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for rows.Next() {
		var id int64
		var s float64
		if err = rows.Scan(&id, &s); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	if len(ids) != 2 || ids[0] != 1 || ids[1] != 2 {
		t.Fatalf("ranked ids=%v", ids)
	}
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec("UPDATE docs SET title='changed' WHERE id=1"); err != nil {
		t.Fatal(err)
	}
	if err = tx.QueryRow("SELECT ngram_similarity(title,'数据库',2,'dice') FROM docs WHERE id=1").Scan(&score); err != nil || score != 0 {
		t.Fatalf("own write score=%g error=%v", score, err)
	}
	if err = tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err = db.QueryRow("SELECT ngram_similarity(title,'数据库',2,'dice') FROM docs WHERE id=1").Scan(&score); err != nil || score != 1 {
		t.Fatalf("rollback score=%g error=%v", score, err)
	}
	for _, q := range []string{"SELECT ngram_similarity('a','a',0,'dice')", "SELECT ngram_similarity('a','a',2,'unknown')", "SELECT levenshtein('a','b',-1)"} {
		if err = db.QueryRow(q).Scan(&score); err == nil {
			t.Fatalf("accepted invalid function input: %s", q)
		}
	}
}
