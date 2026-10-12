# CHECK / EOF-count 源码组合与验收边界

## CHECK / EOF 检查点：完整正常门已通过（2026-10-12 07:22 UTC）

冻结运行树 `e06f35a022536a8ebd157731ee9a860521e70eb0` 于 **07:22:18 UTC**
自然完成 **26 门 / 28 次 Go 调用 PASS**，session 80563 exit 0，lease RELEASED。
ordinary/race 各 **8,948 named PASS / 7 个明确 SKIP**，完整 pinned oracle
**10,687 PASS / 0 named SKIP**。本副本只增加状态文档，运行源码与该冻结树一致；
本副本的新提交、bundle、精确恢复运行及对应 CI 均 **PENDING**。

CHECK 仅资格[已记录的有界 scalar 子集](check-constraints.md#精确-partial-profile)。
EOF-count 仍仅为 decode-only、EOFUnproven；native owner 在写入/Sync 前继续
finite-only fail-closed，未开放 EOF 恢复。外平台只有编译/符号检查，原生手工、
物理持久性/恢复、安全及 M3/M8/M10 未验收。后续 Darwin/Windows backend 和
真实只读媒体 harness 未纳入；其 source-only/HOLD 不被本次 PASS 覆盖。

已发布基点 `e092f0d4623cb243eec6053a4b3db7eef598542f` 的 bundle、精确恢复
26 门及 CI 38209370406 两项 SUCCESS 已另行闭合，只资格该基点。
[本检查点中文报告、精确收据与未完成项](evidence/check-eof-checkpoint-20261012.md)。

从下一节起保留各阶段原文；旧“当前”、SOURCE_ONLY、NOT_RUN、PENDING 与
FAIL/HOLD/UNKNOWN 均只指原时点和原树，不能覆盖本节的新执行记录，也不会被回写。

## 源码身份与当前状态

基点为已发布 `e092f0d4623cb243eec6053a4b3db7eef598542f` / tree
`d8ff20a26b4aee9fe3281a426e41230b18c727cd`。新组合是独立 SOURCE_ONLY 副本，
**所有组合执行 NOT_RUN；本 V2 已完成独立源审允许的唯一 sqlite.go pin 窄修，尚未发布**。
前驱 V1 tree `c788dc4d2aaa58af5efdebdc6bce0d42a6588dfd` 及其 SOURCE_PIN_HOLD 原样冻结。
精确本组合 tree 与全量路径/hash/逐 hunk 来源随独立外置证据封存，避免文档自引用 hash。

输入身份：

- CHECK 格式化 tree `9c3b0f09c1948c2c8b66337643677697d54852dc`；仅取相对于
  `cf1e27556d9b5aec314d72db2ae02a3810992580` 的 21 路径增量。V1 的 SOURCE_HOLD
  保留；V2 空限定名与 descriptor 早期预算修正及 formatter-only 后继已经独立源码复审。
- EOF 格式化 tree `1f5997643d83c6cf51687d6edac68a3c7eb9f06c`；仅取相对于
  `59ce26879c4301babc7648fbddc5cc66c4097263` 的 15 路径增量。第一且唯一 EOF-count
  解码、同 plan finite-only native 拒绝门与格式等价后继均有独立源码审阅。
- 17 个 CHECK 和 8 个 EOF production/test 文件的旧完整字节在新基点均一致；没有
  production/test 语义合并冲突。只移入这些增量，未用旧 sqlite/planner 整文件回退基点。
- 保留 c382 automatic UNIQUE 空结果表头修复、healthy read-only WAL keepers、
  LEFT/Rows、encrypted inspect、Darwin/Connector 手工门与全部更新基点文件。
  共享中文状态文档按来源合并；旧 FAIL/HOLD/UNKNOWN/PASS 不改写。

## 两项真实源码增量

CHECK 仅支持普通 rowid 表的有界、无显式函数 scalar profile：保留折叠前 source
准入、NULL-pass、UPDATE 赋值依赖、NOT NULL → CHECK → UNIQUE/storage 顺序及原
storage.Statement undo。DB.Check 只在 primary scan 每行评估一次；配额仍是累计 work
2^24、metadata 16 MiB 并受 MaxSortBytes 约束、2,000 声明和 8,192 原始节点；一般
函数、FK、STRICT、WITHOUT ROWID 与更广 SQL 不因此开放。详见[CHECK 合同](check-constraints.md)。

EOF-count 只解码 offset 0 第一且唯一 published `0xffffffff` 头；raw Header 保留，
有效数量为 len(Pages)，CountEvidence 明确为 Unknown 零值、FiniteHeaders 或 EOFUnproven。
完整 sector 后余量必须恰为完整记录，配额先于 metadata/图像分配；整条尾记录截断仍可能
结构合法，checksum/extent 不证明事务完整、认证、writer 来源或 durability。native
nil-default 和 opt-in 均不恢复 sentinel；同一新 plan 必须在 extent/restored-header/Sync/
写入前通过 finite-only admission，拒绝 unknown/未来枚举、EOFUnproven 及矛盾 raw sentinel。
writer、later sentinel、legacy page-size-zero、super/master 的原拒绝边界不放宽。
详见[内存解码](rollback-replay-memory.md)与[原生准入](rollback-recovery-open.md)。

CHECK 不改文件算法。EOF 改字节 decoder 与必须拒绝的 consumer admission；它不扩展
物理恢复、写入格式或 native authority。两者没有改变原 storage.Statement 唯一 undo
权威，也没有打开 default recovery、WAL/Backup publisher 或原生手工双门。

## 有限输入结果与组合结果分开

| 输入 | 已读终态记录 | 适用范围 |
|---|---|---|
| CHECK tree9c3b，session27563 | 2026-10-12 04:35:59 UTC，3 条普通调用，18 根 / 28 named PASS | parser、内存、健康小文件、driver 具名范围 |
| CHECK tree9c3b，session90221 | 05:27:58 UTC，1 条有限 oracle，3 根 / 7 named PASS，零 SKIP/FAIL | 原 CHECK oracle 对照；官方 14 进程计数是源码推导，不是逐子进程独立收据 |
| EOF tree1f599764，session88274 | completion 05:52:27 UTC，4 条普通调用，38 Test 根 + 2 固定种子 Fuzz 根，485 named PASS（477 ordinary + 8 seed 根/子项） | 具名内存、descriptor-free/source-AST 与六个原始固定种子；不执行 native recovery 或 oracle |
| 本 CHECK / EOF 组合 | ALL_COMBINED_EXECUTION_NOT_RUN；V2 唯一 pin 窄修，待后继读回 | 输入有限资格与基点 full26 均不转授 |

EOF 工具会话 88274 已自然 exit0（chunk 6eaac0），无强制终止，lease 已 RELEASED；
completion SHA-256 为 `b37e6c6ab97783210a6d6264bbf963207fd39f474a038c056e25f6090efa9b38`，
工具终态收据 SHA-256 为 `25b95f7d9cf55b69556627876e3adfd419884eb5b2548097199a62b1f10faccc`。
这些有限记录同样不构成本组合执行资格。
既有五切片运行 tree59ce 的完整正常 26 门 PASS 是基点历史。检查点 e092 的
Library bundle 已静态精确恢复到 d8ff；截至该恢复收据 05:49:18 UTC，恢复 full26
与对应 CI 仍 PENDING。静态恢复、输入有限 PASS、组合运行与物理资格是不同层。
本作者没有运行任何 Go/gofmt/compiler/test/guard/oracle/native、Git、远程或上传。

## 输入的持久归档补充

EOF 格式化原源码、有限原始日志、formatter/审阅/绑定资料已保存并完整读回：
Library `libfile_5433b237e50c8191a3e7e98452f88d84`，20,784,631 字节，SHA-256
`044a62f0e6c124b3fad2c2ad685d6e1d2aa1efe9ea1dd348f80e2b4d73aca9fe`；
3,743 成员 / 3,580 文件，3,270 个源码文件重算 tree1f599764 一致。
CHECK oracle 终态包 `libfile_a2ca813ce754819180b1601cbb6c6ce3` 已完整读回，SHA-256
`7d0384ea7788fc6621979e9015df6438a51c9dd972735cf9319c058a7980a573`。
这些归档只保存各自输入身份与收据，不是本 V2 源码归档、组合 PASS 或恢复/CI 资格。

## 独立审阅后的唯一 source-pin 窄修

CHECK 对 sqlite/sqlite.go 仅新增 MaxSortBytes 的 CHECK metadata/scratch 说明注释；
本组合新文件 SHA-256 为
`0e34c8d0e8a4bf537cb464d544e4dedde50ab04b19e1fbcb426fcca1931ecc6e`。
前驱 V1 的 `internal/testscope/wal_tail_publication.go` 仍保留原 hash
`0ea98f4f06300a819107dd2229dee9a2964142e4542bb2e582488749e056bf03`。

V1 的 **SOURCE_PIN_HOLD** 是源码身份阻塞，未被回写成执行 guard FAIL。独立审阅已经
完整核对 593 行 sqlite.go、Open/Close/ownership、publisher consumer 与 native recovery
准入闭包，仅批准 walPublicationProductionBoundaries 的 sqlite/sqlite.go 这一项替换为
上述新 hash。本 V2 的唯一 Go 改动就是该一个 hash 字面量；逆向替换逐字节还原 V1 guard。
没有“只有注释即可免审”的规则。其余 guard/所有手工 hash、默认排除、脚本/workflow、
模块、isolated oracle 工厂和 harness 原字节保留。source acceptance 不是 guard/runtime PASS；
仍待后继独立读回、实际默认闭包审阅与另批正常组合执行。

## 不变的目标与下一步

1. 冻结本 V2 完整身份与唯一 pin 前后证明；V1/HOLD 和全部历史输入保持不变。
2. 独立复核后继字节与文档；真实组合默认 import/init/test 选择闭包仍须另审。
3. 只在另行批准、精确后继身份绑定后运行正常组合资格；新发布/恢复/对应 CI 单独闭合。

Darwin 与 Connector 及 Backup/WAL/recovery/namespace 手工双门保持；ANN V4、41-target
fuzz campaign、未知中断链不启动。更广 SQL/文件格式、其他 OS/真实只读媒体、完整
crash/power-loss/short-write/Sync/ENOSPC/torn-write/认证负面矩阵、长测、大数据、
独立安全/密码学/法律审查仍未闭合。M3/M8/M10、生产安全及原始 goal 不宣布完成，
也不能归纳成“只差人工运行”。
