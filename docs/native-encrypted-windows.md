# Native encrypted storage on Windows: bounded implementation

Date: 2026-10-10. Target: Windows/amd64, Windows 10 1709 or later, local fixed
NTFS, CGO disabled. **Native execution is NOT_RUN.** This implementation is
experimental and does not qualify M8, M3, M10, security or physical durability.
Darwin encrypted storage is still a missing implementation, not an unrun test.

## Capability and missing implementation matrix

| Platform / operation | Implementation | Evidence boundary |
| --- | --- | --- |
| Linux amd64/arm64 encrypted modes 0, 1 and 2 | Existing OFD backend unchanged | Scoped ordinary Linux regressions and frozen format vectors pass; separate historical acceptance gates remain |
| Windows/amd64 modes 0 and 1 | Native handles, stable lifecycle locks, handle-relative publication | Cross-build/compile-only tests; native runtime NOT_RUN |
| Windows/amd64 mode 2 reads, writer, refresh, checkpoint | Native exact byte-0/1/2 locking and immutable-selection publication | Cross-build/compile-only tests; native runtime NOT_RUN |
| Windows legacy-to-shared explicit upgrade | Exclusive lifecycle retained through publication, then shared-over-exclusive downgrade | Source and memory-model evidence; native runtime NOT_RUN |
| Windows mode-2 rekey | **Unsupported implementation gap** | Refused before target provider acquisition and before writer reservation; no unlock/relock substitute |
| Windows encrypted archive, restore, explicit plaintext copy helpers | Managed handle and retained-directory lifetime paths | Compiled only on Windows; inherited ordinary Linux API regressions pass |
| Darwin encrypted storage | **Unsupported implementation gap** | Needs a coordinated inode owner covering every encrypted descriptor and plain-VFS alias, deferred close, namespace and publication primitives |
| Other OSes and Windows architectures | Explicit unsupported backend | Cross-building a stub is not native storage support |
| Windows directory namespace barrier | **Not supplied** | Directory Sync performs no flush; successful publication does not establish Unix-style namespace persistence |

Modes remain `Snapshot=0`, `WAL=1`, `WALShared=2`; root layouts 1/2/3, crypto
records, child references, content commitments, archive bytes and protocol bytes
are unchanged. No SQLite C engine, CGO production dependency, new module or
fallback filesystem implementation is introduced. `platform_linux.go` remains
byte-identical to its frozen shared-encryption baseline.

## Native namespace and metadata

Every opened Windows file is a private managed lease. Its handle is synchronous
and noninheritable. Existing files use open-only dispositions; creation uses
open-if or exclusive-create as requested, never truncation/overwrite. A retained
parent directory handle is opened without delete sharing and verified as local
fixed NTFS. Windows 10 1709+ is checked before opening storage; target-directory
capability is checked before a target key provider is invoked by `Prepare`.
Unsupported OSes are rejected before `Open` can acquire a creation credential.

Live member opens use the pinned directory as `OBJECT_ATTRIBUTES.RootDirectory`
in `NtCreateFile`. Names are single validated basenames; no absolute-path
reconstruction is used for live member lookup. Final reparse processing is
refused with `FILE_OPEN_REPARSE_POINT` and handle inspection before data access.
Every resolved member has exactly one component, so no intermediate reparse
component exists. The backend does not depend on version-variable
`OBJ_DONT_REPARSE` or retry with weaker flags. Full native volume
and 128-bit file IDs, attribute tags, disk type, regular-file/directory type,
delete-pending state and single hard-link count are checked. Reparse, device,
offline/recall/cloud-placeholder storage, UNC/device/ADS paths, invalid UTF-8,
DOS-device names, ambiguous trailing dots/spaces, parent traversal and oversized
basenames are unsupported. User target basenames are capped at 177 UTF-16
units, reserving the 78-unit longest future generated suffix before any native
open or credential request. This conservative target cap also applies to
archive and explicit plaintext-output paths. Ordinary relative paths resolve
through a native directory handle and `GetFinalPathNameByHandle`; explicit
`.`/`..` components are conservatively refused.

