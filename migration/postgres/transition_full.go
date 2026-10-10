package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

const (
	// FullTableCopy is available only through GenerateFullTableTransition. Existing
	// TransitionOptions and catalog v1 encodings intentionally remain unchanged.
	FullTableCopy             TransitionApproach = "full_table_copy_swap"
	FullTableEvidenceVersion                     = 1
	FullTableCollector                           = "gosqlite-pg16-full-table-v1"
	MaxFullTableEvidenceBytes                    = 2 << 20
)

// FullTableOptions separates operator choices from captured catalog evidence.
// ArchiveNames name retained source indexes (including constraint indexes).
// All other source names are preserved on the new relation after the swap.
type FullTableOptions struct {
	Transition   TransitionOptions `json:"transition"`
	CopyTable    string            `json:"copy_table"`
	BackupTable  string            `json:"backup_table"`
	ArchiveNames []FullArchiveName `json:"archive_names"`
}
type FullArchiveName struct {
	IndexOID uint32 `json:"index_oid"`
	Name     string `json:"name"`
}

// FullTableEvidence is a separately versioned PostgreSQL 16 catalog supplement.
// It is evidence, not approval or proof that the source server is authentic.
// Complete inventories are required even when their corresponding lists are empty.
// Every catalog dependency must also appear unchanged in Dependencies.
type FullTableEvidence struct {
	Namespaces                []FullNamespace         `json:"namespaces"`
	LegacyFamilies            []FullLegacyFamily      `json:"legacy_families"`
	DeparseSearchPath         string                  `json:"deparse_search_path"`
	StandardConformingStrings bool                    `json:"standard_conforming_strings"`
	FormatVersion             int                     `json:"format_version"`
	Collector                 string                  `json:"collector"`
	CatalogSHA256             string                  `json:"catalog_sha256"`
	ExtractedAt               string                  `json:"extracted_at"`
	ServerVersionNum          int                     `json:"server_version_num"`
	SourceProfile             string                  `json:"source_profile"`
	TargetProfile             string                  `json:"target_profile"`
	Coverage                  []Coverage              `json:"coverage"`
	Table                     FullTableFacts          `json:"table"`
	Roles                     []FullRole              `json:"roles"`
	Columns                   []FullColumnFacts       `json:"columns"`
	Defaults                  []FullDefault           `json:"defaults"`
	Constraints               []FullConstraint        `json:"constraints"`
	Indexes                   []FullIndexFacts        `json:"indexes"`
	Sequences                 []FullSequence          `json:"sequences"`
	Privileges                []FullGrant             `json:"privileges"`
	CreationPrivileges        []FullGrant             `json:"creation_privileges"`
	Dependencies              []Dependency            `json:"dependencies"`
	UnsupportedObjects        []FullUnsupportedObject `json:"unsupported_objects"`
}
type FullNamespace struct {
	OID  uint32 `json:"oid"`
	Name string `json:"name"`
}
type FullRole struct {
	OID  uint32 `json:"oid"`
	Name string `json:"name"`
}
type FullTableFacts struct {
	ToastRelationOID uint32   `json:"toast_relation_oid"`
	ToastIndexOID    uint32   `json:"toast_index_oid"`
	ToastOwnerOID    uint32   `json:"toast_owner_oid"`
	ToastOptions     []string `json:"toast_options"`
	RelationOID      uint32   `json:"relation_oid"`
	Schema           string   `json:"schema"`
	Name             string   `json:"name"`
	NamespaceOID     uint32   `json:"namespace_oid"`
	OwnerOID         uint32   `json:"owner_oid"`
	RowTypeOID       uint32   `json:"row_type_oid"`
	RowArrayTypeOID  uint32   `json:"row_array_type_oid"`
	AccessMethodOID  uint32   `json:"access_method_oid"`
	AccessMethod     string   `json:"access_method"`
	TablespaceOID    uint32   `json:"tablespace_oid"`
	Tablespace       string   `json:"tablespace"`
	ForceRLS         bool     `json:"force_rls"`
	ReplicaIdentity  string   `json:"replica_identity"`
	Options          []string `json:"options"`
	Comment          string   `json:"comment"`
}
type FullColumnFacts struct {
	AttNum          int      `json:"attnum"`
	CollationOID    uint32   `json:"collation_oid"`
	CollationSchema string   `json:"collation_schema"`
	CollationName   string   `json:"collation_name"`
	Storage         string   `json:"storage"`
	Compression     string   `json:"compression"`
	Statistics      int      `json:"statistics"`
	Options         []string `json:"options"`
	Comment         string   `json:"comment"`
}
type FullDefault struct {
	OID        uint32 `json:"oid"`
	AttNum     int    `json:"attnum"`
	Expression string `json:"expression"`
	// SequenceOID must be nonzero only for the bounded nextval(regclass) form.
	SequenceOID uint32 `json:"sequence_oid"`
}
type FullConstraint struct {
	OID               uint32 `json:"oid"`
	Name              string `json:"name"`
	Kind              string `json:"kind"` // p, u, c; all other kinds are gated
	Columns           []int  `json:"columns"`
	IndexOID          uint32 `json:"index_oid"`
	Validated         bool   `json:"validated"`
	Deferrable        bool   `json:"deferrable"`
	InitiallyDeferred bool   `json:"initially_deferred"`
	NoInherit         bool   `json:"no_inherit"`
	Expression        string `json:"expression"`
	Comment           string `json:"comment"`
}
type FullIndexKey struct {
	AttNum     int  `json:"attnum"`
	Descending bool `json:"descending"`
	NullsFirst bool `json:"nulls_first"`
}
type FullIndexFacts struct {
	Immediate        bool           `json:"immediate"`
	Live             bool           `json:"live"`
	OID              uint32         `json:"oid"`
	Unique           bool           `json:"unique"`
	Primary          bool           `json:"primary"`
	ConstraintOID    uint32         `json:"constraint_oid"`
	NullsNotDistinct bool           `json:"nulls_not_distinct"`
	Keys             []FullIndexKey `json:"keys"`
	Included         []int          `json:"included"`
	TablespaceOID    uint32         `json:"tablespace_oid"`
	Tablespace       string         `json:"tablespace"`
	ReplicaIdentity  bool           `json:"replica_identity"`
	Clustered        bool           `json:"clustered"`
	Comment          string         `json:"comment"`
}

