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
	"strings"
	"testing"
	"time"

	"github.com/ms-xiaosnow/gosqlite/sqlite"
)

// Pre-canceled runtime calls return before opening. These AST checks also pin
// the actual dispatch, public opener, options and single-snapshot ownership path.
// Negative controls mutate parsed source strings only, never native state.
func inspectSourceErrors(mainSource, inspectSource []byte) []string {
	set := token.NewFileSet()
	mainFile, err := parser.ParseFile(set, "main.go", mainSource, 0)
	if err != nil {
		return []string{err.Error()}
	}
	helperFile, err := parser.ParseFile(set, "inspect.go", inspectSource, 0)
	if err != nil {
		return []string{err.Error()}
	}
	var failures []string
	text := func(node ast.Node) string {
		var out bytes.Buffer
		if err := format.Node(&out, set, node); err != nil {
			failures = append(failures, err.Error())
		}
		return out.String()
	}
	functions := map[string]*ast.FuncDecl{}
	for _, file := range []*ast.File{mainFile, helperFile} {
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok {
				if functions[fn.Name.Name] != nil {
					failures = append(failures, "duplicate function "+fn.Name.Name)
				}
				functions[fn.Name.Name] = fn
			}
		}
	}
	var inspect *ast.CaseClause
	if run := functions["runContext"]; run != nil {
		for _, stmt := range run.Body.List {
			sw, ok := stmt.(*ast.SwitchStmt)
			if !ok || text(sw.Tag) != "args[0]" {
				continue
			}
			for _, stmt := range sw.Body.List {
				clause := stmt.(*ast.CaseClause)
				for _, value := range clause.List {
					if text(value) == `"inspect"` {
						if inspect != nil {
							failures = append(failures, "duplicate inspect branch")
						}
						inspect = clause
					}
				}
			}
		}
	}
	if inspect == nil || len(inspect.Body) != 1 || text(inspect.Body[0]) != "return runInspect(ctx, args[1:], out)" {
		failures = append(failures, "inspect must forward caller context, arguments and output once")
	}
	// Compare parsed statement structure, not textual presence. Full bodies pin
	// parse-before-open, provider scope, options, checked close and emission order.
	expectBody := func(name, body string) {
		fn := functions[name]
		expected, err := parser.ParseFile(set, "expected.go", "package main; func expected() {"+body+"}", 0)
		if err != nil {
			failures = append(failures, "invalid source contract: "+err.Error())
			return
		}
		if fn == nil || strings.Join(strings.Fields(text(fn.Body)), " ") != strings.Join(strings.Fields(text(expected.Decls[0].(*ast.FuncDecl).Body)), " ") {
			failures = append(failures, name+" forwarding/ownership body changed")
		}
	}
	expectBody("openInspectDB", `return sqlite.OpenContext(ctx, path, opts)`)
	expectBody("runInspectWithInput", `return runInspectWithOpener(ctx, args, out, input, openInspectDB)`)
	expectBody("runInspectWithOpener", `
		if err := ctx.Err(); err != nil { return err }
		o, err := parseInspect(args)
		if err != nil { return err }
		var provider *cliKeyProvider
		if o.provider != "" {
			provider, err = newCLIKeyProvider(o.provider, input)
			if err != nil { return err }
			defer provider.close()
		}
		result, err := executeInspect(ctx, o, provider, open)
		if err == nil { err = json.NewEncoder(out).Encode(result) }
		if err != nil && provider != nil { return inspectError(err) }
		return err
	`)
	expectBody("executeInspect", `
		opts := sqlite.Options{ReadOnly: true, NoCreate: true, WALShared: o.walReadOnly}
		if provider != nil {
			opts.Encryption = &sqlite.EncryptionOptions{KeyProvider: provider, CredentialKind: provider.kind, Mode: o.mode}
			opts.KeepWALOnClose = o.mode != sqlite.EncryptionSnapshot
		}
		db, err := open(ctx, o.path, opts)
		if err != nil { return nil, err }
		inspection, inspectErr := db.Inspect(ctx, sqlite.InspectOptions{IncludeExtensions: o.extensions})
		if err := errors.Join(inspectErr, db.Close()); err != nil { return nil, err }
		return inspectOutput(o, provider != nil, inspection), nil
	`)
	parse := functions["parseInspect"]
	if parse == nil || len(parse.Body.List) != 7 || text(parse.Body.List[0]) != "flags, tail, err := parseReadOnlyWALFlags(args, true)" {
		failures = append(failures, "inspect must parse legacy leading flags before routing")
	} else {
		guard, ok := parse.Body.List[1].(*ast.IfStmt)
		if !ok || text(guard.Cond) != `err != nil || len(tail) == 0 || tail[0] == "" || flags.walExclusive || flags.walShared` || len(guard.Body.List) != 1 || text(guard.Body.List[0]) != "return inspectCommandOptions{}, errors.New(inspectUsage)" {
			failures = append(failures, "inspect must reject invalid paths and writable WAL modes")
		}
		if text(parse.Body.List[2]) != "o := inspectCommandOptions{path: tail[0], walReadOnly: flags.walReadOnly, extensions: flags.extensions}" {
			failures = append(failures, "inspect parsed path/flags were not preserved")
		}
		guard, ok = parse.Body.List[5].(*ast.IfStmt)
		if !ok || text(guard.Cond) != `seen["--mode"] && o.provider == "" || o.provider != "" && o.walReadOnly` || len(guard.Body.List) != 1 || text(guard.Body.List[0]) != "return inspectCommandOptions{}, errors.New(inspectUsage)" {
			failures = append(failures, "inspect must reject mode without provider and plaintext WAL/encryption conflicts")
		}
	}
	// Check the user-facing wrapper structurally without binding whitespace or
	// the noninteractive stdin closure's formatting to the source test.
	run := functions["runInspect"]
	if run == nil || len(run.Body.List) != 1 {
		failures = append(failures, "runInspect forwarding missing")
	} else if ret, ok := run.Body.List[0].(*ast.ReturnStmt); !ok || len(ret.Results) != 1 {
		failures = append(failures, "runInspect must return its input runner")
	} else if call, ok := ret.Results[0].(*ast.CallExpr); !ok || text(call.Fun) != "runInspectWithInput" || len(call.Args) != 4 || text(call.Args[0]) != "ctx" || text(call.Args[1]) != "args" || text(call.Args[2]) != "out" {
		failures = append(failures, "runInspect lost caller context/arguments/output")
	}
	openers, inspections, closes := 0, 0, 0
	ast.Inspect(helperFile, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok { return true }
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok { return true }
		if text(sel.X) == "sqlite" && (sel.Sel.Name == "Open" || sel.Sel.Name == "OpenContext") { openers++ }
		if sel.Sel.Name == "Inspect" { inspections++ }
		if sel.Sel.Name == "Close" { closes++ }
		if sel.Sel.Name == "VerifyEncrypted" { failures = append(failures, "inspect must not add verification/debug reads") }
		return true
	})
	if openers != 1 || inspections != 1 || closes != 1 {
		failures = append(failures, fmt.Sprintf("inspect has %d openers, %d inspections, %d closes; want one each", openers, inspections, closes))
	}
	// Parser object identity catches a same-spelled context shadow as well.
	for _, name := range []string{"openInspectDB", "runInspect", "runInspectWithInput", "runInspectWithOpener", "executeInspect"} {
		fn := functions[name]
		if fn == nil || len(fn.Type.Params.List) == 0 || len(fn.Type.Params.List[0].Names) != 1 { continue }
		ctx := fn.Type.Params.List[0].Names[0]
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			id, ok := node.(*ast.Ident)
			if ok && id.Name == "ctx" && (id.Obj == nil || id.Obj != ctx.Obj) {
				failures = append(failures, name+" lost caller context identity")
			}
			return true
		})
	}
	return failures
}

