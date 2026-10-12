package main

// This finite source contract has no native runner. It is deliberately separate
// from every older guard: no manual-tag exemption or existing pin is widened.
// Its pins are proposals pending independent source review, not validation or
// authority to compile/run the manual adapter. AST checks are a conservative
// lexical boundary, not a type checker or proof about imported implementations.
import (
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const rollbackFaultCommit = "internal/pager/commit.go"
const rollbackFaultPoints = "internal/pager/rollback_io_points.go"
const rollbackFaultDefault = "internal/pager/rollback_io_default.go"
const rollbackFaultManual = "internal/pager/rollback_io_manual_linux.go"
const rollbackFaultWriterTest = "internal/pager/rollback_io_contract_test.go"
const rollbackFaultContract = "internal/testkit/rollbackfault/contract.go"
const rollbackFaultContractTest = "internal/testkit/rollbackfault/contract_test.go"
const rollbackFaultImport = "github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackfault"
const rollbackFaultManualTag = "//go:build linux && manual_fault_rollback"
const rollbackFaultDefaultTag = "//go:build !linux || !manual_fault_rollback"

var rollbackFaultPins = map[string]string{
	rollbackFaultCommit:       "9ae36612afac5cfae48bd972e6162c94722bcf7355ea462b4e15289b5188aa32",
	rollbackFaultPoints:       "e9f4349eac27fe2f2ee6726edeb3cd26e3f5e851e75ae342f2f3a7995f038905",
	rollbackFaultDefault:      "f1d45680aa93940d9d181f01ecc40f5e88c258970c940a28b4f19df712182bd1",
	rollbackFaultManual:       "7b918b0cebc53e228884a1c264600283e79b209562fe3f4d889e64e19114599b",
	rollbackFaultWriterTest:   "a7695da7b49488be37a98b239b1b31d59bc406455d0b3300709081fd96ae49c4",
	rollbackFaultContract:     "dc7ec44ec7be3f444f4e7501c84d6875a1a0e9b4d46e06e5586b2e5d225828ed",
	rollbackFaultContractTest: "8c43b22c2fbfdc6d4aef423abd44956196a2b15e223a8f56ef00065c8d77c362",
}

var rollbackFaultPointIDs = map[string]string{
	"rbJCreate":        "RB.JCREATE",
	"rbJInvalid":       "RB.JINVALID",
	"rbDBOriginalRead": "RB.DB_ORIGINAL_READ",
	"rbJRecord":        "RB.JRECORD",
	"rbJBodySync":      "RB.JBODY_SYNC",
	"rbJValidHeader":   "RB.JVALID_HEADER",
	"rbJHeaderSync":    "RB.JHEADER_SYNC",
	"rbPublishDirSync": "RB.PUBLISH_DIRSYNC",
	"rbDBPage":         "RB.DB_PAGE",
	"rbDBSync":         "RB.DB_SYNC",
	"rbJUnlink":        "RB.JUNLINK",
	"rbFinalDirSync":   "RB.FINAL_DIRSYNC",
	"rbCleanupUnlink":  "RB.CLEANUP_UNLINK",
	"rbCleanupDirSync": "RB.CLEANUP_DIRSYNC",
}

// Exact call/argument/source identities matter, not just fourteen calls. The
// failure-closure calls are included; oracleEnclosingFunction intentionally
// does not cross closures, so the commit check uses the explicit Commit node.
var rollbackFaultCommitCalls = map[string]string{
	"rbJCreate":        "rollbackCreate(t, rbJCreate, jpath, os.O_WRONLY|os.O_CREATE|os.O_EXCL)",
	"rbJInvalid":       "rollbackWrite(t, rbJInvalid, jf, invalid, 0)",
	"rbDBOriginalRead": "rollbackRead(t, rbDBOriginalRead, t.file, page, int64(n-1)*int64(h.PageSize))",
	"rbJRecord":        "rollbackWrite(t, rbJRecord, jf, record, offset)",
	"rbJBodySync":      "rollbackFileSync(t, rbJBodySync, jf)",
	"rbJValidHeader":   "rollbackWrite(t, rbJValidHeader, jf, valid[:12], 0)",
	"rbJHeaderSync":    "rollbackFileSync(t, rbJHeaderSync, jf)",
	"rbPublishDirSync": "rollbackDirectorySync(t, rbPublishDirSync, t.path)",
	"rbDBPage":         "rollbackWrite(t, rbDBPage, t.file, data, int64(n-1)*int64(t.header.PageSize))",
	"rbDBSync":         "rollbackFileSync(t, rbDBSync, t.file)",
	"rbJUnlink":        "rollbackUnlink(t, rbJUnlink, jpath)",
	"rbFinalDirSync":   "rollbackDirectorySync(t, rbFinalDirSync, t.path)",
	"rbCleanupUnlink":  "rollbackUnlink(t, rbCleanupUnlink, jpath)",
	"rbCleanupDirSync": "rollbackDirectorySync(t, rbCleanupDirSync, t.path)",
}

func rollbackFaultBridge(name string) bool {
	switch name {
	case "rollbackCreate", "rollbackRead", "rollbackWrite", "rollbackFileSync", "rollbackDirectorySync", "rollbackUnlink":
		return true
	}
	return false
}

func rollbackFaultManualPrivate(name string) bool {
	switch name {
	case "rollbackFaultRegistry", "rollbackFaultScope", "rollbackFaultTicket", "rollbackFaultReport", "rollbackFaultWriter", "rollbackFaultFor", "rollbackFaultPoint", "rollbackFaultBefore", "rollbackFaultAfter", "errRollbackFaultGate", "errRollbackFaultScope", "errRollbackFaultInjected":
		return true
	}
	return false
}

func rollbackFaultText(set *token.FileSet, raw []byte, node ast.Node) string {
	return strings.Join(strings.Fields(oracleNodeText(set, raw, node)), " ")
}

// Token equality preserves string/number/character literal values. Whitespace
// inside a quoted diagnostic is not erased, unlike a Fields-only comparison.
// Comments and spacing are immaterial here; their byte changes still hit pins.
func rollbackFaultShapeTokens(raw string) string {
	set := token.NewFileSet()
	file := set.AddFile("declaration", -1, len(raw))
	var scan scanner.Scanner
	scan.Init(file, []byte(raw), nil, 0)
	var out strings.Builder
	for {
		_, kind, literal := scan.Scan()
		if kind == token.EOF {
			break
		}
		if kind == token.SEMICOLON {
			literal = ";"
		}
		out.WriteString(kind.String())
		out.WriteByte(':')
		out.WriteString(strconv.Quote(literal))
		out.WriteByte('\n')
	}
	return out.String()
}

func rollbackFaultDirectReference(id *ast.Ident, parents map[ast.Node]ast.Node) *ast.CallExpr {
	call, ok := parents[id].(*ast.CallExpr)
	if !ok || call.Fun != id {
		return nil
	}
	return call
}

func rollbackFaultInside(node ast.Node, fn *ast.FuncDecl) bool {
	return fn != nil && fn.Body != nil && node.Pos() > fn.Body.Pos() && node.End() < fn.Body.End()
}

func inspectRollbackFaultSource(path string, raw []byte) ([]string, error) {
	path = filepath.ToSlash(filepath.Clean(path))
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, path, raw, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	var findings []string
	report := func(message string) { findings = append(findings, path+": rollback fault contract: "+message) }
	_, owned := rollbackFaultPins[path]
	adapter := path == rollbackFaultDefault || path == rollbackFaultManual
	pure := path == rollbackFaultContract || path == rollbackFaultContractTest || path == rollbackFaultWriterTest
	if strings.HasPrefix(path, "internal/testkit/rollbackfault/") && !owned {
		report("unreviewed contract-package source")
	}
	if strings.HasPrefix(path, "internal/pager/rollback_io_") && !owned {
		report("unreviewed adapter source or native caller")
	}
	first := strings.SplitN(string(raw), "\n", 2)[0]
	if strings.Contains(first, "manual_fault_rollback") && !adapter {
		report("manual tag outside exact two-file selection")
	}
	for _, group := range file.Comments {
		for _, comment := range group.List {
			if (strings.HasPrefix(comment.Text, "//go:build") || strings.HasPrefix(comment.Text, "// +build")) && strings.Contains(comment.Text, "manual_fault_rollback") && !adapter {
				report("manual tag outside exact two-file selection")
			}
		}
	}
	if owned {
		if fmt.Sprintf("%x", sha256.Sum256(raw)) != rollbackFaultPins[path] {
			report("proposed complete-source pin changed")
		}
		if adapter && (strings.Count(string(raw), "//go:build") != 1 || strings.Contains(string(raw), "// +build")) {
			report("adapter requires one modern build line and no alternate constraints")
		}
		switch path {
		case rollbackFaultDefault:
			if first != rollbackFaultDefaultTag {
				report("exact default build complement required")
			}
		case rollbackFaultManual:
			if first != rollbackFaultManualTag {
				report("exact Linux AND manual build gate required")
			}
		default:
			if strings.Contains(string(raw), "//go:build") || strings.Contains(string(raw), "// +build") {
				report("portable source cannot add build constraints")
			}
		}
	}
	imports := map[string]string{}
	for _, imp := range file.Imports {
		value, err := strconv.Unquote(imp.Path.Value)
		if err != nil {
			return nil, err
		}
		name := filepath.Base(value)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		imports[name] = value
		if value == rollbackFaultImport && (path != rollbackFaultManual || imp.Name != nil) {
			report("contract production consumer must be the exact unaliased manual file")
		}
		if owned && imp.Name != nil {
			report("aliased, dot or blank import outside closed source binding")
		}
		if owned && (value == "unsafe" || value == "reflect" || value == "os/exec" || value == "syscall" || strings.HasPrefix(value, "net") || value == "golang.org/x/sys/unix") {
			report("unreviewed native/process/network import")
		}
	}
	parents := oracleParents(file)
	var commit *ast.FuncDecl
	for _, declaration := range file.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if path == rollbackFaultCommit && fn.Name.Name == "Commit" {
			commit = fn
		}
		if owned && (fn.Name.Name == "init" || fn.Name.Name == "TestMain") {
			report("initializer or TestMain is forbidden")
		}
		if (adapter || path == rollbackFaultPoints || path == rollbackFaultContract) && (strings.HasPrefix(fn.Name.Name, "Test") || strings.HasPrefix(fn.Name.Name, "Benchmark") || strings.HasPrefix(fn.Name.Name, "Fuzz") || strings.HasPrefix(fn.Name.Name, "Example")) {
			report("native execution entry in adapter implementation")
		}
	}
	seen := map[string]int{}
	ast.Inspect(file, func(node ast.Node) bool {
		if id, ok := node.(*ast.Ident); ok {
			if id.Name == "armRollbackFault" || id.Name == "finishRollbackFault" || id.Name == "disarmRollbackFault" {
				fn, declaration := parents[id].(*ast.FuncDecl)
				if !declaration || fn.Name != id || path != rollbackFaultManual || fn.Recv != nil {
					report("scope entry has no approved caller, alias, function value or copied declaration")
				}
			}
			if rollbackFaultManualPrivate(id.Name) && path != rollbackFaultManual {
				report("manual-private reference outside exact source")
			}
			if id.Name == "rollbackIOPoint" && path != rollbackFaultPoints && path != rollbackFaultDefault && path != rollbackFaultManual {
				report("private point type reference outside exact source")
			}
			if rollbackFaultBridge(id.Name) {
				if fn, declaration := parents[id].(*ast.FuncDecl); declaration && fn.Name == id && adapter && fn.Recv == nil {
					return true
				}
				call := rollbackFaultDirectReference(id, parents)
				if path != rollbackFaultCommit || call == nil || !rollbackFaultInside(call, commit) {
					report("bridge reference outside exact Commit call or adapter declaration")
					return true
				}
				matched := false
				for point, want := range rollbackFaultCommitCalls {
					if rollbackFaultText(set, raw, call) == want {
						seen[point]++
						matched = true
					}
				}
				if !matched {
					report("Commit bridge point/owner/offset/length/argument binding changed")
				}
			}
			if _, point := rollbackFaultPointIDs[id.Name]; point && path != rollbackFaultPoints && path != rollbackFaultCommit && path != rollbackFaultManual && path != rollbackFaultDefault {
				report("point reference outside exact production boundary")
			}
		}
		if call, ok := node.(*ast.CallExpr); ok && path != rollbackFaultManual {
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && (sel.Sel.Name == "Getenv" || sel.Sel.Name == "LookupEnv") {
				for _, arg := range call.Args {
					if literal, ok := arg.(*ast.BasicLit); ok && literal.Kind == token.STRING {
						value, e := strconv.Unquote(literal.Value)
						if e == nil && value == "GOSQLITE_MANUAL_FAULT_ROLLBACK" {
							report("manual environment consumer outside exact armer source")
						}
					}
				}
			}
		}
		if owned {
			switch node.(type) {
			case *ast.GoStmt, *ast.SendStmt:
				report("new concurrency execution is outside finite adapter scope")
			}
			if pure {
				if sel, ok := node.(*ast.SelectorExpr); ok {
					switch sel.Sel.Name {
					case "TempDir", "CreateTemp", "MkdirTemp", "Setenv", "Chdir", "Getenv", "LookupEnv", "Open", "OpenFile", "Create", "ReadFile", "WriteFile", "Remove", "RemoveAll", "Rename", "Close", "Sync", "Begin", "RecoverRollback", "RetryPlaintextCleanup", "Parallel", "Skip", "Skipf", "SkipNow":
						report("non-memory or skipped-test reference " + sel.Sel.Name)
					}
					// Event.Commit is a descriptor-free value record, not a
					// callable Tx. Closed declaration shapes forbid method
					// values/aliases too; this check singles out actual calls.
					if call, ok := parents[sel].(*ast.CallExpr); ok && call.Fun == sel && sel.Sel.Name == "Commit" {
						report("non-memory Commit call")
					}
				}
			}
		}
		return true
	})
	if path == rollbackFaultCommit {
		for point := range rollbackFaultCommitCalls {
			if seen[point] != 1 {
				report("missing/duplicate exact Commit binding " + point)
			}
		}
		findings = append(findings, inspectRollbackFaultCommit(path, raw, set, file)...)
	}
	if owned && path != rollbackFaultCommit {
		findings = append(findings, inspectRollbackFaultOwned(path, raw, set, file, imports)...)
	}
	return findings, nil
}

func inspectRollbackFaultContracts(root string) ([]string, error) {
	var findings []string
	for path := range rollbackFaultPins {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return nil, err
		}
		items, err := inspectRollbackFaultSource(path, raw)
		if err != nil {
			return nil, err
		}
		findings = append(findings, items...)
	}
	return findings, nil
}

func rollbackFaultDeclarationShape(path string, raw []byte, set *token.FileSet, file *ast.File, expected string) []string {
	var findings []string
	report := func(message string) { findings = append(findings, path+": rollback fault contract: "+message) }
	wantSet := token.NewFileSet()
	wantRaw := []byte(expected)
	want, err := parser.ParseFile(wantSet, "approved-source-shape", wantRaw, 0)
	if err != nil {
		report("invalid internal proposed declaration contract")
		return findings
	}
	if file.Name.Name != want.Name.Name || len(file.Decls) != len(want.Decls) {
		report("closed declaration/package inventory changed")
		return findings
	}
	for i, declaration := range file.Decls {
		actual := rollbackFaultShapeTokens(oracleNodeText(set, raw, declaration))
		approved := rollbackFaultShapeTokens(oracleNodeText(wantSet, wantRaw, want.Decls[i]))
		if actual != approved {
			report(fmt.Sprintf("closed declaration %d source binding/body/order changed", i))
		}
	}
	return findings
}

func inspectRollbackFaultCommit(path string, raw []byte, set *token.FileSet, file *ast.File) []string {
	findings := rollbackFaultDeclarationShape(path, raw, set, file, rollbackFaultCommitShape)
	writes := 0
	for _, declaration := range file.Decls {
		fn, ok := declaration.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "writeFullAt" {
			continue
		}
		writes++
		want := "func writeFullAt(f io.WriterAt, b []byte, off int64) error { n, err := f.WriteAt(b, off) if err != nil { return err } if n != len(b) { return io.ErrShortWrite } return nil }"
		if rollbackFaultText(set, raw, fn) != want {
			findings = append(findings, path+": rollback fault contract: WriterAt interface-only signature and original helper body required")
		}
	}
	if writes != 1 {
		findings = append(findings, path+": rollback fault contract: unique writeFullAt required")
	}
	return findings
}

