package vector_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

// Review oracles below do not call dense metrics: an implementation shared by
// dense and extended values must not be able to make both sides agree wrongly.
func TestExtendedReviewIndependentMetricArithmetic(t *testing.T) {
	var points [][3]int64
	for _, a := range []int64{-2, 0, 3} {
		for _, b := range []int64{-2, 0, 3} {
			for _, c := range []int64{-2, 0, 3} {
				points = append(points, [3]int64{a, b, c})
			}
		}
	}
	for _, mode := range []struct {
		name  string
		half  bool
		scale float64
	}{
		{"half-subnormal", true, 0x1p-24},
		{"half-unit", true, 1},
		{"half-large", true, 0x1p14},
		{"sparse-subnormal", false, 0x1p-149},
		{"sparse-unit", false, 1},
		{"sparse-large", false, 0x1p124},
	} {
		t.Run(mode.name, func(t *testing.T) {
			half := make([]vector.HalfVector, len(points))
			sparse := make([]vector.SparseVector, len(points))
			for i, p := range points {
				values := make([]float32, 3)
				entries := make([]vector.SparseEntry, 3)
				indices := [3]uint32{0, 511, vector.MaxSparseDimensions - 1}
				for j := range p {
					values[j] = float32(float64(p[j]) * mode.scale)
					// Descending construction order must be canonicalized without
					// changing which coordinates participate in multiplication.
					entries[2-j] = vector.SparseEntry{Index: indices[j], Value: values[j]}
				}
				var err error
				if mode.half {
					half[i], err = vector.NewHalf(values)
				} else {
					sparse[i], err = vector.NewSparse(vector.MaxSparseDimensions, entries)
				}
				if err != nil {
					t.Fatal(err)
				}
			}
			for i, p := range points {
				for j, q := range points {
					var l1, l2, dot, aa, bb int64
					for k := range p {
						d := p[k] - q[k]
						l2 += d * d
						if d < 0 {
							d = -d
						}
						l1 += d
						dot += p[k] * q[k]
						aa += p[k] * p[k]
						bb += q[k] * q[k]
					}
					for _, metric := range []vector.MetricKind{vector.L1, vector.L2, vector.NegativeIP, vector.Cosine} {
						var got float64
						var err error
						if mode.half {
							got, err = metric.DistanceHalf(half[i], half[j])
						} else {
							got, err = metric.DistanceSparse(sparse[i], sparse[j])
						}
						if metric == vector.Cosine && (aa == 0 || bb == 0) {
							if got != 0 || !errors.Is(err, vector.ErrZeroNorm) {
								t.Fatalf("zero cosine: %g, %v", got, err)
							}
							continue
						}
						var want float64
						switch metric {
						case vector.L1:
							want = float64(l1) * mode.scale
						case vector.L2:
							want = math.Sqrt(float64(l2)) * mode.scale
						case vector.NegativeIP:
							want = -float64(dot) * mode.scale * mode.scale
						case vector.Cosine:
							want = 1 - float64(dot)/math.Sqrt(float64(aa*bb))
						}
						// Power-of-two scaling leaves these integer calculations
						// exact; only sqrt/division can introduce a rounding step.
						ulp := math.Abs(math.Nextafter(want, math.Inf(1)) - want)
						if err != nil || math.IsNaN(got) || math.IsInf(got, 0) || math.Abs(got-want) > 2*ulp {
							t.Fatalf("%v %v %v: got %.17g (%v), want %.17g", metric, p, q, got, err, want)
						}
					}
				}
			}
		})
	}
}

// Decode a finite binary16 value by assembling binary32 fields, independently
// of the production floating-point scaling implementation.
func extendedReviewWiden(bits uint16) float32 {
	sign := uint32(bits&0x8000) << 16
	exponent, fraction := uint32(bits>>10)&31, uint32(bits&1023)
	if exponent == 0 {
		if fraction == 0 {
			return math.Float32frombits(sign)
		}
		exponent = 113
		for fraction&1024 == 0 {
			fraction <<= 1
			exponent--
		}
		fraction &= 1023
	} else {
		exponent += 112
	}
	return math.Float32frombits(sign | exponent<<23 | fraction<<13)
}

