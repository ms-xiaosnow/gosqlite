### 2026-10-11：三份必需入口文档的静态合同对账

- 状态：IN_PROGRESS（仅完成本次文档修正；M9/M10 和原始 goal 的最终验收未完成）。
- 基点：`1352ae524575e4d6fe8b5434fd930c3b07be4ac8`。依据 goal §12.3，
  本次只改 migration-pgembedding、file-formats、security 三份入口文档的独立副本；
  未改运行源码、测试、原始 goal 或冻结候选。没有新架构/格式/API 变更。
- 迁移入口补齐当前 inspect/convert/import/verify/transition 与 PG16
  full-table-evidence 的职责、真实参数及已有示例；`--apply/--resume/--output`
  的限制按子命令分别记录。保留 A 检查点的历史语境，不再写成当前能力上限。
- 格式入口区分加密 snapshot/layout 1、WAL/layout 2、shared/layout 3；
  修正“没有加密 WAL”和“扩展 codec 尚无 SQL/ANN”的过时叙述，保留具体
  typed ANN 支持矩阵、未知版本拒绝、显式升级与完整迁移/安全门的限制。
- 安全入口将 shared 初始不存在来源与 Reader post-SHARED stat 列为已合入
  的窄源码修正，链接现有精确测试和历史审阅；原失败样本、负向控制与短时
  通过各自保留。没有把这些修正推广为完整生命周期、长期 soak 或生产安全。

| §12.3 要求 / 旧差异 | 当前合同与代码证据 | 既有测试/执行证据及限制 |
|---|---|---|
| migration：SQL/导入/模板仍写为下一阶段 | [迁移入口](migration-pgembedding.md)；migrate.go、migrate_import.go、migrate_transition.go | [转换审阅](evidence/m9-converter-review.txt)、[导入审阅](evidence/m9-import-review.txt)、[迁移 CLI](evidence/m9-transition-cli-independent-review.txt)、[整表审阅](evidence/m9-full-table-independent-review.md)；真实 PG/Neon NOT_RUN |
| migration：校验/续传与回滚边界 | Import / VerifyImport / VerifyImportDistances；逐批原生事务和显式正常续传 | [距离 v1 失败/v2 修正](evidence/import-distance-v2-handoff.md)；完整精确距离不等于 ANN recall 或崩溃恢复 |
| file-formats：否认加密 WAL/shared | sqlite/encryption.go、encstore/wal_format.go、shared_format.go | TestEncryptionWALReviewExplicitUpgradeAndCloseQuota、TestEncryptionSharedExplicitUpgradeCopiesAndRestore；[WAL](evidence/m8-wal-components.txt)、[shared](evidence/m8-shared-review.txt) 既有普通证据 |
| file-formats：否认 typed SQL/ANN | [typed ANN 限定矩阵](typed-ann.md)、sqlite/ann_typed.go | TestTypedANNHNSWAllMetricsPersistentSQLAndNative、TestTypedANNIVFPersistentListsAllMetricsAndDML；[旧组合](evidence/m9-typed-ann-aggregate.txt)，不解除 ANN harness HOLD |
| security：两项旧源码问题仍列 open | shared_session.go / plain_session_cleanup.go；pager.OpenAdmitted | [shared 独立审阅](evidence/shared-absence-lead-review/REVIEW.md)、[Reader 独立审阅](evidence/rollback-reader-acquisition-lead-review/REVIEW.md)、[Reader 原失败与有限通过](evidence/rollback-reader-acquisition-evidence/handoff-v1.md) |

- 本次验证仅为文件内容/路径/参数静态核对：三份文档共 79 个仓库相对链接
  均可定位；15 个精确测试名均在当前源码定义；10 条示例命令的子命令和参数
  均对照现有 CLI，未执行。原始 goal SHA-256 仍为
  `9ae7b1d776856669a10eb753408c7754d015d266f8783240165450e96e4bac81`。
- 本次没有新增 SQLite/pgvector 对照、数据库测试、build、race、fuzz、soak、
  native OS 运行或外部动作。所有链接日志保留原日期/源码身份，不算本次 PASS。
  仅验证本任务三份 main 原文与 before 哈希相同，不声称其他并行工作的 main
  工作树干净。进展片段由主负责人另行审阅整合。
- 阻塞不变：未知中断、ANN V3 FAIL/HOLD 与 V4 暂停；公开恢复/明文 backup/
  WAL-tail/planner/部分 backend 候选未合入；完整物理故障 harness 有缺口；
  原生环境与外部独立数据安全/密码学/法律审查未完成。详见
  [验收矩阵](acceptance-status.md) 和[中断记录](interruption-20261010.md)。
- 下一步最小任务：审阅本次文档补丁及 before/after 哈希后按已授权流程保存；
  不以文档收尾为理由恢复受阻候选、运行受限工作或宣称全目标已完成。
