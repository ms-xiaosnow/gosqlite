# LIMIT/OFFSET 数值转换的独立修正

日期：2026-10-11 UTC。对应 goal G1、§4.2 已声明支持的 SQL 类型、affinity 与错误行为；不是新增语法，也不是旧 planner 全量合入。

## 源码身份与范围

- 已保存基点：`63c2ebb10e3a2c4ff2378ee9f33c46133134c70f`，tree `d735409ebd1c43d90ee1650c8a7c2a084e270674`。
- 专用候选：tree `3fc9087ad121d423174a658e118ac455d1ace8c2`，2,964文件；原代码加最终相同测试的基线 tree `80d2ac039453de28581ea6f499d3914f65fee965`。
- 生产仅改 `sqlite/query.go:limitValue` 与删除不再使用的 strconv import。两个新回归文件为 `sqlite/limit_conversion_test.go` 和 `sqlite/limit_conversion_oracle_test.go`。
- 360个原测试文件和2,961个其他原文件保持字节及Git模式；未改 numericText/ApplyAffinity、grouping/counter/preparation、VFS、recovery、backup、ANN 或 backend。
- 精确 patch SHA-256：`b712838805fadd0215464c828169226425170df00377a3c13df424cb0a4bc4bc`。见[源码身份](limit-conversion-20261011/source-provenance.json)及[完整补丁](limit-conversion-20261011/source.patch)。

转换复用既有 NUMERIC affinity，拒绝值包装 ErrMismatch，公开返回 CodeMismatch(20)。完整 Eval/preflight 在任何 NUL 截断之前完成，保留原配额、取消及表达式错误。嵌入 NUL 文本只在此处采用前缀 REAL 后 NUMERIC 的规则，不改全局 affinity。

## 实际修前与修后

最终测试字节在原代码与修正代码上保持相同，独立复核重新取得一致结果：

| 固定选择 | 原代码 | 修正代码普通 / race |
|---|---|---|
| 63个直接类型值、9个错误所有权、944个公开DB/Tx叶案例 | 630 FAIL、386 PASS | 每模式1,016 PASS、零skip |
| 43固定SQL表达式 × 4查询形状 × LIMIT/OFFSET | 208 FAIL、136 PASS | 每模式344 PASS、零skip |
| 独立补充的有限值、完整值配额、正常同文件peer-write读锁归还 | 不作原代码覆盖推断 | 每模式118 PASS、零skip；仅审阅证据，不属于新增主树测试 |

原问题包括误接受Go十六进制/下划线语法和Unicode空白、REAL MinInt64端点误接受、嵌入NUL误拒、错误未包装导致公开code1而非20。INTEGER与精确整数TEXT端点、正常结果、非有限值、BLOB/NULL及数学overflow类别有保留控制。

三个普通顶层测试为 `TestLimitConversionTypes`、`TestLimitConversionErrorOwnership`、`TestLimitConversionPublicSQL`；oracle为 `TestLimitConversionPinnedOracle`。完整行为/选择审阅见[范围记录](limit-conversion-20261011/source-scope.md)，统计见[结果](limit-conversion-20261011/owner-test-summary.json)。

## 保留的测试包装器失败

第一次oracle包装器错误地要求普通stdin诊断相邻文字 `datatype mismatch (20)`；固定CLI该处并不输出数字码。因此v1基线184项包装器失败与24项生产差异、v1修正代码184项包装器失败与160项成功，不能算完整有效对照。

v2仅变两行：开启官方CLI已有 `.log stderr`，要求stderr以真实回调 `(20) ` 开头并包含datatype-mismatch文字。自然exit1、空stdout、原生ErrMismatch/Code20/nil Rows、精确成功行与资源断言均保留。已核对官方shellLog和VDBE日志来源；不从SQL文本任意位置猜错误码，不新增C observer、扩展、环境变量或CI依赖。

原v1日志与[两行修正](limit-conversion-20261011/oracle-harness-v1-v2.patch)均保留。最终相同v2字节重新在原代码与修正代码执行，才取得上表208 FAIL→344 PASS的有效对照。没有将包装器错误归为平台拒绝或全部归为生产缺陷。

## 独立审阅与组合门

[独立审阅](limit-conversion-20261011/independent-review.md)为本切片 ACCEPTED，SHA-256 `2a00770dd9331d9afffb3934925d6417b56dfd62947d5400e329f66acb3c4137`。源码guard、限定vet/build、CGO0生产依赖检查与精确补丁重放均通过；旧断言没有删除或削弱。

已与五份[限制分类更正](../restriction-provenance-20261011.md)文档合成冻结 tree `ca8947811d60266c19db2d6b155bd62b843d6e48`，2,965文件/142目录；八路径差异，779个非docs文件与候选完全一致。见[组合身份](limit-conversion-20261011/combined-source-manifest.json)。该组合的18阶段完整既有普通门于07:39:21 UTC全部自然exit0：全普通37测试包、完整oracle38包、全race37包、vet/trimpath build、两次guard、五目标生产编译、模块及全源前后身份均PASS。原始结果见[完整门报告](limit-conversion-20261011/aggregate-report.md)及[机器记录](limit-conversion-20261011/aggregate-summary.json)。非verbose全套输出不枚举叶断言，不把此前118个额外审阅控制算作本次全套新执行。其后主树仅追加文档/证据，779个非docs文件将再次核对；新提交、Library恢复与CI待实际结果，不引用旧63c2替代。

## 明确未覆盖

LIMIT=0的OFFSET抑制、EXPLAIN bound执行、完整name/ON绑定顺序、其他planner阶段/排序差异仍需后续源码修正；此切片不声称旧V4c广泛回归全部通过。公开测试选择非零有效bound，直接转换仍覆盖零；这是新增回归的明示范围，不是删弱旧测试。

本批不执行物理故障、crash/强杀、热恢复、namespace干预、磁盘错误密钥/篡改/文件缺失、中断发布/rekey、ANN V4、41目标fuzz或未知中断链路。其他OS仅后续构建，非原生资格。整体M3/M8/M10验收未完成。

复验步骤、环境、输出与证据保存见[中文限定复验](../limit-conversion-verification-zh.md)。压缩日志可用 `gzip -cd` 查看，文件哈希见[证据清单](limit-conversion-20261011/evidence-files.json)。
