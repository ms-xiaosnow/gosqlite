> 2026-10-12 三设施源码组合状态：本文件下方保留原供体说明与当时状态。当前组合从 main `7503531d2e3d9336bb5943ccd4c836c804d5ce03` 保留既有检查点，接入 shared SQL/A5 源树 `11ade9b1d33f7b317cdd613eed32cce886b6c1e1` 与单页 recovery 供体 `363b5cc7437f58e07722af3fef057fc0518ba309`。shared 最终七项有限续验已 PASS；recovery 供体原十项有限验证已 PASS。它们只适用于各自原始源树，不能算作本次组合 PASS。本组合仍为 SOURCE_ONLY / NOT_RUN，须在完整源审查后另行有限验证。所有实际 manual/native、真实故障、恢复和 process-exit 验收仍为 NOT_RUN；PhysicalValidated / Pass 不因此变为 true。下文旧 CODE_GAP、NOT_RUN 或候选 pin 字样是历史记录，不覆盖此组合状态。

# 单页 plain rollback fault → 保留输入 → 恢复副本（SOURCE ONLY）

当前状态：SOURCE_IMPLEMENTED_PENDING_INDEPENDENT_REVIEW；下面所有新测试、格式化、Go 列表、编译、源码守卫与人工物理调用均 NOT_RUN。当前候选尚有 main.go 旧全文件 pin 的有意 HOLD；不得声称守卫 PASS、完整 goal 完成、M3/M8/M10 或数据安全验收。

本文件描述真实源码接口，不是执行许可。build tag、环境变量、审阅过的源码或编译成功都不授予 physical 运行权限。当前代理不运行此流程，也不以另一个机器、远端、CI、subprocess 或 controller 绕过限制。

## 1. 唯一入口与有限三例

源码为 `internal/pager/rollback_fault_recovery_manual_linux_test.go`，精确构建条件 `linux && manual_fault_rollback`，入口 `TestManualRollbackFaultRecovery`。一个进程只选一个 case；无循环执行、子测试、后台 goroutine、重试、自动重启或 kill。旧 adapter lifetime tombstone 不清零。

- `control`：DB_PAGE 第一次 Pass。实际 Commit 必须 nil；完整新页、原 journal absent。恢复调用为零，fresh replay.db 的新 Reader 全页等于新页。
- `page-prefix-64`：DB_PAGE 第一次 PrefixShortWrite，prefix=64、PrefixError=false。实际 native WriteAt 返回 n=64、err=nil；原 writeFullAt 返回 io.ErrShortWrite；实际 Commit 同时保留它与 ErrRecoveryRequired。故障后全部 4096 bytes 必须等于旧页前64 bytes 被新页覆盖的固定图像。
- `db-sync-before`：DB_SYNC 第一次 FailBefore，目标 Sync 未执行。完整新页已经写入；实际 Commit 必须保留注入 cause 与 ErrRecoveryRequired。没有成功 ack，不将新页当作 durable。

两个 fault 都要求仅一次实际 RecoverRollback，Recovered=true、RestoredPages=1、OriginalPages=1；之后新 Reader 全部4096 bytes等于旧页，journal absent。不会把 ordinary reopen、Recovered=false、无journal、replay失败后再试或合成旧页算作恢复。

固定 fixture 为4096-byte、UTF-8、rollback、一个合法空 sqlite_schema table-leaf页；page/sector/extent/originals/dirty=4096/4096/1/1/1。旧 ChangeCounter/SchemaCookie/VersionValidFor=7/9/7；实际 BumpSchemaCookie 后，正常 Commit 预期8/10/8。未用区域固定 sentinel 也全部比较。独立内存规格先生成 old/new/partial，expected-new 在 Commit 前保留；不从 actual 生成 expected。

## 2. 未来执行前的独立要求

必须同时具备：

1. 独立源码审阅（包括 native call/owner闭包、纯判断器、负例和旧policy传播）、精确 main pin 单值迁移审阅；本候选未释放。
2. 针对最终源码与准确工具链的另行准许的 compile-only 收据，以及该确切二进制 hash；不能以旧树有限PASS代替。
3. 平台允许、用户明确批准这一个case的物理运行、保留路径与资源需求。此文不提供授权，也不让平台禁止的动作变得可执行。
4. 用户预建的全新、空、0700、归当前执行用户所有、绝对且clean、无symlink路径组件的私有根；整个调用期没有其他进程修改其命名空间。代码检查元数据变化，但不宣称抵御恶意并发替换。源码没有 chmod、mount 或权限变更。
5. 独立审阅的绑定 JSON、source inventory、compile receipt、工具chain文件，以及该次原始stdout/exit记录的保留方案。没有这些来源证据仍是 BLOCKED/HOLD。

