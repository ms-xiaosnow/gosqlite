package rollbackfault

import "testing"

// SOURCE ONLY. These finite synthetic values do not create fixtures, descriptors,
// native owners, SQL executions, Commit attempts, or recovery operations. Even
// successful tests would establish only this evidence contract's consistency.

func sqlLiteralRows(state SQLState) [3]SQLRow {
	if state == SQLNew {
		return [3]SQLRow{
			{IDType: SQLInteger, ID: 1, KeyType: SQLText, Key: "green", ValueType: SQLText, Value: "ONE"},
			{IDType: SQLInteger, ID: 3, KeyType: SQLText, Key: "red", ValueType: SQLText, Value: "three"},
			{IDType: SQLInteger, ID: 4, KeyType: SQLText, Key: "blue", ValueType: SQLText, Value: "four"},
		}
	}
	return [3]SQLRow{
		{IDType: SQLInteger, ID: 1, KeyType: SQLText, Key: "red", ValueType: SQLText, Value: "one"},
		{IDType: SQLInteger, ID: 2, KeyType: SQLText, Key: "blue", ValueType: SQLText, Value: "two"},
		{IDType: SQLInteger, ID: 3, KeyType: SQLText, Key: "red", ValueType: SQLText, Value: "three"},
	}
}

func sqlLiteralQueryText() [SQLQueryCount]string {
	return [SQLQueryCount]string{
		"SELECT id,k,v FROM t NOT INDEXED ORDER BY id",
		"SELECT id,k,v FROM t WHERE k='blue' ORDER BY id",
		"SELECT id,k,v FROM t NOT INDEXED WHERE k='blue' ORDER BY id",
		"SELECT id,k,v FROM t WHERE k='red' ORDER BY id",
		"SELECT id,k,v FROM t NOT INDEXED WHERE k='red' ORDER BY id",
		"SELECT id,k,v FROM t WHERE k='green' ORDER BY id",
		"SELECT id,k,v FROM t NOT INDEXED WHERE k='green' ORDER BY id",
	}
}

func sqlSnapshot(identity Identity, owner, firstIterator uint64, state SQLState) SQLSnapshotEvidence {
	snapshot := SQLSnapshotEvidence{
		Identity: identity, Owner: owner, CloseCalls: 1, OwnerState: OwnerRetired,
		Check: SQLCheckEvidence{Identity: identity, Owner: owner, Calls: 1, Pages: 3, Tables: 1, Indexes: 1, TableRows: 3, IndexEntries: 3},
	}
	all := sqlLiteralRows(state)
	texts := sqlLiteralQueryText()
	keys := [SQLQueryCount]string{"", "blue", "blue", "red", "red", "green", "green"}
	for i := 0; i < SQLQueryCount; i++ {
		query := SQLQueryEvidence{
			Identity: identity, Query: SQLQuery(i), Owner: owner, Iterator: firstIterator + uint64(i), SQL: texts[i],
			EOF: true, ErrCalled: true, CloseCalls: 1,
			Plan: SQLQueryPlan{Access: "SCAN", Table: "t", Reason: "NOT INDEXED requested"},
			Work: SQLQueryWork{BaseRowFetches: 3},
		}
		for _, row := range all {
			if i == 0 || row.Key == keys[i] {
				query.Rows[query.RowCount] = row
				query.RowCount++
			}
		}
		query.Work.ReturnedRows = int64(query.RowCount)
		if i == 1 || i == 3 || i == 5 {
			n := int64(query.RowCount)
			query.Plan = SQLQueryPlan{Access: "SEARCH", Table: "t", Method: "btree", Index: "by_k", IndexCount: 1, Field: "k", FieldCount: 1, Recheck: true, Reason: "BINARY ASC leading equality prefix; rowid-ordered candidates; ANN and grams retain precedence"}
			query.Work = SQLQueryWork{IndexKeysRead: n, IndexPageReads: 1, RowIDSeeks: n, Candidates: n, BaseRowFetches: n, ReturnedRows: n}
		}
		snapshot.Queries[i] = query
	}
	return snapshot
}

func sqlReceipt(t *testing.T, plan Plan, events []Event) Receipt {
	t.Helper()
	receipt, problem := NewReceipt(plan)
	if problem != NoProblem {
		t.Fatalf("synthetic plan refused: %v", problem)
	}
	for _, event := range events {
		if receipt.Append(event) != NoProblem {
			break
		}
	}
	return receipt
}

