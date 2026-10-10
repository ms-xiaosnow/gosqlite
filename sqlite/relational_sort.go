package sqlite

import sqlast "github.com/ms-xiaosnow/gosqlite/internal/sql"

// Unlike the legacy sort.Interface callback, this merge loop can stop exactly
// at cancellation/work exhaustion and never continues sorting a partial result.
func (r *Rows) sortRelational() error {
	n := len(r.sorted)
	if n < 2 {
		return nil
	}
	bytes := int64(n) * 64
	if err := r.relational.reserve(bytes); err != nil {
		return err
	}
	defer r.relational.release(bytes)
	scratch := make([]resultRow, n)
	src, dst := r.sorted, scratch
	for width := 1; width < n; width *= 2 {
		for lo := 0; lo < n; lo += 2 * width {
			mid, hi := min(lo+width, n), min(lo+2*width, n)
			a, b := lo, mid
			for k := lo; k < hi; k++ {
				if err := r.relational.step(1); err != nil {
					return err
				}
				chooseLeft := b >= hi
				if a < mid && b < hi {
					cmp, err := r.compareRelationalRows(src[a], src[b])
					if err != nil {
						return err
					}
					chooseLeft = cmp <= 0
				}
				if a < mid && chooseLeft {
					dst[k] = src[a]
					a++
				} else {
					dst[k] = src[b]
					b++
				}
			}
		}
		src, dst = dst, src
	}
	r.sorted = src
	return nil
}
func (r *Rows) compareRelationalRows(a, b resultRow) (int, error) {
	for i, o := range r.order {
		if err := r.relational.step(1); err != nil {
			return 0, err
		}
		x, y := a.keys[i], b.keys[i]
		if o.nulls != sqlast.NullsDefault && (x == nil) != (y == nil) {
			if (x == nil) == (o.nulls == sqlast.NullsFirst) {
				return -1, nil
			}
			return 1, nil
		}
		if err := r.relational.step(valuesSize([]any{x, y})); err != nil {
			return 0, err
		}
		cmp, err := sqlast.Compare(x, y)
		if err != nil {
			return 0, err
		}
		if cmp != 0 {
			if o.desc {
				cmp = -cmp
			}
			return cmp, nil
		}
	}
	return 0, nil
}
