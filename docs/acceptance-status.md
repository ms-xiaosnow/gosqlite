# 当前实现与验收矩阵

核对时间：2026-10-11；本次修改的已保存父基点为 `7a9972547f49531df04cf1b624dfa662b483907b`。
本次最终保存提交与 current CI 发布时仍待按精确 commit 确认；这里不预写尚未生成的自指提交。旧 `1352ae524575e4d6fe8b5434fd930c3b07be4ac8` 保留为历史基点。
本检查点叠加三项有限收尾：四项 CLI/示例资源修正、三份必需入口文档对账、五例目录终结与输出错误修正。四项 CLI 固定树完整普通门、三份文档独立静态审阅、五例独立源码和限定普通/race 审阅分别 PASS；最终组合与新提交持久门见文末，不能沿用基点结果。四项 CLI 的精确结果见[组合证据](evidence/cli-resources-20261011.md)，五例的操作边界见[留存限定复验](example-retention-verification-zh.md)。
已有 inspect 精确源码和边界见[元数据证据](evidence/inspect-metadata-20261011.md)和[上下文证据](evidence/inspect-context-20261011.md)。
这是开发检查点清单，不是最终交付。原始 [goal.md](../goal.md) 不变。
平台受限项目可以暂停执行并记录用户自测步骤，不能取消真实实现义务。

## 状态定义与证据

- `PASS（限定范围）`：有精确源码、实际命令与结果；仅覆盖记录的范围。
- `PARTIAL / 未实现`：必需代码未完成、未合入或仅支持部分配置；文档不能替代实现。
- `FAIL / HOLD`：已有失败或审阅阻断；后续有限通过不能抹去历史失败。
- `NOT_RUN`：没有执行证据。`平台阻塞` 还表示禁止在本环境重试或换路执行。
- `需真实外部环境`：缺少实际目标 OS、持久性硬件、独立审计或授权服务证据。

[2026-10-10 原规范审计](evidence/goal-acceptance-audit-20261010.md)
保留自己的较早源码和时间身份。当前摘要以下表为准；
[checkpoints.json](checkpoints.json)、[progress.md](progress.md) 与
[中断记录](interruption-20261010.md) 保留具体历史证据。

## 一、必须实现：仍有真实代码缺口

| 原规范条款 | main 已有能力 | 剩余代码 / 状态 |
|---|---|---|
| G1、§3、§4.1、M3 | 有界有限计数多段 rollback 字节解析、internal 恢复所有权；Reader post-SHARED stat/shared 初始不存在来源已修正 | `PARTIAL`：公开 Open 恢复尚未合入，更多 journal 形式及 native exclusive WAL 尾部安全发布缺实现。物理恢复另列人工门 |
| G1、§4.2、§11.1/11.3 | 索引范围、有限 JOIN/GROUP BY/HAVING、连接计数和 cached preparation 的限定普通门通过 | `已知差异`：支持查询的 planner 错误/排序/阶段一致性仍待修正。V4c 树 `552286a…` 有独立有限 PASS，但没有和 main 新 grouped/counter 组合；不能替代 main 结果 |
| §4.2、§8.1、§12.2 | correct-key 加密 backup/rekey CLI、native DB/Tx/CLI Check V2 已合入 | `未实现/未合入`：main 明文 backup 命令仍明确 unsupported；Linux rollback V2 候选终审未完成，WAL/非 Linux backup 和真正只读 plaintext-WAL CLI 缺代码。native VACUUM 未实现 |
| G1、§3.6、§12.2 | query/check/inspect CLI 均使用 OpenContext；DB.Inspect 和 inspect --extensions 在单快照呈现持久 method/version/config/generation，默认 JSON 保持 | `PARTIAL`：本次实测只涵盖 Linux plaintext rollback、grams/custom metadata；加密 inspect、真正只读 WAL、完整 CLI 目标仍缺实现。持久 metadata 不证明索引可用或正确，中途 I/O 取消未动态验证 |
| §12.1、§14 | 四项 CLI/示例资源修正与五例目录终结/输出错误修正已完成，各自取得限定独立审阅与普通证据 | `限定修正 PASS，完整验收未完成`：sparse 状态、readonly 显式只读、write/query 关闭错误及五例目录终结已有证据。实际失败进程/原生关闭失败/真实失败目录保留未执行；WAL 原 t.TempDir 清理和清除进程密钥不允许推断失败现场完整或可解密 |
| §9.2(2/5) | caller-dataset 范数、完整精确距离与 Top-K v2 已合入；高维 v1 失败保留，v2 有有理数参考验证 | `PARTIAL`：ANN caller-dataset recall 及真实 PG/Neon 验证单列；精确距离不能替代 ANN |
| §5.2、§7.2、§11.3 | NFC 组合语料、真实 grams build/空间/P50/P95 工具和普通事务 V4 已合入 | `FAIL/HOLD（候选工具）`：ANN 报告 V3 三类实质准入缺陷未修，V4 链路受阻。性能报告 V3 有限审阅通过但未合入/部署；没有可采信测量不得宣称完整基准通过 |
| §8、§11.3、§12.3 | Linux/Darwin/Windows 有受支持 backend 源码，只有 Linux 普通运行证据 | `PARTIAL`：main Windows shared rekey 及 Darwin 名称配置受限；修正版在未发布 `a314028…` 组合。更广 Unicode root 支持未实现，但不能虚构为规范要求任意无限名字。保留目录 VFS 未冻结/整合 |
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
不因允许普通测试就运行新的广泛 fuzz、soak 或此前未收尾的冻结候选。

