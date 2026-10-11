package fixturecleanup

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// This reads source only. It never imports or runs the example packages.
func TestExampleFixtureFinalizersSource(t *testing.T) {
	for _, tc := range []struct {
		name        string
		result      string
		outputCalls int
		closes      []string
	}{
		{"encrypted", "err", 4, []string{
			"run: defer func() { err = errors.Join(err, db.Close()) }()",
			"run: defer func() { err = errors.Join(err, reopened.Close()) }()",
			"run: defer func() { err = errors.Join(err, restored.Close()) }()",
			"checkNote: defer func() { err = errors.Join(err, rows.Close()) }()",
		}},
		{"encrypted-wal", "err", 4, []string{
			"run: defer func() { err = errors.Join(err, db.Close()) }()",
			"run: defer func() { err = errors.Join(err, reopened.Close()) }()",
			"run: defer func() { err = errors.Join(err, reader.Close()) }()",
			"checkNote: defer func() { err = errors.Join(err, rows.Close()) }()",
		}},
		{"encrypted-shared", "err", 4, []string{
			"run: defer func() { err = errors.Join(err, a.Close()) }()",
			"run: defer func() { err = errors.Join(err, b.Close()) }()",
			"run: defer func() { err = errors.Join(err, c.Close()) }()",
			"checkValue: defer func() { err = errors.Join(err, rows.Close()) }()",
		}},
		{"encrypted-cli", "resultErr", 3, []string{
			"run: defer func() { resultErr = errors.Join(resultErr, db.Close()) }()",
			"verify: defer func() { resultErr = errors.Join(resultErr, db.Close()) }()",
			"verify: defer func() { resultErr = errors.Join(resultErr, rows.Close()) }()",
		}},
		{"native-check", "err", 1, []string{
			"create: defer func() { retErr = errors.Join(retErr, db.Close()) }()",
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, filepath.Join("..", "..", tc.name, "main.go"), nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			var run *ast.FuncDecl
			var closes []string
			for _, decl := range file.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok {
					continue
				}
				if fn.Name.Name == "run" {
					run = fn
				}
				ast.Inspect(fn.Body, func(n ast.Node) bool {
					if d, ok := n.(*ast.DeferStmt); ok && hasSelectorCall(d.Call, "Close") {
						closes = append(closes, fn.Name.Name+": "+sourceText(t, fset, d))
					}
					return true
				})
			}
			if !reflect.DeepEqual(closes, tc.closes) {
				t.Fatalf("Close aggregation changed:\ngot %q\nwant %q", closes, tc.closes)
			}
			if run == nil || run.Type.Results == nil || len(run.Type.Results.List) != 1 {
				t.Fatal("run must have one named error result")
			}
			result := run.Type.Results.List[0]
			if len(result.Names) != 1 || result.Names[0].Name != tc.result || sourceText(t, fset, result.Type) != "error" {
				t.Fatal("run result must keep the expected error name")
			}
			stmts := run.Body.List
			mkdir := -1
			for i, stmt := range stmts {
				if a, ok := stmt.(*ast.AssignStmt); ok && len(a.Rhs) == 1 {
					if call, ok := a.Rhs[0].(*ast.CallExpr); ok && sourceText(t, fset, call.Fun) == "os.MkdirTemp" {
						if mkdir != -1 {
							t.Fatal("multiple temporary directories")
						}
						mkdir = i
					}
				}
			}
			if mkdir < 0 || mkdir+3 >= len(stmts) {
				t.Fatal("missing temporary-directory finalizer")
			}
			if sourceText(t, fset, stmts[mkdir+1]) != "if err != nil { return err }" || sourceText(t, fset, stmts[mkdir+2]) != "completed := false" {
				t.Fatal("completion flag must follow the successful MkdirTemp guard")
			}
			want := fmt.Sprintf("defer func() { %s = fixturecleanup.Finalize(dir, completed, %s, os.RemoveAll) }()", tc.result, tc.result)
			finalizer, ok := stmts[mkdir+3].(*ast.DeferStmt)
			if !ok || sourceText(t, fset, finalizer) != want {
				t.Fatalf("finalizer must delay evaluation of completion and final error; want %s", want)
			}
			if len(finalizer.Call.Args) != 0 {
				t.Fatal("deferred closure must not capture early argument values")
			}
			ast.Inspect(run.Body, func(n ast.Node) bool {
				if d, ok := n.(*ast.DeferStmt); ok && d != finalizer && d.Pos() < finalizer.Pos() {
					t.Error("resource defer precedes the directory finalizer")
				}
				return true
			})
			if len(stmts) < 2 || sourceText(t, fset, stmts[len(stmts)-2]) != "completed = true" || sourceText(t, fset, stmts[len(stmts)-1]) != "return nil" {
				t.Fatal("ordinary completion must be marked only at the final successful return")
			}
			completionWrites, removals := 0, 0
			ast.Inspect(run.Body, func(n ast.Node) bool {
				if a, ok := n.(*ast.AssignStmt); ok {
					for _, lhs := range a.Lhs {
						if ident, ok := lhs.(*ast.Ident); ok && ident.Name == "completed" {
							completionWrites++
						}
					}
				}
				if s, ok := n.(*ast.SelectorExpr); ok && sourceText(t, fset, s) == "os.RemoveAll" {
					removals++
				}
				return true
			})
			if completionWrites != 2 || removals != 1 {
				t.Fatalf("completion assignments = %d; removal references = %d", completionWrites, removals)
			}
			checkRunOutputs(t, fset, run, tc.outputCalls)
		})
	}
}

func checkRunOutputs(t *testing.T, fset *token.FileSet, run *ast.FuncDecl, want int) {
	t.Helper()
	parents := make(map[ast.Node]ast.Node)
	var stack []ast.Node
	ast.Inspect(run.Body, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		if len(stack) != 0 {
			parents[n] = stack[len(stack)-1]
		}
		stack = append(stack, n)
		return true
	})
	calls := 0
	ast.Inspect(run.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		fun := sourceText(t, fset, call.Fun)
		if !strings.HasPrefix(fun, "fmt.Print") && !strings.HasPrefix(fun, "fmt.Fprint") && fun != "json.NewEncoder(os.Stdout).Encode" {
			return true
		}
		calls++
		assign, ok := parents[call].(*ast.AssignStmt)
		if !ok || len(assign.Lhs) == 0 || sourceText(t, fset, assign.Lhs[len(assign.Lhs)-1]) != "err" {
			t.Errorf("unchecked ordinary output: %s", fun)
			return true
		}
		guard, ok := parents[assign].(*ast.IfStmt)
		if !ok || guard.Init != assign || sourceText(t, fset, guard.Cond) != "err != nil" || sourceText(t, fset, guard.Body) != "{ return err }" {
			t.Errorf("output error must return before completion: %s", fun)
		}
		return true
	})
	if calls != want {
		t.Fatalf("ordinary output calls = %d, want %d", calls, want)
	}
}

func hasSelectorCall(n ast.Node, name string) bool {
	found := false
	ast.Inspect(n, func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			if s, ok := c.Fun.(*ast.SelectorExpr); ok && s.Sel.Name == name {
				found = true
			}
		}
		return true
	})
	return found
}

func sourceText(t *testing.T, fset *token.FileSet, n ast.Node) string {
	t.Helper()
	var out bytes.Buffer
	if err := format.Node(&out, fset, n); err != nil {
		t.Fatal(err)
	}
	return strings.Join(strings.Fields(out.String()), " ")
}
