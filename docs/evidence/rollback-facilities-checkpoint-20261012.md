# Rollback 三设施检查点证据与验收边界

日期：2026-10-12 UTC。文档草稿v4：**实际新结果与独立复用分账，发布仍PENDING**。旧0ef／78992真实FAIL保留；51f63新相关验证已取得race和七项补充结果。没有新full26／完整ordinary／完整oracle脚本PASS的声明，也没有M10通过或全部编码完成的结论。

## 一 当前修正身份与原失败

受测修正树`51f63a56c22ab6396d5b542a1f0655a82dad4cd5`，3334文件／835 Go。44909前五门PASS；race于18:09:05自然process0，实际10928 PASS／7 SKIP（10935 named，313固定seed）。race本身test_status=PASS，唯一pause锁存使记录器自然exit2、lease RELEASED。原22计划保留，race后的16门未启动，不写22门完成。

验证范围经依赖复用核对缩减：普通组件为新metadata根1 PASS、testscope960 PASS加原78992未受影响9967 PASS／7 SKIP；oracle是明确组件复用，详见下方。额外session2054于18:14:09自然完成七项PASS，工具exit0、lease RELEASED：四个目标平台仅编译三个变化支持包、实际whole依赖213包／1378输入、Linux双manual-tag compile／只读nm。manual映像17180165 bytes、SHA-256 `03b465ee4ebf9595b1d863aa853d65476449883fe9469e95ee460e4019f5217b`；454 required／18 forbidden roots，映像未执行。

补充v1在18:09准入阶段因缺失便利字段KeyError退出，Go启动0、无源码变化；v2仅修记录器取字段方式，此失败原件保留。它不是项目测试失败或物理实验。

[私有仓库工程证据](20261012-rollback-facilities/README.zh.md)记录精确旧／新命令、退出状态、工具/source身份、原始失败、复用依据和Library索引；不包含审批转发、内部工作指令或大日志。下面准确保留原0ef失败。

- 冻结运行源：`recovery-three-facility-integrated-source-v1`，tree `0ef6d751edb19f55cf3cd46314a2d1f6bd597c6e`，3,334文件／835 Go文件。源码组合与源审分别见 `recovery-three-facility-integrated-evidence-v1/REPORT.md`、`recovery-three-facility-independent-review-v1/REPORT.md`。源审CLEAR不代替执行结果。
- full26 session `78992` 已 **FAIL**：前六门（guard、四条quick、live-writer readonly oracle）PASS；ordinary于 `2026-10-12T17:29:28.011817+00:00` 自然exit1；随后十九门 **NOT_RUN**。recorder于17:29:33停止，lease于 `2026-10-12T17:29:36.969399+00:00` RELEASED；无强制终止，无仍活跃的被观察child。外层工具终态回执chunk `7446c4`、exit1，见 `recovery-three-facility-full26-78992-tool-terminal.json`。
- ordinary原始JSON终态计数实测 **10,933 named = 10,925 PASS／1 FAIL／7 SKIP**。唯一失败：`github.com/ms-xiaosnow/gosqlite/internal/vfs.TestMetadataOwnershipCallChainSource`。准确诊断：`../pager/rollback_restart_manual_linux.go:85:131: bypasses platform-owned metadata: os.SameFile`。这是源码所有权约束失败，不是已观察到的manual/native数据损坏。
- ordinary原预计10,926 PASS／7 SKIP没有实现。race预期10,933 named和oracle预期12,665 named不是实际结果；两门都NOT_RUN。后续build／vet／闭包／跨平台compile与nm也未运行，不借历史结果补记。
- 最小metadata-owner wrapper后继源审现已CLEAR，完整新tree为51f63；源审不等于运行PASS。`0ef6d751…`始终对应本次FAIL，修正版按已更新的差异验证计划绑定新执行及明确复用证据。
- 新增generic Frame classifier的TypeParams检查及双重repin反例只属source-guard精确性加固，不宣称可执行native漏洞。本次metadata错误也不被扩大成未知安全事件。
- 本v4和三状态页只在独立overlay起草，未改冻结源、失败原件或活动控制文件。文档后继tree／新commit未确定。v1旧PENDING草稿保留作其时间快照，不再用作当前结论。

## 二 三项设施及供体结果分账

