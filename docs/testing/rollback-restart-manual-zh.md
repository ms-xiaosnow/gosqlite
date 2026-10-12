> 2026-10-12 三设施源码组合状态：本文件下方保留原供体说明与当时状态。当前组合从 main `7503531d2e3d9336bb5943ccd4c836c804d5ce03` 保留既有检查点，接入 shared SQL/A5 源树 `11ade9b1d33f7b317cdd613eed32cce886b6c1e1` 与单页 recovery 供体 `363b5cc7437f58e07722af3fef057fc0518ba309`。shared 最终七项有限续验已 PASS；recovery 供体原十项有限验证已 PASS。它们只适用于各自原始源树，不能算作本次组合 PASS。本组合仍为 SOURCE_ONLY / NOT_RUN，须在完整源审查后另行有限验证。所有实际 manual/native、真实故障、恢复和 process-exit 验收仍为 NOT_RUN；PhysicalValidated / Pass 不因此变为 true。下文旧 CODE_GAP、NOT_RUN 或候选 pin 字样是历史记录，不覆盖此组合状态。

# Plain rollback：有限进程退出／重启手动设施

状态：SOURCE_ONLY / REVIEW_HOLD。本目录中的模型、桥、controller、writer、recovery 都只是源码。Go、gofmt、解析器、纯测试、selection、编译、native、fixture、Commit、退出、ack、recovery **全部 NOT_RUN**。本文不授予执行许可，也不是 A5 或物理持久性验收。

## 唯一范围

Linux；plain rollback；配置 page/sector 均 4096；恰三页 schema/table/普通索引；一个独立父 controller，每个 case 启动一个 writer 和一个全新 recovery/readback 子进程。五个 case 必须分别使用预先建立、空的 0700 私有 root；每个进程只选择一个精确测试根，禁止 case 循环、重跑和目录复用。

| case | 真正停止点 | 退出后的原件 | 新进程结论 |
|---|---|---|---|
| control | 公开 Commit 返回 nil，明确正常 Finish/Disarm/DB.Close，子进程自然退出 0 | NEW，无 journal | 不恢复，完整 NEW SQL/index/Check |
| before-jcreate | 第 1 个 RB.JCREATE Intent，尚未调用原 open | 与 baseline 逐字节相同，无 journal | 不恢复，完整 OLD |
| after-table-page | 第 14 个原调用，即 RB.DB_PAGE 第 2 次 Returned，offset=4096，完整写 4096 字节 | schema/table 已写，index 页仍 OLD，完整 16408 字节 journal | 只恢复独立 replay 副本，恰 3 records/原 extent 3，完整 OLD |
| after-junlink | 第 17 个原调用 RB.JUNLINK Returned，原 unlink 为 nil，final dir Sync 尚未调用 | NEW，无 journal | 不恢复，完整 NEW；不推断断电持久性 |
| acknowledged-exit | 18 个原调用完成，公开 Commit 原样 nil；父完成 ack 文件与目录 Sync/Close 后才发 EXIT | NEW，无 journal，保留持久 ack | 不恢复，完整 NEW，精确绑定同一 ack |

专用成功目标退出码固定 86；它本身不是 PASS。协议中止码 87、自然退出 0、panic、signal、OOM、超时、丢失 frame、错误 owner、未命中点、缺终态都不能替代目标退出证据。控制格和 recovery 必须为真实 Wait 观察到自然退出 0。

不覆盖 WAL、加密、cold journal 自动清理、任意切点、torn write、设备掉电、缓存丢失、checkpoint/replay 再中断或扩展索引。进程退出保留内核页缓存。ack 的 Sync 只持久化测试记录，不给数据库补 Sync。

## 源码入口与隔离

精确 tag 为 `linux && manual_fault_rollback && manual_restart_rollback`。三根：

- `TestManualRollbackRestartController`
- `TestManualRollbackRestartWriter`
- `TestManualRollbackRestartRecovery`

三个根的首句均检查两项独立授权 env；缺门明确拒绝，不能以静默 Skip 充当执行结果。每个根随后按固定顺序要求：精确 `-test.run=^对应根$`、`-test.count=1`、`-test.shuffle=off`、`-test.parallel=1`、`-test.timeout=0`、`-test.cpu=1`、`-test.v=true`。没有额外 flags、正则 union、子测试选择或 shell 包装。

