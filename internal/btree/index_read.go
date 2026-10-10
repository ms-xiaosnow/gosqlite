package btree

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"sort"
	"strings"

	"github.com/ms-xiaosnow/gosqlite/internal/pager"
	"github.com/ms-xiaosnow/gosqlite/internal/record"
)

// KeyInfo describes an ordinary rowid-table index with Columns indexed fields,
// all using BINARY collation and ascending order. Each stored key also contains
// its final int64 rowid. Expression, descending, and WITHOUT ROWID keys require
// different metadata and are not represented by this type.
type KeyInfo struct{ Columns int }

type indexCell struct {
	child                       uint32
	payload, local, offset, end int
}

type indexFrame struct {
	number uint32
	data   []byte
	leaf   bool
	cells  []indexCell
	right  uint32
}

type indexCursorFrame struct {
	indexFrame
	next int
}

// IndexCursor streams index records in SQLite key order. Interior cells own
// records, so traversal visits child, record, child rather than just leaves.
// The cursor retains a bounded current path, one previous key, and visited-page
// bookkeeping. Methods must not run concurrently.
type IndexCursor struct {
	ctx       context.Context
	reader    PageReader
	header    pager.Header
	root      uint32
	leafDepth int
	info      KeyInfo
	stack     []indexCursorFrame
	visited   map[uint32]struct{}
	overflow  map[uint32]indexOverflowOwner
	last      []any
	done      bool
	terminal  error
}

func validateKeyInfo(info KeyInfo) error {
	if info.Columns <= 0 {
		return fmt.Errorf("%w: index must have at least one indexed column", record.ErrUnsupported)
	}
	if info.Columns >= record.MaxColumns {
		return fmt.Errorf("%w: index column count", record.ErrLimit)
	}
	return nil
}

func validateIndexKey(info KeyInfo, key []any) error {
	if err := validateKeyInfo(info); err != nil {
		return err
	}
	if len(key) != info.Columns+1 {
		return fmt.Errorf("%w: index key has %d fields, expected %d", record.ErrUnsupported, len(key), info.Columns+1)
	}
	if _, ok := key[len(key)-1].(int64); !ok {
		return fmt.Errorf("%w: index rowid must be int64", record.ErrUnsupported)
	}
	return validateIndexFields(key)
}

// validateIndexFields also validates partial range keys without padding them
// with invented values. Callers validate the field count and rowid position.
func validateIndexFields(key []any) error {
	// Compute the encoded size without allocating an encoding. In particular,
	// small integers use fewer than eight bytes, and NaN is encoded as NULL.
	size, header := 0, len(key)
	for _, v := range key {
		n := 0
		var serial uint64
		switch x := v.(type) {
		case nil:
		case int64:
			switch {
			case x == 0 || x == 1:
			case x >= -128 && x <= 127:
				n = 1
			case x >= -32768 && x <= 32767:
				n = 2
			case x >= -8388608 && x <= 8388607:
				n = 3
			case x >= -2147483648 && x <= 2147483647:
				n = 4
			case x >= -140737488355328 && x <= 140737488355327:
				n = 6
			default:
				n = 8
			}
		case float64:
			if !math.IsNaN(x) {
				n = 8
			}
		case string:
			n = len(x)
			serial = uint64(n)*2 + 13
		case []byte:
			n = len(x)
			serial = uint64(n)*2 + 12
		default:
			return fmt.Errorf("%w: index field %T", record.ErrUnsupported, v)
		}
		if n > record.MaxPayload-size {
			return fmt.Errorf("%w: index key payload", record.ErrLimit)
		}
		size += n
		for serial >= 128 {
			header++
			serial >>= 7
		}
	}
	// The header-length varint includes itself, possibly growing at a boundary.
	headerSize := header + 1
	for {
		n := 1
		for v := headerSize; v >= 128; v >>= 7 {
			n++
		}
		if header+n == headerSize {
			break
		}
		headerSize = header + n
	}
	if headerSize > record.MaxPayload-size {
		return fmt.Errorf("%w: index key payload and header", record.ErrLimit)
	}
	return nil
}

func indexStorageClass(v any) int {
	switch x := v.(type) {
	case nil:
		return 0
	case int64:
		return 1
	case float64:
		if math.IsNaN(x) { // SQLite stores NaN as NULL.
			return 0
		}
		return 1
	case string:
		return 2
	case []byte:
		return 3
	default:
		panic("btree: comparing unvalidated index field")
	}
}

