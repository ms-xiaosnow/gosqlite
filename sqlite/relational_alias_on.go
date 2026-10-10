package sqlite

import sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"

// Pinned resolve.c resolveAlias replaces a direct result-alias reference with
// its projection expression, dropping the reference's EP_OuterON provenance.
// Only a whole top-level ON conjunct can thereby become a WHERE term. A parent
// operator (+alias, alias=0, alias OR 0, etc.) retains its outer-join provenance.
// Physical columns still win; unresolved TRUE/FALSE literals are not aliases.
func (s *relationalScope) extractOuterAliasTerms(original, bound sqlast.Expr, aliases map[string]sqlast.Expr) (sqlast.Expr, []sqlast.Expr, error) {
	var promoted []sqlast.Expr
	var visit func(sqlast.Expr, sqlast.Expr) (sqlast.Expr, error)
	visit = func(original, bound sqlast.Expr) (sqlast.Expr, error) {
		if original == nil {
			return bound, nil
		}
		if err := s.budget.step(relationalNodeWork(original)); err != nil {
			return nil, err
		}
		if x, ok := original.(*sqlast.Binary); ok && x.Op == "AND" {
			// Binding preserves operators; only an alias leaf can expand into a tree.
			b := bound.(*sqlast.Binary)
			left, err := visit(x.Left, b.Left)
			if err != nil {
				return nil, err
			}
			right, err := visit(x.Right, b.Right)
			if err != nil {
				return nil, err
			}
			if left == nil {
				return right, nil
			}
			if right == nil {
				return left, nil
			}
			if left == b.Left && right == b.Right {
				return bound, nil
			}
			if err = s.budget.reserve(80); err != nil {
				return nil, err
			}
			return &sqlast.Binary{Op: "AND", Left: left, Right: right}, nil
		}
		name, ok := orderResultName(original)
		if !ok {
			return bound, nil
		}
		if _, ok = aliases[fold(name)]; !ok {
			return bound, nil
		}
		_, physical, err := s.resolve("", name)
		if err != nil {
			return nil, err
		}
		if physical {
			return bound, nil
		}
		if err = s.budget.reserve(32); err != nil {
			return nil, err
		}
		promoted = append(promoted, bound)
		return nil, nil
	}
	residual, err := visit(original, bound)
	return residual, promoted, err
}
