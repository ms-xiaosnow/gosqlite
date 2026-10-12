# R2: one fixed journal-protected misplaced-write source slice

Status: DESIGN ONLY / NOT IMPLEMENTED / EXECUTION NOT_RUN.

Reviewed on 2026-10-12 against `gosqlite` HEAD `edcc8be3e4c0d0dc46066020dc900a5effcaee30`. The relevant rollback sources were unchanged from that HEAD during this inspection; unrelated R1/R6 integration edits were visible and were not touched. This assignment only read sources and wrote this proposal. No Go tool, gofmt, test, fixture constructor, database operation, native fault, recovery, controller, alternate executor, Git write, or CI run was invoked.

## 1. Recommended finite case

Add exactly one case to the existing SQL manual root, tentatively named `index-misplace-minus64-error` / `SQLIndexMisplaceMinus64Error`. Append the enum value after the existing nine values; retain their numeric identities and behavior. Add one fixed adapter action, tentatively `MisplaceIndexPageMinus64`, after `PrefixShortWrite`.

The operation is the actual third DB-page write of the unchanged three-page SQL transaction:

- Requested owner: the original managed database `*vfs.File`, bound to the same pager Tx and cleanup owner.
- Requested point: `DBPage`, occurrence 3, offset 8192, length 4096.
- Actual managed-owner call: the same full 4096-byte index-page payload, at offset **8128**, exactly 64 bytes earlier.
- Accepted native result: exactly `(4096, nil)` from that one `WriteAt`.
- Adapter result: `(4096, errRollbackFaultInjected)`, with the existing private non-nil injected cause.
- `writeFullAt` result: that same non-nil cause, because its original error-first rule remains unchanged.
- Public Commit: `CodeIOErr`, `ErrPoisoned`, `ErrRecoveryRequired`, preserving the original injected cause. No `io.ErrShortWrite`, `ErrCommitUncertain`, cleanup-pending, or extra consumed cleanup error qualifies.
- Accepted terminal readback: **only complete OLD**, after exactly one recovery of a fresh independent copy. NEW, mixed rows/index entries, or a receipt alone cannot qualify.

This is an actual misdirected managed-owner write followed by an adapter error. It is not an OS-originated error, a short write, a lost-write case, a power cut, or hardware torn-sector qualification. Code/model/source/compile completion cannot confer physical PASS.

### Why minus 64 rather than plus 64

The selected index is page 3, the last page. Writing 4096 bytes at 8192+64 would cover `[8256,12352)`, extending the 12288-byte file by 64 bytes. That would require changing the current SQL artifact/read limits and exact extents. Production recovery is not intrinsically unable to handle that: `ReplayPlan.ValidateExtent` permits extra current bytes, and `RecoverRollback` subsequently truncates to the journal's original extent. `MaxDatabaseBytes` bounds the restored extent, not necessarily the pre-replay file's extra tail.

The proposed minus-64 case covers `[8128,12224)`, entirely inside the existing three-page extent. It overwrites the last 64 bytes of page 2 and the first 4032 bytes of page 3. Both pages have complete original images in the journal. The final 64 bytes `[12224,12288)` are not written and must remain identical to the frozen baseline. No fourth page, new extent class, larger limit, or truncation-specific case is needed. Recovery still executes its existing `Truncate(12288)`; this slice does not claim it tests shrinkage.

## 2. Geometry and call order derived from current sources

These are source-derived facts, not assumptions about a future run:

