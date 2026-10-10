package vector_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

func TestExtendedTypeBoundaryContracts(t *testing.T) {
	h := mustHalfM9(t, "[1]")
	s := mustSparseM9(t, "{1:1}/1")
	b, _ := vector.ParseBit("1")
	d := must(t, "[1]")
	type binaryValue interface{ MarshalBinary() ([]byte, error) }
	values := []binaryValue{d, h, b, s}
	decoders := []func([]byte) error{
		func(b []byte) error { _, err := vector.Decode(b); return err },
		func(b []byte) error { _, err := vector.DecodeHalf(b); return err },
		func(b []byte) error { _, err := vector.DecodeBit(b); return err },
		func(b []byte) error { _, err := vector.DecodeSparse(b); return err },
	}
	for i, v := range values {
		encoded, err := v.MarshalBinary()
		if err != nil {
			t.Fatal(err)
		}
		for j, decode := range decoders {
			err := decode(encoded)
			if i == j && err != nil {
				t.Fatal(i, j, err)
			}
			if i != j && !errors.Is(err, vector.ErrEncoding) {
				t.Fatal("cross-kind decode", i, j, err)
			}
		}
	}
	for _, n := range []int{0, 16001} {
		if _, err := vector.NewHalfBits(make([]uint16, n)); !errors.Is(err, vector.ErrDimensions) {
			t.Fatal(n, err)
		}
	}
	if err := h.RequireDims(0); !errors.Is(err, vector.ErrDimensions) {
		t.Fatal(err)
	}
	if err := (vector.HalfVector{}).RequireDims(1); !errors.Is(err, vector.ErrInvalid) {
		t.Fatal(err)
	}
	if err := s.RequireDims(0); !errors.Is(err, vector.ErrDimensions) {
		t.Fatal(err)
	}
	if err := (vector.SparseVector{}).RequireDims(1); !errors.Is(err, vector.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := vector.ParseHalfDims("bad", 1); err == nil {
		t.Fatal("invalid half text")
	}
	if _, err := vector.ParseSparseDims("bad", 1); err == nil {
		t.Fatal("invalid sparse text")
	}
	for _, v := range []interface{ MarshalText() ([]byte, error) }{h, s} {
		text, err := v.MarshalText()
		if err != nil || len(text) == 0 {
			t.Fatal(string(text), err)
		}
	}
	for _, v := range []interface{ MarshalText() ([]byte, error) }{vector.HalfVector{}, vector.SparseVector{}} {
		if _, err := v.MarshalText(); !errors.Is(err, vector.ErrInvalid) {
			t.Fatal(err)
		}
	}
	for _, input := range []string{"{1:1", "{1:1 ;}/1"} {
		if _, err := vector.ParseSparse(input); !errors.Is(err, vector.ErrSyntax) {
			t.Fatal(input, err)
		}
	}
}

func TestExtendedMetricContextPublication(t *testing.T) {
	h := mustHalfM9(t, "[1]")
	s := mustSparseM9(t, "{1:1}/1")
	for _, run := range []func(context.Context) (float64, error){
		func(c context.Context) (float64, error) { return vector.L2.DistanceHalfContext(c, h, h) },
		func(c context.Context) (float64, error) { return vector.L2.DistanceSparseContext(c, s, s) },
		func(c context.Context) (float64, error) { return vector.HalfNormContext(c, h) },
		func(c context.Context) (float64, error) { return vector.SparseNormContext(c, s) },
	} {
		if _, err := run(nil); !errors.Is(err, vector.ErrContext) {
			t.Fatal(err)
		}
		for _, at := range []int{1, 2, 3} {
			ctx := &m9CancelAfter{Context: context.Background(), limit: at}
			got, err := run(ctx)
			if got != 0 || !errors.Is(err, context.Canceled) || ctx.checks != at {
				t.Fatal(at, got, err, ctx.checks)
			}
		}
	}
	if _, err := vector.HalfNorm(vector.HalfVector{}); !errors.Is(err, vector.ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := vector.SparseNorm(vector.SparseVector{}); !errors.Is(err, vector.ErrInvalid) {
		t.Fatal(err)
	}
}

func TestSparseRetainedEdgePolicies(t *testing.T) {
	data, err := os.ReadFile("../../testdata/pgvector-v0.8.7/extended/sparse-edge-policies.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Cases []struct {
			Input      string
			Local      *string
			LocalError string `json:"local_error"`
		}
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Cases) != 10 {
		t.Fatal("unreviewed policy coverage")
	}
	for _, c := range fixture.Cases {
		v, err := vector.ParseSparse(c.Input)
		if c.Local != nil {
			if err != nil || v.String() != *c.Local {
				t.Fatal(c.Input, v, err)
			}
			continue
		}
		var want error
		switch c.LocalError {
		case "ErrSparseIndex":
			want = vector.ErrSparseIndex
		case "ErrSparseUnderflow":
			want = vector.ErrSparseUnderflow
		default:
			t.Fatal(c.LocalError)
		}
		if !errors.Is(err, want) || v.Valid() {
			t.Fatal(c.Input, v, err)
		}
	}
}
