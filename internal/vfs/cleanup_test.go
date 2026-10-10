package vfs

import (
	"errors"
	"fmt"
	"testing"
)

// Empty File owners below contain no raw descriptor and never reach a backend.
// They model already-consumed managed resources, not native close failures.
func TestCleanupOwnersMemoryNestedManagedAndTerminal(t *testing.T) {
	first, second := &File{}, &File{}
	original := errors.New("memory nested constructor rejection")
	one := retainedFileCleanup(first, original)
	two := retainedFileCleanup(second, one)
	owners := CleanupOwners(fmt.Errorf("index open: %w", errors.Join(one, two, one)))
	if len(owners) != 2 {
		t.Fatalf("did not adopt exactly two managed owners: %d", len(owners))
	}
	if owners[0].(*cleanupError).file != first || owners[1].(*cleanupError).file != second {
		t.Fatal("nested owner was not ordered before its parent")
	}
	for _, owner := range owners {
		if err := owner.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if !errors.Is(two, original) {
		t.Fatal("cause lost through managed ownership error")
	}
	terminalCause := errors.Join(ErrPoisoned, original)
	terminal := terminalCleanup(terminalCause)
	all := CleanupOwners(errors.Join(terminal, retainedFileCleanup(first, terminal)))
	if len(all) != 2 || all[0] != terminal {
		t.Fatal("terminal nested obligation missing, duplicated or out of order")
	}
	for i := 0; i < 3; i++ {
		if err := all[0].Close(); err != terminalCause {
			t.Fatal("terminal obligation invented a cleanup route", err)
		}
	}
}

func TestCleanupOwnersMemoryRepeatedOwnerDistinctNestedCauses(t *testing.T) {
	parent, first, second := &File{}, &File{}, &File{}
	one := retainedFileCleanup(first, errors.New("first nested owner"))
	two := retainedFileCleanup(second, errors.New("second nested owner"))
	err := errors.Join(retainedFileCleanup(parent, one), fmt.Errorf("wrapper: %w", retainedFileCleanup(parent, two)))
	owners := CleanupOwners(err)
	if len(owners) != 3 || owners[0].(*cleanupError).file != first || owners[1].(*cleanupError).file != second || owners[2].(*cleanupError).file != parent {
		t.Fatalf("repeated File hid or prematurely retired a child: %v", owners)
	}
	// The repeated child can itself gain another dependency later in the tree.
	leaf := &File{}
	err = errors.Join(err, retainedFileCleanup(first, retainedFileCleanup(leaf, errors.New("leaf"))))
	owners = CleanupOwners(err)
	want := []*File{leaf, first, second, parent}
	if len(owners) != len(want) {
		t.Fatal("missing transitive owner")
	}
	for i, o := range owners {
		if o.(*cleanupError).file != want[i] {
			t.Fatal("transitive child retired after a parent")
		}
	}
}

func TestCleanupOwnersMemoryConflictingDependenciesFailClosed(t *testing.T) {
	first, second := &File{}, &File{}
	err := errors.Join(retainedFileCleanup(first, retainedFileCleanup(second, errors.New("forward"))), retainedFileCleanup(second, retainedFileCleanup(first, errors.New("reverse"))))
	owners := CleanupOwners(err)
	if len(owners) != 3 || !errors.Is(owners[0].Close(), ErrPoisoned) {
		t.Fatal("inconsistent ownership graph was allowed to retire")
	}
}

func TestCleanupOwnersMemoryPreFilePoisonIsOpaque(t *testing.T) {
	cause := errors.New("memory pre-File retirement uncertainty")
	original := errors.Join(ErrPoisoned, cause)
	err := unmanagedOpenError(original)
	owners := CleanupOwners(err)
	if len(owners) != 1 || !errors.Is(err, cause) || !errors.Is(err, ErrPoisoned) {
		t.Fatal("pre-File ownership disappeared", err)
	}
	if got := owners[0].Close(); got != original {
		t.Fatal("opaque ownership invented a native retry", got)
	}
	if again := unmanagedOpenError(err); again != err {
		t.Fatal("precise lower obligation was replaced or duplicated")
	}
	ordinary := errors.New("memory open acquired nothing")
	if unmanagedOpenError(ordinary) != ordinary || len(CleanupOwners(ordinary)) != 0 || len(CleanupOwners(nil)) != 0 {
		t.Fatal("ordinary rejection invented ownership")
	}
}

func TestCleanupOwnersMemoryMetadataTerminalComposes(t *testing.T) {
	r := metadataOwners{limit: 1}
	owner, err := r.open(func() (uintptr, error) { return 23, nil })
	if err != nil {
		t.Fatal(err)
	}
	cause := errors.New("memory metadata close unknown")
	calls := 0
	err = r.close(owner, func(uintptr) error { calls++; return cause })
	owners := CleanupOwners(err)
	if len(owners) != 1 || calls != 1 || !errors.Is(err, cause) {
		t.Fatal("metadata terminal not adopted", err)
	}
	for i := 0; i < 3; i++ {
		if err := owners[0].Close(); !errors.Is(err, cause) || !errors.Is(err, ErrPoisoned) {
			t.Fatal(err)
		}
	}
	if calls != 1 || !owner.unknown || len(r.live) != 1 {
		t.Fatal("terminal retry touched a numeric handle or discarded its record")
	}
	if len(CleanupOwners(errors.Join(err, r.check()))) != 1 {
		t.Fatal("prior global poison duplicated its ownership marker")
	}
}
