package main

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// The old program is parsed as text only. No main function or process-exit
// database workflow is executed by this regression.
const oldWriteProgram = `// Command write demonstrates ordinary native Go SQLite CRUD and transactions.
package main

import (
	"context"
	"database/sql"
	"fmt"
	_ "github.com/ms-xiaosnow/gosqlite/driver"
	"log"
	"os"
)

func main() {
	if len(os.Args) != 2 {
		log.Fatal("usage: go run ./examples/write <database-path>")
	}
	db, err := sql.Open("gosqlite", os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	ctx := context.Background()
	if _, err = db.ExecContext(ctx, "CREATE TABLE IF NOT EXISTS docs(id INTEGER PRIMARY KEY,title TEXT NOT NULL)"); err != nil {
		log.Fatal(err)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, "INSERT INTO docs(title) VALUES(?)", "数据库检索")
	if err != nil {
		log.Fatal(err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		log.Fatal(err)
	}
	var title string
	if err = tx.QueryRowContext(ctx, "SELECT title FROM docs WHERE id=?", id).Scan(&title); err != nil {
		log.Fatal(err)
	}
	if err = tx.Commit(); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("inserted %d: %s\n", id, title)
}
`

func writeAST(node ast.Node) string {
	var out bytes.Buffer
	if err := format.Node(&out, token.NewFileSet(), node); err != nil {
		return "<invalid AST>"
	}
	return out.String()
}

func writeStatements(source string) ([]ast.Stmt, error) {
	file, err := parser.ParseFile(token.NewFileSet(), "contract.go", "package main; func contract(){"+source+"}", 0)
	if err != nil {
		return nil, err
	}
	return file.Decls[0].(*ast.FuncDecl).Body.List, nil
}

// inspectWriteLifecycle checks the source-level resource/status contract. It
// does not simulate native failures or substitute a storage driver.
func inspectWriteLifecycle(source []byte) error {
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", source, 0)
	if err != nil {
		return err
	}
	funcs := map[string]*ast.FuncDecl{}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			funcs[fn.Name.Name] = fn
		}
	}
	for _, name := range []string{"example", "run", "main"} {
		if funcs[name] == nil {
			return fmt.Errorf("missing %s helper", name)
		}
	}
	exampleFn := funcs["example"]
	namedError := false
	if exampleFn.Type.Results != nil {
		for _, field := range exampleFn.Type.Results.List {
			if writeAST(field.Type) == "error" && len(field.Names) == 1 && field.Names[0].Name == "err" {
				namedError = true
			}
		}
	}
	if !namedError {
		return errors.New("example must retain its named error")
	}
	var cleanup []ast.Stmt
	for _, stmt := range exampleFn.Body.List {
		if _, ok := stmt.(*ast.DeferStmt); ok {
			cleanup = append(cleanup, stmt)
		}
	}
	expectedCleanup, err := writeStatements(`
        defer func() { err = errors.Join(err, db.Close()) }()
        defer func() {
            if rollbackErr := tx.Rollback(); rollbackErr != nil && !errors.Is(rollbackErr, sql.ErrTxDone) {
                err = errors.Join(err, rollbackErr)
            }
        }()
    `)
	if err != nil {
		return err
	}
	if len(cleanup) != len(expectedCleanup) {
		return errors.New("explicit database/transaction cleanup required")
	}
	for i := range cleanup {
		if writeAST(cleanup[i]) != writeAST(expectedCleanup[i]) {
			return errors.New("cleanup must join errors and filter only the completed transaction")
		}
	}
	var forbidden string
	ast.Inspect(exampleFn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		name := writeAST(call.Fun)
		switch name {
		case "os.Exit", "log.Fatal", "log.Fatalf", "log.Fatalln", "panic", "fmt.Print", "fmt.Printf", "fmt.Println", "fmt.Fprint", "fmt.Fprintf", "fmt.Fprintln":
			forbidden = name
		}
		return true
	})
	if forbidden != "" {
		return fmt.Errorf("resource helper contains %s", forbidden)
	}
	expectedRun, err := writeStatements(`
        if len(args) != 1 { return 1, errors.New("usage: go run ./examples/write <database-path>") }
        message, err := runExample(context.Background(), args[0])
        if err != nil { return 1, err }
        if _, err := fmt.Fprint(out, message); err != nil { return 1, err }
        return 0, nil
    `)
	if err != nil {
		return err
	}
	if len(funcs["run"].Body.List) != len(expectedRun) {
		return errors.New("run must validate, retire, check, then output")
	}
	for i, stmt := range funcs["run"].Body.List {
		if writeAST(stmt) != writeAST(expectedRun[i]) {
			return errors.New("run must preserve helper/output errors before reporting success")
		}
	}
	expectedMain, err := writeStatements(`
        status, err := run(os.Args[1:], os.Stdout, example)
        if err != nil {
            if _, reportErr := fmt.Fprintln(os.Stderr, err); reportErr != nil { status = 1 }
        }
        os.Exit(status)
    `)
	if err != nil {
		return err
	}
	if len(funcs["main"].Body.List) != len(expectedMain) {
		return errors.New("main must only report and exit after run returns")
	}
	for i, stmt := range funcs["main"].Body.List {
		if writeAST(stmt) != writeAST(expectedMain[i]) {
			return errors.New("main must wire arguments, output, diagnostics and exit status")
		}
	}
	return nil
}

