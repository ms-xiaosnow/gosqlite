package sqlite

import (
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// Expression matching is intentionally structural, after scoped binding. It
// does not assert algebraic equivalence. Preflight both complete bounded trees
// before reflect's traversal/scratch; shared aliases cannot bypass work limits.
func sameGroupExpression(a, c sqlast.Expr, b *relationalBudget) (bool, error) {
	defer b.clearScalar()
	for _, e := range []sqlast.Expr{a, c} {
		if err := sqlast.Walk(e, func(e sqlast.Expr) error {
			cost := relationalNodeWork(e)
			if lit, ok := e.(*sqlast.Literal); ok {
				cost += valuesSize([]any{lit.Value})
			}
			if err := b.step(cost); err != nil {
				return err
			}
			return b.Allocate(128)
		}); err != nil {
			return false, err
		}
	}
	return reflect.DeepEqual(a, c), nil
}

func (r *Rows) prepareGroupedAggregate(factory relationalFactory) error {
	b := r.relational
	if err := b.reserve(2048 + 32*int64(len(r.exprs)+len(r.order)+len(r.sel.GroupBy))); err != nil {
		return err
	}
	child := &Rows{db: r.db, ctx: r.ctx, table: r.table, sel: r.sel, params: r.params, aliases: r.aliases, functions: r.functions, where: r.where, fixedColumns: r.fixedColumns, startupPredicates: r.startupPredicates, relational: b, limit: -1}
	output := &table{ipk: -1, index: map[string]int{}}
	groupKeys := r.sel.GroupBy
	// Pinned select.c copies directions when ORDER/GROUP lengths match, then
	// elides the result sorter only for identical expressions and NULL order.
	groupDesc := make([]bool, len(groupKeys))
	directOrder := len(groupKeys) > 0 && len(r.order) == len(groupKeys)
	if len(r.order) == len(groupKeys) {
		for i, o := range r.order {
			groupDesc[i] = o.desc
			e := o.expr
			if o.column >= 0 {
				e = r.exprs[o.column]
			}
			equal, err := sameGroupExpression(e, groupKeys[i], b)
			if err != nil {
				return err
			}
			natural := o.nulls == sqlast.NullsDefault || o.nulls == sqlast.NullsFirst && !o.desc || o.nulls == sqlast.NullsLast && o.desc
			directOrder = directOrder && equal && natural
		}
	}
	keySlots := make([]sqlast.Expr, len(groupKeys))
	for i, e := range groupKeys {
		if err := b.reserve(256); err != nil {
			return err
		}
		name := fmt.Sprintf("\x00group%d", i)
		typ := ""
		// A synthetic group column must preserve expression affinity. Unary +
		// strips the synthetic column's affinity for an affinity-less expression.
		switch x := e.(type) {
		case *sqlast.Column:
			typ = child.table.columns[child.table.index[x.Name]].Type
		case *sqlast.Cast:
			typ = sqlast.ExpressionAffinity(x, nil)
		}
		output.index[name] = len(output.columns)
		output.columns = append(output.columns, sqlast.ColumnDef{Name: name, Type: typ})
		keySlots[i] = &sqlast.Column{Name: name}
		if sqlast.ExpressionAffinity(e, func(q, n string) string { return affinity(child.table.columns[child.table.index[n]].Type) }) == "" {
			keySlots[i] = &sqlast.Unary{Op: "+", Expr: keySlots[i]}
		}
		child.exprs = append(child.exprs, e)
	}
	// Retain only source slots needed to evaluate grouping expressions on the
	// representative row. Borrow raw cells so unused typed CASE arms stay lazy.
	repIndex := map[string]int{}
	var repColumns []int
	repKeys := make([]sqlast.Expr, len(groupKeys))
	for i, key := range groupKeys {
		var err error
		repKeys[i], err = sqlast.RewriteChecked(key, func(e sqlast.Expr) error { return groupedRewriteAdmission(e, b) }, func(e sqlast.Expr) (sqlast.Expr, error) {
			c, ok := e.(*sqlast.Column)
			if !ok {
				return e, nil
			}
			j, exists := repIndex[c.Name]
			if !exists {
				if len(output.columns) >= sqlast.MaxColumns {
					return nil, fmt.Errorf("%w: grouping result slots exceed %d", ErrUnsupported, sqlast.MaxColumns)
				}
				if err := b.reserve(256 + int64(len(c.Name))); err != nil {
					return nil, err
				}
				j = len(repColumns)
				repIndex[c.Name] = j
				input := child.table.index[c.Name]
				repColumns = append(repColumns, input)
				name := fmt.Sprintf("\x00representative%d", j)
				output.index[name] = len(output.columns)
				output.columns = append(output.columns, sqlast.ColumnDef{Name: name, Type: child.table.columns[input].Type})
			}
			return &sqlast.Column{Name: fmt.Sprintf("\x00representative%d", j)}, nil
		})
		if err != nil {
			return err
		}
	}
	var states []sqlast.Aggregate
	var calls []sqlast.Expr
	lowerCall := func(e sqlast.Expr) (sqlast.Expr, error) {
		c, ok := e.(*sqlast.Call)
		if !ok || !sqlast.AggregateName(c.Name) {
			return e, nil
		}
		for i, old := range calls {
			equal, err := sameGroupExpression(c, old, b)
			if err != nil {
				return nil, err
			}
			if equal {
				return &sqlast.Unary{Op: "+", Expr: &sqlast.Column{Name: fmt.Sprintf("\x00aggregate%d", i)}}, nil
			}
		}
		if len(output.columns) >= sqlast.MaxColumns {
			return nil, fmt.Errorf("%w: grouping result slots exceed %d", ErrUnsupported, sqlast.MaxColumns)
		}
		if err := b.reserve(768); err != nil {
			return nil, err
		}
		arg := sqlast.Expr(&sqlast.Literal{Value: int64(1)})
		if len(c.Args) == 1 {
			if _, star := c.Args[0].(*sqlast.Star); !star {
				arg = c.Args[0]
			}
		}
		i := len(states)
		states = append(states, sqlast.Aggregate{Name: c.Name})
		calls = append(calls, c)
		child.exprs = append(child.exprs, arg)
		name := fmt.Sprintf("\x00aggregate%d", i)
		output.index[name] = len(output.columns)
		output.columns = append(output.columns, sqlast.ColumnDef{Name: name})
		return &sqlast.Unary{Op: "+", Expr: &sqlast.Column{Name: name}}, nil
	}
	lower := func(e sqlast.Expr, resultRoot bool) (sqlast.Expr, error) {
		if err := sqlast.Walk(e, func(e sqlast.Expr) error {
			if err := b.step(relationalNodeWork(e)); err != nil {
				return err
			}
			if c, ok := e.(*sqlast.Call); ok && sqlast.AggregateName(c.Name) {
				if err := b.reserve(128); err != nil {
					return err
				}
				_, err := lowerCall(c)
				return err
			}
			return nil
		}); err != nil {
			return nil, err
		}
		e, err := sqlast.RewriteChecked(e, func(e sqlast.Expr) error { return groupedRewriteAdmission(e, b) }, lowerCall)
		if err != nil {
			return nil, err
		}
		// Lower aggregates first so their input expressions cannot turn into group
		// output slots. Then replace complete grouping expressions in scalar output.
		first := true
		e, err = sqlast.RewritePrunedChecked(e, func(e sqlast.Expr) error { return groupedRewriteAdmission(e, b) }, func(e sqlast.Expr) (sqlast.Expr, bool, error) {
			root := first
			first = false
			for i, key := range groupKeys {
				equal, err := sameGroupExpression(e, key, b)
				if err != nil {
					return nil, false, err
				}
				if equal {
					if _, column := key.(*sqlast.Column); !column && root && resultRoot {
						return keySlots[i], true, nil
					}
					return repKeys[i], true, nil
				}
			}
			return e, false, nil
		})
		if err != nil {
			return nil, err
		}
		err = sqlast.Walk(e, func(e sqlast.Expr) error {
			if err := b.step(relationalNodeWork(e)); err != nil {
				return err
			}
			if c, ok := e.(*sqlast.Column); ok {
				if _, yes := output.index[c.Name]; !yes {
					return fmt.Errorf("%w: source expression is not a grouping key", ErrUnsupported)
				}
			}
			return validateNode(e, output, nil)
		})
		return e, err
	}
	expressions := make([]sqlast.Expr, len(r.exprs))
	for i, e := range r.exprs {
		var err error
		expressions[i], err = lower(e, true)
		if err != nil {
			return err
		}
	}
	var err error
	for i, o := range r.order {
		if o.column < 0 {
			r.order[i].expr, err = lower(o.expr, true)
			if err != nil {
				return err
			}
		}
	}
	having, err := lower(r.sel.Having, false)
	if err != nil {
		return err
	}

	src, err := factory()
	if err != nil {
		return err
	}
	// A single physical rowid in the grouping tuple proves every input row
	// is a separate group. SQLite uses the physical stream even when that
	// unique key is not the first GROUP BY term.
	stream, reverse := false, false
	if leaf, ok := src.(*relationalLeaf); ok {
		isRowid := func(e sqlast.Expr) bool {
			c, ok := e.(*sqlast.Column)
			if !ok {
				return false
			}
			i := child.table.index[c.Name]
			return i == leaf.binding.offset+len(leaf.binding.table.columns) || leaf.binding.table.ipk >= 0 && i == leaf.binding.offset+leaf.binding.table.ipk
		}
		for _, key := range groupKeys {
			stream = stream || isRowid(key)
		}
		if stream {
			directOrder = directOrder && isRowid(groupKeys[0])
			reverse = directOrder && groupDesc[0]
			leaf.reverse = reverse
		}
	}
	if len(groupKeys) == 0 || directOrder {
		r.order = nil
	}
	arguments := &Rows{ctx: r.ctx, table: child.table, params: r.params, functions: r.functions, exprs: child.exprs[len(groupKeys):], aliases: map[string]int{}, where: &predicateProgram{}, relational: b}
	child.exprs = child.exprs[:len(groupKeys)]
	raw := &groupedInputSource{source: aggregateInput(src, child)}
	child.source = raw
	r.source = &groupedAggregateSource{child: child, template: states, keyCount: len(groupKeys), budget: b, global: len(groupKeys) == 0, raw: raw, arguments: arguments, repColumns: repColumns, groupDesc: groupDesc, stream: stream}
	r.exprs, r.table = expressions, output
	r.where = &predicateProgram{residual: []sqlast.Expr{having}}
	if having == nil {
		r.where.residual = nil
	}
	r.startupPredicates, r.fixedColumns = nil, nil
	r.aliases = map[string]int{}
	finalSelect := *r.sel
	finalSelect.Where = having
	r.sel = &finalSelect
	return nil
}

type aggregateGroup struct {
	keys                []any
	states              []sqlast.Aggregate
	retained            []int64
	values              []any
	cost                int64
	representative      []any
	representativeBytes int64
	seen                bool
	err                 error
}
type groupedAggregateSource struct {
	child                          *Rows
	arguments                      *Rows
	template                       []sqlast.Aggregate
	keyCount                       int
	raw                            *groupedInputSource
	repColumns                     []int
	groupDesc                      []bool
	budget                         *relationalBudget
	groups                         []*aggregateGroup // key-sorted; binary search, explicitly charged movement
	index                          int
	capacityBytes                  int64
	ready, closed, global          bool
	stream, pendingReady, inputEOF bool
	pending                        resultRow
}

func (s *groupedAggregateSource) compare(a, b []any) (int, error) {
	for i, x := range a {
		if err := s.budget.step(1 + valuesSize([]any{x, b[i]})); err != nil {
			return 0, err
		}
		c, err := sqlast.Compare(x, b[i])
		if i < len(s.groupDesc) && s.groupDesc[i] {
			c = -c
		}
		if err != nil || c != 0 {
			return c, err
		}
	}
	return 0, nil
}
func detachedValue(v any) any {
	switch x := v.(type) {
	case string:
		return strings.Clone(x)
	case []byte:
		return append([]byte{}, x...)
	}
	return v
}
func (s *groupedAggregateSource) group(keys []any) (*aggregateGroup, error) {
	lo, hi := 0, len(s.groups)
	for lo < hi {
		m := lo + (hi-lo)/2
		c, err := s.compare(keys, s.groups[m].keys)
		if err != nil {
			return nil, err
		}
		if c == 0 {
			return s.groups[m], nil
		}
		if c < 0 {
			hi = m
		} else {
			lo = m + 1
		}
	}
	// Include backing capacity growth, the key copy, all fixed-size accumulator
	// state, retained-charge slots, finalized slots and insertion movement.
	cost := 256 + valuesSize(keys) + int64(len(s.template))*320 + int64(len(keys)+len(s.repColumns)+len(s.template))*16
	if err := s.budget.step(1 + int64(len(s.groups)-lo) + valuesSize(keys)); err != nil {
		return nil, err
	}
	if err := s.budget.reserve(cost); err != nil {
		return nil, err
	}
	if len(s.groups) == cap(s.groups) {
		capacity := max(4, 2*cap(s.groups))
		if err := s.budget.reserve(int64(capacity) * 8); err != nil {
			s.budget.release(cost)
			return nil, err
		}
		s.capacityBytes += int64(capacity) * 8
		groups := make([]*aggregateGroup, len(s.groups), capacity)
		copy(groups, s.groups)
		s.groups = groups
	}
	g := &aggregateGroup{keys: make([]any, len(keys)), states: append([]sqlast.Aggregate(nil), s.template...), retained: make([]int64, len(s.template)), cost: cost}
	for i, v := range keys {
		g.keys[i] = detachedValue(v)
	}
	s.groups = append(s.groups, nil)
	copy(s.groups[lo+1:], s.groups[lo:])
	s.groups[lo] = g
	return g, nil
}
func (s *groupedAggregateSource) build() error {
	if s.global {
		if _, err := s.group(nil); err != nil {
			return err
		}
	}
	for {
		if err := s.budget.step(1); err != nil {
			return err
		}
		row, _, err := s.child.fetch()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		g, err := s.group(row.values[:s.keyCount])
		if err != nil {
			return err
		}
		if err := s.accumulate(g); err != nil {
			return err
		}
	}
	if err := s.child.source.Close(); err != nil {
		return err
	}
	s.child.source = nil
	s.budget.clearResult()
	s.ready = true
	return nil
}
func (s *groupedAggregateSource) accumulate(g *aggregateGroup) error {
	if g.err != nil {
		return nil
	}
	s.budget.clearResult()
	args, cost, _, argErr := s.arguments.evaluateSourceRow(0, s.raw.values)
	if argErr != nil {
		if err := s.retainGroupError(g, argErr); err != nil {
			return err
		}
		return nil
	}
	if err := s.budget.reserve(cost); err != nil {
		return err
	}
	s.budget.resultBytes = cost
	state := aggregateSource{child: s.child, states: g.states, retained: g.retained, budget: s.budget}
	replaceRepresentative := !g.seen
	for i, v := range args.values {
		name := g.states[i].Name
		if name == "min" || name == "max" {
			old := g.states[i].RetainedValue()
			replaceRepresentative = old == nil
			if v != nil && old != nil {
				if err := s.budget.step(valuesSize([]any{v, old})); err != nil {
					return err
				}
				cmp, err := sqlast.Compare(v, old)
				if err != nil {
					return err
				}
				replaceRepresentative = name == "min" && cmp < 0 || name == "max" && cmp > 0
			}
		}

		if err := state.step(i, v); err != nil {
			if err := s.retainGroupError(g, err); err != nil {
				return err
			}
			break
		}
	}
	if replaceRepresentative {
		if err := s.retainRepresentative(g); err != nil {
			return err
		}
	}
	g.seen = true
	return nil
}
func (s *groupedAggregateSource) Next() (int64, []any, error) {
	if s.closed {
		return 0, nil, io.EOF
	}
	if s.stream {
		return s.nextStream()
	}
	if !s.ready {
		if err := s.build(); err != nil {
			return 0, nil, err
		}
	}
	if s.index == len(s.groups) {
		return 0, nil, io.EOF
	}
	g := s.groups[s.index]
	return s.finalize(g)
}

func (s *groupedAggregateSource) finalize(g *aggregateGroup) (int64, []any, error) {
	if g.err != nil {
		return 0, nil, g.err
	}
	g.values = make([]any, s.keyCount+len(s.repColumns)+len(g.states)) // admitted in group()
	copy(g.values, g.keys)
	copy(g.values[s.keyCount:], g.representative)
	for i := range g.states {
		if err := s.budget.step(1); err != nil {
			return 0, nil, err
		}
		v, err := g.states[i].Result()
		if err != nil {
			return 0, nil, err
		}
		g.values[s.keyCount+len(s.repColumns)+i] = v
	}
	s.index++
	return int64(s.index), g.values, nil
}
func (s *groupedAggregateSource) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	var err error
	if s.child != nil && s.child.source != nil {
		err = s.child.source.Close()
		s.child.source = nil
	}
	for _, g := range s.groups {
		for _, n := range g.retained {
			s.budget.release(n)
		}
		s.budget.release(g.cost + g.representativeBytes)
	}
	s.budget.release(s.capacityBytes)
	s.capacityBytes = 0
	s.groups, s.template, s.child, s.arguments, s.raw = nil, nil, nil, nil, nil
	return err
}

