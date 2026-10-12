package main

// Pure source/AST contract only: no native test, subprocess, fixture, or build.
// A deliberate source change requires a new independent review and pin update.
import (
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const darwinInventoryManualPath = "internal/encstore/inventory_manual_darwin_test.go"
const darwinInventoryManualTag = "//go:build darwin && manual_darwin_inventory"
const darwinInventoryManualEntry = "TestManualDarwinUnicodeInventory"
const darwinInventoryManualSHA256 = "af6d46be245a224648be56be4bbe2712e35464e8015ce805254ace9a3fdf80ce"

func darwinInventoryManualCallName(expr ast.Expr) string {
	switch x := expr.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return darwinInventoryManualCallName(x.X) + "." + x.Sel.Name
	case *ast.CallExpr:
		return "()"
	case *ast.ArrayType:
		if x.Len == nil && darwinInventoryManualCallName(x.Elt) == "byte" {
			return "[]byte"
		}
	}
	return "<unreviewed expression>"
}

func darwinInventoryManualString(expr ast.Expr, want string) bool {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return false
	}
	value, err := strconv.Unquote(lit.Value)
	return err == nil && value == want
}

func darwinInventoryManualGetenv(expr ast.Expr, name string) bool {
	call, ok := expr.(*ast.CallExpr)
	return ok && darwinInventoryManualCallName(call.Fun) == "os.Getenv" && len(call.Args) == 1 && darwinInventoryManualString(call.Args[0], name)
}

func darwinInventoryManualGate(fn *ast.FuncDecl) bool {
	if fn.Body == nil || len(fn.Body.List) < 3 {
		return false
	}
	gate, ok := fn.Body.List[0].(*ast.IfStmt)
	if !ok || gate.Init != nil || gate.Else != nil || len(gate.Body.List) != 1 {
		return false
	}
	condition, ok := gate.Cond.(*ast.BinaryExpr)
	if !ok || condition.Op != token.NEQ || !darwinInventoryManualGetenv(condition.X, "GOSQLITE_MANUAL_DARWIN_INVENTORY") || !darwinInventoryManualString(condition.Y, "1") {
		return false
	}
	failure, ok := gate.Body.List[0].(*ast.ExprStmt)
	if !ok {
		return false
	}
	call, ok := failure.X.(*ast.CallExpr)
	if !ok || darwinInventoryManualCallName(call.Fun) != "t.Fatal" {
		return false
	}
	root, ok := fn.Body.List[1].(*ast.AssignStmt)
	return ok && root.Tok == token.DEFINE && len(root.Lhs) == 1 && darwinInventoryManualCallName(root.Lhs[0]) == "rootPath" && len(root.Rhs) == 1 && darwinInventoryManualGetenv(root.Rhs[0], "GOSQLITE_MANUAL_DARWIN_INVENTORY_ROOT")
}

