# inspect 调用者上下文：独立修正与普通验收

日期：2026-10-11 UTC。仅此最小修正的限定验收通过；M10 未完成。
中文完整自测入口仍为 [manual-verification-zh.md](../manual-verification-zh.md)。

## 修改和身份

- 原规范：G1、§3.6、§12.2。
- 生产仅改 `cmd/gosqlite/main.go` 的 inspect 打开调用：
  `sqlite.Open(args[1], sqlite.Options{ReadOnly: true})` →
  `sqlite.OpenContext(ctx, args[1], sqlite.Options{ReadOnly: true})`。
- 新增 `cmd/gosqlite/inspect_context_test.go`：三个 Test、两个只读 helper。
  参数、错误合并、正常 Close、JSON 格式和原有全部测试保持。
- 原实现与独立审阅基点：`776b23eb59ce9613bf4498d352a716a193cdc9c3`。
  独立审阅源 tree：`df90e7980520c872c0c21647ab57f892306bb14f`；
  原码加完全相同新回归 tree：`fb9844c1df4cc1c750fe4e38d8b3a52cabbc541d`。
- 全普通门基于文档提交 `e40b0081e173d171e05816a04bcd1a188d14609f`，
  只叠加两条已审阅源码路径；精确组合 tree：
  `0124d04f32966846bc38537863274390ca5daccc`，2,879 个文件。
- 最终提交另外加入本次证据和状态文档；其完整 tree 因文档而不同。
  所有非文档源码必须逐字节、模式等于上述通过普通门的组合，不声称两个
  完整 tree 相同。最终远端、Library 恢复和 CI 身份另记检查点。

## 原码负对照和独立审阅

原 `sqlite.Open` 使用 Background context。新增 AST 断言对原码真实
FAIL/exit 1，同一个断言在一行修正后 PASS。预取消/已过期 context
在原码也会于 runContext 入口返回；原码普通运行回归本来就通过，
不能把这次发现说成实测中途取消超时故障。

完整 readonly fixture 覆盖 512/4096/65536 页面和含 81 个 schema
条目、600 列 overflow CREATE TABLE 的 schema512。完整 JSON 加换行
逐字节保持；两套独立 CLI 共八次输出都与原证据相同，exit 0、stderr 空。
目录十个既有文件的 inventory/SHA 和提交 manifest 前后保持，无 sidecar。
限定六项 race、vet/build、无 CgoFiles 检查通过。

[独立审阅全文](inspect-context-20261011/independent-review.md) 保留其执行
时的工作目录和精确命令；那些外部开发副本路径是历史出处，不是仓库依赖。
[原始日志 JSON](inspect-context-20261011/scoped-original-logs.json) 逐字保存
owner/independent 的负对照、正向运行、race 和环境结果。

首次 baseline build 因默认 `/home/agent/go` 只读而 exit 1，原日志保留。
这是 Go/OS 缓存配置错误，没有工具安全拒绝。随后沿用项目已有离线
GOPATH/GOCACHE 成功，没有下载、提权或改变 GOPROXY/GOSUMDB 策略。

## 精确组合的完整普通门

Go 1.27.2 linux/amd64；固定官方 SQLite 3.54.0 oracle；既有离线依赖，
GOMAXPROCS=2，默认 `/tmp`，生产 CGO_ENABLED=0；race 单独为 1。
仅 Linux 实际运行，其他五目标是交叉编译。

| 实际范围 | 自然结果（UTC） |
|---|---|
| 全仓普通单元，count=1 | PASS 02:08:00 |
| 全仓 vet / trimpath build | PASS 02:08:03 / 02:08:07 |
| 完整 scripts/test-oracle.sh，未加 run/package 筛选 | PASS 02:12:02 |
| 全仓 race，count=1（sqlite 405.029s） | PASS 02:21:05 |
| linux/arm64、darwin/arm64、darwin/amd64、windows/amd64、windows/arm64 build | 全 PASS，末项 02:21:51 |
| go mod verify；前后完整内容/模式/清单与 tree | PASS 02:21:52 |
| 总退出码 | 0，02:21:52 |

执行前、oracle 前和 race 前均通过现有 source guard，并审阅本次新增测试
选择。CI 仍为 `.github/workflows/check.yml` 的原有 Linux ordinary/oracle
作业，没有新作业、runner、fuzz、manual 标签或受阻候选。

- [完整命令记录](inspect-context-20261011/ordinary-aggregate-command.txt)
- [源码身份](inspect-context-20261011/ordinary-source-identity.json)
- [阶段与完整日志哈希](inspect-context-20261011/result.json)
- [未裁剪原始日志 gzip](inspect-context-20261011/ordinary-aggregate.txt.gz)

解压日志仅作文本阅读：

```sh
gzip -dc docs/evidence/inspect-context-20261011/ordinary-aggregate.txt.gz > inspect-context-ordinary.log
sha256sum inspect-context-ordinary.log
# d57406b53c36cddedcc657c1248b186d143b473c4790990abb92f35167942c20
```

## 当前仓库的独立只读复核命令

按中文手册先准备 Go 1.27.2、隔离源码副本、证据目录和依赖；从副本根运行：

```sh
export GOTOOLCHAIN=local CGO_ENABLED=0
go run ./internal/testscope
go test -count=1 -v -run '^(TestInspectSourceForwardsCallerContext|TestInspectContextReadonlyFixtures|TestInspectContextCanceled|TestInspectReadonlyFixture|TestQueryReadonlyFixture|TestQueryCanceled)$' ./cmd/gosqlite
CGO_ENABLED=1 go test -race -count=1 -v -run '^(TestInspectSourceForwardsCallerContext|TestInspectContextReadonlyFixtures|TestInspectContextCanceled|TestInspectReadonlyFixture|TestQueryReadonlyFixture|TestQueryCanceled)$' ./cmd/gosqlite
go vet ./cmd/gosqlite
go build ./cmd/gosqlite
```

预期每项 exit 0；输出测试完整比较四个 fixture，取消测试不输出 JSON，
AST 必须找到唯一 OpenContext(ctx, …)。任何失败保存完整 stdout/stderr、
源码 SHA 和环境后停止，不修改断言、删除 fixture 或转换成故障测试。
清理时仅删除用户创建的隔离副本/构建产物，保留原仓库和证据。

## 仍未通过的项目

中途原生 I/O 取消未动态验证；inspect 扩展配置/代际呈现尚未实现。
明文只读 WAL、公开恢复、明文 backup 等真实代码缺口仍存在。
ANN V4、新 41 目标 fuzz 和未知中断链路保持阻塞；物理故障、原生 OS、
长测/大型性能、独立安全/法律审查不在本次结论内。这里的普通 PASS
不证明 tmpfs 或 fsync=volatile 环境的断电持久性。
