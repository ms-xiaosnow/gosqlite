# Plain Windows metadata ownership correction

Date: 2026-10-10. Target: Windows/amd64; production remains pure Go, CGO disabled.
This is a corrective implementation and scoped source/model review, not native
runtime, durability, security, or release acceptance.

## Why prior evidence was insufficient

The earlier `native-ownership-*` and `native-windows-*` evidence remains an
accurate record of what ran. It did not cover all native metadata resources.
The pinned Go 1.27.2 sources expose three relevant hidden-open paths:

- `src/os/types_windows.go`: `fileStat.loadFileId`, reached by `os.SameFile`
  for pathname snapshots, opens a raw file handle and discards deferred close
  errors. Handle snapshots already have the Go library's narrower file ID.
- `src/os/stat_windows.go`: `Stat`/`Lstat` have handle/search fallback paths
  whose close outcomes are discarded. Calling them without `SameFile` does
  not remove the complete exceptional ownership issue.
- `src/path/filepath/symlink_windows.go`: pathname normalization calls
  `FindFirstFile`/`FindClose`; the common symlink walker also uses `os.Lstat`.

The old VFS's explicit `nativeValidatePath` probes checked a `CloseHandle`
error but did not retain a corresponding owner. Its native-open parent failure
cleanup had the same unrepresented retirement issue.

This is an exceptional resource-accounting gap, not an established data-loss
incident. Windows file locks belong to their acquisition handle, so closing a
separate metadata handle does not by itself release another handle's locks.
No physical close/failure experiment was executed to investigate this finding.

## Implemented boundary

`internal/vfs/metadata_windows.go` now supplies platform-owned metadata
snapshots, `Lstat`, `SameFile`, and `ResolvePath`. The snapshot contains the
64-bit volume identity and all 128 file-ID bits obtained from `FILE_ID_INFO`.
Size, attributes and write time come from the same already-owned handle.
`SameFile` compares only these immutable IDs and performs no lazy native open.
No metadata snapshot exposes a HANDLE, `os.File`, finalizer, or lazy path.

Metadata probes open with `FILE_READ_ATTRIBUTES`, `OPEN_EXISTING`, compatible
read/write/delete sharing, and `BACKUP_SEMANTICS`. Final pathname probes use
`OPEN_REPARSE_POINT`; inspected reparse/device/offline/recall/deletion-pending
objects remain unsupported. Alias resolution follows its target while choosing
the canonical database family, then uses checked handle paths and local fixed
NTFS validation. Actual live database/WAL/SHM/journal leases retain their existing
no-delete-sharing policy; only explicitly replaceable WAL data leases differ.
Parent-directory handles retain the stricter no-delete-sharing policy.

The common VFS identity check still compares the current handle, original
handle snapshot, current pathname and parent snapshot. Native Windows checks
also revalidate its retained parent handle. Replacing `os.SameFile` is not
removing an identity check; each Windows comparison now uses the complete
native ID and owned probes. The pager's size/time/header/WAL checks and the
WAL-index's mapped-extent checks remain in place.

All internal plain pager/WAL-index metadata paths use these helpers, including
sidecar absence/presence observations, read-only inspection, journal source-only
recovery, imported-WAL source paths, and canonical path resolution. Generic
non-Windows helpers delegate to the same existing `os`/`filepath` operations,
so Linux OFD and Darwin process-lock algorithms are not changed by this
correction. The public `sqlite.OpenContext` prelude also dispatches through
`sqlite/path_windows.go`, eliminating generic metadata/search handles before
entering the pager. `sqlite/path_default.go` is a faithful extraction of the
previous non-Windows prelude, including its encrypted final-link policy.

Encrypted Windows public path selection calls the new
`encstore.CanonicalWindowsPath` helper, which delegates directly to the existing
managed `canonicalPath`. This small integration depends on the independently
reviewed Windows encrypted backend source tree frozen at
`d52c82cfa393f220bfcd2a6c54fe4f6c9b47b3d3`; it makes no changes to that backend's
platform, locking, namespace or retirement implementation. `DB.path` and key
provider requests use the same backend canonical family. Public encrypted input
also reaches the backend's strict path/family-name preflight before a provider
call. No broader encrypted-storage acceptance follows from this integration.
A source-level regression checks the VFS, pager, WAL-index, public SQLite and
driver call chain and verifies both non-Windows build exclusions.

## Bounded owner and exact outcomes