| 设施 | 组合中已有源码与有限场景 | 实际历史证据及精确范围 | 当前 manual 状态 |
|---|---|---|---|
| 单页 rollback recovery | 4096-byte 空 schema 页；control、page-prefix-64、db-sync-before；保留原件，只对 distinct-inode 副本 replay，逐字节检查整页 | recovery tree `363b5cc7437f58e07722af3fef057fc0518ba309`，session `19016` 原十项 PASS；68 roots、562 children、630 named、0 SKIP；14:56:44 UTC 自然终结 | 三格均 NOT_RUN；供体结果不授予组合／真实恢复资格 |
| 三页 SQL ledger | schema／table／by_k；control、index-prefix-64、db-sync-before；唯一 public Commit，完整 OLD／NEW 行和普通索引、七查询及 Check | tree `3ff9d6d155908f6542389505169f3148bff6a062`、session `65480`：SQL 31 roots／319 named PASS | 三格均 NOT_RUN；该批 guard 后来真实 FAIL，纯模型 PASS 不改它的终态 |
| A5 五格退出／重启协议 | control、before-jcreate、after-table-page、after-junlink、acknowledged-exit；controller／writer／fresh recovery 三角色与持久 ack | 同一旧 tree `3ff9d6d1…`、session `65480`：A5 13 roots／157 named PASS | 五格均 NOT_RUN；目标 exit86、进程 Wait／ack 均未实际取得 |

shared 最终供体 tree `11ade9b1d33f7b317cdd613eed32cce886b6c1e1` 在 session `73976` 于 `2026-10-12T17:08:56.016780+00:00` 自然完成七项 PASS、lease RELEASED：format、67 roots／759 children／826 named／0 SKIP 的 source-guard tests、完整 source guard、两次 Linux manual compile-only 及两次只读 nm。fault-only 符号 required／absent 为 451／21；fault+restart 为 454／18。两个 ELF 映像均未执行。SQL 319／A5 157 是旧树既有结果，**没有在 shared11ade 重跑，也不计为七项的新 PASS**。

shared 持久保存：Library `libfile_cb349e3aff048191a31dc59e269fb7e5`，archive SHA-256 `2537e2f609fa24350b197d871374a4cc15ebc75dae96129033c8a82cdab402e7`；静态读回完整通过。此包属于 shared11ade，不是组合 full26 的终态包、Git bundle 或已发布新检查点。

纯模型中的 ScenarioObserved／evidence-consistent 表示输入证据的有界一致性；PhysicalValidated 与 Pass 保持 false。compile-only、只读 nm、source/AST 正反例均不能改为 native recovery／fault／process-exit PASS。完整普通门可能实际运行既有 Linux 健康文件、WAL、正确密钥路径；不能把整个普通门错误称为 descriptor-free 或“所有 native 都未运行”。受限的三个新增 manual 设施仍单独 NOT_RUN。

## 三 历史失败和未知状态原样保留

以下四个设施有限验证失败各有原始 result／stdout／stderr／STOP／退出证据，后继修正不覆盖、重写或重新归类它们：

| 历史 session | 原源树或证据 | 必须保留的结论 |
|---|---|---|
| `43377` | `recovery-guard-failure-archive-proof-v1` | 原 recovery guard 失败；后继363b有限十项 PASS不将旧收据改成 PASS |
| `65480` | shared `3ff9d6d155908f6542389505169f3148bff6a062` | SQL／A5 纯模型已 PASS，但 07_all_guard_67_roots FAIL_TEST_VALIDATION；后续未运行门不补记 |
| `88359` | `c86801a00f1a6e739c451a041674b9b0a1b8839b` | 07_all_guard_revised_roots FAIL_TEST_VALIDATION |
| `89823` | `26fec15e271f79bdf742aaf698a8462dcc371b5d` | 67 roots／826 named PASS 与 08_all_source_guard 真实 FAIL 并存；后四项 compile／nm 当时 NOT_RUN |

`89823` 的失败源和证据已在 Library `libfile_a10e406013f48191ac4cd3c7f77fe626` 留存，SHA-256 `df19ee32ce3cb730529d12f1797cadf5c09de6dd7c1e5475b066793e3e2f00c8`。较早 readonly full26 `48019` guard FAIL／后25门 NOT_RUN、原环境 `54102` UNKNOWN、原 ANN V3 三项已知 FAIL 也不改写。ANN V4、41-target campaign／fuzz 活动及未知／拒绝中断链不启动、不重建、不换名、不借本检查点、CI 或另一环境替代。

