package pager

import (
	"context"
	"errors"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

// Uses only the descriptor-free owners from plain_cleanup_test.go. Placeholder
// File values never reach a native operation in these production-helper laws.
func TestPlainTxCleanupMemoryJournalReservation(t *testing.T) {
	reg := &plainRetirementRegistry{limit: 1}
	g, e := reg.reserve()
	if e != nil {
		t.Fatal(e)
	}
	main := &plainMemoryOwner{name: "main"}
	g.main = main
	if e = g.reserveTemporary(); e != nil || !g.temporaryReserved {
		t.Fatal(e)
	}
	if e = g.reserveTemporary(); !errors.Is(e, ErrLimit) {
		t.Fatal("duplicate journal admission", e)
	}
	g.acquiredTemporary(nil) // an ordinary no-resource acquisition error frees slot
	if e = g.reserveTemporary(); e != nil {
		t.Fatal(e)
	}
	g.acquiredTemporary(&vfs.File{}) // nil-raw bookkeeping is already retired
	if e = g.reserveTemporary(); !errors.Is(e, ErrLimit) {
		t.Fatal("existing child slot reused", e)
	}
	if e = g.closeTemporary(nil); e != nil || main.calls != 0 || g.temporary != nil {
		t.Fatal(e)
	}
	if e = g.begin(nil); e != nil || !g.done {
		t.Fatal(e)
	}
}
func TestPlainTxCleanupMemoryJournalBarrierAndDeferredWholeClose(t *testing.T) {
	for _, nilPending := range []bool{false, true} {
		reg := &plainRetirementRegistry{limit: 1}
		g, e := reg.reserve()
		if e != nil {
			t.Fatal(e)
		}
		main := &plainMemoryOwner{name: "main"}
		journal := &plainMemoryOwner{name: "journal", remaining: 1, nilPending: nilPending, cause: errors.New("memory journal remains")}
		g.main, g.temporary = main, journal
		txx := &Tx{file: &vfs.File{}, cleanup: g, dirty: map[uint32][]byte{1: {1}}, free: map[uint32]struct{}{2: {}}}
		operation := errors.New("memory commit operation")
		e = g.closeTemporary(operation)
		if !errors.Is(e, ErrPlaintextCleanupPending) || main.calls != 0 || journal.calls != 1 {
			t.Fatal(e)
		}
		token := txx.deferPlainRetirementLocked(true)
		if !errors.Is(token, ErrRecoveryRequired) || !errors.Is(token, ErrPlaintextCleanupPending) || !errors.Is(token, operation) || journal.calls != 1 || main.calls != 0 {
			t.Fatal("defer repeated child or lost cause", token)
		}
		if !txx.LogicallyClosed() || txx.dirty != nil || txx.free != nil {
			t.Fatal("logical state retained")
		}
		if _, e = txx.ReadPage(context.Background(), 1); !errors.Is(e, ErrClosed) {
			t.Fatal(e)
		}
		journal.nilPending = false
		if nilPending {
			journal.remaining = 0
		}
		if e = txx.Rollback(); e != nil || !g.done || journal.calls != 2 || main.calls != 1 {
			t.Fatal(e)
		}
		if e = txx.Rollback(); e != nil || main.calls != 1 {
			t.Fatal(e)
		}
		if !errors.Is(token, ErrPlaintextCleanupPending) {
			t.Fatal("old token changed")
		}
	}
}
func TestPlainTxCleanupMemoryLiveRetryPreservesUnrequestedJournal(t *testing.T) {
	reg := &plainRetirementRegistry{limit: 1}
	g, child, main, _ := plainMemoryGroup(t, reg)
	journal := &plainMemoryOwner{name: "journal"}
	g.temporary = journal
	g.cause = errors.Join(ErrPoisoned, child.cause)
	token := g.pendingLocked(g.cause)
	if e := RetryPlaintextCleanup(token); e != nil || main.calls != 0 || journal.calls != 0 {
		t.Fatal("live retry retired active transaction", e)
	}
	if e := g.begin(nil); !errors.Is(e, ErrPoisoned) || errors.Is(e, ErrPlaintextCleanupPending) || !g.done || journal.calls != 1 || main.calls != 1 {
		t.Fatal(e)
	}
}
func TestPlainTxCleanupMemoryConsumedCommitDiagnosticAndPoison(t *testing.T) {
	for _, pending := range []bool{false, true} {
		reg := &plainRetirementRegistry{limit: 1}
		g, e := reg.reserve()
		if e != nil {
			t.Fatal(e)
		}
		main := &plainMemoryOwner{consumed: true, cause: errors.New("memory consumed main diagnostic")}
		if pending {
			main.remaining = 1
		}
		g.main = main
		txx := &Tx{file: &vfs.File{}, cleanup: g, dirty: map[uint32][]byte{1: {1}}, free: map[uint32]struct{}{2: {}}}
		e = txx.retirePlainLocked(nil, errors.Join(ErrPoisoned, ErrCommitUncertain), true)
		if !errors.Is(e, ErrCommitUncertain) || !errors.Is(e, ErrPoisoned) || !errors.Is(e, main.cause) || !txx.LogicallyClosed() || txx.dirty != nil || txx.free != nil {
			t.Fatal(e)
		}
		if errors.Is(e, ErrPlaintextCleanupPending) != pending {
			t.Fatal("consumption misclassified", e)
		}
		if pending {
			e = txx.Rollback()
			if !errors.Is(e, main.cause) || errors.Is(e, ErrPlaintextCleanupPending) || !g.done {
				t.Fatal("completion retained historical pending", e)
			}
		}
		if e = txx.Rollback(); e != nil {
			t.Fatal(e)
		}
	}
	reg := &plainRetirementRegistry{limit: 1}
	g, child, main, _ := plainMemoryGroup(t, reg)
	child.remaining = 1
	txx := &Tx{file: &vfs.File{}, cleanup: g, dirty: map[uint32][]byte{1: {1}}}
	original := errors.New("memory original I/O diagnostic")
	e := txx.poison(original, true)
	if !errors.Is(e, ErrRecoveryRequired) || !errors.Is(e, original) || !errors.Is(e, ErrPlaintextCleanupPending) || main.calls != 0 || !txx.LogicallyClosed() {
		t.Fatal(e)
	}
	if e = txx.Rollback(); e != nil || !g.done {
		t.Fatal(e)
	}
}
func TestPlainTxCleanupMemoryRollbackDropsUndoBeforeCompletion(t *testing.T) {
	reg := &plainRetirementRegistry{limit: 1}
	g, e := reg.reserve()
	if e != nil {
		t.Fatal(e)
	}
	main := &plainMemoryOwner{remaining: 1, cause: errors.New("memory main pending")}
	g.main = main
	txx := &Tx{file: &vfs.File{}, cleanup: g, dirty: map[uint32][]byte{1: {1}}, free: map[uint32]struct{}{2: {}}}
	if _, e = txx.newUndoMarkLocked("memory mark"); e != nil {
		t.Fatal(e)
	}
	if e = txx.Rollback(); !errors.Is(e, ErrPlaintextCleanupPending) || len(txx.undo.marks) != 0 || txx.dirty != nil || txx.free != nil {
		t.Fatal(e)
	}
	if e = txx.Rollback(); e != nil {
		t.Fatal(e)
	}
}

func TestPlainTxCleanupMemoryPreservesOrdinaryErrorStructure(t *testing.T) {
	cause := errors.New("memory ordinary diagnostic")
	if got := plainDiagnostic(cause); got != cause {
		t.Fatal("ordinary diagnostic structure hidden", got)
	}
}

// Regression laws from the independent TxV1 review. All owners are memory-only.
func TestPlainTxCleanupMemoryDeferredDiagnosticAndRecovery(t *testing.T) {
	reg := &plainRetirementRegistry{limit: 1}
	g, err := reg.reserve()
	if err != nil {
		t.Fatal(err)
	}
	operation, retirement := errors.New("memory operation"), errors.New("memory journal retirement")
	main, journal := &plainMemoryOwner{}, &plainMemoryOwner{remaining: 1, cause: retirement}
	g.main, g.temporary = main, journal
	tx := &Tx{file: &vfs.File{}, cleanup: g}
	first := g.closeTemporary(operation)
	current := tx.deferPlainRetirementLocked(true)
	for _, e := range []error{first, current, reg.snapshot()[0]} {
		if !errors.Is(e, operation) || !errors.Is(e, retirement) || !errors.Is(e, ErrPlaintextCleanupPending) {
			t.Fatal("first diagnostic lost", e)
		}
	}
	if !errors.Is(current, ErrRecoveryRequired) || journal.calls != 1 || main.calls != 0 {
		t.Fatal("pending journal lost recovery/barrier", current)
	}
	cause := g.cause
	_ = RetryPlaintextCleanup(current)
	if !g.done || g.cause != nil || cause == nil {
		t.Fatal("retirement incomplete")
	}
}
func TestPlainTxCleanupMemoryPoisonClearsUndoWithoutOuterDefer(t *testing.T) {
	reg := &plainRetirementRegistry{limit: 1}
	g, err := reg.reserve()
	if err != nil {
		t.Fatal(err)
	}
	g.main = &plainMemoryOwner{remaining: 1, cause: errors.New("memory main pending")}
	tx := &Tx{file: &vfs.File{}, cleanup: g, dirty: map[uint32][]byte{1: {1}}, free: map[uint32]struct{}{2: {}}}
	if _, err = tx.newUndoMarkLocked("memory mark"); err != nil {
		t.Fatal(err)
	}
	_ = tx.poison(errors.New("memory exclusive poison"), false)
	if !tx.LogicallyClosed() || len(tx.undo.marks) != 0 || tx.dirty != nil || tx.free != nil {
		t.Fatal("logical poison retained transaction data")
	}
	_ = g.retry()
}
