package vector_test

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"math"
	"math/rand"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

func mustSparseM9(t testing.TB, s string) vector.SparseVector {
	t.Helper()
	v, err := vector.ParseSparse(s)
	if err != nil {
		t.Fatal(s, err)
	}
	return v
}

func TestSparseParseCanonicalAndEdges(t *testing.T) {
	for _, c := range []struct{ input, want string }{
		{" { 3 : 2. , 1 : 1e0 } / +5 ", "{1:1,3:2}/5"},
		{"{1:0,1:-0,1:2,2:0}/2", "{1:2}/2"},
		{"{1:0,1:-0}/1", "{}/1"},
		{"{}/1000000000", "{}/1000000000"},
		{"{1000000000:1}/1000000000", "{1000000000:1}/1000000000"},
		{"{1:1.000000059604644775390626}/1", "{1:1.0000001}/1"},
		{"{1:-0.000e-999,2:0e+999}/2", "{}/2"},
		{"{1:1e-45,2:-1e-45}/2", "{1:1e-45,2:-1e-45}/2"},
	} {
		v := mustSparseM9(t, c.input)
		if v.String() != c.want {
			t.Fatal(c.input, v.String(), c.want)
		}
	}
	for _, s := range []string{"{1:1,1:2}/2", "{1:1,2:2,1:-1}/2", "{0:0}/2", "{-1:0}/2", "{3:0}/2", "{3:-0}/2", "{2147483648:0}/2", "{0:1}/1", "{2:1}/1"} {
		if v, err := vector.ParseSparse(s); !errors.Is(err, vector.ErrSparseIndex) || v.Valid() {
			t.Fatal(s, v, err)
		}
	}
	for _, s := range []string{"{1:1e-46}/1", "{1:-1e-999}/1", "{1:0.00001e-999}/1"} {
		if _, err := vector.ParseSparse(s); !errors.Is(err, vector.ErrSparseUnderflow) {
			t.Fatal(s, err)
		}
	}
	for _, s := range []string{"{1:NaN}/1", "{1:Infinity}/1", "{1:-Infinity}/1", "{1:4e38}/1"} {
		if _, err := vector.ParseSparse(s); !errors.Is(err, vector.ErrNonFinite) {
			t.Fatal(s, err)
		}
	}
	for _, s := range []string{"", "{}", "{}/", "{1:}/1", "{1:1,}/1", "{1:0x1p0}/1", "{1:1_0}/1", "{1:1}/1x", "{1:1}1", "{1 1}/1", "{1:1}/1\x00", "{1:1}/1\u00a0"} {
		if v, err := vector.ParseSparse(s); err == nil || v.Valid() {
			t.Fatal(s, v, err)
		}
	}
	for _, s := range []string{"{}/0", "{}/-1", "{}/1000000001", "{}/9223372036854775808"} {
		if _, err := vector.ParseSparse(s); !errors.Is(err, vector.ErrDimensions) {
			t.Fatal(s, err)
		}
	}
	if _, err := vector.ParseSparse(strings.Repeat(" ", vector.MaxTextBytes+1)); !errors.Is(err, vector.ErrLimit) {
		t.Fatal(err)
	}
	if _, err := vector.ParseSparse("{1:" + strings.Repeat("1", 129) + "}/1"); !errors.Is(err, vector.ErrLimit) {
		t.Fatal(err)
	}
	if _, err := vector.ParseSparse("{" + strings.Repeat("1", 129) + ":1}/1"); !errors.Is(err, vector.ErrLimit) {
		t.Fatal(err)
	}
	if _, err := vector.ParseSparse("{" + strings.Repeat("1:0,", 16000) + "1:0}/1"); !errors.Is(err, vector.ErrLimit) {
		t.Fatal(err)
	}
}

