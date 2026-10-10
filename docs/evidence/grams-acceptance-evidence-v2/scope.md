# Grams acceptance / performance slice: scope and provenance

## Source boundaries

Baseline: `plain-final-verify`, tree `7d3184f87165e11e12560884ec6e4f92c9bfd039`.
Final candidate: `grams-acceptance-freeze-v2`, tree
`762a9e9dd8105cf620c68a8398922ddf254a9d5f`.
The complete inventories contain 1,252 baseline files and 1,256 final files.
Only four new files are owned:

- `benchmarks/grams/main.go`
- `benchmarks/grams/main_test.go`
- `benchmarks/grams/README.md`
- `sqlite/plan_grams_normalized_test.go`

The original engine, APIs, existing tests, main CLI, central documentation and
original `goal.md` remain byte-identical. No new dependency, commit, upload,
remote write, CI mutation or main-worktree modification is included. Git trees
are inventory objects in a separate local bare directory, not commits.
`changes.patch` applied cleanly to an independent copy of the baseline, and the
complete path set, file modes and content hashes match the final inventory.

Audit read: `goal-acceptance-audit-20261010/report.md`, SHA256
`0f1d1a830f3d851096b3fe61bb0921819008c6812705c59b54a3a8025693fe58`.
The original goal's clauses 5.2 and 11.3 are controlling. The goal's measurement
word “combinations” is not converted to a requirement for 10,000 SQL executions.
There is no original numeric large-benchmark threshold.

## Audited executable scope

`go run ./internal/testscope` precedes selected suites. Behavioral review covers
new native persisted benchmark CRUD/build/read/ordinary close/reopen/consistency
and cleanup of its own normally retired temporary directory; pure-memory corpus,
statistics and bounds tests; and `sqlite -run '^TestGrams'`. The latter includes
unchanged raw/decomposed/Unicode/ESCAPE/NUL/NULL/Boolean/high-frequency/MATCH
controls and the added NFC-input corpus and ordinary lifecycle coverage.

The lifecycle test exercises rollback and shared-WAL modes, INSERT/UPDATE/DELETE,
rowid change, read-own-writes, transaction rollback, SQL savepoint rollback,
REINDEX, another normal shared connection, checkpoint and normal close/reopen.
Native `CheckExtensionIndex` reconciles current source/posting/frequency state
at each stage; exact full query results and actual 1/2/3-gram SEARCH are required.

No broad repository suite was run. No physical corruption/tamper/wrong-key or
missing-selected-file test, live namespace mutation/race injection, forced
process exit/kill/crash/hot recovery, failed native I/O/lock/unmap/close/Sync,
short write/disk-full/permission fault, or interrupted publication/rekey was
run. A CLI invalid-flag exit occurs before any database opens; it is not a
forced-exit durability experiment. Ordinary WAL checkpoint/reopen is never
reported as recovery time. Native failures are not deliberately induced.

## Exact test-source attribution

The owner first ran selected checks on freeze-v1, tree
`cbe1e73af1bb617a2368e34d54077a29fbfcbc7e`. V2 adds conservative failed-run
temporary-directory retention in the benchmark and explains it in its README.
The complete `v1-v2-delta.json` proves these are the only two differences.
The entire sqlite test package and its imported execution closure are exactly
byte-identical between those freezes, so its selected normal/race results are
reported with this attribution, not falsely described as a v2 whole-tree suite.
The changed v2 benchmark is recompiled and rechecked separately. Original v1
logs are preserved in `grams-acceptance-evidence/`.

Go 1.27.2 comes from the sibling toolchain; module and build caches are local,
`GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOFLAGS=-buildvcs=false`.
Native normal tests/builds and standalone runs use `CGO_ENABLED=0`; only the
separate detector runs use `CGO_ENABLED=1 -race`. No benchmark result comes from
a race-instrumented executable. GOMAXPROCS is 2. Exact commands/logs are retained.

## Measurement limits

Predeclared and permitted: smoke (128 rows, 6 queries per length, 2 builds) and
medium (512 rows, 12 queries per length, 3 builds), seed2026101052, page4096,
max32768 dirty pages. Proposed larger release (10,000 rows, 96 queries per
length, 5 builds) is not executed without lead approval. Native budgets may
legitimately reject larger profiles; that must remain failure evidence.

Each measurement uses actual persisted posting B-trees and full real source
scans, captures observed plans/work/timing, and keeps fallback controls separate.
Cold connection is explicitly not cold OS cache. Repeats, nearest-rank P50/P95,
means and sample standard deviations are descriptive; CI bounds are null.
This shared cloud environment has uncontrolled co-tenant load and cache state.
`lscpu` could not read `/sys/devices/system/cpu/possible`; visible model and CPU
allow-list were instead read from `/proc/cpuinfo` and `/proc/self/status`.
The physical device underneath the overlay filesystem is unknown.

These observations cannot establish stable-hardware release performance,
statistically defensible speedup, general natural-language selectivity, broad
platform runtime, long soak, physical durability/recovery, or full M4/M10/goal
acceptance. CI regression integration, transaction/ANN/soak measurements and
all restricted/external qualification remain separate.
