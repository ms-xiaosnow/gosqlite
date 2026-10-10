# Independent harness V3 re-review

## Verdict

**V3 remains blocked by one closure-report gap.** The seven original joined-error classification failures are fixed and independently pass. The run-scoped DB ledger fixes the earlier reader-close failure overwrite and avoids treating a second nil Close as retirement proof. However, pending-retirement diagnostics already returned by SQL APIs are not recognized by the new cleanup latch, permitting an incorrect normal-closure report. Four new descriptor-free memory probes reproduce this; no native fault was induced.

## Exact source/replay verification

- Harness-only freeze: `../transaction-soak-freeze-v3`, tree `73237e2c1e593722dfc375f39e4498013df48fea`.
- Plain baseline: `7d3184f87165e11e12560884ec6e4f92c9bfd039`; all1252 baseline files are unchanged. Exactly10 harness files are added (1262 total).
- Full patch SHA256: `95c546d8adf2980dfe9adc440980d09534706c58f24eb9d6c98bfe3128830322`.
- V2-to-V3 patch SHA256: `eef9c7296af43b90fe703d4d82098ef394761efa547fb76f52b7f758765f4485`.
- Runtime-only reader-fix composition: `../transaction-soak-fixed-freeze-v3`, tree `20c676788917ff59c58228a77a463e5f75662b6b` (1264 files). All10 harness files are byte/mode-identical to the harness freeze.

Independent full Git-object/tree reconstruction, full SHA256 inventories, exact filesystem/Git modes, full baseline patch replay and V2 delta replay all pass. Both replay trees equal the immutable harness tree. Focused execution uses a separate exact fixed-composition copy. Six tree inventories rechecked unchanged after execution; only the separate negative copy adds reviewer source. Evidence: `identity.json`, `*-inventory.json`, `post-run-source-verification.json`, and `../transaction_soak_independent_identity_v3.py`.

## Remaining blocking finding

`benchmarks/transactions/closure.go:75–114` recognizes only the harness's errCleanup marker, not public SQL cleanup diagnostics. It traverses a sqlite.Error but ignores its CodeIOErr, then ignores the pending/poisoned leaves or opaque native token below it. Consequently these public diagnostic shapes are not latched:

- sqlite.ErrPlaintextCleanupPending
- sqlite.ErrOpenCleanupPending
- sqlite.ErrCloseCleanupPending
- sqlite.ErrEncryptedCleanupPending

The pure-memory probe wraps each marker plus sqlite.ErrPoisoned in sqlite.Error{Code: CodeIOErr}, stores the original diagnostic in Report, and finalizes an otherwise clean fake writer. All four cases finish with unverified_retirement=false and shutdown=workers_joined_normal_close. Status remains failed; this finding concerns false retirement evidence, not an unexpected passing status.

This shape is supported by the SQL API and is relevant without a subsequent failing Close. For example, sqlite/rows.go:close is idempotent once closed; Rows.fail stores the original retirement error in Rows.Err via wrapCleanup. A later Rows.Close may return nil. Existing source-only tests in sqlite/plaintext_cleanup_internal_test.go assert precisely this composed CodeIOErr/cleanup preservation. The harness's subsequent cleanupJoin(nil-close) therefore adds no errCleanup marker, leaving the original diagnostic unrecognized.

Initial Open has an additional reporting path: main.go:277–283 returns before the finalizer at291–295 is installed. Merely making cleanupDiagnostic true is insufficient to guarantee a truthful shutdown value there. A later errors.Is(ErrUnsupported) can also overwrite status with skipped_unsupported; a latched retirement failure must not be downgraded. Centralize failure/retirement reporting for both pre-handle and ordinary-return paths, conservatively recognize public pending diagnostics/CodeIOErr without invoking custom error callbacks, and retain original token references. No token retry is requested by this finding.

Reproduction in `pure-memory-negative` with the pinned/offline environment in checks.sh:

`go run ./internal/testscope`

`go test ./benchmarks/transactions -run '^TestIndependentPendingRetirementDiagnosticsLatch$' -count=1 -timeout=0 -v`

`negative-guard.exit`=0; `negative.exit`=1. Test source: `pure-memory-negative/benchmarks/transactions/independent_retirement_test.go`. Full output: `negative.txt`. Every object is in-memory and descriptor-free; no database is opened by the probe and no native operation is made to fail.

## Repaired areas independently assessed

