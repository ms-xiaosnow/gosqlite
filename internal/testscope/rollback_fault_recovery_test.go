package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These tests only inspect Go source text and inert mutated copies. They do not
// open databases, invoke the manual entry, arm scopes, run recovery, compile a
// manual binary, start a process or establish a physical acceptance result.
func rollbackFaultRecoverySources(t *testing.T) map[string][]byte {
	t.Helper()
	sources := map[string][]byte{}
	for _, path := range []string{rollbackFaultRecoveryNative, rollbackFaultRecoveryReceipt, rollbackFaultRecoveryReceiptTest} {
		raw, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		sources[path] = raw
	}
	return sources
}

func TestRollbackFaultRecoverySourceContracts(t *testing.T) {
	for path, raw := range rollbackFaultRecoverySources(t) {
		for _, scanner := range []struct {
			name string
			scan func(string, []byte) ([]string, error)
		}{
			{"recovery", inspectRollbackFaultRecoverySource},
			{"adapter", inspectRollbackFaultSource},
			{"ordinary", inspectSource},
			{"SQL", inspectRollbackFaultSQLSource},
			{"restart", inspectRollbackRestartSource},
		} {
			findings, err := scanner.scan(path, raw)
			if err != nil || len(findings) != 0 {
				t.Fatal("unchanged recovery source rejected", scanner.name, path, findings, err)
			}
		}
	}
	for _, owner := range rollbackFaultRecoverySharedOwners() {
		raw := rollbackFaultRecoveryReadSource(t, owner.path)
		if !rollbackFaultRecoverySharedManualPinned(owner.path, raw) {
			t.Fatal("unchanged shared environment owner not admitted", owner.path)
		}
		findings, err := inspectRollbackFaultRecoverySource(owner.path, raw)
		if err != nil || len(findings) != 0 {
			t.Fatal("unchanged shared environment owner rejected", owner.path, findings, err)
		}
	}
	// The recovery guard must not reach across the restart native namespace.
	guard := rollbackFaultRecoveryReadSource(t, "internal/testscope/rollback_fault_recovery.go")
	findings, err := inspectRollbackRestartSource("internal/testscope/rollback_fault_recovery.go", guard)
	if err != nil || len(findings) != 0 {
		t.Fatal("owner-local restart admission wrapper rejected", findings, err)
	}
}

