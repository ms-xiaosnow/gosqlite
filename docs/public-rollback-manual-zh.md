# Linux 公开 Open 回滚恢复：有限手工验收

状态：四场景手工测试源码已完成独立源码审阅；固定 V3 树的 AST、默认/oracle/manual 文件选择、Linux 编译及符号检查共 6 条 Go 命令已通过（2026-10-11 08:58 UTC）。所有物理场景均 **BLOCKED / NOT_RUN**，测试二进制从未执行。最终组合树 `0af9b064bebfbec9c8270c4bf2f10ec2055fb186` 的六条同类限定检查也于09:32:51 UTC通过；其后只有文档/证据闭合更新。审阅和编译证据见[本批证据](evidence/public-recovery-harness-20261011.md)；它们不证明实际恢复或持久性。

实际入口为 [`TestManualPublicRollbackRecovery`](../sqlite/rollback_recovery_manual_linux_test.go)。它使用公开 `sqlite.OpenContext` 和 `Options.RollbackRecovery`，不直接调用内部 `RecoverRollback`。生产代码没有因本测试增加行为。

## 1. 四个具体场景

1. `nil-default-preserves-input`：nil 恢复策略返回 nil DB、`ErrRecoveryRequired` 和 `CodeIOErr`，主库和 journal 字节不变。
2. `opt-in-restores-and-reopens`：显式 opt-in 恢复到原始两页数据库，检查元数据、空 sqlite_schema、完整字节和 journal 消失；关闭后用只读且 NoCreate 的公开入口重开，字节仍不变。
3. `journal-limit-preserves-input`：`MaxJournalBytes=1` 返回 nil DB，保留 `ErrRecoveryRequired`、`ErrMemoryLimit` 和 `CodeIOErr`，主库与 journal 原样保留。
4. `no-create-preserves-absence`：对隔离目录中的缺失路径设置 NoCreate 和 opt-in，返回 nil DB 与 `os.ErrNotExist`，不生成主库、journal、WAL、SHM 或其他目录项。

前三项各自使用新的目录和确定性两页输入。输入备份、SHA256 清单和工作文件始终保留，包括 PASS 后；没有自动删除。测试关闭所有返回的 DB，保留 Close 错误；检查公开 pending-cleanup 列表为空，不隐式重试清理。原始输入备份在测试清理阶段再次逐字节核对。

这四项不覆盖驱动 Connector 实际恢复、默认自动恢复、更广 journal 格式、其他平台、WAL-tail 发布、短写、Sync/ENOSPC、断电、进程中断、torn sector、恶意目录替换、完整加密故障矩阵。相应缺失实现和 NOT_RUN 状态见[公开恢复边界](rollback-recovery-open.md)与[总验收矩阵](acceptance-status.md)。

## 2. 环境与独立授权

- 仅在你自行批准的、可丢弃的 Linux 测试环境运行。不能把环境变量当作解除平台限制的授权。
- Go **1.27.2**，CGO=0；依赖须与该提交 go.mod/go.sum 相同。这里无需 SQLite CLI；其他官方 oracle 仍固定 SQLite 3.54.0。
- 需要支持项目 Linux 本地文件锁和目录同步语义的文件系统。记录内核、架构、挂载与实际存储。tmpfs 或 `fsync=volatile` 环境的成功不能证明持久性。
- 只使用没有业务数据、没有其他进程使用的独立源码副本与全新空 fixture 目录。fixture 根路径及其组件不能是符号链接；不得把数据库原件或既有目录传入。
- 当前工程资源门为临时目录至少 1 GiB、工作空间至少 2 GiB、各至少 10000 可用 inode。不足就停；不要把阈值降低或改到其他环境绕过限制。
- 本源码有双门：`manual_recovery && linux` 编译标签，以及函数首语句要求 `GOSQLITE_MANUAL_RECOVERY=independently_authorized`。默认和 oracle 构建均应排除它。没有环境门时显式 FAIL，而不是静默 PASS。

## 3. 隔离副本和证据目录

以下命令供已满足以上条件的人工验收使用。本项目中的文档整理和编译检查没有执行这些恢复命令。先在希望验收的、已确认提交的干净仓库根目录执行；不要用正在开发的脏工作树。

```sh
set -eu
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOENV=off GOWORK=off
export GOFLAGS=-buildvcs=false GOMAXPROCS=2 CGO_ENABLED=0
unset GOSQLITE_MANUAL_RECOVERY GOSQLITE_MANUAL_NAMESPACE
test -z "$(git status --porcelain)"
COMMIT=$(git rev-parse --verify HEAD)
test "$(go version | awk '{print $3}')" = go1.27.2
test "$(uname -s)" = Linux
SOURCE=$(pwd -P)
RUN=$(mktemp -d "${TMPDIR:-/tmp}/gosqlite-public-recovery-review.XXXXXX")
mkdir "$RUN/source" "$RUN/evidence" "$RUN/fixtures"
git archive "$COMMIT" > "$RUN/source.tar"
tar -xf "$RUN/source.tar" -C "$RUN/source"
EVIDENCE="$RUN/evidence"
FIXTURES="$RUN/fixtures"
printf '%s\n' "$COMMIT" > "$EVIDENCE/commit.txt"
git rev-parse 'HEAD^{tree}' > "$EVIDENCE/tree.txt"
git status --porcelain > "$EVIDENCE/original-status.txt"
go version > "$EVIDENCE/go-version.txt"
uname -a > "$EVIDENCE/kernel.txt"
df -Pk "$RUN" /tmp > "$EVIDENCE/disk.txt"
df -Pi "$RUN" /tmp > "$EVIDENCE/inodes.txt"
findmnt -T "$RUN" > "$EVIDENCE/mount.txt"
printf '源码原件=%s\n提交=%s\n隔离目录=%s\n证据=%s\nfixture=%s\n' "$SOURCE" "$COMMIT" "$RUN" "$EVIDENCE" "$FIXTURES"
cd "$RUN/source"
go env GOOS GOARCH CGO_ENABLED GOCACHE GOPATH GOTMPDIR > "$EVIDENCE/go-env.txt"
go mod verify > "$EVIDENCE/modules.txt"
```

