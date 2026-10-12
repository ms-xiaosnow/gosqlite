from pathlib import Path
import shutil
R=Path('/workspace/scratch/5e06a74e5f65/restart-20261012T2033'); W=R/'gosqlite'; O=R/'misplaced-write-overlay'
files=['internal/testkit/rollbackfault/contract.go','internal/testkit/rollbackfault/contract_test.go','internal/testkit/rollbackfault/sql_ledger.go','internal/testkit/rollbackfault/sql_ledger_test.go','internal/pager/rollback_io_manual_linux.go','internal/pager/rollback_fault_sql_manual_linux.go','sqlite/rollback_fault_sql_manual_linux_test.go','internal/testscope/rollback_fault.go','internal/testscope/rollback_fault_test.go','internal/testscope/rollback_fault_sql.go','internal/testscope/rollback_fault_sql_test.go','docs/testing/rollback-fault-sql-manual-zh.md','docs/testing/rollback-fault-adapter-zh.md']
for f in files:
 p=O/f; p.parent.mkdir(parents=True,exist_ok=True); shutil.copyfile(W/f,p)
def edit(f, old, new, count=1):
 p=O/f; s=p.read_text(); assert s.count(old)>=count,(f,old[:150],s.count(old)); p.write_text(s.replace(old,new,count))
