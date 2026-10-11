# 当前实现与验收矩阵

核对时间：2026-10-11；源码基点为 `776b23eb59ce9613bf4498d352a716a193cdc9c3`。
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
| G1、§3.6、§12.2 | query/check CLI 使用 OpenContext | `待合入独立修正`：inspect 打开使用 sqlite.Open，未向打开阶段传递 caller context；当前只输出 header/schema，扩展配置/代际可见性有限。原码在新增 AST 传递断言中 FAIL；既有只读运行回归通过，尚无实测取消时限失败证据 |
| §12.1、§14 | 可运行示例存在，普通功能样本有历史证据 | `示例报告/证据缺口`：sparse-exact 错误可能仍返回 0；write 示例忽略 Close，log.Fatal 绕过 defer；部分加密/native-check 示例错误时也删除临时 fixture。手册精确披露，不把这些示例当完整失败现场或资源退休证明；独立修正另计 |
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

2026-10-11 首轮仅核对源码身份、Library 和文档，不把旧结果登记为新执行。
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
