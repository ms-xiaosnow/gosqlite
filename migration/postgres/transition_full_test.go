package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// These tests are entirely offline synthetic catalog/SQL/JSON tests. No database
// is opened, no SQL executed and no physical fault/namespace workflow performed.
func fullFixture(t *testing.T) (CatalogSnapshot, FullTableOptions, FullTableEvidence) {
	t.Helper()
	c := fixtureCatalog(t)
	c.ServerVersionNum = 160004
	c.Types = append(c.Types, Type{OID: 30001, Schema: "public", Name: "documents", Kind: "c"}, Type{OID: 30002, Schema: "public", Name: "_documents", Kind: "b", ElementOID: 30001})
	c.Types = append(c.Types, Type{OID: 17, Schema: "pg_catalog", Name: "bytea", Kind: "b"}, Type{OID: 26, Schema: "pg_catalog", Name: "oid", Kind: "b"}, Type{OID: 16, Schema: "pg_catalog", Name: "bool", Kind: "b"}, Type{OID: 25, Schema: "pg_catalog", Name: "text", Kind: "b"})
	c.Relations[0].Columns[0].Default = "nextval('public.documents_id_seq'::regclass)"
	c.Relations[0].Columns[1].Default = "0"
	c.Relations[0].Columns = append(c.Relations[0].Columns, Column{AttNum: 5, Name: "title", TypeOID: 25, TypeMod: -1, NotNull: true, Default: "'untitled'::text"})
	c.Relations = append(c.Relations, Relation{OID: 30300, Schema: "public", Name: "documents_id_seq", Kind: "S", Persistence: "p", Columns: []Column{}, Parents: []uint32{}, PrimaryKey: []int{}})
	c.Scope.Schemas = append(c.Scope.Schemas, "pg_toast")
	c.Relations = append(c.Relations, Relation{OID: 30400, Schema: "pg_toast", Name: "pg_toast_30000", Kind: "t", Persistence: "p", Parents: []uint32{}, PrimaryKey: []int{}, Columns: []Column{{AttNum: 1, Name: "chunk_id", TypeOID: 26, TypeMod: -1}, {AttNum: 2, Name: "chunk_seq", TypeOID: 23, TypeMod: -1}, {AttNum: 3, Name: "chunk_data", TypeOID: 17, TypeMod: -1}}})
	c.Functions = append(c.Functions, Function{OID: 184, Schema: "pg_catalog", Name: "oideq", ArgumentOIDs: []uint32{26, 26}, ReturnOID: 16})
	c.Operators = append(c.Operators, Operator{OID: 607, Schema: "pg_catalog", Name: "=", LeftOID: 26, RightOID: 26, FunctionOID: 184})
	c.OpClasses = append(c.OpClasses, OpClass{OID: 1981, Schema: "pg_catalog", Name: "oid_ops", MethodOID: 403, InputOID: 26, Default: true, OperatorOID: 607, FunctionOID: 184})
	c.Indexes = append(c.Indexes, Index{OID: 30401, Schema: "pg_toast", Name: "pg_toast_30000_index", TableOID: 30400, MethodOID: 403, Keys: []int{1, 2}, OpClasses: []uint32{1981, 1978}, Collations: []uint32{0, 0}, Options: []string{}, Valid: true, Ready: true, Definition: "Synthetic internal TOAST index."})
	c.AccessMethods = append(c.AccessMethods, AccessMethod{OID: 403, Name: "btree", HandlerOID: 330})
	c.Functions = append(c.Functions, Function{OID: 330, Schema: "pg_catalog", Name: "bthandler", ArgumentOIDs: []uint32{2281}, ReturnOID: 325}, Function{OID: 65, Schema: "pg_catalog", Name: "int4eq", ArgumentOIDs: []uint32{23, 23}, ReturnOID: 16})
	c.Operators = append(c.Operators, Operator{OID: 96, Schema: "pg_catalog", Name: "=", LeftOID: 23, RightOID: 23, FunctionOID: 65})
	c.OpClasses = append(c.OpClasses, OpClass{OID: 1978, Schema: "pg_catalog", Name: "int4_ops", MethodOID: 403, InputOID: 23, Default: true, OperatorOID: 96, FunctionOID: 65})
	for i, name := range []string{"documents_pkey", "documents_flags_key", "documents_flags_idx"} {
		key := 2
		if i == 0 {
			key = 1
		}
		c.Indexes = append(c.Indexes, Index{OID: uint32(30200 + i), Schema: "public", Name: name, TableOID: 30000, MethodOID: 403, Keys: []int{key}, OpClasses: []uint32{1978}, Collations: []uint32{0}, Options: []string{}, Valid: true, Ready: true, Definition: "Synthetic ordinary index evidence; never executable."})
	}
	base := transitionFixtureOptions()
	base.Approach = FullTableCopy
	base.Mode = TransitionSingle
	base.ShadowColumn = ""
	base.StateSchema = ""
	base.StateTable = ""
	base.BatchSize = 0
	o := FullTableOptions{Transition: base, CopyTable: "documents_copy", BackupTable: "documents_backup", ArchiveNames: []FullArchiveName{{30200, "documents_backup_pkey"}, {30201, "documents_backup_flags_key"}, {30202, "documents_backup_flags_idx"}}}
	e := FullTableEvidence{Namespaces: []FullNamespace{{2200, "public"}, {20002, "legacy"}}, LegacyFamilies: []FullLegacyFamily{}, FormatVersion: FullTableEvidenceVersion, Collector: FullTableCollector, ExtractedAt: c.ExtractedAt, ServerVersionNum: c.ServerVersionNum, StandardConformingStrings: true, DeparseSearchPath: "pg_catalog", SourceProfile: SourceProfile, TargetProfile: TargetProfile,
		Coverage: []Coverage{}, Table: FullTableFacts{ToastRelationOID: 30400, ToastIndexOID: 30401, ToastOwnerOID: 10, ToastOptions: []string{}, RelationOID: 30000, Schema: "public", Name: "documents", NamespaceOID: 2200, OwnerOID: 10, RowTypeOID: 30001, RowArrayTypeOID: 30002, AccessMethodOID: 2, AccessMethod: "heap", TablespaceOID: 1663, Tablespace: "pg_default", ReplicaIdentity: "d", Options: []string{}, Comment: "Synthetic full-table fixture."},
		Roles: []FullRole{{10, "owner"}, {20, "reader"}, {21, "writer"}}, Columns: []FullColumnFacts{}, Defaults: []FullDefault{{32000, 1, c.Relations[0].Columns[0].Default, 30300}, {32001, 2, "0", 0}, {32002, 5, "'untitled'::text", 0}},
		Constraints: []FullConstraint{{OID: 31000, Name: "documents_pkey", Kind: "p", Columns: []int{1}, IndexOID: 30200, Validated: true, NoInherit: true}, {OID: 31001, Name: "documents_flags_key", Kind: "u", Columns: []int{2}, IndexOID: 30201, Validated: true, NoInherit: true}, {OID: 31002, Name: "documents_flags_check", Kind: "c", Columns: []int{2}, Validated: true, NoInherit: true, Expression: "(flags >= 0)", Comment: "Nonnegative flags."}},
		Indexes:     []FullIndexFacts{}, Sequences: []FullSequence{{30300, "public", "documents_id_seq", 10, 1, "a"}}, Privileges: []FullGrant{}, CreationPrivileges: []FullGrant{{10, 20, 0, "SELECT", false}}, Dependencies: []Dependency{}, UnsupportedObjects: []FullUnsupportedObject{}}
	for _, category := range fullCoverage {
		e.Coverage = append(e.Coverage, Coverage{Category: category, Complete: true, Reason: "Synthetic exhaustive evidence only; no live capture."})
	}
	for _, a := range c.Relations[0].Columns {
		fact := FullColumnFacts{AttNum: a.AttNum, Storage: "p", Statistics: -1, Options: []string{}}
		if a.TypeOID == 1021 || a.TypeOID == 25 {
			fact.Storage = "x"
		}
		if a.TypeOID == 25 {
			fact.CollationOID = 100
			fact.CollationSchema = "pg_catalog"
			fact.CollationName = "default"
		}
		e.Columns = append(e.Columns, fact)
	}
	for _, p := range []string{"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER"} {
		e.Privileges = append(e.Privileges, FullGrant{10, 10, 0, p, false})
	}
	e.Privileges = append(e.Privileges, FullGrant{10, 20, 0, "SELECT", true}, FullGrant{10, 21, 5, "UPDATE", false})
	dep := func(class string, id uint32, kind, refclass string, ref uint32, sub int) {
		e.Dependencies = append(e.Dependencies, Dependency{Class: class, OID: id, Identity: "Synthetic explicit object address", ReferencedClass: refclass, ReferencedOID: ref, ReferencedSubID: sub, Kind: kind, Definition: "Evidence only."})
	}
	for _, x := range c.Indexes {
		if x.TableOID != 30000 {
			continue
		}
		fact := FullIndexFacts{OID: x.OID, Immediate: true, Live: true, Keys: []FullIndexKey{{AttNum: x.Keys[0]}}, Included: []int{}, TablespaceOID: 1663, Tablespace: "pg_default", Comment: "Synthetic index."}
		if x.OID == 30200 || x.OID == 30201 {
			fact.Unique = true
			fact.Primary = x.OID == 30200
			fact.ConstraintOID = x.OID + 800
			dep("pg_class", x.OID, "index", "pg_constraint", fact.ConstraintOID, 0)
		} else {
			dep("pg_class", x.OID, "index", "pg_class", 30000, x.Keys[0])
		}
		if x.OID == 30202 {
			fact.Keys[0].Descending = true
			fact.Keys[0].NullsFirst = true
		}
		e.Indexes = append(e.Indexes, fact)
	}
	for _, con := range e.Constraints {
		for _, a := range con.Columns {
			dep("pg_constraint", con.OID, "constraint", "pg_class", 30000, a)
		}
	}
	for _, d := range e.Defaults {
		dep("pg_attrdef", d.OID, "default", "pg_class", 30000, d.AttNum)
		if d.SequenceOID != 0 {
			dep("pg_attrdef", d.OID, "default", "pg_class", d.SequenceOID, 0)
		}
	}
	dep("pg_class", 30400, "internal_storage", "pg_class", 30000, 0)
	dep("pg_class", 30401, "internal_storage", "pg_class", 30400, 1)
	dep("pg_class", 30401, "internal_storage", "pg_class", 30400, 2)
	dep("pg_class", 30300, "sequence", "pg_class", 30000, 1)
	dep("pg_type", 30001, "row_type", "pg_class", 30000, 0)
	dep("pg_type", 30002, "row_type", "pg_type", 30001, 0)
	for i, oc := range c.OpClasses {
		if oc.ExtensionOID != 20000 {
			continue
		}
		n := uint32(i)
		fam := FullLegacyFamily{OID: 20400 + n, Schema: oc.Schema, Name: oc.Name, MethodOID: oc.MethodOID, OpClassOID: oc.OID, AMOpOID: 20500 + n, AMProcOID: 20600 + n, Strategy: 1, Support: 1, LeftOID: 1021, RightOID: 1021, OperatorOID: oc.OperatorOID, FunctionOID: oc.FunctionOID, SortFamilyOID: 1970, SortFamilySchema: "pg_catalog", SortFamilyName: "float_ops", SortMethodOID: 403}
		e.LegacyFamilies = append(e.LegacyFamilies, fam)
		dep("pg_opfamily", fam.OID, "extension_member", "pg_extension", 20000, 0)
		dep("pg_opclass", fam.OpClassOID, "operator_family", "pg_opfamily", fam.OID, 0)
		dep("pg_amop", fam.AMOpOID, "operator_family", "pg_opclass", fam.OpClassOID, 0)
		dep("pg_amop", fam.AMOpOID, "operator_family", "pg_operator", fam.OperatorOID, 0)
		// Pinned built-in float_ops has no ordinary pg_depend row.
		dep("pg_amproc", fam.AMProcOID, "operator_family", "pg_opclass", fam.OpClassOID, 0)
		dep("pg_amproc", fam.AMProcOID, "operator_family", "pg_proc", fam.FunctionOID, 0)
	}
	dep("pg_class", 30000, "namespace", "pg_namespace", 2200, 0)
	dep("pg_class", 30300, "namespace", "pg_namespace", 2200, 0)
	dep("pg_extension", 20000, "namespace", "pg_namespace", 20002, 0)
	for _, x := range c.AccessMethods {
		if x.ExtensionOID == 20000 {
			dep("pg_am", x.OID, "extension_member", "pg_extension", 20000, 0)
			dep("pg_am", x.OID, "legacy_implementation", "pg_proc", x.HandlerOID, 0)
		}
	}
	for _, x := range c.Functions {
		if x.ExtensionOID == 20000 {
			dep("pg_proc", x.OID, "extension_member", "pg_extension", 20000, 0)
			dep("pg_proc", x.OID, "namespace", "pg_namespace", 20002, 0)
		}
	}
	for _, x := range c.Operators {
		if x.ExtensionOID == 20000 {
			dep("pg_operator", x.OID, "extension_member", "pg_extension", 20000, 0)
			dep("pg_operator", x.OID, "namespace", "pg_namespace", 20002, 0)
			dep("pg_operator", x.OID, "legacy_implementation", "pg_proc", x.FunctionOID, 0)
		}
	}
	for _, x := range c.OpClasses {
		if x.ExtensionOID == 20000 {
			dep("pg_opclass", x.OID, "extension_member", "pg_extension", 20000, 0)
			dep("pg_opclass", x.OID, "namespace", "pg_namespace", 20002, 0)
		}
	}
	for _, x := range e.LegacyFamilies {
		dep("pg_opfamily", x.OID, "namespace", "pg_namespace", 20002, 0)
		dep("pg_opfamily", x.OID, "operator_family", "pg_am", x.MethodOID, 0)
	}
	for _, x := range c.Indexes {
		if x.MethodOID == 20001 {
			for _, oid := range x.OpClasses {
				dep("pg_class", x.OID, "index", "pg_opclass", oid, 0)
			}
		}
	}
	c.Dependencies = append([]Dependency{}, e.Dependencies...)
	rebindFull(t, c, &e)
	// Match the public CLI's decoded-file provenance as well as canonical facts.
	// A programmatically mutated base fixture has no unchanged source-byte hash.
	raw, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	c, err = DecodeCatalog(bytes.NewReader(append(raw, '\n')), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	return c, o, e
}
func rebindFull(t *testing.T, c CatalogSnapshot, e *FullTableEvidence) {
	t.Helper()
	r, err := Inspect(context.Background(), c, InspectOptions{Selected: []Selection{{RelationOID: 30000, AttNum: 3, Dimensions: 3}}})
	if err != nil {
		t.Fatal(err)
	}
	e.CatalogSHA256 = r.CatalogSHA256
}
func generateFull(t *testing.T, c CatalogSnapshot, o FullTableOptions, e FullTableEvidence) TransitionBundle {
	t.Helper()
	b, err := GenerateFullTableTransition(context.Background(), c, o, e)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func assertFullBlocked(t *testing.T, b TransitionBundle, gate string) {
	t.Helper()
	if b.Report.Status != "blocked" || !transitionGate(b, gate) {
		t.Fatalf("missing %s: %+v", gate, b.Report.Gates)
	}
	for _, a := range b.Artifacts {
		if a.Kind == "review_only_sql" && !strings.Contains(a.Content, "statements withheld") {
			t.Fatal("blocked proposal escaped", a.Name)
		}
	}
}

func TestFullTableCopySwapAndContracts(t *testing.T) {
	c, o, e := fullFixture(t)
	before, _ := json.Marshal(struct {
		C CatalogSnapshot
		O FullTableOptions
		E FullTableEvidence
	}{c, o, e})
	b := generateFull(t, c, o, e)
	if b.Report.Status != "review_required" {
		t.Fatalf("unexpected blocked gates: %+v", b.Report.Gates)
	}
	copy := transitionFile(t, b, "20-full-table-copy.sql")
	for _, want := range []string{`CREATE TABLE "public"."documents_copy"`, `"embedding" "public"."vector"(3)`, `"ordinary_samples" pg_catalog.float4[]`, `INSERT INTO "public"."documents_copy" ("id", "flags", "embedding", "ordinary_samples", "title")`, `src."embedding"::"public"."vector"(3)`, `FROM ONLY "public"."documents" AS src`, `SET LOCAL ROLE "owner";`} {
		if !strings.Contains(copy, want) {
			t.Error(want)
		}
	}
	swap := transitionFile(t, b, "40-swap-and-rebind.sql")
	for _, want := range []string{`ALTER INDEX "public"."documents_pkey" RENAME TO "documents_backup_pkey";`, `ALTER TABLE "public"."documents" RENAME TO "documents_backup";`, `ALTER TABLE "public"."documents_copy" RENAME TO "documents";`, `ADD CONSTRAINT "documents_pkey" PRIMARY KEY ("id")`, `UNIQUE ("flags") USING INDEX TABLESPACE "pg_default" NOT DEFERRABLE`, `CHECK ("flags" OPERATOR(pg_catalog.>=) E'0'::"pg_catalog"."int4") NO INHERIT`, `"flags" "pg_catalog"."int4_ops" DESC NULLS FIRST`, `ALTER SEQUENCE "public"."documents_id_seq" OWNED BY "public"."documents"."id";`, `ALTER TABLE "public"."documents_backup" ALTER COLUMN "id" DROP DEFAULT;`, `SET DEFAULT pg_catalog.nextval(E'"public"."documents_id_seq"'::pg_catalog.regclass)`, `GRANT SELECT ON TABLE "public"."documents" TO "reader" WITH GRANT OPTION;`, `GRANT UPDATE ("title") ON TABLE "public"."documents" TO "writer";`, `REVOKE ALL PRIVILEGES ON TABLE "public"."documents" FROM "reader";`, `USING hnsw ("embedding" "public"."vector_l1_ops")`} {
		if !strings.Contains(swap, want) {
			t.Error(want)
		}
	}
	if strings.Index(swap, "RENAME TO \"documents_backup_pkey\"") > strings.Index(swap, "ADD CONSTRAINT \"documents_pkey\"") {
		t.Fatal("recreation before archive")
	}
	for _, bad := range []string{"DROP TABLE ", "CASCADE;", "SELECT *", "setval(", "RESTART WITH", "ADD COLUMN"} {
		if strings.Contains(copy+swap, bad) {
			t.Error("unexpected shortcut", bad)
		}
	}
	verify := transitionFile(t, b, "30-verify-copy.sql")
	for _, want := range []string{"extra_keys", "missing_keys", "column_5_binary_mismatches", "pg_catalog.textsend", "pg_catalog.array_send", "unique_copy_keys", "dimension_mismatches"} {
		if !strings.Contains(verify, want) {
			t.Error(want)
		}
	}
	for _, a := range b.Artifacts {
		if a.SHA256 != sqlHash(a.Content) {
			t.Fatal("hash")
		}
		if a.Kind == "review_only_sql" {
			_, segments, err := pgScan(a.Content, DefaultSQLLimits())
			if err != nil || len(segments) != 0 {
				t.Fatal("active SQL", a.Name, err)
			}
		}
	}
	for _, g := range b.Report.Gates {
		if g.State != "unresolved" {
			t.Fatal(g)
		}
	}
	after, _ := json.Marshal(struct {
		C CatalogSnapshot
		O FullTableOptions
		E FullTableEvidence
	}{c, o, e})
	if !bytes.Equal(before, after) {
		t.Fatal("input mutated")
	}
	again := generateFull(t, c, o, e)
	if !reflect.DeepEqual(b, again) {
		t.Fatal("nondeterministic")
	}
}

func TestFullTableEvidenceGates(t *testing.T) {
	cases := []struct {
		name, gate string
		change     func(*CatalogSnapshot, *FullTableOptions, *FullTableEvidence)
	}{
		{"hash", "full_evidence_binding", func(c *CatalogSnapshot, o *FullTableOptions, e *FullTableEvidence) { e.CatalogSHA256 = "stale" }},
		{"version", "full_server_profile", func(c *CatalogSnapshot, o *FullTableOptions, e *FullTableEvidence) {
			c.ServerVersionNum = 180000
			e.ServerVersionNum = 180000
			rebindFull(t, *c, e)
		}},
		{"coverage", "full_coverage_triggers", func(c *CatalogSnapshot, o *FullTableOptions, e *FullTableEvidence) {
			for i := range e.Coverage {
				if e.Coverage[i].Category == "triggers" {
					e.Coverage[i].Complete = false
				}
			}
		}},
		{"column", "full_column_5", func(c *CatalogSnapshot, o *FullTableOptions, e *FullTableEvidence) { e.Columns = e.Columns[:4] }},
		{"default_sql", "full_default_32001", func(c *CatalogSnapshot, o *FullTableOptions, e *FullTableEvidence) {
			c.Relations[0].Columns[1].Default = "untrusted()"
			e.Defaults[1].Expression = "untrusted()"
			rebindFull(t, *c, e)
		}},
		{"check_sql", "full_constraint_31002", func(c *CatalogSnapshot, o *FullTableOptions, e *FullTableEvidence) {
			e.Constraints[2].Expression = "flags >= 0 OR untrusted()"
		}},
		{"converted_check", "full_constraint_31002", func(c *CatalogSnapshot, o *FullTableOptions, e *FullTableEvidence) {
			e.Constraints[2].Columns = []int{3}
			e.Constraints[2].Expression = "embedding IS NOT NULL"
		}},
		{"primary_inherit", "full_constraint_31000", func(c *CatalogSnapshot, o *FullTableOptions, e *FullTableEvidence) {
			e.Constraints[0].NoInherit = false
		}},
		{"unique_inherit", "full_constraint_31001", func(c *CatalogSnapshot, o *FullTableOptions, e *FullTableEvidence) {
			e.Constraints[1].NoInherit = false
		}},
		{"primary_nulls_not_distinct", "full_index_nulls_30200", func(c *CatalogSnapshot, o *FullTableOptions, e *FullTableEvidence) {
			for i := range e.Indexes {
				if e.Indexes[i].OID == 30200 {
					e.Indexes[i].NullsNotDistinct = true
				}
			}
		}},
		{"deferrable", "full_constraint_31001", func(c *CatalogSnapshot, o *FullTableOptions, e *FullTableEvidence) {
			e.Constraints[1].Deferrable = true
		}},
		{"fk", "full_constraint_31002", func(c *CatalogSnapshot, o *FullTableOptions, e *FullTableEvidence) { e.Constraints[2].Kind = "f" }},
		{"index_missing", "full_index_30202", func(c *CatalogSnapshot, o *FullTableOptions, e *FullTableEvidence) {
			e.Indexes = e.Indexes[:len(e.Indexes)-1]
		}},
		{"index_shape", "full_index_shape_30202", func(c *CatalogSnapshot, o *FullTableOptions, e *FullTableEvidence) {
			e.Indexes[len(e.Indexes)-1].Keys = append(e.Indexes[len(e.Indexes)-1].Keys, FullIndexKey{AttNum: 5})
		}},
		{"sequence_owner", "full_sequence_30300", func(c *CatalogSnapshot, o *FullTableOptions, e *FullTableEvidence) { e.Sequences[0].OwnerOID = 20 }},
		{"sequence_absence", "full_default_32000", func(c *CatalogSnapshot, o *FullTableOptions, e *FullTableEvidence) { e.Sequences = []FullSequence{} }},
		{"acl_grantor", "full_acl_contract", func(c *CatalogSnapshot, o *FullTableOptions, e *FullTableEvidence) { e.Privileges[0].GrantorOID = 20 }},
		{"archive_collision", "full_name_collision", func(c *CatalogSnapshot, o *FullTableOptions, e *FullTableEvidence) {
			o.BackupTable = "documents_flags_idx"
		}},
		{"archive_constraint_collision", "full_name_collision", func(c *CatalogSnapshot, o *FullTableOptions, e *FullTableEvidence) {
			o.ArchiveNames[0].Name = e.Constraints[2].Name
		}},
		{"index_schema", "full_index_schema_30202", func(c *CatalogSnapshot, o *FullTableOptions, e *FullTableEvidence) {
			for i := range c.Indexes {
				if c.Indexes[i].OID == 30202 {
					c.Indexes[i].Schema = "pg_toast"
				}
			}
			rebindFull(t, *c, e)
		}},
		{"view", "full_dependency_40000", func(c *CatalogSnapshot, o *FullTableOptions, e *FullTableEvidence) {
			e.Dependencies = append(e.Dependencies, Dependency{Class: "pg_rewrite", OID: 40000, ReferencedClass: "pg_class", ReferencedOID: 30000, Kind: "view", Definition: "SELECT untrusted()"})
		}},
		{"missing_edge", "full_dependency_missing_30100", func(c *CatalogSnapshot, o *FullTableOptions, e *FullTableEvidence) {
			e.Dependencies = e.Dependencies[1:]
		}},
		{"toast", "full_toast_contract", func(c *CatalogSnapshot, o *FullTableOptions, e *FullTableEvidence) {
			e.Table.ToastOptions = []string{"autovacuum_enabled=false"}
		}},
		{"family", "full_legacy_family_20400", func(c *CatalogSnapshot, o *FullTableOptions, e *FullTableEvidence) { e.LegacyFamilies[0].Strategy = 2 }},
		{"sort_family_identity", "full_legacy_family_20400", func(c *CatalogSnapshot, o *FullTableOptions, e *FullTableEvidence) {
			for i := range e.LegacyFamilies {
				e.LegacyFamilies[i].SortFamilyOID = 99999
			}
		}},
		{"event_trigger_coverage", "full_coverage_event_triggers", func(c *CatalogSnapshot, o *FullTableOptions, e *FullTableEvidence) {
			for i := range e.Coverage {
				if e.Coverage[i].Category == "event_triggers" {
					e.Coverage[i].Complete = false
				}
			}
		}},
		{"event_trigger", "full_unsupported_object_45001", func(c *CatalogSnapshot, o *FullTableOptions, e *FullTableEvidence) {
			e.UnsupportedObjects = append(e.UnsupportedObjects, FullUnsupportedObject{Class: "pg_event_trigger", OID: 45001, Kind: "event_trigger", Definition: "Synthetic DDL event trigger"})
		}},
		{"namespace_identity", "full_namespace_identity", func(c *CatalogSnapshot, o *FullTableOptions, e *FullTableEvidence) { e.Namespaces = e.Namespaces[1:] }},
		{"namespace_wrong_edge", "full_dependency_30000", func(c *CatalogSnapshot, o *FullTableOptions, e *FullTableEvidence) {
			e.Dependencies = append(e.Dependencies, Dependency{Class: "pg_class", OID: 30000, Kind: "namespace", ReferencedClass: "pg_namespace", ReferencedOID: 20002, Identity: "Wrong namespace"})
		}},
		{"missing_family", "full_legacy_family_missing_20300", func(c *CatalogSnapshot, o *FullTableOptions, e *FullTableEvidence) {
			e.LegacyFamilies = e.LegacyFamilies[1:]
		}},
		{"publication", "full_unsupported_object_45000", func(c *CatalogSnapshot, o *FullTableOptions, e *FullTableEvidence) {
			e.UnsupportedObjects = append(e.UnsupportedObjects, FullUnsupportedObject{Class: "pg_publication", OID: 45000, Kind: "publication", Definition: "untrusted"})
		}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			c, o, e := fullFixture(t)
			tt.change(&c, &o, &e)
			assertFullBlocked(t, generateFull(t, c, o, e), tt.gate)
		})
	}
}

func TestFullTableStrictJSONOptionsAndLimits(t *testing.T) {
	c, o, e := fullFixture(t)
	for _, v := range []struct {
		raw    []byte
		decode func([]byte) error
	}{
		{mustFullJSON(t, o), func(p []byte) error { _, err := DecodeFullTableOptions(bytes.NewReader(p)); return err }},
		{mustFullJSON(t, e), func(p []byte) error { _, err := DecodeFullTableEvidence(bytes.NewReader(p)); return err }},
	} {
		if err := v.decode(v.raw); err != nil {
			t.Fatal(err)
		}
		for _, raw := range [][]byte{[]byte(`{}`), []byte(`null`), append(v.raw, []byte(`{}`)...), bytes.Replace(v.raw, []byte(`{`), []byte(`{"unknown":1,`), 1), bytes.Replace(v.raw, []byte(`{`), []byte(`{"duplicate":1,"duplicate":2,`), 1)} {
			if err := v.decode(raw); err == nil {
				t.Fatal("invalid JSON accepted")
			}
		}
	}
	for _, change := range []func(*FullTableOptions){func(o *FullTableOptions) { o.Transition.Mode = TransitionBatches }, func(o *FullTableOptions) { o.Transition.ConcurrentIndexes = true }, func(o *FullTableOptions) { o.Transition.ShadowColumn = "shadow" }, func(o *FullTableOptions) { o.CopyTable = "x\rSELECT 1" }, func(o *FullTableOptions) { o.Transition.MaxBytes = 100 }} {
		copy := o
		change(&copy)
		if _, err := GenerateFullTableTransition(context.Background(), c, copy, e); err == nil {
			t.Fatal("invalid options accepted")
		}
	}
	if _, err := GenerateTransition(context.Background(), c, o.Transition); err == nil {
		t.Fatal("full-table bypassed supplement API")
	}
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := GenerateFullTableTransition(canceled, c, o, e); err != context.Canceled {
		t.Fatal(err)
	}
	e.Table.Comment = strings.Repeat("x", MaxFullTableEvidenceBytes+1)
	if _, err := GenerateFullTableTransition(context.Background(), c, o, e); err == nil {
		t.Fatal("programmatic budget")
	}
}
func mustFullJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestFullTableLiteralAndIdentifierBoundary(t *testing.T) {
	for _, raw := range []string{"0; SELECT untrusted()", "pg_catalog.random()", "0::numeric", "(0 OR true)", "E'escape'", "'bad'::int4", "9223372036854775808", "1e999"} {
		ts, err := fullTokens(raw)
		if err == nil {
			_, err = fullLiteral(ts, 23)
		}
		if err == nil {
			t.Error("unsupported literal", raw)
		}
	}
	c, o, e := fullFixture(t)
	name := `copy"; SELECT injected; --路径`
	o.CopyTable = name
	e.Table.Comment = "text\rSELECT hidden();\nDROP something;"
	b := generateFull(t, c, o, e)
	if b.Report.Status != "review_required" {
		t.Fatal(b.Report.Gates)
	}
	if !strings.Contains(transitionFile(t, b, "20-full-table-copy.sql"), quoteIdent(name)) {
		t.Fatal("quoting")
	}
	for _, a := range b.Artifacts {
		if a.Kind == "review_only_sql" {
			_, segments, err := pgScan(a.Content, DefaultSQLLimits())
			if err != nil || len(segments) != 0 {
				t.Fatal("comment escape", a.Name, err)
			}
		}
	}
	e.Defaults[0].Expression = "nextval('evil.documents_id_seq'::regclass)"
	c.Relations[0].Columns[0].Default = e.Defaults[0].Expression
	rebindFull(t, c, &e)
	assertFullBlocked(t, generateFull(t, c, o, e), "full_default_32000")
}

func TestFullTableGoldenFixture(t *testing.T) {
	c, o, e := fullFixture(t)
	root := "../../examples/migration-full-table"
	for _, pair := range []struct {
		name  string
		value any
	}{{"catalog.json", c}, {"options.json", o}, {"evidence.json", e}} {
		raw, err := os.ReadFile(filepath.Join(root, pair.name))
		if err != nil {
			t.Fatal(err)
		}
		var got, want any
		if err = json.Unmarshal(raw, &got); err != nil {
			t.Fatal(err)
		}
		if err = json.Unmarshal(mustFullJSON(t, pair.value), &want); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Error("fixture drift", pair.name)
		}
	}
	b := generateFull(t, c, o, e)
	for _, a := range b.Artifacts {
		raw, err := os.ReadFile(filepath.Join(root, "golden", a.Name))
		if err != nil {
			t.Fatal(err)
		}
		if string(raw) != a.Content {
			t.Error("golden drift", a.Name)
		}
	}
}

