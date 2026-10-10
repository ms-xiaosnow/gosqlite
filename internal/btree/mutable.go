package btree

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"sort"

	"github.com/ms-xiaosnow/gosqlite/internal/pager"
	"github.com/ms-xiaosnow/gosqlite/internal/record"
)

// PageWriter is a transaction's logical-page overlay. Mutations may change
// several pages before returning an error: callers must provide statement
// rollback (pager.Tx.Statement), and must not commit a failed mutation.
type PageWriter interface {
	PageReader
	WritePage(context.Context, uint32, []byte) error
	AllocatePage(context.Context) (uint32, error)
	FreePage(context.Context, uint32) error
}

type TreeKind uint8

const (
	TableTree TreeKind = iota + 1
	IndexTree
)

var (
	ErrDuplicate = errors.New("duplicate B-tree key")
	ErrNotFound  = errors.New("B-tree key not found")
)

// An entry owns its overflow chain. body is the on-page cell excluding the
// optional interior child pointer and minimum-size padding. Moving an entry
// between siblings or into an index parent transfers that ownership unchanged.
type mutableEntry struct {
	rowid int64
	body  []byte
}
type mutableNode struct {
	number   uint32
	leaf     bool
	entries  []mutableEntry
	children []uint32
	original []byte
}
type nodeSplit struct {
	separator mutableEntry
	right     uint32
	next      *nodeSplit
}
type mutation struct {
	ctx   context.Context
	w     PageWriter
	h     pager.Header
	kind  TreeKind
	info  KeyInfo
	path  map[uint32]bool
	reads int
	bytes int
}

const maxMutationBytes = 128 << 20

// MaxMutableIndexPayload bounds the encoded key of an index mutation. Read
// cursors can read the larger record.MaxPayload limit. The smaller write bound
// reserves space for the input key and an existing comparison key together.
const MaxMutableIndexPayload = 16 << 20

