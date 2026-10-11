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
func sharedAbsenceSources(t *testing.T) map[string]string {
	t.Helper()
	sources := make(map[string]string)
	for _, path := range []string{"shared_session.go", "plain_session_cleanup.go", "shared_open_cleanup.go"} {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		sources[path] = string(raw)
	}
	return sources
}

// The same pure checker handles production source and in-memory negative
// controls. Exact wrapper bodies prohibit work before admission and prove all
// arguments, including the capability bit, reach the inspected implementation.
func sharedAbsenceSourceErrors(sources map[string]string, admission bool) []string {
	var failures []string
	get := func(path, name string) (*token.FileSet, *ast.FuncDecl) {
		set := token.NewFileSet()
		file, err := parser.ParseFile(set, path, sources[path], 0)
		if err != nil {
			failures = append(failures, err.Error())
			return set, nil
		}
		var found *ast.FuncDecl
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == name {
				if found != nil {
					failures = append(failures, "duplicate source function "+name)
				}
				found = fn
			}
		}
		if found == nil {
			failures = append(failures, "missing source function "+name)
		}
		return set, found
	}
	text := func(set *token.FileSet, node ast.Node) string {
		var out bytes.Buffer
		if err := format.Node(&out, set, node); err != nil {
			failures = append(failures, err.Error())
		}
		return out.String()
	}
	body := func(path, name string) (*ast.FuncDecl, string) {
		set, fn := get(path, name)
		if fn == nil || fn.Body == nil {
			return fn, ""
		}
		return fn, text(set, fn.Body)
	}
	for _, tc := range []struct{ name, capability string }{
		{"OpenSharedSessionAdmitted", "false"},
		{"OpenReadOnlySharedSessionAdmitted", "true"},
	} {
		_, got := body("shared_session.go", tc.name)
		want := "{\n\treturn openSharedSessionAdmitted(ctx, path, opts, admission, " + tc.capability + ")\n}"
		if got != want {
			failures = append(failures, tc.name+" lost exact admitted capability dispatch")
		}
	}
	set, open := get("shared_session.go", "openSharedSessionAdmitted")
	if open == nil || open.Body == nil {
		failures = append(failures, "shared implementation body missing")
		return failures
	}
	if text(set, open.Type) != "func(ctx context.Context, path string, opts Options, admission *SharedCloseAdmission, readOnly bool) (result *SharedSession, retErr error)" {
		failures = append(failures, "shared implementation lost forwarded inputs or named cleanup results")
	}
	openBody := text(set, open.Body)
	require := func(where, value string, required ...string) {
		for _, needle := range required {
			if !strings.Contains(value, needle) {
				failures = append(failures, fmt.Sprintf("%s missing %q", where, needle))
			}
		}
	}
	before := func(value, first, second string) {
		a, b := strings.Index(value, first), strings.Index(value, second)
		if a < 0 || b < 0 || a >= b {
			failures = append(failures, fmt.Sprintf("expected %q before %q", first, second))
		}
	}
	if !admission {
		require("shared open provenance", openBody,
			"resolved, initialAbsent, err := resolveNativeReaderPath(path)",
			"else if opts.Create && initialAbsent {",
			"dir, e := resolveNativePath(filepath.Dir(path))",
			"if st, absent, e := nativeLstatForCreate(path + \"-wal\"); e == nil && st.Size() != 0 {",
			"else if e != nil && !absent {\n\t\t\treturn nil, e")
		stable, stableBody := body("shared_session.go", "checkStableLocked")
		require("shared stable journal provenance", stableBody,
			"if _, absent, err := nativeLstatForCreate(s.path + \"-journal\"); err == nil {",
			"else if !absent {\n\t\treturn err")
		probe, probeBody := body("plain_session_cleanup.go", "probePathLocked")
		if probeBody != "{\n\treturn nativeLstatForCreate(path)\n}" {
			failures = append(failures, "transition probe must forward explicit provenance in every scope")
		}
		for _, fn := range []*ast.FuncDecl{open, stable, probe} {
			if fn == nil || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				if selector, ok := node.(*ast.SelectorExpr); ok && selector.Sel.Name == "IsNotExist" {
					failures = append(failures, fn.Name.Name+" regressed to generic NotExist classification")
				}
				return true
			})
		}
		return failures
	}
	before(openBody, "admission.take()", "resolveNativeReaderPath(path)")
	before(openBody, "sharedOpenRetirements.reserve()", "resolveNativeReaderPath(path)")
	before(openBody, "cleanup.failedOpen(retErr)", "resolveNativeReaderPath(path)")
	before(openBody, "openWritableDatabase(path, flags)", "cleanup.main = sharedOpenFileOwner{file: f}")
	before(openBody, "cleanup.main = sharedOpenFileOwner{file: f}", "lockShared(f)")
	before(openBody, "openWALFile(", "cleanup.wal = sharedOpenFileOwner{file: s.walFile}")
	for _, indexOpen := range []string{"walindex.Open(path + \"-shm\")", "walindex.OpenReadOnly(path + \"-shm\")"} {
		before(openBody, "cleanup.wal = sharedOpenFileOwner{file: s.walFile}", indexOpen)
		before(openBody, indexOpen, "cleanup.index = sharedOpenIndexOwner{index: s.index}")
	}
	before(openBody, "cleanup.index = sharedOpenIndexOwner{index: s.index}", "s.checkStableLocked()")
	require("shared admission/transfer/error exit", openBody,
		"} else {\n\t\treturn nil, err\n\t}",
		"defer func() {\n\t\tif result == nil {\n\t\t\tcloseGroup.abandon()\n\t\t}\n\t}()",
		"defer func() {\n\t\tif result != nil {\n\t\t\tcleanup.transfer()\n\t\t} else {\n\t\t\tretErr = cleanup.failedOpen(retErr)\n\t\t}\n\t}()")
	for _, owner := range []string{"index = sharedCloseIndex{s.index}", "wal = sharedCloseFile{s.walFile}", "main = sharedCloseFile{s.file}"} {
		before(openBody, "s.checkStableLocked()", "closeGroup."+owner)
		before(openBody, "closeGroup."+owner, "return s, nil")
	}
	ast.Inspect(open.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		name := ""
		switch fn := call.Fun.(type) {
		case *ast.SelectorExpr:
			name = fn.Sel.Name
		case *ast.Ident:
			name = fn.Name
		}
		switch name {
		case "Close", "closeNativeFile", "Remove", "RemoveAll", "Munmap", "CloseHandle":
			failures = append(failures, "shared opener bypasses managed cleanup owner: "+name)
		}
		return true
	})
	_, failedBody := body("shared_open_cleanup.go", "failedOpen")
	before(failedBody, "g.nested = vfs.CleanupOwners(openErr)", "g.retireLocked()")
	require("failed-open error provenance", failedBody, "g.cause = errors.Join(openErr, err)", "return openErr")
	return failures
}

