package ann

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

type typedTrainingPoint struct {
	rowID  int64
	values []float64
}
type typedTrainingRun struct {
	ctx   context.Context
	cfg   TypedConfig
	stats *TrainingStats
}

func (t typedTrainingRun) charge(n int64) error {
	if err := t.ctx.Err(); err != nil {
		return err
	}
	if n < 0 || n > t.cfg.MaxTrainingWork-t.stats.ComponentWork {
		return fmt.Errorf("%w: training component work", ErrLimit)
	}
	t.stats.ComponentWork += n
	return nil
}

func (t typedTrainingRun) distance(a, b []float64) (float64, error) {
	if err := t.charge(int64(t.cfg.Dimensions)); err != nil {
		return 0, err
	}
	t.stats.DistanceEvaluations++
	var result compensated
	if t.cfg.Metric == vector.L2 {
		for i, x := range a {
			d := x - b[i]
			result.add(d * d)
		}
		return result.value(), nil
	}
	for i, x := range a {
		result.add(x * b[i])
	}
	return 1 - math.Max(-1, math.Min(1, result.value())), nil
}

// Train performs real deterministic Lloyd iterations. L2 minimizes squared
// Euclidean distance. IP/cosine use normalized float64 sample directions and
// spherical means; original inputs are never normalized or replaced. The first
// seed is the smallest seeded hash/rowid; later seeds are farthest from the
// existing set (rowid ties). Distinct sample identities are chosen even when
// their vectors coincide. Empty clusters and exact-zero spherical sums retain
// their previous centroid. No arbitrary vector is generated.
//
// Samples may be in any order and must have distinct rowids. Insufficient
// trainable input returns nil,stats,false,nil; the caller persists untrained
// state and must explicitly retrain. IP zeros do not count as trainable. Cosine
// zeros always fail. Convergence means assignments did not change after at
// least one mean update; otherwise exactly TrainingIterations updates occur.
// All failures return no centroids. Dense output is float32 (GSV-ready). Half
// output rounds only derived float32 centers to binary16 (GSH); source values
// are unchanged. Bit output follows packed majority Lloyd training separately.
func TrainValues(ctx context.Context, samples []TypedSample, cfg TypedConfig) (centers []TypedCentroid, stats TrainingStats, ready bool, err error) {
	defer func() {
		if err != nil {
			centers = nil
			ready = false
			stats.StopReason = ivfStopReason(err)
		}
	}()
	if err = ivfContext(ctx); err != nil {
		return
	}
	if err = validateTypedIVF(cfg); err != nil {
		return
	}
	if cfg.Kind == BitKind {
		return trainBitIVF(ctx, samples, cfg)
	}
	stats.Samples = len(samples)
	if len(samples) > cfg.TrainingSamples || int64(len(samples)) > cfg.MaxNodes {
		err = fmt.Errorf("%w: training sample count", ErrLimit)
		return
	}
	// Avoid reserving all lists for an empty/untrainable index.
	lightBytes := int64(1024) + int64(len(samples))*(typedValueStorageBytes(cfg)+384)
	if lightBytes > cfg.MaxMemoryBytes {
		err = fmt.Errorf("%w: training input memory", ErrLimit)
		return
	}
	stats.PeakBytes = lightBytes
	ordered := append([]TypedSample(nil), samples...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].RowID < ordered[j].RowID })
	if err = ctx.Err(); err != nil {
		return
	}
	run := typedTrainingRun{ctx, cfg, &stats}
	for i, s := range ordered {
		if i > 0 && s.RowID == ordered[i-1].RowID {
			err = vector.ErrDuplicateRowID
			return
		}
		if cfg.Metric != vector.L2 {
			if err = run.charge(int64(cfg.Dimensions)); err != nil {
				return
			}
		}
		var ok bool
		ok, err = trainableValueContext(ctx, SampleValue(s), cfg)
		if err != nil {
			return
		}
		if ok {
			stats.TrainableSamples++
		} else {
			stats.SkippedZeros++
		}
	}
	if err = ctx.Err(); err != nil {
		return
	}
	if stats.TrainableSamples < cfg.Lists {
		stats.StopReason = "insufficient_trainable_samples"
		return
	}
	var memory int64
	memory, err = TypedTrainingMemoryBytes(len(samples), cfg)
	if err != nil {
		return
	}
	if memory > cfg.MaxMemoryBytes {
		err = fmt.Errorf("%w: training retained memory", ErrLimit)
		return
	}
	stats.PeakBytes = memory
	points := make([]typedTrainingPoint, 0, stats.TrainableSamples)
	for _, s := range ordered {
		if err = run.charge(int64(cfg.Dimensions)); err != nil {
			return
		}
		original := SampleValue(s).Dense().Values()
		if cfg.Kind == HalfKind {
			original = SampleValue(s).Half().Values()
		}
		values := make([]float64, cfg.Dimensions)
		var norm compensated
		for i, x := range original {
			values[i] = float64(x)
			if cfg.Metric != vector.L2 {
				norm.add(float64(x) * float64(x))
			}
		}
		if cfg.Metric != vector.L2 {
			if err = run.charge(int64(cfg.Dimensions)); err != nil {
				return
			}
			n := math.Sqrt(norm.value())
			if n == 0 {
				continue
			}
			for i := range values {
				values[i] /= n
			}
		}
		points = append(points, typedTrainingPoint{s.RowID, values})
	}
	var current [][]float64
	current, err = run.initialize(points)
	if err != nil {
		return
	}
	assignment := make([]int, len(points))
	for i := range assignment {
		assignment[i] = -1
	}
	sums := make([]compensated, cfg.Lists*cfg.Dimensions)
	counts := make([]int, cfg.Lists)
	for iteration := 0; iteration < cfg.TrainingIterations; iteration++ {
		changes := 0
		clear(sums)
		clear(counts)
		for i, p := range points {
			best := 0
			var bestDistance float64
			for j, c := range current {
				var distance float64
				distance, err = run.distance(p.values, c)
				if err != nil {
					return
				}
				if j == 0 || distance < bestDistance {
					best = j
					bestDistance = distance
				}
			}
			if assignment[i] != best {
				changes++
				assignment[i] = best
			}
			counts[best]++
			if err = run.charge(int64(cfg.Dimensions)); err != nil {
				return
			}
			offset := best * cfg.Dimensions
			for d, x := range p.values {
				sums[offset+d].add(x)
			}
		}
		stats.Iterations++
		if changes == 0 {
			stats.Converged = true
			break
		}
		for k := range current {
			if counts[k] == 0 {
				stats.EmptyClusters++
				continue
			}
			if err = run.charge(int64(cfg.Dimensions)); err != nil {
				return
			}
			values := make([]float64, cfg.Dimensions)
			var squaredNorm compensated
			for d := range values {
				values[d] = sums[k*cfg.Dimensions+d].value() / float64(counts[k])
				if cfg.Metric != vector.L2 {
					squaredNorm.add(values[d] * values[d])
				}
			}
			if cfg.Metric != vector.L2 {
				if squaredNorm.value() == 0 {
					stats.ZeroSumClusters++
					continue
				}
				if err = run.charge(int64(cfg.Dimensions)); err != nil {
					return
				}
				n := math.Sqrt(squaredNorm.value())
				for d := range values {
					values[d] /= n
				}
			}
			current[k] = values
		}
	}
	centers = make([]TypedCentroid, cfg.Lists)
	for k, values := range current {
		if err = run.charge(int64(cfg.Dimensions)); err != nil {
			return
		}
		converted := make([]float32, cfg.Dimensions)
		for i, x := range values {
			converted[i] = float32(x)
		}
		var v vector.Vector
		v, err = vector.New(converted)
		if err != nil {
			return
		}
		centers[k] = TypedCentroid{ID: k + 1, Value: DenseValue(v)}
		if cfg.Kind == HalfKind {
			var half vector.HalfVector
			half, err = vector.NewHalf(converted)
			if err != nil {
				return
			}
			centers[k] = TypedCentroid{ID: k + 1, Value: HalfValue(half)}
		}
		if err = ValidateTypedCentroid(centers[k], cfg); err != nil {
			return
		}
	}
	if err = ctx.Err(); err != nil {
		return
	}
	ready = true
	if stats.Converged {
		stats.StopReason = "converged"
	} else {
		stats.StopReason = "iteration_limit"
	}
	return
}

