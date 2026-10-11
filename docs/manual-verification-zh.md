# gosqlite 中文手工验收操作手册

五个示例目录终结的后续限定普通复验见[补充规程](example-retention-verification-zh.md)；其真实失败现场仍未动态验证，旧受限门与缺失设施保持原状态。

2026-10-11 P0更新：有限public Open恢复源码已接入，独立source/model/compile审阅PASS；真实恢复仍BLOCKED / NOT_RUN，公开物理harness为IMPLEMENTATION_MISSING，详见第18节及[专题](rollback-recovery-open.md)。下面固定776b基线的缺口表和历史步骤保留当时身份，不能据其旧“public Open未接线”推断当前源码仍未实现。

本文供维护者规划后续独立验收，核对实际可执行入口、应保存的证据和不能据此作出的结论。**本手册编写过程只读核对并写入这个新文件，没有执行任何测试、构建、fuzz、数据库命令、故障操作或 namespace 操作，也没有修改项目源码或 CI。下面的执行步骤均未因编写本文而运行，状态为 NOT_RUN。** 文档持久保存及同期独立更改和测试如有结果，在 docs/progress.md 中另行记录，不属于本文编写证据。

## 1 先读结论与执行边界

当前不能宣布原始 `goal.md` 全部验收通过，也不能宣布稳定版、生产可用或加密安全已经认证。原因同时包括真实实现缺口、未合入的独立候选、已知失败、未运行的验收以及平台限制，不能统称为“只差人工测试”。

本手册的源码基线为本地只读确认的提交：

- 项目：`durable-restart/gosqlite`
- HEAD：`776b23eb59ce9613bf4498d352a716a193cdc9c3`
- tree：`8f6026330f968314456548a3b8684476fea73afb`，来自中断交接记录
- 编写开始时只读 `git status --short` 为空；随后验收相关文档进行了更新，不能再把当前工作树称为干净
- 原规范 `goal.md` SHA-256：`9ae7b1d776856669a10eb753408c7754d015d266f8783240165450e96e4bac81`
- 本文日期：2026-10-11 UTC

本文不是执行授权。2026-10-10 中断时新增执行均暂停；2026-10-11 首轮恢复先做只读核对与中文文档，清楚独立的后续工作须先说明源码范围、测试选择和与受阻链路无关的理由。被中断或既有受限链路继续暂停。即使某个普通测试以前被允许，也不能凭本文自动启动。未来维护者应先完成独立人工审查，确认合法执行范围与具体来源版本，随后仅在自己有权限、隔离且可丢弃的环境操作。

后续更新说明：下文固定776b基线中的 write/readonly/sparse-exact缺口是历史状态。四项独立修正的当前源码、限定结果和操作步骤见[CLI资源复验](cli-resource-verification-zh.md)与[组合证据](evidence/cli-resources-20261011.md)；不得把新测试套在旧基线上。其他受限门和未实现设施不因这次修正解除。

当前仓库说明入口：[验收矩阵](acceptance-status.md)、[中断记录](interruption-20261010.md)。本手册补充已有历史手工记录，不改变原始 goal.md。

特别规则：

1. 本文中的命令块只能逐项审阅，不能整篇提取为脚本执行。本文未创建或启动任何命令文件。
2. 物理故障、崩溃、进程强制退出、热恢复、原生 I/O 失败、错误密钥、磁盘篡改、namespace 干预及中断发布/rekey 在当前环境保持 BLOCKED / NOT_RUN。不得由代理换命令、换输入、换机器、换 CI 重试以绕过限制。
3. 22:39 中断的精确触发操作与对象未知。ANN V4 修正链路及约 22:30 被中断的 41 目标 fuzz 链路不得重发、改写或用另一项执行替代。没有证据证明其完整运行，也没有可记为通过的结果。
4. 当前 main 的 Windows/Darwin 两个 raw namespace 测试尚未合入新门控。**不要在该提交直接开启新的 Windows/macOS 全量测试或 OS CI runner。**
5. 新 OS runner、性能 CI runner、Actions 用量/配额/计费读取都不在本轮权限内。本文不给出启用、付款、配额检查或远端工作流触发操作。
6. 未来环境也必须是合法独立验收，不能被当作恢复被拒绝链路的替代路线。执行不确定时先暂停并查明原因，不推测“换写法就能解锁”。

## 2 状态如何记录

| 状态 | 本文含义 | 能否据此宣布功能通过 |
|---|---|---|
| PASS 历史证据 | 旧日志在明确源码、命令和范围下记录成功；本轮未重跑 | 只能证明该历史范围 |
| PASS 本次独立验收 | 未来操作者实际完成，原始日志、退出码、断言与身份完整 | 仅限声明范围，不能覆盖其他门禁 |
| FAIL | 实际观测失败，包括保留的历史失败 | 否；后续修复应新增证据而不覆盖原记录 |
| NOT_RUN | 未运行，或没有可确认的完成结果 | 否 |
| BLOCKED | 平台、权限、环境或前置条件阻止执行 | 否；不能当作跳过后通过 |
| HOLD | 候选或报告存在待解决审查问题 | 否 |
| PARTIAL / 实现缺口 | 部分实现、未实现或尚未集成到此提交 | 否；没有可运行入口时不编造命令 |
| IMPLEMENTATION_MISSING | 对该要求没有完整可用实现或执行 harness | 否；不能写成只待运行 |
| INCONCLUSIVE | 测量不足以支持结论，例如性能置信度不足 | 否，保留原始样本 |
| SKIP / skipped_unsupported | 测试/平台没有覆盖到目标 | 否；进程退出成功也不自动转为覆盖成功 |

通过负面对照有两种不同含义：测试程序正确检测到应拒绝的输入，可以记该负面对照 PASS；旧版本错误接受输入、导致回归测试退出 1，仍是旧版本 FAIL。外层记录脚本退出 0 不会把内层真实失败变成产品通过。

## 3 当前能力与未完成项

### 3.1 已有源码和历史普通证据

当前提交包含原生纯 Go 明文读写、受限 SQL 与 `database/sql`、事务/savepoint、普通 rollback/WAL/shared-WAL 生命周期、grams13、文本相似度、dense/half/bit/sparse 向量、实际 HNSW/IVFFlat、正确密钥加密生命周期、加密备份/rekey CLI、离线迁移与距离复核、Native Check V2、事务测量 V4 等实现。具体支持列表以 `docs/compatibility.md` 及专题文档为准。

中断记录给出的历史证据：

- 固定组合源码 `8b604e3c667152284a29d7ad7dfba5a10f63b053` 的完整普通 unit/vet/build、固定 SQLite oracle、全包 race、五目标构建、模块与源码哈希，在 2026-10-10 22:10:33Z 记录 PASS。
- 776b23e 的恢复副本检查于 22:37:31Z 记录 PASS；它是 focused normal/race、vet/build、五目标、模块和干净工作树证据，不要写成另一次完整全量 oracle/race。
- 该 checkpoint 已有 CI 运行 `38091940645` 于 22:52:30Z 自然完成 SUCCESS；交接记录分别核实 ordinary 与 oracle 作业。历史链接：<https://github.com/ms-xiaosnow/gosqlite/actions/runs/38091940645>。
- 本轮没有连接远端重验这些结果，也没有触发或重跑 CI。

### 3.2 必须保留的真实缺口

| 项目 | 当前 776b23e 的准确状态 | 后续前置条件 |
|---|---|---|
| 自动 public Open rollback 恢复 | 内部有限多段解析和 `RecoverRollback` 有源码；public Open 集成仍缺 | 独立候选集成与精确源码审查，再另行处理受限物理门禁 |
| WAL 尾部恢复/发布 | 支持普通成功路径；原生 exclusive-tail publisher 仍缺 | 不能把正常 close/reopen 当作实现；wal-tail V2 仍待独立复审 |
| 通用明文备份 | 当前 CLI 和公共 copy API 未实现 | Linux rollback V2 只是未发布候选；WAL 和非 Linux 明文备份仍未实现 |
| 真正只读的明文 WAL 全库 CLI Check | 未实现 | 现有已配置 WAL 会话可 `DB.Check`；命名扩展 WAL 检查使用可写协调，不能改称只读 |
| SQL/planner | 有限支持；更广的错误阶段、排序和多键差异未完全关闭 | V3 有 34 个保留失败，V4 有 6 个 bound-LIMIT 回归；V4c 是未合入候选，不能借其日志给 main 判 PASS |
| Windows shared encrypted rekey | main 中不支持 | 已审查的候选未并入 main；原生 Windows 仍 NOT_RUN |
| Darwin Unicode 与目录诊断 | main 保留受限文件名/目录清单范围 | V3 修正在未发布组合；更广 Unicode 根名能力仍受限 |
| raw namespace 门控 | main 的两个 OS 原生测试未迁入 manual gate | 先合入已审查门控并重新核对精确版本；不能直接新增 OS runner |
| ANN 报告验收工具 | main 是既有简单 harness；V3 候选 HOLD，V4 未冻结 | 不得以历史算法测试替代报告审查；被中断链路保持暂停 |
| 性能回归 CI | 后续 V3 候选有有界审查，未部署 | 受控机器、配额读取授权、资源决策和单独部署授权均未完成 |
| 长时间 soak、大规模测量 | 有 harness 与提案，无完整长时间结果 | 预声明负载、资源、目录、停止条件和审查；本文不启动 |
| 外部 PG/Neon 验证 | 现有路径主要是离线导入、转换和带注释模板 | 需要用户授权的可丢弃真实服务及版本/权限/扩展核验 |
| 独立数据安全、密码学、许可、API 审查 | 未完成 | 具备资格的独立评审；普通源码审查不能代替 |

未支持的 SQLite SQL/文件形式要明确拒绝，而不是忽略：例如未声明的 schema、UTF-16、WITHOUT ROWID、autovacuum、广泛 UNIQUE/CHECK/FK、CTE/subquery/window 等。不要把通过有限子集写成与 SQLite 3.54.0 全功能等价。Native Check 的受支持范围也比“能够 SELECT 某张表”更窄。

### 3.3 已知失败和未完成记录

