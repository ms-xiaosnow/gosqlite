package main

import (
	"go/build/constraint"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// 全部输入是惰性 Go 源码字符串；不创建数据库、不构造 native owner。
func TestWALPublicationGuardRejectsActivationReferences(t *testing.T) {
	for _, source := range []string{
		`package sqlite; var options=Options{WALTailPublication:&WALTailPublicationOptions{}}`,
		`package x; func f(){opts.WALTailPublication=cfg}`,
		`package x; func f(){field:=&opts.WALTailPublication; _=field}`,
		`package x; import s "github.com/ms-xiaosnow/gosqlite/sqlite"; type O=s.Options; var options=O{nil}`,
		`package x; import s "github.com/ms-xiaosnow/gosqlite/sqlite"; type A=B; type B=s.Options; var options=A{nil}`,
		`package x; import s "github.com/ms-xiaosnow/gosqlite/sqlite"; type O=s.Options; func f(){type O struct{N int}; _=O{1}}; var options=O{nil}`,
		`package sqlite; type O[T any]=Options; var options=O[int]{nil}`,
		`package x; import s "github.com/ms-xiaosnow/gosqlite/sqlite"; var options=[]s.Options{{nil}}`,
		`package x; import s "github.com/ms-xiaosnow/gosqlite/sqlite"; var options=[]*s.Options{{nil}}`,
		`package x; import s "github.com/ms-xiaosnow/gosqlite/sqlite"; var options=map[s.Options]bool{{nil}:true}`,
		`package x; import s "github.com/ms-xiaosnow/gosqlite/sqlite"; type List=[]s.Options; var options=List{{nil}}`,
		`package x; import s "github.com/ms-xiaosnow/gosqlite/sqlite"; var options=map[string]s.Options{"x":{nil}}`,
		`package x; import s "github.com/ms-xiaosnow/gosqlite/sqlite"; type Alias=s.WALTailPublicationOptions`,
		`package x; import . "github.com/ms-xiaosnow/gosqlite/sqlite"; var options=Options{nil}`,
		`package x; import _ "github.com/ms-xiaosnow/gosqlite/internal/vfs"`,
		`package x; import p "github.com/ms-xiaosnow/gosqlite/internal/pager"; var open=p.OpenSessionTailPublicationAdmitted`,
		`package x; func f(){openSessionAdmitted(ctx,path,opts,admission,true)}`,
		`package x; func f(){session.publishWALTailLocked(ctx,pending,header,plan)}`,
		`package x; func f(){group.reserveWALPublication(main,old,mainInfo,oldInfo)}`,
		`package x; import v "github.com/ms-xiaosnow/gosqlite/internal/vfs"; var factory=v.NewWALPublication`,
		`package vfs; var factory=(newNativeWALPublication)`,
		`package vfs; type Factory=nativeWALPublication; var owner=Factory{}`,
		`package vfs; var raw=nativeWALPublicationStatx`,
		`package vfs; func f(){newWALPublicationWithOps(ops)}`,
		`package sqlite; func init(){opts.WALTailPublication=cfg}`,
		"//go:build manual_namespace\n\npackage x; var factory=NewWALPublication",
		"//go:build manual_recovery && linux\n\npackage x; var factory=OpenSessionTailPublicationAdmitted",
		"//go:build manual_wal_publication\n\npackage x; var factory=newNativeWALPublication",
	} {
		findings, err := inspectWALPublicationSource("fixture_test.go", []byte(source))
		if err != nil || len(findings) == 0 {
			t.Fatalf("activation reference escaped: %s: %v %v", source, findings, err)
		}
	}
}

func TestWALPublicationGuardSafeControls(t *testing.T) {
	for _, source := range []string{
		`package sqlite; var options=Options{WALExclusive:true}`,
		`package x; import s "github.com/ms-xiaosnow/gosqlite/sqlite"; type O=s.Options; var options=O{ReadOnly:true}`,
		`package x; import s "github.com/ms-xiaosnow/gosqlite/sqlite"; var options=[]s.Options{{WALExclusive:true}}`,
		`package x; type Options struct{N int}; var options=Options{1}`,
		`package x; import s "other/package"; type O=s.Options; var options=O{nil}`,
		`package x; var _="WALTailPublication NewWALPublication" // OpenSessionTailPublicationAdmitted`,
		`package x; type A=B; type B=A; var value=A{1}`,
	} {
		findings, err := inspectWALPublicationSource("fixture_test.go", []byte(source))
		if err != nil || len(findings) != 0 {
			t.Fatalf("safe source rejected: %s: %v %v", source, findings, err)
		}
	}
	if _, err := inspectWALPublicationSource("broken_test.go", []byte("package")); err == nil {
		t.Fatal("broken source accepted")
	}
}

func TestWALPublicationGuardPureFilesRejectNativeAliasesAndVariables(t *testing.T) {
	for _, path := range []string{"sqlite/wal_tail_publication_test.go", "driver/wal_tail_publication_test.go", walPublicationElapsedPath} {
		for _, source := range []string{
			`package sqlite; func f(){options:=Options{}; Open("fixture",options)}`,
			`package sqlite; var run=OpenContext`,
			`package driver; func f(){connector.Connect(ctx)}`,
			`package driver; var call=(*Connector).Connect`,
			`package sqlite; func TestMain(m *testing.M){m.Run()}`,
			`package sqlite; func init(){helper()}`,
		} {
			findings, err := inspectWALPublicationSource(path, []byte(source))
			joined := strings.Join(findings, "\n")
			if err != nil || !strings.Contains(joined, "inert policy/connector source cannot reference execution entry") || !strings.Contains(joined, "fixed-source boundary changed") {
				t.Fatalf("path-only exception or variable/alias bypass: %v %v", findings, err)
			}
		}
	}
}

func TestWALPublicationGuardExactCallBoundaries(t *testing.T) {
	for _, fixture := range []struct {
		path   string
		source string
	}{
		{"sqlite/sqlite.go", `package sqlite; func OpenContext(){pager.OpenSessionTailPublicationAdmitted(ctx, path, d.pagerOptions(), plainAdmission)}`},
		{"internal/pager/session.go", `package pager; func OpenSessionAdmitted(){openSessionAdmitted(ctx, path, opts, admission, false)}`},
		{"internal/pager/session.go", `package pager; func OpenSessionTailPublicationAdmitted(){openSessionAdmitted(ctx, path, opts, admission, true)}`},
		{"internal/pager/plain_wal_publication.go", `package pager; func reserveWALPublication(){vfs.NewWALPublication(main, old, mainInfo, oldInfo)}`},
	} {
		findings, err := inspectWALPublicationSource(fixture.path, []byte(fixture.source))
		if err != nil || len(findings) != 0 {
			t.Fatalf("exact caller rejected: %v %v", findings, err)
		}
		wrongArguments := strings.Replace(fixture.source, "ctx, path", "other, path", 1)
		if wrongArguments == fixture.source {
			wrongArguments = strings.Replace(fixture.source, "main, old", "other, old", 1)
		}
		for _, changed := range []string{
			strings.Replace(fixture.source, "(){", "(){go func(){", 1) + "()}",
			wrongArguments,
		} {
			findings, err := inspectWALPublicationSource(fixture.path, []byte(changed))
			if err != nil || len(findings) == 0 {
				t.Fatalf("changed caller accepted: %s: %v %v", changed, findings, err)
			}
		}
	}
	duplicate := `package pager; func OpenSessionAdmitted(){openSessionAdmitted(ctx, path, opts, admission, false);openSessionAdmitted(ctx, path, opts, admission, false)}`
	if findings, err := inspectWALPublicationSource("internal/pager/session.go", []byte(duplicate)); err != nil || len(findings) == 0 {
		t.Fatal("duplicate constructor call accepted", findings, err)
	}
	for _, source := range []string{
		`package sqlite; func OpenContext(){opts.WALTailPublication=cfg}`,
		`package sqlite; func OpenContext(){pointer:=&opts.WALTailPublication; _=pointer}`,
	} {
		if findings, err := inspectWALPublicationSource("sqlite/sqlite.go", []byte(source)); err != nil || len(findings) == 0 {
			t.Fatal("public bridge auto-enable or alias accepted", findings, err)
		}
	}
	wrongDefault := `package pager; func OpenSessionAdmitted(){openSessionAdmitted(ctx, path, opts, admission, true)}`
	if findings, err := inspectWALPublicationSource("internal/pager/session.go", []byte(wrongDefault)); err != nil || len(findings) == 0 {
		t.Fatal("default constructor enabled publication", findings, err)
	}
}

func TestWALPublicationGuardFixedSourceAndWiring(t *testing.T) {
	root := filepath.Join("..", "..")
	for path := range walPublicationFrozenSources {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		findings := inspectWALPublicationFrozenSource(path, raw)
		if len(findings) != 0 {
			t.Fatal(findings)
		}
		changed := append(append([]byte(nil), raw...), []byte("\n// unreviewed change\n")...)
		if findings := inspectWALPublicationFrozenSource(path, changed); len(findings) == 0 {
			t.Fatal("changed approved source accepted", path)
		}
	}
	raw, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, gate := range []string{"inspectWALPublicationSource(path, raw)", `inspectWALPublicationContracts(".")`} {
		if strings.Count(string(raw), gate) != 1 {
			t.Fatal("guard invocation missing", gate)
		}
		changed := []byte(strings.Replace(string(raw), gate, "disabledGate()", 1))
		if findings := inspectWALPublicationFrozenSource("internal/testscope/main.go", changed); len(findings) == 0 {
			t.Fatal("guard relaxation accepted", gate)
		}
	}
}

func TestWALPublicationGuardFakeFactoryIsExactSource(t *testing.T) {
	path := "internal/vfs/wal_publication_model_test.go"
	raw, err := os.ReadFile(filepath.Join("..", "vfs", "wal_publication_model_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	if findings, err := inspectWALPublicationSource(path, raw); err != nil || len(findings) != 0 {
		t.Fatal("fixed inert model rejected", findings, err)
	}
	for _, replacement := range []string{
		"NewWALPublication(m.main, m.old, nil, nil)",
		"newWALPublicationWithOps(newNativeWALPublication(m.main, m.old, nil, nil))",
	} {
		changed := strings.Replace(string(raw), "newWALPublicationWithOps(m)", replacement, 1)
		findings, err := inspectWALPublicationSource(path, []byte(changed))
		joined := strings.Join(findings, "\n")
		if err != nil || !strings.Contains(joined, "restricted activation/factory reference") || !strings.Contains(joined, "fixed-source boundary changed") {
			t.Fatal("fake factory admitted native source", findings, err)
		}
	}
}

func TestWALPublicationGuardProductionBoundariesAreRequired(t *testing.T) {
	paths := []string{
		"sqlite/sqlite.go",
		"driver/driver.go",
		"internal/pager/session.go",
		"internal/pager/wal.go",
		"internal/pager/wal_tail_publication.go",
		"internal/pager/plain_wal_publication.go",
		"internal/vfs/wal_publication.go",
		"internal/vfs/wal_publication_linux.go",
		"internal/vfs/wal_publication_other.go",
		"internal/vfs/wal_publication_namespace.go",
	}
	if len(walPublicationProductionBoundaries) != len(paths) {
		t.Fatal("production boundary membership changed")
	}
	for _, path := range paths {
		if digest, exists := walPublicationProductionBoundaries[path]; !exists || len(digest) != 64 {
			t.Fatal("required production boundary missing", path)
		}
		raw, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		if findings := inspectWALPublicationProductionSource(path, raw); len(findings) != 0 {
			t.Fatal(findings)
		}
		// 每个完整边界都必须拒绝哪怕只增加注释的字节变化；这不是运行
		// 变异后的程序，也不能用原 caller 拼写仍存在来授权默认选择。
		changed := append(append([]byte(nil), raw...), []byte("\n// unreviewed production change\n")...)
		if findings := inspectWALPublicationProductionSource(path, changed); len(findings) == 0 {
			t.Fatal("changed production boundary accepted", path)
		}
	}
}

func TestWALPublicationGuardRejectsControlFlowAndFactoryDrift(t *testing.T) {
	for _, change := range []struct {
		path string
		from string
		to   string
	}{
		{"sqlite/sqlite.go", "else if opts.WALTailPublication != nil {", "else if true {"},
		{"internal/pager/session.go", "openSessionAdmitted(ctx, path, opts, admission, false)", "openSessionAdmitted(ctx, path, opts, admission, true)"},
		{"internal/vfs/wal_publication.go", "return newWALPublicationWithOps(newNativeWALPublication(main, original, mainInfo, originalInfo))", "owner := newWALPublicationWithOps(newNativeWALPublication(main, original, mainInfo, originalInfo)); _ = owner.Acquire(); return owner"},
	} {
		raw, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(change.path)))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Count(string(raw), change.from) != 1 {
			t.Fatal("control-flow source variant is stale", change.path)
		}
		changed := []byte(strings.Replace(string(raw), change.from, change.to, 1))
		if findings := inspectWALPublicationProductionSource(change.path, changed); len(findings) == 0 {
			t.Fatal("activation or effectful factory drift accepted", change.path)
		}
	}
}

