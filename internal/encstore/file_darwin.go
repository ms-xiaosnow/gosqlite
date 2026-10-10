//go:build darwin

package encstore

import (
	"errors"
	"os"
	"sync"

	"github.com/ms-xiaosnow/gosqlite/encryption"
	"github.com/ms-xiaosnow/gosqlite/internal/darwinfile"
)

// managedFile is a logical lease, never an alias to os.File. Descriptor state,
// directory pins and lock ledgers use darwinfile.Process's shared lifecycle
// gate. Data I/O uses the lease mutex, never the process-wide gate.
type managedFile struct {
	managedOwnership
	mu              sync.Mutex
	descriptor      *encryptedDarwinDescriptor
	closed          bool
	children        int
	closeErr        error
	locks           processByteOwner
	directoryStream darwinDirectoryStream
}
type encryptedDarwinDescriptor struct {
	raw       *os.File
	record    *darwinfile.Record
	node      *encryptedDarwinNode
	access    int
	directory bool
	parent    *managedFile
	path      string
	idle      bool
	lease     *managedFile
}
type encryptedDarwinNode struct {
	id          darwinfile.Identity
	descriptors map[*encryptedDarwinDescriptor]struct{}
	locks       processByteLocks
}

var encryptedDarwinNodes = make(map[darwinfile.Identity]*encryptedDarwinNode)

func encryptedDarwinError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, darwinfile.ErrLimit) {
		return errors.Join(encryption.ErrLimit, err)
	}
	return errors.Join(ErrPoisoned, err)
}

// checkHeld requires the common gate. A closed directory can be used only by a
// child that already retains its actual descriptor; public closure is permanent.
func (f *managedFile) checkHeld(retained bool) error {
	if f == nil || f.descriptor == nil {
		return encryption.ErrClosed
	}
	if f.closeErr != nil {
		return f.closeErr
	}
	if f.closed && !(retained && f.children > 0 && f.descriptor.directory) {
		return encryption.ErrClosed
	}
	d := f.descriptor
	if d.lease != f || d.idle {
		return encryption.ErrClosed
	}
	if err := darwinfile.Process.Check(d.record); err != nil {
		return encryptedDarwinError(err)
	}
	if d.node.locks.poisoned != nil {
		return d.node.locks.poisoned
	}
	return nil
}
func (f *managedFile) rawLocked() (*os.File, error) {
	darwinfile.Process.Lock()
	defer darwinfile.Process.Unlock()
	if err := f.checkHeld(false); err != nil {
		return nil, err
	}
	return f.descriptor.raw, nil
}
func (f *managedFile) Read(b []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, e := f.rawLocked()
	if e != nil {
		return 0, e
	}
	return r.Read(b)
}
func (f *managedFile) ReadAt(b []byte, off int64) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, e := f.rawLocked()
	if e != nil {
		return 0, e
	}
	return r.ReadAt(b, off)
}
func (f *managedFile) Write(b []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, e := f.rawLocked()
	if e != nil {
		return 0, e
	}
	return r.Write(b)
}
func (f *managedFile) WriteAt(b []byte, off int64) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, e := f.rawLocked()
	if e != nil {
		return 0, e
	}
	return r.WriteAt(b, off)
}
func (f *managedFile) Stat() (os.FileInfo, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, e := f.rawLocked()
	if e != nil {
		return nil, e
	}
	return r.Stat()
}
func (f *managedFile) Sync() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	r, e := f.rawLocked()
	if e != nil {
		return e
	}
	return r.Sync()
}

// closeHeld runs only under the lifecycle gate; callers have retired active I/O
// through their lease mutex. Internal parent-pin retirement never acquires a
// File mutex. Terminal close errors remain retained and are never retried.
func (f *managedFile) closeHeld() error {
	if f.closeErr != nil {
		return f.closeErr
	}
	d := f.descriptor
	if d == nil || d.lease != f {
		return nil
	}
	if f.children != 0 {
		return nil
	}
	if !d.idle {
		err := d.node.locks.close(&f.locks, d.nativeLock)
		darwinfile.Process.SetLocks(d.node.id, "encstore", d.node.locks.any())
		if err != nil {
			darwinfile.Process.Poison(d.node.id, err)
			return err
		}
		d.idle = true
	}
	return encryptedDarwinError(darwinfile.Process.Retire(d.record))
}
func (f *managedFile) closeNative() error {
	if f == nil {
		return nil
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	darwinfile.Process.Lock()
	defer darwinfile.Process.Unlock()
	f.closed = true
	return f.closeHeld()
}

// RetirementComplete is a managed-lease fact, not an interpretation of Close's
// diagnostic. A safely idle regular descriptor may belong to the process
// registry, but directory pins, unreleased holds and terminal close uncertainty
// remain this scope's obligations.
func (f *managedFile) RetirementComplete() bool {
	if f == nil {
		return true
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	darwinfile.Process.Lock()
	defer darwinfile.Process.Unlock()
	return f.retirementCompleteHeld()
}
func (f *managedFile) retirementCompleteHeld() bool {
	if !f.closed || f.closeErr != nil || f.children != 0 || f.locks.held != [3]uint8{} {
		return false
	}
	d := f.descriptor
	if d == nil {
		return true
	}
	if d.lease != f {
		return true
	}
	if d.raw == nil {
		return d.parent == nil
	}
	return !d.directory && d.idle
}
