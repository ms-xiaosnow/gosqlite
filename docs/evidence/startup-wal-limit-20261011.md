# 2026-10-11 startup / WAL / LIMIT / 示例分层证据

本文件记录已经存在的结果与当前待办。编写本文件只做静态文本/JSON/哈希核对，
没有运行 Go、编译器、shell 检查、oracle、数据库、手工 fixture 或 CI。下面各
候选有不同身份；904ca 与 209ba 已先后取得 guard PASS / ordinary FAIL，各自
第 3–15 项 NOT_RUN。52b 的只改新测试常量批次已于 12:31:15 UTC 完成两 quick
加全 15 门 PASS，完整运行与 Library 归档见第 13 节。没有新 commit、提交恢复
或 CI 结果。原始 source-only 报告、独立 PASS 和两次 FAIL 分开保存。
第 1–7 节保留首次汇总时点的 HOLD/未运行表述；最新 52b 资格以第 13 节为准。

## 1 b349 已发布检查点闭合

- commit：`b3492ed822d59834e5159cb7964c9dbd937027f7`
- tree：`ff13ded52883edfbabf747547a11b4e4ed7301ee`
- parent：`50b627e11e9dbce7f2fde8df35c4634c52a06d52`
- 全历史 bundle：Library `libfile_c416989d053081919f257c1f3200564c`，5,769,076 字节，
  SHA-256 `dfe931d13fce5ec1984b50e69128478a2d025176827306a6585898845b8f4251`
- 保存读回：逐字节、bundle verify、fresh clone/fsck、精确 HEAD/tree/goal PASS
- 独立恢复：09:51:23–09:51:37 UTC，3,128 文件 / 789 非 docs 完整身份；六条
  AST/list/compile/nm Go 命令及文本选择核对 PASS。测试二进制从未执行，四个
  physical case 全部 BLOCKED / NOT_RUN，不包含 Connector 真实恢复资格
