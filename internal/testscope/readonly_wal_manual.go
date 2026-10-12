package main

// Inert source/AST checks only. Complete-file pins require independent review;
// these checks are a bounded prevention aid, not imported-behavior proof.
import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const readOnlyWALManualNativePath = "sqlite/readonly_wal_manual_linux_test.go"
const readOnlyWALManualPurePath = "sqlite/readonly_wal_manual_contract_test.go"
const readOnlyWALManualGuardPath = "internal/testscope/readonly_wal_manual.go"
const readOnlyWALManualGuardTestPath = "internal/testscope/readonly_wal_manual_test.go"
const readOnlyWALManualTag = "//go:build manual_readonly_wal && linux"
const readOnlyWALManualEntry = "TestManualReadOnlyWALPreparedMedia"

// HOLD: source authoring does not authorize self-review or source pin release.
const readOnlyWALManualNativeSHA256 = "c92b6c7ff45313443cdf4abb492adbe77c082ea398d7fc3316b5aa1e8ac5dad8"
const readOnlyWALManualPureSHA256 = "f012016c3662d802c4878531bc78bf963b14144eb6c48c966b1a086ca5c07f73"

// The complete inspectSource body, including the narrow SQL recovery exception,
// was independently reviewed; this fixed pin still rejects unreviewed changes.
const readOnlyWALManualPriorOrdinarySHA256 = "98341345e43f39b39dbc78eaf49a916a0701f3d7f2f383d4949b9a3ed439d199"

func readOnlyWALManualCompact(node ast.Node) string {
	var out bytes.Buffer
	if err := printer.Fprint(&out, token.NewFileSet(), node); err != nil {
		return "<printer-error>"
	}
	return strings.ReplaceAll(strings.Join(strings.Fields(out.String()), ""), ";", "")
}
func readOnlyWALManualCompactText(text string) string {
	return strings.ReplaceAll(strings.Join(strings.Fields(text), ""), ";", "")
}
func readOnlyWALManualName(expression ast.Expr) string {
	switch x := expression.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return readOnlyWALManualName(x.X) + "." + x.Sel.Name
	case *ast.IndexExpr:
		return readOnlyWALManualName(x.X) + "[]"
	case *ast.ArrayType:
		if x.Len == nil {
			return "[]" + readOnlyWALManualName(x.Elt)
		}
	}
	return "<dynamic>"
}
func readOnlyWALManualFunctions(file *ast.File) map[string]*ast.FuncDecl {
	result := make(map[string]*ast.FuncDecl)
	for _, declaration := range file.Decls {
		if fn, ok := declaration.(*ast.FuncDecl); ok {
			result[fn.Name.Name] = fn
		}
	}
	return result
}
func readOnlyWALManualRequire(functions map[string]*ast.FuncDecl, name, diagnostic string, parts ...string) []string {
	fn := functions[name]
	if fn == nil || fn.Body == nil {
		return []string{diagnostic + ": missing " + name}
	}
	body := readOnlyWALManualCompact(fn.Body)
	for _, part := range parts {
		if !strings.Contains(body, readOnlyWALManualCompactText(part)) {
			return []string{diagnostic + ": reviewed decision/order missing in " + name}
		}
	}
	return nil
}
func readOnlyWALManualCalls(functions map[string]*ast.FuncDecl) map[string]map[string]int {
	result := make(map[string]map[string]int)
	for name, fn := range functions {
		calls := make(map[string]int)
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			if call, ok := node.(*ast.CallExpr); ok {
				calls[readOnlyWALManualName(call.Fun)]++
			}
			return true
		})
		result[name] = calls
	}
	return result
}
func inspectReadOnlyWALManualSource(path string, raw []byte) ([]string, error) {
	path = filepath.ToSlash(filepath.Clean(path))
	file, err := parser.ParseFile(token.NewFileSet(), path, raw, 0)
	if err != nil {
		return nil, err
	}
	var findings []string
	report := func(diagnostic string) { findings = append(findings, path+": readonly WAL manual: "+diagnostic) }
	native, pure := path == readOnlyWALManualNativePath, path == readOnlyWALManualPurePath
	guard := path == readOnlyWALManualGuardPath || path == readOnlyWALManualGuardTestPath
	if !native && !pure {
		if !guard {
			for _, line := range strings.Split(string(raw), "\n") {
				if strings.HasPrefix(strings.TrimSpace(line), "//go:build") && strings.Contains(line, "manual_readonly_wal") {
					report("whole-root copied manual tag")
				}
			}
			ast.Inspect(file, func(node ast.Node) bool {
				if id, ok := node.(*ast.Ident); ok && (id.Name == readOnlyWALManualEntry || strings.HasPrefix(id.Name, "manualReadOnlyWAL")) {
					report("whole-root native/pure declaration or reference escaped approved files")
				}
				return true
			})
		}
		for _, imp := range file.Imports {
			value, e := strconv.Unquote(imp.Path.Value)
			if e != nil {
				return findings, e
			}
			lower := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(value, "_", ""), "-", ""))
			if strings.Contains(lower, "manualreadonlywal") || strings.Contains(lower, "readonlywalmanual") {
				report("whole-root unapproved manual import consumer")
			}
		}
		if path == "internal/testscope/main.go" {
			findings = append(findings, inspectReadOnlyWALManualWiring(raw)...)
		}
		return findings, nil
	}
	if file.Name.Name != "sqlite_test" {
		report("exact external test package required")
	}
	first := strings.SplitN(string(raw), "\n", 2)[0]
	if native && first != readOnlyWALManualTag {
		report("exact Linux AND manual build constraint required")
	}
	if pure && strings.Contains(string(raw), "//go:build") {
		report("pure closure must be untagged")
	}
	pin := readOnlyWALManualNativeSHA256
	if pure {
		pin = readOnlyWALManualPureSHA256
	}
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != pin {
		report("complete source pin requires independent review")
	}
	imports := map[string]bool{}
	allowed := "context crypto/rand crypto/sha256 encoding/binary encoding/hex encoding/json errors fmt os path/filepath runtime runtime/debug slices strconv strings testing time github.com/ms-xiaosnow/gosqlite/sqlite golang.org/x/sys/unix"
	if pure {
		allowed = "encoding/json errors strconv strings testing unicode/utf8"
	}
	for _, value := range strings.Fields(allowed) {
		imports[value] = true
	}
	for _, imp := range file.Imports {
		value, e := strconv.Unquote(imp.Path.Value)
		if e != nil {
			return findings, e
		}
		if !imports[value] || imp.Name != nil {
			if pure {
				report("pure closure import outside descriptor-free allowlist")
			} else {
				report("native effect import/alias outside allowlist")
			}
		}
	}
	functions := readOnlyWALManualFunctions(file)
	entries, retained := 0, 0
	for _, declaration := range file.Decls {
		switch d := declaration.(type) {
		case *ast.FuncDecl:
			if d.Name.Name == "init" || d.Name.Name == "TestMain" || d.Recv != nil || d.Type.TypeParams != nil || d.Name.Name == "MarshalJSON" || d.Name.Name == "UnmarshalJSON" {
				report("initializer/receiver/generic/callback forbidden")
			}
			if strings.HasPrefix(d.Name.Name, "Test") {
				entries++
				if native && d.Name.Name != readOnlyWALManualEntry {
					report("unique native root required")
				}
				if pure && d.Name.Name != "TestReadOnlyWALManualManifestMemory" && d.Name.Name != "TestReadOnlyWALManualProfileMemory" && d.Name.Name != "TestReadOnlyWALManualHandshakeMemory" && d.Name.Name != "TestReadOnlyWALManualRetirementMemory" {
					report("pure closure extra root")
				}
			} else if !strings.HasPrefix(d.Name.Name, "manualReadOnlyWAL") {
				report("unreviewed helper declaration")
			}
		case *ast.GenDecl:
			if d.Tok == token.VAR {
				for _, spec := range d.Specs {
					value, ok := spec.(*ast.ValueSpec)
					if !ok || !native || len(value.Names) != 1 || value.Names[0].Name != "manualReadOnlyWALNativeRetained" || len(value.Values) != 0 {
						report("initializer ownership forbidden")
					} else {
						retained++
					}
				}
			}
		}
	}
	if native && (entries != 1 || retained != 1) {
		report("unique native root and inert retention required")
	}
	if pure && entries != 4 {
		report("pure closure four model roots required")
	}
	parents := oracleParents(file)
	mountFields := readOnlyWALManualMountFields(file, native, parents)
	ownerFields := readOnlyWALManualOwnerFields(file, native, parents)
	ast.Inspect(file, func(node ast.Node) bool {
		switch x := node.(type) {
		case *ast.GoStmt:
			report("native effect goroutine forbidden")
		case *ast.FuncLit:
			report("native/pure callback or dynamic path forbidden")
		case *ast.InterfaceType:
			report("native/pure callback interface forbidden")
		case *ast.IndexListExpr:
			report("generic/dynamic path forbidden")
		case *ast.Ident:
			if x.Name == "Mount" && mountFields[x] {
				break
			}
			switch x.Name {
			case "O_RDWR", "O_TRUNC", "Chmod", "Fchmod", "Chown", "Fchown", "Mount", "Unmount", "Unshare", "Setns", "Setuid", "Setresuid", "Setgid", "Setresgid", "Setgroups", "Capset", "Exec", "Command", "CommandContext", "StartProcess", "FindProcess", "Kill", "Signal", "Checkpoint", "RecoverRollback", "Backup", "Rename", "Renameat", "Renameat2", "Link", "Linkat", "Unlink", "Unlinkat", "Remove", "RemoveAll", "TempDir", "Copy", "Skip", "Skipf", "SkipNow", "RetryOpenCleanup", "RetryCloseCleanup", "RetryPlaintextCleanup", "RetryEncryptedCleanup", "Environ":
				report("native effect forbidden operation " + x.Name)
			case "Faccessat", "Access", "STATX_MNT_ID_UNIQUE":
				report("profile requires direct effective access and ordinary mount ID")
			}
		case *ast.CallExpr:
			if readOnlyWALManualName(x.Fun) == "<dynamic>" {
				report("native/pure dynamic call forbidden")
			}
		}
		return true
	})
	for _, line := range strings.Split(string(raw), "\n") {
		if strings.Contains(line, "go:linkname") {
			report("linkname forbidden")
		}
	}
	shared := map[string]bool{}
	for _, name := range strings.Fields("manualReadOnlyWALHex manualReadOnlyWALPath manualReadOnlyWALSeparate manualReadOnlyWALDecodeManifest manualReadOnlyWALDecodeReceipt manualReadOnlyWALValidateProfile manualReadOnlyWALMountValid manualReadOnlyWALAdvancePhase manualReadOnlyWALReserve manualReadOnlyWALAdopt manualReadOnlyWALBeginEnd manualReadOnlyWALFinishEnd manualReadOnlyWALFinishNext manualReadOnlyWALRetired") {
		shared[name] = true
	}
	builtins := map[string]bool{}
	for _, name := range strings.Fields("len make append string int int64 uint32 uint64 byte []byte") {
		builtins[name] = true
	}
	ast.Inspect(file, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.CallExpr:
			if id, ok := n.Fun.(*ast.Ident); ok && functions[id.Name] == nil && !builtins[id.Name] && !(native && (shared[id.Name] || readOnlyWALManualBoundCancel(n, parents, functions))) {
				report("native/pure unreviewed direct or function-value call")
			}
		case *ast.Ident:
			if functions[n.Name] != nil || native && shared[n.Name] {
				if declaration, ok := parents[n].(*ast.FuncDecl); ok && declaration.Name == n {
					break
				}
				if call, ok := parents[n].(*ast.CallExpr); !ok || call.Fun != n {
					report("native/pure function-value reference forbidden")
				}
			}
		case *ast.SelectorExpr:
			name := readOnlyWALManualName(n)
			for _, method := range strings.Fields("Close Next Err Commit Rollback Query Schema Inspect Check Begin") {
				if n.Sel.Name == method && !ownerFields[n] {
					if call, ok := parents[n].(*ast.CallExpr); !ok || call.Fun != n {
						report("native/pure owner method-value reference forbidden")
					}
				}
			}
			if strings.HasPrefix(name, "unix.") || strings.HasPrefix(name, "sqlite.") {
				if call, ok := parents[n].(*ast.CallExpr); ok && call.Fun == n {
					break
				}
				// Uppercase constants/types are inert. Function names in these
				// imports are explicitly inventoried rather than assumed inert.
				for _, method := range strings.Fields("Open Openat Openat2 Mkdirat Read Pread Write Fsync Close Statx Fstat Fstatfs Faccessat2 Getresuid Getresgid Getgroups Capget Seek ReadDirent ParseDirent Poll OpenContext PendingCloseCleanups PendingPlaintextCleanups") {
					if n.Sel.Name == method {
						report("native effect function-value reference forbidden")
					}
				}
			}
		}
		return true
	})
	if native {
		findings = append(findings, readOnlyWALManualNativeSemantics(file, functions)...)
	} else {
		findings = append(findings, readOnlyWALManualPureSemantics(file, functions)...)
	}
	return findings, nil
}

