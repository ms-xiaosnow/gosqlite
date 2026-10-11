package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io"
	"os"
	"testing"
)

func TestSparseExactRun(t *testing.T) {
	ordinaryErr := errors.New("example failed")
	for _, tc := range []struct {
		name       string
		args       []string
		output     string
		err        error
		wantStatus int
		wantCalls  int
		wantStderr string
	}{
		{name: "missing path", wantStatus: 2, wantStderr: "usage: sparse-exact NEW_DATABASE_PATH\n"},
		{name: "extra argument", args: []string{"new.db", "extra"}, wantStatus: 2, wantStderr: "usage: sparse-exact NEW_DATABASE_PATH\n"},
		{name: "success", args: []string{"new.db"}, output: "example output\n", wantCalls: 1},
		{name: "ordinary returned error", args: []string{"new.db"}, output: "partial output\n", err: ordinaryErr, wantStatus: 1, wantCalls: 1, wantStderr: "example failed\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			type contextKey struct{}
			ctx := context.WithValue(context.Background(), contextKey{}, tc.name)
			calls := 0
			fakeExample := func(gotCtx context.Context, path string, out io.Writer) error {
				calls++
				if gotCtx != ctx || path != "new.db" || out != &stdout {
					t.Fatalf("callback arguments: context matches=%t, path=%q, writer matches=%t", gotCtx == ctx, path, out == &stdout)
				}
				fmt.Fprint(out, tc.output)
				return tc.err
			}
			if got := run(ctx, tc.args, &stdout, &stderr, fakeExample); got != tc.wantStatus {
				t.Fatalf("status = %d, want %d", got, tc.wantStatus)
			}
			if calls != tc.wantCalls {
				t.Fatalf("callback calls = %d, want %d", calls, tc.wantCalls)
			}
			if stdout.String() != tc.output || stderr.String() != tc.wantStderr {
				t.Fatalf("stdout=%q, stderr=%q; want stdout=%q, stderr=%q", stdout.String(), stderr.String(), tc.output, tc.wantStderr)
			}
		})
	}
}

func TestSparseExactMainStatusWiring(t *testing.T) {
	source, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	if err := mainStatusWiringError(source); err != nil {
		t.Fatal(err)
	}
}

func TestSparseExactMainStatusWiringRejectsLegacy(t *testing.T) {
	// Static negative control only: no child process or database is opened.
	// Exact main body from baseline 1352ae524575e4d6fe8b5434fd930c3b07be4ac8.
	// Baseline main.go SHA-256: 1306a749e5820ea8c2604d6616d6e361e41f69d1c5b1d509c221bd33439ef051.
	const legacyMain = `func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: sparse-exact NEW_DATABASE_PATH")
		return
	}
	if err := example(context.Background(), os.Args[1], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
`
	if err := mainStatusWiringError([]byte("package main\n" + legacyMain)); err == nil {
		t.Fatal("legacy main unexpectedly propagates its status")
	}
}

func mainStatusWiringError(source []byte) error {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", source, 0)
	if err != nil {
		return err
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Name.Name != "main" {
			continue
		}
		var body bytes.Buffer
		if err := format.Node(&body, fset, fn.Body); err != nil {
			return err
		}
		const want = "{\n\tos.Exit(run(context.Background(), os.Args[1:], os.Stdout, os.Stderr, example))\n}"
		if body.String() != want {
			return fmt.Errorf("main must pass the helper status to os.Exit; body = %s", body.String())
		}
		return nil
	}
	return errors.New("main function not found")
}
