package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// All tests in this file are offline catalog/text tests. They neither open a
// database nor execute generated SQL, subprocesses or filesystem fault paths.
func transitionFixtureOptions() TransitionOptions {
	return TransitionOptions{RelationOID: 30000, AttNum: 3, Dimensions: 3, Approach: TypedCopyShadow, Mode: TransitionBatches,
		VectorSchema: "public", TargetVersion: "0.8.7", ShadowColumn: "embedding_vector", StateSchema: "public", StateTable: "documents_vector_migration", BatchSize: 2,
		Indexes: []TransitionIndex{{SourceIndexOID: 30100, Name: "documents_vector_l2", Method: "hnsw", M: 16, EFConstruction: 64, EFSearch: 64}, {SourceIndexOID: 30101, Name: "documents_vector_cosine", Method: "hnsw", M: 16, EFConstruction: 64, EFSearch: 64}, {SourceIndexOID: 30102, Name: "documents_vector_l1", Method: "hnsw", M: 16, EFConstruction: 64, EFSearch: 64}}}
}
func transitionFile(t *testing.T, b TransitionBundle, name string) string {
	t.Helper()
	for _, a := range b.Artifacts {
		if a.Name == name {
			return a.Content
		}
	}
	t.Fatalf("missing artifact %s", name)
	return ""
}
func transitionGate(b TransitionBundle, code string) bool {
	for _, g := range b.Report.Gates {
		if g.Code == code {
			return true
		}
	}
	return false
}
func transitionGenerate(t *testing.T, c CatalogSnapshot, o TransitionOptions) TransitionBundle {
	t.Helper()
	b, e := GenerateTransition(context.Background(), c, o)
	if e != nil {
		t.Fatal(e)
	}
	return b
}

func TestTransitionModesAndCommentBoundary(t *testing.T) {
	for _, approach := range []TransitionApproach{RetainArrayCast, TypedCopyShadow} {
		for _, mode := range []string{TransitionSingle, TransitionBatches} {
			t.Run(string(approach)+"/"+mode, func(t *testing.T) {
				o := transitionFixtureOptions()
				o.Approach = approach
				o.Mode = mode
				if approach == RetainArrayCast {
					o.ShadowColumn = ""
				}
				if approach == RetainArrayCast || mode == TransitionSingle {
					o.StateSchema = ""
					o.StateTable = ""
					o.BatchSize = 0
				}
				b := transitionGenerate(t, fixtureCatalog(t), o)
				if b.Report.Status != "review_required" || len(b.Artifacts) != 13 {
					t.Fatalf("status/artifacts: %s %d", b.Report.Status, len(b.Artifacts))
				}
				for _, g := range b.Report.Gates {
					if g.State != "unresolved" {
						t.Fatalf("gate unexpectedly satisfied/blocked: %+v", g)
					}
				}
				for _, a := range b.Artifacts {
					if sqlHash(a.Content) != a.SHA256 {
						t.Fatal("bad artifact hash")
					}
					if a.Kind == "review_only_sql" {
						_, segments, e := pgScan(a.Content, DefaultSQLLimits())
						if e != nil || len(segments) != 0 {
							t.Fatalf("active statement escaped comment boundary %s: %v", a.Name, e)
						}
					}
				}
				preflight := transitionFile(t, b, "00-preflight.sql")
				tokens, _, e := pgScan(preflight, DefaultSQLLimits())
				if e != nil {
					t.Fatal(e)
				}
				for _, tok := range tokens {
					if tok.Kind == pgIdentifier && !tok.Quoted {
						switch strings.ToUpper(tok.Text) {
						case "DROP", "ALTER", "CREATE", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "CALL", "DO", "COPY":
							t.Fatalf("mutating token in preflight: %s", tok.Text)
						}
					}
				}
				for _, required := range []string{"READ ONLY", "pg_available_extension_versions", "pg_am", "pg_depend", "WITH RECURSIVE", "bad_shape_rows", "null_element_rows", "nonfinite_rows", "zero_norm_rows", "has_table_privilege", "pg_policy"} {
					if !strings.Contains(preflight, required) {
						t.Error(required)
					}
				}
				for _, a := range b.Artifacts {
					if a.Kind == "review_only_sql" && (strings.Contains(a.Content, " CASCADE;") || strings.Contains(a.Content, "DROP COLUMN") || strings.Contains(a.Content, "DROP TABLE")) {
						t.Fatal("destructive cleanup default")
					}
				}
			})
		}
	}
}

