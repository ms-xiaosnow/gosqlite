from pathlib import Path
R=Path('/workspace/scratch/5e06a74e5f65/restart-20261012T2033'); O=R/'misplaced-write-overlay'
p=O/'sqlite/rollback_fault_sql_manual_linux_test.go';s=p.read_text()
def edit(old,new,count=1):
 global s
 assert s.count(old)>=count,(old[:180],s.count(old));s=s.replace(old,new,count)
edit('\tdefault:\n\t\treturn fmt.Errorf("exactly one of nine fixed cases required")','\tcase "index-misplace-minus64-error":\n\t\tc.caseID = rollbackfault.SQLIndexMisplaceMinus64Error\n\tdefault:\n\t\treturn fmt.Errorf("exactly one of ten fixed cases required")')
# Keep the old public branches verbatim and add the one new cause condition.
edit('\t\t// Finish has checked errors.Is against the actual private target error.','''\t	if c.caseID == rollbackfault.SQLIndexMisplaceMinus64Error && (commitErr != publicError || !report.MisplaceOnlyTargetCause || errors.Is(commitErr, io.ErrShortWrite)) {
			return report, fmt.Errorf("INCOMPLETE: misplaced-write public cause or cleanup differs: %v", commitErr)
		}
		// Finish has checked errors.Is against the actual private target error.''')
