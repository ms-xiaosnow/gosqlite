# CLI / 示例资源边界：四项独立修正

## 身份与结果

- 基点 commit：`1352ae524575e4d6fe8b5434fd930c3b07be4ac8`，tree `2b91b20abec4b2d651de1dd860fe86e79b62bac9`。
- 四项固定组合 tree：`4c2af13646c973c53340450f8f1310492aae5628`；2,911 文件、137 目录、14 个增改路径，353 个既有测试文件字节和 Git 模式不变。
- 独立组合普通/race 共用 18 个具名顶层测试，无跳过；sourceguard、四包 vet/四构建通过。[独立报告](cli-resources-20261011/independent-review.md) SHA256 `49b719d3b3ac4b40b1083b0c476897839613f834c7fbd3f421a6f2fd5cb9f2d7`。
- 完整既有普通门首次运行于 2026-10-11 03:53:38 UTC 自然 exit 0：CGO0 全单测/vet/trimpath build、完整 pinned SQLite oracle、两次 sourceguard、CGO1 全 race、五目标 CGO0 build、模块验证及全源身份检查全部通过。
- 完整日志 SHA256：`79832852402562dd53e1c1ffb811530740bfceea9ac6f43f0d485ed095885dd9`，19,104 字节。[命令](cli-resources-20261011/ordinary-aggregate-command.txt)、[日志](cli-resources-20261011/ordinary-aggregate.txt)、[阶段时间](cli-resources-20261011/stages.tsv)、[前身份](cli-resources-20261011/identity-before.json)、[后身份](cli-resources-20261011/identity-after.json) 均保存。

这些是限定源码证据。最终提交在同一已验证运行源码之外补充文档和证据，远端、恢复及 CI 结果须按其实际 commit 另记；不能以冻结树通过预先宣布远端 CI 通过。

## 实际变更与证明边界

| 修正 | 实现 | 已执行的覆盖 | 没有得到的证据 |
|---|---|---|---|
| query Rows.Close | 早退 cleanup 的错误合并到实际命名返回值，保留原错误 | AST 识别具体 Rows/返回对象；拒绝旧 defer；完整 fixture 的正常查询/CRUD与内存 Writer 早退 | 未注入真正 Rows.Close 失败。Writer 用例本身旧版本也会通过，不能冒充该问题的动态失败再修复 |
| readonly 示例 | 显式 ReadOnly/NoCreate connector；传递上下文；保留行、输出、关闭错误；报告前归还资源 | 三种页大小 × path/ro/rw/rwc 的 12 个普通读取组合与最终文件身份；纯格式/报告错误；AST 包括原始旧程序 | 没有动态尝试禁止写入/创建、存储关闭失败、中途取消或信号验证；没有执行旧 main，未观测源库被修改 |
| write 示例 | named-error helper 归还 Tx/DB 后才向外报告；保留 rollback/close/output 错误，正常 ErrTxDone 单独处理 | 成功提交/输出/重开、CRUD 后正常 rollback、支持域 Check；纯参数/callback；原程序和八个变体的 AST | 没有故障注入、真实 cleanup 失败或失败进程；失败 Writer 传播只有 AST 断言 |
| sparse-exact 示例 | 参数/状态 helper；main 接入结果，成功 0、用法 2、普通返回错误 1 | 纯 callback/输出/context；原样既有成功例；精确主函数 AST 与旧源码负面对照 | 没有执行实际失败子进程；没有运行数据库 process-exit/crash 工作流 |

原问题来自源码审阅。旧程序作为 AST 数据不满足新契约，不等于曾执行旧数据库程序并观测失败。readonly 外部 AST 进程提案在运行前未被采纳，改为 passing 单元断言；其未运行材料保留为 NOT_RUN，没有平台/工具拒绝。

## 环境、范围与原始证据

固定 Go 1.27.2 linux/amd64；GOTOOLCHAIN=local、GOPROXY=off、GOSUMDB=off、GOFLAGS=-buildvcs=false、GOMAXPROCS=2。SQLite 3.54.0 CLI 只作测试 oracle；生产 CGO0。race 工具单独 CGO1。五个额外构建为 linux/arm64、darwin/arm64、darwin/amd64、windows/amd64、windows/arm64，不是这些平台的运行验证。

[原始 scoped 命令、输出与退出记录](cli-resources-20261011/owner-scoped-evidence.json)、[组合选择结果](cli-resources-20261011/independent-summary.json)、[增改路径清单](cli-resources-20261011/source-manifest.json) 保留各自身份。readonly 先五项、后新增静态旧源码断言再六项的成功记录均保留，没有覆盖原日志。没有失败、拒绝后的重试，亦未改动原有测试。

完整门源码前后字节、Git 模式、原始文件模式和目录清单一致；最终 Git 提交及 Library checkout 的原始 POSIX 权限不必相同，应另外比较 Git 模式和内容。默认 `/tmp` 和当前文件系统条件不提供真实断电耐久性证明。

## 持久化与后续

- 基点 1352ae5 的 CI `38108065647` 于 03:42:12 UTC SUCCESS，普通 job `114377684464` 与 oracle job `114377684618` 均成功；Library `libfile_f978e66501808191974c6b8b0dbf8b8d` 的精确恢复已通过。见 [持久证据](inspect-metadata-20261011/persistence.json)。
- 本四项候选的预发布静态备份：Library `libfile_04f3cc112b6c819195835806eb19903a`，3,181,408 字节，SHA256 `a8f786b30d4a7f72eada013508c07eeb4a302a0d782ddf6ff21d5791b81b3c73`。读回全部 300 个有清单条目并以已恢复基点加 14 路径重建精确 4c2af136 树通过；该包创建时完整门仍运行，其 README 的 pending 状态是当时事实。静态归档完整性不是执行验收。
- [中文限定复验规程](../cli-resource-verification-zh.md) 给出环境、隔离副本、实际选择、预期断言、失败与证据清理；编写该规程未执行其中命令。
- [有限收尾边界](../closeout-scope-20261011.md) 保留实际代码 HOLD、缺失 harness 和外部资格。当前仍不作整体验收或最终交付声明。