func sqlEvidenceFixture(t *testing.T, which SQLCase) (Plan, Receipt, SQLLedgerEvidence) {
	t.Helper()
	identity := Identity{Suite: "sql-suite-v1", Source: "source-v1", Tool: "tool-v1", Binary: "binary-v1", Manifest: "manifest-v1", Trial: "sql-trial-1", Scope: "sql-scope-1", Profile: PlainRollback4096}
	plan, problem := SQLPlan(identity, which)
	if problem != NoProblem {
		t.Fatalf("synthetic SQL plan refused: %v", problem)
	}
	state := SQLOld
	if which == SQLControl {
		state = SQLNew
	}
	var hashes [3][32]byte
	hashes[0][0], hashes[1][0], hashes[2][0] = 11, 22, 33
	evidence := SQLLedgerEvidence{
		Identity: identity, Case: which,
		Before:    sqlSnapshot(identity, 101, 1, SQLOld),
		After:     sqlSnapshot(identity, 102, 8, state),
		Originals: SQLOriginalEvidence{Frozen: hashes, Before: hashes, After: hashes, BeforeBytes: 12288, AfterBytes: 12288},
		Recovery:  SQLRecoveryEvidence{Identity: identity, OwnersRetired: true},
		Lifecycle: SQLLifecycleEvidence{ArmCalls: 1, CommitCalls: 1, FinishCalls: 1, DisarmCalls: 1, WriterOpens: 1, ReopenCalls: 1, ReopenReadOnly: true, WriterRetiredBeforeRecovery: true, RecoveryFinishedBeforeReopen: true, CommitCapturedUnchanged: true},
	}
	if which != SQLControl {
		evidence.Recovery.Calls = 1
		evidence.Recovery.Limits = SQLRecoveryLimits{MaxJournalBytes: 16408, MaxRecords: 3, MaxDatabaseBytes: 12288}
		evidence.Recovery.JournalPresentBefore = true
		evidence.Recovery.Replayed, evidence.Recovery.ReplayedPages = true, 3
		evidence.Recovery.OriginalPages = 3
		evidence.Lifecycle.PublicCodeIOErr, evidence.Lifecycle.PublicRecoveryRequired = true, true
		evidence.Lifecycle.PublicShortWrite = which == SQLIndexPrefix64
	}
	labels := [SQLArtifactCount]string{"baseline.input.db", "trial.db", "trial.db-journal", "post-commit.input.db", "post-commit-journal.input.bin", "replay.db", "replay.db-journal"}
	for i := range evidence.Artifacts {
		role := SQLArtifactRole(i)
		evidence.Artifacts[i].Role = role
		journal := role == SQLOriginalJournalArtifact || role == SQLJournalCopyArtifact || role == SQLReplayJournalArtifact
		if journal && which == SQLControl {
			continue
		}
		stamp := SQLArtifactStamp{Identity: labels[i], Device: 1, Inode: uint64(i + 1), Exists: true, Bytes: 12288}
		stamp.SHA256[0] = 22
		if role == SQLBaselineArtifact {
			stamp.SHA256[0] = 11
		}
		if journal {
			stamp.Bytes, stamp.SHA256[0] = 16408, 33
		}
		evidence.Artifacts[i].Before, evidence.Artifacts[i].After = stamp, stamp
		if which != SQLControl && role == SQLReplayArtifact {
			evidence.Artifacts[i].After.SHA256[0] = 11
		}
		if role == SQLReplayJournalArtifact {
			evidence.Artifacts[i].After = SQLArtifactStamp{}
		}
	}
	return plan, sqlReceipt(t, plan, fixtureEvents(plan.Spec())), evidence
}

func sqlMustReject(t *testing.T, observation SQLObservation) {
	t.Helper()
	if observation.EvidenceConsistent || observation.ScenarioObserved || observation.Problem == SQLProblemNone || observation.PhysicalValidated() || observation.Pass() {
		t.Fatalf("invalid synthetic SQL evidence qualified: %+v", observation)
	}
}

func TestSQLLedgerThreeFrozenScenariosRemainObservationOnly(t *testing.T) {
	for _, which := range []SQLCase{SQLControl, SQLIndexPrefix64, SQLDBSyncBefore} {
		plan, receipt, evidence := sqlEvidenceFixture(t, which)
		observation := ObserveSQLLedger(plan, receipt, evidence)
		if !observation.EvidenceConsistent || !observation.ScenarioObserved || observation.Problem != SQLProblemNone || observation.Adapter.Outcome != AdapterObserved {
			t.Fatalf("scenario %v: %+v", which, observation)
		}
		if observation.PhysicalValidated() || observation.Pass() {
			t.Fatal("finite SQL evidence claimed physical acceptance")
		}
	}
	// The public result itself is not a capability to claim physical acceptance.
	forged := SQLObservation{EvidenceConsistent: true, ScenarioObserved: true}
	if forged.PhysicalValidated() || forged.Pass() {
		t.Fatal("fabricated result claimed physical acceptance")
	}
}

func TestSQLLedgerLiteralExpectationsAndQueriesAreDetached(t *testing.T) {
	for _, state := range []SQLState{SQLOld, SQLNew} {
		rows := SQLLedgerRows(state)
		if rows != sqlLiteralRows(state) {
			t.Fatalf("state %v differs from independent literal ledger", state)
		}
		rows[0].ID, rows[0].Key, rows[0].Value = 99, "wrong", "wrong"
		if SQLLedgerRows(state) != sqlLiteralRows(state) {
			t.Fatal("returned ledger aliases its frozen expectations")
		}
	}
	if SQLLedgerRows(SQLStateUnknown) != ([3]SQLRow{}) || SQLLedgerRows(SQLState(255)) != ([3]SQLRow{}) {
		t.Fatal("unknown ledger state received valid rows")
	}
	for query, text := range sqlLiteralQueryText() {
		if SQLQueryText(SQLQuery(query)) != text {
			t.Fatalf("query %v lost frozen text", query)
		}
	}
	if SQLQueryText(SQLQuery(255)) != "" {
		t.Fatal("unknown query received a valid statement")
	}
}

