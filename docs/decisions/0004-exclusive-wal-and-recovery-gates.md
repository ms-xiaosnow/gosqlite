# ADR 0004: Explicit exclusive WAL and source-only rollback recovery

Status: ordinary exclusive-WAL slice verified; full M3 code/acceptance remains incomplete and durability acceptance BLOCKED.

SQLite permits WAL without a shared-memory WAL index when the connection retains an EXCLUSIVE database lock. This project implements that explicit restricted mode first. It is sequential interoperability, not concurrent shared-WAL support. The existing rollback mode stays the default.

## Session and snapshot ownership

sqlite.Options.WALExclusive requests a read-write pager Session with one database descriptor and an EXCLUSIVE lock held through idle periods until Close. All reads, schema access and transactions lease that descriptor. At most one reader/transaction lease exists; overlap returns BUSY. A read-only transaction is a read lease inside the writable session. ReadOnly+WALExclusive is rejected. database/sql callers set MaxOpenConns(1). Native options are supplied through driver.NewConnector, not unrecognized DSN settings.

The mode-one to mode-two header transition uses the existing standard rollback-journal publication while retaining the same descriptor/lock. A mode-two database with no WAL is valid. OpenContext observes cancellation before physical publication; it cannot abandon a mode-switch commit after I/O starts.

## WAL format and normal publication

The native codec validates public SQLite version3007000 WAL headers and frames, both checksum word orders, salts, actual512..65536 page sizes, frame/page/byte/index quotas and last committed page images. WAL checksums are not cryptographic authentication. The logical database extent and overlaid page-one header come from the latest complete committed WAL transaction, not the stale main-file length.

The scanner distinguishes incomplete, invalid, uncommitted and salt-mismatching suffixes and returns an error with diagnostics. The session refuses all such tails without modifying them, including some ordinary SQLite reset leftovers. It does not silently claim recovery. WAL/main files must be correctly paired by pathname; SQLite WAL salts do not authenticate a database identity.

Before the first append to a reopened committed WAL, the session may normalize its append boundary: copy the validated bounded WAL to an owned same-directory temporary inode, append checksum-valid duplicate final commit frames through the sector boundary, Sync that complete equivalent WAL, atomically replace only the WAL pathname under retained EXCLUSIVE, Sync the directory, then adopt the descriptor/index. The acknowledged original stays unchanged until the replacement is durable. The main database is never replaced. This needs local-filesystem atomic rename plus file/directory Sync assumptions and temporary space up to the bounded WAL size; interrupted replacement remains NOT_RUN.

A new or imported header-only WAL header is separately synced before frame writes. Commit writes page frames and a nonzero database-size commit marker, uses complete repeated commit frames to protect the final storage sector, then Syncs before publishing the in-memory snapshot. Resource limits are checked before I/O and must remain reopenable under scanner limits. Once physical publication starts, cancellation does not interrupt it. An ambiguous marker/Sync outcome poisons the session and is never reported as success.

Checkpoint requires no active lease: Sync WAL; copy latest committed images into the main file and truncate to committed extent; Sync main; only then remove WAL and Sync its directory. EXCLUSIVE stays held throughout. Default Close performs this normal checkpoint. KeepWALOnClose instead closes normally while preserving already-synced committed frames. This option is not an interrupted-transaction simulation.

Existing -shm data is not used as an index and is invalidated/removed only while the exclusive database lock excludes cooperating clients. No partial implementation of shared-memory headers, read marks or mmap coordination is claimed. Official cooperating clients are BUSY until normal Close. File identity/extent/generation checks reject unexpected replacements.

## Rollback recovery boundary

internal/journal.ParseReplay constructs a bounded owned replay plan in memory. The conservative multi-segment subset accepts published finite-count segments with matching first-header geometry/extent, per-segment nonces, globally unique original pages and exact final consumption. Cumulative input/record/image-memory bounds apply before any plan escapes; page copies occur only after complete validation. The first Header is preserved unchanged and len(Pages) is cumulative. Super/master-journal trailers, repeated originals, geometry disagreement, no-sync counts and unpublished/ambiguous tails remain rejected. See [the full parser contract and source proof](../rollback-replay-memory.md). Partial/cold/ambiguous inputs never produce a partial plan.

internal/pager.RecoverRollback is explicit and is not called by Open, OpenTx, Session or SQL. Its source takes SHARED, checks another writer's RESERVED lock, escalates PENDING→EXCLUSIVE without RESERVED, rechecks identities, validates the complete plan/restored header, syncs journal, restores/truncates/syncs main, then removes journal and syncs directory. Physical replay is NOT_RUN. A doubly gated manual-only test source can be compiled/reviewed but must not execute in ordinary CI or this environment.

## Evidence boundaries

Permitted evidence includes memory-only codec/replay-plan tests, valid-data WAL commit/rollback/checkpoint/reopen/race and ordinary cross-process contention. Official .dbconfig no_ckpt_on_close1 with wal_autocheckpoint=0 and normal .quit intentionally retains a committed WAL without checkpoint; native KeepWALOnClose provides the symmetric normal-close path. These exercise WAL overlays without a crash/fault workflow.

Hot replay, failed/short/wrong-offset writes, Sync errors, disk-full/permission faults, forced process exit, interrupted checkpoint/rekey and power-loss behavior remain BLOCKED/NOT_RUN. Source review and normal operation do not establish them. M3 remains partial/conditional; later independent features can be developed under the user's revised sequencing without upgrading these gates to verified.

Sources: public SQLite3.54.0 src/wal.c, src/pager.c and src/os_unix.c (pinned in upstream.md); [WAL file format](https://www.sqlite.org/fileformat2.html#walformat), [exclusive WAL without shared memory](https://www.sqlite.org/wal.html#use_of_wal_without_shared_memory), [locking and rollback recovery](https://www.sqlite.org/lockingv3.html).
