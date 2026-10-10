# Boolean ranking predicate repair: behavior audit

Base source: immutable combined tree 1336a5d7b5e1a336fb01e012334cec776a61c4a7.
The original lead full-suite failure log and exit are not modified.

All selected suites precede execution with `go run ./internal/testscope`.
No manual test tags are enabled. Existing ordinary-source exclusions and the
behavior audit in docs/test-scope-audit.md remain applicable. The parent verified
this exact combined tree's default suite scope before its failed full run; this
change alters no existing test body, helper or source-selection constraint.
Relevant inherited audits are docs/evidence/boolean-semantics/scope-audit.md and
docs/evidence/relational-execution-audit.md. Full sqlite and internal/sql package
checks, selected driver Boolean/vector/typed/ANN/expression/similarity checks,
and their matching race checks retain the audited ordinary scope.

New sqlite/boolean_rank_filter_test.go:
- Memory Rows use sparseReviewMemoryRows/halfReviewMemoryRows and a synthetic
  reviewVectorSource. Invalid typed strings never enter a physical file.
- Direct public filter tests hold only table metadata and values. They use no
  reader or descriptor. Errors are SQL overflow, invalid typed-cell access or
  explicitly canceled in-memory contexts.
- SQL/public search fixtures use reviewVectorDB (owned TempDir, Open, Close) and
  normal CREATE/INSERT/DELETE; HNSW/IVFFlat/grams indices are created through their
  ordinary public APIs on valid typed values. No native I/O failure is injected.
- The candidate-limit regression is a logical in-memory budget over ordinary
  rows. It preserves public API work charging even when every row is filtered.
- False/error startup is checked before graph traversal and query decoding;
  counts are logical work counters. Valid source vectors are never modified
  outside ordinary SQL.

New internal/testkit/boolean_rank_filter_oracle_test.go:
- Uses confOracle's exact version/source-ID check and normal writeExec fixtures.
- Readonly quote-mode SQLite subprocess reads supported scalar filters. It has
  no timeout/kill/interrupt path and exits normally at EOF.
- Native SQL and public exact/ANN rankings have equal vector distances, so the
  signed-rowid tiebreak equals ordinary SQLite ORDER BY id. Vector math itself
  is not claimed as a SQLite oracle comparison.

No physical corruption/tamper/wrong-key/missing-selected-file, namespace mutation,
physical race/crash/process kill/hot recovery, native I/O fault/lock/unmap/close/
Sync/short-write/disk-full, interrupted publication or rekey is executed.
