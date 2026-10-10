package record

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"math/rand"
	"reflect"
	"testing"
)

func TestVarintGolden(t *testing.T) {
	cases := []struct {
		v uint64
		p []byte
	}{
		{0, []byte{0}}, {127, []byte{127}}, {128, []byte{0x81, 0}},
		{16383, []byte{0xff, 0x7f}}, {16384, []byte{0x81, 0x80, 0}},
		{0x00ffffffffffffff, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x7f}},
		{0x0100000000000000, []byte{0x80, 0xc0, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0}},
		{math.MaxUint64, []byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}},
	}
	for _, tt := range cases {
		got := AppendVarint(nil, tt.v)
		if !bytes.Equal(got, tt.p) {
			t.Errorf("%x encode %x want %x", tt.v, got, tt.p)
		}
		v, n, e := DecodeVarint(tt.p)
		if e != nil || v != tt.v || n != len(tt.p) {
			t.Errorf("%x decode %x/%d/%v", tt.p, v, n, e)
		}
	}
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 10000; i++ {
		want := rng.Uint64()
		p := AppendVarint([]byte{42}, want)
		got, n, e := DecodeVarint(p[1:])
		if e != nil || got != want || n != len(p)-1 {
			t.Fatalf("roundtrip %x", want)
		}
	}
	for n := 0; n < 9; n++ {
		p := bytes.Repeat([]byte{0x80}, n)
		if _, _, e := DecodeVarint(p); !errors.Is(e, ErrCorrupt) {
			t.Fatalf("truncated %x: %v", p, e)
		}
	}
}

func TestRecordRoundTrip(t *testing.T) {
	values := []any{nil, int64(math.MinInt64), int64(-140737488355329), int64(-140737488355328), int64(-2147483649), int64(-2147483648), int64(-8388609), int64(-8388608), int64(-32769), int64(-32768), int64(-129), int64(-128), int64(-1), int64(0), int64(1), int64(127), int64(128), int64(32767), int64(32768), int64(8388607), int64(8388608), int64(2147483647), int64(2147483648), int64(140737488355327), int64(140737488355328), int64(math.MaxInt64), -1.25, math.Inf(1), math.Copysign(0, -1), "", "数据库🙂é한글", []byte{}, []byte{0, 255, 128}, string([]byte{0xff, 0})}
	p, e := Encode(values)
	if e != nil {
		t.Fatal(e)
	}
	got, e := Decode(p)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(got, values) {
		t.Fatalf("roundtrip got %#v want %#v", got, values)
	}
	if !math.Signbit(got[28].(float64)) {
		t.Fatal("lost negative zero")
	}
	// Header length crosses a varint boundary; string length crosses it too.
	many := make([]any, 150)
	for i := range many {
		many[i] = string(bytes.Repeat([]byte{'x'}, 130))
	}
	p, e = Encode(many)
	if e != nil {
		t.Fatal(e)
	}
	got, e = Decode(p)
	if e != nil || !reflect.DeepEqual(got, many) {
		t.Fatalf("large header: %v", e)
	}
}

func TestRecordGoldenAndOwnership(t *testing.T) {
	// Header 07, serials NULL/int8/int16/int0/text2/blob2, then body.
	p := []byte{7, 0, 1, 2, 8, 17, 16, 0xff, 1, 0, 'h', 'i', 0, 255}
	want := []any{nil, int64(-1), int64(256), int64(0), "hi", []byte{0, 255}}
	got, e := Decode(p)
	if e != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("%#v %v", got, e)
	}
	encoded, e := Encode(want)
	if e != nil || !bytes.Equal(encoded, p) {
		t.Fatalf("%x %v", encoded, e)
	}
	p[len(p)-1] = 42
	if got[5].([]byte)[1] != 255 {
		t.Fatal("BLOB borrowed input memory")
	}
}

func TestRecordMalformed(t *testing.T) {
	for _, p := range [][]byte{nil, {0}, {2}, {1, 0}, {2, 10}, {2, 11}, {2, 1}, {0x81, 0}, {3, 0x80, 0x80}, {2, 127}} {
		if _, e := Decode(p); !errors.Is(e, ErrCorrupt) {
			t.Errorf("%x: %v", p, e)
		}
	}
	if _, e := Encode([]any{42}); !errors.Is(e, ErrUnsupported) {
		t.Fatal(e)
	}
	p := make([]byte, 10)
	p[0], p[1] = 2, 7
	binary.BigEndian.PutUint64(p[2:], math.Float64bits(math.NaN()))
	got, e := Decode(p)
	if e != nil || got[0] != nil {
		t.Fatalf("NaN decode %#v %v", got, e)
	}
	p, e = Encode([]any{math.NaN()})
	if e != nil || !bytes.Equal(p, []byte{2, 0}) {
		t.Fatalf("NaN encode %x %v", p, e)
	}
}
