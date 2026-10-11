package main

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"testing"
)

// This source-level regression checks the production defer without injecting
// any database, native I/O or Rows.Close failure.
func TestQuerySourceJoinsRowsCloseError(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if err := queryRowsCloseAssignment(source); err != nil {
		t.Fatal(err)
	}
}

func TestQuerySourceRejectsLegacyRowsClose(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	fixed := []byte("defer func() { resultErr = errors.Join(resultErr, rows.Close()) }()")
	if bytes.Count(source, fixed) != 1 {
		t.Fatal("expected exactly one production query close defer")
	}
	// Parse an in-memory copy containing the exact former statement. No legacy
	// executable or failing subprocess is built or run.
	legacy := bytes.Replace(source, fixed, []byte("defer rows.Close()"), 1)
	if err := queryRowsCloseAssignment(legacy); err == nil {
		t.Fatal("source assertion accepted the legacy discarded close error")
	}
}

func queryRowsCloseAssignment(source []byte) error {
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, "main.go", source, 0)
	if err != nil {
		return err
	}
	text := func(node ast.Node) string {
		var out bytes.Buffer
		if err := format.Node(&out, set, node); err != nil {
			return ""
		}
		return out.String()
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "runContext" {
			continue
		}
		if fn.Type.Results == nil || len(fn.Type.Results.List) != 1 {
			return fmt.Errorf("runContext must have one named error result")
		}
		result := fn.Type.Results.List[0]
		if len(result.Names) != 1 || result.Names[0].Name != "resultErr" || text(result.Type) != "error" {
			return fmt.Errorf("runContext must return named resultErr error")
		}
		for _, stmt := range fn.Body.List {
			sw, ok := stmt.(*ast.SwitchStmt)
			if !ok || text(sw.Tag) != "args[0]" {
				continue
			}
			for _, stmt := range sw.Body.List {
				clause := stmt.(*ast.CaseClause)
				if len(clause.List) != 1 || text(clause.List[0]) != `"query"` {
					continue
				}
				for i, stmt := range clause.Body {
					if text(stmt) != "rows, err := db.Query(ctx, args[2])" {
						continue
					}
					if i+2 >= len(clause.Body) {
						return fmt.Errorf("query rows cleanup defer missing")
					}
					deferStmt, ok := clause.Body[i+2].(*ast.DeferStmt)
					if !ok {
						return fmt.Errorf("query rows cleanup must follow the query error guard")
					}
					closure, ok := deferStmt.Call.Fun.(*ast.FuncLit)
					if !ok || len(deferStmt.Call.Args) != 0 || len(closure.Body.List) != 1 {
						return fmt.Errorf("query Rows.Close must defer a named-result assignment")
					}
					assignment, ok := closure.Body.List[0].(*ast.AssignStmt)
					if !ok || text(assignment) != "resultErr = errors.Join(resultErr, rows.Close())" {
						return fmt.Errorf("query defer must join the existing result and Rows.Close error")
					}
					// Check lexical identity as well as spelling: the destination is
					// the actual runContext result, not a closure-local shadow.
					if assignment.Lhs[0].(*ast.Ident).Obj != result.Names[0].Obj {
						return fmt.Errorf("query close assignment does not update runContext's named result")
					}
					join := assignment.Rhs[0].(*ast.CallExpr)
					closeCall := join.Args[1].(*ast.CallExpr)
					closedRows := closeCall.Fun.(*ast.SelectorExpr).X.(*ast.Ident)
					queryRows := stmt.(*ast.AssignStmt).Lhs[0].(*ast.Ident)
					if closedRows.Obj == nil || closedRows.Obj != queryRows.Obj {
						return fmt.Errorf("query defer does not close the acquired rows")
					}
					return nil
				}
			}
		}
	}
	return fmt.Errorf("runContext query row acquisition not found")
}

// Only the in-memory output writer fails. The existing ordinary database
// fixture remains intact, and its full inventory/hash is checked on return.
func TestQueryOutputWriterError(t *testing.T) {
	inspectFixtureUnchanged(t)
	for _, tc := range []struct {
		name   string
		failAt int
		prefix string
	}{
		{"columns", 1, ""},
		{"row", 2, "{\"columns\":[\"id\",\"label\"]}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := errors.New("in-memory query output error")
			out := &queryOutputErrorWriter{failAt: tc.failAt, err: want}
			err := run([]string{"query", inspectFixturePath("page4096.db"), "SELECT id,label FROM samples WHERE id=29"}, out)
			if !errors.Is(err, want) {
				t.Fatalf("query error = %v, want original output error", err)
			}
			if out.calls != tc.failAt || out.String() != tc.prefix {
				t.Fatalf("output: calls=%d, bytes=%q; want calls=%d, bytes=%q", out.calls, out.String(), tc.failAt, tc.prefix)
			}
		})
	}
}

type queryOutputErrorWriter struct {
	bytes.Buffer
	failAt, calls int
	err           error
}

func (w *queryOutputErrorWriter) Write(p []byte) (int, error) {
	w.calls++
	if w.calls == w.failAt {
		return 0, w.err
	}
	return w.Buffer.Write(p)
}
