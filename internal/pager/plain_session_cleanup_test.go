package pager

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
	"github.com/ms-xiaosnow/gosqlite/internal/wal"
)

// Every exceptional owner below is a descriptor-free state model. Inert File
// values mark logical openness only; Close always reaches the model owners.
func plainSessionMemory(t *testing.T) (*Session, *plainRetirement, *plainMemoryOwner, *plainMemoryOwner) {
	t.Helper()
	r := &plainRetirementRegistry{limit: 1}
	g, err := r.reserve()
	if err != nil {
		t.Fatal(err)
	}
	main, current := &plainMemoryOwner{name: "main"}, &plainMemoryOwner{name: "wal"}
	g.main, g.wal = main, current
	s := &Session{file: &vfs.File{}, walFile: &vfs.File{}, cleanup: g, opts: Options{PageSize: 4096}, snapshot: wal.Snapshot{Pages: map[uint32]wal.PageRef{1: {Frame: 1}}}}
	return s, g, main, current
}
func TestPlainSessionCleanupMemoryCheckpointOnceAndBarrier(t *testing.T) {
	s, g, main, current := plainSessionMemory(t)
	diagnostic := errors.New("memory WAL retirement")
	current.remaining, current.cause = 1, diagnostic
	checkpoints := 0
	token := s.closeLocked(nil, func() error { checkpoints++; return g.closeWAL(nil) })
	if !errors.Is(token, ErrPlaintextCleanupPending) || !errors.Is(token, diagnostic) || checkpoints != 1 || current.calls != 1 || main.calls != 0 {
		t.Fatal(token, checkpoints, current.calls, main.calls)
	}
	if !s.closed || s.file != nil || s.walFile != nil || s.opts != (Options{}) || s.snapshot.Pages != nil {
		t.Fatal("logical state retained")
	}
	if e := s.closeLocked(nil, func() error { checkpoints++; return nil }); e != nil || checkpoints != 1 || !g.done || main.calls != 1 {
		t.Fatal(e, checkpoints)
	}
	if e := RetryPlaintextCleanup(token); e != nil || main.calls != 1 || current.calls != 2 {
		t.Fatal("completed token replay", e)
	}
}
func TestPlainSessionCleanupMemoryBusyKeepAndPriorError(t *testing.T) {
	for _, mode := range []string{"keep", "cause", "poison"} {
		s, g, main, current := plainSessionMemory(t)
		s.active = &Tx{}
		originalOptions := s.opts
		calls := 0
		if e := s.closeLocked(nil, func() error { calls++; return nil }); !errors.Is(e, ErrBusy) || s.closed || s.opts != originalOptions || main.calls != 0 || current.calls != 0 || calls != 0 {
			t.Fatal("BUSY changed session", e)
		}
		s.active = nil
		var cause error
		diagnostic := errors.New("memory prior disposal")
		switch mode {
		case "keep":
			s.opts.KeepWALOnClose = true
		case "cause":
			cause = diagnostic
		case "poison":
			s.poisoned = diagnostic
		}
		e := s.closeLocked(cause, func() error { calls++; return nil })
		if calls != 0 || !g.done || main.calls != 1 || current.calls != 1 {
			t.Fatal("skip policy or retirement", mode, e)
		}
		if mode != "keep" && !errors.Is(e, diagnostic) {
			t.Fatal("prior cause lost", e)
		}
	}
}
func TestPlainSessionCleanupMemoryCandidateSwapOrder(t *testing.T) {
	s, g, main, old := plainSessionMemory(t)
	events := []string{}
	main.events, old.events = &events, &events
	old.remaining, old.cause = 1, errors.New("memory superseded WAL close")
	candidate := &plainMemoryOwner{name: "candidate", events: &events}
	if e := g.reserveTemporary(); e != nil {
		t.Fatal(e)
	}
	// Acquisition callbacks publish into the pre-reserved slot; model owners are
	// installed directly without obtaining any native resource.
	g.temporaryReserved = false
	g.temporary = candidate
	g.replaceWAL()
	if g.wal != candidate || g.temporary != old {
		t.Fatal("replacement lost an owner")
	}
	operation := errors.New("memory directory diagnostic")
	pending := g.closeTemporary(operation)
	if !errors.Is(pending, old.cause) || !errors.Is(pending, operation) || candidate.calls != 0 || main.calls != 0 {
		t.Fatal(pending)
	}
	current := s.closeLocked(pending, func() error { t.Fatal("poison checkpoint"); return nil })
	if old.calls != 1 || !errors.Is(current, old.cause) || !errors.Is(current, operation) {
		t.Fatal("close retried or lost first error", current)
	}
	if e := RetryPlaintextCleanup(current); e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(events, []string{"wal", "wal", "candidate", "main"}) || !g.done {
		t.Fatal(events)
	}
}
func TestPlainSessionCleanupMemoryReservationAndBorrowedJournal(t *testing.T) {
	s, g, main, current := plainSessionMemory(t)
	if e := g.reserveWAL(); !errors.Is(e, ErrLimit) {
		t.Fatal("overwrote current WAL", e)
	}
	if e := g.reserveTemporary(); e != nil {
		t.Fatal(e)
	}
	if e := g.reserveTemporary(); !errors.Is(e, ErrLimit) {
		t.Fatal(e)
	}
	g.acquiredTemporary(nil)
	if g.temporaryReserved {
		t.Fatal("unused reservation retained")
	}
	tx := &Tx{file: &vfs.File{}, borrowedCleanup: g, keepFile: true, dirty: map[uint32][]byte{1: {1}}, free: map[uint32]struct{}{}}
	if _, e := tx.newUndoMarkLocked("memory"); e != nil {
		t.Fatal(e)
	}
	e := tx.poison(errors.New("memory transition failure"), true)
	if !errors.Is(e, ErrPoisoned) || !errors.Is(e, ErrRecoveryRequired) || !tx.LogicallyClosed() || len(tx.undo.marks) != 0 || main.calls != 0 || current.calls != 0 {
		t.Fatal(e)
	}
	if e = s.closeLocked(e, func() error { t.Fatal("failed transition checkpoint"); return nil }); !errors.Is(e, ErrRecoveryRequired) || !g.done {
		t.Fatal(e)
	}
}
func TestPlainSessionCleanupMemoryFourStageOutcomes(t *testing.T) {
	for bits := 0; bits < 256; bits++ {
		r := &plainRetirementRegistry{limit: 1}
		g, e := r.reserve()
		if e != nil {
			t.Fatal(e)
		}
		owners := make([]*plainMemoryOwner, 4)
		firstBlocked := 4
		for i, name := range []string{"nested", "temporary", "wal", "main"} {
			mode := (bits >> (i * 2)) & 3
			o := &plainMemoryOwner{name: name, cause: errors.New("memory " + name)}
			switch mode {
			case 1:
				o.consumed = true
			case 2:
				o.nilPending = true
			case 3:
				o.terminal = true
			}
			if mode >= 2 && firstBlocked == 4 {
				firstBlocked = i
			}
			owners[i] = o
		}
		g.nested = []plainResource{owners[0]}
		g.temporary, g.wal, g.main = owners[1], owners[2], owners[3]
		token := g.begin(nil)
		if errors.Is(token, ErrPlaintextCleanupPending) != (firstBlocked < 4) {
			t.Fatal(bits, token)
		}
		firstCause := g.cause
		if firstBlocked < 4 {
			for i := 0; i < 3; i++ {
				token = RetryPlaintextCleanup(token)
				if g.cause != firstCause {
					t.Fatal("unbounded retry history")
				}
			}
		}
		for i, o := range owners {
			if i > firstBlocked && o.calls != 0 {
				t.Fatal("protector retired below incomplete child", bits, i)
			}
			if o.terminal && o.rawCalls > 1 {
				t.Fatal("terminal native identity retried")
			}
			if o.terminal || o.nilPending {
				o.done = true
			}
		}
		if e = RetryPlaintextCleanup(token); errors.Is(e, ErrPlaintextCleanupPending) || !g.done || len(r.groups) != 0 {
			t.Fatal(bits, e)
		}
	}
}

