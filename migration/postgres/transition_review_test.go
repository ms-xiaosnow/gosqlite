package postgres

import (
	"context"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Independent review tests only generate/inspect in-memory SQL text and read
// ordinary golden fixtures. They never open a database or execute SQL, start
// subprocesses, change namespaces, or exercise interrupted/fault workflows.
func TestTransitionReviewCorrelatedColumnIdentity(t *testing.T) {
	for _, approach := range []TransitionApproach{RetainArrayCast, TypedCopyShadow} {
		for _, name := range []string{"x", "n", "a", "b", `value"quoted`} {
			t.Run(string(approach)+"/"+name, func(t *testing.T) {
				c, o := fixtureCatalog(t), transitionFixtureOptions()
				o.Approach = approach
				if approach == RetainArrayCast {
					c.Relations[0].Columns[2].Name = name
					o.ShadowColumn, o.StateSchema, o.StateTable, o.BatchSize = "", "", "", 0
				} else {
					o.ShadowColumn = name
				}
				b := transitionGenerate(t, c, o)
				if b.Report.Status != "review_required" {
					t.Fatal("ordinary quoted identifiers unexpectedly blocked")
				}
				sql := transitionFile(t, b, "50-verify.sql")
				start, end := strings.Index(sql, "SELECT pg_catalog.count(*) AS rows"), strings.Index(sql, "-- -- Norm tolerance")
				if start < 0 || end <= start {
					t.Fatal("missing exhaustive value-comparison SELECT")
				}
				check := sql[start:end]
				expr := "src." + quoteIdent(name)
				if approach == RetainArrayCast {
					expr = "(" + expr + `::"public"."vector"(3))`
				}
				// The second table function can see a(x,n) via implicit LATERAL.
				// All external arguments must therefore use the outer table alias.
				for _, want := range []string{
					`FROM "public"."documents" AS src;`,
					"pg_catalog.unnest(src." + quoteIdent(c.Relations[0].Columns[2].Name) + ") WITH ORDINALITY AS a(x,n)",
					"FULL JOIN pg_catalog.unnest((" + expr + ")::pg_catalog.float4[])",
				} {
					if !strings.Contains(check, want) {
						t.Errorf("outer identity is not explicitly qualified: %s", want)
					}
				}
			})
		}
	}
}

func TestTransitionReviewInheritanceParentBlocked(t *testing.T) {
	for _, approach := range []TransitionApproach{RetainArrayCast, TypedCopyShadow} {
		c, o := fixtureCatalog(t), transitionFixtureOptions()
		o.Approach = approach
		if approach == RetainArrayCast {
			o.ShadowColumn, o.StateSchema, o.StateTable, o.BatchSize = "", "", "", 0
		}
		child := c.Relations[0]
		child.OID, child.Name, child.Parents = 30002, "inherited_documents", []uint32{o.RelationOID}
		child.Columns = append([]Column(nil), child.Columns...)
		c.Relations = append(c.Relations, child)
		b := transitionGenerate(t, c, o)
		if b.Report.Status != "blocked" || !transitionGate(b, "source_inheritance_unsupported") {
			t.Fatalf("%s accepted an inheritance parent: %s", approach, b.Report.Status)
		}
		for _, a := range b.Artifacts {
			if a.Kind == "review_only_sql" && a.Name != "70-rollback.sql" && !strings.Contains(a.Content, "statements withheld") {
				t.Errorf("inheritance blocker emitted proposed phase %s", a.Name)
			}
		}
	}
}

func TestTransitionReviewCompleteGoldenManifest(t *testing.T) {
	for _, kind := range []string{"retain-single", "shadow-batches"} {
		t.Run(kind, func(t *testing.T) {
			o := transitionFixtureOptions()
			if kind == "retain-single" {
				o.Approach, o.Mode = RetainArrayCast, TransitionSingle
				o.ShadowColumn, o.StateSchema, o.StateTable, o.BatchSize = "", "", "", 0
			}
			b := transitionGenerate(t, fixtureCatalog(t), o)
			files := map[string]TransitionArtifact{}
			for _, a := range b.Artifacts {
				if _, duplicate := files[a.Name]; duplicate {
					t.Fatal("duplicate artifact")
				}
				files[a.Name] = a
				raw, err := os.ReadFile(filepath.Join("../../examples/migration-transition/golden", kind, a.Name))
				if err != nil {
					t.Fatal(err)
				}
				if string(raw) != a.Content || sqlHash(string(raw)) != a.SHA256 {
					t.Errorf("golden bytes/hash differ: %s", a.Name)
				}
			}
			var manifest struct {
				PlanSHA256 string `json:"plan_sha256"`
				Files      []struct {
					Name, Kind, Transaction, SHA256 string
					Bytes                           int
				} `json:"files"`
			}
			if err := json.Unmarshal([]byte(files["manifest.json"].Content), &manifest); err != nil {
				t.Fatal(err)
			}
			if len(manifest.Files) != len(files)-1 || manifest.PlanSHA256 != b.Report.PlanSHA256 {
				t.Fatal("manifest membership/identity mismatch")
			}
			seen := map[string]bool{}
			for _, f := range manifest.Files {
				a, ok := files[f.Name]
				if !ok || seen[f.Name] || f.Name == "manifest.json" || f.Bytes != len(a.Content) || f.SHA256 != a.SHA256 || f.Kind != a.Kind || f.Transaction != a.Transaction {
					t.Errorf("manifest entry inconsistent: %s", f.Name)
				}
				seen[f.Name] = true
			}
		})
	}
}

func TestTransitionReviewBatchStateAndRollbackContracts(t *testing.T) {
	c, o := fixtureCatalog(t), transitionFixtureOptions()
	b := transitionGenerate(t, c, o)
	batch := transitionFile(t, b, "30-batch.sql")
	for _, want := range []string{"plan_sha256=" + transitionLiteral(b.Report.PlanSHA256), "AND NOT complete FOR UPDATE", "p.last_id IS NULL OR src.\"id\" > p.last_id", "changed AS (", "RETURNING dst.\"id\" AS id", "FROM changed", "IS NOT DISTINCT FROM prior.last_id", "state.rows_done=prior.rows_done", "state.batches=prior.batches", "progress.n AS batch_rows", "Application writes and PK changes MUST remain paused"} {
		if !strings.Contains(batch, want) {
			t.Errorf("batch contract absent: %s", want)
		}
	}
	for _, forbidden := range []string{"SKIP LOCKED", "OFFSET ", "COALESCE", "-9223372036854775808", `"embedding_vector" IS NULL`} {
		if strings.Contains(batch, forbidden) {
			t.Errorf("unsafe batch shortcut: %s", forbidden)
		}
	}
	// Pure logical boundary illustration, not a PostgreSQL execution claim.
	keys := []int64{math.MinInt64, -1, 0, math.MaxInt64}
	var cursor *int64
	var visited []int64
	for {
		var selected []int64
		for _, key := range keys {
			if cursor == nil || key > *cursor {
				selected = append(selected, key)
				if len(selected) == o.BatchSize {
					break
				}
			}
		}
		if len(selected) == 0 {
			break
		}
		visited = append(visited, selected...)
		hi := selected[len(selected)-1]
		cursor = &hi
	}
	if len(visited) != len(keys) || visited[0] != math.MinInt64 || visited[len(visited)-1] != math.MaxInt64 {
		t.Fatal("NULL-first cursor reference semantics lost signed boundary")
	}
	rollback := transitionFile(t, b, "70-rollback.sql")
	for _, want := range []string{"SAME connection", "After any earlier COMMIT", "stale retained array", "available old binaries", "Modern vector owns hnsw", "No CASCADE or automatic cleanup"} {
		if !strings.Contains(rollback, want) {
			t.Errorf("rollback boundary absent: %s", want)
		}
	}
	if !transitionGate(b, "fresh_catalog") || !transitionGate(b, "maintenance_window") || !transitionGate(b, "snapshot_and_old_service") {
		t.Fatal("external evidence was waived")
	}
	o.BatchSize++
	other := transitionGenerate(t, c, o)
	if other.Report.PlanSHA256 == b.Report.PlanSHA256 || strings.Contains(transitionFile(t, other, "30-batch.sql"), transitionLiteral(b.Report.PlanSHA256)) {
		t.Fatal("state can silently adopt different plan options")
	}
}

func TestTransitionReviewUnsafeEvidenceAndCoverage(t *testing.T) {
	c, o := fixtureCatalog(t), transitionFixtureOptions()
	payload := "never_interpolate_payload();\r\nDROP TABLE unrelated;"
	c.Indexes[0].Definition = payload
	c.SQLCorpus = append(c.SQLCorpus, SQLSource{Name: "untrusted evidence", SQL: payload})
	c.Dependencies = append(c.Dependencies, Dependency{Class: "pg_rewrite", OID: 49000, Identity: "unhandled view", ReferencedClass: "pg_class", ReferencedOID: o.RelationOID, Kind: "view", Definition: payload})
	b := transitionGenerate(t, c, o)
	if b.Report.Status != "blocked" || !strings.Contains(transitionFile(t, b, "inventory.json"), "never_interpolate_payload") {
		t.Fatal("evidence inventory or dependent-object block lost")
	}
	for _, a := range b.Artifacts {
		if strings.HasSuffix(a.Name, ".sql") && strings.Contains(a.Content, "never_interpolate_payload") {
			t.Errorf("raw catalog SQL leaked into %s", a.Name)
		}
		if a.Kind == "review_only_sql" {
			for _, line := range strings.Split(strings.TrimSpace(a.Content), "\n") {
				if !strings.HasPrefix(line, "--") {
					t.Errorf("active SQL phase line: %s", a.Name)
				}
			}
		}
	}
	c = fixtureCatalog(t)
	for i := range c.Scope.Coverage {
		if c.Scope.Coverage[i].Category == "dependencies" {
			c.Scope.Coverage[i].Complete, c.Scope.Coverage[i].Reason = false, "Only direct dependencies supplied"
		}
	}
	b = transitionGenerate(t, c, o)
	if b.Report.Status != "blocked" || !transitionGate(b, "dependency_inventory_incomplete") {
		t.Fatal("direct-only inventory accepted")
	}
	if _, err := GenerateTransition(context.Background(), c, o); err != nil {
		t.Fatal("blocked diagnostic generation should remain available", err)
	}
}

func TestTransitionReviewPinnedCapabilitiesAndIdentifierBounds(t *testing.T) {
	for _, target := range []TransitionIndex{
		{SourceIndexOID: 30100, Name: "lower_hnsw", Method: "hnsw", M: 2, EFConstruction: 4, EFSearch: 1},
		{SourceIndexOID: 30100, Name: "upper_hnsw", Method: "hnsw", M: 100, EFConstruction: 1000, EFSearch: 1000},
		{SourceIndexOID: 30100, Name: "lower_ivf", Method: "ivfflat", Lists: 1, Probes: 1},
		{SourceIndexOID: 30100, Name: "upper_ivf", Method: "ivfflat", Lists: 32768, Probes: 32768},
	} {
		o := transitionFixtureOptions()
		o.Indexes[0] = target
		b := transitionGenerate(t, fixtureCatalog(t), o)
		if b.Report.Status != "review_required" {
			t.Errorf("valid explicit endpoint rejected: %s", target.Name)
		}
	}
	for _, bad := range []TransitionIndex{
		{SourceIndexOID: 30100, Name: "bad", Method: "hnsw", M: 101, EFConstruction: 1000, EFSearch: 1},
		{SourceIndexOID: 30100, Name: "bad", Method: "hnsw", M: 2, EFConstruction: 4, EFSearch: 1001},
		{SourceIndexOID: 30100, Name: "bad", Method: "hnsw", M: 100, EFConstruction: 199, EFSearch: 1},
		{SourceIndexOID: 30100, Name: "bad", Method: "ivfflat", Lists: 32769, Probes: 1},
		{SourceIndexOID: 30100, Name: "bad", Method: "ivfflat", Lists: 1, Probes: 2},
		{SourceIndexOID: 30100, Name: "bad", Method: "hnsw; SELECT 1", M: 2, EFConstruction: 4, EFSearch: 1},
	} {
		o := transitionFixtureOptions()
		o.Indexes[0] = bad
		if _, err := GenerateTransition(context.Background(), fixtureCatalog(t), o); err == nil {
			t.Errorf("invalid explicit target accepted: %+v", bad)
		}
	}
	c, o := fixtureCatalog(t), transitionFixtureOptions()
	c.Extensions = append(c.Extensions, Extension{OID: 29999, Name: "vector", Version: "0.8.7", Schema: "public"})
	if b := transitionGenerate(t, c, o); b.Report.Status != "blocked" || !transitionGate(b, "target_already_installed") {
		t.Fatal("assumed same-database extension coexistence")
	}
	for _, version := range []int{120000, 180000, 190000} {
		c = fixtureCatalog(t)
		c.ServerVersionNum = version
		b := transitionGenerate(t, c, o)
		if (version == 190000) != transitionGate(b, "server_version_unsupported") {
			t.Errorf("server profile boundary mismatch: %d", version)
		}
	}
	c = fixtureCatalog(t)
	o.ShadowColumn = strings.Repeat("界", 21) // 63 UTF-8 bytes, not 63 runes.
	if b := transitionGenerate(t, c, o); b.Report.Status != "review_required" {
		t.Fatal("63-byte Unicode identifier rejected")
	}
	o.ShadowColumn += "x"
	if _, err := GenerateTransition(context.Background(), c, o); err == nil {
		t.Fatal("64-byte identifier could truncate to a different identity")
	}
}
