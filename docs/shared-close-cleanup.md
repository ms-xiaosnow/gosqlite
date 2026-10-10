# Native shared-WAL successful-session close retirement

Date: 2026-10-10. Status: implemented source with focused descriptor-free memory
laws and ordinary Linux lifecycle checks. Native teardown-failure qualification
is **NOT_RUN**. This is a scoped M3 lifecycle correction, not M3 completion or a
production-safety claim. The separate [failed-open contract](shared-open-cleanup.md)
continues to apply to unsuccessful opens.

## Logical closure and managed ownership

`sqlite.DB.Close` invalidates its Rows and rolls back its transaction exactly once.
SQL operations thereafter return `ErrClosed`. Native shared-WAL close continues
to attempt resource retirement on subsequent calls even though the DB is already
logically closed. The first Rows/transaction release errors are passed to the
shared session so error-carried nested managed owners are not discarded.

A direct pager session with an active lease still returns ordinary BUSY and stays
open. Once lease-free closure starts, the implicit checkpoint is attempted at
most once. `KeepWALOnClose` and an existing error/poison skip that checkpoint.
An ordinary reader-limited checkpoint BUSY remains a successful close when the
session was not poisoned. A retry never checkpoints, replays a transaction,
reopens a file, resolves a pathname, or invalidates the SQL objects again.

The session transfers retirement responsibility to a small strongly retained
cleanup group. It keeps no session, DB, Options, Rows, transaction, SQL page
cache, registry configuration, or callback back to the session. It retains only
managed resource owners and error causes needed for cleanup. Retirement order is:

1. Error-carried nested obligations, in the VFS dependency order.
2. WAL index, whose managed file retires every mapped view/mapping object before
   DMS and the SHM descriptor.
3. WAL file. Main protection remains while WAL ownership is unresolved.
4. Main database file and its SHARED protection.

Every stage asks the managed owner for explicit `RetirementComplete` status.
`err != nil` and `err == nil` are neither completion proofs. A consumed Linux
`os.File.Close` may report an error and nevertheless retire its lease; cleanup
then continues in order and frees the reservation. An already-poisoned session
can likewise retire completely while returning its historical diagnostic. A
nil result from an owner still reporting retained state fails closed. The
WAL-index adapter now clears its references when its file explicitly retired,
even if that close returned a consumed-resource diagnostic.

The new retirement group never extracts or retries a numeric descriptor/handle;
it delegates only to managed owners. Windows indeterminate
close and mapping-object uncertainty remain terminal under their existing managed
owners. Darwin can transfer an idle descriptor to its process-inode registry;
lease retirement does not assert all process-wide deferred physical closes have
happened. Terminal uncertainty or an unresolved nested owner can retain protection
and admission indefinitely. There are no finalizers, background sweeps, eviction,
pathname recovery, or automatic retries of somebody else's group.

## Admission and bounded retention

The process has a fixed capacity of **1024** successful-session close reservations.
A reservation is taken before `sqlite.OpenContext` performs its native shared-WAL
path canonicalization, and before direct `pager.OpenSharedSession` metadata or
file acquisition. It lasts for opening plus the successful connection's entire
live and pending-close lifetime. A full registry returns `ErrMemoryLimit` with
`CodeNoMem` before that attempted open acquires native resources. A canceled or
rejected preflight releases an unused reservation. Admission never evicts owners,
cleans another group, or waits on a native/database lock.

1024 is a deliberately conservative fixed bound for this implementation: it
permits typical pools and the existing 65-live-session regression while placing a
finite ceiling on sessions that can become unreachable through `database/sql`.
It is not an OS descriptor limit or a recommended pool size. It is an explicit
availability change: applications needing more than 1024 simultaneous native
shared-WAL sessions are refused instead of obtaining an unretainable cleanup
obligation. There is no runtime knob or overflow slot. Existing platform-native
owner/metadata/mapping limits still apply independently and may reject sooner.

The separate **64** failed-open reservation limit is unchanged. An open reserves
both before pager acquisition; success releases the failed-open reservation and
keeps its close reservation. Failure transfers unresolved ownership to the
existing failed-open group and releases the unused successful-close reservation.
The 65-successful-session test therefore still passes; it does not imply live
sessions have become unlimited. A pure-memory law fills all 1024 admissions;
no test opens 1024 physical databases to establish this bound.

