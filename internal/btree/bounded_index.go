package btree

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"unsafe"

	"github.com/ms-xiaosnow/gosqlite/internal/record"
)

// IndexBound is a composite leading-key endpoint in SQLite storage-class and
// BINARY ASC order. Key has 1..Columns+1 fields; its optional final rowid must be
// int64. For a partial key, inclusive includes every matching suffix; exclusive
// excludes every matching suffix. nil *IndexBound means no endpoint. This is
// field-prefix comparison, not a text/BLOB substring and not SQL NULL logic.
type IndexBound struct {
	Key       []any
	Inclusive bool
}

type boundedIndexFrame struct {
	indexFrame
	next         int
	lower, upper []any // strict full-key bounds inherited from ancestor separators
}

// BoundedIndexCursor performs a native seek and ordered forward/reverse walk.
// It retains only its path, copied endpoints, one last key and capped ownership
// metadata. Interior index cells are records and are returned exactly once.
// The reader supplies a coherent snapshot. Methods are not concurrent-safe.
type BoundedIndexCursor struct {
	base         *IndexCursor
	budget       *rangeBudget
	stack        []boundedIndexFrame
	lower, upper *IndexBound
	reverse      bool
	terminal     error
}

// compareIndexEdges orders cuts immediately before/after all keys with a
// prefix. Unlike ordinary slice comparison, a shorter 'after' cut follows all
// its longer suffix cuts. This also detects empty mixed-length intervals.
func compareIndexEdges(a []any, afterA bool, b []any, afterB bool) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	if cmp := compareIndexKeys(a[:n], b[:n]); cmp != 0 {
		return cmp
	}
	if len(a) < len(b) {
		if afterA {
			return 1
		}
		return -1
	}
	if len(a) > len(b) {
		if afterB {
			return -1
		}
		return 1
	}
	if afterA == afterB {
		return 0
	}
	if afterA {
		return 1
	}
	return -1
}

func NewBoundedIndexCursor(ctx context.Context, r PageReader, root uint32, info KeyInfo, lower, upper *IndexBound, reverse bool, limits RangeLimits) (*BoundedIndexCursor, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateKeyInfo(info); err != nil {
		return nil, err
	}
	for _, bound := range []*IndexBound{lower, upper} {
		if bound != nil {
			if len(bound.Key) == 0 {
				return nil, fmt.Errorf("%w: empty index bound", record.ErrUnsupported)
			}
			if err := validateIndexRangeKey(info, bound.Key); err != nil {
				return nil, err
			}
		}
	}
	b, err := newRangeBudget(ctx, r, limits)
	if err != nil {
		return nil, err
	}
	c := &BoundedIndexCursor{budget: b, reverse: reverse}
	if lower != nil && upper != nil && compareIndexEdges(lower.Key, !lower.Inclusive, upper.Key, upper.Inclusive) >= 0 {
		c.terminal = io.EOF
		return c, nil
	}
	// Admit both copies before allocating either one. Work accounts for the
	// retained endpoint struct, interface slice, and cloned BLOB data. Strings
	// are immutable and shared. PayloadBytes continues to count decode payloads.
	var boundBytes int64
	addBoundBytes := func(n int64) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		// Subtract from remaining credit before adding, so even a MaxInt64
		// configured budget cannot overflow the cumulative admission counter.
		if n < 0 || n > b.limits.Work-b.stats.Work-boundBytes {
			return fmt.Errorf("%w: range endpoint-copy work budget", record.ErrLimit)
		}
		boundBytes += n
		return nil
	}
	for _, bound := range []*IndexBound{lower, upper} {
		if bound != nil {
			// validateIndexRangeKey already caps the field count before this product.
			if err := addBoundBytes(int64(unsafe.Sizeof(IndexBound{})) + int64(len(bound.Key))*int64(unsafe.Sizeof(any(nil)))); err != nil {
				return nil, err
			}
			for _, field := range bound.Key {
				if blob, ok := field.([]byte); ok {
					if err := addBoundBytes(int64(len(blob))); err != nil {
						return nil, err
					}
				}
			}
		}
	}
	if err := b.work(boundBytes); err != nil {
		return nil, err
	}
	if lower != nil {
		c.lower = &IndexBound{copyIndexKey(lower.Key), lower.Inclusive}
	}
	if upper != nil {
		c.upper = &IndexBound{copyIndexKey(upper.Key), upper.Inclusive}
	}
	h := r.Header()
	if h.PageSize < 512 || h.PageSize > 65536 || h.PageSize&(h.PageSize-1) != 0 || h.UsableSize < 480 || h.UsableSize > h.PageSize || h.PageCount == 0 {
		return nil, corrupt("invalid bounded-index PageReader header")
	}
	c.base = &IndexCursor{ctx: ctx, reader: b, header: h, root: root, info: info, visited: make(map[uint32]struct{})}
	if err = c.push(root, nil, nil); err == nil {
		err = c.seek()
	}
	if err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

