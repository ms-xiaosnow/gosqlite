# Original-goal acceptance traceability audit

Date: 2026-10-10 UTC. Read-only engineering audit; no test, database program, fault workflow, commit, upload, or remote write was executed. Only this new sibling report is written. This is not an external security, cryptographic, legal, or native-OS qualification.

## Decision and immediate priorities

**Full goal acceptance is not established.** There is a real native database with substantial ordinary-path evidence, not an in-memory substitute. However, acceptance is blocked by both implementable code and unavailable qualification. These must not be collapsed into a single “manual testing remaining” statement.

Priority order:

1. Complete ownership/recovery composition and the remaining shared-path absence provenance; finish supported-query error-order and cached-schema corrections. Preserve failed controls and the exact source identities.
2. Implement mandatory user-facing outcomes that SQL expansion can obscure: safe CLI backup/rekey, sufficiently scoped check/inspect, and reusable per-import distance verification. Backup/rekey is actively being implemented according to the lead's 20:36 update.
3. Add the missing grams/transaction benchmark and real-duration ordinary soak harnesses, normalized randomized LIKE acceptance coverage, and performance-regression CI. These are implementation work, not unavailable external evidence.
4. Run ordinary integrated verification on the exact composed source, then distinguish conditional save/restore/CI success from acceptance.
5. Keep restricted physical recovery/security gates blocked. Obtain actual target-OS/independent evidence only through legitimately available, authorized channels; this report does not propose relocating restricted execution.

The required four-category classification is below. A feature can have code in 必须实现 and a separate acceptance test in 受限待人工; finishing one does not finish the other.

## 1. Audit identities and status vocabulary

All relative source/evidence paths below start in `durable-restart/`. `gosqlite/` is the main repository; named sibling freezes are separate immutable candidates. Read all 516 lines of `gosqlite/goal.md`, the complete current compatibility/progress/manual-verification documents, and the relevant source/tests/evidence cited below.

- Original goal SHA-256, independently read twice: `9ae7b1d776856669a10eb753408c7754d015d266f8783240165450e96e4bac81`.
- Requested starting durable commit: `13a214607c01c2b663dfc9d7e680050aff1bbaba`, tree `c405112b6076b5db7d16f1664923c455db6014ff`.
- During the audit, HEAD advanced to `f2bc1b0fdf39890aa8e4cd51ab6b35925010f3e4`, tree `edb1e2f148dc86b610feccfd48683ce066a3b529`, subject `fix: retain plaintext cleanup ownership`. This was observed through read-only Git metadata. Remote/backup/CI completion for that new commit is not independently asserted here.
- At audit start, the main worktree already contained staged plaintext ownership code and prospective documentation. Therefore a current worktree document was not treated as proof that a change belonged to `13a2146`. Durable and pending code are separated explicitly.
- Read snapshot document hashes: compatibility `4e23bd5e219f79edce64fc2a4f03b13866449d067794fade6d30b8507f74e626`; progress `640c5451475bbae79a94be3d87c13c64c1d564bf4276ff91a85dd68113ed3bcd`; manual-verification `f1e51b32603ce6fdab9c421ac345fed8ad9f48bd5a58cf204baaee63582334a7`. These are the audit's read versions, not a promise that concurrently maintained files remain unchanged.

Status meanings:

- **Durable code:** present in the identified main commit; only its recorded test scope is credited.
- **Reviewed pending:** independently accepted bounded source candidate, not yet credited as a completely integrated or durable result.
- **Active correction:** work in progress, including preserved failed parity tests. It is not acceptance-ready.
- **Missing implementation:** an actual API/path/harness/semantics gap. Documentation or manual labels do not fill it.
- **Ordinary verification pending:** a permitted test/measurement can be executed once its code and exact composition are ready.
- **Restricted/real-environment evidence:** separately identified in categories 受限待人工 and 需要真实外部环境.

### Reviewed source ledger