func newMutation(ctx context.Context, w PageWriter, kind TreeKind, info KeyInfo) (*mutation, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	h := w.Header()
	if h.PageSize < 512 || h.PageSize > 65536 || h.PageSize&(h.PageSize-1) != 0 || h.UsableSize < 480 || h.UsableSize > h.PageSize || h.PageCount == 0 {
		return nil, corrupt("invalid writable page header")
	}
	if kind != TableTree && kind != IndexTree {
		return nil, fmt.Errorf("%w: B-tree kind %d", pager.ErrUnsupported, kind)
	}
	if kind == IndexTree && (info.Columns < 1 || info.Columns >= record.MaxColumns) {
		return nil, fmt.Errorf("%w: index key column count", pager.ErrUnsupported)
	}
	if h.TextEncoding != 0 && h.TextEncoding != 1 {
		return nil, fmt.Errorf("%w: non-UTF8 B-tree", pager.ErrUnsupported)
	}
	return &mutation{ctx: ctx, w: w, h: h, kind: kind, info: info, path: make(map[uint32]bool)}, nil
}
func (m *mutation) charge(n int) error {
	if n < 0 || n > maxMutationBytes-m.bytes {
		return fmt.Errorf("%w: B-tree mutation memory budget", record.ErrLimit)
	}
	m.bytes += n
	return m.ctx.Err()
}
func (m *mutation) read(number uint32) ([]byte, error) {
	if err := m.ctx.Err(); err != nil {
		return nil, err
	}
	if number == 0 || number > m.w.Header().PageCount {
		return nil, corrupt("page %d outside database", number)
	}
	if m.reads >= MaxVisitedPages {
		return nil, fmt.Errorf("%w: mutation page budget", record.ErrLimit)
	}
	m.reads++
	p, err := m.w.ReadPage(m.ctx, number)
	if err != nil {
		return nil, err
	}
	if len(p) != int(m.h.PageSize) {
		return nil, corrupt("page %d byte count", number)
	}
	if err = m.charge(len(p)); err != nil {
		return nil, err
	}
	return p, nil
}
func (m *mutation) enter(number uint32, depth int) error {
	if depth >= MaxDepth {
		return fmt.Errorf("%w: B-tree mutation depth", record.ErrLimit)
	}
	if m.path[number] {
		return corrupt("B-tree cycle at page %d", number)
	}
	m.path[number] = true
	return m.ctx.Err()
}
func (m *mutation) load(number uint32) (*mutableNode, error) {
	p, err := m.read(number)
	if err != nil {
		return nil, err
	}
	n := &mutableNode{number: number, original: p}
	off := 0
	if number == 1 {
		off = 100
	}
	if m.kind == TableTree {
		f, e := parsePage(p, number, int(m.h.UsableSize), bounds{})
		if e != nil {
			return nil, e
		}
		n.leaf = f.leaf
		for i, c := range f.cells {
			start := int(binary.BigEndian.Uint16(p[off+8+boolInt(!f.leaf)*4+2*i:]))
			if !f.leaf {
				start += 4
				n.children = append(n.children, c.child)
			}
			end := c.offset
			if f.leaf {
				end += c.local
				if c.local < c.payload {
					end += 4
				}
			}
			n.entries = append(n.entries, mutableEntry{rowid: c.rowid, body: p[start:end]})
		}
		if !f.leaf {
			n.children = append(n.children, f.right)
		}
	} else {
		if number == 1 {
			return nil, fmt.Errorf("%w: index on schema root", pager.ErrUnsupported)
		}
		f, e := parseIndexPage(p, number, int(m.h.UsableSize))
		if e != nil {
			return nil, e
		}
		n.leaf = f.leaf
		for i, c := range f.cells {
			start := int(binary.BigEndian.Uint16(p[8+boolInt(!f.leaf)*4+2*i:]))
			if !f.leaf {
				start += 4
				n.children = append(n.children, c.child)
			}
			end := c.offset + c.local
			if c.local < c.payload {
				end += 4
			}
			n.entries = append(n.entries, mutableEntry{body: p[start:end]})
		}
		if !f.leaf {
			n.children = append(n.children, f.right)
		}
	}
	if !n.leaf {
		seen := make(map[uint32]bool, len(n.children))
		for _, child := range n.children {
			if child == number || child == 0 || child > m.w.Header().PageCount || seen[child] {
				return nil, corrupt("page %d invalid or repeated child", number)
			}
			seen[child] = true
		}
	}
	if err = m.charge(len(n.entries)*40 + len(n.children)*4); err != nil {
		return nil, err
	}
	return n, nil
}
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
func cellSpace(e mutableEntry, leaf bool) int {
	n := len(e.body)
	if !leaf {
		n += 4
	}
	if n < 4 {
		n = 4
	}
	return n + 2
}
func (m *mutation) nodeSize(n *mutableNode) int {
	size := 8
	if !n.leaf {
		size = 12
	}
	if n.number == 1 {
		size += 100
	}
	for _, e := range n.entries {
		size += cellSpace(e, n.leaf)
	}
	return size
}
func (m *mutation) fits(n *mutableNode) bool { return m.nodeSize(n) <= int(m.h.UsableSize) }
func (m *mutation) write(n *mutableNode) error {
	if !m.fits(n) {
		return fmt.Errorf("%w: B-tree node does not fit page", record.ErrLimit)
	}
	if !n.leaf && len(n.children) != len(n.entries)+1 {
		return corrupt("interior child count")
	}
	p := make([]byte, int(m.h.PageSize))
	// Reserved tail bytes and the database header belong to other layers.
	if len(n.original) == len(p) {
		copy(p[int(m.h.UsableSize):], n.original[int(m.h.UsableSize):])
	}
	off := 0
	if n.number == 1 {
		old, err := m.w.ReadPage(m.ctx, 1)
		if err != nil {
			return err
		}
		if len(old) != len(p) {
			return corrupt("page 1 byte count")
		}
		copy(p[:100], old[:100])
		off = 100
	}
	typ := byte(13)
	if m.kind == IndexTree {
		typ = 10
	}
	if !n.leaf {
		typ -= 8
	}
	p[off] = typ
	binary.BigEndian.PutUint16(p[off+3:], uint16(len(n.entries)))
	hdr := 8
	if !n.leaf {
		hdr = 12
		binary.BigEndian.PutUint32(p[off+8:], n.children[len(n.children)-1])
	}
	at := int(m.h.UsableSize)
	for i, e := range n.entries {
		size := cellSpace(e, n.leaf) - 2
		at -= size
		binary.BigEndian.PutUint16(p[off+hdr+2*i:], uint16(at))
		cell := p[at : at+size]
		if !n.leaf {
			binary.BigEndian.PutUint32(cell, n.children[i])
			cell = cell[4:]
		}
		copy(cell, e.body)
	}
	binary.BigEndian.PutUint16(p[off+5:], uint16(at)) // 65536 is encoded as zero.
	return m.w.WritePage(m.ctx, n.number, p)
}
func (m *mutation) allocate(leaf bool) (*mutableNode, error) {
	number, err := m.w.AllocatePage(m.ctx)
	if err != nil {
		return nil, err
	}
	if number <= 1 || number > m.w.Header().PageCount {
		return nil, corrupt("allocator returned invalid page %d", number)
	}
	return &mutableNode{number: number, leaf: leaf}, nil
}

