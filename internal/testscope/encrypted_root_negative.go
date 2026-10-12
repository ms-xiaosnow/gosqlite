package main

// A finite source boundary for one manual entry, not arbitrary-program safety
// analysis. Full-source pins require independent review; structure checks remain
// useful when testing deliberate repinned counterexamples in memory.
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

const encryptedRootNative = "sqlite/encrypted_root_negative_manual_linux_test.go"
const encryptedRootModel = "internal/testkit/encryptedroot/contract.go"
const encryptedRootModelTest = "internal/testkit/encryptedroot/contract_test.go"
const encryptedRootGuard = "internal/testscope/encrypted_root_negative.go"
const encryptedRootGuardTest = "internal/testscope/encrypted_root_negative_test.go"
const encryptedRootImport = "github.com/ms-xiaosnow/gosqlite/internal/testkit/encryptedroot"
const encryptedRootEntry = "TestManualEncryptedRootNegative"
const encryptedRootTag = "//go:build linux && manual_encrypted_root_negative"

// These exact source hashes were independently reviewed after the sole
// ReadDir diagnostic correction and formatting. Physical cases remain NOT_RUN.
var encryptedRootPins = map[string]string{
	encryptedRootNative:          "3e1e3640ff321a7673d9f39c29d32cfe73ec2d4c53858205ee9a61b7738bbbc3",
	encryptedRootModel:           "21713928744c666e6bccc08e053d6f18ab4e3a92740c655db5896c38f3fd1a3f",
	encryptedRootModelTest:       "4fcfaaa2b47fc8fbaa58e096b0db56f6be2c01c9b75c021590ed91e2e59facbf",
	"internal/testscope/main.go": "1d9f41ed857bf64a3d2913f96fee3409a173f036df753efd81a7a9190ee2536a",
}

func encryptedRootName(x ast.Expr) string {
	switch n := x.(type) {
	case *ast.Ident:
		return n.Name
	case *ast.SelectorExpr:
		return encryptedRootName(n.X) + "." + n.Sel.Name
	case *ast.ParenExpr:
		return encryptedRootName(n.X)
	case *ast.CallExpr:
		return "()"
	case *ast.IndexExpr:
		return encryptedRootName(n.X) + "[]"
	case *ast.ArrayType:
		if n.Len == nil && encryptedRootName(n.Elt) == "byte" {
			return "[]byte"
		}
	}
	return "<unreviewed>"
}
func encryptedRootText(set *token.FileSet, raw []byte, n ast.Node) string {
	return encryptedRootCompact(string(raw[set.Position(n.Pos()).Offset:set.Position(n.End()).Offset]))
}

// Token spelling ignores gofmt whitespace and separator semicolons but keeps
// literal contents. It only compares the finite statements below; input is
// already parsed and is never type-checked or executed by this normalizer.
func encryptedRootCompact(raw string) string {
	set := token.NewFileSet()
	file := set.AddFile("statement", set.Base(), len(raw))
	var scan scanner.Scanner
	scan.Init(file, []byte(raw), nil, 0)
	var parts []string
	for {
		_, kind, literal := scan.Scan()
		if kind == token.EOF {
			break
		}
		if kind == token.SEMICOLON {
			continue
		}
		if literal == "" {
			literal = kind.String()
		}
		parts = append(parts, literal)
	}
	return strings.Join(parts, " ")
}

func encryptedRootSet(words string) map[string]bool {
	out := map[string]bool{}
	for _, word := range strings.Fields(words) {
		out[word] = true
	}
	return out
}

