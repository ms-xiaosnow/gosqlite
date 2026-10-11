# 公开 Open 回滚恢复：手工测试源码检查点

状态：有限四场景 harness 已实现，独立源码审阅及 V3 编译限定验证 PASS；真实物理恢复 **BLOCKED / NOT_RUN**。最终组合资格、远端提交、Library 恢复和 CI 在各自完成前不记成功。本批不增加生产行为，不等于 M3/M10 验收。

## 源码与支持边界

入口为 [`sqlite/rollback_recovery_manual_linux_test.go`](../../sqlite/rollback_recovery_manual_linux_test.go)，10657 字节，SHA256 `dd5a63e2a8e7d4fdd969ba03273a8f8147d5eff81babf8fce44a5228c5cdaf5e`。编译标签 `manual_recovery && linux`，第一语句检查 `GOSQLITE_MANUAL_RECOVERY=independently_authorized`。默认和 oracle 排除文件；该环境值本身不提供运行许可。

四个子场景分别验证 nil 默认拒绝及输入保留、显式 opt-in 后恢复/只读重开、journal 配额拒绝及输入保留、NoCreate 保持不存在。它们使用公开 OpenContext，不直接调用内部 RecoverRollback。独立空隔离目录及全部原始/工作输入始终留存；资源关闭错误可见，无隐式恢复/cleanup 重试。详细可复制人工步骤、环境、预期断言、排错和清理见[中文专册](../public-rollback-manual-zh.md)及[总手册第20节](../manual-verification-zh.md)。

Driver Connector 真实恢复 harness、完整 fault/crash 矩阵、更广 journal/平台支持、默认自动恢复及 native WAL-tail publisher 仍有实现缺口。不能靠运行这四项关闭这些缺口。tmpfs、workspace overlay 的 fsync=volatile 均不能证明物理持久性。

## 独立审阅与原始问题

[V1审阅](public-recovery-harness-20261011/v1-source-review.md)发现三个真实问题：手册否定 grep 在 set -e 下可能 fail-open；fixture 清理只查部分备份且不查 manifest；早期 Fatal/Close 错误可能漏报 pending 或继续后续场景。V2修复并重新审阅；V3将受控 Go 环境置于首个 Go 命令前、将 archive 管道拆成独立 fail-stop 步骤。原报告和两次补丁完整保留，无失败抹除。

[V3审阅](public-recovery-harness-20261011/v3-compiler-review.md)固定树 `1755d67431f9de082449b08fa24b26b2b35eb612`，基于 P0 提交 `56bbcf3febe233197aa0a5eac965c67a67a88011`。08:58:28–08:58:38 UTC 六条 Go 命令：外部标准库 AST 检查、三种文件选择（0/0/1）、Linux amd64 test-c、go tool nm 精确符号一项，全部 PASS。一个 Python 选择文本检查也 PASS。测试二进制从未运行，未以 -run 不匹配或 -test.list 间接启动。

V3原始手册 9711 字节 SHA256 `a49ae1882c4a5dcb8c58860fa8df3b011f88c9ee99963a5721831049477ec4d2` 保存为 [v3-reviewed-manual.md.txt](public-recovery-harness-20261011/v3-reviewed-manual.md.txt)。发布版仅首段状态更新，全部命令、断言和其余正文保持逐字节一致。历史 V3 树与当前组合树分别记录，不声称相同。

## 组合与已有检查点

当前组合基点为 namespace 提交 `50b627e11e9dbce7f2fde8df35c4634c52a06d52`，tree `8f2709293287a3517a52b3f989888ded8230ef53`。新增单一 manual Go 文件，既有生产/测试/CI文件不变；八份现有状态文档逐路径比对基点后组合，progress保留原全文前缀。文档初检45个既有shell块只做 bash -n，不执行命令。

namespace Library bundle `libfile_e5dcbc0ac16c8191bce977aa165163f9` 已读回核验，SHA256 `111d9cc34e9d011c10ab9612010a5c7c3b4841900bc199fc6f1a4e7af30faaec`；恢复207条限定检查于09:14:09 UTC PASS，manual二进制不执行。CI `38127196102` 截至09:26 UTC仍 ordinary运行、oracle成功；这不是终态。

[所选证据清单](public-recovery-harness-20261011/evidence-files.json)固定每个所选原始文件的大小与SHA256。源文件完整性/编译/归档均不能代替实际恢复验收。


## 2026-10-11 09:32 UTC：namespace 父检查点终态闭合

提交 `50b627e11e9dbce7f2fde8df35c4634c52a06d52`、tree `8f2709293287a3517a52b3f989888ded8230ef53` 的 CI `38127196102` 于09:29:33Z自然完成 SUCCESS；ordinary `114433911927`、oracle `114433912045` 均SUCCESS。Library bundle `libfile_e5dcbc0ac16c8191bce977aa165163f9` 已读回并精确恢复，207条限定检查PASS。全部manual/真实namespace仍NOT_RUN。终态证据归档 `libfile_6b7a1c37d8fc8191bed2389e8dbe14dd`，15356765字节，SHA256 `21cd1a73b622123605eb48a5031b0cc38314a04eac8a9b71c72f873ecaa3d701`，Library读回843项/842清单哈希PASS。此前“运行中”只表示当时观测；此终态不属于新增public harness提交，不代替它的保存/恢复/CI。


## 2026-10-11 09:35 UTC：public harness 精确组合限定检查完成

独立组合树 `0af9b064bebfbec9c8270c4bf2f10ec2055fb186`，3104文件/789非docs（原788全部保持，加唯一手工测试源码），于09:32:51完成六条AST/list/编译/符号命令 PASS；196相对链接和48shell块仅静态检查PASS。独立报告SHA256 `ba6e3c4458d8d192b9dd2f231936c61706d9f6b2fdb64249f34ea34d1b71d765`，81项证据清单SHA256 `394e491d4564c594cb7f7e1e304495f95c8b620122c7e7408a14bcae30f6279e`。物理四场景全部BLOCKED/NOT_RUN，二进制从未执行。随后仅追加文档和证据；发布前逐路径核对全部789非docs不变，文档另做最终静态读回。精确新提交、Library恢复和CI在实际完成后另记，不继承父namespace的成功。
