package pager

import (
	"errors"
	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

type plainBackupSource struct{ namespace *vfs.BackupSourceNamespace }
func (o plainBackupSource) Close() error { return nativeFileError(o.namespace.Close()) }
func (o plainBackupSource) RetirementComplete() bool { return o.namespace.RetirementComplete() }

// BeginBackupNamespace reserves and installs the source-only owner before any
// retained-parent acquisition. The proof stays alive during full Check and copy.
func (r *Reader) BeginBackupNamespace() (*vfs.BackupSourceNamespace, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.lease != nil { return nil, ErrUnsupported }
	if r.closed || r.file == nil { return nil, ErrClosed }
	if err := r.cleanup.check(); err != nil { return nil, err }
	if err := r.cleanup.reserveTemporary(); err != nil { return nil, err }
	proof := vfs.NewBackupSourceNamespace()
	r.cleanup.mu.Lock()
	r.cleanup.temporaryReserved = false
	r.cleanup.temporary = plainBackupSource{namespace: proof}
	r.cleanup.mu.Unlock()
	if err := proof.Acquire(r.file); err != nil {
		// End this live temporary scope once. A later DB-specific close must
		// not repeat an incomplete close in the same Backup operation.
		return nil, r.closeFailedBackupNamespace(proof, err)
	}
	return proof, nil
}
// Reader.mu is held by the sole caller, the failed Acquire branch. Bind both
// the status and temporary owner to that same proof, and the main to r.file.
func (r *Reader) closeFailedBackupNamespace(proof *vfs.BackupSourceNamespace, cause error) error {
	cause = nativeFileError(cause)
	benign := proof.BenignAcquisitionRefusal()
	r.cleanup.mu.Lock()
	defer r.cleanup.mu.Unlock()
	return r.cleanup.closeFailedBackupSourceLocked(plainBackupSource{namespace: proof}, plainFile{file: r.file}, cause, benign)
}

// This source-only transition never starts whole-reader retirement. It avoids
// introducing poison on one proved-clean refusal; it never clears old poison.
// expected/main are comparable private owners supplied by the adapter above;
// descriptor-free models use pointer resources at this same state boundary.
func (g *plainRetirement) closeFailedBackupSourceLocked(expected, main plainResource, cause error, benign bool) error {
	originalCause, originalPending := g.cause, g.pending.Load()
	originalNested := len(g.nested)
	sameOwner := expected != nil && main != nil && g.temporary == expected && g.main == main
	cause = g.adoptLocked(cause)
	adoptedOwnership := len(g.nested) != originalNested
	// Acquisition diagnostics normally contain no exposed native owners: the
	// namespace facade retains them. If that contract is violated, an adopted
	// owner might alias main; never let generic nested-first retirement guess.
	if len(g.nested) != 0 { g.nested = append([]plainResource{plainTerminal{ErrPoisoned}}, g.nested...) }
	canAttempt := sameOwner && !adoptedOwnership && !g.started && !g.done && !g.temporaryRequested && !g.temporaryReserved && originalPending == nil && originalNested == 0 && g.wal == nil && !g.walReserved && !g.walRequested
	if !canAttempt {
		if sameOwner && len(g.nested) != 0 { g.temporaryRequested = true }
		// A substituted owner or already-ended scope cannot authorize a close.
		// Preserve the unexpected supplied proof too, behind a terminal barrier.
		if expected != nil && g.temporary != expected && g.main != expected {
			g.nested = append(g.nested, plainTerminal{ErrMisuse}, expected)
		}
		g.cause = plainDiagnostic(errors.Join(originalCause, ErrPoisoned, ErrMisuse, cause))
		if g.temporary != nil || len(g.nested) != 0 { return g.pendingLocked(g.cause) }
		return g.cause
	}
	cleanEntry := benign && originalCause == nil && !adoptedOwnership && !main.RetirementComplete() && !expected.RetirementComplete() && errors.Is(cause, ErrUnsupported) && !errors.Is(cause, ErrPoisoned) && !errors.Is(cause, ErrChanged) && !errors.Is(cause, vfs.ErrIdentity) && !errors.Is(cause, vfs.ErrPoisoned) && !errors.Is(cause, ErrCommitUncertain) && !errors.Is(cause, ErrClosed) && !errors.Is(cause, ErrMisuse) && !errors.Is(cause, ErrLimit)
	g.temporaryRequested = true
	retirementErr := g.retireLocked() // g.started is false: source main stays held
	if cleanEntry && retirementErr == nil && g.temporary == nil && len(g.nested) == 0 && !g.temporaryRequested && !g.temporaryReserved && g.main == main && !g.started && !g.done && g.cause == nil && g.pending.Load() == nil {
		return plainDiagnostic(cause)
	}
	g.cause = plainDiagnostic(errors.Join(originalCause, ErrPoisoned, cause, retirementErr))
	if g.temporary != nil || len(g.nested) != 0 { return g.pendingLocked(g.cause) }
	return g.cause
}
func (r *Reader) ObserveBackupNamespace(proof *vfs.BackupSourceNamespace) (vfs.BackupSourceIdentity, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || r.file == nil { return vfs.BackupSourceIdentity{}, ErrClosed }
	if err := r.cleanup.check(); err != nil { return vfs.BackupSourceIdentity{}, err }
	r.cleanup.mu.Lock()
	owned, ok := r.cleanup.temporary.(plainBackupSource)
	r.cleanup.mu.Unlock()
	if !ok || owned.namespace != proof { return vfs.BackupSourceIdentity{}, ErrMisuse }
	identity, err := proof.ObserveDirectSourceFamily(r.file)
	return identity, r.cleanup.observe(nativeFileError(err))
}
func (r *Reader) EndBackupNamespace(proof *vfs.BackupSourceNamespace) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.cleanup.mu.Lock()
	owned, ok := r.cleanup.temporary.(plainBackupSource)
	r.cleanup.mu.Unlock()
	if !ok || owned.namespace != proof { return ErrMisuse }
	return r.cleanup.closeTemporary(nil)
}

// CloseBackupReader closes the DB-created Reader without a second immediate
// attempt on a source-proof close that already remained pending in this call.
// Explicit RetryPlaintextCleanup can later retire represented native owners.
// Ordinary Reader.Close and all non-Backup lifecycle behavior are unchanged.
func (r *Reader) CloseBackupReader() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cleanup == nil { return ErrMisuse }
	r.cleanup.mu.Lock()
	alreadyAttempted := r.cleanup.temporaryRequested && r.cleanup.temporary != nil
	r.cleanup.mu.Unlock()
	r.closed = true
	r.file = nil
	return r.cleanup.beginClass(nil, nil, !alreadyAttempted)
}
