# Independent source review: publication errno model overlay

Review time: 2026-10-12 UTC.

## Verdict

**Code: CLEAR_SOURCE_ONLY. Evidence/selection blocker: ERRNO_MODEL_SELECTOR_INCOMPLETE.**

The two proposed test-file changes are source-clear for their eight finite inert errno cases. Their state, visibility, uncertainty, cause, no-replay, and modeled retirement assertions agree with the actual production facades. No production defect or new native execution edge was found.

Before claiming a complete affected validation closure, correct the manifest and source report: they omit `TestBackupDestinationModelAdmissionBlocksMutation` in `internal/vfs/backup_namespace_model_test.go:117`. That existing test constructs the changed `backupPublicationModel` and reaches its changed `event` helper through `openDestinationLocked -> openDirectory -> event`. Ordinary and race selectors therefore require **20 roots**, not 19. This omitted root is itself inert and source-clear. No source edit to the proposed tests is needed for this finding.

The other integration prerequisite remains unchanged: refresh only the WAL model entry in `walPublicationFrozenSources` after integrating and, if authorized, formatting the reviewed source. Its whole-file pin must match the final bytes. This review does not change that guard or authorize native/physical qualification.

## Review boundaries and identities

Repository (`W`): `/workspace/scratch/5e06a74e5f65/restart-20261012T2033/gosqlite`.

Overlay: `errno-model-overlay`, exactly these two existing test files:

- `internal/vfs/wal_publication_model_test.go`
- `internal/vfs/backup_publication_model_test.go`

Independently read the source report, manifest, patch, both whole overlay files, production facades/diagnostic helpers, native ownership-transfer source, preserved-model helper closure, source-contract helpers, and the 11 WAL guard roots. Ordinary source reads, text searches/diffs, and SHA-256 identity checks were used. **No Go command, gofmt, compiler, parser executable, test, native publication, fault operation, or change to W was performed.** This report is the only reviewer-created file.

Verified SHA-256 values:

- Base WAL model: `7381288113bc424421bf8dade2f3a4e75bfd3b46f3201b3468762341a54ad9e7`
- Base backup model: `fd2fa131a8df925d457c94fc4c6326eca03ab773c51cce29ff55db2938093989`
- Overlay WAL model: `e39f22ae87e6d70d3fc716bf9615ef862acea555a4cc7a9a10220c66c61f5c6b`
- Overlay backup model: `bf4e458cf27fd564f80d2d15b9f45bbcde6f18c9524a14da4302ca36bfb99380`
- Patch: `55f113d2c5b27977f70a61023438719dc90283458c9989eae0507cb113ce2b2c`

All five match the submitted manifest. Direct base-to-overlay diffs confirm that all nine prior WAL and four prior backup test bodies remain byte-for-byte unchanged. WAL helper changes only append inert consumed identities after the existing event call. `event` does not modify the completion flags, so the new evaluation order preserves old return outcomes. Backup `failErr == nil` preserves the old generic failure behavior.

## Assertion review

### WAL: four cases

All cases successfully acquire the fake owner, create the candidate, and read the source before arming the error. They call the real `WALPublication` transitions via `newWALPublicationWithOps(m)`.

- Partial write ENOSPC: 16 of 32 bytes are retained, `io.ErrShortWrite` and ENOSPC survive together, `prepareAttempted` remains false, and no publication attempt occurs.
- Candidate-Sync ENOSPC: all 32 bytes are present; prepare is attempted but not completed; replacement and directory barrier are not attempted.
- Replace EACCES: copy, candidate Sync, and the final recheck have succeeded. Replacement is attempted but not known complete. No transfer or post-publication observation occurs; exactly one protective directory barrier occurs. A successful barrier cannot clear the prior failure. `replaced == false` is absence of a known-success fact, not proof of physical invisibility.
- Directory-Sync ENOSPC: replacement and owner transfer are known complete; post-publication observation occurs; the failed barrier prevents `Durable` before and after retirement.

The expected event arrays match the actual placement of both `Prepare` rechecks. `errors.Is` verifies the exact errno and `errors.As` verifies the retained `os.SyscallError` operation and cause. All four expected failures are nonterminal safe diagnostic graphs (`failed`, not `terminal`, with no `unsafe` graph).

Removing the injected error does not enable any retried write, prepare, replace, transfer, or finish call; the event list stays unchanged. Before cleanup the retirement identity list is empty. Cleanup retires exactly candidate then directory before transfer, or original/superseded WAL then directory after transfer. The borrowed main and transferred current WAL are excluded. The second `Close` is event-free and cannot turn failure into durability.

The inert identity recorder matches `nativeWALPublication.transfer` assigning `n.child` to the original WAL, followed by `seal` touching only child/directory, and `closeChild`/`closeDirectory` retiring those two leases. These native methods were inspected as source only and are outside the selected call closure.

### Backup: four cases

The fake admitted owner successfully creates and locks its candidate before arming write ENOSPC, file-Sync ENOSPC, rename EPERM, or directory-Sync ENOSPC. Real `BackupPublication` methods perform all transitions.

