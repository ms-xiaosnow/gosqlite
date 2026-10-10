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
	"sparsevec":             builtinVectorFunction("sparsevec"),
	"sparsevec_dims":        builtinVectorFunction("sparsevec_dims"),
	"sparsevec_nnz":         builtinVectorFunction("sparsevec_nnz"),
	"sparsevec_norm":        builtinVectorFunction("sparsevec_norm"),
	"sparsevec_to_text":     builtinVectorFunction("sparsevec_to_text"),
	"sparsevec_from_vector": builtinVectorFunction("sparsevec_from_vector"),
	"vector_from_sparsevec": builtinVectorFunction("vector_from_sparsevec"),

	"halfvec":                    builtinVectorFunction("halfvec"),
	"halfvec_dims":               builtinVectorFunction("halfvec_dims"),
	"halfvec_norm":               builtinVectorFunction("halfvec_norm"),
	"halfvec_to_text":            builtinVectorFunction("halfvec_to_text"),
	"halfvec_from_vector":        builtinVectorFunction("halfvec_from_vector"),
	"vector_from_halfvec":        builtinVectorFunction("vector_from_halfvec"),
	"halfvec_conversion_changed": builtinVectorFunction("halfvec_conversion_changed"),
	"halfvec_conversion_max_abs": builtinVectorFunction("halfvec_conversion_max_abs"),
	"halfvec_conversion_max_rel": builtinVectorFunction("halfvec_conversion_max_rel"),
	"bit_vector":                 builtinVectorFunction("bit_vector"),
	"bit_dims":                   builtinVectorFunction("bit_dims"),
	"bit_to_text":                builtinVectorFunction("bit_to_text"),
	"hamming_distance":           builtinVectorFunction("hamming_distance"),
	"jaccard_distance":           builtinVectorFunction("jaccard_distance"),
	"subvector":                  builtinVectorFunction("subvector"),
	"binary_quantize":            builtinVectorFunction("binary_quantize"),
	"vector":                     builtinVectorFunction("vector"),
	"vector_dims":                builtinVectorFunction("vector_dims"),
	"vector_norm":                builtinVectorFunction("vector_norm"),
	"vector_to_text":             builtinVectorFunction("vector_to_text"),
	"l2_distance":                builtinVectorFunction("l2_distance"),
	"l1_distance":                builtinVectorFunction("l1_distance"),
	"cosine_distance":            builtinVectorFunction("cosine_distance"),
	"inner_product":              builtinVectorFunction("inner_product"),
	"negative_inner_product":     builtinVectorFunction("negative_inner_product"),
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
	case "sparsevec", "sparsevec_dims", "sparsevec_nnz", "sparsevec_norm", "sparsevec_to_text", "sparsevec_from_vector", "vector_from_sparsevec", "abs", "length", "lower", "upper", "typeof", "hex", "vector", "vector_dims", "vector_norm", "vector_to_text", "bit_vector", "bit_dims", "bit_to_text", "binary_quantize", "halfvec", "halfvec_dims", "halfvec_norm", "halfvec_to_text", "halfvec_from_vector", "vector_from_halfvec", "halfvec_conversion_changed", "halfvec_conversion_max_abs", "halfvec_conversion_max_rel":
	case "nullif", "ifnull", "bigram_similarity", "l2_distance", "l1_distance", "cosine_distance", "inner_product", "negative_inner_product", "hamming_distance", "jaccard_distance":
		want = 2
	case "subvector":
		want = 3
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
