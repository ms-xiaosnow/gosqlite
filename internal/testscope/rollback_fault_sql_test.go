package main

// SOURCE_NOT_RUN: these are inert text/AST counterexamples. No Go run, native
// fixture, Commit, recovery, target binary or subprocess is invoked here.
import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func rollbackFaultSQLSources(t *testing.T) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	for path := range rollbackFaultSQLPins {
		raw, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		out[path] = raw
	}
	return out
}
func TestRollbackFaultSQLSourceContracts(t *testing.T) {
	for path, raw := range rollbackFaultSQLSources(t) {
		findings, err := inspectRollbackFaultSQLSource(path, raw)
		if err != nil || len(findings) != 0 {
			t.Fatal(path, findings, err)
		}
		old, err := inspectRollbackFaultSource(path, raw)
		if err != nil || len(old) != 0 {
			t.Fatal("narrow old boundary rejected SQL source", path, old, err)
		}
		if path == rollbackFaultSQLNative {
			ordinary, err := inspectSource(path, raw)
			if err != nil || len(ordinary) != 0 {
				t.Fatal("exact recovery reference rejected", ordinary, err)
			}
		}
	}
}

// Both byte and full declaration/body snapshots are repinned. A negative must
// hit an independent semantic diagnostic, never just an obsolete snapshot.
func TestRollbackFaultSQLRepinnedSemanticCounterexamples(t *testing.T) {
	sources := rollbackFaultSQLSources(t)
	variants := []struct{ name, path, old, replacement, finding string }{
		{"bridge error case mapped to nil", rollbackFaultSQLBridge, "caseID = rollbackfault.SQLIndexPrefix64Error", "caseID = rollbackfault.SQLIndexPrefix64", "bridge frozen four-case plan"},
		{"bridge prefix bit flipped", rollbackFaultSQLBridge, "if spec.PrefixError {", "if !spec.PrefixError {", "bridge frozen four-case plan"},
		{"new prefix bit dropped", rollbackFaultSQLModel, "PrefixShortWrite, 64, true", "PrefixShortWrite, 64, false", "frozen three-page four-case selectors"},
		{"new prefix length changed", rollbackFaultSQLModel, "PrefixShortWrite, 64, true", "PrefixShortWrite, 63, true", "frozen three-page four-case selectors"},
		{"new plan name aliased", rollbackFaultSQLModel, "case SQLIndexPrefix64Error:", "case SQLCase(5):", "four exact fixed plan branches"},
		{"new admission mapped to nil", rollbackFaultSQLNative, "c.caseID = rollbackfault.SQLIndexPrefix64Error", "c.caseID = rollbackfault.SQLIndexPrefix64", "four exact names and case mappings"},
		{"new admission name widened", rollbackFaultSQLNative, `case "index-prefix-64-error":`, `case "index-prefix-64-error", "another-prefix-error":`, "four exact names and case mappings"},
		{"public plan case binding dropped", rollbackFaultSQLNative, "problem != rollbackfault.NoProblem || frozen.Spec() != spec", "problem != rollbackfault.NoProblem", "exact case-bound public return modes"},
		{"public new cause inverted", rollbackFaultSQLNative, "c.caseID == rollbackfault.SQLIndexPrefix64Error && errors.Is(commitErr, io.ErrShortWrite)", "c.caseID == rollbackfault.SQLIndexPrefix64Error && !errors.Is(commitErr, io.ErrShortWrite)", "exact case-bound public return modes"},
		{"public new error accepted as nil", rollbackFaultSQLNative, `return report, fmt.Errorf("injected prefix error confused with nil-error short write")`, `return report, nil`, "exact case-bound public return modes"},
		{"nil prefix public cause no longer required", rollbackFaultSQLNative, "c.caseID == rollbackfault.SQLIndexPrefix64 && !errors.Is(commitErr, io.ErrShortWrite)", "false", "exact case-bound public return modes"},
		{"prefix tail checks only nil case", rollbackFaultSQLNative, "(c.caseID == rollbackfault.SQLIndexPrefix64 || c.caseID == rollbackfault.SQLIndexPrefix64Error) && !bytes.Equal", "c.caseID == rollbackfault.SQLIndexPrefix64 && !bytes.Equal", "root independent baseline and copy-only recovery order"},
		{"prefix error claims public shortwrite", rollbackFaultSQLModel, "lifecycle.PublicShortWrite = evidence.Case == SQLIndexPrefix64", "lifecycle.PublicShortWrite = evidence.Case == SQLIndexPrefix64 || evidence.Case == SQLIndexPrefix64Error", "one-shot public error lifecycle"},
		{"bridge OR tag", rollbackFaultSQLBridge, rollbackFaultManualTag, "//go:build linux || manual_fault_rollback", "exact Linux AND manual SQL gate"},
		{"root OR tag", rollbackFaultSQLNative, rollbackFaultManualTag, "//go:build linux || manual_fault_rollback", "exact Linux AND manual SQL gate"},
		{"bridge permissive environment", rollbackFaultSQLBridge, `!= "independently_authorized"`, `== "independently_authorized"`, "bridge first action"},
		{"bridge action before environment", rollbackFaultSQLBridge, `if os.Getenv("GOSQLITE_MANUAL_FAULT_ROLLBACK")`, `_ = plan.Spec(); if os.Getenv("GOSQLITE_MANUAL_FAULT_ROLLBACK")`, "bridge first action"},
		{"root weakened environment", rollbackFaultSQLNative, `!= "independently_authorized"`, `== "independently_authorized"`, "root first action"},
		{"root action before gate", rollbackFaultSQLNative, `if os.Getenv("GOSQLITE_MANUAL_FAULT_ROLLBACK")`, `_ = os.Args; if os.Getenv("GOSQLITE_MANUAL_FAULT_ROLLBACK")`, "root first action"},
		{"extra manual root", rollbackFaultSQLNative, "func TestManualRollbackFaultSQLLedger", "func TestExtraManualSQL(t *testing.T) {}\nfunc TestManualRollbackFaultSQLLedger", "one exact manual SQL root"},
		{"bridge owner close", rollbackFaultSQLBridge, "var out ManualRollbackSQLSnapshot", "_ = tx.file.Close(); var out ManualRollbackSQLSnapshot", "bridge cannot call native owner"},
		{"bridge reset registry", rollbackFaultSQLBridge, "var out ManualRollbackSQLSnapshot", "rollbackFaultRegistry.used = false; var out ManualRollbackSQLSnapshot", "snapshot cannot mutate"},
		{"bridge deletes retirement registry", rollbackFaultSQLBridge, "if g.pending.Load() != nil {\n\t\t\tout.Pending++\n\t\t}", "delete(plainRetirements.groups,g); if g.pending.Load() != nil { out.Pending++ }", "bridge closed call inventory"},
		{"bridge clears retirement registry", rollbackFaultSQLBridge, "var out ManualRollbackSQLSnapshot", "clear(plainRetirements.groups); var out ManualRollbackSQLSnapshot", "bridge closed call inventory"},
		{"snapshot owner pointer", rollbackFaultSQLBridge, "Groups        int", "File *os.File; Groups int", "snapshot must expose exactly five"},
		{"actual fourth page admitted", rollbackFaultSQLBridge, "tx.originalPages == 3", "tx.originalPages == 4", "exact actual three-page writer extent"},
		{"pending retirement ignored", rollbackFaultSQLBridge, "state.Groups != 0 || state.Pending != 0", "state.Groups != 0", "bridge no pending owner"},
		{"private entry alias", rollbackFaultSQLBridge, "scope, err := armRollbackFault(tx, plan)", "arm := armRollbackFault; scope, err := arm(tx, plan)", "bridge frozen four-case plan"},
		{"storage Commit bypass", rollbackFaultSQLNative, "commitErr := tx.Commit(ctx)", "commitErr := tx.storage.Commit(ctx)", "one public SQL Commit only"},
		{"hidden bootstrap Commit", rollbackFaultSQLNative, "var image [12288]byte", "_ = bootstrap.Commit(ctx); var image [12288]byte", "one public SQL Commit only"},
		{"second public Commit", rollbackFaultSQLNative, "commitErr := tx.Commit(ctx)", "_ = tx.Commit(ctx); commitErr := tx.Commit(ctx)", "exactly one public SQL Commit reference"},
		{"root repeats Commit helper", rollbackFaultSQLNative, `report, commitCheckErr := manualRollbackSQLCommit(&c, ctx, filepath.Join(c.root, "trial.db"), plan)`, `_, _ = manualRollbackSQLCommit(&c, ctx, filepath.Join(c.root, "trial.db"), plan); report, commitCheckErr := manualRollbackSQLCommit(&c, ctx, filepath.Join(c.root, "trial.db"), plan)`, "one root Commit helper call"},
		{"root loops Commit helper", rollbackFaultSQLNative, `report, commitCheckErr := manualRollbackSQLCommit(&c, ctx, filepath.Join(c.root, "trial.db"), plan)`, `for i := 0; i < 1; i++ { report, commitCheckErr := manualRollbackSQLCommit(&c, ctx, filepath.Join(c.root, "trial.db"), plan); _,_ = report,commitCheckErr }`, "one root Commit/baseline helper must"},
		{"root retains Commit helper", rollbackFaultSQLNative, `report, commitCheckErr := manualRollbackSQLCommit(&c, ctx, filepath.Join(c.root, "trial.db"), plan)`, `commit := manualRollbackSQLCommit; report, commitCheckErr := commit(&c, ctx, filepath.Join(c.root, "trial.db"), plan)`, "one root Commit/baseline helper cannot"},
		{"Commit function value", rollbackFaultSQLNative, "commitErr := tx.Commit(ctx)", "commit := tx.Commit; commitErr := commit(ctx)", "one public SQL Commit only"},
		{"Finish delayed", rollbackFaultSQLNative, "report := pager.FinishManualRollbackSQLFault(scope, commitErr)", "_ = commitErr; report := pager.FinishManualRollbackSQLFault(scope, commitErr)", "immediately followed by Finish"},
		{"Finish error substituted", rollbackFaultSQLNative, "pager.FinishManualRollbackSQLFault(scope, commitErr)", "pager.FinishManualRollbackSQLFault(scope, nil)", "SQL bridge entry outside exact"},
		{"public error identity dropped", rollbackFaultSQLNative, "report.CommitError != commitErr || ", "", "public error identity"},
		{"IOErr classification weakened", rollbackFaultSQLNative, "publicError.Code != CodeIOErr", "publicError.Code == CodeIOErr", "public error identity"},
		{"recovery sentinel ignored", rollbackFaultSQLNative, "!errors.Is(commitErr, ErrRecoveryRequired)", "false", "public error identity"},
		{"cleanup pending ignored", rollbackFaultSQLNative, "errors.Is(commitErr, pager.ErrPlaintextCleanupPending)", "false", "public error identity"},
		{"healthy cleanup bypass", rollbackFaultSQLNative, "state.AdapterActive || state.AdapterUsed || state.Pending != 0 || !state.HealthyTx", "state.AdapterActive", "healthy pre-arm abandonment"},
		{"retry deferred owner", rollbackFaultSQLNative, "func (l *manualRollbackSQLLedger) finish(t *testing.T) {", "func (l *manualRollbackSQLLedger) finish(t *testing.T) { _ = l.close(0)", "deferred owner report cannot"},
		{"owner double close", rollbackFaultSQLNative, "if o.closes != 0", "if o.closes > 1", "owner close-once"},
		{"open creates files", rollbackFaultSQLNative, "NoCreate: true", "NoCreate: false", "bounded independent no-create"},
		{"readback writable", rollbackFaultSQLNative, "manualRollbackSQLOpen(c, path, true)", "manualRollbackSQLOpen(c, path, false)", "fresh read-only checked snapshot"},
		{"recovery unbounded journal", rollbackFaultSQLNative, "MaxJournalBytes: 16408", "MaxJournalBytes: 0", "recovery outside exact helper"},
		{"recovery extra records", rollbackFaultSQLNative, "MaxRecords: 3", "MaxRecords: 4", "recovery outside exact helper"},
		{"recovery wrong extent", rollbackFaultSQLNative, "MaxDatabaseBytes: 12288", "MaxDatabaseBytes: 16384", "recovery outside exact helper"},
		{"recovery original basename", rollbackFaultSQLNative, `filepath.Base(replayPath) != "replay.db"`, `filepath.Base(replayPath) != "trial.db"`, "fresh replay copy exact bounded recovery"},
		{"recovery copied helper", rollbackFaultSQLNative, "func manualRollbackSQLRecoverCopy(", "func rogueRecoverCopy(", "recovery outside exact helper"},
		{"recovery function value", rollbackFaultSQLNative, "result, recoveryErr := pager.RecoverRollback(", "recover := pager.RecoverRollback; result, recoveryErr := recover(", "recovery outside exact helper"},
		{"equal-count old-new baseline", rollbackFaultSQLNative, `table := [][]any{{nil, "red", "one"}, {nil, "blue", "two"}, {nil, "red", "three"}}`, `table := [][]any{{nil, "green", "ONE"}, {nil, "blue", "two"}, {nil, "red", "three"}}`, "fixed independent old three-page"},
		{"Rows Err omitted", rollbackFaultSQLNative, "rowsErr := rows.Err()", "rowsErr := error(nil)", "real typed SQL rows"},
		{"Rows int type widened", rollbackFaultSQLNative, "values[0].(int64)", "values[0].(string)", "real typed SQL rows"},
		{"Check omitted", rollbackFaultSQLNative, "db.Check(ctx, CheckOptions{MaxPages: 3, MaxPageReads: 512})", "fakeCheck(ctx)", "fresh read-only checked snapshot"},
		{"test run regex union", rollbackFaultSQLNative, `"^TestManualRollbackFaultSQLLedger$"`, `"TestManualRollbackFaultSQLLedger|TestOther"`, "one exact fresh-process selection"},
		{"test repeated case", rollbackFaultSQLNative, `"-test.count": "1"`, `"-test.count": "3"`, "one exact fresh-process selection"},
		{"plan wrong short prefix", rollbackFaultSQLModel, "PrefixShortWrite, 64, false", "PrefixShortWrite, 65, false", "frozen three-page four-case selectors"},
		{"plan wrong DBPage occurrence", rollbackFaultSQLModel, "Point: DBPage, Occurrence: 3", "Point: DBPage, Occurrence: 2", "frozen three-page four-case selectors"},
		{"plan wrong third-page offset", rollbackFaultSQLModel, "Point: DBPage, Occurrence: 3, Role: Database, Owner: 11, Offset: 8192", "Point: DBPage, Occurrence: 3, Role: Database, Owner: 11, Offset: 4096", "frozen three-page four-case selectors"},
		{"model count-only rows", rollbackFaultSQLModel, "query.RowCount != count || query.Rows != rows", "query.RowCount != count", "full rows, Check, EOF"},
		{"model receipt trusted", rollbackFaultSQLModel, "result.Adapter = receipt.Seal()", "result.Adapter = Observation{Outcome: AdapterObserved}", "receipt identity and old/new"},
		{"model old-new mixture", rollbackFaultSQLModel, "state := SQLOld", "state := SQLNew", "receipt identity and old/new"},
		{"model index zero-work accepted", rollbackFaultSQLModel, "work.IndexPageReads > 0", "work.IndexPageReads >= 0", "index claim requires actual bounded work"},
		{"model EOF ignored", rollbackFaultSQLModel, "!query.EOF || !query.ErrCalled || ", "", "full rows, Check, EOF"},
		{"model digest substitution", rollbackFaultSQLModel, "original.Frozen != original.After", "original.Before != original.After", "receipt identity and old/new"},
		{"control recovery call admitted", rollbackFaultSQLModel, "SQLRecoveryEvidence{Identity: recovery.Identity, OwnersRetired: true}", "SQLRecoveryEvidence{Identity: recovery.Identity, OwnersRetired: true, Calls: 1}", "control no recovery; fault exact one bounded replay"},
		{"model physical pass", rollbackFaultSQLModel, "func (SQLObservation) Pass() bool              { return false }", "func (SQLObservation) Pass() bool { return true }", "cannot confer physical"},
		{"model owner pointer", rollbackFaultSQLModel, "type SQLRow struct {", "type SQLRow struct { owner *SQLRow", "descriptor-free fixed scalar"},
		{"model aliases replay", rollbackFaultSQLModel, "before.Device == other.Device && before.Inode == other.Inode", "before.Device == other.Device && false", "immutable originals and unaliased"},
	}
	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			original := string(sources[variant.path])
			if !strings.Contains(original, variant.old) {
				t.Fatal("source-only counterexample anchor missing", variant.path, variant.old)
			}
			changed := []byte(strings.Replace(original, variant.old, variant.replacement, 1))
			pin, shape := rollbackFaultSQLPins[variant.path], rollbackFaultSQLShapes[variant.path]
			rollbackFaultSQLPins[variant.path] = fmt.Sprintf("%x", sha256.Sum256(changed))
			rollbackFaultSQLShapes[variant.path] = string(changed)
			t.Cleanup(func() { rollbackFaultSQLPins[variant.path], rollbackFaultSQLShapes[variant.path] = pin, shape })
			findings, err := inspectRollbackFaultSQLSource(variant.path, changed)
			if err != nil {
				t.Fatal("counterexample must parse", err)
			}
			joined := strings.Join(findings, "\n")
			if !strings.Contains(joined, variant.finding) {
				t.Fatal("semantic rejection missing after both snapshots were repinned", variant.finding, findings)
			}
			if strings.Contains(joined, "snapshot changed") || strings.Contains(joined, "source binding/body/order changed") {
				t.Fatal("negative relied on supplemental snapshot", findings)
			}
		})
	}
}

