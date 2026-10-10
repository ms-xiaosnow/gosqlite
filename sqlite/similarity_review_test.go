package sqlite_test

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/grams13"
	"github.com/ms-xiaosnow/gosqlite/extension/similarity"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestSimilarityReviewRejectsFunctionDefaults(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	query := "CREATE TABLE review_defaults(id INTEGER PRIMARY KEY, value INTEGER DEFAULT (levenshtein('" + strings.Repeat("a", 4096) + "','" + strings.Repeat("b", 4096) + "')))"
	_, err := db.Exec(context.Background(), query)
	if !errors.Is(err, sqlite.ErrUnsupported) {
		t.Fatalf("function default must be rejected before context-free default validation: %v", err)
	}
}

// This reference uses independent rune windows, maps, cardinality arithmetic,
// and a full sort. Inputs are already NFC and need no other default transform;
// it shares neither production tokenization nor production ranking helpers.
func reviewSimilarityScore(a, b string, lo, hi int, metric similarity.Metric) similarity.Score {
	set := func(text string) map[string]bool {
		runes := []rune(text)
		grams := map[string]bool{}
		for n := lo; n <= hi; n++ {
			for i := 0; i+n <= len(runes); i++ {
				grams[string(runes[i:i+n])] = true
			}
		}
		return grams
	}
	x, y := set(a), set(b)
	common := 0
	for gram := range x {
		if y[gram] {
			common++
		}
	}
	score := similarity.Score{LeftTokens: len(x), RightTokens: len(y), Intersection: common}
	if len(x)+len(y) == 0 {
		score.Value = 1
	} else if metric == similarity.Dice {
		score.Value = float64(2*common) / float64(len(x)+len(y))
	} else {
		score.Value = float64(common) / float64(len(x)+len(y)-common)
	}
	return score
}

func TestSimilarityReviewIndependentRecall(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY,body TEXT,active INTEGER)")
	type document struct {
		id     int64
		body   string
		active int
	}
	documents := []document{{math.MinInt64, "ab数🙂", 1}, {math.MaxInt64, "ab数🙂", 1}}
	texts := []string{"", "a", "b", "数", "🙂", "\x00"}
	for _, a := range []string{"a", "b", "数", "🙂", "\x00"} {
		for _, b := range []string{"a", "b", "数", "🙂", "\x00"} {
			texts = append(texts, a+b, a+b+a)
		}
	}
	for i, text := range texts {
		documents = append(documents, document{int64(i) - 30, text, i % 2})
	}
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range documents {
		txExecute(t, tx, "INSERT INTO docs VALUES(?,?,?)", row.id, row.body, row.active)
	}
	txExecute(t, tx, "INSERT INTO docs VALUES(1000,NULL,1)")
	if err := tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := db.CreateGramsIndex(context.Background(), "docs_grams", "docs", []string{"body"}); err != nil {
		t.Fatal(err)
	}
	queries := []string{"", "a", "aba", "ab数🙂", "🙂\x00🙂", "数a数", "xyz"}
	for _, q := range queries {
		for lo := 1; lo <= 3; lo++ {
			for hi := lo; hi <= 3; hi++ {
				for _, metric := range []similarity.Metric{similarity.Dice, similarity.Jaccard} {
					for _, threshold := range []float64{0, math.SmallestNonzeroFloat64, 1.0 / 3.0, .5, math.Nextafter(.5, 1), 1} {
						want := []similarity.Hit{}
						for _, row := range documents {
							if row.active != 1 {
								continue
							}
							score := reviewSimilarityScore(q, row.body, lo, hi, metric)
							if score.Value >= threshold {
								want = append(want, similarity.Hit{RowID: row.id, Score: score})
							}
						}
						sort.Slice(want, func(i, j int) bool {
							return want[i].Score.Value > want[j].Score.Value || want[i].Score.Value == want[j].Score.Value && want[i].RowID < want[j].RowID
						})
						if len(want) > 7 {
							want = want[:7]
						}
						got, err := db.SimilaritySearch(context.Background(), "docs_grams", "body", q, sqlite.SimilarityOptions{MinGram: lo, MaxGram: hi, Metric: metric, Threshold: threshold, Limit: 7, Where: "active=1"})
						if err != nil || !reflect.DeepEqual(got.Hits, want) {
							t.Fatalf("query %q range %d..%d metric %s threshold %.17g: got %+v error %v want %+v", q, lo, hi, metric, threshold, got, err, want)
						}
						queryTokens := reviewSimilarityScore(q, q, lo, hi, metric).LeftTokens
						access := "SEARCH"
						if threshold == 0 || queryTokens == 0 {
							access = "SCAN"
						}
						if got.Plan.Access != access || got.Plan.QueryTokens != queryTokens {
							t.Fatalf("untruthful plan: %+v, wanted %s and %d query tokens", got.Plan, access, queryTokens)
						}
						if access == "SEARCH" && got.Stats.PostingRangesRead != int64(queryTokens) {
							t.Fatalf("not every selected query gram was read: %+v", got)
						}
					}
				}
			}
		}
	}
}