func TestExtendedReviewConversionReportAndComparison(t *testing.T) {
	// Around representable half values, binary32 neighbors cover every exponent,
	// both signs, signed zero and the subnormal/normal transition.
	var source []float32
	for _, h := range []uint16{0, 1, 2, 3, 0x03ff, 0x0400, 0x0401, 0x3555, 0x3bff, 0x3c00, 0x3c01, 0x7bff} {
		for _, sign := range []uint16{0, 0x8000} {
			x := extendedReviewWiden(h | sign)
			source = append(source, math.Nextafter32(x, float32(math.Inf(-1))), x, math.Nextafter32(x, float32(math.Inf(1))))
		}
	}
	dense, err := vector.New(source)
	if err != nil {
		t.Fatal(err)
	}
	half, report, err := vector.HalfFromVector(dense)
	if err != nil {
		t.Fatal(err)
	}
	var want vector.HalfConversion
	for i, h := range half.Bits() {
		x, y := source[i], extendedReviewWiden(h)
		if math.Float32bits(x) != math.Float32bits(y) {
			want.Changed++
		}
		diff := math.Abs(float64(x) - float64(y))
		want.MaxAbsoluteError = math.Max(want.MaxAbsoluteError, diff)
		if x != 0 {
			want.MaxRelativeError = math.Max(want.MaxRelativeError, diff/math.Abs(float64(x)))
		}
	}
	if report != want || report.Changed == 0 || report.MaxRelativeError != 1 {
		t.Fatalf("report: %+v, want %+v", report, want)
	}
	for _, overflowAt := range []int{0, len(source) / 2, len(source) - 1} {
		input := append([]float32(nil), source...)
		input[overflowAt] = 65520
		v, _ := vector.New(input)
		h, r, err := vector.HalfFromVector(v)
		if !errors.Is(err, vector.ErrNonFinite) || h.Valid() || r != (vector.HalfConversion{}) {
			t.Fatalf("partial conversion at %d: %+v %v", overflowAt, r, err)
		}
	}
	// Ranking is on the actual stored representation: a half conversion can
	// collapse a positive dense component, and its strict-positive bit, to zero.
	tiny, _ := vector.New([]float32{math.SmallestNonzeroFloat32, float32(math.Copysign(0, -1)), -math.SmallestNonzeroFloat32})
	h, _, _ := vector.HalfFromVector(tiny)
	db, _ := vector.BinaryQuantize(tiny)
	hb, _ := vector.HalfBinaryQuantize(h)
	if db.String() != "100" || hb.String() != "000" {
		t.Fatal(db, hb)
	}
	if d, err := vector.HammingDistance(db, hb); d != 1 || err != nil {
		t.Fatal(d, err)
	}
	s, _ := vector.SparseFromVector(tiny)
	back, err := s.Dense()
	if err != nil || s.NNZ() != 2 {
		t.Fatal(s, err)
	}
	for i, x := range back.Values() {
		wantBits := math.Float32bits(tiny.Values()[i])
		if i == 1 {
			wantBits = 0
		}
		if math.Float32bits(x) != wantBits {
			t.Fatalf("sparse round trip at %d", i)
		}
	}
}

type extendedReviewContext struct {
	context.Context
	calls, cancelAt int
}

func (c *extendedReviewContext) Err() error {
	c.calls++
	if c.cancelAt > 0 && c.calls >= c.cancelAt {
		return context.Canceled
	}
	return nil
}

