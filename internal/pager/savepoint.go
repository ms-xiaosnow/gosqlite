package pager

import (
	"context"
	"errors"
	"fmt"
)

// Each mark retains immutable page images and independent dirty/freelist maps.
// Charging every referenced page at every mark intentionally overestimates
// shared versions. Unlike a per-mark cap, the aggregate never grows with depth
// beyond the configured limit. Entry allowances include map capacity/metadata;
// the mark allowance also includes the SQL coordinator's name and stack entry.
const undoMarkBytes uint64 = 1024
const undoPageEntryBytes uint64 = 64
const undoFreeEntryBytes uint64 = 32

type undoMark struct {
	id     uint64
	header Header
	dirty  map[uint32][]byte
	free   map[uint32]struct{}
	charge uint64
}
type undoState struct {
	marks []*undoMark
	bytes uint64
	next  uint64
}

// SavepointMetadataBytes is also charged before a lazy SQL transaction has a
// pager snapshot. Callers must clone names retained from a parsed SQL buffer.
func SavepointMetadataBytes(name string) uint64 { return undoMarkBytes + 2*uint64(len(name)) }

// UndoLimit uses the actual database page size, not the creation default. The
// derived limit admits one worst-case preexisting Statement snapshot under
// both older page/freelist quotas. Arithmetic stays below 2^49 even when both
// uint32 limits and page size are maximal. An explicit byte limit is strict.
func UndoLimit(o Options, pageSize uint32) uint64 {
	if o.MaxUndoBytes > 0 {
		return uint64(o.MaxUndoBytes)
	}
	if o.MaxDirtyPages == 0 {
		o.MaxDirtyPages = 4096
	}
	if o.MaxFreelistPages == 0 {
		o.MaxFreelistPages = 1 << 20
	}
	if pageSize == 0 {
		pageSize = 4096
	}
	return uint64(o.MaxDirtyPages)*(uint64(pageSize)+undoPageEntryBytes) + uint64(o.MaxFreelistPages)*undoFreeEntryBytes + 65536
}

// Copy entries into length-sized maps instead of maps.Clone: the original
// freelist map may have a much larger allocation after many pages are reused.
// Retaining that spare capacity in every mark would invalidate len-based charges.
func cloneUndoPages(source map[uint32][]byte) map[uint32][]byte {
	result := make(map[uint32][]byte, len(source))
	for n, p := range source {
		result[n] = p
	}
	return result
}
func cloneUndoFree(source map[uint32]struct{}) map[uint32]struct{} {
	result := make(map[uint32]struct{}, len(source))
	for n := range source {
		result[n] = struct{}{}
	}
	return result
}
func (t *Tx) newUndoMarkLocked(name string) (*undoMark, error) {
	charge := SavepointMetadataBytes(name) + uint64(len(t.dirty))*(uint64(t.header.PageSize)+undoPageEntryBytes) + uint64(len(t.free))*undoFreeEntryBytes
	limit := UndoLimit(t.opts, t.header.PageSize)
	if charge > limit || t.undo.bytes > limit-charge || t.undo.next == ^uint64(0) {
		return nil, fmt.Errorf("%w: aggregate transaction undo memory", ErrLimit)
	}
	// Reserve/check all retained metadata and images before allocating maps.
	m := &undoMark{id: t.undo.next + 1, header: t.header, dirty: cloneUndoPages(t.dirty), free: cloneUndoFree(t.free), charge: charge}
	t.undo.next = m.id
	t.undo.bytes += charge
	t.undo.marks = append(t.undo.marks, m)
	return m, nil
}
func (t *Tx) discardUndoLocked(first int) {
	for _, m := range t.undo.marks[first:] {
		t.undo.bytes -= m.charge
	}
	clear(t.undo.marks[first:])
	if first == 0 {
		t.undo.marks = nil
	} else {
		// Do not retain a large former stack backing array after freeing marks.
		kept := make([]*undoMark, first)
		copy(kept, t.undo.marks[:first])
		t.undo.marks = kept
	}
}
func (t *Tx) undoIndexLocked(id uint64) int {
	for i := len(t.undo.marks) - 1; i >= 0; i-- {
		if t.undo.marks[i].id == id {
			return i
		}
	}
	return -1
}
func (t *Tx) checkUndoControlLocked(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := t.checkOpen(); err != nil {
		return err
	}
	if t.statement {
		return fmt.Errorf("%w: savepoint control inside statement", ErrMisuse)
	}
	return nil
}

