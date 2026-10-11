# CLI 与示例资源边界复验

适用范围：包含 query 关闭错误合并、readonly 显式只读、write 资源归还后输出、sparse-exact 状态传递四项修正的源码。它们分别对应 goal G1、§12.1 和 §12.2。旧中文手册固定的 `776b23e…` 不含这些修正；不要把新测试命令套在旧提交上。完整验收状态仍见 [验收矩阵](acceptance-status.md)。

本文是操作者后续复验规程，编写命令不等于执行命令。开发过程中已有的限定结果单独列入对应证据；实际进程退出、信号取消及原生 Close/Rollback 故障不因 AST 断言而成为已运行。

## 1 环境与隔离副本

先按 [中文手工验收手册](manual-verification-zh.md) 核对来源、执行边界和证据保存规则。使用已经授权取得的本项目完整本地 Git 仓库，进入所选修正提交的干净 checkout。下列步骤只生成独立源码/证据副本，不更改原 checkout、远端或系统配置。需要 Go 1.27.2、Git、Python 3 和 shell；race 另需该 Go 版本支持的本机 C 工具链。普通生产构建仍禁用 CGO。模块必须已取得且校验，离线缺依赖时停止，不能把缺依赖记为测试失败或自动更换来源。

```sh
set -eu
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOFLAGS=-buildvcs=false
unset TMPDIR GOTMPDIR TMP TEMP
SOURCE_REPO=$(pwd)
MODULE_CACHE=$(go env GOMODCACHE)
test -d "$MODULE_CACHE"
SOURCE_COMMIT=$(git rev-parse HEAD)
SOURCE_STATUS=$(git status --porcelain)
test -z "$SOURCE_STATUS"
SESSION=$(mktemp -d /tmp/gosqlite-cli-resource-review.XXXXXX)
mkdir "$SESSION/source" "$SESSION/evidence" "$SESSION/gopath" "$SESSION/gocache" "$SESSION/bin"
git archive --output="$SESSION/source.tar" "$SOURCE_COMMIT"
tar -x -f "$SESSION/source.tar" -C "$SESSION/source"
EVIDENCE="$SESSION/evidence"
printf '%s\n' "$SOURCE_COMMIT" > "$EVIDENCE/source-commit.txt"
git rev-parse "$SOURCE_COMMIT^{tree}" > "$EVIDENCE/source-tree.txt"
cd "$SESSION/source"
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOFLAGS=-buildvcs=false
export GOMAXPROCS=2 CGO_ENABLED=0
export GOPATH="$SESSION/gopath" GOCACHE="$SESSION/gocache" GOMODCACHE="$MODULE_CACHE"
test -w "$GOPATH" && test -w "$GOCACHE"
go version > "$EVIDENCE/go-version.txt"
go env GOVERSION GOHOSTOS GOHOSTARCH GOOS GOARCH CGO_ENABLED GOTOOLCHAIN GOPROXY GOSUMDB GOPATH GOCACHE GOMODCACHE GOFLAGS > "$EVIDENCE/go-env.txt"
test "$(go env GOVERSION)" = go1.27.2
test "$(go env GOHOSTOS)/$(go env GOHOSTARCH)" = linux/amd64
test "$(go env GOOS)/$(go env GOARCH)" = linux/amd64
printf '%s\n' "$SESSION"
```

Linux/amd64 是现有普通运行证据范围；其他目标交叉编译不代表原生文件锁或数据库操作通过。空间不足、默认 tmpfs 或 `fsync=volatile` 环境不能证明断电持久性。不要在收到拒绝或资源不足之后换目录/执行器继续同一受阻流程。

在单独新开的 shell 中逐段执行，不把验收段粘贴进承载生产工作或其他未保存任务的 shell。记录器先独占创建每个标签目录，再保存 JSON argv 和 cwd，日志初始化失败时不会启动命令；重复标签会停止。若命令执行后写入结果失败，保留目录并记为证据不完整，不重跑。保存原始 stdout、stderr、状态和时间；失败立即停止本段，不覆盖日志或自动重试：

```sh
cat > "$EVIDENCE/capture.py" <<'PY'
from pathlib import Path
import datetime, json, os, subprocess, sys
base, label, *argv = sys.argv[1:]
if not label or label in ('.', '..') or '/' in label or not argv:
    raise SystemExit('Invalid evidence label or empty command')
entry = Path(base) / label
entry.mkdir()  # exclusive: an existing label stops before launching anything
now = lambda: datetime.datetime.now(datetime.timezone.utc).isoformat()
with (entry / 'command.json').open('x') as f:
    json.dump({'argv': argv, 'cwd': os.getcwd(), 'started_utc': now()}, f)
with (entry / 'stdout.txt').open('xb') as out, (entry / 'stderr.txt').open('xb') as err:
    result = subprocess.run(argv, stdout=out, stderr=err, check=False)
with (entry / 'result.json').open('x') as f:
    json.dump({'returncode': result.returncode, 'finished_utc': now()}, f)
with (entry / 'exit.txt').open('x') as f:
    f.write(str(result.returncode) + '\n')
raise SystemExit(result.returncode if result.returncode >= 0 else 128 - result.returncode)
PY
capture() {
    python3 "$EVIDENCE/capture.py" "$EVIDENCE" "$@"
}
```

