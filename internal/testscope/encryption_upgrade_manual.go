package main

// SOURCE_ONLY, pins HOLD. The structural checks are independent of the pin and
// have inert counterexamples. This is not a type checker or a native qualification.
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

const formatUpgradeNative = "sqlite/encryption_upgrade_manual_linux_test.go"
const formatUpgradePure = "internal/testkit/formatupgrade/contract.go"
const formatUpgradePureTest = "internal/testkit/formatupgrade/contract_test.go"
const formatUpgradeImport = "github.com/ms-xiaosnow/gosqlite/internal/testkit/formatupgrade"
const formatUpgradeEntry = "TestManualEncryptedSnapshotToWALUpgrade"
const formatUpgradeTag = "//go:build linux && manual_format_upgrade"

// Do not replace HOLD with a computed hash before independent full-source review.
// The pre-existing WAL guard's main.go pin also deliberately remains unreleased.
var formatUpgradePins = map[string]string{
	formatUpgradeNative:          "61944780e8a3c4e4d809c9d2ce335da5a6b92ec4ef4c10371ab3d2b577b490d4",
	formatUpgradePure:            "74c615d7adda5ecfcd6d1e296832f8f0078030ca4699b1e93c42e3c987357c9d",
	formatUpgradePureTest:        "678923185ace3852da1c588441a9c7b491c8c11c89baabae015a8053cfa043de",
	"internal/testscope/main.go": "dd0d43330c5ad020b4ab2e5649f32f963aa4909316be64ae32c01d9f3c4371aa",
}

func formatUpgradeName(x ast.Expr) string {
	switch n := x.(type) {
	case *ast.Ident:
		return n.Name
	case *ast.SelectorExpr:
		return formatUpgradeName(n.X) + "." + n.Sel.Name
	case *ast.CallExpr:
		return "()"
	case *ast.ParenExpr:
		return formatUpgradeName(n.X)
	case *ast.CompositeLit:
		return "<composite>"
	case *ast.IndexExpr:
		return "<indexed>"
	case *ast.FuncLit:
		return "<literal>"
	case *ast.ArrayType:
		if n.Len == nil && formatUpgradeName(n.Elt) == "byte" {
			return "[]byte"
		}
	}
	return "<unreviewed>"
}
func formatUpgradeText(set *token.FileSet, raw []byte, n ast.Node) string {
	return strings.Join(strings.Fields(oracleNodeText(set, raw, n)), " ")
}

func formatUpgradeDirectStatement(node ast.Node, fn *ast.FuncDecl, parents map[ast.Node]ast.Node) bool {
	switch statement := parents[node].(type) {
	case *ast.ExprStmt:
		return parents[statement] == fn.Body
	case *ast.AssignStmt:
		return parents[statement] == fn.Body
	}
	return false
}

// This exact first statement rejects before any fixture, provider, source walk,
// acquisition, registration or defer. It is not a general dominance analysis.
func formatUpgradeGate(fn *ast.FuncDecl, set *token.FileSet, raw []byte) bool {
	return fn.Body != nil && len(fn.Body.List) > 0 && formatUpgradeText(set, raw, fn.Body.List[0]) == `if os.Getenv("GOSQLITE_MANUAL_FORMAT_UPGRADE") != "independently_authorized" { t.Fatal("manual format upgrade is not independently authorized") }`
}