func TestExtendedReviewCancellationBoundaries(t *testing.T) {
	type run struct {
		name      string
		positions int
		eval      func(context.Context) (float64, error)
	}
	var runs []run
	for _, n := range []int{1, 255, 256, 257, vector.MaxHalfDimensions} {
		values := make([]float32, n)
		for i := range values {
			values[i] = 1
		}
		h, _ := vector.NewHalf(values)
		for _, m := range []vector.MetricKind{vector.L1, vector.L2, vector.NegativeIP, vector.Cosine} {
			runs = append(runs, run{fmt.Sprintf("half-%d-%s", n, m), n, func(c context.Context) (float64, error) { return m.DistanceHalfContext(c, h, h) }})
		}
		runs = append(runs, run{fmt.Sprintf("half-%d-norm", n), n, func(c context.Context) (float64, error) { return vector.HalfNormContext(c, h) }})
	}
	for _, n := range []int{0, 1, 255, 256, 257, vector.MaxSparseNonzeros} {
		a, b := make([]vector.SparseEntry, n), make([]vector.SparseEntry, n)
		for i := range a {
			a[i] = vector.SparseEntry{Index: uint32(2 * i), Value: 1}
			b[i] = vector.SparseEntry{Index: uint32(2*i + 1), Value: 1}
		}
		x, _ := vector.NewSparse(vector.MaxSparseDimensions, a)
		y, _ := vector.NewSparse(vector.MaxSparseDimensions, b)
		for _, m := range []vector.MetricKind{vector.L1, vector.L2, vector.NegativeIP, vector.Cosine} {
			runs = append(runs, run{fmt.Sprintf("sparse-disjoint-%d-%s", n, m), 2 * n, func(c context.Context) (float64, error) { return m.DistanceSparseContext(c, x, y) }})
		}
		runs = append(runs, run{fmt.Sprintf("sparse-%d-norm", n), n, func(c context.Context) (float64, error) { return vector.SparseNormContext(c, x) }})
	}
	for _, n := range []int{0, 8, 2040, 2048, 2049, vector.MaxBitDimensions} {
		b, _ := vector.NewBit(n, make([]byte, (n+7)/8))
		for _, m := range []vector.BitMetricKind{vector.Hamming, vector.Jaccard} {
			runs = append(runs, run{fmt.Sprintf("bit-%d-%s", n, m), (n + 7) / 8, func(c context.Context) (float64, error) { return m.DistanceContext(c, b, b) }})
		}
	}
	for _, r := range runs {
		t.Run(r.name, func(t *testing.T) {
			checks := 2 + (r.positions+255)/256
			for at := 1; at <= checks; at++ {
				c := &extendedReviewContext{Context: context.Background(), cancelAt: at}
				got, err := r.eval(c)
				if got != 0 || !errors.Is(err, context.Canceled) || c.calls != at {
					t.Fatalf("probe %d/%d: %g, %v, calls=%d", at, checks, got, err, c.calls)
				}
			}
			c := &extendedReviewContext{Context: context.Background(), cancelAt: checks + 1}
			_, err := r.eval(c)
			if (err != nil && !errors.Is(err, vector.ErrZeroNorm)) || c.calls != checks {
				t.Fatalf("uncanceled: %v, calls=%d, want %d", err, c.calls, checks)
			}
			if _, err := r.eval(nil); !errors.Is(err, vector.ErrContext) {
				t.Fatal(err)
			}
			if allocs := testing.AllocsPerRun(10, func() {
				_, err := r.eval(context.Background())
				if err != nil && !errors.Is(err, vector.ErrZeroNorm) {
					panic(err)
				}
			}); allocs != 0 {
				t.Fatalf("metric allocated: %g", allocs)
			}
		})
	}
}

