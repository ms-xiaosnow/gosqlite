package encstore

import "errors"

// admitLifecycle uses the existing capture byte as a short Windows admission
// gate for every initial lifecycle acquisition, including legacy-mode opens.
// It adds no lock byte or persistent metadata. Callers hold the lease mutex.
// This file is portable only so its exact production ledger can be exercised
// with inert callbacks; only the Windows backend invokes it for native locks.
func (l *exactByteLocks) admitLifecycle(shared bool, call byteLockCall) error {
	if l.poisoned != nil {
		return l.poisoned
	}
	if l.held != [3]uint8{} {
		return ErrUnsupported
	}
	if err := l.change(captureByte, false, false, call); err != nil {
		return err
	}
	err := l.change(lifecycleByte, shared, false, call)
	if l.poisoned != nil {
		return err // Retain the gate and handle for managed retirement.
	}
	return errors.Join(err, l.change(captureByte, false, true, call))
}

// upgradeLifecycle is bounded guarded conversion, NOT an atomic LockFileEx
// upgrade. It requires the existing writer reservation and excludes updated
// lifecycle admission through byte 2 before temporarily releasing byte 0.
// Another shared owner causes BUSY only after SHARED is restored. Older clients
// may ignore admission: an intervening exclusive owner can prevent restoration,
// which poisons the lease and retains both protectors and every child. Even a
// successful conversion requires the Store's post-conversion exact authority
// check before validation/copy/publication, since an old owner may have retired.
func (l *exactByteLocks) upgradeLifecycle(call byteLockCall) error {
	if l.poisoned != nil {
		return l.poisoned
	}
	if l.held != [3]uint8{1, 2, 0} {
		return ErrUnsupported
	}
	if err := l.change(captureByte, false, false, call); err != nil {
		return err
	}
	if err := l.change(lifecycleByte, false, true, call); err != nil {
		return err
	}
	err := l.change(lifecycleByte, false, false, call)
	if l.poisoned != nil {
		return err // No ordinary action may release the remaining protectors.
	}
	if err != nil {
		if restore := l.change(lifecycleByte, true, false, call); restore != nil {
			// Restoration BUSY is lost lifecycle ownership, never retryable BUSY.
			return l.poison(errors.Join(err, restore))
		}
	}
	return errors.Join(err, l.change(captureByte, false, true, call))
}