- Write failure: `CopyComplete`, data-Sync, publication, and directory-Sync facts remain false.
- File-Sync failure: copy is complete and data Sync attempted, but `DataSynced` and later facts remain false.
- Rename failure: data is synced and publication attempted; `Published` and directory-Sync facts remain false. The fake returns its error before setting visibility. This models that finite outcome, not a universal claim that a real failed rename cannot have changed visibility.
- Directory-Sync failure: `Published` and `DirectorySyncAttempted` stay true; `DirectorySynced` remains false.

Exact public status equality covers every status field, including the intentionally false fields. Before cleanup, modeled main/directory ownership, exclusive protector, and lifetime pin remain represented. After cleanup only `Sealed` and `Completed` change in the status: modeled visibility persists, both owners and lock/pin are gone, and the exact additional events are `seal`, `close-directory`, `close-main`, once each. Retried transitions produce `ErrPoisoned` without events. `SuccessfulPublication` remains unsuccessful after retirement. Cause checks correctly establish the operation error returned to the caller; they do not imply that the facade stores that original errno internally.

## Static executable call closure

The following is the application-level selected-root closure, not a claim that the Go runtime, race runtime, or ordinary source-file reads have no operating-system activity.

1. WAL model roots -> `newWALPublicationModel`, `walPublicationModelPrepared`, event/count/terminal-graph helpers and inert error/info types -> `newWALPublicationWithOps` and real `Acquire`, `CreateCandidate`, `Source`/`walPublicationSource.ReadAt`, `WriteAt`, `Prepare`, `Replace`, `TransferWAL`, `FinishPublication`, `Durable`, `Close`, `RetirementComplete` -> `usable`, `failure`, `walPublicationError` -> only the bound `walPublicationModel` ops. Model ops use byte slices, `bytes.Reader`, maps, identity pointers, and event arrays. `File` values are never receivers of `File` methods.
2. Preserved WAL namespace roots also reach `walPublicationDirectChild`, `walPublicationDistinct`, `walPublicationBasename`, and the inert fact methods. Preserved diagnostic roots reach `retainedFileCleanup`, `CleanupOwnersBounded`, and inert graph wrappers. Discovery does not invoke owner `Close` or `RetirementComplete`.
3. Backup publication roots -> `backupModelOwner`/`backupModelCopy` and the inert model methods -> real `CreatePrivateCandidate`, `WritePage`, `SyncData`, `PublishNoReplace`, `SyncPublication`, `Status`, `SealForRetirement`, `Close`, `RetirementComplete`, `SuccessfulPublication` -> `active`, `fail`, `diagnostic`, `sealLocked`, `backupError` -> only bound `backupPublicationModel` ops.
4. Preserved acquisition root and the omitted admission-blocking root also reach `openDestinationLocked`, `backupDestinationAdmission`, `BackupBasename`, `backupFamiliesDisjoint`, inert `backupFact` methods, and `backupModelDirectory` from `backup_namespace_model_test.go`. No call to `OpenBackupDestination` occurs, so its `rand.Read` route is not reached. Preserved diagnostic checks use `retainedFileCleanup`, `terminalCleanup`, `CleanupOwnersBounded`, and inert error types.
5. Four VFS source-contract roots -> their in-file `walPublicationAST*`/`walPublicationSource*` and `backupAST*` helpers -> fixed `os.ReadFile` inputs, Go AST/parser/printer/format library functions, bytes/string operations, and assertions. Native source is data, not invoked code. These roots do not execute a compiler, subprocess, parsed program, or native constructor.
6. Eleven WAL guard roots -> `inspectWALPublicationSource`, frozen/production/elapsed checks and their in-file source-shape helpers -> `oracleParents`, `oracleEnclosingFunction`, `oracleNodeText`, `oracleSourceDigest` -> source-byte hashing/AST/string analysis, build-constraint evaluation, fixed source reads, and assertions. The shared oracle helpers are in `oracle_startup.go` and do not invoke an oracle. Manual/native source is only read as bytes.

The ordinary closure uses standard-library bytes/errors/io/os error values/reflect/syscall errno values/testing/sync/time values/strings/fmt, plus the listed source-analysis libraries for source-contract roots. No model imports a native factory, acquires a descriptor, calls a random source, changes permissions, fills storage, mounts anything, or performs actual publication. No package `init` or `TestMain` activation root was found in VFS or testscope. Reviewed package initializers are error creation, literal tables/maps, or inert set construction; source strings containing native calls are not executable initializers. The Linux VFS import of `golang.org/x/sys/unix` does not create a selected publication edge. Ordinary trusted runtime/import initialization is not physical filesystem qualification.

Repository-wide source searches found no other caller of the changed WAL model helpers and only the additional backup admission root described above outside the two changed files. Other namespace roots reference unchanged inert error types, not the changed event helper.

## Corrected finite selectors

New roots only, package `./internal/vfs` (2 roots, 8 finite subtests):

```text
^(TestWALPublicationMemoryPublicationErrno|TestBackupPublicationModelPublicationErrno)$
```

