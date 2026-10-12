# Linux Connector 回滚恢复：四场景手工入口

## 当前五切片组合状态（2026-10-12）

当前冻结运行树为 `59ce26879c4301babc7648fbddc5cc66c4097263`（3,267 文件、783 个 Go 文件），包含 LEFT/Rows、encrypted inspect、Darwin inventory manual、Connector rollback manual 和 automatic UNIQUE（含 c382 空结果表头 helper 修复）；**不包含 CHECK**。该树于 **2026-10-12 05:21:06 UTC 自然完成完整 26 门 / 28 次 Go 调用 PASS**，lease 已 RELEASED。ordinary/race 各 **8,569 named PASS / 7 个固定预期 SKIP**，完整 pinned oracle **10,301 PASS / 0 SKIP**，LIMIT oracle/race **394 PASS**。新检查点提交、Git bundle、精确恢复及对应 CI 均 **PENDING**；已验证的未发布源包不代替这些步骤。

已发布 main 仍为 `8bc189474bdb2b0a683c6c059992f9acbc426250`，其 bundle、恢复 26 门和两项 CI 已闭合，只资格该精确基点。当前范围、具名输入收据和真实代码/原生余缺以[当前状态](current-status-20261012.md)及[组合证据](evidence/combined-checkpoint-20261012.md)为准。下文带旧日期的 SOURCE_ONLY、NOT_RUN、PENDING、FAIL/HOLD/UNKNOWN 均保留原时点含义；历史失败不回写，手工、物理及 M3/M8/M10 门不因此开放。

## 历史源码准备状态与不变的手工合同

状态：**SOURCE_ONLY / NOT_RUN**（2026-10-12）。本增量以提交
`8bc189474bdb2b0a683c6c059992f9acbc426250`、tree
`c0959fb87e8aaedd946e6bb7abbaddb67cf5ced0` 为只读基点。
这不是该基点已有的运行结果：固定官方 gofmt 已完成，仅 sourceguard 的对齐
发生变化，手工测试源码字节未变。两个 AST 根、三项 go-list、一次 test-c 和
一次只读 nm 这六个静态门尚未执行；原生恢复、fixture 和手工测试仍为 NOT_RUN。
未执行 oracle；独立审阅也不能替代执行证据。
完整新 tree、逐文件 SHA-256、精确差异及读文件/辅助函数闭包应与交付的
树外 source-evidence 一起核验，不从历史 8bc PASS 推定本增量 PASS。

实际入口是
[`TestManualConnectorRollbackRecovery`](../sqlite/connector_rollback_recovery_manual_linux_test.go)。
它在外部 `sqlite_test` 包通过 `driver.NewConnector` → `Connector.Connect`
→ 既有 `sqlite.OpenContext` 路径触发策略；没有新生产行为，没有绕过公开桥。
既有 [`TestManualPublicRollbackRecovery`](../sqlite/rollback_recovery_manual_linux_test.go)
及其 fixture、O_EXCL 写入、文件/目录 Sync、输入备份和哈希保留辅助函数原样复用。
两入口独立选择；不得用旧 public Open 的编译/运行结果替代 Connector 结果。

## 1. 四个有界场景及断言

依次运行，任一子项或其清理失败就停止后续场景，没有 Parallel。

1. `nil-default-preserves-input`：NewConnector 捕获 nil policy；随后把调用方
   Options 的 policy 改为非 nil，Connect 仍返回 nil connection、CodeIOErr 和
   ErrRecoveryRequired，主库/journal 完整字节不变，WAL/SHM 不存在。
2. `opt-in-restores-and-reconnects`：NewConnector 捕获零值 opt-in policy；
   随后把调用方三个限额全部改为 1，Connect 仍按已捕获的默认限额恢复。
   用 adapter Prepare → Stmt.Query → Rows.Next 读取空 sqlite_schema，列名是
   name，严格 EOF，无清理错误。完成行、statement、connection 退休后，主库
   与原始两页输入逐字节一致，journal/WAL/SHM 不存在。另建只读、NoCreate、
   nil-policy Connector 再连接、执行同样的逻辑读、关闭并复核同样字节与缺失。
