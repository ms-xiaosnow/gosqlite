# Experimental encrypted snapshot and WAL storage

The **M8c slice** adds explicit shared-reader/single-writer coordination to
immutable authenticated dirty-page WAL segments and existing snapshot storage.
Explicitly setting `sqlite.Options.Encryption`
connects the authenticated-record primitives to the SQL engine, code-configured
`database/sql` connectors, rekey, portable encrypted backup/restore, and explicit
plaintext import/export. A nil `Encryption` preserves the ordinary plaintext
engine. The zero-valued `EncryptionSnapshot` mode and its layout-1 bytes remain
supported; `EncryptionWAL` explicitly selects lifetime-exclusive layout 2.
`EncryptionWALShared` explicitly selects shared coordination with layout 3.

The implementation is experimental. Linux has ordinary-path runtime evidence;
Windows/amd64 now has a bounded native implementation with compile-only evidence,
and Darwin now has a shared plain/encrypted inode coordinator with compile-only
evidence and strict ASCII inventory limits. Windows shared-mode rekey now has
guarded admission/conversion source with a post-conversion same-source-key authority
check in this unpublished composition; native Windows execution remains NOT_RUN. See [native encrypted Windows support](native-encrypted-windows.md)
for the precise matrix and no-directory-barrier policy. It is an independent
container, not a SQLite/SEE ciphertext format. Full M8 remains
partial: simultaneous encrypted writer/readers now have an ordinary-path Linux
implementation, while fault/security/durability acceptance remains pending.
There is no stable encrypted-format promise,
independent cryptographic/security audit, or production-security/crash-durability
qualification. Do not use irreplaceable data. Read [security.md](security.md) and
the [primitive design](encryption-primitives.md) before using the API.

## Start with the disposable examples

From the repository root with Go 1.27.2 on Linux:

```sh
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./examples/encrypted
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./examples/encrypted-wal
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./examples/encrypted-shared
```

[The snapshot example](../examples/encrypted/main.go) supplies independent
ephemeral `crypto/rand` keys through code-only providers, creates and queries an
encrypted database, verifies it, rekeys, reopens read-only with the new key, and
backs up/restores to a fresh family. It prints no keys and stores no credential
files. Its private temporary data is removed on normal completion; removal and
best-effort buffer clearing are not secure erase. This provider is intentionally
unsuitable for a database that must survive the process. Real applications must
provide trusted credential management and retain their keys appropriately.

[The WAL example](../examples/encrypted-wal/main.go) creates a layout-2 family,
makes changed commits, authenticates the complete selected head, normally closes
with `KeepWALOnClose`, reopens with the same correct key, and explicitly
checkpoints. It verifies that checkpoint selects zero segments while preserving
the logical data, then closes and reopens read-only. It also keeps its random key
only in memory and removes its private temporary directory on normal completion.
Neither example performs a wrong-key, disk-corruption, interruption or fault test.

[The shared example](../examples/encrypted-shared/main.go) keeps old readers
pinned while an independent writer commits, admits a new reader during an
uncommitted IMMEDIATE transaction, demonstrates stale promotion, and checkpoints
with a reader still active. Its credentials also stay only in memory.

## API and configuration

Use `sqlite.OpenContext(ctx, path, sqlite.Options{Encryption: &cfg})` or
`sqlite.Open(path, options)`. The `EncryptionOptions` fields are:

