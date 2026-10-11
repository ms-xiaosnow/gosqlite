# 当前环境的有限收尾边界

核对日期：2026-10-11。原始 `goal.md` 不变。本表按原规范、当前实现和已有证据列出有限收尾项；它不是缩减原目标，也不是完整验收证书。进展与最终持久身份以 [验收矩阵](acceptance-status.md)、[checkpoints.json](checkpoints.json)、具名证据为准。

## 一、本轮有限收尾清单

| 项目 | 原规范依据 | 明确结束条件 |
|---|---|---|
| query 关闭错误合并；readonly 显式只读与资源归还；write 资源归还后输出；sparse-exact 状态传递 | G1 的上下文/资源/错误行为；§12.1 可编译示例；§12.2 CLI；§0/§14 如实记录 | 已固定四项组合，独立 18 项普通/race 检查通过；完整普通门、精确保存/恢复及 CI 终态据实闭合。不得重复作为新功能无限扩展 |
| 必需入口文档对账 | §12.3 迁移指南、格式说明、安全文档 | 修正 migration-pgembedding.md 的过时 checkpoint A 入口；区分 file-formats.md 的 layout 1/2/3；security.md 区分已修复的 Reader/shared 来源问题和仍存在的风险。保留历史失败和原阶段契约 |
| 已知五个示例的失败现场与错误报告 | G1/§12.1 的资源和错误行为；现有手册披露的规程缺口 | 限 encrypted、encrypted-wal、encrypted-shared、encrypted-cli、native-check 的目录终结/输出错误；仅纯 helper、AST 和原有正常成功路径验证。不得扩展成整仓示例改造，不把源码断言当真实故障现场保留证明 |
| 状态、步骤与证据一致 | §0、§12.3、§14 | 全部 PASS 指向精确源码/命令/结果；HOLD、FAIL、未实现设施、NOT_RUN、外部资格分开；中文命令均有实际实现，未执行的命令不记为成功 |

第三项是已明确选择的有限规程修正，不是原 goal 逐字要求新增的数据库内核功能。现有完整能力（Native Check V2、Inspect、正确密钥维护 CLI、迁移数据精确距离复核、grams/事务测量等）不重复列作待开发来拉长清单。

## 二、仍缺实现或未合入，不能变成“只差人工测试”

| 优先项 | 真实状态 | 为什么不能按普通独立收尾推进 |
|---|---|---|
| §4.1 P0 恢复与安全发布 | public Open rollback 恢复未合入；native exclusive WAL-tail publisher 仍缺 | 旧 native-recovery/a314028、WAL-tail/715b15 工作 HOLD，不能改名或换路线继续。普通 reopen/checkpoint 不等于热恢复 |
| 已声明支持 SQL 的正确性 | CAST/IPK、LEFT-ON/WHERE、排序/执行阶段等差异；planner V4c 尚未与 main grouped/counter 组合 | 旧 planner 链路 HOLD；这不是可忽略的后期新语法，也不是缺一台测试机器 |
| 明文备份、真正只读 WAL | main plaintext backup 仍 unsupported；rollback V2 未终审/合入；WAL/非 Linux 备份及真正只读 WAL 入口缺实现 | 与旧 backup、WAL、目录所有权候选重叠，保持 HOLD；已有可写协调 named Check 不称为只读 |
| 原生 backend/目录所有权 | Windows shared rekey、Darwin V3、public rollback-open、OS namespace 门控候选未合入；retained-directory 未完整冻结/整合 | 旧组合与未完成目录候选 HOLD；不能把交叉编译称为原生运行，也不能直接开启未经选择审阅的新 OS 全套 |
| ANN 报告和发布测量 | ANN V3 三个报告准入缺陷仍 FAIL；V4 未完成；performance V3 未合入/部署 | ANN V4、41 目标 fuzz 和未知中断链路保持阻塞。旧算法 PASS 不消除报告工具 FAIL；报告工具 FAIL 也不自动证明生产 ANN 算法全部错误 |
| 完整物理安全设施 | crash/power-loss/process-exit、short write、Sync/ENOSPC、torn write、checkpoint/rekey 中断、完整磁盘认证负面矩阵等 harness 仍有源码缺口 | 标记 IMPLEMENTATION_MISSING 加受限执行；没有现成完整命令，不编造测试名来让操作者误以为只待运行 |
| 格式升级/回退与恢复测量 | 全部受支持模式的升级/回退覆盖、WAL 恢复耗时工具及必要安全恢复路径尚未闭合 | 既有少数跨版本 fixture/普通测量不能代表完整矩阵 |

精确候选身份和原始失败见 [中断记录](interruption-20261010.md) 与 [验收矩阵](acceptance-status.md)。静态归档、Library 读回和 Git 树一致只证明代码保存，不能解除 HOLD 或提供运行证据。

## 三、代码已有的人工门与真实外部资格

- 已存在的有限 manual rollback recovery 和两个 manual_namespace 回归，仍按主手册的标签、单项选择和实际覆盖记录；它们不覆盖上表缺失的完整物理故障设施。
- 已实现 backend 在真实目标 OS/架构、文件系统和持久性硬件上的锁、mapping、目录同步、恢复边界验证。
- 已有普通长测工具的长期并发 soak、固定硬件和数据集的大规模测量；短样本和有界正确性测试不替代。
- 现有加密、事务和索引实现对应的物理安全资格及独立数据安全/密码学/威胁模型审查；不能用正确密钥 round-trip 或无明文 marker 代替。
- 独立依赖许可/法律与稳定 API 审查，以及授权可丢弃 PG/Neon 上的真实版本/依赖/模板验证。
- 新 OS/perf runner、计费/额度读取和新费用/权限仍待单独授权。现有普通 CI 的批准不扩展到这些操作。

环境限制、隔离副本、可复制命令、预期断言、失败排查、清理和证据保存见 [中文手工验收手册](manual-verification-zh.md)。四项 CLI/示例的补充规程见 [限定复验](cli-resource-verification-zh.md)。未知触发、失败和未运行项不能因为“跳过”而改记 PASS。

## 四、分期范围不能被悄悄改写

- §4.2 原文为“建议按下列能力矩阵推进”。CTE、UPSERT、FK、view/trigger、window、JSON、FTS5、ATTACH、VACUUM 等仍按兼容矩阵记录未实现/部分实现，本轮不无限补语法；已声明支持域的错误行为和数据安全不能因此豁免。
- 通用 backup API 列于第三批，§12.2 命令是“目标行为”。明文 backup 目标仍在，完整 CLI 仍 PARTIAL；不能将每个命令×每种加密/WAL 模式都不加说明地升格为当前必须新实现，也不能宣称它们已齐全。
- 加密 inspect 确实未实现，应准确标出所支持 profile；原规范没有给出逐命令×逐模式的强制矩阵。
- §9.2 将 pgx 直连明文列作可选；CSV/NDJSON 最低路径与真实 PG/Neon 原地迁移是不同工作流。

## 五、达到有限边界之后

本轮前三项有限改动的精确门禁、必要文档和持久证据闭合后，不再为延长任务自行添加新功能。若收尾发现旧 HOLD 的依赖，暂停该依赖并报告，不另起同义链路。

允许的结论是：“本环境已确认的独立收尾项已完成；现有声明范围取得列明的普通验证证据，剩余实现缺口和受限/外部验收逐项保留。”

不能写“所有当前可实现功能已完成”“只剩人工测试”“M0–M10 全部 VERIFIED”或“生产安全验收通过”。完整原始目标仍需上表的实际代码、安全与资格门共同闭合；在此之前不作最终交付声明。