func inspectEncryptedRootNegativeStructure(path string, raw []byte) ([]string, error) {
	path = filepath.ToSlash(filepath.Clean(path))
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, path, raw, 0)
	if err != nil {
		return nil, err
	}
	var findings []string
	report := func(text string) { findings = append(findings, path+": encrypted root negative: "+text) }
	native := path == encryptedRootNative
	model := path == encryptedRootModel || path == encryptedRootModelTest
	guard := path == encryptedRootGuard || path == encryptedRootGuardTest
	if !native && !model {
		if !guard {
			for _, line := range strings.Split(string(raw), "\n") {
				if strings.HasPrefix(strings.TrimSpace(line), "//go:build") && strings.Contains(line, "manual_encrypted_root_negative") {
					report("copied manual tag")
				}
			}
			ast.Inspect(file, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok && (id.Name == encryptedRootEntry || strings.HasPrefix(id.Name, "rootAuth")) {
					report("copied manual capability")
				}
				return true
			})
		}
		for _, imp := range file.Imports {
			value, e := strconv.Unquote(imp.Path.Value)
			if e != nil {
				return findings, e
			}
			if value == encryptedRootImport {
				report("unapproved model consumer")
			}
		}
		if path == "internal/testscope/main.go" {
			counts := map[string]int{}
			ast.Inspect(file, func(n ast.Node) bool {
				if call, ok := n.(*ast.CallExpr); ok {
					counts[encryptedRootName(call.Fun)]++
				}
				return true
			})
			if counts["inspectEncryptedRootNegativeSource"] != 1 || counts["inspectEncryptedRootNegativeContracts"] != 1 {
				report("missing or duplicate main guard hook")
			}
		}
		return findings, nil
	}
	allowedImports := encryptedRootSet("fmt reflect strings")
	if path == encryptedRootModelTest {
		allowedImports = encryptedRootSet("fmt strings testing")
	}
	if native {
		allowedImports = encryptedRootSet("bytes context crypto/sha256 encoding/json errors flag fmt io os path/filepath reflect runtime runtime/debug sort sync syscall testing time github.com/ms-xiaosnow/gosqlite/encryption github.com/ms-xiaosnow/gosqlite/internal/testkit/encryptedroot github.com/ms-xiaosnow/gosqlite/sqlite")
		if strings.SplitN(string(raw), "\n", 2)[0] != encryptedRootTag || file.Name.Name != "sqlite_test" {
			report("exact Linux AND manual source selection required")
		}
	} else if file.Name.Name != "encryptedroot" || strings.Contains(string(raw), "//go:build") {
		report("untagged scalar model required")
	}
	for _, imp := range file.Imports {
		value, e := strconv.Unquote(imp.Path.Value)
		if e != nil {
			return findings, e
		}
		if imp.Name != nil || !allowedImports[value] {
			report("import capability expanded")
		}
	}
	if !native {
		return findings, nil
	}
	parents := oracleParents(file)
	functions, calls, critical := map[string]int{}, map[string]int{}, map[string]int{}
	pairs := 0
	ast.Inspect(file, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncDecl:
			functions[node.Name.Name]++
			if !encryptedRootFunctions[node.Name.Name] || node.Body == nil {
				report("function capability expanded")
			}
			if node.Name.Name == encryptedRootEntry && (node.Body == nil || len(node.Body.List) == 0 || encryptedRootText(set, raw, node.Body.List[0]) != encryptedRootCompact(encryptedRootGate)) {
				report("first authorization gate changed")
			}
		case *ast.GoStmt:
			report("goroutine forbidden")
		case *ast.ForStmt:
			if node.Cond == nil {
				report("unbounded loop forbidden")
			}
		case *ast.CallExpr:
			name := encryptedRootName(node.Fun)
			calls[name]++
			if !encryptedRootCalls[name] {
				report("call capability expanded: " + name)
			}
		case *ast.SelectorExpr:
			if encryptedRootEffects[node.Sel.Name] {
				call, ok := parents[node].(*ast.CallExpr)
				if !ok || call.Fun != node {
					report("effect alias forbidden")
				}
			}
		case *ast.BlockStmt:
			for i, statement := range node.List {
				fn := oracleEnclosingFunction(statement, parents)
				if fn != nil {
					for key := range encryptedRootCritical {
						parts := strings.SplitN(key, "/", 2)
						if fn.Name.Name == parts[0] && encryptedRootText(set, raw, statement) == encryptedRootCompact(parts[1]) {
							critical[key]++
						}
					}
				}
				assignment, ok := statement.(*ast.AssignStmt)
				if !ok || len(assignment.Rhs) != 1 || len(assignment.Lhs) != 2 {
					continue
				}
				call, ok := assignment.Rhs[0].(*ast.CallExpr)
				if !ok {
					continue
				}
				slot := 0
				switch encryptedRootName(call.Fun) {
				case "sqlite.OpenContext":
					slot = 2
				case "owner.db.Query":
					slot = 3
				case "os.Open", "os.OpenFile":
					slot = 4
				}
				if slot == 0 {
					continue
				}
				valid := false
				if i+1 < len(node.List) {
					if next, ok := node.List[i+1].(*ast.AssignStmt); ok && len(next.Rhs) == 1 {
						if capture, ok := next.Rhs[0].(*ast.CallExpr); ok && encryptedRootName(capture.Fun) == "r.capture" && len(capture.Args) == 6 && encryptedRootName(capture.Args[slot]) == encryptedRootName(assignment.Lhs[0]) && encryptedRootName(capture.Args[5]) == encryptedRootName(assignment.Lhs[1]) {
							valid = true
						}
					}
				}
				if !valid {
					report("acquisition must immediately capture original result and owner")
				} else {
					pairs++
				}
			}
		}
		return true
	})
	for fn := range encryptedRootFunctions {
		if functions[fn] != 1 {
			report("missing or duplicate finite function: " + fn)
		}
	}
	for name, count := range map[string]int{"sqlite.OpenContext": 2, "seedDB.Exec": 2, "owner.db.Query": 1, "owner.db.VerifyEncrypted": 1, "owner.db.Check": 1, "r.ledger": 2, "os.MkdirTemp": 1, "os.Mkdir": 1, "os.OpenFile": 1, "os.Open": 2, "o.db.Close": 1, "o.rows.Close": 1, "o.file.Close": 1} {
		if calls[name] != count {
			report("effect count changed: " + name)
		}
	}
	if pairs != 6 {
		report("owner capture inventory changed")
	}
	for key := range encryptedRootCritical {
		if critical[key] != 1 {
			report("critical statement changed or duplicated: " + key)
		}
	}
	return findings, nil
}
func inspectEncryptedRootNegativeSource(path string, raw []byte) ([]string, error) {
	findings, err := inspectEncryptedRootNegativeStructure(path, raw)
	if err != nil {
		return findings, err
	}
	path = filepath.ToSlash(filepath.Clean(path))
	if pin, ok := encryptedRootPins[path]; ok && fmt.Sprintf("%x", sha256.Sum256(raw)) != pin {
		findings = append(findings, path+": encrypted root negative full-source review HOLD/mismatch")
	}
	return findings, nil
}
func inspectEncryptedRootNegativeContracts(root string) ([]string, error) {
	var findings []string
	for path := range encryptedRootPins {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return findings, err
		}
		got, err := inspectEncryptedRootNegativeSource(path, raw)
		if err != nil {
			return findings, err
		}
		findings = append(findings, got...)
	}
	return findings, nil
}

