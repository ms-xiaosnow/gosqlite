package sql

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"math/big"
	"math/bits"
	"strconv"
	"strings"
	"unicode/utf8"
)

type Function func([]any) (any, error)
type EvalContext struct {
	// Context is the actual operation context for bounded extension functions.
	// A nil value retains Background semantics for pure internal evaluation.
	Context  context.Context
	Row      map[string]any
	Params   []any
	Resolver func(table, name string) (any, error)
	Affinity func(table, name string) string
	// ConstantColumn reports a bound constant result alias, never a physical
	// column whose current row value happens to be known. Used by IN-list
	// evaluation to preserve SQLite's constant-RHS error behavior.
	ConstantColumn func(table, name string) bool
	// DeclaredType preserves representation identity separately from SQL affinity.
	// Derived aliases may return a closed vector/halfvec/bit/mixed kind tag.
	DeclaredType func(table, name string) string
	Functions    map[string]Function
}

func Eval(e Expr, ctx *EvalContext) (any, error) {
	if ctx == nil {
		ctx = &EvalContext{}
	}
	if ctx.Context != nil {
		if err := ctx.Context.Err(); err != nil {
			return nil, err
		}
	}
	if err := scalarStep(ctx, e); err != nil {
		return nil, err
	}
	switch x := e.(type) {
	case *Literal:
		return normalize(x.Value)
	case *Column:
		if ctx.Resolver != nil {
			v, err := ctx.Resolver(x.Table, x.Name)
			if err != nil {
				return nil, err
			}
			return normalize(v)
		}
		key := x.Name
		if x.Table != "" {
			key = x.Table + "." + key
		}
		for n, v := range ctx.Row {
			if EqualName(n, key) {
				return normalize(v)
			}
		}
		return nil, fmt.Errorf("no such column: %s", key)
	case *Parameter:
		if x.Index < 1 || x.Index > len(ctx.Params) {
			return nil, fmt.Errorf("parameter %d is unbound", x.Index)
		}
		return normalize(ctx.Params[x.Index-1])
	case *Star:
		return nil, fmt.Errorf("* is not a scalar expression")
	case *Unary:
		v, err := Eval(x.Expr, ctx)
		if err != nil || v == nil {
			return nil, err
		}
		if err := scalarPreflight(ctx, x.Op, v); err != nil {
			return nil, err
		}
		switch x.Op {
		case "+":
			return v, nil
		case "NOT":
			t, _ := Truth(v)
			return boolean(!t), nil
		case "~":
			return ^integerValue(v), nil
		case "-":
			n := numeric(v)
			if i, ok := n.(int64); ok && i != math.MinInt64 {
				return -i, nil
			}
			return -real(n), nil
		}
		return nil, &ErrUnsupported{Feature: "unary operator " + x.Op}
	case *Binary:
		a, err := Eval(x.Left, ctx)
		if err != nil {
			return nil, err
		}
		b, err := Eval(x.Right, ctx)
		if err != nil {
			return nil, err
		}
		if err := scalarPreflight(ctx, x.Op, a, b); err != nil {
			return nil, err
		}
		switch x.Op {
		case "<->", "<#>", "<=>", "<+>", "<~>", "<%>":
			if a != nil && b != nil {
				if err := scalarTreeWork(ctx, x.Left); err != nil {
					return nil, err
				}
				if err := scalarTreeWork(ctx, x.Right); err != nil {
					return nil, err
				}
			}
			return vectorExpressionFunction(ctx, vectorOperatorFunction(x.Op), []Expr{x.Left, x.Right}, []any{a, b})
		case "AND":
			at, an := Truth(a)
			bt, bn := Truth(b)
			if !an && !at || !bn && !bt {
				return int64(0), nil
			}
			if an || bn {
				return nil, nil
			}
			return int64(1), nil
		case "OR":
			at, an := Truth(a)
			bt, bn := Truth(b)
			if !an && at || !bn && bt {
				return int64(1), nil
			}
			if an || bn {
				return nil, nil
			}
			return int64(0), nil
		case "IS", "IS NOT":
			var same bool
			if lit, ok := x.Right.(*Literal); ok && lit.Boolean {
				v, isNull := Truth(a)
				same = !isNull && v == (lit.Value == int64(1))
			} else if a == nil || b == nil {
				same = a == nil && b == nil
			} else {
				a, b, err = comparisonAffinity(x.Left, x.Right, a, b, ctx)
				if err != nil {
					return nil, err
				}
				cmp, err := Compare(a, b)
				if err != nil {
					return nil, err
				}
				same = cmp == 0
			}
			if x.Op == "IS NOT" {
				same = !same
			}
			return boolean(same), nil
		}
		if a == nil || b == nil {
			return nil, nil
		}
		switch x.Op {
		case "=", "==", "!=", "<>", "<", ">", "<=", ">=":
			a, b, err = comparisonAffinity(x.Left, x.Right, a, b, ctx)
			if err != nil {
				return nil, err
			}
			c, err := Compare(a, b)
			if err != nil {
				return nil, err
			}
			var result bool
			switch x.Op {
			case "=", "==":
				result = c == 0
			case "!=", "<>":
				result = c != 0
			case "<":
				result = c < 0
			case ">":
				result = c > 0
			case "<=":
				result = c <= 0
			case ">=":
				result = c >= 0
			}
			return boolean(result), nil
		case "||":
			return textValue(a) + textValue(b), nil
		case "&", "|", "<<", ">>":
			return arithmetic(x.Op, integerValue(a), integerValue(b)), nil
		case "%":
			an, bn := numeric(a), numeric(b)
			_, ai := an.(int64)
			_, bi := bn.(int64)
			if ai && bi {
				return arithmetic("%", an, bn), nil
			}
			av, bv := integerValue(a), integerValue(b)
			if bv == 0 {
				return nil, nil
			}
			return float64(av % bv), nil
		case "+", "-", "*", "/":
			return normalize(arithmetic(x.Op, numeric(a), numeric(b)))
		}
		return nil, &ErrUnsupported{Feature: "binary operator " + x.Op}
	case *Like:
		a, err := Eval(x.Expr, ctx)
		if err != nil {
			return nil, err
		}
		pattern, err := Eval(x.Pattern, ctx)
		if err != nil {
			return nil, err
		}
		switch pattern.(type) {
		case []byte, int64, float64:
			if err := scalarPreflight(ctx, "pattern text", pattern); err != nil {
				return nil, err
			}
		}
		if len(textValue(pattern)) > MaxLikePatternBytes {
			return nil, fmt.Errorf("LIKE pattern exceeds 50000 bytes")
		}
		var escape *rune
		if x.Escape != nil {
			e, err := Eval(x.Escape, ctx)
			if err != nil {
				return nil, err
			}
			if e == nil {
				return nil, nil
			}
			if err := scalarPreflight(ctx, "ESCAPE", e); err != nil {
				return nil, err
			}
			s := textValue(e)
			if !utf8.ValidString(s) {
				return nil, &ErrUnsupported{Feature: "invalid UTF-8 in LIKE ESCAPE"}
			}
			if i := strings.IndexByte(s, 0); i >= 0 {
				s = s[:i]
			}
			if utf8.RuneCountInString(s) != 1 {
				return nil, fmt.Errorf("ESCAPE expression must be a single character")
			}
			r, _ := utf8.DecodeRuneInString(s)
			escape = &r
		}
		if a == nil || pattern == nil {
			return nil, nil
		}
		if err := scalarPreflight(ctx, "LIKE", a, pattern); err != nil {
			return nil, err
		}
		valueText, err := LikeText(a)
		if err != nil {
			return nil, err
		}
		patternText, err := LikeText(pattern)
		if err != nil {
			return nil, err
		}
		matched := LikeMatch(valueText, patternText, escape)
		if x.Not {
			matched = !matched
		}
		return boolean(matched), nil
	case *Case:
		return evalCase(x, ctx)
	case *Cast:
		v, err := Eval(x.Expr, ctx)
		if err != nil {
			return nil, err
		}
		if err := scalarPreflight(ctx, "CAST", v); err != nil {
			return nil, err
		}
		return castValue(v, x.Type), nil
	case *Between:
		return evalBetween(x, ctx)
	case *InList:
		return evalInList(x, ctx)
	case *Call:
		name := FoldName(x.Name)
		if name == "coalesce" || name == "ifnull" {
			if name == "ifnull" && len(x.Args) != 2 || name == "coalesce" && len(x.Args) < 2 {
				return nil, fmt.Errorf("wrong number of arguments to %s()", name)
			}
			for _, arg := range x.Args {
				v, err := Eval(arg, ctx)
				if err != nil {
					return nil, err
				}
				if v != nil {
					return v, nil
				}
			}
			return nil, nil
		}
		if err := scalarAllocate(ctx, int64(len(x.Args))*16); err != nil {
			return nil, err
		}
		args := make([]any, len(x.Args))
		for i, arg := range x.Args {
			v, err := Eval(arg, ctx)
			if err != nil {
				return nil, err
			}
			args[i] = v
		}
		if err := scalarPreflight(ctx, name, args...); err != nil {
			return nil, err
		}
		fn := ctx.Functions[name]
		if fn == nil && isSimilarityFunction(name) {
			return similarityFunction(ctx.Context, name, args)
		}
		if fn == nil && isVectorFunction(name) {
			nonnull := true
			for _, a := range args {
				if a == nil {
					nonnull = false
				}
			}
			if nonnull {
				for _, e := range x.Args {
					if err := scalarTreeWork(ctx, e); err != nil {
						return nil, err
					}
				}
			}
			return vectorExpressionFunction(ctx, name, x.Args, args)
		}
		if fn == nil {
			fn = BuiltinFunctions[name]
		}
		if fn == nil {
			return nil, &ErrUnsupported{Feature: "function " + x.Name}
		}
		v, err := fn(args)
		if err != nil {
			return nil, err
		}
		return normalize(v)
	default:
		return nil, fmt.Errorf("invalid scalar expression %T", e)
	}
}
func normalize(v any) (any, error) {
	switch x := v.(type) {
	case nil, int64, string, []byte:
		return v, nil
	case float64:
		if math.IsNaN(x) {
			return nil, nil
		}
		return x, nil
	case int:
		return int64(x), nil
	case int8:
		return int64(x), nil
	case int16:
		return int64(x), nil
	case int32:
		return int64(x), nil
	case uint8:
		return int64(x), nil
	case uint16:
		return int64(x), nil
	case uint32:
		return int64(x), nil
	case uint:
		if uint64(x) <= math.MaxInt64 {
			return int64(x), nil
		}
	case uint64:
		if x <= math.MaxInt64 {
			return int64(x), nil
		}
	case float32:
		return normalize(float64(x))
	case bool:
		return boolean(x), nil
	}
	return nil, fmt.Errorf("unsupported SQL value type %T", v)
}
func boolean(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

// Truth implements SQLite's numeric boolean conversion. isNull is true only
// for SQL NULL, so WHERE includes a row exactly when value is true and non-NULL.
func Truth(v any) (value, isNull bool) {
	if v == nil {
		return false, true
	}
	return real(numeric(v)) != 0, false
}

// Compare orders storage classes NULL < number < text < blob, without affinity.
func Compare(a, b any) (int, error) {
	a, err := normalize(a)
	if err != nil {
		return 0, err
	}
	b, err = normalize(b)
	if err != nil {
		return 0, err
	}
	ra, rb := rank(a), rank(b)
	if ra < rb {
		return -1, nil
	}
	if ra > rb {
		return 1, nil
	}
	switch av := a.(type) {
	case nil:
		return 0, nil
	case int64:
		switch bv := b.(type) {
		case int64:
			if av < bv {
				return -1, nil
			}
			if av > bv {
				return 1, nil
			}
			return 0, nil
		case float64:
			return compareIntReal(av, bv), nil
		}
	case float64:
		if bv, ok := b.(int64); ok {
			return -compareIntReal(bv, av), nil
		}
		bv := b.(float64)
		if av < bv {
			return -1, nil
		}
		if av > bv {
			return 1, nil
		}
		return 0, nil
	case string:
		return strings.Compare(av, b.(string)), nil
	case []byte:
		return bytes.Compare(av, b.([]byte)), nil
	}
	return 0, fmt.Errorf("cannot compare %T and %T", a, b)
}
func rank(v any) int {
	switch v.(type) {
	case nil:
		return 0
	case int64, float64:
		return 1
	case string:
		return 2
	case []byte:
		return 3
	}
	return 4
}
func compareIntReal(i int64, f float64) int {
	if f < float64(math.MinInt64) {
		return 1
	}
	if f >= 9223372036854775808.0 {
		return -1
	}
	j := int64(f)
	if i < j {
		return -1
	}
	if i > j {
		return 1
	}
	g := float64(i)
	if g < f {
		return -1
	}
	if g > f {
		return 1
	}
	return 0
}
func real(v any) float64 {
	switch x := v.(type) {
	case int64:
		return float64(x)
	case float64:
		return x
	}
	return 0
}
func integer(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case float64:
		if x >= 9223372036854775808.0 {
			return math.MaxInt64
		}
		if x <= float64(math.MinInt64) {
			return math.MinInt64
		}
		return int64(x)
	}
	return 0
}
func numeric(v any) any {
	switch x := v.(type) {
	case int64, float64:
		return x
	case string:
		return numericText(x, false)
	case []byte:
		return numericText(string(x), false)
	default:
		n, err := normalize(v)
		if err == nil && n != v {
			return numeric(n)
		}
		return int64(0)
	}
}
func asciiSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f' || c == '\v'
}
func trimSpace(s string) string {
	for len(s) > 0 && asciiSpace(s[0]) {
		s = s[1:]
	}
	for len(s) > 0 && asciiSpace(s[len(s)-1]) {
		s = s[:len(s)-1]
	}
	return s
}