func TestFullTableTypedLiteralProfiles(t *testing.T) {
	for _, tt := range []struct {
		source string
		oid    uint32
		want   string
	}{
		{"-9223372036854775808", 20, `E'-9223372036854775808'::"pg_catalog"."int8"`},
		{"-2147483648", 23, `E'-2147483648'::"pg_catalog"."int4"`},
		{"-32768", 21, `E'-32768'::"pg_catalog"."int2"`},
		{"32767::smallint", 21, `E'32767'::"pg_catalog"."int2"`},
		{"TRUE", 16, `TRUE::"pg_catalog"."bool"`},
		{"'quoted''value'::text", 25, `E'quoted''value'::"pg_catalog"."text"`},
		{"'1.25'::real", 700, `E'1.25'::"pg_catalog"."float4"`},
		{"NULL::bytea", 17, `NULL::"pg_catalog"."bytea"`},
	} {
		ts, err := fullTokens(tt.source)
		if err != nil {
			t.Fatal(err)
		}
		got, err := fullLiteral(ts, tt.oid)
		if err != nil || got != tt.want {
			t.Errorf("%s => %s (%v)", tt.source, got, err)
		}
	}
	for _, tt := range []struct {
		source string
		oid    uint32
	}{{"32768", 21}, {"2147483648", 23}, {"'NaN'::real", 700}, {"1e-999", 700}, {"true", 23}, {"'x'::public.text", 25}, {`'x'::"real"`, 700}} {
		ts, err := fullTokens(tt.source)
		if err == nil {
			_, err = fullLiteral(ts, tt.oid)
		}
		if err == nil {
			t.Error("unsupported literal accepted", tt)
		}
	}
	c, o, e := fullFixture(t)
	e.Constraints[2].Expression = `flags "is" "null"`
	assertFullBlocked(t, generateFull(t, c, o, e), "full_constraint_31002")
}

