# Independent native global-aggregate review

## Verdict

**Accepted for the declared bounded global-aggregate scope at tree `e92277d411e0ca8702b5319566c7588549a9a517`. No unresolved review findings on that exact candidate.**

The original candidate `ba1f9611f9665db8481782e60f598ae5d0427a57` required changes. Independent probes found two correctness families; the owner corrected them, preserved reproductions, and the reviewer verified the immutable v3 source with the same regressions. This is an integration acceptance, not broad SQLite compatibility, a full combined-tree pass, milestone completion, or a physical-safety/release qualification.

Reviewed candidate: sibling `aggregate-global-final-v3`. Independent execution copy: sibling `aggregate-independent-copy-v3`. Baseline: sibling `boolean-affinity-lead-verify`, tree `1336a5d7b5e1a336fb01e012334cec776a61c4a7`.

## Resolved findings

1. **P2: aggregate LIMIT/OFFSET binding and runtime evaluation were interleaved.** Original `sqlite/relational_bind.go:481-505` evaluated OFFSET even after LIMIT became zero, evaluated runtime bounds under EXPLAIN, and could execute a failing LIMIT before validating OFFSET names/functions. Pinned SQLite returned an empty result for `SELECT count(*) LIMIT 0 OFFSET NULL`, `... OFFSET 1.5`, and `... OFFSET abs(-9223372036854775808)`, while native Query failed. `EXPLAIN QUERY PLAN SELECT count(*) LIMIT abs(-9223372036854775808)` also failed natively instead of emitting a plan. `SELECT count(*) LIMIT NULL OFFSET missing` masked the missing-column error with a runtime integer error. V3 validates both trees first, skips both runtime evaluations under EXPLAIN, and skips OFFSET evaluation after LIMIT=0. Name/arity/misuse checks are retained. All exact probes now pass, including table/join/empty inputs, computed zero and parameters.

2. **P2: embedded-NUL numeric TEXT could become REAL and hide required SUM overflow.** Original `internal/sql/aggregate.go:95-99` treated `CAST(x'310032' AS TEXT)` as REAL 1 in SUM; pinned SQLite returns INTEGER 1. With max-int, integer 1, then `CAST(x'300078' AS TEXT)`, the erroneous REAL transition cleared the prior integer overflow, producing a successful floating result. V3 terminates TEXT numeric conversion at the first NUL while preserving BLOB's distinct REAL conversion. Exact class and false-success regressions, leading NUL, decimal/partial prefixes, whitespace and same-byte BLOB variants all pass.

The two production fixes are confined to `internal/sql/aggregate.go` and the aggregate-only bounds block in `sqlite/relational_bind.go`. Nonaggregate bounds behavior is unchanged.

## Verified source and integration invariants

- COUNT(*), COUNT()/COUNT(expr), SUM/TOTAL/AVG and MIN/MAX use native Go state. No recursive DB.Query, extra connection, extra snapshot or extra transaction lease is introduced.
- Compared arithmetic to pinned SQLite 3.54.0 public `src/func.c`: integer overflow and later REAL clearing, large-integer splitting, compensated additions, empty/all-NULL results, infinity/indeterminate outcomes and finalizer types. Exact oracle checks also cover mixed TEXT/BLOB/numeric classes and 256 independent six-value decimal-TEXT sequences across normal/subnormal/extreme exponents.
- MIN/MAX use the existing BINARY/storage-class comparator, keep the earliest equal value, preflight work and reserve bytes before cloning selected TEXT/BLOB values, and retain the old charge until replacement succeeds. Public Values BLOB copies survive subsequent Next/Close independently.
- The root alone owns Rows registration, shared snapshot release and Tx activeRows. The private child uses internal fetch without locks/leases. Existing left join callbacks and every restarted right child are retargeted to the input scope before the root changes to aggregate output slots.
- One statement budget is created before bound-parameter cloning. Planning, source pages/records, ON/WHERE, aggregate arguments, coercions/comparisons, state, retained values, finalization and scalar output share it. Cancellation and pure-memory preallocation checks pass. This is conservative accounting, not an RSS bound or a claim of spilling.
- LIMIT 0 binds but opens no source cursor. OFFSET removing the sole output still scans/finalizes, preserving input and SUM overflow errors while suppressing final scalar projection errors. EXPLAIN validates without runtime-bound evaluation or source cursor reads.
- Prepared native/driver reuse, named/positional parameters, duplicate labels/aliases, integer ordinals, source affinity, Boolean fallback names, joined multiplicities/null extension, lazy typed-column validation and output lifetime are covered.
- Plans explicitly report AGGREGATE/global table scans and no index/ANN pushdown. Stats count actual leaf work and one root result. The aggregate route remains before ordinary single-source index/ANN planning.
- Four original capability-negative SQL strings remain exact positive complete-label/value/storage-class pinned comparisons: COUNT(*) FROM t, SUM(x) FROM t, COUNT(*) FROM samples, COUNT(*) FROM edge. Other original rejection assertions remain intact. Preserved pre-support failure evidence was inspected; no assertion weakening was found.
- Existing public sqlite/driver struct layouts and dependency/disk-format contracts are unchanged. The goal.md SHA256 remains `9ae7b1d776856669a10eb753408c7754d015d266f8783240165450e96e4bac81`.

