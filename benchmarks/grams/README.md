# Ordinary persisted grams benchmark

This CGO-free runnable harness measures the real native database and persisted
`grams13` posting B-trees. It never shells out to another SQL engine. It builds
one joint 1/2/3-gram index, normally closes the database, reopens it, and compares
complete ordered ID results to actual `NOT INDEXED` source scans. A required
selective query that does not execute real `LIKE_RAW_V1` gram SEARCH with the
specified length fails the run. Fallback controls must remain explained scans.

This is ordinary measurement code, not release/production acceptance or a
recovery harness. No fault injection, OS-cache flush, namespace interference,
process crash, damaged-file test, or recovery timing occurs. Temporary files
belong to the harness and are removed only after ordinary resource cleanup.

## Run

Use the project's pinned Go 1.27.2, with its existing offline module cache.
From the repository root, after reviewing the selected tests and running the
ordinary source guard:

```sh
export CGO_ENABLED=0 GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off
# Set SOURCE_ID to the exact reviewed tree/commit or complete source-manifest hash.
# Set HARDWARE_ID to an actual machine description and measurement conditions.
go run ./internal/testscope
go test ./benchmarks/grams -count=1
go test ./sqlite -run '^TestGrams' -count=1
go build -trimpath -o /tmp/gosqlite-grams ./benchmarks/grams
/tmp/gosqlite-grams -rows 128 -queries 6 -repeats 2 \
  -seed 2026101052 -page-size 4096 -max-pages 32768 \
  -source-id "$SOURCE_ID" -environment "$HARDWARE_ID" > grams-smoke.json
```

The default workload is 256 rows, 12 queries per length, 3 independent builds.
`source-id` and `environment` are required for timing runs; they are clearly
operator-supplied, not machine attestation. Capture CPU model, memory/storage
context, other load/isolation and executable/source hashes alongside the JSON.
The report also captures the actual Go build settings, dependencies, OS,
architecture, visible CPUs and GOMAXPROCS. Production/native execution uses
CGO0. A separate `CGO_ENABLED=1 go test -race` is a detector-only check; race
instrumented samples are not performance measurements.

Options are bounded: 64..20,000 source rows, 1..128 queries **per gram length**,
2..10 database builds, page size 512/4096/65536, and 4,096..65,536 dirty pages.
A larger allowed option value is not evidence that it was run or can complete
within native budgets. Quota failures are errors, not silent dataset reduction.
On a run failure, stdout retains partial JSON with `status: failed`, error and
completed/partial trial data; stderr and exit status also report failure. Failed
run directories are retained and reported, rather than removing a namespace
after a possibly incomplete native operation or close.
Invalid CLI options fail before opening a database.

## Predeclared profiles

- Smoke: 128 rows, 6 queries per length, 2 database builds, page 4096, max 32768 pages.
- Medium: 512 rows, 12 queries per length, 3 builds, page 4096, max 32768 pages.
- Proposed larger release profile: 10,000 rows, 96 queries per length, 5 builds,
  page 4096, max 65536 pages. This is not yet approved or executed by this work.
  It increases source cardinality about 20× from medium and retains repeated
  builds and hundreds of selective queries. The original goal defines no
  numeric “large” threshold; this proposal is a bounded engineering choice,
  not an original contractual number or proof of representative user traffic.

All use seed 2026101052 and both cache profiles. Each additionally runs two
fallback controls per cache profile per build. Do not run the larger release
profile until the lead approves the resource/workload commitment. Controlled
fixed-hardware release measurements, broader selectivities, document lengths,
page-size matrix and real workloads remain separate follow-up qualification.

## Corpus and exactness

`grams-nfc-selectivity-v1` generates deterministic mixed-Unicode random text,
normalizes each complete input to NFC, and adds controlled unique markers.
Markers have exactly 1/2/3 Unicode code points and approximately 1/32 source
selectivity. There are 32 marker groups; query draws may repeat, especially
above 32 queries/length. Timed queries are explicitly controlled synthetic
substring probes, not independent natural-language search samples. `%` and
`%common%` are literal-free/high-frequency SCAN controls. The joint index stores
both raw LIKE and configured MATCH namespaces.

