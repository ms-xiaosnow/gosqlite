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

const backupManualHelper = "internal/testkit/backupmanual/manual_linux.go"
const backupManifestSource = "internal/testkit/backupmanual/manifest_schema.go"
const backupManifestTests = "internal/testkit/backupmanual/manifest_schema_test.go"
const backupManualImport = "github.com/ms-xiaosnow/gosqlite/internal/testkit/backupmanual"

// These whole-source pins are deliberately stricter than a general control-flow
// approximation: every actual gate, effect, plan, assertion and end ordering is
// fixed. An unreviewed harmless refactor is rejected as well. The pins live
// outside the six files, so no self-referential digest is involved. AST checks
// independently constrain the exact entry/import/effect inventories. None is a
// proof about imported behavior, the toolchain, native storage or authorization.
var backupManualPins = map[string]string{
	"sqlite/backup_test.go": "041773165730cd2840d657ae8ac996761f20ea08cf3ba11db81d0b2df5e586f7",
	"sqlite/backup_oracle_test.go": "7fbae19a215cd6e315d1eef7ebba08b3425a150ede502293c1ba16b5b91d3d41",
	"cmd/gosqlite/plain_backup_linux_test.go": "84318877e24af06b1a70770cde2f8223a7828d73435da93e0076e4bd3355f9a5",
	backupManualHelper: "2be71c35c6be3c8e1004e1a2d5dec2373a71f8ea6577cce1411b908c54afb48f",
	backupManifestSource: "ca34656116664a3af317bf98979574bee0e7042c60b89bfd20f5a36dd840110d",
	backupManifestTests: "d9c7f0f9227619e9056fa00e357301d6751c01d1a2ebe146a07f35ef94cb2c68",
}
var backupManualTags = map[string]string{
	"sqlite/backup_test.go": "//go:build linux && manual_backup_publication",
	"sqlite/backup_oracle_test.go": "//go:build oracle && linux && manual_backup_publication",
	"cmd/gosqlite/plain_backup_linux_test.go": "//go:build linux && manual_backup_publication",
	backupManualHelper: "//go:build linux && manual_backup_publication",
}
// Direct selector-call counts cover the actual native and oracle boundaries.
// Whole-source pins also freeze argument values and surrounding control flow.
var backupManualEffectCounts = map[string]map[string]int{
	"sqlite/backup_test.go": {"Backup":6, "Begin":2, "Check":5, "Exec":1, "ForTest":4, "Next":1, "Open":1, "Query":1, "Run":1},
	"sqlite/backup_oracle_test.go": {"Backup":1, "Check":1, "ForTest":2, "Run":2},
	"cmd/gosqlite/plain_backup_linux_test.go": {"Exec":1, "ForTest":1, "Open":1},
	backupManualHelper: {"Close":3, "Commit":1, "Fstat":5, "Fstatfs":1, "Lstat":3, "Mkdir":2, "Open":2, "Read":2, "Statx":1, "Write":4},
}
var backupManualEffectNames = map[string]bool{
	"Open":true,"Close":true,"Read":true,"Write":true,"Mkdir":true,
	"Lstat":true,"Fstat":true,"Fstatfs":true,"Statx":true,
	"ForTest":true,"Backup":true,"Begin":true,"Commit":true,"Next":true,
	"Query":true,"Exec":true,"Check":true,"Run":true,
}
var backupManualEntries = map[string]string{
	"TestPlaintextBackupOrdinaryRoundTrip": "sqlite/backup_test.go",
	"TestPlaintextBackupExistingDestinationPreflight": "sqlite/backup_test.go",
	"TestPlaintextBackupPinnedReadOnlyAndConcurrentReaders": "sqlite/backup_test.go",
	"TestPlaintextBackupEmptyAndLazyReadOnly": "sqlite/backup_test.go",
	"TestCLIPlainBackupOrdinaryRoundTrip": "cmd/gosqlite/plain_backup_linux_test.go",
	"TestPlaintextBackupOracleOrdinaryInteroperability": "sqlite/backup_oracle_test.go",
}

func backupManualPath(path string) string { return filepath.ToSlash(filepath.Clean(path)) }
func backupManualOwned(path string) bool { _, ok := backupManualPins[backupManualPath(path)]; return ok }

