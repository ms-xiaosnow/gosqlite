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

真实 publication 与 elapsed 样本均 **NATIVE_NOT_RUN**。计时合同是总公开调用 entry-to-return 延迟，含扫描、锁、发布和 constructor 退休；不是 recovery-only/fsync-only，也没有 cold-cache 或 P95 结果。

下列原文逐字保留，包括作者阶段 SOURCE_ONLY/HOLD、旧 pin 状态和既有命令模板。
这些是历史记录；新状态以上述检查点为准。这里没有生成新命令，任何 tag、环境变量、
模板、编译产物或本说明均不构成 native/物理执行授权。

## 历史手册原文完整保留

# Linux 独占 WAL 尾部发布：四场景公开入口手工验收

## 2026-10-12 elapsed 六路径候选：SOURCE_ONLY / HOLD

本次仅补齐现有唯一手工入口的有限耗时采集和纯值准入合同。没有运行 Go、gofmt、
parser/AST runner、编译、源码 guard、纯值测试、手工测试或任何 physical/native 场景。
所有真实物理场景仍 **NOT_RUN**。旧 active manual pin 保留，新 manual/pure 全文件 SHA
仅列于 proposed 表，guard 明确 **HOLD**；必须先独立完整源码审阅，再决定新的纯模型/
编译范围。旧树 PASS、本文模板与 proposed hash 均不授予执行许可。

### 采集范围和计时定义

- 原 tag/env 双门不变，env 仍为测试第一条语句。保留唯一根和四个原场景，精确 selector、
  count=1、timeout=0，并在原 flag gate 要求 -test.v=true 以保留日志；不增加独立 runner、benchmark root、自动重复、进程或 controller。
- 在 options、context 和 NewConnector 纯配置准备完成后，紧邻实际公开调用放置
  time.Now → Open / OpenContext / Connector.Connect → time.Since。返回后的第一条
  语句就是 time.Since，登记 owner、JSON、查询、普通写、Close、重开均在计时外。
- publication 字段为 public_open_with_tail_publication_elapsed_ns，int64 纳秒；
  timing_boundary 固定为
  public_call_entry_to_return_including_scan_lock_publication_and_constructor_retirement。
  它包含策略、canonicalization/admission、锁、扫描、候选写/Sync/替换和 constructor
  内退休，是总公开调用延迟。没有内部阶段 hook，不能写成 scan/publish/fsync/recovery-only
  耗时，也不能减去普通 reopen 来推导这些值。
- 三个 nil-policy 对照只记 refusal_attempt_elapsed_ns，始终 qualified=false、n=0。
  正常 INSERT 和 nil-policy reopen 仍是原功能断言，不增加恢复耗时样本。
- 每个入口每 invocation 恰好一个 publication 样本，各自 n=1。不将三个入口合并为 n=3
  分位数，不提供 P95 或性能回归结论。未来任何人工重复需另批新 root/日志，固定全部身份。
- case_end_to_end_elapsed_ns 从该 case 开始、fixture 准备前，到原断言和 rows/DB/Conn/
  observation owners 一次退休、保留输入复核完成；目录仍由 root 退休。
  invocation_end_to_end_elapsed_ns 从 env gate 后至完整四场景、root 退休和内存判定完成，
  不含最终 JSON/日志输出。这两个值也不能当作纯恢复延迟。UTC 仅关联，duration 来自同进程
  time.Now/time.Since 单调差，不能用 UTC 字符串相减替代。
- fixture 已写入、Sync 并读取核验，OS cache 为 warm/unspecified，无 eviction，绝不声称 cold。

### 有界身份和输入来源

qualification JSON 新增 elapsed_metadata。只填以下外部声明字段：
source_manifest_sha256_declared、binary_sha256_declared、qualification_evidence_sha256_declared、
build_flags_declared、hardware_declared、cpu_declared。三个 SHA 为小写 64-hex；hardware/CPU
为非空可打印 ASCII、每项最多 256 bytes；flags 必须是
CGO_ENABLED=0 GOFLAGS=-buildvcs=false tags=manual_wal_publication。
源码 manifest 和实际 binary 必须由外层独立测量并保存原件；声明不是被测程序自认证。
profile 的完整 SHA、实际 Go/OS/architecture/ext 类型、既有 sector 声明和固定 cache 标签
由 harness 填入报告；原 qualification.input.json 全字节保留。

