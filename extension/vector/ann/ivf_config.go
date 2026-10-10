package ann

import (
	"bytes"
	"encoding/json"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

// IVF has its own closed encoding: extending it cannot change HNSW hashes.
// Policy strings version all local choices which can affect persisted lists.
type ivfConfigEnvelope struct {
	Method                 string            `json:"method"`
	Version                int               `json:"version"`
	Dimensions             int               `json:"dimensions"`
	Metric                 vector.MetricKind `json:"metric"`
	Opclass                string            `json:"opclass"`
	Seed                   uint64            `json:"seed"`
	Lists                  int               `json:"lists"`
	TrainingSamples        int               `json:"training_samples"`
	TrainingIterations     int               `json:"training_iterations"`
	MaxTrainingWork        int64             `json:"max_training_work"`
	MaxNodes               int64             `json:"max_nodes"`
	MaxVisited             int               `json:"max_visited"`
	MaxDistanceEvaluations int               `json:"max_distance_evaluations"`
	MaxMemoryBytes         int64             `json:"max_memory_bytes"`
	BuildPolicy            string            `json:"build_policy"`
	SamplePolicy           string            `json:"sample_policy"`
	ZeroPolicy             string            `json:"zero_policy"`
	EmptyPolicy            string            `json:"empty_policy"`
}

func newIVFConfigEnvelope(c Config) ivfConfigEnvelope {
	return ivfConfigEnvelope{c.Method, c.Version, c.Dimensions, c.Metric, c.Opclass, c.Seed, c.Lists, c.TrainingSamples, c.TrainingIterations, c.MaxTrainingWork, c.MaxNodes, c.MaxVisited, c.MaxDistanceEvaluations, c.MaxMemoryBytes,
		"farthest-first-lloyd-f64-spherical-v1", "splitmix64-bottom-hash-rowid-v1", "reject-cosine-zero-skip-ip-zero-training", "retain-prior-empty-or-zero-sum"}
}

func parseIVFConfig(data []byte) (Config, error) {
	var e ivfConfigEnvelope
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&e); err != nil {
		return Config{}, ErrConfig
	}
	c := Config{Method: e.Method, Version: e.Version, Dimensions: e.Dimensions, Metric: e.Metric, Opclass: e.Opclass, Seed: e.Seed, Lists: e.Lists, TrainingSamples: e.TrainingSamples, TrainingIterations: e.TrainingIterations, MaxTrainingWork: e.MaxTrainingWork, MaxNodes: e.MaxNodes, MaxVisited: e.MaxVisited, MaxDistanceEvaluations: e.MaxDistanceEvaluations, MaxMemoryBytes: e.MaxMemoryBytes}
	canonical, err := c.Canonical()
	if err != nil || !bytes.Equal(data, canonical) {
		return Config{}, ErrConfig
	}
	return c, nil
}
