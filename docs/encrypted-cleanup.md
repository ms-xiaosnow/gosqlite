# Encrypted managed ownership and cleanup

Date: 2026-10-10. This is a source-level ownership correction with descriptor-free
state laws and intact, correct-key Linux lifecycle evidence. Actual native
teardown failures and native Darwin/Windows runtime qualification are **NOT_RUN**.
It does not complete M3/M8/M10 or establish production security/durability.

## Managed owner graph

An encrypted root-directory acquisition creates a cleanup scope. Relative opens,
including metadata, inventory and rename probes, inherit that scope. A Store's
scope owns its selected root/base/segments and all transient/replacement files,
the stable `.gse-lock` lifecycle owner, and the pinned namespace directory. The
scope contains managed resources, role tags, scope dependencies and error causes;
it has no Store, pager session, DB, options, provider, crypto reader or page-buffer
field and stores no application cleanup callback. Opaque supplied error values
can themselves retain arbitrary references, so this is not a transitive bound on
application data or proof of absolute secret erasure.

Each acquisition reserves a scope/lease slot before invoking its native open.
Darwin/Windows put the actual physical resource into their existing native
registry before platform validation; a failed validation returns the acquired
managed lease plus the error. The common boundary adopts that lease before any
cleanup or caller validation can discard it. Darwin's unknown-inode quarantine
is represented by an explicit terminal obligation in the already-reserved scope.
It does not invent a safe callable native close.

Retirement completion is explicit and independent of diagnostic errors:

- Linux uses a managed wrapper instead of an `os.File` alias. A single Close
  consumes its `os.File` even when it reports a diagnostic. The actual file is
  detached before that call; its numeric descriptor is never retried. Directory
  pins survive until child ownership retires.
- Darwin releases the lease's exact byte holds under the common process-inode
  gate. A safely idle regular lease may transfer into the strongly retained
  native registry. That does not mean physical close happened. Its actual
  directory pin remains owned until physical retirement. Unreleased holds,
  dependent directories and terminal native close uncertainty are incomplete.
  A later failure while draining a previously transferred descriptor atomically
  marks its surviving scope terminal, preserving lifecycle/directory protection
  and snapshot visibility. Reuse transfers native parent pins without reopening
  a closed logical lease.
- Windows requires explicit `handleLifetime.retired`, an empty raw-handle field,
  no remaining exact lock record and no parent/child pin. Failed unlock remains
  represented and can be retried by its managed owner. Indeterminate native
  close is terminal: no retry invokes the old numeric HANDLE.

A directory's ordinary single-file logical Close can return nil while live
children retain its actual pin. That normal deferred result is not poison. Whole
scope retirement, however, cannot treat nil/noncomplete as successful retirement.

## Logical close, staged retirement and secrets

The Store clears its crypto readers, master and owned selection buffers once.
Native ownership already belongs to the scope, so discarding old selection
metadata does not discard unresolved resources. Old-selection and temporary
close failures poison further work and remain in the same ownership graph.
Whole-scope cleanup runs these barriers:

1. Independently acquired nested scopes carried by errors
2. Selected, replacement, inventory, capture and other ordinary child owners
3. The stable lifecycle-lock owner (native byte cleanup remains gate, writer,
   then lifecycle)
4. The pinned root directory

A new nested obligation discovered during native retirement is adopted before
the next stage can proceed. Cyclic/inconsistent dependency graphs fail closed.
One unresolved child prevents releasing its lifecycle and directory protection.
A consumed Linux close diagnostic does not falsely block later stages.

EncryptedSession uses the existing DB `CloseWithError` bridge. DB invalidates
Rows and rolls back its transaction once; all later SQL operations remain closed.
A direct pager close with an active lease returns BUSY without starting closure
or clearing its options (including KeepWALOnClose).
After closure starts, the optional checkpoint is attempted at most once. An
existing poison, earlier Rows/Tx release error, read-only mode or KeepWALOnClose
skips it. Repeated Close retries only managed retirement. It does not checkpoint,
replay a transaction, resolve a path, or invoke a key provider again.

## Constructor and temporary-owner audit

An operation error is adopted before its first cleanup call, not merely joined
afterwards. Direct child Close also stops behind unresolved independent scope
dependencies because it can release an already-closed parent pin. Metadata probe
errors remain errors; an absent-root check reports ErrChanged only for an actual
present path. ReaderAt causes are retained together with ErrFormat classification.

The following common paths now join cleanup diagnostics and retain ownership
when no Store, Archive or DB can be returned:

- `acquire`, `Open`, new namespace and checked-file validation
- Complete-object and segment open/write/verify cleanup
- Captured roots, authority checks and replacement selection cleanup
- Base/WAL/root publication candidates and superseded roots/objects
- Inventory enumeration and member probes; namespace writes/retained copies
- Portable archive open/write/close, fresh-family creation and plaintext export
- Pager encrypted session/archive header-validation errors
- Public restore's archive-close defer

Native metadata and rename constructors are inside the same pre-acquisition
boundary. A scoped failed constructor cannot lose its retry route merely because
it returns nil plus an error. Pure crypto Close calls have no native ownership;
their existing guaranteed-nil idempotent cleanup remains separate.

Explicit plaintext import's rollback-mode source Reader/DB still uses the old
plaintext ownership contract. Joining that source's cleanup errors would not
repair its independent missing ownership route, and this slice makes no such
claim. Other ordinary plaintext Reader/Tx and WALExclusive gaps described in
[shared-close-cleanup.md](shared-close-cleanup.md) remain outside scope.

