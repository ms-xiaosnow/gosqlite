package pager

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"os"

	"github.com/ms-xiaosnow/gosqlite/internal/wal"

	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

func (s *Session) openWALLocked(ctx context.Context) error {
	st, absent, err := s.probePathLocked(s.path + "-wal")
	if absent {
		return nil
	}
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() {
		return fmt.Errorf("%w: WAL must be a regular file", ErrUnsupported)
	}
	if s.cleanup != nil {
		if err = s.cleanup.reserveWAL(); err != nil {
			return err
		}
	}
	f, err := openWALFile(s.path+"-wal", os.O_RDWR, true)
	if s.cleanup != nil {
		s.cleanup.acquiredWAL(f)
	}
	if err != nil {
		return err
	}
	s.walFile = f
	s.walInfo, err = statNativeFile(f)
	if err != nil {
		return err
	}
	if !vfs.SameFile(st, s.walInfo) {
		return ErrChanged
	}
	if err = validateWritableFile(s.walFile); err != nil {
		return err
	}
	if s.walInfo.Size() == 0 {
		return nil
	}
	prefix, err := wal.ScanPrefix(ctx, nativeReaderAt{f}, s.walInfo.Size(), wal.Limits{})
	if err != nil {
		// Typed scanner evidence is necessary but not sufficient: the existing
		// path-based intact-WAL normalizer is not a retained-directory tail
		// publisher. Do not silently treat pre/post pathname checks as that
		// capability, or truncate/append to the old authoritative WAL.
		if exclusiveWALPrefixAllowed(prefix, err) {
			return errors.Join(ErrUnsupported, errWALTailPublicationUnavailable, err)
		}
		if errors.Is(err, wal.ErrLimit) {
			return fmt.Errorf("%w: %w", ErrLimit, err)
		}
		return fmt.Errorf("%w: WAL validation: %w", ErrUnsupported, err)
	}
	s.snapshot = prefix.Snapshot()
	return nil
}

func (s *Session) checkWALStableLocked() error {
	pathInfo, absent, err := s.probePathLocked(s.path + "-wal")
	if s.walFile == nil {
		if absent {
			return nil
		}
		if err != nil {
			return err
		}
		return fmt.Errorf("%w: unexpected WAL", ErrChanged)
	}
	if err != nil {
		return errors.Join(ErrChanged, err)
	}
	if !pathInfo.Mode().IsRegular() {
		return fmt.Errorf("%w: WAL pathname", ErrChanged)
	}
	if err := validateNativePath(s.walFile); err != nil {
		return err
	}
	st, err := statNativeFile(s.walFile)
	if err != nil {
		return err
	}
	if !vfs.SameFile(st, pathInfo) || st.Size() != s.walInfo.Size() || !st.ModTime().Equal(s.walInfo.ModTime()) {
		return fmt.Errorf("%w: WAL file", ErrChanged)
	}
	return validateWritableFile(s.walFile)
}

// ensureWALLocked writes and separately syncs a fresh WAL header before any
// frames can be appended. In particular, a frame Sync cannot replace this
// header Sync (SQLite wal.c walFrames, header-reordering protection).
func (s *Session) ensureWALLocked() (retErr error) {
	defer func() { retErr = s.observeLocked(retErr) }()
	if s.walFile != nil && s.walInfo.Size() != 0 {
		return nil
	}
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return err
	}
	h := wal.Header{Magic: wal.MagicLittleEndian, Version: wal.Version,
		PageSize: s.header.PageSize,
		Salt:     [2]uint32{binary.BigEndian.Uint32(random[:4]), binary.BigEndian.Uint32(random[4:])}}
	b, err := wal.EncodeHeader(h)
	if err != nil {
		return err
	}
	h, err = wal.ParseHeader(b)
	if err != nil {
		return err
	}
	if s.walFile == nil {
		if s.cleanup != nil {
			if err = s.cleanup.reserveWAL(); err != nil {
				return err
			}
		}
		s.walFile, err = openWALFile(s.path+"-wal", os.O_RDWR|os.O_CREATE|os.O_EXCL, true)
		if s.cleanup != nil {
			s.cleanup.acquiredWAL(s.walFile)
		}
		if err != nil {
			return err
		}
	}
	if err = writeFullAt(s.walFile, b, 0); err != nil {
		return s.poisonLocked(err)
	}
	if err = s.walFile.Sync(); err != nil {
		return s.poisonLocked(err)
	}
	if err = syncDirectory(s.path); err != nil {
		return s.poisonLocked(err)
	}
	s.walInfo, err = statNativeFile(s.walFile)
	if err != nil {
		return s.poisonLocked(err)
	}
	s.snapshot = wal.Snapshot{Header: h, Pages: make(map[uint32]wal.PageRef),
		Checksum: h.Checksum, EndOffset: wal.HeaderSize, ValidEndOffset: wal.HeaderSize,
		Complete: true}
	s.appendSafe = true
	return nil
}

