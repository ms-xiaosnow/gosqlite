package vfs

import (
	"errors"
	"fmt"
	"sync"
)

// metadataOwners is the bounded ownership gate for Windows metadata-only
// handles, including retained parents. Callbacks keep its laws testable using
// plain integers and memory only; production callbacks are private Win32 calls.
// The gate covers reservation, native creation, publication and retirement.
// An uncertain close leaves a tombstone forever, never a recyclable HANDLE.
type metadataOwners struct {
	mu       sync.Mutex
	limit    int
	live     map[*metadataOwner]struct{}
	poisoned error
}

type metadataOwner struct {
	handle  uintptr
	unknown bool
	retired bool
}

func (r *metadataOwners) check() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.poisoned
}

func (r *metadataOwners) open(create func() (uintptr, error)) (*metadataOwner, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.poisoned != nil {
		return nil, r.poisoned
	}
	if len(r.live) >= r.limit {
		return nil, fmt.Errorf("%w: metadata ownership capacity reached", ErrUnsupported)
	}
	if r.live == nil {
		r.live = make(map[*metadataOwner]struct{})
	}
	o := &metadataOwner{}
	r.live[o] = struct{}{} // Reserve before opening, including failed-open cleanup.
	h, err := create()
	if err != nil {
		delete(r.live, o) // CreateFile failure supplies no acquired resource.
		return nil, err
	}
	o.handle = h
	return o, nil
}

func (r *metadataOwners) close(o *metadataOwner, closeHandle func(uintptr) error) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if o == nil || o.retired {
		return nil
	}
	if o.unknown {
		return errors.Join(ErrPoisoned, r.poisoned) // Never close an indeterminate, possibly reused value.
	}
	if _, ok := r.live[o]; !ok {
		return ErrPoisoned
	}
	if err := closeHandle(o.handle); err != nil {
		o.unknown = true
		r.poisoned = terminalCleanup(errors.Join(ErrPoisoned, r.poisoned, err))
		return r.poisoned
	}
	o.handle = 0
	o.retired = true
	delete(r.live, o)
	return nil
}

// observeMetadata is the actual probe composition: an observation is not
// published until the owned probe retires successfully and the gate is usable.
// Callback-only laws cover the failure branches without native fault tests.
func observeMetadata[T any](r *metadataOwners, o *metadataOwner, inspect func(uintptr) (T, error), closeHandle func(uintptr) error) (T, error) {
	value, inspectErr := inspect(o.handle)
	if err := errors.Join(inspectErr, r.close(o, closeHandle), r.check()); err != nil {
		var zero T
		return zero, err
	}
	return value, nil
}

// rejectMetadata retires an acquired parent when the subsequent open or
// validation fails. Preserve the original error directly if cleanup succeeds:
// os.IsNotExist/IsExist do not unwrap a generic errors.Join node.
func rejectMetadata(r *metadataOwners, o *metadataOwner, cause error, closeHandle func(uintptr) error) error {
	cleanup := errors.Join(r.close(o, closeHandle), r.check())
	if cleanup == nil {
		return cause
	}
	return errors.Join(cause, cleanup)
}