| Field | Meaning and zero/default behavior |
| --- | --- |
| `KeyProvider` | Required `encryption.KeyProvider`; credentials are acquired from trusted application code. |
| `CredentialKind` | Required explicit `encryption.RawKey256` or `encryption.PasswordArgon2id`; zero is invalid. No kind is inferred from credential length. |
| `CreateKDF` | Argon2id creation/target costs. All-zero selects 64 MiB, 3 passes, 4 lanes in password mode; must be zero in raw-key mode. Existing password objects use stored parameters, bounded before derivation and authenticated with the object. |
| `KDFPolicy` | Optional stricter KDF maxima; zero fields use the format maxima below. It cannot weaken the production minima. |
| `DerivationBudget` | Shared Argon2 concurrency budget. Nil/zero uses the process-wide single-derivation budget; explicit budgets permit 1–8 and must be shared by the application. |
| `Limits` | Lower per-object record/page/logical-size bounds; zero fields use the hard format maxima below. |
| `Mode` | `sqlite.EncryptionSnapshot` (zero) writes complete layout-1 snapshots; `sqlite.EncryptionWAL` writes lifetime-exclusive layout-2 segments; `sqlite.EncryptionWALShared` writes layout-3 segments with simultaneous readers and one writer. Existing storage must match the selected mode. |
| `WALLimits` | `sqlite.EncryptionWALLimits` bounds the selected graph and storage-adapter work; zero fields select defaults below. Root/identity-ledger guards also apply to snapshot operations. |
| `MaxGenerations` | Maximum successful root sequence; zero selects 256. |
| `MaxRetainedBytes` | Maximum accounted physical family bytes; zero selects 1 GiB. |
| `MaxDirectoryEntries` | Maximum names examined per inventory scan, including unrelated entries; zero selects 16,384. |

Options are copied by `Open` and `driver.NewConnector`; provider objects and
derivation budgets remain shared trusted application objects. Shared providers
must be concurrency-safe. `Acquire` returns one stable borrowed `KeyMaterial`;
the library copies the bounded credential and calls `Release` exactly once for
nonnil material, including error cases. The provider owns cleanup of its own
buffers. Provider errors are redacted rather than formatted or wrapped. Never
put credentials in a DSN, SQL, argv, logs, repository files, or static examples.

Raw keys are exactly 32 bytes; passwords are 1–4,096 bytes. Password derivation
uses Argon2id v0x13 from the pinned `golang.org/x/crypto`. The default/minimum
memory is 65,536 KiB (64 MiB), and minimum passes is 3. Hard maxima are 262,144 KiB
(256 MiB), 6 passes, 8 lanes, and 1,572,864 KiB-passes. Lanes must be 1–8; memory
must be divisible by four times the lane count. Public costs are checked before
expensive derivation. Context cancellation can interrupt waiting for a budget
slot, and is checked before and after derivation, but the synchronous inner
Argon2 operation is **not cancellable**. The concurrency budget does not bound
later Go runtime/GC memory retention.

`KeyRequest` contains an operation, path, kind and public IDs. IDs received during
open are not authenticated yet; do not use them to choose arbitrary external
accounts, endpoints, or credentials. Bind that authority in application code.

For `database/sql`, construct a connector with
`driver.NewConnector(path, sqlite.Options{Encryption: &cfg})`, check its error,
then pass it to `sql.OpenDB`. Legacy writable encrypted pools must use
`SetMaxOpenConns(1)` because each connection holds an exclusive lifetime lease.
`EncryptionWALShared` permits independent pooled connections; each still permits
one local Rows/Tx lease and only one connection may own a writer transaction. A plain `sql.Open("gosqlite", dsn)` has no key-provider
configuration and uses the plaintext engine. DSNs accept only a path or a
supported `file:` URI with `mode=ro|rw|rwc`; encryption or key URI parameters are
not supported.

Ordinary SQL, schema/index support, bounded sorting and transaction overlays use
the same supported engine subset. `PageSize` defaults to 4,096 for creation;
`MaxTransactionPages` defaults to 4,096 dirty pages and `MaxSortBytes` to 64 MiB.
Sorting does not spill to disk. Encryption does not expand SQL compatibility.
`WALExclusive`, `WALShared`, and `WALPowersafeOverwrite` are incompatible with all
encrypted modes and are rejected before opening storage. `KeepWALOnClose` is
accepted with `EncryptionWAL` or `EncryptionWALShared` and rejected with
`EncryptionSnapshot`. Existing
native `-journal`, `-wal`, or `-shm` sidecars are refused. These encrypted modes
do not use SQLite's native WAL/SHM protocol.

### Explicit mode selection and upgrade

Creation with `Mode: sqlite.EncryptionWAL` writes a full initial base and a
layout-2 root selecting zero segments. Every later changed transaction writes
one immutable segment containing the final sorted dirty-page images, including
finalized page one. A page changed multiple times within the same transaction
appears once. A no-op commit writes nothing; ordinary rollback discards the
in-memory overlay. Changed WAL commits do not rewrite the complete base.