// Imported WALs may have been written under a smaller sector assumption or a
// powersafe-overwrite VFS. Appending in their final sector could damage their
// only acknowledged commit. Normalize on a NEW inode, never by extending the
// authoritative WAL: copy, pad with full commit frames, Sync, replace only the
// WAL pathname under EXCLUSIVE, then Sync its directory. Either namespace
// outcome represents the identical committed database; the main inode and
// its lock are untouched. This is not a shared-WAL reset protocol.
func (s *Session) normalizeAppendWALLocked() (retErr error) {
	defer func() { retErr = s.observeLocked(retErr) }()
	if s.appendSafe {
		return nil
	}
	if s.snapshot.FrameCount == 0 {
		// An imported header-only WAL has no old commit to preserve, but the
		// header still needs its own ordering Sync before the first frames.
		if err := s.walFile.Sync(); err != nil {
			return s.poisonLocked(err)
		}
		s.appendSafe = true
		return nil
	}
	frameSize := int64(wal.FrameHeaderSize) + int64(s.header.PageSize)
	offset := s.snapshot.EndOffset
	boundary := (offset + int64(s.opts.SectorSize) - 1) / int64(s.opts.SectorSize) * int64(s.opts.SectorSize)
	if offset == boundary {
		s.appendSafe = true
		return nil
	}
	extra := uint64((boundary - offset + frameSize - 1) / frameSize)
	if uint64(s.snapshot.FrameCount)+extra > 1<<20 || offset+int64(extra)*frameSize > 1<<30 ||
		uint64(len(s.snapshot.Pages))+1 > 1<<20 || s.snapshot.IndexWork > 16<<20 || 2*extra > 16<<20-s.snapshot.IndexWork {
		return fmt.Errorf("%w: imported WAL sector normalization", ErrLimit)
	}
	last := make([]byte, frameSize)
	if _, err := readNativeAt(s.walFile, last, offset-frameSize); err != nil {
		return err
	}
	pageNo, dbSize := binary.BigEndian.Uint32(last), binary.BigEndian.Uint32(last[4:])
	if dbSize == 0 || dbSize != s.snapshot.DBSize {
		return fmt.Errorf("%w: WAL final commit marker", ErrCorrupt)
	}
	if s.cleanup != nil {
		if err := s.cleanup.reserveTemporary(); err != nil {
			return err
		}
	}
	candidate, err := vfs.CreateWALCandidate(s.path)
	if s.cleanup != nil {
		s.cleanup.acquiredTemporary(candidate)
	}
	if err != nil {
		return nativeFileError(err)
	}
	tempPath := candidate.Name()
	renamed := false
	fail := func(cause error) error {
		if !renamed {
			if s.cleanup != nil {
				cause = s.cleanup.closeTemporary(cause)
				if errors.Is(cause, ErrPlaintextCleanupPending) {
					return s.poisonLocked(cause)
				}
			} else {
				cause = errors.Join(cause, closeNativeFile(candidate))
			}
			if e := os.Remove(tempPath); e != nil {
				cause = errors.Join(cause, e)
			} else {
				cause = errors.Join(cause, syncDirectory(s.path))
			}
		}
		// The previous acknowledged logical state remains in either complete
		// inode. Poison anyway: never retry an uncertain namespace operation.
		return s.poisonLocked(cause)
	}
	buf := make([]byte, 64<<10)
	for copied := int64(0); copied < offset; {
		n := int64(len(buf))
		if n > offset-copied {
			n = offset - copied
		}
		if _, err = readNativeAt(s.walFile, buf[:n], copied); err != nil {
			return fail(err)
		}
		if err = writeFullAt(candidate, buf[:n], copied); err != nil {
			return fail(err)
		}
		copied += n
	}
	checksum, frames := s.snapshot.Checksum, s.snapshot.FrameCount
	var ref wal.PageRef
	for offset < boundary {
		b, next, e := wal.EncodeFrame(s.snapshot.Header, pageNo, dbSize, last[wal.FrameHeaderSize:], checksum)
		if e != nil {
			return fail(e)
		}
		if e = writeFullAt(candidate, b, offset); e != nil {
			return fail(e)
		}
		frames++
		ref = wal.PageRef{Frame: frames, Offset: offset + wal.FrameHeaderSize}
		offset += int64(len(b))
		checksum = next
	}
	if err = candidate.Sync(); err != nil {
		return fail(err)
	}
	info, err := statNativeFile(candidate)
	if err != nil {
		return fail(err)
	}
	if err = s.checkWALStableLocked(); err != nil {
		return fail(err)
	}
	if err = vfs.ReplaceWAL(s.file, s.walFile, candidate); err != nil {
		return fail(err)
	}
	renamed = true
	old := s.walFile
	s.walFile, s.walInfo = candidate, info
	if s.cleanup != nil {
		s.cleanup.replaceWAL()
	}
	if err = syncDirectory(s.path); err != nil {
		cause := errors.Join(ErrCommitUncertain, err)
		if s.cleanup != nil {
			cause = s.cleanup.closeTemporary(cause)
		} else {
			cause = errors.Join(cause, closeNativeFile(old))
		}
		return fail(cause)
	}
	s.snapshot.IndexWork += 2 * uint64(frames-s.snapshot.FrameCount)
	s.snapshot.FrameCount, s.snapshot.ValidFrameCount = frames, frames
	s.snapshot.EndOffset, s.snapshot.ValidEndOffset = offset, offset
	s.snapshot.Checksum = checksum
	if pageNo <= dbSize {
		s.snapshot.Pages[pageNo] = ref
	}
	s.appendSafe = true
	if s.cleanup != nil {
		err = s.cleanup.closeTemporary(nil)
	} else {
		err = closeNativeFile(old)
	}
	if err != nil {
		return s.poisonLocked(err)
	}
	return nil
}

