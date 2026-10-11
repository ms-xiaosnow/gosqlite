# 五个示例的目录终结限定复验

本文补充 encrypted、encrypted-wal、encrypted-shared、encrypted-cli、native-check 五个示例的普通返回路径：只有执行完整且最终没有错误时才清理私有临时目录；否则返回带目录位置的诊断。范围对应 G1 和 §12.1 的资源及错误行为。本规程用于未来独立复验，编写期间仅作静态核对，以下命令均未因本文而执行，状态为 NOT_RUN。

已有[独立证据](evidence/example-retention-20261011/independent-review.md)是固定候选的源码审阅、五个顶层测试的普通/race 与限定 vet/build PASS。它没有执行真实失败后的目录保留验证，也没有执行 snapshot、encrypted-cli 或 native-check 的 main/run。本补充不扩大该结论。整体状态见[验收矩阵](acceptance-status.md)，执行边界见[中文主手册](manual-verification-zh.md)。

## 1 已知范围和保留限制

- Finalize 的两个回调测试不创建文件、数据库句柄或凭据。它们只核对删除回调次数、完成标记、延迟错误和 errors.Is 错误链。
- 五文件 AST 测试只读取源码，检查终结器位置、延迟求值、具名结果、已有 Close 错误合并及输出错误检查，不运行这些文件中的 main。
- WAL/shared 的原有两个测试运行完整文件、正确密钥和正常完成流程。shared 中原有 stale-snapshot BUSY 是普通并发契约的预期返回，不是存储故障注入；整个 run 正常成功返回。
- WAL 原测试将 TMPDIR 指向 t.TempDir，测试框架结束时会自行清理。它可以断言正常完成后示例目录已消失，不能证明真实错误返回后目录仍留在磁盘。不要改原测试、制造失败或把 t.TempDir 自动清理后的结果当作保留证明。
- 五个示例仍会清除进程内自有凭据。即使保留了剩余密文，也可能已无法解密；目录保留不是密钥托管、完整可恢复备份或历史密钥撤销。
- RemoveAll 出错可能已删除部分文件。诊断仅承诺不再重试删除，不承诺完整现场；普通删除和 clear 都不是安全擦除。
- 该策略不是 panic、进程退出、崩溃、断电或任意故障保证。没有新增 keep-dir 开关，也没有通过凭据日志来帮助恢复。

## 2 选择源码与准备环境

先在已经授权取得的本地完整仓库中选择包含全部八个留存源码文件的干净提交，再在该 checkout 根目录开启专用 Bash。下面直接读取已选择的 HEAD 并保存完整 commit/tree；不切换 checkout、不修改原仓库、不接触远端。旧 776b23e、1352ae5 及四项 CLI 的早期源码都不能自动当作包含新 helper 的版本。

静态依据是五例留存冻结候选和组合 tree `d7393c254a6fca4452ae194d64d9045659f9781d`。该 tree 是候选身份，不是可凭空填写的已发布 commit，也不代表最终文档加入后的完整 tree。操作者必须另行确认所选提交确实包含已审阅的相同运行源码；存在文件只能阻止明显选错版本，不能替代源码身份审阅。

依赖固定为 Go 1.27.2、golang.org/x/crypto v0.58.0、golang.org/x/sys v0.49.0、golang.org/x/text v0.43.0，以所选 go.mod/go.sum 为准。另需 Git、Bash、tar、Python 3、cmp、SHA-256 工具；race 需该 Go 版本支持的本机 C 工具链。只适用于 Linux/amd64 主机与目标，生产构建 CGO_ENABLED=0；不需要 SQLite oracle 或 PostgreSQL 服务。

缓存须事先合法取得并校验。离线缺依赖、工具链不符或资源不足时停止；本规程不安装软件、不自动下载、不更换环境重试。先按主手册第 4 节只读核对实际 /tmp 和工作盘的容量、inode 与挂载条件。普通成功路径不能在 tmpfs 或 fsync=volatile 上证明断电持久性。

