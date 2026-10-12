# gosqlite 中文手工验收操作手册

> 2026-10-12 当前组合状态：Storage metadata 修正树 `7389e7b7` 的三条回归
> 已实际 PASS（14 根 / 92 named PASS），UNIQUE 树 `fd4e3471` 的十二条有限门
> 已实际 PASS（31 根 / 223 named PASS / 零 SKIP）；两者均自然退出并释放 lease。
> 本文所在新 Storage + UNIQUE 组合仍是 **SOURCE_ONLY / NOT_RUN**：完整 ordinary、
> race、oracle、跨编译/依赖、发布/精确恢复/对应 CI 尚未运行。下文旧的
> SOURCE_ONLY、PENDING、FAIL 与 NOT_RUN 均保留为各自身份的历史，不覆盖上述有限结果。
> 原 V1 显示契约 FAIL 与 V2 metadata 真失败（8,126 PASS / 1 FAIL / 7 SKIP；
> 后续 19 NOT_RUN）不改写。native Backup（包括 UNIQUE 数据库）、WAL publisher
> 和物理资格仍 **NOT_RUN**；完整目标与 M10 未验收。详见[新检查点证据](evidence/storage-unique-checkpoint-20261012.md)。

## 2026-10-11 Storage 手册入口与当前结果

严格明文 Backup 和 Linux 显式独占 WAL-tail publisher 已有组合源码；对应
[Backup 六入口规程](testing/plaintext-backup-manual.md)与
[WAL 公开四场景规程](testing/wal-tail-publication-manual.md)均已有有限 harness，
实际 native 运行及存储资格仍 **NOT_RUN**。完整物理故障/崩溃/恢复设施仍
**IMPLEMENTATION_MISSING**，更广平台/FS/WAL Backup 与恢复能力仍有工程缺口。

已发布 f189 的 Library 精确恢复 26 门和 CI 38179686483 已闭合；Backup 输入
ec9f30d2 的 36 条与 WAL 输入 3f772f7e 的 5 条有限命令已 PASS。冻结 Storage
组合树 `2602cc60e8c82e3c7f900eb44439e4217cb0b8c3` 的完整普通 26 门 V2 已在
ordinary 门 **FAIL**，后续 19 门 NOT_RUN；修正后新树/完整复验 **PENDING**，
文档叠加后的新提交、恢复与 CI 仍 **NOT_RUN**。原生 publisher
手工门与物理限制没有因此放开。完整 ordinary 本来就包含健康 native 文件/WAL/
正确密钥维护；不得将其称为纯模型，也不得将其结果称为全平台/耐久性合格。

本次只补说明，所有既有命令、参数、隔离、留证和清理步骤原样保留。以下旧日期、
基点、未合入说明与历史失败仅指原时点；当前身份、V1 记录器失败与 V2 源码断言失败
见[Storage 证据补记](evidence/storage-checkpoint-20261011.md)。

## 2026-10-11 17:02 UTC 开发资格补记

本次 Darwin 有界清单/诊断源码组合 `8a324a8cedeccc8ba46234924c8074890aefa1da`
的 21 项限定普通验证已全部闭合。ordinary/race 各 5,614 PASS、7 个固定预期
SKIP；完整 SQLite 3.54.0 oracle 为 7,246 PASS、零 SKIP；LIMIT oracle/race
394 PASS。Darwin 两架构及 Windows 测试二进制仅编译并核对符号，未运行。
完整身份、原始记录器失败和修正、支持域及缺口见
[Darwin 清单专题的本次补记](darwin-unicode-inventory.md)。

