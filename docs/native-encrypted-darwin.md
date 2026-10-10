# Native encrypted storage on Darwin: bounded implementation

Date: 2026-10-10. Targets: Darwin/arm64 and Darwin/amd64, CGO disabled.
**Native runtime execution is NOT_RUN.** This source/compile checkpoint does
not qualify M3, M8, M10, security, interoperability or physical durability.

## Capability and remaining work

| Capability | Implementation | Evidence boundary |
| --- | --- | --- |
| Encrypted modes 0/1/2 | Managed descriptors, immutable selection and native byte locks | Native runtime NOT_RUN; target source compiled only |
| Shared-mode rekey and legacy-to-shared upgrade | Atomic POSIX lifecycle conversion with local-owner conflict checks | Memory state laws; no native acceptance |
| Plain/encrypted managed same-inode probes | One cross-package physical-close gate and inode registry | Portable synthetic-resource laws; target test compiled only |
| Relative namespace and publication | openat/fstatat, nofollow, renameatx_np(RENAME_EXCL), renameat replacement | Target source/test compilation only |
| Local filesystem and regular-file policy | fstat/fstatfs, MNT_LOCAL, single-link regular members | No native filesystem qualification |
| User target basename | Printable ASCII, maximum 177 bytes | Explicit temporary capability restriction |
| Directory contents for inventory | Every enumerated name must be ASCII, including unrelated siblings | Explicit additional temporary capability restriction |
| Unicode target basenames | **Required implementation gap** | Apple filesystem Unicode equivalence/accounting not implemented; Unicode parent paths are allowed |
| Sync | fsync for regular files and directories | No F_FULLFSYNC/device-cache or power-loss qualification |
| Windows shared-mode rekey | **Required implementation gap, unchanged** | Darwin implementation supplies no Windows atomic conversion |

Modes `Snapshot=0`, `WAL=1`, `WALShared=2`, authenticated layouts 1/2/3,
coordination protocol 1, KDF/record framing, content commitments and archives
are unchanged. Existing Linux OFD and Windows native syscall sources are
unchanged. Unsupported operating systems and Windows architectures remain
fail-closed. The existing Go transaction/replay/publication algorithms remain
in use; this is not another database engine or a lock-only fallback.

## Shared physical ownership, separate logical protocols

Darwin POSIX fcntl locks belong to a process. Closing any descriptor for an
inode removes that process's locks, even when the closing descriptor never
acquired a lock. A private encrypted-only registry would therefore be unsafe:
a plain VFS inspection probe could release an encrypted connection's locks.

`internal/darwinfile` owns the common lifecycle gate and physical descriptor
records for both backends. Native open, fstat identity discovery, registration,
all successful kernel-lock ledger changes, and physical close are serialized
under this gate. The identity is `(device, inode)` from the opened descriptor.
Plain VFS and encstore retain their separate per-owner interval arbitration;
SQLite's ranges/DMS and the encrypted three-byte protocol do not change.
Each domain publishes whether any of its inode's kernel locks remain, while
still holding the same gate used for the native lock call.

Every descriptor enters the shared registry before validation or failed-open
cleanup can retire it. A logical close first releases only its owner's logical
holds. Physical close waits until neither domain has a process lock on that
inode. Successful last-hold release drains already logically closed records.
The physical close function strongly retains the actual `os.File`; neither
GC nor a finalizer can close it underneath another owner. The encrypted API
exposes only `managedFile` leases, never `*os.File` aliases or raw handles.

Deferred regular descriptors can be reused by access mode. Encrypted reuse
also requires the same parent directory identity, transfers the directory pin
to the new logical lease's parent, and resets sequential position. Publicly
closed leases remain closed; reuse creates a new logical lease. Plain VFS
preserves its existing access/role reuse policy. The managed inode budget is
256 descriptors and the process budget is 16,384, shared across both domains.
Admission precedes native open. A raced identity may discover one overflow
record; it is retained safely and all further physical opens are refused until
that overage drains. No live descriptor is evicted to satisfy a limit.

