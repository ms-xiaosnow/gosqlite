package pager

import (
	"os"
	"syscall"
)

// initialReaderAbsence is issued only for conclusive absence during the first
// path resolution, before the main database has been acquired. Later missing
// paths cannot authorize a new database, even if their ownership was consumed.
type initialReaderAbsence struct{ cause error }

func (e *initialReaderAbsence) Error() string { return e.cause.Error() }
func (e *initialReaderAbsence) Unwrap() error { return e.cause }

func markInitialReaderAbsence(cause error, initialAbsent bool) error {
	if initialAbsent && plainSingleCause(cause, func(e error) bool {
		if e == os.ErrNotExist {
			return true
		}
		code, ok := e.(syscall.Errno)
		return ok && code.Is(os.ErrNotExist)
	}) {
		return &initialReaderAbsence{cause: cause}
	}
	return cause
}

// ReaderProbeAbsent recognizes a single initial-acquisition absence through
// ordinary wrappers and single-cause joins. Any additional diagnostic or pending
// ownership rejects creation. It neither performs cleanup nor inspects paths.
func ReaderProbeAbsent(cause error) bool {
	return plainSingleCause(cause, func(e error) bool {
		marker, ok := e.(*initialReaderAbsence)
		return ok && marker != nil
	})
}

// Bound traversal and scratch storage. Accepted leaves are deliberately opaque:
// only the private provenance marker can authorize create, never errors.Is on an
// arbitrary aggregate carrying NotExist alongside another cause.
func plainSingleCause(cause error, accepted func(error) bool) bool {
	stack := []error{cause}
	leaves, count := 0, 0
	for len(stack) != 0 {
		if count >= maxPlainCleanupErrorNodes {
			return false
		}
		e := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		count++
		if e == nil {
			continue
		}
		if accepted(e) {
			leaves++
			if leaves > 1 {
				return false
			}
			continue
		}
		switch x := e.(type) {
		case *plainCleanupError, *initialReaderAbsence:
			return false
		case interface{ Unwrap() []error }:
			children := x.Unwrap()
			if len(children) == 0 || len(children) > maxPlainCleanupErrorNodes-count-len(stack) {
				return false
			}
			stack = append(stack, children...)
		case interface{ Unwrap() error }:
			child := x.Unwrap()
			if child == nil {
				return false
			}
			stack = append(stack, child)
		default:
			return false
		}
	}
	return leaves == 1
}