`metadataOwners` is a synchronized, strongly retained registry with at most
16,384 metadata slots process-wide for the plain Windows backend. One slot is
reserved before each native creation attempt, for both temporary probes and
retained parents. Capacity exhaustion returns an explicit unsupported/resource
limit before opening anything; no resource is evicted. A failed native create
releases only its reservation, because no handle was acquired.

A successful close retires precisely that owner's record and releases its slot.
A failed close retains its handle value as an indeterminate tombstone and
latches `ErrPoisoned`. The numeric handle is never passed to CloseHandle again:
no assumption is made that an error proves it remains a valid, unreused handle.
No snapshot or canonical path is published after an unresolved probe close.
The plain Windows metadata gate refuses new probes/opens; managed file entry
checks reject subsequent I/O and lock operations. This is deliberately a
process-wide fail-closed availability restriction for this backend, not a
resource recovery promise. Other already-known resources may still be retired
through explicit Close. Tombstones have no automatic reset, retry or eviction.

Retained parent handles use this same record from native-open setup through
normal/failed-open cleanup, rather than raw close calls. Reader, transaction,
exclusive-session and WAL-index failed-open cleanup propagate close outcomes;
metadata error wrapping preserves poisoned ownership and its original cause.
Plain ReadAt, generic SectionReader/WAL-scan, mapping/write entry and close
adapters preserve that classification through their existing public error
wrappers, including the zero-dirty commit-close path. A preliminary optional
WAL-header read cannot ignore metadata poison and proceed with publication.
The separate unresolved-view path still protects DMS/main locks and retains the
original mapping ledger. This correction does not implement the distinct
failed-OpenSharedSession pending-retirement group or prove arbitrary mapping
handle-close retries safe.

## Evidence and limits

| Surface | Evidence/contract | Unsupported or unverified |
| --- | --- | --- |
| Reservation, capacity, successful/uncertain retirement | Real owner state machine exercised with synthetic integers and memory callbacks | No failed native CreateFile/CloseHandle calls executed |
| Probe and rejected-open composition | Production observation gate tested across abstract inspect/close outcomes; rejected-open parent retention modeled | Native calls are not replaced or fault-injected |
| Public prelude and error propagation | Ordinary plain/exclusive/shared/encrypted create/reopen and provider-path agreement; fake ReaderAt/Close error models | Windows public execution and native close-failure outcomes NOT_RUN |
| Indeterminate numeric handle | Retained tombstone, no second close; known sibling cleanup remains possible | No reclamation, reset, or retry of an uncertain handle |
| Snapshot/identity | Ordinary Linux metadata/file operations and source-level call-chain regression | Windows full-ID test compiled only; native runtime NOT_RUN |
| Plain lock/mapping algorithms | Existing exact Windows ledger unchanged; scoped ordinary Linux regression/SQLite oracle | Native Windows/Darwin process/SQLite qualification NOT_RUN |
| Namespace | Checked parent and full file identity observations | No hostile namespace anchoring/retarget guarantee or race test |
| Recovery and durability | Recovery paths source-reviewed and compile-only | No physical failure, crash/forced exit, hot recovery, corruption/tamper, missing selected file, or interrupted publication execution |

Exact commands, owned source paths and hashes are recorded in
`docs/evidence/native-plain-metadata-components.txt`,
`docs/evidence/native-plain-metadata-owned-paths.txt`, and
`docs/evidence/native-plain-metadata-source-sha256.txt`. The scoped independent
source review is in `docs/evidence/native-plain-metadata-independent-review.txt`.
Earlier evidence is not rewritten or promoted to cover this correction.

## Primary sources reviewed

The pinned Go source files above were read directly from `../toolchains/go1.27.2`.
Microsoft documentation was checked on 2026-10-10:

- [CreateFileW](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-createfilew): metadata access, compatible sharing, backup semantics, final-reparse behavior and returned handle ownership.
- [GetFileInformationByHandle](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-getfileinformationbyhandle) and [GetFileInformationByHandleEx](https://learn.microsoft.com/en-us/windows/win32/api/winbase/nf-winbase-getfileinformationbyhandleex): metadata from an existing handle.
- [FILE_ID_INFO](https://learn.microsoft.com/en-us/windows/win32/api/winbase/ns-winbase-file_id_info): complete file and volume identity.
- [CloseHandle](https://learn.microsoft.com/en-us/windows/win32/api/handleapi/nf-handleapi-closehandle): checked success/failure and one close per opened handle. Conservatively retaining every failed metadata close as terminal uncertainty is this implementation's policy, not a documented promise that all failures preserve a valid handle.
- [LockFileEx](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-lockfileex): acquisition-handle lock semantics, distinct from descriptor-wide POSIX close effects.