An fstat failure after raw open cannot justify closing an unknown inode. The
raw integer, without an `os.File` finalizer, enters quarantine. All further
physical opens stop. Its one close attempt waits until every inode in both
domains has no process lock. A close error is terminal uncertainty: the numeric
descriptor is never retried, the tombstone remains charged, and further opens
remain refused. Known-inode close uncertainty poisons that inode and remains
strongly retained. Failed unlocks keep the unreleased successful ledger entries
and may be retried by internal ordered cleanup; ordinary I/O remains poisoned.

Retirement hooks are private, synchronous, gate-owned bookkeeping. They do not
acquire any per-file mutex, perform application callbacks or create new opens.
Native close is marked attempted before invocation. Parent-close errors and
unexpected hook panics cannot cause retry of a consumed descriptor number.
Ordinary data I/O holds its lease mutex, not the process-wide lifecycle gate.

This containment is not universal resource recovery. A common Store or failed
open may discard the caller's retry route while an exceptional resource remains
retained for process lifetime. There is no background collector. Arbitrary
unmanaged application descriptors or independent SQLite libraries inside the
same process cannot be intercepted; their same-inode closes remain outside the
supported Darwin contract. Official SQLite interoperability requires a separate
process and has not been run on Darwin here.

## Stable three-byte coordination

`.gse-lock` remains stable, empty, regular and single-link:

- Byte 0: shared lifecycle for mode 2; exclusive for creation, legacy writable
  sessions, explicit upgrade and globally quiescent rekey
- Byte 1: exclusive writer reservation
- Byte 2: short shared root-capture or exclusive publication gate

The inode ledger tracks each logical lease's exact byte holds separately from
the aggregate kernel mode. Same-mode repeats are idempotent. Local exclusive
conflicts return BUSY before a syscall. Multiple local readers share one kernel
read lock; closing one cannot release another's hold. Only a successful native
`F_SETLK` changes the aggregate and local ledgers.

Shared-to-exclusive and exclusive-to-shared conversion use native F_SETLK
without an unlock gap. A contended upgrade preserves the old shared hold.
Unexpected acquisition/release errors poison the inode rather than being
reported as safely retryable BUSY. Cleanup releases capture, writer, then
lifecycle; partially successful cleanup retains every remaining exact hold.
Windows still refuses shared-mode rekey because its separate LockFileEx backend
does not provide the same atomic conversion.

## Namespace, identity and lifetime

Initial absolute/canonical selection uses Darwin's descriptor-free Lstat and
EvalSymlinks. The selected parent then receives a managed O_DIRECTORY handle.
Members are opened with one validated basename, relative to that retained
handle, using `O_NOFOLLOW | O_CLOEXEC | O_NONBLOCK`; no truncating disposition is
accepted. Enumeration opens `.` through the retained parent and has its own
managed directory handle. It uses the exported x/sys `SYS_GETDIRENTRIES64`
call directly, not Go's or x/sys's ReadDir/Getdirentries emulation: those
wrappers create hidden directory descriptors and discard some close errors.
The native ABI is checked against Apple's syscall 344 and pinned Go's Darwin
Syscall6 stubs. A 64 KiB stream buffer and at most 64 returned names bound each
batch. A strict memory-tested parser consumes only the returned byte count,
never the trailing extended-flags word. It validates record/name lengths,
termination and separators, skips zero-inode and dot entries, and performs no
lazy metadata opens. Direct-kernel ABI and native-version support remain
compile-only limitations requiring approved native validation; there is no
weaker enumeration fallback.

Before data access, fstat checks actual type/identity/link count and fstatfs
requires local storage. Named identity uses `fstatat(AT_SYMLINK_NOFOLLOW)` through
the retained directory. Held descriptor identity, original size/time evidence,
single-link count and named parent identity are checked. These metadata probes
open no extra descriptors. `os.SameFile` compares Darwin stat snapshots without
opening a lazy Windows-style metadata handle.

A directory's logical close is permanent, but its actual descriptor stays pinned
until every child physically retires. A retained child can still make relative
identity checks through that pin; public directory operations cannot reopen the
closed lease. Known or unknown child close uncertainty retains the parent pin.

