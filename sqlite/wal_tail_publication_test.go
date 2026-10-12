package sqlite

import (
	"bytes"
	"errors"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"runtime"
	"strings"
	"testing"
)

// 只运行纯策略、值快照及源码断言；不调用启用策略的公开打开入口。
func TestWALTailPublicationMemoryPolicy(t *testing.T) {
	for _, platform := range []string{"linux", "darwin", "windows", "other"} {
		for bits := 0; bits < 128; bits++ {
			enabled, readOnly, exclusive := bits&1 != 0, bits&2 != 0, bits&4 != 0
			shared, encrypted, recovery, powersafe := bits&8 != 0, bits&16 != 0, bits&32 != 0, bits&64 != 0
			err := validateWALTailPublication(enabled, readOnly, exclusive, shared, encrypted, recovery, powersafe, platform)
			allowed := !enabled || !readOnly && exclusive && !shared && !encrypted && !recovery && !powersafe && platform == "linux"
			if (err == nil) != allowed {
				t.Fatalf("platform=%s bits=%d: %v", platform, bits, err)
			}
			if enabled && readOnly && !errors.Is(err, ErrReadOnly) {
				t.Fatal("read-only classification lost", err)
			}
			if err != nil && !readOnly && !errors.Is(err, ErrUnsupported) {
				t.Fatal("unsupported classification lost", err)
			}
		}
	}
}

func TestWALTailPublicationMemorySnapshot(t *testing.T) {
	plain := Options{ReadOnly: true, NoCreate: true, PageSize: 512}
	got, err := snapshotWALTailPublication(plain)
	if err != nil || got.WALTailPublication != nil || !got.ReadOnly || !got.NoCreate || got.PageSize != 512 {
		t.Fatal("nil policy changed existing options", got, err)
	}
	input := Options{WALExclusive: true, WALTailPublication: &WALTailPublicationOptions{}, KeepWALOnClose: true, NoCreate: true, PageSize: 512}
	got, err = snapshotWALTailPublication(input)
	if runtime.GOOS != "linux" {
		if !errors.Is(err, ErrUnsupported) {
			t.Fatal("unsupported platform accepted", err)
		}
		return
	}
	if err != nil || got.WALTailPublication == nil || !got.WALExclusive || !got.KeepWALOnClose || !got.NoCreate || got.PageSize != 512 {
		t.Fatal("policy or ordinary options lost", got, err)
	}
	input.WALTailPublication = nil
	if got.WALTailPublication == nil {
		t.Fatal("caller replacement changed captured policy")
	}
	// 空结构体的不同分配可以有相同地址；复制保证通过源码断言检查。
}

func walPublicationSourceFunctions(t *testing.T, path string) map[string]string {
	t.Helper()
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[string]string)
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			var text bytes.Buffer
			if err := printer.Fprint(&text, set, fn); err != nil {
				t.Fatal(err)
			}
			out[fn.Name.Name] = text.String()
		}
	}
	return out
}

func TestWALTailPublicationSourcePublicBridgeAndNilDefault(t *testing.T) {
	functions := walPublicationSourceFunctions(t, "wal_tail_publication.go")
	snapshot := functions["snapshotWALTailPublication"]
	for _, required := range []string{
		"opts.WALTailPublication != nil", "opts.ReadOnly", "opts.WALExclusive", "opts.WALShared",
		"opts.Encryption != nil", "opts.RollbackRecovery != nil", "opts.WALPowersafeOverwrite", "runtime.GOOS",
		"copy := *opts.WALTailPublication", "opts.WALTailPublication = &copy",
	} {
		if !strings.Contains(snapshot, required) {
			t.Fatal("policy input or value snapshot missing", required)
		}
	}
	if strings.Index(snapshot, "validateWALTailPublication(") > strings.Index(snapshot, "copy :=") ||
		strings.Index(snapshot, "copy :=") > strings.Index(snapshot, "opts.WALTailPublication = &copy") {
		t.Fatal("policy must validate before capturing configuration")
	}
	open := walPublicationSourceFunctions(t, "sqlite.go")["OpenContext"]
	for _, required := range []string{
		"opts, err = snapshotWALTailPublication(opts)",
		"else if opts.WALTailPublication != nil",
		"pager.OpenSessionTailPublicationAdmitted(ctx, path, d.pagerOptions(), plainAdmission)",
		"pager.OpenSessionAdmitted(ctx, path, d.pagerOptions(), plainAdmission)",
	} {
		if strings.Count(open, required) != 1 {
			t.Fatal("explicit single enabled/legacy route missing", required)
		}
	}
	for _, later := range []string{"normalizeConnectionLimits(opts)", "pager.ReservePlainOpen()", "canonicalOpenPath(", "d := &DB{"} {
		if strings.Index(open, "snapshotWALTailPublication(opts)") > strings.Index(open, later) {
			t.Fatal("policy validation moved after acquisition or stored options", later)
		}
	}
	if strings.Index(open, "if opts.WALShared {") > strings.Index(open, "else if opts.WALTailPublication != nil") {
		t.Fatal("shared branch must remain separate")
	}
	session := walPublicationSourceFunctions(t, "../internal/pager/session.go")
	for name, required := range map[string]string{
		"OpenSessionAdmitted": "openSessionAdmitted(ctx, path, opts, admission, false)",
		"OpenSessionTailPublicationAdmitted": "openSessionAdmitted(ctx, path, opts, admission, true)",
	} {
		if strings.Count(session[name], required) != 1 {
			t.Fatal("pager activation/default bridge changed", name)
		}
	}
	if !strings.Contains(session["OpenSession"], "OpenSessionAdmitted(ctx, path, opts, nil)") {
		t.Fatal("legacy constructor must retain disabled path")
	}
}