func TestRollbackFaultSQLOutsideConsumersAndOldEntriesRejected(t *testing.T) {
	variants := []struct{ path, source, finding string }{
		{"internal/pager/rogue.go", `package pager; func rogue(){ ArmManualRollbackSQLFault(nil,plan) }`, "SQL bridge entry outside exact"},
		{"sqlite/rogue_test.go", `package sqlite; func TestRogue(){ pager.FinishManualRollbackSQLFault(scope,err) }`, "SQL bridge entry outside exact"},
		{"internal/pager/rogue.go", `package pager; var arm = ArmManualRollbackSQLFault`, "SQL bridge entry outside exact"},
		{"internal/pager/rogue.go", `package pager; var inspect = ManualRollbackSQLState`, "SQL bridge entry outside exact"},
		{"internal/pager/rogue.go", `package pager; func ManualRollbackSQLState(){}`, "SQL bridge entry outside exact"},
		{"sqlite/copied_test.go", rollbackFaultManualTag + "\npackage sqlite; func TestManualRollbackFaultSQLLedger(t *testing.T) {}", "SQL manual declaration/reference outside exact"},
		{"internal/testkit/rollbackfault/sql_unreviewed.go", `package rollbackfault; func extra(){}`, "unreviewed SQL adapter/model/caller source"},
		{"internal/pager/rogue.go", "package pager\nimport \"" + rollbackFaultImport + "\"", "contract import outside exact"},
		{rollbackFaultSQLBridge, "package pager\nimport rf \"" + rollbackFaultImport + "\"", "aliased, blank or dot import forbidden"},
	}
	for _, variant := range variants {
		findings, err := inspectRollbackFaultSQLSource(variant.path, []byte(variant.source))
		if err != nil || !strings.Contains(strings.Join(findings, "\n"), variant.finding) {
			t.Fatal(variant.path, findings, err)
		}
	}
	oldVariants := []string{
		`package pager; func rogue(){ armRollbackFault(tx,plan) }`,
		`package pager; func ArmManualRollbackSQLFault(){ arm := armRollbackFault; arm(tx,plan) }`,
		`package pager; func ArmManualRollbackSQLFault(){ armRollbackFault(other,plan) }`,
		`package pager; func FinishManualRollbackSQLFault(){ finishRollbackFault(scope.scope,nil) }`,
		`package pager; func DisarmManualRollbackSQLFault(){ disarmRollbackFault(other) }`,
		`package pager; var scope = rollbackFaultRegistry`,
		`package pager; type alias = rollbackFaultScope`,
		`package pager; func rogue(){ rollbackFaultBefore(tx,point) }`,
	}
	for _, raw := range oldVariants {
		findings, err := inspectRollbackFaultSource(rollbackFaultSQLBridge, []byte(raw))
		if err != nil || len(findings) == 0 {
			t.Fatal("new bridge path broadened old-private boundary", findings, err)
		}
	}
	for path, raw := range rollbackFaultSQLSources(t) {
		findings, err := inspectRollbackFaultSQLSource("copied/"+filepath.Base(path), raw)
		if err != nil || len(findings) == 0 {
			t.Fatal("whole copied SQL source accepted", path, findings, err)
		}
	}
}

