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

	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

var (
	ErrLimit            = errors.New("SQLite transaction resource limit exceeded")
	ErrPoisoned         = errors.New("SQLite transaction cannot continue after an I/O failure")
	ErrRecoveryRequired = errors.New("SQLite database requires rollback-journal recovery")
	ErrCommitUncertain  = errors.New("SQLite commit durability is uncertain")
	ErrMisuse           = errors.New("invalid SQLite transaction operation")
	ErrBusySnapshot     = fmt.Errorf("%w: WAL read snapshot is no longer current", ErrBusy)
)

// Options controls native rollback transactions and WAL sessions.
// A zero PageSize chooses
// 4096 for a newly created database; existing databases retain their page size.
// SectorSize is a conservative storage-sector assumption, not autodetection.
// Set it no smaller than the actual storage sector. Default: 4096 bytes.
// MaxDirtyPages bounds the overlay. MaxUndoBytes bounds all retained statement
// and savepoint snapshots together; zero derives a limit for one maximum-size
// statement snapshot plus metadata. WAL commits also reserve unchanged sector neighbors
// within this bound. No dirty data spills to disk before Commit.
// MaxFreelistPages bounds validation metadata, default 1,048,576 pages.
type Options struct {
	Create           bool
	PageSize         uint32
	MaxDirtyPages    uint32
	MaxFreelistPages uint32
	MaxUndoBytes     int64
	SectorSize       uint32
	// KeepWALOnClose retains a synced WAL on normal Session.Close. It has no
	// effect on the default rollback-mode OpenTx API.
	KeepWALOnClose bool
	// WALPowersafeOverwrite explicitly asserts that WAL/main writes cannot
	// damage bytes outside the requested range. Shared WAL alone uses this
	// opt-in when an imported WAL lacks conservative sector protection.
	// This is a storage assumption, not a durability qualification.
	WALPowersafeOverwrite bool
}

// Tx owns either a writable descriptor with its SHARED snapshot lock, or the
// sole transaction lease on an exclusive or shared WAL session. Rollback-mode promotion
// obtains RESERVED on that same descriptor. Each method is data-race safe, but
// callers must serialize transaction-level operations, including Statement's
// callback. A successful Commit or Rollback closes the transaction. BUSY from
// Commit leaves the complete in-memory transaction available for retry.
type Tx struct {
	mu                 sync.Mutex
	file               *vfs.File
	path               string
	info               os.FileInfo
	header             Header
	originalHeader     [100]byte
	originalPages      uint32
	opts               Options
	dirty              map[uint32][]byte // immutable values: replace, never mutate in place
	free               map[uint32]struct{}
	reserved           bool
	statement          bool
	undo               undoState
	exclusive          bool
	poisoned           error
	session            *Session
	shared             *SharedSession
	encrypted          *EncryptedSession
	encryptedAuthority [32]byte
	encryptedHead      [32]byte
	sharedRead         sharedRead
	readOnly           bool
	// keepFile is private to the rollback-journal WAL-mode transition. The
	// WAL session already holds EXCLUSIVE and retains the same descriptor.
	keepFile bool
}

func normalizedOptions(o Options) (Options, error) {
	if o.MaxUndoBytes < 0 {
		return o, fmt.Errorf("MaxUndoBytes must not be negative")
	}
	if o.PageSize == 0 {
		o.PageSize = 4096
	}
	if o.MaxDirtyPages == 0 {
		o.MaxDirtyPages = 4096
	}
	if o.MaxFreelistPages == 0 {
		o.MaxFreelistPages = 1 << 20
	}
	if o.SectorSize == 0 {
		o.SectorSize = 4096
	}
	if o.PageSize < 512 || o.PageSize > 65536 || o.PageSize&(o.PageSize-1) != 0 || o.SectorSize < 512 || o.SectorSize > 65536 || o.SectorSize&(o.SectorSize-1) != 0 {
		return o, fmt.Errorf("%w: page or sector size", ErrUnsupported)
	}
	return o, nil
}

