> 最新验证状态：本七格源码经独立审查和受影响normal/race、sourceguard、双tag编译/nm通过；原Doc比较FAIL及最小修正保留。完整命令、原始事件与复用范围见[2026-10-12记录](../evidence/20261012-presync/README.zh.md)。下方SOURCE_ONLY是各次源稿历史状态，不能覆盖此普通验证结果；所有实际manual/物理仍NOT_RUN。

> 2026-10-12 三个 pre-DB caller 小型 overlay：从冻结的已发布第四格 main `a49d9bf` 重新生成，仅改受影响的 SQL caller／模型／守卫／测试／文档，当前运行树保持不变。重启前的 pre-DB overlay 未保全，也没有可供逐字节比对的旧 overlay hash；这里是新生成的源码，不能称为恢复旧 overlay 或继承其验证结果。本次状态为 **SOURCE_ONLY / REVIEW_PENDING / EXECUTION_NOT_RUN**。只做源码文本编辑、字节检查，以及另行获准的固定官方 gofmt（Go 1.27.2，SHA-256 5583dbb3339147fee7cdb067eb3cd4a50015b48c27084377f49d93833aa4ad77）六个 overlay Go 文件格式化；没有运行 Go 测试、AST/parser/scanner runner、编译、manual/native、fixture、Commit、恢复或 controller。实际物理注入仍全部 **NOT_RUN**。最终完整文件／函数 pins 及其对应的源审另行核验；旧 pin 或旧通过记录不能转授新字节。下方历史状态仅说明其各自原范围，不覆盖本次状态。

> 2026-10-12 第四格源码更新：从已发布 `50b85ed048f56eee698a62d745b66d2e4b3773b9`、tree `d3bf2388ec66f91b1f081ee4177a07ab601bb8de` 的当前工作文件制作独立无 `.git` 后继，仅修改本 SQL slice 的七个既有文件。新增 `index-prefix-64-error`，状态为 **SOURCE_ONLY / REVIEW_PENDING / EXECUTION_NOT_RUN**。本次没有运行 Go、gofmt、AST/parser/scanner runner、测试、编译、manual/native、fixture、Commit、恢复、进程 controller 或 Git/GitHub 写入；下方历史供体已通过的有限检查不转授新源码。生产/default 路径、A5、单页 recovery 和全部其它 case 的既有边界保持原字节。

> 2026-10-12 三设施源码组合状态：本文件下方保留原供体说明与当时状态。当前组合从 main `7503531d2e3d9336bb5943ccd4c836c804d5ce03` 保留既有检查点，接入 shared SQL/A5 源树 `11ade9b1d33f7b317cdd613eed32cce886b6c1e1` 与单页 recovery 供体 `363b5cc7437f58e07722af3fef057fc0518ba309`。shared 最终七项有限续验已 PASS；recovery 供体原十项有限验证已 PASS。它们只适用于各自原始源树，不能算作本次组合 PASS。本组合仍为 SOURCE_ONLY / NOT_RUN，须在完整源审查后另行有限验证。所有实际 manual/native、真实故障、恢复和 process-exit 验收仍为 NOT_RUN；PhysicalValidated / Pass 不因此变为 true。下文旧 CODE_GAP、NOT_RUN 或候选 pin 字样是历史记录，不覆盖此组合状态。

# plain rollback 三页 SQL／普通索引手工故障设施

## 状态与边界

本候选为 SOURCE_ONLY／独立源审 HOLD。本文和源码都不是执行许可。本次只执行了另行获准的固定 gofmt 格式化，未运行 Go 测试、parser runner、编译、fixture 构造、Commit、恢复、目标二进制、oracle、fuzz、Git 或网络上传。纯模型只输出 evidence-consistent／ScenarioObserved；PhysicalValidated() 与 Pass() 永远为 false。

早期 SQL 供体曾从 readonly-acceptance-integration-source-v2 独立全量复制，保留既有只读媒体、升级、WAL、备份、恢复和 source guard，并经批准逐路径合入 v3 两份只读 guard 修正及 SQL hook 严格结果／错误／append 绑定负例。这是供体历史，不是本次复制动作。v3 的既有源审和7根AST／sourceguard执行结果只属于其原树，不转授本候选。整个 main.go 与 inspectSource body pin 释放须另行审阅；保留旧 pin 的明确 HOLD，不通过剥去新例外再重构旧 body 来绕过复审。供体当时未修改 armer；本次仅对 gated SQL armer／caller 做本节列明的有限追加，不修改生产 Commit 或默认 shipping 路径。

