package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func backupManualSources(t *testing.T) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	for path := range backupManualPins {
		raw, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(path)))
		if err != nil { t.Fatal(err) }
		out[path] = raw
	}
	return out
}
func TestBackupManualSourceContracts(t *testing.T) {
	for path, raw := range backupManualSources(t) {
		findings, err := inspectBackupManualSource(path, raw)
		if err != nil || len(findings) != 0 { t.Fatal(path, findings, err) }
	}
}
func TestBackupManualSourceMutationCounterexamples(t *testing.T) {
	sources := backupManualSources(t)
	// Inert text only. Each variant must parse before rejection counts. These
	// deliberate exact-source contracts reject unknown refactors too; they do
	// not execute any fixture, syscall, end operation, collector or oracle.
	variants := []struct{ name, path, old, replacement string }{
		{"remove manual tag", "sqlite/backup_test.go", "//go:build linux && manual_backup_publication", "//go:build linux"},
		{"or gate", "sqlite/backup_test.go", "//go:build linux && manual_backup_publication", "//go:build linux || manual_backup_publication"},
		{"negated gate", "sqlite/backup_test.go", "//go:build linux && manual_backup_publication", "//go:build linux && !manual_backup_publication"},
		{"oracle missing manual", "sqlite/backup_oracle_test.go", "//go:build oracle && linux && manual_backup_publication", "//go:build oracle && linux"},
		{"helper missing manual", backupManualHelper, "//go:build linux && manual_backup_publication", "//go:build linux"},
		{"missing entry gate", "sqlite/backup_test.go", "run := backupmanual.ForTest(t)", "run := backupmanual.ForTestAfterOpen(t)"},
		{"implicit temp", "sqlite/backup_test.go", "scope.Dir(\"source\")", "t.TempDir()"},
		{"relative database", "sqlite/backup_test.go", "filepath.Join(sourceDir, \"source.db\")", "\"source.db\""},
		{"dropped page size", "sqlite/backup_test.go", "[]uint32{512, 4096, 65536}", "[]uint32{4096, 65536}"},
		{"weaker bytes assertion", "sqlite/backup_test.go", "!bytes.Equal(before, backupBytes(t, scope, path))", "false"},
		{"weaker schema assertion", "sqlite/backup_test.go", "!reflect.DeepEqual(gotSchema, wantSchema)", "false"},
		{"weaker query assertion", "sqlite/backup_test.go", "!reflect.DeepEqual(got, want[i])", "false"},
		{"lost check equality", "sqlite/backup_test.go", "gotCheck != wantCheck", "false"},
		{"serialization of concurrency", "sqlite/backup_test.go", "go func()", "func()"},
		{"automatic DB retry", "sqlite/backup_test.go", "return db", "t.Cleanup(func() { _ = db.Close() }); return db"},
		{"rollback after commit", "sqlite/backup_test.go", "scope.CommitTx(ctx, tx)", "tx.Rollback()"},
		{"direct oracle", "sqlite/backup_oracle_test.go", "testoracle.ForTest(t, exe, append(args, path)...)", "exec.Command(exe, append(args, path)...)"},
		{"oracle env override", "sqlite/backup_oracle_test.go", "cmd.Stdin =", "cmd.Env = nil; cmd.Stdin ="},
		{"oracle CWD override", "sqlite/backup_oracle_test.go", "cmd.Stdin =", "cmd.Dir = \"/tmp\"; cmd.Stdin ="},
		{"oracle readonly removed", "sqlite/backup_oracle_test.go", "args = append(args, \"-readonly\")", "args = append(args, \"-list\")"},
		{"oracle metadata assertion weakened", "sqlite/backup_oracle_test.go", "got != \"ok\\n37\\n12345\\n1|数据库 😀|0001FF\\n2|second|\"", "false"},
		{"extra runtime flag", backupManualHelper, "!reflect.DeepEqual(m.RuntimeArgv, expectedArgv)", "false"},
		{"extra build flag", backupManualHelper, "!reflect.DeepEqual(m.BuildArgv, expectedBuild)", "false"},
		{"lost pre-log retention", backupManualHelper, "s.RetainErrors(err)\n\ts.Record(stage,result)", "s.Record(stage,result)"},
		{"lost concurrent second error", "sqlite/backup_test.go", "scope.RetainErrors(ea, eb)", "scope.RetainErrors(ea)"},
		{"lost oracle second error", "sqlite/backup_oracle_test.go", "scope.RetainErrors(err, outErr)", "scope.RetainErrors(err)"},
		{"missing acknowledgement", backupManualHelper, "os.Getenv(ackKey) != \"1\"", "false"},
		{"arbitrary acknowledgement", backupManualHelper, "os.Getenv(ackKey) != \"1\"", "os.Getenv(ackKey) == \"\""},
		{"root fallback", backupManualHelper, "root := os.Getenv(rootKey)", "root := os.Getenv(rootKey); if root == \"\" { root = \"/tmp\" }"},
		{"relative root repaired", backupManualHelper, "root := os.Getenv(rootKey)", "root, _ := filepath.Abs(os.Getenv(rootKey))"},
		{"shared tmp admitted", backupManualHelper, "|| root == \"/tmp\"", ""},
		{"dirty root admitted", backupManualHelper, "filepath.Clean(value) == value", "true"},
		{"skip instead of fatal", backupManualHelper, "t.Fatalf(\"%s\", message)", "t.Skipf(\"%s\", message)"},
		{"filesystem magic bypass", backupManualHelper, "st.Type != unix.EXT4_SUPER_MAGIC", "false"},
		{"requested mask only", backupManualHelper, "st.Mask&statxMask != statxMask", "false"},
		{"reusable mount ID", backupManualHelper, "unix.STATX_MNT_ID_UNIQUE", "unix.STATX_MNT_ID"},
		{"cross mount accepted", backupManualHelper, "root != nil && !sameMount(held, *root)", "false"},
		{"symlink directory admitted", backupManualHelper, "unix.O_NOFOLLOW|unix.O_NONBLOCK", "unix.O_NONBLOCK"},
		{"CWD mismatch admitted", backupManualHelper, "cwd != filepath.Join(root, \"work\")", "false"},
		{"tmp fallback", backupManualHelper, "os.Getenv(\"TMPDIR\") != filepath.Join(root, \"tmp\")", "false"},
		{"oracle arbitrary root", backupManualHelper, "!strings.HasPrefix(r.root, \"/tmp/\")", "false"},
		{"repeated end", backupManualHelper, "o.state!=live", "false"},
		{"end marked after call", backupManualHelper, "o.state=endAttempted // must precede", "o.state=live // must precede"},
		{"parent with unknown child", backupManualHelper, "child.parent==db && child.state!=endedOK", "false"},
		{"Rows error followed by Close", backupManualHelper, "if err:=rows.Err();err!=nil", "if err:=rows.Err();false"},
		{"unknown owner discarded", backupManualHelper, "r.errors = append(r.errors, err)", "r.errors = nil"},
		{"automatic unlink", backupManualHelper, "s.finished=true", "_ = os.RemoveAll(s.fixture); s.finished=true"},
		{"automatic snapshot", backupManualHelper, "s.finished=true", "snapshotCopy(s.fixture); s.finished=true"},
		{"unknown graph Unwrap", backupManualHelper, "default:\n\t\t\treturn false", "default:\n\t\t\tstack=append(stack,err.(interface{Unwrap() error}).Unwrap())"},
		{"expected refusal arbitrary", backupManualHelper, "!cleanExisting(err)", "false"},
		{"whole environment", backupManualHelper, "m := r.input", "_ = os.Environ(); m := r.input"},
		{"collector child", backupManualHelper, "m := r.input", "_ = exec.Command(\"go\", \"version\"); m := r.input"},
		{"HOME read", backupManualHelper, "m := r.input", "_, _ = os.ReadFile(os.Getenv(\"HOME\")+\"/.sqliterc\"); m := r.input"},
		{"duplicate JSON keys", backupManifestSource, "|| keys[key]", ""},
		{"multiple JSON values", backupManifestSource, "if _, err := d.Token(); err != io.EOF", "if _, err := d.Token(); false"},
		{"unknown JSON fields", backupManifestSource, "d.DisallowUnknownFields()", "_ = d"},
		{"missing manifest hash allowed", backupManualHelper, "string(digest) != fmt.Sprintf(\"%x\", sha256.Sum256(raw))", "false"},
		{"manifest overwrite", backupManualHelper, "unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL", "unix.O_WRONLY|unix.O_CREAT|unix.O_TRUNC"},
		{"unbounded binary", backupManualHelper, "r.hashBinary(m.Executable)", "_, _ = os.ReadFile(m.Executable.Path)"},
		{"binary mismatch fallback", backupManualHelper, "hex.EncodeToString(h.Sum(nil)) != pin.SHA256", "false"},
		{"BuildInfo missing accepted", backupManualHelper, "!ok || bi.GoVersion != m.GoVersion", "false"},
		{"all BuildInfo settings exposed", backupManualHelper, "if allowed[setting.Key]", "if true"},
		{"fixture blob only hash", backupManualHelper, "base64.StdEncoding.EncodeToString(v)", "fmt.Sprintf(\"%x\",sha256.Sum256(v))"},
		{"R1 prepass removed", backupManifestSource, "if err := validateManifestSchema(raw); err != nil { return manifest{}, err }", "_ = raw"},
		{"R1 prepass delayed", backupManifestSource, "if err := validateManifestSchema(raw); err != nil { return manifest{}, err }", "defer validateManifestSchema(raw)"},
		{"R1 folded object keys", backupManifestSource, "object[key]; !ok", "object[strings.ToLower(key)]; !ok"},
		{"R1 unknown field count ignored", backupManifestSource, "len(object) != len(required)", "false"},
		{"R1 native load schema removed", backupManualHelper, "m, err := decodeManifest(raw)", "var m manifest; var err error"},
		{"R1 native sidecar before schema", backupManualHelper, "m, err := decodeManifest(raw)", "_ = r.readExact(filepath.Join(base, \"source-inventory.json\"), 8 << 20, nil); m, err := decodeManifest(raw)"},
		{"R1 decoder native import", backupManifestSource, "\"io\"", "\"io\"; \"os\""},
		{"R1 decoder package mutation", backupManifestSource, "const (", "var schemaMutable = 1; const ("},
		{"R1 portable native effect", backupManifestTests, "func schemaTestPin(path string) schemaTestObject {", "func schemaTestPin(path string) schemaTestObject { _, _ = os.Open(\"/tmp/unapproved\");"},
		{"R1 portable manual constraint", backupManifestSource, "package backupmanual", "//go:build linux && manual_backup_publication\n\npackage backupmanual"},
		{"kill on output overflow", backupManualHelper, "if o.total>outputLimit", "if o.total>outputLimit { _ = unix.Kill(os.Getpid(),9) }; if o.total>outputLimit"},
	}
	if len(variants) != 74 { t.Fatal("frozen mutation count changed", len(variants)) }
	for _, v := range variants {
		t.Run(v.name, func(t *testing.T) {
			original := string(sources[v.path])
			if !strings.Contains(original, v.old) { t.Fatal("mutation anchor missing",v.path,v.old) }
			changed := []byte(strings.Replace(original,v.old,v.replacement,1))
			if _,err:=parser.ParseFile(token.NewFileSet(),v.path,changed,0);err!=nil { t.Fatal("invalid mutation is not credited",err) }
			findings,err:=inspectBackupManualSource(v.path,changed)
			if err!=nil || len(findings)==0 { t.Fatal("mutation accepted",v.name,findings,err) }
			if v.name=="R1 portable manual constraint" {
				found:=false
				for _,finding:=range findings { if strings.Contains(finding,"portable schema source must not have a build constraint") { found=true } }
				if !found { t.Fatal("portable tag variant rejected only by hash") }
			}
			if strings.HasPrefix(v.name,"R1 ") && v.name!="R1 portable manual constraint" {
				set:=token.NewFileSet()
				file,parseErr:=parser.ParseFile(set,v.path,changed,0)
				if parseErr!=nil { t.Fatal(parseErr) }
				var behavior []string
				if v.path==backupManualHelper {
					behavior=inspectBackupManifestLoad(v.path,changed,set,file)
				} else {
					imports:=map[string]string{}
					for _,item:=range file.Imports {
						value,unquoteErr:=strconv.Unquote(item.Path.Value)
						if unquoteErr!=nil { t.Fatal(unquoteErr) }
						name:=filepath.Base(value)
						if item.Name!=nil { name=item.Name.Name }
						imports[name]=value
					}
					behavior=inspectBackupManifestPure(v.path,changed,set,file,imports)
				}
				if len(behavior)==0 { t.Fatal("R1 variant only changed a hash; actual flow/purity check accepted it",v.name) }
			}
		})
	}
	backupManualMountReferenceCounterexamples(t, sources[backupManualHelper])
}
// These fixtures are inert Go source. Parsing is required; type checking and
// native operations are not performed. Count the exact semantic whitelist and
// the actual forbidden-Mount findings, independently of whole-source digests.
func backupManualMountReferenceCounterexamples(t *testing.T, raw []byte) {
	t.Helper()
	variants := []struct {
		name, path, old, replacement, suffix string
		allowed, forbidden int
	}{
		{"Mount metadata baseline", backupManualHelper, "", "", "", 4, 0},
		{"Mount metadata inert comment", backupManualHelper, "", "", "\n// Inert source variant; no metadata operation.\n", 4, 0},
		{"Mount metadata wrong helper path", "sqlite/backup_test.go", "", "", "", 0, 4},
		{"Mount metadata wrong package", backupManualHelper, "package backupmanual", "package changed", "", 0, 4},
		{"Mount metadata wrong identity function", backupManualHelper, "func sameIdentity(a, b fact)", "func otherIdentity(a, b fact)", "", 2, 2},
		{"Mount metadata wrong mount function", backupManualHelper, "func sameMount(a, b fact)", "func otherMount(a, b fact)", "", 2, 2},
		{"Mount metadata method receiver", backupManualHelper, "func sameMount(a, b fact)", "func (a fact) sameMount(b fact)", "", 2, 2},
		{"Mount metadata other operand", backupManualHelper, "a.Mount == b.Mount", "c.Mount == b.Mount", "", 2, 2},
		{"Mount metadata other parameter type", backupManualHelper, "func sameMount(a, b fact)", "type otherFact fact\nfunc sameMount(a, b otherFact)", "", 2, 2},
		{"Mount metadata pointer parameter", backupManualHelper, "func sameMount(a, b fact)", "func sameMount(a, b *fact)", "", 2, 2},
		{"Mount metadata split parameters", backupManualHelper, "func sameMount(a, b fact)", "func sameMount(a fact, b fact)", "", 2, 2},
		{"Mount metadata selector calls", backupManualHelper, "a.Mount == b.Mount", "a.Mount() == b.Mount()", "", 2, 2},
		{"Mount metadata function value", backupManualHelper, "func sameMount(a, b fact) bool { return a.Major == b.Major && a.Minor == b.Minor && a.Mount == b.Mount }", "func sameMount(a, b fact) bool { _ = a.Mount; return a.Major == b.Major && a.Minor == b.Minor && a.Mount == b.Mount }", "", 2, 3},
		{"Mount metadata argument values", backupManualHelper, "func sameMount(a, b fact) bool { return a.Major == b.Major && a.Minor == b.Minor && a.Mount == b.Mount }", "func sameMount(a, b fact) bool { return compare(a.Mount, b.Mount) }", "", 2, 2},
		{"Mount metadata different comparison", backupManualHelper, "a.Mount == b.Mount", "a.Mount != b.Mount", "", 2, 2},
		{"Mount metadata unix package call", backupManualHelper, "", "", "\nfunc mountProbe() { _ = unix.Mount(\"\", \"\", \"\", 0, \"\") }\n", 4, 1},
		{"Mount metadata unix package value", backupManualHelper, "", "", "\nfunc mountProbe() { _ = unix.Mount }\n", 4, 1},
		{"Mount metadata syscall package call", backupManualHelper, "", "", "\nfunc mountProbe() { _ = syscall.Mount(\"\", \"\", \"\", 0, \"\") }\n", 4, 1},
		{"Mount metadata package alias call", backupManualHelper, "\"syscall\"", "mountsys \"syscall\"", "\nfunc mountProbe() { _ = mountsys.Mount(\"\", \"\", \"\", 0, \"\") }\n", 4, 1},
		{"Mount metadata package alias value", backupManualHelper, "\"syscall\"", "mountsys \"syscall\"", "\nfunc mountProbe() { _ = mountsys.Mount }\n", 4, 1},
		{"Mount metadata local function field call", backupManualHelper, "", "", "\nfunc mountProbe() { value := struct { Mount func() error }{}; _ = value.Mount() }\n", 4, 1},
		{"Mount metadata local function field value", backupManualHelper, "", "", "\nfunc mountProbe() { value := struct { Mount func() error }{}; _ = value.Mount }\n", 4, 1},
		{"Mount metadata other fact reference", backupManualHelper, "", "", "\nfunc mountProbe(a fact) { _ = a.Mount }\n", 4, 1},
		{"Mount metadata nested function", backupManualHelper, "func sameMount(a, b fact) bool { return a.Major == b.Major && a.Minor == b.Minor && a.Mount == b.Mount }", "func sameMount(a, b fact) bool { return func(a, b fact) bool { return a.Mount == b.Mount }(a, b) }", "", 2, 2},
		{"Mount metadata wrong field type", backupManualHelper, "Mount uint64", "Mount uintptr", "", 0, 4},
		{"Mount metadata function field type", backupManualHelper, "Mount uint64", "Mount func() uint64", "", 0, 4},
		{"Mount metadata missing field", backupManualHelper, "Mount uint64", "MountID uint64", "", 0, 4},
		{"Mount metadata shadowed uint64", backupManualHelper, "", "", "\ntype uint64 = uint32\n", 0, 4},
		{"Mount metadata import named uint64", backupManualHelper, "\"syscall\"", "uint64 \"syscall\"", "", 0, 4},
		{"Mount metadata fact type alias", backupManualHelper, "type fact struct {", "type fact = struct {", "", 0, 4},
		{"Mount metadata duplicate helper", backupManualHelper, "", "", "\nfunc sameMount(a, b fact) bool { return a.Major == b.Major && a.Minor == b.Minor && a.Mount == b.Mount }\n", 2, 4},
		{"Mount metadata dot import", backupManualHelper, "\"golang.org/x/sys/unix\"", ". \"golang.org/x/sys/unix\"", "", 0, 4},
	}
	if len(variants) != 32 { t.Fatal("frozen Mount metadata variant count changed", len(variants)) }
	for _, v := range variants {
		t.Run(v.name, func(t *testing.T) {
			original := string(raw)
			if !strings.Contains(original, v.old) { t.Fatal("Mount mutation anchor missing", v.name, v.old) }
			changed := []byte(strings.Replace(original, v.old, v.replacement, 1) + v.suffix)
			set := token.NewFileSet()
			file, err := parser.ParseFile(set, v.path, changed, 0)
			if err != nil { t.Fatal("invalid Mount mutation is not credited", err) }
			parents := oracleParents(file)
			allowed, forbidden := 0, 0
			ast.Inspect(file, func(n ast.Node) bool {
				sel, ok := n.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "Mount" { return true }
				if backupManualFactMountRead(v.path, changed, set, file, sel, parents) { allowed++ } else { forbidden++ }
				return true
			})
			if allowed != v.allowed || forbidden != v.forbidden { t.Fatal("Mount semantic classification changed", v.name, allowed, forbidden, v.allowed, v.forbidden) }
			findings, err := inspectBackupManualSource(v.path, changed)
			if err != nil { t.Fatal(err) }
			mountFindings := 0
			pinned := false
			for _, finding := range findings {
				if strings.HasSuffix(finding, ": backup manual contract: forbidden effect/reference Mount") { mountFindings++ }
				if strings.HasSuffix(finding, ": backup manual contract: complete reviewed source shape changed") { pinned = true }
			}
			if mountFindings != v.forbidden { t.Fatal("Mount guard did not independently reject exact references", v.name, findings) }
			if v.name == "Mount metadata baseline" && len(findings) != 0 { t.Fatal("unchanged helper source rejected", findings) }
			if v.name == "Mount metadata inert comment" && !pinned { t.Fatal("whole-source pin was weakened", findings) }
		})
	}
}

