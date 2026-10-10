# Manual verification and release-blocking gates

Test-scope correction: historical full suites from M2 through 7addc493 included two namespace-mutation tests. Their PASS outputs remain factual, but are not clean ordinary-scope evidence. See [the audit and gating correction](test-scope-audit.md).

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

## M4 ordinary index verification

The ordinary suite now includes real persistent index build, delta maintenance, full posting/frequency reconciliation, statement/transaction rollback, normal checkpoint/reopen, exact indexed-versus-NOT INDEXED comparison, external SQLite DML invalidation, missing guard repair and ordinary VACUUM. These are valid-data lifecycle operations, not physical fault simulations. The shared oracle helper no longer has a CommandContext auto-kill hook; disk processes finish through normal SQL/EOF/Wait.

```sh
./scripts/check.sh
SQLITE_ORACLE=/absolute/path/to/verified/sqlite3 ./scripts/test-oracle.sh
CGO_ENABLED=1 SQLITE_ORACLE=/absolute/path/to/verified/sqlite3 go test -race -tags oracle ./internal/testkit -run 'TestOraclePersistentGramsInterop/page4096|TestOracleGramsVacuumEpoch|TestOracleM4Review'
```

Actual commands/results belong in evidence/m4-aggregate.txt and m4-review.md. No checkpoint or ordinary rollback result qualifies interrupted index publication/recovery. Those gates remain BLOCKED/NOT_RUN, and full M4 acceptance is conditional on them.

## M5 ordinary similarity verification

```sh
./scripts/check.sh
SQLITE_ORACLE=/absolute/path/to/verified/sqlite3 ./scripts/test-oracle.sh
CGO_ENABLED=1 SQLITE_ORACLE=/absolute/path/to/verified/sqlite3 go test -race -tags oracle ./internal/testkit -run TestOracleSimilarityDirtyIndexFallback -count=1
CGO_ENABLED=0 go test ./sqlite ./driver ./internal/sql ./extension/similarity -run Similarity -count=1
CGO_ENABLED=0 go run ./examples/similarity /new/disposable/path.db
```

Component native indexed-versus-forced-scan, independent reference, SQL/driver, rollback/reopen/config rebuild and ordinary external SQLite invalidation/checkpoint tests pass. The final live-oracle race regression passed15.417s on2026-10-10. Exact commands/output for the final aggregate and scoped review are in evidence/m5-aggregate.txt and m5-review.md. PostgreSQL/pgvector is not executed: vector prerequisite tests use retained public expected outputs and independent math only. Those M5/M6 integration gaps were open at that checkpoint and were subsequently implemented, as recorded below. Restricted crash/fault/recovery, native-OS, independent security/legal and soak evidence remains BLOCKED/NOT_RUN and is not implied by these commands.

## M6 ordinary typed-vector and hybrid verification

```sh
./scripts/check.sh
SQLITE_ORACLE=/absolute/path/to/verified/sqlite3 ./scripts/test-oracle.sh
CGO_ENABLED=1 SQLITE_ORACLE=/absolute/path/to/verified/sqlite3 go test -race -tags oracle ./internal/testkit -run 'TestOracleDenseVectorStorageInterop|TestOracleCreatedDenseVectorDeclaration|TestOracleVectorTypedReadErrorParity' -count=1
CGO_ENABLED=1 go test -race ./driver -run 'TestDriverDenseVectorBindingsAndTransactions|TestDriverVectorQuotaErrorClassification' -count=3
CGO_ENABLED=0 go run ./examples/vector /new/disposable/hybrid.db
```

Tests exercise valid SQLite files, normal transactions and type validation. Official SQL can legally put a wrong-dimension TEXT in VECTOR(n), which SQLite treats as affinity only; testing native rejection is ordinary SQL compatibility, not corruption of SQLite page bytes or physical recovery. Invalid vector mutation inputs fail before publication. No physical fault/tamper/recovery workflow is executed. Complete commands/results are in evidence/m6-driver-oracle-example.txt, m6-review.md and m6-aggregate.txt.