fault-only selection 使用 `linux && manual_fault_rollback && !manual_restart_rollback` 的两个空 observer stub。shipping 默认选择不编译 manual adapter，也不包含任何 restart hook。既有 manual adapter 仅把 Before/After 原函数体原样移到 Core；wrapper 只在 Core 返回、私有 mutex 已释放后调用具体 observer。Commit 原 Tx mutex 仍有意保持。observer 不重锁 Tx，不执行 owner 方法，不接收 callback。

## 独立授权与未来输入资格

尚未执行下列步骤。后续必须先完整独立源审 new main/inspectSource、旧约束传递、所有新闭包，再单独授权有限纯验证／selection／compile-only；native 授权另列。旧 ordinary/full26/CI 权限、tag、环境值和编译成功都不自动涵盖本设施。

controller 的输入必须由独立审阅给定：

- `GOSQLITE_MANUAL_FAULT_ROLLBACK=independently_authorized`
- `GOSQLITE_MANUAL_RESTART_ROLLBACK=independently_authorized`
- `GOSQLITE_RESTART_PRIVATE_ROOT_CONFIRMED=private_stable_namespace_retained`
- `GOSQLITE_RESTART_ROLE` 为 controller 角色的固定数字
- `GOSQLITE_RESTART_ROOT` 为预先属于调用者、0700、空、稳定且祖先无 symlink 的绝对目录
- `GOSQLITE_RESTART_BINDING` 与 `GOSQLITE_RESTART_BINDING_SHA256` 为独立冻结的 canonical JSON 和 hash

binding schema `rollback-restart-binding-v1`，字段全部必需且必须按实际 Go struct 的 canonical JSON 顺序生成，不能重复键、未知字段、不同编码或缺字段：Case、Trial、SourceTree、SourceManifest 及其 Path、Tool 及其 Path、Binary 及其 Path、CompileReceipt 及其 Path、Filesystem、NativeDeclaration。Schema 也在其中。NativeDeclaration 精确为 `linux-native-process-exit-no-powerloss-4096-retain-all`。Filesystem 精确为本 root 原生 statfs 的 `linux-fstype-<hex>`。4096 是明确的受试配置，不是设备扇区自动探测结论。

完整 manifest 必须有限、全文件，声明精确 Git tree、path、mode、bytes、Git blob、SHA-256；source-to-binary compile receipt 的 schema 为 `rollback-restart-compile-v1`，package `./sqlite`，tags `manual_fault_rollback,manual_restart_rollback`，CompileOnly=true，Source 和 Manifest 绑定完整 manifest hash，SourceTree、Files、Tool、Binary 全部一致。compile receipt 只是经独立审阅的输入，不能自行证明来源。当前没有这样的已执行 compile receipt。

每个进程实际打开固定 `/proc/self/exe` 并有界 hash、fstat 前后重检，再与独立审阅的 absolute BinaryPath 的实际 inode/hash 相比。固定 procfs self 入口是唯一 symlink 例外。其他输入必须 regular、single-link、属于调用者、绝对 clean 路径且无 symlink 祖先。procfs 或 identity 不可核实时拒绝，不改用 argv[0]、PATH、同名镜像、任意 executable 或其他机器。

parent 只执行这一个已验证自映像；argv 固定；子环境完整 allowlist，继承 env 不透传；固定 ExtraFiles fd3 控制读端、fd4 事件写端。writer 和 recovery 不能 spawn。PID 来自 parent 实际 Start 返回的 process object；child PID 仅用于交叉核对。generation 与挑战值由父分别生成并绑定，局部 owner ordinal 可以在不同 generation 重复。

## 协议与 ack

闭合值模型的每帧最大 4096 字节 JSON body，加 4 字节大端长度；总帧数 64；adapter 事件上限 128。canonical JSON 验证拒绝未知字段、重复键、尾随 bytes、非规范编码、错 sequence、旧 trial、错 role/generation/challenge，以及超长／截断／丢失帧。

父单向控制 pipe 与子单向事件 pipe 分开。stdout/stderr 是独立有界诊断，使用含原始字节及 overflow 标记的 JSON envelope 保存，绝不解析其 PASS 字符串。每个 pipe 与 owner 都有明确一次 Close；crash child 遗留 owner 仅记为预期进程退出放弃，不伪称库正常退休。

目标取自同一实际 scope/ticket 的新记录事件：先 snapshot 已记录 scalar，再释放私锁，才发协议。parent 在纯前缀模型核验完整 call/phase/offset/length/owner/original result 后才发 ExitGranted；child 再核对当前目标帧 hash，直接 `os.Exit(86)`，不返回 error、不 panic、不走 defer、DB.Close、Rollback、Finish/Disarm 或 retry。