// Savepoint records a memory-only rollback boundary. Names are owned by the SQL
// coordinator; the opaque id disambiguates repeated names without string state.
func (t *Tx) Savepoint(ctx context.Context, name string) (uint64, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := t.checkUndoControlLocked(ctx); err != nil {
		return 0, err
	}
	m, err := t.newUndoMarkLocked(name)
	if err != nil {
		return 0, err
	}
	return m.id, nil
}

// Release discards this and all younger marks without publishing a transaction.
func (t *Tx) Release(ctx context.Context, id uint64) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := t.checkUndoControlLocked(ctx); err != nil {
		return err
	}
	i := t.undoIndexLocked(id)
	if i < 0 {
		return fmt.Errorf("%w: unknown savepoint", ErrMisuse)
	}
	t.discardUndoLocked(i)
	return nil
}

// RollbackTo restores pages, logical extent, freelist and header, retains the
// named mark, and discards younger marks. Cloned maps keep the retained mark
// immutable, so repeated rollback-to after new writes remains correct.
func (t *Tx) RollbackTo(ctx context.Context, id uint64) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := t.checkUndoControlLocked(ctx); err != nil {
		return err
	}
	i := t.undoIndexLocked(id)
	if i < 0 {
		return fmt.Errorf("%w: unknown savepoint", ErrMisuse)
	}
	m := t.undo.marks[i]
	t.dirty, t.free, t.header = cloneUndoPages(m.dirty), cloneUndoFree(m.free), m.header
	t.discardUndoLocked(i + 1)
	return nil
}
func (t *Tx) statementUndo(ctx context.Context, fn func() error) (err error) {
	if fn == nil {
		return ErrMisuse
	}
	t.mu.Lock()
	if err = t.checkUndoControlLocked(ctx); err != nil {
		t.mu.Unlock()
		return err
	}
	m, err := t.newUndoMarkLocked("")
	if err != nil {
		t.mu.Unlock()
		return err
	}
	t.statement = true
	t.mu.Unlock()
	complete := false
	defer func() {
		t.mu.Lock()
		defer t.mu.Unlock()
		if !complete || err != nil {
			t.dirty, t.free, t.header = m.dirty, m.free, m.header
		}
		t.discardUndoLocked(len(t.undo.marks) - 1)
		t.statement = false
		t.clearClosedUndoLocked()
	}()
	err = fn()
	if err == nil {
		err = ctx.Err()
	}
	complete = true
	return err
}
func (t *Tx) clearClosedUndoLocked() {
	if t.statement || t.file != nil || t.encrypted != nil {
		return
	}
	// No mark may keep transaction page images alive after lease/file release.
	// Clear owned buffers too; as elsewhere this is not a Go GC erasure guarantee.
	for _, m := range t.undo.marks {
		for _, p := range m.dirty {
			clear(p)
		}
	}
	t.undo = undoState{}
}

// Exclusive acquires rollback-mode EXCLUSIVE now. Shared WAL uses its writer
// lock; exclusive WAL/encrypted sessions already own stronger lifetime locks.
func (t *Tx) Exclusive(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := t.promoteLocked(ctx); err != nil {
		return err
	}
	if t.shared != nil || t.session != nil || t.encrypted != nil || t.exclusive {
		return nil
	}
	if err := lockExclusive(t.file); err != nil {
		if errors.Is(err, ErrPoisoned) {
			return t.poison(err, false)
		}
		return err
	}
	t.exclusive = true
	return nil
}
