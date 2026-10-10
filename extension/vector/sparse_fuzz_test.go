package vector_test

import (
	"encoding/binary"
	"errors"
	"math"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

func FuzzSparseFloatBitsMetrics(f *testing.F) {
	f.Add([]byte{0, 0, 0, 0, 0x80, 0, 0, 0, 0, 0, 0, 1, 0xff, 0x7f, 0xff, 0xff})
	f.Add([]byte{0x7f, 0x80, 0, 0})
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 512 {
			data = data[:512]
		}
		dim := 1 + len(data)/8
		a, b := make([]float32, dim), make([]float32, dim)
		finite := true
		for i := 0; i+4 <= len(data); i += 4 {
			x := math.Float32frombits(binary.BigEndian.Uint32(data[i:]))
			if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
				finite = false
			}
			if i%8 == 0 {
				a[i/8] = x
			} else {
				b[i/8] = x
			}
		}
		x, xe := vector.New(a)
		y, ye := vector.New(b)
		if !finite {
			if xe == nil && ye == nil {
				t.Fatal("accepted nonfinite")
			}
			return
		}
		if xe != nil || ye != nil {
			t.Fatal(xe, ye)
		}
		s, err := vector.SparseFromVector(x)
		if err != nil {
			t.Fatal(err)
		}
		r, err := vector.SparseFromVector(y)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range []vector.MetricKind{vector.L1, vector.L2, vector.NegativeIP, vector.Cosine} {
			got, err := m.DistanceSparse(s, r)
			want, werr := m.Distance(x, y)
			if !errors.Is(err, werr) || got != want {
				t.Fatal(m, got, want, err, werr)
			}
		}
	})
}
