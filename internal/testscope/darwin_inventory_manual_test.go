package main

import (
	"go/build/constraint"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These checks parse inert source strings. They never build or invoke encstore.
func darwinInventoryManualFixture(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "encstore", "inventory_manual_darwin_test.go"))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func darwinInventoryManualReject(t *testing.T, path string, raw []byte, want string) {
	t.Helper()
	findings, err := inspectDarwinInventoryManualSource(path, raw)
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range findings {
		if strings.Contains(finding, want) {
			return
		}
	}
	t.Fatalf("missing rejection %q: %v", want, findings)
}

func TestDarwinInventoryManualSourceContract(t *testing.T) {
	raw := darwinInventoryManualFixture(t)
	findings, err := inspectDarwinInventoryManualSource(darwinInventoryManualPath, raw)
	if err != nil || len(findings) != 0 {
		t.Fatal(findings, err)
	}
	// The old general guard still sees this source, without a new exemption.
	findings, err = inspectSource(darwinInventoryManualPath, raw)
	if err != nil || len(findings) != 0 {
		t.Fatal(findings, err)
	}
}

func TestDarwinInventoryManualDefaultExclusion(t *testing.T) {
	raw := darwinInventoryManualFixture(t)
	expr, err := constraint.Parse(strings.SplitN(string(raw), "\n", 2)[0])
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{os string; manual, want bool}{
		{"darwin", false, false}, {"linux", false, false}, {"windows", false, false},
		{"linux", true, false}, {"windows", true, false}, {"darwin", true, true},
	} {
		got := expr.Eval(func(tag string) bool { return tag == tc.os || tag == "manual_darwin_inventory" && tc.manual })
		if got != tc.want {
			t.Fatalf("os=%s manual=%t got=%t want=%t", tc.os, tc.manual, got, tc.want)
		}
	}
}

func TestDarwinInventoryManualGateOrderMutations(t *testing.T) {
	raw := string(darwinInventoryManualFixture(t))
	for _, changed := range []string{
		strings.Replace(raw, "GOSQLITE_MANUAL_DARWIN_INVENTORY\") != \"1\"", "GOSQLITE_MANUAL_DARWIN_INVENTORY\") != \"yes\"", 1),
		strings.Replace(raw, "func TestManualDarwinUnicodeInventory(t *testing.T) {", "func TestManualDarwinUnicodeInventory(t *testing.T) { t.Log(\"before gate\")", 1),
		strings.Replace(raw, "GOSQLITE_MANUAL_DARWIN_INVENTORY_ROOT", "UNREVIEWED_ROOT", 1),
		strings.Replace(raw, "t.Fatal(\"explicit GOSQLITE", "t.Skip(\"explicit GOSQLITE", 1),
	} {
		if changed == raw {
			t.Fatal("inert mutation did not change source")
		}
		darwinInventoryManualReject(t, darwinInventoryManualPath, []byte(changed), "first-action gate/root order")
	}
	// R1 controls remain inert source strings. Require the dedicated semantic
	// diagnostic, so the source hash mismatch alone cannot satisfy these cases.
	initialBlock := "\tencryptedCleanups.mu.Lock()\n\tinitialGroups, initialLeases := len(encryptedCleanups.groups), encryptedCleanups.leases\n\tencryptedCleanups.mu.Unlock()\n\tif initialGroups != 0 || initialLeases != 0 {\n\t\tt.Fatalf(\"initial registry must be empty: groups=%d leases=%d; existing ownership untouched\", initialGroups, initialLeases)\n\t}\n"
	without := strings.Replace(raw, initialBlock, "", 1)
	late := strings.Replace(without, "\tif !l.known(t) {", initialBlock+"\tif !l.known(t) {", 1)
	for _, changed := range []string{
		without,
		late,
		strings.Replace(raw, "initialGroups != 0 || initialLeases != 0", "initialGroups != 0", 1),
		strings.Replace(raw, "initialGroups != 0 || initialLeases != 0", "initialLeases != 0", 1),
		strings.Replace(raw, "initialGroups != 0 || initialLeases != 0", "initialGroups != 0 && initialLeases != 0", 1),
		strings.Replace(raw, "initialGroups, initialLeases := len(encryptedCleanups.groups), encryptedCleanups.leases", "initialGroups, initialLeases := 0, encryptedCleanups.leases", 1),
		strings.Replace(raw, "initialGroups, initialLeases := len(encryptedCleanups.groups), encryptedCleanups.leases", "initialGroups, initialLeases := len(encryptedCleanups.groups), 0", 1),
		strings.Replace(raw, "t.Fatalf(\"initial registry must be empty:", "t.Logf(\"initial registry must be empty:", 1),
	} {
		if changed == raw {
			t.Fatal("inert initial-registry mutation did not change source")
		}
		darwinInventoryManualReject(t, darwinInventoryManualPath, []byte(changed), "initial zero-group/zero-lease gate before ownership required")
	}
}