1. `sqlite/rollback_fault_sql_manual_linux_test.go:481–510`, `manualRollbackSQLBaseline`, creates a 12288-byte SQLite image, page size 4096, header page count 3. Schema rows explicitly assign table `t` root 2 and index `by_k` root 3. The first-page schema and page-2 table are table leaves (`13`); page 3 is an index leaf (`10`). The existing OLD and NEW row sets are independently literal in `SQLLedgerRows`.
2. `manualRollbackSQLCommit:605–692` performs only the existing UPDATE id 1, DELETE id 2, INSERT id 4, each with exactly one affected row, then one public `tx.Commit(ctx)`. The next statement supplies its untouched error to `FinishManualRollbackSQLFault`.
3. `internal/pager/rollback_fault_sql_manual_linux.go:111–127` already checks original count 3, final page count 3, exactly three dirty buffers, and full page buffers for keys 1, 2, 3. `armRollbackFault` independently binds sorted dirty and journal-original page offsets to every manifest call.
4. `internal/pager/commit.go:33–72` sorts dirty keys and obtains the sector-neighbor journal set. With page size = sector size = 4096, original extent 3, and dirty keys exactly 1/2/3, the original set is exactly 1/2/3. No unknown neighbor or unjournaled page intersects the proposed shifted write.
5. `commit.go:123–144` finalizes page-1 change counters, builds a finite journal header with Records=3 and OriginalPages=3. `:218–250` records all three OLD pages, then performs journal body Sync, valid-header publication, journal header Sync, and directory publication Sync before any DB write.
6. `commit.go:251–258` iterates the sorted dirty pages. Page 1 uses finalized `p1`, page 2 uses the dirty table leaf, and page 3 uses the dirty index leaf. The existing `SQLPlan` inventory puts this selected write at zero-based Calls[14], the fifteenth call. This number follows from the actual path; it is not an arbitrary global call-number hook.
7. `writeFullAt` at `commit.go:19–29` calls `WriteAt` once, preserves a non-nil error before checking length, and never retries.
8. On the selected write error, `databaseTouched` is already true. The existing failure/poison path closes the journal owner and retires the main owner, preserves the journal pathname, and classifies recovery-required. It does not call DB Sync, JournalUnlink, FinalDirectorySync, or the pre-DB cleanup-unlink branch.
9. The journal is 4096 + 3×(4+4096+4) = **16408 bytes**. Record offsets are 4096, 8200, 12304. Original-image data start at those offsets+4, and their page numbers are 1,2,3.

The new accepted transcript is the existing first 15 calls, with only the selected action changed. It has 35 events: RunStarted, 15 Intent/Returned pairs, CommitReturned, OwnersObserved, ValidationObserved, Terminal. The requested DBPage Call remains `(3,8192,4096)` throughout. A Return event whose Call.Offset is changed to 8128 remains a forged/mismatched receipt and must be rejected.

## 3. Smallest model change that preserves A5's exact cores

### Generic contract

In `internal/testkit/rollbackfault/contract.go`:

- Append only the fixed action enum. Do **not** add `OffsetDelta`, a caller-supplied destination offset, callbacks, an action registry, or any new profile.
- `NewPlan` admits that action only for page size/sector 4096, three dirty pages, three original images, target DBPage/Database/MainOwner/occurrence 3/offset 8192/length 4096, Prefix=0, PrefixError=false, ExpectedCommit=CommitRecoveryRequired, CallCount=15, and the exact 0/4096/8192 original-read and dirty-write inventories. The existing structural call validator continues to apply. OriginalPages counts images, so native arming separately validates the actual original extent.
- `validResult` accepts this action's selected successful fault only as Executed=true, NativeN=ReturnedN=4096, NativeError=None/0, ReturnedError=NormalizedError=Injected/0, EffectPossible. The existing native-error branch stays first and preserves the original underlying error/token; an actual error is never overwritten with the desired injection.
- Native short/invalid counts, a missing error, a short-write normalization, or an unexpected effect cannot qualify. A native error/short count remains retained evidence of an unfulfilled plan, not success.
- Keep `Call`, `Result`, `Event`, `CommitResult`, the bounded receipt capacity, and all existing event shapes unchanged. Both physical-acceptance methods remain false.

This is deliberately different from adding native-offset fields to generic `Result`: doing so would require changing `rollbackFaultAfterCore`, whose body is independently frozen by the A5 guard. There is no need to perturb that core or its wrappers merely to carry SQL-only placement evidence.

### Fixed SQL placement evidence

In `sql_ledger.go`, add one small scalar/fixed-array record for the actual writer arguments, for example `SQLMisplacedWrite`:

