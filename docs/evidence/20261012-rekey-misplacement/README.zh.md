# R2 与 R5：有限设施代码及受影响验证闭合

基线提交：01c63e6f8fb3d9e196afe2f42a163eb3eac4ad5a。组合受测源树：cdcce9e3145d7811b0448b4f02d21e71dbe32345。此后的本批增量仅为证据和状态文档，31 个生产／测试／手册路径按该树逐字节保存。

## 实现与真实执行

- R2：一个 index-misplace-minus64-error case，实际手工调用设计固定 8192→8128、完整 4096 字节写及独立 OLD journal／恢复账本。原九格不变，默认生产路径不改。手工入口没有运行。
- R5：六格换钥中断 source-only controller、writer、独立 old/new/archive probe；三个默认空观察点，旧 publishRoot 调用顺序不变。Linux 健康正确密钥默认路径两根已真实运行，六格中断／错误密钥／退出没有运行。
- R5 最后 11 门于 22:53:31 UTC PASS：7 AST 根和 8 纯模型根 normal/race、sourceguard、三组选择、一个 Linux 手工映像编译/nm。54 个旧 AST 根 normal/race 及两个健康默认根复用前轮适用 PASS。
- R2 最后 13 门于 23:04:54 UTC PASS：两失败根精确 normal 重验 235 命名 PASS，91 根 race 共 2418 命名 PASS（54 模型 1569、13 旧协议 157、24 AST 692）；sourceguard、三组选择、三个手工映像编译/nm。89 个未受影响 normal 根复用首轮 PASS。所有本轮 test 结果零 SKIP；编译映像没有执行。
- R2 符号集合分别 452 / 156 / 455 根，与 go-list 所选 Test/Fuzz/Example 集合逐项一致。
- 原真实失败均保留：R5 两轮 guard 作用域／新模型标识符碰撞；R2 旧 case 分类遗漏第十格及三个 mutation 错命中新增加的解析调用。仅修对应最小源码／测试，未改错误预期隐藏差异。

## 证据与复用

- [R5 原始日志、命令、失败修复与复用](rekey/README-zh.md)
- [R2 原始日志、命令、失败修复与复用](misplacement/README-zh.md)
- [01c 精确 CI 完整 SUCCESS](prior-ci-01c/README.zh.md)：run 38315462277；ordinary 115030907689、oracle 115030907357。
- [edcc 精确 CI 完整 SUCCESS](prior-ci-edcc/README.zh.md)：run 38314186578；ordinary 115026774139、oracle 115026774110。

本地没有重跑无关 SQL/oracle 或完整套件。未改的生产／测试依赖继承此前精确提交的适用证据，不能称新树全套新 PASS。GitHub 的两个既有 Ubuntu 工作流会按 push 自然触发，终态另记录；未改 workflow、tag 或运行环境权限。新默认包只增加无 descriptor 纯模型／AST；所有 native 文件有 manual tag 与环境双门，默认 CI 不选择它们。R2 修改只有 manual bridge 与纯模型／guard；R5 默认三个 void observer 是空调用，健康默认路径已检查。

## 剩余与手工

R1、R2、R5、R6 的有限代码与适用普通验证已闭合。最后 R3/R4 共用 checkpoint／扩展索引设施已完成独立源码审查，尚未合入此树或运行有限检查，仍为待完成代码批。已知 ANN V4 报告修正保持单列阻塞，不能当成只差测试。

中文手工规程：[换钥](../../testing/rekey-interruption-manual-zh.md)、[错位与 SQL 完整性](../../testing/rollback-fault-sql-manual-zh.md)、[adapter 合同](../../testing/rollback-fault-adapter-zh.md)。实际物理故障／恢复／退出／错误密钥、其他原生 OS、真实持久介质及独立安全许可资格仍 NOT_RUN／外部待验，整体未验收，M10 未通过。

手册正文是受测源码树中的冻结历史文本，开头的 REVIEW_HOLD／REVIEW_PENDING／FORMAT_NOT_RUN／MODEL_GUARD_COMPILE_NOT_RUN 描述其编写时点。它们的 source/format/model/guard/compile 当前状态由本页 2026-10-12 23:04:54 UTC 结果覆盖；物理 NOT_RUN 不变。保留原字节与历史，不为文档更新使同码证据失配。
