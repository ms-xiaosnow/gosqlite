package pager

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/ms-xiaosnow/gosqlite/internal/wal"
	"github.com/ms-xiaosnow/gosqlite/internal/walindex"

	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

// SharedSession implements SQLite's native single-writer/multiple-reader WAL
// protocol. Each connection retains a database SHARED lock and a SHM DMS lock;
// each transaction independently locks its immutable WAL read-mark snapshot.
// A connection permits one local lease. Use independent connections for
// concurrent transactions. An explicit readonly session joins a live Linux index
// without creating files, publishing readmarks, or checkpointing.
type SharedSession struct {
	mu         sync.Mutex
	file       *vfs.File
	walFile    *vfs.File
	index      *walindex.Index
	path       string
	opts       Options
	header     Header
	active     *Tx
	poisoned   error
	closed     bool
	readOnly   bool
	closeGroup *sharedCloseRetirement
}

type sharedRead struct {
	hdr      walindex.Header
	slot     int
	minFrame uint32
	write    bool
}

func sharedIndexError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, ErrPoisoned) {
		return err
	}
	switch {
	case errors.Is(err, walindex.ErrReadOnly):
		return fmt.Errorf("%w: %w", ErrReadOnly, err)
	case errors.Is(err, walindex.ErrPoisoned):
		return fmt.Errorf("%w: %w", ErrPoisoned, err)
	case errors.Is(err, walindex.ErrBusy):
		return fmt.Errorf("%w: shared WAL lock: %w", ErrBusy, err)
	case errors.Is(err, walindex.ErrLimit):
		return fmt.Errorf("%w: %w", ErrLimit, err)
	case errors.Is(err, walindex.ErrUnsupported):
		return fmt.Errorf("%w: %w", ErrUnsupported, err)
	case errors.Is(err, walindex.ErrIdentity):
		return fmt.Errorf("%w: %w", ErrChanged, err)
	case errors.Is(err, walindex.ErrHeader):
		return fmt.Errorf("%w: %w", ErrCorrupt, err)
	default:
		return err
	}
}

// OpenSharedSessionAdmitted consumes a close admission and enters standard shared WAL. Mode changes require
// the database EXCLUSIVE lock, but an already-WAL database does not. The WAL
// and SHM inodes are never replaced or unlinked by a SharedSession.
func OpenSharedSessionAdmitted(ctx context.Context, path string, opts Options, admission *SharedCloseAdmission) (*SharedSession, error) {
	return openSharedSessionAdmitted(ctx, path, opts, admission, false)
}

// OpenReadOnlySharedSessionAdmitted joins an existing live initialized Linux
// WAL generation. It consumes the same admission as writable shared sessions.
func OpenReadOnlySharedSessionAdmitted(ctx context.Context, path string, opts Options, admission *SharedCloseAdmission) (*SharedSession, error) {
	return openSharedSessionAdmitted(ctx, path, opts, admission, true)
}

