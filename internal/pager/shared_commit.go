package pager

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/ms-xiaosnow/gosqlite/internal/wal"
	"github.com/ms-xiaosnow/gosqlite/internal/walindex"
)

func (t *Tx) commitSharedWALLocked(ctx context.Context) error {
	if err := t.checkStable(); err != nil {
		return err
	}
	if len(t.dirty) == 0 {
		err := t.shared.release(t)
		t.file, t.dirty, t.free = nil, nil, nil
		return err
	}
	if err := t.promoteLocked(ctx); err != nil {
		return err
	}
	images, err := walSectorImages(ctx, t.dirty, t.header, t.opts, t.readLocked)
	if err != nil {
		return err
	}
	p1, err := t.readLocked(ctx, 1)
	if err != nil {
		return err
	}
	final := t.header
	final.ChangeCounter++
	final.VersionValidFor = final.ChangeCounter
	binary.BigEndian.PutUint32(p1[24:], final.ChangeCounter)
	binary.BigEndian.PutUint32(p1[28:], final.PageCount)
	binary.BigEndian.PutUint32(p1[92:], final.VersionValidFor)
	images[1] = p1
	s := t.shared
	s.mu.Lock()
	defer s.mu.Unlock()
	if err = s.checkStableLocked(); err != nil {
		return err
	}
	if s.active != t || !t.sharedRead.write {
		return ErrMisuse
	}
	if err = s.appendSharedLocked(ctx, t, images, final); err != nil {
		if s.poisoned != nil {
			t.poisoned = s.poisoned
			err = errors.Join(err, s.releaseLocked(t))
			t.file, t.dirty, t.free = nil, nil, nil
		}
		return err
	}
	s.header = final
	t.header = final
	err = s.releaseLocked(t)
	t.file, t.dirty, t.free = nil, nil, nil
	return err
}

// appendProtectedLocked proves that a conservative append cannot destroy the
// only equivalent committed image in its first touched storage sector. This
// relies on complete identical adjacent commit-frame padding, never a guess
// about an imported client's VFS. Unknown padding requires an explicit PSOW
// assertion or a completed checkpoint followed by safe generation reset.
func (s *SharedSession) appendProtectedLocked(h walindex.Header) (bool, error) {
	if s.opts.WALPowersafeOverwrite || h.MaxFrame == 0 {
		return true, nil
	}
	frameSize := int64(wal.FrameHeaderSize) + int64(h.PageSize)
	end := int64(wal.HeaderSize) + int64(h.MaxFrame)*frameSize
	boundary := end / int64(s.opts.SectorSize) * int64(s.opts.SectorSize)
	if end == boundary {
		return true, nil
	}
	var last []byte
	for n := h.MaxFrame; n > 0; n-- {
		b := make([]byte, frameSize)
		off := int64(wal.HeaderSize) + int64(n-1)*frameSize
		if _, err := readNativeAt(s.walFile, b, off); err != nil {
			return false, err
		}
		if binary.BigEndian.Uint32(b[4:8]) != h.DBSize || !bytes.Equal(b[8:16], h.Salt[:]) {
			return false, nil
		}
		if n == h.MaxFrame && (binary.BigEndian.Uint32(b[16:20]) != h.FrameChecksum[0] || binary.BigEndian.Uint32(b[20:24]) != h.FrameChecksum[1]) {
			return false, fmt.Errorf("%w: shared WAL commit checksum differs from index", ErrCorrupt)
		}
		if last != nil && (!bytes.Equal(last[:8], b[:8]) || !bytes.Equal(last[wal.FrameHeaderSize:], b[wal.FrameHeaderSize:])) {
			return false, nil
		}
		if off+frameSize <= boundary {
			return true, nil
		}
		last = b
	}
	return false, nil
}

