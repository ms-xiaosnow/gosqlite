# 2026-10-12 R2：固定错位写入源码后继

状态：**SOURCE_ONLY / INDEPENDENT_REVIEW_PENDING / FORMAT_NOT_RUN / MODEL_GUARD_COMPILE_NOT_RUN / NATIVE_NOT_RUN**。
此后继基于 main `01c63e6f8fb3d9e196afe2f42a163eb3eac4ad5a`，只编辑独立小型 overlay 的十一份既有 Go 源文件与两份中文手册，不改运行工作树。没有运行 Go、gofmt、AST/parser/scanner runner、测试、fixture、Commit、miswrite、recovery、目标二进制或 CI。下方历史已通过记录仍只属于原始字节，不能转授本后继。R2 设施代码终点为独立源审与另行批准的有限模型／guard／编译验证；本稿尚未达到该终点。完整 goal/M3 与真实介质资格没有因此关闭。

## 唯一新增 case 与完整允许结果

在原九格之后仅追加 `index-misplace-minus64-error`（`SQLIndexMisplaceMinus64Error=10`）。旧九个 case 的值、目标及分类不变。新增 `MisplaceIndexPageMinus64`，没有任意 offset 参数、回调、profile 或重试框架。

- 请求仍为 DBPage 第3次、同一个原 managed database owner、offset=8192、length=4096。通用 Call／Result／Event 字段形状不变。
- 真正调用为同一个 owner 的 `WriteAt(b, off-64)`，b 是完整4096字节 payload，native offset=8128。写入区间 `[8128,12224)` 只覆盖原 journal 保护的 page2 尾64字节和 page3 前4032字节，文件仍必须恰12288字节。
- 只有实际 `(4096,nil)` 才追加已有 private injected error，向原 writeFullAt 返回 `(4096,injected)`。实际非nil错误、短计数、无效计数均保留，不覆盖、不补写、不重试，也不能满足该计划。
- public Commit 必须保留原 `*sqlite.Error`、CodeIOErr、ErrPoisoned、ErrRecoveryRequired 与唯一原注入原因。短写、uncertain、pending、额外 Close/cleanup 原因不合格。原 public Commit／立即 Finish 邻接保留。
- 完整允许结果集只有 `{OLD}`：一次 fresh independent replay.db 恢复后，七个 typed SQL 查询、实际 SEARCH/by_k 与 NOT INDEXED 工作量、EOF/Err/一次Close、三页完整 Check 和全文件 baseline hash 都必须相等。NEW、混合行/索引、同count不同成员均不合格。损坏原 trial 从不作为健康 NEW 打开。

## 实际参数、字节与 journal 证据

SQL 专用固定记录保存 writer 入参 offset/len、真正 native 变量 offset/len、调用点累计次数及调用前完整 payload SHA-256。新分支只调用一次原 WriteAt；scope 在独立锁内保存这一观察，锁内无I/O。不会从 plan 反填 native offset，不会把 Returned.Call.Offset 改成8128。其余九格的完整 SQLMisplacementEvidence 必须为零。

arm 额外检查实际 t.info.Size=12288、original/header count=3、dirty与journal originals 恰1/2/3、完整4096字节 buffer及schema/table/index叶种类13/13/10。既有健康独立owner、no session/shared/encryption/keepFile/borrowed cleanup、one-lifetime registry全部保留。

确认预期的非 nil ErrRecoveryRequired public Commit、唯一注入原因与 owner 退休检查全部合格后，caller 才读原 trial：同 inode、恰12288字节，post[8128:12224] hash 等于调用前 payload hash，且不同于独立 baseline 同区间；post[12224:12288] 与 baseline 尾64字节逐字节相等。不能要求其余所有字节均OLD，因为前两页已正常写入。

原16408字节 journal 在内存中用现有纯 `journal.ParseReplay` 解析一次，限制仍16408/3/12288。要求 finite-header count、Records=3、OriginalPages=3、page/sector=4096、DatabaseBytes=12288、ValidateExtent(12288)成功、pages按1/2/3排序且每个完整original image逐字节等于独立baseline。保留各页hash；通用receipt中的journal body/header/directory发布成功前缀仍不可省略。

