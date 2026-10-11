# LIMIT 转换修正的限定复验

这是普通SQL兼容性复验，不运行恢复、故障、namespace、错密钥、磁盘篡改或中断链路。它不覆盖其他planner阶段问题。已有平台受限测试仍按[主手册](manual-verification-zh.md)保留BLOCKED/NOT_RUN。

## 1 环境与隔离

- Linux/amd64、Bash、Git、Python3、gzip、Go **1.27.2**；race另需受支持C工具链。生产普通门使用CGO_ENABLED=0。
- 官方SQLite **3.54.0**，source ID `2026-10-09 15:46:58 be8d059e9a49089ab2dce5ed26dd87aaf598fdc5fbe0b107c0dd758464bcd5e3`。它仅为测试对照，不进入生产调用。
- 先按主手册环境章节取得固定工具和依赖缓存。仓库现有 `scripts/install-baselines.sh /absolute/empty/toolchains-dir` 可在Linux/amd64安装并校验官方源码；`scripts/build-oracle.sh /absolute/new/oracle-dir` 可单独构建固定CLI。脚本需要curl、unzip、make、cc；不要使用未知来源可执行文件或跳过哈希。
- 使用新clone和独立日志目录，不在生产数据库旁运行。测试自身只创建三行完好fixture及官方CLI内存库；不需要用户数据库、密钥或外部SQL服务。
- 将 `GOSQLITE_GO_BIN` 设为已安装Go的bin绝对路径，将 `SQLITE_ORACLE` 设为已校验CLI绝对路径。保留默认/tmp，不把该普通复验当作持久性或断电证明。

```bash
set -euo pipefail
: "${GOSQLITE_GO_BIN:?先设置固定Go的bin绝对路径}"
: "${SQLITE_ORACLE:?先设置固定SQLite CLI绝对路径}"
test "${TMPDIR:-/tmp}" = /tmp
test -z "${GOTMPDIR:-}"
CASE=$(mktemp -d "${TMPDIR:-/tmp}/gosqlite-limit-verify.XXXXXXXX")
mkdir "$CASE/evidence"
git clone https://github.com/ms-xiaosnow/gosqlite.git "$CASE/repo"
cd "$CASE/repo"
git rev-parse HEAD > "$CASE/evidence/commit.txt"
git rev-parse HEAD^{tree} > "$CASE/evidence/tree.txt"
git status --porcelain=v1 > "$CASE/evidence/status-before.txt"
test ! -s "$CASE/evidence/status-before.txt"
export PATH="$GOSQLITE_GO_BIN:$PATH"
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off GOFLAGS=-buildvcs=false GOMAXPROCS=2
test "$(go env GOVERSION)" = go1.27.2
test "$(go env GOOS)/$(go env GOARCH)" = linux/amd64
go version | tee "$CASE/evidence/go-version.txt"
go env GOVERSION GOOS GOARCH GOTOOLCHAIN GOPROXY GOSUMDB GOPATH GOCACHE CGO_ENABLED GOFLAGS > "$CASE/evidence/go-env.txt"
sha256sum goal.md sqlite/query.go sqlite/limit_conversion_test.go sqlite/limit_conversion_oracle_test.go > "$CASE/evidence/source-before.sha256"
printf '%s\n' "$CASE" > "$CASE/evidence/case-directory.txt"
```

clone需操作者已有私库读取权限；不要把登录凭据写入命令、日志或报告。若测试文件不存在，所选提交不含此切片，应停止并确认版本，不复制新测试到旧提交后称其通过。

## 2 精确普通和race选择

以下函数保存原始输出与真实退出码，任一失败即停止。本节**不**添加manual标签、不选旧planner大矩阵、不启动fuzz或benchmark。

