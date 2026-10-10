package pager

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

// Every exceptional owner below is memory-only: no descriptors, paths, native
// locks, mappings or filesystem fault hooks participate in these state laws.
type plainMemoryOwner struct {
	name                                 string
	events                               *[]string
	calls, rawCalls, remaining           int
	done, consumed, terminal, nilPending bool
	cause                                error
	statusEntered, statusResume          chan struct{}
	closeEntered, closeResume            chan struct{}
}

func (o *plainMemoryOwner) RetirementComplete() bool {
	if o.statusEntered != nil {
		close(o.statusEntered)
		<-o.statusResume
		o.statusEntered = nil
	}
	return o.done
}
func (o *plainMemoryOwner) Close() error {
	o.calls++
	if o.events != nil {
		*o.events = append(*o.events, o.name)
	}
	if o.closeEntered != nil {
		close(o.closeEntered)
		<-o.closeResume
		o.closeEntered = nil
	}
	if o.terminal {
		if o.rawCalls == 0 {
			o.rawCalls++
		}
		return o.cause
	}
	if o.nilPending {
		return nil
	}
	o.rawCalls++
	if o.remaining > 0 {
		o.remaining--
		return o.cause
	}
	o.done = true
	if o.consumed {
		return o.cause
	}
	return nil
}
func plainMemoryGroup(t *testing.T, r *plainRetirementRegistry) (*plainRetirement, *plainMemoryOwner, *plainMemoryOwner, *[]string) {
	t.Helper()
	g, err := r.reserve()
	if err != nil {
		t.Fatal(err)
	}
	events := new([]string)
	child := &plainMemoryOwner{name: "child", events: events, cause: errors.New("memory child diagnostic")}
	main := &plainMemoryOwner{name: "main", events: events, cause: errors.New("memory main diagnostic")}
	g.nested = []plainResource{child}
	g.main = main
	return g, child, main, events
}
func TestPlainCleanupMemoryStagesAndConsumedDiagnostics(t *testing.T) {
	for _, stage := range []string{"child", "main"} {
		t.Run(stage, func(t *testing.T) {
			reg := &plainRetirementRegistry{limit: 1}
			g, child, main, events := plainMemoryGroup(t, reg)
			failing := child
			if stage == "main" {
				failing = main
			}
			failing.remaining = 1
			original := errors.New("memory original")
			token := g.begin(original)
			if !errors.Is(token, ErrPlaintextCleanupPending) || !errors.Is(token, ErrPoisoned) || !errors.Is(token, original) {
				t.Fatal(token)
			}
			if stage == "child" && main.calls != 0 {
				t.Fatal("main released below child")
			}
			if len(reg.snapshot()) != 1 {
				t.Fatal("lost snapshot")
			}
			if _, err := reg.reserve(); !errors.Is(err, ErrLimit) {
				t.Fatal("lost admission", err)
			}
			failing.consumed = true
			err := RetryPlaintextCleanup(fmt.Errorf("wrapped: %w", token))
			if !errors.Is(err, failing.cause) || errors.Is(err, ErrPlaintextCleanupPending) {
				t.Fatal("diagnostic confused with ownership", err)
			}
			if !g.done || len(reg.groups) != 0 || g.main != nil || g.nested != nil || g.cause != nil {
				t.Fatal("retired group retains state")
			}
			before := append([]string(nil), (*events)...)
			if err := RetryPlaintextCleanup(token); err != nil || !reflect.DeepEqual(before, *events) {
				t.Fatal("repeated retired owner", err)
			}
			if !errors.Is(token, ErrPlaintextCleanupPending) {
				t.Fatal("historical token mutated")
			}
		})
	}
}
func TestPlainCleanupMemoryTerminalNilAndBoundedHistory(t *testing.T) {
	for _, nilPending := range []bool{false, true} {
		reg := &plainRetirementRegistry{limit: 1}
		g, child, main, _ := plainMemoryGroup(t, reg)
		child.terminal, child.nilPending = !nilPending, nilPending
		token := g.begin(nil)
		original := token
		first := g.cause
		for i := 0; i < 5000; i++ {
			child.cause = fmt.Errorf("memory diagnostic %d", i)
			token = RetryPlaintextCleanup(token)
			if !errors.Is(token, ErrPlaintextCleanupPending) || g.cause != first {
				t.Fatal("history unbounded", token)
			}
		}
		if main.calls != 0 || len(reg.groups) != 1 {
			t.Fatal("terminal lost protector")
		}
		if !nilPending && child.rawCalls != 1 {
			t.Fatal("terminal numeric owner replayed")
		}
		snapshot := reg.snapshot()
		snapshot[0] = nil
		if reg.snapshot()[0] == nil || !errors.Is(original, ErrPlaintextCleanupPending) {
			t.Fatal("mutable snapshot")
		}
	}
}
func TestPlainCleanupMemoryAdmission1024(t *testing.T) {
	reg := &plainRetirementRegistry{limit: maxPlainRetirements}
	var groups []*plainRetirement
	for i := 0; i < maxPlainRetirements; i++ {
		g, e := reg.reserve()
		if e != nil {
			t.Fatal(i, e)
		}
		groups = append(groups, g)
	}
	if len(reg.snapshot()) != 0 {
		t.Fatal("ordinary opening/live reservation is pending")
	}
	if _, e := reg.reserve(); !errors.Is(e, ErrLimit) {
		t.Fatal(e)
	}
	for _, g := range groups {
		if e := g.begin(nil); e != nil {
			t.Fatal(e)
		}
	}
	if len(reg.groups) != 0 {
		t.Fatal("empty failed constructor retained")
	}
}

