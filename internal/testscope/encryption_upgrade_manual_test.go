package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func formatUpgradeReadSource(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(path)))
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func TestFormatUpgradeManualStructuralCounterexamples(t *testing.T) {
	raw := formatUpgradeReadSource(t, formatUpgradeNative)
	findings, err := inspectFormatUpgradeStructure(formatUpgradeNative, raw)
	if err != nil || len(findings) != 0 {
		t.Fatal("base structure", findings, err)
	}
	// These deliberately bypass full-source pins. Each counterexample must be
	// rejected by a shape, closure or actual assertion check, not a hash mismatch.
	mutants := []struct{ name, old, new string }{
		{"tag-or", "linux && manual_format_upgrade", "linux || manual_format_upgrade"},
		{"gate-order", "if os.Getenv(\"GOSQLITE_MANUAL_FORMAT_UPGRADE\")", "t.Log(\"before gate\"); if os.Getenv(\"GOSQLITE_MANUAL_FORMAT_UPGRADE\")"},
		{"gate-value", "!= \"independently_authorized\"", "!= \"yes\""},
		{"wide-selector", "^TestManualEncryptedSnapshotToWALUpgrade$", "TestManual"},
		{"repeated-run", "\"test.count\": \"1\"", "\"test.count\": \"2\""},
		{"timeout-kill", "\"test.timeout\": \"0s\"", "\"test.timeout\": \"1s\""},
		{"verbose-omitted", `, "test.v": "true"`, ``},
		{"verbose-false", `"test.v": "true"`, `"test.v": "false"`},
		{"missing-page", "[]uint32{512, 4096, 65536}", "[]uint32{512, 4096}"},
		{"missing-adjacent-capture", "o := r.capture(\"source\", db, nil, nil, 0, err)", "r.t.Log(\"observation before capture\"); o := r.capture(\"source\", db, nil, nil, 0, err)"},
		{"wrong-result-owner", "r.capture(\"source\", db, nil, nil, 0, err)", "r.capture(\"source\", nil, nil, nil, 0, err)"},
		{"hidden-error", "r.capture(\"source\", db, nil, nil, 0, err)", "r.capture(\"source\", db, nil, nil, 0, nil)"},
		{"borrowed-helper", "r.ledger(ctx, o, \"snapshot\")", "encryptedExec(r.t, db, \"SELECT 1\")"},
		{"cleanup", "r.preflight()", "r.preflight(); os.RemoveAll(\"fixture\")"},
		{"close-alias", "err = o.db.Close()", "closer := o.db.Close; err = closer()"},
		{"unknown-retry", "if o.state.Present && o.state.Terminal == \"live\" && o.state.Attempts == 0", "if o.state.Present"},
		{"eof-error-swallowed", "o.state.Retire(\"eof\", manualFormatError(err))", "o.state.Retire(\"eof\", \"\")"},
		{"lost-old-root", "formatupgrade.Preserve(c.SourceBefore, c.SourceAfter, \"source.gse\")", "formatupgrade.Equal(c.SourceBefore, c.SourceAfter)"},
		{"old-mode-sentinel-only", "errors.As(oldErr, &oldDetail) && oldDetail.Code == sqlite.CodeError", "true"},
		{"nonfresh-restore", "r.absent(restored)", "r.event(\"existing restore allowed\", nil, nil)"},
		{"missing-ledger-check", "\"dense-vector-exact-ledger\"", "\"unreviewed-vector-check\""},
		{"missing-case-result", "r.receipt.Cases = append(r.receipt.Cases, c)", "r.receipt.Cases = nil"},
		{"false-terminal", "err := r.receipt.Validate()", "var err error"},
		{"row-assertion-true", "r.require(reflect.DeepEqual(got, want), \"complete-typed-row-ledger\", nil)", "r.require(true, \"complete-typed-row-ledger\", nil)"},
		{"row-comment-preserved-true", "r.require(reflect.DeepEqual(got, want), \"complete-typed-row-ledger\", nil)", "// retained old assertion: r.require(reflect.DeepEqual(got, want), \"complete-typed-row-ledger\", nil)\nr.require(true, \"complete-typed-row-ledger\", nil)"},
		{"vector-assertion-true", "r.require(reflect.DeepEqual(vectors, [][]any{{int64(-7), \"[1,0]\", int64(2), float64(0)}, {int64(2), \"[4,4]\", int64(2), float64(5)}, {int64(9), \"[0,0]\", int64(2), float64(1)}}), \"dense-vector-exact-ledger\", nil)", "r.require(true, \"dense-vector-exact-ledger\", nil)"},
		{"vector-comment-preserved-true", "r.require(reflect.DeepEqual(vectors, [][]any{{int64(-7), \"[1,0]\", int64(2), float64(0)}, {int64(2), \"[4,4]\", int64(2), float64(5)}, {int64(9), \"[0,0]\", int64(2), float64(1)}}), \"dense-vector-exact-ledger\", nil)", "// retained old assertion: r.require(reflect.DeepEqual(vectors, [][]any{{int64(-7), \"[1,0]\", int64(2), float64(0)}, {int64(2), \"[4,4]\", int64(2), float64(5)}, {int64(9), \"[0,0]\", int64(2), float64(1)}}), \"dense-vector-exact-ledger\", nil)\nr.require(true, \"dense-vector-exact-ledger\", nil)"},
		{"index-assertion-true", "r.require(reflect.DeepEqual(indexed, [][]any{{int64(-7)}, {int64(9)}}) && reflect.DeepEqual(indexed, scan) && plan.Method == \"grams13\" && plan.Access == \"SEARCH\" && reflect.DeepEqual(plan.Indexes, []string{\"docs_grams\"}) && scanPlan.Access == \"SCAN\" && scanPlan.Reason == \"NOT INDEXED requested\", \"grams-index-and-forced-scan\", nil)", "r.require(true, \"grams-index-and-forced-scan\", nil)"},
		{"index-comment-preserved-true", "r.require(reflect.DeepEqual(indexed, [][]any{{int64(-7)}, {int64(9)}}) && reflect.DeepEqual(indexed, scan) && plan.Method == \"grams13\" && plan.Access == \"SEARCH\" && reflect.DeepEqual(plan.Indexes, []string{\"docs_grams\"}) && scanPlan.Access == \"SCAN\" && scanPlan.Reason == \"NOT INDEXED requested\", \"grams-index-and-forced-scan\", nil)", "// retained old assertion: r.require(reflect.DeepEqual(indexed, [][]any{{int64(-7)}, {int64(9)}}) && reflect.DeepEqual(indexed, scan) && plan.Method == \"grams13\" && plan.Access == \"SEARCH\" && reflect.DeepEqual(plan.Indexes, []string{\"docs_grams\"}) && scanPlan.Access == \"SCAN\" && scanPlan.Reason == \"NOT INDEXED requested\", \"grams-index-and-forced-scan\", nil)\nr.require(true, \"grams-index-and-forced-scan\", nil)"},
		{"unique-assertion-true", "r.require(reflect.DeepEqual(got, [][]any{{int64(2)}}) && plan.Access == \"SCAN\" && plan.Method == \"\" && len(plan.Indexes) == 0, \"explicit-unique-supported-scan\", nil)", "r.require(true, \"explicit-unique-supported-scan\", nil)"},
		{"unique-comment-preserved-true", "r.require(reflect.DeepEqual(got, [][]any{{int64(2)}}) && plan.Access == \"SCAN\" && plan.Method == \"\" && len(plan.Indexes) == 0, \"explicit-unique-supported-scan\", nil)", "// retained old assertion: r.require(reflect.DeepEqual(got, [][]any{{int64(2)}}) && plan.Access == \"SCAN\" && plan.Method == \"\" && len(plan.Indexes) == 0, \"explicit-unique-supported-scan\", nil)\nr.require(true, \"explicit-unique-supported-scan\", nil)"},
		{"ledger-dead-branch", `r.ledger(ctx, o, "snapshot")`, `if false { r.ledger(ctx, o, "snapshot") }`},
		{"row-assertion-dead-branch", `r.require(reflect.DeepEqual(got, want), "complete-typed-row-ledger", nil)`, `if false { r.require(reflect.DeepEqual(got, want), "complete-typed-row-ledger", nil) }`},
		{"ledger-noop-comment-preserved", "r.ledger(ctx, o, \"snapshot\")", "r.event(\"ledger-noop\", nil, nil) // r.ledger(ctx, o, \"snapshot\")"},
		{"pending-parent-bypass", "formatupgrade.CanRetire(o.state, r.states(), r.receipt.InternalPendingObserved)", "formatupgrade.CanRetire(o.state, r.states(), false)"},
		{"pending-capture-noop", "r.capturePending(err)", "r.causes = append(r.causes, err) // r.capturePending(err)"},
	}
	for _, m := range mutants {
		t.Run(m.name, func(t *testing.T) {
			if !strings.Contains(string(raw), m.old) {
				t.Fatal("mutant did not match")
			}
			changed := []byte(strings.Replace(string(raw), m.old, m.new, 1))
			findings, err := inspectFormatUpgradeStructure(formatUpgradeNative, changed)
			if err == nil && len(findings) == 0 {
				t.Fatal("structural mutant accepted")
			}
		})
	}
	findings, err = inspectFormatUpgradeStructure("sqlite/copied_upgrade_test.go", raw)
	if err == nil && len(findings) == 0 {
		t.Fatal("copied native source accepted")
	}
}
func TestFormatUpgradePureClosureCounterexamples(t *testing.T) {
	for _, path := range []string{formatUpgradePure, formatUpgradePureTest} {
		raw := formatUpgradeReadSource(t, path)
		findings, err := inspectFormatUpgradeStructure(path, raw)
		if err != nil || len(findings) != 0 {
			t.Fatal(path, findings, err)
		}
		changed := []byte(strings.Replace(string(raw), "import (", "import (\n\"os\"", 1))
		findings, err = inspectFormatUpgradeStructure(path, changed)
		if err == nil && len(findings) == 0 {
			t.Fatal("native import admitted to pure helper")
		}
	}
	for _, raw := range []string{
		"package formatupgrade; import \"testing\"; func TestBad(t *testing.T){t.TempDir()}",
		"package formatupgrade; var fired = sideEffect()",
		"package formatupgrade; func init(){}",
	} {
		findings, err := inspectFormatUpgradeStructure(formatUpgradePureTest, []byte(raw))
		if err == nil && len(findings) == 0 {
			t.Fatal("pure closure mutant accepted")
		}
	}
	findings, err := inspectFormatUpgradeStructure("sqlite/consumer_test.go", []byte("package sqlite_test; import \""+formatUpgradeImport+"\""))
	if err == nil && len(findings) == 0 {
		t.Fatal("unreviewed helper consumer accepted")
	}
}
func TestFormatUpgradeMainKeepsExistingPolicies(t *testing.T) {
	raw := string(formatUpgradeReadSource(t, "internal/testscope/main.go"))
	for _, required := range []string{
		"manual, err := inspectBackupManualSource(path, raw)", "findings = append(findings, manual...)",
		"ordinary, inspectErr := inspectSource(path, raw)", "findings = append(findings, ordinary...)",
		"publication, err := inspectWALPublicationSource(path, raw)", "findings = append(findings, publication...)",
		"startup, err := inspectOracleSource(path, raw)", "findings = append(findings, startup...)",
		"contracts, err := inspectOracleContracts(\".\")", "manualContracts, manualErr := inspectBackupManualContracts(\".\")",
		"publicationContracts, err := inspectWALPublicationContracts(\".\")", "darwinManual, err := inspectDarwinInventoryManualContract(\".\")",
		"formatUpgrade, formatErr := inspectFormatUpgradeSource(path, raw)", "findings = append(findings, formatUpgrade...)",
		"formatContracts, formatErr := inspectFormatUpgradeContracts(\".\")",
	} {
		if strings.Count(raw, required) != 1 {
			t.Fatal("missing/duplicate policy propagation", required)
		}
	}
}
