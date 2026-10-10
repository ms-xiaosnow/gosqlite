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
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/grams13"
	"github.com/ms-xiaosnow/gosqlite/extension/similarity"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

type similaritySearcher interface {
	SimilaritySearch(context.Context, string, string, any, sqlite.SimilarityOptions) (sqlite.SimilarityResult, error)
}

func searchSimilarity(t *testing.T, db similaritySearcher, query any, options sqlite.SimilarityOptions) sqlite.SimilarityResult {
	t.Helper()
	r, err := db.SimilaritySearch(context.Background(), "docs_grams", "body", query, options)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func sameSimilarity(t *testing.T, db similaritySearcher, query any, options sqlite.SimilarityOptions) sqlite.SimilarityResult {
	t.Helper()
	indexed := searchSimilarity(t, db, query, options)
	options.ForceScan = true
	scanned := searchSimilarity(t, db, query, options)
	if !reflect.DeepEqual(indexed.Hits, scanned.Hits) {
		t.Fatalf("query=%q options=%+v\nindexed=%+v\nscan=%+v", query, options, indexed, scanned)
	}
	return indexed
}

func TestSimilarityNativeCandidateSeeksAndBusinessFilter(t *testing.T) {
	ctx := context.Background()
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY, body, active INTEGER)")
	execute(t, db, "INSERT INTO docs VALUES(-9,'abcd',0),(1,'abce',1),(2,'abcf',1),(3,NULL,1),(4,'disjoint',1),(5,'',1),(6,'a',1)")
	if err := db.CreateGramsIndex(ctx, "docs_grams", "docs", []string{"body"}); err != nil {
		t.Fatal(err)
	}
	o := sqlite.SimilarityOptions{Threshold: 0.6, Limit: 2, Where: "docs.active=:live", Args: []any{sql.Named("live", 1)}}
	r := sameSimilarity(t, db, "abcd", o)
	if len(r.Hits) != 2 || r.Hits[0].RowID != 1 || r.Hits[1].RowID != 2 || r.Hits[0].Score.Value != 2.0/3.0 {
		t.Fatalf("ranking/filter: %+v", r)
	}
	if r.Plan.Access != "SEARCH" || r.Plan.CandidateMode != similarity.UnionPostings || !r.Plan.Exact || r.Plan.QueryTokens != 3 {
		t.Fatalf("plan: %+v", r.Plan)
	}
	if r.Stats.BaseRowFetches != 3 || r.Stats.Candidates != 3 || r.Stats.PostingKeysRead != 7 || r.Stats.PostingRangesRead != 3 || r.Stats.FilteredRows != 1 || r.Stats.ScoredRows != 2 || r.Stats.MatchedRows != 2 || r.Stats.ReturnedRows != 2 {
		t.Fatalf("actual work: %+v", r.Stats)
	}
	if r.Plan.ConfigHash == "" || r.Plan.TokenizerHash == "" || r.Plan.Generation != 1 || !strings.Contains(r.Plan.Detail(), "exact=true") {
		t.Fatalf("identity/detail: %+v", r.Plan)
	}
	// Rank ties are signed rowid ascending, independently of insertion order.
	o.Where, o.Args, o.Threshold = "", nil, 0
	r = sameSimilarity(t, db, "not-present", o)
	if r.Plan.Access != "SCAN" || r.Plan.CandidateMode != similarity.FullScan || r.Stats.BaseRowFetches != 7 {
		t.Fatalf("zero threshold must scan: %+v", r)
	}
	// No overlap at all keeps the rowid tie rather than dropping zero-score rows.
	r = sameSimilarity(t, db, "漢字", o)
	if len(r.Hits) != 2 || r.Hits[0].RowID != -9 || r.Hits[1].RowID != 1 || r.Hits[0].Score.Value != 0 {
		t.Fatalf("zero-score tie: %+v", r)
	}
}

