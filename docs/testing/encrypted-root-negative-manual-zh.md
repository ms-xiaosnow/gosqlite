# 布局 1 加密根文件：六项有限手动反例操作手册

当前状态（2026-10-12）：独立源码复审、限定纯模型/AST 普通与 race 验证、源守卫、默认/手动源选择、compile-only 和静态符号检查已通过；六项真实磁盘场景，包括正确密钥对照，全部 **PHYSICAL_NOT_RUN**。精确范围、首轮失败及修正后的证据见[本次验证记录](../evidence/20261012-root-auth/README.zh.md)。这不是全仓库测试通过，也不把源码、模型或编译成功提升为物理认证、断电持久性或全部 A8 完成。

本文件提供以后在另获明确许可的用户自有隔离 Linux 环境中执行的步骤，不是运行许可。**本平台不得运行这些反例**，包括 gate-off、无匹配 selector、`-test.list` 或正确密钥对照；不能改名、切换进程或另建活动绕过既有拒绝。编辑本手册时未执行下列命令、fixture 或手动二进制。

## 原始目标和实际缺口

范围只对应原始目标 §§8.2、8.3、11.1 的加密认证拒绝与测试设施需求。既有 `encryption/record_test.go` 已有逐字节内存篡改、错误密钥及位置替换；`internal/encstore/review_test.go` 已有内存根认证和 manifest 规范性反例；`sqlite/encryption_test.go` 已有正确密钥磁盘生命周期和拷贝。不能把它们说成“没有加密测试”。本次只补公共 `OpenContext` 到实际根文件的有限手动调用器，没有新增生产接缝。

六项固定选择，每个新进程只选择一项：

| case | 候选根变化 | 必须观察的公共结果 |
| --- | --- | --- |
| control | 无，正确公开的一次性测试密钥 | 可用 DB；Verify、Check、完整三行字面量账本及 EOF/Err 成功 |
| wrong-key | 文件不变，一次性原始密钥首字节翻转一位 | 无 DB；保留认证原因；CodeNotADB |
| root-salt | 偏移 144 XOR 1，前后 RootSalt 都非零 | 无 DB；保留认证原因；CodeNotADB |
| metadata-ciphertext | 偏移 536 XOR 1 | 无 DB；保留认证原因；CodeNotADB |
| metadata-tag | 偏移 664 XOR 1 | 无 DB；保留认证原因；CodeNotADB |
| unknown-major | 偏移 9 XOR 2，使大端 major 1 成为 3 | 无 DB；保留加密与公共 unsupported 原因；CodeError |

字节几何来自 `encryption/format.go` 和 `internal/encstore/format.go`：512 字节头、24 字节记录头、128 字节布局 1 manifest、16 字节 tag，总根长度 680。调用器先核实健康头和记录几何，再复制和只改选定的一个字节；从不重编码受损根。

## 有限设施

