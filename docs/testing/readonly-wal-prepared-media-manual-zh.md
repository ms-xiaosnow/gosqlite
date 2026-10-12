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

真实只读媒体 A/B 均 **NATIVE_NOT_RUN**；keeper、非特权身份、只读挂载、真实设备/文件系统资格没有从 AST/编译取得。public readonly-WAL 仍为 Linux live initialized-SHM 子集。

下列原文逐字保留，包括作者阶段 SOURCE_ONLY/HOLD、旧 pin 状态和既有命令模板。
这些是历史记录；新状态以上述检查点为准。这里没有生成新命令，任何 tag、环境变量、
模板、编译产物或本说明均不构成 native/物理执行授权。

## 历史手册原文完整保留

# Linux 预备真只读 WAL 介质：手工资格验证边界

日期：2026-10-12 UTC。当前交付：**HARNESS_SOURCE_IMPLEMENTED / SOURCE_ONLY / REVIEW_HOLD**（v2 修正候选，仍待完整独立复审）。

- 原生 profile A：**NOT_RUN**
- 原生 profile B：**NOT_RUN**
- Go、gofmt、源码守卫、测试、编译、二进制与原生调用：本轮全部 **NOT_RUN**
- 新的两个完整源码 pin 尚为 `HOLD_INDEPENDENT_REVIEW`；旧 WAL 发布守卫的整个 main.go pin 保持原值。独立审阅前，这个候选不能通过完整 source contract。

这里的“已实现”只描述源码已写出，不能理解为已编译、已审阅通过或已执行。源码 pin 放行、精确构建方案以及两个原生运行仍各需独立批准。tag、环境变量、manifest、二进制和本文均不构成运行授权。

## 1. 只覆盖什么

入口是 `sqlite/readonly_wal_manual_linux_test.go` 中的 `TestManualReadOnlyWALPreparedMedia`。约束必须恰为 `manual_readonly_wal && linux`，外部测试包是 `sqlite_test`。共享纯模型位于 `readonly_wal_manual_contract_test.go`，只有内存 JSON/profile/阶段/所有权判断；普通测试不打开数据库或观察操作系统。

本入口消费用户另外准备好的健康 `live-ro-wal-4096-v1` 三文件家族和已经持有 live DMS/readmark、已经静止的 keeper。它不创建、修复、复制、恢复、checkpoint 或修改 fixture，不创建或控制 keeper，不设置权限/身份/ACL/mount/namespace/capability，不运行外部进程，不重试 cleanup，不删除任何输入或证据。

两个 mandatory case 是 `keep-false` 和 `keep-true`，顺序固定，直接在同一个 `LockOSThread` 原生测试线程调用。它们不是 Go 命名子测试：`t.Run` 会使用另一个 goroutine，破坏已观察线程身份的前提。任何 case 失败就停止；不能选择或省略其一，也不切换 profile。

A、B 是两次分别批准的单进程结果：

- A：`nonprivileged-denied-v1`。四个源对象必须在该读取者可写 mount view 中，直接 `Faccessat2(..., AT_EMPTY_PATH|AT_EACCESS)` 的 W_OK 结果全部恰为 EACCES。
- B：`readonly-mount-v1`。源目录和三文件 retained descriptor 的 Fstatfs.ST_RDONLY 必须为真，普通 statx mount ID 对应的唯一 mountinfo 记录必须标明 ro。W_OK 可为 EROFS 或 EACCES，按实际值记录。一个 ro bind view 的 superblock 仍可为 rw；不把 reader view 当作底层物理介质证明。