func TestSparseImmutabilityAndConversions(t *testing.T) {
	entries := []vector.SparseEntry{{Index: 4, Value: 2}, {Index: 1, Value: -3}, {Index: 2, Value: float32(math.Copysign(0, -1))}}
	v, err := vector.NewSparse(5, entries)
	if err != nil || v.Dims() != 5 || v.NNZ() != 2 || v.String() != "{2:-3,5:2}/5" {
		t.Fatal(v, err)
	}
	want := v.String()
	entries[0].Value = 50
	out := v.Entries()
	out[0].Index = 0
	out[0].Value = 90
	if v.String() != want {
		t.Fatal("storage escaped")
	}
	dense, err := v.Dense()
	if err != nil || !sameBits(dense, must(t, "[0,-3,0,0,2]")) {
		t.Fatal(dense, err)
	}
	back, err := vector.SparseFromVector(dense)
	if err != nil || !reflect.DeepEqual(back.Entries(), v.Entries()) {
		t.Fatal(back, err)
	}
	d := must(t, "[-0,0,1e-45,-1e-45]")
	s, err := vector.SparseFromVector(d)
	if err != nil || s.NNZ() != 2 {
		t.Fatal(s, err)
	}
	wide, err := s.Dense()
	if err != nil || math.Signbit(float64(wide.Values()[0])) || math.Float32bits(wide.Values()[2]) != 1 || math.Float32bits(wide.Values()[3]) != 0x80000001 {
		t.Fatal(wide, err)
	}
	for _, dim := range []int{0, -1, 1000000001} {
		if _, err := vector.NewSparse(dim, nil); !errors.Is(err, vector.ErrDimensions) {
			t.Fatal(dim, err)
		}
	}
	for _, entry := range []vector.SparseEntry{{Index: 5, Value: 0}, {Index: math.MaxUint32, Value: 0}, {Index: 5, Value: 1}} {
		if _, err := vector.NewSparse(5, []vector.SparseEntry{entry}); !errors.Is(err, vector.ErrSparseIndex) {
			t.Fatal(entry, err)
		}
	}
	for _, value := range []float32{float32(math.NaN()), float32(math.Inf(1)), float32(math.Inf(-1))} {
		if _, err := vector.NewSparse(1, []vector.SparseEntry{{Value: value}}); !errors.Is(err, vector.ErrNonFinite) {
			t.Fatal(value, err)
		}
	}
	if _, err := vector.NewSparse(1, make([]vector.SparseEntry, 16001)); !errors.Is(err, vector.ErrLimit) {
		t.Fatal(err)
	}
	if _, err := vector.SparseFromVector(vector.Vector{}); !errors.Is(err, vector.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := (vector.SparseVector{}).Dense(); !errors.Is(err, vector.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := vector.ParseSparseDims("{}/5", 0); !errors.Is(err, vector.ErrDimensions) {
		t.Fatal(err)
	}
	if _, err := vector.ParseSparseDims("{}/5", 4); !errors.Is(err, vector.ErrDimensions) {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				if v.String() != want {
					t.Error("concurrent read")
				}
				_, _ = vector.SparseL2Distance(v, v)
			}
		}()
	}
	wg.Wait()
}

func TestSparseBillionDimensionBounded(t *testing.T) {
	a := mustSparseM9(t, "{1:3,1000000000:4}/1000000000")
	b := mustSparseM9(t, "{}/1000000000")
	if got, err := vector.SparseL2Distance(a, b); got != 5 || err != nil {
		t.Fatal(got, err)
	}
	if got, err := vector.SparseL1Distance(a, b); got != 7 || err != nil {
		t.Fatal(got, err)
	}
	if got, err := vector.SparseInnerProduct(a, b); got != 0 || err != nil {
		t.Fatal(got, err)
	}
	if got, err := vector.SparseNorm(a); got != 5 || err != nil {
		t.Fatal(got, err)
	}
	if _, err := vector.SparseCosineDistance(a, b); !errors.Is(err, vector.ErrZeroNorm) {
		t.Fatal(err)
	}
	if _, err := a.Dense(); !errors.Is(err, vector.ErrLimit) {
		t.Fatal(err)
	}
	if allocs := testing.AllocsPerRun(100, func() {
		if _, err := vector.SparseL2Distance(a, b); err != nil {
			panic(err)
		}
	}); allocs != 0 {
		t.Fatal("sparse metric allocated", allocs)
	}
	if allocs := testing.AllocsPerRun(100, func() {
		if _, err := a.Dense(); !errors.Is(err, vector.ErrLimit) {
			panic("dense limit")
		}
	}); allocs > 3 {
		t.Fatal("dimension preflight allocated output", allocs)
	}
	large := make([]vector.SparseEntry, 16000)
	for i := range large {
		large[i] = vector.SparseEntry{Index: uint32(15999 - i), Value: 1}
	}
	v, err := vector.NewSparse(1000000000, large)
	if err != nil || v.NNZ() != 16000 {
		t.Fatal(v, err)
	}
	encoded, err := v.MarshalBinary()
	if err != nil || len(encoded) != vector.MaxSparseBinaryBytes {
		t.Fatal(len(encoded), err)
	}
	back, err := vector.DecodeSparse(encoded)
	if err != nil || back.NNZ() != 16000 {
		t.Fatal(back, err)
	}
}

