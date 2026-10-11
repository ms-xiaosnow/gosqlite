# 源码开发与受限执行的证据分类

核对日期：2026-10-11 UTC。本文纠正此前将多项剩余编码工作一并列作平台禁止的过宽表述。原始 [goal.md](../goal.md) 不变；初次分类时的开发基点为 `63c2ebb10e3a2c4ff2378ee9f33c46133134c70f`，tree `d735409ebd1c43d90ee1650c8a7c2a084e270674`。07:01:55 UTC 的远端只读记录仍指向该提交。本轮后续新提交、精确恢复和 CI 尚待实际产生及核验，不预写自指 SHA。

**结论：在已搜索的可访问证据中，没有定位到 recovery、backup、WAL、planner、backend 或缺失 harness 的精确源码编辑动作被拒绝的原始记录。** 已定位的事实包括受限测试执行、开发时的谨慎暂停、真实工程缺陷、待审阅/待集成，以及触发动作不明的整轮安全中断。这些事实应分开记载。没有找到记录，不证明从未拒绝，也不自动授予任何操作权限。

2026-10-11 06:53 UTC 用户的新指示为：

> 1. 受限于平台安全问题的测试，你跳过并记录文档就行（附带详细测试步骤供我自行测试）
> 2. 其余部分继续编码，完成一定量的任务就提交并报告一次，一直循环到整个项目编码完成为止

据此，继续推进能够明确划定范围的编码批次；平台受限执行仍跳过并如实记录。此前“三项有限收尾后不再新增功能”“在现有限制下完成了独立可推进项”的结论，依赖当时过宽的暂停分类，只是当时的有限范围判断，不能代表原目标完成、所有可编码项完成或已证明这些源码禁止编辑。

## 一 分类标准与证据限度

| 类别 | 必须具有的依据 | 可以得出的结论 |
|---|---|---|
| ① 精确源码编辑被拒绝 | 原始拒绝能定位编辑动作与目标 | 该动作停止；不能改名、换输入、换环境重试。当前所查源码项没有定位到此类记录 |
| ② 执行或测试受限 | 记录明确限制某个测试或运行类别，但未证明其源码编辑被拒绝 | 保留执行限制与 BLOCKED / NOT_RUN；源码、测试设施是否缺失另列。多数物理限制的原始工具参数/拒绝全文没有找到，不能补造 |
| ③ 谨慎开发暂停或工程待办 | 暂停决定、缺陷报告、未完成审阅/集成、分期或资源前置 | 真实缺陷和待办仍成立；逐个源码动作核定范围，不能仅凭 HOLD 标签声称平台禁止编码 |
| ④ 整轮安全标记的操作未知 | 已有中断/标记事实，但精确动作、目标或未报告执行未知 | 保留 UNKNOWN 与对应链路禁重试；不能任意归因到附近某个文件，也不能借未知认定已获许可 |

一个项目可以同时属于②和③。下表中的“①未定位”仅限本次证据集；“④未归因”表示无法把整轮事件归因到该项，并不表示该项确定未受影响。所有后续动作按具体源码、依赖和验证选择审阅，本文没有一揽子解除旧候选。

## 二 原始证据与后来暂停的区别

1. 原始交接 `INTERRUPTION-20261010T2239Z.md:9–13` 明确记录：22:39 中断的精确触发动作和目标未知；附近活动是 ANN V4 修正以及 WAL、backup、soak-plan 审阅。附近活动名称不能替代被拒绝动作。
2. 同文件 `:15–21` 保存约 22:30 的 41-target 活动错误原文：“This content was flagged for possible cybersecurity risk.” 最后记录仍为目标审阅；有限搜索没有找到新 campaign 日志或结果。没有计入该轮执行次数或 PASS，但也不能证明没有未报告动作。
3. 同文件 `:23–31` 的“All new implementation, tests, pushes, CI activation, and interrupted-chain retries are paused.” 是中断后的全面暂停记录，没有指明某项 source edit 被工具拒绝。此前项目文档将这种暂停扩大到多项源码待办，属于③，原触发范围仍是④。
4. [历史手工记录](manual-verification.md) 第22、35、57–63行将 crash/fault/process-exit、物理恢复、I/O 故障等列为执行限制，并明确测试源码可审阅和记录。所列恢复命令是保存的调用形状，不是已找回的原始被拒绝调用。
5. [测试范围审计](test-scope-audit.md) 第5–29行保留历史 namespace 测试曾执行的事实；第33–39行保留当前物理执行禁令。queued oracle 被额外谨慎阻断，不能据此推导源码编辑被拒绝。