func TestTransitionDeterminismAndGoldenPhases(t *testing.T) {
	for _, kind := range []string{"shadow-batches", "retain-single"} {
		t.Run(kind, func(t *testing.T) {
			o := transitionFixtureOptions()
			if kind == "retain-single" {
				o.Approach = RetainArrayCast
				o.Mode = TransitionSingle
				o.ShadowColumn = ""
				o.StateSchema = ""
				o.StateTable = ""
				o.BatchSize = 0
			}
			c := fixtureCatalog(t)
			a := transitionGenerate(t, c, o)
			b := transitionGenerate(t, c, o)
			var first, second bytes.Buffer
			if e := RenderTransitionJSON(&first, a); e != nil {
				t.Fatal(e)
			}
			if e := RenderTransitionJSON(&second, b); e != nil {
				t.Fatal(e)
			}
			if !bytes.Equal(first.Bytes(), second.Bytes()) {
				t.Fatal("nondeterministic bundle")
			}
			for _, f := range a.Artifacts {
				if !strings.HasSuffix(f.Name, ".sql") {
					continue
				}
				want, e := os.ReadFile(filepath.Join("../../examples/migration-transition/golden", kind, f.Name))
				if e != nil {
					t.Fatal(e)
				}
				if string(want) != f.Content {
					t.Errorf("golden mismatch %s", f.Name)
				}
			}
			// Reordering explicit index mappings has no semantic effect or output drift.
			o.Indexes[0], o.Indexes[2] = o.Indexes[2], o.Indexes[0]
			reordered := transitionGenerate(t, c, o)
			if reordered.Report.PlanSHA256 != a.Report.PlanSHA256 {
				t.Fatal("index-option ordering affected identity")
			}
		})
	}
}

func TestTransitionBatchAtomicCursorAndNullRows(t *testing.T) {
	b := transitionGenerate(t, fixtureCatalog(t), transitionFixtureOptions())
	sql := transitionFile(t, b, "30-batch.sql")
	for _, want := range []string{"prior AS MATERIALIZED", "FOR UPDATE", "p.last_id IS NULL OR src.\"id\" > p.last_id", "ORDER BY src.\"id\" LIMIT 2", "changed AS", "rows_done=state.rows_done+progress.n", "IS NOT DISTINCT FROM prior.last_id", "state.batches=prior.batches", "progress.n=0 AND state.rows_done=state.expected_rows", "RETURNING state.plan_sha256", "COMMIT;"} {
		if !strings.Contains(sql, want) {
			t.Error(want)
		}
	}
	for _, bad := range []string{"-9223372036854775808", "SKIP LOCKED", "OFFSET ", "embedding_vector\" IS NULL"} {
		if strings.Contains(sql, bad) {
			t.Errorf("unsafe cursor pattern: %s", bad)
		}
	}
	setup := transitionFile(t, b, "20-shadow-copy.sql")
	if !strings.Contains(setup, ",NULL,0,4,0,false)") || !strings.Contains(setup, "rows_done<=expected_rows") || !strings.Contains(setup, b.Report.PlanSHA256) {
		t.Fatal("state is not initialized/plan-bound")
	}
	verify := transitionFile(t, b, "50-verify.sql")
	for _, s := range []string{"float4send", "FULL JOIN", "float32_bit_mismatches", "null_mismatches", "dimension_mismatches", "norm_mismatches", "0.000001", "rows_done=expected_rows"} {
		if !strings.Contains(verify, s) {
			t.Error(s)
		}
	}
}

func TestTransitionMetricMethodsAndTransactionBoundaries(t *testing.T) {
	o := transitionFixtureOptions()
	o.Indexes[0] = TransitionIndex{SourceIndexOID: 30100, Name: "documents_vector_l2", Method: "ivfflat", Lists: 2, Probes: 1}
	o.ConcurrentIndexes = true
	b := transitionGenerate(t, fixtureCatalog(t), o)
	indexes := transitionFile(t, b, "40-indexes.sql")
	if b.Report.Status != "review_required" || !strings.Contains(indexes, "CREATE INDEX CONCURRENTLY") || !strings.Contains(indexes, "USING ivfflat (\"embedding_vector\" \"public\".\"vector_l2_ops\") WITH (lists=2)") {
		t.Fatal(indexes)
	}
	if strings.Contains(indexes, "BEGIN;") || strings.Contains(indexes, "COMMIT;") {
		t.Fatal("concurrent build wrapped in transaction")
	}
	app := transitionFile(t, b, "60-application-cutover.sql")
	for _, s := range []string{"ivfflat.probes = 1", "hnsw.ef_search = 64", "OPERATOR(\"public\".<+>)", "$1::\"public\".\"vector\"(3)", "ASC NULLS LAST"} {
		if !strings.Contains(app, s) {
			t.Error(s)
		}
	}
	o.Indexes[2] = TransitionIndex{SourceIndexOID: 30102, Name: "documents_vector_l1", Method: "ivfflat", Lists: 2, Probes: 1}
	bad := transitionGenerate(t, fixtureCatalog(t), o)
	if !transitionGate(bad, "ivfflat_l1_unsupported") {
		t.Fatal("IVF L1 accepted")
	}
}

