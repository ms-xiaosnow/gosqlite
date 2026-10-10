# Ordinary oracle CI source and helper audit

Audit date: 2026-10-10. Source base: 883999c35b29ed31b4c4e0e3d66712b4a8ed996c,
with only this CI/test correction applied. No production source changes.

## Scope and clarification

The handoff initially abbreviated the restriction as prohibiting rekey. Before
running tests, source inspection found ordinary successful `DB.Rekey` calls in
`sqlite/encryption_test.go`, `sqlite/encryption_review_test.go`,
`sqlite/encrypted_wal_review_test.go`, `sqlite/encryption_shared_test.go`,
`internal/testkit/encryption_conformance_test.go`, and
`internal/testkit/encrypted_wal_conformance_test.go`.

The lead clarified before execution that successful rekey with intact files and
correct credentials, and normal encrypted copy/checkpoint/backup/restore, remain
permitted. The restriction covers interrupted rekey/publication, wrong-key or
missing/tampered files, injected native I/O/unmap/close/lock/Sync failures, short
writes, disk-full/permission scenarios, hostile namespace changes, and
crash/kill/forced process-exit/hot-recovery workflows. No tests were run before
this clarification. No positive rekey tests were skipped, removed, or gated.

## Corrected test and reachable helpers

- `TestOracleOrdinaryFreshFixtures/self_join_on` preserves the exact former
  unsupported query: `SELECT e.id FROM edge e JOIN edge f ON e.id=f.id`.
  It compares complete column labels, row counts, storage types, and values with
  the exact official SQLite oracle via the existing `confRun`/`confCompare`.
- `TestOracleErrorsAndResources` changes only that obsolete rejection to
  `SELECT e.id FROM edge e JOIN edge f USING(id)`. The exact
  `errors.Is(err, sqlite.ErrUnsupported)` assertion is unchanged.
- `confFresh` creates a new ordinary database using normal official SQL and
  checks its integrity. Both test bodies then open it read-only.
- `confUnchanged` checks the database digest and absence of new sidecars after
  read-only access; it does not remove, replace, damage, or truncate files.
- `confOracle` validates the version/source ID. `confCLI` and error comparisons
  use ordinary `exec.Command`, EOF and normal completion, with no kill hook.
- Remaining resource checks use context cancellation of native iteration,
  normal repeated Close, copied result slices, quotas, and intact fixtures.
  They do not cancel or terminate an external database process.

## Other JOIN rejections

A source search of every `_test.go` JOIN occurrence found no other stale native
SQLite JOIN rejection needing replacement. Existing fail-closed cases in
`internal/sql/from_test.go`, `internal/sql/sql_test.go`, and
`sqlite/sqlite_test.go` concern USING/NATURAL/RIGHT/FULL, grouping/subqueries,
other unimplemented syntax, invalid names, or ambiguity. Existing relational
oracle negative cases test documented SQLite errors, not generic JOIN denial.

`migration/postgres/convert_parser_test.go` and `convert_test.go` still reject
JOIN in the separate restricted PostgreSQL migration converter/parser. Those
are intentional converter-scope tests and do not exercise the SQLite parser.
They must remain unchanged.

## Full ordinary selection and limitations

The baseline full default/oracle sources had already received the lead's
behavioral audit recorded in `docs/test-scope-audit.md` and later checkpoint
evidence. This patch changes only the two read-only queries above; the other
ordinary test bodies and helper graph are unchanged. Additional source review
of oracle filesystem/process calls confirmed:

- intact fixture copies are written to fresh temporary paths;
- encrypted normal lifecycle tests use correct credentials and complete files;
- cooperative SQLite processes finish through normal SQL/EOF/Wait;
- no new restricted test, build tag, skip, fault injection, or physical fixture
  mutation is introduced by this patch.

Before each recorded suite command, run `go run ./internal/testscope`. It is a
reference guard, not a complete call-graph proof; behavioral audit remains
necessary. The CI workflow selects only default and `oracle` suites, never the
manual recovery/namespace tags. No upstream SQLite test suite is executed.

The worker's focused result is not a full-suite result. The lead owns the
corrected complete-source unit/vet/build/race/full-oracle acceptance and any
publication; those results must be reported separately.
