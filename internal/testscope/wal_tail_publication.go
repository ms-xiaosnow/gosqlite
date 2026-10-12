package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const walPublicationSQLiteImport = "github.com/ms-xiaosnow/gosqlite/sqlite"
const walPublicationManualPath = "sqlite/wal_tail_publication_manual_linux_test.go"
const walPublicationManualTag = "//go:build manual_wal_publication && linux"

// 此守卫独立于旧 recovery/namespace manual 排除规则；任何 build tag 都
// 不能绕过新的显式启用、私有桥接和 native factory 引用检查。它是有限源码
// 防线，不是类型检查或完整跨文件调用图证明；反射、外部 helper、init/import
// 闭包及新的执行选择仍须按固定源码独立审阅。
var walPublicationDeclarations = map[string]string{
	"WALTailPublicationOptions": "sqlite/wal_tail_publication.go",
	"validateWALTailPublication": "sqlite/wal_tail_publication.go",
	"snapshotWALTailPublication": "sqlite/wal_tail_publication.go",
	"OpenSessionTailPublicationAdmitted": "internal/pager/session.go",
	"openSessionAdmitted": "internal/pager/session.go",
	"publishWALTailLocked": "internal/pager/wal_tail_publication.go",
	"reserveWALPublication": "internal/pager/plain_wal_publication.go",
	"NewWALPublication": "internal/vfs/wal_publication.go",
	"newWALPublicationWithOps": "internal/vfs/wal_publication.go",
}

// 精确的真实 caller；函数值、别名、包装调用和新增 helper 不在白名单内。
var walPublicationCalls = map[string]map[string][]string{
	"sqlite/sqlite.go": {
		"OpenContext": {
			"snapshotWALTailPublication(opts)",
			"pager.OpenSessionTailPublicationAdmitted(ctx, path, d.pagerOptions(), plainAdmission)",
		},
	},
	"sqlite/wal_tail_publication.go": {
		"snapshotWALTailPublication": {
			"validateWALTailPublication(opts.WALTailPublication != nil, opts.ReadOnly, opts.WALExclusive, opts.WALShared, opts.Encryption != nil, opts.RollbackRecovery != nil, opts.WALPowersafeOverwrite, runtime.GOOS)",
		},
	},
	"internal/pager/session.go": {
		"OpenSessionAdmitted": {"openSessionAdmitted(ctx, path, opts, admission, false)"},
		"OpenSessionTailPublicationAdmitted": {"openSessionAdmitted(ctx, path, opts, admission, true)"},
		"openSessionAdmitted": {"s.publishWALTailLocked(ctx, pending, validated, plan)"},
	},
	"internal/pager/wal_tail_publication.go": {
		"publishWALTailLocked": {"s.cleanup.reserveWALPublication(s.file, s.walFile, s.info, s.walInfo)"},
	},
	"internal/pager/plain_wal_publication.go": {
		"reserveWALPublication": {"vfs.NewWALPublication(main, old, mainInfo, oldInfo)"},
	},
	"internal/vfs/wal_publication_model_test.go": {
		"newWALPublicationModel": {"newWALPublicationWithOps(m)"},
	},
	"internal/vfs/wal_publication.go": {
		"NewWALPublication": {
			"newNativeWALPublication(main, original, mainInfo, originalInfo)",
			"newWALPublicationWithOps(newNativeWALPublication(main, original, mainInfo, originalInfo))",
		},
	},
}

// 仅这些完整、固定的源可以构造 opt-in 配置；manual 源另有精确双门。
// inert VFS ops 的旧例外不变；新的 manual 文件不是惰性模型或执行许可。
// 路径本身从不形成豁免。更新哈希前必须审阅整个文件及依赖闭包。
var walPublicationFrozenSources = map[string]string{
	walPublicationManualPath: "2ee5023f1f844966e90f45d8e2220632895f25bd839cb53b27afe399e310ddd9",
	"internal/vfs/wal_publication_model_test.go": "7381288113bc424421bf8dade2f3a4e75bfd3b46f3201b3468762341a54ad9e7",
	"internal/testscope/main.go": "a1a4216646d16bf5331522d8a0b032a792f8676a520be2ae6d0101c3e31b4d42",
	"sqlite/wal_tail_publication.go": "a2e699dd0de40fadba140adc644fc41954516c425afc45fc509400c8342d001f",
	"sqlite/wal_tail_publication_test.go": "b4344960fc0be27f33f71c54717f41fa88b2574bf0c0fcdf7ee3289e58c351d8",
	"driver/wal_tail_publication_test.go": "b64ea77b48bff5c6592f5bb1fcac69d98c616821b3c1bced02cc5dcc70b58418",
}