// Source-only checks. These never import the harness or execute its root.
func TestWALPublicationGuardManualSourceIsExactAndDoubleGated(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(walPublicationManualPath)))
	if err != nil {
		t.Fatal(err)
	}
	findings, err := inspectWALPublicationSource(walPublicationManualPath, raw)
	if err != nil || len(findings) != 0 {
		t.Fatal("exact pinned manual source rejected", findings, err)
	}
	text := string(raw)
	first := strings.SplitN(text, "\n", 2)[0]
	if first != walPublicationManualTag {
		t.Fatal("exact manual Linux constraint missing")
	}
	expression, err := constraint.Parse(first)
	if err != nil {
		t.Fatal(err)
	}
	for _, selection := range []struct{ linux, oracle, manual, want bool }{
		{true, false, false, false}, {true, true, false, false},
		{false, false, false, false}, {false, true, false, false},
		{true, false, true, true}, {false, false, true, false},
	} {
		got := expression.Eval(func(tag string) bool {
			switch tag {
			case "linux":
				return selection.linux
			case "oracle":
				return selection.oracle
			case "manual_wal_publication":
				return selection.manual
			}
			return false
		})
		if got != selection.want {
			t.Fatalf("manual default/oracle/platform source selection: %+v got=%v", selection, got)
		}
	}
	for _, required := range []string{
		"func TestManualPublicWALTailPublication(t *testing.T) {\n\tif os.Getenv(\"GOSQLITE_MANUAL_WAL_PUBLICATION\") != \"independently_authorized\" {",
		"GOSQLITE_MANUAL_WAL_PUBLICATION_DIR", "GOSQLITE_MANUAL_WAL_PUBLICATION_PROFILE",
		"STATX_MNT_ID_UNIQUE", "RESOLVE_BENEATH", "RESOLVE_NO_SYMLINKS", "RESOLVE_NO_MAGICLINKS", "RESOLVE_NO_XDEV",
		"nil-default-preserves-input", "open-publishes-writes-reopens", "open-context-publishes-writes-reopens", "connector-publishes-writes-reopens",
		"sqlite.Open(", "sqlite.OpenContext(", "connector.Connect(",
		"wal.TailUncommitted", "wal.TailClean", "sqlite.PendingPlaintextCleanups()",
	} {
		if !strings.Contains(text, required) {
			t.Fatal("manual source prerequisite missing", required)
		}
	}
	for _, forbidden := range []string{"func init(", "func TestMain(", "t.Parallel(", "t.TempDir(", "os.Remove(", "os.RemoveAll(", "os/exec", "testoracle", "RetryPlaintextCleanup(", "NewWALPublication(", "OpenSessionTailPublicationAdmitted("} {
		if strings.Contains(text, forbidden) {
			t.Fatal("unexpected manual activation/effect", forbidden)
		}
	}
	if strings.Count(text, "func Test") != 1 {
		t.Fatal("manual source must retain one finite root")
	}
	for _, changed := range []string{
		strings.Replace(text, walPublicationManualTag, "//go:build linux", 1),
		strings.Replace(text, walPublicationManualTag, "//go:build manual_wal_publication || linux", 1),
		strings.Replace(text, "independently_authorized", "automatic", 1),
		text + "\nfunc init() {}\n",
		text + "\n// unreviewed manual change\n",
	} {
		findings, err := inspectWALPublicationSource(walPublicationManualPath, []byte(changed))
		if err != nil || !strings.Contains(strings.Join(findings, "\n"), "fixed-source boundary changed") {
			t.Fatal("manual mutation escaped", findings, err)
		}
	}
	for _, path := range []string{"sqlite/other_manual_linux_test.go", "driver/wal_tail_publication_manual_linux_test.go"} {
		findings, err := inspectWALPublicationSource(path, raw)
		if err != nil || len(findings) == 0 {
			t.Fatal("manual file relocation escaped exact path", findings, err)
		}
	}
	// A manual build constraint never creates an exception in another file.
	bad := []byte(walPublicationManualTag + "\n\npackage x\nfunc f(){ opts.WALTailPublication = cfg }\n")
	if findings, err := inspectWALPublicationSource("fixture_test.go", bad); err != nil || len(findings) == 0 {
		t.Fatal("tag-only manual exemption", findings, err)
	}
}