func TestRollbackFaultSQLOrdinaryRecoveryExceptionIsOneReference(t *testing.T) {
	valid := rollbackFaultManualTag + `\n\npackage sqlite
import (
 "os"
 "context"
 "github.com/ms-xiaosnow/gosqlite/internal/pager"
 "github.com/ms-xiaosnow/gosqlite/internal/journal"
)
func manualRollbackSQLRecoverCopy(ctx context.Context, replayPath string) (pager.RecoveryResult, error) {
 return pager.RecoverRollback(ctx, replayPath, pager.RecoveryOptions{Limits: journal.ReplayLimits{MaxJournalBytes: 16408, MaxRecords: 3, MaxDatabaseBytes: 12288}})
}
`
	valid = strings.Replace(valid, `\n\n`, "\n\n", 1)
	findings, err := inspectSource(rollbackFaultSQLNative, []byte(valid))
	if err != nil || len(findings) != 0 {
		t.Fatal("one exact reference not accepted", findings, err)
	}
	variants := []struct{ path, old, replacement string }{
		{"sqlite/copied_linux_test.go", "", ""},
		{rollbackFaultSQLNative, rollbackFaultManualTag, "//go:build linux || manual_fault_rollback"},
		{rollbackFaultSQLNative, "manualRollbackSQLRecoverCopy", "manualRollbackSQLRecoverOriginal"},
		{rollbackFaultSQLNative, "MaxRecords: 3", "MaxRecords: 4"},
		{rollbackFaultSQLNative, "ctx, replayPath, pager.RecoveryOptions", "ctx, originalPath, pager.RecoveryOptions"},
		{rollbackFaultSQLNative, "return pager.RecoverRollback(", "recover := pager.RecoverRollback; return recover("},
		{rollbackFaultSQLNative, `"github.com/ms-xiaosnow/gosqlite/internal/pager"`, `pager "example.invalid/rogue"`},
		{rollbackFaultSQLNative, "return pager.RecoverRollback(", "_ = os.Remove(replayPath); return pager.RecoverRollback("},
		{rollbackFaultSQLNative, "return pager.RecoverRollback(", "_ = Options{RollbackRecovery: recovery}; return pager.RecoverRollback("},
	}
	for _, variant := range variants {
		raw := valid
		if variant.old != "" {
			raw = strings.Replace(raw, variant.old, variant.replacement, 1)
		}
		findings, err := inspectSource(variant.path, []byte(raw))
		if err != nil || len(findings) == 0 {
			t.Fatal("ordinary restricted-reference policy broadened", variant, findings, err)
		}
	}
}