type sharedAbsenceNegative struct{ name, path, old, replacement string }

func sharedAbsenceCheck(t *testing.T, admission bool, controls []sharedAbsenceNegative) {
	t.Helper()
	sources := sharedAbsenceSources(t)
	if failures := sharedAbsenceSourceErrors(sources, admission); len(failures) != 0 {
		t.Fatal(failures)
	}
	for _, tc := range controls {
		t.Run(tc.name, func(t *testing.T) {
			if strings.Count(sources[tc.path], tc.old) != 1 {
				t.Fatal("negative control must replace exactly one source fragment", tc.old)
			}
			mutated := make(map[string]string, len(sources))
			for path, value := range sources {
				mutated[path] = value
			}
			mutated[tc.path] = strings.Replace(mutated[tc.path], tc.old, tc.replacement, 1)
			if _, err := parser.ParseFile(token.NewFileSet(), tc.path, mutated[tc.path], 0); err != nil {
				t.Fatal("malformed negative control", err)
			}
			if failures := sharedAbsenceSourceErrors(mutated, admission); len(failures) == 0 {
				t.Fatal("source guard accepted negative control", tc.name)
			}
		})
	}
}

func TestSharedAbsenceSourceInitialProvenance(t *testing.T) {
	sharedAbsenceCheck(t, false, []sharedAbsenceNegative{
		{"writer_capability", "shared_session.go", "return openSharedSessionAdmitted(ctx, path, opts, admission, false)", "return openSharedSessionAdmitted(ctx, path, opts, admission, true)"},
		{"reader_capability", "shared_session.go", "return openSharedSessionAdmitted(ctx, path, opts, admission, true)", "return openSharedSessionAdmitted(ctx, path, opts, admission, false)"},
		{"writer_context", "shared_session.go", "return openSharedSessionAdmitted(ctx, path, opts, admission, false)", "return openSharedSessionAdmitted(context.Background(), path, opts, admission, false)"},
		{"reader_admission", "shared_session.go", "return openSharedSessionAdmitted(ctx, path, opts, admission, true)", "return openSharedSessionAdmitted(ctx, path, opts, nil, true)"},
		{"reader_path", "shared_session.go", "return openSharedSessionAdmitted(ctx, path, opts, admission, true)", "return openSharedSessionAdmitted(ctx, \"different\", opts, admission, true)"},
		{"reader_options", "shared_session.go", "return openSharedSessionAdmitted(ctx, path, opts, admission, true)", "return openSharedSessionAdmitted(ctx, path, Options{}, admission, true)"},
		{"wrapper_probe_before_admission", "shared_session.go", "return openSharedSessionAdmitted(ctx, path, opts, admission, false)", "_, _, _ = resolveNativeReaderPath(path); return openSharedSessionAdmitted(ctx, path, opts, admission, false)"},
		{"missing_helper", "shared_session.go", "func openSharedSessionAdmitted(", "func uncheckedSharedSessionAdmitted("},
		{"generic_not_exist", "shared_session.go", "else if opts.Create && initialAbsent {", "else if opts.Create && os.IsNotExist(err) {"},
		{"lost_initial_provenance", "shared_session.go", "else if opts.Create && initialAbsent {", "else if opts.Create {"},
		{"late_wal_error", "shared_session.go", "else if e != nil && !absent {\n\t\t\treturn nil, e", "else if e != nil && !absent {\n\t\t\treturn nil, nil"},
		{"late_journal_error", "shared_session.go", "} else if !absent {\n\t\treturn err", "} else if !absent {\n\t\treturn nil"},
		{"probe_provenance", "plain_session_cleanup.go", "return nativeLstatForCreate(path)", "return nil, false, nil"},
	})
}

