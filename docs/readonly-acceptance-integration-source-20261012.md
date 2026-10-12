# 只读验收 V3 检查点完整正常门终态

## 2026 年 10 月 12 日 13 时 31 分 UTC 最新结果

冻结运行候选 `readonly-acceptance-integration-source-v3`，tree
`f8192e090acb9acedb07f811a66d043c1a9a24c2`（3,312 文件、817 Go 文件），
于 **13:31:44 UTC 自然完成完整 26 门 / 28 次 Go 调用 PASS**。
session `75199` 工具自然 exit 0、chunk `903c9f`，lease **RELEASED**；
全部阶段的原始 stdout/stderr、自然返回、源码身份与资源收据均已静态核对。

- ordinary / race 各 **9,735 named PASS / 7 SKIP**，合计 9,742 named；
  各 1,519 Test、41 Fuzz、6 Examples，313 个固定种子 PASS。
- 完整 pinned oracle **11,474 named PASS / 0 named SKIP**；
  1,637 Test、41 Fuzz、6 Examples，313 个固定种子 PASS。
- LIMIT oracle/race **394 named PASS / 2 根**；guard、四条 quick、live-writer
  readonly oracle、vet、CGO0 build、模块核验及五目标生产编译均 PASS。
- Darwin 双架构和 Windows 的 test-c/nm 六门均 PASS，生成的外国映像未执行。
  whole 非测试闭包实测 **212 包 / 1,372 输入**，shipping library/CLI 实测
  **164 包 / 1,261 输入**；shipping 未到达 testoracle/testscope/os/exec 等禁止依赖。
- **6,651 个资源样本零违反**；所有 outer children 自然返回，没有强制终止或
  仍活跃的被观察子进程。该结论不改写旧环境 session `54102` 的 **UNKNOWN**。

本次包含实际 Linux 健康文件/WAL/正确密钥路径、纯模型、source/AST 检查和既有
固定 seed，并非全部 descriptor-free 或“所有 native 均未执行”。真正 manual
readonly-media A/B、升级/恢复、fault、WAL publication/elapsed 及外平台原生
资格仍 **NOT_RUN / 未验收**；普通 PASS 不取得掉电/真实持久介质资格。

首轮 full26 V1/session `48019` 在 V2 源上的 guard **FAIL** 与后续 25 门
**NOT_RUN** 保留。V3 的两 guard 修正、56 条根内 inert 负断言和 quick `11576`
两门 PASS 分别记账，不将旧失败回写。

已发布基点仍为 main `b9f6e8e00036a3b264c8a7eb83d0c61539899782` / tree
`c7e412092f6d6e2c8bf98c95e096e3c65b38dd93`，其 bundle、恢复 `80868` 的
26 门 PASS 和 CI `38219105955` 两项 SUCCESS 已闭合。**本新检查点提交、
新 Git bundle、精确恢复及对应新 CI 仍 PENDING**。文档叠加未改运行源，
叠加后的完整树须另行固定，不能把它写成已经运行的 f8192e 树。

独立 source-only donor `d87e78a7…` / `bfca0d83…` 与在独立源码推进的 A5
退出/重启设施均未合入。已有 donor 的 Library 静态保存不转授本批或 native 资格。
[终态收据、闭包、持久性边界和有限余缺](evidence/readonly-acceptance-checkpoint-20261012.md)
给出准确依据。public readonly 仍为 Linux live initialized-SHM 子集；CHECK
有界子集与 EOFUnproven decode-only/native finite-only 拒绝不变，M3/M8/M10 未完成。

## 历史阶段快照与原文完整保留

以下全文保留此前在途快照及更早历史。旧 PENDING、EXPECTED、SOURCE_ONLY、
FAIL/HOLD/UNKNOWN 与“当前”均只指其各自时点，不能覆盖上面的完整正常门终态。

# 只读验收 V3 检查点当前状态

## 2026 年 10 月 12 日 13 时 04 分 UTC 阶段快照

