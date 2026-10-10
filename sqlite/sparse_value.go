package sqlite

import (
	"errors"
	"fmt"
	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"
)

func decodeSparseColumn(value any, dimensions int) (vector.SparseVector, bool, error) {
	if value == nil {
		return vector.SparseVector{}, true, nil
	}
	v, err := sqlast.SparseValueDims(value, dimensions)
	if err != nil {
		if errors.Is(err, vector.ErrLimit) {
			return vector.SparseVector{}, false, err
		}
		return vector.SparseVector{}, false, fmt.Errorf("%w: %w", ErrMismatch, err)
	}
	return v, false, nil
}

func sparseValueEntries(value any) (int, error) {
	n, err := sqlast.SparseValueEntries(value)
	if err != nil && !errors.Is(err, vector.ErrLimit) {
		err = fmt.Errorf("%w: %w", ErrMismatch, err)
	}
	return n, err
}

// Conservative decoded query/candidate scratch; dimensions never participate.
// Input text/BLOB ownership, arbitrary SQL, and Valuer callbacks are external
// to this logical retained-memory budget.
func sparseScratchBytes(queryEntries, candidateEntries int) int64 {
	return 128 + 32*(int64(queryEntries)+int64(candidateEntries))
}
