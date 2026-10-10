package pager

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/ms-xiaosnow/gosqlite/internal/wal"

	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

// Session is the opt-in exclusive WAL pager. A single read-write database
// descriptor and its EXCLUSIVE SQLite lock are retained until Close, including
// between transactions. There is no shared-memory WAL index or shared-WAL
// concurrency. One local reader or transaction may hold a lease at a time.
type Session struct {
	mu         sync.Mutex
	file       *vfs.File
	path       string
	opts       Options
	info       os.FileInfo
	rawHeader  [100]byte // Physical main-file header, not the WAL overlay.
	header     Header    // Latest durable logical snapshot.
	walFile    *vfs.File
	walInfo    os.FileInfo
	snapshot   wal.Snapshot
	active     *Tx
	poisoned   error
	cleanup    *plainRetirement
	closed     bool
	appendSafe bool // Last acknowledged WAL commit survives our append sector.
}

// OpenSession opens a database in exclusive WAL mode. It requires write access
// even if its subsequent transactions are read-only. Entering WAL mode is a
// standard rollback-journal transaction using the same continuously locked
// database descriptor. A mode-2 database without a WAL is valid.
func OpenSession(ctx context.Context, path string, opts Options) (*Session, error) {
	return OpenSessionAdmitted(ctx, path, opts, nil)
}

