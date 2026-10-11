package pager

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"testing"
)

// All owners and diagnostics in these tests are descriptor-free models. Paths
// are equality labels, never passed to filesystem or native recovery functions.
func TestReaderRecoveryMemoryRetiredPathProof(t *testing.T) {
	reg := &plainRetirementRegistry{limit: 1}
	g, err := reg.reserve()
	if err != nil {
		t.Fatal(err)
	}
	main := &plainMemoryOwner{}
	g.main = main
	presence := &rollbackJournalPresence{path: "memory-path"}
	proof := finishReaderProbe(g, "memory-path", presence)
	if !ReaderProbeRecoveryReady(proof, "memory-path") || !errors.Is(proof, ErrRecoveryRequired) || !g.done || main.calls != 1 || len(reg.groups) != 0 {
		t.Fatal("retired proof lost", proof)
	}
	for _, candidate := range []error{nil, ErrRecoveryRequired, presence, fmt.Errorf("wrap: %w", proof), errors.Join(proof), errors.Join(proof, io.ErrUnexpectedEOF), errors.Join(proof, ErrPoisoned), errors.Join(proof, proof)} {
		if ReaderProbeRecoveryReady(candidate, "memory-path") {
			t.Fatal("untrusted or composed error authorized recovery", candidate)
		}
	}
	if ReaderProbeRecoveryReady(proof, "other-path") {
		t.Fatal("proof was not path-bound")
	}
	// Readiness must neither trust a typed-nil proof nor traverse an arbitrary
	// error implementation. This is a memory-only hostile-error model.
	var nilProof *retiredReaderJournal
	for _, candidate := range []error{nilProof, readerRecoveryNoTraversal{}} {
		if ReaderProbeRecoveryReady(candidate, "memory-path") {
			t.Fatal("untrusted identity authorized recovery")
		}
	}
}

type readerRecoveryNoTraversal struct{}

func (readerRecoveryNoTraversal) Error() string { return "memory-only untrusted error" }
func (readerRecoveryNoTraversal) Is(error) bool { panic("readiness invoked Is") }
func (readerRecoveryNoTraversal) As(any) bool   { panic("readiness invoked As") }
func (readerRecoveryNoTraversal) Unwrap() error { panic("readiness invoked Unwrap") }

func TestReaderRecoveryMemoryDiagnosticAndOwnershipRefusal(t *testing.T) {
	for _, mode := range []string{"consumed", "pending", "nil-pending", "terminal", "nested", "mixed", "wrapped", "path", "untyped"} {
		t.Run(mode, func(t *testing.T) {
			reg := &plainRetirementRegistry{limit: 1}
			g, err := reg.reserve()
			if err != nil {
				t.Fatal(err)
			}
			main := &plainMemoryOwner{cause: io.ErrUnexpectedEOF}
			g.main = main
			var cause error = &rollbackJournalPresence{path: "memory"}
			switch mode {
			case "consumed":
				main.consumed = true
			case "pending":
				main.remaining = 1
			case "nil-pending":
				main.nilPending = true
			case "terminal":
				main.terminal = true
			case "nested":
				g.nested = []plainResource{&plainMemoryOwner{nilPending: true}}
			case "mixed":
				cause = errors.Join(cause, io.ErrUnexpectedEOF)
			case "wrapped":
				cause = fmt.Errorf("untrusted: %w", cause)
			case "path":
				cause = &rollbackJournalPresence{path: "other"}
			case "untyped":
				cause = ErrRecoveryRequired
			}
			got := finishReaderProbe(g, "memory", cause)
			if ReaderProbeRecoveryReady(got, "memory") || !errors.Is(got, ErrRecoveryRequired) {
				t.Fatal("unsafe proof or original diagnostic lost", got)
			}
			if mode == "nested" && main.calls != 0 {
				t.Fatal("main retired before pending child")
			}
			if mode == "pending" {
				if !errors.Is(got, ErrPlaintextCleanupPending) || len(reg.snapshot()) != 1 {
					t.Fatal("pending owner lost")
				}
				if err := RetryPlaintextCleanup(got); err != nil {
					t.Fatal(err)
				}
				if ReaderProbeRecoveryReady(got, "memory") {
					t.Fatal("retry retroactively authorized recovery")
				}
			}
		})
	}
}

func TestReaderRecoverySourceProofBoundary(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "reader_recovery.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var constructor, checker *ast.FuncDecl
	for _, decl := range f.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			switch fn.Name.Name {
			case "finishReaderProbe":
				constructor = fn
			case "ReaderProbeRecoveryReady":
				checker = fn
			}
		}
	}
	if constructor == nil || checker == nil {
		t.Fatal("proof boundary missing")
	}
	first, ok := constructor.Body.List[0].(*ast.AssignStmt)
	if !ok || len(first.Rhs) != 1 {
		t.Fatal("retirement must precede proof inspection")
	}
	call, ok := first.Rhs[0].(*ast.CallExpr)
	if !ok {
		t.Fatal("retirement call missing")
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "begin" {
		t.Fatal("constructor cleanup contract changed")
	}
	ast.Inspect(checker, func(n ast.Node) bool {
		if _, ok := n.(*ast.CallExpr); ok {
			t.Fatal("proof checker must not unwrap, clean up or perform native work")
		}
		return true
	})
}