// Next is also the integer handshake counter, and InspectResult.Schema is
// data. Exempt only those reviewed fields, bound to their actual lexical
// declarations and immediate acquisition. No owner method name is exempted.
func readOnlyWALManualOwnerFields(file *ast.File, native bool, parents map[ast.Node]ast.Node) map[*ast.SelectorExpr]bool {
	allowed := make(map[*ast.SelectorExpr]bool)
	if file.Name.Name != "sqlite_test" || file.Scope == nil || file.Scope.Lookup("int") != nil || native && file.Scope.Lookup("sqlite") != nil {
		return allowed
	}
	shapes := map[string]string{
		"manualReadOnlyWALPhase": `manualReadOnlyWALPhase struct { Next                                  int; RunID, Session, Generation, Challenge string; Used                                  [4]string }`,
	}
	if native {
		shapes["manualReadOnlyWALNativeRun"] = `manualReadOnlyWALNativeRun struct { Ledger                                                        manualReadOnlyWALLedger; Owners                                                        []manualReadOnlyWALNativeOwner; Errors                                                        []error; Manifest                                                      manualReadOnlyWALManifest; RawManifest                                                   []byte; Phase                                                         manualReadOnlyWALPhase; Source, Evidence, Child, Events, Input, Control, ManifestFile int; Family                                                        [3]int; Candidate                                                     string; ControlPath, ManifestName                                     string; ControlStat, ManifestStat                                     manualReadOnlyWALNativeStat; APICtx                                                        context.Context; Locked, Complete, OutputFailed, Started, Preexisting          bool; Failure                                                       string; LastErrno                                                     uint64; Baseline                                                      *manualReadOnlyWALNativeSnapshot }`
		shapes["manualReadOnlyWALNativeOwner"] = `manualReadOnlyWALNativeOwner struct { FD                 int; DB                 *sqlite.DB; Tx                 *sqlite.Tx; Rows               *sqlite.Rows; Original, Terminal error; Label              string }`
	}
	for name, shape := range shapes {
		object := file.Scope.Lookup(name)
		// The native counter type is supplied by the separately pinned pure
		// companion; a same-name native declaration is never a substitute.
		if native && name == "manualReadOnlyWALPhase" {
			if object != nil {
				return allowed
			}
			continue
		}
		if object == nil {
			return allowed
		}
		spec, ok := object.Decl.(*ast.TypeSpec)
		if !ok || spec.Assign.IsValid() || spec.TypeParams != nil || readOnlyWALManualCompact(spec) != readOnlyWALManualCompactText(shape) {
			return allowed
		}
	}
	for _, imp := range file.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		if err != nil || imp.Name != nil || shapes[filepath.Base(path)] != "" {
			return allowed
		}
	}
	typed := func(expression ast.Expr, name string, pointer bool) bool {
		if pointer {
			star, ok := expression.(*ast.StarExpr)
			if !ok {
				return false
			}
			expression = star.X
		}
		id, ok := expression.(*ast.Ident)
		return ok && id.Name == name && id.Obj == file.Scope.Lookup(name)
	}
	parameter := func(expression ast.Expr, fn *ast.FuncDecl, name, kind string, pointer bool) bool {
		id, ok := expression.(*ast.Ident)
		if !ok || id.Name != name || id.Obj == nil || fn.Type.Params == nil {
			return false
		}
		for _, field := range fn.Type.Params.List {
			for _, declaration := range field.Names {
				if declaration.Obj == id.Obj && id.Obj.Decl == field && typed(field.Type, kind, pointer) {
					return true
				}
			}
		}
		return false
	}
	ast.Inspect(file, func(node ast.Node) bool {
		selector, ok := node.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Next" && selector.Sel.Name != "Schema" {
			return true
		}
		fn := oracleEnclosingFunction(selector, parents)
		if fn == nil || fn.Recv != nil {
			return true
		}
		field := false
		if selector.Sel.Name == "Next" {
			if native && (fn.Name.Name == "manualReadOnlyWALNativeSnapshotNow" || fn.Name.Name == "manualReadOnlyWALNativeReceipt" || fn.Name.Name == "manualReadOnlyWALNativeRunPrepared") {
				phase, ok := selector.X.(*ast.SelectorExpr)
				field = ok && phase.Sel.Name == "Phase" && parameter(phase.X, fn, "r", "manualReadOnlyWALNativeRun", true)
			}
			if !native && fn.Name.Name == "manualReadOnlyWALAdvancePhase" {
				field = parameter(selector.X, fn, "s", "manualReadOnlyWALPhase", false)
			}
			if !native && fn.Name.Name == "TestReadOnlyWALManualHandshakeMemory" {
				if id, ok := selector.X.(*ast.Ident); ok && id.Name == "s" && id.Obj != nil {
					if declaration, ok := id.Obj.Decl.(*ast.AssignStmt); ok && declaration.Tok == token.DEFINE && len(declaration.Lhs) == 1 && len(declaration.Rhs) == 1 && parents[declaration] == fn.Body {
						if lhs, ok := declaration.Lhs[0].(*ast.Ident); ok && lhs.Obj == id.Obj {
							if literal, ok := declaration.Rhs[0].(*ast.CompositeLit); ok {
								field = typed(literal.Type, "manualReadOnlyWALPhase", false)
							}
						}
					}
				}
			}
		} else if native && fn.Name.Name == "manualReadOnlyWALNativeMetadata" {
			if id, ok := selector.X.(*ast.Ident); ok && id.Name == "inspection" && id.Obj != nil {
				if declaration, ok := id.Obj.Decl.(*ast.AssignStmt); ok && declaration.Tok == token.DEFINE && len(declaration.Lhs) == 2 && len(declaration.Rhs) == 1 && parents[declaration] == fn.Body {
					lhs, lhsOK := declaration.Lhs[0].(*ast.Ident)
					call, callOK := declaration.Rhs[0].(*ast.CallExpr)
					if lhsOK && lhs.Obj == id.Obj && callOK && readOnlyWALManualCompact(call) == readOnlyWALManualCompactText(`r.Owners[dbID].DB.Inspect(r.APICtx, sqlite.InspectOptions{IncludeExtensions: true})`) {
						field = true
						ast.Inspect(call, func(node ast.Node) bool {
							if id, ok := node.(*ast.Ident); ok {
								if id.Name == "r" && !parameter(id, fn, "r", "manualReadOnlyWALNativeRun", true) || id.Name == "dbID" && !parameter(id, fn, "dbID", "int", false) {
									field = false
								}
							}
							return true
						})
					}
				}
			}
		}
		if !field {
			return true
		}
		// Accept the existing comparison/index reads, the pure counter's
		// single increment shape, and the direct schema assertion argument.
		// Captures, addresses, assignments, ranges and callable fields do not
		// gain an exception merely because they reuse these selector names.
		switch use := parents[selector].(type) {
		case *ast.BinaryExpr:
			allowed[selector] = selector.Sel.Name == "Next"
		case *ast.IndexExpr:
			allowed[selector] = selector.Sel.Name == "Next" && use.Index == selector
		case *ast.IncDecStmt:
			if !native && fn.Name.Name == "manualReadOnlyWALAdvancePhase" && selector.Sel.Name == "Next" && use.Tok == token.INC && parents[use] == fn.Body {
				increments := 0
				ast.Inspect(fn.Body, func(node ast.Node) bool {
					if step, ok := node.(*ast.IncDecStmt); ok && readOnlyWALManualCompact(step.X) == "s.Next" {
						increments++
					}
					return true
				})
				allowed[selector] = increments == 1
			}
		case *ast.CallExpr:
			id, ok := use.Fun.(*ast.Ident)
			allowed[selector] = selector.Sel.Name == "Schema" && ok && id.Name == "manualReadOnlyWALNativeSchemaOK" && id.Obj != nil && id.Obj == file.Scope.Lookup(id.Name) && len(use.Args) == 1 && use.Args[0] == selector
		}
		return true
	})
	return allowed
}

