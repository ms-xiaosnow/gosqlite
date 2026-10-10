package main

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/encstore"
	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
	"github.com/ms-xiaosnow/gosqlite/internal/walindex"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Pure memory values only: no native callbacks, files or failing database I/O.
type forbiddenOutcomeMethods struct{}

func (*forbiddenOutcomeMethods) Error() string { panic("must not format unknown error") }
func (*forbiddenOutcomeMethods) Is(error) bool { panic("must not call custom Is") }
func (*forbiddenOutcomeMethods) As(any) bool   { panic("must not call custom As") }
func (*forbiddenOutcomeMethods) Unwrap() error { panic("must not call custom Unwrap") }

type uncomparableOutcome []byte

func (uncomparableOutcome) Error() string { return "opaque diagnostic" }

func TestStrictOutcomeGraph(t *testing.T) {
	busy := func(e error) error { return &sqlite.Error{Code: sqlite.CodeBusy, Op: "memory", Err: e} }
	cancel := func(e error) error { return &sqlite.Error{Code: sqlite.CodeInterrupt, Op: "memory", Err: e} }
	for _, e := range []error{sqlite.ErrBusy, sqlite.ErrBusySnapshot, encstore.ErrBusySnapshot, fmt.Errorf("lock: %w: %w", sqlite.ErrBusy, vfs.ErrBusy), errors.Join(sqlite.ErrBusy, walindex.ErrBusy), busy(errors.Join(sqlite.ErrBusy, encstore.ErrBusy))} {
		if got := errorOutcome(e); got != outcomeBusy || !ordinaryBusy(e) {
			t.Fatalf("legitimate BUSY outcome=%v", got)
		}
	}
	for _, e := range []error{context.Canceled, context.DeadlineExceeded, cancel(fmt.Errorf("operation: %w", context.Canceled)), errors.Join(busy(sqlite.ErrBusy), cancel(context.Canceled)), busy(errors.Join(sqlite.ErrBusy, context.Canceled))} {
		if got := errorOutcome(e); got != outcomeCanceled || ordinaryBusy(e) {
			t.Fatalf("legitimate cancellation outcome=%v", got)
		}
	}
	cycle := &sqlite.Error{Code: sqlite.CodeBusy, Op: "memory-cycle"}
	cycle.Err = cycle
	var typedNil *sqlite.Error
	var customNil *forbiddenOutcomeMethods
	tooDeep := error(sqlite.ErrBusy)
	for i := 0; i < maxOutcomeErrorNodes; i++ {
		tooDeep = busy(tooDeep)
	}
	many := make([]error, maxOutcomeErrorNodes)
	for i := range many {
		many[i] = sqlite.ErrBusy
	}
	bad := []error{&forbiddenOutcomeMethods{}, customNil, uncomparableOutcome{1}, typedNil, cycle, tooDeep, errors.Join(many...), busy(nil), busy(context.Canceled), cancel(sqlite.ErrBusy), &sqlite.Error{Code: sqlite.CodeIOErr, Err: sqlite.ErrBusy}, &sqlite.Error{Code: sqlite.CodeCorrupt, Err: context.Canceled}, cleanupJoin(sqlite.ErrBusy, errors.New("consumed close diagnostic")), errors.Join(context.Canceled, errCleanup)}
	for i, e := range bad {
		var c Counts
		c.Attempts = 1
		if !classify(&c, e) || ordinaryBusy(e) || !c.Valid() || c.Failed != 1 {
			t.Fatalf("bad graph %d accepted: counts=%+v", i, c)
		}
	}
	if errorOutcome(nil) != outcomeSuccess {
		t.Fatal("nil error not successful")
	}
	// Node budget is inclusive: envelope(s) plus one recognized leaf.
	boundary := error(sqlite.ErrBusy)
	for i := 1; i < maxOutcomeErrorNodes; i++ {
		boundary = busy(boundary)
	}
	if errorOutcome(boundary) != outcomeBusy {
		t.Fatal("exact budget refused")
	}
}

func TestGenerationAbsencePreservesIteratorErrors(t *testing.T) {
	if missingGenerationError(context.Canceled) != context.Canceled {
		t.Fatal("cancellation relabeled missing row")
	}
	joined := errors.Join(context.Canceled, errors.New("iterator diagnostic"))
	if missingGenerationError(joined) != joined || errorOutcome(missingGenerationError(joined)) != outcomeFailed {
		t.Fatal("iterator diagnostic suppressed")
	}
	if errorOutcome(missingGenerationError(nil)) != outcomeFailed {
		t.Fatal("clean missing row accepted")
	}
}
