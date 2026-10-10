package similarity_test

import (
	"math"
	"testing"

	sim "github.com/ms-xiaosnow/gosqlite/extension/similarity"
)

func countScore(a, b, c int, m sim.Metric) float64 {
	if a+b == 0 {
		return 1
	}
	if m == sim.Dice {
		return float64(2*c) / float64(a+b)
	}
	return float64(c) / float64(a+b-c)
}

func TestMinimumIntersectionExhaustive(t *testing.T) {
	for _, m := range []sim.Metric{sim.Dice, sim.Jaccard} {
		for a := 0; a <= 32; a++ {
			for b := 0; b <= 32; b++ {
				thresholds := []float64{0, math.SmallestNonzeroFloat64, .1, .2, .5, .7, 1}
				for c := 0; c <= min(a, b); c++ {
					value := countScore(a, b, c, m)
					thresholds = append(thresholds, value)
					if value > 0 {
						thresholds = append(thresholds, math.Nextafter(value, 0))
					}
					if value < 1 {
						thresholds = append(thresholds, math.Nextafter(value, 1))
					}
				}
				for _, threshold := range thresholds {
					wanted, possible := 0, false
					for c := 0; c <= min(a, b); c++ {
						if countScore(a, b, c, m) >= threshold {
							wanted, possible = c, true
							break
						}
					}
					got, can, err := sim.MinimumIntersection(a, b, threshold, m)
					if err != nil || got != wanted || can != possible {
						t.Fatalf("a%d b%d t%.18g %s: %d %v %v want %d %v", a, b, threshold, m, got, can, err, wanted, possible)
					}
					plan, err := sim.PlanCandidates(a, threshold)
					if err != nil {
						t.Fatal(err)
					}
					if plan.Mode == sim.UnionPostings && possible && wanted == 0 {
						t.Fatalf("unsafe union: a%d b%d t%g", a, b, threshold)
					}
				}
			}
		}
	}
}

func TestCandidateBoundsExtremes(t *testing.T) {
	for _, threshold := range []float64{math.NaN(), math.Inf(1), math.Inf(-1), -1, math.Nextafter(1, 2)} {
		if _, err := sim.PlanCandidates(1, threshold); err == nil {
			t.Fatal("accepted threshold", threshold)
		}
		if _, _, err := sim.MinimumIntersection(1, 1, threshold, sim.Dice); err == nil {
			t.Fatal("accepted threshold", threshold)
		}
	}
	for _, n := range []int{-1, 1<<20 + 1, int(^uint(0) >> 1)} {
		if _, err := sim.PlanCandidates(n, .5); err == nil {
			t.Fatal("cardinality", n)
		}
		if _, _, err := sim.MinimumIntersection(1, n, .5, sim.Dice); err == nil {
			t.Fatal("cardinality", n)
		}
	}
	if _, _, err := sim.MinimumIntersection(1, 1, .5, "bad"); err == nil {
		t.Fatal("metric")
	}
	for _, m := range []sim.Metric{sim.Dice, sim.Jaccard} {
		n, ok, err := sim.MinimumIntersection(1<<20, 1<<20, 1, m)
		if err != nil || !ok || n != 1<<20 {
			t.Fatal(n, ok, err)
		}
		n, ok, err = sim.MinimumIntersection(1<<20, 1, math.SmallestNonzeroFloat64, m)
		if err != nil || !ok || n != 1 {
			t.Fatal(n, ok, err)
		}
	}
	cases := []struct {
		n    int
		t    float64
		want sim.CandidateMode
	}{
		{0, 0, sim.FullScan}, {0, 1, sim.FullScan}, {5, 0, sim.FullScan}, {5, .1, sim.UnionPostings}, {5, 1, sim.UnionPostings},
	}
	for _, tt := range cases {
		p, err := sim.PlanCandidates(tt.n, tt.t)
		if err != nil || p.Mode != tt.want || p.Reason == "" {
			t.Fatal(p, err)
		}
	}
}