工具链固定 Go 1.27.2。manifest 须明确来源 commit/tree、逐路径/长度/SHA 和被审阅的六路径
候选；外层保留实际编译 argv、go env 和二进制 SHA，将声明逐项核对，缺一不认证结果。
硬件/CPU/storage/fsync 资格仍须独立证据，标签或定时器不能证明断电持久性。

每个 attempt 同时保存固定 fixture ID、input-sha256.json 的 dataset digest、main/WAL/
expected/ledger SHA、页/frame/字节数、little-endian、table rowid/IPK 范围。
seed=not_applicable_deterministic_fixture_v1；input_provenance=
synthetic_encoded_committed_prefix，pre_crash_commit_ack=not_present。
合成 commit-marker 不能冒充崩溃前真实 Commit acknowledgement，也不是业务 crash artifact；
当前 ledger 不证明 secondary/extension indexes、ANN 或 grams 一致性。

### Raw attempt、最终准入与完整日志

每次公开调用返回后输出一行 WAL_ELAPSED_ATTEMPT JSON，始终未准入。它保留实际调用/
返回/handle/error/elapsed；nil error 本身不代表成功。错误摘要最多 1024 bytes，若截断即
不能准入，原错误仍由既有诊断日志保留。raw 行最多 16 KiB，不新增结果文件 writer。
未执行槽位在最终报告明确 NOT_RUN，attempted=false，零值不能当作零耗时成功。

唯一 WAL_ELAPSED_REPORT JSON 位于 root cleanup：原 checkPending、retire(3) 和最后一次
pending 检查完成之后。它要求恰好六个唯一 attempt、完整四场景、全部原 bytes/inode/scan/
ledger/普通写/重开/保留输入断言，以及 Rows→DB/Conn→observation fd→目录的所有一次 Close
成功，无 pending token、Close diagnostic 或 sticky failure。任意失败/未知/不完整一律不给
任何 publication qualified=true。第二次 nil Close 不能擦掉第一次错误，阻断后的新资源和
诊断读文件仍禁止。原 held fd 生命周期和所有保留材料不变。

序列化先作完整有界检查；最终报告最多 128 KiB。成功形态只称
QUALIFIED_CANDIDATE_EXTERNAL_TERMINAL_REQUIRED，并明确 external terminal required。
最终 JSON 不单独证明完整输出，更不证明进程自然结束。只有外层同时取得自然 exit=0、
完整 stdout/stderr、唯一最终报告、顶层+恰好四子项 PASS、恰好六行 raw attempts、三个
publication 各 qualified=true/n=1、三个对照 n=0、无 FAIL/SKIP/UNKNOWN/截断、完整原件
和来源/二进制/硬件绑定，才可接纳一次测量。缺最终报告、半截 JSON、挂起或非自然退出
一律 UNKNOWN；不从局部 JSON 抽出“成功”时间。所有失败材料同样保留，不覆盖或复用。

### 纯值六根和下一验证边界

sqlite/wal_tail_elapsed_contract_test.go 仅 testing import、scalar/struct/slice 数据和纯 reducer，
无 clock、sqlite/native/os、fixture、Open/Close callback。六根名称为：

1. TestWALPublicationElapsedMemoryQualifiedSample
2. TestWALPublicationElapsedMemoryRejectsUnexecutedOrPartial
3. TestWALPublicationElapsedMemoryRejectsFailureAndUnknown
4. TestWALPublicationElapsedMemoryRequiresRetirement
5. TestWALPublicationElapsedMemoryKeepsEntrancesSeparate
6. TestWALPublicationElapsedMemoryRejectsInvalidTiming

这些根尚未运行。即使以后纯值测试通过，也只验证所给证据的准入，真实 Now/Since 邻接和
调用覆盖依赖源码审阅，实际延迟和物理资格依赖另行授权的现有手工入口。
独立 crash acknowledgement、interrupted/restart recovery controller/fixture/receipt，以及
其它明确支持 recovery profiles 的计时合同仍是独立 CODE_GAP/未验收；本切片不能声称
全部 WAL recovery、§11.3 或 M3/M10 已完成。benchmarks 索引见
[有限 WAL tail-publication elapsed](../../benchmarks/wal-tail-publication/README.md)。

