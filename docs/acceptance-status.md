# 当前实现与验收矩阵

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

分类更新：Darwin 有界名称/ReadDir 诊断代码取得上述限定普通与编译资格；原生平台仍 NOT_RUN。真实只读 WAL、通用备份/更多 journal、完整物理 harness、支持域 SQL 差异以及受阻 ANN V4 等代码义务仍开放。整体继续 M9 范围实现，M3/M8/M10 未完整验收。

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

当前后续状态（2026-10-11）：有限公开恢复手工 harness 源码及中文步骤已补齐，
独立源码审阅和六项 AST/选择/编译/符号检查 PASS，四个物理场景全部
**BLOCKED / NOT_RUN**。旧 P0 段落中的整个 public harness
`IMPLEMENTATION_MISSING` 是当时状态；当前精确范围和组合/持久证据分层见
[本次补记](#2026-10-11-有限公开恢复-harness-源码与编译资格)。

核对时间：2026-10-11；以下历史开发基点为 `63c2ebb10e3a2c4ff2378ee9f33c46133134c70f`，tree `d735409ebd1c43d90ee1650c8a7c2a084e270674`。该历史检查点的恢复与 CI 已闭合，详见文末；本轮新源码、提交、恢复及 CI 仍须分别核验，不预写自指提交。旧 `7a9972547f49531df04cf1b624dfa662b483907b` 与 `1352ae524575e4d6fe8b5434fd930c3b07be4ac8` 保留为历史基点。
63c2 历史检查点叠加三项有限收尾：四项 CLI/示例资源修正、三份必需入口文档对账、五例目录终结与输出错误修正。四项 CLI 固定树完整普通门、三份文档独立静态审阅、五例独立源码和限定普通/race 审阅分别 PASS；最终组合与新提交持久门见文末，不能沿用基点结果。四项 CLI 的精确结果见[组合证据](evidence/cli-resources-20261011.md)，五例的操作边界见[留存限定复验](example-retention-verification-zh.md)。
已有 inspect 精确源码和边界见[元数据证据](evidence/inspect-metadata-20261011.md)和[上下文证据](evidence/inspect-context-20261011.md)。
这是开发检查点清单，不是最终交付。原始 [goal.md](../goal.md) 不变。
平台受限项目可以暂停执行并记录用户自测步骤，不能取消真实实现义务。

06:53 UTC 用户要求跳过并记录受限测试，其余继续分批编码、提交并报告。已将过宽的源码 HOLD 分类纠正为[逐项限制溯源](restriction-provenance-20261011.md)：可访问证据中未定位到 recovery/backup/WAL/planner/backend/harness 的精确源码编辑拒绝，不能据此断言它们从未被拒绝或一揽子解限。实际受限执行和 ANN V4、41目标 fuzz、未知中断的禁重试保持。当前 LIMIT/OFFSET 数值转换与 ErrMismatch 的三文件候选已取得限定普通/race与独立审阅PASS；精确组合普通18阶段已于07:39:21 UTC全部PASS；提交及恢复/CI仍待本批精确身份闭合。具体原始失败、包装器纠正及范围见[新切片证据](evidence/limit-conversion-20261011.md)。

## 2026-10-11 P0 public Open 源码更新

本批在已保存 `c1502790c4b233757b0f9a3c4d5e5bdee1321310`（tree
`5a98da017584fe708e43bce9f9e52dca1f06bd20`）上组合公开恢复源码，保留
LIMIT 修正、限制分类及既有进展。上文关于 LIMIT 提交待办是该批写作时点；
此处不推定 c150 的恢复或 CI 结果，也不让其普通门替代本批新组合门。

- 源码已实现：`Options.RollbackRecovery` 的 nil-default 有限 Linux 可写明文
  rollback opt-in、reader 退休/path proof、一次恢复和 fresh no-create probe、
  Connector 配置值复制及防误执行 guard；仍属实验性 source/model 范围。
- 独立候选 tree `75c074548e8b5208beb06e6ead314d835cd16f4f` 的源码审阅、
  11 项精确 descriptor-free 测试（CGO0 与相同 race 选择）、guard、限定 vet、
  全包 CGO0 build / 无 CgoFiles 闭包 PASS；独立最终阶段于 07:49:35 UTC 完成。
- 当前文档对账时，本批 c150 加 P0 的完整普通/oracle/race 组合、新提交恢复及
  新提交 CI 尚待各自终态证据。旧63c2、LIMIT和上述候选证据不自动覆盖新身份。
- 真实 RecoverRollback、production bridge、开启恢复的真实 Open/Connect
  均 `BLOCKED / NOT_RUN`；public physical harness 为 `IMPLEMENTATION_MISSING`。
  现有内部 manual harness 不覆盖该公开桥接；它的原命令及限制保持。

API、预算、诊断兼容边界与精确 source/model 命令见
[专题文档](rollback-recovery-open.md)；详细人工前置见
[中文手册第18节](manual-verification-zh.md)。默认拒绝、ErrRecoveryRequired
匹配与 CodeIOErr 保持，但诊断文本和私有包装有变化，不承诺错误字节相同。

## 状态定义与证据

- `PASS（限定范围）`：有精确源码、实际命令与结果；仅覆盖记录的范围。
- `PARTIAL / 未实现`：必需代码未完成、未合入或仅支持部分配置；文档不能替代实现。
- `FAIL`：已有真实失败；后续有限通过不能抹去历史失败。`HOLD` 必须注明是精确动作拒绝、执行限制、开发暂停还是工程审阅/集成待办，不能单凭这个标签声称源码禁止编辑。
- `NOT_RUN`：没有执行证据。`平台阻塞` 还表示禁止在本环境重试或换路执行。
- `需真实外部环境`：缺少实际目标 OS、持久性硬件、独立审计或授权服务证据。

[2026-10-10 原规范审计](evidence/goal-acceptance-audit-20261010.md)
保留自己的较早源码和时间身份。当前摘要以下表为准；
[checkpoints.json](checkpoints.json)、[progress.md](progress.md) 与
[中断记录](interruption-20261010.md) 保留具体历史证据。

## 一、必须实现：仍有真实代码缺口

| 原规范条款 | main 已有能力 | 剩余代码 / 状态 |
|---|---|---|
| G1、§3、§4.1、M3 | 有界多段解析及 internal 恢复所有权；新增默认关闭的 Linux 可写明文 Open/Connector 恢复源码、准确 path 的退休 proof 和一次 fresh no-create probe，源码/模型独立 PASS | `PARTIAL / experimental`：更多 journal 形式、默认开启策略、其他原生平台与 native exclusive WAL 尾部安全发布仍未完成；四场景 public manual harness 已实现并独立 source/compiler-only PASS；driver Connector 物理 harness 与更广覆盖仍缺，实际恢复/开启选项的真实 Open/Connect 均 BLOCKED / NOT_RUN |
| G1、§4.2、§11.1/11.3 | 索引范围、有限 JOIN/GROUP BY/HAVING、连接计数和 cached preparation 的限定普通门通过 | `已知差异`：支持查询的 planner 错误/排序/阶段一致性仍待修正。V4c 树 `552286a…` 有独立有限 PASS，但没有和 main 新 grouped/counter 组合；不能替代 main 结果。新 LIMIT 转换/ErrMismatch 三文件修正独立限定PASS，组合完整普通18阶段PASS；不覆盖其余绑定/执行阶段缺口 |
| §4.2、§8.1、§12.2 | correct-key 加密 backup/rekey CLI、native DB/Tx/CLI Check V2 已合入 | `未实现/未合入`：main 明文 backup 命令仍明确 unsupported；Linux rollback V2 候选终审未完成，WAL/非 Linux backup 和真正只读 plaintext-WAL CLI 缺代码。native VACUUM 未实现 |
| G1、§3.6、§12.2 | query/check/inspect CLI 均使用 OpenContext；DB.Inspect 和 inspect --extensions 在单快照呈现持久 method/version/config/generation，默认 JSON 保持 | `PARTIAL`：本次实测只涵盖 Linux plaintext rollback、grams/custom metadata；加密 inspect CLI 的密钥/profile 路由、真正只读 WAL 及其余已选 CLI 目标仍有实现缺口。持久 metadata 不证明索引可用或正确，中途 I/O 取消未动态验证 |
| §12.1、§14 | 四项 CLI/示例资源修正与五例目录终结/输出错误修正已完成，各自取得限定独立审阅与普通证据 | `限定修正 PASS，完整验收未完成`：sparse 状态、readonly 显式只读、write/query 关闭错误及五例目录终结已有证据。实际失败进程/原生关闭失败/真实失败目录保留未执行；WAL 原 t.TempDir 清理和清除进程密钥不允许推断失败现场完整或可解密 |
| §9.2(2/5) | caller-dataset 范数、完整精确距离与 Top-K v2 已合入；高维 v1 失败保留，v2 有有理数参考验证 | `PARTIAL`：ANN caller-dataset recall 及真实 PG/Neon 验证单列；精确距离不能替代 ANN |
| §5.2、§7.2、§11.3 | NFC 组合语料、真实 grams build/空间/P50/P95 工具和普通事务 V4 已合入 | `FAIL/待集成/部署HOLD分列`：ANN 报告 V3 三类实质准入缺陷未修，V4 链路禁重试。性能报告 V3 限定源码审阅通过但未合入；自动性能 CI 部署另受操作许可限制。没有可采信测量不得宣称完整基准通过 |
| §8、§11.3、§12.3 | Linux/Darwin/Windows 有受支持 backend 源码，只有 Linux 普通运行证据 | `PARTIAL`：main Windows shared rekey 及 Darwin 名称配置受限；修正版在未发布 `a314028…` 组合。Darwin 部分由下述当前补记接续：受限 Unicode sibling/诊断修正已获限定源码审阅，尚未发布；固定源码的 Linux/amd64 精确 20 项模型/AST normal/race PASS；额外编译门/原生验证仍 NOT_RUN，真实目录 Unicode-sibling harness 为 IMPLEMENTATION_MISSING。更广 Unicode root 支持未实现，但不能虚构为规范要求任意无限名字。保留目录 VFS 未冻结/整合 |
| G1、§4.2 | 明确的 phased SQL/格式矩阵 | UNIQUE/CHECK/FK、CTE/UPSERT/view/trigger/window/JSON/FTS5/ATTACH 等和 UTF-16/WITHOUT ROWID/autovacuum 未实现；保留分期状态，优先现有支持域正确性 |
| §11.1、M3/M8/M10 | 有限手工 replay/namespace 测试源已存在 | 完整故障点、断电、短写/Sync、ENOSPC、错位/torn write、checkpoint/rekey 中断的 harness 仍有源码缺口；不能全部称为已有命令只待运行 |
| §11.3、§12.3、M10 | 来源、许可、威胁模型、格式说明和示例已有 | 稳定 API、完整格式升级/回滚矩阵、独立数据安全/密码学/法律审查与稳定版判断均未闭合 |

## 二、可自动验证：限明确独立且经过选择审阅的普通范围

| 原规范条款 | 已有证据 | 允许的普通范围与限制 |
|---|---|---|
| §0、§11.1/11.2 | 精确 main `776b23e…` 的 CI `38091940645` 于 10 日 22:52:30 SUCCESS；普通与完整 pinned oracle 两项均成功 | 独立修改可用现有 Linux 普通单元/vet/no-CGO build、相关 race、普通 oracle；固定源码并记录选择器。不得移到未审阅 OS/标签 |
| G1、M0–M2、§11.1 | 受支持明文格式的普通读写、合法事务/保存点、互操作、锁争用、取消、正常关闭/重开/checkpoint | bounded 正常完成回归；不含故障注入、强杀、受限 namespace、物理恢复、错误密钥或磁盘篡改 |
| §5.2、M4–M6 | 100×128 组合成员核验、NFC 改变输入、数学参考及实际索引对 scan 证据 | 规范是至少 10,000 个规范化随机组合，不是 10,000 条 query。新 41 目标 fuzz 被阻塞，旧内存测试通过不是重新启动许可 |
| §11.3 | no-CGO 五目标构建和精确 Library 恢复曾通过 | 构建、备份字节身份与运行分别报告；交叉编译不等于原生文件锁/恢复验收 |

2026-10-11 首轮仅核对源码身份、Library 和文档；其后独立 inspect 修正完成
完整普通单元/vet/build、完整 pinned oracle、全 race、五目标编译与全源身份门，
组合 tree `0124d04f32966846bc38537863274390ca5daccc` 于 02:21:52 exit 0。
文档提交 e40 的 CI `38103659605` 于 02:13:13 SUCCESS，Library 恢复亦通过。
这些新结果范围明确，不改变旧结果身份或下列未完成项。
随后 inspect metadata tree `185944581731b5f21a5582ceb60ed340d4ad9012`
完整普通/oracle/race/五目标/模块/身份门于 03:05:03 exit 0。基点 7a3c457
的 CI `38105152184` 已于 02:48:20 SUCCESS，精确 Library 恢复 02:29:59 PASS。
独立后续任务必须先写清源码范围、测试选择以及与被中断链路无关的理由。
不因允许普通测试就运行新的广泛 fuzz、soak 或此前未收尾的冻结候选。新增源码批次按具体动作及依赖审阅；旧谨慎暂停不能反推为已证明的源码编辑禁令，也不能以本次纠正重试未知被阻动作。

## 三、受限待人工：已有测试与未实现设施分开

| 原规范条款 | 状态 | 自测入口和限制 |
|---|---|---|
| §4.1、M3、§11.1 | `平台阻塞 / NOT_RUN`：实际 native rollback replay、public production bridge 和开启恢复的真实 Open | 内部 harness 的原门控/命令保留；新增 [TestManualPublicRollbackRecovery 的四场景规程](public-rollback-manual-zh.md)已有独立源码/编译资格，物理全部 NOT_RUN。它不覆盖 Connector、真实断电、强杀、短写、Sync/ENOSPC/torn write；完整设施仍缺 |
| §4.1、§11.1 | `平台阻塞 / NOT_RUN（当前）`：已有 plaintext namespace 测试 | 手册列出 manual_namespace 独立门。历史 M2 至 7addc493 全套曾包含两项 namespace 测试，原 PASS 是事实，不能改称当时全都未运行或当时范围完全干净 |
| §4.1、§8、§11.3 | `平台阻塞 / NOT_RUN`：Windows/Darwin 两个 encstore raw namespace 测试 | 本批完整移动原函数，manual_namespace 加显式环境门；default/oracle 刻意减少这两个声明，其余原声明保持。source/model/race、188 项选择和六项外平台编译/符号独立 PASS；[中文第 19 节](manual-verification-zh.md#19-os-raw-namespace-门控与独立原生验收2026-10-11) 给独立用户步骤，编译不是 native PASS |
| M3/M4/M7/M8、§8.3、§11.1 | `未实现设施 + 平台阻塞`：完整物理故障/崩溃/重启、checkpoint/rekey 中断、安全篡改矩阵 | 没有现成 harness 的项先补源码和独立审阅；只提供实验前置/断言，不编造现成测试名或成功命令 |
| M7/M10 | `平台阻塞 / 未知触发`：ANN V4、41 目标 fuzz、未知整轮中断 | 不重发/换路。ANN V3 FAIL 是未发布报告工具缺陷，不自动推断生产 ANN 算法已坏，也不能因为受阻改成 PASS |

步骤见 [中文手工验收手册](manual-verification-zh.md)；
旧命令和历史范围见 [manual-verification.md](manual-verification.md)。
写入命令不等于执行或通过，用户未来日志仍须核对精确源码与环境。

## 四、需要真实外部环境

| 原规范条款 | 缺少的环境/证据 | 状态 |
|---|---|---|
| §11.3、M3/M8/M10 | Linux/amd64、Linux/arm64、Darwin/arm64、Windows/amd64 实际文件锁、目录同步和恢复边界 | 部分 Linux 普通证据，其他原生门 NOT_RUN。本批两个 encstore namespace 测试已移入 manual tag/env gate，三目标六项编译/符号 PASS 不等于原生资格；新 OS runner 未启用，不直接默认全套 |
| §11.3、M10 | 固定硬件/文件系统/冷暖缓存的大规模基准及长并发 soak | `NOT_RUN`；两小时计划没启动。10 日预检显示默认 /tmp 对拟议长测余量不足，当时 workspace 挂载为 fsync=volatile；未来运行前须重新核验，普通 Sync 不能证明断电持久性 |
| §8.2/8.3、§11.3 | 有资格且独立的数据安全、密码学、威胁模型、依赖许可/法律、API 审查 | `NOT_RUN`；开发工程复审和自动扫描不能替代这些资格与结论 |
| §9.2、M9 | 授权的可丢弃 PostgreSQL/Neon、已核验版本/依赖、回滚快照 | `NOT_RUN`；离线转换/生成 SQL 不等于服务器执行成功；pgx 直连本身为可选项 |
| §11.3 | Actions 额度/计费确认和新性能/OS 作业许可 | 待单独许可；继续开发不等于允许读取计费、增加费用或权限 |

## 必须保留的历史失败和测量

- 首次 rollback 20 秒样本在 14.0159 秒报 ErrChanged；Reader 修正及四次
  20 秒普通样本通过不等于长期 soak 通过。
- Transaction V2/V3 错误图、延迟退休报告和 WAL 配额失败；V4 有界通过。
- Native Check V1 惰性内存假成功；V2 原始控制通过，限定 full-profile。
- Dataset distance V1 的 16,000 维误拒绝；V2 派生外向误差界通过参考核验。
- Planner V3 的 34 个差异、V4 的 6 个 bound-LIMIT 回归、V4b 的 16 个
  非法绑定错误码差异；V4c 有限修正通过，但尚未与 main 组合。
- Plain backup V1 源/目标 sidecar 重叠 P1 只做源码发现，未执行破坏性情形；
  V2 局部通过不替代未完成的最终审阅。WAL-tail V1 nil child Unwrap 缺陷
  已由 V2 局部修正，独立 V2 复核未完成。
- ANN V1 wire 缺陷、V3 三个报告准入缺陷；V4 未完成。
- Performance V1/V2 准入缺陷；V3 有限修正通过，未部署、无合格性能结论。
- Windows 默认 cross-vet 的 mapped-address unsafe.Pointer 告警仍未取得
  干净门禁；不能因 scoped vet 通过而删除原告警。
- Grams medium 512 行约 147 倍 DB 大小、4/18 个选择性中位数慢于 scan，
  小样本全部 12 个更慢；ANN 小数据也有慢于 exact 的实际样本。
  规范没有为所有数据集保证普遍加速，不应捏造通过/失败性能阈值。

历史源码、失败及修正身份保留在证据与 Git 历史。只有满足必要实现和
安全门才能把整个里程碑标为 VERIFIED；当前 M10 尚未完成。

## 历史有限收尾与继续编码

[有限清单](closeout-scope-20261011.md)的三项独立改动及各自 scoped/static 审阅已完成。四项资源 tree `4c2af13646c973c53340450f8f1310492aae5628` 的完整普通门于 03:53:38 UTC PASS；三份合同文档静态 PASS；五例目录终结取得源码、精选五项普通/race 与限定 vet/build 独立 PASS，真实失败目录保留仍未动态验证。

历史最终组合 tree `d7393c254a6fca4452ae194d64d9045659f9781d` 完整普通门 PASS（2026-10-11 04:23:35 UTC，18阶段全部自然 exit 0）。随后保存的63c2检查点，其精确恢复V2于04:41:10 UTC通过36阶段，CI `38111973546` 于04:55:57 UTC SUCCESS，ordinary `114389234612` 和oracle `114389234442` 均成功。原“PENDING”是当时发布前状态，现按后续证据闭合；这些结果不能用于尚在开发的新LIMIT源码。基点 `1352ae524575e4d6fe8b5434fd930c3b07be4ac8` 的 CI `38108065647` 于 03:42:12 UTC SUCCESS，以及该基点的恢复 PASS，各自只证明原身份。

当时“在现有限制下完成了独立可推进项，整体未验收”的结论，只在旧有限清单与过宽暂停分类下成立；它不表示原目标完成、所有源码已完成或存在已证明的全面编码禁令。现按06:53 UTC指示继续逐批推进，其余实现、集成和受限验收义务保留。

其余真实代码缺口、原始 FAIL、具体工程 HOLD/集成待办、已有但受限的人工门、IMPLEMENTATION_MISSING 和真实外部资格分别保留；不再用未定位的源码编辑拒绝统一解释这些待办。ANN V4、41 目标 fuzz 与未知中断的操作/目标没有新增证据，不重发、不换路，不作通过推断。

本次组合普通门原始记录见[五例组合证据](evidence/example-retention-20261011.md)。该次运行之后至63c2发布仅追加文档/证据；777个非 docs 文件的字节、路径、大小与Git模式已独立核对相同。精确恢复曾有30文件的POSIX权限常规归一化，不概括为所有元数据相同。当前新批次须按实际新源码、提交、恢复和CI另行核验。

父基点 `7a9972547f49531df04cf1b624dfa662b483907b` 的 Library `libfile_f28c9a0df2108191981d41eb76fafc58` 已读回并精确恢复；30阶段、18项每模式具名测试/29子项、8 sourceguard、vet/build/五目标/模块/全部身份于04:13:29 UTC PASS。该提交 CI `38110617335` ordinary/oracle 均 SUCCESS（04:30:02 UTC）；[闭合证据](evidence/cli-resources-20261011/persistence.json)只属于父基点，不替代本次新提交。

63c2 的全历史 bundle 为 Library `libfile_ce332d051b4c8191a833e09094b5722e`，5,334,849字节，SHA-256 `5c0c18551f6874a6bc2e5766362e47308432424c366eec5eff448010376bfbf7`；终态证据归档为 `libfile_7fea2541ad7881919b7236a66499c138`，SHA-256 `d1a8e98dae0d1260006f75c938360e04b3b40744b69b1c7d1b4e286800e6bd45`。首次恢复V1的验证器FAIL保留：745/777计数差来自额外排除32个Markdown，在任何Go检查前失败；V2只修筛选谓词。完整身份、分类依据及下一批边界见[限制溯源](restriction-provenance-20261011.md)。

### LIMIT 父基点闭合补记（2026-10-11 08:13 UTC）

已保存父基点 `c1502790c4b233757b0f9a3c4d5e5bdee1321310` 的精确 Library 恢复15阶段于07:57:22 UTC PASS；1016普通叶用例与344官方oracle比较分别在normal/race全通过、零skip，2998文件/779非docs身份保持。该提交 CI `38122662808` 于08:10:53 UTC SUCCESS（ordinary `114420432223`、oracle `114420432058`）。bundle Library `libfile_37b21d0517b48191aa12da9186208cd8`；完整证据归档 `libfile_609edcfa90988191b5f3b29595ec7c8b`，均已读回核验。这些结果只属于LIMIT父基点，不替代本批P0新源码组合、提交恢复与CI。

### P0 默认关闭完整普通门结果（2026-10-11 08:22 UTC）

当前P0组合执行冻结tree `442a7104dc780ddb20826e5ee4f1250b291f2b24` 于08:22:39 UTC自然完成18阶段PASS：完整CGO0普通测试/vet/build、固定SQLite3.54.0完整普通oracle、完整CGO1 race、guard、五目标生产编译、模块及全部3004文件/143目录前后身份。785非docs文件固定；最终文档/证据改变整个tree但不得改变这些运行源码。两个外部预检的报告版本/基线POSIX比较问题在Go执行前发现并保留，修正外部验证器后一次完成运行门，无运行重试。详见[精确组合证据](evidence/public-open-integration-20261011.md)。本批新提交/Library恢复/CI仍待实际核验。真实RecoverRollback、production bridge和开启恢复选项的真实Open/Connect均BLOCKED / NOT_RUN，公开物理harness仍IMPLEMENTATION_MISSING；M3/M10/整体未验收。

## 2026-10-11 OS namespace 六路径批次

2026-10-11 namespace 后续更新：本批以已保存 P0 提交
`56bbcf3febe233197aa0a5eac965c67a67a88011`（tree
`c31e7dcbeed9732688681741ee38ec60ff0bfb49`）为组合基点，只移出两个
Windows/Darwin raw namespace 测试并追加通用 Rename/rename selector guard。
恢复 guard 和 LIMIT 成果保持。六路径候选的 source/model/race、188 项选择矩阵
及六项外平台编译/符号核对已有独立 PASS；原生/manual 二进制全部 NOT_RUN。
详见[本批手册](manual-verification-zh.md#19-os-raw-namespace-门控与独立原生验收2026-10-11)及[范围审计](test-scope-audit.md)。
本批新组合、提交、恢复和 CI 必须各自记录，不能继承 P0 或候选 PASS。

P0 父基点 56bb 的 Library bundle `libfile_04182e9da3f88191b348c6790628a7bd`
已读回并核验字节身份；其精确恢复于 08:43:14 UTC 完成 21 项命令、11 项
具名测试在普通与 race 均 PASS。CI `38125268420` 于 08:44 UTC 仍在进行，
尚无终态；bundle 字节身份与恢复测试是分开的证据。P0 默认关闭完整普通门的既有结果
只属于其冻结 tree 442a；本批 source/model/compile 结果只属于候选 01918609。


## 2026-10-11 namespace 最终组合限定门与 P0 持久闭合

运行冻结 tree `697e45c6aba7a5cf62ebbf2bab2662f10903c5be` 于09:01完成独立核验：3070文件、788非docs与已审候选完整字节/Git/POSIX模式相同；guard、4项纯模型及同集合race PASS，manual GOFLAGS按预期拒绝。实际7条Go工具调用含早于后发5条限定的只读version/env前检，完整记录保留。另157链接、45shell语法块、29选录工件/617前置原件哈希PASS。没有运行该组合完整aggregate/oracle、外平台二进制或手工物理测试。报告见[最终组合限定证据](evidence/namespace-gating-20261011/final-composition-report.md)。此后仅追加终态文档/证据；最终提交的788非docs须逐字节核验等同该运行冻结源。

父P0提交 `56bbcf3febe233197aa0a5eac965c67a67a88011` 的 CI `38125268420` 已于08:56:41 UTC SUCCESS，两job ordinary114428206934/oracle114428207050均SUCCESS；精确Library恢复21条命令PASS。终态证据归档 `libfile_4a413988d4608191aebf695542a04caf`，10211483B，SHA256 `0d6579c338465be5df401b4f2a594df8fe63737900a78dbcc9e4ef28e85bbfb3`，读回411条/410文件哈希PASS。原08:44/08:45 PENDING是先前写作时点，现仅对父P0闭合，不能当作namespace新提交CI。


## 2026-10-11 有限公开恢复 harness 源码与编译资格

- 本次组合基点是已保存的 namespace 提交
  `50b627e11e9dbce7f2fde8df35c4634c52a06d52`，tree
  `8f2709293287a3517a52b3f989888ded8230ef53`。仅新增一个手工测试源码和
  配套专题手册，保留 LIMIT、P0、namespace 源码及历史；没有生产行为或 CI 改动。
- 独立 V3 候选 tree `1755d67431f9de082449b08fa24b26b2b35eb612`
  是 P0 `56bbcf3febe233197aa0a5eac965c67a67a88011` 加两项新增文件，
  **不是**当前 namespace 组合或新提交的 tree。单一测试源码 SHA-256
  `dd5a63e2a8e7d4fdd969ba03273a8f8147d5eff81babf8fce44a5228c5cdaf5e`；
  原 V3 审阅手册 SHA-256
  `a49ae1882c4a5dcb8c58860fa8df3b011f88c9ee99963a5721831049477ec4d2`。
- V1 的三项阻断发现原样保留；V2 修正 fail-open 选择检查、备份及 manifest
  全量核对、失败即停与一次 Close；V3 仅改手册的环境设置先后和 archive
  分步检查。六项检查于 08:58:38 UTC 自然完成：AST inspector、三次
  go list、go test -c、go tool nm 全部 PASS；default/oracle/manual 文件次数
  为 0/0/1，精确顶层测试符号唯一。未执行任何 test binary、fixture 生成器、
  gate-off、-test.list、无匹配选择器或物理用例。
- 实际源码是 [TestManualPublicRollbackRecovery](../sqlite/rollback_recovery_manual_linux_test.go)，
  首行 `//go:build manual_recovery && linux`，首语句环境门缺值即 Fatal。
  只含 nil-default 拒绝并保留输入、opt-in 恢复及只读重开、journal 配额拒绝、
  NoCreate 保持缺失四场景；源码/编译 PASS 不等于四场景运行 PASS。
- [证据摘要](evidence/public-recovery-harness-20261011.md)、
  [V3 独立报告](evidence/public-recovery-harness-20261011/v3-compiler-review.md)
  和[精确命令](evidence/public-recovery-harness-20261011/compiler-commands.json)
  分开记录候选资格与当前组合身份。[中文步骤](public-rollback-manual-zh.md)
  的发布状态段单独更新，命令及断言保持；[原 V3 审阅手册](evidence/public-recovery-harness-20261011/v3-reviewed-manual.md.txt)
  保存当时原字节。旧审阅身份与当前发布文档不混同，物理 NOT_RUN 不变。
- 本次写作时，namespace 提交的 Library
  `libfile_e5dcbc0ac16c8191bce977aa165163f9` 已读回核验字节身份；其精确限定
  恢复的 207 项调用于 09:14:09 UTC 完成 PASS，涵盖纯模型/选择/编译/符号，
  不含完整普通/oracle 运行或原生物理测试。CI `38127196102` 尚无已核验终态，
  本次最后已知为 in_progress，不能写为 SUCCESS。
  父 P0 的 CI `38125268420` 于 08:56:41 UTC SUCCESS 及恢复 PASS 已在上一节
  闭合，只属于父 P0。本次新增 harness 的组合、新提交/保存/恢复/CI
  仍须各自按实际身份追加，不能继承上述任何结果。

公开 Open 四场景不再整体列为 IMPLEMENTATION_MISSING；driver Connector
真实恢复 harness、更广 journal/平台/native WAL-tail publisher 和完整 fault/crash
设施仍有实现缺口。实际 public bridge、恢复启用的 Open/Connect 与全部物理场景
仍 BLOCKED / NOT_RUN。M3、M10 和原始目标未完整验收。


## 2026-10-11 09:32 UTC：namespace 父检查点终态闭合

提交 `50b627e11e9dbce7f2fde8df35c4634c52a06d52`、tree `8f2709293287a3517a52b3f989888ded8230ef53` 的 CI `38127196102` 于09:29:33Z自然完成 SUCCESS；ordinary `114433911927`、oracle `114433912045` 均SUCCESS。Library bundle `libfile_e5dcbc0ac16c8191bce977aa165163f9` 已读回并精确恢复，207条限定检查PASS。全部manual/真实namespace仍NOT_RUN。终态证据归档 `libfile_6b7a1c37d8fc8191bed2389e8dbe14dd`，15356765字节，SHA256 `21cd1a73b622123605eb48a5031b0cc38314a04eac8a9b71c72f873ecaa3d701`，Library读回843项/842清单哈希PASS。此前“运行中”只表示当时观测；此终态不属于新增public harness提交，不代替它的保存/恢复/CI。


## 2026-10-11 09:35 UTC：public harness 精确组合限定检查完成

独立组合树 `0af9b064bebfbec9c8270c4bf2f10ec2055fb186`，3104文件/789非docs（原788全部保持，加唯一手工测试源码），于09:32:51完成六条AST/list/编译/符号命令 PASS；196相对链接和48shell块仅静态检查PASS。独立报告SHA256 `ba6e3c4458d8d192b9dd2f231936c61706d9f6b2fdb64249f34ea34d1b71d765`，81项证据清单SHA256 `394e491d4564c594cb7f7e1e304495f95c8b620122c7e7408a14bcae30f6279e`。物理四场景全部BLOCKED/NOT_RUN，二进制从未执行。随后仅追加文档和证据；发布前逐路径核对全部789非docs不变，文档另做最终静态读回。精确新提交、Library恢复和CI在实际完成后另记，不继承父namespace的成功。

## b349 公开 harness 检查点终态与后续候选的分界

b349 的全历史 bundle：Library `libfile_c416989d053081919f257c1f3200564c`，
5,769,076 字节，SHA-256 `dfe931d13fce5ec1984b50e69128478a2d025176827306a6585898845b8f4251`。
读回逐字节、bundle/fresh clone/fsck/精确 HEAD/tree/goal 检查 PASS；精确恢复
3,128 文件、789 非 docs，六条 AST/list/compile/nm Go 调用 PASS，二进制未运行。
四个物理场景仍 BLOCKED / NOT_RUN。终态归档 Library
`libfile_611e42320f0081919d6ef2a4f490a493`，12,197,085 字节，SHA-256
`d0e5477c3b8e01daea3af7d23b1cbdd73a084fa9347ace441110e54c0ecade9e`，
读回 266 条目 / 265 文件哈希 PASS。CI 的真实 SUCCESS 已在页首限定。

startup 修正收口的是用户初始化和继承环境输入；静态审计没有读取真实 sqliterc、
真实环境变量值，也没有观察或推断历史额外命令。`-version/-help` 位于初始化之后，
SQLITE_DEBUG_BREAK 位于 flags 之前；只加 `-noinit` 或只改 HOME 均不足。
候选覆盖全部 46 Go / 5 shell 既有站点；另有 LIMIT 两处已在四切片组合中适配。
该组合普通门现已 FAIL，实际 oracle 仍未运行；备份一处仍在外部候选。三处均需
单列，不能把 51 个源码点数当作进程运行数。纯模型、静态 source guard、
新 oracle、精确组合、持久恢复、CI 各自立项记结果；目前不能互相替代。

## 12:04 UTC 独立 backup 与 Windows 切片的分界

backup tree `fc0b1324441456f40160e81d5125dc7b97992773` 的六条限定 normal
纯模型/源码 AST 命令 PASS：34 顶层、142 命名子项，共 176 路径，零 skip/fail。
源码未发布，六个旧 native Backup 用例尚未迁移；原生持久性、oracle、aggregate
及通用备份能力未验收，main 明文 backup 仍 unsupported，不能混入本批源码资格。
Windows 候选 `5d1e457eed1e01b3705c0650c7aa9e42efa96559` 的限定模型/AST
normal/race 及单项 Linux 正确密钥 shared-rekey normal/race 有独立 PASS，仍未
发布、不属于本批组合，也不提升 Windows 原生资格。原始报告和归档见
[分层证据](evidence/startup-wal-limit-20261011.md)。


## 2026-10-11 Darwin 当前源码与九文档补记

已发布基点 `3ea6d3ab15f678fe48bee312921da60828845869` 的 tree 为
`f6edabfba3e004a0e7874441b8c0a3e38cf099c6`。当前十路径源码候选 tree
`88c400a4caeb40d8d96fae7700e7048d69db4ff5` 获限定独立源码审阅，报告
`darwin-current-composed-independent-review-v1/REPORT.md` 的 SHA-256 为
`ec90e774280c6669a44911ed5bebcf646f757dbc7c54d4c0d675f1fa6e96e134`。
该结论不等于 main 已含此修正，含本补记的九文档组合还需独立文档审阅。

[清单证明](darwin-unicode-inventory.md) 只覆盖可打印 ASCII 根名内的前导差异
及实际枚举 owner 的本地 APFS/hfs subtype 0–3 门，177 字节根名与 78 字节后缀
预留不变。V1 文档的 NOT_RUN 记录保持历史身份；固定源码的 Linux/amd64 精确
20 项模型/源码 normal/race 已各一次 PASS，额外编译门和原生验证仍 NOT_RUN；
一般 Unicode 根名与真实 managed-directory/Unicode-sibling 资格 harness 仍为
IMPLEMENTATION_MISSING。AST 不构成控制流证明，现有 ASCII/nil-directory 原生
测试不能覆盖新门。历史 V3、Windows b0e 本地 17 门及新基点 CI 各属其精确身份，
不授予当前 Darwin PASS。M3/M8/M10、安全/恢复/生产验收和既有物理限制保持开放。
