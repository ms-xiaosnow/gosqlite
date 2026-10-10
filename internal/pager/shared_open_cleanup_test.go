package pager

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"sync"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
	"github.com/ms-xiaosnow/gosqlite/internal/walindex"
)

// All negative retirement scenarios below are pure-memory fake owners. They
// never acquire a file, mapping or lock, and are not native failure injection.
type memoryOpenOwner struct {
	name      string
	calls     int
	remaining int
	terminal  bool
	rawCalls  int
	failure   error
	events    *[]string
}

func (o *memoryOpenOwner) Close() error {
	o.calls++
	*o.events = append(*o.events, o.name)
	if o.terminal && o.rawCalls != 0 {
		return o.failure // terminal uncertainty never retries a raw close
	}
	o.rawCalls++
	if o.remaining > 0 {
		o.remaining--
		return o.failure
	}
	return nil
}

type memoryOpenIndex struct {
	views      [2]bool
	failSecond bool
	dms        bool
	events     *[]string
	failure    error
}

func (o *memoryOpenIndex) Close() error {
	for i := range o.views {
		if !o.views[i] {
			continue
		}
		*o.events = append(*o.events, fmt.Sprintf("view%d", i))
		if i == 1 && o.failSecond {
			return o.failure
		}
		o.views[i] = false
	}
	*o.events = append(*o.events, "dms")
	o.dms = false
	return nil
}

func memoryOpenGroup(t *testing.T, limit int) (*sharedOpenRetirementRegistry, *sharedOpenRetirement) {
	t.Helper()
	r := &sharedOpenRetirementRegistry{limit: limit}
	g, err := r.reserve()
	if err != nil {
		t.Fatal(err)
	}
	return r, g
}

func TestSharedOpenCleanupMemoryViewsProtectDMSAndMain(t *testing.T) {
	r, g := memoryOpenGroup(t, 1)
	var events []string
	openErr := errors.New("logical open refusal")
	unmapErr := errors.New("memory view still live")
	index := &memoryOpenIndex{views: [2]bool{true, true}, failSecond: true, dms: true, events: &events, failure: unmapErr}
	wal := &memoryOpenOwner{name: "wal", events: &events}
	main := &memoryOpenOwner{name: "main", events: &events}
	g.index, g.wal, g.main = index, wal, main
	err := g.failedOpen(openErr)
	if !errors.Is(err, openErr) || !errors.Is(err, unmapErr) || !errors.Is(err, ErrPoisoned) || !errors.Is(err, ErrOpenCleanupPending) {
		t.Fatalf("opening/cleanup classification lost: %v", err)
	}
	if index.views != [2]bool{false, true} || !index.dms || wal.calls != 0 || main.calls != 0 || len(r.groups) != 1 {
		t.Fatalf("partial retirement dropped protection: %+v, %v", index, events)
	}
	// Dropping session reachability still leaves the registry's group and all
	// remaining resource owners strongly linked; no finalizer or GC is needed.
	if _, ok := r.groups[g]; !ok || g.index != index || g.wal != wal || g.main != main {
		t.Fatal("remaining group not strongly owned")
	}
	if _, e := r.reserve(); !errors.Is(e, ErrLimit) {
		t.Fatalf("pending owner evicted at capacity: %v", e)
	}
	index.failSecond = false
	wrapped := fmt.Errorf("connector open: %w", err)
	if e := RetryOpenCleanup(wrapped); e != nil {
		t.Fatal(e)
	}
	want := []string{"view0", "view1", "view1", "dms", "wal", "main"}
	if !reflect.DeepEqual(events, want) || index.dms || len(r.groups) != 0 || !g.done {
		t.Fatalf("retirement order/state: %v", events)
	}
	if g.index != nil || g.wal != nil || g.main != nil {
		t.Fatal("completed group retained owners")
	}
	if e := RetryOpenCleanup(err); e != nil || !reflect.DeepEqual(events, want) {
		t.Fatalf("cleanup not idempotent: %v %v", e, events)
	}
	if !errors.Is(err, openErr) || !errors.Is(err, ErrOpenCleanupPending) {
		t.Fatal("historical opening error mutated")
	}
	if _, e := r.reserve(); e != nil {
		t.Fatalf("retired reservation not reusable: %v", e)
	}
}

