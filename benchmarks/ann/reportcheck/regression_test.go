package reportcheck

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

// These tests are proposed source only, not a recorded execution result. They
// depend on the unchanged V3 modelMethodReport helper and its compile-embedded
// historical JSON. Each call creates independent in-memory data. No test calls
// run, guard, observedHardware, native database APIs, or the old review probe.
// See REVIEW.md for the complete source/import closure and execution HOLD.

func threeFixResummarize(t *testing.T, r *Report) {
	t.Helper()
	for i := range r.Samples {
		old := r.Samples[i]
		r.Samples[i] = Sample{K: old.K, Observations: old.Observations}
		if e := summarizeSample(&r.Samples[i]); e != nil {
			t.Fatal(e)
		}
	}
}

// Refresh every summary before both entry points. Rejection must name the
// targeted material invariant, so a stale summary or unrelated wire failure
// cannot turn a broken regression into an apparent success.
func threeFixCheck(t *testing.T, r Report, want string) {
	t.Helper()
	threeFixResummarize(t, &r)
	direct := r.ValidatePass()
	raw, e := json.Marshal(r)
	if e != nil {
		t.Fatal(e)
	}
	_, wire := DecodeReport(raw)
	for name, err := range map[string]error{"ValidatePass": direct, "DecodeReport": wire} {
		if want == "" {
			if err != nil {
				t.Fatalf("%s rejected valid control: %v", name, err)
			}
		} else if err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("%s: want %q, got %v", name, want, err)
		}
	}
}

func TestANNThreeFixUnchangedFixtureControls(t *testing.T) {
	for _, method := range []string{"hnsw", "ivfflat"} {
		t.Run(method, func(t *testing.T) {
			threeFixCheck(t, modelMethodReport(t, method), "")
		})
	}
}

func TestANNThreeFixHNSWRejectsBrokenConservation(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Observation)
	}{
		{"omit traversal reads", func(o *Observation) {
			o.ANN.Stats.ANNNodeReads = o.ANN.Stats.BaseRowFetches
		}},
		{"one read too few", func(o *Observation) {
			o.ANN.Stats.ANNNodeReads--
		}},
		{"one read too many", func(o *Observation) {
			o.ANN.Stats.ANNNodeReads++
		}},
		{"rescore without traversal", func(o *Observation) {
			s := &o.ANN.Stats
			s.ANNVisited = s.ANNExactRescores - 1
			s.ANNNodeReads = s.BaseRowFetches + s.ANNVisited
		}},
		{"visit without distance", func(o *Observation) {
			o.ANN.Stats.ANNDistanceEvaluations = o.ANN.Stats.ANNVisited - 1
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := modelMethodReport(t, "hnsw")
			threeFixCheck(t, r, "")
			o := &r.Samples[0].Observations[0]
			if o.ANN.Stats.ANNExactRescores < 2 || o.ANN.Stats.ANNVisited < 2 {
				t.Fatal("fixture no longer supports positive old-bound counter mutations")
			}
			tc.mutate(o)
			threeFixCheck(t, r, "HNSW traversal/read/rescore conservation mismatch")
		})
	}
}

func TestANNThreeFixHNSWBoundaryControls(t *testing.T) {
	// Equality at both lower bounds must be admitted. These are arithmetic
	// controls, not claims that modified counters were measured by the engine.
	r := modelMethodReport(t, "hnsw")
	s := &r.Samples[0].Observations[0].ANN.Stats
	s.ANNVisited = s.ANNExactRescores
	s.ANNDistanceEvaluations = s.ANNVisited
	s.ANNNodeReads = s.BaseRowFetches + s.ANNVisited
	threeFixCheck(t, r, "")

	// Cumulative-counter arithmetic control: separate iterations may count
	// the same traversal node again, while source rechecks stay deduplicated.
	// This does not claim the synthetic widening was measured or necessary
	// for this particular unfiltered fixture.
	r = modelMethodReport(t, "hnsw")
	o := &r.Samples[2].Observations[0]
	q := queryPolicy(r.Config)
	ef := max(q.EFSearch, min(r.Samples[2].K, q.MaxEFSearch))
	o.ANN.Plan.ANNEffectiveEF = min(q.MaxEFSearch, max(ef+1, ef*2))
	s = &o.ANN.Stats
	s.ANNIterations = 2
	s.ANNVisited = int64(r.Config.Rows) + 1
	s.ANNDistanceEvaluations = s.ANNVisited
	s.ANNNodeReads = s.BaseRowFetches + s.ANNVisited
	threeFixCheck(t, r, "")

	for _, tc := range []struct {
		name string
		set  func(*Observation)
		want string
	}{
		{"fewer reads than source fetches", func(o *Observation) { o.ANN.Stats.ANNNodeReads = o.ANN.Stats.BaseRowFetches - 1 }, "HNSW traversal/read/rescore conservation mismatch"},
		{"max signed visits", func(o *Observation) { o.ANN.Stats.ANNVisited = math.MaxInt64 }, "HNSW traversal/read/rescore conservation mismatch"},
		{"max signed reads", func(o *Observation) { o.ANN.Stats.ANNNodeReads = math.MaxInt64 }, "native query work quota exceeded"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := modelMethodReport(t, "hnsw")
			tc.set(&r.Samples[0].Observations[0])
			threeFixCheck(t, r, tc.want)
		})
	}
}