ANN V4、41 目标 fuzz 与未知中断操作继续禁止重发、改写、拆小替代、换环境或迁移 CI。包括对同一 ANN V3 验证器进行“纯内存修正”以继续 V4，也不能作为绕行。既有物理故障、强杀、热恢复、namespace 干扰、磁盘错密钥/篡改/选中文件缺失、中断发布/rekey 执行限制不变。

## 三 逐项源码待办分类

2026-10-11 P0后续对账：下表公开恢复一行按本次有限source/model集成更新，
其他行保留其已有证据身份。第四至六节保留LIMIT/分类批次的历史时点；
具体新增范围及尚未闭合的组合/持久/CI门见第七节。

### 3.1 恢复 备份 WAL 与目录所有权

下表每行均为“①未定位；④未归因”，另外明确②执行与③源码状态。

| 剩余项及源码位置 | ③ 源码或集成待办 | ② 仍受限的执行 | 依赖及溯源 |
|---|---|---|---|
| public Open rollback 恢复 | 已接入 `sqlite/rollback_recovery.go` 的 nil-default Linux 可写明文有限 opt-in、pager retired-reader/path proof 和 driver 配置复制；独立 source/model/compile PASS，仍属实验性实现 | 实际 RecoverRollback、启用恢复的真实 Open/Connect、生产桥接均 BLOCKED / NOT_RUN；public physical harness 为 IMPLEMENTATION_MISSING | 依赖既有 retirement/admission、路径身份和有限记录数恢复，不依赖 ANN。旧V2 tree `5f2766d…`保留历史；本次独立候选 tree `75c074548e8b5208beb06e6ead314d835cd16f4f`，c150组合另验，见[专题](rollback-recovery-open.md)及第七节 |
| 更广 rollback journal 形式 | `internal/journal/replay.go:59–74` 仍排除 super/master journal、no-sync 记录数和旧式未指定页大小；真实解析/恢复兼容性缺口 | 物理 replay 受限；字节解析与磁盘执行是不同动作 | 需明确定义受支持格式与恢复边界。没有定位到该 parser 编辑的原始拒绝 |
| Linux rollback plaintext Backup V2 | `sqlite/backup.go`、`internal/pager/plain_backup.go`、namespace identity 和 CLI 接线候选未完成最终审阅/集成；main 明文 backup 仍 unsupported | 原生故障、namespace 碰撞、恢复/发布负向场景受限 | V1 P1 为源/目标 sidecar 家族重叠的工程缺陷；V2 局部修正不补齐终审。`plaintext-backup-ownership-review-v1/review.md:5–52`、`plaintext-backup-api-v2-report.md:20–49,61–84`、原交接 `:94–100,115`。V2 tree `d73faa3d653df5382b01ebc318145ed921833732` |
| WAL plaintext backup | 缺 WAL 快照/复制所有权协议与 API profile dispatch，不能只运行现有 rollback backup | 物理失败/恢复资格受限 | 依赖 WAL 快照、发布和目录所有权设计。`plaintext-backup-api-v2-report.md:83–84`；63c2 版 acceptance 第29行 |
| 非 Linux plaintext backup | 缺 Darwin 名称规范化、Windows alternate/short-name 身份策略；现有 V2 仅 Linux | 目标 OS 与物理负向资格分开；没有找到精确编辑拒绝 | 依赖 backend 的名称/锁/目录协议。`plaintext-backup-api-v2-report.md:38–49` |
| WAL-prefix V2 | 字节 scanner/proof/recipe 已有候选，`internal/wal/prefix.go:tailError.Unwrap` 的 V1 nil-child 缺陷经 V2 局部修正；精确 V2 独立复核待完成 | 字节模型不执行 native 尾部恢复；物理尾部处理受限 | `wal-tail-independent-review-v1/REVIEW.md:5–31`、`wal-tail-evidence-v2/RESULTS.md:7–9,27–36`、原交接 `:114`。V2 tree `715b15f50577b23d60b6848a3309fcba6ca491f9` |
| native exclusive WAL-tail publisher | `internal/pager/wal.go:openWALLocked` 所需尾部安全发布器尚缺实现；字节 recipe 没有生产 caller | native tail/namespace/fault/crash/发布流程受限 | 需候选创建、替换、sync、所有权转移及保留目录集成。`wal-tail-independent-review-v1/REVIEW.md:25–31,226–227` |
| retained-directory VFS | `internal/vfs/retained_directory*.go`、`file.go` 有未完成冻结/审阅的候选，尚未接入 pager/backup | raw namespace/原生 acquisition、replacement、故障资格受限 | 来源基于旧 a314 组合；不能整树替换。`retained-directory-work/docs/retained-directory-vfs.md:3–6,10–31,70–108`、原交接 `:118`。静态快照 tree `57d9bfbf6c9e853a14f750d527c4c7ecdd7a5978` 只证明保存 |
| 真正只读 plaintext WAL | `sqlite/sqlite.go:237–239` 拒绝 ReadOnly 与 native WAL 组合；当前 query 的可写协调及 named Check 不实现真正只读入口 | 异常 WAL/namespace/recovery 验证受限 | 需 DB/WAL/SHM 快照、锁和所有权协议；不能仅放宽 CLI 拒绝。`internal/pager/pager.go:94–110`、`cmd/gosqlite/main.go:35–52`、`cmd/gosqlite/check.go:67–72`，行号固定于63c2 |