- 标签严格为 `linux && manual_encrypted_root_negative`。默认源选择排除实际调用器；模型和源守卫不打开数据库。
- 入口第一条语句检查 `GOSQLITE_MANUAL_ENCRYPTED_ROOT_NEGATIVE=independently_authorized`。环境值只是额外门禁，不能替代独立授权。
- `GOSQLITE_MANUAL_ENCRYPTED_ROOT_CASE` 只接受上述六项。要求精确单入口 `test.run`、count=1、parallel=1、shuffle=off、timeout=0s、verbose=true，不接受其它测试标志。应用上下文另限 30 秒；无子进程或强制终止。
- `GOSQLITE_MANUAL_ENCRYPTED_ROOT_ROOT` 必须是调用者明确提供、非 root 用户拥有的 0700 直接绝对目录。命名空间声明必须是 `private-stable-no-alias-no-other-process`。所有原件、候选及诊断均保留在独立的新目录；不删除、不修复、不改权限、不操作挂载。
- `GOSQLITE_MANUAL_ENCRYPTED_ROOT_BINARY_SHA256` 必须匹配实际二进制哈希。独立审核/编译证据负责把该二进制绑定到已审源码；收据不伪称自身证明源到二进制对应关系。
- 原件由固定正确的一次性公开测试密钥生成：512 字节页、两页、第三次提交、三行固定 ID/正文。整个原件先通过 VerifyEncrypted、Check 和逐值账本，关闭后才复制。
- 健康族恰有七个文件：当前根、空锁、三个 base、两个 retained 根。单页 base 为 1256 字节，两份两页 base 为 1808 字节。副本使用 O_EXCL 新文件，独立 inode；保留同名成员。原件不写入，候选只在首次写入时含所选突变。
- 唯一候选 Open 始终使用生产 `ReadOnly:true, NoCreate:true`。每个反例要求 DB=nil，绝不尝试 Query、Verify、重开、另一密钥、恢复、重放、Rekey、Restore 或升级。
- 收据只存标量、文件名、哈希和公开身份。原始 error 对象另保留于进程内。错误使用实际公共 wrapper、Code 和 Is/As 原因分类；cleanup/native I/O/context 等混合错误拒绝作为成功。普通非 nil 错误不算认证拒绝。
- 生产受控 cleanup diagnostic 刻意支持 Is/As 而不公开 Unwrap；本设施遵守这一来源合同，没有杜撰“必须能展开到单个叶节点”的条件。
- 获取 DB、Rows、检查文件和复制文件后立即登记 owner。40 个文件 owner，每个最多关闭一次；Rows 在 EOF 自行退休，不再次 Close。未知退休不重试；存在未退休子 owner 或 pending cleanup 时不关闭父 DB。
- 对完整原件和候选分别留存前后成员、长度、哈希、设备/inode、UID/mode/link 数。Open 后原件和候选必须逐项相同。文件扫描包含固定明文标记和 SQLite 明文头；JSON 诊断输出再次扫描标记。日志不输出密钥、查询值或原始错误文本。

## 环境、依赖与授权前提

1. 下述可复制命令限原生 Linux/amd64、Bash、GNU coreutils/findutils（`sha256sum`、`stat`、`realpath`、`mktemp`、`sort -z`、`xargs -0`）、已审 Go **1.27.2**。调用器源码只要求 Linux；本文和现有编译证据没有替其它架构做资格确认。
2. `go.mod` 固定 `golang.org/x/crypto v0.58.0`、`golang.org/x/sys v0.49.0`、`golang.org/x/text v0.43.0`，以已审 `go.mod`/`go.sum` 的完整字节为准。预先提供对应离线模块缓存及可信工具链；不在本流程下载、升级依赖或修改 module 文件。不需要 SQLite oracle、PostgreSQL、C 编译器或真实业务数据库。手动二进制必须 `CGO_ENABLED=0`，不能用 race 二进制替代；已有 race 结果只是独立模型/AST 验证。
3. 操作者必须为非 root 用户；容器里的 UID 0 也不接受。不得使用 `sudo`、修改门禁、切换身份绕过失败。先由用户明确确认六项场景、源码版本、最终二进制 SHA-256、目标目录及保留方式；只获准编译时，停在编译后的审核点。
4. 源码必须是已审、无并发编辑的独立副本；证据、缓存和 fixture 位于源码之外。禁止接触用户数据库、凭据、真实密钥或旧运行目录。源码快照不得混入凭据或未审可执行内容。
5. 事先选定用户独占的 0700 直接绝对父目录：规范化后路径完全相同、各级路径无符号链接、无别名挂载、无其他进程访问。由用户核实命名空间稳定和磁盘空间充足；哈希及 `realpath` 检查不能证明没有 bind mount 或并发访问。不要为此改变挂载、系统安全设置或既有目录权限。
6. 每次启动只运行一项；使用前台自然退出，无并行批次、循环重试、外部 `timeout`、杀进程、崩溃注入或清理 trap。Go 测试 timeout 必须为 `0s`；调用器内部 30 秒 context 是操作上限，不是可强制终止进程的许可。

## 第一步：准备新的证据目录并固定源码

以下均为待人工执行的示例。先替换四个绝对路径；`SESSION_PARENT` 已按上述前提准备好，`GOMODCACHE` 已含固定版本。任一步失败就停，保留已经生成的文件，不修复或复用本次目录后继续。

