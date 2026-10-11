# Native encrypted storage on Windows: bounded implementation

Source update: 2026-10-11. Target: Windows/amd64, Windows 10 1709 or later, local fixed
NTFS, CGO disabled. **Native execution is NOT_RUN.** This implementation is
experimental and does not qualify M8, M3, M10, security or physical durability.
Darwin has a separate bounded implementation with native runtime NOT_RUN; its
ASCII inventory/Unicode work is not changed by this Windows composition. This
page describes unpublished source based on b0e21b81; exact identities and separate
results are in [the shared-rekey record](windows-shared-rekey.md).

## Capability and missing implementation matrix

| Platform / operation | Implementation | Evidence boundary |
| --- | --- | --- |
| Linux amd64/arm64 encrypted modes 0, 1 and 2 | Existing OFD backend unchanged | Scoped ordinary Linux regressions and frozen format vectors pass; separate historical acceptance gates remain |
| Windows/amd64 modes 0 and 1 | Native handles, stable lifecycle locks, handle-relative publication | Cross-build/compile-only tests; native runtime NOT_RUN |
| Windows/amd64 mode 2 reads, writer, refresh, checkpoint | Native exact byte-0/1/2 locking and immutable-selection publication | Cross-build/compile-only tests; native runtime NOT_RUN |
| Windows legacy-to-shared explicit upgrade | Exclusive lifecycle retained through publication, then shared-over-exclusive downgrade | Source and memory-model evidence; native runtime NOT_RUN |
| Windows mode-2 rekey | Guarded admission/conversion plus post-conversion exact source-authority check | Implemented source; portable laws/source-AST and separate Linux ordinary checks; native Windows NOT_RUN |
| Windows encrypted archive, restore, explicit plaintext copy helpers | Managed handle and retained-directory lifetime paths | Compiled only on Windows; inherited ordinary Linux API regressions pass |
| Darwin encrypted storage | Separate bounded shared plain/encrypted inode owner and namespace implementation | Native runtime NOT_RUN; existing ASCII inventory and Unicode gap remain |
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

Exclusive cannot overlap shared on the same handle. A direct ledger upgrade is
still refused without dropping protection, but shared rekey now uses the separate
bounded protocol in [lifecycle_windows_protocol.go](../internal/encstore/lifecycle_windows_protocol.go):

1. Every updated initial lifecycle opener, including read-only/shared and legacy
   modes, takes exclusive capture byte 2, acquires byte 0, then releases capture.
2. Upgrade requires shared byte 0, exclusive writer byte 1 and no capture hold.
   It takes capture, releases shared byte 0, tries exclusive byte 0 once, and
   releases capture on success. This is a real gap, not atomic LockFileEx upgrade.
3. An ordinary exclusive-acquisition BUSY restores shared byte 0 before releasing
   capture and returning ordinary BUSY. Failed restoration (including BUSY),
   unexpected native result or failed release poisons the owner and retains the
   remaining exact protectors. A joined BUSY/poison is not a retryable BUSY.
4. Older binaries can ignore admission. After conversion, `BeginMaintenance`
   therefore takes capture again and checks the pinned directory/lock identity,
   authenticates the current named root using the retained source master/options,
   compares its exact bytes with the pre-gap refreshed selection, and checks
   handle/name identity. Refusal joins gate release and `EndMaintenance(false)`.
   No source-provider reacquisition or different post-gap graph adoption occurs.

The writer reservation and latest-head refresh precede conversion. The authority
check precedes copy budgeting, full-source verification, logical validation and
publication. Existing publisher checks and rename-attempt uncertainty remain.
The common authority check also applies to successful shared rekey on Linux and
Darwin; their platform backend source is unchanged. Target credential preparation
is the existing earlier step, not a source credential refresh after the gap.

No new protocol byte, persistent format, hidden exclusive-session fallback,
process mutex or retry loop is introduced. This source implementation does not
establish actual mixed-old-binary or native Windows safety. See the
[protocol and evidence boundary](windows-shared-rekey.md).

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

[The original component evidence](evidence/native-encrypted-windows-components.txt)
records its historical commands/results and source freeze. At that old checkpoint,
the five target-only names were:

- `TestWindowsEncryptedExactLocksAndProbeLifetime`
- `TestWindowsEncryptedRelativePublicationAndRetainedRoot`
- `TestWindowsEncryptedStandaloneParentRetention`
- `TestWindowsEncryptedPathPolicyMemory`
- `TestWindowsEncryptedSharedRekeyPreflightMemory`

That list is historical. The namespace function now lives in
[namespace_manual_windows_amd64_test.go](../internal/encstore/namespace_manual_windows_amd64_test.go)
under `manual_namespace` plus `GOSQLITE_MANUAL_NAMESPACE=independently_authorized`;
this composition leaves its source/gate unchanged. The old preflight-refusal
name is removed and replaced by `TestWindowsEncryptedSharedRekeyCapability` in
[native_windows_test.go](../internal/encstore/native_windows_test.go). Despite the
old PathPolicyMemory name, that existing target test invokes native ordinal path
classification; it must not be counted as Linux inert-memory execution. The new
capability test calls the native version API, but opens no database. Neither was
run on Windows. Current compile/static-symbol qualification is separately listed
in [the new record](windows-shared-rekey.md), never inferred from the old binary.

The new ten `TestWindowsLifecycleMemory*` laws are portable inert callback/ledger
models with 20 named children (30 paths). `TestWindowsLifecycleSourceIntegration`
reads/parses two source files; it is not native or full control-flow proof. On b0e,
these two separate normal commands passed, followed by two separate correct-key
Linux shared-rekey tests (one path each); another CLI normal passed four paths.
The exact b0e code tree's 17 gates now PASS (14:42 UTC): ordinary/race each
5,602 PASS + seven expected oracle skips, complete oracle 7,234 PASS, and the
specific LIMIT oracle-race pair 394 PASS. Five target builds and Windows encstore
test compilation plus host nm succeeded; the PE was never run. This is not native
Windows acceptance or a test run of the later documentation composition. The
[full record](windows-shared-rekey.md) explains the host nm bootstrap and exact
command/evidence boundary. Old candidate/main passes are not transferred.

Fourteen earlier portable ownership memory tests exercise exact lock acquisition/release,
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
mutation is established by this Windows evidence. Historical broader-scope
namespace runs are separately corrected in [the scope audit](test-scope-audit.md). Independent source/security/legal and
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