## Final independent checks

All executed test selections were behaviorally reviewed and preceded by `go run ./internal/testscope`.

- PASS: expanded aggregate/parser/native/driver/pinned-oracle selection, adopted independent regressions, and the reviewer's separate full probe copy, including 256 decimal sequences.
- PASS: the same selection under the race detector.
- PASS: whole-tree `go vet ./...`, CGO-disabled `go build -trimpath ./...`, and `go mod verify`.
- PASS: CGO-disabled sqlite/driver production dependency closure contains no CgoFiles.
- PASS, compile only: core packages for linux/arm64, darwin/arm64, darwin/amd64, windows/amd64 and windows/arm64.

Toolchain: Go 1.27.2, GOTOOLCHAIN=local, GOPROXY=off, GOSUMDB=off, GOFLAGS=-buildvcs=false. The oracle helper verified the explicit pinned SQLite 3.54.0 executable/source ID. Exact final command script and output: `run-final-v3.sh`, `final-v3-checks.txt`. Exact additional probe source: `aggregate_reviewer_final_test.go.txt`; only this file was added to the independent candidate copy, and all 1,062 inherited file hashes stayed unchanged.

No independent full default or full oracle suite pass is claimed. Inspected owner v2 full-default evidence has exactly the three known baseline Boolean ANN/sparse failures, with original assertions preserved. Final v3 is aggregate-focused; the separately accepted rank repair and fresh complete combined-tree acceptance remain the integration lead's responsibility.

## Inventory and patch integrity

Before and after review, independently rebuilt candidate and baseline Git trees in isolated inventory repositories, verified all **1,062** candidate SHA256 entries, exactly **28** owned paths and **zero removals**. The production delta from the initial candidate is exactly the two corrected files above. A fresh baseline copy accepted the full patch and reconstructed every candidate file hash.

- Final tree: `e92277d411e0ca8702b5319566c7588549a9a517`
- Baseline tree: `1336a5d7b5e1a336fb01e012334cec776a61c4a7`
- Full patch SHA256: `a3ca18e68eaf7e9df19aa3d5143e2b639c479356d4fb0bc09f9e224853409040`
- Fix-only patch SHA256: `1f47b25db72fa94edb35fa22ccb4450359c8d186195d8ff3de3aea6a0f0fb6cc`

Inventory evidence: `initial-inventory-v3.json`, `after-inventory-v3.json`, `patch-verification-v3.json`. Original review evidence is retained in `review-v1.md`, `probes-v1.txt`, and `probes-explain-bind-nul-v1.txt`. The earlier `probes-explain-bind-v1.txt` contains three EXPLAIN harness failures caused by using the CLI quote parser on pretty plan output; those are explicitly not product failures. The corrected direct-CLI harness produced the genuine native errors and subsequently passed v3.

## Remaining boundaries

GROUP BY/HAVING, DISTINCT/FILTER/windows, aggregate-local/general aggregate ORDER BY expressions, bare-column aggregate projections and multiargument scalar MIN/MAX remain explicitly unsupported. Broader preexisting scalar, optimizer and schema limitations remain. Cross-target results are compilation evidence only.

No physical tamper/corruption, wrong-key, missing-file, namespace mutation, crash/kill/process-exit, hot recovery, native I/O/lock/unmap/close/Sync/short-write/disk-full/permission fault, interrupted publication or rekey workflow was executed. All database operations used intact owned ordinary SQL fixtures. No production edits, staging, commits or uploads were made by the reviewer; main-checkout writes are confined to this review-evidence directory.


## Combined-tree interaction addendum

At the lead's request, inspected the aggregate integration with identifier parser/binder changes and rowid planning in immutable combined tree `8d7a0fcc5a2e3a83850ea780b64b403047677568`. Verified all 1,225 hashes before/after and independently rebuilt its tree. Used a separate `aggregate-independent-combined-v1` copy with one added probe file only.

**PASS: 27 focused pinned-oracle interaction cases, normal and race.** Cases cover direct LEFT-ON alias promotion versus unary-plus/OR/comparison provenance, implicit FALSE versus explicit aliases, NULL/zero/one aliases, combined WHERE and repeated LEFT joins, aggregate-alias misuse in ON even under LIMIT 0, TRUE/FALSE qualified table aliases/physical columns/output aliases/function names, and aggregate routing ahead of ordinary rowid-range planning. Complete values/storage classes/labels and relevant error categories match. Every positive query still reports AGGREGATE/global.

Source inspection confirms the explicit-alias promotion happens during relation building before aggregate legality checks and input predicate compilation; promoted aggregate references are rejected. Private input Rows retains the resulting predicate scope, including right-side restarts. The parser's TRUE/FALSE fallback is now after function-call/qualified-name handling without changing the aggregate call branch. The rowid planner remains below the aggregate early dispatch.

No integration issue found in this requested focused scope. This does not extend the review to a full combined-tree suite or release gate. Sources/logs: `aggregate_interaction_review_test.go.txt`, `combined-interaction.txt`, `combined-interaction-race.txt`, `combined-initial-inventory.json`, `combined-after-inventory.json`. The same behavioral audit and internal/testscope guard preceded each tiny interaction run; no broad suite was started.
