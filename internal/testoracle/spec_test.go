package testoracle

import (
	"reflect"
	"strings"
	"testing"
)

// All executable paths below are inert strings. These tests inspect specs and
// unstarted commands only; they never invoke a process or read real env/config.
func TestSpecArguments(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"identity", []string{"-batch", "-bail", ":memory:", "SELECT sqlite_version(),sqlite_source_id();"}},
		{"version", []string{"-version"}},
		{"double-version", []string{"--version"}},
		{"help", []string{"-help"}},
		{"double-help", []string{"--help"}},
		{"readonly-query", []string{"-readonly", "-batch", "-bail", "/tmp/fixture.db", "SELECT 1;"}},
		{"variadic-confCLI", []string{"-batch", "-bail", "-noheader", "-quote", "-readonly", "../../testdata/readonly/page512.db"}},
		{"backup-write", []string{"-batch", "-bail", "-noheader", "-list", "/tmp/backup-source.db"}},
		{"backup-readonly", []string{"-batch", "-bail", "-noheader", "-list", "-readonly", "/tmp/backup.db"}},
		{"limit-two-cmd", []string{"-batch", "-bail", "-json", "-cmd", ".explain off", "-cmd", ".log stderr", ":memory:"}},
		{"live-shared-wal", []string{"-batch", "-noheader", "-list", "/tmp/live.db"}},
		{"live-counters", []string{"-batch", "-header", "-quote", "/tmp/counters.db"}},
		{"existing-noinit-preserved", []string{"-noinit", "-batch", ":memory:"}},
		{"explicit-init-still-suppressed", []string{"-init", "/tmp/inert-sqliterc", "-version"}},
		{"no-arguments", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spec, err := NewSpec("linux", "/tmp/inert/sqlite3", "/tmp/private-root", "/tmp/repo/internal/testkit", tc.args...)
			if err != nil {
				t.Fatal(err)
			}
			want := append([]string{"/tmp/inert/sqlite3", "-noinit"}, tc.args...)
			if !reflect.DeepEqual(spec.Args(), want) {
				t.Fatalf("args = %q, want %q", spec.Args(), want)
			}
			cmd, err := spec.Command()
			if err != nil {
				t.Fatal(err)
			}
			if cmd.Path != spec.Path() || cmd.Dir != spec.Dir() || !reflect.DeepEqual(cmd.Args, want) || !reflect.DeepEqual(cmd.Env, spec.Env()) {
				t.Fatal("command differs from explicit specification")
			}
			if cmd.Env == nil || cmd.Process != nil || cmd.ProcessState != nil || cmd.Cancel != nil || cmd.ExtraFiles != nil || cmd.SysProcAttr != nil || cmd.WaitDelay != 0 || cmd.Err != nil {
				t.Fatal("command has inherited environment or unexpected process state/hooks")
			}
			if cmd.Stdin != nil || cmd.Stdout != nil || cmd.Stderr != nil {
				t.Fatal("constructor changed caller-owned stdio")
			}
		})
	}
}

func TestSpecRejectsInvalidPathsAndPlatforms(t *testing.T) {
	for _, tc := range []struct {
		name, platform, executable, tempRoot, workDir string
	}{
		{"empty-platform", "", "/tmp/sqlite3", "/tmp/private", "/tmp/repo"},
		{"windows", "windows", "C:\\tools\\sqlite3.exe", "C:\\Temp", "C:\\repo"},
		{"windows-linux-paths", "windows", "/tmp/sqlite3", "/tmp/private", "/tmp/repo"},
		{"darwin", "darwin", "/tmp/sqlite3", "/tmp/private", "/tmp/repo"},
		{"empty-executable", "linux", "", "/tmp/private", "/tmp/repo"},
		{"path-name", "linux", "sqlite3", "/tmp/private", "/tmp/repo"},
		{"relative-executable", "linux", "./sqlite3", "/tmp/private", "/tmp/repo"},
		{"root-executable", "linux", "/", "/tmp/private", "/tmp/repo"},
		{"unc-executable", "linux", "//server/share/sqlite3", "/tmp/private", "/tmp/repo"},
		{"drive-executable", "linux", "C:/tools/sqlite3.exe", "/tmp/private", "/tmp/repo"},
		{"drive-relative-executable", "linux", "C:sqlite3.exe", "/tmp/private", "/tmp/repo"},
		{"backslash-executable", "linux", "/tmp/tools\\sqlite3", "/tmp/private", "/tmp/repo"},
		{"nul-executable", "linux", "/tmp/sqlite3\x00other", "/tmp/private", "/tmp/repo"},
		{"dotdot-executable", "linux", "/tmp/tools/../sqlite3", "/tmp/private", "/tmp/repo"},
		{"double-slash-executable", "linux", "/tmp//sqlite3", "/tmp/private", "/tmp/repo"},
		{"trailing-slash-executable", "linux", "/tmp/sqlite3/", "/tmp/private", "/tmp/repo"},
		{"empty-temp", "linux", "/tmp/sqlite3", "", "/tmp/repo"},
		{"relative-temp", "linux", "/tmp/sqlite3", "tmp", "/tmp/repo"},
		{"outside-temp", "linux", "/tmp/sqlite3", "/var/tmp", "/tmp/repo"},
		{"prefix-only-temp", "linux", "/tmp/sqlite3", "/tmp-other", "/tmp/repo"},
		{"dotdot-temp", "linux", "/tmp/sqlite3", "/tmp/private/..", "/tmp/repo"},
		{"nul-temp", "linux", "/tmp/sqlite3", "/tmp/private\x00", "/tmp/repo"},
		{"empty-cwd", "linux", "/tmp/sqlite3", "/tmp/private", ""},
		{"relative-cwd", "linux", "/tmp/sqlite3", "/tmp/private", "internal/testkit"},
		{"unc-cwd", "linux", "/tmp/sqlite3", "/tmp/private", "//server/repo"},
		{"dot-cwd", "linux", "/tmp/sqlite3", "/tmp/private", "/tmp/./repo"},
		{"backslash-cwd", "linux", "/tmp/sqlite3", "/tmp/private", "/tmp/repo\\subdir"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewSpec(tc.platform, tc.executable, tc.tempRoot, tc.workDir, "-version"); err == nil {
				t.Fatal("invalid launch specification accepted")
			}
		})
	}
	if _, err := NewSpec("linux", "/tmp/sqlite3", "/tmp/private", "/tmp/repo", "SELECT\x00 1"); err == nil {
		t.Fatal("NUL argument accepted")
	}
	if cmd, err := (Spec{}).Command(); err == nil || cmd != nil {
		t.Fatal("zero specification produced a command")
	}
}

