# Offline ordinary-performance comparator

This is the offline regression-tool slice of goal §11.3. It reads already
collected, operator-supplied evidence for the fixed grams and rollback transaction
profiles. It does not collect measurements, build or launch native programs,
or contact measurement services. There is no `run.py` in this slice. A manual-only
CI workflow can compare existing committed evidence offline. Native collection
requires separate authorization. No real baseline or measured performance
acceptance is supplied by this integration.

The comparator and its tests use only Python's standard library. Python 3.11+
is required; the integration environment has Python 3.12.14. Go is not required
to run the offline CLI or its fixture suite. Admitted reports must record pinned
Go 1.27.2, the fixed flags below and matching reviewed harness implementations.
ANN reports and external SQLite/pg_bigm/pgvector results are unsupported here.

## Offline use

From the repository root, point the CLI at an existing evidence bundle:

```sh
PYTHONDONTWRITEBYTECODE=1 python3 benchmarks/performance/compare.py \
  /absolute/path/to/evidence/bundle.json \
  --output /absolute/path/to/comparison.json
```

`--output` is optional. When output I/O succeeds, the CLI prints the decision JSON
and, if requested, writes that same JSON to the output path first. An output-write
failure exits with an exception before JSON is printed; it is not a PASS. Use a separate new output file: it must
not overwrite the bundle or any evidence artifact. Relative artifact paths are
resolved under the directory containing `bundle.json`. Keep the input directory
stable during comparison. Inputs are regular files, not pipes or devices. JSON
must be UTF-8 and at most 32 MiB per document. Each JSON document is read once,
bounded to limit + 1 bytes; a referenced JSON document is hashed and parsed from
that same snapshot. Other artifact hashes use 64 KiB chunks. This bounds the
hashing allocation, not total artifact bytes, read time or total evidence size.
This is not a hostile-filesystem or concurrent-mutation security protocol.

Every status matters; only `PASS` exits zero:

- `PASS` (0): all three admitted, controlled, whole-invocation median cost-ratio
  confidence intervals have upper bounds at or below their engineering margins.
- `REGRESSION` (2): at least one qualified interval has a lower bound strictly
  above its margin. This is an engineering regression against that baseline.
- `FAILED` (3): an explicitly failed process/report, correctness/model failure,
  closure uncertainty or other mandatory successful-work predicate failed.
- `INADMISSIBLE` (3): malformed or contradictory evidence, a hash/identity/profile
  mismatch, invalid chronology/accounting, oversize/nonregular JSON or a read
  error other than a missing file. Inspect `reason`; do not reinterpret as a pass.
- `MISSING` (4): an expected file, pair or completed evidence set is missing.
- `UNSUPPORTED` (4): an unknown schema/revision, incompatible harness/dependency
  baseline, or a fully validated candidate-only bootstrap. A bootstrap has
  `bootstrap_validated=true` and zero comparison pairs; it never establishes a
  baseline comparison. Malformed bootstrap evidence does not get this exception.
- `INCONCLUSIVE` (4): fewer than eight pairs, an uncontrolled host, or intervals
  overlapping margins. Attractive median timings do not change this status.

The JSON `status` and `reason` are the result. A successful fixture suite or CI
badge is not a measured performance PASS. A caller must preserve nonzero exit
codes rather than silently converting missing/unsupported/inconclusive evidence
into acceptance.

## Manual offline CI input

[performance-evidence.yml](../../.github/workflows/performance-evidence.yml) has
only a `workflow_dispatch` trigger. Its required `evidence_directory` input is a
directory relative to the fixed `benchmarks/performance/evidence/` root in the
same checked-out repository, for example `reviewed-run-001`. Commit the complete
input set before dispatching the selected ref. There is no URL input, artifact
download, native collector, scheduled run or performance run on push/PR. The
job uses the existing ordinary `ubuntu-latest` target, its Python 3.11+ runtime,
read-only contents permission and checkout with credentials not persisted.
The existing benchmarks use synthetic datasets; no real business database or
secret is required. Review reports and environment metadata before committing
them to a shared or public repository. No real evidence set is supplied here,
so an actual measurement comparison remains NOT_RUN until approved input exists.