## 三、受限待人工：已有测试与未实现设施分开

| 原规范条款 | 状态 | 自测入口和限制 |
|---|---|---|
| §4.1、M3、§11.1 | `平台阻塞 / NOT_RUN`：实际 native rollback replay | 中文手册列出 manual_recovery 标签、显式门和真实测试名；现有 deterministic harness 不覆盖真实断电、强杀、短写、Sync/ENOSPC/torn write |
| §4.1、§11.1 | `平台阻塞 / NOT_RUN（当前）`：已有 plaintext namespace 测试 | 手册列出 manual_namespace 独立门。历史 M2 至 7addc493 全套曾包含两项 namespace 测试，原 PASS 是事实，不能改称当时全都未运行或当时范围完全干净 |
| M3/M4/M7/M8、§8.3、§11.1 | `未实现设施 + 平台阻塞`：完整物理故障/崩溃/重启、checkpoint/rekey 中断、安全篡改矩阵 | 没有现成 harness 的项先补源码和独立审阅；只提供实验前置/断言，不编造现成测试名或成功命令 |
| M7/M10 | `平台阻塞 / 未知触发`：ANN V4、41 目标 fuzz、未知整轮中断 | 不重发/换路。ANN V3 FAIL 是未发布报告工具缺陷，不自动推断生产 ANN 算法已坏，也不能因为受阻改成 PASS |

步骤见 [中文手工验收手册](manual-verification-zh.md)；
旧命令和历史范围见 [manual-verification.md](manual-verification.md)。
写入命令不等于执行或通过，用户未来日志仍须核对精确源码与环境。

## 四、需要真实外部环境

| 原规范条款 | 缺少的环境/证据 | 状态 |
|---|---|---|
| §11.3、M3/M8/M10 | Linux/amd64、Linux/arm64、Darwin/arm64、Windows/amd64 实际文件锁、目录同步和恢复边界 | 部分 Linux 普通证据，其他原生门 NOT_RUN。新 OS runner 未启用；main 两项 OS raw namespace 测试尚未手工门控，不能直接默认全套 |
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

## 有限收尾

[有限清单](closeout-scope-20261011.md)的三项独立改动及各自 scoped/static 审阅已完成。四项资源 tree `4c2af13646c973c53340450f8f1310492aae5628` 的完整普通门于 03:53:38 UTC PASS；三份合同文档静态 PASS；五例目录终结取得源码、精选五项普通/race 与限定 vet/build 独立 PASS，真实失败目录保留仍未动态验证。

最终组合 tree `d7393c254a6fca4452ae194d64d9045659f9781d` 完整普通门 PASS（2026-10-11 04:23:35 UTC，18阶段全部自然 exit 0）；最终新提交的精确持久恢复和 current CI PENDING。基点 `1352ae524575e4d6fe8b5434fd930c3b07be4ac8` 的 CI `38108065647` 于 03:42:12 UTC SUCCESS，以及该基点的恢复 PASS，各自只证明原身份。

在现有限制下完成了独立可推进项，整体未验收。

其余真实代码缺口、原始 FAIL/HOLD、已有但受限的人工门、IMPLEMENTATION_MISSING 和真实外部资格分别保留。ANN V4、41 目标 fuzz 与未知中断的操作/目标没有新增证据，不重发、不换路，不作通过推断。

本次组合普通门原始记录见[五例组合证据](evidence/example-retention-20261011.md)。后续仅文档/证据变更；777个非 docs 文件的字节、权限、Git模式与清单已独立核对相同，仍按实际新提交另行核验恢复和CI。

父基点 `7a9972547f49531df04cf1b624dfa662b483907b` 的 Library `libfile_f28c9a0df2108191981d41eb76fafc58` 已读回并精确恢复；30阶段、18项每模式具名测试/29子项、8 sourceguard、vet/build/五目标/模块/全部身份于04:13:29 UTC PASS。该提交 CI `38110617335` ordinary/oracle 均 SUCCESS（04:30:02 UTC）；[闭合证据](evidence/cli-resources-20261011/persistence.json)只属于父基点，不替代本次新提交。