// commitWALLocked is called with t.mu held. The database descriptor never
// changes or releases its lock. Cancellation stops before the first physical
// write; once writing starts the safety-critical sequence runs to completion.
func (t *Tx) commitWALLocked(ctx context.Context) error {
	if err := t.checkStable(); err != nil {
		return err
	}
	var p1 []byte
	var err error
	finalHeader := t.header
	commitPages := t.dirty
	if len(t.dirty) != 0 {
		commitPages, err = walSectorImages(ctx, t.dirty, t.header, t.opts, t.readLocked)
		if err != nil {
			return err
		}
		p1, err = t.readLocked(ctx, 1)
		if err != nil {
			return err
		}
		finalHeader.ChangeCounter++
		finalHeader.VersionValidFor = finalHeader.ChangeCounter
		binary.BigEndian.PutUint32(p1[24:], finalHeader.ChangeCounter)
		binary.BigEndian.PutUint32(p1[28:], finalHeader.PageCount)
		binary.BigEndian.PutUint32(p1[92:], finalHeader.VersionValidFor)
		commitPages[1] = p1
	}
	s := t.session
	s.mu.Lock()
	defer s.mu.Unlock()
	if err = s.checkStableLocked(); err != nil {
		return err
	}
	if s.active != t {
		return ErrMisuse
	}
	finish := func() {
		s.active = nil
		t.file, t.dirty, t.free = nil, nil, nil
	}
	if len(t.dirty) == 0 {
		finish()
		return nil
	}
	if t.readOnly {
		return fmt.Errorf("%w: read-only WAL transaction", ErrMisuse)
	}
	err = s.appendWALPagesLocked(ctx, commitPages, finalHeader)
	if err == nil {
		t.header = finalHeader
		finish()
	} else if s.poisoned != nil {
		t.poisoned = s.poisoned
		finish()
	}
	return err
}

