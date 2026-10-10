package pager

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"reflect"
	"strings"
	"testing"
)

// These laws use only descriptor-free owners and source ASTs. They never call
// the native recovery entry point or its path/descriptor helpers.
func TestPlainRecoveryCleanupMemoryChildBarrierAndSnapshot(t *testing.T) {
	for _, touched := range []bool{false, true} {
		for _, stage := range []string{"nested", "journal", "main"} {
			t.Run(fmt.Sprintf("touched=%v/%s", touched, stage), func(t *testing.T) {
				reg := &plainRetirementRegistry{limit: 1}
				g, nested, main, events := plainMemoryGroup(t, reg)
				journal := &plainMemoryOwner{name: "journal", events: events, cause: errors.New("memory journal diagnostic")}
				g.temporary = journal
				failing := map[string]*plainMemoryOwner{"nested": nested, "journal": journal, "main": main}[stage]
				failing.remaining = 1
				operation := errors.New("memory recovery operation")
				token := finishPlainRecovery(g, operation, touched, false)
				if !errors.Is(token, ErrPlaintextCleanupPending) || !errors.Is(token, operation) || !errors.Is(token, failing.cause) || errors.Is(token, ErrRecoveryRequired) != touched {
					t.Fatal(token)
				}
				if stage != "main" && main.calls != 0 || stage == "nested" && journal.calls != 0 {
					t.Fatal("protector retired before incomplete child", *events)
				}
				if len(reg.snapshot()) != 1 || !errors.Is(reg.snapshot()[0], operation) {
					t.Fatal("discarded return loses ownership")
				}
				if _, err := reg.reserve(); !errors.Is(err, ErrLimit) {
					t.Fatal("pending recovery stopped counting toward admission", err)
				}
				if err := RetryPlaintextCleanup(reg.snapshot()[0]); err != nil || !g.done || len(reg.groups) != 0 {
					t.Fatal(err)
				}
				if main.calls != 1 && stage != "main" || journal.calls != 1 && stage != "journal" {
					t.Fatal("completed stage retried", *events)
				}
				before := append([]string(nil), (*events)...)
				if err := RetryPlaintextCleanup(token); err != nil || !reflect.DeepEqual(before, *events) {
					t.Fatal("retired token repeated work", err)
				}
			})
		}
	}
}

func TestPlainRecoveryCleanupMemoryExplicitChildCloseIsNotRepeated(t *testing.T) {
	for _, touched := range []bool{false, true} {
		for _, nilPending := range []bool{false, true} {
			reg := &plainRetirementRegistry{limit: 1}
			g, err := reg.reserve()
			if err != nil {
				t.Fatal(err)
			}
			main := &plainMemoryOwner{name: "main"}
			journal := &plainMemoryOwner{name: "journal", remaining: 1, nilPending: nilPending, cause: errors.New("memory close diagnostic")}
			g.main, g.temporary = main, journal
			operation := errors.New("memory candidate read or replay cause")
			first := g.closeTemporary(operation)
			token := finishPlainRecovery(g, first, touched, false)
			if !errors.Is(token, ErrPlaintextCleanupPending) || !errors.Is(token, operation) || errors.Is(token, ErrRecoveryRequired) != touched || journal.calls != 1 || main.calls != 0 {
				t.Fatal("deferred closure repeated failed child or lost class", token)
			}
			journal.nilPending, journal.remaining = false, 0
			if err := RetryPlaintextCleanup(token); err != nil || !g.done || journal.calls != 2 || main.calls != 1 {
				t.Fatal("retry did not retire child before main", err)
			}
		}
	}
}

func TestPlainRecoveryCleanupMemoryClassificationAndConsumedDiagnostics(t *testing.T) {
	for _, touched := range []bool{false, true} {
		for _, removed := range []bool{false, true} {
			for _, operationError := range []bool{false, true} {
				reg := &plainRetirementRegistry{limit: 1}
				g, err := reg.reserve()
				if err != nil {
					t.Fatal(err)
				}
				main := &plainMemoryOwner{consumed: true, cause: errors.New("memory consumed diagnostic")}
				g.main = main
				var cause error
				if operationError {
					cause = errors.New("memory operation")
				}
				err = finishPlainRecovery(g, cause, touched, removed)
				if !errors.Is(err, main.cause) || cause != nil && !errors.Is(err, cause) || !g.done || errors.Is(err, ErrPlaintextCleanupPending) || len(reg.snapshot()) != 0 {
					t.Fatal("consumed diagnostic lost or mistaken for ownership", err)
				}
				if errors.Is(err, ErrCommitUncertain) != (touched && removed) || errors.Is(err, ErrRecoveryRequired) != (touched && !removed) {
					t.Fatal("wrong recovery outcome class", touched, removed, err)
				}
			}
		}
	}
	reg := &plainRetirementRegistry{limit: 1}
	g, _ := reg.reserve()
	g.main = &plainMemoryOwner{}
	if err := finishPlainRecovery(g, nil, true, true); err != nil || !g.done {
		t.Fatal("successful disposal gained an error", err)
	}
}