以下历史状态按原身份保留。手工模板仍全部 NOT_RUN；新增 metadata 使旧 qualification
JSON 单独不再足够。准备时先完成隔离，再在独立授权范围完成第 4 节编译及 binary hash，
然后才运行第 3 节 profile 生成模板（它现在要求外部提供 metadata）。本顺序不授权执行。

> 2026-10-12 当前组合状态：Storage metadata 修正树 `7389e7b7` 的三条回归
> 已实际 PASS（14 根 / 92 named PASS），UNIQUE 树 `fd4e3471` 的十二条有限门
> 已实际 PASS（31 根 / 223 named PASS / 零 SKIP）；两者均自然退出并释放 lease。
> 本文所在新 Storage + UNIQUE 组合仍是 **SOURCE_ONLY / NOT_RUN**：完整 ordinary、
> race、oracle、跨编译/依赖、发布/精确恢复/对应 CI 尚未运行。下文旧的
> SOURCE_ONLY、PENDING、FAIL 与 NOT_RUN 均保留为各自身份的历史，不覆盖上述有限结果。
> 原 V1 显示契约 FAIL 与 V2 metadata 真失败（8,126 PASS / 1 FAIL / 7 SKIP；
> 后续 19 NOT_RUN）不改写。native Backup（包括 UNIQUE 数据库）、WAL publisher
> 和物理资格仍 **NOT_RUN**；完整目标与 M10 未验收。详见[新检查点证据](../evidence/storage-unique-checkpoint-20261012.md)。

## 2026-10-11 当前源码与执行边界

本页四场景公开 Open / OpenContext / Connector 手工 harness **源码已存在**；
实际 native publisher、设备/文件系统资格和四场景运行仍为 **NOT_RUN**。
完整物理故障、跨进程锁连续性、崩溃/断电/恢复设施仍为 **IMPLEMENTATION_MISSING**。

WAL 输入树 `3f772f7e18e59e3061b72ab8b37ee1953e55ddab` 已取得 5 条有限命令
PASS、35 根 / 100 named PASS。这里的 source/model 结果没有执行本页手工测试
二进制，也不能把新手工 harness 的编译或原生运行记作 PASS。冻结组合树
`2602cc60e8c82e3c7f900eb44439e4217cb0b8c3` 的完整普通 26 门 V2 在 ordinary 门
**FAIL**、后续 19 门 NOT_RUN；修正后新树/完整复验 **PENDING**。默认/oracle
构建继续排除此 manual 入口。

以下原日期、SOURCE_ONLY/NOT_RUN、旧 34 根 / 99 PASS 与 heredoc 事故叙述原样
保留为历史记录；新的 35 根 / 100 PASS 只属于上面精确输入树。本文全部命令、
环境门、资格前置、失败保留和清理步骤仍保留且未执行；elapsed metadata 补充见本页开头。详情见
[Storage 证据补记](../evidence/storage-checkpoint-20261011.md)。

### 历史首次准备说明（原文保留）

2026-10-11 UTC。状态：本次新增 **SOURCE_ONLY** 手工 harness 源码，尚未取得
parser、编译、源码守卫或任何物理场景的验证结果。下述手工验收序列仍为 **NOT_RUN**，
需独立审阅和单独执行授权。本页的命令不是当前受限会话的执行许可。
文档写入曾发生 heredoc 事故并误触发两个失败的系统 go 调度；独立影响审查未发现
测试/编译/native 场景执行或源码残留修改。事故不是测试结果，也不计入验证证据。
历史 WAL v2 的 34 个模型根 / 99 个具名 PASS 不能替代此处真实公开入口证据。

实际入口为 `sqlite/wal_tail_publication_manual_linux_test.go` 的
`TestManualPublicWALTailPublication`。它通过公开 `sqlite.Open`、
`sqlite.OpenContext` 和 `driver.NewConnector(...).Connect(...)` 启用
`Options.WALTailPublication`。没有修改生产代码，没有新增 DSN、CLI、环境自动启用
生产策略，也不直接调用内部 publisher、VFS factory 或恢复入口。

## 1. 四个有限场景与明确边界

1. `nil-default-preserves-input`：同一组物理 main/WAL 输入依次通过 Open、
   OpenContext、Connector.Connect；nil policy 均返回 nil handle、公开 CodeError、
   ErrUnsupported 与 WAL ErrTail，main/WAL 全部字节和 inode 不变，目录无新增项。
