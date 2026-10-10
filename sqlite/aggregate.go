package sqlite

import (
	"fmt"
	"io"
	"strings"

	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// Detection runs over the parser's already bounded tree before binding. Every
// aggregate clause activates one budget before engine-owned parameter copies.
func selectUsesAggregate(s *sqlast.Select) bool {
	found := len(s.GroupBy) > 0 || s.Having != nil
	check := func(e sqlast.Expr) {
		_ = sqlast.Walk(e, func(e sqlast.Expr) error {
			if c, ok := e.(*sqlast.Call); ok && sqlast.AggregateName(c.Name) {
				found = true
			}
			return nil
		})
	}
	for _, c := range s.Columns {
		check(c.Expr)
	}
	check(s.Having)
	for _, e := range s.GroupBy {
		check(e)
	}
	check(s.Where)
	check(s.Limit)
	check(s.Offset)
	for _, o := range s.OrderBy {
		check(o.Expr)
	}
	var relation func(sqlast.Relation)
	relation = func(r sqlast.Relation) {
		if j, ok := r.(*sqlast.JoinRelation); ok {
			check(j.On)
			relation(j.Left)
			relation(j.Right)
		}
	}
	relation(s.Relation)
	return found
}

// Bind every name/function even in dead arms or LIMIT 0, then reject forms whose
// semantics this global-only slice cannot implement honestly.
func checkAggregateExpression(e sqlast.Expr, allowed, inside bool, b *relationalBudget) error {
	if e == nil {
		return nil
	}
	if err := b.step(relationalNodeWork(e)); err != nil {
		return err
	}
	if c, ok := e.(*sqlast.Call); ok && sqlast.AggregateName(c.Name) {
		if !allowed || inside {
			return fmt.Errorf("misuse of aggregate function %s()", c.Name)
		}
		if (c.Name == "min" || c.Name == "max") && len(c.Args) > 1 {
			return fmt.Errorf("%w: multiargument scalar %s()", ErrUnsupported, c.Name)
		}
		if len(c.Args) != 1 && !(c.Name == "count" && len(c.Args) == 0) {
			return fmt.Errorf("wrong number of arguments to %s()", c.Name)
		}
		if c.Name == "count" && len(c.Args) == 1 {
			if star, ok := c.Args[0].(*sqlast.Star); ok && star.Table == "" {
				return nil
			}
		}
		return sqlast.VisitChildren(e, func(child sqlast.Expr) error { return checkAggregateExpression(child, true, true, b) })
	}
	if _, ok := e.(*sqlast.Star); ok {
		return fmt.Errorf("%w: aggregate argument wildcard", ErrUnsupported)
	}
	return sqlast.VisitChildren(e, func(child sqlast.Expr) error { return checkAggregateExpression(child, allowed, inside, b) })
}

// prepareAggregate lowers aggregate calls to private result slots. The child
// Rows is an internal evaluator only: it is never registered, owns no snapshot
// or transaction lease, and is never consumed through public locking methods.
func (r *Rows) prepareAggregate(factory relationalFactory) error {
	b := r.relational
	child := &Rows{db: r.db, ctx: r.ctx, table: r.table, sel: r.sel, params: r.params, aliases: r.aliases, functions: r.functions, where: r.where, fixedColumns: r.fixedColumns, startupPredicates: r.startupPredicates, relational: b, limit: -1}
	output := &table{ipk: -1, index: map[string]int{}}
	var states []sqlast.Aggregate
	lower := func(e sqlast.Expr) (sqlast.Expr, error) {
		return sqlast.RewriteChecked(e, func(e sqlast.Expr) error { return b.step(relationalNodeWork(e)) }, func(e sqlast.Expr) (sqlast.Expr, error) {
			c, ok := e.(*sqlast.Call)
			if !ok || !sqlast.AggregateName(c.Name) {
				return e, nil
			}
			if len(states) >= sqlast.MaxColumns {
				return nil, fmt.Errorf("%w: aggregate states exceed %d", ErrUnsupported, sqlast.MaxColumns)
			}
			if err := b.reserve(512); err != nil {
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
			child.exprs = append(child.exprs, arg)
			name := fmt.Sprintf("\x00aggregate%d", i)
			output.index[name] = i
			output.columns = append(output.columns, sqlast.ColumnDef{Name: name})
			return &sqlast.Column{Name: name}, nil
		})
	}
	expressions := make([]sqlast.Expr, len(r.exprs))
	for i, e := range r.exprs {
		var err error
		expressions[i], err = lower(e)
		if err != nil {
			return err
		}
		if err = sqlast.Walk(expressions[i], func(e sqlast.Expr) error {
			if err := b.step(relationalNodeWork(e)); err != nil {
				return err
			}
			if c, ok := e.(*sqlast.Column); ok {
				if _, found := output.index[c.Name]; !found {
					return fmt.Errorf("%w: bare source column in global aggregate projection", ErrUnsupported)
				}
			}
			return validateNode(e, output, nil)
		}); err != nil {
			return err
		}
	}
	if len(states) == 0 {
		return fmt.Errorf("misuse of aggregate function outside result expression")
	}
	src, err := factory()
	if err != nil {
		return err
	}
	child.source = aggregateInput(src, child)
	r.source = &aggregateSource{child: child, states: states, retained: make([]int64, len(states)), budget: b}
	r.exprs, r.table = expressions, output
	r.where, r.startupPredicates, r.fixedColumns = nil, nil, nil
	r.aliases = map[string]int{}
	// A single global row is already ordered. SQLite omits sorter-key evaluation
	// here; the supported alias/ordinal forms were all validated before lowering.
	r.order = nil
	finalSelect := *r.sel
	finalSelect.Where = nil
	r.sel = &finalSelect
	return nil
}

type aggregateSource struct {
	child         *Rows
	states        []sqlast.Aggregate
	retained      []int64
	budget        *relationalBudget
	done, closed  bool
	discardOutput bool
}

func (s *aggregateSource) Next() (int64, []any, error) {
	if s.done || s.closed {
		return 0, nil, io.EOF
	}
	s.done = true
	for {
		if err := s.budget.step(1); err != nil {
			return 0, nil, err
		}
		row, _, err := s.child.fetch()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, nil, err
		}
		for i, v := range row.values {
			if err := s.step(i, v); err != nil {
				return 0, nil, err
			}
		}
	}
	// Leaf pages/records can be released before final scalar output evaluation.
	if err := s.child.source.Close(); err != nil {
		return 0, nil, err
	}
	s.child.source = nil
	s.budget.clearResult()
	if err := s.budget.reserve(int64(len(s.states))*16 + 64); err != nil {
		return 0, nil, err
	}
	values := make([]any, len(s.states))
	for i := range s.states {
		if err := s.budget.step(1); err != nil {
			return 0, nil, err
		}
		v, err := s.states[i].Result()
		if err != nil {
			return 0, nil, err
		}
		values[i] = v
	}
	if s.discardOutput {
		// OFFSET suppresses scalar projection, but only after every input and
		// aggregate finalizer has run (SUM overflow must still be reported).
		return 0, nil, io.EOF
	}
	return 0, values, nil
}
func (s *aggregateSource) step(i int, v any) error {
	defer s.budget.clearScalar()
	a := &s.states[i]
	if err := sqlast.PreflightValueContext(s.child.ctx, a.Name, v); err != nil {
		return err
	}
	if (a.Name == "min" || a.Name == "max") && v != nil {
		old := a.RetainedValue()
		if old != nil {
			if err := sqlast.PreflightValueContext(s.child.ctx, a.Name, old); err != nil {
				return err
			}
			cmp, err := sqlast.Compare(v, old)
			if err != nil {
				return err
			}
			if (a.Name == "min" && cmp >= 0) || (a.Name == "max" && cmp <= 0) {
				return nil
			}
		}
		cost := valuesSize([]any{v}) + 32
		if err := s.budget.reserve(cost); err != nil {
			return err
		}
		// Detach selected substrings/BLOBs from the borrowed input record. The old
		// retained charge remains live until the replacement allocation succeeds.
		switch x := v.(type) {
		case string:
			v = strings.Clone(x)
		case []byte:
			v = append([]byte{}, x...)
		}
		if err := a.Step(v); err != nil {
			s.budget.release(cost)
			return err
		}
		s.budget.release(s.retained[i])
		s.retained[i] = cost
		return nil
	}
	return a.Step(v)
}
func (s *aggregateSource) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	var err error
	if s.child != nil && s.child.source != nil {
		err = s.child.source.Close()
		s.child.source = nil
	}
	for _, n := range s.retained {
		s.budget.release(n)
	}
	s.child = nil
	s.states = nil
	s.retained = nil
	return err
}

// Rebind internal ON evaluation to the input scope before the root is lowered
// to aggregate slots. Restarts keep using this same input evaluator/snapshot.
func aggregateInput(src source, input *Rows) source {
	if j, ok := src.(*relationalJoin); ok {
		j.rows = input
		j.left = aggregateInput(j.left, input)
		right := j.rightFactory
		j.rightFactory = func() (source, error) {
			s, err := right()
			if err != nil {
				return nil, err
			}
			return aggregateInput(s, input), nil
		}
	}
	return src
}
