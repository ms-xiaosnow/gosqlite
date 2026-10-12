from pathlib import Path
import re,json
O=Path('/workspace/scratch/5e06a74e5f65/restart-20261012T2033/misplaced-write-overlay')
def source(f):return (O/f).read_text()
def fn(f,name):
 s=source(f);m=re.search(r'^func (?:\([^\n]+?\) )?'+re.escape(name)+r'\([^\n]*',s,re.M);assert m,(f,name)
 # Every selected declaration ends at an unindented close brace.
 end=s.index('\n}',m.start())+2
 return s[m.start():end]
# Add closed function decisions independently of complete-source snapshots.
p=O/'internal/testscope/rollback_fault.go';s=p.read_text();anchor='\tif path == rollbackFaultPoints {\n'; pos=s.index(anchor,s.index('func inspectRollbackFaultOwned'))
block='\tfunctions := readOnlyWALManualFunctions(file)\n'
for var,file,names in [
 ('rollbackFaultManual','internal/pager/rollback_io_manual_linux.go',['armRollbackFault','WriteAt','rollbackWrite','disarmRollbackFault','rollbackFaultBefore','rollbackFaultBeforeCore','rollbackFaultAfter','rollbackFaultAfterCore']),
 ('rollbackFaultContract','internal/testkit/rollbackfault/contract.go',['NewPlan','validMisplacedIndexPlan','validResult'])]:
 block+='\tif path == '+var+' {\n'
 for name in names:
  block+='\t\tfindings = append(findings, rollbackFaultSQLExactFunction(path, functions, '+json.dumps(name)+', '+json.dumps('fixed misplaced-write '+name+' semantics')+', `'+fn(file,name)+'`)... )\n'
 block+='\t}\n'