// This is an exact bounded entry structure check, not general dominance or
// receiver analysis. The full source pin and independent whole-tree review
// remain required. Slots 0..6 are the previously reviewed env/root/build gates.
func darwinInventoryManualInitialRegistry(fn *ast.FuncDecl) bool {
	if fn.Body == nil || len(fn.Body.List) < 12 {
		return false
	}
	callStatement := func(statement ast.Stmt, name string) bool {
		expression, ok := statement.(*ast.ExprStmt)
		if !ok {
			return false
		}
		call, ok := expression.X.(*ast.CallExpr)
		return ok && darwinInventoryManualCallName(call.Fun) == name && len(call.Args) == 0
	}
	if !callStatement(fn.Body.List[7], "encryptedCleanups.mu.Lock") || !callStatement(fn.Body.List[9], "encryptedCleanups.mu.Unlock") {
		return false
	}
	counts, ok := fn.Body.List[8].(*ast.AssignStmt)
	if !ok || counts.Tok != token.DEFINE || len(counts.Lhs) != 2 || len(counts.Rhs) != 2 || darwinInventoryManualCallName(counts.Lhs[0]) != "initialGroups" || darwinInventoryManualCallName(counts.Lhs[1]) != "initialLeases" || darwinInventoryManualCallName(counts.Rhs[1]) != "encryptedCleanups.leases" {
		return false
	}
	groups, ok := counts.Rhs[0].(*ast.CallExpr)
	if !ok || darwinInventoryManualCallName(groups.Fun) != "len" || len(groups.Args) != 1 || darwinInventoryManualCallName(groups.Args[0]) != "encryptedCleanups.groups" {
		return false
	}
	gate, ok := fn.Body.List[10].(*ast.IfStmt)
	if !ok || gate.Init != nil || gate.Else != nil || len(gate.Body.List) != 1 {
		return false
	}
	condition, ok := gate.Cond.(*ast.BinaryExpr)
	if !ok || condition.Op != token.LOR {
		return false
	}
	nonzero := func(expression ast.Expr, name string) bool {
		comparison, ok := expression.(*ast.BinaryExpr)
		if !ok || comparison.Op != token.NEQ || darwinInventoryManualCallName(comparison.X) != name {
			return false
		}
		zero, ok := comparison.Y.(*ast.BasicLit)
		return ok && zero.Kind == token.INT && zero.Value == "0"
	}
	if !nonzero(condition.X, "initialGroups") || !nonzero(condition.Y, "initialLeases") {
		return false
	}
	failure, ok := gate.Body.List[0].(*ast.ExprStmt)
	if !ok {
		return false
	}
	fatal, ok := failure.X.(*ast.CallExpr)
	if !ok || darwinInventoryManualCallName(fatal.Fun) != "t.Fatalf" || len(fatal.Args) != 3 || !darwinInventoryManualString(fatal.Args[0], "initial registry must be empty: groups=%d leases=%d; existing ownership untouched") || darwinInventoryManualCallName(fatal.Args[1]) != "initialGroups" || darwinInventoryManualCallName(fatal.Args[2]) != "initialLeases" {
		return false
	}
	// The rejection precedes creation/adoption of the ledger and its defer.
	ledger, ok := fn.Body.List[11].(*ast.AssignStmt)
	return ok && ledger.Tok == token.DEFINE && len(ledger.Lhs) == 1 && darwinInventoryManualCallName(ledger.Lhs[0]) == "l"
}

