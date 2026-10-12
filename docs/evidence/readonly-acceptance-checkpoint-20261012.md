# 只读验收 V3 检查点完整正常门终态与边界

## 最新结论

冻结候选 `readonly-acceptance-integration-source-v3`，tree
`f8192e090acb9acedb07f811a66d043c1a9a24c2`（3,312 文件、817 Go 文件），
已于 **2026-10-12 13:31:44 UTC 完成完整 26 门 / 28 次 Go 调用 PASS**。
这是真实新运行结果，不再是 EXPECTED，也没有继承输入切片或 b9 的 PASS。
新提交、Git bundle、精确恢复和对应新 CI 仍 **PENDING**；原始 goal 与物理/
原生手工/外部资格均未完整验收。本文是仅文档叠加，运行源和原始收据未被改写。

## 自然终态及准确来源

- `completion.json`：13:31:44.315624 UTC，状态
  `ALL_TWENTY_SIX_READONLY_ACCEPTANCE_NORMAL_PASS`；SHA-256
  `19bdc9616862af42ddbd9b10f14f577f926c9a64bf83e67e4859bb689e1c37dd`。
- `progress.jsonl` 的 COMPLETE：13:31:44.317421 UTC；该瞬间记为
  RELEASE_PENDING_FINAL_RECEIPT，随后独立 lease 收据补齐，不能停在旧瞬间状态。
- `COMPILER-LEASE-RELEASE.json`：13:31:44.317464 UTC，**RELEASED**；
  all outer children naturally returned=true、still-live observed children=[]、
  forced termination=false；SHA-256
  `6f159a86a42f1876673629bc923ad9017dd3456f9c302494f7a45e19c0a3d427`。
- 工具终态：session `75199`，write_stdin chunk `903c9f`，自然 exit 0。
  `readonly-acceptance-full75199-tool-terminal.json` 的 SHA-256 为
  `f4ba6dd9736e862034de9de867b696af218096c725397efd8311db75f50debc5`。

上述本次执行收据保存在 `readonly-acceptance-full-normal-verification-v2`。
本次文档核验逐个读完 26 个 result，与 completion 内记录一致；每项均 PASS、
exit 0、自然返回、被观察 descendants 已静止、身份检查 PASS、无 latched condition。
全部原 stdout/stderr 的字节长度和 SHA-256 已静态复核。未重新执行任何项目脚本、
Go、测试或文档中的命令；静态 JSON/hash 核对本身不是另一轮测试。

## 实际测试结果及不同证据层

| 门或结果 | 实际记录 | 证据边界 |
| --- | --- | --- |
| ordinary | 9,735 named PASS / 7 SKIP；1,519 Test + 41 Fuzz + 6 Examples；313 固定 seed PASS | 9,742 named，1,566 top-level，其中 top-level 1,565 PASS / 1 SKIP |
| race | 9,735 named PASS / 7 SKIP；1,519 Test + 41 Fuzz + 6 Examples；313 固定 seed PASS | 与 ordinary 分别执行；不是把两个结果合成唯一测试数 |
| 完整 pinned oracle | 11,474 named PASS / 0 named SKIP；1,637 Test + 41 Fuzz + 6 Examples；313 固定 seed PASS | 1,684 top-level；package 层另有 11 个无测试 SKIP，不冒称所有层均零 SKIP |
| LIMIT oracle/race | 394 named PASS，2 Test 根，零 named SKIP | 有限 phase-conversion 范围 |
| 四条 quick | CLI 26、pager 31、range 3、IPK oracle 9 named PASS | 各自精确选择，不与 full suite 相加为唯一测试数 |
| live-writer readonly oracle | 7 named PASS，1 根 | 既有健康官方 live-writer 互操作路径，不是真只读挂载 profile A/B |
| guard、vet、CGO0 build、modules verify | 全部 PASS | source/静态构建/依赖核验不替代手工或掉电验收 |
| 五目标生产编译 | Linux amd64/arm64、Darwin amd64/arm64、Windows amd64 全 PASS | 编译成功不代表五个 OS/架构都执行过 |
| 三个 foreign test binary 与 nm | Darwin amd64/arm64、Windows amd64 的六门全部 PASS；各映像 executed=false | 外平台仅 compile/symbol inspection，原生资格未取得 |

