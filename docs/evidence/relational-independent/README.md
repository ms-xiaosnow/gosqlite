# Independent relational SQL review

Status: scoped checks PASS after repairs, with the explicitly documented inherited eager AND/OR semantic gap still open. This is not full SQL or database acceptance.

## Provenance and scope

Initial baseline: efe40d70b190eaa8a8138b49ebd295279c9a0813. Final baseline: 69008bf658611318154a112178913c693b679a2b.
Initial isolated owner SQL overlay manifest SHA-256: 39b9947b3b8cd8fe770e68471bd473d8f4d4a756968ac045242afed729de37ae.
The review's `source/` directory preserves that initial snapshot independently of the owner's evolving freeze; `initial-manifest-verify.log` verifies all 19 owned paths. Only reviewer test files were added there. No production edits, staging, commit, upload or external service changes were made by this reviewer.
Concurrent native-close and migration work was excluded. Production targets are native Go. Official SQLite was used only for explicitly tagged test oracle comparisons.

Every executed suite was preceded by `go run ./internal/testscope` and a behavioral audit. Executed native cases used intact SQL-created files and ordinary query/write/BUSY/cancel/close/reopen/checkpoint lifetimes. Negative parser/math/quota cases were memory-only. No physical crash, fault, corruption, tamper, wrong-key, missing-file, namespace replacement, native I/O/close/unmap/lock/Sync failure, disk-full, permission, process-exit or hot-recovery scenarios were run. Restricted manual sources remained excluded.

## Initial blockers reported to implementation owner

1. Boolean ORDER BY is not integer ordinal syntax. New JOIN binding called integerConstant before binding physical TRUE/FALSE names, yielding wrong ordering or ordinal-zero errors. Existing single-source integerConstant also mishandled unshadowed TRUE/FALSE constants. Independent oracle reproductions distinguish old and new regressions. Quoted Boolean output aliases in ON/WHERE were also incorrectly ignored by the initial JOIN binder.
2. Physical Boolean projection labels lost declared spelling. `SELECT TRUE,FALSE` and parenthesized forms over physical lowercase true/false columns returned expression spelling rather than SQLite's declared column labels.
3. Cursor parse reservation was too small. PageSize*4+512 did not cover per-cell cell and span arrays: their amd64 sizes are 48 and 16 bytes, respectively, and valid dense small-record leaves can contain approximately page size divided by 6 cells. Owner implemented exact preflight and a valid in-memory page regression.
4. Previous emitted result lifetime was under-accounted. `fetch` released resultBytes while `Rows.current` still retained prior output through allocation of the next row. A descriptor-free source callback reproduced this; owner moves current release before the next fetch.
5. Sort TEXT/BLOB comparison work ignored operand bytes. Two distinct 10001-byte common-prefix strings compared successfully with Work=1 under MaxWork=100.
6. Searched CASE/ON/WHERE truth conversion and LIKE ESCAPE coercion bypassed byte-work/scratch preflight. Pure-memory CASE scanned a 10001-byte numeric string with Work=3 / Allocate=0; LIKE ESCAPE coerced a 10002-byte BLOB with Work=5 / Allocate=128.
7. Alias-expanded planning was uncharged. A 90-term result expression reused 91 times in WHERE with LIMIT 0 completed planning at Work=3 under MaxWork=100; no row execution was needed to expose it. An initial incorrectly named alias collided with existing physical x columns and was corrected before the decisive v3 evidence; that preliminary non-quota error is not used as proof.
8. Once scope.resolve gained work errors, label resolution initially swallowed them. This was caught during source review and reported before corrected freeze acceptance; a quota error must never become a successful fallback label.

9. Engine-owned parameter cloning preceded the new cap. An 8 MiB caller BLOB with MaxBytes=1 caused8,403,896B of engine allocations inside Query before rejection. Owner is moving the root budget before parameter binding, while explicitly excluding caller Valuer callback allocation.

The lead independently identified LIKE's 8x scratch underestimate. Source review concurred: pattern runes plus append-grown 8-byte term buffers exceed it. Owner added conservative LIKE and float decimal-conversion scratch reservations.

## Independent testing material

- `internal/testkit/relational_independent_test.go`: seeded 420 two-source plus 160 three-source property comparisons; targeted Boolean order/metadata/alias cases; duplicate labels, rowids, mixed inner/LEFT/comma joins, ON versus WHERE null semantics, and NULL order/LIMIT edges. Official quote-mode output retains storage classes and duplicate labels.
- `sqlite/relational_independent_budget_test.go`: descriptor-free previous-result and sort-work laws, plus an intact fixture LIMIT 0 planning-work regression.
- `internal/sql/relational_independent_budget_test.go`: pure-memory truth and escape work limits.