s=s[:pos]+block+s[pos:];p.write_text(s)
# SQL semantic expectations: old case branches are retained; append fixed tenth.
p=O/'internal/testscope/rollback_fault_sql.go';s=p.read_text()
# Do not edit huge stored snapshots yet. They are updated independently below.
head,sep,tail=s.partition('// These closed semantic decisions')
s=tail
s=s.replace('nine-case','ten-case').replace('nine exact','ten exact').replace('nine fixed','ten fixed').replace('through 9','through 10').replace('four recovery','five recovery').replace('four post-write','five post-write')
s=s.replace('case rollbackfault.FailBefore: switch spec.Target.Point', 'case rollbackfault.MisplaceIndexPageMinus64: caseID = rollbackfault.SQLIndexMisplaceMinus64Error; case rollbackfault.FailBefore: switch spec.Target.Point')
s=s.replace('PostDBOnlyTargetCause:postDBOnlyTargetCause}`','PostDBOnlyTargetCause:postDBOnlyTargetCause,MisplacedWrite:misplaced,MisplaceOnlyTargetCause:misplaceOnlyTargetCause}`')
s=s.replace('PostDBOnlyTargetCause bool }`)', 'PostDBOnlyTargetCause bool; MisplacedWrite rollbackfault.SQLMisplacedWrite; MisplaceOnlyTargetCause bool }`)',1)
s=s.replace('== "manualRollbackSQLOnlyTargetCause(commitErr, poison, target, permitted)"))','== "manualRollbackSQLOnlyTargetCause(commitErr, poison, target, permitted)" || rollbackFaultText(set, raw, call) == "manualRollbackSQLOnlyTargetCause(commitErr, poison, target, ErrRecoveryRequired)"))')
s=s.replace('CountCall(file, "manualRollbackSQLOnlyTargetCause") != 2', 'CountCall(file, "manualRollbackSQLOnlyTargetCause") != 3').replace('pre/post-DB cause observer has two exact calls','pre/post-DB and misplacement cause observer has three exact calls')
s=s.replace('case "final-dirsync-before": c.caseID = rollbackfault.SQLFinalDirectorySyncBefore; default:', 'case "final-dirsync-before": c.caseID = rollbackfault.SQLFinalDirectorySyncBefore; case "index-misplace-minus64-error": c.caseID = rollbackfault.SQLIndexMisplaceMinus64Error; default:')
s=s.replace('case SQLFinalDirectorySyncBefore: spec.Target,spec.Action = spec.Calls[17],FailBefore; spec.ExpectedCommit,spec.CallCount = CommitUncertain,18; default:', 'case SQLFinalDirectorySyncBefore: spec.Target,spec.Action = spec.Calls[17],FailBefore; spec.ExpectedCommit,spec.CallCount = CommitUncertain,18; case SQLIndexMisplaceMinus64Error: spec.Action = MisplaceIndexPageMinus64; spec.ExpectedCommit,spec.CallCount = CommitRecoveryRequired,15; default:')
s=s.replace('SQLJournalUnlinkBefore; SQLFinalDirectorySyncBefore )','SQLJournalUnlinkBefore; SQLFinalDirectorySyncBefore; SQLIndexMisplaceMinus64Error )')
s=s.replace('Recovery SQLRecoveryEvidence; Lifecycle SQLLifecycleEvidence; PhysicalClaim bool','Recovery SQLRecoveryEvidence; Lifecycle SQLLifecycleEvidence; Misplacement SQLMisplacementEvidence; PhysicalClaim bool')
s=s.replace('which == SQLIndexPrefix64 || which == SQLDBSyncBefore || which == SQLIndexPrefix64Error || which == SQLJournalUnlinkBefore', 'which == SQLIndexPrefix64 || which == SQLDBSyncBefore || which == SQLIndexPrefix64Error || which == SQLJournalUnlinkBefore || which == SQLIndexMisplaceMinus64Error')
s=s.replace('c.caseID == rollbackfault.SQLIndexPrefix64 || c.caseID == rollbackfault.SQLDBSyncBefore || c.caseID == rollbackfault.SQLIndexPrefix64Error || c.caseID == rollbackfault.SQLJournalUnlinkBefore', 'c.caseID == rollbackfault.SQLIndexPrefix64 || c.caseID == rollbackfault.SQLDBSyncBefore || c.caseID == rollbackfault.SQLIndexPrefix64Error || c.caseID == rollbackfault.SQLJournalUnlinkBefore || c.caseID == rollbackfault.SQLIndexMisplaceMinus64Error')
s=s.replace('report.PreDBOnlyTargetCause,report.PostDBOnlyTargetCause},"","  ")','report.PreDBOnlyTargetCause,report.PostDBOnlyTargetCause,misplacement,misplacementText},"","  ")')
# Replace fixed semantic function literals, retaining all diagnostics for old mutations.
for file,name in [
 ('internal/pager/rollback_fault_sql_manual_linux.go','FinishManualRollbackSQLFault'),
 ('sqlite/rollback_fault_sql_manual_linux_test.go','manualRollbackSQLCommit'),
 ('sqlite/rollback_fault_sql_manual_linux_test.go','TestManualRollbackFaultSQLLedger'),
 ('internal/testkit/rollbackfault/sql_ledger.go','SQLPlan'),
 ('internal/testkit/rollbackfault/sql_ledger.go','ObserveSQLLedger')]:
 pattern=r'(`func '+re.escape(name)+r'\([^`]*?)(`\)\.\.\.\))'
 s,n=re.subn(pattern,lambda m:'`'+fn(file,name)+m.group(2),s);assert n==1,(name,n)