func inspectRollbackFaultOwned(path string, raw []byte, set *token.FileSet, file *ast.File, imports map[string]string) []string {
	var findings []string
	report := func(message string) { findings = append(findings, path+": rollback fault contract: "+message) }
	if expected, ok := rollbackFaultOwnedShapes[path]; ok {
		findings = append(findings, rollbackFaultDeclarationShape(path, raw, set, file, expected)...)
	} else {
		report("missing closed declaration contract")
	}
	if path == rollbackFaultPoints {
		if len(imports) != 0 {
			report("point catalogue must have no imports")
		}
		constants := map[string]int{}
		types := 0
		for _, declaration := range file.Decls {
			gen, ok := declaration.(*ast.GenDecl)
			if !ok || (gen.Tok != token.CONST && gen.Tok != token.TYPE) {
				report("point catalogue contains executable or mutable declaration")
				continue
			}
			for _, spec := range gen.Specs {
				if typ, ok := spec.(*ast.TypeSpec); ok {
					types++
					if rollbackFaultText(set, raw, typ) != "rollbackIOPoint string" {
						report("point type must remain private non-alias string")
					}
					continue
				}
				value, ok := spec.(*ast.ValueSpec)
				if !ok || len(value.Names) != 1 || len(value.Values) != 1 || value.Type == nil || rollbackFaultText(set, raw, value.Type) != "rollbackIOPoint" {
					report("point must have one explicit private typed ID")
					continue
				}
				name := value.Names[0].Name
				constants[name]++
				literal, ok := value.Values[0].(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					report("point value must be a fixed string")
					continue
				}
				id, err := strconv.Unquote(literal.Value)
				if want, ok := rollbackFaultPointIDs[name]; err != nil || !ok || id != want {
					report("point ID/name binding changed")
				}
			}
		}
		if types != 1 || len(constants) != len(rollbackFaultPointIDs) {
			report("exact fourteen-point catalogue required")
		}
		for name := range rollbackFaultPointIDs {
			if constants[name] != 1 {
				report("point missing or duplicate " + name)
			}
		}
	}
	if path == rollbackFaultDefault {
		wantCalls := map[string]string{
			"rollbackCreate":        "openJournal(path, flags)",
			"rollbackRead":          "f.ReadAt(b, off)",
			"rollbackWrite":         "writeFullAt(f, b, off)",
			"rollbackFileSync":      "f.Sync()",
			"rollbackDirectorySync": "syncDirectory(path)",
			"rollbackUnlink":        "os.Remove(path)",
		}
		counts := map[string]int{}
		for _, declaration := range file.Decls {
			if gen, ok := declaration.(*ast.GenDecl); ok && gen.Tok != token.IMPORT {
				report("default cannot contain state, types or initializers")
			}
			fn, ok := declaration.(*ast.FuncDecl)
			if !ok {
				continue
			}
			counts[fn.Name.Name]++
			want, known := wantCalls[fn.Name.Name]
			if !known || fn.Recv != nil || fn.Body == nil || len(fn.Body.List) != 1 {
				report("default bridge must contain one direct original return")
				continue
			}
			ret, ok := fn.Body.List[0].(*ast.ReturnStmt)
			if !ok || len(ret.Results) != 1 || rollbackFaultText(set, raw, ret.Results[0]) != want {
				report("default bridge must return exact original owner call")
			}
		}
		if len(counts) != len(wantCalls) {
			report("exact six default bridges required")
		}
		for name := range wantCalls {
			if counts[name] != 1 {
				report("missing/duplicate default bridge " + name)
			}
		}
	}
	if path == rollbackFaultManual {
		armers := 0
		getenv := 0
		for _, declaration := range file.Decls {
			fn, ok := declaration.(*ast.FuncDecl)
			if !ok || fn.Name.Name != "armRollbackFault" {
				continue
			}
			armers++
			if fn.Body == nil || len(fn.Body.List) == 0 {
				report("armer lacks first-action runtime gate")
				continue
			}
			want := `if os.Getenv("GOSQLITE_MANUAL_FAULT_ROLLBACK") != "independently_authorized" { return nil, errRollbackFaultGate }`
			if rollbackFaultText(set, raw, fn.Body.List[0]) != want {
				report("armer first action must be exact rejecting runtime gate")
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			if selector, ok := node.(*ast.SelectorExpr); ok {
				if selector.Sel.Name == "Getenv" {
					getenv++
				}
				switch selector.Sel.Name {
				case "Fd", "SyscallConn", "RawConn", "Open", "OpenFile", "Create", "TempDir", "Mkdir", "MkdirAll", "CreateTemp", "Close", "RecoverRollback", "RetryPlaintextCleanup", "Setenv", "LookupEnv", "Environ", "Truncate", "Kill", "Signal", "Sleep":
					report("unreviewed native ownership/control reference " + selector.Sel.Name)
				}
			}
			return true
		})
		if armers != 1 || getenv != 1 {
			report("one armer and one environment read required")
		}
	}
	if path == rollbackFaultContract {
		if len(imports) != 0 {
			report("descriptor-free contract has no imports")
		}
		var fieldsPure func(ast.Expr) bool
		fieldsPure = func(expression ast.Expr) bool {
			switch typ := expression.(type) {
			case *ast.Ident:
				return typ.Name != "error" && typ.Name != "any" && typ.Name != "uintptr" && typ.Name != "unsafe"
			case *ast.ArrayType:
				return typ.Len != nil && fieldsPure(typ.Elt)
			case *ast.StructType:
				for _, field := range typ.Fields.List {
					if !fieldsPure(field.Type) {
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
				if gen.Tok == token.VAR {
					report("descriptor-free contract cannot store package state")
				}
				for _, spec := range gen.Specs {
					if typ, ok := spec.(*ast.TypeSpec); ok && (typ.Assign.IsValid() || !fieldsPure(typ.Type)) {
						report("contract types must be scalar or fixed-array value records, never descriptors/callbacks/interfaces")
					}
				}
			}
			if fn, ok := declaration.(*ast.FuncDecl); ok && (fn.Name.Name == "PhysicalValidated" || fn.Name.Name == "Pass") {
				physical[fn.Name.Name]++
				want := "func (Observation) " + fn.Name.Name + "() bool { return false }"
				if rollbackFaultText(set, raw, fn) != want {
					report("adapter observation can never confer physical PASS")
				}
			}
		}
		if physical["PhysicalValidated"] != 1 || physical["Pass"] != 1 {
			report("two explicit false physical-acceptance methods required")
		}
	}
	if path == rollbackFaultWriterTest {
		allowed := map[string]bool{"errors": true, "io": true, "testing": true}
		for _, value := range imports {
			if !allowed[value] {
				report("fake WriterAt tests must have only memory/assertion imports")
			}
		}
		for _, declaration := range file.Decls {
			fn, ok := declaration.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if fn.Name.Name != "WriteAt" && fn.Name.Name != "TestRollbackWriteFullAtMemoryContract" {
				report("extra fake-writer helper or native caller")
			}
		}
	}
	if path == rollbackFaultContractTest {
		if len(imports) != 1 || imports["testing"] != "testing" {
			report("receipt tests must have only the testing import")
		}
	}
	return findings
}

// The following explicit source declarations are proposed closed contracts, not
// learned at runtime or regenerated by the guard. Compare declarations, bodies,
// order, imports and package identity independently of the supplementary pins.
// Any intended edit requires independent review of both source and contract.
const rollbackFaultCommitShape = `package pager

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/ms-xiaosnow/gosqlite/internal/journal"

	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

// writeFullAt treats short writes as errors, even if the OS reports no error.
func writeFullAt(f io.WriterAt, b []byte, off int64) error {
	n, err := f.WriteAt(b, off)
	if err != nil {
		return err
	}
	if n != len(b) {
		return io.ErrShortWrite
	}
	return nil
}
func syncDirectory(path string) error { return nativeFileError(vfs.SyncDirectory(filepath.Dir(path))) }
func sortedPages(m map[uint32][]byte) []uint32 {
	out := make([]uint32, 0, len(m))
	for n := range m {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

// journalPages includes original images of complete storage sectors touched
// by the write, including old neighbors of newly appended pages. No original
// image is required for pages beyond the pre-transaction database extent.
func (t *Tx) journalPages(dirty []uint32) []uint32 {
	set := make(map[uint32]struct{})
	for _, n := range dirty {
		first, last := uint64(n), uint64(n)
		if t.opts.SectorSize > t.header.PageSize {
			per := uint64(t.opts.SectorSize / t.header.PageSize)
			first = (uint64(n)-1)/per*per + 1
			last = first + per - 1
		}
		if last > uint64(t.originalPages) {
			last = uint64(t.originalPages)
		}
		for p := first; p <= last && p != 0; p++ {
			if p != uint64(lockPage(t.header.PageSize)) {
				set[uint32(p)] = struct{}{}
			}
		}
	}
	pages := make([]uint32, 0, len(set))
	for n := range set {
		pages = append(pages, n)
	}
	sort.Slice(pages, func(i, j int) bool { return pages[i] < pages[j] })
	return pages
}

// Commit writes a standard SQLite DELETE-mode rollback journal, then the
// database. Order: EXCLUSIVE; invalid header + body; journal Sync; valid header
// + Sync; directory Sync; database writes + Sync; journal unlink; directory
// Sync; release locks. Atomicity after interruption is NOT acceptance-verified.
//
// Cancellation is checked until the journal is created. Once physical I/O
// starts, the entire commit or error cleanup completes without cancellation.
// A BUSY lock is returned before creating any sidecar or writing database data.
func (t *Tx) Commit(ctx context.Context) (retErr error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	defer t.clearClosedUndoLocked()
	cleanup := t.plainScope()
	if cleanup != nil {
		defer func() { retErr = cleanup.observe(retErr) }()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := t.checkOpen(); err != nil {
		return err
	}
	if t.statement {
		return fmt.Errorf("%w: commit inside statement", ErrMisuse)
	}
	if t.encrypted != nil {
		return t.commitEncryptedLocked(ctx)
	}
	if t.shared != nil {
		return t.commitSharedWALLocked(ctx)
	}
	if t.session != nil {
		return t.commitWALLocked(ctx)
	}
	if err := t.checkStable(); err != nil {
		return err
	}
	if len(t.dirty) == 0 {
		if t.cleanup != nil {
			return t.retirePlainLocked(nil, nil, true)
		}
		var err error
		if !t.keepFile {
			err = closeNativeFile(t.file)
		}
		t.file = nil
		t.free = nil
		return err
	}
	if err := t.promoteLocked(ctx); err != nil {
		return err
	}
	p1, err := t.readLocked(ctx, 1)
	if err != nil {
		return err
	}
	finalHeader := t.header
	finalHeader.ChangeCounter++
	finalHeader.VersionValidFor = finalHeader.ChangeCounter
	binary.BigEndian.PutUint32(p1[24:], finalHeader.ChangeCounter)
	binary.BigEndian.PutUint32(p1[28:], finalHeader.PageCount)
	binary.BigEndian.PutUint32(p1[92:], finalHeader.VersionValidFor)
	dirty := sortedPages(t.dirty)
	if len(dirty) == 0 || dirty[0] != 1 {
		return fmt.Errorf("%w: dirty transaction lacks page one", ErrMisuse)
	}
	originals := t.journalPages(dirty)
	var random [4]byte
	if _, err = rand.Read(random[:]); err != nil {
		return err
	}
	h := journal.Header{Records: uint32(len(originals)), Nonce: binary.BigEndian.Uint32(random[:]), OriginalPages: t.originalPages, SectorSize: t.opts.SectorSize, PageSize: t.header.PageSize}
	invalid, err := journal.EncodeHeader(h, false)
	if err != nil {
		return err
	}
	valid, err := journal.EncodeHeader(h, true)
	if err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	if err = lockExclusive(t.file); err != nil {
		if errors.Is(err, ErrPoisoned) {
			return t.poison(err, false)
		}
		return err
	}
	// Pre-I/O failures remain retryable and release PENDING/EXCLUSIVE while
	// retaining this transaction's SHARED+RESERVED locks.
	preflight := func(e error, journalAcquired bool) error {
		if cleanup != nil {
			e = cleanup.observe(e)
			if errors.Is(e, ErrPlaintextCleanupPending) {
				return t.deferPlainRetirementLocked(journalAcquired)
			}
		}
		if t.keepFile || t.exclusive {
			return e
		}
		if unlock := unlockExclusive(t.file); unlock != nil {
			return t.poison(errors.Join(e, unlock), false)
		}
		return e
	}
	if err = t.checkStable(); err != nil {
		return preflight(err, false)
	}
	if err = ctx.Err(); err != nil {
		return preflight(err, false)
	}
	jpath := t.path + "-journal"
	if cleanup != nil {
		if err = cleanup.reserveTemporary(); err != nil {
			return preflight(err, false)
		}
	}
	jf, err := rollbackCreate(t, rbJCreate, jpath, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
	if cleanup != nil {
		cleanup.acquiredTemporary(jf)
	}
	if err != nil {
		// The journal constructor may already have created the sidecar.
		return preflight(err, true)
	}
	databaseTouched := false
	// After a database write is attempted, even an error may have changed
	// bytes. Preserve the fully synced journal for explicit recovery.
	fail := func(cause error) error {
		if cleanup != nil {
			cause = cleanup.closeTemporary(cause)
			if errors.Is(cause, ErrPlaintextCleanupPending) {
				return t.deferPlainRetirementLocked(true)
			}
		} else {
			cause = errors.Join(cause, jf.Close())
		}
		if !databaseTouched {
			if e := rollbackUnlink(t, rbCleanupUnlink, jpath); e != nil {
				return t.poison(errors.Join(cause, e), true)
			}
			if e := rollbackDirectorySync(t, rbCleanupDirSync, t.path); e != nil {
				return t.poison(errors.Join(cause, e), true)
			}
		}
		return t.poison(cause, databaseTouched)
	}
	if err = rollbackWrite(t, rbJInvalid, jf, invalid, 0); err != nil {
		return fail(err)
	}
	offset := int64(h.SectorSize)
	page := make([]byte, h.PageSize)
	for _, n := range originals {
		if _, err = rollbackRead(t, rbDBOriginalRead, t.file, page, int64(n-1)*int64(h.PageSize)); err != nil {
			return fail(err)
		}
		record, e := journal.EncodeRecord(h, n, page)
		if e != nil {
			return fail(e)
		}
		if err = rollbackWrite(t, rbJRecord, jf, record, offset); err != nil {
			return fail(err)
		}
		offset += int64(len(record))
	}
	if err = rollbackFileSync(t, rbJBodySync, jf); err != nil {
		return fail(err)
	}
	// Publish only the first 12 bytes after the body is durable. All remaining
	// header fields were included in the first journal Sync.
	if err = rollbackWrite(t, rbJValidHeader, jf, valid[:12], 0); err != nil {
		return fail(err)
	}
	if err = rollbackFileSync(t, rbJHeaderSync, jf); err != nil {
		return fail(err)
	}
	if err = rollbackDirectorySync(t, rbPublishDirSync, t.path); err != nil {
		return fail(err)
	}
	for _, n := range dirty {
		data := t.dirty[n]
		if n == 1 {
			data = p1
		}
		databaseTouched = true
		if err = rollbackWrite(t, rbDBPage, t.file, data, int64(n-1)*int64(t.header.PageSize)); err != nil {
			return fail(err)
		}
	}
	// Extent only grows here. Freelist pages are retained; no unrelated physical
	// trailing pages are ever truncated.
	if err = rollbackFileSync(t, rbDBSync, t.file); err != nil {
		return fail(err)
	}
	if cleanup != nil {
		err = cleanup.closeTemporary(nil)
	} else {
		err = jf.Close()
	}
	if err != nil {
		if cleanup != nil && errors.Is(err, ErrPlaintextCleanupPending) {
			return t.deferPlainRetirementLocked(true)
		}
		return t.poison(err, true)
	}
	if err = rollbackUnlink(t, rbJUnlink, jpath); err != nil {
		return t.poison(err, true)
	}
	if err = rollbackDirectorySync(t, rbFinalDirSync, t.path); err != nil {
		return t.poison(errors.Join(ErrCommitUncertain, err), false)
	}
	t.header = finalHeader
	if t.cleanup != nil {
		return t.retirePlainLocked(nil, errors.Join(ErrPoisoned, ErrCommitUncertain), true)
	}
	if !t.keepFile {
		err = closeNativeFile(t.file)
	}
	t.file = nil
	t.dirty = nil
	t.free = nil
	if err != nil {
		t.poisoned = errors.Join(ErrPoisoned, ErrCommitUncertain, err)
		return t.poisoned
	}
	return nil
}
func (t *Tx) poison(cause error, recovery bool) error {
	if t.borrowedCleanup != nil {
		if recovery {
			cause = errors.Join(ErrRecoveryRequired, cause)
		}
		t.file, t.dirty, t.free = nil, nil, nil
		t.clearClosedUndoLocked()
		t.poisoned = t.borrowedCleanup.observe(errors.Join(ErrPoisoned, cause))
		return t.poisoned
	}
	if t.cleanup != nil {
		if recovery {
			cause = errors.Join(ErrRecoveryRequired, cause)
		}
		return t.retirePlainLocked(errors.Join(ErrPoisoned, cause), nil, true)
	}
	if t.encrypted != nil {
		s := t.encrypted
		s.mu.Lock()
		s.poisoned = errors.Join(ErrPoisoned, cause)
		t.poisoned = s.poisoned
		s.active = nil
		s.mu.Unlock()
		t.clearEncrypted()
		return t.poisoned
	}
	if t.session != nil {
		t.session.mu.Lock()
		t.poisoned = t.session.poisonLocked(cause)
		t.session.active = nil
		t.session.mu.Unlock()
		t.file, t.dirty, t.free = nil, nil, nil
		return t.poisoned
	}
	if t.file != nil {
		cause = errors.Join(cause, closeNativeFile(t.file))
		t.file = nil
	}
	if recovery {
		cause = errors.Join(ErrRecoveryRequired, cause)
	}
	t.poisoned = errors.Join(ErrPoisoned, cause)
	t.dirty = nil
	t.free = nil
	return t.poisoned
}
`

var rollbackFaultOwnedShapes = map[string]string{
	rollbackFaultPoints: `package pager

// rollbackIOPoint names source sites, not a process-wide I/O ordinal.
// Directory points cover the complete existing open/Sync/Close helper.
type rollbackIOPoint string

const (
	rbJCreate        rollbackIOPoint = "RB.JCREATE"
	rbJInvalid       rollbackIOPoint = "RB.JINVALID"
	rbDBOriginalRead rollbackIOPoint = "RB.DB_ORIGINAL_READ"
	rbJRecord        rollbackIOPoint = "RB.JRECORD"
	rbJBodySync      rollbackIOPoint = "RB.JBODY_SYNC"
	rbJValidHeader   rollbackIOPoint = "RB.JVALID_HEADER"
	rbJHeaderSync    rollbackIOPoint = "RB.JHEADER_SYNC"
	rbPublishDirSync rollbackIOPoint = "RB.PUBLISH_DIRSYNC"
	rbDBPage         rollbackIOPoint = "RB.DB_PAGE"
	rbDBSync         rollbackIOPoint = "RB.DB_SYNC"
	rbJUnlink        rollbackIOPoint = "RB.JUNLINK"
	rbFinalDirSync   rollbackIOPoint = "RB.FINAL_DIRSYNC"
	rbCleanupUnlink  rollbackIOPoint = "RB.CLEANUP_UNLINK"
	rbCleanupDirSync rollbackIOPoint = "RB.CLEANUP_DIRSYNC"
)
`,
	rollbackFaultDefault: `//go:build !linux || !manual_fault_rollback

package pager

import (
	"os"

	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

// The default complement has no armer, registry, environment or observer.
// Each bridge preserves the concrete original managed owner and original call.
func rollbackCreate(t *Tx, point rollbackIOPoint, path string, flags int) (*vfs.File, error) {
	return openJournal(path, flags)
}
func rollbackRead(t *Tx, point rollbackIOPoint, f *vfs.File, b []byte, off int64) (int, error) {
	return f.ReadAt(b, off)
}
func rollbackWrite(t *Tx, point rollbackIOPoint, f *vfs.File, b []byte, off int64) error {
	return writeFullAt(f, b, off)
}
func rollbackFileSync(t *Tx, point rollbackIOPoint, f *vfs.File) error {
	return f.Sync()
}
func rollbackDirectorySync(t *Tx, point rollbackIOPoint, path string) error {
	return syncDirectory(path)
}
func rollbackUnlink(t *Tx, point rollbackIOPoint, path string) error {
	return os.Remove(path)
}
`,
	rollbackFaultManual: `//go:build linux && manual_fault_rollback

package pager

import (
	"errors"
	"io"
	"os"
	"sync"

	"github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackfault"
	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

var errRollbackFaultGate = errors.New("rollback fault adapter requires separate authorization")
var errRollbackFaultScope = errors.New("rollback fault adapter scope not qualified")
var errRollbackFaultInjected = errors.New("rollback fault adapter injected I/O error")

// This is one lifetime slot, not an assignable VFS callback or a public option.
// Even a successfully disarmed scope cannot be reused for another trial.
var rollbackFaultRegistry struct {
	mu     sync.Mutex
	used   bool
	active *rollbackFaultScope
}

type rollbackFaultScope struct {
	mu               sync.Mutex
	tx               *Tx
	main             *vfs.File
	journal          *vfs.File
	cleanup          *plainRetirement
	plan             rollbackfault.Plan
	receipt          rollbackfault.Receipt
	counts           [14]uint8
	events           uint8
	diagnostics      [rollbackfault.MaxEvents]error
	problem          error
	firstError       bool
	inFlight         bool
	finalized        bool
	retired          bool
	journalAbsent    bool
	journalUnknown   bool
	targetError      error
	targetErrorToken uint64
}

type rollbackFaultTicket struct {
	scope  *rollbackFaultScope
	call   rollbackfault.Call
	action rollbackfault.Action
}

type rollbackFaultReport struct {
	Observation rollbackfault.Observation
	Events      [rollbackfault.MaxEvents]rollbackfault.Event
	EventCount  uint8
	Diagnostics [rollbackfault.MaxEvents]error
	CommitError error
	Problem     error
}

// No caller is supplied in this source slice. The future separately reviewed
// caller must gate before fixture or owner work, invoke Commit exactly once,
// and supply its real return to finishRollbackFault. Neither tags nor this
// environment value are execution authorization or physical validation.
func armRollbackFault(t *Tx, plan rollbackfault.Plan) (*rollbackFaultScope, error) {
	if os.Getenv("GOSQLITE_MANUAL_FAULT_ROLLBACK") != "independently_authorized" {
		return nil, errRollbackFaultGate
	}
	if t == nil {
		return nil, errRollbackFaultScope
	}
	spec := plan.Spec()
	receipt, problem := rollbackfault.NewReceipt(plan)
	if problem != rollbackfault.NoProblem {
		return nil, errRollbackFaultScope
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.session != nil || t.shared != nil || t.encrypted != nil || t.keepFile || t.borrowedCleanup != nil || t.readOnly || t.statement || t.poisoned != nil || t.file == nil || t.cleanup == nil || t.path == "" || t.info == nil {
		return nil, errRollbackFaultScope
	}
	if t.header.PageSize != 4096 || t.opts.SectorSize != 4096 || t.header.ReadVersion != 1 || t.header.WriteVersion != 1 || t.originalPages == 0 || t.originalPages > 4 || t.header.PageCount == 0 || t.header.PageCount > 4 || len(t.dirty) == 0 || len(t.dirty) > 4 || len(t.dirty[1]) != 4096 {
		return nil, errRollbackFaultScope
	}
	for page, data := range t.dirty {
		if page == 0 || page > t.header.PageCount || len(data) != 4096 {
			return nil, errRollbackFaultScope
		}
	}
	dirty := sortedPages(t.dirty)
	originals := t.journalPages(dirty)
	if len(originals) == 0 || len(originals) > 4 || int(spec.OriginalPages) != len(originals) || int(spec.DirtyPages) != len(dirty) {
		return nil, errRollbackFaultScope
	}
	// Bind the frozen manifest to this Tx's exact page order, not a similar path.
	for i := 0; i < int(spec.CallCount); i++ {
		call := spec.Calls[i]
		if call.Point == rollbackfault.DBOriginalRead {
			index := int(call.Occurrence) - 1
			if index < 0 || index >= len(originals) || call.Offset != int64(originals[index]-1)*4096 {
				return nil, errRollbackFaultScope
			}
		}
		if call.Point == rollbackfault.DBPage {
			index := int(call.Occurrence) - 1
			if index < 0 || index >= len(dirty) || call.Offset != int64(dirty[index]-1)*4096 {
				return nil, errRollbackFaultScope
			}
		}
	}
	g := t.cleanup
	g.mu.Lock()
	main, concrete := g.main.(plainFile)
	healthy := concrete && main.file == t.file && !g.started && !g.done && g.cause == nil && g.pending.Load() == nil && g.temporary == nil && !g.temporaryReserved && !g.temporaryRequested && g.wal == nil && !g.walReserved && !g.walRequested && len(g.nested) == 0
	g.mu.Unlock()
	if !healthy || t.file.RetirementComplete() {
		return nil, errRollbackFaultScope
	}
	s := &rollbackFaultScope{tx: t, main: t.file, cleanup: g, plan: plan, receipt: receipt}
	s.append(rollbackfault.Event{Kind: rollbackfault.RunStarted})
	if s.problem != nil {
		return nil, s.problem
	}
	// Admission contains no owner queries and no native operation.
	rollbackFaultRegistry.mu.Lock()
	defer rollbackFaultRegistry.mu.Unlock()
	if rollbackFaultRegistry.used || rollbackFaultRegistry.active != nil {
		return nil, errRollbackFaultScope
	}
	rollbackFaultRegistry.used = true
	rollbackFaultRegistry.active = s
	return s, nil
}

func rollbackFaultFor(t *Tx) *rollbackFaultScope {
	rollbackFaultRegistry.mu.Lock()
	s := rollbackFaultRegistry.active
	rollbackFaultRegistry.mu.Unlock()
	if s == nil || s.tx != t {
		return nil
	}
	return s
}

func (s *rollbackFaultScope) append(event rollbackfault.Event) {
	if s.events >= rollbackfault.MaxEvents {
		s.problem = errRollbackFaultScope
		return
	}
	event.Identity = s.plan.Spec().Identity
	event.Ordinal = s.events + 1
	s.events++
	if s.receipt.Append(event) != rollbackfault.NoProblem {
		s.problem = errRollbackFaultScope
	}
}

func rollbackFaultPoint(point rollbackIOPoint) (rollbackfault.Point, int) {
	switch point {
	case rbJCreate:
		return rollbackfault.JournalCreate, 0
	case rbJInvalid:
		return rollbackfault.JournalInvalid, 1
	case rbDBOriginalRead:
		return rollbackfault.DBOriginalRead, 2
	case rbJRecord:
		return rollbackfault.JournalRecord, 3
	case rbJBodySync:
		return rollbackfault.JournalBodySync, 4
	case rbJValidHeader:
		return rollbackfault.JournalValidHeader, 5
	case rbJHeaderSync:
		return rollbackfault.JournalHeaderSync, 6
	case rbPublishDirSync:
		return rollbackfault.PublishDirectorySync, 7
	case rbDBPage:
		return rollbackfault.DBPage, 8
	case rbDBSync:
		return rollbackfault.DBSync, 9
	case rbJUnlink:
		return rollbackfault.JournalUnlink, 10
	case rbFinalDirSync:
		return rollbackfault.FinalDirectorySync, 11
	case rbCleanupUnlink:
		return rollbackfault.CleanupUnlink, 12
	case rbCleanupDirSync:
		return rollbackfault.CleanupDirectorySync, 13
	default:
		return 0, -1
	}
}

// Called under the original Tx mutex. Its private mutex is released before
// every original operation; registry admission is never held during I/O.
func rollbackFaultBefore(t *Tx, point rollbackIOPoint, f *vfs.File, path string, off int64, length int) rollbackFaultTicket {
	s := rollbackFaultFor(t)
	if s == nil {
		return rollbackFaultTicket{action: rollbackfault.Pass}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	p, index := rollbackFaultPoint(point)
	if index < 0 || s.finalized || s.inFlight || s.counts[index] >= 4 {
		s.problem = errRollbackFaultScope
		return rollbackFaultTicket{action: rollbackfault.Pass}
	}
	s.counts[index]++
	spec := s.plan.Spec()
	call := rollbackfault.Call{Point: p, Occurrence: s.counts[index], Offset: off, Length: length}
	valid := t.file == s.main && t.cleanup == s.cleanup && t.session == nil && t.shared == nil && t.encrypted == nil && !t.keepFile && t.borrowedCleanup == nil
	switch point {
	case rbDBOriginalRead, rbDBPage, rbDBSync:
		call.Role, call.Owner = rollbackfault.Database, spec.MainOwner
		valid = valid && f == s.main && path == ""
	case rbPublishDirSync, rbFinalDirSync, rbCleanupDirSync:
		call.Role, call.Owner = rollbackfault.Directory, spec.DirectoryOwner
		valid = valid && f == nil && path == t.path
	case rbJCreate, rbJUnlink, rbCleanupUnlink:
		call.Role, call.Owner = rollbackfault.Journal, spec.JournalOwner
		valid = valid && f == nil && path == t.path+"-journal"
	default:
		call.Role, call.Owner = rollbackfault.Journal, spec.JournalOwner
		valid = valid && f != nil && f == s.journal && path == ""
	}
	if !valid {
		s.problem = errRollbackFaultScope
	}
	action := rollbackfault.Pass
	if valid && !s.firstError && s.problem == nil && call == spec.Target && point != rbCleanupUnlink && point != rbCleanupDirSync {
		action = spec.Action
	}
	s.inFlight = true
	s.append(rollbackfault.Event{Kind: rollbackfault.Intent, Call: call, Action: action})
	if s.problem != nil {
		action = rollbackfault.Pass
	}
	return rollbackFaultTicket{scope: s, call: call, action: action}
}

func (s *rollbackFaultScope) diagnostic(err error) uint64 {
	if err == nil {
		return 0
	}
	if s.events >= rollbackfault.MaxEvents {
		s.problem = errRollbackFaultScope
		return 0
	}
	// The original error remains strongly retained, never serialized as success.
	s.diagnostics[s.events] = err
	return uint64(s.events) + 1
}

func rollbackFaultAfter(ticket rollbackFaultTicket, executed bool, nativeN int, nativeErr error, returnedN int, returnedErr error, normalizedErr error, effect rollbackfault.SideEffect) {
	s := ticket.scope
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	result := rollbackfault.Result{Executed: executed, NativeN: nativeN, ReturnedN: returnedN, Effect: effect}
	if nativeErr != nil {
		result.NativeError = rollbackfault.Underlying
		result.NativeErrorToken = s.diagnostic(nativeErr)
	}
	if returnedErr != nil {
		switch {
		case nativeErr != nil && returnedErr == nativeErr:
			result.ReturnedError, result.ReturnedErrorToken = rollbackfault.Underlying, result.NativeErrorToken
		case returnedErr == errRollbackFaultInjected:
			result.ReturnedError = rollbackfault.Injected
		default:
			s.problem = errRollbackFaultScope
		}
	}
	if normalizedErr != nil {
		switch {
		case nativeErr != nil && normalizedErr == nativeErr:
			result.NormalizedError, result.NormalizedErrorToken = rollbackfault.Underlying, result.NativeErrorToken
		case normalizedErr == errRollbackFaultInjected:
			result.NormalizedError = rollbackfault.Injected
		case normalizedErr == io.ErrShortWrite:
			result.NormalizedError = rollbackfault.ShortWrite
		default:
			s.problem = errRollbackFaultScope
		}
	}
	if ticket.call == s.plan.Spec().Target {
		s.targetError = normalizedErr
		s.targetErrorToken = result.NormalizedErrorToken
	}
	if normalizedErr != nil || returnedErr != nil || nativeErr != nil {
		s.firstError = true
	}
	s.inFlight = false
	s.append(rollbackfault.Event{Kind: rollbackfault.Returned, Call: ticket.call, Action: ticket.action, Result: result})
}

func rollbackCreate(t *Tx, point rollbackIOPoint, path string, flags int) (*vfs.File, error) {
	ticket := rollbackFaultBefore(t, point, nil, path, 0, 0)
	if flags != os.O_WRONLY|os.O_CREATE|os.O_EXCL && ticket.scope != nil {
		ticket.scope.mu.Lock()
		ticket.scope.problem = errRollbackFaultScope
		ticket.scope.mu.Unlock()
		ticket.action = rollbackfault.Pass
	}
	if ticket.action == rollbackfault.FailBefore {
		ticket.scope.mu.Lock()
		ticket.scope.journalAbsent = true
		ticket.scope.mu.Unlock()
		rollbackFaultAfter(ticket, false, 0, nil, 0, errRollbackFaultInjected, errRollbackFaultInjected, rollbackfault.EffectNone)
		return nil, errRollbackFaultInjected
	}
	f, err := openJournal(path, flags)
	if ticket.scope != nil {
		ticket.scope.mu.Lock()
		ticket.scope.journal = f
		ticket.scope.journalUnknown = err != nil || f == nil
		ticket.scope.mu.Unlock()
	}
	rollbackFaultAfter(ticket, true, 0, err, 0, err, err, rollbackfault.EffectUnknown)
	return f, err
}

func rollbackRead(t *Tx, point rollbackIOPoint, f *vfs.File, b []byte, off int64) (int, error) {
	ticket := rollbackFaultBefore(t, point, f, "", off, len(b))
	if ticket.action == rollbackfault.FailBefore {
		rollbackFaultAfter(ticket, false, 0, nil, 0, errRollbackFaultInjected, errRollbackFaultInjected, rollbackfault.EffectNone)
		return 0, errRollbackFaultInjected
	}
	n, err := f.ReadAt(b, off)
	rollbackFaultAfter(ticket, true, n, err, n, err, err, rollbackfault.EffectNone)
	return n, err
}

type rollbackFaultWriter struct {
	file        *vfs.File
	action      rollbackfault.Action
	prefix      int
	prefixError bool
	executed    bool
	nativeN     int
	nativeErr   error
	returnedN   int
	returnedErr error
}

// Only the original managed owner is invoked, once. No raw descriptor, bypass,
// arbitrary callback, retry, extra Sync or hidden second injected failure.
func (w *rollbackFaultWriter) WriteAt(b []byte, off int64) (int, error) {
	if w.action == rollbackfault.FailBefore {
		w.returnedErr = errRollbackFaultInjected
		return 0, w.returnedErr
	}
	write := b
	if w.action == rollbackfault.PrefixShortWrite {
		write = b[:w.prefix]
	}
	w.executed = true
	w.nativeN, w.nativeErr = w.file.WriteAt(write, off)
	w.returnedN, w.returnedErr = w.nativeN, w.nativeErr
	if w.action == rollbackfault.PrefixShortWrite && w.nativeErr == nil && w.nativeN == w.prefix && w.prefixError {
		w.returnedErr = errRollbackFaultInjected
	}
	return w.returnedN, w.returnedErr
}

func rollbackWrite(t *Tx, point rollbackIOPoint, f *vfs.File, b []byte, off int64) error {
	ticket := rollbackFaultBefore(t, point, f, "", off, len(b))
	if ticket.scope == nil {
		return writeFullAt(f, b, off)
	}
	spec := ticket.scope.plan.Spec()
	w := &rollbackFaultWriter{file: f, action: ticket.action, prefix: spec.Prefix, prefixError: spec.PrefixError}
	err := writeFullAt(w, b, off)
	effect := rollbackfault.EffectPossible
	if !w.executed {
		effect = rollbackfault.EffectNone
	} else if w.nativeErr != nil || w.nativeN < 0 || w.nativeN > len(b) {
		effect = rollbackfault.EffectUnknown
	} else if w.action == rollbackfault.PrefixShortWrite && w.nativeN == w.prefix && w.prefix == 0 {
		effect = rollbackfault.EffectNone
	} else if w.action == rollbackfault.PrefixShortWrite && w.nativeN == w.prefix {
		effect = rollbackfault.EffectPrefix
	}
	rollbackFaultAfter(ticket, w.executed, w.nativeN, w.nativeErr, w.returnedN, w.returnedErr, err, effect)
	return err
}

func rollbackFileSync(t *Tx, point rollbackIOPoint, f *vfs.File) error {
	ticket := rollbackFaultBefore(t, point, f, "", 0, 0)
	if ticket.action == rollbackfault.FailBefore {
		rollbackFaultAfter(ticket, false, 0, nil, 0, errRollbackFaultInjected, errRollbackFaultInjected, rollbackfault.EffectNone)
		return errRollbackFaultInjected
	}
	err := f.Sync()
	rollbackFaultAfter(ticket, true, 0, err, 0, err, err, rollbackfault.EffectUnknown)
	return err
}

func rollbackDirectorySync(t *Tx, point rollbackIOPoint, path string) error {
	ticket := rollbackFaultBefore(t, point, nil, path, 0, 0)
	if ticket.action == rollbackfault.FailBefore {
		rollbackFaultAfter(ticket, false, 0, nil, 0, errRollbackFaultInjected, errRollbackFaultInjected, rollbackfault.EffectNone)
		return errRollbackFaultInjected
	}
	err := syncDirectory(path)
	rollbackFaultAfter(ticket, true, 0, err, 0, err, err, rollbackfault.EffectUnknown)
	return err
}

func rollbackUnlink(t *Tx, point rollbackIOPoint, path string) error {
	ticket := rollbackFaultBefore(t, point, nil, path, 0, 0)
	// Cleanup is observation-only even after an underlying, unplanned error.
	if ticket.action == rollbackfault.FailBefore {
		rollbackFaultAfter(ticket, false, 0, nil, 0, errRollbackFaultInjected, errRollbackFaultInjected, rollbackfault.EffectNone)
		return errRollbackFaultInjected
	}
	err := os.Remove(path)
	rollbackFaultAfter(ticket, true, 0, err, 0, err, err, rollbackfault.EffectUnknown)
	return err
}

// This accepts a caller-supplied Commit return; it cannot prove that a caller
// executed Commit. There is intentionally no native caller or physical verifier
// here. It only snapshots the bound owners and seals an adapter-only receipt.
func finishRollbackFault(s *rollbackFaultScope, commitErr error) rollbackFaultReport {
	if s == nil {
		return rollbackFaultReport{Problem: errRollbackFaultScope}
	}
	s.tx.mu.Lock()
	defer s.tx.mu.Unlock()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.finalized || s.inFlight {
		return rollbackFaultReport{Problem: errRollbackFaultScope}
	}
	s.finalized = true
	commit := rollbackfault.CommitResult{Class: rollbackfault.CommitFailed, Logical: rollbackfault.LogicalOpen}
	if s.tx.file == nil && s.tx.encrypted == nil {
		commit.Logical = rollbackfault.LogicalClosed
	}
	switch {
	case commitErr == nil:
		commit.Class = rollbackfault.CommitSucceeded
	case errors.Is(commitErr, ErrPlaintextCleanupPending):
		commit.Class = rollbackfault.CommitCleanupPending
	case errors.Is(commitErr, ErrCommitUncertain):
		commit.Class = rollbackfault.CommitUncertain
	case errors.Is(commitErr, ErrRecoveryRequired):
		commit.Class = rollbackfault.CommitRecoveryRequired
	case errors.Is(commitErr, ErrUnsupported):
		commit.Class = rollbackfault.CommitUnsupported
	}
	if commitErr != nil {
		commit.Error = rollbackfault.Underlying
		commit.ErrorToken = s.diagnostic(commitErr)
	}
	if s.targetError != nil && errors.Is(commitErr, s.targetError) {
		switch {
		case s.targetErrorToken != 0:
			commit.TargetError, commit.TargetErrorToken = rollbackfault.Underlying, s.targetErrorToken
		case s.targetError == errRollbackFaultInjected:
			commit.TargetError = rollbackfault.Injected
		case s.targetError == io.ErrShortWrite:
			commit.TargetError = rollbackfault.ShortWrite
		}
	}
	s.append(rollbackfault.Event{Kind: rollbackfault.CommitReturned, Commit: commit})
	spec := s.plan.Spec()
	owners := rollbackfault.OwnersResult{MainOwner: spec.MainOwner, JournalOwner: spec.JournalOwner, CleanupOwner: spec.CleanupOwner, Main: rollbackfault.OwnerLive, Journal: rollbackfault.OwnerUnknown, Cleanup: rollbackfault.OwnerPending}
	if s.main.RetirementComplete() {
		owners.Main = rollbackfault.OwnerRetired
	}
	if s.journal != nil && s.journal.RetirementComplete() {
		owners.Journal = rollbackfault.OwnerRetired
	} else if s.journalAbsent {
		owners.Journal = rollbackfault.OwnerNotAcquired
	} else if s.journal != nil && !s.journalUnknown {
		owners.Journal = rollbackfault.OwnerPending
	}
	g := s.cleanup
	g.mu.Lock()
	if g.done && g.main == nil && g.temporary == nil && g.wal == nil && len(g.nested) == 0 && g.pending.Load() == nil {
		owners.Cleanup = rollbackfault.OwnerRetired
	}
	for _, nested := range g.nested {
		if _, unknown := nested.(plainTerminal); unknown {
			owners.Cleanup = rollbackfault.OwnerUnknown
		}
	}
	g.mu.Unlock()
	s.retired = owners.Main == rollbackfault.OwnerRetired && (owners.Journal == rollbackfault.OwnerRetired || owners.Journal == rollbackfault.OwnerNotAcquired) && owners.Cleanup == rollbackfault.OwnerRetired
	s.append(rollbackfault.Event{Kind: rollbackfault.OwnersObserved, Owners: owners})
	s.append(rollbackfault.Event{Kind: rollbackfault.ValidationObserved, Validation: rollbackfault.ValidationAbsent})
	s.append(rollbackfault.Event{Kind: rollbackfault.Terminal})
	observation := s.receipt.Seal()
	if s.problem != nil {
		observation.Outcome = rollbackfault.ObservationInvalid
	}
	events, count := s.receipt.Events()
	return rollbackFaultReport{Observation: observation, Events: events, EventCount: count, Diagnostics: s.diagnostics, CommitError: commitErr, Problem: s.problem}
}

func disarmRollbackFault(s *rollbackFaultScope) error {
	if s == nil {
		return errRollbackFaultScope
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.finalized || !s.retired || s.problem != nil || s.receipt.Seal().Outcome != rollbackfault.AdapterObserved {
		return errRollbackFaultScope
	}
	rollbackFaultRegistry.mu.Lock()
	defer rollbackFaultRegistry.mu.Unlock()
	if rollbackFaultRegistry.active != s {
		return errRollbackFaultScope
	}
	rollbackFaultRegistry.active = nil
	// used remains true. Pending/unknown owners never reach this assignment.
	return nil
}
`,
	rollbackFaultWriterTest: `package pager

import (
	"errors"
	"io"
	"testing"
)

// This fake has no descriptors, native owner, filesystem or callbacks.
type rollbackMemoryWriter struct {
	n     int
	err   error
	calls int
	got   []byte
	off   int64
}

func (w *rollbackMemoryWriter) WriteAt(b []byte, off int64) (int, error) {
	w.calls++
	w.got = append([]byte(nil), b...)
	w.off = off
	return w.n, w.err
}

func TestRollbackWriteFullAtMemoryContract(t *testing.T) {
	original := errors.New("memory-only original error")
	for _, tc := range []struct {
		name string
		n    int
		err  error
		want error
	}{
		{"complete", 4, nil, nil},
		{"zero-short", 0, nil, io.ErrShortWrite},
		{"prefix-short", 2, nil, io.ErrShortWrite},
		{"long-invalid-count", 5, nil, io.ErrShortWrite},
		{"short-error-priority", 2, original, original},
		{"full-error-priority", 4, original, original},
	} {
		t.Run(tc.name, func(t *testing.T) {
			w := &rollbackMemoryWriter{n: tc.n, err: tc.err}
			if got := writeFullAt(w, []byte("body"), 37); got != tc.want {
				t.Fatalf("got %v, want exact error %v", got, tc.want)
			}
			if w.calls != 1 || string(w.got) != "body" || w.off != 37 {
				t.Fatalf("write retried or arguments changed: %+v", w)
			}
		})
	}
}
`,
	rollbackFaultContract: `// Package rollbackfault models a finite, descriptor-free plain rollback adapter
// receipt. It performs no I/O and cannot establish physical durability, recovery,
// WAL correctness, encryption correctness, or a physical acceptance PASS.
package rollbackfault

const (
	MaxEvents   = 128
	MaxCalls    = 32
	MaxIDLength = 128
)

type Profile uint8

const (
	ProfileUnknown Profile = iota
	PlainRollback4096
)

// Identity binds every event to one frozen artifact, trial, and transaction scope.
// IDs are labels supplied by a future caller, not hashes verified by this model.
type Identity struct {
	Suite    string
	Source   string
	Tool     string
	Binary   string
	Manifest string
	Trial    string
	Scope    string
	Profile  Profile
}

type Point uint8

const (
	PointUnknown Point = iota
	JournalCreate
	JournalInvalid
	DBOriginalRead
	JournalRecord
	JournalBodySync
	JournalValidHeader
	JournalHeaderSync
	PublishDirectorySync
	DBPage
	DBSync
	JournalUnlink
	FinalDirectorySync
	CleanupUnlink
	CleanupDirectorySync
)

type Role uint8

const (
	RoleUnknown Role = iota
	Database
	Journal
	Directory
)

type Action uint8

const (
	Pass Action = iota
	FailBefore
	PrefixShortWrite
)

type Call struct {
	Point      Point
	Occurrence uint8
	Role       Role
	Offset     int64
	Length     int
	Owner      uint64
}

type CommitClass uint8

const (
	CommitUnknown CommitClass = iota
	CommitSucceeded
	CommitFailed
	CommitRecoveryRequired
	CommitUncertain
	CommitCleanupPending
	CommitUnsupported
	CommitRefused
	CommitSkipped
)

// PlanSpec is copied at admission. OriginalPages counts journaled original page
// images, not the database's original extent. The concrete armer must separately
// check that extent and all actual page offsets against the supplied manifest.
// DirectoryOwner names the composite directory helper scope, not a descriptor.
type PlanSpec struct {
	Identity       Identity
	Target         Call
	Action         Action
	Prefix         int
	PrefixError    bool
	MainOwner      uint64
	JournalOwner   uint64
	DirectoryOwner uint64
	CleanupOwner   uint64
	OriginalPages  uint8
	DirtyPages     uint8
	PageSize       uint32
	SectorSize     uint32
	Calls          [MaxCalls]Call
	CallCount      uint8
	ExpectedCommit CommitClass
}

type Plan struct {
	spec  PlanSpec
	valid bool
}

type Problem uint8

const (
	ProblemNone Problem = iota
	ProblemIdentity
	ProblemProfile
	ProblemOwners
	ProblemGeometry
	ProblemAction
	ProblemTarget
	ProblemCallPath
	ProblemCommitExpectation
	ProblemUnadmittedPlan
	ProblemSealed
	ProblemEventLimit
	ProblemOrdinal
	ProblemEventOrder
	ProblemEventPayload
	ProblemCallMismatch
	ProblemErrorEvidence
	ProblemNativeResult
	ProblemErrorMasked
	ProblemUnexpectedCommit
	ProblemLogicalState
	ProblemOwnerObservation
	ProblemPhysicalClaim
	ProblemIncomplete
	ProblemUnderlyingFailure
	ProblemOwnersUnresolved
	ProblemUnqualifiedCommit
)

const NoProblem = ProblemNone

func NewPlan(spec PlanSpec) (Plan, Problem) {
	if !validIdentity(spec.Identity) {
		return Plan{}, ProblemIdentity
	}
	if spec.Identity.Profile != PlainRollback4096 {
		return Plan{}, ProblemProfile
	}
	owners := [4]uint64{spec.MainOwner, spec.JournalOwner, spec.DirectoryOwner, spec.CleanupOwner}
	for i, owner := range owners {
		if owner == 0 {
			return Plan{}, ProblemOwners
		}
		for j := 0; j < i; j++ {
			if owners[j] == owner {
				return Plan{}, ProblemOwners
			}
		}
	}
	if spec.PageSize != 4096 || spec.SectorSize != 4096 || spec.OriginalPages < 1 || spec.OriginalPages > 4 || spec.DirtyPages < 1 || spec.DirtyPages > 4 || spec.OriginalPages > spec.DirtyPages {
		return Plan{}, ProblemGeometry
	}
	if spec.Action > PrefixShortWrite || (spec.Action != PrefixShortWrite && (spec.Prefix != 0 || spec.PrefixError)) {
		return Plan{}, ProblemAction
	}
	if spec.Target.Point < JournalCreate || spec.Target.Point > FinalDirectorySync || spec.Target.Occurrence == 0 {
		return Plan{}, ProblemTarget
	}
	if spec.Action == PrefixShortWrite && (!writePoint(spec.Target.Point) || spec.Prefix < 0 || spec.Prefix >= spec.Target.Length) {
		return Plan{}, ProblemAction
	}
	if spec.ExpectedCommit != expectedCommit(spec) {
		return Plan{}, ProblemCommitExpectation
	}
	if problem := validCalls(spec); problem != ProblemNone {
		return Plan{}, problem
	}
	// A short caller substring can share a much larger backing allocation.
	// Freeze only these already validated bytes, never the caller's storage.
	spec.Identity = copyIdentity(spec.Identity)
	return Plan{spec: spec, valid: true}, ProblemNone
}

func copyIdentity(id Identity) Identity {
	id.Suite = copyIdentityID(id.Suite)
	id.Source = copyIdentityID(id.Source)
	id.Tool = copyIdentityID(id.Tool)
	id.Binary = copyIdentityID(id.Binary)
	id.Manifest = copyIdentityID(id.Manifest)
	id.Trial = copyIdentityID(id.Trial)
	id.Scope = copyIdentityID(id.Scope)
	return id
}

// Only called after identity admission. The fixed buffer bounds even a possible
// retained conversion backing array; it cannot alias the input's large storage.
func copyIdentityID(value string) string {
	var bounded [MaxIDLength]byte
	n := copy(bounded[:], value)
	return string(bounded[:n])
}

// Spec returns a detached value, including the fixed-size call inventory.
func (p Plan) Spec() PlanSpec { return p.spec }

func validIdentity(id Identity) bool {
	ids := [7]string{id.Suite, id.Source, id.Tool, id.Binary, id.Manifest, id.Trial, id.Scope}
	for _, value := range ids {
		if len(value) == 0 || value == "0" || len(value) > MaxIDLength {
			return false
		}
		for _, ch := range value {
			if ch < '!' || ch > '~' {
				return false
			}
		}
	}
	return true
}

func writePoint(point Point) bool {
	return point == JournalInvalid || point == JournalRecord || point == JournalValidHeader || point == DBPage
}

func expectedCommit(spec PlanSpec) CommitClass {
	if spec.Action == Pass {
		return CommitSucceeded
	}
	switch spec.Target.Point {
	case DBPage, DBSync, JournalUnlink:
		return CommitRecoveryRequired
	case FinalDirectorySync:
		return CommitUncertain
	default:
		return CommitFailed
	}
}

// The complete expected trace is structural, not an arbitrary caller checklist.
// Native errors outside the selected action may depart from it; that is evidence
// of an unfulfilled plan, never grounds to silently shorten the checklist.
func validCalls(spec PlanSpec) Problem {
	if spec.CallCount == 0 || int(spec.CallCount) > MaxCalls {
		return ProblemCallPath
	}
	next := JournalCreate
	var occurrences [15]uint8
	var hits uint8
	lastRead, lastWrite := int64(-1), int64(-1)
	for i := 0; i < int(spec.CallCount); i++ {
		call := spec.Calls[i]
		if next == PointUnknown || call.Point != next {
			return ProblemCallPath
		}
		occurrences[call.Point]++
		if call.Occurrence != occurrences[call.Point] || !validCall(spec, call) {
			return ProblemCallPath
		}
		if call.Point == DBOriginalRead {
			if (call.Occurrence == 1 && call.Offset != 0) || call.Offset <= lastRead {
				return ProblemCallPath
			}
			lastRead = call.Offset
		}
		if call.Point == DBPage {
			if (call.Occurrence == 1 && call.Offset != 0) || call.Offset <= lastWrite {
				return ProblemCallPath
			}
			lastWrite = call.Offset
		}
		selected := call == spec.Target
		if selected {
			hits++
		}
		if selected && spec.Action != Pass {
			next = PointUnknown
			if call.Point > JournalCreate && call.Point < DBPage {
				next = CleanupUnlink
			}
			continue
		}
		switch call.Point {
		case JournalCreate:
			next = JournalInvalid
		case JournalInvalid:
			next = DBOriginalRead
		case DBOriginalRead:
			next = JournalRecord
		case JournalRecord:
			next = JournalBodySync
			if call.Occurrence < spec.OriginalPages {
				next = DBOriginalRead
			}
		case JournalBodySync:
			next = JournalValidHeader
		case JournalValidHeader:
			next = JournalHeaderSync
		case JournalHeaderSync:
			next = PublishDirectorySync
		case PublishDirectorySync:
			next = DBPage
		case DBPage:
			next = DBSync
			if call.Occurrence < spec.DirtyPages {
				next = DBPage
			}
		case DBSync:
			next = JournalUnlink
		case JournalUnlink:
			next = FinalDirectorySync
		case FinalDirectorySync, CleanupDirectorySync:
			next = PointUnknown
		case CleanupUnlink:
			next = CleanupDirectorySync
		default:
			return ProblemCallPath
		}
	}
	if next != PointUnknown || hits != 1 {
		return ProblemTarget
	}
	for i := int(spec.CallCount); i < MaxCalls; i++ {
		if spec.Calls[i] != (Call{}) {
			return ProblemCallPath
		}
	}
	return ProblemNone
}

func validCall(spec PlanSpec, call Call) bool {
	role, owner := Journal, spec.JournalOwner
	length, offset := 0, int64(0)
	switch call.Point {
	case JournalCreate, JournalBodySync, JournalHeaderSync, JournalUnlink, CleanupUnlink:
	case JournalInvalid:
		length = 4096
	case JournalRecord:
		length, offset = 4104, 4096+int64(call.Occurrence-1)*4104
	case JournalValidHeader:
		length = 12
	case DBOriginalRead, DBPage:
		role, owner = Database, spec.MainOwner
		length, offset = 4096, call.Offset
		if offset < 0 || offset%4096 != 0 || offset > 3*4096 {
			return false
		}
	case DBSync:
		role, owner = Database, spec.MainOwner
	case PublishDirectorySync, FinalDirectorySync, CleanupDirectorySync:
		role, owner = Directory, spec.DirectoryOwner
	default:
		return false
	}
	return call.Role == role && call.Owner == owner && call.Length == length && call.Offset == offset
}

type ErrorKind uint8

const (
	None ErrorKind = iota
	Injected
	ShortWrite
	Underlying
)

type SideEffect uint8

const (
	EffectUnknown SideEffect = iota
	EffectNone
	EffectPrefix
	EffectPossible
)

// Result separates the actual managed-owner return, adapter return, and the
// writeFullAt result. Diagnostic tokens refer only to caller-owned evidence;
// this package never stores an error interface, function, or file handle.
type Result struct {
	Executed             bool
	NativeN              int
	NativeError          ErrorKind
	NativeErrorToken     uint64
	ReturnedN            int
	ReturnedError        ErrorKind
	ReturnedErrorToken   uint64
	NormalizedError      ErrorKind
	NormalizedErrorToken uint64
	Effect               SideEffect
}

type LogicalState uint8

const (
	LogicalUnknown LogicalState = iota
	LogicalOpen
	LogicalClosed
)

type CommitResult struct {
	Class      CommitClass
	Error      ErrorKind
	ErrorToken uint64
	// TargetError is independently derived from preservation of the original
	// target error in the returned Commit error; copying the plan is not evidence.
	TargetError      ErrorKind
	TargetErrorToken uint64
	Logical          LogicalState
}

type OwnerState uint8

const (
	OwnerUnknown OwnerState = iota
	OwnerLive
	OwnerPending
	OwnerRetired
	OwnerNotAcquired
)

type OwnersResult struct {
	MainOwner    uint64
	JournalOwner uint64
	CleanupOwner uint64
	Main         OwnerState
	Journal      OwnerState
	Cleanup      OwnerState
}

// This source slice has no native recovery evidence input at all. Ledger,
// ordinary/extension index checks and durable acknowledgement checks remain
// absent; journal disappearance cannot stand in for any of them.
type Validation uint8

const ValidationAbsent Validation = 0

type EventKind uint8

const (
	EventUnknown EventKind = iota
	RunStarted
	Intent
	Returned
	CommitReturned
	OwnersObserved
	ValidationObserved
	Terminal
	// This retention marker is never a legal protocol step. Append produces
	// it only on identity rejection, without retaining any untrusted string.
	EventIdentityRejected
)

type Event struct {
	Identity   Identity
	Ordinal    uint8
	Kind       EventKind
	Call       Call
	Action     Action
	Result     Result
	Commit     CommitResult
	Owners     OwnersResult
	Validation Validation
	// Rejection metadata is zero on every legal event. In a rejection marker,
	// Ordinal is the supplied ordinal and RejectedAt is the retained slot.
	Rejection  Problem
	RejectedAt uint8
}

type Outcome uint8

const (
	ObservationInvalid Outcome = iota
	ObservationIncomplete
	ObservationRefused
	AdapterObserved
)

type Observation struct {
	Outcome    Outcome
	Problem    Problem
	TargetHits uint8
	EventCount uint8
}

// Even a syntactically complete caller-supplied observation has no physical
// authority. A future independently reviewed evidence type must remain separate.
func (Observation) PhysicalValidated() bool { return false }
func (Observation) Pass() bool              { return false }

type Receipt struct {
	plan         Plan
	events       [MaxEvents]Event
	count        uint8
	callIndex    uint8
	targetHits   uint8
	next         EventKind
	problem      Problem
	refusal      Problem
	targetResult Result
	commit       CommitResult
	owners       OwnersResult
	sealed       bool
}

func NewReceipt(plan Plan) (Receipt, Problem) {
	if !plan.valid {
		return Receipt{}, ProblemUnadmittedPlan
	}
	return Receipt{plan: plan, next: RunStarted}, ProblemNone
}

// Events returns a detached bounded snapshot. Identity rejection retains only
// a distinct scalar marker, never the rejected identity or other input payload.
// Other rejected events contain only frozen identity and fixed-size values.
// Append errors are sticky, never reset by a retry.
func (r Receipt) Events() ([MaxEvents]Event, uint8) { return r.events, r.count }

func (r *Receipt) Append(event Event) Problem {
	if r.sealed {
		return ProblemSealed
	}
	if r.problem != ProblemNone {
		return r.problem
	}
	if !r.plan.valid {
		r.problem = ProblemUnadmittedPlan
		return r.problem
	}
	if int(r.count) >= MaxEvents {
		r.problem = ProblemEventLimit
		return r.problem
	}
	// Check all variable-size input before retention. Comparing after storage
	// would keep rejected unbounded strings alive even on a sticky failure.
	if !validIdentity(event.Identity) || event.Identity != r.plan.spec.Identity {
		r.problem = ProblemIdentity
		r.events[r.count] = Event{Ordinal: event.Ordinal, Kind: EventIdentityRejected, Rejection: ProblemIdentity, RejectedAt: r.count + 1}
		r.count++
		return r.problem
	}
	// Equal short strings can still borrow large backing storage. Keep only
	// this plan's bounded copied identity; every other event field is scalar.
	event.Identity = r.plan.spec.Identity
	r.events[r.count] = event
	r.count++
	r.problem = r.accept(event)
	return r.problem
}

func (r *Receipt) accept(event Event) Problem {
	if event.Identity != r.plan.spec.Identity {
		return ProblemIdentity
	}
	if event.Ordinal != r.count {
		return ProblemOrdinal
	}
	if event.Kind != r.next || r.next == EventUnknown {
		return ProblemEventOrder
	}
	if !canonicalEvent(event) {
		return ProblemEventPayload
	}
	spec := r.plan.spec
	switch event.Kind {
	case RunStarted:
		r.next = Intent
	case Intent, Returned:
		if r.callIndex >= spec.CallCount || event.Call != spec.Calls[r.callIndex] {
			return ProblemCallMismatch
		}
		action := Pass
		if event.Call == spec.Target {
			action = spec.Action
		}
		if event.Action != action {
			return ProblemAction
		}
		if event.Kind == Intent {
			if event.Call == spec.Target {
				r.targetHits++
			}
			r.next = Returned
			break
		}
		if problem := validResult(spec, event.Call, action, event.Result); problem != ProblemNone {
			return problem
		}
		if event.Result.NativeError != None || (action == Pass && event.Result.NormalizedError != None) {
			r.refusal = ProblemUnderlyingFailure
		}
		if event.Call == spec.Target {
			r.targetResult = event.Result
		}
		r.callIndex++
		r.next = Intent
		if r.callIndex == spec.CallCount {
			r.next = CommitReturned
		}
	case CommitReturned:
		if !validError(event.Commit.Error, event.Commit.ErrorToken) || !validError(event.Commit.TargetError, event.Commit.TargetErrorToken) || event.Commit.Class > CommitSkipped || event.Commit.Logical > LogicalClosed {
			return ProblemErrorEvidence
		}
		if event.Commit.Class == CommitUnknown || event.Commit.Class >= CommitCleanupPending {
			r.refusal = ProblemUnqualifiedCommit
		} else if event.Commit.Class != spec.ExpectedCommit {
			return ProblemUnexpectedCommit
		}
		if event.Commit.Class == CommitSucceeded {
			if event.Commit.Error != None || event.Commit.Logical != LogicalClosed {
				return ProblemUnexpectedCommit
			}
		} else if event.Commit.Error == None {
			return ProblemUnexpectedCommit
		}
		if event.Commit.TargetError != r.targetResult.NormalizedError || event.Commit.TargetErrorToken != r.targetResult.NormalizedErrorToken {
			return ProblemErrorMasked
		}
		if event.Commit.Logical == LogicalUnknown {
			r.refusal = ProblemLogicalState
		} else if spec.Target.Point != JournalCreate || spec.Action == Pass {
			if event.Commit.Logical != LogicalClosed {
				return ProblemLogicalState
			}
		}
		r.commit, r.next = event.Commit, OwnersObserved
	case OwnersObserved:
		owners := event.Owners
		if owners.MainOwner != spec.MainOwner || owners.JournalOwner != spec.JournalOwner || owners.CleanupOwner != spec.CleanupOwner || owners.Main > OwnerNotAcquired || owners.Journal > OwnerNotAcquired || owners.Cleanup > OwnerNotAcquired || owners.Main == OwnerNotAcquired || owners.Cleanup == OwnerNotAcquired {
			return ProblemOwnerObservation
		}
		unacquired := spec.Target.Point == JournalCreate && spec.Action == FailBefore
		if owners.Journal == OwnerNotAcquired && !unacquired {
			return ProblemOwnerObservation
		}
		if unacquired && owners.Journal == OwnerRetired {
			return ProblemOwnerObservation
		}
		r.owners, r.next = owners, ValidationObserved
	case ValidationObserved:
		if event.Validation != ValidationAbsent {
			return ProblemPhysicalClaim
		}
		r.next = Terminal
	case Terminal:
		if r.targetHits != 1 || r.callIndex != spec.CallCount {
			return ProblemIncomplete
		}
		r.next = EventUnknown
	}
	return ProblemNone
}

func canonicalEvent(event Event) bool {
	kind := event.Kind
	event.Identity, event.Ordinal, event.Kind = Identity{}, 0, EventUnknown
	switch kind {
	case Intent:
		event.Call, event.Action = Call{}, Pass
	case Returned:
		event.Call, event.Action, event.Result = Call{}, Pass, Result{}
	case CommitReturned:
		event.Commit = CommitResult{}
	case OwnersObserved:
		event.Owners = OwnersResult{}
	case ValidationObserved:
		event.Validation = ValidationAbsent
	}
	return event == (Event{})
}

func validError(kind ErrorKind, token uint64) bool {
	return kind <= Underlying && ((kind == Underlying && token != 0) || (kind != Underlying && token == 0))
}

func validResult(spec PlanSpec, call Call, action Action, result Result) Problem {
	if !validError(result.NativeError, result.NativeErrorToken) || !validError(result.ReturnedError, result.ReturnedErrorToken) || !validError(result.NormalizedError, result.NormalizedErrorToken) || result.Effect > EffectPossible {
		return ProblemErrorEvidence
	}
	if result.NativeN < 0 || result.ReturnedN < 0 || result.NativeN > call.Length || result.ReturnedN > call.Length {
		return ProblemNativeResult
	}
	if action == FailBefore {
		if result.Executed || result.NativeN != 0 || result.NativeError != None || result.ReturnedN != 0 || result.ReturnedError != Injected || result.NormalizedError != Injected || result.Effect != EffectNone {
			return ProblemNativeResult
		}
		return ProblemNone
	}
	if !result.Executed || result.NativeError == Injected {
		return ProblemNativeResult
	}
	if result.NativeError != None {
		expectedEffect := EffectUnknown
		if call.Point == DBOriginalRead {
			expectedEffect = EffectNone
		}
		if result.Effect != expectedEffect {
			return ProblemNativeResult
		}
		if result.ReturnedN != result.NativeN || result.ReturnedError != result.NativeError || result.ReturnedErrorToken != result.NativeErrorToken || result.NormalizedError != result.NativeError || result.NormalizedErrorToken != result.NativeErrorToken {
			return ProblemErrorMasked
		}
		if action == PrefixShortWrite && result.NativeN > spec.Prefix {
			return ProblemNativeResult
		}
		return ProblemNone
	}
	if action == PrefixShortWrite {
		exposed, normalized := None, ShortWrite
		if spec.PrefixError {
			exposed, normalized = Injected, Injected
		}
		effect := EffectPrefix
		if spec.Prefix == 0 {
			effect = EffectNone
		}
		if result.NativeN != spec.Prefix || result.ReturnedN != spec.Prefix || result.ReturnedError != exposed || result.NormalizedError != normalized || result.Effect != effect {
			return ProblemNativeResult
		}
		return ProblemNone
	}
	if result.ReturnedN != result.NativeN || result.ReturnedError != None {
		return ProblemErrorMasked
	}
	expectedEffect := EffectUnknown
	if call.Point == DBOriginalRead {
		expectedEffect = EffectNone
	}
	if writePoint(call.Point) {
		expectedEffect = EffectPossible
	}
	if result.Effect != expectedEffect {
		return ProblemNativeResult
	}
	normalized := None
	if writePoint(call.Point) && result.NativeN != call.Length {
		normalized = ShortWrite
	}
	if result.NormalizedError != normalized || (call.Point == DBOriginalRead && result.NativeN != call.Length) {
		return ProblemNativeResult
	}
	return ProblemNone
}

// Seal consumes no descriptor, closes no owner, performs no validation, and
// does not infer retirement from logical closure or the absence of diagnostics.
func (r *Receipt) Seal() Observation {
	r.sealed = true
	observation := Observation{Outcome: ObservationInvalid, Problem: r.problem, TargetHits: r.targetHits, EventCount: r.count}
	if !r.plan.valid {
		observation.Problem = ProblemUnadmittedPlan
		return observation
	}
	if r.problem != ProblemNone {
		return observation
	}
	if r.next != EventUnknown || r.targetHits != 1 {
		observation.Outcome, observation.Problem = ObservationIncomplete, ProblemIncomplete
		return observation
	}
	if r.refusal != ProblemNone {
		observation.Outcome, observation.Problem = ObservationRefused, r.refusal
		return observation
	}
	if r.owners.Main != OwnerRetired || r.owners.Cleanup != OwnerRetired || (r.owners.Journal != OwnerRetired && r.owners.Journal != OwnerNotAcquired) {
		observation.Outcome, observation.Problem = ObservationIncomplete, ProblemOwnersUnresolved
		return observation
	}
	observation.Outcome, observation.Problem = AdapterObserved, ProblemNone
	return observation
}
`,
	rollbackFaultContractTest: `package rollbackfault

import "testing"

// SOURCE ONLY: these tests have not been compiled or executed. All fixtures are
// finite values. They create no database, owner, descriptor, process or recovery
// input, and a passing test would validate only this receipt contract.
func onePageSpec(action Action, targetIndex int) PlanSpec {
	spec := PlanSpec{
		Identity:  Identity{Suite: "suite-v1", Source: "source-v1", Tool: "tool-v1", Binary: "binary-v1", Manifest: "manifest-v1", Trial: "trial-1", Scope: "scope-1", Profile: PlainRollback4096},
		MainOwner: 11, JournalOwner: 12, DirectoryOwner: 13, CleanupOwner: 14,
		OriginalPages: 1, DirtyPages: 1, PageSize: 4096, SectorSize: 4096,
		Action: action, ExpectedCommit: CommitSucceeded,
		Calls: [MaxCalls]Call{
			{Point: JournalCreate, Occurrence: 1, Role: Journal, Owner: 12},
			{Point: JournalInvalid, Occurrence: 1, Role: Journal, Owner: 12, Length: 4096},
			{Point: DBOriginalRead, Occurrence: 1, Role: Database, Owner: 11, Length: 4096},
			{Point: JournalRecord, Occurrence: 1, Role: Journal, Owner: 12, Offset: 4096, Length: 4104},
			{Point: JournalBodySync, Occurrence: 1, Role: Journal, Owner: 12},
			{Point: JournalValidHeader, Occurrence: 1, Role: Journal, Owner: 12, Length: 12},
			{Point: JournalHeaderSync, Occurrence: 1, Role: Journal, Owner: 12},
			{Point: PublishDirectorySync, Occurrence: 1, Role: Directory, Owner: 13},
			{Point: DBPage, Occurrence: 1, Role: Database, Owner: 11, Length: 4096},
			{Point: DBSync, Occurrence: 1, Role: Database, Owner: 11},
			{Point: JournalUnlink, Occurrence: 1, Role: Journal, Owner: 12},
			{Point: FinalDirectorySync, Occurrence: 1, Role: Directory, Owner: 13},
		},
		CallCount: 12,
	}
	spec.Target = spec.Calls[targetIndex]
	if action == Pass {
		return spec
	}
	spec.ExpectedCommit = CommitFailed
	if targetIndex >= 8 && targetIndex <= 10 {
		spec.ExpectedCommit = CommitRecoveryRequired
	}
	if targetIndex == 11 {
		spec.ExpectedCommit = CommitUncertain
	}
	if action == PrefixShortWrite {
		spec.Prefix = 7
	}
	spec.CallCount = uint8(targetIndex + 1)
	for i := targetIndex + 1; i < MaxCalls; i++ {
		spec.Calls[i] = Call{}
	}
	if targetIndex > 0 && targetIndex < 8 {
		spec.Calls[spec.CallCount] = Call{Point: CleanupUnlink, Occurrence: 1, Role: Journal, Owner: 12}
		spec.CallCount++
		spec.Calls[spec.CallCount] = Call{Point: CleanupDirectorySync, Occurrence: 1, Role: Directory, Owner: 13}
		spec.CallCount++
	}
	return spec
}

func fixtureEvents(spec PlanSpec) []Event {
	events := []Event{{Kind: RunStarted}}
	targetError := None
	for i := 0; i < int(spec.CallCount); i++ {
		call, action := spec.Calls[i], Pass
		if call == spec.Target {
			action = spec.Action
		}
		result := Result{Executed: true, NativeN: call.Length, ReturnedN: call.Length, Effect: EffectUnknown}
		if call.Point == DBOriginalRead {
			result.Effect = EffectNone
		}
		if writePoint(call.Point) {
			result.Effect = EffectPossible
		}
		switch action {
		case FailBefore:
			result = Result{ReturnedError: Injected, NormalizedError: Injected, Effect: EffectNone}
		case PrefixShortWrite:
			result = Result{Executed: true, NativeN: spec.Prefix, ReturnedN: spec.Prefix, NormalizedError: ShortWrite, Effect: EffectPrefix}
			if spec.Prefix == 0 {
				result.Effect = EffectNone
			}
			if spec.PrefixError {
				result.ReturnedError, result.NormalizedError = Injected, Injected
			}
		}
		if call == spec.Target {
			targetError = result.NormalizedError
		}
		events = append(events, Event{Kind: Intent, Call: call, Action: action}, Event{Kind: Returned, Call: call, Action: action, Result: result})
	}
	commit := CommitResult{Class: spec.ExpectedCommit, Logical: LogicalClosed, TargetError: targetError}
	owners := OwnersResult{MainOwner: 11, JournalOwner: 12, CleanupOwner: 14, Main: OwnerRetired, Journal: OwnerRetired, Cleanup: OwnerRetired}
	if spec.Action != Pass {
		commit.Error, commit.ErrorToken = Underlying, 900
	}
	if spec.Target.Point == JournalCreate && spec.Action == FailBefore {
		commit.Logical = LogicalOpen
		owners.Journal = OwnerNotAcquired
	}
	events = append(events, Event{Kind: CommitReturned, Commit: commit}, Event{Kind: OwnersObserved, Owners: owners}, Event{Kind: ValidationObserved}, Event{Kind: Terminal})
	renumber(events, spec.Identity)
	return events
}

func renumber(events []Event, identity Identity) {
	for i := range events {
		events[i].Identity, events[i].Ordinal = identity, uint8(i+1)
	}
}

func observe(t *testing.T, spec PlanSpec, events []Event) Observation {
	t.Helper()
	plan, problem := NewPlan(spec)
	if problem != NoProblem {
		t.Fatalf("fixture plan refused: %v", problem)
	}
	receipt, problem := NewReceipt(plan)
	if problem != NoProblem {
		t.Fatalf("fixture receipt refused: %v", problem)
	}
	for _, event := range events {
		if receipt.Append(event) != NoProblem {
			break
		}
	}
	return receipt.Seal()
}

func mustNotQualify(t *testing.T, observation Observation) {
	t.Helper()
	if observation.Outcome == AdapterObserved || observation.PhysicalValidated() || observation.Pass() {
		t.Fatalf("invalid evidence qualified: %+v", observation)
	}
}

func TestRollbackFaultFiniteActionsAreAdapterObservationOnly(t *testing.T) {
	for index := 0; index < 12; index++ {
		for _, action := range []Action{Pass, FailBefore, PrefixShortWrite} {
			if action == PrefixShortWrite && index != 1 && index != 3 && index != 5 && index != 8 {
				continue
			}
			spec := onePageSpec(action, index)
			observation := observe(t, spec, fixtureEvents(spec))
			if observation.Outcome != AdapterObserved || observation.TargetHits != 1 || observation.Problem != NoProblem {
				t.Fatalf("point %v action %v: %+v", spec.Target.Point, action, observation)
			}
			if observation.Pass() || observation.PhysicalValidated() {
				t.Fatal("adapter evidence became physical acceptance")
			}
		}
	}
	for _, prefix := range []int{0, 1, 4095} {
		for _, injectError := range []bool{false, true} {
			spec := onePageSpec(PrefixShortWrite, 8)
			spec.Prefix, spec.PrefixError = prefix, injectError
			if observation := observe(t, spec, fixtureEvents(spec)); observation.Outcome != AdapterObserved || observation.Pass() {
				t.Fatalf("prefix %d error %v: %+v", prefix, injectError, observation)
			}
		}
	}
}

func TestRollbackFaultPlanRejectsMalformedFrozenSelectors(t *testing.T) {
	for _, name := range []string{"suite", "source", "tool", "binary", "manifest", "trial", "zero-trial", "scope", "profile", "whitespace", "long-id", "point", "occurrence", "role", "offset", "length", "owner", "duplicate-owner", "zero-owner", "page-size", "sector-size", "zero-originals", "five-dirty", "action", "prefix-read", "prefix-negative", "prefix-full", "prefix-on-pass", "cleanup-target", "commit-success", "missing-call", "duplicated-call", "reordered-call", "hidden-tail", "zero-call-count", "oversized-call-count"} {
		spec := onePageSpec(FailBefore, 8)
		switch name {
		case "suite":
			spec.Identity.Suite = ""
		case "source":
			spec.Identity.Source = ""
		case "tool":
			spec.Identity.Tool = ""
		case "binary":
			spec.Identity.Binary = ""
		case "manifest":
			spec.Identity.Manifest = ""
		case "trial":
			spec.Identity.Trial = ""
		case "zero-trial":
			spec.Identity.Trial = "0"
		case "scope":
			spec.Identity.Scope = ""
		case "profile":
			spec.Identity.Profile = Profile(9)
		case "whitespace":
			spec.Identity.Scope = "scope 1"
		case "long-id":
			for len(spec.Identity.Trial) <= MaxIDLength {
				spec.Identity.Trial += "x"
			}
		case "point":
			spec.Target.Point = PointUnknown
		case "occurrence":
			spec.Target.Occurrence = 0
		case "role":
			spec.Target.Role = Journal
		case "offset":
			spec.Target.Offset = 4096
		case "length":
			spec.Target.Length--
		case "owner":
			spec.Target.Owner = 99
		case "duplicate-owner":
			spec.DirectoryOwner = spec.MainOwner
		case "zero-owner":
			spec.CleanupOwner = 0
		case "page-size":
			spec.PageSize = 8192
		case "sector-size":
			spec.SectorSize = 512
		case "zero-originals":
			spec.OriginalPages = 0
		case "five-dirty":
			spec.DirtyPages = 5
		case "action":
			spec.Action = Action(99)
		case "prefix-read":
			spec = onePageSpec(PrefixShortWrite, 2)
		case "prefix-negative":
			spec.Action, spec.Prefix = PrefixShortWrite, -1
		case "prefix-full":
			spec.Action, spec.Prefix = PrefixShortWrite, spec.Target.Length
		case "prefix-on-pass":
			spec.Prefix = 1
		case "cleanup-target":
			spec.Target.Point = CleanupUnlink
		case "commit-success":
			spec.ExpectedCommit = CommitSucceeded
		case "missing-call":
			spec.Calls[3] = spec.Calls[4]
		case "duplicated-call":
			spec.Calls[4] = spec.Calls[3]
		case "reordered-call":
			spec.Calls[2], spec.Calls[3] = spec.Calls[3], spec.Calls[2]
		case "hidden-tail":
			spec.Calls[spec.CallCount] = spec.Calls[0]
		case "zero-call-count":
			spec.CallCount = 0
		case "oversized-call-count":
			spec.CallCount = MaxCalls + 1
		}
		if _, problem := NewPlan(spec); problem == NoProblem {
			t.Fatalf("malformed %s selector admitted", name)
		}
	}
}

func TestRollbackFaultEveryTruncationAndOutOfOrderEventFailsClosed(t *testing.T) {
	spec := onePageSpec(FailBefore, 8)
	events := fixtureEvents(spec)
	for end := 0; end < len(events); end++ {
		mustNotQualify(t, observe(t, spec, events[:end]))
	}
	for index := range events {
		// Renumbering defeats a mere ordinal/checksum guard; structural omissions
		// and duplicates must still fail against the complete finite call path.
		missing := append([]Event{}, events[:index]...)
		missing = append(missing, events[index+1:]...)
		renumber(missing, spec.Identity)
		mustNotQualify(t, observe(t, spec, missing))
		duplicate := append([]Event{}, events[:index]...)
		duplicate = append(duplicate, events[index])
		duplicate = append(duplicate, events[index:]...)
		renumber(duplicate, spec.Identity)
		mustNotQualify(t, observe(t, spec, duplicate))
		if index+1 < len(events) {
			reordered := append([]Event{}, events...)
			reordered[index], reordered[index+1] = reordered[index+1], reordered[index]
			renumber(reordered, spec.Identity)
			mustNotQualify(t, observe(t, spec, reordered))
		}
	}
}

func TestRollbackFaultStaleIdentityAndMismatchedCallFailClosed(t *testing.T) {
	for _, name := range []string{"suite", "source", "tool", "binary", "manifest", "trial", "scope", "profile", "ordinal", "role", "owner", "offset", "length", "occurrence", "not-hit", "wrong-action", "hidden-payload", "native-return-missing"} {
		spec := onePageSpec(FailBefore, 8)
		events := fixtureEvents(spec)
		index := 1 + 2*8
		event := &events[index]
		switch name {
		case "suite":
			event.Identity.Suite = "old-suite"
		case "source":
			event.Identity.Source = "old-source"
		case "tool":
			event.Identity.Tool = "old-tool"
		case "binary":
			event.Identity.Binary = "old-binary"
		case "manifest":
			event.Identity.Manifest = "old-manifest"
		case "trial":
			event.Identity.Trial = "old-trial"
		case "scope":
			event.Identity.Scope = "old-scope"
		case "profile":
			event.Identity.Profile = ProfileUnknown
		case "ordinal":
			event.Ordinal++
		case "role":
			event.Call.Role = Journal
		case "owner":
			event.Call.Owner = 12
		case "offset":
			event.Call.Offset = 4096
		case "length":
			event.Call.Length--
		case "occurrence":
			event.Call.Occurrence++
		case "not-hit":
			event.Call.Point = DBSync
		case "wrong-action":
			event.Action = Pass
		case "hidden-payload":
			event.Owners.Main = OwnerRetired
		case "native-return-missing":
			events[index+1].Kind = CommitReturned
		}
		mustNotQualify(t, observe(t, spec, events))
	}
}

func TestRollbackFaultUnderlyingErrorNeverBecomesExpectedInjection(t *testing.T) {
	for _, mutation := range []string{"preserved", "masked-native", "masked-exposed", "masked-normalized", "wrong-token", "missing-token", "commit-cause-masked"} {
		spec := onePageSpec(PrefixShortWrite, 8)
		events := fixtureEvents(spec)
		result := &events[2+2*8].Result
		*result = Result{Executed: true, NativeN: 3, NativeError: Underlying, NativeErrorToken: 71, ReturnedN: 3, ReturnedError: Underlying, ReturnedErrorToken: 71, NormalizedError: Underlying, NormalizedErrorToken: 71, Effect: EffectUnknown}
		commit := &events[len(events)-4].Commit
		commit.TargetError, commit.TargetErrorToken = Underlying, 71
		switch mutation {
		case "masked-native":
			result.NativeError, result.NativeErrorToken = None, 0
		case "masked-exposed":
			result.ReturnedError, result.ReturnedErrorToken = Injected, 0
		case "masked-normalized":
			result.NormalizedError, result.NormalizedErrorToken = ShortWrite, 0
		case "wrong-token":
			result.NormalizedErrorToken = 72
		case "missing-token":
			result.NativeErrorToken = 0
		case "commit-cause-masked":
			commit.TargetError, commit.TargetErrorToken = Injected, 0
		}
		observation := observe(t, spec, events)
		mustNotQualify(t, observation)
		if mutation == "preserved" && (observation.Outcome != ObservationRefused || observation.Problem != ProblemUnderlyingFailure) {
			t.Fatalf("preserved original error not distinguished from contract invalidity: %+v", observation)
		}
	}
}

func TestRollbackFaultOwnersAndLogicalClosureAreIndependent(t *testing.T) {
	for _, state := range []OwnerState{OwnerUnknown, OwnerLive, OwnerPending} {
		for owner := 0; owner < 3; owner++ {
			spec := onePageSpec(FailBefore, 8)
			events := fixtureEvents(spec)
			observed := &events[len(events)-3].Owners
			switch owner {
			case 0:
				observed.Main = state
			case 1:
				observed.Journal = state
			case 2:
				observed.Cleanup = state
			}
			observation := observe(t, spec, events)
			mustNotQualify(t, observation)
			if observation.Problem != ProblemOwnersUnresolved {
				t.Fatalf("logical close incorrectly resolved owner %d state %v: %+v", owner, state, observation)
			}
		}
	}
	for _, mutation := range []string{"wrong-main", "wrong-journal", "wrong-cleanup", "journal-gone", "main-not-acquired", "cleanup-not-acquired", "logical-unknown", "logical-open"} {
		spec := onePageSpec(FailBefore, 8)
		events := fixtureEvents(spec)
		owners := &events[len(events)-3].Owners
		switch mutation {
		case "wrong-main":
			owners.MainOwner++
		case "wrong-journal":
			owners.JournalOwner++
		case "wrong-cleanup":
			owners.CleanupOwner++
		case "journal-gone":
			owners.Journal = OwnerNotAcquired
		case "main-not-acquired":
			owners.Main = OwnerNotAcquired
		case "cleanup-not-acquired":
			owners.Cleanup = OwnerNotAcquired
		case "logical-unknown":
			events[len(events)-4].Commit.Logical = LogicalUnknown
		case "logical-open":
			events[len(events)-4].Commit.Logical = LogicalOpen
		}
		mustNotQualify(t, observe(t, spec, events))
	}
}

func TestRollbackFaultUncertainAndUnqualifiedCommitNeverPass(t *testing.T) {
	for _, class := range []CommitClass{CommitUnknown, CommitSucceeded, CommitFailed, CommitUncertain, CommitCleanupPending, CommitUnsupported, CommitRefused, CommitSkipped} {
		spec := onePageSpec(FailBefore, 8)
		events := fixtureEvents(spec)
		events[len(events)-4].Commit.Class = class
		if class == CommitSucceeded {
			events[len(events)-4].Commit.Error, events[len(events)-4].Commit.ErrorToken = None, 0
		}
		mustNotQualify(t, observe(t, spec, events))
	}
	// DB write fail-before still requires recovery in the production protocol,
	// despite this individual operation reporting no native execution or effect.
	db := onePageSpec(FailBefore, 8)
	if db.ExpectedCommit != CommitRecoveryRequired {
		t.Fatal("fixture lost conservative databaseTouched classification")
	}
	final := onePageSpec(FailBefore, 11)
	events := fixtureEvents(final)
	observation := observe(t, final, events)
	if observation.Outcome != AdapterObserved || observation.PhysicalValidated() || observation.Pass() {
		t.Fatalf("uncertain commit was upgraded to physical success: %+v", observation)
	}
	events[len(events)-4].Commit.Class = CommitSucceeded
	events[len(events)-4].Commit.Error, events[len(events)-4].Commit.ErrorToken = None, 0
	mustNotQualify(t, observe(t, final, events))
}

func TestRollbackFaultPhysicalEvidenceCannotBeManufactured(t *testing.T) {
	spec := onePageSpec(Pass, 8)
	for _, claim := range []Validation{1, 2, 3, 255} {
		events := fixtureEvents(spec)
		events[len(events)-2].Validation = claim
		mustNotQualify(t, observe(t, spec, events))
	}
	// No recovery, old/new ledger, index verification, or acknowledged-commit
	// ledger can be attached through this model. Even a fabricated outcome value
	// has no method that grants physical acceptance.
	for _, outcome := range []Outcome{ObservationInvalid, ObservationIncomplete, ObservationRefused, AdapterObserved, Outcome(255)} {
		observation := Observation{Outcome: outcome}
		if observation.Pass() || observation.PhysicalValidated() {
			t.Fatal("manufactured physical validation")
		}
	}
}

func TestRollbackFaultValueCopiesAndStickyTerminal(t *testing.T) {
	spec := onePageSpec(FailBefore, 8)
	plan, problem := NewPlan(spec)
	if problem != NoProblem {
		t.Fatal(problem)
	}
	spec.Calls[0].Owner = 999
	copySpec := plan.Spec()
	copySpec.Identity.Trial, copySpec.Calls[0].Owner = "altered", 998
	if plan.Spec().Calls[0].Owner != 12 || plan.Spec().Identity.Trial != "trial-1" {
		t.Fatal("plan retains mutable caller storage")
	}
	receipt, problem := NewReceipt(plan)
	if problem != NoProblem {
		t.Fatal(problem)
	}
	events := fixtureEvents(plan.Spec())
	for _, event := range events {
		if problem := receipt.Append(event); problem != NoProblem {
			t.Fatal(problem)
		}
	}
	snapshot, count := receipt.Events()
	snapshot[0].Identity.Scope = "old-scope"
	second, secondCount := receipt.Events()
	if count != secondCount || second[0].Identity.Scope != "scope-1" {
		t.Fatal("receipt snapshot aliases internal evidence")
	}
	first := receipt.Seal()
	if problem := receipt.Append(events[0]); problem != ProblemSealed {
		t.Fatal(problem)
	}
	if again := receipt.Seal(); again != first {
		t.Fatal("sealing changed terminal receipt")
	}
	bad, _ := NewReceipt(plan)
	malformed := events[0]
	malformed.Ordinal = 9
	if bad.Append(malformed) != ProblemOrdinal || bad.Append(events[0]) != ProblemOrdinal {
		t.Fatal("invalid receipt was silently reset as a new trial")
	}
	mustNotQualify(t, bad.Seal())
	if _, problem := NewReceipt(Plan{}); problem != ProblemUnadmittedPlan {
		t.Fatal(problem)
	}
	var zero Receipt
	if problem := zero.Append(events[0]); problem != ProblemUnadmittedPlan {
		t.Fatal(problem)
	}
	mustNotQualify(t, zero.Seal())
}

func TestRollbackFaultBoundedStorageCannotOverflowOrBecomeNewTrial(t *testing.T) {
	spec := onePageSpec(FailBefore, 8)
	plan, problem := NewPlan(spec)
	if problem != NoProblem {
		t.Fatal(problem)
	}
	receipt, _ := NewReceipt(plan)
	// The admitted finite path normally rejects long streams before this bound.
	// This same-package source test addresses the fixed-array overflow guard
	// independently without executing any adapter, fixture, or native operation.
	receipt.count = MaxEvents
	if receipt.Append(Event{}) != ProblemEventLimit {
		t.Fatal("event bound not enforced")
	}
	if receipt.count != MaxEvents {
		t.Fatal("overflow wrapped ordinal counter")
	}
	mustNotQualify(t, receipt.Seal())
	complete, _ := NewReceipt(plan)
	for _, event := range fixtureEvents(spec) {
		if complete.Append(event) != NoProblem {
			t.Fatal("fixture rejected")
		}
	}
	extra := Event{Identity: spec.Identity, Ordinal: complete.count + 1, Kind: RunStarted}
	if complete.Append(extra) != ProblemEventOrder {
		t.Fatal("second trial admitted after terminal")
	}
	mustNotQualify(t, complete.Seal())
}

func TestRollbackFaultFourPageManifestUsesPerPointOccurrences(t *testing.T) {
	spec := onePageSpec(Pass, 8)
	spec.OriginalPages, spec.DirtyPages, spec.CallCount = 4, 4, 21
	spec.Calls = [MaxCalls]Call{
		{Point: JournalCreate, Occurrence: 1, Role: Journal, Owner: 12},
		{Point: JournalInvalid, Occurrence: 1, Role: Journal, Owner: 12, Length: 4096},
		{Point: DBOriginalRead, Occurrence: 1, Role: Database, Owner: 11, Length: 4096},
		{Point: JournalRecord, Occurrence: 1, Role: Journal, Owner: 12, Offset: 4096, Length: 4104},
		{Point: DBOriginalRead, Occurrence: 2, Role: Database, Owner: 11, Offset: 4096, Length: 4096},
		{Point: JournalRecord, Occurrence: 2, Role: Journal, Owner: 12, Offset: 8200, Length: 4104},
		{Point: DBOriginalRead, Occurrence: 3, Role: Database, Owner: 11, Offset: 8192, Length: 4096},
		{Point: JournalRecord, Occurrence: 3, Role: Journal, Owner: 12, Offset: 12304, Length: 4104},
		{Point: DBOriginalRead, Occurrence: 4, Role: Database, Owner: 11, Offset: 12288, Length: 4096},
		{Point: JournalRecord, Occurrence: 4, Role: Journal, Owner: 12, Offset: 16408, Length: 4104},
		{Point: JournalBodySync, Occurrence: 1, Role: Journal, Owner: 12},
		{Point: JournalValidHeader, Occurrence: 1, Role: Journal, Owner: 12, Length: 12},
		{Point: JournalHeaderSync, Occurrence: 1, Role: Journal, Owner: 12},
		{Point: PublishDirectorySync, Occurrence: 1, Role: Directory, Owner: 13},
		{Point: DBPage, Occurrence: 1, Role: Database, Owner: 11, Length: 4096},
		{Point: DBPage, Occurrence: 2, Role: Database, Owner: 11, Offset: 4096, Length: 4096},
		{Point: DBPage, Occurrence: 3, Role: Database, Owner: 11, Offset: 8192, Length: 4096},
		{Point: DBPage, Occurrence: 4, Role: Database, Owner: 11, Offset: 12288, Length: 4096},
		{Point: DBSync, Occurrence: 1, Role: Database, Owner: 11},
		{Point: JournalUnlink, Occurrence: 1, Role: Journal, Owner: 12},
		{Point: FinalDirectorySync, Occurrence: 1, Role: Directory, Owner: 13},
	}
	spec.Target = spec.Calls[17]
	observation := observe(t, spec, fixtureEvents(spec))
	if observation.Outcome != AdapterObserved || observation.EventCount != 47 || observation.TargetHits != 1 {
		t.Fatalf("four-page finite trace rejected: %+v", observation)
	}
	for _, mutation := range []string{"repeat-page", "misaligned-page", "outside-bound", "wrong-record-offset", "wrong-record-length", "global-ordinal", "fewer-originals", "fewer-dirty"} {
		changed := spec
		switch mutation {
		case "repeat-page":
			changed.Calls[15].Offset = 0
		case "misaligned-page":
			changed.Calls[15].Offset++
		case "outside-bound":
			changed.Calls[17].Offset = 4 * 4096
		case "wrong-record-offset":
			changed.Calls[5].Offset = 8192
		case "wrong-record-length":
			changed.Calls[5].Length = 4096
		case "global-ordinal":
			changed.Calls[5].Occurrence = 6
		case "fewer-originals":
			changed.OriginalPages = 3
		case "fewer-dirty":
			changed.DirtyPages = 3
		}
		if _, problem := NewPlan(changed); problem == NoProblem {
			t.Fatalf("invalid multi-page %s accepted", mutation)
		}
	}
	spec.Action, spec.Prefix, spec.ExpectedCommit, spec.CallCount = PrefixShortWrite, 4095, CommitRecoveryRequired, 18
	for i := 18; i < MaxCalls; i++ {
		spec.Calls[i] = Call{}
	}
	if observation := observe(t, spec, fixtureEvents(spec)); observation.Outcome != AdapterObserved || observation.TargetHits != 1 {
		t.Fatalf("fourth occurrence prefix receipt rejected: %+v", observation)
	}
}

func TestRollbackFaultResultNormalizationAndEffectClaimsFailClosed(t *testing.T) {
	for _, mutation := range []string{"executed-fail-before", "native-fail-before", "fail-before-effect", "negative-native-count", "excess-native-count", "wrong-exposed-count", "masked-short-write", "wrong-prefix-count", "prefix-effect-none", "prefix-effect-unknown", "fake-native-injected", "synthetic-token", "pass-write-effect-none", "read-short-nil", "read-fake-effect", "create-fake-no-effect", "native-write-fake-no-effect"} {
		spec := onePageSpec(PrefixShortWrite, 8)
		if mutation == "executed-fail-before" || mutation == "native-fail-before" || mutation == "fail-before-effect" {
			spec = onePageSpec(FailBefore, 8)
		}
		events := fixtureEvents(spec)
		result := &events[18].Result
		switch mutation {
		case "executed-fail-before":
			result.Executed = true
		case "native-fail-before":
			result.NativeN = 1
		case "fail-before-effect":
			result.Effect = EffectPrefix
		case "negative-native-count":
			result.NativeN = -1
		case "excess-native-count":
			result.NativeN = 4097
		case "wrong-exposed-count":
			result.ReturnedN++
		case "masked-short-write":
			result.NormalizedError = None
		case "wrong-prefix-count":
			result.NativeN, result.ReturnedN = 6, 6
		case "prefix-effect-none":
			result.Effect = EffectNone
		case "prefix-effect-unknown":
			result.Effect = EffectUnknown
		case "fake-native-injected":
			result.NativeError = Injected
		case "synthetic-token":
			result.NormalizedErrorToken = 99
		case "pass-write-effect-none":
			events[4].Result.Effect = EffectNone
		case "read-short-nil":
			events[6].Result.NativeN, events[6].Result.ReturnedN = 4095, 4095
		case "read-fake-effect":
			events[6].Result.Effect = EffectPrefix
		case "create-fake-no-effect":
			events[2].Result.Effect = EffectNone
		case "native-write-fake-no-effect":
			*result = Result{Executed: true, NativeN: 3, NativeError: Underlying, NativeErrorToken: 77, ReturnedN: 3, ReturnedError: Underlying, ReturnedErrorToken: 77, NormalizedError: Underlying, NormalizedErrorToken: 77, Effect: EffectNone}
			events[len(events)-4].Commit.TargetError, events[len(events)-4].Commit.TargetErrorToken = Underlying, 77
		}
		mustNotQualify(t, observe(t, spec, events))
	}
}

func TestRollbackFaultAcknowledgementAndRecoveryLedgersRemainUnproven(t *testing.T) {
	for _, action := range []Action{Pass, FailBefore} {
		spec := onePageSpec(action, 8)
		observation := observe(t, spec, fixtureEvents(spec))
		if observation.Outcome != AdapterObserved {
			t.Fatal("fixture observation malformed")
		}
		// This complete trace supplies no independent old/new ledger, ordinary
		// index or extension-index check, or durable acknowledgement history.
		// Neither success nor recovery-required can certify those missing facts.
		if observation.Pass() || observation.PhysicalValidated() {
			t.Fatal("absent recovery/ledger/index/acknowledgement evidence treated as PASS")
		}
	}
}

func TestRollbackFaultCleanupIsCompleteAndObservationOnly(t *testing.T) {
	spec := onePageSpec(FailBefore, 6)
	for _, mutation := range []string{"second-fault-unlink", "second-fault-directory", "missing-cleanup-pair", "reversed-cleanup", "wrong-cleanup-owner", "spontaneous-cleanup-error"} {
		events := fixtureEvents(spec)
		unlinkIntent := 1 + 2*(int(spec.CallCount)-2)
		directoryIntent := unlinkIntent + 2
		switch mutation {
		case "second-fault-unlink":
			events[unlinkIntent].Action = FailBefore
		case "second-fault-directory":
			events[directoryIntent].Action = FailBefore
		case "missing-cleanup-pair":
			events = append(events[:unlinkIntent], events[directoryIntent:]...)
			renumber(events, spec.Identity)
		case "reversed-cleanup":
			events[unlinkIntent].Call, events[directoryIntent].Call = events[directoryIntent].Call, events[unlinkIntent].Call
		case "wrong-cleanup-owner":
			events[directoryIntent].Call.Owner = spec.CleanupOwner
		case "spontaneous-cleanup-error":
			events[directoryIntent+1].Result = Result{Executed: true, NativeError: Underlying, NativeErrorToken: 55, ReturnedError: Underlying, ReturnedErrorToken: 55, NormalizedError: Underlying, NormalizedErrorToken: 55, Effect: EffectUnknown}
		}
		mustNotQualify(t, observe(t, spec, events))
	}
	for _, mutation := range []string{"omit-cleanup", "omit-directory", "third-cleanup"} {
		changed := spec
		switch mutation {
		case "omit-cleanup":
			changed.Calls[changed.CallCount-2], changed.Calls[changed.CallCount-1] = Call{}, Call{}
			changed.CallCount -= 2
		case "omit-directory":
			changed.Calls[changed.CallCount-1] = Call{}
			changed.CallCount--
		case "third-cleanup":
			changed.Calls[changed.CallCount] = changed.Calls[changed.CallCount-1]
			changed.CallCount++
		}
		if _, problem := NewPlan(changed); problem == NoProblem {
			t.Fatalf("malformed cleanup inventory %s admitted", mutation)
		}
	}
}

// RF-01 SOURCE ONLY: these regressions describe bounded retained evidence. They
// do not measure heap allocation or establish native or physical qualification.
func TestRollbackFaultEveryIdentityFieldRejectsOversizedRetention(t *testing.T) {
	for _, size := range []int{MaxIDLength + 1, 1 << 20} {
		payload := make([]byte, size)
		for i := range payload {
			payload[i] = 'x'
		}
		value := string(payload)
		for _, field := range []string{"suite", "source", "tool", "binary", "manifest", "trial", "scope"} {
			spec := onePageSpec(FailBefore, 8)
			plan, problem := NewPlan(spec)
			if problem != NoProblem {
				t.Fatal(problem)
			}
			badSpec := spec
			switch field {
			case "suite":
				badSpec.Identity.Suite = value
			case "source":
				badSpec.Identity.Source = value
			case "tool":
				badSpec.Identity.Tool = value
			case "binary":
				badSpec.Identity.Binary = value
			case "manifest":
				badSpec.Identity.Manifest = value
			case "trial":
				badSpec.Identity.Trial = value
			case "scope":
				badSpec.Identity.Scope = value
			}
			if rejected, problem := NewPlan(badSpec); problem != ProblemIdentity || rejected != (Plan{}) {
				t.Fatalf("oversized %s length %d retained in plan: problem %v", field, size, problem)
			}
			receipt, problem := NewReceipt(plan)
			if problem != NoProblem {
				t.Fatal(problem)
			}
			bad := Event{
				Identity: badSpec.Identity, Ordinal: 251, Kind: Returned,
				Call: spec.Target, Action: PrefixShortWrite,
				Result:     Result{Executed: true, NativeN: 99, NativeError: Underlying, NativeErrorToken: 77, ReturnedN: 98, ReturnedError: Underlying, ReturnedErrorToken: 78, NormalizedError: Underlying, NormalizedErrorToken: 79, Effect: EffectPossible},
				Commit:     CommitResult{Class: CommitSucceeded, Error: Underlying, ErrorToken: 80, TargetError: Underlying, TargetErrorToken: 81, Logical: LogicalClosed},
				Owners:     OwnersResult{MainOwner: 91, JournalOwner: 92, CleanupOwner: 93, Main: OwnerRetired, Journal: OwnerRetired, Cleanup: OwnerRetired},
				Validation: 255, Rejection: ProblemEventOrder, RejectedAt: 250,
			}
			if problem := receipt.Append(bad); problem != ProblemIdentity {
				t.Fatalf("oversized %s length %d lost identity precedence: %v", field, size, problem)
			}
			want := [MaxEvents]Event{{Ordinal: 251, Kind: EventIdentityRejected, Rejection: ProblemIdentity, RejectedAt: 1}}
			snapshot, count := receipt.Events()
			if count != 1 || snapshot != want {
				t.Fatalf("oversized %s length %d retained payload or lost original ordinal/actual slot", field, size)
			}
			if receipt.next != RunStarted || receipt.callIndex != 0 || receipt.targetHits != 0 || receipt.targetResult != (Result{}) {
				t.Fatalf("oversized %s length %d advanced accepted receipt state", field, size)
			}
			observation := receipt.Seal()
			mustNotQualify(t, observation)
			if observation.Outcome != ObservationInvalid || observation.Problem != ProblemIdentity || observation.EventCount != 1 || observation.TargetHits != 0 {
				t.Fatalf("oversized %s length %d lost terminal failure evidence: %+v", field, size, observation)
			}
		}
	}
}

func TestRollbackFaultIdentityRejectionKeepsFirstMarkerAndDetachedSnapshot(t *testing.T) {
	payload := make([]byte, 1<<20)
	for i := range payload {
		payload[i] = 'x'
	}
	for _, prior := range []int{0, 1} {
		spec := onePageSpec(FailBefore, 8)
		plan, problem := NewPlan(spec)
		if problem != NoProblem {
			t.Fatal(problem)
		}
		receipt, problem := NewReceipt(plan)
		if problem != NoProblem {
			t.Fatal(problem)
		}
		events := fixtureEvents(spec)
		want := [MaxEvents]Event{}
		for i := 0; i < prior; i++ {
			if problem := receipt.Append(events[i]); problem != NoProblem {
				t.Fatal(problem)
			}
			want[i] = events[i]
		}
		bad := events[prior]
		bad.Identity.Scope, bad.Ordinal = string(payload), 251
		if problem := receipt.Append(bad); problem != ProblemIdentity {
			t.Fatalf("identity rejection after %d valid events lost precedence over wrong ordinal: %v", prior, problem)
		}
		want[prior] = Event{Ordinal: 251, Kind: EventIdentityRejected, Rejection: ProblemIdentity, RejectedAt: uint8(prior + 1)}
		snapshot, count := receipt.Events()
		if count != uint8(prior+1) || snapshot != want {
			t.Fatalf("first rejection after %d valid events lost prefix, original ordinal, actual slot, or zero payload", prior)
		}
		// Mutating both an earlier event and the marker in a returned snapshot
		// must leave the original finite failure evidence untouched.
		snapshot[prior] = events[prior]
		snapshot[0].Identity.Scope = "changed-snapshot"
		second, secondCount := receipt.Events()
		if secondCount != count || second != want {
			t.Fatal("rejected receipt snapshot aliases internal evidence")
		}
		bad.Identity.Trial, bad.Ordinal = string(payload), 19
		for _, retry := range []Event{events[prior], bad, events[0]} {
			if problem := receipt.Append(retry); problem != ProblemIdentity {
				t.Fatalf("retry after identity failure changed sticky problem: %v", problem)
			}
			retained, retainedCount := receipt.Events()
			if retainedCount != count || retained != want {
				t.Fatal("retry replaced or extended the first rejection evidence")
			}
		}
		observation := receipt.Seal()
		mustNotQualify(t, observation)
		if observation.Outcome != ObservationInvalid || observation.Problem != ProblemIdentity || observation.EventCount != count || observation.TargetHits != 0 {
			t.Fatalf("sticky identity failure lost terminal evidence: %+v", observation)
		}
		if again := receipt.Seal(); again != observation {
			t.Fatal("repeated seal changed identity rejection")
		}
		if problem := receipt.Append(events[prior]); problem != ProblemSealed {
			t.Fatal(problem)
		}
		retained, retainedCount := receipt.Events()
		if retainedCount != count || retained != want {
			t.Fatal("sealed receipt changed first rejection evidence")
		}
	}
}

func TestRollbackFaultStaleIdentityAndBadProfileUseBoundedMarkers(t *testing.T) {
	for _, mutation := range []string{"stale-trial", "stale-scope", "unknown-profile", "unsupported-profile"} {
		spec := onePageSpec(FailBefore, 8)
		plan, problem := NewPlan(spec)
		if problem != NoProblem {
			t.Fatal(problem)
		}
		receipt, problem := NewReceipt(plan)
		if problem != NoProblem {
			t.Fatal(problem)
		}
		bad := fixtureEvents(spec)[0]
		switch mutation {
		case "stale-trial":
			bad.Identity.Trial = "old-trial"
		case "stale-scope":
			bad.Identity.Scope = "old-scope"
		case "unknown-profile":
			bad.Identity.Profile = ProfileUnknown
		case "unsupported-profile":
			bad.Identity.Profile = Profile(255)
		}
		if problem := receipt.Append(bad); problem != ProblemIdentity {
			t.Fatalf("%s did not reject identity before retention: %v", mutation, problem)
		}
		want := [MaxEvents]Event{{Ordinal: 1, Kind: EventIdentityRejected, Rejection: ProblemIdentity, RejectedAt: 1}}
		snapshot, count := receipt.Events()
		if count != 1 || snapshot != want {
			t.Fatalf("%s did not retain only a bounded identity marker", mutation)
		}
		observation := receipt.Seal()
		mustNotQualify(t, observation)
		if observation.Problem != ProblemIdentity || observation.EventCount != 1 {
			t.Fatalf("%s lost identity rejection evidence: %+v", mutation, observation)
		}
	}
}

func TestRollbackFaultRejectionMarkerAndMetadataCannotQualify(t *testing.T) {
	spec := onePageSpec(Pass, 8)
	for _, annotated := range []bool{false, true} {
		events := fixtureEvents(spec)
		events[0].Kind = EventIdentityRejected
		if annotated {
			events[0].Rejection, events[0].RejectedAt = ProblemIdentity, 1
		}
		observation := observe(t, spec, events)
		mustNotQualify(t, observation)
		if observation.Problem != ProblemEventOrder || observation.EventCount != 1 {
			t.Fatalf("forged marker became an admissible event: %+v", observation)
		}
	}
	// Every normal event kind must keep both marker-only fields zero, including
	// an otherwise complete terminal event at the end of an admitted trace.
	for index := range fixtureEvents(spec) {
		for _, field := range []string{"rejection", "rejected-at", "both"} {
			events := fixtureEvents(spec)
			if field != "rejected-at" {
				events[index].Rejection = ProblemIdentity
			}
			if field != "rejection" {
				events[index].RejectedAt = uint8(index + 1)
			}
			observation := observe(t, spec, events)
			mustNotQualify(t, observation)
			if observation.Problem != ProblemEventPayload || observation.EventCount != uint8(index+1) {
				t.Fatalf("normal kind %v with %s marker metadata qualified: %+v", events[index].Kind, field, observation)
			}
		}
	}
}

func TestRollbackFaultLargeBackedValidSubstringsPreserveAdmittedValues(t *testing.T) {
	spec := onePageSpec(Pass, 8)
	want := spec
	values := [7]string{spec.Identity.Suite, spec.Identity.Source, spec.Identity.Tool, spec.Identity.Binary, spec.Identity.Manifest, spec.Identity.Trial, spec.Identity.Scope}
	for i, value := range values {
		payload := make([]byte, 1<<20)
		for j := range payload {
			payload[j] = 'x'
		}
		copy(payload, value)
		backing := string(payload)
		values[i] = backing[:len(value)]
	}
	spec.Identity = Identity{Suite: values[0], Source: values[1], Tool: values[2], Binary: values[3], Manifest: values[4], Trial: values[5], Scope: values[6], Profile: PlainRollback4096}
	plan, problem := NewPlan(spec)
	if problem != NoProblem {
		t.Fatal(problem)
	}
	if plan.Spec() != want {
		t.Fatal("valid substring admission changed frozen values")
	}
	receipt, problem := NewReceipt(plan)
	if problem != NoProblem {
		t.Fatal(problem)
	}
	// Event identities also arrive as short substrings of large source strings.
	// Value checks exercise the admitted path; the explicit bounded copy in the
	// implementation, not this source test, addresses backing-storage retention.
	events := fixtureEvents(spec)
	spec.Identity = Identity{}
	for i := range values {
		values[i] = "changed-source-value"
	}
	if plan.Spec() != want {
		t.Fatal("caller reassignment changed frozen plan")
	}
	for _, event := range events {
		if problem := receipt.Append(event); problem != NoProblem {
			t.Fatal(problem)
		}
	}
	snapshot, count := receipt.Events()
	if count != uint8(len(events)) {
		t.Fatal("valid substring receipt lost events")
	}
	for i := 0; i < int(count); i++ {
		if snapshot[i] != events[i] || snapshot[i].Identity != want.Identity || snapshot[i].Rejection != NoProblem || snapshot[i].RejectedAt != 0 {
			t.Fatalf("valid substring event %d lost canonical admitted values", i)
		}
	}
	observation := receipt.Seal()
	if observation.Outcome != AdapterObserved || observation.Problem != NoProblem || observation.TargetHits != 1 || observation.EventCount != count {
		t.Fatalf("valid substring receipt was refused: %+v", observation)
	}
	if observation.Pass() || observation.PhysicalValidated() {
		t.Fatal("valid substring receipt became physical PASS")
	}
}
`,
}
