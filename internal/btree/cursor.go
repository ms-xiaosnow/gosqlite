// Package btree reads and mutates ordinary SQLite rowid-table B-trees and
// BINARY ascending indexes. WITHOUT ROWID B-trees remain unsupported.
package btree

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"sort"

	"github.com/ms-xiaosnow/gosqlite/internal/pager"
	"github.com/ms-xiaosnow/gosqlite/internal/record"
)

const MaxDepth = 64
const MaxVisitedPages = 1 << 20

type bounds struct {
	lower, upper       int64
	hasLower, hasUpper bool
}

func (b bounds) contains(n int64) bool {
	return (!b.hasLower || n > b.lower) && (!b.hasUpper || n <= b.upper)
}

type cell struct {
	offset, end    int
	rowid          int64
	payload, local int
	child          uint32
}
type frame struct {
	number uint32
	data   []byte
	leaf   bool
	cells  []cell
	next   int
	right  uint32
	bounds bounds
}

// PageReader is the logical-page boundary shared by read-only files and future
// transaction overlays. Implementations must provide a coherent snapshot.
type PageReader interface {
	Header() pager.Header
	ReadPage(context.Context, uint32) ([]byte, error)
}

// Cursor holds only the current B-tree path and one decoded row, rather than
// the whole table. Visited-page bookkeeping is capped to prevent cycles and
// malicious inputs exhausting memory. Cursor methods must not run concurrently.
type Cursor struct {
	ctx       context.Context
	reader    PageReader
	header    pager.Header
	root      uint32
	leafDepth int
	stack     []frame
	visited   map[uint32]struct{}
	last      int64
	hasLast   bool
	done      bool
	terminal  error
}

func NewCursor(ctx context.Context, r PageReader, root uint32) (*Cursor, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	h := r.Header()
	if h.PageSize < 512 || h.PageSize > 65536 || h.PageSize&(h.PageSize-1) != 0 || h.UsableSize < 480 || h.UsableSize > h.PageSize || h.PageCount == 0 {
		return nil, corrupt("invalid PageReader header")
	}
	c := &Cursor{ctx: ctx, reader: r, header: h, root: root, visited: make(map[uint32]struct{})}
	if err := c.push(root, bounds{}); err != nil {
		return nil, err
	}
	return c, nil
}

func corrupt(format string, args ...any) error {
	return fmt.Errorf("%w: %s", pager.ErrCorrupt, fmt.Sprintf(format, args...))
}

func (c *Cursor) readPage(n uint32) ([]byte, error) {
	if n == 0 || n > c.header.PageCount {
		return nil, corrupt("page %d outside database", n)
	}
	if _, ok := c.visited[n]; ok {
		return nil, corrupt("page %d referenced more than once", n)
	}
	if len(c.visited) >= MaxVisitedPages {
		return nil, fmt.Errorf("%w: visited page budget", record.ErrLimit)
	}
	p, err := c.reader.ReadPage(c.ctx, n)
	if err != nil {
		return nil, err
	}
	if len(p) != int(c.header.PageSize) {
		return nil, corrupt("page %d has incorrect byte count", n)
	}
	c.visited[n] = struct{}{}
	return p, nil
}

func (c *Cursor) push(n uint32, b bounds) error {
	if len(c.stack) >= MaxDepth {
		return fmt.Errorf("%w: B-tree depth", record.ErrLimit)
	}
	p, err := c.readPage(n)
	if err != nil {
		return err
	}
	f, err := parsePage(p, n, int(c.header.UsableSize), b)
	if err != nil {
		return err
	}
	if len(f.cells) == 0 && (n != c.root || !f.leaf && n != 1) {
		return corrupt("invalid empty B-tree page %d", n)
	}
	if f.leaf {
		depth := len(c.stack) + 1
		if c.leafDepth != 0 && c.leafDepth != depth {
			return corrupt("B-tree leaves have unequal depths")
		}
		c.leafDepth = depth
	}
	c.stack = append(c.stack, f)
	return nil
}

type span struct{ start, end int }

