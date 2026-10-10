package pager

import (
	"errors"
	"fmt"
	"sync"

	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
	"github.com/ms-xiaosnow/gosqlite/internal/walindex"
)

// ErrOpenCleanupPending marks an unsuccessful shared-WAL open whose ordered
// cleanup still owned resources when this error was produced. The error remains
// immutable after cleanup; RetryOpenCleanup reports the current cleanup result.
var ErrOpenCleanupPending = errors.New("shared WAL failed-open cleanup pending")

// The bound includes in-flight opens, not successful live sessions. Reservation
// happens before native acquisition. Retained groups are never evicted to admit
// another open, and admission never attempts somebody else's cleanup.
const maxSharedOpenRetirements = 64

var sharedOpenRetirements = sharedOpenRetirementRegistry{limit: maxSharedOpenRetirements}

type sharedOpenRetirementRegistry struct {
	mu     sync.Mutex
	limit  int
	groups map[*sharedOpenRetirement]struct{}
}

func (r *sharedOpenRetirementRegistry) reserve() (*sharedOpenRetirement, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.groups) >= r.limit {
		return nil, fmt.Errorf("%w: shared WAL failed-open cleanup capacity exhausted", ErrLimit)
	}
	if r.groups == nil {
		r.groups = make(map[*sharedOpenRetirement]struct{})
	}
	g := &sharedOpenRetirement{registry: r}
	r.groups[g] = struct{}{}
	return g, nil
}

// Only managed owners implement this boundary in production. In particular the
// group never keeps or retries a raw descriptor, mapping handle or native close.
// Pure-memory tests use fake owners with explicit live/terminal retirement state.
type sharedOpenResource interface{ Close() error }

type sharedOpenIndexOwner struct{ index *walindex.Index }

func (o sharedOpenIndexOwner) Close() error { return sharedIndexError(o.index.Close()) }

type sharedOpenFileOwner struct{ file *vfs.File }

func (o sharedOpenFileOwner) Close() error { return closeNativeFile(o.file) }

type sharedOpenRetirement struct {
	mu       sync.Mutex
	registry *sharedOpenRetirementRegistry
	// Open alone installs these owners before publishing an error. Thereafter
	// the group mutex serializes cleanup. A nil owner has conclusively retired
	// or was transferred to the successfully opened session.
	index, wal, main sharedOpenResource
	// Errors from nested constructors can carry owners even when no File or
	// Index was returned. These obligations retire before their protectors.
	nested []vfs.CleanupOwner
	cause  error // immutable original opening and first cleanup errors
	done   bool
}

// releaseLocked removes the reservation only after ownership has either been
// retired or transferred. No registry lock spans a resource operation.
func (g *sharedOpenRetirement) releaseLocked() {
	g.index, g.wal, g.main = nil, nil, nil
	g.nested = nil
	g.done = true
	g.registry.mu.Lock()
	delete(g.registry.groups, g)
	g.registry.mu.Unlock()
}

func (g *sharedOpenRetirement) transfer() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.releaseLocked()
}

func (g *sharedOpenRetirement) retireLocked() error {
	if g.done {
		return nil
	}
	for len(g.nested) != 0 {
		if err := g.nested[0].Close(); err != nil {
			return err
		}
		g.nested[0] = nil
		g.nested = g.nested[1:]
	}
	// Index.Close owns all mapped views, mapping objects and DMS. Preserve
	// WAL/main until it conclusively succeeds; preserve main until WAL succeeds.
	// An error never tells this layer whether a native resource is retryable:
	// only its managed owner may retry live state or retain a terminal tombstone.
	for _, owner := range []*sharedOpenResource{&g.index, &g.wal, &g.main} {
		if *owner == nil {
			continue
		}
		if err := (*owner).Close(); err != nil {
			return err
		}
		*owner = nil
	}
	g.releaseLocked()
	return nil
}

func (g *sharedOpenRetirement) failedOpen(openErr error) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.nested = vfs.CleanupOwners(openErr)
	if err := g.retireLocked(); err != nil {
		g.cause = errors.Join(openErr, err)
		return &sharedOpenCleanupError{group: g, cause: g.cause}
	}
	return openErr
}

type sharedOpenCleanupError struct {
	group *sharedOpenRetirement
	cause error
}

func (e *sharedOpenCleanupError) Error() string {
	return fmt.Sprintf("%v: %v", ErrOpenCleanupPending, e.cause)
}
func (e *sharedOpenCleanupError) Unwrap() []error {
	return []error{ErrOpenCleanupPending, ErrPoisoned, e.cause}
}

const maxOpenCleanupErrorNodes = 4096

// collectOpenCleanupGroups bounds even a cyclic/custom error's Unwrap chain.
// Complete discovery precedes cleanup, so a traversal refusal has no resource
// effects. Group tokens are leaves: their immutable native causes have already
// been adopted and do not need to be rewalked on every cleanup request.
func collectOpenCleanupGroups(openErr error) ([]*sharedOpenRetirement, error) {
	stack := []error{openErr}
	seen := make(map[*sharedOpenRetirement]bool)
	var groups []*sharedOpenRetirement
	count := 0
	limit := func() ([]*sharedOpenRetirement, error) {
		return nil, fmt.Errorf("%w: open cleanup error tree exceeds %d nodes; retain original errors and retry smaller trees", ErrLimit, maxOpenCleanupErrorNodes)
	}
	for len(stack) != 0 {
		if count >= maxOpenCleanupErrorNodes {
			return limit()
		}
		err := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		count++
		switch e := err.(type) {
		case *sharedOpenCleanupError:
			if !seen[e.group] {
				seen[e.group] = true
				groups = append(groups, e.group)
			}
		case interface{ Unwrap() []error }:
			children := e.Unwrap()
			if len(children) > maxOpenCleanupErrorNodes-count-len(stack) {
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

// RetryOpenCleanup synchronously retries every distinct retained group found in
// wrapped/joined unsuccessful OpenSharedSession errors. It discovers all groups
// before cleanup and limits traversal to 4096 error nodes; an over-budget tree
// causes ErrLimit without any cleanup. Keep original inputs to retry smaller
// trees. That traversal refusal deliberately does not wrap an unbounded/cyclic
// input, which would make ordinary errors.Is/As unsafe.
//
// Nil or an ordinary error without groups is a no-op. Nil means these groups
// retired, not that opening succeeded or original errors disappeared. Concurrent
// calls serialize only on each group, and completed groups are idempotent no-ops.
// Current failures are joined after independently attempting every found group;
// every failure preserves original causes and its retry route. This pager adds
// no accumulating error history (backend-owned error history is separate).
func RetryOpenCleanup(openErr error) error {
	groups, err := collectOpenCleanupGroups(openErr)
	if err != nil {
		return err
	}
	var failures []error
	for _, g := range groups {
		g.mu.Lock()
		err := g.retireLocked()
		if err != nil {
			failures = append(failures, &sharedOpenCleanupError{group: g, cause: errors.Join(g.cause, err)})
		}
		g.mu.Unlock()
	}
	return errors.Join(failures...)
}