func (t typedTrainingRun) initialize(points []typedTrainingPoint) ([][]float64, error) {
	first := 0
	for i := 1; i < len(points); i++ {
		if i&255 == 0 {
			if err := t.ctx.Err(); err != nil {
				return nil, err
			}
		}
		a, b := SamplePriority(t.cfg.Seed, points[i].rowID), SamplePriority(t.cfg.Seed, points[first].rowID)
		if a < b || (a == b && points[i].rowID < points[first].rowID) {
			first = i
		}
	}
	centers := make([][]float64, 0, t.cfg.Lists)
	nearest := make([]float64, len(points))
	selected := make([]bool, len(points))
	for i := range nearest {
		nearest[i] = math.Inf(1)
	}
	chosen := first
	for len(centers) < t.cfg.Lists {
		if err := t.charge(int64(t.cfg.Dimensions)); err != nil {
			return nil, err
		}
		centers = append(centers, append([]float64(nil), points[chosen].values...))
		selected[chosen] = true
		if len(centers) == t.cfg.Lists {
			break
		}
		farthest := -1
		for i, p := range points {
			if selected[i] {
				continue
			}
			distance, err := t.distance(p.values, centers[len(centers)-1])
			if err != nil {
				return nil, err
			}
			if distance < nearest[i] {
				nearest[i] = distance
			}
			if farthest < 0 || nearest[i] > nearest[farthest] || (nearest[i] == nearest[farthest] && p.rowID < points[farthest].rowID) {
				farthest = i
			}
		}
		if farthest < 0 {
			return nil, ErrCorrupt
		}
		chosen = farthest
	}
	return centers, nil
}
