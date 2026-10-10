package ann

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

func OpclassFor(kind ValueKind, metric vector.MetricKind) (string, error) {
	if kind == DenseKind {
		return Opclass(metric)
	}
	if kind == BitKind {
		switch metric {
		case vector.MetricKind(vector.Hamming):
			return "bit_hamming_ops", nil
		case vector.MetricKind(vector.Jaccard):
			return "bit_jaccard_ops", nil
		}
		return "", fmt.Errorf("%w: bit metric", ErrConfig)
	}
	if kind != HalfKind && kind != SparseKind {
		return "", ErrConfig
	}
	base, err := Opclass(metric)
	if err != nil {
		return "", err
	}
	return string(kind) + strings.TrimPrefix(base, "vector"), nil
}

// Typed policy metadata is deliberately absent from all legacy v1 envelopes.
type typedPolicies struct {
	Kind           ValueKind `json:"value_kind"`
	Codec          string    `json:"value_codec"`
	Transform      string    `json:"transform_policy"`
	SparseNonzeros int       `json:"max_sparse_nonzeros"`
	Centroid       string    `json:"centroid_policy"`
}

func policies(c TypedConfig) typedPolicies {
	p := typedPolicies{Kind: c.Kind, Transform: "identity-v1", Centroid: "none"}
	switch c.Kind {
	case HalfKind:
		p.Codec = "GSH-v1"
	case BitKind:
		p.Codec = "GSB-v1"
	case SparseKind:
		p.Codec = "GSS-v1"
		p.SparseNonzeros = MaxSparseANNNonzeros
	}
	if c.Method == IVFFlat {
		if c.Kind == HalfKind {
			p.Centroid = "f64-train-f32-to-binary16-rne-output-unit-tolerance-1e-3-v1"
		} else {
			p.Centroid = "packed-majority-retain-prior-bit-on-tie-v1"
		}
	}
	return p
}

type typedHNSWEnvelope struct {
	configEnvelope
	typedPolicies
}
type typedIVFEnvelope struct {
	ivfConfigEnvelope
	typedPolicies
}

func canonicalTypedConfig(c TypedConfig) ([]byte, error) {
	if c.Method == IVFFlat {
		e := newIVFConfigEnvelope(c.Config)
		if c.Kind == BitKind {
			e.BuildPolicy = "farthest-first-hamming-majority-v1"
			e.ZeroPolicy = "include-all-zero-bits"
			e.EmptyPolicy = "retain-prior-empty-cluster-or-tied-bit"
		}
		return json.Marshal(typedIVFEnvelope{e, policies(c)})
	}
	zero := "reject-cosine-zero"
	if c.Kind == BitKind {
		zero = "include-all-zero-bits"
	}
	return json.Marshal(typedHNSWEnvelope{configEnvelope{c.Config, "splitmix64-geometric-v1", zero, MaxLevel}, policies(c)})
}
func parseTypedConfig(data []byte, method string) (TypedConfig, error) {
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	var c TypedConfig
	if method == IVFFlat {
		var e typedIVFEnvelope
		if err := d.Decode(&e); err != nil {
			return TypedConfig{}, ErrConfig
		}
		c = TypedConfig{Kind: e.Kind, Config: Config{Method: e.Method, Version: e.Version, Dimensions: e.Dimensions, Metric: e.Metric, Opclass: e.Opclass, Seed: e.Seed, Lists: e.Lists, TrainingSamples: e.TrainingSamples, TrainingIterations: e.TrainingIterations, MaxTrainingWork: e.MaxTrainingWork, MaxNodes: e.MaxNodes, MaxVisited: e.MaxVisited, MaxDistanceEvaluations: e.MaxDistanceEvaluations, MaxMemoryBytes: e.MaxMemoryBytes}}
	} else {
		var e typedHNSWEnvelope
		if err := d.Decode(&e); err != nil {
			return TypedConfig{}, ErrConfig
		}
		c.Config = e.Config
		c.Kind = e.typedPolicies.Kind
	}
	if c.Kind == DenseKind {
		return TypedConfig{}, ErrConfig
	}
	canonical, err := c.Canonical()
	if err != nil || !bytes.Equal(canonical, data) {
		return TypedConfig{}, ErrConfig
	}
	return c, nil
}

// TypedConfig adds explicit representation identity without changing Config's
// legacy public field layout or canonical encoding. Promoted fields retain the
// same creation and persistence semantics, but validation is typed and closed.
type TypedConfig struct {
	Config
	Kind ValueKind `json:"-"`
}

func DefaultTypedConfig(method string, kind ValueKind, dimensions int, metric vector.MetricKind) (TypedConfig, error) {
	opclass, err := OpclassFor(kind, metric)
	if err != nil {
		return TypedConfig{}, err
	}
	c := TypedConfig{Config: Config{Method: method, Version: MethodVersion, Dimensions: dimensions, Metric: metric, Opclass: opclass, Seed: DefaultSeed, M: 16, EFConstruction: 64, MaxNodes: 100000, MaxTombstones: 25000, MaxVisited: 4096, MaxDistanceEvaluations: 1000000, MaxMemoryBytes: 32 << 20}}
	c.Kind = kind
	if kind != DenseKind {
		c.Version = TypedMethodVersion
	}
	if method == IVFFlat {
		c.M, c.EFConstruction, c.MaxTombstones = 0, 0, 0
		c.MaxVisited = MaxLists
		c.Lists, c.TrainingSamples, c.TrainingIterations, c.MaxTrainingWork = DefaultLists, 10000, 20, 1<<28
	}
	return c, c.Validate()
}

