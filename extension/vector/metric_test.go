package vector_test

import (
	"errors"
	"math"
	"math/big"
	"math/rand/v2"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

// referenceMetrics sums exact rational values of float32 components, then uses
// 768-bit arithmetic for roots and quotients. It shares no summation algorithm
// with production, and 768 bits exceeds the float32 squared exponent spread.
func referenceMetrics(a, b vector.Vector) map[string]float64 {
	av, bv := a.Values(), b.Values()
	dot, aa, bb, squared, l1 := new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat)
	for i, x := range av {
		f := new(big.Rat).SetFloat64(float64(x))
		g := new(big.Rat).SetFloat64(float64(bv[i]))
		dot.Add(dot, new(big.Rat).Mul(f, g))
		aa.Add(aa, new(big.Rat).Mul(f, f))
		bb.Add(bb, new(big.Rat).Mul(g, g))
		d := new(big.Rat).Sub(f, g)
		squared.Add(squared, new(big.Rat).Mul(d, d))
		l1.Add(l1, new(big.Rat).Abs(d))
	}
	bf := func(r *big.Rat) *big.Float { return new(big.Float).SetPrec(768).SetRat(r) }
	f64 := func(f *big.Float) float64 { v, _ := f.Float64(); return v }
	dot64, _ := dot.Float64()
	l164, _ := l1.Float64()
	result := map[string]float64{"inner_product": dot64, "negative_inner_product": -dot64, "l1": l164, "l2": f64(new(big.Float).SetPrec(768).Sqrt(bf(squared))), "norm": f64(new(big.Float).SetPrec(768).Sqrt(bf(aa)))}
	if aa.Sign() != 0 && bb.Sign() != 0 {
		den := new(big.Float).SetPrec(768).Sqrt(bf(new(big.Rat).Mul(aa, bb)))
		sim := new(big.Float).SetPrec(768).Quo(bf(dot), den)
		result["cosine"] = f64(new(big.Float).SetPrec(768).Sub(new(big.Float).SetPrec(768).SetInt64(1), sim))
	}
	return result
}

func closeFloat(a, b, relative, absolute float64) bool {
	return math.Abs(a-b) <= absolute+relative*math.Max(math.Abs(a), math.Abs(b))
}

func TestVectorMetricsIndependentReference(t *testing.T) {
	r := rand.New(rand.NewPCG(42, 17))
	for trial := 0; trial < 240; trial++ {
		n := 1 + r.IntN(40)
		av, bv := make([]float32, n), make([]float32, n)
		for i := range av {
			av[i] = float32(math.Ldexp(r.Float64()*2-1, r.IntN(240)-120))
			bv[i] = float32(math.Ldexp(r.Float64()*2-1, r.IntN(240)-120))
		}
		a, _ := vector.New(av)
		b, _ := vector.New(bv)
		want := referenceMetrics(a, b)
		for _, m := range []vector.MetricKind{vector.L2, vector.L1, vector.Cosine, vector.NegativeIP} {
			got, err := m.Distance(a, b)
			if err != nil || !closeFloat(got, want[m.Name()], 6e-15, 2e-15) {
				t.Fatalf("trial %d metric %s: %.17g want %.17g err %v", trial, m, got, want[m.Name()], err)
			}
			reverse, err := m.Distance(b, a)
			if err != nil || got != reverse {
				t.Fatalf("symmetry %s: %g %g %v", m, got, reverse, err)
			}
			if m == vector.Cosine && (got < 0 || got > 2) {
				t.Fatal("cosine range", got)
			}
			if (m == vector.L1 || m == vector.L2) && got < 0 {
				t.Fatal("negative distance", got)
			}
		}
		norm, err := vector.Norm(a)
		if err != nil || !closeFloat(norm, want["norm"], 6e-15, 0) {
			t.Fatal("norm", norm, want["norm"], err)
		}
		ip, err := vector.InnerProduct(a, b)
		if err != nil || !closeFloat(ip, want["inner_product"], 6e-15, 0) {
			t.Fatal("inner product", ip, want["inner_product"], err)
		}
	}
}

func TestVectorMetricsExtremaAndCancellation(t *testing.T) {
	for _, s := range []string{"[3e38]", "[1e-45]", "[-3e38]", "[-1e-45]"} {
		a := must(t, s)
		negative := a.Values()
		for i := range negative {
			negative[i] = -negative[i]
		}
		b, _ := vector.New(negative)
		ref := referenceMetrics(a, b)
		for _, m := range []vector.MetricKind{vector.L2, vector.L1, vector.Cosine, vector.NegativeIP} {
			d, err := m.Distance(a, b)
			if err != nil || math.IsNaN(d) || math.IsInf(d, 0) || !closeFloat(d, ref[m.Name()], 3e-15, 0) {
				t.Fatal(s, m, d, ref[m.Name()], err)
			}
		}
		if d, err := vector.CosineDistance(a, a); err != nil || d != 0 {
			t.Fatal("self cosine", s, d, err)
		}
	}
	a := must(t, "[1e30,1,-1e30]")
	b := must(t, "[1,1,1]")
	if got, err := vector.InnerProduct(a, b); err != nil || got != 1 {
		t.Fatal("compensation", got, err)
	}
	// More than upstream's ANN limit is valid for every exact primitive.
	av, bv := make([]float32, vector.MaxDimensions), make([]float32, vector.MaxDimensions)
	for i := range av {
		av[i] = math.MaxFloat32
		bv[i] = -math.MaxFloat32
	}
	a, _ = vector.New(av)
	b, _ = vector.New(bv)
	for _, m := range []vector.MetricKind{vector.L2, vector.L1, vector.Cosine, vector.NegativeIP} {
		d, err := m.Distance(a, b)
		if err != nil || math.IsNaN(d) || math.IsInf(d, 0) {
			t.Fatal("extreme full dimensions", m, d, err)
		}
	}
	if d, err := vector.Norm(a); err != nil || math.IsInf(d, 0) || d == 0 {
		t.Fatal(d, err)
	}
}

func TestVectorMetricDomains(t *testing.T) {
	zero := must(t, "[0,-0]")
	a := must(t, "[1,2]")
	b := must(t, "[1]")
	for _, fn := range []func(vector.Vector, vector.Vector) (float64, error){vector.L2Distance, vector.L1Distance, vector.InnerProduct, vector.NegativeInnerProduct, vector.CosineDistance} {
		if _, err := fn(a, b); !errors.Is(err, vector.ErrDimensions) {
			t.Fatal(err)
		}
		if _, err := fn(vector.Vector{}, a); !errors.Is(err, vector.ErrInvalid) {
			t.Fatal(err)
		}
		if _, err := fn(a, vector.Vector{}); !errors.Is(err, vector.ErrInvalid) {
			t.Fatal(err)
		}
	}
	if n, err := vector.Norm(zero); err != nil || n != 0 {
		t.Fatal(n, err)
	}
	if _, err := vector.Norm(vector.Vector{}); !errors.Is(err, vector.ErrInvalid) {
		t.Fatal(err)
	}
	for _, pair := range [][2]vector.Vector{{zero, a}, {a, zero}, {zero, zero}} {
		if _, err := vector.CosineDistance(pair[0], pair[1]); !errors.Is(err, vector.ErrZeroNorm) {
			t.Fatal(err)
		}
	}
	if _, err := vector.MetricKind("hamming").Distance(a, a); !errors.Is(err, vector.ErrMetric) {
		t.Fatal(err)
	}
	if d, err := vector.NegativeInnerProduct(a, a); err != nil || d != -5 {
		t.Fatal(d, err)
	}
}
