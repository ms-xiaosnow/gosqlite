package encstore

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// Source-only controls read the implementation files. They never open storage,
// execute a target syscall, or stand in for an independent protocol review.
func windowsLifecycleSourceFunction(t *testing.T, path, name string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, path, raw, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == name {
			return string(raw[set.Position(fn.Pos()).Offset:set.Position(fn.End()).Offset])
		}
	}
	t.Fatalf("missing function %s", name)
	return ""
}
func TestWindowsLifecycleSourceIntegration(t *testing.T) {
	bridge := windowsLifecycleSourceFunction(t, "platform_windows.go", "rangeLock")
	for _, fragment := range []string{"f.mu.Lock()", "f.checkLocked()", "offset == lifecycleByte && !unlock", "f.locks.held[lifecycleByte] == 0", "f.locks.admitLifecycle(shared, f.nativeLock)", "!shared && f.locks.held[lifecycleByte] == 1", "f.locks.upgradeLifecycle(f.nativeLock)", "f.locks.change(offset, shared, unlock, f.nativeLock)"} {
		if !strings.Contains(bridge, fragment) {
			t.Fatalf("missing native bridge obligation %q", fragment)
		}
	}
	operation := windowsLifecycleSourceFunction(t, "platform_windows.go", "platformOperation")
	if strings.Contains(operation, "ErrUnsupported") || !strings.Contains(operation, "return supported()") {
		t.Fatal("capability does not expose guarded rekey")
	}
	begin := windowsLifecycleSourceFunction(t, "shared.go", "BeginMaintenance")
	at := 0
	for _, fragment := range []string{"s.AcquireWriter(ctx, nil)", "rangeLock(s.lock, lifecycleByte, false, false)", "s.lifecycleExclusive = true", "s.enterGate(false)", "s.checkAuthority()", "s.leaveGate(false)", "s.EndMaintenance(false)"} {
		next := strings.Index(begin[at:], fragment)
		if next < 0 {
			t.Fatalf("missing or reordered maintenance obligation %q", fragment)
		}
		at += next + len(fragment)
	}
	authority := windowsLifecycleSourceFunction(t, "shared.go", "checkAuthority")
	at = 0
	for _, fragment := range []string{"s.checkOwner()", "s.ns.checked(s.path, os.O_RDONLY)", "verifyRootBytes(raw, s.master, s.opts)", "!bytes.Equal(raw, s.rootBytes)", "return ErrChanged", "s.ns.stable(f, s.path, info)"} {
		next := strings.Index(authority[at:], fragment)
		if next < 0 {
			t.Fatalf("missing or reordered authority obligation %q", fragment)
		}
		at += next + len(fragment)
	}
}