2. `open-publishes-writes-reopens`：Open 显式 opt-in；完整发布后检查旧 main inode/
   字节、旧 WAL descriptor 字节、新 WAL 不同 inode、完整 checksum-valid committed
   prefix、预期字节和目录项；查询提交账本，正常 INSERT，再用 nil-policy OpenContext
   重开并查询相同提交账本。
3. `open-context-publishes-writes-reopens`：与第二项相同，但首次打开为公开 OpenContext。
4. `connector-publishes-writes-reopens`：首次为真实 Connector.Connect；直接使用
   database/sql/driver 的 QueryerContext / ExecerContext，不使用连接池、懒连接或
   NewConnector 的纯配置结果代替实际 Connect。关闭后仍用公开 nil-policy 重开。

四项使用相同确定性字节、各自独立目录。fixture 是合法两页 SQLite mode-2 主库，
包含 sqlite_schema 和 `t(id INTEGER PRIMARY KEY, value TEXT NOT NULL)`：
- main 内 row 1 = `base`
- WAL 两个已提交 frame 提供 page 1 与 page 2，提交账本只有 `(1, committed)`
- 最后一个完整、checksum-valid、DBSize=0 的未提交 frame 另带 `(2, uncommitted)`
- 发布后必须只见 `(1, committed)`；普通 INSERT 后必须恰见 `(1, committed),(3, after)`

固定 little-endian checksum、4096-byte page、4096-byte sector assumption、一个已提交
事务、一个未提交 frame、无 SHM/journal。用现有 record/WAL 纯格式编码器构造 fixture，
用固定行账本和独立构造的完整预期 WAL 字节检查结果，不启动 SQLite CLI/oracle 进程。
预期构造不调用生产 PrefixReplacement 计划/写出函数。当前有限范围不覆盖 big-endian、
512–65536 全范围、零/多/缩小 commit、已有 stale SHM、候选碰撞和完整约束矩阵。

不会注入 crash、kill、断电、torn write、short/full-write error、Sync/close/attach error、
命名空间干扰或敌对攻击。也没有独立进程 EXCLUSIVE 连续性/掉电持久性证明。
旧 WAL descriptor 的前后检查证明观察到的字节未变，不声称覆盖扫描期间每一时刻。
完整 native fault/recovery/中断发布设施仍 **IMPLEMENTATION_MISSING / NOT_RUN**。
此四项即使将来 PASS，也不能升级为完整 M3/M8/M10 或生产安全资格。

## 2. 必须先独立确认的环境

- 使用没有业务数据的私人 Linux 测试环境；固定 Go **1.27.2**、CGO=0、go.mod/go.sum
  中的依赖，禁用自动 toolchain 下载和在线模块查找。记录源码 commit/tree、完整路径
  哈希、工具版本和实际二进制哈希。原工作树须干净，运行使用单独复制的源码。
- root 为全新、空、当前有效 UID 所有、0700 的绝对 ASCII 目录；路径每个组件都不许
  是符号链接。仅允许路径字符 A–Z/a–z/0–9、`/._-`。fixture 位于私人 0700 RUN 中。
  不使用业务 DB、共享目录或已有失败 root；所有输入和结果均保留，包括 PASS。
- 实际文件系统须为 ext2/ext3/ext4，返回 EXT4_SUPER_MAGIC，读写挂载；tmpfs、overlay、
  network fs、volatile、nobarrier、barrier=0 均不能替代。固定 `/proc/self/mountinfo`
  的实际完整行，核对 device/source、挂载选项、内核、架构和有效 UID。
- 必须实际返回 statx 所需字段，尤其 **STATX_MNT_ID_UNIQUE**，并实际支持
  Openat2 的 RESOLVE_BENEATH | RESOLVE_NO_SYMLINKS | RESOLVE_NO_MAGICLINKS |
  RESOLVE_NO_XDEV。根目录的 Openat2(".") 探测只证明该次能力/身份准入，不是
  对恶意祖先替换竞态的保护；生产候选仍使用完整 resolve 约束。
  只检查 kernel 版本或 ext magic 不够；不回退旧 mount ID。
- 4096-byte sector assumption 必须不小于实际相关设备扇区；独立确认可信持久设备、
  控制器/虚拟化层、cache 和 fsync 的真实语义，禁止 fsync=volatile。保留可查证的设备
  与测试环境说明。JSON 的 qualification 字符串只是人的明确声明，不能自动证明硬件。
