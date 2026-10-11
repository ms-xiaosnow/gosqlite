package wal

import (
	"bytes"
	"context"
	"errors"
	"testing"
)

// Private metadata-only controls. These do not claim a caller can construct a
// proof, and do not allocate giant logs/maps or acquire native descriptors.
func TestReviewIndependentQuotaMetadata(t *testing.T) {
	l := newMemoryLog(t, MagicLittleEndian, 512)
	l.append(t, 1, 1, 1)
	l.append(t, 1, 0, 2)
	p, e := ScanPrefix(context.Background(), bytes.NewReader(l.bytes), int64(len(l.bytes)), Limits{})
	for _, tc := range []struct {
		name   string
		frames uint32
		end    int64
		work   uint64
		sector uint32
		want   error
	}{
		{"frame-exact", (1 << 20) - 7, 568, 2, 4096, nil},
		{"frame-one-over", (1 << 20) - 6, 568, 2, 4096, ErrLimit},
		{"work-exact", 1, 568, (16 << 20) - 14, 4096, nil},
		{"work-one-over", 1, 568, (16 << 20) - 13, 4096, ErrLimit},
		{"work-over-limit", 1, 568, (16 << 20) + 1, 4096, ErrLimit},
		{"aligned-no-extra", 1, 10752, 16 << 20, 512, nil},
		{"bytes-over-limit", 1, (1 << 30) + 1, 2, 4096, ErrLimit},
		{"max-frame-count", ^uint32(0), 568, 2, 4096, ErrLimit},
		{"huge-legal-frame-extent", 1, 32 + int64(^uint32(0))*65560, 2, 4096, ErrLimit},
		{"max-work", 1, 568, ^uint64(0), 4096, ErrLimit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := p.proof.snapshot
			s.FrameCount = tc.frames
			s.EndOffset = tc.end
			s.IndexWork = tc.work
			q := CommittedPrefix{&prefixProof{snapshot: s, scanError: e}}
			_, err := PlanPrefixReplacement(q, e, tc.sector)
			if tc.want == nil {
				if err != nil {
					t.Fatal(err)
				}
			} else if !errors.Is(err, tc.want) {
				t.Fatal(err)
			}
		})
	}
}
