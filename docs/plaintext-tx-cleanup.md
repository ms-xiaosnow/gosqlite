# Standalone rollback transaction managed retirement

Date: 2026-10-10. This increment extends the independently reviewed
[Reader retirement](plaintext-cleanup.md) scope to standalone rollback `OpenTx`,
Commit, Rollback, poison and its journal. WALExclusive remains separate unfinished
source work. Shared-WAL and encrypted algorithms/format bytes are unchanged.
Native failure qualification and milestone/release acceptance remain NOT_RUN.

## Ownership

OpenTx reserves from the same 1024 opening/live/pending plaintext-scope bound
before pager path resolution or native acquisition. Each group has two fixed
primary slots: main and temporary journal. The root reservation charges these
slots before any acquisition, and the temporary slot is explicitly reserved
before openJournal. A nil-result child acquisition relinquishes its unused slot;
error-carried managed owners are adopted before protection can be downgraded.
The nested owner graph remains separately bounded by 4096-node discovery.

The scope does not retain Tx, DB, Rows, page/undo buffers or application callbacks.
Read/metadata and promotion failures with retained VFS ownership poison further
transaction operations while keeping its main lock. Live retry touches only
already-failed nested owners and a requested journal close, never an active main
or an unrequested live journal. RetirementComplete remains authoritative for
consumed diagnostics, incomplete nil results and terminal managed owners.

Empty Commit, final successful Commit, Rollback and poison clear logical
transaction resources and use the group. Pending scope ownership is independent
of discarded native/public/database/sql transaction objects. Repeated native
pager Rollback retries the group; public closed transactions are one-shot and
use RetryPlaintextCleanup or PendingPlaintextCleanups for pending ownership.

## Journal barriers

Commit preserves the existing successful DELETE-journal sequence. All operation
errors are adopted before journal Close. A nested dependency stops journal and
main retirement; an unresolved journal stops main retirement. A failed journal
cleanup never advances to unlink or main close. It marks the transaction logically
closed and leaves the scope pending without automatically repeating that child's
same cleanup attempt. Explicit retry handles only retirement, never unlink,
publication, synchronization, transaction replay or recovery. An unreleased
journal is classified as RecoveryRequired as well as poison.

If journal retirement completes with a diagnostic, the diagnostic remains an
error even though ownership was consumed. The source-level failure branch may
then execute its pre-existing safe unlink/directory-sync handling; any resulting
nested managed error is adopted before main retirement. No failure branch was
physically executed as evidence. A consumed diagnostic while closing main after
an acknowledged commit preserves the existing CommitUncertain classification;
an unresolved main also carries the cleanup token. Historical pending markers
are diagnostic-only after adoption, so a newly completed result cannot falsely
look pending. Repeated retry history is not retained.

The private KeepFile transition used by shared/exclusive WAL keeps its previous
branch because it has no standalone Tx cleanup scope. This increment does not
claim to repair those transition or Session ownership paths.

## Public and driver behavior

The inherited Reader V5 gate accepts only conclusive initial-acquisition absence
without additional diagnostic causes. Mixed, post-acquisition or pending errors
cannot enter the ordinary create fallback merely because they match NotExist.
Public failed-create rollback, transaction Rows release and a newly created
SAVEPOINT's rollback diagnostics are joined. A Commit whose pager transaction is
logically closed also marks public Tx done, while ordinary BUSY keeps a live
native/public transaction retryable. Read-only release failures likewise end the
logical transaction. A public rollback invalidates its Rows once and preserves
all release causes.

The database/sql adapter still rolls back a BUSY Commit, because database/sql
considers that transaction finished. It invalidates a poisoned connection even
when its already-ended Tx's Rollback is a no-op, joins disposal diagnostics and
places a CodeIOErr wrapper ahead of any earlier BUSY operation. ResetSession's
ErrBadConn disposal can still discard diagnostics; the independent scope snapshot
survives that path. Existing public Options/Plan/Stats/DB/Rows/Tx layouts are
unchanged. The new logically-closed accessor is internal/pager only.

## Evidence and limits

Evidence is in evidence/plain-tx-cleanup. The source guard precedes every selected
suite; the selection audit records helpers. Exceptional-state coverage is
exclusively descriptor-free fake owner/callback/error laws. Native evidence is
intact ordinary create/read/write/commit/rollback/BUSY/cancel/close/reopen,
uninterrupted shared/encrypted lifecycle regressions, and official SQLite
interoperability. Darwin/Windows are compile-only.

No physical corruption/tamper/wrong-key/missing selected file, namespace
replacement/retarget/race, crash/kill/process-exit/hot recovery, failed native
I/O/lock/unmap/close/Sync/short write/disk-full/permissions or interrupted
publication/rekey ran here or elsewhere. WALExclusive acquisition/candidate/
checkpoint/Close ownership remains a real deferred code gap, not a claim of
unrun tests alone. Complete production durability/security acceptance remains
outstanding.

The V2 correction preserves the first journal-retirement diagnostic across a
whole-close transition even when an operation error preceded it. Closed undo
state is cleared centrally, including direct Exclusive poison. Pending journal
constructor ownership is conservatively RecoveryRequired. Direct OpenTx creation
uses explicit initial no-handle absence; its previous os.IsNotExist check was
not demonstrated to accept Windows post-acquisition joined errors.
DB.Exec automatic rollback classifies the complete operation/cleanup join, so an
already-wrapped BUSY cannot mask pending cleanup's CodeIOErr. Driver ExecContext
invalidates poison connections and preserves any disposal error with that same
precedence; ordinary execution errors keep the connection available.