func TestSQLLedgerPlansFreezeThreePageSelectorsAndLimits(t *testing.T) {
	for _, which := range []SQLCase{SQLControl, SQLIndexPrefix64, SQLDBSyncBefore} {
		plan, _, _ := sqlEvidenceFixture(t, which)
		spec := plan.Spec()
		if spec.OriginalPages != 3 || spec.DirtyPages != 3 || spec.PageSize != 4096 || spec.SectorSize != 4096 || spec.MainOwner != 11 || spec.JournalOwner != 12 || spec.DirectoryOwner != 13 || spec.CleanupOwner != 14 {
			t.Fatalf("geometry/ownership changed: %+v", spec)
		}
		for i := 0; i < 3; i++ {
			read := spec.Calls[2+2*i]
			record := spec.Calls[3+2*i]
			write := spec.Calls[12+i]
			if read.Point != DBOriginalRead || read.Occurrence != uint8(i+1) || read.Offset != int64(i*4096) || record.Point != JournalRecord || record.Occurrence != uint8(i+1) || record.Offset != int64(4096+i*4104) || write.Point != DBPage || write.Occurrence != uint8(i+1) || write.Offset != int64(i*4096) {
				t.Fatalf("page role %v was not independently selected", i)
			}
		}
		switch which {
		case SQLControl:
			if spec.CallCount != 18 || spec.Action != Pass || spec.ExpectedCommit != CommitSucceeded || spec.Target != spec.Calls[14] {
				t.Fatal("control selector differs")
			}
		case SQLIndexPrefix64:
			if spec.CallCount != 15 || spec.Action != PrefixShortWrite || spec.Prefix != 64 || spec.PrefixError || spec.ExpectedCommit != CommitRecoveryRequired || spec.Target != spec.Calls[14] {
				t.Fatal("index prefix selector differs")
			}
		case SQLDBSyncBefore:
			if spec.CallCount != 16 || spec.Action != FailBefore || spec.ExpectedCommit != CommitRecoveryRequired || spec.Target != spec.Calls[15] {
				t.Fatal("sync-before selector differs")
			}
		}
		for i := int(spec.CallCount); i < MaxCalls; i++ {
			if spec.Calls[i] != (Call{}) {
				t.Fatal("unused call tail was retained")
			}
		}
	}
	plan, _, _ := sqlEvidenceFixture(t, SQLControl)
	if _, problem := SQLPlan(plan.Spec().Identity, SQLCaseUnknown); problem == NoProblem {
		t.Fatal("unknown SQL case admitted")
	}
	if _, problem := SQLPlan(Identity{}, SQLControl); problem == NoProblem {
		t.Fatal("missing SQL identity admitted")
	}
}

func TestSQLLedgerRejectsOtherwiseAdmittedNonFrozenFaultPlans(t *testing.T) {
	for _, name := range []string{"prefix65", "prefix-error", "earlier-db-page", "fourth-page-offset", "different-owner", "control-sync-target"} {
		which := SQLIndexPrefix64
		if name == "control-sync-target" {
			which = SQLControl
		}
		plan, receipt, evidence := sqlEvidenceFixture(t, which)
		spec := plan.Spec()
		switch name {
		case "prefix65":
			spec.Prefix = 65
		case "prefix-error":
			spec.PrefixError = true
		case "earlier-db-page":
			spec.Target, spec.CallCount = spec.Calls[13], 14
			for i := 14; i < MaxCalls; i++ {
				spec.Calls[i] = Call{}
			}
		case "fourth-page-offset":
			spec.Calls[14].Offset = 12288
			spec.Target = spec.Calls[14]
		case "different-owner":
			spec.MainOwner = 21
			for i := 0; i < int(spec.CallCount); i++ {
				if spec.Calls[i].Role == Database {
					spec.Calls[i].Owner = 21
				}
			}
			spec.Target = spec.Calls[14]
		case "control-sync-target":
			spec.Target = spec.Calls[15]
		}
		changed, problem := NewPlan(spec)
		if problem != NoProblem {
			t.Fatalf("%s negative should pass generic admission: %v", name, problem)
		}
		t.Run(name, func(t *testing.T) { sqlMustReject(t, ObserveSQLLedger(changed, receipt, evidence)) })
	}
}