# Pure, bounded helper makes no owner or recovery calls.
needle='func manualRollbackSQLRecoverCopy(ctx context.Context, replayPath string) (pager.RecoveryResult, error) {'
edit(needle,'''// Inspect only the already-read bounded buffers. These checks are separate
// from the successful journal-publication events in the generic receipt.
func manualRollbackSQLMisplacement(report pager.ManualRollbackSQLReport, baseline, post, journalBytes []byte) (rollbackfault.SQLMisplacementEvidence, error) {
	evidence := rollbackfault.SQLMisplacementEvidence{Writer: report.MisplacedWrite, OnlyTargetCause: report.MisplaceOnlyTargetCause}
	if len(baseline) != 12288 || len(post) != 12288 || len(journalBytes) != 16408 {
		return evidence, fmt.Errorf("misplaced-write exact retained extents required")
	}
	evidence.ShiftedSHA256 = sha256.Sum256(post[8128:12224])
	evidence.BaselineRangeSHA256 = sha256.Sum256(baseline[8128:12224])
	evidence.BaselineTailSHA256 = sha256.Sum256(baseline[12224:12288])
	evidence.UntouchedTailSHA256 = sha256.Sum256(post[12224:12288])
	writer := evidence.Writer
	if writer.RequestedOffset != 8192 || writer.RequestedBytes != 4096 || writer.NativeOffset != 8128 || writer.NativeBytes != 4096 || writer.NativeCalls != 1 || writer.PayloadSHA256 == ([32]byte{}) || writer.PayloadSHA256 != evidence.ShiftedSHA256 || evidence.ShiftedSHA256 == evidence.BaselineRangeSHA256 || !bytes.Equal(post[12224:12288], baseline[12224:12288]) || !evidence.OnlyTargetCause {
		return evidence, fmt.Errorf("misplaced-write actual argument/payload/tail/cause proof differs")
	}
	if report.EventCount != 35 || report.Events[30].Kind != rollbackfault.Returned || report.Events[30].Action != rollbackfault.MisplaceIndexPageMinus64 || report.Events[30].Call != (rollbackfault.Call{Point: rollbackfault.DBPage, Occurrence: 3, Role: rollbackfault.Database, Offset: 8192, Length: 4096, Owner: 11}) || report.Events[30].Result != (rollbackfault.Result{Executed: true, NativeN: 4096, ReturnedN: 4096, ReturnedError: rollbackfault.Injected, NormalizedError: rollbackfault.Injected, Effect: rollbackfault.EffectPossible}) {
		return evidence, fmt.Errorf("misplaced-write original requested call and native return differ")
	}
	plan, err := journal.ParseReplay(journalBytes, journal.ReplayLimits{MaxJournalBytes: 16408, MaxRecords: 3, MaxDatabaseBytes: 12288})
	if err != nil {
		return evidence, err
	}
	if plan.CountEvidence != journal.ReplayCountFiniteHeaders || plan.Header.Records != 3 || plan.Header.OriginalPages != 3 || plan.Header.PageSize != 4096 || plan.Header.SectorSize != 4096 || plan.DatabaseBytes != 12288 || len(plan.Pages) != 3 {
		return evidence, fmt.Errorf("misplaced-write complete finite journal inventory required")
	}
	if err := plan.ValidateExtent(12288); err != nil {
		return evidence, err
	}
	for i, page := range plan.Pages {
		if page.Number != uint32(i+1) || len(page.Data) != 4096 || !bytes.Equal(page.Data, baseline[i*4096:(i+1)*4096]) {
			return evidence, fmt.Errorf("misplaced-write journal original differs from independent baseline")
		}
		evidence.JournalOriginals[i] = sha256.Sum256(page.Data)
	}
	return evidence, nil
}

'''+needle)
edit('needsRecovery := c.caseID == rollbackfault.SQLIndexPrefix64 || c.caseID == rollbackfault.SQLDBSyncBefore || c.caseID == rollbackfault.SQLIndexPrefix64Error || c.caseID == rollbackfault.SQLJournalUnlinkBefore','needsRecovery := c.caseID == rollbackfault.SQLIndexPrefix64 || c.caseID == rollbackfault.SQLDBSyncBefore || c.caseID == rollbackfault.SQLIndexPrefix64Error || c.caseID == rollbackfault.SQLJournalUnlinkBefore || c.caseID == rollbackfault.SQLIndexMisplaceMinus64Error')
# A new-case-only early bounded read permits preserving writer, cause and byte
# proof in commit-observation before any failing dependent branch exits.
needle='\t// Retain the untouched public error text and all original diagnostics even'
edit(needle,'''\tmisplaced := c.caseID == rollbackfault.SQLIndexMisplaceMinus64Error
	var misplacement rollbackfault.SQLMisplacementEvidence
	var misplacementErr error
	var post, journalBytes []byte
	var postInfo os.FileInfo
	if misplaced {
		misplacement.Writer, misplacement.OnlyTargetCause = report.MisplacedWrite, report.MisplaceOnlyTargetCause
		post, postInfo, err = manualRollbackSQLRead(&c.ledger, filepath.Join(c.root, "trial.db"), 12288)
		misplacementErr = err
		if err == nil && (len(post) != 12288 || !os.SameFile(trialInfo, postInfo)) {
			misplacementErr = fmt.Errorf("misplaced-write post-Commit extent/identity changed")
		}
		var journalErr error
		journalBytes, _, journalErr = manualRollbackSQLRead(&c.ledger, filepath.Join(c.root, "trial.db-journal"), 16408)
		misplacementErr = errors.Join(misplacementErr, journalErr)
		if misplacementErr == nil {
			misplacement, misplacementErr = manualRollbackSQLMisplacement(report, baseline[:], post, journalBytes)
		}
	}
'''+needle)
edit('\tcommitRecord, marshalErr := json.MarshalIndent(struct {','\tmisplacementText := ""\n\tif misplacementErr != nil {\n\t\tmisplacementText = misplacementErr.Error()\n\t}\n\tcommitRecord, marshalErr := json.MarshalIndent(struct {')
edit('\t\tPostDBOnlyTargetCause bool\n\t}{identity, plan.Spec(), report.Observation, report.Events, report.EventCount, commitText, diagnosticTexts, report.PreDBOnlyTargetCause, report.PostDBOnlyTargetCause}, "", "  ")','\t\tPostDBOnlyTargetCause bool\n\t\tMisplacement rollbackfault.SQLMisplacementEvidence\n\t\tMisplacementError string\n\t}{identity, plan.Spec(), report.Observation, report.Events, report.EventCount, commitText, diagnosticTexts, report.PreDBOnlyTargetCause, report.PostDBOnlyTargetCause, misplacement, misplacementText}, "", "  ")')
edit('\treceipt, receiptProblem := rollbackfault.NewReceipt(plan)','\tif misplacementErr != nil {\n\t\tt.Fatalf("INCOMPLETE: misplaced-write byte/journal proof: %v; all inputs retained", misplacementErr)\n\t}\n\treceipt, receiptProblem := rollbackfault.NewReceipt(plan)')
edit('evidence := rollbackfault.SQLLedgerEvidence{Identity: identity, Case: c.caseID, Before: before}','evidence := rollbackfault.SQLLedgerEvidence{Identity: identity, Case: c.caseID, Before: before, Misplacement: misplacement}')
edit('\tpost, postInfo, err := manualRollbackSQLRead(&c.ledger, filepath.Join(c.root, "trial.db"), 12288)','\tif !misplaced {\n\t\tpost, postInfo, err = manualRollbackSQLRead(&c.ledger, filepath.Join(c.root, "trial.db"), 12288)\n\t}')
edit('\tvar journalBytes []byte\n','')
edit('\t\tjournalBytes, _, err = manualRollbackSQLRead(&c.ledger, filepath.Join(c.root, "trial.db-journal"), 16408)\n','\t\tif !misplaced {\n\t\t\tjournalBytes, _, err = manualRollbackSQLRead(&c.ledger, filepath.Join(c.root, "trial.db-journal"), 16408)\n\t\t}\n')
p.write_text(s)
print('Manual caller source updated; no execution.')
