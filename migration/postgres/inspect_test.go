package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func fixtureBytes(t *testing.T) []byte {
	t.Helper()
	b, e := os.ReadFile("../../testdata/pgembedding-0.3.6/catalog.json")
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func fixtureCatalog(t *testing.T) CatalogSnapshot {
	t.Helper()
	c, e := DecodeCatalog(bytes.NewReader(fixtureBytes(t)), Limits{})
	if e != nil {
		t.Fatal(e)
	}
	return c
}
func hasCode(r Report, code string) bool {
	for _, f := range r.Findings {
		if f.Code == code {
			return true
		}
	}
	return false
}
func TestInspectCompleteFixture(t *testing.T) {
	c := fixtureCatalog(t)
	r, e := Inspect(context.Background(), c, InspectOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if len(r.Columns) != 2 || len(r.Indexes) != 3 || r.Status != "review_required" {
		t.Fatalf("unexpected report: %+v", r)
	}
	for _, i := range r.Indexes {
		if !i.IdentityVerified || i.Status != "safe" || i.ProposedTarget == nil || i.QueryEFSearch != 64 {
			t.Fatalf("index: %+v", i)
		}
	}
	if r.Columns[0].Classification != "legacy_vector" || !r.Columns[0].Selected || r.Columns[0].Dimensions != 3 || r.Columns[1].Selected {
		t.Fatalf("columns: %+v", r.Columns)
	}
	for _, code := range []string{"hnsw_access_method_collision", "current_neon_chapter_absent", "numeric_semantics_difference", "application_sql_inventory_incomplete", "efsearch_scope_change"} {
		if !hasCode(r, code) {
			t.Error(code)
		}
	}
	var a, b bytes.Buffer
	RenderJSON(&a, r)
	r2, e := Inspect(context.Background(), c, InspectOptions{})
	if e != nil {
		t.Fatal(e)
	}
	RenderJSON(&b, r2)
	if a.String() != b.String() {
		t.Fatal("nondeterministic report")
	}
}
func TestInspectFindings(t *testing.T) {
	tests := []struct {
		name, code string
		change     func(*CatalogSnapshot)
	}{
		{"default100_16", "target_option_constraint", func(c *CatalogSnapshot) { c.Indexes[0].Options = []string{"dims=3"} }},
		{"example3_5", "target_option_constraint", func(c *CatalogSnapshot) { c.Indexes[0].Options = []string{"dims=3", "m=3", "efconstruction=5"} }},
		{"unknown_option", "unknown_index_option", func(c *CatalogSnapshot) { c.Indexes[0].Options = append(c.Indexes[0].Options, "surprise=1") }},
		{"duplicate_option", "duplicate_index_option", func(c *CatalogSnapshot) { c.Indexes[0].Options = append(c.Indexes[0].Options, "m=16") }},
		{"invalid_option", "invalid_index_option", func(c *CatalogSnapshot) { c.Indexes[0].Options = []string{"dims=3", "m=16;DROP"} }},
		{"extension_version", "source_version_unsupported", func(c *CatalogSnapshot) { c.Extensions[0].Version = "0.3.5" }},
		{"fake_owner", "legacy_index_identity_mismatch", func(c *CatalogSnapshot) { c.OpClasses[0].ExtensionOID = 0 }},
		{"fake_operator", "legacy_index_identity_mismatch", func(c *CatalogSnapshot) { c.Operators[0].Name = "<^>" }},
		{"fake_signature", "legacy_index_identity_mismatch", func(c *CatalogSnapshot) { c.Functions[1].ReturnOID = 20 }},
		{"shape", "array_shape_mismatch", func(c *CatalogSnapshot) { c.Profiles[0].Shapes[0].Dimensions = []int{2} }},
		{"bounds", "nondefault_array_bounds", func(c *CatalogSnapshot) { c.Profiles[0].Shapes[0].LowerBounds = []int{0} }},
		{"null_element", "null_element", func(c *CatalogSnapshot) { c.Profiles[0].NullElementRows = 1 }},
		{"zero_norm", "cosine_zero_norm", func(c *CatalogSnapshot) { c.Profiles[0].ZeroNormRows = 1 }},
		{"dimension_conflict", "conflicting_dimensions", func(c *CatalogSnapshot) { c.Indexes[1].Options[0] = "dims=4" }},
		{"efsearch_conflict", "efsearch_session_scope_conflict", func(c *CatalogSnapshot) { c.Indexes[1].Options[3] = "efsearch=32" }},
		{"missing_profile", "data_profile_missing", func(c *CatalogSnapshot) { c.Profiles = []DataProfile{} }},
		{"incomplete", "incomplete_catalog_coverage", func(c *CatalogSnapshot) { c.Scope.Coverage = []Coverage{} }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fixtureCatalog(t)
			tt.change(&c)
			r, e := Inspect(context.Background(), c, InspectOptions{})
			if e != nil {
				t.Fatal(e)
			}
			if !hasCode(r, tt.code) {
				t.Fatalf("missing %s in %+v", tt.code, r.Findings)
			}
		})
	}
}
func TestCatalogStrictJSON(t *testing.T) {
	b := fixtureBytes(t)
	bad := []string{strings.Replace(string(b), `"format_version": 1`, `"format_version": 1, "format_version": 1`, 1), strings.Replace(string(b), `"attnum": 1`, `"attnum": 1, "attnum": 1`, 1), strings.Replace(string(b), `"not_null": true,`, "", 1), strings.Replace(string(b), `"format_version": 1`, `"Format_Version": 1`, 1), strings.Replace(string(b), `"server_version_num": 180000`, `"server_version_num": null`, 1), strings.Replace(string(b), `"format_version": 1`, `"format_version": 2`, 1), string(b) + "{}", `{"unexpected":1}`, strings.Replace(string(b), `"oid": 20`, `"oid": 20.5`, 1), strings.Replace(string(b), `"sql_corpus": [`, `"sql_corpus": null, "other": [`, 1)}
	for i, s := range bad {
		if _, e := DecodeCatalog(strings.NewReader(s), Limits{}); e == nil {
			t.Errorf("case %d accepted", i)
		}
	}
	l := DefaultLimits()
	l.MaxBytes = 20
	if _, e := DecodeCatalog(bytes.NewReader(b), l); e == nil {
		t.Error("bytes limit")
	}
	l = DefaultLimits()
	l.MaxDepth = 2
	if _, e := DecodeCatalog(bytes.NewReader(b), l); e == nil {
		t.Error("depth limit")
	}
	l = DefaultLimits()
	l.MaxObjects = 1
	if _, e := DecodeCatalog(bytes.NewReader(b), l); e == nil {
		t.Error("object limit")
	}
}
func TestExplicitSelectionAndCancellation(t *testing.T) {
	c := fixtureCatalog(t)
	r, e := Inspect(context.Background(), c, InspectOptions{Selected: []Selection{{30000, 4, 3}}})
	if e != nil {
		t.Fatal(e)
	}
	if !r.Columns[1].Selected || r.Columns[1].Classification != "explicitly_selected_real_array" {
		t.Fatal(r.Columns)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e = Inspect(ctx, c, InspectOptions{}); e != context.Canceled {
		t.Fatal(e)
	}
}
func TestMarkdownEscapesCatalogText(t *testing.T) {
	r := Report{Findings: []Finding{{"warning", "test", "<script>\n[x](url)", "*"}}}
	var b bytes.Buffer
	if e := RenderMarkdown(&b, r); e != nil {
		t.Fatal(e)
	}
	if strings.Contains(b.String(), "<script>") || strings.Contains(b.String(), "[x](url)") {
		t.Fatal(b.String())
	}
}
func FuzzCatalogJSON(f *testing.F) {
	b, _ := os.ReadFile("../../testdata/pgembedding-0.3.6/catalog.json")
	f.Add(b)
	f.Add([]byte(`{"a":1,"a":2}`))
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 1<<20 {
			t.Skip()
		}
		c, e := DecodeCatalog(bytes.NewReader(b), Limits{})
		if e == nil {
			if _, e = Inspect(context.Background(), c, InspectOptions{}); e != nil {
				t.Fatal(e)
			}
			if _, e = json.Marshal(c); e != nil {
				t.Fatal(e)
			}
		}
	})
}