Opening an existing family in the other mode returns `sqlite.ErrUnsupported`;
open never silently upgrades or downgrades. To convert an existing snapshot,
open it with `EncryptionSnapshot`, finish all local Rows/Tx, then call
`db.UpgradeEncryptedWAL(ctx)`. This requires a writable connection, validates
the supported logical structure, writes a fresh complete base and layout-2 root,
preserves the database identity and old ciphertext, and advances the root
sequence. The open connection adopts WAL mode; future opens must explicitly use
`EncryptionWAL`. There is no in-place downgrade. Copying through a portable
archive can create a separate family in either destination mode.

### Shared encrypted transactions and lifecycle

`Mode: sqlite.EncryptionWALShared` creates a layout-3 family. On Linux, every
connection holds a shared lifecycle guard on byte 0 of the stable lock inode.
Byte 1 admits one writer; byte 2 is a short root-capture/publication gate. New
readers can start while a writer owns unpublished changes. Gate and writer
contention return nonblocking `ErrBusy`. Legacy formats keep their old locking.

Deferred BEGIN and SAVEPOINT do not capture a head until first data access.
IMMEDIATE and shared-mode EXCLUSIVE reserve the writer before selecting the
latest committed head. A pinned read promoted after another commit/checkpoint
returns `ErrBusySnapshot` (also `ErrBusy`) and keeps its original snapshot.
End and restart the transaction to see the current head. Statement errors and
ROLLBACK TO preserve writer ownership; full rollback/no-op commit release it.

A snapshot owns fully authenticated root bytes and immutable child objects. It
does not require the old root to remain the currently named inode. Strict
selected-child identity checks and per-page authentication remain in force.
Fresh transactions authenticate the complete current graph with the retained
epoch master; they do not reacquire credentials or repeat Argon2 derivation.
`Info()` remains cached most-recently-observed metadata, not a refresh operation.

To upgrade either legacy encrypted format, finish local Rows/Tx and call
`db.UpgradeEncryptedWALShared(ctx)` on its writable connection. The existing
exclusive lifecycle guard stays held through verified fresh-base/layout-3
publication, then becomes shared. Old ciphertext and database identity remain.
Reopen explicitly with `EncryptionWALShared`. `UpgradeEncryptedWAL` still only
means snapshot-to-layout-2 conversion.

Shared rekey requires every other connection to close, including idle/read-only
ones. Linux/Darwin use atomic lifecycle conversion. Windows uses a guarded gap
under writer/capture protection and returns ordinary BUSY only after restoring the
shared lease; failed restoration or gate release retains poison/protectors. After
exclusive conversion, shared rekey reauthenticates the named root with the retained
source master and compares exact bytes/identity before validation or publication.
It never reacquires source credentials or adopts another graph after the gap.
See the [Windows protocol and evidence boundary](windows-shared-rekey.md).
Checkpoint and full-head copies can proceed while other connections keep
old readers. Every copy pins one head through validation and output. There is no
GC, retained-key revocation or protection against replay of a whole valid family.
See [ADR 0016](decisions/0016-shared-encrypted-coordination.md).

### Maintenance and copy operations

Close all `Rows` and finish any local `Tx` before calling a connection's
verification, checkpoint, upgrade, rekey, or copy method; otherwise it reports
`sqlite.ErrBusy`.

- `db.VerifyEncrypted(ctx) (sqlite.EncryptionInfo, error)` authenticates the
  current root and complete referenced base plus every selected WAL segment.
  This is physical selected-head verification, not a global SQL/index semantic
  integrity audit or verification of all retained/unselected history.
- `db.Checkpoint(ctx) error` on a writable WAL connection with active segments
  authenticates the selected graph and materializes its full logical head into
  a new base and mode-preserving layout-2/layout-3 root selecting zero segments. It advances the root
  sequence/head identity without changing logical page-one counters, database
  identity, or mode. With no selected segments it writes nothing. It retains old
  roots, bases and segments; checkpoint is not garbage collection. A read-only
  connection cannot checkpoint a nonempty selection.