func TestRollbackFaultSQLWiringAndMainReviewHold(t *testing.T) {
	raw, err := os.ReadFile("main.go")
	if err != nil {
		t.Fatal(err)
	}
	// The independent whole-main review releases only these exact source bytes.
	// An unreviewed byte change must still produce the original HOLD diagnostic.
	const reviewedMainSHA256 = "e31c233fed8976fddff3236a8804f84d493e2567d6b6d85d11355de2ca0da52b"
	if walPublicationFrozenSources["internal/testscope/main.go"] != reviewedMainSHA256 || formatUpgradePins["internal/testscope/main.go"] != reviewedMainSHA256 {
		t.Fatal("whole-main guard pins differ from independently reviewed literal")
	}
	if fmt.Sprintf("%x", sha256.Sum256(raw)) != reviewedMainSHA256 {
		t.Fatal("main.go bytes differ from independently reviewed literal")
	}
	if findings := inspectRollbackFaultSQLWiring(raw); len(findings) != 0 {
		t.Fatal("independently reviewed SQL wiring rejected", findings)
	}
	if findings := inspectWALPublicationFrozenSource("internal/testscope/main.go", raw); len(findings) != 0 {
		t.Fatal("independently reviewed whole-main source rejected", findings)
	}
	unreviewed := append(append([]byte(nil), raw...), []byte("\n// unreviewed SQL main.go change\n")...)
	if findings := inspectRollbackFaultSQLWiring(unreviewed); len(findings) != 1 || !strings.Contains(findings[0], rollbackFaultSQLMainReview) {
		t.Fatal("unreviewed main.go comment must retain SQL HOLD", findings)
	}
	if findings := inspectWALPublicationFrozenSource("internal/testscope/main.go", unreviewed); len(findings) == 0 {
		t.Fatal("unreviewed main.go comment accepted by existing whole-main guard")
	}
	variants := []struct{ old, replacement, finding string }{
		{"findings = append(findings, rollbackSQL...)", "_ = rollbackSQL", "direct all-source hook"},
		{"return rollbackSQLErr", "return nil", "direct all-source hook"},
		{"fmt.Fprintln(os.Stderr, rollbackSQLContractErr)", "fmt.Fprintln(os.Stderr, nil)", "root SQL contract errors"},
		{"for _, finding := range rollbackSQLContracts", "for _, finding := range []string{}", "root SQL contract errors"},
		{"inspectFormatUpgradeSource(path, raw)", "inspectOtherSource(path, raw)", "missing/duplicate prior"},
		{"inspectRollbackFaultContracts(\".\")", "inspectOtherContracts(\".\")", "missing/duplicate prior"},
		{"inspectReadOnlyWALManualSource(path, raw)", "inspectOtherSource(path, raw)", "missing/duplicate prior"},
	}
	for _, variant := range variants {
		if !strings.Contains(string(raw), variant.old) {
			t.Fatal("wiring counterexample anchor missing", variant.old)
		}
		changed := []byte(strings.Replace(string(raw), variant.old, variant.replacement, 1))
		if findings := inspectRollbackFaultSQLWiring(changed); !strings.Contains(strings.Join(findings, "\n"), variant.finding) {
			t.Fatal("wiring weakening accepted", variant, findings)
		}
	}
}

