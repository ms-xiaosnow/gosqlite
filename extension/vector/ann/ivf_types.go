package ann

import (
	"context"
	"fmt"
	"math"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

// Sample is one original immutable source vector. Training never changes it.
type Sample struct {
	RowID  int64
	Vector vector.Vector
}

// Centroid IDs are the consecutive integers 1..Config.Lists in slice order.
// Spherical centroids are unit length to float32 rounding (tolerance 1e-6).
type Centroid struct {
	ID     int
	Vector vector.Vector
}
type ProbeResult struct {
	ListID   int
	Distance float64
}

// TrainingStats contains actual attempted work, even when training fails.
// ComponentWork charges dimensions per vector stage in normalization,
// distances, centroid reduction and output conversion. Immutable-vector
// validation/copy bookkeeping is excluded; this is not an instruction count.
// PeakBytes is conservative retained logical memory, including input samples,
// not a process RSS/Go allocator bound. Sorting comparison work is separately
// bounded by TrainingSamples and Lists. Empty/ZeroSumClusters count occurrences.
type TrainingStats struct {
	Samples, TrainableSamples, SkippedZeros    int
	Iterations, EmptyClusters, ZeroSumClusters int
	DistanceEvaluations, ComponentWork         int64
	PeakBytes                                  int64
	Converged                                  bool
	StopReason                                 string
}

func ivfContext(ctx context.Context) error {
	if ctx == nil {
		return vector.ErrContext
	}
	return ctx.Err()
}
func validateIVF(c Config) error {
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
func Trainable(v vector.Vector, c Config) (bool, error) {
	if err := validateIVF(c); err != nil {
		return false, err
	}
	if err := ValidateVector(v, c); err != nil {
		return false, err
	}
	if c.Metric == vector.NegativeIP {
		n, err := vector.Norm(v)
		if err != nil {
			return false, err
		}
		return n != 0, nil
	}
	return true, nil
}

// SampleMemoryBytes reserves a retained original vector and conservative
// sampler/heap/sorted-input overhead, including spare backing-array capacity.
// c must have passed Validate before using the result for resource admission.
func SampleMemoryBytes(c Config) int64 { return int64(4*c.Dimensions + 128) }

// TrainingMemoryBytes includes caller-owned samples retained through Train,
// the sampler's allowance, copied float64 samples, assignments/seed work,
// two float64 centroid sets, compensated sums, output float32 centroids and
// transient vector conversion. Actual Go allocator metadata/RSS is excluded.
// All arithmetic is safe after the configuration and sample-count checks.
func TrainingMemoryBytes(sampleCount int, c Config) (int64, error) {
	if err := validateIVF(c); err != nil {
		return 0, err
	}
	if sampleCount < 0 || sampleCount > c.TrainingSamples {
		return 0, ErrConfig
	}
	d, n, k := int64(c.Dimensions), int64(sampleCount), int64(c.Lists)
	return 1024 + n*(12*d+256) + k*(36*d+192) + 32*d, nil
}

func ValidateCentroid(center Centroid, c Config) error {
	if err := validateIVF(c); err != nil {
		return err
	}
	if center.ID < 1 || center.ID > c.Lists {
		return ErrCorrupt
	}
	if err := ValidateVector(center.Vector, c); err != nil {
		return fmt.Errorf("%w: centroid: %w", ErrCorrupt, err)
	}
	if c.Metric != vector.L2 {
		norm, err := vector.Norm(center.Vector)
		if err != nil || math.Abs(norm-1) > 1e-6 {
			return fmt.Errorf("%w: non-unit centroid", ErrCorrupt)
		}
	}
	return nil
}

// CentroidMemoryBytes includes retained vectors and probe ordering storage.
func CentroidMemoryBytes(c Config) int64 { return int64(c.Lists)*int64(4*c.Dimensions+128) + 1024 }
