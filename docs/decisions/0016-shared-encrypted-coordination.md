# ADR 0016: Shared encrypted immutable selections and one writer

Date: 2026-10-10. Status: experimental ordinary Linux paths implemented;
security, hostile-namespace, native-fault and physical-durability acceptance pending.

## Explicit format boundary

`EncryptionWALShared = 2` selects authenticated root layout 3. Legacy values
`EncryptionSnapshot = 0` and `EncryptionWAL = 1`, all layout-1/layout-2 bytes,
base/segment content commitments and portable archive bytes remain unchanged.
The new root uses `GSEMNF03`, version 3, and coordination protocol byte 176 = 1;
bytes 177–255 remain zero. Child refs/content commitments retain layout-2
encoding. The old decoder still rejects layout 3. Opening never upgrades or
downgrades storage or derives encrypted behavior from native `WALShared`.

`UpgradeEncryptedWALShared` is a separate explicit transition from either
legacy encrypted mode. It requires local quiescence, retains the old exclusive
lifecycle lock throughout full-head validation and verified base/root
publication, then downgrades that lock. Database identity/history remain;
sequence advances. Future opens must explicitly choose the new mode.

## Stable inode protocol

Independent connections open their own OFD on the persistent `.gse-lock` inode.
Linux uses three nonblocking one-byte OFD ranges:

| Byte | Ownership |
| --- | --- |
| 0 | Lifetime SHARED for every shared-mode connection, including idle read-only handles; EXCLUSIVE for creation, legacy sessions, upgrade and globally quiescent rekey |
| 1 | EXCLUSIVE writer reservation through admission, transaction completion and validated release |
| 2 | SHARED only while capturing the current named root; EXCLUSIVE only during publication authority check, cutover and postpublication verification |

Legacy read-only handles retain their preexisting SHARED byte-0 behavior, but
mode authentication prevents them from joining a layout-3 family. No POSIX
process-associated lock fallback or process-wide connection mutex is used.

Readers do not take byte 1. A new reader may capture the prior committed root
while another connection owns a dirty, unpublished IMMEDIATE transaction. A
short byte-2 conflict is ordinary nonblocking BUSY; callers can retry later.
Candidates may already exist if publication-gate admission is BUSY. Retention
and minted-ID quotas continue to charge those abandoned encrypted candidates.

## Snapshot authority and ownership

Each connection retains its own master/nonce-ID ledger, namespace and lock
owner, plus a separately cleaned immutable selected graph. Capturing root bytes
requires checked named-open, exact bounded extent/read and final named identity
check under byte 2. The shared root descriptor closes before authentication/KDF
and selected-child opens. Authenticated retained root bytes are snapshot
selection evidence; an old read never claims that its root is still the named
current inode.

Selection checks preserve directory/lock identity and strict named identity,
extent and single-link checks for every selected immutable object. All selected
objects authenticate completely before admission; selected page records
reauthenticate on read. No prefix, historical-root or candidate fallback exists.
A writer additionally proves its current named root bytes authenticate and
match its admitted authority before replacement. A full root-byte SHA-256 token
binds the entire authenticated epoch, geometry, sequence and selected graph.

Refresh builds an isolated candidate with the existing epoch master; it does not
rerun the provider/KDF. The old graph stays owned until complete authentication
and resource admission succeeds. Candidate cleanup cannot close the owner's
master, namespace or stable lock. Provider calls stay outside engine mutexes.

## Transactions and maintenance

Deferred BEGIN, outer SAVEPOINT and constant SELECT remain lazy. First data
access captures one fresh selection. IMMEDIATE and shared-mode EXCLUSIVE take
byte 1 before refresh. Promotion of a pinned read reserves byte 1 and fully
authenticates current authority; a changed token returns both `ErrBusySnapshot`
and `ErrBusy`, releases only that reservation, and retains the original reader.
Restarting the transaction is required to observe the new head.

Ordinary writes, indexes, grams13, HNSW and IVF use the same pager dirty state and
aggregate statement/savepoint undo. Failed statements and ROLLBACK TO keep the
writer reservation; full rollback and no-op/changed commit release it. Each
connection still allows one local Rows/Tx lease. Independent pooled connections
may coexist; driver ResetSession releases the returned connection's transaction.

Verification/copy operations capture once, then retain that head through budget,
physical and catalog/freelist validation, every output page and returned geometry.
The internal operation reader does not reenter the session mutex. Checkpoint
reserves byte 1 and refreshes before checking for segments, so a stale idle head
cannot skip another connection's commits. It selects a new full base and retains
all objects needed by older readers.

Rekey obtains byte 1 and atomically converts byte 0 to EXCLUSIVE. Any other
connection, including an idle read-only one, causes BUSY. Failed conversion keeps
the original SHARED lifecycle lease. Successful rekey validates/copies the same
selected head, adopts the new epoch, downgrades byte 0 and releases byte 1.
Prior ciphertext is retained and remains decryptable with prior credentials.

Default writable Close attempts a nonblocking latest-head checkpoint. A BUSY or
quota error is returned, but all owned resources still close and committed data
remains selected. `KeepWALOnClose` and read-only close skip checkpoint.

## Resource/error boundaries

Before root/metadata allocation, reserve the retained graph, root buffers/AEAD
and codec scratch, reference/identity-map storage and the persistent ID ledger.
Before child opens/overlay allocation, reserve both complete old/new graph
memories and descriptor sets, including disjoint checkpoint generations. A
conservative two-graph handle bound is `oldSegments + newSegments + 10`.
Full scan/replay work is bounded; copy validation and output use one conservative
16-pass operation envelope. Target-epoch publication also charges overlapping
old/target ledgers. Refresh and checkpoint never reset the issuance ledger.

These are per-connection/operation accounting limits, not a process-global RSS
cap or global per-epoch issuance counter. No history GC or freshness oracle is
provided. Retained files/candidates remain charged to family quotas.

Rename attempt remains the uncertainty boundary. Postpublication gate/writer
release or old-descriptor close failure poisons the connection and reports
uncertain completion; poison takes precedence over BUSY. Ordinary stale/BUSY or
prepublication quota/cancellation does not mean a published commit rolled back.
Failure branches were reviewed as source, not fault-injected.

## Evidence and limits

Scoped tests cover memory-only format/authentication/resource negatives,
correct-key intact-file independent readers/writer, lazy/savepoint semantics,
stale promotion, provider reuse, checkpoint, operation-pinned copies, explicit
upgrade, restore/import, globally quiescent rekey, close BUSY and connector
ownership. Separate focused race runs and independent review supplement those
checks. The disposable `examples/encrypted-shared` exercises the ordinary path.

No disk tamper, wrong-key disk open, missing selected object, hostile namespace
race, failed native I/O, short write, disk-full/permission fault, forced process
exit, crash/hot recovery or interrupted publication/rekey was executed. Darwin encrypted coordination remains unsupported. Windows/amd64 now has a
[bounded native implementation](../native-encrypted-windows.md), verified only
by compilation and portable memory laws. Windows shared-mode rekey remains an
explicit implementation gap; no directory namespace persistence barrier is
provided. Native Windows runtime acceptance is NOT_RUN. Cryptographic/legal
review, hostile-namespace security, externally protected freshness, secure erase
and physical durability remain separate gates. Full M8 and production readiness
are not claimed.
