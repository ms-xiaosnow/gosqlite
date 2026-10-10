package sql

// VisitChildren visits immediate children in source order, without allocating
// a child slice. Nil optional children are omitted. Keep all structural walkers
// here so validation and resource checks cannot miss a new expression form.
func VisitChildren(e Expr, visit func(Expr) error) error {
	child := func(e Expr) error {
		if e == nil {
			return nil
		}
		return visit(e)
	}
	switch x := e.(type) {
	case *Unary:
		return child(x.Expr)
	case *Binary:
		if err := child(x.Left); err != nil {
			return err
		}
		return child(x.Right)
	case *Like:
		if err := child(x.Expr); err != nil {
			return err
		}
		if err := child(x.Pattern); err != nil {
			return err
		}
		return child(x.Escape)
	case *Cast:
		return child(x.Expr)
	case *Between:
		if err := child(x.Expr); err != nil {
			return err
		}
		if err := child(x.Lower); err != nil {
			return err
		}
		return child(x.Upper)
	case *InList:
		if err := child(x.Expr); err != nil {
			return err
		}
		for _, v := range x.Values {
			if err := child(v); err != nil {
				return err
			}
		}
	case *Case:
		if err := child(x.Base); err != nil {
			return err
		}
		for _, arm := range x.Arms {
			if err := child(arm.When); err != nil {
				return err
			}
			if err := child(arm.Then); err != nil {
				return err
			}
		}
		return child(x.Else)
	case *Call:
		for _, arg := range x.Args {
			if err := child(arg); err != nil {
				return err
			}
		}
	}
	return nil
}

// Walk visits every expression, including unselected CASE arms, before its
// children. Binding/validation must not depend on the row's runtime value.
func Walk(e Expr, visit func(Expr) error) error {
	if e == nil {
		return nil
	}
	if err := visit(e); err != nil {
		return err
	}
	return VisitChildren(e, func(child Expr) error { return Walk(child, visit) })
}

// Rewrite makes an immutable bottom-up rewrite. Composite nodes and their
// child slices are copied; leaves are shared unless the callback replaces them.
// In particular boolean name binding never mutates a parsed/prepared tree.
func Rewrite(e Expr, visit func(Expr) Expr) Expr {
	if e == nil {
		return nil
	}
	r := func(e Expr) Expr { return Rewrite(e, visit) }
	switch x := e.(type) {
	case *Unary:
		y := *x
		y.Expr = r(x.Expr)
		e = &y
	case *Binary:
		y := *x
		y.Left, y.Right = r(x.Left), r(x.Right)
		e = &y
	case *Like:
		y := *x
		y.Expr, y.Pattern, y.Escape = r(x.Expr), r(x.Pattern), r(x.Escape)
		e = &y
	case *Cast:
		y := *x
		y.Expr = r(x.Expr)
		e = &y
	case *Between:
		y := *x
		y.Expr, y.Lower, y.Upper = r(x.Expr), r(x.Lower), r(x.Upper)
		e = &y
	case *InList:
		y := *x
		y.Expr = r(x.Expr)
		y.Values = make([]Expr, len(x.Values))
		for i, v := range x.Values {
			y.Values[i] = r(v)
		}
		e = &y
	case *Case:
		y := *x
		y.Base, y.Else = r(x.Base), r(x.Else)
		y.Arms = make([]CaseArm, len(x.Arms))
		for i, a := range x.Arms {
			y.Arms[i] = CaseArm{When: r(a.When), Then: r(a.Then)}
		}
		e = &y
	case *Call:
		y := *x
		y.Args = make([]Expr, len(x.Args))
		for i, a := range x.Args {
			y.Args[i] = r(a)
		}
		e = &y
	}
	return visit(e)
}