`commit-observation.json` 在任何依赖性失败退出之前保留实际 writer/cause/已获字节观察和补充错误文本；原错误对象与diagnostics仍保存在内存，文本不替代identity检查。原件、不可变copy与独立replay的文件inventory、长度、不同inode、retention与一次owner退休规则不变。只恢复副本，原 trial与原 journal始终保留。无extra recovery、Close或cleanup retry。

## 兼容性与后续有限范围

生产 Commit、default bridge、VFS、recovery、A5代码与rollbackFaultBefore/Core、rollbackFaultAfter/Core四个函数保持原字节；A5使用Pass，不产生placement记录。现有SQL manual root名字不变，不增加可运行根或controller。

新增纯值正/负例及双重重钉source hash/full snapshot后仍必须命中独立语义诊断的反例，覆盖真实调用offset、完整payload、一次调用、native错误优先、严格实际geometry、journal/tail字节、public cause、OLD-only与旧九格零证据。旧否定断言和A5 exact-core/mutex-order检查保留；未来unknown case边界从10移动到11，因为10现为唯一明确追加case。

另行授权后的固定范围仅为：rollbackfault／rollbackrestart／testscope三个纯值与source包；原内存WriterAt测试根；ordinary sourceguard；sqlite、pager的manual_fault_rollback编译及sqlite双tag编译；三个准确镜像的nm/hash/根inventory。不得运行tagged image（即使空selector），不得扩展到旧ANN、campaign、全仓或物理测试。所有检查必须对应最终source manifest和固定工具哈希，原FAIL/NOT_RUN保留。

以下历史手册与证据说明完整保留。

# 2026 年 10 月 12 日完整正常门终态说明

本手册对应的冻结候选 tree `f8192e090acb9acedb07f811a66d043c1a9a24c2`
已于 13:31:44 UTC 自然完成完整普通 26 门 / 28 次 Go 调用 PASS，session
`75199` exit 0，lease RELEASED；[准确终态与验收边界](../evidence/readonly-acceptance-checkpoint-20261012.md)
同时列明普通/race/oracle、source/AST、闭包和 foreign compile 的不同范围。

这包含真实 Linux 健康文件/WAL 路径，不能统称全部 native 未执行；但本手册的
真正 manual/物理场景仍 **NOT_RUN**。被审选择和闭包只资格已核验的初始化，
单一 selector 不隔离 init，也不能为未知初始化或其他环境担保。新提交/bundle/
精确恢复/新 CI 仍 PENDING。本说明没有更改下方任何命令、tag/env 门或授权要求。

## 历史前置说明和完整原手册

以下在途 PENDING、SOURCE_ONLY/HOLD 与旧 pin 状态保留原时点含义；
最新普通门终态以上文及检查点为准，旧原文与命令逐字保留。

# 2026 年 10 月 12 日只读验收 V3 检查点说明

本手册所述源码已纳入冻结候选 tree
`f8192e090acb9acedb07f811a66d043c1a9a24c2`。对应 source/纯模型/编译有限证据、
已核验的 ordinary 阶段与仍在执行的 full26 必须分开阅读：
[当前检查点与准确证据边界](../evidence/readonly-acceptance-checkpoint-20261012.md)。
本稿快照的完整 26 门、race、完整 oracle、新提交/恢复/CI 均未闭合。

本组合只含 plain rollback Commit 的 14-site adapter，默认六类 bridge 直通原调用；manual arm/finish/disarm 的真实 caller、SQL-ledger/grams 恢复验证没有纳入。fault/recovery 均 **NATIVE_NOT_RUN**，A1–A8 未闭合。

下列原文逐字保留，包括作者阶段 SOURCE_ONLY/HOLD、旧 pin 状态和既有命令模板。
这些是历史记录；新状态以上述检查点为准。这里没有生成新命令，任何 tag、环境变量、
模板、编译产物或本说明均不构成 native/物理执行授权。

## 历史手册原文完整保留

# Plain rollback 有限 fault adapter 首片（SOURCE ONLY）

状态：SOURCE_IMPLEMENTED_PENDING_INDEPENDENT_REVIEW / VALIDATION_NOT_RUN。
本文件只描述已写入候选的源码，不是测试运行收据。原始 goal、M3/M8/M10、A1–A8 和物理持久性均未验收。既有正常测试成功不转授本候选。

## 本片实物与闭合依赖

