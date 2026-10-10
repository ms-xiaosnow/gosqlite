# Independent imported-dataset distance review: v2 scoped PASS

## Verdict

PASS for the bounded caller-dataset norm/exact-distance verification API and verify-only CLI under the audited ordinary-operation scope. No remaining blocker found. The v1 signed-IP bound defect is closed by a justified, outward-evaluated dimension-squared bound; native metrics, source-reference mathematics, ranking and negative controls were not weakened or replaced.

This supports the reusable norm/distance-result portion of goal §9.2 for supported caller-owned CSV/NDJSON exports within the documented limits. It is not a release-wide, durability/recovery, security, PostgreSQL-server arithmetic or ANN acceptance claim.

## Exact target and replay

- Baseline tree: `7d3184f87165e11e12560884ec6e4f92c9bfd039`
- v2 frozen tree: `666e201a98638e7c5759ab25ffe3aed7adbf8b4c`
- Full patch SHA-256: `64697339b9b7a087a1c0ba813fad87effd9200489c3f5f4b227149933a07e37b`
- Independent baseline-plus-patch and v1-plus-incremental-patch replays match all 1,261 frozen files and their modes.
- Baseline, v2 freeze and all replayed production files remain unchanged after testing. Only one new independent test file was added in the isolated review copy.
- Original v1 source, held verdict and normal/race failure logs remain unchanged. All three original independent regression files and both original owner test files are byte-identical in v2.

Evidence: `evidence/identity.json`, `evidence/preservation-checked.json`, `evidence/source-before.json`, `evidence/source-after.json`.

## Defect closure and independent arithmetic challenge

The original, unchanged 16,000-dimensional cancellation fixture has:

- Native/reference absolute error: `7.4377592676582054e-25`
- Old v1 bound: `5.2853384343223043e-26`, insufficient by `14.0724371x`
- New v2 bound: `6.3114788134040037e-24`
- Error / new bound: `0.117844953`

Both the pure-memory witness and ordinary intact two-row import now pass. The historical v1 failures remain failures in their original logs.

The analytic repair uses exact widened binary32 products and exact FastTwoSum residuals, bounds their absolute sum by gamma_n*S, and bounds their ordinary correction accumulation by gamma_n²*S. Native plus reference final rounding is safely covered by 4*u*abs(reference). Each positive scale/bound operation is evaluated outward, including the denominator and the rounded 1+u coefficient. All intermediates stay finite in the supported domain.

The derivation and operation-by-operation audit are in `ANALYTIC-REVIEW.md`. A new independent pure-memory test challenges 13 cases covering sign flips, ±80 exponent scaling, reversed summation order, dimensions 1 through 16,000, and MaxFloat32/subnormal spans. It verifies exact residuals, both gamma inequalities, the formula against rational arithmetic, actual native/report error, and rejection of fabricated beyond-bound values. The owner's exact-rational outward-bound test also passes. Cosine reference precision wording now correctly permits rounding in the product of accumulated squared norms.

New independent regression: `regressions/migration/postgres/import_distance_bound_independent_v2_test.go`.

## API, data and ranking audit

- The separate `VerifyImportDistances` API and verify-only `--distances` JSON route preserve existing public import layouts and ordinary import/verify behavior.
- Existing exhaustive fidelity/count/PK/provenance/schema/index checks remain active. Additional norm/search checks use the same native read snapshot, with source rehash afterward.
- Reference vectors and mapped keys come from source replay. The high-precision mathematical reference does not call native metrics, native norms, native search or SQL. Shared source parsing remains explicitly disclosed.
- Deterministic evenly spaced signed-key queries and explicit source-PK queries retain their documented ordering. Reports distinguish sampled-query coverage from all-source-query coverage and always use all accepted non-NULL candidates.
- One explicit mapped vector column is mathematically checked per invocation; other columns are disclosed, and ordinary fidelity still checks them. Empty/all-NULL and zero-norm cosine outcomes are explicit skipped/partial statuses.
- Row/query/result-pair/component-work/retained-component budgets are bounded and fail without silently sampling candidates. Their logical-work nature and preexisting full-fidelity costs are disclosed.
- Every full native exact result has its PK, uniqueness, distance, native plan/count coverage and signed-key tie order checked. A separately executed Top-K must match the complete-result prefix.
- Reference uncertainty and near-tie ambiguity remain explicit. Severe cancellation is not presented as unique mathematical ranking.
- ANN stays `not_run` in both report sections, including when imported ANN indexes exist.

## Independent reruns

All passed against the isolated exact v2 replay plus the one independent test:

- `go run ./internal/testscope` before normal/race checks
- Selected import/API tests, original cancellation regressions, rational oracle and new gamma proof challenges
- All retained strict JSON, budget, fabricated result, signed tie, zero/subnormal and near-tie negative controls
- CSV/NDJSON mappings, multiple vector columns, empty/NULL/zero outcomes, ordinary committed pause/resume
- Maximum-dimension native import/verify and byte-unchanged source/database read-only checks
- CLI success, structured failed verification, legacy report behavior, verify-only and duplicate-flag rejection
- Selected normal and CGO1 race suites; no race reported
- Published vector golden math and pinned source/hash fixtures
- Focused vet and full CGO0 build
- Empty core/CLI CGO dependency closure
- CSV and NDJSON examples followed by reusable CLI verification: each has 64 norms, 1,024 complete-distance comparisons and 160 separate Top-K comparisons; verifier ANN remains `not_run`

Commands/logs: `checks.sh`, `evidence/checks.txt`, `evidence/checks.exit` (0). Example/report JSONs are in `evidence/`. Toolchain: sibling Go 1.27.2, offline caches, CGO0 normally and CGO1 only for selected race runs.

## Scope exclusions

No broad historical suite or manual tags were run. No owner/main edits, commits, uploads or remote writes occurred. Tests used ordinary intact import/read/verify/committed resume and pure-memory/format/options negatives. No physical tamper/corruption/wrong-key/missing-selected-native-file, namespace mutation/race injection, crash/kill/process exit/hot recovery, failed native I/O/lock/unmap/close/Sync/shortwrite/diskfull/permissions, or interrupted publication/rekey was performed.
