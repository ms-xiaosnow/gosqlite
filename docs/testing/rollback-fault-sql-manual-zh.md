> 2026-10-12 第四格源码更新：从已发布 `50b85ed048f56eee698a62d745b66d2e4b3773b9`、tree `d3bf2388ec66f91b1f081ee4177a07ab601bb8de` 的当前工作文件制作独立无 `.git` 后继，仅修改本 SQL slice 的七个既有文件。新增 `index-prefix-64-error`，状态为 **SOURCE_ONLY / REVIEW_PENDING / EXECUTION_NOT_RUN**。本次没有运行 Go、gofmt、AST/parser/scanner runner、测试、编译、manual/native、fixture、Commit、恢复、进程 controller 或 Git/GitHub 写入；下方历史供体已通过的有限检查不转授新源码。生产/default 路径、A5、单页 recovery 和全部其它 case 的既有边界保持原字节。

> 2026-10-12 三设施源码组合状态：本文件下方保留原供体说明与当时状态。当前组合从 main `7503531d2e3d9336bb5943ccd4c836c804d5ce03` 保留既有检查点，接入 shared SQL/A5 源树 `11ade9b1d33f7b317cdd613eed32cce886b6c1e1` 与单页 recovery 供体 `363b5cc7437f58e07722af3fef057fc0518ba309`。shared 最终七项有限续验已 PASS；recovery 供体原十项有限验证已 PASS。它们只适用于各自原始源树，不能算作本次组合 PASS。本组合仍为 SOURCE_ONLY / NOT_RUN，须在完整源审查后另行有限验证。所有实际 manual/native、真实故障、恢复和 process-exit 验收仍为 NOT_RUN；PhysicalValidated / Pass 不因此变为 true。下文旧 CODE_GAP、NOT_RUN 或候选 pin 字样是历史记录，不覆盖此组合状态。

# plain rollback 三页 SQL／普通索引手工故障设施

## 状态与边界

本候选为 SOURCE_ONLY／独立源审 HOLD。本文和源码都不是执行许可。未运行 Go、gofmt、parser runner、测试、编译、fixture 构造、Commit、恢复、目标二进制、oracle、fuzz、Git 或网络上传。纯模型只输出 evidence-consistent／ScenarioObserved；PhysicalValidated() 与 Pass() 永远为 false。

本片从 readonly-acceptance-integration-source-v2 独立全量复制；保留既有只读媒体、升级、WAL、备份、恢复和 source guard。经协调者批准，已逐路径合入 v3 两份只读 guard 修正，并为 SQL 新 hook 增加严格结果／错误／append 绑定及负例。v3 的既有源审和7根AST／sourceguard执行结果只属于其原树，不转授本候选。整个 main.go 与 inspectSource body pin 释放须另行审阅；保留旧 pin 的明确 HOLD，不通过剥去新例外再重构旧 body 来绕过复审。未修改现有 armer、生产 Commit 或默认 shipping 路径。

本片只填补现有 A1/A2 plain rollback adapter 到公开 SQL Tx 与普通 B-tree 索引的一条有限绑定。它不是 A5 退出／重启／durable-ack controller，不是 A6 replay 再中断，不涵盖 WAL、加密、其它五种 profile、grams13、ANN 或向量。grams13 的 shadow tables/indexes 加 schema/source 至少九页，超出既有四页 armer；本片没有增大页数、事件容量或放宽 keepFile/session/borrowedCleanup 拒绝条件。

## 冻结输入与四个独立 case

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

控制 trace 恰18个原始调用／41 events，两种前缀故障均15个／35 events，Sync故障16个／37 events；原 armer 仍为最多4页、32调用、128事件、一次终身 scope。所有 three-page plan 的 dirty/original 页序固定为0／4096／8192。

## 生命周期与保留要求

唯一 native 入口是 sqlite 包的 TestManualRollbackFaultSQLLedger。文件与 bridge 均以 linux && manual_fault_rollback 编译门隔离；入口第一项是 GOSQLITE_MANUAL_FAULT_ROLLBACK=independently_authorized 环境门。两个门都不取代执行许可。

先验证调用方预建的空、0700、当前用户所有、逐组件无 symlink 的稳定绝对目录。绑定文件及其外部审定 hash、source inventory、独立 compile-only receipt、实际 executable 与工具的 hash 必须一致。此私有 namespace 是环境前提，Lstat／SameFile 复核不能防御恶意并发目录替换。