本片只填补现有 A1/A2 plain rollback adapter 到公开 SQL Tx 与普通 B-tree 索引的一条有限绑定。它不是 A5 退出／重启／durable-ack controller，不是 A6 replay 再中断，不涵盖 WAL、加密、其它五种 profile、grams13、ANN 或向量。grams13 的 shadow tables/indexes 加 schema/source 至少九页，超出既有四页 armer；本片没有增大页数、事件容量或放宽 keepFile/session/borrowedCleanup 拒绝条件。

## 冻结输入与七个独立 case

4096 字节 page／sector，恰三页：page1 sqlite_schema、page2 表 t、page3 普通索引 by_k。无 page split、overflow、freelist、额外 schema 对象。源码直接编码健康三页基线；第一次公开 Open 使用 NoCreate，不以初始化 Commit 偷增事务次数。

旧账本：
- (1, red, one)
- (2, blue, two)
- (3, red, three)

唯一公开 Tx 内依次执行更新 id=1 为 green/ONE、删除 id=2、插入 (4,blue,four)，每条 RowsAffected 必须恰一。新账本：
- (1, green, ONE)
- (3, red, three)
- (4, blue, four)

旧、新都是三行，不能只按 count 判定。类型固定 int64／string／string。预期行在纯模型源码中独立冻结，从不根据损坏文件或恢复输出生成。

每个独立进程仅选一个 case：
1. control：第三次 DB_PAGE，offset=8192，Pass。唯一 public Tx.Commit 返回 nil。新副本读回完整新账本。Recovery 调用数为零。
2. index-prefix-64：同一第三次 DB_PAGE，adapter 对原 managed owner 只请求 64 字节，实际返回必须是 (64,nil)；adapter 对原 4096 字节逻辑请求暴露 (64,nil)，writeFullAt 将其规范化为 io.ErrShortWrite。公开返回必须同时保留 io.ErrShortWrite、ErrRecoveryRequired、CodeIOErr。恰一次恢复副本，读回完整旧账本。
3. db-sync-before：DB_SYNC 执行之前 FailBefore。不把完整写入的新字节当 durable ack；公开返回 ErrRecoveryRequired／CodeIOErr，保留注入原因。副本恢复后必须完整旧账本。
4. index-prefix-64-error：同一 DB_PAGE#3、offset=8192、length=4096、Prefix=64、PrefixError=true。原 managed owner 对 64 字节请求实际返回 (64,nil)，adapter 才向 4096 字节逻辑写暴露 (64,private injected error)。公开返回 CodeIOErr、ErrRecoveryRequired，并通过紧邻的 Finish 保留原注入原因；必须不含 io.ErrShortWrite，不接受 cleanup pending 或 CommitUncertain。这是 generic 注入错误，不是 ENOSPC/权限错误、OS 对 4096 字节请求的真实短写或硬件 torn-sector。副本恰一次恢复为完整旧账本。

5. jbody-sync-before：JournalBodySync、Journal owner=12、occurrence=1，目标是原完整路径的 Calls[8]。journal 的三条完整 original-page record 已记录，所选 journal body Sync 在调用之前 FailBefore；其 native Sync **未被调用**。随后只允许正常 CleanupUnlink（Journal owner=12）和 CleanupDirectorySync（Directory owner=13），不得进入 DB_PAGE／DB_SYNC。
6. jheader-sync-before：JournalHeaderSync、Journal owner=12、occurrence=1，目标为 Calls[10]。body Sync 和 valid-header 写入在此前已完成，所选 header Sync **未被调用**；随后同样要求正常 unlink／cleanup directory Sync，不能写 DB 页。
7. publish-dirsync-before：PublishDirectorySync、Directory owner=13、occurrence=1，目标为 Calls[11]。journal header Sync 在此前已完成，所选 publish directory helper **未被调用**；随后要求正常 cleanup，不能写 DB 页。Directory owner 标识复合 helper scope，不是一个可对外操作的 descriptor。