3. `journal-limit-preserves-input`：捕获 MaxJournalBytes=1 后把调用方改回 0，
   Connect 仍返回 nil connection、CodeIOErr、ErrRecoveryRequired 和
   ErrMemoryLimit，主库/journal 字节不变，WAL/SHM 不存在。
4. `no-create-preserves-absence`：全新子目录中的缺失路径；捕获 NoCreate=true
   和 opt-in 后把调用方 NoCreate 改为 false。Connect 仍返回 nil connection
   和 os.ErrNotExist。构造前、构造后、Connect 后都检查主库、journal、WAL、
   SHM 缺失，目录没有任何条目。

每次 NewConnector 后、Connect 前先检查输入不变或目录仍为空，以及 pending
列表为空。这些是构造阶段没有可观察文件变化的断言；配套 AST 检查另固定构造器
及其 parseDSNMode 的无 I/O 调用闭包，不能把“字节没变”单独当作任意无 I/O 证明。
当前 RollbackRecoveryOptions 的三项均为标量，按值复制整个结构即为独立配置快照；
不覆盖并发修改调用方 Options，也不声称未来新增引用字段自动满足深拷贝。

前三项各有独立 fixture 子目录。original.input.db、current.input.db、
journal.input.bin 和 input-sha256.json 这些原始输入快照及清单始终保留。
成功恢复会由既有恢复实现按预期删除工作 journal（recovery.db-journal）；原始
journal.input.bin 不受影响。测试辅助代码不会另行删除剩余工作文件或清理目录；
失败后保留当时现场，不把已发生的恢复操作自动回退。输入备份在 t.Cleanup 中
重新逐字节检查。合成当前库只是确定性准备输入，没有制造崩溃、短写或故障，
不证明现实断电恢复。

仅拥有本场景实际返回的 adapter connection、statement、rows。显式 Close
在调用前标记，最多尝试一次。固定版本 driver.Rows.Next 的终止路径会在内部
关闭 native Rows；辅助函数在 Next 前标记可能的退休，只有精确 io.EOF 证明
成功，之后不重复 Close。非 EOF 错误、Close 错误或未完成的退休使作用域停止
后续关闭，保留保护它的对象和完整错误；清理按逆取得顺序进行。已知 poisoned、
pending-cleanup、commit-uncertain 诊断不能被预期拒绝的 sentinel 匹配掩盖。
只读枚举全局 PendingPlaintextCleanups，绝不重试或处置别人的 owner。
空 pending 列表本身不证明全部原生句柄已退休。

## 2. 权限、环境及仍未实现的范围

以下命令全部是**未来独立人工验收规程，本次未执行**。环境变量只是代码门，
不是解除限制的授权。当前被阻断的 recovery、ANN V4、41-target campaign、
未知中断链不得执行、重启或转移到 CI/别的执行器绕过限制。
只有在另行取得允许该物理操作的独立环境及执行授权、完成源码/命令/依赖审阅后，
才可执行对应步骤；先获得源码/编译资格，不等于可运行物理测试。

- Linux，Go **1.27.2**，CGO_ENABLED=0；只用该候选 go.mod/go.sum 的离线已核验
  依赖：golang.org/x/crypto v0.58.0、x/sys v0.49.0、x/text v0.43.0。
  不依赖 SQLite CLI；项目其他 oracle 的 3.54.0 基线未改变。
- 独立源码副本、全新空 fixture 根，无业务数据，无其他使用者，无并发路径替换。
  根必须已存在、绝对、clean、不是系统根、没有符号链接或符号链接组件。
  代码不验证“没有其他使用者”，须由人工隔离保证。
- 记录实际内核、架构、文件系统、挂载和存储。须支持本项目 Linux 本地锁及目录
  同步语义；远程/不明文件系统停止验收。tmpfs、fsync=volatile 等只可标注其局限，
  不能作为持久性资格。此入口没有文件系统或内核版本资格检测器。
