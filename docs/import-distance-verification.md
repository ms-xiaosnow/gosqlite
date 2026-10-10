# Bounded imported-dataset distance verification

This implements the norm/distance-result portion of goal §9.2(2) for arbitrary
caller-owned offline exports accepted by the importer, within explicit limits.
It uses the real importer, native typed VECTOR columns, native SQL functions and
native exact search. No PostgreSQL connection, credentials, external server or
ANN recall claim is involved. CSV and NDJSON use the same mapping contract.

## Opt-in API and CLI

Existing `ImportOptions`, `ImportResult`, `ImportManifest` and `VerifyImport`
layouts/signatures are unchanged. Ordinary import/verify reports still say
`exact_distances: not_run`. The intentional API extension is a separate
`VerifyImportDistances(ctx, input, output, manifest, ImportDistanceOptions)`
returning `ImportDistanceVerification { Import, Distances }`, and the strict
`DecodeImportDistanceOptions` reader. The new wrapper's `import.exact_distances`
summarizes its additional report; it does not change the database's job status.

Create a local options file, for example `distances.json`:

```json
{"column":"embedding","query_count":4,"top_k":10,
 "metrics":["l2","l1","negative_inner_product","cosine"],
 "max_rows":10000,"max_pairs":100000,"max_component_work":8000000}
```

Then use the normal read-only verification command:

```sh
gosqlite migrate pgembedding verify --input source.ndjson \
  --manifest export.json --output imported.db --distances distances.json
```

`--distances` is accepted only for `verify`, never silently for apply/dry-run.
The CLI emits the full wrapper JSON and returns an error for failed verification.
The options JSON is at most 64 KiB; duplicate/unknown fields, malformed/trailing
JSON, unsupported metrics, invalid limits and duplicate query PKs are errors.
The required `column` names an explicitly mapped target VECTOR field exactly.
Only that column receives mathematical verification. Other vector columns are
listed as `unselected_vector_columns`; exhaustive typed fidelity still checks
all fields. Repeat explicitly for other columns. No inference from an ANN index
name, SQL expression or source column name is made.

Instead of `query_count`, supply e.g. `"query_pks":[-42,9223372036854775807]`.
These identify non-NULL accepted vectors in the immutable source export; decimal
int64 keys never pass through float64. Missing/rejected/NULL query keys error.
The two selection mechanisms are mutually exclusive. Queries are not arbitrary
new vectors in this version. Unsupported metrics error; they never pass or
silently disappear.

## What is checked

All ordinary verification remains intact: source SHA-256, every accepted value
and component bit, PK/source-record provenance, source/destination/metadata
counts, rejection identity, batches/cursors, schema fingerprint, requested index
identity/consistency and expected source count. Additional verification runs
inside the **same native read snapshot**, and the source is rehashed afterward.
No verification path writes or publishes completion state.

A second bounded source replay supplies reference vectors and PKs, excluding
only already-reconciled rejected source records. It never obtains reference
components from database rows or native search results. Source framing/float4
parsing is shared with the importer and documented as such; independent 768-bit
`math/big` mathematics supplies the norm/distance oracle. It does not call native
vector metrics, native `Norm`, `Scan` or SQL. This verifies the supplied binary32
export, not bits lost before export or a PostgreSQL server's arithmetic.

A complete native SQL scan checks mapped signed PKs, NULLs, VECTOR dimensions and
norms against the source references. For every selected query and supported,
defined metric, one native exact search returns **all** accepted non-NULL rows.
Every PK, duplicate/omitted hit, distance, native exact plan and source/NULL/scored
count is checked. A second native Top-K execution must exactly match the prefix
of the complete native distance-ascending, signed-PK-ascending result. Every
complete hit and Top-K hit is returned with native/reference values and its
absolute error bound. The result is bounded, not a digest-only spot check.

Query defaults are four evenly spaced ranks in the sorted signed-PK non-NULL
population, including both endpoints when two or more queries are selected;
integer division fixes intermediate ranks. At most the population size is used.
One query selects the smallest PK. Explicit PKs retain caller order. Reports
show selected PKs, selection method, candidate/NULL/zero counts and whether all
source vectors or only sampled queries were used. **Candidates are never
sampled**. A query sample is deterministic but not a statistical confidence or
application-workload coverage guarantee.

## Numeric and ranking contract

The reference has 768-bit precision, enough to accumulate the entire finite
binary32 product/difference exponent range and 16,000 terms exactly before
high-precision square root/division and final binary64 rounding. Cosine's product
of the accumulated squared norms may also round at 768 bits; component products
and sums remain exact. Native metrics widen binary32 inputs and use Neumaier compensation. With epsilon = 2^-52:

- L1, L2 and norms: absolute bound `64 * epsilon * abs(reference)`
- Cosine: absolute bound `64 * epsilon`, with both results clamped to [0,2]
- Negative inner product: outward-rounded `4 * u * abs(reference) +
  (1 + u) * gamma_n^2 * S_upper`, where `u = 2^-53`,
  `gamma_n = n*u/(1-n*u)`, `n = dimensions`, and `S_upper` is an upward-rounded
  accumulation of `abs(source products)`

The nonnegative metrics avoid sum cancellation; the conservative relative
coefficient covers subtraction, accumulation and square root. There is no
absolute floor that could hide a zero replacing a small/subnormal norm or L2.
Widened binary32 products are exact in binary64. The signed-IP bound includes
second-order error in the separately accumulated Neumaier correction, which is
summed ordinarily rather than recursively compensated. The v1 linear-dimension
second-order bound was insufficient; v2 retains the dimension-squared term.
Neither native arithmetic nor source-reference arithmetic is loosened or changed.