func TestRollbackFaultRecoveryRepinnedSemanticCounterexamples(t *testing.T) {
	sources := rollbackFaultRecoverySources(t)
	variants := []struct{ name, path, old, replacement, finding string }{
		{"manual OR gate", rollbackFaultRecoveryNative, rollbackFaultManualTag, "//go:build linux || manual_fault_rollback", "exact native package and Linux AND manual build gate"},
		{"gate value weakened", rollbackFaultRecoveryNative, `if os.Getenv("GOSQLITE_MANUAL_FAULT_ROLLBACK") != "independently_authorized"`, `if os.Getenv("GOSQLITE_MANUAL_FAULT_ROLLBACK") == "independently_authorized"`, "manual entry first action"},
		{"path read before gate", rollbackFaultRecoveryNative, `if os.Getenv("GOSQLITE_MANUAL_FAULT_ROLLBACK")`, `_, _ = os.Lstat("unopened-mutant"); if os.Getenv("GOSQLITE_MANUAL_FAULT_ROLLBACK")`, "manual entry first action"},
		{"arm wrong transaction", rollbackFaultRecoveryNative, "armRollbackFault(tx, plan)", "armRollbackFault(other, plan)", "scope calls require exact direct helper/arguments"},
		{"arm wrong plan", rollbackFaultRecoveryNative, "armRollbackFault(tx, plan)", "armRollbackFault(tx, other)", "scope calls require exact direct helper/arguments"},
		{"armer function value", rollbackFaultRecoveryNative, "scope, armErr := armRollbackFault(tx, plan)", "arm := armRollbackFault; scope, armErr := arm(tx, plan)", "scope calls require exact direct helper/arguments"},
		{"false finish", rollbackFaultRecoveryNative, "report := finishRollbackFault(scope, commitErr)", "report := finishRollbackFault(scope, nil)", "actual Commit and original-return finish must be adjacent"},
		{"finish wrong scope", rollbackFaultRecoveryNative, "finishRollbackFault(scope, commitErr)", "finishRollbackFault(other, commitErr)", "scope calls require exact direct helper/arguments"},
		{"close before finish", rollbackFaultRecoveryNative, "commitErr := tx.Commit(ctx)", "commitErr := tx.Commit(ctx); _ = tx.Rollback()", "actual Commit and original-return finish must be adjacent"},
		{"second Commit", rollbackFaultRecoveryNative, "commitErr := tx.Commit(ctx)", "commitErr := tx.Commit(ctx); _ = tx.Commit(ctx)", "missing/duplicate protected native call Commit"},
		{"Commit wrong context", rollbackFaultRecoveryNative, "commitErr := tx.Commit(ctx)", "commitErr := tx.Commit(other)", "Commit must be the direct original tx call"},
		{"Commit wrong receiver", rollbackFaultRecoveryNative, "commitErr := tx.Commit(ctx)", "commitErr := other.Commit(ctx)", "Commit must be the direct original tx call"},
		{"Commit receiver alias", rollbackFaultRecoveryNative, "commitErr := tx.Commit(ctx)", "other := tx; commitErr := other.Commit(ctx)", "Commit must be the direct original tx call"},
		{"Commit indirect selector", rollbackFaultRecoveryNative, "commitErr := tx.Commit(ctx)", "commitErr := (tx.Commit)(ctx)", "Commit must be the direct original tx call"},
		{"Commit method expression", rollbackFaultRecoveryNative, "commitErr := tx.Commit(ctx)", "commitErr := (*Tx).Commit(tx, ctx)", "Commit must be the direct original tx call"},
		{"receipt Commit called", rollbackFaultRecoveryNative, "evidence.CommitReturn = report.Events[i].Commit", "evidence.CommitReturn = report.Events[i].Commit(ctx)", "Commit must be the direct original tx call"},
		{"receipt Commit aliased", rollbackFaultRecoveryNative, "evidence.CommitReturn = report.Events[i].Commit", "commit := report.Events[i].Commit; evidence.CommitReturn = commit", "Commit must be the direct original tx call"},
		{"receipt Commit wrong receiver", rollbackFaultRecoveryNative, "evidence.CommitReturn = report.Events[i].Commit", "evidence.CommitReturn = other.Events[i].Commit", "Commit must be the direct original tx call"},
		{"receipt Commit wrong destination", rollbackFaultRecoveryNative, "evidence.CommitReturn = report.Events[i].Commit", "other.CommitReturn = report.Events[i].Commit", "Commit must be the direct original tx call"},
		{"Commit function value", rollbackFaultRecoveryNative, "commitErr := tx.Commit(ctx)", "commit := tx.Commit; commitErr := commit(ctx)", "Commit must be the direct original tx call"},
		{"disarm wrong scope", rollbackFaultRecoveryNative, "disarmRollbackFault(scope)", "disarmRollbackFault(other)", "scope calls require exact direct helper/arguments"},
		{"recover original trial", rollbackFaultRecoveryNative, "RecoverRollback(ctx, replayPath,", "RecoverRollback(ctx, path,", "recovery must directly target fresh replay"},
		{"recovery limit expanded", rollbackFaultRecoveryNative, "MaxJournalBytes: 8200, MaxRecords: 1, MaxDatabaseBytes: 4096", "MaxJournalBytes: 8201, MaxRecords: 2, MaxDatabaseBytes: 8192", "recovery must directly target fresh replay"},
		{"recovery function value", rollbackFaultRecoveryNative, "result, recoveryErr := RecoverRollback(", "recover := RecoverRollback; result, recoveryErr := recover(", "recovery must directly target fresh replay"},
		{"auto cleanup fixture", rollbackFaultRecoveryNative, "func manualRollbackFaultImages() (old, next, partial [4096]byte) {", "func manualRollbackFaultImages() (old, next, partial [4096]byte) { os.RemoveAll(\"unopened-mutant\");", "unreviewed cleanup/process/selection reference RemoveAll"},
		{"expected derived from partial", rollbackFaultRecoveryNative, "next = old", "next = partial", "source binding/body/order changed"},
		{"retained precheck removed", rollbackFaultRecoveryNative, "if err := c.retainedUnchanged(); err != nil {\n\t\tt.Fatal(err)\n\t}", "", "source binding/body/order changed"},
		{"automatic-cleanup fixture", rollbackFaultRecoveryNative, "var c manualRollbackFaultConfig", "_ = t.TempDir(); var c manualRollbackFaultConfig", "unreviewed cleanup/process/selection reference TempDir"},
		{"actual copied into expected", rollbackFaultRecoveryNative, "wantPost := next", "wantPost := [4096]byte{}; copy(wantPost[:], post)", "source binding/body/order changed"},
		{"target selection changed", rollbackFaultRecoveryNative, "spec.Target = spec.Calls[8]", "spec.Target = spec.Calls[7]", "source binding/body/order changed"},
		{"target occurrence changed", rollbackFaultRecoveryNative, "Point: rollbackfault.DBPage, Occurrence: 1", "Point: rollbackfault.DBPage, Occurrence: 2", "source binding/body/order changed"},
		{"target offset changed", rollbackFaultRecoveryNative, "Point: rollbackfault.DBPage, Occurrence: 1, Role: rollbackfault.Database, Owner: 11, Length: 4096", "Point: rollbackfault.DBPage, Occurrence: 1, Role: rollbackfault.Database, Owner: 11, Offset: 1, Length: 4096", "source binding/body/order changed"},
		{"target length changed", rollbackFaultRecoveryNative, "Point: rollbackfault.DBPage, Occurrence: 1, Role: rollbackfault.Database, Owner: 11, Length: 4096", "Point: rollbackfault.DBPage, Occurrence: 1, Role: rollbackfault.Database, Owner: 11, Length: 4095", "source binding/body/order changed"},
		{"prefix changed", rollbackFaultRecoveryNative, "rollbackfault.PrefixShortWrite, 64, 9", "rollbackfault.PrefixShortWrite, 63, 9", "source binding/body/order changed"},
		{"selection broadened", rollbackFaultRecoveryNative, `"^TestManualRollbackFaultRecovery$"`, `"TestManualRollbackFault"`, "source binding/body/order changed"},
		{"two selected invocations", rollbackFaultRecoveryNative, `"-test.count": "1"`, `"-test.count": "2"`, "source binding/body/order changed"},
		{"verbose omitted from required flags", rollbackFaultRecoveryNative, `"-test.v": "true",`, "", "source binding/body/order changed"},
		{"verbose wrong value allowed", rollbackFaultRecoveryNative, `"-test.v": "true",`, `"-test.v": "false",`, "source binding/body/order changed"},
		{"duplicate verbose allowed", rollbackFaultRecoveryNative, "if !found || seen[key]", `if !found || (seen[key] && key != "-test.v")`, "source binding/body/order changed"},
		{"header-only readback", rollbackFaultRecoveryNative, "|| !bytes.Equal(page, expected[:])", "", "source binding/body/order changed"},
		{"recovery false accepted", rollbackFaultRecoveryNative, "if !result.Recovered ||", "if result.Recovered ||", "source binding/body/order changed"},
		{"owner close failure ignored", rollbackFaultRecoveryNative, "o.closes != 1 || o.closeErr != nil", "o.closes != 1", "source binding/body/order changed"},
		{"contract import aliased", rollbackFaultRecoveryNative, `"github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackfault"`, `rf "github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackfault"`, "aliased, dot or blank import"},
		{"syscall execution", rollbackFaultRecoveryNative, "func manualRollbackFaultImages() (old, next, partial [4096]byte) {", "func manualRollbackFaultImages() (old, next, partial [4096]byte) { _ = syscall.Getpid();", "syscall import is metadata-only Stat_t"},
		{"looped native trial", rollbackFaultRecoveryNative, `report, commitCheckErr := manualRollbackFaultCommit(ctx, filepath.Join(c.root, "trial.db"), plan)`, `for { _, _ = manualRollbackFaultCommit(ctx, filepath.Join(c.root, "trial.db"), plan) }`, "single-trial helper cannot be looped"},
		{"native subtest trial", rollbackFaultRecoveryNative, `report, commitCheckErr := manualRollbackFaultCommit(ctx, filepath.Join(c.root, "trial.db"), plan)`, `t.Run("duplicate", func(t *testing.T) { _, _ = manualRollbackFaultCommit(ctx, filepath.Join(c.root, "trial.db"), plan) })`, "native subtests forbidden"},
		{"extra native entry", rollbackFaultRecoveryNative, "func TestManualRollbackFaultRecovery(t *testing.T) {", "func TestExtra(t *testing.T) {}\nfunc TestManualRollbackFaultRecovery(t *testing.T) {", "one exact manual test entry required"},
		{"single-trial helper alias", rollbackFaultRecoveryNative, "old, next, partial := manualRollbackFaultImages()", "images := manualRollbackFaultImages; old, next, partial := images()", "native helper cannot escape as alias or function value"},
		{"receipt imports descriptor", rollbackFaultRecoveryReceipt, "package rollbackfault", "package rollbackfault\nimport \"os\"", "descriptor-free recovery receipt permits only SHA-256 import"},
		{"receipt true PASS", rollbackFaultRecoveryReceipt, "func (RecoveryObservation) Pass() bool              { return false }", "func (RecoveryObservation) Pass() bool              { return true }", "recovery observation cannot confer physical validation or PASS"},
		{"receipt physical validation", rollbackFaultRecoveryReceipt, "func (RecoveryObservation) PhysicalValidated() bool { return false }", "func (RecoveryObservation) PhysicalValidated() bool { return true }", "recovery observation cannot confer physical validation or PASS"},
		{"receipt descriptor field", rollbackFaultRecoveryReceipt, "type RecoverySpec struct {", "type RecoverySpec struct {\n\tfd uintptr", "recovery receipt types must be scalar/fixed-array value records"},
		{"receipt package state", rollbackFaultRecoveryReceipt, "type RecoverySpec struct {", "var shared int\n\ntype RecoverySpec struct {", "owned sources cannot add mutable package state"},
		{"receipt test native import", rollbackFaultRecoveryReceiptTest, `import "testing"`, "import (\"testing\"; \"os\")", "recovery receipt tests require only testing import"},
	}
	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			original := string(sources[variant.path])
			if !strings.Contains(original, variant.old) {
				t.Fatal("counterexample anchor missing", variant.path, variant.old)
			}
			changed := []byte(strings.Replace(original, variant.old, variant.replacement, 1))
			oldPin := rollbackFaultRecoveryPins[variant.path]
			rollbackFaultRecoveryPins[variant.path] = fmt.Sprintf("%x", sha256.Sum256(changed))
			t.Cleanup(func() { rollbackFaultRecoveryPins[variant.path] = oldPin })
			findings, err := inspectRollbackFaultRecoverySource(variant.path, changed)
			if err != nil {
				t.Fatal("counterexample must parse", err)
			}
			joined := strings.Join(findings, "\n")
			if !strings.Contains(joined, variant.finding) {
				t.Fatal("semantic rejection missing after pin update", variant.finding, findings)
			}
			if strings.Contains(joined, "complete-source pin changed") {
				t.Fatal("counterexample relied on old source digest", findings)
			}
		})
	}
}