// 完整生产边界固定启用条件、控制流与无 I/O factory 约束；词法 caller
// 白名单不能证明这些性质。此表只由完整合同检查读取，惰性片段检查仍可
// 使用独立的正反例。任何边界字节变化都必须先重新独立审阅。
var walPublicationProductionBoundaries = map[string]string{
	"sqlite/sqlite.go": "0ee79f959b53d61fee15dc9ae36618863163790c55b68a056f66acef188cba31",
	"driver/driver.go": "5f6df17c06316ac5090b935c0c6cd48cb730e00885e43cc87ce26d8b6efde03d",
	"internal/pager/session.go": "ed07757c219ecc243f090a05ae96535005d1519d8ddbb370389000119f874b5a",
	"internal/pager/wal.go": "48128a32116dbec6f5412ebc78ce72a9c740e3ff10153f635f1b1285463a9104",
	"internal/pager/wal_tail_publication.go": "46b91e2245e6266aed09e7cbfcf29e607905fff7f3cf71a31865a1eb5f18d396",
	"internal/pager/plain_wal_publication.go": "1d60fd981d20267b9ab2e878c92bd700901bf787d66ec6e1ac70da5b2dc520d7",
	"internal/vfs/wal_publication.go": "0c2233c987ea64813d646416f9af2c70f43cb89bc26ab28af5fc26d267ab627b",
	"internal/vfs/wal_publication_linux.go": "65a19e4d43eea50cd439957acd2492e5935d69710a41b71cb77d30a4060dfb10",
	"internal/vfs/wal_publication_other.go": "b18743a46b3a129e743bd5515800fc1444279fbec2a37ff822fd6476a149bd29",
	"internal/vfs/wal_publication_namespace.go": "cc9ad0c3e0b9b58faa3bd2ad38f8438cf5c3b73f643dc55396e769bf2e9b576a",
}

func walPublicationPureSource(path string) bool {
	return path == "sqlite/wal_tail_publication_test.go" || path == "driver/wal_tail_publication_test.go"
}

func walPublicationSourceText(set *token.FileSet, raw []byte, n ast.Node) string {
	return strings.Join(strings.Fields(oracleNodeText(set, raw, n)), " ")
}

func walPublicationDirectCall(id *ast.Ident, parents map[ast.Node]ast.Node) *ast.CallExpr {
	var target ast.Node = id
	if selector, ok := parents[id].(*ast.SelectorExpr); ok && selector.Sel == id {
		target = selector
	}
	call, ok := parents[target].(*ast.CallExpr)
	if !ok || call.Fun != target {
		return nil
	}
	return call
}

func walPublicationAllowedCall(path string, id *ast.Ident, parents map[ast.Node]ast.Node, set *token.FileSet, raw []byte) bool {
	call := walPublicationDirectCall(id, parents)
	if call == nil {
		return false
	}
	fn := oracleEnclosingFunction(call, parents)
	if fn == nil {
		return false
	}
	text := walPublicationSourceText(set, raw, call)
	for _, approved := range walPublicationCalls[path][fn.Name.Name] {
		if text == approved {
			return true
		}
	}
	return false
}