func openSharedSessionAdmitted(ctx context.Context, path string, opts Options, admission *SharedCloseAdmission, readOnly bool) (result *SharedSession, retErr error) {
	closeGroup, err := admission.take()
	if err != nil {
		return nil, err
	}
	defer func() {
		if result == nil {
			closeGroup.abandon()
		}
	}()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := sharedReadOnlyOpenPolicy(runtime.GOOS, readOnly, opts); err != nil {
		return nil, err
	}
	opts, err = normalizedOptions(opts)
	if err != nil {
		return nil, err
	}
	// Reserve before any native metadata probes or resource acquisition. A full
	// registry fails closed without draining or stalling unrelated databases.
	cleanup, err := sharedOpenRetirements.reserve()
	if err != nil {
		return nil, err
	}
	defer func() {
		if result != nil {
			cleanup.transfer()
		} else {
			retErr = cleanup.failedOpen(retErr)
		}
	}()
	path, err = filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	resolved, initialAbsent, err := resolveNativeReaderPath(path)
	if err == nil {
		path = resolved
	} else if opts.Create && initialAbsent {
		dir, e := resolveNativePath(filepath.Dir(path))
		if e != nil {
			return nil, e
		}
		path = filepath.Join(dir, filepath.Base(path))
	} else {
		return nil, err
	}
	flags := sharedMainOpenFlags(readOnly, opts.Create)
	f, err := openWritableDatabase(path, flags)
	if err != nil {
		return nil, err
	}
	s := &SharedSession{file: f, path: path, opts: opts, closeGroup: closeGroup, readOnly: readOnly}
	cleanup.main = sharedOpenFileOwner{file: f}
	if err = lockShared(f); err != nil {
		return nil, err
	}
	// Reuse only the descriptor-owning rollback transition, never the exclusive
	// session's WAL scanner, inode normalization, or SHM removal path.
	main := &Session{file: f, path: path, opts: opts}
	if err = main.refreshMainLocked(); err != nil {
		return nil, err
	}
	if err = main.checkJournalLocked(); err != nil {
		return nil, err
	}
	if main.info.Size() == 0 || (main.rawHeader[18] == 1 && main.rawHeader[19] == 1) {
		if readOnly {
			return nil, fmt.Errorf("%w: readonly shared WAL requires an existing mode-2 database", ErrUnsupported)
		}
		if err = lockExclusive(f); err != nil {
			return nil, err
		}
		if err = main.refreshMainLocked(); err != nil {
			return nil, err
		}
		if err = main.checkJournalLocked(); err != nil {
			return nil, err
		}
		if st, absent, e := nativeLstatForCreate(path + "-wal"); e == nil && st.Size() != 0 {
			return nil, fmt.Errorf("%w: nonempty WAL with rollback database", ErrUnsupported)
		} else if e != nil && !absent {
			return nil, e
		}
		if err = main.enterWALLocked(ctx); err != nil {
			return nil, err
		}
		if err = unlockExclusive(f); err != nil {
			return nil, err
		}
	}
	if !bytes.Equal(main.rawHeader[:16], []byte("SQLite format 3\x00")) || main.rawHeader[18] != 2 || main.rawHeader[19] != 2 {
		return nil, fmt.Errorf("%w: shared WAL requires mode-2 database", ErrUnsupported)
	}
	size := uint32(binary.BigEndian.Uint16(main.rawHeader[16:18]))
	if size == 1 {
		size = 65536
	}
	if size < 512 || size > 65536 || size&(size-1) != 0 {
		return nil, fmt.Errorf("%w: main page size", ErrCorrupt)
	}
	s.header.PageSize = size
	s.walFile, err = openWALFile(path+"-wal", sharedWALOpenFlags(readOnly), false)
	if err != nil {
		return nil, err
	}
	cleanup.wal = sharedOpenFileOwner{file: s.walFile}
	if readOnly {
		s.index, err = walindex.OpenReadOnly(path + "-shm")
	} else {
		s.index, err = walindex.Open(path + "-shm")
	}
	if err != nil {
		return nil, sharedIndexError(err)
	}
	cleanup.index = sharedOpenIndexOwner{index: s.index}
	if err = s.checkStableLocked(); err != nil {
		return nil, err
	}
	// Establish a real read snapshot before accepting any logical header.
	read, err := s.beginReadLocked(ctx, false)
	if err != nil {
		return nil, err
	}
	h, _, err := s.readLogicalHeaderLocked(ctx, read)
	releaseErr := sharedIndexError(s.unlockLocked(walindex.ReadLock0+read.slot, 1))
	if err != nil || releaseErr != nil {
		return nil, errors.Join(err, releaseErr)
	}
	s.header = h
	closeGroup.index = sharedCloseIndex{s.index}
	closeGroup.wal = sharedCloseFile{s.walFile}
	closeGroup.main = sharedCloseFile{s.file}
	return s, nil
}