func threeFixAllLists(t *testing.T, r *Report) *Observation {
	t.Helper()
	q := queryPolicy(r.Config)
	if q.Probes != 6 || q.MaxProbes != 8 || r.Config.Lists != 8 {
		t.Fatal("fixture no longer has the documented 6 -> 8 widening boundary")
	}
	o := &r.Samples[0].Observations[0]
	o.ANN.Plan.ANNEffectiveProbes = r.Config.Lists
	o.ANN.Stats.ANNListsOpened = int64(r.Config.Lists)
	o.ANN.Stats.ANNIterations = 2
	return o
}

func threeFixMembers(r *Report, o *Observation, members int64) {
	s := &o.ANN.Stats
	s.ANNMemberKeysRead = members
	s.BaseRowFetches = members
	s.Candidates = members
	s.ANNIdentityLookups = members
	s.ANNExactRescores = members
	s.ScoredRows = members
	s.ScoredComponents = (members + 1) * int64(r.Config.Dimensions)
}

func TestANNThreeFixIVFAllListsCoverage(t *testing.T) {
	t.Run("old members with all lists", func(t *testing.T) {
		r := modelMethodReport(t, "ivfflat")
		threeFixCheck(t, r, "")
		o := threeFixAllLists(t, &r)
		if o.ANN.Stats.ANNMemberKeysRead >= int64(r.Config.Rows) {
			t.Fatal("fixture no longer exposes missing source memberships")
		}
		threeFixCheck(t, r, "IVF all-lists/source coverage mismatch")
	})
	t.Run("one member short with every dependent count refreshed", func(t *testing.T) {
		r := modelMethodReport(t, "ivfflat")
		o := threeFixAllLists(t, &r)
		threeFixMembers(&r, o, int64(r.Config.Rows)-1)
		threeFixCheck(t, r, "IVF all-lists/source coverage mismatch")
	})
	t.Run("all members exactly", func(t *testing.T) {
		r := modelMethodReport(t, "ivfflat")
		o := threeFixAllLists(t, &r)
		threeFixMembers(&r, o, int64(r.Config.Rows))
		threeFixCheck(t, r, "")
	})
	t.Run("one member too many", func(t *testing.T) {
		r := modelMethodReport(t, "ivfflat")
		o := threeFixAllLists(t, &r)
		threeFixMembers(&r, o, int64(r.Config.Rows)+1)
		threeFixCheck(t, r, "native base/component/iteration/memory conservation mismatch")
	})
	t.Run("partial lists may have fewer members", func(t *testing.T) {
		r := modelMethodReport(t, "ivfflat")
		o := &r.Samples[0].Observations[0]
		if o.ANN.Stats.ANNListsOpened >= int64(r.Config.Lists) || o.ANN.Stats.ANNMemberKeysRead >= int64(r.Config.Rows) {
			t.Fatal("fixture no longer supplies a partial-list control")
		}
		threeFixCheck(t, r, "")
	})
}

func TestANNThreeFixSameRowDistance(t *testing.T) {
	for _, method := range []string{"hnsw", "ivfflat"} {
		t.Run(method, func(t *testing.T) {
			for _, oneULP := range []bool{false, true} {
				name := "whole-unit discrepancy"
				if oneULP {
					name = "one-ULP discrepancy"
				}
				t.Run(name, func(t *testing.T) {
					r := modelMethodReport(t, method)
					threeFixCheck(t, r, "")
					o := &r.Samples[0].Observations[0]
					if r.Samples[0].K != 1 || o.Exact.Hits[0].RowID != o.ANN.Hits[0].RowID {
						t.Fatal("fixture no longer supplies a same-row singleton")
					}
					if oneULP {
						o.ANN.Hits[0].Distance = math.Nextafter(o.Exact.Hits[0].Distance, math.Inf(1))
					} else {
						o.ANN.Hits[0].Distance += 1
					}
					// K=1 preserves ordering; the RowID intersection is unchanged.
					threeFixCheck(t, r, "same-row exact/rescore distance mismatch")
				})
			}
			t.Run("numeric signed zero", func(t *testing.T) {
				r := modelMethodReport(t, method)
				o := &r.Samples[0].Observations[0]
				o.Exact.Hits[0].Distance = 0
				o.ANN.Hits[0].Distance = math.Copysign(0, -1)
				threeFixCheck(t, r, "")
			})
			t.Run("different rows need not have equal distances", func(t *testing.T) {
				r := modelMethodReport(t, method)
				o := &r.Samples[0].Observations[0]
				o.ANN.Hits[0].RowID = o.Exact.Hits[0].RowID%int64(r.Config.Rows) + 1
				o.ANN.Hits[0].Distance = o.Exact.Hits[0].Distance + 1
				o.Hits = 0
				threeFixCheck(t, r, "")
			})
			t.Run("overlap matches RowID across different ranks", func(t *testing.T) {
				r := modelMethodReport(t, method)
				o := &r.Samples[1].Observations[0]
				exactIDs := make(map[int64]bool)
				for _, h := range o.Exact.Hits {
					exactIDs[h.RowID] = true
				}
				newHit := o.Exact.Hits[len(o.Exact.Hits)-1]
				newHit.RowID = 1
				for exactIDs[newHit.RowID] {
					newHit.RowID++
				}
				if newHit.RowID > int64(r.Config.Rows) {
					t.Fatal("fixture has no nonoverlapping RowID")
				}
				newHit.Distance += 1
				o.ANN.Hits = append(o.ANN.Hits[:0], o.Exact.Hits[1:]...)
				o.ANN.Hits = append(o.ANN.Hits, newHit)
				o.Hits = r.Samples[1].K - 1
				threeFixCheck(t, r, "")
			})
		})
	}
}