// Exact active pins supplement the inert shape and double-gated manual boundary.
// These source checks do not execute the native root or authorize native use.
func TestWALPublicationGuardElapsedSourceIsInertAndPinned(t *testing.T) {
	if walPublicationFrozenSources[walPublicationManualPath] != "79cd3adaf4e946fcb93cbcf66f844d5cdc6596d1dffbfe2a9aac3e101dcfd59d" ||
		walPublicationFrozenSources[walPublicationElapsedPath] != "22874ed59d4ba7549aad1670573cde98d5079a24811e51c5201c6efa5c9506c5" {
		t.Fatal("reviewed elapsed active pin inventory changed")
	}
	for _, path := range []string{walPublicationManualPath, walPublicationElapsedPath} {
		raw, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		if oracleSourceDigest(raw) != walPublicationFrozenSources[path] {
			t.Fatal("reviewed elapsed byte inventory drift", path)
		}
		findings, err := inspectWALPublicationSource(path, raw)
		if err != nil || len(findings) != 0 {
			t.Fatal("exact pinned elapsed source rejected", path, findings, err)
		}
		changed := append(append([]byte(nil), raw...), []byte("\n// unreviewed elapsed change\n")...)
		findings, err = inspectWALPublicationSource(path, changed)
		joined := strings.Join(findings, "\n")
		if err != nil || !strings.Contains(joined, "fixed-source boundary changed") {
			t.Fatal("modified elapsed source escaped active pin", path, findings, err)
		}
		if path == walPublicationManualPath && !strings.Contains(joined, "opt-in field outside exact public policy") {
			t.Fatal("modified manual source retained activation exception", findings)
		}
	}
	raw, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(walPublicationElapsedPath)))
	if err != nil {
		t.Fatal(err)
	}
	for _, changed := range []string{
		strings.Replace(string(raw), "import \"testing\"", "import (\"testing\"; \"os\")", 1),
		strings.Replace(string(raw), "import \"testing\"", "import (\"testing\"; \"time\")", 1),
		strings.Replace(string(raw), "import \"testing\"", "import (\"testing\"; s \"github.com/ms-xiaosnow/gosqlite/sqlite\")", 1),
		string(raw) + "\nvar clock = helper()\n",
		string(raw) + "\nfunc init() {}\n",
		string(raw) + "\nfunc TestMain(m *testing.M) {}\n",
		string(raw) + "\nfunc TestExtra(t *testing.T) {}\n",
		strings.Replace(string(raw), "TestWALPublicationElapsedMemoryQualifiedSample", "TestRenamed", 1),
	} {
		findings, err := inspectWALPublicationSource(walPublicationElapsedPath, []byte(changed))
		joined := strings.Join(findings, "\n")
		if err != nil || !strings.Contains(joined, "fixed-source boundary changed") || !strings.Contains(joined, "WAL elapsed source shape:") {
			t.Fatal("pure scalar source mutation escaped", findings, err)
		}
	}
}

