package main

import (
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/build/constraint"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Pure source checks only. The five inputs are read as text, never imported,
// compiled as fixtures, constructed or executed by this test. The exact pins
// make unknown helper/adapter refactors fail closed pending source review.
const connectorManualPath = "sqlite/connector_rollback_recovery_manual_linux_test.go"

var connectorManualPins = map[string]string{
	connectorManualPath:                             "93008d29eb020aa93bbafbc82798e2f758253b776ade151f85749d59247c8206",
	"sqlite/rollback_recovery_manual_linux_test.go": "dd5a63e2a8e7d4fdd969ba03273a8f8147d5eff81babf8fce44a5228c5cdaf5e",
	"driver/driver.go":                              "5f6df17c06316ac5090b935c0c6cd48cb730e00885e43cc87ce26d8b6efde03d",
	"sqlite/rows.go":                                "0e164cf3fa34b2255962f99e263f573ff390895df0831a71f3489f2ba36dd8cd",
	"sqlite/rollback_recovery.go":                   "320c163d8b79bf458f7685455f49758065fac6eed6269c9001041a9e117189ba",
}

func connectorManualSources(t *testing.T) map[string][]byte {
	t.Helper()
	out := make(map[string][]byte, len(connectorManualPins))
	for path := range connectorManualPins {
		raw, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		out[path] = raw
	}
	return out
}

func connectorManualCallName(expr ast.Expr) string {
	switch x := expr.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		prefix := connectorManualCallName(x.X)
		if prefix != "" {
			return prefix + "." + x.Sel.Name
		}
	}
	return ""
}

func connectorManualNode(raw []byte, set *token.FileSet, node ast.Node) string {
	return string(raw[set.Position(node.Pos()).Offset:set.Position(node.End()).Offset])
}

func connectorManualFunctions(file *ast.File) map[string]*ast.FuncDecl {
	out := make(map[string]*ast.FuncDecl)
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			out[fn.Name.Name] = fn
		}
	}
	return out
}

