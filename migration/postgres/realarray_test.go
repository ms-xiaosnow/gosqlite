package postgres

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func TestRealArrayFiniteBinary32(t *testing.T) {
	tests := []struct {
		s    string
		bits []uint32
	}{
		{" { 1, -2.5, 3e-4 } ", []uint32{math.Float32bits(1), math.Float32bits(-2.5), math.Float32bits(3e-4)}},
		{`[1:3]={"1", "-2.5", "3e-4"}`, []uint32{math.Float32bits(1), math.Float32bits(-2.5), math.Float32bits(3e-4)}},
		{`{-0,1.401298464324817e-45,3.4028234663852886e38}`, []uint32{0x80000000, 1, 0x7f7fffff}},
		{`{-1e-999,1e-999}`, []uint32{0x80000000, 0}},
		{`{"\1",2}`, []uint32{0x3f800000, 0x40000000}},
	}
	for _, tt := range tests {
		v, e := ParseRealArray(tt.s, len(tt.bits))
		if e != nil {
			t.Fatalf("%q: %v", tt.s, e)
		}
		for i, x := range v.Values() {
			if got := math.Float32bits(x); got != tt.bits[i] {
				t.Fatalf("bits[%d]=%x want %x", i, got, tt.bits[i])
			}
		}
	}
}
func TestRealArrayRejects(t *testing.T) {
	tests := []struct {
		s    string
		dims int
		code string
	}{
		{"{}", 3, "empty_array"}, {"NULL", 3, "array_syntax"}, {"{NULL}", 1, "null_element"}, {`{"NULL"}`, 1, "invalid_component"},
		{"{NaN}", 1, "nonfinite_component"}, {"{Infinity}", 1, "nonfinite_component"}, {"{3.5e38}", 1, "nonfinite_component"},
		{"{{1,2},{3,4}}", 4, "multidimensional_array"}, {"[0:1]={1,2}", 2, "nondefault_array_bounds"}, {"[1:1][1:2]={{1,2}}", 2, "multidimensional_array"},
		{"[1:3]={1,2}", 2, "bounds_length_mismatch"}, {"{1,2}", 3, "dimension_mismatch"}, {"{1,}", 2, "invalid_component"},
		{"{1}junk", 1, "array_syntax"}, {"{1 2}", 1, "invalid_component"}, {"{0x1p0}", 1, "invalid_component"}, {"{1_000}", 1, "invalid_component"},
		{"{\xff}", 1, "invalid_encoding"}, {"{\x00}", 1, "invalid_encoding"}, {`{"1,2"}`, 1, "invalid_component"},
		{"{1}", 0, "dimension_required"}, {"{" + strings.Repeat("1", 129) + "}", 1, "resource_limit"},
	}
	for _, tt := range tests {
		v, e := ParseRealArray(tt.s, tt.dims)
		var ve *ValueError
		if !errors.As(e, &ve) || ve.Code != tt.code || v.Valid() {
			t.Errorf("%q dims=%d got=%v valid=%t want=%s", tt.s, tt.dims, e, v.Valid(), tt.code)
		}
	}
}
func FuzzRealArray(f *testing.F) {
	for _, s := range []string{"{1,2,3}", "[1:1]={-0}", `{"NULL"}`, "{{1}}"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > MaxArrayTextBytes+1 {
			t.Skip()
		}
		v, e := ParseRealArray(s, 3)
		if e == nil && v.Dims() != 3 {
			t.Fatal("dimension invariant")
		}
		if e != nil && v.Valid() {
			t.Fatal("partial value")
		}
	})
}
