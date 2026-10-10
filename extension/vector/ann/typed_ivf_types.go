package ann

import (
	"context"
	"fmt"
	"math"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

// TypedSample is one original immutable source vector. Training never changes it.
type TypedSample struct {
	RowID int64
	Value Value
}

// TypedCentroid IDs are the consecutive integers 1..TypedConfig.Lists in slice order.
// Numeric spherical centroids are unit length to output rounding (1e-6 for
// dense float32, 1e-3 for half). Bit centroids are packed majority values.
type TypedCentroid struct {
	ID    int
	Value Value
}

func validateTypedIVF(c TypedConfig) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if c.Method != IVFFlat {
		return ErrConfig
	}
	return nil
}

// Trainable rejects every invalid indexed value, including cosine zeros.
// IP zeros remain indexable, but cannot define a spherical training direction.
func TrainableValue(v Value, c TypedConfig) (bool, error) {
	return trainableValueContext(context.Background(), v, c)
}
func trainableValueContext(ctx context.Context, v Value, c TypedConfig) (bool, error) {
	if err := validateTypedIVF(c); err != nil {
		return false, err
	}
	if err := ValidateValueContext(ctx, v, c); err != nil {
		return false, err
	}
	if c.Metric == vector.NegativeIP {
		n, err := valueNorm(ctx, v)
		if err != nil {
			return false, err
		}
		return n != 0, nil
	}
	return true, nil
}

// TypedSampleMemoryBytes reserves a retained original vector and conservative
// sampler/heap/sorted-input overhead, including spare backing-array capacity.
// c must have passed Validate before using the result for resource admission.
func TypedSampleMemoryBytes(c TypedConfig) int64 { return typedValueStorageBytes(c) + 384 }
func typedValueStorageBytes(c TypedConfig) int64 {
	switch c.Kind {
	case HalfKind:
		return 2 * int64(c.Dimensions)
	case BitKind:
		return int64((c.Dimensions + 7) / 8)
	case SparseKind:
		return 8 * MaxSparseANNNonzeros
	default:
		return 4 * int64(c.Dimensions)
	}
}

// TypedTrainingMemoryBytes includes caller-owned samples retained through Train,
// the sampler's allowance, copied float64 samples, assignments/seed work,
// two float64 centroid sets, compensated sums, output centroids and transient
// conversion. Bit training instead reserves packed copies/centers and uint32
// per-list bit counts; fixed allowances include the inline Value union. Actual Go allocator metadata/RSS is excluded.
// All arithmetic is safe after the configuration and sample-count checks.
func TypedTrainingMemoryBytes(sampleCount int, c TypedConfig) (int64, error) {
	if err := validateTypedIVF(c); err != nil {
		return 0, err
	}
	if sampleCount < 0 || sampleCount > c.TrainingSamples {
		return 0, ErrConfig
	}
	d, n, k := int64(c.Dimensions), int64(sampleCount), int64(c.Lists)
	if c.Kind == BitKind {
		b := (d + 7) / 8
		return 1024 + n*(2*b+512) + k*(4*d+4*b+384) + 2*b, nil
	}
	return 1024 + n*(8*d+typedValueStorageBytes(c)+512) + k*(36*d+384) + 32*d, nil
}

func ValidateTypedCentroid(center TypedCentroid, c TypedConfig) error {
	if err := validateTypedIVF(c); err != nil {
		return err
	}
	if center.ID < 1 || center.ID > c.Lists {
		return ErrCorrupt
	}
	if err := ValidateValue(CentroidValue(center), c); err != nil {
		return fmt.Errorf("%w: centroid: %w", ErrCorrupt, err)
	}
	if c.Kind != BitKind && c.Metric != vector.L2 {
		norm, err := valueNorm(context.Background(), CentroidValue(center))
		tolerance := 1e-6
		if c.Kind == HalfKind {
			tolerance = 1e-3
		}
		if err != nil || math.Abs(norm-1) > tolerance {
			return fmt.Errorf("%w: non-unit centroid", ErrCorrupt)
		}
	}
	return nil
}

// TypedCentroidMemoryBytes includes retained vectors and probe ordering storage.
func TypedCentroidMemoryBytes(c TypedConfig) int64 {
	return int64(c.Lists)*(typedValueStorageBytes(c)+256) + 1024
}

func SampleValue(s TypedSample) Value              { return s.Value }
func CentroidValue(c TypedCentroid) Value          { return c.Value }
func newTypedSample(id int64, v Value) TypedSample { return TypedSample{RowID: id, Value: v} }