func walPublicationPolicyReference(path string, id *ast.Ident, parents map[ast.Node]ast.Node, set *token.FileSet, raw []byte) bool {
	if path == "sqlite/sqlite.go" {
		if field, ok := parents[id].(*ast.Field); ok && len(field.Names) == 1 && field.Names[0] == id {
			return walPublicationSourceText(set, raw, field) == "WALTailPublication *WALTailPublicationOptions"
		}
	}
	fn := oracleEnclosingFunction(id, parents)
	if fn == nil || !((path == "sqlite/sqlite.go" && fn.Name.Name == "OpenContext") ||
		(path == "sqlite/wal_tail_publication.go" && fn.Name.Name == "snapshotWALTailPublication") ||
		(path == "driver/driver.go" && fn.Name.Name == "NewConnector")) {
		return false
	}
	sel, ok := parents[id].(*ast.SelectorExpr)
	if !ok || sel.Sel != id || walPublicationSourceText(set, raw, sel) != "opts.WALTailPublication" {
		return false
	}
	switch use := parents[sel].(type) {
	case *ast.BinaryExpr:
		if use.Op != token.NEQ || use.X != sel || walPublicationSourceText(set, raw, use.Y) != "nil" {
			return false
		}
		if branch, ok := parents[use].(*ast.IfStmt); ok && branch.Cond == use {
			return true
		}
		call, ok := parents[use].(*ast.CallExpr)
		return path == "sqlite/wal_tail_publication.go" && ok && len(call.Args) != 0 && call.Args[0] == use
	case *ast.StarExpr:
		if path == "sqlite/sqlite.go" {
			return false
		}
		assignment, ok := parents[use].(*ast.AssignStmt)
		return ok && assignment.Tok == token.DEFINE && len(assignment.Lhs) == 1 && len(assignment.Rhs) == 1 &&
			assignment.Rhs[0] == use && walPublicationSourceText(set, raw, assignment.Lhs[0]) == "copy"
	case *ast.AssignStmt:
		return path != "sqlite/sqlite.go" && use.Tok == token.ASSIGN && len(use.Lhs) == 1 && len(use.Rhs) == 1 &&
			use.Lhs[0] == sel && walPublicationSourceText(set, raw, use.Rhs[0]) == "&copy"
	}
	return false
}

