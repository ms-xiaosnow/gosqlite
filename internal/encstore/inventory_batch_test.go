package encstore

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"reflect"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/encryption"
)

// These diagnostics are inert memory values. No callbacks replace native
// directory, file, lock or cleanup operations, and inventory is never invoked.
type inventoryBatchMemoryDetail struct{ label string }

func (e *inventoryBatchMemoryDetail) Error() string { return e.label }

type inventoryBatchMemoryOpaque []byte

func (inventoryBatchMemoryOpaque) Error() string { panic("must not format opaque memory error") }
func (inventoryBatchMemoryOpaque) Is(error) bool { panic("must not match opaque memory error") }
func (inventoryBatchMemoryOpaque) As(any) bool   { panic("must not cast opaque memory error") }

func TestInventoryBatchMemoryNilAndExactEOF(t *testing.T) {
	entry := errors.New("entry diagnostic")
	for _, readErr := range []error{nil, io.EOF} {
		for _, entryErr := range []error{nil, entry, errors.Join(entry, encryption.ErrLimit)} {
			if got := inventoryBatchError(readErr, entryErr); got != entryErr {
				t.Fatal("clean batch changed original entry diagnostic")
			}
		}
	}
}

func TestInventoryBatchMemoryPreservesOrderedErrorGraph(t *testing.T) {
	readA := &inventoryBatchMemoryDetail{"read A"}
	readB := errors.New("read B")
	entryA := errors.New("entry A")
	for _, readErr := range []error{
		readA, fmt.Errorf("read wrapper: %w", readA), errors.Join(readA, readB),
		errors.Join(io.EOF, readA), fmt.Errorf("read wrapper: %w", errors.Join(io.EOF, readA)),
	} {
		if got := inventoryBatchError(readErr, nil); got != readErr {
			t.Fatal("lone read graph changed")
		}
		for _, entryErr := range []error{entryA, errors.Join(entryA, encryption.ErrLimit), readErr} {
			got := inventoryBatchError(readErr, entryErr)
			join, ok := got.(interface{ Unwrap() []error })
			if !ok {
				t.Fatal("missing diagnostic join")
			}
			nodes := join.Unwrap()
			if len(nodes) != 2 || nodes[0] != readErr || nodes[1] != entryErr {
				t.Fatal("original read/entry graph was flattened, reordered or deduplicated")
			}
			if !errors.Is(got, readA) || !errors.Is(got, entryErr) {
				t.Fatal("diagnostic reachability lost")
			}
			var detail *inventoryBatchMemoryDetail
			if !errors.As(got, &detail) || detail != readA {
				t.Fatal("typed read provenance lost")
			}
		}
	}
}

func TestInventoryBatchMemoryWrappedJoinedEOFAreDiagnostics(t *testing.T) {
	other := errors.New("additional read diagnostic")
	entry := encryption.ErrFormat
	for _, readErr := range []error{
		fmt.Errorf("wrapped end: %w", io.EOF), errors.Join(io.EOF),
		errors.Join(io.EOF, other), errors.Join(other, io.EOF),
		fmt.Errorf("nested end: %w", errors.Join(io.EOF, other)),
	} {
		if !errors.Is(readErr, io.EOF) {
			t.Fatal("invalid memory fixture")
		}
		if got := inventoryBatchError(readErr, nil); got != readErr {
			t.Fatal("nonliteral EOF was suppressed")
		}
		got := inventoryBatchError(readErr, entry)
		nodes := got.(interface{ Unwrap() []error }).Unwrap()
		if len(nodes) != 2 || nodes[0] != readErr || nodes[1] != entry {
			t.Fatal("EOF-bearing read graph lost beside entry diagnostic")
		}
	}
}

