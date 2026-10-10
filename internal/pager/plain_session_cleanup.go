package pager

import (
	"context"
	"errors"
	"os"

	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
	"github.com/ms-xiaosnow/gosqlite/internal/wal"
)

// Exclusive sessions charge a fixed current-WAL slot in addition to main and
// temporary (journal, candidate or superseded WAL), before any acquisition.
func (g *plainRetirement) reserveWAL() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.started || g.done {
		return ErrClosed
	}
	if g.cause != nil {
		return g.cause
	}
	if g.walReserved || g.wal != nil {
		return ErrLimit
	}
	g.walReserved = true
	return nil
}
func (g *plainRetirement) acquiredWAL(file *vfs.File) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.walReserved = false
	if file != nil {
		g.wal = plainFile{file}
	}
}
func (g *plainRetirement) replaceWAL() {
	g.mu.Lock()
	defer g.mu.Unlock()
	// Both files were already owned before the namespace operation. No native
	// call occurs here; ownership is merely relabelled after successful replace.
	g.wal, g.temporary = g.temporary, g.wal
	g.temporaryRequested = false
}
func (g *plainRetirement) closeWAL(cause error) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	cause = g.adoptLocked(cause)
	if cause != nil && g.cause == nil {
		g.cause = plainDiagnostic(errors.Join(ErrPoisoned, cause))
	}
	firstAttempt := !g.walRequested
	g.walRequested = true
	err := g.retireLocked()
	if len(g.nested) != 0 || g.wal != nil {
		if firstAttempt {
			g.cause = plainDiagnostic(errors.Join(g.cause, ErrPoisoned, err))
		}
		return g.pendingLocked(errors.Join(g.cause, plainDiagnostic(err)))
	}
	return errors.Join(cause, err)
}
func (g *plainRetirement) childClosePending() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.pending.Load() != nil && (g.temporaryRequested || g.walRequested)
}
func (s *Session) observeLocked(cause error) error {
	e := s.cleanup.observe(cause)
	if s.cleanup != nil && s.poisoned == nil && errors.Is(e, ErrPlaintextCleanupPending) {
		s.poisoned = e
	}
	return e
}

// CloseWithError adopts DB/Rows/Tx cleanup errors before any protection can
// drop. The one-time checkpoint is never repeated by explicit retirement retry.
func (s *Session) CloseWithError(cause error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closeLocked(cause, func() error { return s.checkpointLocked(context.Background()) })
}
func (s *Session) closeLocked(cause error, checkpoint func() error) error {
	if s.cleanup == nil {
		return errors.Join(cause, s.closeLegacyLocked())
	}
	if s.closed {
		return s.cleanup.retry()
	}
	cause = s.observeLocked(cause)
	if s.active != nil {
		return errors.Join(cause, ErrBusy)
	}
	cause = errors.Join(cause, s.poisoned, s.cleanup.check())
	if cause == nil && !s.opts.KeepWALOnClose {
		cause = checkpoint()
	}
	// Clear all logical/session state before retirement. The registry retains
	// only managed owners and diagnostics, never Session, leases or options.
	s.closed = true
	s.file, s.walFile, s.info, s.walInfo = nil, nil, nil, nil
	s.snapshot = wal.Snapshot{}
	s.opts = Options{}
	s.poisoned = nil
	return s.cleanup.beginClass(cause, nil, !s.cleanup.childClosePending())
}

// Shared-WAL's private transition Session keeps its existing metadata branch.
func (s *Session) probePathLocked(path string) (os.FileInfo, bool, error) {
	if s.cleanup != nil {
		return nativeLstatForCreate(path)
	}
	info, err := nativeLstat(path)
	return info, os.IsNotExist(err), err
}
