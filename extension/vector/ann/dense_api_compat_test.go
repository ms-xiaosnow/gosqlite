package ann_test

import (
	"context"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
	"github.com/ms-xiaosnow/gosqlite/extension/vector/ann"
)

// This external-package compile fixture intentionally uses positional literals.
// New typed capabilities must not add fields to the stable dense public structs.
func TestDensePublicStructAndFunctionSourceCompatibility(t *testing.T) {
	v, err := vector.New([]float32{1, 2})
	if err != nil {
		t.Fatal(err)
	}
	c := ann.Config{ann.HNSW, 1, 2, vector.L2, "vector_l2_ops", ann.DefaultSeed, 16, 64, 100000, 25000, 4096, 1000000, 32 << 20, 0, 0, 0, 0}
	n := ann.Node{1, -7, true, 0, v, [][]int64{nil}}
	_ = ann.Sample{-7, v}
	_ = ann.Centroid{1, v}
	_ = ann.Limits{4096, 1000000, 32 << 20}
	_ = ann.Stats{0, 0, 0, 0, 0, false, ""}
	_ = ann.State{0, 1, 0, 0, -1, ""}
	if err := ann.ValidateNode(n, c); err != nil {
		t.Fatal(err)
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	var _ func(context.Context, ann.Writer, *ann.State, int64, vector.Vector, ann.Config) error = ann.Insert
	var _ func(context.Context, ann.Reader, ann.State, vector.Vector, ann.Config, int, ann.Limits) ([]ann.Candidate, ann.Stats, error) = ann.Search
	var _ func(context.Context, []ann.Sample, ann.Config) ([]ann.Centroid, ann.TrainingStats, bool, error) = ann.Train
	var _ func(context.Context, vector.Vector, []ann.Centroid, ann.Config, ann.Limits) (int, ann.Stats, error) = ann.Assign
	var _ func(context.Context, vector.Vector, []ann.Centroid, ann.Config, ann.Limits) ([]ann.ProbeResult, ann.Stats, error) = ann.Probe
	if got := ann.SampleMemoryBytes(c); got != 136 {
		t.Fatalf("legacy sample memory changed: %d", got)
	}
}
