package encstore

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/ms-xiaosnow/gosqlite/encryption"
)

// The registry bounds both successful live owners and abandoned cleanup. A slot
// is reserved before acquisition, never after a resource has been obtained.
const maxEncryptedScopes = 1024
const maxEncryptedLeases = 16384
const maxEncryptedCleanupNodes = 4096

var ErrCleanupPending = errors.New("encrypted cleanup pending")
var encryptedCleanups = cleanupRegistry{scopeLimit: maxEncryptedScopes, leaseLimit: maxEncryptedLeases}

type cleanupResource interface {
	closeNative() error
	RetirementComplete() bool
}
type managedOwnership struct {
	owner *cleanupGroup
	entry *cleanupEntry
}
type cleanupRole uint8

const (
	cleanupChild cleanupRole = iota
	cleanupLifecycle
	cleanupDirectory
)

type cleanupEntry struct {
	resource  cleanupResource
	role      cleanupRole
	requested bool
	completed atomic.Bool // explicit native callback proof, never an admission-time status call
}
type cleanupRegistry struct {
	mu                             sync.Mutex
	scopeLimit, leaseLimit, leases int
	groups                         map[*cleanupGroup]struct{}
}
type cleanupGroup struct {
	mu       sync.Mutex
	registry *cleanupRegistry
	files    map[cleanupResource]*cleanupEntry
	// Dependency edges are changed only under mu and registry.mu. No registry
	// mutex is held while a managed resource or another group is operated on.
	dependencies     map[*cleanupGroup]struct{}
	terminal         bool
	closing, done    bool
	cause            error // first failure only; retries never append to retained history
	pending          atomic.Pointer[cleanupError]
	reap             atomic.Bool
	callbackCause    atomic.Pointer[cleanupCallback]
	callbackTerminal atomic.Bool
}

type cleanupCallback struct{ cause error }

func (g *cleanupGroup) syncCallbacksLocked() {
	if g.callbackTerminal.Swap(false) {
		g.terminal = true
	}
	if c := g.callbackCause.Swap(nil); c != nil {
		err := g.rememberLocked(c.cause)
		g.failureLocked(err)
	}
}
func (r *cleanupRegistry) reserve() (*cleanupGroup, error) {
	r.reapCompleted()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.groups) >= r.scopeLimit {
		return nil, fmt.Errorf("%w: encrypted ownership scope capacity", encryption.ErrLimit)
	}
	if r.groups == nil {
		r.groups = make(map[*cleanupGroup]struct{})
	}
	g := &cleanupGroup{registry: r, files: make(map[cleanupResource]*cleanupEntry), dependencies: make(map[*cleanupGroup]struct{})}
	r.groups[g] = struct{}{}
	return g, nil
}

// Reaping consumes already-published completion proofs only. It never calls
// resource status, native cleanup, or error callbacks. TryLock also prevents
// admission waiting for a group undergoing native retirement.
func (r *cleanupRegistry) reapCompleted() {
	r.mu.Lock()
	var candidates []*cleanupGroup
	for g := range r.groups {
		if g.reap.Load() {
			candidates = append(candidates, g)
		}
	}
	r.mu.Unlock()
	for _, g := range candidates {
		if !g.mu.TryLock() {
			continue
		}
		if g.callbackTerminal.Load() || g.callbackCause.Load() != nil {
			g.mu.Unlock()
			continue
		}
		g.reap.Store(false)
		for resource, entry := range g.files {
			if entry.completed.Load() {
				g.dropLocked(resource)
			}
		}
		g.releaseLocked()
		g.mu.Unlock()
	}
}
func (g *cleanupGroup) pruneLocked() {
	for resource := range g.files {
		if resource.RetirementComplete() {
			g.dropLocked(resource)
		}
	}
}
func (r *cleanupRegistry) reserveLease() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.leases >= r.leaseLimit {
		return fmt.Errorf("%w: encrypted managed lease capacity", encryption.ErrLimit)
	}
	r.leases++
	return nil
}
func (r *cleanupRegistry) releaseLease() { r.mu.Lock(); r.leases--; r.mu.Unlock() }
func (g *cleanupGroup) releaseLocked() {
	if len(g.files) != 0 || len(g.dependencies) != 0 || g.terminal {
		return
	}
	g.done = true
	g.cause = nil
	g.pending.Store(nil)
	g.registry.mu.Lock()
	delete(g.registry.groups, g)
	g.registry.mu.Unlock()
}
func (g *cleanupGroup) addLocked(r cleanupResource, role cleanupRole) {
	e := &cleanupEntry{resource: r, role: role}
	g.files[r] = e
	if f, ok := r.(*managedFile); ok {
		f.entry = e
	}
}
func (g *cleanupGroup) dropLocked(r cleanupResource) {
	delete(g.files, r)
	g.registry.releaseLease()
}
func (g *cleanupGroup) failureLocked(err error) {
	if err != nil && g.cause == nil {
		g.cause = errors.Join(ErrPoisoned, cleanupDiagnostic(err))
	}
}
func (g *cleanupGroup) tokenLocked(err error) error {
	if g.done {
		return err
	}
	g.failureLocked(err)
	e := &cleanupError{group: g, cause: errors.Join(g.cause, cleanupDiagnostic(err))}
	g.pending.Store(e)
	return e
}
func (g *cleanupGroup) stepLocked(e *cleanupEntry) error {
	if g.terminal || len(g.dependencies) != 0 {
		return errors.Join(ErrPoisoned, g.cause)
	}
	if e.resource.RetirementComplete() {
		g.dropLocked(e.resource)
		return nil
	}
	err := g.rememberLocked(e.resource.closeNative())
	g.syncCallbacksLocked()
	if e.resource.RetirementComplete() {
		g.dropLocked(e.resource)
		return err
	}
	if err == nil {
		err = ErrPoisoned
	}
	return err
}