// SQLite's BINARY group-key-only HAVING conjuncts are promoted before input
// evaluation. This matters even for numeric-equal keys with different typeof.
// Aggregate-dependent terms remain in HAVING and run after finalization.
func (r *Rows) promoteGroupedHaving() error {
	if len(r.sel.GroupBy) == 0 || r.sel.Having == nil {
		return nil
	}
	b := r.relational
	var promote func(sqlast.Expr) (sqlast.Expr, error)
	promote = func(e sqlast.Expr) (sqlast.Expr, error) {
		if err := b.step(relationalNodeWork(e)); err != nil {
			return nil, err
		}
		if x, ok := e.(*sqlast.Binary); ok && x.Op == "AND" {
			left, err := promote(x.Left)
			if err != nil {
				return nil, err
			}
			right, err := promote(x.Right)
			if err != nil {
				return nil, err
			}
			if left == nil {
				return right, nil
			}
			if right == nil {
				return left, nil
			}
			if err := b.reserve(80); err != nil {
				return nil, err
			}
			return &sqlast.Binary{Op: "AND", Left: left, Right: right}, nil
		}
		if sqlast.SyntacticFalse(e) {
			return e, nil
		}
		replaced, err := sqlast.RewritePrunedChecked(e, func(e sqlast.Expr) error { return groupedRewriteAdmission(e, b) }, func(e sqlast.Expr) (sqlast.Expr, bool, error) {
			for _, key := range r.sel.GroupBy {
				equal, err := sameGroupExpression(e, key, b)
				if err != nil {
					return nil, false, err
				}
				if equal {
					return &sqlast.Literal{Value: int64(1)}, true, nil
				}
			}
			return e, false, nil
		})
		if err != nil {
			return nil, err
		}
		if !sqlast.ConstantExpression(replaced, r.predicateContext()) {
			return e, nil
		}
		if r.sel.Where == nil {
			r.sel.Where = e
		} else {
			if err := b.reserve(80); err != nil {
				return nil, err
			}
			r.sel.Where = &sqlast.Binary{Op: "AND", Left: r.sel.Where, Right: e}
		}
		return nil, nil
	}
	var err error
	r.sel.Having, err = promote(r.sel.Having)
	return err
}

