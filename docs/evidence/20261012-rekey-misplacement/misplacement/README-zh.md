# R2 错位写入：定向验证证据

## 结论与边界

- PASS：最终候选树 `cdcce9e3145d7811b0448b4f02d21e71dbe32345` 的 V2 共 13 道定向门禁全部通过，完成于 2026-10-12 23:04:54 UTC。
- PASS：原先失败的 2 个普通根测试重跑通过（235 个命名 PASS）；原 91 个根测试完整 race 选择通过（2,418 个命名 PASS）。
- REUSED PASS：其余 89 个普通根的通过结果复用 V1。没有在最终树上重新运行完整普通套件，旧记录不改写成新运行。
- PASS：定向格式检查、sourceguard、3 组带标签的 go list 源文件选择、3 次 go test -c 只编译、3 次 nm 精确根符号检查。
- FAIL（保留真实历史）：V1 普通门禁返回 exit 1；2,413 个命名 PASS、5 个命名 FAIL，其中 89 个根 PASS、2 个根 FAIL。失败分别为 case 10 的旧分类预期，以及 3 个既有变异样本的过宽匹配锚点。
- 修复仅涉及 3 文件：补齐 case 10 的测试预期；将 3 个旧变异锚点绑定完整 RecoverRollback 调用；更新对应测试快照 / 摘要。生产代码、原生代码和既有恢复语义判定未改动，独立窄范围复核已保留。
- NOT_RUN：V1 失败后的剩余 11 个门禁没有运行；后续真实执行独立记录为 V2。三个 manual / restart 映像均只编译和检查符号，从未执行；没有物理错位写入或恢复实验。值模型、AST、sourceguard、编译 / nm PASS 不等于物理耐久性 PASS。

## 身份与复用依据

基线提交 `01c63e6f8fb3d9e196afe2f42a163eb3eac4ad5a`。最终 13 个 R2 源文件直接从上述不可变 Git tree 导出，逐个与修复身份文件和 V2 执行前源清单核对 SHA-256。两个完整执行前清单各有 3,475 项，只在审阅的 3 个文件上不同。V2 完成记录确认执行期间源文件未变。

`SOURCE-IDENTITY.json`、`SOURCE-BLOB-VERIFICATION.json` 和 `REUSE-MAP.json` 提供精确路径、摘要、根测试列表和复用理由。正常模式的证据由 V1 通过的 89 个根与 V2 重跑通过的 2 个根组成；race 是 V2 独立的 91 根运行，不把二者伪称为同一次普通运行。

## 编译映像

- fault-sqlite.test：452 个必需符号；SHA-256 `81214cd458936e1089655ae17d2bac384702e24083d3f9d4e17d8a57811b9071`
- fault-pager.test：156 个必需符号；SHA-256 `618fc6b07f18f7ce309b1c74af5b8e8ea51196791cb57f6f63e993e7ff65bb93`
- restart-sqlite.test：455 个必需符号；SHA-256 `48f9345d29fcce0c94a60eee4cc3435c712d8f439d1c5095f225f7e105e09b78`

这 3 个文件只存入完整 Library ZIP；不放入 Git 证据目录，不得将它们执行当作本记录的一部分。

## 原始资料与使用

完整 ZIP 收录两次运行的全部已规划命令、固定环境、stdout / stderr、退出结果、资源采样、停止 / 完成记录、执行前清单；源码提案、早期与格式后独立复核、原 overlay、格式 / 集成证据、三文件修复的 before / diff / 独立审阅、两个 runner、最终 13 文件快照和 3 个未执行映像。历史报告中的“待执行”等状态按当时原文保存，以完成记录确定后续结果。

`RESULTS-SUMMARY.json` 从原始记录与 JSON 测试事件提取索引，不替代原始日志。`MANIFEST.json` 记录除自身外所有 ZIP 成员的长度和 SHA-256；ZIP 自身摘要见外置回执。

Git-ready 目录保留命令、结果、审阅、修复和可逆 gzip 日志。gzip 使用 mtime=0；解压后的字节与原件一致。完整源清单、重复历史快照和 3 个映像通过 `FULL-ARCHIVE-INDEX.json` 指向完整 Library ZIP。Library 精确身份与上传后逐成员回读核验见 `LIBRARY-ARCHIVE.json`。

本次保存只做 Python 文件整理、哈希、归档和只读 Git 对象查询；没有运行新测试、Go 工具、编译器、原生映像或物理场景，没有修改 W 或 GitHub。仅保留 runner 明确记录的固定非秘密环境，不加入凭据、环境转储、签名链接或其他项目资料。
