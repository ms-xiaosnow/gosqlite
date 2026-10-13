# 最后两批设施代码：完整 CI 与持久恢复结果

记录时间：2026-10-13 UTC。本证据提交只改变 docs/ 下的记录，生产源码、测试源码、模块及 CI 工作流均与 c5a8632908cc4747a5f581542a15a8532ef42ad9 完全相同。复用下列精确源提交的结果，没有本地重复全套测试。

## 精确终态

| 源码提交 | CI run / attempt | ordinary | oracle |
|---|---|---|---|
| b06606020fdbb5b73dd0e9b000773f3f02c6979d | [38321827127](https://github.com/ms-xiaosnow/gosqlite/actions/runs/38321827127) / 1，SUCCESS | 115051447164，2026-10-12 23:41:51 UTC SUCCESS | 115051446927，2026-10-12 23:32:30 UTC SUCCESS |
| c5a8632908cc4747a5f581542a15a8532ef42ad9 | [38323357015](https://github.com/ms-xiaosnow/gosqlite/actions/runs/38323357015) / 1，SUCCESS | 115056281469，2026-10-13 00:09:27 UTC SUCCESS | 115056281413，2026-10-12 23:50:33 UTC SUCCESS |

两批均通过既有 Ubuntu ordinary 工作流（sourceguard、go test ./...、go vet ./...、build、race）及固定官方 SQLite3.54.0 的完整普通 oracle 工作流。其真实命令／环境／包输出以原始日志为准，不从包级输出编造每个根的执行数量。固定 fuzz seed 不等于持续 campaign；所有 manual/native fault/crash/process-exit 入口仍由原 tag/env 排除，不能从 CI SUCCESS 推断物理资格。

c5 ordinary 曾在单一步骤运行约29分钟；步骤内没有实时状态。一次支持的 partial-log 读取返回404，按原记录保留，没有反复重试、取消或 rerun。其后自然成功，终态完整日志已取回。

## 可审阅记录

- [b066 完整终态／命令与日志索引](b066/README.zh.md)
- [c5 完整终态／命令与日志索引](c5/README.md)
- [R2/R5 本地修复前后与适用复用](../20261012-rekey-misplacement/README.zh.md)
- [R3/R4 本地修复前后、健康并发验证](../20261012-checkpoint-extensions/README.zh.md)

Git 保存压缩原始日志、元数据、固定 workflow/scripts、恢复与 Library 回执；完整原件可按索引中的字节哈希核验。没有编译映像或重复工作树加入 Git。压缩改变容器表示，不改变解压后的原始日志字节；GitHub原有秘密遮罩保留。

## 持久备份与精确恢复

- b066 bundle：libfile_9bd8c01766e08191a349b3605a63c9ac，11,107,623字节，SHA256 bdacbef29dfe3daa4c678e3ffbaa1ac8f38b740ad6ea77daebe4c25a380fc9ce；裸恢复3,868路径／Git mode／blob字节及fsck PASS。
- c5 bundle：libfile_9f5d934f0c7881918aee2869cd749aae，12,734,677字节，SHA256 2817259ab2d87259204f901800d5d6b2c020cde04e1791573ee5e1b47d3f02f4；裸恢复3,902路径／Git mode／blob字节及严格fsck PASS，61,647,557跟踪字节。
- b066 CI全证据：libfile_688e8fb4e8e08191a31d4f3c476c918e，290,500字节，SHA256 c8b2f43abeef00b64333237596f5fc26f462fba7dfc280b6a328fc419fba8e5b，17成员逐字节读回PASS。
- c5 CI全证据：libfile_c035a7d5e3e48191971d7f105d339069，295,304字节，SHA256 b59c6c58e98f161805d04429166a44cbd27ed295778eba49ca2072ec58756728，17成员逐字节读回PASS。

完整性恢复没有重复 Go 测试；Git mode/对象身份不等于复原全部 POSIX 元数据或真实 fsync 持久性。

## 适用范围与未验收项

锁定的有限R1–R6设施代码及受影响普通验证已闭合，最后代码树的配置内完整普通CI也已通过。已知受阻ANN报告修正仍是实际代码/报告缺口；受限物理场景、外部原生/设备、长期固定硬件、真实迁移输入以及独立安全/许可资格仍待完成。见[准确六项与三类剩余清单](../../testing/finite-acceptance-summary-20261012-zh.md)。整体未验收，M10未通过，没有Release或最终交付声明。

本次 docs-only 保存如自然触发现有工作流，会观察其终态并独立保留结果；不把一次同码的自动重复运行变成持续制造文档提交的循环。此处记录的是精确b066/c5源提交的证据，不提前声称后续docs提交的CI终态。
