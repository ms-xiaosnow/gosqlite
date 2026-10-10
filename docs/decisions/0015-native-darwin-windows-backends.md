# ADR 0015: Native Darwin and Windows file ownership backends

Date: 2026-10-10. Status: implementation and compile verification only; native
Darwin/Windows runtime acceptance is NOT_RUN. Supersedes the missing-backend
section of ADR 0014, not its Linux evidence or exceptional cleanup boundary.

## Scope and acceptance

The plain database pager and WAL-index now select real native managed-file
backends on Darwin and Windows/amd64. They retain the existing Go transaction,
WAL-format, WAL-index, atomic publication and snapshot algorithms. No SQLite
engine delegation, C binding, CGO production requirement, or new dependency is
introduced. Other operating systems and Windows architectures remain explicitly
unsupported by `internal/vfs/native_other.go`. Encrypted container storage is
separate and remains Linux-only.

Successful cross-compilation establishes source selection and ABI/type checking.
It does not establish that ordinary database operations work on a target OS.
Both new backends still need approved native ordinary lock/mapping/process and
official-SQLite interoperability runs. No native runtime, physical durability,
interrupted publication, fault, crash, kill, hot recovery, hostile namespace,
corruption, wrong-key, or interrupted rekey evidence is claimed. M3 and M10
acceptance remain open independently.

## Shared managed-file contract

Every plain database, WAL, SHM, journal, inspection probe and directory open
passes through `vfs.Open`, including early validation failures. Handles remain
private. Native metadata is attached before common identity/type validation.
Every new I/O/lock entry checks native poison. The portable owner is retained
on unresolved native retirement, as it was already retained on failed view
retirement. Mappings must retire before DMS and the file descriptor. SHM slots
accept unlocked-to-held acquisition and same-mode idempotency; conversion
requires explicit release. The Linux OFD syscall/state behavior is unchanged.

Uncertain native replacement results poison the database and both replaceable
WAL participants. Failure to verify the destination after successful replacement
also poisons all three. The exclusive-owner/role/path gates remain mandatory;
shared WAL never receives replacement permission. This namespace path was
reviewed as source, never executed by this assignment.

ADR 0014's failed-OpenSharedSession cleanup limitation remains: protecting
resources can be retained without returning a reachable session for a public
Close retry. No background collector, unsafe eviction, or falsely universal
retry promise is added. A separately designed bounded synchronous retirement
group is still future work.

## Darwin: process locks require whole-library descriptor ownership

Darwin uses real `F_SETLK`/`F_GETLK` POSIX byte locks via the pinned `x/sys/unix`.
A process-level gate serializes native open plus inode registration, aggregate
lock transitions and physical closes. `(device, inode)` keys come from `fstat`
on the actual opened descriptor. Per-connection ownership records distinguish
local readers and writers even though the kernel sees only the process.

All descriptors managed by this library enter the inode registry before
validation can fail. A logical close removes only that connection's holds.
Descriptors are deferred while any aggregate kernel lock remains, and drained
only with the lifecycle gate held after the last such lock is gone. Deferred
read-only/writable descriptors are reused by role and access mode, with bounded
per-inode (256) and process-wide (16,384) managed descriptor limits. Capacity
exhaustion returns an explicit unsupported/resource-limit error. Descriptors
whose identity cannot be obtained after raw open are quarantined without an
`os.File` finalizer; further physical opens stop, and their close is permitted
only after every managed process lock drains. These exceptional branches have
not been executed under injected OS errors.

Local lock conflicts are checked before fcntl calls. Successful syscalls alone
change the kernel ledger. RESERVED queries inspect local ownership before
querying external locks. Failed EXCLUSIVE acquisition retains SHARED and drops
only temporary PENDING. Failed cleanup poisons the inode, blocking all its
managed owners until their resources are explicitly retired.

DMS initialization runs only for the first local attachment. Later local
attachments join the existing shared process hold without querying their own
lock and truncating live storage. First attachment queries external ownership,
acquires exclusive DMS only when initially unlocked, initializes to three bytes,
and converts to shared DMS. Losing exclusive acquisition returns BUSY. SHM
ranges use per-byte process aggregation; partial acquisitions roll back only
new holds. `MAP_SHARED` read/write mappings use runtime page alignment and keep
exact logical slices independently of native view size.

Opening uses `O_NOFOLLOW`, `O_CLOEXEC`, and validation-time `O_NONBLOCK`;
nonregular files, nonlocal filesystems and writable multiple-hard-link files
are refused. Identity validation compares the current open descriptor, final
name and parent directory. These are stable-identity observations, not an
anchored-directory or hostile concurrent mutation guarantee.

The registry covers every descriptor managed by this plain-storage library.
It cannot intercept an application's or unrelated library's unmanaged open and
close of the same inode. Such a close can remove the process's POSIX locks.
Concurrent unmanaged access and independent SQLite libraries using the same
inode inside the same process are outside the supported Darwin contract.
Official SQLite interoperability must use a separate process.

Darwin's declared baseline Sync policy is `fsync` on regular files and
directories. It does not request `F_FULLFSYNC`, claim a full-device-cache flush,
or qualify strict ordering under power loss. A future stronger flush policy
must explicitly request/support that native operation and undergo separate
acceptance. Ordinary successful calls alone cannot establish durability.

## Windows/amd64: handle ownership and exact intervals

