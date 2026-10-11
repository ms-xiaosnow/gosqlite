# Independent LIMIT conversion review

## Verdict: accepted for this bounded slice

The exact final candidate, tree `3fc9087ad121d423174a658e118ac455d1ace8c2`, passes independent source and execution review. No production correction is requested.

- 1,016 native leaf assertions: PASS normal and race.
- 344 fixed official-CLI comparisons: PASS normal and race, all 344 completed.
- 118 additional independent finite conversion, full-value accounting and ordinary reader-lease controls: PASS normal and race.
- Zero skipped tests in any selected run.
- Source guard, scoped vet/build and CGO0 production dependency closure: PASS.
- Exact source patch replay reproduces all 2,964 final files and the final tree.

This accepts only the small `limitValue` conversion change and its two new dedicated regression files. It does not accept the old broad planner phase/order work, LIMIT-zero or EXPLAIN behavior, bound-name phase fixes, general SQLite parity, integrated main or whole-project readiness. It does not change or qualify grouping, counters or preparation code.

## Source identity and preservation

Baseline commit: `63c2ebb10e3a2c4ff2378ee9f33c46133134c70f`.
Baseline committed tree: `d735409ebd1c43d90ee1650c8a7c2a084e270674`.
Original production plus final v2 test bytes: `80d2ac039453de28581ea6f499d3914f65fee965`.
Final candidate: `../limit-conversion-work`, independently copied to `candidate/`.
Final tree: `3fc9087ad121d423174a658e118ac455d1ace8c2`.
Full patch SHA256: `b712838805fadd0215464c828169226425170df00377a3c13df424cb0a4bc4bc`.
Production-only patch SHA256: `547715789a82c42824013610fa46e35e4f95b36c2f783704d3892dadec80f5e5`.

Exactly three paths differ from the baseline:

- `sqlite/query.go`: `b7165a94335ef289ebea33e8a5dbcd0f469d43b0f362e66ffa92bd422266dcb4`
- `sqlite/limit_conversion_test.go`: `3676eb73e1889270a32f80eb38a5fca6ebc455258ff7818a9bfd88c3d4d56186`
- `sqlite/limit_conversion_oracle_test.go`: `3f57fa8b02c34e61c6d1d635731c81d847f2a3dd67a715b80852508d0762e242`

All 2,961 other original tracked files, including all 360 original test files, are unchanged byte-for-byte and mode-for-mode. `internal/sql/eval.go`, its affinity/normalization machinery, all bound call sites and grouping/counter/preparation production are unchanged. Within query.go, the only differences are `limitValue` and deletion of the now-unused strconv import. The new function is byte-identical to the formerly independently reviewed V4c function, SHA256 `8b4525e2c5de309824ce15181ca554ca2ff97cdaef4d4b22b75923fba4b595a5`; no other old V4c code was transplanted.

`verify.py`, `source-proof.json`, full candidate/baseline inventories, `patch-replay-proof.json` and `final-source-verification.log` record the independent proof. Tree computation uses independently recomputed Git blob/tree hashes, SHA256, lengths and executable modes. The patch was independently checked/applied to a fresh archive of the exact baseline, not to the owner's working tree. Candidate, baseline and replay remain unchanged after all tests. The supplement is a separate owned copy that adds only one independent test file; its source and complete inventory are retained.

## Actual original failures and unchanged assertions

Fresh independent v2 baseline execution reproduced the owner's exact failure-name inventories:

- Native: 386 PASS, 630 FAIL, 1,016 leaves total; process exit 1.
- Oracle: 136 PASS, 208 FAIL, 344 leaves total; process exit 1.

The same exact test bytes produce 1,016/1,016 native and 344/344 oracle passes on the fixed source. The baseline source is independently reconstructed by replacing only query.go with the exact committed baseline version. Original/fixed selected test names are identical. No failed original assertion was removed, downgraded, normalized or made conditional on missing infrastructure.

The baseline includes real wrong grammar acceptance, embedded-NUL conversion failures, endpoint differences, ErrMismatch ownership and public Code 20 mismatches. The full per-leaf names, failures and skips are in `test-inventory.json`; raw unabridged logs and exit files are retained. All nine scalar error-ownership controls already pass on the baseline and still pass on final code.

### Oracle v1 transport error is preserved, not counted as a production pass

The first oracle helper incorrectly required adjacent text `datatype mismatch (20)` in the ordinary stdin diagnostic. This pinned shell prints the text without the code there. The v1 baseline oracle log has 184 harness transport failures plus 24 native production failures; its fixed counterpart has 184 harness failures and 160 successes. Those v1 runs do not establish 344 valid comparisons.

The v2 correction changes exactly two helper lines: enable the existing CLI `.log stderr` callback and require the leading actual callback `(20) ` plus datatype-mismatch text. It preserves natural exit status 1, empty stdout, native ErrMismatch, public CodeMismatch 20, nil Rows, all success-row comparisons and lease assertions. Every fixed SQL expression, shape and native query remains unchanged. There is no extra environment variable, build helper, loaded extension or C observer.

Independently inspected pinned shell/source bodies establish that main_init registers `shellLog` with SQLITE_CONFIG_LOG; shellLog prints `(%d) %s` from the actual callback error argument; sqlite3VdbeLogAbort calls sqlite3_log with the actual VDBE rc before appending the statement text. The helper checks the beginning of stderr, so a `(20)` embedded later in SQL cannot satisfy the code check. `output_file_open` maps `stderr` directly to the existing stderr stream. The source also explains why argument-mode SQL did not supply the originally assumed code format; that attempted transport was not adopted.