- 资源门保持临时目录至少 1 GiB、源码工作空间至少 2 GiB，各至少 10000 可用
  inode。空间不足、模块缺失或环境不符就停，不降低门槛或悄悄切换缓存/机器。
- 编译门为 `manual_recovery && linux`；函数第一语句要求
  `GOSQLITE_MANUAL_CONNECTOR_RECOVERY=independently_authorized`，后续读取
  `GOSQLITE_MANUAL_CONNECTOR_RECOVERY_DIR`。缺门是 FAIL/NOT_RUN，不是 SKIP/PASS。
  旧 `GOSQLITE_MANUAL_RECOVERY` 不授权本入口。

四场景源码不再缺失，但这只是有限 Connector happy-path/拒绝矩阵。
更广 journal 形状、其他平台、冷 journal 政策、WAL-tail 发布、短写、Sync/ENOSPC、
close/lock 失败、进程中断、断电、torn-sector、恶意 namespace 替换、加密故障以及
完整 fault/crash/restart 矩阵仍 **IMPLEMENTATION_MISSING / BLOCKED / NOT_RUN**，
应按具体项目区分源码缺口与执行缺口。没有 M3/M8/M10、生产或整目标完成声明。

## 3. 隔离源码、fixture 与证据

从含新增文件的、已独立审阅的候选源码开始，不要仅检出旧 8bc 后直接执行。
人工设置 REVIEWED_SOURCE 为那份只读、无活动修改、可核对完整清单的绝对路径。
先核对交付清单/新 tree，然后再运行以下准备步骤；它们也不是本次执行记录。

```sh
set -eu
: "${REVIEWED_SOURCE:?先指定已审阅候选源码的绝对路径}"
SOURCE=$(cd "$REVIEWED_SOURCE" && pwd -P)
test -f "$SOURCE/sqlite/connector_rollback_recovery_manual_linux_test.go"
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOENV=off GOWORK=off
export GOFLAGS=-buildvcs=false GOMAXPROCS=2 CGO_ENABLED=0
unset GOSQLITE_MANUAL_CONNECTOR_RECOVERY GOSQLITE_MANUAL_CONNECTOR_RECOVERY_DIR
unset GOSQLITE_MANUAL_RECOVERY GOSQLITE_MANUAL_RECOVERY_DIR GOSQLITE_MANUAL_NAMESPACE
test "$(uname -s)" = Linux
test "$(go version | awk '{print $3}')" = go1.27.2
RUN=$(mktemp -d "${TMPDIR:-/tmp}/gosqlite-connector-recovery-review.XXXXXX")
RUN=$(cd "$RUN" && pwd -P)
mkdir "$RUN/source" "$RUN/evidence" "$RUN/fixtures"
EVIDENCE="$RUN/evidence"
FIXTURES="$RUN/fixtures"
tar --exclude='./.git' -cf "$EVIDENCE/source.tar" -C "$SOURCE" .
tar -xf "$EVIDENCE/source.tar" -C "$RUN/source"
printf 'source=%s\nrun=%s\nfixtures=%s\n' "$SOURCE" "$RUN" "$FIXTURES" > "$EVIDENCE/paths.txt"
sha256sum "$EVIDENCE/source.tar" > "$EVIDENCE/source-archive-sha256.txt"
uname -a > "$EVIDENCE/kernel.txt"
findmnt -T "$RUN" > "$EVIDENCE/mount.txt"
df -Pk "$RUN" /tmp > "$EVIDENCE/disk.txt"
df -Pi "$RUN" /tmp > "$EVIDENCE/inodes.txt"
go version > "$EVIDENCE/go-version.txt"
cd "$RUN/source"
go env GOOS GOARCH CGO_ENABLED GOCACHE GOPATH GOTMPDIR > "$EVIDENCE/go-env.txt"
go mod verify > "$EVIDENCE/modules.txt"
```

