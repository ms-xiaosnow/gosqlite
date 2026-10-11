# Billion-dimensional sparse exact search

Run `go run ./examples/sparse-exact /new/disposable/path/sparse.db`.
The path must be new. The example leaves an ordinary SQLite-compatible file.

The 1,000,000,000-dimensional query contains two nonzeros and encodes into 32
GSS bytes. It runs native and SQL L2 searches on one transaction's own writes,
filters an inactive row, excludes NULL explicitly in SQL, and reports support
work (13 charged entries) separately from dimension. Native NULL candidates
are excluded automatically. All three selected distances agree.

The output explicitly checks that converting this value to dense fails before
allocation. This example is exact full-scan search, with no sparse ANN or
PostgreSQL execution. The ranking heap is bounded result retention, not an index.

命令状态：成功返回 0；参数数量不正确时向 stderr 输出用法并返回 2；
示例函数返回错误时向 stderr 输出错误并返回 1。JSON 结果仍写入 stdout。
`go run` 会包装示例进程的非零退出状态；需要区分 1 和 2 时先用 `go build`
构建示例，再检查该可执行文件的退出状态。
