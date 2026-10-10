package sqlite_test

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/similarity"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestSimilaritySQLScalarsAndScan(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	got, _, _ := gramResults(t, db, "SELECT bigram_similarity('abc','abd'), ngram_similarity('abc','abd',2,'jaccard'), levenshtein('kitten','sitting',1), levenshtein('é','é'), bigram_similarity(NULL,'a'), levenshtein('a','b',NULL)")
	want := [][]any{{.5, 1.0 / 3, int64(2), int64(0), nil, nil}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scalar results %#v; want %#v", got, want)
	}
	execute(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY,body TEXT)")
	for i, text := range []any{"abc", "abd", "xyz", "", nil, "a", "éx", "éx", "👩‍💻x"} {
		execute(t, db, "INSERT INTO docs VALUES(?,?)", int64(i+1), text)
	}
	execute(t, db, "CREATE INDEX docs_grams ON docs(body) USING grams13")
	for _, query := range []string{
		"SELECT id,bigram_similarity(body,'abc') AS score FROM docs WHERE score>=.5 ORDER BY score DESC,id",
		"SELECT id,bigram_similarity(body,'abc') AS score FROM docs WHERE bigram_similarity(body,'abc')>=.5 ORDER BY bigram_similarity(body,'abc') DESC,id",
	} {
		got, plan, stats := gramResults(t, db, query)
		if !reflect.DeepEqual(got, [][]any{{int64(1), float64(1)}, {int64(2), .5}}) || plan.Access != "SCAN" || stats.BaseRowFetches != 9 {
			t.Fatalf("%s: got %#v plan=%+v stats=%+v", query, got, plan, stats)
		}
	}
	got, plan, _ := gramResults(t, db, "EXPLAIN QUERY PLAN SELECT id FROM docs WHERE ngram_similarity(body,'abc',2,'dice')>.5 ORDER BY bigram_similarity(body,'abc') DESC")
	if plan.Access != "SCAN" || len(got) != 1 || !strings.HasPrefix(got[0][3].(string), "SCAN docs;") {
		t.Fatalf("false similarity acceleration claim: %#v %+v", got, plan)
	}
	got, _, _ = gramResults(t, db, "SELECT id FROM docs ORDER BY id LIMIT levenshtein('a','abc') OFFSET levenshtein('a','ab')")
	if !reflect.DeepEqual(got, [][]any{{int64(2)}, {int64(3)}}) {
		t.Fatal("similarity LIMIT/OFFSET", got)
	}
	got, _, _ = gramResults(t, db, "SELECT id FROM docs WHERE body LIKE '%' || levenshtein('a','b') || '%' ORDER BY id")
	if len(got) != 0 {
		t.Fatal("similarity LIKE constant expression", got)
	}
	// Validation runs even when there are no source rows or LIMIT is zero.
	for _, query := range []string{"SELECT bigram_similarity(body) FROM docs LIMIT 0", "SELECT levenshtein(body) FROM docs WHERE 0", "SELECT ngram_similarity(body,'x',2) FROM docs"} {
		if rows, err := db.Query(context.Background(), query); err == nil {
			rows.Close()
			t.Fatalf("accepted bad arity: %s", query)
		}
	}
}

func TestSimilaritySQLDML(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY,body TEXT,score REAL)")
	execute(t, db, "INSERT INTO docs VALUES(1,'abc',bigram_similarity('abc','abc')),(2,'abd',ngram_similarity('abc','abd',2,'dice')),(3,'xyz',levenshtein('abc','xyz'))")
	execute(t, db, "CREATE INDEX docs_grams ON docs(body) USING grams13")
	execute(t, db, "UPDATE docs SET score=ngram_similarity(body,'abc',2,'jaccard') WHERE levenshtein(body,'abc')<=1")
	got, _, _ := gramResults(t, db, "SELECT id,score FROM docs ORDER BY id")
	if !reflect.DeepEqual(got, [][]any{{int64(1), float64(1)}, {int64(2), 1.0 / 3}, {int64(3), float64(3)}}) {
		t.Fatal(got)
	}
	execute(t, db, "DELETE FROM docs WHERE bigram_similarity(body,'abc')=0")
	got, _, _ = gramResults(t, db, "SELECT id FROM docs ORDER BY id")
	if !reflect.DeepEqual(got, [][]any{{int64(1)}, {int64(2)}}) {
		t.Fatal(got)
	}
	if _, err := db.Exec(context.Background(), "UPDATE docs SET score=levenshtein(body,?,1)", strings.Repeat("a", 4097)); !errors.Is(err, similarity.ErrLimit) {
		t.Fatalf("distance limit lost: %v", err)
	}
	got, _, _ = gramResults(t, db, "SELECT id,score FROM docs ORDER BY id")
	if !reflect.DeepEqual(got, [][]any{{int64(1), float64(1)}, {int64(2), 1.0 / 3}}) {
		t.Fatal("failed scalar changed rows", got)
	}
}

// This context cancels after normal cooperative checks, making cancellation
// during a substantial scalar deterministic without timers or extra goroutines.
// Its Done/Err state comes from an ordinary cancelable parent context.
type similaritySQLCancelContext struct {
	context.Context
	cancel context.CancelFunc
	checks atomic.Int64
}

func (c *similaritySQLCancelContext) Err() error {
	if c.checks.Add(1) == 128 {
		c.cancel()
	}
	return c.Context.Err()
}

func similaritySQLContext(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return &similaritySQLCancelContext{Context: ctx, cancel: cancel}
}

func TestSimilaritySQLCancellationContexts(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY,body TEXT,score INTEGER)")
	execute(t, db, "INSERT INTO docs VALUES(1,'abc',0)")
	left, right := strings.Repeat("a", 4096), strings.Repeat("b", 4096)
	for _, query := range []string{
		"SELECT levenshtein(?,?)",
		"SELECT id FROM docs WHERE levenshtein(?,?)>0",
		"SELECT levenshtein(?,?) AS distance FROM docs WHERE distance>0",
		"SELECT id FROM docs ORDER BY levenshtein(?,?)",
		"SELECT id FROM docs LIMIT levenshtein(?,?)",
		"SELECT id FROM docs LIMIT 1 OFFSET levenshtein(?,?)",
		"SELECT id FROM docs WHERE body LIKE '%' || levenshtein(?,?) || '%'",
		"SELECT id FROM docs WHERE grams13_match('docs_grams',body,levenshtein(?,?))",
	} {
		t.Run(query, func(t *testing.T) {
			rows, err := db.Query(similaritySQLContext(t), query, left, right)
			if err == nil {
				defer rows.Close()
				for rows.Next() {
				}
				err = rows.Err()
			}
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("operation context not propagated: %v", err)
			}
		})
	}
	for _, query := range []string{
		"INSERT INTO docs VALUES(2,'new',levenshtein(?,?))",
		"UPDATE docs SET score=levenshtein(?,?)",
		"UPDATE docs SET score=1 WHERE levenshtein(?,?)>0",
		"DELETE FROM docs WHERE levenshtein(?,?)>0",
	} {
		t.Run(query, func(t *testing.T) {
			if _, err := db.Exec(similaritySQLContext(t), query, left, right); !errors.Is(err, context.Canceled) {
				t.Fatalf("operation context not propagated: %v", err)
			}
			got, _, _ := gramResults(t, db, "SELECT id,body,score FROM docs ORDER BY id")
			if !reflect.DeepEqual(got, [][]any{{int64(1), "abc", int64(0)}}) {
				t.Fatal("canceled scalar changed rows", got)
			}
		})
	}
}
