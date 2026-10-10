package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func converterCatalog(t *testing.T) CatalogSnapshot {
	c := fixtureCatalog(t)
	c.Relations[0].Columns = c.Relations[0].Columns[:3]
	return c
}
func localConvertOptions() ConvertOptions {
	return ConvertOptions{Target: TargetLocal, Tables: []TableMapping{{RelationOID: 30000, TargetName: "documents"}}, StandardConformingStrings: true}
}
func converted(t *testing.T, c CatalogSnapshot, source string, opts ConvertOptions) Conversion {
	t.Helper()
	r, err := Convert(context.Background(), source, c, opts)
	if err != nil {
		t.Fatal(err)
	}
	if r.Status != "converted" {
		b, _ := json.MarshalIndent(r, "", " ")
		t.Fatalf("not converted: %s", b)
	}
	return r
}

const converterSample = `-- literal <~> and ARRAY[9,9,9] are comment bytes.
CREATE TABLE public.documents(id INTEGER PRIMARY KEY, flags INTEGER NOT NULL, embedding REAL[]);
INSERT INTO public.documents(id,flags,embedding) VALUES (1,1,ARRAY[1,0,0]),(2,1,'{0,1,0}'::real[]),(3,1,ARRAY[1,2,3]),(4,1,NULL);
UPDATE public.documents SET embedding='{3,1,0}'::real[] WHERE id=2;
CREATE INDEX documents_manhattan ON public.documents USING hnsw(embedding legacy.ann_manhattan_ops) WITH(dims=3,m=16,efconstruction=64,efsearch=64);
SELECT id, embedding <~> ARRAY[1,2,3] AS distance, 'literal <~> ARRAY[9,9,9]' AS note FROM public.documents WHERE flags & 3 = 1 ORDER BY embedding <~> ARRAY[1,2,3], id LIMIT 10;
`