- `internal/pager/commit.go`：只将原 plain rollback 分支的 14 个 I/O site 换成六类私有 typed bridge；`writeFullAt` 只将参数从 `*vfs.File` 改为 `io.WriterAt`，函数体不变。
- `rollback_io_points.go`：固定 14 个私有 point 名；不含注册、I/O、init 或选项。
- `rollback_io_default.go`：精确 `!linux || !manual_fault_rollback`，六个 bridge 直接执行原 helper / owner 方法；没有 env、registry、observer、fault。原 WAL / recovery / checkpoint 的 helper 调用者仍传原具体 managed file，源码不变。
- `rollback_io_manual_linux.go`：精确 `linux && manual_fault_rollback`，实现一次显式 arm 的具体 Tx、固定动作、有界事件与原错误对象保留；没有 arm/finish/disarm 的 caller、native `_test`、fixture、controller、goroutine、子进程或恢复入口。
- `internal/testkit/rollbackfault/{contract.go,contract_test.go}`：无 descriptor/文件/接口/回调的冻结计划及收据模型，纯状态机与 false-PASS 反例源码。
- `rollback_io_contract_test.go`：仅内存 fake WriterAt，测试真实 `writeFullAt` 的一次调用、参数原样、错误优先和短写规范化；没有构造 synthetic `vfs.File`。
- `internal/testscope/rollback_fault*.go` 及窄 main hook：精确文件、构建门、env 首语句、声明/调用者、原始 owner/参数绑定与否定变体的源码守卫。

依赖仅现有 managed `vfs.File`、`openJournal → openNativeFile → vfs.Open`、`syncDirectory → vfs.SyncDirectory`、`plainRetirement` 与标准库 `errors/io/os/sync`。未修改 Tx 字段、VFS 接口/native backend、公开 Options/DSN、文件格式、恢复 parser、模块依赖、workflow 或既有 scripts。

## 精确真实绑定（site 不是运行 event 数）

正常分支 12 sites：

1. RB.JCREATE：原 journal O_WRONLY|O_CREATE|O_EXCL constructor。
2. RB.JINVALID：invalid header 写入 offset 0。
3. RB.DB_ORIGINAL_READ：按原顺序读取 journal 原页。
4. RB.JRECORD：写原 journal record。
5. RB.JBODY_SYNC：invalid header/body Sync。
6. RB.JVALID_HEADER：只发布前 12 bytes。
7. RB.JHEADER_SYNC：已发布头 Sync。
8. RB.PUBLISH_DIRSYNC：journal 名称发布的完整目录 helper。
9. RB.DB_PAGE：原 dirty page 写入。
10. RB.DB_SYNC：原 database Sync。
11. RB.JUNLINK：原 journal unlink。
12. RB.FINAL_DIRSYNC：unlink 后完整目录 helper。

失败清理另两 sites：RB.CLEANUP_UNLINK、RB.CLEANUP_DIRSYNC。它们只观测并执行原清理，不接受计划选为故障目标。循环可以形成多个 event，但最多 4 页与 128 个事件，不把 14 sites 宣传为 14 次运行。

目录边界是现有 open/Sync/Close 复合 helper，不分别覆盖内部三个 native 操作。Close、lock、stat/path 检查、random、Open 初始化、恢复 replay、WAL 与 encryption 未接入。constructor 只可在调用前 fail-before；原 constructor 错误可能已有创建/retained owner，不推导“未创建”。

## 双门、冻结准入与一次范围

arm 的第一条语句检查 `GOSQLITE_MANUAL_FAULT_ROLLBACK=independently_authorized`，先于 Tx、计划、owner 或路径查询。没有 init 自动安装。tag/env 都不是执行授权。此首片没有可运行 harness，因此这里不提供假装能够启动它的命令。

只接受一个确切、非借用、可写的 plain rollback Tx：排除 session/shared/encrypted、keepFile、borrowedCleanup、readOnly、statement、poisoned、关闭或不健康 cleanup。page/sector 固定 4096；原数据库 extent、最终 page count、dirty 数和 journal originals 各不超过 4；dirty 必须含完整 page 1。准入只检查已有内存状态及 `RetirementComplete` 状态，不调用 checkStable、路径探测或 native I/O；native 自发错误仍原样返回并使计划不合格。

冻结 Plan 逐值保存 suite/source/tool/binary/manifest/trial/scope/profile、不同的 main/journal/directory-scope/cleanup token、目标 point/occurrence/role/offset/len/owner、动作和完整预计调用路径。ID 只是由未来 caller 提供的有界标签，本模型不自行验证源码/二进制真实性。OriginalPages 在计划中指 journal 原页数量，具体 armer 另检查真实 Tx 原 extent 和所有实际原页/dirty offsets。