```bash
set -euo pipefail
set -C
umask 077
SOURCE=/absolute/path/to/reviewed-gosqlite
GOROOT=/absolute/path/to/reviewed/go1.27.2
GOMODCACHE=/absolute/path/to/reviewed/module-cache
SESSION_PARENT=/absolute/user-owned/private-root-auth

test "$(id -u)" -gt 0
test "$(uname -s)" = Linux
test "$(uname -m)" = x86_64
for p in "$SOURCE" "$GOROOT" "$GOMODCACHE" "$SESSION_PARENT"; do
  test -d "$p"
  test "$(realpath -e -- "$p")" = "$p"
done
test "$(stat -c '%u' -- "$SESSION_PARENT")" = "$(id -u)"
test "$(stat -c '%a' -- "$SESSION_PARENT")" = 700
case "$SESSION_PARENT/" in "$SOURCE/"*) exit 1 ;; esac

SESSION=$(mktemp -d "$SESSION_PARENT/root-auth-evidence-XXXXXXXX")
EVIDENCE="$SESSION/evidence"
FIXTURES="$SESSION/fixtures"
mkdir -m 700 -- "$EVIDENCE" "$FIXTURES" "$SESSION/build-cache" \
  "$SESSION/gopath" "$SESSION/tmp"
mkdir -m 700 -- "$EVIDENCE/cases"
cd -- "$SOURCE"
test -z "$(find . -path './.git' -prune -o -type l -print -quit)"

# 这里定义的 shell 函数只是完整源码文件清单工具，不是项目测试入口。
source_manifest() {
  find . -path './.git' -prune -o -type f -print0 |
    LC_ALL=C sort -z | xargs -0 -r sha256sum
}
source_manifest > "$EVIDENCE/source.before.sha256"
sha256sum "$EVIDENCE/source.before.sha256" > "$EVIDENCE/source-manifest.sha256"
printf '%s\n' "$SOURCE" > "$EVIDENCE/source-path.txt"
printf '%s\n' "$FIXTURES" > "$EVIDENCE/fixture-root.txt"
stat -c 'uid=%u mode=%a device=%d inode=%i' -- "$SESSION" "$FIXTURES" \
  > "$EVIDENCE/directory-identities.txt"
```

清单覆盖当前源码副本全部普通文件（仅排除版本库管理目录 `.git`），包含未提交的新增调用器/守卫；不是只记录 Git commit。应另保留收到的只读源码归档及其已审 SHA-256。清单本身的 SHA-256、归档身份和独立审核记录应存于不会随本地文件被改写的审核记录中；本地哈希不能自证可信，也不是防篡改存储。

## 第二步：只编译，不启动测试二进制

