package vfs

import "errors"

// These byte positions are the native SQLite database and SHM lock protocol.
const (
	PendingByte  int64 = 0x40000000
	ReservedByte int64 = PendingByte + 1
	SharedFirst  int64 = PendingByte + 2
	SharedSize   int64 = 510
	SHMFirst     int64 = 120
	DMSByte      int64 = 128
)

type DBLock uint8

const (
	Shared DBLock = iota + 1
	Reserved
	Exclusive
)

// lockResultLocked preserves uncertain native lock state until Close. In
// particular, a partially completed downgrade must never leave cached EXCLUSIVE
// state usable by a later LockDB call. It performs no additional native I/O.
func (f *File) lockResultLocked(err error) error {
	if errors.Is(err, ErrPoisoned) {
		f.closing = true
	}
	return err
}

func (f *File) LockDB(target DBLock) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(); err != nil {
		return err
	}
	if f.role != Database {
		return ErrLock
	}
	switch target {
	case Shared:
		if f.shared {
			return nil
		}
		if err := nativeShared(f); err != nil {
			return f.lockResultLocked(err)
		}
		f.shared = true
	case Reserved:
		if !f.shared {
			return ErrLock
		}
		if f.reserved {
			return nil
		}
		if err := nativeReserved(f); err != nil {
			return f.lockResultLocked(err)
		}
		f.reserved = true
	case Exclusive:
		if !f.shared {
			return ErrLock
		}
		if f.exclusive {
			return nil
		}
		if err := nativeExclusive(f); err != nil {
			return f.lockResultLocked(err)
		}
		f.exclusive = true
	default:
		return ErrLock
	}
	return nil
}
func (f *File) DowngradeDBToShared() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(); err != nil {
		return err
	}
	if f.role != Database || !f.shared {
		return ErrLock
	}
	if !f.exclusive {
		return nil
	}
	if err := nativeDowngrade(f); err != nil {
		return f.lockResultLocked(err)
	}
	f.exclusive = false
	return nil
}
func (f *File) ReservedHeld() (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(); err != nil {
		return false, err
	}
	if f.role != Database {
		return false, ErrLock
	}
	if f.reserved {
		return true, nil
	}
	return nativeReservedHeld(f)
}
func (f *File) AttachDMS() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(); err != nil {
		return err
	}
	if f.role != SHM {
		return ErrLock
	}
	if f.dms {
		return nil
	}
	if err := nativeAttachDMS(f); err != nil {
		return f.lockResultLocked(err)
	}
	f.dms = true
	return nil
}

type slotLedger [8]uint8

func validSlots(slot, n int) bool { return slot >= 0 && n > 0 && slot < 8 && n <= 8-slot }
func (s *slotLedger) prepare(slot, n int, exclusive bool) (uint8, bool, error) {
	if !validSlots(slot, n) || !exclusive && n != 1 {
		return 0, false, ErrLock
	}
	mode := uint8(1)
	if exclusive {
		mode = 2
	}
	missing := false
	for i := slot; i < slot+n; i++ {
		if s[i] != 0 && s[i] != mode {
			return 0, false, ErrLock
		}
		missing = missing || s[i] == 0
	}
	return mode, missing, nil
}
func (s *slotLedger) assign(slot, n int, mode uint8) {
	for i := slot; i < slot+n; i++ {
		s[i] = mode
	}
}

// LockSHM accepts only explicit unlocked→held and same-mode idempotent
// requests. Changing a held mode requires Unlock first (SQLite xShmLock).
func (f *File) LockSHM(slot, n int, exclusive bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(); err != nil {
		return err
	}
	if f.role != SHM || !f.dms {
		return ErrLock
	}
	if err := readonlySHMLockAccess(f.writable, exclusive); err != nil {
		return err
	}
	mode, missing, err := f.slots.prepare(slot, n, exclusive)
	if err != nil {
		return err
	}
	if !missing {
		return nil
	}
	if err = nativeSHMLock(f, slot, n, exclusive, false); err != nil {
		return f.lockResultLocked(err)
	}
	f.slots.assign(slot, n, mode)
	return nil
}
func (f *File) UnlockSHM(slot, n int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.checkLocked(); err != nil {
		return err
	}
	if f.role != SHM || !f.dms || !validSlots(slot, n) {
		return ErrLock
	}
	any := false
	for i := slot; i < slot+n; i++ {
		any = any || f.slots[i] != 0
	}
	if !any {
		return nil
	}
	if err := nativeSHMLock(f, slot, n, false, true); err != nil {
		return f.lockResultLocked(errors.Join(ErrPoisoned, err))
	}
	f.slots.assign(slot, n, 0)
	return nil
}
