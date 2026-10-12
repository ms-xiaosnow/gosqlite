# 第四格短写错误源码与恢复记录

本开发检查点增加固定 `index-prefix-64-error`，不改变默认生产 Commit、VFS 或恢复协议。六个 Go 文件及中文手册的受测树为 `bf7fcb34e27885d09ae7f2fde3ec23c1315b2a5f`，基于 `50b85ed048f56eee698a62d745b66d2e4b3773b9`。旧枚举、旧测试根和三个原有 case 保留；新增三个模型测试根及十三个语义负例。

## 先前实际结果

2026-10-12 18:47–18:52 UTC，已实际完成固定范围：

- gofmt 对六文件只读检查 PASS，文件未改写。
- rollbackfault 与 testscope 两包 normal、race 各 1,546 个命名测试 PASS，分别为 113 根和 1,433 子项，零 FAIL／SKIP。
- 整仓源码 guard PASS。
- Linux 双 manual tag 的 sqlite 映像编译和只读符号检查 PASS：454 required、18 absent；映像未执行。
- 会话 13980 自然退出 0，租约释放。后五条命令复用已完成的一次格式检查，没有全仓或完整 oracle 重测。

这些结果来自当时已返回的工具结果和已报告的统计。**原始运行日志未能完成持久化**：9,480,119 字节的本地归档只完成构包，随后额度暂停；恢复时原目录已不可见。没有 Library ID，不能将它写成已保存归档。本页不是补造的原始日志，相关缺失保留在 `historical-result.json` 中。不会因环境变化把实际 PASS 改成未运行，也不会把历史回报当作可重新核验的完整事件流。

## 同码恢复

20:35–20:36 UTC，从已验证的 50b85ed Git bundle 和七文件源码 delta 恢复一个固定工作树。逐一核对全部 3,344 文件的长度、SHA-256、Git blob 和 Git mode，结果与保存的原 manifest 完全相同；`git write-tree` 精确等于 bf7fcb34。没有重新执行这些测试。Git checkout 的 POSIX 元数据不声称与旧目录完全相同。

- 基线 bundle：Library `libfile_adb84b2572c8819191c764a1c6ece114`，7,048,672 字节，SHA-256 `6ecc790b8d849721d5e015b79d8811e07be74032cb62de5b339348d8d45cc39b`。
- 七文件 delta：Library `libfile_86ebfd2087e08191b026225114daf0c2`，390,157 字节，SHA-256 `4186af94eda9a978ca569ab2241a611a927dac87227d9d47daf33dca6cd5cd2f`。其 SOURCE_ONLY／NOT_TESTED 标签属于测试前保存时点，保持原样；本页单独记录后续实际验证。
- 基线 CI：run 38291398196，head 50b85ed，两作业 SUCCESS；原日志已独立持久化。该旧 CI 不冒充本次新增源码的 CI。

后续三个 pre-DB fail-before caller 只有未完成且未持久化的中间修改，未包含在本检查点。实际故障注入、恢复、退出重启和物理持久性仍 NOT_RUN，整体 M10 尚未验收。[四格手工入口与步骤](../../testing/rollback-fault-sql-manual-zh.md)保留环境、隔离副本、断言和原件留存要求。