func TestSimilarityNativeAllQueryGramsAndNoCandidateTruncation(t *testing.T) {
	ctx := context.Background()
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY,body TEXT)")
	query := make([]rune, 170)
	for i := range query {
		query[i] = rune(0x4e00 + i)
	}
	execute(t, db, "INSERT INTO docs VALUES(1,?),(2,?),(3,'zzz')", string(query[len(query)-1]), string(query[0]))
	if err := db.CreateGramsIndex(ctx, "docs_grams", "docs", []string{"body"}); err != nil {
		t.Fatal(err)
	}
	o := sqlite.SimilarityOptions{MinGram: 1, MaxGram: 1, Threshold: 0.001, Limit: 10}
	r := sameSimilarity(t, db, string(query), o)
	if len(r.Hits) != 2 || r.Plan.Access != "SEARCH" || r.Plan.QueryTokens != 170 || r.Stats.PostingRangesRead != 170 || r.Stats.PostingKeysRead != 2 || r.Stats.BaseRowFetches != 2 {
		t.Fatalf("must union every query gram, including the final gram: %+v", r)
	}
	// A posting-work quota can prevent completing a union, but never turns a
	// prefix into an allegedly exact result. This case can safely scan 3 rows.
	o.Limits = similarity.DefaultLimits()
	o.Limits.MaxCandidates = 3
	r = sameSimilarity(t, db, string(query), o)
	if r.Plan.Access != "SCAN" || !strings.Contains(r.Plan.Reason, "complete union") || len(r.Hits) != 2 {
		t.Fatalf("quota fallback: %+v", r)
	}
	// Many common postings exceed the posting budget while a bounded full
	// scan remains possible, proving partial union data is not retained.
	execute(t, db, "UPDATE docs SET body='abc' WHERE id<3")
	o.MinGram, o.MaxGram, o.Threshold = 1, 1, 0.1
	r = sameSimilarity(t, db, "abc", o)
	if r.Plan.Access != "SCAN" || r.Stats.PostingKeysRead != 4 || r.Stats.Candidates != 0 || r.Stats.BaseRowFetches != 3 || len(r.Hits) != 2 {
		t.Fatalf("abandoned postings followed by actual full scan: %+v", r)
	}
}

func TestSimilarityNativeEmptyNullUnicodeAndThresholdBoundaries(t *testing.T) {
	ctx := context.Background()
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY,body)")
	execute(t, db, "INSERT INTO docs VALUES(-3,''),(1,'x'),(2,NULL),(3,'abc'),(4,'abd'),(5,?),(6,?),(7,?),(8,12),(9,X'616263')", "é🙂", "e\u0301🙂", "👩‍💻")
	if err := db.CreateGramsIndex(ctx, "docs_grams", "docs", []string{"body"}); err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{"", "x"} {
		r := sameSimilarity(t, db, q, sqlite.SimilarityOptions{Threshold: 1, Limit: 20})
		if r.Plan.Access != "SCAN" || r.Plan.QueryTokens != 0 || len(r.Hits) != 2 || r.Hits[0].RowID != -3 || r.Hits[1].RowID != 1 {
			t.Fatalf("empty-set semantics q=%q: %+v", q, r)
		}
	}
	for _, metric := range []similarity.Metric{similarity.Dice, similarity.Jaccard} {
		boundary := 0.5
		if metric == similarity.Jaccard {
			boundary = 1.0 / 3.0
		}
		for _, threshold := range []float64{math.Nextafter(boundary, 0), boundary, math.Nextafter(boundary, 1)} {
			r := sameSimilarity(t, db, "abc", sqlite.SimilarityOptions{Metric: metric, Threshold: threshold, Limit: 20})
			found := false
			for _, h := range r.Hits {
				found = found || h.RowID == 4
			}
			if found != (threshold <= boundary) {
				t.Fatalf("rounded inclusive threshold metric=%s threshold=%.17g: %+v", metric, threshold, r)
			}
		}
	}
	for _, q := range []any{"é🙂", "e\u0301🙂", "👩‍💻", int64(12), []byte("abc")} {
		r := sameSimilarity(t, db, q, sqlite.SimilarityOptions{Threshold: 1, Limit: 20})
		if len(r.Hits) == 0 || r.Plan.Access != "SEARCH" || r.Hits[0].Score.Value != 1 {
			t.Fatalf("Unicode/coercion query=%q: %+v", q, r)
		}
	}
	for _, q := range []any{nil, []byte(nil)} {
		r := searchSimilarity(t, db, q, sqlite.SimilarityOptions{Limit: 20})
		if len(r.Hits) != 0 || r.Plan.Access != "EMPTY" || r.Stats.BaseRowFetches != 0 || r.Stats.PostingKeysRead != 0 {
			t.Fatalf("NULL query: %+v", r)
		}
	}
	r := searchSimilarity(t, db, "abc", sqlite.SimilarityOptions{Threshold: 1, Limit: 0})
	if r.Plan.Access != "EMPTY" || r.Plan.QueryTokens != 2 || r.Stats.BaseRowFetches != 0 || r.Stats.PostingKeysRead != 0 {
		t.Fatalf("zero limit: %+v", r)
	}
}

