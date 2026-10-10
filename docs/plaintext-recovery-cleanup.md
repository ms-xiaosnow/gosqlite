# Explicit rollback recovery: managed cleanup ownership

Date: 2026-10-10. This is a source-level extension of the bounded plaintext
ownership protocol to the internal `pager.RecoverRollback` operation. It does
not enable recovery from Open, OpenTx or SQL. The replay parser, replay geometry,
page application, locking order and on-disk formats are unchanged. Parser support
is independently specified by `internal/journal`; this change does not impose a
permanent single-segment restriction.

## Admission and ownership

Recovery reserves one of the existing 1024 opening/live/pending plaintext roots
before absolute-path resolution, metadata probes or main acquisition. The fixed
main and temporary slots already exist in every root. The temporary slot is
explicitly reserved before each candidate/replay journal metadata acquisition.
The candidate and replay file occupy that slot sequentially. A successful file
is adopted immediately, before stat, link validation or identity checks. A failed
constructor never directly closes its acquired journal: the outer ownership
scope adopts its error-carried metadata/constructor obligations first.

Retirement order is nested obligations, candidate/replay journal, then protecting
main descriptor/locks. `RetirementComplete` controls advancement independently
of diagnostics. An incomplete nil result remains pending; a consumed error is
still returned. A failed explicit journal close stops all later logical work,
including unlink, and the defer does not immediately retry that same close.
The main owner remains protected until the child conclusively retires.

The registry retains managed owners, bounded diagnostics and its current immutable
token only. It does not retain the recovery function, replay plan, page data,
options, result, DB/Tx/Rows or application callbacks. Existing 4096-node bounded
owner discovery, opaque unsafe-graph barriers, immutable tokens, snapshot behavior,
first-cause retention and no-eviction semantics apply unchanged. This is a bound
on library-owned records; custom opaque errors can retain arbitrary caller data.

The existing `PendingPlaintextCleanups` and `RetryPlaintextCleanup` routes include
these internal recovery roots because they use the same registry. A discarded
recovery error cannot discard pending ownership. Root admission and snapshot
operations never perform native retirement or wait on an owner mutex.

## Absence, diagnostics and retry

No-journal and no-WAL-sidecar decisions use the existing VFS initial no-handle
metadata absence provenance. A later open, stat, identity or close error cannot
be suppressed merely because it matches NotExist. The candidate first-byte read
ignores only direct EOF; a wrapped or joined EOF remains a diagnostic and is
adopted before candidate retirement.

Once database replay starts, operation and retirement errors keep the existing
poison plus RecoveryRequired classification before journal removal, and poison
plus CommitUncertain classification after removal. Those classifications enter
the retained token as well as the immediate return. Any error clears the returned
RecoveryResult. A successful retirement does not invent an error classification.

Explicit retry only advances already-owned managed retirement. It never repeats
journal parsing, page replay, truncation, Sync, directory synchronization, unlink,
path acquisition, lock acquisition or another logical recovery attempt. Historical
tokens retain their marker after completion; completed retry is a no-op. A retained
terminal native owner can consume capacity for process lifetime.

## Verification scope

New `TestPlainRecoveryCleanupMemory*` laws exercise nested/journal/main barriers,
consumed diagnostics, skipped duplicate child attempts, root/temporary admission,
sequential child reuse, snapshot survival, first causes, classification, bounded
error discovery, and mixed EOF diagnostics. They use descriptor-free fake owners.
`TestPlainRecoveryCleanupSourceAdmissionAndRetirementBoundary` reads source ASTs
and checks admission/adoption ordering, no raw closes or IsNotExist suppression,
child-before-unlink placement and the retirement helper's restricted call surface.
It does not execute recovery. Existing bounded VFS owner/absence laws, plaintext
retirement laws and byte-slice parser tests provide adjacent regression coverage.

The selection is guarded by `go run ./internal/testscope`; ordinary intact
Reader/transaction/database/sql create/read/write/commit/close/reopen tests are
separately selected. Compilation targets Linux amd64/arm64, Darwin arm64 and
Windows amd64 without CGO; race checks are limited to selected memory laws and
ordinary intact lifecycle tests with CGO enabled for the detector only. Source
and evidence inventories accompany the frozen patch.

**NOT_RUN:** physical recovery, native recovery fixtures, manual tagged tests,
corruption/tamper/wrong-key/missing-selected-file workflows, namespace replacement/
retarget/race injection, crash/kill/process exit/hot recovery, failed native I/O/
lock/unmap/close/Sync/shortwrite/diskfull/permissions, interrupted publication/rekey,
and Darwin/Windows native runtime qualification. Nothing in this increment
qualifies physical recovery, release readiness, or full M3/M8/M10 acceptance.
