# CHECK / EOF 开发检查点：精确运行与发布边界

## 当前结论

2026-10-12 07:22:18 UTC，冻结运行源码
`check-eof-integration-source-v2` / tree `e06f35a022536a8ebd157731ee9a860521e70eb0`
（3,280 文件、793 个 Go 文件）完成全部 **26 外层门 / 28 次 Go 调用 PASS**。
session **80563** 自然 exit **0**，工具 chunk **7da082**；lease **RELEASED**，
记录中的外层子进程自然返回、已观察子进程无仍活跃项、未强制终止。
这不是全局进程不存在的断言，也不是物理恢复或生产验收。

本 `check-eof-checkpoint-source-v1` 是该冻结源码的纯状态文档后继。
除本次明确列出的 Markdown 状态补记外，全部文件字节和模式保持一致；被测目录不改写。
最终 Git-compatible tree、全量文件 hash、逐文件文档 diff 与非文档一致性证明在外置
`check-eof-checkpoint-docs-evidence-v1/`，不在文档中预写自指 tree/commit。
本后继的新提交、远端发布、bundle、精确 Library 恢复运行及对应 CI 均 **PENDING**。

## 实际执行结果

- ordinary / race 各 **8,948 named PASS、7 个明确 SKIP**；1,461 个 Test 根、
  41 个 Fuzz 根和 6 个 Example 根，313 个固定种子。这里的 Fuzz 是固定种子，
  不代表已暂停的 41-target 持续 fuzz 已运行。
- 完整 pinned SQLite oracle **10,687 named PASS、0 named SKIP**；1,579 个 Test 根、
  41 个 Fuzz 根、6 个 Example 根、313 个固定种子。无测试文件的 package skip 与
  named test skip 区分，不能将其混为省略测试体。
- LIMIT oracle/race **394 PASS**；四条 quick 分别 **26 / 31 / 3 / 9 PASS**；
  healthy live-writer readonly oracle **7 PASS**。
- guard、vet、CGO0 build、五目标生产编译、模块校验和两项依赖闭包均 PASS。
  完整非测试闭包 207 包 / 1,350 输入；shipping CLI/library 164 包 / 1,257 输入。
- Darwin 两架构与 Windows 的 test-compile/nm 门 PASS；外国映像**未执行**，
  不能推导 Darwin/Windows 原生资格或文件系统、Unicode、锁、恢复与 durability 资格。
- 以上只绑定 tree e06f 的本次完整运行；不以输入切片 PASS 替代组合证据，
  不把跨阶段相同 named 项相加称为独立测试数量。

## CHECK 与 EOF 的支持域不扩大

CHECK 支持普通 rowid 表的有界、无显式函数 scalar 子集，保留完整原始 CREATE SQL、
折叠前 source 准入、NULL-pass、UPDATE 赋值依赖和 NOT NULL → CHECK → UNIQUE/storage
顺序；唯一回滚权威仍是原 storage.Statement undo。约束失败仍为 ErrConstraint /
primary CodeConstraint，不承诺 SQLite extended error code 全兼容。
DB.Check/Tx.Check 每个主表行只验证一次；违反为 ErrCorrupt 与零 CheckResult，
取消、quota、unsupported 或求值错误不能当作通过。累计 work 2^24、metadata 16 MiB
且受 MaxSortBytes 约束、2,000 声明与 8,192 原始节点的配额不放宽。