func inspectFormatUpgradeStructure(path string, raw []byte) ([]string, error) {
	path = filepath.ToSlash(filepath.Clean(path))
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, path, raw, 0)
	if err != nil {
		return nil, err
	}
	var findings []string
	report := func(s string) { findings = append(findings, path+": format-upgrade manual: "+s) }
	native := path == formatUpgradeNative
	pure := path == formatUpgradePure || path == formatUpgradePureTest
	owned := native || pure
	if !native && strings.Contains(strings.SplitN(string(raw), "\n", 2)[0], "manual_format_upgrade") {
		report("manual tag outside exact native file")
	}
	if !pure && strings.HasPrefix(path, "internal/testkit/formatupgrade/") {
		report("helper-package inventory expanded")
	}
	if native && strings.SplitN(string(raw), "\n", 2)[0] != formatUpgradeTag {
		report("exact Linux AND manual tag required")
	}
	if pure && (strings.Contains(string(raw), "//go:build") || strings.Contains(string(raw), "// +build")) {
		report("portable helper cannot have build constraints")
	}
	if native && file.Name.Name != "sqlite_test" {
		report("native package changed")
	}
	if pure && file.Name.Name != "formatupgrade" {
		report("pure package changed")
	}
	pureImports := map[string]bool{"encoding/hex": true, "fmt": true, "path": true, "reflect": true, "strings": true}
	if path == formatUpgradePureTest {
		pureImports = map[string]bool{"reflect": true, "strings": true, "testing": true}
	}
	for _, imp := range file.Imports {
		name, e := strconv.Unquote(imp.Path.Value)
		if e != nil {
			return findings, e
		}
		if name == formatUpgradeImport && !native {
			report("pure helper consumer outside exact native file")
		}
		if owned && imp.Name != nil {
			report("aliased/dot/blank import forbidden")
		}
		if native && !formatUpgradeNativeImports[name] {
			report("native import closure expanded")
		}
		if pure && !pureImports[name] {
			report("descriptor-free import closure expanded")
		}
	}
	entries := 0
	functions := map[string]int{}
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Name.Name == formatUpgradeEntry {
				entries++
				if !native || d.Recv != nil || !formatUpgradeGate(d, set, raw) {
					report("copied entry or missing first-action environment gate")
				}
			}
			if !native && strings.HasPrefix(d.Name.Name, "manualFormat") {
				report("native helper copied outside native file")
			}
			if owned && (d.Name.Name == "init" || d.Name.Name == "TestMain" || strings.HasPrefix(d.Name.Name, "Benchmark") || strings.HasPrefix(d.Name.Name, "Fuzz") || strings.HasPrefix(d.Name.Name, "Example")) {
				report("extra executable entry forbidden")
			}
			if native {
				functions[d.Name.Name]++
				if !formatUpgradeNativeFunctions[d.Name.Name] {
					report("unreviewed native helper/test")
				}
			}
		case *ast.GenDecl:
			if owned && d.Tok == token.VAR {
				for _, spec := range d.Specs {
					v, ok := spec.(*ast.ValueSpec)
					if !ok || !native || len(v.Names) != 1 || v.Names[0].Name != "manualFormatRetained" || len(v.Values) != 0 {
						report("package state/initializer forbidden")
					}
				}
			}
		}
	}
	if native {
		if entries != 1 {
			report("exactly one native entry required")
		}
		for name := range formatUpgradeNativeFunctions {
			if functions[name] != 1 {
				report("native function missing/duplicated: " + name)
			}
		}
	}
	if !owned {
		return findings, nil
	}
	calls := map[string]int{}
	capturePairs := 0
	criticalCalls := map[string]int{}
	criticalStatements := map[string]int{}
	assertions := map[string]int{}
	parents := oracleParents(file)
	ast.Inspect(file, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.GoStmt:
			report("goroutine forbidden")
		case *ast.ForStmt:
			if x.Cond == nil {
				report("unbounded loop forbidden")
			}
		case *ast.CallExpr:
			name := formatUpgradeName(x.Fun)
			if native {
				fn := oracleEnclosingFunction(x, parents)
				if fn != nil {
					actual := formatUpgradeText(set, raw, x)
					key := fn.Name.Name + "/" + actual
					if _, known := formatUpgradeCriticalCalls[key]; known {
						criticalCalls[key]++
						if !formatUpgradeDirectStatement(x, fn, parents) {
							report("critical call is not a direct top-level statement")
						}
					}
					if fn.Name.Name == "ledger" && name == "r.require" && len(x.Args) == 3 {
						lit, ok := x.Args[1].(*ast.BasicLit)
						if ok && lit.Kind == token.STRING {
							label, e := strconv.Unquote(lit.Value)
							if e == nil {
								if want, critical := formatUpgradeAssertions[label]; critical {
									assertions[label]++
									if !formatUpgradeDirectStatement(x, fn, parents) {
										report("ledger assertion moved under control flow")
									}
									if actual != want {
										report("actual ledger assertion changed: " + label)
									}
								}
							}
						}
					}
				}
			}
			calls[name]++
			if native && !formatUpgradeNativeCalls[name] {
				report("native call closure expanded: " + name)
			}
			if pure && !formatUpgradePureCalls[name] {
				report("descriptor-free call closure expanded: " + name)
			}
		case *ast.AssignStmt:
			if native {
				fn := oracleEnclosingFunction(x, parents)
				if fn != nil {
					key := fn.Name.Name + "/" + formatUpgradeText(set, raw, x)
					if _, known := formatUpgradeCriticalStatements[key]; known {
						criticalStatements[key]++
						if parents[x] != fn.Body {
							report("critical declaration moved under control flow")
						}
					}
				}
			}
		case *ast.SelectorExpr:
			if native && formatUpgradeAcquisitionNames[x.Sel.Name] {
				call, ok := parents[x].(*ast.CallExpr)
				if !ok || call.Fun != x {
					report("native ownership/effect alias forbidden")
				}
			}
		case *ast.BlockStmt:
			if !native {
				break
			}
			for i, stmt := range x.List {
				a, ok := stmt.(*ast.AssignStmt)
				if !ok || len(a.Rhs) != 1 || len(a.Lhs) != 2 {
					continue
				}
				call, ok := a.Rhs[0].(*ast.CallExpr)
				if !ok {
					continue
				}
				name := formatUpgradeName(call.Fun)
				slot := 0
				switch name {
				case "sqlite.OpenContext":
					slot = 1
				case "dbOwner.db.Query":
					slot = 2
				case "os.Open":
					slot = 3
				}
				if slot == 0 {
					continue
				}
				valid := false
				if i+1 < len(x.List) {
					b, ok := x.List[i+1].(*ast.AssignStmt)
					if ok && len(b.Rhs) == 1 {
						capture, ok := b.Rhs[0].(*ast.CallExpr)
						if ok && formatUpgradeName(capture.Fun) == "r.capture" && len(capture.Args) == 6 && formatUpgradeName(capture.Args[slot]) == formatUpgradeName(a.Lhs[0]) && formatUpgradeName(capture.Args[5]) == formatUpgradeName(a.Lhs[1]) {
							valid = true
						}
					}
				}
				if !valid {
					report("acquisition owner/result must be captured in adjacent statement")
				} else {
					capturePairs++
				}
			}
		}
		return true
	})
	if native {
		for key := range formatUpgradeCriticalCalls {
			if criticalCalls[key] != 1 {
				report("missing/duplicate critical AST call: " + key)
			}
		}
		for key := range formatUpgradeCriticalStatements {
			if criticalStatements[key] != 1 {
				report("missing/duplicate critical AST statement: " + key)
			}
		}
		for label := range formatUpgradeAssertions {
			if assertions[label] != 1 {
				report("missing/duplicate actual ledger assertion: " + label)
			}
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if expected, known := formatUpgradeFirstStatements[fn.Name.Name]; known {
				if fn.Body == nil || len(fn.Body.List) == 0 || formatUpgradeText(set, raw, fn.Body.List[0]) != expected {
					report("first ownership/gate statement changed: " + fn.Name.Name)
				}
			}
		}
		for name, want := range map[string]int{"r.ledger": 4, "r.query": 5, "r.absent": 4, "sqlite.OpenContext": 4, "dbOwner.db.Query": 1, "os.Open": 3, "os.MkdirTemp": 1, "os.Mkdir": 1, "db.UpgradeEncryptedWAL": 1, "db.BackupEncrypted": 1, "sqlite.RestoreEncrypted": 1, "o.db.Close": 1, "o.rows.Close": 1, "o.file.Close": 1} {
			if calls[name] != want {
				report("exact effect count changed: " + name)
			}
		}
		if capturePairs != 8 {
			report("exact immediate owner-capture inventory changed")
		}
		for _, snippet := range formatUpgradeRequiredText {
			if !strings.Contains(string(raw), snippet) {
				report("required finite assertion/lifecycle/gate missing: " + snippet)
			}
		}
	}
	return findings, nil
}
func inspectFormatUpgradeSource(path string, raw []byte) ([]string, error) {
	findings, err := inspectFormatUpgradeStructure(path, raw)
	if err != nil {
		return findings, err
	}
	path = filepath.ToSlash(filepath.Clean(path))
	if want, ok := formatUpgradePins[path]; ok && fmt.Sprintf("%x", sha256.Sum256(raw)) != want {
		findings = append(findings, path+": format-upgrade full-source pin HOLD/mismatch; independent review required")
	}
	return findings, nil
}
func inspectFormatUpgradeContracts(root string) ([]string, error) {
	var findings []string
	for path := range formatUpgradePins {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return findings, err
		}
		got, err := inspectFormatUpgradeSource(path, raw)
		if err != nil {
			return findings, err
		}
		findings = append(findings, got...)
	}
	return findings, nil
}

