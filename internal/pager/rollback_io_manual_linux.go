//go:build linux && manual_fault_rollback

package pager

import (
	"errors"
	"io"
	"os"
	"sync"

	"github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackfault"
	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

var errRollbackFaultGate = errors.New("rollback fault adapter requires separate authorization")
var errRollbackFaultScope = errors.New("rollback fault adapter scope not qualified")
var errRollbackFaultInjected = errors.New("rollback fault adapter injected I/O error")

// This is one lifetime slot, not an assignable VFS callback or a public option.
// Even a successfully disarmed scope cannot be reused for another trial.
var rollbackFaultRegistry struct {
	mu     sync.Mutex
	used   bool
	active *rollbackFaultScope
}

type rollbackFaultScope struct {
	mu               sync.Mutex
	tx               *Tx
	main             *vfs.File
	journal          *vfs.File
	cleanup          *plainRetirement
	plan             rollbackfault.Plan
	receipt          rollbackfault.Receipt
	counts           [14]uint8
	events           uint8
	diagnostics      [rollbackfault.MaxEvents]error
	problem          error
	firstError       bool
	inFlight         bool
	finalized        bool
	retired          bool
	journalAbsent    bool
	journalUnknown   bool
	targetError      error
	targetErrorToken uint64
}

type rollbackFaultTicket struct {
	scope  *rollbackFaultScope
	call   rollbackfault.Call
	action rollbackfault.Action
}

type rollbackFaultReport struct {
	Observation rollbackfault.Observation
	Events      [rollbackfault.MaxEvents]rollbackfault.Event
	EventCount  uint8
	Diagnostics [rollbackfault.MaxEvents]error
	CommitError error
	Problem     error
}

// No caller is supplied in this source slice. The future separately reviewed
// caller must gate before fixture or owner work, invoke Commit exactly once,
// and supply its real return to finishRollbackFault. Neither tags nor this
// environment value are execution authorization or physical validation.
func armRollbackFault(t *Tx, plan rollbackfault.Plan) (*rollbackFaultScope, error) {
	if os.Getenv("GOSQLITE_MANUAL_FAULT_ROLLBACK") != "independently_authorized" {
		return nil, errRollbackFaultGate
	}
	if t == nil {
		return nil, errRollbackFaultScope
	}
	spec := plan.Spec()
	receipt, problem := rollbackfault.NewReceipt(plan)
	if problem != rollbackfault.NoProblem {
		return nil, errRollbackFaultScope
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.session != nil || t.shared != nil || t.encrypted != nil || t.keepFile || t.borrowedCleanup != nil || t.readOnly || t.statement || t.poisoned != nil || t.file == nil || t.cleanup == nil || t.path == "" || t.info == nil {
		return nil, errRollbackFaultScope
	}
	if t.header.PageSize != 4096 || t.opts.SectorSize != 4096 || t.header.ReadVersion != 1 || t.header.WriteVersion != 1 || t.originalPages == 0 || t.originalPages > 4 || t.header.PageCount == 0 || t.header.PageCount > 4 || len(t.dirty) == 0 || len(t.dirty) > 4 || len(t.dirty[1]) != 4096 {
		return nil, errRollbackFaultScope
	}
	for page, data := range t.dirty {
		if page == 0 || page > t.header.PageCount || len(data) != 4096 {
			return nil, errRollbackFaultScope
		}
	}
	dirty := sortedPages(t.dirty)
	originals := t.journalPages(dirty)
	if len(originals) == 0 || len(originals) > 4 || int(spec.OriginalPages) != len(originals) || int(spec.DirtyPages) != len(dirty) {
		return nil, errRollbackFaultScope
	}
	// Bind the frozen manifest to this Tx's exact page order, not a similar path.
	for i := 0; i < int(spec.CallCount); i++ {
		call := spec.Calls[i]
		if call.Point == rollbackfault.DBOriginalRead {
			index := int(call.Occurrence) - 1
			if index < 0 || index >= len(originals) || call.Offset != int64(originals[index]-1)*4096 {
				return nil, errRollbackFaultScope
			}
		}
		if call.Point == rollbackfault.DBPage {
			index := int(call.Occurrence) - 1
			if index < 0 || index >= len(dirty) || call.Offset != int64(dirty[index]-1)*4096 {
				return nil, errRollbackFaultScope
			}
		}
	}
	g := t.cleanup
	g.mu.Lock()
	main, concrete := g.main.(plainFile)
	healthy := concrete && main.file == t.file && !g.started && !g.done && g.cause == nil && g.pending.Load() == nil && g.temporary == nil && !g.temporaryReserved && !g.temporaryRequested && g.wal == nil && !g.walReserved && !g.walRequested && len(g.nested) == 0
	g.mu.Unlock()
	if !healthy || t.file.RetirementComplete() {
		return nil, errRollbackFaultScope
	}
	s := &rollbackFaultScope{tx: t, main: t.file, cleanup: g, plan: plan, receipt: receipt}
	s.append(rollbackfault.Event{Kind: rollbackfault.RunStarted})
	if s.problem != nil {
		return nil, s.problem
	}
	// Admission contains no owner queries and no native operation.
	rollbackFaultRegistry.mu.Lock()
	defer rollbackFaultRegistry.mu.Unlock()
	if rollbackFaultRegistry.used || rollbackFaultRegistry.active != nil {
		return nil, errRollbackFaultScope
	}
	rollbackFaultRegistry.used = true
	rollbackFaultRegistry.active = s
	return s, nil
}

func rollbackFaultFor(t *Tx) *rollbackFaultScope {
	rollbackFaultRegistry.mu.Lock()
	s := rollbackFaultRegistry.active
	rollbackFaultRegistry.mu.Unlock()
	if s == nil || s.tx != t {
		return nil
	}
	return s
}

func (s *rollbackFaultScope) append(event rollbackfault.Event) {
	if s.events >= rollbackfault.MaxEvents {
		s.problem = errRollbackFaultScope
		return
	}
	event.Identity = s.plan.Spec().Identity
	event.Ordinal = s.events + 1
	s.events++
	if s.receipt.Append(event) != rollbackfault.NoProblem {
		s.problem = errRollbackFaultScope
	}
}

func rollbackFaultPoint(point rollbackIOPoint) (rollbackfault.Point, int) {
	switch point {
	case rbJCreate:
		return rollbackfault.JournalCreate, 0
	case rbJInvalid:
		return rollbackfault.JournalInvalid, 1
	case rbDBOriginalRead:
		return rollbackfault.DBOriginalRead, 2
	case rbJRecord:
		return rollbackfault.JournalRecord, 3
	case rbJBodySync:
		return rollbackfault.JournalBodySync, 4
	case rbJValidHeader:
		return rollbackfault.JournalValidHeader, 5
	case rbJHeaderSync:
		return rollbackfault.JournalHeaderSync, 6
	case rbPublishDirSync:
		return rollbackfault.PublishDirectorySync, 7
	case rbDBPage:
		return rollbackfault.DBPage, 8
	case rbDBSync:
		return rollbackfault.DBSync, 9
	case rbJUnlink:
		return rollbackfault.JournalUnlink, 10
	case rbFinalDirSync:
		return rollbackfault.FinalDirectorySync, 11
	case rbCleanupUnlink:
		return rollbackfault.CleanupUnlink, 12
	case rbCleanupDirSync:
		return rollbackfault.CleanupDirectorySync, 13
	default:
		return 0, -1
	}
}

// Called under the original Tx mutex. Its private mutex is released before
// every original operation; registry admission is never held during I/O.
func rollbackFaultBefore(t *Tx, point rollbackIOPoint, f *vfs.File, path string, off int64, length int) rollbackFaultTicket {
	ticket := rollbackFaultBeforeCore(t, point, f, path, off, length)
	rollbackRestartBefore(ticket)
	return ticket
}

func rollbackFaultBeforeCore(t *Tx, point rollbackIOPoint, f *vfs.File, path string, off int64, length int) rollbackFaultTicket {
	s := rollbackFaultFor(t)
	if s == nil {
		return rollbackFaultTicket{action: rollbackfault.Pass}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, index := rollbackFaultPoint(point)
	if index < 0 || s.finalized || s.inFlight || s.counts[index] >= 4 {
		s.problem = errRollbackFaultScope
		return rollbackFaultTicket{action: rollbackfault.Pass}
	}
	s.counts[index]++
	spec := s.plan.Spec()
	call := rollbackfault.Call{Point: p, Occurrence: s.counts[index], Offset: off, Length: length}
	valid := t.file == s.main && t.cleanup == s.cleanup && t.session == nil && t.shared == nil && t.encrypted == nil && !t.keepFile && t.borrowedCleanup == nil
	switch point {
	case rbDBOriginalRead, rbDBPage, rbDBSync:
		call.Role, call.Owner = rollbackfault.Database, spec.MainOwner
		valid = valid && f == s.main && path == ""
	case rbPublishDirSync, rbFinalDirSync, rbCleanupDirSync:
		call.Role, call.Owner = rollbackfault.Directory, spec.DirectoryOwner
		valid = valid && f == nil && path == t.path
	case rbJCreate, rbJUnlink, rbCleanupUnlink:
		call.Role, call.Owner = rollbackfault.Journal, spec.JournalOwner
		valid = valid && f == nil && path == t.path+"-journal"
	default:
		call.Role, call.Owner = rollbackfault.Journal, spec.JournalOwner
		valid = valid && f != nil && f == s.journal && path == ""
	}
	if !valid {
		s.problem = errRollbackFaultScope
	}
	action := rollbackfault.Pass
	if valid && !s.firstError && s.problem == nil && call == spec.Target && point != rbCleanupUnlink && point != rbCleanupDirSync {
		action = spec.Action
	}
	s.inFlight = true
	s.append(rollbackfault.Event{Kind: rollbackfault.Intent, Call: call, Action: action})
	if s.problem != nil {
		action = rollbackfault.Pass
	}
	return rollbackFaultTicket{scope: s, call: call, action: action}
}

func (s *rollbackFaultScope) diagnostic(err error) uint64 {
	if err == nil {
		return 0
	}
	if s.events >= rollbackfault.MaxEvents {
		s.problem = errRollbackFaultScope
		return 0
	}
	// The original error remains strongly retained, never serialized as success.
	s.diagnostics[s.events] = err
	return uint64(s.events) + 1
}

func rollbackFaultAfter(ticket rollbackFaultTicket, executed bool, nativeN int, nativeErr error, returnedN int, returnedErr error, normalizedErr error, effect rollbackfault.SideEffect) {
	rollbackFaultAfterCore(ticket, executed, nativeN, nativeErr, returnedN, returnedErr, normalizedErr, effect)
	rollbackRestartAfter(ticket)
}

func rollbackFaultAfterCore(ticket rollbackFaultTicket, executed bool, nativeN int, nativeErr error, returnedN int, returnedErr error, normalizedErr error, effect rollbackfault.SideEffect) {
	s := ticket.scope
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	result := rollbackfault.Result{Executed: executed, NativeN: nativeN, ReturnedN: returnedN, Effect: effect}
	if nativeErr != nil {
		result.NativeError = rollbackfault.Underlying
		result.NativeErrorToken = s.diagnostic(nativeErr)
	}
	if returnedErr != nil {
		switch {
		case nativeErr != nil && returnedErr == nativeErr:
			result.ReturnedError, result.ReturnedErrorToken = rollbackfault.Underlying, result.NativeErrorToken
		case returnedErr == errRollbackFaultInjected:
			result.ReturnedError = rollbackfault.Injected
		default:
			s.problem = errRollbackFaultScope
		}
	}
	if normalizedErr != nil {
		switch {
		case nativeErr != nil && normalizedErr == nativeErr:
			result.NormalizedError, result.NormalizedErrorToken = rollbackfault.Underlying, result.NativeErrorToken
		case normalizedErr == errRollbackFaultInjected:
			result.NormalizedError = rollbackfault.Injected
		case normalizedErr == io.ErrShortWrite:
			result.NormalizedError = rollbackfault.ShortWrite
		default:
			s.problem = errRollbackFaultScope
		}
	}
	if ticket.call == s.plan.Spec().Target {
		s.targetError = normalizedErr
		s.targetErrorToken = result.NormalizedErrorToken
	}
	if normalizedErr != nil || returnedErr != nil || nativeErr != nil {
		s.firstError = true
	}
	s.inFlight = false
	s.append(rollbackfault.Event{Kind: rollbackfault.Returned, Call: ticket.call, Action: ticket.action, Result: result})
}

func rollbackCreate(t *Tx, point rollbackIOPoint, path string, flags int) (*vfs.File, error) {
	ticket := rollbackFaultBefore(t, point, nil, path, 0, 0)
	if flags != os.O_WRONLY|os.O_CREATE|os.O_EXCL && ticket.scope != nil {
		ticket.scope.mu.Lock()
		ticket.scope.problem = errRollbackFaultScope
		ticket.scope.mu.Unlock()
		ticket.action = rollbackfault.Pass
	}
	if ticket.action == rollbackfault.FailBefore {
		ticket.scope.mu.Lock()
		ticket.scope.journalAbsent = true
		ticket.scope.mu.Unlock()
		rollbackFaultAfter(ticket, false, 0, nil, 0, errRollbackFaultInjected, errRollbackFaultInjected, rollbackfault.EffectNone)
		return nil, errRollbackFaultInjected
	}
	f, err := openJournal(path, flags)
	if ticket.scope != nil {
		ticket.scope.mu.Lock()
		ticket.scope.journal = f
		ticket.scope.journalUnknown = err != nil || f == nil
		ticket.scope.mu.Unlock()
	}
	rollbackFaultAfter(ticket, true, 0, err, 0, err, err, rollbackfault.EffectUnknown)
	return f, err
}

func rollbackRead(t *Tx, point rollbackIOPoint, f *vfs.File, b []byte, off int64) (int, error) {
	ticket := rollbackFaultBefore(t, point, f, "", off, len(b))
	if ticket.action == rollbackfault.FailBefore {
		rollbackFaultAfter(ticket, false, 0, nil, 0, errRollbackFaultInjected, errRollbackFaultInjected, rollbackfault.EffectNone)
		return 0, errRollbackFaultInjected
	}
	n, err := f.ReadAt(b, off)
	rollbackFaultAfter(ticket, true, n, err, n, err, err, rollbackfault.EffectNone)
	return n, err
}

type rollbackFaultWriter struct {
	file        *vfs.File
	action      rollbackfault.Action
	prefix      int
	prefixError bool
	executed    bool
	nativeN     int
	nativeErr   error
	returnedN   int
	returnedErr error
}

// Only the original managed owner is invoked, once. No raw descriptor, bypass,
// arbitrary callback, retry, extra Sync or hidden second injected failure.
func (w *rollbackFaultWriter) WriteAt(b []byte, off int64) (int, error) {
	if w.action == rollbackfault.FailBefore {
		w.returnedErr = errRollbackFaultInjected
		return 0, w.returnedErr
	}
	write := b
	if w.action == rollbackfault.PrefixShortWrite {
		write = b[:w.prefix]
	}
	w.executed = true
	w.nativeN, w.nativeErr = w.file.WriteAt(write, off)
	w.returnedN, w.returnedErr = w.nativeN, w.nativeErr
	if w.action == rollbackfault.PrefixShortWrite && w.nativeErr == nil && w.nativeN == w.prefix && w.prefixError {
		w.returnedErr = errRollbackFaultInjected
	}
	return w.returnedN, w.returnedErr
}

func rollbackWrite(t *Tx, point rollbackIOPoint, f *vfs.File, b []byte, off int64) error {
	ticket := rollbackFaultBefore(t, point, f, "", off, len(b))
	if ticket.scope == nil {
		return writeFullAt(f, b, off)
	}
	spec := ticket.scope.plan.Spec()
	w := &rollbackFaultWriter{file: f, action: ticket.action, prefix: spec.Prefix, prefixError: spec.PrefixError}
	err := writeFullAt(w, b, off)
	effect := rollbackfault.EffectPossible
	if !w.executed {
		effect = rollbackfault.EffectNone
	} else if w.nativeErr != nil || w.nativeN < 0 || w.nativeN > len(b) {
		effect = rollbackfault.EffectUnknown
	} else if w.action == rollbackfault.PrefixShortWrite && w.nativeN == w.prefix && w.prefix == 0 {
		effect = rollbackfault.EffectNone
	} else if w.action == rollbackfault.PrefixShortWrite && w.nativeN == w.prefix {
		effect = rollbackfault.EffectPrefix
	}
	rollbackFaultAfter(ticket, w.executed, w.nativeN, w.nativeErr, w.returnedN, w.returnedErr, err, effect)
	return err
}

func rollbackFileSync(t *Tx, point rollbackIOPoint, f *vfs.File) error {
	ticket := rollbackFaultBefore(t, point, f, "", 0, 0)
	if ticket.action == rollbackfault.FailBefore {
		rollbackFaultAfter(ticket, false, 0, nil, 0, errRollbackFaultInjected, errRollbackFaultInjected, rollbackfault.EffectNone)
		return errRollbackFaultInjected
	}
	err := f.Sync()
	rollbackFaultAfter(ticket, true, 0, err, 0, err, err, rollbackfault.EffectUnknown)
	return err
}

func rollbackDirectorySync(t *Tx, point rollbackIOPoint, path string) error {
	ticket := rollbackFaultBefore(t, point, nil, path, 0, 0)
	if ticket.action == rollbackfault.FailBefore {
		rollbackFaultAfter(ticket, false, 0, nil, 0, errRollbackFaultInjected, errRollbackFaultInjected, rollbackfault.EffectNone)
		return errRollbackFaultInjected
	}
	err := syncDirectory(path)
	rollbackFaultAfter(ticket, true, 0, err, 0, err, err, rollbackfault.EffectUnknown)
	return err
}

func rollbackUnlink(t *Tx, point rollbackIOPoint, path string) error {
	ticket := rollbackFaultBefore(t, point, nil, path, 0, 0)
	// Cleanup is observation-only even after an underlying, unplanned error.
	if ticket.action == rollbackfault.FailBefore {
		rollbackFaultAfter(ticket, false, 0, nil, 0, errRollbackFaultInjected, errRollbackFaultInjected, rollbackfault.EffectNone)
		return errRollbackFaultInjected
	}
	err := os.Remove(path)
	rollbackFaultAfter(ticket, true, 0, err, 0, err, err, rollbackfault.EffectUnknown)
	return err
}

// This accepts a caller-supplied Commit return; it cannot prove that a caller
// executed Commit. There is intentionally no native caller or physical verifier
// here. It only snapshots the bound owners and seals an adapter-only receipt.
func finishRollbackFault(s *rollbackFaultScope, commitErr error) rollbackFaultReport {
	if s == nil {
		return rollbackFaultReport{Problem: errRollbackFaultScope}
	}
	s.tx.mu.Lock()
	defer s.tx.mu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finalized || s.inFlight {
		return rollbackFaultReport{Problem: errRollbackFaultScope}
	}
	s.finalized = true
	commit := rollbackfault.CommitResult{Class: rollbackfault.CommitFailed, Logical: rollbackfault.LogicalOpen}
	if s.tx.file == nil && s.tx.encrypted == nil {
		commit.Logical = rollbackfault.LogicalClosed
	}
	switch {
	case commitErr == nil:
		commit.Class = rollbackfault.CommitSucceeded
	case errors.Is(commitErr, ErrPlaintextCleanupPending):
		commit.Class = rollbackfault.CommitCleanupPending
	case errors.Is(commitErr, ErrCommitUncertain):
		commit.Class = rollbackfault.CommitUncertain
	case errors.Is(commitErr, ErrRecoveryRequired):
		commit.Class = rollbackfault.CommitRecoveryRequired
	case errors.Is(commitErr, ErrUnsupported):
		commit.Class = rollbackfault.CommitUnsupported
	}
	if commitErr != nil {
		commit.Error = rollbackfault.Underlying
		commit.ErrorToken = s.diagnostic(commitErr)
	}
	if s.targetError != nil && errors.Is(commitErr, s.targetError) {
		switch {
		case s.targetErrorToken != 0:
			commit.TargetError, commit.TargetErrorToken = rollbackfault.Underlying, s.targetErrorToken
		case s.targetError == errRollbackFaultInjected:
			commit.TargetError = rollbackfault.Injected
		case s.targetError == io.ErrShortWrite:
			commit.TargetError = rollbackfault.ShortWrite
		}
	}
	s.append(rollbackfault.Event{Kind: rollbackfault.CommitReturned, Commit: commit})
	spec := s.plan.Spec()
	owners := rollbackfault.OwnersResult{MainOwner: spec.MainOwner, JournalOwner: spec.JournalOwner, CleanupOwner: spec.CleanupOwner, Main: rollbackfault.OwnerLive, Journal: rollbackfault.OwnerUnknown, Cleanup: rollbackfault.OwnerPending}
	if s.main.RetirementComplete() {
		owners.Main = rollbackfault.OwnerRetired
	}
	if s.journal != nil && s.journal.RetirementComplete() {
		owners.Journal = rollbackfault.OwnerRetired
	} else if s.journalAbsent {
		owners.Journal = rollbackfault.OwnerNotAcquired
	} else if s.journal != nil && !s.journalUnknown {
		owners.Journal = rollbackfault.OwnerPending
	}
	g := s.cleanup
	g.mu.Lock()
	if g.done && g.main == nil && g.temporary == nil && g.wal == nil && len(g.nested) == 0 && g.pending.Load() == nil {
		owners.Cleanup = rollbackfault.OwnerRetired
	}
	for _, nested := range g.nested {
		if _, unknown := nested.(plainTerminal); unknown {
			owners.Cleanup = rollbackfault.OwnerUnknown
		}
	}
	g.mu.Unlock()
	s.retired = owners.Main == rollbackfault.OwnerRetired && (owners.Journal == rollbackfault.OwnerRetired || owners.Journal == rollbackfault.OwnerNotAcquired) && owners.Cleanup == rollbackfault.OwnerRetired
	s.append(rollbackfault.Event{Kind: rollbackfault.OwnersObserved, Owners: owners})
	s.append(rollbackfault.Event{Kind: rollbackfault.ValidationObserved, Validation: rollbackfault.ValidationAbsent})
	s.append(rollbackfault.Event{Kind: rollbackfault.Terminal})
	observation := s.receipt.Seal()
	if s.problem != nil {
		observation.Outcome = rollbackfault.ObservationInvalid
	}
	events, count := s.receipt.Events()
	return rollbackFaultReport{Observation: observation, Events: events, EventCount: count, Diagnostics: s.diagnostics, CommitError: commitErr, Problem: s.problem}
}

func disarmRollbackFault(s *rollbackFaultScope) error {
	if s == nil {
		return errRollbackFaultScope
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.finalized || !s.retired || s.problem != nil || s.receipt.Seal().Outcome != rollbackfault.AdapterObserved {
		return errRollbackFaultScope
	}
	rollbackFaultRegistry.mu.Lock()
	defer rollbackFaultRegistry.mu.Unlock()
	if rollbackFaultRegistry.active != s {
		return errRollbackFaultScope
	}
	rollbackFaultRegistry.active = nil
	// used remains true. Pending/unknown owners never reach this assignment.
	return nil
}
