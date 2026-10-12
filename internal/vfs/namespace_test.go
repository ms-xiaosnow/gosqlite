package vfs

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"
)

// Source-only proof of the accessor boundary: no handles, alias creation,
// namespace changes, native metadata calls or descriptor retirement are run.
func TestNamespaceSourceOriginalParentContinuity(t *testing.T) {
	f, err := parser.ParseFile(token.NewFileSet(), "namespace.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var fn *ast.FuncDecl
	for _, decl := range f.Decls {
		if d, ok := decl.(*ast.FuncDecl); ok && d.Name.Name == "Namespace" {
			fn = d
		}
	}
	if fn == nil {
		t.Fatal("missing managed namespace accessor")
	}
	var validated, parentReturned token.Pos
	locks, unlocks := 0, 0
	ast.Inspect(fn, func(n ast.Node) bool {
		if kv, ok := n.(*ast.KeyValueExpr); ok {
			if key, ok := kv.Key.(*ast.Ident); ok && key.Name == "Parent" {
				if value, ok := kv.Value.(*ast.SelectorExpr); ok && value.Sel.Name == "parent" {
					if id, ok := value.X.(*ast.Ident); ok && id.Name == "f" {
						parentReturned = kv.Pos()
					}
				}
			}
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			t.Fatal("unexpected accessor call")
			return true
		}
		switch sel.Sel.Name {
		case "validatePathLocked":
			validated = call.Pos()
		case "Lock":
			locks++
		case "Unlock":
			unlocks++
		case "Base":
		default:
			t.Fatal("new unmanaged accessor effect", sel.Sel.Name)
		}
		return true
	})
	if validated == 0 || parentReturned == 0 || validated > parentReturned || locks != 1 || unlocks != 1 {
		t.Fatal(validated, parentReturned, locks, unlocks)
	}
}