// numericText uses SQLite's decimal prefix conversion for arithmetic, and full
// decimal conversion (without hexadecimal interpretation) for column affinity.
func numericText(raw string, strict bool) any {
	s := trimSpace(raw)
	i := 0
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		i++
	}
	digits := 0
	for i < len(s) && digit(s[i]) {
		i++
		digits++
	}
	fp := false
	if i < len(s) && s[i] == '.' {
		fp = true
		i++
		for i < len(s) && digit(s[i]) {
			i++
			digits++
		}
	}
	if digits == 0 {
		if strict {
			return raw
		}
		return int64(0)
	}
	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		save := i
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		start := i
		for i < len(s) && digit(s[i]) {
			i++
		}
		if start == i {
			i = save
		} else {
			fp = true
		}
	}
	if strict && i != len(s) {
		return raw
	}
	s = s[:i]
	if !fp {
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return n
		}
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil && !math.IsInf(f, 0) {
		if strict {
			return raw
		}
		return int64(0)
	}
	if strict && !math.IsInf(f, 0) && f > float64(math.MinInt64) && f < 9223372036854775808.0 && float64(int64(f)) == f {
		return int64(f)
	}
	return f
}

// AffinityType implements SQLite declared-type affinity rules.
func AffinityType(declared string) string {
	s := upperName(declared)
	switch {
	case strings.Contains(s, "INT"):
		return "INTEGER"
	case strings.Contains(s, "CHAR") || strings.Contains(s, "CLOB") || strings.Contains(s, "TEXT"):
		return "TEXT"
	case s == "" || strings.Contains(s, "BLOB"):
		return "BLOB"
	case strings.Contains(s, "REAL") || strings.Contains(s, "FLOA") || strings.Contains(s, "DOUB"):
		return "REAL"
	default:
		return "NUMERIC"
	}
}
func ApplyAffinity(v any, declared string) (any, error) {
	v, err := normalize(v)
	if err != nil || v == nil {
		return v, err
	}
	aff := AffinityType(declared)
	switch aff {
	case "TEXT":
		switch v.(type) {
		case int64, float64:
			return textValue(v), nil
		}
	case "INTEGER", "NUMERIC", "REAL":
		if s, ok := v.(string); ok {
			v = numericText(s, true)
		}
		if aff == "REAL" {
			if n, ok := v.(int64); ok {
				v = float64(n)
			}
		} else if f, ok := v.(float64); ok && f > float64(math.MinInt64) && f < 9223372036854775808.0 && float64(int64(f)) == f {
			v = int64(f)
		}
	}
	return v, nil
}
func comparisonAffinity(ae, be Expr, a, b any, ctx *EvalContext) (any, any, error) {
	if err := scalarPreflight(ctx, "affinity", a, b); err != nil {
		return nil, nil, err
	}
	for _, e := range []Expr{ae, be} {
		if c, ok := e.(*Cast); ok {
			if err := scalarStep(ctx, c); err != nil {
				return nil, nil, err
			}
		}
	}
	affinity := func(e Expr) string {
		a := ExpressionAffinity(e, ctx.Affinity)
		if a == "" {
			return ""
		}
		return AffinityType(a)
	}
	aa, ba := affinity(ae), affinity(be)
	num := func(s string) bool { return s == "INTEGER" || s == "REAL" || s == "NUMERIC" }
	var err error
	if num(aa) && !num(ba) {
		b, err = ApplyAffinity(b, "NUMERIC")
	} else if num(ba) && !num(aa) {
		a, err = ApplyAffinity(a, "NUMERIC")
	} else if aa == "TEXT" && (ba == "" || ba == "BLOB") {
		b, err = ApplyAffinity(b, "TEXT")
	} else if ba == "TEXT" && (aa == "" || aa == "BLOB") {
		a, err = ApplyAffinity(a, "TEXT")
	}
	return a, b, err
}

