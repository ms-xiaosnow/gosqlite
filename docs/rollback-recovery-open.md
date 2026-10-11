# Opt-in rollback recovery during Open

Status (2026-10-11): bounded experimental public Open source is implemented;
independent source/model/compile review PASS. The reviewed candidate was based
on `63c2ebb10e3a2c4ff2378ee9f33c46133134c70f`; the subsequent composition uses
published `c1502790c4b233757b0f9a3c4d5e5bdee1321310` and preserves its LIMIT
fix and restriction classification. This document is not evidence that the new
combined ordinary suite, exact-commit restore or CI has passed. Physical recovery
and native failure qualification remain **BLOCKED / NOT_RUN**. This is not a production-safety or
complete M3 acceptance claim. Use only disposable data until the independent
release gates are met.

## Public policy and defaults

`sqlite.Options.RollbackRecovery` is an optional `*RollbackRecoveryOptions`.
Nil preserves the existing behavior: every existing rollback journal, including
cold and empty files, makes Open fail with `ErrRecoveryRequired` / `CodeIOErr`.
Plain driver DSNs and zero-value Options therefore still never recover automatically.

The journal-presence diagnostic text and private wrapping type intentionally
change even with nil policy. The new presence diagnostic is
`SQLite rollback journal exists; explicit recovery policy required`. Default
refusal, `errors.Is(err, ErrRecoveryRequired)` and the public primary code are
preserved; neither byte-identical error text nor identical error-chain shape is
claimed. Callers must not interpret a generic sentinel match as recovery authority.

A nonnil option requests at most one recovery attempt during Open/OpenContext,
followed by at most one fresh read-only header probe. The option is supported
only for writable plaintext rollback opens on Linux. ReadOnly returns
`ErrReadOnly`; encryption, any native WAL option, KeepWALOnClose and other
platforms return `ErrUnsupported`. Validation happens before path resolution or
native acquisition. No flag silently upgrades read-only access. NoCreate still
prevents the ordinary absent-file creation path; it permits recovery of an
existing file. Recovery itself never creates a database.

Limits are copied by Open, and copied when driver.NewConnector captures Options:

- MaxJournalBytes: zero means 64 MiB input
- MaxRecords: zero means 65536 records, cumulative across all segments
- MaxDatabaseBytes: zero means 1 TiB original database extent

Plan input plus owned images requires up to roughly twice the journal-byte
limit, with additional O(MaxRecords) metadata. These are not total-RSS bounds.
Do not mutate caller Options concurrently with constructing a connection.

There is no CLI recovery flag, DSN recovery parameter or standalone public
recovery function.
Callers configure it through Options/NewConnector. URI mode=ro still forces
ReadOnly and consequently rejects an enabled policy when Connect reaches Open;
mode=rw still forces NoCreate. URI rw/rwc never clears either caller restriction.
The driver's unconfigured registered Open/OpenConnector paths retain nil policy.

This policy is limited to opening a connection. Query, Schema, Begin, Exec,
existing Reader/Tx snapshots and WAL sessions never invoke rollback recovery.
A journal appearing later still fails closed; it is not retried behind an active
connection, row iterator or transaction.

## Ownership and exact attempt bound

1. The initial public open reserves its existing plaintext owner before path
   canonicalization. The reservation transfers into its read-only Reader probe.
2. A successful journal metadata lookup creates a private path-bound presence
   diagnostic. It says nothing about whether the journal is hot or valid.
3. An unsuccessful Reader constructor runs its unchanged one-attempt managed
   retirement. Only full verified completion with that exact sole presence
   diagnostic and no added close/cleanup diagnostic creates a private ready proof.
   Failed, incomplete, terminal or consumed-error retirement cannot create it.
4. Only the exact proof for this resolved path authorizes the recovery bridge.
   Generic ErrRecoveryRequired, errors.Is matches, wrappers, joins and extra
   causes do not authorize recovery. No cleanup retry is performed implicitly.
5. The internal recovery operation reserves its own root, reacquires and checks
   SHARED/RESERVED/PENDING/EXCLUSIVE and file identities, then validates its entire
   replay plan. The first probe holds no native owner at this point.