// Read-only source regressions: none of these four example packages is run.
func rollbackFaultSQLOrdinaryExampleSources(t *testing.T) map[string][]byte {
	t.Helper()
	sources := map[string][]byte{}
	for _, path := range []string{"examples/ann/main.go", "examples/ann/main_test.go", "examples/vector/main.go", "examples/vector/main_test.go"} {
		raw, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(path, err)
		}
		sources[path] = raw
	}
	return sources
}

func TestRollbackFaultSQLOrdinaryExampleFields(t *testing.T) {
	sources := rollbackFaultSQLOrdinaryExampleSources(t)
	for path, raw := range sources {
		t.Run(path, func(t *testing.T) {
			reads := 0
			wantSource := ""
			wantReads := 0
			if strings.HasSuffix(path, "_test.go") {
				wantSource = strings.TrimSuffix(path, "_test.go") + ".go"
				wantReads = 1
			}
			findings, err := inspectRollbackFaultSQLSourceWithReader(path, raw, func(sourcePath string) ([]byte, error) {
				reads++
				if wantReads != 1 || sourcePath != wantSource {
					t.Fatalf("unbounded source read %q for %q", sourcePath, path)
				}
				return os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(sourcePath)))
			})
			if err != nil || len(findings) != 0 || reads != wantReads {
				t.Fatal("ordinary sqlite.QueryPlan field rejected", path, findings, err, reads, wantReads)
			}
		})
	}
}

