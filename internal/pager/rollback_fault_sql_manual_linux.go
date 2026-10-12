//go:build linux && manual_fault_rollback

package pager

// This bridge exists only in the manual build. It exposes neither native
// owners nor a retry/reset path. The default shipping selection is unchanged.
import (
	"errors"
	"os"
	"reflect"

	"github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackfault"
)

type manualRollbackSQLScope struct{ scope *rollbackFaultScope }

type ManualRollbackSQLReport struct {
	Observation             rollbackfault.Observation
	Events                  [rollbackfault.MaxEvents]rollbackfault.Event
	EventCount              uint8
	Diagnostics             [rollbackfault.MaxEvents]error
	CommitError             error
	Problem                 error
	PreDBOnlyTargetCause    bool
	PostDBOnlyTargetCause   bool
	MisplacedWrite          rollbackfault.SQLMisplacedWrite
	MisplaceOnlyTargetCause bool
}

// Snapshots are separately locked scalar observations, not an atomic proof of
// absence of concurrent users. The harness requires an independent process.
// HealthyTx means only an untouched, independently owned preparation Tx can be
// retired through its public Rollback. No native owner method is invoked here.
type ManualRollbackSQLSnapshot struct {
	Groups        int
	Pending       int
	HealthyTx     bool
	AdapterActive bool
	AdapterUsed   bool
}

func ManualRollbackSQLState(tx *Tx) ManualRollbackSQLSnapshot {
	var out ManualRollbackSQLSnapshot
	plainRetirements.mu.Lock()
	out.Groups = len(plainRetirements.groups)
	for g := range plainRetirements.groups {
		if g.pending.Load() != nil {
			out.Pending++
		}
	}
	plainRetirements.mu.Unlock()
	rollbackFaultRegistry.mu.Lock()
	out.AdapterActive = rollbackFaultRegistry.active != nil
	out.AdapterUsed = rollbackFaultRegistry.used
	rollbackFaultRegistry.mu.Unlock()
	if tx != nil {
		tx.mu.Lock()
		g := tx.cleanup
		if g != nil {
			g.mu.Lock()
			main, concrete := g.main.(plainFile)
			out.HealthyTx = concrete && main.file == tx.file && tx.file != nil && tx.session == nil && tx.shared == nil && tx.encrypted == nil && !tx.keepFile && tx.borrowedCleanup == nil && !g.started && !g.done && g.cause == nil && g.pending.Load() == nil && g.temporary == nil && !g.temporaryReserved && !g.temporaryRequested && g.wal == nil && !g.walReserved && !g.walRequested && len(g.nested) == 0
			g.mu.Unlock()
		}
		tx.mu.Unlock()
	}
	return out
}

func ArmManualRollbackSQLFault(tx *Tx, plan rollbackfault.Plan) (*manualRollbackSQLScope, error) {
	if os.Getenv("GOSQLITE_MANUAL_FAULT_ROLLBACK") != "independently_authorized" {
		return nil, errRollbackFaultGate
	}
	spec := plan.Spec()
	var caseID rollbackfault.SQLCase
	switch spec.Action {
	case rollbackfault.Pass:
		caseID = rollbackfault.SQLControl
	case rollbackfault.PrefixShortWrite:
		caseID = rollbackfault.SQLIndexPrefix64
		if spec.PrefixError {
			caseID = rollbackfault.SQLIndexPrefix64Error
		}
	case rollbackfault.MisplaceIndexPageMinus64:
		caseID = rollbackfault.SQLIndexMisplaceMinus64Error
	case rollbackfault.FailBefore:
		switch spec.Target.Point {
		case rollbackfault.DBSync:
			caseID = rollbackfault.SQLDBSyncBefore
		case rollbackfault.JournalBodySync:
			caseID = rollbackfault.SQLJournalBodySyncBefore
		case rollbackfault.JournalHeaderSync:
			caseID = rollbackfault.SQLJournalHeaderSyncBefore
		case rollbackfault.PublishDirectorySync:
			caseID = rollbackfault.SQLPublishDirectorySyncBefore
		case rollbackfault.JournalUnlink:
			caseID = rollbackfault.SQLJournalUnlinkBefore
		case rollbackfault.FinalDirectorySync:
			caseID = rollbackfault.SQLFinalDirectorySyncBefore
		default:
			return nil, errRollbackFaultScope
		}
	default:
		return nil, errRollbackFaultScope
	}
	frozen, problem := rollbackfault.SQLPlan(spec.Identity, caseID)
	if problem != rollbackfault.NoProblem || frozen.Spec() != spec {
		return nil, errRollbackFaultScope
	}
	if tx == nil {
		return nil, errRollbackFaultScope
	}
	tx.mu.Lock()
	threePages := tx.originalPages == 3 && tx.header.PageCount == 3 && len(tx.dirty) == 3 && len(tx.dirty[1]) == 4096 && len(tx.dirty[2]) == 4096 && len(tx.dirty[3]) == 4096
	tx.mu.Unlock()
	if !threePages {
		return nil, errRollbackFaultScope
	}
	state := ManualRollbackSQLState(tx)
	if state.Groups != 1 || state.Pending != 0 || !state.HealthyTx || state.AdapterActive || state.AdapterUsed {
		return nil, errRollbackFaultScope
	}
	scope, err := armRollbackFault(tx, plan)
	if err != nil {
		return nil, err
	}
	return &manualRollbackSQLScope{scope: scope}, nil
}