func compareIndexIntFloat(i int64, r float64) int {
	// Match sqlite3IntFloatCompare: first compare in the integer domain. A
	// float64 conversion of i alone loses order near 2^53 and the int64 ends.
	if r < -9223372036854775808.0 {
		return 1
	}
	if r >= 9223372036854775808.0 {
		return -1
	}
	y := int64(r)
	if i < y {
		return -1
	}
	if i > y {
		return 1
	}
	if float64(i) < r {
		return -1
	}
	if float64(i) > r {
		return 1
	}
	return 0
}

// compareIndexKeys compares already validated keys using SQLite storage class
// order, exact mixed integer/real comparison, and bytewise BINARY text/BLOBs.
func compareIndexKeys(a, b []any) int {
	for i := 0; i < len(a) && i < len(b); i++ {
		ac, bc := indexStorageClass(a[i]), indexStorageClass(b[i])
		if ac < bc {
			return -1
		}
		if ac > bc {
			return 1
		}
		cmp := 0
		switch ac {
		case 1:
			switch x := a[i].(type) {
			case int64:
				switch y := b[i].(type) {
				case int64:
					if x < y {
						cmp = -1
					} else if x > y {
						cmp = 1
					}
				case float64:
					cmp = compareIndexIntFloat(x, y)
				}
			case float64:
				switch y := b[i].(type) {
				case int64:
					cmp = -compareIndexIntFloat(y, x)
				case float64:
					if x < y {
						cmp = -1
					} else if x > y {
						cmp = 1
					}
				}
			}
		case 2:
			cmp = strings.Compare(a[i].(string), b[i].(string))
		case 3:
			cmp = bytes.Compare(a[i].([]byte), b[i].([]byte))
		}
		if cmp != 0 {
			return cmp
		}
	}
	if len(a) < len(b) {
		return -1
	}
	if len(a) > len(b) {
		return 1
	}
	return 0
}

// parseIndexPage validates cell/freeblock bounds before any payload access.
// Offset is the start of payload, after the optional child and length varint.
func parseIndexPage(p []byte, n uint32, usable int) (indexFrame, error) {
	f := indexFrame{number: n, data: p}
	off := 0
	if n == 1 {
		off = 100
	}
	if usable < 480 || usable > 65536 || usable > len(p) || usable < off+8 {
		return f, corrupt("page %d index header truncated or invalid usable size", n)
	}
	switch p[off] {
	case 10:
		f.leaf = true
	case 2:
	case 5, 13:
		return f, fmt.Errorf("%w: table B-tree page %d used as index", pager.ErrUnsupported, n)
	default:
		return f, corrupt("page %d invalid index B-tree type %#x", n, p[off])
	}
	hdr := 8
	if !f.leaf {
		hdr = 12
	}
	if usable < off+hdr {
		return f, corrupt("page %d index interior header truncated", n)
	}
	count := int(binary.BigEndian.Uint16(p[off+3 : off+5]))
	pointerEnd := off + hdr + 2*count
	if pointerEnd > usable {
		return f, corrupt("page %d index cell pointer array exceeds page", n)
	}
	content := int(binary.BigEndian.Uint16(p[off+5 : off+7]))
	if content == 0 {
		content = 65536
	}
	if content < pointerEnd || content > usable || p[off+7] > 60 {
		return f, corrupt("page %d invalid index content area", n)
	}
	if !f.leaf {
		f.right = binary.BigEndian.Uint32(p[off+8 : off+12])
		if f.right == 0 {
			return f, corrupt("page %d zero right index child", n)
		}
	}
	f.cells = make([]indexCell, 0, count)
	spans := make([]span, 0, count)
	for i := 0; i < count; i++ {
		start := int(binary.BigEndian.Uint16(p[off+hdr+2*i : off+hdr+2*i+2]))
		if start < content || start >= usable {
			return f, corrupt("page %d index cell %d offset", n, i)
		}
		item := indexCell{}
		at := start
		if !f.leaf {
			if usable-at < 4 {
				return f, corrupt("page %d index child pointer truncated", n)
			}
			item.child = binary.BigEndian.Uint32(p[at : at+4])
			if item.child == 0 {
				return f, corrupt("page %d zero index child", n)
			}
			at += 4
		}
		size, k, err := record.DecodeVarint(p[at:usable])
		if err != nil {
			return f, fmt.Errorf("page %d index payload: %w", n, err)
		}
		if size > record.MaxPayload {
			return f, fmt.Errorf("%w: page %d index payload size %d", record.ErrLimit, n, size)
		}
		at += k
		item.offset, item.payload = at, int(size)
		item.local = item.payload
		maxLocal := (usable-12)*64/255 - 23
		if item.payload > maxLocal {
			minLocal := (usable-12)*32/255 - 23
			item.local = minLocal + (item.payload-minLocal)%(usable-4)
			if item.local > maxLocal {
				item.local = minLocal
			}
		}
		at += item.local
		if item.local < item.payload {
			at += 4
		}
		if at-start < 4 {
			at = start + 4
		}
		if at > usable {
			return f, corrupt("page %d index cell %d exceeds usable area", n, i)
		}
		item.end = at
		f.cells = append(f.cells, item)
		spans = append(spans, span{start, at})
	}
	free, last := int(binary.BigEndian.Uint16(p[off+1:off+3])), 0
	for free != 0 {
		if free < content || free <= last || free > usable-4 {
			return f, corrupt("page %d index freeblock offset", n)
		}
		next := int(binary.BigEndian.Uint16(p[free : free+2]))
		size := int(binary.BigEndian.Uint16(p[free+2 : free+4]))
		if size < 4 || size > usable-free {
			return f, corrupt("page %d index freeblock length", n)
		}
		spans = append(spans, span{free, free + size})
		last, free = free, next
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].start < spans[j].start })
	for i := 1; i < len(spans); i++ {
		if spans[i].start < spans[i-1].end {
			return f, corrupt("page %d overlapping index cells or freeblocks", n)
		}
	}
	return f, nil
}

