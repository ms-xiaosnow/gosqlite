package vector

// subvectorBounds implements the pinned PostgreSQL-substring-like rule. End
// uses the original signed start; widening before addition avoids int32 wrap.
func subvectorBounds(dims int, start, count int32) (int, int, error) {
	if count < 1 {
		return 0, 0, ErrDimensions
	}
	end := int64(start) + int64(count)
	begin := int64(start)
	if begin < 1 {
		begin = 1
	}
	if end > int64(dims)+1 {
		end = int64(dims) + 1
	}
	if begin >= end {
		return 0, 0, ErrDimensions
	}
	return int(begin) - 1, int(end) - 1, nil
}

// Subvector returns a copied dense subvector using 1-based substring semantics.
// count must be positive; empty results error; oversize ends truncate. Negative
// start consumes part of count before the first component, rather than shifting
// the requested length. Inputs are int32 to match the pinned function domain.
func Subvector(v Vector, start, count int32) (Vector, error) {
	if !v.Valid() {
		return Vector{}, ErrInvalid
	}
	lo, hi, err := subvectorBounds(v.Dims(), start, count)
	if err != nil {
		return Vector{}, err
	}
	return Vector{values: append([]float32(nil), v.values[lo:hi]...)}, nil
}

// HalfSubvector preserves binary16 representation and copies private storage.
func HalfSubvector(v HalfVector, start, count int32) (HalfVector, error) {
	if !v.Valid() {
		return HalfVector{}, ErrInvalid
	}
	lo, hi, err := subvectorBounds(v.Dims(), start, count)
	if err != nil {
		return HalfVector{}, err
	}
	return HalfVector{bits: append([]uint16(nil), v.bits[lo:hi]...)}, nil
}
