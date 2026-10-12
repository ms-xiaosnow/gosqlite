package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These are inert source fixtures. No adapter, Commit, native owner, recovery,
// controller, subprocess, filesystem fixture or manual executable is invoked.
// Authoring this file does not mean this or any other test has been run.
func rollbackFaultSources(t *testing.T) map[string][]byte {
	t.Helper()
	sources := map[string][]byte{}
	for path := range rollbackFaultPins {
		raw, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		sources[path] = raw
	}
	return sources
}

func TestRollbackFaultSourceContracts(t *testing.T) {
	for path, raw := range rollbackFaultSources(t) {
		findings, err := inspectRollbackFaultSource(path, raw)
		if err != nil || len(findings) != 0 {
			t.Fatal(path, findings, err)
		}
	}
}

// Recompute only the supplemental pin. Every mutation must still parse and
// trigger its independent semantic/source-shape finding, never a hash mismatch.
// No parallelism: the replacement pin is scoped to one ordinary text subtest.
func TestRollbackFaultRepinnedSemanticCounterexamples(t *testing.T) {
	sources := rollbackFaultSources(t)
	variants := []struct{ name, path, old, replacement, finding string }{
		{"default build broadened", rollbackFaultDefault, rollbackFaultDefaultTag, "//go:build !linux || manual_fault_rollback", "exact default build complement"},
		{"manual OR gate", rollbackFaultManual, rollbackFaultManualTag, "//go:build linux || manual_fault_rollback", "exact Linux AND manual build gate"},
		{"manual env weakened", rollbackFaultManual, `!= "independently_authorized"`, `== "independently_authorized"`, "armer first action"},
		{"effect before gate", rollbackFaultManual, `if os.Getenv("GOSQLITE_MANUAL_FAULT_ROLLBACK")`, `specBeforeGate := plan.Spec(); _ = specBeforeGate; if os.Getenv("GOSQLITE_MANUAL_FAULT_ROLLBACK")`, "armer first action"},
		{"default owner changed", rollbackFaultDefault, "return writeFullAt(f, b, off)", "return writeFullAt(t.file, b, off)", "default bridge must return exact original owner call"},
		{"default extra environment", rollbackFaultDefault, "return openJournal(path, flags)", `os.Getenv("GOSQLITE_MANUAL_FAULT_ROLLBACK"); return openJournal(path, flags)`, "default bridge must contain one direct original return"},
		{"default alias call", rollbackFaultDefault, "return openJournal(path, flags)", "open := openJournal; return open(path, flags)", "default bridge must contain one direct original return"},
		{"default observer initializer", rollbackFaultDefault, "func rollbackCreate", "var observer = 1\nfunc rollbackCreate", "default cannot contain state"},
		{"point ID changed", rollbackFaultPoints, `"RB.DB_PAGE"`, `"RB.DB_SYNC"`, "point ID/name binding changed"},
		{"point alias", rollbackFaultPoints, "type rollbackIOPoint string", "type rollbackIOPoint = string", "point type must remain private non-alias"},
		{"Commit original owner changed", rollbackFaultCommit, "rollbackWrite(t, rbDBPage, t.file, data", "rollbackWrite(t, rbDBPage, jf, data", "Commit bridge point/owner/offset/length/argument binding changed"},
		{"Commit point repeated", rollbackFaultCommit, "rollbackFileSync(t, rbJHeaderSync, jf)", "rollbackFileSync(t, rbJBodySync, jf)", "missing/duplicate exact Commit binding"},
		{"Commit bridge alias", rollbackFaultCommit, "jf, err := rollbackCreate(", "create := rollbackCreate; jf, err := create(", "bridge reference outside exact Commit call"},
		{"Commit bridge method value", rollbackFaultCommit, "jf, err := rollbackCreate(", "_ = rollbackCreate; jf, err := rollbackCreate(", "bridge reference outside exact Commit call"},
		{"write helper retries", rollbackFaultCommit, "n, err := f.WriteAt(b, off)", "n, err := f.WriteAt(b, off); if err == nil && n != len(b) { n, err = f.WriteAt(b, off) }", "WriterAt interface-only signature and original helper body"},
		{"write helper error priority", rollbackFaultCommit, "if err != nil {\n\t\treturn err\n\t}", "if err != nil {\n\t\treturn io.ErrShortWrite\n\t}", "WriterAt interface-only signature and original helper body"},
		{"write helper concrete signature", rollbackFaultCommit, "f io.WriterAt", "f *vfs.File", "WriterAt interface-only signature and original helper body"},
		{"manual contract alias", rollbackFaultManual, `"github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackfault"`, `rf "github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackfault"`, "contract production consumer"},
		{"manual contract dot import", rollbackFaultManual, `"github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackfault"`, `. "github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackfault"`, "contract production consumer"},
		{"manual armer invoked internally", rollbackFaultManual, "s := rollbackFaultRegistry.active", "_, _ = armRollbackFault(nil, rollbackfault.Plan{}); s := rollbackFaultRegistry.active", "scope entry has no approved caller"},
		{"manual registry reused", rollbackFaultManual, "rollbackFaultRegistry.used = true", "rollbackFaultRegistry.used = false", "source binding/body/order changed"},
		{"manual owner raw descriptor", rollbackFaultManual, "w.executed = true", "_ = w.file.Fd(); w.executed = true", "unreviewed native ownership/control reference Fd"},
		{"manual explicit close", rollbackFaultManual, "w.executed = true", "_ = w.file.Close(); w.executed = true", "unreviewed native ownership/control reference Close"},
		{"manual fake physical validation", rollbackFaultManual, "Validation: rollbackfault.ValidationAbsent", "Validation: 1", "source binding/body/order changed"},
		{"contract imports descriptors", rollbackFaultContract, "package rollbackfault", "package rollbackfault\nimport \"os\"", "descriptor-free contract has no imports"},
		{"contract retains callback", rollbackFaultContract, "type Identity struct {", "type Identity struct {\ncallback func()", "contract types must be scalar or fixed-array value records"},
		{"contract retains descriptor", rollbackFaultContract, "type Identity struct {", "type Identity struct {\nfd uintptr", "contract types must be scalar or fixed-array value records"},
		{"contract mutable state", rollbackFaultContract, "package rollbackfault", "package rollbackfault\nvar sharedReceipt Receipt", "descriptor-free contract cannot store package state"},
		{"contract physical PASS", rollbackFaultContract, "func (Observation) Pass() bool              { return false }", "func (Observation) Pass() bool              { return true }", "adapter observation can never confer physical PASS"},
		{"contract physical validation", rollbackFaultContract, "func (Observation) PhysicalValidated() bool { return false }", "func (Observation) PhysicalValidated() bool { return true }", "adapter observation can never confer physical PASS"},
		{"fake writer imports native", rollbackFaultWriterTest, "import (", "import (\n\"github.com/ms-xiaosnow/gosqlite/internal/vfs\"", "fake WriterAt tests must have only memory/assertion imports"},
		{"fake writer fixture", rollbackFaultWriterTest, "original := errors.New", "_ = t.TempDir(); original := errors.New", "non-memory or skipped-test reference TempDir"},
		{"fake writer silent skip", rollbackFaultWriterTest, "original := errors.New", "t.Skip(); original := errors.New", "non-memory or skipped-test reference Skip"},
	}
	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			original := string(sources[variant.path])
			if !strings.Contains(original, variant.old) {
				t.Fatal("counterexample anchor missing", variant.path, variant.old)
			}
			changed := []byte(strings.Replace(original, variant.old, variant.replacement, 1))
			oldPin := rollbackFaultPins[variant.path]
			rollbackFaultPins[variant.path] = fmt.Sprintf("%x", sha256.Sum256(changed))
			t.Cleanup(func() { rollbackFaultPins[variant.path] = oldPin })
			findings, err := inspectRollbackFaultSource(variant.path, changed)
			if err != nil {
				t.Fatal("counterexample must parse", err)
			}
			joined := strings.Join(findings, "\n")
			if !strings.Contains(joined, variant.finding) {
				t.Fatal("semantic rejection missing after pin update", variant.finding, findings)
			}
			if strings.Contains(joined, "complete-source pin changed") {
				t.Fatal("negative relied on old source digest", findings)
			}
		})
	}
}