Immutable files and `.gse-lock` do not share delete access. Only a namespace's
selected publication root has normal delete sharing; this supports replacement
while a legacy connection retains the older root handle. Temporary metadata and
rename handles share delete access to avoid disturbing that policy. Metadata
probes open relative to the actual retained directory and compare full native
IDs, plus the existing extent/time and generic identity observations. An
independent named-directory probe detects a changed parent identity. Named
probes and Windows canonical path resolution all use managed handles; common
`os.Lstat`/lazy `os.SameFile` opens are confined to non-Windows helpers. A
held-Stat/held-Stat `os.SameFile` comparison does not open an extra handle.

Inventory uses Windows `CompareStringOrdinal(ignoreCase=1)` for only the
root/family prefix, so a case-alias reopen cannot omit retained entries solely
because its spelling differs. IDs and suffix grammar remain strict. This
ordinal name classification is accounting, never a substitute for full native
handle identity. Enumeration opens the retained directory directly, without a
redundant path-open whose close could escape error reporting.

Initial selection/preflight still uses path-based operations, and raw external
writers remain outside the cooperative contract. These checks do not promise
protection against a compromised OS or hostile namespace mutation. Keep the
hierarchy under trusted application control. No mutation-race test was run.

## Locks and lifecycle conversion

The stable `.gse-lock` retains existing byte 0 lifecycle, byte 1 writer, and byte
2 capture-gate meanings. `LockFileEx` uses `LOCKFILE_FAIL_IMMEDIATELY`, a fresh
OVERLAPPED offset and a one-byte interval. Only `ERROR_LOCK_VIOLATION` is BUSY.
The ledger records every successful exact acquisition. Same-mode repeats are
idempotent; unlock calls match the original interval. Unexpected acquisition or
release results latch poison and block ordinary I/O and lock reuse.

Microsoft permits a shared acquisition over this handle's exclusive interval,
and specifies that the first unlock releases exclusive. Downgrade therefore
acquires shared first, records both holds, then releases exclusive. If release
is unresolved, both successful records remain represented and the owner is
poisoned. Cleanup drains capture, writer, then lifecycle; it preserves every
unreleased record and the actual handle on partial failure.

The reverse conversion is not available: exclusive cannot overlap shared.
Dropping shared and reacquiring exclusive would violate the existing protocol
and permit a legacy lifecycle owner to enter. Shared-mode rekey is therefore
explicitly unsupported on Windows. `Prepare` rejects it before filesystem
preflight or provider acquisition, and `BeginMaintenance` rejects it before
writer reservation. No new protocol byte, hidden exclusive-session fallback,
process mutex or weakened rekey claim is introduced.

## Real resource lifetime

All handles enter a strong managed-owner registry before validation can fail.
A directory's public close marks its lease closed but its native handle remains
until every child physically retires. Already retained children may perform
relative metadata probes through this pin; the public closed bit is never
cleared or reopened. Directory enumeration obtains its own native directory
handle. Ordinary read/write/stat/Sync and lock operations reject closed or
poisoned ownership.

Close first retires exact locks and then closes the native file once. A failed
unlock retains the handle and remaining records for safe internal cleanup retry.
An `os.File.Close` error is an uncertain terminal native close; no later call
retries that numeric handle, which might have been reused. Unknown resources and
parent pins remain strongly retained. A process-wide ceiling of 16,384 managed
handles includes opens in progress and unresolved resources and is reserved
before each native open. Capacity exhaustion refuses further opens; it never
evicts a live resource. Existing per-operation encrypted graph budgets remain.

This is fail-closed containment, not universal recoverability: common Store or
failed-open cleanup may drop the caller's retry route while the private registry
retains exceptional resources. There is no background collector or claim that
all exceptional resources are eventually retired. Failure paths were examined
as source and exercised only as pure-memory state laws, never OS failures.