// Mount is also the name of an inert, non-callable observation field. Bind
// that exception to the reviewed model shapes, lexical objects and five
// existing functions. This is deliberately not a general type checker: an
// unknown type, receiver, origin or context keeps the ordinary identifier ban.
// Native model names come from the separately pinned pure companion; local
// redeclarations cannot stand in for those definitions.
func readOnlyWALManualMountFields(file *ast.File, native bool, parents map[ast.Node]ast.Node) map[*ast.Ident]bool {
	allowed := make(map[*ast.Ident]bool)
	if file.Name.Name != "sqlite_test" || file.Scope == nil {
		return allowed
	}
	shapes := map[string]string{
		"manualReadOnlyWALMount":  `manualReadOnlyWALMount struct { Observed bool; ID uint64; Matches int; Major, Minor uint32; Root, Point, Options, SuperOptions, Type string; Magic int64; ReadOnly bool }`,
		"manualReadOnlyWALObject": `manualReadOnlyWALObject struct { Observed bool; Identity manualReadOnlyWALIdentity; UID uint32; Mode uint32; Links uint32; Mount manualReadOnlyWALMount; ReadAllowed bool; AccessObserved bool; WriteResult string; AccessCall string; AccessFlags, ReadMask, WriteMask uint32 }`,
		"manualReadOnlyWALFacts":  `manualReadOnlyWALFacts struct { Credentials manualReadOnlyWALCredentials; Source [4]manualReadOnlyWALObject; Evidence manualReadOnlyWALObject; DistinctLocations bool }`,
	}
	if native {
		shapes["manualReadOnlyWALNativeSnapshot"] = `manualReadOnlyWALNativeSnapshot struct { Facts manualReadOnlyWALFacts; Control manualReadOnlyWALObject; Stats [4]manualReadOnlyWALNativeStat; Hashes [3]string; Groups []uint32 }`
	}
	for _, imp := range file.Imports {
		value, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return allowed
		}
		name := filepath.Base(value)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		if name == "." || shapes[name] != "" {
			return allowed
		}
	}
	for name, shape := range shapes {
		object := file.Scope.Lookup(name)
		if native && name != "manualReadOnlyWALNativeSnapshot" {
			if object != nil {
				return allowed
			}
			continue
		}
		if object == nil {
			return allowed
		}
		spec, ok := object.Decl.(*ast.TypeSpec)
		if !ok || spec.Assign.IsValid() || spec.TypeParams != nil || readOnlyWALManualCompact(spec) != readOnlyWALManualCompactText(shape) {
			return allowed
		}
	}
	var declared func(ast.Expr) string
	declared = func(expression ast.Expr) string {
		switch x := expression.(type) {
		case *ast.Ident:
			if shapes[x.Name] != "" && x.Obj == file.Scope.Lookup(x.Name) {
				return x.Name
			}
		case *ast.StarExpr:
			return declared(x.X)
		case *ast.ArrayType:
			if x.Len != nil && readOnlyWALManualCompact(x.Len) == "4" && declared(x.Elt) == "manualReadOnlyWALObject" {
				return "[4]manualReadOnlyWALObject"
			}
		}
		return ""
	}
	fields := map[string]map[string]string{
		"manualReadOnlyWALNativeSnapshot": {"Facts": "manualReadOnlyWALFacts", "Control": "manualReadOnlyWALObject"},
		"manualReadOnlyWALFacts":          {"Source": "[4]manualReadOnlyWALObject", "Evidence": "manualReadOnlyWALObject"},
	}
	var result func(*ast.CallExpr, int) string
	result = func(call *ast.CallExpr, index int) string {
		id, ok := call.Fun.(*ast.Ident)
		if !ok || id.Obj == nil || id.Obj != file.Scope.Lookup(id.Name) {
			return ""
		}
		fn, ok := id.Obj.Decl.(*ast.FuncDecl)
		if !ok || fn.Recv != nil || fn.Type.Results == nil {
			return ""
		}
		for _, field := range fn.Type.Results.List {
			count := len(field.Names)
			if count == 0 {
				count = 1
			}
			if index < count {
				return declared(field.Type)
			}
			index -= count
		}
		return ""
	}
	var inferred func(ast.Expr, int) string
	inferred = func(expression ast.Expr, depth int) string {
		if depth > 32 {
			return ""
		}
		switch x := expression.(type) {
		case *ast.Ident:
			if x.Obj == nil || x.Obj.Kind != ast.Var {
				return ""
			}
			switch d := x.Obj.Decl.(type) {
			case *ast.Field:
				return declared(d.Type)
			case *ast.ValueSpec:
				return declared(d.Type)
			case *ast.AssignStmt:
				if d.Tok != token.DEFINE {
					return ""
				}
				for i, lhs := range d.Lhs {
					id, ok := lhs.(*ast.Ident)
					if !ok || id.Obj != x.Obj {
						continue
					}
					// go/parser binds range variables through a synthetic
					// AssignStmt. Use their real declaration parent to
					// distinguish the object value from the integer key.
					if ranged, ok := parents[id].(*ast.RangeStmt); ok && ranged.Tok == token.DEFINE && ranged.Value == id && inferred(ranged.X, depth+1) == "[4]manualReadOnlyWALObject" {
						return "manualReadOnlyWALObject"
					}
					if len(d.Lhs) == len(d.Rhs) {
						return inferred(d.Rhs[i], depth+1)
					}
					if len(d.Rhs) == 1 {
						if call, ok := d.Rhs[0].(*ast.CallExpr); ok {
							return result(call, i)
						}
					}
				}
			case *ast.RangeStmt:
				id, ok := d.Value.(*ast.Ident)
				if d.Tok == token.DEFINE && ok && id.Obj == x.Obj && inferred(d.X, depth+1) == "[4]manualReadOnlyWALObject" {
					return "manualReadOnlyWALObject"
				}
			}
		case *ast.SelectorExpr:
			return fields[inferred(x.X, depth+1)][x.Sel.Name]
		case *ast.IndexExpr:
			if inferred(x.X, depth+1) == "[4]manualReadOnlyWALObject" {
				return "manualReadOnlyWALObject"
			}
		case *ast.UnaryExpr:
			if x.Op == token.AND {
				return inferred(x.X, depth+1)
			}
		case *ast.CompositeLit:
			return declared(x.Type)
		case *ast.CallExpr:
			return result(x, 0)
		}
		return ""
	}
	ast.Inspect(file, func(node ast.Node) bool {
		id, ok := node.(*ast.Ident)
		if !ok || id.Name != "Mount" {
			return true
		}
		if field, ok := parents[id].(*ast.Field); ok {
			spec := file.Scope.Lookup("manualReadOnlyWALObject")
			if !native && spec != nil {
				structure := spec.Decl.(*ast.TypeSpec).Type.(*ast.StructType)
				allowed[id] = parents[field] == structure.Fields && len(field.Names) == 1 && field.Names[0] == id && declared(field.Type) == "manualReadOnlyWALMount"
			}
			return true
		}
		fn := oracleEnclosingFunction(id, parents)
		if fn == nil || fn.Recv != nil {
			return true
		}
		approved := native && (fn.Name.Name == "manualReadOnlyWALNativeObserveObject" || fn.Name.Name == "manualReadOnlyWALNativeMetadataNow") || !native && (fn.Name.Name == "manualReadOnlyWALValidateProfile" || fn.Name.Name == "manualReadOnlyWALMemoryFacts" || fn.Name.Name == "TestReadOnlyWALManualProfileMemory")
		if !approved {
			return true
		}
		switch context := parents[id].(type) {
		case *ast.KeyValueExpr:
			literal, ok := parents[context].(*ast.CompositeLit)
			allowed[id] = ok && context.Key == id && declared(literal.Type) == "manualReadOnlyWALObject"
		case *ast.SelectorExpr:
			if context.Sel != id || inferred(context.X, 0) != "manualReadOnlyWALObject" {
				break
			}
			var use ast.Node = context
			for {
				if paren, ok := parents[use].(*ast.ParenExpr); ok {
					use = paren
					continue
				}
				if selector, ok := parents[use].(*ast.SelectorExpr); ok && selector.X == use {
					use = selector
					continue
				}
				break
			}
			allowed[id] = true
			switch useParent := parents[use].(type) {
			case *ast.CallExpr:
				allowed[id] = useParent.Fun != use
			case *ast.UnaryExpr:
				allowed[id] = useParent.Op != token.AND
			case *ast.AssignStmt:
				for _, lhs := range useParent.Lhs {
					if lhs == use {
						allowed[id] = !native && fn.Name.Name == "TestReadOnlyWALManualProfileMemory" && use != context
					}
				}
			case *ast.IncDecStmt:
				allowed[id] = !native && fn.Name.Name == "TestReadOnlyWALManualProfileMemory" && use != context
			case *ast.RangeStmt:
				// Range assignment targets are writes too. The reviewed
				// pure memory variants need no range-target exception.
				allowed[id] = useParent.Key != use && useParent.Value != use
			}
		}
		return true
	})
	return allowed
}

