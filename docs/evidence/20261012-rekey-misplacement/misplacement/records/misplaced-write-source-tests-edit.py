from pathlib import Path
import json
O=Path('/workspace/scratch/5e06a74e5f65/restart-20261012T2033/misplaced-write-overlay')
p=O/'internal/testkit/rollbackfault/contract_test.go';s=p.read_text();old='plan, problem := SQLPlan(onePageSpec(Pass, 8).Identity, SQLIndexMisplaceMinus64Error)';assert s.count(old)==2;s=s.replace(old,'plan, problem := NewPlan(misplacedIndexSpec())')
helper='''// Independent literal plan: this fixture does not call the SQL plan builder.
func misplacedIndexSpec() PlanSpec {
	spec := PlanSpec{
		Identity: onePageSpec(Pass, 8).Identity,
		MainOwner: 11, JournalOwner: 12, DirectoryOwner: 13, CleanupOwner: 14,
		OriginalPages: 3, DirtyPages: 3, PageSize: 4096, SectorSize: 4096,
		Action: MisplaceIndexPageMinus64, ExpectedCommit: CommitRecoveryRequired, CallCount: 15,
		Calls: [MaxCalls]Call{
			{Point: JournalCreate, Occurrence: 1, Role: Journal, Owner: 12},
			{Point: JournalInvalid, Occurrence: 1, Role: Journal, Owner: 12, Length: 4096},
			{Point: DBOriginalRead, Occurrence: 1, Role: Database, Owner: 11, Offset: 0, Length: 4096},
			{Point: JournalRecord, Occurrence: 1, Role: Journal, Owner: 12, Offset: 4096, Length: 4104},
			{Point: DBOriginalRead, Occurrence: 2, Role: Database, Owner: 11, Offset: 4096, Length: 4096},
			{Point: JournalRecord, Occurrence: 2, Role: Journal, Owner: 12, Offset: 8200, Length: 4104},
			{Point: DBOriginalRead, Occurrence: 3, Role: Database, Owner: 11, Offset: 8192, Length: 4096},
			{Point: JournalRecord, Occurrence: 3, Role: Journal, Owner: 12, Offset: 12304, Length: 4104},
			{Point: JournalBodySync, Occurrence: 1, Role: Journal, Owner: 12},
			{Point: JournalValidHeader, Occurrence: 1, Role: Journal, Owner: 12, Length: 12},
			{Point: JournalHeaderSync, Occurrence: 1, Role: Journal, Owner: 12},
			{Point: PublishDirectorySync, Occurrence: 1, Role: Directory, Owner: 13},
			{Point: DBPage, Occurrence: 1, Role: Database, Owner: 11, Offset: 0, Length: 4096},
			{Point: DBPage, Occurrence: 2, Role: Database, Owner: 11, Offset: 4096, Length: 4096},
			{Point: DBPage, Occurrence: 3, Role: Database, Owner: 11, Offset: 8192, Length: 4096},
		},
	}
	spec.Target = spec.Calls[14]
	return spec
}

'''
s=s.replace('// The only additional action is a fixed actual placement, not a caller offset.',helper+'// The only additional action is a fixed actual placement, not a caller offset.');p.write_text(s)
# All new negatives double-repin supplementary snapshots and still need fixed
# semantic diagnostics. Strings contain source only and are never executed.
variants=[
 ('actual correct offset','rollbackFaultManual','w.file.WriteAt(b, nativeOffset)','w.file.WriteAt(b, off)','fixed misplaced-write WriteAt'),
 ('actual plus64','rollbackFaultManual','nativeOffset := off - 64','nativeOffset := off + 64','fixed misplaced-write WriteAt'),
 ('reported offset reconstructed from plan','rollbackFaultManual','s.misplacedWrite = w.misplaced','s.misplacedWrite = w.misplaced; s.misplacedWrite.NativeOffset = spec.Target.Offset - 64','fixed misplaced-write rollbackWrite'),
 ('actual 64-byte write','rollbackFaultManual','w.file.WriteAt(b, nativeOffset)','w.file.WriteAt(b[:64], nativeOffset)','fixed misplaced-write WriteAt'),
 ('native call fabricated','rollbackFaultManual','w.nativeN, w.nativeErr = w.file.WriteAt(b, nativeOffset)','w.nativeN, w.nativeErr = 4096, nil','fixed misplaced-write WriteAt'),
 ('native retry','rollbackFaultManual','w.nativeN, w.nativeErr = w.file.WriteAt(b, nativeOffset)','w.nativeN, w.nativeErr = w.file.WriteAt(b, nativeOffset); w.nativeN, w.nativeErr = w.file.WriteAt(b, nativeOffset)','fixed misplaced-write WriteAt'),
 ('actual payload hash forged','rollbackFaultManual','w.misplaced.PayloadSHA256 = sha256.Sum256(b)','w.misplaced.PayloadSHA256 = [32]byte{44}','fixed misplaced-write WriteAt'),
 ('native count forged','rollbackFaultManual','w.misplaced.NativeCalls++','w.misplaced.NativeCalls = 1','fixed misplaced-write WriteAt'),
 ('injection dropped','rollbackFaultManual','if w.nativeErr == nil && w.nativeN == 4096 {\n\t\t\tw.returnedErr = errRollbackFaultInjected','if w.nativeErr == nil && w.nativeN == 4096 {\n\t\t\tw.returnedErr = nil','fixed misplaced-write WriteAt'),
 ('native errno masked','rollbackFaultManual','if w.nativeErr == nil && w.nativeN == 4096 {','if w.nativeN == 4096 {','fixed misplaced-write WriteAt'),
 ('short native injection','rollbackFaultManual','if w.nativeErr == nil && w.nativeN == 4096 {','if w.nativeErr == nil {','fixed misplaced-write WriteAt'),
 ('wrong extent accepted','rollbackFaultManual','t.info.Size() != 12288','t.info.Size() < 12288','fixed misplaced-write armRollbackFault'),
 ('actual pages not exact','rollbackFaultManual','t.originalPages != 3 || t.header.PageCount != 3','t.originalPages > 3 || t.header.PageCount > 3','fixed misplaced-write armRollbackFault'),
 ('journal originals reordered admitted','rollbackFaultManual','originals[1] != 2 || originals[2] != 3','originals[1] != 3 || originals[2] != 2','fixed misplaced-write armRollbackFault'),
 ('index kind unchecked','rollbackFaultManual','t.dirty[3][0] != 10','false','fixed misplaced-write armRollbackFault'),
 ('requested call overwritten','rollbackFaultManual','s.misplacedWrite = w.misplaced','s.misplacedWrite = w.misplaced; ticket.call.Offset = w.misplaced.NativeOffset','fixed misplaced-write rollbackWrite'),
 ('second observation admitted','rollbackFaultManual','s.misplacedWrite != (rollbackfault.SQLMisplacedWrite{}) ||','false ||','fixed misplaced-write rollbackWrite'),
 ('used registry cleared','rollbackFaultManual','rollbackFaultRegistry.used = true','rollbackFaultRegistry.used = false','fixed misplaced-write armRollbackFault'),
 ('A5 after core order','rollbackFaultManual','rollbackFaultAfterCore(ticket, executed, nativeN, nativeErr, returnedN, returnedErr, normalizedErr, effect)\n\trollbackRestartAfter(ticket)','rollbackRestartAfter(ticket)\n\trollbackFaultAfterCore(ticket, executed, nativeN, nativeErr, returnedN, returnedErr, normalizedErr, effect)','fixed misplaced-write rollbackFaultAfter'),
 ('A5 core deferred unlock changed','rollbackFaultManual','defer s.mu.Unlock()\n\tresult := rollbackfault.Result','s.mu.Unlock()\n\tresult := rollbackfault.Result','fixed misplaced-write rollbackFaultAfterCore'),
 ('model arbitrary placement admission','rollbackFaultContract','spec.Action == MisplaceIndexPageMinus64 && !validMisplacedIndexPlan(spec)','false','fixed misplaced-write NewPlan'),
 ('model two dirty pages admitted','rollbackFaultContract','spec.DirtyPages != 3','spec.DirtyPages > 3','fixed misplaced-write validMisplacedIndexPlan'),
 ('model native original cause masked','rollbackFaultContract','result.ReturnedErrorToken != result.NativeErrorToken','false','fixed misplaced-write validResult'),
 ('model no injected result','rollbackFaultContract','result.NativeN != 4096 || result.ReturnedN != 4096 || result.ReturnedError != Injected','result.NativeN != 4096 || result.ReturnedN != 4096 || result.ReturnedError != None','fixed misplaced-write validResult'),
]
p=O/'internal/testscope/rollback_fault_test.go';s=p.read_text();s+='''
// Repin both layers. Only the independent closed semantic decision can reject.
func TestRollbackFaultMisplacementDoubleRepinnedCounterexamples(t *testing.T) {
	sources := rollbackFaultSources(t)
	variants := []struct{ name, path, old, replacement, finding string }{
'''
for name,path,old,new,finding in variants:
 s+='\t\t{'+json.dumps(name)+', '+path+', '+json.dumps(old)+', '+json.dumps(new)+', '+json.dumps(finding)+'},\n'
