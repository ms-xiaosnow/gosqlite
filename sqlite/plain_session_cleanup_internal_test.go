package sqlite

import (
	"errors"
	"github.com/ms-xiaosnow/gosqlite/internal/pager"
	"testing"
)

// Empty admissions and invalid input only. Capacity must reject before any
// canonicalization/native metadata, so this is not a native failure probe.
func TestPlainSessionCleanupMemoryPublicAdmissionBeforePath(t *testing.T) {
	var admissions []*pager.ExclusiveOpenAdmission
	defer func() {
		for _, a := range admissions {
			_, _ = a.Finish(nil)
		}
	}()
	for i := 0; i < 1024; i++ {
		a, e := pager.ReserveExclusiveOpen()
		if e != nil {
			t.Fatal(i, e)
		}
		admissions = append(admissions, a)
	}
	db, e := Open("\x00", Options{WALExclusive: true})
	var detail *Error
	if db != nil || !errors.Is(e, ErrMemoryLimit) || !errors.As(e, &detail) || detail.Code != CodeNoMem || len(PendingPlaintextCleanups()) != 0 {
		t.Fatal("canonicalization preceded plaintext admission", e)
	}
}