正常 Linux suite 实际执行了允许范围内的健康数据库文件/WAL/正确密钥与 oracle
路径，同时含 descriptor-free 模型、source/AST、Examples 和已有 Fuzz 固定 seed。
不能把本次说成“全部纯模型”，也不能说“所有 native 都未执行”。没有开启 41-target
持续 fuzz campaign；真正 manual 故障、退出/重启、恢复、namespace 干扰、媒体 A/B、
升级、WAL publication/elapsed 以及 Darwin/Windows 原生手工仍 **NOT_RUN**。
精确选择、sourceguard 和独立复审只覆盖已审的 import/init/调用闭包；一个 -run
筛选不能消除初始化，未观察到的问题不能冒充未知初始化已被验证。

## 实际依赖闭包与资源

| 范围 | 包和输入 | 已核验边界 |
| --- | --- | --- |
| whole-module 非测试闭包 | 212 包：148 标准库、52 项目、12 外部；1,372 唯一输入 | 1,310 Go、54 汇编、6 header、2 embed；support 可达 testscope/testoracle/os/exec，不能把 whole 当 shipping |
| shipping sqlite/driver/CLI 联集 | 164 包：132 标准库、20 项目、12 外部；1,261 唯一输入 | 1,202 Go、53 汇编、6 header；forbidden support dependencies absent=true |

两闭包实际输入均在进入下一步前重新 hash；CGO/C/C++/Objective-C/Fortran/Swig/
Syso 字段均为零，不能把存在的汇编与 header 误称不存在。模块实际固定为
x/crypto v0.58.0、x/sys v0.49.0、x/text v0.43.0。新增 go/printer、go/doc/comment、
text/tabwriter 属于 guard/support 闭包；shipping 的包集合边界不因此扩大。
whole closure result SHA-256 为
`17413a2b3f2443535635814af2ba3aa3fb8bd7e9666e0101e359e103d359bdd8`；
shipping closure result SHA-256 为
`da57caa17a2e7402851592221ded4cb27fc391152230c1a3dbb319240f1b87f5`。

原始 `resource-observations.jsonl` 共 **6,651** 个样本，全部 within_floors=true、
violations=[]。最低 /tmp 可用 4,852,948,992 字节、1,273,669 inode；最低 workspace
可用 6,204,809,216 字节、1,621,150 inode。对应原下限为 /tmp 1 GiB、workspace
2 GiB、两者各 10,000 inode。此为采样门限结果，不是连续硬资源上限或 RSS 峰值证明。
目录中没有单独 resource-summary.json；这里直接根据完整原始观测逐行静态汇总，
并核对 final.resources-after.json，不编造额外收据。

源码/工具/依赖/输入身份在各阶段及最终边界通过；当前源码再次读回仍与 V3 清单的
3,312 文件字节和模式一致。读取 atime 不声明不变。新文档叠加后的完整树须单独
固定；f8192e 只绑定实际运行前冻结源，不绑定新增文档或未来提交。

## 旧失败和未合入源码各自保留

首轮 verification V1/session `48019` 实际针对 V2/c3bdb469… 的 guard **FAIL**，
后续 25 门 NOT_RUN，未被本次 PASS 改写。四条诊断来自两个遗漏 append 各被
检查两次。V3 只修两个 guard 文件，56 条 inert 负断言在既有根内，不新增 named
root/child；quick `11576` 的七 AST 根和 full sourceguard 两门 PASS 单独保留。

以下独立源码均**未纳入 f8192e 或本次 full26**：

- 首个 rollback fault/recovery donor tree `d87e78a7cc58b86d2f0c101cddf28211100ea0c1`。
- SQL-ledger donor tree `bfca0d8320e26aef16499c988ef843306f9c91e8`。
- A5 commit/退出/重启的独立源码工作；本检查点不声称已有该 controller 或验收结果。