func TestConvertLocalExecuteExactAndNormalReopen(t *testing.T) {
	c := converterCatalog(t)
	r := converted(t, c, converterSample, localConvertOptions())
	script, err := r.SQL()
	if err != nil {
		t.Fatal(err)
	}
	if r.SourceSHA256 != sqlHash(converterSample) || r.TargetSHA256 != sqlHash(script) {
		t.Fatal("hash mismatch")
	}
	if !strings.Contains(script, "'literal <~> ARRAY[9,9,9]'") || !strings.Contains(script, "-- literal <~> and ARRAY[9,9,9] are comment bytes.") || !strings.Contains(script, "flags & 3") {
		t.Fatal("unrelated bytes changed", script)
	}
	path := filepath.Join(t.TempDir(), "converted.db")
	db, err := sqlite.Open(path, sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for i, st := range r.Statements {
		if _, err := sqlast.Parse(st.ConvertedSQL); err != nil {
			t.Fatalf("local parser statement %d: %v", i, err)
		}
		if st.Kind != "select" {
			if _, err = db.Exec(context.Background(), st.ConvertedSQL); err != nil {
				t.Fatalf("execute %s: %v", st.ConvertedSQL, err)
			}
		}
	}
	query := r.Statements[len(r.Statements)-1].ConvertedSQL
	wantIDs := []int64{3, 1, 2, 4}
	wantDist := []any{float64(0), float64(5), float64(6), nil}
	check := func() {
		rows, err := db.Query(context.Background(), query)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		i := 0
		for rows.Next() {
			v := rows.Values()
			if i >= len(wantIDs) || v[0] != wantIDs[i] || v[1] != wantDist[i] || v[2] != "literal <~> ARRAY[9,9,9]" {
				t.Fatalf("row%d: %v", i, v)
			}
			i++
		}
		if err = rows.Err(); err != nil {
			t.Fatal(err)
		}
		if i != len(wantIDs) {
			t.Fatal(i)
		}
	}
	check()
	if err = db.CheckExtensionIndex(context.Background(), "documents_manhattan"); err != nil {
		t.Fatal(err)
	}
	if err = db.Close(); err != nil {
		t.Fatal(err)
	}
	db, err = sqlite.Open(path, sqlite.Options{NoCreate: true})
	if err != nil {
		t.Fatal(err)
	}
	check()
	// Independent fixture math, not the vector implementation as its own oracle.
	vectors := map[int64][]float64{1: {1, 0, 0}, 2: {3, 1, 0}, 3: {1, 2, 3}}
	queryVector := []float64{1, 2, 3}
	for _, metric := range []struct {
		op   string
		calc func([]float64, []float64) float64
	}{{"<->", func(a, b []float64) float64 {
		sum := 0.0
		for i := range a {
			d := a[i] - b[i]
			sum += d * d
		}
		return math.Sqrt(sum)
	}}, {"<=>", func(a, b []float64) float64 {
		dot, aa, bb := 0.0, 0.0, 0.0
		for i := range a {
			dot += a[i] * b[i]
			aa += a[i] * a[i]
			bb += b[i] * b[i]
		}
		return 1 - dot/math.Sqrt(aa*bb)
	}}, {"<~>", func(a, b []float64) float64 {
		sum := 0.0
		for i := range a {
			sum += math.Abs(a[i] - b[i])
		}
		return sum
	}}} {
		sql := fmt.Sprintf("SELECT id, embedding %s ARRAY[1,2,3] FROM public.documents WHERE id < 4 ORDER BY id;", metric.op)
		cr := converted(t, c, sql, localConvertOptions())
		rows, e := db.Query(context.Background(), cr.Statements[0].ConvertedSQL)
		if e != nil {
			t.Fatal(e)
		}
		for rows.Next() {
			v := rows.Values()
			want := metric.calc(vectors[v[0].(int64)], queryVector)
			got := v[1].(float64)
			if math.Abs(got-want) > 1e-12*math.Max(1, math.Abs(want)) {
				t.Fatalf("%s %v vs %.17g", metric.op, v, want)
			}
		}
		if e = rows.Err(); e != nil {
			t.Fatal(e)
		}
		rows.Close()
	}
	if len(r.Statements[3].Settings) != 1 || r.Statements[3].Settings[0].Value != 64 || strings.Contains(r.Statements[3].ConvertedSQL, "efsearch=") {
		t.Fatal("setting scope", r.Statements[3])
	}
}
func TestConvertSourceTriviaAndTypedOperators(t *testing.T) {
	c := converterCatalog(t)
	source := `/* before <~> */ SELECT id, 'a''b <~> ARRAY[0]' AS literal, embedding /* op comment */ <~> ARRAY[1, /*inside*/ 2,3] AS d FROM public.documents WHERE ~flags & 3 = 2 ORDER BY embedding <~> ARRAY[1,2,3] NULLS LAST; -- tail <~>
`
	r := converted(t, c, source, localConvertOptions())
	sql, _ := r.SQL()
	for _, part := range []string{"/* before <~> */", "'a''b <~> ARRAY[0]'", "/* op comment */", "/*inside*/", "~flags", "& 3", "-- tail <~>"} {
		if strings.Count(sql, part) != 1 {
			t.Fatalf("part %q not preserved once: %s", part, sql)
		}
	}
	if strings.Contains(sql, "NULLS LAST") || !strings.Contains(sql, "IS NULL ASC") || !strings.Contains(sql, "<+>") {
		t.Fatal(sql)
	}
	for _, st := range r.Statements {
		replayed, e := applySQLEdits(source, st.Span, st.Edits)
		if e != nil || replayed != st.ConvertedSQL {
			t.Fatal("span replay", e)
		}
	}
}
func TestConvertUnsupportedIsNeverScript(t *testing.T) {
	c := converterCatalog(t)
	cases := []string{
		`SELECT id FROM public.documents ORDER BY ordinary_samples <~> ARRAY[1,2,3];`,
		`SELECT id FROM public.documents ORDER BY '{1,2,3}'::real[] <~> '{1,2,3}'::real[];`,
		`SELECT id FROM public.documents ORDER BY embedding <^> ARRAY[1,2,3];`,
		`SELECT id FROM public.documents ORDER BY embedding <~> ARRAY[1,ARRAY[2,3]];`,
		`SELECT id FROM public.documents ORDER BY embedding <~> ARRAY[1,id,3];`,
		`SELECT id FROM public.documents ORDER BY embedding <~> ARRAY[1,2,3]::unknown;`,
		`SELECT $1,embedding <~> $1 FROM public.documents;`,
		`SELECT id FROM public.documents d JOIN public.documents e ON d.id=e.id;`,
		`CREATE INDEX bad ON public.documents USING hnsw(embedding unknown) WITH(dims=3,m=16,efconstruction=64);`,
		`CREATE INDEX bad ON public.documents USING hnsw(embedding legacy.ann_l2_ops) WITH(dims=3,m=16,efconstruction=64,mystery=1);`,
		`CREATE INDEX bad ON public.documents USING hnsw(embedding legacy.ann_l2_ops) WITH(dims=3);`,
		`CREATE INDEX bad ON public.documents USING hnsw(embedding legacy.ann_l2_ops) WITH(dims=3,m=3,efconstruction=5);`,
		`CREATE INDEX bad ON public.documents USING hnsw(embedding legacy.ann_l2_ops) WITH(dims=4096,m=16,efconstruction=64);`,
		`CREATE TABLE public.documents(id BIGSERIAL PRIMARY KEY, flags INTEGER NOT NULL,embedding REAL[]);`,
		`SELECT $$body <~>; ARRAY[3]$$ FROM public.documents;`,
		`SELECT E'escaped <~>' FROM public.documents;`,
		`SELECT id FROM public.documents; CREATE FUNCTION f() RETURNS text AS $$SELECT '<~>';$$ LANGUAGE SQL;`,
	}
	for _, source := range cases {
		t.Run(source, func(t *testing.T) {
			r, err := Convert(context.Background(), source, c, localConvertOptions())
			if err != nil {
				t.Fatal(err)
			}
			if r.Status != "unsupported" {
				t.Fatal("accepted", r)
			}
			if _, err = r.SQL(); err == nil {
				t.Fatal("unsupported script returned")
			}
			for _, st := range r.Statements {
				if st.Status == "unsupported" {
					if st.ConvertedSQL != "" || len(st.Edits) != 0 || len(st.Findings) == 0 {
						t.Fatal(st)
					}
					if source[st.Span.Start:st.Span.End] != st.OriginalSQL {
						t.Fatal("original evidence changed")
					}
					last := st.Findings[len(st.Findings)-1]
					if last.Span.Start < st.Span.Start || last.Span.End > st.Span.End {
						t.Fatal("bad failure span", last)
					}
				}
			}
		})
	}
}
func TestConvertBindsFunctionsAndPostgresReview(t *testing.T) {
	c := converterCatalog(t)
	source := `SELECT id,legacy.manhattan_distance(embedding,$2::real[]) AS d FROM public.documents WHERE id=$1 ORDER BY embedding <~> $2 LIMIT 10;`
	r := converted(t, c, source, localConvertOptions())
	st := r.Statements[0]
	if len(st.Bindings) != 2 || st.Bindings[0].Action != "identity" || st.Bindings[1].Action != "real_array_to_vector" || st.Bindings[1].Dimensions != 3 {
		t.Fatal(st.Bindings)
	}
	got, err := st.Bindings[1].ConvertValue("{-0,1.401298464324817e-45,2}")
	if err != nil {
		t.Fatal(err)
	}
	v, err := vector.Decode(got.([]byte))
	if err != nil {
		t.Fatal(err)
	}
	if math.Float32bits(v.Values()[0]) != 0x80000000 || math.Float32bits(v.Values()[1]) != 1 {
		t.Fatal("bind lost binary32")
	}
	for _, invalid := range []any{"{1,2}", []float64{1, 2, 3}, "{NULL,1,2}", "{NaN,1,2}"} {
		if _, err = st.Bindings[1].ConvertValue(invalid); err == nil {
			t.Fatal("invalid bind accepted")
		}
	}
	if value, err := st.Bindings[1].ConvertValue(nil); err != nil || value != nil {
		t.Fatal("null bind")
	}
	opts := localConvertOptions()
	opts.Target = TargetPostgres
	opts.VectorSchema = "vectors"
	opts.Tables[0].TargetSchema = "reviewed"
	pg := converted(t, c, converterSample, opts)
	pgSQL, _ := pg.SQL()
	if pg.Validation != "postgresql_source_review_only_not_executed" || !strings.Contains(pgSQL, `OPERATOR("vectors".<+>)`) || !strings.Contains(pgSQL, `"vectors".vector(3)`) || strings.Contains(pgSQL, "IS NULL ASC") || strings.Contains(pgSQL, "BEGIN;") {
		t.Fatal(pgSQL)
	}
	if pg.Statements[3].Settings[0].ReviewSQL != "SET LOCAL hnsw.ef_search = 64;" {
		t.Fatal(pg.Statements[3].Settings)
	}
	again := converted(t, c, converterSample, opts)
	a, _ := json.Marshal(pg)
	z, _ := json.Marshal(again)
	if !bytes.Equal(a, z) {
		t.Fatal("nondeterministic output")
	}
}
func TestConvertCatalogIdentityAndLimits(t *testing.T) {
	source := `SELECT embedding <~> ARRAY[1,2,3] FROM public.documents;`
	for _, mutate := range []func(*CatalogSnapshot){func(c *CatalogSnapshot) { c.Operators[2].ExtensionOID = 0 }, func(c *CatalogSnapshot) { c.Functions[3].ReturnOID = 20 }, func(c *CatalogSnapshot) { c.Extensions[0].Version = "0.3.5" }, func(c *CatalogSnapshot) { c.OpClasses[2].Name = "shadow" }} {
		c := converterCatalog(t)
		mutate(&c)
		r, e := Convert(context.Background(), source, c, localConvertOptions())
		if e != nil {
			t.Fatal(e)
		}
		if r.Status != "unsupported" {
			t.Fatal("bad identity accepted")
		}
	}
	c := converterCatalog(t)
	c.Functions = append(c.Functions, Function{OID: 40001, Schema: "public", Name: "manhattan_distance", ArgumentOIDs: []uint32{1021, 1021}, ReturnOID: 700})
	r, e := Convert(context.Background(), `SELECT manhattan_distance(embedding,ARRAY[1,2,3]) FROM public.documents;`, c, localConvertOptions())
	if e != nil || r.Status != "unsupported" {
		t.Fatal("shadow accepted", e, r.Status)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = Convert(ctx, source, converterCatalog(t), localConvertOptions()); e != context.Canceled {
		t.Fatal(e)
	}
	opts := localConvertOptions()
	opts.Limits = DefaultSQLLimits()
	opts.Limits.MaxBytes = 5
	if _, e = Convert(context.Background(), source, converterCatalog(t), opts); e == nil {
		t.Fatal("bytes bound")
	}
	for _, source := range []string{"SELECT 'secret", "SELECT 1 /* secret", "\\copy secret", "SELECT \x00", "SELECT \xff"} {
		if _, e = Convert(context.Background(), source, converterCatalog(t), localConvertOptions()); e == nil {
			t.Fatal("invalid lexing")
		}
	}
}
func TestConvertGoldenArtifacts(t *testing.T) {
	c := converterCatalog(t)
	for _, name := range []string{"local", "postgres"} {
		opts := localConvertOptions()
		if name == "postgres" {
			opts.Target = TargetPostgres
			opts.VectorSchema = "public"
			opts.Tables[0].TargetSchema = "public"
		}
		r := converted(t, c, converterSample, opts)
		sql, _ := r.SQL()
		path := "testdata/converter/" + name + ".sql"
		want, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		if sql != string(want) {
			t.Fatalf("%s differs", name)
		}
		var report bytes.Buffer
		if e = RenderConversionJSON(&report, r); e != nil {
			t.Fatal(e)
		}
		want, e = os.ReadFile("testdata/converter/" + name + ".json")
		if e != nil || !bytes.Equal(report.Bytes(), want) {
			t.Fatalf("%s JSON differs: %v", name, e)
		}

	}
}
func TestConvertBindValueInputImmutability(t *testing.T) {
	p := BindTransform{Parameter: 1, SourceKind: "real[]", Target: TargetPostgres, Dimensions: 3, Action: "real_array_to_vector"}
	src := []float32{1, 2, 3}
	before := append([]float32{}, src...)
	v, e := p.ConvertValue(src)
	if e != nil || v != "[1,2,3]" || !reflect.DeepEqual(src, before) {
		t.Fatal(v, e)
	}
}

func TestConvertNullOrderAliasesCastAndBindingsExecute(t *testing.T) {
	c := converterCatalog(t)
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "values.db"), sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for _, source := range []string{`CREATE TABLE public.documents(id INTEGER PRIMARY KEY,flags INTEGER NOT NULL,embedding REAL[]);`, `INSERT INTO public.documents(id,flags,embedding) VALUES(1,1,CAST('{1,0,0}' AS real[])),(2,1,NULL),(3,1,ARRAY[1,2,3]);`} {
		r := converted(t, c, source, localConvertOptions())
		if _, e := db.Exec(context.Background(), r.Statements[0].ConvertedSQL); e != nil {
			t.Fatal(e, r.Statements[0].ConvertedSQL)
		}
	}
	tests := []struct {
		suffix string
		ids    []int64
	}{{"ASC", []int64{3, 1, 2}}, {"DESC", []int64{2, 1, 3}}, {"ASC NULLS FIRST", []int64{2, 3, 1}}, {"ASC NULLS LAST", []int64{3, 1, 2}}, {"DESC NULLS LAST", []int64{1, 3, 2}}, {"DESC NULLS FIRST", []int64{2, 1, 3}}}
	for _, tt := range tests {
		source := `SELECT id,embedding <~> $1 AS d FROM public.documents ORDER BY d ` + tt.suffix + `,id;`
		r := converted(t, c, source, localConvertOptions())
		bind, e := r.Statements[0].Bindings[0].ConvertValue("{1,2,3}")
		if e != nil {
			t.Fatal(e)
		}
		rows, e := db.Query(context.Background(), r.Statements[0].ConvertedSQL, bind)
		if e != nil {
			t.Fatal(e, r.Statements[0].ConvertedSQL)
		}
		var ids []int64
		for rows.Next() {
			ids = append(ids, rows.Values()[0].(int64))
		}
		if e = rows.Err(); e != nil {
			t.Fatal(e)
		}
		rows.Close()
		if !reflect.DeepEqual(ids, tt.ids) {
			t.Fatalf("%s: %v", tt.suffix, ids)
		}
	}
}
func TestConvertBitOperatorIdentityAndNoLegacyRewrite(t *testing.T) {
	c := converterCatalog(t)
	c.Extensions = append(c.Extensions, Extension{OID: 40000, Name: "vector", Version: "0.8.7", Schema: "modern"})
	c.Types = append(c.Types, Type{OID: 1560, Schema: "pg_catalog", Name: "bit", Kind: "b"}, Type{OID: 701, Schema: "pg_catalog", Name: "float8", Kind: "b"})
	for i, name := range []string{"bits_a", "bits_b"} {
		c.Relations[0].Columns = append(c.Relations[0].Columns, Column{AttNum: 4 + i, Name: name, TypeOID: 1560, TypeMod: 3, NotNull: true})
	}
	c.Functions = append(c.Functions, Function{OID: 40001, Schema: "modern", Name: "hamming_distance", ArgumentOIDs: []uint32{1560, 1560}, ReturnOID: 701, ExtensionOID: 40000})
	c.Operators = append(c.Operators, Operator{OID: 40002, Schema: "modern", Name: "<~>", LeftOID: 1560, RightOID: 1560, FunctionOID: 40001, ExtensionOID: 40000})
	source := `SELECT bits_a <~> bits_b AS h,embedding <~> ARRAY[1,2,3] AS l1 FROM public.documents;`
	r := converted(t, c, source, localConvertOptions())
	sql := r.Statements[0].ConvertedSQL
	if !strings.Contains(sql, "bits_a <~> bits_b") || !strings.Contains(sql, "embedding <+>") {
		t.Fatal(sql)
	}
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "bit-coexist.db"), sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	create := `CREATE TABLE public.documents(id INTEGER PRIMARY KEY,flags INTEGER NOT NULL,embedding REAL[],bits_a BIT(3) NOT NULL,bits_b BIT(3) NOT NULL);`
	insert := `INSERT INTO public.documents(id,flags,embedding,bits_a,bits_b) VALUES(1,1,ARRAY[1,2,3],'101','011');`
	for _, input := range []string{create, insert} {
		cr := converted(t, c, input, localConvertOptions())
		if _, e := db.Exec(context.Background(), cr.Statements[0].ConvertedSQL); e != nil {
			t.Fatal(e)
		}
	}
	rows, e := db.Query(context.Background(), sql)
	if e != nil {
		t.Fatal(e)
	}
	if !rows.Next() || !reflect.DeepEqual(rows.Values(), []any{float64(2), float64(0)}) {
		t.Fatal("bit/dense operator execution", rows.Values())
	}
	rows.Close()

	c.Operators[len(c.Operators)-1].ExtensionOID = 0
	r, err = Convert(context.Background(), source, c, localConvertOptions())
	if err != nil || r.Status != "unsupported" {
		t.Fatal("unknown bit overload accepted", err)
	}
}
func TestConvertOnlyTriviaAndQuotedNames(t *testing.T) {
	for _, source := range []string{"", " -- <~> ARRAY[1]\n/* ordinary */ ", ";;; -- empty\n"} {
		r := converted(t, converterCatalog(t), source, localConvertOptions())
		sql, e := r.SQL()
		if e != nil || sql != source {
			t.Fatal("trivia changed", sql, e)
		}
	}
	c := converterCatalog(t)
	c.Relations[0].Name = `odd";table`
	c.Relations[0].Columns[2].Name = `vec<~>"name`
	opts := localConvertOptions()
	opts.Tables[0].TargetName = `local";table`
	source := `SELECT "vec<~>""name" <~> ARRAY[1,2,3] FROM public."odd"";table";`
	r := converted(t, c, source, opts)
	sql, _ := r.SQL()
	if !strings.Contains(sql, `"vec<~>""name" <+>`) || !strings.Contains(sql, `"local"";table"`) {
		t.Fatal(sql)
	}
}
func TestConvertUnselectedArraysSchemaAndParameterConflicts(t *testing.T) {
	c := fixtureCatalog(t)
	source := `SELECT ordinary_samples FROM public.documents;`
	r, e := Convert(context.Background(), source, c, localConvertOptions())
	if e != nil || r.Status != "unsupported" {
		t.Fatal(r, e)
	}
	opts := localConvertOptions()
	opts.Target = TargetPostgres
	opts.VectorSchema = "public"
	opts.Tables[0].TargetSchema = "public"
	r = converted(t, c, source, opts)
	if strings.Contains(r.Statements[0].ConvertedSQL, "vector") {
		t.Fatal("unselected retyped")
	}
	c = converterCatalog(t)
	c.Relations[0].Columns[2].Name = "Flags"
	r, e = Convert(context.Background(), `SELECT flags FROM public.documents;`, c, localConvertOptions())
	if e != nil || r.Status != "unsupported" {
		t.Fatal("local case collision accepted", e)
	}
	for _, source := range []string{`SELECT $1,embedding <~> $1 FROM public.documents;`, `SELECT embedding <~> $1 FROM public.documents WHERE id=$1;`, `SELECT embedding <~> $1 FROM public.documents ORDER BY $1;`} {
		r, e = Convert(context.Background(), source, converterCatalog(t), localConvertOptions())
		if e != nil || r.Status != "unsupported" {
			t.Fatal("incompatible parameter accepted", e)
		}
	}
}
func FuzzConvertTypedSQL(f *testing.F) {
	b, err := os.ReadFile("../../testdata/pgembedding-0.3.6/catalog.json")
	if err != nil {
		f.Fatal(err)
	}
	c, err := DecodeCatalog(bytes.NewReader(b), Limits{})
	if err != nil {
		f.Fatal(err)
	}
	for _, source := range []string{converterSample, `SELECT embedding <~> ARRAY[1,2,3] FROM public.documents;`, `SELECT E'x\\'y' FROM public.documents;`, `SELECT id FROM public.documents;DROP TABLE x;`} {
		f.Add(source)
	}
	f.Fuzz(func(t *testing.T, source string) {
		if len(source) > 1<<16 {
			return
		}
		r, e := Convert(context.Background(), source, c, localConvertOptions())
		if e != nil {
			return
		}
		if r.SourceSHA256 != sqlHash(source) {
			t.Fatal("source identity")
		}
		if r.Status == "converted" {
			sql, e := r.SQL()
			if e != nil || r.TargetSHA256 != sqlHash(sql) {
				t.Fatal("output identity")
			}
			for _, st := range r.Statements {
				if _, e := sqlast.Parse(st.ConvertedSQL); e != nil {
					t.Fatal("unparsed local output")
				}
			}
		} else {
			if _, e := r.SQL(); e == nil {
				t.Fatal("partial script escaped")
			}
		}
	})
}

