package btree

import (
	"context"
	"fmt"
	"io"

	"github.com/ms-xiaosnow/gosqlite/internal/record"
)

// indexOverflowOwner identifies both the cell and the byte position in its
// overflow chain. A search probe may decode the same cell again during Next,
// but another cell cannot share its chain, nor may a chain contain a cycle.
type indexOverflowOwner struct {
	page, offset, position uint32
}

func (c *IndexCursor) readOverflowPage(n uint32, owner indexOverflowOwner) ([]byte, error) {
	previous, seen := c.overflow[n]
	if !seen {
		p, err := c.readPage(n)
		if err != nil {
			return nil, err
		}
		if c.overflow == nil {
			c.overflow = make(map[uint32]indexOverflowOwner)
		}
		c.overflow[n] = owner
		return p, nil
	}
	if previous != owner {
		return nil, corrupt("index overflow page %d has multiple owners or repeats in a chain", n)
	}
	if err := c.ctx.Err(); err != nil {
		return nil, err
	}
	// This is an already validated page number and ownership claim. Keep the
	// claim while rereading so B-tree descent still rejects it as a child page.
	p, err := c.reader.ReadPage(c.ctx, n)
	if err != nil {
		return nil, err
	}
	if err := c.ctx.Err(); err != nil {
		return nil, err
	}
	if len(p) != int(c.header.PageSize) {
		return nil, corrupt("index overflow page %d has incorrect byte count", n)
	}
	return p, nil
}

// IndexRangeCursor streams a bounded suffix of a BINARY ascending index.
// It retains the current B-tree path, a prefix, one previous key, and bounded
// visited-page/overflow-owner metadata, not the preceding records or subtrees.
// The reader must provide a coherent snapshot; methods must not run concurrently.
type IndexRangeCursor struct {
	cursor *IndexCursor
	prefix []any
}

func validateIndexRangeKey(info KeyInfo, key []any) error {
	if len(key) > info.Columns+1 {
		return fmt.Errorf("%w: index range has too many fields", record.ErrUnsupported)
	}
	if len(key) == info.Columns+1 {
		if _, ok := key[len(key)-1].(int64); !ok {
			return fmt.Errorf("%w: index range rowid must be int64", record.ErrUnsupported)
		}
	}
	return validateIndexFields(key)
}

// compareIndexPrefix compares only the supplied leading fields of a validated
// full key. Equality includes every rowid following an equal partial bound;
// no NULL, rowid minimum, or other artificial sentinel is appended.
func compareIndexPrefix(key, prefix []any) int {
	return compareIndexKeys(key[:len(prefix)], prefix)
}

// NewIndexRangeCursor seeks directly to the inclusive lowerBound and returns
// only keys whose leading fields equal prefix. Both arguments contain zero to
// info.Columns+1 actual index fields; the optional final field is an int64
// rowid. An empty lowerBound means no lower bound; an empty prefix means no
// prefix restriction. A prefix compares whole fields, not a text/BLOB substring.
// Numeric equality and storage-class order follow SQLite BINARY ASC semantics.
//
// The seek starts at the later of lowerBound and prefix, using binary search
// within each visited page. It does not scan/discard earlier keys. Partial-key
// equality descends to find earlier equal prefixes; an exact full-key interior
// hit starts at that interior record without reading its preceding child.
// Only visited keys/pages are validated, as with SeekIndex, not unrelated trees.
// Input slices and BLOBs may be reused after this call; returned keys are owned.
func NewIndexRangeCursor(ctx context.Context, r PageReader, root uint32, info KeyInfo, lowerBound, prefix []any) (*IndexRangeCursor, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateKeyInfo(info); err != nil {
		return nil, err
	}
	if err := validateIndexRangeKey(info, lowerBound); err != nil {
		return nil, err
	}
	if err := validateIndexRangeKey(info, prefix); err != nil {
		return nil, err
	}
	base, err := NewIndexCursor(ctx, r, root, info)
	if err != nil {
		return nil, err
	}
	c := &IndexRangeCursor{cursor: base, prefix: copyIndexKey(prefix)}
	target := lowerBound
	if compareIndexKeys(prefix, target) > 0 {
		target = prefix
	}
	if len(target) != 0 {
		if err := c.seek(target); err != nil {
			c.Close()
			return nil, err
		}
	}
	return c, nil
}

func (c *IndexRangeCursor) seek(target []any) error {
	b := c.cursor
	var lower, upper []any
	for {
		if err := b.ctx.Err(); err != nil {
			return err
		}
		f := &b.stack[len(b.stack)-1]
		lo, hi := 0, len(f.cells)
		for lo < hi {
			mid := lo + (hi-lo)/2
			key, err := b.decodeKey(&f.indexFrame, f.cells[mid])
			if err != nil {
				return err
			}
			if (lower != nil && compareIndexKeys(key, lower) <= 0) || (upper != nil && compareIndexKeys(key, upper) >= 0) {
				return corrupt("index range key order or parent bounds")
			}
			cmp := compareIndexPrefix(key, target)
			if cmp == 0 && len(target) == b.info.Columns+1 {
				f.next = mid
				if !f.leaf {
					f.next = 2*mid + 1
				}
				b.last = lower
				return nil
			}
			if cmp < 0 {
				lo, lower = mid+1, key
			} else {
				hi, upper = mid, key
			}
		}
		if f.leaf {
			f.next, b.last = lo, lower
			return nil
		}
		child := f.right
		if lo < len(f.cells) {
			child = f.cells[lo].child
		}
		// The selected child is consumed by seek; resume at its following
		// interior record (or past the page for the rightmost child).
		f.next = 2*lo + 1
		if err := b.push(child); err != nil {
			return err
		}
	}
}

// Next returns an independently owned complete key including its signed rowid.
// A prefix mismatch terminates the range, and EOF/errors are sticky until Close.
func (c *IndexRangeCursor) Next() ([]any, error) {
	key, err := c.cursor.Next()
	if err != nil {
		c.prefix = nil
		return nil, err
	}
	if compareIndexPrefix(key, c.prefix) != 0 {
		c.Close()
		return nil, io.EOF
	}
	if len(c.prefix) == c.cursor.info.Columns+1 {
		// A complete key, including rowid, has at most one match. Do not read
		// any later subtree merely to rediscover that the range has ended.
		c.Close()
	}
	return key, nil
}

func (c *IndexRangeCursor) Close() error {
	c.prefix = nil
	return c.cursor.Close()
}
