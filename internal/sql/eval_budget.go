package sql

import (
	"context"
	"fmt"
	"math"
)

// EvalBudget is supplied by the owning statement. Scalar reservations remain
// charged until that owner finishes evaluating its current row; this includes
// intermediate buffers whose results or backing bytes may still be retained.
// Existing callers without a budget retain their existing scalar limits.
type EvalBudget interface {
	Step(int64) error
	Allocate(int64) error
}
type evalBudgetKey struct{}

func WithEvalBudget(ctx context.Context, budget EvalBudget) context.Context {
	return context.WithValue(ctx, evalBudgetKey{}, budget)
}
func evaluationBudget(ctx *EvalContext) EvalBudget {
	if ctx == nil || ctx.Context == nil {
		return nil
	}
	b, _ := ctx.Context.Value(evalBudgetKey{}).(EvalBudget)
	return b
}
func scalarStep(ctx *EvalContext, e Expr) error {
	if b := evaluationBudget(ctx); b != nil {
		n := int64(0)
		switch x := e.(type) {
		case *Column:
			if ctx.DeclaredType != nil {
				n = int64(len(ctx.DeclaredType(x.Table, x.Name)))
			}
		case *Call:
			n = int64(len(x.Name))
		case *Cast:
			n = int64(len(x.Type))
		}
		if err := b.Step(1 + 8*n); err != nil {
			return err
		}
		if n > 0 {
			return b.Allocate(4 * n)
		}
		return nil
	}
	return nil
}
func scalarAllocate(ctx *EvalContext, n int64) error {
	if b := evaluationBudget(ctx); b != nil {
		return b.Allocate(n)
	}
	return nil
}
func scalarValueBytes(v any) int64 {
	switch x := v.(type) {
	case string:
		return int64(len(x))
	case []byte:
		return int64(len(x))
	default:
		return 32
	}
}

// Preflight allocations and scalar CPU work on already evaluated operands.
// Ordinary coercions/concat/case-fold/hex need at most 8x operand bytes. Vector
// codecs/text formatting are conservatively charged 128x encoded inputs plus
// bounded dense-expansion scratch. Similarity/tokenization is charged 1024x;
// LIKE uses 64x for its rune slices and append-grown term buffers.
// Edit-distance and LIKE additionally charge their worst-case cell comparisons.
// These are conservative quotas, not claims about allocator/RSS precision.
func scalarPreflight(ctx *EvalContext, op string, args ...any) error {
	b := evaluationBudget(ctx)
	if b == nil {
		return nil
	}
	// NULL-propagating extension/match operations allocate no operand scratch.
	if isVectorFunction(op) || isSimilarityFunction(op) || op == "grams13_match" || op == "LIKE" || op == "<->" || op == "<#>" || op == "<=>" || op == "<+>" || op == "<~>" || op == "<%>" {
		for _, v := range args {
			if v == nil {
				return nil
			}
		}
	}
	n := int64(0)
	floatScratch := int64(0)
	for _, v := range args {
		size := scalarValueBytes(v)
		if size > math.MaxInt64-n {
			return fmt.Errorf("scalar allocation size overflow")
		}
		n += size
		// realText's bounded big.Int decimal staging needs scratch beyond
		// its short formatted result (float64 exponents are bounded).
		if _, ok := v.(float64); ok {
			floatScratch += 4096
		}
	}
	factor, base := int64(8), int64(128)+floatScratch
	if op == "LIKE" {
		// Simultaneous pattern/value rune slices and append-grown 8-byte
		// terms (including old/new growth buffers), plus BLOB coercion.
		factor = 64
	}
	if isVectorFunction(op) || op == "<->" || op == "<#>" || op == "<=>" || op == "<+>" || op == "<~>" || op == "<%>" {
		factor = 128
		// Sparse-to-dense is the only legal conversion whose output size need not
		// be proportional to input support. Dense dimensions are capped at 16,000.
		if op == "vector_from_sparsevec" {
			base += 16000 * 32
		}
	}
	if isSimilarityFunction(op) || op == "grams13_match" {
		factor = 1024
	}
	if n > (math.MaxInt64-base)/factor {
		return fmt.Errorf("scalar allocation size overflow")
	}
	if err := b.Allocate(n*factor + base); err != nil {
		return err
	}

	workFactor := int64(8)
	if isVectorFunction(op) || isVectorOperator(op) {
		workFactor = 128
	}
	if isSimilarityFunction(op) || op == "grams13_match" {
		workFactor = 1024
	}
	if n > math.MaxInt64/workFactor {
		return b.Step(math.MaxInt64)
	}
	if err := b.Step(n * workFactor); err != nil {
		return err
	}
	if op == "vector_from_sparsevec" {
		if err := b.Step(16000); err != nil {
			return err
		}
	}
	left, right := 0, 1
	if op == "grams13_match" {
		left, right = 1, 2
	}
	if op == "LIKE" || op == "levenshtein" || op == "grams13_match" {
		if len(args) > right {
			a, c := scalarValueBytes(args[left])+1, scalarValueBytes(args[right])+1
			if a > math.MaxInt64/c {
				return b.Step(math.MaxInt64)
			}
			return b.Step(a * c)
		}
	}
	return nil
}

// TruthContext charges numeric TEXT/BLOB truth coercion before scanning/copying.
// It is also used by ON/WHERE, which execute outside the scalar AST evaluator.
func TruthContext(ctx context.Context, v any) (bool, bool, error) {
	ec := &EvalContext{Context: ctx}
	if err := scalarPreflight(ec, "truth", v); err != nil {
		return false, false, err
	}
	truth, null := Truth(v)
	return truth, null, nil
}

// PreflightValueContext is for scalar conversions owned by a query clause,
// such as LIMIT/OFFSET, rather than an expression node.
func PreflightValueContext(ctx context.Context, op string, values ...any) error {
	return scalarPreflight(&EvalContext{Context: ctx}, op, values...)
}
func scalarTreeWork(ctx *EvalContext, e Expr) error {
	b := evaluationBudget(ctx)
	if b == nil {
		return nil
	}
	return Walk(e, func(e Expr) error { return scalarStep(ctx, e) })
}