func TestSimilarityNativeRandomizedAgainstForcedScan(t *testing.T) {
	ctx := context.Background()
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY,body TEXT,active INTEGER)")
	rng := rand.New(rand.NewSource(15001))
	alphabet := []rune("ab数数据庫あ한🙂é\u0301 ")
	var texts []string
	for i := 0; i < 48; i++ {
		text := make([]rune, rng.Intn(12))
		for j := range text {
			text[j] = alphabet[rng.Intn(len(alphabet))]
		}
		texts = append(texts, string(text))
		execute(t, db, "INSERT INTO docs VALUES(?,?,?)", i-24, string(text), i%3)
	}
	execute(t, db, "INSERT INTO docs VALUES(100,NULL,1)")
	if err := db.CreateGramsIndex(ctx, "docs_grams", "docs", []string{"body"}); err != nil {
		t.Fatal(err)
	}
	searches := 0
	for q := 0; q < 12; q++ {
		for _, gramRange := range [][2]int{{1, 1}, {2, 2}, {3, 3}, {1, 3}} {
			for _, metric := range []similarity.Metric{similarity.Dice, similarity.Jaccard} {
				for _, threshold := range []float64{0, .2, 1} {
					o := sqlite.SimilarityOptions{MinGram: gramRange[0], MaxGram: gramRange[1], Metric: metric, Threshold: threshold, Limit: 7}
					if q%2 == 0 {
						o.Where, o.Args = "active>=?", []any{1}
					}
					sameSimilarity(t, db, texts[q*3], o)
					searches++
				}
			}
		}
	}
	if searches != 288 {
		t.Fatalf("unexpected differential count: %d", searches)
	}
}

func TestSimilarityNativeTransactionsRebuildReopenCheckpoint(t *testing.T) {
	ctx := context.Background()
	for _, wal := range []bool{false, true} {
		t.Run(fmt.Sprintf("wal=%t", wal), func(t *testing.T) {
			opts := sqlite.Options{WALShared: wal}
			db, path := writable(t, opts)
			execute(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY,body TEXT)")
			execute(t, db, "INSERT INTO docs VALUES(1,'Straße'),(2,'STRASSE'),(3,'other')")
			if err := db.CreateGramsIndex(ctx, "docs_grams", "docs", []string{"body"}); err != nil {
				t.Fatal(err)
			}
			o := sqlite.SimilarityOptions{Threshold: 1, Limit: 10}
			before := sameSimilarity(t, db, "STRASSE", o)
			if len(before.Hits) != 1 || before.Hits[0].RowID != 2 {
				t.Fatal(before)
			}
			tx, err := db.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			txExecute(t, tx, "UPDATE docs SET body='STRASSE' WHERE id=3")
			txExecute(t, tx, "INSERT INTO docs VALUES(-10,'STRASSE')")
			r := sameSimilarity(t, tx, "STRASSE", o)
			if len(r.Hits) != 3 || r.Hits[0].RowID != -10 {
				t.Fatalf("read own writes: %+v", r)
			}
			// The DB-level method shares this same active transaction snapshot.
			if viaDB := searchSimilarity(t, db, "STRASSE", o); !reflect.DeepEqual(viaDB.Hits, r.Hits) {
				t.Fatalf("DB/Tx mismatch: %+v %+v", viaDB, r)
			}
			folded := grams13.DefaultMatchConfig()
			folded.CaseFold = grams13.CaseFoldUnicode
			if err := tx.Reindex(ctx, "docs_grams", sqlite.GramsIndexOptions{MatchConfig: folded}); err != nil {
				t.Fatal(err)
			}
			r = sameSimilarity(t, tx, "STRASSE", o)
			if len(r.Hits) != 4 || r.Plan.TokenizerHash == before.Plan.TokenizerHash || r.Plan.Generation <= before.Plan.Generation {
				t.Fatalf("rebuild configuration not pinned: %+v", r)
			}
			if err := tx.Rollback(); err != nil {
				t.Fatal(err)
			}
			r = sameSimilarity(t, db, "STRASSE", o)
			if !reflect.DeepEqual(r.Hits, before.Hits) || r.Plan.TokenizerHash != before.Plan.TokenizerHash || r.Plan.Generation != before.Plan.Generation {
				t.Fatalf("rollback changed identity/results: %+v %+v", before, r)
			}
			if _, err := tx.SimilaritySearch(ctx, "docs_grams", "body", "x", o); !errors.Is(err, sqlite.ErrTxDone) {
				t.Fatalf("completed tx accepted: %v", err)
			}
			if err := db.Reindex(ctx, "docs_grams", sqlite.GramsIndexOptions{MatchConfig: folded}); err != nil {
				t.Fatal(err)
			}
			r = sameSimilarity(t, db, "STRASSE", o)
			if len(r.Hits) != 2 || r.Hits[0].RowID != 1 || r.Hits[1].RowID != 2 {
				t.Fatal(r)
			}
			if err := db.CheckExtensionIndex(ctx, "docs_grams"); err != nil {
				t.Fatal(err)
			}
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
			after := sameSimilarity(t, reopened, "STRASSE", o)
			if !reflect.DeepEqual(after.Hits, r.Hits) || after.Plan.TokenizerHash != r.Plan.TokenizerHash || after.Plan.Generation != r.Plan.Generation || after.Plan.Access != "SEARCH" {
				t.Fatalf("ordinary reopen changed search: %+v %+v", r, after)
			}
		})
	}
}