后三格的公开 Commit 必须为失败、CodeIOErr、ErrPoisoned，并保留唯一所选注入原因；不接受 ErrRecoveryRequired、io.ErrShortWrite、CommitUncertain、cleanup pending 或额外错误原因。trial 全字节必须等于 OLD baseline，正常 cleanup 后不得剩余 journal。对保留的独立新副本作全新只读重开并验证 OLD，Recovery 调用数必须为零，不能为证明“无需恢复”再调用一次 recovery。

控制 trace 恰18个原始调用／41 events，两种前缀故障均15个／35 events，DB Sync故障16个／37 events。三个 pre-DB trace 分别恰11／27、13／31、14／33；这些调用数包括所选未执行调用的 intent/return 和随后两个正常 cleanup 调用，CommitReturned 的类别均为 CommitFailed。原 armer 仍为最多4页、32调用、128事件、一次终身 scope。所有 three-page plan 的 original 页序固定为0／4096／8192；只有前四格路径进入对应 dirty-page 写入。

FailBefore 只表示 adapter 在所选 native Sync／复合 helper 调用之前返回私有注入错误，不表示那个实际 Sync 返回了失败。它也不是断电、OS fsync failure、lost write、硬件 torn-sector 或持久性证明。随后 cleanup 的正常执行／返回必须有独立 transcript 证据；不能因为命中 fail-before 就假定 cleanup 成功，也不能把 synthetic 模型行当成真实 I/O 观察。

## 生命周期与保留要求

唯一 native 入口是 sqlite 包的 TestManualRollbackFaultSQLLedger。文件与 bridge 均以 linux && manual_fault_rollback 编译门隔离；入口第一项是 GOSQLITE_MANUAL_FAULT_ROLLBACK=independently_authorized 环境门。两个门都不取代执行许可。

先验证调用方预建的空、0700、当前用户所有、逐组件无 symlink 的稳定绝对目录。绑定文件及其外部审定 hash、source inventory、独立 compile-only receipt、实际 executable 与工具的 hash 必须一致。此私有 namespace 是环境前提，Lstat／SameFile 复核不能防御恶意并发目录替换。

桥仅导出 Arm／Finish／Disarm，以及只读 scalar ManualRollbackSQLState。opaque scope 不暴露 native file／cleanup 指针或路径。scalar 状态只在锁下观察计数和健康条件，不调用 native owner 方法，不提供 registry reset、cleanup retry 或 session/keepFile 绕过。

同一个 sqlite helper 的 tx.storage 用于 Arm，但 Commit 必须由 tx.Commit(ctx) 调用；下一语句立即以原始 error 对象 Finish。错误对象不替换。检验公开 logical Tx done、db.tx 释放、Rows=0、实际 adapter transcript、owner退休后才 Disarm。正常原生 cleanup 未知或 pending 时不重试，不通过 DB.Close 暗中 Rollback。

正常准备失败时，仅对 bridge 判定健康、未 arm、独立拥有的 Tx 调用一次公开 Rollback。文件、Rows、DB 的每个非 nil 返回，包括同时返回 error 的 owner，都先登记后检查 error；关闭失败保持未知，既不再次 Close 也不隐藏。成功终态要求登记的 owner 全部恰一次退休。延迟处理只报告剩余责任，不重试关闭。

baseline、binding、trial、post-commit 副本以及所有实际产生的失败输出均保留。只有三个既有 recovery case（index-prefix-64、db-sync-before、index-prefix-64-error）保留原 journal 和独立 journal 副本，并恰调用一次恢复；恢复只针对 O_EXCL 新建、与所有原件 device/inode 均不同的 replay.db／replay.db-journal。原 trial 和 journal 的 identity、长度、全字节 hash 在恢复前后不变；不向原 trial 调 RecoverRollback。这三格的固定限额为 MaxJournalBytes=16408、MaxRecords=3、MaxDatabaseBytes=12288，实际 Recovered=true、RestoredPages=OriginalPages=3，否则不接受。

control 和三个 pre-DB case 都不调用恢复，也不创建／保留三种 journal artifact；其 journal 的 Before／After evidence 必须全零。pre-DB 的 baseline、trial、post-commit input copy、独立 replay.db 的完整字节 hash 必须全部等于 OLD baseline，replay.db 名称只沿用固定 artifact 名，不表示做过 replay。各实体依旧要求独立 device/inode、固定长度和正常退休；先保留证据再全新只读 NoCreate Open，不能复用原 writer 或把新副本当修复目标。

