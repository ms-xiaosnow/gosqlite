# 五例目录终结与有限收尾证据

日期：2026-10-11。本次是条件性开发保存，整体未验收。

## 源码与实际结果

- 五例作者及独立审阅：只改 encrypted、encrypted-wal、encrypted-shared、encrypted-cli、native-check 的普通返回/终结器，加三份 fixturecleanup 文件。基点 1352ae524575e4d6fe8b5434fd930c3b07be4ac8；353份原测试不变，39个非 run 函数逐字不变。
- [独立审阅](example-retention-20261011/independent-review.md)：精确5项顶层测试普通/race、两次AST guard、限定vet/build通过。纯helper使用回调及sentinel，AST只读源文件；原有WAL/shared测试运行正常成功流程。
- [最终组合报告](example-retention-20261011/combined-report.md)：固定 tree `d7393c254a6fca4452ae194d64d9045659f9781d`，完整既有CGO0 unit/vet/trimpath build、完整SQLite 3.54.0普通oracle、sourceguards、完整CGO1 race、五目标编译、模块校验及前后源码身份，18阶段于04:23:35 UTC全部自然exit0。2,935文件和141目录未变；353份原测试及358份上一CLI组合测试均保持字节/模式。
- [原始输出](example-retention-20261011/ordinary-aggregate.txt) SHA256：`0a77721969a0ad05f3bdbaf755f501ad825182ccda5644a3c48c34560e8d5a17`；[实际命令](example-retention-20261011/ordinary-aggregate-command.txt)、[阶段退出码](example-retention-20261011/stages.tsv)、[机器记录](example-retention-20261011/aggregate-summary.json)另存。
- [集成静态复核](example-retention-20261011/handbook-independent-review.md)：最终main工作树777个非docs文件与固定组合的路径、字节、大小、权限及Git可执行位一致；goal原哈希不变。后加文档使整个tree不同，不能把固定tree日志改称在最终tree运行。

## 覆盖和限制

正常完整且所有后续defer都没有错误时才尝试删除；未完成/有错误时返回目录诊断并保留原error链。删除失败可能已部分删除；不自动重试，不承诺完整现场。进程内自有凭据照常清除，剩余密文可能不可解密；这不是密钥托管、备份或安全擦除。

真实失败留存、实际Close/RemoveAll故障、panic、失败进程、崩溃/断电、密钥错误/磁盘篡改和受限namespace均未执行。WAL原t.TempDir仍可能在测试结束清理失败目录；shared普通成功没有直接失败留存断言。snapshot/CLI/native-check本次没有main/run(binary)执行。五目标只是编译，普通包输出不是逐测试skip清单；既有普通seed/example不等于新fuzz campaign。

本环境default /tmp及工作盘的tmpfs/fsync=volatile限制仍在，普通Sync不能证明物理持久性；未换临时目录或用新runner规避。ANN V3 FAIL、ANN V4/41目标fuzz/未知中断和所有旧HOLD保持。

## 文档、保存与复验

中文[五例限定复验](../example-retention-verification-zh.md)和[有限剩余清单](../closeout-scope-20261011.md)已静态审阅；写入命令不等于执行。
预发布静态Archive为Library `libfile_d512bea615188191b50b8c2fbc6d8b79`，831,642字节，SHA256 `063176fc950e8fd2426e4e16abceaa65f3bdc3890190b7716900754db4e7bdb1`；142条目读回及基点+10delta文件对2907源文件身份重建通过，只证明保存。

本次新提交的远端SHA、Library bundle、精确恢复及current CI在本页编写时PENDING，必须各自读回并自然终态后才报告。上一提交或候选Archive结果不能替代新提交运行证据。

在现有限制下完成了独立可推进项，整体未验收。