### 3.2 SQL 与原生 backend

以下源码项同样没有定位到①，④仍未归因。工程缺陷保留原始 FAIL，有限候选 PASS 不自动成为当前 main PASS。

| 剩余项 | ③ 源码状态与具体依赖 | ② 执行状态 | 原始工程依据 |
|---|---|---|---|
| 已支持 SQL 的 planner/阶段/排序正确性 | V4c 与 main 新 grouped/counter/preparation 尚需精确 hunk 组合；涉及 CAST/IPK、LEFT-ON/WHERE、排序、绑定/执行阶段，不能写成只缺测试机 | 没有找到 planner 专属测试拒绝；本次只选择下节 LIMIT 转换，不恢复整棵旧候选 | `planner-parity-independent-review-v4c/REVIEW.md:3–7,47–56,77–79`；`planner-parity-close-v4c-evidence/HANDOFF.md:12–29,109–115`。旧 full patch 37路径；tree `552286a4985b1d9f339d8f59756b4eb2cad7071a` |
| Windows shared encrypted rekey | main `internal/encstore/platform_windows.go:81–87` 仍拒绝；候选 guarded lifecycle/authority 检查待按 hunk 集成，不复制整个 a314 | native Windows、namespace、错误密钥/篡改、I/O 故障、中断 rekey 均未资格验收 | `windows-shared-rekey-independent-review-v1/review.md:5–7,28–56,74–94`；候选 tree `7ede866de6a99f5390bc2f912e2e54f7d8c586d0` |
| Darwin V3 名称与诊断 | 受限 Unicode sibling admission、managed apfs/hfs gate、inventory 诊断保存未合入；不等于任意 Unicode root 支持 | 原生 OS、namespace 和故障场景 NOT_RUN | `darwin-unicode-independent-review-v3/REVIEW.md:5–20,30–54,88–90`；tree `62051a4f338d2e3de249694079b742ebab70c19a`。V1/V2 工程 HOLD 保留 |
| OS raw namespace 测试隔离 | 两项现有 Windows/Darwin 测试尚未移入 exact manual tag/env gate；需与 public-recovery guard 逐 hunk 合并 | 这两项物理测试仍受限；main 默认 Windows/macOS 全套不能直接启用 | `native-scope-independent-review-v1/report.md:3–30`；[中文手册](manual-verification-zh.md) §12.3。门控是源码待办，不能等同于运行许可 |
| 更广 Darwin Unicode root | 已有设计提案未实现/未接受；应按必要支持范围单独决策，不能虚构任意无限文件名为强制门槛 | 涉及的原生名称资格另行界定 | `darwin-unicode-sibling-evidence-v2/UNICODE-ROOT-DESIGN-NOT-IMPLEMENTED.md:1–4,60–65` |
| 分期 SQL/文件格式 | UNIQUE/CHECK/FK、CTE/UPSERT/view/trigger/window/JSON/FTS5/ATTACH 等按实际矩阵 PARTIAL/未实现；UTF-16/WITHOUT ROWID/autovacuum 按支持范围 fail closed | 没有找到这些编辑的原始拒绝；不能凭普通测试许可启动任意新功能 | goal §4.2；[兼容矩阵](compatibility.md)。分期状态不能豁免已声明支持域的正确性，也不能用新增语法替代基础恢复义务 |
| native VACUUM | 缺 parser/executor/storage rewrite 能力；官方 SQLite 的历史 VACUUM 互操作不能算原生实现 | 物理重写/恢复资格另列 | goal §4.2 第三批、compatibility 第39行（63c2）；依赖事务、pager、所有权及安全发布设计 |