func NewIndexCursor(ctx context.Context, r PageReader, root uint32, info KeyInfo) (*IndexCursor, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := validateKeyInfo(info); err != nil {
		return nil, err
	}
	h := r.Header()
	if h.PageSize < 512 || h.PageSize > 65536 || h.PageSize&(h.PageSize-1) != 0 || h.UsableSize < 480 || h.UsableSize > h.PageSize || h.PageCount == 0 {
		return nil, corrupt("invalid index PageReader header")
	}
	c := &IndexCursor{ctx: ctx, reader: r, header: h, root: root, info: info, visited: make(map[uint32]struct{})}
	if err := c.push(root); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *IndexCursor) readPage(n uint32) ([]byte, error) {
	if err := c.ctx.Err(); err != nil {
		return nil, err
	}
	if n == 0 || n > c.header.PageCount {
		return nil, corrupt("index page %d outside database", n)
	}
	if _, ok := c.visited[n]; ok {
		return nil, corrupt("index page %d referenced more than once", n)
	}
	if len(c.visited) >= MaxVisitedPages {
		return nil, fmt.Errorf("%w: index visited page budget", record.ErrLimit)
	}
	p, err := c.reader.ReadPage(c.ctx, n)
	if err != nil {
		return nil, err
	}
	if err := c.ctx.Err(); err != nil {
		return nil, err
	}
	if len(p) != int(c.header.PageSize) {
		return nil, corrupt("index page %d has incorrect byte count", n)
	}
	c.visited[n] = struct{}{}
	return p, nil
}

func (c *IndexCursor) push(n uint32) error {
	if len(c.stack) >= MaxDepth {
		return fmt.Errorf("%w: index B-tree depth", record.ErrLimit)
	}
	p, err := c.readPage(n)
	if err != nil {
		return err
	}
	f, err := parseIndexPage(p, n, int(c.header.UsableSize))
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
	c.stack = append(c.stack, indexCursorFrame{indexFrame: f})
	return nil
}

func (c *IndexCursor) payload(f *indexFrame, item indexCell) ([]byte, error) {
	if item.payload == item.local {
		return f.data[item.offset : item.offset+item.local], nil
	}
	if uint64(item.payload-item.local) > uint64(c.header.PageCount)*uint64(c.header.UsableSize-4) {
		return nil, corrupt("index overflow exceeds database size")
	}
	out := make([]byte, item.payload)
	copy(out, f.data[item.offset:item.offset+item.local])
	at := item.local
	next := binary.BigEndian.Uint32(f.data[item.offset+item.local : item.offset+item.local+4])
	usable := int(c.header.UsableSize)
	for at < len(out) {
		if next == 0 {
			return nil, corrupt("truncated index overflow chain on page %d", f.number)
		}
		owner := indexOverflowOwner{page: f.number, offset: uint32(item.offset), position: uint32(at - item.local)}
		p, err := c.readOverflowPage(next, owner)
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
		return nil, corrupt("index overflow chain exceeds record payload")
	}
	return out, nil
}

func (c *IndexCursor) decodeKey(f *indexFrame, item indexCell) ([]any, error) {
	p, err := c.payload(f, item)
	if err != nil {
		return nil, err
	}
	key, err := record.Decode(p)
	if err != nil {
		return nil, fmt.Errorf("index page %d: %w", f.number, err)
	}
	if err := validateIndexKey(c.info, key); err != nil {
		return nil, corrupt("index page %d invalid key: %v", f.number, err)
	}
	if err := c.ctx.Err(); err != nil {
		return nil, err
	}
	return key, nil
}

