package sql

import (
	"context"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

// HalfDimensions recognizes only HALFVEC(n), n=1..16000. The identity is
// separate from dense VECTOR and its ANN capabilities.
func HalfDimensions(declared string) (int, bool) {
	if len(declared) > MaxSQLBytes {
		return 0, false
	}
	s := trimSpace(declared)
	if len(s) < len("HALFVEC(1)") || !EqualName(s[:7], "halfvec") {
		return 0, false
	}
	s = trimSpace(s[7:])
	if len(s) < 3 || s[0] != '(' || s[len(s)-1] != ')' {
		return 0, false
	}
	return boundedDimensionNumber(trimSpace(s[1:len(s)-1]), vector.MaxHalfDimensions)
}

// HalfValue accepts half text or strict GSH v1 only. GSV never implicitly
// quantizes: use HalfFromVector or the explicit halfvec_from_vector function.
func HalfValue(value any) (vector.HalfVector, error) {
	switch v := value.(type) {
	case string:
		return vector.ParseHalf(v)
	case []byte:
		return vector.DecodeHalf(v)
	default:
		return vector.HalfVector{}, fmt.Errorf("%w: SQL halfvec requires text or a versioned GSH BLOB, got %T", vector.ErrInvalid, value)
	}
}

// HalfValueDims rejects oversized input and mismatched dimensions before
// allocating decoded component storage. Validation still uses the strict codec.
func HalfValueDims(value any, dimensions int) (vector.HalfVector, error) {
	switch v := value.(type) {
	case string:
		if len(v) > vector.MaxTextBytes {
			return vector.HalfVector{}, vector.ErrLimit
		}
		if strings.Count(v, ",")+1 != dimensions {
			return vector.HalfVector{}, vector.ErrDimensions
		}
	case []byte:
		if len(v) < vector.HalfBinaryHeaderSize || len(v) > vector.MaxHalfBinaryBytes || string(v[:4]) != "GSH\x00" {
			return vector.HalfVector{}, vector.ErrEncoding
		}
		if uint64(binary.BigEndian.Uint32(v[8:12])) != uint64(dimensions) {
			return vector.HalfVector{}, vector.ErrDimensions
		}
	}
	v, err := HalfValue(value)
	if err == nil {
		err = v.RequireDims(dimensions)
	}
	return v, err
}

func isHalfFunction(name string) bool {
	switch name {
	case "halfvec", "halfvec_dims", "halfvec_norm", "halfvec_to_text", "halfvec_from_vector", "vector_from_halfvec", "halfvec_conversion_changed", "halfvec_conversion_max_abs", "halfvec_conversion_max_rel":
		return true
	}
	return false
}

func halfOverload(name string) bool {
	switch name {
	case "l2_distance", "l1_distance", "cosine_distance", "inner_product", "negative_inner_product", "subvector", "binary_quantize":
		return true
	}
	return false
}

// Codec headers select a signature, never validate it: full decoding always
// follows. A malformed/unknown BLOB cannot become a text value or dense value.
func isHalfBlob(value any) bool {
	b, ok := value.([]byte)
	return ok && len(b) >= 4 && string(b[:4]) == "GSH\x00"
}

// VectorExpressionRepresentation carries known representation identity through
// value-preserving expression forms. Empty means untyped; "mixed" rejects
// implicit cross-kind choice. It does not infer types from arbitrary text.
func VectorExpressionRepresentation(e Expr, declared func(string, string) string) string {
	switch x := e.(type) {
	case *Column:
		if declared == nil {
			return ""
		}
		d := declared(x.Table, x.Name)
		switch d {
		case "halfvec", "vector", "sparsevec", "bit", "mixed":
			return d
		}
		if _, ok := SparseDimensions(d); ok {
			return "sparsevec"
		}
		if _, ok := HalfDimensions(d); ok {
			return "halfvec"
		}
		if _, ok := VectorDimensions(d); ok {
			return "vector"
		}
		if _, ok := BitDimensions(d); ok {
			return "bit"
		}
	case *Unary:
		if x.Op == "+" {
			return VectorExpressionRepresentation(x.Expr, declared)
		}
	case *Call:
		switch FoldName(x.Name) {
		case "halfvec", "halfvec_from_vector":
			return "halfvec"
		case "sparsevec", "sparsevec_from_vector":
			return "sparsevec"
		case "vector", "vector_from_halfvec", "vector_from_sparsevec":
			return "vector"
		case "bit_vector", "binary_quantize":
			return "bit"
		case "subvector", "nullif":
			if len(x.Args) > 0 {
				return VectorExpressionRepresentation(x.Args[0], declared)
			}
		case "coalesce", "ifnull":
			kind := ""
			for _, arg := range x.Args {
				k := VectorExpressionRepresentation(arg, declared)
				if k != "" {
					if kind != "" && kind != k {
						return "mixed"
					}
					kind = k
				}
			}
			return kind
		}
	}
	return ""
}

// vectorExpressionFunction retains column type evidence even when an ordinary
// external SQLite writer stored valid half TEXT. Read results still expose the
// actual storage class. Literal-only numeric calls keep existing dense behavior.
func vectorExpressionFunction(ctx *EvalContext, name string, exprs []Expr, args []any) (any, error) {
	if err := ValidateFunction(name, len(args)); err != nil {
		return nil, err
	}
	if ctx.Context != nil {
		if err := ctx.Context.Err(); err != nil {
			return nil, err
		}
	}
	for _, arg := range args {
		if arg == nil {
			return vectorFunction(ctx.Context, name, args)
		}
	}
	// Explicit constructors and converters keep closed source signatures.
	expected := ""
	switch name {
	case "halfvec", "halfvec_dims", "halfvec_norm", "halfvec_to_text", "vector_from_halfvec":
		expected = "halfvec"
	case "sparsevec", "sparsevec_dims", "sparsevec_nnz", "sparsevec_norm", "sparsevec_to_text", "vector_from_sparsevec":
		expected = "sparsevec"
	case "vector", "vector_dims", "vector_norm", "vector_to_text", "halfvec_from_vector", "sparsevec_from_vector", "halfvec_conversion_changed", "halfvec_conversion_max_abs", "halfvec_conversion_max_rel":
		expected = "vector"
	case "bit_vector", "bit_dims", "bit_to_text", "hamming_distance", "jaccard_distance":
		expected = "bit"
	}
	if expected != "" {
		for i, e := range exprs {
			if i > 0 && name != "hamming_distance" && name != "jaccard_distance" {
				break
			}
			kind := VectorExpressionRepresentation(e, ctx.DeclaredType)
			if kind != "" && kind != expected {
				return nil, fmt.Errorf("%w: %s requires %s representation", vector.ErrInvalid, name, expected)
			}
		}
	}
	if !halfOverload(name) {
		return vectorFunction(ctx.Context, name, args)
	}
	kind, err := numericBlobRepresentation(name, args)
	if err != nil {
		return nil, err
	}
	for i, e := range exprs {
		if i >= len(args) || name == "subvector" && i > 0 {
			break
		}
		k := VectorExpressionRepresentation(e, ctx.DeclaredType)
		if k == "mixed" || k != "" && kind != "" && kind != k {
			return nil, fmt.Errorf("%w: mixed vector expression representations", vector.ErrInvalid)
		}
		if k != "" {
			kind = k
		}
	}
	c := ctx.Context
	if c == nil {
		c = context.Background()
	}
	if kind == "sparsevec" {
		return sparseFunction(c, name, args)
	}
	if kind == "halfvec" {
		return halfFunction(c, name, args)
	}
	return vectorFunction(ctx.Context, name, args)
}

// Called after arity/context/NULL checks. Numeric math uses real binary16
// values and compensated float64 accumulation; no dense storage substitute.
func halfFunction(ctx context.Context, name string, args []any) (any, error) {
	if name == "halfvec_from_vector" || name == "halfvec_conversion_changed" || name == "halfvec_conversion_max_abs" || name == "halfvec_conversion_max_rel" {
		dense, err := VectorValue(args[0])
		if err != nil {
			return nil, err
		}
		h, report, err := vector.HalfFromVector(dense)
		if err != nil {
			return nil, err
		}
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		switch name {
		case "halfvec_conversion_changed":
			return int64(report.Changed), nil
		case "halfvec_conversion_max_abs":
			return report.MaxAbsoluteError, nil
		case "halfvec_conversion_max_rel":
			return report.MaxRelativeError, nil
		default:
			return h.MarshalBinary()
		}
	}
	a, err := HalfValue(args[0])
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	var result any
	switch name {
	case "halfvec":
		result, err = a.MarshalBinary()
	case "halfvec_dims":
		result = int64(a.Dims())
	case "halfvec_norm":
		result, err = vector.HalfNormContext(ctx, a)
	case "halfvec_to_text":
		result = a.String()
	case "vector_from_halfvec":
		var dense vector.Vector
		dense, err = a.Dense()
		if err == nil {
			result, err = dense.MarshalBinary()
		}
	case "binary_quantize":
		var b vector.BitVector
		b, err = vector.HalfBinaryQuantize(a)
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
		var h vector.HalfVector
		h, err = vector.HalfSubvector(a, start, count)
		if err == nil {
			result, err = h.MarshalBinary()
		}
	default:
		b, e := HalfValue(args[1])
		if e != nil {
			return nil, e
		}
		var metric vector.MetricKind
		switch name {
		case "l2_distance":
			metric = vector.L2
		case "l1_distance":
			metric = vector.L1
		case "cosine_distance":
			metric = vector.Cosine
		case "inner_product", "negative_inner_product":
			metric = vector.NegativeIP
		default:
			return nil, &ErrUnsupported{Feature: "halfvec function " + name}
		}
		result, err = metric.DistanceHalfContext(ctx, a, b)
		if err == nil && name == "inner_product" {
			result = -result.(float64)
		}
	}
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