已确认格相邻捕获公开 Commit 原错误对象，必须 nil；完整 18 calls、tx.done/db.tx/Rows 和主/journal/cleanup owner 均闭合。COMMIT_OK 管道帧尚不是 ack。父把原始 canonical COMMIT_OK JSON body 独占写入 `control/ack.json`，检查完整 Write、file Sync、file Close 一次、独立 control dir Sync、dir Close 一次，之后验证内容与 identity，再发送包含同一 ack 的两帧。child 收到完整 AcknowledgementDurable 和 ExitGranted 才退出。任何失败不发预期退出许可。

无 ack 不能推断 OLD。JUNLINK Returned 无 ack 也不代表掉电安全。模型始终 PhysicalValidated=false、Pass=false；一致的人工值输入同样不能成为 native 证据。

## 现场留存与恢复

固定目录：

- inputs：不可变 baseline.input.db 与 binding.input.json
- trial：真实 trial.db，及可能的 trial.db-journal，退出后永远冻结
- evidence：独占 post-exit.input.db、可能的 post-exit-journal.input.bin
- replay：独占 distinct-inode replay.db、可能的 replay.db-journal；仅 recovery child 可修改
- control：plan、baseline SQL、ack、原始协议、stdout/stderr、readback/result、失败及实际 Wait 收据

writer 实际 Wait 以前不读取 trial 用于 copy/recovery。原件、所有 copy、ack、binary/root 的 dev/inode 不得 alias。关键阶段前后核对字节数、SHA-256、逐页 hash、身份、regular/single-link 和稳定 namespace。copy 使用 O_EXCL、完整读写、file Sync/Close、directory Sync/Close。试验原件在退出后只读，不追加 Sync、恢复、删除、覆盖或改名。

只在 after-table-page 的新 recovery 进程上调用一次已审 `manualRollbackSQLRecoverCopy` 闭包：MaxJournalBytes=16408、MaxRecords=3、MaxDatabaseBytes=12288；Recovered=true、RestoredPages=3、OriginalPages=3。replay journal 正常删除，原 trial journal 和 evidence journal 永远保留。首次失败不重试。

baseline/readback 复用既有完整 SQL helper：7 个查询，三列严格 int64/string/string，完整 NOT INDEXED 全行及 blue/red/green 的真实普通索引／NOT INDEXED 对照，穷尽 Rows、Err、Close 一次、SEARCH by_k、索引和基表统计，以及 Check MaxPages=3/MaxPageReads=512 的全部结果。OLD/NEW 比较字面期望，不只比行数或两种访问方式彼此一致。新模型不调用旧 ObserveSQLLedger crash 不适用生命周期。

父拥有两个实际 child 对象且每角色只 Start/Wait 一次。观察期限或 pipe deadline 只产生 CHILD_LIVE/WAIT_UNKNOWN；没有自动 Kill/Signal。失败原收据先保存，若仍活则保持 process object、pipe 与现场，等待真实 Wait，之后另存最终失败收据；不把后来退出改写为最初成功，不开始 recovery。无授权处置时可能持续等待，不能声称已有限结束。

## 后续验证提案（尚未授权、全部 NOT_RUN）

1. 冻结源 inventory、完整 diff、旧 shipping byte identity，独立阅读全部新 native/model/guard 及 whole main/inspectSource；完成前旧 whole-main/body pins 有意拒绝。
2. 单独授权 descriptor-free package `internal/testkit/rollbackrestart` 的具名纯测试根，以及 source guard 的具名 AST/negative-mutant 根；先冻结真实 selector inventory 与计数，不把本手册当执行收据。
3. 单独授权只读 selection：ordinary、oracle、fault-only、Linux 双 manual tag、非 Linux；核对仅双 tag+Linux 含 restart roots／exit bridge，fault-only 只有空 stub。selection 不启动测试二进制。
4. 再单独授权 Linux 双 tag compile-only 到独立输出目录并记录 compiler、source manifest、binary、compile receipt；不得运行该产物，不扩展原项目 workflows/scripts。
5. 若日后要 native，每一格和输入身份、OS/FS/配置、保留目录、管道协议、预期 exit86、父 ack、恢复副本范围及异常处置都须明确独立授权。当前没有执行命令清单，也没有执行许可。

任何普通 check、模型绿色、selection 或 compile-only 都不关闭 A5/M3/M4/M8/M10 完整边界。
