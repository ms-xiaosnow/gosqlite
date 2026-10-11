package pager

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/ms-xiaosnow/gosqlite/internal/wal"
	"github.com/ms-xiaosnow/gosqlite/internal/walindex"
)

// readIndexHeaderLocked normally performs only the checked dual-header read.
// Rebuilding the transient index is serialized by WRITE/CKPT/RECOVER and all
// read slots. It accepts a committed valid WAL, a fully valid discarded
// transaction suffix, or a normal old-salt suffix after reset. Incomplete or
// checksum-invalid current-generation tails remain a fail-closed boundary.
func (s *SharedSession) readIndexHeaderLocked(ctx context.Context) (h walindex.Header, err error) {
	h, err = s.index.ReadHeader()
	if err == nil {
		return h, nil
	}
	if s.readOnly {
		// A shared WRITE-slot probe distinguishes an in-flight publication from
		// a stable invalid index. It grants no publishing/rebuild authority.
		if !errors.Is(err, walindex.ErrUninitialized) && !errors.Is(err, walindex.ErrHeader) {
			return h, sharedIndexError(err)
		}
		if err = ctx.Err(); err != nil {
			return h, err
		}
		if err = s.index.Lock(walindex.WriteLock, 1, false); err != nil {
			return h, sharedIndexError(err)
		}
		h, err = s.index.ReadHeader()
		if errors.Is(err, walindex.ErrUninitialized) {
			err = fmt.Errorf("%w: readonly WAL requires an initialized live index", ErrUnsupported)
		}
		return h, errors.Join(sharedIndexError(err), sharedIndexError(s.unlockLocked(walindex.WriteLock, 1)))
	}
	if errors.Is(err, walindex.ErrUnsupported) || (!errors.Is(err, walindex.ErrUninitialized) && !errors.Is(err, walindex.ErrHeader)) {
		return h, sharedIndexError(err)
	}
	if err = s.index.Lock(walindex.WriteLock, 1, true); err != nil {
		return h, sharedIndexError(err)
	}
	defer s.unlockDeferred(walindex.WriteLock, 1, &err)
	h, err = s.index.ReadHeader()
	if err == nil {
		return h, nil
	}
	if errors.Is(err, walindex.ErrUnsupported) || (!errors.Is(err, walindex.ErrUninitialized) && !errors.Is(err, walindex.ErrHeader)) {
		return h, sharedIndexError(err)
	}
	if err = s.index.Lock(walindex.CheckpointLock, 7, true); err != nil {
		return h, sharedIndexError(err)
	}
	defer s.unlockDeferred(walindex.CheckpointLock, 7, &err)
	st, err := statNativeFile(s.walFile)
	if err != nil {
		return h, err
	}
	h = walindex.Header{Version: wal.Version}
	if st.Size() != 0 {
		prefix, e := wal.ScanPrefix(ctx, nativeReaderAt{s.walFile}, st.Size(), wal.Limits{})
		// Stale-generation bytes after an exact committed prefix are the normal
		// consequence of SQLite WAL reuse. Fully checksum-valid uncommitted
		// frames can likewise follow an ordinary cache-spill then ROLLBACK.
		// Only the last committed prefix enters the shared index.
		if !sharedWALPrefixAllowed(prefix, e) {
			return h, fmt.Errorf("%w: shared WAL reconstruction: %w", ErrUnsupported, e)
		}
		snap := prefix.Snapshot()
		if snap.Header.PageSize != s.header.PageSize {
			return h, fmt.Errorf("%w: physical database and WAL page sizes differ", ErrCorrupt)
		}
		h.PageSize = snap.Header.PageSize
		h.MaxFrame = snap.FrameCount
		h.DBSize = snap.DBSize
		h.FrameChecksum = snap.Checksum
		h.BigEndianChecksum = snap.Header.Magic == wal.MagicBigEndian
		binary.BigEndian.PutUint32(h.Salt[:4], snap.Header.Salt[0])
		binary.BigEndian.PutUint32(h.Salt[4:], snap.Header.Salt[1])
		var raw [wal.FrameHeaderSize]byte
		for n := uint32(1); n <= h.MaxFrame; n++ {
			if err = ctx.Err(); err != nil {
				return h, err
			}
			off := int64(wal.HeaderSize) + int64(n-1)*(int64(wal.FrameHeaderSize)+int64(h.PageSize))
			if _, err = readNativeAt(s.walFile, raw[:], off); err != nil {
				return h, err
			}
			if err = s.index.Append(n, binary.BigEndian.Uint32(raw[:4])); err != nil {
				return h, sharedIndexError(err)
			}
		}
	}
	if err = s.index.InitCheckpoint(h.MaxFrame); err != nil {
		return h, sharedIndexError(err)
	}
	if err = s.index.Publish(h); err != nil {
		return h, sharedIndexError(err)
	}
	h, err = s.index.ReadHeader()
	return h, sharedIndexError(err)
}

// The WAL scanner never exposes pending pages as committed. These two tails
// represent normal rollback/reuse; all incomplete or same-salt invalid tails
// remain refused rather than being silently repaired.
func sharedWALPrefixAllowed(prefix wal.CommittedPrefix, err error) bool {
	return prefix.Allows(err, wal.TailUncommitted, wal.TailSaltMismatch)
}
