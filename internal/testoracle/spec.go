// Package testoracle constructs startup-isolated official SQLite commands for
// tests. It never starts a process, discovers an executable, or opens a file.
// Only Linux launch semantics are reviewed; other platforms fail closed.
package testoracle

import (
	"fmt"
	"os/exec"
	"path"
	"strings"
)

// Spec has no exported mutable fields. Accessors and Command return fresh
// slices, so changing one result cannot alter another command's launch policy.
type Spec struct {
	platform   string
	executable string
	tempRoot   string
	workDir    string
	arguments  []string
}

// NewSpec is pure string construction. All paths must be explicit, clean Linux
// absolute paths. TempRoot is restricted to /tmp or a descendant of /tmp.
// These lexical checks do not establish file identity, directory permissions,
// symlink targets, mount layout, or whether SQLite can use the temp directory.
// Those are separate preflight obligations before an authorized execution.
func NewSpec(platform, executable, tempRoot, workDir string, arguments ...string) (Spec, error) {
	if err := validatePlatform(platform); err != nil {
		return Spec{}, err
	}
	for _, item := range []struct {
		name  string
		value string
	}{
		{"executable", executable},
		{"temporary root", tempRoot},
		{"working directory", workDir},
	} {
		if err := validateAbsolute(item.name, item.value); err != nil {
			return Spec{}, err
		}
	}
	if executable == "/" {
		return Spec{}, fmt.Errorf("oracle executable must name a file")
	}
	if tempRoot != "/tmp" && !strings.HasPrefix(tempRoot, "/tmp/") {
		return Spec{}, fmt.Errorf("oracle temporary root must be /tmp or its descendant")
	}
	for _, arg := range arguments {
		if strings.ContainsRune(arg, '\x00') {
			return Spec{}, fmt.Errorf("oracle argument contains NUL")
		}
	}
	return Spec{
		platform:   platform,
		executable: executable,
		tempRoot:   tempRoot,
		workDir:    workDir,
		arguments:  append([]string(nil), arguments...),
	}, nil
}

func validatePlatform(platform string) error {
	if platform != "linux" {
		return fmt.Errorf("oracle startup contract is reviewed only for linux, got %q", platform)
	}
	return nil
}

func validateAbsolute(name, value string) error {
	if value == "" || !path.IsAbs(value) || path.Clean(value) != value || strings.ContainsAny(value, "\\\x00") {
		return fmt.Errorf("oracle %s must be a clean absolute Linux path", name)
	}
	return nil
}

// TempRoot selects the same nonempty GOTMPDIR, then TMPDIR, then /tmp precedence
// used by Go 1.27.2 testing.TempDir on Linux. Values are explicit inputs here;
// this function does not read the environment or create a temporary directory.
// A present but invalid first choice fails instead of silently falling back.
func TempRoot(platform, goTempDir, tempDir string) (string, error) {
	if err := validatePlatform(platform); err != nil {
		return "", err
	}
	root := goTempDir
	if root == "" {
		root = tempDir
	}
	if root == "" {
		root = "/tmp"
	}
	if err := validateAbsolute("temporary root", root); err != nil {
		return "", err
	}
	if root != "/tmp" && !strings.HasPrefix(root, "/tmp/") {
		return "", fmt.Errorf("oracle temporary root must be /tmp or its descendant")
	}
	return root, nil
}

// Path and Dir preserve the supplied executable and package working directory.
func (s Spec) Path() string { return s.executable }
func (s Spec) Dir() string { return s.workDir }

// Args always places -noinit before every caller argument, including -version,
// -help, -cmd, database names, and SQL. No caller argument is reordered or lost.
func (s Spec) Args() []string {
	args := make([]string, 0, len(s.arguments)+2)
	args = append(args, s.executable, "-noinit")
	return append(args, s.arguments...)
}

// Env is an exact nonnil allowlist. Unset keys stay absent, including HOME,
// XDG_*, SQLITE_DEBUG_BREAK (even an empty value), PATH, and loader overrides.
func (s Spec) Env() []string {
	return []string{"LC_ALL=C", "LANG=C", "TZ=UTC", "SQLITE_TMPDIR=" + s.tempRoot, "TMPDIR=" + s.tempRoot}
}

// Command returns an unstarted command with fresh slices. Revalidation rejects
// a zero Spec. A direct literal avoids exec.Command's PATH lookup and GODEBUG
// read. Stdio, pipes, SQL, normal completion, and cleanup remain caller-owned.
// Windows is rejected before construction: os/exec may otherwise inject an
// ambient SYSTEMROOT, which is outside this Linux-only child-env contract.
func (s Spec) Command() (*exec.Cmd, error) {
	checked, err := NewSpec(s.platform, s.executable, s.tempRoot, s.workDir, s.arguments...)
	if err != nil {
		return nil, err
	}
	return &exec.Cmd{
		Path: checked.Path(),
		Args: checked.Args(),
		Env:  checked.Env(),
		Dir:  checked.Dir(),
	}, nil
}