func TestSharedOpenCleanupMemoryEveryStageRetainsTail(t *testing.T) {
	for failed := 0; failed < 3; failed++ {
		t.Run(fmt.Sprint(failed), func(t *testing.T) {
			r, g := memoryOpenGroup(t, 1)
			var events []string
			openErr := errors.New("open refused")
			closeErr := errors.New("memory owner retained")
			owners := []*memoryOpenOwner{{name: "index", events: &events}, {name: "wal", events: &events}, {name: "main", events: &events}}
			owners[failed].remaining, owners[failed].failure = 2, closeErr
			g.index, g.wal, g.main = owners[0], owners[1], owners[2]
			err := g.failedOpen(openErr)
			for i, o := range owners {
				want := 0
				if i <= failed {
					want = 1
				}
				if o.calls != want {
					t.Fatalf("stage %d calls=%d want=%d", i, o.calls, want)
				}
			}
			retryErr := RetryOpenCleanup(err)
			if !errors.Is(retryErr, openErr) || !errors.Is(retryErr, closeErr) || !errors.Is(retryErr, ErrOpenCleanupPending) || len(r.groups) != 1 {
				t.Fatalf("retry causes/state: %v", retryErr)
			}
			if e := RetryOpenCleanup(retryErr); e != nil || len(r.groups) != 0 {
				t.Fatalf("final cleanup: %v", e)
			}
			for i, o := range owners {
				want := 1
				if i == failed {
					want = 3
				}
				if o.calls != want {
					t.Fatalf("retired owner %d repeated: %d", i, o.calls)
				}
			}
		})
	}
}

func TestSharedOpenCleanupMemoryTerminalOwnerNeverRetriesRawClose(t *testing.T) {
	r, g := memoryOpenGroup(t, 1)
	var events []string
	openErr := errors.New("open refused")
	closeErr := errors.New("memory close outcome unknown")
	index := &memoryOpenOwner{name: "terminal index", terminal: true, remaining: 1, failure: closeErr, events: &events}
	main := &memoryOpenOwner{name: "main", events: &events}
	g.index, g.main = index, main
	err := g.failedOpen(openErr)
	for i := 0; i < 20; i++ {
		err = RetryOpenCleanup(err)
		if !errors.Is(err, openErr) || !errors.Is(err, closeErr) || !errors.Is(err, ErrOpenCleanupPending) {
			t.Fatalf("retry %d: %v", i, err)
		}
		var detail *sharedOpenCleanupError
		if !errors.As(err, &detail) {
			t.Fatal("retry route disappeared")
		}
		if got := detail.cause.(interface{ Unwrap() []error }).Unwrap(); len(got) != 2 || got[0] != g.cause {
			t.Fatal("retry error chain grows with history")
		}
	}
	if index.rawCalls != 1 || main.calls != 0 || len(r.groups) != 1 || g.done {
		t.Fatal("terminal state retried a raw resource or lost protection")
	}
}

func TestSharedOpenCleanupMemoryReservationTransferAndEmptyFailure(t *testing.T) {
	r, g := memoryOpenGroup(t, 1)
	var events []string
	owner := &memoryOpenOwner{name: "main", events: &events}
	g.main = owner
	if _, err := r.reserve(); !errors.Is(err, ErrLimit) {
		t.Fatalf("in-flight reservation not bounded: %v", err)
	}
	g.transfer()
	if len(r.groups) != 0 || owner.calls != 0 || g.main != nil || !g.done {
		t.Fatal("happy path closed or retained transferred owner")
	}
	g, err := r.reserve()
	if err != nil {
		t.Fatal(err)
	}
	original := errors.New("pre-acquisition refusal")
	if err = g.failedOpen(original); err != original || len(r.groups) != 0 {
		t.Fatal("empty cleanup changed original error or leaked capacity")
	}
	for _, err := range []error{nil, original, fmt.Errorf("wrapper: %w", original)} {
		if e := RetryOpenCleanup(err); e != nil {
			t.Fatalf("no pending group should be a no-op: %v", e)
		}
	}
}

