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

// columnValue validates VECTOR(n) on interpretation, preserving the actual SQL
// storage class for reads. Writes canonicalize through storedColumnValue.
func columnValue(value any, declared string) (any, error) {
	if n, ok := sqlast.VectorDimensions(declared); ok {
		if _, _, err := decodeVectorColumn(value, n); err != nil {
			return nil, err
		}
	}
	return value, nil
}

func storedColumnValue(value any, declared string) (any, error) {
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
