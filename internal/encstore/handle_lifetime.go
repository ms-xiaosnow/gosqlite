package encstore

import (
	"errors"

	"github.com/ms-xiaosnow/gosqlite/encryption"
)

// handleLifetime is Windows' private parent/child retirement state. Native
// handles remain in their managedFile; callbacks here permit memory-only law
// tests without executing failed native close or lock operations.
type handleLifetime struct {
	children                      int
	closed, closeUnknown, retired bool
}

func (l *handleLifetime) usable(retainedParent bool) error {
	if l.closeUnknown {
		return ErrPoisoned
	}
	if l.retired || l.closed && !(retainedParent && l.children > 0) {
		return encryption.ErrClosed
	}
	return nil
}
func (l *handleLifetime) addChild(retained bool) error {
	if err := l.usable(retained); err != nil {
		return err
	}
	l.children++
	return nil
}
func (l *handleLifetime) dropChild() error {
	if l.children <= 0 {
		return ErrPoisoned
	}
	l.children--
	return nil
}
func (l *handleLifetime) retire(unlock, closeHandle func() error) (bool, error) {
	l.closed = true
	if l.closeUnknown {
		return false, ErrPoisoned
	}
	if l.retired {
		return true, nil
	}
	if l.children != 0 {
		return false, nil
	}
	if err := unlock(); err != nil {
		return false, errors.Join(ErrPoisoned, err)
	}
	if err := closeHandle(); err != nil {
		l.closeUnknown = true
		return false, errors.Join(ErrPoisoned, err)
	}
	l.retired = true
	return true, nil
}