ASCII case-insensitive family-prefix accounting conservatively handles ordinary
case-alias opens on case-insensitive APFS/HFS and may count extra names on a
case-sensitive volume. Inventory first refuses every non-ASCII directory entry,
including unrelated sibling names. An ASCII requested root is insufficient:
APFS preserves Unicode spelling while looking up normalized equivalents, so
an intact imported Kelvin-sign-prefixed family can alias an ASCII K name.
The blanket refusal prevents undercount without claiming Apple Unicode
normalization compatibility. Use a dedicated directory containing only ASCII
entry names; its parent path may still contain Unicode. IDs and generated
suffix grammar remain strict. The
177-byte ASCII root-name cap reserves the longest 78-byte generated suffix
within a 255-byte basename. Correct Apple Unicode normalization/case equivalence
is an unimplemented capability, not an unrun acceptance test. Requested target basenames outside this
profile are refused before provider acquisition or stable-lock creation.
The additional directory-content restriction is enforced during inventory
scans; it does not promise to precede every credential or stable-lock step.

The source and destination of publication remain in the same pinned directory.
The candidate is opened managed/nofollow and its handle/name identity is checked.
Initial publication uses native `renameatx_np(..., RENAME_EXCL)`; replacement
uses `renameat`. The destination is then checked against the candidate inode and
single-link/type constraints. There is no check-then-replace approximation,
link/unlink, copy/delete, cross-volume or weaker fallback. Unsupported native
rename/filesystem capabilities return an error. Existing candidate Sync,
directory Sync, authentication and uncertainty rules are preserved.

Initial path resolution and metadata comparisons are cooperative observations.
They do not prove safety against a hostile actor racing the parent hierarchy,
retargeting names or modifying already-open file bytes. Keep the hierarchy under
trusted application control. None of those hostile operations were executed.

## Verification and sources

The focused command log and exact source hashes are in
`docs/evidence/native-encrypted-darwin-components.txt` and
`docs/evidence/native-encrypted-darwin-source-sha256.txt`. Portable tests use only
synthetic resources/callbacks for exceptional close, lock and quarantine laws.
Encrypted generic test inspections use `internal/testkit/fileview`: Darwin
reads through the managed VFS, while other platforms retain `os.ReadFile`.
This avoids dropping live `.gse-lock` holds during ciphertext-marker checks.
Linux correct-key intact histories and frozen vectors are separate regression
evidence. Target-only ordinary tests are compiled, never launched here.

No native Darwin or Windows execution, native process acceptance, failed native
I/O/lock/close/Sync, short write, disk tamper/wrong-key/missing-selected-file test,
retarget or namespace race, crash/process exit, hot recovery, interrupted
publication/rekey or physical-durability acceptance was run here or elsewhere.
Security/legal review and M3/M8/M10 qualification remain independent open gates.

Authoritative APIs checked on 2026-10-10:

- Apple [fcntl process-lock semantics](https://developer.apple.com/library/archive/documentation/System/Conceptual/ManPages_iPhoneOS/man2/fcntl.2.html)
- Apple [open/openat manual source](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/man/man2/open.2)
- Apple [renameat/renameatx_np and RENAME_EXCL manual source](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/man/man2/rename.2)
- Apple [APFS normalization/case-preservation policy](https://developer.apple.com/library/archive/documentation/FileManagement/Conceptual/APFS_Guide/FAQ/FAQ.html)
  and [Unicode character data](https://www.unicode.org/Public/9.0.0/ucd/UnicodeData.txt)
- Apple [64-bit directory-entry ABI](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/sys/dirent.h),
  [syscall declarations](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/kern/syscalls.master),
  and [getdirentries64 implementation](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/vfs/vfs_syscalls.c)
- Pinned `golang.org/x/sys v0.49.0`: Darwin Openat, Fstat/Fstatat/Fstatfs,
  FcntlFlock, Renameat and RenameatxNp wrappers and constants
- Pinned Go 1.27.2 Darwin `os.File` and `os.SameFile` behavior, and the existing
  plain managed Darwin protocol in ADR 0015

No new dependency, C engine, CGO production path or copied Apple implementation
was introduced. Official sources are API/protocol references only.