func TestRollbackFaultSQLOrdinaryExamplesKeepModelBoundary(t *testing.T) {
	sources := rollbackFaultSQLOrdinaryExampleSources(t)
	variants := []struct{ name, imports, body, finding string }{
		{"bare call", "", `SQLPlan(identity, caseID)`, "SQL model declaration/reference"},
		{"bare function value", "", `_ = SQLPlan`, "SQL model declaration/reference"},
		{"model call", `import "` + rollbackFaultImport + `"`, `rollbackfault.SQLPlan(identity, caseID)`, "SQL model declaration/reference"},
		{"aliased model call", `import rf "` + rollbackFaultImport + `"`, `rf.SQLPlan(identity, caseID)`, "SQL model declaration/reference"},
		{"model function value", `import "` + rollbackFaultImport + `"`, `f := rollbackfault.SQLPlan; _ = f`, "SQL model declaration/reference"},
		{"aliased model function value", `import rf "` + rollbackFaultImport + `"`, `f := rf.SQLPlan; _ = f`, "SQL model declaration/reference"},
		{"unaliased model import", `import "` + rollbackFaultImport + `"`, ``, "contract import outside exact"},
		{"aliased model import", `import rf "` + rollbackFaultImport + `"`, ``, "contract import outside exact"},
		{"blank model import", `import _ "` + rollbackFaultImport + `"`, ``, "contract import outside exact"},
		{"dot model import", `import . "` + rollbackFaultImport + `"`, `SQLPlan(identity, caseID)`, "contract import outside exact"},
	}
	for path, raw := range sources {
		for _, variant := range variants {
			t.Run(path+"/"+variant.name, func(t *testing.T) {
				changed := strings.Replace(string(raw), "package main", "package main\n"+variant.imports, 1) + "\nfunc ordinaryFieldCounterexample() { " + variant.body + " }\n"
				findings, err := inspectRollbackFaultSQLSourceWithReader(path, []byte(changed), func(sourcePath string) ([]byte, error) {
					if sourcePath != "examples/ann/main.go" && sourcePath != "examples/vector/main.go" {
						t.Fatal("unbounded companion", sourcePath)
					}
					return sources[sourcePath], nil
				})
				if err != nil || !strings.Contains(strings.Join(findings, "\n"), variant.finding) {
					t.Fatal("ordinary field exemption admitted model access", variant.name, findings, err)
				}
			})
		}
	}
}