// Existing sequences are retained, never reset/recreated/advanced during copy.
// Type/parameters/ACL/state consequently keep the same database object identity.
// External users or other ownership/default dependencies still block the plan.
type FullSequence struct {
	OID            uint32 `json:"oid"`
	Schema         string `json:"schema"`
	Name           string `json:"name"`
	OwnerOID       uint32 `json:"owner_oid"`
	OwnedAttNum    int    `json:"owned_attnum"`
	DependencyType string `json:"dependency_type"` // a: serial/OWNED BY, not identity i
}

// Privileges are expanded ACL entries, including owner and PUBLIC (OID 0).
// Only grants originating at the captured table owner are supported. AttNum=0
// means table-level; positive AttNum means a column ACL. Role inheritance is
// unchanged because all referenced roles are retained, not recreated.
type FullGrant struct {
	GrantorOID  uint32 `json:"grantor_oid"`
	GranteeOID  uint32 `json:"grantee_oid"`
	AttNum      int    `json:"attnum"`
	Privilege   string `json:"privilege"`
	GrantOption bool   `json:"grant_option"`
}
type FullUnsupportedObject struct {
	Class      string `json:"class"`
	OID        uint32 `json:"oid"`
	SubID      int    `json:"sub_id"`
	Kind       string `json:"kind"`
	Identity   string `json:"identity"`
	Definition string `json:"definition"`
}

func decodeFullJSON(r io.Reader, dst any, max int) error {
	if r == nil {
		return fmt.Errorf("full-table: nil reader")
	}
	b, err := io.ReadAll(io.LimitReader(r, int64(max)+1))
	if err != nil {
		return err
	}
	if len(b) > max {
		return fmt.Errorf("full-table: input byte limit")
	}
	l := DefaultLimits()
	l.MaxBytes = max
	l.MaxTokens = 1 << 18
	l.MaxDepth = 12
	return strictJSON(b, dst, l)
}
func DecodeFullTableOptions(r io.Reader) (FullTableOptions, error) {
	var o FullTableOptions
	err := decodeFullJSON(r, &o, MaxTransitionOptionsBytes)
	return o, err
}
func DecodeFullTableEvidence(r io.Reader) (FullTableEvidence, error) {
	var e FullTableEvidence
	err := decodeFullJSON(r, &e, MaxFullTableEvidenceBytes)
	return e, err
}

type fullTableModel struct {
	m            *transitionModel
	o            FullTableOptions
	e            FullTableEvidence
	roles        map[uint32]string
	namespaces   map[string]uint32
	cols         map[int]Column
	facts        map[int]FullColumnFacts
	defaults     map[int]string
	constraints  map[uint32]FullConstraint
	checks       map[uint32]string
	indexes      map[uint32]Index
	indexFacts   map[uint32]FullIndexFacts
	sequences    map[uint32]FullSequence
	archive      map[uint32]string
	evidenceHash string
}

