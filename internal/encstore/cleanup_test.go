package encstore

import (
	"context"
	"database/sql"
	driver "database/sql/driver"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ms-xiaosnow/gosqlite/encryption"
)

// These owners contain only memory. Negative callbacks never wrap real files,
// descriptors, native locks, providers, or filesystem operations.
type memoryEncryptedOwner struct {
	calls int
	done  bool
	call  func(*memoryEncryptedOwner) error
}

func (o *memoryEncryptedOwner) closeNative() error {
	o.calls++
	if o.call != nil {
		return o.call(o)
	}
	o.done = true
	return nil
}
func (o *memoryEncryptedOwner) RetirementComplete() bool { return o.done }
func memoryEncryptedGroup(t *testing.T) (*cleanupRegistry, *cleanupGroup) {
	t.Helper()
	r := &cleanupRegistry{scopeLimit: 1024, leaseLimit: 16384}
	g, e := r.reserve()
	if e != nil {
		t.Fatal(e)
	}
	return r, g
}
func memoryEncryptedAdd(t *testing.T, g *cleanupGroup, o cleanupResource, role cleanupRole) {
	t.Helper()
	if e := g.registry.reserveLease(); e != nil {
		t.Fatal(e)
	}
	g.addLocked(o, role)
}
func TestEncryptedCleanupMemoryStageBarriers(t *testing.T) {
	for _, barrier := range []cleanupRole{cleanupChild, cleanupLifecycle, cleanupDirectory} {
		t.Run(fmt.Sprint(barrier), func(t *testing.T) {
			r, g := memoryEncryptedGroup(t)
			cause := errors.New("memory retained close")
			var owners [3]*memoryEncryptedOwner
			for role := cleanupChild; role <= cleanupDirectory; role++ {
				role := role
				owners[role] = &memoryEncryptedOwner{call: func(o *memoryEncryptedOwner) error {
					if role == barrier && o.calls == 1 {
						return cause
					}
					o.done = true
					return nil
				}}
				memoryEncryptedAdd(t, g, owners[role], role)
			}
			e := g.closeAll(nil)
			if !errors.Is(e, ErrCleanupPending) || !errors.Is(e, cause) {
				t.Fatal(e)
			}
			for role := barrier + 1; role <= cleanupDirectory; role++ {
				if owners[role].calls != 0 {
					t.Fatal("protector retired early", role)
				}
			}
			if e = RetryCleanup(e); e != nil {
				t.Fatal(e)
			}
			if !g.done || len(r.groups) != 0 || r.leases != 0 {
				t.Fatal("reservation leaked")
			}
		})
	}
}
func TestEncryptedCleanupMemoryConsumedDiagnosticAndNilIncomplete(t *testing.T) {
	for _, consumed := range []bool{false, true} {
		r, g := memoryEncryptedGroup(t)
		cause := errors.New("memory consumed diagnostic")
		child := &memoryEncryptedOwner{call: func(o *memoryEncryptedOwner) error {
			o.done = consumed
			if consumed {
				return cause
			}
			return nil
		}}
		lock := &memoryEncryptedOwner{}
		memoryEncryptedAdd(t, g, child, cleanupChild)
		memoryEncryptedAdd(t, g, lock, cleanupLifecycle)
		e := g.closeAll(nil)
		if consumed {
			if !errors.Is(e, cause) || errors.Is(e, ErrCleanupPending) || lock.calls != 1 || r.leases != 0 {
				t.Fatal(e)
			}
		} else {
			if !errors.Is(e, ErrCleanupPending) || lock.calls != 0 {
				t.Fatal(e)
			}
			child.done = true
			if e = g.retry(); e != nil {
				t.Fatal(e)
			}
		}
	}
}
func TestEncryptedCleanupMemoryLiveRetryAndHistoricalTokens(t *testing.T) {
	r, g := memoryEncryptedGroup(t)
	cause := errors.New("memory temporary retained")
	temporary := &memoryEncryptedOwner{call: func(o *memoryEncryptedOwner) error {
		if o.calls == 1 {
			return cause
		}
		o.done = true
		return nil
	}}
	selected, lock, dir := &memoryEncryptedOwner{}, &memoryEncryptedOwner{}, &memoryEncryptedOwner{}
	memoryEncryptedAdd(t, g, temporary, cleanupChild)
	memoryEncryptedAdd(t, g, selected, cleanupChild)
	memoryEncryptedAdd(t, g, lock, cleanupLifecycle)
	memoryEncryptedAdd(t, g, dir, cleanupDirectory)
	e := g.closeResource(temporary)
	if !errors.Is(e, ErrCleanupPending) {
		t.Fatal(e)
	}
	if x := RetryCleanup(e); x != nil {
		t.Fatal(x)
	}
	if selected.calls != 0 || lock.calls != 0 || dir.calls != 0 || g.done || r.leases != 3 {
		t.Fatal("live retry closed selected protection")
	}
	if !errors.Is(e, ErrCleanupPending) {
		t.Fatal("historical token mutated")
	}
	if e = g.closeAll(nil); !errors.Is(e, cause) {
		t.Fatal("first cause lost", e)
	}
	if !g.done || r.leases != 0 {
		t.Fatal("retirement incomplete")
	}
}
func TestEncryptedCleanupMemoryCapacityBeforeAcquisition(t *testing.T) {
	r := &cleanupRegistry{scopeLimit: 2, leaseLimit: 1}
	g, e := r.reserve()
	if e != nil {
		t.Fatal(e)
	}
	h, e := r.reserve()
	if e != nil {
		t.Fatal(e)
	}
	if _, e = r.reserve(); !errors.Is(e, encryption.ErrLimit) {
		t.Fatal(e)
	}
	memoryEncryptedAdd(t, g, &memoryEncryptedOwner{}, cleanupChild)
	calls := 0
	_, e = h.acquireLocked(cleanupChild, func() (*managedFile, error) { calls++; return nil, nil })
	if !errors.Is(e, encryption.ErrLimit) || calls != 0 {
		t.Fatal("acquired beyond reservation", e, calls)
	}
	if e = g.closeAll(nil); e != nil {
		t.Fatal(e)
	}
	_, e = h.acquireLocked(cleanupChild, func() (*managedFile, error) { calls++; return nil, errors.New("memory preacquisition rejection") })
	if e == nil || calls != 1 || r.leases != 0 {
		t.Fatal(e, calls, r.leases)
	}
	if e = h.closeAll(nil); e != nil {
		t.Fatal(e)
	}
}
func TestEncryptedCleanupMemoryNoDBUnknownConstructor(t *testing.T) {
	r, g := memoryEncryptedGroup(t)
	cause := errors.New("memory quarantined native owner")
	_, e := g.acquireLocked(cleanupChild, func() (*managedFile, error) { return nil, nativeOwnershipPending(cause) })
	if !errors.Is(e, ErrCleanupPending) || r.leases != 1 || len(r.groups) != 1 {
		t.Fatal(e)
	}
	for i := 0; i < 100; i++ {
		if e = RetryCleanup(e); !errors.Is(e, ErrCleanupPending) {
			t.Fatal(e)
		}
	}
	if r.leases != 1 || !g.terminal {
		t.Fatal("unknown native number was freed")
	}
}
func TestEncryptedCleanupMemoryDependenciesAndCycle(t *testing.T) {
	r, g := memoryEncryptedGroup(t)
	h, e := r.reserve()
	if e != nil {
		t.Fatal(e)
	}
	cause := errors.New("memory nested pending")
	leaf := &memoryEncryptedOwner{call: func(o *memoryEncryptedOwner) error {
		if o.calls < 3 {
			return cause
		}
		o.done = true
		return nil
	}}
	protector := &memoryEncryptedOwner{}
	memoryEncryptedAdd(t, h, leaf, cleanupChild)
	memoryEncryptedAdd(t, g, protector, cleanupLifecycle)
	nested := h.closeAll(nil)
	e = g.closeAll(nested)
	if !errors.Is(e, ErrCleanupPending) || protector.calls != 0 {
		t.Fatal(e)
	}
	if e = RetryCleanup(e); e != nil {
		t.Fatal(e)
	}
	if protector.calls != 1 || !g.done || !h.done || r.leases != 0 {
		t.Fatal("nested retirement not ordered")
	}
	r, g = memoryEncryptedGroup(t)
	h, _ = r.reserve()
	g.remember(&cleanupError{group: h, cause: cause})
	h.remember(&cleanupError{group: g, cause: cause})
	if !h.terminal {
		t.Fatal("cycle not rejected")
	}
	if e = g.closeAll(nil); !errors.Is(e, ErrCleanupPending) {
		t.Fatal(e)
	}
}