- `db.Rekey(ctx, target) error` requires a writable encrypted connection. It
  requires `target.Mode` to match the open mode, validates the supported logical
  structure, and rewrites every page of the complete selected head under fresh
  target epoch/file keys. It retains the logical database ID and advances the
  root sequence. WAL rekey selects a fresh base with zero segments, preserving
  WAL mode. The connection adopts a copy of the target options after success;
  reopen using the target provider and mode. Prior ciphertext is retained.
- `db.BackupEncrypted(ctx, destination, target) (sqlite.EncryptionInfo, error)`
  creates one portable, self-contained encrypted archive with a fresh database
  ID, epoch and role-separated backup keys, sequence 1, and no copied history.
  A read-only encrypted source in any mode is sufficient. The complete head,
  including uncheckpointed committed WAL pages, is copied. `target.Mode` must be
  `EncryptionSnapshot` because portable archives do not contain segment chains.
  Copying the main root alone is **not a backup**. `target.MaxRetainedBytes` also
  bounds archive byte extent.
- `sqlite.RestoreEncrypted(ctx, archive, destination, source, target)` returns
  `(sqlite.EncryptionInfo, error)`. It authenticates the complete archive and
  validates supported logical structure before creating an absent destination
  family with fresh database/epoch/file IDs and sequence 1. Source supplies the
  archive credential/policy and must use `EncryptionSnapshot`; target configures
  a new snapshot, exclusive-WAL or shared-WAL family. A WAL destination initially selects zero
  segments. The archive is unchanged. Do not pass it to ordinary `sqlite.Open`.
- `db.ExportPlaintext(ctx, destination) (sqlite.FileInfo, error)` is an explicit
  **confidentiality downgrade**. It writes all logical pages, schema and index
  contents of the complete selected head, including uncheckpointed committed
  WAL pages, to a named ordinary plaintext SQLite file. There is no additional
  plaintext staging file. Do not use an ambiguous backup name for this output.
- `sqlite.ImportEncrypted(ctx, source, destination, plainOptions, target)`
  returns `(sqlite.EncryptionInfo, error)`. It copies a supported rollback-mode
  plaintext source through a pinned read-only snapshot into an absent encrypted
  snapshot or WAL family. A WAL destination initially selects zero segments.
  Import forces read-only/no-create source access and requires a single-link
  source inode. Native WAL imports and encrypted source options are unsupported.
  Import does not erase, delete, or encrypt the original plaintext file or its
  other copies.

All copy destinations must be absent and their parent directories must exist.
No API here replaces a destination or silently falls back to plaintext. Partial
named outputs can remain on an error: encrypted for backup/restore/import,
**plaintext for `ExportPlaintext`**. Do not treat an unsuccessful output as a
complete backup. Rekey/copy validation covers supported catalog grammar,
B-tree/overflow ownership, and freelist membership, not a complete index-to-table
content proof.

`EncryptionInfo` contains `Mode`, `StorageLayout`, `WALSegments`, `WALFrames`,
`WALBytes`, `HeadCommitID` (`[32]byte`), `PageSize`, `PageCount`, `DatabaseID`
(`[16]byte`), `CommitSequence`, and `RetainsHistory`. Live snapshot families
report layout 1 and the root hash as `HeadCommitID`; live WAL families report
layout 2 or 3, selected-segment totals, and the selected content commit ID. WAL totals
exclude retained/unselected files. Live families report `RetainsHistory=true`
as a retention policy, even at their first generation; it is not a count of old
objects. A portable archive reports false and leaves the new live-layout/head
fields zero. Backup/restore deliberately change database identity, so an
application using an external identity registry must explicitly enroll the new
identity.

### Normal close behavior

By default, writable WAL `Close` attempts a checkpoint if committed segments are
selected. `KeepWALOnClose: true` leaves that selection intact for normal reopen.
Read-only close does not checkpoint. `DB.Close` closes Rows, rolls back an active
local transaction, then closes storage. Always check its returned error: a quota
or another shared-mode writer can prevent the optional checkpoint. Storage still closes its descriptors and
releases its lease on that error; it does not roll back the already committed
head. Normal close/reopen is not evidence for interrupted close or recovery.

