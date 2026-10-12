package vfs

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sync"
)

// backupPublicationIO is the production transition boundary. Its native
// implementation holds only fixed native owners and bounded naming metadata;
// descriptor-free tests implement these exact methods with inert event records.
// No source Reader, callback, context or page buffer is retained by the owner.
type backupPublicationIO interface {
	openDirectory(string) (backupFact, error)
	absentFamily(string) error
	createCandidate(string) error
	lockCandidate() error
	validateCandidate() (uint64, error)
	writeCandidate([]byte, int64) (int, error)
	syncCandidate() error
	publish(string, string) error
	validatePublished() error
	syncDirectory() error
	seal() error
	closeDirectory() error
	directoryComplete() bool
	closeMain() error
	mainComplete() bool
	ownedFiles() []*File
}

// BackupPublicationStatus reports operation facts independently of cleanup.
// Completed means represented native ownership is retired, not that an earlier
// failed operation succeeded or that this unqualified profile is crash-safe.
type BackupPublicationStatus struct {
	MutationAttempted, CandidateLocked, CopyComplete bool
	DataSyncAttempted, DataSynced bool
	PublicationAttempted, Published bool
	DirectorySyncAttempted, DirectorySynced bool
	Sealed, Completed bool
}

// BackupPublication is a single-use composite destination owner. Install it in
// a bounded plaintext retirement group BEFORE calling OpenBackupDestination.
// The memory-only constructor performs no acquisition. Close seals operations,
// drops namespace lifetime pins without closing/unlocking main, retires the
// directory natively, and only then permits the protecting main to retire.
// It is neither a four-name reservation nor a general untrusted-namespace API.
type BackupPublication struct {
	mu sync.Mutex
	ops backupPublicationIO
	status BackupPublicationStatus
	opened, failed, terminal bool
	final, candidate string
	expected, written int64
	unsafe error
}
func NewBackupPublicationOwner() *BackupPublication {
	return &BackupPublication{ops: newNativeBackupPublication()}
}
func (p *BackupPublication) diagnostic(err error) error {
	if err != nil { p.failed = true } // historical close diagnostics also forbid success
	diagnostic, terminal := backupError(err, p.ops.ownedFiles()...)
	if terminal {
		p.terminal = true
		if p.unsafe == nil { p.unsafe = err }
	}
	return diagnostic
}
func (p *BackupPublication) fail(err error) error {
	p.failed = true
	return p.diagnostic(err)
}
func (p *BackupPublication) active() error {
	if p.status.Sealed || p.status.Completed || p.failed || p.terminal { return ErrPoisoned }
	return nil
}
func (p *BackupPublication) Status() BackupPublicationStatus {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status
}

