package encstore

import (
	"errors"
	"fmt"
)

// exactByteLocks is the Windows ledger, also exercised by pure-memory models.
// Every bit denotes one successful acquisition of this exact one-byte interval.
// The caller serializes access. Linux never uses this ledger or its policy.
type exactByteLocks struct {
	held          [3]uint8 // 1 shared, 2 exclusive, 3 shared layered over exclusive
	poisoned      error
	cleanupCauses [3]error
}
type byteLockCall func(offset int64, shared, unlock bool) error

func (l *exactByteLocks) poison(err error) error {
	if l.poisoned == nil {
		l.poisoned = errors.Join(ErrPoisoned, err)
		return l.poisoned
	}
	return errors.Join(l.poisoned, l.cleanupCauses[0], l.cleanupCauses[1], l.cleanupCauses[2], err)
}
func (l *exactByteLocks) change(offset int64, shared, unlock bool, call byteLockCall) error {
	if l.poisoned != nil {
		return l.poisoned
	}
	if offset < lifecycleByte || offset > captureByte {
		return ErrUnsupported
	}
	if unlock {
		return l.release(offset, call)
	}
	want := uint8(2)
	if shared {
		want = 1
	}
	old := l.held[offset]
	if old == want {
		return nil
	}
	// LockFileEx cannot acquire exclusive over a shared lock, even its own.
	// Direct requests never drop shared protection. Windows lifecycle conversion
	// has a separate guarded protocol and requires a Store authority recheck.
	if old == 1 {
		return fmt.Errorf("%w: direct Windows shared-to-exclusive lock conversion", ErrUnsupported)
	}
	if old == 3 {
		return l.poison(ErrUnsupported)
	}
	if err := call(offset, shared, false); err != nil {
		if errors.Is(err, ErrBusy) {
			return err
		}
		return l.poison(err)
	}
	l.held[offset] |= want
	if old == 2 {
		// Microsoft specifies that the first unlock of this stacked interval
		// releases exclusive, leaving the newly acquired shared lock in place.
		if err := call(offset, false, true); err != nil {
			if l.cleanupCauses[offset] == nil {
				l.cleanupCauses[offset] = err
			}
			return l.poison(err)
		}
		l.held[offset] = 1
	}
	return nil
}
func (l *exactByteLocks) release(offset int64, call byteLockCall) error {
	for l.held[offset] != 0 {
		if err := call(offset, false, true); err != nil {
			if l.cleanupCauses[offset] == nil {
				l.cleanupCauses[offset] = err
			}
			return l.poison(err)
		}
		if l.held[offset] == 3 {
			l.held[offset] = 1
		} else {
			l.held[offset] = 0
		}
	}
	return nil
}
func (l *exactByteLocks) close(call byteLockCall) error {
	// Retire short gate, writer, then lifetime. Uncertain cleanup retains the
	// remaining exact records and the actual handle; it is never retryable BUSY.
	for off := captureByte; off >= lifecycleByte; off-- {
		if err := l.release(off, call); err != nil {
			return err
		}
	}
	return nil
}
