package main

import (
	"strings"
	"testing"
)

// These fixtures are Go source strings only; no operation in them is executed.
func TestScopeSourceReferenceGuard(t *testing.T) {
	for _, source := range []string{
		`package x; import "os"; func f(){os.Rename("a","b")}`,
		`package x; import o "os"; var rename=o.Rename`,
		`package x; import "os"; func f(){os.RemoveAll("a")}`,
		`package x; import . "os"; func f(){Rename("a","b")}`,
		`package x; func f(){file.Truncate(0)}`,
		`package x; func f(){file.Chmod(0)}`,
		`package x; func f(){process.Kill()}`,
		`package x; func f(){process.Signal(signal)}`,
		`package x; import "os/exec"; var command=exec.CommandContext`,
		`package x; func f(){RecoverRollback(ctx,path,opts)}`,
		`package x; func f(){pager.RecoverRollback(ctx,path,opts)}`,
		`package x; var replay=RecoverRollback`,
		`package x; var replay=recoverRollbackOpen`,
		`package x; func f(){opts.RollbackRecovery=cfg}`,
		`package x; var opts=Options{RollbackRecovery:cfg}`,
		`package sqlite; var opts=Options{nil,false,false,cfg}`,
		`package x; import s "github.com/ms-xiaosnow/gosqlite/sqlite"; var opts=s.Options{nil,false,false,cfg}`,
		"//go:build manual_namespace || linux\n\npackage x; import \"os\"; func f(){os.Rename(\"a\",\"b\")}",
	} {
		got, err := inspectSource("fixture_test.go", []byte(source))
		if err != nil || len(got) == 0 {
			t.Fatalf("guard failed: %v %v", got, err)
		}
	}
	for _, source := range []string{
		`package x; import "os"; func f(){os.ReadFile("fixture");os.WriteFile("new-copy",validBytes,0600)}`,
		`package x; import "os/exec"; func f(){exec.Command(oracle,"-batch")}`,
		`package x; var _="os.Rename(a,b)" // process.Kill() is a string/comment, not a reference`,
		"//go:build manual_namespace\n\npackage x; import \"os\"; func f(){os.Rename(\"a\",\"b\")}",
		"//go:build manual_recovery && linux\n\npackage x; func f(){RecoverRollback(ctx,path,opts)}",
	} {
		got, err := inspectSource("fixture_test.go", []byte(source))
		if err != nil || len(got) != 0 {
			t.Fatalf("unexpected restriction: %v %v", got, err)
		}
	}
	if _, err := inspectSource("broken_test.go", []byte("package")); err == nil {
		t.Fatal("invalid source accepted")
	}
	got, err := inspectSource("located_test.go", []byte("package x\nfunc f(){process.Kill()}"))
	if err != nil || len(got) != 1 || !strings.Contains(got[0], "located_test.go:2:") {
		t.Fatal(got, err)
	}
}