registry 只有一个 lifetime slot。不是任意函数全局 VFS hook，不跨 Tx 匹配相似路径；不在 registry 锁内执行 owner 查询或 native 调用。即使成功 disarm，used tombstone 不清除，本片不支持进程内第二次试次。pending/unknown/不完整事件保留 scope 与原诊断，拒绝 disarm，不自动 Close 或 RetryPlaintextCleanup。

## 固定动作及原始错误

- Pass：原操作一次，保留原值/原错误。
- FailBefore：选中原操作不执行，固定注入错误，记录 executed=false。DB_PAGE 点在原 `databaseTouched=true` 之后，所以仍按生产 ErrRecoveryRequired 分类。
- PrefixShortWrite：仅四个确切写点，最多调用一次原 `f.WriteAt(b[:k], off)`，0≤k<len(b)。原 helper 继续处理 err 优先以及 n!=len→io.ErrShortWrite，不补写、不重试。k=0 仍记录原 WriteAt 的实际一次调用及返回，不能推断 OS 内部无动作。

前缀 native 返回必须满足真实 n/err，底层非 nil err 不被注入错误覆盖；n 不合计划时收据不能合格。ReadAt 只 pass/fail-before，不伪造 nil-error 短读。记录 native return、adapter return 与 writeFullAt normalized error，原 error 对象另保存在固定 128 槽诊断中。首个真实或注入错误之后不再注入，原 cleanup 路径仍执行。

原生产顺序、分支、poison、errors.Join、temporary 登记/关闭、主 owner 退休和 Tx 锁不变。DB write 已尝试后保留原 journal/RecoveryRequired 分支；final directory sync 失败保留 ErrCommitUncertain；退休失败的 pending/uncertain 不被合成为成功。

## 收据能力与不能证明的内容

顺序为 RunStarted、成对 Intent/Returned、CommitReturned、OwnersObserved、ValidationObserved、Terminal；每项绑定同一 Identity 与连续 ordinal。完整预期路径由协议顺序验证，不接受调用方随意删减的 tiny manifest。漏事件、错 owner/offset/len/occurrence、重复命中、旧 trial/source/binary、超上限、丢原错误或 unexpected success 均拒绝。

`finishRollbackFault` 不调用 Commit。它接受未来 caller 提供的原 Commit 返回对象，在此刻读取已绑定 main/journal/cleanup 的退休状态。逻辑字段是封存时 Tx 状态，不能反推未经观测的 Commit 返回瞬间；外部 caller 的真实单次调用/完整记录仍缺独立证据。实际错误分类和 selected normalized error 的传播使用原 error 对象及 errors.Is，不从预期故障直接编造成功。

完整且合格的纯收据最多得到 AdapterObserved；PhysicalValidated()/Pass() 永远 false。即使 owner 已退休、unlink 已返回、日志 checksum 一致，也没有恢复读回、旧/新账本、普通/扩展索引、已确认 commit ledger 或物理 durability 证据。没有另行审定的真实 native 收据及恢复 verifier 之前不能产生 physical PASS。

## 旧 guard / pin 的 HOLD

仅在 `main.go` 原 all-Go 扫描中增加新 scanner，并在原合同检查之后增加新完整合同检查；所有旧 guard 的规则、exact manual 排除、oracle、Backup、WAL、Darwin 行为保留。`wal_tail_publication.go` 的旧 main.go whole-file pin 原值不改。这意味着当前候选有明确 PENDING_PIN_REVIEW / HOLD；源 guard 未运行，不能把 pin 不匹配称为实际测试 FAIL。独立审阅整个 main wiring 后才可讨论精确单 pin 更新。新 guard 中结构模板/语义否定变体也需独立审阅；pin 不是行为证明。

## A1–A8 仍缺的设施与验收

