# Independent imported-dataset distance review: v1 blocked

## Verdict

The reusable API/CLI substantially implements the requested bounded caller-dataset verification, but v1 has one blocking correctness defect: its signed-inner-product error bound is not conservative over the supported finite binary32/dimension domain. A valid intact dataset is falsely rejected. This is not a storage, concurrency, corruption, or recovery finding.

Review target: tree `81de59c66341039a34d0f16849c46f98f3c8a3bc`, baseline `7d3184f87165e11e12560884ec6e4f92c9bfd039`, patch SHA-256 `943701cc9473e742c311538f71b16ca5904a12bb8fce04ebb3fe6a077b0352bb`.

## Blocking finding

`migration/postgres/import_distances.go:499` uses `8*epsilon*abs(reference) + 32*n*epsilon^2*sum(abs(products))`. Native Neumaier summation separately accumulates its exact local rounding residuals in an ordinary floating sum. A conservative general bound must account for that residual sum's accumulated error; an `O(n*epsilon^2)` term is insufficient here.

The independent deterministic 16,000-dimensional construction uses only valid finite binary32 components. It starts with product +1, adds 15,998 small positive binary32 products chosen to expose correction rounding, and ends with product -1. An independent rational-sum/1024-bit oracle agrees with the production 768-bit reference.

- Native negative IP: `-1.3321008237728481e-12`
- Rational reference: `-1.3321008237721043e-12`
- Absolute error: `7.4377592676582054e-25`
- Claimed bound: `5.2853384343223043e-26`
- Error / bound: `14.0724371`

The pure-memory regression fails. The end-to-end regression imports these two rows normally and `VerifyImportDistances` rejects them with `import distances: distance mismatch at PK 2`. All source and native files remain intact.

Evidence: `evidence/cancellation-v1.txt`, `evidence/cancellation-v1.exit`; independent regressions in `regressions/migration/postgres/import_distance_cancellation_review_test.go` with oracle helper in `import_distance_independent_review_test.go`. Failures are preserved, not excluded or rewritten as passes.

A safe repair can retain the native implementation and use a justified dimension-squared second-order bound, preserving per-hit uncertainty and explicit ambiguous Top-K boundaries. This needs a new frozen target and regression rerun.

## Nonblocking precision wording

The comment at `import_distances.go:553–555` says products/sums are exact before sqrt/division. Component products, differences and accumulations are exact at 768 bits, but cosine's product of accumulated squared norms can require more than 768 significant bits and may round. For `[MaxFloat32, SmallestNonzeroFloat32]`, one squared norm has 554 significant bits and its self-product has 1,108. This is far below the final binary64 tolerance and is a wording issue, not an observed numerical defect. Exact witness: `evidence/cosine-reference-wording-witness.json`.

## Verified architecture and scope

- Separate `VerifyImportDistances`/options/report API and verify-only `--distances` route; existing public import structs unchanged.
- Existing exhaustive source/PK/provenance/count/schema/index/fidelity checks remain; additional checks use the same native read transaction and final source hash recheck.
- Reference components come from source replay, not native rows or native results. Native metric code is not used by the high-precision reference. Source parsing is shared and correctly disclosed.
- Mapped signed keys, deterministic/supplied source-PK queries, one explicit vector column per call, unselected-column reporting, NULL/empty/zero-norm outcomes, and sampled-query/full-candidate coverage are explicit.
- Hard/lowerable budgets cover accepted rows, query count, returned comparisons, planned logical component work and retained components. There is no candidate truncation presented as complete coverage.
- Every native full exact hit is checked; a separate native Top-K run must match its prefix. Native distance/signed-key ordering and interval ambiguity are checked and reported. ANN remains `not_run`.
- No product-wide, release, recovery, ANN-performance or PostgreSQL server arithmetic claim is supported by this scoped review.

## Other verification

Before adding the newly discovered cancellation regressions, selected normal and race suites passed, as did focused vet, full CGO0 build, strict CLI success/error reporting, pinned source/golden math fixtures, and both CSV/NDJSON importer examples followed by reusable CLI verification. Each example reported 64 norms, 1,024 complete distances and 160 independently executed Top-K hits with ANN `not_run`.

Independent added coverage passed for rational-oracle equivalence over full binary32 exponent ranges and widths 1/3/17/129/16,000, real maximum-dimension import/committed resume, all-source query coverage, native/source bytes unchanged after read-only verification, exact budget boundary, strict JSON and fabricated wrong-distance/order controls. Initial passing checks do not override the later reproduced blocker.

Commands and complete logs: `checks.sh`, `evidence/checks.txt`, `evidence/checks.exit`, `evidence/independent-first.txt`. All runs use sibling Go 1.27.2, offline local caches, CGO0 normally and CGO1 only for selected race tests. `go run ./internal/testscope` passed before selected runs. No broad historical suite or manual tags were run.

## Integrity

The baseline and frozen source trees were independently reconstructed in an isolated bare object store, all 1,257 files checked against the inventory, and the exact baseline-plus-patch replay matched every byte. Only three independent regression files were added in the review copy. Baseline, owner freeze and replayed production source remain unchanged; see `evidence/identity.json` and `evidence/source-after-v1.json`.

No commits, uploads, remote writes, owner/main edits, physical tamper/corruption/wrong-key/missing-native-file tests, namespace mutation/race injection, crash/kill/process exit/hot recovery, failed native I/O/lock/unmap/close/Sync/shortwrite/diskfull/permissions, or interrupted publication/rekey were performed.
