//go:build !darwin && !(windows && amd64)

package encstore

import (
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/ms-xiaosnow/gosqlite/encryption"
)

// managedFile owns a single os.File lease. Linux Close consumes that lease even
// when it reports a diagnostic; no path ever retries its numeric descriptor.
// Other operating systems using this type remain fail-closed at native open.
type managedFile struct {
	managedOwnership
	mu               sync.Mutex
	raw              *os.File
	parent           *managedFile
	directory        bool
	children         int
	closed, consumed bool
	closeErr         error
}

func (f *managedFile) checkLocked(retained bool) error {
	if f == nil || f.raw == nil || f.consumed || f.closed && !(retained && f.directory && f.children > 0) {
		return encryption.ErrClosed
	}
	return nil
}
func (f *managedFile) Read(b []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(false); err != nil {
		return 0, err
	}
	return f.raw.Read(b)
}
func (f *managedFile) ReadAt(b []byte, off int64) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(false); err != nil {
		return 0, err
	}
	return f.raw.ReadAt(b, off)
}
func (f *managedFile) Write(b []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(false); err != nil {
		return 0, err
	}
	return f.raw.Write(b)
}
func (f *managedFile) WriteAt(b []byte, off int64) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(false); err != nil {
		return 0, err
	}
	return f.raw.WriteAt(b, off)
}
func (f *managedFile) Stat() (os.FileInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(false); err != nil {
		return nil, err
	}
	return f.raw.Stat()
}
func (f *managedFile) ReadDir(n int) ([]os.DirEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(false); err != nil {
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
	if err := f.checkLocked(false); err != nil {
		return err
	}
	return f.raw.Sync()
}

// finishLocked detaches the actual os.File before its one consumed close call.
// Child pins delay directory closure; releasing a pin never reopens a lease.
func (f *managedFile) finishLocked() (*managedFile, error) {
	if f.consumed {
		return nil, f.closeErr
	}
	if f.children != 0 {
		return nil, nil
	}
	if f.raw == nil {
		return nil, ErrPoisoned
	}
	raw := f.raw
	f.raw = nil
	f.consumed = true
	f.closeErr = raw.Close()
	parent := f.parent
	f.parent = nil
	return parent, f.closeErr
}
func (f *managedFile) releaseChild() error {
	f.mu.Lock()
	if f.children <= 0 {
		f.mu.Unlock()
		return managedRetirementError(f, ErrPoisoned)
	}
	f.children--
	var parent *managedFile
	var err error
	if f.closed {
		parent, err = f.finishLocked()
	}
	// Linux consumes the lease even with a close diagnostic.
	if f.retirementCompleteLocked() {
		managedRetirementComplete(f)
	}
	f.mu.Unlock()
	if parent != nil {
		err = errors.Join(err, parent.releaseChild())
	}
	return err
}
func (f *managedFile) closeNative() error {
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
func (f *managedFile) RetirementComplete() bool {
	if f == nil {
		return true
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.retirementCompleteLocked()
}
func (f *managedFile) retirementCompleteLocked() bool {
	return f.closed && f.consumed && f.raw == nil && f.parent == nil && f.children == 0
}

func canonicalPath(path string) (string, error) {
	p, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(p))
	if err != nil {
		return "", err
	}
	p = filepath.Join(dir, filepath.Base(p))
	if info, e := os.Lstat(p); e == nil && !info.Mode().IsRegular() {
		return "", ErrUnsupported
	} else if e != nil && !os.IsNotExist(e) {
		return "", e
	}
	return p, nil
}
func platformPathInfo(path string) (os.FileInfo, error)   { return os.Lstat(path) }
func platformFamilyEntry(name, base string) (bool, error) { return familyEntry(name, base) }
func platformOperation(encryption.Operation, Mode) error  { return supported() }
func platformPreflight(string) error                      { return supported() }
func platformFileCheck(f *managedFile, path string, info os.FileInfo) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(false); err != nil {
		return err
	}
	named, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !named.Mode().IsRegular() || !os.SameFile(info, named) {
		return ErrChanged
	}
	return nil
}
func platformDirectoryCheck(f *managedFile, path string, info os.FileInfo) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(false); err != nil {
		return err
	}
	named, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !named.IsDir() || !os.SameFile(info, named) {
		return ErrChanged
	}
	return nil
}
func replaceFile(old, new string) error { return os.Rename(old, new) }
