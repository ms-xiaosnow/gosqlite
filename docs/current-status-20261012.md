# 当前开发状态与原目标边界

## 当前状态（2026-10-13 03:37 UTC）

本轮提交前已核验的发布基点为 `fd069d7a30b2e0ca8bdff67ca54c926aa118fc51`；CI `38339706432`
的 oracle 于 **03:09:22 UTC**、ordinary 于 **03:30:31 UTC** 均为 **SUCCESS**。
这些结果只绑定该精确提交。新增 PG 风格 ANN DDL、离线性能比较器的有限结果绑定
本轮证据清单内精确文件；发布及对应 CI 按新提交另行核验，不能继承 fd069 的全套 CI 资格。

- goal §7.3 的两条精确 PG 风格 `CREATE INDEX ... ON ... USING hnsw/ivfflat ...`
  示例已接入既有 ANN 引擎。四个源码/测试路径的有限验证：8 个 parser 根共 100 named，
  加 2 个健康数据库根共 4 named，normal/race **各 104 PASS / 0 SKIP**；source guard **PASS**。
  范围仅为单 dense 列、已声明方法/opclass/options，不代表 PostgreSQL 方言全面兼容。
- goal §11.3 的离线性能比较器六路径已完成独立源码审阅 **CLEAR**，57 个纯离线 fixture
  于 **03:37:08 UTC PASS**。仅在既有 `scripts/check.sh` 接入 unittest；没有新增 runner
  或性能 CI job，没有实际固定基线或实测性能验收。见[离线比较器合同](../benchmarks/performance/README.md)。
- R1–R6 的六个有限设施项已闭合；这不是整个 goal 的完成证明。本次有界通读原目标后
  补齐上述两个可操作缺口及当前文档口径，不把 goal §4.2 的分阶段 SQL 能力或在线、
  依赖完整的 PG collector 自动扩成这批新义务，也不声称所有编码或全部 SQLite 兼容已完成。
- 仍有 ANN 报告的 HNSW 计数、IVF 全 lists membership、同 query-row rescore 三项未闭合；
  整轮安全中断的 UNKNOWN 与 ANN V4、41-target 持续 campaign 的既有阻塞保留。
  物理手工、真实 OS/FS、真实 PG/Neon 输入、受控性能/长时 soak 和独立安全/许可结论分别待取得。
  手工总表固定为 **17 组：13 个核心组、82 个核心场景，另 4 个原生平台组**，全部手工仍 **NOT_RUN**。

[本轮精确证据](evidence/20261013-goal-closeout/README.zh.md)与[有限剩余及权威手工总表](testing/finite-acceptance-summary-20261012-zh.md)区分源码、有限测试、
精确提交 CI 和物理资格。**整体未验收，M3/M8/M10 未取得总体验收；不作最终交付或生产可用声明。**
以下历史段落中的“当前”、PENDING、HOLD、FAIL、UNKNOWN 与 NOT_RUN 均绑定原时点/原树，
原始失败与未运行事实保留，不是本轮状态，也不被新 PASS 回写。

## 当前受支持能力的窄修正