### 3.3 其他工具 格式与测量

| 项目 | 四类判断 | 真实缺口及依赖 |
|---|---|---|
| encrypted inspect / 其他 CLI profile | ①未定位；②磁盘错密钥/篡改/中断发布验证受限；③有限范围与设计待办；④未归因 | 已确认的缺口是 inspect CLI 密钥/profile 路由，不足以认定底层所有 encrypted metadata reader 都不存在。63c2 的 `cmd/gosqlite/main.go:95–124`、`sqlite/inspect.go:40–96`、`sqlite/sqlite.go:294–301,493–503` 支持未来 snapshot-only adapter 的设计审阅；本轮没有批准或开始该候选 |
| 完整格式升级/回退覆盖与工具 | ①未定位；②中断/篡改/恢复资格受限；③支持矩阵与实现/覆盖未闭合；④未归因 | layout 1→2、quiescent 1/2→3 的已有 API 与有限普通测试保留。需明确实际支持的版本/profile/方向，再补必要转移、保留和回退工具；不能虚构“所有升级器均无实现”或要求任意降级边。`sqlite/encryption.go:179–245`、[格式说明](file-formats.md) 第162–178行（63c2） |
| WAL 恢复耗时测量 | ①未定位；②实际恢复测量受限；③工具及安全恢复路径未闭合；④未归因 | goal §11.3 的测量项仍在；普通 checkpoint/事务耗时不能代替。先有真实安全恢复入口和固定协议，才有可信测量 |
| performance V3 离线报告/准入工具 | ①未定位；②受限故障资格不因工具复用放宽；③源码集成待办；④未归因 | `performance-ci-independent-review-v3/REVIEW-v3.md:5–10,185–189` 在原限定范围未发现剩余源码 blocker；main 尚无 `benchmarks/performance/`。旧 tree `b04c112944f042a1987b24692a2eeae3aecb1443` 的 source PASS 不是性能 PASS。分离源码须重新核验精确 patch/tree；test_compare 会导入 run.py，不能声称无 runner 依赖 |
| performance CI 部署 | ①未定位；②不用于绕过任何受限执行；③明确操作/权限 HOLD；④未归因 | 同一 `performance-ci-independent-review-v3/REVIEW-v3.md:12–20,160–166` 禁止启用自动 push/PR 的8组配对任务。完整旧补丁包含 workflow；整补丁安装会启动行为。新 runner、额度/计费读取、费用与权限必须单独决定，未获新增许可 |
| 普通 transaction V4 工具与两小时 soak | ①未定位；②不能改造成 crash/fault/recovery 试验；③实际位置/资源预检和后来暂停；④未归因 | 工具已在 `benchmarks/transactions/main.go` 实现，长测 NOT_RUN。`long-soak-plan-20261010/INDEPENDENT-REVIEW.md:8–21` 的具体 blocker 为未固定 fixture TMPDIR 与空间门；`:45–77,114–117,145–151,172–184` 保留空间、fsync=volatile、协作式时限和读者聚合限制。没有启动新长测或用短样本替代 |
| ANN V3 报告 / V4 修正 / caller-dataset ANN | ①无法从未知动作认定；②没有新 native 测量资格；③V3 三项工程 FAIL；④V4 链路禁重试 | V3 缺 HNSW read conservation、IVF 全列表/源行数守恒、exact/rescore 距离一致性；`ann-harness-independent-review-v3/REVIEW.md:9–46`。V4 只有副本与回归探针，未修复；预期 normal/race FAIL 原样保留。不能以相同验证器的纯内存改写继续 V4。报告工具 FAIL 不自动证明生产 ANN 算法全错；精确距离验证不替代 ANN recall |
| 41-target fuzz 与未知22:39操作 | ①/②精确动作无法分类；③当时暂停保留；④明确 UNKNOWN / 禁重试 | 原交接 `:9–31`；不重提、改名、拆小、换输入、换环境或迁移 CI。既有 bounded fuzz PASS 只属于旧源码/旧运行 |

