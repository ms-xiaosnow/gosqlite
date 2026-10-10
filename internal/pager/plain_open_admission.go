package pager

import "sync/atomic"

// PlainOpenAdmission covers rollback/exclusive public metadata before pager construction.
// Its single precharged root transfers into OpenAdmitted or OpenSessionAdmitted; it never owns
// a Session/DB/callback or makes a native call while admission is locked.
type PlainOpenAdmission struct {
	group atomic.Pointer[plainRetirement]
}

func ReservePlainOpen() (*PlainOpenAdmission, error) {
	g, err := plainRetirements.reserve()
	if err != nil {
		return nil, err
	}
	a := &PlainOpenAdmission{}
	a.group.Store(g)
	return a, nil
}

// Finish adopts an early public-open error and retires an untransferred root.
// handled is false after transfer (or prior Finish), so callers preserve their
// original result. A handled result must replace, not rejoin, the old cause:
// bounded adoption may have made an unsafe error graph opaque.
func (a *PlainOpenAdmission) Finish(cause error) (result error, handled bool) {
	if a == nil {
		return cause, false
	}
	g := a.group.Swap(nil)
	if g == nil {
		return cause, false
	}
	return g.begin(cause), true
}
func claimPlainOpen(a *PlainOpenAdmission) (*plainRetirement, error) {
	if a == nil {
		return plainRetirements.reserve()
	}
	g := a.group.Swap(nil)
	if g == nil {
		return nil, ErrMisuse
	}
	return g, nil
}

// ExclusiveOpenAdmission preserves the exclusive-only admission entry point.
type ExclusiveOpenAdmission = PlainOpenAdmission

func ReserveExclusiveOpen() (*ExclusiveOpenAdmission, error) { return ReservePlainOpen() }
func claimExclusiveOpen(a *ExclusiveOpenAdmission) (*plainRetirement, error) {
	return claimPlainOpen(a)
}