// Remove the mandatory internal ownership links from both inputs, so rejection
// cannot rely on the supplement/base union mismatch alone.
func TestFullTableRequiresRowTypeDependencies(t *testing.T) {
	for _, oid := range []uint32{30001, 30002, 0} {
		t.Run(fmt.Sprint(oid), func(t *testing.T) {
			c, o, e := fullFixture(t)
			keep := func(ds []Dependency) []Dependency {
				out := []Dependency{}
				for _, d := range ds {
					if d.Kind != "row_type" || oid != 0 && d.OID != oid {
						out = append(out, d)
					}
				}
				return out
			}
			c.Dependencies = keep(c.Dependencies)
			e.Dependencies = keep(e.Dependencies)
			rebindFull(t, c, &e)
			assertFullBlocked(t, generateFull(t, c, o, e), "full_row_type_dependency")
		})
	}
}

func TestFullTableRequiresCapturedObjectDependencies(t *testing.T) {
	c, o, e := fullFixture(t)
	for _, omitted := range e.Dependencies {
		if omitted.Kind != "namespace" && omitted.Kind != "legacy_implementation" && !(omitted.Kind == "operator_family" && omitted.ReferencedClass == "pg_am") {
			continue
		}
		t.Run(fmt.Sprintf("%s_%d_%s", omitted.Class, omitted.OID, omitted.Kind), func(t *testing.T) {
			keep := func(ds []Dependency) []Dependency {
				out := []Dependency{}
				for _, d := range ds {
					if d != omitted {
						out = append(out, d)
					}
				}
				return out
			}
			cc, ee := c, e
			cc.Dependencies, ee.Dependencies = keep(c.Dependencies), keep(e.Dependencies)
			rebindFull(t, cc, &ee)
			assertFullBlocked(t, generateFull(t, cc, o, ee), fmt.Sprintf("full_object_dependency_%d", omitted.OID))
		})
	}
}