// 追踪同文件别名（含逆序定义和泛型包装），并识别 []Options/map 值中
// 隐式元素类型的非键控构造。无法证明归属的类型与跨文件数据流仍须审阅。
func walPublicationOptionLiterals(file *ast.File, imports map[string]string, parents map[ast.Node]ast.Node, report func(token.Pos, string)) {
	types := map[string]ast.Expr{}
	var declarations []*ast.TypeSpec
	ast.Inspect(file, func(n ast.Node) bool {
		if spec, ok := n.(*ast.TypeSpec); ok {
			types[spec.Name.Name] = spec.Type
			declarations = append(declarations, spec)
		}
		return true
	})
	localOptions := file.Name.Name == "sqlite" || imports["."] == walPublicationSQLiteImport
	optionNames := map[string]bool{"Options": localOptions}
	var optionReference func(ast.Expr) bool
	optionReference = func(expr ast.Expr) bool {
		switch typ := expr.(type) {
		case *ast.ParenExpr:
			return optionReference(typ.X)
		case *ast.StarExpr:
			return optionReference(typ.X)
		case *ast.IndexExpr:
			return optionReference(typ.X)
		case *ast.IndexListExpr:
			return optionReference(typ.X)
		case *ast.Ident:
			return optionNames[typ.Name]
		case *ast.SelectorExpr:
			if pkg, ok := typ.X.(*ast.Ident); ok {
				return imports[pkg.Name] == walPublicationSQLiteImport && typ.Sel.Name == "Options"
			}
		}
		return false
	}
	// 同名局部声明不能抹掉另一个作用域的 Options 来源；宁可要求复核。
	for changed := true; changed; {
		changed = false
		for _, spec := range declarations {
			if !optionNames[spec.Name.Name] && optionReference(spec.Type) {
				optionNames[spec.Name.Name], changed = true, true
			}
		}
	}
	var resolve func(ast.Expr, map[string]bool) ast.Expr
	resolve = func(expr ast.Expr, seen map[string]bool) ast.Expr {
		switch typ := expr.(type) {
		case *ast.ParenExpr:
			return resolve(typ.X, seen)
		case *ast.StarExpr:
			return resolve(typ.X, seen)
		case *ast.IndexExpr:
			return resolve(typ.X, seen)
		case *ast.IndexListExpr:
			return resolve(typ.X, seen)
		case *ast.Ident:
			if optionNames[typ.Name] {
				return typ
			}
			if value := types[typ.Name]; value != nil && !seen[typ.Name] {
				seen[typ.Name] = true
				return resolve(value, seen)
			}
		}
		return expr
	}
	isOptions := func(expr ast.Expr) bool {
		if optionReference(expr) {
			return true
		}
		expr = resolve(expr, map[string]bool{})
		if id, ok := expr.(*ast.Ident); ok {
			return id.Name == "Options" && localOptions
		}
		if sel, ok := expr.(*ast.SelectorExpr); ok {
			if pkg, ok := sel.X.(*ast.Ident); ok {
				return imports[pkg.Name] == walPublicationSQLiteImport && sel.Sel.Name == "Options"
			}
		}
		return false
	}
	var literalType func(*ast.CompositeLit) ast.Expr
	literalType = func(literal *ast.CompositeLit) ast.Expr {
		if literal.Type != nil {
			if optionReference(literal.Type) {
				return literal.Type
			}
			return resolve(literal.Type, map[string]bool{})
		}
		var element ast.Node = literal
		var fieldKey ast.Expr
		mapKey := false
		if pair, ok := parents[element].(*ast.KeyValueExpr); ok {
			mapKey = pair.Key == element
			fieldKey = pair.Key
			element = pair
		}
		outer, ok := parents[element].(*ast.CompositeLit)
		if !ok {
			return nil
		}
		switch typ := literalType(outer).(type) {
		case *ast.ArrayType:
			return resolve(typ.Elt, map[string]bool{})
		case *ast.MapType:
			if mapKey {
				return resolve(typ.Key, map[string]bool{})
			}
			return resolve(typ.Value, map[string]bool{})
		case *ast.StructType:
			if key, ok := fieldKey.(*ast.Ident); ok && !mapKey {
				for _, field := range typ.Fields.List {
					for _, name := range field.Names {
						if name.Name == key.Name {
							return resolve(field.Type, map[string]bool{})
						}
					}
				}
			}
		}
		return nil
	}
	ast.Inspect(file, func(n ast.Node) bool {
		literal, ok := n.(*ast.CompositeLit)
		if !ok || !isOptions(literalType(literal)) {
			return true
		}
		for _, element := range literal.Elts {
			if _, keyed := element.(*ast.KeyValueExpr); !keyed {
				report(element.Pos(), "unkeyed sqlite.Options can enable WAL tail publication")
				break
			}
		}
		return true
	})
}