| Candidate | Exact source tree | What is established | What is not established |
|---|---|---|---|
| `plain-final-verify` | `7d3184f87165e11e12560884ec6e4f92c9bfd039` | Reader V5, rollback Tx V2, exclusive WAL V2, public metadata-admission V1; independent component review and full frozen ordinary aggregate exit 0 at 20:24:10. Subsequently promoted locally to `f2bc1b0` | Physical failures, recovery, target runtime, or exact new-commit remote/restore/CI acceptance |
| `secondary-final-verify` | `60d8a2d68c36c1a48333baf9af2c22eb5fb83188` | Reviewed leading-secondary inequality slice; genuine bounded B-tree candidates and source seeks | Full integrated terminal result was not present in the read log; compound/covering/reverse/order/join/statistics work is not supplied |
| `journal-final-verify` | `e825d3096dd521a730caaaca2f5c85af0e9d76de` | Composition includes reviewed finite-count multi-segment memory replay parser | Open integration, physical replay/hotness proof, ambiguous tails and full durability; full aggregate had no terminal result when read |
| `grouped-aggregate-freeze-v2` | `6774495e7289aa1cf7a652df673ba2610117900e` | Accepted bounded GROUP BY/HAVING, representative/ordering/rowid-stream corrections; 824 independent comparisons plus owner cases | Combined-tree full gate, arbitrary bare columns, DISTINCT/FILTER/windows, broader SQL |
| `sql-connection-counters-freeze-v1` | `7c8f7a266a63a713b22709268fdd4d25f1b875d1` | Accepted scoped connection counters and native/driver state, six page/mode oracle histories | Exact cached-schema preparation/BUSY parity; strict official control intentionally fails the bounded freeze |
| `recovery-cleanup-freeze-v2` | `201dca235ee217594d231f2f583d3ccf65aa8093` | Accepted four-path recovery ownership repair; managed pre-acquisition roots, journal-before-main retirement, pure-memory/source laws | Parser composition and actual recovery execution; no public recovery caller added |

Evidence: `plain-final-checks.txt/.exit`, main `docs/evidence/plain-final-*`; `secondary-final-tree.txt`, `secondary-final-checks.txt`; `journal-final-tree.txt`, `journal-final-checks.txt`; main `docs/evidence/secondary-range-independent/source-review.md`, `docs/evidence/grouped-aggregate-independent/REVIEW.md`; `sql-counters-independent-final-v1/docs/evidence/connection-counters-independent/README.md`; `recovery-cleanup-independent-evidence-v2/review.md`.

## 2. 必须实现 — code/deliverable work, in dependency order

### I1. Finish native ownership and usable recovery, without activating restricted execution

**Goal:** §§0 final definition, 3 principles 1/2/5/6, 4.1 (lines 159–164), M3, §§11.1/13.9.

Plain Reader/Tx/exclusive/public ownership has just been promoted. Recovery ownership is independently reviewed pending. Their distinction matters: `recovery-cleanup-freeze-v2/docs/plaintext-recovery-cleanup.md` says explicitly that the repair does not enable recovery from Open/SQL and that retry retires owners only, never replays/truncates/syncs/unlinks again. Keep the common admission and child-before-protector ordering when composing it with the new parser.

Remaining concrete work:

- Shared acquisition still has legacy broad `os.IsNotExist` decisions in `gosqlite/internal/pager/shared_session.go` (notably resolve/create and WAL/journal sidecar checks around lines 108–118, 152–155, 227–228). The main compatibility document separately identifies this remaining absence coupling. Port the reviewed initial-no-handle absence provenance; do not interpret a wrapped post-acquisition close/stat error as authority to create or ignore a sidecar.
- `internal/pager/recovery.go:RecoveryOptions/RecoverRollback` has no ordinary Open/OpenTx/SQL caller. Current normal opens refuse journal sidecars. Correct recovery is required by the final definition; a memory parser alone cannot satisfy it. Finish source-level recovery orchestration/admission and define the safe public/Open integration policy, while keeping restricted runtime paths gated until their required evidence exists.
- The new `journal-final-verify/internal/journal/replay.go:59–79` supports only complete, published, finite-count aligned segments. It still rejects super/master journals, sentinel/no-sync counts, legacy unspecified page sizes, duplicate originals, changed geometry, unpublished/ambiguous/trailing data. These are explicit unsupported profiles, not silently repaired files. Decide the final supported recovery profile before adding modes; do not infer every journal variant is mandatory from the word “rollback-journal.”
- `internal/pager/wal.go:openWALLocked` and shared cold-reconstruction policy still refuse incomplete/checksum-invalid current-generation tails. Safe handling of the required supported recovery cases remains code work distinct from testing. Preserving an ambiguous file is correct interim behavior, not completed transaction recovery.

**Completion evidence:** exact composed source identity and independent ownership/parser review, ordinary unaffected lifecycle regressions, source/compile-only checks for restricted paths, then separately qualified physical recovery. Do not wire unqualified automatic replay into the ordinary suite merely to claim integration.

### I2. Correct known supported-SQL semantics before expanding the grammar

**Goal:** G1 lines 31–35, §4.2, §11.1 differential values/NULL/order/error categories, §11.3 100% consistency within supported semantics.

