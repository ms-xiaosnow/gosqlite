package testoracle

import (
	"os"
	"os/exec"
	"runtime"
	"testing"
)

// ForTest adapts existing test callers without changing their argument or
// process-lifetime handling. This package is imported only by test sources.
// Unlike NewSpec, this adapter reads exactly the two approved temp-root inputs
// and captures the current absolute package directory. It does not create a
// directory, inspect startup configuration, inherit Env, or launch a process.
// Future execution must pin GOTMPDIR/TMPDIR and validate their retained /tmp
// directory and the executable's identity before selecting any oracle test.
func ForTest(t testing.TB, executable string, arguments ...string) *exec.Cmd {
	t.Helper()
	if err := validatePlatform(runtime.GOOS); err != nil {
		t.Fatalf("oracle construction: %v", err)
		return nil
	}
	if err := validateAbsolute("executable", executable); err != nil {
		t.Fatalf("oracle construction: %v", err)
		return nil
	}
	workDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("oracle working directory: %v", err)
		return nil
	}
	tempRoot, err := TempRoot(runtime.GOOS, os.Getenv("GOTMPDIR"), os.Getenv("TMPDIR"))
	if err != nil {
		t.Fatalf("oracle construction: %v", err)
		return nil
	}
	spec, err := NewSpec(runtime.GOOS, executable, tempRoot, workDir, arguments...)
	if err != nil {
		t.Fatalf("oracle construction: %v", err)
		return nil
	}
	cmd, err := spec.Command()
	if err != nil {
		t.Fatalf("oracle construction: %v", err)
		return nil
	}
	return cmd
}
