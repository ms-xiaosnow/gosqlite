package vector_test

import (
	"errors"
	"math"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

// FuzzHalfRounding compares the production scale/round algorithm with nearest
// neighbor search over independently computed IEEE-754 half values. Only
// bounded in-memory scalar data is used; no external engine or database I/O.
func FuzzHalfRounding(f *testing.F) {
	for _, bits := range []uint32{0, 0x80000000, 1, 0x80000001, 0x3f801000, 0x477ff000, 0x7f7fffff, 0x7f800000, 0x7fffffff} {
		f.Add(bits)
	}
	value := func(raw int) float64 {
		exp := raw >> 10
		mant := raw & 1023
		if exp == 0 {
			return math.Ldexp(float64(mant), -24)
		}
		return math.Ldexp(float64(1024+mant), exp-25)
	}
	f.Fuzz(func(t *testing.T, bits uint32) {
		x := math.Float32frombits(bits)
		v, err := vector.NewHalf([]float32{x})
		magnitude := math.Abs(float64(x))
		if math.IsNaN(magnitude) || math.IsInf(magnitude, 0) || magnitude >= 65520 {
			if !errors.Is(err, vector.ErrNonFinite) || v.Valid() {
				t.Fatal(bits, v, err)
			}
			return
		}
		if err != nil {
			t.Fatal(bits, err)
		}
		lo, hi := 0, 0x7bff
		for lo < hi {
			mid := (lo + hi) / 2
			if value(mid) < magnitude {
				lo = mid + 1
			} else {
				hi = mid
			}
		}
		want := lo
		if lo > 0 {
			above, below := math.Abs(value(lo)-magnitude), math.Abs(magnitude-value(lo-1))
			if below < above || (below == above && (lo-1)&1 == 0) {
				want = lo - 1
			}
		}
		want |= int(bits>>16) & 0x8000
		if v.Bits()[0] != uint16(want) {
			t.Fatalf("float32 %08x: got %04x want %04x", bits, v.Bits()[0], want)
		}
	})
}