```bash
set -eu
umask 077
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOFLAGS=-buildvcs=false
unset TMPDIR GOTMPDIR TMP TEMP
SOURCE_REPO=$(pwd)
MODULE_CACHE=$(go env GOMODCACHE)
test -d "$MODULE_CACHE"
SOURCE_COMMIT=$(git rev-parse --verify 'HEAD^{commit}')
SOURCE_STATUS=$(git status --porcelain)
test -z "$SOURCE_STATUS"
SESSION=$(mktemp -d /tmp/gosqlite-example-retention-review.XXXXXX)
mkdir "$SESSION/source" "$SESSION/evidence" "$SESSION/gopath" "$SESSION/gocache"
git archive --output="$SESSION/source.tar" "$SOURCE_COMMIT"
tar -x -f "$SESSION/source.tar" -C "$SESSION/source"
EVIDENCE="$SESSION/evidence"
printf '%s\n' "$SOURCE_COMMIT" > "$EVIDENCE/source-commit.txt"
git rev-parse "$SOURCE_COMMIT^{tree}" > "$EVIDENCE/source-tree.txt"
sha256sum "$SESSION/source.tar" > "$EVIDENCE/source-archive.sha256"
cd "$SESSION/source"
export GOMAXPROCS=2 CGO_ENABLED=0
export GOPATH="$SESSION/gopath" GOCACHE="$SESSION/gocache" GOMODCACHE="$MODULE_CACHE"
test -w "$GOPATH" && test -w "$GOCACHE"
go version > "$EVIDENCE/go-version.txt"
go env GOVERSION GOHOSTOS GOHOSTARCH GOOS GOARCH CGO_ENABLED GOTOOLCHAIN GOPROXY GOSUMDB GOPATH GOCACHE GOMODCACHE GOFLAGS > "$EVIDENCE/go-env.txt"
test "$(go env GOVERSION)" = go1.27.2
test "$(go env GOHOSTOS)/$(go env GOHOSTARCH)" = linux/amd64
test "$(go env GOOS)/$(go env GOARCH)" = linux/amd64
for file in \
  examples/internal/fixturecleanup/cleanup.go \
  examples/internal/fixturecleanup/cleanup_test.go \
  examples/internal/fixturecleanup/source_test.go \
  examples/encrypted/main.go examples/encrypted-wal/main.go \
  examples/encrypted-shared/main.go examples/encrypted-cli/main.go \
  examples/native-check/main.go; do
  test -f "$file"
done
sha256sum go.mod go.sum goal.md > "$EVIDENCE/contract-inputs.sha256"
printf '%s\n' "$SESSION"
```

此后只在 `$SESSION/source` 中运行已选步骤；证据和缓存在源码外。独立 GOPATH/GOCACHE 是本规程的前瞻隔离条件，不与开发时共享缓存条件混称完全相同环境。不得整份记录环境变量或凭据。

## 3 保留原始日志与退出状态

沿用已独立静态审阅的[CLI 资源规程](cli-resource-verification-zh.md)记录器。每个标签独占目录，保存完整 argv、cwd、开始/结束时间、原始 stdout/stderr 与原始返回码；不使用丢失命令状态的管道，不覆盖、不自动重试。初始化日志失败时不启动命令；命令已结束而结果文件写入失败时，保留不完整证据并停止，不能推测通过。

```bash
cat > "$EVIDENCE/capture.py" <<'PY'
from pathlib import Path
import datetime, json, os, subprocess, sys
base, label, *argv = sys.argv[1:]
if not label or label in ('.', '..') or '/' in label or not argv:
    raise SystemExit('Invalid evidence label or empty command')
entry = Path(base) / label
entry.mkdir()
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

完整身份核对包含相对路径、文件/目录类型、文件字节、大小、原始权限与 Git 可执行位。它不运行数据库；源码外的记录脚本和日志不进入源码清单。无论选定命令成功或失败，EXIT 处理均尝试写最终身份；原命令失败不会被身份成功改为通过。

```bash
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