PublicPoisoned 在全部六个失败 case 为 true，control 为 false；PublicRecoveryRequired 只在三个旧 recovery case 为 true，PublicShortWrite 只在 nil-prefix 格为 true。PublicCommitUncertain 和 PublicCleanupPending 在全部七格均须为 false。PreDBOnlyTargetCause 仅在三个 pre-DB case 为 true，由 Finish 捕获实际 private target 与 Tx.poisoned，再检查实际固定 public wrapper／raw poison／target cause chain 得出。caller 另要求 commitErr 就是原来的 *sqlite.Error public 对象。链检查在 Unwrap 之前拒绝 top／node 自定义 Is／As；top 的唯一 unwrap 必须等于实际 poison，后续只接受相同具体 errors.Join 类型的节点和精确 injected／ErrPoisoned 叶子，最多访问128个节点且 pending 数量有界。额外／未知／nil 原因、循环溢出都返回 false，不能从 plan 或仅凭 errors.Is 命中一个 cause 就填 true；替换 public error 或真实 cleanup error 不能冒充选定场景。

WriterRetiredBeforeRecovery 和 RecoveryFinishedBeforeReopen 是生命周期边界证据，全部七格都必须为 true。对零 recovery 格，它们表示 writer 已退休、已明确决定跳过 recovery，然后才独立只读重开，不表示调用过恢复。若 cleanup 返回实际错误、出现 pending／unknown owner 或多余原因，停止并保留现场，不重试 cleanup、Sync、Commit、Close 或 recovery，不借 DB.Close／Rollback 修复。

## SQL 与索引证据

在健康基线与处理后独立新副本的全新只读 NoCreate Open 上（仅三个旧 fault case 实际恢复），各执行同样七个固定查询：全表按 id 排序；blue／red／green 各一个索引查询和一个 NOT INDEXED 独立扫描。要求完整行值、类型、行身份与独立 old/new 账本一致，并且索引／scan结果相同。

索引查询必须 Rows.Plan 为 SEARCH／btree／by_k／字段 k／Recheck；Rows.Stats 必须有真实 index-page work，返回的键、候选、rowid seek和base row fetch必须守恒。不存在 green 的旧账本也必须实际探测索引页，不能用未执行或 fallback 冒充。scan计划必须显式 NOT INDEXED，三条基础行均被读取。每个 Rows恰一次EOF循环、Err检查、公共Close。

DB.Check 固定 MaxPages=3、MaxPageReads=512；要求 Scope正确、Pages=3、Tables=1、Indexes=1、TableRows=3、IndexEntries=3、FreePages=ReservedPages=ExtensionIndexes=0。普通重开成功或 rows count相同都不足以通过。

## 将来的有限验证顺序，当前均 NOT_RUN

1. 冻结全部源码与文件清单，独立 review bridge锁顺序、公开 Commit邻接、owner责任、旧 guard增量例外和全 main pin。HOLD不得因更新hash自行解除。
2. 另获许可后，只运行精确 descriptor-free模型及纯AST/source guard入口，包含重钉hash后的语义负例；审核 default／manual selection inventory。
3. 另获许可后，只 compile-only 生成 linux manual sqlite 测试二进制并记录 source／tool／binary／manifest关系；只读 symbol inventory，不执行该二进制。
4. 任何实际七个 case均需新的逐项执行授权、固定环境与保留目录。每次单独进程、独立空目录、唯一绑定case、精确测试参数。严禁执行 Go test ./...、宽selector、count>1、parallel/subtest、timeout-kill、进程controller或更换环境绕过限制。

未来入口的参数合同为：-test.run=^TestManualRollbackFaultSQLLedger$、-test.count=1、-test.shuffle=off、-test.parallel=1、-test.timeout=0、-test.cpu=1、-test.v=true，均须显式 equals形式且无额外参数。环境使用 GOSQLITE_ROLLBACK_SQL_ROOT、GOSQLITE_ROLLBACK_SQL_BINDING、GOSQLITE_ROLLBACK_SQL_BINDING_SHA256 与 GOSQLITE_ROLLBACK_SQL_PRIVATE_ROOT_CONFIRMED=private_stable_namespace_retained。绑定的 Case 为上述七个名称之一；compile receipt Schema 为 rollback-fault-sql-compile-v1、Package为 ./sqlite、Tags为 manual_fault_rollback、CompileOnly=true。这里只描述合同，不给出已获授权的启动命令或自动 runner。

