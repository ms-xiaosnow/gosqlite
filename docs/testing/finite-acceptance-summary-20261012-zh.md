> 2026-10-13终态更新：b066与c5的两组既有普通CI均完整SUCCESS，最新c5 ordinary于00:09:27 UTC完成。[原始日志／恢复与适用范围](../evidence/20261013-facility-ci/README.zh.md)。这补齐配置内普通CI资格，不改变下文ANN代码阻塞或物理/外部NOT_RUN结论。

# 有限六项完成记录与剩余验收

2026-10-12 23:30:52 UTC。R1–R6 是从原目标收敛的六个有限设施项，不是整个 goal 的百分比。**在现有限制下完成了这批独立可推进项；整体未验收，M10 未通过。** M10 以精确源码和依赖闭包复用已有 PASS，只补缺证据或受影响项，不要求把所有测试从头重跑。

## 已闭合的有限六项

| 项目／goal 条款 | 代码及适用证据 | 当前边界／中文步骤 |
|---|---|---|
| R1 errno；goal.md:374 | 实际 publication facade 配合 fake ops，8 个有限 errno 场景；normal/race 各127命名PASS。[证据](../evidence/20261012-publication-nonce/README.zh.md)。 | 没有真实塞满盘／改权限；不新增物理 errno caller 作为额外义务。见 rollback-fault-adapter-zh.md。 |
| R2 写错位；goal.md:356 | 唯一 index-misplace-minus64-error，8192→8128、4096字节、独立 OLD journal／恢复账本。最终13门PASS，91根race2418命名；89根normal复用，两个失败根修后235命名PASS。[证据](../evidence/20261012-rekey-misplacement/misplacement/README-zh.md)。 | 真实写错位／恢复NOT_RUN。见 rollback-fault-sql-manual-zh.md、rollback-fault-recovery-manual-zh.md。 |
| R3 checkpoint中断；goal.md:159,356,374 | 共用十格、三种authority、七个默认空观察点。12纯模型＋38AST＋3健康默认根、sourceguard、选择、compile/nm及三健康根racePASS。[证据](../evidence/20261012-checkpoint-extensions/README.zh.md)。 | 中断／退出／重开恢复NOT_RUN。见 checkpoint-restart-manual-zh.md。 |
| R4 扩展索引恢复账本；goal.md:188,200,258,261,357,360 | 与R3共用315行fixture，grams posting、HNSW graph、IVF membership、实际查询路径和Check；同批有限证据。 | 实际恢复索引一致性NOT_RUN。手册同R3；不是ANN recall基准，不触发V4。 |
| R5 rekey中断；goal.md:301,316,361,378 | 六格controller/独立probe；7AST＋8模型normal/race、54旧AST复用、2健康默认根及compile/nmPASS。[证据](../evidence/20261012-rekey-misplacement/rekey/README-zh.md)。R3复验受影响R5 guards，共享生产default新增观察点由三健康race覆盖。 | 六格真实中断／wrong-key探测／退出NOT_RUN。见 rekey-interruption-manual-zh.md，有限layout1/RawKey256合同。 |
| R6 nonce关联；goal.md:307 | 正常重开写入、rollback、checkpoint、rekey的公开domain/nonce关联，3根normal/race各3PASS。[证据](../evidence/20261012-publication-nonce/README.zh.md)。 | 两根确实运行健康Linux库，一根纯值；不证明物理中断。见 encryption-nonce-lifecycle-zh.md。 |

上列相对手册名均位于 docs/testing/。R3/R4最终受测树 ffc73262e0314f46827b7f4cee1cdac766dbd889；R2/R5已远端保存 b06606020fdbb5b73dd0e9b000773f3f02c6979d。精确CI终态在对应证据中单列，运行中不算PASS。冻结手册早期HOLD/NOT_RUN保留历史；当前source/有限验证状态以本页和日期证据覆盖，物理NOT_RUN不变。

## 1. 先修代码／报告，不能靠运行命令验收

- 已知ANN V3报告的HNSW计数、IVF全lists成员及同query-row rescore准入修正，修正源码和可信结果尚未核验闭合。详见 remaining-goal-boundary-20261012-zh.md、../acceptance-status.md 的ANN记录。这是实际代码/报告缺口，不能标成只需rerun，也不能推断生产ANN算法必坏。
- V4、41-target持续campaign和未知中断执行链维持原阻塞，不换名、换入口、换CI/环境重试。本清单不产生替代工具或扩张新设施。

## 2. 已有实现和可按规程执行的手工资格

这些路径给出版本、隔离输入、具体命令、断言、失败留存和清理条件；用户仅在其有权控制的隔离副本和真实支持环境中执行。当前没有执行授权扩展或物理PASS。

- public/Connector rollback：../public-rollback-manual-zh.md、../connector-rollback-manual-zh.md。
- A5退出/独立恢复：rollback-restart-manual-zh.md；fault/一三页恢复：rollback-fault-adapter-zh.md、rollback-fault-sql-manual-zh.md、rollback-fault-recovery-manual-zh.md。
- checkpoint/扩展索引及rekey：checkpoint-restart-manual-zh.md、rekey-interruption-manual-zh.md。
- root六格负面：encrypted-root-negative-manual-zh.md；加密升级/archive Restore：encrypted-layout1-to2-manual-zh.md；明文backup：plaintext-backup-manual.md。
- WAL尾部发布/耗时：wal-tail-publication-manual.md、../../benchmarks/wal-tail-publication/README.md；只读介质：readonly-wal-prepared-media-manual-zh.md；Darwin：darwin-inventory-manual-zh.md；其余原生/namespace入口：../manual-verification-zh.md。
- 41根/313固定seeds旧普通PASS不等于持续campaign完成。持续受阻链不能因存在命令就算获准在本平台运行。

## 3. 需要真实外部输入或独立结论

- 原生OS/FS/锁/Sync及持久介质、设备掉电资格。普通Linux健康路径已实测，foreign编译不能代表原生Windows/Darwin资格；本环境tmpfs上的fsync为volatile，不能证明掉电持久性。
- 固定同机冷热基准、长时soak：已有../../benchmarks/{transactions,grams,ann}/README.md；WAL单次有界总公开调用耗时不是cold/P95/recovery-only。ANN报告缺口仍归第1类。
- M9真实quiescent snapshot的完整catalog、definitions、应用SQL/profiles、目标版本资料、人工计划及获准disposable PG/Neon复核：../migration-transition.md、../migration-transition-cli.md、../migration-pgembedding.md。不新增在线collector/executor义务，不冒充真实迁移完成。
- 独立安全/密码学、一致性、来源许可和API发布审查：../security.md、../upstream.md、../../THIRD_PARTY_NOTICES.md及goal.md:400。源码和自测不能代替外部结论。

只有对应代码修正、实际适用测试和外部资格逐条取得证据后，才可更新总体验收结论。本记录是开发检查点，不创建Release或宣称生产可用。
