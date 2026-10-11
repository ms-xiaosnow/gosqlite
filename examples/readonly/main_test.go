package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// These tests copy only complete committed fixtures. They do not change database
// bytes, inject storage errors, start database subprocesses, or invoke main.
func TestReadonlyExampleIntactFixtures(t *testing.T) {
	for _, fixture := range []string{"page512.db", "page4096.db", "page65536.db"} {
		for _, mode := range []string{"path", "ro", "rw", "rwc"} {
			t.Run(fixture+"/"+mode, func(t *testing.T) {
				raw, err := os.ReadFile(filepath.Join("..", "..", "testdata", "readonly", fixture))
				if err != nil {
					t.Fatal(err)
				}
				dir := t.TempDir()
				path := filepath.Join(dir, "existing.db")
				if err := os.WriteFile(path, raw, 0600); err != nil {
					t.Fatal(err)
				}
				before := readonlyInventory(t, dir)
				input := path
				if mode != "path" {
					input = (&url.URL{Scheme: "file", Path: path, RawQuery: "mode=" + mode}).String()
				}
				var output bytes.Buffer
				if err := run(context.Background(), []string{input}, &output); err != nil {
					t.Fatal(err)
				}
				if got, want := output.String(), "1\trow-0001\n2\trow-0002\n3\trow-0003\n"; got != want {
					t.Fatalf("output = %q, want %q", got, want)
				}
				if after := readonlyInventory(t, dir); !reflect.DeepEqual(before, after) {
					t.Fatalf("fixture directory changed: before=%v after=%v", before, after)
				}
			})
		}
	}
}

type readonlyFileState struct {
	Mode os.FileMode
	Size int64
	Hash [sha256.Size]byte
}

func readonlyInventory(t *testing.T, dir string) map[string]readonlyFileState {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	result := make(map[string]readonlyFileState, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			t.Fatal(err)
		}
		if !info.Mode().IsRegular() {
			t.Fatalf("unexpected non-regular fixture entry %q", entry.Name())
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		result[entry.Name()] = readonlyFileState{Mode: info.Mode(), Size: info.Size(), Hash: sha256.Sum256(raw)}
	}
	return result
}

func TestReadonlyRunArguments(t *testing.T) {
	for _, args := range [][]string{nil, {"unused", "unused"}} {
		var output bytes.Buffer
		err := run(context.Background(), args, &output)
		if err == nil || err.Error() != "usage: readonly <existing.db>" {
			t.Fatalf("args %v: error = %v", args, err)
		}
		if output.Len() != 0 {
			t.Fatalf("args %v wrote %q", args, output.String())
		}
	}
}

// Callback writers are used only by pure formatting/reporting helpers, never
// while a database is open. The database tests use an ordinary bytes.Buffer.
type readonlyWriterFunc func([]byte) (int, error)

func (f readonlyWriterFunc) Write(p []byte) (int, error) { return f(p) }

func TestReadonlyWriteRow(t *testing.T) {
	var output bytes.Buffer
	if err := writeRow(&output, 29, "数据库🙂é한글-29"); err != nil {
		t.Fatal(err)
	}
	if got, want := output.String(), "29\t数据库🙂é한글-29\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
	writeErr := errors.New("pure row-output callback")
	calls := 0
	out := readonlyWriterFunc(func(p []byte) (int, error) {
		calls++
		if string(p) != "1\trow-0001\n" {
			t.Fatalf("callback output = %q", p)
		}
		return 0, writeErr
	})
	if err := writeRow(out, 1, "row-0001"); !errors.Is(err, writeErr) || calls != 1 {
		t.Fatalf("write error = %v, calls = %d", err, calls)
	}
}

func TestReadonlyReportResult(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		out := readonlyWriterFunc(func([]byte) (int, error) {
			t.Fatal("success should not emit a diagnostic")
			return 0, nil
		})
		if status, err := reportResult(nil, out); status != 0 || err != nil {
			t.Fatalf("status=%d error=%v", status, err)
		}
	})
	t.Run("operation error", func(t *testing.T) {
		operationErr := errors.New("pure operation callback")
		var output bytes.Buffer
		status, err := reportResult(operationErr, &output)
		if status != 1 || !errors.Is(err, operationErr) || output.String() != "readonly: pure operation callback\n" {
			t.Fatalf("status=%d error=%v output=%q", status, err, output.String())
		}
	})
	t.Run("diagnostic error", func(t *testing.T) {
		operationErr := errors.New("pure operation callback")
		writeErr := errors.New("pure diagnostic callback")
		calls := 0
		out := readonlyWriterFunc(func(p []byte) (int, error) {
			calls++
			if string(p) != "readonly: pure operation callback\n" {
				t.Fatalf("callback diagnostic = %q", p)
			}
			return 0, writeErr
		})
		status, err := reportResult(operationErr, out)
		if status != 1 || !errors.Is(err, operationErr) || !errors.Is(err, writeErr) || calls != 1 {
			t.Fatalf("status=%d error=%v calls=%d", status, err, calls)
		}
	})
}

