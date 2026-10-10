package sql

// RewriteChecked is a copy-on-rewrite walk with an admission check before
// visiting children or allocating each copied node. Both callbacks may stop the
// traversal; replacements are not revisited (aliases may share bound trees).
func RewriteChecked(e Expr, enter func(Expr) error, visit func(Expr) (Expr, error)) (Expr, error) {
	return rewriteChecked(e, enter, visit, nil)
}

// RewritePrunedChecked replaces a complete expression before its children,
// and stops at accepted replacements. This preserves GROUP BY expression
// identity when both a complete key and one of its subexpressions are keys.
func RewritePrunedChecked(e Expr, enter func(Expr) error, replace func(Expr) (Expr, bool, error)) (Expr, error) {
	return rewriteChecked(e, enter, func(e Expr) (Expr, error) { return e, nil }, replace)
}
func rewriteChecked(e Expr, enter func(Expr) error, visit func(Expr) (Expr, error), replace func(Expr) (Expr, bool, error)) (Expr, error) {
	if e == nil {
		return nil, nil
	}
	if err := enter(e); err != nil {
		return nil, err
	}
	if replace != nil {
		out, stop, err := replace(e)
		if err != nil || stop {
			return out, err
		}
	}
	r := func(e Expr) (Expr, error) { return rewriteChecked(e, enter, visit, replace) }
	var err error
	switch x := e.(type) {
	case *Unary:
		y := *x
		y.Expr, err = r(x.Expr)
		e = &y
	case *Binary:
		y := *x
		y.Left, err = r(x.Left)
		if err == nil {
			y.Right, err = r(x.Right)
		}
		e = &y
	case *Like:
		y := *x
		y.Expr, err = r(x.Expr)
		if err == nil {
			y.Pattern, err = r(x.Pattern)
		}
		if err == nil {
			y.Escape, err = r(x.Escape)
		}
		e = &y
	case *Cast:
		y := *x
		y.Expr, err = r(x.Expr)
		e = &y
	case *Between:
		y := *x
		y.Expr, err = r(x.Expr)
		if err == nil {
			y.Lower, err = r(x.Lower)
		}
		if err == nil {
			y.Upper, err = r(x.Upper)
		}
		e = &y
	case *InList:
		y := *x
		y.Expr, err = r(x.Expr)
		if err != nil {
			return nil, err
		}
		y.Values = make([]Expr, len(x.Values))
		for i, v := range x.Values {
			y.Values[i], err = r(v)
			if err != nil {
				return nil, err
			}
		}
		e = &y
	case *Case:
		y := *x
		y.Base, err = r(x.Base)
		if err == nil {
			y.Else, err = r(x.Else)
		}
		if err != nil {
			return nil, err
		}
		y.Arms = make([]CaseArm, len(x.Arms))
		for i, a := range x.Arms {
			y.Arms[i].When, err = r(a.When)
			if err != nil {
				return nil, err
			}
			y.Arms[i].Then, err = r(a.Then)
			if err != nil {
				return nil, err
			}
		}
		e = &y
	case *Call:
		y := *x
		y.Args = make([]Expr, len(x.Args))
		for i, a := range x.Args {
			y.Args[i], err = r(a)
			if err != nil {
				return nil, err
			}
		}
		e = &y
	}
	if err != nil {
		return nil, err
	}
	return visit(e)
}
