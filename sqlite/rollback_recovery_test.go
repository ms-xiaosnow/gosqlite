package sqlite

import (
	"bytes"
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Descriptor-free models only. No enabled Options, real Open, native owner,
// journal fixture or production recovery callback is used by these tests.
func TestRollbackRecoveryMemoryPolicy(t *testing.T) {
	for _, platform := range []string{"linux", "darwin", "windows", "other"} {
		for bits := 0; bits < 16; bits++ {
			enabled, readOnly, walMode, encrypted := bits&1 != 0, bits&2 != 0, bits&4 != 0, bits&8 != 0
			err := validateRollbackRecovery(enabled, readOnly, walMode, encrypted, platform)
			allowed := !enabled || !readOnly && !walMode && !encrypted && platform == "linux"
			if (err == nil) != allowed {
				t.Fatalf("platform=%s bits=%d: %v", platform, bits, err)
			}
			if enabled && readOnly && !errors.Is(err, ErrReadOnly) {
				t.Fatal("read-only restriction lost", err)
			}
			if err != nil && !readOnly && !errors.Is(err, ErrUnsupported) {
				t.Fatal("unsupported policy classification lost", err)
			}
		}
	}
	for _, cfg := range []RollbackRecoveryOptions{{}, {MaxJournalBytes: 1234, MaxRecords: 5, MaxDatabaseBytes: 6789}} {
		limits := cfg.pagerOptions().Limits.WithDefaults()
		if cfg.MaxRecords == 0 {
			if limits.MaxJournalBytes != 64<<20 || limits.MaxRecords != 65536 || limits.MaxDatabaseBytes != 1<<40 {
				t.Fatal("defaults changed", limits)
			}
		} else if limits.MaxJournalBytes != cfg.MaxJournalBytes || limits.MaxRecords != cfg.MaxRecords || limits.MaxDatabaseBytes != cfg.MaxDatabaseBytes {
			t.Fatal("caller bounds lost", limits)
		}
	}
}

func TestRollbackRecoveryMemorySingleAttemptSequence(t *testing.T) {
	initial := errors.New("memory trusted initial journal diagnostic")
	secondary := errors.New("memory later diagnostic")
	pending := &Error{Code: CodeIOErr, Op: "memory ownership", Err: errors.Join(ErrPoisoned, secondary)}
	wantInfo := FileInfo{PageSize: 4096, PageCount: 12}
	for _, stage := range []string{"success", "nil-context", "canceled-before", "recovery-error", "recovery-owner", "canceled-after", "probe-error", "probe-owner", "probe-journal", "probe-absence", "late-cancel-success"} {
		t.Run(stage, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var input context.Context = ctx
			if stage == "canceled-before" {
				cancel()
			}
			if stage == "nil-context" {
				input = nil
			}
			var events []string
			info, err := runRollbackRecovery(input, initial, func() error {
				events = append(events, "recover")
				switch stage {
				case "recovery-error":
					return secondary
				case "recovery-owner":
					return pending
				case "canceled-after":
					cancel()
				}
				return nil
			}, func() (FileInfo, error) {
				events = append(events, "probe")
				switch stage {
				case "probe-error":
					return wantInfo, secondary
				case "probe-owner":
					return wantInfo, pending
				case "probe-journal":
					return wantInfo, ErrRecoveryRequired
				case "probe-absence":
					return wantInfo, os.ErrNotExist
				case "late-cancel-success":
					cancel()
				}
				return wantInfo, nil
			})
			wantEvents := []string{"recover", "probe"}
			switch stage {
			case "nil-context", "canceled-before":
				wantEvents = nil
			case "recovery-error", "recovery-owner", "canceled-after":
				wantEvents = []string{"recover"}
			}
			if !reflect.DeepEqual(events, wantEvents) {
				t.Fatal("attempt bound or ordering changed", events)
			}
			if stage == "success" || stage == "late-cancel-success" {
				if err != nil || info != wantInfo {
					t.Fatal("success lost", info, err)
				}
				return
			}
			if info != (FileInfo{}) || !errors.Is(err, initial) {
				t.Fatal("failed open hid initial cause or returned metadata", info, err)
			}
			switch stage {
			case "recovery-error", "probe-error":
				if !errors.Is(err, secondary) {
					t.Fatal("later cause lost", err)
				}
			case "recovery-owner", "probe-owner":
				var got *Error
				if !errors.As(err, &got) || got != pending || !errors.Is(err, ErrPoisoned) {
					t.Fatal("owned diagnostic identity lost", err)
				}
			case "probe-journal":
				if !errors.Is(err, ErrRecoveryRequired) {
					t.Fatal(err)
				}
			case "probe-absence":
				if !errors.Is(err, os.ErrNotExist) {
					t.Fatal(err)
				}
			case "canceled-before", "canceled-after":
				if !errors.Is(err, context.Canceled) {
					t.Fatal(err)
				}
			}
		})
	}
}

func recoverySourceFunctions(t *testing.T, path string) map[string]string {
	t.Helper()
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, path, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[string]string)
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			var b bytes.Buffer
			if err := printer.Fprint(&b, set, fn); err != nil {
				t.Fatal(err)
			}
			out[fn.Name.Name] = b.String()
		}
	}
	return out
}

