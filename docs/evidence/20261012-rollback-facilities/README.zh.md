# 2026 年 10 月 12 日 rollback 三设施工程验证记录

本目录准备随检查点进入用户私有GitHub仓库，当前尚未推送。它保留命令、结果、source／tool身份和复用边界，不包含审批转发、工作指令、密钥、继承环境dump、大型原始测试流或二进制。

## 当前结果

- 旧0ef／78992仍FAIL：ordinary10925 PASS／1 FAIL／7 SKIP。唯一失败根的原始5条JSON事件原样保留。
- 修正51f63／44909：前五门PASS，race自然process0、testPASS，10928 PASS／7 SKIP，实际1005.409秒。范围缩减后等待race自然结束，记录器STOP_AUTHORIZATION_PAUSE／tool2、lease RELEASED。原22计划后16门未启动，不能记22门PASS。
- 补充v1在任何Go启动前因记录器便利字段KeyError自然exit1；源码未变、项目测试未启动，保留失败。
- 补充v2／2054：四平台支持包build、213包／1378输入whole清单、A5双manual-tag compile／nm共7项自然PASS。当前ELF映像未执行，required／forbidden roots为454／18。

## 8份有界文件

1. `identity-and-scope.json`：旧／新tree、三文件差异、44工具pin及module身份、当前whole清单和manual映像身份。
2. `prior-run-summary.json`：原78992完整阶段状态、包级计数、原退出与历史FAIL／UNKNOWN。
3. `sole-failed-test-events.jsonl`：唯一旧失败根全部原JSON事件，保留原字节与行号对应。
4. `exact-commands.json`：旧26计划、原22计划及实际未启动状态、范围缩减说明、新7项精确argv／cwd。所有outer命令固定env-i白名单，文件不是执行脚本。
5. `reuse-map.json`：9974个ordinary名的确定筛选与集合hash、五个quick、逐门后续复用、oracle组件依据和当前实际补充。
6. `archives.json`：完整原始日志、源基包和delta的Library ID＋SHA索引；尚未归档的44909／2054留PENDING。
7. `new-run-result.json`：44909行政pause与racePASS分别记账，补充v1零Go失败及2054七项真实结果。
8. 本README。

## 组合覆盖不伪装成重新执行

ordinary覆盖由当前metadata1＋testscope960和原78992未受影响9967 PASS／7 SKIP组成。旧ordinary依旧FAIL。原06 build与当前04完全相同；modules、164包shipping、LIMIT和encstore旧结果依据源／工具／闭包不变复用。四外平台只新编译三个支持包，其余部分复用main750。

oracle采用main750未变的oracle组件、当前scanner／testscope960／metadata1与78992的rollbackfault492／rollbackrestart157。变化包default／oracle选中文件相同，且没有oracle环境敏感初始化；细节见reuse-map。没有当前完整oracle脚本实跑，也不把预计12667名写成新实际总数。

当前whole闭包213包较main750的212只增加rollbackrestart；go/printer原已存在。A5桥确有变化，所以当前双manual-tag映像新编译并只读nm，不复用旧11ade映像身份，也不执行产物。

## 恢复、发布与边界

从Library取回包先核全文件SHA，再按manifest核members。51f63源码必须从精确0ef基包＋610823-byte delta重建全部3334文件；delta内44909 IN_PROGRESS历史快照不改。旧ordinary日志按reuse-map唯一规则重建9974名集合并核hash，无需重跑Go。

包含本记录的commit由Git历史提供，不内嵌自身SHA，不为写回SHA再做提交。应用文档时只核非文档字节／mode等于受测51f63。新bundle恢复为Git／字节核验＋必要smoke；未来CI当前PENDING，原main750的CI38259544486SUCCESS单独保留。没有本次Git推送或额外文档测试。

manual fault／recovery／process-exit与物理持久性仍NOT_RUN，M3/M8/M10未完成。[三设施主证据页](../rollback-facilities-checkpoint-20261012.md)和[原目标有限余缺](../../testing/remaining-goal-boundary-20261012-zh.md)保留六组代码工作、受阻ANN、M9最低范围及所有禁止扩张边界。