两者均要求观察到的 real/effective/saved/fs UID 相同且非零；GID 相同，附加组一致；capget 的两字 effective/permitted/inheritable 和 proc ambient 全为零，并交叉核对 `/proc/thread-self/status`。两个固定 self-proc 文件都在立即保留 FD/原错误之后、读取之前，要求该 retained FD 的 Fstatfs.Type 恰为 PROC_SUPER_MAGIC；只像 proc 的路径不能作为证据。源、证据与控制目录的文件系统只接受一致的 ext4 或 XFS magic/type；overlay、FUSE、网络文件系统及未知类型均拒绝。缺失 syscall、statx returned mask、mount 记录或实际有效访问结果时不降级。

## 2. 必须另行提供的输入

### 三文件家族

源目录只能包含以下三个 regular、单硬链接、由 manifest 中 reader UID 拥有的不同 inode 文件，且都在目录相同的 reader mount 上：

| 固定名字 | 大小 |
| --- | --- |
| readonly.db | 恰 4096 字节；SQLite 主头 page size=4096、read/write version=2 |
| readonly.db-wal | 32 字节头后至少一帧；每帧 24+4096 字节；总量不超过 1 MiB |
| readonly.db-shm | 恰 32768 字节；keeper 的 live generation 已初始化并有可用 readmark |

逻辑快照固定为三页：UTF-8、schema format 4，`t` 的 root page 2，`ix` 的 root page 3，没有 extension/trigger/view/额外表。SQL 声明必须恰为 `CREATE TABLE t(id INTEGER PRIMARY KEY,text TEXT)` 和 `CREATE INDEX ix ON t(text)`。两行恰为 `(1, 'current')` 与 `(2, 'stable')`。只有一页物理 main 加三页逻辑快照，迫使数据实际来自 WAL。

准备者必须能在另外批准的正常流程中供应该几何。这里没有 fixture 生成器或 keeper 程序；缺输入就停止，不放宽断言，不在 harness 内修补。

### 控制 manifest

仅允许读取三个任务环境变量：

1. `GOSQLITE_MANUAL_READONLY_WAL`：恰为 `independently_authorized`，是第一动作 gate。
2. `GOSQLITE_MANUAL_READONLY_WAL_MANIFEST`：显式提供的绝对、clean、非根 manifest 文件路径。
3. `GOSQLITE_MANUAL_READONLY_WAL_MANIFEST_SHA256`：已经审阅的 exact manifest 字节 SHA-256，小写 64 hex。

没有 HOME/CWD/tmp 默认，没有环境枚举。manifest 文件须为单链接 regular file，不超过 32768 字节，由 reader UID 拥有。控制目录、源目录和证据 parent 必须都是非根目录，且三者两两不同、不嵌套；控制目录是源或证据的祖先时也拒绝。须另外供应符合此前提的目录，本入口不会移动输入或创建控制目录。

消费 manifest 前后分别复核控制目录/manifest 的 retained FD 元数据、控制目录的绝对 pathname 和 manifest basename 相对控制 FD 的 pathname，全部必须仍与读前绑定一致；后续 metadata 观察继续复核。SHA-256 不替代 pathname/FD 身份检查。三目录均以各自普通 statx mount ID、唯一 mountinfo 记录和 Fstatfs 事实推导 mount-root backing place，逐对拒绝同设备上的相等或嵌套 backing 路径及相同目录 inode；同设备某个 backing place 为根目录时，也按祖先关系拒绝。这是有限的命名空间局部检查，不能证明不存在隐藏别名或恶意替换 race。

manifest v1 的以下键全部必填、大小写完全匹配，不允许重复键、null、未知键或额外 JSON 值：