func TestSpecEnvironmentAndCopies(t *testing.T) {
	args := []string{"-batch", ":memory:", "SELECT 1;"}
	spec, err := NewSpec("linux", "/tmp/inert/sqlite3", "/tmp/private-root", "/tmp/repo", args...)
	if err != nil {
		t.Fatal(err)
	}
	wantArgs := []string{"/tmp/inert/sqlite3", "-noinit", "-batch", ":memory:", "SELECT 1;"}
	wantEnv := []string{"LC_ALL=C", "LANG=C", "TZ=UTC", "SQLITE_TMPDIR=/tmp/private-root", "TMPDIR=/tmp/private-root"}
	if !reflect.DeepEqual(spec.Env(), wantEnv) {
		t.Fatalf("environment = %q", spec.Env())
	}
	seen := map[string]bool{}
	for _, entry := range spec.Env() {
		key, _, ok := strings.Cut(entry, "=")
		if !ok || seen[key] {
			t.Fatalf("invalid or duplicate environment key: %q", entry)
		}
		seen[key] = true
	}
	// These are synthetic hostile key names, not reads of the real environment.
	for _, key := range []string{"SQLITE_DEBUG_BREAK", "HOME", "XDG_CONFIG_HOME", "XDG_STATE_HOME", "VISUAL", "SQLITE_HISTORY", "SQLITE_PS1", "SQLITE_PS2", "LD_PRELOAD", "LD_LIBRARY_PATH", "LD_AUDIT", "PATH", "SYSTEMROOT"} {
		if seen[key] {
			t.Fatalf("unapproved key %q", key)
		}
	}
	args[0] = "-init"
	exposedArgs, exposedEnv := spec.Args(), spec.Env()
	exposedArgs[0], exposedArgs[1], exposedEnv[0] = "/tmp/other", "-init", "SQLITE_DEBUG_BREAK="
	first, err := spec.Command()
	if err != nil {
		t.Fatal(err)
	}
	first.Args[0], first.Args[1], first.Env[0] = "/tmp/other", "-init", "HOME=/tmp/unapproved"
	second, err := spec.Command()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(second.Args, wantArgs) || !reflect.DeepEqual(second.Env, wantEnv) || !reflect.DeepEqual(spec.Args(), wantArgs) || !reflect.DeepEqual(spec.Env(), wantEnv) {
		t.Fatal("caller, accessor, or command slices alias the specification")
	}
}

func TestTempRootPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name, goTempDir, tempDir, want string
		invalid                       bool
	}{
		{"go-first", "/tmp/go", "/tmp/other", "/tmp/go", false},
		{"go-only", "/tmp/go", "", "/tmp/go", false},
		{"tmp-fallback", "", "/tmp/private", "/tmp/private", false},
		{"system-fallback", "", "", "/tmp", false},
		{"shared-root-explicit", "/tmp", "/tmp/private", "/tmp", false},
		{"relative-go-no-fallback", "relative", "/tmp/private", "", true},
		{"outside-go-no-fallback", "/var/tmp", "/tmp/private", "", true},
		{"relative-tmp", "", "relative", "", true},
		{"outside-tmp", "", "/var/tmp", "", true},
		{"unc-tmp", "", "//server/tmp", "", true},
		{"dotdot-tmp", "", "/tmp/a/../b", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := TempRoot("linux", tc.goTempDir, tc.tempDir)
			if (err != nil) != tc.invalid || got != tc.want {
				t.Fatalf("root = %q, err = %v", got, err)
			}
		})
	}
	if _, err := TempRoot("windows", "/tmp/private", ""); err == nil {
		t.Fatal("Windows temp selection accepted without a reviewed Windows contract")
	}
}

func TestSpecPreservesRelativeFixtureCWD(t *testing.T) {
	const cwd = "/tmp/repo/internal/testkit"
	const fixture = "../../testdata/readonly/page4096.db"
	spec, err := NewSpec("linux", "/tmp/inert/sqlite3", "/tmp/private-root", cwd, "-readonly", fixture)
	if err != nil {
		t.Fatal(err)
	}
	cmd, err := spec.Command()
	if err != nil {
		t.Fatal(err)
	}
	if cmd.Dir != cwd || cmd.Args[3] != fixture {
		t.Fatal("relative fixture meaning or fixed package CWD changed")
	}
}