func TestWriteExampleLifecycleAST(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if err := inspectWriteLifecycle(source); err != nil {
		t.Fatal(err)
	}
	if err := inspectWriteLifecycle([]byte(oldWriteProgram)); err == nil {
		t.Fatal("old log.Fatal/deferred-cleanup program accepted")
	}
	// Each altered program remains only AST input, never executed.
	mutations := []struct{ name, from, to string }{
		{"unnamed_error", "(message string, err error)", "(string, error)"},
		{"discard_close", "err = errors.Join(err, db.Close())", "db.Close()"},
		{"discard_rollback", "err = errors.Join(err, rollbackErr)", "_ = rollbackErr"},
		{"ignore_done_guard", "!errors.Is(rollbackErr, sql.ErrTxDone)", "true"},
		{"early_output", "return fmt.Sprintf(", "fmt.Printf(\"early success\"); return fmt.Sprintf("},
		{"ignore_callback_error", "return 1, err", "return 0, nil"},
		{"discard_output_error", "if _, err := fmt.Fprint(out, message); err != nil {\n\t\treturn 1, err\n\t}", "fmt.Fprint(out, message)"},
		{"wrong_exit_status", "os.Exit(status)", "os.Exit(0)"},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			changed := strings.Replace(string(source), mutation.from, mutation.to, 1)
			if changed == string(source) {
				t.Fatal("mutation did not apply")
			}
			if err := inspectWriteLifecycle([]byte(changed)); err == nil {
				t.Fatal("unsafe lifecycle accepted")
			}
		})
	}
}

func TestWriteExampleArgumentStatus(t *testing.T) {
	sentinel := errors.New("example callback result")
	for _, args := range [][]string{nil, {"one", "two"}} {
		var out bytes.Buffer
		called := false
		status, err := run(args, &out, func(context.Context, string) (string, error) {
			called = true
			return "unexpected", nil
		})
		if status != 1 || err == nil || err.Error() != "usage: go run ./examples/write <database-path>" || called || out.Len() != 0 {
			t.Fatalf("status=%d error=%v called=%v output=%q", status, err, called, out.String())
		}
	}
	for _, callbackErr := range []error{nil, sentinel} {
		var out bytes.Buffer
		calls := 0
		status, err := run([]string{"example.db"}, &out, func(ctx context.Context, path string) (string, error) {
			calls++
			if ctx == nil || ctx.Err() != nil || path != "example.db" {
				t.Fatal("incorrect example arguments")
			}
			return "ready\n", callbackErr
		})
		if calls != 1 {
			t.Fatalf("callback calls=%d", calls)
		}
		if callbackErr != nil {
			if status != 1 || !errors.Is(err, sentinel) || out.Len() != 0 {
				t.Fatalf("status=%d error=%v output=%q", status, err, out.String())
			}
		} else if status != 0 || err != nil || out.String() != "ready\n" {
			t.Fatalf("status=%d error=%v output=%q", status, err, out.String())
		}
	}
}

func TestWriteExampleOrdinarySuccessReopen(t *testing.T) {
	path := filepath.Join(t.TempDir(), "write.db")
	for id := 1; id <= 2; id++ {
		var out bytes.Buffer
		status, err := run([]string{path}, &out, example)
		if status != 0 || err != nil {
			t.Fatalf("status=%d error=%v", status, err)
		}
		if want := fmt.Sprintf("inserted %d: 数据库检索\n", id); out.String() != want {
			t.Fatalf("output=%q want=%q", out.String(), want)
		}
	}
	db, err := sql.Open("gosqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	ctx := context.Background()
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := tx.Rollback(); err != nil && !errors.Is(err, sql.ErrTxDone) {
			t.Error(err)
		}
	})
	// Ordinary successful CRUD inside a transaction, then an explicit normal
	// rollback. The example's two committed rows must remain unchanged.
	for _, query := range []string{
		"UPDATE docs SET title='temporary' WHERE id=1",
		"INSERT INTO docs(id,title) VALUES(3,'temporary')",
		"DELETE FROM docs WHERE id=2",
	} {
		result, err := tx.ExecContext(ctx, query)
		if err != nil {
			t.Fatal(err)
		}
		affected, err := result.RowsAffected()
		if err != nil || affected != 1 {
			t.Fatalf("affected=%d error=%v", affected, err)
		}
	}
	var title string
	if err := tx.QueryRowContext(ctx, "SELECT title FROM docs WHERE id=1").Scan(&title); err != nil || title != "temporary" {
		t.Fatalf("title=%q error=%v", title, err)
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	rows, err := db.QueryContext(ctx, "SELECT id,title FROM docs ORDER BY id")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := rows.Close(); err != nil {
			t.Error(err)
		}
	})
	count := 0
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id, &title); err != nil {
			t.Fatal(err)
		}
		count++
		if id != int64(count) || title != "数据库检索" {
			t.Fatalf("id=%d title=%q", id, title)
		}
	}
	if err := errors.Join(rows.Err(), rows.Close()); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("rows=%d", count)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	native, err := sqlite.Open(path, sqlite.Options{ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := native.Close(); err != nil {
			t.Error(err)
		}
	})
	checked, err := native.Check(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if checked.Scope != sqlite.CheckScope || checked.Tables != 1 || checked.TableRows != 2 || checked.Indexes != 0 || checked.PageReads == 0 {
		t.Fatalf("check=%+v", checked)
	}
	if err := native.Close(); err != nil {
		t.Fatal(err)
	}
}
