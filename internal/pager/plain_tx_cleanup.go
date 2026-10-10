package pager

import (
	"errors"
	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

// The scope's fixed temporary slot is reserved before any rollback-journal
// acquisition. No native work, status call or scope wait occurs under admission.
func (g *plainRetirement) reserveTemporary() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.started || g.done {
		return ErrClosed
	}
	if g.cause != nil {
		return g.cause
	}
	if g.temporaryReserved || g.temporary != nil {
		return ErrLimit
	}
	g.temporaryReserved = true
	return nil
}
func (g *plainRetirement) acquiredTemporary(file *vfs.File) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.temporaryReserved = false
	if file != nil {
		g.temporary = plainFile{file}
	}
}

// Operation causes are adopted before closing the journal. The already-owned
// temporary slot, rather than a discarded local variable, carries any close
// obligation. An incomplete child leaves main untouched; the caller then starts
// logical whole closure without automatically repeating this same close attempt.
func (g *plainRetirement) closeTemporary(cause error) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	cause = g.adoptLocked(cause)
	if cause != nil && g.cause == nil {
		g.cause = plainDiagnostic(errors.Join(ErrPoisoned, cause))
	}
	firstAttempt := !g.temporaryRequested
	g.temporaryRequested = true
	err := g.retireLocked()
	if len(g.nested) != 0 || g.temporary != nil {
		if firstAttempt {
			// Preserve the first operation AND child-retirement diagnostics when
			// the caller starts whole closure; later retries do not grow history.
			g.cause = plainDiagnostic(errors.Join(g.cause, ErrPoisoned, err))
		}
		return g.pendingLocked(errors.Join(g.cause, plainDiagnostic(err)))
	}
	return errors.Join(cause, err)
}

// Adopted historical tokens remain diagnostics without making a newly completed
// result appear pending. Their original Is/As causes remain accessible.
type plainDiagnosticError struct{ cause error }

func (e *plainDiagnosticError) Error() string { return e.cause.Error() }
func (e *plainDiagnosticError) Is(target error) bool {
	return target != ErrPlaintextCleanupPending && errors.Is(e.cause, target)
}
func (e *plainDiagnosticError) As(target any) bool { return errors.As(e.cause, target) }
func plainDiagnostic(err error) error {
	if err == nil || !errors.Is(err, ErrPlaintextCleanupPending) {
		return err
	}
	return &plainDiagnosticError{err}
}

func (t *Tx) retirePlainLocked(cause, failureClass error, attempt bool) error {
	t.file, t.dirty, t.free = nil, nil, nil
	t.clearClosedUndoLocked()
	err := t.cleanup.beginClass(cause, failureClass, attempt)
	t.poisoned = err
	return err
}
func (t *Tx) deferPlainRetirementLocked(recovery bool) error {
	cause := ErrPoisoned
	if recovery {
		cause = errors.Join(cause, ErrRecoveryRequired)
	}
	if t.borrowedCleanup != nil {
		t.file, t.dirty, t.free = nil, nil, nil
		t.clearClosedUndoLocked()
		t.poisoned = errors.Join(cause, t.borrowedCleanup.check())
		return t.poisoned
	}
	return t.retirePlainLocked(cause, nil, false)
}

// LogicallyClosed distinguishes an ended transaction from a retryable BUSY
// commit. It performs no native work and does not imply physical retirement.
func (t *Tx) LogicallyClosed() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.file == nil && t.encrypted == nil
}

func (t *Tx) plainScope() *plainRetirement {
	if t.cleanup != nil {
		return t.cleanup
	}
	if t.borrowedCleanup != nil {
		return t.borrowedCleanup
	}
	if t.session != nil {
		return t.session.cleanup
	}
	return nil
}