## 四 已发布基点与新检查点严格区分

协调者于 2026-10-12 17:14 UTC 确认 remote main 为 `7503531d2e3d9336bb5943ccd4c836c804d5ce03`，原 tree `bd7cf6175bf7cb3b37f11804c9a5e181c3397d1e`。本文件编写过程没有访问 remote／CI，因此这项远端时点来自协调者；本地已存恢复归档提供以下历史收据：

- main750 bundle：Library `libfile_6fa94497ad888191ba517e8d6405285f`，SHA-256 `05d643be6344e7c3540d748071ef47dda160f27db17d1d182c1a0b16ca788ad9`。
- 该 bundle 的精确恢复 session `53330`：完整26门 PASS；终态保存 Library `libfile_0ad4df1dafb881919faf9c1c87067893`。这是原main750的恢复结果，不是当前三设施组合的恢复结果。
- 原 CI run `38259544486` SUCCESS；两个 Ubuntu jobs `114846234663`／`114846235011` SUCCESS；证据归档 Library `libfile_b54dce7c69ec8191bdf31de5a8d0a377`。原CI不包括当前组合。
- **本次旧full26终态为FAIL；失败原件Library归档已验证（见下面索引）。新commit、私有GitHub证据发布、bundle、独立恢复smoke及对应CI仍PENDING。** 无新URL或ID时不编造链接。

## 五 当前组合覆盖与历史计划变化

旧22命令完整argv作为历史计划留在`exact-commands.json`。18:01按依赖不变部分复用而缩减后续工作；18:02:50设置当前阶段后暂停，race继续自然完成。44909的行政pause与racePASS分别记账，不补写未启动命令；补充2054的七项是独立新执行。

| 范围 | 当前依据 | 不可扩大为 |
|---|---|---|
| ordinary | 当前metadata1／testscope960，加原78992的9967 PASS／7 SKIP；组合10928 PASS／7 SKIP | 当前完整ordinary执行或旧78992改PASS |
| race | 44909实际10928 PASS／7 SKIP，自然process0，1005.409秒 | 记录器22门PASS；行政pause不是测试FAIL |
| 原06 Linux amd64 build | 当前04的argv／env／cwd完全相同，直接复用 | 又执行一次06 |
| modules／shipping／LIMIT／encstore | 依赖、源字节与tool/module身份审计支持main750结果复用；encstore旧binary重新hash，nm绑定原映像 | 当前新建外国映像或新nm实跑 |
| oracle | main750未变oracle组件＋当前scanner／testscope960／metadata1＋78992 rollbackfault492／rollbackrestart157 | 当前完整oracle脚本或新12667 named实际结果 |
| 四外平台build | 2054对testscope／rollbackfault／rollbackrestart三支持包实际编译PASS；其余不变部分复用main750 | 新全模块四平台编译 |
| whole非测试闭包 | 2054实际213包／1378输入，较main750的212只新增rollbackrestart | 声称go/printer新加入，原750已含它 |
| A5双manual-tag | 2054新test-c和nm PASS；当前映像身份及454／18roots | 运行manual映像、实际fault／退出／恢复 |

复用审计SHA-256：`5c06e836907eaf90d7eed089159315debf16756233e72d4524620a0f8f438a52`。oracle变化包的default／oracle选中文件对称差为空；无live SQLITE_ORACLE读取、init或TestMain差异。唯一live GOFLAGS检测是manual_子串，在两个环境均false。source guard、metadata source-reading test采用当前结果；其它具名模型字节未变。实际旧整脚本收据不重标当前执行。

| 交付槽 | 状态 |
|---|---|
| 旧78992 | FAIL，原始记录不改 |
| 44909 | STOP_AUTHORIZATION_PAUSE／tool2；race testPASS／process0，lease RELEASED |
| 补充v1 | admission KeyError／tool1；Go启动0，原件保留 |
| 补充2054 | 七项PASS／tool0，lease RELEASED；映像未执行 |
| 原22与新7完整原始日志归档 | 44909／2054待Library索引；旧0ef失败包已验证 |
| 包含文档的commit | 由Git历史给出，不内嵌自身SHA或为此另建提交 |
| 文档应用 | 非文档字节／mode必须等于受测51f63；不为文档做额外全套测试 |
| 新bundle恢复与CI | PENDING；Git／字节核对＋必要smoke，CI实际结果单独记录 |