- 早期完整 oracle 使用错误的默认可写 fixture 路径、后来 JOIN 断言过时等真实失败已有保留记录；修正后的结果必须另列。
- 原始 20 秒 rollback soak 在约 14.0159 秒报 `ErrChanged`；Reader 修正后的四个 20 秒样本通过，仍不是长期 soak。
- 事务 harness V2/V3 的错误图、retirement 与存储 quota 失败保留；V4 的三个 64-attempt 样本各 48 commits，只是有界 smoke。
- Native Check V1 的 detached-memory 假成功问题保留；V2 修复已集成。不能将源码/内存否定模型说成对真实磁盘进行了故障注入。
- 导入距离 V1 的 16000 维误差界失败约 14.0724 倍，V2 改为有推导的向外舍入误差界并有独立有理数检查；不能删除旧失败或随意加宽容差。
- grams medium 512 行出现约 147 倍数据库空间，18 个选择性中位数里有 4 个慢于扫描；smoke 12 个均慢。必须保留，不能给出普遍加速结论。
- ANN V3 的三个 HOLD：HNSW 节点读数与遍历/重评分不守恒；IVF 探完全部 lists 却遗漏源成员；同一 query/row 在 exact 与 ANN 精确重评分间出现矛盾距离。这是报告准入问题，不能靠重新跑 benchmark 掩盖。
- 上述 ANN V3 失败属于未发布报告工具接受了矛盾报告的缺陷，不自动推断生产 ANN 算法已经错误。
- 原生 Windows 默认 cross-vet 的 mmap `uintptr`/`unsafe.Pointer` 告警有历史记录。不要关闭检查器后把结果记为 clean vet；另行记录精确包、命令、退出码和诊断。
- 两小时每 profile 的 soak 从未开始；41-target fuzz 新活动范围不明且无完成证据；ANN V4 修正未完成。均不得记为通过。

## 4 环境与依赖准备

以下命令面向未来已获许可的 Linux/amd64 独立本地副本，采用 Bash。不是本轮运行指令，也不能原样用于 macOS/Windows。Go 目标平台运行与跨平台编译是不同验收项。

### 4.1 必需条件

1. 可丢弃、非生产且独立的本地工作目录；禁止在用户原数据库、唯一备份或运行中的数据库目录操作。
2. 固定 Go **1.27.2**。`GOTOOLCHAIN=local`，不自动下载或切换编译器。
3. 官方 SQLite **3.54.0** CLI 只作测试 oracle，不允许生产程序链接或启动它。
4. 模块按 `go.mod`/`go.sum` 固定：`golang.org/x/crypto v0.58.0`、`golang.org/x/sys v0.49.0`、`golang.org/x/text v0.43.0`。
5. Git、Bash、tar、SHA-256 工具；核对元数据可用 `stat`、`df`、Linux `findmnt`。将来需要构建 oracle 时还要 Python 3、unzip、make、C 编译器及官方源码构建依赖；使用经过核验的现成 oracle 可省去这一步。
6. `CGO_ENABLED=0` 用于生产构建和一般测试；单独 race 门禁使用 `CGO_ENABLED=1`，因此 race 的主机需有相应 C 工具链。这不意味着生产库依赖 CGO。
7. 记录实际 CPU、OS/内核、架构、内存、文件系统和挂载选项、缓存条件、其他负载、容器配额可见性。不能凭名称“独立主机”声称已经隔离。

仓库 `scripts/install-baselines.sh` 是 Linux/amd64 固定版本安装器；`scripts/build-oracle.sh` 构建并验证固定 oracle。**本轮不执行这两个脚本，也不自动安装软件。** 未来操作者须先审查脚本、官方来源和新目的目录，再决定是否运行。

固定来源材料见 `docs/upstream.md`，本轮仅核对仓库记录，没有重新联网认证上游：

| 对象 | 固定值 |
|---|---|
| Go Linux amd64 tar SHA-256 | `ecbadb99091a3f46e31f5f934b068b1864eafa7995211b39eaddf76996045fe5` |
| SQLite 源码 ZIP 文件名 | `sqlite-src-3540000.zip` |
| SQLite ZIP 字节数 | `14881448` |
| SQLite ZIP SHA3-256 | `a5c29342ca185abcda332448fbb27c536661245be27e20199b46c49099b0e15e` |
| 生成的 sqlite3.c SHA3-256 | `8efd453d08a7cfc4c79de5923153036cdc4dcaa254d4f1e617be6a2a98828f9e` |
| SQLite source ID | `2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3` |

没有本地已验证模块缓存时，应记录依赖缺失并停止该步骤。本文的离线设置不建立新的网络/下载授权；不能在校验失败后直接关掉验证下载替代物。

### 4.2 建立独立源码与数据目录

下面是未来普通验收的准备模板，先替换三个绝对路径；`BASE` 必须是用户新选的、空间足够的本地验收父目录。不要填生产目录、原仓库或中断工作目录。每次新建一个 `RUNROOT`，不能复用旧失败输出。

```bash
# 文档模板。本轮不得执行。
SOURCE_REPO=/absolute/path/to/reviewed/gosqlite
BASE=/absolute/path/to/disposable-acceptance
TOOLCHAINS=/absolute/path/to/verified/toolchains
SOURCE_COMMIT=776b23eb59ce9613bf4498d352a716a193cdc9c3

test -d "$SOURCE_REPO/.git" || exit 1
test -d "$BASE" || exit 1
test -x "$TOOLCHAINS/go1.27.2/bin/go" || exit 1
test -x "$TOOLCHAINS/sqlite-build-3540000/sqlite3" || exit 1
umask 077
set -o pipefail
RUNROOT=$(mktemp -d "$BASE/gosqlite-acceptance-XXXXXXXX") || exit 1
WORK="$RUNROOT/source"
EVIDENCE="$RUNROOT/evidence"
DATA="$RUNROOT/data"
mkdir "$WORK" "$EVIDENCE" "$DATA" "$RUNROOT/bin" "$RUNROOT/tmp" "$RUNROOT/go-cache" || exit 1
git -C "$SOURCE_REPO" archive --format=tar "$SOURCE_COMMIT" > "$RUNROOT/source.tar" || exit 1
tar -xf "$RUNROOT/source.tar" -C "$WORK" || exit 1
git -C "$SOURCE_REPO" rev-parse "$SOURCE_COMMIT^{tree}" > "$EVIDENCE/source-tree.txt" || exit 1
printf '%s\n' "$SOURCE_COMMIT" > "$EVIDENCE/source-commit.txt" || exit 1
sha256sum "$RUNROOT/source.tar" > "$EVIDENCE/source-archive.sha256" || exit 1
cd "$WORK" || exit 1
sha256sum goal.md > "$EVIDENCE/goal.sha256" || exit 1
find . -type f -print0 | LC_ALL=C sort -z | xargs -0 sha256sum > "$EVIDENCE/source-before.sha256" || exit 1
```

`git archive` 只导出已明确选择的提交，不带未提交更改、其他候选或 `.git`。选择更新版本时，要重新编写该版本的范围、已知问题与证据；不能只修改 `SOURCE_COMMIT` 后沿用本文所有结论。源码快照复制不等于备份活数据库：禁止 `cp` 一个正在使用的 main/WAL/SHM 家族来冒充一致性备份。

归档源码没有 Git 元数据，下面 `GOFLAGS=-buildvcs=false` 仅用于正确标识这一情况，不是关闭安全检查或绕过 CI。

```bash
# 文档模板。本轮不得执行。
export PATH="$TOOLCHAINS/go1.27.2/bin:$PATH"
export GOTOOLCHAIN=local CGO_ENABLED=0
export GOPATH=/absolute/path/to/verified/offline-go-path
export GOPROXY=off GOSUMDB=off
export GOCACHE="$RUNROOT/go-cache"
export GOFLAGS=-buildvcs=false
export TMPDIR="$RUNROOT/tmp"
export SQLITE_ORACLE="$TOOLCHAINS/sqlite-build-3540000/sqlite3"
export GOMAXPROCS=2
```

`GOPROXY=off GOSUMDB=off` 只用于已验证缓存的离线复验，不能说明依赖从未验证或新下载无需校验。`GOMAXPROCS=2` 是复验容量条件，不是 CPU 隔离证明。

### 4.3 日志记录函数

未来用户可在这次专用终端定义下面的函数。它不是仓库脚本，不会自动运行后续步骤；每一步名称必须唯一。它分别保存命令、UTC 时间、stdout、stderr、退出码，不依赖 `tee` 的退出状态。

```bash
# 文档模板。本轮不得执行。
capture() {
  local name=$1 rc
  shift
  test ! -e "$EVIDENCE/$name.command" || {
    printf '拒绝覆盖已有记录: %s\n' "$name" >&2
    return 90
  }
  printf '%q ' "$@" > "$EVIDENCE/$name.command" || return 91
  printf '\n' >> "$EVIDENCE/$name.command" || return 91
  date -u +%FT%TZ > "$EVIDENCE/$name.started" || return 91
  if "$@" > "$EVIDENCE/$name.stdout" 2> "$EVIDENCE/$name.stderr"; then
    rc=0
  else
    rc=$?
  fi
  printf '%s\n' "$rc" > "$EVIDENCE/$name.exit" || return 91
  date -u +%FT%TZ > "$EVIDENCE/$name.finished" || return 91
  printf '%s: exit %s\n' "$name" "$rc"
  return "$rc"
}
```

调用中必须用 `|| exit 1` 或人工停止，不得忽略失败后把整组标绿。失败重试应新建一组证据目录，保留旧失败。本文没有 `kill`、超时杀进程、自动修库、自动重试或自动删除失败目录逻辑。

### 4.4 版本、磁盘和源身份前置检查

```bash
# 文档模板。本轮不得执行。
capture env-go go version || exit 1
capture env-go-baseline go env GOVERSION GOOS GOARCH CGO_ENABLED GOTOOLCHAIN || exit 1
capture env-uname uname -a || exit 1
capture env-filesystem findmnt -T "$TMPDIR" || exit 1
capture env-free-bytes df -B1 "$TMPDIR" "$EVIDENCE" || exit 1
capture env-free-inodes df -i "$TMPDIR" "$EVIDENCE" || exit 1
capture env-oracle "$SQLITE_ORACLE" -batch -bail -noheader -list :memory: \
  'SELECT sqlite_version(),sqlite_source_id();' || exit 1
capture modules go mod verify || exit 1
```