func TestPlainSessionCleanupMemoryAdmissionBeforePath(t *testing.T) {
	var groups []*plainRetirement
	defer func() {
		for _, g := range groups {
			_ = g.begin(nil)
		}
	}()
	for i := 0; i < maxPlainRetirements; i++ {
		g, e := plainRetirements.reserve()
		if e != nil {
			t.Fatal("unexpected live scope", i, e)
		}
		groups = append(groups, g)
	}
	// Invalid input must never reach path handling because admission is full.
	s, e := OpenSession(context.Background(), "\x00", Options{Create: true})
	if s != nil || !errors.Is(e, ErrLimit) || len(PendingPlaintextCleanups()) != 0 {
		t.Fatal("acquisition preceded admission", e)
	}
}

func TestPlainSessionCleanupMemoryPublicAdmissionTransfer(t *testing.T) {
	a, err := ReserveExclusiveOpen()
	if err != nil {
		t.Fatal(err)
	}
	g := a.group.Load()
	owner := &plainMemoryOwner{remaining: 1, cause: errors.New("memory public metadata retirement")}
	g.nested = []plainResource{owner}
	original := errors.New("memory public path error")
	pending, handled := a.Finish(original)
	if !handled || !errors.Is(pending, ErrPlaintextCleanupPending) || !errors.Is(pending, original) || len(PendingPlaintextCleanups()) != 1 {
		t.Fatal("no-DB error not discoverable", pending)
	}
	if _, handled = a.Finish(original); handled || owner.calls != 1 {
		t.Fatal("admission finalized twice")
	}
	if e := RetryPlaintextCleanup(PendingPlaintextCleanups()[0]); e != nil || !g.done {
		t.Fatal(e)
	}
	a, err = ReserveExclusiveOpen()
	if err != nil {
		t.Fatal(err)
	}
	transferred, err := claimExclusiveOpen(a)
	if err != nil {
		t.Fatal(err)
	}
	if _, handled = a.Finish(original); handled {
		t.Fatal("transferred owner retired by public defer")
	}
	if _, err = claimExclusiveOpen(a); !errors.Is(err, ErrMisuse) {
		t.Fatal("admission claimed twice", err)
	}
	if e := transferred.begin(nil); e != nil {
		t.Fatal(e)
	}
}
