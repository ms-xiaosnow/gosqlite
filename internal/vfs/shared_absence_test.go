package vfs

import (
	"errors"
	"os"
	"testing"
)

// This complements pager's shared-session source laws at the actual metadata
// decision boundary. Callbacks manipulate only inert integers and error values.
func TestSharedAbsenceMemoryMetadataTerminalProvenance(t *testing.T) {
	for _, inspectionAbsent := range []bool{false, true} {
		for _, retirementAbsent := range []bool{false, true} {
			r := metadataOwners{limit: 1}
			o, err := r.open(func() (uintptr, error) { return 37, nil })
			if err != nil {
				t.Fatal(err)
			}
			inspection := errors.New("memory inspection diagnostic")
			retirement := errors.New("memory retirement diagnostic")
			if inspectionAbsent {
				inspection = &os.PathError{Op: "memory inspection", Path: "memory", Err: os.ErrNotExist}
			}
			if retirementAbsent {
				retirement = &os.PathError{Op: "memory retirement", Path: "memory", Err: os.ErrNotExist}
			}
			calls := 0
			value, absent, cause := observeMetadataForCreate(&r, o, nil,
				func(uintptr) (string, error) { return "unpublished", inspection },
				func(uintptr) error { calls++; return retirement })
			if absent || value != "" || calls != 1 || !errors.Is(cause, inspection) || !errors.Is(cause, retirement) || !errors.Is(cause, ErrPoisoned) {
				t.Fatal("post-acquisition error became absence or lost cause", absent, cause)
			}
			owners := CleanupOwners(cause)
			if len(owners) != 1 || CleanupComplete(owners[0]) || !errors.Is(owners[0].Close(), retirement) || calls != 1 || len(r.live) != 1 {
				t.Fatal("metadata terminal discovery lost ownership or repeated raw close")
			}
			// Later acquisition is refused by the same tombstone. Even an
			// ENOENT inside it must never become initial absence authority.
			later, laterErr := r.open(func() (uintptr, error) { t.Fatal("poisoned metadata gate acquired"); return 0, nil })
			_, absent, laterErr = observeMetadataForCreate(&r, later, laterErr,
				func(uintptr) (string, error) { t.Fatal("poisoned metadata gate inspected"); return "", nil },
				func(uintptr) error { t.Fatal("poisoned metadata gate retired"); return nil })
			if absent || !errors.Is(laterErr, retirement) || len(CleanupOwners(laterErr)) != 1 || calls != 1 {
				t.Fatal("retained acquisition diagnostic authorized absence", absent, laterErr)
			}
		}
	}
}