应见 `go version go1.27.2 linux/amd64`，oracle 身份必须逐字等于：

```text
3.54.0|2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3
```

不要整份打印 `env`，其中可能有密钥。只记录这里列出的非秘密字段。若 `findmnt` 不存在，先手工取得等价挂载证据，不静默跳过。

已有环境曾出现 `/tmp` 为约 5.24 GB tmpfs，仅约 1.33–1.42 GiB 可用；工作区挂载带 `fsync=volatile`。这是历史观测，不是新环境的容量。必须检查实际 `TMPDIR`，不能拿当前目录的 `df` 代替。`fsync=volatile` 主机上的成功 Sync/正常重开不能证明掉电持久性。

## 5 普通自动门禁的手工操作

仅在第 1 节暂停原因已得到人工处理、且未来明确允许普通 Linux 复验后使用。不可在当前 main 的 Darwin/Windows 上照抄整组运行。

### 5.1 普通源码范围检查

```bash
# 文档模板。本轮不得执行。
capture ordinary-scope go run ./internal/testscope || exit 1
```

预期 stdout：

```text
Ordinary test source-reference guard PASS; behavioral review is still required.
```

人工还要阅读所选测试及 helper。此 guard 是防错辅助，不能证明行为安全；main 对 OS raw namespace primitive 的缺口就是已知反例。普通 `GOFLAGS` 不得出现 `manual_`；`manual_namespace` 与 `manual_recovery` 不得混入 unit、race、oracle 或 CI。

### 5.2 无缓存普通全量测试与产物

现有入口 `scripts/check.sh` 依次做固定版本检查、testscope、普通 `go test ./...`、vet、CGO0 build、单独 CGO1 race；脚本本身没加 `-count=1`。若要主张新的运行结果，可逐条执行以下等价、明确禁用测试缓存的普通门禁，而不是把 `(cached)` 算作本次执行：

```bash
# 文档模板。本轮不得执行。
capture ordinary-unit env CGO_ENABLED=0 go test -count=1 ./... || exit 1
capture ordinary-vet env CGO_ENABLED=0 go vet ./... || exit 1
capture production-build env CGO_ENABLED=0 go build -trimpath ./... || exit 1
capture ordinary-race env CGO_ENABLED=1 go test -race -count=1 ./... || exit 1
capture cli-build env CGO_ENABLED=0 go build -trimpath -o "$RUNROOT/bin/gosqlite" ./cmd/gosqlite || exit 1
capture cli-version "$RUNROOT/bin/gosqlite" version || exit 1
capture cli-help "$RUNROOT/bin/gosqlite" help || exit 1
sha256sum "$RUNROOT/bin/gosqlite" > "$EVIDENCE/cli-binary.sha256" || exit 1
```

通过判定：每个命令退出 0、测试真实执行、无 `FAIL`、race 无数据竞争；各个 skip 的目标/原因另记。CLI version 应说明 Go 1.27.2 / SQLite baseline 3.54.0；开发版本字符串不冒充发布号。生产 build 为 CGO0；race 是另一份检测构建，不能用于性能比较。

不要为解决报错加大 quota、改断言、把缺失测试改成空正则或禁用 vet。全套普通测试通过也没有执行 manual 恢复或故障门禁。

### 5.3 固定完整 oracle

```bash
# 文档模板。本轮不得执行。
capture full-oracle env SQLITE_ORACLE="$SQLITE_ORACLE" sh ./scripts/test-oracle.sh || exit 1
```

该脚本实际先跑 testscope，再比较 SQLite source ID，然后执行 `go test -count=1 -timeout=30m -tags oracle ./...`。不能把它缩成 `./internal/testkit` 或某一个 `-run` 后仍称“完整 oracle”。脚本中的 30 分钟是已存在的 Go 测试上限，不是 crash 验收；普通协作 CLI 子进程正常 `.quit`/EOF/Wait 收尾，不能擅自加进程终止机制。

应检查每个已选择 oracle 测试是否真的运行，不能用无匹配的退出 0 或 `SKIP SQLITE_ORACLE` 填通过。官方 fixture 只在隔离副本内使用；不改原仓库 fixture 或降低对照断言。

### 5.4 仅编译的多平台产物

原目标要求 linux/amd64、linux/arm64、darwin/arm64、windows/amd64 的 CGO0 产物。未来可分别做编译证据，**不运行外平台二进制，不开启新 runner**：

```bash
# 文档模板。本轮不得执行。
capture build-linux-amd64 env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath ./... || exit 1
capture build-linux-arm64 env CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath ./... || exit 1
capture build-darwin-arm64 env CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -trimpath ./... || exit 1
capture build-windows-amd64 env CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath ./... || exit 1
```

记录名用 `COMPILE_PASS`，原生 runtime 栏仍 `NOT_RUN`。不要把“历史五目标构建”与本模板的四个原目标混为一项；第五目标若要复验，应先从原日志确认其精确配置。

## 6 手工 SQL 与双向文件互操作

这组只用完整的新建合成 rollback 数据库。所有 CLI 每次只接受一条 SQL；跨调用 `BEGIN` 不保持事务，事务验收用 API/driver 测试。

### 6.1 gosqlite 创建并交给官方 SQLite 检查

```bash
# 文档模板。本轮不得执行。
CLI="$RUNROOT/bin/gosqlite"
PLAIN="$DATA/native-created.db"
test ! -e "$PLAIN" || exit 1
capture sql-create "$CLI" query "$PLAIN" \
  'CREATE TABLE items(id INTEGER PRIMARY KEY,name TEXT NOT NULL,score REAL)' || exit 1
capture sql-insert "$CLI" query "$PLAIN" \
  "INSERT INTO items VALUES(1,'中文',1.25),(2,'emoji🙂',2.5)" || exit 1
capture sql-index "$CLI" query "$PLAIN" \
  'CREATE INDEX items_name ON items(name)' || exit 1
capture sql-select "$CLI" query "$PLAIN" \
  'SELECT id,name,score FROM items ORDER BY id' || exit 1
capture sql-native-check "$CLI" check "$PLAIN" || exit 1
capture sql-inspect "$CLI" inspect "$PLAIN" || exit 1
capture sql-official-integrity "$SQLITE_ORACLE" -batch -bail "$PLAIN" \
  'PRAGMA integrity_check; SELECT id,name,score FROM items ORDER BY id;' || exit 1
```

关键断言：

- SELECT JSON Lines 含列名 `id,name,score` 与两行 `[1,"中文",1.25]`、`[2,"emoji🙂",2.5]`，排序一致。
- `check` 返回 `status:"ok"` 和 `scope:"gosqlite-supported-profile-v1"`，不能只看 inspect。
- 官方输出 `ok`，并返回相同语义值；不要将官方输出格式与 JSON 字节直接比较。
- 主文件是合法 SQLite 格式；`inspect` 只说明 metadata，单独成功不等于完整 integrity。

### 6.2 官方 SQLite 创建并由 gosqlite 更新

```bash
# 文档模板。本轮不得执行。
OFFICIAL="$DATA/official-created.db"
test ! -e "$OFFICIAL" || exit 1
capture sql-official-create "$SQLITE_ORACLE" -batch -bail "$OFFICIAL" \
  "PRAGMA page_size=4096; PRAGMA journal_mode=DELETE; CREATE TABLE items(id INTEGER PRIMARY KEY,name TEXT NOT NULL,score REAL); INSERT INTO items VALUES(7,'原始',3.5);" || exit 1
capture sql-read-official "$CLI" query "$OFFICIAL" \
  'SELECT id,name,score FROM items ORDER BY id' || exit 1
capture sql-update-official "$CLI" query "$OFFICIAL" \
  "UPDATE items SET name='已更新',score=4.5 WHERE id=7" || exit 1
capture sql-recheck-official "$SQLITE_ORACLE" -batch -bail "$OFFICIAL" \
  'PRAGMA integrity_check; SELECT id,name,score FROM items ORDER BY id;' || exit 1
```

应保持 id=7、name=已更新、score=4.5，official integrity 为 `ok`。512/4096/65536 页和 BLOB/NULL/overflow 等完整矩阵由现有 oracle 与 native-check 测试核验；这两个手工例子没有穷尽它们。

### 6.3 database/sql、事务与普通 WAL

以下是真实存在的测试名，精确锚定能减少误选；不要把 focused PASS 代替全量 gate：

```bash
# 文档模板。本轮不得执行。
capture driver-transaction env CGO_ENABLED=0 go test -count=1 -v ./driver \
  -run '^(TestWritableDatabaseSQLTransactions|TestWritableReadOnlyTransactionsAndIsolation|TestWritableExplicitSQLTransactionsAndPoolReset|TestDriverSavepointsPoolResetAndOuterRelease)$' || exit 1
capture native-savepoint env CGO_ENABLED=0 go test -count=1 -v ./sqlite \
  -run '^(TestTransactionDeferredSnapshotTiming|TestTransactionSavepointNamesDDLAndStatementFailure|TestTransactionImmediateExclusiveAndFailedOuterRelease|TestTransactionSavepointsPreserveGramsHNSWAndIVF)$' || exit 1
capture shared-wal env CGO_ENABLED=0 go test -count=1 -v ./sqlite \
  -run '^(TestSharedWALSnapshotsAndWriterContention|TestSharedWALOptionBoundaries)$' || exit 1
capture write-example env CGO_ENABLED=0 go run ./examples/write "$DATA/driver-example.db" || exit 1
```

最后一个在新库应打印 `inserted 1: 数据库检索`，它只能作为写入 smoke：当前 examples/write 忽略 defer Close 的返回错误，且 log.Fatal 会绕过 defer，不能用该示例证明任何失败或正常资源退休路径。事务/关闭结论必须来自具备相应断言的测试与 API 结果。人工检查 rollback/savepoint 的逐行结果、BUSY 分类、上下文取消和正常 Close。普通 WAL retained committed head、checkpoint、关闭再打开、协作进程争用不是进程崩溃或热恢复证据。

## 7 grams13 文本索引与相似度

### 7.1 规范化随机组合和真实 1 2 3 字搜索