The classification function visits all branches within128 nodes. It accepts exact known BUSY/context sentinels through recognized standard fmt/errors.Join wrappers and consistent SQLite diagnostic envelopes. Unknown wrappers, unsafe/inconsistent SQLite codes, nil/malformed nodes, custom callbacks and over-budget/cyclic graphs fail closed. It never calls a custom Is, As, Error or Unwrap. Mixed known BUSY/cancellation becomes canceled and cannot trigger commit retry. ordinaryBusy and classify use the same decision. All seven original negatives and graph boundary controls pass independently.

The missing-generation helper now preserves an iterator error intact instead of appending a fabricated missing-row diagnostic to cancellation. Clean absence remains fatal.

Every successful DB acquisition is registered in the ledger; DB Close is centralized. A failed handle/error remains in the ledger; no blind second Close or automatic retry/sweep occurs. The ledger is single-goroutine-owned after workers join. Finalization closes only unattempted handles and preserves earlier failures. Public closure fields expose failed handle counts; private Report references plus runtime.KeepAlive retain failed handles and original diagnostics through JSON emission. This reference retention and process completion are explicitly **not native retirement proof**, as required by the lead's scoped terminal contract. The remaining diagnostic-recognition issue above prevents full closure acceptance.

The workload/model, ordinal/outcome trace, elapsed/active separation, bounded resources, histogram and confidence semantics are unchanged apart from these corrections; the detailed V2 review assessment continues to apply. No quota-ended soak is accepted.

## Independent executed gates

Behavior review precedes execution in `AUDIT.md`. Pinned sibling Go1.27.2; offline caches; GOTOOLCHAIN=local, GOPROXY=off, GOSUMDB=off, GOFLAGS=-buildvcs=false; CGO0 except race CGO1. Test process timeout disabled; existing native tests use cooperative contexts.

On the unchanged fixed-composition copy:

- Guard: PASS.
- Full benchmarks/transactions selected package, count1: PASS, including original seven negatives, bounded strict graphs, descriptor-free ledger/report probes, AST closure wiring, ordinary shared lifecycle and cooperative cancellation.
- Same focused package with race, count1: PASS (1.266s).
- Focused vet: PASS.
- Separate new pending-retirement probe: FAIL in all four cases, as above.

Exact commands/status logs: `checks.sh`, `checks.txt/.exit`, `normal`, `race`, `vet`, `guard`, `negative` artifacts. The overall script exit0 reflects completion of ordinary gates after preserving the expected independent negative exit1; it does not mean review acceptance. No extra native profile was run by this reviewer.

## Reports and preserved failures

The independent read-only `verify_reports.py` reconstructs all four owner V3 reports' exact final model/generation, every committed deterministic transfer, trace and savepoint counts, global/window outcome counts, elapsed/active timing, throughput arithmetic and histogram upper quantiles/null confidence bounds. All four have actual clean recorded closure fields. Verification passes in `report-verification.json` and `.txt/.exit`.

- Rollback requested3s: passed,3.005099307 active seconds,1404 commits.
- Encrypted requested3s: passed,3.000951392 active seconds,328 commits.
- WAL requested3s: **failed**,1.675996218 active seconds,1056 commits; observed17497216 bytes exceeded16777216-byte guard. It remains failed/incomplete.
- Separate WAL64-attempt profile: passed,0.063013346 active seconds,48 commits. It does not replace the failed duration sample.

The verification artifact records original report hashes and every retained file/hash for the failed WAL fixture. The original V1 rollback failure remains failed at14.015854093 seconds, final_model_verified=false; both report SHA256 `a62f4bfed390582d1125d88a6dfbe06f06f3a33500a55fedacea934424a1e101` and closed-data SHA256 `b996a714bd220924cd93370ca257c553f17ec775159a33382606b9b29ac773e9` were reverified unchanged. Prior V2 findings/negative evidence are not relabeled as passed.

## Limits

No long soak, controlled-host confidence-qualified benchmark, native-failure/recovery qualification, or foreign-platform runtime qualification is established. Three-second and20-second samples are bounded ordinary evidence only. No broad old-base/full suite, manual tag, physical corruption/tamper/wrong-key/missing-file, namespace mutation, crash/kill/abrupt DB-exit test, native I/O/lock/unmap/close/Sync/shortwrite/disk-full/permission failure, interrupted publication/rekey, cleanup, main or remote write occurred. Future immutable repair must receive its own identity and review; V3's failing retirement probes stay preserved.
