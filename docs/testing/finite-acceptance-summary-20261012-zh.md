# 待手工验收清单与有限开发记录

2026-10-13 UTC 更新。**现有待手工验收共17组：13组核心、82个固定场景，另4组原生／namespace规程；实际手工资格全部 NOT_RUN。整体未验收，M10未通过。** 这些是既有入口的有限清点，不是整个 goal 的剩余总数、完成率或生产可用声明。

本页是统一交接入口：先区分真实代码／报告缺口、受限执行、环境／输入不足和外部审查，再按下方各组准备、执行、核验、保全。只供用户在自己有权控制、另行核准的隔离环境中自行测试；记录命令不授予助手新的运行权限。ANN V4、41-target持续campaign和未知中断链不在本页的可执行清单内。

## 当前已取得的资格与适用边界

- R1–R6六个有限设施项于2026-10-12 23:30:52 UTC收口，原表完整保留如下。它们不代表整个目标已完成；M10按精确源码和依赖闭包复用已有PASS，只补缺证据或受影响项，不把“重跑全套”作为默认要求。
- b066与c5的既有配置内普通CI均完整SUCCESS，c5最后任务于2026-10-13 00:09:27 UTC完成。[原始日志、精确身份及恢复证明](../evidence/20261013-facility-ci/README.zh.md)。
- docs-only提交 `fd069d7a30b2e0ca8bdff67ca54c926aa118fc51` 的既有CI [38339706432](https://github.com/ms-xiaosnow/gosqlite/actions/runs/38339706432) 已完整SUCCESS：ordinary于03:30:31 UTC、oracle于03:09:22 UTC完成。它与c5的生产／测试／模块／工作流代码相同；**该CI不覆盖本批新增的PG／性能源码。** [本批精确证据](../evidence/20261013-goal-closeout/README.zh.md)。
- 本次另外补齐两项有限编码／测试：goal所列PG风格向量索引DDL的精确语法适配，以及离线性能报告准入／比较器与普通CI离线单元hook。PG聚焦normal为8个纯解析根100命名PASS＋2个健康根4命名PASS，race合计104命名PASS，均零FAIL／SKIP，sourceguard PASS；性能57个纯fixture测试于03:37:08 UTC PASS、零FAIL／SKIP，检查脚本语法PASS。本批聚焦证据绑定精确文件清单；随后自动CI须按最终提交的实际终态核对，本页不提前声称新CI通过。这两项也不是“整个goal最后且全部完成”的证明。离线fixture通过不等于真实性能测量或性能CI部署。[源码、精确命令和结果](../evidence/20261013-goal-closeout/README.zh.md)。

- 2026-10-13补记：40e287的CI [38346611629](https://github.com/ms-xiaosnow/gosqlite/actions/runs/38346611629) 双SUCCESS。随后ANN离线修正六门于06:11:06 UTC完成，normal/race各467命名PASS；仅报告自洽，不提升手工或M10资格。[新证据](../evidence/20261013-ann-report-check/README.zh.md)。

## 2026-10-13 有界收尾复核与离线性能 CI

- 原目标 §11.3 的性能回归 CI 接线已补齐：只手动触发的 [offline-performance-evidence](../../.github/workflows/performance-evidence.yml) 读取同一提交下固定目录中的已审阅证据，调用既有比较器并保留七类状态／退出码。它不采集性能、启动数据库或获取外部数据。
- 新入口独立源审通过，17 个纯 fixture 测试、Python AST 与 shell 语法检查 PASS；原 57 个比较器测试及未变 Go 代码的资格明确复用。[精确增量证据](../evidence/20261013-performance-ci/README.zh.md)。新的普通 CI 终态以该提交的 Actions 记录为准，不在提交前预填。
- 有界复核未发现除此接线外可独立补完的新增必需代码项。原目标允许按兼容矩阵分阶段保留未支持 SQL；M9 的完整真实 snapshot／依赖／应用 SQL 可由操作者提供，不要求新增在线迁移执行器。
- 原有 ANN 测量工具已实现固定数据、真实索引、recall@1/10/100 与建索引耗时；本次修复的 schema3 离线校验是额外报告合同，原工具输出不会冒充 schema3。更强格式的 native 采集集成不是原 goal 的新增义务。
- 仍需操作者提供并执行下列手工／真实环境资格、受控测量输入、PG／Neon 复核与独立审查。没有真实证据包时，新的离线性能任务是 **NOT_RUN**；fixture PASS、来源声明及绿色普通 CI 都不能证明真实性能或整体 M10。

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

## 仍有代码／报告缺口或中断边界的项目

| 待办 | 真实原因与完成条件 | 当前处理 |
|---|---|---|
| ANN V3三项报告准入修正 | 已以离线reportcheck＋stdin CLI修复HNSW计数守恒、IVF全lists覆盖、同query-row exact／rescore距离一致；29根467命名normal/race各PASS，vet/build/源码guard/格式PASS。旧V3 FAIL保留。 | 三项代码与有限回归已闭合；真实报告来源与性能／recall未验收；schema3 native采集未集成且不是原goal新增义务，旧聚合报告不冒充schema3。见[可调用入口](../../benchmarks/ann/reportcheck/README.md)、[精确证据](../evidence/20261013-ann-report-check/README.zh.md)。 |
| ANN V4及未知整轮安全中断 | 已有整轮安全标记／中断事实，但不能确定精确触发操作和目标；未找到足以证明某个具体源码编辑曾被拒绝的记录。缺失精确记录既不等于从未拒绝，也不是恢复链路的授权。 | 保留UNKNOWN，原整链不重启；2026-10-13用户新请求下限定三项离线修正已完成，不能把旧全面暂停误称所有源码永久禁止。见[中断事实与历史失败](../interruption-20261010.md)、[逐项限制溯源](../restriction-provenance-20261011.md)。本页不提供这些链路的启动命令。 |
| 41-target持续fuzz campaign | 旧41根／313个固定seed普通PASS不等于持续campaign；被中断活动没有可采信的新运行结果。 | 原状态保留，不重新发起，也不把固定seed结果升级为campaign PASS。 |

下列手工组有现成入口和规程，但**不是全部因为平台安全限制而未跑**。H01–H09及H12涉及已记录的物理恢复／故障／退出／磁盘负面限制，N01/N02涉及namespace限制；H10/H11/H13和N03/N04主要缺真实支持环境、介质、keeper或目标OS。每组的精确原因在表中单列。组内control计入既有完整合同，不能据此声称每个健康control曾单独被拒绝。

## 手工验收交接清单

核对日期：2026-10-13 UTC；手工入口清点基于已保存的 `fd069d7a30b2e0ca8bdff67ca54c926aa118fc51` 及本次只读源码核对。该commit不代替未来实际编译的完整source tree／manifest；当前新增PG和性能代码的资格另见上节。**17组实际手工资格均为NOT_RUN。**

### 计数和使用方法

| 编号 | 验收组 | 有限计数 | 当前状态与具体未跑原因 |
|---|---|---:|---|
| H01 | public Open rollback | 4场景 | NOT_RUN；真实Open恢复受物理恢复执行限制，包含完整合同的健康／拒绝control。 |
| H02 | Connector rollback | 4场景 | NOT_RUN；真实Connect恢复受物理恢复执行限制，control没有另行独立拒绝结论。 |
| H03 | 内部确定性 rollback recovery | 13场景 | NOT_RUN；实际RecoverRollback及恢复负例受物理恢复限制；非仅缺编译。 |
| H04 | rollback退出与独立恢复 | 5场景 | NOT_RUN；受控进程退出及独立物理恢复受限，且需绑定真实输入。control随完整合同保留。 |
| H05 | 三页SQL fault与索引账本 | 10场景 | NOT_RUN；真实故障注入／恢复受限，且需绑定真实输入；健康control并非单独被拒绝。 |
| H06 | 单页fault与恢复副本 | 3场景 | NOT_RUN；真实fault／恢复受限，且需绑定真实输入；control不单独归为拒绝。 |
| H07 | checkpoint中断与扩展索引账本 | 10场景 | NOT_RUN；checkpoint切点退出／重开恢复受限，且需真实输入。三个control是同组对照。 |
| H08 | rekey中断 | 6场景 | NOT_RUN；切点退出、wrong-key探测及中断rekey执行受限，且需真实输入。control没有独立拒绝记录。 |
| H09 | 加密root负面 | 6场景 | NOT_RUN；磁盘wrong-key／salt／ciphertext／tag篡改及版本负面执行受限；control是同组健康对照。 |
| H10 | layout1→2与archive Restore | 3页尺寸场景 | NOT_RUN；需要真实受支持介质上的升级／独立archive Restore资格，不能统称为安全拒绝。 |
| H11 | 明文Backup | 6入口、10叶场景 | NOT_RUN；缺严格Linux ext-family、unique-mount／openat2等原生支持环境及每入口真实manifest。 |
| H12 | WAL尾部发布及elapsed | 4场景 | NOT_RUN；真实物理尾部恢复／发布受限，另需符合合同的持久设备profile；nil-default对照不独立视作拒绝。 |
| H13 | 预备真只读WAL介质 | 2 profiles × 2 cases＝4 | NOT_RUN；缺用户提供的真实只读介质、静止keeper、manifest和新鲜回执；harness不会代造。 |
| N01 | 明文namespace | 2入口 | NOT_RUN；symlink重定向／文件替换的namespace干扰执行受限。 |
| N02 | Windows/Darwin raw namespace | 2入口、3个明确OS/arch资格 | NOT_RUN；原生namespace执行受限，且需真实目标OS；交叉编译不补运行资格。 |
| N03 | Darwin managed-directory inventory | 1入口、11个断言 | NOT_RUN；缺真实Darwin／合格APFS或HFS环境，需固定本机编译和输入；不是故障／篡改测试。 |
| N04 | Windows shared-rekey普通路径 | 3入口 | NOT_RUN；缺真实Windows/amd64与合格NTFS环境；不能把普通Linux或PE编译当作原生运行。 |

核心场景合计：4＋4＋13＋5＋10＋3＋10＋6＋6＋3＋10＋4＋4＝82。原生四组的入口/断言/目标资格不混加。checkpoint与扩展索引共一组；elapsed不另计；Backup六入口取十叶；rekey可选archive不算第七case。adapter十四site、已完成R1/R6普通测试均不加入。

除N03明确校正的启动形状外，命令摘自既有手册；变量、包装函数、隔离目录及输入须先按链接原规程准备。canonical JSON／compile receipt／keeper未真实取得即PREPARATION_REQUIRED，不填虚构hash或成功布尔值。旧手册SOURCE_ONLY／HOLD／PENDING是各自时点的历史，不能覆盖现有精确源码已取得的普通／源审／编译资格；实际手工仍NOT_RUN。

### 所有组共同的准备和记录

- 只用有权控制的非生产可丢弃副本，固定commit/tree、全文件hash/mode、工具/依赖；不拿旧binary或邻近PASS替代。仅按变化复验。
- 固定Go1.27.2、CGO_ENABLED=0及核验缓存；SQLite3.54.0只供明确oracle入口，版本/来源见[主手册4.1](../manual-verification-zh.md)、[upstream](../upstream.md)。不自动下载或扩race矩阵。
- 各组分别按真实合同准备空已存或尚不存在的目录、UID/0700/symlink/FS/容量；不要全局套TMPDIR/CWD/缓存。先审入口和init闭包，确认selection/compile来源；tag/env不代替执行资格。
- 保存输入JSON、source/tool/binary、OS/arch/FS/设备、argv/env、UTC起止、stdout/stderr/自然exit、逐case/owner/Wait收据；不记真实秘密。
- FAIL/SKIP/零匹配/unknown均非PASS；异常停余项，不自动重试Close/Commit/恢复、降断言、换环境或kill。仍活进程和管道保留，另行处置。
- 成败均先保全；确认所有owner/进程终结且归档可读后，操作者才清理准确新建目录。**不删除原库、唯一备份或未知目录。** 旧t.TempDir入口不能保证失败实物保留。

记录模板：`source/binary；OS/FS；case/selector；UTC起止；exit；逐名结果；证据目录；未决owner；PASS/FAIL/BLOCKED/NOT_RUN及原因`。有限通过不等于掉电持久性、安全审计或M10。

### H01 public Open rollback 四场景

已有入口 `TestManualPublicRollbackRecovery`，构建门 `manual_recovery && linux`。四场景是nil-default保留输入、opt-in恢复后只读重开、journal-limit拒绝且保留、NoCreate保持缺失。见[完整规程1—6节](../public-rollback-manual-zh.md)，计数依据该文件7—12行；[源码](../../sqlite/rollback_recovery_manual_linux_test.go)。

前置：支持域内的Linux、新空稳定fixture、完整source/binary资格。原规程3—4节给出隔离、default/oracle排除、manual选择、compile-only/nm。不要用全仓manual go test。原启动核心：

```sh
GOSQLITE_MANUAL_RECOVERY=independently_authorized \
GOSQLITE_MANUAL_RECOVERY_DIR="$FIXTURES" \
"$EVIDENCE/public-recovery.test" \
  -test.run '^TestManualPublicRollbackRecovery$' -test.count=1 -test.v -test.timeout=0 \
  > "$EVIDENCE/runtime.log" 2>&1
```

PASS：exit0、顶层及四子项全PASS；原字节、code/cause、journal、NoCreate、Close/pending断言齐全。保留三套原输入、input-sha256.json和前后清单；Close失败不重开/cleanup重试。

### H02 Connector rollback 四场景

tag同H01，独立env/入口；还验证Connector捕获Options、真实Prepare/Query/Rows及reconnect。见[规程1—6节](../connector-rollback-manual-zh.md)29—48行、[源码](../../sqlite/connector_rollback_recovery_manual_linux_test.go)。

按原规程3—4节准备独立新空fixture和compile/nm。原命令：

```sh
GOSQLITE_MANUAL_CONNECTOR_RECOVERY=independently_authorized \
GOSQLITE_MANUAL_CONNECTOR_RECOVERY_DIR="$FIXTURES" \
"$EVIDENCE/connector-recovery.test" \
  -test.run '^TestManualConnectorRollbackRecovery$' -test.count=1 \
  -test.parallel=1 -test.v -test.timeout=0 \
  > "$EVIDENCE/runtime.log" 2>&1
```

PASS：四子项、Options快照、精确类型/cause、EOF退休、NoCreate/字节/pending全合格、exit0。保留三套原输入及manifest；失败停，不用懒连接代替Connect，不自动回退恢复。

### H03 内部 rollback recovery 十三场景

`internal/pager`的 `TestManualRollbackRecovery` 与H01/H02不同；构建门 `manual_recovery && linux`。2个成功/空extent、9个preserve拒绝、2个BUSY；完整名称见源码。依据[主手册12.1](../manual-verification-zh.md)886—921行和[源码](../../internal/pager/recovery_manual_test.go)100、126、143、179行。

原规程在已定义 `capture` 且完成隔离前置后使用：

```sh
capture manual-recovery-run env GOSQLITE_MANUAL_RECOVERY=independently_authorized \
  "$RUNROOT/bin/manual-recovery.test" \
  -test.run '^TestManualRollbackRecovery$' -test.v || exit 1
```

PASS：13子项、两页恢复/截断/journal移除/幂等成立，拒绝和BUSY输入不变。multiple-segments仅限该fixture。旧入口用t.TempDir，若依赖失败DB留存且无保存办法则该用途BLOCKED；保留日志/身份，不推断进程退出或掉电资格。

### H04 rollback退出与独立恢复 五场景

需真实输入后才能启动。仅Linux plain rollback，4096页/sector配置、三页表/普通B-tree；case是control、before-jcreate、after-table-page、after-junlink、acknowledged-exit。见[规程](rollback-restart-manual-zh.md)7—92行和[controller源码](../../sqlite/rollback_restart_manual_linux_test.go)。

构建tags为manual_fault_rollback,manual_restart_rollback。仅人工启动controller，不直接运行writer/recovery。原手册未给完整shell清单；源码203—214行固定依次：`-test.run=^TestManualRollbackRestartController$ -test.count=1 -test.shuffle=off -test.parallel=1 -test.timeout=0 -test.cpu=1 -test.v=true`。

必须提供两个gate `GOSQLITE_MANUAL_FAULT_ROLLBACK=independently_authorized`、`GOSQLITE_MANUAL_RESTART_ROLLBACK=independently_authorized`；以及 `GOSQLITE_RESTART_PRIVATE_ROOT_CONFIRMED=private_stable_namespace_retained`、`GOSQLITE_RESTART_ROLE=1`、`GOSQLITE_RESTART_ROOT`、`GOSQLITE_RESTART_BINDING`、`GOSQLITE_RESTART_BINDING_SHA256`。ROOT预建空0700；Controller=1依据[角色枚举](../../internal/testkit/rollbackrestart/contract.go)74—80行。

输入需canonical rollback-restart-binding-v1、完整manifest、真实rollback-restart-compile-v1收据，绑定SourceTree/Files/Tool/Binary/FS及原声明。每case新root；详细字段见原规程“独立授权与未来输入资格”，不能填假编译结果。

PASS需实际Start/Wait、目标writer86/control与recovery0、协议/challenge/ACK/七查询齐全。仅after-table-page在新replay恢复三页OLD，其余按表OLD/NEW。保留五套目录/帧/收据；unknown停止恢复，不signal/再次Wait/修改原件。退出86或无ACK均不能单独判定成功/OLD。

### H05 三页SQL fault 十场景

需真实输入后才能启动。构建 `linux && manual_fault_rollback`，唯一入口 `TestManualRollbackFaultSQLLedger`。十格：control、两prefix、DB Sync、三个pre-DB、unlink、final dirsync、misplace；准确case字符串见所引表。以[规程顶部R2及旧九格表](rollback-fault-sql-manual-zh.md)8、76—89行为准，不把旧“九格”当当前总数。

无独立通用runner。[源码](../../sqlite/rollback_fault_sql_manual_linux_test.go)248—310行要求gate GOSQLITE_MANUAL_FAULT_ROLLBACK=independently_authorized；GOSQLITE_ROLLBACK_SQL_PRIVATE_ROOT_CONFIRMED=private_stable_namespace_retained；另须GOSQLITE_ROLLBACK_SQL_ROOT、GOSQLITE_ROLLBACK_SQL_BINDING、GOSQLITE_ROLLBACK_SQL_BINDING_SHA256。ROOT预建空0700；Case在binding。

准确argv：`-test.run=^TestManualRollbackFaultSQLLedger$ -test.count=1 -test.shuffle=off -test.parallel=1 -test.timeout=0 -test.cpu=1 -test.v=true`。全部唯一equals-form。binding/source inventory/compile receipt按源码manualRollbackSQLBinding/Compile填真实证据，不能套用单页组schema。

PASS：一次公开Commit、原cause/transcript、七typed查询、真实索引/扫描/EOF/Check及完整OLD/NEW账本。两prefix、db-sync、junlink、misplace五格仅恢复新replay；pre-DB三格OLD/零恢复；control及final-dirsync为NEW/零恢复，后者保留CommitUncertain。错位请求8192/4096、实际8128/4096，恢复OLD。保留全部原件/副本；fail-before不冒充OS Sync失败。

### H06 单页fault与恢复副本 三场景

需真实输入后才能启动。同fault构建tag，包是 `internal/pager`；case为control、page-prefix-64、db-sync-before。见[完整规程2—6节](rollback-fault-recovery-manual-zh.md)及[源码](../../internal/pager/rollback_fault_recovery_manual_linux_test.go)。

gate同H05；其余变量前缀不同：`GOSQLITE_ROLLBACK_FAULT_PRIVATE_ROOT_CONFIRMED=private_stable_namespace_retained`、`GOSQLITE_ROLLBACK_FAULT_ROOT`、`GOSQLITE_ROLLBACK_FAULT_BINDING`、`GOSQLITE_ROLLBACK_FAULT_BINDING_SHA256`。case从binding读取。原规程准确参数形状：

```text
<独立审阅过且获得运行许可的准确二进制> \
  -test.run=^TestManualRollbackFaultRecovery$ \
  -test.count=1 -test.shuffle=off -test.parallel=1 -test.timeout=0 -test.cpu=1 -test.v=true
```

ROOT空0700，binding在root外。第4节有完整JSON和rollback-fault-compile-v1，hash/Files必须真实。两fault恰一次RecoverRollback、RestoredPages=OriginalPages=1、全4096字节OLD且replay journal移除；control NEW/零恢复。保留原trial/journal和独立副本，unknown不重试。

### H07 checkpoint与扩展索引 十场景

需真实输入后才能启动。一个controller、三authority、315行共用fixture，不另乘grams/HNSW/IVF。exclusive三格、shared三格、encrypted四格，准确case字符串见原规程7—18行。见[规程固定范围及一—六节](checkpoint-restart-manual-zh.md)和[源码](../../sqlite/checkpoint_restart_manual_linux_test.go)。

构建manual_checkpoint_restart；原规程2—4节给出compile-only、manifest/receipt及binding的完整生成Python。实际编译成功后才能填收据。RUN尚不存在、父目录0700，binary inode与/proc/self/exe必须绑定。真实输入准备后原命令：

```sh
env -i GOFLAGS= \
 GOSQLITE_MANUAL_CHECKPOINT_RESTART=independently_authorized \
 GOSQLITE_CHECKPOINT_ROLE=controller \
 GOSQLITE_CHECKPOINT_BINDING="$BINDING" \
 GOSQLITE_CHECKPOINT_BINDING_SHA256="$BINDING_SHA256" \
 "$OUT/checkpoint-restart.test" \
 '-test.run=^TestManualCheckpointRestartController$' \
 -test.count=1 -test.parallel=1 -test.shuffle=off -test.timeout=0
```

只用固定flags，新case/new binding/new RUN，不加verbose。PASS核对独立committed-head C、13完整SQL流、96posting/68frequency、180 HNSW nodes、128 IVF memberships/4 centroids、六查询和三扩展Check；完整计数见第5节。writer切点86、control/probe0及Wait/EOF可信；前三套原件不变，仅第四套probe观察。保留全目录；PhysicalValidated和FullModelEvidenceEvaluated不改true，unknown不kill。

### H08 rekey中断 六场景

需真实输入后才能启动。仅Linux snapshot/layout1、RawKey256、4096逻辑页、三页表/普通索引。六case：control、rename前/后、dirsync后、返回无ack、acknowledged；准确字符串见原规程10—17行。见[规程固定范围/人工步骤](rekey-interruption-manual-zh.md)及[源码](../../sqlite/rekey_interruption_manual_linux_test.go)。

构建tag `manual_rekey_interruption`；人工只启动 `TestManualRekeyInterruptionController`。准确源码argv为 `-test.run=^TestManualRekeyInterruptionController$ -test.count=1 -test.timeout=0s -test.parallel=1 -test.shuffle=off -test.v=true`（源码383行）；保留这个顺序。需要 `GOFLAGS`为空、`GOSQLITE_MANUAL_REKEY_INTERRUPTION=independently_authorized`、`GOSQLITE_REKEY_ROLE=controller`及 `GOSQLITE_REKEY_BINDING`/`GOSQLITE_REKEY_BINDING_SHA256`。challenge由父controller给子进程，不能人工启动writer/probe。

原规程3—5列出canonical manifest、rekey-interruption-compile-v1、rekey-interruption-binding-v1全部字段。Root尚不存在、父目录0700，所有hash来自真实来源；control可选ArchiveControl不另算case。

PASS：退出/协议成立，OLD/NEW两独立ReadOnly/NoCreate probe恰一成一authentication失败；成功方完整SQL/index/Check及DBID/generation正确。before-rename选OLD，其余NEW。保留family/nonce、原件/probes/帧/ACK/Wait；不换key重试或fallback。unknown等待，不把86当掉电或安全擦除证明。

### H09 加密root六项负面

构建tag `manual_encrypted_root_negative`，入口 `TestManualEncryptedRootNegative`；control/wrong-key/root-salt/metadata-ciphertext/metadata-tag/unknown-major。见[完整规程](encrypted-root-negative-manual-zh.md)及[源码](../../sqlite/encrypted_root_negative_manual_linux_test.go)。

非root、预建0700稳定fixture、source/binary真实SHA。原规程前两步完成manifest/compile-only，第三步完整定义run_root_case；这是手册包装器，须连定义使用。六调用逐项核验：

```bash
run_root_case control || exit $?
```

其余五个参数按原第三步逐项用wrong-key、root-salt、metadata-ciphertext、metadata-tag、unknown-major；每次独立进程，不自动串跑。

每条后人工核对再决定下一条，不整块自动串跑。包装器使用五个 `GOSQLITE_MANUAL_ENCRYPTED_ROOT_*`变量：NEGATIVE=`independently_authorized`、CASE、ROOT、NAMESPACE=`private-stable-no-alias-no-other-process`、BINARY_SHA256；唯一六flag是精确run、count=1、parallel=1、shuffle=off、timeout=0s、v=true。

PASS：每case恰一ROOT_AUTH_RECEIPT/OBSERVED及exit0；四认证反例DB=nil/CodeNotADB26/authentication，unknown-major unsupported/CodeError1且不取key；control Verify/Check/三行完整。两份各七成员无alias且仅选定根突变、owners退休无pending。保留六份日志/原件，不重开或修复失败候选。

### H10 加密layout1到2与archive恢复 三页尺寸

唯一 `TestManualEncryptedSnapshotToWALUpgrade`，构建 `manual_format_upgrade`；固定512/4096/65536三页尺寸。新健康layout1升级layout2，升级前portable archive恢复到另一个新snapshot/layout1目标，不是原地downgrade。见[规程1—6节](encrypted-layout1-to2-manual-zh.md)及[源码](../../sqlite/encryption_upgrade_manual_linux_test.go)。

先核验非root、0700 known root、sourceDigest算法与binary SHA、稳定支持的Linux存储和无其他访问者；原规程第3节列出实际compile-only。正确密钥仅用合成测试材料。原启动命令：

```sh
env GOFLAGS= \
  GOSQLITE_MANUAL_FORMAT_UPGRADE=independently_authorized \
  GOSQLITE_MANUAL_FORMAT_UPGRADE_APPROVAL="$APPROVAL_REFERENCE" \
  GOSQLITE_MANUAL_FORMAT_UPGRADE_NAMESPACE=private-stable-no-alias-no-other-process \
  GOSQLITE_MANUAL_FORMAT_UPGRADE_SOURCE_ROOT="$REVIEWED_SOURCE" \
  GOSQLITE_MANUAL_FORMAT_UPGRADE_SOURCE_SHA256="$SOURCE_SHA256" \
  GOSQLITE_MANUAL_FORMAT_UPGRADE_BINARY_SHA256="$BINARY_SHA256" \
  GOSQLITE_MANUAL_FORMAT_UPGRADE_ROOT="$KNOWN_PRIVATE_ROOT" \
  "$EVIDENCE/format-upgrade.test" \
  -test.run='^TestManualEncryptedSnapshotToWALUpgrade$' \
  -test.count=1 -test.timeout=0 -test.parallel=1 -test.shuffle=off -test.v \
  >"$EVIDENCE/native.stdout" 2>"$EVIDENCE/native.stderr"
```

PASS：三case完整行/Verify/Check、模式/sequence、新Restore DBID、无alias、immutable/归档不变及旧ciphertext保留，owner退休；完成行须与详细收据一致。失败停余项，保留source/archive/restored；known root和原件不动。

### H11 明文Backup 六入口十叶场景

需真实输入后才能启动。见[规程1—7节](plaintext-backup-manual.md)及其中三个真实源码文件。五个非oracle入口用 `manual_backup_publication`，第六另加 `oracle`；都仅Linux。

| 入口 | 叶数 | 编译产物 |
|---|---:|---|
| TestPlaintextBackupOrdinaryRoundTrip | 3 | sqlite-backup-manual.test |
| TestPlaintextBackupExistingDestinationPreflight | 1 | 同上 |
| TestPlaintextBackupPinnedReadOnlyAndConcurrentReaders | 1 | 同上 |
| TestPlaintextBackupEmptyAndLazyReadOnly | 1 | 同上 |
| TestCLIPlainBackupOrdinaryRoundTrip | 1 | cli-backup-manual.test |
| TestPlaintextBackupOracleOrdinaryInteroperability | 3 | sqlite-backup-manual-oracle.test |

每入口独立ROOT，work/tmp/fixtures/evidence已存在、0700、同设备/unique mount。需ext-family/statx能力、256MiB/1000inode，禁tmpfs/overlay；oracle另需真实合格/tmp/...及固定SQLite3.54身份/startup隔离。十叶跨512/4096/65536、12次Backup；oracle另七次启动。

第4节给出input-manifest/digest和source/toolchain/module inventory，须人工填真实值；无prepare-manifest脚本。第5节第一入口原命令：

```sh
R="$ROOT_1"
(cd "$R/work" && /usr/bin/env -i LC_ALL=C LANG=C TZ=UTC GOMAXPROCS=2 \
  GOSQLITE_MANUAL_BACKUP_PUBLICATION=1 GOSQLITE_MANUAL_BACKUP_ROOT="$R" \
  GOTMPDIR="$R/tmp" TMPDIR="$R/tmp" \
  "$BUILD/sqlite-backup-manual.test" \
  -test.run '^TestPlaintextBackupOrdinaryRoundTrip$' \
  -test.v -test.count=1 -test.failfast -test.parallel=1 -test.shuffle=off -test.timeout=0)
```

其余入口只能按表换对应binary/精确selector及独立root；第六严格使用原规程oracle命令，加入 `SQLITE_ORACLE="$ORACLE"`，不扩大selector。运行前明确外部stdout/stderr接收、预算和观察者。

PASS：十叶完整SQL值/Schema/Check/source字节/sidecar、snapshot/并发、empty/lazy、CLI/oracle断言，owner结束且exit0。保留observed-runtime.json/input-plan/events及源/目标/candidate；unknown会冻结，禁止重做Backup/Close；确认退出归档后仅清理准确case目录。

### H12 WAL尾部发布及elapsed 四场景

需真实输入后才能启动。`manual_wal_publication && linux`、`TestManualPublicWALTailPublication`。四case是nil-default三API拒绝、Open发布、OpenContext发布、Connector发布；elapsed就在同一入口，不能重复计数。见[规程2—6节](wal-tail-publication-manual.md)和[耗时范围](../../benchmarks/wal-tail-publication/README.md)。

需真实ext2/3/4持久介质、statx unique mount/Openat2、4096不小于设备扇区；禁tmpfs/overlay/network/volatile/nobarrier。fixture/source≥2GiB/10000inode、实际Go temp≥1GiB/10000inode，新空0700 ASCII根。第3节有profile生成；声明字符串不证明硬件。

原启动命令：

```sh
GOSQLITE_MANUAL_WAL_PUBLICATION=independently_authorized \
GOSQLITE_MANUAL_WAL_PUBLICATION_DIR="$FIXTURES" \
GOSQLITE_MANUAL_WAL_PUBLICATION_PROFILE="$PROFILE" \
"$EVIDENCE/wal-public-manual.test" \
  -test.run '^TestManualPublicWALTailPublication$' -test.count=1 -test.parallel=1 \
  -test.shuffle=off -test.v -test.timeout=0 \
  > "$EVIDENCE/runtime.stdout" 2> "$EVIDENCE/runtime.stderr"
```

PASS：四子项、三拒绝/三实际API发布、旧字节/inode保持、新WAL有效prefix、完整账本/INSERT/reopen/owner退休。elapsed六attempt/唯一REPORT，三个入口各n=1，不是cold/P95/recovery-only。保存profile/mountinfo/manifest/日志/fixture；unknown不重开或修复。

### H13 预备真只读WAL 两profile四场景

需真实输入后才能启动。`manual_readonly_wal && linux`、`TestManualReadOnlyWALPreparedMedia`。A=`nonprivileged-denied-v1`，B=`readonly-mount-v1`，各固定keep-false再keep-true；两profile分别形成独立结果，不在进程内切换。见[规程1—6节](readonly-wal-prepared-media-manual-zh.md)，源码[profile准入](../../sqlite/readonly_wal_manual_contract_test.go)237行及[真实双调用](../../sqlite/readonly_wal_manual_linux_test.go)1324、1327行。

操作者须另供健康三文件live-ro-wal-4096-v1、已持DMS/readmark的静止keeper、manifest和四新鲜回执。harness不创建keeper/mount/chmod/修复。A实际W_OK=EACCES；B真实ST_RDONLY及mountinfo ro；ext4/XFS、非root、零capability，源/证据/控制位置隔离。

原规程第2—3节给出manifest和keeper回执字段。三个输入变量是 `GOSQLITE_MANUAL_READONLY_WAL=independently_authorized`、`GOSQLITE_MANUAL_READONLY_WAL_MANIFEST`、`GOSQLITE_MANUAL_READONLY_WAL_MANIFEST_SHA256`。第6节刻意不提供暗示已有fixture的shell启动脚本；准确argv及顺序是：`-test.run=^TestManualReadOnlyWALPreparedMedia$ -test.v=true -test.count=1 -test.failfast=true -test.parallel=1 -test.shuffle=off -test.timeout=0`。

PASS：两case读API/提前Close/两种Tx/DB.Close一次退休；源身份/hash/mtime/ctime不变、UID/FS与四回执完整，无pending。保留manifest/events/介质，不重试或擅自释放keeper。

### N01 明文namespace 两入口

状态：NOT_RUN，独立namespace资格。`manual_namespace`；两个入口为 `TestNativeConnectionKeepsResolvedSymlink`、`TestNativeTransactionDetectsReplacedValidFile`。见[主手册12.2](../manual-verification-zh.md)923—945行和[源码](../../sqlite/write_namespace_manual_test.go)。

用操作者自有可丢弃副本、准确平台行为和已编译的sqlite测试binary；已定义capture后原运行形状：

```sh
capture manual-namespace-run env GOSQLITE_MANUAL_NAMESPACE=independently_authorized \
  "$RUNROOT/bin/manual-namespace.test" \
  -test.run '^(TestNativeConnectionKeepsResolvedSymlink|TestNativeTransactionDetectsReplacedValidFile)$' -test.v || exit 1
```

PASS：retarget后旧连接写first且second不变；文件replacement后Commit ErrChanged、Rollback正常、替代行不变。gate缺失Skip不算PASS。临时fixture不能假定失败留存；保留日志/source/平台，不扩为敌对namespace全覆盖。

### N02 Windows和Darwin raw namespace 两入口

构建tag `manual_namespace`；Windows/amd64为 `TestWindowsEncryptedRelativePublicationAndRetainedRoot`，Darwin/amd64及Darwin/arm64为 `TestDarwinEncryptedRelativePublicationAndParentLifetime`。这是2入口、3个明确目标资格，不能乘任意FS/profile。见[主手册19.1—19.5](../manual-verification-zh.md)，1255—1258、1464—1468行。

真实OS/arch、原生Go1.27.2，交叉编译/WSL不算运行。主手册19.2给出capture/TARGET/TEST/BINARY与隔离，19.3给出compile/nm。记录OS/卷/持久性限制；原命令：

```sh
capture namespace-native-run env GOSQLITE_MANUAL_NAMESPACE=independently_authorized \
  "$BINARY" -test.run "^${TEST}$" -test.v -test.count=1 -test.parallel=1 -test.timeout=0 || exit 1
```

PASS：准确根一次RUN/PASS、零Skip/exit0、inventory/发布/旧新字节成立；Darwin parent关闭后child仍有效及children/raw清空。普通first/after字节不代表加密容器；部分defer Close未断言。保存逐目标日志/身份，临时fixture留存不可假定。

### N03 Darwin managed-directory inventory 一个入口十一断言

状态：NOT_RUN；缺真实Darwin资格环境，输入／编译未齐时另记PREPARATION_REQUIRED。`darwin && manual_darwin_inventory`、`TestManualDarwinUnicodeInventory`；真实Darwin amd64/arm64、Go1.27.2、操作者私有0700绝对无symlink根、本地APFS或HFS subtype0—3。见[专用规程](darwin-inventory-manual-zh.md)第2—8节及[当前源码](../../internal/encstore/inventory_manual_darwin_test.go)。

源码检查三项env：`GOSQLITE_MANUAL_DARWIN_INVENTORY=1`、显式ROOT、外部核验的40位小写十六进制SOURCE_TREE；并检查本机OS／Go版本／arch、真实根owner／mode／FS和初始group／lease为零。**该源码没有检查argv timeout，也没有要求TMPDIR。** 原规程第7节的2m／TMPDIR运行块与其后继集成说明不同；这里明确提供校正后的调用，而不把旧模板称作当前精确命令。默认临时目录和已核验缓存的选择是运行计划，不能写成harness源码门。

启动前必须单独完成以下准备：

1. 固定完整已审阅源码、真实本机Go1.27.2、已核验离线依赖及实际Go缓存／GOPATH／module cache，不下载、不换工具、不改既有缓存。先核对GOHOSTOS=darwin、GOHOSTARCH与目标相同；交叉编译不够。
2. 对同一源码核对默认与manual选择、适用source guard，并以 `-c -tags=manual_darwin_inventory` 只编译 `./internal/encstore` 得到 `$EVIDENCE/manual.test`。核对唯一入口符号、binary SHA／大小、真实编译收据及source manifest；无对应binary即停止，不用运行时隐式编译替代。旧手册已通过源码检查的结果按其精确闭包复用，不改回SOURCE_ONLY。
3. `$NATIVE_ROOT` 必须事先由操作者明确提供并验证：绝对clean、非 `/`、≤1024字节、所有组件非symlink、euid拥有且精确0700。`$EVIDENCE`位于fixture外且日志文件尚不存在。`$SOURCE_TREE`填实际完整tree，不能填commit、旧候选tree或虚构hash。
4. 从专用规程第5节使用原样 `capture_inventory` 日志包装函数，并保存它与本次实际展开的白名单／argv。不要整块执行旧 `inventory_go` 或旧第7节。编译准备保留核验过的缓存和默认临时目录；下方实际运行env不设置TMPDIR／TMP／TEMP／GOTMPDIR，不继承其他manual gate。无自动超时，另行约定人工观察与异常保全；未自然退出或owner未知时记UNKNOWN，不自动kill／重跑。

以下是经当前源码逐项核对的**校正运行形状**；只有上述准备真实完成后，用户才在其获准原生环境中启动一次：

```bash
capture_inventory native-inventory /usr/bin/env -i \
  PATH=/usr/bin:/bin LC_ALL=C LANG=C TZ=UTC GOMAXPROCS=2 \
  GOSQLITE_MANUAL_DARWIN_INVENTORY=1 \
  GOSQLITE_MANUAL_DARWIN_INVENTORY_ROOT="$NATIVE_ROOT" \
  GOSQLITE_MANUAL_DARWIN_INVENTORY_SOURCE_TREE="$SOURCE_TREE" \
  "$EVIDENCE/manual.test" \
  -test.v -test.count=1 -test.parallel=1 -test.timeout=0 \
  -test.run '^TestManualDarwinUnicodeInventory$'
```

PASS：唯一精确根RUN／PASS、最终PASS、零子项／SKIP／FAIL、自然exit0／空stderr。11个 `ASSERT ... PASS` 各一次：prerequisites、nil-owner-refusal、retained-enumerator-classification、namespace-exact-bytes、namespace-byte-limit、namespace-entry-limit、selfopen-exact-bytes、selfopen-byte-limit、selfopen-entry-limit、samples-unchanged、retirement-complete。family12／sibling7字节、四个sample前后大小和digest不变；11个owner均attempted／complete／physical，group／lease／unresolved归零。保留fixture、ASSERT、RETIRE、日志与身份；预期ErrLimit不重试。正常／失败fixture均不自动删除，unknown保留owner事实，只有用户确认全部退休且证据封存后才清理准确新建目录。

### N04 Windows shared-rekey 三个普通原生入口

这三项没有manual tag/env，不因此允许整个Windows包或全仓运行。入口是 `TestWindowsEncryptedSharedRekeyCapability`、`TestEncryptionSharedRekeyLatestHeadOrdinary`、`TestEncryptionSharedQuiescentRekeyAndLeaseRetention`；见[主手册22.1—22.4](../manual-verification-zh.md)1579—1595、1701—1788行。

原生Windows/amd64、固定NTFS、Go1.27.2、Git Bash/cygpath与核验缓存。按22.2定义native_go/capture_rekey/环境白名单/source inventory，核实GOHOSTOS/ARCH。原三命令：

```bash
capture_rekey capability native_go test -mod=readonly -buildvcs=false -p=1 \
  -count=1 -json -vet=off -timeout=2m \
  -run '^TestWindowsEncryptedSharedRekeyCapability$' ./internal/encstore || exit 1
capture_rekey latest native_go test -mod=readonly -buildvcs=false -p=1 \
  -count=1 -json -vet=off -timeout=5m \
  -run '^TestEncryptionSharedRekeyLatestHeadOrdinary$' ./sqlite || exit 1
capture_rekey quiescent native_go test -mod=readonly -buildvcs=false -p=1 \
  -count=1 -json -vet=off -timeout=5m \
  -run '^TestEncryptionSharedQuiescentRekeyAndLeaseRetention$' ./sqlite || exit 1
```

逐条审JSON再下一条：准确根RUN/PASS、零子项/Skip、一个包PASS、exit0/空stderr/无cached。capability仅版本准入；latest采用最新head再新key重开；quiescent BUSY后lease保留/rekey和值正确。原模板vet=off不算clean-vet资格。

三测试用t.TempDir，失败DB可能删除且临时key无恢复承诺；若用途依赖失败保全/unknown排查则BLOCKED，不能假装现有harness支持。保存完整source/工具/NTFS/argv/JSON/清单，只资格三个正常路径。

## 本清单之外仍需取得的外部资格

这些事项不能硬折算为“还剩几场手工测试”，也不能全归因于平台安全限制。N01–N04只枚举既有有限原生入口；普通Linux健康文件／WAL路径已有有效实测，foreign编译和tmpfs上的volatile fsync不证明其他OS或掉电持久性。

| 外部待办 | 缺什么／由谁提供 | 完成时保留什么及边界 |
|---|---|---|
| 原生平台、FS、锁／Sync及真实持久介质 | 获准操作者提供目标OS／arch、支持的本地FS、设备／mount和真实持久性条件；需要物理故障或掉电时另核定范围和安全条件。 | 固定源码／binary／OS／卷／设备／缓存条件、实际原生结果和失败实物；普通CI、cross-build或本环境tmpfs不替代。参见[主手册](../manual-verification-zh.md)和各H/N组。 |
| 受控性能基线、同机8–32配对测量及实际CI比较 | 操作者提供受控机器、合格baseline、固定source／数据／seed／cache／page／Go／FS、预声明指标和阈值，并按合同保存真实配对证据。离线CI接线及纯fixture检查已具备，真实输入尚未提供，实际比较NOT_RUN；不需要公开真实业务数据库或秘密。 | 按[比较器与手动CI合同](../../benchmarks/performance/README.md)预声明8–32个完整交错baseline／candidate invocation pairs，保存全部报告、失败／排除原因与统计。先审阅允许分享的数据／环境元数据，再提交到固定evidence目录，在Actions手动选择offline-performance-evidence并填写相对目录。七状态及非零退出原样保留；来源声明不认证真实性，不足或INCONCLUSIVE不填PASS。普通CI只运行fixture，不采集benchmark或查询计费；新机器／费用／权限另由有权操作者决定。 |
| 长时间并发soak与受控冷热基准 | 获准操作者提供受控机器、资源预算、经审阅的观察窗口和已有runner输入。短时单元／有限事务测量不等于长期soak。 | 依[事务工具](../../benchmarks/transactions/README.md)、[grams工具](../../benchmarks/grams/README.md)、[ANN已有测量范围](../../benchmarks/ann/README.md)保存全时段ledger／invariant／owner、吞吐／延迟／资源及失败。冷缓存必须有真实条件证据；H12单次elapsed不是cold、P95或recovery-only。观察窗口结论另经审阅，不造无限矩阵。 |
| M9真实PG／Neon迁移复核 | 获准数据库操作者从**同一真实quiescent snapshot**提供完整catalog、依赖definitions、应用SQL corpus、profiles和源／目标／扩展版本；审阅人工计划、业务切换和回滚，并提供获准可丢弃PG／Neon实例。 | 输入hash绑定的report／inventory／注释模板、逐项支持与阻断、人工依赖计划和实际迁移核验。不能只把coverage标为complete，也不能用样本collector或本次PG语法测试冒充真实迁移。见[迁移计划](../migration-transition.md)、[CLI](../migration-transition-cli.md)、[pg_embedding边界](../migration-pgembedding.md)。不新增在线collector／executor义务，不借迁移执行受阻ANN链。 |
| 独立发布审查 | 独立安全／密码学／威胁模型、一致性、上游来源／依赖许可证以及API／文档审阅者对拟发布版本给出结论；需要法律专业判断时由相应有资质人员负责。 | 固定审查版本、范围、发现、修复复核和签结意见。自测和开发者自评不能替代外部结论；见[安全](../security.md)、[来源](../upstream.md)、[许可证清单](../../THIRD_PARTY_NOTICES.md)及[goal §11.3](../../goal.md)。 |

## 交回结果与停止条件

- 对每个实际case／入口／目标资格填写：source tree与binary身份、输入manifest／receipt、真实OS／FS／设备、完整argv／白名单env、UTC起止、stdout／stderr、自然exit、逐名断言、owner／Wait终态、证据位置以及PASS／FAIL／BLOCKED／NOT_RUN原因。
- 先保全失败、原件、独立副本及未知状态，再由用户决定如何处置；不得补造manifest hash、编译收据、keeper回执或成功布尔值。FAIL／SKIP／零匹配／截断／UNKNOWN不会因随后某项PASS被删除。所有旧失败和中断历史沿链接保留。
- 只把精确受测版本、适用环境和完整断言的实际结果改为PASS；未变源码的可信普通结果可明确复用。当前本页所有17组仍NOT_RUN；不得从源码／compile／普通CI推出物理PASS。
- ANN代码／可信报告缺口、ANN V4、41-target持续campaign和未知中断链保持上节边界，不启动替代流程。只有实际适用测试、剩余代码／报告修正和外部资格逐项取得可信证据，才能重新评估整体M10；17组／82场景即使有限通过，也不自动构成M10、Release或生产资格。
