# 最终有限收尾手册：独立静态审阅

审阅日期：2026-10-11 UTC。

## 结论先行

- 五例目录终结规程：STATIC SCOPED PASS。未发现需要修改运行源码、测试选择、失败处理或清理边界的阻断项。
- 三项有限收尾文字提案：范围和证据分层正确。发现一处最终整合必须修正的旧基点称谓，负责人已明确接受修正；该修正须落入最终文档，不能按草稿逐字保留旧“最新已保存源码基点”。
- main 非 docs 源码身份：PASS。04:17:48.211151–04:17:50.210186 UTC，main 与 final-closeout-combined-freeze-v1 的全部 777 个非 docs 文件、非 docs 目录清单、文件字节、大小、原权限和 Git 可执行位完全相同。第二次读取未发现比较期间非 docs 漂移。全部 11 处差异均在 docs/。
- 原始 goal.md 两端 SHA256 均为 `9ae7b1d776856669a10eb753408c7754d015d266f8783240165450e96e4bac81`。
- 本报告没有运行项目代码、测试、构建、手工命令块、数据库流程或远端动作，没有 Git 写入，没有修改源码或草稿。只执行了静态读取、哈希、清单比较和本报告目录写入。没有新增真实失败留存证据或完整验收结论。

在现有限制下完成了独立可推进项，整体未验收。

## 固定输入

审阅的两份原始草稿保持以下身份；之后的状态修改应另记最终 hash：

- `final-closeout-docs-draft-v1/example-retention-verification-zh.md`：`811705c9910857c2e6570290b3217ffcdcaa5f539af4048005772b524f7be06f`。
- `final-closeout-docs-draft-v1/finite-closeout-updates-zh.md`：`3a816d7829880dbec710bef38931ee2fe907863b684898ae7f6cf0b3da04196d`。

已读取三个 fixturecleanup 文件、五个示例 run、两个原有正常流程测试、go.mod/go.sum、原独立 report/命令/环境/选中测试输出、中文 CLI 资源规程、主手册相关范围和人工门、当前 closeout/acceptance 文档及组合 manifest。main 八个留存文件、两个原有测试和模块文件与 example-retention-freeze-v1 逐字相同。main 内留存独立报告副本与原报告逐字相同。

## 唯一需要最终整合处理的文案

位置：`finite-closeout-updates-zh.md:33` 要求“保留开头基点”；当前 `gosqlite/docs/acceptance-status.md:3` 称 1352ae5 为“最新已保存源码基点”。但第一阶段已经保存为 7a9972547f49531df04cf1b624dfa662b483907b，因此按原句保留“最新”会造成身份误导。

已通知负责人；负责人回复将改成：

“本次修改的已保存父基点7a9972547f49531df04cf1b624dfa662b483907b；最终保存提交和当前CI发布时仍待按精确commit确认”，同时保留 1352 的历史证据。最终提交 SHA 不预先自指，最终读回/CI 终态另行持久报告。

上述方案正确区分父基点和未产生的最终身份。此报告只确认该方案足以修正问题，不称它已经落盘。最终整合还应让新增 guide 的三处相对链接实际落在 `docs/example-retention-verification-zh.md`，并保持组合门、恢复、CI 的 PENDING/实际终态分别有准确证据。

## 测试、包和参数

精确五个真实顶层入口分别为：

1. `examples/internal/fixturecleanup/cleanup_test.go:10`：TestFinalizeCallbackPolicy，六个回调子场景。
2. 同文件第 61 行：TestFinalizeDelayedResult，延迟回调结果。
3. `examples/internal/fixturecleanup/source_test.go:17`：TestExampleFixtureFinalizersSource，五文件 AST。
4. `examples/encrypted-wal/main_test.go:11`：TestEncryptedWALExample。
5. `examples/encrypted-shared/main_test.go:9`：TestEncryptedSharedExampleOrdinary。

锚定 selector 与现有独立执行记录相同。前三个属于 helper 包，后两个属于 WAL/shared 两包；单独 sourceguard 是同一个测试而不是第六项。限定 vet/build 恰为 helper 和五个示例六包。没有 broad `./...`、manual tag、fuzz、benchmark 或新增 failure workflow。

