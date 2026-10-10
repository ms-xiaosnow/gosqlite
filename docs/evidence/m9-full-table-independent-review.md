# M9 whole-table generator independent review

Review date: 2026-10-10 (UTC). Reviewer owns this evidence file only; no production source, fixtures, staging, commits, or uploads were performed by the reviewer.

## Review boundary

- Durable base: `69008bf658611318154a112178913c693b679a2b`.
- Original reviewed immutable migration-only tree: `../full-table-freeze.CJbRPe`.
- Original overlay SHA-256: `7762788783cad7844c7ffadfc87ae3e9aa9bde333a807370a8d993b66076479f`.
- The original overlay's complete `source-sha256.txt` manifest was independently checked against that tree: PASS. The unrelated encryption and relational SQL working-tree changes were not tested or certified.
- Read `goal.md`, migration guides, new full-table guide, engineering instructions, CLI adapter, strict JSON and budget helpers, transition adapters, full-table source, synthetic fixture, examples and goldens. No applicable AGENTS.md was present in the repository/ancestor paths searched.
- Only synthetic Go generator/parser/JSON and normal text-output behavior is exercised. No database is opened by selected tests/probes, and no generated SQL is executed.
- Live PostgreSQL/Neon: **NOT_RUN**. Restricted physical durability, failed native I/O/close/lock/Sync, fault/crash/tamper/wrong-key/missing-selected-file/namespace substitution, hot recovery, interrupted publication, independent release qualification: **NOT_RUN**.

## Initial findings (original frozen bytes)

The original focused suite passed, but that was insufficient to certify PostgreSQL semantics. Independent source review and exported-API probes found the following implementation gaps and notified the owner immediately:

1. **Signed integer minimum literals.** The generator accepted a default captured as `'-2147483648'::integer` but rendered `-2147483648::"pg_catalog"."int4"`. PostgreSQL applies the cast before unary minus, so the positive magnitude overflows. The same issue affects int2/int8 minima and CHECK literals. The old int8 golden unit expectation explicitly accepted the incorrect text. Fix needs signed text cast or correctly grouped expression, with all three minima covered.
2. **Constraint-backed archive name collision.** Setting the PK archive name to the existing CHECK name `documents_flags_check` returned `review_required` and proposed `ALTER INDEX ... RENAME TO "documents_flags_check"`. PostgreSQL also renames the owning PK constraint and rejects the existing same-table constraint name. Relation/type namespace checks alone did not catch this.
3. **Missing mandatory row-type ownership facts.** Removing both `row_type` edges from the base and supplement, then recomputing the catalog identity, still returned `review_required`. Both composite-to-table and implicit-array-to-composite edges are mandatory for this claimed PG16 profile.
4. **Inconsistent index schema identity.** Changing ordinary source index 30202 to schema `pg_toast` still returned `review_required`; generated SQL archived it there and recreated it in `public`. A source table's indexes must be schema-bound consistently.
5. **Standalone PK/UNIQUE inheritance flag reversed.** Actual PG16 standalone PK/UNIQUE `connoinherit=true` was blocked, while the synthetic fixture's false value was accepted. PG16 index_constraint_create explicitly sets true when no parent constraint exists. This prevented a truthful ordinary-heap catalog from passing the claimed profile.
6. **Incomplete legacy/namespace dependency closure.** A known legacy operator-family-to-hnsw-method edge produced `full_dependency_20400`. The fixture omitted actual non-pinned namespace and legacy internal support dependencies. A truthful complete outgoing dependency graph needs exact supported-address classification, not broad unknown-edge permission. Mandatory known edges must also be required.
7. **Unbound built-in sort-family OID.** Replacing every captured float_ops sort-family OID with 99999 and changing matching dependency references still returned `review_required`. The declared PG16 built-in identity is OID 1970.
8. **Database-wide event-trigger absence not required.** Generic table-trigger inventory and pg_trigger queries do not establish absence of pg_event_trigger handlers that fire for CREATE/ALTER/DROP/GRANT/REVOKE. Add an explicit complete inventory category and fail-closed unsupported-presence treatment.

