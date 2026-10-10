# Experimental encrypted snapshot storage

This is the first **snapshot-only M8 slice**, not full M8 acceptance. Explicitly
setting `sqlite.Options.Encryption` connects the authenticated-record primitives
to the SQL engine, code-configured `database/sql` connectors, immutable snapshot
publication, rekey, portable encrypted backup/restore, and explicit plaintext
import/export. A nil `Encryption` preserves the ordinary plaintext engine.

The implementation is experimental and currently supported only on Linux. It is
an independent container, not a SQLite/SEE ciphertext format. It has no encrypted
WAL implementation, stable encrypted-format promise, independent cryptographic
or security audit, or production-security/crash-durability qualification. Do not
use irreplaceable data. Read [security.md](security.md) and the
[primitive design](encryption-primitives.md) before using the API.

## Start with the disposable example

From the repository root with Go 1.27.2 on Linux:

```sh
CGO_ENABLED=0 GOTOOLCHAIN=local go run ./examples/encrypted
```

[The runnable example](../examples/encrypted/main.go) supplies independent
ephemeral `crypto/rand` keys through code-only providers, creates and queries an
encrypted database, verifies it, rekeys, reopens read-only with the new key, and
backs up/restores to a fresh family. It prints no keys and stores no credential
files. Its private temporary data is removed on normal completion; removal and
best-effort buffer clearing are not secure erase. This provider is intentionally
unsuitable for a database that must survive the process. Real applications must
provide trusted credential management and retain their keys appropriately.

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
| `Mode` | Only `sqlite.EncryptionSnapshot` is supported; it is the zero value. |
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
then pass it to `sql.OpenDB`. A writable encrypted pool must use
`SetMaxOpenConns(1)` because each connection holds an exclusive lease for its
entire lifetime. A plain `sql.Open("gosqlite", dsn)` has no key-provider
configuration and uses the plaintext engine. DSNs accept only a path or a
supported `file:` URI with `mode=ro|rw|rwc`; encryption or key URI parameters are
not supported.

Ordinary SQL, schema/index support, bounded sorting and transaction overlays use
the same supported engine subset. `PageSize` defaults to 4,096 for creation;
`MaxTransactionPages` defaults to 4,096 dirty pages and `MaxSortBytes` to 64 MiB.
Sorting does not spill to disk. Encryption does not expand SQL compatibility.
`WALExclusive`, `WALShared`, `WALPowersafeOverwrite`, and `KeepWALOnClose` are
incompatible with encrypted snapshots and are rejected before opening storage.
Existing native `-journal`, `-wal`, or `-shm` sidecars are refused.

### Maintenance and copy operations

Close all `Rows` and finish any local `Tx` before calling a connection's
verification, rekey, or copy method; otherwise it reports `sqlite.ErrBusy`.

- `db.VerifyEncrypted(ctx) (sqlite.EncryptionInfo, error)` authenticates the
  current root and complete referenced base. This is physical snapshot
  verification, not a global SQL/index semantic integrity audit.
- `db.Rekey(ctx, target) error` requires a writable encrypted connection. It
  validates the supported logical structure and rewrites every page under fresh
  target epoch/file keys, retaining the logical database ID and advancing the
  root sequence. The connection adopts a copy of the target options after
  success; reopen using the target provider. Prior ciphertext is retained.
- `db.BackupEncrypted(ctx, destination, target) (sqlite.EncryptionInfo, error)`
  creates one portable, self-contained encrypted archive with a fresh database
  ID, epoch and role-separated backup keys, sequence 1, and no copied history.
  A read-only encrypted source is sufficient. Copying the main root alone is
  **not a backup**. `target.MaxRetainedBytes` also bounds archive byte extent.
- `sqlite.RestoreEncrypted(ctx, archive, destination, source, target)` returns
  `(sqlite.EncryptionInfo, error)`. It authenticates the complete archive and
  validates supported logical structure before creating an absent destination
  family with fresh database/epoch/file IDs and sequence 1. Source supplies the
  archive credential/policy; target configures the new family. The archive is
  unchanged. Do not pass the archive to ordinary `sqlite.Open`.
- `db.ExportPlaintext(ctx, destination) (sqlite.FileInfo, error)` is an explicit
  **confidentiality downgrade**. It writes all logical pages, schema and index
  contents to a named ordinary plaintext SQLite file. There is no additional
  plaintext staging file. Do not use an ambiguous backup name for this output.
- `sqlite.ImportEncrypted(ctx, source, destination, plainOptions, target)`
  returns `(sqlite.EncryptionInfo, error)`. It copies a supported rollback-mode
  plaintext source through a pinned read-only snapshot into an absent encrypted
  family. It forces read-only/no-create source access and requires a single-link
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

