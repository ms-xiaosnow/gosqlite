package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
	"reflect"
	"strings"
)

// closeLedger is single-goroutine-owned. Workers finish before their DB handles
// are closed. Failed handles and original diagnostics stay owned; an unrelated
// successful Close cannot clear the historical failure or prove their retirement.
type closeHandle interface{ Close() error }
type ownedHandle struct {
	handle     closeHandle
	label      string
	attempted  bool
	diagnostic error
}
type closeLedger struct {
	live     map[*ownedHandle]struct{}
	failures int
	acquired int
}

func newCloseLedger() *closeLedger { return &closeLedger{live: make(map[*ownedHandle]struct{})} }
func (l *closeLedger) acquire(handle closeHandle, label string) *ownedHandle {
	owner := &ownedHandle{handle: handle, label: label}
	l.live[owner] = struct{}{}
	l.acquired++
	return owner
}
func (l *closeLedger) close(owner *ownedHandle) error {
	if owner == nil {
		return nil
	}
	if _, ok := l.live[owner]; !ok {
		return nil
	}
	// A second nil Close is not proof of retirement for one-shot rollback handles.
	// Preserve this owner/diagnostic for its documented cleanup route instead.
	if owner.attempted {
		return owner.diagnostic
	}
	owner.attempted = true
	if err := owner.handle.Close(); err != nil {
		l.failures++
		owner.diagnostic = fmt.Errorf("%s close: %w", owner.label, err)
		return owner.diagnostic
	}
	delete(l.live, owner)
	owner.handle = nil
	return nil
}
func (l *closeLedger) closeRemaining() error {
	var err error
	for owner := range l.live {
		if !owner.attempted {
			err = errors.Join(err, l.close(owner))
		}
	}
	return err
}
func (l *closeLedger) shutdown() string {
	if len(l.live) > 0 {
		return "close_error_unverified_retirement"
	}
	if l.failures > 0 {
		return "close_error"
	}
	if l.acquired == 0 {
		return "no_database_handle_returned"
	}
	return "workers_joined_normal_close"
}

// cleanupDiagnostic refuses to infer retirement from a bare success elsewhere.
// All public pending markers, IO/poison/uncertain diagnostics, opaque envelopes
// and malformed/excessive graphs are conservatively unverified. No custom
// Is/As/Error/Unwrap method is called while discovering ownership diagnostics.
func cleanupDiagnostic(err error) bool {
	if err == nil {
		return false
	}
	stack := []error{err}
	visited := 0
	for len(stack) > 0 {
		if visited >= maxOutcomeErrorNodes {
			return true
		}
		visited++
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if node == nil {
			return true
		}
		if value := reflect.ValueOf(node); value.Kind() == reflect.Pointer && value.IsNil() {
			return true
		}
		switch node {
		case errCleanup, sqlite.ErrPlaintextCleanupPending, sqlite.ErrOpenCleanupPending, sqlite.ErrCloseCleanupPending, sqlite.ErrEncryptedCleanupPending, sqlite.ErrPoisoned, sqlite.ErrCommitUncertain, sqlite.ErrRecoveryRequired:
			return true
		}
		if e, ok := node.(*sqlite.Error); ok {
			if e == nil || e.Code == sqlite.CodeIOErr || e.Err == nil {
				return true
			}
			stack = append(stack, e.Err)
			continue
		}
		typ := reflect.TypeOf(node)
		// Standard terminal error strings and exact context sentinels have no hidden
		// owner envelope. Arbitrary custom leaves may hide one and stay unverified.
		if typ == terminalErrorType || node == context.Canceled || node == context.DeadlineExceeded {
			continue
		}
		if typ != singleWrapType && typ != multiWrapType && typ != joinType {
			return true
		}
		if reflect.ValueOf(node).IsNil() {
			return true
		}
		if typ == singleWrapType {
			stack = append(stack, node.(interface{ Unwrap() error }).Unwrap())
			continue
		}
		children := node.(interface{ Unwrap() []error }).Unwrap()
		if len(children) == 0 || len(children) > maxOutcomeErrorNodes-visited-len(stack) {
			return true
		}
		stack = append(stack, children...)
	}
	return false
}

// recordFailure retains original token/error graphs even if Open returned no DB.
// Unsafe retirement is latched immediately, before any enclosing finalizer.
func recordFailure(rep *Report, err error) {
	if err == nil {
		return
	}
	rep.retainedDiagnostics = append(rep.retainedDiagnostics, err)
	rep.Errors = append(rep.Errors, diagnosticText(err))
	if cleanupDiagnostic(err) {
		rep.UnverifiedRetirement = true
	}
	setReportStatus(rep, "failed")
}
func setReportStatus(rep *Report, status string) {
	rep.Status = status
	if rep.UnverifiedRetirement {
		rep.Status = "failed"
		rep.Shutdown = "close_error_unverified_retirement"
	}
}
func recordOpenFailure(rep *Report, err error) {
	recordFailure(rep, err)
	if !rep.UnverifiedRetirement && benignUnsupported(err) {
		setReportStatus(rep, "skipped_unsupported")
	}
}

// diagnosticText is bounded too: unknown error callbacks and cyclic SQLite
// envelopes are never invoked while formatting the retained failure report.
func diagnosticText(err error) string {
	remaining := maxOutcomeErrorNodes
	var render func(error) string
	clip := func(s string) string {
		if len(s) > 1024 {
			return s[:1024] + " [truncated]"
		}
		return s
	}
	render = func(node error) string {
		if remaining == 0 {
			return "[error graph limit]"
		}
		remaining--
		if node == nil {
			return "[nil diagnostic]"
		}
		if value := reflect.ValueOf(node); value.Kind() == reflect.Pointer && value.IsNil() {
			return "[typed nil diagnostic]"
		}
		if e, ok := node.(*sqlite.Error); ok {
			if e == nil {
				return "[nil SQLite diagnostic]"
			}
			return fmt.Sprintf("sqlite %s (code %d): %s", clip(e.Op), e.Code, render(e.Err))
		}
		typ := reflect.TypeOf(node)
		if typ == terminalErrorType || node == context.Canceled || node == context.DeadlineExceeded {
			return clip(node.Error())
		}
		if typ == singleWrapType || typ == multiWrapType {
			if reflect.ValueOf(node).IsNil() {
				return "[nil format wrapper]"
			}
			return clip(node.Error())
		}
		if typ == joinType {
			if reflect.ValueOf(node).IsNil() {
				return "[nil join wrapper]"
			}
			children := node.(interface{ Unwrap() []error }).Unwrap()
			if len(children) > remaining {
				return "[error graph limit]"
			}
			var parts []string
			for _, child := range children {
				parts = append(parts, render(child))
			}
			return strings.Join(parts, "; ")
		}
		return fmt.Sprintf("[opaque diagnostic %T; original retained]", node)
	}
	return render(err)
}

func finalizeClosures(rep *Report, owners *closeLedger) error {
	err := owners.closeRemaining()
	rep.UnverifiedCloseHandles = len(owners.live)
	rep.CloseFailures = owners.failures
	rep.Shutdown = owners.shutdown()
	if owners.failures > 0 || len(owners.live) > 0 {
		rep.UnverifiedRetirement = true
	}
	setReportStatus(rep, rep.Status)
	return err
}
