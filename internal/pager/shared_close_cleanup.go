package pager

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
	"github.com/ms-xiaosnow/gosqlite/internal/walindex"
)

// ErrCloseCleanupPending is a historical marker on an immutable cleanup token.
// The current retry result, not an old token, reports current retirement status.
var ErrCloseCleanupPending = errors.New("shared WAL close cleanup pending")

// Unlike the separate 64 failed-open groups, these reservations last throughout
// successful sessions' live and pending-close lifetimes. Admission is bounded
// before acquisition; database/sql discarding a Close error cannot lose a group.
const maxSharedCloseRetirements = 1024
const maxCloseCleanupErrorNodes = 4096

var sharedCloseRetirements = sharedCloseRetirementRegistry{limit: maxSharedCloseRetirements}

type sharedCloseRetirementRegistry struct {
	mu     sync.Mutex
	limit  int
	groups map[*sharedCloseRetirement]struct{}
}

func (r *sharedCloseRetirementRegistry) reserve() (*sharedCloseRetirement, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.groups) >= r.limit {
		return nil, fmt.Errorf("%w: shared WAL live/close cleanup capacity exhausted", ErrLimit)
	}
	if r.groups == nil {
		r.groups = make(map[*sharedCloseRetirement]struct{})
	}
	g := &sharedCloseRetirement{registry: r}
	r.groups[g] = struct{}{}
	return g, nil
}

// SharedCloseAdmission permits the public API to reserve before its own path
// metadata probes. It is consumed once by OpenSharedSessionAdmitted. Cancel
// releases only an unused reservation; it never closes a live session.
type SharedCloseAdmission struct {
	mu    sync.Mutex
	group *sharedCloseRetirement
}

func ReserveSharedClose() (*SharedCloseAdmission, error) {
	g, err := sharedCloseRetirements.reserve()
	if err != nil {
		return nil, err
	}
	return &SharedCloseAdmission{group: g}, nil
}
func (a *SharedCloseAdmission) take() (*sharedCloseRetirement, error) {
	if a == nil {
		return nil, ErrMisuse
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	g := a.group
	if g == nil {
		return nil, ErrMisuse
	}
	a.group = nil
	return g, nil
}
func (a *SharedCloseAdmission) Cancel() {
	if a == nil {
		return
	}
	a.mu.Lock()
	g := a.group
	a.group = nil
	a.mu.Unlock()
	if g != nil {
		g.abandon()
	}
}

// Only managed resources implement this boundary. No raw descriptor, native
// close function, session, DB, options, Rows, or transaction is stored here.
type sharedCloseResource interface {
	Close() error
	RetirementComplete() bool
}
type sharedCloseFile struct{ file *vfs.File }

func (o sharedCloseFile) Close() error             { return closeNativeFile(o.file) }
func (o sharedCloseFile) RetirementComplete() bool { return o.file.RetirementComplete() }

type sharedCloseIndex struct{ index *walindex.Index }

func (o sharedCloseIndex) Close() error             { return sharedIndexError(o.index.Close()) }
func (o sharedCloseIndex) RetirementComplete() bool { return o.index.RetirementComplete() }

type sharedCloseNested struct{ owner vfs.CleanupOwner }

func (o sharedCloseNested) Close() error             { return nativeFileError(o.owner.Close()) }
func (o sharedCloseNested) RetirementComplete() bool { return vfs.CleanupComplete(o.owner) }

type sharedCloseRetirement struct {
	mu               sync.Mutex
	registry         *sharedCloseRetirementRegistry
	index, wal, main sharedCloseResource
	nested           []sharedCloseResource
	started, done    bool
	cause            error // first close cause + first cleanup failure only, never retry joins
	pending          atomic.Pointer[sharedCloseCleanupError]
}

func (g *sharedCloseRetirement) releaseLocked() {
	g.index, g.wal, g.main = nil, nil, nil
	g.nested, g.cause = nil, nil
	g.done = true
	g.pending.Store(nil)
	g.registry.mu.Lock()
	delete(g.registry.groups, g)
	g.registry.mu.Unlock()
}
func (g *sharedCloseRetirement) abandon() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.releaseLocked() // only before successful ownership transfer
}