func TestRollbackFaultRecoveryRepinnedTriviaAccepted(t *testing.T) {
	for path, original := range rollbackFaultRecoverySources(t) {
		t.Run(path, func(t *testing.T) {
			changed := append(append([]byte(nil), original...), []byte("\n// Additional inert review comment.\n")...)
			oldPin := rollbackFaultRecoveryPins[path]
			rollbackFaultRecoveryPins[path] = fmt.Sprintf("%x", sha256.Sum256(changed))
			t.Cleanup(func() { rollbackFaultRecoveryPins[path] = oldPin })
			findings, err := inspectRollbackFaultRecoverySource(path, changed)
			if err != nil || len(findings) != 0 {
				t.Fatal("declaration contract must ignore comments", findings, err)
			}
		})
	}
}

func TestRollbackFaultRecoveryOutsideOwnershipRejected(t *testing.T) {
	variants := []struct{ name, path, source, finding string }{
		{"copied entry", "internal/pager/copied_test.go", "package pager\nfunc TestManualRollbackFaultRecovery() {}", "native entry/helper reference outside exact source"},
		{"helper wrapper", "internal/pager/copied.go", "package pager\nfunc wrapper() { manualRollbackFaultCommit(ctx,path,plan) }", "native entry/helper reference outside exact source"},
		{"helper function value", "internal/pager/copied.go", "package pager\nvar call = manualRollbackFaultRecoverCopy", "native entry/helper reference outside exact source"},
		{"helper type alias", "internal/pager/copied.go", "package pager\ntype escaped = manualRollbackFaultLedger", "native entry/helper reference outside exact source"},
		{"extra native source", "internal/pager/rollback_fault_recovery_extra_test.go", "package pager", "unreviewed recovery harness source path"},
		{"copied environment consumer", "internal/pager/copied.go", "package pager\nfunc read() { os.Getenv(\"GOSQLITE_ROLLBACK_FAULT_ROOT\") }", "manual environment identity outside exact native/adapter owner"},
		{"environment alias consumer", "internal/pager/copied.go", "package pager\nfunc read() { env := os.Getenv; env(\"GOSQLITE_MANUAL_FAULT_ROLLBACK\") }", "manual environment identity outside exact native/adapter owner"},
		{"extra receipt source", "internal/testkit/rollbackfault/recovery_extra.go", "package rollbackfault", "unreviewed recovery receipt source path"},
	}
	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			findings, err := inspectRollbackFaultRecoverySource(variant.path, []byte(variant.source))
			if err != nil || !strings.Contains(strings.Join(findings, "\n"), variant.finding) {
				t.Fatal(variant.name, findings, err)
			}
		})
	}
	rollbackFaultRecoveryNativeAdmissionCounterexamples(t)
	rollbackFaultRecoverySharedEnvironmentCounterexamples(t)
}

