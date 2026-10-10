# Native Windows VFS: implementation and acceptance boundary

Date: 2026-10-10. Target: `windows/amd64`, native Go with CGO disabled.
Status: implementation present; cross-compilation only. Native execution and
native SQLite interoperability are **NOT_RUN**. This is not M3 or M10 acceptance.

## Declared capability

The backend uses actual Win32 file handles, byte-range locks, and shared
file-backed mappings. It leaves the existing pager, WAL format, WAL-index hash
blocks, snapshot validation, and publication protocol intact.

| Area | Implemented contract | Acceptance or unsupported boundary |
| --- | --- | --- |
| Filesystem | Local fixed NTFS, checked by handle and resolved volume | NTFS runtime acceptance NOT_RUN; remote/SMB, removable, RAM disk, FAT/exFAT, ReFS, cloud-placeholder/reparse files unsupported |
| Paths | Absolute drive paths, including documented extended drive paths; handle-resolved directory aliases | UNC/device namespaces, ADS, ambiguous trailing spaces/dots, DOS device names, final reparse points unsupported |
| Identity | Full 128-bit file ID plus 64-bit volume identity, retained parent handle/identity, compatible metadata probes | Not protection against hostile raw writes, retarget races, or truncation outside cooperating clients |
| Live-file sharing | Read and write sharing, no delete sharing | Delete sharing exists only for explicitly replaceable exclusive-session WAL leases/candidates |
| DB locks | PENDING, RESERVED, and exact 510-byte SHARED/EXCLUSIVE range | Ordinary local/native and cross-process acceptance NOT_RUN |
| SHM locks | Eight exact one-byte intervals; explicit mode changes; partial/union release; rollback of only new holds | Windows kernel arbitrates separate local connection handles; no process-mutex substitute |
| DMS | First local lifecycle owner runs Windows exclusive-reset/shared handshake; later local owners acquire shared DMS without repeating reset | No fallback header erasure on truncation failure; interruption/initializer recovery NOT_RUN |
| Mappings | Native allocation granularity from reviewed amd64 SYSTEM_INFO; exact logical slices and stable original bases | Native 32 KiB first/later-block tests compiled only |
| File synchronization | Regular-file `FlushFileBuffers` | Physical durability remains unqualified |
| Directory synchronization | Intentionally absent; the directory hook succeeds without issuing a flush, matching the SQLite-win32-like policy | This is explicitly **not** a Unix-style namespace barrier; no interruption-safe namespace claim |
| Authorized WAL replacement | Existing portable EXCLUSIVE/role/path/identity gate, then same-volume `MoveFileExW(REPLACE_EXISTING | WRITE_THROUGH)` | No `COPY_ALLOWED`, copy/delete, or delete/rename fallback; no replacement execution in this work |
| Other Windows architectures | Unsupported native backend | No amd64 ABI layout is advertised as arm64/386 support |

## Handle lifecycle and path policy

`CreateFileW` opens synchronous, noninheritable handles. Read-only files request
read access; writable storage requests read/write access. Existing files are
never opened with a truncating disposition. `OPEN_ALWAYS` and `CREATE_NEW`
implement the authorized create modes. The parent directory is resolved before
choosing the final basename, checked as local fixed NTFS, and retained without
delete sharing. The final storage component is opened with
`FILE_FLAG_OPEN_REPARSE_POINT` and inspected before use.

`GetFileInformationByHandleEx` supplies FileIdInfo, FileStandardInfo, and
FileAttributeTagInfo. Writable regular files must have one hard link and no
delete-pending state. Regular files and directories are distinguished by native
metadata. Final normalized handle paths retain their verified extended-path
namespace. Metadata-only path probes verify both the original file name and
resolved parent against the captured identities. They do not use case-folded
path strings as an identity key. Cross-role aliases among managed leases are
also rejected by the inode registry.

The retained parent handle is an additional stabilization measure, not a claim
that every ancestor is immune to an uncooperative namespace change. The generic
identity checks remain in place as a second observation. Platform-specific
checks add the full native IDs missing from generic FileInfo.

## Lock ownership and poison

