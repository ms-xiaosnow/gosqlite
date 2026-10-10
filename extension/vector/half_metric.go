package vector

import (
	"context"
	"math"
)

// DistanceHalf evaluates only matching half values using the same four closed
// metric identities as dense vectors. No lossy cross-kind coercion occurs.
func (m MetricKind) DistanceHalf(a, b HalfVector) (float64, error) {
	return m.DistanceHalfContext(context.Background(), a, b)
}

// DistanceHalfContext performs bounded O(dimensions) work, no allocations, and
// checks cancellation at most every 256 components. Arithmetic widens to
// float64 and uses compensation, intentionally differing from upstream float32.
func (m MetricKind) DistanceHalfContext(ctx context.Context, a, b HalfVector) (float64, error) {
	if ctx == nil {
		return 0, ErrContext
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if !a.Valid() || !b.Valid() {
		return 0, ErrInvalid
	}
	if a.Dims() != b.Dims() {
		return 0, ErrDimensions
	}
	switch m {
	case L2, L1, NegativeIP, Cosine:
	default:
		return 0, ErrMetric
	}
	var sum, aa, bb accumulator
	for i, h := range a.bits {
		if i&255 == 0 {
			if err := ctx.Err(); err != nil {
				return 0, err
			}
		}
		x, y := float64(halfFloat32(h)), float64(halfFloat32(b.bits[i]))
		switch m {
		case L2:
			d := x - y
			sum.add(d * d)
		case L1:
			sum.add(math.Abs(x - y))
		case NegativeIP:
			sum.add(x * y)
		case Cosine:
			sum.add(x * y)
			aa.add(x * x)
			bb.add(y * y)
		}
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	switch m {
	case L2:
		return math.Sqrt(sum.value()), nil
	case L1:
		return sum.value(), nil
	case NegativeIP:
		return -sum.value(), nil
	default:
		if aa.value() == 0 || bb.value() == 0 {
			return 0, ErrZeroNorm
		}
		s := sum.value() / math.Sqrt(aa.value()*bb.value())
		return 1 - math.Max(-1, math.Min(1, s)), nil
	}
}

func HalfL2Distance(a, b HalfVector) (float64, error)           { return L2.DistanceHalf(a, b) }
func HalfL1Distance(a, b HalfVector) (float64, error)           { return L1.DistanceHalf(a, b) }
func HalfNegativeInnerProduct(a, b HalfVector) (float64, error) { return NegativeIP.DistanceHalf(a, b) }
func HalfCosineDistance(a, b HalfVector) (float64, error)       { return Cosine.DistanceHalf(a, b) }
func HalfInnerProduct(a, b HalfVector) (float64, error) {
	x, err := NegativeIP.DistanceHalf(a, b)
	if err != nil {
		return 0, err
	}
	return -x, nil
}
func HalfNorm(a HalfVector) (float64, error) { return HalfNormContext(context.Background(), a) }
func HalfNormContext(ctx context.Context, a HalfVector) (float64, error) {
	if ctx == nil {
		return 0, ErrContext
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if !a.Valid() {
		return 0, ErrInvalid
	}
	var sum accumulator
	for i, h := range a.bits {
		if i&255 == 0 {
			if err := ctx.Err(); err != nil {
				return 0, err
			}
		}
		x := float64(halfFloat32(h))
		sum.add(x * x)
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	return math.Sqrt(sum.value()), nil
}