// OpenTx starts a shared read snapshot. Create permits an absent or empty file.
// Existing rollback journals are deliberately refused: hot-journal recovery
// has not passed the separately authorized durability acceptance gate.
func OpenTx(ctx context.Context, path string, opts Options) (*Tx, error) {
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
	resolved, err := filepath.EvalSymlinks(path)
	if err == nil {
		path = resolved
	} else if opts.Create && os.IsNotExist(err) {
		dir, e := filepath.EvalSymlinks(filepath.Dir(path))
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
	success := false
	defer func() {
		if !success {
			_ = f.Close()
		}
	}()
	info, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("%w: database must be a regular file", ErrUnsupported)
	}
	if err = validateWritableFile(f); err != nil {
		return nil, err
	}
	if err = lockShared(f); err != nil {
		return nil, err
	}
	// Stat after acquiring SHARED: another connection may have committed while
	// this descriptor was being opened.
	info, err = f.Stat()
	if err != nil {
		return nil, err
	}
	t := &Tx{file: f, path: path, info: info, opts: opts, dirty: make(map[uint32][]byte), free: make(map[uint32]struct{})}
	if err = t.checkSidecars(); err != nil {
		return nil, err
	}
	if info.Size() == 0 {
		if !opts.Create {
			return nil, fmt.Errorf("%w: empty database requires Create", ErrCorrupt)
		}
		if err = t.promoteLocked(ctx); err != nil {
			return nil, err
		}
		p := emptyDatabasePage(opts.PageSize)
		t.header, err = ParseHeader(p, int64(len(p)))
		if err != nil {
			return nil, err
		}
		t.dirty[1] = p
	} else {
		if _, err = f.ReadAt(t.originalHeader[:], 0); err != nil {
			return nil, fmt.Errorf("%w: header: %v", ErrCorrupt, err)
		}
		t.header, err = ParseHeader(t.originalHeader[:], info.Size())
		if err != nil {
			return nil, err
		}
		if t.header.ReservedBytes != 0 {
			return nil, fmt.Errorf("%w: writable reserved page bytes", ErrUnsupported)
		}
		if t.header.SchemaFormat != 4 && t.header.SchemaFormat != 0 {
			return nil, fmt.Errorf("%w: writable schema format %d", ErrUnsupported, t.header.SchemaFormat)
		}
		if int64(t.header.PageCount)*int64(t.header.PageSize) != info.Size() {
			return nil, fmt.Errorf("%w: physical pages beyond logical extent", ErrUnsupported)
		}
		t.originalPages = t.header.PageCount
		if err = t.loadFreelist(ctx); err != nil {
			return nil, err
		}
	}
	if err = t.checkStable(); err != nil {
		return nil, err
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	success = true
	return t, nil
}

func emptyDatabasePage(size uint32) []byte {
	p := make([]byte, size)
	copy(p, "SQLite format 3\x00")
	n := uint16(size)
	if size == 65536 {
		n = 1
	}
	binary.BigEndian.PutUint16(p[16:], n)
	p[18], p[19], p[21], p[22], p[23] = 1, 1, 64, 32, 32
	binary.BigEndian.PutUint32(p[28:], 1)
	binary.BigEndian.PutUint32(p[44:], 4)
	binary.BigEndian.PutUint32(p[56:], 1)
	p[100] = 13 // Empty sqlite_schema table-leaf root.
	binary.BigEndian.PutUint16(p[105:], uint16(size))
	return p
}

func (t *Tx) Header() Header { t.mu.Lock(); defer t.mu.Unlock(); return t.header }
func (t *Tx) checkOpen() error {
	if t.poisoned != nil {
		return t.poisoned
	}
	if t.file == nil && t.encrypted == nil {
		return ErrClosed
	}
	return nil
}
func (t *Tx) checkSidecars() error { return checkSidecars(t.path) }

func (t *Tx) checkStable() error {
	if t.encrypted != nil {
		return t.encrypted.checkStable()
	}
	if t.shared != nil {
		return t.shared.checkStable()
	}
	if t.session != nil {
		return t.session.checkStable()
	}
	if err := t.checkSidecars(); err != nil {
		return err
	}
	if err := validateNativePath(t.file); err != nil {
		return err
	}
	st, err := t.file.Stat()
	if err != nil {
		return err
	}
	other, err := os.Lstat(t.path)
	if err != nil {
		return fmt.Errorf("%w: database pathname: %v", ErrChanged, err)
	}
	if !other.Mode().IsRegular() || !os.SameFile(st, other) || st.Size() != t.info.Size() || !st.ModTime().Equal(t.info.ModTime()) {
		return ErrChanged
	}
	if err = validateWritableFile(t.file); err != nil {
		return err
	}
	if t.originalPages > 0 {
		var b [100]byte
		if _, err = t.file.ReadAt(b[:], 0); err != nil {
			return err
		}
		if b != t.originalHeader {
			return ErrChanged
		}
	}
	return nil
}
func (t *Tx) readLocked(ctx context.Context, n uint32) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := t.checkOpen(); err != nil {
		return nil, err
	}
	if n == 0 || n > t.header.PageCount || n == lockPage(t.header.PageSize) {
		return nil, fmt.Errorf("%w: page %d outside usable extent", ErrCorrupt, n)
	}
	if p, ok := t.dirty[n]; ok {
		return append([]byte(nil), p...), nil
	}
	if err := t.checkStable(); err != nil {
		return nil, err
	}
	if t.encrypted != nil {
		return t.encrypted.readPage(ctx, n)
	}
	if t.shared != nil {
		return t.shared.readPage(ctx, t.sharedRead, t.header, n)
	}
	if t.session != nil {
		return t.session.readPage(ctx, n)
	}
	p := make([]byte, t.header.PageSize)
	if _, err := t.file.ReadAt(p, int64(n-1)*int64(t.header.PageSize)); err != nil {
		return nil, err
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return p, nil
}
func lockPage(pageSize uint32) uint32 { return 0x40000000/pageSize + 1 }
func (t *Tx) ReadPage(ctx context.Context, n uint32) ([]byte, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.readLocked(ctx, n)
}
func (t *Tx) promoteLocked(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := t.checkOpen(); err != nil {
		return err
	}
	if t.readOnly {
		return fmt.Errorf("%w: read-only snapshot", ErrMisuse)
	}
	if t.reserved {
		return nil
	}
	if t.encrypted != nil {
		return t.encrypted.promote(ctx, t)
	}
	if t.shared != nil {
		return t.shared.promote(ctx, t)
	}
	if err := lockReserved(t.file); err != nil {
		return err
	}
	t.reserved = true
	return nil
}
func (t *Tx) PromoteWrite(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.promoteLocked(ctx)
}

func (t *Tx) applyPages(pages map[uint32][]byte) error {
	total := uint64(len(t.dirty))
	for n := range pages {
		if _, ok := t.dirty[n]; !ok {
			total++
		}
	}
	if total > uint64(t.opts.MaxDirtyPages) {
		return ErrLimit
	}
	for n, p := range pages {
		t.dirty[n] = p
	}
	return nil
}

func (t *Tx) WritePage(ctx context.Context, n uint32, p []byte) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := t.promoteLocked(ctx); err != nil {
		return err
	}
	if len(p) != int(t.header.PageSize) || n == 0 || n > t.header.PageCount || n == lockPage(t.header.PageSize) {
		return fmt.Errorf("%w: invalid page write", ErrCorrupt)
	}
	if _, free := t.free[n]; free {
		return fmt.Errorf("%w: writing a free page requires allocation", ErrMisuse)
	}
	if n == 1 {
		current, err := t.readLocked(ctx, 1)
		if err != nil {
			return err
		}
		if !bytes.Equal(current[:100], p[:100]) {
			return fmt.Errorf("%w: page-one header is managed by the pager", ErrMisuse)
		}
	}
	changes := map[uint32][]byte{n: append([]byte(nil), p...)}
	if n != 1 {
		p1, err := t.readLocked(ctx, 1)
		if err != nil {
			return err
		}
		changes[1] = p1
	}
	return t.applyPages(changes)
}