```bash
GO_BIN="$GOROOT/bin/go"
BUILD_ENV=(env -i
  "PATH=$GOROOT/bin:/usr/bin:/bin" "GOROOT=$GOROOT"
  "GOMODCACHE=$GOMODCACHE" "GOPATH=$SESSION/gopath"
  "GOCACHE=$SESSION/build-cache" "TMPDIR=$SESSION/tmp"
  GOENV=off GO111MODULE=on GOWORK=off GOTOOLCHAIN=local
  GOPROXY=off GOSUMDB=off GOAUTH=off GOTELEMETRY=off
  GOFLAGS= GOEXPERIMENT= GOFIPS140=off GODEBUG= GOCACHEPROG=
  GOCOVERDIR= GORACE= GOOS=linux GOARCH=amd64 GOAMD64=v1
  CGO_ENABLED=0 GOMAXPROCS=2 LC_ALL=C LANG=C TZ=UTC)
test "$("${BUILD_ENV[@]}" "$GO_BIN" env GOVERSION)" = go1.27.2
test "$("${BUILD_ENV[@]}" "$GO_BIN" env GOHOSTOS)" = linux
test "$("${BUILD_ENV[@]}" "$GO_BIN" env GOHOSTARCH)" = amd64
"${BUILD_ENV[@]}" "$GO_BIN" env GOVERSION GOHOSTOS GOHOSTARCH GOOS GOARCH \
  CGO_ENABLED GOFLAGS GOTOOLCHAIN GOMODCACHE GOCACHE \
  > "$EVIDENCE/build-environment.txt"

BINARY="$EVIDENCE/sqlite-root-negative-manual.test"
test ! -e "$BINARY"
BUILD_CMD=("${BUILD_ENV[@]}" "$GO_BIN" test -mod=readonly -buildvcs=false \
  -p=1 -vet=off -c -tags=manual_encrypted_root_negative \
  -o "$BINARY" ./sqlite)
printf '%q ' "${BUILD_CMD[@]}" > "$EVIDENCE/build-command.txt"
printf '\n' >> "$EVIDENCE/build-command.txt"
build_rc=0
"${BUILD_CMD[@]}" > "$EVIDENCE/build.stdout" 2> "$EVIDENCE/build.stderr" || build_rc=$?
printf '%s\n' "$build_rc" > "$EVIDENCE/build.exit"
test "$build_rc" -eq 0

# 两条 Go 工具命令只读取产物；绝不调用 "$BINARY" -test.list。
"${BUILD_ENV[@]}" "$GO_BIN" version -m "$BINARY" > "$EVIDENCE/binary-build-info.txt"
"${BUILD_ENV[@]}" "$GO_BIN" tool nm "$BINARY" > "$EVIDENCE/binary-nm.txt"
sha256sum "$BINARY" > "$EVIDENCE/binary.sha256"
source_manifest > "$EVIDENCE/source.after-build.sha256"
cmp -- "$EVIDENCE/source.before.sha256" "$EVIDENCE/source.after-build.sha256"
cat "$EVIDENCE/source-manifest.sha256" "$EVIDENCE/binary.sha256"
```

**审核停点：**到这里仍是 compile-only。让独立审核者核对源码清单/归档、构建环境与命令、build info 和符号范围，并把最终产物 SHA-256 绑定到已审源码。build info 必须含 `CGO_ENABLED=0`、`GOOS=linux`、唯一显式 tag `manual_encrypted_root_negative`；不能加 `oracle` 或其它手动标签。`go tool nm` 中应有精确入口 `github.com/ms-xiaosnow/gosqlite/sqlite_test.TestManualEncryptedRootNegative`，不能以存在此符号代替整份审查。

现有本次证据中未执行产物的 SHA-256 为 `0539172b7a0e0b205a031446446944c5de26ac1284293576013d7db5f45ee9c8`；它只标识那一次构建。换路径或重建可能得到不同字节，不得把此值硬套到新二进制，也不得用“刚算出来的值”替代独立审批。

## 第三步：独立获准后，每次手动启动一项

只有在受允许的用户环境完成上述审核并得到物理执行许可后，才填写以下两个已批准的 64 位小写 SHA-256。保持同一 Bash 会话；一旦会话、源码、二进制或授权范围变化，先重新核对证据，不重新生成旧日志或复用旧 fixture。