The original v1 source/tests/logs, exact two-line diff and original/fixed corrected v2 sources/logs are retained in `owner-evidence/`, with a complete snapshot hash manifest. `harness-history.json` records transport-failure counts. Fresh independent corrected v2 baseline oracle has zero harness failures. The sole configuration is existing `SQLITE_ORACLE` plus existing `-tags oracle`; absence or wrong pinned identity is fatal, never skipped.

## Production reasoning

Pinned SQLite 3.54.0 OP_MustBeInt applies NUMERIC affinity and then requires INTEGER storage, raising SQLITE_MISMATCH otherwise. Reusing unchanged native SQL ApplyAffinity makes the LIMIT grammar consistent with the ordinary decimal affinity implementation and removes Go-only hexadecimal, underscore and Unicode-whitespace acceptance.

Exact INTEGER/TEXT endpoints remain distinct from REAL endpoints: REAL MinInt64 rejects, whereas INTEGER MinInt64 and exact decimal-integer TEXT MinInt64 accept. Nonfinite REAL bounds, NaN normalized to NULL, fractional and subnormal bounds reject with ErrMismatch/Code 20. Finite representable neighbors and signed zero behave as expected in the direct conversion controls.

The embedded-NUL special case is narrow and ordered correctly: the whole evaluated value is preflighted before NUL search/prefix conversion; only then the prefix follows REAL followed by NUMERIC affinity. This reproduces the pinned zero-terminated float parser versus length-aware exact-integer fallback, including precision rounding and endpoints, without changing global SQL affinity.

Eval errors and full-value memory/work/context errors are returned before conversion and remain their original errors. Only actual rejection of the converted value wraps ErrMismatch. Direct budget sentinels, cancellation/deadline and overflow controls verify that distinction, with public overflow retained as CodeError rather than CodeMismatch.

## Execution and supplemental coverage

Pinned Go 1.27.2; official SQLite 3.54.0 source identity `2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3`.
Go executable SHA256 `548d3c32f83fa81152f543eb1526be7884cfe0526eb5e9ce29d3eca14786e7e0`.
CLI SHA256 `470775f6ada29ccfb2d3046ac2b4c975df95a9c20b782332d4e8ba6b6233e6c6`.
Pinned amalgamation SHA256 `2da161e310ef0aba3465f22a4d12b8a43723236f0942fa76e8aac523c3317243`.

All selected test bodies, complete local helper chains, scalar-budget helpers, relevant public query/transaction ownership code and the entire AST source guard were read before execution. No sqlite/internal-sql test init or TestMain was present. Default /tmp was retained; capacity was checked before copying/execution and remained sufficient. No cleanup, relocation or prohibited retry was needed.

`run.sh` records the exact commands and environment. It selects only:

1. `^(TestLimitConversionTypes|TestLimitConversionErrorOwnership|TestLimitConversionPublicSQL)$`, normal CGO0 and race CGO1.
2. `^TestLimitConversionPinnedOracle$`, with the oracle tag, normal CGO0 and race CGO1.
3. The inspected source guard, `go vet ./sqlite ./internal/sql`, `go build ./sqlite ./internal/sql` and production dependency listing.

All test invocations use `-timeout=0 -count=1 -v`; no timed interruption is used. The production dependency listing contains no CgoFiles. Full test stdout/stderr, selectors, timestamps and process statuses are saved. No broad/global suite was selected.

The native tests comprise 63 typed conversion leaves, 9 scalar ownership leaves and 944 public DB/Tx leaves across ordinary single-source, LEFT JOIN, global aggregate and grouped query shapes. Valid zero-valued public bounds are deliberately outside this slice; direct zero conversion remains covered. This is selection of bounded new tests, not a skip or removal of an existing assertion.

The fixed oracle corpus is 43 literal expressions × 4 query shapes × LIMIT/OFFSET = 344 comparisons. CLI runs are ordinary in-memory SQL and natural normal/error completion; the native file is a correctly created, intact plaintext database. Exact integer-valued rows are compared without float-JSON coercion.

`run-supplement.sh` selects exactly `^(TestIndependentLimitFiniteConversions|TestIndependentLimitFullValueAccounting|TestIndependentLimitPublicLeaseRelease)$` in the separate `supplement/` copy:

- 90 direct leaves: 30 additional fixed values under literal, parameter and coalesce evaluation, including six individual ASCII whitespace characters, exact signed endpoints, NUL precision rounding in both directions, malformed prefixes, Unicode around NUL, subnormals and outside-endpoint REAL values.
- 12 full-value leaves: exact work and allocation accounting over the complete NUL-containing value, preserving work/cancellation/deadline sentinels before conversion.
- 16 public leaves: eight rejected typed values through DB and read-only Tx. Nil Rows, Code 20 and zero active-query leases are required. An ordinary same-file peer UPDATE after rejection or normal transaction rollback succeeds, followed by a normal positive read. This checks real reader ownership release beyond the bookkeeping map.

All 118 leaves pass unchanged normal/race. The supplemental file is review evidence, not one of the three proposed integration paths.

## Boundaries

No main/owner/frozen production, Git index, remote, workflow or runner was modified. No physical fault, native I/O fault, crash/kill/abrupt exit, namespace interference, hot recovery, missing-selected-file/wrong-key/tamper, interrupted publication/rekey, ANN V4 or 41-target fuzz campaign was run. The only new external processes were approved bounded Go operations and ordinary pinned official SQLite CLI completion.

No further fix is required for this candidate. Integration and any broader project claims remain a separate parent gate.
