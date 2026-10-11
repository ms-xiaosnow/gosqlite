# Linux 明文 WAL 真只读连接：live-index 有限实现

日期：2026-10-11。当前为 **PARTIAL / Linux 普通与健康 live-writer 已验证**。
实际运行冻结树为 `cace45621c10259231a2e764cfbefdcdbe312ccf`；其完整普通
门为 5,758 PASS / 7 个固定 oracle SKIP，健康官方 live-writer 单根六子项
共 7 named PASS。此前 SOURCE_ONLY / 无 oracle 是作者阶段状态，不能继续
作为本树当前结论。完整 26 项已于 **22:05:15 UTC 自然 PASS**；ordinary/race
各 5,758 PASS / 7 SKIP，完整 pinned oracle 7,431 PASS / 零 SKIP。
原组合四根 FAIL、测试专用四文件修正、输入切片与新树结果分别保留，详见
[本次精确证据](evidence/readonly-ipk-combined-20261011.md)和[当前进展](progress.md)。
原只读 V2 `b19ee07536c83d554f70b40d461b0b96ee4e2872` 的 50 named PASS
仅是其自身有限结果；真实只读媒体/非特权身份和其他原生平台仍未验证。

## 已实现的显式入口

只有 Linux 的 `sqlite.Options{ReadOnly:true, WALShared:true}` 选择此配置。
`ReadOnly` 强制 no-create，即使 `NoCreate:false`；它不会自动识别 WAL。
主库必须是已存在、完整且受支持的 mode-2 文件，`-wal` 与 `-shm` 必须存在。
SHM 必须已由一个正常活动的连接初始化，并在 DMS 探测时存在共享持有者。
通常应让可写 keeper 保持打开；已有只读持有者也能继续维持同一活动代。
关闭最后一个持有者后保留的完整字节不构成初始化许可。

这不是只读 SQL 事务叠加可写打开。实际 main/WAL/SHM 均使用 `O_RDONLY`，
SHM 的 managed mapping 为 `PROT_READ | MAP_SHARED`。没有创建、截断、
数据库模式转换、SHM 重建、readmark 写入、WAL 发布、写锁或 checkpoint。
任何值的 `KeepWALOnClose` 都不会使该连接在 Close 时 checkpoint。
`DB.Checkpoint`、SQL 写入和 Immediate/Exclusive 事务返回
`ErrReadOnly / CodeReadOnly`。普通 deferred 事务被固定为只读，仍可正常
Commit/Rollback；Rows/Tx 仍须及时释放，一条连接只有一个本地活动 lease。

内部 session、index 和 VFS 各自保存只读能力。index 的 Publish、Append、
Cleanup、InitCheckpoint、SetBackfill、SetAttempted、SetReadMark、映射增长
以及 exclusive SHM slot 请求在内存 store / 原生效果之前拒绝。
生产使用的纯决策 helper 和 sourceguard 将这些条件与实际路径关联；
它们不是替代生产路径的另一套测试算法，也没有新增可配置 native-I/O hook。

## 快照、并发和所有权

已有 READ_LOCK(0) 用于 fully-backfilled 快照；否则只复用已有 1..4 readmark。
保护性 readmark 可以小于所捕获 header 的 MaxFrame，但逻辑快照仍使用
该 header 的完整 MaxFrame/DBSize，不会降级成较旧 readmark 的状态。
取得共享 slot 后，依次读取 backfill、原 readmark 并复核完整 header，
得到 minFrame=backfill+1；不匹配则解锁重试。忙等待仍最多 100 轮，保留
context cancellation 与 release 失败 poisoning。无可复用 readmark 时明确拒绝，
不取得 exclusive slot 或写新 mark。

readonly header 校验失败时至多通过 WRITE slot 的共享 probe 判断并发发布；
它不能进入重建分支。稳定未初始化的 index 返回 unsupported；稳定无效 header
保留 corruption/unsupported 分类，发布争用保持 BUSY。

复用已有 public admission、失败打开和 checked-close registry：先预留，再获取；
先释放子映射/SHM，再释放 WAL/main 保护者；清理错误与 pending ownership 保留。
没有 raw descriptor 捷径、finalizer、通用 cleanup 重写或未发布 publisher/Backup 依赖。
main/WAL/SHM 的路径、inode、regular-file、single-link 和映射长度检查保持；
页访问保留 snapshot 范围、WAL 页号和代际 salt 检查。

## 支持与拒绝矩阵

