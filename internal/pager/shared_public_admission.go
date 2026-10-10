package pager

import (
	"errors"
	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

// Finish adopts public pre-session failures into the already-reserved shared
// close group. After transfer it leaves the session/open-cleanup protocols alone.
// handled permits the public layer to replace a possibly unsafe original graph
// with the bounded result instead of joining that graph back into the error.
func (a *SharedCloseAdmission) Finish(cause error) (result error, handled bool) {
	if a == nil {
		return cause, false
	}
	a.mu.Lock()
	g := a.group
	a.group = nil
	a.mu.Unlock()
	if g == nil {
		return cause, false
	}
	return g.beginPublicOpen(cause), true
}

// This separate entry point bounds public error discovery without changing the
// previously reviewed Session Close or failed-pager-open state machines.
func (g *sharedCloseRetirement) beginPublicOpen(cause error) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.started || g.done {
		return g.retryLocked()
	}
	g.started = true
	owners, discovery := vfs.CleanupOwnersBounded(cause, maxCloseCleanupErrorNodes)
	if discovery != nil {
		opaque := &plainUnsafeCause{retained: cause, diagnostic: errors.Join(ErrLimit, discovery)}
		g.nested = append(g.nested, plainTerminal{cause: opaque})
		cause = opaque
	} else {
		for _, owner := range owners {
			g.nested = append(g.nested, sharedCloseNested{owner})
		}
	}
	err := g.retireLocked()
	if g.done {
		return errors.Join(cause, err)
	}
	g.cause = errors.Join(cause, err)
	return g.pendingLocked(g.cause)
}