// A lower native constructor can own an indeterminate descriptor before it can
// supply a managed lease. Its native registry retains the resource. This marker
// gives the enclosing pre-reserved scope a terminal barrier, never a raw retry.
type nativeOwnershipError struct{ cause error }

func (e *nativeOwnershipError) Error() string { return e.cause.Error() }
func (e *nativeOwnershipError) Unwrap() error { return e.cause }
func nativeOwnershipPending(err error) error {
	return &nativeOwnershipError{cause: errors.Join(ErrPoisoned, err)}
}

// Adopted tokens become diagnostics inside the owning scope. Their immutable
// historical marker must not make a newly completed result appear pending.
// Is/As preserve original causes; traversal does not rediscover adopted owners.
type cleanupDiagnosticError struct{ cause error }

func (e *cleanupDiagnosticError) Error() string { return e.cause.Error() }
func (e *cleanupDiagnosticError) Is(target error) bool {
	return target != ErrCleanupPending && errors.Is(e.cause, target)
}
func (e *cleanupDiagnosticError) As(target any) bool { return errors.As(e.cause, target) }
func cleanupDiagnostic(err error) error {
	if err == nil {
		return nil
	}
	return &cleanupDiagnosticError{cause: err}
}

type cleanupError struct {
	group *cleanupGroup
	cause error
}

func (e *cleanupError) Error() string   { return fmt.Sprintf("%v: %v", ErrCleanupPending, e.cause) }
func (e *cleanupError) Unwrap() []error { return []error{ErrCleanupPending, ErrPoisoned, e.cause} }

// Discovery has no cleanup effects, handles every joined token, and refuses
// excessive/cyclic caller error trees without wrapping the unsafe tree again.
func cleanupGroups(input error) ([]*cleanupGroup, error) {
	stack := []error{input}
	seen := make(map[*cleanupGroup]bool)
	var out []*cleanupGroup
	count := 0
	limit := func() ([]*cleanupGroup, error) {
		return nil, fmt.Errorf("%w: encrypted cleanup error tree exceeds %d nodes; retain tokens and retry smaller trees", encryption.ErrLimit, maxEncryptedCleanupNodes)
	}
	for len(stack) != 0 {
		if count >= maxEncryptedCleanupNodes {
			return limit()
		}
		count++
		e := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		switch e := e.(type) {
		case *cleanupError:
			if e != nil && !seen[e.group] {
				seen[e.group] = true
				out = append(out, e.group)
			}
		case interface{ Unwrap() []error }:
			children := e.Unwrap()
			if len(children) > maxEncryptedCleanupNodes-count-len(stack) {
				return limit()
			}
			for i := len(children) - 1; i >= 0; i-- {
				stack = append(stack, children[i])
			}
		case interface{ Unwrap() error }:
			stack = append(stack, e.Unwrap())
		}
	}
	return out, nil
}