func inspectWALPublicationSource(path string, raw []byte) ([]string, error) {
	path = filepath.ToSlash(filepath.Clean(path))
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, path, raw, 0)
	if err != nil {
		return nil, err
	}
	parents := oracleParents(file)
	var findings []string
	report := func(pos token.Pos, message string) {
		findings = append(findings, fmt.Sprintf("%s: WAL publication source contract: %s", set.Position(pos), message))
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
		if (name == "." || name == "_") && (value == walPublicationSQLiteImport ||
			value == "github.com/ms-xiaosnow/gosqlite/internal/pager" || value == "github.com/ms-xiaosnow/gosqlite/internal/vfs") {
			report(imp.Pos(), "dot/blank publication-related import requires explicit review")
		}
	}
	findings = append(findings, inspectWALPublicationFrozenSource(path, raw)...)
	// Manual admission is a full-source identity, never a tag/path exemption.
	// Only public option references are admitted below; all native/private
	// factories, aliases and the existing pure-source restrictions still run.
	manual := path == walPublicationManualPath && oracleSourceDigest(raw) == walPublicationFrozenSources[walPublicationManualPath] &&
		strings.SplitN(string(raw), "\n", 2)[0] == walPublicationManualTag
	pure := walPublicationPureSource(path)
	callCounts := map[string]int{}
	walPublicationOptionLiterals(file, imports, parents, report)
	ast.Inspect(file, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		if pure {
			// 不根据传入 opts 是变量、别名、nil 还是字面量推断安全；这些
			// 特许测试中的任何执行入口引用（包括函数值）一律拒绝。
			switch id.Name {
			case "Open", "OpenContext", "Connect", "OpenSession", "OpenSessionAdmitted", "OpenSessionTailPublicationAdmitted", "openSessionAdmitted", "publishWALTailLocked", "reserveWALPublication", "NewWALPublication", "newNativeWALPublication", "newWALPublicationWithOps", "init", "TestMain":
				report(id.Pos(), "inert policy/connector source cannot reference execution entry "+id.Name)
				return true
			}
		}
		// Native ops/lease 类型与辅助函数不得经类型别名或方法值逃到测试或
		// 新包装器；只有对应两个 backend 文件可以引用其私有实现。
		if strings.HasPrefix(id.Name, "nativeWALPublication") || id.Name == "walPublicationLease" || id.Name == "newWALPublicationLease" || id.Name == "unsupportedWALPublication" {
			if path != "internal/vfs/wal_publication_linux.go" && path != "internal/vfs/wal_publication_other.go" {
				report(id.Pos(), "native implementation type/helper escaped its backend")
			}
			return true
		}
		if id.Name == "WALTailPublication" {
			if !pure && !manual && !walPublicationPolicyReference(path, id, parents, set, raw) {
				report(id.Pos(), "opt-in field outside exact public policy or inert model")
			}
			return true
		}
		declarationPath, protected := walPublicationDeclarations[id.Name]
		if id.Name == "newNativeWALPublication" {
			protected = true
			if path == "internal/vfs/wal_publication_linux.go" || path == "internal/vfs/wal_publication_other.go" {
				declarationPath = path
			}
		}
		if !protected {
			return true
		}
		if manual && id.Name == "WALTailPublicationOptions" { return true }
		if pure && (id.Name == "WALTailPublicationOptions" || id.Name == "validateWALTailPublication" || id.Name == "snapshotWALTailPublication") {
			return true
		}
		if fn, ok := parents[id].(*ast.FuncDecl); ok && fn.Name == id && declarationPath == path {
			return true
		}
		if typ, ok := parents[id].(*ast.TypeSpec); ok && typ.Name == id && declarationPath == path {
			return true
		}
		if id.Name == "WALTailPublicationOptions" && path == "sqlite/sqlite.go" {
			if pointer, ok := parents[id].(*ast.StarExpr); ok {
				if field, ok := parents[pointer].(*ast.Field); ok && len(field.Names) == 1 && field.Names[0].Name == "WALTailPublication" {
					return true
				}
			}
		}
		if !walPublicationAllowedCall(path, id, parents, set, raw) {
			report(id.Pos(), "restricted activation/factory reference "+id.Name)
		} else {
			call := walPublicationDirectCall(id, parents)
			fn := oracleEnclosingFunction(call, parents)
			key := fn.Name.Name + ":" + walPublicationSourceText(set, raw, call)
			callCounts[key]++
			if callCounts[key] != 1 {
				report(id.Pos(), "duplicate protected production call")
			}
		}
		return true
	})
	return findings, nil
}

func inspectWALPublicationFrozenSource(path string, raw []byte) []string {
	if want, exists := walPublicationFrozenSources[path]; exists && oracleSourceDigest(raw) != want {
		return []string{fmt.Sprintf("%s: WAL publication fixed-source boundary changed; independent source review is required", path)}
	}
	return nil
}

func inspectWALPublicationProductionSource(path string, raw []byte) []string {
	if want, exists := walPublicationProductionBoundaries[path]; exists && oracleSourceDigest(raw) != want {
		return []string{fmt.Sprintf("%s: WAL publication production boundary changed; independent source review is required before ordinary selection", path)}
	}
	return nil
}

func inspectWALPublicationContracts(root string) ([]string, error) {
	var findings []string
	for path := range walPublicationFrozenSources {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return nil, err
		}
		findings = append(findings, inspectWALPublicationFrozenSource(path, raw)...)
	}
	for path := range walPublicationProductionBoundaries {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			return nil, err
		}
		findings = append(findings, inspectWALPublicationProductionSource(path, raw)...)
	}
	return findings, nil
}