// OpenSessionAdmitted consumes a pre-canonical public admission exactly once.
// Nil requests the direct constructor's own pre-acquisition reservation.
func OpenSessionAdmitted(ctx context.Context, path string, opts Options, admission *ExclusiveOpenAdmission) (result *Session, retErr error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	opts, err := normalizedOptions(opts)
	if err != nil {
		return nil, err
	}
	cleanup, err := claimExclusiveOpen(admission)
	if err != nil {
		return nil, err
	}
	defer func() {
		if result == nil {
			retErr = cleanup.beginClass(retErr, nil, !cleanup.childClosePending())
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
	flags := os.O_RDWR
	if opts.Create {
		flags |= os.O_CREATE
	}
	f, err := openWritableDatabase(path, flags)
	if err != nil {
		return nil, err
	}
	cleanup.main = plainFile{f}
	s := &Session{file: f, path: path, opts: opts, cleanup: cleanup}
	if err = lockShared(f); err != nil {
		return nil, err
	}
	if err = lockReserved(f); err != nil {
		return nil, err
	}
	if err = lockExclusive(f); err != nil {
		return nil, err
	}
	if err = s.refreshMainLocked(); err != nil {
		return nil, err
	}
	if err = s.checkJournalLocked(); err != nil {
		return nil, err
	}
	if err = s.openWALLocked(ctx); err != nil {
		return nil, err
	}
	if s.info.Size() == 0 || (s.rawHeader[18] == 1 && s.rawHeader[19] == 1) {
		if s.walInfo != nil && s.walInfo.Size() != 0 {
			return nil, fmt.Errorf("%w: nonempty WAL with rollback-mode main database", ErrUnsupported)
		}
		if err = s.enterWALLocked(ctx); err != nil {
			return nil, err
		}
	}
	if s.rawHeader[18] != 2 || s.rawHeader[19] != 2 {
		return nil, fmt.Errorf("%w: exclusive WAL requires mode-2 database", ErrUnsupported)
	}
	pageOne := make([]byte, 100)
	copy(pageOne, s.rawHeader[:])
	if ref, ok := s.snapshot.Pages[1]; ok {
		if _, err = readNativeAt(s.walFile, pageOne, ref.Offset); err != nil {
			return nil, err
		}
	}
	s.header, err = parseHeader(pageOne, s.info.Size(), true, s.snapshot.DBSize)
	if err != nil {
		return nil, err
	}
	if s.header.WriteVersion != 2 || s.header.ReadVersion != 2 {
		return nil, fmt.Errorf("%w: WAL page one must remain mode 2", ErrCorrupt)
	}
	mainPageSize := uint32(binary.BigEndian.Uint16(s.rawHeader[16:18]))
	if mainPageSize == 1 {
		mainPageSize = 65536
	}
	if !bytes.Equal(s.rawHeader[:16], []byte("SQLite format 3\x00")) || mainPageSize != s.header.PageSize {
		return nil, fmt.Errorf("%w: physical and logical database headers disagree", ErrCorrupt)
	}
	if s.walInfo != nil && s.walInfo.Size() > 0 && s.snapshot.Header.PageSize != s.header.PageSize {
		return nil, fmt.Errorf("%w: WAL and database page sizes differ", ErrCorrupt)
	}
	if err = writableHeader(s.header); err != nil {
		return nil, err
	}
	physical := uint32(s.info.Size() / int64(s.header.PageSize))
	for n := uint64(physical) + 1; n <= uint64(s.header.PageCount); n++ {
		if uint32(n) == lockPage(s.header.PageSize) {
			continue
		}
		if _, ok := s.snapshot.Pages[uint32(n)]; !ok {
			return nil, fmt.Errorf("%w: WAL has no image for appended page %d", ErrCorrupt, n)
		}
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	// No compatible connection can retain SHARED while we hold EXCLUSIVE.
	// Never consume a pre-existing shared-memory index, and invalidate it
	// before appending so a later shared-WAL opener cannot trust stale frames.
	if err = s.removeStaleSHMLocked(); err != nil {
		return nil, err
	}
	return s, nil
}

func writableHeader(h Header) error {
	if h.ReservedBytes != 0 {
		return fmt.Errorf("%w: writable reserved page bytes", ErrUnsupported)
	}
	if h.SchemaFormat != 4 && h.SchemaFormat != 0 {
		return fmt.Errorf("%w: writable schema format %d", ErrUnsupported, h.SchemaFormat)
	}
	return nil
}

func (s *Session) enterWALLocked(ctx context.Context) error {
	t := &Tx{file: s.file, path: s.path, info: s.info, opts: s.opts,
		dirty: make(map[uint32][]byte), free: make(map[uint32]struct{}),
		originalHeader: s.rawHeader, reserved: true, keepFile: true, borrowedCleanup: s.cleanup}
	var p []byte
	var err error
	if s.info.Size() == 0 {
		if !s.opts.Create {
			return fmt.Errorf("%w: empty database requires Create", ErrCorrupt)
		}
		p = emptyDatabasePage(s.opts.PageSize)
		t.header, err = ParseHeader(p, int64(len(p)))
	} else {
		t.header, err = ParseHeader(s.rawHeader[:], s.info.Size())
		if err == nil {
			err = writableHeader(t.header)
		}
		if err == nil && int64(t.header.PageCount)*int64(t.header.PageSize) != s.info.Size() {
			err = fmt.Errorf("%w: physical pages beyond logical extent", ErrUnsupported)
		}
		if err == nil {
			t.originalPages = t.header.PageCount
			err = t.loadFreelist(ctx)
		}
		if err == nil {
			p, err = t.readLocked(ctx, 1)
		}
	}
	if err != nil {
		return err
	}
	p[18], p[19] = 2, 2
	t.header.WriteVersion, t.header.ReadVersion = 2, 2
	t.dirty[1] = p
	if err = t.Commit(ctx); err != nil {
		return err
	}
	return s.refreshMainLocked()
}

func (s *Session) refreshMainLocked() error {
	info, err := statNativeFile(s.file)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("%w: database must be a regular file", ErrUnsupported)
	}
	if err = validateWritableFile(s.file); err != nil {
		return err
	}
	s.info = info
	s.rawHeader = [100]byte{}
	if info.Size() != 0 {
		if _, err = readNativeAt(s.file, s.rawHeader[:], 0); err != nil {
			return fmt.Errorf("%w: header: %w", ErrCorrupt, err)
		}
	}
	return nil
}

func (s *Session) checkJournalLocked() error {
	_, absent, err := s.probePathLocked(s.path + "-journal")
	if absent {
		return nil
	}
	if err != nil {
		return err
	}
	return fmt.Errorf("%w: existing journal; recovery is not yet acceptance-verified", ErrRecoveryRequired)
}

func (s *Session) checkOpenLocked() error {
	if err := s.cleanup.check(); err != nil {
		return err
	}
	if s.poisoned != nil {
		return s.poisoned
	}
	if s.file == nil {
		return ErrClosed
	}
	return nil
}

func (s *Session) checkStable() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.checkStableLocked()
}

func (s *Session) checkStableLocked() (retErr error) {
	defer func() { retErr = s.observeLocked(retErr) }()
	if err := s.checkOpenLocked(); err != nil {
		return err
	}
	if err := s.checkJournalLocked(); err != nil {
		return err
	}
	if err := validateNativePath(s.file); err != nil {
		return err
	}
	st, err := statNativeFile(s.file)
	if err != nil {
		return err
	}
	pathInfo, err := nativeLstat(s.path)
	if err != nil {
		return fmt.Errorf("%w: database pathname: %w", ErrChanged, err)
	}
	if !pathInfo.Mode().IsRegular() || !vfs.SameFile(st, pathInfo) || st.Size() != s.info.Size() || !st.ModTime().Equal(s.info.ModTime()) {
		return ErrChanged
	}
	if err = validateWritableFile(s.file); err != nil {
		return err
	}
	var header [100]byte
	if _, err = readNativeAt(s.file, header[:], 0); err != nil {
		return errors.Join(ErrChanged, err)
	}
	if header != s.rawHeader {
		return fmt.Errorf("%w: physical database header", ErrChanged)
	}
	return s.checkWALStableLocked()
}

func (s *Session) removeStaleSHMLocked() error {
	st, absent, err := s.probePathLocked(s.path + "-shm")
	if absent {
		return nil
	}
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() {
		return fmt.Errorf("%w: stale WAL index must be a regular file", ErrUnsupported)
	}
	if err = os.Remove(s.path + "-shm"); err != nil {
		return err
	}
	return syncDirectory(s.path)
}

func (s *Session) Header() Header {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.header
}

// Begin acquires the sole local transaction lease. Returning BUSY does not
// release the connection's database lock or alter its durable snapshot.
func (s *Session) Begin(ctx context.Context) (*Tx, error) { return s.begin(ctx, false) }

// Reader acquires a read-only lease on the latest durable WAL snapshot.
func (s *Session) Reader(ctx context.Context) (*Reader, error) {
	t, err := s.begin(ctx, true)
	if err != nil {
		return nil, err
	}
	return &Reader{lease: t}, nil
}

func (s *Session) begin(ctx context.Context, readOnly bool) (*Tx, error) {
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
	p, err := s.readPageLocked(ctx, 1)
	if err != nil {
		s.mu.Unlock()
		return nil, err
	}
	t := &Tx{file: s.file, path: s.path, info: s.info, header: s.header,
		originalPages: s.header.PageCount, opts: s.opts, session: s,
		readOnly: readOnly, reserved: !readOnly,
		dirty: make(map[uint32][]byte), free: make(map[uint32]struct{})}
	copy(t.originalHeader[:], p[:100])
	s.active = t
	s.mu.Unlock()
	if !readOnly {
		if err = t.loadFreelist(ctx); err != nil {
			return nil, errors.Join(err, t.Rollback())
		}
	}
	return t, nil
}

func (s *Session) release(t *Tx) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == t {
		s.active = nil
	}
}