Affected ordinary **and race** selector, package `./internal/vfs` (20 roots; use exactly the same root selector for normal and race validation):

```text
^(TestWALPublicationMemorySuccessOrdering|TestWALPublicationMemoryOneShotFailures|TestWALPublicationMemoryCloseOutcomeMatrix|TestWALPublicationMemoryBoundsAndTransfer|TestWALPublicationMemoryDiagnosticOwnership|TestWALPublicationMemoryNamespaceFacts|TestWALPublicationMemoryUnknownHooksBeforeUnwrap|TestWALPublicationMemoryUnsafeBranchPreservesErrno|TestWALPublicationMemoryNilWrapperFailsClosed|TestWALPublicationMemoryPublicationErrno|TestBackupPublicationModelStateAndFailures|TestBackupPublicationModelSameFinalRace|TestBackupPublicationModelSealAndRetirement|TestBackupPublicationModelOwnedAcquisitionFailure|TestBackupPublicationModelPublicationErrno|TestBackupDestinationModelAdmissionBlocksMutation|TestWALPublicationSourceDependencies|TestWALPublicationSourceNegativeVariants|TestBackupPublicationSourceCapabilityFlow|TestBackupPublicationSourceMutationCounterexamples)$
```

Affected guard selector, package `./internal/testscope` (11 roots):

```text
^(TestWALPublicationGuardRejectsActivationReferences|TestWALPublicationGuardSafeControls|TestWALPublicationGuardPureFilesRejectNativeAliasesAndVariables|TestWALPublicationGuardExactCallBoundaries|TestWALPublicationGuardFixedSourceAndWiring|TestWALPublicationGuardFakeFactoryIsExactSource|TestWALPublicationGuardProductionBoundariesAreRequired|TestWALPublicationGuardRejectsControlFlowAndFactoryDrift|TestWALPublicationGuardManualSourceIsExactAndDoubleGated|TestWALPublicationGuardElapsedSourceIsInertAndPinned|TestWALPublicationGuardElapsedRejectsTimingAndQualificationDrift)$
```

These selectors exclude all native/database/manual roots. Do not substitute a broad `TestBackup`, `TestWAL`, package-wide, or repository-wide execution selection. Normal, race, guard, formatting, compilation, native, and physical stages are all **NOT_RUN by this reviewer**. Any later run is a separate authorized integration step; formatting changes require final-byte hash verification. Keep every guard-negative assertion unchanged.

## Formatted integration addendum — 2026-10-12 22:04 UTC

**CLEAR_SOURCE_ONLY. The selector blocker above is resolved. No remaining source-review blocker.**

Reviewed the integration identified as candidate tree `598b243e946431c90ed0b5da307f89b288cc19e2` over base `edcc8be3e4c0d0dc46066020dc900a5effcaee30`, using `errno-model-integration/SOURCE-IDENTITY.json`, `finite-manifest.json`, `format-only.diff`, `integration.diff`, the two `.before` payloads, and current source bytes. The `.before` SHA-256 values exactly match the already-reviewed overlay identities.

Both formatted model files have identical significant lexical-piece sequences to their reviewed preformat files after excluding whitespace and optional statement semicolon separators. An independent exact double-quoted-literal inventory comparison is identical; no raw string literals occur, and the sole rune literal remains `'x'`. Also compared complete non-whitespace/non-semicolon byte sequences. Reviewed removed semicolons in context: they separate complete statements moved onto their own lines, retaining normal Go semicolon insertion; conditional initializer semicolons remain. These are text/regex comparisons and source inspection, not a parser or compiler execution. No semantic, assertion, call, literal, branch, or ownership change was found. The 13 existing test bodies remain unchanged beyond formatting.

Confirmed the only production-guard edit is the one WAL model whole-source pin, from `7381288113bc424421bf8dade2f3a4e75bfd3b46f3201b3468762341a54ad9e7` to the exact final hash below. No guard-negative assertion was edited. The integration diff affects exactly the two model files and that one guard file.

Independently verified final SHA-256 values:

- `internal/vfs/wal_publication_model_test.go`: `465813a5c6c4b1324ad6b4ed4850585c24d145e5e3a7a4f7e22ca1d178cd0902`
- `internal/vfs/backup_publication_model_test.go`: `9c17d9c4cbc4ed7076c5b665d119fc892d23acffb20a77606bb7d7282f2c73cd`
- `internal/testscope/wal_tail_publication.go`: `bf572575fd3440b46d7ec13d3f094bdb9cac09c9acde16f7d93273b5b647d1c5`

The corrected finite manifest includes `TestBackupDestinationModelAdmissionBlocksMutation`, counts 20 VFS roots, and retains the same 11 guard roots. The original overlay identities and NOT_RUN fields in that copied manifest describe the source-stage payload; the integration identity record above supplies final formatted identities. The reviewed selected-root call closure is unchanged. This addendum adds no new validation scope or review prerequisite. Go/formatter/parser executable/compiler/test/native execution remained NOT_RUN by this reviewer; W was not edited by this reviewer.