func TestRollbackFaultOutsideConsumersRejected(t *testing.T) {
	variants := []struct{ name, path, source, finding string }{
		{"ordinary armer", "internal/pager/rogue_test.go", "package pager\nfunc TestRogue() { armRollbackFault(nil, plan) }", "scope entry has no approved caller"},
		{"tagged armer", "internal/pager/rogue_linux_test.go", rollbackFaultManualTag + "\n\npackage pager\nfunc TestRogue() { armRollbackFault(nil, plan) }", "manual tag outside exact"},
		{"armer function value", "internal/pager/rogue.go", "package pager\nvar delayed = armRollbackFault", "scope entry has no approved caller"},
		{"armer wrapper", "internal/pager/rogue.go", "package pager\nfunc wrapper() { armRollbackFault(nil, plan) }", "scope entry has no approved caller"},
		{"copied declaration", "internal/pager/rogue.go", "package pager\nfunc armRollbackFault() {}", "scope entry has no approved caller"},
		{"finish caller", "internal/pager/rogue.go", "package pager\nfunc wrapper() { finishRollbackFault(nil, nil) }", "scope entry has no approved caller"},
		{"disarm alias", "internal/pager/rogue.go", "package pager\nvar end = disarmRollbackFault", "scope entry has no approved caller"},
		{"manual helper value", "internal/pager/rogue.go", "package pager\nvar before = rollbackFaultBefore", "manual-private reference"},
		{"manual owner type", "internal/pager/rogue.go", "package pager\ntype alias = rollbackFaultScope", "manual-private reference"},
		{"copied bridge", "internal/pager/rogue.go", "package pager\nfunc rollbackWrite() {}", "bridge reference outside exact Commit"},
		{"bridge function value", "internal/pager/rogue_test.go", "package pager\nvar write = rollbackWrite", "bridge reference outside exact Commit"},
		{"contract production consumer", "internal/pager/rogue.go", "package pager\nimport \"" + rollbackFaultImport + "\"", "contract production consumer"},
		{"contract test consumer", "internal/pager/rogue_test.go", "package pager\nimport \"" + rollbackFaultImport + "\"", "contract production consumer"},
		{"contract dot import", "internal/pager/rogue.go", "package pager\nimport . \"" + rollbackFaultImport + "\"", "contract production consumer"},
		{"copied contract file", "internal/testkit/rollbackfault/native.go", "package rollbackfault\nfunc openFixture() {}", "unreviewed contract-package source"},
		{"native adapter fixture file", "internal/pager/rollback_io_native_test.go", "package pager\nfunc TestNativeFixture() {}", "unreviewed adapter source or native caller"},
		{"recovery path wrong arm target", rollbackFaultRecoveryNative, "package pager\nfunc manualRollbackFaultCommit() { armRollbackFault(other, plan) }", "scope entry has no approved caller"},
		{"recovery path scope wrapper", rollbackFaultRecoveryNative, "package pager\nfunc wrapper() { armRollbackFault(tx, plan) }", "scope entry has no approved caller"},
		{"recovery path armer alias", rollbackFaultRecoveryNative, "package pager\nvar arm = armRollbackFault", "scope entry has no approved caller"},
		{"recovery path private registry", rollbackFaultRecoveryNative, "package pager\nvar registry = rollbackFaultRegistry", "manual-private reference"},
		{"recovery path copied bridge", rollbackFaultRecoveryNative, "package pager\nfunc rollbackWrite() {}", "bridge reference outside exact Commit"},
		{"recovery path contract alias", rollbackFaultRecoveryNative, "package pager\nimport rf \"" + rollbackFaultImport + "\"", "contract production consumer"},
		{"environment consumer", "internal/pager/rogue.go", "package pager\nimport \"os\"\nfunc arm() { os.Getenv(\"GOSQLITE_MANUAL_FAULT_ROLLBACK\") }", "manual environment consumer outside"},
	}
	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			findings, err := inspectRollbackFaultSource(variant.path, []byte(variant.source))
			if err != nil || !strings.Contains(strings.Join(findings, "\n"), variant.finding) {
				t.Fatal("outside consumer accepted or unparsable", findings, err)
			}
		})
	}
	for _, path := range []string{rollbackFaultDefault, rollbackFaultManual} {
		raw := rollbackFaultSources(t)[path]
		findings, err := inspectRollbackFaultSource("internal/pager/copied.go", raw)
		if err != nil || len(findings) == 0 {
			t.Fatal("whole copied adapter accepted", path, findings, err)
		}
	}
}