func TestSQLLedgerRejectsLogicalMixturesAndEqualCountWrongMembers(t *testing.T) {
	for _, which := range []SQLCase{SQLControl, SQLIndexPrefix64, SQLDBSyncBefore} {
		for _, name := range []string{"wrong-ledger", "oldrows-newindex", "newrows-oldindex", "equal-count-wrong-member", "equal-count-wrong-value", "index-scan-both-wrong", "wrong-id-type", "wrong-key-type", "wrong-value-type", "same-count-duplicate-id", "unreported-tail", "row-reordered", "empty-green-hidden-member", "before-not-old"} {
			plan, receipt, evidence := sqlEvidenceFixture(t, which)
			old := sqlSnapshot(evidence.Identity, 102, 8, SQLOld)
			newState := sqlSnapshot(evidence.Identity, 102, 8, SQLNew)
			switch name {
			case "wrong-ledger":
				if which == SQLControl {
					evidence.After = old
				} else {
					evidence.After = newState
				}
			case "oldrows-newindex":
				evidence.After = old
				for _, q := range []SQLQuery{SQLBlueIndex, SQLRedIndex, SQLGreenIndex} {
					evidence.After.Queries[q] = newState.Queries[q]
				}
			case "newrows-oldindex":
				evidence.After = newState
				for _, q := range []SQLQuery{SQLBlueIndex, SQLRedIndex, SQLGreenIndex} {
					evidence.After.Queries[q] = old.Queries[q]
				}
			case "equal-count-wrong-member":
				evidence.After.Queries[SQLBlueIndex].Rows[0].ID = 77
			case "equal-count-wrong-value":
				evidence.After.Queries[SQLAllRows].Rows[0].Value = "one!"
			case "index-scan-both-wrong":
				evidence.After.Queries[SQLBlueIndex].Rows[0].ID = 88
				evidence.After.Queries[SQLBlueScan].Rows[0].ID = 88
			case "wrong-id-type":
				evidence.After.Queries[SQLAllRows].Rows[0].IDType = SQLText
			case "wrong-key-type":
				evidence.After.Queries[SQLAllRows].Rows[0].KeyType = SQLInteger
			case "wrong-value-type":
				evidence.After.Queries[SQLAllRows].Rows[0].ValueType = SQLTypeUnknown
			case "same-count-duplicate-id":
				evidence.After.Queries[SQLAllRows].Rows[1].ID = evidence.After.Queries[SQLAllRows].Rows[0].ID
			case "unreported-tail":
				evidence.After.Queries[SQLBlueIndex].Rows[2] = old.Queries[SQLAllRows].Rows[0]
			case "row-reordered":
				evidence.After.Queries[SQLAllRows].Rows[0], evidence.After.Queries[SQLAllRows].Rows[1] = evidence.After.Queries[SQLAllRows].Rows[1], evidence.After.Queries[SQLAllRows].Rows[0]
			case "empty-green-hidden-member":
				evidence.Before.Queries[SQLGreenIndex].Rows[0] = old.Queries[SQLAllRows].Rows[0]
			case "before-not-old":
				evidence.Before = sqlSnapshot(evidence.Identity, 101, 1, SQLNew)
			}
			t.Run(name, func(t *testing.T) { sqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence)) })
		}
	}
}

func TestSQLLedgerRejectsPlanClaimsWithoutActualIndexWorkAndIndependentScans(t *testing.T) {
	for _, name := range []string{"scan-disguised-as-search", "wrong-method", "wrong-table", "wrong-index", "extra-index", "wrong-field", "extra-field", "no-recheck", "no-index-pages", "no-index-keys", "no-rowid-seek", "no-base-fetch", "no-candidates", "wrong-return-count", "negative-work", "unbounded-work", "empty-green-no-probe", "scan-uses-index", "scan-fewer-rows", "scan-has-candidates", "scan-fake-reason", "scan-has-method", "copied-index-query", "changed-sql", "reordered-query", "duplicate-iterator"} {
		plan, receipt, evidence := sqlEvidenceFixture(t, SQLControl)
		query := &evidence.After.Queries[SQLBlueIndex]
		switch name {
		case "scan-disguised-as-search":
			query.Plan.Access = "SCAN"
		case "wrong-method":
			query.Plan.Method = "rowid"
		case "wrong-table":
			query.Plan.Table = "other"
		case "wrong-index":
			query.Plan.Index = "other"
		case "extra-index":
			query.Plan.IndexCount = 2
		case "wrong-field":
			query.Plan.Field = "v"
		case "extra-field":
			query.Plan.FieldCount = 2
		case "no-recheck":
			query.Plan.Recheck = false
		case "no-index-pages":
			query.Work.IndexPageReads = 0
		case "no-index-keys":
			query.Work.IndexKeysRead = 0
		case "no-rowid-seek":
			query.Work.RowIDSeeks = 0
		case "no-base-fetch":
			query.Work.BaseRowFetches = 0
		case "no-candidates":
			query.Work.Candidates = 0
		case "wrong-return-count":
			query.Work.ReturnedRows = 2
		case "negative-work":
			query.Work.IndexPageReads = -1
		case "unbounded-work":
			query.Work.IndexPageReads = 33
		case "empty-green-no-probe":
			evidence.Before.Queries[SQLGreenIndex].Work.IndexPageReads = 0
		case "scan-uses-index":
			evidence.After.Queries[SQLBlueScan].Work.IndexKeysRead = 1
		case "scan-fewer-rows":
			evidence.After.Queries[SQLBlueScan].Work.BaseRowFetches = 2
		case "scan-has-candidates":
			evidence.After.Queries[SQLBlueScan].Work.Candidates = 1
		case "scan-fake-reason":
			evidence.After.Queries[SQLBlueScan].Plan.Reason = "no index"
		case "scan-has-method":
			evidence.After.Queries[SQLBlueScan].Plan.Method = "btree"
		case "copied-index-query":
			evidence.After.Queries[SQLBlueScan] = evidence.After.Queries[SQLBlueIndex]
		case "changed-sql":
			query.SQL = "EXPLAIN QUERY PLAN SELECT id,k,v FROM t WHERE k='blue' ORDER BY id"
		case "reordered-query":
			evidence.After.Queries[SQLBlueScan], evidence.After.Queries[SQLRedScan] = evidence.After.Queries[SQLRedScan], evidence.After.Queries[SQLBlueScan]
		case "duplicate-iterator":
			evidence.After.Queries[SQLBlueScan].Iterator = query.Iterator
		}
		t.Run(name, func(t *testing.T) { sqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence)) })
	}
}

