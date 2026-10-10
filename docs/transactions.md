# Transactions and savepoints

This guide describes the implemented S1 transaction controls. Production execution
is native Go. Ordinary tests cover rollback-journal, shared WAL and the existing
exclusive-session modes; this is not a claim of complete SQLite transaction or
physical durability parity.

## SQL controls

The public `DB.Exec` and `Tx.Exec` paths accept:

```sql
BEGIN;
BEGIN DEFERRED TRANSACTION;
BEGIN IMMEDIATE;
BEGIN EXCLUSIVE;
COMMIT;
END TRANSACTION;
ROLLBACK TRANSACTION;
SAVEPOINT name;
RELEASE name;
RELEASE SAVEPOINT name;
ROLLBACK TO name;
ROLLBACK TRANSACTION TO SAVEPOINT name;
```

`TRANSACTION` is optional on BEGIN, COMMIT/END and ROLLBACK. `SAVEPOINT` is optional
following RELEASE or ROLLBACK TO. Each API call accepts one statement. Nested
BEGIN is rejected; use savepoints for nested rollback boundaries. COMMIT or full
ROLLBACK without an active transaction reports an error.

Outside an explicit transaction, each supported DDL/DML `DB.Exec` runs and commits
atomically. Within a transaction, a failed statement restores its own entry
state while preserving earlier successful statements. Table changes, ordinary
indexes, grams13, HNSW and IVFFlat changes use the same pager transaction and
statement/savepoint undo. DDL and REINDEX are transactional.

## Native Go API and first access

The examples assume `CREATE TABLE items(id INTEGER PRIMARY KEY, value TEXT)`
has already succeeded.

```go
ctx := context.Background()
tx, err := db.Begin(ctx) // Deferred; no transaction snapshot yet.
if err != nil {
    return err
}
defer tx.Rollback()

if _, err := tx.Exec(ctx, "SAVEPOINT edit"); err != nil {
    return err
}
if _, err := tx.Exec(ctx, "UPDATE items SET value=? WHERE id=?", "new", 1); err != nil {
    return err
}
if _, err := tx.Exec(ctx, "RELEASE edit"); err != nil {
    return err
}
return tx.Commit(ctx)
```

`db.Begin(ctx, sqlite.TxOptions{ReadOnly: true})` is also deferred.
`TxOptions{Immediate: true}` reserves the writer at begin; `TxOptions{Exclusive:
true}` requests EXCLUSIVE behavior. Immediate and Exclusive are mutually
exclusive. Neither may be combined with a read-only transaction or connection.
`DB.Autocommit()` returns authoritative logical connection state, including SQL
controls and transactions created by the native API.

Deferred BEGIN and an outermost SAVEPOINT create logical transaction state
without opening a transaction reader or reserving a writer. Empty COMMIT,
ROLLBACK and savepoint controls do not force a snapshot. A constant query such as
`SELECT 1` does not force one either. Thus, after A begins deferred, B may commit;
A's first database read sees B's newly committed state. Connection Open still
performs its existing file validation; this laziness applies to transaction start.

The first table/catalog read or write pins the snapshot until the transaction
ends. This includes `Schema`, `CheckExtensionIndex`, native vector/similarity
searches and catalog-dependent `grams13_match` calls, even without a FROM clause.
A later ROLLBACK TO retains that snapshot. In shared WAL, another connection may
commit after the first read without changing the pinned view. Promoting a stale
shared-WAL snapshot to a writer returns `ErrBusySnapshot`, which also matches
`ErrBusy`; end that transaction and begin a new one to obtain a current snapshot.

A failed first access, for example BUSY behind another connection's EXCLUSIVE
lock, leaves the logical transaction available for retry. Callers must still
honor their contexts and close returned Rows.

## Savepoint stack behavior

Names use SQLite-style ASCII case-insensitive matching. Quoted identifiers and
string-quoted names are accepted, including empty names. Names are not required
to be unique. A control targeting a name finds its most recent occurrence.