func (c *BoundedIndexCursor) push(n uint32, lower, upper []any) error {
	b := c.base
	if len(c.stack) >= MaxDepth {
		return fmt.Errorf("%w: bounded-index B-tree depth", record.ErrLimit)
	}
	p, err := b.readPage(n)
	if err != nil {
		return err
	}
	off := 0
	if n == 1 {
		off = 100
	}
	usable := int(b.header.UsableSize)
	if len(p) < off+8 {
		return corrupt("bounded-index page header truncated")
	}
	count := int(binary.BigEndian.Uint16(p[off+3 : off+5]))
	// A conservative admission bound covers exact cell capacity plus every
	// possible nonoverlapping four-byte freeblock/span, before parser allocation.
	// Overflow pages do not come through push and do not pay parser admission.
	if count > (usable-off-8)/2 {
		return corrupt("bounded-index cell pointer array")
	}
	bytes := int64(count)*int64(unsafe.Sizeof(indexCell{})) + 2*int64(count+usable/4)*int64(unsafe.Sizeof(span{})) + 512
	if err = c.budget.BeforePage(bytes); err != nil {
		return err
	}
	f, err := parseIndexPage(p, n, usable)
	if err != nil {
		return err
	}
	if len(f.cells) == 0 && (n != b.root || !f.leaf && n != 1) {
		return corrupt("invalid empty B-tree page %d", n)
	}
	if f.leaf {
		depth := len(c.stack) + 1
		if b.leafDepth != 0 && b.leafDepth != depth {
			return corrupt("B-tree leaves have unequal depths")
		}
		b.leafDepth = depth
	}
	next := 0
	if c.reverse {
		next = 2 * len(f.cells)
		if f.leaf {
			next = len(f.cells) - 1
		}
	}
	c.stack = append(c.stack, boundedIndexFrame{indexFrame: f, next: next, lower: lower, upper: upper})
	return nil
}
func (c *BoundedIndexCursor) decode(f *boundedIndexFrame, i int) ([]any, error) {
	item := f.cells[i]
	if err := c.budget.BeforeRecord(item.payload); err != nil {
		return nil, err
	}
	key, err := c.base.decodeKey(&f.indexFrame, item)
	if err != nil {
		return nil, err
	}
	if f.lower != nil && compareIndexKeys(key, f.lower) <= 0 || f.upper != nil && compareIndexKeys(key, f.upper) >= 0 {
		return nil, corrupt("bounded-index key order or parent bounds")
	}
	return key, nil
}
func (c *BoundedIndexCursor) seek() error {
	target := c.lower
	if c.reverse {
		target = c.upper
	}
	for {
		if err := c.base.ctx.Err(); err != nil {
			return err
		}
		f := &c.stack[len(c.stack)-1]
		lo, hi := 0, len(f.cells)
		lower, upper := f.lower, f.upper
		if target == nil {
			if c.reverse {
				lo = hi
			}
		} else {
			for lo < hi {
				if err := c.budget.work(1); err != nil {
					return err
				}
				mid := lo + (hi-lo)/2
				key, err := c.decode(f, mid)
				if err != nil {
					return err
				}
				if lower != nil && compareIndexKeys(key, lower) <= 0 || upper != nil && compareIndexKeys(key, upper) >= 0 {
					return corrupt("bounded-index seek probe order")
				}
				cmp := compareIndexPrefix(key, target.Key)
				if cmp == 0 && target.Inclusive && len(target.Key) == c.base.info.Columns+1 {
					f.next = mid
					if !f.leaf {
						f.next = 2*mid + 1
					}
					return nil
				}
				before := cmp < 0 || cmp == 0 && !target.Inclusive
				if c.reverse {
					before = cmp < 0 || cmp == 0 && target.Inclusive
				}
				if before {
					lo, lower = mid+1, key
				} else {
					hi, upper = mid, key
				}
			}
		}
		if f.leaf {
			f.next = lo
			if c.reverse {
				f.next--
			}
			return nil
		}
		n := f.right
		if lo < len(f.cells) {
			n = f.cells[lo].child
		}
		// With an absent endpoint, obtain the one adjacent separator that supplies
		// the extreme child's strict parent bound. No unrelated child is visited.
		if target == nil && len(f.cells) > 0 {
			var err error
			if c.reverse {
				lower, err = c.decode(f, len(f.cells)-1)
			} else {
				upper, err = c.decode(f, 0)
			}
			if err != nil {
				return err
			}
		}
		f.next = 2*lo + 1
		if c.reverse {
			f.next = 2*lo - 1
		}
		if err := c.push(n, lower, upper); err != nil {
			return err
		}
	}
}
func (c *BoundedIndexCursor) finish(err error) {
	c.terminal = err
	c.stack = nil
	c.lower, c.upper = nil, nil
	if c.base != nil {
		c.base.Close()
	}
}