The M5 Library-restored test session lost executor transport before its final result was collected; that uncollected result was not counted. All files/toolchain/bundle remained present. A retained-log rerun completed exit0 with ordinary gates and pinned oracle passing; see evidence/m5-library-restore-recheck.txt. CI also passed for its exact checkpoint. This infrastructure interruption is not a database crash-test result and does not qualify recovery.

## M7 first HNSW ordinary verification

```sh
./scripts/check.sh
SQLITE_ORACLE=/absolute/path/to/verified/sqlite3 ./scripts/test-oracle.sh
CGO_ENABLED=1 SQLITE_ORACLE=/absolute/path/to/verified/sqlite3 go test -race -tags oracle ./internal/testkit -run TestOracleHNSW -count=1
CGO_ENABLED=0 go test ./extension/vector/ann ./sqlite -run 'HNSW|ANN' -count=1
CGO_ENABLED=0 go run ./examples/ann /new/disposable/ann.db
CGO_ENABLED=0 go run ./benchmarks/ann -rows 2048 -dims 12 -queries 48 -seed 20261010
```

The six-way oracle matrix and pre-ANN grams fixture upgrade use valid SQL,
normal transactions/close/reopen/checkpoint and official integrity_check. No
physical crash/tamper/recovery is involved. Persistent benchmark data is
synthetic and seeded; metrics and slower observed latency are preserved in
evidence/m7-hnsw-persistent-benchmark.json. The source fixture is copied before
write and checked by SHA256. ANN format/source-negative checks use memory models
or normal legal SQL, not damaged disk images. That checkpoint still required IVFFlat; the later section records its implementation.
Physical durability and independent release gates remain BLOCKED/NOT_RUN.

M6 Library restore completed all ordinary checks and pinned oracle, exit0 at
2026-10-10T11:43:34Z; evidence/m6-library-restore-recheck.txt. Remote CI passed
for the exact M6 checkpoint. These results do not qualify physical recovery.

## M8 memory-only prerequisite verification

```sh
CGO_ENABLED=0 go test -count=1 ./encryption
CGO_ENABLED=1 go test -race -count=1 ./encryption
CGO_ENABLED=0 go vet ./encryption
CGO_ENABLED=0 go test -tags purego -count=1 ./encryption
CGO_ENABLED=0 go test -count=1 golang.org/x/crypto/argon2 -run '^TestArgon2$'
CGO_ENABLED=0 go test ./encryption -run '^$' -fuzz '^FuzzHeaderCodec$' -fuzztime=10s -parallel=2
CGO_ENABLED=0 go test ./encryption -run '^$' -fuzz '^FuzzRecordFraming$' -fuzztime=10s -parallel=2
CGO_ENABLED=0 go test ./encryption -run '^$' -fuzz '^FuzzAuthenticatedRecord$' -fuzztime=10s -parallel=2
```

All are actual executed permitted memory/standard-vector gates; final command
results/hashes in evidence/m8-primitives.txt. Fuzz campaigns total1400866
executions. Password package tests use production minimum64MiB/3passes; the
separate dependency command runs its own RFC standard vectors. No database
file I/O or physical tamper/crash/recovery workflow exists in the tested package.
At that primitive checkpoint, encrypted storage/rekey/backup integration was
unimplemented; the later snapshot section records it. These primitive results
are not an independent cryptographic audit or M8 acceptance.

## M7 IVFFlat and M8 experimental snapshot verification

Commands run on ordinary valid files and correct credentials:

```sh
./scripts/check.sh
SQLITE_ORACLE=/absolute/path/to/verified/sqlite3 ./scripts/test-oracle.sh
CGO_ENABLED=0 go test ./extension/vector/ann ./sqlite -run 'IVF|ANN' -count=1
CGO_ENABLED=1 go test -race ./sqlite ./driver -run 'IVF|Encrypt' -count=1
CGO_ENABLED=1 SQLITE_ORACLE=/absolute/path/to/verified/sqlite3 go test -race -tags oracle ./internal/testkit -run 'TestOracleIVF|TestOracleEncrypted' -count=1
CGO_ENABLED=0 go run ./examples/ivfflat
CGO_ENABLED=0 go run ./examples/encrypted
CGO_ENABLED=0 go run ./benchmarks/ann -method ivfflat -rows 2048 -dims 12 -queries 48 -seed 20261010 -lists 100 -probes 1
```

