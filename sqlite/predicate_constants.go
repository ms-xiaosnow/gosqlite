package sqlite

import (
	"fmt"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// Maximum expanded nodes for any one propagated constant. This is an
// expression-expansion bound, in addition to runtime operation quotas.
const maxPredicateExpansionWork int64 = 1 << 20

// SQLite's EP_FixedCol retains column affinity while substituting a WHERE
// equality constant. A context-local map has the same separation without
// mutating the parser tree, aliases, projection expressions, or schema nodes.
func (r *Rows) predicateContext() *sqlast.EvalContext {
	ec := &sqlast.EvalContext{Context: r.ctx, Params: r.params, Functions: r.functions, FixedColumns: r.fixedColumns, DeclaredType: declaredTypeFor(r.table)}
	ec.Affinity = func(q, n string) string {
		i, err := r.table.column(q, n)
		if err != nil {
			return ""
		}
		if i < 0 {
			return "INTEGER"
		}
		return affinity(r.table.columns[i].Type)
	}
	return ec
}

func (r *Rows) fixPredicateConstants(expressions []sqlast.Expr) error {
	ec := r.predicateContext()
	fixed := map[*sqlast.Column]sqlast.Expr{}
	weights := map[*sqlast.Column]int64{}
	ec.FixedColumns = fixed
	type equality struct {
		column *sqlast.Column
		value  sqlast.Expr
		weight int64
	}
	ordinal := func(c *sqlast.Column) (int, bool) {
		i, err := r.table.column(c.Table, c.Name)
		if err != nil {
			return 0, false
		}
		if i >= 0 && sqlast.IsVectorDeclaration(r.table.columns[i].Type) {
			return 0, false
		}
		if i == r.table.ipk {
			i = -1
		}
		return i, true
	}
	// Repeated passes expose transitive equalities. Each pass must fix at
	// least one previously-unfixed node; the bounded statement tree terminates.
	for pass := 0; ; pass++ {
		if pass > sqlast.MaxColumns {
			return fmt.Errorf("%w: predicate constant propagation", ErrQueryWorkLimit)
		}
		known := map[int]equality{}
		var collect func(sqlast.Expr) error
		collect = func(e sqlast.Expr) error {
			if e == nil {
				return nil
			}
			if err := sqlast.ChargeExpression(ec, e); err != nil {
				return err
			}
			b, ok := e.(*sqlast.Binary)
			if !ok {
				return nil
			}
			if b.Op == "AND" {
				if err := collect(b.Right); err != nil {
					return err
				}
				return collect(b.Left)
			}
			if b.Op != "=" && b.Op != "==" {
				return nil
			}
			insert := func(cExpr, value sqlast.Expr) error {
				c, ok := cExpr.(*sqlast.Column)
				if !ok || fixed[c] != nil {
					return nil
				}
				id, ok := ordinal(c)
				if !ok {
					return nil
				}
				if _, ok := known[id]; ok {
					return nil
				}
				if err := sqlast.ChargeExpressionTree(ec, value); err != nil {
					return err
				}
				if !sqlast.ConstantExpression(value, ec) || sqlast.ExpressionAffinity(value, ec.Affinity) != "" || !ordinaryConstantResult(value) {
					return nil
				}
				if r.relational != nil {
					if err := r.relational.reserve(192); err != nil {
						return err
					}
				}
				// Account for the expanded evaluator graph without traversing it.
				// Even legacy single-source queries must not acquire exponential
				// work merely because of this planner transformation.
				var weight int64
				if err := sqlast.Walk(value, func(node sqlast.Expr) error {
					n := int64(1)
					if col, ok := node.(*sqlast.Column); ok && weights[col] != 0 {
						n = weights[col]
					}
					if n > maxPredicateExpansionWork-weight {
						return fmt.Errorf("%w: predicate constant expansion", ErrQueryWorkLimit)
					}
					weight += n
					return nil
				}); err != nil {
					return err
				}
				known[id] = equality{c, value, weight}
				return nil
			}
			if err := insert(b.Right, b.Left); err != nil {
				return err
			}
			return insert(b.Left, b.Right)
		}
		// SQLite visits the right side of the combined AND tree first.
		for i := len(expressions) - 1; i >= 0; i-- {
			if err := collect(expressions[i]); err != nil {
				return err
			}
		}
		if len(known) == 0 {
			break
		}
		changed := 0
		mark := func(c *sqlast.Column, allowBlob bool) error {
			if fixed[c] != nil {
				return nil
			}
			id, ok := ordinal(c)
			if !ok {
				return nil
			}
			eq, ok := known[id]
			if !ok || eq.column == c {
				return nil
			}
			if !allowBlob && ec.Affinity(c.Table, c.Name) == "BLOB" {
				return nil
			}
			if r.relational != nil {
				if err := r.relational.reserve(192); err != nil {
					return err
				}
			}
			if eq.weight >= maxPredicateExpansionWork {
				return fmt.Errorf("%w: predicate constant expansion", ErrQueryWorkLimit)
			}
			fixed[c] = eq.value
			weights[c] = eq.weight + 1
			changed++
			return nil
		}
		for _, e := range expressions {
			if err := sqlast.Walk(e, func(e sqlast.Expr) error {
				if err := sqlast.ChargeExpression(ec, e); err != nil {
					return err
				}
				if b, ok := e.(*sqlast.Binary); ok {
					switch b.Op {
					case "=", "==", "<", "<=", ">", ">=", "IS":
						if c, ok := b.Left.(*sqlast.Column); ok {
							if err := mark(c, true); err != nil {
								return err
							}
						}
						if sqlast.ExpressionAffinity(b.Left, ec.Affinity) != "TEXT" {
							if c, ok := b.Right.(*sqlast.Column); ok {
								if err := mark(c, true); err != nil {
									return err
								}
							}
						}
					}
				}
				if c, ok := e.(*sqlast.Column); ok {
					return mark(c, false)
				}
				return nil
			}); err != nil {
				return err
			}
		}
		if changed == 0 {
			break
		}
	}
	if len(fixed) > 0 {
		r.fixedColumns = fixed
	}
	return nil
}

// Extension representations are intentionally not propagated as ordinary SQL
// constants: doing so could erase their representation/subtype validation.
func ordinaryConstantResult(e sqlast.Expr) bool {
	safe := true
	_ = sqlast.Walk(e, func(e sqlast.Expr) error {
		switch x := e.(type) {
		case *sqlast.Call:
			switch sqlast.FoldName(x.Name) {
			case "abs", "length", "lower", "upper", "typeof", "hex", "nullif", "ifnull", "coalesce":
			default:
				safe = false
			}
		case *sqlast.Binary:
			switch x.Op {
			case "<->", "<#>", "<=>", "<+>", "<~>", "<%>":
				safe = false
			}
		}
		return nil
	})
	return safe
}