- `RequestedOffset int64` and `RequestedBytes int`, captured from the real writer's incoming arguments;
- `NativeOffset int64` and `NativeBytes int`, captured from the variables supplied to the original managed owner's actual `WriteAt`;
- `NativeCalls uint8`, incremented at that call site, not retrospectively assigned from the plan;
- `PayloadSHA256 [32]byte`, computed from the actual full write slice before the call.

Add one optional-by-case `SQLMisplacementEvidence` value to `SQLLedgerEvidence`, containing:

- the actual writer record above;
- SHA-256 of the observed post-Commit shifted range;
- SHA-256 of the independently frozen baseline's same range;
- SHA-256 of the baseline and observed untouched final 64-byte tail;
- the three parsed journal-original page hashes in page-number order;
- one actual `OnlyTargetCause` observation for this new case, described below.

All those fields must be canonical zero for the old nine cases. For this case, the model requires requested `(8192,4096)`, native `(8128,4096)`, one native call, a nonzero payload hash equal to the observed shifted-range hash and different from the baseline-range hash, equal nonzero tail hashes, journal-original hashes equal to `Originals.Frozen`, and `OnlyTargetCause=true`. Numeric equality uses literal fixed values, not values copied from an arbitrary plan. The ordinary generic receipt already verifies the native/adapter/normalized result and error identity; do not duplicate those errors into a new weaker success claim.

Hashes are still caller-supplied evidence labels in the pure model, just as existing artifact hashes are. Only the source-gated manual caller derives them from actual bytes; synthetic matching hashes are not physical evidence.

Append the new SQLCase, map it to the fixed action and 15-call trace, add it to the exact recovery-case predicate, and require OLD through the existing seven-query/index/Check contract. It is neither a pre-DB case nor one of the existing two post-DB-publication cases: `PostCommit` remains zero; `ReopenCalls` remains 1; the caller must not try to open the damaged retained pre-replay image as valid NEW.

## 4. Actual writer and public caller

### Manual adapter changes

Only `linux && manual_fault_rollback` code changes its native behavior.

1. In `armRollbackFault`, the new action additionally requires actual `t.info.Size()==12288`, `t.originalPages==3`, `t.header.PageCount==3`, exact dirty keys 1/2/3, full buffers, exact original-image set 1/2/3, and the expected table/index leaf kinds. Existing generic admission, main owner identity, cleanup health, no session/shared/encryption/keepFile/borrowed cleanup, and one-lifetime registry all remain in force. Do not invent geometry from a plan count. The preexisting stable-file checks in Commit remain unchanged.
2. Add a private scope slot for the new writer record. It is empty for every old action.
3. Give `rollbackFaultWriter.WriteAt` one explicit, early-return branch for the new action. Before native execution it rejects an unexpected incoming offset or length. It records requested off/len, computes `nativeOffset=off-64`, records full native length and payload digest, increments the actual call counter, and calls the same `w.file.WriteAt(b,nativeOffset)` exactly once. This branch must not slice to 64 bytes.
4. Start from the actual native `(n,err)`. Replace only nil with the existing injected cause, and only if `n==4096`. Preserve any native non-nil error unchanged; preserve short/invalid counts. Return immediately. Existing Pass/FailBefore/PrefixShortWrite code remains the other branch with its original semantics.
5. `rollbackWrite` receives the writer record from that actual invocation and stores it in the bound scope once, under its private mutex, before calling the original `rollbackFaultAfter`. Reject a second observation or a mismatch with the ticket. No I/O occurs while that mutex is held. Do not synthesize the native offset from `spec.Target`, and do not edit a returned event's Call to hide the difference.
6. Leave `rollbackFaultBeforeCore`, `rollbackFaultAfterCore`, `rollbackFaultBefore`, and `rollbackFaultAfter` byte-identical. In particular, the core's deferred unlock still completes before each existing restart observer is called. A5 uses Pass, so it never enters the new branch or emits placement evidence.

`crypto/sha256` is the only new adapter import needed for the proposed actual payload digest. It creates no file/descriptor or callback. No production Commit, default bridge, VFS, recovery routine, exported option, retry/reset API, or A5 source needs modification.

### SQL bridge changes