桥仅导出 Arm／Finish／Disarm，以及只读 scalar ManualRollbackSQLState。opaque scope 不暴露 native file／cleanup 指针或路径。scalar 状态只在锁下观察计数和健康条件，不调用 native owner 方法，不提供 registry reset、cleanup retry 或 session/keepFile 绕过。

同一个 sqlite helper 的 tx.storage 用于 Arm，但 Commit 必须由 tx.Commit(ctx) 调用；下一语句立即以原始 error 对象 Finish。错误对象不替换。检验公开 logical Tx done、db.tx 释放、Rows=0、实际 adapter transcript、owner退休后才 Disarm。正常原生 cleanup 未知或 pending 时不重试，不通过 DB.Close 暗中 Rollback。

正常准备失败时，仅对 bridge 判定健康、未 arm、独立拥有的 Tx 调用一次公开 Rollback。文件、Rows、DB 的每个非 nil 返回，包括同时返回 error 的 owner，都先登记后检查 error；关闭失败保持未知，既不再次 Close 也不隐藏。成功终态要求登记的 owner 全部恰一次退休。延迟处理只报告剩余责任，不重试关闭。

baseline、binding、trial、原 journal、post-commit 副本、journal 副本以及所有失败输出均保留。恢复只针对 O_EXCL 新建、与所有原件 device/inode 均不同的 replay.db／replay.db-journal。原 trial 和 journal 的 identity、长度、全字节 hash 在恢复前后不变；不向原 trial 调 RecoverRollback。control 不调用恢复。fault 的固定限额为 MaxJournalBytes=16408、MaxRecords=3、MaxDatabaseBytes=12288，实际 Recovered=true、RestoredPages=OriginalPages=3，否则不接受。

## SQL 与索引证据

在健康基线与恢复后全新只读 NoCreate Open 上，各执行同样七个固定查询：全表按 id 排序；blue／red／green 各一个索引查询和一个 NOT INDEXED 独立扫描。要求完整行值、类型、行身份与独立 old/new 账本一致，并且索引／scan结果相同。

索引查询必须 Rows.Plan 为 SEARCH／btree／by_k／字段 k／Recheck；Rows.Stats 必须有真实 index-page work，返回的键、候选、rowid seek和base row fetch必须守恒。不存在 green 的旧账本也必须实际探测索引页，不能用未执行或 fallback 冒充。scan计划必须显式 NOT INDEXED，三条基础行均被读取。每个 Rows恰一次EOF循环、Err检查、公共Close。

DB.Check 固定 MaxPages=3、MaxPageReads=512；要求 Scope正确、Pages=3、Tables=1、Indexes=1、TableRows=3、IndexEntries=3、FreePages=ReservedPages=ExtensionIndexes=0。普通重开成功或 rows count相同都不足以通过。

## 将来的有限验证顺序，当前均 NOT_RUN

1. 冻结全部源码与文件清单，独立 review bridge锁顺序、公开 Commit邻接、owner责任、旧 guard增量例外和全 main pin。HOLD不得因更新hash自行解除。
2. 另获许可后，只运行精确 descriptor-free模型及纯AST/source guard入口，包含重钉hash后的语义负例；审核 default／manual selection inventory。
3. 另获许可后，只 compile-only 生成 linux manual sqlite 测试二进制并记录 source／tool／binary／manifest关系；只读 symbol inventory，不执行该二进制。
4. 任何实际四个 case均需新的逐项执行授权、固定环境与保留目录。每次单独进程、独立空目录、唯一绑定case、精确测试参数。严禁执行 Go test ./...、宽selector、count>1、parallel/subtest、timeout-kill、进程controller或更换环境绕过限制。

未来入口的参数合同为：-test.run=^TestManualRollbackFaultSQLLedger$、-test.count=1、-test.shuffle=off、-test.parallel=1、-test.timeout=0、-test.cpu=1、-test.v=true，均须显式 equals形式且无额外参数。环境使用 GOSQLITE_ROLLBACK_SQL_ROOT、GOSQLITE_ROLLBACK_SQL_BINDING、GOSQLITE_ROLLBACK_SQL_BINDING_SHA256 与 GOSQLITE_ROLLBACK_SQL_PRIVATE_ROOT_CONFIRMED=private_stable_namespace_retained。绑定的 Case 为上述四个名称之一；compile receipt Schema 为 rollback-fault-sql-compile-v1、Package为 ./sqlite、Tags为 manual_fault_rollback、CompileOnly=true。这里只描述合同，不给出已获授权的启动命令或自动 runner。