func (c TypedConfig) Normalize() (TypedConfig, error) {
	if c.Method == "" {
		c.Method = HNSW
	}
	d, err := DefaultTypedConfig(c.Method, c.Kind, c.Dimensions, c.Metric)
	if err != nil {
		return TypedConfig{}, err
	}
	if c.Version == 0 {
		c.Version = d.Version
	}
	if c.Opclass == "" {
		c.Opclass = d.Opclass
	}
	if c.Seed == 0 {
		c.Seed = d.Seed
	}
	if c.M == 0 {
		c.M = d.M
	}
	if c.EFConstruction == 0 {
		c.EFConstruction = d.EFConstruction
	}
	if c.MaxNodes == 0 {
		c.MaxNodes = d.MaxNodes
	}
	if c.MaxTombstones == 0 {
		c.MaxTombstones = d.MaxTombstones
		if c.MaxTombstones > c.MaxNodes {
			c.MaxTombstones = c.MaxNodes
		}
	}
	if c.MaxVisited == 0 {
		c.MaxVisited = d.MaxVisited
	}
	if c.MaxDistanceEvaluations == 0 {
		c.MaxDistanceEvaluations = d.MaxDistanceEvaluations
	}
	if c.MaxMemoryBytes == 0 {
		c.MaxMemoryBytes = d.MaxMemoryBytes
	}
	if c.Lists == 0 {
		c.Lists = d.Lists
	}
	if c.TrainingSamples == 0 {
		c.TrainingSamples = d.TrainingSamples
		if c.Method == IVFFlat && c.TrainingSamples < c.Lists {
			c.TrainingSamples = c.Lists
		}
	}
	if c.TrainingIterations == 0 {
		c.TrainingIterations = d.TrainingIterations
	}
	if c.MaxTrainingWork == 0 {
		c.MaxTrainingWork = d.MaxTrainingWork
	}
	return c, c.Validate()
}

func (c TypedConfig) Validate() error {
	opclass, err := OpclassFor(c.Kind, c.Metric)
	version, maxDims := MethodVersion, vector.ANNMaxDimensions
	switch c.Kind {
	case DenseKind:
	case HalfKind:
		version, maxDims = TypedMethodVersion, MaxHalfANNDimensions
	case BitKind:
		version, maxDims = TypedMethodVersion, vector.MaxBitDimensions
	case SparseKind:
		version, maxDims = TypedMethodVersion, vector.MaxSparseDimensions
	default:
		return ErrConfig
	}
	if err != nil || c.Version != version || c.Opclass != opclass || c.Dimensions < 1 || c.Dimensions > maxDims || c.MaxNodes < 1 || c.MaxNodes > 1<<30 || c.MaxVisited < 1 || c.MaxVisited > 1<<20 || c.MaxDistanceEvaluations < 1 || c.MaxDistanceEvaluations > 1<<30 || c.MaxMemoryBytes < 1024 || c.MaxMemoryBytes > 1<<30 {
		return ErrConfig
	}
	switch c.Method {
	case HNSW:
		if c.M < 2 || c.M > 100 || c.EFConstruction < 4 || c.EFConstruction > 1000 || c.EFConstruction < 2*c.M || c.MaxTombstones < 0 || c.MaxTombstones > c.MaxNodes || c.Lists != 0 || c.TrainingSamples != 0 || c.TrainingIterations != 0 || c.MaxTrainingWork != 0 {
			return ErrConfig
		}
	case IVFFlat:
		if c.Kind == SparseKind || c.Metric == vector.L1 || c.Metric == vector.MetricKind(vector.Jaccard) || c.M != 0 || c.EFConstruction != 0 || c.MaxTombstones != 0 || c.Lists < 1 || c.Lists > MaxLists || c.TrainingSamples < c.Lists || c.TrainingSamples > 1<<20 || c.TrainingIterations < 1 || c.TrainingIterations > 1000 || c.MaxTrainingWork < 1 || c.MaxTrainingWork > 1<<40 {
			return ErrConfig
		}
	default:
		return ErrConfig
	}
	return nil
}

func (c TypedConfig) Canonical() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if c.Kind == DenseKind {
		return c.Config.Canonical()
	}
	return canonicalTypedConfig(c)
}

// ParseTypedConfig accepts legacy dense v1 and closed typed v2 identities.
// ParseConfig remains the unchanged dense-only API.
func ParseTypedConfig(data []byte) (TypedConfig, error) {
	if len(data) > 4096 {
		return TypedConfig{}, ErrConfig
	}
	var identity struct {
		Method  string `json:"method"`
		Version int    `json:"version"`
	}
	if err := json.Unmarshal(data, &identity); err != nil {
		return TypedConfig{}, ErrConfig
	}
	if identity.Version == TypedMethodVersion {
		return parseTypedConfig(data, identity.Method)
	}
	c, err := ParseConfig(data)
	if err != nil {
		return TypedConfig{}, err
	}
	return TypedConfig{Config: c}, nil
}