先检查证据中的空间、inode、挂载和 Go 信息。`findmnt` 是 Linux util-linux 命令；缺失时停止并自行补齐环境信息，不把空文件当作检查通过。离线模块缓存不存在时 `go mod verify`/后续编译会失败，应先核验依赖来源再准备缓存，不能悄悄取消版本锁或下载限制。

## 4. 先检查选择与编译

```sh
go list -f '{{join .XTestGoFiles "\n"}}' ./sqlite > "$EVIDENCE/default-files.txt"
go list -tags oracle -f '{{join .XTestGoFiles "\n"}}' ./sqlite > "$EVIDENCE/oracle-files.txt"
go list -tags manual_recovery -f '{{join .XTestGoFiles "\n"}}' ./sqlite > "$EVIDENCE/manual-files.txt"
python3 - "$EVIDENCE" <<'PY'
import pathlib, sys
root = pathlib.Path(sys.argv[1])
name = 'rollback_recovery_manual_linux_test.go'
for file, expected in [('default-files.txt', 0), ('oracle-files.txt', 0), ('manual-files.txt', 1)]:
    actual = (root / file).read_text().splitlines().count(name)
    if actual != expected:
        raise SystemExit(f'{file}: expected {expected}, got {actual}; STOP')
print('source selection: PASS (default=0 oracle=0 manual=1)')
PY
go test -c -p=1 -vet=off -tags manual_recovery -o "$EVIDENCE/public-recovery.test" ./sqlite
go tool nm "$EVIDENCE/public-recovery.test" > "$EVIDENCE/symbols.txt"
grep -F 'sqlite_test.TestManualPublicRollbackRecovery' "$EVIDENCE/symbols.txt"
sha256sum sqlite/rollback_recovery_manual_linux_test.go "$EVIDENCE/public-recovery.test" > "$EVIDENCE/source-binary-sha256.txt"
```

默认与 oracle 两份列表都必须没有该文件；manual 列表必须有且只有一次。符号表必须包含具名测试。这里只编译并读取二进制，未执行恢复。不要用 `go test -tags manual_recovery ./...` 替代，那个命令会扩大执行集合。

## 5. 独立人工执行与证据

只有已经完成源码审阅、隔离检查且具备独立执行授权后，才执行以下单项命令。确认当前 shell 中 RUN/EVIDENCE/FIXTURES 仍指向第3节刚创建的唯一空目录。不得复用失败目录重跑；失败现场须保留。

```sh
date -u +%FT%TZ > "$EVIDENCE/runtime-start.txt"
set +e
GOSQLITE_MANUAL_RECOVERY=independently_authorized \
GOSQLITE_MANUAL_RECOVERY_DIR="$FIXTURES" \
"$EVIDENCE/public-recovery.test" \
  -test.run '^TestManualPublicRollbackRecovery$' -test.count=1 -test.v -test.timeout=0 \
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
printf '退出码=%s；证据保留于 %s\n' "$RC" "$EVIDENCE"
```

通过标准：退出码0，具名顶层测试及上述四个子测试全部 PASS，没有 SKIP/FAIL；字节、错误码/原因、journal 消失、NoCreate 路径、Close 与 pending-owner 断言均被执行；输入清单及运行前后环境、源码/二进制身份齐全。任一场景失败后停止后续场景；Close 失败不继续只读重开；最终只读记录 pending-owner，不隐式重试清理。已建成的三份原始输入备份及 input-sha256.json 在清理阶段全部检查，单项失败不阻止检查其他备份。输出仅有顶层 PASS 而没有四个子项、选择器没有匹配、门控导致停止或未运行，都不得记 PASS。即使四项通过，也只属于这个提交、Linux/实际架构、所用文件系统及有限场景。

## 6. 失败排查与清理

- build/tag/符号失败：先检查版本、GOOS/GOARCH、源码身份和文件名，不执行未知二进制。
- 环境门或目录检查失败：记录为 NOT_RUN/前置条件失败；不要改源码绕过门，不使用非空目录。
- CodeIOErr、ErrMemoryLimit 或原样保留断言不符：保存日志与所有 fixture，定位公开桥、原因合并与实际字节差异，记录 FAIL；不能改期望让它通过。
- 锁/目录 Sync/Close/pending-owner 失败：保留精确错误和路径，不重复调用清理 API 或删除数据库；先确认是否还有活跃句柄。tmpfs/fsync=volatile 的结果必须附环境限制。
- 任何异常中断：记录 UNKNOWN/未完整运行，保留目录与二进制；不以部分 PASS 代替整体。

先将整个 RUN 目录复制或归档到你自己的证据存储，并重新核对哈希。确认不再有测试进程、需要保留的现场已可恢复后，再由你人工删除这次打印出的唯一临时 RUN 目录。不要删除源码原件、已有数据库或其他运行的目录；本手册没有自动删除步骤。