func TestConvertBitwiseBytesAndCommentedOptions(t *testing.T) {
	c := converterCatalog(t)
	source := `SELECT ~flags AS complement,flags | 4 AS union_bits,1 << 2 AS left_shift,8 >> 1 AS right_shift FROM public.documents WHERE flags & 3 = 1;`
	r := converted(t, c, source, localConvertOptions())
	sql, _ := r.SQL()
	for _, part := range []string{"~flags", "flags | 4", "1 << 2", "8 >> 1", "flags & 3"} {
		if !strings.Contains(sql, part) {
			t.Fatal("bitwise bytes changed", sql)
		}
	}
	source = `CREATE INDEX "quoted;index" ON public.documents USING hnsw(embedding /*ann_cos_ops*/ legacy.ann_manhattan_ops) WITH(dims=3,m=16,/*efconstruction=5, mystery=9*/efconstruction=64,efsearch=32);`
	r = converted(t, c, source, localConvertOptions())
	sql, _ = r.SQL()
	if !strings.Contains(sql, "opclass='vector_l1_ops'") || !strings.Contains(sql, "ef_construction=64") || !strings.Contains(sql, "/*efconstruction=5, mystery=9*/") || r.Statements[0].Settings[0].Value != 32 {
		t.Fatal(sql)
	}
	for _, source := range []string{`INSERT INTO public.documents(id,flags,embedding)VALUES(2147483648,1,ARRAY[1,2,3]);`, `INSERT INTO public.documents(id,flags,embedding)VALUES(-2147483649,1,ARRAY[1,2,3]);`, `SELECT flags << 40 FROM public.documents;`} {
		r, e := Convert(context.Background(), source, c, localConvertOptions())
		if e != nil || r.Status != "unsupported" {
			t.Fatal("source width accepted", source, e)
		}
	}
}