- Planner error order is an **active correction**, not merely unrun verification. `planner-error-independent-review/evidence/final-source-verification.json` identifies reviewed candidate `9363c0313629a5db40a1948ba05481ef7aa8a227`. Its `final-oracle.log` ends FAIL: impossible rowid `IS NULL` predicates expose an overflow SQLite suppresses; LEFT-join predicates differ between an error and an empty result; streaming prefixes before an error differ. Preserve those exact SQL controls. A passing owner subset or a test printing `OPEN:` does not close these failures.
- Connection counters are reviewed pending, but their accepted profile explicitly differs under snapshot-acquisition BUSY from SQLite cached preparation. `.../connection-counters-independent/strict-official-busy-negative.txt` retains the strict failure; `sql-cached-preparation-work-v1` and `sql-cached-preparation-evidence/` are active correction evidence, not an accepted freeze. Ensure schema invalidation, transaction DDL/rollback, supported name/arity validation, and counter outcomes are bounded and checked together.
- GROUP BY/HAVING and leading secondary ranges are reviewed additions awaiting integration. Their existence must replace “not implemented” only once composed; broad grouping and optimizer parity must remain partial.

These corrections take precedence over CTE/windows/FTS5 or general join optimization. Reusable compiled-AST execution is not independently mandated by the goal: driver preparation may reparse, provided the advertised driver/error semantics are correct. Active-Rows restrictions and general SQLite interrupt semantics are documented partial boundaries, not evidence that database/sql is absent.

### I3. Close the mandatory CLI surface, especially backup/rekey

**Goal:** §3 target layout; §8.1 lines 289–301; §12 heading “必须提供的用户体验与文档”; §12.2 lines 432–445.

Exact §12.2 targets include:

- `gosqlite check <db> # integrity / index consistency`
- `gosqlite inspect <db> # format / page / feature / index metadata`
- `gosqlite backup <db> --out <target>`
- `gosqlite rekey <db> --key-provider ... # 凭据不能进入 process argv / logs`

The following sentence is controlling: “命令示例是目标行为；实现时确保参数设计不通过命令行明文密码/URL 泄露凭据，支持环境变量名、stdin 受控输入或系统密钥提供器等安全方式。” Exact flag spelling can therefore vary; the outcomes and credential-safety boundary cannot disappear.

`gosqlite/cmd/gosqlite/main.go:27–132` currently dispatches version/query/check/inspect/migrate only. Check requires an index name and calls `CheckExtensionIndex`; inspect emits FileInfo/schema metadata. No backup or rekey route exists in the audited durable code. The lead reports an active CLI implementation; it needs independent review and exact-source ordinary CLI tests, not another “missing” label once delivered.

The native encrypted operations already exist: `sqlite/encryption.go:Rekey`, `sqlite/encryption_copy.go:BackupEncrypted`, `RestoreEncrypted`, `ExportPlaintext`, `ImportEncrypted`. Reuse these, including absent-destination and complete-selected-head rules; copying the encrypted manifest alone is not a backup. Define provider acquisition without secrets in argv/DSNs/logs, cancellation/exit status, destination/source protection, and supported modes.

**Important scope distinction:** §12.2 does **not** expressly require an online plaintext backup API. General backup API is listed in §4.2's phased third batch. §8.1 directly requires encrypted checked backup/restore/copy. Delivering a documented supported encrypted backup/rekey CLI path closes that portion; the unqualified `<db>` target still needs an explicit plain/encrypted capability decision. A sound offline/snapshot plaintext path could satisfy a bounded CLI profile without implementing SQLite's entire online backup API. Do not claim plaintext online backup exists, and do not turn it into an invented immediate prerequisite.

Similarly, official-oracle `PRAGMA integrity_check` is not a native check implementation. For the final check/inspect target, either implement whole-database structural/logical/index verification for the declared profile and feature/index inspection, or explicitly settle the bounded acceptance meaning of “integrity / index consistency.” Current extension-only checking is real but narrower. This is an implementation/scope decision, not a manual-test gap.

### I4. Add reusable imported-dataset distance verification