For exact products `p_i`, let `S = sum(abs(p_i))`, `s_i` be the ordinary native
partial sum, and `e_i = (s_(i-1) + p_i) - s_i` the exact FastTwoSum residual.
Nonzero binary32 product magnitudes/residuals stay in binary64's normal range;
there is no overflow or underflow in this argument. For `n <= 16000`:

1. `abs(s_i) <= S_i/(1-n*u)`, hence
   `sum(abs(e_i)) <= n*u*S/(1-n*u) = gamma_n*S`.
2. Ordinary summation of those residuals contributes at most
   `gamma_n * sum(abs(e_i)) <= gamma_n^2*S` error.
3. Final native rounding adds `u*abs(exact_sum)` plus `u` times that correction
   error. Rounding the exact source reference to binary64 adds another
   `u*abs(exact_sum)`. Their sum is at most
   `2*u/(1-u)*abs(reference) + (1+u)*gamma_n^2*S`.
4. `4*u*abs(reference)` safely dominates the first term. Every nonzero positive
   scale addition and every bound multiplication/addition/division rounds
   outward with `math.Nextafter`; denominator subtractions round downward.
   Thus neither the positive scale sum nor the computed bound can silently
   round below the claimed upper bound. All-zero products keep an exact zero
   bound. These operations remain finite throughout the supported domain.

Each comparison uses its own reported bound, and NaN/Inf always fail. These
bounds apply to this native widened/compensated arithmetic contract, not upstream
float32/FMA bit-exact results or arbitrary third-party metric implementations.
The independently reported uncertainty intervals remain necessary near severe
cancellation; a nonunique mathematical Top-K boundary is never promoted to exact
mathematical ordering.

Reference ordering is required outside the source-derived uncertainty intervals.
`top_k_boundary` is `clear_under_reference_error_bounds` only when every selected
reference upper bound is below every excluded lower bound. Overlap, including a
mathematical tie or near tie, is explicitly
`ambiguous_within_error_bounds; native distance/signed-PK contract checked`.
Returning every candidate is `all_candidates`. An ambiguous boundary is not a
claim of uniquely ordered mathematical neighbors. Native deterministic ordering
and exact Top-K/prefix equality are still required in every successful search.

## Caps and skipped outcomes

Defaults equal hard maxima: 10,000 accepted rows (including NULLs), 16 queries,
100,000 reported full-plus-Top-K comparisons, 8,000,000 planned component work,
and 1,000,000 retained source components. Top-K defaults to 10 and is 1..10,000;
the effective count is min(K, non-NULL candidates). Callers may lower row/pair/
component-work limits. Preflight charges two norm passes and four per-query /
metric candidate passes (reference, error-bound scale and two native searches),
plus native query validation; skipped cosine metrics are charged conservatively.
Reference mathematics starts only after preflight. Budgets are logical work /
retention bounds, not total process RSS or wall time. Native engine quotas may
fail earlier. Parsing/replay and exhaustive fidelity retain their original
1 GiB source / 1,000,000 record / 1 MiB record limits and costs; these checks are
not made partial by smaller mathematical budgets. Exceeding any mathematical
cap errors rather than truncating candidates or fabricating a pass.

NULL vectors are counted and excluded from native search. Empty/all-NULL data
has `skipped_no_nonnull_vectors`, zero query comparisons and explicit coverage.
Cosine rejects a zero norm in any eligible candidate/query. This verifier reports
`skipped_zero_norm` for that query/metric instead of filtering rows, inventing
values or treating an expected error as a numerical pass. Other metrics can pass,
producing `partial_skipped_metrics`. If nothing is defined the status is
`skipped_no_defined_metrics`. `passed` means all requested nonempty query/metric
checks ran; sampled query coverage and boundary ambiguity remain separately
visible. Errors after verification starts yield a partial `failed` report and must be
inspected. Invalid arguments/options return an error with `not_run` before any
database verification. Unsupported metrics are named explicitly in that error.

`ann_recall` remains `not_run` in both report sections, even if imported native
ANN indexes exist. Existing fixed ANN examples are independent evidence only.

## Verification scope

New tests exercise CSV/NDJSON mapped PKs/columns, ordinary committed pause/resume,
quarantine, complete candidate and Top-K results, deterministic/explicit queries,
MinInt64/MaxInt64, signed zero/subnormals/MaxFloat32, cancellation, near ties,
empty/NULL/zero datasets, budgets and strict option decoding. Negative comparison
controls fabricate only in-memory result slices. Native files stay intact.
No physical tamper/corruption/wrong-key/missing-selected-file, namespace mutation,
crash/kill/process-exit/hot recovery, failed native I/O/lock/unmap/close/Sync,
shortwrite/diskfull/permissions or interrupted publication/rekey is exercised.
Central goal/progress/compatibility documents are deliberately untouched.

## Retained v1 regression and v2 repair

Independent review found a valid 16,000-dimensional signed-IP dataset falsely
rejected by the v1 verifier. Native absolute error was
`7.4377592676582054e-25`, exceeding the old `5.2853384343223043e-26` bound by
14.0724 times. The v1 frozen source and normal/race failure evidence are retained
separately. The deterministic pure-math and ordinary intact two-row import
regressions, independent rational/1024-bit oracle, max-dimension/resume/read-only
checks, and all existing negative controls are preserved unchanged in v2.
The repair changes only the verifier's justified rounding allowance and precision
wording; it does not alter native metric values, source references, search,
ordering, candidate coverage, tolerance-independent rejection controls or ANN
status.
