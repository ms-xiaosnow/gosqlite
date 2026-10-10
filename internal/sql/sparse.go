package sql

import (
	"context"
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

// SparseDimensions recognizes only SPARSEVEC(n), n=1..1e9. Dimension is
// metadata, never an allocation size or a sparse-work charge.
func SparseDimensions(declared string) (int, bool) {
	if len(declared) > MaxSQLBytes {
		return 0, false
	}
	s := trimSpace(declared)
	if len(s) < len("SPARSEVEC(1)") || !EqualName(s[:9], "sparsevec") {
		return 0, false
	}
	s = trimSpace(s[9:])
	if len(s) < 3 || s[0] != '(' || s[len(s)-1] != ')' {
		return 0, false
	}
	return boundedDimensionNumber(trimSpace(s[1:len(s)-1]), vector.MaxSparseDimensions)
}

// SparseValueEntries preflights decoded support capacity without allocating.
// Text charges explicit entries, including zeros, conservatively; binary input
// has an exact nnz count. This is a resource bound, not codec validation.
func SparseValueEntries(value any) (int, error) {
	switch v := value.(type) {
	case string:
		if len(v) > vector.MaxTextBytes {
			return 0, vector.ErrLimit
		}
		n := strings.Count(v, ":")
		if n > vector.MaxSparseNonzeros {
			return 0, vector.ErrLimit
		}
		return n, nil
	case []byte:
		if len(v) < vector.SparseBinaryHeaderSize || len(v) > vector.MaxSparseBinaryBytes || string(v[:4]) != "GSS\x00" {
			return 0, vector.ErrEncoding
		}
		n := binary.BigEndian.Uint32(v[12:16])
		if n > vector.MaxSparseNonzeros || len(v) != vector.SparseBinaryHeaderSize+8*int(n) {
			return 0, vector.ErrEncoding
		}
		return int(n), nil
	default:
		return 0, fmt.Errorf("%w: SQL sparsevec requires text or a versioned GSS BLOB, got %T", vector.ErrInvalid, value)
	}
}

func SparseValue(value any) (vector.SparseVector, error) {
	if _, err := SparseValueEntries(value); err != nil {
		return vector.SparseVector{}, err
	}
	switch v := value.(type) {
	case string:
		return vector.ParseSparse(v)
	case []byte:
		return vector.DecodeSparse(v)
	default:
		return vector.SparseVector{}, vector.ErrInvalid
	}
}
func SparseValueDims(value any, dimensions int) (vector.SparseVector, error) {
	if b, ok := value.([]byte); ok && len(b) >= vector.SparseBinaryHeaderSize && string(b[:4]) == "GSS\x00" && uint64(binary.BigEndian.Uint32(b[8:12])) != uint64(dimensions) {
		return vector.SparseVector{}, vector.ErrDimensions
	}
	v, err := SparseValue(value)
	if err == nil {
		err = v.RequireDims(dimensions)
	}
	return v, err
}
func isSparseFunction(name string) bool {
	switch name {
	case "sparsevec", "sparsevec_dims", "sparsevec_nnz", "sparsevec_norm", "sparsevec_to_text", "sparsevec_from_vector", "vector_from_sparsevec":
		return true
	}
	return false
}

// All work is bounded by support size. Expensive transforms are closed until
// they have explicit sparse algorithms and independently documented budgets.
func sparseFunction(ctx context.Context, name string, args []any) (any, error) {
	if name == "subvector" || name == "binary_quantize" {
		return nil, &ErrUnsupported{Feature: name + " for sparsevec"}
	}
	if name == "sparsevec_from_vector" {
		d, err := VectorValue(args[0])
		if err != nil {
			return nil, err
		}
		v, err := vector.SparseFromVector(d)
		if err != nil {
			return nil, err
		}
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		result, err := v.MarshalBinary()
		if err != nil {
			return nil, err
		}
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		return result, nil
	}
	a, err := SparseValue(args[0])
	if err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	var result any
	switch name {
	case "sparsevec":
		result, err = a.MarshalBinary()
	case "sparsevec_dims":
		result = int64(a.Dims())
	case "sparsevec_nnz":
		result = int64(a.NNZ())
	case "sparsevec_norm":
		result, err = vector.SparseNormContext(ctx, a)
	case "sparsevec_to_text":
		result = a.String()
	case "vector_from_sparsevec":
		var d vector.Vector
		d, err = a.Dense()
		if err == nil {
			result, err = d.MarshalBinary()
		}
	default:
		b, e := SparseValue(args[1])
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
			return nil, &ErrUnsupported{Feature: "sparsevec function " + name}
		}
		result, err = metric.DistanceSparseContext(ctx, a, b)
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

// Representation evidence is distinct from validation. Unknown headers never
// acquire a kind and must still fail the selected signature's strict decoder.
func vectorBlobRepresentation(value any) string {
	b, ok := value.([]byte)
	if !ok || len(b) < 4 {
		return ""
	}
	switch string(b[:4]) {
	case "GSV\x00":
		return "vector"
	case "GSH\x00":
		return "halfvec"
	case "GSS\x00":
		return "sparsevec"
	case "GSB\x00":
		return "bit"
	}
	return ""
}
func numericBlobRepresentation(name string, args []any) (string, error) {
	kind := ""
	for i, value := range args {
		if name == "subvector" && i > 0 {
			break
		}
		k := vectorBlobRepresentation(value)
		if k != "" {
			if kind != "" && kind != k {
				return "", fmt.Errorf("%w: mixed vector representations", vector.ErrInvalid)
			}
			kind = k
		}
	}
	return kind, nil
}