s+='''	}
	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			original := string(sources[variant.path])
			if !strings.Contains(original, variant.old) { t.Fatal("misplacement mutation anchor missing", variant.name) }
			changed := []byte(strings.Replace(original, variant.old, variant.replacement, 1))
			pin, shape := rollbackFaultPins[variant.path], rollbackFaultOwnedShapes[variant.path]
			rollbackFaultPins[variant.path] = fmt.Sprintf("%x", sha256.Sum256(changed))
			rollbackFaultOwnedShapes[variant.path] = string(changed)
			t.Cleanup(func() { rollbackFaultPins[variant.path], rollbackFaultOwnedShapes[variant.path] = pin, shape })
			findings, err := inspectRollbackFaultSource(variant.path, changed)
			if err != nil { t.Fatal("misplacement mutant must parse", err) }
			joined := strings.Join(findings, "\\n")
			if !strings.Contains(joined, variant.finding) || strings.Contains(joined, "complete-source pin changed") || strings.Contains(joined, "source binding/body/order changed") { t.Fatal("independent misplacement rejection missing", variant.name, findings) }
		})
	}
}
''';p.write_text(s)
# Add SQL mutants to the existing double-repinned table to preserve its harness.
variants=[
 ('misplacement helper post-hoc payload','rollbackFaultSQLNative','evidence := rollbackfault.SQLMisplacementEvidence{Writer: report.MisplacedWrite, OnlyTargetCause: report.MisplaceOnlyTargetCause}','evidence := rollbackfault.SQLMisplacementEvidence{Writer: report.MisplacedWrite, OnlyTargetCause: report.MisplaceOnlyTargetCause}; evidence.Writer.PayloadSHA256 = sha256.Sum256(post[8128:12224])','fixed misplaced-write payload, tail and journal originals'),
 ('misplacement shifted proof bypassed','rollbackFaultSQLNative','writer.PayloadSHA256 != evidence.ShiftedSHA256','false','fixed misplaced-write payload, tail and journal originals'),
 ('misplacement tail comparison bypassed','rollbackFaultSQLNative','!bytes.Equal(post[12224:12288], baseline[12224:12288])','false','fixed misplaced-write payload, tail and journal originals'),
 ('misplacement journal bytes bypassed','rollbackFaultSQLNative','!bytes.Equal(page.Data, baseline[i*4096:(i+1)*4096])','false','fixed misplaced-write payload, tail and journal originals'),
 ('misplacement EOF count admitted','rollbackFaultSQLNative','plan.CountEvidence != journal.ReplayCountFiniteHeaders','false','fixed misplaced-write payload, tail and journal originals'),
 ('misplacement parsed from post bytes','rollbackFaultSQLNative','journal.ParseReplay(journalBytes,','journal.ParseReplay(post,','fixed misplaced-write parser and extent call arguments'),
 ('misplacement extent not checked','rollbackFaultSQLNative','plan.ValidateExtent(12288)','plan.ValidateExtent(12352)','fixed misplaced-write parser and extent call arguments'),
 ('misplacement journal helper bypassed','rollbackFaultSQLNative','misplacement, misplacementErr = manualRollbackSQLMisplacement(report, baseline[:], post, journalBytes)','misplacementErr = nil','post-DB root exact one-shot'),
 ('misplacement proof copied from plan','rollbackFaultSQLNative','misplacement.Writer, misplacement.OnlyTargetCause = report.MisplacedWrite, report.MisplaceOnlyTargetCause','misplacement.Writer, misplacement.OnlyTargetCause = report.MisplacedWrite, report.MisplaceOnlyTargetCause; misplacement.Writer.NativeOffset = plan.Spec().Target.Offset - 64','post-DB root exact one-shot'),
 ('misplacement captured evidence lost','rollbackFaultSQLNative','Misplacement: misplacement','Misplacement: rollbackfault.SQLMisplacementEvidence{}','post-DB root exact one-shot'),
 ('misplacement public cause bypassed','rollbackFaultSQLNative','commitErr != publicError || !report.MisplaceOnlyTargetCause || errors.Is(commitErr, io.ErrShortWrite)','false','post-DB public Commit exact classification'),
 ('misplacement capture before owners retired','rollbackFaultSQLNative','if commitCheckErr == nil {','if true {','post-DB root exact one-shot'),
 ('misplacement bridge cause fabricated','rollbackFaultSQLBridge','misplaceOnlyTargetCause = manualRollbackSQLOnlyTargetCause(commitErr, poison, target, ErrRecoveryRequired)','misplaceOnlyTargetCause = true','post-DB Finish cannot bypass'),
 ('misplacement bridge writer fabricated','rollbackFaultSQLBridge','scope.scope.plan.Spec().Action, scope.scope.misplacedWrite','scope.scope.plan.Spec().Action, rollbackfault.SQLMisplacedWrite{}','post-DB Finish cannot bypass'),
 ('misplacement bridge wrong permitted cause','rollbackFaultSQLBridge','manualRollbackSQLOnlyTargetCause(commitErr, poison, target, ErrRecoveryRequired)','manualRollbackSQLOnlyTargetCause(commitErr, poison, target, ErrCommitUncertain)','post-DB Finish cannot bypass'),
 ('misplacement pre-DB predicate','rollbackFaultSQLModel','return which == SQLJournalBodySyncBefore ||','return which == SQLIndexMisplaceMinus64Error || which == SQLJournalBodySyncBefore ||','require closed predicates'),
 ('misplacement post-DB predicate','rollbackFaultSQLModel','return which == SQLJournalUnlinkBefore || which == SQLFinalDirectorySyncBefore','return which == SQLIndexMisplaceMinus64Error || which == SQLJournalUnlinkBefore || which == SQLFinalDirectorySyncBefore','require closed predicates'),
 ('misplacement replay removed','rollbackFaultSQLModel',' || which == SQLIndexMisplaceMinus64Error\n','\n','five post-write recovery cases'),
 ('misplacement NEW allowed','rollbackFaultSQLModel','if evidence.Case == SQLControl || evidence.Case == SQLFinalDirectorySyncBefore {','if evidence.Case == SQLControl || evidence.Case == SQLFinalDirectorySyncBefore || evidence.Case == SQLIndexMisplaceMinus64Error {','receipt identity and old/new mixtures'),
 ('misplacement actual evidence bypassed','rollbackFaultSQLModel','!validSQLMisplacement(evidence.Case, evidence.Misplacement, original.Frozen)','false','post-DB snapshot, actual receipt'),
 ('misplacement old-nine evidence allowed','rollbackFaultSQLModel','return evidence == (SQLMisplacementEvidence{})','return true','fixed misplaced-write argument/hash/OLD proof'),
 ('misplacement native offset unchecked','rollbackFaultSQLModel','writer.NativeOffset == 8128','true','fixed misplaced-write argument/hash/OLD proof'),
 ('misplacement native count unchecked','rollbackFaultSQLModel','writer.NativeCalls == 1','true','fixed misplaced-write argument/hash/OLD proof'),
 ('misplacement cause unchecked','rollbackFaultSQLModel','&& evidence.OnlyTargetCause','&& true','fixed misplaced-write argument/hash/OLD proof'),
 ('misplacement journal hashes unchecked','rollbackFaultSQLModel','evidence.JournalOriginals == originals','true','fixed misplaced-write argument/hash/OLD proof'),
]
p=O/'internal/testscope/rollback_fault_sql_test.go';s=p.read_text();anchor='\tvariants := []struct{ name, path, old, replacement, finding string }{\n';assert s.count(anchor)==1
addition=''.join('\t\t{'+json.dumps(name)+', '+path+', '+json.dumps(old)+', '+json.dumps(new)+', '+json.dumps(finding)+'},\n' for name,path,old,new,finding in variants)
s=s.replace(anchor,anchor+addition,1);p.write_text(s)
print('24 generic and 25 SQL independent source counterexamples authored; no parser execution.')
