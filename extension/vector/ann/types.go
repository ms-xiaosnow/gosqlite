package ann

import (
	"context"
	"fmt"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

// Reader must resolve a node in one index and one consistent snapshot. Missing
// or foreign node identities are corruption, not an absent search candidate.
type Reader interface {
	GetNode(context.Context, int64) (Node, error)
}

// Writer belongs to a caller-owned atomic statement. If any operation fails,
// the caller MUST roll back every PutNode from that operation. The algorithms
// publish *State only after every write succeeds; they cannot undo storage.
// The caller also enforces one live node for each source rowid.
type Writer interface {
	Reader
	PutNode(context.Context, Node) error
}

type Node struct {
	ID, RowID int64
	Live      bool
	Level     int
	Vector    vector.Vector
	Neighbors [][]int64
}

type State struct {
	Entry, NextID, Live, Tombstones int64
	MaxLevel                        int
	TrainingState                   string
}

func NewState() State { return State{NextID: 1, MaxLevel: -1} }

func (s State) Validate(c Config) error {
	if err := c.Validate(); err != nil {
		return err
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

// Limits independently inherit each zero field from Config. Positive values
// may tighten but never relax persisted limits. MaxBytes is a conservative
// logical retained-memory budget, not an exact Go allocator/RSS guarantee.
// The reader's own memory and one fetched node plus a transient single-node
// adjacency clone are outside it; each node is itself structurally bounded.
type Limits struct {
	MaxVisited, MaxDistanceEvaluations int
	MaxBytes                           int64
}

func (l Limits) normalized(c Config) (Limits, error) {
	if l.MaxVisited == 0 {
		l.MaxVisited = c.MaxVisited
	}
	if l.MaxDistanceEvaluations == 0 {
		l.MaxDistanceEvaluations = c.MaxDistanceEvaluations
	}
	if l.MaxBytes == 0 {
		l.MaxBytes = c.MaxMemoryBytes
	}
	if l.MaxVisited < 1 || l.MaxVisited > c.MaxVisited || l.MaxDistanceEvaluations < 1 || l.MaxDistanceEvaluations > c.MaxDistanceEvaluations || l.MaxBytes < 1 || l.MaxBytes > c.MaxMemoryBytes {
		return Limits{}, ErrConfig
	}
	return l, nil
}

type Candidate struct {
	NodeID, RowID int64
	Distance      float64
}

// Stats report actual work, including failed read/distance attempts. Cache hits
// do not increase NodeReads. Visited counts distinct retained graph nodes.
// EdgeReads counts adjacency entries considered by traversal, not structural
// codec checks. DistanceEvaluations counts metric calls, not vector validation.
// Exhausted means the discovered layer-zero frontier ended; it does not prove
// full graph coverage or exact recall. PeakBytes is logical retained accounting.
type Stats struct {
	NodeReads, EdgeReads, Visited, DistanceEvaluations int
	PeakBytes                                          int64
	Exhausted                                          bool
	StopReason                                         string
}

// NodeMemoryBytes reserves vector/adjacency bytes plus a conservative per-node
// allowance for cache maps, layer seen sets, candidate heaps and staged writes.
func NodeMemoryBytes(n Node) int64 {
	b := int64(768 + 4*n.Vector.Dims() + 24*len(n.Neighbors))
	for _, layer := range n.Neighbors {
		b += 8 * int64(len(layer))
	}
	return b
}

func layerCapacity(c Config, layer int) int {
	if layer == 0 {
		return 2 * c.M
	}
	return c.M
}

func ValidateNode(n Node, c Config) error {
	if err := ValidateVector(n.Vector, c); err != nil {
		return fmt.Errorf("%w: node vector: %w", ErrCorrupt, err)
	}
	if n.ID < 1 || n.ID > c.MaxNodes || n.Level < 0 || n.Level > MaxLevel || len(n.Neighbors) != n.Level+1 {
		return ErrCorrupt
	}
	for layer, neighbors := range n.Neighbors {
		if len(neighbors) > layerCapacity(c, layer) {
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

func cloneNode(n Node) Node {
	neighbors := make([][]int64, len(n.Neighbors))
	for i := range neighbors {
		neighbors[i] = append([]int64(nil), n.Neighbors[i]...)
	}
	n.Neighbors = neighbors
	return n
}
