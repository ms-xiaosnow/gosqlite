// Package ann implements independently written, deterministic HNSW algorithms
// over transaction-owned storage ports. It owns no files, commits, or SQL.
//
// Configuration and bounds follow pgvector v0.8.7's dense HNSW parameters, but
// topology, floating-point arithmetic, deletion policy and storage are local.
// Version 1 fixes geometric levels, a maximum level of 32, signed rowid ties,
// and rejection of zero-norm values for cosine indexes. Tombstones retain
// their original vectors and remain traversable until a caller-owned rebuild.
package ann

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

const (
	HNSW                   = "hnsw"
	MethodVersion          = 1
	MaxLevel               = 32
	DefaultEFSearch        = 40
	MaxEFSearch            = 1000
	DefaultSeed     uint64 = 0x243f6a8885a308d3
)

var (
	ErrConfig  = errors.New("ann: invalid configuration")
	ErrCorrupt = errors.New("ann: invalid graph state")
	ErrLimit   = errors.New("ann: resource limit exceeded; reindex or increase limit")
)

// Config is persisted in full. Validate and Canonical never fill defaults;
// Normalize is an explicit convenience for creating a new configuration only.
type Config struct {
	Method                 string            `json:"method"`
	Version                int               `json:"version"`
	Dimensions             int               `json:"dimensions"`
	Metric                 vector.MetricKind `json:"metric"`
	Opclass                string            `json:"opclass"`
	Seed                   uint64            `json:"seed"`
	M                      int               `json:"m"`
	EFConstruction         int               `json:"ef_construction"`
	MaxNodes               int64             `json:"max_nodes"`
	MaxTombstones          int64             `json:"max_tombstones"`
	MaxVisited             int               `json:"max_visited"`
	MaxDistanceEvaluations int               `json:"max_distance_evaluations"`
	MaxMemoryBytes         int64             `json:"max_memory_bytes"`
}

func Opclass(metric vector.MetricKind) (string, error) {
	switch metric {
	case vector.L2:
		return "vector_l2_ops", nil
	case vector.L1:
		return "vector_l1_ops", nil
	case vector.NegativeIP:
		return "vector_ip_ops", nil
	case vector.Cosine:
		return "vector_cosine_ops", nil
	default:
		return "", fmt.Errorf("%w: metric", ErrConfig)
	}
}

func DefaultConfig(method string, dimensions int, metric vector.MetricKind) (Config, error) {
	opclass, err := Opclass(metric)
	if err != nil {
		return Config{}, err
	}
	c := Config{Method: method, Version: MethodVersion, Dimensions: dimensions, Metric: metric, Opclass: opclass, Seed: DefaultSeed, M: 16, EFConstruction: 64, MaxNodes: 100000, MaxTombstones: 25000, MaxVisited: 4096, MaxDistanceEvaluations: 1000000, MaxMemoryBytes: 32 << 20}
	return c, c.Validate()
}

// Normalize fills zero creation options, including Seed and MaxTombstones.
// To intentionally choose seed=0 or forbid tombstones, modify DefaultConfig
// and call Validate instead. Reading persisted configuration uses ParseConfig.
func (c Config) Normalize() (Config, error) {
	if c.Method == "" {
		c.Method = HNSW
	}
	d, err := DefaultConfig(c.Method, c.Dimensions, c.Metric)
	if err != nil {
		return Config{}, err
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
	return c, c.Validate()
}

func (c Config) Validate() error {
	opclass, err := Opclass(c.Metric)
	if err != nil || c.Method != HNSW || c.Version != MethodVersion || c.Opclass != opclass || c.Dimensions < 1 || c.Dimensions > vector.ANNMaxDimensions || c.M < 2 || c.M > 100 || c.EFConstruction < 4 || c.EFConstruction > 1000 || c.EFConstruction < 2*c.M || c.MaxNodes < 1 || c.MaxNodes > 1<<30 || c.MaxTombstones < 0 || c.MaxTombstones > c.MaxNodes || c.MaxVisited < 1 || c.MaxVisited > 1<<20 || c.MaxDistanceEvaluations < 1 || c.MaxDistanceEvaluations > 1<<30 || c.MaxMemoryBytes < 1024 || c.MaxMemoryBytes > 1<<30 {
		return ErrConfig
	}
	return nil
}

type configEnvelope struct {
	Config
	BuildPolicy string `json:"build_policy"`
	ZeroPolicy  string `json:"zero_policy"`
	MaxLevel    int    `json:"max_level"`
}

func (c Config) Canonical() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	return json.Marshal(configEnvelope{c, "splitmix64-geometric-v1", "reject-cosine-zero", MaxLevel})
}

// ParseConfig accepts the exact canonical encoding only. Missing/duplicate
// keys, unknown fields, altered policies, alternate ordering and trailing data
// fail closed rather than silently changing the identity of an existing index.
func ParseConfig(data []byte) (Config, error) {
	if len(data) > 4096 {
		return Config{}, ErrConfig
	}
	var envelope configEnvelope
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&envelope); err != nil {
		return Config{}, ErrConfig
	}
	canonical, err := envelope.Config.Canonical()
	if err != nil || !bytes.Equal(data, canonical) {
		return Config{}, ErrConfig
	}
	return envelope.Config, nil
}

func ValidateVector(v vector.Vector, c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if err := v.RequireDims(c.Dimensions); err != nil {
		return err
	}
	if c.Metric == vector.Cosine {
		n, err := vector.Norm(v)
		if err != nil {
			return err
		}
		if n == 0 {
			return vector.ErrZeroNorm
		}
	}
	return nil
}