Go 1.27.2、crypto v0.58.0、sys v0.49.0、text v0.43.0 与 go.mod 完全一致。静态读取本地固定 Go 工具链的 test/build 参数说明，确认 count/run/verbose/race/timeout=0/buildvcs 参数和多包 build 不留下示例可执行文件的行为。没有调用 Go 工具。主机与目标均限 Linux/amd64，普通 no-CGO、race 才启用 CGO，并要求可用 C 工具链。

显式 timeout=0、独立会话缓存、前后模块校验都是未来规程条件，文档明确不把它们追认到旧 scoped argv。原独立环境记录的实际缓存为 durable-restart/go-path 和 go-cache；新规程明确记录新的独立 GOPATH/GOCACHE，未宣称运行环境完全相同。

## 断言与能力陈述

- Finalize 两项纯回调测试没有创建文件、句柄或凭据。仅检查完成标记、删除次数、延迟结果、errors.Is 原链及诊断。删除回调错误由 sentinel 模拟，不是物理 RemoveAll 失败。
- 五文件 AST 检查成功 MkdirTemp 后登记 finalizer、晚于其他 defer 执行、读取最终具名结果、仅最后成功返回前设置 completed、原 Close 合并、输出错误返回。它不调用示例 main/run。
- WAL 保持正常正确密钥流程：layout 2、三次提交的三个 segment、正常重开 metadata、checkpoint 清零选定 segment/frame/bytes、正确只读重开。原测试核对四行输出及 t.TempDir 内没有示例目录；它的自动清理不能证明失败目录保留。
- shared 保持正常快照/提交、原 stale-snapshot BUSY、主动 rollback、checkpoint 后 layout 3 和零 segment。四个原输出短语被核对；没有直接的真实失败目录断言，也没有独立的正常删除目录文件断言。
- snapshot、encrypted-cli、native-check 仅有 AST 与编译/vet 覆盖；规程没有执行它们的 main/run 或 CLI 数据库子进程。
- 仍清除进程自有密钥、剩余密文可能无法解密、RemoveAll 可能部分完成、普通删除/clear 不是安全擦除、不覆盖 panic/crash/process-exit 均已明确。未承诺完整、可解密或可恢复的现场。

## 隔离、日志、退出码、身份和清理

- 从操作者已授权且干净的本地提交 `git archive` 到新 mktemp 副本；记录完整 commit/tree/archive SHA，工具链、host/target 和模块输入身份。八文件存在性仅防明显选错版本，文档明示仍须审阅运行源码身份。
- 源码、证据、缓存分开；不记录整个环境或密钥；offline/local-toolchain 设置禁止自动下载。缺少模块/工具链/空间记 BLOCKED；断言失败记 FAIL。
- capture 先独占目录、argv/cwd/start、stdout/stderr，再启动命令。结束后写 returncode/finish/exit；没有命令到 tee 的失真管道，没有覆盖或自动重试。日志未完整落盘不能宣称 PASS。
- 记录器及 EXIT 身份处理与现有 CLI 规程一致。失败退出码不会因身份比较成功而变成 0；成功命令后身份漂移也阻止整体通过。信号的原负 returncode 保存在 result/exit 文件，shell 状态映射明确。
- 身份清单覆盖相对路径、类型、字节、大小、原权限和 Git 可执行位；普通后、race 前与终结检查存在。源码外日志不污染源码身份。
- 未提供宽泛 rm、自动失败清理、sidecar 删除、故障制造或强制终止命令。成功后仍保留 evidence/source.tar；操作者确认进程自然退出、本次目录身份和备份可读后自行依保留策略清理。
- 对预备工具启动失败、结果落盘失败等情况只能保留不完整证据并停止；本报告不声称记录器被动态验证过。

## 相对链接与剩余边界

以最终 `gosqlite/docs` 为基准核对 13 个链接，现有目标和三个主手册锚点均存在；新增 guide 的三处引用明确是计划位置。详细结果见 relative-links.json。

