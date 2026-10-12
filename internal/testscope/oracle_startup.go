package main

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

const oracleImportPath = "github.com/ms-xiaosnow/gosqlite/internal/testoracle"
const oracleSpecPath = "internal/testoracle/spec.go"
const oracleAdapterPath = "internal/testoracle/test.go"
const oracleSpecTestPath = "internal/testoracle/spec_test.go"

// This is a fixed-source prevention contract, not a Go type checker or a
// whole-program dataflow proof. It deliberately rejects unreviewed constructor
// references even behind unused functions or build tags. Reflection, imported
// helper behavior, raw syscalls, and executable identity need separate review.
// Historical evidence is not executable current-source input to this policy.
func oracleCurrentGoPath(path string) bool {
	path = filepath.ToSlash(filepath.Clean(path))
	return strings.HasSuffix(path, ".go") && !strings.HasPrefix(path, "docs/evidence/") && !strings.HasPrefix(path, "docs/history/")
}

func oracleParents(file *ast.File) map[ast.Node]ast.Node {
	parents := make(map[ast.Node]ast.Node)
	var stack []ast.Node
	ast.Inspect(file, func(n ast.Node) bool {
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
	return parents
}

func oracleEnclosingFunction(n ast.Node, parents map[ast.Node]ast.Node) *ast.FuncDecl {
	for n != nil {
		if _, nested := n.(*ast.FuncLit); nested {
			return nil
		}
		if f, ok := n.(*ast.FuncDecl); ok {
			return f
		}
		n = parents[n]
	}
	return nil
}

func oracleNodeText(set *token.FileSet, raw []byte, n ast.Node) string {
	return string(raw[set.Position(n.Pos()).Offset:set.Position(n.End()).Offset])
}

// Only pointer declarations are needed outside the single reviewed literal.
// In particular this does not admit a type alias, a conversion, new, a generic
// type argument, a composite (including implicit elements), or a method value.
func oraclePointerDeclaration(sel *ast.SelectorExpr, parents map[ast.Node]ast.Node) bool {
	pointer, ok := parents[sel].(*ast.StarExpr)
	if !ok || pointer.X != sel {
		return false
	}
	switch parent := parents[pointer].(type) {
	case *ast.Field:
		return parent.Type == pointer
	case *ast.ValueSpec:
		return parent.Type == pointer && len(parent.Values) == 0
	}
	return false
}

func oracleApprovedLiteral(path string, literal *ast.CompositeLit, parents map[ast.Node]ast.Node) bool {
	if path != oracleSpecPath {
		return false
	}
	f := oracleEnclosingFunction(literal, parents)
	if f == nil || f.Name.Name != "Command" || f.Recv == nil || len(f.Recv.List) != 1 {
		return false
	}
	receiver, ok := f.Recv.List[0].Type.(*ast.Ident)
	if !ok || receiver.Name != "Spec" {
		return false
	}
	address, ok := parents[literal].(*ast.UnaryExpr)
	if !ok || address.Op != token.AND || address.X != literal {
		return false
	}
	ret, ok := parents[address].(*ast.ReturnStmt)
	if !ok || parents[ret] != f.Body || len(ret.Results) != 2 || ret.Results[0] != address {
		return false
	}
	nilResult, ok := ret.Results[1].(*ast.Ident)
	if !ok || nilResult.Name != "nil" || len(literal.Elts) != 4 {
		return false
	}
	seen := map[string]bool{}
	for _, element := range literal.Elts {
		kv, ok := element.(*ast.KeyValueExpr)
		if !ok {
			return false
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok || seen[key.Name] {
			return false
		}
		switch key.Name {
		case "Path", "Args", "Env", "Dir":
		default:
			return false
		}
		call, ok := kv.Value.(*ast.CallExpr)
		if !ok || len(call.Args) != 0 || call.Ellipsis.IsValid() {
			return false
		}
		method, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || method.Sel.Name != key.Name {
			return false
		}
		checked, ok := method.X.(*ast.Ident)
		if !ok || checked.Name != "checked" {
			return false
		}
		seen[key.Name] = true
	}
	return true
}

var oracleNativeCalls = map[string][]string{
	"examples/native-check/main.go": {
		`exec.Command(binary, args...)`,
	},
	"examples/encrypted-cli/main.go": {
		`exec.Command(binary, "backup", path, "--out", archive, "--key-provider", "env-hex:GOSQLITE_DEMO_OLD", "--out-key-provider", "stdin-hex")`,
		`exec.Command(binary, "rekey", path, "--current-key-provider", "env-hex:GOSQLITE_DEMO_OLD", "--key-provider", "env-hex:GOSQLITE_DEMO_NEW")`,
	},
}

func oracleApprovedNativeCall(path string, sel *ast.SelectorExpr, parents map[ast.Node]ast.Node, set *token.FileSet, raw []byte) bool {
	if sel.Sel.Name != "Command" {
		return false
	}
	call, ok := parents[sel].(*ast.CallExpr)
	if !ok || call.Fun != sel {
		return false
	}
	f := oracleEnclosingFunction(call, parents)
	if f == nil || f.Name.Name != "run" || f.Recv != nil {
		return false
	}
	text := oracleNodeText(set, raw, call)
	for _, approved := range oracleNativeCalls[path] {
		if text == approved {
			return true
		}
	}
	return false
}

func inspectOracleSource(path string, raw []byte) ([]string, error) {
	path = filepath.ToSlash(filepath.Clean(path))
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, path, raw, 0)
	if err != nil {
		return nil, err
	}
	parents := oracleParents(file)
	imports := map[string]string{}
	var findings []string
	report := func(pos token.Pos, message string) {
		findings = append(findings, fmt.Sprintf("%s: oracle startup contract: %s", set.Position(pos), message))
	}
	ownPackage := filepath.ToSlash(filepath.Dir(path)) == "internal/testoracle"
	usesOracle := false
	if ownPackage && path != oracleSpecPath && path != oracleAdapterPath && path != oracleSpecTestPath {
		report(file.Pos(), "new factory package file requires source-contract review")
	}
	for _, imp := range file.Imports {
		importPath, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return nil, err
		}
		name := filepath.Base(importPath)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		imports[name] = importPath
		if importPath == oracleImportPath {
			usesOracle = true
			if !ownPackage && !strings.HasSuffix(path, "_test.go") {
				report(imp.Pos(), "testoracle may only be imported by test sources")
			}
		}
		if importPath == "C" {
			report(imp.Pos(), "cgo process bypass requires separate review")
		}
		if (name == "." || name == "_") && (importPath == "os/exec" || importPath == oracleImportPath || importPath == "os" || importPath == "syscall" || importPath == "golang.org/x/sys/unix") {
			report(imp.Pos(), "dot/blank process-related import requires review")
		}
	}
	nativeCount := 0
	factoryCount := 0
	ast.Inspect(file, func(n ast.Node) bool {
		if f, ok := n.(*ast.FuncDecl); ok && ownPackage && (f.Name.Name == "init" || f.Name.Name == "TestMain") {
			report(f.Pos(), "factory package cannot have init or TestMain effects")
		}
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		qualifier, qualified := sel.X.(*ast.Ident)
		importPath := ""
		if qualified {
			importPath = imports[qualifier.Name]
		}
		if importPath == "os/exec" {
			switch sel.Sel.Name {
			case "Cmd":
				if literal, ok := parents[sel].(*ast.CompositeLit); ok && literal.Type == sel && oracleApprovedLiteral(path, literal, parents) {
					factoryCount++
				} else if !oraclePointerDeclaration(sel, parents) {
					report(sel.Pos(), "exec.Cmd construction/type indirection outside reviewed factory")
				}
			case "ExitError", "Error":
				if !oraclePointerDeclaration(sel, parents) {
					report(sel.Pos(), "unreviewed os/exec error-type reference")
				}
			default:
				if oracleApprovedNativeCall(path, sel, parents, set, raw) || restartGuardProcessReference(path, sel, parents, set, raw) {
					nativeCount++
				} else {
					report(sel.Pos(), "os/exec reference "+sel.Sel.Name+" must use testoracle.ForTest")
				}
			}
		}
		if (importPath == "os" && sel.Sel.Name == "StartProcess") || ((importPath == "syscall" || importPath == "golang.org/x/sys/unix") && (sel.Sel.Name == "ForkExec" || sel.Sel.Name == "StartProcess" || sel.Sel.Name == "Exec")) {
			report(sel.Pos(), "low-level process bypass "+sel.Sel.Name)
		}
		if importPath == oracleImportPath && !ownPackage {
			call, direct := parents[sel].(*ast.CallExpr)
			if sel.Sel.Name != "ForTest" || !direct || call.Fun != sel || len(call.Args) < 2 {
				report(sel.Pos(), "outside callers must use direct testoracle.ForTest calls")
			}
		}
		// Conservative in oracle-caller files: also catches field addresses and
		// aliases of the mutable slices. This is not cross-file object tracking.
		if usesOracle && !ownPackage {
			switch sel.Sel.Name {
			case "Path", "Args", "Env", "Dir", "Cancel", "ExtraFiles", "SysProcAttr", "WaitDelay":
				report(sel.Pos(), "caller reference to mutable launch field "+sel.Sel.Name)
			}
		}
		if ownPackage {
			switch sel.Sel.Name {
			case "Start", "Output", "CombinedOutput", "Wait", "Environ", "StdinPipe", "StdoutPipe", "StderrPipe":
				report(sel.Pos(), "factory and pure tests cannot launch or prepare process I/O")
			case "Run":
				if !qualified || qualifier.Name != "t" {
					report(sel.Pos(), "factory and pure tests cannot run a process")
				}
			}
		}
		return true
	})
	if want, native := oracleNativeCalls[path]; native && nativeCount != len(want) {
		report(file.Pos(), fmt.Sprintf("native CLI site count %d, want %d", nativeCount, len(want)))
	}
	if path == oracleSpecPath && factoryCount != 1 {
		report(file.Pos(), fmt.Sprintf("factory literal count %d, want 1", factoryCount))
	}
	return findings, nil
}

// These hashes freeze reviewed exception/source boundaries. They are not
// executable hashes or permission to execute anything. A deliberate future
// source change requires reviewing the complete changed file and this policy.
// Independent source review and compiler/test authorization are still separate.
var oracleFrozenSources = map[string]string{
	oracleSpecTestPath:               "0835caffc85c48e2e12ae357a4be4b417900e74122172aa5fe5d161bd4b83988",
	oracleSpecPath:                   "581490ee249b7d41a3f339a945084fd00105e71aeb15996b1e58b7d7ac4de76d",
	oracleAdapterPath:                "5099e1df87ea477f7c2b84291634265cc95338dc03abb40a6da3237f2f10f87b",
	"examples/native-check/main.go":  "273d5779d688090bfb7fe0f7a857892ae5ef9765f1f3eafb5bf1ae783dadfabc",
	"examples/encrypted-cli/main.go": "5b7bd2e0edd15ee0ea956c74a085be635e1d121500e92eae36a48d86c491004b",
}

func oracleSourceDigest(raw []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}

func inspectOracleFrozenSource(path string, raw []byte) []string {
	want, frozen := oracleFrozenSources[path]
	if frozen && oracleSourceDigest(raw) != want {
		return []string{fmt.Sprintf("%s: reviewed oracle source contract changed; inspect the entire source before updating its hash", path)}
	}
	return nil
}

func inspectOracleContracts(root string) ([]string, error) {
	var findings []string
	for path := range oracleFrozenSources {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return nil, err
		}
		findings = append(findings, inspectOracleFrozenSource(path, raw)...)
	}
	for path := range oracleShellSites {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return nil, err
		}
		findings = append(findings, inspectOracleShellSource(path, raw)...)
	}
	return findings, nil
}
