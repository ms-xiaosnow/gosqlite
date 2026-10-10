package sqlite

import (
	"context"
	"fmt"
	"strings"

	"github.com/ms-xiaosnow/gosqlite/internal/btree"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// A scope is resolved once. Expressions then address immutable private slots
// through the existing typed scalar adapter, never repeat ambiguous name lookup.
// Rowid is a hidden nullable slot per leaf; it is never a joined-row identity.
type relationalBinding struct {
	table  *table
	offset int
}
type relationalScope struct {
	budget *relationalBudget
	leaves []relationalBinding
	slots  *table
}

func slotName(i int) string { return fmt.Sprintf("\x00slot%d", i) }
func (s *relationalScope) resolve(q, name string) (slot int, found bool, err error) {
	if s.budget != nil {
		if err := s.budget.step(int64(len(q) + len(name))); err != nil {
			return 0, false, err
		}
	}
	slot = -1
	for _, b := range s.leaves {
		if s.budget != nil {
			if err := s.budget.step(1); err != nil {
				return 0, false, err
			}
		}
		if !b.table.qualifier(q) {
			continue
		}
		i, e := b.table.column(q, name)
		if e != nil {
			continue
		}
		if i < 0 {
			i = len(b.table.columns)
		}
		if found {
			return 0, true, fmt.Errorf("ambiguous column name: %s", qualified(q, name))
		}
		slot, found = b.offset+i, true
	}
	return slot, found, nil
}
func (s *relationalScope) bind(e sqlast.Expr, aliases map[string]sqlast.Expr) (sqlast.Expr, error) {
	bound, err := sqlast.RewriteChecked(e,
		func(e sqlast.Expr) error { return s.budget.step(relationalNodeWork(e)) },
		func(e sqlast.Expr) (sqlast.Expr, error) {
			c, ok := e.(*sqlast.Column)
			boolean := false
			if lit, yes := e.(*sqlast.Literal); yes && lit.Boolean {
				name := "false"
				if lit.Value == int64(1) {
					name = "true"
				}
				c, ok, boolean = &sqlast.Column{Name: name}, true, true
			}
			if !ok {
				return e, nil
			}
			i, found, err := s.resolve(c.Table, c.Name)
			if err != nil {
				return nil, err
			}
			if found {
				return &sqlast.Column{Name: slotName(i)}, nil
			}
			if c.Table == "" && aliases != nil {
				if a, ok := aliases[fold(c.Name)]; ok {
					return a, nil
				}
			}
			if boolean {
				return e, nil
			}
			return nil, fmt.Errorf("no such column: %s", qualified(c.Table, c.Name))
		})
	if err != nil {
		return nil, err
	}
	if err := sqlast.Walk(bound, func(e sqlast.Expr) error {
		if err := s.budget.step(relationalNodeWork(e)); err != nil {
			return err
		}
		return validateNode(e, s.slots, nil)
	}); err != nil {
		return nil, err
	}
	return bound, nil
}

// Physical labels are derived from the resolved slot, including TRUE/FALSE
// fallback names. No second resolution can swallow a quota/ambiguity error.
func (s *relationalScope) slotLabel(c *sqlast.Column) string {
	i := s.slots.index[c.Name]
	for _, b := range s.leaves {
		if i < b.offset || i > b.offset+len(b.table.columns) {
			continue
		}
		j := i - b.offset
		if j == len(b.table.columns) {
			if b.table.ipk >= 0 {
				return b.table.columns[b.table.ipk].Name
			}
			return "rowid"
		}
		return b.table.columns[j].Name
	}
	return c.Name
}
func (d *DB) relationalScope(ctx context.Context, reader btree.PageReader, relation sqlast.Relation, budget *relationalBudget) (*relationalScope, error) {
	s := &relationalScope{budget: budget, slots: &table{ipk: -1, index: map[string]int{}}}
	var visit func(sqlast.Relation) error
	visit = func(relation sqlast.Relation) error {
		if join, ok := relation.(*sqlast.JoinRelation); ok {
			if err := visit(join.Left); err != nil {
				return err
			}
			return visit(join.Right)
		}
		leaf, ok := relation.(*sqlast.TableRelation)
		if !ok {
			return fmt.Errorf("%w: FROM relation", ErrUnsupported)
		}
		if err := budget.step(1 + int64(len(leaf.Name)+len(leaf.Alias))); err != nil {
			return err
		}
		t, err := d.table(ctx, reader, &sqlast.Select{From: leaf.Name, Alias: leaf.Alias})
		if err != nil {
			return err
		}
		if len(s.slots.columns)+len(t.columns)+1 > sqlast.MaxColumns {
			return fmt.Errorf("%w: joined slots exceed %d", ErrUnsupported, sqlast.MaxColumns)
		}
		cost := int64(len(t.columns)+1)*256 + int64(len(t.name)+len(t.alias)) + 256
		for _, c := range t.columns {
			cost += int64(len(c.Name) + len(c.Type))
		}
		if err := budget.reserve(cost); err != nil {
			return err
		}
		s.leaves = append(s.leaves, relationalBinding{table: t, offset: len(s.slots.columns)})
		for _, c := range t.columns {
			i := len(s.slots.columns)
			name := slotName(i)
			s.slots.index[name] = i
			s.slots.columns = append(s.slots.columns, sqlast.ColumnDef{Name: name, Type: c.Type})
		}
		i := len(s.slots.columns)
		name := slotName(i)
		s.slots.index[name] = i
		s.slots.columns = append(s.slots.columns, sqlast.ColumnDef{Name: name, Type: "INTEGER"})
		return nil
	}
	if err := visit(relation); err != nil {
		return nil, err
	}
	return s, nil
}

func (d *DB) planRelational(ctx context.Context, reader btree.PageReader, original *sqlast.Select, params []any) (_ *Rows, err error) {
	budget, ok := ctx.Value(relationalRootBudgetKey{}).(*relationalBudget)
	if !ok {
		// Internal plan callers have already supplied bound parameters.
		budget, err = newRelationalBudget(ctx, d.opts.MaxSortBytes)
		if err != nil {
			return nil, err
		}
		if err = budget.reserve(int64(len(original.SQL))*64 + valuesSize(params)*2 + 1024); err != nil {
			return nil, err
		}
	}
	ctx = sqlast.WithEvalBudget(ctx, budget)
	// Schema lookup borrows this reader too; no child acquires a snapshot.
	schemaReader := &relationalReader{base: reader, budget: budget}
	scope, err := d.relationalScope(ctx, schemaReader, original.Relation, budget)
	// Keep schema reservations: parsed names/defaults may retain SQL backing bytes.
	if err != nil {
		return nil, err
	}
	sel := *original
	r := &Rows{db: d, ctx: ctx, table: scope.slots, sel: &sel, params: params, limit: -1, aliases: map[string]int{}, relational: budget}
	defer func() {
		if err != nil && r.source != nil {
			r.source.Close()
		}
	}()
	aliases := map[string]sqlast.Expr{}
	resultIndexes := map[string]int{}
	add := func(e sqlast.Expr, label string) error {
		if err := budget.step(1 + int64(len(label))); err != nil {
			return err
		}
		if len(r.exprs) >= sqlast.MaxColumns {
			return fmt.Errorf("%w: result columns exceed %d", ErrUnsupported, sqlast.MaxColumns)
		}
		if err := budget.reserve(256 + int64(len(label))); err != nil {
			return err
		}
		r.exprs = append(r.exprs, e)
		r.columns = append(r.columns, strings.Clone(label))
		key := fold(label)
		if _, ok := aliases[key]; !ok {
			aliases[key] = e
			resultIndexes[key] = len(r.exprs) - 1
		}
		return nil
	}
	for _, c := range original.Columns {
		if star, ok := c.Expr.(*sqlast.Star); ok {
			found := false
			for _, b := range scope.leaves {
				if !b.table.qualifier(star.Table) {
					continue
				}
				found = true
				for i, col := range b.table.columns {
					qualifier := b.table.alias
					if qualifier == "" {
						qualifier = b.table.name
					}
					if _, _, err := scope.resolve(qualifier, col.Name); err != nil {
						return nil, err
					}
					if err := add(&sqlast.Column{Name: slotName(b.offset + i)}, col.Name); err != nil {
						return nil, err
					}
				}
			}
			if !found {
				return nil, fmt.Errorf("no such table for wildcard: %s", star.Table)
			}
		} else {
			bound, e := scope.bind(c.Expr, nil)
			if e != nil {
				return nil, e
			}
			label := c.Alias
			if label == "" {
				label = c.Text
				_, physical := c.Expr.(*sqlast.Column)
				if lit, ok := c.Expr.(*sqlast.Literal); ok && lit.Boolean {
					physical = true
				}
				if col, ok := bound.(*sqlast.Column); ok && physical {
					label = scope.slotLabel(col)
				}
				if label == "" {
					label = sqlast.ExprName(c.Expr)
				}
			}
			if err := add(bound, label); err != nil {
				return nil, err
			}
		}
	}
	if len(r.exprs) > sqlast.MaxColumns {
		return nil, fmt.Errorf("%w: result columns exceed %d", ErrUnsupported, sqlast.MaxColumns)
	}
	sel.Where, err = scope.bind(original.Where, aliases)
	if err != nil {
		return nil, err
	}
	for _, o := range original.OrderBy {
		if err := sqlast.Walk(o.Expr, func(e sqlast.Expr) error { return budget.step(relationalNodeWork(e)) }); err != nil {
			return nil, err
		}
		if ordinal, ok := integerConstant(o.Expr); ok {
			if ordinal <= 0 || ordinal > int64(len(r.exprs)) {
				return nil, fmt.Errorf("ORDER BY position outside result column range")
			}
			r.order = append(r.order, orderExpr{column: int(ordinal) - 1, desc: o.Desc, nulls: o.Nulls})
			continue
		}
		if name, ok := orderResultName(o.Expr); ok {
			if i, yes := resultIndexes[fold(name)]; yes {
				r.order = append(r.order, orderExpr{column: i, desc: o.Desc, nulls: o.Nulls})
				continue
			}
		}
		bound, e := scope.bind(o.Expr, aliases)
		if e != nil {
			return nil, e
		}
		r.order = append(r.order, orderExpr{expr: bound, column: -1, desc: o.Desc, nulls: o.Nulls})
	}
	if sel.Limit != nil {
		if err = relationalValidate(sel.Limit, nil, budget); err != nil {
			return nil, err
		}
		r.limit, err = limitValue(ctx, sel.Limit, params)
		if err != nil {
			return nil, err
		}
	}
	if sel.Offset != nil {
		if err = relationalValidate(sel.Offset, nil, budget); err != nil {
			return nil, err
		}
		r.offset, err = limitValue(ctx, sel.Offset, params)
		if err != nil {
			return nil, err
		}
		r.offset = max(r.offset, 0)
	}
	// Bind all ON expressions before any row is fetched. Forward references in
	// ON are explicitly excluded for now, including through result aliases.
	n := 0
	var onExprs []sqlast.Expr
	var build func(sqlast.Relation) (relationalFactory, int, error)
	build = func(relation sqlast.Relation) (relationalFactory, int, error) {
		if _, ok := relation.(*sqlast.TableRelation); ok {
			b := scope.leaves[n]
			n++
			return func() (source, error) {
				return &relationalLeaf{ctx: ctx, base: reader, binding: b, budget: budget, stats: &r.queryStats}, nil
			}, len(b.table.columns) + 1, nil
		}
		j := relation.(*sqlast.JoinRelation)
		left, lw, e := build(j.Left)
		if e != nil {
			return nil, 0, e
		}
		right, rw, e := build(j.Right)
		if e != nil {
			return nil, 0, e
		}
		on, e := scope.bind(j.On, aliases)
		if e != nil {
			return nil, 0, e
		}
		prefixWidth := scope.leaves[n-1].offset + rw
		if e = sqlast.Walk(on, func(e sqlast.Expr) error {
			if err := budget.step(1); err != nil {
				return err
			}
			if c, ok := e.(*sqlast.Column); ok {
				if i, yes := scope.slots.index[c.Name]; yes && i >= prefixWidth {
					return fmt.Errorf("%w: ON references a later FROM relation", ErrUnsupported)
				}
			}
			return nil
		}); e != nil {
			return nil, 0, e
		}
		onExprs = append(onExprs, on)
		return func() (source, error) {
			l, e := left()
			if e != nil {
				return nil, e
			}
			return &relationalJoin{left: l, rightFactory: right, leftWidth: lw, rightWidth: rw, kind: j.Kind, on: on, rows: r, budget: budget}, nil
		}, lw + rw, nil
	}
	factory, _, err := build(original.Relation)
	if err != nil {
		return nil, err
	}
	// Include ON-only grams calls in the same snapshot-bound function registry.
	projections := r.exprs
	r.exprs = append(append([]sqlast.Expr{}, projections...), onExprs...)
	// Preflight the legacy registry's metadata-only scan, including shared
	// alias trees, before entering its unchanged single-source helper.
	for _, e := range append(append([]sqlast.Expr{}, r.exprs...), sel.Where) {
		if err = sqlast.Walk(e, func(e sqlast.Expr) error { return budget.step(relationalNodeWork(e)) }); err != nil {
			return nil, err
		}
	}
	for _, o := range r.order {
		if err = sqlast.Walk(o.expr, func(e sqlast.Expr) error { return budget.step(relationalNodeWork(e)) }); err != nil {
			return nil, err
		}
	}
	err = d.prepareGramFunctions(ctx, schemaReader, r)
	r.exprs = projections
	if err != nil {
		return nil, err
	}
	var tables []string
	for _, b := range scope.leaves {
		label := b.table.name
		if b.table.alias != "" {
			label += " AS " + b.table.alias
		}
		tables = append(tables, label)
	}
	r.queryPlan = QueryPlan{Access: "JOIN", Table: strings.Join(tables, ", "), Method: "nested_loop", Reason: "left-to-right bounded nested-loop table scans; ON before null extension; WHERE after join; no index or ANN pushdown"}
	if original.Explain {
		r.explainDetail = r.queryPlan.Detail()
		r.columns = []string{"id", "parent", "notused", "detail"}
		r.limit, r.offset = -1, 0
		return r, nil
	}
	r.source, err = factory()
	if err != nil {
		return nil, err
	}
	if len(r.order) > 0 && r.limit != 0 {
		if err = r.materialize(); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func relationalNodeWork(e sqlast.Expr) int64 {
	switch x := e.(type) {
	case *sqlast.Call:
		return 1 + 3*int64(len(x.Name))
	case *sqlast.Cast:
		return 1 + 3*int64(len(x.Type))
	case *sqlast.Column:
		return 1 + int64(len(x.Table)+len(x.Name))
	}
	return 1
}
func relationalValidate(e sqlast.Expr, t *table, b *relationalBudget) error {
	return sqlast.Walk(e, func(e sqlast.Expr) error {
		if err := b.step(relationalNodeWork(e)); err != nil {
			return err
		}
		return validateNode(e, t, nil)
	})
}