// retireLocked must not infer completion from an error, including nil. A
// consumed Linux close can report an error, and terminal uncertainty remains
// pending even when a broken owner reports nil. Retired diagnostics are returned
// without a pending marker once every dependency has actually retired.
func (g *sharedCloseRetirement) retireLocked() error {
	var errs []error
	step := func(owner sharedCloseResource) bool {
		if owner.RetirementComplete() {
			return true
		}
		err := owner.Close()
		if err != nil {
			errs = append(errs, err)
		}
		if owner.RetirementComplete() {
			return true
		}
		if err == nil {
			errs = append(errs, ErrPoisoned)
		}
		return false
	}
	for len(g.nested) != 0 {
		if !step(g.nested[0]) {
			return errors.Join(errs...)
		}
		g.nested[0] = nil
		g.nested = g.nested[1:]
	}
	for _, owner := range []*sharedCloseResource{&g.index, &g.wal, &g.main} {
		if *owner == nil {
			continue
		}
		if !step(*owner) {
			return errors.Join(errs...)
		}
		*owner = nil
	}
	g.releaseLocked()
	return errors.Join(errs...)
}

func (g *sharedCloseRetirement) begin(cause error) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.started || g.done {
		return g.retryLocked()
	}
	g.started = true
	for _, owner := range vfs.CleanupOwners(cause) {
		g.nested = append(g.nested, sharedCloseNested{owner})
	}
	err := g.retireLocked()
	if g.done {
		return errors.Join(cause, err)
	}
	g.cause = errors.Join(cause, err)
	return g.pendingLocked(g.cause)
}
func (g *sharedCloseRetirement) pendingLocked(cause error) error {
	e := &sharedCloseCleanupError{group: g, cause: cause}
	g.pending.Store(e)
	return e
}
func (g *sharedCloseRetirement) retryLocked() error {
	if !g.started || g.done {
		return nil
	}
	err := g.retireLocked()
	if g.done {
		return err
	}
	return g.pendingLocked(errors.Join(g.cause, err))
}
func (g *sharedCloseRetirement) retry() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.retryLocked()
}

type sharedCloseCleanupError struct {
	group *sharedCloseRetirement
	cause error
}

func (e *sharedCloseCleanupError) Error() string {
	return fmt.Sprintf("%v: %v", ErrCloseCleanupPending, e.cause)
}
func (e *sharedCloseCleanupError) Unwrap() []error {
	return []error{ErrCloseCleanupPending, ErrPoisoned, e.cause}
}

// snapshot never takes a group lock, so a blocked retirement cannot block
// admission or discovery of other pending groups. Returned tokens are immutable;
// replacing or clearing the caller's slice cannot change the registry.
func (r *sharedCloseRetirementRegistry) snapshot() []error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []error
	for g := range r.groups {
		if e := g.pending.Load(); e != nil {
			out = append(out, e)
		}
	}
	return out
}
func PendingCloseCleanups() []error { return sharedCloseRetirements.snapshot() }

func collectCloseCleanupGroups(input error) ([]*sharedCloseRetirement, error) {
	stack := []error{input}
	seen := make(map[*sharedCloseRetirement]bool)
	var groups []*sharedCloseRetirement
	count := 0
	limit := func() ([]*sharedCloseRetirement, error) {
		return nil, fmt.Errorf("%w: close cleanup error tree exceeds %d nodes; retain tokens and retry smaller trees", ErrLimit, maxCloseCleanupErrorNodes)
	}
	for len(stack) != 0 {
		if count >= maxCloseCleanupErrorNodes {
			return limit()
		}
		err := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		count++
		switch e := err.(type) {
		case *sharedCloseCleanupError:
			if e != nil && !seen[e.group] {
				seen[e.group] = true
				groups = append(groups, e.group)
			}
		case interface{ Unwrap() []error }:
			children := e.Unwrap()
			if len(children) > maxCloseCleanupErrorNodes-count-len(stack) {
				return limit()
			}
			for i := len(children) - 1; i >= 0; i-- {
				stack = append(stack, children[i])
			}
		case interface{ Unwrap() error }:
			stack = append(stack, e.Unwrap())
		}
	}
	return groups, nil
}

func RetryCloseCleanup(input error) error {
	groups, err := collectCloseCleanupGroups(input)
	if err != nil {
		return err
	}
	var errs []error
	for _, g := range groups {
		if err := g.retry(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// OpenSharedSession reserves even when called without the public sqlite layer.
func OpenSharedSession(ctx context.Context, path string, opts Options) (*SharedSession, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	admission, err := ReserveSharedClose()
	if err != nil {
		return nil, err
	}
	return OpenSharedSessionAdmitted(ctx, path, opts, admission)
}
