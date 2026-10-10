package sqlite_test

import (
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

type gramQuerier interface {
	Query(context.Context, string, ...any) (*sqlite.Rows, error)
}

func gramResults(t *testing.T, db gramQuerier, q string, args ...any) ([][]any, sqlite.QueryPlan, sqlite.QueryStats) {
	t.Helper()
	r, err := db.Query(context.Background(), q, args...)
	if err != nil {
		t.Fatalf("%s (%#v): %v", q, args, err)
	}
	defer r.Close()
	var out [][]any
	for r.Next() {
		out = append(out, r.Values())
	}
	if err = r.Err(); err != nil {
		t.Fatalf("%s (%#v): %v", q, args, err)
	}
	return out, r.Plan(), r.Stats()
}
func gramCompare(t *testing.T, db gramQuerier, where string, args ...any) (sqlite.QueryPlan, sqlite.QueryStats, sqlite.QueryStats) {
	t.Helper()
	got, p, s := gramResults(t, db, "SELECT id FROM docs WHERE "+where+" ORDER BY id", args...)
	want, sp, ss := gramResults(t, db, "SELECT id FROM docs NOT INDEXED WHERE "+where+" ORDER BY id", args...)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("indexed vs scan mismatch WHERE %s args=%#v\nindexed=%#v\nscan=%#v\nplan=%+v", where, args, got, want, p)
	}
	if sp.Access != "SCAN" || sp.Reason != "NOT INDEXED requested" {
		t.Fatalf("forced scan: %+v", sp)
	}
	return p, s, ss
}
func escapeGramLiteral(s string, escape rune) string {
	var out strings.Builder
	for _, r := range s {
		if r == escape || r == '%' || r == '_' {
			out.WriteRune(escape)
		}
		out.WriteRune(r)
	}
	return out.String()
}
func gramFixture(t *testing.T) (*sqlite.DB, []string) {
	t.Helper()
	db, _ := writable(t, sqlite.Options{MaxTransactionPages: 8192})
	execute(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY,title,body TEXT)")
	texts := []string{"", "e\u0301", "é", "É", "한글", "한글", "かなカナ", "か\u3099", "が", "👩‍💻🙂", "数据库", "数据", "库", "a%b_c\\d", "A_B%", "NUL\x00数据库", "\x00hidden", "white\t \nspace", "punct.,!?", "ßSSİıΣσς", "CGJ\u034f", "rare甲乙丙", "rare甲乙", "rare甲"}
	rng := rand.New(rand.NewSource(540013))
	alphabet := []rune("abcdefghijklmnopqrstuvwxyzABCD数据库检索中文日本語한글かなカナ🙂👩‍💻é_%\\ ")
	for len(texts) < 128 {
		rr := make([]rune, 5+rng.Intn(22))
		for j := range rr {
			rr[j] = alphabet[rng.Intn(len(alphabet))]
		}
		texts = append(texts, string(rr))
	}
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for i, s := range texts {
		txExecute(t, tx, "INSERT INTO docs VALUES(?,?,?)", int64(i-64), s, fmt.Sprintf("body%d:%s", i, s))
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	execute(t, db, "CREATE INDEX docs_text ON docs(title,body) USING grams13")
	return db, texts
}

// 100 deterministic randomized complete result-set comparisons over 128 texts
// establish 12,800 query/text membership comparisons, not 12,800 executions.
func TestGramsIndexedLike12800Combinations(t *testing.T) {
	db, texts := gramFixture(t)
	rng := rand.New(rand.NewSource(354000013))
	search, scan, nonemptySearch := 0, 0, 0
	for i := 0; i < 100; i++ {
		rr := []rune(texts[rng.Intn(len(texts))])
		if len(rr) == 0 {
			rr = []rune("unseen字")
		}
		start := rng.Intn(len(rr))
		n := min(1+rng.Intn(5), len(rr)-start)
		lit := escapeGramLiteral(string(rr[start:start+n]), '\\')
		var pattern string
		switch i % 4 {
		case 0:
			pattern = "%" + lit + "%"
		case 1:
			pattern = "_" + lit + "%"
		case 2:
			pattern = "%" + lit + "_"
		case 3:
			pattern = lit
		}
		p, s, ss := gramCompare(t, db, "title LIKE ? ESCAPE '\\'", pattern)
		if p.Access == "SEARCH" {
			search++
			if s.ReturnedRows > 0 {
				nonemptySearch++
			}
			if s.BaseRowFetches > ss.BaseRowFetches {
				t.Fatal("index fetched more source rows", s, ss)
			}
		} else {
			scan++
		}
	}
	if nonemptySearch < 10 {
		t.Fatalf("only %d nonempty SEARCH queries", nonemptySearch)
	}
	if search < 75 {
		t.Fatalf("only %d SEARCH plans (SCAN=%d)", search, scan)
	}
	t.Logf("100 queries x128 texts =12,800 exact membership comparisons; SEARCH=%d SCAN=%d nonempty_SEARCH=%d", search, scan, nonemptySearch)
}

func TestGramsOneTwoThreeRuneSearchAndExplain(t *testing.T) {
	db, _ := gramFixture(t)
	for n, needle := range []string{"甲", "甲乙", "甲乙丙"} {
		p, s, ss := gramCompare(t, db, "title LIKE ?", "%"+needle+"%")
		if p.Access != "SEARCH" || !p.Recheck || p.Method != "grams13" || !reflect.DeepEqual(p.GramLengths, []int{n + 1}) {
			t.Fatalf("%s plan=%+v", needle, p)
		}
		if s.PostingKeysRead <= 0 || s.Candidates <= 0 || s.BaseRowFetches != s.Candidates || s.BaseRowFetches >= ss.BaseRowFetches || s.ReturnedRows <= 0 {
			t.Fatalf("%s indexed=%+v scan=%+v", needle, s, ss)
		}
		got, ep, es := gramResults(t, db, "EXPLAIN QUERY PLAN SELECT id FROM docs WHERE title LIKE ?", "%"+needle+"%")
		if ep.Access != "SEARCH" || len(got) != 1 || len(got[0]) != 4 || !strings.Contains(got[0][3].(string), "covering_posting_ranges=true") {
			t.Fatalf("explain=%#v plan=%+v", got, ep)
		}
		if es.PostingKeysRead != 0 || es.BaseRowFetches != 0 {
			t.Fatalf("EXPLAIN executed query %+v", es)
		}
	}
	// Unsorted LIMIT stays streaming: posting candidates are known at planning,
	// but only emitted source rows have been fetched. Metadata returns copies.
	r, err := db.Query(context.Background(), "SELECT id FROM docs WHERE title LIKE '%甲%' LIMIT 1")
	if err != nil {
		t.Fatal(err)
	}
	before := r.Stats()
	if before.Candidates != 3 || before.BaseRowFetches != 0 {
		t.Fatal(before)
	}
	plan := r.Plan()
	plan.Indexes[0] = "changed"
	plan.GramLengths[0] = 99
	if r.Plan().Indexes[0] == "changed" || r.Plan().GramLengths[0] == 99 {
		t.Fatal("plan metadata aliased")
	}
	if !r.Next() || r.Next() {
		t.Fatal("streaming LIMIT", r.Err())
	}
	if s := r.Stats(); s.BaseRowFetches != 1 || s.ReturnedRows != 1 {
		t.Fatal(s)
	}
	r.Close()
}

func TestGramsUnicodeEscapeNULLNULAndBooleanSafety(t *testing.T) {
	db, _ := gramFixture(t)
	execute(t, db, "INSERT INTO docs VALUES(900,NULL,NULL),(901,12.5,'numeric'),(902,X'41425F25','blob')")
	for _, tc := range []struct {
		pattern any
		escape  any
	}{
		{"%e%", "\\"}, {"%\u0301%", "\\"}, {"%é%", "\\"}, {"%É%", "\\"},
		{"%하%", "\\"}, {"%한%", "\\"}, {"%か\u3099%", "\\"}, {"%が%", "\\"},
		{"%👩‍💻%", "\\"}, {"%‍%", "\\"}, {"%ß%", "\\"}, {"%ss%", "\\"},
		{"%\\%%", "\\"}, {"%\\_%", "\\"}, {"%\\\\%", "\\"}, {"%界_%", "界"}, {"%界%%", "界"},
		{"%NUL%\x00never", "\\"}, {"%hidden%", "\\"}, {"%\x00ignored", "\\"},
		{"%data%", nil}, {nil, "\\"}, {"%", "\\"}, {"___", "\\"}, {"%ab_%%", "\\"},
		{"%12.5%", "\\"}, {"%A_B%", "\\"}, {"%a%b%", "\\"}, {"%CGJ\u034f%", "\\"},
	} {
		gramCompare(t, db, "title LIKE ? ESCAPE ?", tc.pattern, tc.escape)
	}
	cases := []struct {
		where string
		args  []any
		want  string
	}{
		{"title LIKE ? AND id > ?", []any{"%甲%", int64(-1000)}, "SEARCH"},
		{"id > ? AND title LIKE ?", []any{int64(-1000), "%甲%"}, "SEARCH"},
		{"title LIKE ? OR body LIKE ?", []any{"%甲%", "%body80:%"}, "SEARCH"},
		{"title LIKE ? OR id=0", []any{"%甲%"}, "SCAN"},
		{"title NOT LIKE ?", []any{"%甲%"}, "SCAN"},
		{"NOT (title LIKE ?)", []any{"%甲%"}, "SCAN"},
		{"title LIKE body", nil, "SCAN"},
		{"title LIKE ?", []any{"%"}, "SCAN"},
	}
	for _, tc := range cases {
		p, _, _ := gramCompare(t, db, tc.where, tc.args...)
		if p.Access != tc.want {
			t.Fatalf("%s plan=%+v", tc.where, p)
		}
	}
	got, p, _ := gramResults(t, db, "SELECT id, title AS text FROM docs WHERE text LIKE ? ORDER BY id", "%甲%")
	want, _, _ := gramResults(t, db, "SELECT id, title AS text FROM docs NOT INDEXED WHERE text LIKE ? ORDER BY id", "%甲%")
	if p.Access != "SEARCH" || !reflect.DeepEqual(got, want) {
		t.Fatal(p, got, want)
	}
}

func TestGramsHighFrequencyFallbackAndLimits(t *testing.T) {
	db, _ := gramFixture(t)
	execute(t, db, "UPDATE docs SET title='common' || title")
	p, _, _ := gramCompare(t, db, "title LIKE '%c%'")
	if p.Access != "SCAN" || !strings.Contains(p.Reason, "frequency") {
		t.Fatalf("common gram %+v", p)
	}
	got, p, s := gramResults(t, db, "SELECT id FROM docs WHERE title LIKE '%甲%' ORDER BY id DESC LIMIT 1 OFFSET 1")
	want, _, _ := gramResults(t, db, "SELECT id FROM docs NOT INDEXED WHERE title LIKE '%甲%' ORDER BY id DESC LIMIT 1 OFFSET 1")
	if p.Access != "SEARCH" || !reflect.DeepEqual(got, want) || s.ReturnedRows != 1 {
		t.Fatal(got, want, p, s)
	}
}

func TestGramsConfiguredMatchSnapshotAndCandidates(t *testing.T) {
	db, path := writable(t, sqlite.Options{})
	execute(t, db, "CREATE TABLE docs(id INTEGER PRIMARY KEY,title TEXT,body TEXT)")
	tx, err := db.Begin(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	special := []string{"Straße", "e\u0301", "한글", "👩‍💻", "one,\t \nTWO", "a\x00b", "...", "İstanbul"}
	for i := 0; i < 40; i++ {
		s := fmt.Sprintf("filler%d", i)
		if i < len(special) {
			s = special[i]
		}
		txExecute(t, tx, "INSERT INTO docs VALUES(?,?,?)", i, s, s)
	}
	if err = tx.Commit(context.Background()); err != nil {
		t.Fatal(err)
	}
	execute(t, db, `CREATE INDEX docs_match ON docs(title) USING grams13 WITH (case_fold='unicode-full-default', punctuation='unicode-punct-remove', whitespace='unicode-space-collapse')`)
	for _, needle := range []string{"STRASSE", "é", "한", "👩‍💻", "ONE TWO", "a\x00b", "İSTANBUL"} {
		p, s, ss := gramCompare(t, db, "grams13_match('docs_match',title,?)", needle)
		if p.Access != "SEARCH" || p.Namespace != "MATCH_CONFIG_V1" || s.BaseRowFetches >= ss.BaseRowFetches || s.ReturnedRows < 1 {
			t.Fatalf("%q plan=%+v indexed=%+v scan=%+v", needle, p, s, ss)
		}
	}
	for _, needle := range []string{"", ".,!?"} {
		p, s, ss := gramCompare(t, db, "grams13_match('docs_match',title,?)", needle)
		if p.Access != "SCAN" || !strings.Contains(p.Reason, "empty normalized") || s.ReturnedRows != 40 || s.BaseRowFetches != ss.BaseRowFetches {
			t.Fatal(needle, p, s, ss)
		}
	}
	// The raw LIKE namespace must not adopt MATCH's NFC, casefold or removal.
	p, s, _ := gramCompare(t, db, "title LIKE '%\u0301%'")
	if p.Access != "SEARCH" || p.Namespace != "LIKE_RAW_V1" || s.ReturnedRows == 0 {
		t.Fatal(p, s)
	}
	p, _, _ = gramCompare(t, db, "grams13_match('docs_match',title,'STRASSE') OR title LIKE '%👩%'")
	if p.Access != "SEARCH" || !strings.Contains(p.Namespace, "MATCH_CONFIG_V1") || !strings.Contains(p.Namespace, "LIKE_RAW_V1") {
		t.Fatal(p)
	}
	got, _, _ := gramResults(t, db, "SELECT grams13_match('docs_match','STRAßE','strasse'), grams13_match('docs_match',NULL,'x')")
	if !reflect.DeepEqual(got, [][]any{{int64(1), nil}}) {
		t.Fatal(got)
	}
	r, err := db.Query(context.Background(), "SELECT grams13_match('unknown','x','x')")
	if err == nil {
		r.Next()
		err = r.Err()
		r.Close()
	}
	if err == nil {
		t.Fatal("unknown index accepted")
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	p, _, _ = gramCompare(t, reopened, "grams13_match('docs_match',title,'STRASSE')")
	if p.Access != "SEARCH" {
		t.Fatal(p)
	}
}
