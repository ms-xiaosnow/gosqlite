> 最新有限收口采用[逐项清单](goal-finite-closure-checklist-20261012-zh.md)：4组6项剩余代码＋受阻ANN修正。A8有限八项PASS、六格物理NOT_RUN。本文以下为历史审计，不据旧标签新增无限矩阵。

> 2026-10-12 post-DB更新：JUNLINK/FINAL_DIRSYNC两格caller已实现并通过受影响普通验证，plain rollback SQL设施现九格；见[准确结果](../evidence/20261012-postdb/README.zh.md)。下表旧审计状态保留；受限物理与其余明确代码缺口不因该小片闭合。A8六格入口正在独立源审/有限验证准备，未因此算完成。

> 2026-10-12 21:06 UTC 当前状态更新：下文旧表保留审计时点。第1组第四格已提交a49并完成恢复/CI；第2组pre-DB三格已实现并通过影响验证，实际Sync故障及其余格未因此闭合。下一片是JUNLINK/FINAL_DIRSYNC两边界caller，仍在源码实现；A8六格root磁盘负面入口确认有代码缺口后source-only补齐，既有memory AEAD测试不重做。不增加新的验收义务，物理受限项目继续详细手册NOT_RUN。参阅[最新准确结果](../evidence/20261012-presync/README.zh.md)。

# 原目标有限剩余工作与完成边界

日期：2026-10-12 UTC。下面是六组代码工作和一组受阻ANN报告问题，按原goal条款归类，**不是七个迭代、功能数、进度百分比或工时估计**。具体cut-point矩阵尚未全部冻结。不能称“全部源码完成，只剩人工跑”；也不扩成全SQL／全OS／在线PG系统。

本页是既有静态审计的有界摘要，不运行测试或产生新资格。来源报告`original-goal-finite-remaining-audit-20261012T1744/REPORT.zh.md` SHA-256为`17df2de9cc5febb6e2280ff079f363ce681962163fd7936d87971be30bd75e14`；本文将其中“78992 guard FAIL”精确化为ordinary中的metadata source-contract根FAIL，避免误报原`01_guard`门（实际PASS）。

## 必须继续闭合的有限代码工作

| 分类与原目标 | 已有边界 | 完成条件 |
|---|---|---|
| 1 A1 SQL非nil prefix第四格；§11.1／M3 | adapter支持，SQL caller／ledger当前只接三格；第四格批准为下一片，尚未实现 | 同DB_PAGE#3／offset8192保留原注入非nil cause，不能报io.ErrShortWrite；独立副本恢复完整OLD，普通索引／EOF／Check／owner、邻接receipt和false-PASS反例齐全；不改shipping Commit |
| 2 A1–A4其余适用I/O／Sync／errno／错位和torn；§4.1／§11.1 | DB_SYNC fail-before不是Sync实际返回失败；generic错误不是ENOSPC，prefix不是torn-sector | 冻结已有持久化操作的有限矩阵，先映射已有断言，仅补缺caller；逐格区分OLD／恢复OLD／uncertain／ack，保留原始cause和输入，不一律“失败即回滚” |
| 3 A5–A7余下中断／恢复／发布；M3–M8相关条款、§7.2／8.3／11.1 | plain rollback五格、单页replay、三页SQL源码已有 | 其它已支持profile、扩展索引原子恢复、checkpoint／replay再中断、rekey／upgrade／backup publication有真实有限controller／caller；说明authority、完整允许state、外部ack和owner；不重做已拒链 |
| 4 A8与nonce磁盘安全负面；§8.2／8.3 | 正确密钥、AEAD vectors、marker和已有认证断言保留 | 将缺失格绑定真实磁盘路径：key／salt／tag／有限ciphertext破坏、跨页／main-WAL错配、未知版本fail-closed；重复写／重启／rollback／rekey不复用(key,nonce)，无未认证明文或日志marker泄漏 |
| 5 已声明格式／迁移边矩阵；§3.7／11.3／12.3／13.9 | format3、grams REINDEX、vector codecs、encrypted1→2／1→3／2→3和拒绝规则已有 | 先找现有实物断言，只补缺格；数据／索引／DBID／sequence／模式／未知版本与旧archive独立恢复准确；新目标Restore用合法新DBID，不造未来版本或原地downgrade |
| 6 WAL真实恢复计时工具合同；§11.3 | tail-publication elapsed已有，不能代表其它恢复profile | 对其余实际支持恢复路径绑定输入／commit账本、单调计时、bytes／pages／frames、source／tool／环境、恢复后账本／索引和owner；排除准备、普通reopen、零恢复及失败，未证冷缓存只称warm／unspecified |
| 7 ANN V3三项报告缺陷；M7／§7.2／11.3 | HNSW计数、IVF全lists成员、同query-row重评分准入FAIL，不能推断生产算法必坏 | 需要可信固定数据／真实索引路径的修正报告；当前V4执行链受阻，无新授权，不启动／换名／转CI或remote绕过 |