仅源码清单、仅自报source标签、仅可执行文件hash或JSON里的布尔值不能单独证明 source→binary。下列文件的hash必须来自独立审阅过的实际编译与清单记录。运行代码只核对这些已审阅输入之间的一致性及当前二进制/工具文件hash；它不重新编译、不读取整棵source树、不认证外部签名，也不自行建立审阅信任。

## 3. 已实现环境变量与test flags

入口第一条语句先检查 `GOSQLITE_MANUAL_FAULT_ROLLBACK=independently_authorized`；否则 fatal NOT_RUN，绝不 Skip。此检查早于目录、fixture、owner及manifest读取。

其后必需环境变量：

- `GOSQLITE_ROLLBACK_FAULT_PRIVATE_ROOT_CONFIRMED=private_stable_namespace_retained`：用户确认私有稳定命名空间及保留全部结果。
- `GOSQLITE_ROLLBACK_FAULT_ROOT`：上述全新空根的绝对路径。
- `GOSQLITE_ROLLBACK_FAULT_BINDING`：独立审阅的绑定JSON文件路径；不放在空根中。
- `GOSQLITE_ROLLBACK_FAULT_BINDING_SHA256`：该绑定文件真实字节的64位小写hex SHA-256；零值拒绝。

case 在绑定文件的 Case 字段选择，不另有任意point、prefix、page-size、offset或repeat参数。

独立许可将来若获得，直接调用准确编译二进制时，必须逐一提供下列精确equals-form参数：

```text
<独立审阅过且获得运行许可的准确二进制> \
  -test.run=^TestManualRollbackFaultRecovery$ \
  -test.count=1 -test.shuffle=off -test.parallel=1 -test.timeout=0 -test.cpu=1 -test.v=true
```

shell 对含特殊字符的run参数须引用；传给二进制的实际值仍完全如上。`-test.v=true`也是必需参数，因为唯一成功terminal通过t.Logf输出，非verbose的成功test会抑制该日志。没有额外可选参数。任何缺失（包括verbose）、重复（包括verbose）、错误verbose值、无equals、额外selector、run regex union、子测试、CPU列表、count>1或非零timeout均拒绝。timeout=0只关闭testing的自动kill，不表示无限资源资格；外部监视/停止方式需独立授权，源码不会启动它。

普通/default和oracle构建不含入口。只带 manual_recovery 不含入口。组合tag可能选择其他既有native文件，因此只能另作静态inventory及compile-only审核；本绑定收据只接受 Tags=`manual_fault_rollback`，不提供组合tag人工运行路径。

## 4. 绑定文件和来源文件的准确格式

绑定 JSON 字段大小写如下，未知字段/尾随JSON拒绝。此处占位值不是有效输入：

```json
{
  "Source": "<source-inventory文件SHA256>",
  "Tool": "<准确Go工具文件SHA256>",
  "Binary": "<准确manual测试二进制SHA256>",
  "Manifest": "<同一个source-inventory文件SHA256>",
  "CompileReceipt": "<独立审阅compile-receipt文件SHA256>",
  "Trial": "<本case唯一ASCII无空白标识，1..128bytes>",
  "Scope": "<本Tx唯一ASCII无空白标识，1..128bytes>",
  "Case": "control",
  "ToolPath": "<准确Go工具文件路径>",
  "SourceManifestPath": "<独立冻结SOURCE-INVENTORY.json路径>",
  "CompileReceiptPath": "<实际编译后独立审阅收据路径>"
}
```

Source是冻结source inventory的SHA256身份，不是Git SHA1；本窄格式要求Source=Manifest。该清单沿用当前 SOURCE-INVENTORY.json 格式：顶层 `files`（path→`bytes`、`git_blob`、`git_mode`、`sha256`）与 `tree`。1..4096项；路径相对且clean；mode只接受100644/100755；bytes≥0；每项SHA256非零，git_blob/tree长度40。运行时不重新计算Git tree；完整树来源由独立审阅承担。

准确compile receipt JSON格式：

```json
{
  "Schema": "rollback-fault-compile-v1",
  "Source": "<与绑定Source相同>",
  "Tool": "<与绑定Tool相同>",
  "Binary": "<与绑定Binary相同>",
  "Manifest": "<与绑定Manifest相同>",
  "Package": "./internal/pager",
  "Tags": "manual_fault_rollback",
  "CompileOnly": true,
  "Files": 0,
  "SourceTree": "<准确清单的tree>"
}
```

Files=0只是无效占位，实际必须为清单真实项目数。此收据应由已审阅的实际编译证据提取、再次独立审核，不能预填编译成功或凭人工声称true替代真实工具回执。source/manifest/tool/binary/compile-receipt hash均非零；实际当前os.Executable、ToolPath内容必须匹配。绑定/编译收据读取上限64KiB，source inventory4MiB，工具与二进制各128MiB；读取前后核验regular file/大小/身份/mtime。复制的绑定文件完整保留在root。

