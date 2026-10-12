# R2 formatted overlay handoff

Status: formatting and source-text checks complete; all model/source-guard/test/compile/nm/native execution remains NOT_RUN in this worker. No W files were written.

## Identity
- Overlay: `/workspace/scratch/5e06a74e5f65/restart-20261012T2033/misplaced-write-overlay`
- Final path/SHA manifest: `/workspace/scratch/5e06a74e5f65/restart-20261012T2033/misplaced-write-format/final-path-sha256-manifest.json`
- Manifest SHA-256: `fae80cd29f7a027839d7429e954056b0956c6d7a772a5ae22c5d80f50d949a0e`
- Original 13 reviewed files and original source manifest: `/workspace/scratch/5e06a74e5f65/restart-20261012T2033/misplaced-write-format/before`
- Original independent review SHA-256: `facb10a83ac7f7007f0537e18179d4be71680a04d120f8dff71f706c8b11fcba`
- Complete review-to-final delta: `/workspace/scratch/5e06a74e5f65/restart-20261012T2033/misplaced-write-format/final-format-and-repin.diff`
- Base-01c-to-final patch: `/workspace/scratch/5e06a74e5f65/restart-20261012T2033/misplaced-write-format/final-overlay-from-01c.patch`

## Verified changes
- Fixed official gofmt SHA-256 5583dbb3339147fee7cdb067eb3cd4a50015b48c27084377f49d93833aa4ad77 was invoked exactly once with the eleven named overlay Go files under a successfully acquired nonblocking go-queue.lock. The lock was immediately released.
- Before repinning, all eleven files retained every protected literal and comment span and all non-whitespace text exactly. Only whitespace changed. This was a text comparison, not a second Go parser/scanner/AST or executable validation.
- The reviewed repin script, SHA-256 bbec39f0155e164359d1572ee2f3a7b3e51a3061205df1ec3c9b11ad4192f0b2, was run with the exact overlay path. Only seven owning pin/snapshot values were eligible to change. All seven final pins and full snapshots match exact overlay source bytes. Independent semantic literals, diagnostics, checks and mutant tables were not modified.
- No formatting-dependent anchor adaptation was needed. The exact three mutation tables have 290 entries: 33 generic, 24 new generic misplacement, and 233 SQL. All 290 anchors occur before and after formatting, with the same counts. The older source handoff reported 270; that count was stale.
- No second gofmt was needed: repinning replaced only already-quoted/raw-literal content and pin strings, preserving their formatted surrounding text.
- Both manuals are byte-identical to reviewed originals. All 13 W paths corresponding to this overlay remained byte-identical throughout this worker. The default/Commit/recovery/A5 excluded paths still match the source review; main.go differs only because it belongs to concurrent R5 work and was not changed by this worker.

## Required W seam before any tests
Use a three-way merge from base 01c63e6f8fb3d9e196afe2f42a163eb3eac4ad5a. Exactly two overlay paths overlap W changes:
- internal/testscope/rollback_fault_test.go
- internal/testscope/rollback_fault_sql_test.go
The overlay intentionally retains the old reviewedMainSHA256 literal 6dbab48ff374858bfafe578f0fca99f5176ced7b6bdc008bec04ce6a6d1d0eb2 in both files. W holds the independent R5 main-review updates. Preserve those W changes while adding the R2 edits. The current W delta for both paths is saved in w-overlap-diff-from-01c.patch. No whole-main pin was automatically advanced.

## Finite execution proposal for the lead; NOT_RUN here
Keep the existing fixed tool, single Go queue, CGO_ENABLED=0, readonly/offline module settings and finite flags. Run all tests only in ./internal/testkit/rollbackfault and ./internal/testkit/rollbackrestart. For ./internal/testscope select only the following 24 named roots; selected-testscope-run-selector.txt contains the exact anchored selector. This avoids executing unrelated R5 test roots, although package compilation still covers the package.

internal/testscope/rollback_fault_test.go
- TestRollbackFaultSourceContracts
- TestRollbackFaultRepinnedSemanticCounterexamples
- TestRollbackFaultOutsideConsumersRejected
- TestRollbackFaultGuardWiringPreservesPriorPolicies
- TestRollbackFaultMisplacementDoubleRepinnedCounterexamples

internal/testscope/rollback_fault_sql_test.go
- TestRollbackFaultSQLSourceContracts
- TestRollbackFaultSQLRepinnedSemanticCounterexamples
- TestRollbackFaultSQLOutsideConsumersAndOldEntriesRejected
- TestRollbackFaultSQLOrdinaryRecoveryExceptionIsOneReference
- TestRollbackFaultSQLWiringAndMainReviewHold
- TestRollbackFaultSQLOrdinaryExampleFields
- TestRollbackFaultSQLOrdinaryExamplesKeepModelBoundary
- TestRollbackFaultSQLOrdinaryFieldBindingsReject
- TestRollbackFaultSQLOrdinaryCompanionFailures

internal/testscope/rollback_restart_test.go
- TestRollbackRestartOwnedSourceContracts
- TestRollbackRestartDoubleRepinnedSelectionAndStubCounterexamples
- TestRollbackRestartOutsideConsumersAndAliasesRejected
- TestRollbackRestartOldCoreAndMutexWrapperStayExact
- TestRollbackRestartWiringPreservesAllPoliciesAndReviewHold
- TestRollbackRestartDoubleRepinnedNativeProtocolCounterexamples
- TestRollbackRestartDoubleRepinnedModelCausalCounterexamples
- TestRollbackRestartOracleExceptionRequiresCompletePinnedNativeClosure
- TestRollbackRestartDoubleRepinnedCommandCapabilityClosure
- TestRollbackRestartDoubleRepinnedLaunchInputClosure

Separately select exactly ^TestRollbackWriteFullAtMemoryContract$ in ./internal/pager, without manual tags. Any ordinary sourceguard invocation or the three previously reviewed compile-only/nm steps remains a separate lead action under its existing authorization. Never execute the manual images, including empty run selectors. No native miswrite/recovery or physical qualification is claimed.
