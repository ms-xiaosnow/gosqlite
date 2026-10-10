package pager

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

// ErrPlaintextCleanupPending is an immutable historical ownership token. This
// increment covers rollback Readers, standalone transactions and exclusive WAL sessions.
var ErrPlaintextCleanupPending = errors.New("plaintext cleanup pending")

const maxPlainRetirements = 1024
const maxPlainCleanupErrorNodes = 4096

var plainRetirements = plainRetirementRegistry{limit: maxPlainRetirements}

type plainRetirementRegistry struct {
	mu     sync.Mutex
	limit  int
	groups map[*plainRetirement]struct{}
}

func (r *plainRetirementRegistry) reserve() (*plainRetirement, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.groups) >= r.limit {
		return nil, fmt.Errorf("%w: plaintext live/cleanup capacity exhausted", ErrLimit)
	}
	if r.groups == nil {
		r.groups = make(map[*plainRetirement]struct{})
	}
	g := &plainRetirement{registry: r}
	r.groups[g] = struct{}{}
	return g, nil
}

// No application callback, Reader, Tx, Rows, DB, page buffer or raw handle is
// retained. Status and retirement calls always run outside registry admission.
type plainResource interface {
	Close() error
	RetirementComplete() bool
}

type plainFile struct{ file *vfs.File }

func (o plainFile) Close() error             { return closeNativeFile(o.file) }
func (o plainFile) RetirementComplete() bool { return o.file.RetirementComplete() }

type plainNested struct{ owner vfs.CleanupOwner }

func (o plainNested) Close() error             { return nativeFileError(o.owner.Close()) }
func (o plainNested) RetirementComplete() bool { return vfs.CleanupComplete(o.owner) }

// An unsafe internal error graph cannot establish a retirement order. Retain
// this opaque obligation and every protector, without retrying any native call.
type plainTerminal struct{ cause error }

func (o plainTerminal) Close() error             { return o.cause }
func (o plainTerminal) RetirementComplete() bool { return false }

type plainRetirement struct {
	mu                                    sync.Mutex
	registry                              *plainRetirementRegistry
	main                                  plainResource
	temporary                             plainResource
	wal                                   plainResource
	walReserved, walRequested             bool
	temporaryReserved, temporaryRequested bool
	nested                                []plainResource
	started, done                         bool
	cause                                 error // first operation/cleanup error only, never a chain of retries
	pending                               atomic.Pointer[plainCleanupError]
}

func (g *plainRetirement) releaseLocked() {
	g.main, g.temporary, g.wal, g.nested, g.cause = nil, nil, nil, nil, nil
	g.walReserved, g.walRequested = false, false
	g.temporaryReserved, g.temporaryRequested = false, false
	g.done = true
	g.pending.Store(nil)
	g.registry.mu.Lock()
	delete(g.registry.groups, g)
	g.registry.mu.Unlock()
}

// walkPlainErrors bounds both the node count and traversal storage. Recognized
// tokens are leaves; their owners were already adopted when the token was made.
func walkPlainErrors(input error, visit func(error)) error {
	stack := []error{input}
	count := 0
	limit := func() error {
		return fmt.Errorf("%w: plaintext cleanup error tree exceeds %d nodes; retain tokens and retry smaller trees", ErrLimit, maxPlainCleanupErrorNodes)
	}
	for len(stack) != 0 {
		if count >= maxPlainCleanupErrorNodes {
			return limit()
		}
		e := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		count++
		if visit != nil {
			visit(e)
		}
		switch x := e.(type) {
		case *plainCleanupError:
		case interface{ Unwrap() []error }:
			children := x.Unwrap()
			if len(children) > maxPlainCleanupErrorNodes-count-len(stack) {
				return limit()
			}
			for i := len(children) - 1; i >= 0; i-- {
				stack = append(stack, children[i])
			}
		case interface{ Unwrap() error }:
			stack = append(stack, x.Unwrap())
		}
	}
	return nil
}

func (g *plainRetirement) adoptLocked(cause error) error {
	if cause == nil {
		return nil
	}
	owners, err := vfs.CleanupOwnersBounded(cause, maxPlainCleanupErrorNodes)
	if err != nil {
		// Keep opaque ownership reachable without exposing an unsafe Unwrap tree.
		opaque := &plainUnsafeCause{retained: cause, diagnostic: errors.Join(ErrLimit, err)}
		g.nested = append(g.nested, plainTerminal{opaque})
		return opaque
	}
	for _, owner := range owners {
		g.nested = append(g.nested, plainNested{owner})
	}
	return cause
}