This section is the command inventory; actual final result records are
m7-ivf-m8-snapshot-aggregate.txt and separate focused/example evidence. It must
not be read as passing evidence for a command absent from those logs. Probes10
and50 repeat the last benchmark with only that argument changed. Raw measurements
are preserved; the first probes1 run contains a descriptive legacy 'graph' note,
not a different measured algorithm. Its counters/method identify actual IVFFlat.

The encrypted lifecycle uses intact files and correct keys only. Rekey, backups,
restore, explicit plaintext export/import and ordinary quota refusal are normal
operations. Memory-only malformed manifest/framing checks do not modify physical
files. Generated ciphertext is read-only scanned for a known test marker; marker
absence is not proof of absence of transient plaintext writes. Source review
separately checks that encrypted dispatch bypasses plaintext publication.

Initial encrypted extension restore failed at all three page sizes because the
standalone catalog validator had zero default connection quotas. The shared
normalization fix and unchanged test rerun are retained separately in
m8-snapshot-initial-default-limit-failure.txt and m8-snapshot-oracle-focused.txt.

At this historical snapshot checkpoint, encrypted WAL and concurrent snapshots
remained required code. The later dirty-WAL increment below implements segments;
simultaneous encrypted writer/readers remain required code, not merely unrun tests. Wrong-key or
tampered-on-disk checks, interrupted root/rekey publication, crash/forced-exit,
physical I/O faults and hot recovery remain BLOCKED/NOT_RUN. Do not run them here
or move them elsewhere to bypass restrictions. Full M8 and production acceptance
also require independent cryptographic/security review and native platform proof.

M7 HNSW Library restore completed full ordinary unit/vet/build/race and pinned
oracle exit0 at2026-10-10T12:12:46Z; exact-SHA CI passed. See
m7-library-restore-recheck.txt and checkpoints.json. This does not qualify recovery.

## Ordinary B-tree, encrypted dirty-WAL and M9 prerequisite verification

Only intact-file/correct-key ordinary operations and bounded pure-memory negative
cases are included. Commands and exact outcomes are retained per component and
aggregate under evidence/; an inventory entry is not an assertion that an absent
result passed.

```sh
CGO_ENABLED=0 go test ./... -count=1
CGO_ENABLED=0 go vet ./...
CGO_ENABLED=0 go build -trimpath ./...
CGO_ENABLED=1 go test -race ./... -count=1
CGO_ENABLED=0 SQLITE_ORACLE=/absolute/path/to/verified/sqlite3 go test -tags oracle ./... -count=1
CGO_ENABLED=1 SQLITE_ORACLE=/absolute/path/to/verified/sqlite3 go test -race -tags oracle ./internal/testkit -run '^TestOracleEncryptedWALCompleteHead$' -count=1
CGO_ENABLED=0 go run ./examples/encrypted-wal
```

The WAL test explicitly exports plaintext for the official SQLite oracle; SQLite
never reads or implements the encrypted container. Retained-head normal reopen,
checkpoint, rekey and complete-head backup/restore/import/export are successful
ordinary workflows. Memory-only root/segment mutations do not alter disk files.
Neither sparse publication nor marker absence qualifies physical crash safety.

The migration inspector reads local synthetic fixtures and public reference
source. It does not connect to PostgreSQL or execute the catalog-export SQL.
Extended half/bit/sparse tests are pure-memory math/codec checks, not SQL/ANN
integration. Their published upstream outputs are not live pgvector execution.

