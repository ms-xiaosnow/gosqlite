# SQL connection counters

The bounded native SQL profile implements the zero-argument functions
`changes()`, `total_changes()` and `last_insert_rowid()`. Each returns an SQLite
INTEGER (Go `int64`). Wrong arity is an ordinary SQL error, including dead arms
and `LIMIT 0`. Names use the existing ASCII case-insensitive function lookup.

## Connection state and statement outcomes

Each newly opened native `DB` is a separate connection with all three values
initially zero. State is memory-only, survives ordinary transaction/savepoint
rollback, and resets on closing and opening a new connection. Another connection's
writes never affect these values. The state is not persisted in a database page,
WAL, extension table or transaction undo record.

- Successful INSERT/UPDATE/DELETE sets `changes()` to its directly affected row
  count and adds that count to `total_changes()`. An UPDATE counts matched rows
  even when assigned values equal existing values. A zero-row UPDATE/DELETE sets
  `changes()` to zero and adds nothing to the total.
- `last_insert_rowid()` changes after each successful row insertion, including an
  explicit zero or negative rowid. UPDATE, including rowid-changing UPDATE, and
  DELETE leave it unchanged. A later failing row can roll back the statement's
  data without restoring the last successfully inserted rowid.
- Constraint/expression failure after DML execution starts resets `changes()` to
  zero and adds nothing to `total_changes()`. Earlier successfully inserted rows'
  last rowid remains observable even though their data was rolled back. SQL
  parsing, binding/name/arity errors and unsupported-feature rejection before
  execution preserve prior counters.
- DDL, index maintenance and transaction/savepoint controls leave all three
  unchanged. Successful statements inside an explicit transaction update the
  counters before COMMIT. COMMIT, ROLLBACK and ROLLBACK TO do not recompute or undo
  them. An automatic DML statement adds its row count only after commit succeeds.
- Ordinary writer-promotion BUSY after DML preparation sets `changes()` to zero. An automatic
  commit BUSY contributes no rows to the total, but retains a successfully
  inserted rowid. A BUSY explicit COMMIT leaves the earlier completed statements'
  counters unchanged, just as it leaves that native transaction retryable.

The functions observe their actual evaluation context, not a per-query snapshot.
During multirow INSERT, `changes()` and `total_changes()` retain the preceding
completed statement's values, while `last_insert_rowid()` can observe the prior
VALUES row's successful insertion. They are not deterministic expressions and
are excluded from constant substitution/startup hoisting. Scalar projection,
WHERE/ON, ordering, limits, aliases, and supported global aggregates use the same
connection state. Existing eager sorting can materialize results before Rows
iteration; a streaming projection reads state when that row is evaluated.

## Native and database/sql Result

The existing `Result` contract is preserved. It is a statement result, not a
snapshot of the connection SQL functions:

- `RowsAffected()` is this successful statement's direct row count. DDL and
  transaction controls return zero.
- `LastInsertId()` and `LastInsertID()` return the final inserted rowid for a
  successful INSERT. They return zero for UPDATE, DELETE, DDL and controls, even
  when the SQL `last_insert_rowid()` retains a nonzero value.
- After an execution error, disregard the returned Result; no successful result
  is supplied by database/sql. The native error path returns the existing zero
  Result.

No exported configuration/result fields or their types changed. `DB` gained
private lock-owned state; its private Go struct layout therefore changed.
No disk format changed. Driver prepared statements reuse the same native
connection, so counters carry across repeated execution. For direct SQL counters,
use a pinned `sql.Conn` or `sql.Tx`: separate pooled `sql.DB` calls can select
unrelated physical connections. ResetSession rollback of an abandoned transaction
preserves the physical connection's counters and does not make its data committed.

## Bounded scope and safety

This work does not implement general triggers, foreign keys, REPLACE/IGNORE/FAIL/
ROLLBACK conflict policies, views, virtual tables, INSERT SELECT, RETURNING, or
WITHOUT ROWID. The catalog/parser continue to reject those unsupported mutation
semantics. Their auxiliary-row accounting is not claimed. Engine-owned grams13/
ANN maintenance is implementation bookkeeping and is excluded from these direct
source-row counts. SQL calls in persisted DEFAULT expressions remain unsupported
under the existing default-expression restrictions.

The existing active-Rows restrictions remain: transaction-owned Rows reject a
write/control with BUSY before execution and preserve counters. Session-backed
connections retain their single local lease. An autocommit rollback-mode Rows
lease can make an attempted automatic commit BUSY. This is not a claim of SQLite's
more permissive same-connection read-cursor/write behavior. For supported ordinary rollback-mode schemas retained within the metadata budget,
[cached preparation](sql-cached-preparation.md) now distinguishes invalid DML
(preparation error, counters preserved) from valid DML whose first snapshot is
blocked (BUSY, changes resets to zero). Fresh/no-cache, session-backed,
extension-owned and unsupported catalog states retain conservative admission
behavior. The linked scope documents cache population, invalidation, quotas and
remaining SQLite preparation differences; full preparation parity is not claimed.
Pre-canceled contexts are rejected without changing counters. Cancellation after a row insertion rolls
back the native statement's data, publishes no total, and keeps that successful
rowid. Native cancellation retains its existing statement-local rollback behavior;
it is not a claim of complete `sqlite3_interrupt()` transaction semantics.

All evaluation and mutation is serialized by the existing DB mutex, including
Rows.Next. The internal expression context holds the same connection-owned state
pointer; it never re-locks DB, uses no global side table, and installs no per-call
function map/closure. One fixed context node is retained per query/write, not per
row or function call; the existing relational statement fixed-state allowance
covers this context node. Existing Go SQL-value boxing still applies.

## Evidence and reference

Pinned public SQLite 3.54.0 source ID:
`2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3`.
The reference `src/sqlite.h.in` documents the three connection APIs;
`src/vdbe.c` OP_Insert updates the last rowid, and `src/vdbeaux.c`
`sqlite3VdbeHalt` / `sqlite3VdbeSetChanges` distinguish statement completion,
statement rollback and connection totals. Public reference documentation:
[changes](https://www.sqlite.org/c3ref/changes.html),
[total changes](https://www.sqlite.org/c3ref/total_changes.html),
[last insert rowid](https://www.sqlite.org/c3ref/last_insert_rowid.html).

- `sqlite/connection_counters_test.go`: native statement outcomes, explicit and
  automatic rollback, active Rows, connection isolation, reopen, extension
  bookkeeping exclusion and concurrent serialized writes.
- `sqlite/connection_counters_internal_test.go`: deterministic ordinary context
  cancellation after a successful row insertion.
- `driver/connection_counters_test.go`: pinned connections, prepared reuse,
  statement-specific Result values and pool-reset rollback.
- `internal/sql/connection_test.go`: scalar context, arity and nondeterminism.
- `internal/testkit/connection_counters_conformance_test.go`: complete typed
  values and column labels across 512/4096/65536-byte rollback/shared-WAL histories,
  runtime/preparation error categories and native primary codes, plus live
  ordinary BUSY outcomes. Expected logical failures finish by ordinary CLI EOF.

Only selected normal/race/oracle/build/vet checks qualify this slice. These are
ordinary intact-file results, not evidence for crash, corruption, failed native
I/O/cleanup, namespace replacement, hot recovery or interrupted publication.