6. Only recovery success including all managed retirement permits one new
   pager.Open probe with a fresh charged root. It never reuses the consumed
   admission, recurses into public Open or falls through to creation. A second
   journal or missing pathname ends this attempt without replay or creation.
7. A successful retry probe is closed before Open succeeds. A close error,
   pending token, recovery error, cancellation or retry-probe error is returned
   together with the original journal diagnostic. Any pending native owner stays
   reachable through existing PendingPlaintextCleanups/RetryPlaintextCleanup.
   Those APIs retry ownership retirement only, never logical recovery.

Cancellation is checked before recovery and before the fresh probe. Internal
recovery checks cancellation before its first database mutation, then completes
publication or reports its uncertain/poisoned outcome. No late cancellation check
discards a successful probe result. Because unsuccessful recovery preserves the
initial ErrRecoveryRequired, its public primary code remains CodeIOErr; the
specific BUSY/cancellation/limit/cleanup causes remain available through Is/As.

## Exact supported and remaining limits

The underlying operation is DELETE-mode recovery on stable single-link regular
Linux database/journal paths, with cooperative SQLite byte-range locking. It
accepts bounded, fully published finite-count segments, matching first-header
geometry, segment-specific nonce checksums, globally unique original page images
and exact final consumption. It checks shortened extents and the restored header
before writing. Supported page sizes are powers of two 512..65536; recovery
sector sizes are powers of two 32..65536. SQLite's sparse checksum does not
authenticate file identity or malicious content.

Cold/empty journals, ambiguous empty main files, unpublished/truncated headers,
checksum errors, duplicate pages, incomplete extents and ambiguous trailing bytes
remain preserved errors. Super/master-journal trailers, no-sync 0xffffffff
record counts, unspecified legacy page size, mismatched segment geometry,
reserved-byte pages and unsupported restored database headers remain unsupported.
An original zero-page extent can produce a recovered empty database; the fresh
ordinary probe then refuses that empty file, without silently creating a new one.
This is a possible recovery-success/open-error result, not a lost recovery error.

Any WAL or SHM sidecar blocks rollback recovery, including empty sidecars. No
WAL tail is truncated, reset or reclassified by this option. Darwin/Windows
recovery, default-on recovery, cold-journal deletion, super/master-journal
transactions, sentinel-count interpretation and broader tail handling remain
separate implementation/policy work. Adversarial namespace replacement is
outside the current cooperative path guarantees.

## Ordinary verification boundary

New tests use only descriptor-free owner/error models, memory-only callback
sequences and source AST inspection. They establish policy/defaults, path-bound
proofs, cleanup-error rejection, exact one-attempt ordering, cause preservation,
fresh admission, no-create retry, driver option copying and URI restrictions.
They never call the production recovery bridge or pass recovery-enabled Options
to real Open. Historical selected normal lifecycle evidence belongs to its own exact tree.
This integration uses only the audited source/model selectors listed below; no
new real-database execution is implied.

`go run ./internal/testscope` runs before ordinary suites. It now rejects direct
or captured recovery functions, the production recovery bridge, recovery option
field references and unkeyed sqlite.Options in ordinary test sources, including
dot-imported Options and same-file alias/type-definition chains. The independent
alias/dot-import witness strings are retained as negative guard regressions.
Recognition is conservative and scope-insensitive; it is not whole-package Go
type resolution. Cross-file declarations, inferred container element types,
helper calls and reflection still require behavioral review. Shadowed unrelated
type names can produce conservative false positives. The separate Rename/rename
selector expansion and Windows/Darwin raw-namespace gating are absent from this
base and are not part of this integration; do not run native OS aggregate suites. This guard remains
a prevention aid, not a complete call-graph or reflection proof. No tests bypass it with reflection or hidden recovery hooks.

The existing internal physical harness commands remain in
[manual verification](manual-verification.md), excluded from ordinary CI. They
remain NOT_RUN and do not exercise this public bridge. The public physical
harness is **IMPLEMENTATION_MISSING**; there is no public-bridge physical test
command to supply until that harness is implemented and independently reviewed. No hot/crash recovery, native fault injection, corruption/tamper,
wrong-key, namespace replacement, forced exit or interrupted publication/rekey
was executed or relocated. Compiles and models do not qualify physical replay.

## Reproducing this bounded source/model check