// Buffer the complete unordered grouped answer on its first pull. OFFSET and
// LIMIT are applied before scalar projection, as in the pinned group-output
// loop. All relevant finalizers, HAVING, projection, retention and work checks
// finish before any public row. Ordered grouping uses its eager bounded sorter/top-K path.
func (r *Rows) materializeGrouped() error {
	r.sorted = make([]resultRow, 0)
	for r.limit < 0 || int64(len(r.sorted)) < r.limit {
		row, cost, err := r.fetch()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if err = r.relational.reserve(cost); err != nil {
			return err
		}
		if len(r.sorted) == cap(r.sorted) {
			capacity := max(4, 2*cap(r.sorted))
			if err = r.relational.reserve(int64(capacity) * 64); err != nil {
				return err
			}
			next := make([]resultRow, len(r.sorted), capacity)
			copy(next, r.sorted)
			r.sorted = next
		}
		r.sorted = append(r.sorted, row)
	}
	if err := r.source.Close(); err != nil {
		return err
	}
	r.source = nil
	return nil
}

// A single borrowed raw tuple follows the existing child's cursor lifetime.
// It does not copy rows or acquire another snapshot/transaction lease.
type groupedInputSource struct {
	source
	values []any
}

func (s *groupedInputSource) Next() (int64, []any, error) {
	id, v, err := s.source.Next()
	s.values = v
	return id, v, err
}
func (s *groupedInputSource) Close() error { s.values = nil; return s.source.Close() }
func (s *groupedAggregateSource) retainRepresentative(g *aggregateGroup) error {
	cost := int64(len(s.repColumns))*32 + 32
	for _, i := range s.repColumns {
		cost += valuesSize([]any{s.raw.values[i]})
	}
	if err := s.budget.step(cost); err != nil {
		return err
	}
	if err := s.budget.reserve(cost); err != nil {
		return err
	}
	values := make([]any, len(s.repColumns))
	for j, i := range s.repColumns {
		values[j] = detachedValue(s.raw.values[i])
	}
	s.budget.release(g.representativeBytes)
	g.representative, g.representativeBytes = values, cost
	return nil
}