前两份 donor 已在 Library `libfile_cfc71fe179a88191b8ab9046392f1fbe` 静态保存，
类别为 UNPUBLISHED_DEVELOPMENT_SOURCES；36,623,406 字节，SHA-256
`f38031cd31fa8d287d59581b4338ab66208ffa65ce57b35c63bf3f3c9c254900`。
归档收据明确不含当时活动的主线 full26 或尚在编写的 A5。它保留 donor 的旧失败/
格式化/源码证据；静态保存不表示 donor 已发布、完整有限门已通过或原生物理已验收。

V3 早期静态包 `libfile_52f330fd86948191bbba9175dfdd64ea` 仍只保存源码、原
FAIL 与 quick 两门 PASS，**不含本次 full26 终态**，也不是 Git bundle/恢复/CI。
已发布 b9 的 bundle/恢复80868/CI38219105955只资格 b9；本新检查点提交、新bundle、
精确恢复和新 CI 继续 PENDING，阶段保存不等于这些事项已完成。

## 仍需完成的有限设施与外部资格

本批五组切片仍为相对 b9 的 43 路径：Darwin/Windows backend、prepared media、
layout1→2 升级、rollback adapter、WAL elapsed。默认 rollback 六 bridge 直通原调用，
manual adapter 的 arm/finish/disarm caller 与恢复后 SQL/普通索引/grams ledger
在本检查点仍缺，不能引用外部 donor 来声称已集成。新耗时源码是总 public-call
entry-to-return，含扫描/锁/发布/constructor retirement，无真实 manual 样本，
不是 recovery-only、cold-cache 或 P95 结论。真实媒体 fixture/keeper 也未由此提供。

A1–A8（I/O/短写、Sync、ENOSPC/权限、错位/torn、commit退出重启、replay/checkpoint
再中断、rekey/upgrade/backup中断、磁盘认证负面）继续按六个已有协议 profile 的
实际持久化操作列有限清单。A5 在独立源码推进，其实现/源审/有限测试与物理运行状态
需另报；不能写成“本批已经实现”，也不因独立源码存在就闭合 A5 或六 profile。
正常 layout1→2/归档新目标恢复已有 harness，不再称完全缺源码；完整迁移/拒绝/
失败保留矩阵以及原生执行仍分开。详列和原 goal 依据保留在下方历史审计摘要中。

public readonly 仍只资格 Linux live initialized-SHM 子集；CHECK 有界 scalar、
EOFUnproven decode-only 与 native finite-only 拒绝保持。更广 SQL 不超出 goal
升格为无界兼容目标；ANN 旧 FAIL/HOLD、41-target campaign 和 UNKNOWN 链限制保持。
真实 OS/文件系统/设备持久性、物理故障/掉电、独立安全/密码学/威胁模型、法律/许可/
API、长时 soak/同机数据/冷热缓存性能与获准外部 PG/Neon 验证仍独立，M3/M8/M10
与原始 goal 未完成，不能称只剩人工操作。

## 本环境持久性和历史环境边界

13:12:46 UTC 只读 statfs（chunk `8442bb`）记录项目 R 为 overlayfs
`0x794c7630`、/tmp 为 tmpfs `0x01021994`，两者 block size 均 4,096。
普通 /tmp 数据库成功、Sync 返回或正常 reopen 不证明掉电、真实持久介质或 fsync
持久性资格。statfs 不能推出所有底层 fsync 实现、挂载参数或设备缓存，也不能把
旧环境的 fsync=volatile 观察直接当本环境事实。手册原存储准入条件不因此放宽。

旧环境 session `54102` 继续 **UNKNOWN**，本次新自然终态不能证明旧会话自然退出。
没有改 TMPDIR、挂载/权限或手册命令来绕过限制。下方完整历史，包括旧 SOURCE_ONLY、
在途 PENDING/EXPECTED、所有 FAIL/HOLD/UNKNOWN，都保持其原时点含义。