func inspectDarwinInventoryManualSource(path string, raw []byte) ([]string, error) {
	var findings []string
	report := func(message string) { findings = append(findings, path+": Darwin inventory manual: "+message) }
	if filepath.ToSlash(filepath.Clean(path)) != darwinInventoryManualPath {
		report("exact reviewed source path required")
	}
	if strings.SplitN(string(raw), "\n", 2)[0] != darwinInventoryManualTag {
		report("exact darwin AND manual tag required")
	}
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != darwinInventoryManualSHA256 {
		report("reviewed source pin mismatch")
	}
	file, err := parser.ParseFile(token.NewFileSet(), path, raw, 0)
	if err != nil {
		return findings, err
	}
	if file.Name.Name != "encstore" {
		report("package must be encstore")
	}
	approvedImports := map[string]bool{
		"context": true,
		"crypto/sha256": true,
		"errors": true,
		"fmt": true,
		"io": true,
		"os": true,
		"path/filepath": true,
		"runtime": true,
		"runtime/debug": true,
		"strings": true,
		"testing": true,
		"time": true,
		"github.com/ms-xiaosnow/gosqlite/encryption": true,
		"github.com/ms-xiaosnow/gosqlite/internal/darwinfile": true,
		"golang.org/x/sys/unix": true,
	}
	for _, imp := range file.Imports {
		name, err := strconv.Unquote(imp.Path.Value)
		if err != nil || !approvedImports[name] || imp.Name != nil {
			report("unreviewed or aliased import")
		}
	}
	entries := 0
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Name.Name == "init" {
				report("init is forbidden")
			}
			if strings.HasPrefix(d.Name.Name, "Test") {
				entries++
				if d.Name.Name != darwinInventoryManualEntry || d.Recv != nil || !darwinInventoryManualGate(d) {
					report("unique entry and first-action gate/root order required")
				}
				if !darwinInventoryManualInitialRegistry(d) {
					report("initial zero-group/zero-lease gate before ownership required")
				}
			}
		case *ast.GenDecl:
			if d.Tok == token.VAR {
				for _, spec := range d.Specs {
					v, ok := spec.(*ast.ValueSpec)
					if !ok || len(v.Names) != 1 || v.Names[0].Name != "manualDarwinInventoryRetained" || len(v.Values) != 0 {
						report("package initializers are forbidden")
					}
				}
			}
		}
	}
	if entries != 1 {
		report("exactly one native entry required")
	}
	approvedCalls := map[string]bool{
		"().IsRegular": true,
		"().Perm": true,
		"().UnixNano": true,
		"[]byte": true,
		"append": true,
		"context.Background": true,
		"darwinUnicodeInventoryFilesystem": true,
		"darwinfile.Process.Lock": true,
		"darwinfile.Process.Unlock": true,
		"debug.ReadBuildInfo": true,
		"dir.Stat": true,
		"encryptedCleanups.mu.Lock": true,
		"encryptedCleanups.mu.Unlock": true,
		"entry.Name": true,
		"enumerator.ReadDir": true,
		"errors.Is": true,
		"f.ReadAt": true,
		"f.Stat": true,
		"f.Write": true,
		"f.mu.Lock": true,
		"f.mu.Unlock": true,
		"f.rawLocked": true,
		"filepath.Clean": true,
		"filepath.IsAbs": true,
		"filepath.Join": true,
		"fmt.Sprintf": true,
		"g.callbackTerminal.Load": true,
		"g.mu.Lock": true,
		"g.mu.Unlock": true,
		"i.Mode": true,
		"i.Size": true,
		"int": true,
		"int64": true,
		"inventory": true,
		"l.add": true,
		"l.finish": true,
		"l.known": true,
		"l.record": true,
		"l.retire": true,
		"len": true,
		"make": true,
		"manualDarwinInventoryChildIdentity": true,
		"manualDarwinInventoryIdentity": true,
		"manualDarwinInventoryMkdir": true,
		"manualDarwinInventoryRoot": true,
		"manualDarwinInventorySample": true,
		"ns.checked": true,
		"o.file.Close": true,
		"o.file.RetirementComplete": true,
		"o.file.mu.Lock": true,
		"o.file.mu.Unlock": true,
		"openDirectory": true,
		"openFileAt": true,
		"os.Getenv": true,
		"os.Getpid": true,
		"platformDirectoryCheck": true,
		"platformFamilyEntry": true,
		"raw.Fd": true,
		"root.mu.Lock": true,
		"root.mu.Unlock": true,
		"root.rawLocked": true,
		"runtime.Version": true,
		"sha256.Sum256": true,
		"strings.Repeat": true,
		"strings.Split": true,
		"strings.Trim": true,
		"strings.TrimPrefix": true,
		"t.Error": true,
		"t.Errorf": true,
		"t.Fatal": true,
		"t.Fatalf": true,
		"t.Log": true,
		"t.Logf": true,
		"time.Now": true,
		"uint32": true,
		"unix.Fstat": true,
		"unix.Fstatat": true,
		"unix.Fstatfs": true,
		"unix.Geteuid": true,
		"unix.Lstat": true,
		"unix.Mkdirat": true,
		"unix.Sysctl": true,
		"validateLinks": true,
	}
	calls := map[string]int{}
	total := 0
	ast.Inspect(file, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.GoStmt:
			report("goroutines are forbidden")
		case *ast.ForStmt:
			if n.Cond == nil {
				report("unbounded for loop is forbidden")
			}
		case *ast.CallExpr:
			name := darwinInventoryManualCallName(n.Fun)
			calls[name]++
			total++
			if !approvedCalls[name] {
				report("unreviewed actual call: " + name)
			}
		}
		return true
	})
	if total > 256 {
		report("actual call-site budget exceeded")
	}
	for name, want := range map[string]int{
		"unix.Mkdirat": 1, "unix.Fstat": 1, "unix.Fstatfs": 1,
		"unix.Fstatat": 1, "unix.Lstat": 1, "unix.Sysctl": 1,
		"openDirectory": 2, "openFileAt": 2, "ns.checked": 1,
		"inventory": 1, "platformFamilyEntry": 2, "o.file.Close": 1,
	} {
		if calls[name] != want {
			report("reviewed actual call count changed: " + name)
		}
	}
	return findings, nil
}

func inspectDarwinInventoryManualContract(root string) ([]string, error) {
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(darwinInventoryManualPath)))
	if err != nil {
		return nil, err
	}
	return inspectDarwinInventoryManualSource(darwinInventoryManualPath, raw)
}