- SAVEPOINT inside a transaction pushes a rollback boundary.
- SAVEPOINT outside a transaction starts a deferred transaction and pushes its
  outermost boundary.
- ROLLBACK TO restores the named boundary and removes all younger boundaries. It
  retains the named boundary, so new writes and repeated ROLLBACK TO are valid.
- RELEASE removes the named boundary and all younger boundaries. Inner RELEASE
  does not publish changes; an older savepoint or full ROLLBACK can still undo them.
- Releasing the outermost boundary commits only when that transaction was started
  by SAVEPOINT. Inside an explicit BEGIN, releasing every savepoint still leaves
  the BEGIN transaction active until COMMIT or ROLLBACK.
- COMMIT commits the transaction and removes every savepoint. Full ROLLBACK undoes
  the transaction and removes every savepoint.
- A missing name reports an error without changing the stack or publishing data.

For example:

```sql
SAVEPOINT batch;
INSERT INTO items VALUES(1, 'original');
SAVEPOINT edit;
UPDATE items SET value='temporary' WHERE id=1;
ROLLBACK TO edit;
RELEASE edit;
RELEASE batch;
```

The final RELEASE commits row 1 with `original`. With repeated names, a first
RELEASE removes the newest matching occurrence; a subsequent control can then
find the older occurrence.

## Aggregate undo budget

Dirty pages never spill before commit. `sqlite.Options.MaxTransactionPages`
bounds the current dirty-page overlay; zero selects 4096 pages.
`sqlite.Options.MaxTransactionUndoBytes` separately bounds the aggregate retained
undo for **all named savepoints and the active statement**, including map,
freelist, name and coordinator metadata. Negative values are rejected. This is a
conservative undo-accounting limit, not a whole-process heap or RSS cap; query
budgets and the live dirty overlay remain separate.

With `MaxTransactionUndoBytes == 0`, the limit in bytes is:

```text
D * (P + 64) + F * 32 + 65,536
```

Here D is the normalized dirty-page limit, P is the actual database page size,
and F is the pager's freelist-validation limit. The public connection currently
uses F = 1,048,576. At the default D = 4096 and P = 4096, the derived undo limit is
50,659,328 bytes, or 48.3125 MiB. The derivation admits one worst-case statement
snapshot under the existing page/freelist limits; it does not promise arbitrary
nested depth at maximum transaction size.

Each materialized mark is charged before allocating its retained maps:

```text
1,024 + 2 * name_bytes
      + dirty_pages_at_mark * (P + 64)
      + freelist_entries_at_mark * 32
```

A statement mark has an empty name. A lazy savepoint initially incurs only the
name/mark charge, then the full charge when its snapshot is materialized. First
access can therefore fail its quota while replaying lazy marks. The failed
replay releases its partial storage lease and preserves the logical stack;
releasing unneeded inner marks can make a later retry possible.

Page images are immutable. Marks share image references where possible, but
accounting intentionally charges each reference as a full page. This can reject
a deeply nested workload before its physically shared image bytes reach the
limit. It avoids a per-mark cap that would permit memory growth proportional to
depth. Maps are copied into length-sized allocations so old spare freelist-map
capacity is not retained by every mark. Release, rollback and statement
completion discard obsolete snapshots and trim retained stack backing arrays.

Quota failures match `sqlite.ErrMemoryLimit`. A statement quota failure preserves
prior statements and named marks. Releasing marks recovers their undo allowance.
Applications needing larger histories can set an explicit positive
`MaxTransactionUndoBytes` through `sqlite.Options`, including options passed to
`driver.NewConnector`; it is not a DSN parameter. No disk-backed undo/spill format
is implemented.

## Locks, BUSY and active Rows

