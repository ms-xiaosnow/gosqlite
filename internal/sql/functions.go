package sql

import (
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

// BuiltinFunctions is read-only after initialization. Per-connection overrides
// and extension functions belong in EvalContext.Functions, avoiding global races.
var BuiltinFunctions = map[string]Function{
	"ngram_similarity": func(a []any) (any, error) {
		return similarityFunction(context.Background(), "ngram_similarity", a)
	},
	"bigram_similarity": func(a []any) (any, error) {
		return similarityFunction(context.Background(), "bigram_similarity", a)
	},
	"levenshtein": func(a []any) (any, error) {
		return similarityFunction(context.Background(), "levenshtein", a)
	},
	"abs": func(a []any) (any, error) {
		if err := ValidateFunction("abs", len(a)); err != nil {
			return nil, err
		}
		if a[0] == nil {
			return nil, nil
		}
		n := numeric(a[0])
		if i, ok := a[0].(int64); ok {
			if i == math.MinInt64 {
				return nil, fmt.Errorf("integer overflow")
			}
			if i < 0 {
				i = -i
			}
			return i, nil
		}
		return math.Abs(real(n)), nil
	},
	"length": func(a []any) (any, error) {
		if err := ValidateFunction("length", len(a)); err != nil {
			return nil, err
		}
		if a[0] == nil {
			return nil, nil
		}
		if b, ok := a[0].([]byte); ok {
			return int64(len(b)), nil
		}
		s := textValue(a[0])
		if !utf8.ValidString(s) {
			return nil, &ErrUnsupported{Feature: "invalid UTF-8 in length"}
		}
		if i := strings.IndexByte(s, 0); i >= 0 {
			s = s[:i]
		}
		return int64(utf8.RuneCountInString(s)), nil
	},
	"lower": func(a []any) (any, error) {
		if err := ValidateFunction("lower", len(a)); err != nil {
			return nil, err
		}
		if a[0] == nil {
			return nil, nil
		}
		return FoldName(textValue(a[0])), nil
	},
	"upper": func(a []any) (any, error) {
		if err := ValidateFunction("upper", len(a)); err != nil {
			return nil, err
		}
		if a[0] == nil {
			return nil, nil
		}
		b := []byte(textValue(a[0]))
		for i, c := range b {
			if c >= 'a' && c <= 'z' {
				b[i] = c - 'a' + 'A'
			}
		}
		return string(b), nil
	},
	"typeof": func(a []any) (any, error) {
		if err := ValidateFunction("typeof", len(a)); err != nil {
			return nil, err
		}
		switch a[0].(type) {
		case nil:
			return "null", nil
		case int64:
			return "integer", nil
		case float64:
			return "real", nil
		case string:
			return "text", nil
		case []byte:
			return "blob", nil
		}
		return nil, fmt.Errorf("unknown SQL value type %T", a[0])
	},
	"hex": func(a []any) (any, error) {
		if err := ValidateFunction("hex", len(a)); err != nil {
			return nil, err
		}
		return strings.ToUpper(hex.EncodeToString([]byte(textValue(a[0])))), nil
	},
	"nullif": func(a []any) (any, error) {
		if err := ValidateFunction("nullif", len(a)); err != nil {
			return nil, err
		}
		c, err := Compare(a[0], a[1])
		if err != nil {
			return nil, err
		}
		if c == 0 {
			return nil, nil
		}
		return a[0], nil
	},
}

// ValidateFunction validates the builtin name and arity independently of rows
// and argument values. Extension registries should validate their own functions.
func ValidateFunction(name string, argc int) error {
	name = FoldName(name)
	if argc > MaxFunctionArgs {
		return &ErrUnsupported{Feature: "function arguments exceed 1000"}
	}
	want := 1
	switch name {
	case "abs", "length", "lower", "upper", "typeof", "hex":
	case "nullif", "ifnull", "bigram_similarity":
		want = 2
	case "ngram_similarity":
		want = 4
	case "levenshtein":
		if argc == 2 || argc == 3 {
			return nil
		}
		want = 2
	case "coalesce":
		if argc >= 2 {
			return nil
		}
		want = 2
	default:
		return &ErrUnsupported{Feature: "function " + name}
	}
	if argc != want {
		return fmt.Errorf("wrong number of arguments to %s()", name)
	}
	return nil
}

// FoldName performs SQLite's ASCII-only identifier case folding. Unicode names
// remain distinct (for example, É and é are not interchangeable identifiers).
func FoldName(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 'a' - 'A'
		}
	}
	return string(b)
}
func EqualName(a, b string) bool { return FoldName(a) == FoldName(b) }

func upperName(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'a' && c <= 'z' {
			b[i] = c - 'a' + 'A'
		}
	}
	return string(b)
}