func TestRollbackFaultRecoveryRestrictedReferenceRemainsNarrow(t *testing.T) {
	base := "package pager\nimport \"os\"\nfunc manualRollbackFaultRecoverCopy(ctx context.Context, replayPath string) { " + rollbackFaultRecoveryRecoverCall + " }"
	// The exact base must be admitted before adding the actually imported
	// destructive operation; unrelated findings must not qualify the mutant.
	if findings, err := inspectSource(rollbackFaultRecoveryNative, []byte(base)); err != nil || len(findings) != 0 {
		t.Fatal("exact direct recovery base must be admitted", findings, err)
	}
	variants := []struct{ name, path, source string }{
		{"different path", "internal/pager/rogue_test.go", base},
		{"different helper", rollbackFaultRecoveryNative, strings.Replace(base, "func manualRollbackFaultRecoverCopy", "func wrapper", 1)},
		{"different destination", rollbackFaultRecoveryNative, strings.Replace(base, "RecoverRollback(ctx, replayPath,", "RecoverRollback(ctx, trialPath,", 1)},
		{"alias", rollbackFaultRecoveryNative, "package pager\nfunc manualRollbackFaultRecoverCopy() { recover := RecoverRollback; recover() }"},
		{"manual tag does not bypass", "internal/pager/rogue_test.go", rollbackFaultManualTag + "\n" + base},
		{"extra destructive operation", rollbackFaultRecoveryNative, strings.Replace(base, " { ", " { os.RemoveAll(\"unopened-mutant\"); ", 1)},
	}
	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			findings, err := inspectSource(variant.path, []byte(variant.source))
			if err != nil || len(findings) == 0 {
				t.Fatal("restricted policy was broadened", variant.name, findings, err)
			}
		})
	}
}

