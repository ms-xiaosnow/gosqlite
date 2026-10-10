package sqlite

import (
	"errors"
	"fmt"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

// decodeHalfColumn is the typed HALFVEC boundary. Valid external TEXT remains
// TEXT on reads; native writes always produce canonical GSH binary16 bytes.
func decodeHalfColumn(value any, dimensions int) (vector.HalfVector, bool, error) {
	if value == nil {
		return vector.HalfVector{}, true, nil
	}
	v, err := sqlast.HalfValueDims(value, dimensions)
	if err == nil {
		err = v.RequireDims(dimensions)
	}
	if err != nil {
		if errors.Is(err, vector.ErrLimit) {
			return vector.HalfVector{}, false, err
		}
		return vector.HalfVector{}, false, fmt.Errorf("%w: %w", ErrMismatch, err)
	}
	return v, false, nil
}

func declaredTypeFor(tab *table) func(string, string) string {
	return func(q, name string) string {
		if tab == nil {
			return ""
		}
		i, err := tab.column(q, name)
		if err != nil {
			return ""
		}
		if i < 0 {
			return "INTEGER"
		}
		return tab.columns[i].Type
	}
}