## Storage layout, leases and publication

For a main path `example.gse`, a family consists of:

- `example.gse`: the sole current root authority, an authenticated manifest
  object, replaced on each changed commit. Layout 1 is 680 bytes; layout 2/3 is
  `1064 + 256 * selected segment count` bytes.
- `example.gse.gse-lock`: persistent empty regular lock inode. It is never
  removed by the library, including on connection close.
- `example.gse.gse-<64-lowercase-hex-file-ID>.base`: immutable complete encrypted
  logical page snapshots.
- `example.gse.gse-<file-ID>.wal`: immutable authenticated final-dirty-page
  transaction segments in WAL mode, including retained/unselected segments.
- `example.gse.gse-<file-ID>.retained`: previous authenticated roots.
- `example.gse.gse-<file-ID>.pending`: root candidates, which may remain after
  unsuccessful attempts. Unreferenced bases and segments can also remain.

Use a dedicated, access-controlled directory. The library refuses symlinks,
nonregular objects and multiply linked family files, opens files with no-follow
and nonblocking flags, and exclusive-creates candidates with mode 0600. Linux
For legacy modes, Linux OFD locking on the separate lock inode provides a lifetime **EXCLUSIVE** lease
for one writable connection, or lifetime **SHARED** leases for read-only
connections. A writer cannot coexist with another writer or read-only
connection. Read-only connections may coexist with one another; they hold their
snapshot lease even while idle. One local reader/transaction lease is permitted
per connection. This legacy behavior is unchanged. Shared mode uses the three-byte protocol
described above; immutable snapshots remain valid after named-root replacement.

Snapshot changed commits stream the whole logical database into a fresh base and
root with new file IDs, rather than rebuilding its B-trees. WAL changed commits
write only final dirty-page images into a new segment and replace the root with
an explicitly extended selection. They also rebuild the bounded in-memory page
location overlay; publication is not a constant-work append. Full open verifies
the complete selected base and segments before logical page interpretation.
Selected-page reads reauthenticate their records. Checkpoint, upgrade and rekey
remain full-head operations. All prior objects are retained. The encrypted path
writes no plaintext journal, WAL, staging or spill file; sorting/transaction
overlays stay in memory. Native WAL/checkpoint protocols are not used.

The source publication sequence syncs and verifies the complete new base or
segment, syncs its directory, writes/syncs/verifies a pending root, and syncs a
retained copy of the old root before replacement. Initial publication uses
no-clobber rename; later
roots replace the current path, followed by directory sync and verification.
A rename attempt or subsequent failure marks the session's commit state
uncertain and poisons it; there is no automatic old-root fallback, recovery
selection, or candidate deletion. Preflight validates geometry, credentials and
publication budgets before creating a new family's lock. An orphan lock is not
silently adopted to create a missing root. These are source-level safeguards,
not executed crash/fault or power-loss acceptance evidence. There is no implicit
checkpoint when a WAL quota is reached: an operation can return `ErrLimit` before
creating a candidate, and the application must arrange an explicit checkpoint
or a separately authorized fresh-family copy while its own budgets permit it.

Live families now hold a pinned parent-directory descriptor. Linux `openat`,
`fstatat`, `renameat`/`renameat2`, and directory-descriptor `Sync` anchor live
root/base/segment/retention operations to it; authority boundaries also recheck
the named directory's identity. This improves on the earlier string-only live
namespace. Initial path resolution and preflight remain path-based, as do
portable archive/plaintext-output helpers; fresh-family restore/import acquire
their own live namespace. These checks do not establish protection against a
hostile filesystem namespace or compromised OS. Keep the directory hierarchy
under trusted application control. Namespace-race/fault acceptance is unexecuted.

### Preserved layout-1 snapshot and archive objects

The experimental v1 bootstrap remains the primitive package's canonical
512-byte `GSQLENC1` header. It exposes public IDs, KDF parameters, geometry,
roles, commit/transaction/parent identity and nonce prefix. AES-256-GCM records
add a 24-byte header and full 16-byte tag; authenticated associated data binds
the bootstrap digest, record header, and expected logical location.

