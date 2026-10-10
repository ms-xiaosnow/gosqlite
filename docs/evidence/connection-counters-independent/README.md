# Independent SQL connection-counter review

## Verdict

The exact implementation freeze `7c8f7a266a63a713b22709268fdd4d25f1b875d1` passes the selected independent review for its documented bounded profile. No unresolved implementation defect was found within that profile. This is not full SQLite counter/preparation parity, a broad aggregate pass, physical safety qualification, native-platform qualification, or completion of a project milestone.

One important compatibility boundary was identified before freeze and corrected/documented by the implementer. The first draft treated every snapshot-acquisition BUSY as an executed DML statement and reset `changes()`. This could not distinguish an invalid table reference from valid DML. The frozen implementation conservatively preserves counters before snapshot/catalog admission. Under a peer's EXCLUSIVE lock, official SQLite's cached schema can return a preparation error for invalid DML (preserves counters) while valid DML reaches execution BUSY (resets changes to zero). Native admission cannot yet make that distinction. The lead accepted this explicit limitation; exact parity requires separately bounded schema/statement preparation support, not a guessed reset.

The strict official valid-DML expectation deliberately fails against the freeze and remains in `strict-official-busy-negative.txt` and its exact source snapshot. This is negative evidence, not a passing parity test. The final scoped test asserts the documented native boundary. Validated writer-promotion BUSY and automatic/explicit commit BUSY remain independently tested against pinned SQLite.

## Exact identity

- Baseline owner tree: `7d3184f87165e11e12560884ec6e4f92c9bfd039`
- Frozen owner tree: `7c8f7a266a63a713b22709268fdd4d25f1b875d1`
- Owner full manifest SHA-256: `f05142b0c8e291c41614596e45013e3f87dc5ee4389792b9ea1671c14414710f`
- Owner source patch SHA-256: `7a09d38be5ccb2fd18b11e3f74a1d57ba8b98e04ef7bd6a56491dd5b8ec0099c`
- Independent three-test-file patch SHA-256: `90db24227f5809592b3c2ac7aef3cdaed528e8bf98e6385b6e0def8d6be40bdf`

All 1,259 owner files were checked before/after review. Applying the exact owner patch to a fresh byte-verified baseline reproduces every path and byte in the frozen full manifest. The review copy adds only three independent test sources outside this evidence directory; no owner source changed. Final review inputs also pass a separate post-run manifest check. See `freeze-full-before.txt`, `freeze-full-after.txt`, `freeze-owned-before.txt`, `freeze-owned-after.txt`, `patch-replay-full-identity.txt`, `patch-full-path-set.txt`, `full-source-delta-verification.json`, and `review-final-input-after.txt`.

## Evidence and outcome

`run-selected.sh` records exact commands/environment. `selected-checks-final.txt` and `.exit` record final exit 0, ending 2026-10-10 20:22:55 UTC. Each normal, race, oracle, oracle-race, vet and build batch was preceded by `internal/testscope` and an explicit source-behavior audit. Go was the pinned official 1.27.2 with offline modules, `GOFLAGS=-buildvcs=false`, and shared caches; race checks separately enabled the Go detector runtime.

Passed:

- Selected internal scalar/native/driver tests with `CGO_ENABLED=0` and focused `-race`.
- All six owner typed histories across page sizes 512/4096/65536 and rollback/shared WAL, plus two independent typed rollback/shared-WAL histories. Whole output compares values, labels and numeric/text storage classes; logical SQL failures compare native primary codes and official error categories.
- Function arity/dead-arm errors, ordinary live BUSY comparisons and the explicit early-admission divergence assertion; the same selected oracle checks under `-race`.
- Affected production package vet/build and oracle-tagged testkit vet. The oracle testkit is test-only, so its compilation is checked by `go test`, not `go build`.
- Prepared reuse, pinned connection isolation and pool reset; initial state, DDL/zero rows, insert/update/delete, negative/zero IDs, dynamic last-rowid expressions within VALUES, same-value UPDATE counts, partial multirow NOT NULL/duplicate/mismatch ABORT, ordinary transaction/savepoint rollback, pre-work and deterministic post-row cancellation, extension-maintenance exclusion, concurrent reads/writes, close invalidation and reopen.

Independent tests: `sqlite/connection_counters_independent_test.go`, `driver/connection_counters_independent_test.go`, and `internal/testkit/connection_counters_independent_test.go`. `independent-tests.patch` adds those files and may be adopted by the lead. Source hashes are in `independent-test-final-sha256.txt`.

## Preserved failures and negative controls

- Initial baseline command used an incomplete shared module cache and failed setup offline; retained as `baseline-negative-control.txt`. Corrected cache run reaches the expected missing-feature failure (`function changes`) in `baseline-negative-control-corrected.txt`.
- Initial Python oracle script put a counter query on the same line after a failing statement; SQLite skips that remainder. Original script/output are retained. Corrected separate-line scripts show complete tuples, including valid/invalid DML behind EXCLUSIVE, in `oracle-busy-preparation.txt`.
- Strict official EXCLUSIVE BUSY parity expectation fails for the accepted native admission boundary, as described above.
- The first extended oracle test requested unsupported GROUP BY; retained in `initial-selected-unsupported-group.txt` and the exact test snapshot. The final oracle uses the already-supported global aggregate, while a separate native test asserts GROUP BY rejection and unchanged counters.
- The next harness wrongly required `ErrUnsupported` for all unsupported grammar. Existing parse-time rejection may be `CodeError` without that sentinel. `selected-checks-v2.txt` preserves the failure; final tests require an explicit SQL error and unchanged counters. No production error behavior changed.
- `selected-checks-v3.txt` passed normal/race/oracle/oracle-race/vet and production build, then attempted `go build` on the test-only testkit package. Its command error is retained. The final script builds production packages and compiles testkit through selected oracle tests.

These were not hidden or counted as passes. No runtime assertion was weakened to admit incorrect supported values or errors.

## Source review and remaining missing code

`source-audit.txt` and pinned source/tool hashes record inspection of DB/Tx/Rows lock ownership, admission/preparation/runtime outcome boundaries, driver reuse/reset, per-row insertion state, ordinary indexes/extensions, nondeterministic evaluator behavior, and disposal. The new volatile state is private, per connection and independent of pager/savepoint undo. Existing statement-specific Result layout and values are preserved; no disk format changed.

SQLite reference source is public 3.54.0, source ID `2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3`. Oracle subprocesses finish via normal EOF/.quit and Wait, including expected SQL errors.

General trigger/FK/conflict-policy auxiliary accounting, INSERT SELECT, RETURNING, views/virtual tables, WITHOUT ROWID, grouped/window aggregates, persisted counter-call defaults, full SQLite interrupt semantics and relaxed active-cursor semantics remain absent or explicitly unsupported. The early snapshot-admission/cached-schema gap remains missing code. These ordinary intact-file checks do not cover corruption, tamper, wrong keys, physical recovery, namespace replacement, interrupted publication, crash/kill, or native I/O/lock/cleanup/unmap/Sync failure. No such workflow was run.