findmnt 属于 Linux util-linux；缺失、返回不完整、无法确定实际存储就停止。
逐项人工核对资源门和记录内容。离线模块不足时，先独立验证并准备来源，不取消
GOPROXY/GOSUMDB 锁。保存原候选清单，检查隔离副本每个文件字节/模式与它一致；
无清单或源码活动修改时不能继续。tar SHA-256 固定本次副本，不代替逐文件比对。

## 4. 先做源检查、文件选择与编译，仍不运行 manual binary

此章节需要单独批准的有限 Go 检查资格。这里的所有新检查在本次交付均 NOT_RUN。

```sh
go run -p=1 ./internal/testscope > "$EVIDENCE/ordinary-source-guard.txt" 2>&1
go test -p=1 -vet=off ./internal/testscope \
  -run '^(TestConnectorRollbackManualSourceContract|TestConnectorRollbackManualSourceMutationCounterexamples)$' \
  -count=1 -v -timeout=0 > "$EVIDENCE/connector-source-contract.txt" 2>&1
go list -p=1 -f '{{join .XTestGoFiles "\n"}}' ./sqlite > "$EVIDENCE/default-files.txt"
go list -p=1 -tags oracle -f '{{join .XTestGoFiles "\n"}}' ./sqlite > "$EVIDENCE/oracle-files.txt"
go list -p=1 -tags manual_recovery -f '{{join .XTestGoFiles "\n"}}' ./sqlite > "$EVIDENCE/manual-files.txt"
python3 - "$EVIDENCE" <<'PY'
from pathlib import Path
import sys
root = Path(sys.argv[1])
for name in ['connector_rollback_recovery_manual_linux_test.go',
             'rollback_recovery_manual_linux_test.go']:
    for file, expected in [('default-files.txt', 0), ('oracle-files.txt', 0),
                           ('manual-files.txt', 1)]:
        actual = (root / file).read_text().splitlines().count(name)
        if actual != expected:
            raise SystemExit(f'{file}: {name}: expected {expected}, got {actual}; STOP')
print('source selection: PASS for both manual files (0/0/1)')
PY
go test -c -p=1 -vet=off -tags manual_recovery \
  -o "$EVIDENCE/connector-recovery.test" ./sqlite
go tool nm "$EVIDENCE/connector-recovery.test" > "$EVIDENCE/symbols.txt"
grep -F 'sqlite_test.TestManualConnectorRollbackRecovery' "$EVIDENCE/symbols.txt"
sha256sum sqlite/connector_rollback_recovery_manual_linux_test.go \
  sqlite/rollback_recovery_manual_linux_test.go driver/driver.go sqlite/rows.go \
  sqlite/rollback_recovery.go internal/testscope/connector_rollback_manual_test.go \
  "$EVIDENCE/connector-recovery.test" > "$EVIDENCE/source-binary-sha256.txt"
```

新的 guard 只有两个顶层纯源码测试，读取五个固定文件；反例均是内存中的文本，
不会构造 Connector、数据库或 fixture。它固定完整源哈希，并以 AST 检查双标签、
第一语句环境门、四个串行子项、五次构造/连接、配置变更顺序、退休及读调用路径。
它没有接入或改写生产 testscope/main.go；既有 main 的精确 manual 标签排除不变。
哈希或合同失败时先审查真实差异，不能仅刷新 pin。AST/字符串合同不是完整控制流、
反射或全依赖行为证明，仍须审阅实际辅助函数和 import/init 闭包。

每条命令都要另记 argv、UTC 起止、自然 exit、stdout/stderr；任一错误、SKIP、
未命中 selector 或异常活动都停止。default/oracle 两份文件列表均须排除两文件，
manual 列表均各出现一次；最终符号表须存在精确顶层符号，不能仅靠子函数匹配。
编译/符号检查不执行测试二进制，连 gate-off、-test.list、无匹配选择器探测都不是
本阶段权限。不要用 `go test -tags manual_recovery ./...` 扩大集合。

## 5. 仅在独立原生资格已获批准后运行这一入口