`EncryptionInfo` contains `PageSize`, `PageCount`, `DatabaseID` (`[16]byte`),
`CommitSequence`, and `RetainsHistory`. Live families report `RetainsHistory=true`
as a retention policy, even at their first generation; it is not a count of old
objects. A portable archive reports false. Backup/restore deliberately change
database identity, so an application using an external identity registry must
explicitly enroll the new identity.

## Storage layout, leases and publication

For a main path `example.gse`, a family consists of:

- `example.gse`: the sole current root authority, a 680-byte authenticated
  manifest object, replaced on each changed commit.
- `example.gse.gse-lock`: persistent empty regular lock inode. It is never
  removed by the library, including on connection close.
- `example.gse.gse-<64-lowercase-hex-file-ID>.base`: immutable complete encrypted
  logical page snapshots.
- `example.gse.gse-<file-ID>.retained`: previous authenticated roots.
- `example.gse.gse-<file-ID>.pending`: root candidates, which may remain after
  unsuccessful attempts. Unreferenced bases can also remain.

Use a dedicated, access-controlled directory. The library refuses symlinks,
nonregular objects and multiply linked family files, opens files with no-follow
and nonblocking flags, and exclusive-creates candidates with mode 0600. Linux
OFD locking on the separate lock inode provides a lifetime **EXCLUSIVE** lease
for one writable connection, or lifetime **SHARED** leases for read-only
connections. A writer cannot coexist with another writer or read-only
connection. Read-only connections may coexist with one another; they hold their
snapshot lease even while idle. One local reader/transaction lease is permitted
per connection. This is not shared-reader/concurrent-writer WAL behavior.

Every changed commit streams the whole logical database into a fresh base and
root with new file IDs, rather than rebuilding its B-trees. Publication is
O(database size) and retains prior objects. A no-op commit writes nothing;
ordinary rollback discards its in-memory overlay. The encrypted path writes no
plaintext journal, WAL, staging or spill file. Native WAL/checkpoint protocols
are not used, and there is no encrypted WAL slice yet.

The source publication sequence syncs and verifies the complete new base, syncs
its directory, writes/syncs/verifies a pending root, and durably retains the old
root before replacement. Initial publication uses no-clobber rename; later
roots replace the current path, followed by directory sync and verification.
A rename attempt or subsequent failure marks the session's commit state
uncertain and poisons it; there is no automatic old-root fallback, recovery
selection, or candidate deletion. Preflight validates geometry, credentials and
publication budgets before creating a new family's lock. An orphan lock is not
silently adopted to create a missing root. These are source-level safeguards,
not executed crash/fault or power-loss acceptance evidence.

Parent directories are resolved to canonical **strings**, not held as a
continuously pinned dirfd-relative namespace for every later operation.
Individual directory/file checks do not eliminate adversarial parent-directory
rename or substitution races. Keep the directory hierarchy under trusted
application control; this implementation does not claim protection against a
hostile filesystem namespace or compromised OS.

### Authenticated object layout

The experimental v1 bootstrap remains the primitive package's canonical
512-byte `GSQLENC1` header. It exposes public IDs, KDF parameters, geometry,
roles, commit/transaction/parent identity and nonce prefix. AES-256-GCM records
add a 24-byte header and full 16-byte tag; authenticated associated data binds
the bootstrap digest, record header, and expected logical location.

The root's 128-byte metadata payload has `GSEMNF01` at bytes 0–7, base file ID
at 8–39, big-endian base length at 40–47, complete base SHA-256 at 48–79, logical
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

The initial root has sequence 1; changed commits and rekey advance it. No-op
commits and rollback do not. Rekey does not reset generation history. Before
publication, a bounded batched directory inventory charges the current root,
bases, retained roots and abandoned pending/candidate files using overflow-safe
size accounting. It reserves the complete new base/root plus the worst-case
retained-root copy before writing. A new-family preflight also reserves the
upcoming stable-lock scan entry. Owned filename classes, regular-file/single-link
conditions and context are checked; unknown family names are refused.

`MaxDirectoryEntries` is a **scan-work guard, not an inode quota**. Unrelated
directory names count toward it. A commit can add entries, causing a later
publication to refuse when its next scan exceeds the bound. Dedicated directories
avoid unrelated scan pressure. There is no automatic pruning or garbage
collection. A portable backup followed by restore into a fresh family resets
history accounting without deleting or erasing the source family.

## Security and evidence boundaries

Rekey changes the current credential but does **not** revoke prior credentials'
ability to decrypt retained historical roots and bases. No historical revocation
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
artifacts. In-memory primitive/manifest negative tests are a separate evidence
class. The example covers only its intact, correct-key workflow.

Wrong-key-on-disk, tampered-on-disk, crash/process-termination, hot recovery,
short/wrong-offset writes, failed sync, permission-fault and interrupted
publication/rekey workflows have **not** been executed or accepted for this
checkpoint. They must not be run here or relocated to CI/another executor.
Memory-only malformed-input tests do not qualify these disk/recovery behaviors.
Independent security review and full release qualification remain outstanding.
