# R5 换钥中断：定向验证证据

## 结论与边界

- PASS：最终候选树 `8751a3bed18341a299c060ff494c437d5176b024` 的 V3 共 11 道定向门禁全部通过，完成于 2026-10-12 22:53:31 UTC。
- PASS：7 个 R5 AST 根测试普通 / race，8 个纯值模型根测试普通 / race；源码守卫；Linux 默认、Linux manual、Windows 默认三组 go list；manual 测试映像只编译及 nm 检查。
- PASS：nm 核对 456 个必需符号及 10 个应缺席符号；映像 SHA-256 为 `af32b10e083dc53fb297737881edfa50b1f66d6db824ab2a56a1ef71ce337173`。
- REUSED PASS：54 个既有 AST 根测试普通结果取自 V1、race 结果取自 V2；2 个健康默认根测试取自 V2。不是最终树上重新执行的完整旧测试套件。
- FAIL（真实历史记录，未改写）：V1 普通守卫 60 个根 PASS / 1 个根 FAIL，786 个命名 PASS；R5 main 收集器对另一个函数的 findings 误套作用域约束。修复和独立复核保留。
- FAIL（真实历史记录，未改写）：V2 sourceguard 返回 7 条保留标识符冲突诊断。新 R5 模型的 SQLPlan / SQLQueryText 分别改名为 RekeyQueryPlan / RekeyQueryText；旧守卫未放宽，修复和独立复核保留。
- NOT_RUN：manual 测试映像从未执行；没有物理六情形中断实验、wrong-key 实际探测或 crash / process-exit 场景。编译、nm、值模型与源码守卫 PASS 均不等于物理耐久性 PASS。
- V1 失败后余下命令、V2 失败后余下命令各自保持 NOT_RUN。后续执行记录属于独立的 V2 / V3，不补写到旧记录中。

## 身份和复用

基线提交 `01c63e6f8fb3d9e196afe2f42a163eb3eac4ad5a`。最终 20 个源文件直接从上述不可变 Git tree 的 blob 导出并逐个验证 SHA-256，未读取可能继续变化的工作区作为最终源。

`SOURCE-IDENTITY.json`、`SOURCE-BLOB-VERIFICATION.json` 和 `REUSE-MAP.json` 给出精确身份及复用映射。旧模型普通 / race PASS 属于旧名称版本；仅标识符替换的字节比较已核实，但本证据以 V3 重新运行的 8 + 8 个根作为当前名称结果。

## 原始资料

完整 ZIP 包含四次验证的原始 stdout / stderr、全部已规划命令与固定环境、退出结果、资源采样、执行前源清单、停止 / 完成记录，早期独立审阅、集成记录、两次修复的 before / diff / 审阅、四个 runner，以及最终 20 文件快照。manual 编译映像只收录一次；请勿将它执行为本记录的一部分。

`RESULTS-SUMMARY.json` 是从原始记录和 JSON 测试事件提取的索引，不替代原始日志。`MANIFEST.json` 对除自身外的所有 ZIP 成员记录长度及 SHA-256；ZIP 自身哈希在外置回执中。

Git-ready 目录保留命令、结果、复核、修复和 gzip 原始日志；大体积源码清单、历史快照和编译映像以 `FULL-ARCHIVE-INDEX.json` 索引到完整 Library ZIP。gzip 解压后的字节哈希与原件一致。Library 身份及逐成员回读核验见 `LIBRARY-ARCHIVE.json`（上传后生成）。

仅保留 runner 明确记录的固定环境。没有添加环境转储、凭据或签名下载链接。集成时的执行基础设施进程包装快照不分发，省略项及原件摘要见 `EXCLUSIONS.json`。本次保存只进行了 Python 文件处理、归档与 Git 对象只读查询，未运行任何测试、Go 工具或编译映像，也未修改源代码、API pin 或 GitHub。
