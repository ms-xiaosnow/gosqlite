# 2026-10-11：query 行结果关闭错误传播（独立切片）

## 范围与原因

- 对应 `goal.md` G1 的资源关闭/错误行为与 §12.2 的 CLI query。
- 基线提交为 `1352ae524575e4d6fe8b5434fd930c3b07be4ac8`；工作在独立副本，
  不修改主工作树、Git 历史、远端或其他候选。
- `cmd/gosqlite/main.go` 只把 query 的 `defer rows.Close()` 改为延迟执行
  `resultErr = errors.Join(resultErr, rows.Close())`，保持原有命名返回值。
- 头部/行 JSON 编码或文本检查提前返回时，行关闭的错误现在会加入已有错误。
  `Rows.Close` 会从 DB 注销当前行结果，不能假设后续 `DB.Close` 会重新收集
  被原先裸 defer 丢弃的错误。既有 `DB.Close` 的错误合并不变。
- 正常 EOF 仍由 `Rows.Next`/`Rows.Err` 处理；已关闭的 Rows 再次 Close 返回 nil。
  此切片不修改 SQLite、输出 JSON、查询语义或 EOF 实现。

## 测试设计与明确边界

- `TestQuerySourceJoinsRowsCloseError` 解析实际生产 `runContext`：验证命名
  `resultErr error`、query 中实际 `db.Query` 后的 defer、完整 errors.Join
  赋值，并以 AST 对象身份确认目标是函数返回值、Close 接收者是已取得的 Rows。
- `TestQuerySourceRejectsLegacyRowsClose` 只在内存源码中恢复准确的旧语句，
  确认同一静态断言拒绝该语句；不构建或执行旧程序，不启动预期失败子进程。
- `TestQueryOutputWriterError` 仅让内存输出 writer 分别在列头和第一行 JSON
  写出时返回哨兵错误，检查生产 query 返回保留其 errors.Is 身份及准确输出前缀。
  输入是完好的既有 `page4096.db`。复用未修改的 fixture inventory/hash helper，
  检查全部 10 个输入文件和 manifest，包含目录清单，不改动数据库。
- 既有 `TestQueryReadonlyFixture`、`TestSQLWriteAndQuery` 保持字节不变，作为
  正常只读/写后查询和 JSON/EOF 成功对照。全部 353 个既有测试文件保持不变。
- 不注入数据库/原生 I/O/Rows.Close 故障，不构造缺失或损坏数据库，不运行
  崩溃、进程退出、namespace、recovery、加密、ANN V4、41 目标 fuzz 或旧 held 链。
  静态合并证据不冒充真实关闭故障运行证据，也不扩大数据安全/持久性验收范围。

## 验证状态

主任务阅读生产改动、新测试及复用 helper 后批准固定范围；以下命令首次执行均
通过，2026-10-11 03:27:42 UTC 自然退出 0，没有失败、重试或范围扩展。

- 固定 Go 1.27.2、GOTOOLCHAIN=local、GOPROXY=off、GOSUMDB=off、
  GOFLAGS=-buildvcs=false、GOMAXPROCS=2，固定 GOPATH/GOCACHE；临时目录使用 /tmp。
- `CGO_ENABLED=0 go run ./internal/testscope`：PASS。
- 普通/race 使用同一准确 selector：
  `^(TestQuerySourceJoinsRowsCloseError|TestQuerySourceRejectsLegacyRowsClose|TestQueryOutputWriterError|TestQueryReadonlyFixture|TestSQLWriteAndQuery)$`。
- `CGO_ENABLED=0 go test -count=1 -v ./cmd/gosqlite -run "$selector"`：
  五个定名测试及输出 writer 的两个子测试 PASS，0.016s。
- `CGO_ENABLED=1 go test -race -count=1 -v ./cmd/gosqlite -run "$selector"`：
  相同范围 PASS，1.059s。CGO 只用于竞态工具，生产构建仍 CGO0。
- `CGO_ENABLED=0 go vet ./cmd/gosqlite` 和
  `CGO_ENABLED=0 go build -trimpath -o <证据目录>/gosqlite ./cmd/gosqlite`：PASS。
- 执行前后 2,906 文件、137 目录的完整字节/模式/清单相同；10 个 fixture 文件
  与全部 353 个既有测试文件保持相同。收尾仅更新本中文记录，不修改通过检查的源码。
- 原始环境、脚本、stdout/stderr、退出码和前后身份清单保存在独立
  `query-close-evidence-v1` 证据目录，交主任务独立复审；本切片冻结后不再改写。

这里只覆盖 Linux/amd64、完好普通 fixture 与命名成功对照。未执行全仓测试、全仓
race、oracle、真实 Close 故障、加密/WAL/恢复/其他 OS 验证；不将这些未运行门记为
通过。下一步是独立复审，再由主任务决定是否整合及所需完整门。

生产文件 SHA-256：`b4aa185a54bb662ca92afeeb31fb2c2dff741bda6939ac612ffb91457efa2085`。
新测试文件 SHA-256：`6c5b359a38dac069a804c49f17a331bb2353ba999920e1254f5766e1432f5f6d`。