func TestSimilarityNativeRejectsInvalidAndBoundedWork(t *testing.T) {
	ctx := context.Background()
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY,body TEXT,active INTEGER)")
	execute(t, db, "INSERT INTO docs VALUES(1,'abcd',0),(2,'abce',0),(3,'abcf',0)")
	if err := db.CreateGramsIndex(ctx, "docs_grams", "docs", []string{"body"}); err != nil {
		t.Fatal(err)
	}
	for _, options := range []sqlite.SimilarityOptions{
		{Limit: -1}, {Limit: 10001}, {Limit: 1, Threshold: math.NaN()}, {Limit: 1, Threshold: math.Inf(1)},
		{Limit: 1, Threshold: -1}, {Limit: 1, Threshold: 1.1}, {Limit: 1, Metric: "cosine"},
		{Limit: 1, MinGram: 1}, {Limit: 1, MinGram: 4, MaxGram: 4}, {Limit: 1, Where: "missing=1"},
		{Limit: 1, Where: "active=?"}, {Limit: 1, Where: "active) ORDER BY id --"},
		{Limit: 1, Args: []any{1}}, {Limit: 1, Where: "active=1; DELETE FROM docs"},
	} {
		if _, err := db.SimilaritySearch(ctx, "docs_grams", "body", "abcd", options); err == nil {
			t.Fatalf("accepted invalid options: %+v", options)
		}
	}
	for _, q := range []any{string([]byte{0xff}), []byte{0xff}, struct{}{}} {
		if _, err := db.SimilaritySearch(ctx, "docs_grams", "body", q, sqlite.SimilarityOptions{Limit: 1}); err == nil {
			t.Fatalf("accepted invalid query %T", q)
		}
	}
	for _, pair := range [][2]string{{"missing", "body"}, {"docs_grams", "active"}} {
		if _, err := db.SimilaritySearch(ctx, pair[0], pair[1], "abcd", sqlite.SimilarityOptions{Limit: 1}); err == nil {
			t.Fatalf("accepted bad target: %v", pair)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := db.SimilaritySearch(canceled, "docs_grams", "body", "abcd", sqlite.SimilarityOptions{Limit: 1}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled: %v", err)
	}
	if _, err := db.SimilaritySearch(nil, "docs_grams", "body", "abcd", sqlite.SimilarityOptions{Limit: 1}); err == nil {
		t.Fatal("nil context accepted")
	}
	// Business filtering cannot evade the source-work bound.
	limits := similarity.DefaultLimits()
	limits.MaxCandidates = 2
	r, err := db.SimilaritySearch(ctx, "docs_grams", "body", "abcd", sqlite.SimilarityOptions{Limit: 1, Where: "active=1", Limits: limits})
	if !errors.Is(err, similarity.ErrLimit) || !errors.Is(err, sqlite.ErrMemoryLimit) || !reflect.DeepEqual(r, sqlite.SimilarityResult{}) {
		t.Fatalf("must fail without partial hits when filtered scan exceeds quota: %+v %v", r, err)
	}
	limits = similarity.DefaultLimits()
	limits.MaxTotalInputBytes = 8
	if _, err := db.SimilaritySearch(ctx, "docs_grams", "body", "abcd", sqlite.SimilarityOptions{Limit: 1, Where: "active=1", Limits: limits}); !errors.Is(err, similarity.ErrLimit) {
		t.Fatalf("filtered text bypassed input-byte quota: %v", err)
	}
	limits = similarity.DefaultLimits()
	limits.MaxSetComparisons = 1
	if _, err := db.SimilaritySearch(ctx, "docs_grams", "body", "abcd", sqlite.SimilarityOptions{Threshold: .1, Limit: 1, Limits: limits}); !errors.Is(err, similarity.ErrLimit) {
		t.Fatalf("comparison quota not cumulative: %v", err)
	}
}

func TestSimilarityNativeConfiguredNormalizationAndMatchNamespace(t *testing.T) {
	ctx := context.Background()
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY,body TEXT,active INTEGER)")
	execute(t, db, "INSERT INTO docs VALUES(1,?,1),(2,?,1),(3,?,1),(4,'aaaa',1),(5,'aa',1)", "  A-b\tStraße  ", "ab\x00cd", "ab\x00ce")
	config := grams13.DefaultMatchConfig()
	config.CaseFold = grams13.CaseFoldUnicode
	config.Whitespace = grams13.WhitespaceCollapse
	config.Punctuation = grams13.PunctuationRemove
	if err := db.CreateGramsIndex(ctx, "docs_grams", "docs", []string{"body"}, sqlite.GramsIndexOptions{MatchConfig: config}); err != nil {
		t.Fatal(err)
	}
	for _, gramRange := range [][2]int{{1, 1}, {2, 2}, {3, 3}, {1, 3}} {
		o := sqlite.SimilarityOptions{MinGram: gramRange[0], MaxGram: gramRange[1], Threshold: 1, Limit: 10}
		r := sameSimilarity(t, db, " ab strasse ", o)
		if len(r.Hits) != 1 || r.Hits[0].RowID != 1 || r.Hits[0].Score.Value != 1 || r.Plan.Access != "SEARCH" {
			t.Fatalf("configured transforms: %+v", r)
		}
	}
	// MATCH preserves NUL; raw SQL LIKE postings terminate there. Only the
	// configured namespace can retrieve this common suffix after NUL.
	r := sameSimilarity(t, db, "\x00cd", sqlite.SimilarityOptions{Threshold: .5, Limit: 10, Where: "grams13_match('docs_grams',body,?)", Args: []any{"cd"}})
	if len(r.Hits) != 1 || r.Hits[0].RowID != 2 || r.Plan.Access != "SEARCH" || r.Plan.Namespace != string(grams13.MatchConfigV1) {
		t.Fatalf("NUL-preserving candidate namespace: %+v", r)
	}
	// Distinct sets do not weight repeated grams by occurrence count.
	r = sameSimilarity(t, db, "aaaaaa", sqlite.SimilarityOptions{Threshold: 1, Limit: 10})
	if len(r.Hits) != 2 || r.Hits[0].RowID != 4 || r.Hits[1].RowID != 5 || r.Hits[0].Score.LeftTokens != 1 || r.Hits[0].Score.RightTokens != 1 {
		t.Fatalf("duplicate token set semantics: %+v", r)
	}
}

type cancelingSimilarityValuer struct{ cancel context.CancelFunc }

func (v cancelingSimilarityValuer) Value() (driver.Value, error) {
	v.cancel()
	return nil, nil
}

func TestSimilarityNativeCancellationDuringNullCoercion(t *testing.T) {
	db, _ := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY,body TEXT)")
	if err := db.CreateGramsIndex(context.Background(), "docs_grams", "docs", []string{"body"}); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result, err := db.SimilaritySearch(ctx, "docs_grams", "body", cancelingSimilarityValuer{cancel}, sqlite.SimilarityOptions{Limit: 1})
	if !errors.Is(err, context.Canceled) || !reflect.DeepEqual(result, sqlite.SimilarityResult{}) {
		t.Fatalf("NULL coercion suppressed cancellation: %+v %v", result, err)
	}
	// Ordinary cancellation closes the read snapshot, allowing the next write.
	execute(t, db, "INSERT INTO docs VALUES(1,'ok')")
}