// Composition coverage remains source-only and stays under the five donor roots.
func rollbackFaultRecoveryReadSource(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func rollbackFaultRecoverySharedOwners() []struct{ path, tag, diagnostic string } {
	return []struct{ path, tag, diagnostic string }{
		{rollbackFaultSQLBridge, rollbackFaultManualTag, "bridge first action"},
		{rollbackFaultSQLNative, rollbackFaultManualTag, "root first action"},
		{"internal/pager/rollback_restart_manual_linux.go", "//go:build linux && manual_fault_rollback && manual_restart_rollback", "bridge first action"},
		{"sqlite/rollback_restart_manual_linux_test.go", "//go:build linux && manual_fault_rollback && manual_restart_rollback", "admission gates must dominate"},
	}
}

func rollbackFaultRecoveryNativeAdmissionCounterexamples(t *testing.T) {
	t.Helper()
	original := rollbackFaultRecoverySources(t)[rollbackFaultRecoveryNative]
	if !rollbackFaultRecoveryNativePinned(rollbackFaultRecoveryNative, original) {
		t.Fatal("exact recovery native admission missing")
	}
	variants := []struct{ name, path, old, replacement string }{
		{"native wrong path", "internal/pager/copied_test.go", "", ""},
		{"native moved path", "sqlite/rollback_fault_recovery_manual_linux_test.go", "", ""},
		{"native alias import", rollbackFaultRecoveryNative, `"github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackfault"`, `rf "github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackfault"`},
		{"native dot import", rollbackFaultRecoveryNative, `"github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackfault"`, `. "github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackfault"`},
		{"native blank import", rollbackFaultRecoveryNative, `"github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackfault"`, `_ "github.com/ms-xiaosnow/gosqlite/internal/testkit/rollbackfault"`},
		{"native missing gate", rollbackFaultRecoveryNative, rollbackFaultManualTag, "// no manual gate"},
		{"native OR gate", rollbackFaultRecoveryNative, rollbackFaultManualTag, "//go:build linux || manual_fault_rollback"},
		{"native weakened runtime gate", rollbackFaultRecoveryNative, `!= "independently_authorized"`, `== "independently_authorized"`},
		{"native byte drift", rollbackFaultRecoveryNative, "package pager", "package pager // unreviewed composition"},
		{"native body drift", rollbackFaultRecoveryNative, "finishRollbackFault(scope, commitErr)", "finishRollbackFault(scope, nil)"},
	}
	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			changed := original
			if variant.old != "" {
				if !strings.Contains(string(original), variant.old) {
					t.Fatal("native composition anchor missing", variant.old)
				}
				changed = []byte(strings.Replace(string(original), variant.old, variant.replacement, 1))
			}
			if rollbackFaultRecoveryNativePinned(variant.path, changed) {
				t.Fatal("changed native input received pinned admission", variant.name)
			}
			for _, scanner := range []struct {
				name string
				scan func(string, []byte) ([]string, error)
			}{
				{"adapter", inspectRollbackFaultSource},
				{"SQL", inspectRollbackFaultSQLSource},
				{"recovery", inspectRollbackFaultRecoverySource},
			} {
				findings, err := scanner.scan(variant.path, changed)
				if err != nil || len(findings) == 0 {
					t.Fatal("composition admission widened", scanner.name, variant.name, findings, err)
				}
			}
		})
	}
	t.Run("native repinned semantic drift", func(t *testing.T) {
		changed := []byte(strings.Replace(string(original), "finishRollbackFault(scope, commitErr)", "finishRollbackFault(scope, nil)", 1))
		if string(changed) == string(original) {
			t.Fatal("repinned native composition anchor missing")
		}
		pin := rollbackFaultRecoveryPins[rollbackFaultRecoveryNative]
		rollbackFaultRecoveryPins[rollbackFaultRecoveryNative] = fmt.Sprintf("%x", sha256.Sum256(changed))
		t.Cleanup(func() { rollbackFaultRecoveryPins[rollbackFaultRecoveryNative] = pin })
		if !rollbackFaultRecoveryNativePinned(rollbackFaultRecoveryNative, changed) {
			t.Fatal("repinned semantic mutant must exercise the source admission")
		}
		// Run every participating boundary even though the changed finish call
		// is independently rejected by adapter and recovery semantic checks.
		for _, scan := range []func(string, []byte) ([]string, error){inspectRollbackFaultSource, inspectRollbackFaultSQLSource, inspectRollbackFaultRecoverySource} {
			if _, err := scan(rollbackFaultRecoveryNative, changed); err != nil {
				t.Fatal("repinned composition source must parse", err)
			}
		}
		findings, err := inspectRollbackFaultRecoverySource(rollbackFaultRecoveryNative, changed)
		joined := strings.Join(findings, "\n")
		if err != nil || !strings.Contains(joined, "actual Commit and original-return finish must be adjacent") || strings.Contains(joined, "complete-source pin changed") {
			t.Fatal("repinned native admission bypassed independent semantics", findings, err)
		}
	})
}

