package postgres

import (
	"encoding/json"
	"fmt"
)

type fullDependencyAddress struct {
	class    string
	oid      uint32
	sub      int
	kind     string
	refClass string
	refOID   uint32
	refSub   int
}

func fullDependencyKey(d Dependency) fullDependencyAddress {
	return fullDependencyAddress{d.Class, d.OID, d.SubID, d.Kind, d.ReferencedClass, d.ReferencedOID, d.ReferencedSubID}
}
func (f *fullTableModel) validateDependencies() error {
	required := f.requiredObjectDependencies()
	requiredKeys := make(map[fullDependencyAddress]bool, len(required))
	for _, d := range required {
		requiredKeys[fullDependencyKey(d)] = true
	}
	// Compare exact captured records, including definitions. The supplement may
	// add full-table edges, but cannot erase or replace any base-catalog edge.
	records := map[string]bool{}
	edgeKeys := map[fullDependencyAddress]bool{}
	for _, d := range f.e.Dependencies {
		if d.OID == 0 || d.ReferencedOID == 0 || d.SubID < 0 || d.ReferencedSubID < 0 {
			return fmt.Errorf("full-table: invalid dependency address")
		}
		key := fullDependencyKey(d)
		if edgeKeys[key] {
			return fmt.Errorf("full-table: duplicate dependency address")
		}
		edgeKeys[key] = true
		b, _ := json.Marshal(d)
		records[string(b)] = true
		if !f.supportedDependency(d, requiredKeys) {
			f.objectGate("dependency", d.OID, "No supported identity-bound rebind/preservation action covers this dependency edge. Inspect its class/OID/sub-ID and referenced address in full-table-evidence.json; do not assume renaming rebinds it.")
		}
	}
	for _, d := range f.m.c.Dependencies {
		b, _ := json.Marshal(d)
		if !records[string(b)] {
			f.objectGate("dependency_missing", d.OID, "A base catalog dependency is missing or changed in the supplement; collect the complete identical union.")
		}
	}
	has := func(class string, oid uint32, kind, refClass string, ref uint32, sub int) bool {
		return edgeKeys[fullDependencyAddress{class, oid, 0, kind, refClass, ref, sub}]
	}
	if !has("pg_type", f.e.Table.RowTypeOID, "row_type", "pg_class", f.m.table.OID, 0) || !has("pg_type", f.e.Table.RowArrayTypeOID, "row_type", "pg_type", f.e.Table.RowTypeOID, 0) {
		f.gate("row_type_dependency", "Missing source row-type-to-table or row-array-to-row-type internal ownership dependency; both captured edges are required.")
	}
	for _, want := range required {
		if !has(want.Class, want.OID, want.Kind, want.ReferencedClass, want.ReferencedOID, 0) {
			f.objectGate("object_dependency", want.OID, "Missing mandatory "+want.Kind+" dependency to "+want.ReferencedClass+"; capture the complete exact source/legacy object dependency inventory.")
		}
	}
	if toast := f.e.Table; toast.ToastRelationOID != 0 {
		if !has("pg_class", toast.ToastRelationOID, "internal_storage", "pg_class", f.m.table.OID, 0) {
			f.gate("toast_dependency", "Missing internal TOAST-to-source relation ownership dependency.")
		}
		for _, a := range []int{1, 2} {
			if !has("pg_class", toast.ToastIndexOID, "internal_storage", "pg_class", toast.ToastRelationOID, a) {
				f.gate("toast_dependency", "Missing internal TOAST-index-to-column dependency.")
			}
		}
	}
	for _, x := range f.e.LegacyFamilies {
		if !has("pg_opfamily", x.OID, "extension_member", "pg_extension", f.m.old.OID, 0) || !has("pg_opclass", x.OpClassOID, "operator_family", "pg_opfamily", x.OID, 0) {
			f.objectGate("legacy_family_dependency", x.OID, "Missing exact family extension ownership or opclass-to-family edge.")
		}
		for _, obj := range []struct {
			class string
			oid   uint32
		}{{"pg_amop", x.AMOpOID}, {"pg_amproc", x.AMProcOID}} {
			if !has(obj.class, obj.oid, "operator_family", "pg_opclass", x.OpClassOID, 0) {
				f.objectGate("legacy_family_dependency", obj.oid, "Missing internal support-row-to-opclass ownership edge.")
			}
		}
		if !has("pg_amop", x.AMOpOID, "operator_family", "pg_operator", x.OperatorOID, 0) || !has("pg_amproc", x.AMProcOID, "operator_family", "pg_proc", x.FunctionOID, 0) {
			f.objectGate("legacy_family_dependency", x.OID, "Missing support-row implementation operator/function dependency.")
		}
	}
	for _, d := range f.e.Defaults {
		if !has("pg_attrdef", d.OID, "default", "pg_class", f.m.table.OID, d.AttNum) {
			f.objectGate("default_dependency", d.OID, "Missing default-to-source-column dependency address.")
		}
		if d.SequenceOID != 0 && !has("pg_attrdef", d.OID, "default", "pg_class", d.SequenceOID, 0) {
			f.objectGate("default_sequence_dependency", d.OID, "Missing nextval-default-to-sequence dependency address.")
		}
	}
	for _, s := range f.e.Sequences {
		if !has("pg_class", s.OID, "sequence", "pg_class", f.m.table.OID, s.OwnedAttNum) {
			f.objectGate("sequence_ownership", s.OID, "Missing captured sequence OWNED BY source-column dependency.")
		}
	}
	for _, c := range f.e.Constraints {
		for _, a := range c.Columns {
			if !has("pg_constraint", c.OID, "constraint", "pg_class", f.m.table.OID, a) {
				f.objectGate("constraint_dependency", c.OID, "Missing constraint-to-source-column dependency evidence.")
			}
		}
	}
	for _, x := range f.e.Indexes {
		if x.ConstraintOID != 0 {
			if !has("pg_class", x.OID, "index", "pg_constraint", x.ConstraintOID, 0) {
				f.objectGate("index_dependency", x.OID, "Missing constraint-owned index dependency evidence.")
			}
		} else {
			for _, k := range x.Keys {
				if !has("pg_class", x.OID, "index", "pg_class", f.m.table.OID, k.AttNum) {
					f.objectGate("index_dependency", x.OID, "Missing index-to-source-column dependency evidence.")
				}
			}
		}
	}
	return nil
}
func (f *fullTableModel) supportedDependency(d Dependency, required map[fullDependencyAddress]bool) bool {
	if d.SubID != 0 {
		return false
	}
	if required[fullDependencyKey(d)] {
		return true
	}

	// Extension-owned objects are removed by the explicit extension RESTRICT step.
	if d.Kind == "extension_member" && d.ReferencedClass == "pg_extension" && d.ReferencedOID == f.m.old.OID && d.ReferencedSubID == 0 {
		switch d.Class {
		case "pg_opfamily":
			for _, x := range f.e.LegacyFamilies {
				if x.OID == d.OID {
					return true
				}
			}
		case "pg_am":
			for _, x := range f.m.c.AccessMethods {
				if x.OID == d.OID && x.ExtensionOID == f.m.old.OID {
					return true
				}
			}
		case "pg_proc":
			for _, x := range f.m.c.Functions {
				if x.OID == d.OID && x.ExtensionOID == f.m.old.OID {
					return true
				}
			}
		case "pg_operator":
			for _, x := range f.m.c.Operators {
				if x.OID == d.OID && x.ExtensionOID == f.m.old.OID {
					return true
				}
			}
		case "pg_opclass":
			for _, x := range f.m.c.OpClasses {
				if x.OID == d.OID && x.ExtensionOID == f.m.old.OID {
					return true
				}
			}
		}
		return false
	}
	if d.Kind == "operator_family" && d.ReferencedSubID == 0 {
		for _, x := range f.e.LegacyFamilies {
			if d.Class == "pg_opclass" && d.OID == x.OpClassOID {
				return d.ReferencedClass == "pg_opfamily" && d.ReferencedOID == x.OID
			}
			if d.Class == "pg_amop" && d.OID == x.AMOpOID {
				return d.ReferencedClass == "pg_opfamily" && (d.ReferencedOID == x.OID || d.ReferencedOID == x.SortFamilyOID) || d.ReferencedClass == "pg_opclass" && d.ReferencedOID == x.OpClassOID || d.ReferencedClass == "pg_operator" && d.ReferencedOID == x.OperatorOID
			}
			if d.Class == "pg_amproc" && d.OID == x.AMProcOID {
				return d.ReferencedClass == "pg_opfamily" && d.ReferencedOID == x.OID || d.ReferencedClass == "pg_opclass" && d.ReferencedOID == x.OpClassOID || d.ReferencedClass == "pg_proc" && d.ReferencedOID == x.FunctionOID
			}
		}
	}
	if d.Class == "pg_attrdef" && d.Kind == "default" {
		for _, x := range f.e.Defaults {
			if x.OID == d.OID {
				return d.ReferencedClass == "pg_class" && (d.ReferencedOID == f.m.table.OID && d.ReferencedSubID == x.AttNum || x.SequenceOID != 0 && d.ReferencedOID == x.SequenceOID && d.ReferencedSubID == 0)
			}
		}
	}
	if d.Class == "pg_constraint" && d.Kind == "constraint" {
		c, ok := f.constraints[d.OID]
		if !ok || (c.Kind != "p" && c.Kind != "u" && c.Kind != "c") {
			return false
		}
		if d.ReferencedClass == "pg_class" && d.ReferencedOID == f.m.table.OID {
			if d.ReferencedSubID == 0 {
				return true
			}
			for _, a := range c.Columns {
				if a == d.ReferencedSubID {
					return true
				}
			}
		}
	}
	if d.Class == "pg_class" && d.Kind == "sequence" {
		s, ok := f.sequences[d.OID]
		return ok && d.ReferencedClass == "pg_class" && d.ReferencedOID == f.m.table.OID && d.ReferencedSubID == s.OwnedAttNum
	}
	if d.Class == "pg_class" && d.Kind == "index" {
		x, ok := f.indexes[d.OID]
		if !ok {
			return false
		}
		facts := f.indexFacts[d.OID]
		if d.ReferencedClass == "pg_constraint" && facts.ConstraintOID != 0 && d.ReferencedOID == facts.ConstraintOID && d.ReferencedSubID == 0 {
			return true
		}
		if d.ReferencedClass == "pg_class" && d.ReferencedOID == f.m.table.OID {
			for _, a := range x.Keys {
				if a == d.ReferencedSubID {
					return true
				}
			}
		}
		if d.ReferencedSubID != 0 {
			return false
		}
		if d.ReferencedClass == "pg_opclass" {
			for _, oid := range x.OpClasses {
				if oid == d.ReferencedOID {
					return true
				}
			}
		}
		if d.ReferencedClass == "pg_collation" {
			for _, oid := range x.Collations {
				if oid != 0 && oid == d.ReferencedOID {
					return true
				}
			}
		}
	}
	if d.Class == "pg_class" && d.Kind == "internal_storage" && d.ReferencedClass == "pg_class" {
		t := f.e.Table
		return t.ToastRelationOID != 0 && (d.OID == t.ToastRelationOID && d.ReferencedOID == f.m.table.OID && d.ReferencedSubID == 0 || d.OID == t.ToastIndexOID && d.ReferencedOID == t.ToastRelationOID && (d.ReferencedSubID == 1 || d.ReferencedSubID == 2))
	}
	if d.Class == "pg_type" && d.Kind == "row_type" && d.ReferencedSubID == 0 {
		return d.OID == f.e.Table.RowTypeOID && d.ReferencedClass == "pg_class" && d.ReferencedOID == f.m.table.OID || d.OID == f.e.Table.RowArrayTypeOID && d.ReferencedClass == "pg_type" && d.ReferencedOID == f.e.Table.RowTypeOID
	}
	return false
}