显式函数（包括 abs/length）、参数/子查询/跨表、private vector/typed-vector 引用、
scalar COLLATE、非 BINARY 列、FK、generated columns、STRICT、WITHOUT ROWID、
ON CONFLICT（含显式 ABORT）、INSERT/UPDATE OR、UPSERT 及未支持 DDL 仍拒绝。
详见[CHECK 精确合同](../check-constraints.md#精确-partial-profile)。

EOF-count 仅解码 offset 0 第一且唯一 published `0xffffffff` 头；保留 raw Header，
有效数量取 len(Pages)，CountEvidence 仍分 Unknown、FiniteHeaders、EOFUnproven。
整条尾记录截断仍可能结构合法；checksum/extent 不证明完整事务、认证、writer 来源
或 durability。native nil-default 与 opt-in 均不恢复 sentinel；同一 plan 在 extent、
restored-header、Sync 和写入前必须通过 finite-only admission，继续拒绝 unknown/
未来枚举、EOFUnproven 与矛盾 raw sentinel。writer、later sentinel、legacy page-size-zero
和 super/master 原拒绝域不变。见[内存解码](../rollback-replay-memory.md)与
[原生准入](../rollback-recovery-open.md)。

后续 Darwin/Windows backend 与 readonly-media harness 属于独立未发布源码，
未混入本检查点；媒体 harness 的真实 HOLD 保留。既有 raw namespace/manual 双门、
native Backup/WAL-tail、真实只读媒体/非特权身份、物理故障/恢复、安全与性能资格
不因此开放。ANN V4、41-target 持续 fuzz、旧 session 54102 UNKNOWN、全部既有
FAIL/HOLD/NOT_RUN 保持原记录。完整目标与 M3/M8/M10 尚未验收。

## 已发布基点与持久归档

已发布 main 基点是 `e092f0d4623cb243eec6053a4b3db7eef598542f`，tree
`d8ff20a26b4aee9fe3281a426e41230b18c727cd`，不含本 CHECK/EOF 增量。
[CI 38209370406](https://github.com/ms-xiaosnow/gosqlite/actions/runs/38209370406)
的 oracle / ordinary 分别于 05:52:17 / 06:03:39 UTC SUCCESS。
该基点的 Library bundle 为 `libfile_24024972e86c8191b21a145b4bc94432`；
独立精确恢复 session 59177 于 **06:40:47 UTC** 完成 26 门 / 28 Go PASS，
恢复终态证据包 `libfile_91aa9a24fd7081918427b76911aca236` 已完整读回。
较早 CI 归档中恢复仍 RUNNING 的文字保留原时点；不能据此否定后来单列的恢复终态。

原 e06f CHECK/EOF 与独立 Darwin 原源码的归档
`libfile_4ca40b801e288191b166151b81b5255c` 已读回；它只保存各自原源码和当时证据，
不等于本次完整运行终态归档、本文档后继 bundle、后继精确恢复或新 CI。

## 精确本地证据索引

以下路径相对本次保留目录
`/workspace/scratch/5e06a74e5f65/restart-20261011T2040/`；是本地收据索引，
不承诺发布后仓库含有这些外置文件，也不把尚未完成的归档预写为成功。

- `check-eof80563-tool-terminal.json`，SHA-256
  `f4f5df975ee1de2b47777dba8505cb9cfa56202b4775f665525c9e59fddc3b49`。
- `check-eof-combined-normal-verification-v1/completion.json`，SHA-256
  `fc1e22630d9765703322500c116c558ef7f4eefa471d3cf09499084793ae5671`。
- `check-eof-combined-normal-verification-v1/COMPILER-LEASE-RELEASE.json`，SHA-256
  `e7678f2b520405a53d82e710e379c85fd3dafce28c6e4264958a954be1dcc1ce`。
- 同一验证目录的各门 `.command.json`、`.stdout`、`.stderr`、`.natural-exit.json`、
  `.result.json`、测试 `.summary.json`、输入/源码前后身份和资源观察保留完整原始记录。
- `e092-publication-ci-archive-proof-v1/CI-TERMINAL-SUMMARY.json`，SHA-256
  `e79cefe02c9debb6f4965a4d44b1721c63b80abd9d2048b178d1885a266706ff`。
- `e092-restored-full26-terminal-archive-proof-v1/STATIC-TERMINAL-CROSSCHECK.json`，SHA-256
  `cb63b72e528b335dc9f6cf022d3620bf045aeef0797f4426c97e5c5323556957`。
- `e092-restored-full26-terminal-library-verified.json`，SHA-256
  `9b016708d06f67c7a299c6ab36881a9df48ab6f8b7ad8573f222a3c1ec9ce23d`。
- `check-eof-darwin-sources-library-verified.json`，SHA-256
  `387932ad97688491d2c7df0c9b8ad59ba0bdfb1dfdbafd5fb1b84a4370ce91f4`。

## 下一步

独立检查纯文档 diff、原始段落/命令块保留和最终 source identity；之后另行完成
新检查点发布、bundle、Library 读回、精确恢复及对应 CI，逐项记录真实终态。
本次文档整理未运行 Git、Go、编译器、项目脚本、测试、native、远端查询或上传。
