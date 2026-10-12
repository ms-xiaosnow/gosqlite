# R2 fixed misplaced-write independent source review

Date: 2026-10-12 UTC. Review result: **NO BLOCKING SOURCE DEFECT FOUND in this bounded slice; suitable to proceed to separately authorized finite formatting/model/source/compile validation.** This is not a test result, an execution authorization, a release of unrelated whole-main HOLDs, or physical qualification.

## Reviewed identity and scope

- Root: `/workspace/scratch/5e06a74e5f65/restart-20261012T2033` (`R`).
- Candidate: `R/misplaced-write-overlay`, exactly eleven existing Go files and two existing Chinese manuals.
- Comparison tree: `R/gosqlite`, specified base `01c63e6f8fb3d9e196afe2f42a163eb3eac4ad5a`. Concurrent R5 additions are outside this review and were not edited.
- Source manifest SHA-256, independently recomputed: `d77db0fad47ad18637d8a70a77ab4af43a53436410b56ca19f2cbed29de1cd3a`.
- Patch SHA-256, independently recomputed: `6dbe0c46e7c12a9cf7a23350d68bdac1e57a6d89722ee840bb62b396582044ae`.

Read the handoff, complete implementation/model deltas, affected independent guard decisions and mutants, model test deltas, manuals, and relevant unchanged Commit/journal/owner boundaries. Performed only source/text reads, diffs, declaration inventories, byte hashes, and static text comparisons. No Go, gofmt, Go parser/scanner/AST runner, test, compiler, native image, fixture, Commit, recovery, process controller, CI, or Git operation was executed. The sole authored deliverable is this review file; W and the overlay remain unchanged.

## Findings

### 1. One closed actual write action

`MisplaceIndexPageMinus64` is appended after existing actions, and SQL case 10 is appended after original cases 1–9. `NewPlan` and `validMisplacedIndexPlan` restrict it to a three-page/4096-byte geometry, DBPage occurrence 3, original main owner, requested offset 8192/length 4096, zero prefix fields, 15 calls, and recovery-required. Existing `validCalls` still checks the complete journal/publication/write prefix and zero tail entries. It does not add a general offset API.

Native arming checks actual stored extent 12288, original/header counts 3, sorted dirty and journal-original inventories 1/2/3, page buffers, and leaf kinds 13/13/10. Earlier length checks make the subsequent indexed kind checks safe. Managed owner/cleanup identity, healthy independent Tx restrictions, and one-lifetime registry remain enforced.

The writer uses the unchanged full incoming `b`, computes `nativeOffset := off - 64`, captures the actual incoming/native arguments and pre-call payload hash, increments the call counter at the native call site, and calls the same `w.file.WriteAt(b, nativeOffset)` exactly once. The resulting interval `[8128,12224)` stays inside 12288 bytes and intersects journal-protected pages 2 and 3. Generic requested Call evidence remains `(8192,4096)`.

### 2. Native error precedence remains intact

The adapter first copies both native return values unchanged. Only `nativeErr == nil && nativeN == 4096` changes the returned error to the existing injected sentinel. A native non-nil error remains the original object/count; short or invalid counts are not corrected or retried. Unchanged `writeFullAt` returns non-nil error before checking length.

The generic result validator retains its native-error branch before the new action branch, checks original error tokens, and marks a truthful underlying failure refused rather than expected injection. Nil-error short/invalid counts cannot qualify for the fixed new action. The source mutants cover masked native error/token, fabricated or repeated native call, missing injection, wrong payload/offset/count, and exposed success.

### 3. Proof precedes recovery and retains observations on failure

`manualRollbackSQLCommit` still performs the single public Commit immediately followed by Finish. Its returned `commitCheckErr == nil` means the expected non-nil public error, exact public wrapper, injected cause, recovery-required classification, and owner retirement have been validated. It does not mean Commit returned nil. The new byte-read/proof branch uses this distinction correctly.

Actual writer/cause evidence is copied even when that validation fails. Available range/tail/journal observations and a proof error string are included in `commit-observation.json`; dependent `commitCheckErr`/`misplacementErr` failures are reported only after that record is created. A record-write failure itself cannot promise a saved record and still stops the trial. No proof failure reaches copy recovery.

Before any recovery, the helper requires the exact retained DB/journal extents, post-Commit trial inode identity, shifted bytes hashing to the pre-native payload and differing from baseline, the final 64 bytes equal to baseline, and exact target Returned evidence in the 35-event transcript. It parses the original bounded journal bytes, requires finite headers/three records/original extent/page and sector sizes, calls `ValidateExtent(12288)`, and byte-compares all three original page images to independent baseline pages in order. Journal successful-publication receipt events remain independently required.

### 4. Public cause and complete OLD-only recovery

Finish reads the actual scope target and actual Tx poison under their respective existing locks. The new action calls the existing bounded strict cause observer with only `ErrRecoveryRequired` permitted. The caller requires the original `*sqlite.Error`, CodeIOErr, ErrPoisoned, recovery-required, no short-write/uncertain/pending/extra cleanup cause, and qualified owner retirement.