// CreateTree allocates a stable, initially empty root. It never changes schema.
func CreateTree(ctx context.Context, w PageWriter, kind TreeKind) (uint32, error) {
	info := KeyInfo{Columns: 1}
	m, err := newMutation(ctx, w, kind, info)
	if err != nil {
		return 0, err
	}
	n, err := m.allocate(true)
	if err != nil {
		return 0, err
	}
	if err = m.write(n); err != nil {
		return 0, err
	}
	return n.number, nil
}
func localPayload(size, usable int, kind TreeKind) int {
	max := usable - 35
	if kind == IndexTree {
		max = (usable-12)*64/255 - 23
	}
	if size <= max {
		return size
	}
	min := (usable-12)*32/255 - 23
	n := min + (size-min)%(usable-4)
	if n > max {
		n = min
	}
	return n
}
func (m *mutation) makeEntry(rowid int64, payload []byte) (mutableEntry, error) {
	e := mutableEntry{rowid: rowid}
	e.body = record.AppendVarint(nil, uint64(len(payload)))
	if m.kind == TableTree {
		e.body = record.AppendVarint(e.body, uint64(rowid))
	}
	local := localPayload(len(payload), int(m.h.UsableSize), m.kind)
	e.body = append(e.body, payload[:local]...)
	if local == len(payload) {
		return e, nil
	}
	var first, previous uint32
	var previousData []byte
	for at := local; at < len(payload); {
		if err := m.ctx.Err(); err != nil {
			return e, err
		}
		number, err := m.w.AllocatePage(m.ctx)
		if err != nil {
			return e, err
		}
		if number <= 1 || number > m.w.Header().PageCount {
			return e, corrupt("invalid overflow allocation")
		}
		p := make([]byte, int(m.h.PageSize))
		size := len(payload) - at
		if size > int(m.h.UsableSize)-4 {
			size = int(m.h.UsableSize) - 4
		}
		copy(p[4:], payload[at:at+size])
		at += size
		if first == 0 {
			first = number
		} else {
			binary.BigEndian.PutUint32(previousData, number)
			if err = m.w.WritePage(m.ctx, previous, previousData); err != nil {
				return e, err
			}
		}
		previous, previousData = number, p
	}
	if err := m.w.WritePage(m.ctx, previous, previousData); err != nil {
		return e, err
	}
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], first)
	e.body = append(e.body, b[:]...)
	return e, nil
}
func (m *mutation) entryParts(e mutableEntry) (size, local, offset int, next uint32, err error) {
	v, k, err := record.DecodeVarint(e.body)
	if err != nil {
		return 0, 0, 0, 0, err
	}
	if v > record.MaxPayload {
		return 0, 0, 0, 0, record.ErrLimit
	}
	offset = k
	if m.kind == TableTree {
		_, k, err = record.DecodeVarint(e.body[offset:])
		if err != nil {
			return 0, 0, 0, 0, err
		}
		offset += k
	}
	size = int(v)
	local = localPayload(size, int(m.h.UsableSize), m.kind)
	want := offset + local
	if local < size {
		want += 4
	}
	if want > len(e.body) {
		return 0, 0, 0, 0, corrupt("truncated B-tree payload")
	}
	if local < size {
		next = binary.BigEndian.Uint32(e.body[offset+local:])
	}
	return
}
func (m *mutation) overflow(e mutableEntry, fn func(uint32, []byte, int) error) error {
	size, local, _, next, err := m.entryParts(e)
	if err != nil {
		return err
	}
	remaining := size - local
	if uint64(remaining) > uint64(m.w.Header().PageCount)*uint64(m.h.UsableSize-4) {
		return corrupt("overflow exceeds database size")
	}
	seen := make(map[uint32]bool)
	for remaining > 0 {
		if next == 0 || seen[next] || m.path[next] {
			return corrupt("invalid overflow chain")
		}
		seen[next] = true
		p, err := m.read(next)
		if err != nil {
			return err
		}
		count := remaining
		if count > int(m.h.UsableSize)-4 {
			count = int(m.h.UsableSize) - 4
		}
		following := binary.BigEndian.Uint32(p)
		if err = fn(next, p[4:4+count], count); err != nil {
			return err
		}
		// The callback consumes this page before the next page is read. None of
		// our callers retain its backing array.
		m.bytes -= len(p)
		remaining -= count
		next = following
	}
	if next != 0 {
		return corrupt("overflow chain exceeds payload")
	}
	return nil
}
func (m *mutation) freeEntry(e mutableEntry) error {
	return m.overflow(e, func(number uint32, _ []byte, _ int) error { return m.w.FreePage(m.ctx, number) })
}
func (m *mutation) key(e mutableEntry) ([]any, error) {
	size, local, off, _, err := m.entryParts(e)
	if err != nil {
		return nil, err
	}
	// Reserve transient encoded/decoded storage while reading the chain.
	// Three times the record size conservatively covers copied text/BLOB data
	// and field slice capacity. The caller consumes one comparison key at a time.
	if size > MaxMutableIndexPayload {
		return nil, fmt.Errorf("%w: mutable index key exceeds %d bytes", record.ErrLimit, MaxMutableIndexPayload)
	}
	reserve := size*3 + (m.info.Columns+1)*32
	if err = m.charge(reserve); err != nil {
		return nil, err
	}
	defer func() { m.bytes -= reserve }()
	p := e.body[off : off+local]
	if local < size {
		p = make([]byte, size)
		copy(p, e.body[off:off+local])
		at := local
		if err = m.overflow(e, func(_ uint32, data []byte, n int) error { copy(p[at:], data); at += n; return nil }); err != nil {
			return nil, err
		}
	}
	key, err := record.Decode(p)
	if err != nil {
		return nil, err
	}
	if err = validateIndexKey(m.info, key); err != nil {
		return nil, err
	}
	return key, nil
}
func (m *mutation) search(n *mutableNode, rowid int64, key []any) (int, bool, error) {
	if m.kind == TableTree {
		i := sort.Search(len(n.entries), func(i int) bool { return n.entries[i].rowid >= rowid })
		return i, i < len(n.entries) && n.entries[i].rowid == rowid, nil
	}
	lo, hi := 0, len(n.entries)
	for lo < hi {
		mid := lo + (hi-lo)/2
		k, err := m.key(n.entries[mid])
		if err != nil {
			return 0, false, err
		}
		if compareIndexKeys(k, key) < 0 {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	if lo == len(n.entries) {
		return lo, false, nil
	}
	k, err := m.key(n.entries[lo])
	if err != nil {
		return 0, false, err
	}
	return lo, compareIndexKeys(k, key) == 0, nil
}
func insertEntry(s []mutableEntry, i int, e mutableEntry) []mutableEntry {
	s = append(s, mutableEntry{})
	copy(s[i+1:], s[i:])
	s[i] = e
	return s
}
func insertChild(s []uint32, i int, c uint32) []uint32 {
	s = append(s, 0)
	copy(s[i+1:], s[i:])
	s[i] = c
	return s
}
func removeEntry(s []mutableEntry, i int) []mutableEntry {
	copy(s[i:], s[i+1:])
	s[len(s)-1] = mutableEntry{}
	return s[:len(s)-1]
}
func removeChild(s []uint32, i int) []uint32 { copy(s[i:], s[i+1:]); return s[:len(s)-1] }
func tableDivider(id int64) mutableEntry {
	return mutableEntry{rowid: id, body: record.AppendVarint(nil, uint64(id))}
}

// splitPoint chooses a byte-balanced legal partition. Index and interior table
// nodes move their median into the parent; table leaves retain every record.
func (m *mutation) splitPoint(n *mutableNode) (int, error) {
	promote := m.kind == IndexTree || !n.leaf
	best, balance := -1, int(^uint(0)>>1)
	// Prefix sums keep partition selection linear even on 64 KiB pages full
	// of tiny keys. Child-pointer space is already included in cellSpace.
	prefix := make([]int, len(n.entries)+1)
	for i, e := range n.entries {
		prefix[i+1] = prefix[i] + cellSpace(e, n.leaf)
	}
	header := 8
	if !n.leaf {
		header = 12
	}
	for i := 1; i < len(n.entries); i++ {
		rightStart := i
		if promote {
			rightStart++
		}
		if rightStart >= len(n.entries) {
			continue
		}
		a, b := header+prefix[i], header+prefix[len(n.entries)]-prefix[rightStart]
		if a > int(m.h.UsableSize) || b > int(m.h.UsableSize) {
			continue
		}
		d := a - b
		if d < 0 {
			d = -d
		}
		if d < balance {
			best, balance = i, d
		}
	}
	if best < 0 {
		return 0, fmt.Errorf("%w: no legal page split", record.ErrLimit)
	}
	return best, nil
}

func (m *mutation) finish(n *mutableNode) (*nodeSplit, error) {
	if m.fits(n) {
		return nil, m.write(n)
	}
	if n.number == 1 {
		child, err := m.allocate(n.leaf)
		if err != nil {
			return nil, err
		}
		child.entries, child.children = n.entries, n.children
		split, err := m.finish(child)
		if err != nil {
			return nil, err
		}
		n.leaf = false
		n.entries = nil
		n.children = []uint32{child.number}
		m.addSplit(n, 0, split)
		return nil, m.write(n)
	}
	at, err := m.splitPoint(n)
	if err != nil {
		if m.kind == TableTree && n.leaf {
			return m.splitTableLeaf(n)
		}
		return nil, err
	}
	right, err := m.allocate(n.leaf)
	if err != nil {
		return nil, err
	}
	start := at
	separator := tableDivider(n.entries[at-1].rowid)
	if m.kind == IndexTree || !n.leaf {
		separator = n.entries[at]
		start++
	}
	right.entries = append([]mutableEntry(nil), n.entries[start:]...)
	n.entries = n.entries[:at]
	if !n.leaf {
		right.children = append([]uint32(nil), n.children[start:]...)
		n.children = n.children[:at+1]
	}
	if err = m.write(n); err != nil {
		return nil, err
	}
	if err = m.write(right); err != nil {
		return nil, err
	}
	return &nodeSplit{separator: separator, right: right.number}, nil
}
func (m *mutation) maxTable(number uint32) (int64, error) {
	seen := make(map[uint32]bool)
	for depth := 0; depth < MaxDepth; depth++ {
		if seen[number] {
			return 0, corrupt("cycle finding table maximum")
		}
		seen[number] = true
		n, err := m.load(number)
		if err != nil {
			return 0, err
		}
		if n.leaf {
			if len(n.entries) == 0 {
				return 0, corrupt("empty non-root table subtree")
			}
			return n.entries[len(n.entries)-1].rowid, nil
		}
		number = n.children[len(n.children)-1]
	}
	return 0, fmt.Errorf("%w: table maximum depth", record.ErrLimit)
}
func (m *mutation) refresh(n *mutableNode, i int) error {
	if m.kind != TableTree || n.leaf || i >= len(n.entries) {
		return nil
	}
	id, err := m.maxTable(n.children[i])
	if err != nil {
		return err
	}
	n.entries[i] = tableDivider(id)
	return nil
}
func (m *mutation) addSplit(n *mutableNode, i int, s *nodeSplit) {
	for ; s != nil; s = s.next {
		n.entries = insertEntry(n.entries, i, s.separator)
		n.children = insertChild(n.children, i+1, s.right)
		i++
	}
}
func (m *mutation) insert(number uint32, depth int, rowid int64, key []any, payload []byte, replace bool) (*nodeSplit, error) {
	if err := m.enter(number, depth); err != nil {
		return nil, err
	}
	defer delete(m.path, number)
	n, err := m.load(number)
	if err != nil {
		return nil, err
	}
	i, found, err := m.search(n, rowid, key)
	if err != nil {
		return nil, err
	}
	if n.leaf || m.kind == IndexTree && found {
		if found && !replace {
			return nil, ErrDuplicate
		}
		if !found && replace {
			return nil, ErrNotFound
		}
		e, err := m.makeEntry(rowid, payload)
		if err != nil {
			return nil, err
		}
		if found {
			if err = m.freeEntry(n.entries[i]); err != nil {
				return nil, err
			}
			n.entries[i] = e
		} else {
			n.entries = insertEntry(n.entries, i, e)
		}
		return m.finish(n)
	}
	split, err := m.insert(n.children[i], depth+1, rowid, key, payload, replace)
	if err != nil {
		return nil, err
	}
	m.addSplit(n, i, split)
	if err = m.refresh(n, i); err != nil {
		return nil, err
	}
	for s, j := split, i+1; s != nil; s, j = s.next, j+1 {
		if err = m.refresh(n, j); err != nil {
			return nil, err
		}
	}
	return m.finish(n)
}

// grow preserves the public root number by moving the former root's left
// partition to a newly allocated child. Page 1's 100-byte header is preserved.
func (m *mutation) grow(root uint32, s *nodeSplit) error {
	if s == nil {
		return nil
	}
	left, err := m.load(root)
	if err != nil {
		return err
	}
	moved, err := m.allocate(left.leaf)
	if err != nil {
		return err
	}
	moved.entries, moved.children = left.entries, left.children
	if err = m.write(moved); err != nil {
		return err
	}
	parent := &mutableNode{number: root, children: []uint32{moved.number}, original: left.original}
	m.addSplit(parent, 0, s)
	return m.write(parent)
}
func insertTable(ctx context.Context, w PageWriter, root uint32, rowid int64, values []any, replace bool) error {
	m, err := newMutation(ctx, w, TableTree, KeyInfo{})
	if err != nil {
		return err
	}
	payload, err := record.Encode(values)
	if err != nil {
		return err
	}
	if err = m.charge(len(payload)); err != nil {
		return err
	}
	s, err := m.insert(root, 0, rowid, nil, payload, replace)
	if err != nil {
		return err
	}
	return m.grow(root, s)
}
func InsertTable(ctx context.Context, w PageWriter, root uint32, rowid int64, values []any) error {
	return insertTable(ctx, w, root, rowid, values, false)
}

// ReplaceTable replaces an existing row without changing its rowid.
func ReplaceTable(ctx context.Context, w PageWriter, root uint32, rowid int64, values []any) error {
	return insertTable(ctx, w, root, rowid, values, true)
}
func InsertIndex(ctx context.Context, w PageWriter, root uint32, info KeyInfo, key []any) error {
	if err := validateIndexKey(info, key); err != nil {
		return err
	}
	m, err := newMutation(ctx, w, IndexTree, info)
	if err != nil {
		return err
	}
	payload, err := record.Encode(key)
	if err != nil {
		return err
	}
	if len(payload) > MaxMutableIndexPayload {
		return fmt.Errorf("%w: mutable index key exceeds %d bytes", record.ErrLimit, MaxMutableIndexPayload)
	}
	if err = m.charge(len(payload)); err != nil {
		return err
	}
	if err = m.charge(len(payload)*2 + (info.Columns+1)*32); err != nil {
		return err
	}
	key, err = record.Decode(payload)
	if err != nil {
		return err
	}
	s, err := m.insert(root, 0, 0, key, payload, false)
	if err != nil {
		return err
	}
	return m.grow(root, s)
}

// rebalance touches only the changed child and one adjacent sibling. It merges
// when their combined contents fit, otherwise redistributes around a new
// divider. Index dividers participate as actual records, never duplicates.
func (m *mutation) rebalance(parent *mutableNode, i int) error {
	if len(parent.children) < 2 {
		return nil
	}
	child, err := m.load(parent.children[i])
	if err != nil {
		return err
	}
	if m.nodeSize(child) >= int(m.h.UsableSize)/3 && len(child.entries) > 0 {
		return m.refresh(parent, i)
	}
	j := i
	if j == len(parent.children)-1 {
		j--
	}
	left := child
	if j != i {
		left, err = m.load(parent.children[j])
		if err != nil {
			return err
		}
	}
	right := child
	if j == i {
		right, err = m.load(parent.children[j+1])
		if err != nil {
			return err
		}
	}
	if left.leaf != right.leaf {
		return corrupt("unequal sibling B-tree levels")
	}
	merged := &mutableNode{number: left.number, leaf: left.leaf, original: left.original}
	merged.entries = append(merged.entries, left.entries...)
	promote := m.kind == IndexTree || !left.leaf
	if promote {
		merged.entries = append(merged.entries, parent.entries[j])
	}
	merged.entries = append(merged.entries, right.entries...)
	if !left.leaf {
		merged.children = append(merged.children, left.children...)
		merged.children = append(merged.children, right.children...)
	}
	if m.fits(merged) {
		if err = m.write(merged); err != nil {
			return err
		}
		if err = m.w.FreePage(m.ctx, right.number); err != nil {
			return err
		}
		parent.entries = removeEntry(parent.entries, j)
		parent.children = removeChild(parent.children, j+1)
		return m.refresh(parent, j)
	}
	at, err := m.splitPoint(merged)
	if err != nil {
		return err
	}
	start := at
	sep := tableDivider(merged.entries[at-1].rowid)
	if promote {
		sep = merged.entries[at]
		start++
	}
	left.entries = append([]mutableEntry(nil), merged.entries[:at]...)
	right.entries = append([]mutableEntry(nil), merged.entries[start:]...)
	if !left.leaf {
		left.children = append([]uint32(nil), merged.children[:at+1]...)
		right.children = append([]uint32(nil), merged.children[start:]...)
	}
	parent.entries[j] = sep
	if err = m.write(left); err != nil {
		return err
	}
	if err = m.write(right); err != nil {
		return err
	}
	return m.refresh(parent, j+1)
}

// erase returns the detached record to its caller. takeMax is used for index
// interior deletion: its predecessor is removed from its leaf and transferred
// into the interior. Only the originally deleted record's overflow is freed.
func (m *mutation) erase(number uint32, depth int, rowid int64, key []any, takeMax bool) (mutableEntry, bool, *nodeSplit, error) {
	if err := m.enter(number, depth); err != nil {
		return mutableEntry{}, false, nil, err
	}
	defer delete(m.path, number)
	n, err := m.load(number)
	if err != nil {
		return mutableEntry{}, false, nil, err
	}
	i, found := len(n.entries), false
	if !takeMax {
		i, found, err = m.search(n, rowid, key)
		if err != nil {
			return mutableEntry{}, false, nil, err
		}
	}
	if n.leaf {
		if takeMax {
			i = len(n.entries) - 1
			found = i >= 0
		}
		if !found {
			return mutableEntry{}, false, nil, nil
		}
		removed := n.entries[i]
		n.entries = removeEntry(n.entries, i)
		split, err := m.finish(n)
		return removed, true, split, err
	}
	var removed mutableEntry
	if m.kind == IndexTree && found {
		removed = n.entries[i]
		pred, ok, split, e := m.erase(n.children[i], depth+1, 0, nil, true)
		if e != nil {
			return mutableEntry{}, false, nil, e
		}
		if !ok {
			return mutableEntry{}, false, nil, corrupt("index predecessor missing")
		}
		n.entries[i] = pred
		m.addSplit(n, i, split)
		if split == nil {
			if err = m.rebalance(n, i); err != nil {
				return mutableEntry{}, false, nil, err
			}
		}
	} else {
		var split *nodeSplit
		removed, found, split, err = m.erase(n.children[i], depth+1, rowid, key, takeMax)
		if err != nil || !found {
			return removed, found, nil, err
		}
		m.addSplit(n, i, split)
		if split == nil {
			if err = m.rebalance(n, i); err != nil {
				return mutableEntry{}, false, nil, err
			}
		} else {
			if err = m.refresh(n, i); err != nil {
				return mutableEntry{}, false, nil, err
			}
			if err = m.refresh(n, i+1); err != nil {
				return mutableEntry{}, false, nil, err
			}
		}
	}
	split, err := m.finish(n)
	return removed, true, split, err
}
func (m *mutation) collapse(root uint32) error {
	for depth := 0; depth < MaxDepth; depth++ {
		n, err := m.load(root)
		if err != nil {
			return err
		}
		if n.leaf || len(n.entries) != 0 {
			return nil
		}
		child, err := m.load(n.children[0])
		if err != nil {
			return err
		}
		old := child.number
		child.number = root
		child.original = n.original
		// Page 1 has 100 fewer usable bytes. SQLite allows its empty interior
		// root to retain a single child when that child cannot fit in page 1.
		if !m.fits(child) {
			return nil
		}
		if err = m.write(child); err != nil {
			return err
		}
		if err = m.w.FreePage(m.ctx, old); err != nil {
			return err
		}
	}
	return fmt.Errorf("%w: root collapse depth", record.ErrLimit)
}
func DeleteTable(ctx context.Context, w PageWriter, root uint32, rowid int64) (bool, error) {
	m, err := newMutation(ctx, w, TableTree, KeyInfo{})
	if err != nil {
		return false, err
	}
	removed, found, s, err := m.erase(root, 0, rowid, nil, false)
	if err != nil || !found {
		return found, err
	}
	if err = m.freeEntry(removed); err != nil {
		return false, err
	}
	if err = m.grow(root, s); err != nil {
		return false, err
	}
	if err = m.collapse(root); err != nil {
		return false, err
	}
	return true, nil
}
func DeleteIndex(ctx context.Context, w PageWriter, root uint32, info KeyInfo, key []any) (bool, error) {
	if err := validateIndexKey(info, key); err != nil {
		return false, err
	}
	m, err := newMutation(ctx, w, IndexTree, info)
	if err != nil {
		return false, err
	}
	// Normalize NaNs to SQL NULL and verify supported storage classes.
	p, err := record.Encode(key)
	if err != nil {
		return false, err
	}
	if len(p) > MaxMutableIndexPayload {
		return false, fmt.Errorf("%w: mutable index key exceeds %d bytes", record.ErrLimit, MaxMutableIndexPayload)
	}
	if err = m.charge(len(p)*3 + (info.Columns+1)*32); err != nil {
		return false, err
	}
	key, err = record.Decode(p)
	if err != nil {
		return false, err
	}
	removed, found, s, err := m.erase(root, 0, 0, key, false)
	if err != nil || !found {
		return found, err
	}
	if err = m.freeEntry(removed); err != nil {
		return false, err
	}
	if err = m.grow(root, s); err != nil {
		return false, err
	}
	if err = m.collapse(root); err != nil {
		return false, err
	}
	return true, nil
}

// SeekTable performs a root-to-leaf rowid lookup, reading only the search path
// and the matching record's overflow pages. A missing key returns found=false.
func SeekTable(ctx context.Context, r PageReader, root uint32, rowid int64) (values []any, found bool, err error) {
	c, err := NewCursor(ctx, r, root)
	if err != nil {
		return nil, false, err
	}
	defer c.Close()
	for len(c.stack) > 0 {
		if err = ctx.Err(); err != nil {
			return nil, false, err
		}
		f := &c.stack[len(c.stack)-1]
		i := sort.Search(len(f.cells), func(i int) bool { return f.cells[i].rowid >= rowid })
		if f.leaf {
			if i == len(f.cells) || f.cells[i].rowid != rowid {
				return nil, false, nil
			}
			p, e := c.payload(f, f.cells[i])
			if e != nil {
				return nil, false, e
			}
			v, e := record.Decode(p)
			if e != nil {
				return nil, false, e
			}
			if e = ctx.Err(); e != nil {
				return nil, false, e
			}
			return v, true, nil
		}
		b := f.bounds
		child := f.right
		if i < len(f.cells) {
			child = f.cells[i].child
			b.upper = f.cells[i].rowid
			b.hasUpper = true
		}
		if i > 0 {
			b.lower = f.cells[i-1].rowid
			b.hasLower = true
		}
		if err = c.push(child, b); err != nil {
			return nil, false, err
		}
	}
	return nil, false, nil
}

// LastTableRowID follows the right edge of a table, without scanning records.
func LastTableRowID(ctx context.Context, r PageReader, root uint32) (int64, bool, error) {
	c, err := NewCursor(ctx, r, root)
	if err != nil {
		return 0, false, err
	}
	defer c.Close()
	for {
		if err = ctx.Err(); err != nil {
			return 0, false, err
		}
		f := &c.stack[len(c.stack)-1]
		if f.leaf {
			if len(f.cells) == 0 {
				return 0, false, nil
			}
			return f.cells[len(f.cells)-1].rowid, true, nil
		}
		b := f.bounds
		if len(f.cells) > 0 {
			b.lower = f.cells[len(f.cells)-1].rowid
			b.hasLower = true
		}
		if err = c.push(f.right, b); err != nil {
			return 0, false, err
		}
	}
}

func (m *mutation) splitTableLeaf(n *mutableNode) (*nodeSplit, error) {
	entries := n.entries
	groups := make([][]mutableEntry, 0, 3)
	for start := 0; start < len(entries); {
		end, size := start, 8
		for end < len(entries) && size+cellSpace(entries[end], true) <= int(m.h.UsableSize) {
			size += cellSpace(entries[end], true)
			end++
		}
		if end == start {
			return nil, fmt.Errorf("%w: table leaf cell exceeds page", record.ErrLimit)
		}
		groups = append(groups, entries[start:end])
		start = end
	}
	if len(groups) > 3 {
		return nil, corrupt("table leaf insertion needs more than three pages")
	}
	n.entries = groups[0]
	if err := m.write(n); err != nil {
		return nil, err
	}
	var first, last *nodeSplit
	for i := 1; i < len(groups); i++ {
		right, err := m.allocate(true)
		if err != nil {
			return nil, err
		}
		right.entries = groups[i]
		if err = m.write(right); err != nil {
			return nil, err
		}
		s := &nodeSplit{separator: tableDivider(groups[i-1][len(groups[i-1])-1].rowid), right: right.number}
		if first == nil {
			first = s
		} else {
			last.next = s
		}
		last = s
	}
	return first, nil
}
