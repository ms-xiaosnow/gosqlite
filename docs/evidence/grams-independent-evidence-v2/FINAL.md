# Final independent grams review: scoped PASS

No must-fix source finding. The normalized-input acceptance gap is satisfied, and the native grams benchmark harness plus declared small/medium measurements are verified. This is not large-run, fixed-hardware release, crash/recovery, or universal-performance acceptance.

## Exact identities

- Baseline: tree `7d3184f87165e11e12560884ec6e4f92c9bfd039`, 1,252 files.
- Accepted final: tree `762a9e9dd8105cf620c68a8398922ddf254a9d5f`, 1,256 files.
- Full patch SHA256: `d9feb249e2678f76d58da6d0bd25a56d47c1dc8a9dfd51b8060210fc75d9cbfe`.
- Independent and owner CGO0 executable SHA256: `7c80d5a732adc95b534a8b04b128c51bdfc5f0ff6a64160b9e5f356f170ef40f`.
- Dated acceptance audit SHA256: `0f1d1a830f3d851096b3fe61bb0921819008c6812705c59b54a3a8025693fe58`.

Complete path/mode/size/SHA256 inventories match; independently applying the four-file patch reproduces the exact final tree. The post-test inventory is unchanged. Only `benchmarks/grams/{README.md,main.go,main_test.go}` and `sqlite/plan_grams_normalized_test.go` are added. Production code and raw/decomposed fixtures remain byte-identical.

## Corpus and gates

NFC corpus: 128 distinct randomized texts ×97 distinct patterns = **12,416 unique pairs**, with 12,800 executed memberships. 100 texts changed under NFC. Pinned digest: `ab4a4a4d2cbb9d85630b88d0d7b1bdeabf7d74820f69702ba4cf5431f3472944`. All 100 patterns used real SEARCH; 45 were nonempty. Exact pre-index and forced-scan equality, real 1/2/3 paths, ordinary DML/rowid/savepoint/rollback/REINDEX/shared-WAL/checkpoint/reopen posting consistency pass.

Independent pinned offline Go 1.27.2 gates passed after the source guard:
- Selected sqlite `^TestGrams`: normal 131.769s; race 419.481s.
- Benchmark harness: normal 49.085s; race 105.015s.
- Vet, CGO0 build, no-CGO dependency closure, deterministic corpus, and owner/independent executable byte equality pass.
- CGO1 was used only for race detection. No broad suite, manual tags, physical failure/corruption/recovery workflow or large release profile ran.

## Measurements, including unfavorable results

- Independent smoke (128 rows / 6 queries per length / 2 builds):80 complete observations, 72 genuine SEARCHs, 8 fallbacks, 10,240 memberships; 56.740s wall time. Builds 15.589/12.297s.
- Owner smoke (128 rows / 6 queries per length / 2 builds): independently audited 80/80 observations; 52.052s wall time. Its plans/counters/results exactly match the independent smoke. Builds 11.397/8.920s.
- Both smoke runs: 24,576→3,141,632 DB bytes, **127.83× total-size amplification**; 4 versus 128 source fetches, yet **all 12 selective bucket P50s were slower than scan** in each run.
- Owner medium (512 rows / 12 queries per length / 3 builds): independently audited 228/228 observations, 216 genuine SEARCHs, 12 fallbacks, 116,736 memberships; 290.019s wall time. Builds 65.292/37.267/41.224s. 77,824→11,440,128 bytes, **147× total-size amplification**; 16 versus 512 source fetches. Four of 18 selective bucket P50s were slower than scan.

The independent auditor regenerated expected IDs from corpus text and verified every result hash, exact logical counter, workload identity, P50/P95/stdev and space calculation. Build/space metrics cover the joint 1/2/3 index and both namespaces; they are not isolated payload or per-length metrics. Cold means fresh connection only. OS caches/load are uncontrolled; uncertainty is explicitly descriptive with no inferential CI. Failed benchmark directories are retained after operation/close errors; this was source-reviewed without injecting native failures.

## Retained evidence

`review.md` has the detailed source review. `inventory-result.json`, `independent-diff.json`, `accepted-changes.patch`, the full inventories and `post-tests-inventory-verification.txt` establish identity. `checks.sh/log/exit`, `standalone.sh/log/exit`, `build-info.txt`, `environment.txt`, `smoke.json`, `owner-smoke.json`, `owner-medium.json`, both corpus files, `audit_report.py` and all three audit JSONs preserve execution and independently reproducible checks. Artifact hashes are in `final-evidence-sha256.txt`.