// Next returns a caller-owned complete key, including signed rowid. EOF and
// errors release traversal state and remain sticky until Close.
func (c *BoundedIndexCursor) Next() (key []any, err error) {
	if c.terminal != nil {
		return nil, c.terminal
	}
	defer func() {
		if err != nil {
			c.finish(err)
		}
	}()
	for len(c.stack) > 0 {
		if err = c.base.ctx.Err(); err != nil {
			return nil, err
		}
		f := &c.stack[len(c.stack)-1]
		i := f.next
		if f.leaf {
			if i < 0 || i >= len(f.cells) {
				c.stack = c.stack[:len(c.stack)-1]
				continue
			}
			if c.reverse {
				f.next--
			} else {
				f.next++
			}
		} else {
			if i < 0 || i > 2*len(f.cells) {
				c.stack = c.stack[:len(c.stack)-1]
				continue
			}
			if c.reverse {
				f.next--
			} else {
				f.next++
			}
			step := i
			i /= 2
			if step%2 == 0 {
				n := f.right
				lower, upper := f.lower, f.upper
				if i < len(f.cells) {
					n = f.cells[i].child
					upper, err = c.decode(f, i)
					if err != nil {
						return nil, err
					}
				}
				if i > 0 {
					lower, err = c.decode(f, i-1)
					if err != nil {
						return nil, err
					}
				}
				if lower != nil && upper != nil && compareIndexKeys(lower, upper) >= 0 {
					return nil, corrupt("bounded-index separator order")
				}
				// Strict child bounds can terminate even before visiting that child.
				if !c.reverse && c.upper != nil && lower != nil {
					cmp := compareIndexPrefix(lower, c.upper.Key)
					if cmp > 0 || cmp == 0 && (!c.upper.Inclusive || len(c.upper.Key) == c.base.info.Columns+1) {
						return nil, io.EOF
					}
				}
				if c.reverse && c.lower != nil && upper != nil {
					cmp := compareIndexPrefix(upper, c.lower.Key)
					if cmp < 0 || cmp == 0 && (!c.lower.Inclusive || len(c.lower.Key) == c.base.info.Columns+1) {
						return nil, io.EOF
					}
				}
				if err = c.push(n, lower, upper); err != nil {
					return nil, err
				}
				continue
			}
		}
		key, err = c.decode(f, i)
		if err != nil {
			return nil, err
		}
		if c.lower != nil {
			cmp := compareIndexPrefix(key, c.lower.Key)
			if cmp < 0 || cmp == 0 && !c.lower.Inclusive {
				return nil, io.EOF
			}
		}
		if c.upper != nil {
			cmp := compareIndexPrefix(key, c.upper.Key)
			if cmp > 0 || cmp == 0 && !c.upper.Inclusive {
				return nil, io.EOF
			}
		}
		if c.base.last != nil {
			cmp := compareIndexKeys(c.base.last, key)
			if !c.reverse && cmp >= 0 || c.reverse && cmp <= 0 {
				return nil, corrupt("bounded-index global key order")
			}
		}
		c.base.last = copyIndexKey(key)
		c.budget.stats.Returned++
		end := c.upper
		if c.reverse {
			end = c.lower
		}
		if end != nil && len(end.Key) == c.base.info.Columns+1 && compareIndexKeys(key, end.Key) == 0 {
			c.finish(io.EOF)
		}
		return key, nil
	}
	return nil, io.EOF
}
func (c *BoundedIndexCursor) Stats() RangeStats { return c.budget.stats }
func (c *BoundedIndexCursor) Close() error      { c.finish(io.EOF); return nil }

// Preserve compile-time snapshot compatibility without changing existing types.
var _ PageReader = (*rangeBudget)(nil)