## 2 完整源码前后身份

本段脚本只读取文件并计算清单，不运行数据库。它排除自身输出，因为输出位于 source 之外。目录名、文件字节、大小、原始权限和 Git 可执行模式全部比较；不比较读取时间戳，不把身份检查当作运行验收。

```sh
cat > "$EVIDENCE/source-identity.py" <<'PY'
from pathlib import Path
import hashlib, json, stat, sys
root = Path.cwd()
entries = {}
for p in sorted(root.rglob('*')):
    name = p.relative_to(root).as_posix()
    mode = p.lstat().st_mode
    if stat.S_ISDIR(mode):
        entries[name] = {'kind': 'directory', 'mode': stat.S_IMODE(mode)}
    elif stat.S_ISREG(mode):
        entries[name] = {'kind': 'file', 'mode': stat.S_IMODE(mode), 'size': p.stat().st_size, 'git_mode': '100755' if mode & 0o111 else '100644', 'sha256': hashlib.sha256(p.read_bytes()).hexdigest()}
    else:
        raise SystemExit('Unexpected non-regular source entry: ' + name)
Path(sys.argv[1]).write_text(json.dumps(entries, sort_keys=True, indent=2) + '\n')
PY
python3 "$EVIDENCE/source-identity.py" "$EVIDENCE/source-before.json"
final_identity() {
    original_rc=$1
    identity_rc=0
    python3 "$EVIDENCE/source-identity.py" "$EVIDENCE/source-after.json" > "$EVIDENCE/identity-final.stdout.txt" 2> "$EVIDENCE/identity-final.stderr.txt" || identity_rc=$?
    compare_rc=0
    if [ "$identity_rc" -eq 0 ]; then
        cmp "$EVIDENCE/source-before.json" "$EVIDENCE/source-after.json" > "$EVIDENCE/identity-compare.stdout.txt" 2> "$EVIDENCE/identity-compare.stderr.txt" || compare_rc=$?
    else
        compare_rc=1
    fi
    printf '%s\n' "$original_rc" > "$EVIDENCE/original-command.exit.txt" || identity_rc=1
    printf '%s\n' "$identity_rc" > "$EVIDENCE/identity-final.exit.txt" || identity_rc=1
    printf '%s\n' "$compare_rc" > "$EVIDENCE/source-comparison.exit.txt" || identity_rc=1
    if [ "$original_rc" -ne 0 ]; then return "$original_rc"; fi
    test "$identity_rc" -eq 0 && test "$compare_rc" -eq 0
}
trap 'original_rc=$?; trap - EXIT; final_identity "$original_rc"; exit $?' EXIT
```

## 3 精确普通选择

以下 18 个顶层测试均有真实实现。read-only 组合仅复制完整固定 fixture 并正常读取；write 使用成功提交、正常主动回滚、重新打开和受支持配置 Check；query 的错误输出仅来自内存 Writer；旧程序只被 AST 解析，绝不调用旧 main。不要扩大为受限标签或旧中断链路。

