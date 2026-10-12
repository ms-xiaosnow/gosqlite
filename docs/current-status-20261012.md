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
