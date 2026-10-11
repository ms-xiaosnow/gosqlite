# 当前环境的有限收尾边界

核对日期：2026-10-11。原始 `goal.md` 不变。本表按原规范、当前实现和已有证据列出有限收尾项；它不是缩减原目标，也不是完整验收证书。进展与最终持久身份以 [验收矩阵](acceptance-status.md)、[checkpoints.json](checkpoints.json)、具名证据为准。

## 一、本轮有限收尾清单

| 项目 | 原规范依据 | 明确结束条件 |
|---|---|---|
| query 关闭错误合并；readonly 显式只读与资源归还；write 资源归还后输出；sparse-exact 状态传递 | G1 的上下文/资源/错误行为；§12.1 可编译示例；§12.2 CLI；§0/§14 如实记录 | 有限修正与独立 18 项普通/race 已完成；固定 tree 4c2af13646c973c53340450f8f1310492aae5628 的完整普通门 PASS。最终组合及新提交持久门另列，不扩大修正范围 |
| 必需入口文档对账 | §12.3 迁移指南、格式说明、安全文档 | migration-pgembedding.md、file-formats.md、security.md 已按实际入口、layout 1/2/3 和已修复/仍存风险对账，独立静态审阅 PASS；历史失败、原阶段契约和外部验收限制保留 |
| 已知五个示例的失败现场与错误报告 | G1/§12.1 的资源和错误行为；现有手册披露的规程缺口 | encrypted、encrypted-wal、encrypted-shared、encrypted-cli、native-check 的有限目录终结/输出错误修正已完成；源码、纯 helper/AST、原有两项正常流程的五个精确测试普通/race 与限定 vet/build 独立 PASS。未动态证明真实失败现场保留，不扩展成整仓示例改造 |
| 状态、步骤与证据一致 | §0、§12.3、§14 | 全部 PASS 指向精确源码/命令/结果；HOLD、FAIL、未实现设施、NOT_RUN、外部资格分开；中文命令均有实际实现，未执行的命令不记为成功 |

最终组合 tree `d7393c254a6fca4452ae194d64d9045659f9781d` 的完整普通门：PASS（2026-10-11 04:23:35 UTC，18阶段全部自然 exit 0）；最终新提交的精确持久恢复和 current CI：PENDING。以上不是失败或跳过，也不能借旧基点结果改记成功。每项只有在对应原始结果与精确源码身份齐全后，才单独更新。五例留存的操作步骤见[限定复验补充](example-retention-verification-zh.md)。

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

本轮三项有限改动及各自独立 scoped/static 审阅已经完成；最终组合普通门及新提交持久证据按第一节逐项闭合。完成后不再为延长任务自行添加新功能；若发现旧 HOLD 的依赖，暂停该依赖并报告，不另起同义链路。

在现有限制下完成了独立可推进项，整体未验收。

该结论不表示所有当前可实现功能已完成，也不表示只剩人工测试。公开恢复、WAL-tail publisher、支持域 SQL 正确性、明文备份、真正只读 WAL、部分 backend 和完整故障设施仍有实现或集成缺口；ANN V3 原始 FAIL、ANN V4、41 目标 fuzz 与未知中断/HOLD 不变。当前 M10 未完成，不能宣布生产安全、原始 goal 或 M0–M10 全部 VERIFIED。

## 六、维护者的实际下一步

以下是边界清单，不是本轮继续执行或开发的授权；应保留在收尾说明中，避免笼统说“之后人工验收”。

### 已有可执行入口但当前受限或未运行

- Linux 有限 rollback recovery：`internal/pager/recovery_manual_test.go` 的 `TestManualRollbackRecovery`，独立 manual_recovery gate；具体已实现断言与限制见中文主手册 12.1。当前 BLOCKED / NOT_RUN。
- 两个 plaintext namespace 回归：`sqlite/write_namespace_manual_test.go` 中两个真实具名测试，见主手册 12.2；当前 BLOCKED / NOT_RUN，历史曾运行事实单独保留。
- 现有普通 SQL/互操作、grams/精确向量、正确密钥维护和离线迁移入口见主手册 5–10；未来只对所选提交和授权范围逐项复验，不把这份清单当作立即重跑整套的指令。
- 事务 V4、grams 测量等现有工具可用于将来的有资源预算长测，见主手册 11；完整 soak 与大规模合格结果仍 NOT_RUN。小样本不是长期资格。

### 首先需要实际代码或 harness 并重新审阅

- public Open rollback 集成、native exclusive WAL-tail publisher、支持 SQL 域内已知 planner/阶段/排序错误，旧候选均按现状 HOLD，不在本轮另建修复链。
- main 明文 backup、WAL/非 Linux backup、真正只读 plaintext-WAL 入口、尚未整合的 backend/目录所有权候选及完整格式升级/回退覆盖。
- 完整 crash/power-loss/process-exit、短写/Sync/ENOSPC/torn-write、checkpoint/rekey 中断与磁盘认证负面矩阵：IMPLEMENTATION_MISSING 加受限执行；先补具体协议、实现、断言与独立审阅，不能编造命令。
- ANN V3 的三个源码/报告准入 FAIL 保留；V4 未完成，41 目标 fuzz 和未知中断链路没有明确操作/目标。当前为 HOLD/UNKNOWN/BLOCKED，不是只差一台外部测试机。

### 需要真实外部环境或有资格的独立审查

- 精确源码在真实目标 OS/架构、文件系统和具持久性语义硬件上的原生锁、mapping、目录同步与恢复边界。新 OS 全量运行前还需先解决 raw namespace 门控集成。
- 已声明数据与负载的长期 soak、大规模固定硬件性能测量、实际冷暖缓存及资源预算；不得用交叉编译或 cloud tmpfs 的正常成功代替。
- 授权可丢弃 PG/Neon 的实际版本、依赖、权限、模板与回滚快照；离线 SQL/NDJSON 转换不能代替服务器执行。
- 独立数据安全、密码学、威胁模型、依赖许可/法律和稳定 API 审查。一般工程审阅不能替代相应资格与结论。
- 新 runner、计费/额度读取、新费用或权限先单独取得授权；本次既有普通 CI 权限不扩展到这些动作。

本次组合普通门原始记录见[五例组合证据](evidence/example-retention-20261011.md)。后续仅文档/证据变更；777个非 docs 文件的字节、权限、Git模式与清单已独立核对相同，仍按实际新提交另行核验恢复和CI。

父基点 `7a9972547f49531df04cf1b624dfa662b483907b` 的 Library `libfile_f28c9a0df2108191981d41eb76fafc58` 已读回并精确恢复；30阶段、18项每模式具名测试/29子项、8 sourceguard、vet/build/五目标/模块/全部身份于04:13:29 UTC PASS。该提交 CI `38110617335` ordinary/oracle 均 SUCCESS（04:30:02 UTC）；[闭合证据](evidence/cli-resources-20261011/persistence.json)只属于父基点，不替代本次新提交。
