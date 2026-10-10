package pager

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/ms-xiaosnow/gosqlite/internal/journal"

	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

// writeFullAt treats short writes as errors, even if the OS reports no error.
func writeFullAt(f *vfs.File, b []byte, off int64) error {
	n, err := f.WriteAt(b, off)
	if err != nil {
		return err
	}
	if n != len(b) {
		return io.ErrShortWrite
	}
	return nil
}
func syncDirectory(path string) error { return nativeFileError(vfs.SyncDirectory(filepath.Dir(path))) }
func sortedPages(m map[uint32][]byte) []uint32 {
	out := make([]uint32, 0, len(m))
	for n := range m {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// journalPages includes original images of complete storage sectors touched
// by the write, including old neighbors of newly appended pages. No original
// image is required for pages beyond the pre-transaction database extent.
func (t *Tx) journalPages(dirty []uint32) []uint32 {
	set := make(map[uint32]struct{})
	for _, n := range dirty {
		first, last := uint64(n), uint64(n)
		if t.opts.SectorSize > t.header.PageSize {
			per := uint64(t.opts.SectorSize / t.header.PageSize)
			first = (uint64(n)-1)/per*per + 1
			last = first + per - 1
		}
		if last > uint64(t.originalPages) {
			last = uint64(t.originalPages)
		}
		for p := first; p <= last && p != 0; p++ {
			if p != uint64(lockPage(t.header.PageSize)) {
				set[uint32(p)] = struct{}{}
			}
		}
	}
	pages := make([]uint32, 0, len(set))
	for n := range set {
		pages = append(pages, n)
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i] < pages[j] })
	return pages
}

