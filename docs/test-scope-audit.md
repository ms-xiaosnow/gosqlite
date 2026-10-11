# Ordinary test-scope correction, 2026-10-10

## Finding and historical evidence

At 13:22 UTC, source review identified two existing namespace-mutation regressions
in `sqlite/write_test.go` that did not belong in the restricted ordinary suite:

- `TestNativeTransactionDetectsReplacedValidFile` renames a complete replacement
  SQLite database over the path while a transaction holds the original file.
- `TestNativeConnectionKeepsResolvedSymlink` removes and retargets a symlink after
  a connection has opened through it, then verifies later writes.

Both were introduced in commit
`9feb20357fedb2bff7901a97a462e9bf4821e279` (M2). Subsequent full unit/race/oracle
commands selected these tests. Their historical PASS outputs are retained as
factual results. Earlier general statements that those full runs excluded every
restricted workflow are superseded by this correction. These results must not
be presented as clean ordinary-scope attestation or physical safety acceptance.

Checkpoint `7addc4935c7547ae32f76bcdf5e2635044c84fc9` remains an exact, verified
repository/bundle snapshot. Its CI run
[38055114656](https://github.com/ms-xiaosnow/gosqlite/actions/runs/38055114656)
completed success at 13:25:22 UTC, including the two tests. That status is factual,
not corrected-scope evidence. No CI cancellation was completed. Its clean
Library restore had already passed byte identity, clone/fsck and the goal hash;
the running test validation was stopped with exit 130 after discovery. Its queued
oracle script was blocked as an additional precaution. That local restore is
therefore modified and has no completed full-test result. The persistent bundle
was not modified, and no interrupted database recovery test was performed.

## Correction

Both regression bodies are retained in
`sqlite/write_namespace_manual_test.go`, behind the exact `manual_namespace`
build constraint and a separate explicit environment gate. Default and `oracle`
builds exclude that file. Existing `manual_recovery && linux` source remains
separately excluded. These physical workflows must not execute here, in CI, or
be relocated to another executor to evade restrictions. Compilation-only checks
are distinct from execution and confer no acceptance.

Both ordinary scripts now run `internal/testscope` before any database tests.
The guard parses test source and rejects direct high-risk namespace, permission,
process-termination and recovery references, including import aliases and method
values. Manual exclusions require exact approved constraints, so an expression
such as `manual_namespace || linux` cannot hide an ordinary-selectable test.
Manual tags through GOFLAGS are refused. Pure-memory tests verify the guard.

This guard is a prevention aid, not a security sandbox or complete call-graph
proof. Helpers, wrappers, dynamic values and future APIs still need behavioral
review. File writes cannot all be forbidden: copying an intact fixture into a
new temporary path and ordinary SHM backing growth are legitimate tests.

## Source audit before resuming aggregates

The follow-up audit examined direct filesystem mutations and process calls in
the default and oracle sources, including newly added work:

- Remaining `os.WriteFile` calls copy intact valid fixtures into fresh temporary
  paths. They do not mutate the source fixture or inject invalid physical pages.
- SHM mapping tests allocate backing extent normally before mapping. They do not
  shrink live mappings or inject I/O failure.
- Cooperative lock tests use ordinary lock acquisition/release and close.
- The SQLite oracle helpers finish through normal SQL, `.quit`/EOF and Wait;
  no CommandContext auto-kill, signal or Kill hook is present.
- Malformed page, WAL, crypto, graph and vector checks examined in this audit
  change in-memory bytes/models or reject invalid API inputs before publication.
- The ordinary current-directory test changes process cwd, without changing the
  database namespace, and remains an ordinary path-resolution test.

A second source review confirmed the two files' default/oracle selection metadata
and identified guard hardening gaps, which were fixed before publication. The
scope correction does not certify an exhaustive adversarial audit or undo earlier
execution. Only new audited, gated runs can supply corrected-scope verification.

The next S1/BIT/converter/native-ownership source audit additionally distinguishes
SQL-text fixtures from database files: the converter CLI test writes only an
input text script to a fresh temporary path, and the converter never executes
that input. New ownership cleanup/poison regressions use never-opened synthetic
file placeholders and memory callbacks; every public method is stopped by state
validation before any native backend call. No native lock/unmap/close error is
injected. New live lock tests use intact files, normal acquisition/BUSY/release,
and official CLI normal completion. The source-reference guard passed again
at 2026-10-10T13:44:06Z before the planned combined aggregate.

## Corrected-checkpoint results

Commit c7764896c541c4ed6177288ec8f9625828093ade was mirrored exactly and
restored byte-for-byte from Library libfile_e94b5944a3a481919e03d83293617671.
Its guarded CI38056050522 completed success at13:40:15Z. The clean restored
unit/vet/build/race plus pinned SQLite oracle finished exit0 at13:46:19Z; the
restored Git tree remained clean. See evidence/test-scope-library-restore-recheck.txt.
These are new corrected-scope results, without changing historical findings.

## 2026-10-11 Windows/Darwin raw namespace 后续修正

2026-10-11 namespace 后续更新：本批以已保存 P0 提交
`56bbcf3febe233197aa0a5eac965c67a67a88011`（tree
`c31e7dcbeed9732688681741ee38ec60ff0bfb49`）为组合基点，只移出两个
Windows/Darwin raw namespace 测试并追加通用 Rename/rename selector guard。
恢复 guard 和 LIMIT 成果保持。六路径候选的 source/model/race、188 项选择矩阵
及六项外平台编译/符号核对已有独立 PASS；原生/manual 二进制全部 NOT_RUN。
详见[本批手册](manual-verification-zh.md#19-os-raw-namespace-门控与独立原生验收2026-10-11)及[范围审计](test-scope-audit.md)。
本批新组合、提交、恢复和 CI 必须各自记录，不能继承 P0 或候选 PASS。

本节不重写上面 M2 至 7addc493 两个 plaintext namespace 测试曾执行的事实。
这次移走的是另两个 encstore 函数：

- `TestWindowsEncryptedRelativePublicationAndRetainedRoot` → `namespace_manual_windows_amd64_test.go`。
- `TestDarwinEncryptedRelativePublicationAndParentLifetime` → `namespace_manual_darwin_test.go`。

各文件精确首行 `//go:build manual_namespace`，各函数首句检查
`GOSQLITE_MANUAL_NAMESPACE=independently_authorized`，在 TempDir/原生操作之前；
未提供时 SKIP 不算覆盖。完整 body 去掉该首句后与原函数字节相同，既有断言未删弱。
默认/oracle 在适用平台有意少这两个声明，其他原有声明全部保留；manual 选择
保留原平台条件，包括 Go 既有 Darwin/iOS 关系，不据此宣称 iOS 运行。

Guard 只在当前 selector switch 追加 Rename/rename；撤回该两项即恢复当前 P0
main.go 的全部字节。恢复字段/函数、same-file alias、dot import 和 unkeyed Options
检查及原测试保留。新增 `TestNativeNamespaceMethodReferenceGuard` 只解析惰性源码
字符串，验证直接 selector、captured method value、generic Rename 和 exact tag 豁免。
独立普通/race 及额外 source probes 的结果是模型证据，不能宣称运行了示例中的原生操作。

匹配保守、不做完整类型/调用图分析，可能误报无关 Rename；bare helper、改名 wrapper、
reflection/dynamic 调用和新 API 仍需行为审查。exact manual tag 豁免本身不检查所有
body 的环境门；两项 moved body/gate 已另行独立比对。

新候选 tree 01918609 的 owner、独立 source review 和 compiler addendum 分别保留
在中文第 19 节所列外置证据。188 个选择案例、六个外平台 compile/symbol 检查
均只属于该固定源码。本批原生/manual 二进制从未执行；所有物理限制继续生效，
不把新门控当作新 OS runner、默认全套或迁移受限工作流的许可。