func inspectBackupManualSource(path string, raw []byte) ([]string, error) {
	path = backupManualPath(path)
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, path, raw, 0)
	if err != nil { return nil, err }
	var findings []string
	report := func(message string) { findings = append(findings, path+": backup manual contract: "+message) }
	owned := backupManualOwned(path)
	if !owned && filepath.ToSlash(filepath.Dir(path)) == "internal/testkit/backupmanual" { report("unreviewed file in manual helper package") }
	if !owned && strings.Contains(strings.SplitN(string(raw), "\n", 2)[0], "manual_backup_publication") { report("manual Backup tag outside exact reviewed inventory") }
	if owned {
		if want, native := backupManualTags[path]; native {
			if first := strings.SplitN(string(raw), "\n", 2)[0]; first != want { report("exact Linux/manual build conjunction changed") }
		} else if strings.Contains(string(raw), "//go:build") || strings.Contains(string(raw), "// +build") { report("portable schema source must not have a build constraint") }
		if fmt.Sprintf("%x", sha256.Sum256(raw)) != backupManualPins[path] { report("complete reviewed source shape changed") }
	}
	imports := map[string]string{}
	for _, imp := range file.Imports {
		value, err := strconv.Unquote(imp.Path.Value)
		if err != nil { return nil, err }
		name := filepath.Base(value)
		if imp.Name != nil { name = imp.Name.Name }
		imports[name] = value
		if value == backupManualImport && (!owned || path == backupManualHelper || name != "backupmanual") { report("backupmanual import outside three exact test consumers or aliased") }
		if owned && (value == "os/exec" || (name == "." || name == "_")) { report("unreviewed process/dot/blank import") }
	}
	entries := map[string]int{}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok { continue }
		if owned && (fn.Name.Name == "init" || fn.Name.Name == "TestMain") { report("manual source cannot add initializer/TestMain") }
		wantPath, entry := backupManualEntries[fn.Name.Name]
		if !entry { continue }
		entries[fn.Name.Name]++
		if path != wantPath || fn.Recv != nil { report("original test copied/moved/duplicated outside exact file"); continue }
		if fn.Body == nil || len(fn.Body.List) == 0 { report("missing first-effect gate"); continue }
		want := "run := backupmanual.ForTest(t)"
		if path == "cmd/gosqlite/plain_backup_linux_test.go" { want = "manualRun := backupmanual.ForTest(t)" }
		if oracleNodeText(set, raw, fn.Body.List[0]) != want { report("entry first effect must be exact ForTest gate") }
	}
	if owned {
		for entry, wantPath := range backupManualEntries { if path == wantPath && entries[entry] != 1 { report("missing/duplicate original entry "+entry) } }
	}
	if !owned { return findings, nil }
	if path == backupManifestSource || path == backupManifestTests {
		findings = append(findings, inspectBackupManifestPure(path, raw, set, file, imports)...)
		return findings, nil
	}
	if path == backupManualHelper { findings = append(findings, inspectBackupManifestLoad(path, raw, set, file)... ) }
	counts := map[string]int{}
	parents := oracleParents(file)
	ast.Inspect(file, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && backupManualEffectNames[sel.Sel.Name] { counts[sel.Sel.Name]++ }
		}
		sel, ok := n.(*ast.SelectorExpr)
		if !ok { return true }
		base, qualified := sel.X.(*ast.Ident)
		pkg := ""
		if qualified { pkg = imports[base.Name] }
		switch sel.Sel.Name {
		case "TempDir", "CreateTemp", "MkdirTemp", "Remove", "RemoveAll", "Unlink", "Unlinkat", "Rename", "Renameat", "Renameat2", "Link", "Symlink", "Unmount", "Unshare", "Setns", "Chmod", "Fchmod", "Rollback", "RetryPlaintextCleanup", "Command", "CommandContext", "Environ", "StartProcess", "ForkExec", "Kill", "Signal", "Chdir", "Setenv", "LookupEnv":
			report("forbidden effect/reference "+sel.Sel.Name)
		case "Mount":
			if !backupManualFactMountRead(path, raw, set, file, sel, parents) { report("forbidden effect/reference Mount") }
		}
		if (pkg == "os" || pkg == "path/filepath") && (sel.Sel.Name == "ExpandEnv" || sel.Sel.Name == "UserHomeDir" || sel.Sel.Name == "UserConfigDir" || sel.Sel.Name == "EvalSymlinks" || sel.Sel.Name == "Abs") { report("implicit path discovery/fallback") }
		if path != backupManualHelper && sel.Sel.Name == "Close" { report("direct end bypasses once-owner ledger") }
		if pkg == "os" && sel.Sel.Name == "ReadFile" { report("unbounded direct file collector") }
		return true
	})
	want := backupManualEffectCounts[path]
	if len(counts) != len(want) { report("actual native/oracle call inventory changed") }
	for name, count := range counts { if want[name] != count { report("actual call count changed: "+name) } }
	return findings, nil
}

