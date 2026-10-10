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

func mustHalfM9(t testing.TB, s string) vector.HalfVector {
	t.Helper()
	v, err := vector.ParseHalf(s)
	if err != nil {
		t.Fatal(s, err)
	}
	return v
}

// This enumerates all 65536 encodings, not a sample. The widening reference is
// independent IEEE-754 positional arithmetic, including signed zero.
func TestHalfAll65536Patterns(t *testing.T) {
	finite := 0
	for raw := 0; raw < 1<<16; raw++ {
		h := uint16(raw)
		v, err := vector.NewHalfBits([]uint16{h})
		blob := []byte{'G', 'S', 'H', 0, 1, 2, 1, 0, 0, 0, 0, 1, byte(h >> 8), byte(h)}
		decoded, decodeErr := vector.DecodeHalf(blob)
		if h&0x7c00 == 0x7c00 {
			if !errors.Is(err, vector.ErrNonFinite) || v.Valid() || !errors.Is(decodeErr, vector.ErrEncoding) || decoded.Valid() {
				t.Fatalf("nonfinite %04x accepted", h)
			}
			continue
		}
		finite++
		if err != nil || decodeErr != nil || decoded.Bits()[0] != h {
			t.Fatalf("finite %04x: %v %v", h, err, decodeErr)
		}
		fraction := float64(raw&1023) / 1024
		exponent := (raw >> 10) & 31
		var reference float64
		if exponent == 0 {
			reference = fraction * math.Pow(2, -14)
		} else {
			reference = (1 + fraction) * math.Pow(2, float64(exponent-15))
		}
		if raw&0x8000 != 0 {
			reference = math.Copysign(reference, -1)
		}
		x := v.Values()[0]
		if math.Float32bits(x) != math.Float32bits(float32(reference)) {
			t.Fatalf("widen %04x got %g want %g", h, x, reference)
		}
		back, err := vector.NewHalf([]float32{x})
		if err != nil || back.Bits()[0] != h {
			t.Fatalf("roundtrip %04x: %v", h, err)
		}
		text, err := vector.ParseHalf(v.String())
		if err != nil || text.Bits()[0] != h {
			t.Fatalf("text %04x: %v", h, err)
		}
		encoded, err := v.MarshalBinary()
		if err != nil || !bytes.Equal(blob, encoded) {
			t.Fatalf("encoding %04x: %v", h, err)
		}
	}
	if finite != 63488 {
		t.Fatal(finite)
	}
}

func TestHalfEveryAdjacentMidpoint(t *testing.T) {
	// Every positive finite pair's midpoint is exactly representable as
	// binary32. Check both neighbors and both signs, including subnormals.
	for raw := 0; raw < 0x7bff; raw++ {
		a, _ := vector.NewHalfBits([]uint16{uint16(raw), uint16(raw + 1)})
		x := a.Values()
		mid := float32((float64(x[0]) + float64(x[1])) / 2)
		want := uint16(raw)
		if raw&1 != 0 {
			want++
		}
		for _, c := range []struct {
			x    float32
			want uint16
		}{
			{math.Nextafter32(mid, float32(math.Inf(-1))), uint16(raw)},
			{mid, want}, {math.Nextafter32(mid, float32(math.Inf(1))), uint16(raw + 1)},
		} {
			for _, sign := range []uint16{0, 0x8000} {
				input := math.Float32frombits(math.Float32bits(c.x) | uint32(sign)<<16)
				v, err := vector.NewHalf([]float32{input})
				if err != nil || v.Bits()[0] != c.want|sign {
					t.Fatalf("midpoint %04x value %g got %v err %v want %04x", raw, input, v.Bits(), err, c.want|sign)
				}
			}
		}
	}
}

func TestHalfRoundingAndConversionEdges(t *testing.T) {
	for _, c := range []struct {
		in   string
		bits []uint16
	}{
		{"[65504,65505,65519,-65519]", []uint16{0x7bff, 0x7bff, 0x7bff, 0xfbff}},
		{"[0,-0,1e-8,-1e-8,1e-46,-1e-46]", []uint16{0, 0x8000, 0, 0x8000, 0, 0x8000}},
		{"[5.960464477539063e-8,-5.960464477539063e-8]", []uint16{1, 0x8001}},
		// Direct decimal->half would round upward; the pinned two-stage path
		// first rounds to the binary32 midpoint and then to the even half.
		{"[1.0004882812500000001]", []uint16{0x3c00}},
	} {
		v := mustHalfM9(t, c.in)
		if !reflect.DeepEqual(v.Bits(), c.bits) {
			t.Fatal(c.in, v.Bits(), c.bits)
		}
	}
	for _, x := range []float32{65520, -65520, math.MaxFloat32, float32(math.Inf(1)), float32(math.NaN())} {
		if v, err := vector.NewHalf([]float32{x}); !errors.Is(err, vector.ErrNonFinite) || v.Valid() {
			t.Fatal(x, v, err)
		}
	}
	below := math.Nextafter32(65520, 0)
	v, err := vector.NewHalf([]float32{below, -below})
	if err != nil || !reflect.DeepEqual(v.Bits(), []uint16{0x7bff, 0xfbff}) {
		t.Fatal(v, err)
	}
	dense, _ := vector.New([]float32{1, 1.0001, float32(math.Copysign(0, -1)), 1e-8})
	h, report, err := vector.HalfFromVector(dense)
	if err != nil || report.Changed != 2 || report.MaxRelativeError != 1 || report.MaxAbsoluteError != float64(dense.Values()[1])-1 {
		t.Fatal(h, report, err)
	}
	widened, err := h.Dense()
	if err != nil || !sameBits(widened, must(t, "[1,1,-0,0]")) {
		t.Fatal(widened, err)
	}
	if _, _, err := vector.HalfFromVector(vector.Vector{}); !errors.Is(err, vector.ErrInvalid) {
		t.Fatal(err)
	}
	if _, _, err := vector.HalfFromVector(must(t, "[1e10]")); !errors.Is(err, vector.ErrNonFinite) {
		t.Fatal(err)
	}
}