type memoryEncryptedWrap struct{ next error }

func (e *memoryEncryptedWrap) Error() string { return "memory wrapper" }
func (e *memoryEncryptedWrap) Unwrap() error { return e.next }
func TestEncryptedCleanupMemoryBoundedDiscoveryAndHistory(t *testing.T) {
	_, g := memoryEncryptedGroup(t)
	cause := errors.New("first failure")
	o := &memoryEncryptedOwner{call: func(o *memoryEncryptedOwner) error { return fmt.Errorf("current failure %d", o.calls) }}
	memoryEncryptedAdd(t, g, o, cleanupChild)
	token := g.closeAll(cause)
	for i := 0; i < 5000; i++ {
		token = RetryCleanup(token)
		if len(token.Error()) > 500 {
			t.Fatal("history accumulated", len(token.Error()))
		}
	}
	if !errors.Is(token, cause) {
		t.Fatal("original cause lost")
	}
	calls := o.calls
	var chain error = g.pending.Load()
	for i := 0; i < 4095; i++ {
		chain = &memoryEncryptedWrap{next: chain}
	}
	if _, e := cleanupGroups(chain); e != nil {
		t.Fatal("exact boundary rejected", e)
	}
	chain = &memoryEncryptedWrap{next: chain}
	if e := RetryCleanup(chain); !errors.Is(e, encryption.ErrLimit) || o.calls != calls {
		t.Fatal("refusal performed cleanup", e)
	}
	cycle := &memoryEncryptedWrap{}
	cycle.next = cycle
	if e := RetryCleanup(cycle); !errors.Is(e, encryption.ErrLimit) || o.calls != calls {
		t.Fatal(e)
	}
	o.done = true
	if e := RetryCleanup(token); e != nil {
		t.Fatal(e)
	}
}
func TestEncryptedCleanupMemoryConcurrentRetryDeduplicates(t *testing.T) {
	r, g := memoryEncryptedGroup(t)
	o := &memoryEncryptedOwner{call: func(o *memoryEncryptedOwner) error {
		if o.calls == 1 {
			return errors.New("memory retry")
		}
		o.done = true
		return nil
	}}
	memoryEncryptedAdd(t, g, o, cleanupChild)
	token := g.closeAll(nil)
	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if e := RetryCleanup(errors.Join(token, token)); e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	if o.calls != 2 || r.leases != 0 || len(r.groups) != 0 {
		t.Fatal("duplicate/concurrent retry")
	}
}
func TestEncryptedCleanupMemorySecretReleaseBeforeRetirement(t *testing.T) {
	_, g := memoryEncryptedGroup(t)
	o := &memoryEncryptedOwner{call: func(*memoryEncryptedOwner) error { return errors.New("memory retained native owner") }}
	memoryEncryptedAdd(t, g, o, cleanupChild)
	m := reviewMemoryMaster(t)
	raw := []byte("memory secret root evidence")
	s := &Store{cleanup: g, master: m, selection: selection{rootBytes: raw}, opts: Options{KeyProvider: reviewMemoryProvider(make([]byte, 32))}}
	if e := s.Close(); !errors.Is(e, ErrCleanupPending) {
		t.Fatal(e)
	}
	if s.master != nil || s.opts.KeyProvider != nil || s.rootBytes != nil || strings.Trim(string(raw), "\x00") != "" {
		t.Fatal("logical secrets retained")
	}
	if _, e := m.Epoch(); !errors.Is(e, encryption.ErrClosed) {
		t.Fatal("master not closed", e)
	}
	if e := s.Check(); !errors.Is(e, encryption.ErrClosed) {
		t.Fatal(e)
	}
	o.done = true
	if e := s.Close(); e != nil {
		t.Fatal(e)
	}
}