## 4 精确五项普通与 race 选择

下列恰好五个真实顶层测试：

1. `TestFinalizeCallbackPolicy`：纯回调策略，六个子场景。
2. `TestFinalizeDelayedResult`：纯回调，延迟产生错误时不调用删除。
3. `TestExampleFixtureFinalizersSource`：五文件 AST 契约。
4. `TestEncryptedWALExample`：正确密钥、普通成功、正常重开/checkpoint、正常清理。
5. `TestEncryptedSharedExampleOrdinary`：原有正常完成的 shared 事务/快照流程。

前三个在 `examples/internal/fixturecleanup`，后两个分别在 `examples/encrypted-wal` 和 `examples/encrypted-shared`。单独 sourceguard 是同一 AST 测试，不计作第六个测试。普通和 race 的 selector 与独立审阅证据逐字相同；本规程显式 `-timeout=0`，不安排超时终止场景。开发时命令未显式设置 timeout，二者不伪称完全相同 argv。

```bash
SELECTED='^(TestFinalizeCallbackPolicy|TestFinalizeDelayedResult|TestExampleFixtureFinalizersSource|TestEncryptedWALExample|TestEncryptedSharedExampleOrdinary)$'
PACKAGES=(./examples/internal/fixturecleanup ./examples/encrypted-wal ./examples/encrypted-shared)
BUILD_PACKAGES=(./examples/internal/fixturecleanup ./examples/encrypted ./examples/encrypted-wal ./examples/encrypted-shared ./examples/encrypted-cli ./examples/native-check)
capture modules-before go mod verify || exit $?
capture sourceguard-cgo0 go test -count=1 -timeout=0 -v -run '^TestExampleFixtureFinalizersSource$' ./examples/internal/fixturecleanup || exit $?
capture selected-cgo0 go test -count=1 -timeout=0 -v -run "$SELECTED" "${PACKAGES[@]}" || exit $?
capture identity-before-race python3 "$EVIDENCE/source-identity.py" "$EVIDENCE/source-before-race.json" || exit $?
capture identity-compare-before-race cmp "$EVIDENCE/source-before.json" "$EVIDENCE/source-before-race.json" || exit $?
capture sourceguard-cgo1-race env CGO_ENABLED=1 go test -race -count=1 -timeout=0 -v -run '^TestExampleFixtureFinalizersSource$' ./examples/internal/fixturecleanup || exit $?
capture selected-cgo1-race env CGO_ENABLED=1 go test -race -count=1 -timeout=0 -v -run "$SELECTED" "${PACKAGES[@]}" || exit $?
capture scoped-vet-cgo0 go vet "${BUILD_PACKAGES[@]}" || exit $?
capture scoped-build-cgo0 go build "${BUILD_PACKAGES[@]}" || exit $?
capture modules-after go mod verify || exit $?
trap - EXIT
final_identity 0 || exit $?
```

多包 `go build` 只做编译检查，不在源码目录留下示例可执行文件；本段不执行构建产物。snapshot、encrypted-cli、native-check 在这里仅由 AST 与 build/vet 覆盖，不调用它们的 main/run，也不启动其 CLI 数据库子进程。模块校验和独立缓存是本规程附加前瞻条件，不能补记为原五项 scoped PASS 已覆盖的新门。

## 5 应核对的断言