The selected directory must contain the original schema-2 `bundle.json`, all
its referenced files, and this separate `origin.json` declaration:

```json
{"schema_version":1,"origin":"operator_supplied_measurement"}
```

The only other recognized origin is `synthetic_fixture`, which dispatch rejects
as UNSUPPORTED (4). Unknown labels, extra fields and malformed declarations are
INADMISSIBLE (3). The declaration is an untrusted operator assertion, not proof
that measurements occurred: relabeling fixtures does not make them measurements.
Existing manifest, host and measurement truth requirements still apply. No input
records are rewritten, and the comparator schema/statistics are unchanged.

[ci_input.py](ci_input.py) checks the entire selected static tree first: canonical
relative ASCII directory components (letters, digits, dot, underscore, hyphen;
no empty, dot or parent components), at most 16 components and 1024 characters,
no symlinks in the fixed/input directory path or evidence tree, and only regular
files and directories. Limits are 1024 total tree entries, 64 MiB per file,
32 MiB per `.json` file, 256 MiB combined file bytes and 4096 bytes for origin.json.
The comparator independently bounds every JSON document to 32 MiB regardless
of filename. Missing inputs remain MISSING (4). Other admission errors are
INADMISSIBLE (3). These are limits for stable, trusted filesystem inputs, not a
hostile concurrent-mutation defense. Keep the checkout/evidence unchanged during
comparison. This wrapper never executes retained binaries or command records.

The comparator's complete decision JSON and all seven status/exit mappings above
are preserved. The job prints JSON to its retained step log and exclusively
creates `ordinary-performance-comparison.json` in trusted `RUNNER_TEMP`, outside
the checkout. That temporary file is not a published artifact; job logs plus the
committed source evidence are the retained record. An existing output file or an
output-write failure fails the job before printing a verdict. No evidence file
can be an output target. Inspect the JSON status/reason, not only the job badge.
A synthetic unit-test PASS never establishes measured performance acceptance.

## Statistical contract

The predeclared cost ratios are candidate / baseline:

- Mean grams index-build milliseconds within an invocation: margin 1.30.
- Mean grams incremental database bytes within an invocation: margin 1.10.
- Transaction active seconds per committed transaction: margin 1 / 0.75.

These are engineering alert margins, not original goal SLOs. The unit is one
paired whole invocation, never an individual query, window, transaction or one
of the two correlated grams builds. Predeclare 8–32 complete pairs. The schema
can represent 1–7 pairs, but they are always inconclusive. Odd pairs run baseline
then candidate; even pairs reverse that order. Each arm runs grams then
transactions. A source is built once, not independently recompiled per pair.
Do not discard warmups, filter outliers, retry failed invocations as replacements
or stop when a convenient interval first appears.

The statistical core is unchanged: exact two-sided binomial order-statistic
median intervals use the largest k satisfying
`1 - 2 * sum(comb(n,j), j=0..k-1) / 2**n >= 1 - (1 - .95) / 3`.
The interval is `[sorted_ratios[k-1], sorted_ratios[n-k]]`. Bonferroni targets at
least 95% simultaneous family coverage across three metrics, assuming independent
identically distributed whole-invocation pair ratios. At eight pairs each interval
is min/max with marginal coverage 0.9921875. Ties are conservative. Upper bound
at/below margin means metric PASS; lower bound strictly above means REGRESSION;
otherwise it is INCONCLUSIVE. All three must pass for overall PASS.

A controlled-host declaration needs a specific operator justification for stable
load, thermal, CPU/affinity and storage conditions and credible pair independence.
Shared-host measurements remain INCONCLUSIVE. Alternation alone does not establish
independence; drift and correlated scheduling can invalidate coverage. Grams
"cold_connection" means a new connection, not a cold OS cache. Neither profile
flushes caches. P50/P95 and transaction histograms are descriptive only, not
gated tail-latency metrics or independent confidence samples. PASS does not mean
equivalence, speedup, absolute SLO acceptance or complete release acceptance.

## Obtaining real inputs: separately authorized operator work

