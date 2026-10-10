# M3 component evidence and review boundaries

Date: 2026-10-10 UTC. Go 1.27.2 / official SQLite 3.54.0. Aggregate gate and durable checkpoint are recorded separately when complete.

## WAL codec: memory-only

internal/wal implements native header/frame encoding/decoding and bounded ReaderAt scanning. Component checks passed:

- CGO_ENABLED=0 go test -cover ./internal/wal: 96.9% statements
- CGO_ENABLED=0 go vet ./internal/wal
- CGO_ENABLED=1 go test -race ./internal/wal
- FuzzScan: 30 seconds, 4,056 executions; final 10-second run, 164,082 executions
- FuzzFormatRoundTrip: 30 seconds, 164,047 executions

These campaigns use only in-memory byte slices/readers. Golden checks cover both checksum byteorders and page sizes through 65,536; every header-bit/frame-byte mutation, truncation boundaries, random multi-transaction overlays, duplicate pages, suffix classification, cancellation and concurrent scanners. No malformed WAL file is written to disk.

Review identified potentially quadratic map sweeps after adversarial shrinking commit sizes. Added max-indexed-page shortcut and MaxIndexWork, a fixed charged-work budget. Snapshot records the committed IndexWork so pager append can keep its output reopenable. Memory adversarial and prefix-rescan tests retained. The pager also matches scanner committed+pending index-entry accounting.

## Rollback recovery: source only

Added internal/journal/replay.go and tests; internal/pager/recovery.go, platform lock probes and doubly gated recovery_manual_test.go. Physical recovery is not reached through normal Open/OpenTx/SQL.

Passed: pure-Go journal tests, pager compile with -run '^$', journal/pager vet, race journal tests, byte-only replay-plan fuzz (17,746 executions), Windows/amd64 pager compilation and final manual-tagged test-binary compilation. Two intermediate compile attempts overlapped incomplete WAL edits and failed with undefined identifiers; final compilation passed. No physical-recovery test was executed.

Independent static review verified hot-candidate gating, RESERVED probe without acquiring RESERVED, PENDING/EXCLUSIVE escalation, stable identities, complete bounded plan before mutation, shortened-extent original-image requirement, originalPages=0 restrictions, cancellation before physical replay only, journal preservation before unlink and uncertainty after unlink. This is not a physical correctness or durability certificate.

## Exclusive WAL: ordinary success

Initial focused native API/driver/CLI tests passed at 512/4096/65,536 bytes. After sector-neighbor changes, those tests also passed with race instrumentation three times. Ordinary oracle tests passed native/official roundtrip at all sizes, main-file SHA unchanged during native WAL commits, integrity_check/forced secondary-index reads, official writeback/checkpoint and native reopen.

TestOracleImportedWALCheckpointWithoutNativeWrite starts with a normally checkpointed 512-byte main file and 400 long rows, performs four distant ordinary official WAL updates, then closes normally with no_ckpt_on_close. Native reads and checkpoints with a 16-page transaction budget before any native DML/DDL. All rows/index reads and official integrity_check match afterward; this exercises protection batching for imported sector neighbors.

Source review found and corrected:

1. Missing co-resident small-page images during native commit/imported checkpoint under a larger configured sector assumption.
2. Raw main-header geometry could disagree with WAL overlay, making fallback offsets unsafe.
3. Final symlink replacement could pass a following Stat check; now Lstat requires regular identity.
4. Index-entry/work accounting could admit an append that its own scanner would refuse after reopen.
5. First append to an imported unpadded WAL could overwrite the sector containing its only final commit marker. A bounded WAL-only copy/pad/Sync/atomic-replace/directory-Sync normalization path was implemented and independently source-reviewed. All-size ordinary tests verify equivalent committed snapshots and exact scanner checksum/index/work state; interrupted replacement remains NOT_RUN.

No forced exit, interrupted commit/checkpoint, failed Sync, short/wrong-offset write, disk-full/permission fault, corrupted-disk or physical hot-recovery workflow has run. M3's durability acceptance remains BLOCKED/NOT_RUN. Full shared-memory WAL concurrency is unsupported.


Final lead aggregate passed after all review fixes: CGO0 tests/vet/build, separate CGO1 race, complete pinned official-oracle suite, four CGO0 target builds, manual-recovery compilation only, all six fixture manifest hashes and unchanged goal hash. Exact output: m3-exclusive-aggregate.txt, 2026-10-10T10:14:53Z–10:15:59Z. This verifies the documented ordinary exclusive slice, not full M3. Shared-memory single-writer/multiple-reader code and live concurrent cross-process interoperability remain required unfinished implementation work.
