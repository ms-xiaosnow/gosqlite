package sqlite_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

func TestOpenCleanupNoPendingOrCanceledOpen(t *testing.T) {
	original := errors.New("ordinary application error")
	for _, err := range []error{nil, original, fmt.Errorf("wrapper: %w", original)} {
		if got := sqlite.RetryOpenCleanup(err); got != nil {
			t.Fatal(got)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	// Cancellation is observed before any path or file access.
	db, err := sqlite.OpenContext(ctx, "unused", sqlite.Options{WALShared: true})
	if db != nil || !errors.Is(err, context.Canceled) || errors.Is(err, sqlite.ErrOpenCleanupPending) {
		t.Fatalf("canceled open: %v %v", db, err)
	}
	var detail *sqlite.Error
	if !errors.As(err, &detail) || detail.Code != sqlite.CodeInterrupt {
		t.Fatalf("cancellation classification: %v", err)
	}
	if got := sqlite.RetryOpenCleanup(fmt.Errorf("driver wrapper: %w", err)); got != nil {
		t.Fatal(got)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatal("cleanup changed original error")
	}
}

type cyclicCleanupInput struct{}

func (cyclicCleanupInput) Error() string   { return "memory cyclic wrapper" }
func (e cyclicCleanupInput) Unwrap() error { return e }

func TestOpenCleanupTraversalLimitIsExplicit(t *testing.T) {
	err := sqlite.RetryOpenCleanup(cyclicCleanupInput{})
	var detail *sqlite.Error
	if !errors.Is(err, sqlite.ErrMemoryLimit) || !errors.As(err, &detail) || detail.Code != sqlite.CodeNoMem || errors.Is(err, sqlite.ErrOpenCleanupPending) {
		t.Fatalf("public traversal refusal lost classification: %v", err)
	}
}