IMMEDIATE obtains the existing writer reservation at begin. EXCLUSIVE obtains
rollback-mode EXCLUSIVE locks at begin and prevents other connections from
reading. In shared WAL, EXCLUSIVE uses the WAL writer lock, like IMMEDIATE, while
other readers remain possible. `WALExclusive` connections and the existing
encrypted sessions retain their stronger lifetime locks; deferred SQL controls
do not weaken those connection modes.

Ordinary contention returns `ErrBusy`; there is no hidden busy-wait/retry loop.
A native `Tx.Commit` returning BUSY retains its transaction and may be retried.
SQL COMMIT and outermost RELEASE likewise leave their transaction and savepoints
active when commit fails. After resolving contention, retry or explicitly roll
back; a failed RELEASE is not a successful commit.

The safe cursor restriction remains:

- Close all transaction-owned Rows before writes, SAVEPOINT, RELEASE,
  ROLLBACK TO or COMMIT. Otherwise those operations return BUSY.
- Full ROLLBACK closes and invalidates active transaction Rows; subsequent iteration reports
  `ErrTxDone`. Closing the connection invalidates its Rows with `ErrClosed`.
- Session-backed connections retain their existing single local snapshot lease;
  BEGIN is also rejected while an autocommit Rows lease is active there.

Snapshots are never dropped merely to let a control succeed. More permissive
SQLite read-cursor/commit behavior is deferred to S1b, which requires explicit
pinned-version and schema-invalidation design.

## database/sql behavior

`sql.DB.BeginTx` supports the default and serializable isolation selections and
creates a deferred native transaction, including read-only transactions. Use its
`sql.Tx` for all work belonging to that transaction.

For manual SQL BEGIN/SAVEPOINT controls, use one pinned `sql.Conn` throughout.
Separate `sql.DB.Exec` calls may use different pooled connections. Explicitly
COMMIT or ROLLBACK before returning a pinned connection. Returning it to the pool
is not a commit: on reuse, the driver's `ResetSession` consults `DB.Autocommit()`
and rolls back an abandoned transaction, including one opened by SAVEPOINT.
It does not infer state from the last parsed keyword.

`database/sql` considers `sql.Tx.Commit` terminal even when it returns an error.
The driver therefore rolls back its uncommitted native transaction after a failed
Commit before reuse. Do not retry the same `sql.Tx.Commit`; that retry behavior
belongs to the native API or explicitly managed SQL controls on a pinned
connection. Failed cleanup invalidates the driver connection.

## Scope and evidence

Connection SQL functions `changes()` and `last_insert_rowid()` are not implemented.
The Go statement `Result` exposes affected rows and last insert ID; it does not
supply those connection-level SQL functions. No general cursor relaxation,
savepoint spill, OR ROLLBACK conflict policy, trigger/FK participant system or
complete SQLite semantic parity is claimed here.

Current evidence:

- [Components, full parser tests, affected-package vet/build](evidence/s1-transactions-components.txt)
- [Focused transaction/driver race checks](evidence/s1-transactions-race.txt)
- [Pinned official SQLite transaction oracle and oracle race](evidence/s1-transactions-oracle.txt)
- [Implementation and test source hashes](evidence/s1-transactions-source-sha256.txt)
- Independent regressions in
  [`internal/pager/savepoint_review_test.go`](../internal/pager/savepoint_review_test.go)
  and [`sqlite/transaction_control_review_test.go`](../sqlite/transaction_control_review_test.go)

The oracle records SQLite 3.54.0, source ID
`2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3`.
Histories cover 512/4096/65536-byte pages in rollback/shared WAL, repeated names,
DDL/index undo, outer releases, interleaved first-access timing and cooperating
native/official lock contention. Native tests also reconcile grams13/HNSW/IVF
source and index state and exercise normal encrypted snapshot/WAL reopen.

These are ordinary intact-file results. They do not establish crash, failed-I/O,
hot recovery, interrupted publication or namespace-race safety. Those acceptance
gates remain separate and unrun in this S1 work. See the
[test-scope correction and guard](test-scope-audit.md) before running suites.
