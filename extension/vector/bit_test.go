package vector_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"strings"
	"sync"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

func mustBit(t testing.TB, text string) vector.BitVector {
	t.Helper()
	v, err := vector.ParseBit(text)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func sameBitVector(a, b vector.BitVector) bool {
	return a.Valid() == b.Valid() && a.Dims() == b.Dims() && bytes.Equal(a.Packed(), b.Packed())
}

func TestBitVectorImmutabilityAndZero(t *testing.T) {
	original := []byte{0xa5, 0x80}
	v, err := vector.NewBit(9, original)
	if err != nil || !v.Valid() || v.Dims() != 9 || v.String() != "101001011" {
		t.Fatal(v, err)
	}
	original[0] = 0
	packed := v.Packed()
	packed[0] = 0
	text, err := v.MarshalText()
	if err != nil || string(text) != "101001011" {
		t.Fatal(string(text), err)
	}
	text[0] = '0'
	encoded, err := v.MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	encoded[12] = 0
	value, err := v.Value()
	if err != nil {
		t.Fatal(err)
	}
	value.([]byte)[12] = 0
	if v.String() != "101001011" {
		t.Fatal("constructor or accessor leaked mutable storage")
	}
	if err := v.RequireDims(9); err != nil {
		t.Fatal(err)
	}
	for _, n := range []int{-1, 0, 8, 10, vector.MaxBitDimensions + 1, math.MaxInt} {
		if !errors.Is(v.RequireDims(n), vector.ErrDimensions) {
			t.Fatalf("dimension %d", n)
		}
	}
	var zero vector.BitVector
	if zero.Valid() || zero.Dims() != 0 || zero.Packed() != nil || zero.String() != "<invalid bit vector>" {
		t.Fatal("invalid zero value contract")
	}
	if _, err := zero.MarshalText(); !errors.Is(err, vector.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := zero.MarshalBinary(); !errors.Is(err, vector.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := zero.Value(); !errors.Is(err, vector.ErrInvalid) {
		t.Fatal(err)
	}
	for _, n := range []int{0, 1} {
		if !errors.Is(zero.RequireDims(n), vector.ErrInvalid) {
			t.Fatal("invalid dimension check", n)
		}
	}
	for _, input := range [][]byte{nil, {}} {
		empty, err := vector.NewBit(0, input)
		if err != nil || !empty.Valid() || empty.Dims() != 0 || empty.String() != "" || empty.RequireDims(0) != nil {
			t.Fatal("constructed empty", empty, err)
		}
		if text, err := empty.MarshalText(); err != nil || len(text) != 0 {
			t.Fatal("empty text", text, err)
		}
		value, err := empty.Value()
		if err != nil || value == nil || len(value.([]byte)) != vector.BitBinaryHeaderSize {
			t.Fatal("empty is non-NULL", value, err)
		}
	}
}

func TestBitVectorConstructorStrictBoundaries(t *testing.T) {
	for _, n := range []int{math.MinInt, -1, vector.MaxBitDimensions + 1, math.MaxInt} {
		if v, err := vector.NewBit(n, nil); !errors.Is(err, vector.ErrDimensions) || v.Valid() {
			t.Fatal(n, v, err)
		}
	}
	for _, tc := range []struct {
		n int
		b []byte
	}{{0, []byte{0}}, {1, nil}, {1, []byte{0, 0}}, {8, nil}, {9, []byte{0}}, {16, []byte{0, 0, 0}}} {
		if v, err := vector.NewBit(tc.n, tc.b); !errors.Is(err, vector.ErrEncoding) || v.Valid() {
			t.Fatal(tc, v, err)
		}
	}
	for dims := 1; dims <= 16; dims++ {
		packed := make([]byte, (dims+7)/8)
		packed[len(packed)-1] = 0x80
		v, err := vector.NewBit(dims, packed)
		if err != nil {
			t.Fatal("canonical padding rejected", dims, err)
		}
		encoded, err := v.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		for pad := 0; pad < (8-dims%8)%8; pad++ {
			bad := bytes.Clone(packed)
			bad[len(bad)-1] |= 1 << uint(pad)
			if v, err := vector.NewBit(dims, bad); !errors.Is(err, vector.ErrEncoding) || v.Valid() {
				t.Fatal("noncanonical padding accepted", dims, pad, err)
			}
			badEncoding := bytes.Clone(encoded)
			badEncoding[len(badEncoding)-1] |= 1 << uint(pad)
			if v, err := vector.DecodeBit(badEncoding); !errors.Is(err, vector.ErrEncoding) || v.Valid() {
				t.Fatal("decoder accepted noncanonical padding", dims, pad, err)
			}
		}
	}
	max, err := vector.NewBit(vector.MaxBitDimensions, bytes.Repeat([]byte{0xff}, vector.MaxBitDimensions/8))
	if err != nil || max.String() != strings.Repeat("1", vector.MaxBitDimensions) {
		t.Fatal("maximum", max.Dims(), err)
	}
}

func TestBitVectorStrictText(t *testing.T) {
	for _, text := range []string{"", "0", "1", "001", "10101010", "101010101", strings.Repeat("01", vector.MaxBitDimensions/2)} {
		v, err := vector.ParseBit(text)
		if err != nil || !v.Valid() || v.String() != text || v.Dims() != len(text) {
			t.Fatal("text round trip", len(text), v.Dims(), err)
		}
		if got, err := vector.ParseBitDims(text, len(text)); err != nil || !sameBitVector(v, got) {
			t.Fatal("explicit dimensions", len(text), err)
		}
		if got, err := vector.NewBit(v.Dims(), v.Packed()); err != nil || !sameBitVector(v, got) {
			t.Fatal("packed round trip", len(text), err)
		}
	}
	for _, text := range []string{" ", " 1", "1 ", "1\t", "1\r\n", "1\v", "1\f", "1\x00", "1\xff", "１０", "\u00a01", "B'01'", "b01", "0b01", "[0,1]", "'01'", "0_1", "2", "-1", "+1"} {
		if v, err := vector.ParseBit(text); !errors.Is(err, vector.ErrSyntax) || v.Valid() {
			t.Fatalf("accepted %q: %v", text, err)
		}
	}
	if v, err := vector.ParseBit(strings.Repeat("0", vector.MaxBitDimensions+1)); !errors.Is(err, vector.ErrDimensions) || v.Valid() {
		t.Fatal("text dimension cap", err)
	}
	for _, n := range []int{-1, vector.MaxBitDimensions + 1, math.MaxInt} {
		if v, err := vector.ParseBitDims("bad", n); !errors.Is(err, vector.ErrDimensions) || v.Valid() {
			t.Fatal("dimension validation precedes parsing", n, err)
		}
	}
	if v, err := vector.ParseBitDims("bad", 3); !errors.Is(err, vector.ErrSyntax) || v.Valid() {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		text string
		n    int
	}{{"", 1}, {"0", 0}, {"001", 2}, {"001", 4}} {
		if v, err := vector.ParseBitDims(tc.text, tc.n); !errors.Is(err, vector.ErrDimensions) || v.Valid() {
			t.Fatal("no width coercion", tc, err)
		}
	}
}

func TestBitVectorBinaryGoldenAndFailures(t *testing.T) {
	v := mustBit(t, "101001011")
	want := []byte{'G', 'S', 'B', 0, 1, 3, 1, 0, 0, 0, 0, 9, 0xa5, 0x80}
	b, err := v.MarshalBinary()
	if err != nil || !bytes.Equal(b, want) {
		t.Fatalf("%x %v", b, err)
	}
	decoded, err := vector.DecodeBit(b)
	if err != nil || !sameBitVector(v, decoded) {
		t.Fatal("binary round trip", err)
	}
	b[12] = 0
	if !sameBitVector(v, decoded) {
		t.Fatal("decoder retained caller bytes")
	}
	bad := func(b []byte) {
		t.Helper()
		if v, err := vector.DecodeBit(b); !errors.Is(err, vector.ErrEncoding) || v.Valid() {
			t.Fatalf("accepted %x: %v", b, err)
		}
	}
	for size := 0; size < len(want); size++ {
		bad(want[:size])
	}
	for offset := 0; offset < 8; offset++ {
		c := bytes.Clone(want)
		c[offset]++
		bad(c)
	}
	for _, dim := range []uint32{0, 1, 8, 17, vector.MaxBitDimensions + 1, 1 << 31, math.MaxUint32} {
		c := bytes.Clone(want)
		binary.BigEndian.PutUint32(c[8:12], dim)
		bad(c)
	}
	c := bytes.Clone(want)
	binary.LittleEndian.PutUint32(c[8:12], 9)
	bad(c)
	bad(append(bytes.Clone(want), 0))
	bad(make([]byte, vector.MaxBitBinaryBytes+1))
	for pad := 0; pad < 7; pad++ {
		c := bytes.Clone(want)
		c[len(c)-1] |= 1 << uint(pad)
		bad(c)
	}
	for _, text := range []string{"", "0", "1", "1010101", "11111111", "111111111", strings.Repeat("1", vector.MaxBitDimensions)} {
		v := mustBit(t, text)
		b, err := v.MarshalBinary()
		if err != nil || len(b) != vector.BitBinaryHeaderSize+(v.Dims()+7)/8 {
			t.Fatal("encoded size", len(b), err)
		}
		got, err := vector.DecodeBit(b)
		if err != nil || !sameBitVector(v, got) {
			t.Fatal("edge dimension round trip", v.Dims(), err)
		}
		if len(text) == vector.MaxBitDimensions && len(b) != vector.MaxBitBinaryBytes {
			t.Fatal("maximum binary bound")
		}
	}
	// A bit decoder never guesses other kinds from their payload, and the
	// existing dense decoder never accepts a bit encoding.
	dense, err := must(t, "[1]").MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	bad(dense)
	for _, magic := range []string{"GSV\x00", "GSH\x00", "GSS\x00"} {
		c := bytes.Clone(want)
		copy(c, magic)
		bad(c)
	}
	if _, err := vector.Decode(want); !errors.Is(err, vector.ErrEncoding) {
		t.Fatal("dense accepted packed-bit blob", err)
	}
	if _, err := vector.DecodeBit([]byte("10101010")); !errors.Is(err, vector.ErrEncoding) {
		t.Fatal("binary guessed text", err)
	}
}

func TestBitVectorNullScannerAndValuer(t *testing.T) {
	var n vector.NullBitVector
	if value, err := n.Value(); err != nil || value != nil {
		t.Fatal(value, err)
	}
	if err := n.Scan("001001001"); err != nil || !n.Valid {
		t.Fatal(err)
	}
	old := n.BitVector
	value, err := n.Value()
	if err != nil {
		t.Fatal(err)
	}
	b := value.([]byte)
	if err := n.Scan(b); err != nil || !sameBitVector(old, n.BitVector) {
		t.Fatal(err)
	}
	b[12] = 0xff
	if !sameBitVector(old, n.BitVector) {
		t.Fatal("scanner retained source bytes")
	}
	dense, err := must(t, "[1]").MarshalBinary()
	if err != nil {
		t.Fatal(err)
	}
	for _, src := range []any{1, int64(1), float64(1), true, []byte("001"), []byte(nil), " 001", "2", vector.BitVector{}, dense} {
		if err := n.Scan(src); err == nil {
			t.Fatalf("accepted %T", src)
		}
		if !n.Valid || !sameBitVector(old, n.BitVector) {
			t.Fatal("failed scan changed destination")
		}
	}
	// Preservation applies equally when Valid is false but the ignored value
	// is nonempty, not just to a previously successful scan.
	n.Valid = false
	if err := n.Scan("bad"); err == nil || n.Valid || !sameBitVector(old, n.BitVector) {
		t.Fatal("failed scan changed invalid destination", err)
	}
	if value, err := n.Value(); err != nil || value != nil {
		t.Fatal("invalid nullable ignores vector", value, err)
	}
	if err := n.Scan(""); err != nil || !n.Valid || !n.BitVector.Valid() || n.BitVector.Dims() != 0 {
		t.Fatal("empty must not scan as NULL", err)
	}
	if err := n.Scan(nil); err != nil || n.Valid || n.BitVector.Valid() {
		t.Fatal("NULL scan must reset both fields", err)
	}
	n.Valid = true
	if _, err := n.Value(); !errors.Is(err, vector.ErrInvalid) {
		t.Fatal(err)
	}
	var nilScan *vector.NullBitVector
	if err := nilScan.Scan(nil); !errors.Is(err, vector.ErrInvalid) {
		t.Fatal(err)
	}
}

func TestBitVectorConcurrentImmutableUse(t *testing.T) {
	v := mustBit(t, strings.Repeat("100", 171))
	var wg sync.WaitGroup
	for worker := 0; worker < 12; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for repeat := 0; repeat < 100; repeat++ {
				packed := v.Packed()
				packed[0]++
				b, err := v.MarshalBinary()
				if err != nil {
					t.Error(err)
					return
				}
				got, err := vector.DecodeBit(b)
				if err != nil || !sameBitVector(v, got) {
					t.Error("concurrent decode", err)
					return
				}
				for _, metric := range []vector.BitMetricKind{vector.Hamming, vector.Jaccard} {
					if d, err := metric.Distance(v, got); err != nil || d != 0 {
						t.Error(metric, d, err)
						return
					}
				}
			}
		}()
	}
	wg.Wait()
}
