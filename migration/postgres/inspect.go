package postgres

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Selection positively identifies an unindexed real[] column. Indexed columns
// with verified legacy identities are selected automatically. Dimension
// overrides cannot contradict catalog index or exhaustive profile evidence.
type Selection struct {
	RelationOID        uint32
	AttNum, Dimensions int
}
type InspectOptions struct {
	SourceProfile, TargetProfile string
	Selected                     []Selection
	Limits                       Limits
}

func Inspect(ctx context.Context, c CatalogSnapshot, opts InspectOptions) (Report, error) {
	var r Report
	if ctx == nil {
		return r, fmt.Errorf("inspect: nil context")
	}
	if err := ctx.Err(); err != nil {
		return r, err
	}
	l, err := opts.Limits.normalized()
	if err != nil {
		return r, err
	}
	if err = validateCatalog(c, l); err != nil {
		return r, err
	}
	if opts.SourceProfile != "" && opts.SourceProfile != SourceProfile || opts.TargetProfile != "" && opts.TargetProfile != TargetProfile {
		return r, fmt.Errorf("inspect: unsupported requested profile")
	}
	r = Report{FormatVersion: 1, SourceProfile: SourceProfile, TargetProfile: TargetProfile, ExtractedAt: c.ExtractedAt, Status: "safe", Scope: Scope{Schemas: append([]string(nil), c.Scope.Schemas...), Coverage: append([]Coverage(nil), c.Scope.Coverage...)}, Columns: []ColumnReport{}, Indexes: []IndexReport{}, Findings: []Finding{}, ApprovalPoints: []string{"Review target installation/version, privileges, dependency inventory and application query changes before PostgreSQL execution.", "Review maintenance-window or separate-database strategy for the database-wide hnsw access-method collision.", "Retain source arrays and immutable export; retuning index parameters requires explicit review and new recall measurements."}}
	b, e := json.Marshal(c)
	if e != nil {
		return Report{}, e
	}
	if len(b) > l.MaxBytes {
		return Report{}, fmt.Errorf("catalog: byte limit exceeded")
	}
	h := sha256.Sum256(b)
	r.CatalogSHA256 = hex.EncodeToString(h[:])
	r.CatalogHashKind = "canonical_json_v1"
	if c.semanticSHA256 == r.CatalogSHA256 {
		r.SourceBytesSHA256 = c.sourceSHA256
	}

	r.add("warning", "current_neon_chapter_absent", "source", "Current Neon target page was retrieved on 2026-10-10; its requested migration chapter is absent. The official historical 2024-01-23 chapter is a separate reference, not current-page conformance.")
	r.add("warning", "application_sql_inventory_incomplete", "scope", "Catalog inspection cannot discover arbitrary application or dynamic SQL; supplied SQL corpus is not yet converted by this inspector.")
	r.add("warning", "numeric_semantics_difference", "source", "Legacy distances return architecture-sensitive float32; local metrics use compensated float64, and cosine rejects zero norms. Exact result-bit equality is not claimed.")
	coverage := map[string]Coverage{}
	for _, v := range c.Scope.Coverage {
		coverage[v.Category] = v
	}
	for _, key := range []string{"relations", "columns", "indexes", "types", "extension_membership", "dependencies", "policies"} {
		v, ok := coverage[key]
		if !ok || !v.Complete {
			r.add("warning", "incomplete_catalog_coverage", "scope."+key, "Required catalog category is absent or explicitly incomplete; do not treat the snapshot as complete.")
		}
	}
	var old Extension
	for _, x := range c.Extensions {
		if x.Name == "embedding" {
			old = x
		}
	}
	if old.OID == 0 {
		r.add("error", "legacy_extension_missing", "extensions", "The embedding extension identity is absent.")
	} else if old.Version != "0.3.6" {
		r.add("error", "source_version_unsupported", objectKey(old.Schema, old.Name), "Only the pinned embedding 0.3.6 profile is supported.")
	}
	types := map[uint32]Type{}
	for _, x := range c.Types {
		types[x.OID] = x
	}
	realIdentity := types[700].Schema == "pg_catalog" && types[700].Name == "float4" && types[700].Kind == "b" && types[700].ElementOID == 0 && types[700].BaseOID == 0 && types[1021].Schema == "pg_catalog" && types[1021].Name == "_float4" && types[1021].Kind == "b" && types[1021].ElementOID == 700 && types[1021].BaseOID == 0
	if !realIdentity {
		r.add("error", "real_array_identity_mismatch", "types", "Required pg_catalog float4 (700) and _float4 (1021) base/element identities are not established.")
	}
	handlerTypesVerified := types[2281].Schema == "pg_catalog" && types[2281].Name == "internal" && types[2281].Kind == "p" && types[2281].BaseOID == 0 && types[2281].ElementOID == 0 && types[325].Schema == "pg_catalog" && types[325].Name == "index_am_handler" && types[325].Kind == "p" && types[325].BaseOID == 0 && types[325].ElementOID == 0
	methods := map[uint32]AccessMethod{}
	functions := map[uint32]Function{}
	operators := map[uint32]Operator{}
	classes := map[uint32]OpClass{}
	for _, x := range c.AccessMethods {
		methods[x.OID] = x
		if x.Name == "hnsw" && x.ExtensionOID == old.OID && old.OID != 0 {
			r.add("warning", "hnsw_access_method_collision", "access_method.hnsw", "Both pinned extensions own database-wide hnsw. A side-by-side unmodified extension installation is not a valid in-place migration plan.")
		}
	}
	for _, x := range c.Functions {
		functions[x.OID] = x
	}
	for _, x := range c.Operators {
		operators[x.OID] = x
	}
	for _, x := range c.OpClasses {
		classes[x.OID] = x
	}
	relation := map[uint32]Relation{}
	column := map[string]Column{}
	colReport := map[string]*ColumnReport{}
	selection := map[string]int{}
	key := func(oid uint32, att int) string { return fmt.Sprintf("%d/%d", oid, att) }
	if len(opts.Selected) > l.MaxObjects {
		return Report{}, fmt.Errorf("inspect: selection limit exceeded")
	}
	for _, s := range opts.Selected {
		k := key(s.RelationOID, s.AttNum)
		if s.Dimensions < 1 || s.Dimensions > 16000 || selection[k] != 0 {
			return Report{}, fmt.Errorf("inspect: invalid or duplicate selected column")
		}
		selection[k] = s.Dimensions
	}
	for _, t := range c.Relations {
		relation[t.OID] = t
		for _, a := range t.Columns {
			k := key(t.OID, a.AttNum)
			column[k] = a
			if a.Dropped {
				continue
			}
			if a.TypeOID == 1021 || selection[k] != 0 {
				cr := &ColumnReport{Object: objectKey(t.Schema, t.Name) + "." + quoteIdent(a.Name), RelationOID: t.OID, AttNum: a.AttNum, Classification: "unrelated_real_array_candidate", Dimensions: selection[k], Selected: selection[k] != 0, Status: "safe", Validation: "not_run"}
				if cr.Selected {
					cr.Classification = "explicitly_selected_real_array"
				}
				if a.TypeOID != 1021 || !realIdentity {
					cr.Status = "blocked"
					r.add("error", "selected_type_unsupported", cr.Object, "Only verified pg_catalog real[] columns can be selected; domains and unrelated types require explicit support.")
				}
				colReport[k] = cr
			}
		}
	}
	for k := range selection {
		if colReport[k] == nil {
			return Report{}, fmt.Errorf("inspect: selected column is absent or dropped")
		}
	}
	settings := map[int]bool{}
	legacyMetrics := map[uint32]string{}
	for _, idx := range c.Indexes {
		if err = ctx.Err(); err != nil {
			return Report{}, err
		}
		method := methods[idx.MethodOID]
		legacy := method.ExtensionOID == old.OID && old.OID != 0
		for _, id := range idx.OpClasses {
			if classes[id].ExtensionOID == old.OID && old.OID != 0 {
				legacy = true
			}
		}
		if !legacy && method.Name != "hnsw" {
			continue
		}
		ir := IndexReport{Object: objectKey(idx.Schema, idx.Name), OID: idx.OID, Status: "safe"}
		finding := func(sev, code, msg string) {
			ir.Status = mergeStatus(ir.Status, severityStatus(sev))
			r.add(sev, code, ir.Object, msg)
		}
		if len(idx.Keys) != 1 || idx.Keys[0] == 0 || idx.Expression != "" || idx.Predicate != "" {
			finding("error", "index_shape_unsupported", "Only a single plain key with no expression or predicate is supported.")
			r.Indexes = append(r.Indexes, ir)
			continue
		}
		k := key(idx.TableOID, idx.Keys[0])
		a := column[k]
		cr := colReport[k]
		if cr != nil {
			ir.Column = cr.Object
		}
		oc := classes[idx.OpClasses[0]]
		mp, known := legacyClasses[oc.Name]
		op := operators[oc.OperatorOID]
		fn := functions[oc.FunctionOID]
		handler := functions[method.HandlerOID]
		ir.IdentityVerified = realIdentity && handlerTypesVerified && old.OID != 0 && old.Version == "0.3.6" && a.TypeOID == 1021 && !a.Dropped && method.Name == "hnsw" && method.ExtensionOID == old.OID && handler.ExtensionOID == old.OID && handler.Schema == old.Schema && handler.Name == "hnsw_handler" && len(handler.ArgumentOIDs) == 1 && handler.ArgumentOIDs[0] == 2281 && handler.ReturnOID == 325 && known && oc.Schema == old.Schema && oc.ExtensionOID == old.OID && oc.MethodOID == method.OID && oc.InputOID == 1021 && (oc.Default == (oc.Name == "ann_l2_ops")) && op.ExtensionOID == old.OID && op.Schema == old.Schema && op.Name == mp.operator && op.LeftOID == 1021 && op.RightOID == 1021 && op.FunctionOID == fn.OID && fn.ExtensionOID == old.OID && fn.Schema == old.Schema && fn.Name == mp.function && len(fn.ArgumentOIDs) == 2 && fn.ArgumentOIDs[0] == 1021 && fn.ArgumentOIDs[1] == 1021 && fn.ReturnOID == 700
		if !ir.IdentityVerified {
			finding("error", "legacy_index_identity_mismatch", "Access method, extension, opclass, strategy operator or support-function signature does not match the pinned legacy profile.")
			if cr != nil {
				cr.Status = "blocked"
			}
			r.Indexes = append(r.Indexes, ir)
			continue
		}
		legacyMetrics[idx.OID] = mp.metric
		cr.Classification = "legacy_vector"
		cr.Selected = true
		if !idx.Valid || !idx.Ready {
			finding("error", "index_not_ready", "Source index is not both valid and ready.")
		}
		if idx.Collations[0] != 0 {
			finding("error", "index_collation_unsupported", "Unexpected index collation is not supported.")
		}
		options, e := parseLegacyOptions(idx.Options)
		ir.Legacy = options
		if e != nil {
			finding("error", e.Code, e.Message)
		} else {
			ir.QueryEFSearch = options.EFSearch
			settings[options.EFSearch] = true
			if cr.Dimensions != 0 && cr.Dimensions != options.Dimensions {
				cr.Status = "blocked"
				finding("error", "conflicting_dimensions", "Indexes or explicit selection disagree on the column dimension.")
			} else {
				cr.Dimensions = options.Dimensions
			}
			ir.ProposedTarget = &TargetOptions{options.Dimensions, options.M, options.EFConstruction, mp.target, mp.metric}
			if options.M < 2 || options.M > 100 || options.EFConstruction < 4 || options.EFConstruction > 1000 || options.EFConstruction < 2*options.M {
				finding("error", "target_option_constraint", "Effective legacy m/efconstruction violate target m=2..100, ef_construction=4..1000 and ef_construction>=2*m. Values are reported unchanged; review retuning instead of clamping.")
			}
			if options.Dimensions > 2000 {
				finding("error", "target_index_dimension_limit", "The dense value storage limit is 16000 but the pinned HNSW index limit is 2000.")
			}
			if options.EFSearch > 1000 {
				finding("error", "target_query_setting_limit", "Legacy efsearch exceeds the pinned target/local 1..1000 query-setting bound.")
			}
			r.add("warning", "efsearch_scope_change", ir.Object, "Legacy per-index efsearch becomes an explicit PostgreSQL session/transaction setting or local VectorSearchOptions.EFSearch, never a CREATE INDEX option.")
		}
		if ir.Status != "safe" {
			ir.ProposedTarget = nil
		}
		cr.Status = mergeStatus(cr.Status, ir.Status)
		r.Indexes = append(r.Indexes, ir)
	}
	if len(settings) > 1 {
		r.add("warning", "efsearch_session_scope_conflict", "indexes", "Selected legacy indexes have different efsearch values; application-specific session/query scoping is required.")
	}
	profiles := map[string]DataProfile{}
	for _, p := range c.Profiles {
		profiles[key(p.RelationOID, p.AttNum)] = p
	}
	for k, cr := range colReport {
		if err = ctx.Err(); err != nil {
			return Report{}, err
		}
		if !cr.Selected {
			r.Columns = append(r.Columns, *cr)
			continue
		}
		t := relation[cr.RelationOID]
		a := column[k]
		bad := func(sev, code, msg string) {
			cr.Status = mergeStatus(cr.Status, severityStatus(sev))
			r.add(sev, code, cr.Object, msg)
		}
		if t.Kind != "r" || t.Persistence != "p" || t.RLS || len(t.Parents) != 0 {
			bad("warning", "relation_requires_review", "Automatic subset requires an ordinary permanent heap without RLS or inheritance/partition parent.")
		}
		pkOK := len(t.PrimaryKey) == 1
		if pkOK {
			p := column[key(t.OID, t.PrimaryKey[0])]
			pt := types[p.TypeOID]
			pkOK = p.NotNull && !p.Dropped && pt.Schema == "pg_catalog" && pt.Kind == "b" && pt.BaseOID == 0 && ((pt.OID == 20 && pt.Name == "int8") || (pt.OID == 23 && pt.Name == "int4"))
		}
		if pkOK {
			pk := column[key(t.OID, t.PrimaryKey[0])]
			if pk.Identity != "" || pk.Generated != "" || pk.Default != "" {
				bad("warning", "primary_key_generation_requires_review", "Source primary-key identity/default generation is not silently mapped to local INTEGER PRIMARY KEY allocation.")
			}
		}
		if !pkOK {
			bad("warning", "primary_key_unsupported", "Automatic import/backfill requires one declared non-NULL pg_catalog int4/int8 primary-key column.")
		}
		if a.Generated != "" || a.Identity != "" || a.Default != "" || a.TypeMod != -1 {
			bad("warning", "column_semantics_require_review", "Generated/default expressions or nondefault real[] typmod are outside the automatic subset.")
		}
		if cr.Dimensions < 1 || cr.Dimensions > 16000 {
			bad("error", "dimension_required", "A consistent explicit dimension in 1..16000 is required; dimensions are not inferred from the first row.")
		}
		p, ok := profiles[k]
		if !ok {
			bad("warning", "data_profile_missing", "No full-data profile is supplied; attndims does not establish actual row shape.")
		} else {
			cr.Validation = "sampled"
			if p.Exhaustive {
				cr.Validation = "exhaustive"
			} else {
				bad("warning", "data_profile_sampled", "A sampled profile cannot establish all-row validity.")
			}
			if p.ExportID == "" {
				bad("warning", "export_identity_missing", "Profile is not tied to a named immutable export.")
			}
			if !p.PrimaryKeyUnique || !p.PrimaryKeyOrdered {
				bad("warning", "primary_key_profile_unverified", "Export primary-key uniqueness and ascending order have not both been verified.")
			}
			if p.NullElementRows > 0 {
				bad("error", "null_element", "Source contains arrays with SQL NULL elements.")
			}
			if p.NonFiniteRows > 0 {
				bad("error", "nonfinite_component", "Source contains NaN or infinite real components.")
			}
			if a.NotNull && p.NullRows > 0 {
				bad("error", "profile_nullability_conflict", "Profile NULL rows contradict the selected NOT NULL column.")
			}
			for _, s := range p.Shapes {
				if s.Rows == 0 {
					continue
				}
				if len(s.Dimensions) != 1 || s.Dimensions[0] != cr.Dimensions {
					bad("error", "array_shape_mismatch", "Source contains empty, multidimensional or mismatched-dimension arrays.")
					break
				}
			}
			for _, s := range p.Shapes {
				if s.Rows == 0 {
					continue
				}
				if len(s.LowerBounds) != 1 || s.LowerBounds[0] != 1 {
					bad("error", "nondefault_array_bounds", "Source array bounds are not one-dimensional with lower bound 1.")
					break
				}
			}
			if p.ZeroNormRows > 0 {
				for _, ir := range r.Indexes {
					if ir.Column == cr.Object && legacyMetrics[ir.OID] == "cosine" {
						bad("error", "cosine_zero_norm", "Local cosine distance/index semantics reject zero-norm vectors.")
						break
					}
				}
			}
		}
		r.Columns = append(r.Columns, *cr)
	}
	for i := range r.Indexes {
		if err = ctx.Err(); err != nil {
			return Report{}, err
		}
		for _, cr := range r.Columns {
			if r.Indexes[i].Column == cr.Object && cr.Status != "safe" {
				r.Indexes[i].Status = mergeStatus(r.Indexes[i].Status, cr.Status)
				r.Indexes[i].ProposedTarget = nil
			}
		}
	}
	for _, d := range c.Dependencies {
		if err = ctx.Err(); err != nil {
			return Report{}, err
		}
		code := "dependent_object_requires_review"
		switch d.Kind {
		case "view", "materialized_view", "function", "trigger", "constraint", "policy", "index", "extension_member":
		default:
			code = "unknown_dependency_kind"
		}
		r.add("warning", code, d.Identity, "Dependent object definitions are inventoried as data only; no dependent body is executed or automatically rewritten.")
	}
	r.finish()
	return r, nil
}