var encryptedRootFunctions = encryptedRootSet("Acquire Kind Bytes Release pending check capture closeOwner finish rootAuthOptions directory makeDirectory read snapshot writeCopy ledger rootAuthClass TestManualEncryptedRootNegative")
var encryptedRootEffects = encryptedRootSet("OpenContext Open OpenFile Query Exec Close Write Sync Mkdir MkdirTemp")
var encryptedRootCalls = encryptedRootSet(`
	p.mu.Lock p.mu.Unlock m.provider.mu.Lock m.provider.mu.Unlock r.seed.mu.Lock r.seed.mu.Unlock r.candidate.mu.Lock r.candidate.mu.Unlock
	sha256.Sum256 []byte append clear len errors.Is errors.As errors.New errors.Join r.pending r.t.Fatal r.t.Error r.t.Failed r.t.Log r.t.Logf
	r.closeOwner o.file.Close o.rows.Close o.db.Close o.state.Retire r.receipt.Validate json.Marshal bytes.Contains bytes.Equal bytes.Clone
	sqlite.PendingEncryptedCleanups os.Lstat os.Geteuid os.Open os.OpenFile os.Mkdir os.MkdirTemp os.Getenv os.Executable
	info.Sys info.IsDir info.Mode before.Sys before.Mode before.Size before.ModTime after.Size after.ModTime ().IsRegular ().Perm
	filepath.Join filepath.Base filepath.IsAbs filepath.Clean filepath.EvalSymlinks os.SameFile r.check r.capture file.Stat file.ReadDir file.Write file.Sync
	io.ReadAll io.LimitReader int64 uint64 uint32 int fmt.Sprintf r.read r.directory r.makeDirectory sort.Slice entries[].Name entry.Name
	owner.db.VerifyEncrypted owner.db.Check owner.db.Query rows.Next rows.Err rows.Values rowOwner.state.Retire reflect.DeepEqual
	encryptedroot.Classify encryptedroot.Select encryptedroot.Hash rootAuthOptions rootAuthClass r.finish r.snapshot r.writeCopy r.ledger
	flag.Lookup flag.Visit f.Value.String debug.ReadBuildInfo context.Background context.WithTimeout cancel seedDB.Exec sqlite.OpenContext
	encryption.DecodeHeader encryption.DecodeRecordHeader make t.Fatal t.Logf held.Sys
`)

