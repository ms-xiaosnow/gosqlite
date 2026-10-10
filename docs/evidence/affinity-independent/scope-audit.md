# Independent comparison-affinity review scope

Source review before execution, 2026-10-10 UTC. Compiler maintenance hold was
observed; lead explicitly resumed compilation at 19:07 UTC. No cache, fixture,
process, source freeze, or module cleanup was performed by this reviewer.

## Selected behavior and helpers

- `internal/sql/affinity_comparison_test.go`: parser, maps, scalar values, and
  descriptor-free cancellation/work/byte quota objects only. The existing
  `scalarTestBudget` is a pure memory counter/error source.
- `sqlite/affinity_comparison_test.go`: owned temporary file created by normal
  Open/CREATE/INSERT; intact indexed reads, SQL UPDATE/DELETE, explicit normal
  rollback, ordinary Close and reopen, then normal cleanup by the test runtime.
- `internal/testkit/affinity_*_test.go`: owner historical and expanded matrix
  helpers launch only pinned CLI with normal EOF completion; no timeout,
  cancellation hook, signal, termination, environment fault, or raw DB write.
- Reviewer `affinity_independent_review_test.go`: ordinary 100-row SQL fixture,
  paired left/right inputs, read-only CLI comparison batches, index creation,
  normal close. Only the test harness code is added to an isolated source copy.
  The reference batch requires contiguous unique markers, keeps empty results,
  checks every complete row via reflect.DeepEqual, and checks native iteration
  and Close errors. No truth-only projection or floating-point text formatting
  is used as the comparator. Both raw operand storage values and typeof values
  are projected; comparison result values/classes are checked too.
- Shared helpers individually read: `confOracle` checks regular CLI executable
  and exact sqlite_version/sqlite_source_id, with ordinary process completion;
  `confQuoted` is an in-memory INTEGER/REAL/TEXT/BLOB/NULL quote-mode parser;
  `confCLI` launches a normally finishing process; `writeExec`/`writeTxExec`
  perform ordinary transactions; `writeOpen` normal Open/Close; `writeSame`,
  `confExpected`, `confNative`, `confCompare` compare normal read results;
  `writeIntegrity` performs only read-only PRAGMA integrity_check. Test names
  are selected explicitly; unrelated helper-file tests are not run.
- Supplemental pinned CLI probe is pure `:memory:` CREATE/INSERT/SELECT/EXPLAIN
  and real `.parameter` binding; normal EOF completion only.

Before every Go test/vet/race suite, run `go run ./internal/testscope` with the
same offline pinned Go toolchain and shared cache/path. Review is not delegated
to that reference guard; this audit covers the behavior and helper call graph.

No physical corruption/tamper, wrong-key/missing-file fixtures, namespace
mutation tests, process/crash/hot-recovery tests, native failed I/O/close/unmap/
lock/Sync/short-write/disk-full/permission paths, interrupted publication or
rekey. No external service, main production edit, commit, staging, or upload.
