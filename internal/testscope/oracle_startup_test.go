package main

import (
	"strings"
	"testing"
)

// Every fixture in this file is source text. The snippets are never compiled,
// loaded as a package, or executed. These tests exercise guard parsing only.
func TestOracleSourceConstructorGuard(t *testing.T) {
	for _, tc := range []struct {
		name, source string
	}{
		{"direct", `package x; import "os/exec"; func f(){exec.Command(exe,"-noinit")}`},
		{"aliased-import", `package x; import child "os/exec"; func f(){child.Command(exe,args...)}`},
		{"function-value", `package x; import "os/exec"; var launch=exec.Command`},
		{"context-value", `package x; import child "os/exec"; var launch=child.CommandContext`},
		{"variadic", `package x; import "os/exec"; func f(){exec.Command(exe,args...)}`},
		{"lookup", `package x; import "os/exec"; var lookup=exec.LookPath`},
		{"dot-import", `package x; import . "os/exec"; func f(){Command(exe)}`},
		{"blank-import", `package x; import _ "os/exec"`},
		{"literal", `package x; import "os/exec"; var cmd=&exec.Cmd{Path:exe}`},
		{"unkeyed-literal", `package x; import "os/exec"; var cmd=exec.Cmd{exe}`},
		{"new", `package x; import "os/exec"; var cmd=new(exec.Cmd)`},
		{"new-pointer", `package x; import "os/exec"; var cmd=new(*exec.Cmd)`},
		{"zero-value", `package x; import "os/exec"; var cmd exec.Cmd`},
		{"alias", `package x; import "os/exec"; type C=exec.Cmd; var cmd=C{}`},
		{"defined-type", `package x; import "os/exec"; type C exec.Cmd; var cmd=C{}`},
		{"pointer-alias", `package x; import "os/exec"; type C=*exec.Cmd; var cmd C`},
		{"implicit-elements", `package x; import "os/exec"; var cmds=[]exec.Cmd{{Path:exe}}`},
		{"implicit-pointer-elements", `package x; import "os/exec"; var cmds=[]*exec.Cmd{{Path:exe}}`},
		{"method-expression", `package x; import "os/exec"; var start=(*exec.Cmd).Start`},
		{"generic-type", `package x; import "os/exec"; var cmd=makeValue[exec.Cmd]()`},
		{"os-start", `package x; import "os"; var launch=os.StartProcess`},
		{"os-alias", `package x; import host "os"; func f(){host.StartProcess(exe,args,attr)}`},
		{"syscall-fork", `package x; import "syscall"; var launch=syscall.ForkExec`},
		{"syscall-start", `package x; import s "syscall"; var launch=s.StartProcess`},
		{"syscall-exec", `package x; import s "syscall"; var launch=s.Exec`},
		{"unix-exec", `package x; import u "golang.org/x/sys/unix"; var launch=u.Exec`},
		{"unix-fork", `package x; import u "golang.org/x/sys/unix"; var launch=u.ForkExec`},
		{"os-dot", `package x; import . "os"; var launch=StartProcess`},
		{"syscall-dot", `package x; import . "syscall"; var launch=ForkExec`},
		{"cgo", "package x\nimport \"C\"\n"},
		{"manual-namespace", "//go:build manual_namespace\n\npackage x; import \"os/exec\"; var launch=exec.Command"},
		{"manual-recovery", "//go:build manual_recovery && linux\n\npackage x; import \"os/exec\"; var launch=exec.Command"},
		{"oracle-tag", "//go:build oracle\n\npackage x; import \"os/exec\"; var launch=exec.Command"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := inspectOracleSource("fixture_test.go", []byte(tc.source))
			if err != nil || len(got) == 0 {
				t.Fatalf("unreviewed constructor accepted: %v %v", got, err)
			}
		})
	}
	for _, source := range []string{
		`package x; import oracle "github.com/ms-xiaosnow/gosqlite/internal/testoracle"; func f(){oracle.ForTest(t,exe,"-version")}`,
		`package x; import "github.com/ms-xiaosnow/gosqlite/internal/testoracle"; func f(){testoracle.ForTest(t,exe,args...)}`,
		`package x; import "os/exec"; type live struct { cmd *exec.Cmd }; func f() *exec.Cmd{return nil}`,
		`package x; import "os/exec"; var failure *exec.ExitError`,
		`package x; const source="exec.Command(exe)"; var other="os.StartProcess" // exec.Cmd{} is inert`,
	} {
		got, err := inspectOracleSource("fixture_test.go", []byte(source))
		if err != nil || len(got) != 0 {
			t.Fatalf("safe source rejected: %v %v", got, err)
		}
	}
	if _, err := inspectOracleSource("broken_test.go", []byte("package")); err == nil {
		t.Fatal("invalid source accepted")
	}
	got, err := inspectOracleSource("located_test.go", []byte("package x\nimport \"os/exec\"\nvar launch=exec.Command"))
	if err != nil || len(got) != 1 || !strings.Contains(got[0], "located_test.go:3:") {
		t.Fatal(got, err)
	}
}

