package btree

import (
	"context"
	"fmt"
	"io"

	"github.com/ms-xiaosnow/gosqlite/internal/record"
)

// RowIDBound is a signed rowid endpoint. A nil endpoint means unbounded;
// Inclusive distinguishes >=/<= from >/< without incrementing int64 extrema.
type RowIDBound struct {
	Value     int64
	Inclusive bool
}

// RowIDRangeCursor seeks a native table B-tree and streams a bounded interval
// in either signed-rowid direction. It does not materialize or sort the table.
// The snapshot must remain coherent, and methods must not run concurrently.
// Existing Cursor, equality-seek, page formats, and SQL planning are unchanged.
type RowIDRangeCursor struct {
	cursor       *Cursor
	budget       *rangeBudget
	lower, upper *RowIDBound
	reverse      bool
	terminal     error
}

func NewRowIDRangeCursor(ctx context.Context, r PageReader, root uint32, lower, upper *RowIDBound, reverse bool, limits RangeLimits) (*RowIDRangeCursor, error) {
	b, err := newRangeBudget(ctx, r, limits)
	if err != nil {
		return nil, err
	}
	c := &RowIDRangeCursor{budget: b, reverse: reverse}
	if lower != nil && upper != nil && (lower.Value > upper.Value || lower.Value == upper.Value && (!lower.Inclusive || !upper.Inclusive)) {
		c.terminal = io.EOF
		return c, nil
	}
	if lower != nil {
		v := *lower
		c.lower = &v
	}
	if upper != nil {
		v := *upper
		c.upper = &v
	}
	base, err := NewCursor(ctx, b, root)
	if err != nil {
		return nil, err
	}
	c.cursor = base
	err = c.seek()
	if err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

func (c *RowIDRangeCursor) push(n uint32, b bounds) error {
	return c.cursor.push(n, b)
}
func (c *RowIDRangeCursor) child(f *frame, i int) (uint32, bounds) {
	n, b := f.right, f.bounds
	if i < len(f.cells) {
		n = f.cells[i].child
		b.upper = f.cells[i].rowid
		b.hasUpper = true
	}
	if i > 0 {
		b.lower = f.cells[i-1].rowid
		b.hasLower = true
	}
	return n, b
}
func (c *RowIDRangeCursor) seek() error {
	target := c.lower
	if c.reverse {
		target = c.upper
	}
	for {
		if err := c.budget.ctx.Err(); err != nil {
			return err
		}
		f := &c.cursor.stack[len(c.cursor.stack)-1]
		i := 0
		if target == nil {
			if c.reverse {
				i = len(f.cells)
			}
		} else {
			lo, hi := 0, len(f.cells)
			for lo < hi {
				if err := c.budget.work(1); err != nil {
					return err
				}
				mid := lo + (hi-lo)/2
				id := f.cells[mid].rowid
				// Interior table separators are inclusive maxima of their left child.
				// Reverse seek keeps an equal separator's left child even for '<'.
				before := id < target.Value
				if f.leaf {
					if c.reverse {
						before = id < target.Value || id == target.Value && target.Inclusive
					} else {
						before = id < target.Value || id == target.Value && !target.Inclusive
					}
				} else if !c.reverse {
					before = id < target.Value || id == target.Value && !target.Inclusive
				}
				if before {
					lo = mid + 1
				} else {
					hi = mid
				}
			}
			i = lo
		}
		if f.leaf {
			f.next = i
			if c.reverse {
				f.next = i - 1
			}
			return nil
		}
		n, b := c.child(f, i)
		f.next = i + 1
		if c.reverse {
			f.next = i - 1
		}
		if err := c.push(n, b); err != nil {
			return err
		}
	}
}
func (c *RowIDRangeCursor) finish(err error) {
	c.terminal = err
	c.lower, c.upper = nil, nil
	if c.cursor != nil {
		c.cursor.Close()
	}
}

// Next returns owned record fields. Limits, cancellation, and EOF are sticky;
// Close releases traversal state and changes any terminal error to EOF.
func (c *RowIDRangeCursor) Next() (id int64, values []any, err error) {
	if c.terminal != nil {
		return 0, nil, c.terminal
	}
	defer func() {
		if err != nil {
			c.finish(err)
		}
	}()
	b := c.cursor
	for len(b.stack) > 0 {
		if err = b.ctx.Err(); err != nil {
			return 0, nil, err
		}
		f := &b.stack[len(b.stack)-1]
		if f.leaf {
			if f.next < 0 || f.next >= len(f.cells) {
				b.stack = b.stack[:len(b.stack)-1]
				continue
			}
			item := f.cells[f.next]
			if c.reverse {
				f.next--
			} else {
				f.next++
			}
			if c.lower != nil && (item.rowid < c.lower.Value || item.rowid == c.lower.Value && !c.lower.Inclusive) || c.upper != nil && (item.rowid > c.upper.Value || item.rowid == c.upper.Value && !c.upper.Inclusive) {
				return 0, nil, io.EOF
			}
			if b.hasLast && ((!c.reverse && item.rowid <= b.last) || (c.reverse && item.rowid >= b.last)) {
				return 0, nil, corrupt("bounded rowid order")
			}
			if err = c.budget.BeforeRecord(item.payload); err != nil {
				return 0, nil, err
			}
			p, e := b.payload(f, item)
			if e != nil {
				return 0, nil, e
			}
			values, e = record.Decode(p)
			if e != nil {
				return 0, nil, fmt.Errorf("page %d rowid %d: %w", f.number, item.rowid, e)
			}
			if err = b.ctx.Err(); err != nil {
				return 0, nil, err
			}
			b.last, b.hasLast = item.rowid, true
			c.budget.stats.Returned++
			// An exact terminal rowid is unique. Avoid visiting the adjacent subtree.
			end := c.upper
			if c.reverse {
				end = c.lower
			}
			if end != nil && item.rowid == end.Value {
				c.finish(io.EOF)
			}
			return item.rowid, values, nil
		}
		if f.next < 0 || f.next > len(f.cells) {
			b.stack = b.stack[:len(b.stack)-1]
			continue
		}
		i := f.next
		if c.reverse {
			f.next--
		} else {
			f.next++
		}
		n, bound := c.child(f, i)
		// Subtree separator bounds can prove the whole next child is past the end.
		if !c.reverse && c.upper != nil && bound.hasLower && bound.lower >= c.upper.Value {
			return 0, nil, io.EOF
		}
		if c.reverse && c.lower != nil && bound.hasUpper && (bound.upper < c.lower.Value || bound.upper == c.lower.Value && !c.lower.Inclusive) {
			return 0, nil, io.EOF
		}
		if err = c.push(n, bound); err != nil {
			return 0, nil, err
		}
		child := &b.stack[len(b.stack)-1]
		if c.reverse {
			child.next = len(child.cells)
			if child.leaf {
				child.next--
			}
		}
	}
	return 0, nil, io.EOF
}
func (c *RowIDRangeCursor) Stats() RangeStats { return c.budget.stats }
func (c *RowIDRangeCursor) Close() error      { c.finish(io.EOF); return nil }