`ArmManualRollbackSQLFault` maps the one new action to the one new case and still compares the complete plan to `SQLPlan`. Existing three-page admission and owner checks remain.

`FinishManualRollbackSQLFault` copies the fixed writer observation from the bound finalized scope into its report. For this action only, it reads the actual poisoned Tx error and uses the existing strict `manualRollbackSQLOnlyTargetCause` with permitted `ErrRecoveryRequired`. The caller still requires the top object to be the untouched `*sqlite.Error`. This produces the new case's `OnlyTargetCause` boolean; old pre/post cause flags are unchanged and remain zero for this case.

The strict cause proof rejects extra consumed Close/cleanup errors even when owners subsequently appear retired. Do not extend that proof to arbitrary custom error chains; reuse the current finite exact-leaf/Join validation and bounds.

### SQL manual caller changes

The existing root remains the sole root: `TestManualRollbackFaultSQLLedger`. Add the exact case-name mapping, then retain its existing runtime gate, exact flags, reviewed source/tool/binary/compile receipt binding, namespace checks, full baseline SQL admission, three DML statements, one Commit, immediate Finish, diagnostics, and retirement boundaries.

After Commit and before creating or recovering a replay copy, the new case must perform all of these actual observations:

- Read the original trial as exactly 12288 bytes and the same device/inode as its pre-Commit creation. Copy the bridge's actual writer observation; do not fill its fields from constants or the plan.
- Recheck the exact observed writer arguments and count, and the target event's native/adapter/normalized result. Require injected cause only, recovery-required, and the absence of short-write/uncertain/pending flags.
- Hash `post[8128:12224]` and require equality to the actual pre-native payload hash. Hash `baseline[8128:12224]` and require inequality. This excludes the correct-offset full write accompanied by a forged wrong-offset report, a skipped write, and a 64-byte-only write.
- Compare `post[12224:12288]` byte-for-byte to `baseline[12224:12288]` and record matching hashes. Do not assert that all bytes outside the shifted range are OLD: page 1 and most of page 2 were legitimately written earlier in this failed Commit.
- Read and retain the exact 16408-byte original journal. For the new case only, call the existing pure `journal.ParseReplay` on those already-read bytes with limits 16408/3/12288. Require finite-header count evidence, header Records=3, OriginalPages=3, page/sector 4096, DatabaseBytes=12288, exactly pages 1/2/3 in order, and each complete page image byte-equal to the independent baseline page. Check `ValidateExtent(12288)` as well. Record the three original-image hashes. No new parser or extra recovery call is needed.
- Keep the current successful journal-publication Intent/Returned prefix. A parsed journal alone is not evidence that journal Sync/header Sync/directory publication occurred.
- Preserve `commit-observation.json` including actual placement/cause observations before any dependent failure path can discard the result. Original errors/diagnostics remain strongly retained in memory and represented as supplementary strings on disk; strings never replace identity checks.

The pure parser call and its original-image comparison may be placed in one fixed helper in the same existing manual caller file. It is not a new root, a general parser exception, or an ordinary test fixture generator. Its invocation and call arguments must be included in the narrow source guard.

## 5. Recovery, complete allowed states, and retention

The complete permitted result set for the new case is `{OLD}`. The planned failure occurs before DB Sync and before journal unlink, with a complete published journal, so there is no durable commit acknowledgment. There is no `{OLD,NEW}` fallback and no uncertain-success result.

Retain the current exact artifact inventory and size limits:

- `baseline.input.db`: untouched independent OLD image, 12288 bytes;
- `trial.db`: the original miswritten image, retained unchanged after observation;
- `trial.db-journal`: original finite three-page journal, retained unchanged;
- `post-commit.input.db` and `post-commit-journal.input.bin`: separately created immutable copies;
- `replay.db` and `replay.db-journal`: another O_EXCL-created, independently identified pair used for the one recovery;
- binding, commit observation, and terminal result evidence as currently supported.

