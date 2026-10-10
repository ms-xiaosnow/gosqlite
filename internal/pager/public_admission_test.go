package pager

import (
	"errors"
	"fmt"
	"testing"
)

// Every owner and exceptional result is descriptor-free. These tests exercise
// the production public Finish paths without invoking any native metadata.
func TestPublicAdmissionMemoryPlainTransferAndEarlyFailure(t *testing.T) {
	a, e := ReservePlainOpen()
	if e != nil {
		t.Fatal(e)
	}
	g := a.group.Load()
	main := &plainMemoryOwner{remaining: 1, cause: errors.New("memory public metadata owner")}
	g.nested = []plainResource{main}
	operation := errors.New("memory initial operation")
	token, handled := a.Finish(operation)
	if !handled || !errors.Is(token, operation) || !errors.Is(token, main.cause) || !errors.Is(token, ErrPlaintextCleanupPending) || len(PendingPlaintextCleanups()) != 1 {
		t.Fatal(token)
	}
	if e = RetryPlaintextCleanup(PendingPlaintextCleanups()[0]); e != nil || !g.done {
		t.Fatal(e)
	}
	a, e = ReservePlainOpen()
	if e != nil {
		t.Fatal(e)
	}
	g, e = claimPlainOpen(a)
	if e != nil {
		t.Fatal(e)
	}
	if got, handled := a.Finish(operation); handled || got != operation {
		t.Fatal("transferred Reader root reclaimed", got)
	}
	if _, e = claimPlainOpen(a); !errors.Is(e, ErrMisuse) {
		t.Fatal("duplicate Reader root transfer", e)
	}
	if e = g.begin(nil); e != nil {
		t.Fatal(e)
	}
}
func TestPublicAdmissionMemorySharedEarlyFailureAndTransfer(t *testing.T) {
	a, e := ReserveSharedClose()
	if e != nil {
		t.Fatal(e)
	}
	g := a.group
	owner := &memoryCloseOwner{remaining: 1, cause: errors.New("memory shared public owner")}
	g.nested = []sharedCloseResource{owner}
	operation := errors.New("memory shared public operation")
	token, handled := a.Finish(operation)
	if !handled || !errors.Is(token, operation) || !errors.Is(token, owner.cause) || !errors.Is(token, ErrCloseCleanupPending) || len(PendingCloseCleanups()) != 1 {
		t.Fatal(token)
	}
	a.Cancel()
	if owner.calls != 1 || g.done {
		t.Fatal("Cancel discarded pending owner")
	}
	if e = RetryCloseCleanup(PendingCloseCleanups()[0]); e != nil || !g.done {
		t.Fatal(e)
	}
	a, e = ReserveSharedClose()
	if e != nil {
		t.Fatal(e)
	}
	g, e = a.take()
	if e != nil {
		t.Fatal(e)
	}
	if got, handled := a.Finish(operation); handled || got != operation {
		t.Fatal("transferred shared root reclaimed", got)
	}
	g.abandon()
}
func TestPublicAdmissionMemoryBoundedSharedFailure(t *testing.T) {
	reg := &sharedCloseRetirementRegistry{limit: 1}
	g, e := reg.reserve()
	if e != nil {
		t.Fatal(e)
	}
	a := &SharedCloseAdmission{group: g}
	cycle := &plainAbsenceCycle{}
	token, handled := a.Finish(cycle)
	if !handled || !errors.Is(token, ErrLimit) || !errors.Is(token, ErrCloseCleanupPending) || len(reg.snapshot()) != 1 {
		t.Fatal("unsafe graph not retained safely")
	}
	_ = fmt.Sprint(token) // must never format the cyclic retained graph
	first := g.cause
	for i := 0; i < 3; i++ {
		token = RetryCloseCleanup(token)
		if g.cause != first {
			t.Fatal("retry history grew")
		}
	}
	if !errors.Is(token, ErrCloseCleanupPending) || g.done {
		t.Fatal("terminal obligation disappeared")
	}
}
func TestPublicAdmissionMemoryOrdinaryEarlyErrorsConsumeNoOwners(t *testing.T) {
	original := errors.New("memory ordinary early setup error")
	a, e := ReservePlainOpen()
	if e != nil {
		t.Fatal(e)
	}
	g := a.group.Load()
	if result, handled := a.Finish(original); !handled || !errors.Is(result, original) || !g.done || errors.Is(result, ErrPlaintextCleanupPending) {
		t.Fatal(result)
	}
	s, e := ReserveSharedClose()
	if e != nil {
		t.Fatal(e)
	}
	sg := s.group
	if result, handled := s.Finish(original); !handled || !errors.Is(result, original) || !sg.done || errors.Is(result, ErrCloseCleanupPending) {
		t.Fatal(result)
	}
}