三项完成只指有限修正及各自 source/scoped/static 审阅。最终 d739 组合门、最终提交恢复、current CI 分开；草稿继续 PENDING 正确，第一阶段 7a997 或 1352 的历史恢复/CI 不可替代最终身份。文档变更后的整体 tree 不可冒称原 d739 运行日志身份。

三类后续事项分得清楚：已有 manual/测量/普通入口而未运行；首先需要真实代码/harness 和审阅；需要真实外部环境或资格审查。public recovery、WAL-tail、planner、backup、backend、ANN V4、41 fuzz、未知中断旧链均保持 HOLD/UNKNOWN/BLOCKED；ANN V3 三个 FAIL 和其他历史 FAIL 原文保留。没有 M10 PASS、全部功能已完成或只剩人工测试的断言。

## 独立源码身份附件

- source-identity-comparison.json：比较时间、数量、所有差异、goal hash、非 docs 漂移结果。
- main-source-inventory.json / combined-freeze-inventory.json：完整文件/目录清单和 hash/mode，排除根目录 .git 管理数据。
- relative-links.json：相对路径与锚点检查。
- reviewed-inputs.sha256：审阅输入字节身份。
- SHA256SUMS：本报告及上述附件身份。

main 共 2,944 个常规文件、142 个目录；组合冻结副本共 2,935 个常规文件、141 个目录。差异仅为 docs/progress.md 和新增 docs/evidence 内容，没有 README、源码、测试、模块、脚本或测试数据差异。没有 symlink/特殊对象差异。

一次用于显示本地 Go 参数说明的 grep|head 读取输出产生了 Broken pipe（下游 head 已停止读取）；它不是项目命令或验证门，没有重跑，没有据此改变任何运行结论。

## 整合后追加复核（2026-10-11 04:23 UTC）

结果：INTEGRATED STATIC SCOPED PASS，无剩余整合阻断项。前述旧基点称谓问题已确认修正落盘。

- `acceptance-status.md` 现明确 7a9972547f49531df04cf1b624dfa662b483907b 是本次修改的已保存父基点；最终保存提交/current CI 仍按精确 commit 待确认，1352 只保留历史身份。未预写最终自指 SHA。
- 新 `docs/example-retention-verification-zh.md` 与本报告审阅草稿逐字相同，SHA256 仍为 811705c9910857c2e6570290b3217ffcdcaa5f539af4048005772b524f7be06f。
- 对冻结组合的静态 diff 证明：acceptance 只更新开头、五例限定修正行和末尾有限收尾；所有真实代码缺口、人工/外部门、历史失败正文原样保留。closeout 第二至四节原样保留，末尾新增三类下一步符合已审阅提案；progress 只增加历史语境说明和新状态段，未删改历史结果。主手册只增加新规程链接及限制句，旧命令、锚点和受限门正文不变。
- 五份整合文档共 55 处仓库相对链接静态核对，目标及片段锚点全部存在；新增 guide 引用已落在真实文件，不再只是计划路径。
- 组合完整普通门、最终新提交恢复和 current CI 均仍为 PENDING。没有把负责人已通报的第一阶段 7a997 恢复 PASS 或 CI 38110617335 的部分 job 成功追认为最终提交/组合完成；本次未访问 CI 或执行恢复。
- 04:23:03.208846–04:23:03.836681 UTC 再次核对：全部 777 个非 docs 文件及非 docs 目录与固定组合完全相同，字节/大小/权限/Git 模式相同；也与上一轮身份记录相同，读取期间无漂移。18 处差异全部位于 docs/。goal SHA 保持原值。
- 所有三项完成结论仍限 finite source/scoped/static 工作；明确保留未实现设施、全部旧 HOLD/FAIL、真实失败目录未验证，以及“在现有限制下完成了独立可推进项，整体未验收。”没有新增功能或解除旧链。

本追加只做读取、静态 diff/hash/link 检查及报告目录写入，没有执行项目测试、构建、文档流程、数据库动作或远端/Git 写入。新附件：integrated-source-identity.json、integrated-main-source-inventory.json、integration-doc-diff.patch、integrated-relative-links.json、integrated-doc-inputs.sha256。它们记录本次读取时身份，后续仅文档/状态更新应另记新 hash，不能替换本次快照身份。
