# CLI inspect caller-context 修复：独立窄范围审阅

审阅日期：2026-10-11 UTC。结论：本次指定的上下文传递修复通过独立普通验证，未发现需要阻止该最小改动的缺陷。结论仅覆盖以下固定源码和具名检查，不构成全项目或物理耐久性验收。

## 1. 固定审阅身份与来源

- 基点 commit：`776b23eb59ce9613bf4498d352a716a193cdc9c3`。
- 基点 tree：`8f6026330f968314456548a3b8684476fea73afb`。
- 审阅 frozen/fixed 内容 tree：`df90e7980520c872c0c21647ab57f892306bb14f`。
- 原生产码加同一新增回归测试的独立 negative/control 内容 tree：`fb9844c1df4cc1c750fe4e38d8b3a52cabbc541d`。
- frozen 源：`../inspect-context-freeze-v1`，2876 个文件。
- 工作目录：本目录下 `baseline/` 与 `fixed/`，均为独立副本，不含 `.git`。
- 使用只读 `git ls-tree` / `git show` 取得基点清单与原版 main.go；通过 Python 按 Git blob/tree 编码独立重算内容身份，没有写 Git object/index 或远端。
- `source-identity.json` 与 `identity.log` 保存上述身份。全部 350 个既有 `_test.go` 的字节和文件模式与基点相同；原断言未被移除、弱化或替换。这是源码保留证明，不宣称运行了这 350 个文件中的所有测试。

与基点只有两个路径不同：

1. `cmd/gosqlite/main.go`：生产仅一行 `sqlite.Open(args[1], sqlite.Options{ReadOnly: true})` 改为 `sqlite.OpenContext(ctx, args[1], sqlite.Options{ReadOnly: true})`。
2. `cmd/gosqlite/inspect_context_test.go`：新增 237 行回归文件，三个 Test 与两个只读 helper。

完整一行 diff 在 `one-line-production.diff`。基点原 main.go 与原证据 `baseline-main.go.txt` 一致；新增测试与原证据 `reviewed-inspect-context-test.go.txt` 一致。

## 2. 行为与帮助链审阅

执行前已阅读：新增三个 Test、两个 helper、既有 `readonly_test.go` 三个 Test、`runContext`、`sqlite.Open/OpenContext`、`DB.Schema/Info/Close` 与只读 pager 的 `OpenAdmitted/ReadPage/Close`。

- `inspect` 仍使用 `ReadOnly: true`，参数校验、错误返回、Close 错误合并、Schema(ctx)、格式字符串及 JSON 结构均未改变。
- 原 `sqlite.Open` 委托给带 `context.Background()` 的 OpenContext；一行修复让 inspect 的连接入口使用调用者 ctx。
- AST 测试定位 `runContext` 的 inspect case，要求唯一 sqlite opener，精确检查 `sqlite.OpenContext(ctx, args[1], sqlite.Options{ReadOnly: true})`。它直接抓住本缺陷。
- 完整输出测试覆盖三个页面尺寸的完整数据库及含 81 条 schema/600 列 overflow CREATE TABLE 的既有 schema fixture；整个 JSON 加换行逐字节比较，未只检查片段。
- cancel 用例只用预取消 context 和固定在 Unix epoch 的已过期 deadline，无 sleep/timing race，无后注入取消，无新缺失/损坏数据库。
- 两种预取消都在 runContext 入口返回。这些运行时测试在原码也会通过，因此不能单独证明 opener 已转发 ctx；AST FAIL/PASS 是必要的补充。报告不夸大为新获得完整的中途 I/O 可中断能力。
- fixture helper 对完整平面目录前后 inventory + SHA-256 比较，包括 manifest 与 oracle；另外检查四个数据库及两个 oracle 文件与提交内 manifest 一致。没有写 fixture、生成 sidecar、创建 namespace/fault 场景的帮助链。

## 3. 实际环境

`environment.log` 保存完整环境与工具 SHA-256。

```sh
ROOT=/workspace/scratch/5e06a74e5f65/durable-restart
export PATH="$ROOT/toolchains/go1.27.2/bin:$PATH"
export GOPATH="$ROOT/go-path"
export GOCACHE="$ROOT/go-cache"
export GOTOOLCHAIN=local
export GOPROXY=off
export GOSUMDB=off
export GOFLAGS=-buildvcs=false
```

实际工具：Go 1.27.2，linux/amd64。没有下载依赖或工具链。race 单独设 CGO_ENABLED=1，仅满足 Go race instrumentation 要求；普通测试、vet、build 和依赖核验均为 CGO_ENABLED=0。

原提交者的 `../inspect-context-evidence-v1/baseline-build.log` 记录过默认 `/home/agent/go` 只读导致的编译器失败。本独立验证沿用已授权、既有的 GOPATH/GOCACHE；两次 build 均通过。该历史错误为 Go 编译环境错误，不能标记为工具安全审查拒绝，也未将其原日志删除或改写。本次独立检查没有出现新的安全/权限拒绝。

## 4. 精确命令与结果

所有命令及工作目录逐项保存在相应 `.log`，退出码保存在同名 `.exit`。以下运行均指定单一包和具名 selector。