There is no installed evidence generator and no single collection command in
this integration. An operator must assemble schema-2 records and retain all raw
files. The following documents the existing native commands for a future,
independently authorized collection; the offline CLI and unit tests never run
them. They open real databases and must not be run under a source-only permit.

1. Select exact baseline and candidate source trees with the same reviewed grams
   and `ordinary-transactions-v4` harness Go files, the source guard, Go 1.27.2,
   and identical `go.mod`/`go.sum`. If no compatible historical baseline exists,
   establish a real baseline in a separately permitted run; do not substitute
   fixture values or retrofit an unknown historical harness. Keep the evidence
   directory outside both source roots. Each invocation must create fresh data.
2. Before collection, compute each source's complete regular-file manifest:
   relative POSIX path to `{ "sha256": <file bytes SHA256>, "mode": "100644" or
   "100755" }`. Include all source/build inputs; only top-level Git metadata is
   excluded. Avoid source symlinks, external module replacements/vendor inputs
   or an unrecorded source dependency closure. Set `source.id` to `"sha256:" +
   SHA256(canonical(manifest))`. Canonical means the exact `validation.canonical`
   serialization: sorted keys, compact comma/colon separators, default JSON
   ASCII escaping, no NaN, UTF-8 bytes. Recompute after collection and record
   the unchanged identity as `verified_after`. The comparator verifies the
   submitted manifest's consistency; it cannot inspect or authenticate the
   recorded source tree on another machine.
3. Observe the real host: `system`, `release`, `machine`, `cpu_model`,
   `logical_cpus`, `affinity` (unique allowed CPU IDs, or null), and `storage`.
   Set `hardware_id` to `"sha256:" + SHA256(canonical(observed))`, choose a real
   `runner_id`, a UUID `session_id`, and an honest controlled-host attestation.
   Record real UTC RFC3339 timestamps with up to nine fractional digits.
4. In each source root, use this exact environment and build-command argument
   order, separately recording each process's start, finish, exit code, stdout
   and stderr. `$EVIDENCE` must be an absolute path; `$ARM` is baseline/candidate.
   Create the evidence/bin directory beforehand. Dependencies must already be
   available through the operator's approved module/toolchain setup.

```sh
export CGO_ENABLED=0 GOFLAGS=-buildvcs=false GOTOOLCHAIN=local
export GOMAXPROCS=2 GOENV=off GOWORK=off
go run ./internal/testscope
go build -trimpath -o "$EVIDENCE/bin/$ARM-grams" ./benchmarks/grams
go build -trimpath -o "$EVIDENCE/bin/$ARM-transactions" ./benchmarks/transactions
```

5. For each of the predeclared pairs, run arms in alternating order, grams then
   transactions within each arm. Set `$SOURCE_ID` from that arm's manifest and
   `$HARDWARE_ID` from the observed host. The exact fixed arguments are:

```sh
"$EVIDENCE/bin/$ARM-grams" \
  -rows 64 -queries 1 -repeats 2 -seed 2026101052 -page-size 4096 -max-pages 4096 \
  -source-id "$SOURCE_ID" -environment "$HARDWARE_ID"
"$EVIDENCE/bin/$ARM-transactions" \
  -mode throughput -profile rollback -transactions 64 -rows 16 -readers 1 \
  -batch 1 -seed 20261010 -duration 1s -writer-pause 0 -reader-pause 1ms \
  -snapshot-hold 1ms -max-data-bytes 16777216 \
  -source-id "$SOURCE_ID" -hardware-id "$HARDWARE_ID"
```

`-duration 1s` is required recorded configuration; throughput mode is bounded by
64 attempts, not a promised one-second measurement. Preserve each invocation's
JSON stdout bytes, stderr bytes, exit status and actual timestamps under unique
paths. Use the real executable bytes/hash. Retain failed evidence instead of
relabeling it successful. The command examples do not generate command records,
timestamps or a bundle automatically. Do not infer process timestamps from report
timestamps; capture both. A command record must encompass its report interval.

