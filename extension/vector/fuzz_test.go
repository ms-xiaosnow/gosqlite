package vector_test

import (
	"context"
	"encoding/binary"
	"errors"
	"math"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

// These fuzzers only process bounded in-memory vector data. They never open a
// database or inject I/O faults, kill processes, or perform recovery tests.
func FuzzVectorText(f *testing.F) {
	for _, s := range []string{"[1,2,3]", "[1e-46,-0,1e-45]", "[NaN]", "[0x1p0]", "[1,]", "[]", "[1.000000059604644775390626]", "[1e999999]", "\x00\xff"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		v, err := vector.Parse(s)
		if len(s) > vector.MaxTextBytes {
			if !errors.Is(err, vector.ErrLimit) {
				t.Fatal(err)
			}
			return
		}
		if err != nil {
			if v.Valid() {
				t.Fatal("partial result")
			}
			return
		}
		if !v.Valid() || v.Dims() < 1 || v.Dims() > vector.MaxDimensions {
			t.Fatal("invalid parsed vector")
		}
		b, err := v.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := vector.Decode(b)
		if err != nil || !sameBits(v, decoded) {
			t.Fatal("binary round trip", err)
		}
		text, err := vector.Parse(v.String())
		if err != nil || !sameBits(v, text) {
			t.Fatal("text round trip", err)
		}
	})
}

func FuzzVectorCodec(f *testing.F) {
	for _, s := range []string{"[1]", "[1,-0,3e38,1e-45]"} {
		v, _ := vector.Parse(s)
		b, _ := v.MarshalBinary()
		f.Add(b)
	}
	f.Add([]byte{})
	f.Add([]byte("GSV\x00\x01\x01\x01\x00\xff\xff\xff\xff"))
	f.Fuzz(func(t *testing.T, b []byte) {
		v, err := vector.Decode(b)
		if err != nil {
			if v.Valid() {
				t.Fatal("partial result")
			}
			return
		}
		encoded, err := v.MarshalBinary()
		if err != nil || string(b) != string(encoded) {
			t.Fatal("noncanonical accepted bytes", err)
		}
		for _, x := range v.Values() {
			if math.IsNaN(float64(x)) || math.IsInf(float64(x), 0) {
				t.Fatal("non-finite accepted")
			}
		}
	})
}

func FuzzVectorFloatBits(f *testing.F) {
	f.Add([]byte{0, 0, 0, 0, 0x80, 0, 0, 0, 0, 0, 0, 1, 0x7f, 0x7f, 0xff, 0xff})
	f.Add([]byte{0x7f, 0x80, 0, 0})
	f.Add([]byte{0xff, 0xff, 0xff, 0xff})
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 2048 {
			b = b[:2048]
		}
		values := make([]float32, len(b)/4)
		finite := true
		for i := range values {
			values[i] = math.Float32frombits(binary.BigEndian.Uint32(b[4*i:]))
			if math.IsNaN(float64(values[i])) || math.IsInf(float64(values[i]), 0) {
				finite = false
			}
		}
		v, err := vector.New(values)
		if len(values) == 0 {
			if !errors.Is(err, vector.ErrDimensions) {
				t.Fatal(err)
			}
			return
		}
		if !finite {
			if !errors.Is(err, vector.ErrNonFinite) {
				t.Fatal(err)
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := v.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := vector.Decode(encoded)
		if err != nil || !sameBits(v, decoded) {
			t.Fatal("bits changed", err)
		}
		text, err := vector.Parse(v.String())
		if err != nil || !sameBits(v, text) {
			t.Fatal("text bits changed", err)
		}
		for _, m := range []vector.MetricKind{vector.L1, vector.L2, vector.NegativeIP, vector.Cosine} {
			distance, err := m.Distance(v, v)
			if errors.Is(err, vector.ErrZeroNorm) && m == vector.Cosine {
				continue
			}
			if err != nil || math.IsInf(distance, 0) || math.IsNaN(distance) {
				t.Fatal(distance, err)
			}
		}
	})
}

func FuzzVectorTopK(f *testing.F) {
	f.Add([]byte{7, 2, 3, 4, 5, 6, 7, 8, 9})
	f.Add([]byte{0, 0, 0, 0, 0})
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, b []byte) {
		if len(b) > 129 {
			b = b[:129]
		}
		if len(b) == 0 {
			return
		}
		q, _ := vector.New([]float32{1, 2})
		rows := make([]vector.Row, (len(b)-1)/2)
		for i := range rows {
			v, _ := vector.New([]float32{float32(int8(b[1+2*i])), float32(int8(b[2+2*i]))})
			rows[i] = vector.Row{RowID: int64(len(rows) - i), Vector: v}
		}
		k := int(b[0]) % (len(rows) + 1)
		for _, m := range []vector.MetricKind{vector.L1, vector.L2, vector.NegativeIP} {
			got, err := vector.TopK(context.Background(), q, rows, vector.SearchOptions{Metric: m, Limit: k})
			if err != nil {
				t.Fatal(err)
			}
			want := sortedReference(q, rows, m, k)
			if len(got.Hits) != len(want) {
				t.Fatal("size mismatch")
			}
			for i, h := range got.Hits {
				if h.RowID != want[i].RowID || h.Distance != want[i].Distance {
					t.Fatal("rank mismatch", h, want[i])
				}
			}
		}
	})
}