var formatUpgradeNativeImports = map[string]bool{
	"context":       true,
	"crypto/sha256": true,
	"encoding/hex":  true,
	"encoding/json": true,
	"errors":        true,
	"flag":          true,
	"fmt":           true,
	"io":            true,
	"os":            true,
	"path/filepath": true,
	"reflect":       true,
	"runtime":       true,
	"runtime/debug": true,
	"sort":          true,
	"strings":       true,
	"sync":          true,
	"syscall":       true,
	"testing":       true,
	"time":          true,
	"github.com/ms-xiaosnow/gosqlite/encryption":                     true,
	"github.com/ms-xiaosnow/gosqlite/internal/testkit/formatupgrade": true,
	"github.com/ms-xiaosnow/gosqlite/sqlite":                         true,
}
var formatUpgradeNativeFunctions = map[string]bool{
	"states":                true,
	"capturePending":        true,
	"manualFormatDirectory": true,
	"Acquire":               true,
	"Kind":                  true,
	"Bytes":                 true,
	"Release":               true,
	"manualFormatError":     true,
	"event":                 true,
	"capture":               true,
	"retire":                true,
	"require":               true,
	"finish":                true,
	"options":               true,
	"manualFormatHead":      true,
	"TestManualEncryptedSnapshotToWALUpgrade": true,
	"preflight":     true,
	"directory":     true,
	"makeDirectory": true,
	"absent":        true,
	"hashFile":      true,
	"sourceDigest":  true,
	"manifest":      true,
	"query":         true,
	"verify":        true,
	"ledger":        true,
	"runCase":       true,
}
var formatUpgradeNativeCalls = map[string]bool{
	"formatupgrade.AcquisitionOf":     true,
	"formatupgrade.CanRetire":         true,
	"r.states":                        true,
	"r.capturePending":                true,
	"manualFormatDirectory":           true,
	"r.receipt.SourceRoot.Validate":   true,
	"sourceInfo.IsDir":                true,
	"sourceInfo.Mode":                 true,
	"syscall.Statfs":                  true,
	"syscall.Uname":                   true,
	"().IsRegular":                    true,
	"().Perm":                         true,
	"<literal>":                       true,
	"[]byte":                          true,
	"after.ModTime":                   true,
	"after.Size":                      true,
	"append":                          true,
	"before.IsDir":                    true,
	"before.ModTime":                  true,
	"before.Mode":                     true,
	"before.Size":                     true,
	"before.Sys":                      true,
	"c.Validate":                      true,
	"cancel":                          true,
	"clear":                           true,
	"context.Background":              true,
	"context.WithTimeout":             true,
	"db.BackupEncrypted":              true,
	"db.Exec":                         true,
	"db.UpgradeEncryptedWAL":          true,
	"dbOwner.db.Query":                true,
	"debug.ReadBuildInfo":             true,
	"delete":                          true,
	"entry.IsDir":                     true,
	"entry.Name":                      true,
	"entry.Type":                      true,
	"err.Error":                       true,
	"errors.As":                       true,
	"errors.Is":                       true,
	"errors.Join":                     true,
	"f.ReadDir":                       true,
	"f.Stat":                          true,
	"f.Value.String":                  true,
	"filepath.Base":                   true,
	"filepath.Clean":                  true,
	"filepath.EvalSymlinks":           true,
	"filepath.FromSlash":              true,
	"filepath.IsAbs":                  true,
	"filepath.Join":                   true,
	"filepath.Rel":                    true,
	"filepath.ToSlash":                true,
	"flag.Lookup":                     true,
	"flag.Visit":                      true,
	"float64":                         true,
	"fmt.Errorf":                      true,
	"fmt.Fprintf":                     true,
	"fmt.Sprintf":                     true,
	"formatupgrade.Equal":             true,
	"formatupgrade.Hash":              true,
	"formatupgrade.Preserve":          true,
	"h.Sum":                           true,
	"hex.EncodeToString":              true,
	"info.IsDir":                      true,
	"info.Mode":                       true,
	"info.Sys":                        true,
	"int64":                           true,
	"io.Copy":                         true,
	"io.LimitReader":                  true,
	"json.Marshal":                    true,
	"len":                             true,
	"m.provider.mu.Lock":              true,
	"m.provider.mu.Unlock":            true,
	"manualFormatError":               true,
	"manualFormatHead":                true,
	"o.db.Check":                      true,
	"o.db.Schema":                     true,
	"o.db.Close":                      true,
	"o.db.VerifyEncrypted":            true,
	"o.file.Close":                    true,
	"o.rows.Close":                    true,
	"o.state.Retire":                  true,
	"os.Executable":                   true,
	"os.Getenv":                       true,
	"os.Geteuid":                      true,
	"os.Lstat":                        true,
	"os.Mkdir":                        true,
	"os.MkdirTemp":                    true,
	"os.Open":                         true,
	"os.SameFile":                     true,
	"out.Validate":                    true,
	"p.mu.Lock":                       true,
	"p.mu.Unlock":                     true,
	"r.absent":                        true,
	"r.capture":                       true,
	"r.directory":                     true,
	"r.event":                         true,
	"r.finish":                        true,
	"r.hashFile":                      true,
	"r.ledger":                        true,
	"r.makeDirectory":                 true,
	"r.manifest":                      true,
	"r.options":                       true,
	"r.preflight":                     true,
	"r.query":                         true,
	"r.receipt.Validate":              true,
	"r.require":                       true,
	"r.retire":                        true,
	"r.runCase":                       true,
	"r.sourceDigest":                  true,
	"r.t.Errorf":                      true,
	"r.t.Failed":                      true,
	"r.t.Fatal":                       true,
	"r.t.Fatalf":                      true,
	"r.t.Log":                         true,
	"r.t.Logf":                        true,
	"r.verify":                        true,
	"reflect.DeepEqual":               true,
	"rows.Err":                        true,
	"rows.Next":                       true,
	"rows.Plan":                       true,
	"rows.Values":                     true,
	"runtime.Version":                 true,
	"sha256.New":                      true,
	"sha256.Sum256":                   true,
	"sort.Slice":                      true,
	"sort.Strings":                    true,
	"sqlite.OpenContext":              true,
	"sqlite.PendingEncryptedCleanups": true,
	"sqlite.RestoreEncrypted":         true,
	"strings.HasPrefix":               true,
	"strings.HasSuffix":               true,
	"t.Fatal":                         true,
	"uint32":                          true,
	"uint64":                          true,
	"uint8":                           true,
}
var formatUpgradeRequiredText = []string{
	`"explicit-unique-schema-preserved"`,
	`vector_to_text(embedding)`,
	`"test.run": "^TestManualEncryptedSnapshotToWALUpgrade$"`,
	`"test.count": "1"`,
	`"test.timeout": "0s"`,
	`"test.parallel": "1"`,
	`"test.shuffle": "off"`,
	`"test.v": "true"`,
	`for _, page := range []uint32{512, 4096, 65536}`,
	`os.Getenv("GOSQLITE_MANUAL_FORMAT_UPGRADE_NAMESPACE") == "private-stable-no-alias-no-other-process"`,
	`context.WithTimeout(context.Background(), 2*time.Minute)`,
	`r.receipt.BinarySHA256 = actualBinary`,
	`r.receipt.SourceSHA256 = r.sourceDigest(source)`,
	`if o.state.Present && o.state.Terminal == "live" && o.state.Attempts == 0`,
	`formatupgrade.CanRetire(o.state, r.states(), r.receipt.InternalPendingObserved)`,
	`o.state.Retire("close", manualFormatError(err))`,
	`o.state.Retire("eof", manualFormatError(err))`,
	`if err == nil && !r.t.Failed() {
		r.t.Log("FORMAT_UPGRADE_COMPLETE cases=3 preserved=true owners=retired")
	}`,
	`formatupgrade.Preserve(c.SourceBefore, c.SourceAfter, "source.gse")`,
	`formatupgrade.Equal(c.ArchiveBefore, r.manifest(archiveDir, "archive-after-upgrade"))`,
	`c.Upgraded.Sequence == c.Before.Sequence+1`,
	`errors.As(oldErr, &oldDetail) && oldDetail.Code == sqlite.CodeError`,
	`c.OldModeNil = !oldOwner.state.Present`,
	`r.absent(restored)`,
	`c.Restored == manualFormatHead(restoreInfo)`,
	`"explicit-unique-supported-scan"`,
	`plan.Method == "grams13"`,
	`scanPlan.Reason == "NOT INDEXED requested"`,
	`"dense-vector-exact-ledger"`,
	`c.Checks = append(c.Checks, "source-and-archive-unchanged")`,
	`r.receipt.Cases = append(r.receipt.Cases, c)`,
	`err := r.receipt.Validate()`,
}

