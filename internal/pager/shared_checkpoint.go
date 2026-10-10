package pager

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/ms-xiaosnow/gosqlite/internal/wal"
	"github.com/ms-xiaosnow/gosqlite/internal/walindex"
)

// Checkpoint performs a nonblocking FULL-style checkpoint. A single writer
// lock freezes the committed index while reader marks bound backfill. BUSY
// means some frames remain protected by readers (safe partial progress may
// have occurred). Main/SHM/WAL inodes are retained, even after full backfill.
func (s *SharedSession) Checkpoint(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.checkpointLocked(ctx)
}
func (s *SharedSession) checkpointLocked(ctx context.Context) (err error) {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.checkStableLocked(); err != nil {
		return err
	}
	if s.active != nil {
		return ErrBusy
	}
	if err := s.index.Lock(walindex.CheckpointLock, 1, true); err != nil {
		return sharedIndexError(err)
	}
	defer s.unlockDeferred(walindex.CheckpointLock, 1, &err)
	if err := s.index.Lock(walindex.WriteLock, 1, true); err != nil {
		return sharedIndexError(err)
	}
	defer s.unlockDeferred(walindex.WriteLock, 1, &err)
	h, err := s.index.ReadHeader()
	if err != nil {
		return sharedIndexError(err)
	}
	backfill, err := s.index.Backfill()
	if err != nil {
		return sharedIndexError(err)
	}
	if backfill > h.MaxFrame {
		return fmt.Errorf("%w: backfill exceeds WAL", ErrCorrupt)
	}
	if backfill == h.MaxFrame {
		return nil
	}
	safe := h.MaxFrame
	for i := 1; i < walindex.ReaderCount; i++ {
		mark, e := s.index.ReadMark(i)
		if e != nil {
			return sharedIndexError(e)
		}
		if mark >= safe {
			continue
		}
		e = s.index.Lock(walindex.ReadLock0+i, 1, true)
		if e == nil {
			next := walindex.UnusedReadMark
			if i == 1 {
				next = safe
			}
			e = s.index.SetReadMark(i, next)
			unlock := s.unlockLocked(walindex.ReadLock0+i, 1)
			if e != nil || unlock != nil {
				return sharedIndexError(errors.Join(e, unlock))
			}
		} else if errors.Is(e, walindex.ErrBusy) {
			safe = mark
		} else {
			return sharedIndexError(e)
		}
	}
	if safe <= backfill {
		return ErrBusy
	}
	// The iterator chooses the newest frame across the entire stable WAL,
	// then skips versions newer than safe. Choosing the newest <=safe instead
	// would incorrectly overwrite pages that a concurrent reader may use.
	latest := make(map[uint32]uint32)
	for frame := uint32(1); frame <= h.MaxFrame; frame++ {
		if err = ctx.Err(); err != nil {
			return err
		}
		page, e := s.index.Page(frame)
		if e != nil {
			return sharedIndexError(e)
		}
		if page == 0 || page == lockPage(h.PageSize) || page > wal.MaxPageNumber {
			return fmt.Errorf("%w: checkpoint index page", ErrCorrupt)
		}
		latest[page] = frame
	}
	images := make(map[uint32][]byte)
	for page, frame := range latest {
		if page <= h.DBSize && frame > backfill && frame <= safe {
			images[page] = nil
		}
	}
	// A smaller page than the assumed storage sector requires logged neighbor
	// images. No new frame may be silently appended to an imported unpadded WAL
	// merely to protect its checkpoint; the caller must explicitly opt into its
	// storage PSOW guarantee, or use a conservatively generated WAL.
	if !s.opts.WALPowersafeOverwrite && s.opts.SectorSize > h.PageSize {
		per := uint64(s.opts.SectorSize / h.PageSize)
		for page := range images {
			first := (uint64(page)-1)/per*per + 1
			last := first + per - 1
			if last > uint64(h.DBSize) {
				last = uint64(h.DBSize)
			}
			for n := first; n <= last; n++ {
				if uint32(n) != lockPage(h.PageSize) && latest[uint32(n)] == 0 {
					return fmt.Errorf("%w: imported WAL lacks checkpoint sector-neighbor image for page %d; explicit WALPowersafeOverwrite required", ErrUnsupported, n)
				}
			}
		}
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = s.index.Lock(walindex.ReadLock0, 1, true); err != nil {
		return sharedIndexError(err)
	}
	defer s.unlockDeferred(walindex.ReadLock0, 1, &err)
	// SQLite 3.54 walCheckpoint race fix: always recheck generation salts
	// AFTER acquiring READ_LOCK(0), even though this FULL-style path also owns
	// the writer lock. Never use cached frame offsets after a generation reset.
	live, err := s.index.ReadHeader()
	if err != nil {
		return sharedIndexError(err)
	}
	if live.Salt != h.Salt {
		return ErrBusy
	}
	backfill, err = s.index.Backfill()
	if err != nil {
		return sharedIndexError(err)
	}
	if backfill > safe {
		return fmt.Errorf("%w: checkpoint backfill changed", ErrCorrupt)
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	// No cancellation after physical work starts. A write/sync failure poisons
	// this connection and retains the authoritative WAL for later inspection.
	fail := func(e error) error { return s.poisonLocked(e) }
	if err = s.index.SetAttempted(safe); err != nil {
		return fail(sharedIndexError(err))
	}
	if err = s.walFile.Sync(); err != nil {
		return fail(err)
	}
	frameBytes := make([]byte, int(h.PageSize)+wal.FrameHeaderSize)
	page := frameBytes[wal.FrameHeaderSize:]
	for _, n := range sortedPages(images) {
		frame := latest[n]
		off := int64(wal.HeaderSize) + int64(frame-1)*(int64(wal.FrameHeaderSize)+int64(h.PageSize))
		if _, err = s.walFile.ReadAt(frameBytes, off); err != nil {
			return fail(err)
		}
		if binary.BigEndian.Uint32(frameBytes[:4]) != n || !bytes.Equal(frameBytes[8:16], h.Salt[:]) {
			return fail(fmt.Errorf("%w: checkpoint frame identity", ErrCorrupt))
		}
		if err = writeFullAt(s.file, page, int64(n-1)*int64(h.PageSize)); err != nil {
			return fail(err)
		}
	}
	if safe == h.MaxFrame {
		if err = s.file.Truncate(int64(h.DBSize) * int64(h.PageSize)); err != nil {
			return fail(err)
		}
	}
	if err = s.file.Sync(); err != nil {
		return fail(err)
	}
	if err = s.index.SetBackfill(safe); err != nil {
		return fail(sharedIndexError(err))
	}
	if safe < h.MaxFrame {
		return ErrBusy
	}
	return nil
}
