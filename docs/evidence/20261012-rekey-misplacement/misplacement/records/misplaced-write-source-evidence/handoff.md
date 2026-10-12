# R2 source-only handoff

The overlay contains exactly eleven existing Go files and two existing Chinese manuals. It implements only index-misplace-minus64-error; original case IDs 1–9 remain intact and the appended case is 10. Current source, independent review, formatting and execution statuses are separate: source authored; independent review PENDING; Go/gofmt/parser/model/sourceguard/compile/nm/CI and all native miswrite/recovery NOT_RUN.

- Overlay: /workspace/scratch/5e06a74e5f65/restart-20261012T2033/misplaced-write-overlay
- Base commit: 01c63e6f8fb3d9e196afe2f42a163eb3eac4ad5a
- Patch: /workspace/scratch/5e06a74e5f65/restart-20261012T2033/misplaced-write-source-evidence/misplaced-write.patch
- Patch SHA-256: 6dbe0c46e7c12a9cf7a23350d68bdac1e57a6d89722ee840bb62b396582044ae
- Manifest: /workspace/scratch/5e06a74e5f65/restart-20261012T2033/misplaced-write-source-evidence/source-manifest.json
- Manifest SHA-256: d77db0fad47ad18637d8a70a77ab4af43a53436410b56ca19f2cbed29de1cd3a

## Implemented boundary

The new native action requests the unchanged full 4096-byte b from the original managed owner at the actual argument nativeOffset := off - 64, exactly once. Only native (4096,nil) becomes adapter (4096,existing-injected-error). Native errors and counts remain original. Actual incoming/native offsets and lengths, call count and pre-native payload hash are captured separately from the generic requested receipt. Exact actual three-page arming, strict original public-wrapper/poison/target-only cause, shifted range and untouched tail checks, complete original journal byte comparison and OLD-only one-fresh-copy recovery evidence bind this one case.

The byte-proof branch is gated by commitCheckErr == nil, which means the expected non-nil RecoveryRequired Commit and owner retirement have been validated. It never requires Commit itself to return nil. Original writer/cause evidence is captured regardless; dependent failure is reported only after commit-observation.json retains available observations. Native abnormal failures remain unfulfilled evidence and do not qualify.

## Text/byte checks actually performed

- Every prior test function name in each of the eleven files is retained.
- Four generic Call/Result/Event/CommitResult declarations are byte-identical.
- rollbackFaultBefore, rollbackFaultBeforeCore, rollbackFaultAfter, rollbackFaultAfterCore are byte-identical, including deferred unlock and restart observer order.
- Existing baseline constructor, SQL query/readback and manualRollbackSQLRecoverCopy bodies are byte-identical.
- Seven owning source pins and seven full source snapshots exactly match the overlay bytes by text/hash comparison.
- 270 finite source mutation anchors were found in the intended source text. This is an anchor audit only, not parser execution or a passing guard test.
- No output file was written into W by this worker. Concurrent R5 additions were visible in W and were not modified.

## Authored new tests

- Two generic model roots, two SQL evidence roots, and one generic double-repinned source-guard root; no new native runnable root.
- 24 new generic and 25 new SQL source counterexamples. Existing SQL counterexamples remain with equivalent renamed ten-case/five-recovery diagnostics. The old post-DB overwrite negative uses its existing lifecycle assignment as the updated anchor. Unknown SQLCase 10 tests move to 11 because case 10 is now explicitly admitted.
- Rejections include exact action/geometry, native short/invalid result, wrong requested/native argument, fabricated count/payload, masked native error, misplaced result exposed as success, source-only payload/tail/journal checks, wrong recovery partition, NEW/mixed results and old-nine nonzero placement fields.

## Next finite validation, requiring separate authorization

1. Independently review the eleven-file implementation/guard slice and two manuals; compare all preserved functions/types and default/Commit/A5 files using the manifest.
2. Authorized gofmt only on eleven files, then refresh only seven pins/snapshots with /workspace/scratch/5e06a74e5f65/restart-20261012T2033/misplaced-write-repin-text.py against the intended tree; formatting the two guards afterward must not change the embedded source values. Whole-main/review pins remain the merge lead’s responsibility.
3. Fixed official Go 1.27.2 under the parent’s controlled environment/single Go queue, with CGO_ENABLED=0, -mod=readonly, -buildvcs=false, -p=1, -vet=off, -count=1, -parallel=1, -timeout=0: internal/testkit/rollbackfault, internal/testkit/rollbackrestart, internal/testscope.
4. Only TestRollbackWriteFullAtMemoryContract in internal/pager; ordinary `go run ./internal/testscope`.
5. Compile-only sqlite and pager with manual_fault_rollback; compile-only sqlite with manual_fault_rollback,manual_restart_rollback. Never execute those images, even with an empty run selector.
6. Hash and nm the three exact images and record root inventories. A finite race check, if separately approved, is restricted to the same three model/source packages.

Tool and gofmt hashes must be independently verified against the frozen proposal before execution. Source pins are proposed expectations pending review, not permission to release unrelated whole-main HOLD. All actual native miswrite, native recovery, media/power-loss, durability and external qualification remain NOT_RUN. Do not broaden to ANN/campaign/full repository work. docs/progress.md and acceptance-status.md are left to the parent’s result-aware integration update.