- 可信、合作式命名空间，无其他进程访问 fixture 或替换其祖先；0700 和 statx 不是对
  隐藏别名或敌对命名空间的完整防御。若这些前提不能确认，停止，不换挂载/机器求 PASS。
- fixture/source 所在文件系统至少 2 GiB 可用空间和 10000 free inode；独立检查实际
  GOTMPDIR 至少 1 GiB 和 10000 free inode。保留设备、挂载、空间和 inode 证据。

双门为 `manual_wal_publication && linux` build constraint，加测试函数第一条语句的
`GOSQLITE_MANUAL_WAL_PUBLICATION=independently_authorized`。无环境门直接 FAIL，
不以 SKIP 冒充完成。运行还必须使用精确顶层选择器、count=1、timeout=0、verbose=true，禁止子项筛选、
重复 CPU 组、benchmark 和 shuffle。CI/default/oracle 不应选择此文件。
`internal/testscope` 仅对精确文件路径 + 完整 SHA256 允许 public option 引用；
旧 production pins、惰性模型例外和所有私有/native factory 检查保持不变。

## 3. 隔离准备与资格材料（全部 NOT_RUN）

下面是需要人工执行的真实命令模板。先在已独立审阅、已确认的干净仓库根目录，设置
REVIEWED_COMMIT、REVIEWED_TREE 和一个已经完成上节设备审查的私有 ext-family 父目录
QUALIFIED_PARENT。不能将当前未审阅源码的 HEAD 自动当作审阅结果；这份 source-only
候选尚没有给出物理平台合格结论。不要用 /tmp 的默认挂载猜测资格。

```sh
set -eu
umask 077
: "${REVIEWED_COMMIT:?set independently reviewed commit}"
: "${REVIEWED_TREE:?set independently reviewed tree}"
: "${QUALIFIED_PARENT:?set independently qualified private ext-family directory}"
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOENV=off GOWORK=off
export GOFLAGS=-buildvcs=false GOMAXPROCS=2 CGO_ENABLED=0
unset GOSQLITE_MANUAL_WAL_PUBLICATION GOSQLITE_MANUAL_RECOVERY GOSQLITE_MANUAL_NAMESPACE
unset GOSQLITE_MANUAL_BACKUP_PUBLICATION
SOURCE=$(pwd -P)
test -z "$(git status --porcelain)"
test "$(git rev-parse --verify HEAD)" = "$REVIEWED_COMMIT"
test "$(git rev-parse 'HEAD^{tree}')" = "$REVIEWED_TREE"
test "$(go version | awk '{print $3}')" = go1.27.2
test "$(uname -s)" = Linux
RUN=$(mktemp -d "$QUALIFIED_PARENT/wal-public-manual.XXXXXX")
mkdir "$RUN/source" "$RUN/evidence" "$RUN/fixtures" "$RUN/gotmp"
export GOTMPDIR="$RUN/gotmp"
EVIDENCE="$RUN/evidence"
FIXTURES="$RUN/fixtures"
PROFILE="$RUN/qualification.json"
git archive "$REVIEWED_COMMIT" > "$EVIDENCE/source.tar"
tar -xf "$EVIDENCE/source.tar" -C "$RUN/source"
printf '%s\n' "$REVIEWED_COMMIT" > "$EVIDENCE/commit.txt"
printf '%s\n' "$REVIEWED_TREE" > "$EVIDENCE/tree.txt"
git status --porcelain > "$EVIDENCE/original-status.txt"
go version > "$EVIDENCE/go-version.txt"
uname -a > "$EVIDENCE/kernel.txt"
cat /proc/self/mountinfo > "$EVIDENCE/mountinfo.before.txt"
findmnt -T "$FIXTURES" > "$EVIDENCE/mount.txt"
df -Pk "$RUN" "$GOTMPDIR" > "$EVIDENCE/disk.txt"
df -Pi "$RUN" "$GOTMPDIR" > "$EVIDENCE/inodes.txt"
printf 'source=%s\nrun=%s\nevidence=%s\nfixtures=%s\n' "$SOURCE" "$RUN" "$EVIDENCE" "$FIXTURES"
cd "$RUN/source"
go env GOOS GOARCH CGO_ENABLED GOCACHE GOPATH GOTMPDIR > "$EVIDENCE/go-env.txt"
go mod verify > "$EVIDENCE/modules.txt"
```