func TestSQLLedgerRequiresEOFErrOnceCloseAndIndependentRetiredOwners(t *testing.T) {
	for _, name := range []string{"not-eof", "err-not-called", "err-present", "err-token", "missing-row-close", "duplicate-row-close", "row-close-error", "row-close-token", "missing-db-close", "duplicate-db-close", "db-close-error", "db-close-token", "db-owner-live", "same-owner", "writer-owner", "zero-owner", "zero-iterator", "iterator-reused-across-opens", "query-owner-mismatch", "check-owner-mismatch", "second-arm", "missing-arm", "second-commit", "missing-commit", "missing-finish", "second-finish", "missing-disarm", "second-disarm", "reconstructed-commit-error", "missing-public-code", "missing-public-recovery", "missing-public-shortwrite", "rollback-after-commit", "second-writer-open", "second-reopen", "writable-reopen", "unretired-before-recovery", "reopen-before-recovery"} {
		plan, receipt, evidence := sqlEvidenceFixture(t, SQLIndexPrefix64)
		query := &evidence.After.Queries[SQLAllRows]
		switch name {
		case "not-eof":
			query.EOF = false
		case "err-not-called":
			query.ErrCalled = false
		case "err-present":
			query.Error, query.ErrorToken = Underlying, 10
		case "err-token":
			query.ErrorToken = 10
		case "missing-row-close":
			query.CloseCalls = 0
		case "duplicate-row-close":
			query.CloseCalls = 2
		case "row-close-error":
			query.CloseError, query.CloseErrorToken = Underlying, 10
		case "row-close-token":
			query.CloseErrorToken = 10
		case "missing-db-close":
			evidence.After.CloseCalls = 0
		case "duplicate-db-close":
			evidence.After.CloseCalls = 2
		case "db-close-error":
			evidence.After.CloseError, evidence.After.CloseErrorToken = Underlying, 10
		case "db-close-token":
			evidence.After.CloseErrorToken = 10
		case "db-owner-live":
			evidence.After.OwnerState = OwnerLive
		case "same-owner":
			evidence.After.Owner = evidence.Before.Owner
		case "writer-owner":
			evidence.After.Owner = plan.Spec().MainOwner
		case "zero-owner":
			evidence.After.Owner = 0
		case "zero-iterator":
			query.Iterator = 0
		case "iterator-reused-across-opens":
			query.Iterator = evidence.Before.Queries[SQLAllRows].Iterator
		case "query-owner-mismatch":
			query.Owner = 99
		case "check-owner-mismatch":
			evidence.After.Check.Owner = 99
		case "second-arm":
			evidence.Lifecycle.ArmCalls = 2
		case "missing-arm":
			evidence.Lifecycle.ArmCalls = 0
		case "second-commit":
			evidence.Lifecycle.CommitCalls = 2
		case "missing-commit":
			evidence.Lifecycle.CommitCalls = 0
		case "missing-finish":
			evidence.Lifecycle.FinishCalls = 0
		case "second-finish":
			evidence.Lifecycle.FinishCalls = 2
		case "missing-disarm":
			evidence.Lifecycle.DisarmCalls = 0
		case "second-disarm":
			evidence.Lifecycle.DisarmCalls = 2
		case "reconstructed-commit-error":
			evidence.Lifecycle.CommitCapturedUnchanged = false
		case "missing-public-code":
			evidence.Lifecycle.PublicCodeIOErr = false
		case "missing-public-recovery":
			evidence.Lifecycle.PublicRecoveryRequired = false
		case "missing-public-shortwrite":
			evidence.Lifecycle.PublicShortWrite = false
		case "rollback-after-commit":
			evidence.Lifecycle.RollbackCalls = 1
		case "second-writer-open":
			evidence.Lifecycle.WriterOpens = 2
		case "second-reopen":
			evidence.Lifecycle.ReopenCalls = 2
		case "writable-reopen":
			evidence.Lifecycle.ReopenReadOnly = false
		case "unretired-before-recovery":
			evidence.Lifecycle.WriterRetiredBeforeRecovery = false
		case "reopen-before-recovery":
			evidence.Lifecycle.RecoveryFinishedBeforeReopen = false
		}
		t.Run(name, func(t *testing.T) { sqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence)) })
	}
}

