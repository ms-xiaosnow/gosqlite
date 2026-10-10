# Manual verification and release-blocking gates

This file records checks that require a separately approved native environment or independent reviewer. No command here is evidence that it was executed.

## Current state

M0–M2 ordinary unit/build/vet/race, valid-data read/write/rollback and cross-process lock tests, memory-only fuzz and opt-in official-oracle checks are executable here. Database crash/fault/process-exit workflow execution is prohibited by the current platform boundary; it must not be renamed, obscured, moved to remote CI, or executed elsewhere as a workaround. Such test source may be reviewed and documented; its results remain BLOCKED / NOT_RUN until independently supplied authorized evidence is available.

M2 ordinary manual commands (Go1.27.2, Linux, CGO disabled except race instrumentation):

```sh
./scripts/check.sh
SQLITE_ORACLE=/absolute/path/to/verified/sqlite3 ./scripts/test-oracle.sh
CGO_ENABLED=0 go test ./sqlite ./driver ./internal/pager ./internal/btree -count=10
CGO_ENABLED=1 go test -race ./sqlite ./driver ./internal/pager ./internal/btree
```

These execute normal process completion and supported valid-data operations. The oracle's integrity_check and ordinary lock-contention checks are permitted and do not simulate power loss or injected I/O faults. TestTxStatementPanicAndCancellationUndo only restores an in-memory statement callback and recovers in the same goroutine before commit; it never crashes a process or interrupts disk publication.

Pending restricted gates: interrupted journal creation/publication/database writes/unlink/directory-sync; short-write, Sync-error, disk-full, permissions and wrong-offset injection; hot-journal recovery after forced exit; WAL crash/checkpoint recovery. Status: BLOCKED / NOT_RUN. M2 contains defensive source branches for ambiguous commit errors but no claim that those branches were executed. Native exclusive WAL normal-success paths are implemented; native rollback recovery remains source-only and is never invoked by normal Open. Shared concurrent WAL ordinary paths are implemented and tested; ambiguous cold tails and broader journal recovery remain implementation gaps. Do not run an invented placeholder command; a later source-only harness will document precise independently executable commands once it exists.

One pre-final aggregate oracle attempt FAILED because a legacy M1 read-only test used the new writable-default driver and deleted rows from a generated fixture. The test was corrected to an explicit mode=ro DSN and a read-only transaction assertion; the exact fixture was restored from the verified M1 commit. Final reruns and source hashes are recorded separately in M2 evidence. This was a real test failure, not passing evidence.

## Release requirements

- Linux/amd64, Linux/arm64, Darwin/arm64 and Windows/amd64 native file-lock/durability verification, distinct from cross-compilation.
- Long-running concurrent soak with fixed workload/hardware/seed and real recorded duration.
- Independent data-safety, cryptography/threat-model, dependency/license and API reviews.
- Data-format upgrade/rollback and preservation evidence.

All remain NOT_RUN and block stable/production acceptance. Ordinary Linux cross-process contention tests and multi-target cross-builds do not replace these gates.


## M3 explicit recovery harness: source only

The harness is internal/pager/recovery_manual_test.go with both a manual_recovery build tag and an explicit environment gate. It is excluded from scripts/check.sh, oracle suites and CI. The following is a compilation-only check; compiling does not execute the binary:

```sh
CGO_ENABLED=0 go test -c -tags manual_recovery -o /tmp/gosqlite-manual-recovery.test ./internal/pager
```

The exact physical-recovery execution command below is documented for separately authorized independent evidence only. Status: BLOCKED / NOT_RUN. Do not execute it here or route it elsewhere as a workaround:

```sh
GOSQLITE_MANUAL_RECOVERY=independently_authorized /tmp/gosqlite-manual-recovery.test -test.run '^TestManualRollbackRecovery$' -test.v
```

This source covers deterministic replay/refusal cases, not genuine power loss, process interruption, short writes, failed Sync, ENOSPC, wrong-offset/torn writes or interrupted checkpoint/WAL replacement/rekey. Those additional gates remain unimplemented or unexecuted; no command is invented for them.

M3 ordinary tests are valid-data transactions/checkpoints and normal connection closure. Official .dbconfig no_ckpt_on_close1 and native KeepWALOnClose intentionally retain successfully committed WAL after a normal close; they are not used to simulate interruption. TestOracleExclusiveWALNormalInterop and TestOracleImportedWALCheckpointWithoutNativeWrite run with the pinned SQLITE_ORACLE through scripts/test-oracle.sh. Memory-only WAL/replay parsing may test malformed byte slices without creating corrupted disk files. Native-OS, long soak, cryptographic and independent legal/security gates remain open.

## M3 shared WAL ordinary verification

These are permitted normal-operation checks, recorded in m3-shared-aggregate.txt and m3-shared-review.md:

```sh
./scripts/check.sh
SQLITE_ORACLE=/absolute/path/to/verified/sqlite3 ./scripts/test-oracle.sh
CGO_ENABLED=1 SQLITE_ORACLE=/absolute/path/to/verified/sqlite3 go test -race -tags oracle ./internal/testkit
CGO_ENABLED=0 go test ./driver ./cmd/gosqlite -run 'TestSharedWAL|TestCLISharedWAL' -count=5
CGO_ENABLED=1 go test -race ./sqlite ./driver ./cmd/gosqlite -run 'TestSharedWAL|TestCLISharedWAL' -count=3
```

The live oracle helper starts a cooperating official CLI, exchanges bounded marker-delimited SQL, and closes with ordinary .quit/EOF plus Wait. It does not kill, signal, timeout-terminate or crash a process. The spilled UPDATE→ROLLBACK regression uses valid SQL and normal closure to check a checksum-valid discarded suffix, followed by normal native commit/reopen/checkpoint. It is not hot-journal replay or fault simulation. Shared protocol error branches and ambiguous cold-tail recovery are not qualified by these tests.