func TestEncryptedCleanupMemoryNativeCallbackBarrier(t *testing.T) {
	r, g := memoryEncryptedGroup(t)
	h, e := r.reserve()
	if e != nil {
		t.Fatal(e)
	}
	nested := &memoryEncryptedOwner{call: func(o *memoryEncryptedOwner) error {
		if o.calls == 1 {
			return errors.New("memory nested native")
		}
		o.done = true
		return nil
	}}
	memoryEncryptedAdd(t, h, nested, cleanupChild)
	leaf := &memoryEncryptedOwner{call: func(o *memoryEncryptedOwner) error { o.done = true; return h.closeAll(nil) }}
	lock := &memoryEncryptedOwner{}
	memoryEncryptedAdd(t, g, leaf, cleanupChild)
	memoryEncryptedAdd(t, g, lock, cleanupLifecycle)
	token := g.closeAll(nil)
	if !errors.Is(token, ErrCleanupPending) || lock.calls != 0 {
		t.Fatal("native-returned dependency bypassed", token)
	}
	if e = RetryCleanup(token); e != nil {
		t.Fatal(e)
	}
	if lock.calls != 1 || !g.done || !h.done {
		t.Fatal("native dependency not retained")
	}
}
func TestEncryptedCleanupMemoryAsynchronousTerminalTransfer(t *testing.T) {
	_, g := memoryEncryptedGroup(t)
	lock, dir := &memoryEncryptedOwner{}, &memoryEncryptedOwner{}
	memoryEncryptedAdd(t, g, lock, cleanupLifecycle)
	memoryEncryptedAdd(t, g, dir, cleanupDirectory)
	// Represents a lease previously transferred to a native process registry.
	// Only atomic common callback helpers are called, never managed native Close.
	transferred := &managedFile{managedOwnership: managedOwnership{owner: g}}
	token := managedTerminalRetirementError(transferred, errors.New("memory delayed terminal native close"))
	if !errors.Is(g.health(), ErrPoisoned) {
		t.Fatal("delayed terminal not visible to operation")
	}
	calls := 0
	if _, e := g.acquireLocked(cleanupChild, func() (*managedFile, error) { calls++; return nil, nil }); !errors.Is(e, ErrPoisoned) || calls != 0 {
		t.Fatal("terminal scope acquired", e)
	}
	if e := RetryCleanup(token); !errors.Is(e, ErrCleanupPending) {
		t.Fatal(e)
	}
	if e := g.closeAll(nil); !errors.Is(e, ErrCleanupPending) || lock.calls != 0 || dir.calls != 0 {
		t.Fatal("terminal transfer released protectors", e)
	}
}
func TestEncryptedCleanupMemoryDeferredParentAndReap(t *testing.T) {
	r, g := memoryEncryptedGroup(t)
	parent := &memoryEncryptedOwner{call: func(*memoryEncryptedOwner) error { return nil }}
	child := &memoryEncryptedOwner{}
	memoryEncryptedAdd(t, g, parent, cleanupDirectory)
	memoryEncryptedAdd(t, g, child, cleanupChild)
	if e := g.closeResource(parent); e != nil || g.pending.Load() != nil {
		t.Fatal("normal parent pin poisoned", e)
	}
	if e := g.closeResource(child); e != nil {
		t.Fatal(e)
	}
	parent.done = true
	g.files[parent].completed.Store(true)
	g.reap.Store(true)
	h, e := r.reserve()
	if e != nil {
		t.Fatal(e)
	}
	if !g.done || r.leases != 0 {
		t.Fatal("completed parent bookkeeping leaked")
	}
	if e = h.closeAll(nil); e != nil {
		t.Fatal(e)
	}
}

