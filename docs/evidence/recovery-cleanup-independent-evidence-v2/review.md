# Independent review: recovery cleanup ownership

Decision: no blocking source finding in the four-path ownership repair. This is approval of the bounded ownership/source change only. It does not qualify native recovery or production durability.

## Exact reviewed source

- Baseline: `7d3184f87165e11e12560884ec6e4f92c9bfd039`, independently recomputed from all 1,252 files; baseline SHA-256 manifest matched.
- Final owner freeze: `recovery-cleanup-freeze-v2`, tree `201dca235ee217594d231f2f583d3ccf65aa8093`.
- Independent final copy: `recovery-cleanup-independent-final-v2`.
- Independent baseline-plus-patch reconstruction: `recovery-cleanup-independent-patch-v2`.
- Source patch SHA-256: `e05bcffffc0388e9033a5e9cff212fed8b5eac370157d7284f13ad8e719e3c5c`.
- All 1,255 final file hashes, sizes, and modes match the owner inventory and the exact recomputed Git tree in both independent copies. `git apply --check` and applying the patch to a fresh baseline succeeded. Only the four declared paths differ from baseline.
- V1 to V2 is exactly one comment correction: managed owners can retain ordinary path metadata, so the overly literal no-path-retention claim was removed. No executable statement or test changed.

## Source findings

1. Admission precedes path resolution and any native acquisition. The deferred owner finalization is installed immediately after reservation. The bounded common registry therefore preserves pre-main metadata/constructor obligations, including errors when no DB is returned.
2. Main ownership is captured before error/stat observation. A temporary slot is reserved before each journal metadata probe, and every non-nil journal open result is adopted immediately. The candidate and replay journal reuse that slot sequentially.
3. Error-carried owners are adopted before nested -> temporary -> main retirement. Incomplete child retirement retains its protecting main/locks. Explicit failed candidate/journal close exits the logical flow, and finalization avoids a second close attempt during the same recovery operation. Consumed diagnostics remain errors.
4. Absence suppression is now tied to VFS initial no-handle provenance. Later acquisition/inspection/close errors cannot pass through raw IsNotExist suppression. Only literal EOF is ignored for the one-byte candidate read, preserving joined diagnostics.
5. The logical recovery ordering remains unchanged: SHARED; RESERVED query; candidate; EXCLUSIVE without RESERVED; rechecks; bounded parse/geometry/header validation; journal and directory Sync; replay; truncate; database Sync; identity checks; child retirement; unlink; directory Sync; main retirement. Failure before unlink preserves the journal. The final child-close error return still precedes unlink.
6. The new helper retains no replay callback/plan/page buffers, DB, Tx, Rows, result or options. Its callable surface is only error joining and existing managed retirement. Explicit retry therefore cannot repeat replay, truncate, Sync, unlink, path/lock acquisition or logical recovery.
7. Existing bounded error discovery, opaque unsafe-graph barriers, immutable historical tokens, snapshot visibility and retirement-status laws are reused. First operation/retirement causes and post-touch RecoveryRequired/CommitUncertain classification survive pending ownership.
8. Exported layouts, parser implementation, automatic Open behavior, encrypted/shared-WAL paths, goal, and all unowned files are byte-identical to baseline. RecoverRollback has no new production caller.

## Independent executed checks

See `final-checks.sh`, `final-checks.txt`, and `final-checks.exit` for exact final-V2 commands and outcomes. All test bodies and reachable test helpers were audited before execution. The source-reference guard preceded the unchanged-source selections.

- Go 1.27.2, local caches, GOPROXY=off, GOSUMDB=off, GOTOOLCHAIN=local.
- Seven `TestPlainRecoveryCleanupMemory*`/`Source*` tests: descriptor-free owner/registry/error models or AST parsing only. No native recovery helper is called.
- Two narrowly selected intact lifecycle tests: `TestTxCreatePageSizesAndReadSnapshot` and `TestTxCommitPreservesUnrelatedHeaderAndPages`. Helpers create or copy intact fixtures, perform ordinary open/read/commit/close, and inspect expected journal absence; no physical failure is injected.
- Both selections repeated with the race detector; CGO is enabled only for the race tooling.
- Pager vet and complete CGO0 production build.
- Pager test-binary compilation only for Linux arm64, Darwin amd64/arm64, and Windows amd64. These foreign binaries are never executed.
- Exact source/inventory verification repeated after testing.

## Preserved negative controls and initial failure

Three separate final-V2 source-copy mutations were tested. Each source guard passed; each selected test failed with exit 1 as intended. The mutated native recovery path was never invoked.

1. `negative-doubleclose.txt`: replace `!cleanup.childClosePending()` with `true`. `TestPlainRecoveryCleanupMemoryExplicitChildCloseIsNotRepeated` fails because same-operation finalization repeats the failed child close.
2. `negative-admission.txt`: replace the common root reservation with a new isolated registry reservation. The source admission law fails because the required common pre-acquisition admission disappears.
3. `negative-rawclose.txt`: replace managed candidate retirement with a direct raw-close wrapper. The source law fails because the required managed candidate close before promotion disappears.

The initial provisional build lacked usable Git VCS metadata and failed with Go's VCS-stamping error. Its unchanged output is retained in `../recovery-cleanup-independent-evidence-v1/provisional-checks.txt`. Corrected and final commands use `GOFLAGS=-buildvcs=false`; this changes stamping, not the test selection. Earlier V1 checks and negative-control failures remain preserved separately.

## Remaining limits

No unresolved ownership-code blocker was found within this four-path delta. The independent new multi-segment parser is outside this exact freeze; its integration and combined-source verification remain separate work, without asserting a permanent single-segment restriction.

Physical native recovery remains **NOT_RUN**. No RecoverRollback invocation, native recovery fixture, manual test tag, corruption/tamper/wrong-key/missing-selected-file workflow, hostile namespace mutation, crash/kill/process exit, native I/O/lock/unmap/close/Sync/short-write/disk-full/permission failure, or interrupted publication/rekey was executed here or elsewhere. Broad executed test suites and remote actions were not used. Darwin/Windows native runtime qualification, physical fault/recovery acceptance, and full M3/M8/M10/release readiness remain unestablished. Terminal native uncertainty can intentionally retain an owner and capacity for process lifetime.
