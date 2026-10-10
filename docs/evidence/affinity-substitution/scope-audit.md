# Affinity correction test-scope audit

Reviewed before execution, 2026-10-10 UTC. This supplements the accepted Boolean
source audit at `docs/evidence/boolean-semantics/scope-audit.md`; no storage or
resource-management production changes are part of this patch.

- `internal/sql/affinity_comparison_test.go`: parser/scalar values in memory only;
  canceled contexts and synthetic in-memory budget objects, no file handles.
- `sqlite/affinity_comparison_test.go`: native SQL-created intact temporary
  database; normal statements, indexes, UPDATE/DELETE, rollback, close/reopen.
- `internal/testkit/affinity_substitution_conformance_test.go`: the unchanged
  2160-case independently corrected historical affinity matrix, renamed only;
  native SQL fixtures, read-only pinned SQLite child processes, normal EOF.
- `internal/testkit/affinity_comparison_oracle_test.go`: ordinary native SQL
  fixtures, read-only pinned batched oracle with temporary shell parameter
  bindings (no persistent data changes), standard WALShared choice, normal
  index creation, normal UPDATE commits and DELETE rollback, integrity_check.
  No child is canceled or signaled. Comparison checks complete values/classes.
- `oracle-isolation.sql`: pure in-memory pinned CLI table and EXPLAIN only.
- Existing scalar, Boolean, relational, ordinary-index and expression suites:
  retain the accepted Boolean source's audited selection. Typed-invalid input
  cases are descriptor-free memory values, not malformed database bytes.

Every suite begins with `go run ./internal/testscope` and checks this audit.
No manual tags, physical tampering/corruption, missing-file/wrong-key tests,
namespace mutation, native failed-I/O/close/unmap/lock/Sync/short-write/disk-full,
permission changes, interrupted publication, process killing, hot recovery or
crash testing are executed. Runtime production code remains pure Go.
