package vector

import (
	"context"
	"math"
)

func (m MetricKind) DistanceSparse(a, b SparseVector) (float64, error) {
	return m.DistanceSparseContext(context.Background(), a, b)
}

// DistanceSparseContext merges sorted supports in O(nnz(a)+nnz(b)) time and
// O(1) space, independent of dimension. Cancellation is checked at most every
// 256 visited support positions. Numeric/zero-cosine policy matches dense math.
func (m MetricKind) DistanceSparseContext(ctx context.Context, a, b SparseVector) (float64, error) {
	if ctx == nil {
		return 0, ErrContext
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if !a.Valid() || !b.Valid() {
		return 0, ErrInvalid
	}
	if a.dim != b.dim {
		return 0, ErrDimensions
	}
	switch m {
	case L2, L1, NegativeIP, Cosine:
	default:
		return 0, ErrMetric
	}
	var sum, aa, bb accumulator
	i, j, visited := 0, 0, 0
	for i < len(a.entries) || j < len(b.entries) {
		if visited&255 == 0 {
			if err := ctx.Err(); err != nil {
				return 0, err
			}
		}
		visited++
		var x, y float64
		if j == len(b.entries) || (i < len(a.entries) && a.entries[i].Index < b.entries[j].Index) {
			x = float64(a.entries[i].Value)
			i++
		} else if i == len(a.entries) || b.entries[j].Index < a.entries[i].Index {
			y = float64(b.entries[j].Value)
			j++
		} else {
			x, y = float64(a.entries[i].Value), float64(b.entries[j].Value)
			i++
			j++
		}
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

func SparseL2Distance(a, b SparseVector) (float64, error) { return L2.DistanceSparse(a, b) }
func SparseL1Distance(a, b SparseVector) (float64, error) { return L1.DistanceSparse(a, b) }
func SparseNegativeInnerProduct(a, b SparseVector) (float64, error) {
	return NegativeIP.DistanceSparse(a, b)
}
func SparseCosineDistance(a, b SparseVector) (float64, error) { return Cosine.DistanceSparse(a, b) }
func SparseInnerProduct(a, b SparseVector) (float64, error) {
	x, err := NegativeIP.DistanceSparse(a, b)
	if err != nil {
		return 0, err
	}
	return -x, nil
}
func SparseNorm(a SparseVector) (float64, error) { return SparseNormContext(context.Background(), a) }
func SparseNormContext(ctx context.Context, a SparseVector) (float64, error) {
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
	for i, e := range a.entries {
		if i&255 == 0 {
			if err := ctx.Err(); err != nil {
				return 0, err
			}
		}
		x := float64(e.Value)
		sum.add(x * x)
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	return math.Sqrt(sum.value()), nil
}