模块缓存或必要工具缺失就停止，不自动取消离线限制。核对上节资格和资源门后，将设备/
控制器/cache/fsync/sector/独立命名空间依据写入 `$EVIDENCE/storage-qualification.txt`。
至少写清证据来源、设备/虚拟化层、实际最大相关 sector、审核人和审核日期；一句“已确认”
不能替代调查。只有你确实完成该审查后，才创建下面的显式资格 JSON。该准备代码只捕获
你所审查的实际参数，不自行认证底层存储。

```sh
test -s "$EVIDENCE/storage-qualification.txt"
export GOSQLITE_PROFILE_ARCH="$(go env GOARCH)"
# 先由外层完成源 manifest、实际编译/binary hash 和硬件核验，不让 harness 自认证。
: "${GOSQLITE_ELAPSED_SOURCE_MANIFEST_SHA256:?set externally verified source manifest SHA256}"
: "${GOSQLITE_ELAPSED_BINARY_SHA256:?set actual reviewed test binary SHA256}"
: "${GOSQLITE_ELAPSED_HARDWARE:?set bounded verified hardware description}"
: "${GOSQLITE_ELAPSED_CPU:?set bounded verified CPU description}"
export GOSQLITE_ELAPSED_SOURCE_MANIFEST_SHA256 GOSQLITE_ELAPSED_BINARY_SHA256
export GOSQLITE_ELAPSED_HARDWARE GOSQLITE_ELAPSED_CPU
python3 - "$FIXTURES" "$PROFILE" "$EVIDENCE/storage-qualification.txt" <<'PY'
import hashlib, json, os, pathlib, sys
root, output, evidence = map(pathlib.Path, sys.argv[1:])
assert root.is_absolute() and str(root.resolve()) == str(root)
assert not list(root.iterdir())
assert root.stat().st_uid == os.geteuid() and root.stat().st_mode & 0o7777 == 0o700
matches = []
for line in pathlib.Path('/proc/self/mountinfo').read_text().splitlines():
    fields = line.split()
    if len(fields) < 10:
        continue
    mount = fields[4]
    if str(root) == mount or str(root).startswith(mount.rstrip('/') + '/'):
        matches.append((len(mount), line))
assert matches
mountline = max(matches)[1]
filesystem = mountline.split(' - ')[1].split()[0]
assert filesystem in ('ext2', 'ext3', 'ext4'), filesystem
raw = evidence.read_bytes()
assert raw.strip()
profile = dict(version=1, root=str(root), kernel_release=os.uname().release,
    architecture=os.environ['GOSQLITE_PROFILE_ARCH'], uid=os.geteuid(),
    mountinfo_line=mountline, sector_bytes=4096,
    storage_evidence=f'{evidence.name} sha256={hashlib.sha256(raw).hexdigest()}',
    qualification='independently_verified_durable_private_ext_storage',
    elapsed_metadata=dict(
        source_manifest_sha256_declared=os.environ['GOSQLITE_ELAPSED_SOURCE_MANIFEST_SHA256'],
        binary_sha256_declared=os.environ['GOSQLITE_ELAPSED_BINARY_SHA256'],
        qualification_evidence_sha256_declared=hashlib.sha256(raw).hexdigest(),
        build_flags_declared='CGO_ENABLED=0 GOFLAGS=-buildvcs=false tags=manual_wal_publication',
        hardware_declared=os.environ['GOSQLITE_ELAPSED_HARDWARE'],
        cpu_declared=os.environ['GOSQLITE_ELAPSED_CPU']))
with output.open('x') as f:
    json.dump(profile, f, indent=2)
    f.write('\n')
    f.flush()
    os.fsync(f.fileno())
assert output.stat().st_mode & 0o7777 == 0o600
print(json.dumps(profile, indent=2))
PY
```

## 4. 精确选择和编译检查（全部 NOT_RUN）

先完成独立全文件源码审阅，尤其新 full-file pin、双门、资源退休顺序、依赖与 init。
以下只选择来源/运行纯源码守卫/编译，不启动物理测试二进制。guard PASS 也不是执行许可。

