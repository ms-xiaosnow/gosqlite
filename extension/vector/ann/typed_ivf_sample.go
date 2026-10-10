package ann

import (
	"container/heap"
	"context"
	"fmt"
	"sort"

	"github.com/ms-xiaosnow/gosqlite/extension/vector"
)

type typedSampleEntry struct {
	sample   TypedSample
	priority uint64
}
type typedSampleHeap []typedSampleEntry

func (h typedSampleHeap) Len() int { return len(h) }
func (h typedSampleHeap) Less(i, j int) bool {
	if h[i].priority != h[j].priority {
		return h[i].priority > h[j].priority
	}
	return h[i].sample.RowID > h[j].sample.RowID
}
func (h typedSampleHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *typedSampleHeap) Push(x any)   { *h = append(*h, x.(typedSampleEntry)) }
func (h *typedSampleHeap) Pop() any {
	a := *h
	n := len(a) - 1
	x := a[n]
	a[n] = typedSampleEntry{}
	*h = a[:n]
	return x
}

// TypedIVFSampler retains a bounded bottom-hash sample without allocating the full
// cap up front. Add requires strictly increasing signed source rowids (including
// IP zeros), so duplicate identities are rejected without an unbounded map.
// It owns no source cursor and is not safe for concurrent mutation.
type TypedIVFSampler struct {
	cfg     TypedConfig
	entries typedSampleHeap
	count   int64
	last    int64
	closed  bool
}

func NewTypedIVFSampler(c TypedConfig) (*TypedIVFSampler, error) {
	if err := validateTypedIVF(c); err != nil {
		return nil, err
	}
	return &TypedIVFSampler{cfg: c}, nil
}
func (s *TypedIVFSampler) AddValue(ctx context.Context, rowID int64, v Value) error {
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
	ok, err := trainableValueContext(ctx, v, s.cfg)
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
	entry := typedSampleEntry{newTypedSample(rowID, v), SamplePriority(s.cfg.Seed, rowID)}
	if len(s.entries) < s.cfg.TrainingSamples {
		if 1024+int64(len(s.entries)+1)*TypedSampleMemoryBytes(s.cfg) > s.cfg.MaxMemoryBytes {
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
func (s *TypedIVFSampler) Samples(ctx context.Context) ([]TypedSample, error) {
	if err := ivfContext(ctx); err != nil {
		return nil, err
	}
	if s == nil || s.closed {
		return nil, ErrConfig
	}
	samples := make([]TypedSample, len(s.entries))
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