func (s *SharedSession) checkOpenLocked() error {
	if s.closed {
		return ErrClosed
	}
	if s.poisoned != nil {
		return s.poisoned
	}
	if s.file == nil {
		return ErrClosed
	}
	return nil
}
func (s *SharedSession) checkStable() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.checkStableLocked()
}
func (s *SharedSession) checkStableLocked() error {
	if err := s.checkOpenLocked(); err != nil {
		return err
	}
	if _, absent, err := nativeLstatForCreate(s.path + "-journal"); err == nil {
		return fmt.Errorf("%w: journal beside shared WAL", ErrRecoveryRequired)
	} else if !absent {
		return err
	}
	for _, entry := range []struct {
		f    *vfs.File
		path string
	}{{s.file, s.path}, {s.walFile, s.path + "-wal"}} {
		if err := validateNativePath(entry.f); err != nil {
			return err
		}
		st, err := statNativeFile(entry.f)
		if err != nil {
			return err
		}
		other, err := nativeLstat(entry.path)
		if err != nil {
			return errors.Join(ErrChanged, err)
		}
		if !st.Mode().IsRegular() || !other.Mode().IsRegular() || !vfs.SameFile(st, other) {
			return fmt.Errorf("%w: shared WAL pathname %s", ErrChanged, entry.path)
		}
		if err = validateWritableFile(entry.f); err != nil {
			return err
		}
	}
	return sharedIndexError(s.index.ValidateIdentity())
}
func (s *SharedSession) poisonLocked(err error) error {
	s.poisoned = errors.Join(ErrPoisoned, err)
	return s.poisoned
}
func (s *SharedSession) Header() Header                         { s.mu.Lock(); defer s.mu.Unlock(); return s.header }
func (s *SharedSession) Begin(ctx context.Context) (*Tx, error) { return s.begin(ctx, false) }
func (s *SharedSession) Reader(ctx context.Context) (*Reader, error) {
	t, err := s.begin(ctx, true)
	if err != nil {
		return nil, err
	}
	return &Reader{lease: t}, nil
}
func (s *SharedSession) begin(ctx context.Context, readOnly bool) (*Tx, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.mu.Lock()
	if s.readOnly && !readOnly {
		s.mu.Unlock()
		return nil, ErrReadOnly
	}
	if err := s.checkStableLocked(); err != nil {
		s.mu.Unlock()
		return nil, err
	}
	if s.active != nil {
		s.mu.Unlock()
		return nil, ErrBusy
	}
	snap, err := s.beginReadLocked(ctx, false)
	if err != nil {
		s.mu.Unlock()
		return nil, err
	}
	h, p1, err := s.readLogicalHeaderLocked(ctx, snap)
	if err != nil {
		err = errors.Join(err, sharedIndexError(s.unlockLocked(walindex.ReadLock0+snap.slot, 1)))
		s.mu.Unlock()
		return nil, err
	}
	t := &Tx{file: s.file, path: s.path, header: h, originalPages: h.PageCount, opts: s.opts, shared: s, sharedRead: snap, readOnly: readOnly, dirty: make(map[uint32][]byte), free: make(map[uint32]struct{})}
	copy(t.originalHeader[:], p1[:100])
	s.header = h
	s.active = t
	s.mu.Unlock()
	if !readOnly {
		if err = t.loadFreelist(ctx); err != nil {
			_ = t.Rollback()
			return nil, err
		}
	}
	return t, nil
}

func (s *SharedSession) readLogicalHeaderLocked(ctx context.Context, r sharedRead) (Header, []byte, error) {
	p, err := s.readPageLocked(ctx, r, s.header.PageSize, 1)
	if err != nil {
		return Header{}, nil, err
	}
	st, err := statNativeFile(s.file)
	if err != nil {
		return Header{}, nil, err
	}
	count := uint32(0)
	if r.hdr.MaxFrame != 0 {
		count = r.hdr.DBSize
	}
	h, err := parseHeader(p, st.Size(), true, count)
	if err != nil {
		return Header{}, nil, err
	}
	if h.PageSize != s.header.PageSize || (r.hdr.PageSize != 0 && r.hdr.PageSize != h.PageSize) || h.ReadVersion != 2 || h.WriteVersion != 2 {
		return Header{}, nil, fmt.Errorf("%w: WAL/main logical header mismatch", ErrCorrupt)
	}
	if err = writableHeader(h); err != nil {
		return Header{}, nil, err
	}
	return h, p, nil
}