// restartSharedLocked implements walRestartLog/walRestartHdr. Writer and a
// current read lease are held. Existing slot-0 readers remain valid because
// they ignore the WAL; all WAL-reading slots must be exclusively available.
func (s *SharedSession) restartSharedLocked(t *Tx) (restarted bool, err error) {
	h := t.sharedRead.hdr
	backfill, err := s.index.Backfill()
	if err != nil {
		return false, sharedIndexError(err)
	}
	if h.MaxFrame == 0 || backfill != h.MaxFrame {
		return false, nil
	}
	oldSlot := t.sharedRead.slot
	if oldSlot != 0 {
		if err = s.index.Lock(walindex.ReadLock0, 1, false); err != nil {
			if errors.Is(err, walindex.ErrBusy) {
				return false, nil
			}
			return false, sharedIndexError(err)
		}
		if err = s.unlockLocked(walindex.ReadLock0+oldSlot, 1); err != nil {
			return false, s.poisonLocked(sharedIndexError(err))
		}
		t.sharedRead.slot = 0
	}
	if err = s.index.Lock(walindex.ReadLock0+1, walindex.ReaderCount-1, true); err != nil {
		if errors.Is(err, walindex.ErrBusy) {
			return false, nil
		}
		return false, sharedIndexError(err)
	}
	defer s.unlockDeferred(walindex.ReadLock0+1, walindex.ReaderCount-1, &err)
	var random [4]byte
	if _, err = rand.Read(random[:]); err != nil {
		return false, err
	}
	h.MaxFrame = 0
	binary.BigEndian.PutUint32(h.Salt[:4], binary.BigEndian.Uint32(h.Salt[:4])+1)
	copy(h.Salt[4:], random[:])
	if err = s.index.Publish(h); err != nil {
		return false, s.poisonLocked(sharedIndexError(err))
	}
	if err = s.index.SetBackfill(0); err != nil {
		return false, s.poisonLocked(sharedIndexError(err))
	}
	if err = s.index.SetAttempted(0); err != nil {
		return false, s.poisonLocked(sharedIndexError(err))
	}
	for i := 1; i < walindex.ReaderCount; i++ {
		mark := walindex.UnusedReadMark
		if i == 1 {
			mark = 0
		}
		if err = s.index.SetReadMark(i, mark); err != nil {
			return false, s.poisonLocked(sharedIndexError(err))
		}
	}
	h, err = s.index.ReadHeader()
	if err != nil {
		return false, s.poisonLocked(sharedIndexError(err))
	}
	t.sharedRead.hdr = h
	t.sharedRead.minFrame = 1
	return true, nil
}