func TestSharedOpenCleanupMemoryConcurrentRetryExactlyOnce(t *testing.T) {
	r, g := memoryOpenGroup(t, 1)
	var events []string
	index := &memoryOpenOwner{name: "index", remaining: 1, failure: errors.New("memory live owner"), events: &events}
	main := &memoryOpenOwner{name: "main", events: &events}
	g.index, g.main = index, main
	err := g.failedOpen(errors.New("open refused"))
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if e := RetryOpenCleanup(err); e != nil {
				t.Error(e)
			}
		}()
	}
	wg.Wait()
	if index.calls != 2 || main.calls != 1 || len(r.groups) != 0 {
		t.Fatalf("concurrent cleanup repeated resource retirement: %v", events)
	}
}

type blockedMemoryOpenOwner struct {
	entered, release chan struct{}
}

func (o blockedMemoryOpenOwner) Close() error {
	close(o.entered)
	<-o.release
	return nil
}

func TestSharedOpenCleanupMemoryNoGlobalNativeOperationLock(t *testing.T) {
	r, g := memoryOpenGroup(t, 2)
	owner := blockedMemoryOpenOwner{entered: make(chan struct{}), release: make(chan struct{})}
	g.main = owner
	completed := make(chan error, 1)
	original := errors.New("memory open refusal")
	go func() { completed <- g.failedOpen(original) }()
	<-owner.entered
	// A different group can reserve, retire and release while the first owner
	// is deliberately paused entirely in memory. No native lock or I/O occurs.
	other, err := r.reserve()
	if err != nil {
		close(owner.release)
		t.Fatal(err)
	}
	if e := other.failedOpen(original); e != original {
		close(owner.release)
		t.Fatal(e)
	}
	close(owner.release)
	if e := <-completed; e != original || len(r.groups) != 0 {
		t.Fatalf("independent cleanup: %v", e)
	}
}

func TestSharedOpenCleanupMemoryIndexCausePreservation(t *testing.T) {
	for _, source := range []error{walindex.ErrBusy, walindex.ErrLimit, walindex.ErrUnsupported, walindex.ErrIdentity, walindex.ErrHeader, walindex.ErrPoisoned} {
		detail := errors.New("underlying memory cause")
		err := sharedIndexError(errors.Join(source, detail))
		if !errors.Is(err, source) || !errors.Is(err, detail) {
			t.Fatalf("index cause was discarded: %v", err)
		}
	}
}