```bash
readonly APPROVED_SOURCE_MANIFEST_SHA256='REPLACE_WITH_INDEPENDENTLY_APPROVED_SOURCE_MANIFEST_SHA256'
readonly APPROVED_BINARY_SHA256='REPLACE_WITH_INDEPENDENTLY_APPROVED_BINARY_SHA256'
[[ "$APPROVED_SOURCE_MANIFEST_SHA256" =~ ^[0-9a-f]{64}$ ]]
[[ "$APPROVED_BINARY_SHA256" =~ ^[0-9a-f]{64}$ ]]
test "$(sha256sum "$EVIDENCE/source.before.sha256" | cut -d ' ' -f 1)" = \
  "$APPROVED_SOURCE_MANIFEST_SHA256"
test "$(sha256sum "$BINARY" | cut -d ' ' -f 1)" = "$APPROVED_BINARY_SHA256"

# 本文完整定义的 shell 包装器；项目本身没有 run_root_case 命令。
# 包装器保存证据并启动一次已批准二进制；fixture 仅由该二进制操作。
run_root_case() {
  local selected="$1" case_evidence rc=0
  case "$selected" in
    control|wrong-key|root-salt|metadata-ciphertext|metadata-tag|unknown-major) ;;
    *) return 1 ;;
  esac
  test "$(id -u)" -gt 0 || return 1
  case_evidence="$EVIDENCE/cases/$selected"
  mkdir -m 700 -- "$case_evidence" || return 1
  source_manifest > "$case_evidence/source.before.sha256" || return 1
  cmp -- "$EVIDENCE/source.before.sha256" "$case_evidence/source.before.sha256" || return 1
  sha256sum "$BINARY" > "$case_evidence/binary.before.sha256" || return 1
  test "$(cut -d ' ' -f 1 "$case_evidence/binary.before.sha256")" = \
    "$APPROVED_BINARY_SHA256" || return 1
  local -a command=(env -i PATH=/usr/bin:/bin GOFLAGS= GOMAXPROCS=2 TZ=UTC
    GOSQLITE_MANUAL_ENCRYPTED_ROOT_NEGATIVE=independently_authorized
    "GOSQLITE_MANUAL_ENCRYPTED_ROOT_CASE=$selected"
    "GOSQLITE_MANUAL_ENCRYPTED_ROOT_ROOT=$FIXTURES"
    GOSQLITE_MANUAL_ENCRYPTED_ROOT_NAMESPACE=private-stable-no-alias-no-other-process
    "GOSQLITE_MANUAL_ENCRYPTED_ROOT_BINARY_SHA256=$APPROVED_BINARY_SHA256"
    "$BINARY"
    '-test.run=^TestManualEncryptedRootNegative$'
    -test.count=1 -test.parallel=1 -test.shuffle=off -test.timeout=0s -test.v=true)
  printf '%q ' "${command[@]}" > "$case_evidence/command.txt" || return 1
  printf '\n' >> "$case_evidence/command.txt" || return 1
  date -u +%FT%TZ > "$case_evidence/start.utc" || return 1
  "${command[@]}" > "$case_evidence/stdout.log" 2> "$case_evidence/stderr.log" || rc=$?
  printf '%s\n' "$rc" > "$case_evidence/exit-code.txt" || return 1
  date -u +%FT%TZ > "$case_evidence/end.utc" || return 1
  sha256sum "$BINARY" > "$case_evidence/binary.after.sha256" || return 1
  source_manifest > "$case_evidence/source.after.sha256" || return 1
  sha256sum "$case_evidence/stdout.log" "$case_evidence/stderr.log" \
    "$case_evidence/exit-code.txt" "$case_evidence/command.txt" \
    > "$case_evidence/logs.sha256" || return 1
  cmp -- "$case_evidence/binary.before.sha256" "$case_evidence/binary.after.sha256" || return 1
  cmp -- "$case_evidence/source.before.sha256" "$case_evidence/source.after.sha256" || return 1
  printf 'case=%s exit=%s evidence=%s\n' "$selected" "$rc" "$case_evidence"
  return "$rc"
}
```

以下六条**逐条**执行；每条返回后先核对下一节的日志/收据，再决定是否继续。不要合成循环或并发任务；任一失败或结果未知都停下，未启动项目维持 NOT_RUN。

正确密钥对照：

```bash
run_root_case control || exit $?
```

文件不变、错误测试密钥：

```bash
run_root_case wrong-key || exit $?
```

非零 RootSalt 单字节变化：

```bash
run_root_case root-salt || exit $?
```

metadata 密文单字节变化：

```bash
run_root_case metadata-ciphertext || exit $?
```

metadata tag 单字节变化：

```bash
run_root_case metadata-tag || exit $?
```

未知 container major：

```bash
run_root_case unknown-major || exit $?
```

运行二进制只接受上面的六个显式测试标志。不要加 `-test.list`、`-test.short`、`-test.failfast`、profile/coverage、bench/fuzz 标志或位置参数；不要改成 `go test -tags ...` 自动构建并启动。五个 `GOSQLITE_MANUAL_ENCRYPTED_ROOT_*` 环境变量须全部准确提供，`GOFLAGS` 必须为空。门禁字符串是声明，不能创造或扩展授权。

## 预期输出与每项通过条件

以下是输出结构示意，不是实测日志；`{...}`、路径和时间都只是占位。原始 stdout/stderr、退出码和 SHA-256 保留在各项证据目录。