func TestTransitionBlockedCatalogFacts(t *testing.T) {
	tests := []struct {
		name, gate string
		change     func(*CatalogSnapshot, *TransitionOptions)
	}{
		{"profile", "source_column_not_eligible", func(c *CatalogSnapshot, o *TransitionOptions) { c.Profiles = []DataProfile{} }},
		{"coverage", "dependency_inventory_incomplete", func(c *CatalogSnapshot, o *TransitionOptions) { c.Scope.Coverage[0].Complete = false }},
		{"unknown_target", "target_version_unknown", func(c *CatalogSnapshot, o *TransitionOptions) { o.TargetVersion = "0.9.0" }},
		{"missing_target", "target_version_unknown", func(c *CatalogSnapshot, o *TransitionOptions) { o.TargetVersion = "" }},
		{"unmapped_index", "unplanned_legacy_index", func(c *CatalogSnapshot, o *TransitionOptions) { o.Indexes = o.Indexes[:2] }},
		{"old_defaults", "source_inspection_blocked", func(c *CatalogSnapshot, o *TransitionOptions) { c.Indexes[0].Options = []string{"dims=3"} }},
		{"new_dependency", "dependent_object_requires_manual_plan", func(c *CatalogSnapshot, o *TransitionOptions) {
			c.Dependencies = append(c.Dependencies, Dependency{Class: "pg_rewrite", OID: 45001, Identity: "dependent view", ReferencedClass: "pg_class", ReferencedOID: 30000, Kind: "view", Definition: "SELECT untrusted()"})
		}},
		{"forged_member", "dependent_object_requires_manual_plan", func(c *CatalogSnapshot, o *TransitionOptions) {
			c.Dependencies = append(c.Dependencies, Dependency{Class: "pg_rewrite", OID: 45001, Identity: "dependent view", ReferencedClass: "pg_class", ReferencedOID: 30000, Kind: "extension_member"})
		}},
		{"shadow_collision", "shadow_name_collision", func(c *CatalogSnapshot, o *TransitionOptions) { o.ShadowColumn = "embedding" }},
		{"target_index_collision", "index_name_collision", func(c *CatalogSnapshot, o *TransitionOptions) { o.Indexes[0].Name = c.Indexes[0].Name }},
		{"state_collision", "state_name_collision", func(c *CatalogSnapshot, o *TransitionOptions) { o.StateTable = "documents" }},
		{"source_owner", "source_inspection_blocked", func(c *CatalogSnapshot, o *TransitionOptions) { c.OpClasses[0].ExtensionOID = 0 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := fixtureCatalog(t)
			o := transitionFixtureOptions()
			tt.change(&c, &o)
			b, e := GenerateTransition(context.Background(), c, o)
			if tt.name == "state_collision" {
				if e == nil {
					t.Fatal("same-table state accepted")
				}
				return
			}
			if e != nil {
				t.Fatal(e)
			}
			if b.Report.Status != "blocked" || !transitionGate(b, tt.gate) {
				t.Fatalf("missing gate %s: %+v", tt.gate, b.Report.Gates)
			}
			for _, a := range b.Artifacts {
				if a.Kind == "review_only_sql" && a.Name != "70-rollback.sql" && !strings.Contains(a.Content, "statements withheld") {
					t.Fatal("blocked plan has proposed executable body")
				}
			}
		})
	}
}

