package ann

import (
	"bytes"
	"context"
	"errors"
	"math/rand/v2"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

func typedConfig(t testing.TB, method string, kind ValueKind, dims int, metric vector.MetricKind) TypedConfig {
	t.Helper()
	c, err := DefaultTypedConfig(method, kind, dims, metric)
	if err != nil {
		t.Fatal(err)
	}
	return c
}
func halfValue(t testing.TB, x ...float32) Value {
	t.Helper()
	v, e := vector.NewHalf(x)
	if e != nil {
		t.Fatal(e)
	}
	return HalfValue(v)
}
func bitValue(t testing.TB, s string) Value {
	t.Helper()
	v, e := vector.ParseBit(s)
	if e != nil {
		t.Fatal(e)
	}
	return BitValue(v)
}
func sparseValue(t testing.TB, d int, e ...vector.SparseEntry) Value {
	t.Helper()
	v, err := vector.NewSparse(d, e)
	if err != nil {
		t.Fatal(err)
	}
	return SparseValue(v)
}

func TestTypedConfigMatrixCanonicalAndLimits(t *testing.T) {
	numeric := []vector.MetricKind{vector.L2, vector.L1, vector.NegativeIP, vector.Cosine}
	for _, kind := range []ValueKind{DenseKind, HalfKind, BitKind, SparseKind} {
		metrics := numeric
		maxDims := vector.ANNMaxDimensions
		switch kind {
		case HalfKind:
			maxDims = 4000
		case BitKind:
			maxDims = 64000
			metrics = []vector.MetricKind{vector.MetricKind(vector.Hamming), vector.MetricKind(vector.Jaccard)}
		case SparseKind:
			maxDims = 1000000000
		}
		for _, method := range []string{HNSW, IVFFlat} {
			for _, metric := range metrics {
				supported := method == HNSW || (kind != SparseKind && metric != vector.L1 && metric != vector.MetricKind(vector.Jaccard))
				c, err := DefaultTypedConfig(method, kind, maxDims, metric)
				if !supported {
					if !errors.Is(err, ErrConfig) {
						t.Fatalf("accepted unsupported tuple %s/%s/%s", method, kind, metric)
					}
					continue
				}
				if err != nil {
					t.Fatal(err)
				}
				data, err := c.Canonical()
				if err != nil {
					t.Fatal(err)
				}
				got, err := ParseTypedConfig(data)
				if err != nil || got != c {
					t.Fatalf("roundtrip %s: %+v %v", data, got, err)
				}
				normalized, err := (TypedConfig{Kind: kind, Config: Config{Method: method, Dimensions: maxDims, Metric: metric}}).Normalize()
				if err != nil || normalized != c {
					t.Fatalf("normalize %v", err)
				}
				if _, err := DefaultTypedConfig(method, kind, maxDims+1, metric); !errors.Is(err, ErrConfig) {
					t.Fatal("excess dimensions accepted")
				}
				if kind != DenseKind {
					if c.Version != 2 || !bytes.Contains(data, []byte(`"value_kind":"`+string(kind)+`"`)) {
						t.Fatalf("typed identity missing: %s", data)
					}
					for _, bad := range [][]byte{
						append(append([]byte(nil), data...), ' '),
						bytes.Replace(data, []byte(`"version":2`), []byte(`"version":1`), 1),
						bytes.Replace(data, []byte(`"identity-v1"`), []byte(`"implicit-quantize"`), 1),
						bytes.Replace(data, []byte(`"value_kind":`), []byte(`"value_kind":"halfvec","value_kind":`), 1),
						bytes.Replace(data, []byte(`"value_codec":`), []byte(`"unknown":1,"value_codec":`), 1),
					} {
						if _, err := ParseTypedConfig(bad); !errors.Is(err, ErrConfig) {
							t.Fatalf("accepted altered typed identity %s", bad)
						}
					}
				}
			}
		}
	}
	for _, kind := range []ValueKind{DenseKind, HalfKind, SparseKind} {
		if _, err := DefaultTypedConfig(HNSW, kind, 3, vector.MetricKind(vector.Hamming)); !errors.Is(err, ErrConfig) {
			t.Fatal("bit metric accepted for numeric")
		}
	}
	if _, err := DefaultTypedConfig(HNSW, BitKind, 3, vector.L2); !errors.Is(err, ErrConfig) {
		t.Fatal("numeric metric accepted for bit")
	}
	if _, err := DefaultTypedConfig(HNSW, ValueKind("vector"), 3, vector.L2); !errors.Is(err, ErrConfig) {
		t.Fatal("unknown dense alias accepted")
	}
}

func TestTypedValuesCodecsAndSparseSupportBounds(t *testing.T) {
	values := []Value{DenseValue(vec(t, 1, 2, 3)), halfValue(t, 1, 2, 3), bitValue(t, "101"), sparseValue(t, 3, vector.SparseEntry{Index: 2, Value: 3})}
	for _, v := range values {
		metric := vector.L2
		if v.Kind() == BitKind {
			metric = vector.MetricKind(vector.Hamming)
		}
		c := typedConfig(t, HNSW, v.Kind(), 3, metric)
		data, err := EncodeValue(v, c)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := v.MarshalBinary()
		if !bytes.Equal(data, raw) {
			t.Fatal("codec added implicit payload transform")
		}
		got, err := DecodeValue(data, c)
		if err != nil || !reflect.DeepEqual(got, v) {
			t.Fatal("codec roundtrip", err)
		}
		for _, bad := range [][]byte{data[:len(data)-1], append(append([]byte(nil), data...), 0)} {
			if _, err := DecodeValue(bad, c); err == nil {
				t.Fatal("malformed payload accepted")
			}
		}
		for _, other := range values {
			if other.Kind() == v.Kind() {
				continue
			}
			b, _ := other.MarshalBinary()
			if _, err := DecodeValue(b, c); err == nil {
				t.Fatal("foreign codec accepted")
			}
			if _, err := Distance(c, v, other); err == nil {
				t.Fatal("cross-kind distance accepted")
			}
		}
		n := TypedNode{ID: 1, RowID: -5, Live: true, Level: 0, Value: v, Neighbors: [][]int64{nil}}
		if err := ValidateTypedNode(n, c); err != nil {
			t.Fatal(err)
		}
		b, err := EncodeTypedAdjacency(n, c)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := DecodeTypedAdjacency(b, 1, 0, c); err != nil {
			t.Fatal(err)
		}
	}
	const dims = 1000000000
	entries := make([]vector.SparseEntry, 1001)
	for i := range entries {
		entries[i] = vector.SparseEntry{Index: uint32(i * 999), Value: 1}
	}
	c := typedConfig(t, HNSW, SparseKind, dims, vector.L2)
	atLimit := sparseValue(t, dims, entries[:1000]...)
	if err := ValidateValue(atLimit, c); err != nil {
		t.Fatal(err)
	}
	tooLarge := sparseValue(t, dims, entries...)
	if err := ValidateValue(tooLarge, c); !errors.Is(err, vector.ErrDimensions) {
		t.Fatal("NNZ limit", err)
	}
	payload, _ := tooLarge.MarshalBinary()
	if _, err := DecodeValue(payload, c); err == nil {
		t.Fatal("oversized sparse payload accepted")
	}
	tiny := sparseValue(t, dims, vector.SparseEntry{Index: 999999999, Value: 2})
	if tiny.MemoryBytes() != 8 || tiny.WorkUnits() != 1 || DistanceWork(tiny, atLimit) != 1001 || TypedNodeMemoryBytes(TypedNode{Value: tiny}) > 1024 {
		t.Fatal("sparse accounting scales with declared dimensions")
	}
	if _, err := Distance(c, tiny, tiny); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []ValueKind{HalfKind, SparseKind} {
		z := halfValue(t, 0, 0, 0)
		if kind == SparseKind {
			z = sparseValue(t, 3)
		}
		cosine := typedConfig(t, HNSW, kind, 3, vector.Cosine)
		if err := ValidateValue(z, cosine); !errors.Is(err, vector.ErrZeroNorm) {
			t.Fatal("cosine zero accepted")
		}
	}
}

func TestTypedHNSWRealTraversalMutationAndWorkBudgets(t *testing.T) {
	for _, v := range []Value{halfValue(t, 1, 2), bitValue(t, "101010101"), sparseValue(t, 1000000000, vector.SparseEntry{Index: 999999999, Value: 2})} {
		metric := vector.L2
		if v.Kind() == BitKind {
			metric = vector.MetricKind(vector.Hamming)
		}
		c := typedConfig(t, HNSW, v.Kind(), v.Dims(), metric)
		store := newTypedMemoryStore()
		state := NewState()
		for _, id := range []int64{9, -7, 3} {
			if err := InsertValue(context.Background(), store, &state, id, v, c); err != nil {
				t.Fatal(err)
			}
		}
		got, stats, err := SearchValue(context.Background(), store, state, v, c, 10, TypedLimits{})
		if err != nil || len(got) != 3 || got[0].RowID != -7 || stats.EdgeReads == 0 {
			t.Fatalf("typed traversal %s: %v %+v %v", v.Kind(), got, stats, err)
		}
		if stats.ComponentWork != int64(DistanceWork(v, v))*int64(stats.DistanceEvaluations) {
			t.Fatal("incorrect component work")
		}
		exact := uint64(stats.ComponentWork)
		got, at, err := SearchValue(context.Background(), store, state, v, c, 10, TypedLimits{MaxComponentWork: exact, ComponentWorkLimited: true})
		if err != nil || len(got) != 3 || at.ComponentWork != stats.ComponentWork {
			t.Fatal("exact work budget", err)
		}
		for _, budget := range []uint64{0, exact - 1} {
			got, used, err := SearchValue(context.Background(), store, state, v, c, 10, TypedLimits{MaxComponentWork: budget, ComponentWorkLimited: true})
			if !errors.Is(err, ErrLimit) || got != nil || uint64(used.ComponentWork) > budget {
				t.Fatal("work limit returned partial", err)
			}
		}
		if err := TombstoneValue(context.Background(), store, &state, 2, c); err != nil {
			t.Fatal(err)
		}
		if err := InsertValue(context.Background(), store, &state, -7, v, c); err != nil {
			t.Fatal(err)
		}
		got, _, err = SearchValue(context.Background(), store, state, v, c, 10, TypedLimits{})
		if err != nil || len(got) != 3 || got[0].NodeID != 4 {
			t.Fatal("typed tombstone/reused rowid", err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if got, _, err := SearchValue(ctx, store, state, v, c, 10, TypedLimits{}); !errors.Is(err, context.Canceled) || got != nil {
			t.Fatal("cancellation", err)
		}
	}
	zero := sparseValue(t, 1000000000)
	c := typedConfig(t, HNSW, SparseKind, zero.Dims(), vector.L2)
	store := newTypedMemoryStore()
	state := NewState()
	if err := InsertValue(context.Background(), store, &state, 1, zero, c); err != nil {
		t.Fatal(err)
	}
	got, stats, err := SearchValue(context.Background(), store, state, zero, c, 1, TypedLimits{ComponentWorkLimited: true})
	if err != nil || len(got) != 1 || stats.ComponentWork != 0 || stats.DistanceEvaluations != 1 {
		t.Fatal("zero support budget", err)
	}
}

func typedRecallCorpus(t *testing.T, kind ValueKind, n int, rng *rand.Rand) []Value {
	t.Helper()
	out := make([]Value, n)
	for i := range out {
		switch kind {
		case BitKind:
			b := make([]byte, 8)
			for j := range b {
				b[j] = byte(rng.Uint32())
			}
			b[7] &= 0xfe
			v, err := vector.NewBit(63, b)
			if err != nil {
				t.Fatal(err)
			}
			out[i] = BitValue(v)
		case HalfKind:
			x := make([]float32, 12)
			for j := range x {
				x[j] = float32(rng.NormFloat64())
			}
			out[i] = halfValue(t, x...)
		case SparseKind:
			e := make([]vector.SparseEntry, 12)
			for j := range e {
				e[j] = vector.SparseEntry{Index: uint32(j * 83333333), Value: float32(rng.NormFloat64())}
			}
			out[i] = sparseValue(t, 1000000000, e...)
		}
	}
	return out
}
func TestTypedHNSWSeededRecallAllAdvertisedTuples(t *testing.T) {
	for _, kind := range []ValueKind{HalfKind, BitKind, SparseKind} {
		metrics := []vector.MetricKind{vector.L2, vector.L1, vector.NegativeIP, vector.Cosine}
		if kind == BitKind {
			metrics = []vector.MetricKind{vector.MetricKind(vector.Hamming), vector.MetricKind(vector.Jaccard)}
		}
		for _, metric := range metrics {
			t.Run(string(kind)+"/"+string(metric), func(t *testing.T) {
				rng := rand.New(rand.NewPCG(20261010, 19))
				values := typedRecallCorpus(t, kind, 384, rng)
				queries := typedRecallCorpus(t, kind, 16, rng)
				c := typedConfig(t, HNSW, kind, values[0].Dims(), metric)
				store := newTypedMemoryStore()
				state := NewState()
				for i, v := range values {
					if err := InsertValue(context.Background(), store, &state, int64(i)-192, v, c); err != nil {
						t.Fatal(err)
					}
				}
				matched, reads, work := 0, 0, int64(0)
				for _, q := range queries {
					hits, stats, err := SearchValue(context.Background(), store, state, q, c, 100, TypedLimits{})
					if err != nil || len(hits) < 10 {
						t.Fatal(err)
					}
					reads += stats.NodeReads
					work += stats.ComponentWork
					exact := make([]Candidate, len(values))
					for i, v := range values {
						d, err := Distance(c, q, v)
						if err != nil {
							t.Fatal(err)
						}
						exact[i] = Candidate{RowID: int64(i) - 192, Distance: d}
					}
					sort.Slice(exact, func(i, j int) bool { return candidateLess(exact[i], exact[j]) })
					want := map[int64]bool{}
					for _, x := range exact[:10] {
						want[x.RowID] = true
					}
					for _, hit := range hits[:10] {
						if want[hit.RowID] {
							matched++
						}
					}
				}
				recall := float64(matched) / 160
				t.Logf("kind=%s metric=%s seed=20261010/19 n=384 queries=16 ef=100 recall@10=%.5f mean node reads=%.2f component work=%d", kind, metric, recall, float64(reads)/16, work)
				if recall < .95 {
					t.Fatalf("recall %.5f", recall)
				}
			})
		}
	}
}

func FuzzTypedConfigCanonical(f *testing.F) {
	for _, kind := range []ValueKind{HalfKind, BitKind, SparseKind} {
		metric := vector.L2
		if kind == BitKind {
			metric = vector.MetricKind(vector.Hamming)
		}
		c, _ := DefaultTypedConfig(HNSW, kind, 3, metric)
		b, _ := c.Canonical()
		f.Add(b)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		c, err := ParseTypedConfig(b)
		if err != nil {
			return
		}
		out, err := c.Canonical()
		if err != nil || !bytes.Equal(out, b) {
			t.Fatal("noncanonical accepted")
		}
	})
}

func TestTypedSparseNoImplicitDenseMaterialization(t *testing.T) {
	c := typedConfig(t, HNSW, SparseKind, 1000000000, vector.Cosine)
	v := sparseValue(t, c.Dimensions, vector.SparseEntry{Index: 999999999, Value: 1})
	store := newTypedMemoryStore()
	state := NewState()
	for i := int64(0); i < 8; i++ {
		if err := InsertValue(context.Background(), store, &state, i, v, c); err != nil {
			t.Fatal(err)
		}
	}
	hits, stats, err := SearchValue(context.Background(), store, state, v, c, 8, TypedLimits{Limits: Limits{MaxBytes: 16384}})
	if err != nil || len(hits) != 8 || stats.ComponentWork > 100 || stats.PeakBytes > 16384 {
		t.Fatalf("billion dimensional sparse path not bounded: %+v %v", stats, err)
	}
	if raw, _ := c.Canonical(); strings.Contains(string(raw), "dense") {
		t.Fatal("unexpected dense transform")
	}
}

func FuzzTypedValueCodec(f *testing.F) {
	half, _ := vector.NewHalf([]float32{1, 0, -2})
	bit, _ := vector.ParseBit("101")
	sparse, _ := vector.NewSparse(1000000000, []vector.SparseEntry{{Index: 999999999, Value: 1}})
	for i, v := range []Value{HalfValue(half), BitValue(bit), SparseValue(sparse)} {
		b, _ := v.MarshalBinary()
		f.Add(uint8(i), b)
	}
	f.Fuzz(func(t *testing.T, tag uint8, b []byte) {
		kind, dims, metric := HalfKind, 3, vector.L2
		switch tag % 3 {
		case 1:
			kind, metric = BitKind, vector.MetricKind(vector.Hamming)
		case 2:
			kind, dims = SparseKind, 1000000000
		}
		c, err := DefaultTypedConfig(HNSW, kind, dims, metric)
		if err != nil {
			t.Fatal(err)
		}
		v, err := DecodeValue(b, c)
		if err != nil {
			return
		}
		out, err := EncodeValue(v, c)
		if err != nil || !bytes.Equal(out, b) {
			t.Fatal("noncanonical typed value accepted")
		}
	})
}

type typedMemoryStore struct{ nodes map[int64]TypedNode }

func newTypedMemoryStore() *typedMemoryStore {
	return &typedMemoryStore{nodes: make(map[int64]TypedNode)}
}
func (s *typedMemoryStore) GetNode(ctx context.Context, id int64) (TypedNode, error) {
	if err := ctx.Err(); err != nil {
		return TypedNode{}, err
	}
	n, ok := s.nodes[id]
	if !ok {
		return TypedNode{}, ErrCorrupt
	}
	return n, nil
}
func (s *typedMemoryStore) PutNode(ctx context.Context, n TypedNode) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.nodes[n.ID] = cloneTypedNode(n)
	return nil
}

func TestTypedDenseBridgePreservesCanonicalTopologyAndCentroids(t *testing.T) {
	for _, method := range []string{HNSW, IVFFlat} {
		for _, metric := range []vector.MetricKind{vector.L2, vector.NegativeIP, vector.Cosine} {
			old, err := DefaultConfig(method, 3, metric)
			if err != nil {
				t.Fatal(err)
			}
			typed, err := DefaultTypedConfig(method, DenseKind, 3, metric)
			if err != nil || typed.Config != old || typed.Kind != DenseKind {
				t.Fatal("dense default identity", err)
			}
			before, _ := old.Canonical()
			after, _ := typed.Canonical()
			if !bytes.Equal(before, after) {
				t.Fatal("dense canonical changed")
			}
			parsed, err := ParseTypedConfig(before)
			if err != nil || parsed != typed {
				t.Fatal("dense parse bridge", err)
			}
			normalized, err := (TypedConfig{Config: Config{Method: method, Dimensions: 3, Metric: metric}}).Normalize()
			if err != nil || normalized != typed {
				t.Fatal("dense normalize bridge", err)
			}
		}
	}
	c := config(t, 3, vector.L2)
	tc := TypedConfig{Config: c}
	legacy := newMemoryStore()
	typed := newTypedMemoryStore()
	a, b := NewState(), NewState()
	for i := int64(-32); i < 32; i++ {
		v := vec(t, float32(i%7), float32(i%11), float32(i+35))
		if err := Insert(context.Background(), legacy, &a, i, v, c); err != nil {
			t.Fatal(err)
		}
		if err := InsertValue(context.Background(), typed, &b, i, DenseValue(v), tc); err != nil {
			t.Fatal(err)
		}
	}
	if a != b {
		t.Fatal("dense typed graph state changed")
	}
	for id, n := range legacy.nodes {
		tn := typed.nodes[id]
		if tn.ID != n.ID || tn.RowID != n.RowID || tn.Level != n.Level || tn.Live != n.Live || !reflect.DeepEqual(tn.Value.Dense(), n.Vector) || !reflect.DeepEqual(tn.Neighbors, n.Neighbors) {
			t.Fatal("dense typed graph topology changed")
		}
		x, _ := EncodeAdjacency(n, c)
		y, _ := EncodeTypedAdjacency(tn, tc)
		if !bytes.Equal(x, y) {
			t.Fatal("dense GHA-v1 payload changed")
		}
	}
	q := vec(t, 2, 3, 20)
	want, stats, err := Search(context.Background(), legacy, a, q, c, 40, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	got, ts, err := SearchValue(context.Background(), typed, b, DenseValue(q), tc, 40, TypedLimits{})
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatal("dense typed candidates changed", err)
	}
	if ts.ComponentWork != int64(ts.DistanceEvaluations*3) || ts.PeakBytes != stats.PeakBytes+int64(ts.Visited)*128 {
		t.Fatal("dense typed retained/work accounting")
	}
	for _, metric := range []vector.MetricKind{vector.L2, vector.NegativeIP, vector.Cosine} {
		c := ivfConfig(t, 3, 3, metric)
		tc := TypedConfig{Config: c}
		samples := make([]Sample, 12)
		typedSamples := make([]TypedSample, 12)
		for i := range samples {
			v := vec(t, float32(i%4)+1, float32(i%5), float32(i+1))
			samples[i] = Sample{int64(i), v}
			typedSamples[i] = TypedSample{RowID: int64(i), Value: DenseValue(v)}
		}
		old, _, ready, err := Train(context.Background(), samples, c)
		if err != nil || !ready {
			t.Fatal(err)
		}
		got, _, ready, err := TrainValues(context.Background(), typedSamples, tc)
		if err != nil || !ready || len(got) != len(old) {
			t.Fatal(err)
		}
		for i, center := range old {
			if got[i].ID != center.ID || !reflect.DeepEqual(CentroidValue(got[i]).Dense(), center.Vector) {
				t.Fatal("dense typed centroid changed")
			}
		}
	}
}
