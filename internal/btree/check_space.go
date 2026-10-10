package btree

import (
	"context"
	"encoding/binary"
	"sort"

	"github.com/ms-xiaosnow/gosqlite/internal/record"
)

// CheckTreeSpace supplements CheckOwnership with exact B-tree cell-content
// accounting. Every byte between the content boundary and usable-page end must
// belong to a parsed cell, a freeblock, or a 1–3-byte fragment; the fragment sum
// must match the page header. The unallocated gap before the content boundary
// is distinct. Overflow and freelist pages are never interpreted as tree pages.
// The caller retains the same snapshot and separately validates page ownership.
func CheckTreeSpace(ctx context.Context, r PageReader, roots []RootSpec) error {
	h := r.Header()
	if h.PageCount == 0 || h.PageCount > MaxVisitedPages {
		return record.ErrLimit
	}
	seen := make(map[uint32]bool)
	all := make([]RootSpec, 0, len(roots)+1)
	all = append(all, RootSpec{Page: 1, Kind: TableTree})
	all = append(all, roots...)
	for _, root := range all {
		stack := []uint32{root.Page}
		for len(stack) > 0 {
			if err := ctx.Err(); err != nil {
				return err
			}
			n := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if n == 0 || n > h.PageCount || seen[n] {
				return corrupt("invalid or repeated B-tree space page %d", n)
			}
			seen[n] = true
			p, err := r.ReadPage(ctx, n)
			if err != nil {
				return err
			}
			if len(p) != int(h.PageSize) {
				return corrupt("B-tree space page byte count")
			}
			children, err := checkPageSpace(p, n, int(h.UsableSize), root.Kind)
			if err != nil {
				return err
			}
			if len(children) > int(h.PageCount)-len(stack) {
				return corrupt("B-tree space child count exceeds extent")
			}
			stack = append(stack, children...)
		}
	}
	return ctx.Err()
}

func checkPageSpace(p []byte, n uint32, usable int, kind TreeKind) ([]uint32, error) {
	off := 0
	if n == 1 {
		off = 100
	}
	var ends []int
	var children []uint32
	var leaf bool
	// The existing parsers validate all offsets, cell/overflow-local lengths,
	// freeblock chains and pairwise disjointness before any slice below is used.
	switch kind {
	case TableTree:
		f, err := parsePage(p, n, usable, bounds{})
		if err != nil {
			return nil, err
		}
		leaf = f.leaf
		ends = make([]int, len(f.cells))
		for i, c := range f.cells {
			ends[i] = c.end
			if !leaf {
				children = append(children, c.child)
			}
		}
		if !leaf {
			children = append(children, f.right)
		}
	case IndexTree:
		f, err := parseIndexPage(p, n, usable)
		if err != nil {
			return nil, err
		}
		leaf = f.leaf
		ends = make([]int, len(f.cells))
		for i, c := range f.cells {
			ends[i] = c.end
			if !leaf {
				children = append(children, c.child)
			}
		}
		if !leaf {
			children = append(children, f.right)
		}
	default:
		return nil, corrupt("unknown B-tree space kind")
	}
	hdr := 8
	if !leaf {
		hdr = 12
	}
	content := int(binary.BigEndian.Uint16(p[off+5:]))
	if content == 0 {
		content = 65536
	}
	spans := make([]span, 0, len(ends))
	for i, end := range ends {
		start := int(binary.BigEndian.Uint16(p[off+hdr+2*i:]))
		spans = append(spans, span{start, end})
	}
	for at := int(binary.BigEndian.Uint16(p[off+1:])); at != 0; at = int(binary.BigEndian.Uint16(p[at:])) {
		size := int(binary.BigEndian.Uint16(p[at+2:]))
		spans = append(spans, span{at, at + size})
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
	end, fragments := content, 0
	for _, s := range spans {
		gap := s.start - end
		if gap < 0 || gap > 3 {
			return nil, corrupt("page %d unrepresented cell-content space", n)
		}
		fragments += gap
		end = s.end
	}
	tail := usable - end
	if tail < 0 || tail > 3 {
		return nil, corrupt("page %d unrepresented cell-content tail", n)
	}
	fragments += tail
	if fragments != int(p[off+7]) {
		return nil, corrupt("page %d fragmented byte count differs", n)
	}
	return children, nil
}