func TestSharedAbsenceSourceAdmissionAdoptionAndErrorExit(t *testing.T) {
	sharedAbsenceCheck(t, true, []sharedAbsenceNegative{
		{"dispatch_admission", "shared_session.go", "return openSharedSessionAdmitted(ctx, path, opts, admission, false)", "return openSharedSessionAdmitted(ctx, path, opts, nil, false)"},
		{"missing_reserve", "shared_session.go", "cleanup, err := sharedOpenRetirements.reserve()", "cleanup, err := uncheckedReserve()"},
		{"named_error_result", "shared_session.go", "(result *SharedSession, retErr error)", "(*SharedSession, error)"},
		{"missing_admission_abandon", "shared_session.go", "closeGroup.abandon()", "closeGroup.uncheckedAbandon()"},
		{"missing_cleanup_defer", "shared_session.go", "defer func() {\n\t\tif result != nil {", "func() {\n\t\tif result != nil {"},
		{"missing_main_adoption", "shared_session.go", "cleanup.main = sharedOpenFileOwner{file: f}", "cleanup.main = nil"},
		{"missing_wal_adoption", "shared_session.go", "cleanup.wal = sharedOpenFileOwner{file: s.walFile}", "cleanup.wal = nil"},
		{"missing_index_adoption", "shared_session.go", "cleanup.index = sharedOpenIndexOwner{index: s.index}", "cleanup.index = nil"},
		{"readonly_index_before_wal_adoption", "shared_session.go", "cleanup.wal = sharedOpenFileOwner{file: s.walFile}", "_, _ = walindex.OpenReadOnly(path + \"-shm\"); cleanup.wal = sharedOpenFileOwner{file: s.walFile}"},
		{"missing_transfer", "shared_session.go", "cleanup.transfer()", "cleanup.uncheckedTransfer()"},
		{"missing_close_index", "shared_session.go", "closeGroup.index = sharedCloseIndex{s.index}", "closeGroup.index = nil"},
		{"missing_close_wal", "shared_session.go", "closeGroup.wal = sharedCloseFile{s.walFile}", "closeGroup.wal = nil"},
		{"missing_close_main", "shared_session.go", "closeGroup.main = sharedCloseFile{s.file}", "closeGroup.main = nil"},
		{"lost_nested_owner", "shared_open_cleanup.go", "g.nested = vfs.CleanupOwners(openErr)", "g.nested = nil"},
		{"lost_late_diagnostic", "shared_open_cleanup.go", "return openErr", "return nil"},
		{"direct_destructor", "shared_session.go", "cleanup.main = sharedOpenFileOwner{file: f}", "f.Close(); cleanup.main = sharedOpenFileOwner{file: f}"},
	})
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