**Goal:** §9.2(2), line 340: resumable batched CSV/NDJSON, quarantine, and validation of counts/PKs/dimensions/norms/**distance results**; §9.2(5), line 343: explicit exact-distance tolerance and ANN recall rather than exact-row identity.

The importer already genuinely verifies supplied-export fidelity, counts, PK provenance, component bits/dimensions, NULLs and norm statistics. Do not erase that credit. However `migration/postgres/import_verify.go:150–156` initializes `ExactDistances` and `ANNRecall` to `not_run; separate from data fidelity`; the ordinary import/verify route never upgrades those fields. `docs/import-pgembedding.md:236–245` acknowledges this.

The implemented test is meaningful: `import_distance_test.go:TestImportExactDistancesAgainstIndependentHighPrecision` imports 34 rows and compares four metrics to independent 256-bit arithmetic with `1e-12 * max(1,abs(reference))` tolerance. The 64-row two-format example separately measures ANN recall@4. These do not verify an arbitrary caller's imported dataset.

Add a bounded explicit verification API/CLI phase with query/reference identity, dimensions/metric, exact-vs-ANN separation, tolerance, fixed sample/full-set policy, actual counts and reproducible result report. It may be a separate command; it must be usable beyond a hard-coded example. Mark “not run” honestly until invoked. No PostgreSQL credentials or live server are needed for local mathematical/source-data verification.

### I5. Add missing benchmark and soak harness code

**Goal:** M10; §11.3 lines 396–400. Detailed evidence and category boundaries are in §4 of this report.

Only `benchmarks/ann/{main.go,README.md}` exists. Missing code includes repeatable grams build/time/space/correctness/P50/P95 reporting; transaction-throughput workload/reporting; a real-duration concurrent ordinary soak with invariant/model checks; and CI performance comparison with variance/confidence disclosure. Ordinary repeated unit/race tests and 5–60 second memory fuzz runs are not these deliverables.

Prepare WAL recovery measurement specification/harness **source only** where useful; its restricted execution remains blocked. Do not label normal Close/reopen/checkpoint latency “WAL recovery time.” Native OS timing/failure qualification is separately classified.

### I6. Close exact LIKE corpus traceability without changing correct semantics

**Goal:** §5.2 line 217: “使用经规范化的随机文本产生 ≥1 万组合用例”, indexed LIKE results identical to no-index scan, real 1/2/3-character plans, zero stale postings after DML/checkpoint.

`sqlite/plan_grams_test.go:57–128` creates 128 texts (24 fixed edge strings plus 104 seeded random strings), then executes 100 seeded pattern/result-set comparisons against `NOT INDEXED`: **12,800 query/text membership comparisons, not 12,800 queries**. The goal says combinations, not 10,000 query executions. This can satisfy the stated counting unit; inventing a 10,000-query requirement would be wrong. Random-text pair memberships alone total 10,400.

But that fixture inserts raw strings, including decomposed sequences; it does not establish that the randomized corpus is normalized. `DefaultLikeConfig` deliberately uses raw ASCII folding, and `docs/grams13.md:34–40` correctly separates LIKE_RAW_V1 from NFC MATCH_CONFIG_V1. Do not normalize the production LIKE path: that would create false negatives. Add a clearly labeled deterministic NFC-normalized randomized **input corpus** with at least 10,000 pair combinations, preserving the existing raw/decomposed adversarial tests. This resolves the wording ambiguity cheaply without inflating counts or weakening assertions.

The dedicated 1/2/3 plan test and ordinary lifecycle reconciliation already exist. Their physical recovery guarantee is still separate.

### I7. Consolidate the required current documentation

**Goal:** §§0(2/5), 2, 12.3, 14. All named documents exist; some current-tense content is stale.

Examples from the audited `docs/architecture.md`: line 37 says vector-database fusion remains open, while line 41 describes it implemented; line 58 says simultaneous encrypted writer/readers remain code work; lines 75–76 call typed SQL future; lines 83–84 say Darwin/Windows backends are missing. Actual current source/matrix contradicts those claims. `docs/security.md:33` likewise still says non-Linux encrypted implementation is outstanding, while its opening/current platform discussion says it exists but is unexecuted. `docs/plans/README.md` needs pending/durable ownership and reviewed additions reconciled after integration.

Preserve historical evidence and failed transcripts; label chronology explicitly instead of rewriting results. Add a central THIRD_PARTY_NOTICES pointer for retained `testdata/pgembedding-0.3.6/{LICENSE,NOTICE,embedding--0.3.6.sql,embedding.control}`. Exact notices are present, so this is indexing/provenance completion, not a finding that licenses were omitted or a legal opinion. Final API/CLI docs must match executable routes and accepted source identities.

## 3. 可自动验收 — allowed verification once the corresponding code exists

| Requirement / goal clause | Current evidence and exact next evidence |
|---|---|
| Pure-Go module, build, driver, source baseline (§§0–3, G1, M0) | `go.mod` pins Go 1.27.2 and x/crypto/x/sys/x/text; driver registers gosqlite and exposes code Connector. `docs/upstream.md`, `scripts/build-oracle.sh`, fixture manifests and notices pin official SQLite 3.54.0/public references. Frozen aggregates record CGO0 unit/vet/build, separate CGO1 race, module/no-CGO closure and target builds. Recheck the final composed source. No new dependency or production delegation was found in the inspected production sources. |
| SQLite format and two-way ordinary interoperability (§4.1, M1/M2, §11.1) | `internal/record`, `internal/btree`, `internal/pager`, `sqlite/write_schema.go:validateCatalog`, `internal/btree/ownership.go`; official 512/4096/65536-page fixtures and ordinary read/write/oracle integrity comparisons. Re-run full pinned ordinary oracle after composition. Unsupported UTF-16/WITHOUT ROWID/autovacuum/etc stay explicitly rejected; no unqualified format promise. |
| Transactions, snapshots, state, cancellation (§3, M2/M3, §11.1) | Actual savepoint histories in `internal/testkit/transaction_control_conformance_test.go`, native/driver lifetimes and rollback tests; live cooperative official reader/writer tests. Existing B-tree randomized models are not a substitute for a broad transaction-history model. Extend ordinary history/model coverage in the soak harness; keep abrupt disconnect/recovery in restricted scope. |
| Text/similarity correctness (§§5–6, M4/M5) | Real posting B-trees, original-predicate recheck, complete candidate union or honest scan; source/search/rebuild/dirty-external-writer tests and independent scoring. Add I6 normalized-input corpus; re-run unchanged 1/2/3 plans, DML/savepoint/checkpoint consistency, exact threshold/Top-K and same-snapshot RRF tests. No pg_bigm binary/score claim is needed. |
| Dense/typed vectors and ANN (§7, M6/M7/M9) | Public/driver Vector adapters and typed storage, four dense metrics, independent math + pinned pgvector fixtures, actual persistent HNSW/IVF, exact fallback and format-preservation fixtures. Re-run declared opclass/metric, update/delete/reindex/reopen, typed and legacy-format tests; perform larger ordinary measurements with the existing harness after defining workload. |
| Correct-key encryption lifecycle (§8.1; ordinary portion of M8) | `sqlite/encryption*.go`, `internal/encstore`, native/driver examples; correct-key normal create/read/write/rekey/copy/backup/restore/import/export and known-answer memory crypto tests. Add CLI normal lifecycle/provider-redaction tests, preserve source bytes and formats. This does not exercise wrong-key/tampered disk or interrupted publication. |
| Offline migration (§9, M9) | Identity-aware inspector, typed AST converter, new-destination CSV/NDJSON import, normal committed-stop resume, review-only transition generation. Re-run original Manhattan-to-L1, non-vector/string/comment preservation, report determinism, quarantine/readback, and newly added per-dataset verification. No live PostgreSQL is needed to verify local conversion mechanics. |
| Release measurements (§11.3) | Run ordinary grams/transaction/soak harnesses after implementation. Existing ANN harness can be used without a new engine. Retain hardware/toolchain/page/cache/seed/source identifiers and actual wall-clock duration; do not infer large/long from a flag maximum. |
| Exact conditional checkpoint evidence (§0(4/5), §14) | `docs/checkpoints.json` records source, committed bytes, backups/restore and CI separately. At the read snapshot, `13a2146` CI/restore were still pending; `f2bc1b0` was newly observed. Monitor the exact commit jobs and read terminal outcomes. Do not transfer a prior commit's CI to a later source or call a reviewed patch a whole-tree PASS. |

The ordinary workflow now has two jobs (`.github/workflows/check.yml:6–30`): guarded unit/vet/build/race and a complete pinned ordinary oracle. Earlier CI lacked the oracle; the historical two obsolete JOIN/sort assertion failures remain failures of their original runs. `docs/test-scope-audit.md` also qualifies early full suites that inadvertently included namespace mutation. Current clean scoped evidence must be used rather than retroactively relabeling those runs.

## 4. Quantitative audit: actual measurements versus requirements

| Goal requirement | Actual measured evidence | Classification / remaining action |
|---|---|---|
| ≥10,000 normalized randomized LIKE combinations (§5.2) | 100 full comparisons ×128 texts =12,800 memberships; 104 texts random. Separate 12,000 escaped-literal memory trials; real 1/2/3 seeks asserted | Counting unit is compatible with “combinations”; normalized randomized persisted-input coverage is not explicit. I6 adds that test. Do not call this a 10,000-query numerical failure. |
| HNSW default recall@10 evaluation target ≥0.95, dataset-specific (M7) | 2,048×12 finite float32 rows, 48 held-out queries, seed 20261010, m16/ef_construction64/ef_search40: @1=1, @10=1, @100=.999375; zero exact fallback | Existing small fixed dataset meets that evaluation target. It is not a universal recall guarantee or M10 large-scale result. |
| Measurable effective indexed path vs exact (M7) | HNSW @10 source fetches40 vs2048; graph distances575.083 vs2048; actual persistent graph after reopen | Genuine reduced logical work is established. HNSW p50 40.619529 ms vs exact2.120004 ms is slower; no latency speedup can be claimed. The goal does not impose a numerical latency win. |
| ANN recall@1/10/100, build/time/size (§§7.2,11.3) | `m7-hnsw-persistent-benchmark.json`; build49,382.536143 ms; DB1,462,272 vs286,720 bytes before. Same corpus IVF lists100, probes1/10/50 @10=.2541667/.7708333/.9958333 | Existing harness and measurements exist. Larger/high-dimensional/filtered/update workloads and reproducible hardware qualification are ordinary pending measurements, not missing basic ANN implementation. IVF .95 is not a universal/default requirement in the goal. |
| Fixed hardware/version/page/cache/data/seed (§11.3) | Go1.27.2, Linux/amd64, page4096, seed/hash, warm cache, nine visible CPUs; README names EPYC host. Shared-cloud timings, no confidence intervals/cold-cache/isolation claim | Define repeatable measurement conditions and uncertainty. “Cold/hot conditions” requires clear labeling; do not invent a requirement to flush OS caches. Current runs establish only their labeled warm condition. |
| Large benchmarks (M10) | Only ANN benchmark package; measured2,048 rows,48 queries. Flags permit100..100,000 rows and1..10,000 queries (`main.go:90–103`) | Flag limits are not measured workload sizes. Goal gives no numeric definition of “large”; choose and justify a release profile before measuring. Existing harness may hit legitimate engine budgets at larger sizes; report rather than truncate workload. |
| 1/2/3-gram build time/space/correctness/P50/P95 (§11.3) | Ordinary correctness and work-counter tests only; no benchmark harness/output found | **Missing harness code** then ordinary runnable measurement. Must not be hidden beneath ANN results. |
| Transaction throughput (§11.3) | Many functional/race/oracle transactions; no fixed-workload throughput report/harness found | **Missing harness code** then ordinary runnable measurement. Include supported rollback/shared/encrypted modes and real contention; preserve invariants. |
| WAL recovery duration (§11.3) | No qualified physical recovery measurement; normal retained committed WAL reopen is recorded separately | Recovery implementation/harness-source gap plus **restricted measurement**. Never substitute normal checkpoint or reopen timing. Do not run or relocate the restricted workflow. |
| Long concurrent soak (§11.3) | No soak source/report found. Unit/race runs lasting minutes and memory fuzz of5–60 seconds are real but unrelated | **Missing harness code**, then ordinary real-duration test. Goal specifies no exact hours, rows or ops; predeclare a justified duration/workload/invariants. Failure injection remains outside it. |
| CI performance regression with confidence (§11.3) | Current workflow runs correctness/race/oracle, no benchmark comparison job | **Missing CI/harness integration**, not just waiting for a measurement. Establish ordinary baselines and thresholds/variance policy; do not add restricted recovery jobs. |
| Migration distance/ANN evidence (§9.2) | 34-row independent exact-distance test,64-row two-format import example with one recall@4 query | Good fixture evidence; neither a large benchmark nor caller-dataset verification. I4 supplies the missing reusable verification step. |

Benchmark raw evidence: `gosqlite/docs/evidence/m7-hnsw-persistent-benchmark.json`, `m7-ivf-benchmark-probes{1,10,50}.json`; methodology: `gosqlite/benchmarks/ann/README.md`. Do not sum unrelated oracle/fuzz comparisons into benchmark dataset size or independent physical acceptance.

## 5. 受限待人工 — platform-restricted qualification, never a code-gap substitute

The user explicitly requested `docs/manual-verification.md` with concrete steps and commands for their later independent testing. Preserve that authorized documentation, including its existing commands and precise prerequisites. The instructions themselves are not prohibited or erased by this audit. The boundary is that the agent must not execute or relocate restricted workflows, seek approval as a bypass, or fabricate acceptance from documentation. No new request for the user to run them now is needed.

| Required goal acceptance | Exact missing evidence / boundary | Existing reference |
|---|---|---|
| Transaction durability and recovery (M3; §4.1; §11.1) | Forced exit/power loss, interrupted journal/database publication/unlink/directory sync, short/wrong-offset writes, failed Sync, disk-full/permission faults, hot recovery, interrupted WAL/checkpoint. No accepted no-loss/no-partial-transaction result | `docs/manual-verification.md:Current state`, `M3 explicit recovery harness`; `internal/pager/recovery_manual_test.go` is compile-only here |
| Text/vector index crash consistency (M4, M6, M7; §§5/7/11.1) | Post-crash source/posting/ANN same-transaction consistency and restart recovery. Normal rollback/reopen/checkpoint and snapshot laws cannot establish it | `docs/manual-verification.md` M4/M6/M7 sections; `docs/grams13.md`, `docs/ann.md` |
| Encryption negative physical acceptance (§8.3; M8; §11.1) | Wrong-key/salt/tampered-on-disk/tag/cross-page/main-WAL mismatch, interrupted publication/rekey and physical recovery. Memory AEAD/parser negatives are valuable but do not qualify storage paths | `docs/manual-verification.md` M8 sections; `docs/encryption-storage.md`; `encryption/*_test.go` memory vectors/fuzz |
| Native exceptional resource retirement (G1 resource closure; §§3/4.1) | Real failed native close/unmap/lock/Sync and active namespace/selected-file failures. Descriptor-free state laws and foreign binary compilation are not execution proof | Plain/recovery/shared/encrypted cleanup independent reports; manual verification native sections |

The exact restricted scope is already documented and preserved. Some harnesses do not yet exist beyond the finite replay harness; that missing **source implementation** belongs in I1/I5, while runtime remains blocked. Do not claim that every restricted gate is merely awaiting a command already implemented. Do not advise running these tests on another computer/CI as a workaround.

No approval request can manufacture this evidence or bypass the agent's execution boundary. Valid evidence from the user's later independent testing, or other independently supplied authorized evidence, can be evaluated if it actually identifies the same source/profile; until then, the evidence remains outstanding. The already requested concrete manual-verification instructions must stay intact. Later developmental progress does not waive the acceptance gates.

## 6. 需要真实外部环境 — actual platform/service or independent review

| Goal clause | What is needed | What existing evidence does and does not prove |
|---|---|---|
| §11.3 target builds/platform tiering; M3/M10 | Actual ordinary native Linux/arm64, Darwin/arm64 and Windows/amd64 execution on supported configurations; native locking/interoperability policy reviewed at appropriate tier | Required no-CGO cross-builds exist. Runtime here is Linux/amd64 only. Darwin fcntl/deferred close and Windows LockFileEx/mapping/identity code exist; runtime remains unverified. Windows mapping cross-vet warning is still disclosed, not a clean gate. Restricted physical/native-failure subsets remain in §5 even if a machine becomes available. |
| §8 and supported encrypted platform profile | Darwin Unicode filesystem behavior and Windows shared rekey need implementation first, then suitable native execution | `internal/encstore/platform_darwin.go:44–55` intentionally restricts root basenames; Windows `platform_windows.go:85–86` rejects shared rekey. These are **code gaps**, not merely unavailable runtime. Parent should keep them in the implementation backlog; broader multi-platform encrypted support must not be inferred from successful builds. |
| §9.2 and M9 reference validity | A real PostgreSQL/Neon test installation/catalog for claims about generated migration SQL, extension capabilities, exporter correctness, dependency completeness and real cutover/rollback behavior | Offline generation/local import are implemented and useful. No PostgreSQL/Neon SQL was executed. Goal defaults templates to generation only, so live execution is not permission to mutate a user's server and is not required to prove every local converter function. Final remote-workflow validity remains unestablished. |
| §§8.2/8.3/11.3 and M10 | Independent data-safety, cryptographic/threat-model, dependency/license and API review before stable/production claims | Internal independent engineering review is source/test review, not the external qualification. Source/fixture licenses and crypto known-answer tests exist; audits are not thereby passed. No legal opinion is made here. |
| M10 large fixed-hardware results | A controlled/repeatable declared measurement environment if shared-cloud observations cannot support release claims | Existing ordinary ANN observations are valid at their documented conditions; unknown hardware stability/confidence cannot be repaired by changing labels. Harness development and allowed local measurement can proceed meanwhile. |

Neon reference ambiguity is already correctly disclosed: current page retrieved on 2026-10-10 lacks the requested migration chapter; the pinned official 2024 snapshot is identified in `testdata/pgembedding-0.3.6/README.md` and `docs/upstream.md`. Do not call the missing current chapter followed verbatim. No user decision is needed to preserve that provenance honestly.

## 7. Capability/acceptance summary against M0–M10

- **M0:** native module, pinned Go/SQLite sources, CI/ordinary framework, CLI skeleton and source records exist. Exact current commit CI/restore must finish independently.
- **M1:** actual SQLite header/record/B-tree/overflow/schema reading with real oracle fixtures, three page sizes and source-hash preservation exists for the declared subset. No broad format claim.
- **M2:** genuine native mutable table/index pages, split/merge/freelist/overflow, CRUD, transaction/driver integration and official integrity interoperability. Ordinary MVP evidence exists; failure durability is excluded.
- **M3:** actual shared/exclusive WAL, snapshots, checkpoint, ordinary multi-connection/cooperative cross-process behavior. Recovery integration, remaining ownership provenance and qualification stay open. This blocks full foundation acceptance.
- **M4:** persisted legal shadow tables/B-trees and same-transaction maintenance, real plans, raw LIKE recheck, NFC configurable MATCH and external-writer dirty fallback exist. No virtual-table syntax is required where the permitted project syntax is accurately documented. Normalized corpus traceability and physical recovery acceptance remain.
- **M5:** Dice/Jaccard/rune Levenshtein, exact threshold/Top-K candidate completeness, native/SQL scores, same-snapshot business/text/vector RRF and deterministic ties exist. `bigram_similarity` is correct naming without a pg_bigm equivalence claim.
- **M6:** float32/vector codec, typed binding, four metrics, exact SQL/native KNN and retained upstream mathematical fixtures exist. Published arithmetic differences are explicit; physical restored-data acceptance remains separate.
- **M7:** genuine persistent HNSW and IVFFlat with matching metrics, maintenance, diagnostics and exact fallback exist. Small measured recall/work is real; large benchmark/soak/recovery qualification is incomplete. Sparse IVF/IVF L1/bit Jaccard are declared unsupported extension profiles, not mandatory universal combinations in the original phased goal.
- **M8:** authenticated independent container/KDF/providers, encrypted dirty/shared WAL, ordinary rekey/copy/backup/restore and plaintext conversion exist. Windows shared rekey/Darwin Unicode and mandatory physical negatives/recovery/external review remain. Retained history/old-key decryptability/no external freshness/no secure erase are disclosed risks the goal expressly requires documenting, not silently claimed protections.
- **M9:** identity-aware inspector, typed AST conversion, local import/resume/fidelity and review-only migration templates exist. Caller-dataset distance verification remains an integration gap; real server validity and unsupported dependency/online families are separate. The goal makes pgx streaming extraction optional; absent `--pg-dsn-env` is not a hard missing-core requirement when CSV/NDJSON exists. Full-table/online/dependency-complete remote migration is broader than the minimum generated-shadow-template workflow and must not displace core acceptance work.
- **M10:** not complete. Missing ordinary harnesses/measurements/CI, real-duration soak, target runtime and independent reviews are specifically enumerated above. No stable/production or complete-goal claim is justified.

## 8. Exact scope ambiguities and genuinely necessary user actions

1. **Phased SQL is explicit.** G1 and §4.2 use progressive capability matrices; §§0/10 allow staged work. CTE/UPSERT/triggers/views/windows/JSON/FTS5/etc remain a legitimate backlog, but the goal does not fix an exhaustive SQLite grammar that must all land before any bounded release. Do not silently remove them from the goal either. Define the final declared supported profile and require 100% tested consistency within it. Known errors inside that profile are higher priority than breadth.
2. **LIKE counting:** combinations are not executions. Resolve normalization coverage with an additional ordinary corpus test; no user clarification or new permission is needed.
3. **CLI:** exact syntax is illustrative; backup/rekey functionality is a target. Online plaintext backup is not explicitly a MUST. The accepted plain/encrypted/check profile must be stated, and a genuinely desired broader final scope can be decided with the user only if it cannot be inferred from the work. Do not ask before implementing the already authorized supported encrypted path.
4. **Large/long/confidence:** the goal supplies no numeric dataset, dimensions, soak hours, performance threshold or confidence level. Engineers can propose and predeclare a defensible ordinary release profile, then measure it. Ask the user only when a workload/SLO or machine/cost commitment belongs to them; do not invent a threshold and present it as original contract wording.
5. **No new user action is needed** for permitted source fixes, ordinary tests, CLI/import verification/harness work, document reconciliation, or monitoring already authorized exact-SHA conditional stage CI/restore.
6. **Real access/review decisions may eventually need the user:** provision/authorize the genuinely needed target native platform or disposable PostgreSQL/Neon test environment; appoint independent security/cryptographic/license reviewers; approve any cost/access or external state change not already authorized. Explain the exact purpose and minimum access. Never request production credentials in chat and never treat template generation as permission for destructive SQL.
7. **Preserve authorized manual instructions without turning them into an execution workaround.** The user's explicitly requested concrete steps/commands for later independent testing remain in `docs/manual-verification.md`; this audit does not forbid or erase them. No new request to run them now is needed. The agent must not execute or relocate the restricted workflows, seek approval as a bypass, or fabricate results. Valid same-source evidence remains outstanding; until it exists, final acceptance remains pending even if all implementable ordinary work is completed.

The lead should record these distinctions in the next progress checkpoint, retain the original goal unchanged, and continue conditional developmental saves without a completion claim.
