package postgres

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

var fullCoverage = []string{"table_properties", "column_properties", "defaults", "constraints", "indexes", "sequences", "roles", "privileges", "creation_privileges", "dependencies", "triggers", "rules", "views", "functions", "policies", "publications", "security_labels", "inheritance", "statistics", "comments", "internal_storage", "legacy_operator_families", "namespaces", "event_triggers"}
var fullScalarTypes = map[uint32]string{16: "bool", 17: "bytea", 20: "int8", 21: "int2", 23: "int4", 25: "text", 700: "float4", 701: "float8", 1021: "_float4"}
var fullBtreeClasses = map[uint32]string{16: "bool_ops", 17: "bytea_ops", 20: "int8_ops", 21: "int2_ops", 23: "int4_ops", 25: "text_ops", 700: "float4_ops", 701: "float8_ops"}

func (f *fullTableModel) gate(code, requirement string) { f.m.block("full_"+code, requirement) }
func (f *fullTableModel) objectGate(kind string, oid uint32, requirement string) {
	f.gate(fmt.Sprintf("%s_%d", kind, oid), requirement)
}
func (f *fullTableModel) validate() error {
	m, e := f.m, &f.e
	f.roles = map[uint32]string{}
	f.namespaces = map[string]uint32{}
	namespaceOIDs := map[uint32]bool{}
	for _, n := range e.Namespaces {
		if n.OID == 0 || !transitionName(n.Name) || f.namespaces[n.Name] != 0 || namespaceOIDs[n.OID] {
			return fmt.Errorf("full-table: invalid/duplicate namespace identity")
		}
		if n.Name == "pg_catalog" && n.OID != 11 || n.Name == "pg_toast" && n.OID != 99 || n.OID == 11 && n.Name != "pg_catalog" || n.OID == 99 && n.Name != "pg_toast" {
			f.gate("namespace_identity", "Built-in PG16 namespace names and OIDs must agree exactly.")
		}
		f.namespaces[n.Name], namespaceOIDs[n.OID] = n.OID, true
	}
	if f.namespaces[m.table.Schema] != e.Table.NamespaceOID || f.namespaces[m.old.Schema] == 0 || strings.HasPrefix(m.table.Schema, "pg_") || strings.HasPrefix(m.old.Schema, "pg_") {
		f.gate("namespace_identity", "Capture exact source-table and legacy-extension namespace OID/name pairs; system namespaces are outside this profile.")
	}
	f.cols = map[int]Column{}
	f.facts = map[int]FullColumnFacts{}
	f.defaults = map[int]string{}
	f.constraints = map[uint32]FullConstraint{}
	f.checks = map[uint32]string{}
	f.indexes = map[uint32]Index{}
	f.indexFacts = map[uint32]FullIndexFacts{}
	f.sequences = map[uint32]FullSequence{}
	f.archive = map[uint32]string{}
	if !e.StandardConformingStrings || e.DeparseSearchPath != "pg_catalog" {
		f.gate("lexical_profile", "Captured pg_get_expr literals require standard_conforming_strings=on; other lexical profiles are not implemented.")
	}
	if e.FormatVersion != FullTableEvidenceVersion || e.Collector != FullTableCollector || e.SourceProfile != SourceProfile || e.TargetProfile != TargetProfile {
		f.gate("evidence_profile", "Supply the exact full-table v1 PostgreSQL 16 supplement and pinned source/target profiles.")
	}
	if e.CatalogSHA256 != m.r.CatalogSHA256 || e.ExtractedAt != m.c.ExtractedAt || e.ServerVersionNum != m.c.ServerVersionNum {
		f.gate("evidence_binding", "Supplement hash, UTC extraction timestamp and server version must exactly match the complete catalog snapshot; recollect together under quiescence.")
	}
	if m.c.ServerVersionNum < 160000 || m.c.ServerVersionNum >= 170000 {
		f.gate("server_profile", "Whole-table recreation currently covers PostgreSQL 16 catalog/SQL semantics only. Other versions require a separately implemented profile.")
	}
	if e.Table.RelationOID != m.table.OID || e.Table.Schema != m.table.Schema || e.Table.Name != m.table.Name || e.Table.NamespaceOID == 0 || e.Table.RowTypeOID == 0 || e.Table.RowArrayTypeOID == 0 || e.Table.RowTypeOID == e.Table.RowArrayTypeOID {
		f.gate("table_identity", "Supply matching relation/schema/name, namespace and distinct row/row-array type OIDs.")
	}
	if e.Table.AccessMethodOID != 2 || e.Table.AccessMethod != "heap" || e.Table.ForceRLS || e.Table.ReplicaIdentity != "d" || len(e.Table.Options) != 0 || m.table.Kind != "r" || m.table.Persistence != "p" || m.table.RLS || len(m.table.Parents) != 0 {
		f.gate("table_contract", "Only ordinary permanent heap, no RLS/inheritance/reloptions and default replica identity are implemented; preserve any other table contract in a separate plan.")
	}
	if !fullTablespace(e.Table.TablespaceOID, e.Table.Tablespace) {
		f.gate("tablespace", "Only positively identified pg_default tablespace (OID 1663) is currently supported.")
	}
	cov := map[string]bool{}
	for _, c := range e.Coverage {
		if cov[c.Category] {
			return fmt.Errorf("full-table: duplicate coverage category")
		}
		cov[c.Category] = true
		if !c.Complete {
			f.gate("coverage_"+safeGateCategory(c.Category), "A catalog category is explicitly incomplete; collect an exhaustive visible inventory before proposing a table swap.")
		}
	}
	for _, category := range fullCoverage {
		found := false
		for _, c := range e.Coverage {
			if c.Category == category && c.Complete {
				found = true
			}
		}
		if !found {
			f.gate("coverage_"+category, "Supply a complete "+category+" inventory, including explicit absence when empty. Missing evidence is never absence.")
		}
	}
	names := map[string]bool{}
	for _, r := range e.Roles {
		if r.OID == 0 || !transitionName(r.Name) || f.roles[r.OID] != "" || names[r.Name] {
			return fmt.Errorf("full-table: invalid/duplicate role identity")
		}
		f.roles[r.OID] = r.Name
		names[r.Name] = true
	}
	if f.roles[e.Table.OwnerOID] == "" {
		f.gate("owner_identity", "Resolve the table owner OID to an exact role; ownership is not inferred from the operator.")
	}
	for _, a := range m.table.Columns {
		f.cols[a.AttNum] = a
	}
	for _, a := range e.Columns {
		if f.facts[a.AttNum].AttNum != 0 || f.cols[a.AttNum].AttNum == 0 {
			return fmt.Errorf("full-table: duplicate/unknown column facts")
		}
		f.facts[a.AttNum] = a
	}
	types := map[uint32]Type{}
	for _, t := range m.c.Types {
		types[t.OID] = t
	}
	rowType, arrayType := types[e.Table.RowTypeOID], types[e.Table.RowArrayTypeOID]
	if rowType.OID == 0 || rowType.Schema != m.table.Schema || rowType.Name != m.table.Name || rowType.Kind != "c" || rowType.BaseOID != 0 || rowType.ElementOID != 0 || arrayType.OID == 0 || arrayType.Schema != m.table.Schema || arrayType.Kind != "b" || arrayType.ElementOID != rowType.OID || arrayType.BaseOID != 0 {
		f.gate("row_type_identity", "Capture matching source composite and array type identities in the base catalog; their exact OIDs must match the supplement.")
	}
	for pos, a := range f.sortedColumns() {
		if a.AttNum != pos+1 {
			f.gate("attribute_order", "Source attribute numbers must be contiguous without dropped/omitted columns; physical identity is not renumbered silently.")
		}
		facts, ok := f.facts[a.AttNum]
		if !ok {
			f.objectGate("column", uint32(a.AttNum), "Missing per-column storage/collation/statistics/comment evidence.")
			continue
		}
		t := types[a.TypeOID]
		expected := fullScalarTypes[a.TypeOID]
		if !transitionName(a.Name) {
			return fmt.Errorf("full-table: unsupported column identifier")
		}
		if a.Dropped || a.Generated != "" || a.Identity != "" || a.TypeMod != -1 || expected == "" || t.Schema != "pg_catalog" || t.Name != expected || t.Kind != "b" || t.BaseOID != 0 || a.TypeOID != 1021 && t.ElementOID != 0 || a.TypeOID == 1021 && t.ElementOID != 700 {
			f.objectGate("column", uint32(a.AttNum), "Unsupported or unbound source column type/typmod, dropped/generated/identity attribute; no column is silently omitted.")
		}
		if a.TypeOID != 1021 && a.TypeOID != 25 && a.TypeOID != 17 && (facts.Storage != "p" || facts.Compression != "") {
			f.objectGate("column_storage", uint32(a.AttNum), "Fixed-width scalar columns require PLAIN storage and no compression.")
		}
		if a.TypeOID != 1021 && a.ArrayDimensions != 0 {
			f.objectGate("column", uint32(a.AttNum), "Array dimensions conflict with the scalar source type.")
		}
		if facts.Statistics < -1 || facts.Statistics > 10000 || len(facts.Options) != 0 || !strings.Contains("pexm", facts.Storage) || len(facts.Storage) != 1 || facts.Compression != "" && facts.Compression != "p" && facts.Compression != "l" {
			f.objectGate("column_storage", uint32(a.AttNum), "Unknown storage, compression, statistics target or per-attribute options require an implemented preservation path.")
		}
		if a.TypeOID == 25 {
			if facts.CollationOID != 100 || facts.CollationSchema != "pg_catalog" || facts.CollationName != "default" {
				f.objectGate("collation", uint32(a.AttNum), "Only text using the existing database pg_catalog.default collation (OID 100) is implemented; custom collation evidence needs a separate profile.")
			}
		} else if facts.CollationOID != 0 || facts.CollationSchema != "" || facts.CollationName != "" {
			f.objectGate("collation", uint32(a.AttNum), "Non-text columns in this profile must have no collation.")
		}
		if a.AttNum == m.column.AttNum && a.Default != "" {
			f.objectGate("vector_default", uint32(a.AttNum), "Converting a selected real[] default changes its type semantics and requires a separate supported expression mapping.")
		}
	}
	for _, s := range e.Sequences {
		if s.OID == 0 || f.sequences[s.OID].OID != 0 || !transitionName(s.Schema) || !transitionName(s.Name) {
			return fmt.Errorf("full-table: invalid/duplicate sequence identity")
		}
		f.sequences[s.OID] = s
		col := f.cols[s.OwnedAttNum]
		if s.OwnerOID != e.Table.OwnerOID || s.Schema != m.table.Schema || s.DependencyType != "a" || col.AttNum == 0 || (col.TypeOID != 20 && col.TypeOID != 23 && col.TypeOID != 21) || s.OwnedAttNum == m.column.AttNum {
			f.objectGate("sequence", s.OID, "Only same-schema/same-owner serial-style OWNED BY integer sequences are supported; identity/shared/external sequence contracts require review.")
		}
		rel := Relation{}
		for _, r := range m.c.Relations {
			if r.OID == s.OID {
				rel = r
			}
		}
		if rel.OID == 0 || rel.Schema != s.Schema || rel.Name != s.Name || rel.Kind != "S" || rel.Persistence != "p" {
			f.objectGate("sequence_identity", s.OID, "Supply the original permanent sequence relation OID/name in the base catalog.")
		}
	}
	defaultIDs := map[uint32]bool{}
	for _, d := range e.Defaults {
		if d.OID == 0 || defaultIDs[d.OID] || f.defaults[d.AttNum] != "" || f.cols[d.AttNum].AttNum == 0 {
			return fmt.Errorf("full-table: invalid/duplicate default identity")
		}
		defaultIDs[d.OID] = true
		a := f.cols[d.AttNum]
		if d.Expression != a.Default || d.Expression == "" {
			f.objectGate("default", d.OID, "Default expression differs from its identity-bound base column capture.")
			continue
		}
		sql, err := f.defaultSQL(d, a)
		if err != nil {
			f.objectGate("default", d.OID, "Default is outside the typed literal or verified owned-sequence nextval subset; arbitrary expressions are never copied as SQL.")
			continue
		}
		f.defaults[d.AttNum] = sql
	}
	for _, a := range m.table.Columns {
		if a.Default != "" && f.defaults[a.AttNum] == "" {
			f.objectGate("column_default", uint32(a.AttNum), "Supply a supported captured default object; an omitted default cannot be treated as absent.")
		}
	}
	for _, s := range e.Sequences {
		n := 0
		for _, d := range e.Defaults {
			if d.SequenceOID == s.OID && d.AttNum == s.OwnedAttNum {
				n++
			}
		}
		if n != 1 {
			f.objectGate("sequence_default", s.OID, "Owned sequence requires exactly one matching source-column nextval default; other consumers/unused sequences are outside this profile.")
		}
	}
	for _, x := range m.c.Indexes {
		if x.TableOID == m.table.OID {
			f.indexes[x.OID] = x
		}
	}
	for _, x := range e.Indexes {
		if x.OID == 0 || f.indexFacts[x.OID].OID != 0 || f.indexes[x.OID].OID == 0 {
			return fmt.Errorf("full-table: invalid/duplicate index facts")
		}
		f.indexFacts[x.OID] = x
	}
	for _, x := range e.Constraints {
		if x.OID == 0 || f.constraints[x.OID].OID != 0 || !transitionName(x.Name) {
			return fmt.Errorf("full-table: invalid/duplicate constraint identity")
		}
		f.constraints[x.OID] = x
	}
	if err := f.validateIndexes(); err != nil {
		return err
	}
	if err := f.validateConstraints(); err != nil {
		return err
	}
	if err := f.validateNames(); err != nil {
		return err
	}
	if err := f.validateGrants(); err != nil {
		return err
	}
	for _, x := range e.UnsupportedObjects {
		f.objectGate("unsupported_object", x.OID, "Captured "+safeGateCategory(x.Kind)+" object requires an object-specific recreation/rebinding implementation. See its address and exact definition in full-table-evidence.json.")
	}
	if err := f.validateLegacyFamilies(); err != nil {
		return err
	}
	f.validateToast()
	if err := f.validateDependencies(); err != nil {
		return err
	}
	// Remove only the generic dependency gate: each edge was reclassified above,
	// and unsupported edges have their own specific blocked gate. All other source
	// inspection/capability gates remain unchanged and inspection evidence is intact.
	gates := m.r.Gates[:0]
	for _, g := range m.r.Gates {
		if g.Code != "dependent_object_requires_manual_plan" {
			gates = append(gates, g)
		}
	}
	m.r.Gates = gates
	m.r.Status = "review_required"
	for _, g := range gates {
		if g.State == "blocked" {
			m.r.Status = "blocked"
		}
	}
	for i := range m.r.Gates {
		switch m.r.Gates[i].Code {
		case "application_cutover":
			m.r.Gates[i].Requirement = "Approve new relation OID, original-name typed columns, vector parameter/operator/settings changes and retire all cached/prepared OID bindings. Backup arrays become stale after later writes; reconcile before any reverse migration."
		case "cleanup_separate":
			m.r.Gates[i].Requirement = "Retain the backup table, its ordinary indexes and immutable export through an approved rollback window. Its owned sequence defaults are removed and sequence ownership is rebound to the new table. No cleanup/reverse DDL is generated."
		}
	}
	m.r.Options = f.o.Transition
	m.o = f.o.Transition
	m.r.Unsupported = []string{
		"Full-table copy/swap is limited to the explicit PostgreSQL 16 ordinary-heap catalog profile; unsupported/missing contracts block all mutation proposals.",
		"No online/batched full-table migration, arbitrary expressions, views/functions/triggers/FKs/RLS/partition/publication/security-label recreation, or post-cutover reverse migration is implemented.",
		"No PostgreSQL/Neon connection, execution, credentials, live qualification, cleanup or CASCADE is provided. Every operational approval stays unresolved.",
	}
	m.r.Gates = append(m.r.Gates, TransitionGate{"full_table_rebinding", "unresolved", "Approve the new relation OID, explicit copy/backup/archive names, all recreated constraints/indexes/ACLs, owned-sequence rebinding and prepared-statement/application reconnect. Names alone do not rebind old OIDs; prove no uncaptured external dependencies."}, TransitionGate{"full_table_acl_and_roles", "unresolved", "Verify role identity/membership, same-owner creation privileges, complete current default ACLs, storage headroom and backup access. Review table/column ACL reconstruction and retained sequence access. No permissions have been changed."})
	sort.Slice(m.r.Gates, func(i, j int) bool { return m.r.Gates[i].Code < m.r.Gates[j].Code })
	identity, _ := json.Marshal(struct {
		Catalog, Evidence string
		Options           FullTableOptions
	}{m.r.CatalogSHA256, f.evidenceHash, f.o})
	m.r.PlanSHA256 = sqlHash(string(identity))
	return nil
}
func safeGateCategory(s string) string {
	if len(s) > 80 {
		return "unrecognized"
	}
	for _, r := range s {
		if !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_') {
			return "unrecognized"
		}
	}
	return s
}
func fullTablespace(oid uint32, name string) bool { return oid == 1663 && name == "pg_default" }
func (f *fullTableModel) validateIndexes() error {
	methods := map[uint32]AccessMethod{}
	classes := map[uint32]OpClass{}
	for _, x := range f.m.c.AccessMethods {
		methods[x.OID] = x
	}
	for _, x := range f.m.c.OpClasses {
		classes[x.OID] = x
	}
	ann := map[uint32]bool{}
	for _, x := range f.m.indexes {
		ann[x.source.OID] = true
	}
	for _, x := range f.m.c.Indexes {
		if x.TableOID != f.m.table.OID {
			continue
		}
		fact, ok := f.indexFacts[x.OID]
		if !ok {
			f.objectGate("index", x.OID, "Missing full index facts (uniqueness, constraint ownership, ordering, INCLUDE, tablespace and replica/cluster identity).")
			continue
		}
		if !transitionName(x.Name) || !transitionName(x.Schema) {
			return fmt.Errorf("full-table: invalid index identifier")
		}
		if x.Schema != f.m.table.Schema {
			f.objectGate("index_schema", x.OID, "Source index namespace must be the same as its owning table; collect consistent PostgreSQL index identity facts.")
		}
		if !fact.Immediate || !fact.Live || !x.Valid || !x.Ready || x.Expression != "" || x.Predicate != "" || len(fact.Included) != 0 || fact.ReplicaIdentity || fact.Clustered || !fullTablespace(fact.TablespaceOID, fact.Tablespace) || len(fact.Keys) != len(x.Keys) {
			f.objectGate("index_shape", x.OID, "Expression/partial/INCLUDE/replica-identity/clustered/invalid indexes or unsupported tablespaces are not recreated.")
		}
		if ann[x.OID] {
			for i, k := range fact.Keys {
				if i >= len(x.Keys) || k.AttNum != x.Keys[i] || k.Descending || k.NullsFirst {
					f.objectGate("ann_key", x.OID, "Legacy ANN key/order facts conflict with the verified source index.")
				}
			}
			if fact.Unique || fact.Primary || fact.ConstraintOID != 0 || fact.NullsNotDistinct {
				f.objectGate("ann_contract", x.OID, "Legacy ANN index has unsupported uniqueness/constraint properties.")
			}
			continue
		}
		method := methods[x.MethodOID]
		handlerOK := false
		for _, fn := range f.m.c.Functions {
			if fn.OID == method.HandlerOID && fn.Schema == "pg_catalog" && fn.Name == "bthandler" && fn.ExtensionOID == 0 && reflect.DeepEqual(fn.ArgumentOIDs, []uint32{2281}) && fn.ReturnOID == 325 {
				handlerOK = true
			}
		}
		if !handlerOK || method.OID != 403 || method.Name != "btree" || method.ExtensionOID != 0 || len(x.Options) != 0 {
			f.objectGate("index_method", x.OID, "Only ordinary built-in btree without storage options is implemented in addition to explicitly mapped legacy ANN.")
		}
		for i, k := range fact.Keys {
			if i >= len(x.Keys) {
				break
			}
			a := f.cols[k.AttNum]
			oc := classes[x.OpClasses[i]]
			if k.AttNum != x.Keys[i] || a.AttNum == 0 || k.AttNum == f.m.column.AttNum || fullBtreeClasses[a.TypeOID] == "" || oc.Schema != "pg_catalog" || oc.Name != fullBtreeClasses[a.TypeOID] || oc.InputOID != a.TypeOID || oc.ExtensionOID != 0 || oc.MethodOID != method.OID || !oc.Default || x.Collations[i] != f.facts[a.AttNum].CollationOID {
				f.objectGate("index_key", x.OID, "Index keys/opclasses/collations do not match the unchanged supported scalar columns and built-in default btree profile.")
			}
		}
		if fact.ConstraintOID != 0 {
			con, ok := f.constraints[fact.ConstraintOID]
			if !ok || con.IndexOID != x.OID || (con.Kind != "p" && con.Kind != "u") || !fact.Unique || fact.Primary != (con.Kind == "p") || x.Name != con.Name {
				f.objectGate("index_constraint", x.OID, "Constraint/index ownership, name and unique/primary flags must agree exactly.")
			}
		} else if fact.Primary {
			f.objectGate("index_constraint", x.OID, "Primary index requires captured owning constraint.")
		}
		if fact.NullsNotDistinct && !fact.Unique {
			f.objectGate("index_nulls", x.OID, "NULLS NOT DISTINCT requires a unique index.")
		}
		if fact.NullsNotDistinct && fact.Primary {
			f.objectGate("index_nulls", x.OID, "PostgreSQL primary-key indexes cannot use NULLS NOT DISTINCT; collect consistent index facts.")
		}
	}
	return nil
}
func (f *fullTableModel) validateConstraints() error {
	names := map[string]bool{}
	pk := 0
	for _, c := range f.e.Constraints {
		if names[c.Name] {
			return fmt.Errorf("full-table: duplicate constraint name")
		}
		names[c.Name] = true
		seen := map[int]bool{}
		for _, a := range c.Columns {
			if f.cols[a].AttNum == 0 || seen[a] || a == f.m.column.AttNum {
				f.objectGate("constraint", c.OID, "Constraint has missing/duplicate/converted-column references; vector constraint rewrite is not guessed.")
			}
			seen[a] = true
		}
		if c.Deferrable || c.InitiallyDeferred {
			f.objectGate("constraint", c.OID, "Deferrable constraints require captured internal constraint-trigger enable/function semantics; this profile currently supports only immediate constraints.")
		}
		if c.InitiallyDeferred && !c.Deferrable {
			f.objectGate("constraint", c.OID, "Initially deferred constraint must be deferrable.")
		}
		switch c.Kind {
		case "p", "u":
			if c.Kind == "p" {
				pk++
				if !reflect.DeepEqual(c.Columns, f.m.table.PrimaryKey) {
					f.objectGate("primary_key", c.OID, "Primary key column identities differ from base catalog.")
				}
			}
			idx, ok := f.indexes[c.IndexOID]
			fact := f.indexFacts[c.IndexOID]
			if !ok || fact.ConstraintOID != c.OID || !reflect.DeepEqual(idx.Keys, c.Columns) || !c.Validated || !c.NoInherit || c.Expression != "" || len(c.Columns) == 0 {
				f.objectGate("constraint", c.OID, "Standalone PostgreSQL primary/unique constraint requires matching captured backing index, validated keys, connoinherit=true and no unexplained expression.")
			}
			for _, k := range fact.Keys {
				if k.Descending || k.NullsFirst {
					f.objectGate("constraint", c.OID, "Constraint backing indexes require default ascending/NULLS LAST ordering in this profile.")
				}
			}
		case "c":
			if c.IndexOID != 0 || c.Deferrable || c.InitiallyDeferred || len(c.Columns) != 1 {
				f.objectGate("constraint", c.OID, "CHECK support requires one unchanged scalar column, no backing index and no deferrability.")
				continue
			}
			sql, err := f.checkSQL(c)
			if err != nil {
				f.objectGate("constraint", c.OID, "CHECK expression is outside one typed scalar comparison or IS [NOT] NULL; arbitrary SQL bodies are never interpolated.")
			} else {
				f.checks[c.OID] = sql
			}
		default:
			f.objectGate("constraint", c.OID, "Constraint kind is not implemented. Foreign/exclusion constraints need separate dependency/trigger/referenced-key recreation evidence.")
		}
	}
	if pk != 1 {
		f.gate("primary_key_inventory", "Supply exactly one matching primary-key constraint and its complete backing-index facts.")
	}
	return nil
}
func (f *fullTableModel) validateNames() error {
	used := map[string]bool{}
	constraintNames := map[string]bool{}
	for _, c := range f.e.Constraints {
		constraintNames[c.Name] = true
	}
	for _, r := range f.m.c.Relations {
		if r.Schema == f.m.table.Schema {
			used[r.Name] = true
		}
	}
	for _, x := range f.m.c.Indexes {
		if x.Schema == f.m.table.Schema {
			used[x.Name] = true
		}
	}
	for _, t := range f.m.c.Types {
		if t.Schema == f.m.table.Schema {
			used[t.Name] = true
		}
	}
	generated := map[string]bool{}
	reserve := func(name string) bool {
		if !transitionName(name) || used[name] || generated[name] {
			return false
		}
		generated[name] = true
		return true
	}
	if !reserve(f.o.CopyTable) || !reserve(f.o.BackupTable) {
		f.gate("name_collision", "Copy/backup table or row-type name collides with a supplied relation/index/type; choose distinct unoccupied names and recheck live under quiescence.")
	}
	for _, x := range f.m.indexes {
		if generated[x.target.Name] {
			f.gate("name_collision", "ANN and copy/backup/archive names collide.")
		}
		generated[x.target.Name] = true
	}
	ann := map[uint32]bool{}
	for _, x := range f.m.indexes {
		ann[x.source.OID] = true
	}
	for _, a := range f.o.ArchiveNames {
		if a.IndexOID == 0 || f.archive[a.IndexOID] != "" || !transitionName(a.Name) {
			return fmt.Errorf("full-table: invalid/duplicate archive index mapping")
		}
		f.archive[a.IndexOID] = a.Name
		if f.indexes[a.IndexOID].OID == 0 || ann[a.IndexOID] {
			return fmt.Errorf("full-table: archive mapping must name an ordinary source index")
		}
		if !reserve(a.Name) {
			f.gate("name_collision", "Archive index name collides with an existing/generated schema object.")
		}
		if f.indexFacts[a.IndexOID].ConstraintOID != 0 && constraintNames[a.Name] {
			f.gate("name_collision", "Renaming a constraint-backed archive index also renames its owning constraint; its archive name collides with an existing source-table constraint.")
		}
	}
	for oid := range f.indexes {
		if !ann[oid] && f.archive[oid] == "" {
			f.objectGate("archive_name", oid, "Explicit noncolliding archive name is required to free the source ordinary index/constraint name without dropping its retained backup index.")
		}
	}
	return nil
}
func (f *fullTableModel) validateGrants() error {
	for _, list := range [][]FullGrant{f.e.Privileges, f.e.CreationPrivileges} {
		seen := map[string]bool{}
		for _, g := range list {
			key := fmt.Sprintf("%d/%d/%d/%s", g.GrantorOID, g.GranteeOID, g.AttNum, g.Privilege)
			if seen[key] {
				return fmt.Errorf("full-table: duplicate ACL entry")
			}
			seen[key] = true
			if g.GrantorOID != f.e.Table.OwnerOID || f.roles[g.GrantorOID] == "" || g.GranteeOID != 0 && f.roles[g.GranteeOID] == "" || g.AttNum != 0 && f.cols[g.AttNum].AttNum == 0 || g.GranteeOID == 0 && g.GrantOption {
				f.gate("acl_contract", "ACLs need exact owner-origin grantor, resolved grantee or PUBLIC, valid column and non-PUBLIC grant-option identity; other grant chains are gated.")
			}
			allowed := map[string]bool{"SELECT": true, "INSERT": true, "UPDATE": true, "REFERENCES": true}
			if g.AttNum == 0 {
				allowed["DELETE"] = true
				allowed["TRUNCATE"] = true
				allowed["TRIGGER"] = true
			}
			if !allowed[g.Privilege] {
				f.gate("acl_privilege", "Unsupported privilege for the captured object level/PostgreSQL 16 profile.")
			}
		}
	}
	for _, g := range f.e.CreationPrivileges {
		if g.AttNum != 0 {
			f.gate("creation_acl", "New-table default privileges must describe table ACLs, not hypothetical column grants.")
		}
	}
	return nil
}

