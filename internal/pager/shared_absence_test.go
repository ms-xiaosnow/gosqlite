package pager

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/ms-xiaosnow/gosqlite/internal/vfs"
)

// Absence regressions use source ASTs and descriptor-free owner/error laws.
// They never create a missing selected file or inject a native operation error.
func sharedAbsenceSource(t *testing.T, path, name string) (*token.FileSet, *ast.FuncDecl) {
	t.Helper()
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == name {
			return set, fn
		}
	}
	t.Fatalf("missing source function %s", name)
	return nil, nil
}

func sharedAbsenceText(t *testing.T, set *token.FileSet, node ast.Node) string {
	t.Helper()
	var out bytes.Buffer
	if err := format.Node(&out, set, node); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func TestSharedAbsenceSourceInitialProvenance(t *testing.T) {
	set, open := sharedAbsenceSource(t, "shared_session.go", "OpenSharedSessionAdmitted")
	body := sharedAbsenceText(t, set, open.Body)
	for _, required := range []string{
		"resolved, initialAbsent, err := resolveNativeReaderPath(path)",
		"else if opts.Create && initialAbsent {",
		"dir, e := resolveNativePath(filepath.Dir(path))",
		"if st, absent, e := nativeLstatForCreate(path + \"-wal\"); e == nil && st.Size() != 0 {",
		"else if e != nil && !absent {\n\t\t\treturn nil, e",
	} {
		if !strings.Contains(body, required) {
			t.Errorf("shared open must preserve initial absence provenance: missing %q", required)
		}
	}
	set, stable := sharedAbsenceSource(t, "shared_session.go", "checkStableLocked")
	body = sharedAbsenceText(t, set, stable.Body)
	for _, required := range []string{
		"if _, absent, err := nativeLstatForCreate(s.path + \"-journal\"); err == nil {",
		"else if !absent {\n\t\treturn err",
	} {
		if !strings.Contains(body, required) {
			t.Errorf("shared stable journal must preserve initial absence provenance: missing %q", required)
		}
	}
	// The shared opener calls checkJournalLocked on a private Session without
	// a plaintext cleanup scope. Its probe must not revert to error matching.
	set, probe := sharedAbsenceSource(t, "plain_session_cleanup.go", "probePathLocked")
	if got := sharedAbsenceText(t, set, probe.Body); got != "{\n\treturn nativeLstatForCreate(path)\n}" {
		t.Errorf("transition probe must forward explicit provenance in every scope: %s", got)
	}
	for _, fn := range []*ast.FuncDecl{open, stable, probe} {
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			if selector, ok := node.(*ast.SelectorExpr); ok && selector.Sel.Name == "IsNotExist" {
				t.Errorf("%s regressed to generic NotExist classification", fn.Name.Name)
			}
			return true
		})
	}
}

func TestSharedAbsenceSourceAdmissionAdoptionAndErrorExit(t *testing.T) {
	set, open := sharedAbsenceSource(t, "shared_session.go", "OpenSharedSessionAdmitted")
	body := sharedAbsenceText(t, set, open.Body)
	before := func(first, second string) {
		t.Helper()
		a, b := strings.Index(body, first), strings.Index(body, second)
		if a < 0 || b < 0 || a >= b {
			t.Errorf("expected %q before %q", first, second)
		}
	}
	before("admission.take()", "resolveNativeReaderPath(path)")
	before("sharedOpenRetirements.reserve()", "resolveNativeReaderPath(path)")
	before("cleanup.failedOpen(retErr)", "resolveNativeReaderPath(path)") // installed defer
	before("cleanup.main = sharedOpenFileOwner{file: f}", "lockShared(f)")
	before("cleanup.wal = sharedOpenFileOwner{file: s.walFile}", "walindex.Open(path + \"-shm\")")
	before("cleanup.index = sharedOpenIndexOwner{index: s.index}", "s.checkStableLocked()")
	for _, required := range []string{
		"} else {\n\t\treturn nil, err\n\t}",
		"closeGroup.index = sharedCloseIndex{s.index}",
		"closeGroup.wal = sharedCloseFile{s.walFile}",
		"closeGroup.main = sharedCloseFile{s.file}",
	} {
		if !strings.Contains(body, required) {
			t.Errorf("shared admission/transfer/error exit lost %q", required)
		}
	}
	set, failed := sharedAbsenceSource(t, "shared_open_cleanup.go", "failedOpen")
	body = sharedAbsenceText(t, set, failed.Body)
	before("g.nested = vfs.CleanupOwners(openErr)", "g.retireLocked()")
}

func TestSharedAbsenceMemoryLateDiagnosticSurvivesFailedOpen(t *testing.T) {
	for _, wrapped := range []bool{false, true} {
		t.Run(fmt.Sprint(wrapped), func(t *testing.T) {
			r, g := memoryOpenGroup(t, 1)
			var events []string
			main := &memoryOpenOwner{name: "main", events: &events}
			g.main = main
			late := &os.PathError{Op: "memory inspection", Path: "memory", Err: os.ErrNotExist}
			var cause error = late
			if wrapped {
				cause = fmt.Errorf("memory resolver: %w", cause)
			}
			got := g.failedOpen(cause)
			var detail *os.PathError
			if got != cause || !errors.Is(got, os.ErrNotExist) || !errors.As(got, &detail) || detail != late || !g.done || len(r.groups) != 0 || !reflect.DeepEqual(events, []string{"main"}) {
				t.Fatal("completed cleanup erased or changed late absence diagnostic", got, events)
			}
		})
	}
}

func TestSharedAbsenceMemoryNestedTerminalProtectsOwners(t *testing.T) {
	r, g := memoryOpenGroup(t, 1)
	var events []string
	g.index = &memoryOpenOwner{name: "index", events: &events}
	g.wal = &memoryOpenOwner{name: "wal", events: &events}
	g.main = &memoryOpenOwner{name: "main", events: &events}
	late := &os.PathError{Op: "memory metadata", Path: "memory", Err: os.ErrNotExist}
	// Compose the VFS collection result with the actual pager retirement state
	// machine using a fake nested owner. The separate VFS absence law verifies
	// real error-carried terminal discovery without acquiring a descriptor.
	cause := nativeFileError(errors.Join(late, vfs.ErrPoisoned))
	childEvents := []string{}
	child := &memoryOpenOwner{name: "metadata", remaining: 1, terminal: true, failure: cause, events: &childEvents}
	g.nested = []vfs.CleanupOwner{child}
	g.cause = errors.Join(cause, g.retireLocked())
	var pending error = &sharedOpenCleanupError{group: g, cause: g.cause}
	for i := 0; i < 3; i++ {
		var detail *os.PathError
		if !errors.Is(pending, os.ErrNotExist) || !errors.Is(pending, vfs.ErrPoisoned) || !errors.Is(pending, ErrOpenCleanupPending) || !errors.As(pending, &detail) || detail != late {
			t.Fatal("metadata ownership or cause dropped", pending)
		}
		if g.done || len(r.groups) != 1 || len(g.nested) == 0 || len(events) != 0 {
			t.Fatal("error-carried metadata retired its protecting owners", events)
		}
		if _, err := r.reserve(); !errors.Is(err, ErrLimit) {
			t.Fatal("terminal obligation stopped counting toward admission", err)
		}
		pending = RetryOpenCleanup(pending)
	}
	if child.rawCalls != 1 {
		t.Fatal("terminal metadata invented another raw retirement")
	}
}