func (s *SharedSession) appendSharedLocked(ctx context.Context, t *Tx, images map[uint32][]byte, final Header) error {
	pages := sortedPages(images)
	if len(pages) == 0 || pages[0] != 1 {
		return ErrMisuse
	}
	if final.PageCount < t.sharedRead.hdr.DBSize {
		return fmt.Errorf("%w: native WAL shrink", ErrUnsupported)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	// Resetting a fully checkpointed generation also solves imported padding
	// differences without ever changing the WAL inode. If readers retain the
	// old WAL generation, the conservative append proof must succeed instead.
	if _, err := s.restartSharedLocked(t); err != nil {
		return err
	}
	h := t.sharedRead.hdr
	safe, err := s.appendProtectedLocked(h)
	if err != nil {
		return err
	}
	if !safe {
		return fmt.Errorf("%w: imported WAL lacks sector-safe commit padding; checkpoint after readers finish, or explicitly assert WALPowersafeOverwrite", ErrUnsupported)
	}
	frameSize := int64(wal.FrameHeaderSize) + int64(final.PageSize)
	extra := (int64(s.opts.SectorSize) + frameSize - 1) / frameSize
	if uint64(h.MaxFrame)+uint64(len(pages))+uint64(extra) > 1<<20 || int64(wal.HeaderSize)+(int64(h.MaxFrame)+int64(len(pages))+extra)*frameSize > 1<<30 {
		return fmt.Errorf("%w: shared WAL growth; checkpoint required", ErrLimit)
	}
	if err = s.index.Cleanup(h.MaxFrame); err != nil {
		return sharedIndexError(err)
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	markerAttempted := false
	fail := func(e error) error {
		if markerAttempted {
			e = errors.Join(ErrCommitUncertain, e)
		}
		return s.poisonLocked(e)
	}
	wh := wal.Header{Magic: wal.MagicLittleEndian, Version: wal.Version, PageSize: final.PageSize, Salt: [2]uint32{binary.BigEndian.Uint32(h.Salt[:4]), binary.BigEndian.Uint32(h.Salt[4:])}}
	if h.BigEndianChecksum {
		wh.Magic = wal.MagicBigEndian
	}
	checksum := h.FrameChecksum
	if h.MaxFrame == 0 {
		var old [wal.HeaderSize]byte
		n, readErr := readNativeAt(s.walFile, old[:], 0)
		if errors.Is(readErr, ErrPoisoned) {
			return s.poisonLocked(readErr)
		}
		if n == len(old) {
			if prev, e := wal.ParseHeader(old[:]); e == nil {
				wh.CheckpointSequence = prev.CheckpointSequence
				if !bytes.Equal(old[16:24], h.Salt[:]) {
					wh.CheckpointSequence++
				}
			}
		}
		if h.Salt == [8]byte{} {
			if _, err = rand.Read(h.Salt[:]); err != nil {
				return err
			}
			wh.Salt = [2]uint32{binary.BigEndian.Uint32(h.Salt[:4]), binary.BigEndian.Uint32(h.Salt[4:])}
		}
		raw, e := wal.EncodeHeader(wh)
		if e != nil {
			return e
		}
		wh, e = wal.ParseHeader(raw)
		if e != nil {
			return e
		}
		if err = writeFullAt(s.walFile, raw, 0); err != nil {
			return fail(err)
		}
		if err = s.walFile.Sync(); err != nil {
			return fail(err)
		}
		if err = syncDirectory(s.path); err != nil {
			return fail(err)
		}
		checksum = wh.Checksum
	}
	offset := int64(wal.HeaderSize) + int64(h.MaxFrame)*frameSize
	frames := h.MaxFrame
	appendFrame := func(n, dbSize uint32, p []byte) error {
		b, next, e := wal.EncodeFrame(wh, n, dbSize, p, checksum)
		if e != nil {
			return e
		}
		if dbSize != 0 {
			markerAttempted = true
		}
		if e = writeFullAt(s.walFile, b, offset); e != nil {
			return e
		}
		frames++
		offset += int64(len(b))
		checksum = next
		return sharedIndexError(s.index.Append(frames, n))
	}
	for i, n := range pages {
		dbSize := uint32(0)
		if i == len(pages)-1 {
			dbSize = final.PageCount
		}
		if err = appendFrame(n, dbSize, images[n]); err != nil {
			return fail(err)
		}
	}
	boundary := (offset + int64(s.opts.SectorSize) - 1) / int64(s.opts.SectorSize) * int64(s.opts.SectorSize)
	last := pages[len(pages)-1]
	for offset < boundary {
		if err = appendFrame(last, final.PageCount, images[last]); err != nil {
			return fail(err)
		}
	}
	// A normal rolled-back cache spill may leave a larger same-salt suffix.
	// Remove only bytes beyond this newly complete, padded transaction. Every
	// pinned reader is bounded by the pre-append MaxFrame (or reads main-only
	// after a reset), so none can reference the discarded extent. Trimming
	// prevents the overwritten suffix from looking checksum-invalid on cold
	// reopen. The authoritative WAL inode is never replaced.
	if st, e := statNativeFile(s.walFile); e != nil {
		return fail(e)
	} else if st.Size() > offset {
		if err = s.walFile.Truncate(offset); err != nil {
			return fail(err)
		}
	}
	if err = s.walFile.Sync(); err != nil {
		return fail(err)
	}
	h.Change++
	h.PageSize = final.PageSize
	h.MaxFrame = frames
	h.DBSize = final.PageCount
	h.FrameChecksum = checksum
	h.BigEndianChecksum = wh.Magic == wal.MagicBigEndian
	// Append publishes page/hash entries before either checked header. The
	// durable WAL commit precedes publication to concurrent readers.
	if err = s.index.Publish(h); err != nil {
		return fail(sharedIndexError(err))
	}
	return nil
}