```sh
SPARSE='^(TestSparseExactExample|TestSparseExactRun|TestSparseExactMainStatusWiring|TestSparseExactMainStatusWiringRejectsLegacy)$'
QUERY='^(TestQuerySourceJoinsRowsCloseError|TestQuerySourceRejectsLegacyRowsClose|TestQueryOutputWriterError|TestQueryReadonlyFixture|TestSQLWriteAndQuery)$'
READONLY='^(TestReadonlyExampleIntactFixtures|TestReadonlyRunArguments|TestReadonlyWriteRow|TestReadonlyReportResult|TestReadonlySourceContract|TestReadonlyLegacySourceContract)$'
WRITE='^TestWriteExample(LifecycleAST|ArgumentStatus|OrdinarySuccessReopen)$'
capture modules-before go mod verify || exit $?
capture scope-unit go run ./internal/testscope || exit $?
capture sparse-unit go test -count=1 -timeout=60s -v -run "$SPARSE" ./examples/sparse-exact || exit $?
capture query-unit go test -count=1 -timeout=60s -v -run "$QUERY" ./cmd/gosqlite || exit $?
capture readonly-unit go test -count=1 -timeout=60s -v -run "$READONLY" ./examples/readonly || exit $?
capture write-unit go test -count=1 -timeout=60s -v -run "$WRITE" ./examples/write || exit $?
capture scope-race go run ./internal/testscope || exit $?
capture sparse-race env CGO_ENABLED=1 go test -race -count=1 -timeout=60s -v -run "$SPARSE" ./examples/sparse-exact || exit $?
capture query-race env CGO_ENABLED=1 go test -race -count=1 -timeout=60s -v -run "$QUERY" ./cmd/gosqlite || exit $?
capture readonly-race env CGO_ENABLED=1 go test -race -count=1 -timeout=60s -v -run "$READONLY" ./examples/readonly || exit $?
capture write-race env CGO_ENABLED=1 go test -race -count=1 -timeout=60s -v -run "$WRITE" ./examples/write || exit $?
capture scoped-vet go vet ./cmd/gosqlite ./examples/sparse-exact ./examples/readonly ./examples/write || exit $?
capture query-build go build -trimpath -o "$SESSION/bin/gosqlite" ./cmd/gosqlite || exit $?
capture sparse-build go build -trimpath -o "$SESSION/bin/sparse-exact" ./examples/sparse-exact || exit $?
capture readonly-build go build -trimpath -o "$SESSION/bin/readonly" ./examples/readonly || exit $?
capture write-build go build -trimpath -o "$SESSION/bin/write" ./examples/write || exit $?
capture modules-after go mod verify || exit $?
trap - EXIT
final_identity 0 || exit $?
```

模块校验是此手册额外的前瞻门，开发时的独立 18 项 scoped PASS 不包含它；完整 aggregate 若另有结果须按其精确源码单独引用。四个构建产物只用于构建证据，本段不执行这些二进制。独立 GOPATH/GOCACHE 与开发时共享缓存不同，已单独记录，不能伪称完全相同的机器环境。

## 4 应看到什么

- 每组命令 exit 0；普通/race 各自恰好运行 sparse 4、query 5、readonly 6、write 3 个顶层测试，没有 `SKIP`、`FAIL` 或 `no tests to run`。子测试数量单列，不充当额外顶层覆盖。
- readonly 的 URI 正常读没有动态尝试禁止写入或创建；该限制由显式 connector 参数及源码断言固定。write 的失败输出传播仅有 AST 断言，没有运行失败 Writer。
- readonly 三种页大小 × 四种输入模式输出既有前三行，并保持复制目录最终字节、大小、权限及清单；URI `rw/rwc` 不能覆盖显式 ReadOnly/NoCreate。
- sparse 的纯 helper 成功/参数错误/普通回调错误分别给出状态 0/2/1；AST 验证 main 接到该结果。未启动实际失败进程。
- write 的两次正常示例输出对应 rowid 1、2；后续正常回滚不改变这两行；支持域 Check 报告真实 page reads。成功文字只能在资源 helper 返回后输出。
- query 的内存输出错误保留原 sentinel；AST 验证 Rows.Close 错误写入实际命名结果。没有运行真实 Close 故障。
- before/after 源码身份完全相同。sourceguard、vet/build、模块校验通过不替代上述运行断言。

真实信号取消、实际终止状态的进程级验证、原生资源关闭/回滚故障、崩溃、namespace、恢复、错误密钥和篡改都不包含在这 18 项内。本轮没有对应运行证据；遵守主手册的 NOT_RUN/BLOCKED 区分。

## 5 失败排查、证据与清理

1. 核对提交包含全部具名测试，不要把旧 checkout 的零测试成功当通过。保存 source commit/tree、源码清单、版本和每个独占标签目录内的 command.json、stdout.txt、stderr.txt、result.json、exit.txt 文件以及最终身份/退出码文件。
2. 参数/输出断言失败时保留原日志和副本；不要降低断言、改 fixture、自动删除/重建数据库或重跑来覆盖原结果。
3. `go env GOVERSION` 不符、缺模块、race 无编译器、磁盘不足应分别记录环境阻塞；不要推断内核故障。sourceguard 拒绝则停止并审阅范围，不能删门控或加标签绕过。
4. 源码身份不一致时立即停止，记录新旧清单差异；测试成功不能抵消源码身份漂移。
5. 验收结束保留 `$EVIDENCE` 与失败副本。确认目录确实是本次 `mktemp` 输出、所有进程已正常退出且证据备份可读后，操作者可按自身保留策略删除这个单独隔离目录。不要使用宽泛删除、清理真实数据库 sidecar 或覆盖既有 Library 文件。

只有精确源码、18 项普通/race、无跳过、身份一致及原始证据齐全，才能写“这四项 CLI/示例普通限定复验通过”。这不是 SQLite 全兼容、物理持久性、其他原生 OS 或 M10 完整验收结论。