func TestRollbackFaultSQLOrdinaryFieldBindingsReject(t *testing.T) {
	const valid = `package main
import "github.com/ms-xiaosnow/gosqlite/sqlite"
type output struct { SQLPlan sqlite.QueryPlan }
func inspect() { var got output; _ = got.SQLPlan.Method == "hnsw" }
`
	variants := []struct{ name, old, replacement string }{
		{"wrong field type", "SQLPlan sqlite.QueryPlan", "SQLPlan int"},
		{"function field", "SQLPlan sqlite.QueryPlan", "SQLPlan func()"},
		{"pointer field", "SQLPlan sqlite.QueryPlan", "SQLPlan *sqlite.QueryPlan"},
		{"wrong sqlite import", "github.com/ms-xiaosnow/gosqlite/sqlite", "example.invalid/sqlite"},
		{"explicit sqlite alias", `import "`, `import sqlite "`},
		{"type alias", "type output struct", "type output = struct"},
		{"duplicate field", "SQLPlan sqlite.QueryPlan", "SQLPlan sqlite.QueryPlan; SQLPlan sqlite.QueryPlan"},
		{"missing field", "SQLPlan sqlite.QueryPlan", "Other sqlite.QueryPlan"},
		{"local type shadow", "var got output", "type output int; var got output"},
		{"local type alias", "var got output", "type local = output; var got local"},
		{"receiver alias", "var got output", "var original output; got := original"},
		{"receiver pointer", "var got output", "var got *output"},
		{"receiver call", "got.SQLPlan.Method", "makeOutput().SQLPlan.Method"},
		{"retained field value", `_ = got.SQLPlan.Method == "hnsw"`, "_ = got.SQLPlan"},
		{"field call", `_ = got.SQLPlan.Method == "hnsw"`, "got.SQLPlan()"},
		{"parenthesized field call", `_ = got.SQLPlan.Method == "hnsw"`, "(got.SQLPlan)()"},
		{"scalar field called", `_ = got.SQLPlan.Method == "hnsw"`, "got.SQLPlan.Method()"},
		{"other selector", "got.SQLPlan.Method", "got.SQLPlan.Unreviewed"},
		{"composite field key", `_ = got.SQLPlan.Method == "hnsw"`, "_ = output{SQLPlan: sqlite.QueryPlan{}}"},
		{"other package", "package main", "package copied"},
		{"other reserved name", "SQLPlan", "ObserveSQLLedger"},
	}
	noReads := func(path string) ([]byte, error) {
		t.Fatal("main source must not load a companion", path)
		return nil, fmt.Errorf("unexpected companion")
	}
	if findings, err := inspectRollbackFaultSQLSourceWithReader("examples/ann/main.go", []byte(valid), noReads); err != nil || len(findings) != 0 {
		t.Fatal("bounded positive source rejected", findings, err)
	}
	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			if !strings.Contains(valid, variant.old) {
				t.Fatal("counterexample anchor missing", variant.name)
			}
			raw := []byte(strings.ReplaceAll(valid, variant.old, variant.replacement))
			findings, err := inspectRollbackFaultSQLSourceWithReader("examples/ann/main.go", raw, noReads)
			if err != nil || !strings.Contains(strings.Join(findings, "\n"), "SQL model declaration/reference") {
				t.Fatal("unresolved ordinary-field shape accepted", variant.name, findings, err)
			}
		})
	}
	for _, path := range []string{"examples/copied/main.go", "examples/ann/other_test.go", "internal/pager/rogue.go"} {
		findings, err := inspectRollbackFaultSQLSourceWithReader(path, []byte(valid), noReads)
		if err != nil || !strings.Contains(strings.Join(findings, "\n"), "SQL model declaration/reference") {
			t.Fatal("ordinary field exemption escaped exact source set", path, findings, err)
		}
	}
}