type memoryEncryptedConnector struct {
	registry *cleanupRegistry
	groups   []*cleanupGroup
	reset    bool
}

func (c *memoryEncryptedConnector) Connect(context.Context) (driver.Conn, error) {
	g, e := c.registry.reserve()
	if e != nil {
		return nil, e
	}
	first := len(c.groups) == 0
	c.groups = append(c.groups, g)
	o := &memoryEncryptedOwner{call: func(o *memoryEncryptedOwner) error {
		if first && o.calls == 1 {
			return errors.New("memory pool close retained")
		}
		o.done = true
		return nil
	}}
	if e = c.registry.reserveLease(); e != nil {
		return nil, e
	}
	g.addLocked(o, cleanupChild)
	return &memoryEncryptedConn{group: g, reset: first && c.reset}, nil
}
func (*memoryEncryptedConnector) Driver() driver.Driver { return memoryEncryptedDriver{} }

type memoryEncryptedDriver struct{}

func (memoryEncryptedDriver) Open(string) (driver.Conn, error) {
	return nil, errors.New("memory connector only")
}

type memoryEncryptedConn struct {
	group *cleanupGroup
	reset bool
}

func (*memoryEncryptedConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("memory no statements")
}
func (c *memoryEncryptedConn) Close() error { return c.group.closeAll(nil) }
func (*memoryEncryptedConn) Begin() (driver.Tx, error) {
	return nil, errors.New("memory no transactions")
}
func (*memoryEncryptedConn) Ping(context.Context) error { return nil }
func (c *memoryEncryptedConn) ResetSession(context.Context) error {
	if c.reset {
		return driver.ErrBadConn
	}
	return nil
}
func TestEncryptedCleanupMemoryPoolDiscardedCloseSnapshots(t *testing.T) {
	for _, reset := range []bool{false, true} {
		r := &cleanupRegistry{scopeLimit: 1024, leaseLimit: 16384}
		connector := &memoryEncryptedConnector{registry: r, reset: reset}
		pool := sql.OpenDB(connector)
		pool.SetMaxOpenConns(1)
		if !reset {
			pool.SetMaxIdleConns(0)
		}
		if e := pool.Ping(); e != nil {
			t.Fatal(e)
		}
		if reset {
			if e := pool.Ping(); e != nil {
				t.Fatal(e)
			}
		}
		if e := pool.Close(); e != nil {
			t.Fatal(e)
		}
		tokens := r.snapshot()
		if len(tokens) != 1 {
			t.Fatal("discarded close route lost", len(tokens))
		}
		token := tokens[0]
		tokens[0] = nil
		if len(r.snapshot()) != 1 {
			t.Fatal("snapshot mutation changed registry")
		}
		if e := RetryCleanup(token); e != nil {
			t.Fatal(e)
		}
		if len(r.snapshot()) != 0 || len(r.groups) != 0 || r.leases != 0 {
			t.Fatal("pool cleanup incomplete")
		}
	}
}
func TestEncryptedCleanupMemoryExactCapacityAndBlockedIndependence(t *testing.T) {
	r := &cleanupRegistry{scopeLimit: maxEncryptedScopes, leaseLimit: maxEncryptedLeases}
	var groups []*cleanupGroup
	for i := 0; i < maxEncryptedScopes; i++ {
		g, e := r.reserve()
		if e != nil {
			t.Fatal(i, e)
		}
		groups = append(groups, g)
	}
	if _, e := r.reserve(); !errors.Is(e, encryption.ErrLimit) {
		t.Fatal(e)
	}
	if len(r.snapshot()) != 0 {
		t.Fatal("live reservations exposed as pending")
	}
	for _, g := range groups {
		if e := g.closeAll(nil); e != nil {
			t.Fatal(e)
		}
	}
	g, e := r.reserve()
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < maxEncryptedLeases; i++ {
		memoryEncryptedAdd(t, g, &memoryEncryptedOwner{}, cleanupChild)
	}
	calls := 0
	if _, e := g.acquireLocked(cleanupChild, func() (*managedFile, error) { calls++; return nil, nil }); !errors.Is(e, encryption.ErrLimit) || calls != 0 {
		t.Fatal(e)
	}
	if e = g.closeAll(nil); e != nil {
		t.Fatal(e)
	}
	g, e = r.reserve()
	if e != nil {
		t.Fatal(e)
	}
	entered, resume := make(chan struct{}), make(chan struct{})
	owner := &memoryEncryptedOwner{call: func(o *memoryEncryptedOwner) error { close(entered); <-resume; o.done = true; return nil }}
	memoryEncryptedAdd(t, g, owner, cleanupChild)
	done := make(chan error, 1)
	go func() { done <- g.closeAll(nil) }()
	<-entered
	g.reap.Store(true)
	independent := make(chan error, 1)
	go func() {
		h, e := r.reserve()
		if e == nil {
			_ = r.snapshot()
			e = h.closeAll(nil)
		}
		independent <- e
	}()
	select {
	case e := <-independent:
		if e != nil {
			t.Error(e)
		}
	case <-time.After(time.Second):
		t.Error("blocked group prevented admission/snapshot")
	}
	close(resume)
	if e := <-done; e != nil {
		t.Fatal(e)
	}
}
func TestEncryptedCleanupMemoryCompletedResultHasNoHistoricalMarker(t *testing.T) {
	_, g := memoryEncryptedGroup(t)
	cause := errors.New("memory first operation")
	o := &memoryEncryptedOwner{call: func(o *memoryEncryptedOwner) error {
		if o.calls == 1 {
			return cause
		}
		o.done = true
		return nil
	}}
	memoryEncryptedAdd(t, g, o, cleanupChild)
	old := g.closeResource(o)
	if !errors.Is(old, ErrCleanupPending) {
		t.Fatal(old)
	}
	current := g.closeAll(old)
	if !g.done || errors.Is(current, ErrCleanupPending) || !errors.Is(current, cause) {
		t.Fatal("completed result retained marker or lost cause", current)
	}
	if !errors.Is(old, ErrCleanupPending) {
		t.Fatal("historical token changed")
	}
}
func TestEncryptedCleanupMemoryProbeErrorsProtectAbsentRoot(t *testing.T) {
	r, g := memoryEncryptedGroup(t)
	h, e := r.reserve()
	if e != nil {
		t.Fatal(e)
	}
	original := errors.New("memory metadata probe failure")
	probe := &memoryEncryptedOwner{call: func(o *memoryEncryptedOwner) error {
		if o.calls < 3 {
			return original
		}
		o.done = true
		return nil
	}}
	lock, dir := &memoryEncryptedOwner{}, &memoryEncryptedOwner{}
	memoryEncryptedAdd(t, h, probe, cleanupChild)
	memoryEncryptedAdd(t, g, lock, cleanupLifecycle)
	memoryEncryptedAdd(t, g, dir, cleanupDirectory)
	token := h.closeAll(nil)
	store := &Store{ns: &namespace{group: g}, cleanup: g}
	e = store.checkAbsentRootResult(token)
	if !errors.Is(e, original) || !errors.Is(e, ErrCleanupPending) || len(g.dependencies) != 1 {
		t.Fatal("metadata probe error discarded", e)
	}
	if e = g.closeResource(dir); !errors.Is(e, ErrCleanupPending) || dir.calls != 0 {
		t.Fatal("directory retired before independent probe", e)
	}
	if e = store.Close(); !errors.Is(e, ErrCleanupPending) || lock.calls != 0 || dir.calls != 0 {
		t.Fatal("probe dependency lost before lifecycle", e)
	}
	if e = store.Close(); e != nil || lock.calls != 1 || dir.calls != 1 {
		t.Fatal(e)
	}
	if e = store.checkAbsentRootResult(nil); !errors.Is(e, ErrChanged) {
		t.Fatal("present root not changed", e)
	}
}