func TestDarwinInventoryManualNativeCallMutations(t *testing.T) {
	raw := string(darwinInventoryManualFixture(t))
	for _, expression := range []string{
		"unix.Unlink(\"forbidden\")", "os.RemoveAll(\"forbidden\")", "os.Chmod(\"forbidden\", 0700)",
		"unix.Kill(1, 0)", "t.TempDir()", "RetryCleanup(nil)", "exec.Command(\"forbidden\")",
	} {
		changed := strings.Replace(raw, "childName := fmt.Sprintf", expression+"; childName := fmt.Sprintf", 1)
		darwinInventoryManualReject(t, darwinInventoryManualPath, []byte(changed), "unreviewed actual call")
	}
	// Metadata selectors are not native effects. Only the intentional source-pin
	// mismatch should reject this otherwise approved logging call.
	metadata := strings.Replace(raw, "childName := fmt.Sprintf", "t.Log(rootFS.Mntonname); childName := fmt.Sprintf", 1)
	findings, err := inspectDarwinInventoryManualSource(darwinInventoryManualPath, []byte(metadata))
	if err != nil {
		t.Fatal(err)
	}
	for _, finding := range findings {
		if strings.Contains(finding, "unreviewed actual call") || strings.Contains(finding, "actual call count changed") {
			t.Fatalf("metadata selector falsely classified as an effect: %v", findings)
		}
	}
	darwinInventoryManualReject(t, darwinInventoryManualPath, []byte(metadata), "reviewed source pin mismatch")
	changed := strings.Replace(raw, "root, err := openDirectory(rootPath)", "root, err := unreviewedOpen(rootPath)", 1)
	darwinInventoryManualReject(t, darwinInventoryManualPath, []byte(changed), "reviewed actual call count changed: openDirectory")
}

func TestDarwinInventoryManualIdentityMutations(t *testing.T) {
	raw := darwinInventoryManualFixture(t)
	darwinInventoryManualReject(t, "internal/encstore/ordinary_test.go", raw, "exact reviewed source path")
	for _, tag := range []string{"//go:build darwin || manual_darwin_inventory", "//go:build manual_darwin_inventory", "//go:build darwin"} {
		changed := strings.Replace(string(raw), darwinInventoryManualTag, tag, 1)
		darwinInventoryManualReject(t, darwinInventoryManualPath, []byte(changed), "exact darwin AND manual tag")
	}
	changed := strings.Replace(string(raw), "TestManualDarwinUnicodeInventory", "TestUnreviewedDarwinInventory", 1)
	darwinInventoryManualReject(t, darwinInventoryManualPath, []byte(changed), "unique entry")
	changed = strings.Replace(string(raw), "\"R01\"", "\"R02\"", 1)
	darwinInventoryManualReject(t, darwinInventoryManualPath, []byte(changed), "reviewed source pin mismatch")
}

func TestDarwinInventoryManualNoInitMutations(t *testing.T) {
	raw := string(darwinInventoryManualFixture(t))
	darwinInventoryManualReject(t, darwinInventoryManualPath, []byte(raw+"\nfunc init() {}\n"), "init is forbidden")
	darwinInventoryManualReject(t, darwinInventoryManualPath, []byte(raw+"\nvar unreviewed = openDirectory(\"/forbidden\")\n"), "package initializers are forbidden")
	darwinInventoryManualReject(t, darwinInventoryManualPath, []byte(raw+"\nfunc TestOther(t *testing.T) {}\n"), "exactly one native entry")
}