# Independently pin the pure byte/journal helper and restrict its only invocation.
marker='\troot := readOnlyWALManualBody(functions, rollbackFaultSQLRoot)'
extra='\tfindings = append(findings, rollbackFaultSQLExactFunction(path, functions, "manualRollbackSQLMisplacement", "fixed misplaced-write payload, tail and journal originals", `'+fn('sqlite/rollback_fault_sql_manual_linux_test.go','manualRollbackSQLMisplacement')+'`)... )\n'
s=s.replace(marker,extra+marker,1)
# Narrow parser/extent and helper call-site checks; no global parser exemption.
marker='\t\tif path == rollbackFaultSQLNative {\n\t\t\tif qualifier, ok := sel.X.(*ast.Ident); ok && qualifier.Name == "syscall"'
extra='''\t	if path == rollbackFaultSQLNative && (sel.Sel.Name == "ParseReplay" || sel.Sel.Name == "ValidateExtent") {
			want := "journal.ParseReplay(journalBytes, journal.ReplayLimits{MaxJournalBytes: 16408, MaxRecords: 3, MaxDatabaseBytes: 12288})"
			if sel.Sel.Name == "ValidateExtent" { want = "plan.ValidateExtent(12288)" }
			if !called || fn == nil || fn.Name.Name != "manualRollbackSQLMisplacement" || rollbackFaultText(set, raw, call) != want {
				report("fixed misplaced-write parser and extent call arguments")
			}
		}
'''
assert marker in s;s=s.replace(marker,extra+marker,1)
marker='\t\t\tif id.Name == "manualRollbackSQLOnlyTargetCause" {'
extra='''\t		if id.Name == "manualRollbackSQLMisplacement" {
				declaration, declares := parents[id].(*ast.FuncDecl)
				call := rollbackFaultDirectReference(id, parents)
				fn := oracleEnclosingFunction(id, parents)
				if !(declares && declaration.Name == id && path == rollbackFaultSQLNative && declaration.Recv == nil) && !(path == rollbackFaultSQLNative && fn != nil && fn.Name.Name == rollbackFaultSQLRoot && call != nil && rollbackFaultText(set, raw, call) == "manualRollbackSQLMisplacement(report, baseline[:], post, journalBytes)") {
					report("fixed misplaced-write helper only from original captured buffers")
				}
			}
'''
assert marker in s;s=s.replace(marker,extra+marker,1)
marker='\tif path == rollbackFaultSQLNative {\n\t\tif commitReferences != 1 {'
extra='''\tif path == rollbackFaultSQLNative {
		if readOnlyWALManualCountCall(file, "manualRollbackSQLMisplacement") != 1 || readOnlyWALManualCountCall(file, "journal.ParseReplay") != 1 || readOnlyWALManualCountCall(file, "plan.ValidateExtent") != 1 {
			report("one fixed misplaced-write byte/journal proof required")
		}
	}
'''
assert marker in s;s=s.replace(marker,extra+marker,1)
# Model fixed evidence declarations and decision independent of snapshots.
marker='\tcaseDeclarations, lifecycleDeclarations, ledgerDeclarations := 0, 0, 0'
extra='\tfindings = append(findings, rollbackFaultSQLExactFunction(path, functions, "validSQLMisplacement", "fixed misplaced-write argument/hash/OLD proof and old-nine zero evidence", `'+fn('internal/testkit/rollbackfault/sql_ledger.go','validSQLMisplacement')+'`)... )\n'
s=s.replace(marker,extra+marker,1)
# Put narrow explicit field checks alongside existing scalar constraints.
marker='\t\t\tif typ, ok := spec.(*ast.TypeSpec); ok && typ.Name.Name == "SQLLedgerEvidence" {'
extra='''\t		if typ, ok := spec.(*ast.TypeSpec); ok && typ.Name.Name == "SQLMisplacedWrite" {
				if readOnlyWALManualCompact(typ) != readOnlyWALManualCompactText(`SQLMisplacedWrite struct { RequestedOffset int64; RequestedBytes int; NativeOffset int64; NativeBytes int; NativeCalls uint8; PayloadSHA256 [32]byte }`) {
					report("fixed misplaced-write actual writer record")
				}
			}
			if typ, ok := spec.(*ast.TypeSpec); ok && typ.Name.Name == "SQLMisplacementEvidence" {
				if readOnlyWALManualCompact(typ) != readOnlyWALManualCompactText(`SQLMisplacementEvidence struct { Writer SQLMisplacedWrite; ShiftedSHA256 [32]byte; BaselineRangeSHA256 [32]byte; BaselineTailSHA256 [32]byte; UntouchedTailSHA256 [32]byte; JournalOriginals [SQLPageCount][32]byte; OnlyTargetCause bool }`) {
					report("fixed misplaced-write closed SQL evidence")
				}
			}
'''
assert marker in s;s=s.replace(marker,extra+marker,1)
s=head+sep+s;p.write_text(s)
# Existing mutation diagnostics are renamed, never dropped. Changed anchors are
# moved to equivalent current source fragments while preserving the old defect.
p=O/'internal/testscope/rollback_fault_sql_test.go';s=p.read_text();s=s.replace('nine-case','ten-case').replace('nine exact','ten exact').replace('four post-write','five post-write')
s=s.replace('" || which == SQLJournalUnlinkBefore\\n", "\\n"','" || which == SQLJournalUnlinkBefore || which == SQLIndexMisplaceMinus64Error\\n", " || which == SQLIndexMisplaceMinus64Error\\n"')
s=s.replace('"post, postInfo, err := manualRollbackSQLRead", "evidence.Lifecycle.PostDBOnlyTargetCause = true; post, postInfo, err := manualRollbackSQLRead"','"evidence.Lifecycle.PostDBOnlyTargetCause = report.PostDBOnlyTargetCause", "evidence.Lifecycle.PostDBOnlyTargetCause = report.PostDBOnlyTargetCause; evidence.Lifecycle.PostDBOnlyTargetCause = true"')
p.write_text(s)
print('Independent fixed semantic checks authored; whole-source pins and snapshots not yet refreshed.')