- `version`：整数 1
- `run_id`：本次新 32 小写 hex
- `profile`：上述 A 或 B 的完整名字
- `fixture_profile`：`live-ro-wal-4096-v1`
- `source_root`、`evidence_parent`：不超过 1024 字节的不同绝对 clean 非根目录；禁止彼此嵌套、控制输入混用与明显别名
- `reader_uid`、`reader_gid`：精确 uint32；UID 非零
- `source_tree`：获批候选的 40 小写 hex Git tree identity，不能默认沿用旧 base tree
- `native_source_sha256`、`portable_source_sha256`、`guard_receipt_sha256`、`binary_sha256`：分别为独立审阅/构建记录绑定的 64 小写 hex
- `source_identity`：恰 `{dev_major, dev_minor, inode}`，精确整数
- `family`：固定 main/WAL/SHM 顺序的三个对象，每个恰 `{name, size, sha256, dev_major, dev_minor, inode}`；固定名字/几何，inode 必须与目录及其他成员不同
- `keeper`：恰 `{session, generation, contract}`；session 是 1..64 ASCII 字母/数字/下划线/连字符；generation 是 32 小写 hex；contract 必须为 `already-held-live-dms-readmark-no-mutation-until-operator-release-v1`

解析限制还有深度 8、tokens 512、字符串 1024 字节；哈希和 ID 使用更窄限制，数字禁止浮点/指数/符号/溢出。manifest 不接受 SQL、任意预期行、命令、环境映射、回调或可执行路径。

外部审阅者必须把 source tree → guard receipt → 已构建 binary hash 绑定起来后才批准启动。运行时只检查 allowlisted build metadata；它不能用 manifest 自证其 binary/hash 或源码审阅，也不会读取二进制/任意审阅目录/其他进程来“补证”。Go 版本固定 1.27.2、CGO_ENABLED=0、唯一显式 build tag 为 manual_readonly_wal，模块无 replace、现有版本/sum 固定，trimpath=true；其余可接受 build settings 以源码的严格 allowlist 为准。

## 3. keeper 的四次现状回执

stdin 必须是已经建立的、可信操作员控制、只有这个读取者的 pipe。harness 借用 FD 0，不关闭或改变它。操作员负责 keeper session/generation 的真实绑定；这些现状声明不是密码学认证，也不证明唯一 DMS holder 或系统没有其他 writer。

从第一次 before 回执前直到操作员另行批准释放，keeper 已有合同要求：持续保留初始化 DMS generation/readmark，不执行 SQL、会更新 SHM 的读、写、checkpoint、事务移动、close 或源 namespace 变化。状态回执生产者必须能够不触碰数据库状态地取得现状。harness 不发 release/continue/close 等控制命令。

每阶段输出新 128-bit 随机 challenge。producer 只回一行不超过 2048 字节、以换行终止的 JSON，字段恰为 `{version, run_id, phase, challenge, session, generation, status}`。version=1；run/session/generation 与 manifest 完全一致；challenge 为当前值；status 恰 `still-held-quiescent`。阶段只能依次为：

1. keep-false-before
2. keep-false-after
3. keep-true-before
4. keep-true-after

每行使用 Poll/Read 与 30 秒 monotonic deadline；只有非 owning 读取的 EINTR 在该期限内允许重试。EOF、额外已就绪输入、producer 关闭、错误阶段/nonce、重复/过时回执及超时均停止。不要为了收到 challenge 而改变 keeper 状态。没有 kill、watchdog、receipt 重试或自动 keeper 释放。

## 4. 精确读 API 与 once-retirement

每个 case 用 `OpenContext`，Options 仅为 ReadOnly=true、NoCreate=true、WALShared=true、KeepWALOnClose=该 case、MaxSortBytes=1 MiB。

收到本 case 新鲜 before 回执、完成只读 baseline 后，为该 case 的 SQLite API 序列新建固定 30 秒 `context.WithTimeout`，紧接着注册 `defer cancel()`。Open、DB.Query、Tx.Query、Schema、Inspect、Check、Begin 和 Commit 全部使用这同一个 case context；第二个 case 不复用第一个 context。回执 Poll 的 30 秒 deadline 是另一项独立限制。取消是合作式的，不承诺打断每次内核 I/O，也不形成 hard process timeout、watchdog、kill 或重试权限。