Initial 420 seeded two-source cases passed; targeted initial failures are preserved separately, never represented as passing acceptance.

## Architectural observations

The implementation is a genuine pull nested loop. Right-side restarts create native cursors against the same borrowed PageReader; no recursive DB.Query or fresh snapshot acquisition appears in child execution. The root Rows retains sole reader release, and transaction read detection includes Relation while one root tracks activeRows. Left-match state is independent of post-join WHERE, enabling correct null extension. Expressions bind to private slots with per-leaf hidden nullable rowid identity and declared types. Forward ON references remain deliberately unsupported, rather than being rewritten into different SQL. Single-source routing and index/ANN planning remain separate.

Remaining stage boundaries include unsupported USING/NATURAL/right/full/derived/parenthesized/aggregate/CTE/compound/window forms, no index/ANN join pushdown, no spill, and no general SQLite compatibility or durability claim.

## Final corrected-source verification

Final SQL manifest SHA-256: 4dd07287c3687ee466b51b7d6c343261aeb997ff81c01f75bdfe0188b85a9397, covering 27 owned paths.
Final owner source: ../relational-sql-final-4dd07287 (declared immutable).
Independent tested source: ../relational-independent-review/source-final-v3.
The final snapshot was created from durable baseline 69008bf658611318154a112178913c693b679a2b plus the SQL overlay. The sqlite.go adapter is a one-line Query binding change reconstructed separately from baseline; concurrent uncommitted encryption/migration edits were excluded. Baseline-isolation verification independently checks every other tracked file's Git blob identity.

Seven suites passed with exit 0 and no skips, each preceded by testscope and the behavioral audit recorded in its log:

1. scoped-unit: new relational parser/cursor/native/driver cases plus the reviewer memory/work/name regressions, CGO_ENABLED=0.
2. exact-oracle: pinned SQLite 3.54.0 relational conformance and 604 reviewer comparisons, consisting of 420 seeded two-source cases, 160 seeded three-source cases, and 24 targeted Boolean/label/scope/null/order cases.
3. legacy-regressions: narrowly selected existing scalar/parser, ordinary-index and exact-vector rank regression cases.
4. scoped-race: same scoped native unit/driver/reviewer laws under the Go race detector. CGO_ENABLED=1 is limited to race instrumentation.
5. scoped-vet: changed native packages, CGO_ENABLED=0.
6. cgo0-build: all production packages compile with CGO_ENABLED=0; this does not execute a broad test aggregate.
7. btree-parser-regressions: existing immutable readonly fixture scans at 512/4096/65536-byte page sizes, schema/interior/overflow reads, normal cursor cancellation, and descriptor-free malformed-page parser checks.

All initial reviewer regression failures are fixed on these bytes. The BLOB preflight case now allocated 3,480 bytes before rejecting MaxBytes=1, versus 8,403,896 bytes on the initial implementation. The implementation owner's final reserve-cancellation correction was separately inspected and included in the last complete rerun.

Version/source identity checked by the oracle harness:
SQLite 3.54.0 | 2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3.
Go version and executable SHA-256 values are preserved beside the logs. All 27 final SQL hashes verify after tests. Full-source hashes and reviewer test hashes are also retained.

The intervening 9a797377 candidate and 78868c documentation-only candidate have separately preserved logs. 78868c changed only the guide to disclose inherited eager AND/OR behavior; final 4dd07287 additionally changed reserve cancellation and its pure-memory test. Prior results were not silently relabeled as the final revision.

## Remaining correctness boundary

The inherited evaluator still eagerly evaluates binary AND/OR operands. The final guide explicitly documents native integer-overflow errors for predicates that SQLite constant-folds or short-circuits successfully, including ON/WHERE `0 AND abs(-9223372036854775808)` and WHERE `1 OR abs(-9223372036854775808)`. CASE branch laziness and successful JOIN comparisons do not establish complete predicate/error equivalence. This remains an actual code gap, not a passed acceptance gate or an upstream error. General unsupported grammar, full durability qualification, native non-Linux runtime, and broad integration acceptance remain outside this review.

## Reproduction artifacts

The committable bundle contains final logs and exit codes, initial failing evidence, all 27 source hashes, full tested-source hashes, the exact review runner, and the three reviewer regression sources saved with .go.txt extensions. To reproduce, copy those sources back to their documented package paths in the immutable final checkout and run the runner with that source and an evidence-directory argument. Tests never require a production SQLite subprocess; only the explicitly tagged oracle suite uses the pinned test executable.