成功场景的唯一终态仍名为 ROLLBACK_SQL_SCENARIO_OBSERVED，不能翻译为物理 PASS。保留 receipt、全部 transcript、原始公开 error及诊断、退出状态、source/tool/binary身份。未命中、缺失/重复/截断/旧 receipt、pending owner、unknown、wrong source或trial、old rows/new index混搭、缺EOF、错误索引计划、恢复原件、控制case调用恢复，均拒绝。

代码设施完成、独立源审通过、获准纯测试通过、compile-only通过、手工物理NOT_RUN和硬件资格必须分栏。此有限片结束不关闭完整M3/M8/M10或A5–A8。


## 新增 non-nil 前缀格的核验清单

此节是人工核验合同，全部执行步骤当前 NOT_RUN，不能把 tag、环境变量、旧 binary 或旧 receipt 当作新许可。

- 枚举只在旧 SQLDBSyncBefore 后追加 SQLIndexPrefix64Error，旧数值 Unknown=0、Control=1、nil-prefix=2、Sync-before=3 不变，新值=4。仅四个固定名字放行；armer 先按 PrefixError 区分两种 prefix，再将完整 PlanSpec 与 SQLPlan 冻结值相等比较。caller 在打开 writer 前也绑定 c.caseID 与完整 plan。
- 目标 Returned 必须为 Executed=true、NativeN=64、NativeError=None/0、ReturnedN=64、ReturnedError=Injected/0、NormalizedError=Injected/0、Effect=EffectPrefix。CommitReturned 为 CommitRecoveryRequired、LogicalClosed、TargetError=Injected/0。Finish 使用实际 errors.Is(commitErr, targetError) 建立因果；report.CommitError 仍是 public Commit 原对象。不得从 plan 直接填一份看似正确的实际观察。
- 若 native 实际不足64字节或返回非 nil error，保留原生结果；不得由注入覆盖、转换成 io.ErrShortWrite、忽略错误、补写或 retry。即使错误链保留正确，这也只是 unexpected native failure，不能让固定注入计划成功。非目标 I/O 出错、owner 未退休、事件缺失/重复、错误恢复范围同样停止并保留输入。
- 两个 prefix case 都检查 trial 第三页从 offset=8192+64 到文件末尾的字节仍等于基线尾部；这只是写入范围约束，不替代恢复后整个数据库与索引的 OLD 判定。恢复后的七查询、完整三行类型/值、Rows.Plan/Stats、EOF/Err/Close、DB.Check，以及完整字节 hash 和原件 inode/长度不变都沿用前文要求。
- 复核原 nil-prefix receipt 不能只改 Case 冒充新格；原生 Injected 标签、真实 native error 被 masked、非 nil错误规范化为 ShortWrite、public nil、TargetError/token不匹配、翻转 PrefixError、offset/length/occurrence/prefix 改动、少/重复目标、NEW/混合账本及 PhysicalClaim=true 均应拒绝。

### 可复用与需要重新核验的范围

既有公开测试根名称全部保留，包括 TestSQLLedgerThreeFrozenScenariosRemainObservationOnly。该旧名字现在遍历四格，避免破坏原精确 selector。新增三个 descriptor-free 模型根为：TestSQLLedgerPrefixErrorFreezesNativeAndAdapterReturns、TestSQLLedgerPrefixErrorRejectsPlanAndReceiptConfusion、TestSQLLedgerPrefixErrorRequiresCompleteOldReplayAndOwners。它们只处理合成值，没有 native 权限。

在后续明确允许的环境中，只计划受影响的 internal/testkit/rollbackfault 与 internal/testscope 包的精确模型/SQL guard 验证；SQL guard 负例必须同时重钉 byte hash 与完整 shape，再命中语义诊断，不能依赖过期 pin。保留既有负例并新增错误格映射、位翻转、错误分类、case/plan错配与只检查旧格尾部等反例。若需格式、AST 或编译检查，先绑定最终文件字节，再记录实际命令、工具版本/哈希、退出码和输出；本次并未执行这些检查。

若另获 compile-only 权限，仅涉及 ./sqlite、manual_fault_rollback 的唯一相关测试二进制及只读 symbol inventory。任何额外 tag、A5 controller、全仓测试、宽 selector、真实 SQL/恢复执行都不属于本切片的验证建议。新的 source manifest、compile receipt、tool/binary hash 必须重新对应最终源码；旧设施证据可保留复用其原范围，不能冒充本格已验证。平台限制下跳过的检查明确记为 NOT_RUN／BLOCKED，绝不记为 PASS。
