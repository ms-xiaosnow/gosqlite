# 离线性能 CI 接线：精确增量证据

基点：360809d8c6a89e3e65666e03b87b13d40e3a8c26。原 goal §11.3 要求 CI 检测性能回归；此前只有比较器和 synthetic fixture hook，缺少对实际提交证据的 CI 入口。

## 已完成范围

新增 workflow_dispatch-only 工作流与 ci_input.py，固定 benchmarks/performance/evidence/ 相对根、路径／静态文件树／大小限制、明确 origin 声明。比较器的七类状态与退出码不变；输出仅在仓库外 RUNNER_TEMP 独占新建，并保留完整 JSON 日志。工作流 contents:read，checkout 不保留凭据，输入经环境变量引用，不启动 benchmark／数据库／ANN／网络采集或受限链。

来源声明只是操作者陈述，不证明测量真实。固定 benchmark 使用合成数据，不要求业务数据库；发布输入前须审阅其数据和环境元信息。没有实际证据包，真实 workflow dispatch / 性能比较 NOT_RUN。

## 新执行与复用

- 2026-10-13 增量 unittest discovery 仅选择 test_ci_input.py：17 方法 PASS，零失败／跳过，0.544s。含七状态映射、路径及输出拒绝、完整八对 synthetic PASS／REGRESSION走真实比较器。后者刻意在临时测试中使用来源声明以验证接线，不是实测。
- 两个新 Python 文件 ast.parse PASS；scripts/check.sh 的 sh -n PASS，未执行其正文。
- 只改五个已审路径，见 [source-scope.json](source-scope.json)。compare.py、validation.py、fixtures.json、原57测试均保持哈希；它们的原资格与未变 Go 代码/CI38372629570结果复用，没有本地重跑。
- 新提交自动普通/官方对照 CI 由现有 push 工作流取得，终态以精确提交 Actions 为准；手动性能工作流不会被 push 自动启动。

[completion.json](completion.json) 和三个具名命令的 .result.json/.stdout/.stderr 保存固定 Python3.12.14、argv、净化环境、UTC、自然退出与原始输出哈希。没有新的全仓文件清单、旧进度全文或其他被排除材料。

## 有终点的 goal 核对

原 goal §7.2/M7/§11.3 不指定 ANN schema3。现有 benchmarks/ann/main.go 已做固定种子、真实持久索引及正常重开、K=1/10/100、建索引耗时和工作计数；历史结果不冒充当前 M10 或 schema3 输入。

R1–R6、PG DDL、公开有限恢复/Backup、M9 离线导入/转换/模板已存在；未发现另外需要本轮独立补写的必需代码项。SQL 后续能力依据 goal 的分阶段兼容矩阵保留明确 unsupported。

剩余是权威[手工总清单](../../testing/finite-acceptance-summary-20261012-zh.md)的17组/82核心场景、真实受控性能与长期soak、目标OS/FS/介质、真实PG/Neon输入与复核，以及独立安全/许可/API发布审查。受阻旧链不重启，外部依赖不伪称安全拒绝。整体M10未验收。
