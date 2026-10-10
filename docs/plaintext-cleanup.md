# Plaintext Reader managed retirement

Date: 2026-10-10. This first increment covers ordinary rollback-mode
`pager.Reader` acquisition, read-time metadata obligations and retirement. It is
not a generic plaintext transaction/connection cleanup guarantee. Standalone Tx,
rollback journal and WALExclusive ownership gaps remain real source gaps below.
Shared-WAL/encrypted protocols, formats and their separate APIs are unchanged.
Physical/native failure qualification and full M3/M8/M10 acceptance are NOT_RUN.

## Ownership and ordering

Before pager path resolution or its first native acquisition, Reader.Open reserves
one of 1024 strongly retained plaintext scopes. The limit counts opening, live
and pending Readers together, including temporary public Open probes and every
active rollback query snapshot. Idle rollback DB handles do not retain Readers.
A full bound returns pager ErrLimit (public ErrMemoryLimit/CodeNoMem) without that
attempted pager acquisition. The earlier public canonicalizer's Windows metadata
probes remain owned by their separate existing bounded registry.

A scope retains only its managed main File, nested managed/terminal owners, first
cause and one current immutable token. It has no Reader, DB, Rows, Tx, options,
page buffers or application cleanup closure. Opaque error implementations can
retain arbitrary caller data; this is a structural library-owned-record bound,
not a transitive byte/RSS bound. No raw descriptor is exposed or replayed.

Failed constructor errors are adopted before retiring anything. Nested failed
probes/constructors retire before the protecting main file. Explicit
RetirementComplete controls progress: a consumed diagnostic can advance, while
nil from an incomplete owner cannot release its protector. Indeterminate native
close remains the managed owner's terminal obligation and may consume capacity
for process lifetime. There is no expiry, eviction, finalizer, automatic sweep,
checkpoint, reopen or path-based cleanup in the retry mechanism.

An ownership error encountered by a live Reader's page or plaintext-source
validation adopts the failed probes and poisons further use before the operation
returns. The reader retains its main snapshot protection. An explicit retry of
that token handles only already-failed nested owners; it cannot close the main
file of a live Reader. Even after those probes retire, that Reader remains
poisoned until Close. Close marks the Reader closed and removes its usable file
pointer once, then begins whole-scope child-before-main retirement. Repeated
Close attempts only the managed scope. Header remains readable as historical
metadata; non-canceled page reads and source validations after logical closure
report ErrClosed.
Per-Reader native operations are serialized; independent Readers remain separate.

The registry mutex protects admission/removal/snapshot enumeration only. It never
calls native operations or RetirementComplete, nor waits for a scope/resource
mutex. Same-scope attempts serialize on the scope mutex. Snapshots read atomic
immutable tokens and cannot be blocked by another scope's native status/close.

## APIs and error bounds

- `sqlite.ErrPlaintextCleanupPending` is an immutable historical token marker.
  Actual tokens also match ErrPoisoned and carry CodeIOErr, including an underlying
  BUSY cause. The bare sentinel does not itself imply poison.
- `sqlite.RetryPlaintextCleanup(err)` discovers/deduplicates all distinct Reader
  scopes in wrapped/joined errors before any cleanup. Discovery accepts at most
  4096 error nodes. Excessive/cyclic graphs return ErrMemoryLimit/CodeNoMem without
  effects and without wrapping the unsafe input. Retain tokens and retry smaller
  trees. Errors without tokens and completed groups are no-ops, not global proof
  of clean ownership or assurance that a later open will succeed.
- `sqlite.PendingPlaintextCleanups()` allocates a new slice of current immutable
  tokens, including no-DB constructor failures and errors discarded by
  database/sql or callers. Ordinary live/opening Readers are excluded. Enumerating
  does not retire anything; modifying the slice does not affect ownership.

Internal VFS error-owner discovery is separately single-pass and limited to 4096
nodes. A stateful custom Unwrap cannot change between a validation traversal and
an unbounded second traversal. On refusal the original graph remains opaque in
an explicit terminal obligation; its unsafe Unwrap/Error is not exposed through
the returned diagnostic. Dependency cycles retain all concrete owners behind a
terminal barrier. Existing shared-WAL CleanupOwners is not changed by this helper.
The fixed node bound also bounds discovery metadata/dependency edges; arbitrary
Unwrap implementations themselves can allocate without library control.

Each scope preserves its first operation/first retirement cause and the current
attempt, without appending previous retry results. Pending snapshots replace one
current token; old caller-retained tokens are immutable at caller cost. Completed
scopes drop owners/history and leave the registry. A fully consumed close may
still return a diagnostic without the pending marker; old tokens retain their
historical marker after successful retry.

## Public lifecycle propagation

Query planning failures join Reader.Close diagnostics. Schema's deferred reader
release and plaintext import's source reader/database release preserve errors.
Rows exhaustion/failure joins its release error into Rows.Err rather than losing
it. DB/Rows/Tx invalidation keeps its existing public struct layouts; no cleanup
registry retains their closures. Real database/sql can still discard a connection
Close result, but the independent scope and snapshot route survive that disposal.
Repeating DB.Close on an idle rollback handle is not a Reader retry mechanism;
use the returned token or the snapshot API for discarded Reader cleanup.

## Remaining source gaps

- Standalone rollback OpenTx, empty/successful Commit, Rollback and poison clear
  file references without a scope or public discovery route.
- Rollback journal cleanup can leave a child incomplete and still retire main.
- WALExclusive constructor failure, later WAL/candidate acquisition, replacement,
  checkpoint and Session.Close need the same ownership barriers, one-time logical
  close and no-repeat checkpoint semantics. They are not fixed by this increment.
- Public failed-create rollback and other Tx-only discarded errors still belong
  to that later transaction work; this Reader slice does not claim them.

These are tracked code gaps, not merely unexecuted tests. Existing
shared-open-cleanup.md, shared-close-cleanup.md and encrypted-cleanup.md retain
their independent scopes and historical evidence.

## Verification

The evidence directory `evidence/plain-reader-cleanup` contains the behavioral
selection audit, exact commands, normal/race results, official managed-probe
interop, and Darwin/Windows compile-only results. New exceptional-state tests are
pure memory owner/callback laws. Ordinary tests use intact databases for
create/write/read/schema/SQL-error/cancel/read-only rollback/Rows close/DB close/
reopen and real database/sql zero-idle disposal. The official SQLite 3.54.0 probe
confirms that closing a managed Reader does not lose another live reservation.

No physical corruption/tamper/wrong-key/missing selected file, namespace
replacement/retarget/race, crash/kill/process-exit/hot recovery, failed native
I/O/lock/unmap/close/Sync/short write/disk-full/permission, or interrupted
publication/rekey workflow was executed here or moved elsewhere. Darwin/Windows
native runtime qualification, actual teardown-failure behavior, release review
and milestone completion remain NOT_RUN.

Initial create fallback is provenance-sensitive. A rollback Reader can authorize
normal creation only after conclusive absence during its first path resolution,
before the main file is acquired, with no additional cleanup or operation cause.
Later missing-path/stability errors and consumed cleanup diagnostics fail closed,
as do pending ownership and overbound error graphs. Wrappers and single-cause
joins preserve the initial-absence marker; raw NotExist alone does not authorize
this fallback.

On Windows the initial-absence provenance comes from the metadata open returning
no handle, never a later handle inspection or retirement. The earlier plaintext
public canonicalizer uses the same provenance for both resolver and nofollow
probe fallbacks. Existing VFS ResolvePath/Lstat error shapes remain unchanged;
new creation-aware variants separately report conclusive initial absence.