type plainMemoryCycle struct{}

func (plainMemoryCycle) Error() string   { return "memory cycle" }
func (e plainMemoryCycle) Unwrap() error { return e }
func TestPlainCleanupMemoryDiscoveryBeforeEffects(t *testing.T) {
	reg := &plainRetirementRegistry{limit: 2}
	a, ac, _, _ := plainMemoryGroup(t, reg)
	b, bc, _, _ := plainMemoryGroup(t, reg)
	ac.remaining, bc.remaining = 1, 2
	x, y := a.begin(nil), b.begin(nil)
	wide := make([]error, maxPlainCleanupErrorNodes)
	for i := range wide {
		wide[i] = x
	}
	for _, input := range []error{errors.Join(x, plainMemoryCycle{}), errors.Join(wide...)} {
		e := RetryPlaintextCleanup(input)
		if !errors.Is(e, ErrLimit) || errors.Is(e, ErrPlaintextCleanupPending) || ac.calls != 1 || bc.calls != 1 {
			t.Fatal("partial discovery effects", e)
		}
	}
	e := RetryPlaintextCleanup(errors.Join(x, fmt.Errorf("duplicate: %w", x), y))
	if !a.done || b.done || ac.calls != 2 || bc.calls != 2 || !errors.Is(e, bc.cause) {
		t.Fatal("distinct groups", e)
	}
	if e = RetryPlaintextCleanup(e); e != nil || !b.done {
		t.Fatal(e)
	}
	for _, input := range []error{nil, errors.New("ordinary")} {
		if e = RetryPlaintextCleanup(input); e != nil {
			t.Fatal(e)
		}
	}
	// An exactly 4096-node chain is permitted.
	var chain error = x
	for i := 1; i < maxPlainCleanupErrorNodes; i++ {
		chain = fmt.Errorf("wrapper: %w", chain)
	}
	if e = RetryPlaintextCleanup(chain); e != nil {
		t.Fatal(e)
	}
	if e = RetryPlaintextCleanup(fmt.Errorf("extra: %w", chain)); !errors.Is(e, ErrLimit) {
		t.Fatal(e)
	}
}
func TestPlainCleanupMemoryUnsafeInternalCauseBarrier(t *testing.T) {
	reg := &plainRetirementRegistry{limit: 1}
	g, _, main, _ := plainMemoryGroup(t, reg)
	g.nested = nil
	token := g.begin(plainMemoryCycle{})
	if !errors.Is(token, ErrPlaintextCleanupPending) || !errors.Is(token, ErrLimit) || main.calls != 0 {
		t.Fatal("unsafe graph released protector", token)
	}
	if e := RetryPlaintextCleanup(token); !errors.Is(e, ErrPlaintextCleanupPending) || main.calls != 0 {
		t.Fatal(e)
	}
}
func TestPlainCleanupMemoryLiveRetryAndReaderInvalidation(t *testing.T) {
	reg := &plainRetirementRegistry{limit: 1}
	g, child, main, _ := plainMemoryGroup(t, reg)
	g.cause = errors.Join(ErrPoisoned, child.cause)
	token := g.pendingLocked(g.cause)                // mirrors a failed read-time managed probe
	reader := &Reader{file: &vfs.File{}, cleanup: g} // descriptor-free placeholder, never operated
	if _, e := reader.ReadPage(context.Background(), 1); !errors.Is(e, ErrPoisoned) {
		t.Fatal(e)
	}
	if e := reader.ValidatePlaintextCopySource(); !errors.Is(e, ErrPoisoned) {
		t.Fatal(e)
	}
	if e := RetryPlaintextCleanup(token); e != nil || main.calls != 0 || g.done {
		t.Fatal("live retry closed reader", e)
	}
	if len(reg.snapshot()) != 0 || len(reg.groups) != 1 {
		t.Fatal("live retry ownership")
	}
	if e := g.check(); !errors.Is(e, ErrPoisoned) {
		t.Fatal("live reader unpoisoned", e)
	}
	main.remaining = 1
	e := reader.Close()
	if !reader.closed || reader.file != nil || !errors.Is(e, ErrPlaintextCleanupPending) {
		t.Fatal(e)
	}
	if _, e = reader.ReadPage(context.Background(), 1); !errors.Is(e, ErrClosed) {
		t.Fatal(e)
	}
	if e = reader.Close(); e != nil || !g.done || main.calls != 2 {
		t.Fatal(e)
	}
	if e = reader.Close(); e != nil || main.calls != 2 {
		t.Fatal(e)
	}
}
func TestPlainCleanupMemoryAdmissionIndependentOfOwnerLocks(t *testing.T) {
	for _, blockStatus := range []bool{false, true} {
		reg := &plainRetirementRegistry{limit: 2}
		g, child, _, _ := plainMemoryGroup(t, reg)
		child.remaining = 1
		token := g.begin(nil)
		entered, resume := make(chan struct{}), make(chan struct{})
		if blockStatus {
			child.statusEntered, child.statusResume = entered, resume
		} else {
			child.closeEntered, child.closeResume = entered, resume
		}
		result := make(chan error, 1)
		go func() { result <- RetryPlaintextCleanup(token) }()
		<-entered
		independent := make(chan error, 1)
		go func() {
			if len(reg.snapshot()) != 1 {
				independent <- errors.New("missing pending token")
				return
			}
			other, e := reg.reserve()
			if e == nil {
				e = other.begin(nil)
			}
			independent <- e
		}()
		select {
		case e := <-independent:
			if e != nil {
				t.Fatal(e)
			}
		case <-time.After(3 * time.Second):
			close(resume)
			t.Fatal("owner blocked global admission")
		}
		var wg sync.WaitGroup
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				if e := RetryPlaintextCleanup(token); e != nil {
					t.Error(e)
				}
			}()
		}
		close(resume)
		if e := <-result; e != nil {
			t.Fatal(e)
		}
		wg.Wait()
		if child.calls != 2 || len(reg.groups) != 0 {
			t.Fatal("concurrent duplicate retirement")
		}
	}
}