func TestReadonlySourceContract(t *testing.T) {
	raw, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if findings := readonlySourceContract(raw); len(findings) != 0 {
		t.Fatal(strings.Join(findings, "\n"))
	}
}

// This is a source-only contract: it never executes the parsed source. In
// particular it checks cleanup and final exit ordering without exiting a test
// process, and checks context propagation without interrupting database I/O.
func readonlySourceContract(raw []byte) []string {
	file, err := parser.ParseFile(token.NewFileSet(), "main.go", raw, 0)
	if err != nil {
		return []string{err.Error()}
	}
	functions := make(map[string]*ast.FuncDecl)
	var findings []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		functions[fn.Name.Name] = fn
		ast.Inspect(fn, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			name := readonlyNodeText(call.Fun)
			if name == "log.Fatal" || name == "log.Fatalf" || name == "log.Fatalln" || name == "sql.Open" || (name == "os.Exit" && fn.Name.Name != "main") {
				findings = append(findings, fn.Name.Name+": forbidden call "+name)
			}
			return true
		})
	}
	// Exact ordered statements deliberately fix the ownership boundary. They
	// also ensure errors from Scan, row output, iteration and both Close calls
	// survive until reporting; no synthetic database failures are exercised.
	contracts := []struct {
		name   string
		source string
	}{
		{"example", `func example(ctx context.Context, input string, out io.Writer) (resultErr error) {
			if err := ctx.Err(); err != nil { return err }
			connector, err := driver.NewConnector(input, sqlite.Options{ReadOnly: true, NoCreate: true})
			if err != nil { return err }
			db := sql.OpenDB(connector)
			defer func() { resultErr = errors.Join(resultErr, db.Close()) }()
			rows, err := db.QueryContext(ctx, "SELECT id, label FROM samples WHERE id >= ? ORDER BY id LIMIT ?", int64(1), 3)
			if err != nil { return err }
			defer func() { resultErr = errors.Join(resultErr, rows.Close(), rows.Err()) }()
			for rows.Next() {
				var id int64
				var label string
				if err := rows.Scan(&id, &label); err != nil { return err }
				if err := writeRow(out, id, label); err != nil { return err }
			}
			return nil
		}`},
		{"run", `func run(ctx context.Context, args []string, out io.Writer) error {
			if len(args) != 1 { return fmt.Errorf("usage: readonly <existing.db>") }
			return example(ctx, args[0], out)
		}`},
		{"writeRow", `func writeRow(out io.Writer, id int64, label string) error {
			_, err := fmt.Fprintf(out, "%d\t%s\n", id, label)
			return err
		}`},
		{"reportResult", `func reportResult(err error, errOut io.Writer) (int, error) {
			if err == nil { return 0, nil }
			_, writeErr := fmt.Fprintln(errOut, "readonly:", err)
			return 1, errors.Join(err, writeErr)
		}`},
		{"main", `func main() {
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
			err := run(ctx, os.Args[1:], os.Stdout)
			stop()
			if status, err := reportResult(err, os.Stderr); err != nil { os.Exit(status) }
		}`},
	}
	for _, contract := range contracts {
		fn := functions[contract.name]
		if fn == nil {
			findings = append(findings, "missing function "+contract.name)
			continue
		}
		want, err := parser.ParseFile(token.NewFileSet(), "contract.go", "package main\n"+contract.source, 0)
		if err != nil {
			findings = append(findings, "invalid source contract: "+err.Error())
			continue
		}
		if readonlyNodeText(fn) != readonlyNodeText(want.Decls[0]) {
			findings = append(findings, contract.name+": read-only/context/lifecycle/output contract changed")
		}
	}
	return findings
}

func readonlyNodeText(node ast.Node) string {
	var output bytes.Buffer
	if err := format.Node(&output, token.NewFileSet(), node); err != nil {
		return fmt.Sprintf("invalid syntax node: %v", err)
	}
	return output.String()
}

var _ io.Writer = readonlyWriterFunc(nil)