- 普通 rowid 列/表 BINARY ASC automatic UNIQUE 和规范 autoindex 已实现；有界 scalar CHECK
  已实现。见[UNIQUE 合同](automatic-unique.md)与[CHECK 合同](check-constraints.md#精确-partial-profile)。
  FK、STRICT、WITHOUT ROWID、一般函数 CHECK 和未列 SQL 不因此开放。
- [有界 IPK equality/LIKE](evidence/readonly-ipk-combined-20261011.md)、[严格 Linux rollback Backup](plaintext-backup.md)、
  [Darwin 受限 inventory](darwin-unicode-inventory.md)、[Windows shared rekey](windows-shared-rekey.md)
  均已有实现；旧文的 unpublished/implementation-missing 只属于当时。
  Backup 的成功原生发布、Darwin/Windows 原生、真实只读介质和物理资格仍 NOT_RUN。
- journal 的 nil-default refusal 保留；[显式 RollbackRecovery](rollback-recovery-open.md)
  已有 finite-only opt-in 实现和设施。EOFUnproven 不准入，实际物理恢复未获资格。


## 历史记录（以下保留原时点事实）

# 历史阶段：三设施检查点当前验证结果

受测修正树`51f63a56c22ab6396d5b542a1f0655a82dad4cd5`已取得实际
race **10928 PASS／7 SKIP**，以及source guard、metadata根、testscope、vet、build结果。
44909在race自然process0后按缩减验证范围暂停，记录器exit2；不是测试FAIL，也不是22门PASS。
后续2054的四平台支持包编译、whole依赖清单、双manual-tag compile／nm共7项自然PASS。

ordinary为明确组合覆盖：新metadata1＋testscope960＋复用9967 PASS／7 SKIP。
oracle采用main750未变部分、当前guard／source根和原78992模型的组件复用，未新跑完整脚本。
旧0ef／78992的真实FAIL、补充v1启动前KeyError以及全部历史FAIL／UNKNOWN均保留。

[私有仓库有界工程证据](evidence/20261012-rollback-facilities/README.zh.md)提供精确命令、
原始失败、工具/source身份及复用映射。新提交、bundle、必要恢复smoke与CI仍PENDING。
manual映像未执行，真实fault／recovery／process-exit NOT_RUN，M3/M8/M10未完成。
[有限余缺与M9最低范围](testing/remaining-goal-boundary-20261012-zh.md)不按百分比或工时推算。

## 历史原文

以下冻结源原文逐字保留；旧FAIL/HOLD/UNKNOWN不回写，v1至v3草稿另存。

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

> Combined readonly acceptance V2: SOURCE_ONLY; the reviewed media owner-field correction and exact independently reviewed three-main-pin migration are applied in this fresh candidate. No combined validation or physical acceptance. See [source-only integration status](readonly-acceptance-integration-source-20261012.md). Historical isolated evidence below is not a combined PASS.

# 当前源码、已取得证据与真实余缺（2026-10-12）

## CHECK / EOF 检查点：完整正常门已通过（2026-10-12 07:22 UTC）

冻结运行树 `e06f35a022536a8ebd157731ee9a860521e70eb0` 于 **07:22:18 UTC**
自然完成 **26 门 / 28 次 Go 调用 PASS**，session 80563 exit 0，lease RELEASED。
ordinary/race 各 **8,948 named PASS / 7 个明确 SKIP**，完整 pinned oracle
**10,687 PASS / 0 named SKIP**。本副本只增加状态文档，运行源码与该冻结树一致；
本副本的新提交、bundle、精确恢复运行及对应 CI 均 **PENDING**。

CHECK 仅资格[已记录的有界 scalar 子集](check-constraints.md#精确-partial-profile)。
EOF-count 仍仅为 decode-only、EOFUnproven；native owner 在写入/Sync 前继续
finite-only fail-closed，未开放 EOF 恢复。外平台只有编译/符号检查，原生手工、
物理持久性/恢复、安全及 M3/M8/M10 未验收。后续 Darwin/Windows backend 和
真实只读媒体 harness 未纳入；其 source-only/HOLD 不被本次 PASS 覆盖。

已发布基点 `e092f0d4623cb243eec6053a4b3db7eef598542f` 的 bundle、精确恢复
26 门及 CI 38209370406 两项 SUCCESS 已另行闭合，只资格该基点。
[本检查点中文报告、精确收据与未完成项](evidence/check-eof-checkpoint-20261012.md)。

从下一节起保留各阶段原文；旧“当前”、SOURCE_ONLY、NOT_RUN、PENDING 与
FAIL/HOLD/UNKNOWN 均只指原时点和原树，不能覆盖本节的新执行记录，也不会被回写。

## 当前 CHECK / EOF-count 源码组合（2026-10-12）

本副本在已发布检查点 `e092f0d4623cb243eec6053a4b3db7eef598542f` / tree
`d8ff20a26b4aee9fe3281a426e41230b18c727cd` 上，仅合入已审阅的有界 scalar CHECK
与 decode-only EOF-count 增量。组合仍为 **SOURCE_ONLY / ALL_COMBINED_EXECUTION_NOT_RUN**；
前驱 V1 的 sqlite.go pin HOLD 原样保留；本 V2 已按独立完整文件 ownership/consumer
复审，仅更新 WAL publisher guard 的该一个 hash，未运行 guard。没有运行本组合的 Go、formatter、compiler、guard、tests、
race、oracle、native、restore 或 CI；输入有限 PASS 不转授本组合。

CHECK 原 tree9c3b 的三普通门已取得 18 根 / 28 named PASS，有限 oracle 为 3 根 / 7 named
PASS。EOF 原 tree1f599764 的四普通门终态记录为 38 Test 根加 2 个固定种子 Fuzz 根、
485 named PASS；只资格其具名内存/源码范围，不是物理恢复。已发布基点的精确恢复运行
与对应 CI 仍待独立闭合；本源码组合尚未发布。准确身份、原始收据及下一步见
[本组合范围与证据边界](check-eof-integration-20261012.md)。

CHECK 仅支持普通 rowid 表的有界、无显式函数 scalar profile：保留折叠前 source
准入、NULL-pass、UPDATE 赋值依赖、NOT NULL → CHECK → UNIQUE/storage 顺序及原
storage.Statement undo。DB.Check 只在 primary scan 每行评估一次；配额仍是累计 work
2^24、metadata 16 MiB 并受 MaxSortBytes 约束、2,000 声明和 8,192 原始节点；一般
函数、FK、STRICT、WITHOUT ROWID 与更广 SQL 不因此开放。详见[CHECK 合同](check-constraints.md)。

EOF-count 只解码 offset 0 第一且唯一 published `0xffffffff` 头；raw Header 保留，
有效数量为 len(Pages)，CountEvidence 明确为 Unknown 零值、FiniteHeaders 或 EOFUnproven。
完整 sector 后余量必须恰为完整记录，配额先于 metadata/图像分配；整条尾记录截断仍可能
结构合法，checksum/extent 不证明事务完整、认证、writer 来源或 durability。native
nil-default 和 opt-in 均不恢复 sentinel；同一新 plan 必须在 extent/restored-header/Sync/
写入前通过 finite-only admission，拒绝 unknown/未来枚举、EOFUnproven 及矛盾 raw sentinel。
writer、later sentinel、legacy page-size-zero、super/master 的原拒绝边界不放宽。
详见[内存解码](rollback-replay-memory.md)与[原生准入](rollback-recovery-open.md)。

以下保留各输入与检查点原时点记录；其中“当前”、未合入、NOT_RUN、PENDING 和 PASS 均只绑定原树，不覆盖上面的本组合状态。

本页是当前状态入口，区分源码、有限执行、完整运行、发布/恢复/CI 和真实物理资格。旧文“当前”“缺失”“待合入”与 PENDING 只保留原写作时点。历史 FAIL、HOLD、UNKNOWN 和 NOT_RUN 不回写；原始 goal 与 M0–M10 目标不变。

## 五切片组合：完整正常 26 门 PASS，发布/恢复/CI 待完成

冻结树 `59ce26879c4301babc7648fbddc5cc66c4097263`，3,267 文件、783 个 Go 文件；基点 main `8bc189474bdb2b0a683c6c059992f9acbc426250`。组合包含 LEFT/Rows、encrypted inspect、Darwin inventory manual、Connector rollback manual 和 automatic UNIQUE（c382 空结果表头 helper 修复）；**CHECK 未纳入**。独立 source review 已接受该精确组合，两个旧 source-pin HOLD 另行保留。

`combined-checkpoint-normal-verification-v1` / session 93750 于 **2026-10-12 05:21:06 UTC 自然完成 26/26 门、28 次 Go 调用 PASS**；工具 exit0、chunk 947027，lease RELEASED，零强制终止、零仍活跃观察子进程。ordinary/race 各 **8,569 named PASS / 7 个固定预期 SKIP**（1,436 Test、41 Fuzz、6 Examples、313 固定种子）；完整 pinned oracle **10,301 PASS / 0 SKIP**（1,551 Test、41 Fuzz、6 Examples）；LIMIT oracle/race **394 PASS**。这些是实际结果，不再是 EXPECTED。

guard、四条 quick、live-writer 只读 oracle、vet、CGO0 build、五目标生产编译、模块与两条依赖闭包、三目标 test-c/nm 均 PASS。完整非测试闭包为 207 包 / 1,347 输入；shipping CLI/library 为 164 包 / 1,254 输入，未到达 testoracle/testscope/os/exec，C-family/Swig/Syso 为空。外平台仅编译与读符号，没有执行外国映像。5,689 个资源样本均在原门限内；源码、工具、依赖及历史输入前后身份检查通过，读取 atime 不声明不变。

新检查点提交、Git bundle、精确恢复 full26 与对应 hosted CI 均 **PENDING**。tree59ce 未发布源码包 `libfile_28d074da55e08191a43edef799057a45` 已逐字节/模式/归档树读回；该源包保存早于 full26 终态，不冒充包含完整运行证据，也不是 Git bundle。本次静态文档叠加没有改变已运行源树、命令或门限；[组合证据与来源](evidence/combined-checkpoint-20261012.md)记录完整身份。输入旧结果没有转授，本次 PASS 来自此精确组合的新运行。

后续独立状态补充（2026-10-12 05:27:58 UTC）：CHECK tree `9c3b0f09c1948c2c8b66337643677697d54852dc` 的 session 90221 已自然完成单条有限 oracle，**3 根 / 7 named PASS，零 SKIP/FAIL**，工具 exit0、lease RELEASED。这补充下文 05:22:24 UTC 的未运行快照；CHECK 仍未合入本组合、未发布，不把该有限 PASS 转授 tree59ce、恢复/CI 或物理资格。

完整终态归档补充：tree59ce full26 终态包 `libfile_39e4c60cdfac8191b3b21d2da9cd2356` 已完整读回，32,581,319 字节，SHA-256 `156be5b9b2a914e4843999969a6e7a0f815cc5cde2be7661b9b4e803900de1a6`；610 个成员的字节/模式、GZIP CRC/EOF、静态终态及原始冻结输入不变检查通过。这保存实际运行证据；新检查点 Git bundle、精确恢复及对应 CI 仍 PENDING。

## 当前能力矩阵

| 事项 | 当前源码 | 证据与仍缺内容 |
|---|---|---|
| Storage / Linux 明文 Backup | 已有严格 Linux rollback Backup API/CLI、受检目录与发布协议；PARTIAL | main8bc 有完整普通/恢复/CI 证据；真实手工 Backup（含 UNIQUE 数据库）NATIVE_NOT_RUN。更广文件系统、平台、WAL 与 encrypted 模式仍有未实现/未资格范围，不能称为完整 backup 矩阵 |
| 真正只读 plaintext WAL | 已有 Linux 活动、已初始化 SHM 的只读 Open/CLI 路由；PARTIAL | 基点具有限与完整正常/live-writer 证据；cold/no-SHM、其他 OS、真实只读媒体/非特权身份与物理失败仍未取得资格 |
| Linux WAL-tail publisher | 显式默认关闭 policy、源码/有限模型已保存；PARTIAL | native 发布 NATIVE_NOT_RUN，模型与正常完整门不构成安全发布/持久性验收 |
| Windows / Darwin backend | Windows shared rekey、key-session/provider、retained-directory 与 Darwin inventory 生产修正已在基点保存；PARTIAL | 不再统称“未合入”；跨编译/符号不等于真实 OS/文件系统/硬件原生资格，相关 namespace/manual 原生仍 NOT_RUN |
| 显式普通 UNIQUE | 普通显式命名列 BINARY ASC CREATE UNIQUE INDEX 子集已在 main8bc 保存 | 基点完整 26 门、精确恢复与 CI 通过；不扩展到表/列自动 UNIQUE、一般表达式/排序规则或完整 SQLite SQL |
| LEFT WHERE 预过滤 | 本组合已接入已审阅源码：单个根 LEFT JOIN、两个普通表、原 WHERE 全部为左槽/普通字面量等值 AND，保留完整 WHERE 复核 | 输入 V4 有有限 4 门；本精确组合完整正常 26 门已 PASS；原生手工/物理资格不由此扩展。参数/混合/error-capable WHERE、多重连接、一般 CAST/表达式、OFFSET/投影/排序阶段及广义 planner 差异仍未闭合 |
| encrypted inspect CLI | 本组合已接入 provider 与 snapshot/wal/shared 路由、一次 Close attempt 后输出和固定加密诊断；PARTIAL | 输入同树两段有限 9 门证据闭合，原第一段 FAIL 保留；本精确组合完整正常 26 门已 PASS。schema/config 仍可能含应用信息；未证明索引正确、未知 native owner 已退休、wrong-key/tamper/恢复/其他 OS 原生安全 |
| Darwin inventory manual | 本组合已接入独立双门 harness、source-pinned guard 和 main hook；有限源码已存在 | 输入 19 项 selection/compile/nm 由原输出与独立纯解析补证闭合，原 stage19 observer FAIL 保留；不是新完整 19 门成功运行。harness NATIVE_NOT_RUN，有限 metadata payload 不等于加密格式或完整 Darwin 原生验收 |
| Connector rollback manual | 本组合已接入四场景 harness 与独立纯源码 guard；有限源码已存在 | Rows pin 已经完整独立源码复审并在本后继窄修。原 Connector 输入已实际取得六项静态 PASS：2 个 AST 根加 26 子项、3 次 list、compile/nm 的 404 required / 13 absent；这些结果不转授新组合。真实四场景 Connect/recovery BLOCKED / NOT_RUN。手工源码存在不授权任何物理执行 |
| public rollback recovery | 有限 Linux 可写明文 Open/Connector opt-in、nil 默认拒绝、public Open 手工入口已保存；PARTIAL | 更广 journals/default-on/其他平台与完整 fault/crash/restart harness 仍有实现缺口；实际恢复仍按独立门 BLOCKED / NOT_RUN |
| automatic UNIQUE | 本组合已纳入普通 rowid 列/末尾表级 BINARY ASC UNIQUE、有序去重/编号、真实根/NULL SQL、精确 owner 和原子 CREATE；完整 schema 受 16 MiB / 32,767 行准入 | c382 原 gate11 E2 已有限 3 根 / 9 named PASS；普通/driver/CLI 等输入证据各属原树，本精确组合完整正常 26 门已 PASS。一般表达式/排序规则、广义 SQL 与 native Backup 自动索引数据库仍未获资格 |
| CHECK 与更广 SQL | CHECK 独立 tree9c3b 有有限源码及普通证据，未合入本组合，也未发布 | CHECK 三普通门 18 根 / 28 named PASS；截至 05:22:24 UTC，oracle 已批准但尚未运行。FK、CTE/UPSERT/view/trigger/window/JSON/FTS5/ATTACH/VACUUM 与 UTF-16/WITHOUT ROWID/autovacuum 等继续按兼容矩阵记录真实未实现/部分实现 |
| ANN 报告与测量 | ANN V3 的三个报告准入 FAIL 保留；V4/更广测量未闭合 | ANN V4、41-target fuzz campaign 与未知中断链维持 HOLD/UNKNOWN/BLOCKED；不得改名或另起同义链恢复。旧算法通过不消除报告 FAIL，报告 FAIL 也不证明全部算法错误 |
| 完整格式/物理安全设施 | 全受支持模式升级/回退、WAL 恢复耗时、完整 crash/power-loss/short-write/Sync/ENOSPC/torn-write/认证负面矩阵未闭合 | IMPLEMENTATION_MISSING 与已有入口的 BLOCKED/NOT_RUN 分开；不能编造已存在的完整 harness 或写成“只差人工运行” |
| 真实外部资格 | OS/架构/文件系统/持久性硬件，独立安全/密码学/威胁模型与许可/法律/API 审查，长时间/大数据性能和授权 PG/Neon 验证仍开放 | 普通正常门、交叉编译、离线迁移、短测和代码归档不能替代；新 runner/权限/费用需要另行授权。M3/M8/M10、生产安全与原始 goal 未完整验收 |

## 已保存基点的实际持久闭合

下列事实只属于 main8bc，不属于本组合：

- 提交 `8bc189474bdb2b0a683c6c059992f9acbc426250` / tree `c0959fb87e8aaedd946e6bb7abbaddb67cf5ced0` 已发布并保存。源码 bundle Library ID 为 `libfile_cbcb5098b9fc8191b818e0e7af348546`，SHA-256 `d8bd36da36e505292be417e2cc39a9ed64c42078473eaf82aafd0d4686adda87`。
- 精确恢复 V2 在 2026-10-12 02:16:40 UTC 完成 26 门 PASS；ordinary/race 各 8,231 PASS / 7 个预期 SKIP，完整 pinned oracle 9,925 PASS / 0 SKIP。completion SHA-256 为 `a71a444c115a2dc7a6b9c1246da4cdd755cafe704812284c966e224178269013`。终态归档 Library ID `libfile_1d6c010c88e88191837aadcecb23649a` 已完整读回。
- CI run `38190480842` 终态 SUCCESS；ordinary job `114621985040`、oracle job `114621984795` 均 SUCCESS。CI 终态归档 Library ID `libfile_3d72dca130188191af17abbcb7eb3c5b` 已读回。
- 下文旧“publication/bundle/restore/CI PENDING”保留为作者时点历史，已被上述该精确提交的持久事实补充。旧 Storage metadata 原始真实 FAIL、恢复观察器失败与其他 UNKNOWN 仍保留，不能抹平为一次从未失败的成功流程。

## 五个输入切片的有限证据，各自绑定原树

- LEFT V4 tree `abdcb5e13eda8ab11c49a6b18d1829352bd42ec9`：finite4 PASS，ordinary/race 各 338 named PASS，oracle 17 named PASS；原 formatting STOP 保留。官方子进程数量属于 source-derived accounting，未额外声称逐子进程独立收据。归档 Library ID `libfile_e7204c62cc208191a6e151a4bba6045e` 已完整读回；不能称为新 main 组合或 full suite PASS。
- inspect tree `1969514118a82f1ec1730ad6e32e7d7909d17430`：第一段 gates1–7 PASS、gate8 原 FAIL、gate9 原 NOT_RUN；同源后续 gates8–9 PASS，有限 9 门按两段闭合。ordinary/race 各 136 named PASS，31 个不同根。Library ID `libfile_4b00bcebae60819199eca2a35cd86c1f` 已完整读回；不是一次 all-PASS 运行。
- Darwin tree `8a9c62c53fdb818ae7881ab69db0c61b66ac8c39`：9 次 selection、5 次 compile、5 次 nm 原命令自然 exit0；原严格 observer stage19 FAIL_TEST_VALIDATION、总流程 FAIL_TEST_OR_IDENTITY 保留。对同五份 nm 输出的单字符官方符号语法补正通过独立纯解析复审，仅形成 supplementary static closure；零原生映像执行。
- Connector tree `029344a38b91960eacd06bad156a3e50ed174129`：原 v1 完整源码复审和 v3 格式/中文窄后继复审通过；固定 formatter 已完成。2026-10-12 03:36:59 UTC 六项静态流程自然完成 PASS：2 个 AST 根 + 26 子项 = 28 named PASS / 0 SKIP / 0 FAIL，default/oracle/manual 三次 list，compile-only 与只读 nm 核对 404 required / 13 absent。completion SHA-256 为 `ee2bafe214a6680f80204b89ea0d2096c8df577a91e7b1244f51b1c36b561a6d`；工具会话 85689 自然 exit0、chunk `bdde1d`。未执行手工镜像、fixture、Connect、恢复、oracle 或 race；旧 executor UNKNOWN 保留。该输入静态成功不转授本组合，不构成物理恢复/持久性验收。恢复后工作 journal 的有意消耗和原始输入快照保留必须区分。

- automatic UNIQUE：原 cf1e 的 read-only keeper fixture gate08 FAIL、7dd6 的五门 PASS / gate11 空结果表头 FAIL 均保留。修后 c382 tree `c3824a47a9fc03a9c1808bf62e84bf924b0e771e` 的 E2/session 90085 于 04:40:44 UTC 完成原 gate11：3 根、6 子项、9 named PASS / 0 SKIP / 0 FAIL，lease 自然释放。E1/session 76084 的初始化 FAIL（零 Go）不被覆盖。289 个官方进程结果仅为源码推导的完整 PASS 路径计数，不是逐子进程独立收据。

## CHECK 的独立边界

CHECK tree `9c3b0f09c1948c2c8b66337643677697d54852dc` 的三条普通门已在 session 27563 自然 PASS（18 根 / 28 named，零 SKIP/FAIL）；截至 05:22:24 UTC，oracle 已批准但未运行；随后于 05:22:42 UTC 建立 holder 90221 等待独立 release；full26 已自然结束并释放 lease，后续仍须自己的准入和终态。它未并入本组合，也未发布。CHECK9c3b 与 c382 源码备份 `libfile_74dc1f89dabc8191970b909821546cb5` 已读回、逐字节/模式/归档树核验；备份的早期 NOT_RUN 状态按当时保留，不替代后来的运行收据，也不算发布或恢复运行资格。

CHECK 三普通 PASS、automatic UNIQUE 修后 oracle PASS、原 7dd6 oracle FAIL 和 E1 初始化 FAIL 的四终态归档 `libfile_af0b03bfd1148191bdb207cab09a77d2` 已完整读回（1,025 成员 / 965 文件），不删除任一失败。它是有限收据归档，不是 tree59ce 的完整运行终态包。

## 不变的执行与验收边界

scripts/check.sh、scripts/test-oracle.sh、workflow、模块和 isolated oracle factory 保持原字节。本轮只执行已审阅的正常 full26；固定种子不是 41-target fuzz campaign。Darwin `darwin && manual_darwin_inventory`、Connector `manual_recovery && linux` 以及既有 Backup/WAL/recovery/namespace 手工双门未开放。外平台 test-c/nm 只资格编译与符号，不能记为原生执行。

原生手工/物理资格、真实只读媒体与非特权身份、完整 fault/crash/restart/power-loss/short-write/Sync/ENOSPC/torn-write/认证负面矩阵、全受支持格式升级/回退和长时间/大数据性能仍有未实现、BLOCKED 或 NOT_RUN 项。ANN V4、41-target campaign 与未知中断链不重启、不改名绕过。M3/M8/M10、独立安全/密码学/法律审查及原始 goal 均未完整验收，不能写成“只差人工运行”。

## 历史源码准备与失败记录

以下原快照保留精确源身份、pin 决策与失败，所有“当前”“未纳入”及“全部 NOT_RUN”均指各自准备时点，不覆盖本文顶部及当前能力矩阵。

### 五切片源码准备快照（完整运行前）：SOURCE_ONLY / NOT_RUN

在已审阅 v4 tree `25a5fcb4215a9c532834b13b260c62695db97762` 上按原基线 `4008bd9442ffd21e1e993d4f1c72dbeed3c910f3` 的精确增量加入 automatic UNIQUE tree `7dd67b039ecfdfddafe28f2ee5b6745ab8cb40ed`。main 仍是 `8bc189474bdb2b0a683c6c059992f9acbc426250`。再接入独立复审通过的 automatic UNIQUE c382 后继（`c3824a47a9fc03a9c1808bf62e84bf924b0e771e`）的唯一 oracle helper 修复。没有并入 CHECK；FK 与更广 SQL 仍按原范围。下方完整 v4 状态快照保留为历史，不把其“未纳入 automatic UNIQUE”写作时点当作本组合当前范围。

- 自动 UNIQUE：普通 rowid 列/末尾表级 BINARY ASC UNIQUE、有序去重与编号、真实根和 NULL SQL、精确 owner 绑定、compound CREATE 原子性、完整当前 schema 的 16 MiB / 32,767 行准入。原 UNIQUE DML/Check/statement undo 复用，CHECK 实现没有变化。
- 合并：20 个 production/test 路径没有与 v4 的代码交集；四份共享旧文档逐 hunk 合并，保留 main 的 full26、恢复/CI 和各切片历史。LEFT/Rows、encrypted inspect、Darwin/Connector harness 生产和手工门原字节保留。
- 已独立复审的 source-pin 后继：前序 d79 tree `d79cd45d10a14095a337fdb61bf65063ee1dbcef` 与其 35/36 pin HOLD 原样冻结。独立复审对实际组合完整 sqlite.go、Open/Schema/Close/owner、WAL/recovery 依赖与其他 35 pin 给出 `SOURCE_ACCEPT_EXACT_SINGLE_PIN_SUBSTITUTION_ONLY`。本后继只将 `walPublicationProductionBoundaries["sqlite/sqlite.go"]` 从 `0ee79f959b53d61fee15dc9ae36618863163790c55b68a056f66acef188cba31` 更新为 `0ea98f4f06300a819107dd2229dee9a2964142e4542bb2e582488749e056bf03`；整个 guard SHA-256 为 `de211120809b927cd7971161d214bd599f1de3a23391eaf57a5bf36f4aefc3d6`，其余 guard 字节不变，逆向单替换精确还原原 guard。36/36 静态匹配不是执行 guard PASS。
- 原 automatic UNIQUE 7dd6 修后六门：03 vet、07 memory、08 healthy、09 driver、10 CLI 已 PASS；11 selected oracle 自然 exit1、FAIL_TEST_VALIDATION。ConstraintHistories/page4096/sharedfalse 与 sharedtrue 在 history row57 出现 native counters header 对 official name header；ExactNumeric 和 TwoWayFiles 两根已有子项 PASS。总流程 FAIL_TEST_OR_IDENTITY，绝不写成六门全通过。该历史 FAIL 原样保留。独立 lead 源审已接受 c382 的唯一 helper 修复：`uniqueOracleHistory` 将非空 rows 条件改为非空 columns 条件，以保留 pinned shell 对空结果仍输出的 header；所有 SQL、顺序、比较、错误/exit/诊断断言与工厂不变。该修复还被两个既有显式 UNIQUE oracle history 根共用。原 gate11 的修后运行状态须按独立终态另记，本文不自动继承或宣称 PASS。
- 更早 AU-1 V1 SOURCE_HOLD、原 read-only keeper fixture 的 gate08 FAIL、所有观察器/输入失败继续保留。有限成功只属于自己的原树，不能转授新组合。
- 默认新增 36 根和 oracle-tag 新增 3 根是源码声明；Go 选择、动态子项、完整 named totals、Go/format/compiler/guard/ordinary/race/oracle/build/CI 全部在本组合 **NOT_RUN**。既有 full26 可沿用外层 26 门/28 次 Go 调用的正常范围，但必须重新绑定精确树、相关闭包与预期结果，源 pin 已按独立 review 窄修，输入历史失败不能删除；实际修后 oracle 结果及完整组合资格仍须另记。
- 脚本、workflow、模块、isolated oracle factory 和手工双门保持。无新 launcher、无 manual tag 或 env 打开；ANN V4、41-target fuzz、未知中断链、native fault/recovery/Backup/WAL publication 都不得借此次组合启动。M10、物理/耐久/安全资格未验收。

本状态只报告新源组合与已存在输入收据。未运行 Go、formatter、编译器、测试、项目 guard/recorder、oracle、native、Git、网络、Library、CI 或发布。本后继仍待精确独立读回及另行批准的组合资格范围；已完成的窄源复审不替代该组合的任何执行。

### 历史 v4 状态快照（原内容保留）

#### v4 组合：SOURCE_ONLY，前序 Rows pin HOLD 已作源码修正；全部组合运行 NOT_RUN

本源码在已保存 main `8bc189474bdb2b0a683c6c059992f9acbc426250`、tree `c0959fb87e8aaedd946e6bb7abbaddb67cf5ced0` 上按已审阅增量组合 LEFT WHERE、encrypted inspect、Darwin inventory manual 和 Connector rollback manual。未复制旧整文件覆盖 main 的 Storage、UNIQUE、WAL/recovery、原生门或 oracle factory；未加入 automatic UNIQUE 或 CHECK 的独立在途源码。

**已复审的前序冲突：** v3 tree `5cd0a42ace3af039ee7abf96082ece1921f13556` 因 Connector 的旧 Rows pin 与 LEFT 已审阅终态记账源码不同而保持 HOLD；该冻结树及原证据不修改。独立复审已完整核对 Rows、adapter、手工 owner 与默认排除闭包，并只接受将 `sqlite/rows.go` pin 从 `79f23ee5c53d6ca89441abfe22e49eb18ba76d16b545e2894a2baadf91b49b7f` 改为 `0e164cf3fa34b2255962f99e263f573ff390895df0831a71f3489f2ba36dd8cd`。本新后继的唯一 Go 改动就是这一行；完整 guard SHA-256 为 `7c1e748d032c280282b5fb5f64234aa3e34a161ff06c00fd6ac38a0ec398177e`。其余 35 pin、26 项惰性 mutation、全部 helper/测试及生产源码不变；36 个 pin 静态匹配不等于运行 guard 通过。仍需独立后继读回与另行批准的资格检查；源码接受不授权测试、发布或原生执行。

新组合的 Go/parser/gofmt、sourceguard、ordinary/race、完整 oracle、build/vet/跨编译、恢复、发布和 CI 全部 **NOT_RUN**。输入切片的有限成功、静态审阅和完整基点通过均不转授本组合。所有原生手工与物理受限链保持原门和授权边界。

### 历史默认 CI 影响与下一步计划（完整运行前）



工作流、scripts/check.sh、scripts/test-oracle.sh、模块和 isolated oracle factory 原字节不变。默认脚本仍先运行 testscope，然后普通 tests/vet/build/race；oracle job 仍使用 pinned official SQLite 并运行完整 oracle-tag 普通集合。

源码层面预计新增 LEFT 普通/driver 测试和 oracle-tag 的一个根、encrypted inspect 的模型与健康正确密钥普通测试、Darwin 六个纯 guard 根、Connector 两个纯 guard 根。Darwin main hook 只读取/解析固定源码，不执行原生 harness。Darwin 的 `darwin && manual_darwin_inventory` 和 Connector 的 `manual_recovery && linux` 双门不因普通/oracle tag 而打开；没有新增默认恢复、Backup publication 或 WAL publication 运行权限。实际 Go 选择、完整根/子项计数、guard、性能与 CI 结果仍待独立审阅后验证。

前序 v3 的已知 Rows pin 不匹配已在完整独立源码复审后作单行修正；原 HOLD 冻结记录保留。下一步是精确后继读回及另行批准的有限/完整资格，不能把 36 个静态匹配或原输入六项成功写成新组合 PASS。automatic UNIQUE / CHECK 仍是未纳入、未向本组合转移资格的独立在途候选。不执行手工或物理链，不放松 guard、删除断言或改写历史失败。


## 2026-10-12：plain rollback 有限 fault adapter 首片源码候选

在 check-eof-checkpoint-source-v1 的隔离副本加入 plain rollback Commit 14 个实际 I/O site 的 typed bridge、默认直通/manual 双门单 Tx 单次范围、descriptor-free 收据及 false-PASS 测试源码、真实 writeFullAt 内存 fake 测试与窄 source guard。详见 [精确范围与 A1–A8 缺口](testing/rollback-fault-adapter-zh.md)。

状态 SOURCE_IMPLEMENTED_PENDING_INDEPENDENT_REVIEW / VALIDATION_NOT_RUN；旧 WAL guard 的 main.go whole-file pin 原值保持 HOLD，未机械更新。本片没有 native caller/fixture/controller/恢复 verifier，不能运行完整物理 harness，也不能给 physical PASS；WAL 与 encryption 均 NOT_BOUND。仅源码读取/复制/文本编辑/哈希；Go/gofmt/AST runner/测试/编译/native/恢复/Git/网络/CI/发布均 NOT_RUN。原受限链和 ANN V4、41-target fuzz、UNKNOWN 中断状态不变，M3/M8/M10 与原始 goal 未验收。


## 2026-10-12：rollback fault adapter V2 / RF-01 源码窄修

在冻结 V1 的隔离副本中只修泛用收据的身份留存：非法 Identity 在保存前拒绝，保留无字符串的专用 rejection marker、ProblemIdentity、收到 ordinal 与实际槽位；合法身份用固定有界 buffer 复制后冻结，事件仅借用此份冻结身份。新增超长 payload / sticky / marker 伪造等纯反例源码，并同步新合同 template/pin。详见 [RF-01 说明](testing/rollback-fault-adapter-zh.md)。

状态 SOURCE_RF01_CORRECTION_PENDING_INDEPENDENT_DELTA_REVIEW / VALIDATION_NOT_RUN。V1 及其独立审阅原样保留；Commit、default/manual、main 和旧 guards/pins 全部字节不变，旧 main pin HOLD 不释放。只有源码文本/哈希工作；所有 Go/formatter/AST runner/测试/编译/native/恢复/Git/网络/CI/发布 NOT_RUN，全部原始物理/受限链状态不变。