| 类别 | 本片实际做到 | 仍缺 / NOT_RUN |
|---|---|---|
| A1 I/O/短写 | plain rollback 14 sites 的真实 bridge 与有限动作源码 | native caller/fixture、完整 owner/恢复验证；WAL/encstore 对应 seam；真实运行 |
| A2 Sync | rollback body/header/DB/复合目录点可区分 | directory 内部细分、WAL durable marker/SHM publish、encrypted root、物理设备资格 |
| A3 ENOSPC/权限 | 固定合成 I/O error 可测试错误路径模型 | 不等同真实 ENOSPC；容量/权限/只读环境设施、真实运行 |
| A4 部分/错位/torn | prefix-short-write 软件设施 | 错位/torn/sector/power-loss、WAL frame/marker、authenticated object/root；旧新账本及索引核验 |
| A5 commit/退出/重启 | 无 native controller | child/handshake/forced-exit/reopen/recovery/ack ledger 全部缺独立实现与验收 |
| A6 replay/checkpoint 再中断 | 无绑定 | rollback replay、exclusive/shared checkpoint、encrypted checkpoint；EOFUnproven 不扩大 |
| A7 rekey/upgrade/backup | 无新绑定 | authority/key/archive/immutable/restore 核验；正常升级入口不能代替中断试次 |
| A8 磁盘认证负面 | 无加密覆盖 | wrong-key/salt/tag/ciphertext、跨页/role/DBID/record/parent pairing、nonce 唯一性、磁盘明文 marker 检查 |

六 profile 中只接 plain rollback Commit 子路径；plain exclusive/shared WAL、encrypted snapshot/layout1、exclusive/layout2、shared/layout3 全部 NOT_BOUND。已存在其它专用 publication fixture 不被借用为本片完整设施。

实际动作只有源文件读取、哈希、文本编辑和隔离复制；Go、gofmt、AST runner、source guard、纯测试、编译、oracle、native caller/fixture、controller、recovery、crash、Git、网络、CI 与发布全部 NOT_RUN。ANN V4、41-target fuzz 与旧 UNKNOWN 中断链继续 HOLD/UNKNOWN/BLOCKED，不重启或改名。后续必须先独立源审，再单独审批有限纯验证；完整 physical qualification 仍远未完成。


## 2026-10-12 V2：RF-01 有界身份留存修正（SOURCE ONLY）

V1 独立源码审阅确认默认 14 点、逆还原与原错误顺序；同时指出泛用 Receipt.Append 在校验 Identity 前保存 Event，非法大字符串虽被拒绝仍可能被收据留存。当前 manual adapter 先覆盖成合法计划身份，原可达生产路径没有此输入，但泛用 receipt 的边界必须修正。冻结 V1 和审阅报告不变。

V2 只修此边界：Append 在任何 Event 入库前验证身份长度/内容及同一计划身份。失败时保存专用 EventIdentityRejected，Identity 和其余 payload 全零，仅保留 ProblemIdentity、收到的 ordinal 与实际拒绝槽位 RejectedAt。marker 永远不是协议步骤，外部提交相同 Kind 会被事件顺序拒绝；普通事件夹带 Rejection/RejectedAt 会被 canonical payload 检查拒绝。首次错误保持 sticky，后续 Append 不覆盖 marker，不丢失拒绝位置，也不能把规范化 marker 重新当作成功证据。

合法事件保存前以冻结计划 Identity 替代调用方字符串，避免短 substring 借入大 backing storage；NewPlan 对七个已合法准入的 ID 使用固定 [MaxIDLength]byte buffer 显式复制，再按实际长度转换为新 string。收据自身最多保留七份有界身份字节和固定数组/标量；这不是对调用方总分配、GC 时机或整个进程堆内存的运行测量。

新增纯负例源码覆盖七个字段的 129 bytes / 1 MiB 输入、错误 ordinal 与实际槽位、已有前序事件、第一次拒绝的 sticky 保留、快照改写、外部伪造 marker/普通事件注入 rejection 字段、bad profile/旧身份及大 backing 的合法短 substring。值测试不冒充堆分配测量。新合同两个文件的 supplemental pin 和嵌入模板同步更新；旧 guard、main.go 与 Commit/default/manual 均原字节，旧 main pin 与 HOLD 断言继续保留。

V2 状态 SOURCE_RF01_CORRECTION_PENDING_INDEPENDENT_DELTA_REVIEW / VALIDATION_NOT_RUN。未运行 Go/gofmt/AST/scanner/source guard/测试/编译/native/fixture/controller/recovery/Git/网络。A1–A8、WAL/加密 NOT_BOUND、无 native caller 和无 physical PASS 的全部边界不变；不能将 V1 的有限源码审阅写成 V2 测试通过。