// LikeText is the LIKE evaluator's exact SQL text coercion and UTF-8 check.
// It preserves NUL and validates the entire input; matching/tokenization apply
// first-NUL termination afterward. Callers handle SQL NULL separately.
func LikeText(v any) (string, error) {
	s := textValue(v)
	if !utf8.ValidString(s) {
		return "", &ErrUnsupported{Feature: "invalid UTF-8 in LIKE"}
	}
	return s, nil
}

// TextValue is the evaluator's SQL text coercion, shared with extension indexing.
// NULL converts to empty text here; callers must preserve their own NULL semantics.
func TextValue(v any) string { return textValue(v) }

func textValue(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case []byte:
		return string(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return realText(x)
	}
	return ""
}
func arithmetic(op string, a, b any) any {
	ai, aok := a.(int64)
	bi, bok := b.(int64)
	switch op {
	case "+":
		if aok && bok {
			v := ai + bi
			if (bi >= 0 && v >= ai) || (bi < 0 && v < ai) {
				return v
			}
		}
		return real(a) + real(b)
	case "-":
		if aok && bok {
			v := ai - bi
			if (bi >= 0 && v <= ai) || (bi < 0 && v > ai) {
				return v
			}
		}
		return real(a) - real(b)
	case "*":
		if aok && bok {
			if ai == 0 || bi == 0 {
				return int64(0)
			}
			v := ai * bi
			if !(ai == math.MinInt64 && bi == -1 || bi == math.MinInt64 && ai == -1) && v/bi == ai {
				return v
			}
		}
		return real(a) * real(b)
	case "/":
		if real(b) == 0 {
			return nil
		}
		if aok && bok && !(ai == math.MinInt64 && bi == -1) {
			return ai / bi
		}
		return real(a) / real(b)
	case "%":
		a1, b1 := integer(a), integer(b)
		if b1 == 0 {
			return nil
		}
		v := a1 % b1
		if !aok || !bok {
			return float64(v)
		}
		return v
	case "&":
		return integer(a) & integer(b)
	case "|":
		return integer(a) | integer(b)
	case "<<", ">>":
		shift := integer(b)
		left := op == "<<"
		if shift < 0 {
			left = !left
			if shift == math.MinInt64 {
				shift = 64
			} else {
				shift = -shift
			}
		}
		v := integer(a)
		if shift >= 64 {
			if !left && v < 0 {
				return int64(-1)
			}
			return int64(0)
		}
		if left {
			return v << uint(shift)
		}
		return v >> uint(shift)
	}
	return nil
}

