package vfs

import "errors"

// mappingRetirement owns the independent Windows mapping-object handle and
// mapped view. The file mutex serializes these transitions. Keeping this small
// state machine portable lets ownership laws use only integers and memory
// callbacks; native production callers supply private Win32 operations.
type mappingRetirement struct {
	base   uintptr
	length int
	handle uintptr
	// A failed handle close is terminal uncertainty, not proof that this
	// numeric value still refers to our object. Keep its first cause and all
	// resource records, without ever issuing another close or view retirement.
	closeUnknown error
}

func (m *mappingRetirement) retire(closeHandle, unmapView func(uintptr) error) error {
	if m.closeUnknown != nil {
		return m.closeUnknown
	}
	// Closing the object before the view preserves the exposed Bytes if close
	// fails. A view independently retains the object after a successful close.
	if m.handle != 0 {
		if err := closeHandle(m.handle); err != nil {
			m.closeUnknown = errors.Join(ErrPoisoned, err)
			return m.closeUnknown
		}
		m.handle = 0
	}
	if m.base != 0 {
		if err := unmapView(m.base); err != nil {
			return err // The represented view can be retried; its handle cannot.
		}
		m.base = 0
	}
	m.length = 0
	return nil
}
