package pager

import "github.com/ms-xiaosnow/gosqlite/internal/journal"

// admitRecoveryPlan is a descriptor-free refusal gate, not a publication token.
// Finite evidence and a nonsentinel raw header are both required. A successful
// parser result, complete main extent, or checksum cannot authorize EOF-count
// recovery. Existing extent, restored-header, identity and locking checks still
// apply to admitted finite plans. This helper neither mutates nor retains plans.
func admitRecoveryPlan(plan *journal.ReplayPlan) error {
	if plan == nil || plan.CountEvidence != journal.ReplayCountFiniteHeaders || plan.Header.Records == 0xffffffff {
		return journal.ErrReplayUnsupported
	}
	return nil
}
