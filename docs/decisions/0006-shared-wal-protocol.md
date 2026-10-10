# ADR 0006: Native shared WAL coordination

Status: implemented for the documented ordinary Linux subset; interruption/recovery acceptance remains blocked.

## Decision and protocol

Provide an explicit WALShared option, separate from the older exclusive alternative. Production remains native Go; public SQLite 3.54.0 source is the interoperability reference, never an engine dependency. A session retains the SQLite database SHARED lock and SHM dead-man-switch lock. Transactions hold real SHM read locks, with immutable generation/max-frame/min-frame bounds; a connection permits one lease, while independent connections operate concurrently.

internal/walindex implements native-endian checked dual48-byte headers,32KiB page/hash blocks (first block4062 frame entries, subsequent4096), reader marks, backfill/attempt counters and Linux OFD byte locks. Mapped atomic publication/observation uses barriers. Lookup is a real frame-bounded hash probe, not rescanning the WAL per page. Unsupported platforms fail closed rather than pretend cross-builds establish native behavior.

A deferred writer acquires WRITE and rechecks its header; a changed snapshot returns ErrBusySnapshot wrapping ErrBusy. One writer appends complete page images, sector-neighbor protection and commit padding. WAL Sync precedes index/header publication. Every direct/deferred unlock failure poisons the session so an uncertain held lock cannot masquerade as retryable BUSY. Error-path source review is not physical-error execution evidence.

Checkpoint is nonblocking FULL-style: CKPT then WRITE freezes the committed head, reader marks limit backfill, and READ_LOCK(0) excludes readers relying on database pages. The generation salts are rechecked after acquiring READ_LOCK(0), retaining the upstream3.54.0 race fix. WAL Sync precedes main writes; main Sync precedes backfill publication. Partial progress may return BUSY. Reset requires full backfill and exclusive ownership of all WAL-reader slots; it changes salts while retaining WAL/SHM inodes. Shared mode never renames or unlinks either sidecar.

## Conservative storage assumptions

SectorSize remains an explicit caller assumption, default4096. WALPowersafeOverwrite defaults false. Native commits include unchanged extant sector neighbors and repeated committed padding. Existing imported tails may lack safe append padding or checkpoint neighbor coverage; return Unsupported unless a safe full checkpoint/reset is possible. The PSOW option is an explicit caller assertion that writes cannot damage bytes outside their range, not hardware detection or a guarantee. Ordinary live oracle tests set SQLite .filectrl psow0 and synchronousFULL.

Cold SHM reconstruction takes WRITE/CKPT/RECOVER and all reader locks and validates WAL bytes under bounded scanning. Clean commits, fully checksum-valid discarded ordinary-rollback suffixes, and normal old-salt reset suffixes are supported. Only pages through the last complete commit are indexed. A subsequent shorter native commit truncates obsolete bytes beyond its new committed end under WRITE before final Sync/publication; every existing reader's visible maximum precedes that end. Incomplete or checksum-invalid current-generation input remains refused; broader recovery is required future code.

Live hash storage is bounded to1,048,576 frames; WAL to1GiB. Cold reconstruction additionally inherits1,048,576 combined committed/pending entries and16,777,216 index-work units. Resource exhaustion requires a checkpoint or explicit error, never silent truncation. Native shrink, read-only filesystems/SHM, hardlinks/final sidecar symlinks and existing rollback journals remain unsupported.

## Ordinary evidence and remaining gates

Tests cover512/4096/65536-byte pages, pinned snapshots across commits, all-four-WAL-readmark saturation, slot0 database snapshots, second hash block, deferred-writer conflicts, pooled concurrent writes, partial/full checkpoint/reset, normal retained reopen and old-generation reuse. Live official SQLite processes retain read snapshots across native commits and hold/commit writer transactions while native readers continue. An official cache spill followed by ordinary ROLLBACK and normal close verifies committed-prefix reconstruction, shorter native overwrite and second normal cold reopen. Official integrity_check and forced index reads pass. Helpers use ordinary .quit/EOF/Wait, never process kill or injected I/O failure.

Full checks, scoped race runs and review are recorded in evidence/m3-shared-aggregate.txt and m3-shared-review.md. Restricted forced-exit/power-loss/failed-Sync/short-write/disk-full/permission/wrong-offset/interrupted-checkpoint gates remain BLOCKED/NOT_RUN. Physical hot rollback replay is compiled source only. Native non-Linux execution, independent security/legal review and long soak remain open. This ADR does not mark M3 or production acceptance complete.

References: pinned public SQLite src/wal.c, src/os_unix.c and release source ID in upstream.md; [WAL format](https://www.sqlite.org/fileformat2.html#the_write_ahead_log) and [WAL-index](https://www.sqlite.org/walformat.html).
