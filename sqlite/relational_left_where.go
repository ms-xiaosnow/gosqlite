package sqlite

import (
	"math"

	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// leftWherePrefilter is a structural proof, never an expression evaluator.
// Only one root LEFT JOIN of two ordinary named tables can qualify. A failed
// proof leaves the whole original WHERE in its existing post-join position.
func (s *relationalScope) leftWherePrefilter(original *sqlast.Select, bound sqlast.Expr, promoted bool) (bool, error) {
	if err := s.budget.step(1); err != nil {
		return false, err
	}
	if bound == nil || original.Having != nil || promoted {
		return false, nil
	}
	join, ok := original.Relation.(*sqlast.JoinRelation)
	if !ok || join.Kind != "LEFT" {
		return false, nil
	}
	left, lok := join.Left.(*sqlast.TableRelation)
	right, rok := join.Right.(*sqlast.TableRelation)
	if !lok || !rok || left.Name == "" || right.Name == "" || len(s.leaves) != 2 {
		return false, nil
	}
	for _, leaf := range s.leaves {
		if leaf.table == nil || leaf.table.root <= 1 {
			return false, nil // Exclude the synthetic schema table as well.
		}
		for _, col := range leaf.table.columns {
			if err := s.budget.step(1 + int64(len(col.Type))); err != nil {
				return false, err
			}
			// The existing schema parser canonicalizes these declarations and
			// rejects non-BINARY/collated and non-rowid definitions. No typed
			// vector value or default evaluation belongs in this proof.
			switch col.Type {
			case "", "INTEGER", "INT", "REAL", "TEXT", "NUMERIC", "BLOB":
			default:
				return false, nil
			}
			if col.Default != nil {
				ordinary, err := leftWhereLiteral(col.Default, s.budget)
				if err != nil || !ordinary {
					return false, err
				}
			}
		}
	}
	width := len(s.leaves[0].table.columns) + 1
	if s.leaves[0].offset != 0 || s.leaves[1].offset != width || s.slots == nil {
		return false, nil
	}
	var prove func(sqlast.Expr) (bool, error)
	prove = func(e sqlast.Expr) (bool, error) {
		if err := s.budget.step(relationalNodeWork(e)); err != nil {
			return false, err
		}
		x, ok := e.(*sqlast.Binary)
		if !ok {
			return false, nil
		}
		if x.Op == "AND" {
			ok, err := prove(x.Left)
			if err != nil || !ok {
				return false, err
			}
			return prove(x.Right)
		}
		if x.Op != "=" && x.Op != "==" {
			return false, nil
		}
		column, ok := x.Left.(*sqlast.Column)
		literal := x.Right
		if !ok {
			column, ok = x.Right.(*sqlast.Column)
			literal = x.Left
		}
		if !ok {
			return false, nil
		}
		if err := s.budget.step(relationalNodeWork(column)); err != nil {
			return false, err
		}
		i, found := s.slots.index[column.Name]
		if column.Table != "" || !found || i < 0 || i >= width || i >= len(s.slots.columns) || s.slots.columns[i].Name != column.Name {
			return false, nil
		}
		return leftWhereLiteral(literal, s.budget)
	}
	return prove(bound)
}

// Parameters are deliberately excluded even when their bound values happen to
// be scalars: binding can erase custom/typed argument provenance. A signed
// literal has exactly one unary operator directly around a numeric literal.
func leftWhereLiteral(e sqlast.Expr, budget *relationalBudget) (bool, error) {
	if err := budget.step(relationalNodeWork(e)); err != nil {
		return false, err
	}
	numeric := false
	if unary, ok := e.(*sqlast.Unary); ok {
		if unary.Op != "+" && unary.Op != "-" {
			return false, nil
		}
		e, numeric = unary.Expr, true
		if err := budget.step(relationalNodeWork(e)); err != nil {
			return false, err
		}
	}
	literal, ok := e.(*sqlast.Literal)
	if !ok {
		return false, nil
	}
	switch value := literal.Value.(type) {
	case int64:
		return true, nil
	case float64:
		return !math.IsInf(value, 0) && !math.IsNaN(value), nil
	case nil, string, []byte:
		return !numeric, nil
	}
	return false, nil
}