```text
=== RUN   TestManualEncryptedRootNegative
    ... ROOT_AUTH_RETAINED_DIRECTORY /.../fixtures/root-negative-...
    ... ROOT_AUTH_RECEIPT {"Profile":"layout1-root-negative-v1","Case":"control","Status":"observed",...}
    ... ROOT_AUTH_OBSERVED one_case=true originals_retained=true
--- PASS: TestManualEncryptedRootNegative (...s)
PASS
```

每项必须自然退出为 0，仅出现一个该测试、一个 `ROOT_AUTH_RECEIPT` 和一个 `ROOT_AUTH_OBSERVED`；没有 FAIL、SKIP、panic、timeout、异常 stderr 或诊断拒绝。单独看到 `PASS`、非 nil error、环境门禁通过或创建了目录都不够。JSON 字段名按源码的导出字段拼写（首字母大写），无另设的结果文件或隐藏辅助入口。

| Case | ChangedBytes / ChangedOffset / ChangedXOR | Class / Code | DBReturned / QueryCalls | CandidateAcquires / CandidateReleases |
| --- | --- | --- | --- | --- |
| control | 0 / -1 / 0 | ok / 0 | true / 1 | 1 / 1 |
| wrong-key | 0 / -1 / 0 | authentication / 26 | false / 0 | 1 / 1 |
| root-salt | 1 / 144 / 1 | authentication / 26 | false / 0 | 1 / 1 |
| metadata-ciphertext | 1 / 536 / 1 | authentication / 26 | false / 0 | 1 / 1 |
| metadata-tag | 1 / 664 / 1 | authentication / 26 | false / 0 | 1 / 1 |
| unknown-major | 1 / 9 / 2 | unsupported / 1 | false / 0 | 0 / 0 |

还须全部满足：

- `Profile=layout1-root-negative-v1`、`Status=observed`、`Case` 正确、`UID>0`；`BinarySHA256=ExpectedBinarySHA256=已批准二进制哈希`。源码和二进制的运行前后清单一致。
- `CausePreserved=true`、`OpenCalls=1`；这里 OpenCalls 只统计唯一候选 Open，健康 seed 的构造另计。认证反例是公共 `*sqlite.Error` 的 `Op=open`、`CodeNotADB=26` 和保留的 `encryption.ErrAuthentication`；未知 major 是 `CodeError=1` 且保留加密层及公共层两个 unsupported 原因。混入 I/O、context、cleanup 或其它非目标错误必须失败。
- 所有项 `SeedAcquires=SeedReleases=1`；未知 major 在取得候选密钥之前失败。`DuplicateRelease`、`PendingCleanup`、`PlaintextMarker` 均为 false；`SaltBeforeNonzero`、`SaltAfterNonzero` 均为 true。
- `Seed` 的 Verified、Checked、ExactRows、EOF、RowsErrNil 全为 true；Layout=1、PageSize=512、Pages=2、Sequence=3、Rows=3。control 的 `Control` 同样完整；五项反例 `Control` 为零值且不执行候选 Query/Verify。字面量账本在内存逐值比较，日志不应打印行正文。
- `OriginalBefore=OriginalAfter`、`CandidateBefore=CandidateAfter`，每份均恰好七个成员；同名长度一致，原件与候选 inode 不互为别名。无突变时两份哈希全同，有突变时仅当前 `fixture.gse` 哈希不同。四个目录身份保持稳定；目录 0700、文件 0600、UID 正确、单硬链接。
- `Owners` 有两个 DB 登记、40 个 file 登记及 `1+QueryCalls` 个 Rows 登记：control 共 44 个，反例共 43 个。已存在 owner 恰好一次退休，DB/file 为 `closed`、Rows 为 `eof`；反例不存在的候选 DB 为 `absent`、Attempts=0。没有 live、unknown、blocked 或重复关闭。

只有六项均有各自完整证据时，才能报告“这六项布局 1 根文件公共接口观察通过”。任何一项失败、未知或 NOT_RUN，均不得宣称六项全通过；这一结论仍不等于全部 A8、断电安全、生产可用性或持续模糊测试通过。

