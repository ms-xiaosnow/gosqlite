package ann

import (
	"context"
	"errors"
	"math"
	"math/rand/v2"
	"reflect"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

func TestIVFReviewSphericalObjectiveAndExactQuotaBoundaries(t *testing.T) {
	ctx := context.Background()
	for _, metric := range []vector.MetricKind{vector.L2, vector.Cosine, vector.NegativeIP} {
		t.Run(string(metric), func(t *testing.T) {
			rng := rand.New(rand.NewPCG(9137, 288))
			var samples []Sample
			for i := 0; i < 67; i++ {
				values := make([]float32, 5)
				for j := range values {
					values[j] = float32(rng.NormFloat64() * float64(i%7+1))
				}
				samples = append(samples, Sample{int64(i - 31), vec(t, values...)})
			}
			c := ivfConfig(t, 5, 6, metric)
			previous := math.Inf(1)
			improved := false
			for iterations := 1; iterations <= 15; iterations++ {
				c.TrainingIterations = iterations
				centers, _, ready, err := Train(ctx, samples, c)
				if err != nil || !ready {
					t.Fatal(err)
				}
				var objective float64
				for _, sample := range samples {
					x := sample.Vector.Values()
					best := math.Inf(1)
					for _, center := range centers {
						y := center.Vector.Values()
						var squared, dot, xx, yy float64
						for k, a := range x {
							af, bf := float64(a), float64(y[k])
							squared += (af - bf) * (af - bf)
							dot += af * bf
							xx += af * af
							yy += bf * bf
						}
						score := squared
						if metric != vector.L2 {
							score = 1 - dot/math.Sqrt(xx*yy)
						}
						best = math.Min(best, score)
					}
					objective += best
				}
				if objective > previous+1e-5 {
					t.Fatalf("independent objective increased: iteration=%d previous=%.12g actual=%.12g", iterations, previous, objective)
				}
				if iterations > 1 && objective < previous-1e-5 {
					improved = true
				}
				previous = objective
			}
			if !improved {
				t.Fatal("training never improved after first mean update")
			}
			want, stats, ready, err := Train(ctx, samples, c)
			if err != nil || !ready {
				t.Fatal(err)
			}
			// Exact reported charges must suffice; one fewer unit must fail with
			// no published centroids, not a partially trained success.
			for _, kind := range []string{"work", "memory"} {
				bounded := c
				if kind == "work" {
					bounded.MaxTrainingWork = stats.ComponentWork
				} else {
					bounded.MaxMemoryBytes = stats.PeakBytes
				}
				got, actual, ok, err := Train(ctx, samples, bounded)
				if err != nil || !ok || !reflect.DeepEqual(got, want) || actual != stats {
					t.Fatalf("exact %s quota changed success: stats=%+v want=%+v error=%v", kind, actual, stats, err)
				}
				if kind == "work" {
					bounded.MaxTrainingWork--
				} else {
					bounded.MaxMemoryBytes--
				}
				got, actual, ok, err = Train(ctx, samples, bounded)
				if !errors.Is(err, ErrLimit) || ok || got != nil || actual.StopReason != "resource_limit" {
					t.Fatalf("one-unit-short %s quota: ready=%v centers=%v stats=%+v error=%v", kind, ok, got, actual, err)
				}
			}
		})
	}
}
