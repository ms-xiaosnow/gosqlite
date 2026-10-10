package vector_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

// These fuzzers process bounded in-memory values only. They do not open a
// database or exercise I/O faults, process crashes, or recovery workflows.
func FuzzBitVectorText(f *testing.F) {
	for _, text := range []string{"", "0", "1", "001", "101010101", " 1", "1\n", "B'01'", "\x00\xff"} {
		f.Add(text)
	}
	f.Fuzz(func(t *testing.T, text string) {
		v, err := vector.ParseBit(text)
		if len(text) > vector.MaxBitDimensions {
			if !errors.Is(err, vector.ErrDimensions) || v.Valid() {
				t.Fatal("text bound", err)
			}
			return
		}
		if err != nil {
			if v.Valid() {
				t.Fatal("partial text result")
			}
			return
		}
		if !v.Valid() || v.Dims() != len(text) || v.String() != text {
			t.Fatal("text identity")
		}
		b, err := v.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		got, err := vector.DecodeBit(b)
		if err != nil || !sameBitVector(v, got) {
			t.Fatal("text/binary round trip", err)
		}
	})
}

func FuzzBitVectorCodec(f *testing.F) {
	for _, text := range []string{"", "1", "001001001", "11111111"} {
		v, _ := vector.ParseBit(text)
		b, _ := v.MarshalBinary()
		f.Add(b)
	}
	f.Add([]byte{})
	f.Add([]byte("GSB\x00\x01\x03\x01\x00\xff\xff\xff\xff"))
	f.Fuzz(func(t *testing.T, b []byte) {
		v, err := vector.DecodeBit(b)
		if err != nil {
			if v.Valid() || !errors.Is(err, vector.ErrEncoding) {
				t.Fatal("partial or unclassified binary error", err)
			}
			return
		}
		if !v.Valid() || v.Dims() < 0 || v.Dims() > vector.MaxBitDimensions {
			t.Fatal("invalid decoded dimensions")
		}
		encoded, err := v.MarshalBinary()
		if err != nil || !bytes.Equal(b, encoded) {
			t.Fatal("noncanonical accepted bytes", err)
		}
		text, err := vector.ParseBit(v.String())
		if err != nil || !sameBitVector(v, text) {
			t.Fatal("binary/text round trip", err)
		}
	})
}

func FuzzBitVectorMetrics(f *testing.F) {
	f.Add([]byte{}, []byte{}, uint8(0))
	f.Add([]byte{0xff, 0x80}, []byte{0x01, 0}, uint8(7))
	f.Add([]byte{0x55, 0xaa}, []byte{0xaa, 0x55}, uint8(0))
	f.Fuzz(func(t *testing.T, aBytes, bBytes []byte, padding uint8) {
		// Cap arithmetic and reference strings at 2048 bits each.
		n := min(len(aBytes), len(bBytes), 256)
		aBytes, bBytes = bytes.Clone(aBytes[:n]), bytes.Clone(bBytes[:n])
		dims := n * 8
		if n > 0 {
			pad := padding % 8
			dims -= int(pad)
			mask := byte(0xff << pad)
			aBytes[n-1] &= mask
			bBytes[n-1] &= mask
		}
		a, err := vector.NewBit(dims, aBytes)
		if err != nil {
			t.Fatal(err)
		}
		b, err := vector.NewBit(dims, bBytes)
		if err != nil {
			t.Fatal(err)
		}
		h, j := bitReferenceDistances(a.String(), b.String())
		for _, tc := range []struct {
			metric vector.BitMetricKind
			want   float64
		}{{vector.Hamming, h}, {vector.Jaccard, j}} {
			got, err := tc.metric.Distance(a, b)
			if err != nil || got != tc.want {
				t.Fatal(tc.metric, got, tc.want, err)
			}
			reverse, err := tc.metric.Distance(b, a)
			if err != nil || reverse != got {
				t.Fatal("symmetry", tc.metric, reverse, got, err)
			}
		}
	})
}
