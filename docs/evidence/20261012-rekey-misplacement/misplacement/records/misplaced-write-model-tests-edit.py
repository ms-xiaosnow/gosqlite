from pathlib import Path
O=Path('/workspace/scratch/5e06a74e5f65/restart-20261012T2033/misplaced-write-overlay')
p=O/'internal/testkit/rollbackfault/contract_test.go';s=p.read_text(); needle='\t\tcase PrefixShortWrite:\n'; assert s.count(needle)==1;s=s.replace(needle,'\t\tcase MisplaceIndexPageMinus64:\n\t\t\tresult = Result{Executed: true, NativeN: 4096, ReturnedN: 4096, ReturnedError: Injected, NormalizedError: Injected, Effect: EffectPossible}\n'+needle)
s+='''
// The only additional action is a fixed actual placement, not a caller offset.
func TestRollbackFaultMisplacementAdmissionIsExact(t *testing.T) {
	plan, problem := SQLPlan(onePageSpec(Pass, 8).Identity, SQLIndexMisplaceMinus64Error)
	if problem != NoProblem {
		t.Fatal(problem)
	}
	spec := plan.Spec()
	if spec.Action != MisplaceIndexPageMinus64 || spec.CallCount != 15 || spec.Target != (Call{Point: DBPage, Occurrence: 3, Role: Database, Offset: 8192, Length: 4096, Owner: 11}) || spec.Prefix != 0 || spec.PrefixError || spec.ExpectedCommit != CommitRecoveryRequired {
		t.Fatal("fixed misplacement plan differs", spec)
	}
	observation := observe(t, spec, fixtureEvents(spec))
	if observation.Outcome != AdapterObserved || observation.EventCount != 35 || observation.TargetHits != 1 || observation.Problem != NoProblem || observation.Pass() || observation.PhysicalValidated() {
		t.Fatal("one complete misplaced-write trace must remain nonphysical", observation)
	}
	for _, name := range []string{"occurrence", "journal-role", "directory-role", "owner", "offset-minus64", "offset-plus64", "offset-zero", "length-prefix", "length-short", "dirty-two", "dirty-four", "original-two", "original-four", "page-size", "sector-size", "prefix", "prefix-error", "call-count", "success", "uncertain", "original-read-inventory", "dirty-write-inventory"} {
		bad := spec
		switch name {
		case "occurrence": bad.Target.Occurrence = 2
		case "journal-role": bad.Target.Role = Journal
		case "directory-role": bad.Target.Role = Directory
		case "owner": bad.Target.Owner = bad.JournalOwner
		case "offset-minus64": bad.Target.Offset = 8128
		case "offset-plus64": bad.Target.Offset = 8256
		case "offset-zero": bad.Target.Offset = 0
		case "length-prefix": bad.Target.Length = 64
		case "length-short": bad.Target.Length = 4095
		case "dirty-two": bad.DirtyPages = 2
		case "dirty-four": bad.DirtyPages = 4
		case "original-two": bad.OriginalPages = 2
		case "original-four": bad.OriginalPages = 4
		case "page-size": bad.PageSize = 8192
		case "sector-size": bad.SectorSize = 512
		case "prefix": bad.Prefix = 64
		case "prefix-error": bad.PrefixError = true
		case "call-count": bad.CallCount = 16
		case "success": bad.ExpectedCommit = CommitSucceeded
		case "uncertain": bad.ExpectedCommit = CommitUncertain
		case "original-read-inventory": bad.Calls[4].Offset = 8192
		case "dirty-write-inventory": bad.Calls[13].Offset = 8192
		}
		if _, problem := NewPlan(bad); problem == NoProblem {
			t.Fatal("misplacement selector accepted", name)
		}
	}
	for value := 0; value <= 255; value++ {
		bad := spec
		bad.Target.Point = Point(value)
		if bad.Target.Point != DBPage {
			if _, problem := NewPlan(bad); problem == NoProblem { t.Fatal("misplacement point accepted", value) }
		}
		bad = spec
		bad.Action = Action(value)
		if value > int(MisplaceIndexPageMinus64) {
			if _, problem := NewPlan(bad); problem == NoProblem { t.Fatal("future action accepted", value) }
		}
	}
	for i := 0; i < 12; i++ {
		if _, problem := NewPlan(onePageSpec(MisplaceIndexPageMinus64, i)); problem == NoProblem { t.Fatal("one-page action accepted", i) }
	}
}

func TestRollbackFaultMisplacementNativeResultsKeepOriginalError(t *testing.T) {
	plan, problem := SQLPlan(onePageSpec(Pass, 8).Identity, SQLIndexMisplaceMinus64Error)
	if problem != NoProblem { t.Fatal(problem) }
	spec := plan.Spec()
	for _, name := range []string{"success-exposed", "prefix", "short-nil", "unexecuted", "wrong-effect", "short-write", "normalized-nil", "returned-token", "requested-offset", "masked-native-error", "masked-native-token", "success-commit", "uncertain-commit", "missing-target", "logical-open", "pending-owner", "unknown-owner"} {
		events := fixtureEvents(spec)
		result := &events[30].Result
		switch name {
		case "success-exposed": result.ReturnedError, result.NormalizedError = None, None
		case "prefix": result.NativeN, result.ReturnedN = 64, 64
		case "short-nil": result.NativeN, result.ReturnedN, result.ReturnedError, result.NormalizedError = 4095, 4095, None, ShortWrite
		case "unexecuted": result.Executed = false
		case "wrong-effect": result.Effect = EffectPrefix
		case "short-write": result.ReturnedError, result.NormalizedError = ShortWrite, ShortWrite
		case "normalized-nil": result.NormalizedError = None
		case "returned-token": result.ReturnedErrorToken = 71
		case "requested-offset": events[30].Call.Offset = 8128
		case "masked-native-error": result.NativeError, result.NativeErrorToken, result.Effect = Underlying, 71, EffectUnknown
		case "masked-native-token":
			result.NativeError, result.NativeErrorToken, result.Effect = Underlying, 71, EffectUnknown
			result.ReturnedError, result.ReturnedErrorToken = Underlying, 72
			result.NormalizedError, result.NormalizedErrorToken = Underlying, 72
		case "success-commit": events[31].Commit.Class = CommitSucceeded
		case "uncertain-commit": events[31].Commit.Class = CommitUncertain
		case "missing-target": events[31].Commit.TargetError = None
		case "logical-open": events[31].Commit.Logical = LogicalOpen
		case "pending-owner": events[32].Owners.Cleanup = OwnerPending
		case "unknown-owner": events[32].Owners.Journal = OwnerUnknown
		}
		mustNotQualify(t, observe(t, spec, events))
	}
	for _, count := range []int{0, 64, 4095, 4096} {
		events := fixtureEvents(spec)
		events[30].Result = Result{Executed: true, NativeN: count, NativeError: Underlying, NativeErrorToken: 71, ReturnedN: count, ReturnedError: Underlying, ReturnedErrorToken: 71, NormalizedError: Underlying, NormalizedErrorToken: 71, Effect: EffectUnknown}
		events[31].Commit.TargetError, events[31].Commit.TargetErrorToken = Underlying, 71
		observation := observe(t, spec, events)
		if observation.Outcome != ObservationRefused || observation.Problem != ProblemUnderlyingFailure { t.Fatal("original native error must remain refused evidence", count, observation) }
	}
}
''';p.write_text(s)
p=O/'internal/testkit/rollbackfault/sql_ledger_test.go';s=p.read_text();s=s.replace('which == SQLIndexPrefix64 || which == SQLDBSyncBefore || which == SQLIndexPrefix64Error || which == SQLJournalUnlinkBefore','which == SQLIndexPrefix64 || which == SQLDBSyncBefore || which == SQLIndexPrefix64Error || which == SQLJournalUnlinkBefore || which == SQLIndexMisplaceMinus64Error')
s=s.replace('SQLCase(10)','SQLCase(11)').replace('for value := 10; value <= 255;', 'for value := 11; value <= 255;')
needle='\treturn plan, sqlReceipt(t, plan, fixtureEvents(plan.Spec())), evidence'; assert s.count(needle)==1
s=s.replace(needle,'''\tif which == SQLIndexMisplaceMinus64Error {
		evidence.Misplacement = SQLMisplacementEvidence{
			Writer: SQLMisplacedWrite{RequestedOffset: 8192, RequestedBytes: 4096, NativeOffset: 8128, NativeBytes: 4096, NativeCalls: 1, PayloadSHA256: [32]byte{44}},
			ShiftedSHA256: [32]byte{44}, BaselineRangeSHA256: [32]byte{55}, BaselineTailSHA256: [32]byte{66}, UntouchedTailSHA256: [32]byte{66}, JournalOriginals: hashes, OnlyTargetCause: true,
		}
	}
'''+needle)
s+='''
func TestSQLLedgerMisplacementRequiresCompleteOldAndActualEvidence(t *testing.T) {
	plan, receipt, evidence := sqlEvidenceFixture(t, SQLIndexMisplaceMinus64Error)
	observed := ObserveSQLLedger(plan, receipt, evidence)
	if !observed.EvidenceConsistent || !observed.ScenarioObserved || observed.Problem != SQLProblemNone || observed.Adapter.EventCount != 35 || observed.Adapter.TargetHits != 1 || observed.Pass() || observed.PhysicalValidated() {
		t.Fatal("fixed misplaced-write evidence rejected or became physical", observed)
	}
	if int(SQLIndexMisplaceMinus64Error) != 10 || isSQLPreDBFailure(SQLIndexMisplaceMinus64Error) || isSQLPostDBFailure(SQLIndexMisplaceMinus64Error) || !needsSQLRecovery(SQLIndexMisplaceMinus64Error) || evidence.PostCommit != (SQLSnapshotEvidence{}) || evidence.Lifecycle.ReopenCalls != 1 {
		t.Fatal("misplaced-write case partition changed")
	}
	for _, name := range []string{"missing", "requested-offset", "requested-length", "native-correct-offset", "native-plus64", "native-minus128", "native-zero", "native-prefix", "native-short", "zero-calls", "two-calls", "zero-payload", "wrong-payload", "baseline-payload", "zero-baseline-range", "changed-tail", "zero-tails", "journal-missing", "journal-wrong", "journal-reordered", "extra-close-cause", "NEW", "mixed-index", "wrong-member", "post-copy-NEW", "recovery-zero", "recovery-two", "wrong-limits", "journal-before-absent", "journal-after-present", "owners-live", "replay-alias", "changed-original", "changed-trial", "replay-NEW", "public-success", "public-uncertain", "public-short", "public-pending", "pre-DB-cause", "post-DB-cause", "two-reopens", "physical"} {
		bad := evidence
		m := &bad.Misplacement
		switch name {
		case "missing": *m = SQLMisplacementEvidence{}
		case "requested-offset": m.Writer.RequestedOffset = 8128
		case "requested-length": m.Writer.RequestedBytes = 64
		case "native-correct-offset": m.Writer.NativeOffset = 8192
		case "native-plus64": m.Writer.NativeOffset = 8256
		case "native-minus128": m.Writer.NativeOffset = 8064
		case "native-zero": m.Writer.NativeOffset = 0
		case "native-prefix": m.Writer.NativeBytes = 64
		case "native-short": m.Writer.NativeBytes = 4095
		case "zero-calls": m.Writer.NativeCalls = 0
		case "two-calls": m.Writer.NativeCalls = 2
		case "zero-payload": m.Writer.PayloadSHA256, m.ShiftedSHA256 = [32]byte{}, [32]byte{}
		case "wrong-payload": m.ShiftedSHA256[0]++
		case "baseline-payload": m.BaselineRangeSHA256 = m.Writer.PayloadSHA256
		case "zero-baseline-range": m.BaselineRangeSHA256 = [32]byte{}
		case "changed-tail": m.UntouchedTailSHA256[0]++
		case "zero-tails": m.BaselineTailSHA256, m.UntouchedTailSHA256 = [32]byte{}, [32]byte{}
		case "journal-missing": m.JournalOriginals[2] = [32]byte{}
		case "journal-wrong": m.JournalOriginals[1][0]++
		case "journal-reordered": m.JournalOriginals[1], m.JournalOriginals[2] = m.JournalOriginals[2], m.JournalOriginals[1]
		case "extra-close-cause": m.OnlyTargetCause = false
		case "NEW": bad.After = sqlSnapshot(bad.Identity, 102, 8, SQLNew)
		case "mixed-index": bad.After.Queries[SQLBlueIndex] = sqlSnapshot(bad.Identity, 102, 8, SQLNew).Queries[SQLBlueIndex]
		case "wrong-member": bad.After.Queries[SQLBlueIndex].Rows[0].ID = 77
		case "post-copy-NEW": bad.PostCommit = sqlSnapshot(bad.Identity, 103, 15, SQLNew)
		case "recovery-zero": bad.Recovery.Calls = 0
		case "recovery-two": bad.Recovery.Calls = 2
		case "wrong-limits": bad.Recovery.Limits.MaxJournalBytes++
		case "journal-before-absent": bad.Recovery.JournalPresentBefore = false
		case "journal-after-present": bad.Recovery.JournalPresentAfter = true
		case "owners-live": bad.Recovery.OwnersRetired = false
		case "replay-alias": bad.Artifacts[SQLReplayArtifact].Before.Inode = bad.Artifacts[SQLTrialArtifact].Before.Inode
		case "changed-original": bad.Originals.After[1][0]++
		case "changed-trial": bad.Artifacts[SQLTrialArtifact].After.SHA256[0]++
		case "replay-NEW": bad.Artifacts[SQLReplayArtifact].After.SHA256 = bad.Artifacts[SQLTrialArtifact].Before.SHA256
		case "public-success": bad.Lifecycle.PublicCodeIOErr = false
		case "public-uncertain": bad.Lifecycle.PublicCommitUncertain = true
		case "public-short": bad.Lifecycle.PublicShortWrite = true
		case "public-pending": bad.Lifecycle.PublicCleanupPending = true
		case "pre-DB-cause": bad.Lifecycle.PreDBOnlyTargetCause = true
		case "post-DB-cause": bad.Lifecycle.PostDBOnlyTargetCause = true
		case "two-reopens": bad.Lifecycle.ReopenCalls = 2
		case "physical": bad.PhysicalClaim = true
		}
		if observed := ObserveSQLLedger(plan, receipt, bad); observed.EvidenceConsistent || observed.ScenarioObserved || observed.Problem == SQLProblemNone || observed.Pass() || observed.PhysicalValidated() {
			t.Fatal("misplaced-write counterexample accepted", name, observed)
		}
	}
}

func TestSQLLedgerOldNineCasesRequireZeroMisplacement(t *testing.T) {
	for _, which := range []SQLCase{SQLControl, SQLIndexPrefix64, SQLDBSyncBefore, SQLIndexPrefix64Error, SQLJournalBodySyncBefore, SQLJournalHeaderSyncBefore, SQLPublishDirectorySyncBefore, SQLJournalUnlinkBefore, SQLFinalDirectorySyncBefore} {
		plan, receipt, evidence := sqlEvidenceFixture(t, which)
		if evidence.Misplacement != (SQLMisplacementEvidence{}) { t.Fatal("old case gained misplacement evidence", which) }
		for _, name := range []string{"requested-offset", "requested-bytes", "native-offset", "native-bytes", "native-calls", "payload", "shifted", "baseline-range", "baseline-tail", "tail", "journal", "cause"} {
			bad := evidence
			m := &bad.Misplacement
			switch name {
			case "requested-offset": m.Writer.RequestedOffset = 8192
			case "requested-bytes": m.Writer.RequestedBytes = 4096
			case "native-offset": m.Writer.NativeOffset = 8128
			case "native-bytes": m.Writer.NativeBytes = 4096
			case "native-calls": m.Writer.NativeCalls = 1
			case "payload": m.Writer.PayloadSHA256[0] = 44
			case "shifted": m.ShiftedSHA256[0] = 44
			case "baseline-range": m.BaselineRangeSHA256[0] = 55
			case "baseline-tail": m.BaselineTailSHA256[0] = 66
			case "tail": m.UntouchedTailSHA256[0] = 66
			case "journal": m.JournalOriginals[0][0] = 11
			case "cause": m.OnlyTargetCause = true
			}
			sqlMustReject(t, ObserveSQLLedger(plan, receipt, bad))
		}
	}
}
''';p.write_text(s)
print('Finite model positives, rejection cases and old-nine zero-evidence tests authored only.')