// walSectorImages expands writes to cover every extant page sharing a storage
// sector. Those unchanged images must remain recoverable when a checkpoint
// writes a neighboring page. The bounded expansion happens before WAL I/O.
func walSectorImages(ctx context.Context, dirty map[uint32][]byte, h Header, opts Options, read func(context.Context, uint32) ([]byte, error)) (map[uint32][]byte, error) {
	set := make(map[uint32][]byte, len(dirty))
	for n, p := range dirty {
		set[n] = p
	}
	for n := range dirty {
		first, last := uint64(n), uint64(n)
		if opts.SectorSize > h.PageSize {
			per := uint64(opts.SectorSize / h.PageSize)
			first = (uint64(n)-1)/per*per + 1
			last = first + per - 1
		}
		if last > uint64(h.PageCount) {
			last = uint64(h.PageCount)
		}
		for p := first; p <= last; p++ {
			if uint32(p) == lockPage(h.PageSize) {
				continue
			}
			if _, ok := set[uint32(p)]; !ok {
				set[uint32(p)] = nil
			}
			if uint64(len(set)) > uint64(opts.MaxDirtyPages) {
				return nil, fmt.Errorf("%w: WAL sector-neighbor images", ErrLimit)
			}
		}
	}
	for n, p := range set {
		if p == nil {
			var err error
			set[n], err = read(ctx, n)
			if err != nil {
				return nil, err
			}
		}
	}
	return set, nil
}