6. Assemble the references/records below, finish source re-verification, close
   the evidence set, then run the offline CLI. A real accepted bundle requires
   both arms' real measurements and all matching provenance. Unsigned records,
   manifests, timestamps and host declarations remain subject to input truth:
   hashes detect changed bytes but cannot prove an operator executed a command.

## Required record schema and admission

[compare.py](compare.py) and [validation.py](validation.py) are the executable
schema contracts. [fixtures.json](fixtures.json) contains `grams` and
`transactions` report-shape examples; every value is test data, never a baseline.
The `ComparisonTests.bundle`, `command_fixture` and `fixture` helpers in
[test_compare_offline.py](test_compare_offline.py) show complete synthetic bundle
and record structure. They are tests, not a real-input generator. Do not use
their timestamps, reports, binaries, identities or controlled label as evidence.

Required bundle fields:

- `schema_version`: integer 2; `policy`: `ordinary-performance-v1`; `status`:
  `complete` (or candidate-only `bootstrap`); `session_id`: canonical UUID;
  `started_utc`, `finished_utc`; `recorded_evidence_root`: the absolute collection
  directory, even if the artifact set has since been copied elsewhere.
- `requested_pairs`: integer 1–32; `profile`: exactly `grams: GRAMS_ARGS` and
  `transactions: TRANSACTIONS_ARGS` from validation.py, in the recorded order.
- `environment`: `controlled` boolean, `control_reason` (nonblank if controlled),
  `runner_id`, `observed` as described above, and computed `hardware_id`.
- `sources.baseline` and `.candidate`: `id`, `manifest`, absolute source `root`,
  unchanged `verified_after`. `go.mod` and `go.sum` must be present and identical
  between arms. `harnesses.<arm>` is SHA256 of the canonical submanifest of all
  `.go` files under `benchmarks/grams/` or `benchmarks/transactions/`; both main.go
  files are required and these harness digests must match between arms.
- `binaries.<arm>.grams` and `.transactions`: artifact references. Each reference
  is exactly `{ "path": <relative path>, "sha256": <64 lowercase hex digits> }`.
  Binary files are hashed as data; the comparator never executes them.
- `build_commands.<arm>.guard`, `.grams`, `.transactions`: references to JSON
  command records. Guard precedes both builds, and both arms finish building
  before measurement begins. The comparator requires the precise command arrays
  and fixed build environment above.
- `pairs`: ordered records with `pair` integers starting at 1; `order` alternates
  `["baseline","candidate"]` / `["candidate","baseline"]`; each named arm has
  `grams` and `transactions` invocation references. Each invocation reference
  has exactly `path`, `sha256`, `exit_code` (integer zero), and `command` (a command
  record artifact reference). No missing pairs, reused report paths or duplicate
  report bytes are accepted as independent invocations. Bootstrap has only one
  candidate pair and cannot produce comparison PASS.

Each JSON command record contains `session_id`, `source_id`, absolute source
`cwd`, exact argv list `command`, integer `exit_code`, `started_utc`,
`finished_utc`, exact `build_environment`, and artifact references `stdout` /
`stderr`. Native invocation records also require `executed_binary_sha256` matching
the retained binary. Its stdout reference must equal the invocation report
path/hash. Even empty stdout/stderr files must be retained and hashed. All command
intervals must fit the session; each next invocation starts after the previous
one finishes. Preserve unknown informational fields if supplied, but do not
assume every field is validated or authenticated.

Reports require schema 1 and exact fixed typed configs, Go/runtime/build settings,
page size, cache descriptions, source/hardware identities, workload/data digests,
successful status, no errors and real elapsed/UTC chronology. Grams reports need
the pinned corpus, two builds, eight cache/length buckets per build, actual
selective SEARCH paths and explained fallback controls, full ordered result
equality, work accounting, page-aligned incremental space, and recomputed timing
summaries. Transactions require exactly `ordinary-transactions-v4`, complete
writer/read counts and traces, positive required outcome coverage, zero closure
uncertainty, verified ordinary reopens and the reconstructed final model.

