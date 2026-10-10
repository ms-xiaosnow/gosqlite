package ann

import (
	"bytes"
	"context"
	"errors"
	"math"
	"math/rand/v2"
	"reflect"
	"sort"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

func typedIVFConfig(t testing.TB, kind ValueKind, dims, lists int, metric vector.MetricKind) TypedConfig {
	t.Helper()
	c := typedConfig(t, IVFFlat, kind, dims, metric)
	c.Lists = lists
	return c
}
func TestTypedIVFBitRealMajorityTrainingAndDeterministicTies(t *testing.T) {
	c := typedIVFConfig(t, BitKind, 8, 1, vector.MetricKind(vector.Hamming))
	samples := []TypedSample{{RowID: 9, Value: bitValue(t, "00000000")}, {RowID: -3, Value: bitValue(t, "11000000")}, {RowID: 4, Value: bitValue(t, "10100000")}}
	centers, stats, ready, err := TrainValues(context.Background(), samples, c)
	if err != nil || !ready || len(centers) != 1 || CentroidValue(centers[0]).Bit().String() != "10000000" || stats.Iterations < 2 {
		t.Fatalf("not genuine majority Lloyd training: %+v %+v %v", centers, stats, err)
	}
	// The centroid is absent from the source corpus: a nearest-exemplar stub fails.
	for _, s := range samples {
		if reflect.DeepEqual(SampleValue(s), CentroidValue(centers[0])) {
			t.Fatal("expected derived majority center")
		}
	}
	reversed := append([]TypedSample(nil), samples...)
	sort.Slice(reversed, func(i, j int) bool { return reversed[i].RowID > reversed[j].RowID })
	again, againStats, againReady, err := TrainValues(context.Background(), reversed, c)
	if err != nil || !againReady || !reflect.DeepEqual(again, centers) || againStats != stats {
		t.Fatal("order changed deterministic training", err)
	}
	tie := []TypedSample{{RowID: -5, Value: bitValue(t, "00000000")}, {RowID: 7, Value: bitValue(t, "11111111")}}
	got, _, ready, err := TrainValues(context.Background(), tie, c)
	first := 0
	if SamplePriority(c.Seed, 7) < SamplePriority(c.Seed, -5) {
		first = 1
	}
	if err != nil || !ready || !reflect.DeepEqual(CentroidValue(got[0]), SampleValue(tie[first])) {
		t.Fatal("tied bit did not retain seeded prior", err)
	}
	c.Lists = 3
	duplicate := []TypedSample{{RowID: 1, Value: bitValue(t, "00000000")}, {RowID: 2, Value: bitValue(t, "00000000")}, {RowID: 3, Value: bitValue(t, "00000000")}}
	got, st, ready, err := TrainValues(context.Background(), duplicate, c)
	if err != nil || !ready || st.EmptyClusters == 0 {
		t.Fatal("empty list retention", err)
	}
	for _, x := range got {
		if CentroidValue(x).Bit().String() != "00000000" {
			t.Fatal("empty cluster synthesized bits")
		}
	}
}

func TestTypedIVFHalfDerivedRoundingAndOriginalMagnitudes(t *testing.T) {
	for _, metric := range []vector.MetricKind{vector.L2, vector.NegativeIP, vector.Cosine} {
		c := typedIVFConfig(t, HalfKind, 2, 1, metric)
		samples := []TypedSample{{RowID: 1, Value: halfValue(t, 100, 0)}, {RowID: 2, Value: halfValue(t, 0, 1)}}
		before := make([][]byte, len(samples))
		for i, s := range samples {
			before[i], _ = SampleValue(s).MarshalBinary()
		}
		centers, stats, ready, err := TrainValues(context.Background(), samples, c)
		if err != nil || !ready || stats.Iterations < 2 {
			t.Fatal("half train", err)
		}
		if CentroidValue(centers[0]).Kind() != HalfKind {
			t.Fatal("half center silently stored as dense")
		}
		expected := halfValue(t, 50, .5)
		if metric != vector.L2 {
			expected = halfValue(t, float32(1/math.Sqrt(2)), float32(1/math.Sqrt(2)))
		}
		if !reflect.DeepEqual(CentroidValue(centers[0]), expected) {
			t.Fatalf("incorrect derived half center: %s", CentroidValue(centers[0]).Half())
		}
		for i, s := range samples {
			after, _ := SampleValue(s).MarshalBinary()
			if !bytes.Equal(after, before[i]) {
				t.Fatal("source magnitudes changed")
			}
		}
		encoded, err := EncodeValue(CentroidValue(centers[0]), c)
		if err != nil || string(encoded[:4]) != "GSH\x00" {
			t.Fatal("half centroid codec", err)
		}
		decoded, err := DecodeValue(encoded, c)
		if err != nil || ValidateTypedCentroid(TypedCentroid{ID: 1, Value: decoded}, c) != nil {
			t.Fatal("half centroid reopen codec", err)
		}
	}
	c := typedIVFConfig(t, HalfKind, 2, 1, vector.NegativeIP)
	zero := halfValue(t, 0, 0)
	trainable, err := TrainableValue(zero, c)
	if err != nil || trainable {
		t.Fatal("IP zero training direction", err)
	}
	centers, _, ready, err := TrainValues(context.Background(), []TypedSample{{RowID: 1, Value: halfValue(t, 2, 0)}}, c)
	if err != nil || !ready {
		t.Fatal(err)
	}
	id, _, err := AssignValue(context.Background(), zero, centers, c, TypedLimits{})
	if err != nil || id != 1 {
		t.Fatal("IP zero must remain assignable", err)
	}
	c.Metric = vector.Cosine
	c.Opclass = "halfvec_cosine_ops"
	if _, err := TrainableValue(zero, c); !errors.Is(err, vector.ErrZeroNorm) {
		t.Fatal("half IVF cosine zero accepted")
	}
}

func TestTypedIVFSamplingUntrainedAndExactBudgets(t *testing.T) {
	for _, kind := range []ValueKind{HalfKind, BitKind} {
		v := halfValue(t, 1, 2, 3)
		metric := vector.L2
		if kind == BitKind {
			v = bitValue(t, "101")
			metric = vector.MetricKind(vector.Hamming)
		}
		c := typedIVFConfig(t, kind, 3, 2, metric)
		c.TrainingSamples = 4
		sampler, err := NewTypedIVFSampler(c)
		if err != nil {
			t.Fatal(err)
		}
		for id := int64(-5); id < 10; id++ {
			if err := sampler.AddValue(context.Background(), id, v); err != nil {
				t.Fatal(err)
			}
		}
		samples, err := sampler.Samples(context.Background())
		if err != nil || len(samples) != 4 {
			t.Fatal(err)
		}
		for _, s := range samples {
			if !reflect.DeepEqual(SampleValue(s), v) {
				t.Fatal("sampler changed source representation")
			}
		}
		empty, st, ready, err := TrainValues(context.Background(), samples[:1], c)
		if err != nil || ready || empty != nil || st.StopReason != "insufficient_trainable_samples" {
			t.Fatal("untrained state", err)
		}
		centers, stats, ready, err := TrainValues(context.Background(), samples, c)
		if err != nil || !ready {
			t.Fatal(err)
		}
		c.MaxTrainingWork = stats.ComponentWork
		exact, _, ready, err := TrainValues(context.Background(), samples, c)
		if err != nil || !ready || !reflect.DeepEqual(exact, centers) {
			t.Fatal("exact training work cap", err)
		}
		c.MaxTrainingWork--
		partial, spent, ready, err := TrainValues(context.Background(), samples, c)
		if !errors.Is(err, ErrLimit) || ready || partial != nil || spent.ComponentWork > c.MaxTrainingWork {
			t.Fatal("training work exceeded or partial result", err)
		}
		c.MaxTrainingWork++
		order, pstats, err := ProbeValue(context.Background(), v, centers, c, TypedLimits{})
		if err != nil || len(order) != 2 || pstats.ComponentWork != int64(2*v.WorkUnits()) || pstats.NodeReads != 0 || pstats.EdgeReads != 0 {
			t.Fatal("typed probe accounting", err)
		}
		_, _, err = ProbeValue(context.Background(), v, centers, c, TypedLimits{MaxComponentWork: uint64(pstats.ComponentWork), ComponentWorkLimited: true})
		if err != nil {
			t.Fatal("exact probe work", err)
		}
		partialProbe, spentProbe, err := ProbeValue(context.Background(), v, centers, c, TypedLimits{MaxComponentWork: uint64(pstats.ComponentWork - 1), ComponentWorkLimited: true})
		if !errors.Is(err, ErrLimit) || partialProbe != nil || spentProbe.ComponentWork >= pstats.ComponentWork {
			t.Fatal("probe cap", err)
		}
		partialProbe, _, err = ProbeValue(context.Background(), v, centers, c, TypedLimits{ComponentWorkLimited: true})
		if !errors.Is(err, ErrLimit) || partialProbe != nil {
			t.Fatal("zero probe cap", err)
		}
		limited := c
		limited.MaxMemoryBytes = stats.PeakBytes - 1
		partial, _, ready, err = TrainValues(context.Background(), samples, limited)
		if !errors.Is(err, ErrLimit) || ready || partial != nil {
			t.Fatal("training memory bound", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		partial, _, ready, err = TrainValues(ctx, samples, c)
		if !errors.Is(err, context.Canceled) || ready || partial != nil {
			t.Fatal("training cancellation", err)
		}
	}
}

func TestTypedIVFMaximumBitWidthAndInFlightCancellation(t *testing.T) {
	c := typedIVFConfig(t, BitKind, 64000, 1, vector.MetricKind(vector.Hamming))
	b := make([]byte, 8000)
	for i := range b {
		b[i] = 0xaa
	}
	v, err := vector.NewBit(64000, b)
	if err != nil {
		t.Fatal(err)
	}
	samples := []TypedSample{{RowID: 1, Value: BitValue(v)}, {RowID: 2, Value: BitValue(v)}}
	centers, stats, ready, err := TrainValues(context.Background(), samples, c)
	if err != nil || !ready || stats.PeakBytes > 1<<20 || !reflect.DeepEqual(CentroidValue(centers[0]), BitValue(v)) {
		t.Fatal("maximum width packed training", err)
	}
	partial, _, ready, err := TrainValues(&errAfterContext{after: 50}, samples, c)
	if !errors.Is(err, context.Canceled) || ready || partial != nil {
		t.Fatal("mid-training cancellation", err)
	}
	if _, err := DistanceContext(&errAfterContext{after: 10}, c, BitValue(v), BitValue(v)); !errors.Is(err, context.Canceled) {
		t.Fatal("packed metric did not check cancellation", err)
	}
}

func TestTypedIVFSeededRecallAllAdvertisedTuples(t *testing.T) {
	for _, kind := range []ValueKind{HalfKind, BitKind} {
		metrics := []vector.MetricKind{vector.L2, vector.NegativeIP, vector.Cosine}
		if kind == BitKind {
			metrics = []vector.MetricKind{vector.MetricKind(vector.Hamming)}
		}
		for _, metric := range metrics {
			t.Run(string(kind)+"/"+string(metric), func(t *testing.T) {
				rng := rand.New(rand.NewPCG(20261010, 27))
				values := typedRecallCorpus(t, kind, 512, rng)
				queries := typedRecallCorpus(t, kind, 16, rng)
				c := typedIVFConfig(t, kind, values[0].Dims(), 24, metric)
				samples := make([]TypedSample, len(values))
				for i, v := range values {
					samples[i] = TypedSample{RowID: int64(i) - 256, Value: v}
				}
				centers, stats, ready, err := TrainValues(context.Background(), samples, c)
				if err != nil || !ready {
					t.Fatal(err)
				}
				lists := make([][]Candidate, c.Lists)
				for i, v := range values {
					id, _, err := AssignValue(context.Background(), v, centers, c, TypedLimits{})
					if err != nil {
						t.Fatal(err)
					}
					lists[id-1] = append(lists[id-1], Candidate{RowID: int64(i) - 256})
				}
				matched, scanned := 0, 0
				for _, q := range queries {
					order, pstats, err := ProbeValue(context.Background(), q, centers, c, TypedLimits{})
					if err != nil || pstats.DistanceEvaluations != c.Lists {
						t.Fatal(err)
					}
					exact := make([]Candidate, len(values))
					for i, v := range values {
						d, err := Distance(c, q, v)
						if err != nil {
							t.Fatal(err)
						}
						exact[i] = Candidate{RowID: int64(i) - 256, Distance: d}
					}
					candidate := []Candidate{}
					for _, p := range order[:20] {
						for _, member := range lists[p.ListID-1] {
							d, err := Distance(c, q, values[member.RowID+256])
							if err != nil {
								t.Fatal(err)
							}
							candidate = append(candidate, Candidate{RowID: member.RowID, Distance: d})
						}
					}
					scanned += len(candidate)
					sort.Slice(candidate, func(i, j int) bool { return candidateLess(candidate[i], candidate[j]) })
					sort.Slice(exact, func(i, j int) bool { return candidateLess(exact[i], exact[j]) })
					want := map[int64]bool{}
					for _, x := range exact[:10] {
						want[x.RowID] = true
					}
					if len(candidate) < 10 {
						t.Fatal("too few selected-list candidates")
					}
					for _, x := range candidate[:10] {
						if want[x.RowID] {
							matched++
						}
					}
				}
				recall := float64(matched) / 160
				t.Logf("kind=%s metric=%s seed=20261010/27 n=512 queries=16 lists=24 probes=20 recall@10=%.5f mean candidate rescoring=%.2f training work=%d", kind, metric, recall, float64(scanned)/16, stats.ComponentWork)
				if recall < .95 {
					t.Fatalf("recall %.5f", recall)
				}
				if scanned >= len(values)*len(queries) {
					t.Fatal("all-list exact scan disguised as IVF")
				}
			})
		}
	}
}