func copyIndexKey(key []any) []any {
	out := append([]any(nil), key...)
	for i, v := range out {
		if b, ok := v.([]byte); ok {
			out[i] = bytes.Clone(b)
		}
	}
	return out
}

// Next returns an independently owned key including its final rowid. EOF and
// other terminal errors are sticky until Close.
func (c *IndexCursor) Next() (key []any, err error) {
	if c.terminal != nil {
		return nil, c.terminal
	}
	if c.done {
		return nil, io.EOF
	}
	defer func() {
		if err != nil {
			c.terminal = err
			c.stack, c.visited, c.last = nil, nil, nil
			c.overflow = nil
		}
	}()
	for len(c.stack) > 0 {
		if err = c.ctx.Err(); err != nil {
			return nil, err
		}
		f := &c.stack[len(c.stack)-1]
		var item indexCell
		if f.leaf {
			if f.next >= len(f.cells) {
				c.stack = c.stack[:len(c.stack)-1]
				continue
			}
			item = f.cells[f.next]
			f.next++
		} else {
			// Even steps visit children, odd steps return actual records.
			if f.next > 2*len(f.cells) {
				c.stack = c.stack[:len(c.stack)-1]
				continue
			}
			step := f.next
			f.next++
			i := step / 2
			if step%2 == 0 {
				child := f.right
				if i < len(f.cells) {
					child = f.cells[i].child
				}
				if err := c.push(child); err != nil {
					return nil, err
				}
				continue
			}
			item = f.cells[i]
		}
		key, err = c.decodeKey(&f.indexFrame, item)
		if err != nil {
			return nil, err
		}
		if c.last != nil && compareIndexKeys(c.last, key) >= 0 {
			return nil, corrupt("global index key order")
		}
		// Caller mutation of a returned BLOB or slice cannot alter validation.
		c.last = copyIndexKey(key)
		return key, nil
	}
	c.done = true
	return nil, io.EOF
}

func (c *IndexCursor) Close() error {
	c.done, c.terminal = true, nil
	c.stack, c.visited, c.last = nil, nil, nil
	c.overflow = nil
	return nil
}

// ScanIndex is the streaming callback wrapper around IndexCursor.
func ScanIndex(ctx context.Context, r PageReader, root uint32, info KeyInfo, fn func([]any) error) error {
	c, err := NewIndexCursor(ctx, r, root, info)
	if err != nil {
		return err
	}
	defer c.Close()
	for {
		key, err := c.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if err := fn(key); err != nil {
			return err
		}
	}
}

// SeekIndex looks up one complete key, including its rowid, by B-tree descent.
// Numeric equality follows SQLite, so an integral real can match an integer.
// Like a normal point lookup, it validates visited records rather than scanning
// unrelated subtrees. A missing key returns (nil, false, nil).
func SeekIndex(ctx context.Context, r PageReader, root uint32, info KeyInfo, key []any) ([]any, bool, error) {
	if err := ctx.Err(); err != nil {
		return nil, false, err
	}
	if err := validateIndexKey(info, key); err != nil {
		return nil, false, err
	}
	c, err := NewIndexCursor(ctx, r, root, info)
	if err != nil {
		return nil, false, err
	}
	defer c.Close()
	var lower, upper []any
	for depth := 0; ; depth++ {
		if err := ctx.Err(); err != nil {
			return nil, false, err
		}
		f := &c.stack[0].indexFrame
		lo, hi := 0, len(f.cells)
		for lo < hi {
			mid := lo + (hi-lo)/2
			candidate, err := c.decodeKey(f, f.cells[mid])
			if err != nil {
				return nil, false, err
			}
			if (lower != nil && compareIndexKeys(candidate, lower) <= 0) || (upper != nil && compareIndexKeys(candidate, upper) >= 0) {
				return nil, false, corrupt("index search key order or parent bounds")
			}
			cmp := compareIndexKeys(candidate, key)
			if cmp == 0 {
				return candidate, true, nil
			}
			if cmp < 0 {
				lo, lower = mid+1, candidate
			} else {
				hi, upper = mid, candidate
			}
		}
		if f.leaf {
			return nil, false, nil
		}
		if depth+1 >= MaxDepth {
			return nil, false, fmt.Errorf("%w: index B-tree depth", record.ErrLimit)
		}
		child := f.right
		if lo < len(f.cells) {
			child = f.cells[lo].child
		}
		// Search retains only this page and two bounding keys, not all pivots.
		c.stack = c.stack[:0]
		if err := c.push(child); err != nil {
			return nil, false, err
		}
	}
}