func parsePage(p []byte, n uint32, usable int, b bounds) (frame, error) {
	f := frame{number: n, data: p, bounds: b}
	off := 0
	if n == 1 {
		off = 100
	}
	if usable > len(p) || usable < off+8 {
		return f, corrupt("page %d header truncated", n)
	}
	switch p[off] {
	case 13:
		f.leaf = true
	case 5:
	case 2, 10:
		return f, fmt.Errorf("%w: index or WITHOUT ROWID B-tree page %d", pager.ErrUnsupported, n)
	default:
		return f, corrupt("page %d has invalid B-tree type %#x", n, p[off])
	}
	hdr := 8
	if !f.leaf {
		hdr = 12
	}
	if usable < off+hdr {
		return f, corrupt("page %d interior header truncated", n)
	}
	count := int(binary.BigEndian.Uint16(p[off+3 : off+5]))
	pointerEnd := off + hdr + 2*count
	if pointerEnd > usable {
		return f, corrupt("page %d cell pointer array exceeds page", n)
	}
	content := int(binary.BigEndian.Uint16(p[off+5 : off+7]))
	if content == 0 {
		content = 65536
	}
	if content < pointerEnd || content > usable || p[off+7] > 60 {
		return f, corrupt("page %d invalid content area", n)
	}
	if !f.leaf {
		f.right = binary.BigEndian.Uint32(p[off+8 : off+12])
		if f.right == 0 {
			return f, corrupt("page %d has zero right child", n)
		}
	}
	f.cells = make([]cell, 0, count)
	spans := make([]span, 0, count)
	for i := 0; i < count; i++ {
		start := int(binary.BigEndian.Uint16(p[off+hdr+2*i : off+hdr+2*i+2]))
		if start < content || start >= usable {
			return f, corrupt("page %d cell %d offset", n, i)
		}
		item := cell{offset: start}
		at := start
		if !f.leaf {
			if usable-at < 4 {
				return f, corrupt("page %d child pointer truncated", n)
			}
			item.child = binary.BigEndian.Uint32(p[at : at+4])
			at += 4
			if item.child == 0 {
				return f, corrupt("page %d zero child", n)
			}
		} else {
			size, k, err := record.DecodeVarint(p[at:usable])
			if err != nil {
				return f, fmt.Errorf("page %d payload: %w", n, err)
			}
			at += k
			if size > record.MaxPayload {
				return f, fmt.Errorf("%w: page %d payload size %d", record.ErrLimit, n, size)
			}
			item.payload = int(size)
		}
		row, k, err := record.DecodeVarint(p[at:usable])
		if err != nil {
			return f, fmt.Errorf("page %d rowid: %w", n, err)
		}
		at += k
		item.rowid = int64(row)
		if !b.contains(item.rowid) || (i > 0 && item.rowid <= f.cells[i-1].rowid) {
			return f, corrupt("page %d rowid order or parent bounds", n)
		}
		item.offset = at
		if f.leaf {
			item.local = item.payload
			maxLocal := usable - 35
			if item.payload > maxLocal {
				minLocal := (usable-12)*32/255 - 23
				local := minLocal + (item.payload-minLocal)%(usable-4)
				if local > maxLocal {
					local = minLocal
				}
				item.local = local
			}
			at += item.local
			if item.local < item.payload {
				at += 4
			}
		}
		if at-start < 4 {
			at = start + 4
		}
		if at > usable {
			return f, corrupt("page %d cell %d extends beyond usable area", n, i)
		}
		item.end = at
		f.cells = append(f.cells, item)
		spans = append(spans, span{start, at})
	}
	// Freeblocks are ordered by offset and include a four-byte header. Validate
	// both bounds and disjointness with cells; no untrusted chain is followed
	// before its offsets have been checked.
	free := int(binary.BigEndian.Uint16(p[off+1 : off+3]))
	last := 0
	for free != 0 {
		if free < content || free <= last || free > usable-4 {
			return f, corrupt("page %d freeblock offset", n)
		}
		next := int(binary.BigEndian.Uint16(p[free : free+2]))
		size := int(binary.BigEndian.Uint16(p[free+2 : free+4]))
		if size < 4 || size > usable-free {
			return f, corrupt("page %d freeblock length", n)
		}
		spans = append(spans, span{free, free + size})
		last = free
		free = next
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
	for i := 1; i < len(spans); i++ {
		if spans[i].start < spans[i-1].end {
			return f, corrupt("page %d overlapping cells or freeblocks", n)
		}
	}
	return f, nil
}

func (c *Cursor) payload(f *frame, item cell) ([]byte, error) {
	if item.payload == item.local {
		return f.data[item.offset : item.offset+item.local], nil
	}
	if uint64(item.payload-item.local) > uint64(c.header.PageCount)*uint64(c.header.UsableSize-4) {
		return nil, corrupt("record overflow exceeds database size")
	}
	out := make([]byte, item.payload)
	copy(out, f.data[item.offset:item.offset+item.local])
	at := item.local
	next := binary.BigEndian.Uint32(f.data[item.offset+item.local : item.offset+item.local+4])
	usable := int(c.header.UsableSize)
	for at < len(out) {
		if next == 0 {
			return nil, corrupt("truncated overflow chain on page %d", f.number)
		}
		p, err := c.readPage(next)
		if err != nil {
			return nil, err
		}
		size := len(out) - at
		if size > usable-4 {
			size = usable - 4
		}
		copy(out[at:at+size], p[4:4+size])
		at += size
		next = binary.BigEndian.Uint32(p[:4])
	}
	if next != 0 {
		return nil, corrupt("overflow chain exceeds record payload")
	}
	return out, nil
}

// Next advances in signed rowid order. Returned text/BLOB fields are owned by
// the caller. EOF and other terminal errors are sticky until Close.
func (c *Cursor) Next() (rowid int64, values []any, err error) {
	if c.terminal != nil {
		return 0, nil, c.terminal
	}
	if c.done {
		return 0, nil, io.EOF
	}
	defer func() {
		if err != nil {
			c.terminal = err
			c.stack = nil
			c.visited = nil
		}
	}()
	for len(c.stack) > 0 {
		if err = c.ctx.Err(); err != nil {
			return 0, nil, err
		}
		f := &c.stack[len(c.stack)-1]
		if f.leaf {
			if f.next >= len(f.cells) {
				c.stack = c.stack[:len(c.stack)-1]
				continue
			}
			item := f.cells[f.next]
			f.next++
			if c.hasLast && item.rowid <= c.last {
				return 0, nil, corrupt("global rowid order")
			}
			p, e := c.payload(f, item)
			if e != nil {
				return 0, nil, e
			}
			values, e = record.Decode(p)
			if e != nil {
				return 0, nil, fmt.Errorf("page %d rowid %d: %w", f.number, item.rowid, e)
			}
			if e := c.ctx.Err(); e != nil {
				return 0, nil, e
			}
			c.last, c.hasLast = item.rowid, true
			return item.rowid, values, nil
		}
		if f.next > len(f.cells) {
			c.stack = c.stack[:len(c.stack)-1]
			continue
		}
		i := f.next
		f.next++
		child := f.right
		b := f.bounds
		if i < len(f.cells) {
			child = f.cells[i].child
			b.upper = f.cells[i].rowid
			b.hasUpper = true
		}
		if i > 0 {
			b.lower = f.cells[i-1].rowid
			b.hasLower = true
		}
		if e := c.push(child, b); e != nil {
			return 0, nil, e
		}
	}
	c.done = true
	return 0, nil, io.EOF
}

func (c *Cursor) Close() error {
	c.done = true
	c.terminal = nil
	c.stack = nil
	c.visited = nil
	return nil
}

// Scan is a streaming callback convenience wrapper around Cursor.
func Scan(ctx context.Context, r PageReader, root uint32, fn func(int64, []any) error) error {
	c, err := NewCursor(ctx, r, root)
	if err != nil {
		return err
	}
	defer c.Close()
	for {
		id, row, err := c.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err = fn(id, row); err != nil {
			return err
		}
	}
}
