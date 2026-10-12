package pager

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/ms-xiaosnow/gosqlite/internal/wal"
)

// Checkpoint copies the committed WAL snapshot into the main database and
// removes the WAL only after the main file is synced. It requires no local
// lease and keeps the connection's EXCLUSIVE lock throughout and afterward.
func (s *Session) Checkpoint(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.checkpointLocked(ctx)
}

func (s *Session) checkpointLocked(ctx context.Context) (retErr error) {
	defer func() { retErr = s.observeLocked(retErr) }()
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.checkStableLocked(); err != nil {
		return err
	}
	if s.active != nil {
		return ErrBusy
	}
	if s.walFile == nil {
		return nil
	}
	if err := s.protectCheckpointSectorsLocked(ctx); err != nil {
		return err
	}
	// No cancellation after safety-critical physical work starts. Any failure
	// poisons the session; its durable WAL remains available for later reopen.
	fail := func(err error) error { return s.poisonLocked(err) }
	if err := s.walFile.Sync(); err != nil {
		return fail(err)
	}
	if s.snapshot.DBSize != 0 {
		pages := make([]uint32, 0, len(s.snapshot.Pages))
		for n := range s.snapshot.Pages {
			if n <= s.snapshot.DBSize {
				pages = append(pages, n)
			}
		}
		sort.Slice(pages, func(i, j int) bool { return pages[i] < pages[j] })
		p := make([]byte, s.header.PageSize)
		for _, n := range pages {
			if n == lockPage(s.header.PageSize) {
				return fail(fmt.Errorf("%w: WAL lock-byte page", ErrCorrupt))
			}
			if _, err := s.walFile.ReadAt(p, s.snapshot.Pages[n].Offset); err != nil {
				return fail(err)
			}
			if err := writeFullAt(s.file, p, int64(n-1)*int64(s.header.PageSize)); err != nil {
				return fail(err)
			}
			checkpointObserve(ctx, checkpointExclusiveCopy, s.path, n, uint32(len(pages)), s.snapshot.DBSize, s.snapshot.FrameCount, s.snapshot.Header.Salt, [2]uint32(s.snapshot.Checksum), 0, 0)
		}
		if err := s.file.Truncate(int64(s.snapshot.DBSize) * int64(s.header.PageSize)); err != nil {
			return fail(err)
		}
	}
	if err := s.file.Sync(); err != nil {
		return fail(err)
	}
	if s.cleanup != nil {
		if err := s.cleanup.closeWAL(nil); err != nil {
			return fail(err)
		}
	} else if err := s.walFile.Close(); err != nil {
		return fail(err)
	}
	s.walFile = nil
	if err := os.Remove(s.path + "-wal"); err != nil {
		return fail(err)
	}
	checkpointObserve(ctx, checkpointExclusiveRetired, s.path, 0, uint32(len(s.snapshot.Pages)), s.snapshot.DBSize, s.snapshot.FrameCount, s.snapshot.Header.Salt, [2]uint32(s.snapshot.Checksum), 0, 0)
	// No SQLite connection can hold SHARED while we hold EXCLUSIVE. A stale
	// shared-memory index has no role in this exclusive protocol. Remove it
	// only after checkpoint, so a later shared-WAL opener rebuilds its index.
	if err := os.Remove(s.path + "-shm"); err != nil && !os.IsNotExist(err) {
		return fail(err)
	}
	if err := syncDirectory(s.path); err != nil {
		return fail(err)
	}
	s.walInfo = nil
	s.snapshot = wal.Snapshot{}
	if err := s.refreshMainLocked(); err != nil {
		return fail(err)
	}
	return nil
}

// Official SQLite may have written an imported WAL with a powersafe-overwrite
// VFS (and a smaller sector assumption). Before touching the main file, add
// unchanged neighbor images in ordinary synced WAL transactions. This also
// protects previously unlogged main-file pages under this Session's sector
// assumption. Each batch respects the ordinary transaction memory bound.
func (s *Session) protectCheckpointSectorsLocked(ctx context.Context) error {
	if s.snapshot.DBSize == 0 || s.opts.SectorSize <= s.header.PageSize {
		return nil
	}
	per := uint64(s.opts.SectorSize / s.header.PageSize)
	sectors := make(map[uint32]struct{})
	for n := range s.snapshot.Pages {
		if n <= s.header.PageCount {
			sectors[uint32((uint64(n)-1)/per*per+1)] = struct{}{}
		}
	}
	batch := map[uint32][]byte{1: nil}
	flush := func() error {
		images, err := walSectorImages(ctx, batch, s.header, s.opts, s.readPageLocked)
		if err != nil {
			return err
		}
		if err = s.appendWALPagesLocked(ctx, images, s.header); err != nil {
			return err
		}
		batch = map[uint32][]byte{1: nil}
		return nil
	}
	hasProtection := false
	for first := range sectors {
		if err := ctx.Err(); err != nil {
			return err
		}
		last := uint64(first) + per - 1
		if last > uint64(s.header.PageCount) {
			last = uint64(s.header.PageCount)
		}
		missing := false
		for n := uint64(first); n <= last; n++ {
			if uint32(n) != lockPage(s.header.PageSize) {
				if _, ok := s.snapshot.Pages[uint32(n)]; !ok {
					missing = true
					break
				}
			}
		}
		if !missing {
			continue
		}
		// Reserve the first sector for the required page-one image too.
		needed := len(batch)
		for n := uint64(1); n <= per && n <= uint64(s.header.PageCount); n++ {
			if _, ok := batch[uint32(n)]; !ok {
				needed++
			}
		}
		for n := uint64(first); n <= last; n++ {
			if n > per && uint32(n) != lockPage(s.header.PageSize) {
				if _, ok := batch[uint32(n)]; !ok {
					needed++
				}
			}
		}
		if uint64(needed) > uint64(s.opts.MaxDirtyPages) && hasProtection {
			if err := flush(); err != nil {
				return err
			}
			hasProtection = false
		}
		for n := uint64(first); n <= last; n++ {
			if uint32(n) != lockPage(s.header.PageSize) {
				batch[uint32(n)] = nil
			}
		}
		hasProtection = true
	}
	if hasProtection {
		return flush()
	}
	return nil
}