| 组合 | 源码行为与资格 |
|---|---|
| Linux + 显式 ReadOnly/WALShared + 活动且已初始化的 SHM | 有限实现；本树普通及健康官方 live-writer 已 PASS，完整 26 项终态另记，真实只读媒体未验证 |
| ReadOnly，无 WALShared | 仍为原 rollback-only 入口，不自动升级 |
| 冷/无 DMS 持有者、缺 sidecar、未初始化 SHM | 明确失败；不创建、不修复、不信任保留字节 |
| ReadOnly + WALExclusive | unsupported，资源获取前拒绝 |
| ReadOnly + WALShared + WALPowersafeOverwrite | unsupported，写存储断言不能扩大只读能力 |
| ReadOnly + WALShared + KeepWALOnClose 任一值 | 均不 checkpoint |
| Darwin/Windows/其他 OS 的 ReadOnly + WALShared | unsupported，路径/原生获取前拒绝 |
| Encryption 与 native WALShared/WALExclusive 混用 | 保持不兼容；已有独立加密只读能力没有改动 |
| RollbackRecovery 与 ReadOnly/WALShared 混用 | 保持不兼容，无自动恢复 |
| immutable、无 SHM heap-index、冷 WAL 离线检查 | 未实现；没有 fallback |

## API、driver 与实际 CLI 语法

原生入口：`sqlite.OpenContext(ctx, path, sqlite.Options{ReadOnly:true, NoCreate:true, WALShared:true})`。
已配置 `driver.NewConnector("file:existing.db?mode=ro", sqlite.Options{WALShared:true})`
复用同一公共入口。`mode=rw/rwc` 不能放宽调用者显式 ReadOnly；DSN 没有新
WAL 参数，普通 `sql.Open(...mode=ro)` 不会自动启用本配置。

下列是已实现入口的用户模板，**本段模板没有逐条执行**；CLI 路由由本树
普通用例覆盖。它们要求独立正常 keeper
已打开同一 disposable 数据库并完成 SHM 初始化；不能用前一条已退出的
单次 CLI 调用冒充活动 keeper。`live.db` 和 SQL/table 名需要按现有数据替换。

```sh
gosqlite query --wal-readonly live.db 'SELECT id,text FROM t'
gosqlite inspect --wal-readonly live.db
gosqlite inspect --extensions --wal-readonly live.db
gosqlite check --wal-readonly live.db
gosqlite check --wal-readonly live.db grams
```

query 的新模式只接受 SELECT，并在 Open 之前拒绝 DML/事务控制。
`--wal-readonly` 与 `--wal-shared`、`--wal-exclusive`、加密 key/mode flags
互斥；重复模式也拒绝。inspect 可组合 `--extensions`。full Check 与 named
extension Check 均走真只读入口；原 writable flags 的 full Check 拒绝保持。
原 `query --wal-shared/--wal-exclusive` 仍是可写打开，没有改变默认行为。
Check 的成功仍仅代表 [supported profile](native-check.md)，Inspect 仍只报 metadata。

## 精确证据与未完成项

新测试源码覆盖：生产纯策略、源代码 guard、nil-owner mutation 拒绝、close
清理重试模型；Linux keeper 下 512/4096/65536 页的 SELECT/Schema/Inspect/Check；
旧 readmark 与新 header、slot0、有限多 reader/正常 writer、cold/no-create/rollback
拒绝、公共写/checkpoint 拒绝、configured driver 和 CLI 全路由。
名字/长度/SHA-256 前后比较只在 keeper 静止区间进行；并发 writer 区间单独断言
逻辑快照，不把合法 writer 的改动归咎于 readonly。

V2 输入的 sourceguard、精确选择及九条测试命令已在有限 14 门组合中闭合，
23 根/27 子项共 50 named PASS，零 SKIP/FAIL；其中 8 根/6 子项为正常 Linux
选择。compile/list 没有执行测试体。这些输入结果不传递为当前新组合 PASS。
本树 `oracle && linux` 的 `TestOracleLiveWriterReadOnlyWALSnapshots`
已于 21:36:42 UTC 自然 exit 0：一根六子项全部 PASS，覆盖三种页大小与
两种 KeepWALOnClose；七个官方进程为一次身份查询和六个正常结束的 peer。
这项新执行不再标为无 oracle；同树完整 26 项终态（含完整套件）见本次精确证据。
测试只使用健康库、正常 commits 和正常退出，保留成功/失败 fixture；在 peer
静止区间比对文件名/长度/内容，分别断言 Tx/Rows 快照，不声称 atime 或锁状态。
现有正常 writable 互操作不能移作此配置的资格。
本批没有任何 chmod、权限变更、namespace 替换、损坏/截断物理 fixture、fault、
kill/crash、强制关闭或 recovery 工作流。真实只读媒体与非特权用户的验证另列
**NOT_RUN / 需真实环境**，没有虚构一个已实现的手工 harness 或现成命令。
V2 原有 `t.TempDir` 正常测试不承诺错误现场留存；新增 live-writer 测试单独保留目录。

契约是本库不写这些文件的内容/名称，不保证 OS 不更新 atime。
DMS 与合法 SHM 是合作协议的信任前提，不是认证，也不抵御恶意 peer、
同名替换或任意外部 mmap 截断竞争；每次读取不重算整个 WAL checksum chain。
M3 crash/durability、其他原生平台、安全审查和 M10 仍未验收。