func TestInspectSourceForwardsCallerContext(t *testing.T) {
	mainSource, err := os.ReadFile("main.go")
	if err != nil { t.Fatal(err) }
	helperSource, err := os.ReadFile("inspect.go")
	if err != nil { t.Fatal(err) }
	if failures := inspectSourceErrors(mainSource, helperSource); len(failures) != 0 { t.Fatal(failures) }
	for _, tc := range []struct{ name, file, old, replacement string }{
		{"dispatch_context", "main", "return runInspect(ctx, args[1:], out)", "return runInspect(context.Background(), args[1:], out)"},
		{"dispatch_path", "main", "return runInspect(ctx, args[1:], out)", "return runInspect(ctx, args[2:], out)"},
		{"duplicate_inspect", "main", "case \"inspect\":", "case \"inspect\":\ncase \"inspect\":"},
		{"caller_context", "helper", "sqlite.OpenContext(ctx, path, opts)", "sqlite.OpenContext(context.Background(), path, opts)"},
		{"legacy_open", "helper", "sqlite.OpenContext(ctx, path, opts)", "sqlite.Open(path, opts)"},
		{"wrong_path", "helper", "open(ctx, o.path, opts)", "open(ctx, o.provider, opts)"},
		{"writable_option", "helper", "ReadOnly: true", "ReadOnly: false"},
		{"create_option", "helper", "NoCreate: true", "NoCreate: false"},
		{"missing_readonly", "helper", "ReadOnly: true, ", ""},
		{"missing_nocreate", "helper", "NoCreate: true, ", ""},
		{"wrong_wal_flag", "helper", "WALShared: o.walReadOnly", "WALShared: true"},
		{"duplicate_open", "helper", "db, err := open(ctx, o.path, opts)", "db, err := open(ctx, o.path, opts); _, _ = open(ctx, o.path, opts)"},
		{"parser_bypass", "helper", "parseReadOnlyWALFlags(args, true)", "parseReadOnlyWALFlags(args, false)"},
		{"writable_modes_accepted", "helper", ` || flags.walExclusive || flags.walShared`, ""},
		{"mixed_modes_accepted", "helper", ` || o.provider != "" && o.walReadOnly`, ""},
		{"profile_lost", "helper", "Mode: o.mode", "Mode: sqlite.EncryptionSnapshot"},
		{"provider_lost", "helper", "KeyProvider: provider", "KeyProvider: nil"},
		{"wal_retention_lost", "helper", "opts.KeepWALOnClose = o.mode != sqlite.EncryptionSnapshot", "opts.KeepWALOnClose = false"},
		{"inspection_context", "helper", "db.Inspect(ctx,", "db.Inspect(context.Background(),"},
		{"extensions_lost", "helper", "IncludeExtensions: o.extensions", "IncludeExtensions: true"},
		{"discard_close_error", "helper", "errors.Join(inspectErr, db.Close())", "inspectErr"},
		{"discard_inspect_error", "helper", "errors.Join(inspectErr, db.Close())", "db.Close()"},
		{"early_provider_close", "helper", "defer provider.close()", "provider.close()"},
		{"output_on_error", "helper", "if err == nil {", "if true {"},
		{"input_opener_bypass", "helper", "input, openInspectDB)", "input, nil)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mainCopy, helperCopy := string(mainSource), string(helperSource)
			source := &helperCopy
			if tc.file == "main" { source = &mainCopy }
			if strings.Count(*source, tc.old) != 1 { t.Fatal("negative control must replace exactly one source fragment", tc.old) }
			*source = strings.Replace(*source, tc.old, tc.replacement, 1)
			if _, err := parser.ParseFile(token.NewFileSet(), tc.file+".go", *source, 0); err != nil { t.Fatal("malformed negative control", err) }
			if failures := inspectSourceErrors([]byte(mainCopy), []byte(helperCopy)); len(failures) == 0 { t.Fatal("source guard accepted negative control", tc.name) }
		})
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