func TestPlainRecoveryCleanupMemoryMixedEOFPreservesCause(t *testing.T) {
	reg := &plainRetirementRegistry{limit: 1}
	g, _ := reg.reserve()
	main, candidate := &plainMemoryOwner{}, &plainMemoryOwner{}
	g.main, g.temporary = main, candidate
	operation := errors.New("memory additional candidate diagnostic")
	cause := errors.Join(io.EOF, operation)
	cause = g.closeTemporary(cause)
	cause = finishPlainRecovery(g, cause, false, false)
	if !errors.Is(cause, io.EOF) || !errors.Is(cause, operation) || errors.Is(cause, ErrPlaintextCleanupPending) || !g.done || candidate.calls != 1 || main.calls != 1 {
		t.Fatal("mixed EOF cause was discarded", cause)
	}
}

func TestPlainRecoveryCleanupMemoryAdmissionSequentialSlot(t *testing.T) {
	reg := &plainRetirementRegistry{limit: 1}
	g, err := reg.reserve()
	if err != nil {
		t.Fatal(err)
	}
	if len(reg.snapshot()) != 0 {
		t.Fatal("opening recovery prematurely marked pending")
	}
	if _, err = reg.reserve(); !errors.Is(err, ErrLimit) {
		t.Fatal("opening recovery not charged", err)
	}
	main := &plainMemoryOwner{name: "main"}
	g.main = main
	for _, phase := range []string{"candidate", "replay"} {
		if err = g.reserveTemporary(); err != nil {
			t.Fatal(err)
		}
		if err = g.reserveTemporary(); !errors.Is(err, ErrLimit) {
			t.Fatal("slot duplicated", err)
		}
		g.acquiredTemporary(nil) // memory model of an empty acquisition
		child := &plainMemoryOwner{name: phase}
		g.temporary = child
		if err = g.closeTemporary(nil); err != nil || child.calls != 1 || main.calls != 0 {
			t.Fatal("temporary retirement consumed main", err)
		}
	}
	if err = finishPlainRecovery(g, nil, false, false); err != nil || !g.done || main.calls != 1 {
		t.Fatal(err)
	}
}

func TestPlainRecoveryCleanupMemoryUnsafeErrorAndDiscoveryBeforeEffects(t *testing.T) {
	reg := &plainRetirementRegistry{limit: 2}
	g, _, main, _ := plainMemoryGroup(t, reg)
	g.nested = nil
	journal := &plainMemoryOwner{name: "journal"}
	g.temporary = journal
	token := finishPlainRecovery(g, plainMemoryCycle{}, true, false)
	if !errors.Is(token, ErrPlaintextCleanupPending) || !errors.Is(token, ErrLimit) || main.calls != 0 || journal.calls != 0 {
		t.Fatal("unsafe operation graph released protector", token)
	}
	other, child, otherMain, _ := plainMemoryGroup(t, reg)
	child.remaining = 1
	otherToken := finishPlainRecovery(other, nil, false, false)
	input := errors.Join(otherToken, plainMemoryCycle{}, token)
	if err := RetryPlaintextCleanup(input); !errors.Is(err, ErrLimit) || child.calls != 1 || otherMain.calls != 0 || main.calls != 0 || journal.calls != 0 {
		t.Fatal("partial token discovery performed retirement", err)
	}
	if err := RetryPlaintextCleanup(errors.Join(otherToken, otherToken)); err != nil || !other.done || child.calls != 2 {
		t.Fatal("duplicate discovery repeated owner", err)
	}
}