本文形成时的已发布父基点为 `3ea6d3ab15f678fe48bee312921da60828845869`，
其精确 Library 恢复 17 门和 CI [38149634238](https://github.com/ms-xiaosnow/gosqlite/actions/runs/38149634238)
已通过；只属于该父基点。本批仅同步文档，所有 813 个非 docs 且非 README
文件与上述已验证组合逐字节/模式一致。新提交、精确恢复和 CI 要分别读取
其实际终态，不预写自指 SHA 或未来 PASS。历史段落的“未发布”、旧 main、
NOT_RUN、PASS/FAIL 保留各自写作时点，不能混作当前资格。

原生 Darwin/Windows、实际持久性/故障/恢复、安全审查和 M10 均未验收。
本次没有合入另行开发的原生 Unicode 手工入口、strict plaintext backup、
WAL-tail publisher、只读明文 WAL 或 IPK 修正；相关未实现/未合入项仍分别跟踪。

本轮仅添加开发证据说明，原有手工命令、依赖版本、隔离副本与清理/留证步骤未改，也均未在此轮执行。请仍逐项区别 IMPLEMENTATION_MISSING、FAIL、NOT_RUN、平台阻塞及真实外部环境要求。

## 当前补记：Windows shared rekey 源码与资格分层（2026-10-11）

已发布基点现为 `67318ded6416547f26d685719d3e01e208454a87`，tree
`1452ea832a41bdb8214bffd98fdb1b4d4fc37757`；其精确 Library 恢复两 quick +
全 15 门 PASS，原有 CI [38141275294](https://github.com/ms-xiaosnow/gosqlite/actions/runs/38141275294)
及 ordinary `114475481426` / oracle `114475481239` 均 SUCCESS。这些结果只属于
67318ded，不能移给新候选。下文 startup / WAL / LIMIT 的 12:31 摘要、旧失败、
旧 `main 仍为 b349` 和各历史表保留原时点，不代表这里的新发布状态。

本工作副本基于 Windows 九路径源码 tree
`b0e21b81b6eee598124171ec2fb7f4c6df77751a`，实现 guarded admission/conversion
及转换后同一源密钥的精确 authority 检查；尚未发布。该源码的四条限定 normal
为 30/1/1/1 路径 PASS；另一次 CLI normal 为 root/snapshot/wal/shared 四路径 PASS。
该 b0e 代码树的完整 17 门已于 14:42 UTC 自然 PASS：ordinary/race 各 5,602
PASS + 七个固定 oracle SKIP，完整 oracle 7,234 PASS/零测试 SKIP，LIMIT oracle
race 394 PASS；五目标 build 与 Windows test-compile/nm 均通过，PE 未运行。
这些新执行只资格到 b0e 代码树；含本文档的新组合、新提交恢复和候选 CI 尚未发生。
本次文档同步不运行项目代码。协议、来源、当前资格和剩余门见
[Windows shared-rekey 专题](windows-shared-rekey.md)；原生 Windows 仍 **NOT_RUN**。


## 最新状态：startup / WAL / LIMIT / 示例（2026-10-11）

已闭合的发布基点是 `b3492ed822d59834e5159cb7964c9dbd937027f7`，tree
`ff13ded52883edfbabf747547a11b4e4ed7301ee`。其 Library 精确恢复及历史
CI `38129577026` 已完成；CI 于 10:05:10 UTC SUCCESS，ordinary
`114440917650` 与 oracle `114440917334` 均成功。该旧 oracle 启动隔离
**NOT_PROVEN**，不能把真实历史 SUCCESS 改写为干净启动资格。

**2026-10-11 12:31 UTC 终态：52b 精确本地两 quick 加全 15 门 PASS；两次旧 FAIL 保留。**

- 原组合 `904ca698c3359bfe0674231f6d14ec8a9d704f30`：guard PASS，11:11:04 UTC
  ordinary FAIL，5,341 PASS / 23 FAIL / 7 预期 oracle 叶项 SKIP；9 个顶层的
  20 条直接 EXPLAIN 断言是生产规划提示回归，另 3 条为父项传播。
- 首次修复 `209ba515f4dce18ae57103254b4053d4708b8736`：guard PASS，11:50:37 UTC
  ordinary FAIL，5,547 PASS / 23 FAIL / 7 预期 oracle 叶项 SKIP。旧 9 根及原
  23 条失败路径已实际 PASS，但新增测试误要求 ANN ranking 为 bounded_heap；
  两次失败后各自第 3–15 项仍 NOT_RUN，原始日志和断言不覆盖。
- 最新 `52b03f1d94f578fb6e3722c7bf28fc0056fad6d4` 只修新测试的 shape 常量，
  生产源未改。quick 10 根 / 75 路径 PASS；随后全 15 门从 guard 重新执行，
  12:31:15 UTC 自然终结，17 个顶层命令均 exit 0（含脚本子调用实际 19 次 Go）。
- ordinary/race 各 5,570 PASS 加 7 个预期 oracle 叶项 SKIP；完整 oracle 为
  7,202 路径 PASS、零测试 SKIP；末两名 LIMIT oracle race 为 394 路径 PASS。
  vet、no-CGO build、五目标编译、模块与两种实际依赖闭包均 PASS。

只有这个固定本地 tree 的已执行开发范围取得资格。main 仍为 b349，没有新提交、
新提交 Library 恢复或新 CI 结果；本手册用户模板仍 NOT_RUN，不自动授予执行权限。
native WAL-tail publisher 等真实代码缺口、原生/物理/安全和 M10 未验收均保持。
下表独立切片的历史身份与结果继续保留，不能反向改写为同一源码资格：

| 项目 | 精确源码身份 | 已有结果与当前边界 |
|---|---|---|
| SQLite 启动输入收口 | `5fec5ce5e5339f02dc2715d9a2d6bd4691b1c62c` | 46 个 Go 与 5 个 shell 调用点已在候选源码改造；独立源码审阅 PASS，14 Test / 198 路径纯检查与 AST guard PASS；后来的独立资格批次见最新摘要；不继承旧 PASS，也不自动授权 CI |
| WAL scanner / proof / 普通生命周期 | `dbbb1e4ebca107c9a12695d9ad498a0c2f2057cf` | 13 条普通非 oracle 命令 PASS；normal/race 各 1,208 Test、41 Fuzz 目标的 313 固定种子、5 个可运行 Example；7 个可选 oracle 叶用例 SKIP；原生坏尾安全发布器仍缺代码 |
| LIMIT 绑定与执行阶段 | `4c5c5fa0b60ee728b2c6f2eb153325106270cc86` | 新鲜基线 4 Test / 123 子项产生 49 条直接阶段断言 FAIL；候选 normal/race 各 16 Test / 1,145 子项 PASS、零 SKIP；没有新 oracle 结果 |
| Levenshtein 示例发现 | `ec48c505f48aed452b96cd6544c6e3e84187ac3d` | 仅增加一行空行；精确 normal/race 各实际运行 `ExampleEngine_Levenshtein` 一次并通过原 `3 false` 输出断言；不反向改写旧 WAL 只有 5 个 Example 的事实 |

完整身份、原始日志定位及持久归档见[本次证据](evidence/startup-wal-limit-20261011.md)。
未来用户的启动前置、可复制模板和停止规则见[启动隔离中文规程](oracle-startup-verification-zh.md)。
这些新规程仍是 NOT_RUN，不是执行许可。旧段落的日期、PASS/FAIL、PENDING、
候选身份和当时缺口均保留；若与本摘要不同，按各自时点理解，不能混合资格。

真实代码缺口仍包括 native WAL-tail publisher、更广 CAST/IPK/LEFT-ON/WHERE/
排序阶段、通用明文备份、只读明文 WAL、部分 backend 与完整物理故障设施。
strict Option A 明文备份在另一个候选中实现，不属于本批；main 的 plaintext
backup 仍 unsupported，将来的 ext-family-only profile 也尚未资格验收。
这些不是“只差运行”。四场景 public Open harness 已有源码/编译证据，但实际物理
四场景仍 BLOCKED / NOT_RUN；长期、原生平台、持久性、安全和独立 M10 验收未完成。
总体仍处于 **M9 实现推进、M10 未验收**，继续逐批补真实代码，不能宣布全部可编码项完成。

2026-10-11 后续补记：公开 Open 四场景手工测试入口现已补齐并通过独立
源码/AST/选择/编译/符号检查，全部物理运行仍 **BLOCKED / NOT_RUN**。
当前身份及步骤索引见[第20节](#20-公开-open-四场景-harness-源码与编译资格2026-10-11)；
下面 P0/第18节中“整个 public harness 未实现”的文字保留原阶段时点，
不能据其旧状态否定新入口，也不能把编译资格改成运行 PASS。

五个示例目录终结的后续限定普通复验见[补充规程](example-retention-verification-zh.md)；其真实失败现场仍未动态验证，旧受限门与缺失设施保持原状态。

2026-10-11 P0更新：有限public Open恢复源码已接入，独立source/model/compile审阅PASS；真实恢复仍BLOCKED / NOT_RUN，公开物理harness为IMPLEMENTATION_MISSING，详见第18节及[专题](rollback-recovery-open.md)。下面固定776b基线的缺口表和历史步骤保留当时身份，不能据其旧“public Open未接线”推断当前源码仍未实现。

本文供维护者规划后续独立验收，核对实际可执行入口、应保存的证据和不能据此作出的结论。**本手册编写过程只读核对并写入这个新文件，没有执行任何测试、构建、fuzz、数据库命令、故障操作或 namespace 操作，也没有修改项目源码或 CI。下面的执行步骤均未因编写本文而运行，状态为 NOT_RUN。** 文档持久保存及同期独立更改和测试如有结果，在 docs/progress.md 中另行记录，不属于本文编写证据。

## 1 先读结论与执行边界

当前不能宣布原始 `goal.md` 全部验收通过，也不能宣布稳定版、生产可用或加密安全已经认证。原因同时包括真实实现缺口、未合入的独立候选、已知失败、未运行的验收以及平台限制，不能统称为“只差人工测试”。

本手册初版的历史源码基线为当时本地只读确认的提交；它不包含后续 startup helper，不能用来运行本次新覆盖：

- 项目：`durable-restart/gosqlite`
- HEAD：`776b23eb59ce9613bf4498d352a716a193cdc9c3`
- tree：`8f6026330f968314456548a3b8684476fea73afb`，来自中断交接记录
- 编写开始时只读 `git status --short` 为空；随后验收相关文档进行了更新，不能再把当前工作树称为干净
- 原规范 `goal.md` SHA-256：`9ae7b1d776856669a10eb753408c7754d015d266f8783240165450e96e4bac81`
- 本文日期：2026-10-11 UTC

本文不是执行授权。2026-10-10 中断时新增执行均暂停；2026-10-11 首轮恢复先做只读核对与中文文档，清楚独立的后续工作须先说明源码范围、测试选择和与受阻链路无关的理由。被中断或既有受限链路继续暂停。即使某个普通测试以前被允许，也不能凭本文自动启动。未来维护者应先完成独立人工审查，确认合法执行范围与具体来源版本，随后仅在自己有权限、隔离且可丢弃的环境操作。

后续更新说明：下文固定776b基线中的 write/readonly/sparse-exact缺口是历史状态。四项独立修正的当前源码、限定结果和操作步骤见[CLI资源复验](cli-resource-verification-zh.md)与[组合证据](evidence/cli-resources-20261011.md)；不得把新测试套在旧基线上。其他受限门和未实现设施不因这次修正解除。

当前仓库说明入口：[验收矩阵](acceptance-status.md)、[中断记录](interruption-20261010.md)。本手册补充已有历史手工记录，不改变原始 goal.md。

特别规则：

1. 本文中的命令块只能逐项审阅，不能整篇提取为脚本执行。本文未创建或启动任何命令文件。
2. 物理故障、崩溃、进程强制退出、热恢复、原生 I/O 失败、错误密钥、磁盘篡改、namespace 干预及中断发布/rekey 在当前环境保持 BLOCKED / NOT_RUN。不得由代理换命令、换输入、换机器、换 CI 重试以绕过限制。
3. 22:39 中断的精确触发操作与对象未知。ANN V4 修正链路及约 22:30 被中断的 41 目标 fuzz 链路不得重发、改写或用另一项执行替代。没有证据证明其完整运行，也没有可记为通过的结果。
4. 本批已将 Windows/Darwin 两个 raw namespace 测试完整移入 manual tag/env gate，详见第 19 节；固定 776b 基线尚未具备该修正。**门控不构成新 Windows/macOS 全量测试或 OS CI runner 的执行许可。**
5. 新 OS runner、性能 CI runner、Actions 用量/配额/计费读取都不在本轮权限内。本文不给出启用、付款、配额检查或远端工作流触发操作。
6. 未来环境也必须是合法独立验收，不能被当作恢复被拒绝链路的替代路线。执行不确定时先暂停并查明原因，不推测“换写法就能解锁”。

## 2 状态如何记录

| 状态 | 本文含义 | 能否据此宣布功能通过 |
|---|---|---|
| PASS 历史证据 | 旧日志在明确源码、命令和范围下记录成功；本轮未重跑 | 只能证明该历史范围 |
| PASS 本次独立验收 | 未来操作者实际完成，原始日志、退出码、断言与身份完整 | 仅限声明范围，不能覆盖其他门禁 |
| FAIL | 实际观测失败，包括保留的历史失败 | 否；后续修复应新增证据而不覆盖原记录 |
| NOT_RUN | 未运行，或没有可确认的完成结果 | 否 |
| BLOCKED | 平台、权限、环境或前置条件阻止执行 | 否；不能当作跳过后通过 |
| HOLD | 候选或报告存在待解决审查问题 | 否 |
| PARTIAL / 实现缺口 | 部分实现、未实现或尚未集成到此提交 | 否；没有可运行入口时不编造命令 |
| IMPLEMENTATION_MISSING | 对该要求没有完整可用实现或执行 harness | 否；不能写成只待运行 |
| INCONCLUSIVE | 测量不足以支持结论，例如性能置信度不足 | 否，保留原始样本 |
| SKIP / skipped_unsupported | 测试/平台没有覆盖到目标 | 否；进程退出成功也不自动转为覆盖成功 |

通过负面对照有两种不同含义：测试程序正确检测到应拒绝的输入，可以记该负面对照 PASS；旧版本错误接受输入、导致回归测试退出 1，仍是旧版本 FAIL。外层记录脚本退出 0 不会把内层真实失败变成产品通过。

## 3 当前能力与未完成项

### 3.1 已有源码和历史普通证据

当前提交包含原生纯 Go 明文读写、受限 SQL 与 `database/sql`、事务/savepoint、普通 rollback/WAL/shared-WAL 生命周期、grams13、文本相似度、dense/half/bit/sparse 向量、实际 HNSW/IVFFlat、正确密钥加密生命周期、加密备份/rekey CLI、离线迁移与距离复核、Native Check V2、事务测量 V4 等实现。具体支持列表以 `docs/compatibility.md` 及专题文档为准。

中断记录给出的历史证据：

- 固定组合源码 `8b604e3c667152284a29d7ad7dfba5a10f63b053` 的完整普通 unit/vet/build、固定 SQLite oracle、全包 race、五目标构建、模块与源码哈希，在 2026-10-10 22:10:33Z 记录 PASS。
- 776b23e 的恢复副本检查于 22:37:31Z 记录 PASS；它是 focused normal/race、vet/build、五目标、模块和干净工作树证据，不要写成另一次完整全量 oracle/race。
- 该 checkpoint 已有 CI 运行 `38091940645` 于 22:52:30Z 自然完成 SUCCESS；交接记录分别核实 ordinary 与 oracle 作业。历史链接：<https://github.com/ms-xiaosnow/gosqlite/actions/runs/38091940645>。
- 本轮没有连接远端重验这些结果，也没有触发或重跑 CI。

### 3.2 必须保留的真实缺口

| 项目 | 当前 776b23e 的准确状态 | 后续前置条件 |
|---|---|---|
| 自动 public Open rollback 恢复 | 内部有限多段解析和 `RecoverRollback` 有源码；public Open 集成仍缺 | 独立候选集成与精确源码审查，再另行处理受限物理门禁 |
| WAL 尾部恢复/发布 | 支持普通成功路径；原生 exclusive-tail publisher 仍缺 | 不能把正常 close/reopen 当作实现；wal-tail V2 仍待独立复审 |
| 通用明文备份 | 当前 CLI 和公共 copy API 未实现 | Linux rollback V2 只是未发布候选；WAL 和非 Linux 明文备份仍未实现 |
| 真正只读的明文 WAL 全库 CLI Check | 未实现 | 现有已配置 WAL 会话可 `DB.Check`；命名扩展 WAL 检查使用可写协调，不能改称只读 |
| SQL/planner | 有限支持；更广的错误阶段、排序和多键差异未完全关闭 | V3 有 34 个保留失败，V4 有 6 个 bound-LIMIT 回归；V4c 是未合入候选，不能借其日志给 main 判 PASS |
| Windows shared encrypted rekey | main 中不支持 | 已审查的候选未并入 main；原生 Windows 仍 NOT_RUN |
| Darwin Unicode 与目录诊断 | main 保留受限文件名/目录清单范围 | V3 修正在未发布组合；更广 Unicode 根名能力仍受限 |
| raw namespace 门控 | 此表固定的 776b 基线未迁入 manual gate；本批两项门控更新见第 19 节 | 后续版本已保留原断言并增加 tag/env 双门；原生执行仍 NOT_RUN，不能直接新增 OS runner |
| ANN 报告验收工具 | main 是既有简单 harness；V3 候选 HOLD，V4 未冻结 | 不得以历史算法测试替代报告审查；被中断链路保持暂停 |
| 性能回归 CI | 后续 V3 候选有有界审查，未部署 | 受控机器、配额读取授权、资源决策和单独部署授权均未完成 |
| 长时间 soak、大规模测量 | 有 harness 与提案，无完整长时间结果 | 预声明负载、资源、目录、停止条件和审查；本文不启动 |
| 外部 PG/Neon 验证 | 现有路径主要是离线导入、转换和带注释模板 | 需要用户授权的可丢弃真实服务及版本/权限/扩展核验 |
| 独立数据安全、密码学、许可、API 审查 | 未完成 | 具备资格的独立评审；普通源码审查不能代替 |

未支持的 SQLite SQL/文件形式要明确拒绝，而不是忽略：例如未声明的 schema、UTF-16、WITHOUT ROWID、autovacuum、广泛 UNIQUE/CHECK/FK、CTE/subquery/window 等。不要把通过有限子集写成与 SQLite 3.54.0 全功能等价。Native Check 的受支持范围也比“能够 SELECT 某张表”更窄。

### 3.3 已知失败和未完成记录

- 早期完整 oracle 使用错误的默认可写 fixture 路径、后来 JOIN 断言过时等真实失败已有保留记录；修正后的结果必须另列。
- 原始 20 秒 rollback soak 在约 14.0159 秒报 `ErrChanged`；Reader 修正后的四个 20 秒样本通过，仍不是长期 soak。
- 事务 harness V2/V3 的错误图、retirement 与存储 quota 失败保留；V4 的三个 64-attempt 样本各 48 commits，只是有界 smoke。
- Native Check V1 的 detached-memory 假成功问题保留；V2 修复已集成。不能将源码/内存否定模型说成对真实磁盘进行了故障注入。
- 导入距离 V1 的 16000 维误差界失败约 14.0724 倍，V2 改为有推导的向外舍入误差界并有独立有理数检查；不能删除旧失败或随意加宽容差。
- grams medium 512 行出现约 147 倍数据库空间，18 个选择性中位数里有 4 个慢于扫描；smoke 12 个均慢。必须保留，不能给出普遍加速结论。
- ANN V3 的三个 HOLD：HNSW 节点读数与遍历/重评分不守恒；IVF 探完全部 lists 却遗漏源成员；同一 query/row 在 exact 与 ANN 精确重评分间出现矛盾距离。这是报告准入问题，不能靠重新跑 benchmark 掩盖。
- 上述 ANN V3 失败属于未发布报告工具接受了矛盾报告的缺陷，不自动推断生产 ANN 算法已经错误。
- 原生 Windows 默认 cross-vet 的 mmap `uintptr`/`unsafe.Pointer` 告警有历史记录。不要关闭检查器后把结果记为 clean vet；另行记录精确包、命令、退出码和诊断。
- 两小时每 profile 的 soak 从未开始；41-target fuzz 新活动范围不明且无完成证据；ANN V4 修正未完成。均不得记为通过。

## 4 环境与依赖准备

以下命令面向未来已获许可的 Linux/amd64 独立本地副本，采用 Bash。不是本轮运行指令，也不能原样用于 macOS/Windows。Go 目标平台运行与跨平台编译是不同验收项。

### 4.1 必需条件

1. 可丢弃、非生产且独立的本地工作目录；禁止在用户原数据库、唯一备份或运行中的数据库目录操作。
2. 固定 Go **1.27.2**。`GOTOOLCHAIN=local`，不自动下载或切换编译器。
3. 官方 SQLite **3.54.0** CLI 只作测试 oracle，不允许生产程序链接或启动它。
4. 模块按 `go.mod`/`go.sum` 固定：`golang.org/x/crypto v0.58.0`、`golang.org/x/sys v0.49.0`、`golang.org/x/text v0.43.0`。
5. Git、Bash、tar、SHA-256 工具；核对元数据可用 `stat`、`df`、Linux `findmnt`。将来需要构建 oracle 时还要 Python 3、unzip、make、C 编译器及官方源码构建依赖；使用经过核验的现成 oracle 可省去这一步。
6. `CGO_ENABLED=0` 用于生产构建和一般测试；单独 race 门禁使用 `CGO_ENABLED=1`，因此 race 的主机需有相应 C 工具链。这不意味着生产库依赖 CGO。
7. 记录实际 CPU、OS/内核、架构、内存、文件系统和挂载选项、缓存条件、其他负载、容器配额可见性。不能凭名称“独立主机”声称已经隔离。

仓库 `scripts/install-baselines.sh` 是 Linux/amd64 固定版本安装器；`scripts/build-oracle.sh` 构建并验证固定 oracle。**本轮不执行这两个脚本，也不自动安装软件。** 未来操作者须先审查脚本、官方来源和新目的目录，再决定是否运行。

固定来源材料见 `docs/upstream.md`，本轮仅核对仓库记录，没有重新联网认证上游：

| 对象 | 固定值 |
|---|---|
| Go Linux amd64 tar SHA-256 | `ecbadb99091a3f46e31f5f934b068b1864eafa7995211b39eaddf76996045fe5` |
| SQLite 源码 ZIP 文件名 | `sqlite-src-3540000.zip` |
| SQLite ZIP 字节数 | `14881448` |
| SQLite ZIP SHA3-256 | `a5c29342ca185abcda332448fbb27c536661245be27e20199b46c49099b0e15e` |
| 生成的 sqlite3.c SHA3-256 | `8efd453d08a7cfc4c79de5923153036cdc4dcaa254d4f1e617be6a2a98828f9e` |
| SQLite source ID | `2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3` |

没有本地已验证模块缓存时，应记录依赖缺失并停止该步骤。本文的离线设置不建立新的网络/下载授权；不能在校验失败后直接关掉验证下载替代物。

### 4.2 建立独立源码与数据目录

下面是未来普通验收的准备模板，先替换三个绝对路径；`BASE` 必须是用户新选的、空间足够的本地验收父目录。不要填生产目录、原仓库或中断工作目录。每次新建一个 `RUNROOT`，不能复用旧失败输出。

```bash
# 文档模板。本轮不得执行。
SOURCE_REPO=/absolute/path/to/reviewed/gosqlite
BASE=/absolute/path/to/disposable-acceptance
TOOLCHAINS=/absolute/path/to/verified/toolchains
# 必须先由用户填入已合入 startup helper、经过完整源码审阅的实际提交全 SHA。
# 当前历史 b349 / 776b 均不含该 helper；不得拿候选 tree 当作 commit。
: "${SOURCE_COMMIT:?填写本次已审阅新提交的40位完整commit SHA}"
clean_absolute() {
  case "$1" in /*) ;; *) return 1 ;; esac
  case "$1" in *//*|*/./*|*/../*|*/.|*/..|*\\*) return 1 ;; /) ;; */) return 1 ;; esac
}
for p in "$SOURCE_REPO" "$BASE" "$TOOLCHAINS"; do
  clean_absolute "$p" || exit 1
done
test "$(/usr/bin/uname -s)" = Linux || exit 1
test -d "$SOURCE_REPO/.git" && test -d "$BASE" || exit 1
SOURCE_STATUS=$(git -C "$SOURCE_REPO" status --porcelain) || exit 1
test -z "$SOURCE_STATUS" || exit 1
SOURCE_HEAD=$(git -C "$SOURCE_REPO" rev-parse --verify HEAD) || exit 1
test "$SOURCE_COMMIT" = "$SOURCE_HEAD" || exit 1
SOURCE_TREE=$(git -C "$SOURCE_REPO" rev-parse "$SOURCE_COMMIT^{tree}") || exit 1
for p in internal/testoracle/spec.go internal/testoracle/test.go \
  internal/testoracle/spec_test.go internal/testscope/oracle_startup.go \
  internal/testscope/oracle_shell.go; do
  git -C "$SOURCE_REPO" cat-file -e "$SOURCE_COMMIT:$p" || exit 1
done
test -x "$TOOLCHAINS/go1.27.2/bin/go" || exit 1
test -x "$TOOLCHAINS/sqlite-build-3540000/sqlite3" || exit 1
umask 077
set -o pipefail
RUNROOT=$(mktemp -d "$BASE/gosqlite-acceptance-XXXXXXXX") || exit 1
WORK="$RUNROOT/source"
EVIDENCE="$RUNROOT/evidence"
DATA="$RUNROOT/data"
mkdir "$WORK" "$EVIDENCE" "$DATA" "$RUNROOT/bin" "$RUNROOT/go-cache" || exit 1
# 私有临时根仅供未来获准的普通工具/Go fixture/SQLite child 使用。
# 当前文档工作没有创建它，也不改变现有执行环境的缓存/资源策略。
ORACLE_TMP=$(mktemp -d /tmp/gosqlite-oracle-XXXXXXXX) || exit 1
git -C "$SOURCE_REPO" archive --format=tar "$SOURCE_COMMIT" > "$RUNROOT/source.tar" || exit 1
tar -xf "$RUNROOT/source.tar" -C "$WORK" || exit 1
printf '%s\n' "$SOURCE_TREE" > "$EVIDENCE/source-tree.txt" || exit 1
printf '%s\n' "$SOURCE_COMMIT" > "$EVIDENCE/source-commit.txt" || exit 1
printf '%s\n' "$ORACLE_TMP" > "$EVIDENCE/oracle-temp-root.txt" || exit 1
sha256sum "$RUNROOT/source.tar" > "$EVIDENCE/source-archive.sha256" || exit 1
cd -P "$WORK" || exit 1
WORK=$(pwd -P) || exit 1
ORACLE_CWD="$WORK"
sha256sum goal.md > "$EVIDENCE/goal.sha256" || exit 1
find . -type f -print0 | LC_ALL=C sort -z | xargs -0 sha256sum > "$EVIDENCE/source-before.sha256" || exit 1
```

`git archive` 只导出已明确选择的提交，不带未提交更改、其他候选或 `.git`。选择更新版本时，要重新编写该版本的范围、已知问题与证据；不能只修改 `SOURCE_COMMIT` 后沿用本文所有结论。源码快照复制不等于备份活数据库：禁止 `cp` 一个正在使用的 main/WAL/SHM 家族来冒充一致性备份。

归档源码没有 Git 元数据，下面 `GOFLAGS=-buildvcs=false` 仅用于正确标识这一情况，不是关闭安全检查或绕过 CI。

```bash
# 文档模板。本轮不得执行。
export PATH="$TOOLCHAINS/go1.27.2/bin:$PATH"
export GOTOOLCHAIN=local CGO_ENABLED=0
export GOPATH=/absolute/path/to/verified/offline-go-path
export GOMODCACHE="$GOPATH/pkg/mod"
export GOPROXY=off GOSUMDB=off
export GOCACHE="$RUNROOT/go-cache"
for p in "$GOPATH" "$GOMODCACHE" "$GOCACHE"; do
  clean_absolute "$p" || exit 1
  test -d "$p" && test -r "$p" && test -w "$p" && test -x "$p" || exit 1
done
export GOFLAGS=-buildvcs=false
export GOENV=off GOWORK=off
unset SQLITE_ORACLE GOTMPDIR TMP TEMP
unset GOSQLITE_MANUAL_RECOVERY GOSQLITE_MANUAL_RECOVERY_DIR GOSQLITE_MANUAL_NAMESPACE
export TMPDIR="$ORACLE_TMP"
# ORACLE_BIN 仅为本终端的未导出变量，普通测试不会因此打开可选 oracle。
export -n ORACLE_BIN
ORACLE_BIN="$TOOLCHAINS/sqlite-build-3540000/sqlite3"
export GOMAXPROCS=2
for p in "$ORACLE_BIN" "$ORACLE_TMP" "$ORACLE_CWD"; do
  clean_absolute "$p" || exit 1
done
case "$ORACLE_TMP" in /tmp/*) ;; *) exit 1 ;; esac
test -d "$ORACLE_TMP" && test ! -L "$ORACLE_TMP" || exit 1
test -w "$ORACLE_TMP" && test -x "$ORACLE_TMP" || exit 1
test "$(stat -c %a "$ORACLE_TMP")" = 700 || exit 1
test "$(stat -c %u "$ORACLE_TMP")" = "$(id -u)" || exit 1
test -f "$ORACLE_BIN" && test -x "$ORACLE_BIN" || exit 1
sha256sum "$ORACLE_BIN" > "$EVIDENCE/oracle-binary.sha256" || exit 1
printf '%s\n' "$ORACLE_BIN" "$ORACLE_CWD" > "$EVIDENCE/oracle-path-and-cwd.txt" || exit 1
oracle_child() (
  # 必须保留所审阅的绝对 CWD，不能改到临时根破坏相对 fixture。
  cd "$ORACLE_CWD" || exit 1
  exec /usr/bin/env -i LC_ALL=C LANG=C TZ=UTC \
    "SQLITE_TMPDIR=$ORACLE_TMP" "TMPDIR=$ORACLE_TMP" \
    "$ORACLE_BIN" -noinit "$@"
)
declare -f oracle_child > "$EVIDENCE/oracle-child-wrapper.bash" || exit 1
printf '%s\n' LC_ALL=C LANG=C TZ=UTC \
  "SQLITE_TMPDIR=$ORACLE_TMP" "TMPDIR=$ORACLE_TMP" \
  > "$EVIDENCE/oracle-child-environment.txt" || exit 1
```

上面的五次直接 SQLite 调用（身份、原生库 integrity、官方建库、官方复查、
grams integrity）统一通过 `oracle_child`。SQLite 子进程环境恰为 LC_ALL、LANG、
TZ、SQLITE_TMPDIR、TMPDIR 五键，`-noinit` 先于全部原参数；不继承 HOME/XDG、
SQLITE_DEBUG_BREAK（空值也不允许）、PATH 或 loader 覆盖。父 shell、`/usr/bin/env`
自身启动、编译器及系统 loader 仍为可信前置，不是完整 OS 隔离。
`ForTest` 的 `os.Getwd` 可能读取 PWD 并 stat 路径，不能称该适配器完全纯净。
GOTMPDIR 在 Go 1.27.2 的 test TempDir 优先于 TMPDIR，因此本模板明确 unset；
若用户以后更改它，必须重新核对 fixture、编译临时文件及 SQLite 的共同根。
以上目录存在/权限检查不能证明后续 namespace 稳定，失效时 SQLite 仍可能 fallback。
完整启动步骤与进一步限制见[启动隔离专题](oracle-startup-verification-zh.md)。

`GOPROXY=off GOSUMDB=off` 只用于已验证缓存的离线复验，不能说明依赖从未验证或新下载无需校验。`GOMAXPROCS=2` 是复验容量条件，不是 CPU 隔离证明。

### 4.3 日志记录函数

未来用户可在这次专用终端定义下面的函数。它不是仓库脚本，不会自动运行后续步骤；每一步名称必须唯一。它分别保存命令、UTC 时间、stdout、stderr、退出码，不依赖 `tee` 的退出状态。

```bash
# 文档模板。本轮不得执行。
capture() {
  local name=$1 rc
  shift
  test ! -e "$EVIDENCE/$name.command" || {
    printf '拒绝覆盖已有记录: %s\n' "$name" >&2
    return 90
  }
  printf '%q ' "$@" > "$EVIDENCE/$name.command" || return 91
  printf '\n' >> "$EVIDENCE/$name.command" || return 91
  date -u +%FT%TZ > "$EVIDENCE/$name.started" || return 91
  if "$@" > "$EVIDENCE/$name.stdout" 2> "$EVIDENCE/$name.stderr"; then
    rc=0
  else
    rc=$?
  fi
  printf '%s\n' "$rc" > "$EVIDENCE/$name.exit" || return 91
  date -u +%FT%TZ > "$EVIDENCE/$name.finished" || return 91
  printf '%s: exit %s\n' "$name" "$rc"
  return "$rc"
}
```

调用中必须用 `|| exit 1` 或人工停止，不得忽略失败后把整组标绿。失败重试应新建一组证据目录，保留旧失败。本文没有 `kill`、超时杀进程、自动修库、自动重试或自动删除失败目录逻辑。

### 4.4 版本、磁盘和源身份前置检查

先记录存储、空间、inode 和所选路径权限；这一块尚不调用 Go。

```bash
# 文档模板。本轮不得执行。
capture env-uname uname -a || exit 1
capture env-filesystem findmnt -T "$TMPDIR" || exit 1
capture env-workspace-filesystem findmnt -T "$RUNROOT" || exit 1
capture env-free-bytes df -B1 "$TMPDIR" "$RUNROOT" || exit 1
capture env-free-inodes df -i "$TMPDIR" "$RUNROOT" || exit 1
capture env-path-permissions stat -c '%a %u %n' \
  "$TMPDIR" "$RUNROOT" "$GOPATH" "$GOMODCACHE" "$GOCACHE" || exit 1
```

用户必须先逐项确认实际临时根所在文件系统至少有 1 GiB = 1073741824 bytes、
RUNROOT 所在文件系统至少有 2 GiB = 2147483648 bytes，且两者各至少 10000
可用 inode；所选 TMPDIR/GOPATH/GOMODCACHE/GOCACHE 均已有所需读写/搜索权限，
离线依赖 cache 确为已经验证的 `GOPATH/pkg/mod`。缺记录、身份不符或不足时就在
此停止，不进入下面的第一条 Go 命令，也不清理、降阈值或换 cache/temp 求通过。
每条后续 Go 调用前都须重新完成同样的资源核对，记录名用新的唯一名称；一次
预检不保证后续持续满足阈值。下面三条也分别以新预检与人工判定为前置。

```bash
# 文档模板。本轮不得执行。每一条前先完成上述资源准入，不能整块自动串跑。
capture env-go go version || exit 1
capture env-go-baseline go env GOVERSION GOOS GOARCH CGO_ENABLED GOTOOLCHAIN GOPATH GOMODCACHE GOCACHE || exit 1
capture modules go mod verify || exit 1
```

应见 `go version go1.27.2 linux/amd64`。这里没有启动 oracle；其身份命令移到
第 5.3 节单独的 HOLD 前置之后。未来该步骤获准时，oracle 身份必须逐字等于：

```text
3.54.0|2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3
```

不要整份打印 `env`，其中可能有密钥。只记录这里列出的非秘密字段。若 `findmnt` 不存在，先手工取得等价挂载证据，不静默跳过。

已有环境曾出现 `/tmp` 为约 5.24 GB tmpfs，仅约 1.33–1.42 GiB 可用；工作区挂载带 `fsync=volatile`。这是历史观测，不是新环境的容量。必须检查实际 `TMPDIR`，不能拿当前目录的 `df` 代替。`fsync=volatile` 主机上的成功 Sync/正常重开不能证明掉电持久性。
未来物理持久性资格的合成数据库必须另置于已经独立核验的真实持久文件系统，
与 `/tmp` 临时根分开；`$DATA` 的位置也必须实际核验，不能只因它不叫 tmp 就认可。
本普通模板没有启动、授权或代替受限物理验收。

## 5 普通自动门禁的手工操作

仅在第 1 节暂停原因已得到人工处理、且未来明确允许普通 Linux 复验后使用。不可在当前 main 的 Darwin/Windows 上照抄整组运行。

### 5.1 普通源码范围检查

```bash
# 文档模板。本轮不得执行。
capture ordinary-scope go run ./internal/testscope || exit 1
```

预期 stdout：

```text
Ordinary test and oracle startup source-reference guards PASS; behavioral review is still required.
```

人工还要阅读所选测试及 helper。此 guard 是防错辅助，不能证明行为安全；旧基线的 OS raw namespace selector 缺口已在第 19 节的独立切片补上，但 bare helper、不同名 wrapper、动态调用与 reflection 仍需行为审阅。普通 `GOFLAGS` 不得出现 `manual_`；`manual_namespace` 与 `manual_recovery` 不得混入普通 unit、race、oracle 或 CI。

### 5.2 无缓存普通全量测试与产物

现有入口 `scripts/check.sh` 依次做固定版本检查、testscope、普通 `go test ./...`、vet、CGO0 build、单独 CGO1 race；脚本本身没加 `-count=1`。若要主张新的运行结果，可逐条执行以下等价、明确禁用测试缓存的普通门禁，而不是把 `(cached)` 算作本次执行：

```bash
# 文档模板。本轮不得执行。
capture ordinary-unit env CGO_ENABLED=0 go test -count=1 ./... || exit 1
capture ordinary-vet env CGO_ENABLED=0 go vet ./... || exit 1
capture production-build env CGO_ENABLED=0 go build -trimpath ./... || exit 1
capture ordinary-race env CGO_ENABLED=1 go test -race -count=1 ./... || exit 1
capture cli-build env CGO_ENABLED=0 go build -trimpath -o "$RUNROOT/bin/gosqlite" ./cmd/gosqlite || exit 1
capture cli-version "$RUNROOT/bin/gosqlite" version || exit 1
capture cli-help "$RUNROOT/bin/gosqlite" help || exit 1
sha256sum "$RUNROOT/bin/gosqlite" > "$EVIDENCE/cli-binary.sha256" || exit 1
```

通过判定：每个命令退出 0、测试真实执行、无 `FAIL`、race 无数据竞争；各个 skip 的目标/原因另记。CLI version 应说明 Go 1.27.2 / SQLite baseline 3.54.0；开发版本字符串不冒充发布号。生产 build 为 CGO0；race 是另一份检测构建，不能用于性能比较。

不要为解决报错加大 quota、改断言、把缺失测试改成空正则或禁用 vet。全套普通测试通过也没有执行 manual 恢复或故障门禁。

### 5.3 固定完整 oracle

当前全部新 oracle 与会触发 oracle 的新 CI 仍 **HOLD / NOT_RUN**。
本小节仅提供未来用户已完成源码/依赖/启动/输入闭包审阅后的真实脚本形状，
不能因 pure source guard 或其他候选 PASS 而提前执行，连 version/help 也不能例外。

```bash
# 文档模板。本轮不得执行。
# 只有另行批准完整 oracle 后执行；不会向后续普通命令导出 SQLITE_ORACLE。
capture env-oracle oracle_child -batch -bail -noheader -list :memory: \
  'SELECT sqlite_version(),sqlite_source_id();' || exit 1
capture full-oracle /usr/bin/env -u GOTMPDIR \
  SQLITE_ORACLE="$ORACLE_BIN" TMPDIR="$ORACLE_TMP" \
  /bin/sh ./scripts/test-oracle.sh || exit 1
```

该脚本实际先跑 testscope，再比较 SQLite source ID，然后执行 `go test -count=1 -timeout=30m -tags oracle ./...`。不能把它缩成 `./internal/testkit` 或某一个 `-run` 后仍称“完整 oracle”。脚本中的 30 分钟是已存在的 Go 测试上限，不是 crash 验收；普通协作 CLI 子进程正常 `.quit`/EOF/Wait 收尾，不能擅自加进程终止机制。

应检查每个已选择 oracle 测试是否真的运行，不能用无匹配的退出 0 或 `SKIP SQLITE_ORACLE` 填通过。官方 fixture 只在隔离副本内使用；不改原仓库 fixture 或降低对照断言。

### 5.4 仅编译的多平台产物

原目标要求 linux/amd64、linux/arm64、darwin/arm64、windows/amd64 的 CGO0 产物。未来可分别做编译证据，**不运行外平台二进制，不开启新 runner**：

```bash
# 文档模板。本轮不得执行。
capture build-linux-amd64 env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath ./... || exit 1
capture build-linux-arm64 env CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath ./... || exit 1
capture build-darwin-arm64 env CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath ./... || exit 1
capture build-windows-amd64 env CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath ./... || exit 1
```

记录名用 `COMPILE_PASS`，原生 runtime 栏仍 `NOT_RUN`。不要把“历史五目标构建”与本模板的四个原目标混为一项；第五目标若要复验，应先从原日志确认其精确配置。

## 6 手工 SQL 与双向文件互操作

这组只用完整的新建合成 rollback 数据库。所有 CLI 每次只接受一条 SQL；跨调用 `BEGIN` 不保持事务，事务验收用 API/driver 测试。

### 6.1 gosqlite 创建并交给官方 SQLite 检查

```bash
# 文档模板。本轮不得执行。
CLI="$RUNROOT/bin/gosqlite"
PLAIN="$DATA/native-created.db"
test ! -e "$PLAIN" || exit 1
capture sql-create "$CLI" query "$PLAIN" \
  'CREATE TABLE items(id INTEGER PRIMARY KEY,name TEXT NOT NULL,score REAL)' || exit 1
capture sql-insert "$CLI" query "$PLAIN" \
  "INSERT INTO items VALUES(1,'中文',1.25),(2,'emoji🙂',2.5)" || exit 1
capture sql-index "$CLI" query "$PLAIN" \
  'CREATE INDEX items_name ON items(name)' || exit 1
capture sql-select "$CLI" query "$PLAIN" \
  'SELECT id,name,score FROM items ORDER BY id' || exit 1
capture sql-native-check "$CLI" check "$PLAIN" || exit 1
capture sql-inspect "$CLI" inspect "$PLAIN" || exit 1
capture sql-official-integrity oracle_child -batch -bail "$PLAIN" \
  'PRAGMA integrity_check; SELECT id,name,score FROM items ORDER BY id;' || exit 1
```

关键断言：

- SELECT JSON Lines 含列名 `id,name,score` 与两行 `[1,"中文",1.25]`、`[2,"emoji🙂",2.5]`，排序一致。
- `check` 返回 `status:"ok"` 和 `scope:"gosqlite-supported-profile-v1"`，不能只看 inspect。
- 官方输出 `ok`，并返回相同语义值；不要将官方输出格式与 JSON 字节直接比较。
- 主文件是合法 SQLite 格式；`inspect` 只说明 metadata，单独成功不等于完整 integrity。

### 6.2 官方 SQLite 创建并由 gosqlite 更新

```bash
# 文档模板。本轮不得执行。
OFFICIAL="$DATA/official-created.db"
test ! -e "$OFFICIAL" || exit 1
capture sql-official-create oracle_child -batch -bail "$OFFICIAL" \
  "PRAGMA page_size=4096; PRAGMA journal_mode=DELETE; CREATE TABLE items(id INTEGER PRIMARY KEY,name TEXT NOT NULL,score REAL); INSERT INTO items VALUES(7,'原始',3.5);" || exit 1
capture sql-read-official "$CLI" query "$OFFICIAL" \
  'SELECT id,name,score FROM items ORDER BY id' || exit 1
capture sql-update-official "$CLI" query "$OFFICIAL" \
  "UPDATE items SET name='已更新',score=4.5 WHERE id=7" || exit 1
capture sql-recheck-official oracle_child -batch -bail "$OFFICIAL" \
  'PRAGMA integrity_check; SELECT id,name,score FROM items ORDER BY id;' || exit 1
```

应保持 id=7、name=已更新、score=4.5，official integrity 为 `ok`。512/4096/65536 页和 BLOB/NULL/overflow 等完整矩阵由现有 oracle 与 native-check 测试核验；这两个手工例子没有穷尽它们。

### 6.3 database/sql、事务与普通 WAL

以下是真实存在的测试名，精确锚定能减少误选；不要把 focused PASS 代替全量 gate：

```bash
# 文档模板。本轮不得执行。
capture driver-transaction env CGO_ENABLED=0 go test -count=1 -v ./driver \
  -run '^(TestWritableDatabaseSQLTransactions|TestWritableReadOnlyTransactionsAndIsolation|TestWritableExplicitSQLTransactionsAndPoolReset|TestDriverSavepointsPoolResetAndOuterRelease)$' || exit 1
capture native-savepoint env CGO_ENABLED=0 go test -count=1 -v ./sqlite \
  -run '^(TestTransactionDeferredSnapshotTiming|TestTransactionSavepointNamesDDLAndStatementFailure|TestTransactionImmediateExclusiveAndFailedOuterRelease|TestTransactionSavepointsPreserveGramsHNSWAndIVF)$' || exit 1
capture shared-wal env CGO_ENABLED=0 go test -count=1 -v ./sqlite \
  -run '^(TestSharedWALSnapshotsAndWriterContention|TestSharedWALOptionBoundaries)$' || exit 1
capture write-example env CGO_ENABLED=0 go run ./examples/write "$DATA/driver-example.db" || exit 1
```

最后一个在新库应打印 `inserted 1: 数据库检索`，它只能作为写入 smoke：当前 examples/write 忽略 defer Close 的返回错误，且 log.Fatal 会绕过 defer，不能用该示例证明任何失败或正常资源退休路径。事务/关闭结论必须来自具备相应断言的测试与 API 结果。人工检查 rollback/savepoint 的逐行结果、BUSY 分类、上下文取消和正常 Close。普通 WAL retained committed head、checkpoint、关闭再打开、协作进程争用不是进程崩溃或热恢复证据。

## 7 grams13 文本索引与相似度

### 7.1 规范化随机组合和真实 1 2 3 字搜索

```bash
# 文档模板。本轮不得执行。
capture grams-correctness env CGO_ENABLED=0 go test -count=1 -v ./sqlite \
  -run '^(TestGramsIndexedLike12800Combinations|TestGramsNFCInputs12800RandomizedMemberships|TestGramsNFCLifecyclePostingConsistency|TestGramsOneTwoThreeRuneSearchAndExplain|TestGramsUnicodeEscapeNULLNULAndBooleanSafety|TestGramsHighFrequencyFallbackAndLimits)$' || exit 1
capture grams-example env CGO_ENABLED=0 go run ./examples/grams "$DATA/grams.db" || exit 1
capture grams-full-check "$CLI" check "$DATA/grams.db" || exit 1
capture grams-named-check "$CLI" check "$DATA/grams.db" docs_grams || exit 1
capture grams-official-integrity oracle_child -batch -bail "$DATA/grams.db" \
  'PRAGMA integrity_check;' || exit 1
```

应看到“库”“数据”“数据库”分别有相应 gram 长度的实际 `SEARCH` 路径、结果为包含“数据库查询”的行、真实 work counters，以及索引一致性通过。仅 EXPLAIN 文本包含 INDEX 还不够，应核对 posting 工作和源行读取。

NFC 测试是 **100 patterns × 128 texts = 12800 memberships**，不是 12800 次 SQL 查询；已有记录为 12416 distinct pairs，100 个输入经过 NFC 发生改变。原规范要求 ≥10000 经规范化组合，不要靠重复执行次数凑数。生产 LIKE 仍是 `LIKE_RAW_V1`，不能偷改成自动归一化语义。全量结果必须与 `NOT INDEXED` 扫描完全一致，合法高频/无字面量 fallback 要如实标明。

`check db docs_grams` 的 scope 仅是 `extension source/posting/frequency consistency only`；与不带 index 的全库 Check 是两个不同范围。

### 7.2 文本相似度

```bash
# 文档模板。本轮不得执行。
capture similarity-unit env CGO_ENABLED=0 go test -count=1 ./extension/similarity || exit 1
capture similarity-native env CGO_ENABLED=0 go test -count=1 -v ./sqlite \
  -run '^(TestSimilarityNativeTransactionsRebuildReopenCheckpoint|TestSimilarityNativeConfiguredNormalizationAndMatchNamespace)$' || exit 1
capture similarity-example env CGO_ENABLED=0 go run ./examples/similarity "$DATA/similarity.db" || exit 1
```

通过要同时覆盖独立数学/golden、阈值结果与强制扫描一致、事务后的 posting 一致性、中文/日文/emoji 语义。精确阈值必须零漏召回；无法证明召回界的范围应正常完整扫描，不能悄悄剪枝。加权排序融合输出不能被说成某个数据库原生 BM25 或 pg_bigm 实测。

## 8 向量与 ANN

### 8.1 精确距离与类型

```bash
# 文档模板。本轮不得执行。
capture vector-driver env CGO_ENABLED=0 go test -count=1 -v ./driver \
  -run '^(TestDriverDenseVectorBindingsAndTransactions|TestDriverVectorQuotaErrorClassification|TestDriverBitBindingsTransactionsAndReopen|TestDriverHalfBindingOwnershipTransactionsAndPool|TestDriverSparseBindingOwnershipTransactionsAndPool)$' || exit 1
capture vector-example env CGO_ENABLED=0 go run ./examples/vector "$DATA/vector.db" || exit 1
capture sparse-example env CGO_ENABLED=0 go run ./examples/sparse-exact "$DATA/sparse.db" || exit 1
```

**当前示例退出码缺陷**：基线 `776b23e…` 的 `examples/sparse-exact/main.go`
在参数或 example 返回错误时只打印 stderr，main 仍可能自然返回 0。
因此 `capture ... || exit 1` 不能独自判此例成功。必须人工核对
`sparse-example.stderr` 为空且 stdout 是完整、唯一 JSON：dimensions 为
1000000000，query_nonzeros 为 2，query_gss_bytes 为 32，native_source_rows
为 5，native_support_work/sql_support_work 均为 13，sql_representation 为
sparsevec、sql_ranking 为 bounded_heap；两组 exact L2 hits 必须一致，
三行顺序为 10、20、30，前两行距离为 0、1。任一字段缺失或错误均为 FAIL。
这是示例错误报告缺口，不自动说明向量内核错误；独立源码修正另行记录。

核对实际维数、有限数值、NULL/零范数策略、对应支持的 dense/half/bit/sparse 距离语义、持久化/reopen 和 driver 参数所有权。dense 的 L2/L1/负内积/cosine 应按文档与独立数学对照；bit 距离不要套 dense 算子含义，旧 pg_embedding Manhattan `<~>` 必须按迁移规范转成 `<+>`。

SQLite 官方会将 VECTOR 声明按 affinity 对待，合法 SQL 插入不符合本引擎向量类型的 TEXT 后被本引擎拒绝，是类型兼容负面对照，不是物理页篡改。

### 8.2 现有 ANN 普通功能与冻结的修正链路

已核对的既有测试/入口包括：

- `internal/testkit/ann_conformance_test.go`：`TestOracleHNSWStorageInterop`、`TestOracleHNSWLegacyGramsUpgrade`
- `internal/testkit/ivf_conformance_test.go`：`TestOracleIVFFlatStorageInterop`
- `sqlite/ann_typed_test.go`：`TestTypedANNOrdinaryLifecycleSavepointAndReindex`
- `examples/ann`、`examples/ivfflat`、`examples/typed-ann-rerank`
- main 的 `benchmarks/ann/main.go` 支持 `-rows/-dims/-queries/-seed/-method/-lists/-probes/-max-probes`

这些入口存在，不代表 ANN V4 修复完成。**为避免把被中断修正链路换写成新命令，本文不提供新的 ANN V4 启动、修补、冻结、native smoke 或替代 benchmark 命令。** 未来先人工区分既有普通算法复验与被阻塞的修正，复核精确源码、数据、报告版本和权限，再决定独立验收步骤。

后续有真实测量时必须保存同一 snapshot 下 exact 与 ANN 命中、距离、plan、fallback、图/list 读取和候选重评分计数，给出固定数据集的 recall@1/10/100、build time、空间和延迟。HNSW 默认参数 recall@10 ≥0.95 是原目标的评估目标，不是对所有数据保证；不得通过降低断言或隐藏 fallback 宣称达标。ANN consistency Check 也不证明 recall、拓扑最优或性能优势。

## 9 Native Check 与加密正确密钥路径

### 9.1 全库 Check V2

```bash
# 文档模板。本轮不得执行。
capture check-native env CGO_ENABLED=0 go test -count=1 -v ./sqlite \
  -run '^(TestNativeCheckOrdinaryPageSizesAndReadOnly|TestNativeCheckFreelistRollbackAndReopen|TestNativeCheckWALPinnedSnapshots|TestNativeCheckExtensionsAndTypedValues|TestNativeCheckCorrectKeyReadOnly|TestNativeCheckTransactionOwnershipAndCounters)$' || exit 1
capture check-cli env CGO_ENABLED=0 go test -count=1 -v ./cmd/gosqlite \
  -run '^(TestCLINativeCheckPlainAndNamed|TestCLINativeCheckCorrectKeyProviders|TestCLINativeCheckMemoryArguments)$' || exit 1
capture check-example env CGO_ENABLED=0 go run ./examples/native-check "$CLI" || exit 1
```

检查 catalog、freelist、所有逻辑页所有权、B-tree 空间、表/普通索引双向对应、grams 与支持的 dense/typed HNSW/IVF 语义。Quota/unsupported/取消/close 错误不得输出成功 JSON；dirty 扩展返回 reindex required，检查本身不能修复或悄悄重建。

示例会比较 API 与真实 CLI 的全部计数，核对完整的普通/正确密钥来源家族字节和名称不变。它创建临时合成文件，非持久性故障测试。注意该版本无条件 defer RemoveAll 私有临时目录，失败时也可能删除 fixture，且清理错误未被可靠报告；日志仍由 capture 保留，但不能据此声称保留了完整失败现场或证明所有资源退休。需要失败现场保全的验收必须先完成相应独立 harness 改进。

限制：`inspect` 只是元数据；`check --wal-shared plain.db` 不带命名扩展的 full CLI 路径应在打开前拒绝，不能试图用 named check 替代 full check。Check 专项 page-read quotas 从 reader acquisition 之后开始，不是整个 Open 的总 I/O 或内存保证。逻辑 NULL 验证也不是物理 serial-type-0 独占性审计。

### 9.2 加密正确密钥与 CLI 备份 rekey

优先使用现成合成示例，不为验收使用真实秘密或用户唯一数据：

```bash
# 文档模板。本轮不得执行。
capture encryption-unit env CGO_ENABLED=0 go test -count=1 ./encryption || exit 1
capture encrypted-snapshot-example env CGO_ENABLED=0 go run ./examples/encrypted || exit 1
capture encrypted-wal-example env CGO_ENABLED=0 go run ./examples/encrypted-wal || exit 1
capture encrypted-shared-example env CGO_ENABLED=0 go run ./examples/encrypted-shared || exit 1
capture encrypted-cli-example env CGO_ENABLED=0 go run ./examples/encrypted-cli "$CLI" || exit 1
```

断言是使用正确密钥创建/写入/认证、正常重开、新密钥正常 rekey、实际 archive restore 后行值一致、普通共享快照可见性和 checkpoint。示例生成临时密钥，参数/日志不应出现秘密，不提供之后可恢复这些临时密钥的保证。不要向测试添加错误 key、磁盘损坏或中断步骤。

这些 encrypted/encrypted-wal/encrypted-shared/encrypted-cli 示例在当前基线
都无条件 defer 删除私有临时目录，即使执行失败也可能丢失该 fixture。
因此它们只提供限定的普通演示/成功路径证据；capture 保存日志不代表
保存了失败数据库。要求现场保留或失败退休证明的正式规程尚需独立
实现，标为 IMPLEMENTATION_MISSING，不能在本文中编造替代入口。

未来需要检查已专门准备的合成加密 fixture 时，实际 CLI 形状如下。这里的变量名是 provider 描述，不是密钥；只允许可信启动器/非交互管道提供内容，不在命令行填秘密：

```bash
# 文档模板。本轮不得执行。仅用于已经审查的合成 fixture。
capture encrypted-fixture-backup "$CLI" backup /private/disposable/source.gse --out /private/disposable/new.backup \
  --key-provider env-hex:GOSQLITE_CURRENT --out-key-provider env-hex:GOSQLITE_ARCHIVE --mode snapshot || exit 1
capture encrypted-fixture-rekey "$CLI" rekey /private/disposable/source.gse \
  --key-provider env-hex:GOSQLITE_NEW --current-key-provider env-hex:GOSQLITE_CURRENT --mode snapshot || exit 1
capture encrypted-fixture-check "$CLI" check /private/disposable/source.gse --key-provider env-hex:GOSQLITE_NEW --mode snapshot || exit 1
```

关键操作条件：

- 数据库路径在前，flag/value 分开；当前不支持 `--flag=value`、重复 flag 或自动探测 mode。
- mode snapshot/wal/shared 必须匹配已创建的 encrypted layout；不是原生明文 WAL flag。
- backup 目标必须不存在、父目录已存在。输出是 portable encrypted archive，需用 `sqlite.RestoreEncrypted` 恢复；没有 CLI restore 子命令，不能拿 archive 直接 `query`。
- rekey 前所有其他连接必须正常关闭，包括 idle/read-only shared 连接。当前 main Windows shared rekey 不支持。
- raw key provider 要 64 个 ASCII hex 字符；password provider 为 1–4096 原始字节。password 的换行也是密码一部分。禁止打印环境或启用 shell `set -x`。
- `stdin-password`/`stdin-hex` 只接受管道或普通文件，必须 EOF；两个不同凭据不能同时各自占用同一 stdin。本文不创建凭据文件。
- 成功 JSON 只在操作和正常 close 成功后发出。错误退出仍可能发生在发布之后，不能据非零退出自动判“完全未改动”，也不能自动重试/删除输出。
- rekey 保留旧 roots/bases/segments，旧凭据仍可能解密旧历史；不是撤销旧密钥历史访问，不是安全擦除。
- 本节没有验证 on-disk 错误密钥、篡改、热恢复或 interrupted rekey。密文中找不到一个 marker 也不能证明从未产生临时明文。

## 10 离线 pg_embedding 导入和距离复核

### 10.1 合成导入和正常提交后继续

现成例子创建 CSV/NDJSON、manifest、新库，在一批已提交后正常返回再 resume，不是强制退出：

```bash
# 文档模板。本轮不得执行。
capture import-example env CGO_ENABLED=0 go run ./examples/pgembedding-import \
  -dir "$DATA/import-example" || exit 1
capture import-resume env CGO_ENABLED=0 go test -count=1 -v ./migration/postgres \
  -run '^(TestImportNativeCommittedResumeAndVerify|TestImportDatasetDistancesMappedFormatsAndResume|TestImportDatasetDistancesWideFiniteVectors|TestIndependentImportDistanceV2GammaProofChallenges)$' || exit 1
capture import-cli env CGO_ENABLED=0 go test -count=1 -v ./cmd/gosqlite \
  -run '^(TestMigrateImportDryRunApplyResumeVerifyCLI|TestMigrateImportDatasetDistancesCLI|TestIndependentMigrateImportDistanceCLIReportsFailure)$' || exit 1
```

`-dir` 必须是新路径，父目录存在。输出 JSON 为两个格式分别给出 Input/Manifest/Database/DryRun/Paused/Complete/Verified 等对象；保留 source hash、typed row hash、每行/每组件和主键/游标证据。示例的 ANN recall@4 仅是那一小组 fixture 的局部值，不能填成任意数据集 recall 或 M7 完成。

若普通验收授权不涵盖该例子内部既有 ANN 索引建立/查询，应先停在源码审查，不通过删减或重写例子变相替代被阻塞链路。

### 10.2 对已生成 NDJSON 做精确距离复核

未来在证据目录创建普通非秘密配置，不改仓库：

```bash
# 文档模板。本轮不得执行。
cat > "$EVIDENCE/distances.json" <<'JSON'
{"column":"embedding","query_count":4,"top_k":10,"metrics":["l2","l1","negative_inner_product","cosine"],"max_rows":10000,"max_pairs":100000,"max_component_work":8000000}
JSON
capture import-distances "$CLI" migrate pgembedding verify \
  --input "$DATA/import-example/source.ndjson" \
  --manifest "$DATA/import-example/ndjson-manifest.json" \
  --output "$DATA/import-example/ndjson.db" \
  --distances "$EVIDENCE/distances.json" || exit 1
```

人工检查：源 hash/大小和所有 accepted typed 值、数据库计数、schema/index、游标/拒绝记录一致；数学参考来自源导出，不从待验证数据库取 reference。对每个选中 query/metric，要验证所有非 NULL 候选及 Top-K 前缀，而不只是几个命中。确定性 query 样本不是统计保证。

允许如实出现 `partial_skipped_metrics`、`skipped_zero_norm`、`skipped_no_nonnull_vectors`、近似误差界内边界歧义；这些不能改称所有距离均已通过。普通 VerifyImport 的 exact_distances 原本 `not_run`，必须显式请求距离复核才有该证据。无论创建了什么 ANN 索引，此专用距离报告的 `ann_recall` 仍是 `not_run`。

### 10.3 PG Neon 模板检查

```bash
# 文档模板。本轮不得执行。
capture migration-transition "$CLI" migrate pgembedding transition \
  --catalog testdata/pgembedding-0.3.6/catalog.json \
  --options examples/migration-transition/options.json || exit 1
```

只检查生成的复核报告、版本/依赖/不支持列表、旧 Manhattan `<~>`→`<+>` 映射、回滚/切换前置。mutation SQL 继续保持注释，不连接真实 PostgreSQL，不取消注释执行。原规范列举的 `--pg-dsn-env` 或 `--generate-neon-sql` 不能被当成当前实际存在的完整在线工具。真实用户服务需要独立授权和适用版本核对。

## 11 事务测量与长期 soak 的未来操作

本节是在既有 V4 提案基础上给出的人工操作草案；**本轮不启动，原两小时方案也未启动。** 这是普通正确性观察，不是 crash durability，不能替代物理门禁。

### 11.1 先检查实际保存位置与容量

`benchmarks/transactions/main.go` 用 `os.MkdirTemp("", ...)`，因此数据在 `TMPDIR`，不是重定向 JSON 旁边。本普通 Linux 模板固定为 /tmp 下的用户私有根；若后续长测要采用另一位置，须另审完整路径契约，不能直接套用本 oracle helper。核对 mount/可用字节/inodes，并复核报告的 `retained_data_directory` 确实是其子目录。

既有提案的工程入场门槛为：初始至少 6 GiB = 6442450944 bytes；构建后和每个 profile 启动前至少 4 GiB = 4294967296 bytes；审查建议每次至少 250000 free inodes。这些是入场余量，不是能抵御其他进程占用的硬保证，也不是原规范给定数字。

3 × 1 GiB 指 apparent database bytes，并不含 allocated blocks、journal/COW、日志/缓存/编译/临时空间、内存映射和 tmpfs 的内存账单。256 MiB Go soft memory/epoch-sampled heap 不等于 RSS 硬限制。低资源时不启动；运行中异常保留现场，不能通过删除历史、缩短时长或强杀变出 PASS。

### 11.2 构建和小样本

```bash
# 文档模板。本轮不得执行。
capture tx-harness-tests env CGO_ENABLED=0 go test -count=1 ./benchmarks/transactions || exit 1
capture tx-harness-build env CGO_ENABLED=0 go build -trimpath \
  -o "$RUNROOT/bin/transactions" ./benchmarks/transactions || exit 1
SOURCE_ID="git:$SOURCE_COMMIT"
HARDWARE_ID='填写实际CPU-OS-文件系统及共享负载条件'
capture tx-smoke-rollback "$RUNROOT/bin/transactions" -mode throughput -profile rollback \
  -transactions 64 -rows 64 -readers 2 -batch 2 -seed 20261010 \
  -source-id "$SOURCE_ID" -hardware-id "$HARDWARE_ID" || exit 1
```

64 attempts、48 commits 是部分历史样本的观测值，不是并发调度下新运行必须机械等于的固定结果。当前运行必须按 C/R/B/X/F outcome history 重建确切模型，符合 workload 覆盖和 accounting。BUSY/取消应如实统计，不默认 0。

### 11.3 两小时每 profile 提案

操作者先填实际 host label，审查资源与完整新普通 gate，再一次只启动一个 profile；不要脚本自动串行跳过失败。三个选择为 `rollback`、`wal-shared`、`encrypted-shared`，每次使用不同的记录名。

```bash
# 文档模板。本轮不得执行。每次人工审查上一 profile 后才选择下一个。
PROFILE=rollback
capture "tx-soak-2h-$PROFILE" "$RUNROOT/bin/transactions" \
  -mode soak -profile "$PROFILE" -duration 2h \
  -rows 256 -readers 4 -batch 4 -seed 20261010 \
  -writer-pause 250ms -reader-pause 2ms -snapshot-hold 2ms \
  -max-data-bytes 1073741824 \
  -source-id "$SOURCE_ID" -hardware-id "$HARDWARE_ID" || exit 1
```

完整三组至少六小时 active time，另加初始化、checkpoint、重开、验证和收尾。`-duration 2h` 是实际累积 active time，不是总 wall-clock 上限；部分维护使用 Background context，可能延长总时间。harness 结束才输出 JSON 属于预期，不加 shell timeout/watchdog，不主动制造进程退出。

每组通过都必须逐项有证据：

- 退出码 0，JSON 完整可解析，`status=passed`，`harness_revision=ordinary-transactions-v4`，`errors=[]`。
- 实际 `active_seconds >= 7200`，与 windows 汇总一致；保留 UTC start/end 与 `elapsed_seconds`。
- commits、successful readers、故意 full rollback、committed savepoint rollback/release 各有实际覆盖。
- `final_model_verified=true`、`verified_reopens > 0`；rollback checkpoint 为明确不支持的 skip，WAL profile 有相应成功 checkpoint。
- 按 seed、连续 ordinal 和 C/R/B/X/F history 独立重建最终 generation/hash，而不是只比账户总和。
- `unverified_retirement=false`、`unverified_close_handles=0`、`close_failures=0`、`shutdown=workers_joined_normal_close`；字段缺失不能按零补齐。
- 源码前后不变，目录/二进制/负载身份完整，实际保存数据与原始 report 均保留。

注意四个 reader 是启动数量；报告聚合成功数不证明每一个 reader 都持续取得进展。要披露窗口饥饿或停顿。P50/P95 是对数 histogram 桶上界，不能伪装成精确经验分位；correlated windows 的 confidence bounds 为空是诚实状态。

任何非零、失败/不完整、unsupported、quota、未知 retirement、CI 或源码身份变化，都停止启动下一个 profile，保留所有原文件，不自动重试。一个正常关闭成功不能清除另一个 reader 的 close 不确定性。

### 11.4 grams 性能和更大负载

已有普通 grams harness 可用于未来用户批准的小规模描述性观察：

```bash
# 文档模板。本轮不得执行。
capture grams-harness-build env CGO_ENABLED=0 go build -trimpath \
  -o "$RUNROOT/bin/grams-benchmark" ./benchmarks/grams || exit 1
capture grams-smoke "$RUNROOT/bin/grams-benchmark" \
  -rows 128 -queries 6 -repeats 2 -seed 2026101052 \
  -page-size 4096 -max-pages 32768 \
  -source-id "$SOURCE_ID" -environment "$HARDWARE_ID" || exit 1
```

核对每个 ordered 结果与真实 scan 100% 一致、选择性查询真实 posting work/较少 source fetch、fallback 单独统计、重开与频率一致、P50/P95/raw timing、构建时间和空间放大全部保留。不能仅据 `status=passed` 断言比扫描快。

10000 行/96 queries per length/5 builds grams 和 10000 attempts/1024 rows 事务 throughput 只是已有较大 profile 提案，未完成资源授权与运行。本文不把它们变成自动启动命令。性能比较要同机同数据、固定 source/workload/hash、独立交错重复、预声明指标/回归阈值及统计方法；INCONCLUSIVE 不能填 PASS。不能激活未部署的性能 CI 来替代人工审查。

## 12 受限人工门禁与不能编造的入口

这一节只保存真实源码入口的说明。**在当前平台禁止执行，未来也须先明确独立授权和审查；设置环境变量本身不产生授权。** 未实现的测试只列需求和需要补齐的证据，不给不存在的程序名。

### 12.1 已有有限 rollback 恢复 harness

已核对文件：`internal/pager/recovery_manual_test.go`。

- build tag 精确为 `manual_recovery && linux`
- 入口 `TestManualRollbackRecovery`
- gate `GOSQLITE_MANUAL_RECOVERY=independently_authorized`
- gate 缺失时 `t.Fatal`，文字为 NOT_RUN；这个退出不是产品恢复逻辑的失败证据

编译命令只生成测试二进制，但**本轮连编译也不执行**：

```bash
# 仅供未来经独立授权后采用；本轮 BLOCKED / NOT_RUN。
capture manual-recovery-build env CGO_ENABLED=0 go test -c -tags manual_recovery \
  -o "$RUNROOT/bin/manual-recovery.test" ./internal/pager || exit 1
```

现有精确执行形状：

```bash
# 当前平台禁止执行；不得迁移重试或作为解锁办法。
capture manual-recovery-run env GOSQLITE_MANUAL_RECOVERY=independently_authorized \
  "$RUNROOT/bin/manual-recovery.test" \
  -test.run '^TestManualRollbackRecovery$' -test.v || exit 1
```

源码中的确定性子场景包括：

- `restore-truncate-and-idempotent-reopen`：恢复两页、匹配原字节、journal 移除、再次恢复不发生 replay、正常重开。
- `original-empty-extent`：恢复原空数据库 extent。
- `preserve-checksum/cold/empty/trailing/multiple-segments/truncated/journal-limit/empty-database/canceled`：拒绝所构造的非法/不支持输入且原 db/journal 字节保持。
- `busy-shared-reader`、`busy-reserved-writer`：返回 BUSY 并保持文件。

`preserve-multiple-segments` 是该 fixture 的具体拒绝场景，不能概括为所有多段 journal 均不支持；当前 parser 已支持有界、对齐、有限记录数的部分多段格式。future reviewer 必须重新核对 fixture 与当前 parser 的一致性。

即使这些确定性场景将来通过，也只覆盖它们本身：没有真实掉电/随机 kill/short write/Sync failure/ENOSPC/wrong-offset/torn-sector/checkpoint 中断/rekey 中断证明，也没有 public Open 自动恢复的实现证明。

### 12.2 已有两个手工 namespace 回归

文件 `sqlite/write_namespace_manual_test.go` 的 tag 为 `manual_namespace`；两个真实测试名：

1. `TestNativeConnectionKeepsResolvedSymlink`
2. `TestNativeTransactionDetectsReplacedValidFile`

两者都要求 `GOSQLITE_MANUAL_NAMESPACE=independently_authorized`；未提供时 `t.Skip`。因此二进制返回 0 但全 skip 不是通过。

历史 M2 至 7addc493 的全套曾错误地包含这两个 namespace 测试，其当时 PASS 仍是事实，不能改称“历史从未运行”；但那些全套不能被重新标作完全干净的 ordinary-scope 证据。当前独立受限门禁仍是 BLOCKED / NOT_RUN。

```bash
# 仅供未来独立人工审查；本轮不编译、不运行。
capture manual-namespace-build env CGO_ENABLED=0 go test -c -tags manual_namespace \
  -o "$RUNROOT/bin/manual-namespace.test" ./sqlite || exit 1

# 当前平台禁止执行；环境变量不是授权，不能用于绕过限制。
capture manual-namespace-run env GOSQLITE_MANUAL_NAMESPACE=independently_authorized \
  "$RUNROOT/bin/manual-namespace.test" \
  -test.run '^(TestNativeConnectionKeepsResolvedSymlink|TestNativeTransactionDetectsReplacedValidFile)$' -test.v || exit 1
```

应审查的具体断言：symlink retarget 后已打开连接仍写原 first 数据库，second 不被改；valid-file replacement 后 commit 返回 `sqlite.ErrChanged`、rollback 正常、替代文件中的行保持原样。这不是所有 hostile namespace、权限或 hardlink 问题的充分验收。

### 12.3 OS raw namespace 测试门控的当前状态

旧 776b 基线的 `native_windows_test.go` / `native_darwin_test.go` 中两项测试
只有 OS 约束、没有 manual gate，这是原始审计事实。本批在当前 P0 基础上
逐 hunk 移动这两个完整函数，原断言与操作保持，仅首句增加环境 gate；
当前入口为 `internal/encstore/namespace_manual_windows_amd64_test.go` 与
`internal/encstore/namespace_manual_darwin_test.go`。它们不同于第 12.2 节的
两个 plaintext 测试。真实测试名、平台、标签、可复制人工步骤和证据边界见
[第 19 节](#19-os-raw-namespace-门控与独立原生验收2026-10-11)。

本环境仍禁止执行这两个原生测试；文档步骤仅供用户在独立批准的目标环境
自行验收。设置 sentinel、源码门控、编译成功均不产生执行许可；不启动新 OS runner。

### 12.4 没有完整实现入口的门禁

完整 harness 缺失的部分统一标记 `IMPLEMENTATION_MISSING`；其中受平台限制的执行另记 `BLOCKED / NOT_RUN`。部分已有源码并不补齐整个矩阵。

| 门禁 | 当前可诚实说明的情况 | 将来所需材料，不是现成可执行测试 |
|---|---|---|
| 真正 crash/process-exit/power-loss | 本环境 BLOCKED / NOT_RUN；有限恢复 fixture 不是它 | 审查后的 fixture 协议、已提交/未提交外部事务账本、注入点、恢复点、存储持久化条件及逐行/索引验证 |
| I/O short write、Sync 错误、写错位、disk full、权限失败 | 没有可在本文冒充齐全通过的现成入口 | 明确的可丢弃设备/注入后端、影响范围、失败点矩阵及残留所有权证明 |
| WAL crash/checkpoint/tail publication | 普通成功路径有证据，尾部 publisher 等仍有实现缺口 | 先补真实源码/公开恢复入口，再审查其失败语义；不能先写一条 TestCrash 命令假装存在 |
| 错 key/盐/tag/单字节密文修改/跨页置换/main-WAL 不匹配 | 密码学内存向量不覆盖这些磁盘链路；BLOCKED / NOT_RUN | 审查后的测试数据与确切目标、认证失败/无秘密泄露/无错误成功/来源保持的核验 |
| interrupted publication/rekey | 正确密钥普通 rekey 不覆盖中断；BLOCKED / NOT_RUN | 有完整失败点协议，证明不存在半新半旧被误接受，保留旧/新 generation 判定与恢复规则 |
| selected-file 缺失/namespace 干预/原生 lock unmap close 失败 | 内存 owner laws 不是原生失败注入证据 | 独立原生平台和文件系统证据，所有诊断/retirement obligations 的审查 |

表中列的是原规范要求，不是对未实现功能预先颁发通过标准。只有真实入口、准确可执行规程和独立证据齐备之后，才可以制定最终场景断言并执行。不要使用 `go test -run TestCrash` 匹配不到任何测试仍退出 0 的形式蒙混。

## 13 Fuzz 与中断原因的人工审查前置

本轮不运行任何 fuzz，包括内存-only fuzz。历史有界 campaigns 保留原执行次数与源码身份；它们不能代替被中断的新 41 目标活动。

精确已知：

- 约 22:30 收到内容可能涉及网络安全风险的阻止消息；最后成功记录还在审查 targets。
- 有限只读查找未找到新的 campaign 目录、执行 log 或结果；这不证明从未发生任何未记录动作。
- 22:39 平台安全错误触发的具体 operation/target 在可用记录中未知，附近活动为 ANN V4 修正及独立评审。
- ANN V4 目录未冻结，只有 V3 copy 与一个未改字节的 review probe；不能声称 validator/engine/ownership 修复已实现。

后续人工必须先取得足够的原操作/目标/工具错误和范围信息，判断何者被拒绝、是否有未收集执行、哪些独立操作合法；无法查清时继续 UNKNOWN/BLOCKED。不得重新投递原链路、改任务措辞、拆成更小命令、换参数、换执行环境或把它迁入 CI。本文不提供 41-target campaign 或 ANN V4 修正的“恢复命令”。

## 14 故障排查与停止规则

| 现象 | 先做的只读核对 | 不允许的处理 |
|---|---|---|
| Go 版本不是 1.27.2 | PATH、工具绝对路径、`GOTOOLCHAIN` | 自动切换新 Go 后继续写同一基线 PASS |
| oracle 身份或源码 hash 不符 | 绝对路径、archive、source ID、原构建 log | 跳过 hash/改 expected/换系统 sqlite 冒充固定基线 |
| 离线模块缺失 | GOPATH/module cache/go.sum 来源 | 不告知地改网络、下载未核验模块 |
| testscope 拒绝 | GOFLAGS、实际测试 body/helper、当前 gate 源码 | 删除 guard、加豁免、改名字绕过 |
| `no tests to run` 或 SKIP | 真实函数名、package、OS/tag、env gate、oracle 选择 | 把退出 0 当目标已覆盖 |
| 数据库 unsupported | 支持矩阵、layout/mode/schema/平台 | 自动转模式、换成内存库、丢不支持对象后称通过 |
| BUSY/canceled | 正常锁/事务/cursor 生命周期与完整 error graph | 把 mixed I/O/cleanup 错误吞成普通 BUSY |
| mismatch/ErrChanged/旧回归出现 | 保留 exact query/seed/source、输出/输入 hash | 改排序/容差/断言、删除失败 fixture、静默重建 |
| quota/低空间/内存压力 | 实际 TMPDIR/mount、bytes/inodes、报表预算 | 故意挤满磁盘、删除历史、缩样本补 PASS |
| 没有 JSON、运行时间长 | 既有 session 状态、stdout/stderr 增长、维护阶段 | 强杀/timeout 后叫作正常 completion 或 recovery test |
| close/retirement 不确定 | 完整诊断与持有对象/目录记录 | 再调用一次返回 nil 就抹去第一次不确定性 |
| CLI backup/rekey 非零 | 已有结果、部分输出和原错误，先不改任何文件 | 假定未发布、自动重试、删除潜在部分输出 |
| windows vet unsafe 告警 | 精确包/命令/源位置与历史差异 | 关闭 checker 后声称原门禁通过 |
| 性能较慢/区间重叠 | 同机同数据身份、原始重复、cache/负载 | 挑最好样本、丢慢结果、捏造 SQLite/pgvector 对照 |
| 平台/审查拒绝 | 精确操作、对象、授权与原错误；保留 BLOCKED | 改写、绕行、外移或另起相同链路 |

任何错误都先保留证据。手工规程没有权限改变诊断对象、系统安全设置、存储挂载、OS 权限、CI 额度或终止其他进程。需要这些操作时另行决定和授权。

## 15 证据目录与清理

建议每次独立验收保存以下结构；这里是目录说明，不是已经生成的结果：

```text
RUNROOT/
  source.tar
  source/                         精确导出的源码副本
  bin/                            本轮构建及 hash
  data/                           显式合成数据库与导入数据
  # 普通临时根在单独的新 /tmp/gosqlite-oracle-*，路径另存 oracle-temp-root.txt
  go-cache/                       本轮编译缓存
  evidence/
    source-commit.txt
    source-tree.txt
    source-archive.sha256
    source-before.sha256
    source-after.sha256
    goal.sha256
    <step>.command
    <step>.started
    <step>.finished
    <step>.stdout
    <step>.stderr
    <step>.exit
    results.tsv                   人工逐项结论和范围
    blockers.md                   未运行 缺口 已知失败和审查意见
    host-and-storage.md            非秘密环境与资源记录
```

`results.tsv` 至少包含 case ID、source/tree/binary hash、平台、实际命令、状态、退出码、开始/结束、断言、日志路径、未覆盖范围、审核人。日志中不能保存秘密、生产数据或凭据环境。

所有未来普通步骤结束后，核对源文件未被修改：

```bash
# 文档模板。本轮不得执行。
cd "$WORK" || exit 1
find . -type f -print0 | LC_ALL=C sort -z | xargs -0 sha256sum > "$EVIDENCE/source-after.sha256" || exit 1
cmp "$EVIDENCE/source-before.sha256" "$EVIDENCE/source-after.sha256" || exit 1
sha256sum "$RUNROOT/bin/gosqlite" > "$EVIDENCE/cli-binary-after.sha256" || exit 1
cmp "$EVIDENCE/cli-binary.sha256" "$EVIDENCE/cli-binary-after.sha256" || exit 1
```

如果差异来自测试写回仓库，也仍是差异，要保留且解释；不能先 restore/删除再补“前后相同”。源码 hash 只证明所读文件身份，不证明测试实际执行；还需要终态日志和断言。

清理顺序：

1. 先确认本次所有子进程和 worker 自然结束、Rows/Tx/DB 正常收尾，无 pending retirement。不能因 shell 返回就推定原生资源状态明确。
2. 完成 `results.tsv` 与 blockers，保存全部失败/不完整/未知记录。复制或归档已经正常关闭的合成 fixture 与 evidence，校验归档可读和 hash。
3. 有异常 close、部分输出、源码变化或未知状态时，操作者默认保留整个 `RUNROOT`，不再自动做清理。这无法撤销现成示例内部已经发生的删除：第 9 节所列示例可能在失败时清理私有 fixture，普通 go test 的 t.TempDir 也会自动收尾；这些局限必须记录，不能宣称完整失败现场仍在。不要额外删除失败现场、WAL/journal/加密旧 generation 或 fuzz corpus 来让下一次通过。
4. 对 RUNROOT 和单独的 ORACLE_TMP 分别确认归属与记录。只在操作者再次确认路径、资料保留策略和没有未闭合对象后，手工删除自己明确生成且可丢弃的目录。本文故意不提供宽泛 `rm -rf` 或 namespace sweep。
5. 第 9 节示例会自行清理私有临时目录，其中包含错误返回时的无条件清理和未报告的清理错误；transaction harness 则保留数据目录。逐例记录，成功退出不证明任意失败路径都已安全退休，不假定所有工具行为相同。
6. 删除临时凭据/文件不是安全擦除，不能因此声称 GC、SSD、快照或旧 encrypted history 不可恢复。真实秘密根本不应进入本验收。

## 16 最终通过标准

### 16.1 普通有界复验可以怎样结论

只有对应 exact source、本次真实命令、非缓存执行、退出码、断言与证据完整，才可写：

“本次在已记录的 Linux/amd64 独立环境，对提交 X 的普通无 CGO unit/vet/build、单独 race、完整固定 SQLite oracle 和列明功能路径验收通过。受限故障恢复、安全/平台和未实现能力不在该结论内。”

不得写“所有测试都过”“SQLite 全兼容”“M0–M10 全部 VERIFIED”，除非所有相应规范门槛真实完成。focused test、编译、普通 Check、fixture recall、短期 smoke 或 CI SUCCESS 都不能单独得到这些结论。

### 16.2 完整原规范仍需关闭的事项

原始目标的完整验收至少还要：

1. 真实实现缺口完成并在精确集成源码上审查/验证，包括恢复/public API、WAL tail、通用备份、只读 WAL Check 和已声明支持的 SQL/planner 差异。
2. 通过独立合法取得的物理/崩溃/恢复/故障证据，验证已提交数据、未提交数据、表/索引原子性与所有权状态；不能用内存模拟或普通 checkpoint 替代。
3. 真实目标 OS/架构文件锁、mapping、目录持久化和加密路径证据，明确平台降级能力；跨编译不足。
4. 加密错误密钥/认证/篡改/中断 rekey 与威胁模型审查；不能用 ciphertext marker absence 或正确密钥 round-trip 代替。
5. 预声明且实际完成的长期并发 soak、大规模真实测量、可解释置信度和性能回归策略；记录所有慢结果和失败。
6. 数据格式升级/回退、用户文档/API、许可/来源、安全/密码学独立审查与适用真实迁移环境验证。
7. ANN 报告准入及未完成 fuzz 证据问题经合法人工审查解决；不知道触发原因时不得假装可恢复执行。

本手册只整理规程，当前总状态仍为 **PARTIAL / 完整验收未成立**。

## 17 来源与复核说明

本轮只读依据包括：`goal.md`、`docs/manual-verification.md`、`docs/acceptance-status.md`、新增中文 `docs/interruption-20261010.md`、`docs/compatibility.md`、`docs/upstream.md`、`docs/native-check.md`、`docs/cli-encryption.md`、`docs/import-pgembedding.md`、`docs/import-distance-verification.md`、benchmarks 的 README/源码、实际测试声明、脚本及相关 CLI 实现。

另对照原开发空间 `durable-restart/` 下的 `INTERRUPTION-20261010T2239Z.md`、`long-soak-plan-20261010/PLAN.md`、`long-soak-plan-20261010/INDEPENDENT-REVIEW.md`、`native-scope-independent-review-v1/report.md`、`ann-harness-independent-review-v3/REVIEW.md`。这些不是本仓库内路径，也不保证每个最新版本都已持久保存；各 Library 身份和仅本地材料边界见中断记录。不同时间/候选身份没有合并成一个“通过的版本”。较早文档中的历史缺口与后来的实现可能不同，以精确提交和具名专题为准。

初始只读快照中的关键文件 SHA-256，文档同期更新不反向改变这些初始读数：

- `scripts/check.sh`：`55851844f35325b38fbd368558af4e229d5bfe7c42928685e57dd799274b8f18`
- `scripts/test-oracle.sh`：`9a486b551af436f706d6018f717a2a2d0c54f9ae443c43cee10638105bf320a4`
- `docs/manual-verification.md`：`370aefba6b411769a88cab83768bece4b65606dd9c41169add950061bf1011fe`
- `docs/acceptance-status.md`：`79645fe4267453dbe419df29dae8708c0b3f6fbd7c0592cc9ba9a1b0b4eff114`
- `INTERRUPTION-20261010T2239Z.md`：`13092da4ed0e6ca3d7633dc4951a9ac5d0ab18060f6bbd55a4330d39a3a2a5d2`

本文作为 docs/manual-verification-zh.md 收入项目，保留原 docs/manual-verification.md 的历史记录；没有把历史日志复制为本轮成功证据。


## 18 P0 public Open rollback 恢复源码集成（2026-10-11）

本节补充第 3.2 / 12.1 节的历史状态，不改写其当时结论。候选从已发布
`63c2ebb10e3a2c4ff2378ee9f33c46133134c70f` 按已审查的独立 hunk 构成，
本批在已保存 `c1502790c4b233757b0f9a3c4d5e5bdee1321310` 上组合并保留
LIMIT 修正和限制分类；有限实验性源码已接入
公开 opt-in、pager retirement/path proof、driver limits 值复制及 testscope
恢复防误执行 guard。这里的源码集成、编译和 descriptor-free 模型检查，
与真实恢复运行验收是两回事。**真实 `pager.RecoverRollback`、开启选项的
真实 `Open/OpenContext`、production bridge 和所有物理恢复仍为
BLOCKED / NOT_RUN。** 自动默认恢复仍未启用，M3/全项目验收没有完成。

### 18.1 实际 API、平台和预算

- 入口为 `sqlite.Options.RollbackRecovery *sqlite.RollbackRecoveryOptions`；
  nil / 零值 Options / 未配置的注册 driver 均继续拒绝已有 journal。
  `errors.Is(err, sqlite.ErrRecoveryRequired)` 与 `CodeIOErr` 保持；presence
  诊断文本及私有包装有变化，不能按错误字符串字节或旧链形状判断兼容。
- 只有 Linux、可写、明文、rollback 模式允许非 nil。`ReadOnly` 返回
  `ErrReadOnly`；加密、`WALExclusive`、`WALShared`、
  `WALPowersafeOverwrite`、`KeepWALOnClose` 和非 Linux 返回
  `ErrUnsupported`，这些策略拒绝发生在原生 acquisition 之前。
- `MaxJournalBytes` 默认为 64 MiB，`MaxRecords` 默认为 65536（跨段累计），
  `MaxDatabaseBytes` 默认为 1 TiB 原数据库 extent；各字段 0 选择默认值。
  内存需约两倍 journal bytes 加 O(records) 元数据，不是进程 RSS 上限。
- `NoCreate: true` 不妨碍已有文件恢复，但禁止普通首次缺失文件创建。
  恢复后的 fresh probe 永不创建。API / Connector 都复制 options 指向的值；
  调用者不能在构造连接的同时并发修改配置。
- 没有 recovery CLI flag、DSN 参数或独立 public Recover 函数。
  `driver.NewConnector` 接收 Options，URI `mode=ro` 保留只读并于实际
  Connect 拒绝恢复，`mode=rw` 仍保留 NoCreate；不得静默升级权限。

下面是实际 API 的配置形状，**只展示给未来具备独立授权的人工操作者审查；
本次没有编译或运行该示例，也不能把它放进 ordinary 测试绕过 guard**。
`path` 必须由操作者选定为隔离可丢弃的已有数据库，不能指向重要数据。

```go
opts := sqlite.Options{
    NoCreate: true,
    RollbackRecovery: &sqlite.RollbackRecoveryOptions{
        MaxJournalBytes: 64 << 20,
        MaxRecords: 65536,
        MaxDatabaseBytes: 1 << 40,
    },
}
db, err := sqlite.OpenContext(ctx, path, opts)
// 必须检查 err；只有 err == nil 时才使用 db，并检查 db.Close() 的结果。
// database/sql 的配置入口：
// connector, err := driver.NewConnector("file:/absolute/disposable.db?mode=rw", opts)
// pooled := sql.OpenDB(connector) // 实际 Connect/Ping 才可能执行恢复。
```

### 18.2 本次允许的 source/model 复核

先完整审阅 [专题文档](rollback-recovery-open.md) 所列精确测试及 helper，
再运行其 guard、descriptor-free selector 命令。测试只使用内存 owner、
错误对象、callbacks 和 AST/source 字符串；不构造开启恢复的真实 Open。
若选择 race，只把同一精确 selector 换成 `CGO_ENABLED=1 go test -race`，
并先重跑 guard。不运行全套旧 guard/普通/oracle suite，更不添加 manual tags。

`internal/testscope` 能识别直接/捕获的恢复函数、恢复字段、同文件 alias/
defined type 和 dot-import 的 unkeyed Options。它不是完整类型或调用图证明：
跨文件 alias、推断 composite-literal 元素类型可漏检；同名 shadow 类型可误报；
helper / reflection 仍须人工审查。本次测试没有利用这些缺口。独立的
Rename/rename selector 扩展和 Windows/Darwin raw namespace 门控不在此切片，
不要据此启动新 OS runner 或 native aggregate。

### 18.3 未来物理验收的准确范围和停止条件

本节不给代理新的执行权限，也不提供绕过受限链路的另一套命令。
未来人工必须先取得合法独立授权并明确 fixture、原数据库/sidecar 身份、
原始/已提交/未提交账本、预期页与 schema、各阶段 evidence 和真实测试入口。
现有 `internal/pager/recovery_manual_test.go` 仍是第 12.1 节的内部 harness：
`manual_recovery && linux` 与 `GOSQLITE_MANUAL_RECOVERY=independently_authorized`
是其实际门控，**它不调用新 public Open bridge，不能作为其验收结果**。
本次没有新增 public physical harness，其状态明确为 **IMPLEMENTATION_MISSING**，
执行另列 **BLOCKED / NOT_RUN**，因此不能声称已有一条可完整覆盖它的
`go test -run ...` 命令。环境变量本身不是授权。上面的 API 配置片段也不是
fixture 生成器、物理验收 harness 或可直接运行的完整程序。

独立审查和未来 harness 至少需要逐项固定这些断言，再按许可执行：

1. nil-default 持续拒绝 journal；唯有准确 resolved path 的 private proof，
   且失败 reader 已完全退休、没有其他 cleanup 诊断，才进入显式恢复。
   wrapped/joined/synthetic `ErrRecoveryRequired` 不授权恢复。
2. 内部操作自己获取 owner 和 locks、重新验证完整 journal/database；最多
   一次恢复，成功后最多一次新的只读、no-create probe。不能复用初始 admission、
   自动 cleanup retry、递归 Open 或在 reprobe 失败后创建数据库。
3. 成功 reprobe 必须 close；任何恢复/probe/close/cancel/ownership 错误要保留
   初始与后续 cause/token，返回空 metadata。历史 `ErrRecoveryRequired` 使
   primary code 为 `CodeIOErr`；BUSY/cancel/limit 可通过 `errors.Is/As` 找到。
   它不是当前磁盘仍需恢复的证明：恢复成功后取消或重探测失败仍可使 Open 失败。
4. 支持有限记录数、已发布的 DELETE journal 多段子集；页大小 512..65536，
   recovery sector 32..65536，重复页/geometry 不符/不完整 extent 拒绝。
   cold/empty/ambiguous、super/master journal、0xffffffff no-sync count、
   legacy 未指定页大小和 unsupported header 不丢弃。不认证恶意文件内容。
5. WAL/SHM 包括空 sidecar 都阻止 rollback recovery；不截断或重建 WAL。
   原 extent 为 0 可使恢复后文件为空，fresh probe 此时拒绝且不创建。
   后续 Query/Begin/Exec 永不再次触发恢复。敌对 namespace 替换不在保证内。

遇到缺入口、受限操作、源码身份不符、owner 未退休或审核拒绝即暂停；保留
确切 operation/target/error，不能换输入、改名字、换机器/CI 或静默重试。
ANN V4、41-target fuzz、broader journals、WAL publisher、retained-directory、
backup/backend 和物理 fault/security-negative 的状态不因本节改变。


### 18.4 已取得与尚缺证据逐项记录

| 项目 | 本批准确状态 | 记录方法与不能推出的结论 |
|---|---|---|
| nil-default / policy / proof / sequence / error / option-copy / guard | 独立候选 source/model/compile PASS | tree `75c074548e8b5208beb06e6ead314d835cd16f4f`；11项精确测试各在CGO0和CGO1 race通过，无skip；只含无描述符模型及源码AST |
| guard、四包vet、全包CGO0 build、无CgoFiles闭包 | 同一候选PASS | 编译与依赖检查没有运行production bridge；不证明物理效果或其他OS运行 |
| 候选源码身份和补丁重放 | 独立PASS，07:49:35 UTC完成 | 原63c2基线、候选前后及重放身份对应；不把候选tree当成c150加P0的组合tree |
| c150加P0的完整普通/oracle/race组合 | 本节写作时PENDING | 必须记录该精确组合的终态；旧LIMIT、63c2或候选PASS不能代替 |
| 本批新提交、精确恢复和CI | 本节写作时PENDING | 提交、保存、恢复与CI分别核验；不预填未知SHA或SUCCESS |
| 实际RecoverRollback / public bridge / 启用恢复的真实Open或Connect | BLOCKED / NOT_RUN | 本轮没有调用；源码模型不能证明恢复后的文件、锁、目录或故障语义 |
| public physical harness | IMPLEMENTATION_MISSING | 现有内部手工测试不覆盖它；没有真实入口时不编造命令或测试名 |
| 更广journal/default-on/WAL publisher/其他原生平台 | 未完成的实现或策略范围 | source/model PASS不改变原缺口及物理资格限制 |

独立候选的11项精确测试名、完整命令形状、固定工具链、离线依赖、资源阈值
和原始证据身份见[公开恢复专题](rollback-recovery-open.md)。每步单独记录
实际命令、UTC起止、stdout/stderr、exit、测试匹配数/skip及源码前后哈希；
不应把本节整段提取执行，不因外层shell为0就吞掉内层FAIL。

未来public physical harness的准备还须单独规定：

1. 选择并记录精确新提交和测试源码/二进制身份，核对真实Linux/文件系统与
   持久性条件。使用独立可丢弃已有数据库家族及外部账本，不操作生产数据。
2. 按18.3逐项建立可审查fixture、预期页/schema/表索引结果、锁与retirement
   断言、恢复前后journal/DB/WAL/SHM身份和目录证据，区分成功、拒绝与不确定。
3. 先补齐且审阅public入口harness与显式独立门，再由有相应权限的操作者决定
   具体执行；当前缺少这一步，因此没有公开bridge的运行命令。
4. 任一异常、owner未退休或结果未知时停止，不再次Open、重试恢复或清理
   journal/WAL/旧generation以求成功；保留原始cause/token、完整日志及fixture。
   只有正常退休和证据保存都确认后，才按第15节人工清理自己创建的目录。

这些是未来验收协议要求，不是已经实现的测试断言或新增执行许可。更广物理
crash/fault/tamper/rekey矩阵仍按第12.4节记IMPLEMENTATION_MISSING与受限执行。

## 19 OS raw namespace 门控与独立原生验收（2026-10-11）

### 19.1 源码身份、实际入口与已取得证据

本批组合基点为已保存 P0 提交 `56bbcf3febe233197aa0a5eac965c67a67a88011`、
tree `c31e7dcbeed9732688681741ee38ec60ff0bfb49`。已审阅六路径候选以 P0
运行冻结 tree `442a7104dc780ddb20826e5ee4f1250b291f2b24` 为基础，得到
`01918609a55cc149c5155451da40f7389854db06`；这不是含当前文档的新提交身份。
只改两个旧 native 测试文件、两个新增 manual 文件、`internal/testscope/main.go`
及新增 inert guard 回归 `internal/testscope/native_namespace_test.go`；无生产实现或 CI 改动。

| 实际具名测试 | 当前文件与平台选择 | 精确双门 |
|---|---|---|
| `TestWindowsEncryptedRelativePublicationAndRetainedRoot` | [namespace_manual_windows_amd64_test.go](../internal/encstore/namespace_manual_windows_amd64_test.go)，windows/amd64 | 首行 `//go:build manual_namespace`；首句要求 `GOSQLITE_MANUAL_NAMESPACE=independently_authorized` |
| `TestDarwinEncryptedRelativePublicationAndParentLifetime` | [namespace_manual_darwin_test.go](../internal/encstore/namespace_manual_darwin_test.go)，本规程面向 darwin/amd64、darwin/arm64 | 同一精确 tag/env gate；Darwin 后缀保留 Go 原有 iOS/Darwin 选择关系，不据此新增 iOS 原生验收 |

两个 gate 在 `t.TempDir()` 和原生调用之前，缺值或值不匹配会 `t.Skip`。
去掉且只去掉该首句 gate 后，两个完整函数与原版逐字节相同。默认与仅 oracle
选择在适用平台刻意减少这两个声明，其他原有声明不减少；不是全量 native
覆盖保持不变。manual 文件各只含一个原测试，没有新的 initializer 或 helper。
Windows 原文件保留当前 shared-rekey preflight 所需 `context` import。

此句仅记录 namespace 门控的历史源码。新 Windows 九路径组合已把旧 preflight
拒绝测试替换为 `TestWindowsEncryptedSharedRekeyCapability`，并移除该无用
`context` import；manual namespace 文件及其 gate 原样保持。见第 22 节。

通用 selector switch 只增加 `Rename` / `rename`，原 P0 恢复引用、同文件
alias/type、dot-import、unkeyed Options 检查保持。新增纯源码测试为
`TestNativeNamespaceMethodReferenceGuard`，只解析字符串，不执行字符串里的操作。
selector 规则保守匹配接收者，可检查调用/方法值/方法表达式等，但不是完整类型、
调用图、沙箱或物理安全证明。bare helper、改名 wrapper、反射/动态调用及未来 API
仍需人工审阅；精确 manual tag 豁免也不能单独证明 body 有环境门或行为安全。

| 证据范围 | 已发生结果 | 不覆盖什么 |
|---|---|---|
| 全部六路径、完整函数、其余文件与补丁重放 | owner 与独立源码审阅 PASS | 不代表原生运行 |
| guard、纯 AST/source-string 测试、同选择 race、manual GOFLAGS 预期拒绝 | owner 08:23:50 UTC 自然完成；独立追加检查 08:27:31 UTC 自然完成，PASS | GOFLAGS 拒绝为预期 exit 1；不是物理测试失败或许可 |
| 47 个 Go 支持目标 × default/oracle/manual_namespace/oracle,manual_namespace | 两方各自 188 项实际 `go list -e -json` 选择 PASS，无 package error | 不是 47 平台运行或支持承诺 |
| windows/amd64、darwin/amd64、darwin/arm64 × 两组 manual tags | 两方各自六项 `go test -c` 与精确测试符号核对 PASS | 二进制仅编译/读取符号，全部原生执行 NOT_RUN |
| 本批新组合完整普通门、新提交、恢复与 CI | 各自待精确身份与终态记录 | P0/LIMIT 的旧 PASS 不能代替本批 |
| manual/native 物理门 | BLOCKED / NOT_RUN | 本文没有执行以下任何命令，也没有开启 OS runner |

已保存的[owner 报告](evidence/namespace-gating-20261011/owner-report.md)、
[独立源码报告](evidence/namespace-gating-20261011/independent-source-review.md)与
[独立检查追加报告](evidence/namespace-gating-20261011/independent-checks-addendum.md)
分别对应原工作目录中的 owner report、source review 和 compiler addendum；
[证据清单](evidence/namespace-gating-20261011/evidence-files.json)记录材料身份。
独立原报告的 source-only NOT_RUN 保留，后续 addendum 只追加获准的编译/模型结果。
这些已选材料没有复制全部外置原生二进制或完整工作目录；需要更深重放时先按
清单取得对应原文件，不能凭文件名补造结果。当前章节只做静态核对，未重跑检查；
本批持久身份由后续真实记录闭合。

### 19.2 独立用户验收前置与隔离副本

下列步骤只供用户在另行批准的独立原生环境自行采用；**当前代理不执行、不转交
其他执行器、不迁移 CI 作为绕行**。sentinel 仅防止误启动，不能代替授权。
这里只验两个原有 namespace primitive 回归，不验真实加密容器安全、恢复或故障。

- 主机须为 Windows/amd64，或真实 macOS 的 amd64/arm64。不得在 Linux、WSL、
  iOS 或架构模拟层运行后冒称对应原生结果；架构/虚拟化条件均单独记录。
- 固定原生 Go 1.27.2 和已验证离线模块缓存；依赖版本见本手册 4.1 / `go.mod`。
  使用 Bash（macOS Bash 或 Windows Git Bash）、Git、tar、Python 3；缺少工具先停止，
  本文不安装软件。Windows 的 Go 必须是 Windows 原生工具链，不是 WSL 的 Linux Go。
- 提前记录 OS/build、CPU/架构、真实文件系统/卷与挂载、空间、inode 或等价限制、
  权限及持久性语义。Windows 无 Unix directory barrier、Darwin fsync 非 F_FULLFSYNC
  的既有限制不变。空间不足或平台条件不符即 BLOCKED，不临时改断言/权限/挂载。
- 只用独立、可丢弃且不含用户数据的新目录。不得使用生产路径、唯一备份或活库家族。
  两测试只写合成 `first`/`after` 字节，不能误称实际加密数据库验收。
- 本节的新模板要求 `SOURCE_COMMIT` 同时包含已审阅的六路径 namespace 门控、
  `internal/testoracle` startup helper 和新版 `internal/testscope` startup guard，
  并由用户填入该实际审阅提交的完整 SHA 及 `SOURCE_TREE`。56bb 是原父基点；
  50b/b349 虽已含 namespace 门控，仍缺新版 startup helper/guard，**都不能用于
  下面这个新版模板**。旧版本的手工模板仍保存在各自原提交中，不追改其历史。
  不把未提交 worktree、候选 tree 或动态 `HEAD` 当作固定 commit 来源。
  后面的文件/guard 文本检查只是版本准入，不能代替对完整源码的审阅；本节也
  不运行 SQLite child，不授予 Windows/Darwin 原生 oracle 资格。

以下只建立本次独立副本，变量的示例路径须先替换；Windows Bash 路径可用 `/c/...`。
执行前还须按第 4.3 节在本专用终端定义原有 `capture` 函数，日志名不重复使用。

```bash
# 文档模板：仅供未来独立授权的用户逐步执行。本次 NOT_RUN。
SOURCE_REPO=/absolute/path/to/reviewed/gosqlite
BASE=/absolute/path/to/disposable-native-acceptance
TOOLCHAIN_BIN=/absolute/path/to/native-go1.27.2/bin
# 用户须先设置 SOURCE_COMMIT、SOURCE_TREE 和 GOPATH；GOPATH 为已核验缓存的原生绝对路径。
: "${SOURCE_COMMIT:?填写同时含namespace六路径与新版startup helper/guard的审阅提交全SHA}"
: "${SOURCE_TREE:?填写该提交的实际tree}"
: "${GOPATH:?填写已验证离线模块缓存的原生路径}"
test -d "$BASE" && test -d "$SOURCE_REPO/.git" || exit 1
test "$(git -C "$SOURCE_REPO" rev-parse "$SOURCE_COMMIT^{commit}")" = "$SOURCE_COMMIT" || exit 1
test "$(git -C "$SOURCE_REPO" rev-parse "$SOURCE_COMMIT^{tree}")" = "$SOURCE_TREE" || exit 1
# 新模板同时需要 namespace 门控与 startup helper/guard，拒绝旧 50b/b349。
for p in internal/encstore/namespace_manual_windows_amd64_test.go \
  internal/encstore/namespace_manual_darwin_test.go \
  internal/testoracle/spec.go internal/testoracle/test.go \
  internal/testoracle/spec_test.go internal/testscope/oracle_startup.go \
  internal/testscope/oracle_shell.go; do
  git -C "$SOURCE_REPO" cat-file -e "$SOURCE_COMMIT:$p" || exit 1
done
git -C "$SOURCE_REPO" grep -F -q \
  'Ordinary test and oracle startup source-reference guards PASS; behavioral review is still required.' \
  "$SOURCE_COMMIT" -- internal/testscope/main.go || exit 1
git -C "$SOURCE_REPO" grep -F -q 'inspectOracleContracts(".")' \
  "$SOURCE_COMMIT" -- internal/testscope/main.go || exit 1
umask 077
set -o pipefail
RUNROOT=$(mktemp -d "$BASE/gosqlite-namespace-XXXXXXXX") || exit 1
WORK="$RUNROOT/source"; EVIDENCE="$RUNROOT/evidence"
mkdir "$WORK" "$EVIDENCE" "$RUNROOT/bin" "$RUNROOT/tmp" "$RUNROOT/go-cache" || exit 1
git -C "$SOURCE_REPO" archive "$SOURCE_COMMIT" > "$RUNROOT/source.tar" || exit 1
tar -xf "$RUNROOT/source.tar" -C "$WORK" || exit 1
printf '%s\n' "$SOURCE_COMMIT" > "$EVIDENCE/source-commit.txt" || exit 1
printf '%s\n' "$SOURCE_TREE" > "$EVIDENCE/source-tree.txt" || exit 1
export PATH="$TOOLCHAIN_BIN:$PATH" GOPATH
export GOMODCACHE="$GOPATH/pkg/mod"
# 保留各平台原生路径写法，显式选择已验证模块缓存，不继承其他 GOMODCACHE。
for p in "$GOPATH" "$GOMODCACHE"; do
  test -d "$p" && test -r "$p" && test -w "$p" && test -x "$p" || exit 1
done
export GOTOOLCHAIN=local GOENV=off GOWORK=off GOPROXY=off GOSUMDB=off
export CGO_ENABLED=0 GOMAXPROCS=2 GOFLAGS=-buildvcs=false
unset GOOS GOARCH GOTMPDIR GOSQLITE_MANUAL_NAMESPACE GOSQLITE_MANUAL_RECOVERY
cd "$WORK" || exit 1
# 先由用户核对工具路径和主机身份；这不是交叉编译目标。
test "$(go env GOVERSION)" = go1.27.2 || exit 1
TARGET="$(go env GOHOSTOS)/$(go env GOHOSTARCH)"
case "$TARGET" in
  windows/amd64)
    TEST=TestWindowsEncryptedRelativePublicationAndRetainedRoot
    BINARY="$RUNROOT/bin/namespace-windows-amd64.test.exe"
    GOCACHE=$(cygpath -m "$RUNROOT/go-cache") || exit 1
    NATIVE_TMP=$(cygpath -m "$RUNROOT/tmp") || exit 1
    test -n "$GOCACHE" && test -n "$NATIVE_TMP" || exit 1
    export GOCACHE TMPDIR="$NATIVE_TMP" TEMP="$NATIVE_TMP" TMP="$NATIVE_TMP"
    ;;
  darwin/amd64|darwin/arm64)
    TEST=TestDarwinEncryptedRelativePublicationAndParentLifetime
    BINARY="$RUNROOT/bin/namespace-${TARGET#*/}.test"
    export GOCACHE="$RUNROOT/go-cache" TMPDIR="$RUNROOT/tmp"
    ;;
  *) printf '不属于本原生规程的主机: %s\n' "$TARGET" >&2; exit 1 ;;
esac
capture namespace-version go version || exit 1
capture namespace-env go env GOVERSION GOHOSTOS GOHOSTARCH GOOS GOARCH CGO_ENABLED GOTOOLCHAIN GOFLAGS GOPATH GOCACHE || exit 1
capture namespace-modules go mod verify || exit 1
```

用户另存 `host-and-storage.md`，说明批准的目标与范围、OS/卷/硬件、工具路径、
空间预检和实际临时目录。不要打印整份环境或秘密。`cygpath` 是 Windows Git Bash
路径转换；路径不通时先排查并保留失败，不转到 WSL 或改目标绕过。

保存可跨两平台使用的只读源码清单帮助程序，并记录源码与 archive 的身份：

```bash
# 只供未来用户在本次 EVIDENCE 下保存辅助程序，不改源码树。本次 NOT_RUN。
cat > "$EVIDENCE/source_inventory.py" <<'PYTHON'
import hashlib, json, pathlib, stat, sys
root = pathlib.Path(sys.argv[1])
rows = []
for p in sorted(root.rglob('*')):
    if p.is_file():
        b = p.read_bytes()
        rows.append([p.relative_to(root).as_posix(), len(b), hashlib.sha256(b).hexdigest(), stat.S_IMODE(p.stat().st_mode)])
print(json.dumps(rows, ensure_ascii=True, indent=2))
PYTHON
capture namespace-source-before python3 "$EVIDENCE/source_inventory.py" "$WORK" || exit 1
capture namespace-archive-hash python3 -c 'import hashlib,pathlib,sys; print(hashlib.sha256(pathlib.Path(sys.argv[1]).read_bytes()).hexdigest())' "$RUNROOT/source.tar" || exit 1
```

### 19.3 源码门核对、仅编译与原生单项执行

先人工核对 19.1 两文件的首行、各自完整函数和首句 gate，以及当前 P0 恢复
检查仍在。下面 guard 只运行源码扫描，成功文本仍是
`Ordinary test and oracle startup source-reference guards PASS; behavioral review is still required.`
**不要把 manual tag 放进 GOFLAGS**，也不在本规程运行 package 全套。

```bash
# 未来独立人工步骤。本次 NOT_RUN；每步非零立即停止。
capture namespace-source-guard go run -p=1 ./internal/testscope || exit 1
capture namespace-compile go test -p=1 -c -mod=readonly -tags manual_namespace \
  -o "$BINARY" ./internal/encstore || exit 1
capture namespace-symbols go tool nm "$BINARY" || exit 1
capture namespace-binary-before python3 -c 'import hashlib,pathlib,sys; print(hashlib.sha256(pathlib.Path(sys.argv[1]).read_bytes()).hexdigest())' "$BINARY" || exit 1
```

编译成功只记 COMPILE_PASS。人工从 `namespace-symbols.stdout` 检查完整符号
`github.com/ms-xiaosnow/gosqlite/internal/encstore.` 加本机 `$TEST`，精确函数符号
须存在，另一平台函数须不存在；`deferwrap` 符号不代替函数本体。若出现原有 Windows
unsafe-pointer vet 诊断，保存原始结果，不加 `-vet=off` 或关闭 checker 补 PASS。
这里不用 oracle tag 或官方 CLI，因为两项 primitive 回归没有 oracle 对照调用；
此前两种 tag 的外平台编译是另一条选择/链接证据。

只有用户的独立原生执行授权和上述前置都已成立，才单独执行下一块。
它只选择本机的一个精确函数，没有循环、自动重试、信号或终止进程逻辑。

```bash
# 当前代理禁止执行。仅用户在独立批准环境自行运行；sentinel 不是许可。
capture namespace-native-run env GOSQLITE_MANUAL_NAMESPACE=independently_authorized \
  "$BINARY" -test.run "^${TEST}$" -test.v -test.count=1 -test.parallel=1 -test.timeout=0 || exit 1
capture namespace-source-after python3 "$EVIDENCE/source_inventory.py" "$WORK" || exit 1
capture namespace-binary-after python3 -c 'import hashlib,pathlib,sys; print(hashlib.sha256(pathlib.Path(sys.argv[1]).read_bytes()).hexdigest())' "$BINARY" || exit 1
cmp "$EVIDENCE/namespace-source-before.stdout" "$EVIDENCE/namespace-source-after.stdout" || exit 1
cmp "$EVIDENCE/namespace-binary-before.stdout" "$EVIDENCE/namespace-binary-after.stdout" || exit 1
```

### 19.4 两项原有断言与通过条件

Windows：合成 Unicode basename `ordinary Ω family.gse`；相对目录 inventory、
lock file 创建及独占 lifecycle 锁 acquisition 成功；首次 no-replace 发布 `first` 后取得稳定
root，replacement 发布 `after`；旧 root 仍读出 `first`，新 root 读出 `after`；
新 root stable 和 namespace check 成功。源码中的 defer Close 返回值没有全部断言，
因此即使通过也不能宣称所有 close/retirement 错误路径都已验收。

Darwin：相对 inventory、no-replace 发布、replacement 与旧/新 root 字节断言同上；
显式关闭公开 parent 后，`platformDirectoryCheck` 必须匹配 `encryption.ErrClosed`，
但 retained child 的 stable 检查仍成功；显式关闭两个 root 后，受锁读取的
`ns.dir.children == 0` 且 `ns.dir.descriptor.raw == nil`。这只证明该正常顺序下
测试实际断言的 parent/child 生命周期，不是任意原生 close 失败注入。

每台机器分别判定：准确目标测试出现一次 `=== RUN` 与 `--- PASS`、零 SKIP、
无空匹配/`no tests to run`、进程自然 exit 0、没有测试 FAIL 或未解释 stderr，且
身份/符号/日志/预检齐备。仅 COMPILE_PASS、t.Skip 后 exit 0 或 shell 记录成功都
不能算 NATIVE_CASE_PASS。三种 OS/arch 分别保留结果，不互相替代；任何单项
通过都不等于全原生平台、加密安全、故障恢复、M3/M8/M10 或整个 goal 验收。

### 19.5 排查、证据与清理限制

- gate SKIP：检查精确 env 名和值、所选新提交及标签；没有独立授权就停止，
  不能为去掉 SKIP 自动设置门。空匹配/缺符号：检查平台/架构/包/完整测试名。
- guard 拒绝：核对 manual GOFLAGS、实际 body/helper，保留拒绝记录；不改名、
  添加豁免或删掉 guard。原生错误、hang、身份变化、资源不明：停止后续启动，
  保存原始错误与当前状态，不重试、强杀、修改权限/挂载或清库求 PASS。
- 保存 source commit/tree/archive SHA、源码前后清单、binary SHA/nm 输出、
  实际 tag/env gate（不含任何密钥）、每步 argv/UTC 起止/stdout/stderr/exit、
  原生主机与卷、所有断言/skip/未覆盖范围。结果表将 SOURCE/MODEL/COMPILE/
  NATIVE_CASE 分列；文档本轮为静态核对，native 一律 NOT_RUN。
- 两测试沿用原 `t.TempDir()`，会在测试结束（包括常见断言失败）自动清理 fixture；
  外部 capture 留住日志不等于完整失败目录仍在。本切片没有新增失败留存 harness，
  不能悄悄改 body 或宣称失败现场保存已实现。
- 只有进程自然结束、已知资源正常收尾且日志/身份归档核对完成后，用户才决定
  是否删除自己本次新建的 RUNROOT。失败/不确定时不额外删除残留，不清原仓库、
  活库 sidecar、历史证据或其他任务缓存；不提供宽泛删除命令。不把删除当安全擦除。


## 20 公开 Open 四场景 harness 源码与编译资格（2026-10-11）

本节补充第18节的历史状态，保留第12节内部 harness、第19节 namespace
规程及全部既有命令。新测试保持独立 V3 审阅原字节；专题手册仅更新
发布状态段，命令/断言不改，V3 审阅手册原字节另存证据。
该 V3 基于 P0 提交 `56bbcf3febe233197aa0a5eac965c67a67a88011`，
候选 tree `1755d67431f9de082449b08fa24b26b2b35eb612`。
本次组合基点为已保存 namespace 提交
`50b627e11e9dbce7f2fde8df35c4634c52a06d52` / tree
`8f2709293287a3517a52b3f989888ded8230ef53`，不整树覆盖 namespace 成果。

### 20.1 已有入口与严格有限覆盖

[`TestManualPublicRollbackRecovery`](../sqlite/rollback_recovery_manual_linux_test.go)
位于单一新增 Linux 手工文件，实际通过公开 `sqlite.OpenContext` 和
`Options.RollbackRecovery`；没有直接调用内部恢复函数或改变生产行为。
首行必须为 `//go:build manual_recovery && linux`，函数首语句必须先核对
`GOSQLITE_MANUAL_RECOVERY=independently_authorized`，缺值即 Fatal。
它先拒绝误启动，再检查全新、空、绝对且无符号链接的 fixture 根目录。

| 真实子测试名 | 已实现的断言 | 当前执行 |
|---|---|---|
| `nil-default-preserves-input` | nil DB、ErrRecoveryRequired、CodeIOErr，工作 DB/journal 字节保持 | BLOCKED / NOT_RUN |
| `opt-in-restores-and-reopens` | 恢复到原两页、元数据/空 schema/完整字节、journal 消失；一次正常关闭后只读 NoCreate 重开 | BLOCKED / NOT_RUN |
| `journal-limit-preserves-input` | MaxJournalBytes=1，nil DB，ErrRecoveryRequired 与 ErrMemoryLimit/CodeIOErr，输入保持 | BLOCKED / NOT_RUN |
| `no-create-preserves-absence` | nil DB、os.ErrNotExist，主库/journal/WAL/SHM 与隔离目录其他条目均不创建 | BLOCKED / NOT_RUN |

恰这四项没有覆盖 Connector 真实恢复、更广 journal 形式、其他平台、native
WAL-tail publisher、短写/Sync/ENOSPC、断电/强杀/torn-write、恶意目录替换
或完整加密故障矩阵。该有限入口不再是 IMPLEMENTATION_MISSING；
其余缺失设施和原执行限制保持。

### 20.2 实际通过的是审阅与编译

- [V1 原报告](evidence/public-recovery-harness-20261011/v1-source-review.md)
  的三项阻断保留。[V2](evidence/public-recovery-harness-20261011/v2-source-review.md)
  修正 fail-open 检查、所有备份及 manifest 核对、失败即停/Close/pending 处理；
  [V3](evidence/public-recovery-harness-20261011/v3-compiler-review.md)只移动
  手册环境设置到首个 Go 命令之前，并将 archive 与提取拆开，Go 源码未再改动。
- 六项获准 Go 工具调用为外置标准库 AST 文本检查器、default/oracle/manual
  三次 go list、只编译的 go test -c、只读符号的 go tool nm；08:58:38 UTC
  自然完成，全部 exit 0。独立文本检查要求文件次数恰 0/0/1，具名顶层符号唯一。
- 没有执行 test binary、gate-off、-test.list、无匹配 selector、fixture
  生成器或任何物理测试。本次文档对账本身也没有调用 Go 或运行数据库。
- 源码 SHA-256 为
  `dd5a63e2a8e7d4fdd969ba03273a8f8147d5eff81babf8fce44a5228c5cdaf5e`；
  原 V3 审阅手册 SHA-256 为
  `a49ae1882c4a5dcb8c58860fa8df3b011f88c9ee99963a5721831049477ec4d2`。
  候选、当前组合及后来提交须分别记录，不能互借 PASS。

### 20.3 人工步骤、证据留存与停止

完整步骤见[独立中文规程](public-rollback-manual-zh.md)：环境/资源与独立授权、
固定干净提交的隔离副本、fail-closed 选择计数、compile/nm、精确单项选择、
预期断言、排查及整个 RUN 目录留存。该手册当前发布状态段与
[原 V3 审阅手册](evidence/public-recovery-harness-20261011/v3-reviewed-manual.md.txt)
分开记录，历史 V1/V2/V3 身份保留；资格以本节、[当前验收](acceptance-status.md)与 V3 报告为准。
它的执行命令仍仅供用户在另行批准环境自行使用，不能由代理执行或迁移 CI 绕行。

发生失败即停止后续场景；Close 失败不得继续重开，pending-owner 只记录，
不隐式重试恢复或清理。所有已构造输入备份、manifest、工作文件和目录成功后
仍保留；核对证据归档和资源状态后，才由操作者决定清理自己创建的目录。
四个具名子项全部实际运行且通过、零 FAIL/SKIP、身份/日志/退出码齐备，
才可对该精确提交及实际 Linux/架构/文件系统记四场景 PASS；编译、空匹配或
部分 PASS 均不符合。当前没有这份物理运行证据，M3/M10/整体仍未验收。

## 21 startup / WAL / LIMIT / 示例的新证据与后续手工入口

本次没有运行本手册任一命令或创建手工 fixture。前面历史章节及日志保留原时点；
第 4–7 节的五处直接 SQLite 未来模板已经改用统一清洁子进程，普通命令保持
SQLITE_ORACLE 未导出，不能误把可选 oracle 加入普通范围。新 helper 仅支持 Linux
启动；Windows/Darwin 的原生 oracle 合同仍未实现，跨编译不能补齐这项资格。

逐项状态与旧 b349 持久闭合见[证据摘要](evidence/startup-wal-limit-20261011.md)。
启动纯测试、完整脚本、精确 LIMIT 和 Example 未来选择器见
[中文专题](oracle-startup-verification-zh.md)；只有真实存在的测试/参数才列为命令。
四场景 public recovery 保持第 20 节的 BLOCKED / NOT_RUN；native WAL-tail
publisher 和其他缺失 harness 没有可编造的运行命令。限制分类的更新不允许恢复
ANN V4、受阻 41-target campaign 或未知中断链路。

## 22 Windows shared-rekey 原生普通路径独立规程

### 22.1 当前源码和执行边界

**本节全部命令 NOT_RUN。** 仅供用户以后在独立批准的原生 Windows 环境逐项
采用；不是当前代理、其他执行器或 CI 的执行许可，也不能作为受阻链路的替代。
先读 [Windows 协议与精确源码](windows-shared-rekey.md)。旧第 3.2 节固定 776b
表与第 9.2 节的“不支持”保留历史意义；b0e 九路径源码现已实现 guarded
admission/conversion，尚未发布，不能凭旧 main 或旧编译二进制执行本规程。

当前实际存在的三个测试入口：

| 命令代号 | 实际入口 | 可证明的有限内容 |
|---|---|---|
| capability | [TestWindowsEncryptedSharedRekeyCapability](../internal/encstore/native_windows_test.go) | 原生版本检查允许 shared-rekey target；不打开 DB，不证明 NTFS/锁/发布 |
| latest | [TestEncryptionSharedRekeyLatestHeadOrdinary](../sqlite/encryption_shared_rekey_test.go) | intact/correct-key、其他 owner 正常退出后，rekey 采用另一连接最新 head 并用新正确 key 重开 |
| quiescent | [TestEncryptionSharedQuiescentRekeyAndLeaseRetention](../sqlite/encryption_shared_test.go) | idle/read-only owner 阻塞 rekey；正常退出后原 owner 的 lease 保留、rekey 成功、新正确 key 重开 |

三个测试均没有 manual tag/env gate；必须精确选择，不能因此启动整个 Windows
包或默认全仓。`native_windows_test.go` 的 `windows && amd64` 才是 capability
的平台选择。旧 `TestWindowsEncryptedSharedRekeyPreflightMemory` 已被替换，
不能使用旧名字得到零匹配 exit 0 后报通过。namespace 测试另在 manual 文件，
保留第 19 节双门，本节绝不加 `manual_namespace`、`manual_recovery` 或 oracle。

这些现有普通测试没有原生旧/新二进制调度、强制切换点、真实 lock/Close/Sync
失败或中断 rekey harness。故此规程即使全部 PASS，也只取得三个普通路径的
目标机结果，不能称整个 Windows rekey 通过、mixed-version 安全或故障设施已实现。

### 22.2 工具、隔离副本和可核查的前置

- 原生 Windows/amd64，Windows 10 1709（build 16299）或更高；记录准确 OS
  edition/build、CPU、是否虚拟化、固定本地 NTFS 卷、剩余空间、权限和实际路径。
  WSL 的 Linux 结果、Wine/模拟执行、UNC、网络盘、reparse/cloud/offline 目录
  不能替代。NTFS 本身不证明电源故障安全；Windows Directory Sync 没有承诺的
  namespace 持久性 barrier。
- 已验证原生 Go **1.27.2**、Git Bash、Git、tar、Python 3。不开安装/下载流程。
  模块使用已验证离线缓存：x/crypto v0.58.0、x/sys v0.49.0、x/text v0.43.0。
  项目行为基线仍是官方 SQLite **3.54.0**，但本节没有 SQLite child 或 oracle
  对照；不要求运行 Windows SQLite，不能把 Linux-only startup helper 强行改目标。
- 单独的已审阅干净提交，须包含专题所列九文件精确 SHA-256。操作者填其完整
  commit 和 tree，不能用动态 HEAD、b0e tree 当 commit 或旧 67318ded 代替。
  以后若任一源码不同，先重新审阅/定界，不能直接改下面哈希求 PASS。
- 只用新的可丢弃目录、合成数据及测试自生临时 key，不用真实秘密、原库或唯一
  备份。不打印整份环境，不开 `set -x`。本节私有 temp/cache 是未来用户新环境
  的模板，不变更当前已有复验的 /tmp、缓存和资源门。
- 下列脚本逐块人工审核，空间不足、路径/版本/哈希错误即停。用户先核准本次
  空间上限和资源条件，至少为 source/cache/output 与独立 temp 各预留 2 GiB；
  该下限只是启动条件，不是内存或磁盘最大消耗保证。不因不足自动清理/搬迁。

```bash
# 未来独立用户模板。当前 NOT_RUN；路径先由操作者填入并核验。
SOURCE_REPO=/c/reviewed/gosqlite
BASE=/c/disposable-native-acceptance
TOOLCHAIN_BIN=/c/tools/go1.27.2/bin
GO_BIN="$TOOLCHAIN_BIN/go.exe"
SYSTEMROOT_NATIVE=C:/Windows
SYSTEM32_BASH=/c/Windows/System32
GOPATH_NATIVE=C:/verified-go-cache
GOMODCACHE_NATIVE=C:/verified-go-cache/pkg/mod
: "${SOURCE_COMMIT:?填写包含九文件修正的已审阅完整提交SHA}"
: "${SOURCE_TREE:?填写该提交准确tree}"
test -d "$BASE" && test -d "$SOURCE_REPO/.git" && test -x "$GO_BIN" || exit 1
test -d "$SYSTEM32_BASH" || exit 1
test -d "$(cygpath -u "$GOPATH_NATIVE")" || exit 1
test -d "$(cygpath -u "$GOMODCACHE_NATIVE")" || exit 1
test -z "$(git -C "$SOURCE_REPO" status --porcelain)" || exit 1
test "$(git -C "$SOURCE_REPO" rev-parse "$SOURCE_COMMIT^{commit}")" = "$SOURCE_COMMIT" || exit 1
test "$(git -C "$SOURCE_REPO" rev-parse "$SOURCE_COMMIT^{tree}")" = "$SOURCE_TREE" || exit 1
umask 077
RUNROOT=$(mktemp -d "$BASE/gosqlite-windows-rekey-XXXXXXXX") || exit 1
WORK="$RUNROOT/source"; EVIDENCE="$RUNROOT/evidence"
mkdir "$WORK" "$EVIDENCE" "$RUNROOT/tmp" "$RUNROOT/go-cache" || exit 1
git -C "$SOURCE_REPO" archive "$SOURCE_COMMIT" > "$RUNROOT/source.tar" || exit 1
tar -xf "$RUNROOT/source.tar" -C "$WORK" || exit 1
printf '%s\n' "$SOURCE_COMMIT" > "$EVIDENCE/source-commit.txt" || exit 1
printf '%s\n' "$SOURCE_TREE" > "$EVIDENCE/source-tree.txt" || exit 1
NATIVE_TMP=$(cygpath -m "$RUNROOT/tmp") || exit 1
NATIVE_CACHE=$(cygpath -m "$RUNROOT/go-cache") || exit 1
cd "$WORK" || exit 1
```

只读 source archive 哈希及完整路径/大小/内容/实际 mode 清单采用第 19.2 节的
`source_inventory.py` 原文，存入本次 EVIDENCE，先人工审阅再调用。Git archive
和 Windows 权限不承诺保留原 POSIX 0600/0700；本机实际 mode/ACL 单独记录，
不得称 Git tree 已证明它们。之后所有前后清单必须在同一隔离副本上相等。

下面为独立日志函数及严格限定的 Go 子进程环境。`env -i` 不传入 HOME、用户
GOFLAGS、SQLITE_ORACLE、SQLITE_DEBUG_BREAK 或 manual gates。SystemRoot/WINDIR、
工具和系统 PATH 是操作者已核验的明确值，仍不是 OS 沙箱/不可信 loader 防御。
Git Bash/cygpath 路径转换须先验证；失败时停止，不转 WSL 或改变 target 绕过。

```bash
# 未来独立用户模板。本次 NOT_RUN。不覆盖已有日志。
capture_rekey() {
  local name=$1 rc
  shift
  test ! -e "$EVIDENCE/$name.stdout" || return 1
  printf '%q ' "$@" > "$EVIDENCE/$name.command" || return 1
  printf '\n' >> "$EVIDENCE/$name.command" || return 1
  date -u +%Y-%m-%dT%H:%M:%SZ > "$EVIDENCE/$name.start-utc" || return 1
  "$@" > "$EVIDENCE/$name.stdout" 2> "$EVIDENCE/$name.stderr"
  rc=$?
  printf '%s\n' "$rc" > "$EVIDENCE/$name.exit" || return 1
  date -u +%Y-%m-%dT%H:%M:%SZ > "$EVIDENCE/$name.end-utc" || return 1
  return "$rc"
}
native_go() {
  env -i SystemRoot="$SYSTEMROOT_NATIVE" WINDIR="$SYSTEMROOT_NATIVE" \
    PATH="$TOOLCHAIN_BIN:/usr/bin:/bin:$SYSTEM32_BASH" \
    GOPATH="$GOPATH_NATIVE" GOMODCACHE="$GOMODCACHE_NATIVE" GOCACHE="$NATIVE_CACHE" \
    TMPDIR="$NATIVE_TMP" TMP="$NATIVE_TMP" TEMP="$NATIVE_TMP" \
    GOENV=off GOWORK=off GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GO111MODULE=on \
    CGO_ENABLED=0 GOOS=windows GOARCH=amd64 GOAMD64=v1 GOMAXPROCS=2 \
    GOFLAGS= GOEXPERIMENT= GODEBUG= GORACE= GOCACHEPROG= GOCOVERDIR= GOFIPS140=off \
    LC_ALL=C LANG=C TZ=UTC "$GO_BIN" "$@"
}
capture_rekey version native_go version || exit 1
capture_rekey host native_go env GOVERSION GOHOSTOS GOHOSTARCH GOOS GOARCH CGO_ENABLED || exit 1
# 人工核对依次为 go1.27.2、windows、amd64、windows、amd64、0。
# 核对完成才继续；只看到 GOOS=windows 不证明实际主机为 Windows。
capture_rekey modules native_go mod verify || exit 1
capture_rekey source-before python3 "$EVIDENCE/source_inventory.py" "$WORK" || exit 1
capture_rekey archive-sha256 python3 -c 'import hashlib,pathlib,sys; print(hashlib.sha256(pathlib.Path(sys.argv[1]).read_bytes()).hexdigest())' "$RUNROOT/source.tar" || exit 1
capture_rekey go-sha256 python3 -c 'import hashlib,pathlib,sys; print(hashlib.sha256(pathlib.Path(sys.argv[1]).read_bytes()).hexdigest())' "$GO_BIN" || exit 1
```

保存以上函数原文、明确环境白名单和路径到 `procedure.txt`，与日志同行保存，
因为 command 文件中的 `native_go` 函数名本身不等于完整进程环境。人工把 source
清单中九文件哈希逐项对照专题表，同时核对完整审阅提交的其他源码；九文件相等
不是全仓源码审核替代。保存 `host-and-storage.md`、工具哈希、依赖核验、空间
观测、source archive 和前清单。任何项目不满足就记 BLOCKED，不启动测试。

### 22.3 三个精确命令及断言

以下只在以上独立授权、完整身份、真实主机与资源前置全部满足后逐项执行。
`-vet=off` 明确只限定这三个测试命令，不算 Windows clean-vet 资格；既有
VFS unsafe-pointer cross-vet 历史诊断不被抹去。不是 race 或完整包验收。
使用 Go 的内部 timeout，进程自然退出；不得另加外部 kill、重试或宽 selector。

```bash
# 本次 NOT_RUN。每步非零即停；不得换名跳过失败。
capture_rekey capability native_go test -mod=readonly -buildvcs=false -p=1 \
  -count=1 -json -vet=off -timeout=2m \
  -run '^TestWindowsEncryptedSharedRekeyCapability$' ./internal/encstore || exit 1
capture_rekey latest native_go test -mod=readonly -buildvcs=false -p=1 \
  -count=1 -json -vet=off -timeout=5m \
  -run '^TestEncryptionSharedRekeyLatestHeadOrdinary$' ./sqlite || exit 1
capture_rekey quiescent native_go test -mod=readonly -buildvcs=false -p=1 \
  -count=1 -json -vet=off -timeout=5m \
  -run '^TestEncryptionSharedQuiescentRekeyAndLeaseRetention$' ./sqlite || exit 1
capture_rekey source-after python3 "$EVIDENCE/source_inventory.py" "$WORK" || exit 1
cmp "$EVIDENCE/source-before.stdout" "$EVIDENCE/source-after.stdout" || exit 1
```

执行每条后先审阅完整 JSON，再启动下一条；每条必须恰有表中对应唯一顶层 RUN
和终态 PASS、零子项、一个正确包 PASS、exit 0、空 stderr，无 FAIL/SKIP、额外
名称、重复/缺失/截断事件或 `(cached)`。零匹配即使 exit 0 也 FAIL。capability
只能证明原生版本检查；不能用它跳过 latest/quiescent 或推断卷与锁已验收。

- latest：A 先持旧 head，B 正常写入 `latest` 和 `also copied` 后关闭；A 自己
  rekey 必须采用最新值。DatabaseID 保持，layout=3、mode=shared、segments=0，
  sequence 增加、head 改变；新正确 key 只读重开后值和完整 VerifyEncrypted 成功。
- quiescent：idle/read-only first B 使 A rekey 得 BUSY；B 正常关闭后，同一正确
  key 的 legacy WAL 打开仍 BUSY，证明 A lease 未丢；A 保留 `retained`，随后
  rekey/Close 成功，新正确 key 只读重开保留该值且 mode=shared、segments=0。
- 两个 SQLite 测试的各 DB Close attempt 都先置标志、检查结果，不在 cleanup
  中自动重试失败的同一 DB Close；这不证明 native Close 计数、所有资源终结或
  物理 Close 失败。公共 query helper 的 deferred Rows.Close 未查返回仍是
  已知继承限制，不能扩大为完整清理验收。

### 22.4 失败排查、证据和清理

任何非零、未知 retirement、缺名/多名、SKIP、身份改变或前置不满足，停止后续
命令。记录具体包/测试/错误链和最后成功步骤，不自动重复 Open/Rekey/Close，
不删除候选、切 mode/文件系统、换环境或改 assertion 求 PASS。

- `ErrUnsupported`：检查真实 OS/build/架构、卷是否固定 NTFS、路径政策、九源码
  哈希及 selector。capability 通过而实际 Open 拒绝仍算后者失败，不能放宽路径。
- `ErrBusy`：可来自普通 owner 争用；若还带 poison/uncertain，则不可视为安全
  BUSY。停止并保留错误；不要自动循环到成功或假定写入未发生。
- 认证/authority/`ErrChanged`：保留 source 身份、日志和可取得的目录现场，停止。
  不主动加入旧 key、tamper、namespace 替换或恢复操作来“确认原因”。
- 编译/依赖/资源错误：保留日志，先处理真实前置；不下载替代工具链、不换缓存
  或关闭更多 checker。新的修复/重跑属于新审阅和新证据。

测试使用 `t.TempDir`，Go cleanup 可能自动移除 fixture，包括测试失败后的目录。
因此保留 RUNROOT/log/source archive **不保证保留测试失败数据库**；测试自生密钥
也没有恢复承诺。专门的失败现场保留、混合版本调度和物理故障 harness 仍缺独立
实现/审阅，不在文档中虚构开关。

若验收要求保留失败数据库或事后排查未知 retirement，现有三个普通测试不能
承担该验收：明确记 **BLOCKED / IMPLEMENTATION_MISSING**，停止本节原生命令。
下一步是单独实现并独立审阅 failure-preserving harness，最低合同包括：

- 调用方拥有的唯一可丢弃 fixture root，失败或 pending/unknown retirement 时
  不自动删除，不能借成功返回假定所有 owner 已终结；
- 数据库家族前后 manifest、操作/错误及 owner-state 证据，出错后只收集无需
  重启项目代码即可安全取得的资料；
- 临时合成 key 的明确保管/销毁政策与后续重开限制，不承诺仅凭文件可恢复；
- consuming Close 只做 checked-once attempt，未知结果立即停止，不重试可能已
  被消耗/复用的 numeric native handle；
- harness 源码与任何新增 fault、mixed-old-binary、interruption 或其他已排除
  场景分别审阅、分别明确授权，不能由本节普通路径或文档取得许可。

这是缺失设施的工程下一步，不是一个现有 test/flag/env；本次未实现它。
若独立批准范围仅需要三个有限普通路径，且不依赖失败 DB 留存，可按前述边界
单独规划；不得借这个较窄结果满足留存依赖型验收。

成功或失败后均先保留整个 RUNROOT，补后清单
和故障诊断；某步失败时只做不启动项目代码的证据采集，不接着执行下一测试。

最终归档包含完整 source archive/身份、工具/环境白名单、主机和卷证据、每条
argv/start/end/exit/stdout/stderr、JSON 逐名核对及前后清单，计算 SHA-256 并读回
比对。用户确认副本和证据可用、无遗留 owner 待处理后，才人工决定清理自己新建
的目录；本规程不给自动 `rm`、强制终止或未知 handle 重试命令。

只有上述三个名称全部实际运行并通过、身份/环境/原始证据完整且前后 source
相同，才可写“某精确提交在某原生 Windows/build/NTFS 上的三个普通路径 PASS”。
不得写 mixed-old-binary、native lock/close failure、power loss、中断 rekey、安全
擦除或完整 M3/M8/M10 PASS。当前这些普通原生结果仍 **NOT_RUN**；Windows
namespace barrier 缺失、旧 ciphertext 可解密、native WAL-tail publisher 代码缺口、
独立 backup R1 未发布、ANN V4/受阻 41-target campaign/未知中断限制全部保留。


## 23 Darwin 受限 Unicode 同级清单的当前源码边界

2026-10-11：第 3.2 节的 776b 表及旧 V3 结果保留历史身份。当前已发布基点为
`3ea6d3ab15f678fe48bee312921da60828845869`；新十路径源码候选 tree
`88c400a4caeb40d8d96fae7700e7048d69db4ff5` 尚未发布。独立源码审阅已满足
限定要求，详见[受限清单证明与身份](darwin-unicode-inventory.md)。

根名仍须是最长 177 字节的可打印 ASCII，预留 78 字节生成后缀；Unicode 父路径
原已允许。只有有效 UTF-8 同级名称在根名内部、Unicode/控制字节前已有折叠
ASCII 差异，且实际 managed 枚举 owner 的 Fstatfs 确认本地 APFS 或 hfs subtype
0–3，才可不作为成员。扫描配额在分类前扣计，选中成员所有权及六类读取诊断
在前的 join 保持；只有字面 EOF 清洁终结，取消/清理约束不变。

V1 文档的 18 个内存函数加 2 个源码/AST 函数 normal/race NOT_RUN 保留历史
身份；固定源码的两条 Linux/amd64 精确 20 项现已各一次 PASS，零子项/SKIP/FAIL，
各一个 package PASS，stderr 为空。额外编译/源码门仍 NOT_RUN，AST 只是结构
回归辅助。初始未经核实的 system Go/新 cache 提案历史 HOLD 且未执行；固定
工具修正版 V2 经主任务批准后只执行这两条命令一次，不扩展执行许可。原生 Darwin
另为 NOT_RUN；现有传 nil 目录的分类测试只覆盖 ASCII，真实 managed-directory
加 Unicode-sibling 资格 harness 为 IMPLEMENTATION_MISSING，而非只缺原生机器。
没有为此发明选择器或新增运行命令。一般 Unicode 根名、M3/M8/M10、既有物理
限制和所有手工 tag/env 门保持，历史 baseline FAIL、V1/V2 HOLD 与 V3 旧树结果
不转为当前 PASS。
