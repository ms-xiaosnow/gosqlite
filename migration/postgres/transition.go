package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode"
)

// TransitionApproach deliberately names the narrower typed-copy implementation.
// A full-table copy (including constraints, grants and dependent objects) is not
// implemented. Neither approach removes or renames the source real[] column.
type TransitionApproach string

const (
	RetainArrayCast    TransitionApproach = "retain_real_array_cast"
	TypedCopyShadow    TransitionApproach = "typed_copy_shadow_column"
	TransitionSingle                      = "single_transaction"
	TransitionBatches                     = "multi_transaction"
	MaxTransitionBytes                    = 8 << 20
)

// TransitionIndex is an explicit proposed rebuild of one verified source index.
// Zero parameters do not select defaults. The source metric cannot be changed.
type TransitionIndex struct {
	SourceIndexOID uint32 `json:"source_index_oid"`
	Name           string `json:"name"`
	Method         string `json:"method"`
	M              int    `json:"m"`
	EFConstruction int    `json:"ef_construction"`
	EFSearch       int    `json:"ef_search"`
	Lists          int    `json:"lists"`
	Probes         int    `json:"probes"`
}

// TransitionOptions describes intended objects, not permission to execute them.
// TargetVersion is a requested pinned version, not proof of server capability.
type TransitionOptions struct {
	RelationOID       uint32             `json:"relation_oid"`
	AttNum            int                `json:"attnum"`
	Dimensions        int                `json:"dimensions"`
	Approach          TransitionApproach `json:"approach"`
	Mode              string             `json:"mode"`
	VectorSchema      string             `json:"vector_schema"`
	TargetVersion     string             `json:"target_version"`
	ShadowColumn      string             `json:"shadow_column"`
	StateSchema       string             `json:"state_schema"`
	StateTable        string             `json:"state_table"`
	BatchSize         int                `json:"batch_size"`
	ConcurrentIndexes bool               `json:"concurrent_indexes"`
	Indexes           []TransitionIndex  `json:"indexes"`
	// A caller may lower the aggregate generated-output cap. Zero uses 8 MiB.
	MaxBytes int `json:"max_bytes"`
}

type TransitionGate struct {
	Code        string `json:"code"`
	State       string `json:"state"` // unresolved or blocked, never approved
	Requirement string `json:"requirement"`
}

type TransitionReport struct {
	FormatVersion    int               `json:"format_version"`
	Status           string            `json:"status"`
	Validation       string            `json:"validation"`
	PlanSHA256       string            `json:"plan_sha256"`
	CatalogSHA256    string            `json:"catalog_sha256"`
	DependencySHA256 string            `json:"dependency_sha256"`
	SourceProfile    string            `json:"source_profile"`
	TargetProfile    string            `json:"target_profile"`
	Options          TransitionOptions `json:"options"`
	Inspection       Report            `json:"inspection"`
	Gates            []TransitionGate  `json:"gates"`
	Unsupported      []string          `json:"unsupported"`
}

// TransitionArtifact is text, never an executable operation. SQL mutations and
// post-install queries are commented one physical line at a time. The only
// uncommented SQL artifact is the read-only preflight reference.
type TransitionArtifact struct {
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Transaction string `json:"transaction"`
	SHA256      string `json:"sha256"`
	Content     string `json:"content"`
}

type TransitionBundle struct {
	Report    TransitionReport     `json:"report"`
	Artifacts []TransitionArtifact `json:"artifacts"`
}

type transitionModel struct {
	c          CatalogSnapshot
	o          TransitionOptions
	r          TransitionReport
	table      Relation
	column, pk Column
	old        Extension
	indexes    []transitionIndexModel
	max        int
}
type transitionIndexModel struct {
	source                    Index
	target                    TransitionIndex
	metric, opclass, operator string
}