// This is a finite object-address relation, not a general same-kind allowance.
// Built-in pinned dependencies are not synthesized: PostgreSQL normally omits
// them from pg_depend. Retained namespaces are never dropped or rebound.
func (f *fullTableModel) requiredObjectDependencies() []Dependency {
	out := []Dependency{}
	add := func(class string, oid uint32, kind, refclass string, ref uint32) {
		out = append(out, Dependency{Class: class, OID: oid, Kind: kind, ReferencedClass: refclass, ReferencedOID: ref})
	}
	namespace := func(class string, oid uint32, schema string) {
		add(class, oid, "namespace", "pg_namespace", f.namespaces[schema])
	}
	member := func(class string, oid uint32) {
		add(class, oid, "extension_member", "pg_extension", f.m.old.OID)
	}
	namespace("pg_class", f.m.table.OID, f.m.table.Schema)
	namespace("pg_extension", f.m.old.OID, f.m.old.Schema)
	for _, x := range f.e.Sequences {
		namespace("pg_class", x.OID, x.Schema)
	}
	for _, x := range f.m.c.AccessMethods {
		if x.ExtensionOID == f.m.old.OID {
			member("pg_am", x.OID)
			add("pg_am", x.OID, "legacy_implementation", "pg_proc", x.HandlerOID)
		}
	}
	for _, x := range f.m.c.Functions {
		if x.ExtensionOID == f.m.old.OID {
			member("pg_proc", x.OID)
			namespace("pg_proc", x.OID, x.Schema)
		}
	}
	for _, x := range f.m.c.Operators {
		if x.ExtensionOID == f.m.old.OID {
			member("pg_operator", x.OID)
			namespace("pg_operator", x.OID, x.Schema)
			add("pg_operator", x.OID, "legacy_implementation", "pg_proc", x.FunctionOID)
		}
	}
	for _, x := range f.m.c.OpClasses {
		if x.ExtensionOID == f.m.old.OID {
			member("pg_opclass", x.OID)
			namespace("pg_opclass", x.OID, x.Schema)
		}
	}
	for _, x := range f.e.LegacyFamilies {
		namespace("pg_opfamily", x.OID, x.Schema)
		add("pg_opfamily", x.OID, "operator_family", "pg_am", x.MethodOID)
	}
	for _, x := range f.m.indexes {
		for _, oid := range x.source.OpClasses {
			add("pg_class", x.source.OID, "index", "pg_opclass", oid)
		}
	}
	return out
}
