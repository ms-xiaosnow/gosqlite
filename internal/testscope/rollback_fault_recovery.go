package main

// This source-only boundary owns one Linux manual entry and two descriptor-free
// receipt sources. It is not a runner, a physical result or permission to run.
// Source declarations below are proposals requiring independent review; they
// are never learned from the files under inspection. Supplemental byte pins do
// not replace declaration, caller, argument and sequencing checks.
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

const rollbackFaultRecoveryNative = "internal/pager/rollback_fault_recovery_manual_linux_test.go"
const rollbackFaultRecoveryReceipt = "internal/testkit/rollbackfault/recovery_receipt.go"
const rollbackFaultRecoveryReceiptTest = "internal/testkit/rollbackfault/recovery_receipt_test.go"
const rollbackFaultRecoveryEntry = "TestManualRollbackFaultRecovery"

// This cross-facility admission supplements the recovery scanner's closed
// declarations. A path or manual tag alone cannot authorize a native consumer.
func rollbackFaultRecoveryNativePinned(path string, raw []byte) bool {
	path = filepath.ToSlash(filepath.Clean(path))
	return path == rollbackFaultRecoveryNative && strings.SplitN(string(raw), "\n", 2)[0] == rollbackFaultManualTag && fmt.Sprintf("%x", sha256.Sum256(raw)) == rollbackFaultRecoveryPins[path]
}

// Shared environment ownership is limited to four unchanged, pinned inputs.
// The restart half stays with its guard's private source namespace.
func rollbackFaultRecoverySharedManualPinned(path string, raw []byte) bool {
	path = filepath.ToSlash(filepath.Clean(path))
	if path == rollbackFaultSQLBridge || path == rollbackFaultSQLNative {
		return strings.SplitN(string(raw), "\n", 2)[0] == rollbackFaultManualTag && fmt.Sprintf("%x", sha256.Sum256(raw)) == rollbackFaultSQLPins[path]
	}
	return restartGuardRecoveryManualPinned(path, raw)
}

func rollbackFaultRecoveryOwned(path string) bool {
	return path == rollbackFaultRecoveryNative || path == rollbackFaultRecoveryReceipt || path == rollbackFaultRecoveryReceiptTest
}

// This is the sole addition to the adapter's private-reference ownership. The
// closed native declaration contract below also binds every use of the report type
// and the injected cause. The registry and bridge internals remain forbidden.
func rollbackFaultRecoveryPrivateReference(path, name string) bool {
	return path == rollbackFaultRecoveryNative && (name == "rollbackFaultReport" || name == "errRollbackFaultInjected")
}

func rollbackFaultRecoveryDirectCall(path string, id *ast.Ident, parents map[ast.Node]ast.Node, set *token.FileSet, raw []byte, function, expected string) bool {
	if filepath.ToSlash(filepath.Clean(path)) != rollbackFaultRecoveryNative {
		return false
	}
	call := rollbackFaultDirectReference(id, parents)
	if call == nil || rollbackFaultText(set, raw, call) != expected {
		return false
	}
	fn := oracleEnclosingFunction(call, parents)
	if fn == nil || fn.Name.Name != function || fn.Recv != nil {
		return false
	}
	for n := parents[call]; n != nil && n != fn; n = parents[n] {
		switch n.(type) {
		case *ast.ForStmt, *ast.RangeStmt, *ast.FuncLit, *ast.DeferStmt, *ast.GoStmt:
			return false
		}
	}
	return true
}

func rollbackFaultRecoveryScopeCall(path string, id *ast.Ident, parents map[ast.Node]ast.Node, set *token.FileSet, raw []byte) bool {
	want := map[string]string{
		"armRollbackFault":    "armRollbackFault(tx, plan)",
		"finishRollbackFault": "finishRollbackFault(scope, commitErr)",
		"disarmRollbackFault": "disarmRollbackFault(scope)",
	}
	call, exists := want[id.Name]
	return exists && rollbackFaultRecoveryDirectCall(path, id, parents, set, raw, "manualRollbackFaultCommit", call)
}

func rollbackFaultRecoveryAllowedRecover(path string, id *ast.Ident, parents map[ast.Node]ast.Node, set *token.FileSet, raw []byte) bool {
	return id.Name == "RecoverRollback" && rollbackFaultRecoveryDirectCall(path, id, parents, set, raw, "manualRollbackFaultRecoverCopy", rollbackFaultRecoveryRecoverCall)
}

func inspectRollbackFaultRecoverySource(path string, raw []byte) ([]string, error) {
	path = filepath.ToSlash(filepath.Clean(path))
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, path, raw, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	var findings []string
	report := func(message string) { findings = append(findings, path+": rollback fault recovery contract: "+message) }
	owned := rollbackFaultRecoveryOwned(path)
	native := path == rollbackFaultRecoveryNative
	pure := path == rollbackFaultRecoveryReceipt || path == rollbackFaultRecoveryReceiptTest
	parents := oracleParents(file)
	if strings.HasPrefix(path, "internal/pager/rollback_fault_recovery") && !native {
		report("unreviewed recovery harness source path")
	}
	if strings.HasPrefix(path, "internal/testkit/rollbackfault/recovery_") && !pure {
		report("unreviewed recovery receipt source path")
	}
	if owned {
		if want, ok := rollbackFaultRecoveryPins[path]; !ok || fmt.Sprintf("%x", sha256.Sum256(raw)) != want {
			report("proposed complete-source pin changed")
		}
		if shape, ok := rollbackFaultRecoveryShapes[path]; ok {
			findings = append(findings, rollbackFaultDeclarationShape(path, raw, set, file, shape)...)
		} else {
			report("missing closed declaration contract")
		}
		if native {
			if file.Name.Name != "pager" || strings.SplitN(string(raw), "\n", 2)[0] != rollbackFaultManualTag || strings.Count(string(raw), "//go:build") != 1 || strings.Contains(string(raw), "// +build") {
				report("exact native package and Linux AND manual build gate required")
			}
		} else if file.Name.Name != "rollbackfault" || strings.Contains(string(raw), "//go:build") || strings.Contains(string(raw), "// +build") {
			report("portable receipt package must have no build constraints")
		}
	}
	imports := map[string]string{}
	for _, imp := range file.Imports {
		value, unquoteErr := strconv.Unquote(imp.Path.Value)
		if unquoteErr != nil {
			return nil, unquoteErr
		}
		name := filepath.Base(value)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		imports[name] = value
		if owned && imp.Name != nil {
			report("aliased, dot or blank import outside closed source binding")
		}
		if owned && (value == "unsafe" || value == "reflect" || value == "os/exec" || (value == "syscall" && !native) || value == "time" || strings.HasPrefix(value, "net") || value == "golang.org/x/sys/unix" || value == "github.com/ms-xiaosnow/gosqlite/sqlite" || value == "github.com/ms-xiaosnow/gosqlite/internal/btree") {
			report("process/network/clock/unsafe or cyclic native import forbidden")
		}
	}
	if path == rollbackFaultRecoveryReceipt && (len(imports) != 1 || imports["sha256"] != "crypto/sha256") {
		report("descriptor-free recovery receipt permits only SHA-256 import")
	}
	if path == rollbackFaultRecoveryReceiptTest && (len(imports) != 1 || imports["testing"] != "testing") {
		report("recovery receipt tests require only testing import")
	}
	entries := 0
	functions := map[string]*ast.FuncDecl{}
	for _, declaration := range file.Decls {
		if gen, ok := declaration.(*ast.GenDecl); ok && owned && gen.Tok == token.VAR {
			report("owned sources cannot add mutable package state")
		}
		fn, ok := declaration.(*ast.FuncDecl)
		if !ok {
			continue
		}
		functions[fn.Name.Name] = fn
		if owned && (fn.Name.Name == "init" || fn.Name.Name == "TestMain") {
			report("initializer or TestMain forbidden")
		}
		entry := strings.HasPrefix(fn.Name.Name, "Test") || strings.HasPrefix(fn.Name.Name, "Benchmark") || strings.HasPrefix(fn.Name.Name, "Fuzz") || strings.HasPrefix(fn.Name.Name, "Example")
		if native && entry {
			entries++
			if fn.Name.Name != rollbackFaultRecoveryEntry || fn.Recv != nil || fn.Type.TypeParams != nil || fn.Type.Results != nil || rollbackFaultText(set, raw, fn.Type.Params) != "(t *testing.T)" {
				report("one exact manual test entry required")
			}
		}
		if native && !entry && fn.Recv == nil && !strings.HasPrefix(fn.Name.Name, "manualRollbackFault") {
			report("helper declaration outside exact private prefix")
		}
	}
	counts := map[string]int{}
	ast.Inspect(file, func(node ast.Node) bool {
		if literal, ok := node.(*ast.BasicLit); ok && literal.Kind == token.STRING {
			value, unquoteErr := strconv.Unquote(literal.Value)
			_, policyComparison := parents[literal].(*ast.BinaryExpr)
			policyComparison = policyComparison && (path == "internal/testscope/rollback_fault.go" || path == "internal/testscope/rollback_fault_recovery.go")
			if unquoteErr == nil && !policyComparison && ((strings.HasPrefix(value, "GOSQLITE_ROLLBACK_FAULT_") && len(value) > len("GOSQLITE_ROLLBACK_FAULT_") && !native) || (value == "GOSQLITE_MANUAL_FAULT_ROLLBACK" && !native && path != rollbackFaultManual && !rollbackFaultRecoverySharedManualPinned(path, raw))) {
				report("manual environment identity outside exact native/adapter owner")
			}
		}
		if id, ok := node.(*ast.Ident); ok {
			if id.Name == rollbackFaultRecoveryEntry || strings.HasPrefix(id.Name, "manualRollbackFault") {
				if !native {
					report("native entry/helper reference outside exact source")
				} else if fn, declared := parents[id].(*ast.FuncDecl); declared && fn.Name == id {
					// Closed declaration inventory checks the exact declaration.
				} else if typ, declared := parents[id].(*ast.TypeSpec); declared && typ.Name == id {
					// Types are also frozen in the closed declaration inventory.
				} else if id.Name == rollbackFaultRecoveryEntry {
					report("manual entry cannot be invoked, aliased or wrapped")
				}
			}
			if native && strings.HasPrefix(id.Name, "manualRollbackFault") {
				if helper := functions[id.Name]; helper != nil && helper.Recv == nil {
					if declaration, ok := parents[id].(*ast.FuncDecl); !ok || declaration.Name != id {
						call := rollbackFaultDirectReference(id, parents)
						if call == nil {
							report("native helper cannot escape as alias or function value")
						} else {
							counts[id.Name]++
							if id.Name == "manualRollbackFaultCommit" || id.Name == "manualRollbackFaultRecoverCopy" || id.Name == "manualRollbackFaultReadback" || id.Name == "manualRollbackFaultImages" {
								caller := oracleEnclosingFunction(call, parents)
								if caller == nil || caller.Name.Name != rollbackFaultRecoveryEntry {
									report("single-trial helper must be directly bound to the unique entry")
								}
								for parent := parents[call]; parent != nil && parent != caller; parent = parents[parent] {
									switch parent.(type) {
									case *ast.ForStmt, *ast.RangeStmt, *ast.FuncLit, *ast.DeferStmt, *ast.GoStmt:
										report("single-trial helper cannot be looped, deferred or wrapped")
									}
								}
							}
						}
					}
				}
			}
			if native && (id.Name == "armRollbackFault" || id.Name == "finishRollbackFault" || id.Name == "disarmRollbackFault") {
				if !rollbackFaultRecoveryScopeCall(path, id, parents, set, raw) {
					report("scope calls require exact direct helper/arguments without loop or closure")
				} else {
					counts[id.Name]++
				}
			}
			if native && id.Name == "RecoverRollback" {
				if !rollbackFaultRecoveryAllowedRecover(path, id, parents, set, raw) {
					report("recovery must directly target fresh replay with exact bounded options")
				} else {
					counts[id.Name]++
				}
			}
			if native && (id.Name == "RetryPlaintextCleanup" || id.Name == "TestMain") {
				report("cleanup retry or alternate entry forbidden")
			}
		}
		if !owned {
			return true
		}
		switch node.(type) {
		case *ast.GoStmt, *ast.SendStmt:
			report("concurrent execution forbidden")
		}
		if sel, ok := node.(*ast.SelectorExpr); ok {
			if qualifier, ok := sel.X.(*ast.Ident); ok && imports[qualifier.Name] == "syscall" && (!native || sel.Sel.Name != "Stat_t") {
				report("syscall import is metadata-only Stat_t, never an execution call")
			}
			switch sel.Sel.Name {
			case "TempDir", "CreateTemp", "MkdirTemp", "Remove", "RemoveAll", "Rename", "Symlink", "Link", "Truncate", "Chmod", "Exit", "Command", "CommandContext", "Kill", "Signal", "StartProcess", "Setenv", "Chdir", "Parallel", "Skip", "Skipf", "SkipNow":
				report("unreviewed cleanup/process/selection reference " + sel.Sel.Name)
			}
			if native && sel.Sel.Name == "Run" {
				report("native subtests forbidden")
			}
			if native && sel.Sel.Name == "Commit" {
				call, direct := parents[sel].(*ast.CallExpr)
				fn := oracleEnclosingFunction(sel, parents)
				receipt, receiptRead := parents[sel].(*ast.AssignStmt)
				if receiptRead && receipt.Tok == token.ASSIGN && len(receipt.Lhs) == 1 && len(receipt.Rhs) == 1 && receipt.Rhs[0] == sel && fn != nil && fn.Name.Name == rollbackFaultRecoveryEntry && fn.Recv == nil && rollbackFaultText(set, raw, receipt) == "evidence.CommitReturn = report.Events[i].Commit" {
					// This exact receipt field read is descriptor-free. Its closed
					// declaration binds the surrounding loop and CommitReturned guard;
					// it is not the protected transaction call and must not count as one.
				} else if !direct || call.Fun != sel || fn == nil || fn.Name.Name != "manualRollbackFaultCommit" || rollbackFaultText(set, raw, call) != "tx.Commit(ctx)" {
					report("Commit must be the direct original tx call")
				} else {
					counts["Commit"]++
				}
			}
			if pure {
				switch sel.Sel.Name {
				case "Getenv", "LookupEnv", "Environ", "Open", "OpenFile", "Create", "ReadFile", "WriteFile", "Close", "Sync", "Begin", "RecoverRollback", "RetryPlaintextCleanup":
					report("non-memory recovery receipt reference " + sel.Sel.Name)
				}
				if call, direct := parents[sel].(*ast.CallExpr); direct && call.Fun == sel && sel.Sel.Name == "Commit" {
					report("non-memory recovery receipt Commit call")
				}
			}
		}
		return true
	})
	if native {
		if entries != 1 {
			report("one exact manual test entry required")
		}
		entry := functions[rollbackFaultRecoveryEntry]
		if entry == nil || entry.Body == nil || len(entry.Body.List) == 0 || rollbackFaultShapeTokens(oracleNodeText(set, raw, entry.Body.List[0])) != rollbackFaultShapeTokens(rollbackFaultRecoveryFirstGate) {
			report("manual entry first action must be exact fatal runtime gate")
		}
		for _, name := range []string{"armRollbackFault", "Commit", "finishRollbackFault", "disarmRollbackFault", "RecoverRollback", "manualRollbackFaultCommit", "manualRollbackFaultRecoverCopy", "manualRollbackFaultReadback", "manualRollbackFaultImages"} {
			if counts[name] != 1 {
				report("missing/duplicate protected native call " + name)
			}
		}
		findings = append(findings, inspectRollbackFaultRecoveryCommitSequence(path, raw, set, functions["manualRollbackFaultCommit"])...)
	}
	if path == rollbackFaultRecoveryReceipt {
		var valueOnly func(ast.Expr) bool
		valueOnly = func(expression ast.Expr) bool {
			switch typ := expression.(type) {
			case *ast.Ident:
				return typ.Name != "error" && typ.Name != "any" && typ.Name != "uintptr" && typ.Name != "unsafe"
			case *ast.ArrayType:
				return typ.Len != nil && valueOnly(typ.Elt)
			case *ast.StructType:
				for _, field := range typ.Fields.List {
					if !valueOnly(field.Type) {
						return false
					}
				}
				return true
			}
			return false
		}
		physical := map[string]int{}
		for _, declaration := range file.Decls {
			if gen, ok := declaration.(*ast.GenDecl); ok {
				for _, spec := range gen.Specs {
					if typ, ok := spec.(*ast.TypeSpec); ok && (typ.Assign.IsValid() || !valueOnly(typ.Type)) {
						report("recovery receipt types must be scalar/fixed-array value records")
					}
				}
			}
			if fn, ok := declaration.(*ast.FuncDecl); ok && (fn.Name.Name == "Pass" || fn.Name.Name == "PhysicalValidated") {
				physical[fn.Name.Name]++
				if rollbackFaultText(set, raw, fn) != "func (RecoveryObservation) "+fn.Name.Name+"() bool { return false }" {
					report("recovery observation cannot confer physical validation or PASS")
				}
			}
		}
		if physical["Pass"] != 1 || physical["PhysicalValidated"] != 1 {
			report("two explicit false physical-acceptance methods required")
		}
	}
	return findings, nil
}

