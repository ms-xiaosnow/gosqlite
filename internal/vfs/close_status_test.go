package vfs

import (
	"errors"
	"testing"
)

func TestCloseStatusMemoryExplicitCompletion(t *testing.T) {
	// Zero File has never acquired a descriptor. The historical error token may
	// still name it, but the explicit owner status conclusively says retired.
	file := &File{}
	cause := errors.New("memory historical consumed close error")
	owner := retainedFileCleanup(file, cause).(*cleanupError)
	if !file.RetirementComplete() || !CleanupComplete(owner) {
		t.Fatal("consumed owner inferred from error")
	}
	terminal := terminalCleanup(errors.Join(ErrPoisoned, cause))
	if CleanupComplete(terminal) {
		t.Fatal("terminal uncertainty treated as retired")
	}
	if e := owner.Close(); e != nil {
		t.Fatal(e)
	}
	if !errors.Is(owner, cause) {
		t.Fatal("historical cause changed")
	}
}

func TestCloseStatusMemoryNestedHistoricalWrappersAreNotLive(t *testing.T) {
	child, parent := &File{}, &File{}
	cause := errors.New("memory retired child diagnostic")
	err := retainedFileCleanup(parent, retainedFileCleanup(child, cause))
	owners := CleanupOwners(err)
	if len(owners) != 2 {
		t.Fatal("nested identities lost")
	}
	for _, owner := range owners {
		if !CleanupComplete(owner) {
			t.Fatal("historical wrapper misclassified retired managed owner")
		}
	}
	if !errors.Is(err, cause) {
		t.Fatal("retired dependency lost historical cause")
	}
	// A separate opaque terminal barrier must still retain both owners behind
	// it, even though these particular File identities have already retired.
	terminal := terminalCleanup(errors.New("memory opaque dependency"))
	owners = CleanupOwners(errors.Join(err, retainedFileCleanup(parent, terminal)))
	if len(owners) != 3 {
		t.Fatal("terminal dependency lost")
	}
	seenTerminal := false
	for _, owner := range owners {
		if owner == terminal {
			seenTerminal = true
			if CleanupComplete(owner) {
				t.Fatal("opaque dependency retired")
			}
		}
		if owner.(*cleanupError).file == parent && !seenTerminal {
			t.Fatal("parent ordered before terminal dependency")
		}
	}
}