## 历史在途草稿与既有依据完整保留

以下为 13:04 阶段快照及其后 13:12 环境补充的完整原稿。其 full26/race/oracle/
closure 的 PENDING 或 EXPECTED 已由上面的真实终态补齐；原文件与收据不回写。

# 只读验收 V3 检查点证据与有限余缺

## 当前结论与快照边界

这是一份待终态补齐的中文文档叠加草稿。full26 session `75199` 仍为
**IN_PROGRESS / PENDING**，新提交、Git bundle、精确恢复与对应新 CI 均
**PENDING**。截至 `2026-10-12T13:04:23.042014+00:00` 的实际进展只到八门
`VALIDATED PASS` 与 CGO0 build START；不存在本稿可引用的完整 26 门成功终态。

本稿只读取源文件、既有结果及进度，写到活动源树外的草稿目录。未运行 Go、gofmt、
项目脚本、测试、编译、oracle/native、Git 或网络；没有改动活动候选和原始证据。
后续须先取得自然终态与资源/身份复核，再按实际结果补本文，不用 EXPECTED 填充 PASS。

## 精确源码与五组范围

- 本次实际运行候选：`readonly-acceptance-integration-source-v3`。
- 冻结 tree：`f8192e090acb9acedb07f811a66d043c1a9a24c2`；3,312 文件、817 Go 文件。
- 前驱 V2 tree：`c3bdb46948b8ac3e7a02a257f52c4fdcb657ffd5`。
- 既有 main.go SHA-256：`dd0d43330c5ad020b4ab2e5649f32f963aa4909316be64ae32c01d9f3c4371aa`。
- 相对 b9 基点累计 43 路径（31 Go、12 文档）；V3 相对 V2 只改两个 readonly
  guard 文件，其他 3,310 文件的字节和模式保持。这里不把文档叠加后的树冒充运行树。
- V2 完整 production/default closure 源审及 V3 精确两文件增量复审均 CLEAR；
  源审是源审，不能代替下述实际门或物理资格。

| 五组切片 | 已纳入的准确范围 | 本检查点不宣称的结果 |
| --- | --- | --- |
| Darwin 与 Windows readonly backend | nativeAttachDMS/nativeMap 的有限源码改动、纯模型/source checks；有限隔离验证已有各自收据 | 原生 OS/FS/锁/映射资格 NOT_RUN；不开放 public Darwin/Windows readonly |
| prepared readonly-media | 双门 native harness、receipt/owner/provenance 合同和 source guard；隔离 finite13 已 PASS | A/B 原生 profile NOT_RUN；不生成 fixture/keeper，不取得真实只读介质资格 |
| encrypted layout1→2 | 512/4096/65536 的保留式升级及 portable archive→全新 snapshot 目标恢复 harness；隔离有限步骤已有结果 | 原生升级/恢复 NOT_RUN；不做原地 downgrade，不闭合所有格式迁移或 A7 |
| plain rollback fault adapter | Commit 14 site、六类 bridge、bounded receipt；默认 bridge 精确直通原 owner/helper | 本组合没有 arm/finish/disarm caller、恢复 verifier 或 SQL-ledger/grams 验证；无 physical PASS |
| WAL-tail elapsed | 既有单手工入口的三种公开调用与三条 nil-policy 对照、纯收据、总调用时间合同；隔离 finite11 PASS | 原生 publication/elapsed NOT_RUN；不等于全部 WAL crash/restart recovery 或性能资格 |

真实媒体隔离 V4/session `41545` 的 13 步于 12:18:42 UTC 自然 PASS；升级隔离为
17 个有限步骤加前序 list，WAL elapsed 为 11 步。这里保留它们各自的源身份与范围，
不把隔离结果转授组合 full26，也不把 compile/symbol inspection 写作 native 执行。

[只读媒体手册](../testing/readonly-wal-prepared-media-manual-zh.md)、
[有限升级手册](../testing/encrypted-layout1-to2-manual-zh.md)、
[rollback adapter 手册](../testing/rollback-fault-adapter-zh.md)和
[WAL elapsed 手册](../testing/wal-tail-publication-manual.md)保留完整的前置条件、
失败留存、once-retirement 与独立授权边界。原 SOURCE_ONLY/HOLD 文句按作者时点保留。

