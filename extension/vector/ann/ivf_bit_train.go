package ann

import (
	"context"
	"fmt"
	"math/bits"
	"sort"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

type bitTrainingPoint struct {
	rowID  int64
	packed []byte
}

// bitDistance works on already validated, equal-width, canonical packed values.
func (t typedTrainingRun) bitDistance(a, b []byte) (float64, error) {
	if err := t.charge(int64(len(a))); err != nil {
		return 0, err
	}
	t.stats.DistanceEvaluations++
	var result int
	for i, x := range a {
		if i&255 == 0 {
			if err := t.ctx.Err(); err != nil {
				return 0, err
			}
		}
		result += bits.OnesCount8(x ^ b[i])
	}
	return float64(result), nil
}

// trainBitIVF minimizes Hamming assignment cost with genuine per-bit majority
// updates. Tied bits retain their previous value, and empty lists retain their
// prior centroid. Seeds use the same rowid-stable farthest-first policy as dense.
// Original packed samples never change and no dense numeric expansion is made.
func trainBitIVF(ctx context.Context, samples []TypedSample, cfg TypedConfig) (centers []TypedCentroid, stats TrainingStats, ready bool, err error) {
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
	if cfg.Kind != BitKind {
		err = ErrConfig
		return
	}
	stats.Samples = len(samples)
	if len(samples) > cfg.TrainingSamples || int64(len(samples)) > cfg.MaxNodes {
		err = fmt.Errorf("%w: training sample count", ErrLimit)
		return
	}
	light := int64(1024) + int64(len(samples))*(typedValueStorageBytes(cfg)+384)
	if light > cfg.MaxMemoryBytes {
		err = fmt.Errorf("%w: training input memory", ErrLimit)
		return
	}
	stats.PeakBytes = light
	ordered := append([]TypedSample(nil), samples...)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].RowID < ordered[j].RowID })
	for i, s := range ordered {
		if i > 0 && s.RowID == ordered[i-1].RowID {
			err = vector.ErrDuplicateRowID
			return
		}
		if err = ValidateValueContext(ctx, SampleValue(s), cfg); err != nil {
			return
		}
	}
	stats.TrainableSamples = len(samples)
	if len(samples) < cfg.Lists {
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
	run := typedTrainingRun{ctx, cfg, &stats}
	width := (cfg.Dimensions + 7) / 8
	points := make([]bitTrainingPoint, len(samples))
	first := 0
	for i, s := range ordered {
		if err = run.charge(int64(width)); err != nil {
			return
		}
		points[i] = bitTrainingPoint{s.RowID, SampleValue(s).Bit().Packed()}
		a, b := SamplePriority(cfg.Seed, s.RowID), SamplePriority(cfg.Seed, ordered[first].RowID)
		if a < b || (a == b && s.RowID < ordered[first].RowID) {
			first = i
		}
	}
	current := make([][]byte, 0, cfg.Lists)
	nearest := make([]float64, len(points))
	selected := make([]bool, len(points))
	for i := range nearest {
		nearest[i] = float64(cfg.Dimensions) + 1
	}
	chosen := first
	for len(current) < cfg.Lists {
		if err = run.charge(int64(width)); err != nil {
			return
		}
		current = append(current, append([]byte(nil), points[chosen].packed...))
		selected[chosen] = true
		if len(current) == cfg.Lists {
			break
		}
		farthest := -1
		for i, p := range points {
			if selected[i] {
				continue
			}
			var d float64
			d, err = run.bitDistance(p.packed, current[len(current)-1])
			if err != nil {
				return
			}
			if d < nearest[i] {
				nearest[i] = d
			}
			if farthest < 0 || nearest[i] > nearest[farthest] || (nearest[i] == nearest[farthest] && p.rowID < points[farthest].rowID) {
				farthest = i
			}
		}
		if farthest < 0 {
			err = ErrCorrupt
			return
		}
		chosen = farthest
	}
	assignment := make([]int, len(points))
	for i := range assignment {
		assignment[i] = -1
	}
	ones := make([]uint32, cfg.Lists*cfg.Dimensions)
	counts := make([]uint32, cfg.Lists)
	for iteration := 0; iteration < cfg.TrainingIterations; iteration++ {
		if err = ctx.Err(); err != nil {
			return
		}
		clear(ones)
		clear(counts)
		changes := 0
		for i, p := range points {
			best := 0
			var bestDistance float64
			for j, c := range current {
				var d float64
				d, err = run.bitDistance(p.packed, c)
				if err != nil {
					return
				}
				if j == 0 || d < bestDistance {
					best, bestDistance = j, d
				}
			}
			if assignment[i] != best {
				assignment[i] = best
				changes++
			}
			counts[best]++
			// Reduction visits bits (not only set bits), and charges their actual count.
			if err = run.charge(int64(cfg.Dimensions)); err != nil {
				return
			}
			offset := best * cfg.Dimensions
			for d := 0; d < cfg.Dimensions; d++ {
				if d&255 == 0 {
					if err = ctx.Err(); err != nil {
						return
					}
				}
				ones[offset+d] += uint32(p.packed[d/8] >> uint(7-d%8) & 1)
			}
		}
		stats.Iterations++
		if changes == 0 {
			stats.Converged = true
			break
		}
		for k, c := range current {
			if counts[k] == 0 {
				stats.EmptyClusters++
				continue
			}
			if err = run.charge(int64(cfg.Dimensions)); err != nil {
				return
			}
			for d := 0; d < cfg.Dimensions; d++ {
				if d&255 == 0 {
					if err = ctx.Err(); err != nil {
						return
					}
				}
				one := ones[k*cfg.Dimensions+d]
				mask := byte(1 << uint(7-d%8))
				if 2*one > counts[k] {
					c[d/8] |= mask
				} else if 2*one < counts[k] {
					c[d/8] &= ^mask
				}
			}
		}
	}
	centers = make([]TypedCentroid, cfg.Lists)
	for i, c := range current {
		if err = run.charge(int64(width)); err != nil {
			return
		}
		var v vector.BitVector
		v, err = vector.NewBit(cfg.Dimensions, c)
		if err != nil {
			return
		}
		centers[i] = TypedCentroid{ID: i + 1, Value: BitValue(v)}
		if err = ValidateTypedCentroid(centers[i], cfg); err != nil {
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
