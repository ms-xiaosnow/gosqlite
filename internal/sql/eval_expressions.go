package sql

// evalCase never evaluates unselected result arms. Simple CASE evaluates its
// base exactly once, then applies the same operand affinity as equality.
func evalCase(x *Case, ctx *EvalContext) (any, error) {
	var base any
	var err error
	if x.Base != nil {
		base, err = Eval(x.Base, ctx)
		if err != nil {
			return nil, err
		}
	}
	for _, arm := range x.Arms {
		v, err := Eval(arm.When, ctx)
		if err != nil {
			return nil, err
		}
		var match bool
		if x.Base == nil {
			match, _ = Truth(v)
		} else if base != nil && v != nil {
			a, b, err := comparisonAffinity(x.Base, arm.When, base, v, ctx)
			if err != nil {
				return nil, err
			}
			cmp, err := Compare(a, b)
			if err != nil {
				return nil, err
			}
			match = cmp == 0
		}
		if match {
			return Eval(arm.Then, ctx)
		}
	}
	if x.Else != nil {
		return Eval(x.Else, ctx)
	}
	return nil, nil
}

func evalBetween(x *Between, ctx *EvalContext) (any, error) {
	a, err := Eval(x.Expr, ctx)
	if err != nil {
		return nil, err
	}
	lo, err := Eval(x.Lower, ctx)
	if err != nil {
		return nil, err
	}
	hi, err := Eval(x.Upper, ctx)
	if err != nil {
		return nil, err
	}
	// Each comparison starts from the original value: the two bound
	// affinities may differ. A false comparison dominates a NULL comparison.
	unknown, result := false, true
	for i, bound := range []any{lo, hi} {
		if a == nil || bound == nil {
			unknown = true
			continue
		}
		be := x.Lower
		if i == 1 {
			be = x.Upper
		}
		av, bv, err := comparisonAffinity(x.Expr, be, a, bound, ctx)
		if err != nil {
			return nil, err
		}
		cmp, err := Compare(av, bv)
		if err != nil {
			return nil, err
		}
		if i == 0 && cmp < 0 || i == 1 && cmp > 0 {
			result = false
		}
	}
	if result && unknown {
		return nil, nil
	}
	if x.Not {
		result = !result
	}
	return boolean(result), nil
}

func evalInList(x *InList, ctx *EvalContext) (any, error) {
	// SQLite permits the empty list. It is constant false (true for NOT IN),
	// including with NULL or an error-producing left operand.
	if len(x.Values) == 0 {
		return boolean(x.Not), nil
	}
	eager := len(x.Values) > 2
	if eager {
		for _, expr := range x.Values {
			if !ConstantExpression(expr, ctx) {
				eager = false
				break
			}
		}
	}
	// SQLite builds a >2-item constant RHS before testing membership, so
	// errors in later constant values must not disappear after a match.
	// Keep streaming instead of materializing arbitrary-sized RHS values.
	// Defer any lhs error to preserve eager-RHS error precedence as well.
	a, lhsErr := Eval(x.Expr, ctx)
	if lhsErr != nil && !eager {
		return nil, lhsErr
	}
	unknown, matched := a == nil, false
	for _, expr := range x.Values {
		b, err := Eval(expr, ctx)
		if err != nil {
			return nil, err
		}
		if lhsErr != nil || matched {
			continue
		}
		if a == nil || b == nil {
			unknown = true
			continue
		}
		// IN-list operands have no affinity, even columns and CAST nodes.
		// The left operand still supplies its own affinity to each comparison.
		av, bv, err := comparisonAffinity(x.Expr, nil, a, b, ctx)
		if err != nil {
			return nil, err
		}
		cmp, err := Compare(av, bv)
		if err != nil {
			return nil, err
		}
		if cmp == 0 {
			if !eager {
				return boolean(!x.Not), nil
			}
			matched = true
		}
	}
	if lhsErr != nil {
		return nil, lhsErr
	}
	if matched {
		return boolean(!x.Not), nil
	}
	if unknown {
		return nil, nil
	}
	return boolean(x.Not), nil
}

// ConstantExpression models the pinned sqlite3ExprIsConstant rule for this
// scalar subset, after binding. Parameters are statement constants. It is
// deliberately not value folding: dead arms and physical columns still count,
// and unknown/context-supplied functions have no determinism guarantee.
func ConstantExpression(e Expr, ctx *EvalContext) bool {
	switch x := e.(type) {
	case nil:
		return true
	case *Literal, *Parameter:
		return true
	case *Column:
		return ctx != nil && ctx.ConstantColumn != nil && ctx.ConstantColumn(x.Table, x.Name)
	case *Call:
		name := FoldName(x.Name)
		if name != "coalesce" && name != "ifnull" && ctx != nil && ctx.Functions[name] != nil {
			return false
		}
		switch name {
		case "abs", "length", "lower", "upper", "typeof", "hex", "nullif", "ifnull", "coalesce":
		default:
			// Extension scalar constructors/metrics are pure functions of
			// their arguments; snapshot-backed custom functions are not.
			if !isVectorFunction(name) && !isSimilarityFunction(name) {
				return false
			}
		}
	case *Unary, *Binary, *Like, *Cast, *Case, *Between, *InList:
	default:
		return false
	}
	constant := true
	_ = VisitChildren(e, func(child Expr) error { constant = constant && ConstantExpression(child, ctx); return nil })
	return constant
}

func castValue(v any, typ string) any {
	if v == nil {
		return nil
	}
	switch castAffinity(typ) {
	case "BLOB":
		if b, ok := v.([]byte); ok {
			return b
		}
		return []byte(textValue(v))
	case "TEXT":
		return textValue(v)
	case "INTEGER":
		return integerValue(v)
	case "REAL":
		return real(numeric(v))
	default:
		// NUMERIC preserves an already numeric storage class. Text/BLOB
		// decimal prefixes use integer storage only on a lossless 51-bit
		// floating-point round trip, or on an exact integer-text parse.
		switch v.(type) {
		case int64, float64:
			return v
		}
		n := numeric(v)
		if f, ok := n.(float64); ok && f >= -2251799813685248.0 && f < 2251799813685248.0 && float64(int64(f)) == f {
			return int64(f)
		}
		return n
	}
}

// ExpressionAffinity exposes ordinary scalar affinity independently of vector
// representation identity. Unary + and CASE have no comparison affinity.
func ExpressionAffinity(e Expr, declared func(string, string) string) string {
	switch x := e.(type) {
	case *Column:
		if declared != nil {
			return declared(x.Table, x.Name)
		}
	case *Cast:
		return castAffinity(x.Type)
	}
	return ""
}

// An explicit CAST type, including an empty typename, defaults to NUMERIC.
// An absent column declaration instead has BLOB affinity (AffinityType).
func castAffinity(typ string) string {
	if typ == "" {
		return "NUMERIC"
	}
	return AffinityType(typ)
}