在 `baseline/`：

```sh
CGO_ENABLED=0 go test -count=1 -run '^TestInspectSourceForwardsCallerContext$' -v ./cmd/gosqlite
```

预期 FAIL，退出 1。唯一失败指向 main.go 的 `sqlite.Open(...)` 与期望 `sqlite.OpenContext(ctx, ...)` 不符；没有编译失败或 fixture 故障。证据：`baseline-ast.log` / `baseline-ast-expected-failure.log`。

在 `fixed/` 运行相同命令：PASS，退出 0。证据：`fixed-ast.log`。

在两个副本分别运行：

```sh
CGO_ENABLED=0 go test -count=1 -run '^(TestInspectContextReadonlyFixtures|TestInspectContextCanceled|TestInspectReadonlyFixture|TestQueryReadonlyFixture|TestQueryCanceled)$' -v ./cmd/gosqlite
```

两者 PASS，退出 0，无 Skip。覆盖完整四 fixture 输出、8 个 inspect cancel/expired 子场景与原三项只读/query 回归。证据：`baseline-runtime.log`、`fixed-runtime.log`。

在 `fixed/`：

```sh
CGO_ENABLED=1 go test -race -count=1 -run '^(TestInspectSourceForwardsCallerContext|TestInspectContextReadonlyFixtures|TestInspectContextCanceled|TestInspectReadonlyFixture|TestQueryReadonlyFixture|TestQueryCanceled)$' -v ./cmd/gosqlite
CGO_ENABLED=0 go vet ./cmd/gosqlite
CGO_ENABLED=0 go build -o "$ROOT/inspect-context-independent-review-v1/gosqlite-fixed" ./cmd/gosqlite
CGO_ENABLED=0 go list -deps -f '{{if .CgoFiles}}{{.ImportPath}} {{.CgoFiles}}{{end}}' ./cmd/gosqlite
```

全部退出 0；race 无数据竞争报告，依赖核验无 CgoFiles 结果行。`baseline/` 也独立以相同 Go 环境 build 到 `gosqlite-baseline`，退出 0。

两份独立生成 CLI 二进制分别运行四个命令，cwd 为各自副本：

```sh
../gosqlite-baseline inspect testdata/readonly/page512.db
../gosqlite-baseline inspect testdata/readonly/page4096.db
../gosqlite-baseline inspect testdata/readonly/page65536.db
../gosqlite-baseline inspect testdata/readonly/schema512.db
# fixed 副本将二进制名替换为 ../gosqlite-fixed，四个参数保持一致。
```

全部 8 次退出 0，stderr 为空。每个命令单独保存 `.command`、`.json`、`.stderr`、`.exit`。新运行 baseline/fixed 的完整 JSON 两两逐字节相等，并分别等于原提交者保留的完整对应输出。

输出 SHA-256：

- page512：756 字节，4 条 schema，`394f78bea05f33b79a759bb49896f1400856bd5e25c52385e0cd2d269d5be92d`。
- page4096：757 字节，4 条 schema，`8ff630bcbca8b80a6e2cef9e06e8f9ff30e5f0d883bb7224a4240b626c644be6`。
- page65536：755 字节，4 条 schema，`9376bb44026c4e80bdde3cb42649e0205ddf4d550956e35631f1d9fb23cf6b9c`。
- schema512：17576 字节，81 条 schema，`a91c8c2f67845cdc842d377722e59b8bae399ee08ab4955cf0a55e8ee5815834`。

## 5. 执行后完整性

`final_checks.py` 及 `final-verification.json/.log` 核验：

- frozen 全部文件、模式、Git blob 与 SHA-256 未变。
- fixed 全部文件未变，仍与冻结源逐项相同。
- baseline 除最初明确恢复的原 main.go 外，整个副本未变；新增 regression 仍与 fixed 完全相同。
- 所有 fixture 前后目录清单和 SHA-256 未变，manifest 的六个完整文件 digest 均匹配，没有新 sidecar。
- 原项目主树、原 frozen、原 evidence 未被本审阅写入；执行产物仅位于本独立审阅目录，以及用户已授权的既有 Go cache/module 路径。没有提交、发布、远端写入或旧冻结候选整合。

## 6. 复验步骤与余项

此目录的 `run_checks.sh` 保存所有实际使用的完整命令和停止条件。对本保留副本复验：先核对 `source-identity.json`、`one-line-production.diff` 与相关源；确认使用同一已有工具环境，然后运行该脚本，再运行 `python3 final_checks.py`。脚本保留预期的原码 AST exit 1，其他任一非零退出会立即停止。若发生安全/权限拒绝，应停止对应动作并记录，不能改路径或改方法绕过。

本项普通范围无待修复事项。以下未运行/未证明，仍保持原验收状态：聚合测试、fuzz、进程强杀、故障注入、physical recovery、WAL、backup、rekey、ANN、性能验收、其他 OS runner，以及任何缺失/损坏/namespace 新场景。也未通过确定性动态测试证明 runContext 入口检查之后、opener 内部 I/O 期间的取消时机；本次按授权使用源 AST 保证上下文参数转发。不能将该审阅扩大为以上门禁通过。
