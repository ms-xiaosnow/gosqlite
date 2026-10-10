package ann

import (
	"context"
	"fmt"
)

// TypedReader must resolve a node in one index and one consistent snapshot. Missing
// or foreign node identities are corruption, not an absent search candidate.
type TypedReader interface {
	GetNode(context.Context, int64) (TypedNode, error)
}

// TypedWriter belongs to a caller-owned atomic statement. If any operation fails,
// the caller MUST roll back every PutNode from that operation. The algorithms
// publish *State only after every write succeeds; they cannot undo storage.
// The caller also enforces one live node for each source rowid.
type TypedWriter interface {
	TypedReader
	PutNode(context.Context, TypedNode) error
}

type TypedNode struct {
	ID, RowID int64
	Live      bool
	Level     int
	Value     Value
	Neighbors [][]int64
}

func (s State) ValidateTyped(c TypedConfig) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if c.Method == IVFFlat {
		if s.Entry != 0 || s.MaxLevel != -1 || s.NextID != 1 || s.Tombstones != 0 || s.Live < 0 || s.Live > c.MaxNodes || (s.TrainingState != "ready" && s.TrainingState != "untrained") {
			return ErrCorrupt
		}
		return nil
	}
	if s.Live < 0 || s.Tombstones < 0 || s.Live > c.MaxNodes || s.Tombstones > c.MaxTombstones || s.Live+s.Tombstones > c.MaxNodes || s.NextID != s.Live+s.Tombstones+1 || s.TrainingState != "" {
		return ErrCorrupt
	}
	if s.NextID == 1 {
		if s.Entry != 0 || s.MaxLevel != -1 {
			return ErrCorrupt
		}
		return nil
	}
	if s.Entry < 1 || s.Entry >= s.NextID || s.MaxLevel < 0 || s.MaxLevel > MaxLevel {
		return ErrCorrupt
	}
	return nil
}

// TypedLimits independently inherit each zero field from TypedConfig. Positive values
// may tighten but never relax persisted limits. MaxBytes is a conservative
// logical retained-memory budget, not an exact Go allocator/RSS guarantee.
// The reader's own memory and one fetched node plus a transient single-node
// adjacency clone are outside it; each node is itself structurally bounded.
type TypedLimits struct {
	Limits
	MaxComponentWork     uint64
	ComponentWorkLimited bool
}

func (l TypedLimits) normalized(c TypedConfig) (TypedLimits, error) {
	base, err := l.Limits.normalized(c.Config)
	if err != nil {
		return TypedLimits{}, err
	}
	l.Limits = base
	return l, nil
}

// TypedStats report actual work, including failed read/distance attempts. Cache hits
// do not increase NodeReads. Visited counts distinct retained graph nodes.
// EdgeReads counts adjacency entries considered by traversal, not structural
// codec checks. DistanceEvaluations counts metric calls, not vector validation.
// ComponentWork charges dimensions for dense/half, packed bytes for bit, and
// both supports summed for sparse metric attempts (including failed attempts).
// Validation is excluded. Exhausted means the discovered layer-zero frontier ended; it does not prove
// full graph coverage or exact recall. PeakBytes is logical retained accounting.
type TypedStats struct {
	Stats
	ComponentWork int64
}

// NodeMemoryBytes reserves vector/adjacency bytes plus a conservative per-node
// allowance for cache maps, layer seen sets, candidate heaps and staged writes.
func TypedNodeMemoryBytes(n TypedNode) int64 {
	b := int64(896+24*len(n.Neighbors)) + NodeValue(n).MemoryBytes()
	for _, layer := range n.Neighbors {
		b += 8 * int64(len(layer))
	}
	return b
}

func ValidateTypedNode(n TypedNode, c TypedConfig) error {
	if c.Method != HNSW {
		return ErrConfig
	}
	if err := ValidateValue(NodeValue(n), c); err != nil {
		return fmt.Errorf("%w: node vector: %w", ErrCorrupt, err)
	}
	if n.ID < 1 || n.ID > c.MaxNodes || n.Level < 0 || n.Level > MaxLevel || len(n.Neighbors) != n.Level+1 {
		return ErrCorrupt
	}
	for layer, neighbors := range n.Neighbors {
		if len(neighbors) > layerCapacity(c.Config, layer) {
			return ErrCorrupt
		}
		var prior int64
		for _, id := range neighbors {
			if id < 1 || id > c.MaxNodes || id == n.ID || id <= prior {
				return ErrCorrupt
			}
			prior = id
		}
	}
	return nil
}

func cloneTypedNode(n TypedNode) TypedNode {
	neighbors := make([][]int64, len(n.Neighbors))
	for i := range neighbors {
		neighbors[i] = append([]int64(nil), n.Neighbors[i]...)
	}
	n.Neighbors = neighbors
	return n
}

// NodeValue returns a typed node's immutable value, including explicit DenseValue.
func NodeValue(n TypedNode) Value { return n.Value }
