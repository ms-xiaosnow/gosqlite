package postgres

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

type Finding struct {
	Severity string `json:"severity"`
	Code     string `json:"code"`
	Object   string `json:"object"`
	Message  string `json:"message"`
}
type ColumnReport struct {
	Object         string `json:"object"`
	RelationOID    uint32 `json:"relation_oid"`
	AttNum         int    `json:"attnum"`
	Classification string `json:"classification"`
	Selected       bool   `json:"selected"`
	Dimensions     int    `json:"dimensions"`
	Status         string `json:"status"`
	Validation     string `json:"validation"`
}
type IndexReport struct {
	Object           string         `json:"object"`
	OID              uint32         `json:"oid"`
	Column           string         `json:"column"`
	Status           string         `json:"status"`
	IdentityVerified bool           `json:"identity_verified"`
	Legacy           LegacyOptions  `json:"legacy"`
	ProposedTarget   *TargetOptions `json:"proposed_target"`
	QueryEFSearch    int            `json:"query_ef_search"`
}
type Report struct {
	FormatVersion     int            `json:"format_version"`
	SourceProfile     string         `json:"source_profile"`
	TargetProfile     string         `json:"target_profile"`
	CatalogSHA256     string         `json:"catalog_sha256"`
	CatalogHashKind   string         `json:"catalog_hash_kind"`
	SourceBytesSHA256 string         `json:"source_bytes_sha256,omitempty"`
	ExtractedAt       string         `json:"extracted_at"`
	Status            string         `json:"status"`
	Scope             Scope          `json:"scope"`
	Columns           []ColumnReport `json:"columns"`
	Indexes           []IndexReport  `json:"indexes"`
	Findings          []Finding      `json:"findings"`
	ApprovalPoints    []string       `json:"approval_points"`
}

func severityStatus(s string) string {
	if s == "error" {
		return "blocked"
	}
	return "review_required"
}
func mergeStatus(a, b string) string {
	if a == "blocked" || b == "blocked" {
		return "blocked"
	}
	if a == "review_required" || b == "review_required" {
		return "review_required"
	}
	return "safe"
}
func (r *Report) add(sev, code, obj, msg string) {
	r.Findings = append(r.Findings, Finding{sev, code, obj, msg})
	r.Status = mergeStatus(r.Status, severityStatus(sev))
}
func (r *Report) finish() {
	sort.Slice(r.Columns, func(i, j int) bool { return r.Columns[i].Object < r.Columns[j].Object })
	sort.Slice(r.Indexes, func(i, j int) bool { return r.Indexes[i].Object < r.Indexes[j].Object })
	sort.Slice(r.Findings, func(i, j int) bool {
		a, b := r.Findings[i], r.Findings[j]
		if a.Object != b.Object {
			return a.Object < b.Object
		}
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		if a.Severity != b.Severity {
			return a.Severity < b.Severity
		}
		return a.Message < b.Message
	})
	sort.Strings(r.Scope.Schemas)
	sort.Slice(r.Scope.Coverage, func(i, j int) bool { return r.Scope.Coverage[i].Category < r.Scope.Coverage[j].Category })
	sort.Strings(r.ApprovalPoints)
}

// RenderJSON is deterministic for the same report. It includes no source rows.
func RenderJSON(w io.Writer, r Report) error {
	e := json.NewEncoder(w)
	e.SetIndent("", "  ")
	return e.Encode(r)
}
func md(s string) string {
	replacer := strings.NewReplacer("\\", "\\\\", "`", "\\`", "*", "\\*", "_", "\\_", "[", "\\[", "]", "\\]", "<", "&lt;", ">", "&gt;", "|", "\\|", "\r", " ", "\n", " ")
	return replacer.Replace(s)
}

// RenderMarkdown escapes untrusted catalog identifiers and definitions. Source
// definitions and source row values are deliberately absent from reports.
func RenderMarkdown(w io.Writer, r Report) error {
	var b strings.Builder
	fmt.Fprintf(&b, "# pg_embedding inspection\n\nStatus: %s\n\nSource profile: %s\n\nTarget reference: %s\n\nCatalog SHA-256: %s\n", md(r.Status), md(r.SourceProfile), md(r.TargetProfile), md(r.CatalogSHA256))
	b.WriteString("\n## Columns\n")
	for _, c := range r.Columns {
		fmt.Fprintf(&b, "- %s: %s; selected=%t; dimensions=%d; %s; validation=%s\n", md(c.Object), md(c.Classification), c.Selected, c.Dimensions, md(c.Status), md(c.Validation))
	}
	b.WriteString("\n## Indexes\n")
	for _, i := range r.Indexes {
		fmt.Fprintf(&b, "- %s: %s; identity verified=%t; old m=%d, efconstruction=%d, efsearch=%d\n", md(i.Object), md(i.Status), i.IdentityVerified, i.Legacy.M, i.Legacy.EFConstruction, i.Legacy.EFSearch)
	}
	b.WriteString("\n## Findings\n")
	for _, f := range r.Findings {
		fmt.Fprintf(&b, "- %s / %s / %s: %s\n", md(f.Severity), md(f.Code), md(f.Object), md(f.Message))
	}
	b.WriteString("\n## Approval points\n")
	for _, a := range r.ApprovalPoints {
		fmt.Fprintf(&b, "- %s\n", md(a))
	}
	_, err := io.WriteString(w, b.String())
	return err
}