再次确认 RUN/EVIDENCE/FIXTURES 仍指向刚才唯一的隔离目录；fixture 根仍为空。
本步骤与上一节的源码/编译资格分开，当前任务不会运行它。确切 selector 如下：

```sh
date -u +%FT%TZ > "$EVIDENCE/runtime-start.txt"
printf '%s\n' 'GOSQLITE_MANUAL_CONNECTOR_RECOVERY=independently_authorized' \
  "GOSQLITE_MANUAL_CONNECTOR_RECOVERY_DIR=$FIXTURES" \
  "$EVIDENCE/connector-recovery.test -test.run ^TestManualConnectorRollbackRecovery$ -test.count=1 -test.parallel=1 -test.v -test.timeout=0" \
  > "$EVIDENCE/runtime-command.txt"
set +e
GOSQLITE_MANUAL_CONNECTOR_RECOVERY=independently_authorized \
GOSQLITE_MANUAL_CONNECTOR_RECOVERY_DIR="$FIXTURES" \
"$EVIDENCE/connector-recovery.test" \
  -test.run '^TestManualConnectorRollbackRecovery$' -test.count=1 \
  -test.parallel=1 -test.v -test.timeout=0 \
  > "$EVIDENCE/runtime.log" 2>&1
RC=$?
set -e
printf '%s\n' "$RC" > "$EVIDENCE/runtime-exit.txt"
date -u +%FT%TZ > "$EVIDENCE/runtime-finish.txt"
cat "$EVIDENCE/runtime.log"
python3 - "$FIXTURES" "$EVIDENCE/fixture-sha256.json" <<'PY'
import hashlib, json, pathlib, sys
root = pathlib.Path(sys.argv[1])
items = {str(p.relative_to(root)): {'bytes': p.stat().st_size,
         'sha256': hashlib.sha256(p.read_bytes()).hexdigest()}
         for p in sorted(root.rglob('*')) if p.is_file()}
pathlib.Path(sys.argv[2]).write_text(json.dumps(items, indent=2) + '\n')
PY
printf 'exit=%s; evidence=%s\n' "$RC" "$EVIDENCE"
```

通过要求：自然 exit 0；精确顶层测试和上列四个子项全部 PASS，零 SKIP/FAIL；
字节、错误类型/原因、配置快照、健康逻辑读、只读 reconnect、NoCreate、输入
保留、Close/隐式 EOF 退休及 pending-owner 断言全部执行。缺任一子项、只有
顶层 PASS、被门阻挡或编译成功都不能算运行通过。输入 manifest、实际环境、
源码/二进制/依赖身份和完整日志必须齐全。即使通过，也仅对应精确候选及当时的
Linux/架构/文件系统和四个场景，不推广到故障矩阵。

## 6. 失败保留、停止与人工清理

- 构造/恢复/字节/原因/列名/EOF 不符：保留 FAIL 和全部输入、工作文件、日志，
  不修改期望，不用另一入口掩盖失败，不复用非空目录重跑。
- Close、Rows.Next 或 pending-owner 异常：停止后续场景，保留所有未知 owner、
  保护对象和诊断。没有自动 RetryPlaintextCleanup、重复 Close、删除或重建。
- 任何意外中断或没有自然终态：记 UNKNOWN，保持原现场；没有自动 timeout、
  signal、kill 或子进程清理。没有终态不表示成功，也不授权重启原未知链。
- 源码/选择/编译/pin 失败：只处理当前实际阻断；没有已审源码和二进制身份就不运行。

原始输入快照、input-sha256.json 和完整运行日志必须保留。先保存整个 RUN 目录
到自己的证据存储并逐字节核验。只有用户明确确认本次隔离工作副本的完整路径，
且确认测试进程已自然退出、所有相关 owner/句柄均已退休、输入与日志的保留副本
可恢复后，才可由人工清理该隔离工作副本中的剩余工作文件。未知 owner 未解决时
不得清理；pending 列表为空不够。绝不删除真实数据库、真实数据库的 journal、
源码原件、其他运行目录或受保护现场；保留的原始输入与日志不属于清理对象。
本手册不提供自动删除命令。