func TestConvertRejectsLocalCommentSemanticDrift(t *testing.T) {
	for _, source := range []string{
		"SELECT id FROM public.documents -- comment\r WHERE id=1;",
		"SELECT id FROM public.documents /* outer /* inner */ WHERE id=1 -- */;",
		"SELECT embedding <~> ARRAY[1, /*outer /*nested*/ tail*/ 2,3] FROM public.documents;",
	} {
		r, e := Convert(context.Background(), source, converterCatalog(t), localConvertOptions())
		if e != nil || r.Status != "unsupported" {
			t.Fatal("local comment semantics accepted", e, r.Status)
		}
		opts := localConvertOptions()
		opts.Target = TargetPostgres
		opts.VectorSchema = "public"
		opts.Tables[0].TargetSchema = "public"
		converted(t, converterCatalog(t), source, opts)
	}
	converted(t, converterCatalog(t), "SELECT id FROM public.documents -- CRLF\r\nWHERE id=1;", localConvertOptions())
}

func TestConvertStrictSourceBindsAndMetadata(t *testing.T) {
	c := converterCatalog(t)
	r := converted(t, c, `SELECT id FROM public.documents WHERE id=$1 LIMIT $2;`, localConvertOptions())
	for _, invalid := range []any{nil, int64(2147483648), float64(1.5), "2"} {
		parameter := r.Statements[0].Bindings[0]
		if invalid == nil {
			continue
		}
		if _, err := parameter.ConvertValue(invalid); err == nil {
			t.Fatal("integer source constraint accepted", invalid)
		}
	}
	if _, err := r.Statements[0].Bindings[1].ConvertValue(int64(-1)); err == nil {
		t.Fatal("negative limit bind accepted")
	}
	if !r.Statements[0].Bindings[1].RejectNull {
		t.Fatal("LIMIT lacks required non-NULL constraint")
	}
	if _, err := r.Statements[0].Bindings[1].ConvertValue(nil); err == nil {
		t.Fatal("NULL LIMIT bind accepted")
	}
	r = converted(t, c, `SELECT id,legacy.manhattan_distance(embedding,ARRAY[1,2,3]),embedding <~> ARRAY[1,2,3] FROM public.documents;`, localConvertOptions())
	sql, _ := r.SQL()
	if !strings.Contains(sql, `AS "manhattan_distance"`) || !strings.Contains(sql, `AS "?column?"`) {
		t.Fatal("output names changed", sql)
	}
	for _, target := range []SQLTarget{TargetLocal, TargetPostgres} {
		p := BindTransform{Parameter: 1, SourceKind: "real[]", Target: target, Dimensions: 3, Action: "real_array_to_vector"}
		for _, s := range []string{"{1e-999,2,3}", `{"-1e-999",2,3}`, `[1:3]={"\\1e-999",2,3}`} {
			if _, err := p.ConvertValue(s); err == nil {
				t.Fatal("nonzero source underflow accepted")
			}
		}
		for _, s := range []string{"{-0,0e-999,1.401298464324817e-45}", `{"-0","0e-999",1.401298464324817e-45}`} {
			if _, err := p.ConvertValue(s); err != nil {
				t.Fatal("valid zero/subnormal rejected", err)
			}
		}
		if _, err := p.ConvertValue([]float32{0, 0, 0}); err != nil {
			t.Fatal(err)
		}
	}
	for _, source := range []string{`SELECT 1e-999 FROM public.documents;`, `SELECT 0.1 FROM public.documents;`} {
		r, e := Convert(context.Background(), source, c, localConvertOptions())
		if e != nil || r.Status != "unsupported" {
			t.Fatal("arbitrary numeric conversion accepted", e)
		}
	}
}