// appendWALPagesLocked appends one complete, synced transaction. Callers must
// supply sector-complete images, including page one, and hold s.mu.
func (s *Session) appendWALPagesLocked(ctx context.Context, commitPages map[uint32][]byte, finalHeader Header) (retErr error) {
	defer func() { retErr = s.observeLocked(retErr) }()
	var err error
	pages := sortedPages(commitPages)
	if len(pages) == 0 {
		return ErrMisuse
	}
	if pages[0] != 1 {
		return fmt.Errorf("%w: dirty transaction lacks page one", ErrMisuse)
	}
	if finalHeader.PageCount < s.snapshot.DBSize {
		return fmt.Errorf("%w: native WAL shrinking transaction", ErrUnsupported)
	}
	// Bound the same resources that the codec will accept on reopen. The
	// upper bound includes enough complete last-frame copies to cover a
	// sector. Callers may checkpoint to reset these WAL resource limits.
	frameSize := int64(wal.FrameHeaderSize) + int64(finalHeader.PageSize)
	extra := (int64(s.opts.SectorSize) + frameSize - 1) / frameSize
	end := s.snapshot.EndOffset
	if end == 0 {
		end = wal.HeaderSize
	}
	if !s.appendSafe && s.snapshot.FrameCount != 0 {
		boundary := (end + int64(s.opts.SectorSize) - 1) / int64(s.opts.SectorSize) * int64(s.opts.SectorSize)
		extra += (boundary - end + frameSize - 1) / frameSize
	}
	if uint64(s.snapshot.FrameCount)+uint64(len(pages))+uint64(extra) > 1<<20 || end+(int64(len(pages))+extra)*frameSize > 1<<30 {
		return fmt.Errorf("%w: WAL growth; checkpoint required", ErrLimit)
	}
	// Every unique data frame and every duplicate commit frame consumes one
	// index insertion plus one promotion visit in Scan. Imported WAL history
	// can have additional shrink-sweep work, which IndexWork already includes.
	workBound := 2 * (uint64(len(pages)) + uint64(extra))
	if s.snapshot.IndexWork > 16<<20 || workBound > 16<<20-s.snapshot.IndexWork {
		return fmt.Errorf("%w: WAL index work; checkpoint required", ErrLimit)
	}
	nextPages := make(map[uint32]wal.PageRef, len(s.snapshot.Pages)+len(pages))
	for n, ref := range s.snapshot.Pages {
		if n <= finalHeader.PageCount {
			nextPages[n] = ref
		}
	}
	for _, n := range pages {
		nextPages[n] = wal.PageRef{}
	}
	// The scanner retains its committed map and pending transaction map at
	// the same time. Count overlaps twice, just as Scan does. Padding adds a
	// one-entry pending map after the first commit marker is promoted.
	if len(s.snapshot.Pages)+len(pages) > 1<<20 || len(nextPages)+1 > 1<<20 {
		return fmt.Errorf("%w: WAL page index; checkpoint required", ErrLimit)
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = s.ensureWALLocked(); err != nil {
		return err
	}
	if err = s.normalizeAppendWALLocked(); err != nil {
		return err
	}
	// Normalization may have advanced the latest reference for its duplicate
	// commit page. Preserve those offsets for pages this transaction leaves
	// unchanged, even though the older copied images hold identical bytes.
	for n, ref := range s.snapshot.Pages {
		if _, changed := commitPages[n]; !changed && n <= finalHeader.PageCount {
			nextPages[n] = ref
		}
	}
	markerAttempted := false
	fail := func(cause error) error {
		if markerAttempted {
			cause = errors.Join(ErrCommitUncertain, cause)
		}
		return s.poisonLocked(cause)
	}
	offset, checksum, frameCount := s.snapshot.EndOffset, s.snapshot.Checksum, s.snapshot.FrameCount
	appendFrame := func(n, dbSize uint32, page []byte) error {
		b, next, e := wal.EncodeFrame(s.snapshot.Header, n, dbSize, page, checksum)
		if e != nil {
			return e
		}
		if dbSize != 0 {
			markerAttempted = true
		}
		if e = writeFullAt(s.walFile, b, offset); e != nil {
			return e
		}
		frameCount++
		nextPages[n] = wal.PageRef{Frame: frameCount, Offset: offset + wal.FrameHeaderSize}
		offset += int64(len(b))
		checksum = next
		return nil
	}
	for i, n := range pages {
		p := commitPages[n]
		dbSize := uint32(0)
		if i == len(pages)-1 {
			dbSize = finalHeader.PageCount
		}
		if err = appendFrame(n, dbSize, p); err != nil {
			return fail(err)
		}
	}
	// Repeat the complete final commit frame until its trailing storage
	// sector boundary is crossed. Never append zero bytes or partial frames:
	// each copy is a checksum-valid SQLite commit frame (wal.c walFrames).
	boundary := (offset + int64(s.opts.SectorSize) - 1) / int64(s.opts.SectorSize) * int64(s.opts.SectorSize)
	last := pages[len(pages)-1]
	lastPage := commitPages[last]
	for offset < boundary {
		if err = appendFrame(last, finalHeader.PageCount, lastPage); err != nil {
			return fail(err)
		}
	}
	if err = s.walFile.Sync(); err != nil {
		return fail(err)
	}
	info, err := statNativeFile(s.walFile)
	if err != nil {
		return fail(err)
	}
	// Publish the new snapshot only after its commit frames are durable.
	s.walInfo = info
	s.snapshot.Pages = nextPages
	s.snapshot.DBSize = finalHeader.PageCount
	s.snapshot.IndexWork += 2 * uint64(frameCount-s.snapshot.FrameCount)
	s.snapshot.FrameCount, s.snapshot.ValidFrameCount = frameCount, frameCount
	s.snapshot.EndOffset, s.snapshot.ValidEndOffset = offset, offset
	s.snapshot.Checksum = checksum
	s.snapshot.Complete = true
	s.header = finalHeader
	return nil
}