var formatUpgradeAcquisitionNames = map[string]bool{"OpenContext": true, "Query": true, "Open": true, "Close": true, "Mkdir": true, "MkdirTemp": true, "UpgradeEncryptedWAL": true, "BackupEncrypted": true, "RestoreEncrypted": true}
var formatUpgradePureCalls = map[string]bool{
	"AcquisitionOf": true, "CanRetire": true, "OwnerInventory": true, "append": true,
	"binding.snapshot.Validate": true, "c.Root.Validate": true, "d.ChildOf": true, "d.Validate": true,
	"fmt.Sprintf": true, "path.Base": true, "path.Clean": true, "path.IsAbs": true, "path.Join": true,
	"r.KnownRoot.Validate": true, "r.RunRoot.ChildOf": true, "r.SourceRoot.Validate": true,
	"s.Directory.Validate": true, "s.Files.Validate": true, "strings.TrimPrefix": true, "strings.TrimSuffix": true,
	"<indexed>.Root.ChildOf": true, "add": true, "rows.Retire": true,
	"Equal": true, "Hash": true, "Owners": true, "Preserve": true, "RequiredChecks": true,
	"after.Validate": true, "before.Validate": true, "c.Archive.valid": true, "c.Before.valid": true,
	"c.Reopened.valid": true, "c.Restored.valid": true, "c.Upgraded.valid": true,
	"fmt.Errorf": true, "hex.DecodeString": true, "len": true, "reflect.DeepEqual": true,
	"strings.ContainsAny": true, "strings.HasPrefix": true, "strings.HasSuffix": true,
	"strings.Repeat": true, "uint64": true, "<indexed>.Validate": true,
	"Manifest": true, "m.change": true, "memoryReceipt": true, "mutate": true,
	"o.Retire": true, "r.Validate": true, "t.Fatal": true, "t.Run": true, "u.Retire": true,
	"().Validate": true, "<composite>.Validate": true,
}