func TestBackupManualRejectsCopiedEntriesAndImports(t *testing.T) {
	for name := range backupManualEntries {
		raw:=[]byte("package sqlite_test\nimport \"testing\"\nfunc "+name+"(t *testing.T) {}\n")
		findings,err:=inspectBackupManualSource("sqlite/unreviewed_test.go",raw)
		if err!=nil || len(findings)==0 { t.Fatal(name,findings,err) }
	}
	for _,path:=range []string{"sqlite/unreviewed.go","sqlite/unreviewed_test.go","internal/testkit/backupmanual/unreviewed.go", "internal/testkit/backupmanual/unreviewed_test.go"} {
		raw:=[]byte("package unreviewed\nimport \""+backupManualImport+"\"\n")
		findings,err:=inspectBackupManualSource(path,raw)
		if err!=nil || len(findings)==0 { t.Fatal(path,findings,err) }
	}
	for _,path:=range []string{"internal/testkit/backupmanual/unreviewed.go","internal/testkit/backupmanual/unreviewed_test.go"} {
		findings,err:=inspectBackupManualSource(path,[]byte("package backupmanual\n"))
		if err!=nil { t.Fatal(err) }
		found:=false
		for _,finding:=range findings { if strings.Contains(finding,"unreviewed file in manual helper package") { found=true } }
		if !found { t.Fatal("unreviewed portable file accepted",path,findings) }
	}
}