The case is admitted only to `needsSQLRecovery`; it is absent from both pre-DB and post-publication predicates. It cannot receive a post-Commit healthy NEW snapshot. It uses the existing one-call fresh `replay.db` recovery path, with retained original trial/journal and unaliased copies. The exact recovery limits, journal presence/removal, retired owners, all seven typed SQL/index-versus-scan queries, actual query work, EOF/Err/once-Close, Check, unchanged originals, and whole replay-file baseline hash remain required. The only admitted final state is OLD. Both physical-acceptance methods remain false.

The old nine cases require every added placement field to be zero. Added model counterexamples cover nonzero old-case fields, wrong arguments/payload/tail/journal, missing or repeated recovery, aliases, changed originals, public classifications, NEW, mixed index evidence, wrong members with equal counts, and physical claims.

### 5. Generic/A5/default boundary preserved

Independent text extraction confirms byte-identical Call/Result/Event/CommitResult declarations, four before/after wrapper/core functions, and baseline/query/readback/RecoverCopy helpers. The manifest's extraction hashes match when the final enclosing newline is excluded. The original deferred unlock and restart observer ordering remain unchanged. A5's Pass path does not take the new writer branch or generate placement evidence.

The nine excluded source hashes below independently match. No production Commit/default/VFS/recovery or A5 file is included in the patch. Existing test/root declarations remain, with exactly two generic model roots, two SQL evidence roots, and one generic guard root added; no new native runnable root appears in the source inventory.

### 6. Independent guards and manuals

All seven owning pins match their respective overlay bytes, and all seven complete-source snapshots contain the exact source text. The new generic mutant test repins both digest and full snapshot, then requires a separate fixed-function semantic diagnostic and explicitly rejects reliance on either stale layer. The SQL mutant test retains the same double-repin discipline. Independent literals/argument checks bind actual WriteAt, source capture, arming, public cause, proof helper, proof-before-recovery root flow, OLD partition, and zero old-case evidence. Existing semantic counterexamples remain; changed nine-to-ten/four-to-five diagnostics reflect the exact newly admitted case. Unknown case tests move from 10 to 11 because 10 now has an explicit branch.

The manuals prominently separate SOURCE_ONLY, review, formatting, model/guard/compile, and native statuses. They correctly describe software misdirection plus injected error, exact range and actual return, OLD-only copy recovery, independent journal/tail proof, unchanged old-nine/A5 behavior, and no physical PASS. Historical sections are explicitly retained as historical; their old counts/statuses do not redefine the new case.

No minimal corrective source edit is requested by this review. Finite parser/guard execution can still uncover a syntax, source-shape, or counterexample-anchor issue; none is represented as already passed.

## Exact reviewed overlay SHA-256

| File | SHA-256 |
|---|---|
| docs/testing/rollback-fault-adapter-zh.md | e0e683b7e2c57c31980a606c4ccf9abf5263cdd96e6c5f887b690390012afe60 |
| docs/testing/rollback-fault-sql-manual-zh.md | 8ddd2eac317c52b365bd82ba515487e0067a4c7931f410254fea16a271e682d7 |
| internal/pager/rollback_fault_sql_manual_linux.go | 50ff5277bd0811f4f5f9080520797d9fff6689cce1eb5fb2345d81cbce923b5c |
| internal/pager/rollback_io_manual_linux.go | 1e15ab58e49a4d6eedd72274e20c59f644546609f61a19d96e588688eb0c5940 |
| internal/testkit/rollbackfault/contract.go | dff18faca43a384de7ec37aedee877d30320823e540be71c7486d3ff5a24198d |
| internal/testkit/rollbackfault/contract_test.go | 3b4aae0c648444bf2031b625637aedc2de2a5c9b2632243512b89d8ac3258f31 |
| internal/testkit/rollbackfault/sql_ledger.go | 96def5b6a7545f7e7f8e9e2acaf16e540237cfa2bccd92875b7961560e328267 |
| internal/testkit/rollbackfault/sql_ledger_test.go | 5d0fede7585d788714a380d72691083931c738c903391f0cec76aaecb394f8e3 |
| internal/testscope/rollback_fault.go | c854bfafe0ecedc33e4676595d21a99d460d129ae0a2d6a9ca776178d002caed |
| internal/testscope/rollback_fault_sql.go | ae4adc9a0636e7ca3b9067276fee83965e2aa2002b4754571f661f8926d7f895 |
| internal/testscope/rollback_fault_sql_test.go | fde0e37a3afe4affc649624978eaeaa2f580e9b55b3b5a7d73d9012bce70dab4 |
| internal/testscope/rollback_fault_test.go | 9aa79c9ea8bfeef4ab0f164187e3134ffb5713b6c0292910bab59d45d35ce08e |
| sqlite/rollback_fault_sql_manual_linux_test.go | 576f77b415b98ff02879534480be4753086b949aaea39376fb9488918f9bd8b7 |