func (t *Tx) BumpSchemaCookie(ctx context.Context) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if err := t.promoteLocked(ctx); err != nil {
		return err
	}
	p, err := t.readLocked(ctx, 1)
	if err != nil {
		return err
	}
	next := t.header.SchemaCookie + 1
	binary.BigEndian.PutUint32(p[40:], next)
	if t.header.SchemaFormat == 0 {
		binary.BigEndian.PutUint32(p[44:], 4)
		binary.BigEndian.PutUint32(p[56:], 1)
	}
	if err = t.applyPages(map[uint32][]byte{1: p}); err != nil {
		return err
	}
	t.header.SchemaCookie = next
	if t.header.SchemaFormat == 0 {
		t.header.SchemaFormat = 4
		t.header.TextEncoding = 1
	}
	return nil
}

// Statement creates a bounded memory-only undo boundary, including allocation,
// freelist, and schema-cookie changes. Nested statements are not supported.
// A callback error, cancellation, or panic restores its entry state.
func (t *Tx) Statement(ctx context.Context, fn func() error) error {
	return t.statementUndo(ctx, fn)
}

// Rollback releases locks without changing database bytes. All transactional
// changes are memory-only until Commit enters its safety-critical I/O phase.
func (t *Tx) Rollback() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	defer t.clearClosedUndoLocked()
	if t.statement {
		return fmt.Errorf("%w: rollback inside statement", ErrMisuse)
	}
	if t.encrypted != nil {
		err := t.encrypted.release(t)
		t.clearEncrypted()
		return errors.Join(t.poisoned, err)
	}
	if t.file == nil {
		return t.poisoned
	}
	if t.shared != nil {
		err := t.shared.release(t)
		t.file, t.dirty, t.free = nil, nil, nil
		return errors.Join(t.poisoned, err)
	}
	if t.session != nil {
		t.session.release(t)
		t.file = nil
		t.dirty = nil
		t.free = nil
		return t.poisoned
	}
	err := t.file.Close()
	t.file = nil
	t.dirty = nil
	t.free = nil
	return err
}
func (t *Tx) Close() error { return t.Rollback() }

// ValidateOptions checks transaction configuration without opening a file or
// acquiring a lock. Zero values use the same defaults as OpenTx.
func ValidateOptions(opts Options) error {
	_, err := normalizedOptions(opts)
	return err
}