Transaction window UTC durations must match active time within 2 ms. Sequential
writer and reader latency sums are bounded separately by active time because
the two workers overlap; maintenance is bounded by non-active elapsed time.
The existing sampled 256 MiB heap guard remains a harness admission limit, not
a continuous-memory/RSS SLO. Histograms check counts, extrema, descriptive
quantiles, finite-sample moments, and necessary bucket-weighted mean/deviation
bounds with existing floating-point/Welford tolerances. The last histogram
bucket is overflow and has no artificial finite cap. These checks do not
reconstruct unretained per-window samples or prove full distribution feasibility.
All P50/P95 results remain descriptive.

Duplicate JSON keys, nonfinite values including exponent overflow, malformed
dates/digests/paths, and booleans standing in for required integers are rejected.
Missing files are MISSING; other read failures and nonregular inputs are
INADMISSIBLE. Existing relative-path checks are retained. The caller remains
responsible for trustworthy stable files and the truth of operator records.

## Offline fixture checks and source provenance

```sh
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover \
  -s benchmarks/performance -p test_compare_offline.py -v
PYTHONDONTWRITEBYTECODE=1 python3 -m unittest discover \
  -s benchmarks/performance -p test_ci_input.py -v
```

`scripts/check.sh` includes both pure test commands after the existing source
guard. Its other ordinary Go checks are unchanged. The 17 new input tests use
ci_input.py, compare.py, validation.py, the existing pure test_compare_offline.py
fixture helper, fixtures.json and standard-library Python only. They check
path/origin/size admission, all seven verdicts, exact exits and output isolation
using temporary files, links and injected comparator results. One test also
checks real-comparator PASS/REGRESSION wiring with complete synthetic bundles;
its deliberately false origin declaration proves no measurement authenticity.
Command arrays and synthetic "bin" bytes are never launched. No project native
runner is imported and no subprocess, network, database or ANN operation is
invoked. Standard-library test helpers may import system-operation wrappers;
their presence is not a claim that the test closure invokes those operations.
The dispatch job compares supplied evidence; ordinary CI tests only synthetic
fixture logic.

This imports 45 of the recovered 54 methods: MathTests (3),
HistogramMomentTests (3), ComparisonTests (31), AdmissionRepairs (8). Their
assertions are preserved. The nine RunnerTests/RunnerModelTests/
RunnerAdmissionRepairs methods are excluded because their import closure includes
the native runner. This is selection for a new offline suite; no existing main
checkout coverage is deleted. Twelve focused regressions are added: five
histogram bucket/moment cases and seven bounded-file/snapshot/hash cases, for
57 methods total. Synthetic PASS validates parser/statistical behavior only.

The recovered source hashes, retained for audit (SHA256):

- compare.py: `fe0080cb89d877c40d47ae3f399662646a81d61c4f4db664a63dd72c378369cf`
- validation.py: `8031d0e8d4e4217f5f928c785a1dde2b80b9162c212c1f4bfdd78f4c3bcdd78e`
- fixtures.json, unchanged: `551f522de6528e4b516a5d7bb4a1af943cc27c6256de4e57a81c8ccf3d82d8f4`
- test_compare.py: `3d92296e89006d1ab2879509e78fecb19da5dcc9a20e2957208ccf3a98b507b4`
- prior README.md: `6f4ba946176c030dcf16072c313ee3d0cbdf634a424291763e5b37df858a1213`

Two narrow defects are repaired rather than inheriting a historical PASS:

1. The old `reference()` read the entire report before the later 32 MiB check and
   parsed a second read; command artifacts also hashed and parsed separate reads.
   Snapshot reads now enforce the limit first and hash/parse identical bytes.
   Non-JSON artifacts are streamed with explicit missing/invalid read status.
2. The old histogram checks independently accepted 64 samples whose moments
   claimed min=max=mean=1 ms, deviation zero, but whose buckets and percentiles
   claimed all samples were at most 0.001 ms. Occupied bucket/extrema and
   weighted moment bounds now reject that contradiction, while valid boundary,
   empty, near-constant and overflow examples remain covered by pure tests.

The median interval algorithm, three margins, confidence allocation and decision
logic are unchanged. These repairs and fixture checks do not establish native
performance, authenticate a real baseline or complete §11.3 release acceptance.