当前冻结运行候选为 `readonly-acceptance-integration-source-v3`，tree
`f8192e090acb9acedb07f811a66d043c1a9a24c2`，3,312 文件、817 个 Go 文件。
本节是文档叠加草稿；没有修改正在运行的候选。叠加后的完整文档树不是该冻结树，
新树与新提交身份须另行固定，不能把本页的增加算成已经运行过的字节。

- full26 session `75199`：**IN_PROGRESS / PENDING**，计划为 26 门、28 次 Go 调用。
  本快照截至 `2026-10-12T13:04:23.042014+00:00`；guard、四条 quick、live-writer
  readonly oracle、ordinary、vet 八门已有 `VALIDATED PASS`，CGO0 build 刚开始。
- ordinary 是实际结果：**9,742 named = 9,735 PASS / 7 SKIP**，1,519 Test、
  41 Fuzz、6 Examples，313 个固定种子 PASS；自然 exit 0，结果和源码身份复核通过。
  race 的同一计数、完整 pinned oracle 的 **11,474 named** 仍是 **EXPECTED**，
  不能写成实际终态；full26 也不能从八个已过阶段推定通过。
- V3 的七个既有 readonly AST 根及完整 source guard 已在独立 quick session
  `11576` 实际 PASS，12:47:51 UTC 自然完成。56 条新增 inert 负断言在既有根内，
  没有新增 Test 根或命名子项；它们不构成 native 验收。
- 首轮 full26 V1/session `48019` 在 V2 源树的 guard 真实 **FAIL**，后续 25 门
  **NOT_RUN**。四条诊断来自两个遗漏 append 各被检查两次；V3 仅修两个 guard
  文件及这些精确传播规则。旧失败、旧 pins/HOLD 和所有历史收据继续保留。
- 已发布基点为 main `b9f6e8e00036a3b264c8a7eb83d0c61539899782` / tree
  `c7e412092f6d6e2c8bf98c95e096e3c65b38dd93`，其 bundle、恢复 session `80868`
  的 26 门 PASS（10:05:12 UTC）及 CI `38219105955` 两项 SUCCESS 已分别闭合。
  这些事实只属于 b9；本新检查点提交、bundle、精确恢复及新 CI 均 **PENDING**。

未发布 V3 源码及原 FAIL/quick 两门 PASS 的静态保存包已读回：Library ID
`libfile_52f330fd86948191bbba9175dfdd64ea`。它不含活动 full26 证据，
不是 Git bundle、精确恢复运行或新 CI；不能据此将上述 PENDING 改成 PASS。

五组切片（Darwin/Windows backend、真实只读媒体、layout1→2 升级、rollback adapter、
WAL elapsed）共 43 路径已进入该候选。真实 readonly-media A/B、升级/恢复、fault
注入与 WAL publication 耗时测量，以及 Darwin/Windows 原生行为仍 **NOT_RUN**。
public readonly-WAL 仍只接纳 Linux live、已初始化 SHM 的既定子集；底层 groundwork
不开放其他平台。首个 rollback fault/recovery caller 与后续 SQL-ledger 增量均未纳入。

[精确身份、实际与预期计数、保留失败及有限余缺](evidence/readonly-acceptance-checkpoint-20261012.md)
是本节的详细依据。CHECK 已在 b9 且仍为有界 scalar profile；EOF-count 仍是
EOFUnproven 的 decode-only，native finite-only 拒绝不变。A1–A8、六个已有协议
profile 的有限设施缺口和外部资格分开保留，不能称为只剩人工执行或目标已完成。

## 历史原文完整保留

以下为本页此前的完整字节内容。旧“当前”、e092/main8bc、CHECK 尚未合入、V2
SOURCE_ONLY、PENDING、FAIL/HOLD/UNKNOWN 与 NOT_RUN 均只绑定各自阶段和源码。
它们没有被删除或改写，也不覆盖上面的 V3 阶段快照。

# Readonly acceptance source integration: V2