// Commit writes a standard SQLite DELETE-mode rollback journal, then the
// database. Order: EXCLUSIVE; invalid header + body; journal Sync; valid header
// + Sync; directory Sync; database writes + Sync; journal unlink; directory
// Sync; release locks. Atomicity after interruption is NOT acceptance-verified.
//
// Cancellation is checked until the journal is created. Once physical I/O
// starts, the entire commit or error cleanup completes without cancellation.
// A BUSY lock is returned before creating any sidecar or writing database data.
func (t *Tx) Commit(ctx context.Context) (retErr error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	defer t.clearClosedUndoLocked()
	cleanup := t.plainScope()
	if cleanup != nil {
		defer func() { retErr = cleanup.observe(retErr) }()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := t.checkOpen(); err != nil {
		return err
	}
	if t.statement {
		return fmt.Errorf("%w: commit inside statement", ErrMisuse)
	}
	if t.encrypted != nil {
		return t.commitEncryptedLocked(ctx)
	}
	if t.shared != nil {
		return t.commitSharedWALLocked(ctx)
	}
	if t.session != nil {
		return t.commitWALLocked(ctx)
	}
	if err := t.checkStable(); err != nil {
		return err
	}
	if len(t.dirty) == 0 {
		if t.cleanup != nil {
			return t.retirePlainLocked(nil, nil, true)
		}
		var err error
		if !t.keepFile {
			err = closeNativeFile(t.file)
		}
		t.file = nil
		t.free = nil
		return err
	}
	if err := t.promoteLocked(ctx); err != nil {
		return err
	}
	p1, err := t.readLocked(ctx, 1)
	if err != nil {
		return err
	}
	finalHeader := t.header
	finalHeader.ChangeCounter++
	finalHeader.VersionValidFor = finalHeader.ChangeCounter
	binary.BigEndian.PutUint32(p1[24:], finalHeader.ChangeCounter)
	binary.BigEndian.PutUint32(p1[28:], finalHeader.PageCount)
	binary.BigEndian.PutUint32(p1[92:], finalHeader.VersionValidFor)
	dirty := sortedPages(t.dirty)
	if len(dirty) == 0 || dirty[0] != 1 {
		return fmt.Errorf("%w: dirty transaction lacks page one", ErrMisuse)
	}
	originals := t.journalPages(dirty)
	var random [4]byte
	if _, err = rand.Read(random[:]); err != nil {
		return err
	}
	h := journal.Header{Records: uint32(len(originals)), Nonce: binary.BigEndian.Uint32(random[:]), OriginalPages: t.originalPages, SectorSize: t.opts.SectorSize, PageSize: t.header.PageSize}
	invalid, err := journal.EncodeHeader(h, false)
	if err != nil {
		return err
	}
	valid, err := journal.EncodeHeader(h, true)
	if err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = lockExclusive(t.file); err != nil {
		if errors.Is(err, ErrPoisoned) {
			return t.poison(err, false)
		}
		return err
	}
	// Pre-I/O failures remain retryable and release PENDING/EXCLUSIVE while
	// retaining this transaction's SHARED+RESERVED locks.
	preflight := func(e error, journalAcquired bool) error {
		if cleanup != nil {
			e = cleanup.observe(e)
			if errors.Is(e, ErrPlaintextCleanupPending) {
				return t.deferPlainRetirementLocked(journalAcquired)
			}
		}
		if t.keepFile || t.exclusive {
			return e
		}
		if unlock := unlockExclusive(t.file); unlock != nil {
			return t.poison(errors.Join(e, unlock), false)
		}
		return e
	}
	if err = t.checkStable(); err != nil {
		return preflight(err, false)
	}
	if err = ctx.Err(); err != nil {
		return preflight(err, false)
	}
	jpath := t.path + "-journal"
	if cleanup != nil {
		if err = cleanup.reserveTemporary(); err != nil {
			return preflight(err, false)
		}
	}
	jf, err := openJournal(jpath, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if cleanup != nil {
		cleanup.acquiredTemporary(jf)
	}
	if err != nil {
		// The journal constructor may already have created the sidecar.
		return preflight(err, true)
	}
	databaseTouched := false
	// After a database write is attempted, even an error may have changed
	// bytes. Preserve the fully synced journal for explicit recovery.
	fail := func(cause error) error {
		if cleanup != nil {
			cause = cleanup.closeTemporary(cause)
			if errors.Is(cause, ErrPlaintextCleanupPending) {
				return t.deferPlainRetirementLocked(true)
			}
		} else {
			cause = errors.Join(cause, jf.Close())
		}
		if !databaseTouched {
			if e := os.Remove(jpath); e != nil {
				return t.poison(errors.Join(cause, e), true)
			}
			if e := syncDirectory(t.path); e != nil {
				return t.poison(errors.Join(cause, e), true)
			}
		}
		return t.poison(cause, databaseTouched)
	}
	if err = writeFullAt(jf, invalid, 0); err != nil {
		return fail(err)
	}
	offset := int64(h.SectorSize)
	page := make([]byte, h.PageSize)
	for _, n := range originals {
		if _, err = t.file.ReadAt(page, int64(n-1)*int64(h.PageSize)); err != nil {
			return fail(err)
		}
		record, e := journal.EncodeRecord(h, n, page)
		if e != nil {
			return fail(e)
		}
		if err = writeFullAt(jf, record, offset); err != nil {
			return fail(err)
		}
		offset += int64(len(record))
	}
	if err = jf.Sync(); err != nil {
		return fail(err)
	}
	// Publish only the first 12 bytes after the body is durable. All remaining
	// header fields were included in the first journal Sync.
	if err = writeFullAt(jf, valid[:12], 0); err != nil {
		return fail(err)
	}
	if err = jf.Sync(); err != nil {
		return fail(err)
	}
	if err = syncDirectory(t.path); err != nil {
		return fail(err)
	}
	for _, n := range dirty {
		data := t.dirty[n]
		if n == 1 {
			data = p1
		}
		databaseTouched = true
		if err = writeFullAt(t.file, data, int64(n-1)*int64(t.header.PageSize)); err != nil {
			return fail(err)
		}
	}
	// Extent only grows here. Freelist pages are retained; no unrelated physical
	// trailing pages are ever truncated.
	if err = t.file.Sync(); err != nil {
		return fail(err)
	}
	if cleanup != nil {
		err = cleanup.closeTemporary(nil)
	} else {
		err = jf.Close()
	}
	if err != nil {
		if cleanup != nil && errors.Is(err, ErrPlaintextCleanupPending) {
			return t.deferPlainRetirementLocked(true)
		}
		return t.poison(err, true)
	}
	if err = os.Remove(jpath); err != nil {
		return t.poison(err, true)
	}
	if err = syncDirectory(t.path); err != nil {
		return t.poison(errors.Join(ErrCommitUncertain, err), false)
	}
	t.header = finalHeader
	if t.cleanup != nil {
		return t.retirePlainLocked(nil, errors.Join(ErrPoisoned, ErrCommitUncertain), true)
	}
	if !t.keepFile {
		err = closeNativeFile(t.file)
	}
	t.file = nil
	t.dirty = nil
	t.free = nil
	if err != nil {
		t.poisoned = errors.Join(ErrPoisoned, ErrCommitUncertain, err)
		return t.poisoned
	}
	return nil
}
func (t *Tx) poison(cause error, recovery bool) error {
	if t.borrowedCleanup != nil {
		if recovery {
			cause = errors.Join(ErrRecoveryRequired, cause)
		}
		t.file, t.dirty, t.free = nil, nil, nil
		t.clearClosedUndoLocked()
		t.poisoned = t.borrowedCleanup.observe(errors.Join(ErrPoisoned, cause))
		return t.poisoned
	}
	if t.cleanup != nil {
		if recovery {
			cause = errors.Join(ErrRecoveryRequired, cause)
		}
		return t.retirePlainLocked(errors.Join(ErrPoisoned, cause), nil, true)
	}
	if t.encrypted != nil {
		s := t.encrypted
		s.mu.Lock()
		s.poisoned = errors.Join(ErrPoisoned, cause)
		t.poisoned = s.poisoned
		s.active = nil
		s.mu.Unlock()
		t.clearEncrypted()
		return t.poisoned
	}
	if t.session != nil {
		t.session.mu.Lock()
		t.poisoned = t.session.poisonLocked(cause)
		t.session.active = nil
		t.session.mu.Unlock()
		t.file, t.dirty, t.free = nil, nil, nil
		return t.poisoned
	}
	if t.file != nil {
		cause = errors.Join(cause, closeNativeFile(t.file))
		t.file = nil
	}
	if recovery {
		cause = errors.Join(ErrRecoveryRequired, cause)
	}
	t.poisoned = errors.Join(ErrPoisoned, cause)
	t.dirty = nil
	t.free = nil
	return t.poisoned
}