func TestRollbackRecoverySourcePublicAndDriverBoundaries(t *testing.T) {
	fn := recoverySourceFunctions(t, "rollback_recovery.go")
	bridge := fn["recoverRollbackOpen"]
	for _, required := range []string{
		"if !pager.ReaderProbeRecoveryReady(initial, path)", "return FileInfo{}, initial",
		"pager.RecoverRollback(ctx, path, opts.pagerOptions())", "r, err := pager.Open(path)",
		"if err = r.Close(); err != nil", "return FileInfo{}, err",
	} {
		if !strings.Contains(bridge, required) {
			t.Fatal("production bridge missing", required)
		}
	}
	if strings.Index(bridge, "ReaderProbeRecoveryReady") > strings.Index(bridge, "runRollbackRecovery") || strings.Count(bridge, "pager.RecoverRollback(") != 1 || strings.Count(bridge, "pager.Open(") != 1 {
		t.Fatal("proof or exact attempt bound changed")
	}
	for _, forbidden := range []string{"errors.Is", "OpenContext(", "OpenTx(", "OpenAdmitted(", "RetryPlaintextCleanup(", "for ", "go "} {
		if strings.Contains(bridge, forbidden) {
			t.Fatal("unsafe retry bridge", forbidden)
		}
	}
	open := recoverySourceFunctions(t, "sqlite.go")["OpenContext"]
	for _, required := range []string{"opts, err = snapshotRollbackRecovery(opts)", "opts.RollbackRecovery != nil && pager.ReaderProbeRecoveryReady(err, path)", "recoverRollbackOpen(ctx, path, err, *opts.RollbackRecovery)"} {
		if !strings.Contains(open, required) {
			t.Fatal("public wiring missing", required)
		}
	}
	if strings.Index(open, "snapshotRollbackRecovery(opts)") > strings.Index(open, "pager.ReservePlainOpen()") || strings.Index(open, "snapshotRollbackRecovery(opts)") > strings.Index(open, "canonicalOpenPath(") {
		t.Fatal("policy validation must precede native acquisition")
	}
	if strings.Count(open, "snapshotRollbackRecovery(opts)") != 1 || strings.Count(open, "recoverRollbackOpen(") != 1 ||
		strings.Index(open, "snapshotRollbackRecovery(opts)") > strings.Index(open, "normalizeConnectionLimits(opts)") ||
		strings.Index(open, "snapshotRollbackRecovery(opts)") > strings.Index(open, "d := &DB{") {
		t.Fatal("one immutable policy snapshot must precede normalized/stored connection options")
	}
	snapshot := fn["snapshotRollbackRecovery"]
	for _, required := range []string{"opts.ReadOnly", "opts.Encryption != nil", "opts.WALExclusive || opts.WALShared || opts.WALPowersafeOverwrite || opts.KeepWALOnClose", "runtime.GOOS", "copy := *opts.RollbackRecovery", "opts.RollbackRecovery = &copy"} {
		if !strings.Contains(snapshot, required) {
			t.Fatal("mode restriction or value snapshot missing", required)
		}
	}
	assertRecoveryOptionCopyOrder(t, snapshot, "return opts, nil")
	driver := recoverySourceFunctions(t, "../driver/driver.go")
	connector := driver["NewConnector"]
	for _, required := range []string{"if mode == \"ro\"", "opts.ReadOnly = true", "if mode == \"rw\"", "opts.NoCreate = true", "copy := *opts.RollbackRecovery", "opts.RollbackRecovery = &copy", "return &Connector{path: path, opts: opts}, nil"} {
		if !strings.Contains(connector, required) {
			t.Fatal("driver restriction or copy missing", required)
		}
	}
	assertRecoveryOptionCopyOrder(t, connector, "return &Connector{path: path, opts: opts}, nil")
	if strings.Index(connector, "opts.ReadOnly = true") > strings.Index(connector, "copy := *opts.RollbackRecovery") ||
		strings.Index(connector, "opts.NoCreate = true") > strings.Index(connector, "copy := *opts.RollbackRecovery") {
		t.Fatal("driver URI restrictions must precede captured policy snapshot")
	}
	if !strings.Contains(driver["Connect"], "sqlite.OpenContext(ctx, c.path, c.opts)") {
		t.Fatal("driver bypassed public policy")
	}
	// Count the real production replay bridge. No test invokes any of it.
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), filepath.Clean(entry.Name()), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if sel, ok := n.(*ast.SelectorExpr); ok && sel.Sel.Name == "RecoverRollback" {
				count++
				if entry.Name() != "rollback_recovery.go" {
					t.Fatal("new replay entry point requires review", entry.Name())
				}
			}
			return true
		})
	}
	if count != 1 {
		t.Fatal("physical replay bridge count", count)
	}
}

// Source-only assertion: no enabled Options or Connector is constructed here.
// The public policy has scalar fields, so this dereference makes an independent
// value before the pointer is stored in either Open or the Connector.
func assertRecoveryOptionCopyOrder(t *testing.T, source, result string) {
	t.Helper()
	copy := "copy := *opts.RollbackRecovery"
	store := "opts.RollbackRecovery = &copy"
	if strings.Count(source, copy) != 1 || strings.Count(source, store) != 1 ||
		strings.Index(source, copy) > strings.Index(source, store) ||
		strings.Index(source, store) > strings.Index(source, result) {
		t.Fatal("optional recovery limits must be independently value-copied before storage")
	}
}

func TestRollbackRecoveryMemoryFailureClassification(t *testing.T) {
	initial := ErrRecoveryRequired
	for _, later := range []error{io.ErrUnexpectedEOF, context.Canceled, ErrBusy, ErrPlaintextCleanupPending, ErrCommitUncertain, ErrPoisoned} {
		_, err := runRollbackRecovery(context.Background(), initial, func() error { return later }, func() (FileInfo, error) {
			t.Fatal("probe ran after recovery failure")
			return FileInfo{}, nil
		})
		var detail *Error
		wrapped := wrap("open", err)
		if !errors.As(wrapped, &detail) || detail.Code != CodeIOErr || !errors.Is(wrapped, initial) || !errors.Is(wrapped, later) {
			t.Fatal("error precedence or identity lost", wrapped)
		}
	}
}
