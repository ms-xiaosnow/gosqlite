//go:build windows && amd64

package encstore

import (
	"errors"
	"os"

	"github.com/ms-xiaosnow/gosqlite/encryption"
)

func (f *managedFile) checkLocked() error {
	if f.locks.poisoned != nil {
		return f.locks.poisoned
	}
	if err := f.handleLifetime.usable(false); err != nil {
		return err
	}
	if f.raw == nil {
		return encryption.ErrClosed
	}
	return nil
}
func (f *managedFile) checkParentLocked(retained bool) error {
	if f.locks.poisoned != nil {
		return f.locks.poisoned
	}
	if err := f.handleLifetime.usable(retained); err != nil {
		return err
	}
	if f.raw == nil {
		return encryption.ErrClosed
	}
	return nil
}
func (f *managedFile) Read(b []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(); err != nil {
		return 0, err
	}
	return f.raw.Read(b)
}
func (f *managedFile) ReadAt(b []byte, off int64) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(); err != nil {
		return 0, err
	}
	return f.raw.ReadAt(b, off)
}
func (f *managedFile) Write(b []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(); err != nil {
		return 0, err
	}
	return f.raw.Write(b)
}
func (f *managedFile) WriteAt(b []byte, off int64) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(); err != nil {
		return 0, err
	}
	return f.raw.WriteAt(b, off)
}
func (f *managedFile) Stat() (os.FileInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(); err != nil {
		return nil, err
	}
	return f.raw.Stat()
}
func (f *managedFile) ReadDir(n int) ([]os.DirEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(); err != nil {
		return nil, err
	}
	if !f.directory {
		return nil, ErrUnsupported
	}
	return f.raw.ReadDir(n)
}
func (f *managedFile) Sync() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(); err != nil {
		return err
	}
	// Native Windows has no promised fsync-directory equivalent. This is an
	// explicit namespace-barrier gap, not a successful directory flush claim.
	if f.directory {
		return nil
	}
	return f.raw.Sync()
}

// finishLocked closes exactly once. An os.File.Close error is terminal and
// uncertain; never retry a numeric HANDLE that could already have been reused.
// It returns a parent only after this actual child handle has retired.
func (f *managedFile) finishLocked() (*managedFile, error) {
	if f.raw == nil {
		return nil, nil
	}
	done, err := f.handleLifetime.retire(func() error { return f.locks.close(f.nativeLock) }, f.raw.Close)
	if err != nil {
		return nil, f.locks.poison(err)
	}
	if !done {
		return nil, nil
	}
	f.raw = nil
	parent := f.parent
	f.parent = nil
	encryptedWinOwners.Lock()
	delete(encryptedWinOwners.files, f)
	encryptedWinOwners.Unlock()
	return parent, nil
}
func (f *managedFile) releaseChild() error {
	f.mu.Lock()
	if childErr := f.dropChild(); childErr != nil {
		err := f.locks.poison(childErr)
		f.mu.Unlock()
		return err
	}
	var parent *managedFile
	var err error
	if f.closed {
		parent, err = f.finishLocked()
	}
	f.mu.Unlock()
	if parent != nil {
		err = errors.Join(err, parent.releaseChild())
	}
	return err
}
func (f *managedFile) Close() error {
	if f == nil {
		return nil
	}
	f.mu.Lock()
	f.closed = true
	parent, err := f.finishLocked()
	f.mu.Unlock()
	if parent != nil {
		err = errors.Join(err, parent.releaseChild())
	}
	return err
}
