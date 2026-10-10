package sql

import (
	"context"
	"fmt"
	"math"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

// BitDimensions recognizes only BIT(n), n=1..64000. Native zero-width bit
// values remain usable by scalar functions, but cannot declare a typed column.
func BitDimensions(declared string) (int, bool) {
	if len(declared) > MaxSQLBytes {
		return 0, false
	}
	s := trimSpace(declared)
	if len(s) < len("BIT(1)") || !EqualName(s[:3], "bit") {
		return 0, false
	}
	s = trimSpace(s[3:])
	if len(s) < 3 || s[0] != '(' || s[len(s)-1] != ')' {
		return 0, false
	}
	return boundedDimensionNumber(trimSpace(s[1:len(s)-1]), vector.MaxBitDimensions)
}

// IsVectorDeclaration identifies supported typed vector columns. It deliberately
// does not broaden dense VectorDimensions or dense ANN eligibility.
func IsVectorDeclaration(declared string) bool {
	if _, ok := VectorDimensions(declared); ok {
		return true
	}
	if _, ok := HalfDimensions(declared); ok {
		return true
	}
	_, ok := BitDimensions(declared)
	return ok
}

// BitValue interprets a non-NULL value only as strict bit text or GSB v1 BLOB.
// It never guesses integer width, coerces dense vectors, or repairs padding.
func BitValue(value any) (vector.BitVector, error) {
	switch v := value.(type) {
	case string:
		return vector.ParseBit(v)
	case []byte:
		return vector.DecodeBit(v)
	default:
		return vector.BitVector{}, fmt.Errorf("%w: SQL bit vector requires text or a versioned GSB BLOB, got %T", vector.ErrInvalid, value)
	}
}

func isBitFunction(name string) bool {
	switch name {
	case "bit_vector", "bit_dims", "bit_to_text", "hamming_distance", "jaccard_distance":
		return true
	}
	return false
}

// Called only after arity, context and NULL propagation checks.
func bitFunction(ctx context.Context, name string, args []any) (any, error) {
	a, err := BitValue(args[0])
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	var result any
	switch name {
	case "bit_vector":
		result, err = a.MarshalBinary()
	case "bit_dims":
		result = int64(a.Dims())
	case "bit_to_text":
		result = a.String()
	default:
		b, e := BitValue(args[1])
		if e != nil {
			return nil, e
		}
		metric := vector.Hamming
		if name == "jaccard_distance" {
			metric = vector.Jaccard
		}
		result, err = metric.DistanceContext(ctx, a, b)
	}
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// The transform's index domain is deliberately strict: SQL INTEGER storage,
// representable as signed int32. No rounding, numeric text or REAL coercion.
func subvectorInteger(value any) (int32, error) {
	n, ok := value.(int64)
	if !ok || n < math.MinInt32 || n > math.MaxInt32 {
		return 0, fmt.Errorf("%w: subvector start/count require signed 32-bit SQL integers", vector.ErrInvalid)
	}
	return int32(n), nil
}