固定顺序：完整 SELECT → Schema → Inspect(IncludeExtensions=true) → Check(MaxPages=16,MaxPageReads=256) → 第一行后显式 Close 的 SELECT → deferred Tx 中完整 SELECT 后 Commit → 第二个 deferred Tx 中第一行后 Close，再 Rollback → DB.Close → pending snapshot 与 source 重观察 → after receipt。

唯一查询是 `SELECT id,text FROM t ORDER BY id`；列名、int64/string 类型和两行值均精确核对。Schema/Inspect 验证固定声明、root pages、SQLNull=false、零 extension、page size/usable size=4096、logical pages=3、版本/encoding/schema format；schema cookie 如实记录。Check 必须得到既定 Scope、Pages=3、Tables=1、Indexes=1、ExtensionIndexes=0、TableRows=2、IndexEntries=2、FreePages=0、ReservedPages=0、0<PageReads<=256。不能自动扩大预算。

所有 acquisition 先保留 ledger reservation，返回后先保留实际 owner 与 original error，再做任何验证或日志；DB.Query 与 Tx.Query 的两个分支也受立即 capture 的结构守卫约束。所有 consuming 调用的 receiver/FD 必须绑定到先行 BeginEnd 的同一个 owner index，不能只凭方法名或调用计数认定安全。nonnil owner 加 error 仍被保留；nil owner 加 error 也不视为已知没有 cleanup token。Schema/Inspect/Check 的内部 owner 无公开对象，因此非 nil 返回错误保守地阻挡 DB retirement。

`Rows.Next(false)` 内部已经调用 cleanup，接下来仅检查 Err，不再次 Close。Next(true) 才回到 LIVE；提前 EOF 是断言失败但不重关，第三个意外 true 的 Rows 仍可一次安全退休。Next panic 的 CONSUMING 状态不能重试。Commit/Rollback/Rows.Close/DB.Close/raw observation Close 都在调用前标为 CONSUMING；返回错误变为 UNKNOWN/BLOCKED，保留原对象/错误，到自然退出也不会补 close 或转用 rollback。

Rows 先于 Tx、Tx 先于 DB、源 observation 文件先于目录、证据文件先于 child 再 parent。未知 child 阻挡 parent；只能一次退休仍已知 live 的独立资源。禁止直接关闭数据库自有描述符、Retry*Cleanup、sweep、finalizer 技巧或把 DB.Close 当子对象兜底。

初始与 case 后均检查 `PendingCloseCleanups`（包含 shared-WAL）和 `PendingPlaintextCleanups`。空 snapshot 不涵盖普通 live/opening group 与所有 failed-open registry，不能声称全局零 owner。因此仍需另外审阅 sealed fresh process、完整测试包/import/init/变量 initializer closure；一个 -run filter 无法隔离初始化。

## 5. 证据、退出与保留

证据 parent 必须预先存在、由 reader UID 拥有、恰 0700，在实际 retained mount 上可读/写/搜索且为 rw。不得 chmod/chown 来通过。source/evidence/control 设备 inode 与 reader mount-root provenance 做局部交叉检查；操作员还必须核对稳定不同位置，不能把这当作所有 namespace alias 的证明。

第一次新鲜 before 回执前的 admission 只观察 credentials、mount/access、目录名字、文件类型/身份/大小与位置分离；不读 fixture 内容，不 hash、不读主头，也不调用 SQLite。admission 记录的 Hashes 为空值，不是已验证内容的声明。第一次 content hash/header baseline 严格在该回执后；不额外消耗或复用回执。

通过 admission 后只允许一次 Mkdirat 创建 `readonly-wal-<run_id>`。EEXIST 就失败，禁止采用/改名/循环换名。candidate 名字在创建前已经保留和输出。子目录中只独占创建初始 0600 的 `events.ndjson` 和 `input-manifest.json`，使用 O_WRONLY|O_CREAT|O_EXCL|O_CLOEXEC|O_NOFOLLOW。写入长度、short write 与 Fsync 都检查。只允许这两个受绑定证据 owner 写；所有 fixture/control descriptor 都是 readonly。