Before recovery, all present artifact identities must be distinct, with fixed lengths and matching intended source/copy hashes; original writer and reader owners must be retired. Use the existing `manualRollbackSQLRecoverCopy` unchanged, with `pager.RecoverRollback` exactly once, on basename `replay.db`, fixed limits 16408 journal bytes / 3 records / 12288 database bytes. It must return Recovered=true and RestoredPages=OriginalPages=3; replay journal must be absent; owners must be retired.

Open only the recovered independent `replay.db` read-only/NoCreate. Reuse all seven exact typed row queries, actual SEARCH/by_k work versus NOT INDEXED scans, complete EOF/Err/one Close observations, and Check (three pages, one table, one ordinary index, three table rows, three index entries, zero free/reserved/extension pages). Require its full-file hash equal to the frozen baseline, not just the same row count or a successful Open. Thus the overwritten table tail, damaged index region, unchanged index tail, and page-1 metadata are all restored as a complete image.

Then recheck retained baseline, trial, original journal, and immutable copies by the existing identity/extent/hash machinery. Recovery may remove only the replay journal through the existing production operation. No source artifact is healed in place or deleted, no descriptor is closed a second time, and no registry is reset. Unknown/pending/failed owner retirement, extra native error, mismatched payload, invalid journal, or failed recovery stops the scenario and retains available evidence. Preparation-only healthy Rollback remains as currently permitted; after arming/Commit no Commit, Rollback, Sync, Close, or recovery retry is introduced.

The proposed extra observations add no extra owner types or artifact files. Their byte/hash work operates on already-bounded in-memory buffers. Existing owner/retention capacities need not increase.

## 6. Minimum source file set

Eleven existing Go files are the expected source slice:

1. `internal/testkit/rollbackfault/contract.go`: one action, fixed admission, result semantics; generic event/result shapes unchanged.
2. `internal/testkit/rollbackfault/contract_test.go`: finite admission/result/cause counterexamples and new action-positive model.
3. `internal/testkit/rollbackfault/sql_ledger.go`: one appended case, fixed placement evidence, exact recovery partition and OLD observation checks.
4. `internal/testkit/rollbackfault/sql_ledger_test.go`: new synthetic evidence and independent mutation negatives; retain all old nine-case assertions.
5. `internal/pager/rollback_io_manual_linux.go`: fixed native branch, action-specific actual geometry, actual writer evidence capture; A5-pinned core bodies/wrappers unchanged.
6. `internal/pager/rollback_fault_sql_manual_linux.go`: mapping, bound writer evidence and strict target-cause reporting.
7. `sqlite/rollback_fault_sql_manual_linux_test.go`: existing-root branch, actual byte/journal checks, case mapping and evidence retention.
8. `internal/testscope/rollback_fault.go`: updated reviewed pins/snapshots for changed core-model/manual files plus independent semantic checks for the new fixed action/writer.
9. `internal/testscope/rollback_fault_test.go`: repinned/double-repinned counterexamples for real call arguments, full payload, one call, non-nil error, native-error precedence, and strict admission.
10. `internal/testscope/rollback_fault_sql.go`: exact ten-case declarations/partitions, placement proof, public cause and journal-helper checks; update only affected pins/snapshots.
11. `internal/testscope/rollback_fault_sql_test.go`: matching independent source counterexamples and old-nine zero-evidence invariants.

Update the existing Chinese SQL manual and adapter manual for the new action and evidence boundary; update `docs/progress.md` and the relevant acceptance checkpoint only after the actual slice/check results are known. No broad narrative rewrite is needed. Do not change `goal.md` or add another R item.

Expected untouched: `commit.go`, `recovery.go`, `rollback_io_default.go`, `rollback_io_points.go`, `rollback_io_contract_test.go`, the A5 model/caller/controller/bridge, `rollback_restart.go`, the one-page recovery source/guard, `internal/testscope/main.go`, default public API, VFS, SQL/DML/index algorithms. The existing shared-source-pin helpers read the updated owning maps dynamically, so they should not need new unrelated exemptions. If a concrete guard conflict emerges, identify that exact binding before enlarging this file set; do not release a whole-main HOLD or broaden a tag exception to make checks green.

## 7. Required finite positive and negative checks

### Pure model