func TestTransitionSchemaAndDependencyDrift(t *testing.T) {
	c := fixtureCatalog(t)
	o := transitionFixtureOptions()
	base := transitionGenerate(t, c, o)
	c.Indexes[0].Definition += " changed definition"
	changed := transitionGenerate(t, c, o)
	if base.Report.PlanSHA256 == changed.Report.PlanSHA256 || base.Report.CatalogSHA256 == changed.Report.CatalogSHA256 {
		t.Fatal("definition drift identity lost")
	}
	c.Dependencies = append(c.Dependencies, Dependency{Class: "pg_rewrite", OID: 45001, Identity: "review me", ReferencedClass: "pg_class", ReferencedOID: 30000, Kind: "view"})
	dependent := transitionGenerate(t, c, o)
	if changed.Report.DependencySHA256 == dependent.Report.DependencySHA256 || dependent.Report.Status != "blocked" {
		t.Fatal("dependency drift identity lost")
	}
}

func TestTransitionQuotedIdentifiersAndEvidenceIsolation(t *testing.T) {
	c := fixtureCatalog(t)
	o := transitionFixtureOptions()
	name := "name\"; SELECT secret; --\\路径"
	c.Relations[0].Name = name
	o.VectorSchema = name
	o.ShadowColumn = "copy\"; SELECT hidden; --"
	c.Indexes[0].Definition = "SELECT payload();\rDELETE FROM elsewhere;\nCOPY secrets TO PROGRAM 'false';"
	b := transitionGenerate(t, c, o)
	pre := transitionFile(t, b, "00-preflight.sql")
	tokens, _, e := pgScan(pre, DefaultSQLLimits())
	if e != nil {
		t.Fatal(e)
	}
	found := false
	for _, tok := range tokens {
		if tok.Kind == pgIdentifier && tok.Text == name && tok.Quoted {
			found = true
		}
		if tok.Kind == pgIdentifier && !tok.Quoted && (tok.Text == "secret" || tok.Text == "hidden") {
			t.Fatal("identifier escaped quoting")
		}
	}
	if !found {
		t.Fatal("quoted relation identifier not preserved")
	}
	for _, a := range b.Artifacts {
		if strings.HasSuffix(a.Name, ".sql") && (strings.Contains(a.Content, "payload()") || strings.Contains(a.Content, "COPY secrets")) {
			t.Fatal("raw evidence copied to SQL")
		}
	}
	var inventory map[string]any
	if e = json.Unmarshal([]byte(transitionFile(t, b, "inventory.json")), &inventory); e != nil {
		t.Fatal(e)
	}
	for _, s := range []string{"line\nidentifier", "line\ridentifier", "tab\tidentifier", "x\x00", "x\u2028"} {
		o.VectorSchema = s
		if _, e := GenerateTransition(context.Background(), c, o); e == nil {
			t.Errorf("control-bearing identifier accepted: %q", s)
		}
	}
	if _, segments, e := pgScan(reviewSQL("SELECT 1;\rDELETE FROM t;\r\nDROP TABLE x;\n"), DefaultSQLLimits()); e != nil || len(segments) != 0 {
		t.Fatal("CR escaped review-only comments")
	}
}

func TestTransitionLimitsCancellationAndBadOptions(t *testing.T) {
	c := fixtureCatalog(t)
	base := transitionFixtureOptions()
	for _, change := range []func(*TransitionOptions){func(o *TransitionOptions) { o.MaxBytes = 10 }, func(o *TransitionOptions) { o.MaxBytes = MaxTransitionBytes + 1 }, func(o *TransitionOptions) { o.BatchSize = 0 }, func(o *TransitionOptions) { o.Indexes[0].M = 100; o.Indexes[0].EFConstruction = 16 }, func(o *TransitionOptions) { o.Indexes[0].M = 3; o.Indexes[0].EFConstruction = 5 }, func(o *TransitionOptions) { o.Mode = TransitionSingle; o.ConcurrentIndexes = true }, func(o *TransitionOptions) { o.Approach = "typed_copy_full_table" }, func(o *TransitionOptions) { o.Indexes[1].SourceIndexOID = o.Indexes[0].SourceIndexOID }, func(o *TransitionOptions) { o.Indexes[1].Name = o.Indexes[0].Name }} {
		o := base
		o.Indexes = append([]TransitionIndex{}, base.Indexes...)
		change(&o)
		if _, e := GenerateTransition(context.Background(), c, o); e == nil {
			t.Fatal("invalid option accepted")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, e := GenerateTransition(ctx, c, base); e != context.Canceled {
		t.Fatal(e)
	}
	if _, e := GenerateTransition(nil, c, base); e == nil {
		t.Fatal("nil context")
	}
	b := transitionGenerate(t, c, base)
	b.Artifacts[0].Content += "altered"
	if e := RenderTransitionJSON(&bytes.Buffer{}, b); e == nil {
		t.Fatal("altered artifact rendered")
	}
}
