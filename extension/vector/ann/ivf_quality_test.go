package ann

import (
	"context"
	"math"
	"math/rand/v2"
	"sort"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

func TestIVFLloydObjectiveImprovesAcrossRealIterations(t *testing.T) {
	rng := rand.New(rand.NewPCG(381, 72))
	samples := make([]Sample, 180)
	for i := range samples {
		values := make([]float32, 3)
		for d := range values {
			values[d] = float32(rng.NormFloat64() * 3)
		}
		samples[i] = Sample{int64(i) - 90, vec(t, values...)}
	}
	c := ivfConfig(t, 3, 7, vector.L2)
	previous := math.Inf(1)
	strictImprovement := false
	for iterations := 1; iterations <= 12; iterations++ {
		c.TrainingIterations = iterations
		centers := trainIVF(t, c, samples)
		objective := 0.0
		for _, s := range samples {
			best := math.Inf(1)
			for _, center := range centers {
				distance, _ := vector.L2Distance(s.Vector, center.Vector)
				best = math.Min(best, distance*distance)
			}
			objective += best
		}
		if objective > previous+1e-5 {
			t.Fatalf("Lloyd objective increased at iteration%d: %.12g > %.12g", iterations, objective, previous)
		}
		if iterations > 1 && objective < previous-1e-3 {
			strictImprovement = true
		}
		previous = objective
	}
	if !strictImprovement {
		t.Fatal("training never improved after its first mean update")
	}
}

func TestIVFSubsetListQualityAndOriginalRescoring(t *testing.T) {
	const count, dims, queries, lists, probes = 1024, 8, 24, 32, 8
	for _, metric := range []vector.MetricKind{vector.L2, vector.Cosine, vector.NegativeIP} {
		t.Run(string(metric), func(t *testing.T) {
			rng := rand.New(rand.NewPCG(20261010, 771))
			samples := make([]Sample, count)
			rows := make([]vector.Row, count)
			for i := range samples {
				values := make([]float32, dims)
				for d := range values {
					values[d] = float32(rng.NormFloat64())
				}
				v := vec(t, values...)
				samples[i] = Sample{int64(i) - count/2, v}
				rows[i] = vector.Row{RowID: samples[i].RowID, Vector: v}
			}
			c := ivfConfig(t, dims, lists, metric)
			centers, trainStats, ready, err := Train(context.Background(), samples, c)
			if err != nil || !ready {
				t.Fatal(trainStats, err)
			}
			members := make([][]vector.Row, lists)
			for _, row := range rows {
				id, _, err := Assign(context.Background(), row.Vector, centers, c, Limits{})
				if err != nil {
					t.Fatal(err)
				}
				members[id-1] = append(members[id-1], row)
			}
			var recall1, recall10, recall100, candidateWork int
			for range queries {
				values := make([]float32, dims)
				for d := range values {
					values[d] = float32(rng.NormFloat64())
				}
				query := vec(t, values...)
				order, stats, err := Probe(context.Background(), query, centers, c, Limits{})
				if err != nil || stats.DistanceEvaluations != lists {
					t.Fatal(stats, err)
				}
				var candidates []vector.Row
				for _, p := range order[:probes] {
					candidates = append(candidates, members[p.ListID-1]...)
				}
				candidateWork += len(candidates)
				approx, err := vector.TopK(context.Background(), query, candidates, vector.SearchOptions{Metric: metric, Limit: 100})
				if err != nil {
					t.Fatal(err)
				}
				exact, err := vector.TopK(context.Background(), query, rows, vector.SearchOptions{Metric: metric, Limit: 100})
				if err != nil {
					t.Fatal(err)
				}
				if len(approx.Hits) < 10 {
					t.Fatal("insufficient quality dataset candidates")
				}
				if approx.Hits[0].RowID == exact.Hits[0].RowID {
					recall1++
				}
				top10, top100 := map[int64]bool{}, map[int64]bool{}
				for i, h := range exact.Hits {
					top100[h.RowID] = true
					if i < 10 {
						top10[h.RowID] = true
					}
				}
				for i, h := range approx.Hits {
					if top100[h.RowID] {
						recall100++
					}
					if i < 10 && top10[h.RowID] {
						recall10++
					}
					expected, _ := metric.Distance(query, rows[int(h.RowID)+count/2].Vector)
					if h.Distance != expected {
						t.Fatal("candidate rescoring changed source magnitudes")
					}
				}
				// Opening all lists in this in-memory model equals the exact baseline;
				// the database integration must instead explain its all-list exact mode.
				var all []vector.Row
				for _, p := range order {
					all = append(all, members[p.ListID-1]...)
				}
				sort.Slice(all, func(i, j int) bool { return all[i].RowID < all[j].RowID })
				if len(all) != len(rows) {
					t.Fatal("membership omission")
				}
				for i := range rows {
					if all[i].RowID != rows[i].RowID {
						t.Fatal("membership duplicate or omission")
					}
				}
			}
			mean := float64(candidateWork) / queries
			r10 := float64(recall10) / (queries * 10)
			t.Logf("memory-only IVF quality: rows=%d dims=%d held_out=%d lists=%d probes=%d metric=%s recall@1=%.6f @10=%.6f @100=%.6f mean_original_candidate_scores=%.2f centroid_scores=%d exact_scores=%d training_iterations=%d component_work=%d peak_logical_bytes=%d", count, dims, queries, lists, probes, metric, float64(recall1)/queries, r10, float64(recall100)/(queries*100), mean, lists, count, trainStats.Iterations, trainStats.ComponentWork, trainStats.PeakBytes)
			if mean+lists >= count/2 {
				t.Fatal("list path did not materially reduce distance work")
			}
			if r10 < .75 {
				t.Fatalf("fixed fixture recall@10 %.6f below local regression target .75", r10)
			}
		})
	}
}
