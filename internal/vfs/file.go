// Package vfs owns native files, SQLite byte locks, and shared mappings.
// It deliberately does not implement database transactions or publication.
package vfs

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type Role uint8

const (
	Database Role = iota + 1
	WAL
	SHM
	Journal
	Directory
)

type OpenOptions struct {
	Role  Role
	Flags int
	// Replaceable is confined to an exclusive session's WAL and its candidate.
	// Shared-WAL files must never use it.
	Replaceable bool
}

var (
	ErrBusy        = errors.New("native file lock busy")
	ErrUnsupported = errors.New("native file operation unsupported")
	ErrIdentity    = errors.New("native file identity changed")
	ErrLinks       = fmt.Errorf("%w: file must have exactly one hard link", ErrUnsupported)
	ErrLock        = errors.New("invalid native lock operation")
	ErrClosed      = errors.New("native file is closed")
	ErrPoisoned    = errors.New("native file ownership cannot safely continue")
)

// File is a managed lease. The descriptor is private: all close paths,
// including unsuccessful opens and read-only probes, use this owner.
// Live leases are strongly retained until explicit Close, so a finalizer can
// never silently discard a lock or a still-live shared mapping.
type File struct {
	mu                               sync.Mutex
	raw                              *os.File
	native                           nativeFile
	path                             string
	role                             Role
	writable                         bool
	replaceable                      bool
	identity                         os.FileInfo
	parent                           os.FileInfo
	closing                          bool
	shared, reserved, exclusive, dms bool
	slots                            slotLedger
	maps                             map[*Mapping]struct{}
}

var owners = struct {
	sync.Mutex
	files map[*File]struct{}
}{files: make(map[*File]struct{})}

func Open(path string, opts OpenOptions) (*File, error) {
	if opts.Role < Database || opts.Role > Directory || opts.Replaceable && opts.Role != WAL {
		return nil, ErrUnsupported
	}
	if opts.Flags & ^(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_EXCL) != 0 || opts.Flags&(os.O_WRONLY|os.O_RDWR) == os.O_WRONLY|os.O_RDWR {
		return nil, ErrUnsupported
	}
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	raw, err := nativeOpen(path, opts)
	if err != nil {
		return nil, unmanagedOpenError(err)
	}
	f := &File{raw: raw, path: path, role: opts.Role, writable: opts.Flags&(os.O_WRONLY|os.O_RDWR) != 0, replaceable: opts.Replaceable, maps: make(map[*Mapping]struct{})}
	owners.Lock()
	owners.files[f] = struct{}{}
	owners.Unlock()
	// Register before validation or cleanup. Darwin's backend must additionally
	// register the physical descriptor under its inode lifecycle gate.
	err = nativeOpened(f)
	if err == nil {
		f.identity, err = nativeStat(f)
	}
	if err == nil {
		if opts.Role == Directory {
			if !f.identity.IsDir() {
				err = ErrUnsupported
			}
		} else if !f.identity.Mode().IsRegular() {
			err = ErrUnsupported
		}
	}
	if err == nil {
		f.parent, err = nativeParentInfo(f)
	}
	if err == nil && f.writable {
		err = nativeValidateLinks(f)
	}
	if err == nil {
		err = f.validatePathLocked()
	}
	if err != nil {
		return nil, errors.Join(err, f.Close())
	}
	return f, nil
}
func (f *File) checkLocked() error {
	if f.raw == nil {
		return ErrClosed
	}
	if f.closing {
		return ErrPoisoned
	}
	return nativeCheck(f)
}
func (f *File) Name() string { f.mu.Lock(); defer f.mu.Unlock(); return f.path }
func (f *File) Read(b []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(); err != nil {
		return 0, err
	}
	return f.raw.Read(b)
}
func (f *File) ReadAt(b []byte, off int64) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(); err != nil {
		return 0, err
	}
	return f.raw.ReadAt(b, off)
}
func (f *File) WriteAt(b []byte, off int64) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(); err != nil {
		return 0, err
	}
	return f.raw.WriteAt(b, off)
}
func (f *File) Stat() (os.FileInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(); err != nil {
		return nil, err
	}
	return nativeStat(f)
}
func (f *File) Truncate(size int64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(); err != nil {
		return err
	}
	return f.raw.Truncate(size)
}
func (f *File) Sync() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(); err != nil {
		return err
	}
	return nativeSync(f)
}
func (f *File) ValidateSingleLink() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(); err != nil {
		return err
	}
	return nativeValidateLinks(f)
}
func (f *File) validatePathLocked() error {
	if err := f.checkLocked(); err != nil {
		return err
	}
	if err := nativeValidatePath(f); err != nil {
		return err
	}
	info, err := nativeStat(f)
	if err != nil {
		return err
	}
	named, err := Lstat(f.path)
	if err != nil {
		return errors.Join(ErrIdentity, err)
	}
	parent, err := statParentPath(filepath.Dir(f.path))
	if err != nil {
		return errors.Join(ErrIdentity, err)
	}
	if named.Mode()&os.ModeSymlink != 0 || !SameFile(info, f.identity) || !SameFile(info, named) || !SameFile(parent, f.parent) {
		return ErrIdentity
	}
	return nil
}
func (f *File) ValidatePath() error { f.mu.Lock(); defer f.mu.Unlock(); return f.validatePathLocked() }