## 首轮真实失败与 V3 有限修正

首轮完整正常门的证据目录版本是 V1，session `48019`，实际被测源为 V2 tree
`c3bdb46948b8ac3e7a02a257f52c4fdcb657ffd5`。`01_guard` 在 12:34:15 UTC
自然 exit 1，真实状态为 **FAIL / FAIL_TEST_VALIDATION**，后续 25 门 **NOT_RUN**。
它不是“没有运行”的 pin HOLD，也不因之后成功被改写为 PASS。

四条 findings reset/rebinding 诊断只有两个实际遗漏点：main 中 `formatUpgrade`
与 `rollbackFault` 两个合法 append 未纳入旧五式 inventory。main 被直接 source
scanner 和整 root contract 各检查一次，因此每个遗漏各报告两次；不是四个独立 append。
两个新增 root contract 是 result/error propagation triplet，不是另两个 append。

V3 只改 `internal/testscope/readonly_wal_manual.go` 和
`internal/testscope/readonly_wal_manual_test.go`：将封闭 append 集合从五式扩成
七个精确原式，同时核验四个新组合 hook 的直接位置、顺序、准确 helper/result/error
及错误/结果传播。原 reset/rebinding/address escape/failed-state 规则不放宽，
main、三个既有 pin、生产/default/native/manual/纯 contract、modules/scripts/workflow
均不改。旧 28 条 previous-policy 负断言原样保留。

56 条新 inert 负断言为四个 hook 各十四条，放在既有
`TestReadOnlyWALManualWholeRootInventory` 内；没有新增 Test 根或命名子项，
不能把 56 条算作新 named PASS。它们覆盖缺失、错 helper/result/error、吞错误、
死分支、重复、状态清空/取址、传播错序与完整 triplet 错位；parse error 或无关 pin
失败不能冒充语义拒绝。它仍是 bounded source/AST guard，不是 type-checker 或物理证明。

独立 quick session `11576` 在 12:47:51.142237 UTC 自然完成 exit 0：
七个既有 media AST 根 **7 PASS / 0 SKIP / 0 命名子项**，完整 source guard **PASS**，
无强制终止、无资源违反，源字节/模式复核通过。原 `48019` terminal 的 SHA-256
`a473ec470ad5f4be58d9434a4b8597be374f68ca46c20d1ab4f5b18fa6f59b34` 保持。

## 已保存的未发布源码和有限终态包

Library ID `libfile_52f330fd86948191bbba9175dfdd64ea` 保存 V3 精确源码、源审、
首轮 guard FAIL 与 quick 两门 PASS。归档为 19,724,038 字节，SHA-256
`96c20855c3e24713cb5304a969d1149acef08e0ef114511641e11d1dff1a05dc`。
静态读回逐一核对 3,621 成员的路径、字节、模式、tar checksum、GZIP CRC/EOF，
并复核原始输入未变。读回 receipt SHA-256 为
`7d6dc09468a59f9957bf436bef6ddbed27bbaf8da8b20c16be25cb4468870a7a`。

分类是 **UNPUBLISHED_DEVELOPMENT_CHECKPOINT**。活动 full26 session `75199`
明确不在包内；该包不是 Git bundle，不是一次精确恢复运行，也不含新 CI。
静态归档核验不能转授为尚在执行的 full26 终态或任何 native/物理资格。

## full26 实际阶段与尚未取得的终态

当前证据目录为 `readonly-acceptance-full-normal-verification-v2`，session `75199`。
recorder 于 12:59:22 UTC 启动，12:59:28 UTC 固定 26 门 / 28 次显式 Go 调用。
以下只记录本稿冻结快照，不预判仍在运行的过程。

