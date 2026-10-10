package postgres

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

type boundType struct {
	kind        string
	dims        int
	nullable    bool
	nonNegative bool
	oid         uint32
}
type statementBinder struct {
	cv         *converter
	result     *ConvertedStatement
	relation   Relation
	mapping    TableMapping
	alias      string
	parameters map[int]BindTransform
	aliases    map[string]boundType
}

func (b *statementBinder) failure(code string, span SQLSpan, message string) error {
	return conversionError(code, span, message)
}
func (b *statementBinder) warning(code string, span SQLSpan, message string) {
	for _, f := range b.result.Findings {
		if f.Code == code && f.Span == span {
			return
		}
	}
	b.result.Findings = append(b.result.Findings, SQLFinding{code, span, message})
}
func (b *statementBinder) edit(span SQLSpan, value string) {
	if b.cv.source[span.Start:span.End] != value {
		b.result.Edits = append(b.result.Edits, SQLEdit{span, value})
	}
}
func (b *statementBinder) raw(span SQLSpan) string { return b.cv.source[span.Start:span.End] }

// Compound syntax changes keep every original trivia byte exactly once and in
// its original relative order, following the replacement's significant tokens.
func (b *statementBinder) withTrivia(span SQLSpan, replacement string) string {
	var out strings.Builder
	out.WriteString(replacement)
	i := sort.Search(len(b.cv.tokens), func(i int) bool { return b.cv.tokens[i].Start >= span.Start })
	for ; i < len(b.cv.tokens) && b.cv.tokens[i].End <= span.End; i++ {
		t := b.cv.tokens[i]
		if t.Kind == pgWhitespace || t.Kind == pgComment {
			out.WriteString(b.cv.source[t.Start:t.End])
		}
	}
	return out.String()
}
func (b *statementBinder) compose(span SQLSpan, edits ...SQLEdit) (string, error) {
	return applySQLEdits(b.cv.source, span, edits)
}
func (b *statementBinder) targetTable() string {
	if b.cv.opts.Target == TargetLocal {
		return quoteIdent(b.mapping.TargetName)
	}
	return objectKey(b.mapping.TargetSchema, b.mapping.TargetName)
}
func (b *statementBinder) bindTable(name []pgToken, alias pgToken) error {
	span := pgNameSpan(name)
	for _, rel := range b.cv.catalog.Relations {
		match := len(name) == 1 && name[0].Text == rel.Name || len(name) == 2 && name[0].Text == rel.Schema && name[1].Text == rel.Name
		if match {
			if b.relation.OID != 0 {
				return b.failure("ambiguous_relation", span, "Unqualified relation identity is ambiguous.")
			}
			b.relation = rel
		}
	}
	if b.relation.OID == 0 {
		return b.failure("unknown_relation", span, "Relation is absent from the supplied catalog.")
	}
	b.mapping = b.cv.tables[b.relation.OID]
	if b.mapping.RelationOID == 0 {
		return b.failure("table_mapping_required", span, "An explicit relation-to-target mapping is required.")
	}
	if b.relation.Kind != "r" || b.relation.Persistence != "p" || b.relation.RLS || len(b.relation.Parents) != 0 {
		return b.failure("relation_shape_unsupported", span, "Only ordinary permanent non-inherited tables without RLS are supported.")
	}
	names := map[string]bool{}
	for _, c := range b.relation.Columns {
		if c.Dropped {
			continue
		}
		n := c.Name
		if b.cv.opts.Target == TargetLocal {
			n = strings.ToLower(n)
		}
		if names[n] {
			return b.failure("local_name_collision", span, "Local case-insensitive column names collide.")
		}
		names[n] = true
	}
	if alias.Kind != pgEOF {
		b.alias = alias.Text
	}
	b.edit(span, b.withTrivia(span, b.targetTable()))
	return nil
}
func (b *statementBinder) column(token pgToken) (Column, boundType, error) {
	for _, c := range b.relation.Columns {
		if c.Name != token.Text || c.Dropped {
			continue
		}
		typ, err := b.columnType(c, tokenSpan(token))
		return c, typ, err
	}
	return Column{}, boundType{}, b.failure("unknown_column", tokenSpan(token), "Column does not resolve to the bound relation.")
}
func (b *statementBinder) columnType(c Column, span SQLSpan) (boundType, error) {
	t := b.cv.types[c.TypeOID]
	out := boundType{nullable: !c.NotNull, oid: c.TypeOID}
	selected, ok := b.cv.selected[conversionColumnKey(b.relation.OID, c.AttNum)]
	if ok {
		if selected.Status != "safe" || selected.Dimensions < 1 || selected.Dimensions > vector.MaxDimensions || c.TypeOID != 1021 {
			return out, b.failure("selected_column_blocked", span, "Selected vector column has unresolved inspection findings or dimensions.")
		}
		out.kind = "legacy"
		out.dims = selected.Dimensions
		return out, nil
	}
	if t.Schema != "pg_catalog" || t.Kind != "b" || t.BaseOID != 0 || (c.TypeOID != 1021 && t.ElementOID != 0) {
		return out, b.failure("column_type_unsupported", span, "Column type identity is outside the supported primitive subset.")
	}
	switch {
	case t.OID == 20 && t.Name == "int8", t.OID == 21 && t.Name == "int2", t.OID == 23 && t.Name == "int4":
		out.kind = "integer"
	case t.OID == 700 && t.Name == "float4", t.OID == 701 && t.Name == "float8":
		out.kind = "real"
	case t.OID == 25 && t.Name == "text":
		out.kind = "text"
	case t.OID == 16 && t.Name == "bool":
		out.kind = "bool"
	case t.OID == 1021 && t.Name == "_float4" && t.ElementOID == 700:
		out.kind = "array"
	case t.OID == 1560 && t.Name == "bit":
		out.kind = "bit"
		out.dims = c.TypeMod
		if out.dims < 1 || out.dims > 64000 {
			return out, b.failure("bit_width_required", span, "A fixed bit width in 1..64000 is required.")
		}
	default:
		return out, b.failure("column_type_unsupported", span, "Column type is outside the supported primitive subset.")
	}
	return out, nil
}
func (b *statementBinder) columnExpr(e *pgExpr, allowAlias bool) (boundType, string, error) {
	if len(e.Name) == 0 {
		return boundType{}, "", b.failure("unknown_column", e.Span, "Empty column reference.")
	}
	name := e.Name[len(e.Name)-1]
	if allowAlias && len(e.Name) == 1 {
		if t, ok := b.aliases[name.Text]; ok {
			return t, b.raw(e.Span), nil
		}
	}
	switch len(e.Name) {
	case 1:
	case 2:
		qualifier := b.relation.Name
		if b.alias != "" {
			qualifier = b.alias
		}
		if e.Name[0].Text != qualifier {
			return boundType{}, "", b.failure("unknown_qualifier", e.Span, "Column qualifier does not identify the bound relation.")
		}
	case 3:
		if b.alias != "" || e.Name[0].Text != b.relation.Schema || e.Name[1].Text != b.relation.Name {
			return boundType{}, "", b.failure("unknown_qualifier", e.Span, "Column qualifier does not identify the bound relation.")
		}
	default:
		return boundType{}, "", b.failure("unknown_qualifier", e.Span, "Column reference is outside the single-table subset.")
	}
	_, typ, err := b.column(name)
	if err != nil {
		return typ, "", err
	}
	if b.cv.opts.Target == TargetLocal && typ.kind == "array" {
		return typ, "", b.failure("unselected_array", e.Span, "Unselected arrays have no local SQL representation.")
	}
	text := quoteIdent(name.Text)
	if len(e.Name) > 1 {
		qualifier := b.mapping.TargetName
		if b.alias != "" {
			qualifier = b.alias
		}
		text = quoteIdent(qualifier) + "." + text
		if b.cv.opts.Target == TargetPostgres && len(e.Name) == 3 {
			text = quoteIdent(b.mapping.TargetSchema) + "." + text
		}
	}
	// An unqualified column requires no spelling change; quoted identity and
	// unrelated bytes remain untouched. Qualified mappings are deliberate edits.
	if len(e.Name) == 1 {
		text = b.raw(e.Span)
	} else {
		text = b.withTrivia(e.Span, text)
	}
	return typ, text, nil
}
func (b *statementBinder) hint(e *pgExpr) (boundType, error) {
	if e == nil {
		return boundType{}, nil
	}
	switch e.Kind {
	case "column":
		t, _, err := b.columnExpr(e, false)
		return t, err
	case "paren":
		return b.hint(e.Left)
	case "cast":
		t, err := b.hint(e.Left)
		if err != nil {
			return t, err
		}
		if t.kind == "legacy" {
			return t, nil
		}
		return b.declaredType(e.Type)
	case "null":
		return boundType{kind: "null", nullable: true}, nil
	case "number":
		if strings.ContainsAny(e.Token.Text, ".eE") {
			return boundType{kind: "real"}, nil
		}
		n, err := strconv.ParseInt(e.Token.Text, 10, 64)
		if err != nil {
			return boundType{kind: "integer"}, nil
		}
		oid := uint32(23)
		if n > 2147483647 {
			oid = 20
		}
		return boundType{kind: "integer", oid: oid}, nil
	case "boolean":
		return boundType{kind: "bool"}, nil
	case "array":
		return boundType{kind: "array"}, nil
	}
	return boundType{kind: "unknown", nullable: true}, nil
}
func (b *statementBinder) declaredType(t *pgType) (boundType, error) {
	if t == nil || len(t.Name) < 1 || len(t.Name) > 2 {
		return boundType{}, b.failure("unknown_type", SQLSpan{}, "Missing or unsupported type name.")
	}
	qualified := false
	if len(t.Name) == 2 {
		for i := sort.Search(len(b.cv.tokens), func(i int) bool { return b.cv.tokens[i].Start >= t.Name[0].End }); i < len(b.cv.tokens) && b.cv.tokens[i].End <= t.Name[1].Start; i++ {
			tok := b.cv.tokens[i]
			if tok.Kind == pgSymbol && tok.Text == "." {
				qualified = true
			}
		}
	}
	var oid uint32
	// SQL grammar aliases are built-ins only when unquoted and unqualified.
	// A quoted or qualified name is a catalog typname, not a keyword alias.
	if len(t.Name) == 1 && !t.Name[0].Quoted {
		switch t.Name[0].Text {
		case "integer", "int":
			oid = 23
		case "bigint":
			oid = 20
		case "smallint":
			oid = 21
		case "real":
			oid = 700
		case "boolean":
			oid = 16
		case "bit":
			oid = 1560
		}
	} else if len(t.Name) == 2 && !qualified && !t.Name[0].Quoted && !t.Name[1].Quoted && t.Name[0].Text == "double" && t.Name[1].Text == "precision" {
		oid = 701
	}
	if oid == 0 {
		if len(t.Name) == 2 && !qualified {
			return boundType{}, b.failure("unknown_type", t.Span, "Unknown type syntax is not an SQL built-in alias.")
		}
		matches := 0
		for _, actual := range b.cv.catalog.Types {
			if actual.Name == t.Name[len(t.Name)-1].Text && (len(t.Name) == 1 || actual.Schema == t.Name[0].Text) {
				matches++
				oid = actual.OID
			}
		}
		if matches != 1 {
			return boundType{}, b.failure("type_identity_ambiguous", t.Span, "Named type must resolve uniquely to an actual supplied catalog typname.")
		}
	}
	actual := b.cv.types[oid]
	if actual.Schema != "pg_catalog" || actual.Kind != "b" || actual.BaseOID != 0 || actual.ElementOID != 0 {
		return boundType{}, b.failure("type_identity_unsupported", t.Span, "Type is not a verified pg_catalog scalar base type.")
	}
	var out boundType
	out.oid = oid
	switch {
	case oid == 20 && actual.Name == "int8", oid == 21 && actual.Name == "int2", oid == 23 && actual.Name == "int4":
		out.kind = "integer"
	case oid == 700 && actual.Name == "float4", oid == 701 && actual.Name == "float8":
		out.kind = "real"
	case oid == 25 && actual.Name == "text":
		out.kind = "text"
	case oid == 16 && actual.Name == "bool":
		out.kind = "bool"
	case oid == 1560 && actual.Name == "bit":
		out.kind = "bit"
	default:
		return boundType{}, b.failure("unknown_type", t.Span, "Unknown types, domains, serial and identity mappings require review.")
	}
	if t.ArrayDims != 0 {
		arr := b.cv.types[1021]
		if t.ArrayDims != 1 || oid != 700 || len(t.Args) != 0 || arr.Schema != "pg_catalog" || arr.Name != "_float4" || arr.ElementOID != 700 || arr.BaseOID != 0 || arr.Kind != "b" {
			return boundType{}, b.failure("array_type_unsupported", t.Span, "Only verified one-dimensional real[] is recognized.")
		}
		return boundType{kind: "array", oid: 1021}, nil
	}
	if out.kind == "bit" && len(t.Args) == 1 {
		n, err := strconv.Atoi(t.Args[0].Text)
		if err == nil && n >= 1 && n <= 64000 {
			out.dims = n
			return out, nil
		}
	}
	if len(t.Args) > 0 || out.kind == "bit" {
		return boundType{}, b.failure("type_modifier_unsupported", t.Span, "Type modifier is outside the supported subset; bit requires an exact width.")
	}
	return out, nil
}
func (b *statementBinder) vectorType(dims int) string {
	text := fmt.Sprintf("vector(%d)", dims)
	if b.cv.opts.Target == TargetPostgres {
		return quoteIdent(b.cv.opts.VectorSchema) + "." + text
	}
	return strings.ToUpper(text)
}
func (b *statementBinder) vectorLiteral(v vector.Vector) string {
	text := "'" + v.String() + "'"
	if b.cv.opts.Target == TargetPostgres {
		text += "::" + b.vectorType(v.Dims())
	}
	return text
}
func (b *statementBinder) exprRoot(e *pgExpr, expected boundType) (boundType, error) {
	text, typ, err := b.expression(e, expected)
	if err == nil {
		b.edit(e.Span, text)
	}
	return typ, err
}
func (b *statementBinder) statement(s *pgStatement) error {
	var err error
	switch s.Kind {
	case "select":
		err = b.selectStatement(s.Select)
	case "insert":
		err = b.insertStatement(s.Insert)
	case "update":
		err = b.updateStatement(s.Update)
	case "create_table":
		err = b.createTable(s.CreateTable)
	case "create_index":
		err = b.createIndex(s.CreateIndex)
	default:
		err = b.failure("unsupported_statement", s.Span, "Only SELECT, INSERT VALUES, UPDATE, CREATE TABLE and CREATE INDEX are supported.")
	}
	if err != nil {
		return err
	}
	for _, p := range b.parameters {
		b.result.Bindings = append(b.result.Bindings, p)
	}
	sort.Slice(b.result.Bindings, func(i, j int) bool { return b.result.Bindings[i].Parameter < b.result.Bindings[j].Parameter })
	return nil
}
func (b *statementBinder) selectStatement(s *pgSelect) error {
	if err := b.bindTable(s.From, s.Alias); err != nil {
		return err
	}
	aliasNames := map[string]bool{}
	for _, c := range s.Columns {
		if c.Expr.Kind == "star" {
			return b.failure("star_projection_unsupported", c.Expr.Span, "Explicit columns are required to review changed vector representations.")
		}
		text, typ, err := b.expression(c.Expr, boundType{})
		if err != nil {
			return err
		}
		label := projectionLabel(c)
		aliasKey := label
		if b.cv.opts.Target == TargetLocal {
			aliasKey = strings.ToLower(aliasKey)
			for _, sourceColumn := range b.relation.Columns {
				if !sourceColumn.Dropped && sourceColumn.Name != label && strings.EqualFold(sourceColumn.Name, label) {
					return b.failure("local_alias_column_collision", c.Span, "Case-insensitive local output-label lookup would shadow a differently spelled source column.")
				}
			}
		}
		if aliasNames[aliasKey] {
			return b.failure("ambiguous_alias", c.Span, "Duplicate explicit or implicit projection labels require explicit distinct aliases.")
		}
		aliasNames[aliasKey] = true
		b.aliases[label] = typ
		if c.Alias.Kind == pgEOF && c.Expr.Kind != "column" {
			text += " AS " + quoteIdent(label)
		}
		b.edit(c.Expr.Span, text)
	}

	if s.Where != nil {
		if _, err := b.exprRoot(s.Where, boundType{kind: "bool"}); err != nil {
			return err
		}
	}
	for _, o := range s.OrderBy {
		var text string
		var typ boundType
		var err error
		if o.Expr.Kind == "column" {
			typ, text, err = b.columnExpr(o.Expr, true)
		} else {
			text, typ, err = b.expression(o.Expr, boundType{})
		}
		if err != nil {
			return err
		}
		if typ.kind == "legacy" || typ.kind == "array" || typ.kind == "bit" || typ.kind == "unknown" || o.Expr.Kind == "number" {
			return b.failure("order_expression_unsupported", o.Expr.Span, "Ordering requires a scalar expression or a unique output alias, not an ordinal or vector.")
		}
		if b.cv.opts.Target == TargetLocal {
			if typ.kind == "text" {
				return b.failure("text_collation_unverified", o.Expr.Span, "Local text ordering requires a verified source collation contract absent from this catalog profile.")
			}
			// PostgreSQL defaults ASC NULLS LAST and DESC NULLS FIRST; SQLite
			// defaults are opposite. A leading NULL key retains rows and tie keys.
			nulls := o.Nulls
			if nulls == "" {
				nulls = "last"
				if o.Desc {
					nulls = "first"
				}
			}
			needsKey := typ.nullable && (nulls == "last" && !o.Desc || nulls == "first" && o.Desc)
			if needsKey {
				keyText := text
				if o.Expr.Kind == "column" && len(o.Expr.Name) == 1 {
					for _, projected := range s.Columns {
						if projectionLabel(projected) == o.Expr.Name[0].Text {
							keyText, _, err = b.expression(projected.Expr, boundType{})
							if err != nil {
								return err
							}
							break
						}
					}
				}
				clean, err := stripSQLTrivia(keyText)
				if err != nil {
					return err
				}
				direction := "ASC"
				if nulls == "first" {
					direction = "DESC"
				}
				text = "(" + clean + ") IS NULL " + direction + ", " + text
				b.warning("null_order_exact_sort", o.Span, "Added a NULL ordering key to preserve PostgreSQL rows and ordering; narrow ANN/top-k recognition may be disabled.")
			}
			if o.Nulls != "" {
				b.edit(o.NullsSpan, b.withTrivia(o.NullsSpan, ""))
			}
		}
		b.edit(o.Expr.Span, text)
	}
	if s.Limit != nil {
		limit := s.Limit
		for limit.Kind == "paren" {
			limit = limit.Left
		}
		if limit.Kind != "parameter" {
			text, ok := constantArrayNumber(limit)
			n, e := strconv.ParseInt(text, 10, 64)
			if !ok || e != nil || n < 0 {
				return b.failure("limit_semantics_unsupported", s.Limit.Span, "LIMIT requires a nonnegative integer literal or a constrained parameter.")
			}
		}
		if _, err := b.exprRoot(s.Limit, boundType{kind: "integer", oid: 20, nonNegative: true}); err != nil {
			return err
		}
	}
	return nil
}
func stripSQLTrivia(text string) (string, error) {
	ts, _, err := pgScan(text, SQLLimits{})
	if err != nil {
		return "", err
	}
	var out strings.Builder
	for _, t := range ts {
		if t.Kind == pgComment {
			out.WriteByte(' ')
		} else if t.Kind != pgEOF {
			out.WriteString(text[t.Start:t.End])
		}
	}
	return out.String(), nil
}
func (b *statementBinder) insertStatement(s *pgInsert) error {
	if err := b.bindTable(s.Table, pgToken{}); err != nil {
		return err
	}
	columns := make([]Column, len(s.Columns))
	types := make([]boundType, len(s.Columns))
	seen := map[int]bool{}
	for i, c := range s.Columns {
		col, typ, err := b.column(c)
		if err != nil {
			return err
		}
		if seen[col.AttNum] {
			return b.failure("duplicate_column", tokenSpan(c), "Duplicate INSERT column.")
		}
		seen[col.AttNum] = true
		columns[i], types[i] = col, typ
	}
	for _, c := range b.relation.Columns {
		if c.Dropped {
			continue
		}
		if !seen[c.AttNum] && (c.NotNull && c.Default == "" || containsInt(b.relation.PrimaryKey, c.AttNum)) {
			return b.failure("missing_required_column", s.Span, "INSERT must supply every source primary-key and required no-default column explicitly.")
		}
	}
	for _, row := range s.Rows {
		if len(row) != len(columns) {
			return b.failure("values_arity", s.Span, "VALUES arity differs from the explicit column list.")
		}
		for i, e := range row {
			if err := b.primaryKeyValue(columns[i], e); err != nil {
				return err
			}
			if _, err := b.exprRoot(e, types[i]); err != nil {
				return err
			}
		}
	}
	return nil
}
func (b *statementBinder) updateStatement(s *pgUpdate) error {
	if err := b.bindTable(s.Table, s.Alias); err != nil {
		return err
	}
	if b.cv.opts.Target == TargetLocal && s.Alias.Kind != pgEOF {
		return b.failure("update_alias_unsupported", tokenSpan(s.Alias), "Local UPDATE aliases are not implemented.")
	}
	seen := map[int]bool{}
	for _, a := range s.Assignments {
		c, typ, err := b.column(a.Column)
		if err != nil {
			return err
		}
		if seen[c.AttNum] {
			return b.failure("duplicate_assignment", a.Span, "Duplicate UPDATE assignment.")
		}
		seen[c.AttNum] = true
		if err = b.primaryKeyValue(c, a.Expr); err != nil {
			return err
		}
		if _, err = b.exprRoot(a.Expr, typ); err != nil {
			return err
		}
	}
	if s.Where != nil {
		_, err := b.exprRoot(s.Where, boundType{kind: "bool"})
		return err
	}
	return nil
}
func containsInt(v []int, n int) bool {
	for _, x := range v {
		if x == n {
			return true
		}
	}
	return false
}
func (b *statementBinder) createTable(s *pgCreateTable) error {
	if err := b.bindTable(s.Name, pgToken{}); err != nil {
		return err
	}
	if s.IfNotExists {
		return b.failure("if_not_exists_requires_review", s.Span, "IF NOT EXISTS cannot establish that an existing target schema matches.")
	}
	seen := map[int]bool{}
	for _, def := range s.Columns {
		c, typ, err := b.column(def.Name)
		if err != nil {
			return err
		}
		decl, err := b.declaredType(def.Type)
		if err != nil {
			return err
		}
		if seen[c.AttNum] {
			return b.failure("duplicate_column", def.Span, "Duplicate column declaration.")
		}
		seen[c.AttNum] = true
		if c.TypeOID != decl.oid || decl.kind == "bit" && decl.dims != typ.dims {
			return b.failure("catalog_type_drift", def.Type.Span, "Declaration differs from catalog type identity or width.")
		}
		isPK := containsInt(b.relation.PrimaryKey, c.AttNum)
		if def.PrimaryKey != isPK || (def.NotNull || def.PrimaryKey) != c.NotNull || len(b.relation.PrimaryKey) > 1 {
			return b.failure("catalog_constraint_drift", def.Span, "Primary-key/nullability declaration differs from the bound catalog.")
		}
		if c.Generated != "" || c.Identity != "" {
			return b.failure("generated_identity_unsupported", def.Span, "Generated and identity columns need a separate explicit mapping.")
		}
		if (def.Default == nil) != (c.Default == "") || def.Default != nil && strings.TrimSpace(b.raw(def.Default.Span)) != strings.TrimSpace(c.Default) {
			return b.failure("catalog_default_drift", def.Span, "Default declaration differs from the supplied catalog.")
		}
		replacement := ""
		switch typ.kind {
		case "legacy":
			replacement = b.vectorType(typ.dims)
		case "array":
			if b.cv.opts.Target == TargetLocal {
				return b.failure("unselected_array", def.Type.Span, "Unselected arrays cannot be silently dropped or retyped locally.")
			}
		case "integer":
			if b.cv.opts.Target == TargetLocal {
				replacement = "INTEGER"
				if !isPK {
					b.warning("integer_width_review", def.Type.Span, "Local INTEGER is signed 64-bit; source narrower integer write-range enforcement remains an application responsibility.")
				}
			}
		case "real":
			if b.cv.opts.Target == TargetLocal {
				replacement = "REAL"
				if c.TypeOID == 700 {
					b.warning("real_scalar_width_review", def.Type.Span, "Local scalar REAL is float64; only selected vector components retain source binary32 rounding.")
				}
			}
		case "text":
			if b.cv.opts.Target == TargetLocal {
				replacement = "TEXT"
			}
		case "bit":
			if b.cv.opts.Target == TargetLocal {
				replacement = fmt.Sprintf("BIT(%d)", typ.dims)
			}
		default:
			return b.failure("declaration_unsupported", def.Type.Span, "This column declaration has no supported local mapping.")
		}
		if replacement != "" {
			b.edit(def.Type.Span, b.withTrivia(def.Type.Span, replacement))
		}
		if def.Default != nil {
			if _, err = b.exprRoot(def.Default, typ); err != nil {
				return err
			}
		}
	}
	for _, c := range b.relation.Columns {
		if !c.Dropped && !seen[c.AttNum] {
			return b.failure("catalog_columns_omitted", s.Span, "CREATE TABLE must include every bound source column; columns cannot be silently omitted.")
		}
	}
	return nil
}
func (b *statementBinder) createIndex(s *pgCreateIndex) error {
	if err := b.bindTable(s.Table, pgToken{}); err != nil {
		return err
	}
	// A single compound edit owns the full CREATE INDEX body.
	b.result.Edits = nil
	if s.Method.Text != "hnsw" || len(s.Name) != 1 {
		return b.failure("index_shape_unsupported", s.Span, "Only unqualified index names and the legacy hnsw method are supported.")
	}
	_, typ, err := b.column(s.Column)
	if err != nil {
		return err
	}
	if typ.kind != "legacy" {
		return b.failure("index_column_not_legacy", tokenSpan(s.Column), "HNSW conversion requires a selected legacy real[] vector column.")
	}
	var chosen *OpClass
	for i := range b.cv.catalog.OpClasses {
		oc := &b.cv.catalog.OpClasses[i]
		match := len(s.OpClass) == 0 && oc.Default || len(s.OpClass) == 1 && oc.Name == s.OpClass[0].Text || len(s.OpClass) == 2 && oc.Schema == s.OpClass[0].Text && oc.Name == s.OpClass[1].Text
		if !match {
			continue
		}
		if chosen != nil {
			return b.failure("ambiguous_opclass", s.Span, "Opclass identity is ambiguous.")
		}
		chosen = oc
	}
	if chosen == nil {
		return b.failure("unknown_opclass", s.Span, "Source operator class is absent from the supplied catalog.")
	}
	profile, ok := legacyClasses[chosen.Name]
	if !ok || !b.verifiedClass(*chosen) {
		return b.failure("opclass_identity_mismatch", s.Span, "Source opclass, method, operator, function and extension identities do not match the pinned profile.")
	}
	options := make([]string, len(s.Options))
	for i, o := range s.Options {
		options[i] = o.Name.Text + "=" + o.Value.Text
	}
	opts, oe := parseLegacyOptions(options)
	if oe != nil {
		return b.failure(oe.Code, s.Span, oe.Message)
	}
	if opts.Dimensions != typ.dims {
		return b.failure("conflicting_dimensions", s.Span, "Index dimensions differ from the selected column.")
	}
	if opts.Dimensions > 2000 || opts.M < 2 || opts.M > 100 || opts.EFConstruction < 4 || opts.EFConstruction > 1000 || opts.EFConstruction < 2*opts.M || opts.EFSearch > 1000 {
		return b.failure("target_option_constraint", s.Span, "Source options violate target bounds; no clamping or automatic retuning is performed.")
	}
	name := s.Name[0].Text
	if b.cv.opts.Target == TargetLocal && (strings.EqualFold(name, b.mapping.TargetName) || strings.HasPrefix(strings.ToLower(name), "sqlite_") || strings.HasPrefix(strings.ToLower(name), "__gosqlite_")) {
		return b.failure("local_name_collision", tokenSpan(s.Name[0]), "Index name collides with a target table or reserved namespace.")
	}
	sql := ""
	setting := QuerySetting{Index: name, Value: opts.EFSearch}
	if b.cv.opts.Target == TargetLocal {
		sql = fmt.Sprintf("CREATE INDEX %s ON %s(%s) USING hnsw WITH(metric='%s',opclass='%s',m=%d,ef_construction=%d)", quoteIdent(name), b.targetTable(), quoteIdent(s.Column.Text), profile.metric, profile.target, opts.M, opts.EFConstruction)
		setting.Name = "EFSearch"
		setting.Application = "Set sqlite.VectorSearchOptions.EFSearch for native searches. Local SQL has no per-query EFSearch setting; this artifact is not automatically applied."
	} else {
		sql = fmt.Sprintf("CREATE INDEX %s ON %s USING hnsw (%s %s.%s) WITH (m=%d,ef_construction=%d)", quoteIdent(name), b.targetTable(), quoteIdent(s.Column.Text), quoteIdent(b.cv.opts.VectorSchema), quoteIdent(profile.target), opts.M, opts.EFConstruction)
		setting.Name = "hnsw.ef_search"
		setting.Application = "Review session/transaction scope for queries using this index; apply SET LOCAL only inside the application's existing transaction."
		setting.ReviewSQL = fmt.Sprintf("SET LOCAL hnsw.ef_search = %d;", opts.EFSearch)
	}
	b.edit(s.BodySpan, b.withTrivia(s.BodySpan, sql))
	b.result.Settings = append(b.result.Settings, setting)
	b.warning("efsearch_application_setting", s.Span, "Legacy per-index efsearch becomes a separate application query setting; it is not a target CREATE INDEX option.")
	if b.cv.opts.Target == TargetPostgres {
		b.warning("postgresql_index_review_only", s.Span, "Review target extension ownership/version, hnsw access-method transition and namespace before execution; no PostgreSQL statement was executed.")
	}
	return nil
}
func (b *statementBinder) legacyExtension() uint32 {
	for _, e := range b.cv.catalog.Extensions {
		if e.Name == "embedding" && e.Version == "0.3.6" {
			return e.OID
		}
	}
	return 0
}
func (b *statementBinder) verifiedFunction(f Function, expectedName string) bool {
	ext := b.legacyExtension()
	if ext == 0 || f.ExtensionOID != ext || f.Name != expectedName || len(f.ArgumentOIDs) != 2 || f.ArgumentOIDs[0] != 1021 || f.ArgumentOIDs[1] != 1021 || f.ReturnOID != 700 {
		return false
	}
	for _, e := range b.cv.catalog.Extensions {
		if e.OID == ext {
			return f.Schema == e.Schema
		}
	}
	return false
}
func (b *statementBinder) verifiedOperator(name string) bool {
	want := ""
	for _, p := range legacyClasses {
		if p.operator == name {
			want = p.function
		}
	}
	count := 0
	valid := false
	for _, op := range b.cv.catalog.Operators {
		if op.Name != name || op.LeftOID != 1021 || op.RightOID != 1021 {
			continue
		}
		count++
		for _, f := range b.cv.catalog.Functions {
			if f.OID == op.FunctionOID && b.verifiedFunction(f, want) && op.ExtensionOID == f.ExtensionOID && op.Schema == f.Schema {
				valid = true
			}
		}
	}
	return count == 1 && valid
}
func (b *statementBinder) verifiedClass(oc OpClass) bool {
	p, ok := legacyClasses[oc.Name]
	if !ok || oc.InputOID != 1021 || oc.ExtensionOID != b.legacyExtension() || oc.Default != (oc.Name == "ann_l2_ops") {
		return false
	}
	methodOK := false
	for _, m := range b.cv.catalog.AccessMethods {
		if m.OID == oc.MethodOID && m.Name == "hnsw" && m.ExtensionOID == oc.ExtensionOID {
			for _, f := range b.cv.catalog.Functions {
				if f.OID == m.HandlerOID && f.Name == "hnsw_handler" && f.Schema == oc.Schema && f.ExtensionOID == oc.ExtensionOID && len(f.ArgumentOIDs) == 1 && f.ArgumentOIDs[0] == 2281 && f.ReturnOID == 325 {
					methodOK = true
				}
			}
		}
	}
	if !methodOK {
		return false
	}
	for _, op := range b.cv.catalog.Operators {
		if op.OID == oc.OperatorOID && op.Name == p.operator && op.ExtensionOID == oc.ExtensionOID && op.Schema == oc.Schema && op.LeftOID == 1021 && op.RightOID == 1021 && op.FunctionOID == oc.FunctionOID {
			for _, f := range b.cv.catalog.Functions {
				if f.OID == oc.FunctionOID && f.Schema == oc.Schema && b.verifiedFunction(f, p.function) {
					return true
				}
			}
		}
	}
	return false
}
func finiteNumber(text string) bool {
	v, err := strconv.ParseFloat(text, 64)
	return err == nil && !math.IsNaN(v) && !math.IsInf(v, 0)
}