## 故障处理、保留与清理

- `manual encrypted root negative is not independently authorized`：缺少声明或尚未独立获准；停下，不把设置环境变量当作解决授权问题的办法。
- `nonprivileged linux profile`、`exact test flags`、`only reviewed flags`、`exact manual build profile`、`approved binary hash`：环境、参数或产物不符；保留日志，先核对批准版本。不要通过改源码、放宽校验、重新填写当前哈希来让它通过。
- `known fixture root`、`root has no symlink components`、`private owned directory/file`、`fresh directory only` 或 `fresh copy owner`：目录/所有权/隔离前提失败；不得 chmod/chown、删除现有文件、改链接或改挂载后原地重试。
- `directory/file ...`、read/write/Sync/Close、context、`exact fail-closed public error`、`finite receipt refused`：不是成功的认证反例。保存原始退出状态、已有日志和全部原件/候选，未完成的测试标记失败或未知。不得查看真实密钥、额外输出原始 error 字符串、对候选执行 Query/Verify、换密钥重开、repair/replay/recovery/Rekey/Restore 或升级。
- `owner retirement incomplete; no retry`、unknown/blocked owner 或 pending cleanup：不重试 Close/Release，不调用清理重试 API，不在所有者未确定退休时关闭父 DB、继续下一项或复用目录。程序正常返回不证明未知 owner 已按合同退休。若进程未返回，不追加启动或终止操作；先报告状态并由用户另行决定。
- 每项均保留 `ROOT_AUTH_RETAINED_DIRECTORY` 指向的整个运行目录、原件及候选，不搬动/覆盖它们来补验收。错误对象只能在原进程内保留；退出后的标量日志不是可重新接管 error owner 的令牌。目录可能包含公开测试密钥生成的密文，但依然按私有测试证据处理。
- 本手册没有自动删除命令。**只有用户在查看结果后明确确认具体目录、范围、保留副本及删除时机，且不存在运行中/未知 owner，才可另行清理用户确认的目录。** 运行许可不包括删除许可；失败、超时或空间不足也不是清理许可。保留源码归档、批准的 SHA-256、构建日志、每项命令/退出码/收据及其哈希，避免仅保留成功摘要。

## 审核与状态分栏：历史及当前

下面两段保留首次 source-only 交接时的原始状态文字，**仅为历史**，不是当前状态或执行许可：

当前 overlay 只含本模型、测试、手动调用器、目标源守卫、最小 main hook 和本文。新完整源码 pins 保持 HOLD；既有 main pins 保持以前已审字节值，待整份 main 独立审核后才可同时更新。守卫有限反例在内存源字符串中检查门禁、build tag、readonly/no-create、即时 owner/result 捕获、原件保护、错误分类、账本、重开及 rekey 越界。它不尝试证明任意 Go 程序安全。

本次状态必须独立记录：源码已编写；独立复审待完成；纯模型/AST 测试未运行；有限默认/手动源选择未运行；compile-only 未运行；六项物理操作全部 NOT_RUN。任何之后的非受限测试只能按父任务单独批准范围执行；不得执行编译出的手动二进制。

当前替代上述历史状态的结论：独立复审及固定 pins 已完成；限定纯模型和相关 AST/接线普通及 race 检查 PASS，源守卫 PASS，默认选择不包含/手动选择包含调用器，compile-only 和 `go tool nm` PASS；已构建二进制从未执行。普通模型结果保留原有通过证据，修正后只重做受影响的限定检查，未把未重跑项目写成重跑。首轮守卫接线失败、修正及最终记录均须保留，见[完整证据索引](../evidence/20261012-root-auth/README.zh.md)。本手册的可复制命令没有实际运行结果；六项物理操作仍全部 NOT_RUN。

未覆盖：页/base/segment 替换、DBID/parent 替换、布局 2/3、密码盐、重启/失败发布/rekey 的 nonce 证据，以及任何崩溃、断电或恢复吞吐证明。它们只是原目标中仍未证明的范围，不是本次新增验收义务。ANN V4、41-target 连续 fuzz 和已拒绝/来源不明的中断活动继续受阻。