## Publication and synchronization

Candidate bytes are fully written, synchronized and authenticated by the existing
storage algorithm before cutover. Rename opens the candidate relative to the
pinned parent with DELETE access, rechecks its handle/name identity, and submits
`NtSetInformationFile` with `FileRenameInformationEx` (65) and the same parent
handle plus one target basename. Initial publication sets no replace flag.
Replacement uses `FILE_RENAME_REPLACE_IF_EXISTS | FILE_RENAME_POSIX_SEMANTICS`,
keeping older root handles valid. A relative destination probe verifies the
candidate's full native identity after success. There is no legacy-rename,
MoveFileEx, copy/delete, unlink/rename or cross-volume fallback.

Regular-file Sync uses Go's Windows `FlushFileBuffers` path. Directory Sync is
an explicit no-barrier policy, consistent with the separate plain Windows VFS;
it does not issue a successful-looking native directory flush. Native rename
and file synchronization do not prove power-loss ordering or namespace
persistence. Existing conservative uncertain-publication/poison behavior stays
in force from attempted cutover onward. Interrupted publication and rekey,
physical durability, hostile filesystem behavior and recovery remain unaccepted.

## Verification and sources

`docs/evidence/native-encrypted-windows-components.txt` records exact commands,
results and the source freeze. Five target-only tests compile:

- `TestWindowsEncryptedExactLocksAndProbeLifetime`
- `TestWindowsEncryptedRelativePublicationAndRetainedRoot`
- `TestWindowsEncryptedStandaloneParentRetention`
- `TestWindowsEncryptedPathPolicyMemory`
- `TestWindowsEncryptedSharedRekeyPreflightMemory`

Fourteen portable ownership memory tests exercise exact lock acquisition/release,
all 64 held-state combinations, 192 partial-close positions, stacked downgrade,
parent-child retention, cleanup retry and terminal-unknown-close containment.
Two further memory tests cover strict family-tail classification and the UTF-16
name envelope, and four independent composition tests combine parent lifetime,
exact lock retirement and unknown-close retention.
They do not execute a Windows syscall or qualify a filesystem. Existing Linux
ordinary correct-key create/read/write, pinned readers, stale promotion,
checkpoint, savepoints, rekey and copy regressions are separately scoped.

No Windows/Darwin runtime, native cross-process acceptance, failed native
I/O/lock/close/Sync, disk tamper/wrong-key/missing-selected-file test, crash,
process exit, interrupted publication/rekey, hot recovery or hostile namespace
mutation was executed here or elsewhere. Independent source/security/legal and
full release review remain separate gates. No tests were relocated to CI.

Official Microsoft APIs were checked on 2026-10-10:

- [NtCreateFile](https://learn.microsoft.com/en-us/windows/win32/api/winternl/nf-winternl-ntcreatefile)
- [FILE_RENAME_INFORMATION](https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/ntifs/ns-ntifs-_file_rename_information)
- [FILE_INFORMATION_CLASS](https://learn.microsoft.com/en-us/windows-hardware/drivers/ddi/wdm/ne-wdm-_file_information_class)
- [LockFileEx](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-lockfileex)
- [UnlockFileEx](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-unlockfileex)
- [CompareStringOrdinal](https://learn.microsoft.com/en-us/windows/win32/api/stringapiset/nf-stringapiset-comparestringordinal)
- [FlushFileBuffers](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-flushfilebuffers)

The already pinned `golang.org/x/sys v0.49.0` supplies the native wrappers.
Pinned Go 1.27.2 `src/internal/syscall/windows/at_windows.go` provides an additional
source cross-check for native relative open/rename structures and class 65.
Its fallback rename behavior is deliberately not adopted. No dependency was
installed or downloaded. Existing third-party notices remain applicable.
