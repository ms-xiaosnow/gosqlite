# Imported-dataset verification v2: signed-IP bound repair

## Result and preserved failure

v1 is held because independent review demonstrated a false rejection of a valid,
intact 16,000-dimensional imported dataset. Native signed-IP error was
7.4377592676582054e-25; the old reported bound was 5.2853384343223043e-26,
14.0724 times too small. The original v1 source, normal/race failed evidence and
review verdict remain intact. Nothing was relabeled as a pass.

v2 fixes only the verifier's mathematically justified IP rounding allowance and
related precision wording, retaining native metrics and reference mathematics.
The unchanged cancellation regression now passes with bound
6.3114788134040037e-24, error/bound 0.117844953. The intact two-row real import also
passes. Existing owner negative controls and all three independent regression
files are included byte-for-byte unchanged. A new pure-memory exact-rational test
certifies outward evaluation of the error-bound formula itself.

With u=2^-53, gamma_n=n*u/(1-n*u), and S=sum(abs(source products)):

- Exact local FastTwoSum residuals have absolute sum <= gamma_n*S.
- Ordinary accumulation of those residuals contributes <= gamma_n^2*S error.
- Final native and reference rounding contribute at most
  2*u/(1-u)*abs(reference), conservatively bounded by 4*u*abs(reference).
- The implementation returns an outward-rounded upper bound for
  4*u*abs(reference)+(1+u)*gamma_n^2*S.
- Each positive scale addition is rounded upward; all bound arithmetic rounds
  outward, including downward denominator subtraction. This accounts for
  downward scale rounding without assuming it away.

This dimension-squared term follows the actual separately accumulated correction;
it is not an arbitrary widening of the old constant. Existing per-hit bounds,
complete result and signed-PK checks, sampled-query/full-candidate disclosure,
strict ordering controls, Top-K ambiguity and ANN not_run remain unchanged.

The docs/comments now distinguish exact component products/sums at 768 bits from
cosine's potentially rounded product of squared norms, sqrt and division.

## Exact source and hashes

All paths below are beneath `/workspace/scratch/5e06a74e5f65/durable-restart`.

- Work: `import-distance-work-v2`
- Full freeze: `import-distance-freeze-v2`
- v2 tree: `666e201a98638e7c5759ab25ffe3aed7adbf8b4c`
- Baseline: `plain-final-verify`, tree `7d3184f87165e11e12560884ec6e4f92c9bfd039`
- Checked full baseline patch: `import-distance-v2-source.patch`
- Full patch SHA-256: `64697339b9b7a087a1c0ba813fad87effd9200489c3f5f4b227149933a07e37b`
- Checked incremental patch: `import-distance-v1-to-v2.patch`
- Incremental SHA-256: `406dfb51f549507fa5ef457f9b152d82a334f9853b71dcd20212037947e6f6b2`
- Full patch replay: `import-distance-patch-check-v2`
- Incremental replay: `import-distance-v1-to-v2-patch-check`
- Inventory: `import-distance-v2-owned.json`
- All 1,261 frozen file hashes: `import-distance-v2-all-sha256.txt`
- Full source proof: `import-distance-v2-source-checks.txt`
- Regression preservation: `import-distance-v2-regression-preservation.json`
- Incremental/failure preservation: `import-distance-v2-incremental-and-failure-proof.json`

Both checked-patch replays match the complete freeze. All unowned files match the
plain baseline. Relative to v1, only production `import_distances.go`, its
scope doc and four added test files differ. The production source's changes are
limited to its bound implementation/report description and precision comments.
No public struct layout/signature changes beyond the original separate v1 API.
No main.go, native vector kernel, goal/progress/compatibility, main-tree, commit,
upload or remote writes occurred.

## Verification and scope

Go 1.27.2 sibling toolchain, offline caches, CGO0 normally and CGO1 only for
selected race tests. Guard ran before selected tests. No broad baseline suites
or manual tags were executed.

- New owner API/CLI tests and retained negative controls: PASS
- Unchanged independent cancellation, rational/1024-bit oracle, 16,000-dimension
  normal import/committed resume, unchanged-byte read-only checks, precise budget
  and CLI failure-report tests: PASS
- New exact-rational outward-bound certification: PASS
- Selected prior distance/PK-bit/hash tests and pinned legacy/vector fixtures: PASS
- Selected race suites: PASS, including the previously failing cancellation case
- Focused vet, full CGO0 build and empty core/CLI CGO closure: PASS
- CSV and NDJSON native importer example plus reusable CLI: PASS; each checks
  64 norms, 1,024 complete distances and 160 Top-K hits; ANN still not_run

Evidence:

- `import_distance_checks_v2.sh`
- `import-distance-final-checks-v2.txt` and `.exit`
- `import-distance-evidence-v2/{example,csv-verified,ndjson-verified,options}.json`
- `import-distance-evidence-v2/cgo-closure.txt`
- Exact freeze replay: `import-distance-v2-frozen-checks.txt` and `.exit`
- `import-distance-v2-frozen-hash-verify.txt`
- `import-distance-v2-preserved-v1-hash-verify.txt`
- Preserved failure: `import-distance-independent-review-v1/VERDICT-V1.md`
- Preserved normal failure: `import-distance-independent-review-v1/evidence/cancellation-v1.txt`
- Preserved race failure: `import-distance-independent-review-v1/evidence/cancellation-v1-race.txt`

All native tests use intact owned files, ordinary import/read/verify and normal
committed resume. Error controls are options, source parsing, pure mathematics or
fabricated in-memory results. No physical tamper/corruption/wrong-key/missing-file,
namespace mutation, crash/kill/process-exit/hot recovery, failed native I/O/lock/
unmap/close/Sync/shortwrite/diskfull/permissions or interrupted publication/rekey
was exercised. The independent reviewer has the exact v2 target for re-review.