These are implementation gaps, not the separate, intentionally unresolved operational/live-release acceptance gates. Owner corrections were in progress when this initial evidence section was written. Final verification is recorded below only after independently checking the corrected exact bytes.

## Original selected command result

Toolchain: pinned Go1.27.2; `GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off CGO_ENABLED=0`; shared `../gocache` and `../go-path`.

Before the test suite, ran `go run ./internal/testscope`: PASS. Behavioral review additionally confirmed that the selected transition tests only inspect synthetic catalogs/SQL/JSON and read normal fixtures; CLI tests perform only ordinary exclusive creation/read-back of review text files. No importer/native DB or prohibited workflow is selected.

Command:

```text
go test -count=1 -run '^(TestTransition|TestFullTable|TestMigrateTransition|TestMigrateFullTable)' ./migration/postgres ./cmd/gosqlite ./examples/migration-full-table
```

Results:

```text
ok github.com/ms-xiaosnow/gosqlite/migration/postgres 0.501s
ok github.com/ms-xiaosnow/gosqlite/cmd/gosqlite 0.060s
ok github.com/ms-xiaosnow/gosqlite/examples/migration-full-table 0.040s
```

Independent exported-API probes read the fixed JSON fixture, mutate only in-memory evidence/options, recalculate `Inspect(...).CatalogSHA256`, and call `GenerateFullTableTransition`. Initial relevant results:

```text
baseline review_required
archive_check_collision review_required
missing_rowtype_edges review_required
wrong_index_schema review_required
minimum_int4 review_required (incorrect unary-minus-before-cast text)
actual_pk_noinherit blocked: full_constraint_31000, full_constraint_31001
legacy_am_edge blocked: full_dependency_20400
minimum_int2 review_required
minimum_int8 review_required
sort_family_oid review_required
sequence_alias error: duplicate sequence identity
```

## Primary PostgreSQL 16 references checked

