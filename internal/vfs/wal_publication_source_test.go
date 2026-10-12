package vfs

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"strings"
	"testing"
)

// These regressions parse source bytes only. Native owners/factories, databases,
// environment probes, fixtures, programs and publication are never executed.
func walPublicationASTText(node ast.Node) string {
	var b bytes.Buffer
	_ = printer.Fprint(&b, token.NewFileSet(), node)
	return strings.Join(strings.Fields(b.String()), "")
}
func walPublicationASTFunctions(raw []byte) (map[string]*ast.FuncDecl, error) {
	file, err := parser.ParseFile(token.NewFileSet(), "source.go", raw, 0)
	if err != nil { return nil, err }
	out := make(map[string]*ast.FuncDecl)
	for _, d := range file.Decls {
		if f, ok := d.(*ast.FuncDecl); ok { out[f.Name.Name] = f }
	}
	return out, nil
}
func walPublicationASTCalls(fn *ast.FuncDecl, name string) []*ast.CallExpr {
	var calls []*ast.CallExpr
	if fn == nil { return calls }
	ast.Inspect(fn.Body, func(node ast.Node) bool {
		if call, ok := node.(*ast.CallExpr); ok && walPublicationASTText(call.Fun) == name { calls = append(calls, call) }
		return true
	})
	return calls
}
func walPublicationSourceContract(native, facade, adapter, orchestration, session []byte) []string {
	var problems []string
	parse := func(raw []byte) map[string]*ast.FuncDecl {
		f, err := walPublicationASTFunctions(raw)
		if err != nil { problems = append(problems, err.Error()) }
		return f
	}
	n, f, a, o, s := parse(native), parse(facade), parse(adapter), parse(orchestration), parse(session)
	require := func(condition bool, name string) { if !condition { problems = append(problems, name) } }
	body := func(functions map[string]*ast.FuncDecl, name string) string {
		if fn := functions[name]; fn != nil { return walPublicationASTText(fn.Body) }
		return ""
	}
	oneCall := func(functions map[string]*ast.FuncDecl, fn, callee, exact string) token.Pos {
		calls := walPublicationASTCalls(functions[fn], callee)
		require(len(calls) == 1, fn+"/"+callee+" count")
		if len(calls) != 1 { return 0 }
		if exact != "" { require(walPublicationASTText(calls[0]) == exact, fn+"/"+callee+" arguments") }
		return calls[0].Pos()
	}
	oneCall(n, "create", "unix.Openat2", "unix.Openat2(int(n.dir.file.raw.Fd()),n.candidateBase,&how)")
	create := body(n, "create")
	for _, bit := range []string{"unix.O_RDWR", "unix.O_CREAT", "unix.O_EXCL", "unix.O_NOFOLLOW", "unix.O_CLOEXEC", "unix.O_NONBLOCK", "Resolve:nativeWALPublicationResolve"} { require(strings.Contains(create, bit), "candidate flag "+bit) }
	file, err := parser.ParseFile(token.NewFileSet(), "native.go", native, 0)
	if err != nil { return problems }
	constants := map[string]string{}
	ast.Inspect(file, func(node ast.Node) bool {
		if value, ok := node.(*ast.ValueSpec); ok && len(value.Names) == 1 && len(value.Values) == 1 { constants[value.Names[0].Name] = walPublicationASTText(value.Values[0]) }
		return true
	})
	for _, bit := range []string{"unix.STATX_TYPE", "unix.STATX_NLINK", "unix.STATX_MTIME", "unix.STATX_CTIME", "unix.STATX_INO", "unix.STATX_SIZE", "unix.STATX_MNT_ID_UNIQUE"} { require(strings.Contains(constants["nativeWALPublicationMask"], bit), "statx request "+bit) }
	for _, bit := range []string{"unix.RESOLVE_BENEATH", "unix.RESOLVE_NO_SYMLINKS", "unix.RESOLVE_NO_MAGICLINKS", "unix.RESOLVE_NO_XDEV"} { require(strings.Contains(constants["nativeWALPublicationResolve"], bit), "resolution "+bit) }
	oneCall(n, "nativeWALPublicationStatx", "unix.Statx", "unix.Statx(fd,name,flags,nativeWALPublicationMask,&st)")
	require(strings.Contains(body(n, "nativeWALPublicationStatx"), "st.Mask&nativeWALPublicationMask!=nativeWALPublicationMask"), "returned unique-mount fields")
	for _, gate := range []string{"n.main.role!=Database", "!n.main.writable", "!n.main.shared", "!n.main.reserved", "!n.main.exclusive", "n.original.role!=WAL", "!n.original.replaceable"} { require(strings.Contains(body(n, "guard"), gate), "main/WAL role gate "+gate) }
	oneCall(n, "candidateValid", "walPublicationDistinct", "walPublicationDistinct(n.mainFact,n.originalFact,fact,n.shmFact,n.shmPresent)")
	oneCall(n, "replace", "unix.Renameat2", `unix.Renameat2(int(n.dir.file.raw.Fd()),n.candidateBase,int(n.dir.file.raw.Fd()),n.base+"-wal",0)`)
	replace := body(n, "replace")
	require(strings.Index(replace, "n.replaceAttempted=true") >= 0 && strings.Index(replace, "n.replaceAttempted=true") < strings.Index(replace, "unix.Renameat2("), "native replace permit before syscall")
	require(strings.Contains(replace, "iferr!=nil{returnfalse,nativeWALPublicationError(err)}n.renamed=true"), "known replacement recorded separately")
	oneCall(n, "postPublication", "unix.Unlinkat", `unix.Unlinkat(int(n.dir.file.raw.Fd()),n.base+"-shm",0)`)
	post := body(n, "postPublication")
	require(strings.Index(post, "n.inspectSHM(false)") >= 0 && strings.Index(post, "n.inspectSHM(false)") < strings.Index(post, "unix.Unlinkat("), "anchored SHM identity before unlink")
	oneCall(n, "syncDirectory", "n.dir.file.raw.Sync", "n.dir.file.raw.Sync()")
	for _, forbidden := range []string{"os.Rename", "os.Remove", "nativeReplace", "ReplaceWAL", "RENAME_NOREPLACE", "RENAME_EXCHANGE"} {
		require(!strings.Contains(string(native), forbidden), "forbidden fallback "+forbidden)
	}
	oneCall(f, "NewWALPublication", "newNativeWALPublication", "newNativeWALPublication(main,original,mainInfo,originalInfo)")
	factory := body(n, "newNativeWALPublication")
	for _, forbidden := range []string{"acquire(", "Open(", "Openat2(", "Stat(", "Close(", "Sync("} { require(!strings.Contains(factory, forbidden), "effectful factory "+forbidden) }
	finish := body(f, "FinishPublication")
	require(strings.Contains(finish, "info,observed=p.ops.postPublication()") && strings.Contains(finish, "barrier:=p.ops.syncDirectory()"), "post-failure barrier dependency")
	oneCall(f, "FinishPublication", "p.ops.syncDirectory", "p.ops.syncDirectory()")
	close := body(f, "Close")
	require(strings.Index(close, "p.ops.closeChild()") >= 0 && strings.Index(close, "p.ops.closeChild()") < strings.Index(close, "p.ops.closeDirectory()"), "child before directory retirement")
	require(strings.Contains(close, "if!complete||p.terminal"), "incomplete child holds directory")
	require(strings.Contains(close, "p.childCloseAttempted=true") && strings.Contains(close, "p.directoryCloseAttempted=true"), "one-shot close tombstones")
	reserve := body(a, "reserveWALPublication")
	require(strings.Index(reserve, "g.reserveTemporary()") >= 0 && strings.Index(reserve, "g.reserveTemporary()") < strings.Index(reserve, "vfs.NewWALPublication("), "reservation before composite factory")
	require(strings.Contains(reserve, "g.installWALPublication(owner)"), "preowned composite install")
	transfer := body(a, "transferWALPublication")
	require(strings.Contains(transfer, "g.temporary!=owner") && strings.Contains(transfer, "current.file!=old") && strings.Contains(transfer, "g.wal=plainFile{next}"), "checked owner exchange")
	for _, forbidden := range []string{".Close(", ".Acquire(", ".Sync(", ".LockDB(", ".Stat("} { require(!strings.Contains(transfer, forbidden), "effectful root transfer "+forbidden) }
	publish := body(o, "publishWALTailLocked")
	require(strings.Index(publish, "s.cleanup.reserveWALPublication(") >= 0 && strings.Index(publish, "s.cleanup.reserveWALPublication(") < strings.Index(publish, "runWALTailPublication("), "install before orchestration acquisition")
	require(strings.Index(publish, "s.cleanup.closeTemporary(operation)") >= 0 && strings.Index(publish, "s.cleanup.closeTemporary(operation)") < strings.Index(publish, "s.walInfo,s.header,s.snapshot,s.appendSafe="), "retire before snapshot exposure")
	constructor := body(s, "openSessionAdmitted")
	require(strings.Index(constructor, "prepareWALTailView(") >= 0 && strings.Index(constructor, "prepareWALTailView(") < strings.Index(constructor, "s.publishWALTailLocked("), "paired validation before publication")
	require(strings.Contains(constructor, "s.publishWALTailLocked(ctx,pending,validated,plan);err!=nil{returnnil,err}returns,nil}"), "tail route exits before path SHM removal")
	require(strings.Contains(body(s, "OpenSessionAdmitted"), "openSessionAdmitted(ctx,path,opts,admission,false)"), "legacy nil route")
	return problems
}
func walPublicationSourceBytes(t *testing.T) [][]byte {
	t.Helper()
	var out [][]byte
	for _, name := range []string{"wal_publication_linux.go", "wal_publication.go", "../pager/plain_wal_publication.go", "../pager/wal_tail_publication.go", "../pager/session.go"} {
		raw, err := os.ReadFile(name)
		if err != nil { t.Fatal(err) }
		out = append(out, raw)
	}
	return out
}
func TestWALPublicationSourceDependencies(t *testing.T) {
	s := walPublicationSourceBytes(t)
	if problems := walPublicationSourceContract(s[0], s[1], s[2], s[3], s[4]); len(problems) != 0 { t.Fatal(problems) }
}
func TestWALPublicationSourceNegativeVariants(t *testing.T) {
	s := walPublicationSourceBytes(t)
	for _, change := range []struct{ file int; from, to string }{
		{0, "unix.STATX_MNT_ID_UNIQUE", "unix.STATX_MNT_ID"},
		{0, "unix.RESOLVE_NO_XDEV", "0"},
		{0, "unix.O_NOFOLLOW", "0"},
		{0, "st.Mask&nativeWALPublicationMask != nativeWALPublicationMask", "false"},
		{0, "!n.main.exclusive", "false"},
		{0, `n.base+"-wal", 0)`, `n.base+"-wal", unix.RENAME_NOREPLACE)`},
		{0, "n.replaceAttempted = true", "n.replaceAttempted = false"},
		{0, "n.inspectSHM(false)", "nil"},
		{1, "barrier := p.ops.syncDirectory()", "barrier := error(nil)"},
		{1, "p.ops.closeChild()", "p.ops.closeDirectory()"},
		{2, "g.temporary != owner", "false"},
		{2, "current.file != old", "false"},
		{3, "s.cleanup.closeTemporary(operation)", "operation"},
		{4, "admission, false)", "admission, true)"},
	} {
		variant := append([][]byte(nil), s...)
		if !bytes.Contains(variant[change.file], []byte(change.from)) { t.Fatal("stale negative mutation", change.from) }
		variant[change.file] = bytes.ReplaceAll(variant[change.file], []byte(change.from), []byte(change.to))
		if problems := walPublicationSourceContract(variant[0], variant[1], variant[2], variant[3], variant[4]); len(problems) == 0 { t.Fatal("unsafe source variant accepted", change.from) }
	}
}
