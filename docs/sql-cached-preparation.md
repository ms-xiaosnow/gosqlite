# Bounded cached-schema preparation for early BUSY

## Scope

This slice closes the connection-counter slice's ordinary rollback-mode
cached-schema admission gap for supported INSERT, UPDATE and DELETE statements.
A connection that has retained a complete, supported ordinary table/index schema
can distinguish invalid preparation from valid DML when a peer's ordinary
EXCLUSIVE lock prevents acquiring the first snapshot:

- Invalid table/column/target, VALUES cardinality or function arity returns the
  ordinary native preparation error and preserves all counters.
- Valid DML returns BUSY, resets `changes()` to zero and preserves
  `total_changes()` and `last_insert_rowid()`.
- Runtime expressions are not evaluated. For example, integer overflow in an
  otherwise valid UPDATE remains behind execution BUSY.
- Fresh/no-cache connections preserve the previous conservative BUSY outcome.
  Syntax and parameter binding still happen before snapshot admission.

This is bounded early-BUSY classification, not a reusable executable plan or
complete SQLite preparation equivalence. All successful snapshot paths retain
native catalog/ownership validation and their existing error order. The public
Options and Result layouts and statement-specific Result contract are unchanged.
Private DB/Tx layout gains metadata and schema-change tracking.

## Authority, immutability and limits

The connection-owned cache contains copied table and column names, an INTEGER
PRIMARY KEY ordinal and a schema cookie. It retains no root page numbers,
default-expression ASTs, declared types, index/extension definitions, row data,
reader, pager transaction or storage handle. Published entries are never mutated.
The shared INSERT/UPDATE/DELETE preparation routines are used by ordinary
execution and cache classification; they perform the same name/target/arity
checks, including TRUE/FALSE column shadowing. The cache cannot execute a plan.

Normal snapshot/lock acquisition is always attempted first. Cached metadata is
consulted only if admission fails with an error tree containing ordinary BUSY
sentinels and nothing else. A joined cleanup, cancellation, changed-file, poison
or other native error is not converted into preparation success/error. Read-only
and active-Rows gates still precede this path. Context cancellation during cached
preparation preserves counters. Parameter values are neither retained nor
inspected by this metadata-only step.

Retained metadata uses a conservative per-table/per-column/string charge bounded
by min(MaxSortBytes, 16 MiB), including its 256-byte fixed charge. This reuses the
existing option without changing its layout; it adds no unbounded global cache.
This is a separate retained-metadata allowance, not a joint cap with an active
query's existing sort budget.
A metadata-reading query uses an additional conservative declaration-parsing
charge (128 times SQL bytes plus entry overhead) against that same bound. The
existing SQL/token/depth and schema byte/entry limits also apply. Cache construction
is optional: exceeding its allowance discards the cache and preserves the
ordinary query/write result, rather than introducing a new query error.

## Population and lifecycle

A supported SQL write's fully validated catalog, after normal ownership checking,
can populate the cache even if subsequent DML preparation fails. An ordinary
user-table query or the native Schema API can also populate it after optional
validation of the complete ordinary catalog. All copied strings have independent
storage. Closing the connection releases the cache.

Snapshot success compares the observed cookie and discards a mismatching cache.
Normal execution always reparses its current catalog rather than authorizing data
access from the old cache. A peer's schema change not yet observed under a snapshot
can therefore leave stale name-binding information while EXCLUSIVE blocks access,
as with SQLite. Once a snapshot is admitted, fresh schema validation is decisive.

Successful native CREATE TABLE updates the statement-local catalog before the
immutable cache replacement; ordinary CREATE INDEX advances its cookie without
changing table/column binding. SQL DDL rollback, automatic failed-commit rollback
of DDL, and ROLLBACK TO that discards DDL invalidate metadata. A full rollback
following schema changes also invalidates it. Data-only rollback/savepoint undo
keeps schema metadata and, as before, never undoes the SQL connection counters.

## Explicit remaining differences

- Session-backed WAL/encrypted connections, extension-owned schemas and catalogs
  with unsupported declarations do not use early cached preparation. Their
  previous first-snapshot admission behavior remains. Ordinary validated writer
  promotion and commit BUSY accounting are unchanged.
- Cache quota rejection is a deliberate no-cache state. It is not SQLite parity
  for that unavailable preparation state.
- Native constant SELECT and SQL sqlite_schema projection keep their existing
  behavior and do not themselves populate this optional cache. Official SQLite
  may initialize its schema even for SELECT 1 or counter-only SELECT. Native
  explicit deferred constant queries remain snapshot-free.
- This change does not emulate every stale-schema preparation/reprepare rule
  after a lock becomes available. In particular, official SQLite can report an
  old column-count error for INSERT after a peer drops/recreates a table, until
  another suitable statement refreshes its schema. Native successful admission
  uses the current catalog. The retained oracle negative demonstrates this
  distinction; passing histories refresh with a fixed-label count query before
  proceeding. No full stale-preparation parity is claimed.
- Driver Prepare retains its existing reusable SQL-text behavior, parsing it
  for validity rather than retaining an AST. This is native execution-time
  metadata preparation, not a new public prepared API.
- Existing unsupported triggers/FK/conflict policies/views/virtual tables,
  INSERT SELECT, RETURNING and WITHOUT ROWID are not implemented by this slice.

## Verification and pinned reference

Reference is official SQLite 3.54.0, source ID
`2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3`.
The pinned local public source's `sqlite3ReadSchema`/`sqlite3Init` retain loaded
schema; `schemaIsValid` preserves an existing preparation error if its schema
check cannot start a read transaction. OP_Transaction checks cookies only after
transaction admission and suppresses counter publication on schema reprepare.
`sqlite3RollbackAll` resets connection schemas after schema-changing rollback.

New tests cover fresh/read-warm/cache-disabled states, typed counters and native
primary errors, ordinary cooperative locks, stale positive/negative names,
SQL-only peer ALTER/DROP/recreate, explicit transactions and DDL savepoints,
parameters, deterministic pre-cancellation, metadata immutability/quotas,
prepared driver reuse and unchanged Result values. The previous counter histories
retain partial source-row INSERT/ABORT, rollback, maintenance exclusion and
statement-specific results. Typed oracle histories run across 512/4096/65536-byte
pages; the existing rollback/shared-WAL histories remain selected. Pure error-tree
classification tests do not inject or execute native failures.

Only selected audited normal/race/oracle, affected vet and production build checks
qualify this slice. No broad suite, physical corruption/tamper/wrong-key,
namespace mutation, selected-file disappearance, native failure, crash/kill/hot
recovery, interrupted publication or rekey scenario is executed or qualified.