// Only the four reviewed uint64 fact-field comparisons may use a Mount
// selector. Exact function bodies exclude calls, values, aliases, nested scopes
// and other receivers. Declaration/object checks bind both operands to the
// explicit fact parameters; the whole-source pins remain an additional guard.
func backupManualFactMountRead(path string, raw []byte, set *token.FileSet, file *ast.File, sel *ast.SelectorExpr, parents map[ast.Node]ast.Node) bool {
	if path != backupManualHelper || file.Name.Name != "backupmanual" || sel.Sel.Name != "Mount" { return false }
	fn := oracleEnclosingFunction(sel, parents)
	if fn == nil || fn.Recv != nil { return false }
	want := ""
	switch fn.Name.Name {
	case "sameIdentity":
		want = "func sameIdentity(a, b fact) bool {\n\treturn a.Major == b.Major && a.Minor == b.Minor && a.Inode == b.Inode && a.Mount == b.Mount && a.Mode&unix.S_IFMT == b.Mode&unix.S_IFMT\n}"
	case "sameMount":
		want = "func sameMount(a, b fact) bool { return a.Major == b.Major && a.Minor == b.Minor && a.Mount == b.Mount }"
	default:
		return false
	}
	if oracleNodeText(set, raw, fn) != want { return false }
	if file.Scope == nil || file.Scope.Lookup("uint64") != nil || file.Scope.Lookup("bool") != nil { return false }
	for _, imp := range file.Imports {
		value, err := strconv.Unquote(imp.Path.Value)
		if err != nil { return false }
		name := filepath.Base(value)
		if imp.Name != nil { name = imp.Name.Name }
		if name == "." || name == "fact" || name == "uint64" || name == "bool" { return false }
	}
	var factType *ast.TypeSpec
	functions := 0
	for _, decl := range file.Decls {
		if other, ok := decl.(*ast.FuncDecl); ok && other.Name.Name == fn.Name.Name { functions++ }
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE { continue }
		for _, spec := range gen.Specs {
			typ, ok := spec.(*ast.TypeSpec)
			if !ok || typ.Name.Name != "fact" { continue }
			if factType != nil { return false }
			factType = typ
		}
	}
	if functions != 1 || factType == nil || factType.Assign.IsValid() || factType.TypeParams != nil { return false }
	if factType.Name.Obj == nil || file.Scope.Lookup("fact") != factType.Name.Obj { return false }
	structure, ok := factType.Type.(*ast.StructType)
	if !ok || structure.Fields == nil { return false }
	mountFields := 0
	for _, field := range structure.Fields.List {
		for _, name := range field.Names {
			if name.Name != "Mount" { continue }
			typ, ok := field.Type.(*ast.Ident)
			if !ok || typ.Name != "uint64" || typ.Obj != nil || len(field.Names) != 1 { return false }
			mountFields++
		}
	}
	if mountFields != 1 || fn.Type.Params == nil || len(fn.Type.Params.List) != 1 { return false }
	parameter := fn.Type.Params.List[0]
	typ, ok := parameter.Type.(*ast.Ident)
	if !ok || typ.Name != "fact" || typ.Obj != factType.Name.Obj || len(parameter.Names) != 2 { return false }
	a, b := parameter.Names[0], parameter.Names[1]
	if a.Name != "a" || b.Name != "b" || a.Obj == nil || b.Obj == nil || a.Obj == b.Obj { return false }
	comparison, ok := parents[sel].(*ast.BinaryExpr)
	if !ok || comparison.Op != token.EQL { return false }
	left, leftOK := comparison.X.(*ast.SelectorExpr)
	right, rightOK := comparison.Y.(*ast.SelectorExpr)
	if !leftOK || !rightOK || left.Sel.Name != "Mount" || right.Sel.Name != "Mount" { return false }
	leftBase, leftOK := left.X.(*ast.Ident)
	rightBase, rightOK := right.X.(*ast.Ident)
	return leftOK && rightOK && leftBase.Obj == a.Obj && rightBase.Obj == b.Obj && (sel == left || sel == right)
}