func (g *plainRetirement) pendingLocked(cause error) error {
	e := &plainCleanupError{group: g, cause: cause}
	g.pending.Store(e)
	return e
}

// observe is called before a live Reader can discard an operation's error.
// Once ownership failure poisons that Reader, no further native read is allowed.
// Explicit live retry can retire nested failed probes, but never its main file.
func (g *plainRetirement) observe(cause error) error {
	if cause == nil || g == nil {
		return cause
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.done || g.started || g.cause != nil {
		return cause
	}
	cause = g.adoptLocked(cause)
	if len(g.nested) == 0 {
		return cause
	}
	g.cause = errors.Join(ErrPoisoned, cause)
	return g.pendingLocked(g.cause)
}
func (g *plainRetirement) check() error {
	if g == nil {
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.started || g.done {
		return ErrClosed
	}
	if e := g.pending.Load(); e != nil {
		return e
	}
	return g.cause
}

// RetirementComplete, not Close's diagnostic, controls each stage. A consumed
// close error can advance; nil from an incomplete owner cannot release main.
func (g *plainRetirement) retireLocked() error {
	var errs []error
	step := func(owner plainResource) bool {
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
	if g.temporary != nil && (g.started || g.temporaryRequested) {
		if !step(g.temporary) {
			return errors.Join(errs...)
		}
		g.temporary = nil
		g.temporaryRequested = false
	}
	if g.wal != nil && (g.started || g.walRequested) {
		if !step(g.wal) {
			return errors.Join(errs...)
		}
		g.wal = nil
		g.walRequested = false
	}
	if !g.started {
		g.pending.Store(nil)
		return errors.Join(errs...)
	}
	if g.main != nil && !step(g.main) {
		return errors.Join(errs...)
	}
	g.releaseLocked()
	return errors.Join(errs...)
}
func (g *plainRetirement) begin(cause error) error {
	return g.beginClass(cause, nil, true)
}

func (g *plainRetirement) beginClass(cause, failureClass error, attempt bool) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.started || g.done {
		return g.retryLocked()
	}
	g.started = true
	cause = g.adoptLocked(cause) // operation obligations precede all protector retirement
	original := errors.Join(g.cause, plainDiagnostic(cause))
	if !attempt {
		g.cause = errors.Join(original, ErrPoisoned)
		return g.pendingLocked(g.cause)
	}
	err := g.retireLocked()
	if err != nil {
		err = errors.Join(failureClass, err)
	}
	if g.done {
		return errors.Join(original, err)
	}
	g.cause = plainDiagnostic(errors.Join(original, err))
	return g.pendingLocked(g.cause)
}
func (g *plainRetirement) retryLocked() error {
	if g.done || (!g.started && len(g.nested) == 0 && (!g.temporaryRequested || g.temporary == nil) && (!g.walRequested || g.wal == nil)) {
		return nil
	}
	err := g.retireLocked()
	if g.done || (!g.started && len(g.nested) == 0 && (!g.temporaryRequested || g.temporary == nil) && (!g.walRequested || g.wal == nil)) {
		return err
	}
	return g.pendingLocked(errors.Join(g.cause, err))
}
func (g *plainRetirement) retry() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.retryLocked()
}

type plainCleanupError struct {
	group *plainRetirement
	cause error
}

func (e *plainCleanupError) Error() string {
	return fmt.Sprintf("%v: %v", ErrPlaintextCleanupPending, e.cause)
}
func (e *plainCleanupError) Unwrap() []error {
	return []error{ErrPlaintextCleanupPending, ErrPoisoned, e.cause}
}

func (r *plainRetirementRegistry) snapshot() []error {
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
func PendingPlaintextCleanups() []error { return plainRetirements.snapshot() }
func RetryPlaintextCleanup(input error) error {
	seen := make(map[*plainRetirement]bool)
	var groups []*plainRetirement
	err := walkPlainErrors(input, func(e error) {
		if token, ok := e.(*plainCleanupError); ok && token != nil && !seen[token.group] {
			seen[token.group] = true
			groups = append(groups, token.group)
		}
	})
	if err != nil {
		return err
	} // complete discovery before any retirement effect
	var errs []error
	for _, g := range groups {
		if err := g.retry(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// retained is deliberately not exposed through Unwrap or formatted.
type plainUnsafeCause struct {
	retained   error
	diagnostic error
}

func (e *plainUnsafeCause) Error() string {
	return "plaintext cleanup retained an unsafe internal error graph"
}
func (e *plainUnsafeCause) Unwrap() []error { return []error{ErrPoisoned, e.diagnostic} }