// This owner's status is deliberately unavailable while admission runs. It is
// descriptor-free and the channel does not stand in for a real native failure.
type memoryEncryptedBlockedStatus struct {
	status <-chan struct{}
}

func (o *memoryEncryptedBlockedStatus) RetirementComplete() bool { <-o.status; return true }
func (*memoryEncryptedBlockedStatus) closeNative() error         { return nil }

func TestEncryptedCleanupMemoryAdmissionNeverCallsStatus(t *testing.T) {
	r, g := memoryEncryptedGroup(t)
	resume := make(chan struct{})
	blocked := &memoryEncryptedBlockedStatus{status: resume}
	memoryEncryptedAdd(t, g, blocked, cleanupChild)
	completed := &memoryEncryptedOwner{done: true}
	memoryEncryptedAdd(t, g, completed, cleanupDirectory)
	g.files[completed].completed.Store(true)
	g.reap.Store(true)
	done := make(chan error, 1)
	go func() {
		h, err := r.reserve()
		if err == nil {
			err = h.closeAll(nil)
		}
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Error(err)
		}
		if len(g.files) != 1 || r.leases != 1 {
			t.Error("published completion was not reclaimed")
		}
	case <-time.After(time.Second):
		t.Error("admission called an unrelated resource status")
	}
	close(resume)
	if err := g.closeAll(nil); err != nil {
		t.Fatal(err)
	}
}

