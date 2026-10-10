package pager

import (
	"context"
	"database/sql"
	driver "database/sql/driver"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
)

// These owners have no descriptors, paths, mappings, locks or filesystem calls.
// Error cases are memory state transitions, never injected native failures.
type memoryCloseOwner struct {
	name                                 string
	events                               *[]string
	calls, rawCalls                      int
	remaining                            int
	terminal, consumed, done, nilPending bool
	cause                                error
	entered, resume                      chan struct{}
}

func (o *memoryCloseOwner) RetirementComplete() bool { return o.done }
func (o *memoryCloseOwner) Close() error {
	o.calls++
	if o.events != nil {
		*o.events = append(*o.events, o.name)
	}
	if o.entered != nil {
		close(o.entered)
		<-o.resume
		o.entered = nil
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
func memoryCloseGroup(t *testing.T, r *sharedCloseRetirementRegistry) (*sharedCloseRetirement, []*memoryCloseOwner, *[]string) {
	t.Helper()
	g, err := r.reserve()
	if err != nil {
		t.Fatal(err)
	}
	events := new([]string)
	var owners []*memoryCloseOwner
	for _, name := range []string{"nested", "index", "wal", "main"} {
		owners = append(owners, &memoryCloseOwner{name: name, events: events, cause: fmt.Errorf("memory %s remains", name)})
	}
	g.nested = []sharedCloseResource{owners[0]}
	g.index, g.wal, g.main = owners[1], owners[2], owners[3]
	return g, owners, events
}

func TestSharedCloseCleanupMemoryEveryStageBarrier(t *testing.T) {
	for barrier := 0; barrier < 4; barrier++ {
		t.Run(fmt.Sprint(barrier), func(t *testing.T) {
			r := &sharedCloseRetirementRegistry{limit: 1}
			g, owners, events := memoryCloseGroup(t, r)
			owners[barrier].remaining = 1
			original := errors.New("memory historical operation cause")
			err := g.begin(original)
			if !errors.Is(err, ErrCloseCleanupPending) || !errors.Is(err, ErrPoisoned) || !errors.Is(err, original) || !errors.Is(err, owners[barrier].cause) {
				t.Fatal(err)
			}
			for i, o := range owners {
				if i > barrier && o.calls != 0 {
					t.Fatalf("protector %d released below %d: %v", i, barrier, *events)
				}
			}
			if len(r.snapshot()) != 1 || len(r.groups) != 1 {
				t.Fatal("pending group not retained")
			}
			if _, e := r.reserve(); !errors.Is(e, ErrLimit) {
				t.Fatal("capacity evicted a group", e)
			}
			if e := RetryCloseCleanup(fmt.Errorf("wrapper: %w", err)); e != nil {
				t.Fatal(e)
			}
			if !g.done || len(r.groups) != 0 || g.index != nil || g.wal != nil || g.main != nil || g.cause != nil || len(g.nested) != 0 {
				t.Fatal("completed group retained owners")
			}
			calls := append([]string(nil), (*events)...)
			if e := RetryCloseCleanup(err); e != nil || !reflect.DeepEqual(calls, *events) {
				t.Fatal("completion repeated effects", e)
			}
			if !errors.Is(err, original) || !errors.Is(err, ErrCloseCleanupPending) {
				t.Fatal("historical token mutated")
			}
		})
	}
}

func TestSharedCloseCleanupMemoryConsumedErrorAndHistoricalPoisonRetire(t *testing.T) {
	for consumed := 0; consumed < 4; consumed++ {
		r := &sharedCloseRetirementRegistry{limit: 1}
		g, owners, events := memoryCloseGroup(t, r)
		owners[consumed].consumed = true
		err := g.begin(ErrPoisoned)
		if !errors.Is(err, owners[consumed].cause) || !errors.Is(err, ErrPoisoned) || errors.Is(err, ErrCloseCleanupPending) {
			t.Fatal(err)
		}
		if !g.done || len(r.snapshot()) != 0 || len(r.groups) != 0 || !reflect.DeepEqual(*events, []string{"nested", "index", "wal", "main"}) {
			t.Fatal("error confused with live ownership", *events)
		}
		if err := g.retry(); err != nil {
			t.Fatal(err)
		}
		for _, o := range owners {
			if o.calls != 1 {
				t.Fatal("consumed owner called again")
			}
		}
	}
	// An error in a later successful retry still reports the diagnostic but no
	// pending marker; every discovered group is attempted independently.
	r := &sharedCloseRetirementRegistry{limit: 2}
	g, owners, _ := memoryCloseGroup(t, r)
	owners[2].remaining, owners[2].consumed = 1, true
	err := g.begin(nil)
	if !errors.Is(err, ErrCloseCleanupPending) {
		t.Fatal(err)
	}
	if e := RetryCloseCleanup(err); !errors.Is(e, owners[2].cause) || errors.Is(e, ErrCloseCleanupPending) || !g.done {
		t.Fatal(e)
	}
}

func TestSharedCloseCleanupMemoryTerminalAndNilAreNotCompletion(t *testing.T) {
	for _, nilPending := range []bool{false, true} {
		r := &sharedCloseRetirementRegistry{limit: 1}
		g, owners, _ := memoryCloseGroup(t, r)
		owners[1].terminal, owners[1].nilPending = !nilPending, nilPending
		err := g.begin(nil)
		for i := 0; i < 20; i++ {
			if !errors.Is(err, ErrCloseCleanupPending) || !errors.Is(err, ErrPoisoned) {
				t.Fatal(err)
			}
			err = RetryCloseCleanup(err)
		}
		if owners[2].calls != 0 || owners[3].calls != 0 || len(r.groups) != 1 {
			t.Fatal("terminal protector released")
		}
		if !nilPending {
			if owners[1].rawCalls != 1 {
				t.Fatal("terminal state retried consumed raw ownership")
			}
			continue // terminal owner and protectors remain retained in this local registry
		}
		owners[1].nilPending = false
		if e := RetryCloseCleanup(err); e != nil {
			t.Fatal(e)
		}
	}
}

func TestSharedCloseCleanupMemoryAdmission1024NoEviction(t *testing.T) {
	r := &sharedCloseRetirementRegistry{limit: maxSharedCloseRetirements}
	groups := make([]*sharedCloseRetirement, maxSharedCloseRetirements)
	for i := range groups {
		var err error
		groups[i], err = r.reserve()
		if err != nil {
			t.Fatal(i, err)
		}
	}
	if len(r.snapshot()) != 0 {
		t.Fatal("live admissions appeared as pending")
	}
	if _, err := r.reserve(); !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	for _, g := range groups {
		if g.done {
			t.Fatal("evicted existing group")
		}
	}
	groups[0].abandon()
	last, err := r.reserve()
	if err != nil {
		t.Fatal(err)
	}
	last.abandon()
	for _, g := range groups[1:] {
		g.abandon()
	}
	if len(r.groups) != 0 {
		t.Fatal("admissions leaked")
	}
}

type cyclicCloseInput struct{}

func (cyclicCloseInput) Error() string   { return "memory cycle" }
func (e cyclicCloseInput) Unwrap() error { return e }

func TestSharedCloseCleanupMemoryAllTokensDedupAndNoEffectsRefusal(t *testing.T) {
	r := &sharedCloseRetirementRegistry{limit: 2}
	g1, one, _ := memoryCloseGroup(t, r)
	g2, two, _ := memoryCloseGroup(t, r)
	one[1].remaining, two[1].remaining = 1, 2
	e1, e2 := g1.begin(nil), g2.begin(nil)
	wide := make([]error, maxCloseCleanupErrorNodes)
	for i := range wide {
		wide[i] = e1
	}
	for _, bad := range []error{errors.Join(e1, cyclicCloseInput{}), errors.Join(wide...)} {
		if err := RetryCloseCleanup(bad); !errors.Is(err, ErrLimit) || errors.Is(err, ErrCloseCleanupPending) {
			t.Fatal(err)
		}
		if one[1].calls != 1 || two[1].calls != 1 {
			t.Fatal("partial discovery had effects")
		}
	}
	err := RetryCloseCleanup(errors.Join(e1, fmt.Errorf("duplicate: %w", e1), e2))
	if !g1.done || g2.done || one[1].calls != 2 || two[1].calls != 2 || !errors.Is(err, two[1].cause) {
		t.Fatal("not all distinct groups attempted", err)
	}
	groups, e := collectCloseCleanupGroups(err)
	if e != nil || len(groups) != 1 || groups[0] != g2 {
		t.Fatal("completed group remained in current pending result", e)
	}
	if err = RetryCloseCleanup(err); err != nil || !g2.done {
		t.Fatal(err)
	}
	for _, input := range []error{nil, errors.New("ordinary"), fmt.Errorf("wrapped: %w", errors.New("ordinary"))} {
		if e := RetryCloseCleanup(input); e != nil {
			t.Fatal(e)
		}
	}
}

func TestSharedCloseCleanupMemoryConcurrencyAndRegistryLock(t *testing.T) {
	r := &sharedCloseRetirementRegistry{limit: 2}
	g, owners, _ := memoryCloseGroup(t, r)
	owners[1].remaining = 1
	token := g.begin(nil)
	owners[1].entered, owners[1].resume = make(chan struct{}), make(chan struct{})
	entered, resume := owners[1].entered, owners[1].resume
	result := make(chan error, 1)
	go func() { result <- RetryCloseCleanup(token) }()
	<-entered
	available := make(chan error, 1)
	go func() {
		if len(r.snapshot()) != 1 {
			available <- errors.New("missing snapshot")
			return
		}
		other, err := r.reserve()
		if err == nil {
			other.abandon()
		}
		available <- err
	}()
	select {
	case err := <-available:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		close(resume)
		t.Fatal("cleanup held global registry lock")
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := RetryCloseCleanup(token); err != nil {
				t.Error(err)
			}
		}()
	}
	close(resume)
	if err := <-result; err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	if owners[1].calls != 2 || owners[2].calls != 1 || owners[3].calls != 1 || len(r.snapshot()) != 0 {
		t.Fatal("concurrent retirement repeated effects")
	}
}

func TestSharedCloseCleanupMemoryBoundedHistoryAndSnapshot(t *testing.T) {
	r := &sharedCloseRetirementRegistry{limit: 1}
	g, owners, _ := memoryCloseGroup(t, r)
	owners[1].remaining = 5002
	first := g.begin(nil)
	base := g.cause
	input := first
	for i := 0; i < 5000; i++ {
		owners[1].cause = fmt.Errorf("memory current %d", i)
		last := RetryCloseCleanup(input)
		input = last
		if g.cause != base || !errors.Is(last, owners[1].cause) {
			t.Fatal("history accumulated or current cause lost")
		}
		if i != 0 && errors.Is(first, owners[1].cause) {
			t.Fatal("historical token changed")
		}
	}
	snapshot := r.snapshot()
	if len(snapshot) != 1 {
		t.Fatal(snapshot)
	}
	snapshot[0] = nil
	if len(r.snapshot()) != 1 || r.snapshot()[0] == nil {
		t.Fatal("caller changed registry")
	}
	owners[1].remaining = 0
	if err := RetryCloseCleanup(errors.Join(r.snapshot()...)); err != nil {
		t.Fatal(err)
	}
	if !errors.Is(first, ErrCloseCleanupPending) || len(r.snapshot()) != 0 {
		t.Fatal("snapshot or historical state incorrect")
	}
}

// The real standard-library pool is deliberately allowed to discard Close
// errors, using exclusively descriptor-free owner groups for its connections.
type memoryCloseConnector struct{ group *sharedCloseRetirement }

func (c memoryCloseConnector) Connect(context.Context) (driver.Conn, error) {
	return &memoryCloseConn{group: c.group}, nil
}
func (c memoryCloseConnector) Driver() driver.Driver { return c }
func (c memoryCloseConnector) Open(string) (driver.Conn, error) {
	return c.Connect(context.Background())
}

type memoryCloseConn struct{ group *sharedCloseRetirement }

func (c *memoryCloseConn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unused memory prepare")
}
func (c *memoryCloseConn) Begin() (driver.Tx, error)  { return nil, errors.New("unused memory begin") }
func (c *memoryCloseConn) Close() error               { return c.group.begin(nil) }
func (c *memoryCloseConn) Ping(context.Context) error { return nil }

func TestSharedCloseCleanupMemoryDatabaseSQLDiscardsError(t *testing.T) {
	r := &sharedCloseRetirementRegistry{limit: 1}
	g, owners, _ := memoryCloseGroup(t, r)
	owners[1].remaining = 1
	pool := sql.OpenDB(memoryCloseConnector{g})
	pool.SetMaxIdleConns(0)
	if err := pool.PingContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	// The zero-idle disposal closes its connection after Ping, with no Close
	// error returned to Ping's caller. No connection or DB is rooted by the group.
	if len(r.snapshot()) != 1 || owners[2].calls != 0 || owners[3].calls != 0 {
		t.Fatal("database/sql discarded ownership with Close error")
	}
	if err := pool.Close(); err != nil {
		t.Fatal(err)
	}
	if err := RetryCloseCleanup(errors.Join(r.snapshot()...)); err != nil {
		t.Fatal(err)
	}
	if !g.done || len(r.groups) != 0 {
		t.Fatal("snapshot did not recover discarded-error retry route")
	}
}

func TestSharedCloseCleanupMemoryCheckpointOnceAndBusyLease(t *testing.T) {
	for _, checkpointErr := range []error{nil, ErrBusy, errors.New("memory checkpoint diagnostic")} {
		r := &sharedCloseRetirementRegistry{limit: 1}
		g, owners, _ := memoryCloseGroup(t, r)
		owners[1].remaining = 1
		s := &SharedSession{closeGroup: g, active: &Tx{}}
		calls := 0
		checkpoint := func() error { calls++; return checkpointErr }
		if err := s.closeLocked(nil, checkpoint); !errors.Is(err, ErrBusy) || s.closed || calls != 0 {
			t.Fatal("active lease closure changed state", err)
		}
		s.active = nil
		err := s.closeLocked(nil, checkpoint)
		if !errors.Is(err, ErrCloseCleanupPending) || calls != 1 || !s.closed {
			t.Fatal(err, calls)
		}
		if checkpointErr != nil && checkpointErr != ErrBusy && !errors.Is(err, checkpointErr) {
			t.Fatal("checkpoint cause lost")
		}
		if checkpointErr == ErrBusy && errors.Is(err, ErrBusy) {
			t.Fatal("ordinary reader-limited checkpoint became an error")
		}
		if err = s.closeLocked(nil, checkpoint); err != nil || calls != 1 {
			t.Fatal("checkpoint repeated on retry", err, calls)
		}
		if err = s.checkOpenLocked(); !errors.Is(err, ErrClosed) {
			t.Fatal("retired session reopened", err)
		}
	}
	for _, keep := range []bool{false, true} {
		r := &sharedCloseRetirementRegistry{limit: 1}
		g, _, _ := memoryCloseGroup(t, r)
		s := &SharedSession{closeGroup: g, opts: Options{KeepWALOnClose: keep}}
		if !keep {
			s.poisoned = ErrPoisoned
		}
		if err := s.closeLocked(nil, func() error { t.Fatal("historical poison/keep mode attempted checkpoint"); return nil }); errors.Is(err, ErrCloseCleanupPending) {
			t.Fatal(err)
		}
		if !g.done {
			t.Fatal("historical poison or keep prevented retirement")
		}
	}
}

type memoryResetConnector struct {
	first, second *sharedCloseRetirement
	calls         int
}

func (c *memoryResetConnector) Connect(context.Context) (driver.Conn, error) {
	c.calls++
	if c.calls == 1 {
		return &memoryResetConn{memoryCloseConn: memoryCloseConn{c.first}, bad: true}, nil
	}
	return &memoryResetConn{memoryCloseConn: memoryCloseConn{c.second}}, nil
}
func (c *memoryResetConnector) Driver() driver.Driver { return c }
func (c *memoryResetConnector) Open(string) (driver.Conn, error) {
	return c.Connect(context.Background())
}

type memoryResetConn struct {
	memoryCloseConn
	bad bool
}

func (c *memoryResetConn) ResetSession(context.Context) error {
	if c.bad {
		return driver.ErrBadConn
	}
	return nil
}
func TestSharedCloseCleanupMemoryDatabaseSQLResetDiscardsError(t *testing.T) {
	r := &sharedCloseRetirementRegistry{limit: 2}
	g1, owners, _ := memoryCloseGroup(t, r)
	g2, _, _ := memoryCloseGroup(t, r)
	owners[1].remaining = 1
	connector := &memoryResetConnector{first: g1, second: g2}
	pool := sql.OpenDB(connector)
	defer pool.Close()
	pool.SetMaxOpenConns(1)
	if err := pool.PingContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := pool.PingContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if connector.calls != 2 || len(r.snapshot()) != 1 || owners[2].calls != 0 || owners[3].calls != 0 {
		t.Fatal("bad reset discarded pending owners")
	}
	if err := pool.Close(); err != nil {
		t.Fatal(err)
	}
	if err := RetryCloseCleanup(errors.Join(r.snapshot()...)); err != nil {
		t.Fatal(err)
	}
	if !g1.done || !g2.done || len(r.groups) != 0 {
		t.Fatal("discarded connection did not retire")
	}
}

func TestSharedCloseCleanupMemoryAdmissionCancelAndRejectedOpen(t *testing.T) {
	for _, cancelled := range []bool{false, true} {
		r := &sharedCloseRetirementRegistry{limit: 1}
		g, err := r.reserve()
		if err != nil {
			t.Fatal(err)
		}
		a := &SharedCloseAdmission{group: g}
		ctx, cancel := context.WithCancel(context.Background())
		if cancelled {
			cancel()
		}
		defer cancel()
		// Either canceled context or invalid page size rejects before native
		// metadata/file access. No path is created, probed or opened.
		s, e := OpenSharedSessionAdmitted(ctx, "unused", Options{PageSize: 13}, a)
		if s != nil || e == nil || len(r.groups) != 0 || !g.done {
			t.Fatal("rejected admission not released", e)
		}
		a.Cancel()
		a.Cancel()
		if _, e = OpenSharedSessionAdmitted(ctx, "unused", Options{}, a); !errors.Is(e, ErrMisuse) {
			t.Fatal("reused admission accepted", e)
		}
	}
	r := &sharedCloseRetirementRegistry{limit: 1}
	g, _ := r.reserve()
	a := &SharedCloseAdmission{group: g}
	a.Cancel()
	a.Cancel()
	if !g.done || len(r.groups) != 0 {
		t.Fatal("unused admission leaked")
	}
}

type memoryCloseWrap struct{ err error }

func (memoryCloseWrap) Error() string   { return "memory wrapper" }
func (w memoryCloseWrap) Unwrap() error { return w.err }
func TestSharedCloseCleanupMemoryExact4096NodeBoundary(t *testing.T) {
	r := &sharedCloseRetirementRegistry{limit: 1}
	g, owners, _ := memoryCloseGroup(t, r)
	owners[1].remaining = 3
	token := g.begin(nil)
	var exact error = token
	for i := 1; i < maxCloseCleanupErrorNodes; i++ {
		exact = memoryCloseWrap{exact}
	}
	if err := RetryCloseCleanup(memoryCloseWrap{exact}); !errors.Is(err, ErrLimit) || owners[1].calls != 1 {
		t.Fatal("4097-node refusal had effects", err)
	}
	if err := RetryCloseCleanup(exact); !errors.Is(err, ErrCloseCleanupPending) || owners[1].calls != 2 {
		t.Fatal("4096-node chain incorrectly refused", err)
	}
	wide := make([]error, maxCloseCleanupErrorNodes-1)
	for i := range wide {
		wide[i] = token
	}
	if err := RetryCloseCleanup(errors.Join(wide...)); !errors.Is(err, ErrCloseCleanupPending) || owners[1].calls != 3 {
		t.Fatal("4096-node wide tree incorrectly refused or duplicated", err)
	}
	if err := RetryCloseCleanup(token); err != nil {
		t.Fatal(err)
	}
}

func TestSharedCloseCleanupMemoryFailedOpenCapacityRemainsSeparate(t *testing.T) {
	var openings []*sharedOpenRetirement
	defer func() {
		for _, g := range openings {
			g.transfer()
		}
	}()
	for i := 0; i < maxSharedOpenRetirements; i++ {
		g, err := sharedOpenRetirements.reserve()
		if err != nil {
			t.Fatal(err)
		}
		openings = append(openings, g)
	}
	sharedCloseRetirements.mu.Lock()
	before := len(sharedCloseRetirements.groups)
	sharedCloseRetirements.mu.Unlock()
	// The exhausted failed-open admission rejects before pathname resolution or
	// file acquisition, and must return its unused successful-close reservation.
	s, err := OpenSharedSession(context.Background(), "unused", Options{})
	if s != nil || !errors.Is(err, ErrLimit) {
		t.Fatal(err)
	}
	sharedCloseRetirements.mu.Lock()
	after := len(sharedCloseRetirements.groups)
	sharedCloseRetirements.mu.Unlock()
	if before != after {
		t.Fatal("failed-open refusal leaked successful-close admission")
	}
}