func inspectRollbackFaultRecoveryCommitSequence(path string, raw []byte, set *token.FileSet, fn *ast.FuncDecl) []string {
	var findings []string
	report := func(message string) { findings = append(findings, path+": rollback fault recovery contract: "+message) }
	if fn == nil || fn.Body == nil {
		report("missing native Commit helper")
		return findings
	}
	commit := -1
	for i, statement := range fn.Body.List {
		if rollbackFaultText(set, raw, statement) == "commitErr := tx.Commit(ctx)" {
			if commit != -1 {
				report("duplicate actual Commit statement")
			}
			commit = i
		}
	}
	if commit < 2 || rollbackFaultText(set, raw, fn.Body.List[commit-2]) != "scope, armErr := armRollbackFault(tx, plan)" || rollbackFaultText(set, raw, fn.Body.List[commit-1]) != "if armErr != nil { return rollbackFaultReport{}, manualRollbackFaultAbandon(tx, armErr) }" {
		report("arm admission and its failure-only branch must immediately precede actual Commit")
	}
	if commit < 0 || commit+1 >= len(fn.Body.List) || rollbackFaultText(set, raw, fn.Body.List[commit+1]) != "report := finishRollbackFault(scope, commitErr)" {
		report("actual Commit and original-return finish must be adjacent in same lexical scope")
	}
	return findings
}

func inspectRollbackFaultRecoveryContracts(root string) ([]string, error) {
	var findings []string
	for _, path := range []string{rollbackFaultRecoveryNative, rollbackFaultRecoveryReceipt, rollbackFaultRecoveryReceiptTest} {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return nil, err
		}
		items, err := inspectRollbackFaultRecoverySource(path, raw)
		if err != nil {
			return nil, err
		}
		findings = append(findings, items...)
	}
	return findings, nil
}

// These are proposed source identities, not a release of the existing main.go
// WAL policy pin. No scanner derives its approved shape from inspected files.
const rollbackFaultRecoveryRecoverCall = `RecoverRollback(ctx, replayPath, RecoveryOptions{Limits: journal.ReplayLimits{MaxJournalBytes: 8200, MaxRecords: 1, MaxDatabaseBytes: 4096}})`
const rollbackFaultRecoveryFirstGate = `if os.Getenv("GOSQLITE_MANUAL_FAULT_ROLLBACK") != "independently_authorized" {
		t.Fatal("NOT_RUN: separately authorized execution, reviewed provenance and private retained root are required")
	}`

var rollbackFaultRecoveryPins = map[string]string{
	rollbackFaultRecoveryNative:      "8ae225c5937801cbd952d018ed9f9deb2342beae9790a4aafa3e48968021e04f",
	rollbackFaultRecoveryReceipt:     "8723cb3a32b0bb1e8a8563c26b07f109818604327263350000acb1e77cf13104",
	rollbackFaultRecoveryReceiptTest: "282c6a0c91dea4e41793ab19d235091904b897640e9ffb7d3f1c1e231382b3eb",
}