- One accepted synthetic 15-call/35-event new-action trace, native full write followed by injected error; ObserveSQLLedger accepts only complete OLD recovery and returns physical=false/pass=false.
- Reject action at every other point, owner role, occurrence, offset, length, dirty/original count, prefix setting, call count, or journal-original inventory. Unknown action/case IDs remain rejected. Do not make future enum values valid through a range predicate.
- Reject a requested Call changed to 8128. The actual/native offset belongs only to the separate observation.
- Reject native offset 8192/+64/-128/zero, wrong requested offset, wrong full length, 0 or 2 calls, zero/mismatched payload hash, payload matching baseline, changed untouched tail, wrong/missing/reordered journal page hash, and a nonzero misplacement record on any of the old nine cases.
- Reject `(4096,nil)` exposed as adapter success, `(64,injected)`, `(4095,nil)`, fake ShortWrite, wrong normalized cause, masked underlying errno/error token, a success/uncertain Commit, missing target cause, extra close cause, logical-open, pending or unknown owner, replay count 0/2, wrong limits, missing journal, aliased copies, changed originals, NEW instead of OLD, and mixed row/index members with equal counts.
- Keep existing wrong-offset receipt negatives: they prove transcript rejection, not the actual miswrite.

### Independent source guard

Repin both whole-source digest and stored complete-source snapshot for each mutation, then still require an independent semantic diagnostic. At minimum mutate:

- actual WriteAt offset back to `off` while leaving the reported offset at 8128;
- reported native offset filled from the plan rather than the actual call argument;
- actual write changed to only a 64-byte prefix;
- missing native invocation with fabricated count/hash, or a second native invocation/retry;
- injected error dropped after full native success, or native non-nil cause overwritten;
- payload hash computed from post-Commit bytes, or shifted-range/tail/journal-byte comparisons removed/bypassed;
- new action admitted outside exact actual three-page geometry;
- requested Call overwritten with the native offset;
- selected action added to pre-DB/post-publication predicates, skipped replay, broadened OLD/NEW allowance, or false physical/ack result;
- default build gate/body, A5 wrapper/core body/order, private one-shot registry, provenance gate, original-retention boundary, exact public Commit/Finish adjacency, or fresh-copy-only recovery changed.

Do not satisfy these tests only through a stale hash mismatch. Preserve and run existing A5 exact-core/mutex-order counterexamples; this proposal intentionally avoids modifying their expectations.

## 8. Exact proposed future validation scope (NOT RUN here)

These commands are a proposed next authorization/check scope, not permission to run them now. Use the previously frozen official toolchain only after verifying its bytes, with the current controlled environment, no network/module fetching, and the parent's existing single Go queue. Bind every result to the final source manifest and preserve original FAIL/NOT_RUN records. No unrelated full-repository test, oracle, fuzz campaign, ANN, device/media, mount/chmod, or alternate executor is part of R2.

Let `GO` denote `R/toolchains/go1.27.2/bin/go`, whose previously recorded SHA-256 is `548d3c32f83fa81152f543eb1526be7884cfe0526eb5e9ce29d3eca14786e7e0`; `gofmt` has recorded SHA-256 `5583dbb3339147fee7cdb067eb3cd4a50015b48c27084377f49d93833aa4ad77`. Verify, do not merely copy those labels. Let `E` be a new bounded R2 evidence directory. From `W`:

1. Text/source review of all eleven changed Go files and the changed manuals; explicitly compare the unchanged default/Commit/A5 files and four core/wrapper bodies. Authorized gofmt only on the named changed Go files; final `gofmt -l` must be empty. Source review and gofmt have separate recorded statuses.
2. `CGO_ENABLED=0 GO test -json -mod=readonly -buildvcs=false -p=1 -vet=off -count=1 -parallel=1 -timeout=0 ./internal/testkit/rollbackfault ./internal/testkit/rollbackrestart ./internal/testscope`. These are finite descriptor-free model/source-guard packages. Including rollbackrestart checks compatibility with its unchanged Pass event contract.
3. `CGO_ENABLED=0 GO test -json -mod=readonly -buildvcs=false -p=1 -vet=off -count=1 -parallel=1 -timeout=0 -run '^TestRollbackWriteFullAtMemoryContract$' ./internal/pager`. Only that existing memory-writer root executes; no manual tags or native fixture/root selection.
4. `CGO_ENABLED=0 GO run -mod=readonly -buildvcs=false -p=1 ./internal/testscope`. This executes the ordinary source guard, not the manual adapter. Its full preexisting policy wiring stays intact.
5. Compile only: `CGO_ENABLED=0 GO test -mod=readonly -buildvcs=false -p=1 -vet=off -c -tags=manual_fault_rollback -o E/sqlite-fault.test ./sqlite`.
6. Compile only: the same `-c -tags=manual_fault_rollback` selection for `./internal/pager`, output `E/pager-fault.test`, to check the unchanged one-page recovery caller still compiles with the shared model/adapter.
7. Compile only: `CGO_ENABLED=0 GO test -mod=readonly -buildvcs=false -p=1 -vet=off -c -tags=manual_fault_rollback,manual_restart_rollback -o E/sqlite-fault-restart.test ./sqlite`, to verify A5 coexistence without running any role/controller.
8. `GO tool nm` on each of those three exact images, record hashes and root inventories. The SQL root name is unchanged, and no new runnable root/controller should appear. Merely seeing a symbol is not a run or a correctness result.

If a separately approved finite race check is desired, limit it to the same three pure/source packages with `-race` and the already-reviewed local race C support (`CGO_ENABLED=1` for that test build only). This is separate from CGO_ENABLED=0 production support and is not necessary to pretend a physical miswrite was tested. Never run a tagged binary, even with `-run '^$'`, for this compile-only scope.

The code-facility stopping condition is one independently reviewed implementation of the fixed case, unchanged old-nine/A5/default behavior, passing authorized finite model/source/guard/compile results on the exact final bytes, precise manuals, and an independently saved checkpoint as directed by the parent. Actual miswrite, native recovery, physical durability, power loss, and external qualification remain **NOT_RUN** unless later independently authorized and actually observed. No extra fault case is introduced merely because a new dimension can be imagined; only a concrete discovered bug justifies reconsidering this frozen slice.

## 9. Inspection identity

Observed source SHA-256 values before implementation:

- `rollback_io_manual_linux.go`: `4f26bfec0de7b3c22f056ec374a4c453691f5cd2a5ee5e62116f2f8c66b5e498`
- `rollback_fault_sql_manual_linux.go`: `ad9f263e2d15a496cc70b1d35038f5b0393e8b94c0923ea5db96bf4eff71c0f9`
- `rollbackfault/contract.go`: `dc7ec44ec7be3f444f4e7501c84d6875a1a0e9b4d46e06e5586b2e5d225828ed`
- `rollbackfault/sql_ledger.go`: `7d47e46b34204c12a4df50d02a24179bf22e081d45f2816e3c048b0bc7dd04dc`
- `sqlite/rollback_fault_sql_manual_linux_test.go`: `c368296059cbf29a4756cbb760b3836c580d8615e25a7330f76d83ba97427037`
- unchanged `commit.go`: `9ae36612afac5cfae48bd972e6162c94722bcf7355ea462b4e15289b5188aa32`
- unchanged `rollback_io_default.go`: `f1d45680aa93940d9d181f01ecc40f5e88c258970c940a28b4f19df712182bd1`
- unchanged `rollback_restart_manual_linux.go`: `580c1726e059ed36c86fbaafbe491ec8062f527e9b7db3ef4d45cb501b1f4625`
- unchanged `internal/testscope/rollback_restart.go`: `a6edb3a708ecf81c688c766835b349c26777553613de587673847b451a0e8ab2`

Goal binding: `goal.md` M3 line 356 explicitly names write-misplacement injection; the frozen finite checklist R2 line 50 requires one journal-protected actual action/caller plus complete recovery ledger, with source/model/guard/compile as the facility code endpoint and physical qualification separately NOT_RUN. This proposal satisfies that finite design boundary without treating an invalid offset receipt as a real misplaced write.
