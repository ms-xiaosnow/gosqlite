# ADR 0012: Immutable authenticated dirty-page WAL segments

Date: 2026-10-10. Status: implemented experimental ordinary paths; full M8 partial.

## Context and compatibility

ADR 0011 introduced complete encrypted snapshots. Rewriting all logical pages
on each changed transaction is avoidable, but simply encrypting native SQLite
WAL frames would not supply an authenticated, complete committed selection.

Keep `EncryptionSnapshot` as the zero value and preserve its layout-1 root,
manifest, base/archive and primitive bytes. Add explicit `EncryptionWAL = 1`,
layout-2 roots, and immutable `FileWAL` segments. This is an independent
experimental container; it is not SQLite/SEE ciphertext compatibility, native
SQLite WAL/SHM compatibility, or a stable encrypted format.

## Decision

Each changed WAL transaction publishes exactly one segment containing the final
sorted dirty-page set, including finalized page one. Repeated writes to a page
within that transaction produce one final image. Growth covers every new page,
including a canonical all-zero reserved lock-byte slot when crossed. No-op commit
and ordinary rollback publish nothing. Changed WAL commits do not write a new
complete base.

The sole authoritative root authenticates one complete base and the exact
ordered segment list. Each reference binds expected identity/role, geometry,
transaction/parent identity, commit sequence, byte extent, full ciphertext hash,
page-one hash and derived content commit ID. Segment metadata/data/footer must
all authenticate before use. Open verifies the whole selected graph before
logical pager parsing; later reads reauthenticate selected page records. A
bounded dense location overlay invalidates shrink ranges and requires complete
growth coverage, preventing stale-page resurrection within the selected graph.

Use the existing v1 primitive bootstrap and complete-base format. The layout-2
root has `GSEMNF02` metadata and 256-byte references; its extent is
`1064 + 256*N` bytes for `N` selected segments. `FileWAL` uses `GSEWAL02` metadata
and a `GSEWEND2` authenticated footer, with extent
`928 + F*(PageSize + 44)` for `F` final dirty frames. Exact fields and current
bounds are documented in [encryption-storage.md](../encryption-storage.md).

## Publication, maintenance and copies

Write, sync and fully verify the new immutable object; sync the directory;
write/sync/verify a pending root; preserve the old root and sync the directory;
then replace the sole current root and sync/recheck. Initial publication uses
no-clobber rename. From any rename attempt onward, a failure is uncertain and
poisons the connection. There is no automatic valid-prefix salvage, candidate
promotion, old-root fallback, cleanup or recovery selection. These are reviewed
source rules, not physical crash/fault acceptance evidence.

`UpgradeEncryptedWAL` explicitly materializes a quiescent writable snapshot into
a fresh base/layout-2 root, preserving database identity and history. Open
rejects a mode mismatch; no implicit upgrade/downgrade occurs. Checkpoint writes
the complete current head into a fresh base and selects zero segments without
changing logical page-one counters. It preserves WAL mode and advances the root
sequence. It does not delete history.

Default writable close attempts a nonempty WAL checkpoint; `KeepWALOnClose`
preserves the committed segment selection for normal reopen. Checkpoint quota
failure is returned by close, which still releases descriptors and the lease.
The already committed head remains selected. Read-only close does not checkpoint.

Rekey must preserve mode and materializes the whole head under fresh epoch/file
keys while retaining database identity. Old keys can still decrypt retained
history. Backup/export consume the complete head, including uncheckpointed
commits. Portable backup target and restore archive-source options require
snapshot mode; restore/import destinations may use either mode and start with a
fresh identity, sequence 1, and no selected WAL segments. Copying the main root
alone is not a backup. Plaintext export remains an explicit confidentiality
downgrade, including possible partial plaintext output on error.

## Resource and namespace boundaries

Separate active graph, segment, frame, root, overlay, memory, descriptor, scan,
operation and replay limits compose with existing primitive, transaction and
retained-family limits. Quotas are checked before candidate files and major
storage allocations. Conservative accounting includes both overlay generations,
root revalidation during logical reads, repeated copy passes and per-master
minted-identity attempt ledgers. Limits bound accounted work, not process RSS or
all Go GC/KDF/provider/query behavior. Larger individual caps do not imply that
all hard maxima can be used together.

Inventory charges retained bases/segments/roots, abandoned candidates and known
upcoming entries; unrelated directory names count toward scan work. There is no
automatic garbage collection or quota-triggered checkpoint. Checkpoint reduces
the selected replay graph, not retained history. A portable backup/restore into
a fresh family resets history accounting without erasing the source.

Live storage now pins the parent directory and uses Linux dirfd-relative
open/stat/rename/sync operations for family objects. Named-directory identity is
rechecked at authority boundaries. Initial path selection/preflight and
archive/plaintext-copy helpers remain path-based; restored/imported live
families acquire their own namespace. This does not establish protection from
hostile namespace/OS actors or externally substituted complete valid families.

## Concurrency is still a code gap

Keep Linux OFD lifetime leases on the separate stable lock inode: EXCLUSIVE for
one writable connection, SHARED for read-only connections. Multiple read-only
connections may coexist; a writer cannot coexist with any reader or writer.
Each connection permits one local Rows/Tx lease. This is genuine authenticated
dirty-segment storage with restricted coordination. The required simultaneous
encrypted writer/readers protocol is **unimplemented**, not an acceptance-only
gap. Native plaintext shared WAL does not supply this missing encrypted protocol.

## Evidence and remaining gates

Ordinary correct-key tests cover dirty-page publication with unchanged base
bytes, rollback/no-op, quota preflight, retained close/reopen, checkpoint/default
close, password mode, read-only coexistence, mode-preserving rekey, explicit
upgrade, and complete-head copies with ordinary/grams13/HNSW/IVFFlat state.
Frozen layout vectors and negative framing/graph/replay/resource tests operate
only on in-memory bytes/models. Scoped engineering review and race checks are
additional evidence; they do not certify cryptographic or physical durability.

No wrong-key-on-disk, disk tamper/corruption, crash/forced exit, hot recovery,
failed sync, short/wrong-offset write, permission fault, interrupted
publication/rekey, namespace-race injection or candidate-recovery workflow was
executed or accepted. These restrictions must not be bypassed by relocating work
to CI or another executor. Intact normal reopen is not crash recovery.

Whole-history replay or substitution of another valid family under a reused
credential still requires externally protected expected identity/freshness to
detect; these APIs accept no external root/freshness token. Retained ciphertext,
metadata/access-pattern leakage, memory/GC copies, trusted provider behavior,
host compromise, lack of secure erase, native non-Linux runtime qualification,
independent security/cryptographic/legal review and full release acceptance
remain explicit limitations. Full M8 and production readiness are not claimed.