f=files[0]
edit(f,'\tPrefixShortWrite\n','\tPrefixShortWrite\n\tMisplaceIndexPageMinus64\n')
edit(f,'spec.Action > PrefixShortWrite','(spec.Action != Pass && spec.Action != FailBefore && spec.Action != PrefixShortWrite && spec.Action != MisplaceIndexPageMinus64)')
edit(f,'\tif spec.ExpectedCommit != expectedCommit(spec) {','\tif spec.Action == MisplaceIndexPageMinus64 && !validMisplacedIndexPlan(spec) {\n\t\treturn Plan{}, ProblemAction\n\t}\n\tif spec.ExpectedCommit != expectedCommit(spec) {')
edit(f,'func copyIdentity(id Identity) Identity {','''// This action is one closed three-page SQL geometry, never an offset API.
// OriginalPages counts journal images; the native armer checks actual extent.
func validMisplacedIndexPlan(spec PlanSpec) bool {
	if spec.OriginalPages != 3 || spec.DirtyPages != 3 || spec.PageSize != 4096 || spec.SectorSize != 4096 || spec.Prefix != 0 || spec.PrefixError || spec.CallCount != 15 || spec.ExpectedCommit != CommitRecoveryRequired || spec.Target != (Call{Point: DBPage, Occurrence: 3, Role: Database, Offset: 8192, Length: 4096, Owner: spec.MainOwner}) {
		return false
	}
	for i := 0; i < 3; i++ {
		if spec.Calls[2+2*i] != (Call{Point: DBOriginalRead, Occurrence: uint8(i + 1), Role: Database, Offset: int64(i) * 4096, Length: 4096, Owner: spec.MainOwner}) || spec.Calls[12+i] != (Call{Point: DBPage, Occurrence: uint8(i + 1), Role: Database, Offset: int64(i) * 4096, Length: 4096, Owner: spec.MainOwner}) {
			return false
		}
	}
	return true
}

func copyIdentity(id Identity) Identity {''')
needle='\tif action == PrefixShortWrite {\n\t\texposed, normalized := None, ShortWrite'
edit(f,needle,'''\tif action == MisplaceIndexPageMinus64 {
		if result.NativeN != 4096 || result.ReturnedN != 4096 || result.ReturnedError != Injected || result.NormalizedError != Injected || result.Effect != EffectPossible {
			return ProblemNativeResult
		}
		return ProblemNone
	}
'''+needle)
f=files[2]
edit(f,'\tSQLFinalDirectorySyncBefore\n','\tSQLFinalDirectorySyncBefore\n\tSQLIndexMisplaceMinus64Error\n')
edit(f,'// Counts are observations, not instructions to close or retry anything.','''// The writer supplies these real arguments before its one native invocation.
// The generic receipt still records the requested Call without alteration.
type SQLMisplacedWrite struct {
	RequestedOffset int64
	RequestedBytes  int
	NativeOffset    int64
	NativeBytes     int
	NativeCalls     uint8
	PayloadSHA256   [32]byte
}

// This fixed evidence is zero for every other case. Hashes remain supplied
// labels here; only the source-gated manual caller derives them from bytes.
type SQLMisplacementEvidence struct {
	Writer              SQLMisplacedWrite
	ShiftedSHA256       [32]byte
	BaselineRangeSHA256 [32]byte
	BaselineTailSHA256  [32]byte
	UntouchedTailSHA256 [32]byte
	JournalOriginals    [SQLPageCount][32]byte
	OnlyTargetCause     bool
}

// Counts are observations, not instructions to close or retry anything.''')
edit(f,'// open. Four fixed post-write failures call recovery;', '// open. Five fixed post-write failures call recovery;')
edit(f,'\tLifecycle     SQLLifecycleEvidence\n','\tLifecycle     SQLLifecycleEvidence\n\tMisplacement  SQLMisplacementEvidence\n')
edit(f,'\tSQLProblemArtifacts\n','\tSQLProblemArtifacts\n\tSQLProblemMisplacement\n')
edit(f,'\tcase SQLFinalDirectorySyncBefore:\n\t\tspec.Target, spec.Action = spec.Calls[17], FailBefore\n\t\tspec.ExpectedCommit, spec.CallCount = CommitUncertain, 18','\tcase SQLFinalDirectorySyncBefore:\n\t\tspec.Target, spec.Action = spec.Calls[17], FailBefore\n\t\tspec.ExpectedCommit, spec.CallCount = CommitUncertain, 18\n\tcase SQLIndexMisplaceMinus64Error:\n\t\tspec.Action = MisplaceIndexPageMinus64\n\t\tspec.ExpectedCommit, spec.CallCount = CommitRecoveryRequired, 15')
edit(f,'\tif !validSQLRecovery(evidence.Case, evidence.Recovery) {','\tif !validSQLMisplacement(evidence.Case, evidence.Misplacement, original.Frozen) {\n\t\tresult.Problem = SQLProblemMisplacement\n\t\treturn result\n\t}\n\tif !validSQLRecovery(evidence.Case, evidence.Recovery) {')
edit(f,'func validSQLReaderOwners(spec PlanSpec, before, after uint64) bool {','''func validSQLMisplacement(which SQLCase, evidence SQLMisplacementEvidence, originals [SQLPageCount][32]byte) bool {
	if which != SQLIndexMisplaceMinus64Error {
		return evidence == (SQLMisplacementEvidence{})
	}
	writer := evidence.Writer
	return writer.RequestedOffset == 8192 && writer.RequestedBytes == 4096 && writer.NativeOffset == 8128 && writer.NativeBytes == 4096 && writer.NativeCalls == 1 && writer.PayloadSHA256 != ([32]byte{}) && writer.PayloadSHA256 == evidence.ShiftedSHA256 && evidence.BaselineRangeSHA256 != ([32]byte{}) && evidence.ShiftedSHA256 != evidence.BaselineRangeSHA256 && evidence.BaselineTailSHA256 != ([32]byte{}) && evidence.UntouchedTailSHA256 == evidence.BaselineTailSHA256 && evidence.JournalOriginals == originals && evidence.OnlyTargetCause
}

func validSQLReaderOwners(spec PlanSpec, before, after uint64) bool {''')
edit(f,'return which == SQLIndexPrefix64 || which == SQLDBSyncBefore || which == SQLIndexPrefix64Error || which == SQLJournalUnlinkBefore','return which == SQLIndexPrefix64 || which == SQLDBSyncBefore || which == SQLIndexPrefix64Error || which == SQLJournalUnlinkBefore || which == SQLIndexMisplaceMinus64Error')
f=files[4]
edit(f,'import (\n','import (\n\t"crypto/sha256"\n')
edit(f,'\ttargetErrorToken uint64\n','\ttargetErrorToken uint64\n\tmisplacedWrite   rollbackfault.SQLMisplacedWrite\n')
edit(f,'\t// Bind the frozen manifest to this Tx\'s exact page order, not a similar path.','''\tif spec.Action == rollbackfault.MisplaceIndexPageMinus64 {
		if t.info.Size() != 12288 || t.originalPages != 3 || t.header.PageCount != 3 || len(dirty) != 3 || dirty[0] != 1 || dirty[1] != 2 || dirty[2] != 3 || len(originals) != 3 || originals[0] != 1 || originals[1] != 2 || originals[2] != 3 || len(t.dirty[2]) != 4096 || len(t.dirty[3]) != 4096 || t.dirty[1][100] != 13 || t.dirty[2][0] != 13 || t.dirty[3][0] != 10 {
			return nil, errRollbackFaultScope
		}
	}
	// Bind the frozen manifest to this Tx's exact page order, not a similar path.''')