func TestInventoryBatchMemoryDoesNotInspectOpaqueErrors(t *testing.T) {
	// A noncomparable error value detects unsafe equality/deduplication; its
	// methods also make any diagnostic formatting/matching visible in memory.
	readErr := inventoryBatchMemoryOpaque{1}
	entryErr := inventoryBatchMemoryOpaque{2}
	if got := inventoryBatchError(readErr, nil); !reflect.DeepEqual(got, readErr) {
		t.Fatal("opaque read value changed")
	}
	if got := inventoryBatchError(io.EOF, entryErr); !reflect.DeepEqual(got, entryErr) {
		t.Fatal("opaque entry value changed")
	}
	got := inventoryBatchError(readErr, entryErr)
	nodes := got.(interface{ Unwrap() []error }).Unwrap()
	if len(nodes) != 2 || !reflect.DeepEqual(nodes[0], readErr) || !reflect.DeepEqual(nodes[1], entryErr) {
		t.Fatal("opaque error graph changed")
	}
}

func TestInventoryBatchSourceEveryEntryExitKeepsReadDiagnostic(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "inventory.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var inventory *ast.FuncDecl
	for _, decl := range file.Decls {
		if f, ok := decl.(*ast.FuncDecl); ok && f.Name.Name == "inventory" {
			inventory = f
		}
	}
	if inventory == nil {
		t.Fatal("inventory function not found")
	}
	var batchLoop *ast.RangeStmt
	reads, literalEOF, readReturns := 0, 0, 0
	ast.Inspect(inventory, func(n ast.Node) bool {
		if loop, ok := n.(*ast.RangeStmt); ok {
			if name, ok := loop.X.(*ast.Ident); ok && name.Name == "batch" {
				if batchLoop != nil {
					t.Fatal("multiple batch loops need an explicit diagnostic audit")
				}
				batchLoop = loop
			}
		}
		if assignment, ok := n.(*ast.AssignStmt); ok {
			for _, lhs := range assignment.Lhs {
				if name, ok := lhs.(*ast.Ident); ok && name.Name == "readErr" {
					reads++
					if assignment.Tok != token.DEFINE || len(assignment.Rhs) != 1 {
						t.Fatal("read diagnostic overwritten")
					}
					call, ok := assignment.Rhs[0].(*ast.CallExpr)
					if !ok {
						t.Fatal("read diagnostic not obtained from enumeration")
					}
					method, ok := call.Fun.(*ast.SelectorExpr)
					if !ok || method.Sel.Name != "ReadDir" {
						t.Fatal("read diagnostic not obtained from ReadDir")
					}
				}
			}
		}
		if condition, ok := n.(*ast.BinaryExpr); ok && condition.Op == token.EQL {
			lhs, lOK := condition.X.(*ast.Ident)
			rhs, rOK := condition.Y.(*ast.SelectorExpr)
			if lOK && rOK && lhs.Name == "readErr" && rhs.Sel.Name == "EOF" {
				literalEOF++
			}
		}
		if returned, ok := n.(*ast.ReturnStmt); ok && len(returned.Results) == 2 {
			if name, ok := returned.Results[1].(*ast.Ident); ok && name.Name == "readErr" {
				readReturns++
			}
		}
		return true
	})
	if batchLoop == nil || reads != 1 || literalEOF != 1 || readReturns != 1 {
		t.Fatal("batch/read/exact-EOF/unchanged-read return contract", reads, literalEOF, readReturns)
	}
	exits := 0
	ast.Inspect(batchLoop.Body, func(n ast.Node) bool {
		returned, ok := n.(*ast.ReturnStmt)
		if !ok {
			return true
		}
		exits++
		if len(returned.Results) != 2 {
			t.Fatal("unexpected entry exit")
		}
		call, ok := returned.Results[1].(*ast.CallExpr)
		if !ok || len(call.Args) != 2 {
			t.Fatal("entry exit drops partial ReadDir diagnostic")
		}
		fn, fOK := call.Fun.(*ast.Ident)
		read, rOK := call.Args[0].(*ast.Ident)
		if !fOK || !rOK || fn.Name != "inventoryBatchError" || read.Name != "readErr" {
			t.Fatal("entry exit does not preserve read diagnostic first")
		}
		return true
	})
	if exits != 6 {
		t.Fatal("entry exit set changed; quota/classifier/open/close/size/charge need review", exits)
	}
}
