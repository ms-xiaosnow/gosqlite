package sql

import "math"

// literalTruth is deliberately syntactic. SQLite 3.54 annotates small integer
// tokens and bound TRUE/FALSE, not floating literals, parameters, unary signs,
// arithmetic, or physical columns, with EP_IsTrue/EP_IsFalse.
func literalTruth(e Expr) (truth bool, known bool) {
	x, ok := e.(*Literal)
	if !ok || !(x.truthLiteral || x.Boolean) {
		return false, false
	}
	i, ok := x.Value.(int64)
	return i != 0, ok
}

func integerTruthLiteral(value any, raw string) bool {
	i, ok := value.(int64)
	if !ok || i < 0 || i > math.MaxInt32 {
		return false
	}
	for _, c := range raw {
		if c == '_' {
			return false
		}
	}
	return true
}

// simplifyBoolean follows sqlite3ExprSimplifiedAndOr without modifying the
// bound tree. It never evaluates an operand or assumes a function is pure.
func simplifyBoolean(e Expr, ctx *EvalContext) (Expr, error) {
	if ctx.DisableBooleanSimplification {
		return e, nil
	}
	x, ok := e.(*Binary)
	if !ok || x.Op != "AND" && x.Op != "OR" {
		return e, nil
	}
	if err := scalarStep(ctx, e); err != nil {
		return nil, err
	}
	right, err := simplifyBoolean(x.Right, ctx)
	if err != nil {
		return nil, err
	}
	left, err := simplifyBoolean(x.Left, ctx)
	if err != nil {
		return nil, err
	}
	lt, lk := literalTruth(left)
	rt, rk := literalTruth(right)
	if lk && lt || rk && !rt {
		if x.Op == "AND" {
			return right, nil
		}
		return left, nil
	}
	if rk && rt || lk && !lt {
		if x.Op == "AND" {
			return left, nil
		}
		return right, nil
	}
	return e, nil
}

// EvalPredicate is the branch-code context used by WHERE, ON and searched
// CASE. It does not compute a scalar AND/OR value: false and NULL both reject
// a row, so a NULL left conjunct can skip its right operand. NOT and IS TRUE/
// FALSE carry the branch polarity and NULL destination down the expression.
// The caller must bind/validate the complete expression before executing it.
func EvalPredicate(e Expr, ctx *EvalContext) (bool, error) {
	if ctx == nil {
		ctx = &EvalContext{}
	}
	reject, err := evalJump(e, ctx, false, true)
	return !reject, err
}

func evalJump(e Expr, ctx *EvalContext, want, jumpNull bool) (bool, error) {
	if ctx.Context != nil {
		if err := ctx.Context.Err(); err != nil {
			return false, err
		}
	}
	if err := scalarStep(ctx, e); err != nil {
		return false, err
	}
	switch x := e.(type) {
	case *Binary:
		if x.Op == "AND" || x.Op == "OR" {
			alt, err := simplifyBoolean(e, ctx)
			if err != nil {
				return false, err
			}
			if alt != e {
				return evalJump(alt, ctx, want, jumpNull)
			}
			firstWant, firstNull := want, jumpNull
			if (x.Op == "AND") == want {
				firstWant, firstNull = !want, !jumpNull
			}
			jump, err := evalJump(x.Left, ctx, firstWant, firstNull)
			if err != nil || jump {
				return firstWant == want && jump, err
			}
			return evalJump(x.Right, ctx, want, jumpNull)
		}
		if lit, ok := x.Right.(*Literal); ok && lit.Boolean && (x.Op == "IS" || x.Op == "IS NOT") {
			target := lit.Value == int64(1)
			negate := x.Op == "IS NOT"
			if want != negate {
				return evalJump(x.Left, ctx, target, false)
			}
			return evalJump(x.Left, ctx, !target, true)
		}
	case *Unary:
		if x.Op == "NOT" {
			return evalJump(x.Expr, ctx, !want, jumpNull)
		}
	case *Between:
		return evalBetweenJump(x, ctx, want, jumpNull)
	}
	v, err := Eval(e, ctx)
	if err != nil {
		return false, err
	}
	truth, null, err := TruthContext(ctx.Context, v)
	return null && jumpNull || !null && truth == want, err
}

// The BETWEEN lhs is evaluated once, but each comparison independently uses
// the original expression's affinity. Its second bound is branch-lazy only in
// predicate context; scalar BETWEEN remains eager.
func evalBetweenJump(x *Between, ctx *EvalContext, want, jumpNull bool) (bool, error) {
	if x.Not {
		want = !want
	}
	a, err := Eval(x.Expr, ctx)
	if err != nil {
		return false, err
	}
	compare := func(bound Expr, lower, target, nullTarget bool) (bool, error) {
		b, err := Eval(bound, ctx)
		if err != nil {
			return false, err
		}
		if a == nil || b == nil {
			return nullTarget, nil
		}
		av, bv, err := comparisonAffinity(x.Expr, bound, a, b, ctx)
		if err != nil {
			return false, err
		}
		cmp, err := Compare(av, bv)
		truth := cmp <= 0
		if lower {
			truth = cmp >= 0
		}
		return truth == target, err
	}
	jump, err := compare(x.Lower, true, false, jumpNull != want)
	if err != nil || jump {
		return !want && jump, err
	}
	return compare(x.Upper, false, want, jumpNull)
}

func booleanName(e Expr) bool {
	x, ok := e.(*Literal)
	return ok && x.Boolean
}

// sqlite3PExprIsNull folds known non-NULL literal tokens (including unary
// signs) before binding. NULL and fallback Boolean names are not folded here.
func parseNullCheck(x *Binary) Expr {
	if x.Op != "IS" && x.Op != "IS NOT" {
		return x
	}
	right, ok := x.Right.(*Literal)
	if !ok || right.Value != nil {
		return x
	}
	left := x.Left
	for {
		u, ok := left.(*Unary)
		if !ok || (u.Op != "+" && u.Op != "-") {
			break
		}
		left = u.Expr
	}
	if lit, ok := left.(*Literal); ok && lit.Value != nil && !lit.Boolean {
		return &Literal{Value: boolean(x.Op == "IS NOT"), truthLiteral: true}
	}
	return x
}

// SyntacticFalse identifies the pinned EP_IsFalse form without evaluating an
// expression. GROUP BY's HAVING-to-WHERE transform must leave it after grouping.
func SyntacticFalse(e Expr) bool { truth, known := literalTruth(e); return known && !truth }