The layout-1 root's 128-byte metadata payload has `GSEMNF01` at bytes 0–7, base
file ID at 8–39, big-endian base length at 40–47, complete base SHA-256 at 48–79, logical
page-one SHA-256 at 80–111, and zero padding at 112–127. Expected child identity,
descriptor and hashes come from this authenticated root, not merely from the
child's untrusted header.

Base/backup metadata is 64 bytes: `GSEBASE1` or `GSEBAK01`, page-one SHA-256,
and zero padding. Ordered authenticated data records cover pages 1 through
`PageCount`; the reserved SQLite lock-byte page, when present, has a canonical
authenticated all-zero slot. The 48-byte footer contains `GSEEND01`, SHA-256 of
the exact encoded header/metadata/data prefix, and the big-endian complete object
length, and is separately AEAD-authenticated. Total base/archive extent is
`704 + PageCount * (PageSize + 40)` bytes. Exact extent, metadata, all pages,
footer, hashes and expected identity/descriptor are verified before a logical
page is supplied to the pager. A successful verification is not a freshness
oracle or complete SQL semantic audit.

### Layout-2 root and immutable WAL segments

Layout 2 preserves the primitive `GSQLENC1` header and the existing complete-base
format. Its authenticated root payload begins with `GSEMNF02` and explicitly
selects one base plus an ordered list of zero or more complete segments. It binds
each expected file role/ID, descriptor, exact extent, full ciphertext hash,
page-one hash, and computed commit ID. Consecutive segment sequences and parent
commit IDs must agree. The authenticated head and page-one hash identify the
last selected object; a previous-root hash is a receipt, not an external
freshness authority. Unreferenced files cannot extend the selected head.

Each 256-byte object reference has these canonical fields (all integers are
big-endian; unlisted bytes are zero):

| Byte range | Field |
| --- | --- |
| 0–1 | File role and object revision (base 1, WAL 2) |
| 4–11 | Page size and final page count, 32 bits each |
| 16–31 | Record count and commit sequence, 64 bits each |
| 32–63 / 64–95 | Transaction ID / parent content commit ID |
| 96–127 / 128–135 | File ID / exact complete object bytes |
| 136–167 / 168–199 | Complete ciphertext SHA-256 / logical page-one SHA-256 |
| 200–231 | Content commit ID |

The commit ID is
`SHA256("GSECID02" || DatabaseID[16] || EpochID[32] || reference[0:200])`.
The root payload has a 256-byte fixed header, one base reference, and `N` segment
references. It records layout 2, reference size/counts, selected frame/byte
totals, exact payload length, head/page-one/previous-root hashes, and
`SHA256("GSESET02" || ordered references)`. The full root is
`1064 + 256*N` bytes. The root is authenticated before its selection is parsed.

A `FileWAL` segment contains authenticated `GSEWAL02` metadata (128 fixed bytes
plus one 4-byte page number per frame), sorted unique final page images, and a
208-byte authenticated `GSEWEND2` footer. Metadata binds previous/final page
counts, frame count, commit sequence, transaction/parent identity and page-one
hash. Every segment includes page one; growth must supply every newly addressed
page, including the canonical all-zero SQLite lock-byte slot when crossed.
Record associated data binds the expected page number and frame ordinal. The
footer binds the encoded prefix hash, metadata hash, page-one hash, exact byte
extent, frame count, page counts and ancestry. With `F` frames its exact extent is
`928 + F * (PageSize + 44)` bytes.

All selected records, metadata, footer, extents and root-supplied expectations
must verify before a logical head is exposed. No partial segment, valid prefix,
unselected candidate or historical fallback is promoted. The in-memory location
overlay invalidates truncated ranges; a later growth cannot revive old base or
segment pages. See [ADR 0012](decisions/0012-authenticated-wal-segments.md) for the
decision and evidence boundaries. None of these layouts is a stable-format or
physical-recovery guarantee.

## Resource and retention bounds