func TestRollbackFaultGuardWiringPreservesPriorPolicies(t *testing.T) {
	raw, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, hook := range []string{
		"inspectSource(path, raw)", "inspectBackupManualSource(path, raw)", "inspectWALPublicationSource(path, raw)", "inspectOracleSource(path, raw)",
		`inspectOracleContracts(".")`, `inspectBackupManualContracts(".")`, `inspectWALPublicationContracts(".")`, `inspectDarwinInventoryManualContract(".")`,
		"inspectRollbackFaultSource(path, raw)", `inspectRollbackFaultContracts(".")`,
		"inspectRollbackFaultRecoverySource(path, raw)", `inspectRollbackFaultRecoveryContracts(".")`,
	} {
		if strings.Count(string(raw), hook) != 1 {
			t.Fatal("missing/duplicate guard hook", hook)
		}
	}
	// This exact literal is subject to independent whole-file source review.
	// The map value alone cannot authorize a changed main.go.
	const reviewedMainSHA256 = "e31c233fed8976fddff3236a8804f84d493e2567d6b6d85d11355de2ca0da52b"
	if walPublicationFrozenSources["internal/testscope/main.go"] != reviewedMainSHA256 {
		t.Fatal("main.go WAL pin differs from independently reviewed literal")
	}
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != reviewedMainSHA256 {
		t.Fatal("main.go bytes differ from independently reviewed literal")
	}
	if findings := inspectWALPublicationFrozenSource("internal/testscope/main.go", raw); len(findings) != 0 {
		t.Fatal("reviewed main.go rejected by existing WAL source guard", findings)
	}
	changed := append(append([]byte(nil), raw...), []byte("\n// unreviewed main.go change\n")...)
	if findings := inspectWALPublicationFrozenSource("internal/testscope/main.go", changed); len(findings) == 0 {
		t.Fatal("unreviewed main.go comment accepted by existing WAL source guard")
	}
}