func TestSharedOpenCleanupSuccessfulSessionsDoNotConsumeCapacity(t *testing.T) {
	txPlatform(t)
	path := filepath.Join(t.TempDir(), "ordinary-shared.db")
	var sessions []*SharedSession
	t.Cleanup(func() {
		for _, s := range sessions {
			if e := s.Close(); e != nil {
				t.Error(e)
			}
		}
	})
	// Only successful ordinary opens, commits, reads and closes. The healthy
	// connection count is not the failed-open reservation capacity.
	for i := 0; i < maxSharedOpenRetirements+1; i++ {
		s, err := OpenSharedSession(context.Background(), path, Options{Create: true, KeepWALOnClose: true})
		if err != nil {
			t.Fatal(err)
		}
		sessions = append(sessions, s)
	}
	sharedBump(t, sessions[0])
	for _, s := range sessions {
		r, err := s.Reader(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		sharedCookie(t, r, 1)
		if err = r.Close(); err != nil {
			t.Fatal(err)
		}
	}
	sharedOpenRetirements.mu.Lock()
	count := len(sharedOpenRetirements.groups)
	sharedOpenRetirements.mu.Unlock()
	if count != 0 {
		t.Fatalf("successful opens retained %d reservations", count)
	}
}

// The VFS collection law is tested with private error-carried owners in vfs.
// Here the returned interface list is composed with the actual group state
// machine using fake resources only, including a missing Index return value.
func TestSharedOpenCleanupMemoryNestedOwnersRetainMainAndCapacity(t *testing.T) {
	for _, terminal := range []bool{false, true} {
		t.Run(fmt.Sprint(terminal), func(t *testing.T) {
			r, g := memoryOpenGroup(t, 1)
			var events []string
			cause := errors.New("nested managed owner retained")
			child := &memoryOpenOwner{name: "nested SHM", remaining: 1, terminal: terminal, failure: cause, events: &events}
			main := &memoryOpenOwner{name: "main", events: &events}
			g.main = main
			g.nested = []vfs.CleanupOwner{child}
			g.cause = errors.New("nested constructor returned nil owner and an error")
			if err := g.retireLocked(); !errors.Is(err, cause) || main.calls != 0 || len(r.groups) != 1 {
				t.Fatal("failed nested constructor released its protector", err)
			}
			if _, err := r.reserve(); !errors.Is(err, ErrLimit) {
				t.Fatal("nested failure escaped the reservation bound", err)
			}
			err := RetryOpenCleanup(&sharedOpenCleanupError{group: g, cause: g.cause})
			if terminal {
				if !errors.Is(err, ErrOpenCleanupPending) || main.calls != 0 || child.rawCalls != 1 || len(r.groups) != 1 {
					t.Fatal("terminal nested obligation disappeared or retried raw close", err)
				}
			} else if err != nil || main.calls != 1 || len(r.groups) != 0 || g.nested != nil {
				t.Fatal("known-live nested cleanup did not retire before main", err)
			}
		})
	}
}

func TestSharedOpenCleanupMemoryJoinedIndependentGroups(t *testing.T) {
	r := &sharedOpenRetirementRegistry{limit: 3}
	var events []string
	var failures []error
	var owners []*memoryOpenOwner
	for _, name := range []string{"first", "second", "third"} {
		g, err := r.reserve()
		if err != nil {
			t.Fatal(err)
		}
		owner := &memoryOpenOwner{name: name, remaining: 2, failure: errors.New("memory retained " + name), events: &events}
		g.main = owner
		failures = append(failures, g.failedOpen(errors.New("open refused "+name)))
		owners = append(owners, owner)
	}
	// Retire the first group before it appears first in a joined request.
	owners[0].remaining = 0
	if err := RetryOpenCleanup(failures[0]); err != nil {
		t.Fatal(err)
	}
	joined := errors.Join(failures[0], failures[1], fmt.Errorf("wrapper: %w", failures[2]), failures[1])
	retry := RetryOpenCleanup(joined)
	if !errors.Is(retry, owners[1].failure) || !errors.Is(retry, owners[2].failure) || !errors.Is(retry, ErrOpenCleanupPending) {
		t.Fatal("joined cleanup hid an independent failure", retry)
	}
	if owners[0].calls != 2 || owners[1].calls != 2 || owners[2].calls != 2 || len(r.groups) != 2 {
		t.Fatal("duplicate or completed group was retried, or another group skipped", events)
	}
	if err := RetryOpenCleanup(retry); err != nil || owners[1].calls != 3 || owners[2].calls != 3 || len(r.groups) != 0 {
		t.Fatal("joined current failures did not retry independently", err)
	}
	if err := RetryOpenCleanup(joined); err != nil || owners[1].calls != 3 || owners[2].calls != 3 {
		t.Fatal("joined historical errors repeated completed cleanup", err)
	}
}

type memoryOpenWrapper struct{ cause error }

func (memoryOpenWrapper) Error() string   { return "memory wrapper" }
func (e memoryOpenWrapper) Unwrap() error { return e.cause }

type loopingOpenError struct{}

func (loopingOpenError) Error() string   { return "memory cyclic wrapper" }
func (e loopingOpenError) Unwrap() error { return e }

func TestSharedOpenCleanupMemoryTraversalBoundBeforeAnyCleanup(t *testing.T) {
	r, g := memoryOpenGroup(t, 1)
	var events []string
	owner := &memoryOpenOwner{name: "main", remaining: 1, failure: errors.New("memory retained"), events: &events}
	g.main = owner
	original := g.failedOpen(errors.New("open refused"))
	wide := make([]error, maxOpenCleanupErrorNodes)
	for i := range wide {
		wide[i] = original
	}
	for _, excessive := range []error{errors.Join(original, loopingOpenError{}), errors.Join(append(wide, original)...)} {
		if err := RetryOpenCleanup(excessive); !errors.Is(err, ErrLimit) {
			t.Fatal("unbounded error tree accepted", err)
		}
		if owner.calls != 1 || len(r.groups) != 1 {
			t.Fatal("cleanup began before complete bounded discovery")
		}
	}
	var wrapped error = original
	for i := 0; i < maxOpenCleanupErrorNodes-1; i++ {
		wrapped = memoryOpenWrapper{cause: wrapped}
	}
	if err := RetryOpenCleanup(wrapped); err != nil || owner.calls != 2 || len(r.groups) != 0 {
		t.Fatal("exact-budget discovery failed", err)
	}
}