```bash
# 文档模板。本轮不得执行。
capture grams-correctness env CGO_ENABLED=0 go test -count=1 -v ./sqlite \
  -run '^(TestGramsIndexedLike12800Combinations|TestGramsNFCInputs12800RandomizedMemberships|TestGramsNFCLifecyclePostingConsistency|TestGramsOneTwoThreeRuneSearchAndExplain|TestGramsUnicodeEscapeNULLNULAndBooleanSafety|TestGramsHighFrequencyFallbackAndLimits)$' || exit 1
capture grams-example env CGO_ENABLED=0 go run ./examples/grams "$DATA/grams.db" || exit 1
capture grams-full-check "$CLI" check "$DATA/grams.db" || exit 1
capture grams-named-check "$CLI" check "$DATA/grams.db" docs_grams || exit 1
capture grams-official-integrity "$SQLITE_ORACLE" -batch -bail "$DATA/grams.db" \
  'PRAGMA integrity_check;' || exit 1
```

应看到“库”“数据”“数据库”分别有相应 gram 长度的实际 `SEARCH` 路径、结果为包含“数据库查询”的行、真实 work counters，以及索引一致性通过。仅 EXPLAIN 文本包含 INDEX 还不够，应核对 posting 工作和源行读取。

NFC 测试是 **100 patterns × 128 texts = 12800 memberships**，不是 12800 次 SQL 查询；已有记录为 12416 distinct pairs，100 个输入经过 NFC 发生改变。原规范要求 ≥10000 经规范化组合，不要靠重复执行次数凑数。生产 LIKE 仍是 `LIKE_RAW_V1`，不能偷改成自动归一化语义。全量结果必须与 `NOT INDEXED` 扫描完全一致，合法高频/无字面量 fallback 要如实标明。

`check db docs_grams` 的 scope 仅是 `extension source/posting/frequency consistency only`；与不带 index 的全库 Check 是两个不同范围。

### 7.2 文本相似度

```bash
# 文档模板。本轮不得执行。
capture similarity-unit env CGO_ENABLED=0 go test -count=1 ./extension/similarity || exit 1
capture similarity-native env CGO_ENABLED=0 go test -count=1 -v ./sqlite \
  -run '^(TestSimilarityNativeTransactionsRebuildReopenCheckpoint|TestSimilarityNativeConfiguredNormalizationAndMatchNamespace)$' || exit 1
capture similarity-example env CGO_ENABLED=0 go run ./examples/similarity "$DATA/similarity.db" || exit 1
```

通过要同时覆盖独立数学/golden、阈值结果与强制扫描一致、事务后的 posting 一致性、中文/日文/emoji 语义。精确阈值必须零漏召回；无法证明召回界的范围应正常完整扫描，不能悄悄剪枝。加权排序融合输出不能被说成某个数据库原生 BM25 或 pg_bigm 实测。

## 8 向量与 ANN

### 8.1 精确距离与类型

```bash
# 文档模板。本轮不得执行。
capture vector-driver env CGO_ENABLED=0 go test -count=1 -v ./driver \
  -run '^(TestDriverDenseVectorBindingsAndTransactions|TestDriverVectorQuotaErrorClassification|TestDriverBitBindingsTransactionsAndReopen|TestDriverHalfBindingOwnershipTransactionsAndPool|TestDriverSparseBindingOwnershipTransactionsAndPool)$' || exit 1
capture vector-example env CGO_ENABLED=0 go run ./examples/vector "$DATA/vector.db" || exit 1
capture sparse-example env CGO_ENABLED=0 go run ./examples/sparse-exact "$DATA/sparse.db" || exit 1
```

**当前示例退出码缺陷**：基线 `776b23e…` 的 `examples/sparse-exact/main.go`
在参数或 example 返回错误时只打印 stderr，main 仍可能自然返回 0。
因此 `capture ... || exit 1` 不能独自判此例成功。必须人工核对
`sparse-example.stderr` 为空且 stdout 是完整、唯一 JSON：dimensions 为
1000000000，query_nonzeros 为 2，query_gss_bytes 为 32，native_source_rows
为 5，native_support_work/sql_support_work 均为 13，sql_representation 为
sparsevec、sql_ranking 为 bounded_heap；两组 exact L2 hits 必须一致，
三行顺序为 10、20、30，前两行距离为 0、1。任一字段缺失或错误均为 FAIL。
这是示例错误报告缺口，不自动说明向量内核错误；独立源码修正另行记录。

核对实际维数、有限数值、NULL/零范数策略、对应支持的 dense/half/bit/sparse 距离语义、持久化/reopen 和 driver 参数所有权。dense 的 L2/L1/负内积/cosine 应按文档与独立数学对照；bit 距离不要套 dense 算子含义，旧 pg_embedding Manhattan `<~>` 必须按迁移规范转成 `<+>`。

SQLite 官方会将 VECTOR 声明按 affinity 对待，合法 SQL 插入不符合本引擎向量类型的 TEXT 后被本引擎拒绝，是类型兼容负面对照，不是物理页篡改。

### 8.2 现有 ANN 普通功能与冻结的修正链路

已核对的既有测试/入口包括：

- `internal/testkit/ann_conformance_test.go`：`TestOracleHNSWStorageInterop`、`TestOracleHNSWLegacyGramsUpgrade`
- `internal/testkit/ivf_conformance_test.go`：`TestOracleIVFFlatStorageInterop`
- `sqlite/ann_typed_test.go`：`TestTypedANNOrdinaryLifecycleSavepointAndReindex`
- `examples/ann`、`examples/ivfflat`、`examples/typed-ann-rerank`
- main 的 `benchmarks/ann/main.go` 支持 `-rows/-dims/-queries/-seed/-method/-lists/-probes/-max-probes`

这些入口存在，不代表 ANN V4 修复完成。**为避免把被中断修正链路换写成新命令，本文不提供新的 ANN V4 启动、修补、冻结、native smoke 或替代 benchmark 命令。** 未来先人工区分既有普通算法复验与被阻塞的修正，复核精确源码、数据、报告版本和权限，再决定独立验收步骤。

后续有真实测量时必须保存同一 snapshot 下 exact 与 ANN 命中、距离、plan、fallback、图/list 读取和候选重评分计数，给出固定数据集的 recall@1/10/100、build time、空间和延迟。HNSW 默认参数 recall@10 ≥0.95 是原目标的评估目标，不是对所有数据保证；不得通过降低断言或隐藏 fallback 宣称达标。ANN consistency Check 也不证明 recall、拓扑最优或性能优势。

## 9 Native Check 与加密正确密钥路径

### 9.1 全库 Check V2

```bash
# 文档模板。本轮不得执行。
capture check-native env CGO_ENABLED=0 go test -count=1 -v ./sqlite \
  -run '^(TestNativeCheckOrdinaryPageSizesAndReadOnly|TestNativeCheckFreelistRollbackAndReopen|TestNativeCheckWALPinnedSnapshots|TestNativeCheckExtensionsAndTypedValues|TestNativeCheckCorrectKeyReadOnly|TestNativeCheckTransactionOwnershipAndCounters)$' || exit 1
capture check-cli env CGO_ENABLED=0 go test -count=1 -v ./cmd/gosqlite \
  -run '^(TestCLINativeCheckPlainAndNamed|TestCLINativeCheckCorrectKeyProviders|TestCLINativeCheckMemoryArguments)$' || exit 1
capture check-example env CGO_ENABLED=0 go run ./examples/native-check "$CLI" || exit 1
```

检查 catalog、freelist、所有逻辑页所有权、B-tree 空间、表/普通索引双向对应、grams 与支持的 dense/typed HNSW/IVF 语义。Quota/unsupported/取消/close 错误不得输出成功 JSON；dirty 扩展返回 reindex required，检查本身不能修复或悄悄重建。

示例会比较 API 与真实 CLI 的全部计数，核对完整的普通/正确密钥来源家族字节和名称不变。它创建临时合成文件，非持久性故障测试。注意该版本无条件 defer RemoveAll 私有临时目录，失败时也可能删除 fixture，且清理错误未被可靠报告；日志仍由 capture 保留，但不能据此声称保留了完整失败现场或证明所有资源退休。需要失败现场保全的验收必须先完成相应独立 harness 改进。

限制：`inspect` 只是元数据；`check --wal-shared plain.db` 不带命名扩展的 full CLI 路径应在打开前拒绝，不能试图用 named check 替代 full check。Check 专项 page-read quotas 从 reader acquisition 之后开始，不是整个 Open 的总 I/O 或内存保证。逻辑 NULL 验证也不是物理 serial-type-0 独占性审计。

### 9.2 加密正确密钥与 CLI 备份 rekey

优先使用现成合成示例，不为验收使用真实秘密或用户唯一数据：

```bash
# 文档模板。本轮不得执行。
capture encryption-unit env CGO_ENABLED=0 go test -count=1 ./encryption || exit 1
capture encrypted-snapshot-example env CGO_ENABLED=0 go run ./examples/encrypted || exit 1
capture encrypted-wal-example env CGO_ENABLED=0 go run ./examples/encrypted-wal || exit 1
capture encrypted-shared-example env CGO_ENABLED=0 go run ./examples/encrypted-shared || exit 1
capture encrypted-cli-example env CGO_ENABLED=0 go run ./examples/encrypted-cli "$CLI" || exit 1
```

断言是使用正确密钥创建/写入/认证、正常重开、新密钥正常 rekey、实际 archive restore 后行值一致、普通共享快照可见性和 checkpoint。示例生成临时密钥，参数/日志不应出现秘密，不提供之后可恢复这些临时密钥的保证。不要向测试添加错误 key、磁盘损坏或中断步骤。

这些 encrypted/encrypted-wal/encrypted-shared/encrypted-cli 示例在当前基线
都无条件 defer 删除私有临时目录，即使执行失败也可能丢失该 fixture。
因此它们只提供限定的普通演示/成功路径证据；capture 保存日志不代表
保存了失败数据库。要求现场保留或失败退休证明的正式规程尚需独立
实现，标为 IMPLEMENTATION_MISSING，不能在本文中编造替代入口。

未来需要检查已专门准备的合成加密 fixture 时，实际 CLI 形状如下。这里的变量名是 provider 描述，不是密钥；只允许可信启动器/非交互管道提供内容，不在命令行填秘密：

