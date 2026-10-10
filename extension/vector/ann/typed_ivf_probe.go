package ann

import (
	"context"
	"fmt"
	"sort"
)

func scoreTypedCentroids(ctx context.Context, v Value, centers []TypedCentroid, cfg TypedConfig, limits TypedLimits, stats *TypedStats, accept func(ProbeResult)) error {
	if err := ivfContext(ctx); err != nil {
		return err
	}
	if err := validateTypedIVF(cfg); err != nil {
		return err
	}
	if err := ValidateValueContext(ctx, v, cfg); err != nil {
		return err
	}
	bounded, err := limits.normalized(cfg)
	if err != nil {
		return err
	}
	if len(centers) != cfg.Lists {
		return ErrCorrupt
	}
	memory := TypedCentroidMemoryBytes(cfg)
	if memory > bounded.MaxBytes {
		return fmt.Errorf("%w: centroid memory", ErrLimit)
	}
	stats.PeakBytes = memory
	for i, c := range centers {
		if err := ctx.Err(); err != nil {
			return err
		}
		if stats.Visited >= bounded.MaxVisited {
			return fmt.Errorf("%w: centroid visits", ErrLimit)
		}
		stats.Visited++
		if c.ID != i+1 {
			return ErrCorrupt
		}
		if err := ValidateTypedCentroid(c, cfg); err != nil {
			return err
		}
		if stats.DistanceEvaluations >= bounded.MaxDistanceEvaluations {
			return fmt.Errorf("%w: centroid scores", ErrLimit)
		}
		work := DistanceWork(v, CentroidValue(c))
		if (bounded.MaxComponentWork > 0 || bounded.ComponentWorkLimited) && work > bounded.MaxComponentWork-uint64(stats.ComponentWork) {
			return fmt.Errorf("%w: centroid component work", ErrLimit)
		}
		stats.ComponentWork += int64(work)
		stats.DistanceEvaluations++
		distance, err := DistanceContext(ctx, cfg, v, CentroidValue(c))
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		accept(ProbeResult{c.ID, distance})
	}
	stats.Exhausted = true
	stats.StopReason = "centroids_scored"
	return nil
}

// Assign selects the exact nearest persisted centroid by the index metric,
// then list ID. IP source vectors stay original; a zero IP vector ties all
// centroids and is assigned to list1. No member traversal occurs here.
func AssignValue(ctx context.Context, v Value, centers []TypedCentroid, cfg TypedConfig, limits TypedLimits) (id int, stats TypedStats, err error) {
	var best ProbeResult
	err = scoreTypedCentroids(ctx, v, centers, cfg, limits, &stats, func(c ProbeResult) {
		if best.ListID == 0 || c.Distance < best.Distance || (c.Distance == best.Distance && c.ListID < best.ListID) {
			best = c
		}
	})
	if err != nil {
		stats.StopReason = ivfStopReason(err)
		return 0, stats, err
	}
	return best.ListID, stats, nil
}

// Probe orders the bounded centroid set by the exact built-in metric on the
// original query, then list ID. SQLite opens only selected member ranges.
// These exact centroid scores are not an exact-nearest-neighbor recall claim.
func ProbeValue(ctx context.Context, query Value, centers []TypedCentroid, cfg TypedConfig, limits TypedLimits) (result []ProbeResult, stats TypedStats, err error) {
	// Reserve after validation/admission in scoreTypedCentroids, before first append.
	err = scoreTypedCentroids(ctx, query, centers, cfg, limits, &stats, func(c ProbeResult) {
		if result == nil {
			result = make([]ProbeResult, 0, cfg.Lists)
		}
		result = append(result, c)
	})
	if err != nil {
		stats.StopReason = ivfStopReason(err)
		return nil, stats, err
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Distance != result[j].Distance {
			return result[i].Distance < result[j].Distance
		}
		return result[i].ListID < result[j].ListID
	})
	if err = ctx.Err(); err != nil {
		stats.StopReason = ivfStopReason(err)
		return nil, stats, err
	}
	return
}