func TestSQLLedgerRequiresCheckHashesAndExactRecoveryEvidence(t *testing.T) {
	for _, name := range []string{"missing-check", "duplicate-check", "check-error", "check-token", "pages", "tables", "indexes", "rows", "entries", "extensions", "free-pages", "reserved-pages", "before-hash-changed", "after-hash-changed", "zero-hash", "before-size", "after-size", "missing-recovery", "second-recovery", "recovery-error", "recovery-token", "journal-byte-limit", "record-limit", "database-limit", "missing-hot-journal", "journal-survives", "missing-replay", "too-few-replay-pages", "too-many-replay-pages", "wrong-original-pages", "recovery-owner-live", "physical-claim"} {
		plan, receipt, evidence := sqlEvidenceFixture(t, SQLIndexPrefix64)
		switch name {
		case "missing-check":
			evidence.After.Check.Calls = 0
		case "duplicate-check":
			evidence.After.Check.Calls = 2
		case "check-error":
			evidence.After.Check.Error, evidence.After.Check.ErrorToken = Underlying, 5
		case "check-token":
			evidence.After.Check.ErrorToken = 5
		case "pages":
			evidence.After.Check.Pages = 4
		case "tables":
			evidence.After.Check.Tables = 2
		case "indexes":
			evidence.After.Check.Indexes = 0
		case "rows":
			evidence.After.Check.TableRows = 2
		case "entries":
			evidence.After.Check.IndexEntries = 2
		case "extensions":
			evidence.After.Check.ExtensionIndexes = 1
		case "free-pages":
			evidence.After.Check.FreePages = 1
		case "reserved-pages":
			evidence.After.Check.ReservedPages = 1
		case "before-hash-changed":
			evidence.Originals.Before[1][0]++
		case "after-hash-changed":
			evidence.Originals.After[2][0]++
		case "zero-hash":
			evidence.Originals.Frozen[1], evidence.Originals.Before[1], evidence.Originals.After[1] = [32]byte{}, [32]byte{}, [32]byte{}
		case "before-size":
			evidence.Originals.BeforeBytes--
		case "after-size":
			evidence.Originals.AfterBytes++
		case "missing-recovery":
			evidence.Recovery.Calls = 0
		case "second-recovery":
			evidence.Recovery.Calls = 2
		case "recovery-error":
			evidence.Recovery.Error, evidence.Recovery.ErrorToken = Underlying, 5
		case "recovery-token":
			evidence.Recovery.ErrorToken = 5
		case "journal-byte-limit":
			evidence.Recovery.Limits.MaxJournalBytes++
		case "record-limit":
			evidence.Recovery.Limits.MaxRecords = 4
		case "database-limit":
			evidence.Recovery.Limits.MaxDatabaseBytes = 16384
		case "missing-hot-journal":
			evidence.Recovery.JournalPresentBefore = false
		case "journal-survives":
			evidence.Recovery.JournalPresentAfter = true
		case "missing-replay":
			evidence.Recovery.Replayed = false
		case "too-few-replay-pages":
			evidence.Recovery.ReplayedPages = 2
		case "too-many-replay-pages":
			evidence.Recovery.ReplayedPages = 4
		case "wrong-original-pages":
			evidence.Recovery.OriginalPages = 2
		case "recovery-owner-live":
			evidence.Recovery.OwnersRetired = false
		case "physical-claim":
			evidence.PhysicalClaim = true
		}
		t.Run(name, func(t *testing.T) { sqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence)) })
	}
	plan, receipt, evidence := sqlEvidenceFixture(t, SQLControl)
	evidence.Recovery.Replayed, evidence.Recovery.ReplayedPages = true, 3
	sqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence))
}

