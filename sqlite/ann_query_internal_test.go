package sqlite

import (
	"errors"
	"math"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

// Memory-only identity checks. This does not modify a database or disk bytes.
func TestANNQueryTypedIdentityRecheck(t *testing.T) {
	want, err := vector.New([]float32{0, 1})
	if err != nil {
		t.Fatal(err)
	}
	negativeZero, err := vector.New([]float32{float32(math.Copysign(0, -1)), 1})
	if err != nil {
		t.Fatal(err)
	}
	good, _ := want.MarshalBinary()
	bad, _ := negativeZero.MarshalBinary()
	if _, err = decodeANNVector(good, 2, want); err != nil {
		t.Fatal(err)
	}
	for _, value := range []any{nil, bad, "[1,1]"} {
		if _, err = decodeANNVector(value, 2, want); !errors.Is(err, ErrCorrupt) {
			t.Fatal(value, err)
		}
	}
}