```sh
go test -p=1 -vet=off ./internal/testscope \
  -run '^TestWALPublicationGuardManualSourceIsExactAndDoubleGated$' -count=1 \
  > "$EVIDENCE/manual-guard.log" 2>&1
go run ./internal/testscope > "$EVIDENCE/source-guard.log" 2>&1
go list -f '{{join .XTestGoFiles "\n"}}' ./sqlite > "$EVIDENCE/default-files.txt"
go list -tags oracle -f '{{join .XTestGoFiles "\n"}}' ./sqlite > "$EVIDENCE/oracle-files.txt"
go list -tags manual_wal_publication -f '{{join .XTestGoFiles "\n"}}' ./sqlite > "$EVIDENCE/manual-files.txt"
GOOS=darwin go list -tags manual_wal_publication -f '{{join .XTestGoFiles "\n"}}' ./sqlite > "$EVIDENCE/darwin-manual-files.txt"
GOOS=windows go list -tags manual_wal_publication -f '{{join .XTestGoFiles "\n"}}' ./sqlite > "$EVIDENCE/windows-manual-files.txt"
python3 - "$EVIDENCE" <<'PY'
import pathlib, sys
root = pathlib.Path(sys.argv[1])
name = 'wal_tail_publication_manual_linux_test.go'
for file, expected in [('default-files.txt', 0), ('oracle-files.txt', 0),
        ('manual-files.txt', 1), ('darwin-manual-files.txt', 0), ('windows-manual-files.txt', 0)]:
    actual = (root / file).read_text().splitlines().count(name)
    assert actual == expected, (file, actual, expected)
print('manual source selection PASS; physical runtime NOT_RUN')
PY
go test -c -p=1 -vet=off -tags manual_wal_publication \
  -o "$EVIDENCE/wal-public-manual.test" ./sqlite
go tool nm "$EVIDENCE/wal-public-manual.test" > "$EVIDENCE/symbols.txt"
grep -F 'sqlite_test.TestManualPublicWALTailPublication' "$EVIDENCE/symbols.txt"
sha256sum sqlite/wal_tail_publication_manual_linux_test.go \
  sqlite/wal_tail_elapsed_contract_test.go \
  internal/testscope/wal_tail_publication.go internal/testscope/wal_tail_publication_test.go \
  go.mod go.sum "$EVIDENCE/wal-public-manual.test" > "$EVIDENCE/source-binary-sha256.txt"
```

默认/oracle/非 Linux 列表必须排除该文件；Linux manual 恰选一次。新 tag 不得进入项目
CI、GOFLAGS 或普通脚本。不要使用 `go test -tags manual_wal_publication ./...`，不要添加
oracle/manual_recovery/manual_namespace/其他 manual 标签，也不要调用旧 held scripts、
ANN V4、41-target fuzz/campaign 或未知中断链路。本次实现没有触碰这些保留范围。

## 5. 独立授权后的一次实际运行（NOT_RUN）

审阅已完成、资格已确认且你获准执行之后，再核对当前 RUN/PROFILE/FIXTURES 的唯一路径。
失败后不得重用该 root、降低期望或转移执行器求通过。timeout=0 是为了避免测试框架自动
中断持锁发布过程；若调用挂起，保留 UNKNOWN，任何强制终止另行决定。

```sh
printf '%s\n' 'GOSQLITE_MANUAL_WAL_PUBLICATION=independently_authorized' \
  "GOSQLITE_MANUAL_WAL_PUBLICATION_DIR=$FIXTURES" \
  "GOSQLITE_MANUAL_WAL_PUBLICATION_PROFILE=$PROFILE" \
  "$EVIDENCE/wal-public-manual.test -test.run ^TestManualPublicWALTailPublication$ -test.count=1 -test.parallel=1 -test.shuffle=off -test.v -test.timeout=0" \
  > "$EVIDENCE/runtime-command.txt"
date -u +%FT%TZ > "$EVIDENCE/runtime-start.txt"
set +e
GOSQLITE_MANUAL_WAL_PUBLICATION=independently_authorized \
GOSQLITE_MANUAL_WAL_PUBLICATION_DIR="$FIXTURES" \
GOSQLITE_MANUAL_WAL_PUBLICATION_PROFILE="$PROFILE" \
"$EVIDENCE/wal-public-manual.test" \
  -test.run '^TestManualPublicWALTailPublication$' -test.count=1 -test.parallel=1 \
  -test.shuffle=off -test.v -test.timeout=0 \
  > "$EVIDENCE/runtime.stdout" 2> "$EVIDENCE/runtime.stderr"
RC=$?
set -e
printf '%s\n' "$RC" > "$EVIDENCE/runtime-exit.txt"
date -u +%FT%TZ > "$EVIDENCE/runtime-finish.txt"
cat /proc/self/mountinfo > "$EVIDENCE/mountinfo.after.txt"
cat "$EVIDENCE/runtime.stdout" "$EVIDENCE/runtime.stderr"
printf 'exit=%s evidence=%s fixtures=%s\n' "$RC" "$EVIDENCE" "$FIXTURES"
```