func TestSQLLedgerRetainsOriginalsAndRejectsAliasedOrMixedReplayArtifacts(t *testing.T) {
	for _, which := range []SQLCase{SQLControl, SQLIndexPrefix64, SQLDBSyncBefore} {
		for _, name := range []string{"wrong-role", "absent-baseline", "absent-trial", "absent-replay", "changed-baseline", "changed-trial", "changed-post-copy", "replay-aliases-trial", "post-aliases-trial", "changed-inode", "changed-device", "zero-device", "zero-inode", "zero-hash", "wrong-size", "wrong-label", "same-post-and-baseline", "wrong-replay-input", "wrong-replay-output", "same-old-and-new-hash"} {
			plan, receipt, evidence := sqlEvidenceFixture(t, which)
			switch name {
			case "wrong-role":
				evidence.Artifacts[SQLTrialArtifact].Role = SQLPostCopyArtifact
			case "absent-baseline":
				evidence.Artifacts[SQLBaselineArtifact].Before = SQLArtifactStamp{}
			case "absent-trial":
				evidence.Artifacts[SQLTrialArtifact].Before = SQLArtifactStamp{}
			case "absent-replay":
				evidence.Artifacts[SQLReplayArtifact].Before = SQLArtifactStamp{}
			case "changed-baseline":
				evidence.Artifacts[SQLBaselineArtifact].After.SHA256[0]++
			case "changed-trial":
				evidence.Artifacts[SQLTrialArtifact].After.SHA256[0]++
			case "changed-post-copy":
				evidence.Artifacts[SQLPostCopyArtifact].After.SHA256[0]++
			case "replay-aliases-trial":
				evidence.Artifacts[SQLReplayArtifact].Before.Inode, evidence.Artifacts[SQLReplayArtifact].After.Inode = evidence.Artifacts[SQLTrialArtifact].Before.Inode, evidence.Artifacts[SQLTrialArtifact].Before.Inode
			case "post-aliases-trial":
				evidence.Artifacts[SQLPostCopyArtifact].Before.Inode, evidence.Artifacts[SQLPostCopyArtifact].After.Inode = evidence.Artifacts[SQLTrialArtifact].Before.Inode, evidence.Artifacts[SQLTrialArtifact].Before.Inode
			case "changed-inode":
				evidence.Artifacts[SQLReplayArtifact].After.Inode++
			case "changed-device":
				evidence.Artifacts[SQLReplayArtifact].After.Device++
			case "zero-device":
				evidence.Artifacts[SQLBaselineArtifact].Before.Device = 0
			case "zero-inode":
				evidence.Artifacts[SQLBaselineArtifact].Before.Inode = 0
			case "zero-hash":
				evidence.Artifacts[SQLBaselineArtifact].Before.SHA256 = [32]byte{}
			case "wrong-size":
				evidence.Artifacts[SQLTrialArtifact].Before.Bytes++
			case "wrong-label":
				evidence.Artifacts[SQLTrialArtifact].Before.Identity = "baseline.input.db"
			case "same-post-and-baseline":
				evidence.Artifacts[SQLPostCopyArtifact].Before.SHA256, evidence.Artifacts[SQLPostCopyArtifact].After.SHA256 = evidence.Artifacts[SQLBaselineArtifact].Before.SHA256, evidence.Artifacts[SQLBaselineArtifact].Before.SHA256
			case "wrong-replay-input":
				evidence.Artifacts[SQLReplayArtifact].Before.SHA256[0]++
			case "wrong-replay-output":
				evidence.Artifacts[SQLReplayArtifact].After.SHA256[0]++
			case "same-old-and-new-hash":
				for _, role := range []SQLArtifactRole{SQLTrialArtifact, SQLPostCopyArtifact, SQLReplayArtifact} {
					evidence.Artifacts[role].Before.SHA256 = evidence.Artifacts[SQLBaselineArtifact].Before.SHA256
					evidence.Artifacts[role].After.SHA256 = evidence.Artifacts[SQLBaselineArtifact].Before.SHA256
				}
			}
			t.Run(name, func(t *testing.T) { sqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence)) })
		}
	}
	for _, name := range []string{"original-journal-removed", "original-journal-changed", "journal-copy-removed", "journal-copy-changed", "replay-journal-missing", "replay-journal-survives", "replay-journal-aliased", "different-journal-copy", "different-replay-journal"} {
		plan, receipt, evidence := sqlEvidenceFixture(t, SQLIndexPrefix64)
		switch name {
		case "original-journal-removed":
			evidence.Artifacts[SQLOriginalJournalArtifact].After = SQLArtifactStamp{}
		case "original-journal-changed":
			evidence.Artifacts[SQLOriginalJournalArtifact].After.SHA256[0]++
		case "journal-copy-removed":
			evidence.Artifacts[SQLJournalCopyArtifact].After = SQLArtifactStamp{}
		case "journal-copy-changed":
			evidence.Artifacts[SQLJournalCopyArtifact].After.SHA256[0]++
		case "replay-journal-missing":
			evidence.Artifacts[SQLReplayJournalArtifact].Before = SQLArtifactStamp{}
		case "replay-journal-survives":
			evidence.Artifacts[SQLReplayJournalArtifact].After = evidence.Artifacts[SQLReplayJournalArtifact].Before
		case "replay-journal-aliased":
			evidence.Artifacts[SQLReplayJournalArtifact].Before.Inode = evidence.Artifacts[SQLOriginalJournalArtifact].Before.Inode
		case "different-journal-copy":
			evidence.Artifacts[SQLJournalCopyArtifact].Before.SHA256[0], evidence.Artifacts[SQLJournalCopyArtifact].After.SHA256[0] = 88, 88
		case "different-replay-journal":
			evidence.Artifacts[SQLReplayJournalArtifact].Before.SHA256[0] = 88
		}
		t.Run(name, func(t *testing.T) { sqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence)) })
	}
	plan, receipt, evidence := sqlEvidenceFixture(t, SQLControl)
	evidence.Recovery.Calls = 1
	sqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence))
	plan, receipt, evidence = sqlEvidenceFixture(t, SQLControl)
	evidence.Artifacts[SQLOriginalJournalArtifact].Before.Exists = true
	sqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence))
}