Previous snapshot Library restore passed complete ordinary gates and pinned oracle
exit0 at2026-10-10T12:55:04Z; CI38052742168 succeeded for exact bd6b19c9e88b7729fa0db953b6e280cd15ced239.
See m8-library-restore-recheck.txt. Restricted physical recovery/security gates
remain BLOCKED/NOT_RUN; simultaneous encrypted writer/readers and other missing
code remain implementation work.

## S1, BIT and native ownership ordinary increments

The default and oracle scripts first run the source-scope guard; manual tags are
refused. New component records document only reviewed normal transactions,
intact files, normal cooperating processes and memory-only negative cases:

- evidence/s1-transactions-{components,race,oracle}.txt: actual lazy snapshot,
  savepoint, undo quota, ordinary BUSY, extension restoration and driver checks.
- evidence/m9-bit-sql-{components,oracle,review,example}.txt: bit SQL/persistence,
  exact ranking and dense-regression checks, including legal SQLite affinity.
- evidence/native-ownership-components.txt: role-specific managed descriptors,
  normal Linux byte locks/mappings and official cooperative-lock behavior.

They do not qualify failed native locking/unmap/close, hostile namespace changes,
interrupted publication, physical fault/recovery or native Darwin/Windows runtime.
Darwin/Windows production backends are still required implementation work.
Combined-tree aggregate and restored/remote gates are reported separately.

## M9 B final isolated converter verification

The converter's final source was exported into a separate complete Git tree,
8e4808e09fcd45666bb2efb60d96bbf0033eeb91, excluding later half/native-platform
work. With the pinned offline Go environment and GOFLAGS=-buildvcs=false for the
standalone archive, these actual commands passed after the source-scope guard:

```sh
go run ./internal/testscope
CGO_ENABLED=0 go test ./migration/postgres ./cmd/gosqlite ./examples/migrate-pgembedding -count=1
CGO_ENABLED=0 go vet ./migration/postgres ./cmd/gosqlite ./examples/migrate-pgembedding
CGO_ENABLED=1 go test -race ./migration/postgres ./cmd/gosqlite ./examples/migrate-pgembedding -count=1
CGO_ENABLED=0 go run ./examples/migrate-pgembedding
```

Production builds and Linux amd64/arm64, Darwin arm64, Windows amd64 cross-builds
also passed; no non-Linux runtime was executed. The example uses a new synthetic
local database, valid SQL and normal close/reopen. No supplied SQL is executed
by the converter CLI, and no PostgreSQL server was used. Exact outputs, source
identity and unchanged-tree checks are in evidence/m9-converter-isolated-final.txt.

## M9 F HALFVEC ordinary verification

Focused commands and actual PASS results are in m9-half-sql-components.txt and
m9-half-sql-review.txt. Guarded CGO0 repeated tests, separate CGO1 races, pinned
public source/math fixtures and ordinary official SQLite interop use intact files
and legal SQL. External numeric defaults/type rejection are SQL compatibility
cases, not physical damage. The full aggregate uses an immutable archive tree
98d857b4e07b755fa9bd67346905785135fba2ef and GOFLAGS=-buildvcs=false because
the archive has no Git metadata; -timeout=30m is an ordinary test-run ceiling.
No native target runtime, physical fault/recovery or cryptographic audit is implied.

## Native-platform and shared encrypted ordinary verification

Frozen source tree ba85a097c9c10cf96e0bc9049d796887cfd3ba80 excludes later sparse,
importer, SQL-expression and encrypted-platform edits. Linux commands use pinned
Go1.27.2, offline modules and the mandatory testscope guard. Component/review logs
record named CGO0 and separate CGO1-race checks. Only correct-key intact files,
cooperating ordinary API interleavings and memory-only invalid-state models run.

Native Darwin/Windows build and test-binary compilation are compile-only. They
must not be reported as runtime PASS. Default Windows cross-vet returns exit1
for the documented mmap uintptr/unsafe.Pointer conversion. Linux and Darwin
scoped vet pass. No checker suppression is an acceptance result.