// Exact finite AST span contracts: string occurrences in comments do not count.
var formatUpgradeAssertions = map[string]string{
	"schema-result":                    `r.require(err == nil, "schema-result", err)`,
	"explicit-unique-schema-preserved": `r.require(uniqueSchema == 1, "explicit-unique-schema-preserved", nil)`,
	"check-supported-indexes":          `r.require(err == nil && result.Scope == sqlite.CheckScope && result.Indexes >= 1 && result.ExtensionIndexes == 1, "check-supported-indexes", err)`,
	"complete-typed-row-ledger":        `r.require(reflect.DeepEqual(got, want), "complete-typed-row-ledger", nil)`,
	"explicit-unique-supported-scan":   `r.require(reflect.DeepEqual(got, [][]any{{int64(2)}}) && plan.Access == "SCAN" && plan.Method == "" && len(plan.Indexes) == 0, "explicit-unique-supported-scan", nil)`,
	"grams-index-and-forced-scan":      `r.require(reflect.DeepEqual(indexed, [][]any{{int64(-7)}, {int64(9)}}) && reflect.DeepEqual(indexed, scan) && plan.Method == "grams13" && plan.Access == "SEARCH" && reflect.DeepEqual(plan.Indexes, []string{"docs_grams"}) && scanPlan.Access == "SCAN" && scanPlan.Reason == "NOT INDEXED requested", "grams-index-and-forced-scan", nil)`,
	"dense-vector-exact-ledger":        `r.require(reflect.DeepEqual(vectors, [][]any{{int64(-7), "[1,0]", int64(2), float64(0)}, {int64(2), "[4,4]", int64(2), float64(5)}, {int64(9), "[0,0]", int64(2), float64(1)}}), "dense-vector-exact-ledger", nil)`,
}
var formatUpgradeCriticalCalls = map[string]bool{
	`runCase/r.ledger(ctx, o, "snapshot")`:                                                                                                                       true,
	`runCase/r.ledger(ctx, o, "after-unique-refusal")`:                                                                                                           true,
	`runCase/r.ledger(ctx, walOwner, "wal-reopened")`:                                                                                                            true,
	`runCase/r.ledger(ctx, restoredOwner, "restored-snapshot")`:                                                                                                  true,
	`ledger/r.query(ctx, o, label+"-rows", "SELECT id,token,body,weight,payload,nullable FROM docs ORDER BY id")`:                                                true,
	`ledger/r.query(ctx, o, label+"-unique", "SELECT id FROM docs WHERE token='beta'")`:                                                                          true,
	`ledger/r.query(ctx, o, label+"-grams", "SELECT id FROM docs WHERE body LIKE '%数据库%' ORDER BY id")`:                                                          true,
	`ledger/r.query(ctx, o, label+"-scan", "SELECT id FROM docs NOT INDEXED WHERE body LIKE '%数据库%' ORDER BY id")`:                                               true,
	`ledger/r.query(ctx, o, label+"-vector", "SELECT id,vector_to_text(embedding),vector_dims(embedding),l2_distance(embedding,'[1,0]') FROM docs ORDER BY id")`: true,
	`ledger/o.db.Schema(ctx)`: true,
	`ledger/o.db.Check(ctx, sqlite.CheckOptions{MaxPages: 4096, MaxPageReads: 1 << 20})`: true,
}
var formatUpgradeCriticalStatements = map[string]bool{
	`ledger/want := [][]any{{int64(-7), "alpha", "数据库 alpha", float64(1.5), []byte{0, 1, 254}, nil}, {int64(2), "beta", "plain beta", float64(-2.25), []byte{2, 3}, "kept"}, {int64(9), "gamma", "数据库 gamma", float64(0), []byte{}, nil}}`: true,
	`preflight/wanted := map[string]string{"test.run": "^TestManualEncryptedSnapshotToWALUpgrade$", "test.count": "1", "test.timeout": "0s", "test.parallel": "1", "test.shuffle": "off", "test.v": "true"}`:                               true,
}
var formatUpgradeFirstStatements = map[string]string{
	"event":  `r.capturePending(err)`,
	"finish": `r.capturePending(nil)`,
	"retire": `if err := formatupgrade.CanRetire(o.state, r.states(), r.receipt.InternalPendingObserved); err != nil { o.state.Blocked = err.Error() return err }`,
}