// LikeMatch implements SQLite LIKE's rune-sized '_' and '%' wildcards, ASCII
// case-insensitivity, and a single optional Unicode escape rune. NUL terminates
// text as in SQLite's built-in LIKE implementation. It uses bounded linear space.
func LikeMatch(value, pattern string, escape *rune) bool {
	if i := strings.IndexByte(value, 0); i >= 0 {
		value = value[:i]
	}
	if i := strings.IndexByte(pattern, 0); i >= 0 {
		pattern = pattern[:i]
	}
	type term struct {
		r    rune
		wild byte
	}
	var terms []term
	pr := []rune(pattern)
	for i := 0; i < len(pr); i++ {
		r := pr[i]
		if escape != nil && r == *escape {
			i++
			if i >= len(pr) {
				return false
			}
			terms = append(terms, term{r: asciiFold(pr[i])})
			continue
		}
		if r == '%' {
			if len(terms) == 0 || terms[len(terms)-1].wild != '%' {
				terms = append(terms, term{wild: '%'})
			}
		} else if r == '_' {
			terms = append(terms, term{wild: '_'})
		} else {
			terms = append(terms, term{r: asciiFold(r)})
		}
	}
	// Greedy wildcard backtracking is linear-space; only the latest '%' needs
	// remembering because it can consume every intervening prefix.
	vr := []rune(value)
	i, j, star, resume := 0, 0, -1, 0
	for i < len(vr) {
		if j < len(terms) && terms[j].wild == '%' {
			star = j
			j++
			resume = i
			continue
		}
		if j < len(terms) && (terms[j].wild == '_' || terms[j].wild == 0 && terms[j].r == asciiFold(vr[i])) {
			i++
			j++
			continue
		}
		if star >= 0 {
			resume++
			i = resume
			j = star + 1
			continue
		}
		return false
	}
	for j < len(terms) && terms[j].wild == '%' {
		j++
	}
	return j == len(terms)
}
func asciiFold(r rune) rune {
	if r >= 'A' && r <= 'Z' {
		return r + ('a' - 'A')
	}
	return r
}