- CI：[38129577026](https://github.com/ms-xiaosnow/gosqlite/actions/runs/38129577026)
  于 10:05:10 UTC 自然完成 SUCCESS；ordinary `114440917650`、oracle
  `114440917334` 均成功。这是已经发生的真实结果，不是 startup isolation 证明
- 终态归档：Library `libfile_611e42320f0081919d6ef2a4f490a493`，12,197,085 字节，
  SHA-256 `d0e5477c3b8e01daea3af7d23b1cbdd73a084fa9347ace441110e54c0ecade9e`；
  逐字节与 266 条目 / 265 文件清单哈希读回 PASS

权威原件为工程目录中的 `public-harness-persistence-evidence.json`、
`public-harness-handoff-zh.md`、`public-harness-restore-verification-v1/report.md`
与 `public-harness-ci-evidence-v1` 的终态和原始日志；归档保留这些记录。
恢复报告 SHA-256 `45a4a34ed7ca2cc7d09420523928a54f4d1069ec44ef31f9a7f1d371d58987eb`。
本轮只在 checkpoints.json 末尾补记这个已闭合旧提交，不修改先前记录、不填未来 SHA。

旧 helper 全部未隔离用户初始化及父环境，故该 CI 的启动隔离为 **NOT_PROVEN**。
静态审计没读取真实用户配置或变量值，也没观察/推断额外命令。历史 SUCCESS
继续成立，不能被更强的“干净启动”声明替代；所有新 oracle / CI 当前 HOLD。

## 2 startup 源码修复与未运行资格

候选 `oracle-startup-isolation-work-v1`，tree
`5fec5ce5e5339f02dc2715d9a2d6bd4691b1c62c`，3,136 文件；相对 b349 为
30 改动、8 新增、无删除。源码 patch SHA-256
`776e87fce09b4db36caeb7bb48f3a92065b2d49dddb945028777669d3c8545f0`。

46 个 Go 站点、5 个 shell 站点均已静态映射，原 SQL、args、stdio/assertions/
普通完成方式保留；6 处相对 readonly fixture 来源保留。新增 test-only factory
和 source guard，固定领先 `-noinit`、五键 Env、绝对 executable/temp/CWD，
Linux-only admission。原始候选没有包含新 LIMIT、WAL 或 strict Option A backup；
原 startup 候选外另存 LIMIT 两站点与 backup 一站点适配；LIMIT 两处后来已
并入下述组合，backup 仍未并入。组合普通测试已执行，实际 oracle 仍未启动；适配源码本身不代表已执行。

官方原码审计确认 SQLITE_DEBUG_BREAK 先于 flags，用户初始化先于 version/help；
不存在“先跑版本查询就一定安全”的特例。Getwd/PWD/stat 和 installer 绝对 base
早期 preflight 的澄清独立保留。shell 清洁只覆盖 SQLite child，不宣称 parent
bootstrap/loader/compiler 全隔离；helper 的路径词法校验也不是原生文件身份证明。

原件：`oracle-startup-isolation-audit-v1/REPORT.md`、
`oracle-startup-isolation-evidence-v1/REPORT.md`、
`oracle-startup-isolation-evidence-v1/REPORT-CLARIFICATIONS-v1.md`、
`oracle-startup-independent-review-v1/summary.json`。
owner 原报告 SHA-256 `dd70ba9fd454019c43a667e69e3550aa9ad748899c6de197d052e4979ea028d8`。
独立 inventory/exact replay 和 stated Linux child-startup contract 源码审阅
PASS。独立原报告 SHA-256
`2fa0bef7faf65186fd889af6276a5d74b3276221b1ce78c78984941974261646`，
原 source-only NOT_RUN 原文保留；REPORT-ADDENDUM-v1.md 后续独立记录纯门。

随后单独获准的两条精确命令于 10:38 UTC 终结：14 顶层 Test、184 nested 路径，
合计 198 run/pass、2 package pass，零 skip/fail/空匹配；当前 AST/text guard
输出准确且 stderr 空。原件 `oracle-startup-pure-verification-v1/REPORT.md`，
SHA-256 `be36695da8a706b4a30ccb0ab0f5a508e60fa0980f49a9202078081fa6d11ae4`。
保留 existing cache、默认 /tmp 和原资源门，没有新 namespace、降阈值、清理、
重试或额外 Go 命令。候选全部 bytes/modes/tree 前后不变。此结果是纯字符串/
unstarted Cmd/source fixture/AST 资格；实际 oracle、shell 脚本、新 CI 仍
**NOT_RUN / HOLD**，不是 runtime startup 隔离或组合资格。

备份适配 metadata 里误写了“保留既有 LIMIT noinit”，独立 addendum 已澄清：
backup 原 helper 没有 noinit，由新 factory 增加；LIMIT 两处才是保留原有 noinit
并再增加领先一份。原报告/metadata 不覆盖，真实补丁和哈希未受影响。
完整未来步骤见[启动规程](../oracle-startup-verification-zh.md)。

## 3 WAL 当前普通非 oracle 结果

- 精确 tree：`dbbb1e4ebca107c9a12695d9ad498a0c2f2057cf`
- 基点：b349 / ff13；3,136 文件 / 797 非 docs；12 路径 delta 为 6 production、
  6 test，其他 3,124 基线文件、namespace 门及 public recovery harness 均保持
- 13 条普通非 oracle 命令全部自然 exit 0：guard/version、完整 unit、vet、
  no-CGO build、完整 race、五目标 compile、mod verify、生产依赖闭包
- normal/race 各 1,208 顶层 Test、41 个 Fuzz 目标 / 313 固定 seeds、5 个 runnable
  Example；37 个有测试包 PASS，11 个无测试文件包事件；无 FAIL/缓存结果
- 每 lane 有 7 个可选 oracle 叶项 SKIP；其中 1 个顶层 SKIP，另两个 PASS 父项
  各有 3 个 skipped 子项。不能把父 PASS 写成 oracle 已执行

七项是 TestNativeCheckOfficialSupportedFixtures 的 512/4096/65536，
TestNativeCheckOfficialFragmentedTrees 的 512/4096/65536，以及
TestNativeCheckOfficialUnsupportedCatalogAndDirtyExtension。SQLITE_ORACLE 未设置，
没有 CLI 被该普通范围启动。41 固定种子目标不是受阻的新 fuzz-engine campaign。

早先“6 个 Example 都可运行”是静态计数错误，原断言失败及更正保留。固定 WAL
树中 ExampleEngine_Levenshtein 没有被 go/doc 识别 Output，编译但 NOT_RUN；
没有就地改源码重跑，也没有把旧数量追改为六个通过。

原件：`wal-current-normal-verification-v1/REPORT.md`，SHA-256
`bf8373292d73659fd08c114444229a8bb523c3a9aebaade12451290bc04f9462`。
归档：Library `libfile_659a971392888191b068479e323e4fce`，8,632,822 字节，SHA-256
`849ceec884df8ea25fb16ce91e8e042becab8fd215c346c2d94d71b0e2032848`，
读回 130 条目 / 129 文件哈希 PASS；原记录 `wal-current-normal-archive.json`。

scanner 只接受精确私有 sealed proof，不把任意 errors.Is/As/Unwrap 当作依据；
memory prefix candidate 不提供 native inode/retained-directory/durability。
**原生坏尾安全发布器仍 MISSING CODE**，PlanPrefixReplacement/WriteCandidate
没有 production caller。普通成功 publication、65 并发 shared sessions、正常
checkpoint/reopen 不是物理故障或坏尾发布资格。五目标 no-CGO 编译不是外平台原生验收。

## 4 LIMIT 绑定与执行阶段

基线 tree `75ec6ac6aaf602c74daa1f7d6449534b52fbd5c8`；候选 tree
`4c5c5fa0b60ee728b2c6f2eb153325106270cc86`，各 3,130 文件，准确四路径补丁
SHA-256 `47f5b26fe93800c98f49c75601f00770b3cf95e01a235c94dd651e147207b899`。

| 实际阶段 | UTC | 结果 |
|---|---|---|
| 新鲜基线普通 | 10:25:57–10:26:08 | native exit 1；4 Test / 123 子项，3 顶层 FAIL、资源所有权项 PASS；49 条直接阶段断言失败 |
| 候选普通 | 10:26:49–10:26:59 | native exit 0；16 Test / 1,145 子项 PASS；零 skip/fail |
| 候选 race | 10:27:09–10:27:26 | native exit 0；同 16 Test / 1,145 子项 PASS；零 skip/fail，无 race 诊断 |

49 条直接 FAIL 是零 LIMIT/OFFSET NULL 的 1 条、17 个绑定优先级 case × 2 API
的 34 条、7 个 grouped/连接状态 case × 2 API 的 14 条。基线 resource-ownership
23 子项通过，123 个新增子项均实际到达；不是编译错误、panic、timeout 或空选
制造的预期失败。原始 exit 1、输出与 runner 自日志碰撞的执行前更正都保存。

候选每 lane 恰 1,161 个 test run/pass，另 1 package pass。16 项真实选择器：

1. TestPlannerParityV4cLimitErrorOwnership
2. TestLimitPhaseBindingPrecedence（62 子项）
3. TestLimitPhaseGroupedAndConnectionPreservation（38）
4. TestLimitPhaseResourceOwnership（23）
5. TestLimitConversionTypes（63）
6. TestLimitConversionErrorOwnership（9）
7. TestLimitConversionPublicSQL（946）
8. TestGroupedAggregateSemantics
9. TestGroupedAggregateBinding
10. TestGroupedAggregateLimitsAndLifetime
11. TestGroupedAggregateOffsetAndScalarFailureAtomicity
12. TestGroupedAggregateTopKAdmissionAndFailures
13. TestGroupedAggregateRowidLookaheadAndTopK
14. TestConnectionCountersLazyTransactionReads
15. TestCachedPreparationOutcomes（2）
16. TestReviewPreparationErrorPrecedenceAndIsolation（2）

原件 `limit-phase-normal-verification-v1/REPORT.md`，SHA-256
`3d0726fb025836421b2e5be4218b14db74a07a3ccadeb85c723253f5c4b48a9a`。
仅三条精确 Go 命令；无新 oracle、全包组合、CI 或物理恢复。更广 CAST/IPK/
LEFT-ON/WHERE/排序阶段缺口继续存在，不以这组 focused PASS 关闭整个 SQL 一致性。

## 5 Example 发现修复

candidate tree `ec48c505f48aed452b96cd6544c6e3e84187ac3d`，来自 b349；仅
`extension/similarity/example_test.go` 的 Output 前增加一个 LF 空行。没有改
生产代码、可执行语句、import 或断言。其余 3,127 文件保持。

- before SHA-256：`38a3a127d033e82ab218b77eddd146a781d1d7111721a0826b359df2db7c7868`
- after SHA-256：`08ec12b1419fc6940b25f72cfd330b96a1a44d16d940e1962b7342ecfdee990f`
- normal：10:30:22.830593–10:30:23.842159 UTC，exit 0
- race：10:30:29.731464–10:30:32.063108 UTC，exit 0
- 各 lane 恰 ExampleEngine_Levenshtein 一次 run/pass，另 1 package pass；零
  skip/fail、无其他被选 Test/Fuzz/Example、无缓存或空选，stderr 为空
- 原断言 `3 false` 由标准 example harness 捕获比对；没有单独原始 stdout
  `3 false` 事件。固定源码及 named run/pass 证明的是这项实际输出断言

原件 `example-discovery-independent-verification-v1/REPORT.md`，SHA-256
`5d4f17d29ef1767d23fcaeeb30f2e8662cbf994dc30abfee0e0708b5ff45303f`。
仅两个精确 selector，未做完整包/仓库/WAL/oracle/物理资格。旧 WAL 五个
Example PASS、第六个 NOT_RUN 继续保留；这里是新的独立源码与结果。

LIMIT 与 Example 联合保存于 Library `libfile_3f5a1f80f0a08191918042d9ce975b86`，
15,674,020 字节，SHA-256 `5af28a3a78f396556a5c22675a6732f5145b1daabb63105df5e54043929ed7a8`，
逐字节与 140 条目 / 139 文件哈希读回 PASS。原记录 `limit-phase-example-archive.json`。
该归档保存两个独立树、原始 FAIL 与 normal/race 日志，未推 main，也不代表组合验收。

## 6 未完成与下一步

本节保留首次 source-only 汇总时点；后来的组合失败与修复待验见第 8、9 节。

startup 精确源码独立审阅、纯选择器已有有限 PASS；新 oracle、新 CI 与各候选
的组合运行均须分开取得结果，当前不能合成一个“全部通过”版本。当前已知的 b349 记录是真实
旧提交，本文不填未知新 commit/tree 或自指 SHA。

真实实现缺口：native WAL-tail publisher、更广 SQL 阶段、通用明文 backup、
真正只读 plaintext WAL、部分 backend、Connector 物理恢复 harness 和完整故障
设施。strict Option A backup 正在另一候选中实现，main 明文 backup unsupported，
未来 ext-family-only 支持也未资格验收。缺代码与代码已有但 validation NOT_RUN
分别记录，不能全部称作等待人工测试。

实际物理四场景、故障/恢复/namespace/磁盘负向/中断发布或 rekey、原生外平台、
长期/性能、安全/密码学/法律及 M10 仍未验收。tmpfs、fsync=volatile、短样本、
no-CGO 外平台编译和历史 SUCCESS 都不能填补它们。ANN V4、受阻 41-target
campaign 与未知中断链路不恢复、不改名或换路。总体 M9 实现继续推进，不能
宣布全部可编码工作结束。

## 7 四切片源码组合与静态持久恢复（运行前的历史记录）

组合 tree `904ca698c3359bfe0674231f6d14ec8a9d704f30`：基点 b349 / ff13，
3,146 文件，55 个 owned 路径（37 修改 / 18 新增），其余 3,091 基线文件保持。
含 startup、WAL、LIMIT、Example 及新 LIMIT 两站点适配；不含 backup 代码或本批
七份新文档。组合 patch SHA-256
`c381c30197a3e96051818ea48a3327374f60c33009a2ded2f078d6bfcf201f2d`。
原件 `startup-wal-limit-composition-evidence-v1/identity.json` 及完整 inventories。

Library `libfile_02e12d771bf4819181a1dc96b20402bd`，14,655,519 字节，SHA-256
`2320862dbe7c1344cccc0582cf6d911195cf6557b653c7f363f40c629d2a9ce9`，
逐字节及 246 条目 / 245 文件哈希读回 PASS。原记录
`startup-combined-source-archive.json`。

`startup-combined-static-restore.json` 记录从归档 bundle 独立 clone/fsck 后，
准确覆回 55 路径，3,146 文件完整 bytes、Git/POSIX modes 匹配。HEAD 仍为
b349，组合有意保存为未发布 dirty worktree，不是已推远端 commit。这里只做
静态源码恢复，没有 project/test/script 运行；不能写成组合普通/oracle/CI PASS。
本批文档若随后组合，会再次改变完整 tree，不能用 904ca 当作含新文档的身份。

## 8 11:11 UTC 组合 ordinary 实际失败与原始归档

固定 tree `904ca698c3359bfe0674231f6d14ec8a9d704f30` 的本次计划共 15 条命令，
实际只运行前两条。guard 于 11:08:14.470491 UTC 自然 exit 0；完整 ordinary
于 11:11:04.902042 UTC 自然 exit 1，控制器于 11:11:05.402481 UTC 停止。
stderr 均为空。没有重试、源码修补后重跑、oracle、CI 或发布。

| 实际范围 | 终态 |
|---|---|
| 顶层声明 | 1,225 Test、41 Fuzz 目标、6 Example 全部被选中；1,262 PASS、9 FAIL、1 SKIP |
| 全部测试/子项/固定种子事件 | 5,341 PASS、23 FAIL、7 SKIP；无空匹配/缺失/重复/缓存结果 |
| 包 | 36 PASS、2 FAIL（driver、sqlite），11 个 no-test-file 包 SKIP，不能混为 oracle SKIP |
| 固定种子与示例 | 313 固定 seed、全部 6 Example PASS；没有 fuzz-engine campaign |
| 新增普通顶层测试 | 33 项通过，包含 197 个 startup 路径和 127 个新 LIMIT 路径；不抵消保存性回归 |
| 第 3–15 项 | vet、no-CGO build、race、五目标 build、模块核验、全模块及 shipping 闭包、完整 oracle 脚本、focused LIMIT oracle race 全部 NOT_RUN |

七个 oracle 叶项正是第 3 节所列缺省 SQLITE_ORACLE 的预期 SKIP，没有 SQLite
child 被启动；两个父项 PASS 不能改写为其六个子项执行。所有源声明名字都到达
不等于每个测试内部后续分支都到达，Fatal 后面的 rollback/reopen 等语句未运行。

9 个失败顶层分别是：driver.TestDriverHNSWPreparedBindingsAndRollback（2 条
直接断言）、driver.TestDriverIVFFlatPreparedBindingsRollbackAndReopen（2）、
driver.TestDriverIVFFlatSharedWALSnapshotAcrossReindex（1）、
driver.TestDriverTypedANNPreparedSavepointReopen（10）、
sqlite.TestANNQuerySQLAliasesOffsetsAndExplain（1）、
sqlite.TestIVFQuerySQLAliasesOffsetsExplainAndLaziness（1）、
sqlite.TestIVFQueryNullsAndUntrainedFallback（1）、
sqlite.TestIVFQuerySmallSourceRetentionAndSingleListExact（1）、
sqlite.TestVectorSQLHybridActualAccessAndExplain（1）。合计 20 条直接 EXPLAIN
断言，加 3 条父项传播构成 23 FAIL。typed-ANN 原日志没有打印实际 plan，只能
记录 missing real typed ANN plan / nil error；hybrid 保留 grams SEARCH，缺的是
bounded_heap/vector_metric，不把它编成整个 SCAN fallback。

只读源码推断：新 LIMIT 阶段块在 `!sel.Explain` 下才求值，使 EXPLAIN 的 r.limit
停在 -1；现有 prepareVectorRank 因负 limit 提前返回，后续 ANN 看不到 vectorRank，
无法生成原 ANN/有界堆诊断。此因果链来自固定源码与断言签名，未插桩观察局部变量。
同样 23 条路径在旧 WAL tree 有真实 PASS；测试源及 vector/ANN planner 保持原字节，
query.go 有新阶段差异。旧 PASS 保留为回归比较，不能借作当前资格。

两个资源门均通过，保持原有缓存、默认 /tmp、原阈值，无清理/迁移/降门或附加超时
强杀。源文件、目录模式和工具 pin 前后匹配；这是断言 FAIL，不是资源、编译或 runner
失败。原 source-only 评审未发现该交互，仍作为历史源码评审保留，不倒写成运行 PASS。

原件：`startup-wal-limit-combined-verification-v1/REPORT.md`，SHA-256
`32efe59d00f26e2a04b082c1735df11ea6c876b66be047a1522bd25f699ee8c9`；同目录保存
FAILURE-ATTRIBUTION.md、全部 23 FAIL 事件、精确 command/result/preflight/source
记录、untouched stdout/stderr、STOP.json 和原计划。第 3–15 项未执行不能写 PASS。

原始失败归档：Library `libfile_d29a1f60f5e481919e22ac0b0f2703fa`，11,910,305 字节，
SHA-256 `33ecc95bf99afb1698b2bf987e362dc7aa235e7b1052b3f0948bd67ca519b4b3`，
逐字节及 635 条目 / 634 文件哈希与模式读回 PASS；原记录
`combined-failed-qualification-archive.json`。归档 PASS 证明失败证据保真，运行仍 FAIL。

## 9 EXPLAIN bound hint 窄修待验（首次源码冻结时记录）

本节保留 209ba 尚未执行时的原记录；后来真实 FAIL 及测试修正见第 10、11 节。

已冻结独立源码副本 `startup-wal-limit-explain-repair-v1`，tree
`209ba515f4dce18ae57103254b4053d4708b8736`，3,147 文件；只修改 query.go 并新增
limit_explain_hint_test.go，其余 3,145 个原文件及全部 380 个既有测试文件保持。
源码 patch SHA-256 `e9de77785abcb4b044447fde418d9b8947c0f94c3b76c25629a5ec9e20cc07fa`；
query.go SHA-256 `b90261bec75561fa3ede9bc26a50e70db4c1afa73cd2104f8d1241a0abe8efa6`。
这是实际 owner 身份记录与源码的静态核对，不是未来文档组合 tree 或新 commit。
原件 `limit-explain-repair-evidence-v1/identity.json` 和 source.patch。

窄修只为 EXPLAIN 恢复保守 LIMIT/OFFSET 提示：字面/已准入参数中的 int64、严格
位于 (-2^63,+2^63) 的有限精确整数 REAL、安全 unary +/-；对 MinInt64 取负拒作
已知。unknown NULL/TEXT/BLOB/function/CAST/binary/超界值不执行、不新增 bound
runtime 错误。LIMIT 0 不读 OFFSET hint；正 LIMIT 要求 offset 已知或缺省，负
offset 归 0，安全合计不超过现有 MaxResults；unknown offset 不能猜 0。
实际 SELECT 的绑定、求值、转换、错误阶段与资源语义不变，EXPLAIN 可以保守地
不同于 SELECT（数字文本 LIMIT 即一例）。只有 bound hint 不调用 Eval/预算/回调，
不能说整个 EXPLAIN 零预算或零元数据 I/O。完整合同见
[SQL ranking](../vector.md#sql-bounded-ranking-and-truthful-plans) 和
[ANN SQL integration](../ann.md#automatic-sql-selection-and-frozen-selection-limitations)。

本次文档已逐字核对这份冻结源码；独立源码审阅和运行结果尚未取得，仍为
SOURCE ONLY / NOT_RUN，未编译、未运行。不得预填未来 commit/文档组合 tree/CI。
新的 guard/普通/race/oracle 门必须各自有真实结果，不能抹掉第 8 节原始 FAIL。

## 10 11:50 UTC：209ba 的旧回归通过与新增测试合同失败

精确 tree `209ba515f4dce18ae57103254b4053d4708b8736` 的 guard 于
11:47:48.007436 UTC 自然 exit 0；完整 ordinary 于 11:50:37.244328 UTC 自然
exit 1，11:50:38.116856 UTC 停止。只运行原 15 门的前两条，第 3–15 项全部
NOT_RUN；stderr 为空。原 904ca 的 9 个失败根、23 条旧失败路径及全部 16 个
LIMIT preservation 根 / 1,161 路径已实际 PASS，但本轮总体仍 FAIL。

本轮有 1,229 顶层 Test、41 Fuzz 目标、6 Example；顶层 1,274 PASS / 1 FAIL /
1 SKIP。全部测试/子项/固定种子为 5,547 PASS / 23 FAIL / 7 预期 oracle 叶项
SKIP，合计 5,577；37 包 PASS、sqlite 1 包 FAIL、11 个 no-test-file 包事件。
313 固定 seed、6 Example 全部 PASS；没有 fuzz-engine campaign 或 oracle 运行。

唯一新失败根为 TestLimitExplainHintPlanPreservation：它把 HNSW 与 IVF 的
VectorRanking 统一错要求为 bounded_heap，实际既有 ANN 合同为 ann_exact_rescore。
两个 method 各 10 个直接叶断言，加 2 个 method 父项与 1 个根传播，合计 23
FAIL；不能与原 904ca 生产规划提示回归混为一谈。NOT INDEXED exact 的
bounded_heap 期望正确，原有生产 planner/ANN 常量未改。

原 ranking 判断先为真导致短路，后续 TopK/Method/Exact 比较及单列 IVF ready /
lists 断言未执行；打印字段符合期望只是日志/源码核对，不能倒写为这些断言 PASS。
修正必须取得新运行，不能凭打印值或旧 9 根通过代替整体资格。

原件 `limit-explain-repair-verification-v1/REPORT.md`，SHA-256
`2eb334a5a241a30e1d33f12d0f4925eff8be79e07c6214e3045569b96c8e4485`。
raw stdout SHA-256 `030614611b5063125801d8b517cfebaa2ff8dcffb4fe42c525162b045ae3948d`。
原 904ca 与 209ba 的源码、raw 输出、STOP、断言和此前独立源码评审均保留。

209ba 最初源码归档 Library `libfile_0cfd200f36d081918ff860638fd0af0f`，6,424,628
字节，SHA-256 `b8be38d5af2370893b43f8d56ca5f40a30087429dd7f4adbd3a895715cabf9db`，
88 条目 / 87 文件哈希与模式读回 PASS。当时 source-only NOT_RUN 原记录不改写；
后来的运行 FAIL 由本节单独记录。

## 11 12:04 UTC：只改 shape 常量的 52b 批次 RUNNING（历史观测）

本节保留当时 source-only 归档与运行中状态；终态另列第 13 节。

新 tree `52b03f1d94f578fb6e3722c7bf28fc0056fad6d4`，3,147 文件；只修改
sqlite/limit_explain_hint_test.go，其他 3,146 文件和全部目录模式保持。production
query.go SHA-256 仍为 `b90261bec75561fa3ede9bc26a50e70db4c1afa73cd2104f8d1241a0abe8efa6`。
新测试 SHA-256 `6e5d0fbc1bab3b0244b1b1604d4b3007e6f6962c85383c6a1d6fd1b264d3d94e`；
单文件 patch SHA-256 `9df0d731c55ab74ff8fb26cd70b6ab13a391a28e8db2e2150a3e1d12c0e814f8`。

shape 表显式要求 HNSW/IVF 的 ann_exact_rescore 和 exact 的 bounded_heap；
既有 SQL、参数、名字、TopK/Method/Exact/IVF、零工作量及 Close 断言都保留。
这不是削弱为非空/任意字符串，也不是按实际输出动态生成期望。
owner 报告 `limit-explain-expectation-correction-evidence-v1/REPORT.md` SHA-256
`9b5b0c954312e95aca5a740a5a27e90a685e4b3782c3542c0e4c8740de7fd749`；独立报告
`limit-explain-expectation-independent-review-v1/REPORT.md` SHA-256
`1c94bfb305a737b01b499eee977366f3eb7d331168cecc02ba20570333266a34`，源码审阅无阻断。

静态源码归档 Library `libfile_0ab4292eb7188191bc63c58a4f740cb6`，7,248,258 字节，
SHA-256 `3399a7b0ce2dbbe1b16e76c8ca7f2c88758e2c8d95f709e26ae07137980ebe74`；
33 条目 / 32 文件及内嵌 88 条目 / 87 文件哈希与模式读回 PASS。原件
`limit-expectation-correction-source-archive.json`。这是源码保真，不是运行通过。

最新独立执行批次安排先两条 quick 普通检查共 10 根，再从 guard 开始全新的
15 门；12:04 UTC 状态 **RUNNING / 等待终态**。当时尚无完整终态，不能提前记
阶段 PASS、实际 oracle 进程数或完整资格。所有具体结束时间、
具名结果与新报告哈希待终态后补记；该候选仍未发布，main 与 CI 身份未变。

## 12 独立 backup / Windows 结果，不并入本批资格

backup 固定 tree `fc0b1324441456f40160e81d5125dc7b97992773` 的六条有限 normal
模型/源码 AST 命令于 11:52:54–11:53:22 UTC 全部自然 exit 0。34 个顶层中有
25 个纯模型、9 个 source-AST；142 个命名子项，共 176 路径，零 skip/fail。
这些计数不是每个未命名断言的总数，也没有 native descriptor/lock/publication/
持久性资格。原报告 `backup-refusal-v2-finite-verification-v1/REPORT.md` SHA-256
`b7054292264da300b522cbc136e115c0e2aabd75e46114f188112a5dc67b9651`。

未发布 fc0b 候选中的六个 native Backup 场景仍未迁移；main 原来没有这六项，
不能写成从 main 普通选择减少 5/6 项。实际 native、oracle、aggregate、race、外平台、
CI 均不由这六门取得；原生持久性 NOT_RUN。有限 ext-family/唯一 mount-ID /
Openat2/no-replace 条件仍在，tmpfs/overlay 不资格替代。正常 positive/preflight
场景不能一概归为故障禁令，但迁移和执行仍须自己的审阅与适用存储。旧 Backup
oracle helper 尚未收口新启动合同。源码未发布，main 明文 backup unsupported，
不与 startup/WAL/LIMIT/Example 组合混合验收。

209ba 原始 FAIL 与这六门 backup 结果一起保真归档：Library
`libfile_2e50b4e09b408191a0e8df12c0a4e0d0`，21,917,681 字节，SHA-256
`e6ca6896e4454310426ed9f27fc9a359932e7ea3498b3196b5d13c2d3522ddf4`，
146 条目 / 145 文件哈希、模式和两个内嵌源码归档读回 PASS。原记录
`two-terminal-checks-archive.json`；两个不同树的 FAIL/PASS 范围各自保持。

Windows 切片的准确 tree 为 `5d1e457eed1e01b3705c0650c7aa9e42efa96559`，不是
Windows 原生验收：10 个可移植模型、20 子项与 1 个 source-AST 分别 normal/race
通过；另一个单项 TestEncryptionSharedRekeyLatestHeadOrdinary 在 Linux 上
normal/race 通过。共六条限定测试命令加一次工具版本查询，不能把版本查询算测试。
模型报告 SHA-256 `63a26702c2b2d90bc1ff348b9921204eb56027054110aba3e079f6574057d7e6`；
单项正确密钥报告 SHA-256 `b3b629688e089c37172b7697bd5ac197662b0e979029c03678af7b300c762aba`，
分别位于 windows-rekey-current-model-verification-v1 与
windows-rekey-current-happy-path-verification-v1 的 REPORT.md。
Library `libfile_915c342dc0e8819195b39c4905e2d539`，9,876,280 字节，SHA-256
`01a9cc6ddffb373444c2ad69874531489ac2a588a59d98992986171644db7c44`，
91 条目 / 90 文件及内嵌 52 条目 / 51 文件读回 PASS。该源码也未发布、未混入
52b，Windows 原生、故障/中断 rekey 和物理持久性 NOT_RUN，M10 未验收。

## 13 12:31 UTC：52b 精确本地两 quick 与全 15 门终态 PASS

固定 tree `52b03f1d94f578fb6e3722c7bf28fc0056fad6d4`，3,147 文件、148 个
实际目录。两 quick 于 12:05:26 UTC 开始，之后 12:05:54 UTC 从新 guard 启动
全 15 门。末条命令 12:31:12.989313 UTC 自然 exit 0；控制器完成时间
12:31:15.669004 UTC。全部 17 顶层命令自然 exit 0，因原脚本内含三次 Go，
实际共 19 次 Go 调用，没有追加 version/probe、重试、格式化、CI 或发布。

| 新鲜执行范围 | 实际有限结果 |
|---|---|
| quick sqlite / driver | 6 根 / 57 路径与 4 根 / 18 路径，合计 10 根 / 75 PASS，零 skip/fail；不替代全套 |
| full ordinary | 1,229 Test、41 Fuzz / 313 固定 seed、6 Example；5,570 PASS / 7 预期 oracle 叶项 SKIP；38 包 PASS / 11 no-test-file |
| full ordinary race | 同完整名字与路径集合，5,570 PASS / 7 预期 oracle 叶项 SKIP；零 FAIL |
| complete oracle | 1,334 Test、41 Fuzz / 313 固定 seed、6 Example；7,202 路径 PASS，零测试 SKIP；39 包 PASS / 11 no-test-file |
| two-name LIMIT oracle race | 2 根 / 394 路径 PASS，48 phase 与 344 conversion 比较叶完成，零 skip/fail；不是完整 oracle race |
| 其他完整门 | guard、vet、CGO0 build、五目标 CGO0 编译、模块校验与两种实际依赖闭包全部 PASS |

完整 ordinary/race 顶层各为 1,275 PASS / 1 SKIP；完整 oracle 顶层 1,381 PASS /
零 SKIP。三个 full lane 的全部 313 固定 seed 和 6 Example 新鲜通过，没有
fuzz-engine campaign。普通/race 的七个缺省 oracle 叶项仍是 SKIP，后来在完整
oracle 全部实际通过，不反向改写原 lane。包无测试文件事件也不混作测试 SKIP。

旧 9 个回归根 / 23 条路径、16 个 LIMIT 根 / 1,161 路径、新 4 个 hint 根 /
206 路径在 ordinary、race、完整 oracle 三个 lane 均 PASS。新 shape 常量
通过后，先前短路的 TopK/Method/VectorExact 及 IVF ready/lists 判断实际到达
并通过；不是把旧打印值重解释为执行证据。query.go 生产 hash、全部既有断言、
新测试之外的源码在测试前后保持；旧 904ca 和 209ba 两次 FAIL 完整保存。

完整 oracle 脚本实际用时 383.009 秒，保持原脚本/固定已安装 SQLite 二进制，
没有另行下载/构建。两名 LIMIT 在完整 oracle 与末条 race 各有 49 phase 路径
（1 根 + 48 叶）、345 conversion 路径（1 根 + 344 叶）。对应两根 identity
probe 加比较的固定构造及自然完成断言支持每个 lane 394 个官方进程计数；这
不是新增 OS 进程追踪器，也不能把该数字当完整 oracle 的所有进程总数。
新组合的 48 个 ForTest 站点及 5 个 shell 合同保持领先 -noinit、精确五键 Env、
固定 package CWD 与原 stdio/自然完成方式；不主张父 bootstrap 或 OS 完全隔离。

实际 whole-module non-test 闭包：206 包（49 project、145 standard、12 external），
1,316 个输入，包括 1,254 Go、54 assembly、6 headers、2 个已有 example EmbedFiles。
它包含 testoracle/testscope/os/exec/testing 等工具支持，不能称 shipping 闭包。
单独 shipping sqlite/driver/CLI union：164 包（20 project、132 standard、12 external），
1,227 输入（1,168 Go、53 assembly、6 headers）；无 testoracle/testscope/os/exec/C。
两闭包均无 Cgo/C/CXX/M/F/Swig/SwigCXX/Syso 输入；保留可信 assembly/header 事实。
模块仍仅 x/crypto v0.58.0、x/sys v0.49.0、x/text v0.43.0，mod verify PASS。
五目标编译不执行外平台二进制，不提升 Windows/Darwin 原生资格。

17 次顶层资源前置均维持既有 /tmp ≥ 1 GiB、workspace ≥ 2 GiB、各 10,000
available inodes 及缓存/目录权限要求。默认 /tmp 与原 cache 不迁移、不清理或
降阈值；只有第 14、15 门接收 SQLITE_ORACLE，manual gates 一直缺省。脚本只有
其既有顶层容量门及内部可用性检查，没有声称三次 Go 子调用各有新增容量采样。
固定 Go/SQLite 二进制与完整源码/目录模式/旧失败证据前后匹配，stderr 全为空。

最终报告：`limit-explain-expectation-verification-v1/REPORT.md`，SHA-256
`7d48f955e7404f1edcb13ceeca8c0f91ad62f46c71ba82da68ab4f75e150626e`。
同目录保留 17 组精确 command/preflight/result/source 与原始 streams、选择事件、
回归保持、oracle 比较、实际依赖闭包输入哈希、资源和最终读回；没有运行阶段待续。

完整资格归档：Library `libfile_3873c02e1c208191842637091c38b182`，10,701,653 字节，
SHA-256 `bff64721250496077a8a0ffe6fa4029f69718f3671d8ea57aa91e3e44c197d95`。
220 条目 / 219 文件哈希与模式、内嵌 33/32 和 88/87 源码归档读回全部 PASS；
原 metadata `startup-wal-limit-qualified-archive.json`。这是本地 52b 运行证据的
保真保存，不是新提交的 Library clone/恢复验收；没有未来 commit 或自指 SHA。

main 仍是 b349，当前发布基点的 CI 38129577026 SUCCESS 属于旧身份且启动隔离
NOT_PROVEN。52b 文档合入后会有新完整 tree，不能借本 tree 冒充；新 commit、
精确恢复和 CI 均尚未发生。native WAL-tail publisher、backup/nativeOS 等真实
代码或资格缺口继续，受限项和整体 M10 不因这批可执行开发范围通过而关闭。

## 14 自动 CI 与本地资格的明确差异

既有 workflow 的两个 ubuntu-latest 作业相互独立；ordinary 失败不阻止 oracle
作业开始 bootstrap。后者下载并核验官方 SQLite 源码，再 configure / make，
执行两个 SQLite identity/version 子进程后才进入完整 oracle 脚本。这两个进程
是相对本地 stage 14 复用已安装 oracle 的额外步骤，不是本次新增的 CI 行为。

check.sh 的 ordinary/race 没有显式 -count=1；setup-go 的 cache:false 不禁用
Go 成功测试结果缓存。这里只披露可缓存条件，没有证据证明新 CI 已发生 cached
命中。oracle 脚本自身保留 -count=1。GOFLAGS、全局 Go 设置以及 shell/下载器/
configure/make/编译器等 bootstrap 环境仍可继承，不能把本地 env-i、离线缓存、
固定资源门和明确 flags 说成已覆盖远端。五键清洁环境只约束 SQLite child。

CI 没有本地五目标 crossbuild 矩阵、显式 mod verify、whole-module/shipping
两份依赖输入清单或两名 LIMIT oracle race；两名 LIMIT 只在完整 oracle 中普通
运行。新 commit 的实际 CI、有效 flags、是否缓存及具名运行范围仍待各自观测，
不能由本地 52b PASS 推断；这里不改变 workflow、脚本或其他执行边界。

明确依据为 `candidate-209ba-ci-scope-review-v1/REPORT.md`，SHA-256
`7e431c97410454e91bf522a5d5127b7ab4b1b8718db966f783b4151b66d9eebd`，以及
`candidate-52b03f-ci-scope-addendum-v1/REPORT.md`，SHA-256
`380bec04ba9000bafb2817721edd91b07b12ad9d4bb36d9229015e15eaa74d36`。
前者的 209ba FAIL 和后者 12:07 UTC 的 RUNNING 是各自历史快照；52b 后来的
本地终态另见第 13 节。两份报告已保真读回于 Library
`libfile_6c67e55899088191969012856a770b0d`，10,043,357 字节，SHA-256
`79c37b1301c0891937ab0f86f7bcdd587c52c5bf8b51012ddb639d9858b193b5`。
