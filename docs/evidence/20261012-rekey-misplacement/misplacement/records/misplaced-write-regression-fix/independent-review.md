# R2 observed regression correction: independent narrow review

2026-10-12 UTC. **CLEAR for the three-file correction and bounded V2 continuation.** No remaining blocking defect found in this correction. No production/native behavior changes are needed or requested.

The original normal log independently counts 2,413 named PASS events, 89 passing roots and two failing roots: `TestSQLLedgerCasePartitionsUseOnlyExactIDs` and `TestRollbackFaultSQLRepinnedSemanticCounterexamples`. The earlier static reviews missed these two concrete test-maintenance regressions. The original failure evidence remains valid and must be preserved rather than relabeled PASS.

## Correction verified

1. `internal/testkit/rollbackfault/sql_ledger_test.go` is exactly its saved before file with `|| value == 10` appended to the independently literal `wantRecovery` expression. The test still iterates every value 0–255, preserves exact pre-DB/post-DB partitions, and continues rejecting unknown/future IDs. This aligns the existing test with the deliberately admitted case 10; it does not widen production classification.
2. `internal/testscope/rollback_fault_sql_test.go` is exactly its saved before file with three old broad mutation anchors replaced by the entire `pager.RecoverRollback(ctx, replayPath, pager.RecoveryOptions{Limits: journal.ReplayLimits{MaxJournalBytes: 16408, MaxRecords: 3, MaxDatabaseBytes: 12288}})` call. That exact call occurs once in the unchanged native source. Each replacement changes only its original single target field: journal limit 16408 to 0, record limit 3 to 4, or database extent 12288 to 16384. Names, expected `recovery outside exact helper` diagnostic, double-repin logic and every other mutant are unchanged. The old broad substrings now first occur in the earlier ParseReplay helper, which explains the observed wrong-diagnostic failures. The unchanged recovery guard still binds the exact helper/path/call/limits and rejects these three corrections at the intended seam.
3. `internal/testscope/rollback_fault_sql.go` is exactly its saved before file with the one SQL model-test digest and that model-test's full source snapshot refreshed. The resulting snapshot exactly equals the corrected test source. All independent semantic decisions, native/source pins, other snapshots and expected diagnostics are unchanged.

All 13 current R2 path hashes match the correction identity. The ten other R2 files retain their prior exact hashes, including the model implementation, generic adapter, bridge and native SQL caller. Existing main-pin seams remain unchanged.

## Final identities

- Candidate tree recorded in the identity: `cdcce9e3145d7811b0448b4f02d21e71dbe32345`.
- `misplaced-write-regression-fix/SOURCE-IDENTITY.json`: `2855b329d20ae8ef7465681db160854985db870c6701364dc27ac4b834ee1160`.
- `internal/testkit/rollbackfault/sql_ledger_test.go`: `07083e68442522330edae5a562e74b2e6d901a9429c846b196737e02b5d7d469`.
- `internal/testscope/rollback_fault_sql_test.go`: `39457022d5a8de687a2418ba627f675d69fc0028d0d0ec953c3fbefe840cfbf0`.
- `internal/testscope/rollback_fault_sql.go`: `d5ff12fe8bb17e4976b17561762de95f0086fa927c601ee815e50dcf23e51468`.
- `run_misplaced_write_validation_v2.py`: `ddb2d3825e16d551783a4bb1f08b437d1eda14cef02a882f651323605bd6c27e`.

## Bounded continuation

Reviewed the V2 runner delta. It binds the corrected identity/release, writes a distinct evidence directory, checks formatting of only the three corrected files, runs exactly the two previously failed normal roots, and reuses the other 89 original normal roots as specified. The full original 91-root race selection remains, followed by the previously NOT_RUN sourceguard, three exact source-selection checks, three compile-only images and three nm checks. Result parsing switches to the correct expected package/root set for the two-root normal rerun. Queue/resource/identity controls, original execution flags, fixed tools and image-nonexecution boundary remain unchanged.

No broader rerun or new framework is proposed. The first FAIL, unchanged passing evidence, subsequent normal rerun, and still-pending gates must remain separately attributable. No new native/physical result is implied.

This reviewer performed source/text/hash and existing-log reads only and wrote this report. No W/source/pin/runner/release change, Go invocation, Go parser/AST execution, test, compiler, native image, Commit, recovery or controller was performed. V2 execution results remain pending.
