# inspect 扩展元数据：实现与限定验收证据

日期：2026-10-11 UTC。对应 goal §3 原则 6/7、§12.2 的一项独立切片。
普通支持范围通过；没有完成全部 CLI 或 M10 验收。

## 实现与精确身份

- 基点：`7a3c45706b85104d3dc75d4947a333651226b69c`。
- 独立审阅及完整普通门精确 tree：
  `185944581731b5f21a5582ceb60ed340d4ad9012`，2,890 文件、136 目录。
- 七条路径：README、cmd/gosqlite/main.go、两个新 metadata 测试文件、
  sqlite/inspect.go、docs/cli-inspect.md、docs/progress.md。
  351 个既有测试文件的字节及 Git 文件模式保持，没有替换或弱化原断言。
- 本提交另加入中文契约和证据/状态文档；完整提交 tree 因文档而不同。
  所有非 docs 源（包括 README）保持与通过门的 tree 逐字节、Git 文件模式
  一致；不声称检出副本与冻结副本的全部 POSIX 权限相同。
- 完整门前先将候选增量和原始证据保存至 Library
  `libfile_ce043444c3b48191a4936926bb7b52d8`，版本 0；1,005,919 字节，
  SHA-256 `df9f87a98a957247b137e3d1f6f28e90ef6cca9dc5c92a39087ccc614fc1cea3`。
  字节读回、79 项 manifest、基点加七路径重建树通过；这只是备份完整性。
  最终远端/完整 git bundle/CI 按随后检查点逐项核验。

## 新行为与边界

新增 DB.Inspect(ctx, InspectOptions) 和 opt-in `inspect --extensions`。
Header、Schema、catalog 来自同一个 reader；借用 active Tx reader 时不关闭
事务，独立 reader 在成功返回前关闭。检查、context 或 owned-reader 关闭
错误返回零值结果；CLI 完整取得 metadata 后才编码。

只读取 sqlite_schema 和扩展 catalog，不读取 source、fields、postings、
frequency 或 ANN 数据树；不查找方法实现，也不调用 Configure、Canonical、
Keys。呈现持久 method/version/storage/config/generation，config 为不透明
UTF-8 字符串，支持合法非 JSON `whole-v1`。不会把持久化值解释为可用性、
完整性或安全恢复判断。默认 inspect 的完整 JSON 与已有格式保持。

保留 1024 记录上限；MaxSortBytes 对 metadata map/字符串/结果和解码进行
保守计费，在 overflow 拼接与 Decode 之前预留。原 schema/B-tree 上限
另行适用，不是总堆/RSS 的承诺。[中文 API/CLI 契约](../cli-inspect.md)
包含实际参数、可复制定名复核命令、错误停止、日志保存和适用限制。

## 独立复审与原始失败

先实际审阅测试及 helper，再执行小型、完整、正常 rollback fixture：

- SQLite 七项新定名测试，CLI 五项新测试和四项旧 inspect/context 回归
- 相同选择的 race、source guard、scoped vet 和 CGO0 build
- 完整默认 JSON、新数组、正常 grams Reindex 配置/代际、未知方法不透明配置
- callback 计数不增、被动页记录不进入数据根、低内存预算在 overflow 前拒绝
- 预取消/过期 context、事务继续可用、后续正常提交的完整 header 精确比较
- 只读目录 inventory 和 SHA 前后不变

以上由实现者及独立副本分别通过；没有新增错误注入、损坏文件或平台场景。
reader 关闭错误、无效 UTF-8 拒绝等未动态触发的分支只计源码审阅。

首次新增测试漏传 `tx.Commit(ctx)` 的 ctx，编译 exit 1；当时未运行数据库
测试。保留失败源码和原始日志，单参数修正后同一选择通过。
首次执行前还纠正了一项静态测试假设：建表可正常复用 freelist，不能要求
PageCount 严格增加；改为精确比较 writer 正常提交后的完整 header，并保留
SchemaCookie 增加及新 schema 对象断言。没有伪造修正前的运行结果。

- [原始定名日志及失败源码](inspect-metadata-20261011/scoped-original-logs.json)
- [实现者原始报告](inspect-metadata-20261011/owner-report-original.txt)
- [独立原始报告](inspect-metadata-20261011/independent-review-original.txt)

原始报告中的开发目录是历史出处，不是仓库运行依赖。独立报告原始
SHA-256：`5c2f4679a808b7ae19fbae013d05cb71392c7712e31f42ee817b437dc942f17e`。

## 固定源码完整普通门

实际 Go 1.27.2 linux/amd64，固定官方 SQLite 3.54.0，既有离线依赖；
GOMAXPROCS=2、默认 /tmp。生产 CGO0，race runtime 单独 CGO1。
仅 Linux 运行；五个其他目标只交叉编译，没有新 CI runner。

| 实际范围 | 自然结果（UTC） |
|---|---|
| 全仓普通 test，count=1，无 run/package 筛选 | PASS 02:51:13 |
| 全仓 vet / trimpath build | PASS 02:51:16 / 02:51:21 |
| 完整 scripts/test-oracle.sh | PASS 02:55:04 |
| 全仓 race，count=1；sqlite 411.368s | PASS 03:04:15 |
| linux/arm64、darwin/arm64、darwin/amd64、windows/amd64、windows/arm64 build | 全 PASS，末项 03:05:01 |
| go mod verify 与完整前后源/模式/目录身份 | PASS 03:05:03 |
| 总退出 | 0，03:05:03 |

执行前、oracle 前和 race 前都通过现有静态 guard；全套仅现有经过选择
审阅的 Linux 普通集合及上述新回归，没有 manual 标签、fuzz 或长测。
完整阶段无失败、工具拒绝、源码修改或重跑。

- [实际命令](inspect-metadata-20261011/ordinary-aggregate-command.txt)
- [未裁剪日志](inspect-metadata-20261011/ordinary-aggregate.txt)
- [阶段时间](inspect-metadata-20261011/stages.tsv)
- [执行前身份](inspect-metadata-20261011/identity-before.json)
- [执行后身份](inspect-metadata-20261011/identity-after.json)

完整日志 SHA-256：
`81ef1019f5b89907073d4a944f634b91d2c712a1bf11f04387771675f10148f6`。

## 仍有缺口

实测限定 Linux plaintext rollback、grams/custom catalog。加密 inspect、
真正只读 WAL、公开恢复、明文 backup、已知 SQL 差异和部分故障设施仍有
代码缺口或未合入；本次不关闭这些项。ANN V4、41 目标 fuzz、未知触发链路
保持阻塞。其他 OS 原生行为、物理持久性、长测、性能与独立 M10 审查未通过。
tmpfs/fsync=volatile 条件下的普通测试不能证明断电持久性。

旧源码的额外静态保全见 [中断记录](../interruption-20261010.md)；那次备份
没有执行旧代码，也没有把 held 版本视为 main 已实现功能。