| Limit | Zero/default | Hard maximum |
| --- | --- | --- |
| `MaxGenerations` | 256 | 1,048,576 (`2^20`) |
| `MaxRetainedBytes` | 1 GiB (`2^30` bytes) | 1 TiB (`2^40` bytes) |
| `MaxDirectoryEntries` | 16,384 | 1,048,576 (`2^20`) |
| `Limits.MaxPages` | Hard maximum | 1,048,576 |
| `Limits.MaxRecords` | Hard maximum | 1,048,578 (pages + metadata/footer) |
| `Limits.MaxLogicalBytes` | Hard maximum | 64 GiB (`2^36` bytes) |
| `Limits.MaxRecordBytes` | Hard maximum | 1 MiB plaintext per record |

Page size must be a power of two from 512 to 65,536 bytes. `Limits` can only
lower its hard bounds. Retention limits can be raised from defaults up to the
hard maxima. Other limits can make the practical database size much smaller
than the logical format maximum.

### Active WAL and storage-adapter limits

`EncryptionOptions.WALLimits` fields are independently bounded; zero selects the
default. Configured values can be lower or higher than defaults but cannot
exceed the hard maximum. The effective envelope is the intersection of every
active limit, primitive limit, transaction dirty/undo budget and retained-family
budget. Listed maxima cannot necessarily coexist.

| `EncryptionWALLimits` field | Zero/default | Hard maximum |
| --- | --- | --- |
| `MaxSegments` | 128 | 4,094 |
| `MaxFrames` | 1,048,576 | 8,388,608 |
| `MaxSegmentFrames` | 16,384 | 65,536 |
| `MaxSegmentBytes` | 64 MiB | 256 MiB |
| `MaxBytes` | 256 MiB | 1 GiB |
| `MaxRootBytes` | 64 KiB | 1,048,576 + 552 bytes |
| `MaxOverlayPages` | 1,048,576 | 1,048,576 |
| `MaxMemoryBytes` | 64 MiB | 512 MiB |
| `MaxOpenFiles` | 256 | 8,192 |
| `MaxScanBytes` | 1 GiB | 70 GiB |
| `MaxOperationBytes` | 8 GiB | 560 GiB |
| `MaxReplaySteps` | 8,388,608 | 67,108,864 |

`MaxFrames` and `MaxBytes` count all selected segments, including versions later
superseded by another selected segment. `MaxOverlayPages` bounds the largest
page extent in the selected graph, including earlier larger extents before a
shrink. `MaxScanBytes` charges base plus selected segment extents. Checkpoint
clears the active segment selection but does not reduce retained-family usage.

The memory budget conservatively accounts owned live storage state: both old/new
dense location overlays, directories/references, descriptor/key-reader
allowances, root/AEAD scratch, and minted-file-identity ledgers. It is not a hard
process RSS/Go-GC bound and does not replace KDF, query or transaction budgets.
Open-file limits reserve the selected objects and additional transient handles.
Replay limits include dense initialization/coverage, directory/frame work and
shrink invalidation. Bounds precede candidate files and major storage-adapter
allocations; lowering a limit can refuse an existing head at open.

WAL commit operation accounting includes three new-segment extents plus eight
old-root and eight new-root extents. Copy/rekey/checkpoint reserve a conservative
16-pass logical/source-work envelope, including conservative legacy per-page authority-root rereads;
new base publication has a separate four-pass budget. This can refuse large
operations even when the selected graph alone fits. Per-master file-identity
reservations charge attempted publications, including abandoned candidates;
checkpoint and upgrade do not reset that ledger. Failed operations do not grant
a fresh accounting allowance within the same operation.

### Retained family accounting

The initial root has sequence 1; changed commits, nonempty WAL checkpoints,
upgrade and rekey advance it. No-op commits and rollback do not. Rekey and
checkpoint do not reset generation history. Before publication, a bounded
batched directory inventory charges the current root,
bases/segments, retained roots and abandoned pending/candidate files using
overflow-safe size accounting. It reserves the complete new base or segment,
root and worst-case retained-root copy before writing. Known upcoming directory
entries are also reserved, including the new family's stable-lock entry.
Owned filename classes, regular-file/single-link conditions and context are
checked; unknown family names are refused.