func TestHalfImmutabilityBoundsAndNull(t *testing.T) {
	source := []uint16{0x3c00, 0x8000, 1}
	v, err := vector.NewHalfBits(source)
	if err != nil {
		t.Fatal(err)
	}
	want := v.String()
	source[0] = 0
	bits := v.Bits()
	bits[0] = 0
	values := v.Values()
	values[0] = 0
	if v.String() != want {
		t.Fatal("storage escaped")
	}
	for _, n := range []int{0, -1, vector.MaxHalfDimensions + 1} {
		if _, err := vector.ParseHalfDims("[1]", n); !errors.Is(err, vector.ErrDimensions) {
			t.Fatal(n, err)
		}
	}
	if _, err := vector.NewHalf(make([]float32, vector.MaxHalfDimensions)); err != nil {
		t.Fatal(err)
	}
	if _, err := vector.NewHalf(make([]float32, vector.MaxHalfDimensions+1)); !errors.Is(err, vector.ErrDimensions) {
		t.Fatal(err)
	}
	for _, s := range []string{"[]", "[NaN]", "[Infinity]", "[65520]", "[0x1p0]", "[1_0]", "[1,]", "[1]\x00"} {
		if v, err := vector.ParseHalf(s); err == nil || v.Valid() {
			t.Fatal(s, v, err)
		}
	}
	if _, err := vector.ParseHalf(strings.Repeat(" ", vector.MaxTextBytes+1)); !errors.Is(err, vector.ErrLimit) {
		t.Fatal(err)
	}
	if _, err := vector.ParseHalf("[" + strings.Repeat("1", vector.MaxElementTextBytes+1) + "]"); !errors.Is(err, vector.ErrLimit) {
		t.Fatal(err)
	}
	if _, err := vector.ParseHalfDims("[1,2]", 1); !errors.Is(err, vector.ErrDimensions) {
		t.Fatal(err)
	}
	if (vector.HalfVector{}).Valid() || (vector.HalfVector{}).Values() != nil || (vector.HalfVector{}).Bits() != nil {
		t.Fatal("invalid zero")
	}
	if _, err := (vector.HalfVector{}).MarshalBinary(); !errors.Is(err, vector.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := (vector.HalfVector{}).Dense(); !errors.Is(err, vector.ErrInvalid) {
		t.Fatal(err)
	}
	var scan vector.NullHalfVector
	if err := scan.Scan(want); err != nil {
		t.Fatal(err)
	}
	for _, src := range []any{1, []float32{1}, []byte(want), "[NaN]", vector.HalfVector{}} {
		if err := scan.Scan(src); err == nil || !scan.Valid || scan.HalfVector.String() != want {
			t.Fatal(src, err, scan)
		}
	}
	b, _ := v.MarshalBinary()
	if err := scan.Scan(b); err != nil {
		t.Fatal(err)
	}
	b[12] = 0
	if scan.HalfVector.String() != want {
		t.Fatal("borrowed scan storage")
	}
	if _, err := scan.Value(); err != nil {
		t.Fatal(err)
	}
	if err := scan.Scan(nil); err != nil || scan.Valid || scan.HalfVector.Valid() {
		t.Fatal(scan, err)
	}
	if x, err := scan.Value(); x != nil || err != nil {
		t.Fatal(x, err)
	}
	if _, err := (vector.NullHalfVector{Valid: true}).Value(); !errors.Is(err, vector.ErrInvalid) {
		t.Fatal(err)
	}
	if err := (*vector.NullHalfVector)(nil).Scan(nil); !errors.Is(err, vector.ErrInvalid) {
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
				_, _ = vector.HalfL2Distance(v, v)
			}
		}()
	}
	wg.Wait()
}