func (s *SharedSession) readPage(ctx context.Context, r sharedRead, h Header, n uint32) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkStableLocked(); err != nil {
		return nil, err
	}
	return s.readPageLocked(ctx, r, h.PageSize, n)
}
func (s *SharedSession) readPageLocked(ctx context.Context, r sharedRead, pageSize, n uint32) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	frame := uint32(0)
	if r.slot != 0 && r.hdr.MaxFrame != 0 {
		var err error
		frame, err = s.index.Lookup(n, r.minFrame, r.hdr.MaxFrame)
		if err != nil {
			return nil, sharedIndexError(err)
		}
	}
	page := make([]byte, pageSize)
	if frame == 0 {
		if _, err := readNativeAt(s.file, page, int64(n-1)*int64(pageSize)); err != nil {
			return nil, err
		}
	} else {
		off := int64(wal.HeaderSize) + int64(frame-1)*(int64(wal.FrameHeaderSize)+int64(pageSize))
		var fh [wal.FrameHeaderSize]byte
		if _, err := readNativeAt(s.walFile, fh[:], off); err != nil {
			return nil, err
		}
		if binary.BigEndian.Uint32(fh[:4]) != n || !bytes.Equal(fh[8:16], r.hdr.Salt[:]) {
			return nil, fmt.Errorf("%w: indexed WAL frame identity", ErrCorrupt)
		}
		if _, err := readNativeAt(s.walFile, page, off+wal.FrameHeaderSize); err != nil {
			return nil, err
		}
	}
	return page, ctx.Err()
}

func (s *SharedSession) promote(ctx context.Context, t *Tx) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.readOnly {
		return ErrReadOnly
	}
	if err := s.checkStableLocked(); err != nil {
		return err
	}
	if s.active != t {
		return ErrMisuse
	}
	if err := sharedIndexError(s.index.Lock(walindex.WriteLock, 1, true)); err != nil {
		return err
	}
	live, err := s.index.ReadHeader()
	if err != nil || live != t.sharedRead.hdr {
		unlock := sharedIndexError(s.unlockLocked(walindex.WriteLock, 1))
		if err != nil {
			return errors.Join(sharedIndexError(err), unlock)
		}
		return errors.Join(ErrBusySnapshot, unlock)
	}
	t.sharedRead.write = true
	t.reserved = true
	return nil
}
func (s *SharedSession) release(t *Tx) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.releaseLocked(t)
}
func (s *SharedSession) releaseLocked(t *Tx) error {
	if s.active != t {
		return ErrMisuse
	}
	var err error
	if t.sharedRead.write {
		err = sharedIndexError(s.unlockLocked(walindex.WriteLock, 1))
		t.sharedRead.write = false
	}
	err = errors.Join(err, sharedIndexError(s.unlockLocked(walindex.ReadLock0+t.sharedRead.slot, 1)))
	s.active = nil
	if err != nil {
		return s.poisonLocked(err)
	}
	return nil
}

// Close preserves WAL/SHM names. Other connections may be using either inode.
// A reader-limited checkpoint is harmless: the synced WAL is retained and the
// connection still closes normally. No last-connection deletion is attempted.
func (s *SharedSession) Close() error { return s.CloseWithError(nil) }

// CloseWithError adopts first-close lease errors before retiring their protecting
// index, WAL and main owners. It is called by sqlite.DB after invalidating its
// Rows and transaction once. Retries never checkpoint or re-invalidate leases.
func (s *SharedSession) CloseWithError(cause error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closeLocked(cause, func() error { return s.checkpointLocked(context.Background()) })
}

// The callback isolates the one-shot checkpoint decision for descriptor-free
// policy laws. Production always passes its real checkpoint above; it is not a
// replaceable native-I/O or fault-injection hook.
func (s *SharedSession) closeLocked(cause error, checkpoint func() error) error {
	if s.closed {
		return s.closeGroup.retry()
	}
	if s.active != nil {
		return ErrBusy
	}
	err := errors.Join(cause, s.poisoned)
	if err == nil && sharedCloseMayCheckpoint(s.readOnly, s.opts.KeepWALOnClose) {
		err = checkpoint()
		if errors.Is(err, ErrBusy) && s.poisoned == nil {
			err = nil
		}
	}
	s.closed = true
	// Only managed owners and native causes remain rooted in the group, not the
	// session or any SQL objects. All subsequent operations fail logically closed.
	s.file, s.walFile, s.index = nil, nil, nil
	s.poisoned = nil
	return s.closeGroup.begin(err)
}