// PostgreSQL creates new internal TOAST storage for the new heap; it does not
// rebind old physical chunks. Explicit identity/dependency evidence distinguishes
// those internal objects from user dependents. Nondefault storage options gate.
func (f *fullTableModel) validateToast() {
	t := f.e.Table
	if t.ToastRelationOID == 0 {
		if t.ToastIndexOID != 0 || t.ToastOwnerOID != 0 || len(t.ToastOptions) != 0 {
			f.gate("toast_identity", "Inconsistent explicit absence of TOAST storage.")
		}
		return
	}
	if t.ToastIndexOID == 0 || t.ToastOwnerOID != t.OwnerOID || len(t.ToastOptions) != 0 {
		f.gate("toast_contract", "TOAST ownership/options require the same owner and default options in this profile.")
	}
	rel := Relation{}
	idx := Index{}
	for _, r := range f.m.c.Relations {
		if r.OID == t.ToastRelationOID {
			rel = r
		}
	}
	for _, x := range f.m.c.Indexes {
		if x.OID == t.ToastIndexOID {
			idx = x
		}
		if x.TableOID == t.ToastRelationOID && x.OID != t.ToastIndexOID {
			f.gate("toast_contract", "Additional TOAST indexes require an implemented physical-storage profile.")
		}
	}
	if rel.OID == 0 || rel.Schema != "pg_toast" || rel.Kind != "t" || rel.Persistence != "p" || rel.RLS || len(rel.Parents) != 0 || len(rel.Columns) != 3 || !transitionName(rel.Name) {
		f.gate("toast_identity", "Supply the internal permanent TOAST relation identity and three-column catalog shape.")
	}
	names := []string{"chunk_id", "chunk_seq", "chunk_data"}
	types := []uint32{26, 23, 17}
	for i, a := range rel.Columns {
		if i >= 3 {
			break
		}
		if a.AttNum != i+1 || a.Name != names[i] || a.TypeOID != types[i] || a.TypeMod != -1 || a.Dropped || a.Default != "" || a.Generated != "" || a.Identity != "" {
			f.gate("toast_contract", "Nonstandard internal TOAST columns are outside the recreated heap storage profile.")
		}
	}
	if idx.OID == 0 || idx.Schema != "pg_toast" || !transitionName(idx.Name) || idx.TableOID != t.ToastRelationOID || idx.MethodOID != 403 || !reflect.DeepEqual(idx.Keys, []int{1, 2}) || !reflect.DeepEqual(idx.Collations, []uint32{0, 0}) || len(idx.Options) != 0 || idx.Expression != "" || idx.Predicate != "" || !idx.Valid || !idx.Ready {
		f.gate("toast_index", "Supply the original valid/ready ordinary two-key internal TOAST btree index.")
	}
	classes := map[uint32]OpClass{}
	for _, x := range f.m.c.OpClasses {
		classes[x.OID] = x
	}
	if len(idx.OpClasses) == 2 {
		for i, name := range []string{"oid_ops", "int4_ops"} {
			c := classes[idx.OpClasses[i]]
			if c.Schema != "pg_catalog" || c.Name != name || c.MethodOID != 403 || c.InputOID != types[i] || c.ExtensionOID != 0 || !c.Default {
				f.gate("toast_index", "Internal TOAST opclass/type identity differs from the built-in PG16 profile.")
			}
		}
	}
}