| 阶段 | 本稿可核验状态 | 准确口径 |
| --- | --- | --- |
| guard、四条 quick、live-writer readonly oracle | 六门 VALIDATED PASS | 各阶段结果成立；不是完整 suite 终态 |
| ordinary | VALIDATED PASS，记录于 13:04:05 UTC | 自然返回 13:03:58.941048 UTC、exit 0；9,735 named PASS / 7 SKIP，合计 9,742 |
| vet | VALIDATED PASS，13:04:19 UTC | 只代表本阶段 |
| CGO0 build | START，13:04:23 UTC | 本稿尚无 validated 结果 |
| race | PENDING，计数 EXPECTED | 预期 9,742 named，其中 9,735 PASS / 7 SKIP；不能称实测 |
| 完整 pinned oracle | PENDING，计数 EXPECTED | 预期 11,474 named / 0 SKIP；不能称实测 |
| 后续编译、依赖闭包、LIMIT oracle/race、test-c/nm | PENDING | 逐项以自然完成和身份/资源收据为准；外国映像不执行 |
| full26 总体 | IN_PROGRESS / PENDING | 必须完整 26 门、28 次调用、自然终态和全部复核后才可判定 |

ordinary 原始 stdout 为 9,872,008 字节，SHA-256
`07bff3d3baacb320d2b5b6544fe44006051ac0b0ebb227f9e146696f52dc9e31`；本稿静态读回
与该阶段 result 的长度/hash 一致，并重新按 JSON terminal 计数得 9,735 PASS / 7 SKIP。
1,566 个 top-level terminal = 1,519 Test + 41 Fuzz + 6 Examples，其中 top-level
1,565 PASS / 1 SKIP；313 个固定种子 PASS。不能把 1,566 全称为 Test roots。

组合静态选择的增加量仍为 58 个 Test 根、729 个 named children、787 named paths。
V3 的 56 条新增负断言不改变这个选择增量。预期 oracle top-level 总数为 1,684，
仍是期望清单而非本稿可报告的已执行计数。

新增 `go/printer`、`go/doc/comment`、`text/tabwriter` 属于 sourceguard/support
闭包；shipping sqlite/driver/CLI 闭包的静态预期仍为 164 包，整个非测试模块闭包
预期为 212 包。两个新纯 contract 包和 support imports 不等于 shipping 引入
native fixture 或 subprocess。此处数字是已审源码推导的 EXPECTED，实际 dependency
list 门仍待本次 full26 运行，不能把旧基点闭包结果代替新收据。

## 已发布 b9 基点与新检查点发布分开

已发布 main 为 `b9f6e8e00036a3b264c8a7eb83d0c61539899782`，tree
`c7e412092f6d6e2c8bf98c95e096e3c65b38dd93`。这已包含 CHECK/EOF 检查点，
不是旧 e092 或“CHECK 还未合入”的阶段。

- b9 源码 bundle：Library ID `libfile_7665eae9bc5481918e47d82152773ebc`。
- b9 精确恢复：session `80868`，2026-10-12 10:05:12.341550 UTC 自然完成
  26 门 / 28 Go 调用 PASS，lease 随后 RELEASED。
- b9 恢复终态包：Library ID `libfile_5678ab05620c8191bbdf71941a1dfd41`；
  44,568,383 字节，SHA-256
  `49c047acf2716961236513babbf7896ffb1d4ff9f38ebaa97eefd25d939b4e42`，完整静态读回通过。
- b9 CI run `38219105955`：ordinary job `114710767801` 与 oracle job
  `114710768103` 均 completed/SUCCESS。该归档写作时的 restored26 PENDING
  已被上述后续独立恢复收据补充，旧原收据不回写。
- 本 V3 新检查点：提交、bundle、精确恢复、对应新 CI 均 **PENDING**。
  文档叠加后还需单独固定完整树与非文档一致性，不能把 `f8192e…` 写成新文档树。

## 有限必需设施仍有实现缺口