func inspectConnectorManualSources(sources map[string][]byte) ([]string, error) {
	var findings []string
	files := make(map[string]*ast.File)
	sets := make(map[string]*token.FileSet)
	for path, pin := range connectorManualPins {
		raw, ok := sources[path]
		if !ok {
			return nil, fmt.Errorf("missing source %s", path)
		}
		set := token.NewFileSet()
		file, err := parser.ParseFile(set, path, raw, parser.ParseComments)
		if err != nil {
			return nil, err
		}
		files[path], sets[path] = file, set
		if fmt.Sprintf("%x", sha256.Sum256(raw)) != pin {
			findings = append(findings, "fixed source changed: "+path)
		}
	}
	raw, file, set := sources[connectorManualPath], files[connectorManualPath], sets[connectorManualPath]
	first := strings.SplitN(string(raw), "\n", 2)[0]
	if first != "//go:build manual_recovery && linux" {
		findings = append(findings, "manual build gate changed")
	}
	constraintExpr, err := constraint.Parse(first)
	if err != nil {
		return nil, err
	}
	for _, platform := range []string{"linux", "darwin", "windows"} {
		for _, manual := range []bool{false, true} {
			for _, oracle := range []bool{false, true} {
				selected := constraintExpr.Eval(func(tag string) bool {
					return tag == platform || tag == "manual_recovery" && manual || tag == "oracle" && oracle
				})
				if selected != (platform == "linux" && manual) {
					findings = append(findings, "manual selection matrix changed")
				}
			}
		}
	}
	if file.Name.Name != "sqlite_test" {
		findings = append(findings, "external sqlite_test package changed")
	}
	allowedImports := map[string]string{
		"context": "context", "sqldriver": "database/sql/driver", "errors": "errors",
		"io": "io", "os": "os", "filepath": "path/filepath", "testing": "testing",
		"gosqlitedriver": "github.com/ms-xiaosnow/gosqlite/driver",
		"sqlite":         "github.com/ms-xiaosnow/gosqlite/sqlite",
	}
	if len(file.Imports) != len(allowedImports) {
		findings = append(findings, "manual import closure changed")
	}
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return nil, err
		}
		name := filepath.Base(path)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if allowedImports[name] != path {
			findings = append(findings, "manual import closure changed: "+path)
		}
	}
	functions := connectorManualFunctions(file)
	entry := functions["TestManualConnectorRollbackRecovery"]
	if entry == nil || entry.Body == nil || len(entry.Body.List) == 0 {
		return append(findings, "manual entry missing"), nil
	}
	gate, ok := entry.Body.List[0].(*ast.IfStmt)
	gateOK := ok && gate.Init == nil && gate.Else == nil && len(gate.Body.List) == 1
	if gateOK {
		gateOK = connectorManualNode(raw, set, gate.Cond) == `os.Getenv("GOSQLITE_MANUAL_CONNECTOR_RECOVERY") != "independently_authorized"`
		statement, ok := gate.Body.List[0].(*ast.ExprStmt)
		if !ok {
			gateOK = false
		} else {
			call, ok := statement.X.(*ast.CallExpr)
			gateOK = gateOK && ok && connectorManualCallName(call.Fun) == "t.Fatal"
		}
	}
	if !gateOK {
		findings = append(findings, "manual entry gate changed")
	}
	for _, decl := range file.Decls {
		switch x := decl.(type) {
		case *ast.FuncDecl:
			if x.Name.Name == "init" || x.Name.Name == "TestMain" || strings.HasPrefix(x.Name.Name, "Test") && x != entry {
				findings = append(findings, "unexpected manual entry")
			}
		case *ast.GenDecl:
			if x.Tok == token.VAR {
				findings = append(findings, "manual global state forbidden")
			}
		}
	}
	counts := make(map[string]int)
	ast.Inspect(file, func(node ast.Node) bool {
		switch x := node.(type) {
		case *ast.GoStmt, *ast.DeferStmt:
			findings = append(findings, "manual asynchronous/deferred work forbidden")
		case *ast.CallExpr:
			name := connectorManualCallName(x.Fun)
			counts[name]++
		case *ast.SelectorExpr:
			switch x.Sel.Name {
			case "Open", "OpenContext", "OpenDB", "RecoverRollback", "RetryPlaintextCleanup", "RetryCloseCleanup", "Remove", "RemoveAll", "Rename", "Chmod", "Symlink", "Link", "Truncate", "Kill", "Signal", "Exit", "Parallel", "TempDir", "Skip", "Skipf", "SkipNow":
				findings = append(findings, "forbidden manual reference: "+x.Sel.Name)
			}
		}
		return true
	})
	if counts["gosqlitedriver.NewConnector"] != 5 || counts["connector.Connect"] != 4 || counts["readonly.Connect"] != 1 || counts["t.Run"] != 4 {
		findings = append(findings, "actual Connector call path/count changed")
	}
	caseNames := []string{"nil-default-preserves-input", "opt-in-restores-and-reconnects", "journal-limit-preserves-input", "no-create-preserves-absence"}
	mutations := []string{
		"opts.RollbackRecovery = &sqlite.RollbackRecoveryOptions{}",
		"cfg.MaxJournalBytes, cfg.MaxRecords, cfg.MaxDatabaseBytes = 1, 1, 1",
		"cfg.MaxJournalBytes = 0", "opts.NoCreate = false",
	}
	caseIndex := 0
	for _, statement := range entry.Body.List {
		branch, ok := statement.(*ast.IfStmt)
		if !ok {
			continue
		}
		negation, ok := branch.Cond.(*ast.UnaryExpr)
		if !ok || negation.Op != token.NOT {
			continue
		}
		call, ok := negation.X.(*ast.CallExpr)
		if !ok || connectorManualCallName(call.Fun) != "t.Run" {
			continue
		}
		if caseIndex >= len(caseNames) || len(call.Args) != 2 {
			return append(findings, "serial scenario shape changed"), nil
		}
		name, nameOK := call.Args[0].(*ast.BasicLit)
		body, bodyOK := call.Args[1].(*ast.FuncLit)
		if len(branch.Body.List) != 1 {
			return append(findings, "serial scenario shape changed"), nil
		}
		_, returns := branch.Body.List[0].(*ast.ReturnStmt)
		if !nameOK || name.Value != strconv.Quote(caseNames[caseIndex]) || !bodyOK || !returns || branch.Else != nil {
			return append(findings, "serial scenario shape changed"), nil
		}
		text := connectorManualNode(raw, set, body)
		constructed, mutated, connected := strings.Index(text, "gosqlitedriver.NewConnector("), strings.Index(text, mutations[caseIndex]), strings.Index(text, "connector.Connect(")
		if constructed < 0 || mutated <= constructed || connected <= mutated {
			findings = append(findings, "post-construction snapshot mutation changed")
		}
		caseIndex++
	}
	if caseIndex != 4 {
		findings = append(findings, "serial scenario count changed")
	}
	for name, required := range map[string][]string{
		"TestManualConnectorRollbackRecovery": {"filepath.IsAbs(root)", "filepath.Clean(root) != root", "filepath.Dir(root) == root", "os.Lstat(root)", "filepath.EvalSymlinks(root)", "resolved != root", "os.ReadDir(root)", "len(entries) != 0", "manualPublicRecoveryNoPending(t)", "sqlite.PendingPlaintextCleanups()"},
		"manualConnectorRecoveryScopeFor":     {"t.Cleanup(func()", "len(scope.owners) - 1; i >= 0; i--", "if !scope.close(scope.owners[i])", "break", "if !owner.retired"},
		"close":                               {"if owner.retired", "if scope.frozen || owner.attempted", "owner.attempted = true", "owner.err = owner.value.Close()", "scope.retain(owner.err)", "owner.retired = true"},
		"next":                                {"scope.frozen || owner.attempted || owner.retired", "owner.attempted = true", "err := rows.Next(dest)", "owner.attempted = false", "if err == io.EOF", "owner.retired = true", "scope.retain(err)"},
		"retain":                              {"scope.frozen = true", "scope.errors = append(scope.errors, err)"},
		"manualConnectorRecoveryReadSchema":   {`conn.Prepare("SELECT name FROM sqlite_schema")`, "stmt.Query(nil)", "rows.Columns()", "err != io.EOF", "scope.close(rowOwner)", "scope.close(statement)"},
		"manualConnectorRecoveryEmpty":        {`[]string{"", "-journal", "-wal", "-shm"}`, "os.ReadDir(dir)", "len(entries) != 0"},
	} {
		fn := functions[name]
		if fn == nil {
			findings = append(findings, "manual contract function missing: "+name)
			continue
		}
		body := connectorManualNode(raw, set, fn)
		for _, fragment := range required {
			if !strings.Contains(body, fragment) {
				findings = append(findings, "manual ownership/root/read contract changed: "+name)
			}
		}
		if name == "close" && strings.Index(body, "owner.attempted = true") > strings.Index(body, "owner.value.Close()") || name == "next" && strings.Index(body, "owner.attempted = true") > strings.Index(body, "rows.Next(dest)") {
			findings = append(findings, "retirement must be marked before adapter call")
		}
	}
	driverPath := "driver/driver.go"
	driverFunctions := connectorManualFunctions(files[driverPath])
	for name, fragments := range map[string][]string{
		"NewConnector": {"copy := *opts.RollbackRecovery", "opts.RollbackRecovery = &copy", "return &Connector{path: path, opts: opts}, nil"},
		"Connect":      {"sqlite.OpenContext(ctx, c.path, c.opts)", "return &conn{db: db}, nil"},
		"Next":         {"!r.inner.Next()", "r.inner.Err()", "return io.EOF"},
	} {
		fn := driverFunctions[name]
		if fn == nil {
			findings = append(findings, "driver contract missing: "+name)
			continue
		}
		body := connectorManualNode(sources[driverPath], sets[driverPath], fn)
		for _, fragment := range fragments {
			if !strings.Contains(body, fragment) {
				findings = append(findings, "driver snapshot/Connect/EOF contract changed: "+name)
			}
		}
	}
	// Review the constructor's same-file parser closure as well as its own body.
	// Allowed calls operate on strings/Options only; no database owner is made.
	pureCalls := map[string]bool{
		"parseDSNMode": true, "fmt.Errorf": true, "strings.ContainsRune": true,
		"strings.HasPrefix": true, "strings.Contains": true, "strings.ContainsAny": true,
		"url.Parse": true, "url.ParseQuery": true, "url.PathUnescape": true,
		"len": true, "fail": true,
	}
	for _, name := range []string{"NewConnector", "parseDSNMode"} {
		fn := driverFunctions[name]
		if fn == nil {
			findings = append(findings, "constructor no-I/O closure missing")
			continue
		}
		ast.Inspect(fn, func(node ast.Node) bool {
			if call, ok := node.(*ast.CallExpr); ok && !pureCalls[connectorManualCallName(call.Fun)] {
				findings = append(findings, "constructor no-I/O call closure changed")
			}
			return true
		})
	}
	return findings, nil
}