第2–4项仅围绕六个已有协议：plain rollback、plain exclusive/shared WAL、encrypted layout1/2/3。逐操作注明适用、不可达或明确unsupported，不构造OS×设备×故障无限组合。受支持路径不能靠文字unsupported豁免安全。每片具备实物、独立源审、准确作用域、保留失败、获准验证与精确保存即可结束；这不自动完成全部goal。

## 已有代码但仍需执行或外部资格

- 当前检查点：旧0ef／78992的ordinary metadata根FAIL；原scanner门PASS，后十九门NOT_RUN。51f63最小修正源审CLEAR，44909实际racePASS后行政pause；2054七项新编译／清单／nmPASS与明确复用映射分账，发布仍待完成。按变化重跑相关项，避免无意义完整ordinary或恢复全套重跑。
- 已有manual：internal／public Open／Connector recovery、WAL-tail publication／elapsed、有限backup、layout1→2与新目标Restore、readonly-media A/B、OS namespace／目录和三设施真实运行仍BLOCKED／NOT_RUN。tag、env、compile均不授予运行许可。普通Linux健康文件/WAL路径已有执行，不能说所有native都未运行。
- 外部层：真实平台锁／Sync／设备持久性、既有runner长期soak和固定硬件冷热基准、独立数据安全／密码学／威胁模型／许可法律／API审查仍缺实际资格。普通CI不授权新环境、费用、权限或物理活动。

## M9最低范围的校正

原M9是Neon／pg_embedding迁移与扩展类型，见goal §9.2(1–5)／:339–343、双工作流:345、M9:362和资料核验:508。**目前没有找到必须新增生产代码才能满足M9最低范围的新证据。** 原审计所称自动采集链PARTIAL是真实能力边界，不能自动升级成强制开发全自动采集器或在线执行器。

已有输入与阻断链不是空白：

1. sample collector仅HNSW／direct pg_depend且definition／profile／sql_corpus为空，必须公开coverage gaps，不能把它单独标完整。
2. `CatalogSnapshot`可接收真实、完整的Dependencies／Profiles／SQLCorpus；Inspect检查身份、维数、选项和coverage。
3. `migration/postgres/transition_render.go:34–44`将完整supplied snapshot、原definition和SQL corpus保存进evidence-only `inventory.json`，绑定catalog／dependency hash。报告不回显原SQL不等于输入丢失。
4. `transition.go:304–319`对未生成依赖计划明确blocked／manual-plan；`transition_sql.go:18–24`已有传递pg_depend、目标index／trigger／constraint／rule与数据profile只读preflight。PG16补充事实可人工从同一quiescent snapshot采集，不能只改complete标志。

最低完成条件是提供真实、范围完整的catalog／依赖定义／应用SQL／数据profile／版本，生成绑定输入的report＋inventory＋注释模板；支持片段由已有AST转换，不支持项逐项列出并阻断；人工提交和审阅依赖计划、业务切换与回滚，在授权可丢弃PG／Neon上真实复核。当前缺的是这些外部输入／审阅／运行证据，不要求新增pgx或server executor。

已有AST转换、CSV／NDJSON批事务和正常续传、quarantine／全行／PK／dim／norm／精确距离、影子列／批cursor／PG16有限整表审阅模板、两族fixture和误差／固定fixture ANN recall、half／bit／sparse支持矩阵不重做。正常续传不替代物理恢复。任意PG方言、动态SQL自动发现、任意依赖自动重建、online／dual-write／自动切换或提交后自动逆迁移、sparse IVF及所有metric组合可保持unsupported；不能借真实迁移验证重跑受限ANN V4。

Neon现行章节已有2026-10-10重新取得记录，当时章节缺失、另固定官方历史版本；本页没有重新抓网页，不声称10月12日仍相同。实际目标版本变化时应固定新资料snapshot和必要ADR。

## 停止条件

整个goal须在适用必需格、安全一致性和发布审查通过后才可标VERIFIED／生产可用。当前没有这个结论。INSERT IGNORE仍暂停；不自动新增cold/no-SHM、EOF物理恢复、默认开放恢复或新OS／泛性能工具。ANN V4、41-target新fuzz和未知／denied中断链不启动、不换名、不换环境；原固定seed普通结果不授权新campaign。

[三设施证据与复用边界](../evidence/rollback-facilities-checkpoint-20261012.md)及[私有仓库验证记录](../evidence/20261012-rollback-facilities/README.zh.md)分别记录当前执行事实。