依据 [原始 goal](../../goal.md) 的受支持路径和有限验收要求，范围限于六个既有
协议 profile：plain rollback、plain exclusive WAL、plain shared WAL、encrypted
snapshot/layout1、encrypted exclusive/layout2、encrypted shared/layout3。按每个
profile 实际存在的持久化操作建立有限 cut-point/断言清单，不要求无界 OS×硬件×
语法的笛卡尔积；不支持或不可达点须明确列出。

| 类别 | 本批之后仍需闭合的有限实现和断言 |
| --- | --- |
| A1 I/O 错误与短写 | 已有 rollback 14-site adapter，但本组合缺真实 caller、fixture/owner/recovery verifier、已确认 SQL/普通索引/grams ledger；WAL 与 encstore seam 仍缺 |
| A2 文件和目录 Sync 失败 | 已有 rollback 复合 site，不等于目录 helper 内每步、WAL marker/SHM publish、encrypted root 或错误后 durability 证明 |
| A3 ENOSPC 与权限变化 | 合成 I/O error 不等于真实磁盘满或身份/权限改变；受控设施、输入留存及已知/未知终态判定仍需绑定 |
| A4 部分页、错位和 torn write | prefix short-write 设施只覆盖其有限软件动作；page/sector 边界、已确认事务无部分结果及普通/扩展索引守恒仍需真实绑定 |
| A5 commit 与退出重启 | child/controller、阶段握手、独立恢复入口、退出与已确认 commit receipt 的有限协议仍有实现缺口；正常 Close/reopen 不替代 |
| A6 replay 或 checkpoint 再中断 | rollback replay 与 plain/encrypted WAL checkpoint 的有限再中断、再次恢复或安全拒绝矩阵未闭合 |
| A7 rekey、upgrade、backup 发布中断 | root/key/archive authority、immutable 对象与新目标恢复的中断矩阵未闭合；本次正常 1→2 harness 不能代替 |
| A8 磁盘认证负面 | 错 key/salt/tag、ciphertext 单字节、跨页与 main/WAL/role/DBID/record/父子绑定、未知版本拒绝及 nonce/明文 marker 的实际路径矩阵未闭合 |

首个 rollback fault/recovery harness 与 SQL-ledger 源码切片在本组合之外；不能写成
这里已经接上，也不能将独立候选的存在称为这六个 profile 和 A1–A8 全部闭合。
真实媒体 harness 不提供 fixture/keeper 工具。layout1→2 与新目标恢复已有实物源码，
因此不再笼统称“没有升级 harness”；完整受支持格式/迁移边、拒绝边与失败留存矩阵
仍不完整。grams13 的 version/config/REINDEX、普通/扩展索引 ledger 以及各受支持
迁移边应按既有合同核验，不创造任意未来版本或无界 downgrade 目标。

WAL elapsed 已有有限总公开调用计时源码，不能再称“完全没有计时设施”；但本批
没有任何真实样本，计时包含 scan/lock/publication/constructor retirement，单入口
每 invocation 只有 n=1，cache 为 warm/unspecified。没有 recovery-only/fsync-only、
cold-cache、P95 或已确认 crash acknowledgement。真实 interrupted/restart recovery
controller、ledger 验证及其余受支持 recovery profile 的计时仍是独立 CODE_GAP。

## 本环境的持久性证据边界

2026-10-12 13:12:46 UTC 的只读 statfs 观察（协调者工具回执 chunk `8442bb`）
记录：项目工作目录 `R` 所在文件系统为 overlayfs，magic `0x794c7630`、
block size 4,096；`/tmp` 为 tmpfs，magic `0x01021994`、block size 4,096。
这里的 `R` 指本次 `restart-20261011T2040` 工作目录，不是另行核验的持久设备。

普通测试在 `/tmp` 创建数据库后成功、Sync 返回成功或正常 Close/reopen 成功，
均不能据此认定掉电恢复、真实持久介质或 fsync 持久性已验收。statfs 类型与 block
size 只证明此次文件系统分类观察；不能单靠 overlayfs/tmpfs 标签推断所有底层
fsync 实现、挂载选项、设备缓存或存储持久性，也不能把旧环境的 `fsync=volatile`
观察直接套到本环境。已有[主手册环境前提](../manual-verification-zh.md)中的
volatile 警示和各 manual 的真实存储准入边界仍须遵守。