// MaxLikePatternBytes matches the default SQLite LIKE pattern resource limit.
const MaxLikePatternBytes = 50000

// integerValue performs SQLite's CAST-to-integer style conversion. For TEXT and
// BLOB, unlike arithmetic numeric coercion, an exponent is not part of the prefix.
func integerValue(v any) int64 {
	switch x := v.(type) {
	case int64:
		return x
	case float64:
		return integer(x)
	case []byte:
		return integerValue(string(x))
	case string:
		s := trimSpace(x)
		i := 0
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		start := i
		for i < len(s) && digit(s[i]) {
			i++
		}
		if i == start {
			return 0
		}
		n, err := strconv.ParseInt(s[:i], 10, 64)
		if err == nil {
			return n
		}
		if s[0] == '-' {
			return math.MinInt64
		}
		return math.MaxInt64
	}
	return 0
}

// realText follows the pinned SQLite 3.54 default numeric-to-text precision:
// 17 significant digits, with redundant 0/9 tails shortened only when the
// corresponding decimal round-trips. This is intentionally not Go's universal
// shortest representation, which differs for subnormal and some 16-digit values.
func realText(x float64) string {
	if math.IsInf(x, 1) {
		return "Inf"
	}
	if math.IsInf(x, -1) {
		return "-Inf"
	}
	if x == 0 {
		return "0.0"
	}
	sign := ""
	if x < 0 {
		sign = "-"
		x = -x
	}

	// Reconstruct the public-domain SQLite 3.54 decimal staging behavior with
	// native Go arithmetic. Compute normalized powers on demand rather than
	// copying upstream's lookup tables. Truncating the normalized 64-bit power
	// before multiplication is observable at a handful of rounding boundaries.
	binary := math.Float64bits(x)
	mantissa := binary & ((uint64(1) << 52) - 1)
	exponent2 := int(binary >> 52)
	if exponent2 == 0 {
		shift := bits.LeadingZeros64(mantissa)
		mantissa <<= uint(shift)
		exponent2 = -1074 - shift
	} else {
		mantissa = (mantissa << 11) | (uint64(1) << 63)
		exponent2 -= 1086
	}
	power := 17 - (((exponent2 + 63) * 78913) >> 18)
	power2 := (power * 108853) >> 15
	numerator, denominator := big.NewInt(1), big.NewInt(1)
	factor := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(absInt(power))), nil)
	if power >= 0 {
		numerator.Set(factor)
	} else {
		denominator.Set(factor)
	}
	shift := 63 - power2
	if shift >= 0 {
		numerator.Lsh(numerator, uint(shift))
	} else {
		denominator.Lsh(denominator, uint(-shift))
	}
	powerMantissa := new(big.Int).Quo(numerator, denominator).Uint64()
	high, _ := bits.Mul64(mantissa, powerMantissa)
	high >>= uint(-(exponent2 + power2 + 2))
	staged := (high + ((high << 1) & 2)) >> 1
	digits := strconv.FormatUint(staged, 10)
	exponent := len(digits) - 1 - power

	precision := 17
	tryShort := func(n int, up bool) bool {
		value, _ := strconv.ParseUint(digits[:n], 10, 64)
		if up {
			value++
		}
		candidate := strconv.FormatUint(value, 10) + "e" + strconv.Itoa(exponent+1-n)
		parsed, err := strconv.ParseFloat(candidate, 64)
		return (err == nil || math.IsInf(parsed, 0)) && parsed == x
	}
	if digits[14] == '9' && digits[15] == '9' {
		n := 14
		for n > 0 && digits[n-1] == '9' {
			n--
		}
		if n == 0 {
			if parsed, _ := strconv.ParseFloat("1e"+strconv.Itoa(exponent+1), 64); parsed == x {
				precision = 1
			}
		} else if tryShort(n, true) {
			precision = n + 1
		}
	} else if exponent+1 >= len(digits) || (digits[13] == '0' && digits[14] == '0' && digits[15] == '0') {
		n := 13
		for n > 1 && digits[n-1] == '0' {
			n--
		}
		if tryShort(n, false) {
			precision = n + 1
		}
	}
	rounded := []byte(digits[:precision])
	if digits[precision] >= '5' {
		carry := true
		for i := len(rounded) - 1; i >= 0 && carry; i-- {
			if rounded[i] == '9' {
				rounded[i] = '0'
			} else {
				rounded[i]++
				carry = false
			}
		}
		if carry {
			rounded = append([]byte{'1'}, rounded...)
			exponent++
		}
	}
	d := strings.TrimRight(string(rounded), "0")
	if d == "" {
		d = "0"
	}
	if exponent < -4 || exponent >= 17 {
		fraction := "0"
		if len(d) > 1 {
			fraction = d[1:]
		}
		expSign := "+"
		e := exponent
		if e < 0 {
			expSign = "-"
			e = -e
		}
		exp := strconv.Itoa(e)
		if len(exp) < 2 {
			exp = "0" + exp
		}
		return sign + d[:1] + "." + fraction + "e" + expSign + exp
	}
	point := exponent + 1
	if point <= 0 {
		return sign + "0." + strings.Repeat("0", -point) + d
	}
	if point >= len(d) {
		return sign + d + strings.Repeat("0", point-len(d)) + ".0"
	}
	return sign + d[:point] + "." + d[point:]
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