func (s *Session) readPage(ctx context.Context, n uint32) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.checkOpenLocked(); err != nil {
		return nil, err
	}
	return s.readPageLocked(ctx, n)
}

func (s *Session) readPageLocked(ctx context.Context, n uint32) (_ []byte, retErr error) {
	defer func() { retErr = s.observeLocked(retErr) }()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if n == 0 || n > s.header.PageCount || n == lockPage(s.header.PageSize) {
		return nil, fmt.Errorf("%w: page %d outside committed extent", ErrCorrupt, n)
	}
	p := make([]byte, s.header.PageSize)
	f, off := s.file, int64(n-1)*int64(s.header.PageSize)
	if ref, ok := s.snapshot.Pages[n]; ok {
		f, off = s.walFile, ref.Offset
	}
	if _, err := readNativeAt(f, p, off); err != nil {
		return nil, fmt.Errorf("%w: read page %d: %w", ErrCorrupt, n, err)
	}
	return p, ctx.Err()
}

func (s *Session) poisonLocked(cause error) error {
	cause = s.observeLocked(cause)
	if s.poisoned == nil {
		s.poisoned = errors.Join(ErrPoisoned, cause)
	}
	return s.poisoned
}

// Close normally checkpoints before releasing EXCLUSIVE. KeepWALOnClose
// retains an already-synced committed WAL instead. An active lease returns
// BUSY. A poisoned session only closes descriptors and preserves its WAL.
func (s *Session) Close() error { return s.CloseWithError(nil) }

// Shared-WAL construction uses a private Session without exclusive ownership.
func (s *Session) closeLegacyLocked() error {
	if s.file == nil {
		return s.poisoned
	}
	if s.active != nil {
		return ErrBusy
	}
	err := s.poisoned
	if err == nil && !s.opts.KeepWALOnClose {
		err = s.checkpointLocked(context.Background())
	}
	if s.walFile != nil {
		err = errors.Join(err, closeNativeFile(s.walFile))
		s.walFile = nil
	}
	err = errors.Join(err, closeNativeFile(s.file))
	s.file = nil
	return err
}