[环境替换原始记录](../history/ENVIRONMENT-REPLACEMENT-20261011.md)中的旧会话
`54102` 保持 **UNKNOWN**。本轮新环境的阶段 PASS 不证明旧会话已退出、成功或
自然释放；不以换机重建旧中断链。没有更改 TMPDIR、挂载/权限或四份手册中的命令。

## 既有能力和外部资格的边界

[public readonly-WAL](../plain-readonly-wal.md) 仍限 Linux live initialized-SHM；
cold/no-SHM、其他 OS readonly 不因本批自动开放。[CHECK](../check-constraints.md)
仍为有界 scalar 子集。[EOF 解码](../rollback-replay-memory.md)仍为 EOFUnproven，
[native recovery admission](../rollback-recovery-open.md)仍在写入/Sync 前 finite-only
拒绝，未开启 EOF 恢复。默认 rollback bridge 的逆替换和 io.WriterAt 签名还原可恢复
完整 b9 Commit 源码；这属于默认行为源审，不是故障后的物理恢复证明。

真实 manual/设备/文件系统/OS 锁与持久性、掉电/物理恢复、独立安全/密码学/威胁模型、
许可/法律/API 审查、有资源预算的 soak/同机同数据/冷热缓存性能、获准 PG/Neon
真实验证均与普通门分开，仍未取得完整资格。交叉编译与符号检查不等于原生执行，
默认健康文件/WAL/正确密钥测试不等于 fault/crash/认证负面验收。

更广 SQL 按兼容矩阵记录真实 unsupported/partial，不把 FK/CTE/UPSERT/view/trigger/
window/JSON/FTS5/ATTACH 等建议项提升为本批无限实现目标。INSERT IGNORE 继续暂停。
ANN V3 三个报告准入 FAIL、ANN V4 HOLD、41-target fuzz 与旧 UNKNOWN 中断链的限制
均保留，不重启、不改名重建。M3/M8/M10 和原始 goal 未完成，不能称“只剩人工测试”。

## 终态补齐所需证据

下一次更新只接受此精确候选的新自然终态：26 个阶段 result、完整 stdout/stderr、
实际 Go 调用数与 named lifecycle、源码/工具/依赖/历史证据身份、资源与 lease
终态。若任一门失败或未运行，应保留其真实 FAIL/NOT_RUN，不把期望计数填为结果。
随后新提交、bundle 读回、精确恢复及对应 CI 各自按准确身份补记；原生和外部层仍独立。

本稿使用的只读来源包括 V3 `SUMMARY.json`/`SOURCE-FILES.json`、V2 完整 production
review、V3 narrow review、`readonly-acceptance48019-tool-terminal.json`、quick 的
`completion.json` 与 `readonly-acceptance-guard-quick11576-tool-terminal.json`、
本次 full26 `STARTED.json`/`progress.jsonl`/`02_ordinary.result.json`/原 stdout，
以及 b9 的 CI terminal summary、恢复 terminal crosscheck 与 Library 读回收据。
这些来源在独立证据目录保存；本文没有复制、覆盖或重新生成原始执行证据。


## 完整普通终态的持久保存补记

本精确 f819 运行的完整终态归档已保存并逐字节读回验证：Library
`libfile_ba48d3e206808191bf676d9f8970f2a2`，33,505,560 字节，SHA-256
`68098af12f41f24fcd8c8a822aeece36a6daec8ad88e6c9852bf4384e97bf309`。
809 个归档成员／774 文件均核对 bytes、路径、模式及 gzip CRC/EOF，包含完整
518 文件运行目录、三份仅编译的外平台测试映像、143 项输入 pin 和独立复审。
源码 Library 另存；原 V1 FAIL 保留。此处是开发证据持久保存，不能替代本检查点
尚待完成的 Git bundle、精确恢复运行、对应 CI 或真实物理／M10 验收。