成功场景的唯一终态仍名为 ROLLBACK_SQL_SCENARIO_OBSERVED，不能翻译为物理 PASS。保留 receipt、全部 transcript、原始公开 error及诊断、退出状态、source/tool/binary身份。未命中、缺失/重复/截断/旧 receipt、pending owner、unknown、wrong source或trial、old rows/new index混搭、缺EOF、错误索引计划、恢复原件、控制case调用恢复，均拒绝。

代码设施完成、独立源审通过、获准纯测试通过、compile-only通过、手工物理NOT_RUN和硬件资格必须分栏。此有限片结束不关闭完整M3/M8/M10或A5–A8。


## 第四格原核验清单（保留其范围）

此节是人工核验合同，全部执行步骤当前 NOT_RUN，不能把 tag、环境变量、旧 binary 或旧 receipt 当作新许可。

- 枚举只在旧 SQLDBSyncBefore 后追加 SQLIndexPrefix64Error，旧数值 Unknown=0、Control=1、nil-prefix=2、Sync-before=3 不变，新值=4。该第四格版本原先仅四个固定名字放行；本 overlay 在其后追加三个精确名字，不更改旧四格。armer 先按 PrefixError 区分两种 prefix，再将完整 PlanSpec 与 SQLPlan 冻结值相等比较。caller 在打开 writer 前也绑定 c.caseID 与完整 plan。
- 目标 Returned 必须为 Executed=true、NativeN=64、NativeError=None/0、ReturnedN=64、ReturnedError=Injected/0、NormalizedError=Injected/0、Effect=EffectPrefix。CommitReturned 为 CommitRecoveryRequired、LogicalClosed、TargetError=Injected/0。Finish 使用实际 errors.Is(commitErr, targetError) 建立因果；report.CommitError 仍是 public Commit 原对象。不得从 plan 直接填一份看似正确的实际观察。
- 若 native 实际不足64字节或返回非 nil error，保留原生结果；不得由注入覆盖、转换成 io.ErrShortWrite、忽略错误、补写或 retry。即使错误链保留正确，这也只是 unexpected native failure，不能让固定注入计划成功。非目标 I/O 出错、owner 未退休、事件缺失/重复、错误恢复范围同样停止并保留输入。
- 两个 prefix case 都检查 trial 第三页从 offset=8192+64 到文件末尾的字节仍等于基线尾部；这只是写入范围约束，不替代恢复后整个数据库与索引的 OLD 判定。恢复后的七查询、完整三行类型/值、Rows.Plan/Stats、EOF/Err/Close、DB.Check，以及完整字节 hash 和原件 inode/长度不变都沿用前文要求。
- 复核原 nil-prefix receipt 不能只改 Case 冒充新格；原生 Injected 标签、真实 native error 被 masked、非 nil错误规范化为 ShortWrite、public nil、TargetError/token不匹配、翻转 PrefixError、offset/length/occurrence/prefix 改动、少/重复目标、NEW/混合账本及 PhysicalClaim=true 均应拒绝。

### 可复用与需要重新核验的范围

既有公开测试根名称全部保留，包括 TestSQLLedgerThreeFrozenScenariosRemainObservationOnly。该旧名字在第四格版本遍历四格，本 overlay 扩展为七格，避免破坏原精确 selector。第四格新增的三个 descriptor-free 模型根仍保留：TestSQLLedgerPrefixErrorFreezesNativeAndAdapterReturns、TestSQLLedgerPrefixErrorRejectsPlanAndReceiptConfusion、TestSQLLedgerPrefixErrorRequiresCompleteOldReplayAndOwners。它们只处理合成值，没有 native 权限。

在后续明确允许的环境中，只计划受影响的 internal/testkit/rollbackfault 与 internal/testscope 包的精确模型/SQL guard 验证；SQL guard 负例必须同时重钉 byte hash 与完整 shape，再命中语义诊断，不能依赖过期 pin。保留既有负例并新增错误格映射、位翻转、错误分类、case/plan错配与只检查旧格尾部等反例。若需格式、AST 或编译检查，先绑定最终文件字节，再记录实际命令、工具版本/哈希、退出码和输出；本次并未执行这些检查。