```bash
# 文档模板。本轮不得执行。仅用于已经审查的合成 fixture。
capture encrypted-fixture-backup "$CLI" backup /private/disposable/source.gse --out /private/disposable/new.backup \
  --key-provider env-hex:GOSQLITE_CURRENT --out-key-provider env-hex:GOSQLITE_ARCHIVE --mode snapshot || exit 1
capture encrypted-fixture-rekey "$CLI" rekey /private/disposable/source.gse \
  --key-provider env-hex:GOSQLITE_NEW --current-key-provider env-hex:GOSQLITE_CURRENT --mode snapshot || exit 1
capture encrypted-fixture-check "$CLI" check /private/disposable/source.gse --key-provider env-hex:GOSQLITE_NEW --mode snapshot || exit 1
```

关键操作条件：

- 数据库路径在前，flag/value 分开；当前不支持 `--flag=value`、重复 flag 或自动探测 mode。
- mode snapshot/wal/shared 必须匹配已创建的 encrypted layout；不是原生明文 WAL flag。
- backup 目标必须不存在、父目录已存在。输出是 portable encrypted archive，需用 `sqlite.RestoreEncrypted` 恢复；没有 CLI restore 子命令，不能拿 archive 直接 `query`。
- rekey 前所有其他连接必须正常关闭，包括 idle/read-only shared 连接。当前 main Windows shared rekey 不支持。
- raw key provider 要 64 个 ASCII hex 字符；password provider 为 1–4096 原始字节。password 的换行也是密码一部分。禁止打印环境或启用 shell `set -x`。
- `stdin-password`/`stdin-hex` 只接受管道或普通文件，必须 EOF；两个不同凭据不能同时各自占用同一 stdin。本文不创建凭据文件。
- 成功 JSON 只在操作和正常 close 成功后发出。错误退出仍可能发生在发布之后，不能据非零退出自动判“完全未改动”，也不能自动重试/删除输出。
- rekey 保留旧 roots/bases/segments，旧凭据仍可能解密旧历史；不是撤销旧密钥历史访问，不是安全擦除。
- 本节没有验证 on-disk 错误密钥、篡改、热恢复或 interrupted rekey。密文中找不到一个 marker 也不能证明从未产生临时明文。

## 10 离线 pg_embedding 导入和距离复核

### 10.1 合成导入和正常提交后继续

现成例子创建 CSV/NDJSON、manifest、新库，在一批已提交后正常返回再 resume，不是强制退出：

```bash
# 文档模板。本轮不得执行。
capture import-example env CGO_ENABLED=0 go run ./examples/pgembedding-import \
  -dir "$DATA/import-example" || exit 1
capture import-resume env CGO_ENABLED=0 go test -count=1 -v ./migration/postgres \
  -run '^(TestImportNativeCommittedResumeAndVerify|TestImportDatasetDistancesMappedFormatsAndResume|TestImportDatasetDistancesWideFiniteVectors|TestIndependentImportDistanceV2GammaProofChallenges)$' || exit 1
capture import-cli env CGO_ENABLED=0 go test -count=1 -v ./cmd/gosqlite \
  -run '^(TestMigrateImportDryRunApplyResumeVerifyCLI|TestMigrateImportDatasetDistancesCLI|TestIndependentMigrateImportDistanceCLIReportsFailure)$' || exit 1
```

`-dir` 必须是新路径，父目录存在。输出 JSON 为两个格式分别给出 Input/Manifest/Database/DryRun/Paused/Complete/Verified 等对象；保留 source hash、typed row hash、每行/每组件和主键/游标证据。示例的 ANN recall@4 仅是那一小组 fixture 的局部值，不能填成任意数据集 recall 或 M7 完成。

若普通验收授权不涵盖该例子内部既有 ANN 索引建立/查询，应先停在源码审查，不通过删减或重写例子变相替代被阻塞链路。

### 10.2 对已生成 NDJSON 做精确距离复核

未来在证据目录创建普通非秘密配置，不改仓库：

```bash
# 文档模板。本轮不得执行。
cat > "$EVIDENCE/distances.json" <<'JSON'
{"column":"embedding","query_count":4,"top_k":10,"metrics":["l2","l1","negative_inner_product","cosine"],"max_rows":10000,"max_pairs":100000,"max_component_work":8000000}
JSON
capture import-distances "$CLI" migrate pgembedding verify \
  --input "$DATA/import-example/source.ndjson" \
  --manifest "$DATA/import-example/ndjson-manifest.json" \
  --output "$DATA/import-example/ndjson.db" \
  --distances "$EVIDENCE/distances.json" || exit 1
```

人工检查：源 hash/大小和所有 accepted typed 值、数据库计数、schema/index、游标/拒绝记录一致；数学参考来自源导出，不从待验证数据库取 reference。对每个选中 query/metric，要验证所有非 NULL 候选及 Top-K 前缀，而不只是几个命中。确定性 query 样本不是统计保证。

允许如实出现 `partial_skipped_metrics`、`skipped_zero_norm`、`skipped_no_nonnull_vectors`、近似误差界内边界歧义；这些不能改称所有距离均已通过。普通 VerifyImport 的 exact_distances 原本 `not_run`，必须显式请求距离复核才有该证据。无论创建了什么 ANN 索引，此专用距离报告的 `ann_recall` 仍是 `not_run`。

### 10.3 PG Neon 模板检查

```bash
# 文档模板。本轮不得执行。
capture migration-transition "$CLI" migrate pgembedding transition \
  --catalog testdata/pgembedding-0.3.6/catalog.json \
  --options examples/migration-transition/options.json || exit 1
```

只检查生成的复核报告、版本/依赖/不支持列表、旧 Manhattan `<~>`→`<+>` 映射、回滚/切换前置。mutation SQL 继续保持注释，不连接真实 PostgreSQL，不取消注释执行。原规范列举的 `--pg-dsn-env` 或 `--generate-neon-sql` 不能被当成当前实际存在的完整在线工具。真实用户服务需要独立授权和适用版本核对。

## 11 事务测量与长期 soak 的未来操作

本节是在既有 V4 提案基础上给出的人工操作草案；**本轮不启动，原两小时方案也未启动。** 这是普通正确性观察，不是 crash durability，不能替代物理门禁。

### 11.1 先检查实际保存位置与容量

`benchmarks/transactions/main.go` 用 `os.MkdirTemp("", ...)`，因此数据在 `TMPDIR`，不是重定向 JSON 旁边。必须固定为用户独立目录，核对 mount/可用字节/inodes，并复核报告的 `retained_data_directory` 确实是其子目录。

既有提案的工程入场门槛为：初始至少 6 GiB = 6442450944 bytes；构建后和每个 profile 启动前至少 4 GiB = 4294967296 bytes；审查建议每次至少 250000 free inodes。这些是入场余量，不是能抵御其他进程占用的硬保证，也不是原规范给定数字。

3 × 1 GiB 指 apparent database bytes，并不含 allocated blocks、journal/COW、日志/缓存/编译/临时空间、内存映射和 tmpfs 的内存账单。256 MiB Go soft memory/epoch-sampled heap 不等于 RSS 硬限制。低资源时不启动；运行中异常保留现场，不能通过删除历史、缩短时长或强杀变出 PASS。

### 11.2 构建和小样本

```bash
# 文档模板。本轮不得执行。
capture tx-harness-tests env CGO_ENABLED=0 go test -count=1 ./benchmarks/transactions || exit 1
capture tx-harness-build env CGO_ENABLED=0 go build -trimpath \
  -o "$RUNROOT/bin/transactions" ./benchmarks/transactions || exit 1
SOURCE_ID="git:$SOURCE_COMMIT"
HARDWARE_ID='填写实际CPU-OS-文件系统及共享负载条件'
capture tx-smoke-rollback "$RUNROOT/bin/transactions" -mode throughput -profile rollback \
  -transactions 64 -rows 64 -readers 2 -batch 2 -seed 20261010 \
  -source-id "$SOURCE_ID" -hardware-id "$HARDWARE_ID" || exit 1
```

64 attempts、48 commits 是部分历史样本的观测值，不是并发调度下新运行必须机械等于的固定结果。当前运行必须按 C/R/B/X/F outcome history 重建确切模型，符合 workload 覆盖和 accounting。BUSY/取消应如实统计，不默认 0。

### 11.3 两小时每 profile 提案

操作者先填实际 host label，审查资源与完整新普通 gate，再一次只启动一个 profile；不要脚本自动串行跳过失败。三个选择为 `rollback`、`wal-shared`、`encrypted-shared`，每次使用不同的记录名。

```bash
# 文档模板。本轮不得执行。每次人工审查上一 profile 后才选择下一个。
PROFILE=rollback
capture "tx-soak-2h-$PROFILE" "$RUNROOT/bin/transactions" \
  -mode soak -profile "$PROFILE" -duration 2h \
  -rows 256 -readers 4 -batch 4 -seed 20261010 \
  -writer-pause 250ms -reader-pause 2ms -snapshot-hold 2ms \
  -max-data-bytes 1073741824 \
  -source-id "$SOURCE_ID" -hardware-id "$HARDWARE_ID" || exit 1
```

完整三组至少六小时 active time，另加初始化、checkpoint、重开、验证和收尾。`-duration 2h` 是实际累积 active time，不是总 wall-clock 上限；部分维护使用 Background context，可能延长总时间。harness 结束才输出 JSON 属于预期，不加 shell timeout/watchdog，不主动制造进程退出。

每组通过都必须逐项有证据：

- 退出码 0，JSON 完整可解析，`status=passed`，`harness_revision=ordinary-transactions-v4`，`errors=[]`。
- 实际 `active_seconds >= 7200`，与 windows 汇总一致；保留 UTC start/end 与 `elapsed_seconds`。
- commits、successful readers、故意 full rollback、committed savepoint rollback/release 各有实际覆盖。
- `final_model_verified=true`、`verified_reopens > 0`；rollback checkpoint 为明确不支持的 skip，WAL profile 有相应成功 checkpoint。
- 按 seed、连续 ordinal 和 C/R/B/X/F history 独立重建最终 generation/hash，而不是只比账户总和。
- `unverified_retirement=false`、`unverified_close_handles=0`、`close_failures=0`、`shutdown=workers_joined_normal_close`；字段缺失不能按零补齐。
- 源码前后不变，目录/二进制/负载身份完整，实际保存数据与原始 report 均保留。