func TestSparseCodecAndNull(t *testing.T) {
	v := mustSparseM9(t, "{1:1e-45,3:-3,5:2}/5")
	good, _ := v.MarshalBinary()
	if len(good) != 40 || string(good[:4]) != "GSS\x00" || binary.BigEndian.Uint32(good[12:]) != 3 || binary.BigEndian.Uint32(good[16:]) != 0 || binary.BigEndian.Uint32(good[20:]) != 1 {
		t.Fatal("wire layout", good)
	}
	for i := 0; i < 8; i++ {
		b := append([]byte(nil), good...)
		b[i] ^= 0xff
		if _, err := vector.DecodeSparse(b); !errors.Is(err, vector.ErrEncoding) {
			t.Fatal(i, err)
		}
	}
	for _, c := range []struct {
		at    int
		value uint32
	}{
		{8, 0}, {8, 1000000001}, {8, math.MaxUint32}, {12, math.MaxUint32}, {12, 16001}, {12, 6},
		{16, 5}, {24, 0}, {24, math.MaxUint32}, {32, 1}, {20, 0}, {20, 0x80000000}, {20, 0x7f800000}, {20, 0x7fc00001},
	} {
		b := append([]byte(nil), good...)
		binary.BigEndian.PutUint32(b[c.at:], c.value)
		if got, err := vector.DecodeSparse(b); !errors.Is(err, vector.ErrEncoding) || got.Valid() {
			t.Fatal(c, got, err)
		}
	}
	for i := 0; i < len(good); i++ {
		if _, err := vector.DecodeSparse(good[:i]); !errors.Is(err, vector.ErrEncoding) {
			t.Fatal(i, err)
		}
	}
	if _, err := vector.DecodeSparse(append(good, 0)); !errors.Is(err, vector.ErrEncoding) {
		t.Fatal(err)
	}
	if _, err := vector.Decode(good); !errors.Is(err, vector.ErrEncoding) {
		t.Fatal(err)
	}
	if _, err := vector.DecodeHalf(good); !errors.Is(err, vector.ErrEncoding) {
		t.Fatal(err)
	}
	dense, _ := must(t, "[1]").MarshalBinary()
	if _, err := vector.DecodeSparse(dense); !errors.Is(err, vector.ErrEncoding) {
		t.Fatal(err)
	}
	zero := mustSparseM9(t, "{}/5")
	zeroBytes, _ := zero.MarshalBinary()
	if len(zeroBytes) != 16 {
		t.Fatal(len(zeroBytes))
	}
	if z, err := vector.DecodeSparse(zeroBytes); err != nil || !z.Valid() || z.NNZ() != 0 {
		t.Fatal(z, err)
	}
	var scan vector.NullSparseVector
	if err := scan.Scan(v.String()); err != nil {
		t.Fatal(err)
	}
	for _, src := range []any{1, []float32{1}, []byte(v.String()), "{1:NaN}/1", vector.SparseVector{}} {
		if err := scan.Scan(src); err == nil || !scan.Valid || scan.SparseVector.String() != v.String() {
			t.Fatal(src, scan, err)
		}
	}
	if err := scan.Scan(good); err != nil {
		t.Fatal(err)
	}
	good[20] = 0xff
	if scan.SparseVector.String() != v.String() {
		t.Fatal("borrowed buffer")
	}
	if _, err := scan.Value(); err != nil {
		t.Fatal(err)
	}
	if err := scan.Scan(nil); err != nil || scan.Valid || scan.SparseVector.Valid() {
		t.Fatal(scan, err)
	}
	if x, err := scan.Value(); x != nil || err != nil {
		t.Fatal(x, err)
	}
	if _, err := (vector.NullSparseVector{Valid: true}).Value(); !errors.Is(err, vector.ErrInvalid) {
		t.Fatal(err)
	}
	if err := (*vector.NullSparseVector)(nil).Scan(nil); !errors.Is(err, vector.ErrInvalid) {
		t.Fatal(err)
	}
}

