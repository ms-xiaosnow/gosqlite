package vector

import (
	"context"
	"math/bits"
)

// BitMetricKind identifies exact packed-bit distance semantics. It takes only
// BitVector operands; it cannot be used as a dense-vector Metric.
type BitMetricKind string

const (
	Hamming BitMetricKind = "hamming"
	Jaccard BitMetricKind = "jaccard"
)

func (m BitMetricKind) Name() string { return string(m) }

func (m BitMetricKind) Distance(a, b BitVector) (float64, error) {
	return m.DistanceContext(context.Background(), a, b)
}

// DistanceContext returns an exact distance between valid equal-width values.
// Nil contexts fail with ErrContext. Cancellation is checked before work,
// between blocks of at most 256 packed bytes, and before returning the result.
// On error, no partial distance is returned.
func (m BitMetricKind) DistanceContext(ctx context.Context, a, b BitVector) (float64, error) {
	if ctx == nil {
		return 0, ErrContext
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if m != Hamming && m != Jaccard {
		return 0, ErrMetric
	}
	if !a.Valid() || !b.Valid() {
		return 0, ErrInvalid
	}
	if a.dims != b.dims {
		return 0, ErrDimensions
	}
	var difference, intersection, union int
	for start := 0; start < len(a.packed); start += 256 {
		if err := ctx.Err(); err != nil {
			return 0, err
		}
		end := min(start+256, len(a.packed))
		if m == Hamming {
			for i := start; i < end; i++ {
				difference += bits.OnesCount8(a.packed[i] ^ b.packed[i])
			}
		} else {
			for i := start; i < end; i++ {
				intersection += bits.OnesCount8(a.packed[i] & b.packed[i])
				union += bits.OnesCount8(a.packed[i] | b.packed[i])
			}
		}
	}
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	if m == Hamming {
		return float64(difference), nil
	}
	// Pinned pgvector semantics return one for zero intersection, including
	// all-zero/all-zero and empty/empty, rather than dividing zero by zero.
	if intersection == 0 {
		return 1, nil
	}
	return 1 - float64(intersection)/float64(union), nil
}

// HammingDistance returns popcount(a XOR b), the bit-only <~> distance.
func HammingDistance(a, b BitVector) (float64, error) { return Hamming.Distance(a, b) }

// JaccardDistance returns 1 - popcount(a AND b)/popcount(a OR b), or 1 when the
// intersection is zero, including all-zero and empty pairs, as in pgvector.
func JaccardDistance(a, b BitVector) (float64, error) { return Jaccard.Distance(a, b) }
