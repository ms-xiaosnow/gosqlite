# Boolean ranking predicate repair

## Result

The three lead failures are repaired without changes to existing tests. New
negative-control tests fail against unchanged 1336a5d production and pass against
the frozen repair. Full ordinary sqlite/internal/sql, selected driver/vector tests, vet/build,
selected tagged oracle, full internal/sql race and selected sqlite/driver/vector
race verification all PASS. The lead reports independent selected checks PASS with no current blocker; the
reviewer owns its separate final report. The lead combined-tree verification is
still separate from this repair artifact.

## Root causes and changes

1. Constant WHERE terms are removed from the residual compiled predicate and
   stored for startup evaluation. ANN materialization bypasses `fetch`, which
   was the only startup gate, so those terms were never checked.
2. Descriptor-free or otherwise directly constructed Rows could reach source-row
   evaluation with no compiled predicate, silently skipping the WHERE clause.
3. The native vector/half/sparse/bit/similarity filter helper retained scalar
   evaluation rather than predicate evaluation, producing eager typed/arithmetic
   errors and different error suppression from the documented Boolean contract.
4. Re-entering the startup gate after a false/error result returned success;
   startup now remembers its terminal result while still checking cancellation.

The repair starts SQL ranking predicates before ANN/query decoding/scratch,
compiles unplanned Rows on first entry, and guards direct source-row evaluation.
The public filter helper compiles the same predicate and starts it lazily on the
first candidate. Public APIs retain their existing no-source/NULL/zero-limit
validation timing and candidate-budget accounting. This is not a change to ANN
recall, metrics, storage, planner admission, scalar projection, DML or grammar.

## Exact source

- Base: immutable combined tree 1336a5d7b5e1a336fb01e012334cec776a61c4a7.
- Freeze: ../boolean-rank-filter-freeze-v1.
- Six owned paths and exact base/after SHA256: owned-manifest.json.
- Full-file manifest: frozen-all-sha256.txt, SHA256
  c6a97ba00dbc2ed114ed5979aeb0317fabd203278c09cf6c33854e3fb2e993ff.
- Source patch: repair.patch, SHA256
  1121baa6b58ef3388e96c8c1b667c7fb9c0b4743766ae00b3b5c77d41638b13e.
- owned-manifest.json SHA256:
  7683d73e4ac5a4ee726c7c871f4f72613fc34f70b3f25e1e3a129ccd59f82790.

Existing tests, scripts and CI files are byte-identical to the base. The main
repository/Git/remote were not edited. Neither the immutable lead source nor its
original full-failure log/exit was edited. Their log hashes are retained in
original-lead-failure-sha256.txt.

## Test evidence and honest history

- initial-regressions.txt: unchanged three failing tests plus existing Boolean
  tests PASS after the production repair.
- new-regressions.txt: initial new-test run FAIL because its new positive ANN
  assertion incorrectly expected `bounded_heap` instead of `ann_exact_rescore`.
  The expectation was corrected before freezing; no production change or
  historical-test weakening was made to resolve that new test-authoring error.
- focused-final.txt: all then-current new regressions and historical failures
  PASS; new pinned oracle PASS, 21 scalar predicates through four entry paths
  (84 native comparisons).
- focused-complete.txt: final focused source PASS, including IVF Lists 1/2/4
  trained/untrained/all-lists fallback coverage added after focused-final.
- negative-baseline.txt / negative-baseline.exit: final new tests on untouched
  base production plus those tests FAIL as expected (exit 1), covering all six
  new regression groups. The original full failed run is preserved separately.
- verify-frozen.sh: exact full sqlite/internal/sql, selected driver/vector,
  vet/build and selected tagged oracle commands. See full-selected.txt/.exit (PASS, exit 0, completed 19:25:52 UTC).
- race-frozen.sh: exact selected sqlite/internal/sql/driver/vector race command.
  See race-selected.txt/.exit (PASS, exit 0, completed 19:27:24 UTC; sqlite
  race package 252.464s). Race instrumentation is not fault/race injection.
- full-internal-sql-race.txt/.exit: additional full internal/sql race PASS,
  exit 0, 1.931s. Exact command and audit are included in the log.
- Full-file manifest identity was verified after both frozen aggregate runs.

See scope-audit.md for the per-suite behavioral audit. Every suite command is
preceded by the source-reference guard. Go is pinned to 1.27.2 with module network
disabled; production verification uses CGO_ENABLED=0. The separately pinned
SQLite 3.54.0 CLI is only a read-only test oracle that exits normally at EOF.

## Integration

Apply only the six paths in owned-manifest.json over exact base 1336a5d, or apply
repair.patch after checking its base hashes. Other in-flight Boolean grammar,
aggregate and planner changes must be reconciled by their owners; no unrelated
source was imported. Independent reviewer must inspect startup/error ordering,
false-state caching, lazy typed decoding and public-filter timing/budgets, then
rerun the original three regressions, new tests and the pinned scalar oracle.