Every successful `LockFileEx` call has a recorded original interval and mode.
Calls use a fresh zeroed OVERLAPPED offset, `LOCKFILE_FAIL_IMMEDIATELY`, and no
asynchronous file mode. Only `ERROR_LOCK_VIOLATION` means ordinary contention.
Unexpected pending I/O or uncertain unlock/cleanup latches poison. Same-mode
repeats do not issue another native acquisition.

For database upgrade, exclusive PENDING is acquired even when there was no
RESERVED lock. The original shared 510-byte acquisition is released before
exclusive acquisition. If that acquisition conflicts, SHARED is reacquired
while PENDING is still held; PENDING is released only afterward. A restoration
or cleanup problem returns poisoned ownership, never a safely retryable BUSY.
Downgrade likewise restores SHARED before releasing PENDING, retaining any
RESERVED lock already owned. A reservation query uses a transient **shared**
RESERVED-byte probe, including on a read-only handle.

Each SHM slot is one exact native byte acquisition. Multi-slot acquisitions
skip already-owned same-mode slots and reject mode conversion before any call.
Failure rolls back only newly acquired slots, in reverse order. Logical release
can span distinct acquisitions or a subrange because each native release still
matches its original one-byte interval. Kernel locks remain per connection.

A registry keyed by native identity coordinates DMS lifecycle only. The first
local attachment attempts exclusive DMS and resets the transient index to zero
on success, then releases exclusive DMS and acquires shared DMS. If exclusive
DMS is busy, it attempts shared DMS without reset. Later attached local owners
skip the exclusive/reset stage and retain independent shared DMS holds. The
registry gate serializes final detach with a new attach. Each owner's views
retire before its DMS hold, so some DMS hold remains until every local view is
retired. No recovery/failure test is inferred from this ordinary initialization
algorithm.

Native close explicitly releases recorded locks, then closes the file and
retained parent. Successful partial cleanup updates only those native records.
An unresolved lock, view, or mapping-object close retains its protecting lease
for retry and rejects ordinary I/O. An `os.File.Close` error is conservatively
recorded as an uncertain terminal close; the backend does not repeat a close on
an OS handle whose state is unknown. Exceptional resources can therefore remain
retained. In addition, the existing failed-OpenSharedSession resource-retention
gap can return no reachable session through which to retry cleanup. This work
does not claim a bounded recovery queue or universal retryable cleanup.

## Mapped views

Allocation granularity comes from `GetSystemInfo`, not `os.Getpagesize` or a
hard-coded 64 KiB value. Reviewed amd64 compile-time checks cover SYSTEM_INFO
size and allocation-granularity offset, plus file-information structure sizes
and BOOLEAN field offsets. An offset is rounded down to native granularity;
only `delta + logicalLength` is mapped, exposing exactly the requested logical
slice. A valid 32 KiB initial index does not grow merely because allocation
granularity is 64 KiB. The mapping maximum is the existing backing-file size.
Later mappings may overlap a preceding native view; existing views never move.

Each mapping retains its original base, view length, mapping HANDLE, and owning
file. Teardown first closes the mapping-object HANDLE, then unmaps the original
base. Microsoft documents that the HANDLE can be closed while views remain
mapped. That order keeps the exposed slice live if either operation fails;
successful stages clear only their own record. DMS and the ordinary file handle
remain retained until actual view retirement. A mapping-object cleanup failure
after unsuccessful view creation is likewise retained by the file. No durable
SHM flush substitutes for database/WAL synchronization.

## Source provenance

Implementation uses the already pinned `golang.org/x/sys v0.49.0` wrappers.
No new dependency, download, cgo, C engine, or SEE implementation is involved.
The reviewed SQLite algorithm source is the locally pinned public-domain
`sqlite-src-3540000/src/os_win.c`, particularly `winLock`,
`winCheckReservedLock`, `winLockSharedMemory`, and the per-connection SHM
locking-handle behavior. Microsoft sources were checked on 2026-10-10:

