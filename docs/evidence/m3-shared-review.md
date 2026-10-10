# Shared WAL engineering review

Date: 2026-10-10. Scope: native Linux shared WAL index, pager/session/transaction integration, and public `WALShared` plumbing. This is an internal engineering review, not independent release, security, legal, or durability acceptance.

## Findings resolved

1. **Failed unlock could leave an untracked writer lock.** The stale-snapshot promotion path previously combined a failed writer unlock with `ErrBusySnapshot` without poisoning the connection. Because its transaction's writer flag remained false, rollback could skip that retained lock. Direct readmark/retry releases had the same missing poison handling. Every shared-pager release now uses `SharedSession.unlockLocked`; failed releases poison the session, and public error classification prioritizes poison over BUSY. Reviewed in `internal/pager/shared_session.go` and all direct release call sites. This error-path result is source review only: no lock or I/O failures were injected.

2. **Ordinary spilled rollback could prevent cold reopen.** An official SQLite transaction can write checksum-valid, same-salt uncommitted frames and then explicitly roll back. After every connection closes normally, transient-index reconstruction must retain only the committed prefix. `sharedWALPrefixAllowed` now accepts this valid pending suffix and ordinary old-generation salt tails. Reconstruction uses the scanner's committed `FrameCount`, `DBSize`, and `Checksum`; pending page versions never enter the published index. Incomplete and same-salt checksum-invalid cold tails remain refused.

3. **A shorter later commit could leave an invalid discarded suffix.** Overwriting only part of a rolled-back same-salt suffix changes its checksum chain. Leaving its remainder could make the next normal cold reopen fail the invalid-tail gate. `appendSharedLocked` now trims only bytes beyond the newly complete padded transaction, under WRITE and before final WAL Sync/header publication. Existing WAL snapshots end at or before the prior committed frame; reset requires excluding old WAL readers. The WAL inode is retained.

4. **Readmark saturation lacked direct coverage.** `TestSharedWALAllReadmarksPinned` now retains staggered snapshots in all four nonzero readmarks, starts a fifth snapshot sharing an older mark, and checks snapshots while reader-limited checkpoints advance as holders leave.

5. **Public close documentation was ambiguous.** Corrected `KeepWALOnClose` documentation: shared mode retains WAL/SHM inodes; exclusive mode has different deletion behavior.

## New ordinary-lifecycle regression

Added `internal/testkit/shared_wal_rollback_conformance_test.go`, `TestOracleSharedWALSpilledRollbackNormalReopen`:

- Creates an ordinary native database and a conservatively padded committed prefix.
- Uses the pinned official SQLite CLI with a small cache to UPDATE, spill, explicitly ROLLBACK, and close with `.quit`.
- Reads the untouched resulting WAL and asserts it actually contains a checksum-valid uncommitted suffix after a nonempty committed prefix.
- Closes every connection normally; native reopen returns the committed value, never the discarded value.
- Makes a shorter native commit, closes normally again, and verifies a second cold reopen, checkpoint, and official `integrity_check`.
- Leaves `WALPowersafeOverwrite` false. An initial fixture using an official-written prefix hit the existing conservative append-padding gate; using a native padded prefix isolates this regression without weakening that gate.

No database bytes were manually changed. No process was killed or deliberately interrupted.

## Commands and results

Working directory: `/workspace/scratch/5e06a74e5f65/durable-restart/gosqlite`.

The following environment and executable reproduce the exact focused invocations:

```sh
export GOTOOLCHAIN=local
export GOCACHE=/workspace/scratch/5e06a74e5f65/durable-restart/gocache-review
export SQLITE_ORACLE=/workspace/scratch/5e06a74e5f65/durable-restart/toolchains/sqlite-build-3540000/sqlite3
GO=../toolchains/go1.27.2/bin/go

"$GO" test ./internal/walindex ./internal/pager ./sqlite ./driver \
  -run 'Test(Header|FrameBlock|DMS|IndependentOFD|RealHash|CheckpointMetadata|ConcurrentHeader|NativeHash|SharedWAL)' -count=1

"$GO" test -race ./internal/walindex ./internal/pager ./sqlite ./driver \
  -run 'Test(Header|FrameBlock|DMS|IndependentOFD|RealHash|CheckpointMetadata|ConcurrentHeader|NativeHash|SharedWAL)' -count=3

"$GO" test -tags oracle ./internal/testkit \
  -run '^TestOracleSharedWALSpilledRollbackNormalReopen$' -count=1 -v

"$GO" test -race -tags oracle ./internal/testkit \
  -run '^TestOracleSharedWALSpilledRollbackNormalReopen$' -count=3

"$GO" vet ./internal/walindex ./internal/pager ./sqlite ./driver
```

All final invocations passed, exit 0. Focused ordinary tests: walindex 0.130s, pager 0.065s, sqlite 0.033s, driver 0.084s. Race ×3: walindex 5.617s, pager 2.562s, sqlite 1.569s, driver 2.784s. New oracle regression: 0.062s; race ×3 1.601s. Vet produced no diagnostics. Malformed-input cases selected by these commands are in-memory parser tests, not damaged-disk fixtures.

## Public source comparison

Reviewed pinned public SQLite 3.54.0 under `../toolchains/sqlite-src-3540000`; source identity is `2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3`. The oracle helper checks the exact version and source ID. No private SEE source was used.

- `src/wal.c`: `WalIndexHdr`/`WalCkptInfo` layout; `walIndexWriteHdr` and `walIndexTryHdr` publication order/checksum; `walTryBeginRead` readmark and header rechecks with `minFrame`; `walCheckpoint` latest-frame selection, backfill limits, READ_LOCK(0), and post-lock salt recheck; `walRestartHdr`/`walRestartLog` generation reuse; `sqlite3WalUndo` ordinary rollback without physical tail truncation; `walFrames` header synchronization, commit frames, and padding.
- `src/os_unix.c`: `unixLockSharedMemory` first-opener DMS protocol and three-byte truncation, region mapping, shared-memory lock ranges, and normal unmapping/close lifetime.
- `src/pager.c`: `pagerWalFrames` page ordering and change-counter integration; `pagerBeginReadTransaction` snapshot/cache handling; writer promotion through `sqlite3WalBeginWriteTransaction`; `sqlite3PagerCheckpoint`; `pagerOpenWal` SHARED-lock lifetime requirement.

No remaining normal-operation correctness blocker was found in the reviewed code after these fixes. Dual checked headers, atomic mapped-word/hash access, first-block/hash offsets, SHARED database descriptor lifetime, one local lease per connection, writer promotion, checkpoint snapshot bounds, conservative default storage assumptions, and no shared-WAL rename were inspected. Go race success is not proof of cross-process memory-ordering correctness; the public protocol comparison and ordinary live interoperability provide separate evidence.

## Scope and remaining risks

- This review and regression cover ordinary valid-data operation and normal close on the available Linux environment, plus in-memory parser cases. They are not a long soak or all-platform qualification.
- Imported append/checkpoint data must satisfy conservative sector protection unless the caller explicitly supplies the storage PSOW assertion. Unsupported cold tails remain a fail-closed boundary.
- Same-process interoperability with an independently linked POSIX-locking C SQLite library, hostile concurrent file replacement/truncation, and network-filesystem behavior were not qualified.
- Crash, forced-process-exit, physical-I/O-fault, short-write, Sync-error, disk-full, permission-fault, corrupted-disk, hot-journal replay, and manual-recovery harness gates remain **NOT_RUN**. No interrupted-durability result is implied by source review, ordinary truncation, normal reopen, race, or integrity-check success.