func TestSQLLedgerSnapshotAdmissionRejectsBeforeCommitMismatches(t *testing.T) {
	_, _, evidence := sqlEvidenceFixture(t, SQLControl)
	if problem := ValidateSQLSnapshot(evidence.Before, evidence.Identity, SQLOld); problem != SQLProblemNone {
		t.Fatalf("independent old snapshot refused: %v", problem)
	}
	if ValidateSQLSnapshot(evidence.Before, evidence.Identity, SQLStateUnknown) == SQLProblemNone {
		t.Fatal("unknown snapshot state admitted")
	}
	if ValidateSQLSnapshot(evidence.Before, evidence.Identity, SQLNew) == SQLProblemNone {
		t.Fatal("old snapshot admitted as new")
	}
	changed := evidence.Before
	changed.Identity.Trial = "other-trial"
	if ValidateSQLSnapshot(changed, evidence.Identity, SQLOld) == SQLProblemNone {
		t.Fatal("snapshot identity mismatch admitted")
	}
	changed = evidence.Before
	changed.Owner = 0
	if ValidateSQLSnapshot(changed, evidence.Identity, SQLOld) == SQLProblemNone {
		t.Fatal("zero snapshot owner admitted")
	}
	changed = evidence.Before
	changed.Queries[SQLRedIndex].Rows[0].ID = 9
	if ValidateSQLSnapshot(changed, evidence.Identity, SQLOld) == SQLProblemNone {
		t.Fatal("equal-count wrong index member admitted")
	}
}

func TestSQLLedgerBindsSourceToolBinaryManifestTrialAndScopeEverywhere(t *testing.T) {
	for _, where := range []string{"top", "before", "after", "query", "check", "recovery"} {
		for _, field := range []string{"suite", "source", "tool", "binary", "manifest", "trial", "scope", "profile"} {
			plan, receipt, evidence := sqlEvidenceFixture(t, SQLControl)
			identity := &evidence.Identity
			switch where {
			case "before":
				identity = &evidence.Before.Identity
			case "after":
				identity = &evidence.After.Identity
			case "query":
				identity = &evidence.After.Queries[SQLBlueIndex].Identity
			case "check":
				identity = &evidence.After.Check.Identity
			case "recovery":
				identity = &evidence.Recovery.Identity
			}
			switch field {
			case "suite":
				identity.Suite = "another-suite"
			case "source":
				identity.Source = "another-source"
			case "tool":
				identity.Tool = "another-tool"
			case "binary":
				identity.Binary = "another-binary"
			case "manifest":
				identity.Manifest = "another-manifest"
			case "trial":
				identity.Trial = "another-trial"
			case "scope":
				identity.Scope = "another-scope"
			case "profile":
				identity.Profile = ProfileUnknown
			}
			t.Run(where+"/"+field, func(t *testing.T) { sqlMustReject(t, ObserveSQLLedger(plan, receipt, evidence)) })
		}
	}
}

func TestSQLLedgerRevalidatesReceiptRatherThanTrustingAnObservation(t *testing.T) {
	for _, which := range []SQLCase{SQLControl, SQLIndexPrefix64, SQLDBSyncBefore} {
		plan, _, evidence := sqlEvidenceFixture(t, which)
		events := fixtureEvents(plan.Spec())
		for end := 0; end < len(events); end++ {
			sqlMustReject(t, ObserveSQLLedger(plan, sqlReceipt(t, plan, events[:end]), evidence))
		}
		for _, name := range []string{"second-commit", "missing-owner", "owner-live", "wrong-commit", "missing-target-error", "target-normalization", "wrong-count", "physical-validation", "foreign-trial"} {
			changed := append([]Event(nil), events...)
			for i := range changed {
				switch name {
				case "second-commit":
					if changed[i].Kind == CommitReturned {
						changed = append(changed[:i+1], append([]Event{changed[i]}, changed[i+1:]...)...)
						renumber(changed, plan.Spec().Identity)
						goto evaluate
					}
				case "missing-owner":
					if changed[i].Kind == OwnersObserved {
						changed[i].Owners.Main = OwnerUnknown
					}
				case "owner-live":
					if changed[i].Kind == OwnersObserved {
						changed[i].Owners.Main = OwnerLive
					}
				case "wrong-commit":
					if changed[i].Kind == CommitReturned {
						changed[i].Commit.Class = CommitUncertain
					}
				case "missing-target-error":
					if changed[i].Kind == CommitReturned {
						changed[i].Commit.TargetError = None
						if which == SQLControl {
							changed[i].Commit.TargetError = ShortWrite
						}
					}
				case "target-normalization":
					if changed[i].Kind == Returned && changed[i].Call == plan.Spec().Target {
						changed[i].Result.NormalizedError = None
						if which == SQLControl {
							changed[i].Result.NormalizedError = ShortWrite
						}
					}
				case "wrong-count":
					if changed[i].Kind == Returned && changed[i].Call == plan.Spec().Target {
						changed[i].Result.ReturnedN++
					}
				case "physical-validation":
					if changed[i].Kind == ValidationObserved {
						changed[i].Validation = Validation(1)
					}
				case "foreign-trial":
					changed[i].Identity.Trial = "other-trial"
				}
			}
		evaluate:
			t.Run(name, func(t *testing.T) { sqlMustReject(t, ObserveSQLLedger(plan, sqlReceipt(t, plan, changed), evidence)) })
		}
		sqlMustReject(t, ObserveSQLLedger(Plan{}, Receipt{}, evidence))
		otherPlan, _, _ := sqlEvidenceFixture(t, SQLControl)
		if which == SQLControl {
			otherPlan, _, _ = sqlEvidenceFixture(t, SQLDBSyncBefore)
		}
		sqlMustReject(t, ObserveSQLLedger(plan, sqlReceipt(t, otherPlan, fixtureEvents(otherPlan.Spec())), evidence))
	}
}
