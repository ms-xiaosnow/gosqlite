# Namespace 门控：九路径中文文档组合交接

## 结果

PASS：九个最小文档路径已完成，严格基于提交 `56bbcf3febe233197aa0a5eac965c67a67a88011`、tree `c31e7dcbeed9732688681741ee38ec60ff0bfb49` 的 Git archive，在本目录独立副本内工作。源码、CI、goal、既有 P0/LIMIT 证据及 checkpoints 未编辑。`progress.md` 的原始完整字节是新文件前缀。

文档补丁：`docs.patch`，SHA-256 `7c88ae64dbb09a1a45ef5660f1403d92a269b606c999436a2c651056a256321d`。

- 仅文档候选：`tree/`，tree `982244fa28e4c27ba8dccf6ed368685d900b89b9`。
- 完整文档补丁重放：`replay/`，3036 文件、144 子目录；文件字节/大小/路径/Git 模式/实际 POSIX 模式与候选一致。没有新增文件或改变目录权限。
- 148 个相对文档链接/锚点通过。新增源链接按接受的 `namespace-gating-integration-freeze-v1` 解析；新证据链接按主任务已经保存的 25 个证据文件解析。
- 45 个 Bash/sh fence 经 `bash -n` 通过；新增 Python heredoc 仅 `ast.parse`。没有执行任何 fence 内容。
- 真实测试名、包、平台文件名、精确首行 tag、首句 env gate、原断言、既有编译 argv 和 Go 1.27.2 testing flags 已静态交叉核对。
- 没有运行 Go、测试、构建、数据库、native/manual binary、CI 或受限链路；manual/native 一律 BLOCKED / NOT_RUN。

此候选 tree 是“固定基线加九文档”的身份，不是“六源码加新证据加文档”的最终组合身份。不要把它写作运行树或新提交身份。

## 精确归属路径

1. `docs/manual-verification-zh.md`
2. `docs/manual-verification.md`
3. `docs/progress.md`
4. `docs/acceptance-status.md`
5. `docs/restriction-provenance-20261011.md`
6. `docs/compatibility.md`
7. `docs/closeout-scope-20261011.md`
8. `docs/test-scope-audit.md`
9. `docs/rollback-recovery-open.md`（主任务追加授权的最小历史/当前范围交叉说明）

`owned-paths.txt` 为可机器读取的清单。`base-blobs.json` 和 `owned-inventory.json` 对每一归属文件记录精确旧 blob、旧 SHA-256、字节/模式及新身份。补丁包含完整 index blob 行；应用前应匹配基线 blob，不能覆盖并行文档编辑。

## 文字更新要点

中文主手册新增第 19 节，旧 §12.3 改为历史事实与当前入口指引；固定 776b 缺口表明确保留历史身份。当前矩阵/限制分类/closeout 的旧“尚未门控”行精确修正。P0 专题只追加 dated cross-reference，原 source/model 命令、历史范围和所有证据保留。

本批恰好移动两个 encstore 测试，分别为 Windows retained-root 和 Darwin parent-lifetime；它们不同于旧 M2 两个 plaintext namespace 测试。完整 body 只增加首句环境门，默认/oracle 在适用平台有意减少恰这两个声明，其余声明保持。原 P0 recovery guard 保留，generic selector 只追加 Rename/rename。保守匹配、误报、bare helper/wrapper/reflection 等局限明确披露。

主手册提供用户未来独立授权原生环境的固定 Go/离线依赖、Windows Git Bash 与 macOS 路径准备、固定已审阅新提交导出、源码清单、仅编译/符号读取和精确单函数执行、原断言、失败诊断、日志与清理。Windows `lockFile(false)` 已根据实现明确为独占 lifecycle 锁；t.TempDir 自动清理和原 Close 断言不足等局限未隐藏。没有 `! grep` 否定列表，关键失败使用显式停止；Windows cygpath 先赋值并检查退出，再 export，避免 export 掩盖路径转换失败。

sentinel 始终明确为防误执行措施，不是许可。新 OS runner、默认 OS aggregate、故障/恢复/错密钥/篡改/中断发布、ANN V4、41-target fuzz 与未知链路不因这些步骤放行。

## 证据身份和时点

两方检查的源码仍是 `01918609a55cc149c5155451da40f7389854db06`，基础为 P0 运行冻结 `442a7104dc780ddb20826e5ee4f1250b291f2b24`。owner 08:23:50 UTC 与独立追加 08:27:31 UTC 的 guard/model/race、188 实际选择和六项 compile/symbol PASS 分开记录。原 independent source-only 报告没有被改写；追加报告只说明后来获准且实际执行的范围。没有把六项编译写成 native PASS，或把模型写成全量 aggregate。

文档已依主任务最新核验记录更新 P0 父基点：bundle `libfile_04182e9da3f88191b348c6790628a7bd` 字节读回核验；精确恢复于 08:43:14 UTC 通过 21 项命令、11 项具名测试各普通/race。CI `38125268420` 的 08:44 UTC 状态仍进行中，没有 SUCCESS 声称。本批 namespace 新组合、提交、恢复及 CI 必须取得自己的身份和终态；不会继承 P0 或候选 PASS。

主任务已保存证据的四个直接链接在第 19 节：owner-report、independent-source-review、independent-checks-addendum、evidence-files 清单。原始完整外置工作目录/二进制未假定包含在 checkout。

## 静态核对工件

- `static-summary.json`：最终静态结果、数量、补丁/树身份和 NOT_RUN 边界。
- `base-inventory.json` / `candidate-inventory.json`：全部3036文件与144目录的内容和模式清单。
- `base-blobs.json` / `owned-inventory.json`：九路径基线及新 blob/hash/权限。
- `patch-replay.json`：完整精确补丁重放。
- `doc-links.json`：148个本地链接及解析来源。
- `shell-fence-syntax.json`：45个 shell fence 语法结果；不是命令执行结果。
- `source-command-crosscheck.json`：真实函数/tag/env/平台/flags/原断言/已记录编译 argv。
- `input-baseline-docs.json` / `inputs-read.json`：完整读入的文档和报告身份。
- `compose.py`、`refine.py`、`static_checks.py`：本次文档工作及静态检查代码，均不属于项目补丁；不要当作测试 runner。

## 建议下一检查点简述

“保留当前 P0/LIMIT 成果，完成 Windows/Darwin 两个 raw namespace 回归的双门隔离和 Rename/rename 源码防误执行检查，补齐中文独立原生操作规程。现有 source/model/race、选择及编译证据有限 PASS；全部原生物理验收仍未运行。”

主任务应在最终六源码 + 九文档 + 证据组合完成后独立审阅并取得所获准的普通门、真实提交、保存、恢复和 CI 证据。此处没有写 docs/checkpoints 自记录、没有编造自指 SHA，也没有完整验收结论。