### 大日志与源码恢复索引

旧0ef源码：Library `libfile_920e52d469b481918d0007d078f915f4`，SHA-256 `42d76105b1fe62042b513f257d8170c1896f9590f39eaeccaed80d55e500251d`。其17:25 PENDING为历史快照。

旧78992完整失败：Library `libfile_b0911b5fc444819189ce4fda5a1d7060`，SHA-256 `7743daee52f9fbed0750795fa7f47f0f7b8636a9cbad6e11c285e5226610e957`。

51f63 delta：Library `libfile_38edf39bf6a48191a171cd56aabdb669`，610823 bytes，SHA-256 `72ea3e6c666aa6a1e10cc53313c28fffe16010ca5dae1b14da854dfa31ff7e13`。必须依赖0ef基包，静态重建全部3334文件；不是独立全量包。delta内44909 IN_PROGRESS是历史归档截面，未来终态不回写。其它原基点与终态包见`archives.json`。

## 六 三份现有中文人工手册

手册源码、已实现参数和原历史段落保持原字节。本页只给索引和验收边界，不新增执行许可或可绕过限制的启动流程。平台／用户独立允许之前，以下入口均 NOT_RUN。

| 手册与真实入口 | 精确编译／调用边界 | 必需断言 |
|---|---|---|
| [单页恢复](../testing/rollback-fault-recovery-manual-zh.md)；`TestManualRollbackFaultRecovery` | linux + manual_fault_rollback；唯一case；独立 source/tool/binary/compile receipt 和私有空目录 | control无恢复读NEW；两故障各恰一次副本恢复，8200 journal bytes／1 record／4096 DB bytes，完整OLD页；原trial／journal保持 |
| [三页SQL](../testing/rollback-fault-sql-manual-zh.md)；`TestManualRollbackFaultSQLLedger` | linux + manual_fault_rollback；当前只准三个case；`rollback-fault-sql-compile-v1`，package ./sqlite | OLD／NEW完整三行与类型，by_k／NOT INDEXED七查询、EOF／Err／Close及Check；故障仅副本恢复，16408／3／12288固定限额 |
| [A5退出重启](../testing/rollback-restart-manual-zh.md)；`TestManualRollbackRestartController`／`TestManualRollbackRestartWriter`／`TestManualRollbackRestartRecovery` | linux + manual_fault_rollback + manual_restart_rollback；五格分开；一个writer和全新recovery；canonical绑定、/proc/self/exe身份与固定fd协议 | 命中精确stage才exit86；真实Wait；ack写／file Sync／目录Sync／Close后才退出；仅after-table-page副本恢复，其余格按各自OLD／NEW；不推断掉电 |

三个手册的每次根调用参数都是显式 equals-form：`-test.run=^对应精确根$`、`-test.count=1`、`-test.shuffle=off`、`-test.parallel=1`、`-test.timeout=0`、`-test.cpu=1`、`-test.v=true`；这里“对应精确根”仅是说明占位，不是运行命令。各手册中的真实根名和原参数示例是原合同，不能替换成宽selector、regex union、批量case或重试脚本。timeout0不授予无限资源或进程处置权限。

依赖和隔离：独立审核的最终source inventory、工具、binary、compile receipt和binding实际hash；0700、用户所有、全新空、绝对clean、无symlink祖先且全期稳定的私有root；限定Linux与声明的4096配置。单页和SQL首门为 `GOSQLITE_MANUAL_FAULT_ROLLBACK=independently_authorized`；A5还需 `GOSQLITE_MANUAL_RESTART_ROLLBACK=independently_authorized`。各自ROOT／BINDING／BINDING_SHA256／PRIVATE_ROOT_CONFIRMED及A5 ROLE／Filesystem／NativeDeclaration按原手册取精确值。build tag、env和compile成功都不能独立授予运行资格。普通/default与oracle不选择这些manual入口。