Only the generated input is normalized. Production LIKE semantics remain
`LIKE_RAW_V1`, including raw decomposed substring behavior and ASCII folding.
The existing adversarial tests are unchanged. New
`sqlite/plan_grams_normalized_test.go` separately tests 100 randomized patterns
against 128 randomized NFC texts: 12,800 query/text memberships, **not12,800
queries**. All input strings and patterns are asserted NFC, a fixed
length-prefixed UTF-8 SHA256 pins their identity, and 100 pre-index scans plus
post-index SEARCH/forced-scan comparisons establish complete equality. It also
requires actual 1/2/3-gram plans. Ordinary rollback/shared-WAL lifecycle tests
reconcile posting/source state through DML, rowid changes, read-own-writes,
rollback/savepoint, REINDEX, cross-connection visibility, checkpoint and normal
reopen. No physical recovery claim follows.

The benchmark hashes the compact Go JSON corpus including version, seed, all
texts and query definitions, and records it as `workload_sha256`. To inspect or
compare deterministic artifacts without opening a database:

```sh
/tmp/gosqlite-grams -rows 128 -queries 6 -repeats 2 -corpus-only > corpus-a.json
/tmp/gosqlite-grams -rows 128 -queries 6 -repeats 2 -corpus-only > corpus-b.json
cmp corpus-a.json corpus-b.json
```

The SHA256 of these indented files differs from the compact corpus identity by
formatting. Timing reports deliberately include variable timestamps/timings;
they are not expected to be byte-identical. Raw per-query result hashes,
complete plans and logical work support checking their deterministic content.
Each normally closed build's database SHA256 is recorded without asserting
that physical page bytes are guaranteed stable across future engine versions.

## Timing, space and uncertainty

- Build time is the actual native `CreateGramsIndex` operation including its
  ordinary commit. Insert time is recorded separately. The build statistic is
  for the single joint 1/2/3 index, not three independently built indexes.
- Before/after database file bytes give incremental index allocation,
  with-index/before DB amplification, and incremental bytes/source UTF-8 text
  bytes. Increment includes catalog, both namespaces, frequencies, postings,
  B-tree structure and allocated slack; it is not isolated payload, peak
  temporary disk usage, resident memory or per-gram attributable space.
- `cold_connection`: every query gets a fresh read-only connection with no
  workload warmup. The process and OS caches can already be warm from building
  the DB and prior queries. There is no cold-OS-cache claim.
- `warm_process`: the reopened connection executes the whole automatic and
  scan workload unmeasured once, then collects samples on that connection.
  The OS cache remains uncontrolled. Cache-profile order alternates by build.
- Each query latency includes parsing, planning, native execution, consuming
  every result, and Rows.Close. Connection Open/Close, result comparison,
  warmup and consistency checks are excluded. Automatic/scan order alternates
  by query and build. No concurrent writer changes the persisted corpus.
- JSON retains every query timing, plan and source/index-work observation.
  P50/P95 are nearest-rank empirical quantiles. Mean, sample standard deviation,
  minimum, maximum and sample count are descriptive. Repeats build separate
  databases but share a process/host and corpus; these are correlated
  observations. `confidence_method` states this, and CI bounds are null.
  Tiny-sample P95 may be the maximum. No statistical or speedup claim is made.
- Work counters are native logical frequency lookups, actual posting records,
  candidates, source fetches and returned rows; they do not measure physical
  I/O. Forced scans must fetch every source row and read no postings. Full
  ordered result equality is required 100%; selective paths also require
  positive posting work and fewer source fetches than scan. Fallback latency
  is separately labeled and is never presented as indexed performance.

Source/API layouts outside these added tests/harness files are unchanged.
Transactions, ANN, soak, CI regression policy and restricted recovery timing
are separate deliverables. This harness supplies measured inputs for later
reviewed CI comparison; it does not invent performance thresholds or install
any CI workflow.