The native backend uses synchronous, noninheritable `CreateFileW` handles on
local fixed NTFS volumes. It rejects remote storage, other filesystems,
ambiguous/device/alternate-stream paths, final reparse points, nonregular
storage, pending deletion, and multiple writable hard links. Handle identity
uses the volume serial plus all 128 bits of `FILE_ID_INFO`; parent handle
identity and metadata-only nofollow pathname probes revalidate pairing.
Parent directory aliases are resolved through handles. Ordinary live files
omit delete sharing; only explicitly replaceable exclusive-session WAL files
add it. No general sidecar replacement permission follows from that exception.

All DB and SHM locking uses real `LockFileEx`/`UnlockFileEx` with
`LOCKFILE_FAIL_IMMEDIATELY` and exact successful acquisition records. SHM range
operations lock individual bytes, permitting subrange/union release without
splitting or merging native intervals. Same-mode repeats do not stack locks.
Partial acquisition rollback releases only new bytes. DB EXCLUSIVE upgrade
holds PENDING while dropping SHARED, trying EXCLUSIVE, and restoring SHARED on
contention. BUSY is retryable only after successful restoration and cleanup.
Unexpected asynchronous results or unresolved cleanup poison the owner.
RESERVED checks use a shared-byte probe, also valid on a read-only handle.

Per-inode lifecycle records prevent additional local SHM connections from
resetting live mapped storage; each connection still obtains its own OS DMS
hold. The first local attachment follows the Windows exclusive/reset-to-zero/
release/shared handshake. Arbitrary truncation failures do not trigger a
header-erasure fallback. Each mapped view retains its original aligned base,
length and mapping object handle. Allocation granularity is obtained using the
reviewed amd64 `SYSTEM_INFO` ABI; a logical 32 KiB block does not force file
extension to 64 KiB. Existing live views stay at stable addresses during growth.

The object handle may close before its mapped view, as documented by Windows;
the view keeps the mapping alive. This lets failed teardown keep the remaining
live view represented without exposing a successfully unmapped stale byte
slice. DMS/file retirement waits for all views and retained mapping objects.

Regular-file Sync uses `FlushFileBuffers`. Directory Sync deliberately supplies
no Unix-style namespace barrier, matching SQLite-win32's explicit unused
`syncDir` policy. A successful directory hook therefore means that this VFS
policy was applied, not that a namespace durability barrier happened.
Interrupted namespace durability is unqualified. Same-volume `MoveFileExW`
replacement uses REPLACE_EXISTING and WRITE_THROUGH, without COPY_ALLOWED or a
delete-then-rename fallback; WRITE_THROUGH is not proof of interruption safety.

## Capability matrix and evidence boundaries

| Platform | Native plain-file source | Runtime evidence in this change | Sync policy | Explicit limits |
|---|---|---|---|---|
| Linux amd64/arm64 | Existing OFD backend preserved | Scoped ordinary Linux amd64 checks; arm64 compile-only | Existing file/directory fsync | Existing local-file contract; M3/M10 unqualified |
| Darwin arm64 | POSIX inode registry, DMS, mmap | Compile-only; native ordinary/process/oracle checks NOT_RUN | fsync; no F_FULLFSYNC guarantee | Local filesystems; no unmanaged same-inode closes; bounded retained descriptors |
| Windows amd64 | Win32 handles, interval locks, mapped views | Compile-only; native ordinary/process/oracle checks NOT_RUN | FlushFileBuffers; no directory namespace barrier | Local fixed NTFS, reviewed drive paths; no remote/reparse/ADS storage |
| Other targets | Fail-closed stub | Unsupported | Unsupported | Windows non-amd64 intentionally not enabled |

Exact named commands and frozen source hashes are recorded in
`docs/evidence/native-platform-components.txt`. New target-specific ordinary
tests are compiled only. Existing Linux native tests remain Linux-tagged;
we do not substitute narrower Windows assertions for OFD conversion coverage.

## Authoritative API and protocol references

- Apple [fcntl(2)](https://developer.apple.com/library/archive/documentation/System/Conceptual/ManPages_iPhoneOS/man2/fcntl.2.html), [mmap(2)](https://developer.apple.com/library/archive/documentation/System/Conceptual/ManPages_iPhoneOS/man2/mmap.2.html), [fsync(2)](https://developer.apple.com/library/archive/documentation/System/Conceptual/ManPages_iPhoneOS/man2/fsync.2.html), and [statfs(2)](https://developer.apple.com/library/archive/documentation/System/Conceptual/ManPages_iPhoneOS/man2/statfs.2.html).
- Microsoft [LockFileEx](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-lockfileex), [UnlockFileEx](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-unlockfileex), [MapViewOfFile](https://learn.microsoft.com/en-us/windows/win32/api/memoryapi/nf-memoryapi-mapviewoffile), and [closing mapping objects](https://learn.microsoft.com/en-us/windows/win32/memory/closing-a-file-mapping-object).
- SQLite [process-lock hazards](https://www.sqlite.org/howtocorrupt.html), and the pinned public-domain `src/os_unix.c`, `src/os_win.c`, `src/os.h`, and `src/wal.c` in the recorded SQLite 3.54.0 reference toolchain. The production implementation is native Go; those sources supply protocol reference only.
- Pinned `golang.org/x/sys v0.49.0` Darwin/Windows declarations. No new package download was required.