// Argument evaluation errors belong to their sorted group. Retaining one error
// lets LIMIT stop before a later erroneous group without materializing input
// rows. Resource/cancellation failures are statement-wide and never deferred.
func (s *groupedAggregateSource) retainGroupError(g *aggregateGroup, err error) error {
	if errors.Is(err, ErrQueryWorkLimit) || errors.Is(err, ErrMemoryLimit) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	cost := int64(len(err.Error())) + 512
	if e := s.budget.step(cost); e != nil {
		return e
	}
	if e := s.budget.reserve(cost); e != nil {
		return e
	}
	g.cost += cost
	g.err = err
	return nil
}

// Composite child-array storage is admitted at its parent, before the common
// rewriter allocates it; per-child visits cannot serve as that preflight.
func groupedRewriteAdmission(e sqlast.Expr, b *relationalBudget) error {
	if err := b.step(relationalNodeWork(e)); err != nil {
		return err
	}
	cost := int64(128)
	switch x := e.(type) {
	case *sqlast.Case:
		cost += int64(len(x.Arms)) * 32
	case *sqlast.InList:
		cost += int64(len(x.Values)) * 16
	case *sqlast.Call:
		cost += int64(len(x.Args)) * 16
	}
	return b.reserve(cost)
}

// The next qualifying row's WHERE and grouping expressions must be evaluated
// before the previous row's group can be emitted. Aggregate arguments for that
// lookahead row remain unevaluated until its own group is requested.
func (s *groupedAggregateSource) nextStream() (int64, []any, error) {
	if s.inputEOF {
		return 0, nil, io.EOF
	}
	if !s.pendingReady {
		row, _, err := s.child.fetch()
		if err != nil {
			return 0, nil, err
		}
		s.pending, s.pendingReady = row, true
	}
	g, err := s.group(s.pending.values)
	if err != nil {
		return 0, nil, err
	}
	s.pending, s.pendingReady = resultRow{}, false
	if err = s.accumulate(g); err != nil {
		return 0, nil, err
	}
	if g.err != nil {
		return 0, nil, g.err
	}
	row, _, err := s.child.fetch()
	if err == io.EOF {
		s.inputEOF = true
		if err = s.child.source.Close(); err != nil {
			return 0, nil, err
		}
		s.child.source = nil
		s.budget.clearResult()
	} else if err != nil {
		return 0, nil, err
	} else {
		s.pending, s.pendingReady = row, true
	}
	return s.finalize(g)
}