func (f *fullTableModel) validateLegacyFamilies() error {
	classes := map[uint32]OpClass{}
	for _, c := range f.m.c.OpClasses {
		classes[c.OID] = c
	}
	seen, ops, procs, ownedClasses := map[uint32]bool{}, map[uint32]bool{}, map[uint32]bool{}, map[uint32]bool{}
	for _, x := range f.e.LegacyFamilies {
		if x.OID == 0 || x.AMOpOID == 0 || x.AMProcOID == 0 || seen[x.OID] || ops[x.AMOpOID] || procs[x.AMProcOID] || ownedClasses[x.OpClassOID] {
			return fmt.Errorf("full-table: invalid/duplicate legacy family object addresses")
		}
		seen[x.OID], ops[x.AMOpOID], procs[x.AMProcOID], ownedClasses[x.OpClassOID] = true, true, true, true
		c, ok := classes[x.OpClassOID]
		_, known := legacyClasses[c.Name]
		methodOK := false
		for _, m := range f.m.c.AccessMethods {
			if m.OID == x.MethodOID && m.Name == "hnsw" && m.ExtensionOID == f.m.old.OID {
				methodOK = true
			}
		}
		if !ok || !known || !methodOK || c.Schema != x.Schema || c.ExtensionOID != f.m.old.OID || x.Schema != f.m.old.Schema || x.Name != c.Name || x.MethodOID != c.MethodOID || x.Strategy != 1 || x.Support != 1 || x.LeftOID != 1021 || x.RightOID != 1021 || x.OperatorOID != c.OperatorOID || x.FunctionOID != c.FunctionOID || x.SortFamilyOID != 1970 || x.SortFamilySchema != "pg_catalog" || x.SortFamilyName != "float_ops" || x.SortMethodOID != 403 {
			f.objectGate("legacy_family", x.OID, "Legacy family/support-row identities must match the pinned one-strategy/one-function real[] opclass and built-in btree float_ops sort family.")
		}
	}
	var sortOID uint32
	for _, x := range f.e.LegacyFamilies {
		if sortOID == 0 {
			sortOID = x.SortFamilyOID
		}
		if x.SortFamilyOID != sortOID || seen[x.SortFamilyOID] {
			f.objectGate("legacy_family", x.OID, "The built-in float_ops sort family must have one distinct consistent pg_opfamily identity.")
		}
	}
	for _, c := range f.m.c.OpClasses {
		if c.ExtensionOID == f.m.old.OID && !ownedClasses[c.OID] {
			f.objectGate("legacy_family_missing", c.OID, "Capture the implicit legacy operator family and pg_amop/pg_amproc identities; a complete extension inventory cannot omit them.")
		}
	}
	return nil
}