Shared encrypted tests cover lazy transactions/savepoints, stale promotion,
independent readers/writer, copied selection stability across ordinary concurrent
publication/checkpoint, configured quotas, normal cancellation, quiescent rekey,
explicit upgrades, restore/import/export and close BUSY. No fault/crash/forced
exit, physical tamper/wrong-key/selected-file removal, active namespace mutation,
failed native IO/Sync/lock/unmap or interrupted publication/rekey is executed.
Missing non-Linux encrypted code and failed-open retirement retry remain code
work; physical, independent native/security/crypto/legal/soak gates stay unverified.

## M9 G SPARSEVEC ordinary verification

Only intact ordinary DDL/DML/read/write/rollback/snapshot/reopen and pure-memory
invalid-codec/budget models are selected. The official test oracle reads canonical
GSS overflow values and creates legal TEXT/default values at three page sizes.
Ten independent review regressions plus exact pinned source expectations cover
strict identity, bounded explicit conversion, NULL/zero policies and billion-
dimensional support-sized work. Rejection before decoding is a memory-allocation
precedence check, not physical fault injection. See m9-sparse-sql-components.txt
and m9-sparse-review.txt; future expression/import/platform work is excluded by
immutable lead snapshot57b02e0fa1f99f4c6a4f470c7741624cce1ba977. No full project,
sparse ANN, native target-runtime or independent acceptance follows from these
focused checks.

## M9 C import ordinary verification

Logs distinguish full focused source tree964ecebd1c9b3c479c440f815b2435394ff03326
from final text-ownership delta tree3f83ee9453319a71b0db3c78a098a7855a2986ff.
Both archives were immutable and used GOFLAGS=-buildvcs=false solely because they
lack Git metadata. Named importer/CLI CGO0 and separate CGO1 race, scoped vet/build,
old inspector/converter regressions and synthetic example passed; final delta
adds CSV retained-text heap checks and reruns importer/parser/CLI validation.
The before-fix memory-retention failure is retained, not discarded.

A deliberately committed stop is a normal API return and normal connection close.
Resume reopens intact correctly committed data; no crash, kill, failed native I/O,
namespace race/replacement, corrupt files or interrupted publication was used.
Semantic cosine-zero/index-budget failures preserve pending state and are not
physical fault injection. Tests write only synthetic sources and new databases.
Reports distinguish supplied-export fidelity, exact-distance reference checks,
actual ANN recall measurements, and acceptance gates that remain NOT_RUN.

## S2 expression prerequisite verification

Final source snapshot db470d2ec208a690202253db840620b219a6abdc isolates existing
published core plus reviewed expression changes. Tests compare pure scalar and
ordinary intact-file SQLite3.54 behavior, including original failing IN error,
alias identity and CAST keyword cases. Official CLI is a test-only oracle; no
production delegation exists. Guarded ordinary suites exclude manual namespace/
recovery tags. Logs separately record owner cases, independent review and later
lead full aggregate; their totals are not independent certifications or full
SQLite coverage. No physical/native error injection or external DB migration is
executed. Broad relational S2 and other unsupported SQL remain implementation work.

## M9 D offline transition verification

The exact commands in m9-transition-ordinary.md and the two independent-review
reports exercise bounded in-memory generation/decoding and ordinary synthetic
local review-artifact output. All mutation phases remain line-commented; no
PostgreSQL connection or generated SQL executes. Core/CLI unit and separate race
checks, production vet/build, synthetic CLI output and cross-builds are recorded
for immutable archives. Windows/Darwin cross-builds are not native execution.

For an ordinary offline CLI check from the repository root:

```sh
go run ./internal/testscope
go test ./migration/postgres ./cmd/gosqlite ./examples/migration-transition
go run ./cmd/gosqlite migrate pgembedding transition \
  --catalog testdata/pgembedding-0.3.6/catalog.json \
  --options examples/migration-transition/options.json
```

Use the pinned toolchain/environment documented above. Source-faithful actual
PostgreSQL version/extension/dependency/privilege validation and user-reviewed
cutover/rollback remain pending. Fault/interrupted-publication testing remains
manual pending and is not moved to another environment or CI.