第四格原建议的 compile-only 范围为 ./sqlite、manual_fault_rollback 及只读 symbol inventory；这是历史范围。本次协调者批准的后续兼容性编译可检查既有 manual_fault_rollback 与 manual_restart_rollback 双 tag 及其已冻结符号集合，只为确认未改 A5 wiring 的共享依赖仍可编译，不执行任何二进制，也不新增 A5 物理资格。A5 controller、全仓测试、宽 selector、真实 SQL/恢复执行仍不属于本切片验证。新的 source manifest、compile receipt、tool/binary hash 必须重新对应最终源码；旧设施证据可保留复用其原范围，不能冒充本格已验证。平台限制下跳过的检查明确记为 NOT_RUN／BLOCKED，绝不记为 PASS。


## 三个 pre-DB caller 的有限源审与模型清单

- SQLJournalBodySyncBefore=5、SQLJournalHeaderSyncBefore=6、SQLPublishDirectorySyncBefore=7，只追加，旧 Unknown=0、Control=1、nil-prefix=2、DB-sync-before=3、non-nil-prefix=4 不变。case 名字精确为 jbody-sync-before／jheader-sync-before／publish-dirsync-before。isSQLPreDBFailure 和 needsSQLRecovery 按精确 ID 划分，unknown／未来 ID 不得用数值范围自动归类。
- 三个独立正例字面值冻结各自 target、Calls 下标、call/event count、完整 prefix 路径和正常 cleanup 位置。目标 Returned 必须为 Executed=false、NativeN=ReturnedN=0、NativeError=None/0、ReturnedError=NormalizedError=Injected/0、Effect=EffectNone。其后两个 cleanup 为 Pass、Executed=true、全部 error/token 为零，owner 依次12／13；最后才记录 CommitFailed／LogicalClosed 和退休 owner。任何 DB_PAGE、DB_SYNC、JournalUnlink 或 FinalDirectorySync 冒入均不接受。
- 保留全部原测试根、原四格 artifact-negative 矩阵和 non-nil-prefix 反例，不能把原先“baseline 与 post 相同”的拒绝例直接扩到要求相同的 pre-DB 格而制造无变化反例。pre-DB 使用单独 focused list，覆盖 journal 任一 Before／After 非零、一次 replay、任意 recovery 范围／页数、非 baseline trial、相互一致但非 baseline 的三份副本、aliased inode、NEW／混合索引以及 PhysicalClaim。
- case／target 混淆、错误 owner/action、所选 native 实际执行、native error、目标缺失／重复、cleanup 缺失／真实错误／masked error、public nil、额外 cause、所有公开 flag、pending／unknown owner、生命周期边界缺失均有负例。七格共同检查七个查询的 EOF／Err／Close、完整行值、实际 query work、独立 reader owner 和 DB.Check；逻辑或 count 相同不能省略这些证据。
- 新增 descriptor-free 模型根为 TestSQLLedgerCasePartitionsUseOnlyExactIDs、TestSQLLedgerPreDBFailuresFreezeSkippedTargetAndNormalCleanup、TestSQLLedgerPreDBFailuresRejectPlanTargetAndCleanupConfusion、TestSQLLedgerPreDBFailuresRequireBaselineCopiesAndZeroRecovery、TestSQLLedgerAllSevenCasesRequireExactPublicFailureFlags、TestSQLLedgerAllSevenQueriesRequireEOFCheckAndRetiredOwners。这些测试源码只操作有限 synthetic values，没有创建 native fixture 或执行 SQL／Commit／recovery 的权限；本次没有运行它们。
- 源审应对最终完整文件／函数 body、全部相关 byte/shape pins 和语义负例一起核验，确认未靠旧 pin、部分重构或只覆盖旧四格绕过守卫。受影响模型／守卫的后续验证仍须遵守已批准的精确范围与最终字节绑定，不增加重复全量复制、宽 selector 或额外全仓测试。compile-only 和实际物理注入状态必须分开记录；本 overlay 当前均为 NOT_RUN，旧第四格通过不推出新三格通过。

本次不编辑 A5 文件，也不重新宣称 A5 已获资格。A5 的既有共享依赖仍需准确纳入源审：internal/pager/rollback_restart_manual_linux.go 使用 SQLPlan(...SQLControl)，restart caller 复用 baseline／read／open／RecoverCopy／owner helpers，但不调用 manualRollbackSQLCommit，也不使用 ManualRollbackSQLReport。本 overlay 保持 SQLControl 计划输出及这些共享 helper 的函数字节不变；这只是源码边界说明，不是 A5 新执行或新 PASS。
