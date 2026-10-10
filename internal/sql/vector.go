package sql

import (
	"context"
	"fmt"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

// VectorDimensions recognizes VECTOR(n) declarations with dimensions 1..16000.
// Parse emits the canonical uppercase form, while this helper also accepts
// case variations, ASCII whitespace and decimal leading zeros. A false result
// means the declaration is not a supported vector type; no dimension is inferred.
func VectorDimensions(declared string) (int, bool) {
	if len(declared) > MaxSQLBytes {
		return 0, false
	}
	s := trimSpace(declared)
	if len(s) < len("VECTOR(1)") || !EqualName(s[:6], "vector") {
		return 0, false
	}
	s = trimSpace(s[6:])
	if len(s) < 3 || s[0] != '(' || s[len(s)-1] != ')' {
		return 0, false
	}
	return vectorDimensionNumber(trimSpace(s[1 : len(s)-1]))
}

func vectorDimensionNumber(s string) (int, bool) {
	return boundedDimensionNumber(s, vector.MaxDimensions)
}

func boundedDimensionNumber(s string, limit int) (int, bool) {
	if len(s) == 0 {
		return 0, false
	}
	n := 0
	for i := range s {
		if !digit(s[i]) {
			return 0, false
		}
		n = n*10 + int(s[i]-'0')
		if n > limit {
			return 0, false
		}
	}
	return n, n > 0
}

// VectorValue converts one non-NULL SQL value without affinity or type coercion.
// Text must be a strict bracketed dense vector. BLOBs must contain the complete
// versioned binary codec, never textual bytes or PostgreSQL wire data. Callers
// implement their own NULL and declared-dimension rules. Returned vectors own
// their storage and retain the codec's finite-float32 validation and error types.
func VectorValue(value any) (vector.Vector, error) {
	switch v := value.(type) {
	case string:
		return vector.Parse(v)
	case []byte:
		return vector.Decode(v)
	default:
		return vector.Vector{}, fmt.Errorf("%w: SQL vector requires text or a versioned BLOB, got %T", vector.ErrInvalid, value)
	}
}

func vectorOperatorFunction(op string) string {
	switch op {
	case "<->":
		return "l2_distance"
	case "<#>":
		return "negative_inner_product"
	case "<=>":
		return "cosine_distance"
	case "<+>":
		return "l1_distance"
	case "<~>":
		return "hamming_distance"
	case "<%>":
		return "jaccard_distance"
	}
	return ""
}

func isVectorOperator(op string) bool { return vectorOperatorFunction(op) != "" }

func isVectorFunction(name string) bool {
	switch name {
	case "vector", "vector_dims", "vector_norm", "vector_to_text", "l2_distance", "l1_distance", "cosine_distance", "inner_product", "negative_inner_product", "bit_vector", "bit_dims", "bit_to_text", "hamming_distance", "jaccard_distance", "subvector", "binary_quantize":
		return true
	}
	return false
}

func builtinVectorFunction(name string) Function {
	return func(args []any) (any, error) {
		return vectorFunction(context.Background(), name, args)
	}
}

// vectorFunction preserves SQL NULL propagation before validating non-NULL
// operands. Each conversion is bounded by the vector codec's input/dimension
// limits, and cancellation is checked between conversions and after the bounded
// arithmetic/encoding operation. Zero-norm cosine returns vector.ErrZeroNorm.
func vectorFunction(ctx context.Context, name string, args []any) (any, error) {
	if !isVectorFunction(name) {
		return nil, &ErrUnsupported{Feature: "vector function " + name}
	}
	if err := ValidateFunction(name, len(args)); err != nil {
		return nil, err
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for _, value := range args {
		if value == nil {
			return nil, nil
		}
	}
	if isBitFunction(name) {
		return bitFunction(ctx, name, args)
	}
	a, err := VectorValue(args[0])
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var result any
	switch name {
	case "vector":
		result, err = a.MarshalBinary()
	case "vector_dims":
		result = int64(a.Dims())
	case "vector_norm":
		result, err = vector.Norm(a)
	case "vector_to_text":
		result = a.String()
	case "binary_quantize":
		var b vector.BitVector
		b, err = vector.BinaryQuantize(a)
		if err == nil {
			result, err = b.MarshalBinary()
		}
	case "subvector":
		start, e := subvectorInteger(args[1])
		if e != nil {
			return nil, e
		}
		count, e := subvectorInteger(args[2])
		if e != nil {
			return nil, e
		}
		var b vector.Vector
		b, err = vector.Subvector(a, start, count)
		if err == nil {
			result, err = b.MarshalBinary()
		}
	default:
		b, decodeErr := VectorValue(args[1])
		if decodeErr != nil {
			return nil, decodeErr
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		switch name {
		case "l2_distance":
			result, err = vector.L2Distance(a, b)
		case "l1_distance":
			result, err = vector.L1Distance(a, b)
		case "cosine_distance":
			result, err = vector.CosineDistance(a, b)
		case "inner_product":
			result, err = vector.InnerProduct(a, b)
		case "negative_inner_product":
			result, err = vector.NegativeInnerProduct(a, b)
		default:
			return nil, &ErrUnsupported{Feature: "vector function " + name}
		}
	}
	if err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