func readOnlyWALManualNativeSemantics(file *ast.File, functions map[string]*ast.FuncDecl) []string {
	var findings []string
	require := func(name, diagnostic string, parts ...string) {
		findings = append(findings, readOnlyWALManualRequire(functions, name, diagnostic, parts...)...)
	}
	root := functions[readOnlyWALManualEntry]
	if root == nil || root.Body == nil || len(root.Body.List) < 8 {
		findings = append(findings, "gate/ledger first-action structure missing")
	} else {
		first := readOnlyWALManualCompact(root.Body.List[0])
		if first != readOnlyWALManualCompactText(`if os.Getenv("GOSQLITE_MANUAL_READONLY_WAL") != "independently_authorized" { t.Fatal("NOT_RUN: independent authorization gate absent") }`) {
			findings = append(findings, "gate must be the first native action")
		}
		if !strings.HasPrefix(readOnlyWALManualCompact(root.Body.List[1]), `iferr:=manualReadOnlyWALNativeArgv(os.Args)err!=nil{t.Fatal(`) || readOnlyWALManualCompact(root.Body.List[3]) != "manualReadOnlyWALNativeRetained=r" || readOnlyWALManualCompact(root.Body.List[4]) != "defermanualReadOnlyWALNativeFinish(t,r)" {
			findings = append(findings, "gate/ledger-before-acquisition order")
		}
		if !strings.Contains(readOnlyWALManualCompact(root.Body.List[5]), "len(sqlite.PendingCloseCleanups())!=0||len(sqlite.PendingPlaintextCleanups())!=0") {
			findings = append(findings, "ownership initial shared/plain pending snapshot gate")
		}
	}
	require("manualReadOnlyWALNativeArgv", "gate exact argv", `want := []string{"-test.run=^TestManualReadOnlyWALPreparedMedia$","-test.v=true","-test.count=1","-test.failfast=true","-test.parallel=1","-test.shuffle=off","-test.timeout=0"}`, `len(args) != len(want)+1`, `args[i+1] != arg`)
	require("manualReadOnlyWALNativeDirectory", "effect readonly root acquisition", `id, err := manualReadOnlyWALNativeReserve(r,-1,"fd",label)`, `fd, original := unix.Openat2(unix.AT_FDCWD,path,&unix.OpenHow{Flags:unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC,Resolve:unix.RESOLVE_NO_SYMLINKS|unix.RESOLVE_NO_MAGICLINKS})`, `capture := manualReadOnlyWALNativeCaptureFD(r,id,fd,original)`)
	require("manualReadOnlyWALNativeRelative", "effect readonly relative acquisition", `flags := uint64(unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NONBLOCK|unix.O_NOFOLLOW)`, `Resolve:unix.RESOLVE_BENEATH|unix.RESOLVE_NO_SYMLINKS|unix.RESOLVE_NO_MAGICLINKS|unix.RESOLVE_NO_XDEV`)
	require("manualReadOnlyWALNativeCaptureFD", "ownership original FD capture", `r.Owners[id].FD, r.Owners[id].Original = fd, original`, `return manualReadOnlyWALAdopt(&r.Ledger,id,fd >= 0,original != nil,false)`)
	require("manualReadOnlyWALNativeStatFD", "profile returned statx mask", `unix.Statx(r.Owners[id].FD,"",unix.AT_EMPTY_PATH|unix.AT_SYMLINK_NOFOLLOW,mask,&s)`, `s.Mask&mask != mask`, `unix.STATX_MNT_ID`)
	require("manualReadOnlyWALNativeObserveObject", "profile observed mount/access", `unix.Fstatfs(r.Owners[id].FD,&fs)`, `mount.ReadOnly = fs.Flags&unix.ST_RDONLY != 0`, `matches := mounts[s.MountID]`, `len(matches) != 1`, `unix.Faccessat2(r.Owners[id].FD,"",mode,unix.AT_EMPTY_PATH|unix.AT_EACCESS)`, `write := unix.Faccessat2(r.Owners[id].FD,"",unix.W_OK,unix.AT_EMPTY_PATH|unix.AT_EACCESS)`)
	require("manualReadOnlyWALNativeCredentials", "profile actual credentials/capability words", `unix.Getresuid()`, `unix.Getresgid()`, `unix.Getgroups()`, `var capData [2]unix.CapUserData`, `unix.Capget(&capHeader,&capData[0])`, `unix.LINUX_CAPABILITY_VERSION_3`, `"/proc/thread-self/status",65536`, `c.GroupsMatch = slices.Equal(observedGroups,procGroups)`, `c.Ambient = sets["CapAmb"]`, `c.ProcMatches =`)
	require("manualReadOnlyWALNativeMetadataNow", "profile pure admission decision", `if r.Ledger.Blocked { return snapshot,`, `if err := manualReadOnlyWALValidateProfile(snapshot.Facts,r.Manifest); err != nil { return snapshot,`, `if err := manualReadOnlyWALNativeNames(r); err != nil { return snapshot,err }`)
	require("manualReadOnlyWALNativeSnapshotNow", "profile post-receipt content decision", `snapshot, err := manualReadOnlyWALNativeMetadataNow(r)`, `digest != member.SHA256`, `after != snapshot.Stats[i+1]`)
	require("manualReadOnlyWALNativeMounts", "profile bounded self mountinfo", `"/proc/self/mountinfo",4<<20`, `len(lines) > 8193`, `len(line) > 8192`, `manualReadOnlyWALNativeUnescape(a[3])`, `manualReadOnlyWALNativeUnescape(a[4])`)
	require("manualReadOnlyWALNativeProc", "effect exact bounded self metadata", `path != "/proc/thread-self/status" && path != "/proc/self/mountinfo"`, `unix.Open(path,unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NONBLOCK,0)`)
	require("manualReadOnlyWALNativeEvidence", "effect exclusive evidence only", `r.Candidate = filepath.Join(r.Manifest.EvidenceParent,"readonly-wal-"+r.Manifest.RunID)`, `unix.Mkdirat(r.Owners[r.Evidence].FD,"readonly-wal-"+r.Manifest.RunID,0700)`, `[]string{"events.ndjson","input-manifest.json"}`, `unix.Openat(r.Owners[r.Child].FD,name,unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC|unix.O_NOFOLLOW,0600)`)
	require("manualReadOnlyWALNativeWrite", "effect evidence-owned write/sync", `id != r.Events && id != r.Input`, `r.Ledger.Owners[id].State != manualReadOnlyWALLive`, `r.Owners[id].Label != "events.ndjson" && r.Owners[id].Label != "input-manifest.json"`, `n != len(raw)`, `unix.Fsync(r.Owners[id].FD)`)
	require("manualReadOnlyWALNativeWrite", "effect evidence write receiver binding", `unix.Write(r.Owners[id].FD,raw)`)
	require("manualReadOnlyWALNativeRows", "effect sole fixed Query arguments", `r.Owners[parent].DB.Query(r.APICtx,manualReadOnlyWALQuery)`, `r.Owners[parent].Tx.Query(r.APICtx,manualReadOnlyWALQuery)`)
	require("manualReadOnlyWALNativeRows", "effect fixed row columns", `slices.Equal(rows.Columns(),[]string{"id","text"})`)
	require("manualReadOnlyWALNativeReceipt", "gate fresh bounded keeper receipt", `rand.Read(challenge[:])`, `time.Now().Add(30*time.Second)`, `len(line) < 2048`, `unix.Read(0,b[:])`, `manualReadOnlyWALDecodeReceipt(line)`, `next, err := manualReadOnlyWALAdvancePhase(r.Phase,receipt); if err != nil { return`, `r.Phase = next`)
	require("manualReadOnlyWALNativeCase", "ownership exact Open capture", `id, err := manualReadOnlyWALNativeReserve(r,r.Source,"db","readonly-shared-WAL")`, `db, original := sqlite.OpenContext(r.APICtx,filepath.Join(r.Manifest.SourceRoot,"readonly.db"),sqlite.Options{ReadOnly:true,NoCreate:true,WALShared:true,KeepWALOnClose:keep,MaxSortBytes:1<<20})`, `r.Owners[id].DB, r.Owners[id].Original = db,original`, `capture := manualReadOnlyWALAdopt(&r.Ledger,id,db != nil,original != nil,original != nil)`)
	caseFn := functions["manualReadOnlyWALNativeCase"]
	if caseFn == nil || caseFn.Body == nil || len(caseFn.Body.List) == 0 || !strings.HasPrefix(readOnlyWALManualCompact(caseFn.Body.List[0]), "iferr:=manualReadOnlyWALNativeReceipt(r)err!=nil{returnerr}") {
		findings = append(findings, "gate keeper receipt precedes case observation/Open")
	}
	require("manualReadOnlyWALNativeRunPrepared", "gate sequential fail-stop cases", `if err := manualReadOnlyWALNativeCase(r,false); err != nil { return err }; if err := manualReadOnlyWALNativeCase(r,true); err != nil { return err }`, `r.Phase.Next != 4`, `manualReadOnlyWALDecodeManifest(raw)`, `sha256.Sum256(raw)`, `pipe.Mode&unix.S_IFMT != unix.S_IFIFO`)
	require("manualReadOnlyWALNativeEnd", "ownership consuming-before-terminal", `if err := manualReadOnlyWALBeginEnd(&r.Ledger,id,action); err != nil { return`, `r.Owners[id].Terminal = original`, `decision := manualReadOnlyWALFinishEnd(&r.Ledger,id,original != nil)`, `if original != nil || decision != nil { return`)
	require("manualReadOnlyWALNativeNext", "ownership implicit EOF retirement", `if err := manualReadOnlyWALBeginEnd(&r.Ledger,id,"next"); err != nil { return`, `next := r.Owners[id].Rows.Next()`, `if !next { original = r.Owners[id].Rows.Err() }`, `r.Owners[id].Terminal = original`, `decision := manualReadOnlyWALFinishNext(&r.Ledger,id,next,original != nil)`)
	require("manualReadOnlyWALNativeRows", "ownership query and EOF result", `r.Owners[id].Rows, r.Owners[id].Original = rows,original`, `manualReadOnlyWALAdopt(&r.Ledger,id,rows != nil,original != nil,original != nil)`, `if early { return manualReadOnlyWALNativeEnd(r,id,"close") }`, `if next { return manualReadOnlyWALNativeFail(r,"unexpected third row",nil) }; return nil`)
	require("manualReadOnlyWALNativeTransaction", "ownership commit/rollback alternatives", `r.Owners[id].Tx, r.Owners[id].Original = tx,original`, `sqlite.TxOptions{}`, `if commit { return manualReadOnlyWALNativeEnd(r,id,"commit") }; return manualReadOnlyWALNativeEnd(r,id,"rollback")`)
	require("manualReadOnlyWALNativeMetadata", "effect fixed metadata/check budgets", `sqlite.InspectOptions{IncludeExtensions:true}`, `sqlite.CheckOptions{MaxPages:16,MaxPageReads:256}`, `check.Scope != sqlite.CheckScope`, `check.PageReads > 256`, `h.PageCount != 3`, `inspection.ExtensionIndexes == nil`)
	require("manualReadOnlyWALNativeReadFinish", "ownership opaque read cleanup barrier", `r.Owners[id].Original, r.Owners[id].Terminal = original,original`, `manualReadOnlyWALFinishEnd(&r.Ledger,id,original != nil)`)
	require("manualReadOnlyWALNativeFinish", "ownership final retirement controls result", `state.State == manualReadOnlyWALReserved || state.State == manualReadOnlyWALConsuming`, `state.State = manualReadOnlyWALUnknown; r.Ledger.Blocked = true`, `child.Parent == id && child.State != manualReadOnlyWALEnded`, `r.Ledger.Owners[id].State != manualReadOnlyWALLive`, `eligible := r.Complete && r.Failure == "" && !r.OutputFailed && !pending && manualReadOnlyWALRetired(r.Ledger)`, `Status:status`, `if !eligible { t.Error(`)
	for name, callee := range map[string]string{
		"manualReadOnlyWALNativeDirectory":   "unix.Openat2(",
		"manualReadOnlyWALNativeRelative":    "unix.Openat2(",
		"manualReadOnlyWALNativeProc":        "unix.Open(",
		"manualReadOnlyWALNativeRows":        ".Query(",
		"manualReadOnlyWALNativeTransaction": ".Begin(",
		"manualReadOnlyWALNativeCase":        "sqlite.OpenContext(",
	} {
		fn := functions[name]
		if fn == nil || fn.Body == nil {
			findings = append(findings, "ownership reserve-before-acquisition missing")
			continue
		}
		body := readOnlyWALManualCompact(fn.Body)
		reservation, acquisition := strings.Index(body, "manualReadOnlyWALNativeReserve("), strings.Index(body, callee)
		if reservation < 0 || acquisition < 0 || reservation > acquisition {
			findings = append(findings, "ownership reserve-before-acquisition order: "+name)
		}
	}
	environment := make(map[string]int)
	for _, fn := range functions {
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || readOnlyWALManualName(call.Fun) != "os.Getenv" {
				return true
			}
			if len(call.Args) != 1 {
				findings = append(findings, "effect named environment only")
				return true
			}
			literal, ok := call.Args[0].(*ast.BasicLit)
			if !ok {
				findings = append(findings, "effect named environment only")
				return true
			}
			key, err := strconv.Unquote(literal.Value)
			if err != nil || key != "GOSQLITE_MANUAL_READONLY_WAL" && key != "GOSQLITE_MANUAL_READONLY_WAL_MANIFEST" && key != "GOSQLITE_MANUAL_READONLY_WAL_MANIFEST_SHA256" {
				findings = append(findings, "effect named environment only")
			}
			environment[key]++
			return true
		})
	}
	for _, key := range []string{"GOSQLITE_MANUAL_READONLY_WAL", "GOSQLITE_MANUAL_READONLY_WAL_MANIFEST", "GOSQLITE_MANUAL_READONLY_WAL_MANIFEST_SHA256"} {
		if environment[key] != 1 {
			findings = append(findings, "effect named environment only exact count")
		}
	}
	calls := readOnlyWALManualCalls(functions)
	// Exact effect dispatcher locations/counts also reject function-value
	// indirection through the closed selector inventory below.
	expected := map[string]map[string]int{
		"manualReadOnlyWALNativeEnd":         {"unix.Close": 1, "r.Owners[].Rows.Close": 1, "r.Owners[].Tx.Commit": 1, "r.Owners[].Tx.Rollback": 1, "r.Owners[].DB.Close": 1},
		"manualReadOnlyWALNativeNext":        {"r.Owners[].Rows.Next": 1, "r.Owners[].Rows.Err": 1},
		"manualReadOnlyWALNativeRows":        {"manualReadOnlyWALNativeEnd": 1, "manualReadOnlyWALNativeNext": 2, "r.Owners[].DB.Query": 1, "r.Owners[].Tx.Query": 1},
		"manualReadOnlyWALNativeTransaction": {"manualReadOnlyWALNativeEnd": 2, "r.Owners[].DB.Begin": 1},
		"manualReadOnlyWALNativeCase":        {"sqlite.OpenContext": 1, "manualReadOnlyWALNativeReceipt": 2, "manualReadOnlyWALNativeEnd": 1},
		"manualReadOnlyWALNativeWrite":       {"unix.Write": 1, "unix.Fsync": 1},
		"manualReadOnlyWALNativeEventWrite":  {"os.Stdout.Write": 1},
		"manualReadOnlyWALNativeEvidence":    {"unix.Mkdirat": 1, "unix.Openat": 1, "unix.Openat2": 1},
		"manualReadOnlyWALNativeDirectory":   {"unix.Openat2": 1},
		"manualReadOnlyWALNativeRelative":    {"unix.Openat2": 1},
		"manualReadOnlyWALNativeProc":        {"unix.Open": 1, "manualReadOnlyWALNativeEnd": 1},
		"manualReadOnlyWALNativeFinish":      {"manualReadOnlyWALNativeEnd": 1},
		"manualReadOnlyWALNativeMetadata":    {"r.Owners[].DB.Schema": 1, "r.Owners[].DB.Inspect": 1, "r.Owners[].DB.Check": 1},
	}
	protected := make(map[string]bool)
	for _, inventory := range expected {
		for name := range inventory {
			protected[name] = true
		}
	}
	for fn, inventory := range expected {
		for name, count := range inventory {
			if calls[fn][name] != count {
				findings = append(findings, "ownership/effect exact terminal/acquisition inventory: "+fn+" "+name)
			}
		}
	}
	for fn, inventory := range calls {
		for name, count := range inventory {
			if protected[name] && expected[fn][name] != count {
				findings = append(findings, "ownership/effect escaped dispatcher or duplicate action: "+fn+" "+name)
			}
		}
	}
	selectorAllow := map[string]bool{}
	for _, name := range strings.Fields("os.Getenv os.Stdout.Write context.Background context.WithTimeout rand.Read sha256.New sha256.Sum256 binary.BigEndian.Uint16 hex.EncodeToString json.Marshal errors.New fmt.Sprintf filepath.Join filepath.Dir filepath.Base filepath.Clean runtime.LockOSThread runtime.UnlockOSThread debug.ReadBuildInfo slices.Equal slices.Sort strconv.ParseUint strings.Split strings.SplitN strings.Fields strings.Cut strings.HasPrefix strings.HasSuffix strings.TrimPrefix strings.TrimSuffix time.Parse time.Now time.Until sqlite.PendingCloseCleanups sqlite.PendingPlaintextCleanups sqlite.OpenContext unix.Openat2 unix.Open unix.Openat unix.Mkdirat unix.Read unix.Pread unix.Write unix.Fsync unix.Close unix.Statx unix.Fstat unix.Fstatfs unix.Faccessat2 unix.Getresuid unix.Getresgid unix.Getgroups unix.Capget unix.Seek unix.ReadDirent unix.ParseDirent unix.Poll h.Write h.Sum result.WriteByte result.String rows.Columns rows.Values r.Owners[].Rows.Close r.Owners[].Rows.Next r.Owners[].Rows.Err r.Owners[].Tx.Commit r.Owners[].Tx.Rollback r.Owners[].Tx.Query r.Owners[].DB.Close r.Owners[].DB.Query r.Owners[].DB.Begin r.Owners[].DB.Schema r.Owners[].DB.Inspect r.Owners[].DB.Check t.Fatal t.Error") {
		selectorAllow[name] = true
	}
	for fn, inventory := range calls {
		for name := range inventory {
			if strings.Contains(name, ".") && !selectorAllow[name] && name != "<dynamic>.Add" {
				findings = append(findings, "native effect call inventory outside closed allowlist: "+fn+" "+name)
			}
		}
	}
	// Immediate capture must remain adjacent to each raw acquisition assignment.
	for name, fn := range functions {
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			block, ok := node.(*ast.BlockStmt)
			if !ok {
				return true
			}
			for i, statement := range block.List {
				assignment, ok := statement.(*ast.AssignStmt)
				if !ok || len(assignment.Rhs) != 1 {
					continue
				}
				call, ok := assignment.Rhs[0].(*ast.CallExpr)
				if !ok {
					continue
				}
				callee := readOnlyWALManualName(call.Fun)
				capture := ""
				switch callee {
				case "unix.Open", "unix.Openat", "unix.Openat2":
					capture = "capture:=manualReadOnlyWALNativeCaptureFD(r,"
				case "sqlite.OpenContext":
					capture = "r.Owners[id].DB,r.Owners[id].Original=db,original"
				case "r.Owners[].DB.Begin":
					capture = "r.Owners[id].Tx,r.Owners[id].Original=tx,original"
				case "r.Owners[].DB.Schema", "r.Owners[].DB.Inspect", "r.Owners[].DB.Check":
					capture = "r.Owners[id].Original=original"
				}
				if capture != "" && (i+1 >= len(block.List) || !strings.HasPrefix(readOnlyWALManualCompact(block.List[i+1]), capture)) {
					findings = append(findings, "ownership immediate original capture-before-observation: "+name)
				}
			}
			return true
		})
	}
	findings = append(findings, readOnlyWALManualReceiptBoundary(functions)...)
	findings = append(findings, readOnlyWALManualAPIContext(file, functions)...)
	findings = append(findings, readOnlyWALManualProvenance(functions)...)
	findings = append(findings, readOnlyWALManualTerminalBinding(functions)...)
	findings = append(findings, readOnlyWALManualQueryCapture(functions)...)
	return findings
}

