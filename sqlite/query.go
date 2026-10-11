package sqlite

import (
	"context"
	"fmt"
	"io"
	"math"
	"strings"

	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

type source interface {
	Next() (int64, []any, error)
	Close() error
}
type constantSource struct{ done bool }

func (s *constantSource) Next() (int64, []any, error) {
	if s.done {
		return 0, nil, io.EOF
	}
	s.done = true
	return 0, nil, nil
}
func (s *constantSource) Close() error { s.done = true; return nil }

type table struct {
	name, alias string
	root        uint32
	columns     []sqlast.ColumnDef
	index       map[string]int
	ipk         int
}

func (d *DB) table(ctx context.Context, reader btree.PageReader, sel *sqlast.Select) (*table, error) {
	if sel.From == "" {
		return nil, nil
	}
	t := &table{name: sel.From, alias: sel.Alias, ipk: -1, index: map[string]int{}}
	if fold(sel.From) == "sqlite_schema" || fold(sel.From) == "sqlite_master" {
		t.root = 1
		for _, c := range []struct{ name, typ string }{{"type", "TEXT"}, {"name", "TEXT"}, {"tbl_name", "TEXT"}, {"rootpage", "INTEGER"}, {"sql", "TEXT"}} {
			t.columns = append(t.columns, sqlast.ColumnDef{Name: c.name, Type: c.typ})
		}
	} else {
		entries, err := btree.ReadSchema(ctx, reader)
		if err != nil {
			return nil, err
		}
		d.observePreparationEntriesLocked(ctx, reader.Header().SchemaCookie, entries)
		found := false
		for _, entry := range entries {
			if fold(entry.Name) != fold(sel.From) {
				continue
			}
			if entry.Type != "table" {
				return nil, fmt.Errorf("%w: querying %s objects", ErrUnsupported, entry.Type)
			}
			stmt, err := sqlast.Parse(entry.SQL)
			if err != nil {
				return nil, fmt.Errorf("%w: table %q definition: %w", ErrUnsupported, sel.From, err)
			}
			create, ok := stmt.(*sqlast.CreateTable)
			if !ok {
				return nil, fmt.Errorf("%w: table definition", ErrUnsupported)
			}
			if fold(create.Name) != fold(entry.Name) || entry.RootPage == 0 {
				return nil, fmt.Errorf("%w: table schema mismatch", ErrCorrupt)
			}
			t.columns = create.Columns
			t.root = entry.RootPage
			found = true
			break
		}
		if !found {
			return nil, fmt.Errorf("no such table: %s", sel.From)
		}
	}
	pkCount := 0
	for i, c := range t.columns {
		key := fold(c.Name)
		if _, ok := t.index[key]; ok {
			return nil, fmt.Errorf("%w: duplicate column in schema", ErrCorrupt)
		}
		t.index[key] = i
		if c.PrimaryKey {
			pkCount++
			if strings.ToUpper(strings.TrimSpace(c.Type)) == "INTEGER" {
				t.ipk = i
			}
		}
	}
	if pkCount != 1 {
		t.ipk = -1
	}
	return t, nil
}

func (t *table) qualifier(q string) bool {
	if q == "" {
		return true
	}
	if t == nil {
		return false
	}
	if t.alias != "" {
		return fold(q) == fold(t.alias)
	}
	return fold(q) == fold(t.name)
}
func (t *table) column(q, name string) (int, error) {
	if t == nil || !t.qualifier(q) {
		return 0, fmt.Errorf("no such column: %s", qualified(q, name))
	}
	if i, ok := t.index[fold(name)]; ok {
		return i, nil
	}
	switch fold(name) {
	case "rowid", "_rowid_", "oid":
		return -1, nil
	}
	return 0, fmt.Errorf("no such column: %s", qualified(q, name))
}
func qualified(q, name string) string {
	if q != "" {
		return q + "." + name
	}
	return name
}
func affinity(typ string) string {
	s := strings.ToUpper(typ)
	switch {
	case strings.Contains(s, "INT"):
		return "INTEGER"
	case strings.Contains(s, "CHAR") || strings.Contains(s, "CLOB") || strings.Contains(s, "TEXT"):
		return "TEXT"
	case s == "" || strings.Contains(s, "BLOB"):
		return "BLOB"
	case strings.Contains(s, "REAL") || strings.Contains(s, "FLOA") || strings.Contains(s, "DOUB"):
		return "REAL"
	default:
		return "NUMERIC"
	}
}

func (d *DB) plan(ctx context.Context, reader btree.PageReader, sel *sqlast.Select, params []any) (*Rows, error) {
	if selectUsesAggregate(sel) {
		copy := *sel
		if copy.Relation == nil && copy.From != "" {
			copy.Relation = &sqlast.TableRelation{Name: copy.From, Alias: copy.Alias, NotIndexed: copy.NotIndexed}
		}
		return d.planRelational(ctx, reader, &copy, params)
	}
	if sel.Relation != nil {
		return d.planRelational(ctx, reader, sel, params)
	}
	t, err := d.table(ctx, reader, sel)
	if err != nil {
		return nil, err
	}
	for i := range sel.Columns {
		sel.Columns[i].Expr = bindBooleans(sel.Columns[i].Expr, t)
	}
	sel.Where = bindBooleans(sel.Where, t)
	for i := range sel.OrderBy {
		sel.OrderBy[i].Expr = bindBooleans(sel.OrderBy[i].Expr, t)
	}
	r := &Rows{db: d, ctx: ctx, table: t, sel: sel, params: params, limit: -1, aliases: map[string]int{}}
	for _, c := range sel.Columns {
		if star, ok := c.Expr.(*sqlast.Star); ok {
			if t == nil || !t.qualifier(star.Table) {
				return nil, fmt.Errorf("no such table for wildcard: %s", star.Table)
			}
			for _, col := range t.columns {
				r.exprs = append(r.exprs, &sqlast.Column{Name: col.Name})
				r.columns = append(r.columns, col.Name)
			}
		} else {
			r.exprs = append(r.exprs, c.Expr)
			label := c.Alias
			if label == "" {
				label = c.Text
				if col, ok := c.Expr.(*sqlast.Column); ok {
					label = columnLabel(t, col)
				}
				if label == "" {
					label = sqlast.ExprName(c.Expr)
				}
			}
			r.columns = append(r.columns, label)
		}
	}
	for i, c := range r.columns {
		key := fold(c)
		if _, exists := r.aliases[key]; !exists {
			r.aliases[key] = i
		}
	}
	// TRUE/FALSE remain fallback names after physical binding. Resolve result
	// aliases only in the clauses where SQLite permits them, never projections.
	sel.Where = bindBooleanAliases(sel.Where, r.aliases)
	for i := range sel.OrderBy {
		sel.OrderBy[i].Expr = bindBooleanAliases(sel.OrderBy[i].Expr, r.aliases)
	}
	for _, expr := range r.exprs {
		if err := validate(expr, t, nil); err != nil {
			return nil, err
		}
	}
	if err := validate(sel.Where, t, r.aliases); err != nil {
		return nil, err
	}
	for _, o := range sel.OrderBy {
		// Bare signed integer constants denote a result-column position.
		if ordinal, ok := integerConstant(o.Expr); ok {
			if ordinal <= 0 || ordinal > int64(len(r.exprs)) {
				return nil, fmt.Errorf("ORDER BY position outside result column range")
			}
			r.order = append(r.order, orderExpr{column: int(ordinal) - 1, desc: o.Desc, nulls: o.Nulls})
			continue
		}
		if name, ok := orderResultName(o.Expr); ok {
			if resultIndex, found := r.aliases[fold(name)]; found {
				r.order = append(r.order, orderExpr{column: resultIndex, desc: o.Desc, nulls: o.Nulls})
				continue
			}
		}
		if err := validate(o.Expr, t, r.aliases); err != nil {
			return nil, err
		}
		r.order = append(r.order, orderExpr{expr: o.Expr, column: -1, desc: o.Desc, nulls: o.Nulls})
	}
	if sel.Limit != nil {
		if err := validate(sel.Limit, nil, nil); err != nil {
			return nil, err
		}
		r.limit, err = limitValue(ctx, sel.Limit, params)
		if err != nil {
			return nil, err
		}
	}
	if sel.Offset != nil {
		if err := validate(sel.Offset, nil, nil); err != nil {
			return nil, err
		}
		r.offset, err = limitValue(ctx, sel.Offset, params)
		if err != nil {
			return nil, err
		}
		if r.offset < 0 {
			r.offset = 0
		}
	}
	if err = d.prepareGramFunctions(ctx, reader, r); err != nil {
		return nil, err
	}
	if err = r.prepareWhere(); err != nil {
		return nil, err
	}
	if err = d.planSource(ctx, reader, r); err != nil {
		return nil, err
	}
	if err = r.prepareVectorRank(); err != nil {
		if r.source != nil {
			r.source.Close()
		}
		return nil, err
	}
	if err = r.prepareANNVector(reader); err != nil {
		if r.source != nil {
			r.source.Close()
		}
		return nil, err
	}
	if err = d.planOrdinarySource(ctx, reader, r); err != nil {
		if r.source != nil {
			r.source.Close()
		}
		return nil, err
	}
	if err = d.planRowIDRange(ctx, reader, r); err != nil {
		if r.source != nil {
			r.source.Close()
		}
		return nil, err
	}
	if err = d.planSecondaryRange(ctx, reader, r); err != nil {
		if r.source != nil {
			r.source.Close()
		}
		return nil, err
	}
	if sel.Explain {
		r.explainDetail = r.queryPlan.Detail()
		r.columns = []string{"id", "parent", "notused", "detail"}
		r.limit, r.offset = -1, 0
		return r, nil
	}
	if len(r.order) > 0 && r.limit != 0 {
		if err = r.materialize(); err != nil {
			if r.source != nil {
				r.source.Close()
			}
			return nil, err
		}
	}
	return r, nil
}

func integerConstant(e sqlast.Expr) (int64, bool) {
	switch x := e.(type) {
	case *sqlast.Literal:
		if x.Boolean {
			return 0, false
		}
		v, ok := x.Value.(int64)
		return v, ok
	case *sqlast.Unary:
		v, ok := integerConstant(x.Expr)
		if !ok {
			return 0, false
		}
		if x.Op == "+" {
			return v, true
		}
		if x.Op == "-" && v != math.MinInt64 {
			return -v, true
		}
	}
	return 0, false
}
func limitValue(ctx context.Context, expr sqlast.Expr, params []any) (int64, error) {
	value, err := sqlast.Eval(expr, &sqlast.EvalContext{Context: ctx, Params: params})
	if err != nil {
		return 0, err
	}
	if err := sqlast.PreflightValueContext(ctx, "LIMIT", value); err != nil {
		return 0, err
	}
	// OP_MustBeInt applies NUMERIC affinity and requires INTEGER storage.
	// Reuse the ordinary affinity grammar and exact REAL endpoint rules;
	// Go ParseFloat accepts spellings SQLite does not (hex/underscores).
	// The pinned UTF-8 sqlite3MemRealValueRC parser stops at an embedded
	// NUL. The length-aware exact-integer fallback then fails, so such a
	// string takes the REAL-to-integer affinity path, including endpoints.
	if text, ok := value.(string); ok {
		if end := strings.IndexByte(text, 0); end >= 0 {
			value, err = sqlast.ApplyAffinity(text[:end], "REAL")
			if err != nil {
				return 0, err
			}
		}
	}
	value, err = sqlast.ApplyAffinity(value, "NUMERIC")
	if err != nil {
		return 0, err
	}
	if n, ok := value.(int64); ok {
		return n, nil
	}
	return 0, fmt.Errorf("%w: LIMIT/OFFSET requires an integer", ErrMismatch)
}

func validate(e sqlast.Expr, t *table, aliases map[string]int) error {
	return sqlast.Walk(e, func(e sqlast.Expr) error { return validateNode(e, t, aliases) })
}
func validateNode(e sqlast.Expr, t *table, aliases map[string]int) error {
	switch x := e.(type) {
	case *sqlast.Literal, *sqlast.Parameter, *sqlast.Unary, *sqlast.Binary, *sqlast.Like, *sqlast.Case, *sqlast.Cast, *sqlast.Between, *sqlast.InList:
		return nil
	case *sqlast.Column:
		if x.Table == "" && aliases != nil {
			if _, ok := aliases[fold(x.Name)]; ok {
				return nil
			}
		}
		_, err := t.column(x.Table, x.Name)
		return err
	case *sqlast.Call:
		if fold(x.Name) == "grams13_match" {
			if len(x.Args) != 3 {
				return fmt.Errorf("wrong number of arguments to grams13_match()")
			}
			return nil
		}
		return sqlast.ValidateFunction(x.Name, len(x.Args))
	default:
		return fmt.Errorf("%w: expression context", ErrUnsupported)
	}
}

func columnLabel(t *table, c *sqlast.Column) string {
	i, err := t.column(c.Table, c.Name)
	if err != nil {
		return c.Name
	}
	if i >= 0 {
		return t.columns[i].Name
	}
	if t.ipk >= 0 {
		return t.columns[t.ipk].Name
	}
	return "rowid"
}

// SQLite keywords TRUE/FALSE are fallback literals: a same-named physical
// column takes precedence, including on the right side of the IS operator.
func bindBooleans(e sqlast.Expr, t *table) sqlast.Expr {
	if t == nil || e == nil {
		return e
	}
	return sqlast.Rewrite(e, func(e sqlast.Expr) sqlast.Expr {
		if x, ok := e.(*sqlast.Literal); ok && x.Boolean {
			name := "false"
			if x.Value == int64(1) {
				name = "true"
			}
			if _, ok := t.index[name]; ok {
				return &sqlast.Column{Name: name}
			}
		}
		return e
	})
}

func bindBooleanAliases(e sqlast.Expr, aliases map[string]int) sqlast.Expr {
	if _, yes := aliases["true"]; !yes {
		if _, yes := aliases["false"]; !yes {
			return e
		}
	}
	return sqlast.Rewrite(e, func(e sqlast.Expr) sqlast.Expr {
		if lit, ok := e.(*sqlast.Literal); ok && lit.Boolean {
			name := "false"
			if lit.Value == int64(1) {
				name = "true"
			}
			if _, ok := aliases[name]; ok {
				return &sqlast.Column{Name: name}
			}
		}
		return e
	})
}

func isConstantDefault(e sqlast.Expr) bool {
	switch x := e.(type) {
	case *sqlast.Literal:
		return true
	case *sqlast.Unary:
		return (x.Op == "+" || x.Op == "-") && isConstantDefault(x.Expr)
	}
	return false
}

// TRUE/FALSE are identifier fallbacks, never integer ORDER BY ordinals.
func orderResultName(e sqlast.Expr) (string, bool) {
	if c, ok := e.(*sqlast.Column); ok && c.Table == "" {
		return c.Name, true
	}
	if v, ok := e.(*sqlast.Literal); ok && v.Boolean {
		if v.Value == int64(1) {
			return "true", true
		}
		return "false", true
	}
	return "", false
}
