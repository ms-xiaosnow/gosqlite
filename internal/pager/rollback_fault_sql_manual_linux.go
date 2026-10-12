//go:build linux && manual_fault_rollback

package pager

// This bridge exists only in the manual build. It exposes neither native
// owners nor a retry/reset path. The default shipping selection is unchanged.
import (
	"os"

	"github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackfault"
)

type manualRollbackSQLScope struct{ scope *rollbackFaultScope }

type ManualRollbackSQLReport struct {
	Observation rollbackfault.Observation
	Events      [rollbackfault.MaxEvents]rollbackfault.Event
	EventCount  uint8
	Diagnostics [rollbackfault.MaxEvents]error
	CommitError error
	Problem     error
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
	case rollbackfault.FailBefore:
		caseID = rollbackfault.SQLDBSyncBefore
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
	return ManualRollbackSQLReport{Observation: report.Observation, Events: report.Events, EventCount: report.EventCount, Diagnostics: report.Diagnostics, CommitError: report.CommitError, Problem: report.Problem}
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