// Static source laws complement the owner models without executing recovery or
// providing a native fault hook. They pin admission/adoption and retry surfaces.
func TestPlainRecoveryCleanupSourceAdmissionAndRetirementBoundary(t *testing.T) {
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, "recovery.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	functions := map[string]*ast.FuncDecl{}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			functions[fn.Name.Name] = fn
		}
	}
	callName := func(call *ast.CallExpr) string {
		switch fn := call.Fun.(type) {
		case *ast.Ident:
			return fn.Name
		case *ast.SelectorExpr:
			if x, ok := fn.X.(*ast.Ident); ok {
				return x.Name + "." + fn.Sel.Name
			}
		}
		return ""
	}
	calls := func(fn *ast.FuncDecl) []string {
		var out []string
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if call, ok := n.(*ast.CallExpr); ok {
				out = append(out, callName(call))
			}
			return true
		})
		return out
	}
	before := func(names []string, a, b string) {
		t.Helper()
		ai, bi := -1, -1
		for i, name := range names {
			if name == a && ai < 0 {
				ai = i
			}
			if name == b && bi < 0 {
				bi = i
			}
		}
		if ai < 0 || bi < 0 || ai >= bi {
			t.Fatalf("expected %s before %s: %v", a, b, names)
		}
	}
	recovery := calls(functions["RecoverRollback"])
	before(recovery, "plainRetirements.reserve", "filepath.Abs")
	before(recovery, "plainRetirements.reserve", "resolveNativePath")
	before(recovery, "plainRetirements.reserve", "openRecoveryFile")
	before(recovery, "cleanup.closeTemporary", "lockExclusive")
	childCloses, unlinkAt, finalChildAt := 0, -1, -1
	for i, name := range recovery {
		if name == "cleanup.closeTemporary" {
			childCloses++
			finalChildAt = i
		}
		if name == "os.Remove" {
			unlinkAt = i
		}
	}
	if childCloses != 2 || finalChildAt < 0 || unlinkAt <= finalChildAt {
		t.Fatal("journal retirement no longer precedes unlink", recovery)
	}
	literalEOF := false
	ast.Inspect(functions["RecoverRollback"].Body, func(n ast.Node) bool {
		if condition, ok := n.(*ast.BinaryExpr); ok && condition.Op == token.EQL {
			left, lhs := condition.X.(*ast.Ident)
			right, rhs := condition.Y.(*ast.SelectorExpr)
			if lhs && rhs && left.Name == "readErr" && right.Sel.Name == "EOF" {
				if qualifier, ok := right.X.(*ast.Ident); ok && qualifier.Name == "io" {
					literalEOF = true
				}
			}
		}
		return true
	})
	if !literalEOF {
		t.Fatal("candidate EOF suppression must preserve joined diagnostics")
	}
	journal := calls(functions["recoveryOpenJournal"])
	before(journal, "cleanup.reserveTemporary", "nativeLstatForCreate")
	// Check successful acquisition's immediate next statement specifically.
	found := false
	body := functions["recoveryOpenJournal"].Body.List
	for i, stmt := range body {
		assign, ok := stmt.(*ast.AssignStmt)
		if !ok || len(assign.Rhs) != 1 {
			continue
		}
		call, ok := assign.Rhs[0].(*ast.CallExpr)
		if !ok || callName(call) != "openJournal" {
			continue
		}
		next, ok := body[i+1].(*ast.ExprStmt)
		if !ok {
			t.Fatal("journal acquisition not immediately adopted")
		}
		adopt, ok := next.X.(*ast.CallExpr)
		if !ok || callName(adopt) != "cleanup.acquiredTemporary" {
			t.Fatal("journal acquisition not immediately adopted")
		}
		found = true
	}
	if !found {
		t.Fatal("journal acquisition missing")
	}
	for _, fn := range functions {
		for _, name := range calls(fn) {
			if name == "closeNativeFile" || name == "os.IsNotExist" {
				t.Fatal("raw cleanup or ambiguous absence returned", name)
			}
		}
	}
	finish, err := parser.ParseFile(set, "plain_recovery_cleanup.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	ast.Inspect(finish, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			name := callName(call)
			if name != "errors.Join" && name != "cleanup.beginClass" && name != "cleanup.childClosePending" {
				t.Fatal("retirement retained or repeated logical/native work", name)
			}
		}
		if field, ok := n.(*ast.Field); ok {
			if _, callback := field.Type.(*ast.FuncType); callback {
				t.Fatal("cleanup helper acquired callback")
			}
		}
		return true
	})
	if !strings.Contains(strings.Join(recovery, ","), "finishPlainRecovery") {
		t.Fatal("recovery lost its ownership defer")
	}
}