func (b *statementBinder) primaryKeyValue(c Column, e *pgExpr) error {
	if b.cv.opts.Target != TargetLocal || !containsInt(b.relation.PrimaryKey, c.AttNum) {
		return nil
	}
	node := e
	for node.Kind == "paren" {
		node = node.Left
	}
	if node.Kind == "parameter" {
		return nil
	}
	text, ok := constantArrayNumber(node)
	if ok && !strings.ContainsAny(text, ".eE") {
		if _, err := strconv.ParseInt(text, 10, 64); err == nil {
			return nil
		}
	}
	return b.failure("primary_key_value_unsupported", e.Span, "Local primary-key assignments require explicit integer literals or constrained parameters; expression NULLs must not allocate rowids.")
}

// PostgreSQL FigureColname for the supported expression subset. Non-column
// results receive explicit source labels so target metadata cannot silently
// rename converted functions/operators. Cast/array projections are rejected
// by the typed binder before this point unless their source label is known.
func projectionLabel(c pgResultColumn) string {
	if c.Alias.Kind != pgEOF {
		return c.Alias.Text
	}
	e := c.Expr
	for e.Kind == "paren" {
		e = e.Left
	}
	if (e.Kind == "column" || e.Kind == "call") && len(e.Name) > 0 {
		return e.Name[len(e.Name)-1].Text
	}
	return "?column?"
}
