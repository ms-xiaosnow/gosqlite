package record

import (
	"errors"
	"math"
	"reflect"
	"testing"
)

// These targets use only memory. No database paths, oracle subprocesses,
// filesystem mutation, failure injection, or recovery behavior are involved.
func FuzzVarint(f *testing.F) {
	for _, v := range []uint64{0, 1, 127, 128, 16383, 16384, 0x00ffffffffffffff, 0x0100000000000000, math.MaxInt64, 1 << 63, math.MaxUint64} {
		f.Add(AppendVarint(nil, v))
	}
	for _, p := range [][]byte{nil, {128}, {128, 0}, {255, 255}, {128, 128, 128, 128, 128, 128, 128, 128}} {
		f.Add(p)
	}
	f.Fuzz(func(t *testing.T, p []byte) {
		value, n, err := DecodeVarint(p)
		if err != nil {
			if !errors.Is(err, ErrCorrupt) {
				t.Fatalf("unexpected error: %v", err)
			}
			return
		}
		if n < 1 || n > 9 || n > len(p) {
			t.Fatalf("invalid consumed length %d", n)
		}
		encoded := AppendVarint([]byte{42}, value)
		if encoded[0] != 42 {
			t.Fatal("append modified prefix")
		}
		again, k, err := DecodeVarint(encoded[1:])
		if err != nil || again != value || k != len(encoded)-1 || k > n {
			t.Fatalf("varint roundtrip %x -> %x: %x/%d/%v", p, encoded, again, k, err)
		}
	})
}

func equalStorageValues(t *testing.T, got, want []any) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("value count %d want %d", len(got), len(want))
	}
	for i, g := range got {
		w := want[i]
		if number, ok := w.(float64); ok {
			actual, ok := g.(float64)
			if !ok || math.Float64bits(actual) != math.Float64bits(number) {
				t.Fatalf("float bits differ at column %d", i)
			}
		} else if !reflect.DeepEqual(g, w) {
			t.Fatalf("value differs at column %d: got %T want %T", i, g, w)
		}
	}
}

func FuzzRecordDecode(f *testing.F) {
	for _, p := range [][]byte{nil, {0}, {1}, {2, 0}, {2, 8}, {2, 9}, {2, 10}, {2, 11}, {2, 12}, {2, 13}, {3, 128, 128}, {7, 0, 1, 2, 8, 17, 16, 255, 1, 0, 'h', 'i', 0, 255}} {
		f.Add(p)
	}
	for _, values := range [][]any{
		{int64(math.MinInt64), int64(math.MaxInt64), -1.25, "数据库🙂é한글", []byte{0, 255}, nil},
		{math.Inf(-1), math.Inf(1), math.Copysign(0, -1), "", []byte{}},
	} {
		p, err := Encode(values)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(p)
	}
	f.Fuzz(func(t *testing.T, p []byte) {
		values, err := Decode(p)
		if err != nil {
			if !errors.Is(err, ErrCorrupt) && !errors.Is(err, ErrLimit) {
				t.Fatalf("unexpected error: %v", err)
			}
			return
		}
		encoded, err := Encode(values)
		if err != nil {
			t.Fatalf("decoded value cannot be encoded: %v", err)
		}
		again, err := Decode(encoded)
		if err != nil {
			t.Fatalf("canonical record cannot be decoded: %v", err)
		}
		equalStorageValues(t, again, values)
		// Re-encoding canonical records is byte-stable.
		canonical, err := Encode(again)
		if err != nil || !reflect.DeepEqual(canonical, encoded) {
			t.Fatalf("canonical encoding is unstable: %v", err)
		}
	})
}

func FuzzRecordRoundTrip(f *testing.F) {
	f.Add(int64(0), float64(0), "", []byte{})
	f.Add(int64(math.MinInt64), math.Copysign(0, -1), "数据库🙂é한글", []byte{0, 255, 128})
	f.Add(int64(math.MaxInt64), math.Inf(1), "\xff\x00", []byte{1})
	f.Add(int64(-129), math.NaN(), "NaN becomes SQL NULL", []byte{2})
	f.Fuzz(func(t *testing.T, i int64, r float64, s string, b []byte) {
		// Preserve the distinction between an empty SQL BLOB and SQL NULL.
		blob := make([]byte, len(b))
		copy(blob, b)
		values := []any{nil, i, r, s, blob}
		encoded, err := Encode(values)
		if errors.Is(err, ErrLimit) {
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		if math.IsNaN(r) {
			values[2] = nil
		}
		got, err := Decode(encoded)
		if err != nil {
			t.Fatal(err)
		}
		equalStorageValues(t, got, values)
	})
}