// GenerateTransition validates catalog identity and produces a deterministic,
// bounded, offline review bundle. It has no database, subprocess, network or
// file-writing API. Blocked inputs still receive diagnosis and preflight, but
// receive no proposed mutation statements. All human approval gates stay open.
func GenerateTransition(ctx context.Context, c CatalogSnapshot, o TransitionOptions) (TransitionBundle, error) {
	var empty TransitionBundle
	if ctx == nil {
		return empty, fmt.Errorf("transition: nil context")
	}
	if err := ctx.Err(); err != nil {
		return empty, err
	}
	if o.MaxBytes == 0 {
		o.MaxBytes = MaxTransitionBytes
	}
	if o.MaxBytes < 1 || o.MaxBytes > MaxTransitionBytes || len(o.Indexes) > 256 {
		return empty, fmt.Errorf("transition: resource limit")
	}
	if o.Approach != RetainArrayCast && o.Approach != TypedCopyShadow {
		return empty, fmt.Errorf("transition: unsupported approach; full-table copy is not implemented")
	}
	if o.Mode != TransitionSingle && o.Mode != TransitionBatches {
		return empty, fmt.Errorf("transition: explicit transaction mode required")
	}
	if !transitionName(o.VectorSchema) || o.Dimensions < 1 || o.Dimensions > 16000 {
		return empty, fmt.Errorf("transition: invalid target schema or dimensions")
	}
	if o.ConcurrentIndexes && o.Mode == TransitionSingle {
		return empty, fmt.Errorf("transition: concurrent indexes cannot share the offline transaction")
	}
	if o.Approach == TypedCopyShadow {
		if !transitionName(o.ShadowColumn) {
			return empty, fmt.Errorf("transition: explicit shadow column required")
		}
		if o.Mode == TransitionBatches && (!transitionName(o.StateSchema) || !transitionName(o.StateTable) || o.BatchSize < 1 || o.BatchSize > 100000) {
			return empty, fmt.Errorf("transition: explicit bounded batch state required")
		}
	}
	if o.Approach == RetainArrayCast && (o.ShadowColumn != "" || o.StateSchema != "" || o.StateTable != "" || o.BatchSize != 0) || o.Mode == TransitionSingle && (o.StateSchema != "" || o.StateTable != "" || o.BatchSize != 0) {
		return empty, fmt.Errorf("transition: irrelevant copy/batch options")
	}
	if len(o.TargetVersion) > 32 {
		return empty, fmt.Errorf("transition: invalid version length")
	}
	// Own the options slice before sorting; callers may reuse their input.
	o.Indexes = append([]TransitionIndex{}, o.Indexes...)
	sort.Slice(o.Indexes, func(i, j int) bool { return o.Indexes[i].SourceIndexOID < o.Indexes[j].SourceIndexOID })
	seen := map[uint32]bool{}
	names := map[string]bool{}
	for _, x := range o.Indexes {
		if x.SourceIndexOID == 0 || seen[x.SourceIndexOID] || !transitionName(x.Name) || names[x.Name] {
			return empty, fmt.Errorf("transition: invalid or duplicate index mapping")
		}
		seen[x.SourceIndexOID], names[x.Name] = true, true
		if x.Method == "hnsw" {
			if x.M < 2 || x.M > 100 || x.EFConstruction < 4 || x.EFConstruction > 1000 || x.EFConstruction < 2*x.M || x.EFSearch < 1 || x.EFSearch > 1000 || x.Lists != 0 || x.Probes != 0 {
				return empty, fmt.Errorf("transition: invalid HNSW parameters; no automatic retuning")
			}
		} else if x.Method == "ivfflat" {
			if x.Lists < 1 || x.Lists > 32768 || x.Probes < 1 || x.Probes > x.Lists || x.M != 0 || x.EFConstruction != 0 || x.EFSearch != 0 {
				return empty, fmt.Errorf("transition: invalid IVFFlat parameters")
			}
		} else {
			return empty, fmt.Errorf("transition: unsupported target index method")
		}
	}
	inspection, err := Inspect(ctx, c, InspectOptions{Selected: []Selection{{RelationOID: o.RelationOID, AttNum: o.AttNum, Dimensions: o.Dimensions}}})
	if err != nil {
		return empty, err
	}
	m := transitionModel{c: c, o: o, max: o.MaxBytes}
	m.r = TransitionReport{FormatVersion: 1, Status: "review_required", Validation: "offline_generated_not_postgresql_executed", CatalogSHA256: inspection.CatalogSHA256, SourceProfile: SourceProfile, TargetProfile: TargetProfile, Options: o, Inspection: inspection, Gates: []TransitionGate{}, Unsupported: []string{
		"Full-table typed copy, arbitrary dependency recreation, online/dual-write maintenance and parallel backfill are not implemented.",
		"Actual target availability, privileges, storage headroom, fresh catalog identity and application behavior are not verified offline.",
		"PostgreSQL execution, query-plan verification, ANN recall and operational rollback have not been exercised.",
		"No cleanup, CASCADE, source rename/drop, live connector, credentials or remote execution interface is provided.",
	}}
	for _, t := range c.Relations {
		if t.OID == o.RelationOID {
			m.table = t
			for _, a := range t.Columns {
				if a.AttNum == o.AttNum {
					m.column = a
				}
				if len(t.PrimaryKey) == 1 && a.AttNum == t.PrimaryKey[0] {
					m.pk = a
				}
			}
		}
	}
	if m.table.OID == 0 || m.column.AttNum == 0 {
		return empty, fmt.Errorf("transition: selected source absent")
	}
	for _, x := range c.Extensions {
		if x.Name == "embedding" {
			m.old = x
		}
		if x.Name == "vector" {
			m.block("target_already_installed", "An already installed vector extension requires a separate inspected transition; do not assume hnsw coexistence or reinstall it.")
		}
	}
	if !transitionName(m.table.Schema) || !transitionName(m.table.Name) || !transitionName(m.column.Name) || !transitionName(m.pk.Name) {
		return empty, fmt.Errorf("transition: source identifier contains unsupported control characters or missing primary key")
	}
	if o.TargetVersion != "0.8.7" {
		m.block("target_version_unknown", "Confirm availability of the exact pinned vector 0.8.7 version; missing or other versions are not supported by this template profile.")
	}
	if c.ServerVersionNum < 120000 || c.ServerVersionNum >= 190000 {
		m.block("server_version_unsupported", "Catalog/query reference covers PostgreSQL 12 through 18 only; inspect other server versions separately.")
	}
	for _, cr := range inspection.Columns {
		if cr.RelationOID == o.RelationOID && cr.AttNum == o.AttNum && cr.Status != "safe" {
			m.block("source_column_not_eligible", "Resolve the selected column's inspector findings and obtain a fresh exhaustive immutable-export profile before regenerating.")
		}
	}
	for _, f := range inspection.Findings {
		if f.Code == "incomplete_catalog_coverage" {
			m.block("dependency_inventory_incomplete", "Collect complete relation, column, index, type, membership, dependency and policy inventories; a direct-only catalog export is insufficient.")
		}
		if f.Severity == "error" {
			m.block("source_inspection_blocked", "Resolve all source identity, row-shape, dimension and parameter errors in the inspection report; explicit target choices do not waive them.")
		}
	}
	for _, a := range m.table.Columns {
		if o.Approach == TypedCopyShadow && a.Name == o.ShadowColumn {
			m.block("shadow_name_collision", "Choose a new shadow column; never adopt an existing same-name column by assumption.")
		}
	}
	for _, t := range c.Relations {
		// PostgreSQL SELECT/UPDATE/ALTER normally include inheritance children.
		// The parent's declared PK does not establish uniqueness across them.
		// Reject the known hierarchy rather than broaden this one-heap plan.
		for _, parent := range t.Parents {
			if parent == m.table.OID {
				m.block("source_inheritance_unsupported", "The selected source has an inventoried inheritance/partition child. This one-heap transition cannot establish global key identity or safely copy/index the hierarchy; supply a separate reviewed migration plan.")
			}
		}
		if t.Schema == o.StateSchema && t.Name == o.StateTable {
			m.block("state_name_collision", "Choose a new migration-state relation; existing state is not adopted.")
		}
		if t.Schema == m.table.Schema && names[t.Name] {
			m.block("index_name_collision", "Target index name conflicts with an inventoried relation.")
		}
	}
	for _, x := range c.Indexes {
		if x.Schema == m.table.Schema && names[x.Name] {
			m.block("index_name_collision", "Target index name conflicts with an existing index; choose a distinct name.")
		}
	}
	if o.Mode == TransitionBatches && o.Approach == TypedCopyShadow && o.StateSchema == m.table.Schema && (o.StateTable == m.table.Name || names[o.StateTable]) {
		return empty, fmt.Errorf("transition: generated object name collision")
	}
	irs := map[uint32]IndexReport{}
	for _, x := range inspection.Indexes {
		irs[x.OID] = x
	}
	classes := map[uint32]OpClass{}
	for _, x := range c.OpClasses {
		classes[x.OID] = x
	}
	sourceIndexes := map[uint32]Index{}
	for _, x := range c.Indexes {
		sourceIndexes[x.OID] = x
	}
	for _, x := range o.Indexes {
		idx, exists := sourceIndexes[x.SourceIndexOID]
		ir := irs[x.SourceIndexOID]
		if !exists || idx.TableOID != m.table.OID || len(idx.Keys) != 1 || idx.Keys[0] != o.AttNum || !ir.IdentityVerified || ir.Status != "safe" {
			m.block("index_mapping_unsupported", "Every proposed index must map to a safe, positively identified source index on the selected column.")
			continue
		}
		mp := legacyClasses[classes[idx.OpClasses[0]].Name]
		if !transitionName(idx.Schema) || !transitionName(idx.Name) {
			return empty, fmt.Errorf("transition: source index control character")
		}
		if o.Dimensions > 2000 {
			m.block("target_index_dimension_limit", "Dense HNSW/IVFFlat indexes are limited to 2000 dimensions independently of the 16000-dimension storage limit.")
		}
		if x.Method == "ivfflat" && mp.metric == "l1" {
			m.block("ivfflat_l1_unsupported", "The pinned pgvector profile has no IVFFlat L1 opclass; retain HNSW or review a separate exact plan.")
		}
		m.indexes = append(m.indexes, transitionIndexModel{idx, x, mp.metric, mp.target, map[string]string{"l2": "<->", "cosine": "<=>", "l1": "<+>"}[mp.metric]})
	}
	for _, ir := range inspection.Indexes {
		if !seen[ir.OID] {
			m.block("unplanned_legacy_index", "The extension transition affects an index not covered by this one-column plan; coordinate a broader separately reviewed transition.")
		}
	}
	for _, d := range c.Dependencies {
		ownedMember := d.Kind == "extension_member" && d.ReferencedClass == "pg_extension" && d.ReferencedOID == m.old.OID && m.old.OID != 0
		if !ownedMember && !(d.Kind == "index" && d.Class == "pg_class" && seen[d.OID]) {
			m.block("dependent_object_requires_manual_plan", "An affected dependency has no generated recreate/drop plan. Review inventory.json and supply a separately reviewed dependency transition; no raw definition is executed.")
		}
	}
	for _, g := range []TransitionGate{
		{"fresh_catalog", "unresolved", "Recollect the complete catalog and dependencies under write quiescence; compare canonical_json_v1 catalog and dependency hashes. Any drift requires regeneration. Preflight diagnostics do not compute the full Go catalog hash."},
		{"maintenance_window", "unresolved", "Approve a maintenance window with all application writes, DDL and primary-key changes paused from validation through cutover. Row locks between batches do not enforce this contract. Separate-database/branch transition is a different plan."},
		{"snapshot_and_old_service", "unresolved", "Verify immutable export/backup, restoration procedure, original definitions, and actual availability of embedding 0.3.6 binaries before promising old-ANN rollback."},
		{"privileges_capacity_locks", "unresolved", "Verify table/extension ownership and required CREATE/USAGE/SELECT/UPDATE rights, RLS visibility, disk/WAL headroom, lock and downtime budgets; extension installation may require a provider administrator."},
		{"extension_transition", "unresolved", "Approve each inventoried index drop and DROP EXTENSION embedding RESTRICT individually. Both extensions own database-wide hnsw. Unexpected dependencies mean stop, never CASCADE or catalog/AM renaming hacks."},
		{"target_capabilities", "unresolved", "Verify exact available and installed vector 0.8.7, extension schema, extension-owned hnsw/ivfflat, real[] casts, vector type, operators/functions/opclasses and option bounds before copy/index/application use."},
		{"data_and_training", "unresolved", "Revalidate all source rows and PK identity, then full NULL/value/norm alignment and exact distance/business fixtures. Review IVF training/list/probe choices or HNSW retuning; evaluate ANN recall separately."},
		{"application_cutover", "unresolved", "Approve query/parameter/operator/column/settings/NULL/zero/tie/filter changes together, then cut over explicitly. Maintain retained arrays on every later write or reconcile before rollback."},
		{"cleanup_separate", "unresolved", "Keep source arrays, export and migration state through an agreed rollback window. Cleanup and post-cutover old-service restoration require new operational plans; no cleanup statements are generated."},
	} {
		m.r.Gates = append(m.r.Gates, g)
	}
	sort.Slice(m.r.Gates, func(i, j int) bool { return m.r.Gates[i].Code < m.r.Gates[j].Code })
	deps, err := json.Marshal(c.Dependencies)
	if err != nil {
		return empty, err
	}
	m.r.DependencySHA256 = sqlHash(string(deps))
	identity, _ := json.Marshal(struct {
		Catalog string
		Options TransitionOptions
	}{inspection.CatalogSHA256, o})
	m.r.PlanSHA256 = sqlHash(string(identity))
	return m.render(ctx)
}