注意四个 reader 是启动数量；报告聚合成功数不证明每一个 reader 都持续取得进展。要披露窗口饥饿或停顿。P50/P95 是对数 histogram 桶上界，不能伪装成精确经验分位；correlated windows 的 confidence bounds 为空是诚实状态。

任何非零、失败/不完整、unsupported、quota、未知 retirement、CI 或源码身份变化，都停止启动下一个 profile，保留所有原文件，不自动重试。一个正常关闭成功不能清除另一个 reader 的 close 不确定性。

### 11.4 grams 性能和更大负载

已有普通 grams harness 可用于未来用户批准的小规模描述性观察：

```bash
# 文档模板。本轮不得执行。
capture grams-harness-build env CGO_ENABLED=0 go build -trimpath \
  -o "$RUNROOT/bin/grams-benchmark" ./benchmarks/grams || exit 1
capture grams-smoke "$RUNROOT/bin/grams-benchmark" \
  -rows 128 -queries 6 -repeats 2 -seed 2026101052 \
  -page-size 4096 -max-pages 32768 \
  -source-id "$SOURCE_ID" -environment "$HARDWARE_ID" || exit 1
```

核对每个 ordered 结果与真实 scan 100% 一致、选择性查询真实 posting work/较少 source fetch、fallback 单独统计、重开与频率一致、P50/P95/raw timing、构建时间和空间放大全部保留。不能仅据 `status=passed` 断言比扫描快。

10000 行/96 queries per length/5 builds grams 和 10000 attempts/1024 rows 事务 throughput 只是已有较大 profile 提案，未完成资源授权与运行。本文不把它们变成自动启动命令。性能比较要同机同数据、固定 source/workload/hash、独立交错重复、预声明指标/回归阈值及统计方法；INCONCLUSIVE 不能填 PASS。不能激活未部署的性能 CI 来替代人工审查。

## 12 受限人工门禁与不能编造的入口

这一节只保存真实源码入口的说明。**在当前平台禁止执行，未来也须先明确独立授权和审查；设置环境变量本身不产生授权。** 未实现的测试只列需求和需要补齐的证据，不给不存在的程序名。

### 12.1 已有有限 rollback 恢复 harness

已核对文件：`internal/pager/recovery_manual_test.go`。

- build tag 精确为 `manual_recovery && linux`
- 入口 `TestManualRollbackRecovery`
- gate `GOSQLITE_MANUAL_RECOVERY=independently_authorized`
- gate 缺失时 `t.Fatal`，文字为 NOT_RUN；这个退出不是产品恢复逻辑的失败证据

编译命令只生成测试二进制，但**本轮连编译也不执行**：

```bash
# 仅供未来经独立授权后采用；本轮 BLOCKED / NOT_RUN。
capture manual-recovery-build env CGO_ENABLED=0 go test -c -tags manual_recovery \
  -o "$RUNROOT/bin/manual-recovery.test" ./internal/pager || exit 1
```

现有精确执行形状：

```bash
# 当前平台禁止执行；不得迁移重试或作为解锁办法。
capture manual-recovery-run env GOSQLITE_MANUAL_RECOVERY=independently_authorized \
  "$RUNROOT/bin/manual-recovery.test" \
  -test.run '^TestManualRollbackRecovery$' -test.v || exit 1
```

源码中的确定性子场景包括：

- `restore-truncate-and-idempotent-reopen`：恢复两页、匹配原字节、journal 移除、再次恢复不发生 replay、正常重开。
- `original-empty-extent`：恢复原空数据库 extent。
- `preserve-checksum/cold/empty/trailing/multiple-segments/truncated/journal-limit/empty-database/canceled`：拒绝所构造的非法/不支持输入且原 db/journal 字节保持。
- `busy-shared-reader`、`busy-reserved-writer`：返回 BUSY 并保持文件。

`preserve-multiple-segments` 是该 fixture 的具体拒绝场景，不能概括为所有多段 journal 均不支持；当前 parser 已支持有界、对齐、有限记录数的部分多段格式。future reviewer 必须重新核对 fixture 与当前 parser 的一致性。

即使这些确定性场景将来通过，也只覆盖它们本身：没有真实掉电/随机 kill/short write/Sync failure/ENOSPC/wrong-offset/torn-sector/checkpoint 中断/rekey 中断证明，也没有 public Open 自动恢复的实现证明。

### 12.2 已有两个手工 namespace 回归

文件 `sqlite/write_namespace_manual_test.go` 的 tag 为 `manual_namespace`；两个真实测试名：

1. `TestNativeConnectionKeepsResolvedSymlink`
2. `TestNativeTransactionDetectsReplacedValidFile`

两者都要求 `GOSQLITE_MANUAL_NAMESPACE=independently_authorized`；未提供时 `t.Skip`。因此二进制返回 0 但全 skip 不是通过。

历史 M2 至 7addc493 的全套曾错误地包含这两个 namespace 测试，其当时 PASS 仍是事实，不能改称“历史从未运行”；但那些全套不能被重新标作完全干净的 ordinary-scope 证据。当前独立受限门禁仍是 BLOCKED / NOT_RUN。

```bash
# 仅供未来独立人工审查；本轮不编译、不运行。
capture manual-namespace-build env CGO_ENABLED=0 go test -c -tags manual_namespace \
  -o "$RUNROOT/bin/manual-namespace.test" ./sqlite || exit 1

# 当前平台禁止执行；环境变量不是授权，不能用于绕过限制。
capture manual-namespace-run env GOSQLITE_MANUAL_NAMESPACE=independently_authorized \
  "$RUNROOT/bin/manual-namespace.test" \
  -test.run '^(TestNativeConnectionKeepsResolvedSymlink|TestNativeTransactionDetectsReplacedValidFile)$' -test.v || exit 1
```

应审查的具体断言：symlink retarget 后已打开连接仍写原 first 数据库，second 不被改；valid-file replacement 后 commit 返回 `sqlite.ErrChanged`、rollback 正常、替代文件中的行保持原样。这不是所有 hostile namespace、权限或 hardlink 问题的充分验收。

### 12.3 尚未合入门控的 OS raw namespace 测试

main 的以下文件只受 OS tag 限制，**没有**上面的 manual tag/env gate：

- `internal/encstore/native_windows_test.go`：`windows && amd64`；`TestWindowsEncryptedRelativePublicationAndRetainedRoot`
- `internal/encstore/native_darwin_test.go`：`darwin`；`TestDarwinEncryptedRelativePublicationAndParentLifetime`

它们直接做 namespace publication/rename/retained-root 操作。修正候选已把原 body 搬到 manual gated 文件并增强 guard，但该候选不在当前 main。不能把 main 普通 OS 全量运行当作干净 ordinary scope。本文不提供 raw namespace 启动命令；先由人工审查精确候选和集成状态，并通过合法范围决定是否进行独立原生验收。

### 12.4 没有完整实现入口的门禁

完整 harness 缺失的部分统一标记 `IMPLEMENTATION_MISSING`；其中受平台限制的执行另记 `BLOCKED / NOT_RUN`。部分已有源码并不补齐整个矩阵。

| 门禁 | 当前可诚实说明的情况 | 将来所需材料，不是现成可执行测试 |
|---|---|---|
| 真正 crash/process-exit/power-loss | 本环境 BLOCKED / NOT_RUN；有限恢复 fixture 不是它 | 审查后的 fixture 协议、已提交/未提交外部事务账本、注入点、恢复点、存储持久化条件及逐行/索引验证 |
| I/O short write、Sync 错误、写错位、disk full、权限失败 | 没有可在本文冒充齐全通过的现成入口 | 明确的可丢弃设备/注入后端、影响范围、失败点矩阵及残留所有权证明 |
| WAL crash/checkpoint/tail publication | 普通成功路径有证据，尾部 publisher 等仍有实现缺口 | 先补真实源码/公开恢复入口，再审查其失败语义；不能先写一条 TestCrash 命令假装存在 |
| 错 key/盐/tag/单字节密文修改/跨页置换/main-WAL 不匹配 | 密码学内存向量不覆盖这些磁盘链路；BLOCKED / NOT_RUN | 审查后的测试数据与确切目标、认证失败/无秘密泄露/无错误成功/来源保持的核验 |
| interrupted publication/rekey | 正确密钥普通 rekey 不覆盖中断；BLOCKED / NOT_RUN | 有完整失败点协议，证明不存在半新半旧被误接受，保留旧/新 generation 判定与恢复规则 |
| selected-file 缺失/namespace 干预/原生 lock unmap close 失败 | 内存 owner laws 不是原生失败注入证据 | 独立原生平台和文件系统证据，所有诊断/retirement obligations 的审查 |

表中列的是原规范要求，不是对未实现功能预先颁发通过标准。只有真实入口、准确可执行规程和独立证据齐备之后，才可以制定最终场景断言并执行。不要使用 `go test -run TestCrash` 匹配不到任何测试仍退出 0 的形式蒙混。

## 13 Fuzz 与中断原因的人工审查前置

本轮不运行任何 fuzz，包括内存-only fuzz。历史有界 campaigns 保留原执行次数与源码身份；它们不能代替被中断的新 41 目标活动。

精确已知：

- 约 22:30 收到内容可能涉及网络安全风险的阻止消息；最后成功记录还在审查 targets。
- 有限只读查找未找到新的 campaign 目录、执行 log 或结果；这不证明从未发生任何未记录动作。
- 22:39 平台安全错误触发的具体 operation/target 在可用记录中未知，附近活动为 ANN V4 修正及独立评审。
- ANN V4 目录未冻结，只有 V3 copy 与一个未改字节的 review probe；不能声称 validator/engine/ownership 修复已实现。

后续人工必须先取得足够的原操作/目标/工具错误和范围信息，判断何者被拒绝、是否有未收集执行、哪些独立操作合法；无法查清时继续 UNKNOWN/BLOCKED。不得重新投递原链路、改任务措辞、拆成更小命令、换参数、换执行环境或把它迁入 CI。本文不提供 41-target campaign 或 ANN V4 修正的“恢复命令”。

## 14 故障排查与停止规则