- [CreateFileW](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilew)
- [FILE_ID_INFO](https://learn.microsoft.com/en-us/windows/win32/api/winbase/ns-winbase-file_id_info)
- [FILE_STANDARD_INFO](https://learn.microsoft.com/en-us/windows/win32/api/winbase/ns-winbase-file_standard_info)
- [GetFinalPathNameByHandleW](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-getfinalpathnamebyhandlew)
- [GetVolumePathNameW](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-getvolumepathnamew) and [GetDriveTypeW](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-getdrivetypew)
- [LockFileEx](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-lockfileex) and [UnlockFileEx](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-unlockfileex)
- [SYSTEM_INFO](https://learn.microsoft.com/en-us/windows/win32/api/sysinfoapi/ns-sysinfoapi-system_info) and [GetSystemInfo](https://learn.microsoft.com/en-us/windows/win32/api/sysinfoapi/nf-sysinfoapi-getsysteminfo)
- [CreateFileMappingW](https://learn.microsoft.com/en-us/windows/win32/api/memoryapi/nf-memoryapi-createfilemappingw), [MapViewOfFile](https://learn.microsoft.com/en-us/windows/win32/api/memoryapi/nf-memoryapi-mapviewoffile), and [Closing a File Mapping Object](https://learn.microsoft.com/en-us/windows/win32/memory/closing-a-file-mapping-object)
- [FlushFileBuffers](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-flushfilebuffers)
- [MoveFileExW](https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-movefileexw)

## Compiled native ordinary tests

`internal/vfs/native_windows_test.go` contains these named cases:

- `TestWindowsDBLockTransitionsAndProbeRetention`
- `TestWindowsDirectSharedExclusiveTransition`
- `TestWindowsSHMExactIntervalsRollbackAndUnion`
- `TestWindowsMappingGranularityAndDMSLifecycle`
- `TestWindowsHandleIdentityAndSyncPolicy`
- `TestWindowsPathPolicyPureValidation`

They exercise only ordinary cooperating behavior and pure pathname validation.
They have not run on this Linux host. No error injection, active namespace
replacement/retarget, tampering, wrong-key/corruption, selected-file removal,
crash/kill/process-exit, interrupted publication/rekey, or hot recovery was run.
Native process/SQLite interoperability, physical durability, interrupted
recovery, long soak, and release/security review remain separate missing gates.

## Static-analysis qualification

Default cross-target `go vet ./internal/vfs ./internal/pager ./internal/walindex`
reports one `possible misuse of unsafe.Pointer` diagnostic at the conversion
of the successful `MapViewOfFile` address into its byte slice. Default Windows
vet is therefore **not clean**. This address is OS-owned mapped memory, never
a Go-heap pointer converted to an integer and left unrooted. The conversion is
the direct `unsafe.Slice((*byte)(unsafe.Pointer(address)), length)` form also
used in the pinned Go 1.27.2 sources `src/internal/fuzz/sys_windows.go` and
`src/cmd/go/internal/mmap/mmap_windows.go`. View length is validated before the
native call, and the original view base and owner remain explicitly retained.
The diagnostic is recorded rather than hidden with type-punning, fabricated
slice headers, analyzer suppression, or disabled default-vet claims. Native
execution is still required to validate this unsafe/native boundary.

## Source-review correction: exceptional metadata probes

A later source review on2026-10-10 identified an ownership gap not covered by
previous portable tests/cross-builds. Common VFS/pager/WAL-index os.Lstat/Stat plus
os.SameFile paths can cause Go1.27.2 os/types_windows.go:loadFileId to open a raw
metadata handle and discard its deferred CloseHandle result. Native VFS path
validation also opens metadata probes and returns close errors without retaining
an unresolved probe record. These handles are outside the stated managed-close
accounting. Earlier evidence remains factual, but does not establish all-probe
exceptional retirement or bounded cleanup.

Windows byte locks are per handle: closing a metadata alias does not itself
release another handle's locks. No data loss or ordinary locking failure has been
proven, and no native close/fault experiment was executed. Required corrective
code is managed/bounded metadata probes across the plain VFS/pager/WAL-index
call chain, followed by portable laws and separately unrun native qualification.
The encrypted Windows backend is receiving its own equivalent fix before release.