edit(f,'\treturnedErr error\n','\treturnedErr error\n\tmisplaced   rollbackfault.SQLMisplacedWrite\n')
edit(f,'func (w *rollbackFaultWriter) WriteAt(b []byte, off int64) (int, error) {','''func (w *rollbackFaultWriter) WriteAt(b []byte, off int64) (int, error) {
	if w.action == rollbackfault.MisplaceIndexPageMinus64 {
		if off != 8192 || len(b) != 4096 || w.misplaced != (rollbackfault.SQLMisplacedWrite{}) {
			w.returnedErr = errRollbackFaultScope
			return 0, w.returnedErr
		}
		nativeOffset := off - 64
		w.misplaced.RequestedOffset, w.misplaced.RequestedBytes = off, len(b)
		w.misplaced.NativeOffset, w.misplaced.NativeBytes = nativeOffset, len(b)
		w.misplaced.PayloadSHA256 = sha256.Sum256(b)
		w.executed = true
		w.misplaced.NativeCalls++
		w.nativeN, w.nativeErr = w.file.WriteAt(b, nativeOffset)
		w.returnedN, w.returnedErr = w.nativeN, w.nativeErr
		if w.nativeErr == nil && w.nativeN == 4096 {
			w.returnedErr = errRollbackFaultInjected
		}
		return w.returnedN, w.returnedErr
	}''')
edit(f,'\trollbackFaultAfter(ticket, w.executed, w.nativeN, w.nativeErr, w.returnedN, w.returnedErr, err, effect)','''\tif ticket.action == rollbackfault.MisplaceIndexPageMinus64 {
		s := ticket.scope
		s.mu.Lock()
		if s.misplacedWrite != (rollbackfault.SQLMisplacedWrite{}) || !s.inFlight || ticket.call != spec.Target || ticket.call.Point != rollbackfault.DBPage || ticket.call.Occurrence != 3 || ticket.call.Owner != spec.MainOwner || ticket.call.Role != rollbackfault.Database || ticket.call.Offset != 8192 || ticket.call.Length != 4096 || w.misplaced.RequestedOffset != ticket.call.Offset || w.misplaced.RequestedBytes != ticket.call.Length || w.misplaced.NativeOffset != 8128 || w.misplaced.NativeBytes != 4096 || w.misplaced.NativeCalls != 1 {
			s.problem = errRollbackFaultScope
		} else {
			s.misplacedWrite = w.misplaced
		}
		s.mu.Unlock()
	}
	rollbackFaultAfter(ticket, w.executed, w.nativeN, w.nativeErr, w.returnedN, w.returnedErr, err, effect)''')
f=files[5]
edit(f,'\tPostDBOnlyTargetCause bool\n','\tPostDBOnlyTargetCause bool\n\tMisplacedWrite        rollbackfault.SQLMisplacedWrite\n\tMisplaceOnlyTargetCause bool\n')
edit(f,'\tcase rollbackfault.FailBefore:\n','\tcase rollbackfault.MisplaceIndexPageMinus64:\n\t\tcaseID = rollbackfault.SQLIndexMisplaceMinus64Error\n\tcase rollbackfault.FailBefore:\n')
edit(f,'\ttarget, point := scope.scope.targetError, scope.scope.plan.Spec().Target.Point\n','\ttarget, point := scope.scope.targetError, scope.scope.plan.Spec().Target.Point\n\taction, misplaced := scope.scope.plan.Spec().Action, scope.scope.misplacedWrite\n')
needle='\treturn ManualRollbackSQLReport{Observation: report.Observation, Events: report.Events, EventCount: report.EventCount, Diagnostics: report.Diagnostics, CommitError: report.CommitError, Problem: report.Problem, PreDBOnlyTargetCause: preDBOnlyTargetCause, PostDBOnlyTargetCause: postDBOnlyTargetCause}'
edit(f,needle,'''\tmisplaceOnlyTargetCause := false
	if action == rollbackfault.MisplaceIndexPageMinus64 {
		scope.scope.tx.mu.Lock()
		poison := scope.scope.tx.poisoned
		scope.scope.tx.mu.Unlock()
		misplaceOnlyTargetCause = manualRollbackSQLOnlyTargetCause(commitErr, poison, target, ErrRecoveryRequired)
	}
'''+needle[:-1]+', MisplacedWrite: misplaced, MisplaceOnlyTargetCause: misplaceOnlyTargetCause}')
print('Created 13-file source overlay; first five implementation edits written.')