Structural memory bounds are explicit, rather than a claimed global byte ceiling:

- At most 1024 close groups, each with three fixed primary managed-owner slots,
  one initial dependency-ordered nested-owner list, a first-cause error graph,
  and at most one current pending token retained by the registry.
- Nested owners come from the first internal close/poison error graph. Their
  managed resources and error payload sizes retain their existing VFS/index
  bounds; this layer does not claim a separate constant-byte cap on arbitrary
  caller-retained errors, native error strings, or live database mappings.
- Retries never append prior retry results to group history. They return the
  fixed first cause plus this attempt's diagnostics. The next snapshot replaces
  the current token; callers can retain old immutable tokens at their own cost.
  Preserved first-error wrappers can still refer to already-retired Go owner
  bookkeeping; that does not make those resources live or eligible for another
  raw close. Those fixed historical references are dropped with the group cause
  on completion, unless the caller retains the original error.
- Error discovery visits at most 4096 nodes and allocates bounded traversal,
  group-deduplication and result slices. Custom `Unwrap` implementations themselves
  remain caller code; no claim is made to bound allocations inside them.
- A snapshot allocates at most 1024 error entries. Mutating that slice cannot
  change the registry. Registry locking covers only admission/removal/snapshot
  enumeration; it never covers cleanup, native work, or waiting on a group lock.
  Same-group retry attempts serialize on that group's mutex.

The reservation before public canonicalization does not take over failed metadata
probes made by the public canonicalizer. Windows' separately bounded managed
metadata registry still owns those; the failed-open group's ownership boundary
still begins at pager acquisition. This increment does not broaden the earlier
failed-open contract or manufacture a retry route for pre-pager metadata failure.

## Public API and diagnostics

`sqlite.ErrCloseCleanupPending` identifies an immutable historical pending token.
Such results also match `ErrPoisoned` and carry `CodeIOErr`, including when a nested
original cause was BUSY. Do not interpret them as ordinary retryable contention.
The sentinel alone does not imply poison without an actual pending error.

Use one of these deliberate retry routes:

- Repeat `db.Close()` for that native shared-WAL connection.
- Call `sqlite.RetryCloseCleanup(closeErr)` for every distinct group in a wrapped
  or joined error. Discovery and pointer deduplication finish before any owner
  operation; every discovered group is attempted independently.
- Call `sqlite.PendingCloseCleanups()` to obtain a new snapshot of pending tokens,
  including close errors discarded by `database/sql`, then retry chosen tokens.
  Live connections and in-progress openings are excluded. A snapshot is not a
  process-wide leak report and does not include failed-open groups or other modes.

An ordinary `%w` wrapper and `errors.Join` preserve retry routes. Exact 4096-node
chains/trees are accepted. Excessive trees and cyclic wrappers return
`ErrMemoryLimit`/`CodeNoMem` with **no cleanup effects**. The refusal intentionally
does not wrap the unsafe input: keep the original tokens and retry smaller trees.
Extra classification/application wrappers can make a joined full snapshot exceed
4096 nodes; retry tokens individually or in smaller batches rather than assuming
one join always fits. Recognized tokens are discovery leaves because their
resource obligations were adopted at closure.

A pending retry result retains the first close's causes and current errors, with
no accumulated intermediate history. A completed group disappears from the
current result and snapshot. An owner can fully retire with a diagnostic error:
an error without `ErrCloseCleanupPending` does not imply retained ownership. Nil
means no currently reported error from discovered groups; nil input or an error
with no close token is simply a no-op, not proof every subsystem is clean. Old
tokens keep their historical pending marker after completion; retrying them is
an idempotent no-op. Nothing here promises future opens succeed or repairs the
original operation that caused poison.

Example using an error returned by Close:

```go
err := db.Close()
if errors.Is(err, sqlite.ErrCloseCleanupPending) {
    retryErr := sqlite.RetryCloseCleanup(err)
    // Inspect retryErr. A terminal owner can remain pending indefinitely.
    _ = retryErr
}
```

