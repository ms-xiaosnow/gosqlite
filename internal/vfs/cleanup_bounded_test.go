package vfs

import (
	"errors"
	"fmt"
	"testing"
)

type boundedCleanupCycle struct{}

func (boundedCleanupCycle) Error() string   { return "memory cycle" }
func (e boundedCleanupCycle) Unwrap() error { return e }

type boundedChangingWrapper struct {
	calls int
	first error
}

func (e *boundedChangingWrapper) Error() string { return "memory stateful wrapper" }
func (e *boundedChangingWrapper) Unwrap() error {
	e.calls++
	if e.calls == 1 {
		return e.first
	}
	return boundedCleanupCycle{}
}
func TestCleanupOwnersBoundedMemorySinglePassAndLimits(t *testing.T) {
	file := &File{}
	child := retainedFileCleanup(file, errors.New("memory cause"))
	wrapper := &boundedChangingWrapper{first: child}
	owners, e := CleanupOwnersBounded(wrapper, 4096)
	if e != nil || len(owners) != 1 || wrapper.calls != 1 || !CleanupComplete(owners[0]) {
		t.Fatal(owners, e, wrapper.calls)
	}
	for _, input := range []error{boundedCleanupCycle{}, errors.Join(child, boundedCleanupCycle{})} {
		if owners, e = CleanupOwnersBounded(input, 4096); !errors.Is(e, ErrCleanupLimit) || len(owners) != 0 {
			t.Fatal(e)
		}
	}
	var chain error = errors.New("leaf")
	for i := 1; i < 4096; i++ {
		chain = fmt.Errorf("wrap: %w", chain)
	}
	if _, e = CleanupOwnersBounded(chain, 4096); e != nil {
		t.Fatal(e)
	}
	if _, e = CleanupOwnersBounded(fmt.Errorf("extra: %w", chain), 4096); !errors.Is(e, ErrCleanupLimit) {
		t.Fatal(e)
	}
}
func TestCleanupOwnersBoundedMemoryDependencies(t *testing.T) {
	parent, one, two := &File{}, &File{}, &File{}
	first := retainedFileCleanup(parent, retainedFileCleanup(one, errors.New("one")))
	second := retainedFileCleanup(parent, retainedFileCleanup(two, errors.New("two")))
	owners, e := CleanupOwnersBounded(errors.Join(first, second), 4096)
	if e != nil || len(owners) != 3 || owners[0].(*cleanupError).file != one || owners[1].(*cleanupError).file != two || owners[2].(*cleanupError).file != parent {
		t.Fatal(owners, e)
	}
	cycle := errors.Join(retainedFileCleanup(parent, retainedFileCleanup(one, errors.New("forward"))), retainedFileCleanup(one, retainedFileCleanup(parent, errors.New("back"))))
	owners, e = CleanupOwnersBounded(cycle, 4096)
	if e != nil || len(owners) != 3 || CleanupComplete(owners[0]) || !errors.Is(owners[0].Close(), ErrPoisoned) {
		t.Fatal(owners, e)
	}
}