## Bounds and concurrency

The process permits at most **1024 opening/live/pending encrypted scopes** and
**16,384 opening/live/pending common managed leases**. Admission precedes the
native call; a full bound returns `encryption.ErrLimit` (public
ErrMemoryLimit/CodeNoMem) without that attempted native acquisition. Every
relative child is charged, including metadata and constructor failures. Native
physical budgets remain additional constraints: Darwin's shared registry and
Windows' private handle ceiling are not replaced by the common logical budget.
A transferred Darwin lease can cease to be a common lease while its physical
record remains charged natively and its directory scope remains retained.
Unknown/terminal resources can consume capacity for process lifetime. No owner
is evicted and there is no finalizer, background cleanup, or pathname recovery.

The scope mutex serializes acquisition and cleanup. Native lease mutexes and the
Darwin process gate are below it. Native parent callbacks never acquire a scope
mutex; they publish atomic completion/error signals. Registry locking covers
admission, snapshots and dependency-graph updates only, never native work or
waiting on a scope. Admission can prune explicitly completed, atomically marked
parent bookkeeping with TryLock and per-entry atomic completion proofs. Native
callbacks publish those proofs only after checking completion under their existing
mutex/gate. Admission calls neither RetirementComplete nor Close, traverses no
callback errors, and does not wait for a blocked scope, lease I/O or status mutex.
Groups with unconsumed callback failures are left for explicit cleanup. Snapshot clearing uses compare-and-swap so it
cannot erase a concurrently published native callback token.

Per-scope causes retain the initial diagnostic and the current attempt, without
accumulating retry history. Native exact-lock ledgers retain their first poison
and at most one first release failure per each of the three byte positions,
plus the current returned error. Error-tree discovery visits at most 4096 nodes
before any retry effects. The fixed limits bound library-owned records, not
allocations made inside arbitrary error implementations or total process RSS.

## Public API

`sqlite.ErrEncryptedCleanupPending` marks an immutable historical cleanup token.
Actual pending errors also match ErrPoisoned and use CodeIOErr, even if an
underlying cause is BUSY. The bare sentinel does not itself imply poison.

- Repeat `db.Close()` to retry that encrypted connection's retirement.
- `sqlite.RetryEncryptedCleanup(err)` finds all distinct scopes in ordinary
  wrapped/joined tokens before doing any cleanup. Excessive or cyclic error
  trees return ErrMemoryLimit/CodeNoMem without cleanup effects and without
  wrapping the unsafe tree; keep original tokens and retry smaller trees.
- `sqlite.PendingEncryptedCleanups()` returns a new immutable-token snapshot,
  including failed constructors and close errors discarded by `database/sql`.
  Enumeration itself performs no cleanup; live unfailed scopes are excluded.

A token produced while a connection is still live retries only already-requested
retirement. It cannot close selected live objects or its lifecycle protection.
Whole-scope retirement becomes eligible only after Store/constructor shutdown.
Retired groups are no-ops. Pending retry results preserve the owning route and
original causes. Completed current results suppress adopted historical pending
markers while preserving their diagnostic causes; old tokens remain historical.
A diagnostic without the marker can describe consumed ownership. Nil or a token
from one subsystem is not a universal process leak report or proof a new open
will succeed. These APIs remain distinct from native plaintext shared-WAL's
RetryOpenCleanup/RetryCloseCleanup APIs.

## Verification and unchanged storage

Snapshot, WAL and shared-layout frozen memory vectors remain unchanged. The
patch does not change ciphertext framing, persisted coordination bytes, lock-byte
protocol, KDF policy or normal provider call counts. Ordinary tests cover all
three encrypted modes, repeated close/Rows/Tx invalidation, correct-key reopen,
optional checkpoint BUSY, password refresh counts, rekey and archive/restore/
import/export, and the real database/sql pool's intact operations.
Descriptor-free tests cover native status, stage barriers, nil/noncomplete and
consumed-error distinctions, live retries, exact capacity, noDB quarantine,
nested/cyclic dependencies, delayed native callbacks, immutable snapshots,
5000 retries, secrets before retirement, concurrent deduplication, blocked-scope
and blocked-status admission independence, original read causes, constructor
error adoption before child close, active-session option preservation, and discarded Close under zero-idle and ErrBadConn pool disposal.
The latter uses the real standard library over memory-only owners, not failed
native I/O.

The exact owned source manifest, isolated verification and independent review are
recorded alongside this document under evidence/encrypted-cleanup-*. The isolated
base is `69008bf658611318154a112178913c693b679a2b`; sqlite.go contains only this
slice's classification and Close GoDoc hunks, excluding concurrent SQL Query changes.
Those same two isolated hunks also apply to the later durable SQL base
`8f75ccf927b8dc0048d43c65736bdc3ad6fee20c`.

No actual failed native I/O/close/unmap/lock/Sync, file corruption/tamper/wrong
key, missing selected file, namespace replacement, disk-full/permission failure,
crash/kill/process exit/hot recovery or interrupted publication/rekey was run or
moved elsewhere. Darwin/Windows target tests are compiled only. Their native
runtime, physical-failure qualification, cryptographic/security/legal review and
full milestone acceptance remain **NOT_RUN**.
