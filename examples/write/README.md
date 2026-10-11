# 普通写入与事务示例

本例使用默认明文回滚日志模式，通过 `database/sql` 创建 `docs` 表，在事务中绑定参数插入中文标题、读取该行并提交。只对可丢弃的数据运行；重复运行会再插入一行。

```sh
CGO_ENABLED=0 go run ./examples/write disposable-write.db
```

新数据库的正常输出为：

```text
inserted 1: 数据库检索
```

`example` 的具名错误返回值合并操作错误与数据库关闭、事务回滚错误。成功提交后的延迟 `Rollback` 返回 `sql.ErrTxDone`，这是 `database/sql` 的正常生命周期结果，不作为失败。资源释放完成后，外层 `run` 才输出成功消息；输出失败也返回非零状态。`main` 仅在该调用返回后报告错误并退出，不在持有数据库资源的函数中调用 `log.Fatal`。

验证范围：源码 AST 检查具名返回、显式错误合并、成功输出顺序及退出状态接线，并拒绝原始旧代码和纯文本改写；参数/状态测试仅使用内存回调；普通运行测试使用新临时明文数据库，检查精确输出、再次打开、事务内成功 CRUD 后正常回滚以及原生受支持配置检查。它不等同于 SQLite 全量 `integrity_check`，也不证明崩溃恢复或物理故障下的清理行为。

定向命令（须先完成源码及调用链审核）：

```sh
CGO_ENABLED=0 go run ./internal/testscope
CGO_ENABLED=0 go test ./examples/write -run '^TestWriteExample(LifecycleAST|ArgumentStatus|OrdinarySuccessReopen)$' -count=1
CGO_ENABLED=0 go run ./internal/testscope
CGO_ENABLED=1 go test -race ./examples/write -run '^TestWriteExample(LifecycleAST|ArgumentStatus|OrdinarySuccessReopen)$' -count=1
CGO_ENABLED=0 go vet ./examples/write
CGO_ENABLED=0 go build -trimpath -o /tmp/gosqlite-write-example ./examples/write
```

构建产物不作为上述测试的一部分执行。本次没有运行失败子进程、进程退出数据库工作流、原生 I/O 故障注入或异常数据库恢复。
