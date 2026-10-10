package sqlite

import (
	"context"
	"io"

	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// A WHERE clause is a sequence of top-level AND terms, not one scalar Boolean
// value. SQLite's false-WHERE bypass evaluates deterministic row-independent
// terms first, in source order, even for an empty source or an empty index seek.
// Do not simplify across those term boundaries: err() AND 0 must still error.
type predicateProgram struct {
	constant, residual []sqlast.Expr
	outerON            bool
	fixedColumns       map[*sqlast.Column]sqlast.Expr
}

func compilePredicate(e sqlast.Expr, ec *sqlast.EvalContext, outerON bool, budget *relationalBudget) (*predicateProgram, error) {
	p := &predicateProgram{outerON: outerON, fixedColumns: ec.FixedColumns}
	var add func(sqlast.Expr) error
	add = func(e sqlast.Expr) error {
		if e == nil {
			return nil
		}
		if err := sqlast.ChargeExpression(ec, e); err != nil {
			return err
		}
		if x, ok := e.(*sqlast.Binary); ok && x.Op == "AND" {
			if err := add(x.Left); err != nil {
				return err
			}
			return add(x.Right)
		}
		// The full structural walk remains visible to budgets; determining
		// constancy never invokes functions or decodes typed values.
		if err := sqlast.ChargeExpressionTree(ec, e); err != nil {
			return err
		}
		if budget != nil {
			if err := budget.reserve(32); err != nil {
				return err
			}
		}
		if !outerON && sqlast.ConstantExpression(e, ec) {
			p.constant = append(p.constant, e)
		} else {
			p.residual = append(p.residual, e)
		}
		return nil
	}
	return p, add(e)
}

func (p *predicateProgram) evaluate(ec *sqlast.EvalContext) (bool, error) {
	copy := *ec
	copy.DisableBooleanSimplification = p.outerON
	copy.FixedColumns = p.fixedColumns
	for _, e := range p.residual {
		keep, err := sqlast.EvalPredicate(e, &copy)
		if err != nil || !keep {
			return false, err
		}
	}
	return true, nil
}

func (r *Rows) prepareWhere() error {
	// Single-source aliases are expanded after full binding/validation, just
	// as relational aliases already are. A WHERE alias must retain predicate
	// context rather than force its expression through the scalar resolver.
	if r.relational == nil {
		r.sel.Where = sqlast.Rewrite(r.sel.Where, func(e sqlast.Expr) sqlast.Expr {
			c, ok := e.(*sqlast.Column)
			if !ok || c.Table != "" {
				return e
			}
			if _, err := r.table.column(c.Table, c.Name); err == nil {
				return e
			}
			if i, ok := r.aliases[fold(c.Name)]; ok {
				return r.exprs[i]
			}
			return e
		})
	}
	if r.relational == nil {
		var err error
		r.sel.Where, err = r.clonePredicate(r.sel.Where)
		if err != nil {
			return err
		}
		if err := r.fixPredicateConstants([]sqlast.Expr{r.sel.Where}); err != nil {
			return err
		}
	}
	var err error
	r.where, err = compilePredicate(r.sel.Where, r.predicateContext(), false, r.relational)
	if err == nil {
		r.startupPredicates = append(r.startupPredicates, r.where.constant...)
	}
	return err
}

func (r *Rows) startPredicates() (err error) {
	if err := r.ctx.Err(); err != nil {
		return err
	}
	if r.predicatesStarted {
		return r.predicateStartErr
	}
	r.predicatesStarted = true
	// Startup is terminal even for false predicates or errors. ANN rechecks,
	// fallback scans and public search filters can enter this gate repeatedly.
	defer func() { r.predicateStartErr = err }()
	// A Rows evaluator may be assembled without the query planner. Give those
	// entry points the same compiled WHERE contract instead of skipping it.
	if r.where == nil && r.sel != nil {
		if err := r.prepareWhere(); err != nil {
			return err
		}
	}
	if r.relational != nil {
		defer r.relational.clearScalar()
	}
	ec := r.predicateContext()
	for _, e := range r.startupPredicates {
		keep, err := sqlast.EvalPredicate(e, ec)
		if err != nil {
			return err
		}
		if !keep {
			return io.EOF
		}
	}
	return nil
}

// DML uses the same false-WHERE bypass, before touching the first source row.
func constantPredicates(ctx context.Context, p *predicateProgram, params []any) (bool, error) {
	ec := &sqlast.EvalContext{Context: ctx, Params: params}
	for _, e := range p.constant {
		keep, err := sqlast.EvalPredicate(e, ec)
		if err != nil || !keep {
			return false, err
		}
	}
	return true, nil
}

// Every predicate occurrence gets its own Column identity. Alias expansion may
// otherwise share the same leaf with another occurrence or the projection.
func (r *Rows) clonePredicate(e sqlast.Expr) (sqlast.Expr, error) {
	ec := r.predicateContext()
	return sqlast.RewriteChecked(e, func(e sqlast.Expr) error {
		if err := sqlast.ChargeExpression(ec, e); err != nil {
			return err
		}
		if r.relational != nil {
			return r.relational.reserve(80)
		}
		return nil
	}, func(e sqlast.Expr) (sqlast.Expr, error) {
		if c, ok := e.(*sqlast.Column); ok {
			copy := *c
			return &copy, nil
		}
		return e, nil
	})
}