For a `database/sql` pool that disposed of its connection and discarded its close
error, enumerate `sqlite.PendingCloseCleanups()` and explicitly retry each token.
Enumeration alone never starts cleanup and is safe during other groups' retries.

## Driver context and discarded Close errors

The redundant post-success `Connector.Connect` context check is removed. The
pinned Go 1.27.2 `database/sql/driver.Connector` contract says context governs
connection dialing; `database/sql.(*DB).conn` accepts a successful synchronous
`Connect` result without another context check. `Connector.Connect` still checks
cancellation before opening, and `sqlite.OpenContext` retains its existing
cancellation checkpoints. Successful acquisition transfers the returned connection
instead of immediately closing it and discarding its close error when cancellation
races with return. This is a connection-ownership change, not permission to
continue arbitrary canceled queries.

`ResetSession` and ordinary pool disposal can still discard a close error. The
retained group now survives independently of the driver connection. Descriptor-
free tests use the real standard-library pool for both zero-idle disposal and
`ErrBadConn` reset disposal, then recover their cleanup routes via a snapshot.
Ordinary native driver tests cover pre-cancel, zero-idle create/write/read/close,
normal reopen, and rollback of an abandoned SQL transaction before pool reuse.

## Scope and remaining code gaps

This guarantee applies to successful **native plaintext WALShared sessions**.
It is not a generic database cleanup framework. Source-reviewed gaps remain:

- Ordinary rollback-mode `pager.Reader.Close` and standalone `Tx.Rollback` clear
  file pointers after a close error. Managed VFS retention may protect a file,
  but they have no new close-group discovery/retry API.
- Native `WALExclusive` `Session.Close` closes WAL/main without this stage barrier
  and clears references even when ownership remains unresolved.
- `EncryptedSession.Close` clears its Store reference, and `encstore.Store.Close`
  clears selection/master/lock/namespace references while joining diagnostics.
  The encrypted backend has separate managed owners but not this group contract.
- Other constructors/lease paths that ignore close results remain outside scope.
  This is not proof every failed open, discarded lease, or temporary file in the
  engine has recoverable ownership. The pre-pager metadata boundary above remains.

Those are actual source gaps, not merely unexecuted tests. No broader guarantee
is inferred from unchanged ordinary encrypted or rollback-mode behavior.

## Evidence and qualification boundary

The initial focused test/race/vet/build commands are in
[evidence/shared-close-cleanup-components.txt](evidence/shared-close-cleanup-components.txt).
Final owned-overlay verification uses immutable base
`efe40d70b190eaa8a8138b49ebd295279c9a0813`; its exact isolated commands and
results are in [evidence/shared-close-cleanup-isolated.txt](evidence/shared-close-cleanup-isolated.txt).
The owned-path manifest and SHA-256 snapshot identify this delta separately from
prior failed-open, Darwin poison-retention and Windows mapping-retirement work.
The central milestone/compatibility matrix remains lead-owned.

Memory laws cover every nested/index/WAL/main barrier, consumed-error completion,
terminal one-attempt raw ownership, nil-error/noncomplete refusal, historical
poison, at-most-once checkpoint, active-lease BUSY, 1024 no-eviction admission,
public admission before canonicalization, unused/canceled/failed-open reservation
release, 4096-node exact/deep/wide boundaries, duplicate/all joined groups,
no-effects refusal, 5000 bounded-history retries, immutable snapshots, concurrent
same-group completion, and independence from a blocked group's mutex. Public DB
laws check once-only Rows/transaction invalidation and continued Close retirement.

Physical checks are limited to intact ordinary create/read/write/BUSY/cancel/
rollback/normal-close/reopen, cooperating concurrency and normal mapping/DMS
lifecycles. Actual native I/O/close/unmap/lock/Sync failures, short writes, physical
corruption/tamper/wrong keys, missing selected files, namespace substitution,
crash/kill/process-exit/hot recovery and interrupted publication are **NOT_RUN**
and are not moved into CI or another environment. Darwin/Windows/Linux-arm64
cross-builds are compile evidence only; those native runtime outcomes remain
**NOT_RUN**. Full M3 durability, encryption security qualification, soak and
independent release review remain outstanding.