func TestHalfCodecRejectsNoncanonical(t *testing.T) {
	v := mustHalfM9(t, "[1,-0,0.000000059604644775390625]")
	good, _ := v.MarshalBinary()
	for i := 0; i < 8; i++ {
		b := append([]byte(nil), good...)
		b[i] ^= 0xff
		if _, err := vector.DecodeHalf(b); !errors.Is(err, vector.ErrEncoding) {
			t.Fatal(i, err)
		}
	}
	for _, dim := range []uint32{0, 16001, math.MaxUint32} {
		b := append([]byte(nil), good...)
		binary.BigEndian.PutUint32(b[8:], dim)
		if _, err := vector.DecodeHalf(b); !errors.Is(err, vector.ErrEncoding) {
			t.Fatal(dim, err)
		}
	}
	for i := 0; i < len(good); i++ {
		if _, err := vector.DecodeHalf(good[:i]); !errors.Is(err, vector.ErrEncoding) {
			t.Fatal(i, err)
		}
	}
	if _, err := vector.DecodeHalf(append(good, 0)); !errors.Is(err, vector.ErrEncoding) {
		t.Fatal(err)
	}
	if _, err := vector.Decode(good); !errors.Is(err, vector.ErrEncoding) {
		t.Fatal("dense accepted half", err)
	}
	dense, _ := must(t, "[1]").MarshalBinary()
	if _, err := vector.DecodeHalf(dense); !errors.Is(err, vector.ErrEncoding) {
		t.Fatal(err)
	}
}

type m9CancelAfter struct {
	context.Context
	checks, limit int
}

func (c *m9CancelAfter) Err() error {
	c.checks++
	if c.checks >= c.limit {
		return context.Canceled
	}
	return nil
}

func TestHalfMetricReferenceAndCancellation(t *testing.T) {
	rng := rand.New(rand.NewSource(903))
	for n := 0; n < 500; n++ {
		dim := 1 + rng.Intn(64)
		a, b := make([]uint16, dim), make([]uint16, dim)
		for i := range a {
			a[i] = uint16(rng.Intn(0x7c00)) | uint16(rng.Intn(2))<<15
			b[i] = uint16(rng.Intn(0x7c00)) | uint16(rng.Intn(2))<<15
		}
		x, _ := vector.NewHalfBits(a)
		y, _ := vector.NewHalfBits(b)
		xd, _ := x.Dense()
		yd, _ := y.Dense()
		for _, m := range []vector.MetricKind{vector.L1, vector.L2, vector.NegativeIP, vector.Cosine} {
			got, e := m.DistanceHalf(x, y)
			want, werr := m.Distance(xd, yd)
			if !errors.Is(e, werr) || got != want {
				t.Fatal(m, got, want, e, werr)
			}
		}
		got, err := vector.HalfNorm(x)
		want, werr := vector.Norm(xd)
		if got != want || !errors.Is(err, werr) {
			t.Fatal(got, want, err, werr)
		}
	}
	zero := mustHalfM9(t, "[0]")
	one := mustHalfM9(t, "[1]")
	if _, err := vector.HalfCosineDistance(zero, one); !errors.Is(err, vector.ErrZeroNorm) {
		t.Fatal(err)
	}
	if _, err := vector.HalfL1Distance(one, mustHalfM9(t, "[1,2]")); !errors.Is(err, vector.ErrDimensions) {
		t.Fatal(err)
	}
	if _, err := vector.HalfL2Distance(vector.HalfVector{}, one); !errors.Is(err, vector.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := vector.MetricKind("hamming").DistanceHalf(one, one); !errors.Is(err, vector.ErrMetric) {
		t.Fatal(err)
	}
	if x, err := vector.HalfInnerProduct(one, one); x != 1 || err != nil {
		t.Fatal(x, err)
	}
	if _, err := vector.L2.DistanceHalfContext(nil, one, one); !errors.Is(err, vector.ErrContext) {
		t.Fatal(err)
	}
	large, _ := vector.NewHalf(make([]float32, 1024))
	for _, m := range []vector.MetricKind{vector.L1, vector.L2, vector.NegativeIP, vector.Cosine} {
		c := &m9CancelAfter{Context: context.Background(), limit: 3}
		if _, err := m.DistanceHalfContext(c, large, large); !errors.Is(err, context.Canceled) || c.checks != 3 {
			t.Fatal(m, err, c.checks)
		}
	}
	c := &m9CancelAfter{Context: context.Background(), limit: 3}
	if _, err := vector.HalfNormContext(c, large); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func FuzzHalfCodec(f *testing.F) {
	f.Add([]byte{'G', 'S', 'H', 0, 1, 2, 1, 0, 0, 0, 0, 1, 0x3c, 0})
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, b []byte) {
		v, err := vector.DecodeHalf(b)
		if err != nil {
			if v.Valid() {
				t.Fatal("partial result")
			}
			return
		}
		out, err := v.MarshalBinary()
		if err != nil || !bytes.Equal(b, out) {
			t.Fatal("noncanonical accepted", err)
		}
	})
}

func FuzzHalfText(f *testing.F) {
	for _, s := range []string{"[0,-0,1e-8,65519]", "[NaN]", "[1.0004882812500000001]", "[]", "[65520]"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		v, err := vector.ParseHalf(s)
		if err != nil {
			if v.Valid() {
				t.Fatal("partial")
			}
			return
		}
		back, err := vector.ParseHalf(v.String())
		if err != nil || !reflect.DeepEqual(v.Bits(), back.Bits()) {
			t.Fatal("roundtrip", err)
		}
	})
}