func TestWALPublicationGuardElapsedRejectsTimingAndQualificationDrift(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(walPublicationManualPath)))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, change := range []struct{ from, to string }{
		{"func TestManualPublicWALTailPublication(t *testing.T) {\n\tif os.Getenv", "func TestManualPublicWALTailPublication(t *testing.T) {\n\tprepareFixture()\n\tif os.Getenv"},
		{"db, err = sqlite.Open(f.path, opts)\n\t\telapsed := time.Since(start)", "db, err = sqlite.Open(f.path, opts)\n\t\ts.query(t, db, conn, nil)\n\t\telapsed := time.Since(start)"},
		{"conn, err = connector.Connect(ctx)\n\t\telapsed := time.Since(start)", "connector, err = gosqldriver.NewConnector(f.path, opts)\n\t\telapsed := time.Since(start)"},
		{"s.retire(t, 3)\n\t\ts.checkPending(t)", "s.finishElapsed(t, invocationStart)\n\t\ts.retire(t, 3)\n\t\ts.checkPending(t)"},
		{"flag.Lookup(\"test.v\").Value.String() != \"true\" ||", "flag.Lookup(\"test.v\").Value.String() == \"true\" ||"},
		{"a.Qualified, a.Samples = false, 0", "a.Qualified, a.Samples = true, 1"},
		{"s.elapsed.FailureLatched = s.elapsed.FailureLatched || t.Failed() || s.blocked", "s.elapsed.FailureLatched = false"},
		{"{\"nil-default-preserves-input\", \"\"},", "{\"skipped-control\", \"\"},"},
	} {
		if strings.Count(text, change.from) != 1 {
			t.Fatal("stale elapsed source mutation", change.from)
		}
		changed := []byte(strings.Replace(text, change.from, change.to, 1))
		findings, err := inspectWALPublicationSource(walPublicationManualPath, changed)
		joined := strings.Join(findings, "\n")
		if err != nil || !strings.Contains(joined, "fixed-source boundary changed") || !strings.Contains(joined, "WAL elapsed source shape:") {
			t.Fatal("timing/qualification drift escaped", findings, err)
		}
	}
	for _, suffix := range []string{
		"\nfunc TestExtra(t *testing.T) {}\n",
		"\nvar hidden = NewWALPublication\n",
		"\nvar alias = OpenSessionTailPublicationAdmitted\n",
	} {
		findings, err := inspectWALPublicationSource(walPublicationManualPath, []byte(text+suffix))
		if err != nil || !strings.Contains(strings.Join(findings, "\n"), "fixed-source boundary changed") {
			t.Fatal("extra root/private factory escaped", findings, err)
		}
	}
}
