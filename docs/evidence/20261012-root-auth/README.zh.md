# A8 六格加密根 caller：影响验证与复用

基线 main `6d668f80d199b2c49647a3cbc949442d125988ae`；最终受测树 `a6ca3b307531c7479a08add7325de8d996cd7246`。新增六格源码是布局1根文件的 control、wrong-key、root-salt、metadata-ciphertext、metadata-tag、unknown-major。生产数据库路径没有变更。真实磁盘六格全部 **NOT_RUN**。

## 实际验证与复用

会话17741在2026-10-12 21:47:51 UTC自然退出0，八项PASS、源码稳定、无资源越界。准确命令、环境、时刻、单项计数及哈希见 commands.json、runtime-start.json、result.json。

- format：此前11个未变Go文件结果复用；本次只格式检查两处修正guard文件。
- normal：本次受影响AST 10根、0子项、10PASS/0FAIL/0SKIP；模型4根+32子项=36PASS复用17139同字节闭包。
- race：14根+32子项=46PASS/0FAIL/0SKIP。
- sourceguard：整仓只读源码扫描PASS。
- 默认/精确手工tag选择：新文件在默认IgnoredGoFiles，精确tag在XTestGoFiles；它属于sqlite_test外部包。
- Linux amd64 CGO0手工映像：仅test-c，nm核451 required/22 absent；未执行映像。

未变生产、SQL、oracle和外平台闭包沿原适用证据复用，没有本地全仓重新运行。默认CI只选择新增纯模型与源码guard；手工标签没有进入现有工作流。随后自动CI终态须以精确提交单列，不预填成功。

## 原始失败及最小修正

首轮cae458的format与模型36项通过，guard8PASS/1FAIL；唯一FAIL为 TestRollbackRestartWiringPreservesAllPoliciesAndReviewHold。旧readonly-WAL完整接线清单漏列合法的新root-auth hook，判为findings重新绑定。首轮后六项未启动，不改写为历史PASS。

只修readonly_wal_manual.go及其测试：精确注册来源捕获/error-return/append triplet和仓库合同调用的顺序、次数及参数；保留任意重置、逃逸、重复、错序等拒绝。原10行×14种变体保留，新增2行，共168个根内仅在内存构造的源码变体，28个新增断言，不冒充28个新命名测试。整份main仅增加两个已审hook；五个旧pin只更换同一已审main字节哈希，原post-DB逻辑保留。

## 持久记录与限制

Git保存三份无损gzip原始事件流、命令/工具/源码身份、审查及失败修复差异。`gzip -dc *.events.jsonl.gz`可读取日志。大型映像、nm完整输出和资源采样见persistence-index.json所列Library归档。源码恢复核Git/字节完整性，不机械重复已适用PASS。

六格实际wrong-key/篡改/版本反例、故障恢复和设备持久性均未在本平台运行。源守卫与模型不是物理认证通过；PhysicalValidated/Pass仍false。详细用户自测步骤见[中文手册](../../testing/encrypted-root-negative-manual-zh.md)。整体未验收，M10未通过。
