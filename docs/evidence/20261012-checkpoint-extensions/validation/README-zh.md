# R3/R4 checkpoint / 扩展索引：定向验证证据

## 结论与边界

最终受测源码树为 `ffc73262e0314f46827b7f4cee1cdac766dbd889`，基线提交为 `b06606020fdbb5b73dd0e9b000773f3f02c6979d`；相对基线共 30 个源码/说明路径。本档案的最终源码全部从不可变 Git 对象提取，不读取随后可能更新的工作树说明文件。

- 新执行：12 个纯模型根 / 12 个具名 PASS；38 个 AST 源码契约根 / 120 个具名 PASS；3 个现有健康默认数据库根 / 9 个具名 PASS。源码守卫、修复两文件格式检查、manual 编译及 nm 检查均 PASS。V2 于 2026-10-12 23:28:13 UTC 完成。
- 新补充：同 3 个健康默认数据库根的 race 检查 / 9 个具名 PASS，于 23:29:24 UTC 完成；零 FAIL、零 SKIP，源码清单未变。纯模型、AST、健康数据库证据分开统计。
- 另补充 1 个实际受影响的现有 VFS 静态消费者 `TestMetadataOwnershipCallChainSource`，1 根 / 1 个具名 PASS，于 23:30:52 UTC 完成；它扫描新 pager 源码，未触发数据库或 manual native。合计源码契约 39 根 / 121 个具名 PASS。
- 复用：首次执行的 Linux 默认、Linux manual 标签、非 Linux 回退共 3 项 go-list 文件/标签/根选择结果。两文件修复不改变文件名、标签或根集合。旧记录中的完整 import 内容不是最终树的新鲜元数据。未重跑完整仓库测试或无关测试。
- 已修正但保留的原始失败：首次 4 个格式/选择门通过，随后纯模型包因未使用 bytes import 编译失败，exit 1；没有任何 Test 事件，不能写成 12 根测试失败。后续 6 个原定门当次 NOT_RUN。删除该 import，并同步其精确 pin、完整模板及 import 清单后，独立窄审阅 CLEAR，V2 才获得上述新结果。
- 四份独立源码审阅均 CLEAR；审阅的历史 NOT_RUN 声明按原文保存。源码审阅 CLEAR 不等于物理结果。

## 仍未执行

manual checkpoint 中断、进程退出、恢复场景及其物理持久性验证均为 NOT_RUN；oracle campaign、完整套件也为 NOT_RUN。唯一 manual 测试映像只做 test -c 与 nm：454 个要求符号、精确根集合及 3 个 manual 根符号通过；映像从未执行。健康默认测试的真实数据库操作不是 manual 中断/恢复资格证据。不得将本档案表述为物理 PASS。

## 如何核验与使用

`RESULTS-SUMMARY.json` 为结果索引；`COMMANDS.json` 聚合原始命令及固定环境；`RUN-METADATA.json` 聚合每门结果、开始/停止/完成记录；`RESOURCES.json` 聚合资源采样；`REUSE-MAP.json` 解释复用、失败修复和未运行范围；`REVIEWS.md` 保留四份完整审阅。

Git-ready 文件中的 `RAW-RECORDS.tar.gz` 以固定 tar 时间、uid/gid 和 gzip mtime=0 保存原始 stdout/stderr、资源日志、原始 JSON 记录、格式/修复差异及 runner。解包后的每个字节可按 `FULL-ARCHIVE-INDEX.json` 中 SHA-256 核验。Git 不放编译映像、重复源码树或历史 before 源码树。完整 ZIP 另含最终 30 文件快照、修复/格式前后材料、四个执行阶段的全部原始证据及唯一未执行映像。

`MANIFEST.json` 列出完整 ZIP 除自身外所有成员及 SHA-256；`LOCAL-ARCHIVE-VERIFICATION.json` 记录 ZIP 整体摘要；`LIBRARY-ARCHIVE.json` 在外置回执中记录一次上传的精确身份、回读及全部成员核验结果。先前独立保存的 SOURCE_ONLY 档案只在 REUSE-MAP 中引用，不重复上传。

归档过程只使用文件读写、Python 哈希/JSON/压缩、只读 Git 对象读取和 Library 保存/回读。没有运行新 Go、格式化、解析器、编译器、测试、映像或物理场景，没有修改工作树或 GitHub。保留固定非秘密工具/环境记录，排除无关进程清单、凭据、签名链接和其他项目资料。
