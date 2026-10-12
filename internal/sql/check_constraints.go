package sql

// WalkCheckSource validates a bounded, unfurled ordinary scalar source tree.
// It also protects descriptor construction from manually assembled ASTs, cycles,
// oversized child slices and typed nil nodes. It never evaluates or folds a node.
// The caller must separately resolve every source column, including dead arms.
func WalkCheckSource(e Expr, visit func(Expr) error) (int, error) {
	return walkCheckSource(e, MaxCheckNodes, visit)
}

// WalkCheckDefaultSource is the early shape guard for defaults in a
// CHECK-bearing descriptor. Parsed DEFAULT trees contain no columns, calls or
// parameters. The existing SQL token bound limits total default source nodes;
// defaults do not consume the separate CHECK declaration-node allowance.
func WalkCheckDefaultSource(e Expr, visit func(Expr) error) (int, error) {
	return walkCheckSource(e, MaxTokens, func(e Expr) error {
		if _, column := e.(*Column); column {
			return &ErrUnsupported{Feature: "CHECK table nonconstant DEFAULT"}
		}
		if visit != nil {
			return visit(e)
		}
		return nil
	})
}

func walkCheckSource(e Expr, maxNodes int, visit func(Expr) error) (int, error) {
	nodes := 0
	bad := func(feature string) error { return &ErrUnsupported{Feature: "CHECK " + feature} }
	var walk func(Expr, int) error
	walk = func(e Expr, depth int) error {
		if depth > maxExpressionDepth || nodes >= maxNodes {
			return bad("source node or depth budget")
		}
		nodes++
		switch x := e.(type) {
		case *Literal:
			if x == nil {
				return bad("nil literal")
			}
			switch x.Value.(type) {
			case nil, int64, float64, string, []byte:
			default:
				return bad("literal storage class")
			}
			if x.Boolean && x.Value != int64(0) && x.Value != int64(1) {
				return bad("Boolean literal shape")
			}
		case *Column:
			if x == nil {
				return bad("column shape")
			}
		case *Unary:
			if x == nil || x.Expr == nil {
				return bad("unary shape")
			}
			switch x.Op {
			case "+", "-", "~", "NOT":
			default:
				return bad("unary operator")
			}
		case *Binary:
			if x == nil || x.Left == nil || x.Right == nil {
				return bad("binary shape")
			}
			switch x.Op {
			case "OR", "AND", "=", "==", "!=", "<>", "IS", "IS NOT", "<", ">", "<=", ">=", "&", "|", "<<", ">>", "+", "-", "*", "/", "%", "||":
			default:
				return bad("binary operator")
			}
		case *Like:
			if x == nil || x.Expr == nil || x.Pattern == nil {
				return bad("LIKE shape")
			}
		case *Cast:
			if x == nil || x.Expr == nil || len(x.Type) > MaxSQLBytes {
				return bad("CAST shape")
			}
		case *Between:
			if x == nil || x.Expr == nil || x.Lower == nil || x.Upper == nil {
				return bad("BETWEEN shape")
			}
		case *InList:
			if x == nil || x.Expr == nil || len(x.Values) > maxNodes {
				return bad("IN shape")
			}
			for _, value := range x.Values {
				if value == nil {
					return bad("nil IN value")
				}
			}
		case *Case:
			if x == nil || len(x.Arms) == 0 || len(x.Arms) > maxNodes/2 {
				return bad("CASE shape")
			}
			for _, arm := range x.Arms {
				if arm.When == nil || arm.Then == nil {
					return bad("nil CASE arm")
				}
			}
		default:
			return bad("requires row-local scalars without calls, parameters or stars")
		}
		if visit != nil {
			if err := visit(e); err != nil {
				return err
			}
		}
		return VisitChildren(e, func(child Expr) error { return walk(child, depth+1) })
	}
	err := walk(e, 1)
	return nodes, err
}

func (p *parser) parseNullCheck(x *Binary) Expr {
	if p.checkMode {
		return x
	}
	return parseNullCheck(x)
}

func (p *parser) appendCheckConstraint(s *CreateTable, name string, terms *int) error {
	if len(s.CheckConstraints) >= MaxCheckConstraints {
		return p.unsupported("CHECK declaration budget")
	}
	if err := p.need("CHECK"); err != nil {
		return err
	}
	if err := p.need("("); err != nil {
		return err
	}
	start := p.tokens[p.pos-1].End
	p.checkMode = true
	expr, err := p.expression(0)
	p.checkMode = false
	if err != nil {
		return err
	}
	end := p.peek().Start
	if err = p.need(")"); err != nil {
		return err
	}
	nodes, err := WalkCheckSource(expr, nil)
	if err != nil {
		return err
	}
	if nodes > MaxCheckNodes-*terms {
		return p.unsupported("total CHECK source node budget")
	}
	*terms += nodes
	s.CheckConstraints = append(s.CheckConstraints, CheckConstraint{Named: p.constraintNamed, Name: name, Text: p.text[start:end], Start: start, End: end, Expr: expr})
	return nil
}

// NormalizeCheckSource copies an already admitted tree and applies only the
// existing parse-time folds. Boolean name binding happens after this function.
// UPDATE dependency discovery must use this result, never the unfurled source.
// Do not apply execution-time Boolean simplification to dependency discovery.
func NormalizeCheckSource(e Expr) Expr {
	return Rewrite(e, func(e Expr) Expr {
		switch x := e.(type) {
		case *Literal:
			y := *x
			if blob, ok := x.Value.([]byte); ok {
				y.Value = append([]byte(nil), blob...)
			}
			return &y
		case *Column:
			y := *x
			return &y
		case *InList:
			if len(x.Values) == 0 {
				return &Literal{Value: boolean(x.Not), truthLiteral: true}
			}
		case *Binary:
			if x.Op == "AND" {
				lt, lk := literalTruth(x.Left)
				rt, rk := literalTruth(x.Right)
				if lk && !lt && !booleanName(x.Left) || rk && !rt && !booleanName(x.Right) {
					return &Literal{Value: int64(0), truthLiteral: true}
				}
			}
			return parseNullCheck(x)
		}
		return e
	})
}

// CheckSourceLabel avoids retaining a generated rendering of a large tree.
func CheckSourceLabel(check CheckConstraint) string {
	if check.Named || check.Name != "" {
		return check.Name
	}
	if check.Text != "" {
		return check.Text
	}
	return "anonymous CHECK"
}