func TestInspectHashTracksMutationAndBudgets(t *testing.T) {
	c := fixtureCatalog(t)
	before, e := Inspect(context.Background(), c, InspectOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if before.SourceBytesSHA256 == "" || before.CatalogHashKind != "canonical_json_v1" {
		t.Fatal("missing source/hash kind")
	}
	c.Indexes[0].Options[1] = "m=15"
	after, e := Inspect(context.Background(), c, InspectOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if after.CatalogSHA256 == before.CatalogSHA256 || after.SourceBytesSHA256 != "" {
		t.Fatal("stale provenance after mutation")
	}
	c = fixtureCatalog(t)
	c.Scope.Coverage[0].Reason = strings.Repeat("x", (1<<20)+1)
	if _, e = Inspect(context.Background(), c, InspectOptions{}); e == nil {
		t.Fatal("programmatic long string")
	}
	c = fixtureCatalog(t)
	l := DefaultLimits()
	l.MaxBytes = 200
	if _, e = Inspect(context.Background(), c, InspectOptions{Limits: l}); e == nil {
		t.Fatal("programmatic total budget")
	}
	c = fixtureCatalog(t)
	c.Profiles[0].Shapes[0].Dimensions = make([]int, 200)
	l = DefaultLimits()
	l.MaxTokens = 100
	if _, e = Inspect(context.Background(), c, InspectOptions{Limits: l}); e == nil {
		t.Fatal("programmatic work budget")
	}
}

func TestStrictJSONRejectsUnpairedSurrogates(t *testing.T) {
	for _, s := range []string{`"\ud800"`, `"\udc00"`, `"\ud800x"`, `"\ud800\u1234"`} {
		var text string
		if e := strictJSON([]byte(s), &text, DefaultLimits()); e == nil {
			t.Fatal("accepted surrogate", s)
		}
	}
	for _, s := range []string{`"\ud83d\ude00"`, `"\\ud800"`, `"plain"`, `"\""`} {
		var text string
		if e := strictJSON([]byte(s), &text, DefaultLimits()); e != nil {
			t.Fatal(s, e)
		}
	}
}

func TestInspectionGoldenReports(t *testing.T) {
	c := fixtureCatalog(t)
	r, e := Inspect(context.Background(), c, InspectOptions{})
	if e != nil {
		t.Fatal(e)
	}
	for _, tt := range []struct {
		name   string
		render func(*bytes.Buffer, Report) error
	}{
		{"report.golden.json", func(b *bytes.Buffer, r Report) error { return RenderJSON(b, r) }},
		{"report.golden.md", func(b *bytes.Buffer, r Report) error { return RenderMarkdown(b, r) }},
	} {
		var got bytes.Buffer
		if e = tt.render(&got, r); e != nil {
			t.Fatal(e)
		}
		want, e := os.ReadFile("../../testdata/pgembedding-0.3.6/" + tt.name)
		if e != nil {
			t.Fatal(e)
		}
		if !bytes.Equal(got.Bytes(), want) {
			t.Fatalf("%s differs from checked-in deterministic report", tt.name)
		}
	}
}

func TestIdentityPrimaryKeyRequiresReview(t *testing.T) {
	c := fixtureCatalog(t)
	c.Relations[0].Columns[0].Identity = "a"
	r, e := Inspect(context.Background(), c, InspectOptions{})
	if e != nil {
		t.Fatal(e)
	}
	if !hasCode(r, "primary_key_generation_requires_review") {
		t.Fatal(r.Findings)
	}
	for _, i := range r.Indexes {
		if i.ProposedTarget != nil {
			t.Fatal("review-required column proposed target")
		}
	}
}