// Actual database/sql disposal over memory-only owners proves that discarding a
// driver Close result does not discard the scope. No real native failure runs.
type plainMemoryConnector struct {
	g     *plainRetirement
	reset bool
}

func (c plainMemoryConnector) Connect(context.Context) (driver.Conn, error) {
	return &plainMemoryConn{c.g, c.reset}, nil
}
func (c plainMemoryConnector) Driver() driver.Driver { return c }
func (c plainMemoryConnector) Open(string) (driver.Conn, error) {
	return c.Connect(context.Background())
}

type plainMemoryConn struct {
	g     *plainRetirement
	reset bool
}

func (c *plainMemoryConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("unused") }
func (c *plainMemoryConn) Begin() (driver.Tx, error)           { return nil, errors.New("unused") }
func (c *plainMemoryConn) Close() error                        { return c.g.begin(nil) }
func (c *plainMemoryConn) Ping(context.Context) error          { return nil }
func (c *plainMemoryConn) ResetSession(context.Context) error {
	if c.reset {
		return driver.ErrBadConn
	}
	return nil
}
func TestPlainCleanupMemoryDatabaseSQLDiscardedClose(t *testing.T) {
	for _, reset := range []bool{false, true} {
		reg := &plainRetirementRegistry{limit: 1}
		g, child, main, _ := plainMemoryGroup(t, reg)
		child.terminal = true
		pool := sql.OpenDB(plainMemoryConnector{g, reset})
		if !reset {
			pool.SetMaxIdleConns(0)
		}
		if e := pool.PingContext(context.Background()); e != nil {
			t.Fatal(e)
		}
		if reset {
			_ = pool.PingContext(context.Background())
		}
		if e := pool.Close(); e != nil && !errors.Is(e, ErrPlaintextCleanupPending) {
			t.Fatal(e)
		}
		tokens := reg.snapshot()
		if len(tokens) != 1 || main.calls != 0 {
			t.Fatal("discarded error lost owner")
		}
		child.terminal = false
		if e := RetryPlaintextCleanup(errors.Join(tokens...)); e != nil || !g.done {
			t.Fatal(e)
		}
	}
}

func TestPlainCleanupMemoryNoDBFailedConstructorSnapshot(t *testing.T) {
	reg := &plainRetirementRegistry{limit: 1}
	g, e := reg.reserve()
	if e != nil {
		t.Fatal(e)
	}
	nested := &plainMemoryOwner{remaining: 1, cause: errors.New("memory nested constructor owner")}
	g.nested = []plainResource{nested}
	original := errors.New("memory constructor returns no Reader")
	e = g.begin(original)
	if !errors.Is(e, ErrPlaintextCleanupPending) || !errors.Is(e, original) || len(reg.snapshot()) != 1 || g.main != nil {
		t.Fatal(e)
	}
	if e = RetryPlaintextCleanup(reg.snapshot()[0]); e != nil || !g.done || len(reg.groups) != 0 {
		t.Fatal(e)
	}
}
