package vfs

import (
	"errors"
	"os"
	"testing"
)

// These regressions are pure ownership models. The synthetic os.File values
// are never opened, registered, or passed to any native backend operation.
func TestOwnershipReviewPartialRetirementRetainsAllLocks(t *testing.T) {
	f := &File{
		raw: &os.File{}, closing: true,
		shared: true, reserved: true, exclusive: true, dms: true,
		slots: slotLedger{2, 0, 0, 1}, maps: make(map[*Mapping]struct{}),
	}
	retired := &Mapping{Bytes: []byte{10}, owner: f}
	retained := &Mapping{Bytes: []byte{20}, owner: f}
	f.maps[retired], f.maps[retained] = struct{}{}, struct{}{}
	pending := errors.New("model resource still retained")
	closeCalls := 0
	closeFile := func() error {
		closeCalls++
		if len(f.maps) != 0 {
			t.Fatal("descriptor close ran before all view retirement")
		}
		return nil
	}
	unmap := func(native *nativeMapping) error {
		if native == &retained.native {
			return pending
		}
		return nil
	}
	err := f.closeResourcesLocked(unmap, closeFile)
	if !errors.Is(err, ErrPoisoned) || !errors.Is(err, pending) {
		t.Fatalf("missing ownership error: %v", err)
	}
	if closeCalls != 0 || f.raw == nil || !f.shared || !f.reserved || !f.exclusive || !f.dms || f.slots != (slotLedger{2, 0, 0, 1}) {
		t.Fatal("partial retirement released a descriptor or lock")
	}
	if len(f.maps) != 1 || retired.Bytes != nil || len(retained.Bytes) != 1 || retained.Bytes[0] != 20 {
		t.Fatal("partial retirement lost retained mapping or kept retired mapping")
	}
	if _, ok := f.maps[retained]; !ok {
		t.Fatal("unresolved mapping is no longer owned")
	}
	if err := f.checkLocked(); !errors.Is(err, ErrPoisoned) {
		t.Fatalf("closing owner remains usable: %v", err)
	}
	unmapCalls := 0
	err = f.closeResourcesLocked(func(native *nativeMapping) error {
		unmapCalls++
		if native != &retained.native || !f.dms || closeCalls != 0 {
			t.Fatal("retry violated view-before-lock retirement")
		}
		return nil
	}, closeFile)
	if err != nil || closeCalls != 1 || unmapCalls != 1 || len(f.maps) != 0 || retained.Bytes != nil {
		t.Fatalf("retry failed to retire just the remaining view: %v", err)
	}
}

func TestOwnershipReviewClosingLeaseRejectsReuse(t *testing.T) {
	// A non-nil, never-opened placeholder lets checkLocked inspect only state.
	// Every tested operation must return before reaching this placeholder.
	f := &File{raw: &os.File{}, closing: true, role: Database, shared: true, exclusive: true}
	checks := map[string]func() error{
		"read":           func() error { _, err := f.Read(nil); return err },
		"read at":        func() error { _, err := f.ReadAt(nil, 0); return err },
		"write at":       func() error { _, err := f.WriteAt(nil, 0); return err },
		"stat":           func() error { _, err := f.Stat(); return err },
		"sync":           f.Sync,
		"path":           f.ValidatePath,
		"links":          f.ValidateSingleLink,
		"shared":         func() error { return f.LockDB(Shared) },
		"reserved":       func() error { return f.LockDB(Reserved) },
		"exclusive":      func() error { return f.LockDB(Exclusive) },
		"downgrade":      f.DowngradeDBToShared,
		"reserved probe": func() error { _, err := f.ReservedHeld(); return err },
		"DMS":            f.AttachDMS,
		"SHM lock":       func() error { return f.LockSHM(0, 1, true) },
		"SHM unlock":     func() error { return f.UnlockSHM(0, 1) },
		"mapping":        func() error { _, err := f.MapShared(0, 1); return err },
	}
	for name, check := range checks {
		t.Run(name, func(t *testing.T) {
			if err := check(); !errors.Is(err, ErrPoisoned) {
				t.Fatalf("operation escaped poisoned owner: %v", err)
			}
		})
	}
}

func TestOwnershipReviewLockPoisonLatch(t *testing.T) {
	// The cached EXCLUSIVE bit models a downgrade that changed physical lock
	// mode before its cleanup failed. No syscall or OS error is produced here.
	f := &File{raw: &os.File{}, role: Database, shared: true, exclusive: true}
	if err := f.lockResultLocked(ErrBusy); err != ErrBusy || f.closing {
		t.Fatal("ordinary contention must remain retryable")
	}
	if err := f.lockResultLocked(nil); err != nil || f.closing {
		t.Fatal("a successful lock result must not poison its owner")
	}
	cause := errors.New("model cleanup result")
	result := errors.Join(ErrBusy, ErrPoisoned, cause)
	if err := f.lockResultLocked(result); err != result || !f.closing || !errors.Is(err, cause) {
		t.Fatal("uncertain lock ownership was not latched intact")
	}
	if err := f.LockDB(Exclusive); !errors.Is(err, ErrPoisoned) {
		t.Fatalf("cached exclusive ownership was reused after poison: %v", err)
	}
	if err := f.lockResultLocked(nil); err != nil || !f.closing {
		t.Fatal("later result cleared poison")
	}
	if !f.shared || !f.exclusive || f.raw == nil {
		t.Fatal("poison discarded resource ownership needed for close")
	}
}

func TestOwnershipReviewReplacementRejectsIneligibleRoles(t *testing.T) {
	// No case can reach pathname validation or native replacement: no object
	// has a descriptor, and at least one ownership precondition is invalid.
	newFiles := func() (*File, *File, *File) {
		return &File{role: Database, exclusive: true},
			&File{role: WAL, replaceable: true},
			&File{role: WAL, replaceable: true}
	}
	cases := map[string]func(*File, *File, *File){
		"shared database":           func(db, _, _ *File) { db.exclusive = false },
		"wrong database role":       func(db, _, _ *File) { db.role = Journal },
		"shared WAL lease":          func(_, old, _ *File) { old.replaceable = false },
		"non-replaceable candidate": func(_, _, candidate *File) { candidate.replaceable = false },
		"wrong old role":            func(_, old, _ *File) { old.role = Database },
		"wrong candidate role":      func(_, _, candidate *File) { candidate.role = SHM },
	}
	for name, alter := range cases {
		t.Run(name, func(t *testing.T) {
			db, old, candidate := newFiles()
			alter(db, old, candidate)
			if err := ReplaceWAL(db, old, candidate); !errors.Is(err, ErrLock) {
				t.Fatalf("ineligible replacement accepted: %v", err)
			}
		})
	}
	db, old, candidate := newFiles()
	for _, args := range [][3]*File{{db, db, candidate}, {db, old, old}, {db, old, db}} {
		if err := ReplaceWAL(args[0], args[1], args[2]); !errors.Is(err, ErrLock) {
			t.Fatalf("aliased replacement ownership accepted: %v", err)
		}
	}
}
