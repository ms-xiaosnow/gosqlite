# 三个 pre-DB caller：影响验证与复用记录

基线 main `a49d9bf2cb98101569fc15f6cbebf7ffc43069a1`，新受测树 `d023d4a01d48011c60e67b96a83230d806ab5660`。增加三个固定 FailBefore 场景：journal body Sync、journal header Sync、publish directory Sync。七路径有界增量，默认 shipping Commit/VFS/恢复协议不改。实际 Sync/helper 在所选点尚未被调用，不能将它们写成 OS Sync 返回失败或物理掉电通过。

## 已执行和复用

2026-10-12 21:06:47 UTC，session 83450 自然退出0，资源闩锁未越界，源码未改变。

| 检查 | 结果与准确范围 |
|---|---|
| 格式 | V1 六Go文件 PASS；V2仅改动guard再查PASS |
| 模型 normal | **复用**48278：47根＋1290子项＝1337 PASS，0FAIL/SKIP；修正只改guard，模型及导入/init闭包字节未变 |
| guard normal | V2新跑：72根＋971子项＝1043 PASS，0FAIL/SKIP |
| 两包 race | V2新跑：119根＋2261子项＝2380 PASS，0FAIL/SKIP |
| 整仓源码 guard | PASS；扫描源码，不运行manual入口 |
| Linux双tag test-c | PASS；生成映像未运行 |
| 官方nm只读符号 | 454 required、18 absent精确通过；映像 SHA-256见result.json |

不重跑未受影响的普通SQL/oracle/跨平台项目测试。pure rollbackrestart只依赖未变的SQLSnapshotEvidence/SQLRecoveryLimits/SQLState/ValidateSQLSnapshot，原适用结果复用；其manual armer共享SQLPlan(Control)的编译依赖由本次双tag编译核对。这不是A5新物理资格。

## 保留真实失败与最小修正

原V1树 `db1f6099d8da8421048cfbcd6f15f45e98cb8231` 的normal为模型1337PASS、guard1042PASS/1FAIL，失败是TestRollbackFaultSQLSourceContracts。Go printer对完整FuncDecl打印了Doc说明注释，而独立期望只有函数声明/body。仅guard浅拷贝FuncDecl并清副本Doc；原AST、完整签名/body/顺序、全部141语义变体和四源pins保留。原71变体全部保留，新增70；先前69/139元数据计数已纠正，未改断言凑数。

原始FAIL保存在failure-summary.json与v1-normal.events.jsonl.gz；后续race/guard/compile/nm当时未运行，不记FAIL或PASS。修后只重跑受影响guard包，接续原未跑四门，不从头重跑整个项目。

## 可审阅证据

- commands.json：每条实际argv、cwd与显式环境；没有继承manual/oracle环境或启动continuous fuzz。
- result.json：源身份、每门起止/退出、PASS/FAIL/SKIP和新执行/复用原因。
- 三份events.jsonl.gz：原Go JSON事件流无损gzip，mtime固定0；原字节和压缩件SHA-256都在result.json。可用`gzip -dc 文件名`查看完整事件，不需重新运行测试。
- source-review.md / guard-doc-correction.diff：独立源码结论与保留的最小修复。
- 大型编译映像、nm全量输出、资源采样与完整source manifest保存在独立Library归档，以persistence-index.json索引为准；Git不装大型映像。

所有实际故障注入、恢复、退出重启和本手册物理场景仍NOT_RUN。纯模型的PhysicalValidated()/Pass()保持false。整体未验收，M10未通过。
