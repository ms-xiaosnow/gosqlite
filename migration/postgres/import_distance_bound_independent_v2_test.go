package postgres

import (
	"math"
	"math/big"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

// Pure-memory independent challenges of the v2 proof and outward arithmetic.
func TestIndependentImportDistanceV2GammaProofChallenges(t *testing.T) {
	baseA, baseB := independentReviewCancellationInputs()
	type pair struct {
		name string
		a, b []float32
	}
	cases := []pair{}
	for _, exponent := range []int{-80, 0, 80} {
		for _, sign := range []int{-1, 1} {
			a, b := make([]float32, len(baseA)), make([]float32, len(baseB))
			for i := range a {
				a[i] = float32(math.Ldexp(float64(baseA[i]), exponent))
				b[i] = float32(sign) * float32(math.Ldexp(float64(baseB[i]), exponent))
			}
			name := "scaled"
			if sign < 0 {
				name = "scaled-sign-flipped"
			}
			cases = append(cases, pair{name, a, b})
		}
	}
	reverseA, reverseB := append([]float32(nil), baseA...), append([]float32(nil), baseB...)
	for i, j := 0, len(reverseA)-1; i < j; i, j = i+1, j-1 {
		reverseA[i], reverseA[j] = reverseA[j], reverseA[i]
		reverseB[i], reverseB[j] = reverseB[j], reverseB[i]
	}
	cases = append(cases, pair{"reverse-order", reverseA, reverseB})
	for _, n := range []int{1, 2, 3, 127, 4095, 16000} {
		a, b := make([]float32, n), make([]float32, n)
		for i := range a {
			a[i] = math.MaxFloat32
			b[i] = math.MaxFloat32
			if i%2 == 0 {
				b[i] = -b[i]
			}
		}
		if n > 2 {
			a[n/2], b[n/2] = math.SmallestNonzeroFloat32, math.SmallestNonzeroFloat32
		}
		cases = append(cases, pair{"full-exponent-span", a, b})
	}
	rat := func(x float64) *big.Rat { return new(big.Rat).SetFloat64(x) }
	unit := new(big.Rat).SetFrac(big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 53))
	one := new(big.Rat).SetInt64(1)
	for caseIndex, tc := range cases {
		n := len(tc.a)
		sum, correction := 0.0, 0.0
		exactSum, scale, residualSum, residualAbs := new(big.Rat), new(big.Rat), new(big.Rat), new(big.Rat)
		for i, x := range tc.a {
			// Exact source products are rationally computed, not inferred from native output.
			exactProduct := new(big.Rat).Mul(rat(float64(x)), rat(float64(tc.b[i])))
			product := float64(x) * float64(tc.b[i])
			if rat(product).Cmp(exactProduct) != 0 {
				t.Fatal("binary32 product not exactly widened")
			}
			exactSum.Add(exactSum, exactProduct)
			scale.Add(scale, new(big.Rat).Abs(exactProduct))
			next := sum + product
			exactResidual := new(big.Rat).Sub(new(big.Rat).Add(rat(sum), exactProduct), rat(next))
			var residual float64
			if math.Abs(sum) >= math.Abs(product) {
				residual = (sum - next) + product
			} else {
				residual = (product - next) + sum
			}
			if rat(residual).Cmp(exactResidual) != 0 {
				t.Fatalf("case=%d index=%d residual not exact", caseIndex, i)
			}
			residualSum.Add(residualSum, exactResidual)
			residualAbs.Add(residualAbs, new(big.Rat).Abs(exactResidual))
			correction += residual
			sum = next
		}
		nu := new(big.Rat).Mul(new(big.Rat).SetInt64(int64(n)), unit)
		gamma := new(big.Rat).Quo(nu, new(big.Rat).Sub(one, nu))
		residualCap := new(big.Rat).Mul(gamma, scale)
		if residualAbs.Cmp(residualCap) > 0 {
			t.Fatalf("case=%d residual absolute sum violates gamma*S", caseIndex)
		}
		correctionError := new(big.Rat).Abs(new(big.Rat).Sub(rat(correction), residualSum))
		if correctionError.Cmp(new(big.Rat).Mul(gamma, residualAbs)) > 0 {
			t.Fatalf("case=%d correction violates ordinary summation bound", caseIndex)
		}
		exactNegative := new(big.Rat).Neg(exactSum)
		f := new(big.Float).SetPrec(1024).SetRat(exactNegative)
		reference, _ := f.Float64()
		av, _ := vector.New(tc.a)
		bv, _ := vector.New(tc.b)
		native, err := vector.NegativeInnerProduct(av, bv)
		if err != nil {
			t.Fatal(err)
		}
		if native != -(sum + correction) {
			t.Fatalf("case=%d proof simulation differs from native", caseIndex)
		}
		if importDistanceReference(tc.a, tc.b, vector.NegativeIP) != reference {
			t.Fatalf("case=%d source reference differs from exact rational sum", caseIndex)
		}
		bound := importDistanceErrorBound(tc.a, tc.b, vector.NegativeIP, reference)
		gammaSquared := new(big.Rat).Mul(gamma, gamma)
		second := new(big.Rat).Mul(new(big.Rat).Add(one, unit), new(big.Rat).Mul(gammaSquared, scale))
		first := new(big.Rat).Mul(new(big.Rat).Mul(new(big.Rat).SetInt64(4), unit), rat(math.Abs(reference)))
		formula := new(big.Rat).Add(first, second)
		if rat(bound).Cmp(formula) < 0 {
			t.Fatalf("case=%d outward arithmetic rounds inward", caseIndex)
		}
		exactReportedError := new(big.Rat).Abs(new(big.Rat).Sub(rat(native), rat(reference)))
		if exactReportedError.Cmp(rat(bound)) > 0 {
			t.Fatalf("case=%d %s valid native IP violates bound", caseIndex, tc.name)
		}
		// A fabricated distance clearly outside the widened bound must still fail.
		fake := reference + 4*bound
		refs := map[int64]importDistanceReferenceValue{1: {reference, bound}}
		if fake != reference {
			if _, err := checkImportDistanceHits([]vector.Hit{{RowID: 1, Distance: fake}}, refs); err == nil {
				t.Fatalf("case=%d fabricated beyond-bound value accepted", caseIndex)
			}
		}
	}
	t.Logf("checked %d signed/scaled/reordered/full-exponent cases and exact residual/correction/formula inequalities", len(cases))
}
