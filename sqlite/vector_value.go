package sqlite

import (
	"errors"
	"fmt"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// decodeVectorColumn is the single typed interpretation boundary for dense
// columns. A string is text, []byte is strictly the versioned binary format,
// and NULL is distinct from an invalid empty/zero Go Vector. Ordinary SQLite
// writers can store other values; those fail here rather than being guessed.
func decodeVectorColumn(value any, dimensions int) (vector.Vector, bool, error) {
	if value == nil {
		return vector.Vector{}, true, nil
	}
	v, err := sqlast.VectorValue(value)
	if err == nil {
		err = v.RequireDims(dimensions)
	}
	if err != nil {
		if errors.Is(err, vector.ErrLimit) {
			return vector.Vector{}, false, err
		}
		return vector.Vector{}, false, fmt.Errorf("%w: %w", ErrMismatch, err)
	}
	return v, false, nil
}

// columnValue validates supported vector types on interpretation, preserving the actual SQL
// storage class for reads. Writes canonicalize through storedColumnValue.
func columnValue(value any, declared string) (any, error) {
	if n, ok := sqlast.VectorDimensions(declared); ok {
		if _, _, err := decodeVectorColumn(value, n); err != nil {
			return nil, err
		}
	}
	if n, ok := sqlast.BitDimensions(declared); ok {
		if _, _, err := decodeBitColumn(value, n); err != nil {
			return nil, err
		}
	}
	return value, nil
}

func storedColumnValue(value any, declared string) (any, error) {
	if n, ok := sqlast.BitDimensions(declared); ok {
		v, null, err := decodeBitColumn(value, n)
		if err != nil || null {
			return nil, err
		}
		return v.MarshalBinary()
	}
	n, ok := sqlast.VectorDimensions(declared)
	if !ok {
		return sqlast.ApplyAffinity(value, declared)
	}
	v, null, err := decodeVectorColumn(value, n)
	if err != nil || null {
		return nil, err
	}
	return v.MarshalBinary()
}

// decodeBitColumn validates the known representation and declared width before
// returning a value. External numeric-affinity coercion is never reversed.
func decodeBitColumn(value any, dimensions int) (vector.BitVector, bool, error) {
	if value == nil {
		return vector.BitVector{}, true, nil
	}
	v, err := sqlast.BitValue(value)
	if err == nil {
		err = v.RequireDims(dimensions)
	}
	if err != nil {
		if errors.Is(err, vector.ErrLimit) {
			return vector.BitVector{}, false, err
		}
		return vector.BitVector{}, false, fmt.Errorf("%w: %w", ErrMismatch, err)
	}
	return v, false, nil
}

// missingColumnValue interprets an external ALTER-added column's default for
// an older short record. BIT has NUMERIC affinity in SQLite: a text default
// such as '001' is logically integer 1, not a stored GSB value. Preserve that
// value and let column interpretation reject it rather than invent lost width.
// Existing dense/default behavior is intentionally unchanged.
func missingColumnValue(value any, declared string) (any, error) {
	if _, bit := sqlast.BitDimensions(declared); bit {
		return sqlast.ApplyAffinity(value, declared)
	}
	return storedColumnValue(value, declared)
}