- 两次 selected 命令各有且只有五个顶层 `=== RUN` / `--- PASS`，不能出现 SKIP、FAIL 或 `no tests to run`；AST 单独两次各只有一个顶层测试。子测试另列，不扩大覆盖数。退出 0 只是必要条件。
- helper 成功且 completed=true 时删除回调正好一次；未完成或原结果有错误时零次；原 operation/close sentinel 均保持 errors.Is；删除错误保留 remove sentinel，只调用一次并提示可能部分删除。
- 延迟错误测试证明 Finalize 执行时读取最终结果；没有真实 Close 错误注入。AST 核对成功 MkdirTemp 后注册、晚于其他 defer 执行、最终成功才标记 completed、输出错误先返回以及既有 Close 聚合未变。
- WAL 的 run 断言 layout 2、三次提交产生三个选定 segment、正常重开 metadata 保持，checkpoint 后选定 segment/frame/bytes 为零、正确只读重开数据保持。原测试逐字比较四行输出并检查其 t.TempDir 内没有剩余示例目录。
- shared 的 run 检查读者快照、普通提交、预期 stale-snapshot BUSY 后主动回滚与新值、checkpoint 后 layout 3 和零选定 segment；原测试检查四个既有输出短语。它没有直接检查失败保留目录，也没有直接给出正常删除目录的独立文件断言。
- 全部命令状态、原始日志、前后源码清单齐全，身份比较退出 0。只读身份、build/vet、模块校验及 AST 通过不能替代运行断言。

只有这些要求都满足，才可写“五例目录终结的纯 helper/AST 与两项原有正常流程限定复验通过”。不得写“真实失败现场保留已实测”“五个示例所有运行分支通过”或“M10 完整验收通过”。

## 6 失败排查与清理

1. 缺具名测试或零匹配时，保留提交身份与输出，先核对提交/package；不能给旧版本加新测试名后把空运行记为成功。
2. 工具链、离线依赖、C 工具链、磁盘/inode 缺口分别记 BLOCKED；测试断言确实失败记 FAIL。不要把基础设施错误推断为内核问题，也不要删 guard、换标签或扩大选择器绕过。
3. 首个非零返回后停止；保留 command.json、stdout.txt、stderr.txt、result.json、exit.txt、时间与源码清单。不自动修改断言、重建数据、重跑、删除 sidecar 或试图诱发第二种失败。
4. 源码身份漂移或证据落盘不完整时停止并保留所有原始文件；不能因部分测试 PASS 把完整门记为 PASS。耗时异常先只读核对状态与日志，不新增强制终止指令。
5. 如原有 WAL 测试意外失败，报告框架可能已经清理 t.TempDir 的事实；不能承诺现场还在。其他诊断若给出剩余目录，也不能假设文件齐全或密钥可用。不要打印、索取或保存示例密钥。
6. 成功后也保留 `$EVIDENCE`、源码身份与 source.tar。操作者确认所有进程自然退出、确为本次 mktemp 目录、证据已备份且备份可读后，再按自己的保留策略清理该独立目录；本规程不提供宽泛删除或自动失败清理命令。

## 7 仍然不能运行或不能假装已有的项目

- 已有有限 manual rollback recovery 与两个 manual_namespace 回归的真实入口见[主手册第 12 节](manual-verification-zh.md#12-受限人工门禁与不能编造的入口)。本补充不复制或启动这些命令，当前仍为 BLOCKED / NOT_RUN。
- 真实 crash、power-loss、process-exit、short-write/Sync/ENOSPC/torn-write、错误密钥、磁盘篡改、namespace 干预和 checkpoint/rekey 中断均不在本选择内；现有受限场景仍 NOT_RUN。完整设施缺失的项按[主手册第 12.4 节](manual-verification-zh.md#124-没有完整实现入口的门禁)标 IMPLEMENTATION_MISSING，先要实际代码和独立审阅，不能只交一条虚构测试命令。
- ANN V4、41 目标 fuzz 及未知中断的原操作/目标仍不明，保持 UNKNOWN/BLOCKED/HOLD，见[主手册第 13 节](manual-verification-zh.md#13-fuzz-与中断原因的人工审查前置)。ANN V3 的三个源码/报告准入 FAIL 和其他原始 FAIL 保留，不借此补充解除旧链。
- 真实原生 OS/文件系统/硬件、长期 soak、大规模测量、授权 PG/Neon 和独立数据安全/密码学/许可/API 审查的外部前置仍见[验收矩阵](acceptance-status.md)。本补充不增加 runner、计费读取或权限。

在现有限制下完成了独立可推进项，整体未验收。
