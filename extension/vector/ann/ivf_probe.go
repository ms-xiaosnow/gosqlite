package ann

import (
	"context"
	"fmt"
	"sort"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

func scoreCentroids(ctx context.Context, v vector.Vector, centers []Centroid, cfg Config, limits Limits, stats *Stats, accept func(ProbeResult)) error {
	if err := ivfContext(ctx); err != nil {
		return err
	}
	if err := validateIVF(cfg); err != nil {
		return err
	}
	if err := ValidateVector(v, cfg); err != nil {
		return err
	}
	bounded, err := limits.normalized(cfg)
	if err != nil {
		return err
	}
	if len(centers) != cfg.Lists {
		return ErrCorrupt
	}
	memory := CentroidMemoryBytes(cfg)
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
		if err := ValidateCentroid(c, cfg); err != nil {
			return err
		}
		if stats.DistanceEvaluations >= bounded.MaxDistanceEvaluations {
			return fmt.Errorf("%w: centroid scores", ErrLimit)
		}
		stats.DistanceEvaluations++
		distance, err := cfg.Metric.Distance(v, c.Vector)
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
func Assign(ctx context.Context, v vector.Vector, centers []Centroid, cfg Config, limits Limits) (id int, stats Stats, err error) {
	var best ProbeResult
	err = scoreCentroids(ctx, v, centers, cfg, limits, &stats, func(c ProbeResult) {
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
func Probe(ctx context.Context, query vector.Vector, centers []Centroid, cfg Config, limits Limits) (result []ProbeResult, stats Stats, err error) {
	// Reserve after validation/admission in scoreCentroids, before first append.
	err = scoreCentroids(ctx, query, centers, cfg, limits, &stats, func(c ProbeResult) {
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