| 现象 | 先做的只读核对 | 不允许的处理 |
|---|---|---|
| Go 版本不是 1.27.2 | PATH、工具绝对路径、`GOTOOLCHAIN` | 自动切换新 Go 后继续写同一基线 PASS |
| oracle 身份或源码 hash 不符 | 绝对路径、archive、source ID、原构建 log | 跳过 hash/改 expected/换系统 sqlite 冒充固定基线 |
| 离线模块缺失 | GOPATH/module cache/go.sum 来源 | 不告知地改网络、下载未核验模块 |
| testscope 拒绝 | GOFLAGS、实际测试 body/helper、当前 gate 源码 | 删除 guard、加豁免、改名字绕过 |
| `no tests to run` 或 SKIP | 真实函数名、package、OS/tag、env gate、oracle 选择 | 把退出 0 当目标已覆盖 |
| 数据库 unsupported | 支持矩阵、layout/mode/schema/平台 | 自动转模式、换成内存库、丢不支持对象后称通过 |
| BUSY/canceled | 正常锁/事务/cursor 生命周期与完整 error graph | 把 mixed I/O/cleanup 错误吞成普通 BUSY |
| mismatch/ErrChanged/旧回归出现 | 保留 exact query/seed/source、输出/输入 hash | 改排序/容差/断言、删除失败 fixture、静默重建 |
| quota/低空间/内存压力 | 实际 TMPDIR/mount、bytes/inodes、报表预算 | 故意挤满磁盘、删除历史、缩样本补 PASS |
| 没有 JSON、运行时间长 | 既有 session 状态、stdout/stderr 增长、维护阶段 | 强杀/timeout 后叫作正常 completion 或 recovery test |
| close/retirement 不确定 | 完整诊断与持有对象/目录记录 | 再调用一次返回 nil 就抹去第一次不确定性 |
| CLI backup/rekey 非零 | 已有结果、部分输出和原错误，先不改任何文件 | 假定未发布、自动重试、删除潜在部分输出 |
| windows vet unsafe 告警 | 精确包/命令/源位置与历史差异 | 关闭 checker 后声称原门禁通过 |
| 性能较慢/区间重叠 | 同机同数据身份、原始重复、cache/负载 | 挑最好样本、丢慢结果、捏造 SQLite/pgvector 对照 |
| 平台/审查拒绝 | 精确操作、对象、授权与原错误；保留 BLOCKED | 改写、绕行、外移或另起相同链路 |

任何错误都先保留证据。手工规程没有权限改变诊断对象、系统安全设置、存储挂载、OS 权限、CI 额度或终止其他进程。需要这些操作时另行决定和授权。

## 15 证据目录与清理

建议每次独立验收保存以下结构；这里是目录说明，不是已经生成的结果：

```text
RUNROOT/
  source.tar
  source/                         精确导出的源码副本
  bin/                            本轮构建及 hash
  data/                           显式合成数据库与导入数据
  tmp/                            明确指定的临时与 retained fixture 根
  go-cache/                       本轮编译缓存
  evidence/
    source-commit.txt
    source-tree.txt
    source-archive.sha256
    source-before.sha256
    source-after.sha256
    goal.sha256
    <step>.command
    <step>.started
    <step>.finished
    <step>.stdout
    <step>.stderr
    <step>.exit
    results.tsv                   人工逐项结论和范围
    blockers.md                   未运行 缺口 已知失败和审查意见
    host-and-storage.md            非秘密环境与资源记录
```

`results.tsv` 至少包含 case ID、source/tree/binary hash、平台、实际命令、状态、退出码、开始/结束、断言、日志路径、未覆盖范围、审核人。日志中不能保存秘密、生产数据或凭据环境。

所有未来普通步骤结束后，核对源文件未被修改：

```bash
# 文档模板。本轮不得执行。
cd "$WORK" || exit 1
find . -type f -print0 | LC_ALL=C sort -z | xargs -0 sha256sum > "$EVIDENCE/source-after.sha256" || exit 1
cmp "$EVIDENCE/source-before.sha256" "$EVIDENCE/source-after.sha256" || exit 1
sha256sum "$RUNROOT/bin/gosqlite" > "$EVIDENCE/cli-binary-after.sha256" || exit 1
cmp "$EVIDENCE/cli-binary.sha256" "$EVIDENCE/cli-binary-after.sha256" || exit 1
```

如果差异来自测试写回仓库，也仍是差异，要保留且解释；不能先 restore/删除再补“前后相同”。源码 hash 只证明所读文件身份，不证明测试实际执行；还需要终态日志和断言。

清理顺序：

1. 先确认本次所有子进程和 worker 自然结束、Rows/Tx/DB 正常收尾，无 pending retirement。不能因 shell 返回就推定原生资源状态明确。
2. 完成 `results.tsv` 与 blockers，保存全部失败/不完整/未知记录。复制或归档已经正常关闭的合成 fixture 与 evidence，校验归档可读和 hash。
3. 有异常 close、部分输出、源码变化或未知状态时，操作者默认保留整个 `RUNROOT`，不再自动做清理。这无法撤销现成示例内部已经发生的删除：第 9 节所列示例可能在失败时清理私有 fixture，普通 go test 的 t.TempDir 也会自动收尾；这些局限必须记录，不能宣称完整失败现场仍在。不要额外删除失败现场、WAL/journal/加密旧 generation 或 fuzz corpus 来让下一次通过。
4. 只在操作者再次确认路径、资料保留策略和没有未闭合对象后，手工删除自己明确生成且可丢弃的目录。本文故意不提供宽泛 `rm -rf` 或 namespace sweep。
5. 第 9 节示例会自行清理私有临时目录，其中包含错误返回时的无条件清理和未报告的清理错误；transaction harness 则保留数据目录。逐例记录，成功退出不证明任意失败路径都已安全退休，不假定所有工具行为相同。
6. 删除临时凭据/文件不是安全擦除，不能因此声称 GC、SSD、快照或旧 encrypted history 不可恢复。真实秘密根本不应进入本验收。

## 16 最终通过标准

### 16.1 普通有界复验可以怎样结论

只有对应 exact source、本次真实命令、非缓存执行、退出码、断言与证据完整，才可写：

“本次在已记录的 Linux/amd64 独立环境，对提交 X 的普通无 CGO unit/vet/build、单独 race、完整固定 SQLite oracle 和列明功能路径验收通过。受限故障恢复、安全/平台和未实现能力不在该结论内。”

不得写“所有测试都过”“SQLite 全兼容”“M0–M10 全部 VERIFIED”，除非所有相应规范门槛真实完成。focused test、编译、普通 Check、fixture recall、短期 smoke 或 CI SUCCESS 都不能单独得到这些结论。

### 16.2 完整原规范仍需关闭的事项

原始目标的完整验收至少还要：

1. 真实实现缺口完成并在精确集成源码上审查/验证，包括恢复/public API、WAL tail、通用备份、只读 WAL Check 和已声明支持的 SQL/planner 差异。
2. 通过独立合法取得的物理/崩溃/恢复/故障证据，验证已提交数据、未提交数据、表/索引原子性与所有权状态；不能用内存模拟或普通 checkpoint 替代。
3. 真实目标 OS/架构文件锁、mapping、目录持久化和加密路径证据，明确平台降级能力；跨编译不足。
4. 加密错误密钥/认证/篡改/中断 rekey 与威胁模型审查；不能用 ciphertext marker absence 或正确密钥 round-trip 代替。
5. 预声明且实际完成的长期并发 soak、大规模真实测量、可解释置信度和性能回归策略；记录所有慢结果和失败。
6. 数据格式升级/回退、用户文档/API、许可/来源、安全/密码学独立审查与适用真实迁移环境验证。
7. ANN 报告准入及未完成 fuzz 证据问题经合法人工审查解决；不知道触发原因时不得假装可恢复执行。

本手册只整理规程，当前总状态仍为 **PARTIAL / 完整验收未成立**。

## 17 来源与复核说明

本轮只读依据包括：`goal.md`、`docs/manual-verification.md`、`docs/acceptance-status.md`、新增中文 `docs/interruption-20261010.md`、`docs/compatibility.md`、`docs/upstream.md`、`docs/native-check.md`、`docs/cli-encryption.md`、`docs/import-pgembedding.md`、`docs/import-distance-verification.md`、benchmarks 的 README/源码、实际测试声明、脚本及相关 CLI 实现。

另对照原开发空间 `durable-restart/` 下的 `INTERRUPTION-20261010T2239Z.md`、`long-soak-plan-20261010/PLAN.md`、`long-soak-plan-20261010/INDEPENDENT-REVIEW.md`、`native-scope-independent-review-v1/report.md`、`ann-harness-independent-review-v3/REVIEW.md`。这些不是本仓库内路径，也不保证每个最新版本都已持久保存；各 Library 身份和仅本地材料边界见中断记录。不同时间/候选身份没有合并成一个“通过的版本”。较早文档中的历史缺口与后来的实现可能不同，以精确提交和具名专题为准。

初始只读快照中的关键文件 SHA-256，文档同期更新不反向改变这些初始读数：

- `scripts/check.sh`：`55851844f35325b38fbd368558af4e229d5bfe7c42928685e57dd799274b8f18`
- `scripts/test-oracle.sh`：`9a486b551af436f706d6018f717a2a2d0c54f9ae443c43cee10638105bf320a4`
- `docs/manual-verification.md`：`370aefba6b411769a88cab83768bece4b65606dd9c41169add950061bf1011fe`
- `docs/acceptance-status.md`：`79645fe4267453dbe419df29dae8708c0b3f6fbd7c0592cc9ba9a1b0b4eff114`
- `INTERRUPTION-20261010T2239Z.md`：`13092da4ed0e6ca3d7633dc4951a9ac5d0ab18060f6bbd55a4330d39a3a2a5d2`

本文作为 docs/manual-verification-zh.md 收入项目，保留原 docs/manual-verification.md 的历史记录；没有把历史日志复制为本轮成功证据。


## 18 P0 public Open rollback 恢复源码集成（2026-10-11）

