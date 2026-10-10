package sqlite

import (
	"errors"
	"github.com/ms-xiaosnow/gosqlite/internal/pager"
	"testing"
)

// No native path lookup can occur: empty admissions fill the bound first.
func TestPublicAdmissionMemoryRollbackBeforeCanonicalization(t *testing.T) {
	var admissions []*pager.PlainOpenAdmission
	defer func() {
		for _, a := range admissions {
			_, _ = a.Finish(nil)
		}
	}()
	for i := 0; i < 1024; i++ {
		a, e := pager.ReservePlainOpen()
		if e != nil {
			t.Fatal(i, e)
		}
		admissions = append(admissions, a)
	}
	for _, options := range []Options{{}, {NoCreate: true}, {ReadOnly: true, NoCreate: true}} {
		db, e := Open("\x00", options)
		var detail *Error
		if db != nil || !errors.Is(e, ErrMemoryLimit) || !errors.As(e, &detail) || detail.Code != CodeNoMem {
			t.Fatal("public metadata preceded admission", e)
		}
	}
	if len(PendingPlaintextCleanups()) != 0 {
		t.Fatal("empty admissions published as pending")
	}
}