func TestOracleSourceImportAndMutationGuard(t *testing.T) {
	const header = `package x; import "github.com/ms-xiaosnow/gosqlite/internal/testoracle"; `
	for _, tc := range []struct {
		name, path, source string
	}{
		{"production-import", "sqlite/ordinary.go", header + `func f(){testoracle.ForTest(t,exe)}`},
		{"function-alias", "fixture_test.go", header + `var makeCommand=testoracle.ForTest`},
		{"alternate-factory", "fixture_test.go", header + `var makeSpec=testoracle.NewSpec`},
		{"dot-import", "fixture_test.go", `package x; import . "github.com/ms-xiaosnow/gosqlite/internal/testoracle"; var f=ForTest`},
		{"blank-import", "fixture_test.go", `package x; import _ "github.com/ms-xiaosnow/gosqlite/internal/testoracle"`},
		{"environment", "fixture_test.go", header + `func f(){cmd:=testoracle.ForTest(t,exe);cmd.Env=ambient}`},
		{"environment-element", "fixture_test.go", header + `func f(){cmd:=testoracle.ForTest(t,exe);cmd.Env[0]="HOME=/tmp"}`},
		{"environment-alias", "fixture_test.go", header + `func f(){cmd:=testoracle.ForTest(t,exe);values:=cmd.Env;values[0]="HOME=/tmp"}`},
		{"environment-address", "fixture_test.go", header + `func f(){cmd:=testoracle.ForTest(t,exe);mutate(&cmd.Env)}`},
		{"arguments", "fixture_test.go", header + `func f(){cmd:=testoracle.ForTest(t,exe);cmd.Args[1]="-init"}`},
		{"path", "fixture_test.go", header + `func f(){cmd:=testoracle.ForTest(t,exe);cmd.Path=other}`},
		{"cwd", "fixture_test.go", header + `func f(){cmd:=testoracle.ForTest(t,exe);cmd.Dir=other}`},
		{"cancel", "fixture_test.go", header + `func f(){cmd:=testoracle.ForTest(t,exe);cmd.Cancel=cancel}`},
		{"extra-files", "fixture_test.go", header + `func f(){cmd:=testoracle.ForTest(t,exe);cmd.ExtraFiles=files}`},
		{"sysproc", "fixture_test.go", header + `func f(){cmd:=testoracle.ForTest(t,exe);cmd.SysProcAttr=attr}`},
		{"wait-delay", "fixture_test.go", header + `func f(){cmd:=testoracle.ForTest(t,exe);cmd.WaitDelay=delay}`},
		{"new-factory-file", "internal/testoracle/extra.go", `package testoracle; func f(){}`},
		{"new-factory-test", "internal/testoracle/extra_test.go", `package testoracle; func f(){}`},
		{"factory-init", oracleSpecTestPath, `package testoracle; func init(){}`},
		{"factory-testmain", oracleSpecTestPath, `package testoracle; func TestMain(m *testing.M){}`},
		{"factory-start", oracleSpecTestPath, `package testoracle; func f(){cmd.Start()}`},
		{"factory-output", oracleSpecTestPath, `package testoracle; func f(){cmd.Output()}`},
		{"factory-run", oracleSpecTestPath, `package testoracle; func f(){cmd.Run()}`},
		{"factory-environ", oracleSpecTestPath, `package testoracle; func f(){cmd.Environ()}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := inspectOracleSource(tc.path, []byte(tc.source))
			if err != nil || len(got) == 0 {
				t.Fatalf("boundary escaped: %v %v", got, err)
			}
		})
	}
}

func TestOracleSourceFactoryLiteralGuard(t *testing.T) {
	const source = `package testoracle
import "os/exec"
type Spec struct{}
func (s Spec) Command() (*exec.Cmd,error) {
 return &exec.Cmd{Path:checked.Path(),Args:checked.Args(),Env:checked.Env(),Dir:checked.Dir()},nil
}`
	got, err := inspectOracleSource(oracleSpecPath, []byte(source))
	if err != nil || len(got) != 0 {
		t.Fatalf("exact factory shape rejected: %v %v", got, err)
	}
	for _, tc := range []struct {
		name, old, replacement string
	}{
		{"wrong-method", "Command()", "Other()"},
		{"wrong-receiver", "(s Spec)", "(s *Spec)"},
		{"wrong-path", "checked.Path()", "other.Path()"},
		{"wrong-args", "checked.Args()", "ambient"},
		{"wrong-env", "checked.Env()", "nil"},
		{"wrong-dir", "checked.Dir()", `""`},
		{"missing-dir", ",Dir:checked.Dir()", ""},
		{"extra-hook", "Dir:checked.Dir()", "Dir:checked.Dir(),Cancel:cancel"},
		{"duplicate-field", "Dir:checked.Dir()", "Env:checked.Env()"},
		{"unkeyed", "Path:checked.Path()", "checked.Path()"},
		{"nonzero-call-args", "checked.Env()", "checked.Env(ambient)"},
		{"variadic-call", "checked.Env()", "checked.Env(ambient...)"},
		{"wrong-error", "},nil", "},err"},
		{"nested-closure", " return &exec.Cmd", " return func() (*exec.Cmd,error) { return &exec.Cmd"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			changed := strings.Replace(source, tc.old, tc.replacement, 1)
			if tc.name == "nested-closure" {
				changed = strings.Replace(changed, "},nil\n}", "},nil }()\n}", 1)
			}
			got, err := inspectOracleSource(oracleSpecPath, []byte(changed))
			if err != nil || len(got) == 0 {
				t.Fatalf("changed factory shape accepted: %v %v", got, err)
			}
		})
	}
	got, err = inspectOracleSource("internal/other/factory.go", []byte(source))
	if err != nil || len(got) == 0 {
		t.Fatal("factory admitted outside exact source path", got, err)
	}
	if len(inspectOracleFrozenSource(oracleSpecPath, []byte(source))) == 0 {
		t.Fatal("shape-only fixture passed the separate reviewed-source hash contract")
	}
}

func TestOracleSourceNativeExceptionGuard(t *testing.T) {
	for path, calls := range oracleNativeCalls {
		source := `package main; import "os/exec"; func run(binary string){` + strings.Join(calls, ";") + `}`
		got, err := inspectOracleSource(path, []byte(source))
		if err != nil || len(got) != 0 {
			t.Fatalf("reviewed native call shape rejected: %v %v", got, err)
		}
		// A shape match is not sufficient: full native source is hash-pinned.
		if len(inspectOracleFrozenSource(path, []byte(source))) == 0 {
			t.Fatal("truncated native fixture passed source hash contract")
		}
		for _, changed := range []string{
			strings.Replace(source, "exec.Command(binary,", "exec.Command(oracle,", 1),
			strings.Replace(source, "func run(", "func other(", 1),
			source + `; var launch=exec.Command`,
			strings.Replace(source, calls[0], calls[0]+";"+calls[0], 1),
		} {
			got, err := inspectOracleSource(path, []byte(changed))
			if err != nil || len(got) == 0 {
				t.Fatalf("native exception broadened: %v %v", got, err)
			}
		}
		got, err = inspectOracleSource("examples/new/main.go", []byte(source))
		if err != nil || len(got) == 0 {
			t.Fatal("native exception applied to unreviewed path", got, err)
		}
	}
}

func TestOracleSourcePathScope(t *testing.T) {
	for _, path := range []string{"main.go", "./internal/testoracle/spec.go", "internal/testoracle/test.go", "sqlite/ordinary.go", "sqlite/oracle_test.go", "internal/pager/manual_test.go"} {
		if !oracleCurrentGoPath(path) {
			t.Fatalf("current Go source excluded: %s", path)
		}
	}
	for _, path := range []string{"docs/evidence/old/copied.go", "docs/history/copied.go", "scripts/build-oracle.sh", "sqlite/notes.txt"} {
		if oracleCurrentGoPath(path) {
			t.Fatalf("historical/non-Go source included: %s", path)
		}
	}
}