// Close retires every mapped resource before releasing DMS or its descriptor.
// An unresolved unmap or poisoned native retirement keeps the owner strongly
// retained and rejects new I/O. Close may retry represented resources; a backend
// with an indeterminate physical close can retain an explicit tombstone instead
// of retrying a possibly reused handle. No finalizer releases protecting locks.
func (f *File) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.raw == nil {
		return nil
	}
	f.closing = true
	err := f.closeResourcesLocked(nativeUnmap, func() error { return nativeClose(f) })
	if len(f.maps) != 0 || errors.Is(err, ErrPoisoned) {
		// Nested constructors may return nil plus this error. Carry the
		// managed retry obligation to their parent instead of losing it.
		return retainedFileCleanup(f, err)
	}
	f.raw = nil
	owners.Lock()
	delete(owners.files, f)
	owners.Unlock()
	return err
}

// closeResourcesLocked keeps native retirement and descriptor release in one
// ordered operation. Tests exercise it with memory-only backend callbacks.
func (f *File) closeResourcesLocked(unmap func(*nativeMapping) error, closeFile func() error) error {
	var errs []error
	for m := range f.maps {
		if err := m.retireLocked(unmap); err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) != 0 {
		return errors.Join(append([]error{ErrPoisoned}, errs...)...)
	}
	return closeFile()
}

// SyncDirectory preserves the platform's declared directory publication policy.
func SyncDirectory(path string) error {
	f, err := Open(path, OpenOptions{Role: Directory})
	if err != nil {
		return err
	}
	return errors.Join(f.Sync(), f.Close())
}

// CreateWALCandidate creates an exclusive, managed file in the database's
// existing directory. It never opens an unmanaged temporary descriptor.
func CreateWALCandidate(databasePath string) (*File, error) {
	for attempt := 0; attempt < 100; attempt++ {
		var random [12]byte
		if _, err := rand.Read(random[:]); err != nil {
			return nil, err
		}
		path := databasePath + "-wal-pad-" + hex.EncodeToString(random[:])
		f, err := Open(path, OpenOptions{Role: WAL, Flags: os.O_RDWR | os.O_CREATE | os.O_EXCL, Replaceable: true})
		if os.IsExist(err) {
			continue
		}
		return f, err
	}
	return nil, fmt.Errorf("create managed WAL candidate: %w", os.ErrExist)
}

// ReplaceWAL is used only by imported-WAL normalization in an exclusive
// session. It does not sync or close either lease; publication order belongs
// to the pager. A shared-WAL lease cannot enter this namespace operation.
func ReplaceWAL(db, old, candidate *File) error {
	if db == old || old == candidate || db == candidate {
		return ErrLock
	}
	// Callers serialize their session. Fixed role order avoids cross-role cycles.
	db.mu.Lock()
	defer db.mu.Unlock()
	old.mu.Lock()
	defer old.mu.Unlock()
	candidate.mu.Lock()
	defer candidate.mu.Unlock()
	if db.role != Database || !db.exclusive || old.role != WAL || candidate.role != WAL || !old.replaceable || !candidate.replaceable {
		return ErrLock
	}
	if old.path != db.path+"-wal" || filepath.Dir(candidate.path) != filepath.Dir(old.path) || !strings.HasPrefix(filepath.Base(candidate.path), filepath.Base(db.path)+"-wal-pad-") {
		return ErrIdentity
	}
	for _, f := range []*File{db, old, candidate} {
		if err := f.validatePathLocked(); err != nil {
			return err
		}
	}
	if err := nativeReplace(candidate.path, old.path); err != nil {
		if errors.Is(err, ErrPoisoned) {
			db.closing, old.closing, candidate.closing = true, true, true
		}
		return err
	}
	candidate.path = old.path
	if err := candidate.validatePathLocked(); err != nil {
		// A completed namespace operation with unverified destination identity
		// must not leave any publication participant usable as a cached owner.
		db.closing, old.closing, candidate.closing = true, true, true
		return errors.Join(ErrPoisoned, err)
	}
	return nil
}