PASS 要求自然退出码 0；顶层和恰好四个上述子测试全部 PASS，三次 nil-default 拒绝、
三个真实入口发布、查询字节/账本/旧新 inode/目录/Close/pending-owner 断言的输出完整。
只有符号存在、只见顶层 PASS、未匹配测试、前置门 FAIL/SKIP、中断或部分 PASS 均不是完成。
所有文件的最终清单应在进程已自然结束、owner 无未决且现场无其他访问者时再只读采集。
若 owner/Close 不明，先保留日志和目录，不自行重新打开观察文件或尝试“修复”。

```sh
# 仅在自然结束且无 UNKNOWN / pending owner 时，由人工确认后采集。
python3 - "$FIXTURES" "$EVIDENCE/fixture-final-sha256.json" <<'PY'
import hashlib, json, pathlib, stat, sys
root, output = map(pathlib.Path, sys.argv[1:])
items = {}
for p in sorted(root.rglob('*')):
    st = p.lstat()
    if stat.S_ISLNK(st.st_mode):
        raise SystemExit(f'Unexpected symlink: {p}; STOP')
    entry = dict(mode=oct(st.st_mode), device=st.st_dev, inode=st.st_ino,
        nlink=st.st_nlink, size=st.st_size)
    if stat.S_ISREG(st.st_mode):
        entry['sha256'] = hashlib.sha256(p.read_bytes()).hexdigest()
    elif not stat.S_ISDIR(st.st_mode):
        raise SystemExit(f'Unexpected file type: {p}; STOP')
    items[str(p.relative_to(root))] = entry
with output.open('x') as f:
    json.dump(items, f, indent=2)
    f.write('\n')
PY
```

## 6. 保留、失败诊断与人工清理

harness 没有 t.TempDir、Remove/RemoveAll 或自动删除路径。每场景保存原 main、带 tail
原 WAL、预期 published WAL、固定 ledger、SHA256 清单及工作文件。通过后原输入再次
核对；失败现场同样保留。正常操作显式检查 Write/Sync/Close，不隐藏短写或 close 错误。

资源按 rows → DB/Conn → 观察文件 → retained directories 顺序，一次性尝试退休。
main 观察 fd 在 DB 持锁期间始终保留，防止辅助 read-and-close 意外释放 POSIX 进程锁。
未知 close 或 production pending owner 会阻断后续 case 和资源退休，不隐式重试 Close/
RetryPlaintextCleanup。Go 引用保留到进程结束；不能声称进程退出后仍保有文件锁。
旧 WAL inode 仅在保留的 descriptor 生命周期内可观察，成功关闭后它可能消失；
原 WAL 字节已另存为 wal.input.bin，持续保留，不声称长期保留已退役 inode。
若发生失败，检查日志最后的 ASSERT/RETIRE 阶段和精确错误，保留全部输入/候选/旁文件。

- statx/openat2/ext/profile 不满足：NOT_RUN/前置失败，保留参数；不得换平台或放宽检查。
- nil-default 接受、输入变化、错误分类变化：FAIL，记录公开桥和 ownership 的诊断。
- published bytes/inode/ledger 不符：FAIL；不要只查看返回 nil、修改期望或运行 checkpoint
  来覆盖现场。
- Sync、Close、pending-owner、unknown fd、挂起或非自然结束：UNKNOWN/FAIL，停止其它
  操作；保留引用与日志，确认进程/所有者状态后另行审阅，不删除或复用 fixture。

清理是你审阅后的独立手工动作：先完整归档该唯一 RUN，核对归档读回哈希；确认测试进程
已结束、没有未决 owner 或其它使用者、结果和失败诊断已保存，才由你人工删除打印出的
本次 RUN。不要删除源仓库、历史 fixture 或其他目录。本手册不包含自动清理命令。