func TestRollbackFaultSQLOrdinaryCompanionFailures(t *testing.T) {
	sources := rollbackFaultSQLOrdinaryExampleSources(t)
	variants := []struct {
		name, old, replacement string
		missing, malformed     bool
	}{
		{name: "missing source", missing: true},
		{name: "malformed source", malformed: true},
		{name: "wrong package", old: "package main", replacement: "package copied"},
		{name: "wrong concrete type", old: " struct {", replacement: " int; type other struct {"},
		{name: "wrong field type", old: "sqlite.QueryPlan", replacement: "int"},
		{name: "callable field", old: "sqlite.QueryPlan", replacement: "func()"},
		{name: "aliased sqlite import", old: `"github.com/ms-xiaosnow/gosqlite/sqlite"`, replacement: `sqlite "github.com/ms-xiaosnow/gosqlite/sqlite"`},
		{name: "wrong sqlite import", old: "github.com/ms-xiaosnow/gosqlite/sqlite", replacement: "example.invalid/sqlite"},
		{name: "model import", old: "package main", replacement: "package main\nimport \"" + rollbackFaultImport + "\""},
		{name: "aliased model import", old: "package main", replacement: "package main\nimport rf \"" + rollbackFaultImport + "\""},
	}
	for _, path := range []string{"examples/ann/main_test.go", "examples/vector/main_test.go"} {
		sourcePath := strings.TrimSuffix(path, "_test.go") + ".go"
		for _, variant := range variants {
			t.Run(path+"/"+variant.name, func(t *testing.T) {
				companion := sources[sourcePath]
				if variant.malformed {
					companion = []byte("package main; type broken struct {")
				} else if variant.old != "" {
					if !strings.Contains(string(companion), variant.old) {
						t.Fatal("companion counterexample anchor missing", variant.name)
					}
					companion = []byte(strings.ReplaceAll(string(companion), variant.old, variant.replacement))
				}
				reads := 0
				findings, err := inspectRollbackFaultSQLSourceWithReader(path, sources[path], func(actual string) ([]byte, error) {
					reads++
					if actual != sourcePath {
						t.Fatal("unexpected companion source", actual, sourcePath)
					}
					if variant.missing {
						return nil, os.ErrNotExist
					}
					return companion, nil
				})
				if reads != 1 {
					t.Fatal("companion read count", reads)
				}
				if variant.missing || variant.malformed {
					if err == nil {
						t.Fatal("unreadable companion did not fail closed", variant.name, findings)
					}
				} else if err != nil || !strings.Contains(strings.Join(findings, "\n"), "SQL model declaration/reference") {
					t.Fatal("unproved companion field accepted", variant.name, findings, err)
				}
			})
		}
		typeName, receiver := "output", "got"
		if path == "examples/vector/main_test.go" {
			typeName, receiver = "hybridOutput", "result"
		}
		original := "var " + receiver + " " + typeName
		changedSources := []string{
			string(sources[path]) + "\ntype " + typeName + " int\n",
			string(sources[path]) + "\ntype " + typeName + " = int\n",
			strings.Replace(string(sources[path]), original, "type "+typeName+" int; "+original, 1),
			strings.Replace(string(sources[path]), original, "type local = "+typeName+"; var "+receiver+" local", 1),
			strings.Replace(string(sources[path]), original, "var original "+typeName+"; "+receiver+" := original", 1),
		}
		if !strings.Contains(string(sources[path]), original) {
			t.Fatal("actual receiver declaration missing", path, original)
		}
		for _, changed := range changedSources {
			findings, err := inspectRollbackFaultSQLSourceWithReader(path, []byte(changed), func(actual string) ([]byte, error) {
				if actual != sourcePath {
					t.Fatal("unexpected companion source", actual)
				}
				return sources[sourcePath], nil
			})
			if err != nil || !strings.Contains(strings.Join(findings, "\n"), "SQL model declaration/reference") {
				t.Fatal("shadowed type or receiver alias replaced companion binding", path, findings, err)
			}
		}
	}
}