// The caller must pass the untouched return from public sqlite.Tx.Commit in
// its immediately following statement. This bridge never invokes Commit.
func FinishManualRollbackSQLFault(scope *manualRollbackSQLScope, commitErr error) ManualRollbackSQLReport {
	if scope == nil {
		return ManualRollbackSQLReport{Problem: errRollbackFaultScope}
	}
	report := finishRollbackFault(scope.scope, commitErr)
	scope.scope.mu.Lock()
	target, point := scope.scope.targetError, scope.scope.plan.Spec().Target.Point
	action, misplaced := scope.scope.plan.Spec().Action, scope.scope.misplacedWrite
	scope.scope.mu.Unlock()
	preDBOnlyTargetCause := false
	if point == rollbackfault.JournalBodySync || point == rollbackfault.JournalHeaderSync || point == rollbackfault.PublishDirectorySync {
		scope.scope.tx.mu.Lock()
		poison := scope.scope.tx.poisoned
		scope.scope.tx.mu.Unlock()
		preDBOnlyTargetCause = manualRollbackSQLOnlyTargetCause(commitErr, poison, target, nil)
	}
	postDBOnlyTargetCause := false
	if point == rollbackfault.JournalUnlink || point == rollbackfault.FinalDirectorySync {
		scope.scope.tx.mu.Lock()
		poison := scope.scope.tx.poisoned
		scope.scope.tx.mu.Unlock()
		permitted := ErrRecoveryRequired
		if point == rollbackfault.FinalDirectorySync {
			permitted = ErrCommitUncertain
		}
		postDBOnlyTargetCause = manualRollbackSQLOnlyTargetCause(commitErr, poison, target, permitted)
	}
	misplaceOnlyTargetCause := false
	if action == rollbackfault.MisplaceIndexPageMinus64 {
		scope.scope.tx.mu.Lock()
		poison := scope.scope.tx.poisoned
		scope.scope.tx.mu.Unlock()
		misplaceOnlyTargetCause = manualRollbackSQLOnlyTargetCause(commitErr, poison, target, ErrRecoveryRequired)
	}
	return ManualRollbackSQLReport{Observation: report.Observation, Events: report.Events, EventCount: report.EventCount, Diagnostics: report.Diagnostics, CommitError: report.CommitError, Problem: report.Problem, PreDBOnlyTargetCause: preDBOnlyTargetCause, PostDBOnlyTargetCause: postDBOnlyTargetCause, MisplacedWrite: misplaced, MisplaceOnlyTargetCause: misplaceOnlyTargetCause}
}

// This observation is limited to the original public wrapper and the actual
// retired Tx poison. The caller also requires the top object to be *sqlite.Error.
// Below that wrapper only errors.Join nodes, exact injected/ErrPoisoned leaves,
// and the one explicitly permitted post-DB classification sentinel are accepted.
// Unknown shapes and custom Is/As are refused BEFORE any
// Unwrap, so a consumed cleanup diagnostic cannot disappear behind traversal.
// No native operation, reset, callback, or retry is introduced here.
func manualRollbackSQLOnlyTargetCause(commitErr, poison, target, permitted error) bool {
	if commitErr == nil || poison == nil || target != errRollbackFaultInjected {
		return false
	}
	if permitted != nil && permitted != ErrRecoveryRequired && permitted != ErrCommitUncertain {
		return false
	}
	if _, custom := commitErr.(interface{ Is(error) bool }); custom {
		return false
	}
	if _, custom := commitErr.(interface{ As(any) bool }); custom {
		return false
	}
	public, ok := commitErr.(interface{ Unwrap() error })
	if !ok || public.Unwrap() != poison {
		return false
	}
	joinType := reflect.TypeOf(errors.Join(ErrPoisoned, target))
	var pending [rollbackfault.MaxEvents]error
	pending[0] = poison
	count, visited := 1, 0
	sawTarget, sawPoisoned := false, false
	sawPermitted := permitted == nil
	for count != 0 {
		if visited == len(pending) {
			return false
		}
		visited++
		count--
		current := pending[count]
		pending[count] = nil
		if _, custom := current.(interface{ Is(error) bool }); custom {
			return false
		}
		if _, custom := current.(interface{ As(any) bool }); custom {
			return false
		}
		if current == target {
			sawTarget = true
			continue
		}
		if current == ErrPoisoned {
			sawPoisoned = true
			continue
		}
		if permitted != nil && current == permitted {
			sawPermitted = true
			continue
		}
		if reflect.TypeOf(current) != joinType {
			return false
		}
		children := current.(interface{ Unwrap() []error }).Unwrap()
		if len(children) == 0 || len(children) > len(pending)-count {
			return false
		}
		for _, child := range children {
			if child == nil {
				return false
			}
			pending[count] = child
			count++
		}
	}
	return sawTarget && sawPoisoned && sawPermitted
}

func DisarmManualRollbackSQLFault(scope *manualRollbackSQLScope) error {
	if scope == nil {
		return errRollbackFaultScope
	}
	state := ManualRollbackSQLState(nil)
	if state.Groups != 0 || state.Pending != 0 {
		return errRollbackFaultScope
	}
	return disarmRollbackFault(scope.scope)
}
