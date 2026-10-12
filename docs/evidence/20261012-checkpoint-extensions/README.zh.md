# R3/R4 checkpoint 与扩展索引设施：有限代码闭合

基线：b06606020fdbb5b73dd0e9b000773f3f02c6979d；最终受测源树：ffc73262e0314f46827b7f4cee1cdac766dbd889。30 个源／测试／手册路径；此后本批变化只为证据与状态文档。

## 真实实现

共用一个十格 controller，覆盖 plain exclusive、plain shared、encrypted WAL layout2 三类 checkpoint authority。七个观察调用默认为空，不更改原调用顺序。315 行组合 fixture 包含 7 grams、180 HNSW、128 IVF；已提交账本 C、完整行／posting／graph／membership、实际索引查询路径和 Check 由独立角色核验。checkpoint 只改变表示，不允许回退到旧 SQL 账本。

源码审查修正了密文 extent/Epoch 关联、明确留存配额和 encrypted writer 在 arm 前的 credential 退休三项实际缺口。格式/pin 同步独立复核；R2/R5 新逻辑逐 hunk 保留。真实十格中断、退出、重开恢复没有执行，不能把下列源码／模型／编译结果当成物理资格。

## 本地执行与复用

- 首轮源树 4ebb3108d4add7122cad73d583eccd01573827c4：format 和三组 go-list PASS；纯模型包编译 FAIL，原因是一个 unused bytes import，未进入测试。原 stdout/STOP 完整保留。
- 最小修正仅两文件：去除 unused import，并同步模型 guard 的精确 pin、完整快照与 import inventory。行为、tag、文件集合、测试根集合不变，独立复核 CLEAR。
- 23:28:13 UTC 续验 PASS：12 纯值根／12 命名项；38 AST 根／120 命名项；3 个健康默认根／9 命名项；整仓 sourceguard；Linux manual 映像编译与 nm。原三组选择证据按未变选择闭包明确复用，不称新树重跑。
- 23:29:24 UTC：仅这三个健康默认根补 race，3 根／9 命名 PASS，覆盖七个新增默认观察点所在路径。
- 23:30:52 UTC：补运行实际读取新增 pager 文件的 TestMetadataOwnershipCallChainSource，一根／一命名 PASS。它是源读取 AST 检查，不是磁盘实验。
- 所有实际运行测试零 FAIL／SKIP；只编译映像从未执行。此前编译 FAIL 仍是历史真实 FAIL，不被成功覆盖。未重跑无关普通套件／oracle／campaign。

[完整命令、工具、原始失败/通过记录和复用映射](validation/README-zh.md)随本批保存。大编译映像和完整归档留在独立 Library，仓库保存索引与哈希。固定工具为 Go1.27.2；健康 race 使用原固定 GCC；没有启动外部 SQLite 进程。

## 持久化与 CI

前批 b066 已远端读回，Library bundle 为 libfile_9bd8c01766e08191a349b3605a63c9ac；11,107,623 字节，SHA-256 bdacbef29dfe3daa4c678e3ffbaa1ac8f38b740ad6ea77daebe4c25a380fc9ce。裸恢复核对 3,868 路径／Git mode／blob 字节及 fsck PASS，无 Go 重跑。其 CI run38321827127（ordinary115051447164／oracle115051446927）截至本页编写仍运行，不能预填 SUCCESS。

本批保存会自然触发现有两 Ubuntu 工作流；默认源只增加空观察点、无 descriptor 模型及 AST，全部 manual native 仍由 tag/env 排除。工作流/权限未改，实际终态另据精确提交记录。

## 有限结束点与剩余

R1–R6 六个锁定有限设施代码及受影响验证至此闭合。它们不是整个项目的完成百分比。已知受阻 ANN 报告修正仍是未核验闭合的实际代码/报告项；真实受限物理实验与外部资格仍待完成。整体未验收，M10 未通过。

[当前六项与三类剩余清单](../../testing/finite-acceptance-summary-20261012-zh.md)；[checkpoint/索引中文手册](../../testing/checkpoint-restart-manual-zh.md)。手册中的 source-only/NOT_RUN 是冻结源稿编写时点；source/model/guard/compile 的新状态由本页日期结果覆盖，native NOT_RUN 不变。