func TestSimilarityReviewWhereMatchesSQL(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY,body TEXT,active INTEGER,flag TEXT)")
	execute(t, db, "INSERT INTO docs VALUES(-3,'abc',1,'1'),(1,'abc',0,'0'),(2,'abc',NULL,NULL),(3,'abc',2,'2')")
	if err := db.CreateGramsIndex(context.Background(), "docs_grams", "docs", []string{"body"}); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		where string
		args  []any
	}{
		{"active", nil},
		{"active=TRUE", nil},
		{"active IS FALSE", nil},
		{"active IS NULL", nil},
		{"flag=1", nil},
		{"docs.active=?2 AND id>=?1", []any{-5, 1}},
		{"docs.active=:value OR docs.id=@value", []any{sql.Named("value", 1)}},
		{"_rowid_>=? AND oid< ?", []any{-3, 2}},
		{"active=1 /* trailing comment */", nil},
		{"active=1 -- trailing comment", nil},
		{"active=? -- trailing parameter comment", []any{1}},
	} {
		t.Run(tc.where, func(t *testing.T) {
			wantRows, _, _ := gramResults(t, db, "SELECT id FROM docs WHERE "+tc.where+"\n ORDER BY id", tc.args...)
			want := make([]int64, len(wantRows))
			for i, row := range wantRows {
				want[i] = row[0].(int64)
			}
			for _, scan := range []bool{false, true} {
				got, err := db.SimilaritySearch(context.Background(), "docs_grams", "body", "abc", sqlite.SimilarityOptions{Threshold: 1, Limit: 10, Where: tc.where, Args: tc.args, ForceScan: scan})
				if err != nil {
					t.Fatal(err)
				}
				ids := make([]int64, len(got.Hits))
				for i, hit := range got.Hits {
					ids[i] = hit.RowID
				}
				if !reflect.DeepEqual(ids, want) {
					t.Fatalf("scan=%t got %v want %v", scan, ids, want)
				}
			}
		})
	}
}

func TestSimilarityReviewRuntimeFailuresDiscardHits(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY,body TEXT,n INTEGER)")
	execute(t, db, "INSERT INTO docs VALUES(1,'abc',2),(2,'abc',4)")
	if err := db.CreateGramsIndex(context.Background(), "docs_grams", "docs", []string{"body"}); err != nil {
		t.Fatal(err)
	}
	for _, forceScan := range []bool{false, true} {
		// The first row qualifies and reaches the Top-K heap; the next row's
		// runtime error must discard all previously retained hits and metadata.
		got, err := db.SimilaritySearch(context.Background(), "docs_grams", "body", "abc", sqlite.SimilarityOptions{Threshold: .1, Limit: 1, ForceScan: forceScan, Where: "ngram_similarity(body,'abc',n,'dice')>=0"})
		if !errors.Is(err, similarity.ErrValue) || !reflect.DeepEqual(got, sqlite.SimilarityResult{}) {
			t.Fatalf("runtime filter failure leaked partial results: %+v %v", got, err)
		}
		limits := similarity.DefaultLimits()
		limits.MaxSetComparisons = 3 // One two-comparison row fits; the next fails.
		got, err = db.SimilaritySearch(context.Background(), "docs_grams", "body", "abc", sqlite.SimilarityOptions{Threshold: .1, Limit: 1, ForceScan: forceScan, Limits: limits})
		if !errors.Is(err, similarity.ErrLimit) || !errors.Is(err, sqlite.ErrMemoryLimit) || !reflect.DeepEqual(got, sqlite.SimilarityResult{}) {
			t.Fatalf("late aggregate budget failure leaked partial results: %+v %v", got, err)
		}
		got, err = db.SimilaritySearch(similaritySQLContext(t), "docs_grams", "body", "abc", sqlite.SimilarityOptions{Threshold: .1, Limit: 1, ForceScan: forceScan, Where: "levenshtein(?,?)>0", Args: []any{strings.Repeat("a", 4096), strings.Repeat("b", 4096)}})
		if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(got, sqlite.SimilarityResult{}) {
			t.Fatalf("native filter cancellation failed: %+v %v", got, err)
		}
	}
	// The connection stays usable after each read-only failure.
	got := searchSimilarity(t, db, "abc", sqlite.SimilarityOptions{Threshold: 1, Limit: 2})
	if len(got.Hits) != 2 || got.Hits[0].RowID != 1 || got.Hits[1].RowID != 2 {
		t.Fatal(got)
	}
}

func TestSimilarityReviewScalarDefaultVersusPersistedConfiguration(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY,body TEXT)")
	execute(t, db, "INSERT INTO docs VALUES(1,'ab'),(2,'A-B')")
	config := grams13.DefaultMatchConfig()
	config.Punctuation = grams13.PunctuationRemove
	config.CaseFold = grams13.CaseFoldUnicode
	if err := db.CreateGramsIndex(context.Background(), "docs_grams", "docs", []string{"body"}, sqlite.GramsIndexOptions{MatchConfig: config}); err != nil {
		t.Fatal(err)
	}
	got := sameSimilarity(t, db, "ab", sqlite.SimilarityOptions{Threshold: 1, Limit: 10})
	if len(got.Hits) != 2 || got.Hits[1].Score.Value != 1 {
		t.Fatal(got)
	}
	scalar, _, _ := gramResults(t, db, "SELECT id,bigram_similarity(body,'ab') FROM docs ORDER BY id")
	if !reflect.DeepEqual(scalar, [][]any{{int64(1), float64(1)}, {int64(2), float64(0)}}) {
		t.Fatalf("scalar default unexpectedly followed persisted index configuration: %+v", scalar)
	}
}
