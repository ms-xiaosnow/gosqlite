# 离线 ANN 报告自洽性校验

`reportcheck.DecodeReport([]byte)` 是可复用入口；`Report.ValidatePass()` 校验已解码的
Go 值。命令 `cmd/ann-report-check` 只从标准输入读取一个 JSON 文档。
这里的 VALID 只代表 schema3 的报告内容自洽，绝不代表真实性能、真实 recall、
实际运行、宿主机或源代码身份、数据库文件、物理持久性已经验收。

## 调用

在 gosqlite 模块根目录，使用固定 Go 1.27.2 构建：

```sh
CGO_ENABLED=0 GOTOOLCHAIN=local go build -buildvcs=false -trimpath -o ann-report-check ./cmd/ann-report-check
./ann-report-check < /path/to/schema3-report.json
```

这里的命令是使用说明，不是已执行记录。它使用仓库固定的 Go 1.27.2 工具链。
命令不接受路径参数或其他参数；输入重定向由调用方 shell 完成。读取最多
256 MiB + 1 字节，超过 256 MiB 时拒绝，不继续读完超大流。输入必须结束，
才会开始校验；不设流读取超时。内存占用不是 256 MiB 的硬上限，解码时还会
分配对象、令牌和种子摘要缓冲区。

正常自洽时退出 0，stdout 是一行 JSON：

```json
{"status":"VALID","self_consistency_valid":true,"performance_accepted":false}
```

输入/读取/校验失败退出 1，stdout 是 INVALID、false/false 和 `error`。
stdout 写入失败时退出 1 并向 stderr 报错。用法错误退出 2，只向 stderr 打印
用法。错误文本属于诊断信息，调用方不应依赖其顺序或精确字串。

Go 调用示例（`input` 为调用方已读入、受控大小的报告字节）：

```go
report, err := reportcheck.DecodeReport(input)
if err != nil {
    // 输入未通过报告自洽性检查。
    return err
}
_ = report // 自洽并不证明输入记录是真实测量。
```

导入路径为 `github.com/ms-xiaosnow/gosqlite/benchmarks/ann/reportcheck`。
对不可信 JSON 应用 DecodeReport，不要先用普通 json.Unmarshal 再只调用
ValidatePass，否则会丢失重复键、未知键和缺失字段的线级检查。

## 接受的输入合同

只接受完整 schema3：`schema_version=3`、`harness_revision=ordinary-ann-v3`、
`benchmark=native-ann`、`workload_version=gaussian-heldout-gsv-v1`、
`measurement_version=paired-exact-ann-reopened-v3`。

`report.go` 的 Report、Sample、Observation 和 `wire.go` 的配置/协议类型是
精确字段定义。每个没有 `omitempty` 的导出字段都必须出现，包含值为 false、0、
空字符串的字段。字段大小写必须完全一致；重复键、未知字段、尾随第二个 JSON、
非法 UTF-8、错误数值类型、null 标量/结构体均拒绝。数组长度、字段空值和集合
内容还受语义检查约束。JSON 最多 64 层、4,000,000 个被计数的值节点。

完整报告至少包括以下相互一致的证据组，不能只给聚合统计：

- Config 的源数据 rows/dimensions/queries/seed、方法与探测参数、源代码摘要、
  受控硬件标签及四项预算；Admission 必须与配置重新计算的保守估计一致。
- Algorithm 必须是既有 ANN 配置的规范 envelope，保留 uint64 seed 的整数精度、
  规范字段顺序和所有训练/算法策略；QueryPolicy、Limits、TimingSemantics 必须
  与固定 schema3 合同完全一致。
- 独立 dataset、query 和合并 GSV SHA-256；校验器仅在内存中重建固定种子序列的
  摘要。工作负载、build、hardware 摘要均从输入的完整身份字段重新核对。
- 报告声明的 Go 1.27.2、支持平台、编译配置和硬件观测字段；这些是待校验的输入，
  校验器不读取当前机器的硬件、build info、环境变量或 `/proc`。
- 开始/结束 UTC、elapsed、插入和建索引时间、空间/堆峰值、正常关闭计数和状态、
  reopen/model 标记；`status=passed`、无错误和无未确认关闭只是报告中的声明。
