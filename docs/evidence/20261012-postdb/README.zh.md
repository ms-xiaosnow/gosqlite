# 两个 post-DB caller：影响验证与复用记录

基线 main `4dc3979aa8d503913b75e3c0c432c234d7bc0a40`，受测源码树 `449d420ca57e9ab7a454dcfdd33bc2d9aec8871a`。仅追加 JournalUnlink 与 FinalDirectorySync 的 FailBefore 手工 caller、账本、守卫和中文步骤；实际被选调用未执行。前者要求独立 NEW 快照及保留 journal 后在新副本恢复 OLD；后者要求无 journal、零恢复、观察 NEW 和 CommitUncertain，不能生成持久提交确认。

## 新执行与复用

会话34351六门退出0、源码稳定、资源闩锁未越界。准确时间、每个测试名、计数与命令见 result.json、commands.json 和三个无损 gzip 原始事件流。

- 格式：V1六Go文件通过，V2只改动的guard通过。
- rollbackfault normal：复用17587同字节模型50根＋1515子项，共1565 PASS、0FAIL/SKIP。
- testscope normal：V2新跑72根＋1038子项，共1110 PASS、0FAIL/SKIP。
- 两包race：V2新跑122根＋2553子项，共2675 PASS、0FAIL/SKIP。
- 整仓sourceguard：只扫描源码，PASS。
- Linux amd64双manual-tag编译与nm：454 required、18 absent；映像未执行。

没有重复运行无影响的全仓SQL、oracle或跨平台套件。A5共享的SQLControl计划及baseline/read/open/recovery helpers未变；新双tag编译检查其依赖可编译，不代表A5物理资格。

## 真实失败保留

首轮83fc源码的格式与模型通过，guard1110个命名项中1109PASS/1FAIL。唯一失败是 TestRollbackFaultSQLSourceContracts：实际源码中局部 var 的三行说明被 Go parser 附到 GenDecl.Doc，期望字面量未包含它。四行修正使期望按 ParseComments 保留同样说明；完整函数签名、body、顺序、四源pins、208个语义负例都保留。不是删除合同或改物理断言。

修前还发现并修正一条被新赋值覆盖的旧 pre-DB uncertainty 变体；它现在变异真实赋值，只错误放行旧5至7格而保留新9格合法分类。原源码/诊断/日志与修复均留存。首次FAIL后的race/sourceguard/compile/nm原为NOT_RUN，不改写成此前PASS。

## 证据与限制

Git保存命令、源树/工具绑定、结果、原始压缩日志及独立审查。大型编译映像、nm全输出和资源采样由 persistence-index.json 指向已验证Library归档；归档字节完整性不等运行验收。`gzip -dc *.events.jsonl.gz`可查看事件，不需重跑测试。

实际物理故障、恢复、进程退出、OS Sync返回失败、硬件持久性仍NOT_RUN。九格纯模型 PhysicalValidated()/Pass()仍false。整体未验收，M10未通过。