// GenerateFullTableTransition emits a genuine new-table INSERT SELECT, verifies
// every column, then proposes a rename-and-rebind sequence. It executes nothing.
// Unsupported or incomplete contracts produce concrete blocked gates and withhold
// every proposed mutation. All human operational gates remain unresolved.
func GenerateFullTableTransition(ctx context.Context, c CatalogSnapshot, o FullTableOptions, e FullTableEvidence) (TransitionBundle, error) {
	if ctx == nil {
		return TransitionBundle{}, fmt.Errorf("full-table: nil context")
	}
	if err := ctx.Err(); err != nil {
		return TransitionBundle{}, err
	}
	limits := DefaultLimits()
	limits.MaxBytes = MaxFullTableEvidenceBytes
	limits.MaxTokens = 1 << 18
	limits.MaxDepth = 12
	if err := budgetValue(e, limits); err != nil {
		return TransitionBundle{}, err
	}
	if o.Transition.Approach != FullTableCopy || o.Transition.Mode != TransitionSingle || o.Transition.ConcurrentIndexes || o.Transition.ShadowColumn != "" || o.Transition.StateSchema != "" || o.Transition.StateTable != "" || o.Transition.BatchSize != 0 {
		return TransitionBundle{}, fmt.Errorf("full-table: explicit single-transaction copy/swap without shadow/batch/concurrent options required")
	}
	if !transitionName(o.CopyTable) || !transitionName(o.BackupTable) || o.CopyTable == o.BackupTable || len(o.ArchiveNames) > 256 {
		return TransitionBundle{}, fmt.Errorf("full-table: invalid copy/backup/index names")
	}
	// Bound programmatic inputs as well as strict JSON. No SQL definitions are
	// rendered before their typed validation; JSON strings remain evidence only.
	raw, err := json.Marshal(e)
	if err != nil || len(raw) > MaxFullTableEvidenceBytes {
		return TransitionBundle{}, fmt.Errorf("full-table: evidence byte limit")
	}
	// Round-trip structural validation also catches nil arrays and invalid strings.
	var owned FullTableEvidence
	l := DefaultLimits()
	l.MaxBytes = MaxFullTableEvidenceBytes
	l.MaxTokens = 1 << 18
	l.MaxDepth = 12
	if err = strictJSON(raw, &owned, l); err != nil {
		return TransitionBundle{}, fmt.Errorf("full-table evidence: %w", err)
	}
	o.ArchiveNames = append([]FullArchiveName{}, o.ArchiveNames...)
	sort.Slice(o.ArchiveNames, func(i, j int) bool { return o.ArchiveNames[i].IndexOID < o.ArchiveNames[j].IndexOID })
	base := o.Transition
	base.Approach = RetainArrayCast
	return generateTransition(ctx, c, base, func(m *transitionModel) (TransitionBundle, error) {
		o.Transition = m.o
		o.Transition.Approach = FullTableCopy
		f := fullTableModel{m: m, o: o, e: owned, evidenceHash: sqlHash(string(raw))}
		if err := f.validate(); err != nil {
			return TransitionBundle{}, err
		}
		return f.render(ctx)
	})
}

// The general inspector intentionally gates sequence/default allocation for local
// import. A full-table plan copies explicit keys and separately validates every
// captured default and sequence instead. No other inspector finding is waived.
func fullTableColumnEligible(r Report, object string) bool {
	for _, f := range r.Findings {
		if f.Object == object && f.Code != "primary_key_generation_requires_review" && f.Code != "dependent_object_requires_review" && f.Code != "unknown_dependency_kind" {
			return false
		}
	}
	return true
}
func fullTableIndexEligible(r Report, object string) bool {
	for _, f := range r.Findings {
		if f.Object == object && f.Code != "dependent_object_requires_review" && f.Code != "unknown_dependency_kind" && f.Code != "efsearch_scope_change" {
			return false
		}
	}
	return true
}

// FullLegacyFamily captures the implicit operator family and internal support
// rows created by the pinned legacy CREATE OPERATOR CLASS statements. They are
// removed by the reviewed extension transition, never recreated from raw SQL.
type FullLegacyFamily struct {
	OID              uint32 `json:"oid"`
	Schema           string `json:"schema"`
	Name             string `json:"name"`
	MethodOID        uint32 `json:"method_oid"`
	OpClassOID       uint32 `json:"opclass_oid"`
	AMOpOID          uint32 `json:"amop_oid"`
	AMProcOID        uint32 `json:"amproc_oid"`
	Strategy         int    `json:"strategy"`
	Support          int    `json:"support"`
	LeftOID          uint32 `json:"left_oid"`
	RightOID         uint32 `json:"right_oid"`
	OperatorOID      uint32 `json:"operator_oid"`
	FunctionOID      uint32 `json:"function_oid"`
	SortFamilyOID    uint32 `json:"sort_family_oid"`
	SortFamilySchema string `json:"sort_family_schema"`
	SortFamilyName   string `json:"sort_family_name"`
	SortMethodOID    uint32 `json:"sort_method_oid"`
}
