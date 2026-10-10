package vector_test

import (
	"errors"
	"math"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

func TestSubvectorPinnedEdges(t *testing.T) {
	d := must(t, "[1,2,3,4,5]")
	h := mustHalfM9(t, "[1,2,3,4,5]")
	for _, c := range []struct {
		start, count int32
		want         string
	}{
		{1, 3, "[1,2,3]"}, {3, 2, "[3,4]"}, {-1, 3, "[1]"}, {3, 9, "[3,4,5]"},
		{1, 0, ""}, {3, -1, ""}, {-1, 2, ""}, {math.MaxInt32, 10, ""}, {3, math.MaxInt32, "[3,4,5]"},
		{-2147483644, math.MaxInt32, "[1,2]"}, {math.MinInt32, math.MaxInt32, ""}, {0, 2, "[1]"}, {5, 1, "[5]"}, {6, 1, ""},
	} {
		vd, ed := vector.Subvector(d, c.start, c.count)
		vh, eh := vector.HalfSubvector(h, c.start, c.count)
		if c.want == "" {
			if !errors.Is(ed, vector.ErrDimensions) || !errors.Is(eh, vector.ErrDimensions) || vd.Valid() || vh.Valid() {
				t.Fatal(c, vd, vh, ed, eh)
			}
			continue
		}
		if ed != nil || eh != nil || vd.String() != c.want || vh.String() != c.want {
			t.Fatal(c, vd, vh, ed, eh)
		}
	}
	if _, err := vector.Subvector(vector.Vector{}, 1, 1); !errors.Is(err, vector.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := vector.HalfSubvector(vector.HalfVector{}, 1, 1); !errors.Is(err, vector.ErrInvalid) {
		t.Fatal(err)
	}
	signed := must(t, "[1,-0,1e-45]")
	sub, err := vector.Subvector(signed, 2, 9)
	if err != nil || math.Float32bits(sub.Values()[0]) != 0x80000000 || math.Float32bits(sub.Values()[1]) != 1 {
		t.Fatal(sub, err)
	}
	half := mustHalfM9(t, "[1,-0,5.960464477539063e-8]")
	halfSub, err := vector.HalfSubvector(half, 2, 9)
	if err != nil || halfSub.Bits()[0] != 0x8000 || halfSub.Bits()[1] != 1 {
		t.Fatal(halfSub, err)
	}
}

func FuzzSubvectorBounds(f *testing.F) {
	f.Add(int32(-1), int32(3))
	f.Add(int32(math.MinInt32), int32(math.MaxInt32))
	f.Add(int32(3), int32(math.MaxInt32))
	f.Fuzz(func(t *testing.T, start, count int32) {
		v := must(t, "[1,2,3,4,5]")
		h := mustHalfM9(t, "[1,2,3,4,5]")
		d, de := vector.Subvector(v, start, count)
		half, he := vector.HalfSubvector(h, start, count)
		var want []float32
		if count > 0 {
			for i := int64(1); i <= 5; i++ {
				if i >= int64(start) && i < int64(start)+int64(count) {
					want = append(want, float32(i))
				}
			}
		}
		if len(want) == 0 {
			if de == nil || he == nil || d.Valid() || half.Valid() {
				t.Fatal(start, count, d, half)
			}
			return
		}
		w, _ := vector.New(want)
		if de != nil || he != nil || d.String() != w.String() || half.String() != w.String() {
			t.Fatal(start, count, d, half, w, de, he)
		}
	})
}