## Independently checked excluded source SHA-256

| File | SHA-256 |
|---|---|
| goal.md | 9ae7b1d776856669a10eb753408c7754d015d266f8783240165450e96e4bac81 |
| internal/pager/commit.go | 9ae36612afac5cfae48bd972e6162c94722bcf7355ea462b4e15289b5188aa32 |
| internal/pager/recovery.go | 8865a4dcf37bf40f27245a949372d6e8ef62ec84f804d7f128c0d7d53f140786 |
| internal/pager/rollback_io_contract_test.go | a7695da7b49488be37a98b239b1b31d59bc406455d0b3300709081fd96ae49c4 |
| internal/pager/rollback_io_default.go | f1d45680aa93940d9d181f01ecc40f5e88c258970c940a28b4f19df712182bd1 |
| internal/pager/rollback_io_points.go | e9f4349eac27fe2f2ee6726edeb3cd26e3f5e851e75ae342f2f3a7995f038905 |
| internal/pager/rollback_restart_manual_linux.go | 580c1726e059ed36c86fbaafbe491ec8062f527e9b7db3ef4d45cb501b1f4625 |
| internal/testscope/main.go | 6dbab48ff374858bfafe578f0fca99f5176ced7b6bdc008bec04ce6a6d1d0eb2 |
| internal/testscope/rollback_restart.go | a6edb3a708ecf81c688c766835b349c26777553613de587673847b451a0e8ab2 |

## Finite next validation proposal, all NOT_RUN here

Retain the existing bounded proposal. This section proposes scope only; the lead must verify execution authorization and acquire the existing single Go queue. Preserve the frozen source-only manifest/review, then bind new post-format/integration bytes and any independent R5 files to the actual validation tree. Do not silently transfer old results to new bytes.

Tool files were read and hashed, never invoked, during this review:

- `R/toolchains/go1.27.2/bin/go`: `548d3c32f83fa81152f543eb1526be7884cfe0526eb5e9ce29d3eca14786e7e0`.
- `R/toolchains/go1.27.2/bin/gofmt`: `5583dbb3339147fee7cdb067eb3cd4a50015b48c27084377f49d93833aa4ad77`.
- `R/misplaced-write-repin-text.py`: `bbec39f0155e164359d1572ee2f3a7b3e51a3061205df1ec3c9b11ad4192f0b2`. Static inspection shows seven source pin/snapshot replacements, not semantic-literal regeneration. It was not executed.

1. If separately authorized, gofmt only the eleven named Go files. Refresh only the seven owning pins/snapshots; formatting the two guard files afterward must not change embedded source bytes. Check final gofmt inventory, pins, snapshots, excluded source identities, and the post-format delta. Whole-main/review pins remain the lead's independent responsibility.
2. With the fixed GO under the existing controlled environment, CGO_ENABLED=0 and no module fetching: `GO test -json -mod=readonly -buildvcs=false -p=1 -vet=off -count=1 -parallel=1 -timeout=0 ./internal/testkit/rollbackfault ./internal/testkit/rollbackrestart ./internal/testscope`. This includes the two new generic model roots, two SQL roots, generic double-repinned root, preserved SQL semantic mutant root, and unchanged A5 compatibility/mutex-order checks. Record exact selected roots and JSON exits.
3. The sole pager execution selector is `GO test -json -mod=readonly -buildvcs=false -p=1 -vet=off -count=1 -parallel=1 -timeout=0 -run '^TestRollbackWriteFullAtMemoryContract$' ./internal/pager`, CGO_ENABLED=0, with no manual tags.
4. Ordinary source guard only: `GO run -mod=readonly -buildvcs=false -p=1 ./internal/testscope`, CGO_ENABLED=0. This reads the repository under its unchanged policy wiring; it is not a manual path.
5. Exactly three compile-only images with CGO_ENABLED=0: `GO test -mod=readonly -buildvcs=false -p=1 -vet=off -c -tags=manual_fault_rollback -o E/sqlite-fault.test ./sqlite`; the same selection for `./internal/pager` to `E/pager-fault.test`; and `GO test -mod=readonly -buildvcs=false -p=1 -vet=off -c -tags=manual_fault_rollback,manual_restart_rollback -o E/sqlite-fault-restart.test ./sqlite`.
6. Hash and run `GO tool nm` only on those exact three images, retaining source/tool/binary binding and root inventories. Never execute these images, including with an empty run selector. A separately approved race run, if any, remains limited to the same three pure/source packages with the reviewed local race support.

Stop the facility slice after the exact finite authorized results and checkpoint are established. No broad framework extension, extra case, all-repository test campaign, ANN/fuzz restart, native miswrite, native recovery, A5 controller, media/power-loss, durability, or external qualification is proposed. Native/physical status remains **NOT_RUN**.
