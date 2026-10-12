package vfs

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// Source-only guards: ordinary source reads and AST construction, never native
// acquisition, a database, a compiler subprocess, an oracle or fault injection.
// The accepted shape is intentionally narrow: error gates must be direct
// sequential statements whose error arm returns before the protected action.
func backupASTText(n ast.Node) string {
	var b bytes.Buffer
	if n == nil { return "" }
	_ = format.Node(&b, token.NewFileSet(), n)
	return b.String()
}
func backupASTExpression(text string) string {
	expr, err := parser.ParseExpr(text)
	if err != nil { return "invalid expression: " + text }
	return backupASTText(expr)
}
func backupASTStatement(text string) string {
	file, err := parser.ParseFile(token.NewFileSet(), "expected.go", "package vfs; func expected(){"+text+"}", 0)
	if err != nil { return "invalid statement: " + text }
	return backupASTText(file.Decls[0].(*ast.FuncDecl).Body.List[0])
}
func backupASTFunction(f *ast.File, name, receiver string) *ast.FuncDecl {
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != name { continue }
		if receiver == "" && fn.Recv == nil { return fn }
		if fn.Recv != nil && len(fn.Recv.List) == 1 && backupASTText(fn.Recv.List[0].Type) == receiver { return fn }
	}
	return nil
}
func backupASTCalls(n ast.Node, name string) []*ast.CallExpr {
	var found []*ast.CallExpr
	ast.Inspect(n, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok && backupASTText(call.Fun) == name { found = append(found, call) }
		return true
	})
	return found
}
func backupASTArguments(fn *ast.FuncDecl, name string, args ...string) error {
	if fn == nil { return fmt.Errorf("missing function for %s", name) }
	calls := backupASTCalls(fn, name)
	if len(calls) != 1 || len(calls[0].Args) != len(args) { return fmt.Errorf("wrong %s call count/arity", name) }
	for i, arg := range args { if backupASTText(calls[0].Args[i]) != backupASTExpression(arg) { return fmt.Errorf("wrong %s argument %d", name, i) } }
	return nil
}
func backupASTReturnGate(stmt ast.Stmt) bool {
	gate, ok := stmt.(*ast.IfStmt)
	if !ok || gate.Else != nil || len(gate.Body.List) < 1 || len(gate.Body.List) > 2 { return false }
	if len(gate.Body.List) == 2 && backupASTText(gate.Body.List[0]) != "p.failed = true" { return false }
	_, ok = gate.Body.List[len(gate.Body.List)-1].(*ast.ReturnStmt)
	return ok
}
func backupASTGuard(fn *ast.FuncDecl, condition string, before string) error {
	if fn == nil { return fmt.Errorf("missing guarded function") }
	gate := -1
	for i, stmt := range fn.Body.List {
		if branch, ok := stmt.(*ast.IfStmt); ok && backupASTText(branch.Cond) == backupASTExpression(condition) && backupASTReturnGate(stmt) { gate = i }
		if len(backupASTCalls(stmt, before)) != 0 {
			if gate < 0 || gate >= i { return fmt.Errorf("%s does not dominate %s", condition, before) }
			return nil
		}
	}
	return fmt.Errorf("missing protected call %s", before)
}
func backupASTErrorGate(fn *ast.FuncDecl, callName, before string) error {
	if fn == nil { return fmt.Errorf("missing error-gated function") }
	gate := -1
	for i, stmt := range fn.Body.List {
		if branch, ok := stmt.(*ast.IfStmt); ok && branch.Init != nil && backupASTText(branch.Cond) == "err != nil" && backupASTReturnGate(stmt) && len(backupASTCalls(branch.Init, callName)) == 1 { gate = i }
		if len(backupASTCalls(stmt, before)) != 0 {
			if gate < 0 || gate >= i { return fmt.Errorf("error result of %s does not gate %s", callName, before) }
			return nil
		}
	}
	return fmt.Errorf("missing protected call %s", before)
}
func backupASTSequence(fn *ast.FuncDecl, statements ...string) error {
	if fn == nil { return fmt.Errorf("missing sequential function") }
	index := 0
	for _, stmt := range fn.Body.List {
		if index < len(statements) && backupASTText(stmt) == backupASTStatement(statements[index]) { index++ }
	}
	if index != len(statements) { return fmt.Errorf("missing sequential statement %q", statements[index]) }
	return nil
}
// These two admission boundaries deliberately accept only the reviewed direct
// statement shape. A subsequence or a helper-body check cannot exclude an early
// return/create, a conditional gate, an overwritten observation, or a discarded
// error. Matching every canonical statement preserves the actual input/result
// wiring and returned-error dominance; harmless refactors require fresh review.
// This is a source-shape guard, not a general control-flow or native proof.
func backupASTExactFlow(fn *ast.FuncDecl, statements ...string) error {
	if fn == nil || fn.Body == nil { return fmt.Errorf("missing exact-flow function") }
	if len(fn.Body.List) != len(statements) { return fmt.Errorf("changed %s admission statement count", fn.Name.Name) }
	for i, stmt := range fn.Body.List {
		if backupASTText(stmt) != backupASTStatement(statements[i]) { return fmt.Errorf("changed %s admission statement %d", fn.Name.Name, i) }
	}
	return nil
}
func backupASTFindValue(f *ast.File, name string) ast.Expr {
	for _, decl := range f.Decls {
		g, ok := decl.(*ast.GenDecl); if !ok { continue }
		for _, spec := range g.Specs {
			v, ok := spec.(*ast.ValueSpec); if !ok { continue }
			for i, n := range v.Names { if n.Name == name && i < len(v.Values) { return v.Values[i] } }
		}
	}
	return nil
}
func backupASTFields(fn *ast.FuncDecl, variable, typ string, wanted map[string]string) error {
	for _, stmt := range fn.Body.List {
		assignment, ok := stmt.(*ast.AssignStmt)
		if !ok || len(assignment.Lhs) != 1 || backupASTText(assignment.Lhs[0]) != variable || len(assignment.Rhs) != 1 { continue }
		literal, ok := assignment.Rhs[0].(*ast.CompositeLit)
		if !ok || backupASTText(literal.Type) != typ || len(literal.Elts) != len(wanted) { return fmt.Errorf("invalid %s construction", variable) }
		for _, elt := range literal.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok || backupASTExpression(wanted[backupASTText(kv.Key)]) != backupASTText(kv.Value) { return fmt.Errorf("invalid %s field", variable) }
		}
		return nil
	}
	return fmt.Errorf("missing %s", variable)
}
func backupASTValidate(source map[string]string) error {
	files := make(map[string]*ast.File)
	for name, body := range source {
		f, err := parser.ParseFile(token.NewFileSet(), name, body, 0)
		if err != nil { return err }; files[name] = f
	}
	fn := func(file, name, receiver string) *ast.FuncDecl { return backupASTFunction(files[file], name, receiver) }
	ns := "backup_namespace_linux.go"; native := "backup_publication_linux.go"; state := "backup_publication.go"
	if backupASTText(backupASTFindValue(files[ns], "nativeBackupResolve")) != backupASTExpression("unix.RESOLVE_BENEATH | unix.RESOLVE_NO_SYMLINKS | unix.RESOLVE_NO_MAGICLINKS | unix.RESOLVE_NO_XDEV") { return fmt.Errorf("resolve flags changed") }
	if backupASTText(backupASTFindValue(files[ns], "nativeBackupStatxMask")) != backupASTExpression("unix.STATX_TYPE | unix.STATX_NLINK | unix.STATX_INO | unix.STATX_SIZE | unix.STATX_MNT_ID_UNIQUE") { return fmt.Errorf("statx requested masks changed") }
	create := fn(native, "createCandidate", "*nativeBackupPublication")
	if err := backupASTArguments(create, "unix.Openat2", "int(n.dir.file.raw.Fd())", "base", "&how"); err != nil { return err }
	if err := backupASTFields(create, "how", "unix.OpenHow", map[string]string{"Flags": "unix.O_RDWR | unix.O_CREAT | unix.O_EXCL | unix.O_NOFOLLOW | unix.O_CLOEXEC | unix.O_NONBLOCK", "Mode": "0600", "Resolve": "nativeBackupResolve"}); err != nil { return err }
	if err := backupASTSequence(create, "n.main.attempted = true", "n.childUsesDirectory = true", "n.main.fd, err = unix.Openat2(int(n.dir.file.raw.Fd()), base, &how)"); err != nil { return err }
	if err := backupASTErrorGate(create, "nativeBackupAttach", "n.main.file.raw.Stat"); err != nil { return err }
	if err := backupASTArguments(create, "nativeBackupAttach", "&n.main", "filepath.Join(n.dir.file.path, base)"); err != nil { return err }
	attach := fn(ns, "nativeBackupAttach", "")
	if err := backupASTSequence(attach, "l.file.raw = os.NewFile(uintptr(l.fd), name)", "owners.files[l.file] = struct{}{}", "l.fd = -1"); err != nil { return err }
	open := fn(ns, "nativeBackupOpenDirectory", "")
	if err := backupASTArguments(open, "unix.Open", "path", "unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC | unix.O_NOFOLLOW | unix.O_NONBLOCK", "0"); err != nil { return err }
	if err := backupASTErrorGate(open, "nativeBackupAttach", "l.file.raw.Stat"); err != nil { return err }
	publish := fn(native, "publish", "*nativeBackupPublication")
	if err := backupASTArguments(publish, "unix.Renameat2", "int(n.dir.file.raw.Fd())", "candidate", "int(n.dir.file.raw.Fd())", "final", "unix.RENAME_NOREPLACE"); err != nil { return err }
	if err := backupASTGuard(publish, "n.sealed || !n.childUsesDirectory || !n.main.file.exclusive || candidate != n.name || !BackupBasename(final)", "unix.Renameat2"); err != nil { return err }
	stat := fn(ns, "nativeBackupStatx", "")
	if err := backupASTArguments(stat, "unix.Statx", "fd", "name", "flags", "nativeBackupStatxMask", "&st"); err != nil { return err }
	foundMask := false
	for _, stmt := range stat.Body.List {
		if gate, ok := stmt.(*ast.IfStmt); ok && backupASTText(gate.Cond) == backupASTExpression("st.Mask&nativeBackupStatxMask != nativeBackupStatxMask") && backupASTReturnGate(stmt) { foundMask = true }
	}
	if !foundMask { return fmt.Errorf("lost statx returned-mask rejection") }
	if err := backupASTArguments(fn(ns, "nativeBackupHeldFact", ""), "nativeBackupStatx", "int(f.raw.Fd())", `""`, "unix.AT_EMPTY_PATH|unix.AT_SYMLINK_NOFOLLOW"); err != nil { return err }
	if err := backupASTArguments(fn(ns, "nativeBackupNamedFact", ""), "nativeBackupStatx", "int(dir.file.raw.Fd())", "base", "unix.AT_SYMLINK_NOFOLLOW"); err != nil { return err }
	if len(backupASTCalls(fn(ns, "nativeBackupObserveSource", ""), "nativeBackupOpenDirectory")) != 0 { return fmt.Errorf("source main reopened") }
	// Parent, held main and named main must be separate successful observations
	// of the actual retained inputs before the direct-child error gates all later
	// identity/sidecar work and the sole successful parent result.
	if err := backupASTExactFlow(fn(ns, "nativeBackupObserveSource", ""),
		"if err := nativeBackupContinuity(dir); err != nil { return backupFact{}, err }",
		"parent, err := nativeBackupHeldFact(dir.file)",
		"if err != nil { return backupFact{}, err }",
		"held, err := nativeBackupHeldFact(main)",
		"if err != nil { return backupFact{}, err }",
		"named, err := nativeBackupNamedFact(dir, base)",
		"if err != nil { return backupFact{}, err }",
		"if err = backupDirectChild(parent, held, named); err != nil { return backupFact{}, err }",
		"info, err := main.raw.Stat()",
		"if err != nil { return backupFact{}, err }",
		"if !SameFile(info, main.identity) || !SameFile(dir.file.identity, main.parent) { return backupFact{}, ErrIdentity }",
		`for _, suffix := range []string{"-journal", "-wal", "-shm"} {
			fact, err := nativeBackupNamedFact(dir, base+suffix)
			if errors.Is(err, syscall.ENOENT) { continue }
			if err != nil { return backupFact{}, err }
			if err = backupSourceSidecar(parent, suffix, fact); err != nil { return backupFact{}, err }
		}`,
		"return parent, nil",
	); err != nil { return err }
	// No candidate creation is accepted in acquisition. The actual held-parent
	// result must reach the admission gate before either final/candidate family
	// probe or success; every acquisition/admission failure poisons p via p.fail.
	if err := backupASTExactFlow(fn(state, "openDestinationLocked", "*BackupPublication"),
		"if err := p.active(); err != nil { return err }",
		"if p.opened || !source.valid || !BackupBasename(final) || !BackupBasename(candidate) || size <= 0 { return p.fail(ErrUnsupported) }",
		"p.opened = true",
		"p.final, p.candidate, p.expected = final, candidate, size",
		"parentIdentity, err := p.ops.openDirectory(parent)",
		"if err != nil { return p.fail(err) }",
		"if err = backupDestinationAdmission(source, parentIdentity, p.final, p.candidate); err != nil { return p.fail(err) }",
		"if err = p.ops.absentFamily(p.final); err != nil { return p.fail(err) }",
		"if err = p.ops.absentFamily(p.candidate); err != nil { return p.fail(err) }",
		"return nil",
	); err != nil { return err }
	p := fn(state, "PublishNoReplace", "*BackupPublication")
	if err := backupASTGuard(p, "!p.status.CandidateLocked || !p.status.CopyComplete || !p.status.DataSynced || p.status.PublicationAttempted", "p.ops.publish"); err != nil { return err }
	if err := backupASTErrorGate(p, "p.ops.validateCandidate", "p.ops.publish"); err != nil { return err }
	if err := backupASTErrorGate(p, "p.ops.absentFamily", "p.ops.publish"); err != nil { return err }
	if err := backupASTArguments(p, "p.ops.publish", "p.candidate", "p.final"); err != nil { return err }
	if err := backupASTErrorGate(fn(state, "CreatePrivateCandidate", "*BackupPublication"), "p.ops.createCandidate", "p.ops.lockCandidate"); err != nil { return err }
	if err := backupASTGuard(fn(state, "WritePage", "*BackupPublication"), "!p.status.CandidateLocked || p.status.DataSyncAttempted || len(page) == 0 || offset != p.written || offset < 0 || int64(len(page)) > p.expected-offset", "p.ops.writeCandidate"); err != nil { return err }
	if err := backupASTGuard(fn(state, "SyncData", "*BackupPublication"), "!p.status.CandidateLocked || !p.status.CopyComplete || p.status.DataSyncAttempted", "p.ops.syncCandidate"); err != nil { return err }
	close := fn(state, "Close", "*BackupPublication")
	if err := backupASTErrorGate(close, "p.sealLocked", "p.ops.closeDirectory"); err != nil { return err }
	if err := backupASTGuard(close, "p.terminal || !p.ops.directoryComplete()", "p.ops.closeMain"); err != nil { return err }
	if err := backupASTGuard(fn(native, "closeMain", "*nativeBackupPublication"), "!n.sealed || n.childUsesDirectory || !n.dir.complete()", "n.main.close"); err != nil { return err }
	seal := fn(native, "seal", "*nativeBackupPublication")
	if err := backupASTSequence(seal, "n.sealed = true", "n.main.seal()", "n.dir.seal()", "n.childUsesDirectory = false"); err != nil { return err }
	for _, name := range []string{"n.main.close", "n.dir.close", "n.main.file.Close", "n.main.file.DowngradeDBToShared", "n.dir.file.Sync"} {
		if len(backupASTCalls(seal, name)) != 0 { return fmt.Errorf("seal performs native retirement/I/O") }
	}
	for _, node := range []*ast.FuncDecl{close, fn(state, "sealLocked", "*BackupPublication"), fn(native, "closeDirectory", "*nativeBackupPublication"), fn(native, "closeMain", "*nativeBackupPublication")} {
		for _, name := range []string{"p.ops.publish", "p.ops.syncDirectory", "p.ops.syncCandidate", "p.ops.writeCandidate", "p.ops.createCandidate", "unix.Renameat2", "unix.Openat2"} {
			if len(backupASTCalls(node, name)) != 0 { return fmt.Errorf("operation replay in retirement: %s", name) }
		}
	}
	admit := fn(state, "backupDestinationAdmission", "")
	if err := backupASTGuard(admit, "!source.valid || !source.parent.complete() || !parent.complete() || parent.mode&backupTypeMask != backupDirectory", "source.parent.samePhysical"); err != nil { return err }
	for name, file := range files {
		if strings.HasSuffix(name, "_test.go") { continue }
		var banned error
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr); if !ok { return true }
			if s, ok := call.Fun.(*ast.SelectorExpr); ok {
				switch s.Sel.Name {
				case "Remove", "RemoveAll", "Truncate", "Rename", "Renameat", "Link", "Linkat", "Unlink", "Unlinkat": banned = fmt.Errorf("unsafe mutation in %s", name)
				}
			}
			return true
		})
		if banned != nil { return banned }
	}
	return nil
}
func backupASTInputs(t *testing.T) map[string]string {
	t.Helper()
	out := make(map[string]string)
	for _, name := range []string{"backup_namespace.go", "backup_namespace_linux.go", "backup_namespace_other.go", "backup_publication.go", "backup_publication_linux.go", "backup_publication_other.go"} {
		body, err := os.ReadFile(name); if err != nil { t.Fatal(err) }; out[name] = string(body)
	}
	return out
}
func TestBackupPublicationSourceCapabilityFlow(t *testing.T) {
	if err := backupASTValidate(backupASTInputs(t)); err != nil { t.Fatal(err) }
}
func TestBackupPublicationSourceMutationCounterexamples(t *testing.T) {
	original := backupASTInputs(t)
	if err := backupASTValidate(original); err != nil { t.Fatal(err) }
	const destinationGate = "if err = backupDestinationAdmission(source, parentIdentity, p.final, p.candidate); err != nil { return p.fail(err) }"
	const destinationParent = "parentIdentity, err := p.ops.openDirectory(parent)\n\tif err != nil { return p.fail(err) }"
	const destinationFinal = "if err = p.ops.absentFamily(p.final); err != nil { return p.fail(err) }"
	const destinationCandidate = "if err = p.ops.absentFamily(p.candidate); err != nil { return p.fail(err) }"
	const sourceGate = "if err = backupDirectChild(parent, held, named); err != nil { return backupFact{}, err }"
	const sourceParent = "parent, err := nativeBackupHeldFact(dir.file)\n\tif err != nil { return backupFact{}, err }"
	const sourceHeld = "held, err := nativeBackupHeldFact(main)\n\tif err != nil { return backupFact{}, err }"
	const sourceNamed = "named, err := nativeBackupNamedFact(dir, base)\n\tif err != nil { return backupFact{}, err }"
	const sourceStat = "info, err := main.raw.Stat()\n\tif err != nil { return backupFact{}, err }"
	// These are inert source-string variants, never native calls. Their syntax
	// must parse before rejection is credited to the source guard. Helpers remain
	// unchanged in the call-site mutations, reproducing R-1's missing-link cases.
	for _, mutation := range []struct{name, file, before, after string}{
		{"lose-owned-return-slot", "backup_publication_linux.go", "n.main.fd, err = unix.Openat2", "unowned, err = unix.Openat2"},
		{"omit-no-xdev", "backup_namespace_linux.go", " | unix.RESOLVE_NO_XDEV", ""},
		{"overwrite-rename", "backup_publication_linux.go", "final, unix.RENAME_NOREPLACE", "final, 0"},
		{"absolute-create-fd", "backup_publication_linux.go", "unix.Openat2(int(n.dir.file.raw.Fd()), base", "unix.Openat2(unix.AT_FDCWD, base"},
		{"lose-exclusive-create", "backup_publication_linux.go", "|unix.O_EXCL", ""},
		{"missing-mount-mask", "backup_namespace_linux.go", "st.Mask&nativeBackupStatxMask != nativeBackupStatxMask", "false"},
		{"early-protector", "backup_publication.go", "p.terminal || !p.ops.directoryComplete()", "p.terminal"},
		{"forged-source", "backup_publication.go", "!source.valid || !source.parent.complete()", "!source.parent.complete()"},
		{"unlock-on-seal", "backup_publication_linux.go", "n.main.seal()", "n.main.file.Close()"},
		{"omit-data-sync-gate", "backup_publication.go", " || !p.status.DataSynced", ""},
		{"delete-destination-admission", "backup_publication.go", destinationGate, "_ = parentIdentity"},
		{"ignore-destination-admission", "backup_publication.go", destinationGate, "_ = backupDestinationAdmission(source, parentIdentity, p.final, p.candidate)"},
		{"substitute-destination-admission", "backup_publication.go", destinationGate, "if err = p.active(); err != nil { return p.fail(err) }"},
		{"destination-wrong-source", "backup_publication.go", destinationGate, strings.Replace(destinationGate, "source,", "BackupSourceIdentity{},", 1)},
		{"destination-wrong-parent", "backup_publication.go", destinationGate, strings.Replace(destinationGate, "parentIdentity,", "source.parent,", 1)},
		{"destination-wrong-final", "backup_publication.go", destinationGate, strings.Replace(destinationGate, "p.final,", "p.candidate,", 1)},
		{"destination-wrong-candidate", "backup_publication.go", destinationGate, strings.Replace(destinationGate, "p.candidate)", "p.final)", 1)},
		{"destination-discard-error", "backup_publication.go", destinationGate, strings.Replace(destinationGate, "if err =", "if _ =", 1)},
		{"destination-invert-error-condition", "backup_publication.go", destinationGate, strings.Replace(destinationGate, "err != nil", "err == nil", 1)},
		{"destination-swallow-error", "backup_publication.go", destinationGate, strings.Replace(destinationGate, "return p.fail(err)", "return nil", 1)},
		{"destination-substitute-error", "backup_publication.go", destinationGate, strings.Replace(destinationGate, "p.fail(err)", "p.fail(ErrUnsupported)", 1)},
		{"destination-error-without-failure-state", "backup_publication.go", destinationGate, strings.Replace(destinationGate, "return p.fail(err)", "return err", 1)},
		{"destination-conditional-gate", "backup_publication.go", destinationGate, "if false { " + destinationGate + " }"},
		{"destination-after-final-family", "backup_publication.go", destinationGate + "\n\t" + destinationFinal, destinationFinal + "\n\t" + destinationGate},
		{"destination-after-candidate-family", "backup_publication.go", destinationGate + "\n\t" + destinationFinal + "\n\t" + destinationCandidate, destinationFinal + "\n\t" + destinationCandidate + "\n\t" + destinationGate},
		{"destination-before-held-parent", "backup_publication.go", destinationParent + "\n\t" + destinationGate, "parentIdentity := source.parent\n\tvar err error\n\t" + destinationGate + "\n\tparentIdentity, err = p.ops.openDirectory(parent)\n\tif err != nil { return p.fail(err) }"},
		{"destination-substitute-held-parent", "backup_publication.go", destinationParent, strings.Replace(destinationParent, "p.ops.openDirectory(parent)", "source.parent, error(nil)", 1)},
		{"destination-wrong-open-parent", "backup_publication.go", destinationParent, strings.Replace(destinationParent, "openDirectory(parent)", "openDirectory(p.final)", 1)},
		{"destination-ignore-parent-error", "backup_publication.go", destinationParent, strings.Replace(destinationParent, "return p.fail(err)", "_ = err", 1)},
		{"destination-clear-parent-error", "backup_publication.go", destinationParent, strings.Replace(destinationParent, "\n\tif err", "\n\terr = nil\n\tif err", 1)},
		{"destination-overwrite-held-parent", "backup_publication.go", destinationGate, "parentIdentity = source.parent\n\t" + destinationGate},
		{"destination-create-before-gate", "backup_publication.go", destinationGate, "if err = p.ops.createCandidate(p.candidate); err != nil { return p.fail(err) }\n\t" + destinationGate},
		{"destination-create-before-parent", "backup_publication.go", destinationParent, "if err := p.ops.createCandidate(p.candidate); err != nil { return p.fail(err) }\n\t" + destinationParent},
		{"destination-success-before-gate", "backup_publication.go", destinationGate, "if parentIdentity.complete() { return nil }\n\t" + destinationGate},
		{"destination-swap-stored-names", "backup_publication.go", "p.final, p.candidate, p.expected = final, candidate, size", "p.final, p.candidate, p.expected = candidate, final, size"},
		{"delete-source-direct-child", "backup_namespace_linux.go", sourceGate, "_ = held; _ = named"},
		{"ignore-source-direct-child", "backup_namespace_linux.go", sourceGate, "_ = backupDirectChild(parent, held, named)"},
		{"substitute-source-direct-child", "backup_namespace_linux.go", sourceGate, "if err = nativeBackupContinuity(dir); err != nil { return backupFact{}, err }"},
		{"source-wrong-parent-input", "backup_namespace_linux.go", sourceGate, strings.Replace(sourceGate, "parent,", "held,", 1)},
		{"source-wrong-held-input", "backup_namespace_linux.go", sourceGate, strings.Replace(sourceGate, "held,", "parent,", 1)},
		{"source-wrong-named-input", "backup_namespace_linux.go", sourceGate, strings.Replace(sourceGate, "named)", "held)", 1)},
		{"source-discard-error", "backup_namespace_linux.go", sourceGate, strings.Replace(sourceGate, "if err =", "if _ =", 1)},
		{"source-invert-error-condition", "backup_namespace_linux.go", sourceGate, strings.Replace(sourceGate, "err != nil", "err == nil", 1)},
		{"source-swallow-error", "backup_namespace_linux.go", sourceGate, strings.Replace(sourceGate, "return backupFact{}, err", "return backupFact{}, nil", 1)},
		{"source-substitute-error", "backup_namespace_linux.go", sourceGate, strings.Replace(sourceGate, "return backupFact{}, err", "return backupFact{}, ErrIdentity", 1)},
		{"source-return-unvalidated-parent", "backup_namespace_linux.go", sourceGate, strings.Replace(sourceGate, "return backupFact{}, err", "return parent, nil", 1)},
		{"source-conditional-gate", "backup_namespace_linux.go", sourceGate, "if false { " + sourceGate + " }"},
		{"source-gate-after-main-stat", "backup_namespace_linux.go", sourceGate + "\n\t" + sourceStat, sourceStat + "\n\t" + sourceGate},
		{"source-gate-before-named-observation", "backup_namespace_linux.go", sourceNamed + "\n\t" + sourceGate, "named := held\n\t" + sourceGate + "\n\tnamed, err = nativeBackupNamedFact(dir, base)\n\tif err != nil { return backupFact{}, err }"},
		{"source-wrong-held-parent-observation", "backup_namespace_linux.go", sourceParent, strings.Replace(sourceParent, "nativeBackupHeldFact(dir.file)", "nativeBackupHeldFact(main)", 1)},
		{"source-wrong-held-main-observation", "backup_namespace_linux.go", sourceHeld, strings.Replace(sourceHeld, "nativeBackupHeldFact(main)", "nativeBackupHeldFact(dir.file)", 1)},
		{"source-wrong-named-directory", "backup_namespace_linux.go", sourceNamed, strings.Replace(sourceNamed, "nativeBackupNamedFact(dir, base)", "nativeBackupNamedFact(nil, base)", 1)},
		{"source-wrong-named-basename", "backup_namespace_linux.go", sourceNamed, strings.Replace(sourceNamed, "nativeBackupNamedFact(dir, base)", "nativeBackupNamedFact(dir, base+\"-wal\")", 1)},
		{"source-ignore-parent-observation-error", "backup_namespace_linux.go", sourceParent, strings.Replace(sourceParent, "return backupFact{}, err", "_ = err", 1)},
		{"source-ignore-held-observation-error", "backup_namespace_linux.go", sourceHeld, strings.Replace(sourceHeld, "return backupFact{}, err", "_ = err", 1)},
		{"source-ignore-named-observation-error", "backup_namespace_linux.go", sourceNamed, strings.Replace(sourceNamed, "return backupFact{}, err", "_ = err", 1)},
		{"source-clear-named-observation-error", "backup_namespace_linux.go", sourceNamed, strings.Replace(sourceNamed, "\n\tif err", "\n\terr = nil\n\tif err", 1)},
		{"source-overwrite-held-observation", "backup_namespace_linux.go", sourceGate, "held = named\n\t" + sourceGate},
		{"source-success-before-gate", "backup_namespace_linux.go", sourceGate, "if parent.complete() { return parent, nil }\n\t" + sourceGate},
		{"source-return-wrong-success-fact", "backup_namespace_linux.go", "return parent, nil", "return held, nil"},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			copy := make(map[string]string); for name, body := range original { copy[name] = body }
			if !strings.Contains(copy[mutation.file], mutation.before) { t.Fatal("mutation no longer applies") }
			copy[mutation.file] = strings.Replace(copy[mutation.file], mutation.before, mutation.after, 1)
			if _, err := parser.ParseFile(token.NewFileSet(), mutation.file, copy[mutation.file], 0); err != nil { t.Fatalf("mutation is not syntactically valid: %v", err) }
			if backupASTValidate(copy) == nil { t.Fatal("unsafe source mutation escaped guard") }
		})
	}
}