func rollbackFaultRecoverySharedEnvironmentCounterexamples(t *testing.T) {
	t.Helper()
	for _, owner := range rollbackFaultRecoverySharedOwners() {
		original := rollbackFaultRecoveryReadSource(t, owner.path)
		variants := []struct{ name, path, source string }{
			{"wrong path", "internal/pager/copied_test.go", string(original)},
			{"missing tag", owner.path, strings.Replace(string(original), owner.tag, "// no manual gate", 1)},
			{"OR tag", owner.path, strings.Replace(string(original), owner.tag, strings.Replace(owner.tag, "&&", "||", 1), 1)},
			{"wrong byte pin", owner.path, string(original) + "\n// unreviewed shared owner\n"},
			{"weakened gate", owner.path, strings.Replace(string(original), `!= "independently_authorized"`, `== "independently_authorized"`, 1)},
			{"copied gate helper", owner.path, string(original) + `
func copiedFaultGate() { os.Getenv("GOSQLITE_MANUAL_FAULT_ROLLBACK") }
`},
			{"hidden alias consumer", owner.path, string(original) + `
func hiddenFaultGate() { env := os.Getenv; env("GOSQLITE_MANUAL_FAULT_ROLLBACK") }
`},
			{"foreign recovery environment", owner.path, string(original) + `
func foreignRecoveryGate() { os.Getenv("GOSQLITE_ROLLBACK_FAULT_ROOT") }
`},
		}
		for _, variant := range variants {
			t.Run(owner.path+"/"+variant.name, func(t *testing.T) {
				changed := []byte(variant.source)
				if variant.path == owner.path && variant.source == string(original) {
					t.Fatal("shared owner composition mutation missing", variant.name)
				}
				if rollbackFaultRecoverySharedManualPinned(variant.path, changed) {
					t.Fatal("changed shared owner received environment admission", variant.name)
				}
				findings, err := inspectRollbackFaultRecoverySource(variant.path, changed)
				if err != nil || !strings.Contains(strings.Join(findings, "\n"), "manual environment identity outside exact native/adapter owner") {
					t.Fatal("shared environment ownership widened", variant.name, findings, err)
				}
			})
		}
		t.Run(owner.path+"/repinned semantic gate", func(t *testing.T) {
			if owner.path != rollbackFaultSQLBridge && owner.path != rollbackFaultSQLNative {
				// The owner-local helper avoids direct protected restart map access.
				restartGuardRepinned(t, owner.path, original, `!= "independently_authorized"`, `== "independently_authorized"`, owner.diagnostic)
				return
			}
			changed := []byte(strings.Replace(string(original), `!= "independently_authorized"`, `== "independently_authorized"`, 1))
			pin, shape := rollbackFaultSQLPins[owner.path], rollbackFaultSQLShapes[owner.path]
			rollbackFaultSQLPins[owner.path] = fmt.Sprintf("%x", sha256.Sum256(changed))
			rollbackFaultSQLShapes[owner.path] = string(changed)
			t.Cleanup(func() { rollbackFaultSQLPins[owner.path], rollbackFaultSQLShapes[owner.path] = pin, shape })
			if !rollbackFaultRecoverySharedManualPinned(owner.path, changed) {
				t.Fatal("repinned shared owner must exercise the environment admission")
			}
			findings, err := inspectRollbackFaultSQLSource(owner.path, changed)
			joined := strings.Join(findings, "\n")
			if err != nil || !strings.Contains(joined, owner.diagnostic) || strings.Contains(joined, "snapshot changed") || strings.Contains(joined, "source binding/body/order changed") {
				t.Fatal("repinned shared gate bypassed independent SQL semantics", findings, err)
			}
		})
	}
	for _, reference := range []string{"rollbackRestartPins", "rollbackRestartNative"} {
		t.Run("recovery guard direct protected "+reference, func(t *testing.T) {
			raw := []byte("package main\nvar forbidden = " + reference)
			findings, err := inspectRollbackRestartSource("internal/testscope/rollback_fault_recovery.go", raw)
			if err != nil || !strings.Contains(strings.Join(findings, "\n"), "restart declaration/reference outside exact source closure") {
				t.Fatal("recovery guard bypassed restart namespace", reference, findings, err)
			}
		})
	}
}