`MaxDirectoryEntries` is a **scan-work guard, not an inode quota**. Unrelated
directory names count toward it, and concurrent additions can cause refusal.
Known publication additions are reserved, but this does not enforce an OS inode
quota or eliminate unrelated directory growth. Dedicated directories avoid
unrelated scan pressure. There is no automatic pruning or garbage
collection. A portable backup followed by restore into a fresh family resets
history accounting without deleting or erasing the source family.

## Security and evidence boundaries

Rekey changes the current credential but does **not** revoke prior credentials'
ability to decrypt retained historical roots, bases and segments. No historical revocation
or secure deletion is implemented. A fresh-family restore does not remove old
ciphertext or erase backups, filesystem snapshots or storage copies.

An authentic historical family can still authenticate. Under a reused
credential, another complete valid family can also be substituted unless an
application independently checks the expected identity. These APIs do not
accept an external expected root identity/freshness token. Protect expected
database identity and currentness outside the replaceable family, and compare
authenticated information before accepting its data. AEAD, internal parent
hashes and commit sequences alone cannot prevent complete-family substitution
or whole-history rollback when the external expectation is absent or rolled
back too. Public size/identity/KDF metadata, activity patterns, memory/GC copies,
weak passwords, trusted provider behavior, denial of service and host compromise
remain in the threat model.

Existing evidence covers ordinary correct-key raw/password create/read/write,
statement undo, quotas, rollback, close/reopen, rekey, encrypted backup/restore,
explicit export/import, driver configuration/DSN restrictions, locking/BUSY,
extension copies, and plaintext-marker absence in normally generated encrypted
artifacts. The WAL slice adds correct-key dirty-segment publication, base-byte
preservation, selected-head query/copy/rekey, explicit upgrade, ordinary retained
close/reopen, checkpoint/default close, quota preflight, read-only coexistence,
and lease/BUSY checks. Independent scoped source review, frozen memory vectors,
pure-memory format/replay/resource negatives, and race checks are engineering
evidence, not an independent cryptographic audit. The runnable examples cover
only their intact, correct-key workflows.

Wrong-key-on-disk, tampered-on-disk, crash/process-termination, hot recovery,
short/wrong-offset writes, failed sync, permission-fault and interrupted
publication/rekey workflows have **not** been executed or accepted for this
checkpoint. They must not be run here or relocated to CI/another executor.
Memory-only malformed-input tests do not qualify these disk/recovery behaviors.
The shared slice adds simultaneous intact-file readers/writer, operation-pinned
copies, stale promotion, quiescent rekey and combined-resource tests. Darwin/Windows encrypted coordination has source/compile evidence only; Darwin
Unicode inventory remains an implementation gap. Windows shared-mode rekey has
bounded source implementation, but native admission/conversion, mixed-old-client
behavior and failure/durability qualification remain open. Independent
security review and full release qualification remain outstanding.

## Layout-3 coordination and combined admission

The shared root has the same extent and child-reference/content-commit encoding
as layout 2. Its authenticated payload starts with `GSEMNF03`, has version 3 in
bytes 8–9, coordination protocol value 1 at byte 176, and zeros in bytes 177–255.
All other canonical graph fields retain their layout-2 meaning. The layout-2
decoder continues to reject these bytes. There is no automatic format inference.

Shared refresh reserves the complete retained and candidate graph together,
including overlays, references/directories, object/key readers, root evidence,
codec/AEAD scratch and minted-ID ledgers. Descriptor admission conservatively
requires `oldSegments + newSegments + 10` handles. Full authentication/scan and
replay are bounded; validation and copying share a conservative 16-pass work
allowance. A small individual graph cap does not guarantee that two graphs fit.
A failed refresh or promotion keeps an exposed old snapshot intact. Admission
failure may require ending a transaction, checkpointing through another eligible
connection, or reopening with explicitly suitable limits; nothing is silently
removed or downgraded.

A successful publication followed by a gate/writer release failure is poisoned
and conservatively uncertain. Such errors take precedence over BUSY. Default
shared Close reports a BUSY checkpoint but still releases all ownership; it never
rolls back an earlier committed selection. Candidate creation can precede a
short publication-gate BUSY, and those encrypted candidates remain retained and
charged. This source contract is not native-fault or crash acceptance evidence.
