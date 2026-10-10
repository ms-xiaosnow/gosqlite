# Independent global aggregate review, v1

Candidate: `ba1f9611f9665db8481782e60f598ae5d0427a57` (all 1,058 files verified). Baseline: `1336a5d7b5e1a336fb01e012334cec776a61c4a7`. Exact 24 owned deltas, no removals. Patch SHA256: `14c27a7b2812cab16a0590b8676826e2f6994a9a7ae1ddd8f5d2a06bf0ba6305`.

Result: changes required. No production files, stages, commits or uploads were made by the reviewer. Probes exist only in the independent sibling copy; review evidence is confined to this directory.

## Findings

1. P2, `sqlite/relational_bind.go:481-505`: aggregate LIMIT/OFFSET compile-time validation and runtime evaluation are interleaved. `SELECT count(*) LIMIT 0 OFFSET NULL`, `... OFFSET 1.5`, and `... OFFSET abs(-9223372036854775808)` return zero rows in the pinned official SQLite but error natively. Same result with computed-zero LIMIT, table, join and empty input. EXPLAIN also evaluates the runtime bounds before its branch, e.g. `EXPLAIN QUERY PLAN SELECT count(*) LIMIT abs(-9223372036854775808)` errors instead of producing a plan. Finally, invalid OFFSET names/arity can be masked by an earlier LIMIT runtime error, e.g. `SELECT count(*) LIMIT NULL OFFSET missing` returns a type error rather than `no such column`. Validate both bounds first; runtime bounds must be omitted under EXPLAIN, and OFFSET runtime evaluation omitted after LIMIT=0. Keep name, arity and aggregate-misuse checks on dead paths.

2. P2, `internal/sql/aggregate.go:95-99`: NUL-containing numeric TEXT is misclassified as nonnumeric TEXT, choosing REAL instead of SQLite INTEGER. `SELECT sum(CAST(x'310032' AS TEXT)),typeof(sum(CAST(x'310032' AS TEXT)))` is native REAL 1 but SQLite INTEGER 1. This also hides a mandatory integer overflow: `SELECT sum(CASE id WHEN 1 THEN 9223372036854775807 WHEN 2 THEN 1 ELSE CAST(x'300078' AS TEXT) END) FROM a` errors in SQLite but succeeds natively with 9.223372036854776e+18. The test fixture has six rows, ids 1 through 6. Clip TEXT numeric conversion at its first NUL; preserve all BLOB coercions as REAL.

## Evidence and scope

- `initial-inventory.json`: complete hashes/delta and independently rebuilt Git trees using an isolated inventory repository.
- `focused.txt`: original aggregate arithmetic/parser/native/driver/pinned-oracle tests pass.
- `probes-v1.txt`: independent LIMIT-zero failures and NUL class failure; other numeric edges and joined scopes pass.
- `probes-explain-bind-v1.txt`: binding-order reproductions; initial EXPLAIN harness incorrectly used the quote parser for CLI plan output, subsequently corrected. Do not treat those three harness errors as product failures.
- `probes-explain-bind-nul-v1.txt`: corrected EXPLAIN reproductions, validation ordering, and NUL overflow false success.
- `aggregate_independent_review_test.go.txt`: exact independent probe source.

Reviewed pinned SQLite `src/func.c` sum state, compensation, split integers, overflow clearing and finalizers against the Go implementation. Apart from the NUL numeric classification issue, fixed-size arithmetic matches the inspected contract. MIN/MAX reserve before cloning and detach selected TEXT/BLOB values; original state remains charged through replacement. One root Rows owns the snapshot/transaction lease; internal child uses private fetch and owns neither. `aggregateInput` retargets existing left callbacks and all future right restarts. One budget is created before parameter cloning, and child leaves, expressions, aggregate state, result evaluation, cancellation and retained bytes share it. Plans explicitly state global table-scan aggregation with no index/ANN pushdown; statistics count actual leaf work and root output. Source routing leaves nonaggregate ordinary planning unchanged.

The four original unsupported SQL strings were preserved in capability-failure evidence and moved to positive complete-label/value/storage-class pinned comparisons, not weakened. Existing unrelated unsupported checks remain. Public sqlite/driver struct layouts and production dependency constraints are unaffected in the six production deltas.

No full default suite PASS is claimed. Owner evidence has exactly three known baseline ANN/sparse Boolean failures. The independent scope is aggregate-focused. No physical tamper, wrong-key, missing-file, namespace mutations, crash/kill/process-exit, hot recovery, native fault, interruption or rekey testing was executed. All running suites were behaviorally reviewed and preceded by internal/testscope.