func backupDestinationAdmission(source BackupSourceIdentity, parent backupFact, final, candidate string) error {
	if !source.valid || !source.parent.complete() || !parent.complete() || parent.mode&backupTypeMask != backupDirectory {
		return ErrUnsupported
	}
	if !BackupBasename(source.base) || !BackupBasename(final) || !BackupBasename(candidate) { return ErrUnsupported }
	if !backupFamiliesDisjoint(final, candidate) { return ErrUnsupported }
	// Physical equality matters even across whole-directory bind aliases with
	// different mount IDs. A mount-ID difference does not prove disjointness.
	if source.parent.samePhysical(parent) && (!backupFamiliesDisjoint(source.base, final) || !backupFamiliesDisjoint(source.base, candidate)) {
		return ErrUnsupported
	}
	return nil
}
func (p *BackupPublication) OpenBackupDestination(parent string, final string, source BackupSourceIdentity, size int64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.active(); err != nil { return err }
	if p.opened || !source.valid || !BackupBasename(final) || size <= 0 { return p.fail(ErrUnsupported) }
	var random [12]byte
	if _, err := rand.Read(random[:]); err != nil { return p.fail(err) }
	return p.openDestinationLocked(parent, final, source, size, ".gosqlite-backup-"+hex.EncodeToString(random[:]))
}
func (p *BackupPublication) openDestinationLocked(parent, final string, source BackupSourceIdentity, size int64, candidate string) error {
	if err := p.active(); err != nil { return err }
	if p.opened || !source.valid || !BackupBasename(final) || !BackupBasename(candidate) || size <= 0 { return p.fail(ErrUnsupported) }
	p.opened = true // at most one directory acquisition, including a failed one
	p.final, p.candidate, p.expected = final, candidate, size
	parentIdentity, err := p.ops.openDirectory(parent)
	if err != nil { return p.fail(err) }
	if err = backupDestinationAdmission(source, parentIdentity, p.final, p.candidate); err != nil { return p.fail(err) }
	if err = p.ops.absentFamily(p.final); err != nil { return p.fail(err) }
	if err = p.ops.absentFamily(p.candidate); err != nil { return p.fail(err) }
	return nil
}
func (p *BackupPublication) CreatePrivateCandidate() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.active(); err != nil { return err }
	if !p.opened || p.status.MutationAttempted { return p.fail(ErrLock) }
	p.status.MutationAttempted = true
	if err := p.ops.createCandidate(p.candidate); err != nil { return p.fail(err) }
	if err := p.ops.lockCandidate(); err != nil { return p.fail(err) }
	p.status.CandidateLocked = true
	if err := p.ops.absentFamily(p.final); err != nil { return p.fail(err) }
	return nil
}
func (p *BackupPublication) WritePage(page []byte, offset int64) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.active(); err != nil { return err }
	if !p.status.CandidateLocked || p.status.DataSyncAttempted || len(page) == 0 || offset != p.written || offset < 0 || int64(len(page)) > p.expected-offset {
		return p.fail(ErrLock)
	}
	if _, err := p.ops.validateCandidate(); err != nil { return p.fail(err) }
	n, err := p.ops.writeCandidate(page, offset)
	if err != nil { return p.fail(err) }
	if n != len(page) { return p.fail(io.ErrShortWrite) }
	p.written += int64(n)
	p.status.CopyComplete = p.written == p.expected
	return nil
}
func (p *BackupPublication) SyncData() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.active(); err != nil { return err }
	if !p.status.CandidateLocked || !p.status.CopyComplete || p.status.DataSyncAttempted { return p.fail(ErrLock) }
	size, err := p.ops.validateCandidate()
	if err != nil { return p.fail(err) }
	if size != uint64(p.expected) { return p.fail(ErrIdentity) }
	if err = p.ops.absentFamily(p.final); err != nil { return p.fail(err) }
	p.status.DataSyncAttempted = true
	if err = p.ops.syncCandidate(); err != nil { return p.fail(err) }
	p.status.DataSynced = true
	return nil
}
func (p *BackupPublication) PublishNoReplace() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.active(); err != nil { return err }
	if !p.status.CandidateLocked || !p.status.CopyComplete || !p.status.DataSynced || p.status.PublicationAttempted { return p.fail(ErrLock) }
	if _, err := p.ops.validateCandidate(); err != nil { return p.fail(err) }
	if err := p.ops.absentFamily(p.final); err != nil { return p.fail(err) }
	p.status.PublicationAttempted = true // consume the permit even on failure
	if err := p.ops.publish(p.candidate, p.final); err != nil { return p.fail(err) }
	p.status.Published = true
	return nil
}
func (p *BackupPublication) SyncPublication() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if err := p.active(); err != nil { return err }
	if !p.status.Published || p.status.DirectorySyncAttempted { return p.fail(ErrLock) }
	p.status.DirectorySyncAttempted = true
	validation := p.ops.validatePublished()
	syncErr := p.ops.syncDirectory()
	p.status.DirectorySynced = syncErr == nil
	if err := errors.Join(validation, syncErr); err != nil { return p.fail(err) }
	return nil
}
func (p *BackupPublication) sealLocked() error {
	if p.terminal { return ErrPoisoned }
	if p.status.Sealed { return nil }
	// The same gate serializes all namespace/I/O calls, so no admitted operation
	// can survive this memory-only seal or use a detached directory lifetime pin.
	if err := p.ops.seal(); err != nil { p.terminal = true; return p.diagnostic(err) }
	p.status.Sealed = true
	return nil
}
func (p *BackupPublication) SealForRetirement() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.sealLocked()
}
func (p *BackupPublication) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.status.Completed { return nil }
	if err := p.sealLocked(); err != nil { return err }
	var errs []error
	if !p.ops.directoryComplete() {
		if err := p.ops.closeDirectory(); err != nil { errs = append(errs, p.diagnostic(err)) }
	}
	if p.terminal || !p.ops.directoryComplete() {
		p.failed = true
		return errors.Join(append(errs, ErrPoisoned)...)
	}
	if !p.ops.mainComplete() {
		if err := p.ops.closeMain(); err != nil { errs = append(errs, p.diagnostic(err)) }
	}
	if p.terminal || !p.ops.mainComplete() {
		p.failed = true
		return errors.Join(append(errs, ErrPoisoned)...)
	}
	p.status.Completed = true
	return errors.Join(errs...)
}
func (p *BackupPublication) RetirementComplete() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.status.Completed
}

// SuccessfulPublication includes every operation fact, but not source-owner
// completion: the caller must separately retire its own proof/source owner.
func (p *BackupPublication) SuccessfulPublication() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.failed || p.terminal || !p.status.DataSynced || !p.status.Published || !p.status.DirectorySynced || !p.status.Completed {
		return fmt.Errorf("%w: backup publication did not complete", ErrPoisoned)
	}
	return nil
}
