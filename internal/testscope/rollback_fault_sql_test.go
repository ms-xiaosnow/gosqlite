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
		{"pre-DB helper early return", rollbackFaultSQLModel, "func isSQLPreDBFailure(which SQLCase) bool {\n", "func isSQLPreDBFailure(which SQLCase) bool {\n\treturn true\n", "require closed predicates"},
		{"recovery helper early return", rollbackFaultSQLModel, "func needsSQLRecovery(which SQLCase) bool {\n", "func needsSQLRecovery(which SQLCase) bool {\n\treturn true\n", "require closed predicates"},
		{"pre-DB target identity weakened", rollbackFaultSQLBridge, "target != errRollbackFaultInjected", "target == nil", "closed bounded pre-DB cause traversal"},
		{"pre-DB raw poison nil admitted", rollbackFaultSQLBridge, "commitErr == nil || poison == nil ||", "commitErr == nil || false ||", "closed bounded pre-DB cause traversal"},
		{"pre-DB nil joined cause admitted", rollbackFaultSQLBridge, "if child == nil {\n\t\t\t\treturn false\n\t\t\t}", "if child == nil {\n\t\t\t\tcontinue\n\t\t\t}", "closed bounded pre-DB cause traversal"},
		{"pre-DB empty join accepted", rollbackFaultSQLBridge, "len(children) == 0 || len(children) > len(pending)-count", "len(children) > len(pending)-count", "closed bounded pre-DB cause traversal"},
		{"pre-DB child capacity widened", rollbackFaultSQLBridge, "len(children) > len(pending)-count", "len(children) > len(pending)", "closed bounded pre-DB cause traversal"},
		{"pre-DB target leaf requirement lost", rollbackFaultSQLBridge, "return sawTarget && sawPoisoned", "return sawPoisoned", "closed bounded pre-DB cause traversal"},
		{"pre-DB poisoned leaf requirement lost", rollbackFaultSQLBridge, "return sawTarget && sawPoisoned", "return sawTarget", "closed bounded pre-DB cause traversal"},
		{"pre-DB pending bound widened", rollbackFaultSQLBridge, "var pending [rollbackfault.MaxEvents]error", "var pending [256]error", "closed bounded pre-DB cause traversal"},
		{"pre-DB public checks moved after Unwrap", rollbackFaultSQLBridge, "\tif _, custom := commitErr.(interface{ Is(error) bool }); custom {\n\t\treturn false\n\t}\n\tif _, custom := commitErr.(interface{ As(any) bool }); custom {\n\t\treturn false\n\t}\n\tpublic, ok := commitErr.(interface{ Unwrap() error })\n\tif !ok || public.Unwrap() != poison {\n\t\treturn false\n\t}\n", "\tpublic, ok := commitErr.(interface{ Unwrap() error })\n\tif !ok || public.Unwrap() != poison {\n\t\treturn false\n\t}\n\tif _, custom := commitErr.(interface{ Is(error) bool }); custom {\n\t\treturn false\n\t}\n\tif _, custom := commitErr.(interface{ As(any) bool }); custom {\n\t\treturn false\n\t}\n", "cannot add shapes, effects or change rejection order"},
		{"pre-DB known join check moved after Unwrap", rollbackFaultSQLBridge, "\t\tif reflect.TypeOf(current) != joinType {\n\t\t\treturn false\n\t\t}\n\t\tchildren := current.(interface{ Unwrap() []error }).Unwrap()\n", "\t\tchildren := current.(interface{ Unwrap() []error }).Unwrap()\n\t\tif reflect.TypeOf(current) != joinType {\n\t\t\treturn false\n\t\t}\n", "cannot add shapes, effects or change rejection order"},
		{"pre-DB inner checks moved after Unwrap", rollbackFaultSQLBridge, "\t\tif _, custom := current.(interface{ Is(error) bool }); custom {\n\t\t\treturn false\n\t\t}\n\t\tif _, custom := current.(interface{ As(any) bool }); custom {\n\t\t\treturn false\n\t\t}\n\t\tif current == target {\n\t\t\tsawTarget = true\n\t\t\tcontinue\n\t\t}\n\t\tif current == ErrPoisoned {\n\t\t\tsawPoisoned = true\n\t\t\tcontinue\n\t\t}\n\t\tif permitted != nil && current == permitted {\n\t\t\tsawPermitted = true\n\t\t\tcontinue\n\t\t}\n\t\tif reflect.TypeOf(current) != joinType {\n\t\t\treturn false\n\t\t}\n\t\tchildren := current.(interface{ Unwrap() []error }).Unwrap()\n", "\t\tif current == target {\n\t\t\tsawTarget = true\n\t\t\tcontinue\n\t\t}\n\t\tif current == ErrPoisoned {\n\t\t\tsawPoisoned = true\n\t\t\tcontinue\n\t\t}\n\t\tif permitted != nil && current == permitted {\n\t\t\tsawPermitted = true\n\t\t\tcontinue\n\t\t}\n\t\tif reflect.TypeOf(current) != joinType {\n\t\t\treturn false\n\t\t}\n\t\tchildren := current.(interface{ Unwrap() []error }).Unwrap()\n\t\tif _, custom := current.(interface{ Is(error) bool }); custom {\n\t\t\treturn false\n\t\t}\n\t\tif _, custom := current.(interface{ As(any) bool }); custom {\n\t\t\treturn false\n\t\t}\n", "cannot add shapes, effects or change rejection order"},
		{"pre-DB body target moved", rollbackFaultSQLModel, "spec.Target, spec.Action = spec.Calls[8], FailBefore", "spec.Target, spec.Action = spec.Calls[9], FailBefore", "nine exact fixed plan branches"},
		{"pre-DB body count widened", rollbackFaultSQLModel, "spec.ExpectedCommit, spec.CallCount = CommitFailed, 11", "spec.ExpectedCommit, spec.CallCount = CommitFailed, 12", "nine exact fixed plan branches"},
		{"pre-DB body recovery classification", rollbackFaultSQLModel, "spec.ExpectedCommit, spec.CallCount = CommitFailed, 11", "spec.ExpectedCommit, spec.CallCount = CommitRecoveryRequired, 11", "nine exact fixed plan branches"},
		{"pre-DB body bridge relabeled", rollbackFaultSQLBridge, "caseID = rollbackfault.SQLJournalBodySyncBefore", "caseID = rollbackfault.SQLDBSyncBefore", "bridge frozen nine-case plan"},
		{"pre-DB body admission relabeled", rollbackFaultSQLNative, "c.caseID = rollbackfault.SQLJournalBodySyncBefore", "c.caseID = rollbackfault.SQLDBSyncBefore", "nine exact names and case mappings"},
		{"pre-DB header target moved", rollbackFaultSQLModel, "spec.Target, spec.Action = spec.Calls[10], FailBefore", "spec.Target, spec.Action = spec.Calls[11], FailBefore", "nine exact fixed plan branches"},
		{"pre-DB header count widened", rollbackFaultSQLModel, "spec.ExpectedCommit, spec.CallCount = CommitFailed, 13", "spec.ExpectedCommit, spec.CallCount = CommitFailed, 14", "nine exact fixed plan branches"},
		{"pre-DB header recovery classification", rollbackFaultSQLModel, "spec.ExpectedCommit, spec.CallCount = CommitFailed, 13", "spec.ExpectedCommit, spec.CallCount = CommitRecoveryRequired, 13", "nine exact fixed plan branches"},
		{"pre-DB header bridge relabeled", rollbackFaultSQLBridge, "caseID = rollbackfault.SQLJournalHeaderSyncBefore", "caseID = rollbackfault.SQLDBSyncBefore", "bridge frozen nine-case plan"},
		{"pre-DB header admission relabeled", rollbackFaultSQLNative, "c.caseID = rollbackfault.SQLJournalHeaderSyncBefore", "c.caseID = rollbackfault.SQLDBSyncBefore", "nine exact names and case mappings"},
		{"pre-DB publish target moved", rollbackFaultSQLModel, "spec.Target, spec.Action = spec.Calls[11], FailBefore", "spec.Target, spec.Action = spec.Calls[12], FailBefore", "nine exact fixed plan branches"},
		{"pre-DB publish count widened", rollbackFaultSQLModel, "spec.ExpectedCommit, spec.CallCount = CommitFailed, 14", "spec.ExpectedCommit, spec.CallCount = CommitFailed, 15", "nine exact fixed plan branches"},
		{"pre-DB publish recovery classification", rollbackFaultSQLModel, "spec.ExpectedCommit, spec.CallCount = CommitFailed, 14", "spec.ExpectedCommit, spec.CallCount = CommitRecoveryRequired, 14", "nine exact fixed plan branches"},
		{"pre-DB publish bridge relabeled", rollbackFaultSQLBridge, "caseID = rollbackfault.SQLPublishDirectorySyncBefore", "caseID = rollbackfault.SQLDBSyncBefore", "bridge frozen nine-case plan"},
		{"pre-DB publish admission relabeled", rollbackFaultSQLNative, "c.caseID = rollbackfault.SQLPublishDirectorySyncBefore", "c.caseID = rollbackfault.SQLDBSyncBefore", "nine exact names and case mappings"},
		{"pre-DB IDs relabeled", rollbackFaultSQLModel, "\tSQLJournalBodySyncBefore\n", "\tSQLJournalBodySyncBefore SQLCase = 8\n", "exact fixed SQL case IDs"},
		{"pre-DB IDs reordered", rollbackFaultSQLModel, "\tSQLJournalBodySyncBefore\n\tSQLJournalHeaderSyncBefore\n", "\tSQLJournalHeaderSyncBefore\n\tSQLJournalBodySyncBefore\n", "exact fixed SQL case IDs"},
		{"pre-DB cleanup unlink skipped", rollbackFaultSQLModel, "spec.Calls[spec.CallCount-2] = Call{Point: CleanupUnlink", "spec.Calls[spec.CallCount-2] = Call{Point: JournalUnlink", "pre-DB fixed target geometry and normal cleanup tail"},
		{"pre-DB cleanup sync skipped", rollbackFaultSQLModel, "spec.Calls[spec.CallCount-1] = Call{Point: CleanupDirectorySync", "spec.Calls[spec.CallCount-1] = Call{Point: FinalDirectorySync", "pre-DB fixed target geometry and normal cleanup tail"},
		{"pre-DB cleanup owner changed", rollbackFaultSQLModel, "Call{Point: CleanupDirectorySync, Occurrence: 1, Role: Directory, Owner: 13}", "Call{Point: CleanupDirectorySync, Occurrence: 1, Role: Directory, Owner: 14}", "pre-DB fixed target geometry and normal cleanup tail"},
		{"pre-DB case predicate widened", rollbackFaultSQLModel, "return which == SQLJournalBodySyncBefore ||", "return which == SQLCaseUnknown || which == SQLJournalBodySyncBefore ||", "exact three pre-DB cases"},
		{"pre-DB recovery predicate widened", rollbackFaultSQLModel, "return which == SQLIndexPrefix64 ||", "return which == SQLJournalBodySyncBefore || which == SQLIndexPrefix64 ||", "exact four post-write recovery cases"},
		{"pre-DB missing poison classification", rollbackFaultSQLModel, "lifecycle.PublicCodeIOErr, lifecycle.PublicPoisoned = true, true", "lifecycle.PublicCodeIOErr, lifecycle.PublicPoisoned = true, false", "one-shot public error lifecycle"},
		{"pre-DB claims recovery sentinel", rollbackFaultSQLModel, "lifecycle.PublicRecoveryRequired = needsSQLRecovery(evidence.Case)", "lifecycle.PublicRecoveryRequired = evidence.Case != SQLControl", "one-shot public error lifecycle"},
		{"pre-DB missing cause evidence", rollbackFaultSQLModel, "lifecycle.PreDBOnlyTargetCause = isSQLPreDBFailure(evidence.Case)", "lifecycle.PreDBOnlyTargetCause = false", "one-shot public error lifecycle"},
		{"pre-DB uncertain accepted", rollbackFaultSQLModel, "lifecycle.PublicCommitUncertain = evidence.Case == SQLFinalDirectorySyncBefore", "lifecycle.PublicCommitUncertain = isSQLPreDBFailure(evidence.Case) || evidence.Case == SQLFinalDirectorySyncBefore", "one-shot public error lifecycle"},
		{"pre-DB cleanup pending accepted", rollbackFaultSQLModel, "CommitCapturedUnchanged: true}", "CommitCapturedUnchanged: true, PublicCleanupPending: true}", "one-shot public error lifecycle"},
		{"pre-DB lifecycle field omitted", rollbackFaultSQLModel, "\tPublicCleanupPending         bool\n", "", "closed public lifecycle evidence fields"},
		{"pre-DB recovery zero branch lost", rollbackFaultSQLModel, "if which == SQLControl || isSQLPreDBFailure(which) || which == SQLFinalDirectorySyncBefore {", "if which == SQLControl {", "control/pre-DB no recovery; post-write exact one bounded replay"},
		{"unknown recovery case admitted", rollbackFaultSQLModel, "!needsSQLRecovery(which) || recovery.Calls != 1", "recovery.Calls != 1", "control/pre-DB no recovery; post-write exact one bounded replay"},
		{"pre-DB journal presence admitted", rollbackFaultSQLModel, "journal && (which == SQLControl || isSQLPreDBFailure(which) || which == SQLFinalDirectorySyncBefore)", "journal && which == SQLControl", "pre-DB journal absence and whole OLD copies"},
		{"pre-DB whole OLD input weakened", rollbackFaultSQLModel, "return baseline == trial && replayAfter == baseline", "return replayAfter == baseline", "pre-DB journal absence and whole OLD copies"},
		{"pre-DB copied result changed", rollbackFaultSQLModel, "return baseline == trial && replayAfter == baseline", "return baseline == trial", "pre-DB journal absence and whole OLD copies"},
		{"pre-DB cause target fabricated", rollbackFaultSQLBridge, "target, point := scope.scope.targetError, scope.scope.plan.Spec().Target.Point", "target, point := error(nil), scope.scope.plan.Spec().Target.Point", "pre-DB cause bound to actual frozen target and Tx poison"},
		{"pre-DB poison substituted", rollbackFaultSQLBridge, "poison := scope.scope.tx.poisoned", "poison := commitErr", "pre-DB cause bound to actual frozen target and Tx poison"},
		{"pre-DB public unwrap identity dropped", rollbackFaultSQLBridge, "public.Unwrap() != poison", "false", "closed bounded pre-DB cause traversal"},
		{"pre-DB custom Is accepted", rollbackFaultSQLBridge, "if _, custom := commitErr.(interface{ Is(error) bool }); custom {", "if _, custom := commitErr.(interface{ Is(error) bool }); !custom {", "closed bounded pre-DB cause traversal"},
		{"pre-DB custom As accepted", rollbackFaultSQLBridge, "if _, custom := current.(interface{ As(any) bool }); custom {", "if _, custom := current.(interface{ As(any) bool }); !custom {", "closed bounded pre-DB cause traversal"},
		{"pre-DB unknown cause shape accepted", rollbackFaultSQLBridge, "reflect.TypeOf(current) != joinType", "false", "closed bounded pre-DB cause traversal"},
		{"pre-DB traversal unbounded", rollbackFaultSQLBridge, "visited == len(pending)", "visited > len(pending)", "closed bounded pre-DB cause traversal"},
		{"pre-DB extra leaf admitted", rollbackFaultSQLBridge, "if current == target {", "if current != nil {", "closed bounded pre-DB cause traversal"},
		{"pre-DB sentinel evidence disjoined", rollbackFaultSQLBridge, "return sawTarget && sawPoisoned", "return sawTarget || sawPoisoned", "closed bounded pre-DB cause traversal"},
		{"pre-DB custom error reflection retained", rollbackFaultSQLBridge, "joinType := reflect.TypeOf(errors.Join(ErrPoisoned, target))", "_ = reflect.ValueOf; joinType := reflect.TypeOf(errors.Join(ErrPoisoned, target))", "pre-DB reflection/error access outside exact bounded cause observer"},
		{"pre-DB helper function value", rollbackFaultSQLBridge, "preDBOnlyTargetCause = manualRollbackSQLOnlyTargetCause(commitErr, poison, target, nil)", "inspect := manualRollbackSQLOnlyTargetCause; preDBOnlyTargetCause = inspect(commitErr, poison, target, nil)", "pre-DB cause observer cannot be copied, aliased"},
		{"pre-DB public wrapper replaced", rollbackFaultSQLNative, "commitErr != publicError || !report.PreDBOnlyTargetCause", "!report.PreDBOnlyTargetCause", "public error identity and pre-DB exact cause"},
		{"pre-DB cause boolean ignored", rollbackFaultSQLNative, "commitErr != publicError || !report.PreDBOnlyTargetCause", "commitErr != publicError", "public error identity and pre-DB exact cause"},
		{"pre-DB recovery sentinel allowed", rollbackFaultSQLNative, "!report.PreDBOnlyTargetCause || errors.Is(commitErr, ErrRecoveryRequired)", "!report.PreDBOnlyTargetCause || false", "public error identity and pre-DB exact cause"},
		{"pre-DB public poisoned ignored", rollbackFaultSQLNative, "!errors.Is(commitErr, pager.ErrPoisoned)", "false", "public error identity"},
		{"pre-DB validates only page prefix", rollbackFaultSQLNative, "preDB && !bytes.Equal(post, baseline[:])", "preDB && !bytes.Equal(post[:64], baseline[:64])", "pre-DB whole OLD bytes and no journal or recovery"},
		{"pre-DB journal absence bypassed", rollbackFaultSQLNative, "if !needsRecovery {", "if c.caseID == rollbackfault.SQLControl {", "pre-DB whole OLD bytes and no journal or recovery"},
		{"pre-DB journal copy admitted", rollbackFaultSQLNative, "if needsRecovery {\n\t\tif _, err := c.create(\"replay.db-journal\"", "if c.caseID != rollbackfault.SQLControl {\n\t\tif _, err := c.create(\"replay.db-journal\"", "pre-DB whole OLD bytes and no journal or recovery"},
		{"pre-DB executes recovery", rollbackFaultSQLNative, "if needsRecovery {\n\t\tresult, recoveryErr :=", "if c.caseID != rollbackfault.SQLControl {\n\t\tresult, recoveryErr :=", "control and pre-DB may never execute recovery"},
		{"pre-DB selector later reassigned", rollbackFaultSQLNative, "baseline, err := manualRollbackSQLBaseline()", "needsRecovery = true; baseline, err := manualRollbackSQLBaseline()", "fixed pre-DB/recovery selectors cannot be shadowed or reassigned"},
		{"pre-DB selector shadowed", rollbackFaultSQLNative, "baseline, err := manualRollbackSQLBaseline()", "var needsRecovery bool; baseline, err := manualRollbackSQLBaseline()", "fixed pre-DB/recovery selectors cannot be shadowed or reassigned"},
		{"pre-DB actual poison observation faked", rollbackFaultSQLNative, "evidence.Lifecycle.PublicPoisoned = errors.Is(report.CommitError, pager.ErrPoisoned)", "evidence.Lifecycle.PublicPoisoned = true", "actual public lifecycle observations"},
		{"pre-DB actual cleanup observation faked", rollbackFaultSQLNative, "evidence.Lifecycle.PublicCleanupPending = errors.Is(report.CommitError, pager.ErrPlaintextCleanupPending)", "evidence.Lifecycle.PublicCleanupPending = false", "actual public lifecycle observations"},
		{"pre-DB actual cause observation faked", rollbackFaultSQLNative, "evidence.Lifecycle.PreDBOnlyTargetCause = report.PreDBOnlyTargetCause", "evidence.Lifecycle.PreDBOnlyTargetCause = preDB", "actual public lifecycle observations"},
		{"bridge error case mapped to nil", rollbackFaultSQLBridge, "caseID = rollbackfault.SQLIndexPrefix64Error", "caseID = rollbackfault.SQLIndexPrefix64", "bridge frozen nine-case plan"},
		{"bridge prefix bit flipped", rollbackFaultSQLBridge, "if spec.PrefixError {", "if !spec.PrefixError {", "bridge frozen nine-case plan"},
		{"new prefix bit dropped", rollbackFaultSQLModel, "PrefixShortWrite, 64, true", "PrefixShortWrite, 64, false", "frozen three-page nine-case selectors"},
		{"new prefix length changed", rollbackFaultSQLModel, "PrefixShortWrite, 64, true", "PrefixShortWrite, 63, true", "frozen three-page nine-case selectors"},
		{"new plan name aliased", rollbackFaultSQLModel, "case SQLIndexPrefix64Error:", "case SQLCase(5):", "nine exact fixed plan branches"},
		{"new admission mapped to nil", rollbackFaultSQLNative, "c.caseID = rollbackfault.SQLIndexPrefix64Error", "c.caseID = rollbackfault.SQLIndexPrefix64", "nine exact names and case mappings"},
		{"new admission name widened", rollbackFaultSQLNative, `case "index-prefix-64-error":`, `case "index-prefix-64-error", "another-prefix-error":`, "nine exact names and case mappings"},
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
		{"private entry alias", rollbackFaultSQLBridge, "scope, err := armRollbackFault(tx, plan)", "arm := armRollbackFault; scope, err := arm(tx, plan)", "bridge frozen nine-case plan"},
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
		{"plan wrong short prefix", rollbackFaultSQLModel, "PrefixShortWrite, 64, false", "PrefixShortWrite, 65, false", "frozen three-page nine-case selectors"},
		{"plan wrong DBPage occurrence", rollbackFaultSQLModel, "Point: DBPage, Occurrence: 3", "Point: DBPage, Occurrence: 2", "frozen three-page nine-case selectors"},
		{"plan wrong third-page offset", rollbackFaultSQLModel, "Point: DBPage, Occurrence: 3, Role: Database, Owner: 11, Offset: 8192", "Point: DBPage, Occurrence: 3, Role: Database, Owner: 11, Offset: 4096", "frozen three-page nine-case selectors"},
		{"model count-only rows", rollbackFaultSQLModel, "query.RowCount != count || query.Rows != rows", "query.RowCount != count", "full rows, Check, EOF"},
		{"model receipt trusted", rollbackFaultSQLModel, "result.Adapter = receipt.Seal()", "result.Adapter = Observation{Outcome: AdapterObserved}", "receipt identity and old/new"},
		{"model old-new mixture", rollbackFaultSQLModel, "state := SQLOld", "state := SQLNew", "receipt identity and old/new"},
		{"model index zero-work accepted", rollbackFaultSQLModel, "work.IndexPageReads > 0", "work.IndexPageReads >= 0", "index claim requires actual bounded work"},
		{"model EOF ignored", rollbackFaultSQLModel, "!query.EOF || !query.ErrCalled || ", "", "full rows, Check, EOF"},
		{"model digest substitution", rollbackFaultSQLModel, "original.Frozen != original.After", "original.Before != original.After", "receipt identity and old/new"},
		{"control recovery call admitted", rollbackFaultSQLModel, "SQLRecoveryEvidence{Identity: recovery.Identity, OwnersRetired: true}", "SQLRecoveryEvidence{Identity: recovery.Identity, OwnersRetired: true, Calls: 1}", "control/pre-DB no recovery; post-write exact one bounded replay"},
		{"model physical pass", rollbackFaultSQLModel, "func (SQLObservation) Pass() bool              { return false }", "func (SQLObservation) Pass() bool { return true }", "cannot confer physical"},
		{"model owner pointer", rollbackFaultSQLModel, "type SQLRow struct {", "type SQLRow struct { owner *SQLRow", "descriptor-free fixed scalar"},
		{"model aliases replay", rollbackFaultSQLModel, "before.Device == other.Device && before.Inode == other.Inode", "before.Device == other.Device && false", "immutable originals and unaliased"},
		{"post-DB full ledger field omitted", rollbackFaultSQLModel, "\tPostCommit    SQLSnapshotEvidence\n", "", "closed SQL ledger includes exactly one"},
		{"post-DB extra readback site", rollbackFaultSQLNative, "postCommit, readErr := manualRollbackSQLReadback", "_, _ = manualRollbackSQLReadback(&c, ctx, filepath.Join(c.root, \"post-commit.input.db\"), identity); postCommit, readErr := manualRollbackSQLReadback", "exact three native readback sites"},
		{"post-DB case IDs reordered", rollbackFaultSQLModel, "\tSQLJournalUnlinkBefore\n\tSQLFinalDirectorySyncBefore\n", "\tSQLFinalDirectorySyncBefore\n\tSQLJournalUnlinkBefore\n", "exact fixed SQL case IDs"},
		{"post-DB future case appended", rollbackFaultSQLModel, "\tSQLFinalDirectorySyncBefore\n", "\tSQLFinalDirectorySyncBefore\n\tSQLFutureCase\n", "exact fixed SQL case IDs"},
		{"unlink target moved before DB sync", rollbackFaultSQLModel, "spec.Target, spec.Action = spec.Calls[16], FailBefore", "spec.Target, spec.Action = spec.Calls[15], FailBefore", "nine exact fixed plan branches"},
		{"unlink case executes final sync", rollbackFaultSQLModel, "spec.ExpectedCommit, spec.CallCount = CommitRecoveryRequired, 17", "spec.ExpectedCommit, spec.CallCount = CommitRecoveryRequired, 18", "nine exact fixed plan branches"},
		{"unlink incorrectly acknowledged", rollbackFaultSQLModel, "spec.ExpectedCommit, spec.CallCount = CommitRecoveryRequired, 17", "spec.ExpectedCommit, spec.CallCount = CommitSucceeded, 17", "nine exact fixed plan branches"},
		{"final sync target becomes unlink", rollbackFaultSQLModel, "spec.Target, spec.Action = spec.Calls[17], FailBefore", "spec.Target, spec.Action = spec.Calls[16], FailBefore", "nine exact fixed plan branches"},
		{"final sync trace truncated", rollbackFaultSQLModel, "spec.ExpectedCommit, spec.CallCount = CommitUncertain, 18", "spec.ExpectedCommit, spec.CallCount = CommitUncertain, 17", "nine exact fixed plan branches"},
		{"final sync incorrectly recovery-required", rollbackFaultSQLModel, "spec.ExpectedCommit, spec.CallCount = CommitUncertain, 18", "spec.ExpectedCommit, spec.CallCount = CommitRecoveryRequired, 18", "nine exact fixed plan branches"},
		{"post-DB unlink occurrence changed", rollbackFaultSQLModel, "{Point: JournalUnlink, Occurrence: 1, Role: Journal, Owner: 12}", "{Point: JournalUnlink, Occurrence: 2, Role: Journal, Owner: 12}", "post-DB exact fail-before target geometry"},
		{"post-DB final sync role changed", rollbackFaultSQLModel, "{Point: FinalDirectorySync, Occurrence: 1, Role: Directory, Owner: 13}", "{Point: FinalDirectorySync, Occurrence: 1, Role: Journal, Owner: 12}", "post-DB exact fail-before target geometry"},
		{"post-DB future predicate admitted", rollbackFaultSQLModel, "return which == SQLJournalUnlinkBefore || which == SQLFinalDirectorySyncBefore", "return which >= SQLJournalUnlinkBefore", "require closed predicates"},
		{"post-DB predicate early return", rollbackFaultSQLModel, "func isSQLPostDBFailure(which SQLCase) bool {\n", "func isSQLPostDBFailure(which SQLCase) bool {\n\treturn true\n", "require closed predicates"},
		{"future recovery classifier admitted", rollbackFaultSQLModel, "return which == SQLIndexPrefix64 || which == SQLDBSyncBefore || which == SQLIndexPrefix64Error || which == SQLJournalUnlinkBefore", "return which != SQLControl && !isSQLPreDBFailure(which)", "require closed predicates"},
		{"unlink recovery omitted", rollbackFaultSQLModel, " || which == SQLJournalUnlinkBefore\n", "\n", "exact four post-write recovery cases"},
		{"post-DB target reflection skipped", rollbackFaultSQLModel, "result.Adapter = receipt.Seal()", "result.Adapter = Observation{Outcome: AdapterObserved, TargetHits: 1}", "receipt identity and old/new"},
		{"post-DB receipt plan identity lost", rollbackFaultSQLModel, "!receipt.plan.valid || receipt.plan.spec != plan.spec", "!receipt.plan.valid", "receipt identity and old/new"},
		{"post-DB public chain scalar omitted", rollbackFaultSQLModel, "lifecycle.PostDBOnlyTargetCause = isSQLPostDBFailure(evidence.Case)", "lifecycle.PostDBOnlyTargetCause = false", "one-shot public error lifecycle"},
		{"final sync uncertainty scalar lost", rollbackFaultSQLModel, "lifecycle.PublicCommitUncertain = evidence.Case == SQLFinalDirectorySyncBefore", "lifecycle.PublicCommitUncertain = false", "one-shot public error lifecycle"},
		{"unlink uncertainty falsely accepted", rollbackFaultSQLModel, "lifecycle.PublicCommitUncertain = evidence.Case == SQLFinalDirectorySyncBefore", "lifecycle.PublicCommitUncertain = isSQLPostDBFailure(evidence.Case)", "one-shot public error lifecycle"},
		{"post-DB second reader omitted", rollbackFaultSQLModel, "lifecycle.ReopenCalls = 2", "lifecycle.ReopenCalls = 1", "post-DB full NEW snapshot"},
		{"post-DB NEW snapshot omitted", rollbackFaultSQLModel, "validateSQLSnapshot(evidence.PostCommit, evidence.Identity, SQLNew)", "SQLProblemNone", "post-DB full NEW snapshot"},
		{"post-DB accepts OLD intermediate ledger", rollbackFaultSQLModel, "validateSQLSnapshot(evidence.PostCommit, evidence.Identity, SQLNew)", "validateSQLSnapshot(evidence.PostCommit, evidence.Identity, SQLOld)", "post-DB full NEW snapshot"},
		{"post-DB prior reader owner reused", rollbackFaultSQLModel, " || !validSQLReaderOwners(plan.spec, evidence.Before.Owner, evidence.PostCommit.Owner)", "", "post-DB full NEW snapshot"},
		{"post-DB final reader owner reused", rollbackFaultSQLModel, " || !validSQLReaderOwners(plan.spec, evidence.After.Owner, evidence.PostCommit.Owner)", "", "post-DB full NEW snapshot"},
		{"post-DB final iterator reused", rollbackFaultSQLModel, " || post.Iterator == evidence.After.Queries[i].Iterator", "", "post-DB full NEW snapshot"},
		{"old seven nonzero PostCommit admitted", rollbackFaultSQLModel, "} else if evidence.PostCommit != (SQLSnapshotEvidence{}) {", "} else if false {", "post-DB full NEW snapshot"},
		{"final sync OLD ledger wrongly accepted", rollbackFaultSQLModel, "if evidence.Case == SQLControl || evidence.Case == SQLFinalDirectorySyncBefore {", "if evidence.Case == SQLControl {", "receipt identity and old/new"},
		{"final sync recovery allowed", rollbackFaultSQLModel, "if which == SQLControl || isSQLPreDBFailure(which) || which == SQLFinalDirectorySyncBefore {", "if which == SQLControl || isSQLPreDBFailure(which) {", "exact no-replay/replay decisions"},
		{"future recovery accepted by early return", rollbackFaultSQLModel, "func validSQLRecovery(which SQLCase, recovery SQLRecoveryEvidence) bool {\n", "func validSQLRecovery(which SQLCase, recovery SQLRecoveryEvidence) bool {\n\treturn true\n", "exact no-replay/replay decisions"},
		{"final sync journal artifacts admitted", rollbackFaultSQLModel, "journal && (which == SQLControl || isSQLPreDBFailure(which) || which == SQLFinalDirectorySyncBefore)", "journal && (which == SQLControl || isSQLPreDBFailure(which))", "exact post-DB artifact decisions"},
		{"final sync artifact output changed", rollbackFaultSQLModel, "if which == SQLControl || which == SQLFinalDirectorySyncBefore {\n\t\treturn replayAfter == trial", "if which == SQLControl || which == SQLFinalDirectorySyncBefore {\n\t\treturn true", "exact post-DB artifact decisions"},
		{"future artifact accepted by early return", rollbackFaultSQLModel, "func validSQLArtifacts(which SQLCase, artifacts [SQLArtifactCount]SQLArtifactEvidence) bool {\n", "func validSQLArtifacts(which SQLCase, artifacts [SQLArtifactCount]SQLArtifactEvidence) bool {\n\treturn true\n", "exact post-DB artifact decisions"},
		{"unlink bridge target mislabeled", rollbackFaultSQLBridge, "caseID = rollbackfault.SQLJournalUnlinkBefore", "caseID = rollbackfault.SQLFinalDirectorySyncBefore", "bridge frozen nine-case plan"},
		{"final sync bridge target mislabeled", rollbackFaultSQLBridge, "caseID = rollbackfault.SQLFinalDirectorySyncBefore", "caseID = rollbackfault.SQLJournalUnlinkBefore", "bridge frozen nine-case plan"},
		{"post-DB bridge sentinel relabeled", rollbackFaultSQLBridge, "permitted := ErrRecoveryRequired", "permitted := ErrCommitUncertain", "post-DB actual target poison"},
		{"final sync bridge sentinel lost", rollbackFaultSQLBridge, "permitted = ErrCommitUncertain", "permitted = ErrRecoveryRequired", "post-DB actual target poison"},
		{"post-DB third sentinel admitted", rollbackFaultSQLBridge, "permitted != nil && permitted != ErrRecoveryRequired && permitted != ErrCommitUncertain", "permitted != nil && permitted != ErrRecoveryRequired && permitted != ErrCommitUncertain && permitted != ErrPlaintextCleanupPending", "closed bounded pre-DB cause traversal"},
		{"post-DB permitted sentinel optional", rollbackFaultSQLBridge, "sawPermitted := permitted == nil", "sawPermitted := true", "closed bounded pre-DB cause traversal"},
		{"post-DB permitted leaf omitted", rollbackFaultSQLBridge, "return sawTarget && sawPoisoned && sawPermitted", "return sawTarget && sawPoisoned", "closed bounded pre-DB cause traversal"},
		{"post-DB either classification leaf accepted", rollbackFaultSQLBridge, "permitted != nil && current == permitted", "current == ErrRecoveryRequired || current == ErrCommitUncertain", "closed bounded pre-DB cause traversal"},
		{"post-DB bridge uses public error as poison", rollbackFaultSQLBridge, "postDBOnlyTargetCause = manualRollbackSQLOnlyTargetCause(commitErr, poison, target, permitted)", "postDBOnlyTargetCause = manualRollbackSQLOnlyTargetCause(commitErr, commitErr, target, permitted)", "post-DB actual target poison"},
		{"post-DB bridge result fabricated", rollbackFaultSQLBridge, "PostDBOnlyTargetCause: postDBOnlyTargetCause", "PostDBOnlyTargetCause: true", "bridge exact untouched public error forwarding"},
		{"post-DB helper copied by function value", rollbackFaultSQLBridge, "postDBOnlyTargetCause = manualRollbackSQLOnlyTargetCause(commitErr, poison, target, permitted)", "inspect := manualRollbackSQLOnlyTargetCause; postDBOnlyTargetCause = inspect(commitErr, poison, target, permitted)", "pre-DB cause observer cannot be copied"},
		{"unlink admission mislabeled", rollbackFaultSQLNative, "c.caseID = rollbackfault.SQLJournalUnlinkBefore", "c.caseID = rollbackfault.SQLFinalDirectorySyncBefore", "nine exact names and case mappings"},
		{"final sync admission name broadened", rollbackFaultSQLNative, "case \"final-dirsync-before\":", "case \"final-dirsync-before\", \"future-sync\":", "nine exact names and case mappings"},
		{"final sync uncertainty forced false", rollbackFaultSQLNative, "uncertain := c.caseID == rollbackfault.SQLFinalDirectorySyncBefore", "uncertain := false", "post-DB public Commit exact classification"},
		{"post-DB public wrapper identity removed", rollbackFaultSQLNative, "commitErr != publicError || !report.PostDBOnlyTargetCause", "!report.PostDBOnlyTargetCause", "post-DB public Commit exact classification"},
		{"post-DB cause observation ignored", rollbackFaultSQLNative, "commitErr != publicError || !report.PostDBOnlyTargetCause", "commitErr != publicError", "post-DB public Commit exact classification"},
		{"post-DB recovery versus uncertainty inverted", rollbackFaultSQLNative, "errors.Is(commitErr, ErrRecoveryRequired) != !uncertain", "errors.Is(commitErr, ErrRecoveryRequired) != uncertain", "post-DB public Commit exact classification"},
		{"post-DB classification flag fabricated", rollbackFaultSQLNative, "evidence.Lifecycle.PostDBOnlyTargetCause = report.PostDBOnlyTargetCause", "evidence.Lifecycle.PostDBOnlyTargetCause = postDB", "actual public lifecycle observations"},
		{"final sync captured uncertainty fabricated", rollbackFaultSQLNative, "evidence.Lifecycle.PublicCommitUncertain = errors.Is(report.CommitError, pager.ErrCommitUncertain)", "evidence.Lifecycle.PublicCommitUncertain = false", "actual public lifecycle observations"},
		{"post-DB captured report overwritten later", rollbackFaultSQLNative, "post, postInfo, err := manualRollbackSQLRead", "evidence.Lifecycle.PostDBOnlyTargetCause = true; post, postInfo, err := manualRollbackSQLRead", "post-DB root exact one-shot"},
		{"post-DB NEW snapshot condition widened", rollbackFaultSQLNative, "if postDB {\n\t\t// Read the retained", "if c.caseID != rollbackfault.SQLControl {\n\t\t// Read the retained", "post-DB retained NEW snapshot"},
		{"post-DB intermediate read opens original", rollbackFaultSQLNative, "filepath.Join(c.root, \"post-commit.input.db\"), identity", "filepath.Join(c.root, \"trial.db\"), identity", "post-DB retained NEW snapshot"},
		{"post-DB intermediate NEW validated as OLD", rollbackFaultSQLNative, "ValidateSQLSnapshot(postCommit, identity, rollbackfault.SQLNew)", "ValidateSQLSnapshot(postCommit, identity, rollbackfault.SQLOld)", "post-DB retained NEW snapshot"},
		{"post-DB intermediate result fabricated", rollbackFaultSQLNative, "evidence.PostCommit = postCommit", "evidence.PostCommit = before", "post-DB retained NEW snapshot"},
		{"post-DB intermediate retirement ignored", rollbackFaultSQLNative, "if !c.ledger.retired() || !manualRollbackSQLRegistryEmpty() {\n\t\t\tt.Fatal(\"INCOMPLETE: post-DB", "if !manualRollbackSQLRegistryEmpty() {\n\t\t\tt.Fatal(\"INCOMPLETE: post-DB", "post-DB retained NEW snapshot"},
		{"post-DB selector later reassigned", rollbackFaultSQLNative, "baseline, err := manualRollbackSQLBaseline()", "postDB = false; baseline, err := manualRollbackSQLBaseline()", "fixed pre-DB/recovery selectors cannot be shadowed or reassigned"},
		{"post-DB future selector admitted", rollbackFaultSQLNative, "postDB := c.caseID == rollbackfault.SQLJournalUnlinkBefore || c.caseID == rollbackfault.SQLFinalDirectorySyncBefore", "postDB := c.caseID >= rollbackfault.SQLJournalUnlinkBefore", "fixed pre-DB/recovery selectors cannot be shadowed or reassigned"},
		{"final sync executes recovery", rollbackFaultSQLNative, "if needsRecovery {\n\t\tresult, recoveryErr :=", "if needsRecovery || c.caseID == rollbackfault.SQLFinalDirectorySyncBefore {\n\t\tresult, recoveryErr :=", "control and pre-DB may never execute recovery"},
		{"unlink skips recovery", rollbackFaultSQLNative, "if needsRecovery {\n\t\tresult, recoveryErr :=", "if needsRecovery && c.caseID != rollbackfault.SQLJournalUnlinkBefore {\n\t\tresult, recoveryErr :=", "control and pre-DB may never execute recovery"},
		{"final sync journal absence ignored", rollbackFaultSQLNative, "if !needsRecovery {", "if !needsRecovery && c.caseID != rollbackfault.SQLFinalDirectorySyncBefore {", "pre-DB whole OLD bytes and no journal or recovery"},
		{"final sync final ledger changed to OLD", rollbackFaultSQLNative, "if c.caseID == rollbackfault.SQLControl || c.caseID == rollbackfault.SQLFinalDirectorySyncBefore {", "if c.caseID == rollbackfault.SQLControl {", "root independent baseline and copy-only recovery order"},
		{"post-DB retirement failure accepted", rollbackFaultSQLNative, "return report, fmt.Errorf(\"Commit owner retirement incomplete\")", "return report, nil", "post-DB public Commit exact classification"},
		{"post-DB physical validation no longer rejected", rollbackFaultSQLNative, " || observation.PhysicalValidated() || observation.Pass()", " || observation.Pass()", "root independent baseline and copy-only recovery order"},
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