func TestConnectorRollbackManualSourceContract(t *testing.T) {
	findings, err := inspectConnectorManualSources(connectorManualSources(t))
	if err != nil || len(findings) != 0 {
		t.Fatal(findings, err)
	}
}

func TestConnectorRollbackManualSourceMutationCounterexamples(t *testing.T) {
	sources := connectorManualSources(t)
	// All counterexamples are inert text. Parsing must succeed before rejection
	// counts; the actual manual test binary is never launched by this test.
	variants := []struct{ name, path, old, replacement, finding string }{
		{"build or", connectorManualPath, "manual_recovery && linux", "manual_recovery || linux", "manual build gate"},
		{"missing linux", connectorManualPath, "manual_recovery && linux", "manual_recovery", "manual selection matrix"},
		{"weaker env", connectorManualPath, `os.Getenv("GOSQLITE_MANUAL_CONNECTOR_RECOVERY") != "independently_authorized"`, "false", "manual entry gate"},
		{"skip gate", connectorManualPath, `t.Fatal("NOT_RUN:`, `t.Skip("NOT_RUN:`, "manual entry gate"},
		{"different root", connectorManualPath, "resolved != root", "false", "manual ownership/root/read contract"},
		{"dirty root", connectorManualPath, "len(entries) != 0", "false", "manual ownership/root/read contract"},
		{"early native call", connectorManualPath, `if os.Getenv("GOSQLITE_MANUAL_CONNECTOR_RECOVERY")`, `nativeProbe(); if os.Getenv("GOSQLITE_MANUAL_CONNECTOR_RECOVERY")`, "manual entry gate"},
		{"missing cfg mutation", connectorManualPath, "cfg.MaxJournalBytes, cfg.MaxRecords, cfg.MaxDatabaseBytes = 1, 1, 1", "cfg.MaxJournalBytes = 1", "post-construction snapshot mutation"},
		{"relaxed journal snapshot", connectorManualPath, "cfg.MaxJournalBytes = 0", "cfg.MaxJournalBytes = 1", "post-construction snapshot mutation"},
		{"missing NoCreate mutation", connectorManualPath, "opts.NoCreate = false", "opts.NoCreate = true", "post-construction snapshot mutation"},
		{"direct recovery path", connectorManualPath, "connector.Connect(context.Background())", "sqlite.OpenContext(context.Background(), path, opts)", "actual Connector call path/count"},
		{"repeated end allowed", connectorManualPath, "scope.frozen || owner.attempted {", "scope.frozen {", "manual ownership/root/read contract"},
		{"end marked late", connectorManualPath, "owner.attempted = true\n\towner.err = owner.value.Close()", "owner.err = owner.value.Close()\n\towner.attempted = true", "retirement must be marked"},
		{"retry implicit rows close", connectorManualPath, "if err == io.EOF {\n\t\towner.retired = true", "if err == io.EOF {\n\t\towner.attempted = false", "manual ownership/root/read contract"},
		{"discard unknown error", connectorManualPath, "scope.errors = append(scope.errors, err)", "scope.errors = nil", "manual ownership/root/read contract"},
		{"automatic deletion", connectorManualPath, "return scope\n}", "os.RemoveAll(\"fixture\"); return scope\n}", "forbidden manual reference"},
		{"child process import", connectorManualPath, "\"context\"", "\"context\"; _ \"os/exec\"", "manual import closure"},
		{"constructor native I/O", "driver/driver.go", "path, mode, err := parseDSNMode(dsn)", "touchFile(dsn); path, mode, err := parseDSNMode(dsn)", "constructor no-I/O"},
		{"parser native I/O", "driver/driver.go", "mode := \"rwc\"", "touchFile(dsn); mode := \"rwc\"", "constructor no-I/O"},
		{"lost policy copy", "driver/driver.go", "copy := *opts.RollbackRecovery", "copy := sqlite.RollbackRecoveryOptions{}", "driver snapshot/Connect/EOF"},
		{"lost public Connect", "driver/driver.go", "sqlite.OpenContext(ctx, c.path, c.opts)", "otherOpen(ctx, c.path, c.opts)", "driver snapshot/Connect/EOF"},
		{"EOF hides cleanup error", "driver/driver.go", "r.inner.Err()", "noError()", "driver snapshot/Connect/EOF"},
		{"input retention weakened", "sqlite/rollback_recovery_manual_linux_test.go", "!bytes.Equal(got, want)", "false", "fixed source changed"},
		{"input publication changed", "sqlite/rollback_recovery_manual_linux_test.go", "os.O_WRONLY|os.O_CREATE|os.O_EXCL", "os.O_WRONLY|os.O_CREATE", "fixed source changed"},
		{"native implicit close changed", "sqlite/rows.go", "r.err = wrapCleanup(\"next\", r.err, r.close())", "r.err = r.err", "fixed source changed"},
		{"policy shape changed", "sqlite/rollback_recovery.go", "MaxRecords       uint32", "MaxRecords       *uint32", "fixed source changed"},
	}
	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			original := string(sources[variant.path])
			if !strings.Contains(original, variant.old) {
				t.Fatal("mutation anchor missing")
			}
			changed := make(map[string][]byte, len(sources))
			for path, raw := range sources {
				changed[path] = raw
			}
			changed[variant.path] = []byte(strings.Replace(original, variant.old, variant.replacement, 1))
			findings, err := inspectConnectorManualSources(changed)
			if err != nil || !strings.Contains(strings.Join(findings, "\n"), variant.finding) {
				t.Fatalf("counterexample was not detected by its source contract: %v %v", findings, err)
			}
		})
	}
}
