package main

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// The fakes below are descriptor-free memory objects, not database/native hooks.
type memoryCloseHandle struct {
	calls int
	err   error
}

func (m *memoryCloseHandle) Close() error { m.calls++; return m.err }
func TestCloseLedgerRetainsFailures(t *testing.T) {
	l := newCloseLedger()
	bad := &memoryCloseHandle{err: errors.New("memory diagnostic")}
	good := &memoryCloseHandle{}
	failed := l.acquire(bad, "reader")
	writer := l.acquire(good, "writer")
	if l.close(failed) == nil || l.failures != 1 || len(l.live) != 2 || failed.handle != bad || failed.diagnostic == nil {
		t.Fatal("failed owner lost")
	}
	// A hypothetical second nil result must not be mistaken for proof that the
	// one-shot original owner retired. No automatic retry is invoked.
	bad.err = nil
	if l.close(failed) == nil || bad.calls != 1 {
		t.Fatal("failed owner blindly retried")
	}
	if err := l.close(writer); err != nil {
		t.Fatal(err)
	}
	if l.shutdown() != "close_error_unverified_retirement" || len(l.live) != 1 || failed.handle != bad {
		t.Fatal("writer success hid reader failure")
	}
	if err := l.closeRemaining(); err != nil || bad.calls != 1 {
		t.Fatal("unrequested retry", err)
	}
	clean := newCloseLedger()
	a, b := &memoryCloseHandle{}, &memoryCloseHandle{}
	clean.acquire(a, "one")
	clean.acquire(b, "two")
	if clean.closeRemaining() != nil || a.calls != 1 || b.calls != 1 || len(clean.live) != 0 || clean.shutdown() != "workers_joined_normal_close" {
		t.Fatal("clean ledger")
	}
}
func TestCleanupDiagnosticLatchDiscovery(t *testing.T) {
	if !cleanupDiagnostic(cleanupJoin(nil, errors.New("memory rows close"))) || !cleanupDiagnostic(errors.Join(errors.New("operation"), cleanupJoin(nil, errors.New("memory tx rollback")))) {
		t.Fatal("cleanup marker lost")
	}
	if cleanupDiagnostic(errors.New("model mismatch")) || cleanupDiagnostic(nil) {
		t.Fatal("plain memory failure incorrectly marked unverified")
	}
}
func TestDBCloseOnlyThroughLedger(t *testing.T) {
	// Source contract: native DB Close is centralized in the ledger. In
	// particular, the epoch error path cannot nil out a failed DB owner.
	for _, path := range []string{"main.go", "workload.go"} {
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "Close" {
				return true
			}
			id, ok := sel.X.(*ast.Ident)
			if ok && (id.Name == "db" || id.Name == "writer") {
				t.Errorf("raw DB.Close bypass in %s", path)
			}
			return true
		})
	}
}

func TestFinalClosureReportCannotEraseReaderFailure(t *testing.T) {
	l := newCloseLedger()
	reader := l.acquire(&memoryCloseHandle{err: errors.New("memory reader close")}, "reader")
	if l.close(reader) == nil {
		t.Fatal("fixture")
	}
	writer := &memoryCloseHandle{}
	l.acquire(writer, "writer")
	rep := Report{Status: "passed", retainedOwners: l}
	if finalizeClosures(&rep, l) != nil {
		t.Fatal("historical reader error must not be retried")
	}
	if rep.Status != "failed" || !rep.UnverifiedRetirement || rep.Shutdown != "close_error_unverified_retirement" || rep.UnverifiedCloseHandles != 1 || rep.CloseFailures != 1 || writer.calls != 1 || reader.handle == nil {
		t.Fatalf("false closure report: %+v", rep)
	}
	// Nested Rows/Tx retirement diagnostics remain unverified even if all DB
	// handles subsequently close without a fresh diagnostic.
	clean := newCloseLedger()
	clean.acquire(&memoryCloseHandle{}, "writer")
	nested := Report{Status: "failed", UnverifiedRetirement: true}
	if finalizeClosures(&nested, clean) != nil || nested.Shutdown != "close_error_unverified_retirement" || !nested.UnverifiedRetirement {
		t.Fatal("nested retirement diagnostic erased")
	}
}