func TestExtendedReviewCodecCanonicalityAndPreflight(t *testing.T) {
	h, _ := vector.NewHalfBits([]uint16{0x8000, 1, 0x7bff})
	b, _ := vector.ParseBit("100000001")
	s, _ := vector.NewSparse(vector.MaxSparseDimensions, []vector.SparseEntry{{Index: 0, Value: math.SmallestNonzeroFloat32}, {Index: vector.MaxSparseDimensions - 1, Value: -math.MaxFloat32}})
	type codec struct {
		name   string
		value  interface{ MarshalBinary() ([]byte, error) }
		decode func([]byte) (bool, error)
	}
	for _, c := range []codec{
		{"half", h, func(b []byte) (bool, error) { v, e := vector.DecodeHalf(b); return v.Valid(), e }},
		{"bit", b, func(b []byte) (bool, error) { v, e := vector.DecodeBit(b); return v.Valid(), e }},
		{"sparse", s, func(b []byte) (bool, error) { v, e := vector.DecodeSparse(b); return v.Valid(), e }},
	} {
		t.Run(c.name, func(t *testing.T) {
			good, err := c.value.MarshalBinary()
			if err != nil {
				t.Fatal(err)
			}
			for at := 0; at < 8; at++ {
				for value := 0; value < 256; value++ {
					if byte(value) == good[at] {
						continue
					}
					mutated := bytes.Clone(good)
					mutated[at] = byte(value)
					if valid, err := c.decode(mutated); valid || !errors.Is(err, vector.ErrEncoding) {
						t.Fatalf("accepted header byte %d=%d: %v", at, value, err)
					}
				}
			}
		})
	}
	// Every final-byte bit pattern at every padding width, including aligned
	// byte lengths. Noncanonical encodings may not be silently normalized.
	for dims := 1; dims <= 16; dims++ {
		for last := 0; last < 256; last++ {
			packed := make([]byte, (dims+7)/8)
			packed[len(packed)-1] = byte(last)
			unused := (8 - dims%8) % 8
			canonical := last&((1<<unused)-1) == 0
			v, err := vector.NewBit(dims, packed)
			if !canonical {
				if v.Valid() || !errors.Is(err, vector.ErrEncoding) {
					t.Fatalf("padding dims=%d last=%x: %v", dims, last, err)
				}
				raw := make([]byte, vector.BitBinaryHeaderSize+len(packed))
				copy(raw, []byte{'G', 'S', 'B', 0, 1, 3, 1, 0})
				binary.BigEndian.PutUint32(raw[8:], uint32(dims))
				copy(raw[vector.BitBinaryHeaderSize:], packed)
				if decoded, err := vector.DecodeBit(raw); decoded.Valid() || !errors.Is(err, vector.ErrEncoding) {
					t.Fatalf("decoded noncanonical padding dims=%d last=%x: %v", dims, last, err)
				}
				continue
			}
			encoded, e := v.MarshalBinary()
			if err != nil || e != nil {
				t.Fatal(err, e)
			}
			back, e := vector.DecodeBit(encoded)
			if e != nil || !bytes.Equal(packed, back.Packed()) {
				t.Fatal(dims, last, e)
			}
		}
	}
	// A complete maximum-sized sparse payload with an invalid final value must
	// be rejected before allocating a destination entry slice.
	bad := make([]byte, vector.MaxSparseBinaryBytes)
	copy(bad, []byte{'G', 'S', 'S', 0, 1, 4, 1, 0})
	binary.BigEndian.PutUint32(bad[8:], vector.MaxSparseDimensions)
	binary.BigEndian.PutUint32(bad[12:], vector.MaxSparseNonzeros)
	for i := 0; i < vector.MaxSparseNonzeros; i++ {
		binary.BigEndian.PutUint32(bad[16+8*i:], uint32(i))
		binary.BigEndian.PutUint32(bad[20+8*i:], math.Float32bits(1))
	}
	binary.BigEndian.PutUint32(bad[len(bad)-4:], 0x80000000)
	if a := testing.AllocsPerRun(10, func() {
		v, e := vector.DecodeSparse(bad)
		if v.Valid() || !errors.Is(e, vector.ErrEncoding) {
			panic("accepted negative zero")
		}
	}); a != 0 {
		t.Fatalf("allocated before canonicality validation: %g", a)
	}
	oversize := make([]vector.SparseEntry, vector.MaxSparseNonzeros+1)
	if a := testing.AllocsPerRun(10, func() {
		v, e := vector.NewSparse(vector.MaxSparseDimensions, oversize)
		if v.Valid() || !errors.Is(e, vector.ErrLimit) {
			panic("accepted oversized support")
		}
	}); a != 0 {
		t.Fatalf("allocated before support-count validation: %g", a)
	}
}
