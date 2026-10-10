package encstore

import (
	"errors"
	"path/filepath"

	"github.com/ms-xiaosnow/gosqlite/encryption"
)

// managedRetirementError is safe in a native parent-retirement callback. It
// never takes a group mutex (which may already be held), calls cleanup, or opens
// a resource. The parent scope already strongly owns this managed lease.
func managedRetirementError(parent *managedFile, err error) error {
	if err == nil || parent == nil || parent.owner == nil {
		return err
	}
	parent.owner.callbackCause.CompareAndSwap(nil, &cleanupCallback{cause: err})
	e := &cleanupError{group: parent.owner, cause: errors.Join(ErrPoisoned, err)}
	parent.owner.pending.Store(e)
	return e
}
func managedTerminalRetirementError(parent *managedFile, err error) error {
	if parent != nil && parent.owner != nil && err != nil {
		parent.owner.callbackTerminal.Store(true)
	}
	return managedRetirementError(parent, err)
}
func managedRetirementComplete(parent *managedFile) {
	if parent != nil && parent.owner != nil && parent.entry != nil {
		parent.entry.completed.Store(true)
		parent.owner.reap.Store(true)
	}
}
func (g *cleanupGroup) acquireLocked(role cleanupRole, call func() (*managedFile, error)) (*managedFile, error) {
	g.syncCallbacksLocked()
	if g.done || g.closing {
		return nil, encryption.ErrClosed
	}
	if g.terminal {
		return nil, errors.Join(ErrPoisoned, g.cause)
	}
	if g.cause != nil {
		return nil, g.cause
	}
	if err := g.registry.reserveLease(); err != nil {
		return nil, err
	}
	f, err := call()
	if f != nil {
		f.owner = g
		g.addLocked(f, role)
		g.syncCallbacksLocked()
		if err == nil && (g.cause != nil || g.terminal) {
			err = errors.Join(ErrPoisoned, g.cause)
		}
		if err == nil {
			return f, nil
		}
		g.rememberLocked(err)
		entry := g.files[f]
		entry.requested = true
		closeErr := g.stepLocked(entry)
		if _, retained := g.files[f]; retained {
			return nil, g.tokenLocked(errors.Join(err, closeErr))
		}
		return nil, errors.Join(err, closeErr)
	}
	var unknown *nativeOwnershipError
	if errors.As(err, &unknown) {
		// The native registry owns the quarantined/tombstoned resource. Its reserved
		// common lease is intentionally never freed without a completion proof.
		g.terminal = true
		return nil, g.tokenLocked(err)
	}
	g.registry.releaseLease()
	return nil, g.rememberLocked(err)
}
func scopedNativeOpen(parent *managedFile, call func() (*managedFile, error)) (*managedFile, error) {
	if parent == nil || parent.owner == nil {
		return nil, ErrUnsupported
	}
	g := parent.owner
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.acquireLocked(cleanupChild, call)
}
func openDirectory(path string) (*managedFile, error) {
	g, err := encryptedCleanups.reserve()
	if err != nil {
		return nil, err
	}
	g.mu.Lock()
	f, err := g.acquireLocked(cleanupDirectory, func() (*managedFile, error) { return nativeOpenDirectory(path) })
	if err != nil {
		g.releaseLocked()
	}
	g.mu.Unlock()
	if err != nil {
		return nil, g.closeAll(err)
	}
	return f, nil
}
func openFileAt(dir *managedFile, name string, flags int) (*managedFile, error) {
	return scopedNativeOpen(dir, func() (*managedFile, error) { return nativeOpenFileAt(dir, name, flags, false) })
}
func openRootFileAt(dir *managedFile, name string, flags int) (*managedFile, error) {
	return scopedNativeOpen(dir, func() (*managedFile, error) { return nativeOpenFileAt(dir, name, flags, true) })
}
func openFile(path string, flags int) (*managedFile, error) {
	d, err := openDirectory(filepath.Dir(path))
	if err != nil {
		return nil, err
	}
	f, err := openFileAt(d, filepath.Base(path), flags)
	if err != nil {
		return nil, d.owner.closeAll(err)
	}
	if err = d.Close(); err != nil {
		return nil, d.owner.closeAll(err)
	}
	return f, nil
}
