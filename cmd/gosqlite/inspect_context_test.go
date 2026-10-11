package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Pre-canceled runtime calls return at runContext's entry guard. Inspect the
// actual opener as well, so dropping the caller context cannot pass unnoticed.
// This uses no scheduling hooks, native failure injection or database mutation.
func TestInspectSourceForwardsCallerContext(t *testing.T) {
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, "main.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var inspect *ast.CaseClause
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "runContext" {
			continue
		}
		for _, stmt := range fn.Body.List {
			sw, ok := stmt.(*ast.SwitchStmt)
			if !ok {
				continue
			}
			for _, stmt := range sw.Body.List {
				clause := stmt.(*ast.CaseClause)
				for _, value := range clause.List {
					if lit, ok := value.(*ast.BasicLit); ok && lit.Kind == token.STRING && lit.Value == `"inspect"` {
						if inspect != nil {
							t.Fatal("duplicate inspect branch")
						}
						inspect = clause
					}
				}
			}
		}
	}
	if inspect == nil {
		t.Fatal("runContext inspect branch missing")
	}
	openers := 0
	ast.Inspect(inspect, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || (sel.Sel.Name != "Open" && sel.Sel.Name != "OpenContext") {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || pkg.Name != "sqlite" {
			return true
		}
		openers++
		var got bytes.Buffer
		if err := format.Node(&got, set, call); err != nil {
			t.Fatal(err)
		}
		const want = "sqlite.OpenContext(ctx, args[1], sqlite.Options{ReadOnly: true})"
		if got.String() != want {
			t.Errorf("inspect must forward runContext's caller context and preserve read-only options: got %s; want %s", got.String(), want)
		}
		return true
	})
	if openers != 1 {
		t.Fatalf("inspect has %d SQLite open calls, want exactly one", openers)
	}
}

func TestInspectContextReadonlyFixtures(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("this ordinary fixture run qualifies Linux only")
	}
	inspectFixtureUnchanged(t)
	for _, tc := range []struct {
		file                               string
		pageSize, pageCount, cookie        uint32
		numbersRoot, quotesRoot, indexRoot uint32
	}{
		{"page512.db", 512, 1262, 4, 830, 831, 832},
		{"page4096.db", 4096, 161, 4, 105, 106, 107},
		{"page65536.db", 65536, 15, 4, 10, 11, 12},
		{"schema512.db", 512, 110, 81, 0, 0, 0},
	} {
		t.Run(tc.file, func(t *testing.T) {
			wantSchema := []sqlite.SchemaEntry{
				{Type: "table", Name: "samples", TableName: "samples", SQL: "CREATE TABLE samples(id INTEGER PRIMARY KEY,label TEXT,payload BLOB,score REAL,optional TEXT)", RootPage: 2},
				{Type: "table", Name: "numbers", TableName: "numbers", SQL: "CREATE TABLE numbers(value INTEGER)", RootPage: tc.numbersRoot},
				{Type: "table", Name: "quotes", TableName: "quotes", SQL: `CREATE TABLE quotes("a""b" TEXT, "中文" TEXT)`, RootPage: tc.quotesRoot},
				{Type: "index", Name: "sample_label", TableName: "samples", SQL: "CREATE INDEX sample_label ON samples(label)", RootPage: tc.indexRoot},
			}
			if tc.file == "schema512.db" {
				// This committed oracle result covers an interior schema root and
				// the complete 600-column CREATE TABLE overflow record.
				raw, err := os.ReadFile(inspectFixturePath("schema.json"))
				if err != nil {
					t.Fatal(err)
				}
				var oracle []struct {
					Type, Name, SQL string
					TableName       string `json:"tbl_name"`
					RootPage        uint32 `json:"rootpage"`
				}
				if err := json.Unmarshal(raw, &oracle); err != nil {
					t.Fatal(err)
				}
				if len(oracle) != 81 {
					t.Fatalf("oracle schema rows = %d, want 81", len(oracle))
				}
				wantSchema = nil
				for _, row := range oracle {
					wantSchema = append(wantSchema, sqlite.SchemaEntry{Type: row.Type, Name: row.Name, TableName: row.TableName, SQL: row.SQL, RootPage: row.RootPage})
				}
			}
			want, err := json.Marshal(struct {
				Format string               `json:"format"`
				Header sqlite.FileInfo      `json:"header"`
				Schema []sqlite.SchemaEntry `json:"schema"`
			}{
				"SQLite format 3 (metadata inspection only)",
				sqlite.FileInfo{PageSize: tc.pageSize, UsableSize: tc.pageSize, PageCount: tc.pageCount, SchemaCookie: tc.cookie, SchemaFormat: 4, TextEncoding: 1, ReadVersion: 1, WriteVersion: 1},
				wantSchema,
			})
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var out bytes.Buffer
			if err := runContext(ctx, []string{"inspect", inspectFixturePath(tc.file)}, &out); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(out.Bytes(), append(want, '\n')) {
				t.Fatalf("complete inspect output differs: got %s; want %s", out.Bytes(), want)
			}
		})
	}
}

func TestInspectContextCanceled(t *testing.T) {
	inspectFixtureUnchanged(t)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	// An already-expired fixed deadline is deterministic, with no timer race.
	expired, stop := context.WithDeadline(context.Background(), time.Unix(0, 0))
	defer stop()
	for _, tc := range []struct {
		name string
		ctx  context.Context
		want error
	}{
		{"canceled", canceled, context.Canceled},
		{"expired", expired, context.DeadlineExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, file := range []string{"page512.db", "page4096.db", "page65536.db", "schema512.db"} {
				t.Run(file, func(t *testing.T) {
					var out bytes.Buffer
					err := runContext(tc.ctx, []string{"inspect", inspectFixturePath(file)}, &out)
					if !errors.Is(err, tc.want) {
						t.Fatalf("inspect error = %v, want %v", err, tc.want)
					}
					if out.Len() != 0 {
						t.Fatalf("canceled inspect emitted %q", out.String())
					}
				})
			}
		})
	}
}

func inspectFixturePath(name string) string {
	return filepath.Join("..", "..", "testdata", "readonly", name)
}

// Read every committed fixture byte before/after, including the manifest and
// oracle expectations. Comparing the entire inventory also detects sidecars.
func inspectFixtureUnchanged(t *testing.T) {
	t.Helper()
	snapshot := func() map[string]string {
		t.Helper()
		entries, err := os.ReadDir(inspectFixturePath(""))
		if err != nil {
			t.Fatal(err)
		}
		result := make(map[string]string, len(entries))
		for _, entry := range entries {
			raw, err := os.ReadFile(inspectFixturePath(entry.Name()))
			if err != nil {
				t.Fatal(err)
			}
			result[entry.Name()] = fmt.Sprintf("%x", sha256.Sum256(raw))
		}
		return result
	}
	before := snapshot()
	t.Cleanup(func() {
		if after := snapshot(); !reflect.DeepEqual(after, before) {
			t.Errorf("read-only fixture inventory/hash changed: before %v; after %v", before, after)
		}
	})
	raw, err := os.ReadFile(inspectFixturePath("manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Files map[string]struct{ SHA256 string } `json:"files"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"page512.db", "page4096.db", "page65536.db", "schema512.db", "schema.json", "samples.json"} {
		if before[name] != manifest.Files[name].SHA256 || before[name] == "" {
			t.Fatalf("fixture %s does not match committed oracle digest", name)
		}
	}
}
