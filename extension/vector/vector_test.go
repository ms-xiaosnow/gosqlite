package vector_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

func must(t testing.TB, s string) vector.Vector {
	t.Helper()
	v, err := vector.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func sameBits(a, b vector.Vector) bool {
	av, bv := a.Values(), b.Values()
	if len(av) != len(bv) {
		return false
	}
	for i := range av {
		if math.Float32bits(av[i]) != math.Float32bits(bv[i]) {
			return false
		}
	}
	return true
}

func TestVectorImmutabilityAndZero(t *testing.T) {
	original := []float32{1, math.Float32frombits(0x80000000), math.SmallestNonzeroFloat32}
	v, err := vector.New(original)
	if err != nil {
		t.Fatal(err)
	}
	original[0] = 7
	copied := v.Values()
	copied[0] = 9
	if v.Values()[0] != 1 {
		t.Fatal("mutable backing slice")
	}
	if !v.Valid() || v.Dims() != 3 {
		t.Fatal("invalid constructed vector")
	}
	if text, err := v.MarshalText(); err != nil || string(text) != v.String() {
		t.Fatal(string(text), err)
	}
	if err := v.RequireDims(3); err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{-1, 0, 2, 16001} {
		if !errors.Is(v.RequireDims(n), vector.ErrDimensions) {
			t.Fatalf("dimension %d", n)
		}
	}
	if _, err := vector.New(nil); !errors.Is(err, vector.ErrDimensions) {
		t.Fatal(err)
	}
	if _, err := vector.New([]float32{}); !errors.Is(err, vector.ErrDimensions) {
		t.Fatal(err)
	}
	if _, err := vector.New(make([]float32, vector.MaxDimensions+1)); !errors.Is(err, vector.ErrDimensions) {
		t.Fatal(err)
	}
	for _, value := range []float32{float32(math.Inf(1)), float32(math.Inf(-1)), float32(math.NaN())} {
		if _, err := vector.New([]float32{value}); !errors.Is(err, vector.ErrNonFinite) {
			t.Fatal(err)
		}
	}
	var zero vector.Vector
	if zero.Valid() || zero.Dims() != 0 || zero.Values() != nil {
		t.Fatal("zero validity")
	}
	if zero.String() != "<invalid vector>" {
		t.Fatal(zero.String())
	}
	if _, err := zero.MarshalBinary(); !errors.Is(err, vector.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := zero.MarshalText(); !errors.Is(err, vector.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := zero.Value(); !errors.Is(err, vector.ErrInvalid) {
		t.Fatal(err)
	}
	if !errors.Is(zero.RequireDims(1), vector.ErrInvalid) {
		t.Fatal("invalid dimension check")
	}
}

func TestVectorStrictText(t *testing.T) {
	cases := map[string]string{
		"\t\n\v\f\r [ +1.,-.5,2E+2 ] \r\t":               "[1,-0.5,200]",
		"[.5,01,-0,1e-46,-1e-46]":                        "[0.5,1,-0,0,-0]",
		"[3.4028234663852886e+38,1.401298464324817e-45]": "[3.4028235e+38,1e-45]",
		// More precise than float64 at the float32 halfway boundary: direct
		// float32 parsing must avoid a double-round down to 1.
		"[1.00000005960464477539062500000000000000000000000000001]": "[1.0000001]",
	}
	for input, want := range cases {
		v, err := vector.Parse(input)
		if err != nil || v.String() != want {
			t.Errorf("%q: %s, %v; want %s", input, v, err, want)
		}
		p, err := vector.Parse(v.String())
		if err != nil || !sameBits(v, p) {
			t.Fatal("text round trip", err)
		}
	}
	invalid := []string{"", "[]", "[ ]", "[", " [  ", "1]", "[1", "[1]9", "[1,]", "[,1]", "[1,,2]", "[1 2]", "[1e]", "[1e+]", "[+]", "[.]", "['1']", "[0x1p0]", "[1_0]", "[--1]", "[1, null]", "[1]\x00", "[\u00a01]", "[１]", "[\xff]", "[1,\n]"}
	for _, s := range invalid {
		if v, err := vector.Parse(s); err == nil || v.Valid() {
			t.Errorf("accepted %q: %v", s, err)
		}
	}
	for _, s := range []string{"[NaN]", "[-nan]", "[Inf]", "[-Infinity]", "[+Infinity]", "[4e38]", "[1e999999999]"} {
		if _, err := vector.Parse(s); !errors.Is(err, vector.ErrNonFinite) {
			t.Errorf("%q: %v", s, err)
		}
	}
	if _, err := vector.Parse("[" + strings.Repeat("0", vector.MaxElementTextBytes+1) + "]"); !errors.Is(err, vector.ErrLimit) {
		t.Fatal(err)
	}
	if _, err := vector.Parse(strings.Repeat(" ", vector.MaxTextBytes+1)); !errors.Is(err, vector.ErrLimit) {
		t.Fatal(err)
	}
	if _, err := vector.ParseDims("[1,2]", 2); err != nil {
		t.Fatal(err)
	}
	if _, err := vector.ParseDims("[1,2]", 3); !errors.Is(err, vector.ErrDimensions) {
		t.Fatal(err)
	}
	if _, err := vector.ParseDims("malformed", 0); !errors.Is(err, vector.ErrDimensions) {
		t.Fatal(err)
	}
	if _, err := vector.ParseDims("malformed", 1); !errors.Is(err, vector.ErrSyntax) {
		t.Fatal(err)
	}
	max := "[" + strings.Repeat("0,", vector.MaxDimensions-1) + "0]"
	v, err := vector.Parse(max)
	if err != nil || v.Dims() != vector.MaxDimensions {
		t.Fatal(v.Dims(), err)
	}
	if _, err := vector.Parse(strings.TrimSuffix(max, "]") + ",0]"); !errors.Is(err, vector.ErrDimensions) {
		t.Fatal(err)
	}
}

func TestVectorBinaryGoldenAndFailures(t *testing.T) {
	v := must(t, "[1,-0,-2.5]")
	want := []byte{'G', 'S', 'V', 0, 1, 1, 1, 0, 0, 0, 0, 3, 0x3f, 0x80, 0, 0, 0x80, 0, 0, 0, 0xc0, 0x20, 0, 0}
	b, err := v.MarshalBinary()
	if err != nil || !bytes.Equal(b, want) {
		t.Fatalf("%x %v", b, err)
	}
	round, err := vector.Decode(b)
	if err != nil || !sameBits(v, round) {
		t.Fatal("round trip", err)
	}
	b[12] = 0
	if !sameBits(v, round) {
		t.Fatal("retained caller bytes")
	}
	for n := 0; n < len(want); n++ {
		if _, err := vector.Decode(want[:n]); !errors.Is(err, vector.ErrEncoding) {
			t.Errorf("truncated %d: %v", n, err)
		}
	}
	for _, i := range []int{0, 1, 2, 3, 4, 5, 6, 7} {
		c := bytes.Clone(want)
		c[i]++
		if _, err := vector.Decode(c); !errors.Is(err, vector.ErrEncoding) {
			t.Errorf("header %d: %v", i, err)
		}
	}
	for _, dim := range []uint32{0, 1, 2, 4, 16001, 1 << 31, math.MaxUint32} {
		c := bytes.Clone(want)
		binary.BigEndian.PutUint32(c[8:12], dim)
		if _, err := vector.Decode(c); !errors.Is(err, vector.ErrEncoding) {
			t.Errorf("dimension %d: %v", dim, err)
		}
	}
	for _, bits := range []uint32{0x7f800000, 0xff800000, 0x7fc00000, 0x7f800001, 0xffffffff} {
		c := bytes.Clone(want)
		binary.BigEndian.PutUint32(c[12:16], bits)
		if _, err := vector.Decode(c); !errors.Is(err, vector.ErrNonFinite) {
			t.Errorf("bits %x: %v", bits, err)
		}
	}
	if _, err := vector.Decode(append(bytes.Clone(want), 0)); !errors.Is(err, vector.ErrEncoding) {
		t.Fatal(err)
	}
	if _, err := vector.Decode(make([]byte, vector.MaxBinaryBytes+1)); !errors.Is(err, vector.ErrEncoding) {
		t.Fatal(err)
	}
	// A reversed dimension cannot trick Decode into an allocation; a reversed
	// payload without changing the endian tag is data, not detectable corruption.
	c := bytes.Clone(want)
	binary.LittleEndian.PutUint32(c[8:12], 3)
	if _, err := vector.Decode(c); !errors.Is(err, vector.ErrEncoding) {
		t.Fatal(err)
	}
	max, err := vector.New(make([]float32, vector.MaxDimensions))
	if err != nil {
		t.Fatal(err)
	}
	b, err = max.MarshalBinary()
	if err != nil || len(b) != vector.MaxBinaryBytes {
		t.Fatal(len(b), err)
	}
	if got, err := vector.Decode(b); err != nil || !sameBits(max, got) {
		t.Fatal(err)
	}
}

func TestVectorNullScannerAndValuer(t *testing.T) {
	var n vector.NullVector
	if x, err := n.Value(); err != nil || x != nil {
		t.Fatal(x, err)
	}
	if err := n.Scan("[1,-0]"); err != nil || !n.Valid {
		t.Fatal(err)
	}
	old := n.Vector
	x, err := n.Value()
	if err != nil {
		t.Fatal(err)
	}
	b := x.([]byte)
	if err := n.Scan(b); err != nil || !sameBits(old, n.Vector) {
		t.Fatal(err)
	}
	b[12] = 0
	if !sameBits(old, n.Vector) {
		t.Fatal("scan retained driver bytes")
	}
	for _, src := range []any{42, float64(1), true, []byte("[1,2]"), []byte(nil), "[]", vector.Vector{}} {
		if err := n.Scan(src); err == nil {
			t.Errorf("accepted %T", src)
		}
		if !n.Valid || !sameBits(old, n.Vector) {
			t.Fatal("failed scan changed destination")
		}
	}
	if err := n.Scan(nil); err != nil || n.Valid || n.Vector.Valid() {
		t.Fatal(err)
	}
	n.Valid = true
	if _, err := n.Value(); !errors.Is(err, vector.ErrInvalid) {
		t.Fatal(err)
	}
	var nilScan *vector.NullVector
	if err := nilScan.Scan(nil); !errors.Is(err, vector.ErrInvalid) {
		t.Fatal(err)
	}
}

func TestVectorConcurrentImmutableUse(t *testing.T) {
	v := must(t, "[1,-0,3,1e-45]")
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				values := v.Values()
				values[0]++
				b, err := v.MarshalBinary()
				if err != nil {
					t.Error(err)
					return
				}
				other, err := vector.Decode(b)
				if err != nil || !sameBits(v, other) {
					t.Error("concurrent decode", err)
					return
				}
				if d, err := vector.L2Distance(v, other); err != nil || d != 0 {
					t.Error(d, err)
					return
				}
				if !reflect.DeepEqual(v.Values(), other.Values()) {
					t.Error("values changed")
					return
				}
			}
		}()
	}
	wg.Wait()
}
