package main

// All mutations below are inert text. No compiler, shell, native root, fixture,
// process, permission setting, or keeper is called by these default tests.
import (
	"go/build/constraint"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readOnlyWALManualFixture(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
func readOnlyWALManualMutation(t *testing.T, raw, before, after string) string {
	t.Helper()
	if !strings.Contains(raw, before) || before == after {
		t.Fatal("inert mutation anchor missing")
	}
	return strings.Replace(raw, before, after, 1)
}
func readOnlyWALManualReject(t *testing.T, path, raw, diagnostic string) {
	t.Helper()
	findings, err := inspectReadOnlyWALManualSource(path, []byte(raw))
	if err != nil {
		t.Fatal("mutation must remain parseable inert Go", err)
	}
	for _, finding := range findings {
		if strings.Contains(finding, diagnostic) {
			return
		}
	}
	t.Fatalf("dedicated semantic diagnostic %q missing; a pin mismatch does not satisfy the counterexample: %v", diagnostic, findings)
}
func TestReadOnlyWALManualSourceContract(t *testing.T) {
	readOnlyWALManualSemanticBaseline(t)

	// HOLD pins deliberately make this fail until independent review releases
	// the actual source identities. No source author silently self-pins.
	for _, path := range []string{readOnlyWALManualNativePath, readOnlyWALManualPurePath} {
		raw := []byte(readOnlyWALManualFixture(t, path))
		findings, err := inspectReadOnlyWALManualSource(path, raw)
		if err != nil || len(findings) != 0 {
			t.Fatal(path, findings, err)
		}
		ordinary, err := inspectSource(path, raw)
		if err != nil || len(ordinary) != 0 {
			t.Fatal("old ordinary guard still active", ordinary, err)
		}
	}
	findings, err := inspectReadOnlyWALManualContracts(filepath.Join("..", ".."))
	if err != nil || len(findings) != 0 {
		t.Fatal(findings, err)
	}
}
func TestReadOnlyWALManualDefaultExclusion(t *testing.T) {
	raw := readOnlyWALManualFixture(t, readOnlyWALManualNativePath)
	expression, err := constraint.Parse(strings.SplitN(raw, "\n", 2)[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, system := range []string{"linux", "darwin", "windows"} {
		for _, manual := range []bool{false, true} {
			selected := expression.Eval(func(tag string) bool { return tag == system || tag == "manual_readonly_wal" && manual })
			if selected != (system == "linux" && manual) {
				t.Fatal("default/platform exclusion", system, manual)
			}
		}
	}
	for _, tag := range []string{"// missing build constraint", "//go:build manual_readonly_wal || linux", "//go:build manual_readonly_wal && !linux", "//go:build linux"} {
		changed := readOnlyWALManualMutation(t, raw, readOnlyWALManualTag, tag)
		readOnlyWALManualReject(t, readOnlyWALManualNativePath, changed, "exact Linux AND manual build constraint")
	}
}
func TestReadOnlyWALManualWholeRootInventory(t *testing.T) {
	readOnlyWALManualPreviousPolicyPropagationMutations(t)
	readOnlyWALManualCombinedPolicyPropagationMutations(t)

	for path, raw := range map[string]string{
		"sqlite/copied_windows_test.go": "package sqlite_test\nfunc TestManualReadOnlyWALPreparedMedia() {}\n",
		"testdata/hidden_linux_test.go": "//go:build manual_readonly_wal && linux\n\npackage hidden\n",
		"unapproved/manual/helper.go":   "package helper\nfunc manualReadOnlyWALNativeCopy() {}\n",
		"sqlite/indirect_test.go":       "package sqlite_test\nvar borrowed = manualReadOnlyWALDecodeManifest\n",
		"sqlite/alias_test.go":          "package sqlite_test\nfunc x() { borrowed := TestManualReadOnlyWALPreparedMedia; _ = borrowed }\n",
		"sqlite/receiver_test.go":       "package sqlite_test\ntype x int\nfunc (x) manualReadOnlyWALNativeShim() {}\n",
	} {
		readOnlyWALManualReject(t, path, raw, "whole-root")
	}
	for _, imported := range []string{`"example.invalid/manualreadonlywal"`, `renamed "example.invalid/readonly_wal_manual"`, `. "example.invalid/readonly-wal-manual"`, `_ "example.invalid/manual-readonly-wal"`} {
		readOnlyWALManualReject(t, "sqlite/consumer_test.go", "package sqlite_test\nimport "+imported+"\n", "whole-root unapproved manual import consumer")
	}
	main := readOnlyWALManualFixture(t, "internal/testscope/main.go")
	for _, mutation := range [][2]string{
		{"inspectReadOnlyWALManualSource(path, raw)", "inspectBackupManualSource(path, raw)"},
		{`inspectReadOnlyWALManualContracts(".")`, `inspectBackupManualContracts(".")`},
		{"findings = append(findings, readonlyManual...)", "_ = readonlyManual"},
	} {
		readOnlyWALManualReject(t, "internal/testscope/main.go", readOnlyWALManualMutation(t, main, mutation[0], mutation[1]), "dead scanner wiring")
	}
	readOnlyWALManualReject(t, "internal/testscope/main.go", readOnlyWALManualMutation(t, main, "inspectWALPublicationSource(path, raw)", "inspectOracleSource(path, raw)"), "previous guard wiring weakened")
	readOnlyWALManualReject(t, "internal/testscope/main.go", readOnlyWALManualMutation(t, main, `first == "//go:build manual_namespace"`, `first == "//go:build manual_namespace" || first == "//go:build manual_readonly_wal && linux"`), "previous ordinary policy/exemptions weakened")
}
func TestReadOnlyWALManualGateAndProfileMutations(t *testing.T) {
	readOnlyWALManualPreReceiptContentMutations(t)
	readOnlyWALManualCaseContextMutations(t)
	readOnlyWALManualProcProvenanceMutations(t)
	readOnlyWALManualControlBindingMutations(t)

	raw := readOnlyWALManualFixture(t, readOnlyWALManualNativePath)
	cases := []struct{ before, after, want string }{
		{`func TestManualReadOnlyWALPreparedMedia(t *testing.T) {`, `func TestManualReadOnlyWALPreparedMedia(t *testing.T) { t.Log("before gate")`, "gate must be the first native action"},
		{`!= "independently_authorized"`, `!= "1"`, "gate must be the first native action"},
		{`"-test.timeout=0"`, `"-test.timeout=30s"`, "gate exact argv"},
		{`"-test.parallel=1"`, `"-test.parallel=2"`, "gate exact argv"},
		{`unix.Faccessat2`, `unix.Faccessat`, "profile requires direct effective access"},
		{`unix.STATX_MNT_ID`, `unix.STATX_MNT_ID_UNIQUE`, "profile requires direct effective access and ordinary mount ID"},
		{`s.Mask&mask != mask`, `false`, "profile returned statx mask"},
		{`mount.ReadOnly = fs.Flags&unix.ST_RDONLY != 0`, `mount.ReadOnly = true`, "profile observed mount/access"},
		{`matches := mounts[s.MountID]`, `matches := mounts[1]`, "profile observed mount/access"},
		{`if err := manualReadOnlyWALValidateProfile(snapshot.Facts, r.Manifest); err != nil`, `if err := manualReadOnlyWALValidateProfile(snapshot.Facts,r.Manifest); false`, "profile pure admission decision"},
		{`var capData [2]unix.CapUserData`, `var capData [1]unix.CapUserData`, "profile actual credentials/capability words"},
		{`if err := manualReadOnlyWALNativeReceipt(r); err != nil {
		return err
	}`, `if false { return nil }`, "gate keeper receipt precedes case observation/Open"},
		{`if err := manualReadOnlyWALNativeCase(r, false); err != nil {
		return err
	}`, `_ = manualReadOnlyWALNativeCase(r,false)`, "gate sequential fail-stop cases"},
		{`r.Phase = next`, `_ = next`, "gate fresh bounded keeper receipt"},
	}
	for _, tc := range cases {
		readOnlyWALManualReject(t, readOnlyWALManualNativePath, readOnlyWALManualMutation(t, raw, tc.before, tc.after), tc.want)
	}
	pure := readOnlyWALManualFixture(t, readOnlyWALManualPurePath)
	readOnlyWALManualReject(t, readOnlyWALManualPurePath, readOnlyWALManualMutation(t, pure, `!object.Observed || !object.AccessObserved || !object.ReadAllowed`, `!object.Observed || !object.ReadAllowed`), "pure observed profile fail-closed")
}
func TestReadOnlyWALManualEffectBoundaryMutations(t *testing.T) {
	readOnlyWALManualMountFieldMutations(t)

	raw := readOnlyWALManualFixture(t, readOnlyWALManualNativePath)
	for _, tc := range []struct{ before, after, want string }{
		{`Flags: unix.O_RDONLY | unix.O_DIRECTORY | unix.O_CLOEXEC`, `Flags:unix.O_RDWR|unix.O_DIRECTORY|unix.O_CLOEXEC`, "native effect forbidden operation O_RDWR"},
		{`unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC|unix.O_NOFOLLOW`, `unix.O_WRONLY|unix.O_CREAT|unix.O_TRUNC|unix.O_CLOEXEC|unix.O_NOFOLLOW`, "native effect forbidden operation O_TRUNC"},
		{`unix.Openat(r.Owners[r.Child].FD, name,`, `unix.Openat(r.Owners[r.Source].FD,name,`, "effect exclusive evidence only"},
		{`id != r.Events && id != r.Input`, `false`, "effect evidence-owned write/sync"},
		{`unix.Write(r.Owners[id].FD, raw)`, `unix.Write(r.Owners[r.Source].FD,raw)`, "effect evidence write receiver binding"},
		{`"/proc/thread-self/status", 65536`, `"/proc/1/status",65536`, "profile actual credentials/capability words"},
		{`MaxPages: 16, MaxPageReads: 256`, `MaxPages:160,MaxPageReads:2560`, "effect fixed metadata/check budgets"},
		{`os.Getenv("GOSQLITE_MANUAL_READONLY_WAL_MANIFEST")`, `os.Getenv("HOME")`, "effect named environment only"},
		{`rows.Columns()`, `rows.Values()`, "effect fixed row columns"},
		{`r.Owners[parent].DB.Query(r.APICtx, manualReadOnlyWALQuery)`, `r.Owners[parent].DB.Query(r.APICtx,"SELECT 1")`, "effect sole fixed Query arguments"},
	} {
		readOnlyWALManualReject(t, readOnlyWALManualNativePath, readOnlyWALManualMutation(t, raw, tc.before, tc.after), tc.want)
	}
	for _, extra := range []string{
		`func manualReadOnlyWALNativeExtra() { unix.Chmod("/fixture",0600) }`,
		`func manualReadOnlyWALNativeExtra() { sqlite.RetryCloseCleanup(nil) }`,
		`func manualReadOnlyWALNativeExtra() { os.RemoveAll("/evidence") }`,
	} {
		readOnlyWALManualReject(t, readOnlyWALManualNativePath, raw+"\n"+extra+"\n", "native effect forbidden operation")
	}
	readOnlyWALManualReject(t, readOnlyWALManualNativePath, raw+"\nfunc manualReadOnlyWALNativeExtra() { f := unix.Openat2; _ = f }\n", "function-value reference forbidden")
	pure := readOnlyWALManualFixture(t, readOnlyWALManualPurePath)
	readOnlyWALManualReject(t, readOnlyWALManualPurePath, readOnlyWALManualMutation(t, pure, `const manualReadOnlyWALQuery = "SELECT id,text FROM t ORDER BY id"`, `const manualReadOnlyWALQuery = "SELECT id,text FROM t ORDER BY id; DELETE FROM t"`), "effect sole fixed SELECT literal")
}

// These are inert source variants in the existing effect-boundary root. Safe
// field reads must have no Mount diagnostic, so unrelated old field false
// positives cannot make the operation-reference counterexamples pass.
func readOnlyWALManualMountFieldMutations(t *testing.T) {
	const diagnostic = "native effect forbidden operation Mount"
	native := readOnlyWALManualFixture(t, readOnlyWALManualNativePath)
	pure := readOnlyWALManualFixture(t, readOnlyWALManualPurePath)
	const nativeAnchor = `snapshot.Control = control`
	const pureAnchor = `func manualReadOnlyWALValidateProfile(f manualReadOnlyWALFacts, m manualReadOnlyWALManifest) error {`
	accept := func(path, raw string) {
		t.Helper()
		findings, err := inspectReadOnlyWALManualSource(path, []byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		for _, finding := range findings {
			if strings.Contains(finding, diagnostic) {
				t.Fatal("typed metadata field mistaken for operation", path, finding)
			}
		}
	}
	accept(readOnlyWALManualNativePath, native)
	accept(readOnlyWALManualPurePath, pure)
	for _, read := range []string{
		`_ = control.Mount`,
		`_ = control.Mount == evidence.Mount`,
		`_ = control.Mount.ReadOnly`,
	} {
		accept(readOnlyWALManualNativePath, readOnlyWALManualMutation(t, native, nativeAnchor, nativeAnchor+"; "+read))
	}
	for _, read := range []string{
		`_ = f.Evidence.Mount`,
		`_ = f.Evidence.Mount == f.Source[0].Mount`,
		`var direct manualReadOnlyWALObject; _ = direct.Mount`,
	} {
		accept(readOnlyWALManualPurePath, readOnlyWALManualMutation(t, pure, pureAnchor, pureAnchor+" "+read))
	}
	// Both real packages, with and without import aliases, remain forbidden
	// for direct calls, direct function values and saved function values.
	for _, pkg := range []string{"golang.org/x/sys/unix", "syscall"} {
		for _, alias := range []string{"", "mountsys"} {
			raw, receiver := native, filepath.Base(pkg)
			if alias != "" {
				receiver = alias
			}
			if pkg != "golang.org/x/sys/unix" || alias != "" {
				raw = readOnlyWALManualMutation(t, raw, "import (", "import (\n\t"+alias+" \""+pkg+"\"")
			}
			for _, effect := range []string{
				`_ = RECEIVER.Mount("", "", "", 0, "")`,
				`_ = RECEIVER.Mount`,
				`saved := RECEIVER.Mount; _ = saved`,
			} {
				effect = strings.ReplaceAll(effect, "RECEIVER", receiver)
				readOnlyWALManualReject(t, readOnlyWALManualNativePath, readOnlyWALManualMutation(t, raw, nativeAnchor, nativeAnchor+"; "+effect), diagnostic)
			}
		}
	}
	for _, effect := range []string{
		`_ = unknown.Mount`,
		`_ = unknown.Mount()`,
		`var other otherObject; _ = other.Mount`,
		`value := struct { Mount func() error }{}; _ = value.Mount`,
		`value := struct { Mount func() error }{}; _ = value.Mount()`,
		`{ var control otherObject; _ = control.Mount }`,
		`_ = otherObject{Mount: control.Mount}`,
		`_ = control.Mount()`,
		`_ = (control.Mount)()`,
		`control.Mount = evidence.Mount`,
		`control.Mount.ReadOnly = true`,
		`control.Mount.ID++`,
		`_ = &control.Mount`,
		`for _, control.Mount.ID = range []uint64{7} {}`,
		`for control.Mount.ID = range map[uint64]bool{7: true} {}`,
		`for control.Mount.ID, evidence.Mount.ID = range map[uint64]uint64{7: 8} {}`,
		`for _, (control.Mount.ID) = range []uint64{7} {}`,
		`for _, control.Mount = range []manualReadOnlyWALMount{{ID: 7}} {}`,
	} {
		readOnlyWALManualReject(t, readOnlyWALManualNativePath, readOnlyWALManualMutation(t, native, nativeAnchor, nativeAnchor+"; "+effect), diagnostic)
	}
	readOnlyWALManualReject(t, readOnlyWALManualNativePath, native+"\nfunc manualReadOnlyWALNativeOther(value manualReadOnlyWALObject) { _ = value.Mount }\n", diagnostic)
	readOnlyWALManualReject(t, readOnlyWALManualNativePath, native+"\ntype manualReadOnlyWALObject struct { Mount func() error }\n", diagnostic)
	readOnlyWALManualReject(t, readOnlyWALManualPurePath, readOnlyWALManualMutation(t, pure, "Mount                            manualReadOnlyWALMount", "Mount                            func() error"), diagnostic)
	readOnlyWALManualReject(t, readOnlyWALManualPurePath, readOnlyWALManualMutation(t, pure, pureAnchor, pureAnchor+" type manualReadOnlyWALObject struct {}; var other manualReadOnlyWALObject; _ = other.Mount"), diagnostic)
	readOnlyWALManualReject(t, readOnlyWALManualPurePath, readOnlyWALManualMutation(t, pure, pureAnchor, pureAnchor+" for _, f.Evidence.Mount.ID = range []uint64{7} {}"), diagnostic)
	readOnlyWALManualReject(t, readOnlyWALManualPurePath, readOnlyWALManualMutation(t, pure, `valid := manualReadOnlyWALMemoryFacts(m)`, `valid := manualReadOnlyWALMemoryFacts(m); for _, valid.Source[0].Mount.ID = range []uint64{7} {}`), diagnostic)
}

// These source-only regressions separate non-callable data fields from
// actual owner method captures; unrelated baseline findings cannot satisfy them.
func readOnlyWALManualOwnerFieldMutations(t *testing.T) {
	const diagnostic = "native/pure owner method-value reference forbidden"
	native := readOnlyWALManualFixture(t, readOnlyWALManualNativePath)
	pure := readOnlyWALManualFixture(t, readOnlyWALManualPurePath)
	accept := func(path, raw string) {
		t.Helper()
		findings, err := inspectReadOnlyWALManualSource(path, []byte(raw))
		if err != nil {
			t.Fatal(err)
		}
		for _, finding := range findings {
			if strings.Contains(finding, diagnostic) {
				t.Fatal("inert typed field mistaken for owner method", path, finding)
			}
		}
	}
	accept(readOnlyWALManualNativePath, native)
	accept(readOnlyWALManualPurePath, pure)
	for _, mutation := range [][2]string{
		{`if r.Phase.Next == 0 {`, `if r.Phase.Next == 0 || r.Phase.Next < 0 {`},
		{`manualReadOnlyWALNativeSchemaOK(inspection.Schema)`, `manualReadOnlyWALNativeSchemaOK(inspection.Schema) && manualReadOnlyWALNativeSchemaOK(inspection.Schema)`},
	} {
		accept(readOnlyWALManualNativePath, readOnlyWALManualMutation(t, native, mutation[0], mutation[1]))
	}
	accept(readOnlyWALManualPurePath, readOnlyWALManualMutation(t, pure, `if s.Next != 4 {`, `if s.Next != 4 || s.Next < 0 {`))
	const nativeAnchor = `func manualReadOnlyWALNativeMetadata(r *manualReadOnlyWALNativeRun, dbID int) error {`
	for _, method := range []string{
		`r.Owners[dbID].Rows.Close`, `r.Owners[dbID].Rows.Next`, `r.Owners[dbID].Rows.Err`,
		`r.Owners[dbID].Tx.Commit`, `r.Owners[dbID].Tx.Rollback`, `r.Owners[dbID].Tx.Query`,
		`r.Owners[dbID].DB.Query`, `r.Owners[dbID].DB.Schema`, `r.Owners[dbID].DB.Inspect`,
		`r.Owners[dbID].DB.Check`, `r.Owners[dbID].DB.Begin`, `r.Owners[dbID].DB.Close`,
	} {
		for _, capture := range []string{`_ = METHOD`, `saved := METHOD; _ = saved`, `_ = (METHOD)`} {
			capture = strings.ReplaceAll(capture, "METHOD", method)
			readOnlyWALManualReject(t, readOnlyWALManualNativePath, readOnlyWALManualMutation(t, native, nativeAnchor, nativeAnchor+" "+capture), diagnostic)
		}
	}
	for _, mutation := range [][2]string{
		{`if r.Phase.Next == 0 {`, `if r.Owners[0].Rows.Next == nil {`},
		{`if r.Phase.Next == 0 {`, `{ r := struct { Phase struct { Next func() bool } }{}; if r.Phase.Next == nil {} }; if r.Phase.Next == 0 {`},
		{`if r.Phase.Next == 0 {`, `_ = r.Phase.Next; if r.Phase.Next == 0 {`},
		{`if r.Phase.Next == 0 {`, `_ = &r.Phase.Next; if r.Phase.Next == 0 {`},
		{`if r.Phase.Next == 0 {`, `r.Phase.Next = 4; if r.Phase.Next == 0 {`},
		{`if r.Phase.Next == 0 {`, `for _, r.Phase.Next = range []int{4} {}; if r.Phase.Next == 0 {`},
		{`manualReadOnlyWALNativeSchemaOK(inspection.Schema)`, `manualReadOnlyWALNativeSchemaOK(r.Owners[dbID].DB.Schema)`},
		{`inspection, original := r.Owners[dbID].DB.Inspect(r.APICtx, sqlite.InspectOptions{IncludeExtensions: true})`, `inspection, original := unknown.Inspect(r.APICtx, sqlite.InspectOptions{IncludeExtensions: true})`},
		{`h := inspection.Header`, `h := inspection.Header; { inspection := struct { Schema func() }{}; _ = manualReadOnlyWALNativeSchemaOK(inspection.Schema) }`},
		{`h := inspection.Header`, `h := inspection.Header; _ = &inspection.Schema`},
		{`h := inspection.Header`, `h := inspection.Header; inspection.Schema = nil`},
		{`h := inspection.Header`, `h := inspection.Header; for _, inspection.Schema = range [][]sqlite.SchemaEntry{} {}`},
	} {
		readOnlyWALManualReject(t, readOnlyWALManualNativePath, readOnlyWALManualMutation(t, native, mutation[0], mutation[1]), diagnostic)
	}
	readOnlyWALManualReject(t, readOnlyWALManualNativePath, native+"\ntype manualReadOnlyWALPhase struct { Next func() bool }\n", diagnostic)
	readOnlyWALManualReject(t, readOnlyWALManualPurePath, pure+"\ntype int func() bool\n", diagnostic)
	for _, mutation := range [][2]string{
		{`Next                                  int`, `Next                                  func() bool`},
		{`if s.Next != 4 {`, `{ s := struct { Next func() bool }{}; if s.Next == nil {} }; if s.Next != 4 {`},
		{`if s.Next != 4 {`, `_ = unknown.Next; if s.Next != 4 {`},
		{`if s.Next != 4 {`, `_ = &s.Next; if s.Next != 4 {`},
		{`if s.Next != 4 {`, `s.Next = 4; if s.Next != 4 {`},
		{`if s.Next != 4 {`, `for _, s.Next = range []int{4} {}; if s.Next != 4 {`},
		{`s.Next++`, `s.Next--`},
		{`s.Next++`, `s.Next++; s.Next++`},
	} {
		readOnlyWALManualReject(t, readOnlyWALManualPurePath, readOnlyWALManualMutation(t, pure, mutation[0], mutation[1]), diagnostic)
	}
}

func TestReadOnlyWALManualOwnershipMutations(t *testing.T) {
	readOnlyWALManualOwnerFieldMutations(t)
	readOnlyWALManualTerminalReceiverMutations(t)
	readOnlyWALManualQueryCaptureMutations(t)

	raw := readOnlyWALManualFixture(t, readOnlyWALManualNativePath)
	for _, tc := range []struct{ before, after, want string }{
		{`defer manualReadOnlyWALNativeFinish(t, r)`, `defer runtime.UnlockOSThread()`, "gate/ledger-before-acquisition order"},
		{`len(sqlite.PendingCloseCleanups()) != 0 || len(sqlite.PendingPlaintextCleanups()) != 0`, `len(sqlite.PendingPlaintextCleanups()) != 0`, "ownership initial shared/plain pending snapshot gate"},
		{`r.Owners[id].DB, r.Owners[id].Original = db, original`, `r.Owners[id].DB = db`, "ownership exact Open capture"},
		{`capture := manualReadOnlyWALNativeCaptureFD(r, id, fd, original)`, `fmt.Sprintf("observation before capture"); capture := manualReadOnlyWALNativeCaptureFD(r,id,fd,original)`, "ownership immediate original capture-before-observation"},
		{`if err := manualReadOnlyWALBeginEnd(&r.Ledger, id, action); err != nil`, `if err := manualReadOnlyWALBeginEnd(&r.Ledger,id,action); false`, "ownership consuming-before-terminal"},
		{`original = r.Owners[id].DB.Close()`, `original = r.Owners[id].DB.Close(); original = r.Owners[id].DB.Close()`, "ownership/effect exact terminal/acquisition inventory"},
		{`if !next {
		original = r.Owners[id].Rows.Err()
	}`, `if !next { original = r.Owners[id].Rows.Close() }`, "ownership implicit EOF retirement"},
		{`if next {
		return manualReadOnlyWALNativeFail(r, "unexpected third row", nil)
	}`, `if next { return manualReadOnlyWALNativeFail(r,"unexpected third row",nil) }; _ = manualReadOnlyWALNativeEnd(r,id,"close")`, "ownership query and EOF result"},
		{`if commit {
		return manualReadOnlyWALNativeEnd(r, id, "commit")
	}`, `if commit { _ = manualReadOnlyWALNativeEnd(r,id,"commit"); return manualReadOnlyWALNativeEnd(r,id,"rollback") }`, "ownership commit/rollback alternatives"},
		{`child.Parent == id && child.State != manualReadOnlyWALEnded`, `child.Parent == id && false`, "ownership final retirement controls result"},
		{`!r.OutputFailed && !pending && manualReadOnlyWALRetired(r.Ledger)`, `!pending`, "ownership final retirement controls result"},
		{`r.Owners[id].Original, r.Owners[id].Terminal = original, original`, `r.Owners[id].Terminal = nil`, "ownership opaque read cleanup barrier"},
	} {
		readOnlyWALManualReject(t, readOnlyWALManualNativePath, readOnlyWALManualMutation(t, raw, tc.before, tc.after), tc.want)
	}
	openEarly := readOnlyWALManualMutation(t, raw, `id, err := manualReadOnlyWALNativeReserve(r, -1, "fd", label)
	if err != nil {
		return -1, err
	}`, `id, err := 0, error(nil); if err != nil { return -1,err }`)
	openEarly = readOnlyWALManualMutation(t, openEarly, `capture := manualReadOnlyWALNativeCaptureFD(r, id, fd, original)`, `id, err = manualReadOnlyWALNativeReserve(r,-1,"fd",label); if err != nil { return -1,err }; capture := manualReadOnlyWALNativeCaptureFD(r,id,fd,original)`)
	readOnlyWALManualReject(t, readOnlyWALManualNativePath, openEarly, "ownership reserve-before-acquisition order")
	pure := readOnlyWALManualFixture(t, readOnlyWALManualPurePath)
	readOnlyWALManualReject(t, readOnlyWALManualPurePath, readOnlyWALManualMutation(t, pure, `o.State != manualReadOnlyWALLive`, `o.State == manualReadOnlyWALEnded`), "pure ownership dependency/once decision")
	readOnlyWALManualReject(t, readOnlyWALManualPurePath, readOnlyWALManualMutation(t, pure, `if failed {
		o.State = manualReadOnlyWALUnknown
		l.Blocked = true`, `if failed { o.State = manualReadOnlyWALEnded; l.Blocked = false`), "pure ownership failed terminal barrier")
}
func TestReadOnlyWALManualPureClosureMutations(t *testing.T) {
	pure := readOnlyWALManualFixture(t, readOnlyWALManualPurePath)
	for _, imported := range []string{`"os"`, `"time"`, `"runtime"`, `"net"`, `"reflect"`, `"unsafe"`, `"golang.org/x/sys/unix"`, `"github.com/ms-xiaosnow/gosqlite/sqlite"`} {
		readOnlyWALManualReject(t, readOnlyWALManualPurePath, readOnlyWALManualMutation(t, pure, "import (", "import (\n\t"+imported), "pure closure import outside descriptor-free allowlist")
	}
	for _, extra := range []string{`var manualReadOnlyWALInitialized = manualReadOnlyWALMemoryManifest()`, `func init() {}`, `func TestMain() {}`, `func (x manualReadOnlyWALManifest) UnmarshalJSON(raw []byte) error { return nil }`} {
		changed := pure + "\n" + extra + "\n"
		want := "initializer/receiver/generic/callback forbidden"
		if strings.HasPrefix(extra, "var ") {
			want = "initializer ownership forbidden"
		}
		readOnlyWALManualReject(t, readOnlyWALManualPurePath, changed, want)
	}
	for _, tc := range []struct{ before, after, want string }{
		{`if _, exists := m[key]; exists`, `if _, exists := m[key]; false && exists`, "pure strict duplicate/budget decoder"},
		{`if _, ok := object[key]; !ok`, `if _, ok := object[strings.ToLower(key)]; !ok`, "pure exact case-sensitive keys"},
		{`"version run_id phase challenge session generation status"`, `"Version run_id phase challenge session generation status"`, "pure closed receipt shape"},
		{`len(raw) > limit || !utf8.Valid(raw)`, `len(raw) > limit`, "pure exact bytes/UTF8/trailing decoder"},
		{`depth > 8 || *tokens >= 512`, `depth > 80 || *tokens >= 5120`, "pure strict duplicate/budget decoder"},
	} {
		readOnlyWALManualReject(t, readOnlyWALManualPurePath, readOnlyWALManualMutation(t, pure, tc.before, tc.after), tc.want)
	}
}

// These checks have deliberately not been executed as part of source-only
// correction. When separately authorized, they distinguish semantic failures
// from the intentionally held complete-file pins.
func readOnlyWALManualSemanticBaseline(t *testing.T) {
	for _, path := range []string{readOnlyWALManualNativePath, readOnlyWALManualPurePath, "internal/testscope/main.go"} {
		findings, err := inspectReadOnlyWALManualSource(path, []byte(readOnlyWALManualFixture(t, path)))
		if err != nil {
			t.Fatal(err)
		}
		for _, finding := range findings {
			if !strings.Contains(finding, "complete source pin requires independent review") {
				t.Fatal("unmutated semantic source contract", path, finding)
			}
		}
	}
}
func readOnlyWALManualPreReceiptContentMutations(t *testing.T) {
	raw := readOnlyWALManualFixture(t, readOnlyWALManualNativePath)
	const diagnostic = "gate pre-receipt fixture content call closure"
	for _, mutation := range [][2]string{
		{`admission, err := manualReadOnlyWALNativeMetadataNow(r)`, `admission, err := manualReadOnlyWALNativeSnapshotNow(r)`},
		{`admission, err := manualReadOnlyWALNativeMetadataNow(r)`, `_ = manualReadOnlyWALNativeHeader(r); admission, err := manualReadOnlyWALNativeMetadataNow(r)`},
		{`admission, err := manualReadOnlyWALNativeMetadataNow(r)`, `_, _ = unix.Pread(r.Owners[r.Family[0]].FD,nil,0); admission, err := manualReadOnlyWALNativeMetadataNow(r)`},
		{`admission, err := manualReadOnlyWALNativeMetadataNow(r)`, `_, _ = manualReadOnlyWALNativeRead(r,r.Family[0],100); admission, err := manualReadOnlyWALNativeMetadataNow(r)`},
		{`if r.Phase.Next == 0 {
		return snapshot, manualReadOnlyWALNativeFail(r, "fixture content requires a fresh keeper receipt", nil)
	}`, `if false { return snapshot,nil }`},
		{`func manualReadOnlyWALNativeArgv(args []string) error {`, `func manualReadOnlyWALNativeArgv(args []string) error { _, _ = unix.Pread(9,nil,0)`},
	} {
		readOnlyWALManualReject(t, readOnlyWALManualNativePath, readOnlyWALManualMutation(t, raw, mutation[0], mutation[1]), diagnostic)
	}
	// A helper introduced anywhere inside the admission closure must not hide
	// a fixture read behind metadata or evidence-only helper names.
	for _, anchor := range []string{
		`func manualReadOnlyWALNativeMetadataNow(r *manualReadOnlyWALNativeRun) (manualReadOnlyWALNativeSnapshot, error) {`,
		`func manualReadOnlyWALNativeNames(r *manualReadOnlyWALNativeRun) error {`,
		`func manualReadOnlyWALNativeEventWrite(r *manualReadOnlyWALNativeRun, event manualReadOnlyWALNativeEvent) error {`,
	} {
		changed := readOnlyWALManualMutation(t, raw, anchor, anchor+` _ = manualReadOnlyWALNativeHiddenRead(r)`)
		changed += "\nfunc manualReadOnlyWALNativeHiddenRead(r *manualReadOnlyWALNativeRun) error { return manualReadOnlyWALNativeHeader(r) }\n"
		readOnlyWALManualReject(t, readOnlyWALManualNativePath, changed, diagnostic)
	}
}
func readOnlyWALManualCaseContextMutations(t *testing.T) {
	raw := readOnlyWALManualFixture(t, readOnlyWALManualNativePath)
	const diagnostic = "gate fresh fixed case API context"
	for _, mutation := range [][2]string{
		{`apiCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)`, `apiCtx, cancel := context.WithCancel(context.Background())`},
		{`apiCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)`, `apiCtx, cancel := context.WithTimeout(context.Background(),300*time.Second)`},
		{`defer cancel()`, `_ = cancel`},
		{`defer cancel()`, `cancel()`},
		{`defer cancel()`, `if false { defer cancel() }`},
		{`r.APICtx = apiCtx`, `_ = apiCtx`},
		{`r.APICtx = apiCtx`, `r.APICtx = context.Background()`},
		{`r.APICtx = apiCtx`, `r.APICtx = apiCtx; apiCtx = context.Background()`},
	} {
		readOnlyWALManualReject(t, readOnlyWALManualNativePath, readOnlyWALManualMutation(t, raw, mutation[0], mutation[1]), diagnostic)
	}
	for _, api := range []string{
		`sqlite.OpenContext(r.APICtx,`,
		`r.Owners[parent].DB.Query(r.APICtx,`,
		`r.Owners[parent].Tx.Query(r.APICtx,`,
		`r.Owners[dbID].DB.Schema(r.APICtx)`,
		`r.Owners[dbID].DB.Inspect(r.APICtx,`,
		`r.Owners[dbID].DB.Check(r.APICtx,`,
		`r.Owners[dbID].DB.Begin(r.APICtx,`,
		`r.Owners[id].Tx.Commit(r.APICtx)`,
	} {
		readOnlyWALManualReject(t, readOnlyWALManualNativePath, readOnlyWALManualMutation(t, raw, api, strings.Replace(api, "r.APICtx", "context.Background()", 1)), diagnostic)
	}
}
func readOnlyWALManualProcProvenanceMutations(t *testing.T) {
	raw := readOnlyWALManualFixture(t, readOnlyWALManualNativePath)
	for _, mutation := range [][2]string{
		{`if fs.Type != unix.PROC_SUPER_MAGIC`, `if false`},
		{`if fs.Type != unix.PROC_SUPER_MAGIC`, `if fs.Type != unix.EXT4_SUPER_MAGIC`},
		{`unix.Fstatfs(r.Owners[id].FD, &fs); err != nil {
		return nil, manualReadOnlyWALNativeFail(r, "self metadata filesystem provenance", err)
	}`, `unix.Fstatfs(r.Owners[0].FD,&fs); err != nil { return nil,manualReadOnlyWALNativeFail(r,"self metadata filesystem provenance",err) }`},
		{`if err := unix.Fstatfs(r.Owners[id].FD, &fs); err != nil {
		return nil, manualReadOnlyWALNativeFail(r, "self metadata filesystem provenance", err)
	}`, `_ = unix.Fstatfs(r.Owners[id].FD,&fs)`},
		{`var fs unix.Statfs_t`, `_, _ = manualReadOnlyWALNativeRead(r,id,limit); var fs unix.Statfs_t`},
	} {
		readOnlyWALManualReject(t, readOnlyWALManualNativePath, readOnlyWALManualMutation(t, raw, mutation[0], mutation[1]), "profile retained self-proc provenance before read")
	}
}
func readOnlyWALManualControlBindingMutations(t *testing.T) {
	raw := readOnlyWALManualFixture(t, readOnlyWALManualNativePath)
	const diagnostic = "profile control pathname and backing provenance"
	for _, mutation := range [][2]string{
		{`manualReadOnlyWALNativePathMatches(r, unix.AT_FDCWD, r.ControlPath, control)`, `manualReadOnlyWALNativePathMatches(r,unix.AT_FDCWD,r.Manifest.SourceRoot,control)`},
		{`manualReadOnlyWALNativePathMatches(r, r.Owners[r.Control].FD, r.ManifestName, manifest)`, `manualReadOnlyWALNativePathMatches(r,r.Owners[r.Source].FD,r.ManifestName,manifest)`},
		{`manualReadOnlyWALNativePathMatches(r, r.Owners[r.Control].FD, r.ManifestName, manifest)`, `manualReadOnlyWALNativePathMatches(r,r.Owners[r.Control].FD,"readonly.db",manifest)`},
		{`if control != r.ControlStat || manifest != r.ManifestStat`, `if false`},
		{`r.ControlPath, r.ManifestName = filepath.Dir(manifestPath), filepath.Base(manifestPath)`, `r.ControlPath,r.ManifestName = "/tmp",filepath.Base(manifestPath)`},
		{`if err := manualReadOnlyWALNativeControlMatches(r); err != nil {
		return err
	}
	raw, err := manualReadOnlyWALNativeRead(r, manifestFile, 32768)`, `_ = manualReadOnlyWALNativeControlMatches(r)
	raw, err := manualReadOnlyWALNativeRead(r,manifestFile,32768)`},
		{`raw, err := manualReadOnlyWALNativeRead(r, manifestFile, 32768)
	if err != nil {
		return err
	}
	if err := manualReadOnlyWALNativeControlMatches(r); err != nil {
		return err
	}`, `raw, err := manualReadOnlyWALNativeRead(r,manifestFile,32768); if err != nil { return err }
	_ = manualReadOnlyWALNativeControlMatches(r)`},
		{`manualReadOnlyWALNativeObserveObject(r, r.Control, true, mounts)`, `manualReadOnlyWALNativeObserveObject(r, r.Evidence, true, mounts)`},
		{`!manualReadOnlyWALMountValid(control.Mount, control.Identity, control.Mount.ReadOnly)`, `false`},
		{`manualReadOnlyWALNativeBackingPlace(r.ControlPath, control.Mount)`, `manualReadOnlyWALNativeBackingPlace(r.ControlPath,evidence.Mount)`},
		{`manualReadOnlyWALSeparate(controlPlace, sourcePlace)`, `true`},
		{`manualReadOnlyWALSeparate(controlPlace, evidencePlace)`, `true`},
		{`&& separateBacking && separateControlSource && separateControlEvidence`, `&& separateBacking`},
		{`!manualReadOnlyWALSeparate(m.EvidenceParent, r.ControlPath)`, `false`},
		{`&& a.Control == b.Control`, `&& true`},
		{`if !manualReadOnlyWALPath(r.ControlPath)`, `if false`},
		{`sourcePlace != "/" && evidencePlace != "/" && manualReadOnlyWALSeparate(sourcePlace, evidencePlace)`, `manualReadOnlyWALSeparate(sourcePlace, evidencePlace)`},
		{`controlPlace != "/" && sourcePlace != "/" && manualReadOnlyWALSeparate(controlPlace, sourcePlace)`, `manualReadOnlyWALSeparate(controlPlace, sourcePlace)`},
		{`controlPlace != "/" && evidencePlace != "/" && manualReadOnlyWALSeparate(controlPlace, evidencePlace)`, `manualReadOnlyWALSeparate(controlPlace, evidencePlace)`},
	} {
		readOnlyWALManualReject(t, readOnlyWALManualNativePath, readOnlyWALManualMutation(t, raw, mutation[0], mutation[1]), diagnostic)
	}
}
func readOnlyWALManualTerminalReceiverMutations(t *testing.T) {
	raw := readOnlyWALManualFixture(t, readOnlyWALManualNativePath)
	const diagnostic = "ownership terminal receiver bound to ledger id"
	for _, terminal := range []string{
		`unix.Close(r.Owners[id].FD)`, `r.Owners[id].Rows.Close()`,
		`r.Owners[id].Tx.Commit(r.APICtx)`, `r.Owners[id].Tx.Rollback()`,
		`r.Owners[id].DB.Close()`, `r.Owners[id].Rows.Next()`, `r.Owners[id].Rows.Err()`,
	} {
		for _, wrong := range []string{"0", "unknownOwner"} {
			readOnlyWALManualReject(t, readOnlyWALManualNativePath, readOnlyWALManualMutation(t, raw, terminal, strings.Replace(terminal, "[id]", "["+wrong+"]", 1)), diagnostic)
		}
	}
	for _, mutation := range [][2]string{
		{`unix.Close(r.Owners[id].FD)`, `unix.Close(123)`},
		{`r.Owners[id].DB.Close()`, `borrowedDB.Close()`},
		{`manualReadOnlyWALBeginEnd(&r.Ledger, id, action)`, `manualReadOnlyWALBeginEnd(&r.Ledger,0,action)`},
		{`r.Owners[id].Terminal = original`, `r.Owners[0].Terminal = original`},
		{`manualReadOnlyWALFinishEnd(&r.Ledger, id, original != nil)`, `manualReadOnlyWALFinishEnd(&r.Ledger,0,original != nil)`},
	} {
		readOnlyWALManualReject(t, readOnlyWALManualNativePath, readOnlyWALManualMutation(t, raw, mutation[0], mutation[1]), diagnostic)
	}
}
func readOnlyWALManualQueryCaptureMutations(t *testing.T) {
	raw := readOnlyWALManualFixture(t, readOnlyWALManualNativePath)
	const diagnostic = "ownership Query branch immediate capture-before-observation"
	for _, query := range []string{`rows, original = r.Owners[parent].DB.Query(r.APICtx, manualReadOnlyWALQuery)`, `rows, original = r.Owners[parent].Tx.Query(r.APICtx, manualReadOnlyWALQuery)`} {
		changed := readOnlyWALManualMutation(t, raw, query, query+`; _ = manualReadOnlyWALNativeEventWrite(r,manualReadOnlyWALNativeEvent{Kind:"before-query-capture"})`)
		readOnlyWALManualReject(t, readOnlyWALManualNativePath, changed, diagnostic)
		readOnlyWALManualReject(t, readOnlyWALManualNativePath, readOnlyWALManualMutation(t, raw, query, strings.Replace(query, "rows, original", "rows,_", 1)), diagnostic)
	}
	for _, mutation := range [][2]string{
		{`if r.Ledger.Owners[parent].Kind == "db" {
		rows, original`, `if false { rows,original`},
		{`r.Owners[id].Rows, r.Owners[id].Original = rows, original`, `_ = manualReadOnlyWALNativeEventWrite(r,manualReadOnlyWALNativeEvent{Kind:"before-query-capture"}); r.Owners[id].Rows, r.Owners[id].Original = rows,original`},
		{`r.Owners[id].Rows, r.Owners[id].Original = rows, original`, `r.Owners[id].Rows, r.Owners[id].Original = rows,nil`},
	} {
		readOnlyWALManualReject(t, readOnlyWALManualNativePath, readOnlyWALManualMutation(t, raw, mutation[0], mutation[1]), diagnostic)
	}
}
func readOnlyWALManualPreviousPolicyPropagationMutations(t *testing.T) {
	raw := readOnlyWALManualFixture(t, "internal/testscope/main.go")
	const diagnostic = "previous guard wiring weakened"
	for _, mutation := range [][2]string{
		{`for _, f := range findings {`, `findings = nil; for _, f := range findings {`},
		{`for _, f := range findings {`, `findings = findings[:0]; for _, f := range findings {`},
		{`if failed {`, `failed = false; if failed {`},
		{`if failed {`, `savedFailure := &failed; _ = savedFailure; if failed {`},
	} {
		readOnlyWALManualReject(t, "internal/testscope/main.go", readOnlyWALManualMutation(t, raw, mutation[0], mutation[1]), diagnostic)
	}
	cases := []struct{ assignment, result, resultName, errorBlock, errorName string }{
		{`manual, err := inspectBackupManualSource(path, raw)`, `findings = append(findings, manual...)`, "manual", `if err != nil {
			return err
		}`, "err"},
		{`ordinary, inspectErr := inspectSource(path, raw)`, `findings = append(findings, ordinary...)`, "ordinary", `if inspectErr != nil {
				return inspectErr
			}`, "inspectErr"},
		{`publication, err := inspectWALPublicationSource(path, raw)`, `findings = append(findings, publication...)`, "publication", "if err != nil {\n\t\t\t\treturn err\n\t\t\t}", "err"},
		{`startup, err := inspectOracleSource(path, raw)`, `findings = append(findings, startup...)`, "startup", "if err != nil {\n\t\t\t\treturn err\n\t\t\t}", "err"},
		{`contracts, err := inspectOracleContracts(".")`, "for _, finding := range contracts {\n\t\tfmt.Fprintln(os.Stderr, finding)\n\t\tfailed = true\n\t}", "contracts", "if err != nil {\n\t\tfmt.Fprintln(os.Stderr, err)\n\t\tfailed = true\n\t}", "err"},
		{`manualContracts, manualErr := inspectBackupManualContracts(".")`, `for _, finding := range manualContracts {
		fmt.Fprintln(os.Stderr, finding)
		failed = true
	}`, "manualContracts", `if manualErr != nil {
		fmt.Fprintln(os.Stderr, manualErr)
		failed = true
	}`, "manualErr"},
		{`publicationContracts, err := inspectWALPublicationContracts(".")`, "for _, finding := range publicationContracts {\n\t\tfmt.Fprintln(os.Stderr, finding)\n\t\tfailed = true\n\t}", "publicationContracts", "if err != nil {\n\t\tfmt.Fprintln(os.Stderr, err)\n\t\tfailed = true\n\t}", "err"},
		{`darwinManual, err := inspectDarwinInventoryManualContract(".")`, "for _, finding := range darwinManual {\n\t\tfmt.Fprintln(os.Stderr, finding)\n\t\tfailed = true\n\t}", "darwinManual", "if err != nil {\n\t\tfmt.Fprintln(os.Stderr, err)\n\t\tfailed = true\n\t}", "err"},
	}
	for _, policy := range cases {
		start := strings.Index(raw, policy.assignment)
		if start < 0 {
			t.Fatal("prior policy assignment anchor missing", policy.assignment)
		}
		end := strings.Index(raw[start:], policy.result)
		if end < 0 {
			t.Fatal("prior policy result anchor missing", policy.assignment)
		}
		region := raw[start : start+end+len(policy.result)]
		for _, changedRegion := range []string{
			readOnlyWALManualMutation(t, region, policy.result, "_ = "+policy.resultName),
			readOnlyWALManualMutation(t, region, policy.errorBlock, "_ = "+policy.errorName),
			"if false {\n" + region + "\n}",
		} {
			readOnlyWALManualReject(t, "internal/testscope/main.go", readOnlyWALManualMutation(t, raw, region, changedRegion), diagnostic)
		}
	}
}

// Sixteen checkpoint/rekey/encrypted-root/combined/SQL/restart/recovery hooks, fourteen inert counterexamples each. Keep the old
// propagation rows above unchanged and add no Test root or named child.
func readOnlyWALManualCombinedPolicyPropagationMutations(t *testing.T) {
	raw := readOnlyWALManualFixture(t, "internal/testscope/main.go")
	const diagnostic = "previous guard wiring weakened"
	findings, err := inspectReadOnlyWALManualSource("internal/testscope/main.go", []byte(raw))
	if err != nil || len(findings) != 0 {
		t.Fatal("unmutated combined wiring must have zero findings", findings, err)
	}
	cases := []struct {
		assignment, helper, result, resultName, errorBlock, errorName, errorReport, moveBefore, state string
	}{
		{
			assignment:  `checkpoint, checkpointErr := inspectCheckpointRestartSource(path, raw)`,
			helper:      "inspectCheckpointRestartSource",
			result:      `findings = append(findings, checkpoint...)`,
			resultName:  "checkpoint",
			errorBlock:  "if checkpointErr != nil {\n\t\t\treturn checkpointErr\n\t\t}",
			errorName:   "checkpointErr",
			errorReport: `return checkpointErr`,
			moveBefore:  `for _, f := range findings {`,
			state:       "findings",
		},
		{
			assignment:  `checkpointContracts, checkpointContractErr := inspectCheckpointRestartContracts(".")`,
			helper:      "inspectCheckpointRestartContracts",
			result:      "for _, finding := range checkpointContracts {\n\t\tfmt.Fprintln(os.Stderr, finding)\n\t\tfailed = true\n\t}",
			resultName:  "checkpointContracts",
			errorBlock:  "if checkpointContractErr != nil {\n\t\tfmt.Fprintln(os.Stderr, checkpointContractErr)\n\t\tfailed = true\n\t}",
			errorName:   "checkpointContractErr",
			errorReport: `fmt.Fprintln(os.Stderr, checkpointContractErr)`,
			moveBefore:  `contracts, err := inspectOracleContracts(".")`,
			state:       "failed",
		},
		{
			assignment:  `rekey, rekeyErr := inspectRekeyInterruptionSource(path, raw)`,
			helper:      "inspectRekeyInterruptionSource",
			result:      `findings = append(findings, rekey...)`,
			resultName:  "rekey",
			errorBlock:  "if rekeyErr != nil {\n\t\t\treturn rekeyErr\n\t\t}",
			errorName:   "rekeyErr",
			errorReport: `return rekeyErr`,
			moveBefore:  `for _, f := range findings {`,
			state:       "findings",
		},
		{
			assignment:  `rootNegative, rootNegativeErr := inspectEncryptedRootNegativeSource(path, raw)`,
			helper:      "inspectEncryptedRootNegativeSource",
			result:      `findings = append(findings, rootNegative...)`,
			resultName:  "rootNegative",
			errorBlock:  "if rootNegativeErr != nil {\n\t\t\treturn rootNegativeErr\n\t\t}",
			errorName:   "rootNegativeErr",
			errorReport: `return rootNegativeErr`,
			moveBefore:  `for _, f := range findings {`,
			state:       "findings",
		},
		{
			assignment:  `formatUpgrade, formatErr := inspectFormatUpgradeSource(path, raw)`,
			helper:      "inspectFormatUpgradeSource",
			result:      `findings = append(findings, formatUpgrade...)`,
			resultName:  "formatUpgrade",
			errorBlock:  "if formatErr != nil {\n\t\t\treturn formatErr\n\t\t}",
			errorName:   "formatErr",
			errorReport: `return formatErr`,
			moveBefore:  `for _, f := range findings {`,
			state:       "findings",
		},
		{
			assignment:  `rollbackFault, err := inspectRollbackFaultSource(path, raw)`,
			helper:      "inspectRollbackFaultSource",
			result:      `findings = append(findings, rollbackFault...)`,
			resultName:  "rollbackFault",
			errorBlock:  "if err != nil {\n\t\t\treturn err\n\t\t}",
			errorName:   "err",
			errorReport: `return err`,
			moveBefore:  `for _, f := range findings {`,
			state:       "findings",
		},
		{
			assignment:  `rollbackFaultContracts, rollbackFaultErr := inspectRollbackFaultContracts(".")`,
			helper:      "inspectRollbackFaultContracts",
			result:      "for _, finding := range rollbackFaultContracts {\n\t\tfmt.Fprintln(os.Stderr, finding)\n\t\tfailed = true\n\t}",
			resultName:  "rollbackFaultContracts",
			errorBlock:  "if rollbackFaultErr != nil {\n\t\tfmt.Fprintln(os.Stderr, rollbackFaultErr)\n\t\tfailed = true\n\t}",
			errorName:   "rollbackFaultErr",
			errorReport: `fmt.Fprintln(os.Stderr, rollbackFaultErr)`,
			moveBefore:  `contracts, err := inspectOracleContracts(".")`,
			state:       "failed",
		},
		{
			assignment:  `formatContracts, formatErr := inspectFormatUpgradeContracts(".")`,
			helper:      "inspectFormatUpgradeContracts",
			result:      "for _, finding := range formatContracts {\n\t\tfmt.Fprintln(os.Stderr, finding)\n\t\tfailed = true\n\t}",
			resultName:  "formatContracts",
			errorBlock:  "if formatErr != nil {\n\t\tfmt.Fprintln(os.Stderr, formatErr)\n\t\tfailed = true\n\t}",
			errorName:   "formatErr",
			errorReport: `fmt.Fprintln(os.Stderr, formatErr)`,
			moveBefore:  `contracts, err := inspectOracleContracts(".")`,
			state:       "failed",
		},
		{
			assignment:  `rollbackSQL, rollbackSQLErr := inspectRollbackFaultSQLSource(path, raw)`,
			helper:      "inspectRollbackFaultSQLSource",
			result:      `findings = append(findings, rollbackSQL...)`,
			resultName:  "rollbackSQL",
			errorBlock:  "if rollbackSQLErr != nil {\n\t\t\treturn rollbackSQLErr\n\t\t}",
			errorName:   "rollbackSQLErr",
			errorReport: `return rollbackSQLErr`,
			moveBefore:  `for _, f := range findings {`,
			state:       "findings",
		},
		{
			assignment:  `rollbackSQLContracts, rollbackSQLContractErr := inspectRollbackFaultSQLContracts(".")`,
			helper:      "inspectRollbackFaultSQLContracts",
			result:      "for _, finding := range rollbackSQLContracts {\n\t\tfmt.Fprintln(os.Stderr, finding)\n\t\tfailed = true\n\t}",
			resultName:  "rollbackSQLContracts",
			errorBlock:  "if rollbackSQLContractErr != nil {\n\t\tfmt.Fprintln(os.Stderr, rollbackSQLContractErr)\n\t\tfailed = true\n\t}",
			errorName:   "rollbackSQLContractErr",
			errorReport: `fmt.Fprintln(os.Stderr, rollbackSQLContractErr)`,
			moveBefore:  `contracts, err := inspectOracleContracts(".")`,
			state:       "failed",
		},
		{
			assignment:  `restart, restartErr := inspectRollbackRestartSource(path, raw)`,
			helper:      "inspectRollbackRestartSource",
			result:      `findings = append(findings, restart...)`,
			resultName:  "restart",
			errorBlock:  "if restartErr != nil {\n\t\t\treturn restartErr\n\t\t}",
			errorName:   "restartErr",
			errorReport: `return restartErr`,
			moveBefore:  `for _, f := range findings {`,
			state:       "findings",
		},
		{
			assignment:  `restartContracts, restartContractErr := inspectRollbackRestartContracts(".")`,
			helper:      "inspectRollbackRestartContracts",
			result:      "for _, finding := range restartContracts {\n\t\tfmt.Fprintln(os.Stderr, finding)\n\t\tfailed = true\n\t}",
			resultName:  "restartContracts",
			errorBlock:  "if restartContractErr != nil {\n\t\tfmt.Fprintln(os.Stderr, restartContractErr)\n\t\tfailed = true\n\t}",
			errorName:   "restartContractErr",
			errorReport: `fmt.Fprintln(os.Stderr, restartContractErr)`,
			moveBefore:  `contracts, err := inspectOracleContracts(".")`,
			state:       "failed",
		},
		{
			assignment:  `recovery, err := inspectRollbackFaultRecoverySource(path, raw)`,
			helper:      "inspectRollbackFaultRecoverySource",
			result:      `findings = append(findings, recovery...)`,
			resultName:  "recovery",
			errorBlock:  "if err != nil {\n\t\t\treturn err\n\t\t}",
			errorName:   "err",
			errorReport: `return err`,
			moveBefore:  `for _, f := range findings {`,
			state:       "findings",
		},
		{
			assignment:  `rollbackRecoveryContracts, rollbackRecoveryErr := inspectRollbackFaultRecoveryContracts(".")`,
			helper:      "inspectRollbackFaultRecoveryContracts",
			result:      "for _, finding := range rollbackRecoveryContracts {\n\t\tfmt.Fprintln(os.Stderr, finding)\n\t\tfailed = true\n\t}",
			resultName:  "rollbackRecoveryContracts",
			errorBlock:  "if rollbackRecoveryErr != nil {\n\t\tfmt.Fprintln(os.Stderr, rollbackRecoveryErr)\n\t\tfailed = true\n\t}",
			errorName:   "rollbackRecoveryErr",
			errorReport: `fmt.Fprintln(os.Stderr, rollbackRecoveryErr)`,
			moveBefore:  `contracts, err := inspectOracleContracts(".")`,
			state:       "failed",
		},
		{
			assignment:  `rootNegativeContracts, rootNegativeErr := inspectEncryptedRootNegativeContracts(".")`,
			helper:      "inspectEncryptedRootNegativeContracts",
			result:      "for _, finding := range rootNegativeContracts {\n\t\tfmt.Fprintln(os.Stderr, finding)\n\t\tfailed = true\n\t}",
			resultName:  "rootNegativeContracts",
			errorBlock:  "if rootNegativeErr != nil {\n\t\tfmt.Fprintln(os.Stderr, rootNegativeErr)\n\t\tfailed = true\n\t}",
			errorName:   "rootNegativeErr",
			errorReport: `fmt.Fprintln(os.Stderr, rootNegativeErr)`,
			moveBefore:  `contracts, err := inspectOracleContracts(".")`,
			state:       "failed",
		},
		{
			assignment:  `rekeyContracts, rekeyContractErr := inspectRekeyInterruptionContracts(".")`,
			helper:      "inspectRekeyInterruptionContracts",
			result:      "for _, finding := range rekeyContracts {\n\t\tfmt.Fprintln(os.Stderr, finding)\n\t\tfailed = true\n\t}",
			resultName:  "rekeyContracts",
			errorBlock:  "if rekeyContractErr != nil {\n\t\tfmt.Fprintln(os.Stderr, rekeyContractErr)\n\t\tfailed = true\n\t}",
			errorName:   "rekeyContractErr",
			errorReport: `fmt.Fprintln(os.Stderr, rekeyContractErr)`,
			moveBefore:  `contracts, err := inspectOracleContracts(".")`,
			state:       "failed",
		},
	}
	checked := 0
	for _, policy := range cases {
		start := strings.Index(raw, policy.assignment)
		if start < 0 || strings.Count(raw, policy.assignment) != 1 {
			t.Fatal("combined policy assignment must occur exactly once", policy.assignment)
		}
		end := strings.Index(raw[start:], policy.result)
		if end < 0 || strings.Count(raw, policy.result) != 1 {
			t.Fatal("combined policy result must occur exactly once", policy.result)
		}
		region := raw[start : start+end+len(policy.result)]
		reset := policy.state + " = nil"
		if policy.state == "failed" {
			reset = "failed = false"
		}
		for _, changedRegion := range []string{
			"",
			readOnlyWALManualMutation(t, region, policy.helper, "inspectWrongCombinedSource"),
			readOnlyWALManualMutation(t, region, policy.assignment, strings.Replace(policy.assignment, policy.resultName, "wrongFindings", 1)),
			readOnlyWALManualMutation(t, region, policy.result, strings.Replace(policy.result, policy.resultName, "wrongFindings", 1)),
			readOnlyWALManualMutation(t, region, "if "+policy.errorName+" != nil", "if wrongErr != nil"),
			readOnlyWALManualMutation(t, region, policy.errorReport, strings.Replace(policy.errorReport, policy.errorName, "wrongErr", 1)),
			readOnlyWALManualMutation(t, region, policy.errorBlock, "_ = "+policy.errorName),
			"if false {\n" + region + "\n}",
			region + "\n" + region,
			region + "; " + reset,
			readOnlyWALManualMutation(t, region, policy.result, policy.resultName+" = nil; "+policy.result),
			region + "; savedResult := &" + policy.state + "; _ = savedResult",
			policy.assignment + "\n" + policy.result + "\n" + policy.errorBlock,
		} {
			readOnlyWALManualReject(t, "internal/testscope/main.go", readOnlyWALManualMutation(t, raw, region, changedRegion), diagnostic)
			checked++
		}
		// Move the intact, correctly spelled triplet to a different direct
		// position. Merely preserving its text must not preserve approval.
		moved := readOnlyWALManualMutation(t, raw, region, "")
		moved = readOnlyWALManualMutation(t, moved, policy.moveBefore, region+"\n"+policy.moveBefore)
		readOnlyWALManualReject(t, "internal/testscope/main.go", moved, diagnostic)
		checked++
	}
	if len(cases) != 16 || checked != 224 {
		t.Fatal("combined wiring counterexample inventory", len(cases), checked)
	}
}
