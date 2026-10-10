package vector

// BinaryQuantize maps each dense component to one bit, set iff it is strictly
// greater than zero. +/-0 and negative values become 0. Output width is exactly
// the input dimension. This lossy transform does not preserve distance/recall;
// callers should retain original vectors for final reranking.
func BinaryQuantize(v Vector) (BitVector, error) {
	if !v.Valid() {
		return BitVector{}, ErrInvalid
	}
	b := make([]byte, (v.Dims()+7)/8)
	for i, x := range v.values {
		if x > 0 {
			b[i/8] |= 0x80 >> uint(i%8)
		}
	}
	return NewBit(v.Dims(), b)
}

// HalfBinaryQuantize preserves half-vector width and uses the same strict >0
// rule. A positive source rounded down to half zero consequently produces 0.
func HalfBinaryQuantize(v HalfVector) (BitVector, error) {
	if !v.Valid() {
		return BitVector{}, ErrInvalid
	}
	b := make([]byte, (v.Dims()+7)/8)
	for i, h := range v.bits {
		if h&0x8000 == 0 && h&0x7fff != 0 {
			b[i/8] |= 0x80 >> uint(i%8)
		}
	}
	return NewBit(v.Dims(), b)
}
