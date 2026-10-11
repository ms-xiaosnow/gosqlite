# P0 公开 rollback Open：源码与默认关闭普通门证据

本批是实验性有限实现，整体未验收。真实恢复、production bridge、开启恢复选项的真实 Open/Connect 均 BLOCKED / NOT_RUN；public physical harness 仍为 IMPLEMENTATION_MISSING。不能把下面的默认关闭普通门或编译/模型结果当作物理恢复验收。

## 源码身份及范围

- 已保存父基点：`c1502790c4b233757b0f9a3c4d5e5bdee1321310`，tree `5a98da017584fe708e43bce9f9e52dca1f06bd20`。
- 独立源码候选：`75c074548e8b5208beb06e6ead314d835cd16f4f`，来自63c2但只逐hunk移入本次13个Go路径；旧候选整树未覆盖当前代码。
- 本批执行冻结 tree：`442a7104dc780ddb20826e5ee4f1250b291f2b24`，3,004文件、143目录、785非docs文件。
- nil-default公开选项、Linux可写明文模式约束、配置值复制、已退休reader的准确路径proof、一次恢复加一次fresh no-create probe，以及防误执行guard。内部物理恢复实现只改说明注释。
- 保留父基点LIMIT测试/实现与所有未涉及源码。362个原测试文件中360个不变；仅一个AST预期defer名字更新、一个guard测试新增断言；新增4个source/model测试文件。
- 默认拒绝、ErrRecoveryRequired匹配和CodeIOErr保留；journal错误文本及私有包装形状有变化，不承诺错误字节相同。

[逐hunk身份](public-open-integration-20261011/source-integration.json)、[候选原审阅](public-open-integration-20261011/independent-review.md)、[源码/模型行为审查](public-open-integration-20261011/owner-behavioral-audit.md)。

## 新检查实际结果

候选的11项精确模型/源码测试在CGO0及相同CGO1 race选择均PASS；guard、限定vet、全包CGO0构建及空CgoFiles闭包PASS。没有开启实际恢复。

最终组合的完整普通门于2026-10-11 07:59:40–08:22:39 UTC自然完成，18阶段全部exit0：

- guard、完整CGO0普通测试（37有测试包，11无测试包）、vet、trimpath build
- 未修改的完整SQLite3.54.0普通oracle脚本（38有测试包，11无测试包）
- 第二次guard与完整CGO1 race（37有测试包，11无测试包）
- Linux/arm64、Darwin/arm64与amd64、Windows/amd64与arm64的CGO0生产编译
- 模块完整性；全部冻结/执行副本的字节、Git模式、实际POSIX模式与目录清单前后一致

[完整报告](public-open-integration-20261011/aggregate-report.md)、[机器结果](public-open-integration-20261011/aggregate-summary.json)、[各阶段时间/退出码](public-open-integration-20261011/aggregate-stages.tsv)、[精确命令](public-open-integration-20261011/aggregate-command.sh)、[最终身份](public-open-integration-20261011/aggregate-identity-after.json)。报告SHA256 `3a7c48ef657ababdf2441a60c049be5fcda857b954959884e47e27b6a88e7abc`。

这套普通门中的真实数据库操作均沿既有默认关闭恢复的路径运行；新增测试只做内存模型/源码读取。AST/source读取会打开源码文件，但不获取数据库、WAL或journal资源。执行边界已审阅，guard不是完整调用图证明。真实恢复与开启选项的真实Open从未因普通门而执行。

环境固定Go1.27.2、SQLite3.54.0、Linux/amd64、离线依赖，GOMAXPROCS=2、p=1、默认/tmp及原缓存。只在race启用CGO；生产CGO0。原生外国OS运行、真实故障和持久性未获该结果。tmpfs和fsync=volatile的overlay不能证明断电安全。

## 保留的准备阶段诊断

首次外部验证器预检发现审阅报告哈希变化，在任何Go执行前停止。准确还原被记录版本得到14,314字节/94113361…；最终报告14,299字节/e507af6e…，差异仅环境说明中重复短语及换行，源码/测试/判断和运行结果未变。此前14,302字节属于更早版本，不能与941哈希混用。旧字节明确标为按精确最后补丁重建，不伪装为原先留存。[完整更正](public-open-integration-20261011/review-report-finalization.json)、[差异](public-open-integration-20261011/review-report-finalization.patch)、[原预检记录](public-open-integration-20261011/aggregate-review-pin-preflight.json)。

第二个外部预检把Git基线归一化权限与工作冻结POSIX权限强行视为相同，也在任何Go执行前停止。记录30文件/2目录差异后，仅调整验证器比较关系：基线按完整字节/Git模式；冻结/审查/执行副本仍严格比较全部实际POSIX模式及目录，未chmod、未删弱源码检查。[诊断和修正](public-open-integration-20261011/aggregate-posix-checker-correction.json)、[权限差异](public-open-integration-20261011/aggregate-baseline-posix-delta.json)。

无运行阶段失败重试或源码修改。137项清单内完整普通门证据及清单自身的哈希校验另行保存；本目录为可直接审阅的精选原始证据，不改写原失败。

## 持久化与后续资格

本文件写作时本批新提交、Library恢复执行与CI尚待产生，不自指预写SHA。最终文档/证据会使整个Git tree不同；发布前须校验785非docs文件仍等于上述执行树。

父基点c150的准确bundle、恢复15阶段和CI38122662808 SUCCESS见[父基点持久证据](public-open-integration-20261011/prior-limit-checkpoint-persistence.json)，不替代本批新身份。

[专题API与边界](../rollback-recovery-open.md)、[中文手册第18节](../manual-verification-zh.md)明确真实API、预算、环境、隔离副本、实际source/model命令和未来断言。缺公开物理harness时不能编造完整可运行验收命令。更广journal、默认自动恢复、其他原生平台、WAL-tail publisher、retained-directory、backup、ANN V4和受阻41fuzz等各自状态不变。