// Match direct, adjacent AST statements, not a substring hidden in a dead
// conditional. All semantic checks below remain independent of source pins.
func readOnlyWALManualSequence(block *ast.BlockStmt, parts ...string) int {
	if block == nil || len(parts) == 0 {
		return -1
	}
	found := -1
	for i := 0; i+len(parts) <= len(block.List); i++ {
		match := true
		for j, part := range parts {
			if readOnlyWALManualCompact(block.List[i+j]) != readOnlyWALManualCompactText(part) {
				match = false
				break
			}
		}
		if match {
			if found >= 0 {
				return -1
			}
			found = i
		}
	}
	return found
}
func readOnlyWALManualBody(functions map[string]*ast.FuncDecl, name string) *ast.BlockStmt {
	if fn := functions[name]; fn != nil {
		return fn.Body
	}
	return nil
}
func readOnlyWALManualCountCall(node ast.Node, name string) int {
	count := 0
	if node == nil {
		return count
	}
	if block, ok := node.(*ast.BlockStmt); ok && block == nil {
		return count
	}
	ast.Inspect(node, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok && readOnlyWALManualName(call.Fun) == name {
			count++
		}
		return true
	})
	return count
}
func readOnlyWALManualBoundCancel(call *ast.CallExpr, parents map[ast.Node]ast.Node, functions map[string]*ast.FuncDecl) bool {
	if readOnlyWALManualName(call.Fun) != "cancel" || len(call.Args) != 0 {
		return false
	}
	deferred, ok := parents[call].(*ast.DeferStmt)
	if !ok || deferred.Call != call {
		return false
	}
	body := readOnlyWALManualBody(functions, "manualReadOnlyWALNativeCase")
	if body == nil || parents[deferred] != body {
		return false
	}
	index := readOnlyWALManualSequence(body, `apiCtx, cancel := context.WithTimeout(context.Background(),30*time.Second)`, `defer cancel()`, `r.APICtx = apiCtx`)
	return index >= 0 && body.List[index+1] == deferred
}
func readOnlyWALManualReceiptBoundary(functions map[string]*ast.FuncDecl) []string {
	const diagnostic = "gate pre-receipt fixture content call closure"
	var findings []string
	report := func(detail string) { findings = append(findings, diagnostic+": "+detail) }
	prepared := readOnlyWALManualBody(functions, "manualReadOnlyWALNativeRunPrepared")
	boundary := readOnlyWALManualSequence(prepared, `if err := manualReadOnlyWALNativeCase(r,false); err != nil { return err }`, `if err := manualReadOnlyWALNativeCase(r,true); err != nil { return err }`)
	admission := readOnlyWALManualSequence(prepared, `admission, err := manualReadOnlyWALNativeMetadataNow(r)`, `if err != nil { return err }`)
	if boundary < 0 || admission < 0 || admission >= boundary {
		report("direct metadata-only admission and case boundary required")
	}
	caseBody := readOnlyWALManualBody(functions, "manualReadOnlyWALNativeCase")
	if caseBody == nil || len(caseBody.List) == 0 || readOnlyWALManualCompact(caseBody.List[0]) != readOnlyWALManualCompactText(`if err := manualReadOnlyWALNativeReceipt(r); err != nil { return err }`) {
		report("first case action must consume the fresh receipt")
	}
	snapshot := readOnlyWALManualBody(functions, "manualReadOnlyWALNativeSnapshotNow")
	if readOnlyWALManualSequence(snapshot, `var snapshot manualReadOnlyWALNativeSnapshot`, `if r.Phase.Next == 0 { return snapshot,manualReadOnlyWALNativeFail(r,"fixture content requires a fresh keeper receipt",nil) }`, `snapshot, err := manualReadOnlyWALNativeMetadataNow(r)`, `if err != nil { return snapshot,err }`) != 0 {
		report("post-receipt snapshot must start with checked metadata")
	}
	// Traverse every helper reachable from the root/admission prefix, including
	// error/cleanup branches and the receipt reader. The first case body is the
	// boundary, not an unchecked global exemption for SnapshotNow. Read is only
	// admitted at its two reviewed control/self-metadata call sites; raw Read
	// is admitted only inside that bounded reader or the status-only pipe reader.
	visited := make(map[string]bool)
	var visit func(string)
	visit = func(name string) {
		if visited[name] {
			return
		}
		visited[name] = true
		body := readOnlyWALManualBody(functions, name)
		if body == nil {
			report("missing reachable helper " + name)
			return
		}
		statements := body.List
		if name == "manualReadOnlyWALNativeRunPrepared" {
			if boundary < 0 {
				return
			}
			statements = statements[:boundary]
		}
		for _, statement := range statements {
			ast.Inspect(statement, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				callee := readOnlyWALManualName(call.Fun)
				switch callee {
				case "manualReadOnlyWALNativeSnapshotNow", "manualReadOnlyWALNativeHash", "manualReadOnlyWALNativeHeader", "manualReadOnlyWALNativeCase", "sqlite.OpenContext", "unix.Pread", "unix.Readv", "unix.Preadv", "unix.Mmap", "os.ReadFile":
					report(name + " reaches " + callee)
					return true
				case "manualReadOnlyWALNativeRead":
					want := ""
					if name == "manualReadOnlyWALNativeRunPrepared" {
						want = `manualReadOnlyWALNativeRead(r,manifestFile,32768)`
					}
					if name == "manualReadOnlyWALNativeProc" {
						want = `manualReadOnlyWALNativeRead(r,id,limit)`
					}
					if want == "" || readOnlyWALManualCompact(call) != readOnlyWALManualCompactText(want) {
						report("unbound control/self read in " + name)
					}
				case "unix.Read":
					want := ""
					if name == "manualReadOnlyWALNativeRead" {
						want = `unix.Read(r.Owners[id].FD,buffer)`
					}
					if name == "manualReadOnlyWALNativeReceipt" {
						want = `unix.Read(0,b[:])`
					}
					if want == "" || readOnlyWALManualCompact(call) != readOnlyWALManualCompactText(want) {
						report("unbound raw content read in " + name)
					}
				}
				if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
					switch selector.Sel.Name {
					case "Query", "Schema", "Inspect", "Check", "Begin":
						report("database API before receipt in " + name)
					}
				}
				if functions[callee] != nil {
					visit(callee)
				}
				return true
			})
		}
	}
	visit(readOnlyWALManualEntry)
	visit("manualReadOnlyWALNativeReceipt")
	// A metadata helper must remain content-free even if admission wiring is
	// itself broken; this independent root catches an indirect helper mutation.
	visit("manualReadOnlyWALNativeMetadataNow")
	return findings
}
func readOnlyWALManualAPIContext(file *ast.File, functions map[string]*ast.FuncDecl) []string {
	const diagnostic = "gate fresh fixed case API context"
	var findings []string
	report := func(detail string) { findings = append(findings, diagnostic+": "+detail) }
	body := readOnlyWALManualBody(functions, "manualReadOnlyWALNativeCase")
	setup := readOnlyWALManualSequence(body, `apiCtx, cancel := context.WithTimeout(context.Background(),30*time.Second)`, `defer cancel()`, `r.APICtx = apiCtx`)
	open := -1
	if body != nil {
		for i, statement := range body.List {
			if readOnlyWALManualCountCall(statement, "sqlite.OpenContext") != 0 {
				open = i
			}
		}
	}
	if setup < 1 || open <= setup+2 {
		report("unique direct timeout/defer/assignment must precede Open after receipt")
	}
	fields, assignments, setups, backgrounds, cancels := 0, 0, 0, 0, 0
	for _, declaration := range file.Decls {
		gen, ok := declaration.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range gen.Specs {
			typ, ok := spec.(*ast.TypeSpec)
			if !ok || typ.Name.Name != "manualReadOnlyWALNativeRun" {
				continue
			}
			structure, ok := typ.Type.(*ast.StructType)
			if !ok {
				continue
			}
			for _, field := range structure.Fields.List {
				for _, name := range field.Names {
					if name.Name == "APICtx" && readOnlyWALManualCompact(field.Type) == "context.Context" {
						fields++
					}
				}
			}
		}
	}
	for name, fn := range functions {
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			if assignment, ok := node.(*ast.AssignStmt); ok {
				for _, left := range assignment.Lhs {
					switch readOnlyWALManualCompact(left) {
					case "r.APICtx":
						assignments++
						if name != "manualReadOnlyWALNativeCase" || readOnlyWALManualCompact(assignment) != "r.APICtx=apiCtx" {
							report("case context rebound or substituted")
						}
					case "apiCtx", "cancel":
						if name != "manualReadOnlyWALNativeCase" || readOnlyWALManualCompact(assignment) != readOnlyWALManualCompactText(`apiCtx, cancel := context.WithTimeout(context.Background(),30*time.Second)`) {
							report("fresh context/cancel binding rewritten")
						}
					}
				}
			}
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			callee := readOnlyWALManualName(call.Fun)
			switch callee {
			case "context.WithTimeout":
				setups++
				if name != "manualReadOnlyWALNativeCase" {
					report("timeout outside case")
				}
			case "context.Background":
				backgrounds++
			case "cancel":
				cancels++
				if name != "manualReadOnlyWALNativeCase" {
					report("cancel outside case")
				}
			}
			contextAPI := callee == "sqlite.OpenContext"
			if selector, ok := call.Fun.(*ast.SelectorExpr); ok {
				switch selector.Sel.Name {
				case "Query", "Schema", "Inspect", "Check", "Begin", "Commit":
					contextAPI = true
				}
			}
			if contextAPI && (len(call.Args) == 0 || readOnlyWALManualCompact(call.Args[0]) != "r.APICtx") {
				report("API uses a different context: " + name + " " + callee)
			}
			return true
		})
	}
	if fields != 1 || assignments != 1 || setups != 1 || backgrounds != 1 || cancels != 1 {
		report("exact fresh context field/setup/assignment/cancellation count required")
	}
	return findings
}
func readOnlyWALManualTerminalBinding(functions map[string]*ast.FuncDecl) []string {
	const diagnostic = "ownership terminal receiver bound to ledger id"
	var findings []string
	report := func(detail string) { findings = append(findings, diagnostic+": "+detail) }
	end := readOnlyWALManualBody(functions, "manualReadOnlyWALNativeEnd")
	if readOnlyWALManualSequence(end,
		`if err := manualReadOnlyWALBeginEnd(&r.Ledger,id,action); err != nil { return manualReadOnlyWALNativeFail(r,"checked-once dependency barrier",err) }`,
		`var original error`,
		`switch r.Ledger.Owners[id].Kind {
		case "fd": original = unix.Close(r.Owners[id].FD)
		case "rows": original = r.Owners[id].Rows.Close()
		case "tx": if action == "commit" { original = r.Owners[id].Tx.Commit(r.APICtx) } else { original = r.Owners[id].Tx.Rollback() }
		case "db": original = r.Owners[id].DB.Close()
		default: r.Ledger.Blocked = true; return manualReadOnlyWALNativeFail(r,"unknown terminal dispatcher",nil)
		}`,
		`r.Owners[id].Terminal = original`,
		`decision := manualReadOnlyWALFinishEnd(&r.Ledger,id,original != nil)`) != 0 {
		report("dispatcher transition, actual receiver, result capture or finish binding changed")
	}
	next := readOnlyWALManualBody(functions, "manualReadOnlyWALNativeNext")
	if readOnlyWALManualSequence(next,
		`if err := manualReadOnlyWALBeginEnd(&r.Ledger,id,"next"); err != nil { return false,manualReadOnlyWALNativeFail(r,"Next begin",err) }`,
		`next := r.Owners[id].Rows.Next()`, `var original error`,
		`if !next { original = r.Owners[id].Rows.Err() }`,
		`r.Owners[id].Terminal = original`,
		`decision := manualReadOnlyWALFinishNext(&r.Ledger,id,next,original != nil)`) != 0 {
		report("Next/Err receiver or implicit terminal transition changed")
	}
	// Do not use normalized Owners[] names for receiver authorization. This
	// second pass diagnoses wrong/unknown indexes even when call counts match.
	for name, fn := range functions {
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			callee := readOnlyWALManualName(call.Fun)
			if callee == "unix.Close" {
				if name != "manualReadOnlyWALNativeEnd" || len(call.Args) != 1 || readOnlyWALManualCompact(call.Args[0]) != "r.Owners[id].FD" {
					report("raw Close owner mismatch in " + name)
				}
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			kind := ""
			switch selector.Sel.Name {
			case "Close":
				kind = "end"
			case "Commit", "Rollback":
				kind = "tx"
			case "Next", "Err":
				kind = "rows"
			}
			if kind == "" {
				return true
			}
			receiver := readOnlyWALManualCompact(selector.X)
			valid := name == "manualReadOnlyWALNativeEnd" && (kind == "end" && (receiver == "r.Owners[id].Rows" || receiver == "r.Owners[id].DB") || kind == "tx" && receiver == "r.Owners[id].Tx") || name == "manualReadOnlyWALNativeNext" && kind == "rows" && receiver == "r.Owners[id].Rows"
			if !valid {
				report("unknown or mismatched terminal receiver in " + name)
			}
			return true
		})
	}
	return findings
}
func readOnlyWALManualQueryCapture(functions map[string]*ast.FuncDecl) []string {
	body := readOnlyWALManualBody(functions, "manualReadOnlyWALNativeRows")
	sequence := readOnlyWALManualSequence(body,
		`if r.Ledger.Owners[parent].Kind == "db" { rows,original = r.Owners[parent].DB.Query(r.APICtx,manualReadOnlyWALQuery) } else { rows,original = r.Owners[parent].Tx.Query(r.APICtx,manualReadOnlyWALQuery) }`,
		`r.Owners[id].Rows, r.Owners[id].Original = rows,original`,
		`capture := manualReadOnlyWALAdopt(&r.Ledger,id,rows != nil,original != nil,original != nil)`,
		`if original != nil || capture != nil { return manualReadOnlyWALNativeFail(r,"query acquisition ownership",original) }`)
	if sequence < 0 || readOnlyWALManualCountCall(body, "r.Owners[].DB.Query") != 1 || readOnlyWALManualCountCall(body, "r.Owners[].Tx.Query") != 1 {
		return []string{"ownership Query branch immediate capture-before-observation: both branches must only acquire and immediately retain the same owner/error"}
	}
	return nil
}
func readOnlyWALManualProvenance(functions map[string]*ast.FuncDecl) []string {
	var findings []string
	proc := readOnlyWALManualBody(functions, "manualReadOnlyWALNativeProc")
	if readOnlyWALManualSequence(proc,
		`if path != "/proc/thread-self/status" && path != "/proc/self/mountinfo" { return nil,manualReadOnlyWALNativeFail(r,"unapproved self metadata",nil) }`,
		`id, err := manualReadOnlyWALNativeReserve(r,-1,"fd","bounded-self-metadata")`, `if err != nil { return nil,err }`,
		`fd, original := unix.Open(path,unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NONBLOCK,0)`,
		`capture := manualReadOnlyWALNativeCaptureFD(r,id,fd,original)`,
		`if original != nil || capture != nil { return nil,manualReadOnlyWALNativeFail(r,"self metadata acquisition",original) }`,
		`var fs unix.Statfs_t`,
		`if err := unix.Fstatfs(r.Owners[id].FD,&fs); err != nil { return nil,manualReadOnlyWALNativeFail(r,"self metadata filesystem provenance",err) }`,
		`if fs.Type != unix.PROC_SUPER_MAGIC { return nil,manualReadOnlyWALNativeFail(r,"self metadata is not procfs",nil) }`,
		`raw, err := manualReadOnlyWALNativeRead(r,id,limit)`, `if err != nil { return nil,err }`) != 0 {
		findings = append(findings, "profile retained self-proc provenance before read")
	}
	const diagnostic = "profile control pathname and backing provenance"
	report := func(detail string) { findings = append(findings, diagnostic+": "+detail) }
	control := readOnlyWALManualBody(functions, "manualReadOnlyWALNativeControlMatches")
	if readOnlyWALManualSequence(control,
		`control, err := manualReadOnlyWALNativeStatFD(r,r.Control)`, `if err != nil { return err }`,
		`manifest, err := manualReadOnlyWALNativeStatFD(r,r.ManifestFile)`, `if err != nil { return err }`,
		`if control != r.ControlStat || manifest != r.ManifestStat { return manualReadOnlyWALNativeFail(r,"control metadata changed",nil) }`,
		`if err := manualReadOnlyWALNativePathMatches(r,unix.AT_FDCWD,r.ControlPath,control); err != nil { return err }`,
		`if err := manualReadOnlyWALNativePathMatches(r,r.Owners[r.Control].FD,r.ManifestName,manifest); err != nil { return err }`,
		`return nil`) != 0 {
		report("retained control/manifest and both pathname identities must be checked directly")
	}
	prepared := readOnlyWALManualBody(functions, "manualReadOnlyWALNativeRunPrepared")
	if readOnlyWALManualSequence(prepared, `r.ControlPath,r.ManifestName = filepath.Dir(manifestPath),filepath.Base(manifestPath)`, `if !manualReadOnlyWALPath(r.ControlPath) { return manualReadOnlyWALNativeFail(r,"nonroot control directory required",nil) }`) < 0 ||
		readOnlyWALManualSequence(prepared,
			`control, err := manualReadOnlyWALNativeDirectory(r,r.ControlPath,"control-directory")`, `r.Control = control`, `if err != nil { return err }`,
			`manifestFile, err := manualReadOnlyWALNativeRelative(r,control,r.ManifestName,"control-manifest",false)`, `r.ManifestFile = manifestFile`, `if err != nil { return err }`,
			`stat, err := manualReadOnlyWALNativeStatFD(r,manifestFile)`, `if err != nil { return err }`,
			`if stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Links != 1 || stat.Size > 32768 { return manualReadOnlyWALNativeFail(r,"manifest regular bounded file",nil) }`,
			`controlStat, err := manualReadOnlyWALNativeStatFD(r,control)`, `if err != nil { return err }`,
			`if controlStat.Mode&unix.S_IFMT != unix.S_IFDIR { return manualReadOnlyWALNativeFail(r,"control directory type",nil) }`,
			`r.ControlStat,r.ManifestStat = controlStat,stat`,
			`if err := manualReadOnlyWALNativeControlMatches(r); err != nil { return err }`,
			`raw, err := manualReadOnlyWALNativeRead(r,manifestFile,32768)`, `if err != nil { return err }`,
			`if err := manualReadOnlyWALNativeControlMatches(r); err != nil { return err }`,
			`if fmt.Sprintf("%x",sha256.Sum256(raw)) != manifestDigest { return manualReadOnlyWALNativeFail(r,"reviewed manifest digest",nil) }`) < 0 {
		report("acquisition/binding must bracket manifest consumption before decode")
	}
	if readOnlyWALManualSequence(prepared, `if !manualReadOnlyWALSeparate(m.SourceRoot,r.ControlPath) || !manualReadOnlyWALSeparate(m.EvidenceParent,r.ControlPath) || stat.UID != m.ReaderUID { return manualReadOnlyWALNativeFail(r,"separate owned control input",nil) }`) < 0 {
		report("absolute control directory separation required")
	}
	metadata := readOnlyWALManualBody(functions, "manualReadOnlyWALNativeMetadataNow")
	if readOnlyWALManualSequence(metadata,
		`if err := manualReadOnlyWALNativeControlMatches(r); err != nil { return snapshot,err }`,
		`control, controlStat, err := manualReadOnlyWALNativeObserveObject(r,r.Control,true,mounts)`, `if err != nil { return snapshot,err }`, `snapshot.Control = control`,
		`if controlStat != r.ControlStat || control.Mode&unix.S_IFMT != unix.S_IFDIR || !manualReadOnlyWALMountValid(control.Mount,control.Identity,control.Mount.ReadOnly) { return snapshot,manualReadOnlyWALNativeFail(r,"control directory mount provenance",nil) }`,
		`source := snapshot.Facts.Source[0]`,
		`sourcePlace, err := manualReadOnlyWALNativeBackingPlace(r.Manifest.SourceRoot,source.Mount)`, `if err != nil { return snapshot,manualReadOnlyWALNativeFail(r,"source mount-root provenance",err) }`,
		`evidencePlace, err := manualReadOnlyWALNativeBackingPlace(r.Manifest.EvidenceParent,evidence.Mount)`, `if err != nil { return snapshot,manualReadOnlyWALNativeFail(r,"evidence mount-root provenance",err) }`,
		`controlPlace, err := manualReadOnlyWALNativeBackingPlace(r.ControlPath,control.Mount)`, `if err != nil { return snapshot,manualReadOnlyWALNativeFail(r,"control mount-root provenance",err) }`,
		`separateBacking := source.Identity.Major != evidence.Identity.Major || source.Identity.Minor != evidence.Identity.Minor || sourcePlace != "/" && evidencePlace != "/" && manualReadOnlyWALSeparate(sourcePlace,evidencePlace)`,
		`separateControlSource := control.Identity.Major != source.Identity.Major || control.Identity.Minor != source.Identity.Minor || controlPlace != "/" && sourcePlace != "/" && manualReadOnlyWALSeparate(controlPlace,sourcePlace)`,
		`separateControlEvidence := control.Identity.Major != evidence.Identity.Major || control.Identity.Minor != evidence.Identity.Minor || controlPlace != "/" && evidencePlace != "/" && manualReadOnlyWALSeparate(controlPlace,evidencePlace)`,
		`snapshot.Facts.DistinctLocations = source.Identity != evidence.Identity && control.Identity != source.Identity && control.Identity != evidence.Identity && separateBacking && separateControlSource && separateControlEvidence`,
		`if err := manualReadOnlyWALValidateProfile(snapshot.Facts,r.Manifest); err != nil { return snapshot,manualReadOnlyWALNativeFail(r,"actual Linux profile refused",err) }`) < 0 {
		report("observed control mount, pairwise backing places and fail-closed profile required")
	}
	if readOnlyWALManualCountCall(prepared, "manualReadOnlyWALNativeControlMatches") != 2 || readOnlyWALManualCountCall(metadata, "manualReadOnlyWALNativeControlMatches") != 1 || readOnlyWALManualSequence(readOnlyWALManualBody(functions, "manualReadOnlyWALNativeSameSnapshot"), `return a.Stats == b.Stats && a.Hashes == b.Hashes && a.Facts == b.Facts && a.Control == b.Control && slices.Equal(a.Groups,b.Groups)`) != 0 {
		report("control reobservation cannot be dropped")
	}
	return findings
}
func readOnlyWALManualPreviousWiring(mainBody, walkBody *ast.BlockStmt) []string {
	const diagnostic = "readonly WAL manual previous guard wiring weakened"
	var findings []string
	report := func(name string) {
		findings = append(findings, diagnostic+": direct placement and checked findings/error propagation required for "+name)
	}
	rootNegative := readOnlyWALManualSequence(walkBody, `rootNegative, rootNegativeErr := inspectEncryptedRootNegativeSource(path,raw)`, `if rootNegativeErr != nil { return rootNegativeErr }`, `findings = append(findings,rootNegative...)`)
	if rootNegative < 0 {
		report("encrypted root negative source")
	}
	formatUpgrade := readOnlyWALManualSequence(walkBody, `formatUpgrade, formatErr := inspectFormatUpgradeSource(path,raw)`, `if formatErr != nil { return formatErr }`, `findings = append(findings,formatUpgrade...)`)
	if formatUpgrade < 0 {
		report("format upgrade source")
	}
	if readOnlyWALManualSequence(walkBody, `manual, err := inspectBackupManualSource(path,raw)`, `if err != nil { return err }`, `findings = append(findings,manual...)`) < 0 {
		report("Backup source")
	}
	rollbackFault := readOnlyWALManualSequence(walkBody, `rollbackFault, err := inspectRollbackFaultSource(path,raw)`, `if err != nil { return err }`, `findings = append(findings,rollbackFault...)`)
	if rollbackFault < 0 {
		report("rollback fault source")
	}
	rollbackSQL := readOnlyWALManualSequence(walkBody, `rollbackSQL, rollbackSQLErr := inspectRollbackFaultSQLSource(path,raw)`, `if rollbackSQLErr != nil { return rollbackSQLErr }`, `findings = append(findings,rollbackSQL...)`)
	if rollbackSQL < 0 {
		report("rollback SQL source")
	}
	restart := readOnlyWALManualSequence(walkBody, `restart, restartErr := inspectRollbackRestartSource(path,raw)`, `if restartErr != nil { return restartErr }`, `findings = append(findings,restart...)`)
	if restart < 0 {
		report("rollback restart source")
	}
	recovery := readOnlyWALManualSequence(walkBody, `recovery, err := inspectRollbackFaultRecoverySource(path,raw)`, `if err != nil { return err }`, `findings = append(findings,recovery...)`)
	if recovery < 0 {
		report("rollback fault recovery source")
	}
	ordinary, oracle := -1, -1
	if walkBody != nil {
		for i, statement := range walkBody.List {
			conditional, ok := statement.(*ast.IfStmt)
			if !ok || conditional.Init != nil || conditional.Else != nil {
				continue
			}
			switch readOnlyWALManualCompact(conditional.Cond) {
			case `strings.HasSuffix(path,"_test.go")`:
				if ordinary >= 0 || readOnlyWALManualSequence(conditional.Body, `ordinary, inspectErr := inspectSource(path,raw)`, `if inspectErr != nil { return inspectErr }`, `findings = append(findings,ordinary...)`) != 0 || len(conditional.Body.List) != 3 {
					report("ordinary source")
				}
				ordinary = i
			case "oracleCurrentGoPath(path)":
				if oracle >= 0 || readOnlyWALManualSequence(conditional.Body,
					`publication, err := inspectWALPublicationSource(path,raw)`, `if err != nil { return err }`, `findings = append(findings,publication...)`,
					`startup, err := inspectOracleSource(path,raw)`, `if err != nil { return err }`, `findings = append(findings,startup...)`) != 0 || len(conditional.Body.List) != 6 {
					report("WAL publication/oracle source")
				}
				oracle = i
			}
		}
	}
	backup := readOnlyWALManualSequence(walkBody, `manual, err := inspectBackupManualSource(path,raw)`, `if err != nil { return err }`, `findings = append(findings,manual...)`)
	readonly := readOnlyWALManualSequence(walkBody, `readonlyManual, err := inspectReadOnlyWALManualSource(path,raw)`, `if err != nil { return err }`, `findings = append(findings,readonlyManual...)`)
	if rootNegative < 0 || formatUpgrade <= rootNegative || backup <= formatUpgrade || rollbackFault <= backup || rollbackSQL <= rollbackFault || restart <= rollbackSQL || recovery <= restart || readonly <= recovery || ordinary <= readonly || oracle <= ordinary {
		report("earlier source filter order")
	}
	last, first := -1, -1
	for _, contract := range []struct{ name, result, err string }{
		{"inspectOracleContracts", "contracts", "err"},
		{"inspectBackupManualContracts", "manualContracts", "manualErr"},
		{"inspectRollbackFaultContracts", "rollbackFaultContracts", "rollbackFaultErr"},
		{"inspectRollbackFaultSQLContracts", "rollbackSQLContracts", "rollbackSQLContractErr"},
		{"inspectRollbackRestartContracts", "restartContracts", "restartContractErr"},
		{"inspectRollbackFaultRecoveryContracts", "rollbackRecoveryContracts", "rollbackRecoveryErr"},
		{"inspectWALPublicationContracts", "publicationContracts", "err"},
		{"inspectDarwinInventoryManualContract", "darwinManual", "err"},
		{"inspectFormatUpgradeContracts", "formatContracts", "formatErr"},
		{"inspectReadOnlyWALManualContracts", "readonlyContracts", "err"},
		{"inspectEncryptedRootNegativeContracts", "rootNegativeContracts", "rootNegativeErr"},
	} {
		index := readOnlyWALManualSequence(mainBody,
			contract.result+", "+contract.err+" := "+contract.name+`(".")`,
			"if "+contract.err+" != nil { fmt.Fprintln(os.Stderr,"+contract.err+"); failed = true }",
			"for _, finding := range "+contract.result+" { fmt.Fprintln(os.Stderr,finding); failed = true }")
		if index < 0 || index <= last {
			report(contract.name)
		}
		if first < 0 {
			first = index
		}
		last = index
	}
	// The traversal's own error and findings must reach the final failure bit.
	walk := -1
	if mainBody != nil {
		for i, statement := range mainBody.List {
			assignment, ok := statement.(*ast.AssignStmt)
			if !ok || len(assignment.Lhs) != 1 || readOnlyWALManualCompact(assignment.Lhs[0]) != "err" || len(assignment.Rhs) != 1 {
				continue
			}
			call, ok := assignment.Rhs[0].(*ast.CallExpr)
			if ok && readOnlyWALManualName(call.Fun) == "filepath.WalkDir" {
				walk = i
			}
		}
	}
	if walk < 0 || first <= walk+1 || mainBody == nil || walk+1 >= len(mainBody.List) || readOnlyWALManualCompact(mainBody.List[walk+1]) != readOnlyWALManualCompactText(`if err != nil { fmt.Fprintln(os.Stderr,err); failed = true }`) {
		report("whole traversal error")
	}
	output := readOnlyWALManualSequence(walkBody, `for _, f := range findings { fmt.Fprintln(os.Stderr,f); failed = true }`, `return nil`)
	if walkBody == nil || output < 0 || output <= oracle || output != len(walkBody.List)-2 || readOnlyWALManualSequence(mainBody, `if failed { os.Exit(1) }`) <= last {
		report("all findings final failure")
	}
	// A correctly placed append/error block is insufficient if later code
	// silently resets its accumulator. Preserve the actual result state too.
	initializations := 0
	// Only exact encrypted-root, combined, SQL, restart and recovery triplets extend the append inventory.
	// Matching append text alone never grants a helper or placement exemption.
	allowedAppends := map[string]int{
		"findings=append(findings,rootNegative...)":   0,
		"findings=append(findings,formatUpgrade...)":  0,
		"findings=append(findings,rollbackFault...)":  0,
		"findings=append(findings,rollbackSQL...)":    0,
		"findings=append(findings,restart...)":        0,
		"findings=append(findings,recovery...)":       0,
		"findings=append(findings,manual...)":         0,
		"findings=append(findings,readonlyManual...)": 0,
		"findings=append(findings,ordinary...)":       0,
		"findings=append(findings,publication...)":    0,
		"findings=append(findings,startup...)":        0,
	}
	if mainBody != nil {
		ast.Inspect(mainBody, func(node ast.Node) bool {
			if unary, ok := node.(*ast.UnaryExpr); ok && unary.Op == token.AND && (readOnlyWALManualName(unary.X) == "findings" || readOnlyWALManualName(unary.X) == "failed") {
				report("result-state address escape")
			}
			assignment, ok := node.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for _, left := range assignment.Lhs {
				name := readOnlyWALManualName(left)
				if name == "findings" || strings.HasPrefix(name, "findings[") {
					compact := readOnlyWALManualCompact(assignment)
					if _, allowed := allowedAppends[compact]; !allowed {
						report("findings reset/rebinding")
					} else {
						allowedAppends[compact]++
					}
				}
				if name == "failed" || strings.HasPrefix(name, "failed[") {
					compact := readOnlyWALManualCompact(assignment)
					if compact == "failed:=false" && len(mainBody.List) > 1 && mainBody.List[1] == assignment {
						initializations++
					} else if compact != "failed=true" {
						report("failure-state reset/rebinding")
					}
				}
			}
			return true
		})
	}
	if initializations != 1 {
		report("failure initialization")
	}
	for name, count := range allowedAppends {
		if count != 1 {
			report("result append inventory " + name)
		}
	}
	return findings
}
func readOnlyWALManualPureSemantics(file *ast.File, functions map[string]*ast.FuncDecl) []string {
	var findings []string
	require := func(name, diagnostic string, parts ...string) {
		findings = append(findings, readOnlyWALManualRequire(functions, name, diagnostic, parts...)...)
	}
	require("manualReadOnlyWALJSONValue", "pure strict duplicate/budget decoder", `depth > 8 || *tokens >= 512`, `if _, exists := m[key]; exists { return nil,errors.New("duplicate JSON key") }`, `strconv.ParseUint(string(x),10,64)`)
	require("manualReadOnlyWALDecodeObject", "pure exact bytes/UTF8/trailing decoder", `len(raw) > limit || !utf8.Valid(raw)`, `d.UseNumber()`, `strings.TrimSpace(string(raw[d.InputOffset():])) != ""`)
	require("manualReadOnlyWALKeys", "pure exact case-sensitive keys", `len(object) != len(expected)`, `if _, ok := object[key]; !ok { return false }`)
	require("manualReadOnlyWALDecodeManifest", "pure closed manifest shape", `manualReadOnlyWALDecodeObject(raw,32768)`, `manualReadOnlyWALKeys(o,"version run_id profile fixture_profile source_root evidence_parent reader_uid reader_gid source_tree native_source_sha256 portable_source_sha256 guard_receipt_sha256 binary_sha256 source_identity family keeper")`, `manualReadOnlyWALKeys(keeper,"session generation contract")`, `len(family) != 3`, `json.Unmarshal(raw,&m)`)
	require("manualReadOnlyWALDecodeReceipt", "pure closed receipt shape", `manualReadOnlyWALDecodeObject(raw,2048)`, `manualReadOnlyWALKeys(o,"version run_id phase challenge session generation status")`)
	require("manualReadOnlyWALValidateProfile", "pure observed profile fail-closed", `!c.Observed || !c.GroupsMatch || !c.ProcMatches`, `c.UID[i] != m.ReaderUID || c.GID[i] != m.ReaderGID`, `c.Effective != [2]uint32{} || c.Permitted != [2]uint32{} || c.Inheritable != [2]uint32{} || c.Ambient != 0`, `!object.Observed || !object.AccessObserved || !object.ReadAllowed`, `object.WriteResult != "EACCES" && !(readonly && object.WriteResult == "EROFS")`, `e.Mode != 0040700`)
	require("manualReadOnlyWALAdvancePhase", "pure exact bounded keeper transitions", `s.Next < 0 || s.Next >= len(phases)`, `r.Phase != phases[s.Next]`, `r.Challenge != s.Challenge`, `s.Used[i] == r.Challenge`)
	require("manualReadOnlyWALBeginEnd", "pure ownership dependency/once decision", `o.State != manualReadOnlyWALLive`, `child.Parent == id && child.State != manualReadOnlyWALEnded`, `o.State,o.Action = manualReadOnlyWALConsuming,action`)
	require("manualReadOnlyWALFinishEnd", "pure ownership failed terminal barrier", `o.State = manualReadOnlyWALUnknown; l.Blocked = true`, `o.State = manualReadOnlyWALEnded`)
	require("manualReadOnlyWALFinishNext", "pure ownership Next restore-or-terminal", `if next { o.State,o.Action = manualReadOnlyWALLive,"" } else { o.State = manualReadOnlyWALEnded }`)
	queryFound := false
	for _, declaration := range file.Decls {
		gen, ok := declaration.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			for i, name := range value.Names {
				if name.Name != "manualReadOnlyWALQuery" {
					continue
				}
				if i < len(value.Values) {
					literal, ok := value.Values[i].(*ast.BasicLit)
					if ok {
						text, err := strconv.Unquote(literal.Value)
						queryFound = err == nil && text == "SELECT id,text FROM t ORDER BY id"
					}
				}
			}
		}
	}
	if !queryFound {
		findings = append(findings, "effect sole fixed SELECT literal")
	}
	allowed := map[string]bool{}
	for _, name := range strings.Fields("json.NewDecoder json.Unmarshal json.Marshal strings.NewReader strings.HasPrefix strings.HasSuffix strings.Contains strings.Split strings.Fields strings.TrimSpace strings.Repeat strings.Replace strconv.ParseUint strconv.Itoa utf8.Valid errors.New d.Token d.More d.UseNumber d.InputOffset t.Fatal t.Fatalf") {
		allowed[name] = true
	}
	calls := readOnlyWALManualCalls(functions)
	for fn, inventory := range calls {
		for name := range inventory {
			if strings.Contains(name, ".") && name != "json.Delim" && !allowed[name] {
				findings = append(findings, "pure closure call outside decoder/comparison allowlist: "+fn+" "+name)
			}
		}
	}
	return findings
}
func inspectReadOnlyWALManualWiring(raw []byte) []string {
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, "internal/testscope/main.go", raw, 0)
	if err != nil {
		return []string{"readonly WAL manual wiring parse failure"}
	}
	functions := readOnlyWALManualFunctions(file)
	var findings []string
	mainFn := functions["main"]
	if mainFn == nil || mainFn.Body == nil {
		return []string{"readonly WAL manual dead scanner wiring"}
	}
	parents := oracleParents(file)
	var walkBody *ast.BlockStmt
	ast.Inspect(mainFn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || readOnlyWALManualName(call.Fun) != "filepath.WalkDir" || len(call.Args) != 2 {
			return true
		}
		if callback, ok := call.Args[1].(*ast.FuncLit); ok {
			walkBody = callback.Body
		}
		return true
	})
	placement := map[string]int{}
	ast.Inspect(mainFn.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		name := readOnlyWALManualName(call.Fun)
		if name != "inspectReadOnlyWALManualSource" && name != "inspectReadOnlyWALManualContracts" {
			return true
		}
		assignment, ok := parents[call].(*ast.AssignStmt)
		if !ok {
			return true
		}
		block, ok := parents[assignment].(*ast.BlockStmt)
		if !ok {
			return true
		}
		if name == "inspectReadOnlyWALManualSource" && block == walkBody || name == "inspectReadOnlyWALManualContracts" && block == mainFn.Body {
			placement[name]++
		}
		return true
	})
	if walkBody == nil || placement["inspectReadOnlyWALManualSource"] != 1 || placement["inspectReadOnlyWALManualContracts"] != 1 {
		findings = append(findings, "readonly WAL manual dead scanner wiring: direct all-file/root placement required")
	}
	body := readOnlyWALManualCompact(mainFn.Body)
	for _, part := range []string{`readonlyManual, err := inspectReadOnlyWALManualSource(path,raw); if err != nil { return err }; findings = append(findings,readonlyManual...)`, `readonlyContracts, err := inspectReadOnlyWALManualContracts(".")`, `for _, finding := range readonlyContracts { fmt.Fprintln(os.Stderr,finding); failed = true }`} {
		if !strings.Contains(body, readOnlyWALManualCompactText(part)) {
			findings = append(findings, "readonly WAL manual dead scanner wiring")
		}
	}
	perFile := strings.Index(body, "readonlyManual,err:=inspectReadOnlyWALManualSource(path,raw)")
	ordinaryFilter := strings.Index(body, `ifstrings.HasSuffix(path,"_test.go")`)
	oracleFilter := strings.Index(body, "iforacleCurrentGoPath(path)")
	if perFile < 0 || ordinaryFilter < 0 || oracleFilter < 0 || perFile > ordinaryFilter || perFile > oracleFilter {
		findings = append(findings, "readonly WAL manual traversal must precede platform/test filters")
	}
	for _, call := range []string{"inspectEncryptedRootNegativeSource(path,raw)", "inspectFormatUpgradeSource(path,raw)", "inspectBackupManualSource(path,raw)", "inspectRollbackFaultSource(path,raw)", "inspectRollbackFaultSQLSource(path,raw)", "inspectRollbackRestartSource(path,raw)", "inspectRollbackFaultRecoverySource(path,raw)", "inspectSource(path,raw)", "inspectWALPublicationSource(path,raw)", "inspectOracleSource(path,raw)", `inspectOracleContracts(".")`, `inspectBackupManualContracts(".")`, `inspectRollbackFaultContracts(".")`, `inspectRollbackFaultSQLContracts(".")`, `inspectRollbackRestartContracts(".")`, `inspectRollbackFaultRecoveryContracts(".")`, `inspectWALPublicationContracts(".")`, `inspectDarwinInventoryManualContract(".")`, `inspectFormatUpgradeContracts(".")`, `inspectEncryptedRootNegativeContracts(".")`} {
		if strings.Count(body, call) != 1 {
			findings = append(findings, "readonly WAL manual previous guard wiring weakened")
		}
	}
	findings = append(findings, readOnlyWALManualPreviousWiring(mainFn.Body, walkBody)...)
	ordinary := functions["inspectSource"]
	if ordinary == nil || ordinary.Body == nil || len(ordinary.Body.List) < 2 {
		findings = append(findings, "readonly WAL manual previous ordinary policy missing")
	} else {
		start, end := set.Position(ordinary.Pos()).Offset, set.Position(ordinary.End()).Offset
		if start < 0 || end > len(raw) || fmt.Sprintf("%x", sha256.Sum256(raw[start:end])) != readOnlyWALManualPriorOrdinarySHA256 {
			findings = append(findings, "readonly WAL manual previous ordinary policy body changed")
		}
		gate := readOnlyWALManualCompact(ordinary.Body.List[1])
		if gate != readOnlyWALManualCompactText(`if first == "//go:build manual_recovery && linux" || first == "//go:build manual_namespace" { return nil,nil }`) {
			findings = append(findings, "readonly WAL manual previous ordinary policy/exemptions weakened")
		}
	}
	return findings
}
func inspectReadOnlyWALManualContracts(root string) ([]string, error) {
	var findings []string
	found := make(map[string]bool)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		observed, err := inspectReadOnlyWALManualSource(relative, raw)
		if err != nil {
			return err
		}
		findings = append(findings, observed...)
		found[relative] = true
		return nil
	})
	if err != nil {
		return findings, err
	}
	for _, path := range []string{readOnlyWALManualNativePath, readOnlyWALManualPurePath, readOnlyWALManualGuardPath, readOnlyWALManualGuardTestPath, "internal/testscope/main.go"} {
		if !found[path] {
			findings = append(findings, "readonly WAL manual required closure file missing: "+path)
		}
	}
	return findings, nil
}
