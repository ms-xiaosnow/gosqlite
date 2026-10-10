package ann

import (
	"container/heap"
	"context"
	"fmt"
	"sort"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

// SamplePriority fixes the version-one, platform-independent sampling policy.
// Keep the smallest (priority, signed rowid) pairs. The seed is persisted.
func SamplePriority(seed uint64, rowID int64) uint64 {
	x := seed ^ (uint64(rowID) * 0xd6e8feb86659fd93)
	x += 0x9e3779b97f4a7c15
	x = (x ^ (x >> 30)) * 0xbf58476d1ce4e5b9
	x = (x ^ (x >> 27)) * 0x94d049bb133111eb
	return x ^ (x >> 31)
}

type sampleEntry struct {
	sample   Sample
	priority uint64
}
type sampleHeap []sampleEntry

func (h sampleHeap) Len() int { return len(h) }
func (h sampleHeap) Less(i, j int) bool {
	if h[i].priority != h[j].priority {
		return h[i].priority > h[j].priority
	}
	return h[i].sample.RowID > h[j].sample.RowID
}
func (h sampleHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *sampleHeap) Push(x any)   { *h = append(*h, x.(sampleEntry)) }
func (h *sampleHeap) Pop() any {
	a := *h
	n := len(a) - 1
	x := a[n]
	a[n] = sampleEntry{}
	*h = a[:n]
	return x
}

// IVFSampler retains a bounded bottom-hash sample without allocating the full
// cap up front. Add requires strictly increasing signed source rowids (including
// IP zeros), so duplicate identities are rejected without an unbounded map.
// It owns no source cursor and is not safe for concurrent mutation.
type IVFSampler struct {
	cfg     Config
	entries sampleHeap
	count   int64
	last    int64
	closed  bool
}

func NewIVFSampler(c Config) (*IVFSampler, error) {
	if err := validateIVF(c); err != nil {
		return nil, err
	}
	return &IVFSampler{cfg: c}, nil
}
func (s *IVFSampler) Add(ctx context.Context, rowID int64, v vector.Vector) error {
	if err := ivfContext(ctx); err != nil {
		return err
	}
	if s == nil || s.closed {
		return ErrConfig
	}
	if s.count > 0 && rowID <= s.last {
		return vector.ErrDuplicateRowID
	}
	if s.count >= s.cfg.MaxNodes {
		return fmt.Errorf("%w: sampled source rows", ErrLimit)
	}
	ok, err := Trainable(v, s.cfg)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if !ok {
		s.count++
		s.last = rowID
		return nil
	}
	entry := sampleEntry{Sample{rowID, v}, SamplePriority(s.cfg.Seed, rowID)}
	if len(s.entries) < s.cfg.TrainingSamples {
		if 1024+int64(len(s.entries)+1)*SampleMemoryBytes(s.cfg) > s.cfg.MaxMemoryBytes {
			return fmt.Errorf("%w: training sample memory", ErrLimit)
		}
		heap.Push(&s.entries, entry)
	} else if entry.priority < s.entries[0].priority || (entry.priority == s.entries[0].priority && rowID < s.entries[0].sample.RowID) {
		s.entries[0] = entry
		heap.Fix(&s.entries, 0)
	}
	s.count++
	s.last = rowID
	return nil
}

// Samples drains the sampler and returns rowid-sorted immutable original
// vectors. The sampler cannot be used again, avoiding retained duplicate state.
func (s *IVFSampler) Samples(ctx context.Context) ([]Sample, error) {
	if err := ivfContext(ctx); err != nil {
		return nil, err
	}
	if s == nil || s.closed {
		return nil, ErrConfig
	}
	samples := make([]Sample, len(s.entries))
	for i, e := range s.entries {
		if i&255 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}
		samples[i] = e.sample
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i].RowID < samples[j].RowID })
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.entries = nil
	s.closed = true
	return samples, nil
}