// beginReadLocked follows walTryBeginRead: header, slot, then header/mark
// recheck. A read-lock alone is insufficient, as a writer may reset between
// reading the header and obtaining the slot. Each retry releases its slot.
func (s *SharedSession) beginReadLocked(ctx context.Context, forceWAL bool) (sharedRead, error) {
	for attempt := 0; attempt < 100; attempt++ {
		if err := ctx.Err(); err != nil {
			return sharedRead{}, err
		}
		h, err := s.readIndexHeaderLocked(ctx)
		if err != nil {
			if s.readOnly && errors.Is(err, ErrBusy) && !errors.Is(err, ErrPoisoned) {
				runtime.Gosched()
				continue
			}
			return sharedRead{}, err
		}
		backfill, err := s.index.Backfill()
		if err != nil {
			return sharedRead{}, sharedIndexError(err)
		}
		if backfill > h.MaxFrame {
			runtime.Gosched()
			continue
		}
		if !forceWAL && backfill == h.MaxFrame {
			err = s.index.Lock(walindex.ReadLock0, 1, false)
			if err == nil {
				current, e := s.index.ReadHeader()
				if e == nil && current == h {
					return sharedRead{hdr: h, slot: 0, minFrame: backfill + 1}, nil
				}
				if e = s.unlockLocked(walindex.ReadLock0, 1); e != nil {
					return sharedRead{}, sharedIndexError(e)
				}
				runtime.Gosched()
				continue
			} else if !errors.Is(err, walindex.ErrBusy) {
				return sharedRead{}, sharedIndexError(err)
			}
		}
		chosen := 0
		mark := uint32(0)
		for i := 1; i < walindex.ReaderCount; i++ {
			m, e := s.index.ReadMark(i)
			if e != nil {
				return sharedRead{}, sharedIndexError(e)
			}
			if sharedReadMarkPreferred(h.MaxFrame, chosen, mark, m) {
				chosen, mark = i, m
			}
		}
		if !s.readOnly && (chosen == 0 || mark < h.MaxFrame) {
			for i := 1; i < walindex.ReaderCount; i++ {
				if e := s.index.Lock(walindex.ReadLock0+i, 1, true); e == nil {
					e = s.index.SetReadMark(i, h.MaxFrame)
					unlock := s.unlockLocked(walindex.ReadLock0+i, 1)
					if e != nil || unlock != nil {
						return sharedRead{}, sharedIndexError(errors.Join(e, unlock))
					}
					chosen, mark = i, h.MaxFrame
					break
				} else if !errors.Is(e, walindex.ErrBusy) {
					return sharedRead{}, sharedIndexError(e)
				}
			}
		}
		if chosen == 0 {
			if err := sharedReadOnlyMarkError(s.readOnly, chosen); err != nil {
				return sharedRead{}, err
			}
			runtime.Gosched()
			continue
		}
		if err = s.index.Lock(walindex.ReadLock0+chosen, 1, false); err != nil {
			if errors.Is(err, walindex.ErrBusy) {
				runtime.Gosched()
				continue
			}
			return sharedRead{}, sharedIndexError(err)
		}
		backfill, e1 := s.index.Backfill()
		m, e2 := s.index.ReadMark(chosen)
		current, e3 := s.index.ReadHeader()
		if e1 == nil && e2 == nil && e3 == nil && sharedReadSnapshotStable(h, current, mark, m, backfill) {
			return sharedRead{hdr: h, slot: chosen, minFrame: backfill + 1}, nil
		}
		if err = s.unlockLocked(walindex.ReadLock0+chosen, 1); err != nil {
			return sharedRead{}, sharedIndexError(err)
		}
		runtime.Gosched()
	}
	return sharedRead{}, fmt.Errorf("%w: WAL snapshot changed repeatedly", ErrBusy)
}

// Every failed release poisons the handle, including preflight/retry paths.
// An untracked retained writer/read lock must not be reported as retryable.
func (s *SharedSession) unlockLocked(slot, n int) error {
	if e := s.index.Unlock(slot, n); e != nil {
		return s.poisonLocked(sharedIndexError(e))
	}
	return nil
}
func (s *SharedSession) unlockDeferred(slot, n int, result *error) {
	if e := s.unlockLocked(slot, n); e != nil {
		*result = errors.Join(*result, e)
	}
}