日志包含 manifest 原字节、匹配的观察事实、R/W masks 与 direct Faccessat2 errno、匹配 mount 记录、所有四次 challenge/receipt、固定断言、原生 API 结果和资源退休尝试。不会拷贝 fixture，不保存全部 proc/environment、不转储任意错误字符串。原始 error 对象和 owner 留在进程内 retention ledger。

源文件名字、dev/inode/mount、mode/UID/GID/nlink/size、mtime/ctime 与 SHA-256 在 case 前后核对，排除 atime；API 周围再次核对 held descriptor 和路径绑定。合作稳定前提之外的恶意替换 race、隐藏 alias 或另一个 writer 的不存在均未得到证明。

`events.ndjson` 的 pre-final 记录不是 PASS。证据文件/child/parent 的 Close 结果不能要求被关闭文件自行记下，最终 stdout 才记录全部退休结果。成功候选标记是 `PROFILE_CANDIDATE_PASS_NATURAL_EXIT_REQUIRED`，仍需外部保存 stdout、stderr、自然的整个测试进程 exit status 和根 PASS 才能汇总该 profile PASS。缺 terminal/natural-exit 证据是 INCOMPLETE。stdout 写失败或任何证据关闭失败都不能 PASS。前提拒绝为 PRECONDITION_UNSUPPORTED_NOT_RUN；断言失败为 FAIL；未知 retirement 为 FAIL_OWNERSHIP_UNKNOWN。

保留所有原输入、失败/成功 evidence child、stdout/stderr、自然退出记录；不自动删除、不重用目录、不 kill。仍在运行就只报告当前状态。keeper 由操作员在终态后另行决定释放，本入口永不控制它。

## 6. 后续授权与检查清单

1. 先完成这次候选全部改动、纯模型和 sourceguard 的独立静态审阅；保持旧策略/exemptions 不变。只有审阅实际字节之后才替换新 pins 及旧 main.go 单 pin。
2. 单独批准 Go/gofmt/普通纯验证和完整构建闭包；保留原始输出，任何格式改动都形成新的字节候选并重新审阅受影响 pins。本次未运行这些步骤。
3. 单独冻结实际 Go 工具路径、完整 build argv/environment、现有模块 cache、二进制输出路径/hash、所有 package/init closure 与外部审阅绑定。不得下载安装或自动更换工具链。
4. 分别提供并核对 A 或 B 的健康 fixture、实际身份/mount、已静止 keeper、状态 pipe、独立 evidence parent 与外部 capture。需要 OS 准备、权限/挂载/身份改变时另行提出，本文不授权。
5. 原生二进制的参数及顺序必须恰为：`-test.run=^TestManualReadOnlyWALPreparedMedia$`、`-test.v=true`、`-test.count=1`、`-test.failfast=true`、`-test.parallel=1`、`-test.shuffle=off`、`-test.timeout=0`。不允许额外 skip/list/bench/fuzz/coverage/profile/race/子选择参数或额外 tag。这里故意不给一个暗示已经可运行的 shell 命令。
6. 任一不支持/失败/不确定均停止后续 case；不改变 profile、fixture、身份、权限、预算或环境重试。若后续需要 syscall tracing 证明没有尝试 write-capable acquisition，必须另行审阅其精确范围；字节相等不是 syscall 缺席证明。

最终报告只能说“这个固定 Linux live-index profile，在记录的 reader identity/mount view 和 cooperatively quiescent live generation 区间内通过”。A 不替 B，B 不替 A。冷 WAL/unheld DMS/缺 SHM、其他 page size、其他文件系统/平台、物理不可写、crash/recovery/durability、生产安全与 M3/M8/M10 验收都仍不在该结论内。既有拒绝/受限活动（包括未获准的 ANN V4、41-fuzz、whole-chain）不因此重新开放。