type optionError struct{ Code, Message string }

func parseLegacyOptions(options []string) (LegacyOptions, *optionError) {
	o := LegacyOptions{M: 100, EFConstruction: 16, EFSearch: 64}
	seen := map[string]bool{}
	for _, raw := range options {
		p := strings.Split(raw, "=")
		if len(p) != 2 {
			return o, &optionError{"invalid_index_option", "Index options must be exact name=integer entries."}
		}
		name, value := p[0], p[1]
		if seen[name] {
			return o, &optionError{"duplicate_index_option", "Duplicate index option names are not accepted."}
		}
		seen[name] = true
		switch name {
		case "dims", "m", "efconstruction", "efsearch":
		default:
			return o, &optionError{"unknown_index_option", "Unknown index options are not silently dropped."}
		}
		if value == "" || len(value) > 10 {
			return o, &optionError{"invalid_index_option", "Invalid integer index option."}
		}
		for _, b := range value {
			if b < '0' || b > '9' {
				return o, &optionError{"invalid_index_option", "Index options require unsigned decimal integer syntax."}
			}
		}
		n, err := strconv.ParseInt(value, 10, 32)
		if err != nil {
			return o, &optionError{"invalid_index_option", "Index option is out of range."}
		}
		switch name {
		case "dims":
			o.Dimensions = int(n)
		case "m":
			o.M = int(n)
		case "efconstruction":
			o.EFConstruction = int(n)
		case "efsearch":
			o.EFSearch = int(n)
		}
	}
	if o.Dimensions < 1 || o.Dimensions > 16000 {
		return o, &optionError{"dimension_required", "An explicit dims option in the supported storage range 1..16000 is required."}
	}
	if o.EFConstruction < 1 || o.EFSearch < 1 {
		return o, &optionError{"invalid_index_option", "Legacy efconstruction/efsearch must be positive."}
	}
	return o, nil
}