func inspectBackupManualContracts(root string) ([]string, error) {
	var findings []string
	for path := range backupManualPins {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil { return nil, err }
		items, err := inspectBackupManualSource(path, raw)
		if err != nil { return nil, err }
		findings = append(findings, items...)
	}
	return findings, nil
}


// Pure default-build schema sources cannot acquire the native helper's effects,
// even when a later manual-tag compilation makes both files visible together.
func inspectBackupManifestPure(path string, raw []byte, set *token.FileSet, file *ast.File, imports map[string]string) []string {
	var findings []string
	report := func(message string) { findings=append(findings,path+": portable manifest contract: "+message) }
	allowed:=map[string]bool{"bytes":true,"encoding/json":true,"fmt":true,"io":true}
	if path==backupManifestTests { allowed=map[string]bool{"bytes":true,"encoding/json":true,"fmt":true,"strings":true,"strconv":true,"testing":true} }
	for alias,pkg:=range imports { if !allowed[pkg] || alias!=filepath.Base(pkg) { report("non-memory or aliased import "+pkg) } }
	entries:=map[string]int{}
	for _,decl:=range file.Decls {
		if gen,ok:=decl.(*ast.GenDecl);ok && gen.Tok==token.VAR { report("portable source must not contain mutable package variables") }
		if fn,ok:=decl.(*ast.FuncDecl);ok {
			if fn.Name.Name=="init" || fn.Name.Name=="TestMain" { report("portable initializer/TestMain") }
			if strings.HasPrefix(fn.Name.Name,"Test") { entries[fn.Name.Name]++ }
		}
	}
	if path==backupManifestTests {
		want:=[]string{"TestManualBackupManifestExactSchema","TestManualBackupManifestRejectsCaseAliases","TestManualBackupManifestRejectsMissingAndUnknownFields","TestManualBackupManifestRejectsDuplicateAndUnboundedJSON"}
		if len(entries)!=len(want) { report("pure regression entry inventory changed") }
		for _,name:=range want { if entries[name]!=1 { report("missing/duplicate pure regression "+name) } }
	} else if len(entries)!=0 { report("tests in portable implementation file") }
	parents:=oracleParents(file)
	ast.Inspect(file,func(n ast.Node)bool{
		switch n.(type) { case *ast.GoStmt,*ast.SendStmt:report("portable concurrency effect") }
		if id,ok:=n.(*ast.Ident);ok {
			if id.Name=="Run" {
				sel,member:=parents[id].(*ast.SelectorExpr)
				if !member || sel.Sel!=id { report("native Run type/function reference") }
			}
			switch id.Name { case "ForTest","Scope","process","fdOwner","Output","unsafe":report("native helper reference "+id.Name) }
		}
		if sel,ok:=n.(*ast.SelectorExpr);ok {
			switch sel.Sel.Name { case "Parallel","TempDir","Setenv","Chdir","Skip","Skipf","SkipNow","Open","Close","ReadFile","WriteFile","Getenv","Environ","Command","CommandContext","Start","Wait","Backup","OwnDB","OwnTx","OwnRows":report("non-pure reference "+sel.Sel.Name) }
			// Test Fatal/Fatalf and t.Run are assertion reporting/finite
			// subtests, never hooks accepted by the actual pure decoder.
			if path==backupManifestSource && (sel.Sel.Name=="Fatal" || sel.Sel.Name=="Fatalf" || sel.Sel.Name=="FailNow" || sel.Sel.Name=="Run") { report("decoder test hook") }
			if path==backupManifestTests && sel.Sel.Name=="Run" {
				base,ok:=sel.X.(*ast.Ident)
				if !ok || base.Name!="t" { report("unreviewed pure-test Run call") }
			}
		}
		return true
	})
	if path==backupManifestSource {
		contracts:=map[string][]string{
			"decodeManifest":{
				`if len(raw) > manifestLimit { return manifest{}, fmt.Errorf("manifest byte limit") }`,
				`if err := uniqueJSON(raw); err != nil { return manifest{}, err }`,
				`if err := validateManifestSchema(raw); err != nil { return manifest{}, err }`,
				`var result manifest`,
				`d := json.NewDecoder(bytes.NewReader(raw))`,
				`d.DisallowUnknownFields()`,
				`if err := d.Decode(&result); err != nil { return manifest{}, err }`,
				`return result, nil`,
			},
			"schemaObject":{
				`trimmed := bytes.TrimSpace(raw)`,
				`if len(trimmed) == 0 || trimmed[0] != '{' { return nil, fmt.Errorf("schema object required") }`,
				`var object map[string]json.RawMessage`,
				`if err := json.Unmarshal(raw, &object); err != nil { return nil, err }`,
				`if len(object) != len(required) { return nil, fmt.Errorf("schema exact member count") }`,
				`for _, key := range required {
		if _, ok := object[key]; !ok { return nil, fmt.Errorf("schema missing exact member %q", key) }
	}`,
				`return object, nil`,
			},
		}
		seen:=map[string]bool{}
		for _,decl:=range file.Decls { if fn,ok:=decl.(*ast.FuncDecl);ok {
			if want,ok:=contracts[fn.Name.Name];ok {
				seen[fn.Name.Name]=true
				if fn.Body==nil || len(fn.Body.List)!=len(want) { report("actual decoder/schema sequence changed");continue }
				for i,stmt:=range fn.Body.List { if oracleNodeText(set,raw,stmt)!=want[i] { report("actual ordered statement changed: "+fn.Name.Name) } }
			}
		} }
		for name:=range contracts { if !seen[name] { report("required pure boundary missing: "+name) } }
		// These exact conditions bind the inherited finite token contract, in
		// addition to the behavioral boundary regressions in the pure package.
		for _,required:=range []string{"manifestLimit = 256 << 10","depth > 16 || count > 12000","len(s) > 4096","len(key) > 256 || keys[key]","d.Token(); err != io.EOF"} { if !strings.Contains(string(raw),required) { report("finite JSON contract changed: "+required) } }
	}
	return findings
}