func (m *transitionModel) block(code, requirement string) {
	for _, g := range m.r.Gates {
		if g.Code == code {
			return
		}
	}
	m.r.Status = "blocked"
	m.r.Gates = append(m.r.Gates, TransitionGate{code, "blocked", requirement})
}
func transitionName(s string) bool {
	if !validName(s) {
		return false
	}
	for _, r := range s {
		if unicode.IsControl(r) || r == '\u2028' || r == '\u2029' {
			return false
		}
	}
	return true
}

// transitionLiteral is independent of standard_conforming_strings and safely
// preserves quote/backslash characters in catalog-name comparisons.
func transitionLiteral(s string) string {
	return "E'" + strings.NewReplacer("\\", "\\\\", "'", "''", "\r", "\\r", "\n", "\\n", "\t", "\\t").Replace(s) + "'"
}

// RenderTransitionJSON does not execute or save any artifact. It refuses an
// altered/inconsistent artifact hash and bounds programmatic input before marshal.
func RenderTransitionJSON(w io.Writer, b TransitionBundle) error {
	if w == nil {
		return fmt.Errorf("transition: nil writer")
	}
	if len(b.Artifacts) > 32 {
		return fmt.Errorf("transition: artifact limit")
	}
	n := 0
	for _, a := range b.Artifacts {
		n += len(a.Content)
		if n > MaxTransitionBytes || len(a.Name) > 80 || len(a.Kind) > 80 || len(a.Transaction) > 80 || sqlHash(a.Content) != a.SHA256 {
			return fmt.Errorf("transition: invalid artifact or byte limit")
		}
	}
	// Render a manifest plus artifacts; the authoritative report is already
	// included as the hashed report.json artifact, avoiding a second mutable copy.
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	return e.Encode(struct {
		Artifacts []TransitionArtifact `json:"artifacts"`
	}{b.Artifacts})
}
