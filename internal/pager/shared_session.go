package pager

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
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
// concurrent transactions. Shared WAL currently requires writable sidecars.
type SharedSession struct {
	mu       sync.Mutex
	file     *vfs.File
	walFile  *vfs.File
	index    *walindex.Index
	path     string
	opts     Options
	header   Header
	active   *Tx
	poisoned error
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
	case errors.Is(err, walindex.ErrPoisoned):
		return fmt.Errorf("%w: %w", ErrPoisoned, err)
	case errors.Is(err, walindex.ErrBusy):
		return fmt.Errorf("%w: shared WAL lock", ErrBusy)
	case errors.Is(err, walindex.ErrLimit):
		return fmt.Errorf("%w: %v", ErrLimit, err)
	case errors.Is(err, walindex.ErrUnsupported):
		return fmt.Errorf("%w: %v", ErrUnsupported, err)
	case errors.Is(err, walindex.ErrIdentity):
		return fmt.Errorf("%w: %v", ErrChanged, err)
	case errors.Is(err, walindex.ErrHeader):
		return fmt.Errorf("%w: %v", ErrCorrupt, err)
	default:
		return err
	}
}

// OpenSharedSession enters or opens standard shared WAL. Mode changes require
// the database EXCLUSIVE lock, but an already-WAL database does not. The WAL
// and SHM inodes are never replaced or unlinked by a SharedSession.
func OpenSharedSession(ctx context.Context, path string, opts Options) (result *SharedSession, retErr error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	opts, err := normalizedOptions(opts)
	if err != nil {
		return nil, err
	}
	path, err = filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	resolved, err := resolveNativePath(path)
	if err == nil {
		path = resolved
	} else if opts.Create && os.IsNotExist(err) {
		dir, e := resolveNativePath(filepath.Dir(path))
		if e != nil {
			return nil, e
		}
		path = filepath.Join(dir, filepath.Base(path))
	} else {
		return nil, err
	}
	flags := os.O_RDWR
	if opts.Create {
		flags |= os.O_CREATE
	}
	f, err := openWritableDatabase(path, flags)
	if err != nil {
		return nil, err
	}
	s := &SharedSession{file: f, path: path, opts: opts}
	success := false
	defer func() {
		if !success {
			if s.index != nil {
				if err := s.index.Close(); err != nil {
					// Keep the database SHARED lock too: an exclusive opener
					// must not remove SHM beneath an unresolved native view.
					retErr = errors.Join(retErr, sharedIndexError(err))
					return
				}
			}
			if s.walFile != nil {
				retErr = errors.Join(retErr, closeNativeFile(s.walFile))
			}
			retErr = errors.Join(retErr, closeNativeFile(f))
		}
	}()
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
		if err = lockExclusive(f); err != nil {
			return nil, err
		}
		if err = main.refreshMainLocked(); err != nil {
			return nil, err
		}
		if err = main.checkJournalLocked(); err != nil {
			return nil, err
		}
		if st, e := nativeLstat(path + "-wal"); e == nil && st.Size() != 0 {
			return nil, fmt.Errorf("%w: nonempty WAL with rollback database", ErrUnsupported)
		} else if e != nil && !os.IsNotExist(e) {
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
	s.walFile, err = openWALFile(path+"-wal", os.O_RDWR|os.O_CREATE, false)
	if err != nil {
		return nil, err
	}
	s.index, err = walindex.Open(path + "-shm")
	if err != nil {
		return nil, sharedIndexError(err)
	}
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
	success = true
	return s, nil
}

func (s *SharedSession) checkOpenLocked() error {
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
	if _, err := nativeLstat(s.path + "-journal"); err == nil {
		return fmt.Errorf("%w: journal beside shared WAL", ErrRecoveryRequired)
	} else if !os.IsNotExist(err) {
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
func (s *SharedSession) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.file == nil {
		return s.poisoned
	}
	if s.active != nil {
		return ErrBusy
	}
	err := s.poisoned
	if err == nil && !s.opts.KeepWALOnClose {
		err = s.checkpointLocked(context.Background())
		if errors.Is(err, ErrBusy) && s.poisoned == nil {
			err = nil
		}
	}
	if closeErr := s.index.Close(); closeErr != nil {
		// Retain main SHARED and WAL while the index still owns native views.
		// A later Close can retry cleanup; no transaction may continue.
		return s.poisonLocked(errors.Join(err, sharedIndexError(closeErr)))
	}
	err = errors.Join(err, closeNativeFile(s.walFile), closeNativeFile(s.file))
	s.file = nil
	s.walFile = nil
	s.index = nil
	return err
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
			if m <= h.MaxFrame && (chosen == 0 || m >= mark) {
				chosen, mark = i, m
			}
		}
		if chosen == 0 || mark < h.MaxFrame {
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
		if e1 == nil && e2 == nil && e3 == nil && m == mark && current == h && backfill <= h.MaxFrame {
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