- [Lexical precedence](https://www.postgresql.org/docs/16/sql-syntax-lexical.html#SQL-PRECEDENCE): cast/unary-minus parsing and identifier/string boundaries.
- [ALTER INDEX](https://www.postgresql.org/docs/16/sql-alterindex.html) and [constraint rename source](https://raw.githubusercontent.com/postgres/postgres/REL_16_STABLE/src/backend/catalog/pg_constraint.c): backing-index rename also renames constraints; same-table constraint collision is rejected.
- [Index constraint creation source](https://raw.githubusercontent.com/postgres/postgres/REL_16_STABLE/src/backend/catalog/index.c): standalone p/u connoinherit and internal constraint-index ownership.
- [Type dependency source](https://raw.githubusercontent.com/postgres/postgres/REL_16_STABLE/src/backend/catalog/pg_type.c): relation row-type and implicit array internal dependency links.
- [Operator-class/family dependency source](https://raw.githubusercontent.com/postgres/postgres/REL_16_STABLE/src/backend/commands/opclasscmds.c), [operator dependency source](https://raw.githubusercontent.com/postgres/postgres/REL_16_STABLE/src/backend/catalog/pg_operator.c), [heap catalog source](https://raw.githubusercontent.com/postgres/postgres/REL_16_STABLE/src/backend/catalog/heap.c), and [extension catalog source](https://raw.githubusercontent.com/postgres/postgres/REL_16_STABLE/src/backend/commands/extension.c): exact non-pinned namespace/support edges.
- [Built-in operator families](https://raw.githubusercontent.com/postgres/postgres/REL_16_STABLE/src/include/catalog/pg_opfamily.dat) and [operator classes](https://raw.githubusercontent.com/postgres/postgres/REL_16_STABLE/src/include/catalog/pg_opclass.dat): PG16 built-in identities.
- [TOAST creation source](https://raw.githubusercontent.com/postgres/postgres/REL_16_STABLE/src/backend/catalog/toasting.c): three-column internal storage, default two-key unique btree and ownership.
- [Dependency catalog](https://www.postgresql.org/docs/16/catalog-pg-depend.html): object addresses, automatic/internal semantics and omission of dependencies to pinned built-ins.
- [ALTER SEQUENCE](https://www.postgresql.org/docs/16/sql-altersequence.html): same-schema/same-owner OWNED BY replacement; dropping the new owner table/column would drop the sequence, while backup ownership is removed; unchanged parameters remain unchanged.
- [GRANT](https://www.postgresql.org/docs/16/sql-grant.html): owner implicit grant options, explicit self-revocable ordinary privileges, grantor identity and separate sequence privileges.
- [CREATE TABLE](https://www.postgresql.org/docs/16/sql-createtable.html), [CREATE INDEX](https://www.postgresql.org/docs/16/sql-createindex.html) and [constraint catalog](https://www.postgresql.org/docs/16/catalog-pg-constraint.html): table/type/index namespaces, scalar storage, index/constraint clauses and inheritance flags.
- [Event triggers](https://www.postgresql.org/docs/16/event-trigger-definition.html): database-wide DDL behavior independent of table pg_trigger rows.

These are official source/documentation reviews, not execution against a PostgreSQL binary. Stable-branch URLs are not a pinned executable oracle.

## Corrected-byte verification

**PASS within the declared offline generator profile; no remaining blocking implementation finding from this review.** This is not PostgreSQL execution approval or full M9/M10 qualification.

Corrected immutable tree: `../full-table-review-freeze.zC6p1M`.

- Base remains `69008bf658611318154a112178913c693b679a2b`.
- Corrected 33-path overlay SHA-256: `ecd01b2650c9eb54e013fb7e2c40ef15aa72c4056071d62a3c8ddbb9af1349dc`.
- Corrected source manifest SHA-256: `143319358ed7a5f68b35cd345c065503548ede19dfee5e93610cfbe4fb7b6100`.
- Independently reconstructed the expected file membership/content from `git archive` of the durable base plus the owned overlay, without modifying the candidate. Compared every regular file, not only the overlay: **850/850 files matched**, with exactly the 33 manifest-owned overlay paths. No extra or missing files. Per-path SHA-256 manifest also passed. The unfiltered shared working tree was not used for tests.

All eight initial findings were re-reviewed against these exact corrected bytes. The dependency classifier now requires both row-type links, exact namespace OID/name pairs, all legacy extension membership, known method-handler/operator-function/family-method links, mapped ANN opclasses and support-row ownership/implementation edges. It does not broaden unknown dependencies into permissions. The finite required-address map is constructed once per validation and reused, avoiding the draft's repeated allocation of the closure per supplied edge. Pinned built-in float_ops dependency rows are not fabricated in the synthetic fixture. System source/legacy schemas are explicitly gated by this bounded profile.

The corrected source also rejects a primary-key backing index with NULLS NOT DISTINCT, rather than silently losing that property during ordinary PRIMARY KEY reconstruction. PK/UNIQUE fixtures now reflect actual standalone PG16 connoinherit=true. Event-trigger coverage and a read-only pg_event_trigger collection query are explicit; both incomplete evidence and inventoried event-trigger presence suppress proposals.

### Independently rerun checks

Each test-suite/probe-suite run was immediately preceded by the pinned `go run ./internal/testscope` guard: PASS. The final two added tests were behaviorally reviewed: they only remove synthetic catalog dependency records in memory, regenerate reports, and check blocked gates. The remaining selected tests retain the previously reviewed generator/JSON/normal text-output behavior.

Same offline environment and shared caches as above. Unit, vet, build and standalone generation use CGO_ENABLED=0. Only the Go race instrumentation run uses CGO_ENABLED=1; it does not change production dependencies or select physical database workflows.

```text
go test -count=1 -run '^(TestTransition|TestFullTable|TestMigrateTransition|TestMigrateFullTable)' ./migration/postgres ./cmd/gosqlite ./examples/migration-full-table
ok github.com/ms-xiaosnow/gosqlite/migration/postgres 0.795s
ok github.com/ms-xiaosnow/gosqlite/cmd/gosqlite 0.070s
ok github.com/ms-xiaosnow/gosqlite/examples/migration-full-table 0.023s

go test -race -count=1 -run '^(TestTransition|TestFullTable|TestMigrateTransition|TestMigrateFullTable)' ./migration/postgres ./cmd/gosqlite ./examples/migration-full-table
ok github.com/ms-xiaosnow/gosqlite/migration/postgres 9.433s
ok github.com/ms-xiaosnow/gosqlite/cmd/gosqlite 1.444s
ok github.com/ms-xiaosnow/gosqlite/examples/migration-full-table 1.167s

go vet -buildvcs=false ./migration/postgres ./cmd/gosqlite ./examples/migration-full-table
PASS (exit 0)
go build -buildvcs=false ./migration/postgres ./cmd/gosqlite ./examples/migration-full-table
PASS (exit 0)
```

There are 26 selected top-level tests, including unchanged original retained-array/shadow API goldens and their CLI route. Catalog v1 and old option decoders/goldens were not modified by the owned overlay.

Ran the fixed standalone example and the explicit full-table CLI against the exact candidate fixture. Their JSON output was byte-identical:

```text
go run ./examples/migration-full-table
go run ./cmd/gosqlite migrate pgembedding transition --catalog examples/migration-full-table/catalog.json --options examples/migration-full-table/options.json --full-table-evidence examples/migration-full-table/evidence.json
Output SHA-256: eb6a99a0960fdbb86866122f2123d2d4ae0043b9b5beb4f89cf0f0ba9045f6ed
```

Independently parsed the complete output and checked all 14 artifacts, the 13 unique non-self manifest entries, every content SHA-256/UTF-8 byte count/kind/transaction marker, matching plan/catalog identities, review_required status and every human gate unresolved: PASS.

### Independent exported-API regression probes

These are reviewer-authored checks, separate from the implementation's unit assertions. They read ordinary fixture files and only mutate in-memory structs/JSON. For altered base catalogs, the evidence hash is rebound using the public inspector before regeneration, so missing-edge tests do not merely trip stale-hash protection.

```text
PASS baseline, hashes, commented SQL, option-order determinism
PASS archive-check collision
PASS missing rowtype ownership
PASS wrong source index namespace
PASS false PK noinherit
PASS wrong sort family OID
PASS event-trigger coverage missing
PASS event-trigger present
PASS source namespace dependency missing
PASS legacy family method missing
PASS owner grantor mismatch
PASS PUBLIC grant option
PASS raw default SQL
PASS converted-vector CHECK
PASS FK
PASS view dependency
PASS signed minimum int2
PASS signed minimum int4
PASS signed minimum int8
PASS strict duplicate/exact/missing/null/invalid UTF8/trailing/byte limits
```

Every blocked probe additionally checks that **all** review-only SQL artifacts say statements are withheld. Accepted integer boundary probes check the actual SET DEFAULT signed-text-cast output. No generated mutation is uncommented or executed.

### Semantic review outcome and remaining gates

The proposal genuinely creates a separate typed table, explicitly copies every supported column, compares row/key coverage and every column's binary-send representation, archives ordinary source index names, swaps table names, and reconstructs supported defaults/constraints/indexes/ACLs/comments. It is not shadow-column generation under a new label.

The original sequence OIDs are retained, with no nextval/setval/restart during the copy. The backup default is removed before ownership is rebound to the new table. The new table/column becomes the dependency target for future sequence drop behavior. Owner-origin ACLs are rebuilt after clearing captured creation recipients, with role and effective default-ACL facts remaining explicit. Non-owner grant chains, unsupported source/dependent facts and arbitrary raw SQL are gated rather than guessed.

Actual catalog authenticity/completeness, effective session/owner privileges, GUC/build capabilities, current provider extension availability, quiescence, capacity/locks, PostgreSQL SQL execution, data/business/ANN correctness and post-cutover reconciliation are still operational or live-qualification gates. The supplement is identity-bound evidence, not proof of those facts. Live PostgreSQL/Neon remains **NOT_RUN**. All restricted physical durability/security and independent release workflows remain **NOT_RUN**. No production-ready or full milestone completion claim is made.


## Reviewer probe source

The exact standalone probe used for the corrected candidate is preserved below. It imports the candidate module and must be run only from the verified migration-only tree, after the source-reference guard and behavioral review.

```go
package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	p "github.com/ms-xiaosnow/gosqlite/migration/postgres"
	"os"
	"strings"
)

func fail(s string) { panic(s) }
func must(e error) {
	if e != nil {
		panic(e)
	}
}
func input() (p.CatalogSnapshot, p.FullTableOptions, p.FullTableEvidence) {
	var c p.CatalogSnapshot
	var o p.FullTableOptions
	var e p.FullTableEvidence
	read := func(n string, v any) {
		b, er := os.ReadFile("examples/migration-full-table/" + n)
		must(er)
		must(json.Unmarshal(b, v))
	}
	read("catalog.json", &c)
	read("options.json", &o)
	read("evidence.json", &e)
	return c, o, e
}
func generate(c p.CatalogSnapshot, o p.FullTableOptions, e p.FullTableEvidence) p.TransitionBundle {
	r, er := p.Inspect(context.Background(), c, p.InspectOptions{Selected: []p.Selection{{RelationOID: o.Transition.RelationOID, AttNum: o.Transition.AttNum, Dimensions: o.Transition.Dimensions}}})
	must(er)
	e.CatalogSHA256 = r.CatalogSHA256
	b, er := p.GenerateFullTableTransition(context.Background(), c, o, e)
	must(er)
	return b
}
func blocked(b p.TransitionBundle, want string) {
	if b.Report.Status != "blocked" {
		fail("expected blocked: " + want)
	}
	hit := false
	for _, g := range b.Report.Gates {
		if g.Code == want && g.State == "blocked" {
			hit = true
		}
	}
	if !hit {
		fail("missing gate " + want)
	}
	for _, a := range b.Artifacts {
		if a.Kind == "review_only_sql" && !strings.Contains(a.Content, "statements withheld") {
			fail("mutation escaped " + want)
		}
	}
}
func remove(c *p.CatalogSnapshot, e *p.FullTableEvidence, pred func(p.Dependency) bool) {
	filter := func(ds []p.Dependency) []p.Dependency {
		out := []p.Dependency{}
		for _, d := range ds {
			if !pred(d) {
				out = append(out, d)
			}
		}
		return out
	}
	c.Dependencies = filter(c.Dependencies)
	e.Dependencies = filter(e.Dependencies)
}
func main() {
	c, o, e := input()
	base := generate(c, o, e)
	if base.Report.Status != "review_required" {
		fail("baseline blocked")
	}
	if len(base.Artifacts) != 14 {
		fail("artifact count")
	}
	for _, a := range base.Artifacts {
		sum := sha256.Sum256([]byte(a.Content))
		if a.SHA256 != hex.EncodeToString(sum[:]) {
			fail("artifact hash")
		}
		if a.Kind == "review_only_sql" {
			for _, l := range strings.Split(a.Content, "\n") {
				if strings.TrimSpace(l) != "" && !strings.HasPrefix(l, "--") {
					fail("active SQL")
				}
			}
		}
	}
	for i, j := 0, len(o.ArchiveNames)-1; i < j; i, j = i+1, j-1 {
		o.ArchiveNames[i], o.ArchiveNames[j] = o.ArchiveNames[j], o.ArchiveNames[i]
	}
	for i, j := 0, len(o.Transition.Indexes)-1; i < j; i, j = i+1, j-1 {
		o.Transition.Indexes[i], o.Transition.Indexes[j] = o.Transition.Indexes[j], o.Transition.Indexes[i]
	}
	canonical := generate(c, o, e)
	a, _ := json.Marshal(base)
	b, _ := json.Marshal(canonical)
	if !bytes.Equal(a, b) {
		fail("operator-order nondeterminism")
	}
	fmt.Println("PASS baseline, hashes, commented SQL, option-order determinism")
	for _, tc := range []struct {
		name, gate string
		change     func(*p.CatalogSnapshot, *p.FullTableOptions, *p.FullTableEvidence)
	}{
		{"archive-check collision", "full_name_collision", func(c *p.CatalogSnapshot, o *p.FullTableOptions, e *p.FullTableEvidence) {
			o.ArchiveNames[0].Name = e.Constraints[2].Name
		}},
		{"missing rowtype ownership", "full_row_type_dependency", func(c *p.CatalogSnapshot, o *p.FullTableOptions, e *p.FullTableEvidence) {
			remove(c, e, func(d p.Dependency) bool { return d.Kind == "row_type" })
		}},
		{"wrong source index namespace", "full_index_schema_30202", func(c *p.CatalogSnapshot, o *p.FullTableOptions, e *p.FullTableEvidence) {
			for i := range c.Indexes {
				if c.Indexes[i].OID == 30202 {
					c.Indexes[i].Schema = "pg_toast"
				}
			}
		}},
		{"false PK noinherit", "full_constraint_31000", func(c *p.CatalogSnapshot, o *p.FullTableOptions, e *p.FullTableEvidence) {
			e.Constraints[0].NoInherit = false
		}},
		{"wrong sort family OID", "full_legacy_family_20400", func(c *p.CatalogSnapshot, o *p.FullTableOptions, e *p.FullTableEvidence) {
			for i := range e.LegacyFamilies {
				e.LegacyFamilies[i].SortFamilyOID = 99999
			}
		}},
		{"event-trigger coverage missing", "full_coverage_event_triggers", func(c *p.CatalogSnapshot, o *p.FullTableOptions, e *p.FullTableEvidence) {
			a := []p.Coverage{}
			for _, v := range e.Coverage {
				if v.Category != "event_triggers" {
					a = append(a, v)
				}
			}
			e.Coverage = a
		}},
		{"event-trigger present", "full_unsupported_object_48000", func(c *p.CatalogSnapshot, o *p.FullTableOptions, e *p.FullTableEvidence) {
			e.UnsupportedObjects = append(e.UnsupportedObjects, p.FullUnsupportedObject{Class: "pg_event_trigger", OID: 48000, Kind: "event_trigger", Identity: "synthetic DDL handler", Definition: "never_execute()"})
		}},
		{"source namespace dependency missing", "full_object_dependency_30000", func(c *p.CatalogSnapshot, o *p.FullTableOptions, e *p.FullTableEvidence) {
			remove(c, e, func(d p.Dependency) bool { return d.Kind == "namespace" && d.Class == "pg_class" && d.OID == 30000 })
		}},
		{"legacy family method missing", "full_object_dependency_20400", func(c *p.CatalogSnapshot, o *p.FullTableOptions, e *p.FullTableEvidence) {
			remove(c, e, func(d p.Dependency) bool {
				return d.Class == "pg_opfamily" && d.OID == 20400 && d.ReferencedClass == "pg_am"
			})
		}},
		{"owner grantor mismatch", "full_acl_contract", func(c *p.CatalogSnapshot, o *p.FullTableOptions, e *p.FullTableEvidence) {
			e.Privileges[0].GrantorOID = 20
		}},
		{"PUBLIC grant option", "full_acl_contract", func(c *p.CatalogSnapshot, o *p.FullTableOptions, e *p.FullTableEvidence) {
			e.Privileges = append(e.Privileges, p.FullGrant{GrantorOID: 10, GranteeOID: 0, Privilege: "SELECT", GrantOption: true})
		}},
		{"raw default SQL", "full_default_32001", func(c *p.CatalogSnapshot, o *p.FullTableOptions, e *p.FullTableEvidence) {
			c.Relations[0].Columns[1].Default = "untrusted()"
			e.Defaults[1].Expression = "untrusted()"
		}},
		{"converted-vector CHECK", "full_constraint_31002", func(c *p.CatalogSnapshot, o *p.FullTableOptions, e *p.FullTableEvidence) {
			e.Constraints[2].Columns = []int{3}
			e.Constraints[2].Expression = "embedding IS NOT NULL"
		}},
		{"FK", "full_constraint_31002", func(c *p.CatalogSnapshot, o *p.FullTableOptions, e *p.FullTableEvidence) { e.Constraints[2].Kind = "f" }},
		{"view dependency", "full_dependency_49000", func(c *p.CatalogSnapshot, o *p.FullTableOptions, e *p.FullTableEvidence) {
			e.Dependencies = append(e.Dependencies, p.Dependency{Class: "pg_rewrite", OID: 49000, Kind: "view", ReferencedClass: "pg_class", ReferencedOID: 30000, Identity: "synthetic view", Definition: "never_execute()"})
		}},
	} {
		c, o, e := input()
		tc.change(&c, &o, &e)
		blocked(generate(c, o, e), tc.gate)
		fmt.Println("PASS", tc.name)
	}
	for _, tc := range []struct {
		oid         uint32
		name, value string
	}{{21, "int2", "-32768"}, {23, "int4", "-2147483648"}, {20, "int8", "-9223372036854775808"}} {
		c, o, e := input()
		found := false
		for _, t := range c.Types {
			if t.OID == tc.oid {
				found = true
			}
		}
		if !found {
			c.Types = append(c.Types, p.Type{OID: tc.oid, Schema: "pg_catalog", Name: tc.name, Kind: "b"})
		}
		c.Relations[0].Columns[4].TypeOID = tc.oid
		c.Relations[0].Columns[4].Default = "'" + tc.value + "'::" + tc.name
		e.Defaults[2].Expression = c.Relations[0].Columns[4].Default
		e.Columns[4].CollationOID = 0
		e.Columns[4].CollationName = ""
		e.Columns[4].CollationSchema = ""
		e.Columns[4].Storage = "p"
		b := generate(c, o, e)
		if b.Report.Status != "review_required" {
			fail("integer minimum rejected")
		}
		found = false
		for _, a := range b.Artifacts {
			if a.Name == "40-swap-and-rebind.sql" && strings.Contains(a.Content, "SET DEFAULT E'"+tc.value+"'::\"pg_catalog\".\""+tc.name+"\"") {
				found = true
			}
		}
		if !found {
			fail("incorrect integer literal " + tc.name)
		}
		fmt.Println("PASS signed minimum", tc.name)
	}
	_, o, e = input()
	raw, _ := json.Marshal(e)
	for _, x := range [][]byte{[]byte(`{}`), append([]byte(`{"catalog_sha256":"duplicate",`), raw[1:]...), bytes.Replace(raw, []byte(`"namespaces"`), []byte(`"NAMESPACES"`), 1), bytes.Replace(raw, []byte(`"standard_conforming_strings":true`), []byte(`"standard_conforming_strings":null`), 1), append(raw, []byte(`{}`)...), []byte{'{', 0xff, '}'}, []byte(strings.Repeat(" ", p.MaxFullTableEvidenceBytes+1))} {
		if _, err := p.DecodeFullTableEvidence(bytes.NewReader(x)); err == nil {
			fail("strict JSON accepted")
		}
	}
	if _, err := p.DecodeFullTableEvidence(nil); err == nil {
		fail("nil reader")
	}
	ro, _ := json.Marshal(o)
	for _, x := range [][]byte{[]byte(`{}`), append([]byte(`{"copy_table":"duplicate",`), ro[1:]...), []byte(strings.Repeat(" ", p.MaxTransitionOptionsBytes+1))} {
		if _, err := p.DecodeFullTableOptions(bytes.NewReader(x)); err == nil {
			fail("strict options accepted")
		}
	}
	fmt.Println("PASS strict duplicate/exact/missing/null/invalid UTF8/trailing/byte limits")
	fmt.Println("INDEPENDENT EXPORT-API SYNTHETIC PROBES PASS; PostgreSQL/Neon NOT_RUN")
}
```