保留和清理：只在用户拥有的独立试验目录／新副本上工作，绝不恢复或清理原用户数据库；trial、原journal、输入、post副本、协议、诊断与失败结果全部保留。replay family必须O_EXCL新建、distinct inode，仅它可被恢复改变。没有自动RemoveAll、重试Close、reset tombstone、隐藏Rollback或cleanup开关；后来清理要先备份证据，再由用户确认具体对象。A5超时／child-live／Wait-unknown保留未知，不自动Kill／Signal，不将晚到退出改写成初始成功。

## 七 仍须完成的有限代码和资格边界

本批rollback设施主要补M3／M4／M6／M8的基础验收；不记作M9迁移／扩展类型的新增完成度。M9须按其原条款与迁移源码证据另行核对，不由本批绿色结果替代。

原有限审计 `required-acceptance-boundary-audit-v1/REPORT.zh.md` 的A1–A8是审计分类。升级1→2及WAL-tail elapsed等后继设施已存在源码，不继续当作“完全缺失”；实际manual仍NOT_RUN，它们也不关闭全部格式边或全部WAL恢复计时。公开readonly仍只支持Linux live initialized-SHM既有子集；CHECK有界profile、EOFUnproven decode-only/native finite-only拒绝保持。

| 类别 | 这次已有的窄设施 | 真实剩余代码／profile／harness缺口 |
|---|---|---|
| A1 I/O与短写 | plain rollback有限adapter、单页caller、三页nil-prefix SQL caller | 非nil prefix SQL caller尚未实现；read/create/unlink及其他受支持操作caller未齐；不能把generic错误当具体errno |
| A2 Sync | 多个受审source site，单页／SQL仅DB_SYNC fail-before | 其他Sync点的真实caller、允许终态与收据未齐；fail-before不是原Sync返回失败或部分完成 |
| A3 ENOSPC／权限 | 意外native错误会保留，generic注入有界 | 无完整受控errno／真实容量或权限变化设施；逻辑quota和generic sentinel不能冒充 |
| A4 部分／错位／torn | 明确64-byte prefix软件写入 | 无错位、扇区散写、torn-media／重排／断电设施；native实际写的是64-byte请求，不是OS对4096请求的物理短写 |
| A5 commit／退出／重启 | plain rollback三页五格协议源码和纯模型 | 本组合native NOT_RUN；WAL／加密其他profile的切点、ack／恢复状态和扩展索引绑定仍缺 |
| A6 checkpoint／replay再中断 | 正常恢复和checkpoint已有各自路径 | 有限再中断controller／阶段收据仍缺；正常reopen不代替 |
| A7 rekey／upgrade／backup发布中断 | 正常升级／备份／恢复已有有限入口 | 中断authority／old-new root/key/archive／输入保留矩阵未齐；正常升级PASS不转授 |
| A8 磁盘认证负面 | 现有AEAD／正确密钥与部分拒绝断言 | 受支持磁盘profile的完整有限篡改、交换、错配、身份绑定与未认证明文拒绝矩阵仍须补齐 |

六profile为plain rollback、plain exclusive WAL、plain shared WAL、encrypted snapshot/layout1、encrypted exclusive/layout2、encrypted shared/layout3；按实际持久化操作冻结有限cut-point，不做无限OS×FS×硬件×SQL笛卡尔积。更多SQL、cold/no-SHM、其他OS或任意未来格式不自动成为本检查点必需范围；不支持合同照实拒绝。INSERT IGNORE保持暂停，ANN既有FAIL／V4／campaign／UNKNOWN边界不动。

外部M10层仍包括真实OS／FS／device持久性与锁／Sync、授权只读媒体和原生平台资格、有预算的soak及真实冷热性能、独立数据安全／密码学／威胁模型／许可法律／API审查、授权PG／Neon迁移和回滚验证。这些NOT_RUN／未验收不能由普通CI、模型、compile或本页文字关闭。既有V4事务soak工具不是缺失，但其未取得的长时结果仍NOT_RUN。

## 八 M9 原目标的精确小清单

后续[有限余缺审计](../testing/remaining-goal-boundary-20261012-zh.md)复核了supplied snapshot、inventory、blocked依赖计划和传递preflight：**没有找到M9最低范围必须新增生产代码的新证据**。旧自动采集PARTIAL是真实能力边界，不能升级成强制全自动采集或在线PG任务。下面按这一校正理解原小清单。

