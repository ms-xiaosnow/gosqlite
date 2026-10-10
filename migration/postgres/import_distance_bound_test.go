package postgres

import (
	"math"
	"math/big"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

// Pure-memory certification against an exact rational evaluation of the bound
// itself. This supplements unchanged independent native/import regressions.
func TestImportDistanceIPBoundRoundsOutward(t *testing.T) {
	for _, n := range []int{1, 3, 17, 16000} {
		a, b := make([]float32, n), make([]float32, n)
		for i := range a {
			a[i] = math.Float32frombits(uint32((i*47)%255)<<23 | uint32((i*104729+1)&0x7fffff))
			b[i] = math.Float32frombits(uint32((i*71+11)%255)<<23 | uint32((i*65537+3)&0x7fffff))
			if i%2 == 0 {
				b[i] = -b[i]
			}
		}
		ref := independentReviewDistance(a, b, vector.NegativeIP)
		actual := importDistanceErrorBound(a, b, vector.NegativeIP, ref)
		one := new(big.Rat).SetInt64(1)
		unit := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 53))
		nu := new(big.Rat).Mul(new(big.Rat).SetInt64(int64(n)), unit)
		gamma := new(big.Rat).Quo(nu, new(big.Rat).Sub(one, nu))
		scale := new(big.Rat)
		for i, x := range a {
			product := new(big.Rat).Mul(new(big.Rat).SetFloat64(float64(x)), new(big.Rat).SetFloat64(float64(b[i])))
			scale.Add(scale, new(big.Rat).Abs(product))
		}
		correction := new(big.Rat).Mul(gamma, gamma)
		correction.Mul(correction, scale)
		correction.Mul(correction, new(big.Rat).Add(one, unit))
		final := new(big.Rat).Mul(new(big.Rat).SetInt64(4), unit)
		final.Mul(final, new(big.Rat).SetFloat64(math.Abs(ref)))
		exact := new(big.Rat).Add(final, correction)
		rounded := new(big.Rat).SetFloat64(actual)
		if rounded == nil || rounded.Cmp(exact) < 0 {
			t.Fatalf("n=%d bound rounded below exact formula: %g", n, actual)
		}
		// The outward machinery should cost rounding slack, not orders of magnitude.
		twice := new(big.Rat).Mul(new(big.Rat).SetInt64(2), exact)
		if rounded.Cmp(twice) > 0 {
			t.Fatalf("n=%d unexpectedly loose bound: %g", n, actual)
		}
	}
	if bound := importDistanceErrorBound([]float32{0, 0}, []float32{1, -1}, vector.NegativeIP, 0); bound != 0 {
		t.Fatal("all-zero products lost exact-zero bound", bound)
	}
}