func TestConvertScalarFloatAssignmentWidth(t *testing.T) {
	c := converterCatalog(t)
	c.Types = append(c.Types, Type{OID: 701, Schema: "pg_catalog", Name: "float8", Kind: "b"})
	c.Relations[0].Columns = append(c.Relations[0].Columns,
		Column{AttNum: 4, Name: "small_real", TypeOID: 700, TypeMod: -1},
		Column{AttNum: 5, Name: "wide_real", TypeOID: 701, TypeMod: -1})
	db, err := sqlite.Open(filepath.Join(t.TempDir(), "scalar-width.db"), sqlite.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	source := `CREATE TABLE public.documents(id INTEGER PRIMARY KEY,flags INTEGER NOT NULL,embedding REAL[],small_real REAL,wide_real DOUBLE PRECISION);
INSERT INTO public.documents(id,flags,small_real,wide_real) VALUES(1,1,16777217,9007199254740993),(2,1,-16777217,0.1);
SELECT id,small_real,wide_real FROM public.documents ORDER BY id;`
	r := converted(t, c, source, localConvertOptions())
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
	for i, want := range [][2]float64{{16777216, 9007199254740992}, {-16777216, 0.1}} {
		if !rows.Next() {
			t.Fatal("missing scalar row", rows.Err())
		}
		values := rows.Values()
		if values[0] != int64(i+1) || values[1] != want[0] || values[2] != want[1] {
			t.Fatalf("scalar rounding: %v, want %v", values, want)
		}
	}
	if rows.Next() || rows.Err() != nil {
		t.Fatal("extra/error scalar row", rows.Err())
	}
	for _, source := range []string{
		`UPDATE public.documents SET small_real=wide_real;`,
		`UPDATE public.documents SET small_real=flags;`,
		`SELECT id FROM public.documents WHERE small_real=16777217;`,
		`SELECT id FROM public.documents WHERE small_real=0.1;`,
		`SELECT id FROM public.documents WHERE small_real=wide_real;`,
	} {
		r, err := Convert(context.Background(), source, c, localConvertOptions())
		if err != nil || r.Status != "unsupported" {
			t.Fatal("unverified numeric coercion accepted", source, err)
		}
	}
}

func TestConvertRejectsUnadaptedBitParameter(t *testing.T) {
	c := converterCatalog(t)
	c.Types = append(c.Types, Type{OID: 1560, Schema: "pg_catalog", Name: "bit", Kind: "b"})
	c.Relations[0].Columns = append(c.Relations[0].Columns, Column{AttNum: 4, Name: "bits", TypeOID: 1560, TypeMod: 3})
	for _, target := range []SQLTarget{TargetLocal, TargetPostgres} {
		opts := localConvertOptions()
		opts.Target = target
		if target == TargetPostgres {
			opts.VectorSchema, opts.Tables[0].TargetSchema = "public", "public"
		}
		r, err := Convert(context.Background(), `UPDATE public.documents SET bits=$1;`, c, opts)
		if err != nil || r.Status != "unsupported" || len(r.Statements[0].Bindings) != 0 {
			t.Fatal("unadapted bit parameter accepted", target, err)
		}
	}
}