var rollbackFaultRecoveryShapes = map[string]string{
	rollbackFaultRecoveryNative: `//go:build linux && manual_fault_rollback

package pager

// SOURCE ONLY / MANUAL PHYSICAL NOT_RUN. One process admits one fixed case.
// A private, stable namespace and independent execution/provenance approval are
// prerequisites. Lstat/SameFile checks detect changes; they do not defend against
// an adversary replacing names concurrently. No input is removed by this file.
import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/journal"
	"github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackfault"
)

// Every descriptor returned to this layer, even with a simultaneous error,
// enters this ledger before the error is examined. Closed entries are retained.
// A close error is terminal/unknown: it is recorded and never retried.
type manualRollbackFaultOwner struct {
	file     *os.File
	reader   *Reader
	path     string
	closes   uint8
	closeErr error
}

type manualRollbackFaultLedger struct {
	owners [96]manualRollbackFaultOwner
	count  int
}

func (l *manualRollbackFaultLedger) reserve() (int, error) {
	if l.count == len(l.owners) {
		return 0, fmt.Errorf("owner ledger full")
	}
	i := l.count
	l.count++
	return i, nil
}

func (l *manualRollbackFaultLedger) close(i int) error {
	o := &l.owners[i]
	if o.closes != 0 {
		return fmt.Errorf("duplicate close refused: %s", o.path)
	}
	if o.file == nil && o.reader == nil {
		return nil
	}
	o.closes++
	if o.file != nil {
		o.closeErr = o.file.Close()
	} else {
		o.closeErr = o.reader.Close()
	}
	return o.closeErr
}

func (l *manualRollbackFaultLedger) retired() bool {
	for i := 0; i < l.count; i++ {
		o := l.owners[i]
		if (o.file != nil || o.reader != nil) && (o.closes != 1 || o.closeErr != nil) {
			return false
		}
	}
	return true
}

func (l *manualRollbackFaultLedger) finish(t *testing.T) {
	for i := 0; i < l.count; i++ {
		o := &l.owners[i]
		if o.closes == 0 && (o.file != nil || o.reader != nil) {
			if err := l.close(i); err != nil {
				t.Errorf("retained owner close: %s: %v", o.path, err)
			}
		}
		if o.closeErr != nil {
			t.Errorf("owner retirement unknown; no retry: %s: %v", o.path, o.closeErr)
		}
	}
}

type manualRollbackFaultBinding struct {
	Source             string
	Tool               string
	Binary             string
	Manifest           string
	CompileReceipt     string
	Trial              string
	Scope              string
	Case               string
	ToolPath           string
	SourceManifestPath string
	CompileReceiptPath string
}

// An external reviewed compile receipt is a bounded evidence input, not an
// executable command or a self-authenticating approval. Its exact bytes must
// have been reviewed independently and its hash supplied by that review.
type manualRollbackFaultCompile struct {
	Schema      string
	Source      string
	Tool        string
	Binary      string
	Manifest    string
	Package     string
	Tags        string
	CompileOnly bool
	Files       uint16
	SourceTree  string
}

type manualRollbackFaultConfig struct {
	root          string
	rootInfo      os.FileInfo
	caseID        rollbackfault.RecoveryCase
	binding       manualRollbackFaultBinding
	bindingBytes  []byte
	sourceFiles   uint16
	ledger        manualRollbackFaultLedger
	retained      [12]manualRollbackFaultSaved
	retainedCount int
}

type manualRollbackFaultSaved struct {
	name string
	info os.FileInfo
	hash [32]byte
	size int64
}

func manualRollbackFaultHash(value string) ([32]byte, error) {
	var out [32]byte
	if len(value) != 64 {
		return out, fmt.Errorf("SHA-256 requires 64 lowercase hex bytes")
	}
	data, err := hex.DecodeString(value)
	if err != nil || hex.EncodeToString(data) != value {
		return out, fmt.Errorf("invalid SHA-256")
	}
	copy(out[:], data)
	if out == ([32]byte{}) {
		return out, fmt.Errorf("zero SHA-256")
	}
	return out, nil
}

func manualRollbackFaultNamespace(root string) (os.FileInfo, error) {
	if !filepath.IsAbs(root) || filepath.Clean(root) != root || root == "/" {
		return nil, fmt.Errorf("absolute clean private root required")
	}
	current := string(filepath.Separator)
	for _, component := range strings.Split(strings.TrimPrefix(root, "/"), "/") {
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
			return nil, fmt.Errorf("symlink/non-directory component: %s", current)
		}
	}
	info, err := os.Lstat(root)
	if err != nil {
		return nil, err
	}
	metadata, ok := info.Sys().(*syscall.Stat_t)
	if !ok || metadata.Uid != uint32(os.Getuid()) {
		return nil, fmt.Errorf("private root must belong to the invoking user")
	}
	if info.Mode().Perm() != 0700 {
		return nil, fmt.Errorf("precreated private root must already have mode 0700; no permission changes are made")
	}
	return info, nil
}

func (c *manualRollbackFaultConfig) stable() error {
	info, err := manualRollbackFaultNamespace(c.root)
	if err != nil {
		return err
	}
	if !os.SameFile(info, c.rootInfo) {
		return fmt.Errorf("root namespace changed; retain everything")
	}
	return nil
}

func manualRollbackFaultRead(l *manualRollbackFaultLedger, path string, limit int64) ([]byte, os.FileInfo, error) {
	before, err := os.Lstat(path)
	if err != nil {
		return nil, nil, err
	}
	if !before.Mode().IsRegular() || before.Size() < 1 || before.Size() > limit {
		return nil, nil, fmt.Errorf("bounded regular input required: %s", path)
	}
	i, err := l.reserve()
	if err != nil {
		return nil, nil, err
	}
	f, openErr := os.Open(path)
	l.owners[i] = manualRollbackFaultOwner{file: f, path: path}
	if openErr != nil {
		return nil, nil, errors.Join(openErr, l.close(i))
	}
	actual, statErr := f.Stat()
	if statErr != nil || !os.SameFile(before, actual) || actual.Size() != before.Size() {
		return nil, nil, errors.Join(fmt.Errorf("input changed on open: %s", path), statErr, l.close(i))
	}
	data, readErr := io.ReadAll(io.LimitReader(f, limit+1))
	after, afterErr := f.Stat()
	closeErr := l.close(i)
	pathAfter, pathErr := os.Lstat(path)
	if err := errors.Join(readErr, afterErr, closeErr, pathErr); err != nil {
		return nil, nil, err
	}
	if int64(len(data)) != before.Size() || !os.SameFile(before, after) || !os.SameFile(before, pathAfter) || after.Size() != before.Size() || pathAfter.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) || !pathAfter.ModTime().Equal(before.ModTime()) {
		return nil, nil, fmt.Errorf("input changed during bounded read: %s", path)
	}
	return data, before, nil
}

func manualRollbackFaultDecode(raw []byte, value any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err != nil {
		return err
	}
	var tail any
	if decoder.Decode(&tail) != io.EOF {
		return fmt.Errorf("trailing JSON value")
	}
	return nil
}

func manualRollbackFaultAdmit(c *manualRollbackFaultConfig) error {
	// Exact flags are intentional. No regex union, repeated test, subtest,
	// random order, competing test, timeout kill, or wrapper is admitted.
	want := map[string]string{
		"-test.run": "^TestManualRollbackFaultRecovery$", "-test.count": "1",
		"-test.shuffle": "off", "-test.parallel": "1", "-test.timeout": "0", "-test.cpu": "1",
		"-test.v": "true", // The unique successful terminal uses t.Logf.
	}
	seen := map[string]bool{}
	for _, arg := range os.Args[1:] {
		key, value, found := strings.Cut(arg, "=")
		if !found || seen[key] {
			return fmt.Errorf("explicit unique equals-form test flags required")
		}
		seen[key] = true
		if expected, ok := want[key]; !ok || value != expected {
			return fmt.Errorf("unreviewed test flag: %s", arg)
		}
	}
	for key := range want {
		if !seen[key] {
			return fmt.Errorf("missing exact flag: %s", key)
		}
	}
	if os.Getenv("GOSQLITE_ROLLBACK_FAULT_PRIVATE_ROOT_CONFIRMED") != "private_stable_namespace_retained" {
		return fmt.Errorf("private stable namespace and retention must be explicitly confirmed")
	}
	c.root = os.Getenv("GOSQLITE_ROLLBACK_FAULT_ROOT")
	info, err := manualRollbackFaultNamespace(c.root)
	if err != nil {
		return err
	}
	c.rootInfo = info
	i, err := c.ledger.reserve()
	if err != nil {
		return err
	}
	dir, openErr := os.Open(c.root)
	c.ledger.owners[i] = manualRollbackFaultOwner{file: dir, path: c.root}
	if openErr != nil {
		return errors.Join(openErr, c.ledger.close(i))
	}
	entries, readErr := dir.ReadDir(1)
	closeErr := c.ledger.close(i)
	if readErr != io.EOF || len(entries) != 0 || closeErr != nil {
		return errors.Join(fmt.Errorf("root must be empty before admission"), readErr, closeErr)
	}
	bindingPath := os.Getenv("GOSQLITE_ROLLBACK_FAULT_BINDING")
	bindingHash, err := manualRollbackFaultHash(os.Getenv("GOSQLITE_ROLLBACK_FAULT_BINDING_SHA256"))
	if err != nil {
		return err
	}
	raw, _, err := manualRollbackFaultRead(&c.ledger, bindingPath, 65536)
	if err != nil {
		return err
	}
	if sha256.Sum256(raw) != bindingHash {
		return fmt.Errorf("independently reviewed binding hash mismatch")
	}
	if err := manualRollbackFaultDecode(raw, &c.binding); err != nil {
		return err
	}
	c.bindingBytes = raw
	switch c.binding.Case {
	case "control":
		c.caseID = rollbackfault.RecoveryControl
	case "page-prefix-64":
		c.caseID = rollbackfault.RecoveryPagePrefix64
	case "db-sync-before":
		c.caseID = rollbackfault.RecoveryDBSyncBefore
	default:
		return fmt.Errorf("exactly one of three fixed cases required")
	}
	manifest, _, err := manualRollbackFaultRead(&c.ledger, c.binding.SourceManifestPath, 4<<20)
	if err != nil {
		return err
	}
	manifestHash, err := manualRollbackFaultHash(c.binding.Manifest)
	if err != nil || sha256.Sum256(manifest) != manifestHash || c.binding.Source != c.binding.Manifest {
		return fmt.Errorf("frozen source inventory digest mismatch: %v", err)
	}
	var inventory struct {
		Files map[string]struct {
			Bytes   int64  ` + "`" + `json:"bytes"` + "`" + `
			GitBlob string ` + "`" + `json:"git_blob"` + "`" + `
			GitMode string ` + "`" + `json:"git_mode"` + "`" + `
			SHA256  string ` + "`" + `json:"sha256"` + "`" + `
		} ` + "`" + `json:"files"` + "`" + `
		Tree string ` + "`" + `json:"tree"` + "`" + `
	}
	if err := manualRollbackFaultDecode(manifest, &inventory); err != nil {
		return err
	}
	if len(inventory.Files) == 0 || len(inventory.Files) > 4096 || len(inventory.Tree) != 40 {
		return fmt.Errorf("finite reviewed source inventory required")
	}
	for path, entry := range inventory.Files {
		if filepath.IsAbs(path) || filepath.ToSlash(filepath.Clean(path)) != path || strings.HasPrefix(path, "../") || entry.Bytes < 0 || (entry.GitMode != "100644" && entry.GitMode != "100755") || len(entry.GitBlob) != 40 {
			return fmt.Errorf("invalid reviewed source inventory entry")
		}
		if _, err := manualRollbackFaultHash(entry.SHA256); err != nil {
			return err
		}
	}
	c.sourceFiles = uint16(len(inventory.Files))
	compileRaw, _, err := manualRollbackFaultRead(&c.ledger, c.binding.CompileReceiptPath, 65536)
	if err != nil {
		return err
	}
	compileHash, err := manualRollbackFaultHash(c.binding.CompileReceipt)
	if err != nil || sha256.Sum256(compileRaw) != compileHash {
		return fmt.Errorf("independently reviewed compile receipt digest mismatch: %v", err)
	}
	var receipt manualRollbackFaultCompile
	if err := manualRollbackFaultDecode(compileRaw, &receipt); err != nil {
		return err
	}
	if receipt.Schema != "rollback-fault-compile-v1" || receipt.Source != c.binding.Source || receipt.Tool != c.binding.Tool || receipt.Binary != c.binding.Binary || receipt.Manifest != c.binding.Manifest || receipt.Package != "./internal/pager" || receipt.Tags != "manual_fault_rollback" || !receipt.CompileOnly || receipt.Files != c.sourceFiles || receipt.SourceTree != inventory.Tree {
		return fmt.Errorf("source-to-binary compile receipt does not bind this trial")
	}
	tool, _, err := manualRollbackFaultRead(&c.ledger, c.binding.ToolPath, 128<<20)
	if err != nil {
		return err
	}
	toolHash, err := manualRollbackFaultHash(c.binding.Tool)
	if err != nil || sha256.Sum256(tool) != toolHash {
		return fmt.Errorf("tool digest mismatch: %v", err)
	}
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	image, _, err := manualRollbackFaultRead(&c.ledger, executable, 128<<20)
	if err != nil {
		return err
	}
	binaryHash, err := manualRollbackFaultHash(c.binding.Binary)
	if err != nil || sha256.Sum256(image) != binaryHash {
		return fmt.Errorf("actual executable digest mismatch: %v", err)
	}
	if _, err := manualRollbackFaultHash(c.binding.Source); err != nil {
		return err
	}
	return c.stable()
}

func manualRollbackFaultImages() (old, next, partial [4096]byte) {
	copy(old[:], "SQLite format 3\x00")
	binary.BigEndian.PutUint16(old[16:], 4096)
	old[18], old[19], old[21], old[22], old[23] = 1, 1, 64, 32, 32
	binary.BigEndian.PutUint32(old[24:], 7)
	binary.BigEndian.PutUint32(old[28:], 1)
	binary.BigEndian.PutUint32(old[40:], 9)
	binary.BigEndian.PutUint32(old[44:], 4)
	binary.BigEndian.PutUint32(old[56:], 1)
	binary.BigEndian.PutUint32(old[92:], 7)
	old[100] = 13
	binary.BigEndian.PutUint16(old[105:], 4096)
	for i := 108; i < len(old); i++ {
		old[i] = byte((i*17 + 23) % 251)
	}
	next = old
	binary.BigEndian.PutUint32(next[24:], 8)
	binary.BigEndian.PutUint32(next[40:], 10)
	binary.BigEndian.PutUint32(next[92:], 8)
	partial = old
	copy(partial[:64], next[:64])
	return old, next, partial
}

func (c *manualRollbackFaultConfig) create(name string, data []byte, retain bool) (os.FileInfo, error) {
	if err := c.stable(); err != nil {
		return nil, err
	}
	if filepath.Base(name) != name || name == "." || len(data) == 0 || len(data) > 1<<20 {
		return nil, fmt.Errorf("fixed bounded basename required")
	}
	path := filepath.Join(c.root, name)
	i, err := c.ledger.reserve()
	if err != nil {
		return nil, err
	}
	f, openErr := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	c.ledger.owners[i] = manualRollbackFaultOwner{file: f, path: path}
	if openErr != nil {
		return nil, errors.Join(openErr, c.ledger.close(i))
	}
	info, statErr := f.Stat()
	if statErr != nil {
		return nil, errors.Join(statErr, c.ledger.close(i))
	}
	for n := 0; n < c.retainedCount; n++ {
		if os.SameFile(info, c.retained[n].info) {
			return nil, errors.Join(fmt.Errorf("retained input alias"), c.ledger.close(i))
		}
	}
	n, writeErr := f.Write(data)
	if writeErr == nil && n != len(data) {
		writeErr = io.ErrShortWrite
	}
	var syncErr error
	if writeErr == nil {
		syncErr = f.Sync()
	}
	closeErr := c.ledger.close(i)
	if err := errors.Join(writeErr, syncErr, closeErr); err != nil {
		return nil, err
	}
	actual, actualInfo, err := manualRollbackFaultRead(&c.ledger, path, int64(len(data)))
	if err != nil {
		return nil, err
	}
	if !os.SameFile(info, actualInfo) || !bytes.Equal(actual, data) {
		return nil, fmt.Errorf("created bytes or identity changed")
	}
	if retain {
		if c.retainedCount == len(c.retained) {
			return nil, fmt.Errorf("retained inventory full")
		}
		c.retained[c.retainedCount] = manualRollbackFaultSaved{name: name, info: actualInfo, hash: sha256.Sum256(actual), size: int64(len(actual))}
		c.retainedCount++
	}
	return actualInfo, c.stable()
}

func (c *manualRollbackFaultConfig) retainedUnchanged() error {
	if err := c.stable(); err != nil {
		return err
	}
	for i := 0; i < c.retainedCount; i++ {
		saved := c.retained[i]
		raw, info, err := manualRollbackFaultRead(&c.ledger, filepath.Join(c.root, saved.name), saved.size)
		if err != nil {
			return err
		}
		if !os.SameFile(info, saved.info) || info.Size() != saved.size || sha256.Sum256(raw) != saved.hash {
			return fmt.Errorf("retained input changed: %s", saved.name)
		}
	}
	return nil
}

func manualRollbackFaultPlan(c *manualRollbackFaultConfig) (rollbackfault.Plan, error) {
	b := c.binding
	spec := rollbackfault.PlanSpec{
		Identity:  rollbackfault.Identity{Suite: "rollback-fault-recovery-v1", Source: b.Source, Tool: b.Tool, Binary: b.Binary, Manifest: b.Manifest, Trial: b.Trial, Scope: b.Scope, Profile: rollbackfault.PlainRollback4096},
		MainOwner: 11, JournalOwner: 12, DirectoryOwner: 13, CleanupOwner: 14,
		OriginalPages: 1, DirtyPages: 1, PageSize: 4096, SectorSize: 4096,
		Action: rollbackfault.Pass, ExpectedCommit: rollbackfault.CommitSucceeded,
		Calls: [rollbackfault.MaxCalls]rollbackfault.Call{
			{Point: rollbackfault.JournalCreate, Occurrence: 1, Role: rollbackfault.Journal, Owner: 12},
			{Point: rollbackfault.JournalInvalid, Occurrence: 1, Role: rollbackfault.Journal, Owner: 12, Length: 4096},
			{Point: rollbackfault.DBOriginalRead, Occurrence: 1, Role: rollbackfault.Database, Owner: 11, Length: 4096},
			{Point: rollbackfault.JournalRecord, Occurrence: 1, Role: rollbackfault.Journal, Owner: 12, Offset: 4096, Length: 4104},
			{Point: rollbackfault.JournalBodySync, Occurrence: 1, Role: rollbackfault.Journal, Owner: 12},
			{Point: rollbackfault.JournalValidHeader, Occurrence: 1, Role: rollbackfault.Journal, Owner: 12, Length: 12},
			{Point: rollbackfault.JournalHeaderSync, Occurrence: 1, Role: rollbackfault.Journal, Owner: 12},
			{Point: rollbackfault.PublishDirectorySync, Occurrence: 1, Role: rollbackfault.Directory, Owner: 13},
			{Point: rollbackfault.DBPage, Occurrence: 1, Role: rollbackfault.Database, Owner: 11, Length: 4096},
			{Point: rollbackfault.DBSync, Occurrence: 1, Role: rollbackfault.Database, Owner: 11},
			{Point: rollbackfault.JournalUnlink, Occurrence: 1, Role: rollbackfault.Journal, Owner: 12},
			{Point: rollbackfault.FinalDirectorySync, Occurrence: 1, Role: rollbackfault.Directory, Owner: 13},
		},
		CallCount: 12,
	}
	spec.Target = spec.Calls[8]
	if c.caseID == rollbackfault.RecoveryPagePrefix64 {
		spec.Action, spec.Prefix, spec.CallCount, spec.ExpectedCommit = rollbackfault.PrefixShortWrite, 64, 9, rollbackfault.CommitRecoveryRequired
	}
	if c.caseID == rollbackfault.RecoveryDBSyncBefore {
		spec.Target, spec.Action, spec.CallCount, spec.ExpectedCommit = spec.Calls[9], rollbackfault.FailBefore, 10, rollbackfault.CommitRecoveryRequired
	}
	for i := int(spec.CallCount); i < len(spec.Calls); i++ {
		spec.Calls[i] = rollbackfault.Call{}
	}
	plan, problem := rollbackfault.NewPlan(spec)
	if problem != rollbackfault.NoProblem {
		return plan, fmt.Errorf("plan refused: %v", problem)
	}
	return plan, nil
}

func manualRollbackFaultRegistriesEmpty() bool {
	plainRetirements.mu.Lock()
	empty := len(plainRetirements.groups) == 0
	plainRetirements.mu.Unlock()
	return empty && len(PendingPlaintextCleanups()) == 0
}

// Before adapter ownership transfer, close only an untouched live preparation
// transaction. A started/pending/unknown retirement is preserved, never retried.
func manualRollbackFaultAbandon(tx *Tx, cause error) error {
	if tx == nil {
		return cause
	}
	tx.mu.Lock()
	g := tx.cleanup
	live := tx.file != nil && g != nil
	if g != nil {
		g.mu.Lock()
		live = live && !g.started && !g.done && g.cause == nil && g.pending.Load() == nil && len(g.nested) == 0
		g.mu.Unlock()
	}
	tx.mu.Unlock()
	if !live {
		return cause
	}
	return errors.Join(cause, tx.Rollback())
}

func manualRollbackFaultCommit(ctx context.Context, path string, plan rollbackfault.Plan) (rollbackFaultReport, error) {
	if !manualRollbackFaultRegistriesEmpty() {
		return rollbackFaultReport{}, fmt.Errorf("preexisting plain owner obligations")
	}
	tx, openErr := OpenTx(ctx, path, Options{PageSize: 4096, SectorSize: 4096, MaxDirtyPages: 1, MaxFreelistPages: 1, MaxUndoBytes: 16384})
	if openErr != nil {
		// A constructor-carried owner belongs to the production registry. Do
		// not retry its cleanup or infer retirement from a nil Tx.
		return rollbackFaultReport{}, manualRollbackFaultAbandon(tx, openErr)
	}
	if err := tx.BumpSchemaCookie(ctx); err != nil {
		return rollbackFaultReport{}, manualRollbackFaultAbandon(tx, err)
	}
	scope, armErr := armRollbackFault(tx, plan)
	if armErr != nil {
		return rollbackFaultReport{}, manualRollbackFaultAbandon(tx, armErr)
	}
	commitErr := tx.Commit(ctx)
	report := finishRollbackFault(scope, commitErr)
	if report.CommitError != commitErr || report.Problem != nil || report.Observation.Outcome != rollbackfault.AdapterObserved || report.Observation.TargetHits != 1 || report.Observation.Pass() || report.Observation.PhysicalValidated() {
		return report, fmt.Errorf("adapter incomplete; original error retained: %v; problem: %v", commitErr, report.Problem)
	}
	if plan.Spec().Action == rollbackfault.Pass {
		if commitErr != nil {
			return report, fmt.Errorf("control Commit: %w", commitErr)
		}
	} else {
		if !errors.Is(commitErr, ErrRecoveryRequired) || errors.Is(commitErr, ErrPlaintextCleanupPending) || errors.Is(commitErr, ErrCommitUncertain) {
			return report, fmt.Errorf("unqualified Commit failure: %v", commitErr)
		}
		if plan.Spec().Action == rollbackfault.PrefixShortWrite && !errors.Is(commitErr, io.ErrShortWrite) {
			return report, fmt.Errorf("original short-write cause missing")
		}
		if plan.Spec().Action == rollbackfault.FailBefore && !errors.Is(commitErr, errRollbackFaultInjected) {
			return report, fmt.Errorf("original injected cause missing")
		}
	}
	if err := disarmRollbackFault(scope); err != nil {
		return report, err
	}
	if !manualRollbackFaultRegistriesEmpty() {
		return report, fmt.Errorf("Commit owner retirement incomplete")
	}
	return report, nil
}

func manualRollbackFaultRecoverCopy(ctx context.Context, replayPath string) (RecoveryResult, error) {
	if filepath.Base(replayPath) != "replay.db" || !manualRollbackFaultRegistriesEmpty() {
		return RecoveryResult{}, fmt.Errorf("fresh replay path and empty registry required")
	}
	result, recoveryErr := RecoverRollback(ctx, replayPath, RecoveryOptions{Limits: journal.ReplayLimits{MaxJournalBytes: 8200, MaxRecords: 1, MaxDatabaseBytes: 4096}})
	if recoveryErr != nil {
		return result, recoveryErr
	}
	if !manualRollbackFaultRegistriesEmpty() {
		return result, fmt.Errorf("recovery retirement incomplete; no fabricated owner token")
	}
	if !result.Recovered || result.RestoredPages != 1 || result.OriginalPages != 1 {
		return result, fmt.Errorf("exact one-page actual recovery required")
	}
	return result, nil
}

func manualRollbackFaultReadback(c *manualRollbackFaultConfig, ctx context.Context, replayPath string, expected [4096]byte) ([]byte, Header, error) {
	before, err := os.Lstat(replayPath)
	if err != nil || before.Size() != 4096 {
		return nil, Header{}, fmt.Errorf("replay extent before Reader: %v", err)
	}
	i, err := c.ledger.reserve()
	if err != nil {
		return nil, Header{}, err
	}
	reader, openErr := Open(replayPath)
	c.ledger.owners[i] = manualRollbackFaultOwner{reader: reader, path: replayPath}
	if openErr != nil {
		return nil, Header{}, errors.Join(openErr, c.ledger.close(i))
	}
	page, readErr := reader.ReadPage(ctx, 1)
	header := reader.Header()
	closeErr := c.ledger.close(i)
	if err := errors.Join(readErr, closeErr); err != nil {
		return nil, header, err
	}
	after, err := os.Lstat(replayPath)
	if err != nil {
		return nil, header, err
	}
	if !os.SameFile(before, after) || after.Size() != 4096 || !bytes.Equal(page, expected[:]) {
		return nil, header, fmt.Errorf("full-page Reader bytes/extent mismatch")
	}
	wanted, err := ParseHeader(expected[:], 4096)
	if err != nil || header != wanted {
		return nil, header, fmt.Errorf("Reader header mismatch: %v", err)
	}
	if !manualRollbackFaultRegistriesEmpty() || !c.ledger.retired() {
		return nil, header, fmt.Errorf("Reader or preparation owner unresolved")
	}
	return page, header, nil
}

func manualRollbackFaultSnapshot(c *manualRollbackFaultConfig, name string, limit int64) (rollbackfault.RecoveryFileSnapshot, error) {
	var snapshot rollbackfault.RecoveryFileSnapshot
	if err := c.stable(); err != nil {
		return snapshot, err
	}
	path := filepath.Join(c.root, name)
	_, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return snapshot, nil
	}
	if err != nil {
		return snapshot, err
	}
	raw, info, err := manualRollbackFaultRead(&c.ledger, path, limit)
	if err != nil {
		return snapshot, err
	}
	metadata, ok := info.Sys().(*syscall.Stat_t)
	if !ok || metadata.Nlink != 1 || metadata.Ino == 0 {
		return snapshot, fmt.Errorf("single-link concrete input identity required")
	}
	snapshot = rollbackfault.RecoveryFileSnapshot{Present: true, Device: uint64(metadata.Dev), Inode: metadata.Ino, Size: info.Size(), Hash: sha256.Sum256(raw)}
	return snapshot, nil
}

func manualRollbackFaultCapture(c *manualRollbackFaultConfig, evidence *rollbackfault.RecoveryEvidence, after bool) error {
	names := [8]string{"baseline.input.db", "expected-new.input.db", "trial.db", "trial.db-journal", "post-commit.input.db", "post-commit-journal.input.bin", "replay.db", "replay.db-journal"}
	for i, name := range names {
		limit := int64(4096)
		if i == 3 || i == 5 || i == 7 {
			limit = 8200
		}
		snapshot, err := manualRollbackFaultSnapshot(c, name, limit)
		if err != nil {
			return err
		}
		evidence.Files[i].Role = rollbackfault.RecoveryFileRole(i + 1)
		if after {
			evidence.Files[i].After = snapshot
		} else {
			evidence.Files[i].Before = snapshot
		}
	}
	return nil
}

func manualRollbackFaultOwners(c *manualRollbackFaultConfig, evidence *rollbackfault.RecoveryEvidence) error {
	if !c.ledger.retired() {
		return fmt.Errorf("ordinary owner ledger incomplete")
	}
	evidence.OwnerCount = 0
	for i := 0; i < c.ledger.count; i++ {
		o := c.ledger.owners[i]
		if o.file == nil && o.reader == nil {
			continue
		}
		kind := rollbackfault.RecoveryFileOwner
		if o.reader != nil {
			kind = rollbackfault.RecoveryReaderOwner
		}
		evidence.Owners[evidence.OwnerCount] = rollbackfault.RecoveryOwnerEvidence{ID: uint64(i + 1000), Kind: kind, OpenCalls: 1, CloseCalls: o.closes, State: rollbackfault.OwnerRetired}
		evidence.OwnerCount++
	}
	return nil
}

func manualRollbackFaultStage(evidence *rollbackfault.RecoveryEvidence, stage rollbackfault.RecoveryStage) {
	evidence.Stages[evidence.StageCount] = stage
	evidence.StageCount++
}

func TestManualRollbackFaultRecovery(t *testing.T) {
	if os.Getenv("GOSQLITE_MANUAL_FAULT_ROLLBACK") != "independently_authorized" {
		t.Fatal("NOT_RUN: separately authorized execution, reviewed provenance and private retained root are required")
	}
	var c manualRollbackFaultConfig
	defer c.ledger.finish(t)
	if err := manualRollbackFaultAdmit(&c); err != nil {
		t.Fatalf("BLOCKED before fixture: %v", err)
	}
	old, next, partial := manualRollbackFaultImages()
	plan, err := manualRollbackFaultPlan(&c)
	if err != nil {
		t.Fatalf("BLOCKED before fixture: %v", err)
	}
	var spec rollbackfault.RecoverySpec
	spec.Case, spec.Plan = c.caseID, plan.Spec()
	spec.SourceHash, err = manualRollbackFaultHash(c.binding.Source)
	if err != nil {
		t.Fatal(err)
	}
	spec.ToolHash, err = manualRollbackFaultHash(c.binding.Tool)
	if err != nil {
		t.Fatal(err)
	}
	spec.BinaryHash, err = manualRollbackFaultHash(c.binding.Binary)
	if err != nil {
		t.Fatal(err)
	}
	spec.ManifestHash, err = manualRollbackFaultHash(c.binding.Manifest)
	if err != nil {
		t.Fatal(err)
	}
	spec.CompileReceiptHash, err = manualRollbackFaultHash(c.binding.CompileReceipt)
	if err != nil {
		t.Fatal(err)
	}
	spec.OldPageHash, spec.NewPageHash, spec.PartialPageHash = sha256.Sum256(old[:]), sha256.Sum256(next[:]), sha256.Sum256(partial[:])
	evidence := rollbackfault.RecoveryEvidence{Identity: spec.Plan.Identity, Case: c.caseID, SelectedCases: 1}
	evidence.Provenance = rollbackfault.RecoveryProvenance{
		SourceHash: spec.SourceHash, ToolHash: spec.ToolHash, BinaryHash: spec.BinaryHash, ManifestHash: spec.ManifestHash, CompileReceiptHash: spec.CompileReceiptHash,
		CompileSourceHash: spec.SourceHash, CompileToolHash: spec.ToolHash, CompileBinaryHash: spec.BinaryHash, CompileManifestHash: spec.ManifestHash,
		SourceFiles: c.sourceFiles, ManifestFiles: c.sourceFiles, CompileFiles: c.sourceFiles,
	}
	manualRollbackFaultStage(&evidence, rollbackfault.RecoveryAdmitted)
	if _, err := c.create("baseline.input.db", old[:], true); err != nil {
		t.Fatal(err)
	}
	if _, err := c.create("expected-new.input.db", next[:], true); err != nil {
		t.Fatal(err)
	}
	trialInfo, err := c.create("trial.db", old[:], false)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.create("binding.input.json", c.bindingBytes, true); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	report, commitCheckErr := manualRollbackFaultCommit(ctx, filepath.Join(c.root, "trial.db"), plan)
	// No recovery, original Close, error substitution or cleanup retry is
	// allowed to turn an incomplete adapter result into a successful trial.
	if commitCheckErr != nil {
		t.Logf("original Commit error: %v; adapter: %+v", report.CommitError, report.Observation)
		for i, diagnostic := range report.Diagnostics {
			if diagnostic != nil {
				t.Logf("adapter diagnostic %d: %v", i, diagnostic)
			}
		}
		t.Fatalf("INCOMPLETE: adapter/owner qualification: %v; all inputs retained", commitCheckErr)
	}
	evidence.ArmCalls, evidence.CommitCalls, evidence.FinishCalls, evidence.DisarmCalls = 1, 1, 1, 1
	evidence.AdapterPlan, evidence.AdapterEvents, evidence.AdapterEventCount, evidence.AdapterObservation = plan.Spec(), report.Events, report.EventCount, report.Observation
	var adapterHashProblem rollbackfault.RecoveryProblem
	evidence.AdapterHash, adapterHashProblem = rollbackfault.RecoveryAdapterHash(evidence.AdapterPlan, evidence.AdapterEvents, evidence.AdapterEventCount)
	if adapterHashProblem != rollbackfault.RecoveryProblemNone {
		t.Fatalf("adapter transcript hash: %v", adapterHashProblem)
	}
	for i := uint8(0); i < report.EventCount; i++ {
		if report.Events[i].Kind == rollbackfault.CommitReturned {
			evidence.CommitReturn = report.Events[i].Commit
		}
	}
	if report.CommitError != nil {
		evidence.CommitDiagnosticHash = sha256.Sum256([]byte(report.CommitError.Error()))
	}
	manualRollbackFaultStage(&evidence, rollbackfault.RecoveryCommitReturned)
	manualRollbackFaultStage(&evidence, rollbackfault.RecoveryAdapterSealed)
	manualRollbackFaultStage(&evidence, rollbackfault.RecoveryAdapterDisarmed)
	if err := c.stable(); err != nil {
		t.Fatal(err)
	}
	post, postInfo, err := manualRollbackFaultRead(&c.ledger, filepath.Join(c.root, "trial.db"), 4096)
	if err != nil {
		t.Fatal(err)
	}
	wantPost := next
	if c.caseID == rollbackfault.RecoveryPagePrefix64 {
		wantPost = partial
	}
	if !os.SameFile(trialInfo, postInfo) || !bytes.Equal(post, wantPost[:]) {
		t.Fatal("FAILED: exact post-Commit page/identity mismatch")
	}
	if _, err := c.create("post-commit.input.db", post, true); err != nil {
		t.Fatal(err)
	}
	var journalBytes []byte
	var journalInfo os.FileInfo
	if c.caseID == rollbackfault.RecoveryControl {
		if _, err := os.Lstat(filepath.Join(c.root, "trial.db-journal")); !os.IsNotExist(err) {
			t.Fatalf("control journal must be absent: %v", err)
		}
	} else {
		journalBytes, journalInfo, err = manualRollbackFaultRead(&c.ledger, filepath.Join(c.root, "trial.db-journal"), 8200)
		if err != nil || len(journalBytes) != 8200 {
			t.Fatalf("FAILED: actual journal bounds: %v", err)
		}
		if _, err := c.create("post-commit-journal.input.bin", journalBytes, true); err != nil {
			t.Fatal(err)
		}
	}
	manualRollbackFaultStage(&evidence, rollbackfault.RecoveryInputsRetained)
	replayInfo, err := c.create("replay.db", post, false)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(replayInfo, trialInfo) {
		t.Fatal("replay aliases original trial")
	}
	if c.caseID != rollbackfault.RecoveryControl {
		if _, err := c.create("replay.db-journal", journalBytes, false); err != nil {
			t.Fatal(err)
		}
	}
	if err := manualRollbackFaultCapture(&c, &evidence, false); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < len(evidence.Files); i++ {
		for j := 0; j < i; j++ {
			a, b := evidence.Files[i].Before, evidence.Files[j].Before
			if a.Present && b.Present && a.Device == b.Device && a.Inode == b.Inode {
				t.Fatal("pre-recovery file alias refused")
			}
		}
	}
	// Reject any already-observed namespace/content change before replay, not
	// only in the final receipt validator. The stable private namespace is
	// still an assumption, not a claim of race-proof path security.
	if err := c.retainedUnchanged(); err != nil {
		t.Fatal(err)
	}
	postHash, journalHash := sha256.Sum256(post), sha256.Sum256(journalBytes)
	wantHashes := [8][32]byte{spec.OldPageHash, spec.NewPageHash, postHash, journalHash, postHash, journalHash, postHash, journalHash}
	for i, file := range evidence.Files {
		isJournal := i == 3 || i == 5 || i == 7
		if isJournal && c.caseID == rollbackfault.RecoveryControl {
			if file.Before != (rollbackfault.RecoveryFileSnapshot{}) {
				t.Fatal("unexpected control journal before replay")
			}
			continue
		}
		if !file.Before.Present || file.Before.Hash != wantHashes[i] {
			t.Fatal("pre-recovery retained/copy bytes changed")
		}
	}
	for index, info := range map[int]os.FileInfo{2: trialInfo, 6: replayInfo, 3: journalInfo} {
		if info == nil {
			continue
		}
		metadata, ok := info.Sys().(*syscall.Stat_t)
		actual := evidence.Files[index].Before
		if !ok || actual.Device != uint64(metadata.Dev) || actual.Inode != metadata.Ino {
			t.Fatal("pre-recovery original/replay identity changed")
		}
	}
	manualRollbackFaultStage(&evidence, rollbackfault.RecoveryReplayCopied)
	if !manualRollbackFaultRegistriesEmpty() {
		t.Fatal("pre-recovery registry nonempty")
	}
	evidence.Recovery.RegistryChecks++
	if c.caseID != rollbackfault.RecoveryControl {
		result, recoveryErr := manualRollbackFaultRecoverCopy(ctx, filepath.Join(c.root, "replay.db"))
		evidence.Recovery.Calls++
		if recoveryErr != nil {
			t.Fatalf("INCOMPLETE: actual recovery: %v; all original inputs retained", recoveryErr)
		}
		evidence.Recovery.Recovered, evidence.Recovery.RestoredPages, evidence.Recovery.OriginalPages = result.Recovered, result.RestoredPages, result.OriginalPages
		evidence.Recovery.MaxJournalBytes, evidence.Recovery.MaxDatabaseBytes, evidence.Recovery.MaxRecords = 8200, 4096, 1
	}
	if !manualRollbackFaultRegistriesEmpty() {
		t.Fatal("post-recovery registry nonempty")
	}
	evidence.Recovery.RegistryChecks++
	if _, err := os.Lstat(filepath.Join(c.root, "replay.db-journal")); !os.IsNotExist(err) {
		t.Fatalf("replay journal must be absent after recovery/control: %v", err)
	}
	manualRollbackFaultStage(&evidence, rollbackfault.RecoveryChecked)
	wantRead := old
	if c.caseID == rollbackfault.RecoveryControl {
		wantRead = next
	}
	readerOwner := uint64(c.ledger.count + 1000)
	page, header, err := manualRollbackFaultReadback(&c, ctx, filepath.Join(c.root, "replay.db"), wantRead)
	if err != nil {
		t.Fatalf("FAILED: new Reader full-page observation: %v", err)
	}
	evidence.Reader = rollbackfault.RecoveryReadEvidence{
		Owner: readerOwner, ReadCalls: 1, CloseCalls: 1, PageNumber: 1, BytesRead: uint32(len(page)), ComparedBytes: 4096, MatchingBytes: 4096,
		ExtentBefore: 4096, ExtentAfter: 4096, PageHash: sha256.Sum256(page),
		Header: rollbackfault.RecoveryPageHeader{PageSize: header.PageSize, DatabasePages: header.PageCount, ChangeCounter: header.ChangeCounter, SchemaCookie: header.SchemaCookie, VersionValidFor: header.VersionValidFor, Encoding: uint8(header.TextEncoding), ReadVersion: header.ReadVersion, WriteVersion: header.WriteVersion},
	}
	manualRollbackFaultStage(&evidence, rollbackfault.RecoveryReaderClosed)
	if err := manualRollbackFaultCapture(&c, &evidence, true); err != nil {
		t.Fatal(err)
	}
	if err := c.retainedUnchanged(); err != nil {
		t.Fatal(err)
	}
	if !manualRollbackFaultRegistriesEmpty() {
		t.Fatal("final plain registry nonempty")
	}
	manualRollbackFaultStage(&evidence, rollbackfault.RecoveryInputsRechecked)
	if err := manualRollbackFaultOwners(&c, &evidence); err != nil {
		t.Fatal(err)
	}
	var diagnostics [rollbackfault.MaxEvents]string
	for i, diagnostic := range report.Diagnostics {
		if diagnostic != nil {
			diagnostics[i] = diagnostic.Error()
		}
	}
	commitDiagnostic := ""
	if report.CommitError != nil {
		commitDiagnostic = report.CommitError.Error()
	}
	preterminal, err := json.Marshal(struct {
		Status           string
		Spec             rollbackfault.RecoverySpec
		Evidence         rollbackfault.RecoveryEvidence
		CommitDiagnostic string
		Diagnostics      [rollbackfault.MaxEvents]string
	}{"PRETERMINAL_NOT_ACCEPTANCE", spec, evidence, commitDiagnostic, diagnostics})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.create("preterminal.json", preterminal, false); err != nil {
		t.Fatalf("INCOMPLETE: receipt output: %v", err)
	}
	// The receipt descriptor and its readback descriptor are now retired.
	// Final terminal is emitted only to the caller-owned test log, preventing a
	// circular claim that a file contains proof of its own successful Close.
	if err := manualRollbackFaultOwners(&c, &evidence); err != nil {
		t.Fatal(err)
	}
	manualRollbackFaultStage(&evidence, rollbackfault.RecoveryTerminal)
	observation := rollbackfault.ValidateRecoveryEvidence(spec, evidence)
	if observation.Outcome != rollbackfault.ScenarioObserved || observation.Pass() || observation.PhysicalValidated() {
		t.Fatalf("FAILED: recovery evidence consistency: %+v", observation)
	}
	terminal, err := json.Marshal(struct {
		Status          string
		PreterminalHash [32]byte
		Spec            rollbackfault.RecoverySpec
		Evidence        rollbackfault.RecoveryEvidence
		Observation     rollbackfault.RecoveryObservation
	}{"ScenarioObserved_INPUT_CONSISTENCY_ONLY", sha256.Sum256(preterminal), spec, evidence, observation})
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("ROLLBACK_RECOVERY_TERMINAL %s", terminal)
}
`,
	rollbackFaultRecoveryReceipt: `package rollbackfault

import "crypto/sha256"

// This file models bounded caller-supplied evidence, not physical execution.
// Hash agreement is input consistency. It neither authenticates a build receipt
// nor proves that a caller measured a file, performed recovery, or retired an
// owner. Independent source/build review and authorized physical evidence remain
// separate requirements. There are no descriptors, error interfaces, callbacks,
// environment reads, clocks, goroutines, or I/O in this model.
const (
	MaxRecoveryOwners      = 96
	MaxRecoverySourceFiles = 4096
	RecoveryStageCount     = 10
)

type RecoveryCase uint8

const (
	RecoveryCaseUnknown RecoveryCase = iota
	RecoveryControl
	RecoveryPagePrefix64
	RecoveryDBSyncBefore
)

// RecoverySpec is the frozen one-case expectation. The first four hash fields
// also bind the corresponding lowercase hexadecimal Plan.Identity labels.
// CompileReceiptHash identifies separately retained source-to-binary evidence;
// a self-reported binary digest alone is deliberately insufficient.
type RecoverySpec struct {
	Case                                                               RecoveryCase
	Plan                                                               PlanSpec
	SourceHash, ToolHash, BinaryHash, ManifestHash, CompileReceiptHash [32]byte
	OldPageHash, NewPageHash, PartialPageHash                          [32]byte
}

type RecoveryProvenance struct {
	SourceHash, ToolHash, BinaryHash, ManifestHash, CompileReceiptHash         [32]byte
	CompileSourceHash, CompileToolHash, CompileBinaryHash, CompileManifestHash [32]byte
	SourceFiles, ManifestFiles, CompileFiles                                   uint16
}

type RecoveryFileRole uint8

const (
	RecoveryFileUnknown RecoveryFileRole = iota
	RecoveryBaseline
	RecoveryExpectedNew
	RecoveryTrialDatabase
	RecoveryTrialJournal
	RecoveryRetainedDatabase
	RecoveryRetainedJournal
	RecoveryReplayDatabase
	RecoveryReplayJournal
)

// Missing snapshots must be entirely zero. Before is the retained/pre-recovery
// checkpoint, including the post-Commit trial and fresh replay copy. After is
// the final recheck. Device/Inode are caller-observed identities, not handles.
type RecoveryFileSnapshot struct {
	Present       bool
	Device, Inode uint64
	Size          int64
	Hash          [32]byte
}

type RecoveryFileEvidence struct {
	Role          RecoveryFileRole
	Before, After RecoveryFileSnapshot
}

// Registry evidence is deliberately aggregate. The production recovery API
// does not expose an independent descriptor-owner token, so none is invented.
type RecoveryRegistrySnapshot struct {
	PlainGroups, Pending uint16
}

type RecoveryCallEvidence struct {
	Calls                             uint8
	Error                             ErrorKind
	ErrorToken                        uint64
	Recovered                         bool
	RestoredPages, OriginalPages      uint32
	MaxJournalBytes, MaxDatabaseBytes uint64
	MaxRecords                        uint32
	RegistryChecks                    uint8
	RegistryBefore, RegistryAfter     RecoveryRegistrySnapshot
}

type RecoveryPageHeader struct {
	PageSize, DatabasePages, ChangeCounter, SchemaCookie, VersionValidFor uint32
	Encoding, ReadVersion, WriteVersion                                   uint8
}

type RecoveryReadEvidence struct {
	Owner                                               uint64
	ReadCalls, CloseCalls                               uint8
	Error, CloseError                                   ErrorKind
	ErrorToken, CloseErrorToken                         uint64
	PageNumber, BytesRead, ComparedBytes, MatchingBytes uint32
	ExtentBefore, ExtentAfter                           int64
	PageHash                                            [32]byte
	Header                                              RecoveryPageHeader
}

type RecoveryOwnerKind uint8

const (
	RecoveryOwnerUnknown RecoveryOwnerKind = iota
	RecoveryFileOwner
	RecoveryReaderOwner
)

// These bounded scalar IDs identify the caller's preparation/readback ledger.
// They do not pretend to be recovery's undisclosed descriptor-owner IDs.
type RecoveryOwnerEvidence struct {
	ID                    uint64
	Kind                  RecoveryOwnerKind
	OpenCalls, CloseCalls uint8
	CloseError            ErrorKind
	CloseErrorToken       uint64
	State                 OwnerState
}

type RecoveryStage uint8

const (
	RecoveryStageUnknown RecoveryStage = iota
	RecoveryAdmitted
	RecoveryCommitReturned
	RecoveryAdapterSealed
	RecoveryAdapterDisarmed
	RecoveryInputsRetained
	RecoveryReplayCopied
	RecoveryChecked
	RecoveryReaderClosed
	RecoveryInputsRechecked
	RecoveryTerminal
)

// The nested adapter transcript is complete: its plan and every event are
// replayed through the original contract, rather than trusting an outcome flag.
// CommitReturn and its diagnostic hash refer to the original Commit return.
// Native caller source must separately bind that return directly to finish.
type RecoveryEvidence struct {
	Identity                                                       Identity
	Case                                                           RecoveryCase
	SelectedCases, ArmCalls, CommitCalls, FinishCalls, DisarmCalls uint8
	DisarmError                                                    ErrorKind
	DisarmErrorToken                                               uint64
	CommitReturn                                                   CommitResult
	CommitDiagnosticHash                                           [32]byte
	UnexpectedErrorCount                                           uint8
	AdapterPlan                                                    PlanSpec
	AdapterEvents                                                  [MaxEvents]Event
	AdapterEventCount                                              uint8
	AdapterHash                                                    [32]byte
	AdapterObservation                                             Observation
	Provenance                                                     RecoveryProvenance
	Files                                                          [8]RecoveryFileEvidence
	Recovery                                                       RecoveryCallEvidence
	Reader                                                         RecoveryReadEvidence
	Owners                                                         [MaxRecoveryOwners]RecoveryOwnerEvidence
	OwnerCount                                                     uint8
	Stages                                                         [RecoveryStageCount]RecoveryStage
	StageCount                                                     uint8
}

type RecoveryProblem uint8

const (
	RecoveryProblemNone RecoveryProblem = iota
	RecoveryProblemCase
	RecoveryProblemSpec
	RecoveryProblemIdentity
	RecoveryProblemProvenance
	RecoveryProblemCalls
	RecoveryProblemAdapter
	RecoveryProblemAdapterHash
	RecoveryProblemCommit
	RecoveryProblemFiles
	RecoveryProblemAliases
	RecoveryProblemRecovery
	RecoveryProblemReadback
	RecoveryProblemOwners
	RecoveryProblemTerminal
)

type RecoveryOutcome uint8

const (
	RecoveryInvalid RecoveryOutcome = iota
	// ScenarioObserved means only that supplied finite evidence is consistent.
	// It is never an acceptance, durability, physical, or all-three-cases PASS.
	ScenarioObserved
)

type RecoveryObservation struct {
	Outcome RecoveryOutcome
	Problem RecoveryProblem
	Case    RecoveryCase
	Adapter Observation
}

func (RecoveryObservation) PhysicalValidated() bool { return false }
func (RecoveryObservation) Pass() bool              { return false }

func ValidateRecoveryEvidence(spec RecoverySpec, evidence RecoveryEvidence) RecoveryObservation {
	observation := RecoveryObservation{Case: spec.Case}
	if problem := validRecoverySpec(spec); problem != RecoveryProblemNone {
		observation.Problem = problem
		return observation
	}
	if evidence.Case != spec.Case || evidence.SelectedCases != 1 {
		observation.Problem = RecoveryProblemCase
		return observation
	}
	if !validIdentity(evidence.Identity) || evidence.Identity != spec.Plan.Identity {
		observation.Problem = RecoveryProblemIdentity
		return observation
	}
	if !validRecoveryProvenance(spec, evidence.Provenance) {
		observation.Problem = RecoveryProblemProvenance
		return observation
	}
	if evidence.ArmCalls != 1 || evidence.CommitCalls != 1 || evidence.FinishCalls != 1 || evidence.DisarmCalls != 1 || evidence.DisarmError != None || evidence.DisarmErrorToken != 0 || evidence.UnexpectedErrorCount != 0 {
		observation.Problem = RecoveryProblemCalls
		return observation
	}
	if evidence.AdapterPlan != spec.Plan || evidence.AdapterEventCount == 0 || int(evidence.AdapterEventCount) > MaxEvents {
		observation.Problem = RecoveryProblemAdapter
		return observation
	}
	plan, planProblem := NewPlan(evidence.AdapterPlan)
	if planProblem != NoProblem {
		observation.Problem = RecoveryProblemAdapter
		return observation
	}
	receipt, receiptProblem := NewReceipt(plan)
	if receiptProblem != NoProblem {
		observation.Problem = RecoveryProblemAdapter
		return observation
	}
	for i := 0; i < int(evidence.AdapterEventCount); i++ {
		if receipt.Append(evidence.AdapterEvents[i]) != NoProblem {
			observation.Problem = RecoveryProblemAdapter
			return observation
		}
	}
	for i := int(evidence.AdapterEventCount); i < MaxEvents; i++ {
		if evidence.AdapterEvents[i] != (Event{}) {
			observation.Problem = RecoveryProblemAdapter
			return observation
		}
	}
	observation.Adapter = receipt.Seal()
	if observation.Adapter.Outcome != AdapterObserved || observation.Adapter.Problem != NoProblem || observation.Adapter.TargetHits != 1 || observation.Adapter != evidence.AdapterObservation {
		observation.Problem = RecoveryProblemAdapter
		return observation
	}
	digest, hashProblem := RecoveryAdapterHash(evidence.AdapterPlan, evidence.AdapterEvents, evidence.AdapterEventCount)
	if hashProblem != RecoveryProblemNone || digest != evidence.AdapterHash || digest == ([32]byte{}) {
		observation.Problem = RecoveryProblemAdapterHash
		return observation
	}
	commitIndex := 1 + 2*int(spec.Plan.CallCount)
	if commitIndex >= int(evidence.AdapterEventCount) || evidence.AdapterEvents[commitIndex].Kind != CommitReturned || evidence.CommitReturn != evidence.AdapterEvents[commitIndex].Commit || evidence.CommitReturn.Class != spec.Plan.ExpectedCommit || evidence.CommitReturn.Logical != LogicalClosed {
		observation.Problem = RecoveryProblemCommit
		return observation
	}
	if spec.Case == RecoveryControl {
		if evidence.CommitReturn.Error != None || evidence.CommitReturn.ErrorToken != 0 || evidence.CommitReturn.TargetError != None || evidence.CommitReturn.TargetErrorToken != 0 || evidence.CommitDiagnosticHash != ([32]byte{}) {
			observation.Problem = RecoveryProblemCommit
			return observation
		}
	} else {
		cause := Injected
		if spec.Case == RecoveryPagePrefix64 {
			cause = ShortWrite
		}
		if evidence.CommitReturn.Error != Underlying || evidence.CommitReturn.ErrorToken == 0 || evidence.CommitReturn.TargetError != cause || evidence.CommitReturn.TargetErrorToken != 0 || evidence.CommitDiagnosticHash == ([32]byte{}) {
			observation.Problem = RecoveryProblemCommit
			return observation
		}
	}
	if problem := validRecoveryFiles(spec, evidence.Files); problem != RecoveryProblemNone {
		observation.Problem = problem
		return observation
	}
	if !validRecoveryCall(spec.Case, evidence.Recovery) {
		observation.Problem = RecoveryProblemRecovery
		return observation
	}
	if !validRecoveryRead(spec, evidence.Reader) {
		observation.Problem = RecoveryProblemReadback
		return observation
	}
	if !validRecoveryOwners(evidence) {
		observation.Problem = RecoveryProblemOwners
		return observation
	}
	if evidence.StageCount != RecoveryStageCount {
		observation.Problem = RecoveryProblemTerminal
		return observation
	}
	for i, stage := range evidence.Stages {
		if stage != RecoveryStage(i+1) {
			observation.Problem = RecoveryProblemTerminal
			return observation
		}
	}
	observation.Outcome = ScenarioObserved
	return observation
}

func validRecoverySpec(spec RecoverySpec) RecoveryProblem {
	if spec.Case < RecoveryControl || spec.Case > RecoveryDBSyncBefore {
		return RecoveryProblemCase
	}
	if _, problem := NewPlan(spec.Plan); problem != NoProblem {
		return RecoveryProblemSpec
	}
	if spec.Plan.OriginalPages != 1 || spec.Plan.DirtyPages != 1 || spec.Plan.Target.Occurrence != 1 || spec.Plan.Target.Offset != 0 || spec.Plan.Target.Role != Database || spec.Plan.Target.Owner != spec.Plan.MainOwner {
		return RecoveryProblemSpec
	}
	point, action, length, prefix := DBPage, Pass, 4096, 0
	switch spec.Case {
	case RecoveryPagePrefix64:
		action, prefix = PrefixShortWrite, 64
	case RecoveryDBSyncBefore:
		point, action, length = DBSync, FailBefore, 0
	}
	if spec.Plan.Target.Point != point || spec.Plan.Action != action || spec.Plan.Target.Length != length || spec.Plan.Prefix != prefix || spec.Plan.PrefixError {
		return RecoveryProblemSpec
	}
	hashes := [8][32]byte{spec.SourceHash, spec.ToolHash, spec.BinaryHash, spec.ManifestHash, spec.CompileReceiptHash, spec.OldPageHash, spec.NewPageHash, spec.PartialPageHash}
	for _, hash := range hashes {
		if hash == ([32]byte{}) {
			return RecoveryProblemSpec
		}
	}
	if spec.OldPageHash == spec.NewPageHash || spec.OldPageHash == spec.PartialPageHash || spec.NewPageHash == spec.PartialPageHash {
		return RecoveryProblemSpec
	}
	if spec.Plan.Identity.Source != recoveryHashLabel(spec.SourceHash) || spec.Plan.Identity.Tool != recoveryHashLabel(spec.ToolHash) || spec.Plan.Identity.Binary != recoveryHashLabel(spec.BinaryHash) || spec.Plan.Identity.Manifest != recoveryHashLabel(spec.ManifestHash) {
		return RecoveryProblemIdentity
	}
	return RecoveryProblemNone
}

func validRecoveryProvenance(spec RecoverySpec, p RecoveryProvenance) bool {
	return p.SourceHash == spec.SourceHash && p.ToolHash == spec.ToolHash && p.BinaryHash == spec.BinaryHash && p.ManifestHash == spec.ManifestHash && p.CompileReceiptHash == spec.CompileReceiptHash && p.CompileSourceHash == spec.SourceHash && p.CompileToolHash == spec.ToolHash && p.CompileBinaryHash == spec.BinaryHash && p.CompileManifestHash == spec.ManifestHash && p.SourceFiles > 0 && p.SourceFiles <= MaxRecoverySourceFiles && p.SourceFiles == p.ManifestFiles && p.SourceFiles == p.CompileFiles
}

func validRecoveryFiles(spec RecoverySpec, files [8]RecoveryFileEvidence) RecoveryProblem {
	post, final := spec.NewPageHash, spec.OldPageHash
	if spec.Case == RecoveryPagePrefix64 {
		post = spec.PartialPageHash
	}
	if spec.Case == RecoveryControl {
		final = spec.NewPageHash
	}
	for i, file := range files {
		if file.Role != RecoveryFileRole(i+1) {
			return RecoveryProblemFiles
		}
		journal := file.Role == RecoveryTrialJournal || file.Role == RecoveryRetainedJournal || file.Role == RecoveryReplayJournal
		if journal && spec.Case == RecoveryControl {
			if file.Before != (RecoveryFileSnapshot{}) || file.After != (RecoveryFileSnapshot{}) {
				return RecoveryProblemFiles
			}
			continue
		}
		size := int64(4096)
		if journal {
			size = 8200
		}
		if !validRecoverySnapshot(file.Before, size) {
			return RecoveryProblemFiles
		}
		if file.Role == RecoveryReplayJournal {
			if file.After != (RecoveryFileSnapshot{}) {
				return RecoveryProblemFiles
			}
		} else if !validRecoverySnapshot(file.After, size) {
			return RecoveryProblemFiles
		}
		if file.Role != RecoveryReplayDatabase && file.Role != RecoveryReplayJournal && file.Before != file.After {
			return RecoveryProblemFiles
		}
		want := post
		switch file.Role {
		case RecoveryBaseline:
			want = spec.OldPageHash
		case RecoveryExpectedNew:
			want = spec.NewPageHash
		case RecoveryTrialJournal, RecoveryRetainedJournal, RecoveryReplayJournal:
			want = files[3].Before.Hash
		}
		if file.Before.Hash != want {
			return RecoveryProblemFiles
		}
		if file.Role == RecoveryReplayDatabase && (file.After.Hash != final || file.After.Device != file.Before.Device || file.After.Inode != file.Before.Inode) {
			return RecoveryProblemFiles
		}
		for j := 0; j < i; j++ {
			other := files[j].Before
			if other.Present && other.Device == file.Before.Device && other.Inode == file.Before.Inode {
				return RecoveryProblemAliases
			}
		}
	}
	return RecoveryProblemNone
}

func validRecoverySnapshot(snapshot RecoveryFileSnapshot, size int64) bool {
	return snapshot.Present && snapshot.Device != 0 && snapshot.Inode != 0 && snapshot.Size == size && snapshot.Hash != ([32]byte{})
}

func validRecoveryCall(selected RecoveryCase, evidence RecoveryCallEvidence) bool {
	if evidence.RegistryChecks != 2 || evidence.RegistryBefore != (RecoveryRegistrySnapshot{}) || evidence.RegistryAfter != (RecoveryRegistrySnapshot{}) {
		return false
	}
	evidence.RegistryChecks = 0
	if selected == RecoveryControl {
		return evidence == (RecoveryCallEvidence{})
	}
	return evidence.Calls == 1 && evidence.Error == None && evidence.ErrorToken == 0 && evidence.Recovered && evidence.RestoredPages == 1 && evidence.OriginalPages == 1 && evidence.MaxJournalBytes == 8200 && evidence.MaxDatabaseBytes == 4096 && evidence.MaxRecords == 1
}

func validRecoveryRead(spec RecoverySpec, evidence RecoveryReadEvidence) bool {
	hash, counter, cookie := spec.OldPageHash, uint32(7), uint32(9)
	if spec.Case == RecoveryControl {
		hash, counter, cookie = spec.NewPageHash, 8, 10
	}
	header := RecoveryPageHeader{PageSize: 4096, DatabasePages: 1, ChangeCounter: counter, SchemaCookie: cookie, VersionValidFor: counter, Encoding: 1, ReadVersion: 1, WriteVersion: 1}
	return evidence.Owner != 0 && evidence.ReadCalls == 1 && evidence.CloseCalls == 1 && evidence.Error == None && evidence.ErrorToken == 0 && evidence.CloseError == None && evidence.CloseErrorToken == 0 && evidence.PageNumber == 1 && evidence.BytesRead == 4096 && evidence.ComparedBytes == 4096 && evidence.MatchingBytes == 4096 && evidence.ExtentBefore == 4096 && evidence.ExtentAfter == 4096 && evidence.PageHash == hash && evidence.Header == header
}

func validRecoveryOwners(evidence RecoveryEvidence) bool {
	if evidence.OwnerCount < 2 || int(evidence.OwnerCount) > MaxRecoveryOwners {
		return false
	}
	readers := 0
	for i := 0; i < int(evidence.OwnerCount); i++ {
		owner := evidence.Owners[i]
		if owner.ID == 0 || owner.Kind < RecoveryFileOwner || owner.Kind > RecoveryReaderOwner || owner.OpenCalls != 1 || owner.CloseCalls != 1 || owner.CloseError != None || owner.CloseErrorToken != 0 || owner.State != OwnerRetired {
			return false
		}
		if owner.Kind == RecoveryReaderOwner {
			readers++
			if owner.ID != evidence.Reader.Owner {
				return false
			}
		}
		for j := 0; j < i; j++ {
			if evidence.Owners[j].ID == owner.ID {
				return false
			}
		}
	}
	for i := int(evidence.OwnerCount); i < MaxRecoveryOwners; i++ {
		if evidence.Owners[i] != (RecoveryOwnerEvidence{}) {
			return false
		}
	}
	return readers == 1
}

func recoveryHashLabel(hash [32]byte) string {
	const digits = "0123456789abcdef"
	var text [64]byte
	for i, value := range hash {
		text[2*i], text[2*i+1] = digits[value>>4], digits[value&15]
	}
	return string(text[:])
}

// RecoveryAdapterHash uses a domain-separated, length-delimited SHA-256 chain.
// It hashes the full plan and every field of each retained event, including
// normally-zero payloads. No formatting, native-width layout, or JSON encoding
// is part of this protocol. It checks bounded identities before hashing them;
// structural/semantic adapter validation is still done separately above.
func RecoveryAdapterHash(plan PlanSpec, events [MaxEvents]Event, count uint8) ([32]byte, RecoveryProblem) {
	if _, problem := NewPlan(plan); problem != NoProblem || count == 0 || int(count) > MaxEvents {
		return [32]byte{}, RecoveryProblemAdapter
	}
	for i := 0; i < int(count); i++ {
		if !validIdentity(events[i].Identity) {
			return [32]byte{}, RecoveryProblemIdentity
		}
	}
	for i := int(count); i < MaxEvents; i++ {
		if events[i] != (Event{}) {
			return [32]byte{}, RecoveryProblemAdapter
		}
	}
	var digest recoveryHashState
	digest.id("rollback-recovery-adapter-v1")
	digest.identity(plan.Identity)
	digest.call(plan.Target)
	digest.number(uint64(plan.Action))
	digest.number(uint64(plan.Prefix))
	digest.boolean(plan.PrefixError)
	digest.number(plan.MainOwner)
	digest.number(plan.JournalOwner)
	digest.number(plan.DirectoryOwner)
	digest.number(plan.CleanupOwner)
	digest.number(uint64(plan.OriginalPages))
	digest.number(uint64(plan.DirtyPages))
	digest.number(uint64(plan.PageSize))
	digest.number(uint64(plan.SectorSize))
	digest.number(uint64(plan.CallCount))
	digest.number(uint64(plan.ExpectedCommit))
	for _, call := range plan.Calls {
		digest.call(call)
	}
	digest.number(uint64(count))
	for i := 0; i < int(count); i++ {
		event := events[i]
		digest.identity(event.Identity)
		digest.number(uint64(event.Ordinal))
		digest.number(uint64(event.Kind))
		digest.call(event.Call)
		digest.number(uint64(event.Action))
		digest.boolean(event.Result.Executed)
		digest.number(uint64(event.Result.NativeN))
		digest.number(uint64(event.Result.NativeError))
		digest.number(event.Result.NativeErrorToken)
		digest.number(uint64(event.Result.ReturnedN))
		digest.number(uint64(event.Result.ReturnedError))
		digest.number(event.Result.ReturnedErrorToken)
		digest.number(uint64(event.Result.NormalizedError))
		digest.number(event.Result.NormalizedErrorToken)
		digest.number(uint64(event.Result.Effect))
		digest.number(uint64(event.Commit.Class))
		digest.number(uint64(event.Commit.Error))
		digest.number(event.Commit.ErrorToken)
		digest.number(uint64(event.Commit.TargetError))
		digest.number(event.Commit.TargetErrorToken)
		digest.number(uint64(event.Commit.Logical))
		digest.number(event.Owners.MainOwner)
		digest.number(event.Owners.JournalOwner)
		digest.number(event.Owners.CleanupOwner)
		digest.number(uint64(event.Owners.Main))
		digest.number(uint64(event.Owners.Journal))
		digest.number(uint64(event.Owners.Cleanup))
		digest.number(uint64(event.Validation))
		digest.number(uint64(event.Rejection))
		digest.number(uint64(event.RejectedAt))
	}
	return digest.sum, RecoveryProblemNone
}

type recoveryHashState struct{ sum [32]byte }

func (digest *recoveryHashState) number(value uint64) {
	var input [41]byte
	copy(input[:32], digest.sum[:])
	input[32] = 1
	for i := 0; i < 8; i++ {
		input[40-i] = byte(value)
		value >>= 8
	}
	digest.sum = sha256.Sum256(input[:])
}

func (digest *recoveryHashState) boolean(value bool) {
	if value {
		digest.number(1)
	} else {
		digest.number(0)
	}
}

func (digest *recoveryHashState) id(value string) {
	var input [35 + MaxIDLength]byte
	copy(input[:32], digest.sum[:])
	input[32], input[33], input[34] = 2, byte(len(value)>>8), byte(len(value))
	copy(input[35:], value)
	digest.sum = sha256.Sum256(input[:35+len(value)])
}

func (digest *recoveryHashState) identity(value Identity) {
	digest.id(value.Suite)
	digest.id(value.Source)
	digest.id(value.Tool)
	digest.id(value.Binary)
	digest.id(value.Manifest)
	digest.id(value.Trial)
	digest.id(value.Scope)
	digest.number(uint64(value.Profile))
}

func (digest *recoveryHashState) call(value Call) {
	digest.number(uint64(value.Point))
	digest.number(uint64(value.Occurrence))
	digest.number(uint64(value.Role))
	digest.number(uint64(value.Offset))
	digest.number(uint64(value.Length))
	digest.number(value.Owner)
}
`,
	rollbackFaultRecoveryReceiptTest: `package rollbackfault

import "testing"

// SOURCE ONLY / NOT RUN. These are synthetic descriptor-free consistency
// vectors. Their invented hashes, inode numbers and scalar owner IDs are not
// measured files, build provenance, a database fixture, or physical evidence.
// None of these tests constructs a database or invokes Commit or recovery.
func recoveryFixture(t *testing.T, selected RecoveryCase) (RecoverySpec, RecoveryEvidence) {
	t.Helper()
	action, target := Pass, 8
	if selected == RecoveryPagePrefix64 {
		action = PrefixShortWrite
	}
	if selected == RecoveryDBSyncBefore {
		action, target = FailBefore, 9
	}
	spec := RecoverySpec{
		Case: selected, Plan: onePageSpec(action, target),
		SourceHash: [32]byte{1}, ToolHash: [32]byte{2}, BinaryHash: [32]byte{3},
		ManifestHash: [32]byte{4}, CompileReceiptHash: [32]byte{5},
		OldPageHash: [32]byte{6}, NewPageHash: [32]byte{7}, PartialPageHash: [32]byte{8},
	}
	if selected == RecoveryPagePrefix64 {
		spec.Plan.Prefix = 64
	}
	spec.Plan.Identity.Source = recoveryHashLabel(spec.SourceHash)
	spec.Plan.Identity.Tool = recoveryHashLabel(spec.ToolHash)
	spec.Plan.Identity.Binary = recoveryHashLabel(spec.BinaryHash)
	spec.Plan.Identity.Manifest = recoveryHashLabel(spec.ManifestHash)
	events := fixtureEvents(spec.Plan)
	evidence := RecoveryEvidence{
		Identity: spec.Plan.Identity, Case: selected, SelectedCases: 1,
		ArmCalls: 1, CommitCalls: 1, FinishCalls: 1, DisarmCalls: 1,
		AdapterPlan: spec.Plan, AdapterEventCount: uint8(len(events)),
		AdapterObservation: observe(t, spec.Plan, events),
		Provenance: RecoveryProvenance{
			SourceHash: spec.SourceHash, ToolHash: spec.ToolHash, BinaryHash: spec.BinaryHash,
			ManifestHash: spec.ManifestHash, CompileReceiptHash: spec.CompileReceiptHash,
			CompileSourceHash: spec.SourceHash, CompileToolHash: spec.ToolHash,
			CompileBinaryHash: spec.BinaryHash, CompileManifestHash: spec.ManifestHash,
			SourceFiles: 17, ManifestFiles: 17, CompileFiles: 17,
		},
		Recovery: RecoveryCallEvidence{RegistryChecks: 2},
		Reader: RecoveryReadEvidence{
			Owner: 102, ReadCalls: 1, CloseCalls: 1, PageNumber: 1,
			BytesRead: 4096, ComparedBytes: 4096, MatchingBytes: 4096,
			ExtentBefore: 4096, ExtentAfter: 4096, PageHash: spec.OldPageHash,
			Header: RecoveryPageHeader{PageSize: 4096, DatabasePages: 1, ChangeCounter: 7, SchemaCookie: 9, VersionValidFor: 7, Encoding: 1, ReadVersion: 1, WriteVersion: 1},
		},
		Owners: [MaxRecoveryOwners]RecoveryOwnerEvidence{
			{ID: 101, Kind: RecoveryFileOwner, OpenCalls: 1, CloseCalls: 1, State: OwnerRetired},
			{ID: 102, Kind: RecoveryReaderOwner, OpenCalls: 1, CloseCalls: 1, State: OwnerRetired},
		},
		OwnerCount: 2, StageCount: RecoveryStageCount,
	}
	copy(evidence.AdapterEvents[:], events)
	evidence.CommitReturn = events[1+2*int(spec.Plan.CallCount)].Commit
	if selected != RecoveryControl {
		evidence.CommitDiagnosticHash = [32]byte{9}
		evidence.Recovery = RecoveryCallEvidence{Calls: 1, Recovered: true, RestoredPages: 1, OriginalPages: 1, MaxJournalBytes: 8200, MaxDatabaseBytes: 4096, MaxRecords: 1, RegistryChecks: 2}
	} else {
		evidence.Reader.PageHash = spec.NewPageHash
		evidence.Reader.Header.ChangeCounter = 8
		evidence.Reader.Header.SchemaCookie = 10
		evidence.Reader.Header.VersionValidFor = 8
	}
	post := spec.NewPageHash
	if selected == RecoveryPagePrefix64 {
		post = spec.PartialPageHash
	}
	for i := range evidence.Files {
		role := RecoveryFileRole(i + 1)
		file := RecoveryFileEvidence{Role: role}
		journal := role == RecoveryTrialJournal || role == RecoveryRetainedJournal || role == RecoveryReplayJournal
		if !journal || selected != RecoveryControl {
			file.Before = RecoveryFileSnapshot{Present: true, Device: 1, Inode: uint64(201 + i), Size: 4096, Hash: post}
			if journal {
				file.Before.Size, file.Before.Hash = 8200, [32]byte{10}
			}
			if role == RecoveryBaseline {
				file.Before.Hash = spec.OldPageHash
			}
			if role == RecoveryExpectedNew {
				file.Before.Hash = spec.NewPageHash
			}
			file.After = file.Before
			if role == RecoveryReplayDatabase {
				file.After.Hash = evidence.Reader.PageHash
			}
			if role == RecoveryReplayJournal {
				file.After = RecoveryFileSnapshot{}
			}
		}
		evidence.Files[i] = file
	}
	for i := range evidence.Stages {
		evidence.Stages[i] = RecoveryStage(i + 1)
	}
	recoveryRehash(t, &evidence)
	return spec, evidence
}

func recoveryRehash(t *testing.T, evidence *RecoveryEvidence) {
	t.Helper()
	digest, problem := RecoveryAdapterHash(evidence.AdapterPlan, evidence.AdapterEvents, evidence.AdapterEventCount)
	if problem != RecoveryProblemNone {
		t.Fatalf("bounded synthetic transcript cannot be hashed: %v", problem)
	}
	evidence.AdapterHash = digest
}

func recoveryMustReject(t *testing.T, spec RecoverySpec, evidence RecoveryEvidence, problem RecoveryProblem) {
	t.Helper()
	observation := ValidateRecoveryEvidence(spec, evidence)
	if observation.Outcome == ScenarioObserved || observation.Pass() || observation.PhysicalValidated() || observation.Problem == RecoveryProblemNone {
		t.Fatalf("false scenario/physical qualification: %+v", observation)
	}
	if problem != RecoveryProblemNone && observation.Problem != problem {
		t.Fatalf("wrong rejection: got %v want %v", observation.Problem, problem)
	}
}

func TestRollbackRecoverySyntheticConsistencyNeverPhysicalPass(t *testing.T) {
	for _, selected := range [3]RecoveryCase{RecoveryControl, RecoveryPagePrefix64, RecoveryDBSyncBefore} {
		spec, evidence := recoveryFixture(t, selected)
		observation := ValidateRecoveryEvidence(spec, evidence)
		if observation.Outcome != ScenarioObserved || observation.Problem != RecoveryProblemNone || observation.Adapter.Outcome != AdapterObserved || observation.Case != selected {
			t.Fatalf("synthetic case %v rejected: %+v", selected, observation)
		}
		if observation.Pass() || observation.PhysicalValidated() || observation.Adapter.Pass() || observation.Adapter.PhysicalValidated() {
			t.Fatal("synthetic input consistency acquired physical authority")
		}
	}
	// Even manually constructed exported outcome fields cannot change methods.
	if (RecoveryObservation{Outcome: ScenarioObserved}).Pass() || (RecoveryObservation{Outcome: ScenarioObserved}).PhysicalValidated() {
		t.Fatal("exported outcome bypassed permanently absent physical status")
	}
	recoveryMustReject(t, RecoverySpec{}, RecoveryEvidence{}, RecoveryProblemCase)
	// A source inventory may itself be the manifest. Equal provenance digests
	// are legal; old/new/partial page images must nevertheless be distinct.
	spec, evidence := recoveryFixture(t, RecoveryControl)
	spec.ManifestHash = spec.SourceHash
	spec.Plan.Identity.Manifest = spec.Plan.Identity.Source
	evidence.Identity = spec.Plan.Identity
	evidence.AdapterPlan = spec.Plan
	evidence.Provenance.ManifestHash = spec.ManifestHash
	evidence.Provenance.CompileManifestHash = spec.ManifestHash
	for i := 0; i < int(evidence.AdapterEventCount); i++ {
		evidence.AdapterEvents[i].Identity = spec.Plan.Identity
	}
	recoveryRehash(t, &evidence)
	if observation := ValidateRecoveryEvidence(spec, evidence); observation.Outcome != ScenarioObserved || observation.Pass() || observation.PhysicalValidated() {
		t.Fatalf("shared inventory digest changed consistency-only semantics: %+v", observation)
	}
}

func TestRollbackRecoveryRejectsCasePlanAndProvenanceFalsePass(t *testing.T) {
	for _, name := range [...]string{"unknown-case", "foreign-case", "empty-selection", "multiple-selection", "mismatched-case", "target-occurrence", "target-offset", "target-length", "target-point", "prefix-63", "prefix-error", "dirty-two", "original-two", "source-zero", "tool-zero", "binary-zero", "manifest-zero", "compile-zero", "old-zero", "new-zero", "partial-zero", "old-new-equal", "old-partial-equal", "new-partial-equal", "identity-label", "stale-trial", "stale-scope", "unbounded-id", "stale-source", "stale-tool", "stale-binary", "stale-manifest", "missing-compile", "unbound-compile-source", "unbound-compile-tool", "unbound-compile-binary", "unbound-compile-manifest", "zero-source-count", "too-many-sources", "manifest-count", "compile-count"} {
		spec, evidence := recoveryFixture(t, RecoveryPagePrefix64)
		problem := RecoveryProblemSpec
		switch name {
		case "unknown-case":
			spec.Case, problem = RecoveryCaseUnknown, RecoveryProblemCase
		case "foreign-case":
			spec.Case, problem = RecoveryCase(255), RecoveryProblemCase
		case "empty-selection":
			evidence.SelectedCases, problem = 0, RecoveryProblemCase
		case "multiple-selection":
			evidence.SelectedCases, problem = 2, RecoveryProblemCase
		case "mismatched-case":
			evidence.Case, problem = RecoveryControl, RecoveryProblemCase
		case "target-occurrence":
			spec.Plan.Target.Occurrence = 2
		case "target-offset":
			spec.Plan.Target.Offset = 4096
		case "target-length":
			spec.Plan.Target.Length = 64
		case "target-point":
			spec.Plan.Target.Point = JournalUnlink
		case "prefix-63":
			spec.Plan.Prefix = 63
		case "prefix-error":
			spec.Plan.PrefixError = true
		case "dirty-two":
			spec.Plan.DirtyPages = 2
		case "original-two":
			spec.Plan.OriginalPages = 2
		case "source-zero":
			spec.SourceHash = [32]byte{}
		case "tool-zero":
			spec.ToolHash = [32]byte{}
		case "binary-zero":
			spec.BinaryHash = [32]byte{}
		case "manifest-zero":
			spec.ManifestHash = [32]byte{}
		case "compile-zero":
			spec.CompileReceiptHash = [32]byte{}
		case "old-zero":
			spec.OldPageHash = [32]byte{}
		case "new-zero":
			spec.NewPageHash = [32]byte{}
		case "partial-zero":
			spec.PartialPageHash = [32]byte{}
		case "old-new-equal":
			spec.NewPageHash = spec.OldPageHash
		case "old-partial-equal":
			spec.PartialPageHash = spec.OldPageHash
		case "new-partial-equal":
			spec.PartialPageHash = spec.NewPageHash
		case "identity-label":
			spec.Plan.Identity.Source, problem = "unverified-source-name", RecoveryProblemIdentity
		case "stale-trial":
			evidence.Identity.Trial, problem = "old-trial", RecoveryProblemIdentity
		case "stale-scope":
			evidence.Identity.Scope, problem = "old-scope", RecoveryProblemIdentity
		case "unbounded-id":
			var oversized [MaxIDLength + 1]byte
			for i := range oversized {
				oversized[i] = 'a'
			}
			evidence.Identity.Scope, problem = string(oversized[:]), RecoveryProblemIdentity
		case "stale-source":
			evidence.Provenance.SourceHash, problem = [32]byte{99}, RecoveryProblemProvenance
		case "stale-tool":
			evidence.Provenance.ToolHash, problem = [32]byte{99}, RecoveryProblemProvenance
		case "stale-binary":
			evidence.Provenance.BinaryHash, problem = [32]byte{99}, RecoveryProblemProvenance
		case "stale-manifest":
			evidence.Provenance.ManifestHash, problem = [32]byte{99}, RecoveryProblemProvenance
		case "missing-compile":
			evidence.Provenance.CompileReceiptHash, problem = [32]byte{}, RecoveryProblemProvenance
		case "unbound-compile-source":
			evidence.Provenance.CompileSourceHash, problem = [32]byte{99}, RecoveryProblemProvenance
		case "unbound-compile-tool":
			evidence.Provenance.CompileToolHash, problem = [32]byte{99}, RecoveryProblemProvenance
		case "unbound-compile-binary":
			evidence.Provenance.CompileBinaryHash, problem = [32]byte{99}, RecoveryProblemProvenance
		case "unbound-compile-manifest":
			evidence.Provenance.CompileManifestHash, problem = [32]byte{99}, RecoveryProblemProvenance
		case "zero-source-count":
			evidence.Provenance.SourceFiles, problem = 0, RecoveryProblemProvenance
		case "too-many-sources":
			evidence.Provenance.SourceFiles, problem = MaxRecoverySourceFiles+1, RecoveryProblemProvenance
		case "manifest-count":
			evidence.Provenance.ManifestFiles, problem = 16, RecoveryProblemProvenance
		case "compile-count":
			evidence.Provenance.CompileFiles, problem = 16, RecoveryProblemProvenance
		}
		t.Run(name, func(t *testing.T) { recoveryMustReject(t, spec, evidence, problem) })
	}
}

func TestRollbackRecoveryRejectsCommitAndAdapterFalsePass(t *testing.T) {
	for _, name := range [...]string{"arm-zero", "arm-two", "commit-zero", "commit-two", "finish-zero", "finish-two", "disarm-zero", "disarm-two", "disarm-error", "disarm-token", "native-extra-error", "replaced-return", "masked-short-write", "masked-injected", "missing-diagnostic", "stale-plan", "adapter-unknown", "adapter-refused", "adapter-incomplete", "adapter-invalid", "adapter-zero-hits", "adapter-two-hits", "adapter-wrong-count", "adapter-wrong-hash", "adapter-zero-hash", "adapter-hidden-tail", "adapter-overflow", "adapter-zero-count"} {
		selected := RecoveryPagePrefix64
		if name == "masked-injected" {
			selected = RecoveryDBSyncBefore
		}
		spec, evidence := recoveryFixture(t, selected)
		problem := RecoveryProblemAdapter
		switch name {
		case "arm-zero":
			evidence.ArmCalls, problem = 0, RecoveryProblemCalls
		case "arm-two":
			evidence.ArmCalls, problem = 2, RecoveryProblemCalls
		case "commit-zero":
			evidence.CommitCalls, problem = 0, RecoveryProblemCalls
		case "commit-two":
			evidence.CommitCalls, problem = 2, RecoveryProblemCalls
		case "finish-zero":
			evidence.FinishCalls, problem = 0, RecoveryProblemCalls
		case "finish-two":
			evidence.FinishCalls, problem = 2, RecoveryProblemCalls
		case "disarm-zero":
			evidence.DisarmCalls, problem = 0, RecoveryProblemCalls
		case "disarm-two":
			evidence.DisarmCalls, problem = 2, RecoveryProblemCalls
		case "disarm-error":
			evidence.DisarmError, problem = Underlying, RecoveryProblemCalls
		case "disarm-token":
			evidence.DisarmErrorToken, problem = 81, RecoveryProblemCalls
		case "native-extra-error":
			evidence.UnexpectedErrorCount, problem = 1, RecoveryProblemCalls
		case "replaced-return":
			evidence.CommitReturn.ErrorToken, problem = 901, RecoveryProblemCommit
		case "masked-short-write", "masked-injected":
			evidence.CommitReturn.TargetError, problem = None, RecoveryProblemCommit
		case "missing-diagnostic":
			evidence.CommitDiagnosticHash, problem = [32]byte{}, RecoveryProblemCommit
		case "stale-plan":
			evidence.AdapterPlan.Identity.Trial = "old-trial"
		case "adapter-unknown":
			evidence.AdapterObservation = Observation{}
		case "adapter-refused":
			evidence.AdapterObservation.Outcome = ObservationRefused
		case "adapter-incomplete":
			evidence.AdapterObservation.Outcome = ObservationIncomplete
		case "adapter-invalid":
			evidence.AdapterObservation.Outcome = ObservationInvalid
		case "adapter-zero-hits":
			evidence.AdapterObservation.TargetHits = 0
		case "adapter-two-hits":
			evidence.AdapterObservation.TargetHits = 2
		case "adapter-wrong-count":
			evidence.AdapterObservation.EventCount--
		case "adapter-wrong-hash":
			evidence.AdapterHash, problem = [32]byte{88}, RecoveryProblemAdapterHash
		case "adapter-zero-hash":
			evidence.AdapterHash, problem = [32]byte{}, RecoveryProblemAdapterHash
		case "adapter-hidden-tail":
			evidence.AdapterEvents[evidence.AdapterEventCount] = evidence.AdapterEvents[0]
		case "adapter-overflow":
			evidence.AdapterEventCount = MaxEvents + 1
		case "adapter-zero-count":
			evidence.AdapterEventCount = 0
		}
		t.Run(name, func(t *testing.T) { recoveryMustReject(t, spec, evidence, problem) })
	}
	for _, class := range [...]CommitClass{CommitUnknown, CommitFailed, CommitUncertain, CommitCleanupPending, CommitUnsupported, CommitRefused, CommitSkipped} {
		spec, evidence := recoveryFixture(t, RecoveryPagePrefix64)
		index := 1 + 2*int(spec.Plan.CallCount)
		evidence.AdapterEvents[index].Commit.Class = class
		evidence.CommitReturn.Class = class
		recoveryRehash(t, &evidence)
		recoveryMustReject(t, spec, evidence, RecoveryProblemAdapter)
	}
}

func TestRollbackRecoveryNestedTranscriptEveryTruncationOmissionDuplicateAndOrder(t *testing.T) {
	spec, original := recoveryFixture(t, RecoveryPagePrefix64)
	count := int(original.AdapterEventCount)
	for end := 0; end < count; end++ {
		evidence := original
		evidence.AdapterEventCount = uint8(end)
		for i := end; i < MaxEvents; i++ {
			evidence.AdapterEvents[i] = Event{}
		}
		if end > 0 {
			recoveryRehash(t, &evidence)
		}
		recoveryMustReject(t, spec, evidence, RecoveryProblemAdapter)
	}
	for index := 0; index < count; index++ {
		for mode := 0; mode < 3; mode++ {
			if mode == 2 && index+1 == count {
				continue
			}
			evidence := original
			switch mode {
			case 0:
				for i := index; i < count-1; i++ {
					evidence.AdapterEvents[i] = original.AdapterEvents[i+1]
				}
				evidence.AdapterEvents[count-1], evidence.AdapterEventCount = Event{}, uint8(count-1)
			case 1:
				for i := count; i > index; i-- {
					evidence.AdapterEvents[i] = original.AdapterEvents[i-1]
				}
				evidence.AdapterEventCount++
			case 2:
				evidence.AdapterEvents[index], evidence.AdapterEvents[index+1] = evidence.AdapterEvents[index+1], evidence.AdapterEvents[index]
			}
			for i := 0; i < int(evidence.AdapterEventCount); i++ {
				evidence.AdapterEvents[i].Ordinal = uint8(i + 1)
			}
			// Recompute hashes and ordinals: rejection must be semantic, never
			// just a stale checksum or an unchanged ordinal mismatch.
			recoveryRehash(t, &evidence)
			recoveryMustReject(t, spec, evidence, RecoveryProblemAdapter)
		}
	}
	for _, name := range [...]string{"stale-source", "stale-binary", "stale-trial", "target-duplicate", "short-native-63", "short-native-error", "masked-original-cause", "owner-unknown", "owner-live", "owner-pending", "validation-claim"} {
		evidence := original
		target := 2 + 2*8
		commit := 1 + 2*int(spec.Plan.CallCount)
		switch name {
		case "stale-source":
			evidence.AdapterEvents[1].Identity.Source = "old-source"
		case "stale-binary":
			evidence.AdapterEvents[1].Identity.Binary = "old-binary"
		case "stale-trial":
			evidence.AdapterEvents[1].Identity.Trial = "old-trial"
		case "target-duplicate":
			evidence.AdapterEvents[1].Call = spec.Plan.Target
		case "short-native-63":
			evidence.AdapterEvents[target].Result.NativeN = 63
		case "short-native-error":
			evidence.AdapterEvents[target].Result.NativeError = Underlying
		case "masked-original-cause":
			evidence.AdapterEvents[commit].Commit.TargetError = None
		case "owner-unknown":
			evidence.AdapterEvents[commit+1].Owners.Main = OwnerUnknown
		case "owner-live":
			evidence.AdapterEvents[commit+1].Owners.Main = OwnerLive
		case "owner-pending":
			evidence.AdapterEvents[commit+1].Owners.Main = OwnerPending
		case "validation-claim":
			evidence.AdapterEvents[commit+2].Validation = Validation(1)
		}
		recoveryRehash(t, &evidence)
		t.Run(name, func(t *testing.T) { recoveryMustReject(t, spec, evidence, RecoveryProblemAdapter) })
	}
}

func TestRollbackRecoveryRejectsRetainedInputAndReplayFalsePass(t *testing.T) {
	for _, name := range [...]string{"wrong-role", "source-inode-copy", "retained-inode-copy", "journal-inode-copy", "missing-baseline", "zero-inode", "zero-device", "wrong-db-size", "wrong-journal-size", "zero-hash", "baseline-mutated", "expected-mutated", "trial-mutated", "trial-journal-consumed", "retained-db-mutated", "retained-journal-consumed", "retained-journal-mismatch", "replay-source-mismatch", "replay-journal-mismatch", "replay-replaced-inode", "replay-journal-remains", "replay-not-restored", "replay-wrong-extent", "control-journal-present", "control-absence-with-payload", "control-restored-old"} {
		selected := RecoveryPagePrefix64
		if name == "control-journal-present" || name == "control-absence-with-payload" || name == "control-restored-old" {
			selected = RecoveryControl
		}
		spec, evidence := recoveryFixture(t, selected)
		problem := RecoveryProblemFiles
		switch name {
		case "wrong-role":
			evidence.Files[0].Role = RecoveryTrialDatabase
		case "source-inode-copy":
			evidence.Files[6].Before.Inode, evidence.Files[6].After.Inode, problem = evidence.Files[2].Before.Inode, evidence.Files[2].Before.Inode, RecoveryProblemAliases
		case "retained-inode-copy":
			evidence.Files[4].Before.Inode, evidence.Files[4].After.Inode, problem = evidence.Files[2].Before.Inode, evidence.Files[2].Before.Inode, RecoveryProblemAliases
		case "journal-inode-copy":
			evidence.Files[7].Before.Inode, problem = evidence.Files[3].Before.Inode, RecoveryProblemAliases
		case "missing-baseline":
			evidence.Files[0].Before = RecoveryFileSnapshot{}
		case "zero-inode":
			evidence.Files[0].Before.Inode = 0
		case "zero-device":
			evidence.Files[0].Before.Device = 0
		case "wrong-db-size":
			evidence.Files[2].Before.Size = 8192
		case "wrong-journal-size":
			evidence.Files[3].Before.Size = 8199
		case "zero-hash":
			evidence.Files[3].Before.Hash = [32]byte{}
		case "baseline-mutated":
			evidence.Files[0].After.Hash = spec.NewPageHash
		case "expected-mutated":
			evidence.Files[1].After.Hash = spec.OldPageHash
		case "trial-mutated":
			evidence.Files[2].After.Hash = spec.OldPageHash
		case "trial-journal-consumed":
			evidence.Files[3].After = RecoveryFileSnapshot{}
		case "retained-db-mutated":
			evidence.Files[4].After.Hash = spec.OldPageHash
		case "retained-journal-consumed":
			evidence.Files[5].Before, evidence.Files[5].After = RecoveryFileSnapshot{}, RecoveryFileSnapshot{}
		case "retained-journal-mismatch":
			evidence.Files[5].Before.Hash, evidence.Files[5].After.Hash = [32]byte{33}, [32]byte{33}
		case "replay-source-mismatch":
			evidence.Files[6].Before.Hash = spec.OldPageHash
		case "replay-journal-mismatch":
			evidence.Files[7].Before.Hash = [32]byte{33}
		case "replay-replaced-inode":
			evidence.Files[6].After.Inode = 999
		case "replay-journal-remains":
			evidence.Files[7].After = evidence.Files[7].Before
		case "replay-not-restored":
			evidence.Files[6].After.Hash = spec.PartialPageHash
		case "replay-wrong-extent":
			evidence.Files[6].After.Size = 8192
		case "control-journal-present":
			evidence.Files[3].Before, evidence.Files[3].After = evidence.Files[2].Before, evidence.Files[2].After
		case "control-absence-with-payload":
			evidence.Files[3].Before.Hash = [32]byte{33}
		case "control-restored-old":
			evidence.Files[6].After.Hash = spec.OldPageHash
		}
		t.Run(name, func(t *testing.T) { recoveryMustReject(t, spec, evidence, problem) })
	}
}

func TestRollbackRecoveryRejectsRecoveryReadbackOwnerAndTerminalFalsePass(t *testing.T) {
	for _, name := range [...]string{"no-recovery-but-old-page", "recovery-retried", "not-recovered", "wrong-restored-count", "wrong-original-count", "recovery-error", "recovery-token", "journal-limit", "database-limit", "record-limit", "registry-unchecked", "registry-once", "registry-before-nonempty", "registry-after-nonempty", "pending-before", "pending-after", "control-fake-recovery", "control-fake-result", "reader-zero", "reader-two", "header-only-comparison", "nonheader-byte-wrong", "wrong-page-hash", "read-before-extent", "read-after-extent", "header-cookie", "header-counter", "header-version", "header-encoding", "header-pages", "header-page-size", "header-read-version", "header-write-version", "reader-page-number", "reader-short", "reader-error", "reader-close-error", "reader-double-close", "reader-unclosed", "owner-missing", "owner-overflow", "owner-duplicate", "owner-unknown", "owner-live", "owner-pending", "owner-unacquired", "file-close-failed", "file-double-close", "file-unclosed", "file-double-open", "file-unknown-kind", "owner-hidden-tail", "reader-owner-mismatch", "two-reader-owners", "no-reader-owner", "terminal-truncated", "terminal-overflow", "terminal-duplicate", "terminal-reordered", "terminal-unknown"} {
		selected := RecoveryPagePrefix64
		if name == "control-fake-recovery" || name == "control-fake-result" {
			selected = RecoveryControl
		}
		spec, evidence := recoveryFixture(t, selected)
		problem := RecoveryProblemRecovery
		switch name {
		case "no-recovery-but-old-page":
			evidence.Recovery.Calls = 0
		case "recovery-retried":
			evidence.Recovery.Calls = 2
		case "not-recovered":
			evidence.Recovery.Recovered = false
		case "wrong-restored-count":
			evidence.Recovery.RestoredPages = 0
		case "wrong-original-count":
			evidence.Recovery.OriginalPages = 2
		case "recovery-error":
			evidence.Recovery.Error = Underlying
		case "recovery-token":
			evidence.Recovery.ErrorToken = 77
		case "journal-limit":
			evidence.Recovery.MaxJournalBytes++
		case "database-limit":
			evidence.Recovery.MaxDatabaseBytes++
		case "record-limit":
			evidence.Recovery.MaxRecords++
		case "registry-unchecked":
			evidence.Recovery.RegistryChecks = 0
		case "registry-once":
			evidence.Recovery.RegistryChecks = 1
		case "registry-before-nonempty":
			evidence.Recovery.RegistryBefore.PlainGroups = 1
		case "registry-after-nonempty":
			evidence.Recovery.RegistryAfter.PlainGroups = 1
		case "pending-before":
			evidence.Recovery.RegistryBefore.Pending = 1
		case "pending-after":
			evidence.Recovery.RegistryAfter.Pending = 1
		case "control-fake-recovery":
			evidence.Recovery.Calls = 1
		case "control-fake-result":
			evidence.Recovery.Recovered = true
		case "reader-zero":
			evidence.Reader.ReadCalls, problem = 0, RecoveryProblemReadback
		case "reader-two":
			evidence.Reader.ReadCalls, problem = 2, RecoveryProblemReadback
		case "header-only-comparison":
			evidence.Reader.ComparedBytes, problem = 100, RecoveryProblemReadback
		case "nonheader-byte-wrong":
			evidence.Reader.MatchingBytes, problem = 4095, RecoveryProblemReadback
		case "wrong-page-hash":
			evidence.Reader.PageHash, problem = [32]byte{33}, RecoveryProblemReadback
		case "read-before-extent":
			evidence.Reader.ExtentBefore, problem = 8192, RecoveryProblemReadback
		case "read-after-extent":
			evidence.Reader.ExtentAfter, problem = 8192, RecoveryProblemReadback
		case "header-cookie":
			evidence.Reader.Header.SchemaCookie, problem = 10, RecoveryProblemReadback
		case "header-counter":
			evidence.Reader.Header.ChangeCounter, problem = 8, RecoveryProblemReadback
		case "header-version":
			evidence.Reader.Header.VersionValidFor, problem = 8, RecoveryProblemReadback
		case "header-encoding":
			evidence.Reader.Header.Encoding, problem = 2, RecoveryProblemReadback
		case "header-pages":
			evidence.Reader.Header.DatabasePages, problem = 2, RecoveryProblemReadback
		case "header-page-size":
			evidence.Reader.Header.PageSize, problem = 512, RecoveryProblemReadback
		case "header-read-version":
			evidence.Reader.Header.ReadVersion, problem = 2, RecoveryProblemReadback
		case "header-write-version":
			evidence.Reader.Header.WriteVersion, problem = 2, RecoveryProblemReadback
		case "reader-page-number":
			evidence.Reader.PageNumber, problem = 2, RecoveryProblemReadback
		case "reader-short":
			evidence.Reader.BytesRead, problem = 4095, RecoveryProblemReadback
		case "reader-error":
			evidence.Reader.Error, problem = Underlying, RecoveryProblemReadback
		case "reader-close-error":
			evidence.Reader.CloseError, problem = Underlying, RecoveryProblemReadback
		case "reader-double-close":
			evidence.Reader.CloseCalls, problem = 2, RecoveryProblemReadback
		case "reader-unclosed":
			evidence.Reader.CloseCalls, problem = 0, RecoveryProblemReadback
		case "owner-missing":
			evidence.OwnerCount, problem = 0, RecoveryProblemOwners
		case "owner-overflow":
			evidence.OwnerCount, problem = MaxRecoveryOwners+1, RecoveryProblemOwners
		case "owner-duplicate":
			evidence.Owners[0].ID, problem = evidence.Owners[1].ID, RecoveryProblemOwners
		case "owner-unknown":
			evidence.Owners[0].State, problem = OwnerUnknown, RecoveryProblemOwners
		case "owner-live":
			evidence.Owners[0].State, problem = OwnerLive, RecoveryProblemOwners
		case "owner-pending":
			evidence.Owners[0].State, problem = OwnerPending, RecoveryProblemOwners
		case "owner-unacquired":
			evidence.Owners[0].State, problem = OwnerNotAcquired, RecoveryProblemOwners
		case "file-close-failed":
			evidence.Owners[0].CloseError, problem = Underlying, RecoveryProblemOwners
		case "file-double-close":
			evidence.Owners[0].CloseCalls, problem = 2, RecoveryProblemOwners
		case "file-unclosed":
			evidence.Owners[0].CloseCalls, problem = 0, RecoveryProblemOwners
		case "file-double-open":
			evidence.Owners[0].OpenCalls, problem = 2, RecoveryProblemOwners
		case "file-unknown-kind":
			evidence.Owners[0].Kind, problem = RecoveryOwnerUnknown, RecoveryProblemOwners
		case "owner-hidden-tail":
			evidence.Owners[2], problem = evidence.Owners[0], RecoveryProblemOwners
		case "reader-owner-mismatch":
			evidence.Owners[1].ID, problem = 999, RecoveryProblemOwners
		case "two-reader-owners":
			evidence.Owners[0].Kind, problem = RecoveryReaderOwner, RecoveryProblemOwners
		case "no-reader-owner":
			evidence.Owners[1].Kind, problem = RecoveryFileOwner, RecoveryProblemOwners
		case "terminal-truncated":
			evidence.StageCount, problem = RecoveryStageCount-1, RecoveryProblemTerminal
		case "terminal-overflow":
			evidence.StageCount, problem = RecoveryStageCount+1, RecoveryProblemTerminal
		case "terminal-duplicate":
			evidence.Stages[8], problem = RecoveryReaderClosed, RecoveryProblemTerminal
		case "terminal-reordered":
			evidence.Stages[1], evidence.Stages[2], problem = evidence.Stages[2], evidence.Stages[1], RecoveryProblemTerminal
		case "terminal-unknown":
			evidence.Stages[9], problem = RecoveryStageUnknown, RecoveryProblemTerminal
		}
		t.Run(name, func(t *testing.T) { recoveryMustReject(t, spec, evidence, problem) })
	}
	spec, complete := recoveryFixture(t, RecoveryDBSyncBefore)
	for end := 0; end < RecoveryStageCount; end++ {
		evidence := complete
		evidence.StageCount = uint8(end)
		for i := end; i < RecoveryStageCount; i++ {
			evidence.Stages[i] = RecoveryStageUnknown
		}
		recoveryMustReject(t, spec, evidence, RecoveryProblemTerminal)
	}
	for index := 0; index < RecoveryStageCount; index++ {
		evidence := complete
		evidence.Stages[index] = RecoveryStageUnknown
		recoveryMustReject(t, spec, evidence, RecoveryProblemTerminal)
		if index+1 < RecoveryStageCount {
			evidence = complete
			evidence.Stages[index], evidence.Stages[index+1] = evidence.Stages[index+1], evidence.Stages[index]
			recoveryMustReject(t, spec, evidence, RecoveryProblemTerminal)
		}
	}
}

func TestRollbackRecoveryHashBindsFullNestedPlanAndEventPayload(t *testing.T) {
	_, original := recoveryFixture(t, RecoveryPagePrefix64)
	for _, name := range [...]string{"plan-owner", "event-identity", "event-ordinal", "event-kind", "event-call", "event-action", "event-native", "event-commit", "event-owners", "event-validation", "event-rejection", "event-rejected-at"} {
		evidence := original
		switch name {
		case "plan-owner":
			evidence.AdapterPlan.CleanupOwner++
		case "event-identity":
			evidence.AdapterEvents[0].Identity.Scope = "another-scope"
		case "event-ordinal":
			evidence.AdapterEvents[0].Ordinal++
		case "event-kind":
			evidence.AdapterEvents[0].Kind = Terminal
		case "event-call":
			evidence.AdapterEvents[0].Call.Owner++
		case "event-action":
			evidence.AdapterEvents[0].Action = FailBefore
		case "event-native":
			evidence.AdapterEvents[0].Result.NativeN++
		case "event-commit":
			evidence.AdapterEvents[0].Commit.TargetError = Injected
		case "event-owners":
			evidence.AdapterEvents[0].Owners.Main = OwnerRetired
		case "event-validation":
			evidence.AdapterEvents[0].Validation = Validation(1)
		case "event-rejection":
			evidence.AdapterEvents[0].Rejection = ProblemIdentity
		case "event-rejected-at":
			evidence.AdapterEvents[0].RejectedAt++
		}
		digest, problem := RecoveryAdapterHash(evidence.AdapterPlan, evidence.AdapterEvents, evidence.AdapterEventCount)
		if problem != RecoveryProblemNone || digest == original.AdapterHash {
			t.Fatalf("hash did not bind %s: problem=%v", name, problem)
		}
	}
	var invalid [MaxEvents]Event
	if _, problem := RecoveryAdapterHash(original.AdapterPlan, invalid, 1); problem != RecoveryProblemIdentity {
		t.Fatalf("unbounded/unadmitted identity accepted: %v", problem)
	}
	if _, problem := RecoveryAdapterHash(original.AdapterPlan, original.AdapterEvents, 0); problem == RecoveryProblemNone {
		t.Fatal("zero transcript hash accepted")
	}
	if _, problem := RecoveryAdapterHash(original.AdapterPlan, original.AdapterEvents, MaxEvents+1); problem == RecoveryProblemNone {
		t.Fatal("oversized transcript hash accepted")
	}
	invalid = original.AdapterEvents
	invalid[original.AdapterEventCount] = invalid[0]
	if _, problem := RecoveryAdapterHash(original.AdapterPlan, invalid, original.AdapterEventCount); problem == RecoveryProblemNone {
		t.Fatal("hidden transcript tail ignored")
	}
	var oversized [MaxIDLength + 1]byte
	for i := range oversized {
		oversized[i] = 'x'
	}
	invalid = original.AdapterEvents
	invalid[0].Identity.Source = string(oversized[:])
	if _, problem := RecoveryAdapterHash(original.AdapterPlan, invalid, original.AdapterEventCount); problem != RecoveryProblemIdentity {
		t.Fatalf("unbounded identity reached the hash encoder: %v", problem)
	}
}
`,
}