Status: SOURCE_ONLY_PENDING_COMBINED_REVIEW_AND_VALIDATION. This fresh V2 combines the reviewed media owner-field guard correction with the independently reviewed three-main-pin migration. No combined Go, formatter, parser/AST runner, compile, test, oracle/native, Git, network or publication action has run. Finite isolated evidence does not transfer to a combined PASS.

## Exact scope and preserved history

The V1 combination remains frozen at tree 827ce8784d1150618424bcac3c37e665613151b7, including its main-pin HOLD and known media guard failure. V2 replays only the two reviewed media guard-file deltas and applies exactly the previously proposed three hash literals after complete-main independent review. Combined main.go is unchanged, SHA-256 dd0d43330c5ad020b4ab2e5649f32f963aa4909316be64ae32c01d9f3c4371aa. All shipping production code, manual/native harness bytes, pure contract bytes, prior guard rules, CHECK/EOF logic, scripts, workflows and module files remain unchanged from V1. The separate rollback-fault recovery harness is not imported.

The owner-field correction binds only the reviewed integer Next counters and InspectResult.Schema data to exact lexical declarations/acquisition and approved uses. It keeps real Rows/Tx/DB method-value captures, aliases, shadowing, callable fields, address-taking, assignments and range targets rejected. Its regressions stay inside existing test roots; the prior mount-field correction remains intact. This source integration is not a type-checker or native behavior result.

## Isolated finite status at V2 preparation

- Darwin and Windows backend finite source checks and rollback-adapter finite checks were previously qualified on their own exact slices.
- WAL elapsed: all eleven finite steps PASS, including six memory roots, eleven AST roots, five OS/tag lists and compile/symbol inspection. The manual binary was not executed.
- Encrypted format upgrade: seventeen finite steps plus the prior list PASS, including eight roots and 106 named paths, and the whole-source guard. Earlier recorder classification failures remain preserved.
- Prepared readonly media V4, session 41545: all thirteen finite steps PASS naturally at 12:18:42 UTC, including seven AST and four pure roots with zero skips and zero named children, six OS/tag matrix checks, and Linux manual test compilation/version/symbol inspection. The queue lease is released with no live child. The native/manual image and real readonly-media profiles were NOT_RUN. Earlier V3 six-PASS/one-FAIL evidence remains preserved.

These isolated statuses come from the parent-reviewed exact-source evidence. This integration performs no execution and makes no combined qualification claim. This V2 source/document snapshot is now frozen; full combined review and validation remain separate.

## Default selection and production boundaries

Default and oracle-only selections exclude the manual fault adapter and every integrated native/manual harness. The default adapter contains six exact one-return calls to the original I/O owner/helper; reversing its 14 Commit call substitutions and the io.WriterAt signature recovers the b9 Commit source exactly. The manual adapter requires Linux AND manual_fault_rollback and has no approved recovery harness in this batch.

The three harnesses require Linux AND their explicit respective manual tag plus environment-first authorization. Public readonly-WAL admission stays Linux-only. Darwin/Windows changes are backend groundwork confined to nativeAttachDMS/nativeMap; native device/filesystem behavior and public support remain unqualified. No physical recovery or elapsed measurement result follows from pure/source checks or compilation.

## Conditional ordinary selection expectations

Static additions remain 58 ordinary Test roots, 729 named children, 787 named paths and two test packages; no existing root is removed. Ordinary/race expected names would be 8,955 + 787 = 9,742 and top-levels 1,508 + 58 = 1,566. Oracle expected names would be 10,687 + 787 = 11,474 and top-levels 1,626 + 58 = 1,684. Existing 41 Fuzz roots, 313 fixed-seed passes, 6 Examples and ordinary expected skips remain unchanged. These are conditional future expectations, not observed combined results.

The known stale-main-pin hold is resolved in V2 by the exact independently reviewed proposal. That does not prove the combined guard or full26 gate passes. Full combined source/production-closure review and any execution gate require their own exact-source authorization and evidence. Native/physical acceptance and the original overall goal remain open.
