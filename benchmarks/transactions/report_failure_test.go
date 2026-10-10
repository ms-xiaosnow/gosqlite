package main

import (
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/pager"
	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Descriptor-free errors/report state only: this does not call a native Open or
// manufacture a native failure. The real pre-acquisition placement is AST-pinned.
func TestPreHandleRetirementFailureCannotBecomeSkipOrPass(t *testing.T) {
	for _, marker := range []error{sqlite.ErrPlaintextCleanupPending, sqlite.ErrOpenCleanupPending, sqlite.ErrCloseCleanupPending, sqlite.ErrEncryptedCleanupPending} {
		original := &sqlite.Error{Code: sqlite.CodeIOErr, Op: "memory open", Err: errors.Join(sqlite.ErrUnsupported, marker, sqlite.ErrPoisoned)}
		rep := Report{Status: "passed", Shutdown: "not_started"}
		recordOpenFailure(&rep, original)
		if !rep.UnverifiedRetirement || rep.Status != "failed" || rep.Shutdown != "close_error_unverified_retirement" || len(rep.retainedDiagnostics) != 1 || rep.retainedDiagnostics[0] != original {
			t.Fatalf("pre-handle marker lost: %+v", rep)
		}
		for _, status := range []string{"skipped_unsupported", "passed", "rejected"} {
			setReportStatus(&rep, status)
			if rep.Status != "failed" {
				t.Fatal("unsafe status overwritten")
			}
		}
		// No DB was returned, but its original managed token remains retained.
		empty := newCloseLedger()
		if err := finalizeClosures(&rep, empty); err != nil {
			t.Fatal(err)
		}
		if rep.Shutdown != "close_error_unverified_retirement" || !rep.UnverifiedRetirement || rep.UnverifiedCloseHandles != 0 || rep.retainedDiagnostics[0] != original {
			t.Fatal("pre-handle finalizer erased uncertainty")
		}
	}
	benign := &sqlite.Error{Code: sqlite.CodeError, Op: "memory open", Err: fmt.Errorf("%w: %w: %w", sqlite.ErrUnsupported, pager.ErrUnsupported, vfs.ErrUnsupported)}
	rep := Report{}
	recordOpenFailure(&rep, benign)
	finalizeClosures(&rep, newCloseLedger())
	if rep.Status != "skipped_unsupported" || rep.UnverifiedRetirement || rep.Shutdown != "no_database_handle_returned" {
		t.Fatalf("benign unsupported report %+v", rep)
	}
	extra := Report{}
	recordOpenFailure(&extra, errors.Join(sqlite.ErrUnsupported, errors.New("extra diagnostic")))
	if extra.Status != "failed" {
		t.Fatal("extra diagnostic hidden by unsupported")
	}
}
func TestRetirementDiscoveryAndRenderingAreBounded(t *testing.T) {
	for _, node := range []error{sqlite.ErrPlaintextCleanupPending, sqlite.ErrOpenCleanupPending, sqlite.ErrCloseCleanupPending, sqlite.ErrEncryptedCleanupPending, &sqlite.Error{Code: sqlite.CodeIOErr, Err: errors.New("unknown IO diagnostic")}, &forbiddenOutcomeMethods{}} {
		if !cleanupDiagnostic(node) {
			t.Fatalf("unverified diagnostic missed: %T", node)
		}
	}
	cycle := &sqlite.Error{Code: sqlite.CodeBusy}
	cycle.Err = cycle
	if !cleanupDiagnostic(cycle) || !strings.Contains(diagnosticText(cycle), "graph limit") {
		t.Fatal("unbounded graph")
	}
	var typedNil *sqlite.Error
	terminalNil := reflect.Zero(terminalErrorType).Interface().(error)
	for _, node := range []error{typedNil, terminalNil, &forbiddenOutcomeMethods{}} {
		rep := Report{}
		recordFailure(&rep, node)
		if !rep.UnverifiedRetirement || rep.Status != "failed" || len(rep.Errors) != 1 {
			t.Fatal("unsafe graph not retained")
		}
	}
	rep := Report{Status: "passed"}
	recordFailure(&rep, errors.Join(context.Canceled, errors.New("real diagnostic")))
	if rep.Status != "failed" || len(rep.retainedDiagnostics) != 1 {
		t.Fatal("joined failure discarded")
	}
}
func TestFinalizerInstalledBeforeDatabaseOpen(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var run *ast.FuncDecl
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FuncDecl); ok && fn.Name.Name == "run" {
			run = fn
		}
	}
	if run == nil {
		t.Fatal("run missing")
	}
	var finalizer token.Pos
	var opens []token.Pos
	ast.Inspect(run.Body, func(n ast.Node) bool {
		if d, ok := n.(*ast.DeferStmt); ok {
			found := false
			ast.Inspect(d, func(x ast.Node) bool {
				if id, ok := x.(*ast.Ident); ok && id.Name == "finalizeClosures" {
					found = true
				}
				return true
			})
			if found {
				finalizer = d.Pos()
			}
		}
		if c, ok := n.(*ast.CallExpr); ok {
			if sel, ok := c.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Open" {
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == "sqlite" {
					opens = append(opens, c.Pos())
				}
			}
		}
		return true
	})
	if finalizer == token.NoPos || len(opens) != 2 {
		t.Fatalf("expected one finalizer and initial/reopen paths: %v %v", finalizer, opens)
	}
	for _, pos := range opens {
		if finalizer >= pos {
			t.Fatal("database acquisition before finalizer")
		}
	}
}
func TestHarnessRevisionOnRejectedReport(t *testing.T) {
	c := validConfig()
	c.Profile = "not-supported"
	rep := run(context.Background(), c)
	if rep.HarnessRevision != "ordinary-transactions-v4" || rep.Status != "rejected" || rep.DataDirectory != "" {
		t.Fatal("revision/rejection contract")
	}
}
