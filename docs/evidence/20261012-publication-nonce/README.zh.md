# R1＋R6：有限验收缺口的增量闭合

基线main `edcc8be3e4c0d0dc46066020dc900a5effcaee30`。R1受测树 `598b243e946431c90ed0b5da307f89b288cc19e2`；R6合入后的组合受测树 `8c1eaf0ae116a3c0803ae6b115326a83887a027e`。本检查点只改测试、固定源码pin及文档，生产数据库协议和密码算法没有变更。

## R1：生产状态机的 errno 断言

对应goal §11.1:374的磁盘满/权限变化语义。沿既有WALPublication/BackupPublication fake ops，新增8个有界场景：候选Write/数据Sync/目录Sync的ENOSPC，以及rename的EACCES或EPERM。核对原原因、uncertain/visible/durable区分、无重放/虚假成功、精确一次资源退休。未真实填满磁盘或改变权限。

独立复核确认原13模型根断言保留；发现最初选择遗漏一个共享fake helper消费者，执行前补入TestBackupDestinationModelAdmissionBlocksMutation，未丢弃此发现。最终20个VFS根＋11个guard根，normal与race各127命名项（31根＋96子项）、0FAIL/0SKIP。格式与整仓源码扫描通过，会话89015于22:05:30 UTC自然退出0。完整命令与原始事件在r1/。

固定gofmt还整理了这两旧文件的既有紧凑排版；前后显著词法/全部字符串字面量一致，单独format-only.diff保留。WAL整源pin只更新到独立复核后的实际格式字节。初审与格式后补审为同一报告的不同时间段；tested-source中的初审hash保留原记录，当前完整报告包含补审。

## R6：普通重开写入的 nonce 域关联

对应goal §8.2:307。复用两个已有正常加密生命周期，新增有界只读公开header/record清单。域包含完整Epoch、FileID、file role、record role及nonce，不把AAD当派生域；按历史对象内容身份识别retained副本，避免将重读误报为新Seal。

新增关联检查正常关闭/重开后的新写入、普通checkpoint、rollback没有新对象及同凭据rekey新域。WAL的ParentCommitID是逻辑head，未错误要求它等于旧root字节hash。保留现有全部数据、Verify和生命周期断言；不读取/输出真实密钥。

精确3根normal/race各3PASS、0子项/FAIL/SKIP，format与sourceguard通过，会话68178于22:08:23 UTC自然退出0。两根确实操作隔离健康Linux数据库，另一根是纯值控制，不能统称纯模型；无wrong-key Open、篡改、故障恢复、进程退出或campaign。完整命令和事件在r6/。

## 复用、持久性与未完成项

组合后R1源码/调用闭包不变，复用其同码结果；未为R6重跑R1或全仓oracle/foreign。组合sourceguard新跑通过。详细绑定见reuse-map.json。Git包含压缩原始日志、命令、工具/源码身份、独立审阅与diff；两Library完整原包均已精确ID读回，见各目录receipt。恢复完整性不等运行验收。

R1/R6在[有限清单](../../testing/goal-finite-closure-checklist-20261012-zh.md)可勾选代码与非受限验证完成。R2写错位、R3checkpoint中断、R4扩展索引恢复账本、R5中断rekey仍在开发；原受阻ANN报告修正仍单列。真实物理/外部资格不因本批PASS而通过，整体未验收、M10未通过。