```bash
capture() {
  name=$1; shift
  test ! -e "$CASE/evidence/$name.log"
  printf '%q ' "$@" > "$CASE/evidence/$name.command"
  printf '\n' >> "$CASE/evidence/$name.command"
  set +e
  "$@" 2>&1 | tee "$CASE/evidence/$name.log"
  statuses=("${PIPESTATUS[@]}")
  rc=${statuses[0]}
  tee_rc=${statuses[1]}
  set -e
  printf '%s\n' "$rc" > "$CASE/evidence/$name.exit"
  printf '%s\n' "$tee_rc" > "$CASE/evidence/$name.tee-exit"
  test "$rc" -eq 0
  test "$tee_rc" -eq 0
}
export CGO_ENABLED=0
capture sourceguard go run ./internal/testscope
capture normal go test -timeout=0 -count=1 -v ./sqlite -run '^(TestLimitConversionTypes|TestLimitConversionErrorOwnership|TestLimitConversionPublicSQL)$'
capture oracle go test -timeout=0 -count=1 -v -tags oracle ./sqlite -run '^TestLimitConversionPinnedOracle$'
capture sourceguard-race go run ./internal/testscope
capture race env CGO_ENABLED=1 go test -race -timeout=0 -count=1 -v ./sqlite -run '^(TestLimitConversionTypes|TestLimitConversionErrorOwnership|TestLimitConversionPublicSQL)$'
capture oracle-race env CGO_ENABLED=1 go test -race -timeout=0 -count=1 -v -tags oracle ./sqlite -run '^TestLimitConversionPinnedOracle$'
capture vet go vet ./sqlite ./internal/sql
capture build go build ./sqlite ./internal/sql
capture modules go mod verify
sha256sum -c "$CASE/evidence/source-before.sha256" | tee "$CASE/evidence/source-after.txt"
git status --porcelain=v1 > "$CASE/evidence/status-after.txt"
test ! -s "$CASE/evidence/status-after.txt"
```

## 3 预期与通过标准

- 所有命令exit0；sourceguard通过，不能出现 `[no tests to run]`、`SKIP`、`FAIL` 或 `WARNING: DATA RACE`。
- normal/race各有3个顶层测试；1,016个叶案例全部通过：63类型、9错误所有权、944公开DB/Tx。包括2个DB/Tx父子测试节点时总PASS记录为1,021，不要把容器节点当成额外案例。
- oracle/oracle-race各344个固定比较全部通过，日志含 `fixed comparisons completed: 344`；固定版本不匹配或SQLITE_ORACLE缺失必须失败，不能skip。
- 拒绝转换要求ErrMismatch、公开Code20、nil Rows、无活动Rows租约；正常查询行精确相等。内存/工作量/取消/overflow错误不得改成mismatch。
- 此main切片不包含独立审阅补充的118项源码；那些结果在证据归档，不能声称本页命令额外执行了118项。
- 源码哈希和Git状态前后不变；保存所选commit/tree，不能用本次通过代替其他提交或完整SQL/平台验收。

## 4 失败排查与证据

版本/依赖错误先记录ENVIRONMENT_NOT_READY。不要换Go/SQLite版本、关闭数值错误码断言或在线替换依赖来制造通过。用既有安装/校验步骤补齐环境后，若需复验，建立新的CASE目录保留两轮证据。

初始旧oracle包装器曾把CLI普通错误文本误当成含数字码；最终测试采用官方 `.log stderr`，要求实际回调前缀 `(20)`。若本地没有该前缀，先保留完整stdout/stderr、CLI版本与来源；不能删掉Code20要求。历史HARNESS_FAIL和修正版对照见[源码证据](evidence/limit-conversion-20261011.md)。

任何非预期差异保留日志、exit、commit/tree、环境和哈希，标为FAIL，不扩大选测范围或转跑受限场景。将整个 `$CASE/evidence` 保存到操作者持久位置；其中无真实业务数据库或密钥。长测、物理恢复、其他OS及独立安全/法律审查仍未由此验收。

## 5 清理

先确认日志已复制到持久位置，人工核对 `$CASE` 确为本次新建隔离目录。随后只删除该CASE目录及其测试副本；不对当前工作区、原数据库、工具链或通配路径执行清理。失败时先保留目录调查，不重复运行覆盖旧证据。