### 3.4 缺失的物理安全 harness

以下六项均为：①未定位源码编辑拒绝；②对应物理执行受限；③源码/协议/覆盖仍有缺口，先前全面暂停不能充当已证明的编码禁令；④整轮标记未归因。没有完整入口时继续记 `IMPLEMENTATION_MISSING`，执行另记 `BLOCKED / NOT_RUN`。

| 需要补齐的源码或协议 | 必要依赖与未运行证据 | 现有来源 |
|---|---|---|
| crash/process-exit/power-loss harness | fixture 协议、外部已提交/未提交账本、注入与恢复点、持久化条件、逐行/索引验证；有限 replay fixture 不替代真实掉电 | 中文手册 §12.4，第708行 |
| short-write/Sync/ENOSPC/permissions/wrong-offset/torn-write 注入后端 | 可丢弃设备/后端、失败点矩阵、影响范围与残留所有权；内存错误模型不能作为物理证据 | 同手册第709行；原规范 §11.1 |
| WAL crash/checkpoint/tail-publication matrix | 先补实际 publisher/恢复入口与失败语义，不能编造 TestCrash 命令 | 同手册第710行 |
| 磁盘错误 key/盐/tag/单字节修改/跨页/main-WAL 不匹配矩阵 | 确切测试目标、认证拒绝、无秘密泄露、来源保持；内存密码学向量与正确密钥路径不能替代 | 同手册第711行 |
| interrupted publication/rekey 与 generation 判定 | 完整失败点协议、旧/新代际与恢复规则，避免半新半旧误接受 | 同手册第712行 |
| selected-file 缺失/namespace 干预/原生 lock-unmap-close 失败矩阵 | 原生 OS/文件系统、diagnostic/retirement obligations；已有两项 manual namespace 不补齐矩阵 | 同手册第669–713行；test-scope-audit 第5–39行 |

现有 `TestManualRollbackRecovery` 和两项 `manual_namespace` 仍按[中文手工验收手册](manual-verification-zh.md) §12.1–12.2 的真实源码、标签、门、步骤、断言、证据与清理规则保存，当前 BLOCKED / NOT_RUN。没有补写虚假测试名，也没有把编写步骤记成执行。历史 M2 至 7addc493 曾误纳入两项 namespace 的 PASS 与不干净范围事实继续保留。

## 四 本检查点的独立源码批次

状态：**三文件源码限定普通/race及独立审阅PASS；精确组合18阶段完整普通门于07:39:21 UTC全部PASS。提交、恢复和CI仍待本批精确身份闭合。** 实际修前FAIL、最初oracle包装器HARNESS_FAIL及修后证据见[本批记录](evidence/limit-conversion-20261011.md)。

- 目标是原 goal G1、§4.2 已支持 LIMIT/OFFSET 的数值 affinity 与公开错误类别。生产范围仅 `sqlite/query.go:limitValue`，必要时移除该文件不再使用的 import，加专用回归源；不移植整个 V4c，也不替换更新后的 grouped/counter/preparation 文件。
- 在保留 Eval、完整值 preflight 和原错误传播的基础上，复用既有 `sqlast.ApplyAffinity(..., "NUMERIC")`；嵌入 NUL 文本遵循已核对规则；拒绝值保留 `ErrMismatch` 与 `CodeMismatch(20)`。旧 Go ParseFloat 文法、REAL MinInt64 边界及未包装错误为本批具体差异。
- 只读依赖为现有 numericText/ApplyAffinity、公开错误映射和 limitValue 调用方；没有 VFS、recovery、backup、ANN 或原生 backend 依赖。
- 验证仅在实际测试/helper/selector 逐项审阅后进行有界普通回归、固定 SQLite oracle 和相关 race。范围包括类型/端点/十进制/NUL、取消/内存/工作量/overflow 类别保持，以及完好 fixture 上的 Query 错误、nil Rows 和资源归还。固定 oracle 不是 fuzz；代码与未跑测试分别报告。
- LIMIT=0 对 OFFSET 的抑制、EXPLAIN 的 bound 求值抑制、完整 name/ON 绑定顺序、partial-index/cost/sort 及 WHERE/ON 阶段问题仍属另批真实源码待办。旧 V4c 的广泛回归不得删弱，也不能冒称已由这一个函数覆盖。
- 若发现必要依赖涉及被明确拒绝或仍无法识别的中断操作，停止该依赖并记录具体目标。后续批次逐项审阅；public rollback已另行开始限定源码集成和descriptor-free模型切片，未合入本检查点，真实RecoverRollback、启用恢复的真实Open和物理恢复仍NOT_RUN。backend、performance或encrypted inspect等未因这次分类纠正整体启动。