Read the selected tests and their helper chains before executing these commands;
the source guard does not replace that review. The commands do not enable either
manual tag. Use pinned Go 1.27.2, CGO disabled except for the separate race check,
and preserve the exact source tree, command, UTC timestamp and exit code.

The commands below reproduce the reviewed argv/selectors with `go` resolved to
the independently verified Go 1.27.2 binary. Use a reviewed repository-root
snapshot containing these tests, `GOTOOLCHAIN=local`, `GOPROXY=off`, `GOSUMDB=off`,
`GOFLAGS=-buildvcs=false`, `GOMAXPROCS=2` and verified offline GOPATH/GOCACHE.
The recorded runs used default `/tmp` with GOTMPDIR unset; their resource gates
were at least 1 GiB available there, 2 GiB in the workspace and 10000 available
inodes plus writability at both. A failed or different precondition requires a
new decision, not silent cache relocation or a lower threshold. Document the
actual environment; these values are not a disk-persistence qualification.

```sh
CGO_ENABLED=0 go run -p=1 ./internal/testscope
CGO_ENABLED=0 go test -p=1 ./internal/testscope -run '^(TestScopeSourceReferenceGuard|TestScopeRecoveryOptionTypeChains|TestIndependentReviewGuardDirectAndAliasLimit)$' -count=1 -v
CGO_ENABLED=0 go test -p=1 ./internal/pager -run '^(TestReaderRecoveryMemoryRetiredPathProof|TestReaderRecoveryMemoryDiagnosticAndOwnershipRefusal|TestReaderRecoverySourceProofBoundary|TestReaderAcquisitionSourceSnapshotAfterShared)$' -count=1 -v
CGO_ENABLED=0 go test -p=1 ./sqlite -run '^(TestRollbackRecoveryMemoryPolicy|TestRollbackRecoveryMemorySingleAttemptSequence|TestRollbackRecoverySourcePublicAndDriverBoundaries|TestRollbackRecoveryMemoryFailureClassification)$' -count=1 -v
CGO_ENABLED=0 go vet -p=1 ./internal/testscope ./internal/pager ./sqlite ./driver
CGO_ENABLED=0 go build -p=1 ./...
CGO_ENABLED=0 go list -p=1 -deps -f '{{if .CgoFiles}}{{.ImportPath}}: {{.CgoFiles}}{{end}}' ./...
CGO_ENABLED=0 go run -p=1 ./internal/testscope
CGO_ENABLED=1 go test -p=1 -race ./internal/testscope -run '^(TestScopeSourceReferenceGuard|TestScopeRecoveryOptionTypeChains|TestIndependentReviewGuardDirectAndAliasLimit)$' -count=1 -v
CGO_ENABLED=1 go test -p=1 -race ./internal/pager -run '^(TestReaderRecoveryMemoryRetiredPathProof|TestReaderRecoveryMemoryDiagnosticAndOwnershipRefusal|TestReaderRecoverySourceProofBoundary|TestReaderAcquisitionSourceSnapshotAfterShared)$' -count=1 -v
CGO_ENABLED=1 go test -p=1 -race ./sqlite -run '^(TestRollbackRecoveryMemoryPolicy|TestRollbackRecoveryMemorySingleAttemptSequence|TestRollbackRecoverySourcePublicAndDriverBoundaries|TestRollbackRecoveryMemoryFailureClassification)$' -count=1 -v
```

Run each reviewed command separately and stop on nonzero exit, SKIP, an unmatched
selector, a guard violation, unexpected activity or missing evidence. Retain the
exact command, exit, UTC clocks, stdout/stderr and pre/post source inventory.
The guard must precede both test modes, and each mode must actually match all
11 top-level tests with zero skips. The dependency listing must contain no
nonempty CgoFiles entry. Build/vet and this listing are compilation/dependency
evidence only. The physical production
bridge, an enabled option passed to real Open, both manual-tag suites, physical
replay/fault/namespace workflows and default/oracle aggregates are **NOT_RUN** in
this integration. No historical V1/V2 PASS is attributed to this integrated tree.

For the public API configuration example and the separate future manual review
steps, see [the Chinese manual, section 18](manual-verification-zh.md).

## This integration's executed evidence