本节补充第 3.2 / 12.1 节的历史状态，不改写其当时结论。候选从已发布
`63c2ebb10e3a2c4ff2378ee9f33c46133134c70f` 按已审查的独立 hunk 构成，
本批在已保存 `c1502790c4b233757b0f9a3c4d5e5bdee1321310` 上组合并保留
LIMIT 修正和限制分类；有限实验性源码已接入
公开 opt-in、pager retirement/path proof、driver limits 值复制及 testscope
恢复防误执行 guard。这里的源码集成、编译和 descriptor-free 模型检查，
与真实恢复运行验收是两回事。**真实 `pager.RecoverRollback`、开启选项的
真实 `Open/OpenContext`、production bridge 和所有物理恢复仍为
BLOCKED / NOT_RUN。** 自动默认恢复仍未启用，M3/全项目验收没有完成。

### 18.1 实际 API、平台和预算

- 入口为 `sqlite.Options.RollbackRecovery *sqlite.RollbackRecoveryOptions`；
  nil / 零值 Options / 未配置的注册 driver 均继续拒绝已有 journal。
  `errors.Is(err, sqlite.ErrRecoveryRequired)` 与 `CodeIOErr` 保持；presence
  诊断文本及私有包装有变化，不能按错误字符串字节或旧链形状判断兼容。
- 只有 Linux、可写、明文、rollback 模式允许非 nil。`ReadOnly` 返回
  `ErrReadOnly`；加密、`WALExclusive`、`WALShared`、
  `WALPowersafeOverwrite`、`KeepWALOnClose` 和非 Linux 返回
  `ErrUnsupported`，这些策略拒绝发生在原生 acquisition 之前。
- `MaxJournalBytes` 默认为 64 MiB，`MaxRecords` 默认为 65536（跨段累计），
  `MaxDatabaseBytes` 默认为 1 TiB 原数据库 extent；各字段 0 选择默认值。
  内存需约两倍 journal bytes 加 O(records) 元数据，不是进程 RSS 上限。
- `NoCreate: true` 不妨碍已有文件恢复，但禁止普通首次缺失文件创建。
  恢复后的 fresh probe 永不创建。API / Connector 都复制 options 指向的值；
  调用者不能在构造连接的同时并发修改配置。
- 没有 recovery CLI flag、DSN 参数或独立 public Recover 函数。
  `driver.NewConnector` 接收 Options，URI `mode=ro` 保留只读并于实际
  Connect 拒绝恢复，`mode=rw` 仍保留 NoCreate；不得静默升级权限。

下面是实际 API 的配置形状，**只展示给未来具备独立授权的人工操作者审查；
本次没有编译或运行该示例，也不能把它放进 ordinary 测试绕过 guard**。
`path` 必须由操作者选定为隔离可丢弃的已有数据库，不能指向重要数据。

```go
opts := sqlite.Options{
    NoCreate: true,
    RollbackRecovery: &sqlite.RollbackRecoveryOptions{
        MaxJournalBytes: 64 << 20,
        MaxRecords: 65536,
        MaxDatabaseBytes: 1 << 40,
    },
}
db, err := sqlite.OpenContext(ctx, path, opts)
// 必须检查 err；只有 err == nil 时才使用 db，并检查 db.Close() 的结果。
// database/sql 的配置入口：
// connector, err := driver.NewConnector("file:/absolute/disposable.db?mode=rw", opts)
// pooled := sql.OpenDB(connector) // 实际 Connect/Ping 才可能执行恢复。
```

### 18.2 本次允许的 source/model 复核

先完整审阅 [专题文档](rollback-recovery-open.md) 所列精确测试及 helper，
再运行其 guard、descriptor-free selector 命令。测试只使用内存 owner、
错误对象、callbacks 和 AST/source 字符串；不构造开启恢复的真实 Open。
若选择 race，只把同一精确 selector 换成 `CGO_ENABLED=1 go test -race`，
并先重跑 guard。不运行全套旧 guard/普通/oracle suite，更不添加 manual tags。

`internal/testscope` 能识别直接/捕获的恢复函数、恢复字段、同文件 alias/
defined type 和 dot-import 的 unkeyed Options。它不是完整类型或调用图证明：
跨文件 alias、推断 composite-literal 元素类型可漏检；同名 shadow 类型可误报；
helper / reflection 仍须人工审查。本次测试没有利用这些缺口。独立的
Rename/rename selector 扩展和 Windows/Darwin raw namespace 门控不在此切片，
不要据此启动新 OS runner 或 native aggregate。

### 18.3 未来物理验收的准确范围和停止条件

本节不给代理新的执行权限，也不提供绕过受限链路的另一套命令。
未来人工必须先取得合法独立授权并明确 fixture、原数据库/sidecar 身份、
原始/已提交/未提交账本、预期页与 schema、各阶段 evidence 和真实测试入口。
现有 `internal/pager/recovery_manual_test.go` 仍是第 12.1 节的内部 harness：
`manual_recovery && linux` 与 `GOSQLITE_MANUAL_RECOVERY=independently_authorized`
是其实际门控，**它不调用新 public Open bridge，不能作为其验收结果**。
本次没有新增 public physical harness，其状态明确为 **IMPLEMENTATION_MISSING**，
执行另列 **BLOCKED / NOT_RUN**，因此不能声称已有一条可完整覆盖它的
`go test -run ...` 命令。环境变量本身不是授权。上面的 API 配置片段也不是
fixture 生成器、物理验收 harness 或可直接运行的完整程序。

独立审查和未来 harness 至少需要逐项固定这些断言，再按许可执行：

1. nil-default 持续拒绝 journal；唯有准确 resolved path 的 private proof，
   且失败 reader 已完全退休、没有其他 cleanup 诊断，才进入显式恢复。
   wrapped/joined/synthetic `ErrRecoveryRequired` 不授权恢复。
2. 内部操作自己获取 owner 和 locks、重新验证完整 journal/database；最多
   一次恢复，成功后最多一次新的只读、no-create probe。不能复用初始 admission、
   自动 cleanup retry、递归 Open 或在 reprobe 失败后创建数据库。
3. 成功 reprobe 必须 close；任何恢复/probe/close/cancel/ownership 错误要保留
   初始与后续 cause/token，返回空 metadata。历史 `ErrRecoveryRequired` 使
   primary code 为 `CodeIOErr`；BUSY/cancel/limit 可通过 `errors.Is/As` 找到。
   它不是当前磁盘仍需恢复的证明：恢复成功后取消或重探测失败仍可使 Open 失败。
4. 支持有限记录数、已发布的 DELETE journal 多段子集；页大小 512..65536，
   recovery sector 32..65536，重复页/geometry 不符/不完整 extent 拒绝。
   cold/empty/ambiguous、super/master journal、0xffffffff no-sync count、
   legacy 未指定页大小和 unsupported header 不丢弃。不认证恶意文件内容。
5. WAL/SHM 包括空 sidecar 都阻止 rollback recovery；不截断或重建 WAL。
   原 extent 为 0 可使恢复后文件为空，fresh probe 此时拒绝且不创建。
   后续 Query/Begin/Exec 永不再次触发恢复。敌对 namespace 替换不在保证内。

遇到缺入口、受限操作、源码身份不符、owner 未退休或审核拒绝即暂停；保留
确切 operation/target/error，不能换输入、改名字、换机器/CI 或静默重试。
ANN V4、41-target fuzz、broader journals、WAL publisher、retained-directory、
backup/backend 和物理 fault/security-negative 的状态不因本节改变。


### 18.4 已取得与尚缺证据逐项记录

| 项目 | 本批准确状态 | 记录方法与不能推出的结论 |
|---|---|---|
| nil-default / policy / proof / sequence / error / option-copy / guard | 独立候选 source/model/compile PASS | tree `75c074548e8b5208beb06e6ead314d835cd16f4f`；11项精确测试各在CGO0和CGO1 race通过，无skip；只含无描述符模型及源码AST |
| guard、四包vet、全包CGO0 build、无CgoFiles闭包 | 同一候选PASS | 编译与依赖检查没有运行production bridge；不证明物理效果或其他OS运行 |
| 候选源码身份和补丁重放 | 独立PASS，07:49:35 UTC完成 | 原63c2基线、候选前后及重放身份对应；不把候选tree当成c150加P0的组合tree |
| c150加P0的完整普通/oracle/race组合 | 本节写作时PENDING | 必须记录该精确组合的终态；旧LIMIT、63c2或候选PASS不能代替 |
| 本批新提交、精确恢复和CI | 本节写作时PENDING | 提交、保存、恢复与CI分别核验；不预填未知SHA或SUCCESS |
| 实际RecoverRollback / public bridge / 启用恢复的真实Open或Connect | BLOCKED / NOT_RUN | 本轮没有调用；源码模型不能证明恢复后的文件、锁、目录或故障语义 |
| public physical harness | IMPLEMENTATION_MISSING | 现有内部手工测试不覆盖它；没有真实入口时不编造命令或测试名 |
| 更广journal/default-on/WAL publisher/其他原生平台 | 未完成的实现或策略范围 | source/model PASS不改变原缺口及物理资格限制 |

独立候选的11项精确测试名、完整命令形状、固定工具链、离线依赖、资源阈值
和原始证据身份见[公开恢复专题](rollback-recovery-open.md)。每步单独记录
实际命令、UTC起止、stdout/stderr、exit、测试匹配数/skip及源码前后哈希；
不应把本节整段提取执行，不因外层shell为0就吞掉内层FAIL。

未来public physical harness的准备还须单独规定：

1. 选择并记录精确新提交和测试源码/二进制身份，核对真实Linux/文件系统与
   持久性条件。使用独立可丢弃已有数据库家族及外部账本，不操作生产数据。
2. 按18.3逐项建立可审查fixture、预期页/schema/表索引结果、锁与retirement
   断言、恢复前后journal/DB/WAL/SHM身份和目录证据，区分成功、拒绝与不确定。
3. 先补齐且审阅public入口harness与显式独立门，再由有相应权限的操作者决定
   具体执行；当前缺少这一步，因此没有公开bridge的运行命令。
4. 任一异常、owner未退休或结果未知时停止，不再次Open、重试恢复或清理
   journal/WAL/旧generation以求成功；保留原始cause/token、完整日志及fixture。
   只有正常退休和证据保存都确认后，才按第15节人工清理自己创建的目录。

这些是未来验收协议要求，不是已经实现的测试断言或新增执行许可。更广物理
crash/fault/tamper/rekey矩阵仍按第12.4节记IMPLEMENTATION_MISSING与受限执行。