每个实际完成的批次仍须提交、保存证据并报告；新提交验证只属于它自己的源码身份。原始 goal 的实现、物理安全和外部资格未齐备前，不能宣布编码全完、M3/M8/M10 VERIFIED 或生产安全。

## 五 历史63c2检查点已闭合的证据

以下为已保存历史结果的对账，没有重新执行。它们不能验证新的 LIMIT 源码。

| 项目 | 精确证据 |
|---|---|
| 已保存提交 | `63c2ebb10e3a2c4ff2378ee9f33c46133134c70f`；tree `d735409ebd1c43d90ee1650c8a7c2a084e270674`；父提交 `7a9972547f49531df04cf1b624dfa662b483907b` |
| 完整普通组合门 | 另一个冻结 tree `d7393c254a6fca4452ae194d64d9045659f9781d`，04:23:35 UTC，18阶段自然 exit0；其后777个非docs文件字节/路径/大小/Git模式一致，不把该旧 tree 日志改成在63c2运行 |
| 当前历史提交 CI | [38111973546](https://github.com/ms-xiaosnow/gosqlite/actions/runs/38111973546)，04:55:57 UTC SUCCESS；ordinary `114389234612`、oracle `114389234442` 均成功 |
| Library 全历史 bundle | `libfile_ce332d051b4c8191a833e09094b5722e`，版本0，5,334,849字节；SHA-256 `5c0c18551f6874a6bc2e5766362e47308432424c366eec5eff448010376bfbf7`；字节读回与精确恢复核验通过 |
| 精确恢复 V2 | 04:41:10 UTC，36阶段自然 exit0；每模式23个顶层/40个子测试，普通及race，零skip/FAIL/空匹配；guard、vet/build、五目标、模块及身份通过。恢复未重跑完整oracle/全仓测试；不能扩大覆盖 |
| 原始恢复 V1 FAIL | 04:37:10 UTC 在首个身份阶段 exit1，尚未运行Go检查。验证器额外排除32个非docs Markdown，得到745而断言777。V2只修筛选谓词，未改源码/测试；原脚本/FAIL/诊断保留 |
| 终态证据归档 | `libfile_7fea2541ad7881919b7236a66499c138`，版本0；2,576,930字节，SHA-256 `d1a8e98dae0d1260006f75c938360e04b3b40744b69b1c7d1b4e286800e6bd45`；378条tar entry/377文件哈希已核验，包含历史收尾报告与恢复/CI证据 |

证据归档的字节身份只证明保存，不替代运行验收。Git clone 曾对30个文件做常规 POSIX 权限归一化，字节与 Git 可执行模式一致；不能扩大为全部原始 POSIX 元数据相同。历史 CI Node.js deprecation warning 也没有因 SUCCESS 被宣称修复。

原始失败仍保留：rollback 20秒样本 ErrChanged、transaction V2/V3、Native Check V1、distance V1、planner V3/V4/V4b、backup V1、WAL-tail V1、ANN V1/V3、performance V1/V2、Windows cross-vet unsafe.Pointer 告警，以及 grams/ANN 慢于 scan/exact 的实测。对应后续有限 PASS 不抹去失败、原范围问题或缺失的资格。

## 六 可追溯材料与核对范围

本次读取项目当前文档、原始中断交接、相关候选/独立审阅、现有源码和三份只读溯源报告；只做文档编辑、文件/链接/哈希核对，没有执行项目代码、测试、build、数据库、候选脚本或新的外部动作。证据范围限于可访问的已保存文件，没有将未取得的原始拒绝补写成事实。

| 已保存材料 | 定位与身份 |
|---|---|
| 原始中断交接 | `INTERRUPTION-20261010T2239Z.md:9–31,94–123,147–169`；Library `libfile_0958d62d04dc8191bdb2ea277785b416`；11,762字节；SHA-256 `13092da4ed0e6ca3d7633dc4951a9ac5d0ab18060f6bbd55a4330d39a3a2a5d2` |
| planner/backend/目录溯源 | `restriction-provenance-planner-backends-20261011.md:20–29,31–114,116–138`；SHA-256 `2d23d39a2d961485b15c3b61211cdb53b9e7ae77c5c254f6df8e924a363f5ef3` |
| recovery/backup/WAL/harness 溯源 | `restriction-provenance-recovery-20261011.md:26–99,135–141`；SHA-256 `a49cef17ccd32552070c6f8153a5e34535a61a7f7fb2bd014e3f78a0c5897cc6` |
| 其余能力溯源 | `restriction-provenance-remaining-20261011.md:13–35,54–56`；SHA-256 `546507c0a9d1e8304cce783b934377744fa33bbf7951c0a7c8447a0342345410` |
| 旧有限收尾及终态证据 | `finite-closeout-handoff-zh.md` §1–4、§8；SHA-256 `718a78becd0ea59d47b14bbee53235c47e207076d7141467002eb71bd4a1a52d`；归档身份见上节 |
| 本次纠正的旧结论 | 63c2 版 `docs/closeout-scope-20261011.md:20–30,50–74`、`docs/acceptance-status.md:105–117` 与 progress 原有限收尾段；这些旧文字保留在原提交历史，新状态以本页及最新进展为准 |

表中非仓库路径是原工程证据文件名，不能假定仅 checkout main 就已取得这些外置材料。此前 Library 源码归档与后续日志归档覆盖不同：30705保留旧组合，276312保留六份静态候选；均不证明未合入候选已验收。后续需要时应按实际文件及哈希核对，不能以归档存在替代缺失审阅。

原始 goal.md 的 SHA-256 仍为 `9ae7b1d776856669a10eb753408c7754d015d266f8783240165450e96e4bac81`，43,100字节。用户自测步骤以[中文手工验收手册](manual-verification-zh.md)、[CLI资源限定复验](cli-resource-verification-zh.md)及[五例限定复验](example-retention-verification-zh.md)为准；本分类纠正没有改写这些步骤或解除其限制。


## 七 2026-10-11 P0 public Open 后续源码与模型更新

本次有限公开恢复源码在已保存 c150 上逐 hunk 组合，保留该提交的 LIMIT
和分类文档。候选原基点63c2，独立复核 tree
`75c074548e8b5208beb06e6ead314d835cd16f4f`，13项命令阶段自然 exit0，
末次 race 07:49:34.878759 UTC、身份复核07:49:35.178221 UTC完成。
11个精确无描述符测试在CGO0与相同race选择均PASS，guard、限定vet、
全包CGO0 build、无CgoFiles闭包及精确补丁重放PASS，未发现阻断源码问题。
这不是完整 ordinary/oracle 组合验收；c150加P0的新组合、新提交恢复和CI
仍须按各自身份取得结果，不能继承旧基点或候选的PASS。

真实API是 `Options.RollbackRecovery *RollbackRecoveryOptions`，nil继续拒绝
已有journal；Linux可写明文rollback的非nil配置只允许一次恢复及一次新的
no-create probe。默认拒绝、`errors.Is(err, ErrRecoveryRequired)`和CodeIOErr
保持，journal诊断文本与私有包装有变化。其他SQL/Reader/Tx入口没有新增恢复。
完整API、三个预算及错误/所有权界限见[公开恢复专题](rollback-recovery-open.md)。

公开物理harness仍为IMPLEMENTATION_MISSING；现有内部manual_recovery
测试不调用新public bridge，不能作为公开入口验收。实际RecoverRollback、
production bridge及开启恢复的真实Open/OpenContext/Connect均BLOCKED / NOT_RUN。
默认开启、更广journal、WAL publisher、Darwin/Windows恢复及原生失败资格
没有被这一源码/模型PASS补齐；ANN V4、41目标fuzz和未知链路的禁重试不变。
中文第18节记录实际配置和未来断言；缺少入口不编造可运行命令，旧第12节
的已存在手工命令保持原门控和限制。
