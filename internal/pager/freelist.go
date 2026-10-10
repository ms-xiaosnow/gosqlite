package pager

import (
	"context"
	"encoding/binary"
	"fmt"
)

func (t *Tx) loadFreelist(ctx context.Context) error {
	count := binary.BigEndian.Uint32(t.originalHeader[36:])
	if count > t.opts.MaxFreelistPages {
		return fmt.Errorf("%w: freelist validation metadata", ErrLimit)
	}
	trunk := binary.BigEndian.Uint32(t.originalHeader[32:])
	add := func(n uint32) error {
		if n < 2 || n > t.header.PageCount || n == lockPage(t.header.PageSize) {
			return fmt.Errorf("%w: invalid freelist page %d", ErrCorrupt, n)
		}
		if _, ok := t.free[n]; ok {
			return fmt.Errorf("%w: duplicate or cyclic freelist page %d", ErrCorrupt, n)
		}
		if uint32(len(t.free)) >= count {
			return fmt.Errorf("%w: freelist exceeds declared count", ErrCorrupt)
		}
		t.free[n] = struct{}{}
		return nil
	}
	for trunk != 0 {
		if err := add(trunk); err != nil {
			return err
		}
		p, err := t.readLocked(ctx, trunk)
		if err != nil {
			return err
		}
		next, n := binary.BigEndian.Uint32(p), binary.BigEndian.Uint32(p[4:])
		if n > t.header.UsableSize/4-2 {
			return fmt.Errorf("%w: freelist trunk count", ErrCorrupt)
		}
		for i := uint32(0); i < n; i++ {
			if err = add(binary.BigEndian.Uint32(p[8+4*i:])); err != nil {
				return err
			}
		}
		trunk = next
	}
	if uint32(len(t.free)) != count {
		return fmt.Errorf("%w: freelist count mismatch", ErrCorrupt)
	}
	return nil
}

// AllocatePage reuses a validated freelist leaf or trunk before extending the
// logical extent. The reserved lock-byte page is never returned.
func (t *Tx) AllocatePage(ctx context.Context) (uint32, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := t.promoteLocked(ctx); err != nil {
		return 0, err
	}
	p1, err := t.readLocked(ctx, 1)
	if err != nil {
		return 0, err
	}
	changes := map[uint32][]byte{1: p1}
	head := binary.BigEndian.Uint32(p1[32:])
	count := binary.BigEndian.Uint32(p1[36:])
	var n uint32
	nextCount := t.header.PageCount
	if head != 0 {
		trunk, e := t.readLocked(ctx, head)
		if e != nil {
			return 0, e
		}
		leaves := binary.BigEndian.Uint32(trunk[4:])
		if leaves > t.header.UsableSize/4-2 || count == 0 {
			return 0, fmt.Errorf("%w: freelist trunk", ErrCorrupt)
		}
		if leaves > 0 {
			n = binary.BigEndian.Uint32(trunk[8+4*(leaves-1):])
			binary.BigEndian.PutUint32(trunk[4:], leaves-1)
			binary.BigEndian.PutUint32(trunk[8+4*(leaves-1):], 0)
			changes[head] = trunk
		} else {
			n = head
			binary.BigEndian.PutUint32(p1[32:], binary.BigEndian.Uint32(trunk))
		}
		if _, ok := t.free[n]; !ok {
			return 0, fmt.Errorf("%w: untracked free page", ErrCorrupt)
		}
		binary.BigEndian.PutUint32(p1[36:], count-1)
	} else {
		if count != 0 {
			return 0, fmt.Errorf("%w: freelist head", ErrCorrupt)
		}
		if nextCount >= 0xfffffffe {
			return 0, ErrLimit
		}
		n = nextCount + 1
		if n == lockPage(t.header.PageSize) {
			n++
		}
		if n > 0xfffffffe {
			return 0, ErrLimit
		}
		nextCount = n
		binary.BigEndian.PutUint32(p1[28:], nextCount)
	}
	changes[n] = make([]byte, t.header.PageSize)
	if err = t.applyPages(changes); err != nil {
		return 0, err
	}
	delete(t.free, n)
	t.header.PageCount = nextCount
	return n, nil
}

// FreePage adds an allocated page to the freelist. It does not shrink the
// database, overwrite unrelated free leaves, or change secure_delete policy.
func (t *Tx) FreePage(ctx context.Context, n uint32) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := t.promoteLocked(ctx); err != nil {
		return err
	}
	if n < 2 || n > t.header.PageCount || n == lockPage(t.header.PageSize) {
		return fmt.Errorf("%w: invalid freed page", ErrCorrupt)
	}
	if _, ok := t.free[n]; ok {
		return fmt.Errorf("%w: double free of page %d", ErrMisuse, n)
	}
	if uint64(len(t.free))+1 > uint64(t.opts.MaxFreelistPages) {
		return fmt.Errorf("%w: freelist metadata", ErrLimit)
	}
	p1, err := t.readLocked(ctx, 1)
	if err != nil {
		return err
	}
	changes := map[uint32][]byte{1: p1}
	head, count := binary.BigEndian.Uint32(p1[32:]), binary.BigEndian.Uint32(p1[36:])
	newHead := true
	if head != 0 {
		trunk, e := t.readLocked(ctx, head)
		if e != nil {
			return e
		}
		leaves := binary.BigEndian.Uint32(trunk[4:])
		if leaves > t.header.UsableSize/4-2 {
			return fmt.Errorf("%w: freelist trunk count", ErrCorrupt)
		}
		// Leave the last six slots unused, as SQLite does for legacy readers.
		if leaves < t.header.UsableSize/4-8 {
			binary.BigEndian.PutUint32(trunk[8+4*leaves:], n)
			binary.BigEndian.PutUint32(trunk[4:], leaves+1)
			changes[head] = trunk
			newHead = false
		}
	}
	if newHead {
		p := make([]byte, t.header.PageSize)
		binary.BigEndian.PutUint32(p, head)
		changes[n] = p
		binary.BigEndian.PutUint32(p1[32:], n)
	}
	binary.BigEndian.PutUint32(p1[36:], count+1)
	if err = t.applyPages(changes); err != nil {
		return err
	}
	t.free[n] = struct{}{}
	return nil
}

// FreePages returns an independent snapshot of all validated freelist pages,
// both trunks and leaves. The B-tree/schema layer must check global live/free
// ownership before authorizing writes to an existing database.
func (t *Tx) FreePages() []uint32 {
	t.mu.Lock()
	defer t.mu.Unlock()
	pages := make([]uint32, 0, len(t.free))
	for n := range t.free {
		pages = append(pages, n)
	}
	return pages
}
