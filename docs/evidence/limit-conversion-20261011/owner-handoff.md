# Independent LIMIT/OFFSET scalar conversion slice

## Result

Ready for independent review and lead integration. Production changes only
`sqlite/query.go`: `limitValue` exactly matches the previously reviewed V4c
function, and its unused `strconv` import is removed. The other 2,961 original
tracked files and all 360 original test files are unchanged. This is NOT broad
planner integration. Main, its Git metadata, remotes, Library and CI were not
modified by this worker.

Candidate: `../limit-conversion-work`, tree
`3fc9087ad121d423174a658e118ac455d1ace8c2`, 2,964 files.
Original main: HEAD `63c2ebb10e3a2c4ff2378ee9f33c46133134c70f`, tree
`d735409ebd1c43d90ee1650c8a7c2a084e270674`, 2,962 tracked files.
Original production plus final byte-identical regression tests:
`../limit-conversion-baseline-v2`, tree
`80d2ac039453de28581ea6f499d3914f65fee965`.

Exact final patch: `source.patch`, SHA256
`b712838805fadd0215464c828169226425170df00377a3c13df424cb0a4bc4bc`.
Fresh tracked-main extraction and `git apply --check`/application in
`../limit-conversion-replay` exactly reproduced the full candidate inventory.
No Git index/object/ref writes were used for source-tree calculation or replay.

Changed paths (only):
- sqlite/query.go
- sqlite/limit_conversion_test.go (new)
- sqlite/limit_conversion_oracle_test.go (new; existing oracle tag/SQLITE_ORACLE)

## Genuine original → corrected contrast

Identical final test bytes ran on original and fixed production:

- Original normal: exit 1; 630 failing and 386 passing named leaf cases.
- Corrected normal and race: exit 0; all 1,016 named leaf cases PASS each.
  Three top-level tests, plus two DB/Tx parent subtests, give 1,021 terminal
  PASS records. No skips. There are also two final positive owner-usability
  controls, one each for DB and Tx.
- Corrected oracle baseline: exit 1; 208 failing and 136 passing comparisons.
- Corrected production oracle normal and race: exit 0; all 344 comparisons
  PASS each (43 fixed expressions × 4 query shapes × LIMIT/OFFSET).

The 1,016 named cases comprise 63 direct scalar types/endpoints/text grammar/NUL,
9 direct error/preflight ownership cases, and 944 public DB/Tx query controls.
Public controls cover a three-row single table, LEFT JOIN, global aggregate,
and grouped aggregate, with nonzero valid bounds. Conversion rejection requires
nil Rows, ErrMismatch, CodeMismatch(20), and no DB/Tx Rows lease. Mathematical
integer overflow remains CodeError, not mismatch. Direct controls preserve the
exact work/memory/context errors, including full embedded-NUL value preflight.

Original defects include Go-only hexadecimal/underscore text acceptance,
Unicode whitespace trimming, REAL MinInt64 acceptance, embedded-NUL rejection,
and unwrapped generic conversion errors (public code 1 instead of 20).
Exact INTEGER and integer-text endpoints, precision, signed zero, underflow,
nonfinite values, BLOB/NULL, ordinary decimal grammar and positive row results
are retained controls. No grammar is added.

## Oracle harness correction, preserved honestly

The first oracle wrapper incorrectly assumed pinned CLI stdin diagnostics ended
in `datatype mismatch (20)`. Actual output omitted `(20)`, so its invalid-case
assertions failed before comparison to native SQL. Initial oracle source and
all logs/inventories are preserved in `initial-harness/` and the untouched
`../limit-conversion-baseline` tree (`6f1862e45686c65bc3714639f56c1275786497ec`).
The first fixed source tree was `e4d9e03319fbf01a70431108dde11565a1760eb0`.
Both initial oracle runs are HARNESS_FAIL, not accepted differential evidence.
Their full error counts cannot be credited to production.

After reading official shell.c.in, v2 enables the built-in `.log stderr` option
and requires a leading actual SQLite `(20)` log code plus datatype-mismatch
text. Natural exit 1, zero stdout, all native code/nil-Rows/lease assertions and
successful exact rows remain mandatory. The exact TWO changed lines are in
`oracle-harness-v1-v2.patch`. SQL argument mode was probed and also lacked the
code; it is not adopted. No custom C observer, extra environment variable,
extension, build dependency or workflow/script change was introduced. The
whole v2 helper chain was inspected and identical v2 bytes were rerun on both
original and fixed production, producing the valid contrast above.

Direct/public regression bytes NEVER changed after their first baseline run.
Both corrected helper source files also remained unchanged during v2 runs.

## Completed bounded gates

- Source-reference guard before/after: PASS, normal completion.
- Final normal/race direct/public selectors: PASS.
- Final pinned SQLite 3.54.0 oracle normal/race selector: PASS.
- Scoped `go vet ./sqlite ./internal/sql`: PASS.
- CGO0 `go build ./sqlite ./internal/sql`: PASS.
- Full patch replay, all before/after hashes/sizes/modes, original test inventory,
  and post-run candidate/baseline identity: PASS.

All commands use pinned Go 1.27.2, GOTOOLCHAIN=local, GOPROXY=off, GOSUMDB=off,
GOFLAGS=-buildvcs=false, fixed GOPATH/GOCACHE, GOMAXPROCS=2, CGO0 ordinary and
CGO1 race. Default TMPDIR remained `/tmp`; capacity was read before fixed runs.
Oracle normal completion only; no kill/cancellation/timeout or custom observer.
A shell evidence-runner initially ended after its successful first race command;
its recorded COMMAND_EXIT=0 and natural tool exit were preserved, then remaining
unrun checks were continued. No test was reclassified from that runner behavior.

Exact commands/selectors and helper behavior: `SCOPE-AUDIT.md` and `env.sh`.
Terminal logs/exits: `baseline-v2-{normal,oracle}.*`,
`fixed-v2-{normal,race,oracle,oracle-race}.*`, `sourceguard-*.log`, `vet.*`, `build.*`.
Structured totals: `test-results-summary.json`.
Full identities: `original-tracked-files.json`, `baseline-v2-all-files.json`,
`fixed-v2-all-files.json`, `after-*-all-files.json`, `replay-all-files.json`.
Inventories/provenance: `original-test-inventory.json`, `new-test-inventory.json`,
`source-provenance.json`; hashes cover prior V4c source/review and toolchain.

## Bounds and exclusions

This proves the bounded conversion slice only. It does not implement or accept
LIMIT-zero OFFSET suppression, EXPLAIN bound suppression, bound/ON/name binding
order, other planner ordering/error phases, broader optimizer/index work, or the
full held V4c composition. Existing newer grouping, counters and prepared-code
files are preserved byte-for-byte. The old V4c error-ownership/full limit-code
matrices are not copied, weakened or claimed to pass on current main.

No global broad suites, fuzz campaign, native Windows/macOS execution, CI,
physical faults, crash/process interruption, hot recovery, namespace interference,
missing-selected-file, wrong-key/tamper, interrupted publication/rekey, backup,
VFS, ANN, or unknown interrupted chain was run. Independent review and lead
integration remain separate pending gates. This worker makes no whole-project,
native durability or full-goal acceptance claim.

## Concurrent main observation

A final read-only comparison to original-main bytes observed documentation
changes in the lead working tree (see main-working-tree-observation.json).
The worker did not edit main. No main-working-tree-unchanged PASS is claimed;
all preservation/replay statements above concern the pinned tracked base and
the independent candidate/baseline copies, which are verified unchanged.