// rememberLocked preserves dependency ownership before enclosing protection is
// eligible to retire. The registry gate makes cycle checking and edge insertion
// atomic; a cycle becomes a terminal barrier and never starts recursive cleanup.
func (g *cleanupGroup) rememberLocked(err error) error {
	groups, e := cleanupGroups(err)
	if e != nil {
		g.terminal = true
		g.failureLocked(e)
		return e
	}
	g.registry.mu.Lock()
	defer g.registry.mu.Unlock()
	for _, other := range groups {
		if other == g {
			continue
		}
		stack := []*cleanupGroup{other}
		seen := map[*cleanupGroup]bool{}
		cyclic := false
		for len(stack) > 0 {
			p := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if p == g {
				cyclic = true
				break
			}
			if seen[p] {
				continue
			}
			seen[p] = true
			for dep := range p.dependencies {
				stack = append(stack, dep)
			}
		}
		if cyclic {
			g.terminal = true
			g.failureLocked(errors.New("encrypted cleanup dependency cycle"))
		}
		g.dependencies[other] = struct{}{}
		g.failureLocked(err)
	}
	return err
}
func (g *cleanupGroup) remember(err error) error {
	if err == nil {
		return nil
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.rememberLocked(err)
}
func (g *cleanupGroup) protect(r cleanupResource) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if e := g.files[r]; e != nil {
		e.role = cleanupLifecycle
	}
}
func (g *cleanupGroup) health() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.syncCallbacksLocked()
	if g.closing || g.done {
		return encryption.ErrClosed
	}
	if pending := g.pending.Load(); pending != nil {
		return pending
	}
	if g.terminal {
		return errors.Join(ErrPoisoned, g.cause)
	}
	return g.cause
}
func (g *cleanupGroup) closeResource(r cleanupResource) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.done {
		return nil
	}
	e := g.files[r]
	if e == nil {
		return nil
	}
	e.requested = true
	g.syncCallbacksLocked()
	if g.terminal || len(g.dependencies) != 0 {
		return g.tokenLocked(g.cause)
	}
	if e.role == cleanupLifecycle {
		for _, child := range g.files {
			if child.role == cleanupChild && !child.resource.RetirementComplete() {
				return g.tokenLocked(ErrPoisoned)
			}
		}
	}
	err := g.rememberLocked(r.closeNative())
	g.syncCallbacksLocked()
	g.pruneLocked()
	if _, pending := g.files[r]; pending {
		// A directory can be logically closed while a normally live child pins it.
		// Such a deferred parent is not a failure and has no standalone retry need.
		if err != nil {
			return g.tokenLocked(err)
		}
	} else if err != nil {
		g.failureLocked(err)
	}
	if len(g.dependencies) != 0 || g.terminal {
		return g.tokenLocked(err)
	}
	if len(g.files) == 0 && len(g.dependencies) == 0 && !g.terminal {
		g.releaseLocked()
	}
	return err
}
func (f *managedFile) Close() error {
	if f == nil {
		return nil
	}
	if f.owner == nil {
		return f.closeNative()
	} // native construction before adoption
	return f.owner.closeResource(f)
}

// closeResult adopts an operation's independent dependencies before the native
// close can release a child pin or its already-logically-closed parent.
func closeResult(f *managedFile, cause error) error {
	if f == nil {
		return cause
	}
	if f.owner != nil {
		cause = f.owner.remember(cause)
	}
	closeErr := f.Close()
	if closeErr == nil {
		return cause
	}
	if cause == nil {
		return closeErr
	}
	return errors.Join(cause, closeErr)
}
func (g *cleanupGroup) complete() bool { g.mu.Lock(); defer g.mu.Unlock(); return g.done }
func (g *cleanupGroup) retireLocked() error {
	observedPending := g.pending.Load()
	g.syncCallbacksLocked()
	if g.done {
		return nil
	}
	if g.terminal {
		return g.tokenLocked(ErrPoisoned)
	}
	var errs []error
	for dep := range g.dependencies {
		err := dep.retry()
		if err != nil {
			errs = append(errs, err)
		}
		if dep.complete() {
			g.registry.mu.Lock()
			delete(g.dependencies, dep)
			g.registry.mu.Unlock()
		}
	}
	if len(g.dependencies) != 0 {
		return g.tokenLocked(errors.Join(errs...))
	}
	for role := cleanupChild; role <= cleanupDirectory; role++ {
		pending := false
		for _, e := range g.files {
			if e.role != role {
				continue
			}
			if !g.closing && !e.requested {
				continue
			}
			if !g.closing && role == cleanupDirectory {
				live := false
				for _, child := range g.files {
					if child.role != cleanupDirectory && !child.requested {
						live = true
						break
					}
				}
				if live {
					continue
				}
			}
			e.requested = true
			err := g.stepLocked(e)
			if err != nil {
				errs = append(errs, err)
			}
			if _, ok := g.files[e.resource]; ok {
				pending = true
			}
		}
		g.syncCallbacksLocked()
		if pending || len(g.dependencies) != 0 || g.terminal {
			return g.tokenLocked(errors.Join(errs...))
		}
		// A live-scope token must never retire unrequested selected objects, its
		// lifecycle lease, or its directory merely because one temporary failed.
	}
	err := errors.Join(errs...)
	if len(g.files) == 0 {
		g.releaseLocked()
	} else {
		g.pending.CompareAndSwap(observedPending, nil)
	}
	return err
}
func (g *cleanupGroup) retry() error { g.mu.Lock(); defer g.mu.Unlock(); return g.retireLocked() }
func (g *cleanupGroup) closeAll(cause error) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.done {
		return cause
	}
	cause = g.rememberLocked(cause)
	if !g.closing {
		g.cause = errors.Join(cleanupDiagnostic(cause), g.cause)
	}
	g.closing = true
	initial := g.cause
	err := g.retireLocked()
	if g.done {
		return errors.Join(cleanupDiagnostic(cause), initial, cleanupDiagnostic(err))
	}
	return g.tokenLocked(errors.Join(cause, err))
}
func RetryCleanup(err error) error {
	groups, e := cleanupGroups(err)
	if e != nil {
		return e
	}
	var errs []error
	for _, g := range groups {
		if e := g.retry(); e != nil {
			errs = append(errs, e)
		}
	}
	return errors.Join(errs...)
}
func PendingCleanups() []error { return encryptedCleanups.snapshot() }
func (r *cleanupRegistry) snapshot() []error {
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