const encryptedRootGate = `if os.Getenv("GOSQLITE_MANUAL_ENCRYPTED_ROOT_NEGATIVE") != "independently_authorized" { t.Fatal("manual encrypted root negative is not independently authorized") }`

var encryptedRootCritical = map[string]bool{
	`TestManualEncryptedRootNegative/plan, err := encryptedroot.Select(os.Getenv("GOSQLITE_MANUAL_ENCRYPTED_ROOT_CASE"))`:                                                                                     true,
	`TestManualEncryptedRootNegative/db, openErr := sqlite.OpenContext(ctx, filepath.Join(candidate, encryptedroot.RootName), sqlite.Options{Encryption:&candidateOptions, ReadOnly:true, NoCreate:true})`:    true,
	`TestManualEncryptedRootNegative/r.receipt.Class, r.receipt.Code, r.receipt.CausePreserved = rootAuthClass(openErr)`:                                                                                      true,
	`TestManualEncryptedRootNegative/r.receipt.OriginalBefore, originalBytes = r.snapshot(original)`:                                                                                                          true,
	`TestManualEncryptedRootNegative/r.receipt.CandidateBefore, candidateBytes = r.snapshot(candidate)`:                                                                                                       true,
	`TestManualEncryptedRootNegative/r.receipt.OriginalAfter, _ = r.snapshot(original)`:                                                                                                                       true,
	`TestManualEncryptedRootNegative/r.receipt.CandidateAfter, _ = r.snapshot(candidate)`:                                                                                                                     true,
	`TestManualEncryptedRootNegative/r.writeCopy(filepath.Join(candidate, member.Name), data)`:                                                                                                                true,
	`TestManualEncryptedRootNegative/if member.Name == encryptedroot.RootName && plan.Offset >= 0 { data[plan.Offset] ^= plan.XOR }`:                                                                          true,
	`TestManualEncryptedRootNegative/r.check(openErr != nil && db == nil && r.receipt.Class == plan.Class && r.receipt.Code == plan.Code && r.receipt.CausePreserved, "exact fail-closed public error", nil)`: true,
	`TestManualEncryptedRootNegative/if plan.Case == "control" {
		r.check(openErr == nil && db != nil, "correct key control open", openErr)
		r.receipt.QueryCalls++
		r.receipt.Control = r.ledger(ctx, candidateOwner)
	} else {
		r.check(openErr != nil && db == nil && r.receipt.Class == plan.Class && r.receipt.Code == plan.Code && r.receipt.CausePreserved, "exact fail-closed public error", nil)
	}`: true,
	`TestManualEncryptedRootNegative/r.check(r.receipt.ChangedBytes == wantedChanges && r.receipt.SaltBeforeNonzero && r.receipt.SaltAfterNonzero, "mutation complete before open", nil)`: true,
	`writeCopy/file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)`:                                                                                                   true,
	`ledger/want := [][]any{{int64(-7), "alpha"}, {int64(2), rootAuthMarker}, {int64(9), "omega"}}`:                                                                                       true,
	`ledger/r.check(result.ExactRows && result.EOF && result.RowsErrNil, "complete literal row ledger", nil)`:                                                                             true,
	`finish/if err := r.receipt.Validate(); err != nil { r.receipt.Status = "failed"; r.t.Error("finite receipt refused") }`:                                                              true,
}