type memoryEncryptedReaderAt func([]byte, int64) (int, error)

func (r memoryEncryptedReaderAt) ReadAt(p []byte, off int64) (int, error) { return r(p, off) }
func TestEncryptedCleanupMemoryReadExactPreservesCause(t *testing.T) {
	cause := errors.New("memory read cause")
	_, err := readExact(memoryEncryptedReaderAt(func([]byte, int64) (int, error) { return 0, cause }), 8, 0)
	if !errors.Is(err, encryption.ErrFormat) || !errors.Is(err, cause) {
		t.Fatal(err)
	}
}
func TestEncryptedCleanupMemoryConstructorCauseBlocksChildren(t *testing.T) {
	r, g := memoryEncryptedGroup(t)
	dep, err := r.reserve()
	if err != nil {
		t.Fatal(err)
	}
	cause := errors.New("memory independent constructor cause")
	probe := &memoryEncryptedOwner{call: func(o *memoryEncryptedOwner) error {
		if o.calls < 3 {
			return cause
		}
		o.done = true
		return nil
	}}
	child, lock, dir := &memoryEncryptedOwner{}, &memoryEncryptedOwner{}, &memoryEncryptedOwner{}
	memoryEncryptedAdd(t, dep, probe, cleanupChild)
	memoryEncryptedAdd(t, g, child, cleanupChild)
	memoryEncryptedAdd(t, g, lock, cleanupLifecycle)
	memoryEncryptedAdd(t, g, dir, cleanupDirectory)
	token := dep.closeAll(nil)
	g.remember(token)
	if err = g.closeResource(child); !errors.Is(err, ErrCleanupPending) || child.calls != 0 {
		t.Fatal("direct child close crossed independent dependency", err)
	}
	store := &Store{cleanup: g}
	if err = store.CloseWithError(token); !errors.Is(err, cause) || child.calls != 0 || lock.calls != 0 || dir.calls != 0 {
		t.Fatal("failed constructor cause did not guard all stages", err)
	}
	if err = store.Close(); err != nil || !g.done || child.calls != 1 || lock.calls != 1 || dir.calls != 1 {
		t.Fatal(err)
	}
}
