# Manual verification and release-blocking gates

## 2026-10-11 中文入口与当前边界

请先阅读[中文手工验收手册](manual-verification-zh.md)、
[当前验收矩阵](acceptance-status.md)与[中断说明](interruption-20261010.md)。
下文保留历史命令和当时范围，不能把旧版本状态或旧 PASS 当成最新源码的
验收结论。2026-10-10 被中止的 ANN V4、新 41 目标 fuzz 及未知触发链路
继续暂停，不通过改名、变更输入、其他执行环境或 CI 重试。

本批已将 Windows/Darwin 两项 raw namespace 测试完整移到精确 manual tag/env
门，保留原断言和当前 P0 恢复 guard；原生执行仍 BLOCKED / NOT_RUN。
详见[中文第 19 节](manual-verification-zh.md#19-os-raw-namespace-门控与独立原生验收2026-10-11)。门控不授权默认 OS 全套、新 runner 或计费读取；
明确未实现的 harness 或生产能力仍列代码缺口，不伪装成只差手工运行。

Test-scope correction: historical full suites from M2 through 7addc493 included two namespace-mutation tests. Their PASS outputs remain factual, but are not clean ordinary-scope evidence. See [the audit and gating correction](test-scope-audit.md).

This file records checks that require a separately approved native environment or independent reviewer. No command here is evidence that it was executed.

## 2026-10-11 public Open recovery update

The bounded experimental public source is now connected through nil-default
`Options.RollbackRecovery`, Open/OpenContext and configured driver.NewConnector.
It permits only Linux writable plaintext rollback recovery, at most one attempt
and one fresh no-create probe after verified reader retirement. Nil still refuses
existing journals with ErrRecoveryRequired / CodeIOErr. Journal diagnostic text
and private wrapping changed; byte-identical errors are not claimed.

Independent candidate tree `75c074548e8b5208beb06e6ead314d835cd16f4f` passed the
11 exact descriptor-free tests in CGO0 and the identical race selection, guards,
scoped vet, full CGO0 build and dependency/identity checks. This is source/model/
compile evidence only. The later c150 composition's complete ordinary/oracle/race,
exact-commit restore and new CI remain pending at this documentation checkpoint.

Actual RecoverRollback, the production bridge and recovery-enabled real
Open/OpenContext/Connect remain BLOCKED / NOT_RUN. The public physical harness
is IMPLEMENTATION_MISSING: the existing internal TestManualRollbackRecovery does
not exercise that bridge. Its commands below remain the same restricted internal
harness, not a newly working public acceptance command. Default-on recovery,
broader journals, WAL-tail publication and other native platforms remain gaps.
See [API, source/model commands and evidence](rollback-recovery-open.md) and
[Chinese manual section 18](manual-verification-zh.md) for exact boundaries.
Older milestone statements below retain their original source/time identities.

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

Pending restricted gates: interrupted journal creation/publication/database writes/unlink/directory-sync; short-write, Sync-error, disk-full, permissions and wrong-offset injection; hot-journal recovery after forced exit; WAL crash/checkpoint recovery. Status: BLOCKED / NOT_RUN. M2 contains defensive source branches for ambiguous commit errors but no claim that those branches were executed. Native exclusive WAL normal-success paths are implemented; physical rollback recovery remains unqualified. Nil-policy Open never invokes it. The explicit experimental Linux writable-plaintext opt-in now has a production source call path, but that bridge and recovery-enabled real Open have not been executed. Shared concurrent WAL ordinary paths are implemented and tested; ambiguous cold tails and broader journal recovery remain implementation gaps. Do not run an invented placeholder command; a later source-only harness will document precise independently executable commands once it exists.

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

## Windows encrypted/public metadata source qualification

Component and independent-review logs distinguish pure-memory lock/lifetime/
metadata/error laws, Linux intact correct-key operations, and target-only
Windows compile checks. The combined isolated-source suite runs VFS/WAL-index/
encstore/pager unit/race, ordinary public canonical paths/encrypted/shared DB
operations, full vet/build, pinned transaction oracle and cross-builds. Native
Windows binary tests are not executed. The Windows encstore vet pass is scoped;
the plain VFS default vet warning remains disclosed, never suppressed as clean.

No native failed lock/unmap/close/Sync, wrong key, physical tamper, namespace
race/retarget, crash/exit/recovery or interrupted publication/rekey is executed.
Those commands remain manual pending in their existing sections. Public code
coverage, source review and cross-compilation do not replace platform runtime
and independent security/legal verification.

Windows mapping correction uses genuine production state-machine code with
synthetic uintptr identities and memory callbacks only. Repeated handle-close
uncertainty, successful object-close followed by represented view-unmap retry,
failed view-creation cleanup and idempotence are memory laws; no Windows
CloseHandle/UnmapViewOfFile failure is induced. Both corrected native callsites
are compiled and source-reviewed, with unchanged lock/identity/granularity
contracts. A new native runtime or physical gate is not inferred from these laws.

## Darwin coordination and bounded failed-open cleanup

Earlier statements above about missing Darwin encrypted code and failed-open
cleanup describe their historical checkpoints. The native Darwin backend and
64-group failed-open retirement source are now implemented. See
native-encrypted-darwin.md and shared-open-cleanup.md for exact ordinary commands,
compile-only scope and manual gates. The later Darwin poison delta retains only
the first error graph; 10,000 pure-memory retries verify bounded retained state.
Native Darwin/Windows runtime remains NOT_RUN. Restricted failure cases remain
BLOCKED and must not be run here or relocated. Successful-session Close, public
DB.Close retry, driver disposal, Windows shared rekey and Darwin Unicode
inventory are still actual required code work, separate from these manual gates.

## Typed ANN ordinary verification boundary

See typed-ann.md, typed-ann-independent-review.txt and m9-typed-ann-* evidence.
Frozen27de0d52… full ordinary unit/race/oracle and cross-build PASS is source-tree
evidence. Dense v1 cross-version fixtures retain original byte/config/layout
contracts. Approximate recall uses actual graph/list work and fixture-specific
measurements. No physical tamper/crash workflow or native Darwin/Windows runtime
was executed. All earlier manual prohibitions and independent M10 gates remain.

## Native shared successful-close source qualification

The successful native WALShared close delta is separate from the64 failed-open
groups. Its focused isolated source/memory/ordinary/race/vet/build and native
compile-only commands are preserved in shared-close-cleanup-isolated.txt and
shared-close-cleanup-independent-review.txt. New negative laws use descriptor-
free managed owners/fake sessions; real database/sql disposal models use memory-
only connections. Ordinary native tests use intact databases and normal locks.
No injected native close/unmap/lock/Sync failure or physical recovery ran. See
shared-close-cleanup.md for the1024 lifetime bound and exact remaining code gaps.

## Encrypted retirement and complete ordinary oracle correction

The reviewed encrypted ownership increment has descriptor-free admission,
retirement/dependency, bounded error-history and public cleanup-token laws,
plus intact correct-key Linux lifecycle coverage. It does not execute native
close/unmap/lock/Sync failures. Native Darwin/Windows target binaries are compiled
only. The exact source and commands are in encrypted-cleanup.md and its
independent evidence; remaining plaintext Reader/Tx/exclusive-WAL ownership is
separate implementation work at this checkpoint.

The historical default CI workflow omitted the tagged oracle suite. Its past
SUCCESS results therefore cover only their stated unit/vet/build/race scope.
Two full local aggregates retained the obsolete now-supported JOIN rejection;
both exit-1 results remain in the evidence directory. The corrected assertion
compares that original JOIN with official SQLite and keeps an unsupported USING
negative. The new CI oracle job builds the exact official source archive,
verifies its archive and generated-amalgamation SHA3 hashes and CLI source ID,
then runs `./scripts/test-oracle.sh` without a package/test-name filter. The
ordinary reference guard and behavioral audit still precede execution; restricted
manual tags are never enabled. See ordinary-ci.md for exact commands and pins.

Corrected source `cd80bbc80228e1970f8d7e465799cc674eede3fb` passed complete
CGO0 unit/vet/build at2026-10-10T19:09:35Z and the complete pinned tagged ordinary
oracle at19:13:56Z. The complete frozen aggregate passed/exit0 at19:21:45Z,
including full race (SQLite392.800s), five target builds, module verification and
unchanged inventory. Hosted CI and exact restored commit results are recorded
separately when terminal; no result from a different tree is substituted.
Native platform runtime, restricted physical recovery/failure, independent
security/cryptography/legal and complete M10 acceptance remain NOT_RUN.


## Predicate and affinity ordinary qualification

Exact source816e84d58da57b537376bcff461776474c1759a4 passed the complete guarded
CGO0 unit/vet/build, pinned ordinary oracle, separate full race, five cross-builds,
module and inventory checks at2026-10-10T19:47:07Z. Exact commands/logs are
`evidence/boolean-rank-lead-commands.txt` and `boolean-rank-lead-checks.txt`.
The first combined source's three genuine sparse/ANN false-predicate failures
are retained with exit1. Existing assertions were not weakened; independent
memory/ordinary negative controls prove the correction. All new executed
fixtures were intact ordinary SQL/read-write/rollback/BUSY/cancel/close/reopen,
correct-key inherited lifecycle checks, or descriptor-free AST/quota/math laws.
No restricted manual tag, physical corruption/tamper/wrong-key/namespace/crash/
hot-recovery/native-failure/interrupted-publication workflow was run. Target
cross-compilation is not native runtime acceptance. Plain ownership, broader
SQL/planner/migration code and independent M10 gates remain separately open.


## Rowid/identifier/global aggregate ordinary gate

Final source48370bbe2dc585c1b45821fbc360c525d156a9f5 completed the full guarded
ordinary unit/vet/build, pinned oracle, separate full race, five target builds,
module and unchanged-source checks at2026-10-10T20:23:10Z (exit0). Original8d7 ordinary
FAIL and1e full-oracle FAIL are preserved; the exact query and genuine quota
negative remain tested after the diagnosed corrections. Component and combined
independent reports include exact source hashes and isolated negative controls.
All executed new tests use intact ordinary databases, normal SQL/transaction/
BUSY/cancel/close/reopen/snapshots, pure-memory arithmetic/AST/quota laws or pinned
read-only CLI queries with normal process completion. No manual tag or physical
corruption/tamper/wrong-key/namespace/crash/hot-recovery/native-I/O-failure/
interrupted-publication/rekey workflow ran. Cross-builds remain compile evidence.
Broader missing SQL/recovery/platform code and M10 gates are not waived.


## Plaintext managed ownership ordinary gate

Source 7d3184f87165e11e12560884ec6e4f92c9bfd039 completed full guarded ordinary
unit/vet/build, pinned oracle, separate race, five target builds, modules and
unchanged-source checks at 2026-10-10T20:24:10Z (exit 0). The four owner increments and
independent reviews are preserved under evidence/. Exceptional retirement,
admission, traversal and absence checks were descriptor-free; native checks used
intact ordinary create/read/write/commit/rollback/BUSY/cancel/close/reopen and
checkpoint fixtures. None executed failed native I/O/lock/unmap/close/Sync,
physical corruption/tamper/wrong-key/missing-file, namespace mutation, crash,
hot recovery or interrupted publication/rekey. Those gates remain NOT_RUN with
the previously recorded manual commands. Cross-builds are compile evidence only.


## Leading secondary inequalities ordinary gate

Exact source60d8a2d68c36c1a48333baf9af2c22eb5fb83188 passed full guarded ordinary
unit/vet/build/oracle/race, five target builds/modules/inventory at 2026-10-10T20:58:20Z.
New runtime fixtures are intact SQL/index/BUSY/cancel/lifecycle or byte-only
range/quota laws. No physical/native-fault/namespace/crash/hot-recovery or
interrupted-publication/rekey workflow ran. Existing concrete manual commands
remain user-requested documentation; these results do not qualify those gates.


## Source-only bounded multi-segment rollback parsing and native recovery ownership

Exact combined source 293a25e4ed905f970b437c28ce6419c1e1b89176 extends verified secondary source 60d8a2d68c36c1a48333baf9af2c22eb5fb83188 with ten
checked paths. The parser accepts aligned published finite-count segments with
matching geometry, per-segment checksums and bounded unique original pages. Its
576 legacy single-segment digests remain unchanged. Unpublished/no-sync/master,
ambiguous and other unsupported forms still fail closed. Native RecoverRollback
consumes this broader parser, so unchanged former consumer bytes would not have
proved unchanged native behavior.

The consumer now reserves bounded cleanup before resolution/acquisition, retains
metadata and temporary/main obligations, distinguishes initial absence, and
retires children before their protecting main owner. Retrying cleanup cannot
replay, truncate, Sync, unlink or republish. Independent negative memory/AST laws
and all prior findings remain in evidence. No RecoverRollback call or physical
recovery workflow was executed. Automatic public Open recovery is still absent.

Exact combined full guarded ordinary unit/vet/build, complete pinned oracle,
full race, five builds, modules and inventory passed exit0 at 2026-10-10T21:12:23Z.
All published non-document bytes equal that fixed source. Journal predecessor
full-source results are retained separately, not substituted for this combined
identity. Native failure/crash/namespace recovery remains NOT_RUN. Shared legacy
absence provenance and the separately reproduced ordinary rollback reader-open
stat/SHARED timing race remain genuine engineering gaps. M10 is unfinished.


## Bounded grouped aggregates, connection state and cached ordinary preparation

Exact combined source13f53e4cdadaaca2530e17e5f7a61b9dfe8e5c33 adds79 checked paths
onto verified replay/ownership source293a25e4ed905f970b437c28ce6419c1e1b89176.
It composes independently reviewed grouped-v2, counters-v1, cached-preparation-v3
and independent regressions by checked Git hunks, preserving existing range,
cleanup and recovery code. GROUP BY/HAVING retains bounded group/representative/
state allocations, conservative supported keys and proved rowid grouping. Stable
LIMIT/OFFSET admission and lookahead fixes retain earlier failing transcripts.

Counter state is connection-local and distinct from statement Result; successful
source inserts, ABORT, transaction rollback/savepoints, DDL and maintenance have
explicit typed SQLite controls. Warm cached ordinary preparation now distinguishes
invalid SQL from execution-BUSY. Cache admission includes fixed empty-catalog
cost, invalidation, binding and supported DDL/rollback paths. Metadata cache never
authorizes data or native acquisition. Sessions/extensions/constant-query warming
and complete stale-schema reprepare remain unsupported cache profiles.

Two imported legacy assertions were identified as outdated BEFORE execution, not
reported as failed runs. The retained warm valid INSERT under EXCLUSIVE now has
strict official changes=0, not the old limitation's2; missing-table control is
strengthened to reject BUSY. Original GROUP BY counter SQL is retained as a full
typed value positive, with official counterpart and additional grouped histories;
the actual unsupported window negative remains unchanged. All old source/assertion
artifacts are preserved. No control is replaced by a log-only assertion.

Exact-source focused normal/race/typed oracle/oracle-race preflight passed20:48:25Z. Full guarded CGO0 unit/vet/build20:52:30Z, complete pinned
ordinary oracle21:05:18Z, full race(SQLite475.910s), five target builds, module
verification and all hashes PASS exit0 at2026-10-10T21:14:01Z. All published
non-document changed bytes equal that fixed source. Exact remote/restored/CI
gates are separately recorded; no recovery/native-fault or foreign runtime ran.

Known wider planner error/phase differences remain held and corrective. Reader/
shared acquisition fixes, CLI/import verification, grams/transaction harnesses and
native integrity/performance-CI additions are excluded from this checkpoint.
M10 remains unfinished; no complete-goal delivery claim follows.


### Native acquisition fixes, maintenance CLI and dataset/grams tooling

Exact source ddb957e2a1d804b236a295f7fe6ce383954b852c adds32 checked paths to
13f53e4cdadaaca2530e17e5f7a61b9dfe8e5c33. It composes independently reviewed
Reader post-SHARED stat, shared initial-absence provenance, encrypted backup/rekey
CLI, importer-distance v2 and NFC grams measurement/correctness sources. Central
documents are reconciled separately; every changed production/test byte equals
the fixed checked composition. Complete ordinary unit/vet/build, pinned complete
oracle, full race, five target builds, modules and full source hashes passed;
exact clocks and all commands remain in accepted-tools-combined-checks.txt.

The Reader fix closes the stat-before-SHARED race observed in the original failed
20-second rollback sample. Two retained baseline controls fail; four corrected
20-second samples pass. These observations do not constitute long-soak acceptance.
Shared absence handling accepts only initial no-handle absence and retains later
lookup/retirement diagnostics. Exceptional native paths were checked only with
inert ownership/source laws, never injected into disk-backed operations.

CLI backup/rekey has independently verified correct-key snapshot/WAL/shared
archive/native restore and explicit provider boundaries. Plaintext backup remains
unsupported. Existing history remains decryptable with prior credentials; no
secure-erasure or native-failure/rekey durability claim is made. Dataset verifier
uses source norms, full exact distances and separate Top-K checks; ANN is not_run.
Its original16000-dimensional cancellation bound failed by14.0724x and remains
failed history. V2 derives outward arithmetic bounds challenged with rational
references, rather than widening an arbitrary tolerance or changing kernels.

NFC corpus covers12800 memberships/12416 distinct pairs, including100 inputs
changed by normalization, without changing LIKE_RAW_V1. Grams smoke and medium
measure real SEARCH/fallback paths and preserve slowdowns: medium512rows reports
147x DB size and4/18 selective medians slower than scan; smoke all12 are slower.
No universal speedup, large benchmark or confidence-qualified result is claimed.

TransactionV4 and nativeCheckV2 have separate independent scoped PASS, but are
excluded from this checkpoint pending their own full-source gate. Wider planner
V3 remains held history for34 newly exposed inherited multi-key/partial-sort
failures; V4b is awaiting independent review after6 V4 bound-LIMIT regressions. Windows
shared-rekey source has independent model/compile review, not native runtime.
Public recovery/Darwin/performance tooling remain separate corrective slices.
Restricted recovery/fault, target runtime and independent M10 gates remain open.


### Native full-profile integrity checks and ordinary transaction/soak tooling

Exact source8b604e3c667152284a29d7ad7dfba5a10f63b053 adds22 checked paths over
ddb957e2a1d804b236a295f7fe6ce383954b852c: independently reviewed transaction
harness V4 and native CheckV2. Full ordinary unit/vet/build passed21:49:37Z,
complete pinned ordinary oracle21:59:34Z, full race(SQLite583.504s), five builds,
modules and complete source hashes passed22:10:33Z. Changed non-document bytes
in the published checkpoint equal that fixed source; central docs are reconciled.

DB.Check/Tx.Check/CLI full-profile checks share one pinned snapshot and bounded
traversal. They validate catalog/free/all-page ownership, B-tree cell/freeblock/
fragment accounting, table/ordinary-index correspondence and built-in grams plus
dense/typed HNSW/IVF semantics. Detached-memory V1 false-success cases remain
failed historical evidence; V2 exact accounting and builtin-only validation pass
those retained controls. Custom registry callbacks never redefine full-check
truth. Acquisition work has existing pager caps outside traversal counters;
logical-NULL IPK checks do not claim physical serial-type0-only validation.
Existing configured WAL sessions support Check; a genuine readonly plaintext-WAL
full CLI is still an unimplemented profile. Existing named WAL checks retain
historical writable coordination and are not relabeled readonly.

TransactionV4 records actual concurrent epochs, deterministic transfer histories,
reader snapshots, full rollback/savepoint outcomes, ordinary checkpoint/reopen,
source/workload identity and truthful error/retirement status. Original V1 Reader
ErrChanged run, V2/V3 mixed-error/pending-retirement model findings and V3 WAL
storage-quota failure remain preserved. V4 requires all errorgraph branches and
keeps pending owners through reporting; a later nilClose cannot erase uncertainty.
Three64-attempt fixed-candidate runs passed, each with48 commits, exact independent
model reconstruction and normal closure. These are bounded smoke observations.
The predeclared two-hour-per-profile run, larger throughput measurements and
confidence-qualified performance acceptance have NOT_RUN status here. There is
no crash, fault, native recovery, namespace substitution or interrupted-rekey
execution. Correctness/ordinary source review is not independent security/legal
release approval. Wider planner, Windows/Darwin/public recovery and repaired
ANN/performance-CI sources remain separate increments.

## 2026-10-11 OS namespace 门控后续记录

2026-10-11 namespace 后续更新：本批以已保存 P0 提交
`56bbcf3febe233197aa0a5eac965c67a67a88011`（tree
`c31e7dcbeed9732688681741ee38ec60ff0bfb49`）为组合基点，只移出两个
Windows/Darwin raw namespace 测试并追加通用 Rename/rename selector guard。
恢复 guard 和 LIMIT 成果保持。六路径候选的 source/model/race、188 项选择矩阵
及六项外平台编译/符号核对已有独立 PASS；原生/manual 二进制全部 NOT_RUN。
详见[本批手册](manual-verification-zh.md#19-os-raw-namespace-门控与独立原生验收2026-10-11)及[范围审计](test-scope-audit.md)。
本批新组合、提交、恢复和 CI 必须各自记录，不能继承 P0 或候选 PASS。

原 M2 至 7addc493 的两个 plaintext namespace 历史 PASS 与 unsafe-scope 事实保持。
本批移动的是另两个 encstore 原生测试，不能混为历史 plaintext 用例。
旧 P0 段落的 namespace 未纳入是该切片范围，不反推当前仍未门控。
真实原生测试须用户独立批准环境，具体平台、tag、env、隔离副本、命令、原断言、
失败保留局限和清理条件都在中文第 19 节；设置 gate 本身不是授权。
