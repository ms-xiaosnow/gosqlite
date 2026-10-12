// Command testscope checks high-risk references in ordinary test sources.
// This is a prevention aid, not a substitute for reviewing test behavior.
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Exact SQL and single-page recovery exceptions, plus the narrow scalar Signal
// forwarding, require independent full-function and whole-main source review.
func inspectSource(path string, raw []byte) ([]string, error) {
	first := strings.SplitN(string(raw), "\n", 2)[0]
	// Exact approved constraints only: a prefix could hide "manual_namespace || linux".
	if first == "//go:build manual_recovery && linux" || first == "//go:build manual_namespace" {
		return nil, nil
	}
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, path, raw, 0)
	if err != nil {
		return nil, err
	}
	imports := map[string]string{}
	sqliteDotImported := false
	var findings []string
	for _, i := range file.Imports {
		p, e := strconv.Unquote(i.Path.Value)
		if e != nil {
			return nil, e
		}
		name := filepath.Base(p)
		if i.Name != nil {
			name = i.Name.Name
		}
		imports[name] = p
		if name == "." && p == "github.com/ms-xiaosnow/gosqlite/sqlite" {
			sqliteDotImported = true
		}
		if name == "." && (p == "os" || p == "os/exec" || p == "syscall" || p == "golang.org/x/sys/unix") {
			findings = append(findings, fmt.Sprintf("%s: high-risk dot import requires review", set.Position(i.Pos())))
		}
	}
	// Conservatively follow same-file type names (including aliases), not Go
	// scope/type identity or cross-file data flow. Cycles cannot make progress.
	// Behavioral review is still required for helper calls, reflection, implicit
	// element types and declarations in other files.
	optionTypes := map[string]bool{}
	if file.Name.Name == "sqlite" || sqliteDotImported {
		optionTypes["Options"] = true
	}
	var isOptionType func(ast.Expr) bool
	isOptionType = func(expr ast.Expr) bool {
		switch typ := expr.(type) {
		case *ast.ParenExpr:
			return isOptionType(typ.X)
		case *ast.IndexExpr:
			return isOptionType(typ.X)
		case *ast.IndexListExpr:
			return isOptionType(typ.X)
		case *ast.Ident:
			return optionTypes[typ.Name]
		case *ast.SelectorExpr:
			if qualifier, ok := typ.X.(*ast.Ident); ok {
				return imports[qualifier.Name] == "github.com/ms-xiaosnow/gosqlite/sqlite" && typ.Sel.Name == "Options"
			}
		}
		return false
	}
	var declarations []*ast.TypeSpec
	ast.Inspect(file, func(n ast.Node) bool {
		if spec, ok := n.(*ast.TypeSpec); ok {
			declarations = append(declarations, spec)
		}
		return true
	})
	for changed := true; changed; {
		changed = false
		for _, spec := range declarations {
			if !optionTypes[spec.Name.Name] && isOptionType(spec.Type) {
				optionTypes[spec.Name.Name] = true
				changed = true
			}
		}
	}
	report := func(pos token.Pos, name string) {
		findings = append(findings, fmt.Sprintf("%s: restricted reference %s requires separate manual source and review", set.Position(pos), name))
	}
	parents := oracleParents(file)
	ast.Inspect(file, func(n ast.Node) bool {
		// Includes function values and opt-in field keys, not only direct calls.
		// Recovery-enabled public opens are source/model-only in ordinary suites.
		if id, ok := n.(*ast.Ident); ok {
			switch id.Name {
			case "RecoverRollback", "recoverRollbackOpen", "RollbackRecovery":
				if !rollbackFaultSQLRecoveryReference(path, id, parents, set, raw) && !rollbackFaultRecoveryAllowedRecover(path, id, parents, set, raw) {
					report(id.Pos(), id.Name)
				}
			}
		}
		if literal, ok := n.(*ast.CompositeLit); ok && len(literal.Elts) != 0 {
			if isOptionType(literal.Type) {
				for _, elt := range literal.Elts {
					if _, keyed := elt.(*ast.KeyValueExpr); !keyed {
						report(elt.Pos(), "unkeyed sqlite.Options may enable recovery")
					}
				}
			}
		}
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		prohibited := false
		if x, ok := sel.X.(*ast.Ident); ok {
			p := imports[x.Name]
			if p == "os" {
				switch sel.Sel.Name {
				case "Rename", "Symlink", "Link", "Chmod", "Remove", "RemoveAll", "Truncate", "Exit":
					prohibited = true
				}
			}
			if p == "os/exec" && sel.Sel.Name == "CommandContext" {
				prohibited = true
			}
			if p == "syscall" || p == "golang.org/x/sys/unix" {
				switch sel.Sel.Name {
				case "Kill", "Rename", "Renameat", "Renameat2", "Unlink", "Unlinkat", "Truncate", "Ftruncate", "Chmod", "Fchmod":
					prohibited = true
				}
			}
		}
		switch sel.Sel.Name {
		case "Kill", "Signal", "Truncate", "Chmod", "RecoverRollback", "Rename", "rename":
			prohibited = true
		}
		if prohibited && !rollbackFaultSQLRecoveryReference(path, sel.Sel, parents, set, raw) &&
			!(path == "internal/testkit/rollbackrestart/contract_test.go" && sel.Sel.Name == "Signal" && restartGuardSignalField(path, sel, parents, set, raw, file)) {
			report(sel.Pos(), sel.Sel.Name)
		}
		return true
	})
	return findings, nil
}
func main() {
	if strings.Contains(os.Getenv("GOFLAGS"), "manual_") {
		fmt.Fprintln(os.Stderr, "manual test tags are forbidden in ordinary checks")
		os.Exit(1)
	}
	failed := false
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var findings []string
		formatUpgrade, formatErr := inspectFormatUpgradeSource(path, raw)
		if formatErr != nil {
			return formatErr
		}
		findings = append(findings, formatUpgrade...)
		// All Go sources are checked for copied manual entries/import consumers;
		// only the four exact files receive the additional frozen-source policy.
		manual, err := inspectBackupManualSource(path, raw)
		if err != nil {
			return err
		}
		findings = append(findings, manual...)
		// The rollback adapter boundary ignores build-tag exclusions and has
		// no native caller exemption. Older guards remain independent.
		rollbackFault, err := inspectRollbackFaultSource(path, raw)
		if err != nil {
			return err
		}
		findings = append(findings, rollbackFault...)
		rollbackSQL, rollbackSQLErr := inspectRollbackFaultSQLSource(path, raw)
		if rollbackSQLErr != nil {
			return rollbackSQLErr
		}
		findings = append(findings, rollbackSQL...)
		restart, restartErr := inspectRollbackRestartSource(path, raw)
		if restartErr != nil {
			return restartErr
		}
		findings = append(findings, restart...)
		recovery, err := inspectRollbackFaultRecoverySource(path, raw)
		if err != nil {
			return err
		}
		findings = append(findings, recovery...)
		readonlyManual, err := inspectReadOnlyWALManualSource(path, raw)
		if err != nil {
			return err
		}
		findings = append(findings, readonlyManual...)
		// Keep the existing restricted-reference policy and its exact manual
		// exclusions separate from startup isolation, which ignores build tags.
		if strings.HasSuffix(path, "_test.go") {
			ordinary, inspectErr := inspectSource(path, raw)
			if inspectErr != nil {
				return inspectErr
			}
			findings = append(findings, ordinary...)
		}
		if oracleCurrentGoPath(path) {
			// WAL opt-in 守卫没有 manual-tag 排除；惰性模型例外固定完整
			// 源码身份，文件名本身不授予调用 native 入口的权限。
			publication, err := inspectWALPublicationSource(path, raw)
			if err != nil {
				return err
			}
			findings = append(findings, publication...)
			startup, err := inspectOracleSource(path, raw)
			if err != nil {
				return err
			}
			findings = append(findings, startup...)
		}
		for _, f := range findings {
			fmt.Fprintln(os.Stderr, f)
			failed = true
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		failed = true
	}
	contracts, err := inspectOracleContracts(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		failed = true
	}
	for _, finding := range contracts {
		fmt.Fprintln(os.Stderr, finding)
		failed = true
	}
	manualContracts, manualErr := inspectBackupManualContracts(".")
	if manualErr != nil {
		fmt.Fprintln(os.Stderr, manualErr)
		failed = true
	}
	for _, finding := range manualContracts {
		fmt.Fprintln(os.Stderr, finding)
		failed = true
	}
	rollbackFaultContracts, rollbackFaultErr := inspectRollbackFaultContracts(".")
	if rollbackFaultErr != nil {
		fmt.Fprintln(os.Stderr, rollbackFaultErr)
		failed = true
	}
	for _, finding := range rollbackFaultContracts {
		fmt.Fprintln(os.Stderr, finding)
		failed = true
	}
	rollbackSQLContracts, rollbackSQLContractErr := inspectRollbackFaultSQLContracts(".")
	if rollbackSQLContractErr != nil {
		fmt.Fprintln(os.Stderr, rollbackSQLContractErr)
		failed = true
	}
	for _, finding := range rollbackSQLContracts {
		fmt.Fprintln(os.Stderr, finding)
		failed = true
	}
	restartContracts, restartContractErr := inspectRollbackRestartContracts(".")
	if restartContractErr != nil {
		fmt.Fprintln(os.Stderr, restartContractErr)
		failed = true
	}
	for _, finding := range restartContracts {
		fmt.Fprintln(os.Stderr, finding)
		failed = true
	}
	rollbackRecoveryContracts, rollbackRecoveryErr := inspectRollbackFaultRecoveryContracts(".")
	if rollbackRecoveryErr != nil {
		fmt.Fprintln(os.Stderr, rollbackRecoveryErr)
		failed = true
	}
	for _, finding := range rollbackRecoveryContracts {
		fmt.Fprintln(os.Stderr, finding)
		failed = true
	}
	publicationContracts, err := inspectWALPublicationContracts(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		failed = true
	}
	for _, finding := range publicationContracts {
		fmt.Fprintln(os.Stderr, finding)
		failed = true
	}
	// A separate exact-path, source-pinned manual contract. This does not add
	// an exemption to inspectSource or broaden any existing manual gate.
	darwinManual, err := inspectDarwinInventoryManualContract(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		failed = true
	}
	for _, finding := range darwinManual {
		fmt.Fprintln(os.Stderr, finding)
		failed = true
	}
	formatContracts, formatErr := inspectFormatUpgradeContracts(".")
	if formatErr != nil {
		fmt.Fprintln(os.Stderr, formatErr)
		failed = true
	}
	for _, finding := range formatContracts {
		fmt.Fprintln(os.Stderr, finding)
		failed = true
	}
	readonlyContracts, err := inspectReadOnlyWALManualContracts(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		failed = true
	}
	for _, finding := range readonlyContracts {
		fmt.Fprintln(os.Stderr, finding)
		failed = true
	}
	if failed {
		os.Exit(1)
	}
	fmt.Println("Ordinary test, WAL publication, oracle startup and exact Backup manual source-reference guards PASS; behavioral review is still required.")
}
