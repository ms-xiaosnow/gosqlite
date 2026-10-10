package vfs

import (
	"os"
	"syscall"
)

// ordinaryPathAbsence accepts only one ordinary NotExist cause. It is called
// at a no-handle failure boundary; inspection/retirement errors never reach it.
// Limit both work and scratch storage without errors.Is on an arbitrary graph.
func ordinaryPathAbsence(cause error) bool {
	const limit = 4096
	stack := []error{cause}
	leaves, count := 0, 0
	for len(stack) != 0 {
		if count >= limit {
			return false
		}
		e := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		count++
		if e == nil {
			continue
		}
		missing := e == os.ErrNotExist
		if code, ok := e.(syscall.Errno); ok {
			missing = code.Is(os.ErrNotExist)
		}
		if missing {
			leaves++
			if leaves > 1 {
				return false
			}
			continue
		}
		switch x := e.(type) {
		case interface{ Unwrap() []error }:
			children := x.Unwrap()
			if len(children) == 0 || len(children) > limit-count-len(stack) {
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

// observeMetadataForCreate preserves whether the initial metadata open failed
// before acquiring a handle. A later inspection error cannot become creation
// authority merely because cleanup completed or it happens to match NotExist.
// Callbacks are immediate, never retained, and memory laws use no descriptors.
func observeMetadataForCreate[T any](r *metadataOwners, o *metadataOwner, openErr error, inspect func(uintptr) (T, error), closeHandle func(uintptr) error) (T, bool, error) {
	if openErr != nil {
		var zero T
		return zero, o == nil && ordinaryPathAbsence(openErr), openErr
	}
	value, err := observeMetadata(r, o, inspect, closeHandle)
	return value, false, err
}