## 5. 固定文件布局和不可逆证据保留

仅在确认的空根中以固定basename、O_CREATE|O_EXCL建立新文件，绝不覆盖已有名字：

- baseline.input.db：原始旧页，保留不变
- expected-new.input.db：事先固定的新页，保留不变
- binding.input.json：准确绑定输入，保留不变
- trial.db 与故障时 trial.db-journal：实际Commit结果；故障后不再恢复/Close/回滚原Tx来改变它们
- post-commit.input.db 与故障时 post-commit-journal.input.bin：实际故障后字节的新exclusive副本；不替换为生成的预期值
- replay.db 与故障时 replay.db-journal：再次exclusive新建的恢复family；仅这一family能被RecoverRollback触碰
- preterminal.json：含原Commit诊断、全部adapter诊断字符串、完整嵌套事件、Spec与尚未终态的Evidence

控制例三个journal名字都记录确切absence。故障journal固定实际长度8200，不固定生产随机nonce/hash；将真实bytes/hash保留。恢复前每个有内容的八个受检DB/journal都必须single-link、不同Dev/Ino；拒绝同inode/hardlink，不使用hardlink/reflink复制。

恢复参数固定MaxJournalBytes=8200、MaxRecords=1、MaxDatabaseBytes=4096。输入DB长度只能4096。恢复前后对原trial DB/journal、baseline、expected-new和retained副本做身份/尺寸/全部bytes hash复核，恢复后新Reader另比对完整页及header/extent。不会把只有header正确当成完整内容一致。

**成功、失败、BLOCKED、INCOMPLETE均不自动删除任何目录/文件。** 不用t.TempDir/RemoveAll、权限变化或重试cleanup。以后清理必须由用户审核并明确确认具体保留/删除对象与证据备份；本文不授权删除，也没有cleanup开关。

## 6. owner与结果解释

同词法Tx/scope中只有一次 `commitErr := tx.Commit(ctx)`，下一语句立即 `report := finishRollbackFault(scope, commitErr)`。不先Close、不替换error、不做第二Commit。完整AdapterObserved/targethit1、真实cause传播及所有具体owner退休后才disarm；否则保留unknown/pending scope与诊断并停止。不会清used tombstone或调用RetryPlaintextCleanup。

普通准备/读取/写入/receipt文件和Reader的非nil返回立即进入96槽owner ledger，即使同时返回error。每个实际Close至多一次；close error记录为未知且不重试。只有仍处于健康、未开始retirement的pre-arm Tx允许一次Rollback退休；pending/unknown/已开始retirement不重试。Commit接管后的owner不由普通ledger重复关闭。

实际RecoverRollback返回前生产defer已结束；同包源码在前后核对plain registry与PendingPlaintextCleanups为空，没有伪造recovery owner ID，也不从journal消失推断owner退休。新Reader单次Open/ReadPage/Close；全部普通owner完成后才进入最终一致性判断。

preterminal.json特意不是终态：文件不可能包含它自己的将来Close成功证明。该文件及其readback descriptor关闭后，代码更新完整owner清单，附第10个终态stage，调用纯ValidateRecoveryEvidence，再向调用方拥有的test log输出唯一一条 `ROLLBACK_RECOVERY_TERMINAL` JSON。终态绑定preterminal SHA256、完整Spec/Evidence/Observation。终态前输出文件错误必须INCOMPLETE；没有完整最终JSON、被截断、重复/错序stage、选中零case、只看testing退出码或字符串PASS，均不能认定ScenarioObserved。

纯Observation只叫ScenarioObserved，表示传入有界证据自洽。旧/新PhysicalValidated与Pass始终false。纯正反向vectors是合成值；不会因此变成真实物理证据。终态stdout的完整捕获也由未来独立执行记录负责，本源码没有stdout controller。

## 7. 验证状态与仍缺的代码

新pure receipt tests、旧adapter回归、WriterAt回归、AST正反例、默认/oracle/manual/组合包文件清单和compile-only均NOT_RUN，需冻结后分别审阅/审批。main.go whole-file旧pin保留，候选只提出精确新hash，不悄悄释放或伪造guard通过。

本片仅将单页真实caller/固定fixture/保留副本/恢复读取源码从CODE_GAP推进至SOURCE_IMPLEMENTED_PENDING_REVIEW/NOT_RUN。公开sqlite/driver fault caller、SQL行/普通与扩展索引/grams/vector事务账本、退出/重启controller、ack账本、replay再中断、其它五profile仍CODE_GAP或NOT_BOUND。真实ENOSPC/权限、错位/torn/sector、power-loss/FS/device持久性与全部A1–A8资格未关闭。正常control一次返回不是durable acknowledged transaction history。原goal尚未完成。