func TestSparseMetricsAgainstDense(t *testing.T) {
	rng := rand.New(rand.NewSource(904))
	for round := 0; round < 1000; round++ {
		dim := 1 + rng.Intn(96)
		a, b := make([]float32, dim), make([]float32, dim)
		for i := range a {
			if rng.Intn(3) == 0 {
				a[i] = float32(rng.NormFloat64() * 10)
			}
			if rng.Intn(3) == 0 {
				b[i] = float32(rng.NormFloat64() * 10)
			}
		}
		xd, _ := vector.New(a)
		yd, _ := vector.New(b)
		x, _ := vector.SparseFromVector(xd)
		y, _ := vector.SparseFromVector(yd)
		for _, m := range []vector.MetricKind{vector.L1, vector.L2, vector.NegativeIP, vector.Cosine} {
			got, err := m.DistanceSparse(x, y)
			want, werr := m.Distance(xd, yd)
			if !errors.Is(err, werr) || got != want {
				t.Fatal(m, got, want, err, werr)
			}
		}
		got, err := vector.SparseNorm(x)
		want, werr := vector.Norm(xd)
		if got != want || !errors.Is(err, werr) {
			t.Fatal(got, want, err, werr)
		}
	}
	for _, c := range []struct{ a, b string }{
		{"[3e38,1,-3e38]", "[1,1,1]"}, {"[1e-45,-1e-45]", "[-1e-45,1e-45]"}, {"[3e38,-3e38]", "[-3e38,3e38]"},
	} {
		a := must(t, c.a)
		b := must(t, c.b)
		x, _ := vector.SparseFromVector(a)
		y, _ := vector.SparseFromVector(b)
		for _, m := range []vector.MetricKind{vector.L1, vector.L2, vector.NegativeIP, vector.Cosine} {
			got, err := m.DistanceSparse(x, y)
			want, _ := m.Distance(a, b)
			if err != nil || got != want || math.IsNaN(got) || math.IsInf(got, 0) {
				t.Fatal(c, m, got, want, err)
			}
		}
	}
	if got, err := vector.SparseInnerProduct(mustSparseM9(t, "{1:3e38,2:1,3:-3e38}/3"), mustSparseM9(t, "{1:1,2:1,3:1}/3")); err != nil || got != 1 {
		t.Fatal("compensation", got, err)
	}
}

func TestSparseMetricErrorsAndCancellation(t *testing.T) {
	one := mustSparseM9(t, "{1:1}/1")
	if _, err := vector.SparseL2Distance(vector.SparseVector{}, one); !errors.Is(err, vector.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := vector.SparseL1Distance(one, mustSparseM9(t, "{1:1}/2")); !errors.Is(err, vector.ErrDimensions) {
		t.Fatal(err)
	}
	if _, err := vector.MetricKind("hamming").DistanceSparse(one, one); !errors.Is(err, vector.ErrMetric) {
		t.Fatal(err)
	}
	if _, err := vector.L2.DistanceSparseContext(nil, one, one); !errors.Is(err, vector.ErrContext) {
		t.Fatal(err)
	}
	entries := make([]vector.SparseEntry, 1024)
	for i := range entries {
		entries[i] = vector.SparseEntry{Index: uint32(i), Value: 1}
	}
	v, _ := vector.NewSparse(1000000000, entries)
	for _, m := range []vector.MetricKind{vector.L1, vector.L2, vector.NegativeIP, vector.Cosine} {
		c := &m9CancelAfter{Context: context.Background(), limit: 3}
		if _, err := m.DistanceSparseContext(c, v, v); !errors.Is(err, context.Canceled) || c.checks != 3 {
			t.Fatal(m, err, c.checks)
		}
	}
	c := &m9CancelAfter{Context: context.Background(), limit: 3}
	if _, err := vector.SparseNormContext(c, v); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func FuzzSparseCodec(f *testing.F) {
	for _, s := range []string{"{}/1000000000", "{1:1e-45,5:-3}/5"} {
		v, _ := vector.ParseSparse(s)
		b, _ := v.MarshalBinary()
		f.Add(b)
	}
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, b []byte) {
		v, err := vector.DecodeSparse(b)
		if err != nil {
			if v.Valid() {
				t.Fatal("partial")
			}
			return
		}
		out, err := v.MarshalBinary()
		if err != nil || !bytes.Equal(b, out) {
			t.Fatal("noncanonical", err)
		}
	})
}

func FuzzSparseText(f *testing.F) {
	for _, s := range []string{"{}/1000000000", "{1:0,1:1}/5", "{0:0}/1", "{1:1e-46}/1", "{1:NaN}/1", "{3:1,1:2}/5"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		v, err := vector.ParseSparse(s)
		if err != nil {
			if v.Valid() {
				t.Fatal("partial")
			}
			return
		}
		back, err := vector.ParseSparse(v.String())
		if err != nil || v.Dims() != back.Dims() || !reflect.DeepEqual(v.Entries(), back.Entries()) {
			t.Fatal("roundtrip", err)
		}
	})
}
