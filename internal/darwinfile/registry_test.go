package darwinfile

import (
	"errors"
	"testing"
)

// Every resource and syscall result in this file is a Go value or callback.
// No native handle, pathname, file, process, or failing OS operation exists.
func memoryRecord(t *testing.T, r *Registry, id Identity, calls *int) *Record {
	t.Helper()
	rec, e := r.Register(id, func() error { *calls++; return nil }, nil)
	if e != nil {
		t.Fatal(e)
	}
	return rec
}
func TestRegistryMemoryCrossDomainLastHold(t *testing.T) {
	for _, first := range []string{"vfs", "encstore"} {
		r := New()
		id := Identity{1, 2}
		a, b := 0, 0
		x := memoryRecord(t, r, id, &a)
		y := memoryRecord(t, r, id, &b)
		r.SetLocks(id, "vfs", true)
		r.SetLocks(id, "encstore", true)
		if e := r.Retire(x); e != nil {
			t.Fatal(e)
		}
		if e := r.Retire(y); e != nil {
			t.Fatal(e)
		}
		r.SetLocks(id, first, false)
		if e := r.Drain(id); e != nil {
			t.Fatal(e)
		}
		if a != 0 || b != 0 {
			t.Fatal("closed underneath other domain")
		}
		r.SetLocks(id, "vfs", false)
		r.SetLocks(id, "encstore", false)
		if e := r.Drain(id); e != nil {
			t.Fatal(e)
		}
		if a != 1 || b != 1 || len(r.records) != 0 || len(r.nodes) != 0 {
			t.Fatal("did not drain exactly once")
		}
	}
}
func TestRegistryMemoryDeferredReuseAndIdentity(t *testing.T) {
	r := New()
	id := Identity{1, 2}
	other := Identity{1, 3}
	calls := 0
	rec := memoryRecord(t, r, id, &calls)
	r.SetLocks(id, "encstore", true)
	if e := r.Retire(rec); e != nil {
		t.Fatal(e)
	}
	if !r.Reuse(rec) || r.Check(rec) != nil || r.Reuse(rec) {
		t.Fatal("invalid reuse state")
	}
	r.SetLocks(id, "encstore", false)
	if e := r.Drain(id); e != nil {
		t.Fatal(e)
	}
	if calls != 0 {
		t.Fatal("closed active reused record")
	}
	r.SetLocks(other, "vfs", true)
	if e := r.Retire(rec); e != nil {
		t.Fatal(e)
	}
	if calls != 1 || r.Reuse(rec) {
		t.Fatal("distinct inode delayed retirement or reused closed number")
	}
}
func TestRegistryMemoryCloseUnknownIsTerminal(t *testing.T) {
	r := New()
	id := Identity{1, 2}
	calls := 0
	fault := errors.New("synthetic close uncertainty")
	rec, e := r.Register(id, func() error { calls++; return fault }, nil)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 3; i++ {
		if e = r.Retire(rec); !errors.Is(e, ErrPoisoned) || !errors.Is(e, fault) {
			t.Fatal(e)
		}
	}
	if calls != 1 || len(r.records) != 1 || r.Check(rec) == nil || r.Admit(&id) == nil || r.Reuse(rec) {
		t.Fatal("terminal record escaped")
	}
}
func TestRegistryMemoryUnknownWaitsForAllDomainsAndInodes(t *testing.T) {
	r := New()
	a, b := Identity{1, 2}, Identity{1, 3}
	calls := 0
	r.SetLocks(a, "vfs", true)
	r.SetLocks(b, "encstore", true)
	if e := r.Quarantine(func() error { calls++; return nil }, nil); e != nil {
		t.Fatal(e)
	}
	if calls != 0 || r.Admit(nil) == nil {
		t.Fatal("unknown identity was not contained")
	}
	r.SetLocks(a, "vfs", false)
	if e := r.Drain(a); e != nil {
		t.Fatal(e)
	}
	if calls != 0 {
		t.Fatal("other inode still held locks")
	}
	r.SetLocks(b, "encstore", false)
	if e := r.Drain(b); e != nil {
		t.Fatal(e)
	}
	if calls != 1 || r.Admit(nil) != nil {
		t.Fatal("unknown did not retire")
	}
}
func TestRegistryMemoryUnknownCloseNeverRetries(t *testing.T) {
	r := New()
	calls := 0
	fault := errors.New("synthetic unknown close")
	if e := r.Quarantine(func() error { calls++; return fault }, nil); !errors.Is(e, ErrPoisoned) {
		t.Fatal(e)
	}
	for i := 0; i < 3; i++ {
		if e := r.DrainUnknown(); !errors.Is(e, fault) {
			t.Fatal(e)
		}
	}
	if calls != 1 || r.Admit(nil) == nil || len(r.unknown) != 1 {
		t.Fatal("unknown tombstone escaped")
	}
}
func TestRegistryMemoryReentrantParentRetirement(t *testing.T) {
	for _, sameInode := range []bool{false, true} {
		r := New()
		id, parentID := Identity{1, 2}, Identity{1, 3}
		if sameInode {
			parentID = id
		}
		parentCalls, childCalls := 0, 0
		parent := memoryRecord(t, r, parentID, &parentCalls)
		child, e := r.Register(id, func() error { childCalls++; return nil }, func(err error) error {
			if err != nil {
				return err
			}
			return r.Retire(parent)
		})
		if e != nil {
			t.Fatal(e)
		}
		r.SetLocks(id, "encstore", true)
		if e = r.Retire(child); e != nil {
			t.Fatal(e)
		}
		if parentCalls != 0 || childCalls != 0 {
			t.Fatal("lost directory pin")
		}
		r.SetLocks(id, "encstore", false)
		if e = r.Drain(id); e != nil {
			t.Fatal(e)
		}
		if parentCalls != 1 || childCalls != 1 || len(r.records) != 0 {
			t.Fatal("parent order/double close")
		}
	}
}
func TestRegistryMemoryParentCloseErrorPropagates(t *testing.T) {
	r := New()
	fault := errors.New("synthetic parent close")
	calls := 0
	parent, _ := r.Register(Identity{1, 1}, func() error { calls++; return fault }, nil)
	child, _ := r.Register(Identity{1, 2}, func() error { return nil }, func(error) error { return r.Retire(parent) })
	if e := r.Retire(child); !errors.Is(e, fault) || !errors.Is(e, ErrPoisoned) {
		t.Fatal(e)
	}
	if e := r.Retire(child); !errors.Is(e, fault) || calls != 1 {
		t.Fatal("retry lost parent result", e)
	}
}
func TestRegistryMemoryHookPanicCannotRetryClose(t *testing.T) {
	r := New()
	calls := 0
	rec, _ := r.Register(Identity{1, 2}, func() error { calls++; return nil }, func(error) error { panic("synthetic bookkeeping panic") })
	for i := 0; i < 3; i++ {
		if e := r.Retire(rec); !errors.Is(e, ErrPoisoned) {
			t.Fatal(e)
		}
	}
	if calls != 1 {
		t.Fatal("native close repeated")
	}
}
func TestRegistryMemoryAdmissionBoundsAndRacedAlias(t *testing.T) {
	r := New()
	id := Identity{1, 2}
	calls := 0
	r.SetLocks(id, "encstore", true)
	for i := 0; i < MaxPerInode; i++ {
		if e := r.Admit(&id); e != nil {
			t.Fatal(i, e)
		}
		memoryRecord(t, r, id, &calls)
	}
	if !errors.Is(r.Admit(&id), ErrLimit) {
		t.Fatal("inode capacity")
	}
	// A stale descriptor-free hint cannot justify an immediate physical close.
	other := Identity{2, 3}
	if e := r.Admit(&other); e != nil {
		t.Fatal(e)
	}
	raced, e := r.Register(id, func() error { calls++; return nil }, nil)
	if !errors.Is(e, ErrLimit) {
		t.Fatal(e)
	}
	if e = r.Retire(raced); e != nil {
		t.Fatal(e)
	}
	if calls != 0 || !errors.Is(r.Admit(&other), ErrLimit) {
		t.Fatal("overflow reservation escaped")
	}
	r.SetLocks(id, "encstore", false)
	if e = r.Drain(id); e != nil {
		t.Fatal(e)
	}
	if calls != 1 || r.Admit(&other) != nil {
		t.Fatal("overflow did not drain")
	}
	r = New()
	for i := 0; i < MaxDescriptors; i++ {
		memoryRecord(t, r, Identity{1, uint64(i)}, &calls)
	}
	if !errors.Is(r.Admit(nil), ErrLimit) {
		t.Fatal("process capacity")
	}
}
func TestRegistryMemoryPoisonCrossesDomains(t *testing.T) {
	r := New()
	id := Identity{1, 2}
	calls := 0
	a := memoryRecord(t, r, id, &calls)
	b := memoryRecord(t, r, id, &calls)
	r.Poison(id, errors.New("synthetic uncertain unlock"))
	if r.Check(a) == nil || r.Check(b) == nil || r.Admit(&id) == nil {
		t.Fatal("sibling retained usable ownership")
	}
}

func TestRegistryMemoryConcurrentDomainRetirement(t *testing.T) {
	r := New()
	id := Identity{7, 11}
	calls := 0
	const count = 8
	ready := make(chan struct{}, count)
	start := make(chan struct{})
	done := make(chan error, count)
	for i := 0; i < count; i++ {
		go func(i int) {
			domain := string(rune('a' + i))
			r.Lock()
			rec, e := r.Register(id, func() error { calls++; return nil }, nil)
			r.SetLocks(id, domain, true)
			r.Unlock()
			if e != nil {
				done <- e
				return
			}
			ready <- struct{}{}
			<-start
			r.Lock()
			e = r.Retire(rec)
			r.SetLocks(id, domain, false)
			e = errors.Join(e, r.Drain(id))
			r.Unlock()
			done <- e
		}(i)
	}
	for i := 0; i < count; i++ {
		<-ready
	}
	close(start)
	for i := 0; i < count; i++ {
		if e := <-done; e != nil {
			t.Fatal(e)
		}
	}
	if calls != count || len(r.records) != 0 || len(r.nodes) != 0 {
		t.Fatal("concurrent retirement", calls)
	}
}