At 2026-10-11 07:42:37 UTC the serial scoped checks above passed: both source
guard runs; all 11 exact source/model tests in CGO0 and the same selection under
CGO1 race; vet of testscope/pager/sqlite/driver; a full-package CGO0 build; and an
empty CgoFiles production dependency closure. Go 1.27.2, local/offline caches,
GOMAXPROCS=2 and -p=1 were used. Complete pre/post-check byte/size/mode inventories
match. Only this results paragraph and the progress entry were added afterward;
no Go source changed. This describes the candidate owner's run, not the later
c150 composition.

Independent review of exact tree `75c074548e8b5208beb06e6ead314d835cd16f4f`
then found no blocking source defect and reproduced all 13 command stages,
including version and read-only gofmt checks, with natural exit 0. The last race
stage finished at 07:49:34.878759 UTC and post-check identity at
07:49:35.178221 UTC. All 11 top-level tests matched in each mode, with no skips,
failures or unmatched selectors; the tests include 9 ownership-refusal and
11 sequencing subcases, and the policy enumerates 64 mode/platform combinations.
Snapshot and driver integration are AST/source checks, not enabled connections.
The baseline, candidate and independently replayed full inventories were checked;
the candidate's before/after/replayed trees match. All 13 Go source paths and
three candidate document paths were accounted for, with no changed unowned paths.

The report and exact argv/resources/log index are retained as external evidence
`public-open-independent-review-v1/report.md` (SHA-256
`e507af6e674aae7802b060cb3c798fc2b21e7f4c6771271533576b65c91a6d17`) and
`public-open-independent-review-v1/commands.json` (SHA-256
`e0ab817d9f0993a14409d61b4f2bced3c8fc09f0992c3f16b11103e2d94c3e00`).
Those names do not imply the evidence is present in every checkout. The later
c150 composition must preserve the reviewed 13 source paths and all unrelated
c150 changes, then obtain its own full ordinary/oracle/race, exact-commit restore
and CI records. At this documentation checkpoint those combined/persistence
results are **PENDING**, not inherited candidate PASS.

These are model/source/compile PASS results, with zero real-database workload.
They do not qualify the physical bridge, recovery-enabled Open or any manual
recovery/native-failure workflow, all of which remain **BLOCKED / NOT_RUN**.

### P0 默认关闭完整普通门结果（2026-10-11 08:22 UTC）

当前P0组合执行冻结tree `442a7104dc780ddb20826e5ee4f1250b291f2b24` 于08:22:39 UTC自然完成18阶段PASS：完整CGO0普通测试/vet/build、固定SQLite3.54.0完整普通oracle、完整CGO1 race、guard、五目标生产编译、模块及全部3004文件/143目录前后身份。785非docs文件固定；最终文档/证据改变整个tree但不得改变这些运行源码。两个外部预检的报告版本/基线POSIX比较问题在Go执行前发现并保留，修正外部验证器后一次完成运行门，无运行重试。详见[精确组合证据](evidence/public-open-integration-20261011.md)。本批新提交/Library恢复/CI仍待实际核验。真实RecoverRollback、production bridge和开启恢复选项的真实Open/Connect均BLOCKED / NOT_RUN，公开物理harness仍IMPLEMENTATION_MISSING；M3/M10/整体未验收。

## 2026-10-11 后续 namespace 切片交叉说明

上文 Rename/rename 与 OS namespace 未纳入，是原 P0 切片及其当时基线的历史
范围。后续在已保存 P0 提交 `56bbcf3febe233197aa0a5eac965c67a67a88011` 上
逐 hunk 组合独立六路径门控；仅追加 Rename/rename selectors，P0 恢复 guard
及本页原 descriptor-free 命令/证据全部保持。具体测试移动、默认覆盖减少、
188 项选择、六项外平台编译/符号证据及用户独立原生规程见
[中文手册第 19 节](manual-verification-zh.md#19-os-raw-namespace-门控与独立原生验收2026-10-11)；该切片不增加 public physical harness，
所有真实恢复、启用恢复的 Open/Connect、namespace 原生执行仍 BLOCKED / NOT_RUN。
P0 完整普通门、已保存基点、精确恢复及尚待终态的 CI，按[当前验收补记](acceptance-status.md#2026-10-11-os-namespace-六路径批次)分别记录；不将新候选 PASS 归入旧切片。