依据 `goal.md:318–345`（§9.1／9.2）、`:362`（M9）与`:508`（Neon资料核验），M9指“Neon／pg_embedding迁移与扩展类型”，不是完成百分比，也不替代M0–M8验收。独立静态报告为工作区 `m9-goal-status-audit-20261012T1729/REPORT.zh.md`，SHA-256 `ae5541c4c5a732b39b01606098a5de4c388b080fb66b7c09cef95ab499465776`；325条有界M9路径逐字节匹配冻结manifest，且全部同main750，故本三设施没有M9新增代码。

| 原目标条款 | 已有实现和当前依据 | 尚缺或边界 |
|---|---|---|
| §9.2(1) schema inspector | OID／类型／operator／opclass身份感知Inspect和JSON／Markdown报告 | **PARTIAL自动采集能力，非最低强制新代码**：样例只选HNSW／direct dependency且缺正文/profile；但完整supplied snapshot进inventory、缺计划blocked及传递preflight已有。最低范围需真实完整输入／审阅／PG验证，不要求新增在线采集或执行器 |
| §9.1／§9.2(4) AST转换 | 类型和ownership确认后`<~>`→`<+>`，保留bit语义、注释与字符串；ARRAY／REAL[]与参数转换 | 更广SQL／动态依赖转换尚未实现，但并非要求立即支持全部PG方言 |
| §9.2(2) 导入和校验 | CSV／NDJSON批事务、正常close/reopen续传、quarantine、count／PK／dim／norm和显式独立精确距离 | 正常续传不证明crash恢复；任意用户数据集一键ANN recall路径未接入，但固定fixture recall已有，不能把扩展需求一律升为最低必需 |
| §9.2(3) PG／Neon审阅包 | 影子列／批cursor／回滚边界及受限PG16整表迁移模板；默认只生成，破坏性SQL人工门保留 | live PG／Neon及业务切换回滚 **NOT_RUN外部验证**；更广依赖重建／在线dual-write确实未实现，并非原M9全部强制项；pgx直连可选 |
| §9.2(5) 两族fixture | 固定pg_embedding 0.3.6与pgvector v0.8.7来源、分项误差约定、实际索引固定fixture recall | 合成输入明确标注；不承诺任意用户数据的ANN排序／recall；V3报告FAIL／V4主要邻接M7，不挪成M9代码编辑禁令 |
| M9／§7.1／§7.3 扩展类型 | halfvec／bit／sparsevec的codec、SQL／driver／存储／精确检索及明确typed ANN支持矩阵 | sparse IVF、IVF L1等明确unsupported，不自动变成必须全补的组合 |

本次失败ordinary中仍有可单列的实际M9结果：`migration/postgres` **333 named PASS／0 FAIL／0 SKIP**，包终态在`02_ordinary.stdout:33927`。它属于原0ef实际运行，不是历史转授；不会将整轮FAIL或后续race／oracle NOT_RUN改成PASS。其他关联包的细分与测试行号见该审计FACTS.json。

Neon现行URL在2026-10-10已有重新取得记录，当时请求的迁移章节缺失；2024-01-23官方历史章节另行固定。不得称“从未核验”，也不据已有记录声称2026-10-12网页内容未变。为新目标版本实施时需重核并固定新snapshot／差异ADR。上述原始资料、真实外部PG／Neon执行与代码PARTIAL分别记账。

## 九 唯一下一源码切片

`rollback-next-facility-source-audit-v1/REPORT.zh.md` 给出的下一项是第四格 **`index-prefix-64-error`**，只在完成本检查点后从独立后继做有限改动。当前 **NOT_IMPLEMENTED**，不得给出假装可立即选择它的命令。

它复用原DB_PAGE#3／offset8192／4096-byte逻辑写点，软件实际WriteAt仅64 bytes，native `(64,nil)` 后返回原注入非nil原因；公开Commit必须保留CodeIOErr＋ErrRecoveryRequired＋该cause，不能误报io.ErrShortWrite。副本恢复完整OLD、七查询／普通索引和owner规则不变。预计四格plan／armer／caller／pure／guard／中文手册局部同步，不改shipping commit、VFS或A5。此片只填A1一个caller缺口，不关闭上表其余项，也不拓宽执行权限。