// Actual loadManifest statements must authenticate the raw sidecar bytes, then
// completely decode their exact schema, before consuming any inventory pin.
func inspectBackupManifestLoad(path string, raw []byte, set *token.FileSet, file *ast.File) []string {
	var findings []string
	report:=func(message string){ findings=append(findings,path+": manifest load order: "+message) }
	want:=[]string{
		`base := filepath.Join(r.root, "evidence")`,
		`raw := r.readExact(filepath.Join(base, "input-manifest.json"), manifestLimit, nil)`,
		`digest := r.readExact(filepath.Join(base, "input-manifest.sha256"), 65, nil)`,
		`if len(digest) == 65 && digest[64] == '\n' { digest = digest[:64] }`,
		`if !lowerHex(string(digest), 64) || string(digest) != fmt.Sprintf("%x", sha256.Sum256(raw)) { r.blocked("manifest raw digest mismatch", nil) }`,
		`m, err := decodeManifest(raw)`,
		`if err != nil { r.blocked("manifest exact schema", err) }`,
	}
	count:=0
	for _,decl:=range file.Decls { if fn,ok:=decl.(*ast.FuncDecl);ok && fn.Name.Name=="loadManifest" {
		count++
		if fn.Body==nil || len(fn.Body.List)<len(want) { report("missing prefix");continue }
		for i,text:=range want { if oracleNodeText(set,raw,fn.Body.List[i])!=text { report("raw digest/decode/pre-sidecar order changed") } }
	} }
	if count!=1 { report("missing/duplicate load boundary") }
	return findings
}