- 三个 Sample，按 K=1/10/100 排列，每个具有 Config.Queries 个原始 Observation。
  每条 Observation 包括 ordinal、exact-first、完成状态、两次耗时、交集计数、
  exact/ANN 各自完整 Hits、Stats、Plan。sqlite 公共值类型的字段保留原大小写，
  例如 `Hits`、`RowID`、`Distance`、`Stats`、`Plan`，不能改成猜测的小写别名。
  汇总 recall、分位数和计数均由原始 observations 重新计算并逐项一致。
- ANN 必须符合本合同的 native ANN 路径；exact fallback 会保留为输入事实但拒绝
  自洽通过。本检查不读取数据库，不重新进行向量搜索，也不认证 Hits 的来源。

报告里的 `retained_data_directory` 和 `retained_report_path` 只按报告 GOOS 检查
路径语法和二者关系。不会打开、遍历、stat 或验证路径下的任何文件；绝不把路径
存在或数据库内容完好当作已经证实。

现有 `benchmarks/ann/main.go` 保持原样，它产出的旧聚合报告缺少上述原始证据和
schema3 身份，不能输入这里取得 VALID。不能仅改版本号、补默认值或重新算摘要
就宣称旧聚合报告已兼容。本切片没有接入或执行 schema3 的 native 测量生产器。

## 输入示例及测试资料

完整合成输入位于 `cmd/ann-report-check/testdata/synthetic-schema3-hnsw.json`，
来源与机械构造说明见同目录 README。它明确带有 SYNTHETIC 标记，复制历史原始
观测并构造 schema3 元数据，不能用于性能验收。可以实际把它送入命令：

```sh
./ann-report-check < cmd/ann-report-check/testdata/synthetic-schema3-hnsw.json
```

预期为 VALID、`performance_accepted:false`；2026-10-13 的有限 normal/race
测试已确认这一完整入口结果。`TestANNReportCheckSyntheticStdinRoundTrip` 将完整覆盖
stdin reader → DecodeReport → ValidatePass → JSON 结果，而不只是测试输出格式。

`examples/incomplete-schema3.json` 是明确标为 incomplete 的合成骨架，只演示输入
方式；它缺少完整证据，预期 INVALID/退出 1，绝不是测量报告或可通过示例：

```sh
./ann-report-check < benchmarks/ann/reportcheck/examples/incomplete-schema3.json
```

包内的正向结构控制在 `report_test.go` 的 `modelMethodReport` 中明确实现：仅把编译期
嵌入的历史原始观测解码为内存值，并构造合成 schema3 元数据。这些 test-only
对象即使通过 ValidatePass/DecodeReport，也只能证明校验器接受相应自洽模型，
不能作为新的实测、性能基线或旧报告升级器。它不导出生成测量报告的公共函数。

`testdata/schema2-hnsw.json`、`testdata/schema2-ivf.json` 保留原字节，既有 SHA-256
断言和直接 schema2 拒绝测试保留。不改写历史来源文件。

## 边界与验证状态

这是源代码最小拆分：保留值类型、config、wire、ValidatePass、纯种子/GSV 摘要
和三处计数/同 row 距离守恒修复。排除旧 main/run、native 测量、owner/ledger、错误图
遍历、硬件采集、进程/文件扫描、边界测量测试、fuzz 和 benchmark。

保留 sqlite.VectorResult/VectorStats/VectorPlan/QueryPlan/QueryStats 和默认限额
函数、ANN 配置规范化函数、pager.UndoLimit 的纯数值函数，因此它不是仅依赖
标准库的包。编译依赖包含有 native 存储实现的包；这与实际调用数据库入口不同。
逐函数调用闭包及初始化审核应与本次精确源码绑定，不应仅凭包名推断纯度。

2026-10-13 06:11:06 UTC，29根467命名项normal/race各PASS，格式、两包
vet/build和sourceguard PASS；native benchmark未运行。
[精确命令、源码身份和原始日志](../../../docs/evidence/20261013-ann-report-check/README.zh.md)。
