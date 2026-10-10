package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"math"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// These review regressions use only synthetic catalogs, memory-only conversion,
// and ordinary SQL on a newly created intact temporary database.
func reviewConverterDB(t *testing.T) *sqlite.DB {
	t.Helper()
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "review.db"), sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	c := converterCatalog(t)
	for _, src := range []string{
		`CREATE TABLE public.documents(id INTEGER PRIMARY KEY,flags INTEGER NOT NULL,embedding REAL[]);`,
		`INSERT INTO public.documents(id,flags,embedding) VALUES(1,20,ARRAY[1,2,3]),(2,10,NULL);`,
	} {
		r := converted(t, c, src, localConvertOptions())
		if _, err := db.Exec(context.Background(), r.Statements[0].ConvertedSQL); err != nil {
			t.Fatal(err)
		}
	}
	return db
}

func TestConvertReviewQuotedAliasCaseCollision(t *testing.T) {
	c := converterCatalog(t)
	src := `SELECT flags AS "ID" FROM public.documents ORDER BY id;`
	r, err := Convert(context.Background(), src, c, localConvertOptions())
	if err != nil {
		t.Fatal(err)
	}
	if r.Status == "unsupported" {
		return
	}
	db := reviewConverterDB(t)
	rows, err := db.Query(context.Background(), r.Statements[0].ConvertedSQL)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var values []int64
	for rows.Next() {
		values = append(values, rows.Values()[0].(int64))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(values, []int64{20, 10}) {
		t.Fatalf("case-sensitive PostgreSQL source ordering changed: got %v; SQL=%s", values, r.Statements[0].ConvertedSQL)
	}
}

func TestConvertReviewPrimaryKeyNullBind(t *testing.T) {
	c := converterCatalog(t)
	src := `INSERT INTO public.documents(id,flags,embedding) VALUES($1,1,ARRAY[1,2,3]);`
	r, err := Convert(context.Background(), src, c, localConvertOptions())
	if err != nil {
		t.Fatal(err)
	}
	if r.Status == "unsupported" {
		return
	}
	db := reviewConverterDB(t)
	if len(r.Statements[0].Bindings) != 1 {
		t.Fatal("missing primary-key binding contract")
	}
	v, err := r.Statements[0].Bindings[0].ConvertValue(nil)
	if err != nil {
		return
	}
	if _, err = db.Exec(context.Background(), r.Statements[0].ConvertedSQL, v); err == nil {
		t.Fatal("source NOT NULL primary key became local automatic rowid allocation")
	}
}

func TestConvertReviewNegativeConstantLimit(t *testing.T) {
	r, err := Convert(context.Background(), `SELECT id FROM public.documents LIMIT -1;`, converterCatalog(t), localConvertOptions())
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "unsupported" {
		t.Fatalf("negative PostgreSQL LIMIT must not become local unbounded SELECT: %s", r.Statements[0].ConvertedSQL)
	}
}

func TestConvertReviewNegativeLimitBind(t *testing.T) {
	r, err := Convert(context.Background(), `SELECT id FROM public.documents LIMIT $1;`, converterCatalog(t), localConvertOptions())
	if err != nil {
		t.Fatal(err)
	}
	if r.Status == "unsupported" {
		return
	}
	if len(r.Statements[0].Bindings) != 1 {
		t.Fatal("missing LIMIT binding contract")
	}
	if _, err := r.Statements[0].Bindings[0].ConvertValue(int64(-1)); err == nil {
		t.Fatal("negative LIMIT parameter accepted")
	}
}

func TestConvertReviewUnverifiedTextCollation(t *testing.T) {
	c := converterCatalog(t)
	c.Types = append(c.Types, Type{OID: 25, Schema: "pg_catalog", Name: "text", Kind: "b"})
	c.Relations[0].Columns = append(c.Relations[0].Columns, Column{AttNum: 4, Name: "note", TypeOID: 25, TypeMod: -1})
	for _, src := range []string{
		`SELECT note FROM public.documents ORDER BY note;`,
		`SELECT id FROM public.documents WHERE note = 'A';`,
		`SELECT id FROM public.documents WHERE note < 'a';`,
	} {
		r, err := Convert(context.Background(), src, c, localConvertOptions())
		if err != nil {
			t.Fatal(err)
		}
		if r.Status != "unsupported" {
			t.Errorf("unverified source text collation converted: %s", r.Statements[0].ConvertedSQL)
		}
	}
}

func TestConvertReviewLocalLexerDivergenceBlocked(t *testing.T) {
	for _, src := range []string{
		"SELECT id FROM public.documents -- before predicate\r WHERE id=1;",
		"SELECT id FROM public.documents /* outer /* inner */ WHERE id=1 /* tail */ */;",
	} {
		r, err := Convert(context.Background(), src, converterCatalog(t), localConvertOptions())
		if err != nil {
			t.Fatal(err)
		}
		if r.Status != "unsupported" {
			t.Fatalf("lexical semantic difference not blocked: %s", r.Statements[0].ConvertedSQL)
		}
		st := r.Statements[0]
		if st.OriginalSQL != src || st.ConvertedSQL != "" || len(st.Edits) != 0 {
			t.Fatal("unsupported source not retained atomically")
		}
	}
}

func TestConvertReviewBindBinary32BoundaryBits(t *testing.T) {
	bits := []uint32{0x80000000, 1, 0x007fffff, 0x00800000, 0x3f800001, 0x7f7fffff}
	values := make([]float32, len(bits))
	for i, b := range bits {
		values[i] = math.Float32frombits(b)
	}
	for _, target := range []SQLTarget{TargetLocal, TargetPostgres} {
		transform := BindTransform{Parameter: 1, SourceKind: "real[]", Target: target, Dimensions: len(bits), Action: "real_array_to_vector"}
		value, err := transform.ConvertValue(values)
		if err != nil {
			t.Fatal(err)
		}
		var v vector.Vector
		if target == TargetLocal {
			v, err = vector.Decode(value.([]byte))
		} else {
			v, err = vector.Parse(value.(string))
		}
		if err != nil {
			t.Fatal(err)
		}
		actual := v.Values()
		for i, b := range bits {
			if math.Float32bits(actual[i]) != b {
				t.Fatalf("%s component %d bits changed", target, i)
			}
		}
		if !reflect.DeepEqual(values, v.Values()) {
			t.Fatal("bind components changed")
		}
	}
}

func TestConvertReviewSpanReplayAndAtomicReport(t *testing.T) {
	src := `/* lead */ SELECT d.id, d.embedding /* metric */ <~> CAST(ARRAY[1,/* component */2,3] AS real[]) AS "Distance", 'literal <~>; ARRAY[5]' AS note FROM public.documents AS d ORDER BY "Distance" DESC NULLS /* ordering */ FIRST,d.id; -- tail
SELECT id FROM public.documents; DROP TABLE public.documents;`
	r, err := Convert(context.Background(), src, converterCatalog(t), localConvertOptions())
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "unsupported" || len(r.Statements) != 3 {
		t.Fatal("script not rejected as a whole")
	}
	if sql, err := r.SQL(); err == nil || sql != "" || r.TargetSHA256 != "" {
		t.Fatal("blocked script leaked executable output or target hash")
	}
	st := r.Statements[0]
	if st.Status != "converted" {
		t.Fatalf("supported leading statement unexpectedly rejected: %+v", st.Findings)
	}
	replay, err := applySQLEdits(src, st.Span, st.Edits)
	if err != nil || replay != st.ConvertedSQL {
		t.Fatal("nonoverlapping span replay failed", err)
	}
	for _, trivia := range []string{"/* lead */", "/* metric */", "/* component */", "/* ordering */", "'literal <~>; ARRAY[5]'"} {
		if strings.Count(st.ConvertedSQL, trivia) != 1 {
			t.Fatalf("original trivia/literal %q changed or duplicated", trivia)
		}
	}
	blocked := r.Statements[2]
	if blocked.Status != "unsupported" || len(blocked.Edits) != 0 || len(blocked.Bindings) != 0 || len(blocked.Settings) != 0 || blocked.ConvertedSQL != "" || src[blocked.Span.Start:blocked.Span.End] != blocked.OriginalSQL {
		t.Fatal("blocked statement artifacts not atomic")
	}
	again, err := Convert(context.Background(), src, converterCatalog(t), localConvertOptions())
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(r)
	b, _ := json.Marshal(again)
	if !bytes.Equal(a, b) {
		t.Fatal("same source/catalog/options report differs")
	}
}

func TestConvertReviewEditOverlapRejected(t *testing.T) {
	for _, edits := range [][]SQLEdit{
		{{Span: SQLSpan{1, 4}, Replacement: "x"}, {Span: SQLSpan{3, 5}, Replacement: "y"}},
		{{Span: SQLSpan{0, 1}, Replacement: "x"}},
		{{Span: SQLSpan{2, 7}, Replacement: "x"}},
	} {
		if out, err := applySQLEdits("abcdef", SQLSpan{1, 6}, edits); err == nil || out != "" {
			t.Fatal("invalid or overlapping edits escaped", out, err)
		}
	}
}

func TestConvertReviewStaticErrorDoesNotAllocatePrimaryKey(t *testing.T) {
	src := `INSERT INTO public.documents(id,flags,embedding) VALUES(1/0,1,ARRAY[1,2,3]);`
	r, err := Convert(context.Background(), src, converterCatalog(t), localConvertOptions())
	if err != nil {
		t.Fatal(err)
	}
	if r.Status == "unsupported" {
		return
	}
	db := reviewConverterDB(t)
	if _, err = db.Exec(context.Background(), r.Statements[0].ConvertedSQL); err == nil {
		t.Fatal("source division error became a successful insert with an allocated primary key")
	}
}

func TestConvertReviewQuotedTypeAliasIdentity(t *testing.T) {
	for _, src := range []string{
		`CREATE TABLE public.documents(id "integer" PRIMARY KEY,flags INTEGER NOT NULL,embedding REAL[]);`,
		`SELECT embedding <~> ARRAY[1,2,3]::"real"[] FROM public.documents;`,
		`SELECT embedding <~> ARRAY[1,2,3]::pg_catalog.real[] FROM public.documents;`,
	} {
		r, err := Convert(context.Background(), src, converterCatalog(t), localConvertOptions())
		if err != nil {
			t.Fatal(err)
		}
		if r.Status != "unsupported" {
			t.Errorf("SQL type alias was mistaken for a catalog type name: %s", r.Statements[0].ConvertedSQL)
		}
	}
}

func TestConvertReviewUnqualifiedTypeShadow(t *testing.T) {
	c := converterCatalog(t)
	c.Types = append(c.Types, Type{OID: 49000, Schema: "shadow", Name: "float4", Kind: "b"})
	src := `SELECT embedding <~> ARRAY[1,2,3]::float4[] FROM public.documents;`
	r, err := Convert(context.Background(), src, c, localConvertOptions())
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "unsupported" {
		t.Fatalf("unqualified named type with supplied catalog shadow was rewritten: %s", r.Statements[0].ConvertedSQL)
	}
}

func TestConvertReviewImplicitOutputAliasAmbiguity(t *testing.T) {
	for _, src := range []string{
		`SELECT id,flags AS id FROM public.documents ORDER BY id;`,
		`SELECT flags AS id,id FROM public.documents ORDER BY id;`,
	} {
		r, err := Convert(context.Background(), src, converterCatalog(t), localConvertOptions())
		if err != nil {
			t.Fatal(err)
		}
		if r.Status != "unsupported" {
			t.Errorf("ambiguous source output-column label converted: %s", r.Statements[0].ConvertedSQL)
		}
	}
}

func TestConvertReviewExplicitMappingRequired(t *testing.T) {
	opts := localConvertOptions()
	opts.Tables = nil
	source := `SELECT embedding <~> ARRAY[1,2,3] FROM public.documents;`
	r, err := Convert(context.Background(), source, converterCatalog(t), opts)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "unsupported" || r.Statements[0].ConvertedSQL != "" || len(r.Statements[0].Edits) != 0 {
		t.Fatal("relation without explicit mapping yielded executable SQL")
	}
	opts = localConvertOptions()
	opts.Tables[0].TargetName = `mapped";table`
	r = converted(t, converterCatalog(t), `SELECT documents.id FROM public.documents;`, opts)
	if !strings.Contains(r.Statements[0].ConvertedSQL, `"mapped"";table"."id"`) || !strings.Contains(r.Statements[0].ConvertedSQL, `FROM "mapped"";table"`) {
		t.Fatal("qualified relation mapping not quoted consistently", r.Statements[0].ConvertedSQL)
	}
}

func TestConvertReviewComparisonBindIntegerWidth(t *testing.T) {
	r := converted(t, converterCatalog(t), `SELECT id FROM public.documents WHERE id=$1;`, localConvertOptions())
	if len(r.Statements[0].Bindings) != 1 {
		t.Fatal("missing comparison bind contract")
	}
	b := r.Statements[0].Bindings[0]
	for _, value := range []any{int64(2147483648), "not-an-int", 1.5} {
		if _, err := b.ConvertValue(value); err == nil {
			t.Errorf("source-inferred int4 comparison parameter accepted incompatible value of type %T", value)
		}
	}
}

func TestConvertReviewRealArrayLiteralUnderflowError(t *testing.T) {
	for _, target := range []SQLTarget{TargetLocal, TargetPostgres} {
		opts := localConvertOptions()
		opts.Target = target
		if target == TargetPostgres {
			opts.Tables[0].TargetSchema = "public"
			opts.VectorSchema = "vectors"
		}
		src := `SELECT embedding <~> '{1e-999,2,3}'::real[] FROM public.documents;`
		r, err := Convert(context.Background(), src, converterCatalog(t), opts)
		if err != nil {
			t.Fatal(err)
		}
		if r.Status != "unsupported" {
			t.Errorf("%s source real[] input underflow error changed to zero: %s", target, r.Statements[0].ConvertedSQL)
		}
		b := BindTransform{Parameter: 1, SourceKind: "real[]", Target: target, Dimensions: 3, Action: "real_array_to_vector"}
		for _, text := range []string{`{1e-999,2,3}`, `[1:3]={"-1e-999",2,3}`, `{1,2,"1\e-999"}`} {
			if _, err := b.ConvertValue(text); err == nil {
				t.Errorf("%s nonzero-underflow array bind accepted", target)
			}
		}
		for _, text := range []string{`{-0e999,2,3}`, `[1:3]={"-0",1.401298464324817e-45,3}`} {
			if _, err := b.ConvertValue(text); err != nil {
				t.Errorf("%s true zero/subnormal array bind rejected: %v", target, err)
			}
		}
	}
}

func TestConvertReviewMixedRealParameterWidths(t *testing.T) {
	c := converterCatalog(t)
	c.Types = append(c.Types, Type{OID: 701, Schema: "pg_catalog", Name: "float8", Kind: "b"})
	c.Relations[0].Columns = append(c.Relations[0].Columns, Column{AttNum: 4, Name: "small_real", TypeOID: 700, TypeMod: -1}, Column{AttNum: 5, Name: "wide_real", TypeOID: 701, TypeMod: -1})
	src := `SELECT id FROM public.documents WHERE small_real=$1 OR wide_real=$1;`
	r, err := Convert(context.Background(), src, c, localConvertOptions())
	if err != nil {
		t.Fatal(err)
	}
	if r.Status == "unsupported" {
		return
	}
	if len(r.Statements[0].Bindings) != 1 {
		t.Fatal("missing repeated real binding")
	}
	if _, err := r.Statements[0].Bindings[0].ConvertValue(float64(0.1)); err == nil {
		t.Fatal("later float8 context erased the earlier proven float4 parameter precision")
	}
}

func TestConvertReviewImplicitAliasNullOrdering(t *testing.T) {
	c := converterCatalog(t)
	c.Relations[0].Columns = append(c.Relations[0].Columns, Column{AttNum: 4, Name: "manhattan_distance", TypeOID: 23, TypeMod: -1})
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "implicit-label.db"), sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, src := range []string{
		`CREATE TABLE public.documents(id INTEGER PRIMARY KEY,flags INTEGER NOT NULL,embedding REAL[],manhattan_distance INTEGER);`,
		`INSERT INTO public.documents(id,flags,embedding,manhattan_distance) VALUES(1,1,ARRAY[1,2,3],NULL),(2,1,NULL,1);`,
	} {
		r := converted(t, c, src, localConvertOptions())
		if _, err := db.Exec(context.Background(), r.Statements[0].ConvertedSQL); err != nil {
			t.Fatal(err)
		}
	}
	src := `SELECT id,legacy.manhattan_distance(embedding,ARRAY[1,2,3]) FROM public.documents ORDER BY manhattan_distance,id;`
	r, err := Convert(context.Background(), src, c, localConvertOptions())
	if err != nil {
		t.Fatal(err)
	}
	if r.Status == "unsupported" {
		return
	}
	rows, err := db.Query(context.Background(), r.Statements[0].ConvertedSQL)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if !reflect.DeepEqual(rows.Columns(), []string{"id", "manhattan_distance"}) {
		t.Fatal("implicit source result labels not preserved", rows.Columns())
	}
	var ids []int64
	for rows.Next() {
		ids = append(ids, rows.Values()[0].(int64))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ids, []int64{1, 2}) {
		t.Fatalf("implicit output alias NULL order resolved physical column: %v; SQL=%s", ids, r.Statements[0].ConvertedSQL)
	}
}

func TestConvertReviewIntegerSpelledFloat4Assignment(t *testing.T) {
	c := converterCatalog(t)
	c.Relations[0].Columns[1].TypeOID = 700
	src := `CREATE TABLE public.documents(id INTEGER PRIMARY KEY,flags REAL NOT NULL,embedding REAL[]); INSERT INTO public.documents(id,flags,embedding) VALUES(1,16777217,ARRAY[1,2,3]),(2,-16777217,ARRAY[1,2,3]); SELECT flags FROM public.documents ORDER BY id;`
	r, err := Convert(context.Background(), src, c, localConvertOptions())
	if err != nil {
		t.Fatal(err)
	}
	if r.Status == "unsupported" {
		return
	}
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "float-width.db"), sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, st := range r.Statements[:2] {
		if _, err := db.Exec(context.Background(), st.ConvertedSQL); err != nil {
			t.Fatal(err)
		}
	}
	rows, err := db.Query(context.Background(), r.Statements[2].ConvertedSQL)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var actual []float64
	for rows.Next() {
		actual = append(actual, rows.Values()[0].(float64))
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(actual, []float64{16777216, -16777216}) {
		t.Fatalf("source float4 assignment rounding changed: %v", actual)
	}
}
